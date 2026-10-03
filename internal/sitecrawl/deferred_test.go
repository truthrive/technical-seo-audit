package sitecrawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

func TestTemporaryRefusal429IsRetried(t *testing.T) {
	fastCrawl(t)

	var refusals atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/robots.txt", "/sitemap.xml":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
		case "/":
			fmt.Fprint(w, `<!doctype html><html><body><a href="/busy-429">Busy</a></body></html>`)
		case "/busy-429":
			if refusals.Add(1) == 1 {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			fmt.Fprint(w, `<!doctype html><html><body><a href="/behind-busy-429">Deeper</a></body></html>`)
		case "/behind-busy-429":
			fmt.Fprint(w, `<!doctype html><html><body>Deeper content</body></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      2,
		FollowRedirects:  true,
		CrawlExternal:    false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("run state = %s, want %s", summary.State, StateCompleted)
	}

	if got := refusals.Load(); got < 2 {
		t.Fatalf("/busy-429 was requested %d time(s), want at least 2", got)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	var busyFound, deeperFound bool
	for _, p := range pages {
		if p.URL == srv.URL+"/busy-429" {
			busyFound = true
			if p.Status != http.StatusOK {
				t.Errorf("/busy-429 status = %d, want %d", p.Status, http.StatusOK)
			}
		}
		if p.URL == srv.URL+"/behind-busy-429" {
			deeperFound = true
			if p.Status != http.StatusOK {
				t.Errorf("/behind-busy-429 status = %d, want %d", p.Status, http.StatusOK)
			}
		}
	}

	if !busyFound {
		t.Error("/busy-429 is missing from crawled pages")
	}
	if !deeperFound {
		t.Error("/behind-busy-429 linked only from /busy-429 is missing from crawled pages")
	}
}

func TestTemporaryRefusal503IsRetried(t *testing.T) {
	fastCrawl(t)

	var refusals atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		switch r.URL.Path {
		case "/robots.txt", "/sitemap.xml":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNotFound)
		case "/":
			fmt.Fprint(w, `<!doctype html><html><body><a href="/busy-503">Busy</a></body></html>`)
		case "/busy-503":
			if refusals.Add(1) == 1 {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			fmt.Fprint(w, `<!doctype html><html><body><a href="/behind-busy-503">Deeper</a></body></html>`)
		case "/behind-busy-503":
			fmt.Fprint(w, `<!doctype html><html><body>Deeper content</body></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      2,
		FollowRedirects:  true,
		CrawlExternal:    false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("run state = %s, want %s", summary.State, StateCompleted)
	}

	if got := refusals.Load(); got < 2 {
		t.Fatalf("/busy-503 was requested %d time(s), want at least 2", got)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	var busyFound, deeperFound bool
	for _, p := range pages {
		if p.URL == srv.URL+"/busy-503" {
			busyFound = true
			if p.Status != http.StatusOK {
				t.Errorf("/busy-503 status = %d, want %d", p.Status, http.StatusOK)
			}
		}
		if p.URL == srv.URL+"/behind-busy-503" {
			deeperFound = true
			if p.Status != http.StatusOK {
				t.Errorf("/behind-busy-503 status = %d, want %d", p.Status, http.StatusOK)
			}
		}
	}

	if !busyFound {
		t.Error("/busy-503 is missing from crawled pages")
	}
	if !deeperFound {
		t.Error("/behind-busy-503 linked only from /busy-503 is missing from crawled pages")
	}
}

func TestPersistentRefusal429IsRecorded(t *testing.T) {
	fastCrawl(t)

	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt", "/sitemap.xml":
			w.WriteHeader(http.StatusNotFound)
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html><body><a href="/always-429">Always Busy</a></body></html>`)
		case "/always-429":
			hits.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      2,
		FollowRedirects:  true,
		CrawlExternal:    false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("run state = %s, want %s", summary.State, StateCompleted)
	}

	if got := hits.Load(); got != 2 {
		t.Errorf("/always-429 requested %d times, want exactly 2 (initial attempt + one retry)", got)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	var found bool
	for _, p := range pages {
		if p.URL == srv.URL+"/always-429" {
			found = true
			if p.Status != http.StatusTooManyRequests {
				t.Errorf("/always-429 status = %d, want %d", p.Status, http.StatusTooManyRequests)
			}
		}
	}
	if !found {
		t.Error("/always-429 was not recorded in crawled pages")
	}
}

func TestPersistentRefusal503IsRecorded(t *testing.T) {
	fastCrawl(t)

	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt", "/sitemap.xml":
			w.WriteHeader(http.StatusNotFound)
		case "/":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html><body><a href="/always-503">Always 503</a></body></html>`)
		case "/always-503":
			hits.Add(1)
			w.WriteHeader(http.StatusServiceUnavailable)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      2,
		FollowRedirects:  true,
		CrawlExternal:    false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("run state = %s, want %s", summary.State, StateCompleted)
	}

	if got := hits.Load(); got != 2 {
		t.Errorf("/always-503 requested %d times, want exactly 2 (initial attempt + one retry)", got)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	var found bool
	for _, p := range pages {
		if p.URL == srv.URL+"/always-503" {
			found = true
			if p.Status != http.StatusServiceUnavailable {
				t.Errorf("/always-503 status = %d, want %d", p.Status, http.StatusServiceUnavailable)
			}
		}
	}
	if !found {
		t.Error("/always-503 was not recorded in crawled pages")
	}
}
