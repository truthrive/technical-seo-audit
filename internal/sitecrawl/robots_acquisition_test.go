package sitecrawl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// Helper to create a test response with custom headers and status.
func newTestResponseWithHeaders(finalURL string, status int, contentType string, headers map[string][]string, body string) *http.Response {
	reqURL, _ := url.Parse(finalURL)
	h := make(http.Header)
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	for k, vals := range headers {
		for _, v := range vals {
			h.Add(k, v)
		}
	}
	return &http.Response{
		StatusCode:    status,
		Header:        h,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       &http.Request{URL: reqURL},
	}
}

// 1. Single robots meta declaration.
func TestRobotsMetaAcquisition_SingleDeclaration(t *testing.T) {
	html := `<!doctype html><html><head><meta name="robots" content="noindex, nofollow"></head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if d.RobotsMetaTotalCount != 1 {
		t.Fatalf("RobotsMetaTotalCount = %d, want 1", d.RobotsMetaTotalCount)
	}
	if len(d.RawRobotsMeta) != 1 {
		t.Fatalf("len(RawRobotsMeta) = %d, want 1", len(d.RawRobotsMeta))
	}
	m := d.RawRobotsMeta[0]
	if m.Seq != 0 || m.Name != "robots" || m.NameRaw != "robots" || m.ContentRaw != "noindex, nofollow" || !m.HasContentAttr {
		t.Errorf("RawRobotsMeta[0] = %+v, unexpected fields", m)
	}

	p := buildPage(frontierItem{URL: "https://example.com/"}, fetched{Status: 200, Doc: d}, "example.com", Options{})
	if !p.RobotsMetaExtracted {
		t.Errorf("p.RobotsMetaExtracted = false, want true")
	}
	if p.RobotsMetaTotalCount != 1 || len(p.RawRobotsMeta) != 1 {
		t.Errorf("Page counts mismatch: Total=%d, len=%d", p.RobotsMetaTotalCount, len(p.RawRobotsMeta))
	}
}

// 2. Multiple same-name robots meta declarations.
func TestRobotsMetaAcquisition_MultipleSameName(t *testing.T) {
	html := `<!doctype html><html><head>
		<meta name="robots" content="noindex">
		<meta name="robots" content="nofollow">
	</head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if d.RobotsMetaTotalCount != 2 {
		t.Fatalf("RobotsMetaTotalCount = %d, want 2", d.RobotsMetaTotalCount)
	}
	if len(d.RawRobotsMeta) != 2 {
		t.Fatalf("len(RawRobotsMeta) = %d, want 2", len(d.RawRobotsMeta))
	}
	if d.RawRobotsMeta[0].Seq != 0 || d.RawRobotsMeta[0].ContentRaw != "noindex" {
		t.Errorf("RawRobotsMeta[0] = %+v, want seq 0, content noindex", d.RawRobotsMeta[0])
	}
	if d.RawRobotsMeta[1].Seq != 1 || d.RawRobotsMeta[1].ContentRaw != "nofollow" {
		t.Errorf("RawRobotsMeta[1] = %+v, want seq 1, content nofollow", d.RawRobotsMeta[1])
	}

	p := buildPage(frontierItem{URL: "https://example.com/"}, fetched{Status: 200, Doc: d}, "example.com", Options{})
	// Backward compatibility checks
	if p.MetaRobots != "noindex, nofollow" {
		t.Errorf("p.MetaRobots = %q, want 'noindex, nofollow'", p.MetaRobots)
	}
	if p.MetaTags["robots"] != "nofollow" {
		t.Errorf("p.MetaTags['robots'] = %q, want 'nofollow'", p.MetaTags["robots"])
	}
}

// 3. Multiple crawler-specific meta declarations.
func TestRobotsMetaAcquisition_MultipleCrawlerSpecific(t *testing.T) {
	html := `<!doctype html><html><head>
		<meta name="robots" content="all">
		<meta name="googlebot" content="noindex">
		<meta name="bingbot" content="noarchive">
		<meta name="OAI-SearchBot" content="nosnippet">
		<meta name="author" content="Not a crawler">
	</head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if d.RobotsMetaTotalCount != 4 {
		t.Fatalf("RobotsMetaTotalCount = %d, want 4", d.RobotsMetaTotalCount)
	}
	if len(d.RawRobotsMeta) != 4 {
		t.Fatalf("len(RawRobotsMeta) = %d, want 4", len(d.RawRobotsMeta))
	}
	expected := []struct {
		seq        int
		name       string
		nameRaw    string
		contentRaw string
	}{
		{0, "robots", "robots", "all"},
		{1, "googlebot", "googlebot", "noindex"},
		{2, "bingbot", "bingbot", "noarchive"},
		{3, "oai-searchbot", "OAI-SearchBot", "nosnippet"},
	}
	for i, exp := range expected {
		got := d.RawRobotsMeta[i]
		if got.Seq != exp.seq || got.Name != exp.name || got.NameRaw != exp.nameRaw || got.ContentRaw != exp.contentRaw || !got.HasContentAttr {
			t.Errorf("RawRobotsMeta[%d] = %+v, want %+v", i, got, exp)
		}
	}
}

// 4. Empty content="".
func TestRobotsMetaAcquisition_EmptyContent(t *testing.T) {
	html := `<!doctype html><html><head><meta name="robots" content=""></head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if d.RobotsMetaTotalCount != 1 || len(d.RawRobotsMeta) != 1 {
		t.Fatalf("counts: total=%d, len=%d, want 1", d.RobotsMetaTotalCount, len(d.RawRobotsMeta))
	}
	m := d.RawRobotsMeta[0]
	if m.ContentRaw != "" || !m.HasContentAttr {
		t.Errorf("RawRobotsMeta[0] = %+v, want ContentRaw='', HasContentAttr=true", m)
	}
}

