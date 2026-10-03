package sitecrawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

func TestRedirectChainPreserved(t *testing.T) {
	fastCrawl(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body><a href="/hop1">Redirect chain</a></body></html>`)
	})
	mux.HandleFunc("/hop1", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/hop2", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/hop2", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/hop3", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/hop3", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/about", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body><h1>About</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
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
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	var hop1Found, aboutFound bool
	for _, p := range pages {
		if p.URL == srv.URL+"/hop1" {
			hop1Found = true
			if len(p.Redirects) != 3 {
				t.Errorf("/hop1 redirects count = %d, want 3", len(p.Redirects))
			}
			if p.RedirectTo != srv.URL+"/hop2" {
				t.Errorf("/hop1 RedirectTo = %q, want %q", p.RedirectTo, srv.URL+"/hop2")
			}
		}
		if p.URL == srv.URL+"/about" {
			aboutFound = true
			if p.Status != http.StatusOK {
				t.Errorf("/about status = %d, want 200", p.Status)
			}
		}
	}
	if !hop1Found {
		t.Error("/hop1 was not recorded in crawled pages")
	}
	if !aboutFound {
		t.Error("/about target of redirect chain was not crawled")
	}
}

func TestRedirectLoopTerminates(t *testing.T) {
	fastCrawl(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body><a href="/loop-a">Loop</a></body></html>`)
	})
	mux.HandleFunc("/loop-a", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop-b", http.StatusFound)
	})
	mux.HandleFunc("/loop-b", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop-a", http.StatusFound)
	})

	srv := httptest.NewServer(mux)
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
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}

	var loopFound bool
	for _, p := range pages {
		if strings.Contains(p.URL, "/loop-a") {
			loopFound = true
			if p.Error == "" && p.ErrorType == "" {
				t.Errorf("redirect loop on %s did not record an error: %+v", p.URL, p)
			}
			if len(p.Redirects) > maxRedirects+2 {
				t.Errorf("redirect loop exceeded safety cap: followed %d times", len(p.Redirects))
			}
		}
	}
	if !loopFound {
		t.Error("/loop-a was not found in crawled pages")
	}
}

func TestSeedRedirectOffsiteExplainsItself(t *testing.T) {
	fastCrawl(t)

	const target = "https://genuinely-elsewhere.example/landing"
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	collector := standalone.NewEventCollector()
	runner := NewRunner(db)
	runner.Events = collector

	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      1,
		FollowRedirects:  false,
		CrawlExternal:    false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}
	if summary.StopReason != StopSeedRedirect {
		t.Errorf("stop reason = %q, want %q", summary.StopReason, StopSeedRedirect)
	}
	if summary.Found != 1 || summary.Crawled != 1 {
		t.Errorf("found/crawled = %d/%d, want 1/1", summary.Found, summary.Crawled)
	}

	var lastState *RunStateEvent
	for _, ev := range collector.Events() {
		if ev.Name == EventRunState {
			if rs, ok := ev.Data.(RunStateEvent); ok {
				lastState = &rs
			}
		}
	}
	if lastState == nil {
		t.Fatal("no EventRunState event was emitted")
	}
	if lastState.Reason != StopSeedRedirect {
		t.Errorf("event reason = %q, want %q", lastState.Reason, StopSeedRedirect)
	}
	if lastState.RedirectTarget != target {
		t.Errorf("event redirect target = %q, want %q", lastState.RedirectTarget, target)
	}
}

func TestSitemapEntriesOnAnotherHostAreIgnored(t *testing.T) {
	fastCrawl(t)

	var foreignHits sync.Map

	cdnMux := http.NewServeMux()
	cdn := httptest.NewServer(cdnMux)
	defer cdn.Close()

	siteMux := http.NewServeMux()
	site := httptest.NewServer(siteMux)
	defer site.Close()

	siteLocal := strings.Replace(site.URL, "127.0.0.1", "localhost", 1)

	siteMux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", cdn.URL)
	})
	plain := func(title string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
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

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      2,
		RespectRobots:    true,
		DiscoverSitemaps: true,
		FollowRedirects:  false,
		CrawlExternal:    false,
	}

	summary, err := runner.Crawl(context.Background(), []string{siteLocal}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}

	var fetchedForeign []string
	foreignHits.Range(func(k, _ any) bool {
		fetchedForeign = append(fetchedForeign, k.(string))
		return true
	})
	if len(fetchedForeign) > 0 {
		t.Errorf("foreign sitemap URLs were fetched: %v", fetchedForeign)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	var fromSitemapFound bool
	for _, p := range pages {
		if strings.HasSuffix(p.URL, "/from-sitemap") {
			fromSitemapFound = true
		}
	}
	if !fromSitemapFound {
		t.Error("the sitemap entry pointing at the crawl's own site was not crawled")
	}

	// found = seed + the one same-site sitemap entry. The two foreign entries must not count.
	if summary.Found != 2 {
		t.Errorf("found = %d, want 2 (foreign sitemap entries must not inflate found count)", summary.Found)
	}
}

func TestFoundConvergesToCrawled(t *testing.T) {
	fastCrawl(t)

	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nAllow: /\n")
	})
	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
	})

	html := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}

	// Home page links to 10 pages.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var links []string
		for i := 1; i <= 10; i++ {
			links = append(links, fmt.Sprintf(`<a href="/page-%d">Page %d</a>`, i, i))
		}
		html(w, fmt.Sprintf(`<!doctype html><html><body><h1>Home</h1>%s</body></html>`, strings.Join(links, " ")))
	})

	for i := 1; i <= 10; i++ {
		p := fmt.Sprintf("/page-%d", i)
		title := fmt.Sprintf("Page %d", i)
		mux.HandleFunc(p, func(w http.ResponseWriter, r *http.Request) {
			html(w, fmt.Sprintf(`<!doctype html><html><body><h1>%s</h1><p>Content</p></body></html>`, title))
		})
	}

	srv := httptest.NewServer(mux)
	defer srv.Close()

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      4,
		FollowRedirects:  true,
		CrawlExternal:    false,
		RespectRobots:    true,
		DiscoverSitemaps: true,
	}

	summary, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("run state = %s, want %s", summary.State, StateCompleted)
	}
	if summary.Crawled < 11 {
		t.Errorf("crawled %d pages, expected at least 11", summary.Crawled)
	}
	if summary.Found != summary.Crawled {
		t.Errorf("found %d != crawled %d on clean completion", summary.Found, summary.Crawled)
	}
}
