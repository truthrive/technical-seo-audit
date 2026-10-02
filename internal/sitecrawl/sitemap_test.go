package sitecrawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestSitemapChildrenFetchedTogether is the regression test for a crawl that sat
// in "Preparing" before fetching a single page.
//
// Sitemaps are generated on demand, so a CMS answers them slowly whatever their
// size — a real WordPress site measured ~2.5s per file. Walking its 8 children
// one at a time cost 21 seconds up front; together they took 6.4s.
func TestSitemapChildrenFetchedTogether(t *testing.T) {
	const (
		children = 8
		perFile  = 200 * time.Millisecond
	)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(perFile)
		w.Header().Set("Content-Type", "application/xml")
		if r.URL.Path == "/sitemap.xml" {
			var sb strings.Builder
			sb.WriteString(`<?xml version="1.0"?><sitemapindex>`)
			for i := 0; i < children; i++ {
				fmt.Fprintf(&sb, `<sitemap><loc>%s/child%d.xml</loc></sitemap>`, srv.URL, i)
			}
			sb.WriteString(`</sitemapindex>`)
			w.Write([]byte(sb.String()))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/child") {
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset><url><loc>%s/page%s</loc></url></urlset>`,
				srv.URL, strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/child"), ".xml"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	start := time.Now()
	entries, read := discoverSitemaps(context.Background(), srv.Client(), presetFor(DefaultUserAgent),
		srv.URL, nil, children)
	elapsed := time.Since(start)

	if len(entries) != children {
		t.Errorf("got %d entries from %d child sitemaps, want one each: %v", len(entries), children, read)
	}
	// Serial would be at least children*perFile on top of the index and the
	// probes for the other common paths. Two levels of latency plus slack is the
	// most a concurrent walk can honestly need.
	budget := 4 * perFile
	if elapsed > budget {
		t.Errorf("discovery took %v for %d children at %v each — the level is being walked serially, not together",
			elapsed.Round(time.Millisecond), children, perFile)
	}
}

// TestSitemapIndexLoopTerminates: an index that points at itself must not walk
// forever. The seen-set is what stops it, and it now spans levels.
func TestSitemapIndexLoopTerminates(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0"?><sitemapindex>
			<sitemap><loc>%s/sitemap.xml</loc></sitemap>
			<sitemap><loc>%s/other.xml</loc></sitemap></sitemapindex>`, srv.URL, srv.URL)
	}))
	defer srv.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		discoverSitemaps(context.Background(), srv.Client(), presetFor(DefaultUserAgent), srv.URL, nil, 4)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("discovery never returned on a self-referencing sitemap index")
	}
}
