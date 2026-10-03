package sitecrawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

// TestRenderCapIsEnforced verifies that the render page cap is enforced by renderGate
// and that releasing a slot returns it. Runs environment-independently without Chrome.
func TestRenderCapIsEnforced(t *testing.T) {
	gate := newRenderGate(Options{JSMaxPages: 2}.normalized())
	if !gate.admit("https://a.example/1") || !gate.admit("https://a.example/2") {
		t.Fatal("render gate rejected pages under the configured cap")
	}
	if gate.admit("https://a.example/3") {
		t.Error("render gate admitted a page over the configured cap")
	}
	gate.release()
	if !gate.admit("https://a.example/4") {
		t.Error("release did not return the render slot")
	}
}

// TestRenderPatternGate verifies that when patterns are configured, only matching
// URLs are admitted to render. Runs environment-independently without Chrome.
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

// TestRenderExtractsJSContent proves end-to-end rendering:
// JavaScript-injected content (H1 and link) is extracted from the post-rendered DOM.
// If no supported Chromium-family browser is found on the host machine, the test skips cleanly.
func TestRenderExtractsJSContent(t *testing.T) {
	if detectBrowser() == "" {
		t.Skip("no Chrome/Edge installed; skipping end-to-end browser render test")
	}
	fastCrawl(t)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><title>Static title</title></head>
<body><div id="app"></div>
<script>
  document.getElementById("app").innerHTML =
    '<h1>Rendered heading</h1><a href="/js-only">JS link</a>';
</script></body></html>`)
	})
	mux.HandleFunc("/js-only", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><title>Only reachable through JS</title></head><body><h1>JS only</h1></body></html>`)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	runner := NewRunner(db)
	opts := Options{
		Concurrency:      2,
		CrawlExternal:    false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
		EnableJavaScript: true,
		JSWaitMs:         500,
		JSTimeoutSec:     30,
	}.normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	summary, err := runner.Crawl(ctx, []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	var home *Page
	var jsOnly *Page
	for i := range pages {
		if pages[i].URL == srv.URL+"/" || pages[i].URL == srv.URL {
			home = &pages[i]
		}
		if pages[i].URL == srv.URL+"/js-only" {
			jsOnly = &pages[i]
		}
	}

	if home == nil {
		t.Fatal("homepage was not crawled")
	}
	if !home.Rendered {
		t.Error("homepage was not marked as rendered")
	}
	if len(home.H1) == 0 || home.H1[0] != "Rendered heading" {
		t.Errorf("H1 = %v, want ['Rendered heading']", home.H1)
	}

	if jsOnly == nil {
		t.Error("/js-only was never crawled — JS-discovered links are not being admitted/followed")
	}
}
