package sitecrawl

import (
	"io"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// Collection caps. Screaming Frog shows the first two of most repeated
// elements and a count; keeping ten is generous and bounds the blob.
const (
	maxHeadings   = 10
	maxImages     = 500
	maxLinks      = 5000
	maxResources  = 200 // stylesheets or scripts per page
	maxAnchorLen  = 100
	maxJSONLD     = 20
	maxSchemas    = 40
	maxScriptScan = 256 << 10
)

// rawLink is one <a href> or image link as found in the document.
type rawLink struct {
	Href      string
	Anchor    string
	Rel       string
	Target    string
	Placement int
	IsImage   bool
}

// document is one page's parsed content, before it is resolved against the
// page's URL and turned into a Page.
type document struct {
	Titles      []string
	MetaDescs   []string
	H1          []string
	H2          []string
	H3          []string
	Lang        string
	Charset     string
	Viewport    string
	MetaRobots  string
	MetaRefresh string
	Author      string
	Keywords    string
	Generator   string
	ThemeColor  string
	BaseHref    string

	Canonicals []string
	RelNext    string
	RelPrev    string

	MetaTags    map[string]string
	OGTags      map[string]string
	TwitterTags map[string]string
	Hreflang    []Hreflang
	JSONLD      []string
	SchemaOrg   []Schema

	Links       []rawLink
	Images      []Image
	Stylesheets []string
	Scripts     []string

	WordCount  int
	TextBytes  int
	TotalBytes int

	analyticsBuf strings.Builder
}

// elem is one entry of the ancestor stack, carrying just enough to decide link
// placement plus the microdata scope this element opened, if any.
type elem struct {
	tag   string
	class string
	id    string
	scope *Schema
}

// voidElements never get an end tag, so they must not be pushed onto the
// ancestor stack — pushing them leaves the stack permanently unbalanced and
// every later link inherits a bogus placement.
var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}

// wordRe mirrors LibreCrawl's `\b\w+\b`.
var wordRe = regexp.MustCompile(`[\p{L}\p{N}_]+`)

var (
	ga4Re = regexp.MustCompile(`G-[A-Z0-9]{10}`)
	gtmRe = regexp.MustCompile(`GTM-[A-Z0-9]+`)
	gaRe  = regexp.MustCompile(`(?i)gtag\(|[^a-z]ga\(|GoogleAnalyticsObject|google-analytics\.com|googletagmanager\.com`)
	fbRe  = regexp.MustCompile(`(?i)fbq\(|facebook\.com/tr`)
	hjRe  = regexp.MustCompile(`(?i)hotjar\.com|hj\(`)
	mpRe  = regexp.MustCompile(`(?i)mixpanel\.com|mixpanel\.track`)
)