// 5. Whitespace-only content="   ".
func TestRobotsMetaAcquisition_WhitespaceOnlyContent(t *testing.T) {
	html := `<!doctype html><html><head><meta name="robots" content="   "></head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if d.RobotsMetaTotalCount != 1 || len(d.RawRobotsMeta) != 1 {
		t.Fatalf("counts: total=%d, len=%d, want 1", d.RobotsMetaTotalCount, len(d.RawRobotsMeta))
	}
	m := d.RawRobotsMeta[0]
	if m.ContentRaw != "   " || !m.HasContentAttr {
		t.Errorf("RawRobotsMeta[0] = %+v, want ContentRaw='   ', HasContentAttr=true", m)
	}
}

// 6. Missing content attribute.
func TestRobotsMetaAcquisition_MissingContentAttribute(t *testing.T) {
	html := `<!doctype html><html><head><meta name="robots"></head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if d.RobotsMetaTotalCount != 1 || len(d.RawRobotsMeta) != 1 {
		t.Fatalf("counts: total=%d, len=%d, want 1", d.RobotsMetaTotalCount, len(d.RawRobotsMeta))
	}
	m := d.RawRobotsMeta[0]
	if m.ContentRaw != "" || m.HasContentAttr {
		t.Errorf("RawRobotsMeta[0] = %+v, want ContentRaw='', HasContentAttr=false", m)
	}
}

// 7. Raw content preservation before trimming.
func TestRobotsMetaAcquisition_RawContentPreservationBeforeTrimming(t *testing.T) {
	html := `<!doctype html><html><head><meta name="robots" content="  noindex,   nofollow  "></head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if len(d.RawRobotsMeta) != 1 {
		t.Fatalf("len(RawRobotsMeta) = %d, want 1", len(d.RawRobotsMeta))
	}
	m := d.RawRobotsMeta[0]
	if m.ContentRaw != "  noindex,   nofollow  " {
		t.Errorf("ContentRaw = %q, want exact untrimmed '  noindex,   nofollow  '", m.ContentRaw)
	}
}

// 8. Document-order Seq preservation.
func TestRobotsMetaAcquisition_DocumentOrderSeqPreservation(t *testing.T) {
	html := `<!doctype html><html><head>
		<meta name="googlebot" content="first">
		<meta name="robots" content="second">
		<meta name="bingbot" content="third">
	</head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if len(d.RawRobotsMeta) != 3 {
		t.Fatalf("len(RawRobotsMeta) = %d, want 3", len(d.RawRobotsMeta))
	}
	for i := 0; i < 3; i++ {
		if d.RawRobotsMeta[i].Seq != i {
			t.Errorf("RawRobotsMeta[%d].Seq = %d, want %d", i, d.RawRobotsMeta[i].Seq, i)
		}
	}
	if d.RawRobotsMeta[0].Name != "googlebot" || d.RawRobotsMeta[1].Name != "robots" || d.RawRobotsMeta[2].Name != "bingbot" {
		t.Errorf("Unexpected declaration ordering: %+v", d.RawRobotsMeta)
	}
}

// 9. Exactly 10 declarations.
func TestRobotsMetaAcquisition_Exactly10Declarations(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<!doctype html><html><head>")
	for i := 0; i < 10; i++ {
		sb.WriteString(fmt.Sprintf(`<meta name="robots" content="rule%d">`, i))
	}
	sb.WriteString("</head><body>ok</body></html>")
	d := parse(t, "https://example.com/", sb.String())

	if d.RobotsMetaTotalCount != 10 {
		t.Errorf("RobotsMetaTotalCount = %d, want 10", d.RobotsMetaTotalCount)
	}
	if len(d.RawRobotsMeta) != 10 {
		t.Fatalf("len(RawRobotsMeta) = %d, want 10", len(d.RawRobotsMeta))
	}
	for i := 0; i < 10; i++ {
		if d.RawRobotsMeta[i].Seq != i || d.RawRobotsMeta[i].ContentRaw != fmt.Sprintf("rule%d", i) {
			t.Errorf("RawRobotsMeta[%d] = %+v mismatch", i, d.RawRobotsMeta[i])
		}
	}
}

