package sitecrawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

type politenessHit struct {
	path string
	at   int64 // nanoseconds since test server started
}

type politenessSite struct {
	*httptest.Server
	mu    sync.Mutex
	order []politenessHit
}

func (s *politenessSite) log() []politenessHit {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]politenessHit, len(s.order))
	copy(out, s.order)
	return out
}

func newPolitenessSite(t *testing.T) *politenessSite {
	t.Helper()
	site := &politenessSite{}
	start := time.Now()

	mux := http.NewServeMux()
	site.Server = httptest.NewServer(mux)
	t.Cleanup(site.Close)

	record := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			site.mu.Lock()
			site.order = append(site.order, politenessHit{
				path: r.URL.Path,
				at:   int64(time.Since(start)),
			})
			site.mu.Unlock()
			h(w, r)
		}
	}

	html := func(w http.ResponseWriter, body string) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(body))
	}

	mux.HandleFunc("/", record(func(w http.ResponseWriter, r *http.Request) {
		html(w, `<!doctype html><html><body>
			<a href="/p1">P1</a><a href="/p2">P2</a><a href="/p3">P3</a><a href="/p4">P4</a>
			<a href="/p5">P5</a><a href="/p6">P6</a><a href="/p7">P7</a><a href="/p8">P8</a>
		</body></html>`)
	}))

	for i := 1; i <= 8; i++ {
		p := fmt.Sprintf("/p%d", i)
		mux.HandleFunc(p, record(func(w http.ResponseWriter, r *http.Request) {
			html(w, "<!doctype html><html><body>Page</body></html>")
		}))
	}

	return site
}

func TestCrawlDelayIsEnforced(t *testing.T) {
	site := newPolitenessSite(t)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	const delay = 40 * time.Millisecond
	opts := Options{
		Mode:             ModeSpider,
		Concurrency:      4,
		CrawlDelayMs:     int(delay / time.Millisecond),
		FollowRedirects:  false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
		CrawlImages:      false,
		CrawlCSS:         false,
		CrawlJS:          false,
	}

	summary, err := runner.Crawl(context.Background(), []string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}
	if summary.State != StateCompleted {
		t.Fatalf("run state = %s, want %s", summary.State, StateCompleted)
	}

	log := site.log()
	if len(log) < 5 {
		t.Fatalf("only %d requests recorded, not enough to measure pacing", len(log))
	}

	const tolerance = 15 * time.Millisecond
	violations := 0
	for i := 1; i < len(log); i++ {
		gap := time.Duration(log[i].at - log[i-1].at)
		if gap+tolerance < delay {
			violations++
		}
	}
	if violations > 0 {
		t.Errorf("%d of %d consecutive request gaps were shorter than %v delay", violations, len(log)-1, delay)
	}
}

func TestPerHostConcurrencyMatchesThreads(t *testing.T) {
	const threads = 20
	gate := newHostGate(Options{Concurrency: threads}.normalized())
	for i := 0; i < threads; i++ {
		if !gate.ready("example.com") {
			t.Fatalf("host refused request %d of %d threads configured", i+1, threads)
		}
		gate.reserve("example.com")
	}
	if gate.ready("example.com") {
		t.Errorf("host accepted more than the %d simultaneous requests configured", threads)
	}
	gate.release("example.com", 200, "")
	if !gate.ready("example.com") {
		t.Error("host still refusing after a request completed")
	}
}

func TestAdaptiveBackoffOn429(t *testing.T) {
	gate := newHostGate(Options{}.normalized())
	now := time.Now()
	gate.now = func() time.Time { return now }

	gate.reserve("example.com")
	if !gate.ready("example.com") {
		t.Fatal("host not ready with no delay configured")
	}
	gate.release("example.com", http.StatusTooManyRequests, "")
	if gate.ready("example.com") {
		t.Error("host is still ready immediately after a 429 — no backoff applied")
	}

	before := gate.state("example.com").penalty
	gate.reserve("example.com")
	gate.release("example.com", http.StatusTooManyRequests, "")
	if after := gate.state("example.com").penalty; after <= before {
		t.Errorf("penalty did not grow on a second 429: %v -> %v", before, after)
	}
}

func TestAdaptiveBackoffOn503(t *testing.T) {
	gate := newHostGate(Options{}.normalized())
	now := time.Now()
	gate.now = func() time.Time { return now }

	gate.reserve("example.com")
	gate.release("example.com", http.StatusServiceUnavailable, "")
	if gate.ready("example.com") {
		t.Error("host is still ready immediately after a 503 — no backoff applied")
	}

	before := gate.state("example.com").penalty
	gate.reserve("example.com")
	gate.release("example.com", http.StatusServiceUnavailable, "")
	if after := gate.state("example.com").penalty; after <= before {
		t.Errorf("penalty did not grow on a second 503: %v -> %v", before, after)
	}
}