// extractDocument runs a single streaming pass over the HTML.
//
// A tokenizer rather than html.Parse: no tree is allocated, it tolerates the
// truncated document a byte cap produces, and at 50k pages the difference in
// allocation is the difference between a crawl that finishes and one that
// swaps.
func extractDocument(r io.Reader, base *url.URL) *document {
	d := &document{
		MetaTags:    map[string]string{},
		OGTags:      map[string]string{},
		TwitterTags: map[string]string{},
	}

	z := html.NewTokenizer(r)
	z.SetMaxBuf(0) // bounded by the caller's LimitReader instead

	var (
		stack []elem
		// text collection
		skipDepth int // >0 while inside <script>/<style>/<noscript>
		// open anchor, if any
		anchorOpen bool
		anchor     strings.Builder
		anchorLink rawLink
		// open <title>
		titleOpen bool
		title     strings.Builder
		// open heading
		headingTag string
		heading    strings.Builder
		// open ld+json
		jsonldOpen bool
		jsonld     strings.Builder
		// innermost open microdata scope, and the property being collected
		propName string
		propText strings.Builder
	)

	// openScope returns the innermost microdata item still on the stack.
	openScope := func() *Schema {
		for i := len(stack) - 1; i >= 0; i-- {
			if stack[i].scope != nil {
				return stack[i].scope
			}
		}
		return nil
	}

	flushHeading := func() {
		if headingTag == "" {
			return
		}
		text := squash(heading.String())
		switch headingTag {
		case "h1":
			d.H1 = appendCapped(d.H1, text, maxHeadings)
		case "h2":
			d.H2 = appendCapped(d.H2, text, maxHeadings)
		case "h3":
			d.H3 = appendCapped(d.H3, text, maxHeadings)
		}
		headingTag = ""
		heading.Reset()
	}

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		raw := z.Raw()
		d.TotalBytes += len(raw)

		switch tt {
		case html.TextToken:
			text := string(raw)
			if skipDepth > 0 {
				// Script and style bodies feed analytics detection but never the
				// word count: a React bundle would otherwise read as a 50,000-word
				// page. LibreCrawl counts them; Screaming Frog does not, and this
				// tool follows Screaming Frog.
				d.scanAnalytics(text)
				// ld+json is a script body too, so it has to be collected here —
				// the branch below is never reached for it.
				if jsonldOpen {
					jsonld.WriteString(text)
				}
				continue
			}
			d.TextBytes += len(text)
			d.WordCount += len(wordRe.FindAllString(text, -1))
			if anchorOpen {
				anchor.WriteString(text)
			}
			if titleOpen {
				title.WriteString(text)
			}
			if headingTag != "" {
				heading.WriteString(text)
			}
			if propName != "" {
				propText.WriteString(text)
			}

		case html.StartTagToken, html.SelfClosingTagToken:
			nameB, hasAttr := z.TagName()
			name := string(nameB)
			attrs := readAttrs(z, hasAttr)

			switch name {
			case "script", "style", "noscript", "template":
				if src := attrs["src"]; src != "" {
					d.scanAnalytics(src)
					if name == "script" {
						d.Scripts = appendCapped(d.Scripts, strings.TrimSpace(src), maxResources)
					}
				}
				if name == "script" && strings.Contains(strings.ToLower(attrs["type"]), "ld+json") {
					jsonldOpen = true
					jsonld.Reset()
				}
				if tt == html.StartTagToken {
					skipDepth++
				}
			case "title":
				if tt == html.StartTagToken {
					titleOpen = true
					title.Reset()
				}
			case "html":
				if d.Lang == "" {
					d.Lang = strings.TrimSpace(attrs["lang"])
				}
			case "base":
				if d.BaseHref == "" {
					d.BaseHref = strings.TrimSpace(attrs["href"])
				}
			case "meta":
				d.readMeta(attrs)
			case "link":
				d.readLink(attrs)
			case "h1", "h2", "h3":
				if tt == html.StartTagToken {
					flushHeading()
					headingTag = name
					heading.Reset()
				}
			case "a":
				if tt == html.StartTagToken {
					anchorOpen = true
					anchor.Reset()
					anchorLink = rawLink{
						Href:      strings.TrimSpace(attrs["href"]),
						Rel:       strings.ToLower(strings.TrimSpace(attrs["rel"])),
						Target:    strings.TrimSpace(attrs["target"]),
						Placement: placementOf(stack),
					}
				}
			case "img":
				img := Image{
					Src:    strings.TrimSpace(attrs["src"]),
					Alt:    attrs["alt"],
					Width:  attrs["width"],
					Height: attrs["height"],
				}
				// Lazy loaders (LiteSpeed, WP Rocket, lazysizes) park a data:
				// placeholder — or nothing — in src and the real URL in a data-*
				// attribute. A data: src is a placeholder, never the image.
				if img.Src == "" || strings.HasPrefix(img.Src, "data:") {
					if real := strings.TrimSpace(firstOf(attrs, "data-src", "data-lazy-src", "data-original")); real != "" {
						img.Src = real
					} else if ss := firstOf(attrs, "srcset", "data-srcset"); ss != "" {
						// First srcset candidate: "url 1x, url2 2x".
						if cand := strings.Fields(strings.SplitN(ss, ",", 2)[0]); len(cand) > 0 {
							img.Src = cand[0]
						}
					} else if strings.HasPrefix(img.Src, "data:") {
						img.Src = ""
					}
				}
				if img.Src != "" && len(d.Images) < maxImages {
					d.Images = append(d.Images, img)
				}
				if anchorOpen && anchorLink.Anchor == "" && img.Alt != "" {
					anchorLink.Anchor = img.Alt
				}
			}

			// Microdata: itemprop collects into the innermost open item.
			if p := strings.TrimSpace(attrs["itemprop"]); p != "" {
				if scope := openScope(); scope != nil {
					propName = p
					propText.Reset()
					// Attribute-valued props resolve immediately.
					if v := microdataAttrValue(name, attrs); v != "" {
						scope.Properties[p] = v
						propName = ""
					}
				}
			}

			if tt == html.StartTagToken && !voidElements[name] {
				e := elem{tag: name, class: attrs["class"], id: attrs["id"]}
				// An itemscope opens an item that closes with this element.
				if _, ok := attrs["itemscope"]; ok && len(d.SchemaOrg) < maxSchemas {
					e.scope = &Schema{Type: attrs["itemtype"], Properties: map[string]string{}}
				}
				stack = append(stack, e)
			}

		case html.EndTagToken:
			nameB, _ := z.TagName()
			name := string(nameB)

			switch name {
			case "script", "style", "noscript", "template":
				if skipDepth > 0 {
					skipDepth--
				}
				if jsonldOpen {
					if s := strings.TrimSpace(jsonld.String()); s != "" && len(d.JSONLD) < maxJSONLD {
						d.JSONLD = append(d.JSONLD, s)
					}
					jsonldOpen = false
					jsonld.Reset()
				}
			case "title":
				if titleOpen {
					d.Titles = appendCapped(d.Titles, squash(title.String()), 5)
					titleOpen = false
				}
			case "h1", "h2", "h3":
				flushHeading()
			case "a":
				if anchorOpen {
					if anchorLink.Anchor == "" {
						anchorLink.Anchor = squash(anchor.String())
					}
					if anchorLink.Href != "" && len(d.Links) < maxLinks {
						anchorLink.Anchor = truncate(anchorLink.Anchor, maxAnchorLen)
						d.Links = append(d.Links, anchorLink)
					}
					anchorOpen = false
				}
			}

			if propName != "" {
				if scope := openScope(); scope != nil {
					if v := squash(propText.String()); v != "" {
						scope.Properties[propName] = v
					}
				}
				propName = ""
			}

			// Pop the ancestor stack to the matching tag, harvesting any microdata
			// items that closed with it. Unbalanced markup is the norm, so scan
			// back rather than assuming the top matches.
			for i := len(stack) - 1; i >= 0; i-- {
				if stack[i].tag != name {
					continue
				}
				for _, e := range stack[i:] {
					d.SchemaOrg = harvestScope(d.SchemaOrg, e.scope)
				}
				stack = stack[:i]
				break
			}
		}
	}

	flushHeading()
	// A truncated document leaves elements open; harvest whatever they collected.
	for _, e := range stack {
		d.SchemaOrg = harvestScope(d.SchemaOrg, e.scope)
	}
	d.resolve(base)
	return d
}