// 10. More than 10 declarations with uncapped total count.
func TestRobotsMetaAcquisition_MoreThan10Declarations_UncappedTotal(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("<!doctype html><html><head>")
	for i := 0; i < 15; i++ {
		sb.WriteString(fmt.Sprintf(`<meta name="robots" content="tag%d">`, i))
	}
	sb.WriteString("</head><body>ok</body></html>")
	d := parse(t, "https://example.com/", sb.String())

	if d.RobotsMetaTotalCount != 15 {
		t.Errorf("RobotsMetaTotalCount = %d, want 15 (uncapped)", d.RobotsMetaTotalCount)
	}
	if len(d.RawRobotsMeta) != 10 {
		t.Fatalf("len(RawRobotsMeta) = %d, want 10 (sample capped)", len(d.RawRobotsMeta))
	}
	if d.RawRobotsMeta[9].Seq != 9 || d.RawRobotsMeta[9].ContentRaw != "tag9" {
		t.Errorf("RawRobotsMeta[9] = %+v mismatch", d.RawRobotsMeta[9])
	}
}

// 11. No robots meta declarations after complete HTML extraction.
func TestRobotsMetaAcquisition_NoDeclarations_CompleteExtraction(t *testing.T) {
	html := `<!doctype html><html><head><title>Clean page</title><meta name="description" content="desc"></head><body>ok</body></html>`
	d := parse(t, "https://example.com/", html)

	if d.RobotsMetaTotalCount != 0 {
		t.Errorf("RobotsMetaTotalCount = %d, want 0", d.RobotsMetaTotalCount)
	}
	if len(d.RawRobotsMeta) != 0 {
		t.Errorf("len(RawRobotsMeta) = %d, want 0", len(d.RawRobotsMeta))
	}

	p := buildPage(frontierItem{URL: "https://example.com/"}, fetched{Status: 200, Doc: d}, "example.com", Options{})
	if !p.RobotsMetaExtracted {
		t.Errorf("p.RobotsMetaExtracted = false, want true (complete extraction proven)")
	}
	if p.RobotsMetaTotalCount != 0 {
		t.Errorf("p.RobotsMetaTotalCount = %d, want 0", p.RobotsMetaTotalCount)
	}
	if len(p.RawRobotsMeta) != 0 {
		t.Errorf("p.RawRobotsMeta = %+v, want empty", p.RawRobotsMeta)
	}
}

// 12. Legacy Page JSON missing new fields.
func TestRobotsMetaAcquisition_LegacyPageJSON(t *testing.T) {
	legacyJSON := `{
		"url": "https://example.com/legacy",
		"status": 200,
		"metaRobots": "noindex, follow",
		"metaTags": {"robots": "noindex, follow"},
		"xRobotsTag": "googlebot: noarchive"
	}`
	var p Page
	if err := json.Unmarshal([]byte(legacyJSON), &p); err != nil {
		t.Fatalf("failed to unmarshal legacy JSON: %v", err)
	}

	if p.RobotsMetaExtracted {
		t.Errorf("RobotsMetaExtracted = true on legacy row, want false")
	}
	if p.RobotsMetaTotalCount != 0 {
		t.Errorf("RobotsMetaTotalCount = %d on legacy row, want 0", p.RobotsMetaTotalCount)
	}
	if p.RawRobotsMeta != nil {
		t.Errorf("RawRobotsMeta = %+v on legacy row, want nil", p.RawRobotsMeta)
	}
	if p.XRobotsTagTotalCount != 0 {
		t.Errorf("XRobotsTagTotalCount = %d on legacy row, want 0", p.XRobotsTagTotalCount)
	}
	if p.RawXRobotsTags != nil {
		t.Errorf("RawXRobotsTags = %+v on legacy row, want nil", p.RawXRobotsTags)
	}

	// Verify existing legacy fields remain fully accessible
	if p.MetaRobots != "noindex, follow" {
		t.Errorf("p.MetaRobots = %q, want 'noindex, follow'", p.MetaRobots)
	}
	if p.XRobotsTag != "googlebot: noarchive" {
		t.Errorf("p.XRobotsTag = %q, want 'googlebot: noarchive'", p.XRobotsTag)
	}
}

