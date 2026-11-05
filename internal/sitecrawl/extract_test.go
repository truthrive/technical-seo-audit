package sitecrawl

import (
	"net/url"
	"strings"
	"testing"
)

func parse(t *testing.T, baseURL, htmlSrc string) *document {
	t.Helper()
	base, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("bad base %q: %v", baseURL, err)
	}
	return extractDocument(strings.NewReader(htmlSrc), base)
}

func TestExtractHead(t *testing.T) {
	d := parse(t, "https://example.com/a/b", `
<!doctype html>
<html lang="vi">
<head>
  <meta charset="utf-8">
  <title>  Trang   chủ | Acme </title>
  <meta name="description" content="Mô tả ngắn">
  <meta name="viewport" content="width=device-width">
  <meta name="robots" content="index, follow">
  <meta name="author" content="Kayn">
  <meta name="theme-color" content="#006fee">
  <meta property="og:title" content="OG title">
  <meta property="og:image" content="/og.png">
  <meta name="twitter:card" content="summary">
  <link rel="canonical" href="/a/b">
  <link rel="alternate" hreflang="en" href="https://example.com/en/a/b">
  <link rel="next" href="?page=2">
  <script type="application/ld+json">{"@type":"Article"}</script>
</head>
<body>
  <h1>Tiêu đề chính</h1>
  <h2>Mục một</h2><h2>Mục hai</h2>
  <h3>Nhỏ</h3>
  <p>Một hai ba bốn năm.</p>
</body>
</html>`)

	if got := first(d.Titles); got != "Trang chủ | Acme" {
		t.Errorf("title = %q, want whitespace-squashed %q", got, "Trang chủ | Acme")
	}
	if got := first(d.MetaDescs); got != "Mô tả ngắn" {
		t.Errorf("meta description = %q", got)
	}
	if d.Lang != "vi" {
		t.Errorf("lang = %q, want vi", d.Lang)
	}
	if d.Charset != "utf-8" {
		t.Errorf("charset = %q, want utf-8", d.Charset)
	}
	if d.Viewport == "" {
		t.Error("viewport not captured")
	}
	if d.MetaRobots != "index, follow" {
		t.Errorf("meta robots = %q", d.MetaRobots)
	}
	if d.Author != "Kayn" || d.ThemeColor != "#006fee" {
		t.Errorf("author/theme-color = %q / %q", d.Author, d.ThemeColor)
	}
	if d.OGTags["title"] != "OG title" {
		t.Errorf("og:title = %q", d.OGTags["title"])
	}
	// OG image is a meta property, so it is NOT absolutized — only link-ish
	// fields are. Assert the raw value so a future change is deliberate.
	if d.OGTags["image"] != "/og.png" {
		t.Errorf("og:image = %q", d.OGTags["image"])
	}
	if d.TwitterTags["card"] != "summary" {
		t.Errorf("twitter:card = %q", d.TwitterTags["card"])
	}
	if got := first(d.Canonicals); got != "https://example.com/a/b" {
		t.Errorf("canonical = %q, want absolutized", got)
	}
	if len(d.Hreflang) != 1 || d.Hreflang[0].Lang != "en" {
		t.Errorf("hreflang = %+v", d.Hreflang)
	}
	if d.RelNext != "https://example.com/a/b?page=2" {
		t.Errorf("rel=next = %q, want resolved against the page URL", d.RelNext)
	}
	if len(d.JSONLD) != 1 || !strings.Contains(d.JSONLD[0], "Article") {
		t.Errorf("json-ld = %v", d.JSONLD)
	}
	if len(d.H1) != 1 || d.H1[0] != "Tiêu đề chính" {
		t.Errorf("h1 = %v", d.H1)
	}
	if len(d.H2) != 2 {
		t.Errorf("h2 = %v, want 2", d.H2)
	}
	if len(d.H3) != 1 {
		t.Errorf("h3 = %v", d.H3)
	}
}

