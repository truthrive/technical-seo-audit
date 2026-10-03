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

type acquisitionSite struct {
	*httptest.Server
	mu         sync.Mutex
	hits       map[string]int
	methodHits map[string]map[string]int
	total      int
}

func (s *acquisitionSite) count(path string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits[path]
}

func (s *acquisitionSite) countMethod(path, method string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if m, ok := s.methodHits[path]; ok {
		return m[method]
	}
	return 0
}

func (s *acquisitionSite) totalRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.total
}

func newAcquisitionSite(t *testing.T) *acquisitionSite {
	t.Helper()
	site := &acquisitionSite{
		hits:       map[string]int{},
		methodHits: map[string]map[string]int{},
	}

	mux := http.NewServeMux()
	site.Server = httptest.NewServer(mux)
	t.Cleanup(site.Close)

	record := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			site.mu.Lock()
			site.hits[r.URL.Path]++
			site.total++
			if site.methodHits[r.URL.Path] == nil {
				site.methodHits[r.URL.Path] = map[string]int{}
			}
			site.methodHits[r.URL.Path][r.Method]++
			site.mu.Unlock()
			h(w, r)
		}
	}

	html := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}

	mux.HandleFunc("/robots.txt", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "User-agent: *\nAllow: /\n")
	}))

	mux.HandleFunc("/sitemap.xml", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusNotFound)
	}))

	mux.HandleFunc("/", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html>
<head>
	<title>Home</title>
	<link rel="stylesheet" href="/style.css">
	<script src="/app.js"></script>
</head>
<body>
	<header><a href="/">Home</a></header>
	<nav>
		<a href="/about">About</a>
		<a href="/contact">Contact</a>
	</nav>
	<main>
		<h1>Home</h1>
		<a href="/depth-1">Depth 1</a>
		<a href="/noindex" rel="nofollow">Noindex</a>
		<img src="/logo.png" alt="Company Logo">
		<img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=" data-src="/lazy.png" alt="Lazy Loaded">
		<img src="/broken.png" alt="Broken Asset">
	</main>
	<footer>
		<a href="/privacy">Privacy Policy</a>
	</footer>
</body></html>`)
	}))

	mux.HandleFunc("/about", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html><body><h1>About</h1><a href="/contact">Contact</a></body></html>`)
	}))

	mux.HandleFunc("/contact", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html><body><h1>Contact</h1><p>Contact Us</p></body></html>`)
	}))

	mux.HandleFunc("/privacy", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html><body><h1>Privacy</h1></body></html>`)
	}))

	mux.HandleFunc("/noindex", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html><head><meta name="robots" content="noindex"></head><body><h1>Noindex</h1></body></html>`)
	}))

	mux.HandleFunc("/depth-1", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html><body><h1>Depth 1</h1><a href="/depth-2">Go deeper</a></body></html>`)
	}))

	mux.HandleFunc("/depth-2", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html><body><h1>Depth 2</h1><a href="/depth-3">Go deepest</a></body></html>`)
	}))

	mux.HandleFunc("/depth-3", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html><body><h1>Depth 3</h1><p>Deepest</p></body></html>`)
	}))

	mux.HandleFunc("/style.css", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte("main{margin:0}"))
	}))

	mux.HandleFunc("/app.js", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("console.log('loaded')"))
	}))

	mux.HandleFunc("/logo.png", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))

	mux.HandleFunc("/lazy.png", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))

	mux.HandleFunc("/broken.png", record(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	return site
}

func TestCrawlFetchesPageResources(t *testing.T) {
	fastCrawl(t)
	site := newAcquisitionSite(t)

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
		CrawlImages:      true,
		CrawlCSS:         true,
		CrawlJS:          true,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}

	if n := site.count("/style.css"); n != 1 {
		t.Errorf("/style.css fetched %d times, want 1", n)
	}
	if n := site.count("/app.js"); n != 1 {
		t.Errorf("/app.js fetched %d times, want 1", n)
	}
	if n := site.count("/logo.png"); n != 1 {
		t.Errorf("/logo.png fetched %d times, want 1", n)
	}
	if n := site.count("/lazy.png"); n != 1 {
		t.Errorf("/lazy.png fetched %d times, want 1 (via data-src fallback)", n)
	}
}

func TestCrawlResourceOptionsOff(t *testing.T) {
	fastCrawl(t)
	site := newAcquisitionSite(t)

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
		CrawlImages:      false,
		CrawlCSS:         false,
		CrawlJS:          false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}

	if n := site.count("/style.css"); n != 0 {
		t.Errorf("/style.css fetched %d times with CrawlCSS off", n)
	}
	if n := site.count("/app.js"); n != 0 {
		t.Errorf("/app.js fetched %d times with CrawlJS off", n)
	}
	if n := site.count("/logo.png"); n != 0 {
		t.Errorf("/logo.png fetched %d times with CrawlImages off", n)
	}
	if n := site.count("/lazy.png"); n != 0 {
		t.Errorf("/lazy.png fetched %d times with CrawlImages off", n)
	}
}

func TestBrokenImagePersistedWithoutIssueVerdict(t *testing.T) {
	fastCrawl(t)
	site := newAcquisitionSite(t)

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
		CrawlImages:      true,
		CrawlCSS:         false,
		CrawlJS:          false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{site.URL}, opts)
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

	var brokenImgFound bool
	for _, p := range pages {
		if strings.HasSuffix(p.URL, "/broken.png") {
			brokenImgFound = true
			if p.Status != http.StatusNotFound {
				t.Errorf("/broken.png status = %d, want 404", p.Status)
			}
			if len(p.Issues) > 0 {
				t.Errorf("/broken.png has unexpected issues: %v", p.Issues)
			}
		}
		if p.URL == site.URL+"/" {
			for _, iss := range p.Issues {
				if strings.Contains(strings.ToLower(iss), "broken") {
					t.Errorf("homepage has legacy broken image issue: %s", iss)
				}
			}
		}
	}
	if !brokenImgFound {
		t.Error("/broken.png was not persisted in crawled pages")
	}

	// Issue total on the run must remain neutral 0.
	if summary.Issues != 0 {
		t.Errorf("summary.Issues = %d, want 0", summary.Issues)
	}
}