// harvestScope records a microdata item once its element closes. Nested items
// flatten into one list, which is what LibreCrawl produces too.
func harvestScope(out []Schema, s *Schema) []Schema {
	if s == nil || len(out) >= maxSchemas {
		return out
	}
	if s.Type == "" && len(s.Properties) == 0 {
		return out
	}
	return append(out, *s)
}

func microdataAttrValue(tag string, attrs map[string]string) string {
	switch tag {
	case "meta":
		return attrs["content"]
	case "img", "audio", "embed", "iframe", "source", "track", "video":
		return attrs["src"]
	case "a", "area", "link":
		return attrs["href"]
	case "time":
		return attrs["datetime"]
	}
	return ""
}

// readAttrs drains every attribute of the current token. The tokenizer requires
// all of them to be read before Next is called again.
func readAttrs(z *html.Tokenizer, hasAttr bool) map[string]string {
	attrs := map[string]string{}
	for hasAttr {
		var k, v []byte
		k, v, hasAttr = z.TagAttr()
		attrs[strings.ToLower(string(k))] = string(v)
	}
	return attrs
}

func (d *document) readMeta(attrs map[string]string) {
	content := strings.TrimSpace(attrs["content"])

	if cs := strings.TrimSpace(attrs["charset"]); cs != "" && d.Charset == "" {
		d.Charset = cs
	}
	if he := strings.ToLower(strings.TrimSpace(attrs["http-equiv"])); he != "" {
		switch he {
		case "content-type":
			if d.Charset == "" {
				if i := strings.Index(strings.ToLower(content), "charset="); i >= 0 {
					cs := content[i+len("charset="):]
					cs = strings.TrimSpace(strings.SplitN(cs, ";", 2)[0])
					d.Charset = cs
				}
			}
		case "refresh":
			if d.MetaRefresh == "" {
				d.MetaRefresh = content
			}
		case "content-language":
			if d.Lang == "" {
				d.Lang = content
			}
		}
		return
	}

	if prop := strings.ToLower(strings.TrimSpace(attrs["property"])); prop != "" {
		if strings.HasPrefix(prop, "og:") {
			d.OGTags[strings.TrimPrefix(prop, "og:")] = content
		}
		return
	}

	name := strings.ToLower(strings.TrimSpace(attrs["name"]))
	if name == "" {
		return
	}
	if strings.HasPrefix(name, "twitter:") {
		d.TwitterTags[strings.TrimPrefix(name, "twitter:")] = content
		return
	}
	d.MetaTags[name] = content

	switch name {
	case "description":
		d.MetaDescs = appendCapped(d.MetaDescs, content, 5)
	case "robots":
		if d.MetaRobots == "" {
			d.MetaRobots = content
		} else {
			d.MetaRobots += ", " + content
		}
	case "googlebot":
		// Google honours a googlebot-specific directive over the generic one.
		if d.MetaRobots == "" {
			d.MetaRobots = content
		} else {
			d.MetaRobots += ", " + content
		}
	case "viewport":
		d.Viewport = content
	case "author":
		d.Author = content
	case "keywords":
		d.Keywords = content
	case "generator":
		d.Generator = content
	case "theme-color":
		d.ThemeColor = content
	}
}