// 13. Single X-Robots-Tag header.
func TestXRobotsAcquisition_SingleHeader(t *testing.T) {
	f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
	res := newTestResponseWithHeaders("https://example.com/a", 200, "text/html", map[string][]string{
		"X-Robots-Tag": {"noindex"},
	}, "<html><body>ok</body></html>")
	out := fetched{FinalURL: "https://example.com/a", Status: 200}

	f.readTerminal(res, &out, true)

	if out.XRobotsTagTotalCount != 1 {
		t.Fatalf("XRobotsTagTotalCount = %d, want 1", out.XRobotsTagTotalCount)
	}
	if len(out.RawXRobotsTags) != 1 || out.RawXRobotsTags[0] != "noindex" {
		t.Errorf("RawXRobotsTags = %v, want ['noindex']", out.RawXRobotsTags)
	}
	if out.XRobotsTag != "noindex" {
		t.Errorf("out.XRobotsTag = %q, want 'noindex'", out.XRobotsTag)
	}

	p := buildPage(frontierItem{URL: "https://example.com/a"}, out, "example.com", Options{})
	if p.XRobotsTagTotalCount != 1 || len(p.RawXRobotsTags) != 1 || p.RawXRobotsTags[0] != "noindex" {
		t.Errorf("Page XRobots mismatch: total=%d, raw=%v", p.XRobotsTagTotalCount, p.RawXRobotsTags)
	}
}

// 14. Multiple X-Robots-Tag headers.
func TestXRobotsAcquisition_MultipleHeaders(t *testing.T) {
	f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
	res := newTestResponseWithHeaders("https://example.com/m", 200, "text/html", map[string][]string{
		"X-Robots-Tag": {"googlebot: noindex", "nofollow"},
	}, "<html><body>ok</body></html>")
	out := fetched{FinalURL: "https://example.com/m", Status: 200}

	f.readTerminal(res, &out, true)

	if out.XRobotsTagTotalCount != 2 {
		t.Fatalf("XRobotsTagTotalCount = %d, want 2", out.XRobotsTagTotalCount)
	}
	if len(out.RawXRobotsTags) != 2 {
		t.Fatalf("len(RawXRobotsTags) = %d, want 2", len(out.RawXRobotsTags))
	}
	if out.RawXRobotsTags[0] != "googlebot: noindex" || out.RawXRobotsTags[1] != "nofollow" {
		t.Errorf("RawXRobotsTags = %v, want ['googlebot: noindex', 'nofollow']", out.RawXRobotsTags)
	}
	// Backward compatibility comma-joined string
	if out.XRobotsTag != "googlebot: noindex, nofollow" {
		t.Errorf("out.XRobotsTag = %q, want 'googlebot: noindex, nofollow'", out.XRobotsTag)
	}
}

// 15. Exactly 10 X-Robots-Tag headers.
func TestXRobotsAcquisition_Exactly10Headers(t *testing.T) {
	f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
	var headers []string
	for i := 0; i < 10; i++ {
		headers = append(headers, fmt.Sprintf("rule-%d", i))
	}
	res := newTestResponseWithHeaders("https://example.com/10h", 200, "text/html", map[string][]string{
		"X-Robots-Tag": headers,
	}, "<html><body>ok</body></html>")
	out := fetched{FinalURL: "https://example.com/10h", Status: 200}

	f.readTerminal(res, &out, true)

	if out.XRobotsTagTotalCount != 10 {
		t.Errorf("XRobotsTagTotalCount = %d, want 10", out.XRobotsTagTotalCount)
	}
	if len(out.RawXRobotsTags) != 10 {
		t.Fatalf("len(RawXRobotsTags) = %d, want 10", len(out.RawXRobotsTags))
	}
	for i := 0; i < 10; i++ {
		if out.RawXRobotsTags[i] != fmt.Sprintf("rule-%d", i) {
			t.Errorf("RawXRobotsTags[%d] = %q mismatch", i, out.RawXRobotsTags[i])
		}
	}
}

// 16. More than 10 headers with uncapped count.
func TestXRobotsAcquisition_MoreThan10Headers_UncappedCount(t *testing.T) {
	f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
	var headers []string
	for i := 0; i < 14; i++ {
		headers = append(headers, fmt.Sprintf("header-%d", i))
	}
	res := newTestResponseWithHeaders("https://example.com/14h", 200, "text/html", map[string][]string{
		"X-Robots-Tag": headers,
	}, "<html><body>ok</body></html>")
	out := fetched{FinalURL: "https://example.com/14h", Status: 200}

	f.readTerminal(res, &out, true)

	if out.XRobotsTagTotalCount != 14 {
		t.Errorf("XRobotsTagTotalCount = %d, want 14 (uncapped)", out.XRobotsTagTotalCount)
	}
	if len(out.RawXRobotsTags) != 10 {
		t.Fatalf("len(RawXRobotsTags) = %d, want 10 (sample capped)", len(out.RawXRobotsTags))
	}
	if out.RawXRobotsTags[9] != "header-9" {
		t.Errorf("RawXRobotsTags[9] = %q, want 'header-9'", out.RawXRobotsTags[9])
	}
	// Backward compatibility field contains all joined headers
	if !strings.HasPrefix(out.XRobotsTag, "header-0, header-1") || !strings.HasSuffix(out.XRobotsTag, "header-13") {
		t.Errorf("out.XRobotsTag = %q, expected all 14 joined", out.XRobotsTag)
	}
}

