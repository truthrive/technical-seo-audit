package sitecrawl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// lastRunStateEvent returns the most recent RunStateEvent the service emitted.
func lastRunStateEvent(e *stubEmitter) (RunStateEvent, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := len(e.events) - 1; i >= 0; i-- {
		if e.events[i].name != EventRunState {
			continue
		}
		if rs, ok := e.events[i].data.(RunStateEvent); ok {
			return rs, true
		}
	}
	return RunStateEvent{}, false
}

// TestSeedRedirectOffsiteExplainsItself covers Screaming Frog's most-asked
// question: the start URL 301s to another domain, so the crawl can only ever
// record that one redirect. Completing silently reads as a bug — the run has
// to say why it ended and where the site actually lives.
func TestSeedRedirectOffsiteExplainsItself(t *testing.T) {
	fastCrawl(t)
	s, db, emitter := newTestService(t)

	const target = "https://genuinely-elsewhere.example/landing"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	opts := s.DefaultOptions()
	opts.Concurrency = 1
	// Hermetic: never leave the test server. RedirectTo still comes from the
	// Location header of the first hop, and CrawlExternal off keeps the fetch
	// of the off-site target (a real DNS lookup) out of a unit test.
	opts.FollowRedirects = false
	opts.CrawlExternal = false
	opts.RespectRobots = false
	opts.DiscoverSitemaps = false

	started, err := s.Start([]string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, s, started.RunID, StateCompleted, StateFailed)

	run, err := loadRun(db, started.RunID)
	if err != nil {
		t.Fatalf("loadRun: %v", err)
	}
	if run.State != StateCompleted {
		t.Fatalf("state = %s, want completed", run.State)
	}
	if run.StopReason != StopSeedRedirect {
		t.Errorf("stop reason = %q, want %q", run.StopReason, StopSeedRedirect)
	}
	// Found once counted every skipped external and read "1 / 91" — on a
	// one-page crawl both numbers must be exactly 1.
	if run.Found != 1 || run.Crawled != 1 {
		t.Errorf("found/crawled = %d/%d, want 1/1", run.Found, run.Crawled)
	}

	ev, ok := lastRunStateEvent(emitter)
	if !ok {
		t.Fatal("no run-state event emitted")
	}
	if ev.Reason != StopSeedRedirect {
		t.Errorf("event reason = %q, want %q", ev.Reason, StopSeedRedirect)
	}
	if ev.RedirectTarget != target {
		t.Errorf("event redirect target = %q, want %q — the UI's \"crawl that instead\" button needs it", ev.RedirectTarget, target)
	}
}

// TestSitemapEntriesOnAnotherHostAreIgnored: the sitemap fetch follows
// redirects, so a shell domain that 301s elsewhere serves the TARGET's
// sitemap. Its foreign URLs must be dropped — ingesting them once inflated
// "found" with 90 URLs the crawl could never fetch. Entries pointing back at
// the crawl's own site stay welcome (sitemaps legitimately live on CDNs).
func TestSitemapEntriesOnAnotherHostAreIgnored(t *testing.T) {
	fastCrawl(t)
	s, db, _ := newTestService(t)

	var foreignHits sync.Map

	// The "CDN": hosts the sitemap, plus pages that must never be fetched.
	cdnMux := http.NewServeMux()
	cdn := httptest.NewServer(cdnMux)
	t.Cleanup(cdn.Close)

	// The site under crawl. Its robots.txt declares the CDN-hosted sitemap.
	siteMux := http.NewServeMux()
	site := httptest.NewServer(siteMux)
	t.Cleanup(site.Close)

	// Both servers bind 127.0.0.1 and sameSite compares hostnames (ports
	// ignored) — so the crawl enters through the "localhost" name to give the
	// site a hostname genuinely different from the CDN's.
	siteLocal := strings.Replace(site.URL, "127.0.0.1", "localhost", 1)

	siteMux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", cdn.URL)
	})
	plain := func(title string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<!doctype html><html><head><title>%s</title></head><body><h1>%s</h1></body></html>", title, title)
		}
	}
	siteMux.HandleFunc("/", plain("Seed"))
	siteMux.HandleFunc("/from-sitemap", plain("From sitemap"))

	cdnMux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/from-sitemap</loc></url>
  <url><loc>%s/foreign-1</loc></url>
  <url><loc>%s/foreign-2</loc></url>
</urlset>`, siteLocal, cdn.URL, cdn.URL)
	})
	cdnMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		foreignHits.Store(r.URL.Path, true)
		plain("Foreign")(w, r)
	})

	opts := s.DefaultOptions()
	opts.Concurrency = 2
	opts.RespectRobots = true
	opts.DiscoverSitemaps = true
	opts.FollowRedirects = false

	started, err := s.Start([]string{siteLocal}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, s, started.RunID, StateCompleted, StateFailed)

	run, err := loadRun(db, started.RunID)
	if err != nil {
		t.Fatalf("loadRun: %v", err)
	}
	if run.State != StateCompleted {
		t.Fatalf("state = %s, want completed", run.State)
	}

	var fetched []string
	foreignHits.Range(func(k, _ any) bool {
		fetched = append(fetched, k.(string))
		return true
	})
	if len(fetched) > 0 {
		t.Errorf("foreign sitemap URLs were fetched: %v", fetched)
	}

	var fromSitemap int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ? AND url = ?`,
		started.RunID, siteLocal+"/from-sitemap").Scan(&fromSitemap)
	if fromSitemap != 1 {
		t.Error("the sitemap entry pointing at the crawl's own site was not crawled — CDN-hosted sitemaps must still work")
	}

	// found = seed + the one same-site sitemap entry. The two foreign entries
	// must not count.
	if run.Found != 2 {
		t.Errorf("found = %d, want 2 (foreign sitemap entries must not inflate it)", run.Found)
	}
}

// TestFoundConvergesToCrawled: on a clean completion every queued URL was
// fetched, so the two headline numbers must agree — a progress bar that ends
// at "14 / 91" reads as a hang.
func TestFoundConvergesToCrawled(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	runID := crawlFixture(t, s, site, nil)
	st, err := s.Status(runID)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.State != StateCompleted {
		t.Skipf("run ended %s; convergence only holds for completed runs", st.State)
	}
	if st.Found != st.Crawled {
		t.Errorf("found %d != crawled %d after a clean completion", st.Found, st.Crawled)
	}
	if st.Crawled < 10 {
		t.Errorf("only %d pages crawled — the fixture has far more, the counter fix broke discovery", st.Crawled)
	}
}