func (d *document) readLink(attrs map[string]string) {
	rel := strings.ToLower(strings.TrimSpace(attrs["rel"]))
	href := strings.TrimSpace(attrs["href"])
	if rel == "" || href == "" {
		return
	}
	for _, token := range strings.Fields(rel) {
		switch token {
		case "stylesheet":
			d.Stylesheets = appendCapped(d.Stylesheets, href, maxResources)
		case "canonical":
			d.Canonicals = appendCapped(d.Canonicals, href, 5)
		case "next":
			if d.RelNext == "" {
				d.RelNext = href
			}
		case "prev", "previous":
			if d.RelPrev == "" {
				d.RelPrev = href
			}
		case "alternate":
			if lang := strings.TrimSpace(attrs["hreflang"]); lang != "" {
				d.Hreflang = append(d.Hreflang, Hreflang{Lang: lang, URL: href})
			}
		}
	}
}

// scanAnalytics feeds script bodies and src attributes to the tracker
// detectors, bounded so a huge inline bundle cannot dominate the crawl's CPU.
func (d *document) scanAnalytics(s string) {
	if d.analyticsBuf.Len() >= maxScriptScan {
		return
	}
	room := maxScriptScan - d.analyticsBuf.Len()
	if len(s) > room {
		s = s[:room]
	}
	d.analyticsBuf.WriteString(s)
}

// analytics runs the detectors once, at the end.
func (d *document) analytics() Analytics {
	s := d.analyticsBuf.String()
	var a Analytics
	if m := ga4Re.FindString(s); m != "" {
		a.GA4ID = m
		a.Gtag = true
	}
	if m := gtmRe.FindString(s); m != "" {
		a.GTMID = m
	}
	a.GoogleAnalytics = a.Gtag || a.GTMID != "" || gaRe.MatchString(s)
	a.FacebookPixel = fbRe.MatchString(s)
	a.Hotjar = hjRe.MatchString(s)
	a.Mixpanel = mpRe.MatchString(s)
	return a
}

// resolve turns every collected reference into an absolute URL, honouring a
// <base href> when the document declares one.
func (d *document) resolve(base *url.URL) {
	effective := base
	if d.BaseHref != "" {
		if b, err := base.Parse(d.BaseHref); err == nil && b.Host != "" {
			effective = b
		}
	}
	for i := range d.Canonicals {
		d.Canonicals[i] = absolutize(effective, d.Canonicals[i])
	}
	for i := range d.Hreflang {
		d.Hreflang[i].URL = absolutize(effective, d.Hreflang[i].URL)
	}
	for i := range d.Images {
		d.Images[i].Src = absolutize(effective, d.Images[i].Src)
	}
	for i := range d.Stylesheets {
		d.Stylesheets[i] = absolutize(effective, d.Stylesheets[i])
	}
	for i := range d.Scripts {
		d.Scripts[i] = absolutize(effective, d.Scripts[i])
	}
	d.RelNext = absolutize(effective, d.RelNext)
	d.RelPrev = absolutize(effective, d.RelPrev)

	out := d.Links[:0]
	for _, l := range d.Links {
		if abs := resolveLink(effective, l.Href); abs != "" {
			l.Href = abs
			out = append(out, l)
		}
	}
	d.Links = out
}

func absolutize(base *url.URL, raw string) string {
	if raw == "" {
		return ""
	}
	ref, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	return base.ResolveReference(ref).String()
}

// placementOf walks the ancestor stack to classify where a link sits.
// Footer wins over navigation, matching LibreCrawl.
func placementOf(stack []elem) int {
	for i := len(stack) - 1; i >= 0; i-- {
		e := stack[i]
		hint := strings.ToLower(e.class + " " + e.id)
		if e.tag == "footer" || strings.Contains(hint, "footer") {
			return PlacementFooter
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		e := stack[i]
		hint := strings.ToLower(e.class + " " + e.id)
		switch {
		case e.tag == "nav", strings.Contains(hint, "nav"), strings.Contains(hint, "menu"):
			return PlacementNav
		case e.tag == "header", strings.Contains(hint, "header"):
			return PlacementHeader
		}
	}
	return PlacementBody
}

func appendCapped(dst []string, v string, max int) []string {
	if v == "" || len(dst) >= max {
		return dst
	}
	return append(dst, v)
}

func firstOf(attrs map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := attrs[k]; v != "" {
			return v
		}
	}
	return ""
}

// squash collapses all whitespace runs into single spaces and trims.
func squash(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Cut on a rune boundary so the stored anchor is never invalid UTF-8.
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func first(list []string) string {
	if len(list) == 0 {
		return ""
	}
	return list[0]
}
