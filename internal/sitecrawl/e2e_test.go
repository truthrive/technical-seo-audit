package sitecrawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

type fixtureSite struct {
	*httptest.Server
	mu    sync.Mutex
	hits  map[string]int
	total int
}

func (f *fixtureSite) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func fixtureHTML(title, meta, h1, body, extraHead string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s</title>
<meta name="description" content="%s">
<meta name="viewport" content="width=device-width, initial-scale=1">
%s
</head>
<body>
<header><a href="/">Home</a></header>
<nav><a href="/about">About</a><a href="/contact">Contact</a></nav>
<main><h1>%s</h1><h2>Section</h2>%s</main>
<footer><a href="/privacy">Privacy</a></footer>
</body></html>`, title, meta, extraHead, h1, body)
}

func newFixtureSite(t *testing.T) *fixtureSite {
	t.Helper()
	f := &fixtureSite{hits: map[string]int{}}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	f.Server = srv
	t.Cleanup(srv.Close)

	record := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			f.hits[r.URL.Path]++
			f.total++
			f.mu.Unlock()
			h(w, r)
		}
	}

	html := func(body string) http.HandlerFunc {
		return record(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(body))
		})
	}

	mux.HandleFunc("/robots.txt", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "User-agent: *\nDisallow: /private/\nSitemap: %s/sitemap.xml\n", f.URL)
	}))

	mux.HandleFunc("/sitemap.xml", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/</loc></url>
  <url><loc>%s/orphan</loc><lastmod>2026-01-01</lastmod></url>
</urlset>`, f.URL, f.URL)
	}))

	mux.HandleFunc("/", record(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(fixtureHTML("Not found", "", "404", "", "")))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(fixtureHTML(
			"Home page of fixture",
			"Comfortably sized meta description for the fixture home page.",
			"Welcome",
			`<a href="/about">About</a>
			 <a href="/missing">Missing 404</a>
			 <a href="/hop1">Redirect hop</a>
			 <a href="/private/secret">Blocked by robots</a>
			 <a href="/dup">Duplicate</a>
			 <img src="/logo.png" alt="Logo">
			 <img src="/noalt.png">`,
			`<link rel="canonical" href="`+f.URL+`/">`)))
	}))

	mux.HandleFunc("/about", html(fixtureHTML("About Us", "About page description.", "About", `<p>About content</p>`, "")))
	mux.HandleFunc("/contact", html(fixtureHTML("Contact Us", "Contact page description.", "Contact", `<p>Contact content</p>`, "")))
	mux.HandleFunc("/privacy", html(fixtureHTML("Privacy Policy", "Privacy description.", "Privacy", `<p>Privacy content</p>`, "")))

	// 2-hop redirect chain: /hop1 -> /hop2 -> /about
	mux.HandleFunc("/hop1", record(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/hop2", http.StatusMovedPermanently)
	}))
	mux.HandleFunc("/hop2", record(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/about", http.StatusMovedPermanently)
	}))

	mux.HandleFunc("/private/secret", html(fixtureHTML("Secret", "", "Secret", "", "")))

	mux.HandleFunc("/logo.png", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))
	mux.HandleFunc("/noalt.png", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))

	mux.HandleFunc("/dup", html(fixtureHTML("About Us", "About page description.", "About", `<p>Duplicate content</p>`, "")))
	mux.HandleFunc("/orphan", html(fixtureHTML("Orphan Page", "Orphan description.", "Orphan", `<p>Orphan from sitemap</p>`, "")))

	return f
}

func testOptions() Options {
	return Options{
		Mode:             ModeSpider,
		MaxDepth:         3,
		MaxURLs:          50,
		Concurrency:      4,
		TimeoutSec:       5,
		FollowRedirects:  true,
		CrawlExternal:    false,
		CrawlImages:      true,
		RespectRobots:    true,
		DiscoverSitemaps: true,
	}
}