func TestBackoffClearsAfterCleanResponses(t *testing.T) {
	gate := newHostGate(Options{Concurrency: 10}.normalized())
	now := time.Now()
	gate.now = func() time.Time { return now }

	for i := 0; i < 5; i++ {
		gate.reserve("example.com")
		gate.release("example.com", http.StatusServiceUnavailable, "")
	}
	if gate.state("example.com").penalty == 0 {
		t.Fatal("five 503s left no penalty at all")
	}

	for i := 0; i < penaltyClearAfter; i++ {
		gate.reserve("example.com")
		gate.release("example.com", 200, "")
	}
	if got := gate.state("example.com").penalty; got != 0 {
		t.Errorf("penalty is %v after %d clean responses, want 0", got, penaltyClearAfter)
	}
	if !gate.ready("example.com") {
		t.Error("host is still holding a cool-off after penalty cleared")
	}
}

func TestThrottledHostKeepsSomeParallelism(t *testing.T) {
	gate := newHostGate(Options{Concurrency: 10}.normalized())
	now := time.Now()
	gate.now = func() time.Time { return now }

	gate.reserve("example.com")
	gate.release("example.com", http.StatusTooManyRequests, "")
	now = now.Add(gate.state("example.com").penalty)

	for i := 0; i < throttleBurst; i++ {
		if !gate.ready("example.com") {
			t.Fatalf("throttled host refused request %d of a %d-request burst", i+1, throttleBurst)
		}
		gate.reserve("example.com")
	}
	if gate.ready("example.com") {
		t.Error("throttled host took more than the burst without waiting out the penalty")
	}
}

func TestRobotsCrawlDelayIsCapped(t *testing.T) {
	raw := []byte("User-agent: *\nCrawl-delay: 30\n")
	asked := parseCrawlDelay(raw, []string{"1ScoutSiteCrawl"})
	if asked != 30*time.Second {
		t.Fatalf("parsed Crawl-delay = %v, want 30s", asked)
	}

	gate := newHostGate(Options{}.normalized())
	capped := asked
	if capped > maxRobotsDelayMs*time.Millisecond {
		capped = maxRobotsDelayMs * time.Millisecond
	}
	gate.setRobotsDelay("example.com", capped, asked)

	if got := gate.state("example.com").delay; got != maxRobotsDelayMs*time.Millisecond {
		t.Errorf("applied delay = %v, want it capped at %v", got, maxRobotsDelayMs*time.Millisecond)
	}
	if got := gate.robotsAsked("example.com"); got != 30*time.Second {
		t.Errorf("uncapped value = %v, want the original 30s kept", got)
	}
}

func TestRobotsCrawlDelayNeedsTheOption(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("User-agent: *\nCrawl-delay: 2\n"))
	}))
	defer srv.Close()

	for _, respect := range []bool{false, true} {
		opts := Options{RespectRobots: true, RespectCrawlDelay: respect}.normalized()
		c, err := newCoordinator(context.Background(), nil, "run", opts, srv.URL, nil, nil, nil)
		if err != nil {
			t.Fatalf("newCoordinator: %v", err)
		}
		if err := c.prepare(context.Background(), []string{srv.URL}); err != nil {
			t.Fatalf("prepare: %v", err)
		}

		got := c.hosts.state(strings.ToLower(hostOf(srv.URL))).delay
		want := time.Duration(0)
		if respect {
			want = 2 * time.Second
		}
		if got != want {
			t.Errorf("RespectCrawlDelay=%v: pacing delay is %v, want %v", respect, got, want)
		}
		if c.robotsDelayAsked != 2*time.Second {
			t.Errorf("RespectCrawlDelay=%v: robots asked %v, want 2s", respect, c.robotsDelayAsked)
		}
	}
}

func TestRobotsCrawlDelayNamedGroupWins(t *testing.T) {
	raw := []byte("User-agent: *\nCrawl-delay: 10\n\nUser-agent: 1ScoutSiteCrawl\nCrawl-delay: 2\n")
	got := parseCrawlDelay(raw, []string{"1ScoutSiteCrawl"})
	if got != 2*time.Second {
		t.Errorf("Crawl-delay = %v, want named group's 2s", got)
	}
}

func TestParseSitemapLinesKeepsScheme(t *testing.T) {
	raw := []byte("User-agent: *\nDisallow: /x\nSitemap: https://example.com/sitemap.xml\n# Sitemap: https://example.com/commented.xml\n")
	got := parseSitemapLines(raw)
	if len(got) != 1 {
		t.Fatalf("got %v, want exactly 1 sitemap", got)
	}
	if got[0] != "https://example.com/sitemap.xml" {
		t.Errorf("sitemap = %q, want scheme intact", got[0])
	}
}
