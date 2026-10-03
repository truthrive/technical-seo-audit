package sitecrawl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestCrawlDelayIsEnforced is the regression test for LibreCrawl's dead rate
// limiter.
//
// LibreCrawl constructs a RateLimiter and calls update_rate on it, but
// acquire() is only reached from _crawl_async_with_js() — never from
// _crawl_url_with_requests() or _crawl_worker(). On a plain HTTP crawl the
// configured delay therefore does nothing at all, and the user's "be gentle"
// setting is silently ignored.
//
// Here the gate lives in the dispatcher, which is the only place that can
// enforce it.
func TestCrawlDelayIsEnforced(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	const delay = 40 * time.Millisecond
	crawlFixture(t, s, site, func(o *Options) {
		o.Concurrency = 4 // deliberately higher than 1: the gate, not the pool, must pace
		o.CrawlDelayMs = int(delay / time.Millisecond)
		// Turned off so that one dispatch is exactly one HTTP request, which is
		// what makes the gaps measurable. A redirect chain is one dispatch but
		// several requests by design — the same way Screaming Frog treats it —
		// and robots.txt and sitemaps are fetched outside the queue.
		o.FollowRedirects = false
		o.RespectRobots = false
		o.DiscoverSitemaps = false
	})

	log := site.requestLog()
	if len(log) < 5 {
		t.Fatalf("only %d requests recorded, not enough to measure pacing", len(log))
	}

	// Every consecutive pair against the same host must be at least one delay
	// apart, allowing a small tolerance for scheduling jitter.
	const tolerance = 12 * time.Millisecond
	violations := 0
	for i := 1; i < len(log); i++ {
		gap := time.Duration(log[i].at - log[i-1].at)
		if gap+tolerance < delay {
			violations++
		}
	}
	if violations > 0 {
		t.Errorf("%d of %d consecutive request gaps were shorter than the %v crawl delay — the delay is not being applied",
			violations, len(log)-1, delay)
	}
}

// TestPerHostConcurrencyMatchesThreads guards the other half of politeness: one
// host sees exactly the number of simultaneous requests the user asked for.
//
// It used to be capped at a hidden 8, so a crawl of one site — the normal case —
// ran identically at 20 threads and at 50. The number in the Threads field now
// means what it says, and the config dialog warns above 10 instead.
func TestPerHostConcurrencyMatchesThreads(t *testing.T) {
	const threads = 20
	gate := newHostGate(Options{Concurrency: threads}.normalized())
	for i := 0; i < threads; i++ {
		if !gate.ready("example.com") {
			t.Fatalf("host refused request %d of the %d threads configured", i+1, threads)
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

// TestAdaptiveBackoffOn429 checks the behaviour LibreCrawl has no equivalent
// for: a site that says "slow down" gets listened to.
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

	// The penalty doubles while the site keeps refusing.
	before := gate.state("example.com").penalty
	gate.reserve("example.com")
	gate.release("example.com", http.StatusTooManyRequests, "")
	if after := gate.state("example.com").penalty; after <= before {
		t.Errorf("penalty did not grow on a second 429: %v -> %v", before, after)
	}
}

// TestBackoffClearsAfterCleanResponses is the regression test for a crawl that
// never recovered from a blip.
//
// The penalty grew geometrically (0.5s, 1s, 2s, 4s, 8s) but was paid back at a
// flat 250ms per success, and every one of those successes was itself spaced by
// the penalty still in force. Measured on a 111-page crawl of a site that
// answered five requests with 503: 1m57s, against 0.5s with no blip at all.
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
		t.Errorf("penalty is %v after %d clean responses, want it cleared", got, penaltyClearAfter)
	}
	if !gate.ready("example.com") {
		t.Error("host is still holding a cool-off after the penalty cleared")
	}
}

// TestThrottledHostKeepsSomeParallelism: while a penalty is in force the host
// must still take a small burst together. Spacing every single request made the
// user's thread count meaningless for the whole recovery.
func TestThrottledHostKeepsSomeParallelism(t *testing.T) {
	gate := newHostGate(Options{Concurrency: 10}.normalized())
	now := time.Now()
	gate.now = func() time.Time { return now }

	gate.reserve("example.com")
	gate.release("example.com", http.StatusTooManyRequests, "")
	// Step past the cool-off the 429 itself imposed.
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

// TestRobotsCrawlDelayIsCapped: a CDN serving "Crawl-delay: 30" would turn a
// 50k crawl into days. The directive is honoured but bounded, and the raw value
// is kept so the UI can explain the difference instead of silently doing either.
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
		t.Errorf("uncapped value = %v, want the original 30s kept for the UI", got)
	}
}

// TestRobotsCrawlDelayNeedsTheOption: the directive is read either way, but it
// only paces the crawl when the user turned RespectCrawlDelay on.
//
// Honouring it by default is what made this tool visibly slower than Screaming
// Frog, which ignores Crawl-delay outright: the delay spaces every request, so
// a "Crawl-delay: 1" measured 1.1 URL/s where the same crawl otherwise ran at
// 65 URL/s — with every thread but one idle.
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
		c, err := newCoordinator(context.Background(), nil, "run", opts, srv.URL, nil, nil, nil, psiSetup{})
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
		// Read either way, so the status strip can explain the speed.
		if c.robotsDelayAsked != 2*time.Second {
			t.Errorf("RespectCrawlDelay=%v: robots.txt asked %v, want it recorded as 2s",
				respect, c.robotsDelayAsked)
		}
	}
}

// TestRobotsNamedGroupBeatsWildcard: a group naming our agent wins over "*".
func TestRobotsCrawlDelayNamedGroupWins(t *testing.T) {
	raw := []byte("User-agent: *\nCrawl-delay: 10\n\nUser-agent: 1ScoutSiteCrawl\nCrawl-delay: 2\n")
	got := parseCrawlDelay(raw, []string{"1ScoutSiteCrawl"})
	if got != 2*time.Second {
		t.Errorf("Crawl-delay = %v, want the named group's 2s", got)
	}
}

// TestParseSitemapLinesKeepsScheme guards the field-separator split.
//
// (The original recon claimed LibreCrawl loses the scheme here; reading its
// source shows `line.split(':', 1)[1]` keeps it, so this is not a regression
// test for a real bug — just a guard on our own parser.)
func TestParseSitemapLinesKeepsScheme(t *testing.T) {
	raw := []byte("User-agent: *\nDisallow: /x\nSitemap: https://example.com/sitemap.xml\n# Sitemap: https://example.com/commented.xml\n")
	got := parseSitemapLines(raw)
	if len(got) != 1 {
		t.Fatalf("got %v, want exactly one sitemap (the commented line must be skipped)", got)
	}
	if got[0] != "https://example.com/sitemap.xml" {
		t.Errorf("sitemap = %q, want the scheme intact", got[0])
	}
}