func TestEndToEndCrawl(t *testing.T) {
	fixture := newFixtureSite(t)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	collector := standalone.NewEventCollector()
	runner := NewRunner(db)
	runner.Events = collector

	// 1. Test capability gates first.
	opts := testOptions()
	opts.EnablePageSpeed = true
	if _, err := runner.Crawl(context.Background(), []string{fixture.URL}, opts); err == nil || !strings.Contains(err.Error(), "capability unsupported") {
		t.Fatalf("expected ErrCapabilityUnsupported for PageSpeed, got %v", err)
	}

	opts = testOptions()
	opts.EnableDuplication = true
	if _, err := runner.Crawl(context.Background(), []string{fixture.URL}, opts); err == nil || !strings.Contains(err.Error(), "capability unsupported") {
		t.Fatalf("expected ErrCapabilityUnsupported for Duplication, got %v", err)
	}

	opts = testOptions()
	opts.UseProxy = true
	runner.Proxy = nil
	if _, err := runner.Crawl(context.Background(), []string{fixture.URL}, opts); err == nil || !strings.Contains(err.Error(), "proxy requested") {
		t.Fatalf("expected ErrProxyUnavailable, got %v", err)
	}

	// 2. Test full valid crawl.
	opts = testOptions()
	summary, err := runner.Crawl(context.Background(), []string{fixture.URL}, opts)
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	if summary.State != StateCompleted {
		t.Fatalf("expected run state %q, got %q (reason: %s)", StateCompleted, summary.State, summary.StopReason)
	}
	if summary.Found < 5 {
		t.Fatalf("expected at least 5 found URLs, got %d", summary.Found)
	}
	if summary.Crawled < 4 {
		t.Fatalf("expected at least 4 crawled pages, got %d", summary.Crawled)
	}

	// Verify URL dictionary rows.
	urls, err := runner.URLs(summary.ID)
	if err != nil {
		t.Fatalf("load URLs: %v", err)
	}
	if len(urls) < 5 {
		t.Fatalf("expected at least 5 URLs in dictionary, got %d", len(urls))
	}

	// Verify Pages rows.
	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("load Pages: %v", err)
	}
	if len(pages) < 4 {
		t.Fatalf("expected at least 4 Pages, got %d", len(pages))
	}

	var has200, has404, hasRedirect, hasRobotsBlocked bool
	for _, p := range pages {
		if p.Status == 200 {
			has200 = true
		}
		if p.Status == 404 {
			has404 = true
		}
		if p.Status == 301 || len(p.Redirects) > 0 {
			hasRedirect = true
		}
		if p.RobotsState == RobotsBlocked {
			hasRobotsBlocked = true
		}
	}

	if !has200 {
		t.Errorf("missing status 200 page")
	}
	if !has404 {
		t.Errorf("missing status 404 page")
	}
	if !hasRedirect {
		t.Errorf("missing redirect evidence")
	}
	if !hasRobotsBlocked {
		t.Errorf("missing robots blocked evidence")
	}

	// Verify Link rows.
	links, err := runner.Links(summary.ID)
	if err != nil {
		t.Fatalf("load Links: %v", err)
	}
	if len(links) < 4 {
		t.Fatalf("expected at least 4 Links, got %d", len(links))
	}

	// Verify inlinks computation.
	var inlinkCount int
	err = db.QueryRow(`SELECT SUM(inlinks) FROM sitecrawl_pages WHERE run_id = ?`, summary.ID).Scan(&inlinkCount)
	if err != nil {
		t.Fatalf("query inlinks: %v", err)
	}
	if inlinkCount <= 0 {
		t.Fatalf("expected inlinks > 0, got %d", inlinkCount)
	}

	// Verify events were emitted.
	if collector.Len() == 0 {
		t.Fatalf("expected events to be emitted, got 0")
	}
	var sawProgress, sawRunState bool
	for _, ev := range collector.Events() {
		if ev.Name == EventProgress {
			sawProgress = true
		}
		if ev.Name == EventRunState {
			sawRunState = true
		}
	}
	if !sawProgress {
		t.Errorf("did not observe EventProgress")
	}
	if !sawRunState {
		t.Errorf("did not observe EventRunState")
	}
}

func TestProcessLevelReadback(t *testing.T) {
	fixture := newFixtureSite(t)

	dbPath := filepath.Join(t.TempDir(), "crawl_process_readback.db")

	// Phase 1: Open, migrate, crawl, close.
	db1, err := standalone.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db1: %v", err)
	}

	runner1 := NewRunner(db1)
	opts := testOptions()
	summary1, err := runner1.Crawl(context.Background(), []string{fixture.URL}, opts)
	if err != nil {
		db1.Close()
		t.Fatalf("crawl 1 failed: %v", err)
	}
	runID := summary1.ID
	if err := db1.Close(); err != nil {
		t.Fatalf("close db1: %v", err)
	}

	// Phase 2: Reopen the database, re-run schema migration (must be idempotent), read back.
	db2, err := standalone.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db2: %v", err)
	}
	defer db2.Close()

	runner2 := NewRunner(db2)
	// Calling EnsureSchema on an already-migrated database must succeed without errors.
	if err := runner2.EnsureSchema(); err != nil {
		t.Fatalf("re-running schema on reopened db failed: %v", err)
	}

	loadedRun, err := runner2.LoadRun(runID)
	if err != nil {
		t.Fatalf("load run from reopened db: %v", err)
	}
	if loadedRun.ID != runID {
		t.Fatalf("expected run ID %s, got %s", runID, loadedRun.ID)
	}
	if loadedRun.State != StateCompleted {
		t.Fatalf("expected StateCompleted, got %s", loadedRun.State)
	}
	if loadedRun.Crawled != summary1.Crawled {
		t.Fatalf("expected crawled %d, got %d", summary1.Crawled, loadedRun.Crawled)
	}
	if loadedRun.Found != summary1.Found {
		t.Fatalf("expected found %d, got %d", summary1.Found, loadedRun.Found)
	}

	pages, err := runner2.Pages(runID)
	if err != nil {
		t.Fatalf("load pages from reopened db: %v", err)
	}
	if len(pages) != summary1.Crawled {
		t.Fatalf("expected %d pages, got %d", summary1.Crawled, len(pages))
	}

	links, err := runner2.Links(runID)
	if err != nil {
		t.Fatalf("load links from reopened db: %v", err)
	}
	if len(links) == 0 {
		t.Fatalf("expected persisted links after reopen, got 0")
	}

	urls, err := runner2.URLs(runID)
	if err != nil {
		t.Fatalf("load urls from reopened db: %v", err)
	}
	if len(urls) != summary1.Found {
		t.Fatalf("expected %d urls, got %d", summary1.Found, len(urls))
	}
}