// 17. Single-hop redirect with initial-header evidence.
func TestXRobotsAcquisition_SingleHopRedirect_InitialHeaderEvidence(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/source":
			w.Header().Add("X-Robots-Tag", "initial-redirect-tag")
			w.Header().Set("Location", "/destination")
			w.WriteHeader(http.StatusMovedPermanently) // 301
			w.Write([]byte("redirecting"))
		case "/destination":
			w.Header().Add("X-Robots-Tag", "terminal-destination-tag")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>terminal</body></html>"))
		}
	}))
	defer ts.Close()

	opts := Options{FollowRedirects: true}.normalized()
	f := newFetcher(opts, nil, "127.0.0.1", true)

	res := f.fetch(context.Background(), ts.URL+"/source", true)

	if res.Status != 301 {
		t.Fatalf("res.Status = %d, want 301", res.Status)
	}
	if len(res.Hops) != 1 {
		t.Fatalf("len(res.Hops) = %d, want 1", len(res.Hops))
	}
	// Assert source row preserves its OWN initial headers, NOT destination's headers!
	if res.XRobotsTagTotalCount != 1 {
		t.Errorf("res.XRobotsTagTotalCount = %d, want 1", res.XRobotsTagTotalCount)
	}
	if len(res.RawXRobotsTags) != 1 || res.RawXRobotsTags[0] != "initial-redirect-tag" {
		t.Errorf("res.RawXRobotsTags = %v, want ['initial-redirect-tag']", res.RawXRobotsTags)
	}
	if res.XRobotsTag != "initial-redirect-tag" {
		t.Errorf("res.XRobotsTag = %q, want 'initial-redirect-tag'", res.XRobotsTag)
	}

	p := buildPage(frontierItem{URL: ts.URL + "/source"}, res, "127.0.0.1", opts)
	if p.XRobotsTag != "initial-redirect-tag" {
		t.Errorf("p.XRobotsTag = %q, want 'initial-redirect-tag'", p.XRobotsTag)
	}
	if p.XRobotsTagTotalCount != 1 || len(p.RawXRobotsTags) != 1 || p.RawXRobotsTags[0] != "initial-redirect-tag" {
		t.Errorf("Page XRobots mismatch on 301 row: total=%d, raw=%v", p.XRobotsTagTotalCount, p.RawXRobotsTags)
	}
}

// 18. Multi-hop redirect with different initial and terminal headers.
func TestXRobotsAcquisition_MultiHopRedirect_DifferentInitialAndTerminal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/hop1":
			w.Header().Add("X-Robots-Tag", "hop1-rule")
			w.Header().Set("Location", "/hop2")
			w.WriteHeader(http.StatusMovedPermanently) // 301
		case "/hop2":
			w.Header().Add("X-Robots-Tag", "hop2-rule")
			w.Header().Set("Location", "/dest")
			w.WriteHeader(http.StatusFound) // 302
		case "/dest":
			w.Header().Add("X-Robots-Tag", "dest-rule")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>terminal</body></html>"))
		}
	}))
	defer ts.Close()

	opts := Options{FollowRedirects: true}.normalized()
	f := newFetcher(opts, nil, "127.0.0.1", true)

	res := f.fetch(context.Background(), ts.URL+"/hop1", true)

	if res.Status != 301 {
		t.Fatalf("res.Status = %d, want 301", res.Status)
	}
	if len(res.Hops) != 2 {
		t.Fatalf("len(res.Hops) = %d, want 2", len(res.Hops))
	}
	if res.XRobotsTagTotalCount != 1 || len(res.RawXRobotsTags) != 1 || res.RawXRobotsTags[0] != "hop1-rule" {
		t.Errorf("res.RawXRobotsTags = %v, want ['hop1-rule']", res.RawXRobotsTags)
	}
	if res.XRobotsTag != "hop1-rule" {
		t.Errorf("res.XRobotsTag = %q, want 'hop1-rule'", res.XRobotsTag)
	}
}

