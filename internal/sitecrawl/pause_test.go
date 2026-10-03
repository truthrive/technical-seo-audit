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

// newMultiPageSite creates a hermetic fixture site with 10 interlinked pages.
func newMultiPageSite(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	var srv *httptest.Server

	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		path := r.URL.Path
		if path == "/" {
			path = "/page-0"
		}
		var links string
		for i := 0; i < 10; i++ {
			links += fmt.Sprintf(`<a href="/page-%d">Page %d</a> `, i, i)
		}
		fmt.Fprintf(w, `<!doctype html><html><head><title>Title for %s</title></head><body><h1>Heading %s</h1>%s</body></html>`, path, path, links)
	}

	for i := 0; i < 10; i++ {
		mux.HandleFunc(fmt.Sprintf("/page-%d", i), handler)
	}
	mux.HandleFunc("/", handler)

	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestPauseKeepsFrontierAndResumes(t *testing.T) {
	fastCrawl(t)
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()
	srv := newMultiPageSite(t)

	collector := standalone.NewEventCollector()
	runner := NewRunner(db)
	runner.Events = collector

	opts := Options{
		Concurrency:      1,
		CrawlDelayMs:     60, // slow enough that pause lands mid-crawl
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}.normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	handle, err := runner.Start(ctx, []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait until at least 2 pages have landed, then pause.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := runner.Status(handle.RunID()); err == nil && st.Crawled >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := handle.Pause(); err != nil {
		t.Fatalf("handle.Pause: %v", err)
	}

	// Wait for the pause to settle
	pauseDeadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(pauseDeadline) {
		st, _ := runner.Status(handle.RunID())
		if st.State == StatePaused || st.State == StateCompleted {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	st, err := runner.Status(handle.RunID())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st.State == StateCompleted {
		t.Skip("crawl completed before pause could land; nothing to assert")
	}
	if st.State != StatePaused {
		t.Fatalf("state at pause = %s, want %s", st.State, StatePaused)
	}

	// Check that a paused run-state event was emitted
	sawPaused := false
	for _, ev := range collector.Events() {
		if ev.Name == EventRunState {
			if rs, ok := ev.Data.(RunStateEvent); ok && rs.State == StatePaused {
				sawPaused = true
				break
			}
		}
	}
	if !sawPaused {
		t.Errorf("no StatePaused event emitted; got events = %v", collector.Events())
	}

	// Queue must be checkpointed in SQLite
	var queued int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_frontier WHERE run_id = ?`, handle.RunID()).Scan(&queued); err != nil {
		t.Fatalf("query frontier: %v", err)
	}
	if queued == 0 {
		t.Error("frontier was not checkpointed on pause")
	}

	crawledAtPause := st.Crawled

	// Ensure no new work is dispatched while paused
	time.Sleep(150 * time.Millisecond)
	stAfterPause, _ := runner.Status(handle.RunID())
	if stAfterPause.Crawled > crawledAtPause+1 { // at most one in-flight request finishes
		t.Errorf("crawled increased from %d to %d while paused", crawledAtPause, stAfterPause.Crawled)
	}

	// Resume the live coordinator
	if err := handle.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	summary, err := handle.Wait()
	if err != nil && err != context.Canceled {
		t.Fatalf("Wait after resume: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary after Wait")
	}

	final, _ := runner.Status(handle.RunID())
	if final.Crawled <= crawledAtPause {
		t.Errorf("crawled %d after resume vs %d at pause — the run did not continue", final.Crawled, crawledAtPause)
	}

	// Verify nothing was fetched or stored twice
	var pages, distinct int
	db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT url) FROM sitecrawl_pages WHERE run_id = ?`, handle.RunID()).Scan(&pages, &distinct)
	if pages != distinct {
		t.Errorf("%d page rows for %d distinct URLs — resume recrawled something", pages, distinct)
	}
}

func TestStopKeepsPartialResults(t *testing.T) {
	fastCrawl(t)
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()
	srv := newMultiPageSite(t)

	runner := NewRunner(db)
	opts := Options{
		Concurrency:      1,
		CrawlDelayMs:     60,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}.normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	handle, err := runner.Start(ctx, []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := runner.Status(handle.RunID()); err == nil && st.Crawled >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if err := handle.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	summary, _ := handle.Wait()
	if summary == nil {
		s, err := runner.Status(handle.RunID())
		if err != nil {
			t.Fatalf("Status after stop: %v", err)
		}
		summary = &s
	}

	if summary.State != StateStopped {
		t.Errorf("terminal state = %s, want %s", summary.State, StateStopped)
	}
	if summary.StopReason != StopUser {
		t.Errorf("stop reason = %s, want %s", summary.StopReason, StopUser)
	}

	var pagesCount int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, handle.RunID()).Scan(&pagesCount)
	if pagesCount == 0 {
		t.Error("Stop discarded existing completed pages")
	}

	var urlsCount int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_urls WHERE run_id = ?`, handle.RunID()).Scan(&urlsCount)
	if urlsCount == 0 {
		t.Error("Stop discarded URL dictionary")
	}

	var frontierCount int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_frontier WHERE run_id = ?`, handle.RunID()).Scan(&frontierCount)
	if frontierCount == 0 {
		t.Error("Stop did not checkpoint remaining frontier")
	}
	if !summary.Resumable {
		t.Error("summary.Resumable should be true when remaining frontier was checkpointed")
	}
}