// TestExtractWordCountExcludesScripts is a deliberate divergence from
// LibreCrawl, which counts script and style bodies as page text. A bundled
// React app would read as tens of thousands of words.
func TestExtractWordCountExcludesScripts(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><body>
<p>one two three</p>
<script>var alpha = 1; function beta() { return gamma + delta + epsilon; }</script>
<style>.a { color: red } .b { color: blue }</style>
</body></html>`)
	if d.WordCount != 3 {
		t.Errorf("word count = %d, want 3 (script and style excluded)", d.WordCount)
	}
}

func TestExtractLinkPlacement(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><body>
  <header><a href="/logo">Logo</a></header>
  <nav><a href="/nav">Nav link</a></nav>
  <div class="site-menu"><a href="/menu">Menu link</a></div>
  <main><a href="/body">Body link</a></main>
  <footer><a href="/foot">Footer link</a></footer>
  <div id="page-footer"><a href="/foot2">Footer by id</a></div>
</body></html>`)

	want := map[string]int{
		"https://example.com/logo":  PlacementHeader,
		"https://example.com/nav":   PlacementNav,
		"https://example.com/menu":  PlacementNav,
		"https://example.com/body":  PlacementBody,
		"https://example.com/foot":  PlacementFooter,
		"https://example.com/foot2": PlacementFooter,
	}
	if len(d.Links) != len(want) {
		t.Fatalf("got %d links, want %d: %+v", len(d.Links), len(want), d.Links)
	}
	for _, l := range d.Links {
		if got, ok := want[l.Href]; !ok {
			t.Errorf("unexpected link %q", l.Href)
		} else if l.Placement != got {
			t.Errorf("%s placement = %d, want %d", l.Href, l.Placement, got)
		}
	}
}

// TestExtractStackSurvivesVoidElements guards the ancestor stack: pushing a
// void element (which never gets an end tag) leaves it permanently unbalanced,
// and every later link inherits a bogus placement.
func TestExtractStackSurvivesVoidElements(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><body>
  <footer><img src="/x.png"><br><input type="text"><a href="/in-footer">x</a></footer>
  <p><a href="/after">y</a></p>
</body></html>`)

	var inFooter, after int = -1, -1
	for _, l := range d.Links {
		switch l.Href {
		case "https://example.com/in-footer":
			inFooter = l.Placement
		case "https://example.com/after":
			after = l.Placement
		}
	}
	if inFooter != PlacementFooter {
		t.Errorf("link inside footer placement = %d, want footer", inFooter)
	}
	if after != PlacementBody {
		t.Errorf("link after the footer placement = %d, want body — the stack did not unwind", after)
	}
}

func TestExtractAnchorAndRel(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><body>
  <a href="/a" rel="NOFOLLOW ugc" target="_blank">  Đọc   thêm  </a>
  <a href="/b"><img src="/i.png" alt="Ảnh sản phẩm"></a>
  <a href="/c">` + strings.Repeat("x", 300) + `</a>
  <a href="#top">skip</a>
  <a href="mailto:a@b.c">skip</a>
  <a href="javascript:void(0)">skip</a>
</body></html>`)

	if len(d.Links) != 3 {
		t.Fatalf("got %d links, want 3 (fragment, mailto and javascript dropped): %+v", len(d.Links), d.Links)
	}
	if d.Links[0].Anchor != "Đọc thêm" {
		t.Errorf("anchor = %q, want whitespace-squashed", d.Links[0].Anchor)
	}
	if d.Links[0].Rel != "nofollow ugc" {
		t.Errorf("rel = %q, want lowercased token list", d.Links[0].Rel)
	}
	if d.Links[0].Target != "_blank" {
		t.Errorf("target = %q", d.Links[0].Target)
	}
	if d.Links[1].Anchor != "Ảnh sản phẩm" {
		t.Errorf("image-only anchor = %q, want the alt text", d.Links[1].Anchor)
	}
	if n := len([]rune(d.Links[2].Anchor)); n != maxAnchorLen {
		t.Errorf("long anchor kept %d runes, want %d", n, maxAnchorLen)
	}
}

