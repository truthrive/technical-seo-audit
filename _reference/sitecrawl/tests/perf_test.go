package sitecrawl

// Throughput harness. Gated behind CRAWL_PERF because it takes ~30s and its
// numbers depend on the machine — the correctness guards for everything it
// measures live in politeness_test.go and run normally.
//
// It exists because "the crawl feels slow" is unanswerable without numbers.
// Baseline on the fixture tree (1112 pages, ~40KB each, 10 links per page),
// before and after the 2026-08-07 pass:
//
//	                             before      after
//	remote 50ms, 5 threads        84/s        87/s
//	remote 50ms, 20 threads      127/s       273/s   (was capped at 8 in flight)
//	remote 50ms, 50 threads      128/s       499/s
//	robots.txt Crawl-delay: 1    1.1/s        73/s   (directive now opt-in)
//	after five 503 responses     0.5/s       242/s   (backoff cleared, not decayed)

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type synthSite struct {
	*httptest.Server
	conns atomic.Int64
	reqs  atomic.Int64
	// concurrent request tracking, to see how many the crawler actually keeps
	// in flight against one host.
	inFlight atomic.Int64
	peak     atomic.Int64
}

// newSynthSite serves a fanout-10 tree of pages, each ~40KB with 10 child links
// plus the usual nav/footer noise. latency is added to every response to
// simulate a real remote host.
func newSynthSite(t *testing.T, latency time.Duration) *synthSite {
	return newSynthSiteRobots(t, latency, "", 0)
}

