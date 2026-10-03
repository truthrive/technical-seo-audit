package sitecrawl

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

// TestResumeFromCheckpointAfterRestart tests process restart resume:
// The crawl is stopped mid-way, the database is completely closed, a fresh
// DB handle and Runner are opened (simulating a new process invocation), and
// the original run is resumed from its SQLite checkpoint.
func TestResumeFromCheckpointAfterRestart(t *testing.T) {
	fastCrawl(t)
	dbPath := filepath.Join(t.TempDir(), "resume_restart.db")
	srv := newMultiPageSite(t)

	// Step 1: Start crawl in first session
	db1, err := standalone.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB session 1: %v", err)
	}

	runner1 := NewRunner(db1)
	opts := Options{
		Concurrency:      1,
		CrawlDelayMs:     80, // slow enough to leave pages in queue
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}.normalized()

	ctx1, cancel1 := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel1()

	handle1, err := runner1.Start(ctx1, []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start session 1: %v", err)
	}
	runID := handle1.RunID()

	// Wait until at least 2 pages have landed
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := runner1.Status(runID); err == nil && st.Crawled >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Stop the crawl, checkpointing remaining work
	if err := handle1.Stop(); err != nil {
		t.Fatalf("Stop session 1: %v", err)
	}
	_, _ = handle1.Wait()

	// Confirm frontier checkpoint exists in DB
	var queued, before int
	db1.QueryRow(`SELECT COUNT(*) FROM sitecrawl_frontier WHERE run_id = ?`, runID).Scan(&queued)
	if queued == 0 {
		t.Skip("crawl drained queue before stop landed; nothing to resume")
	}
	db1.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, runID).Scan(&before)

	// Step 2: Simulate process exit by closing the database completely
	if err := db1.Close(); err != nil {
		t.Fatalf("Close db1: %v", err)
	}

	// Step 3: Simulate new process startup with a fresh DB handle and fresh Runner
	db2, err := standalone.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("OpenDB session 2: %v", err)
	}
	defer db2.Close()

	runner2 := NewRunner(db2)

	// Step 4: Resume original run from checkpoint
	ctx2, cancel2 := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel2()

	handle2, err := runner2.Resume(ctx2, runID)
	if err != nil {
		t.Fatalf("Resume session 2: %v", err)
	}
	if handle2.RunID() != runID {
		t.Errorf("resumed run ID = %s, want %s (must not create a second run)", handle2.RunID(), runID)
	}

	summary, err := handle2.Wait()
	if err != nil && err != context.Canceled {
		t.Fatalf("Wait session 2: %v", err)
	}
	if summary == nil {
		t.Fatal("expected non-nil summary after resume")
	}

	// Step 5: Verify existing and new pages coexist
	var after int
	db2.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, runID).Scan(&after)
	if after <= before {
		t.Errorf("%d pages after checkpoint resume vs %d before — crawl did not continue", after, before)
	}

	// Verify counters continue rather than restart from zero
	final, err := runner2.Status(runID)
	if err != nil {
		t.Fatalf("Status session 2: %v", err)
	}
	if final.Crawled != after {
		t.Errorf("run counter crawled = %d, but page table has %d (resume reset count)", final.Crawled, after)
	}
	if final.State != StateCompleted {
		t.Errorf("final state = %s, want %s", final.State, StateCompleted)
	}

	// Verify URLs are distinct
	var distinct int
	db2.QueryRow(`SELECT COUNT(DISTINCT url) FROM sitecrawl_pages WHERE run_id = ?`, runID).Scan(&distinct)
	if after != distinct {
		t.Errorf("%d total page rows for %d distinct URLs — resume recrawled something", after, distinct)
	}
}