func TestExtractBaseHrefChangesResolution(t *testing.T) {
	d := parse(t, "https://example.com/deep/page", `
<html><head><base href="https://cdn.example.net/root/"></head>
<body><a href="x">x</a><img src="y.png"></body></html>`)

	if len(d.Links) != 1 || d.Links[0].Href != "https://cdn.example.net/root/x" {
		t.Errorf("link = %+v, want resolved against <base href>", d.Links)
	}
	if len(d.Images) != 1 || d.Images[0].Src != "https://cdn.example.net/root/y.png" {
		t.Errorf("image = %+v, want resolved against <base href>", d.Images)
	}
}

func TestExtractImages(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><body>
  <img src="/a.png" alt="Có alt" width="100" height="50">
  <img src="/b.png" alt="">
  <img src="/c.png">
  <img data-src="/lazy.png" alt="Lazy">
  <img alt="no src at all">
</body></html>`)

	if len(d.Images) != 4 {
		t.Fatalf("got %d images, want 4 (the one with no src is dropped): %+v", len(d.Images), d.Images)
	}
	if d.Images[0].Width != "100" || d.Images[0].Height != "50" {
		t.Errorf("dimensions = %q x %q", d.Images[0].Width, d.Images[0].Height)
	}
	if d.Images[3].Src != "https://example.com/lazy.png" {
		t.Errorf("lazy image src = %q, want data-src picked up", d.Images[3].Src)
	}
}

func TestExtractAnalytics(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><head>
<script async src="https://www.googletagmanager.com/gtag/js?id=G-ABCDE12345"></script>
<script>window.dataLayer=[];function gtag(){dataLayer.push(arguments)}gtag('config','G-ABCDE12345');</script>
<script>(function(w,d,s,l,i){})(window,document,'script','dataLayer','GTM-ABC1234');</script>
<script>!function(f,b,e,v,n,t,s){}(window,document);fbq('init','123');</script>
</head><body>x</body></html>`)

	a := d.analytics()
	if a.GA4ID != "G-ABCDE12345" {
		t.Errorf("GA4 id = %q", a.GA4ID)
	}
	if a.GTMID != "GTM-ABC1234" {
		t.Errorf("GTM id = %q", a.GTMID)
	}
	if !a.Gtag || !a.GoogleAnalytics {
		t.Errorf("gtag/GA flags = %v / %v", a.Gtag, a.GoogleAnalytics)
	}
	if !a.FacebookPixel {
		t.Error("facebook pixel not detected")
	}
	if a.Hotjar || a.Mixpanel {
		t.Errorf("hotjar/mixpanel falsely detected: %v / %v", a.Hotjar, a.Mixpanel)
	}
}

func TestExtractCharsetFromHTTPEquiv(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><head><meta http-equiv="Content-Type" content="text/html; charset=windows-1258"></head><body>x</body></html>`)
	if d.Charset != "windows-1258" {
		t.Errorf("charset = %q, want windows-1258", d.Charset)
	}
}

func TestExtractMicrodata(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><body>
<div itemscope itemtype="https://schema.org/Product">
  <span itemprop="name">Bàn phím</span>
  <meta itemprop="sku" content="KB-1">
  <a itemprop="url" href="/p/1">link</a>
</div>
</body></html>`)

	if len(d.SchemaOrg) != 1 {
		t.Fatalf("got %d microdata items, want 1: %+v", len(d.SchemaOrg), d.SchemaOrg)
	}
	s := d.SchemaOrg[0]
	if s.Type != "https://schema.org/Product" {
		t.Errorf("itemtype = %q", s.Type)
	}
	if s.Properties["name"] != "Bàn phím" {
		t.Errorf("name = %q", s.Properties["name"])
	}
	if s.Properties["sku"] != "KB-1" {
		t.Errorf("sku = %q, want the meta content attribute", s.Properties["sku"])
	}
	if s.Properties["url"] != "/p/1" {
		t.Errorf("url = %q, want the raw href attribute", s.Properties["url"])
	}
}

func TestExtractMultipleTitlesAndDescriptions(t *testing.T) {
	d := parse(t, "https://example.com/", `
<html><head>
<title>First</title><title>Second</title>
<meta name="description" content="One">
<meta name="description" content="Two">
</head><body>x</body></html>`)

	if len(d.Titles) != 2 {
		t.Errorf("titles = %v, want both kept so the Multiple filter can flag them", d.Titles)
	}
	if len(d.MetaDescs) != 2 {
		t.Errorf("descriptions = %v, want both kept", d.MetaDescs)
	}
}

