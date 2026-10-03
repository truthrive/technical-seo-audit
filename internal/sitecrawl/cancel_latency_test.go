package sitecrawl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

// TestCancelStopsTheCrawlWithinOneSecond verifies responsiveness of cancellation:
// From the cancellation/stop request until the run reaches a terminal state
// must take under 1 second on a local deterministic fixture with work in flight.
func TestCancelStopsTheCrawlWithinOneSecond(t *testing.T) {
	fastCrawl(t)
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Fixture site with 30 pages
	mux := http.NewServeMux()
	for i := 0; i < 30; i++ {
		pageID := i
		mux.HandleFunc(fmt.Sprintf("/page-%d", pageID), func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			var links string
			for j := 0; j < 30; j++ {
				links += fmt.Sprintf(`<a href="/page-%d">P%d</a> `, j, j)
			}
			fmt.Fprintf(w, `<!doctype html><html><body><h1>Page %d</h1>%s</body></html>`, pageID, links)
		})
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var links string
		for j := 0; j < 30; j++ {
			links += fmt.Sprintf(`<a href="/page-%d">P%d</a> `, j, j)
		}
		fmt.Fprintf(w, `<!doctype html><html><body><h1>Home</h1>%s</body></html>`, links)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	runner := NewRunner(db)
	opts := Options{
		Concurrency:      4,
		CrawlDelayMs:     10,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}.normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := runner.Start(ctx, []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait until pages are actually landing, proving work is actively in flight
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st, err := runner.Status(handle.RunID())
		if err == nil && st.Crawled >= 3 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	st, _ := runner.Status(handle.RunID())
	if st.State == StateCompleted {
		t.Skip("fixture crawl finished before cancel landed; nothing to measure")
	}

	// Measure cancellation latency
	at := time.Now()
	if err := handle.Stop(); err != nil {
		t.Fatalf("handle.Stop: %v", err)
	}

	_, _ = handle.Wait()
	took := time.Since(at)
	t.Logf("Cancel to terminal state took: %v", took)

	final, err := runner.Status(handle.RunID())
	if err != nil {
		t.Fatalf("Status after cancel: %v", err)
	}
	if final.State != StateStopped {
		t.Errorf("terminal state = %s, want %s", final.State, StateStopped)
	}
	if took > time.Second {
		t.Errorf("Cancel took %v to stop the crawl, want under 1s", took)
	}
}
