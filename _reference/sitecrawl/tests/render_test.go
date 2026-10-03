package sitecrawl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRenderExtractsJSContent proves the whole rendering path: a page whose
// links and heading exist only after JavaScript runs must still contribute
// them to the crawl. Skips when the machine has no Chromium-family browser —
// rendering borrows an installed one by design.
func TestRenderExtractsJSContent(t *testing.T) {
	if detectBrowser() == "" {
		t.Skip("no Chrome/Edge installed")
	}
	fastCrawl(t)
	s, _, _ := newTestService(t)

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><head><title>Static title</title></head>
<body><div id="app"></div>
<script>
  document.getElementById("app").innerHTML =
    '<h1>Rendered heading</h1><a href="/js-only">JS link</a>';
</script></body></html>`)
	})
	mux.HandleFunc("/js-only", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!doctype html><html><head><title>Only reachable through JS</title></head><body><h1>JS only</h1></body></html>`)
	})

	opts := s.DefaultOptions()
	opts.Concurrency = 2
	opts.CrawlExternal = false
	opts.RespectRobots = false
	opts.DiscoverSitemaps = false
	opts.EnableJavaScript = true
	opts.JSWaitMs = 500
	opts.JSTimeoutSec = 30

	started, err := s.Start([]string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, s, started.RunID, StateCompleted, StateFailed)

	home, err := s.Page(started.RunID, srv.URL+"/")
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if !home.Rendered {
		t.Fatal("homepage was not marked rendered")
	}
	if len(home.H1) == 0 || home.H1[0] != "Rendered heading" {
		t.Errorf("H1 = %v, want the JS-injected heading", home.H1)
	}

	// The link that exists only post-JS must have been discovered and crawled.
	if _, err := s.Page(started.RunID, srv.URL+"/js-only"); err != nil {
		t.Error("/js-only was never crawled — JS-discovered links are being dropped")
	}
}

// TestRenderCapIsEnforced: the hard cap lives in Go, not the UI.
func TestRenderCapIsEnforced(t *testing.T) {
	gate := newRenderGate(Options{JSMaxPages: 2}.normalized())
	// normalized() turns 2 into 2 (min 1). Admit twice, third must fail.
	if !gate.admit("https://a.example/1") || !gate.admit("https://a.example/2") {
		t.Fatal("cap rejected pages under the limit")
	}
	if gate.admit("https://a.example/3") {
		t.Error("cap admitted a page over the limit")
	}
	gate.release()
	if !gate.admit("https://a.example/4") {
		t.Error("release did not return the slot")
	}
}

// TestRenderPatternGate: with patterns set, only matching URLs render.
func TestRenderPatternGate(t *testing.T) {
	opts := Options{JSPatterns: []string{`/blog/`}}.normalized()
	gate := newRenderGate(opts)
	if gate.admit("https://a.example/about") {
		t.Error("non-matching URL admitted")
	}
	if !gate.admit("https://a.example/blog/post") {
		t.Error("matching URL rejected")
	}
}
