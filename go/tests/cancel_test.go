package sitecrawl

import (
	"testing"
	"time"

	"onescout/desktop/internal/core/runs"
)

// TestCancelStopsTheCrawlWithinOneSecond measures the thing the user actually
// presses: from Cancel to the run reaching a terminal state.
//
// It is a measurement, not a formality. Threading a context down to every
// Exec on the crawl path is a large change, and it is only worth making if the
// writes are what the Cancel is waiting on. This test says how long the wait is
// with a crawl in full flight.
func TestCancelStopsTheCrawlWithinOneSecond(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	opts := s.DefaultOptions()
	opts.Concurrency = 4
	opts.CrawlDelayMs = 10

	started, err := s.Start([]string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait until pages are actually landing, so the cancel interrupts work in
	// progress rather than a crawl that has not begun.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := s.Status(started.RunID); err == nil && st.Crawled >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if st, _ := s.Status(started.RunID); runs.Terminal(st.State) {
		t.Skip("the fixture crawl finished before the cancel landed; nothing to measure")
	}

	at := time.Now()
	s.Jobs.Cancel(started.JobID)

	for {
		st, err := s.Status(started.RunID)
		if err == nil && runs.Terminal(st.State) {
			break
		}
		if time.Since(at) > 10*time.Second {
			t.Fatalf("the run never stopped: %v after Cancel", time.Since(at))
		}
		time.Sleep(5 * time.Millisecond)
	}
	took := time.Since(at)
	t.Logf("Cancel to terminal state: %v", took)
	if took > time.Second {
		t.Errorf("Cancel took %v to stop the crawl, want under 1s", took)
	}
}