func TestExtractTruncatedDocument(t *testing.T) {
	// A byte cap cuts the document mid-element. The tokenizer must still yield
	// everything it did see rather than failing the page.
	d := parse(t, "https://example.com/", `
<html><head><title>Kept</title></head><body><footer><a href="/x">y</a`)
	if first(d.Titles) != "Kept" {
		t.Errorf("title = %q, want the head parsed before truncation", first(d.Titles))
	}
}

func TestExtractCanonicalProvenanceAndUncappedCount(t *testing.T) {
	t.Run("empty and whitespace canonical declarations are counted and raw values preserved", func(t *testing.T) {
		d := parse(t, "https://example.com/dir/page", `
<html><head>
<link rel="canonical" href="">
<link rel="canonical" href="   ">
<link rel="canonical">
<link rel="canonical" href="/valid">
</head><body>ok</body></html>`)

		if d.CanonicalTotalCount != 4 {
			t.Fatalf("CanonicalTotalCount = %d, want 4", d.CanonicalTotalCount)
		}
		wantRaw := []string{"", "   ", "", "/valid"}
		if len(d.RawCanonicals) != len(wantRaw) {
			t.Fatalf("RawCanonicals len = %d, want %d (%v)", len(d.RawCanonicals), len(wantRaw), d.RawCanonicals)
		}
		for i, w := range wantRaw {
			if d.RawCanonicals[i] != w {
				t.Errorf("RawCanonicals[%d] = %q, want %q", i, d.RawCanonicals[i], w)
			}
		}
		// Backward compatibility: Canonicals only stores non-empty post-absolutized URLs
		if len(d.Canonicals) != 1 || d.Canonicals[0] != "https://example.com/valid" {
			t.Errorf("Canonicals = %v, want [https://example.com/valid]", d.Canonicals)
		}
	})

	t.Run("more than 5 canonical declarations tracks uncapped total while capping stored samples at 5", func(t *testing.T) {
		d := parse(t, "https://example.com/", `
<html><head>
<link rel="canonical" href="/c1">
<link rel="canonical" href="">
<link rel="canonical" href="/c3">
<link rel="canonical" href="/c4">
<link rel="canonical" href="/c5">
<link rel="canonical" href="/c6">
<link rel="canonical" href="/c7">
</head><body>ok</body></html>`)

		if d.CanonicalTotalCount != 7 {
			t.Fatalf("CanonicalTotalCount = %d, want 7", d.CanonicalTotalCount)
		}
		if len(d.RawCanonicals) != 5 {
			t.Fatalf("len(RawCanonicals) = %d, want 5", len(d.RawCanonicals))
		}
		wantRaw := []string{"/c1", "", "/c3", "/c4", "/c5"}
		for i, w := range wantRaw {
			if d.RawCanonicals[i] != w {
				t.Errorf("RawCanonicals[%d] = %q, want %q", i, d.RawCanonicals[i], w)
			}
		}
		// Canonicals stores up to 5 non-empty resolved URLs
		if len(d.Canonicals) != 5 {
			t.Fatalf("len(Canonicals) = %d, want 5", len(d.Canonicals))
		}
	})

	t.Run("base href raw and count are preserved accurately", func(t *testing.T) {
		d := parse(t, "https://example.com/dir/page", `
<html><head>
<base target="_blank">
<base href="https://cdn.example.com/base/">
<base href="https://other.example.com/second/">
<link rel="canonical" href="sub.html">
</head><body>ok</body></html>`)

		if d.BaseHrefCount != 2 {
			t.Errorf("BaseHrefCount = %d, want 2 (only <base> with href counted)", d.BaseHrefCount)
		}
		if d.BaseHrefRaw != "https://cdn.example.com/base/" {
			t.Errorf("BaseHrefRaw = %q, want first <base href>", d.BaseHrefRaw)
		}
		if len(d.RawCanonicals) != 1 || d.RawCanonicals[0] != "sub.html" {
			t.Errorf("RawCanonicals = %v, want [sub.html]", d.RawCanonicals)
		}
	})
}
