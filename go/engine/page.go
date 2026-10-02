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
// Indexability is NOT computed here: it has to be settled before the JSON blob
// is marshalled, or the stored record and the column disagree — the blob would
// say "not indexable" with no reason while the grid said the opposite.
func projectRow(id int64, p *Page, blob string, issues []issue) pageRow {
	maxSev := 0
	for _, is := range issues {
		if s := severityOf(is.Code); s > maxSev {
			maxSev = s
		}
	}

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
		IssueCount:   len(issues),
		IssueMaxSev:  maxSev,
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

// dupeKeys are the normalised strings the exact-duplicate pass groups on.
func dupeKeys(p *Page) (title, meta, h1 string) {
	return strings.ToLower(strings.TrimSpace(p.Title)),
		strings.ToLower(strings.TrimSpace(p.MetaDesc)),
		strings.ToLower(strings.TrimSpace(first(p.H1)))
}
