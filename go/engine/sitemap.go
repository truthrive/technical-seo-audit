package sitecrawl

import (
	"compress/gzip"
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"sync"

	"onescout/desktop/internal/core/safe"
)

const (
	// sitemapByteCap bounds one sitemap read. A 50k-URL sitemap is ~5MB; the
	// spec caps them at 50MB uncompressed.
	sitemapByteCap = 64 << 20
	// sitemapMaxDepth bounds index recursion. A sitemap index pointing at itself
	// is not unheard of.
	sitemapMaxDepth = 10
	// sitemapMaxURLs bounds what one crawl will take from sitemaps, so a site
	// with a 5-million-URL index cannot blow past MaxURLs before the frontier
	// even gets a say.
	sitemapMaxURLs = 200000
	// sitemapMaxParallel caps how many sitemaps are fetched at once, however many
	// threads the crawl is configured for. Sitemap generation is the expensive
	// end of a CMS, and this burst arrives before the crawl has learned anything
	// about how well the site takes load.
	sitemapMaxParallel = 8
)

// commonSitemapPaths are probed when robots.txt declares none, in this order.
var commonSitemapPaths = []string{
	"/sitemap.xml",
	"/sitemap_index.xml",
	"/sitemaps.xml",
	"/sitemap/sitemap.xml",
}

// sitemapEntry is one <url> element. Only loc is required; the rest feed the
// Sitemaps tab.
type sitemapEntry struct {
	Loc        string
	LastMod    string
	ChangeFreq string
	Priority   string
}

// sitemapDoc covers both a urlset and a sitemapindex. Namespaces are ignored
// via XMLName matching on local names only, which is what makes this work
// across the many namespace variants in the wild.
type sitemapDoc struct {
	URLs []struct {
		Loc        string `xml:"loc"`
		LastMod    string `xml:"lastmod"`
		ChangeFreq string `xml:"changefreq"`
		Priority   string `xml:"priority"`
	} `xml:"url"`
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

// discoverSitemaps finds and reads every sitemap for an origin.
//
// Returns the entries plus the sitemap URLs that were actually read, so the UI
// can report which files a crawl drew from.
//
// One level of the index tree is fetched at a time, but everything within a
// level goes out together. Sitemaps are generated on demand by most CMSes and
// answer slowly whatever their size — a WordPress site measured 2.5s per file,
// so walking its 8 children one by one held the crawl in "Preparing" for 21
// seconds before a single page was fetched. Fetched together the same 8 took
// 6.4s, with no rate-limit response from a site that does rate-limit.
//
// parallel is the crawl's own thread count, capped: this runs before the crawl
// proper, so it must not be the moment a site first sees a burst.
func discoverSitemaps(ctx context.Context, client *http.Client, ua uaPreset, origin string,
	declared []string, parallel int) ([]sitemapEntry, []string) {

	if parallel < 1 {
		parallel = 1
	}
	if parallel > sitemapMaxParallel {
		parallel = sitemapMaxParallel
	}

	queue := append([]string{}, declared...)
	for _, p := range commonSitemapPaths {
		queue = append(queue, origin+p)
	}

	var (
		entries []sitemapEntry
		read    []string
		seen    = map[string]bool{}
	)

	for depth := 0; depth <= sitemapMaxDepth; depth++ {
		if len(queue) == 0 || len(entries) >= sitemapMaxURLs || ctx.Err() != nil {
			break
		}
		// Dedupe within and across levels before spending a request on anything.
		level := make([]string, 0, len(queue))
		for _, u := range queue {
			if !seen[u] {
				seen[u] = true
				level = append(level, u)
			}
		}
		queue = nil
		if len(level) == 0 {
			break
		}

		// Results are collected by index, so the order of entries does not depend
		// on which response happened to land first.
		docs := make([]*sitemapDoc, len(level))
		var wg sync.WaitGroup
		sem := make(chan struct{}, parallel)
		for i, rawURL := range level {
			wg.Add(1)
			go func(i int, rawURL string) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				// Parses third-party XML; one malformed sitemap must cost that
				// sitemap, not the crawl and not the app.
				safe.Do("sitecrawl/fetchSitemap", func() {
					if doc, ok := fetchSitemap(ctx, client, ua, rawURL); ok {
						docs[i] = doc
					}
				})
			}(i, rawURL)
		}
		wg.Wait()

		for i, doc := range docs {
			if doc == nil {
				continue
			}
			read = append(read, level[i])
			for _, u := range doc.URLs {
				loc := strings.TrimSpace(u.Loc)
				if loc == "" || len(entries) >= sitemapMaxURLs {
					continue
				}
				entries = append(entries, sitemapEntry{
					Loc: loc, LastMod: strings.TrimSpace(u.LastMod),
					ChangeFreq: strings.TrimSpace(u.ChangeFreq), Priority: strings.TrimSpace(u.Priority),
				})
			}
			for _, s := range doc.Sitemaps {
				if loc := strings.TrimSpace(s.Loc); loc != "" {
					queue = append(queue, loc)
				}
			}
		}
	}
	return entries, read
}

func fetchSitemap(ctx context.Context, client *http.Client, ua uaPreset, rawURL string) (*sitemapDoc, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false
	}
	ua.apply(req, "", nil)

	res, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(res.Body, drainCap))
		return nil, false
	}

	var body io.Reader = io.LimitReader(res.Body, sitemapByteCap)
	// Go's transport decompresses Content-Encoding: gzip transparently, but a
	// .xml.gz served as application/gzip is a gzip *payload* and has to be
	// unwrapped by hand.
	if strings.HasSuffix(strings.ToLower(rawURL), ".gz") ||
		strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "gzip") {
		zr, err := gzip.NewReader(body)
		if err != nil {
			return nil, false
		}
		defer zr.Close()
		body = io.LimitReader(zr, sitemapByteCap)
	}

	var doc sitemapDoc
	dec := xml.NewDecoder(body)
	// Sitemaps in the wild carry undeclared entities and stray encodings;
	// refusing to parse those would silently lose whole sites.
	dec.Strict = false
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	if err := dec.Decode(&doc); err != nil {
		return nil, false
	}
	if len(doc.URLs) == 0 && len(doc.Sitemaps) == 0 {
		return nil, false
	}
	return &doc, true
}