// 19. Redirect without initial X-Robots-Tag.
func TestXRobotsAcquisition_RedirectWithoutInitialXRobots(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/clean-redirect":
			// No X-Robots-Tag on redirect!
			w.Header().Set("Location", "/tagged-dest")
			w.WriteHeader(http.StatusMovedPermanently)
		case "/tagged-dest":
			w.Header().Add("X-Robots-Tag", "terminal-noindex")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body>terminal</body></html>"))
		}
	}))
	defer ts.Close()

	opts := Options{FollowRedirects: true}.normalized()
	f := newFetcher(opts, nil, "127.0.0.1", true)

	res := f.fetch(context.Background(), ts.URL+"/clean-redirect", true)

	if res.Status != 301 {
		t.Fatalf("res.Status = %d, want 301", res.Status)
	}
	if res.XRobotsTagTotalCount != 0 {
		t.Errorf("res.XRobotsTagTotalCount = %d, want 0", res.XRobotsTagTotalCount)
	}
	if len(res.RawXRobotsTags) != 0 {
		t.Errorf("res.RawXRobotsTags = %v, want empty/nil", res.RawXRobotsTags)
	}
	if res.XRobotsTag != "" {
		t.Errorf("res.XRobotsTag = %q, want empty (must not inherit terminal tag)", res.XRobotsTag)
	}
}

// 20. Non-HTML response with X-Robots-Tag.
func TestXRobotsAcquisition_NonHTMLResponse(t *testing.T) {
	f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
	res := newTestResponseWithHeaders("https://example.com/file.pdf", 200, "application/pdf", map[string][]string{
		"X-Robots-Tag": {"noindex, noarchive"},
	}, "%PDF-1.4 ...")
	out := fetched{FinalURL: "https://example.com/file.pdf", Status: 200}

	f.readTerminal(res, &out, false)

	if out.XRobotsTagTotalCount != 1 {
		t.Fatalf("XRobotsTagTotalCount = %d, want 1", out.XRobotsTagTotalCount)
	}
	if len(out.RawXRobotsTags) != 1 || out.RawXRobotsTags[0] != "noindex, noarchive" {
		t.Errorf("RawXRobotsTags = %v, want ['noindex, noarchive']", out.RawXRobotsTags)
	}

	p := buildPage(frontierItem{URL: "https://example.com/file.pdf"}, out, "example.com", Options{})
	if p.RobotsMetaExtracted {
		t.Errorf("p.RobotsMetaExtracted = true on PDF, want false")
	}
	if p.XRobotsTagTotalCount != 1 || len(p.RawXRobotsTags) != 1 || p.RawXRobotsTags[0] != "noindex, noarchive" {
		t.Errorf("Page XRobots mismatch on PDF: total=%d, raw=%v", p.XRobotsTagTotalCount, p.RawXRobotsTags)
	}
}

// 21. JSON persistence round trip.
func TestRobotsAcquisition_JSONPersistenceRoundTrip(t *testing.T) {
	orig := Page{
		URL:                  "https://example.com/roundtrip",
		Status:               200,
		ContentType:          "text/html",
		RobotsMetaExtracted:  true,
		RobotsMetaTotalCount: 3,
		RawRobotsMeta: []RawMetaDirective{
			{Seq: 0, Name: "robots", NameRaw: "robots", ContentRaw: "noindex", HasContentAttr: true},
			{Seq: 1, Name: "googlebot", NameRaw: "Googlebot", ContentRaw: "  noarchive  ", HasContentAttr: true},
			{Seq: 2, Name: "bingbot", NameRaw: "bingbot", ContentRaw: "", HasContentAttr: false},
		},
		XRobotsTag:           "googlebot: noindex, nofollow",
		XRobotsTagTotalCount: 2,
		RawXRobotsTags:       []string{"googlebot: noindex", "nofollow"},
	}

	blob, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var roundtrip Page
	if err := json.Unmarshal(blob, &roundtrip); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if roundtrip.RobotsMetaExtracted != orig.RobotsMetaExtracted {
		t.Errorf("RobotsMetaExtracted = %v, want %v", roundtrip.RobotsMetaExtracted, orig.RobotsMetaExtracted)
	}
	if roundtrip.RobotsMetaTotalCount != orig.RobotsMetaTotalCount {
		t.Errorf("RobotsMetaTotalCount = %d, want %d", roundtrip.RobotsMetaTotalCount, orig.RobotsMetaTotalCount)
	}
	if len(roundtrip.RawRobotsMeta) != len(orig.RawRobotsMeta) {
		t.Fatalf("len(RawRobotsMeta) = %d, want %d", len(roundtrip.RawRobotsMeta), len(orig.RawRobotsMeta))
	}
	for i := range orig.RawRobotsMeta {
		if roundtrip.RawRobotsMeta[i] != orig.RawRobotsMeta[i] {
			t.Errorf("RawRobotsMeta[%d] = %+v, want %+v", i, roundtrip.RawRobotsMeta[i], orig.RawRobotsMeta[i])
		}
	}
	if roundtrip.XRobotsTagTotalCount != orig.XRobotsTagTotalCount {
		t.Errorf("XRobotsTagTotalCount = %d, want %d", roundtrip.XRobotsTagTotalCount, orig.XRobotsTagTotalCount)
	}
	if len(roundtrip.RawXRobotsTags) != len(orig.RawXRobotsTags) {
		t.Fatalf("len(RawXRobotsTags) = %d, want %d", len(roundtrip.RawXRobotsTags), len(orig.RawXRobotsTags))
	}
	for i := range orig.RawXRobotsTags {
		if roundtrip.RawXRobotsTags[i] != orig.RawXRobotsTags[i] {
			t.Errorf("RawXRobotsTags[%d] = %q, want %q", i, roundtrip.RawXRobotsTags[i], orig.RawXRobotsTags[i])
		}
	}
}