func TestCrawlIssuesOneRequestPerURL(t *testing.T) {
	fastCrawl(t)
	site := newAcquisitionSite(t)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      1,
		MaxFileSizeMB:    DefaultMaxFileSizeMB,
		FollowRedirects:  true,
		CrawlExternal:    false,
		CrawlImages:      false,
		CrawlCSS:         false,
		CrawlJS:          false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}

	// /contact is an ordinary unredirected page: exactly 1 GET, zero HEAD.
	if gets := site.countMethod("/contact", "GET"); gets != 1 {
		t.Errorf("/contact GET count = %d, want exactly 1", gets)
	}
	if heads := site.countMethod("/contact", "HEAD"); heads != 0 {
		t.Errorf("/contact HEAD count = %d, want 0 (no pre-flight HEAD)", heads)
	}

	total := site.totalRequests()
	if total > summary.Crawled*2 {
		t.Errorf("%d total HTTP requests for %d crawled pages, ratio too high", total, summary.Crawled)
	}
}

func TestCrawlRespectsMaxDepth(t *testing.T) {
	fastCrawl(t)
	site := newAcquisitionSite(t)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeSpider,
		MaxDepth:         1,
		Concurrency:      2,
		FollowRedirects:  true,
		CrawlExternal:    false,
		CrawlImages:      false,
		CrawlCSS:         false,
		CrawlJS:          false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}

	var maxDepth int
	if err := db.QueryRow(`SELECT COALESCE(MAX(depth), 0) FROM sitecrawl_pages WHERE run_id = ?`, summary.ID).Scan(&maxDepth); err != nil {
		t.Fatalf("query max depth: %v", err)
	}
	if maxDepth > 1 {
		t.Errorf("max depth = %d, want <= 1", maxDepth)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	for _, p := range pages {
		if strings.Contains(p.URL, "/depth-2") || strings.Contains(p.URL, "/depth-3") {
			t.Errorf("page beyond maxDepth crawled: %s (depth %d)", p.URL, p.Depth)
		}
	}
}

func TestCrawlListModeDoesNotDiscover(t *testing.T) {
	fastCrawl(t)
	site := newAcquisitionSite(t)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		Mode:             ModeList,
		Concurrency:      2,
		FollowRedirects:  true,
		CrawlExternal:    false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	seeds := []string{site.URL + "/about", site.URL + "/contact"}
	summary, err := runner.Crawl(context.Background(), seeds, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("state = %s, want %s", summary.State, StateCompleted)
	}

	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, summary.ID).Scan(&count); err != nil {
		t.Fatalf("count pages: %v", err)
	}
	if count != 2 {
		t.Errorf("list mode crawled %d pages, want exactly 2", count)
	}
}

func TestLinkEvidencePreserved(t *testing.T) {
	fastCrawl(t)
	site := newAcquisitionSite(t)

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
		CrawlImages:      true,
		CrawlCSS:         true,
		CrawlJS:          true,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}

	summary, err := runner.Crawl(context.Background(), []string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	links, err := runner.Links(summary.ID)
	if err != nil {
		t.Fatalf("Links: %v", err)
	}
	if len(links) == 0 {
		t.Fatal("expected persisted links, got 0")
	}

	var sawNav, sawFooter, sawAnchor, sawImage, sawCSS, sawJS bool
	for _, l := range links {
		if l.SrcID <= 0 || l.DstID <= 0 {
			t.Errorf("invalid edge endpoints: SrcID=%d, DstID=%d", l.SrcID, l.DstID)
		}
		if l.Anchor != "" {
			sawAnchor = true
		}
		if l.Placement == PlacementNav {
			sawNav = true
		}
		if l.Placement == PlacementFooter {
			sawFooter = true
		}
		if l.Flags&FlagImageLink != 0 {
			sawImage = true
		}
		if l.Flags&FlagStylesheet != 0 {
			sawCSS = true
		}
		if l.Flags&FlagScript != 0 {
			sawJS = true
		}
	}

	if !sawAnchor {
		t.Error("no anchor text was recorded on any link")
	}
	if !sawNav {
		t.Error("no nav-placed link was recorded")
	}
	if !sawFooter {
		t.Error("no footer-placed link was recorded")
	}
	if !sawImage {
		t.Error("no image link was recorded with FlagImageLink")
	}
	if !sawCSS {
		t.Error("no stylesheet link was recorded with FlagStylesheet")
	}
	if !sawJS {
		t.Error("no script link was recorded with FlagScript")
	}
}

func TestNofollowRecorded(t *testing.T) {
	fastCrawl(t)
	site := newAcquisitionSite(t)

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

	summary, err := runner.Crawl(context.Background(), []string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	var nofollowCount int
	err = db.QueryRow(`
		SELECT COUNT(*)
		  FROM sitecrawl_links l
		  JOIN sitecrawl_urls u ON u.run_id = l.run_id AND u.id = l.dst_id
		 WHERE l.run_id = ? AND u.url LIKE '%/noindex' AND (l.flags & ?) != 0`,
		summary.ID, FlagNofollow).Scan(&nofollowCount)
	if err != nil {
		t.Fatalf("query nofollow link: %v", err)
	}
	if nofollowCount == 0 {
		t.Error("link to /noindex did not record FlagNofollow in sitecrawl_links")
	}
}
