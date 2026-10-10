package sitecrawl

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/truthrive/technical-seo-audit/go/deps/safe"
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

// queuedSitemap tracks a sitemap URL and its discovery provenance for traversal.
type queuedSitemap struct {
	url      string
	source   string
	parentID int
}

// discoverSitemaps finds and reads every sitemap for an origin.
//
// Returns the entries plus the sitemap URLs that were actually read, so the UI
// can report which files a crawl drew from.
func discoverSitemaps(ctx context.Context, client *http.Client, ua uaPreset, origin string,
	declared []string, parallel int) ([]sitemapEntry, []string) {
	_, entries, read := discoverSitemapsDetailed(ctx, client, ua, origin, declared, parallel)
	return entries, read
}

// discoverSitemapsDetailed performs sitemap discovery while capturing complete
// document, source provenance, entry, and limit telemetry.
func discoverSitemapsDetailed(ctx context.Context, client *http.Client, ua uaPreset, origin string,
	declared []string, parallel int) (sitemapEvidence, []sitemapEntry, []string) {

	startedAt := nowStamp()

	if parallel < 1 {
		parallel = 1
	}
	if parallel > sitemapMaxParallel {
		parallel = sitemapMaxParallel
	}

	var (
		ev      sitemapEvidence
		entries []sitemapEntry
		read    []string
		seen    = map[string]bool{}

		nextDocID = 1
		docIDs    = map[string]int{}
		getDocID  = func(u string) int {
			if id, ok := docIDs[u]; ok {
				return id
			}
			id := nextDocID
			nextDocID++
			docIDs[u] = id
			return id
		}

		sources   []sitemapSourceRecord
		seenSrc   = map[string]bool{}
		addSource = func(sitemapID int, src string, parentID int) {
			k := fmt.Sprintf("%d:%s:%d", sitemapID, src, parentID)
			if !seenSrc[k] {
				seenSrc[k] = true
				sources = append(sources, sitemapSourceRecord{
					SitemapID: sitemapID,
					Source:    src,
					ParentID:  parentID,
				})
			}
		}
	)

	var queue []queuedSitemap

	for _, d := range declared {
		trimmed := strings.TrimSpace(d)
		if trimmed != "" {
			docID := getDocID(trimmed)
			addSource(docID, "robots_txt", 0)
			queue = append(queue, queuedSitemap{url: trimmed, source: "robots_txt", parentID: 0})
		}
	}
	for _, p := range commonSitemapPaths {
		u := origin + p
		docID := getDocID(u)
		addSource(docID, "common_path", 0)
		queue = append(queue, queuedSitemap{url: u, source: "common_path", parentID: 0})
	}

	ev.Discovery.Status = "COMPLETED"
	ev.Discovery.StartedAt = startedAt

	var depthReached int

	for depth := 0; depth <= sitemapMaxDepth; depth++ {
		depthReached = depth
		if len(queue) == 0 {
			break
		}
		if ctx.Err() != nil {
			ev.Discovery.Status = "ATTEMPTED_INCOMPLETE"
			ev.Discovery.StopReason = "context_canceled"
			break
		}
		if len(entries) >= sitemapMaxURLs {
			ev.Discovery.URLsCapped = true
			ev.Discovery.StopReason = "urls_capped"
			break
		}

		// Dedupe within and across levels before spending a request.
		level := make([]queuedSitemap, 0, len(queue))
		for _, q := range queue {
			if !seen[q.url] {
				seen[q.url] = true
				level = append(level, q)
			}
		}
		queue = nil
		if len(level) == 0 {
			break
		}

		docs := make([]*sitemapDoc, len(level))
		telemetries := make([]sitemapFetchTelemetry, len(level))

		var wg sync.WaitGroup
		sem := make(chan struct{}, parallel)
		for i, q := range level {
			wg.Add(1)
			go func(i int, q queuedSitemap) {
				defer wg.Done()
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				case <-ctx.Done():
					return
				}
				safe.Do("sitecrawl/fetchSitemap", func() {
					doc, tel := fetchSitemapWithTelemetry(ctx, client, ua, q.url)
					docs[i] = doc
					telemetries[i] = tel
				})
			}(i, q)
		}
		wg.Wait()

		for i, q := range level {
			doc := docs[i]
			tel := telemetries[i]
			docID := getDocID(q.url)

			if tel.ByteCapped {
				ev.Discovery.ByteCapped = true
			}

			smRec := sitemapRecord{
				ID:              docID,
				URL:             q.url,
				DiscoverySource: q.source,
				ParentID:        q.parentID,
				InitialStatus:   tel.InitialStatus,
				FinalStatus:     tel.FinalStatus,
				Status:          tel.FinalStatus,
				FetchError:      tel.FetchError,
				RedirectTo:      tel.RedirectTo,
				RedirectHops:    tel.RedirectHops,
				FetchComplete:   tel.FetchComplete,
				DocType:         tel.DocType,
				ParseStatus:     tel.ParseStatus,
				ParseError:      tel.ParseError,
				EntryCount:      tel.EntryCount,
				FetchedAt:       nowStamp(),
			}

			if doc == nil {
				ev.Sitemaps = append(ev.Sitemaps, smRec)
				continue
			}

			read = append(read, q.url)

			for seq, u := range doc.URLs {
				loc := strings.TrimSpace(u.Loc)
				if loc == "" {
					continue
				}
				eRec := sitemapEntryRecord{
					SitemapID:  docID,
					Seq:        seq,
					URLID:      0,
					Loc:        loc,
					LastMod:    strings.TrimSpace(u.LastMod),
					ChangeFreq: strings.TrimSpace(u.ChangeFreq),
					Priority:   strings.TrimSpace(u.Priority),
				}
				ev.Entries = append(ev.Entries, eRec)

				if len(entries) < sitemapMaxURLs {
					entries = append(entries, sitemapEntry{
						Loc:        loc,
						LastMod:    eRec.LastMod,
						ChangeFreq: eRec.ChangeFreq,
						Priority:   eRec.Priority,
					})
				} else {
					ev.Discovery.URLsCapped = true
					ev.Discovery.StopReason = "urls_capped"
				}
			}

			for _, s := range doc.Sitemaps {
				loc := strings.TrimSpace(s.Loc)
				if loc != "" {
					childID := getDocID(loc)
					addSource(childID, "sitemap_index", docID)
					if depth < sitemapMaxDepth {
						queue = append(queue, queuedSitemap{
							url:      loc,
							source:   "sitemap_index",
							parentID: docID,
						})
					} else {
						ev.Discovery.DepthCapped = true
						ev.Discovery.StopReason = "depth_capped"
					}
				}
			}

			ev.Sitemaps = append(ev.Sitemaps, smRec)
		}
	}

	ev.Discovery.DepthReached = depthReached
	ev.Discovery.SitemapsFound = len(ev.Sitemaps)
	ev.Discovery.EntriesFound = len(ev.Entries)
	ev.Discovery.FinishedAt = nowStamp()
	ev.Sources = sources

	if ctx.Err() != nil && ev.Discovery.Status == "COMPLETED" {
		ev.Discovery.Status = "ATTEMPTED_INCOMPLETE"
		ev.Discovery.StopReason = "context_canceled"
	}

	return ev, entries, read
}