// 22. Existing MetaTags/MetaRobots/XRobotsTag backward compatibility.
func TestRobotsAcquisition_BackwardCompatibility(t *testing.T) {
	html := `<!doctype html><html><head>
		<meta name="robots" content="index, follow">
		<meta name="googlebot" content="noindex">
	</head><body>ok</body></html>`
	d := parse(t, "https://example.com/bwc", html)

	if d.MetaRobots != "index, follow, noindex" {
		t.Errorf("d.MetaRobots = %q, want 'index, follow, noindex'", d.MetaRobots)
	}
	if d.MetaTags["robots"] != "index, follow" {
		t.Errorf("d.MetaTags['robots'] = %q, want 'index, follow'", d.MetaTags["robots"])
	}
	if d.MetaTags["googlebot"] != "noindex" {
		t.Errorf("d.MetaTags['googlebot'] = %q, want 'noindex'", d.MetaTags["googlebot"])
	}

	p := buildPage(frontierItem{URL: "https://example.com/bwc"}, fetched{Status: 200, Doc: d, XRobotsTag: "noarchive"}, "example.com", Options{})
	if p.MetaRobots != "index, follow, noindex" {
		t.Errorf("p.MetaRobots = %q, want 'index, follow, noindex'", p.MetaRobots)
	}
	if p.XRobotsTag != "noarchive" {
		t.Errorf("p.XRobotsTag = %q, want 'noarchive'", p.XRobotsTag)
	}
}

// 23. Existing canonical extraction regression.
func TestRobotsAcquisition_ExistingCanonicalExtractionRegression(t *testing.T) {
	html := `<!doctype html><html><head>
		<base href="https://example.com/base/">
		<link rel="canonical" href="target-1">
		<link rel="canonical" href="target-2">
		<meta name="robots" content="noindex">
	</head><body>ok</body></html>`
	d := parse(t, "https://example.com/page", html)

	if d.CanonicalTotalCount != 2 {
		t.Errorf("CanonicalTotalCount = %d, want 2", d.CanonicalTotalCount)
	}
	if len(d.RawCanonicals) != 2 {
		t.Errorf("len(RawCanonicals) = %d, want 2", len(d.RawCanonicals))
	}
	if d.BaseHrefCount != 1 || d.BaseHrefRaw != "https://example.com/base/" {
		t.Errorf("BaseHref = %q (count %d)", d.BaseHrefRaw, d.BaseHrefCount)
	}
	// Verify robots directive did not interfere with canonical extraction
	if d.RobotsMetaTotalCount != 1 || len(d.RawRobotsMeta) != 1 {
		t.Errorf("RobotsMeta = %+v", d.RawRobotsMeta)
	}
}

// 24. Stream truncated at size limit preserves pre-truncation meta directives and flags truncated status.
func TestRobotsAcquisition_StreamTruncatedSizeLimit_PreservesPreTruncationEvidence(t *testing.T) {
	f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
	limit := int(f.opts.maxFileSize())
	prefix := `<!doctype html><html><head><meta name="robots" content="noindex, nofollow"></head><body>`
	suffix := `</body></html>`
	overBody := prefix + strings.Repeat("x", limit-len(prefix)-len(suffix)+1) + suffix

	res := newTestHTMLResponse("https://example.com/trunc-robots", 200, -1, strings.NewReader(overBody))
	out := fetched{FinalURL: "https://example.com/trunc-robots", Status: 200}

	f.readTerminal(res, &out, true)

	if out.ExtractionStatus != ExtractionStatusTruncatedSizeLimit {
		t.Fatalf("out.ExtractionStatus = %q, want %q", out.ExtractionStatus, ExtractionStatusTruncatedSizeLimit)
	}

	p := buildPage(frontierItem{URL: "https://example.com/trunc-robots"}, out, "example.com", Options{})
	if p.ExtractionStatus != ExtractionStatusTruncatedSizeLimit {
		t.Errorf("p.ExtractionStatus = %q, want %q", p.ExtractionStatus, ExtractionStatusTruncatedSizeLimit)
	}
	if !p.RobotsMetaExtracted {
		t.Errorf("p.RobotsMetaExtracted = false, want true (extractor ran on stream prefix)")
	}
	if p.RobotsMetaTotalCount != 1 {
		t.Errorf("p.RobotsMetaTotalCount = %d, want 1", p.RobotsMetaTotalCount)
	}
	if len(p.RawRobotsMeta) != 1 {
		t.Fatalf("len(p.RawRobotsMeta) = %d, want 1", len(p.RawRobotsMeta))
	}
	if p.RawRobotsMeta[0].Name != "robots" || p.RawRobotsMeta[0].ContentRaw != "noindex, nofollow" {
		t.Errorf("p.RawRobotsMeta[0] = %+v, unexpected content", p.RawRobotsMeta[0])
	}
}