// newSynthSiteRobots adds two knobs: a Crawl-delay directive in robots.txt, and
// a number of early requests answered with 503 to trip the adaptive backoff.
func newSynthSiteRobots(t *testing.T, latency time.Duration, crawlDelay string, blips int) *synthSite {
	t.Helper()
	s := &synthSite{}
	body := strings.Repeat("alpha beta gamma delta epsilon zeta eta theta iota kappa ", 700) // ~39KB
	var served atomic.Int64

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		s.reqs.Add(1)
		if r.URL.Path == "/robots.txt" {
			if crawlDelay == "" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprintf(w, "User-agent: *\nCrawl-delay: %s\n", crawlDelay)
			return
		}
		// The 503 burst hits the first few child pages — never the seed — the way
		// a site answers when a crawl briefly outruns it.
		if blips > 0 && strings.HasPrefix(r.URL.Path, "/p/") && served.Add(1) <= int64(blips) {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		n := s.inFlight.Add(1)
		for {
			p := s.peak.Load()
			if n <= p || s.peak.CompareAndSwap(p, n) {
				break
			}
		}
		defer s.inFlight.Add(-1)
		if latency > 0 {
			time.Sleep(latency)
		}
		if r.URL.Path == "/sitemap.xml" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := 0
		if strings.HasPrefix(r.URL.Path, "/p/") {
			id, _ = strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/p/"))
		}
		var links strings.Builder
		for i := 1; i <= 10; i++ {
			fmt.Fprintf(&links, `<a href="/p/%d">child %d</a>`, id*10+i, i)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html lang="en"><head><meta charset="utf-8">
<title>Synthetic page %d for the crawl benchmark</title>
<meta name="description" content="A synthetic page used to measure crawl throughput, long enough to pass the meta rule.">
<link rel="canonical" href="%s/p/%d"></head><body>
<header><a href="/">Home</a></header><main><h1>Page %d</h1><h2>Section</h2><p>%s</p>%s</main></body></html>`,
			id, s.URL, id, id, body, links.String())
	})

	srv := httptest.NewUnstartedServer(mux)
	srv.Config.ConnState = func(_ net.Conn, st http.ConnState) {
		if st == http.StateNew {
			s.conns.Add(1)
		}
	}
	srv.Start()
	s.Server = srv
	t.Cleanup(srv.Close)
	return s
}

func runPerf(t *testing.T, name string, latency time.Duration, concurrency, maxDepth int) {
	t.Helper()
	site := newSynthSite(t, latency)
	svc, _, _ := newTestService(t)

	opts := svc.DefaultOptions()
	opts.Concurrency = concurrency
	opts.MaxDepth = maxDepth
	opts.CrawlExternal = false
	opts.EnableDuplication = false // isolate the crawl phase from finalize

	start := time.Now()
	started, err := svc.Start([]string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, svc, started.RunID, StateCompleted, StateFailed, StateStopped)
	elapsed := time.Since(start)

	st, _ := svc.Status(started.RunID)
	reqs := site.reqs.Load()
	conns := site.conns.Load()
	t.Logf("%-28s pages=%-5d elapsed=%-8s rate=%6.1f/s reqs=%-5d conns=%-5d reqs/conn=%4.1f peakParallel=%d",
		name, st.Crawled, elapsed.Round(time.Millisecond),
		float64(st.Crawled)/elapsed.Seconds(), reqs, conns,
		float64(reqs)/float64(max64(conns, 1)), site.peak.Load())
}

// waitLong is waitForRun with a deadline long enough for a throttled crawl.
func waitLong(t *testing.T, s *Service, runID string, deadline time.Duration) {
	t.Helper()
	until := time.Now().Add(deadline)
	for time.Now().Before(until) {
		st, err := s.Status(runID)
		if err == nil {
			switch st.State {
			case StateCompleted, StateFailed, StateStopped:
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("run %s never finished within %s", runID, deadline)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func TestCrawlPerf(t *testing.T) {
	if os.Getenv("CRAWL_PERF") == "" {
		t.Skip("set CRAWL_PERF=1 to run the crawl throughput harness")
	}
	// depth 3 over fanout 10 = 1111 pages.
	runPerf(t, "local, 5 threads, 0ms", 0, 5, 3)
	runPerf(t, "local, 20 threads, 0ms", 0, 20, 3)
	runPerf(t, "remote 50ms, 5 threads", 50*time.Millisecond, 5, 3)
	runPerf(t, "remote 50ms, 20 threads", 50*time.Millisecond, 20, 3)
	runPerf(t, "remote 50ms, 50 threads", 50*time.Millisecond, 50, 3)
}

// TestCrawlPerfRobotsDelay measures what a robots.txt Crawl-delay costs. This is
// the directive Screaming Frog ignores outright.
func TestCrawlPerfRobotsDelay(t *testing.T) {
	if os.Getenv("CRAWL_PERF") == "" {
		t.Skip("set CRAWL_PERF=1")
	}
	type run struct {
		delay   string
		respect bool
	}
	for _, r := range []run{{"", false}, {"1", false}, {"0.5", true}, {"1", true}} {
		site := newSynthSiteRobots(t, 20*time.Millisecond, r.delay, 0)
		svc, _, _ := newTestService(t)
		opts := svc.DefaultOptions()
		opts.Concurrency = 20
		opts.MaxDepth = 1 // 11 pages — a delayed crawl of 111 would take minutes
		opts.CrawlExternal = false
		opts.EnableDuplication = false
		opts.RespectCrawlDelay = r.respect
		delay := r.delay

		start := time.Now()
		started, err := svc.Start([]string{site.URL}, opts)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		waitLong(t, svc, started.RunID, 3*time.Minute)
		elapsed := time.Since(start)
		st, _ := svc.Status(started.RunID)
		label := delay
		if label == "" {
			label = "(none)"
		}
		t.Logf("Crawl-delay=%-7s respect=%-5v pages=%-4d elapsed=%-9s rate=%6.1f/s peakParallel=%d",
			label, r.respect, st.Crawled, elapsed.Round(time.Millisecond),
			float64(st.Crawled)/elapsed.Seconds(), site.peak.Load())
	}
}

// TestCrawlPerfPenalty measures how long the adaptive backoff keeps a crawl slow
// after a short burst of 503s — the shape of a site under momentary load.
func TestCrawlPerfPenalty(t *testing.T) {
	if os.Getenv("CRAWL_PERF") == "" {
		t.Skip("set CRAWL_PERF=1")
	}
	for _, blips := range []int{0, 1, 3, 5} {
		site := newSynthSiteRobots(t, 20*time.Millisecond, "", blips)
		svc, _, _ := newTestService(t)
		opts := svc.DefaultOptions()
		opts.Concurrency = 20
		opts.MaxDepth = 2 // 111 pages
		opts.CrawlExternal = false
		opts.EnableDuplication = false

		start := time.Now()
		started, err := svc.Start([]string{site.URL}, opts)
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		waitLong(t, svc, started.RunID, 5*time.Minute)
		elapsed := time.Since(start)
		st, _ := svc.Status(started.RunID)
		t.Logf("503 blips=%-2d  pages=%-4d elapsed=%-9s rate=%6.1f/s",
			blips, st.Crawled, elapsed.Round(time.Millisecond),
			float64(st.Crawled)/elapsed.Seconds())
	}
}
