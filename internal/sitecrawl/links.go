package sitecrawl

import (
	"net/url"
	"strings"
)

// edge is one link from one page to another, as stored.
type edge struct {
	SrcID     int64
	DstID     int64
	Seq       int
	Placement int
	Flags     uint32
	Anchor    string
}

// linkTargets is what a crawled page contributes to the graph: the edges to
// store, and the URLs to offer the frontier.
type linkTargets struct {
	Edges []edge
	// Discovered pairs a URL with the depth it was found at.
	Discovered []discovered
}

type discovered struct {
	URL   string
	Depth int
}

// relFlags reads a rel token list. LibreCrawl never looks at rel at all, so
// nofollow, ugc and sponsored links are invisible to it — one of the gaps this
// port closes.
func relFlags(rel string) uint32 {
	var f uint32
	for _, token := range strings.Fields(strings.ToLower(rel)) {
		switch token {
		case "nofollow":
			f |= FlagNofollow
		case "ugc":
			f |= FlagUGC
		case "sponsored":
			f |= FlagSponsored
		}
	}
	return f
}

// collectLinks turns a parsed document into edges and frontier candidates.
//
// It allocates a dictionary id for every target, including ones that will never
// be crawled: the Links tab has to be able to show an edge to an excluded or
// external URL.
func collectLinks(f *frontier, srcID int64, p *Page, doc *document, opts Options) linkTargets {
	out := linkTargets{}
	if doc == nil {
		return out
	}
	// Links are only followed from an HTML page that answered 2xx, and never
	// from an external one — following external links would walk the whole web.
	follow := p.Internal && p.Kind == KindHTML && p.Status >= 200 && p.Status < 300 && !opts.listMode()
	childDepth := p.Depth + 1

	seq := 0
	for _, l := range doc.Links {
		u, err := url.Parse(l.Href)
		if err != nil || u.Host == "" {
			continue
		}
		internal := sameSite(f.seedHost, u.Hostname(), opts.crawlSubdomains())

		flags := relFlags(l.Rel)
		if internal {
			flags |= FlagInternal
		}

		var dstID int64
		if follow {
			// A nofollow link still gets recorded, and still gets crawled: a
			// crawler's job is to report what is there, and Google itself treats
			// nofollow as a hint now.
			dstID, _ = f.admit(l.Href, childDepth, SourceLink, srcID)
			out.Discovered = append(out.Discovered, discovered{URL: l.Href, Depth: childDepth})
		} else {
			dstID, _ = f.idFor(frontierKey(l.Href, opts.IgnoreQueryParam), l.Href)
		}

		out.Edges = append(out.Edges, edge{
			SrcID: srcID, DstID: dstID, Seq: seq,
			Placement: l.Placement, Flags: flags, Anchor: l.Anchor,
		})
		seq++
	}

	// Images become edges too, so "which pages use this image?" is answerable
	// and the Images tab can report inlinks — and with CrawlImages on they are
	// admitted for fetching, which is what puts kind='image' rows in the grid.
	for _, img := range doc.Images {
		u, err := url.Parse(img.Src)
		if err != nil || u.Host == "" {
			continue
		}
		flags := FlagImageLink
		if sameSite(f.seedHost, u.Hostname(), opts.crawlSubdomains()) {
			flags |= FlagInternal
		}
		var dstID int64
		if follow && opts.CrawlImages {
			dstID, _ = f.admit(img.Src, childDepth, SourceLink, srcID)
		} else {
			dstID, _ = f.idFor(frontierKey(img.Src, opts.IgnoreQueryParam), img.Src)
		}
		out.Edges = append(out.Edges, edge{
			SrcID: srcID, DstID: dstID, Seq: seq,
			Placement: PlacementImage, Flags: flags, Anchor: truncate(img.Alt, maxAnchorLen),
		})
		seq++
	}

	// Stylesheets and scripts follow the images' pattern: always an edge, and a
	// fetch when their option is on. Without the fetch the CSS/JS tabs could
	// only ever show files someone linked with an <a href> — in practice, none.
	resource := func(list []string, flag uint32, fetch bool) {
		for _, src := range list {
			u, err := url.Parse(src)
			if err != nil || u.Host == "" {
				continue
			}
			flags := flag
			if sameSite(f.seedHost, u.Hostname(), opts.crawlSubdomains()) {
				flags |= FlagInternal
			}
			var dstID int64
			if follow && fetch {
				dstID, _ = f.admit(src, childDepth, SourceLink, srcID)
			} else {
				dstID, _ = f.idFor(frontierKey(src, opts.IgnoreQueryParam), src)
			}
			out.Edges = append(out.Edges, edge{SrcID: srcID, DstID: dstID, Seq: seq, Flags: flags})
			seq++
		}
	}
	resource(doc.Stylesheets, FlagStylesheet, opts.CrawlCSS)
	resource(doc.Scripts, FlagScript, opts.CrawlJS)
	return out
}

// referencedURLs returns the URLs a page points at through something other than
// an <a href>: canonical, hreflang alternates and rel=next/prev.
//
// They have to be crawled, or the rules that verify them have nothing to verify
// against — a canonical chain is only visible once both ends are known.
func referencedURLs(p *Page) []string {
	out := make([]string, 0, len(p.Hreflang)+len(p.Canonicals)+2)
	out = append(out, p.Canonicals...)
	for _, h := range p.Hreflang {
		if h.URL != "" {
			out = append(out, h.URL)
		}
	}
	if p.RelNext != "" {
		out = append(out, p.RelNext)
	}
	if p.RelPrev != "" {
		out = append(out, p.RelPrev)
	}
	return out
}