// 25. Pre-parse Content-Length > cap skips body but preserves HTTP header X-Robots-Tag with truncated status.
func TestRobotsAcquisition_ContentLengthExceedsCap_PreservesHeadersWithTruncatedStatus(t *testing.T) {
	f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
	capBytes := f.opts.maxFileSize()
	res := newTestResponseWithHeaders("https://example.com/huge-robots", 200, "text/html", map[string][]string{
		"X-Robots-Tag": {"noarchive"},
	}, "<html><head><meta name=\"robots\" content=\"noindex\"></head></html>")
	res.ContentLength = capBytes + 100
	out := fetched{FinalURL: "https://example.com/huge-robots", Status: 200}

	f.readTerminal(res, &out, true)

	if out.ErrorType != ErrTooLarge {
		t.Errorf("out.ErrorType = %q, want %q", out.ErrorType, ErrTooLarge)
	}
	if out.ExtractionStatus != ExtractionStatusTruncatedSizeLimit {
		t.Errorf("out.ExtractionStatus = %q, want %q", out.ExtractionStatus, ExtractionStatusTruncatedSizeLimit)
	}
	if out.XRobotsTagTotalCount != 1 || len(out.RawXRobotsTags) != 1 || out.RawXRobotsTags[0] != "noarchive" {
		t.Errorf("out XRobots mismatch: total=%d, raw=%v", out.XRobotsTagTotalCount, out.RawXRobotsTags)
	}

	p := buildPage(frontierItem{URL: "https://example.com/huge-robots"}, out, "example.com", Options{})
	if p.ExtractionStatus != ExtractionStatusTruncatedSizeLimit {
		t.Errorf("p.ExtractionStatus = %q, want %q", p.ExtractionStatus, ExtractionStatusTruncatedSizeLimit)
	}
	if p.RobotsMetaExtracted {
		t.Errorf("p.RobotsMetaExtracted = true, want false (body never downloaded)")
	}
	if p.RobotsMetaTotalCount != 0 || len(p.RawRobotsMeta) != 0 {
		t.Errorf("p robots meta counts mismatch: total=%d, raw=%v", p.RobotsMetaTotalCount, p.RawRobotsMeta)
	}
	if p.XRobotsTagTotalCount != 1 || len(p.RawXRobotsTags) != 1 || p.RawXRobotsTags[0] != "noarchive" {
		t.Errorf("p XRobots mismatch: total=%d, raw=%v", p.XRobotsTagTotalCount, p.RawXRobotsTags)
	}
}

// 26. Mid-stream read error preserves meta directives parsed before failure and flags read_error status.
func TestRobotsAcquisition_MidStreamReadError_PreservesPreErrorEvidence(t *testing.T) {
	f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
	partialHTML := []byte(`<!doctype html><html><head><meta name="googlebot" content="nosnippet"></head><body><div>chunk`)
	bodyReader := &errAfterBytesReader{
		data: partialHTML,
		err:  fmt.Errorf("simulated network reset mid-stream"),
	}
	res := newTestHTMLResponse("https://example.com/err-robots", 200, -1, bodyReader)
	out := fetched{FinalURL: "https://example.com/err-robots", Status: 200}

	f.readTerminal(res, &out, true)

	if out.ExtractionStatus != ExtractionStatusReadError {
		t.Fatalf("out.ExtractionStatus = %q, want %q", out.ExtractionStatus, ExtractionStatusReadError)
	}

	p := buildPage(frontierItem{URL: "https://example.com/err-robots"}, out, "example.com", Options{})
	if p.ExtractionStatus != ExtractionStatusReadError {
		t.Errorf("p.ExtractionStatus = %q, want %q", p.ExtractionStatus, ExtractionStatusReadError)
	}
	if !p.RobotsMetaExtracted {
		t.Errorf("p.RobotsMetaExtracted = false, want true (extractor parsed pre-error bytes)")
	}
	if p.RobotsMetaTotalCount != 1 {
		t.Errorf("p.RobotsMetaTotalCount = %d, want 1", p.RobotsMetaTotalCount)
	}
	if len(p.RawRobotsMeta) != 1 {
		t.Fatalf("len(p.RawRobotsMeta) = %d, want 1", len(p.RawRobotsMeta))
	}
	if p.RawRobotsMeta[0].Name != "googlebot" || p.RawRobotsMeta[0].ContentRaw != "nosnippet" {
		t.Errorf("p.RawRobotsMeta[0] = %+v, unexpected content", p.RawRobotsMeta[0])
	}
}