// sitemapFetchTelemetry captures raw HTTP and parse diagnostics for a sitemap document.
type sitemapFetchTelemetry struct {
	InitialStatus int
	FinalStatus   int
	RedirectTo    string
	RedirectHops  int
	FetchComplete bool
	FetchError    string
	DocType       string
	ParseStatus   string
	ParseError    string
	ByteCapped    bool
	EntryCount    int
}

// fetchSitemap fetches and decodes a sitemap document, maintaining backward compatibility.
func fetchSitemap(ctx context.Context, client *http.Client, ua uaPreset, rawURL string) (*sitemapDoc, bool) {
	doc, tel := fetchSitemapWithTelemetry(ctx, client, ua, rawURL)
	return doc, tel.ParseStatus == "parsed"
}

// fetchSitemapWithTelemetry fetches a sitemap document, recording initial/final HTTP status,
// redirect traversal, decompression, and permissive XML decode diagnostics.
func fetchSitemapWithTelemetry(ctx context.Context, client *http.Client, ua uaPreset, rawURL string) (*sitemapDoc, sitemapFetchTelemetry) {
	var tel sitemapFetchTelemetry
	tel.ParseStatus = "not_attempted"
	tel.DocType = "unknown"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		tel.FetchError = err.Error()
		return nil, tel
	}
	ua.apply(req, "", nil)

	var (
		initialStatus int
		hops          int
		finalTarget   string
	)

	reqClient := *client
	origCheck := client.CheckRedirect
	reqClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		hops = len(via)
		if initialStatus == 0 {
			if req.Response != nil {
				initialStatus = req.Response.StatusCode
			} else if len(via) > 0 && via[0].Response != nil {
				initialStatus = via[0].Response.StatusCode
			}
		}
		finalTarget = req.URL.String()
		if origCheck != nil {
			return origCheck(req, via)
		}
		if len(via) >= 10 {
			return http.ErrUseLastResponse
		}
		return nil
	}

	res, err := reqClient.Do(req)
	tel.RedirectHops = hops
	if hops > 0 {
		tel.InitialStatus = initialStatus
		tel.RedirectTo = finalTarget
	}

	if err != nil {
		tel.FetchError = err.Error()
		return nil, tel
	}
	defer res.Body.Close()

	tel.FetchComplete = true
	tel.FinalStatus = res.StatusCode
	if hops == 0 {
		tel.InitialStatus = res.StatusCode
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		io.Copy(io.Discard, io.LimitReader(res.Body, drainCap))
		return nil, tel
	}

	var body io.Reader = io.LimitReader(res.Body, sitemapByteCap+1)
	if strings.HasSuffix(strings.ToLower(rawURL), ".gz") ||
		strings.Contains(strings.ToLower(res.Header.Get("Content-Type")), "gzip") {
		zr, err := gzip.NewReader(body)
		if err != nil {
			tel.ParseStatus = "unavailable"
			tel.ParseError = "gzip: " + err.Error()
			return nil, tel
		}
		defer zr.Close()
		body = io.LimitReader(zr, sitemapByteCap+1)
	}

	rawBytes, err := io.ReadAll(body)
	if err != nil {
		tel.ParseStatus = "unavailable"
		tel.ParseError = err.Error()
		return nil, tel
	}
	if len(rawBytes) > sitemapByteCap {
		tel.ByteCapped = true
		rawBytes = rawBytes[:sitemapByteCap]
	}
	if len(rawBytes) == 0 {
		tel.ParseStatus = "empty_input"
		tel.DocType = "unknown"
		return nil, tel
	}

	var doc sitemapDoc
	dec := xml.NewDecoder(bytes.NewReader(rawBytes))
	dec.Strict = false
	dec.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }
	if err := dec.Decode(&doc); err != nil {
		tel.ParseStatus = "xml_error"
		tel.ParseError = err.Error()
		tel.DocType = "unknown"
		return nil, tel
	}
	if len(doc.URLs) == 0 && len(doc.Sitemaps) == 0 {
		tel.ParseStatus = "unsupported_structure"
		tel.DocType = "unknown"
		return nil, tel
	}
	if len(doc.URLs) > 0 {
		tel.DocType = "urlset"
		tel.ParseStatus = "parsed"
		tel.EntryCount = len(doc.URLs)
	} else {
		tel.DocType = "sitemapindex"
		tel.ParseStatus = "parsed"
		tel.EntryCount = 0
	}
	return &doc, tel
}
