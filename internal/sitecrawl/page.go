package sitecrawl

import (
	"net/url"
	"strings"
)


// buildPage assembles the stored record for one crawled URL from its transport
// result and parsed document.
func buildPage(item frontierItem, res fetched, seedHost string, opts Options) *Page {
	p := &Page{
		URL:         item.URL,
		Depth:       item.Depth,
		Source:      item.Source,
		Status:      res.Status,
		ContentType: res.ContentType,
		Kind:        res.Kind,
		SizeBytes:   res.Size,
		ResponseMs:  res.ElapsedMs,
		ErrorType:   res.ErrorType,
		Error:       res.Error,
		Redirects:   res.Hops,
		RedirectTo:  res.RedirectTo,
		LastMod:     res.LastMod,
		XRobotsTag:  res.XRobotsTag,
		BotBlocked:  res.BotBlocked,
		RobotsState: RobotsUnknown,
		MetaTags:    map[string]string{},
		OGTags:      map[string]string{},
		TwitterTags: map[string]string{},
		JSONLD:      []string{},
		SchemaOrg:   []Schema{},
		Hreflang:    []Hreflang{},
		Images:      []Image{},
		Issues:      []string{},
		Titles:      []string{},
		MetaDescs:   []string{},
		H1:          []string{},
		H2:          []string{},
		H3:          []string{},
		CrawledAt:   nowStamp(),
	}
	if u, err := url.Parse(item.URL); err == nil {
		p.Internal = sameSite(seedHost, u.Hostname(), opts.crawlSubdomains())
	}
	if res.Hops != nil {
		p.Redirects = res.Hops
	} else {
		p.Redirects = []Hop{}
	}

	d := res.Doc
	// A redirecting URL has no content of its own. The body that came back
	// belongs to the end of the chain, and that URL gets its own row — showing
	// its title here would report a 301 as if it were the page.
	if d == nil || len(res.Hops) > 0 {
		return p
	}

	p.Titles = d.Titles
	p.Title = first(d.Titles)
	p.MetaDescs = d.MetaDescs
	p.MetaDesc = first(d.MetaDescs)
	p.H1 = d.H1
	p.H2 = d.H2
	p.H3 = d.H3
	p.WordCount = d.WordCount
	if d.TotalBytes > 0 {
		p.TextRatio = float64(d.TextBytes) / float64(d.TotalBytes)
	}
	p.Lang = d.Lang
	p.Charset = d.Charset
	p.Viewport = d.Viewport
	p.Canonicals = d.Canonicals
	p.Canonical = first(d.Canonicals)
	p.MetaRobots = d.MetaRobots
	p.MetaRefresh = d.MetaRefresh
	p.RelNext = d.RelNext
	p.RelPrev = d.RelPrev
	p.Author = d.Author
	p.Keywords = d.Keywords
	p.Generator = d.Generator
	p.ThemeColor = d.ThemeColor
	p.MetaTags = d.MetaTags
	p.OGTags = d.OGTags
	p.TwitterTags = d.TwitterTags
	p.JSONLD = d.JSONLD
	p.SchemaOrg = d.SchemaOrg
	p.Hreflang = d.Hreflang
	p.Analytics = d.analytics()
	p.Images = d.Images

	for _, l := range d.Links {
		if u, err := url.Parse(l.Href); err == nil && sameSite(seedHost, u.Hostname(), opts.crawlSubdomains()) {
			p.OutlinksInternal++
		} else {
			p.OutlinksExternal++
		}
	}
	return p
}

// pageRow is the promoted-column projection written alongside the JSON blob.
// Everything here exists because SQL must filter, sort or aggregate on it.
type pageRow struct {
	URLID        int64
	URL          string
	Data         string
	Kind         string
	IsInternal   int
	Depth        int
	DiscoveredBy string
	Status       int
	StatusClass  int
	ContentType  string
	SizeBytes    int64
	ResponseMs   int
	RedirectTo   string
	RedirectHops int
	ErrorType    string
	Title        string
	TitleLen     int
	MetaDesc     string
	MetaDescLen  int
	H1           string
	H1Len        int
	H1Count      int
	H2           string
	H2Count      int
	WordCount    int
	TextRatio    float64
	Lang         string
	Canonical    string
	MetaRobots   string
	XRobots      string
	Indexable    int
	Indexability string
	RobotsState  string
	LastMod      string
	Outlinks     int
	OutlinksExt  int
	ImagesCount  int
	ImagesNoAlt  int
	IssueCount   int
	IssueMaxSev  int
	Rendered     int
	CrawledAt    string
}

