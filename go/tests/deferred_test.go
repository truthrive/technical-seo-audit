package sitecrawl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestTemporaryRefusalIsRetried is the regression test for a report full of
// failures that were not failures.
//
// A real crawl of a rate-limited site recorded 104 pages as "429" and stopped
// there. Those pages were fine — the site was momentarily busy — and because a
// refused response carries no body, every page linked only from one of them was
// never discovered either.
func TestTemporaryRefusalIsRetried(t *testing.T) {
	fastCrawl(t)

	var refusals atomic.Int64
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/robots.txt", "/sitemap.xml":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
		case "/":
			fmt.Fprintf(w, `<!doctype html><html lang="en"><head><title>Home of the retry fixture</title>
<meta name="description" content="Seed page for the temporary-refusal retry test, long enough for the rule."></head>
<body><h1>Home</h1><a href="/busy">Busy page</a></body></html>`)
		case "/busy":
			// Refused on the first ask only. The retry must find the real page —
			// and with it the link that is reachable no other way.
			if refusals.Add(1) == 1 {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			fmt.Fprintf(w, `<!doctype html><html lang="en"><head><title>Busy page that came back</title>
<meta name="description" content="This page was refused once and must still end up in the crawl."></head>
<body><h1>Busy</h1><a href="/only-behind-busy">Deeper</a></body></html>`)
		case "/only-behind-busy":
			fmt.Fprintf(w, `<!doctype html><html lang="en"><head><title>Only reachable behind the busy page</title>
<meta name="description" content="Discoverable only from the page that was refused on the first attempt."></head>
<body><h1>Deeper</h1></body></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svc, _, _ := newTestService(t)
	opts := svc.DefaultOptions()
	opts.Concurrency = 2
	opts.CrawlExternal = false
	opts.EnableDuplication = false
	started, err := svc.Start([]string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, svc, started.RunID, StateCompleted, StateFailed, StateStopped)

	if got := refusals.Load(); got < 2 {
		t.Fatalf("/busy was requested %d time(s) — it was never retried after the 429", got)
	}

	busy, err := svc.Page(started.RunID, srv.URL+"/busy")
	if err != nil {
		t.Fatalf("Page(/busy): %v", err)
	}
	if busy.Status != http.StatusOK {
		t.Errorf("/busy recorded as %d — the retry's result must replace the refusal", busy.Status)
	}

	// The whole point of retrying: the page behind the refused one exists.
	if _, err := svc.Page(started.RunID, srv.URL+"/only-behind-busy"); err != nil {
		t.Errorf("the page linked only from /busy is missing from the crawl: %v", err)
	}
}

// TestPersistentRefusalIsRecorded: one retry, not a loop. A site that keeps
// saying 429 must end up reported as 429 rather than crawled forever.
func TestPersistentRefusalIsRecorded(t *testing.T) {
	fastCrawl(t)

	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt", "/sitemap.xml":
			w.WriteHeader(http.StatusNotFound)
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html lang="en"><head><title>Home of the refusal fixture</title>
<meta name="description" content="Seed page for the persistent-refusal test, long enough for the rule."></head>
<body><h1>Home</h1><a href="/always-busy">Busy</a></body></html>`)
		case "/always-busy":
			hits.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	svc, _, _ := newTestService(t)
	opts := svc.DefaultOptions()
	opts.Concurrency = 2
	opts.CrawlExternal = false
	opts.EnableDuplication = false
	started, err := svc.Start([]string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, svc, started.RunID, StateCompleted, StateFailed, StateStopped)

	if got := hits.Load(); got != 2 {
		t.Errorf("/always-busy was requested %d times, want exactly 2 (the attempt and its one retry)", got)
	}
	p, err := svc.Page(started.RunID, srv.URL+"/always-busy")
	if err != nil {
		t.Fatalf("Page(/always-busy): %v", err)
	}
	if p.Status != http.StatusTooManyRequests {
		t.Errorf("a URL refused twice recorded as %d, want it reported as 429", p.Status)
	}
}