func TestContextCancellation(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	slowStarted := make(chan struct{})
	slowRelease := make(chan struct{})

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "User-agent: *\nAllow: /\n")
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><html><body><a href="/slow">Slow</a></body></html>`))
	})

	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		close(slowStarted)
		<-slowRelease
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(`<!doctype html><html><body>Slow Done</body></html>`))
	})

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := testOptions()
	opts.Concurrency = 1

	go func() {
		select {
		case <-slowStarted:
			cancel()
			close(slowRelease)
		case <-time.After(5 * time.Second):
			cancel()
			close(slowRelease)
		}
	}()

	summary, err := runner.Crawl(ctx, []string{srv.URL}, opts)
	if err == nil {
		t.Fatalf("expected context cancellation error, got nil")
	}

	if summary == nil {
		t.Fatalf("expected non-nil summary on cancellation")
	}
	if summary.State != StateStopped {
		t.Fatalf("expected run state %q, got %q", StateStopped, summary.State)
	}
	if summary.StopReason != StopUser {
		t.Fatalf("expected stop reason %q, got %q", StopUser, summary.StopReason)
	}

	// Verify the database is readable and intact.
	urls, err := runner.URLs(summary.ID)
	if err != nil {
		t.Fatalf("read URLs after cancellation: %v", err)
	}
	if len(urls) == 0 {
		t.Fatalf("expected buffered URLs to be saved, got 0")
	}
}

func TestBotProfileFallbackEvidence(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var (
		botRequests     int
		browserRequests int
		mu              sync.Mutex
	)

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "User-agent: *\nAllow: /\n")
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		ua := r.Header.Get("User-Agent")
		if strings.Contains(ua, "Googlebot") || strings.Contains(ua, "1ScoutSiteCrawl") {
			botRequests++
			// 1. Refuse the configured bot-profile request.
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("Forbidden to bots"))
			return
		}
		// 2. Allow the existing browser fallback.
		if strings.Contains(ua, "Chrome") {
			browserRequests++
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<!doctype html><html><head><title>Bot Protected</title></head><body>Content for browsers</body></html>"))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	})

	dbPath := filepath.Join(t.TempDir(), "bot_fallback.db")
	db, err := standalone.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	runner := NewRunner(db)
	opts := testOptions()
	opts.UserAgent = "googlebot"

	// 3. Completes page fetch & 4. Persists the Page.
	summary, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("expected state %q, got %q", StateCompleted, summary.State)
	}

	mu.Lock()
	bots := botRequests
	browsers := browserRequests
	mu.Unlock()

	if bots == 0 {
		t.Fatalf("expected at least 1 bot request to be refused, got 0")
	}
	if browsers == 0 {
		t.Fatalf("expected at least 1 browser fallback request, got 0")
	}

	// Close database to verify persistence across close/reopen.
	if err := db.Close(); err != nil {
		t.Fatalf("close db: %v", err)
	}

	reopenedDB, err := standalone.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("reopen db: %v", err)
	}
	defer reopenedDB.Close()

	// 5. Read back through Runner.
	reopenedRunner := NewRunner(reopenedDB)
	pages, err := reopenedRunner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("load pages: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	// 6. Confirm BotBlocked == true.
	p := pages[0]
	if !p.BotBlocked {
		t.Fatalf("expected p.BotBlocked to be true, got false")
	}
	if p.Status != 200 {
		t.Fatalf("expected status 200 from successful fallback, got %d", p.Status)
	}

	single, err := reopenedRunner.Page(summary.ID, 1)
	if err != nil {
		t.Fatalf("load single page: %v", err)
	}
	if !single.BotBlocked {
		t.Fatalf("expected single.BotBlocked to be true, got false")
	}
}