// projectRow builds the promoted columns from an already-finished Page.
//
// In standalone acquisition, issue evaluation is decoupled from the crawler,
// so IssueCount and IssueMaxSev default to neutral zero.
func projectRow(id int64, p *Page, blob string) pageRow {
	r := pageRow{
		URLID:        id,
		URL:          p.URL,
		Data:         blob,
		Kind:         p.Kind,
		Depth:        p.Depth,
		DiscoveredBy: p.Source,
		Status:       p.Status,
		StatusClass:  p.Status / 100,
		ContentType:  p.ContentType,
		SizeBytes:    p.SizeBytes,
		ResponseMs:   p.ResponseMs,
		RedirectTo:   p.RedirectTo,
		RedirectHops: len(p.Redirects),
		ErrorType:    p.ErrorType,
		Title:        p.Title,
		TitleLen:     len([]rune(p.Title)),
		MetaDesc:     p.MetaDesc,
		MetaDescLen:  len([]rune(p.MetaDesc)),
		H1:           first(p.H1),
		H1Count:      len(p.H1),
		H2:           first(p.H2),
		H2Count:      len(p.H2),
		WordCount:    p.WordCount,
		TextRatio:    p.TextRatio,
		Lang:         p.Lang,
		Canonical:    p.Canonical,
		MetaRobots:   p.MetaRobots,
		XRobots:      p.XRobotsTag,
		Indexability: p.Indexability,
		RobotsState:  p.RobotsState,
		LastMod:      p.LastMod,
		Outlinks:     p.OutlinksInternal,
		OutlinksExt:  p.OutlinksExternal,
		ImagesCount:  len(p.Images),
		ImagesNoAlt:  countMissingAlt(p.Images),
		IssueCount:   0,
		IssueMaxSev:  0,
		CrawledAt:    p.CrawledAt,
	}
	r.H1Len = len([]rune(r.H1))
	if p.Internal {
		r.IsInternal = 1
	}
	if p.Indexable {
		r.Indexable = 1
	}
	if p.Rendered {
		r.Rendered = 1
	}
	return r
}

// countMissingAlt counts images without an alt attribute. Extracted from reference
// issues.go as a pure observation helper for stored image metrics.
func countMissingAlt(images []Image) int {
	n := 0
	for _, img := range images {
		if strings.TrimSpace(img.Alt) == "" {
			n++
		}
	}
	return n
}

// robotsHasToken reports whether a robots directive list carries a token.
// Extracted from reference issues.go as a pure string helper for compatibility display.
func robotsHasToken(directives, want string) bool {
	for _, part := range strings.Split(strings.ToLower(directives), ",") {
		part = strings.TrimSpace(part)
		if i := strings.LastIndex(part, ":"); i >= 0 {
			head := strings.TrimSpace(part[:i])
			if !strings.Contains(head, "_") {
				part = strings.TrimSpace(part[i+1:])
			}
		}
		if part == want {
			return true
		}
	}
	return false
}

// indexabilityOf reduces transport and markup signals to the legacy display verdict.
// Extracted from reference issues.go as a compatibility projection helper.
// Audit V1 has zero dependency on this function; it serves only compatibility storage.
func indexabilityOf(p *Page) (bool, string) {
	switch {
	case p.RobotsState == RobotsBlocked:
		return false, IndexBlockedRobots
	case robotsHasToken(p.MetaRobots+", "+p.XRobotsTag, "noindex"):
		return false, IndexNoindex
	case p.Status >= 300 && p.Status < 400:
		return false, IndexRedirect
	case p.Status < 200 || p.Status >= 300:
		return false, IndexNonOK
	case p.Canonical != "" && frontierKey(p.Canonical, false) != frontierKey(p.URL, false):
		return false, IndexCanonicalised
	}
	return true, ""
}

// dupeKeys are the normalised strings the exact-duplicate pass groups on.
func dupeKeys(p *Page) (title, meta, h1 string) {
	return strings.ToLower(strings.TrimSpace(p.Title)),
		strings.ToLower(strings.TrimSpace(p.MetaDesc)),
		strings.ToLower(strings.TrimSpace(first(p.H1)))
}
