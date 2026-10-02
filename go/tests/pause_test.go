package sitecrawl

import (
	"testing"
	"time"
)

// TestPauseKeepsFrontierAndResumes covers the whole point of a real pause: the
// job goroutine stays alive holding the queue, so resuming continues instead of
// starting over.
func TestPauseKeepsFrontierAndResumes(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, emitter := newTestService(t)

	opts := s.DefaultOptions()
	opts.Concurrency = 1
	// Slow enough that the pause lands mid-crawl rather than after it.
	opts.CrawlDelayMs = 60

	started, err := s.Start([]string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait until some pages exist, then pause.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := s.Status(started.RunID); err == nil && st.Crawled >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := s.Pause(started.RunID); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	waitForRun(t, s, started.RunID, StatePaused, StateCompleted)

	st, _ := s.Status(started.RunID)
	if st.State == StateCompleted {
		t.Skip("crawl finished before the pause landed; nothing to assert")
	}

	// The pause must be visible as a run-state event: job:progress cannot carry
	// it, and a UI keyed on that alone would show a spinner over a stopped crawl.
	states := emitter.runStates()
	sawPaused := false
	for _, s := range states {
		if s == StatePaused {
			sawPaused = true
		}
	}
	if !sawPaused {
		t.Errorf("no paused run-state event emitted; states = %v", states)
	}

	// The queue has to be on disk, or closing the app would lose it.
	db, _ := s.readDB()
	var queued int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_frontier WHERE run_id = ?`, started.RunID).Scan(&queued)
	if queued == 0 {
		t.Error("frontier was not checkpointed on pause")
	}

	crawledAtPause := st.Crawled

	// Resuming the live gate continues the same run.
	if _, err := s.Resume(started.RunID); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	waitForRun(t, s, started.RunID, StateCompleted, StateFailed, StateStopped)

	final, _ := s.Status(started.RunID)
	if final.Crawled <= crawledAtPause {
		t.Errorf("crawled %d after resume vs %d at pause — the run did not continue", final.Crawled, crawledAtPause)
	}

	// And nothing was fetched twice.
	var pages, distinct int
	db.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT url) FROM sitecrawl_pages WHERE run_id = ?`, started.RunID).
		Scan(&pages, &distinct)
	if pages != distinct {
		t.Errorf("%d page rows for %d distinct URLs — the resume re-crawled something", pages, distinct)
	}
}

// TestStopKeepsPartialResults: Stop is not destructive. Whatever the crawl
// found stays, and the frontier is checkpointed so it can be resumed later.
func TestStopKeepsPartialResults(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	opts := s.DefaultOptions()
	opts.Concurrency = 1
	opts.CrawlDelayMs = 60

	started, err := s.Start([]string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := s.Status(started.RunID); err == nil && st.Crawled >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := s.Stop(started.RunID); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	waitForRun(t, s, started.RunID, StateStopped, StateCompleted)

	db, _ := s.readDB()
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, started.RunID).Scan(&n)
	if n == 0 {
		t.Error("stopping the crawl discarded everything it had already found")
	}
}

// TestRecoverStaleRunsMarksResumable: a job dies with the process, so a run row
// still saying "running" at startup is from a previous session. One with a
// checkpointed queue is resumable, not cancelled — the user closed the app
// mid-crawl and expects to pick it back up.
func TestRecoverStaleRunsMarksResumable(t *testing.T) {
	s, db, _ := newTestService(t)

	if err := insertRun(db, "run-with-queue", "https://example.com/", "example.com", s.DefaultOptions()); err != nil {
		t.Fatalf("insertRun: %v", err)
	}
	if err := insertRun(db, "run-without-queue", "https://example.com/", "example.com", s.DefaultOptions()); err != nil {
		t.Fatalf("insertRun: %v", err)
	}
	if err := saveFrontier(db, "run-with-queue", []frontierItem{{URL: "https://example.com/a", Depth: 1}}); err != nil {
		t.Fatalf("saveFrontier: %v", err)
	}

	recoverStaleRuns(db)

	withQueue, err := loadRun(db, "run-with-queue")
	if err != nil {
		t.Fatalf("loadRun: %v", err)
	}
	if withQueue.State != StatePaused || !withQueue.Resumable {
		t.Errorf("run with a checkpoint = %s (resumable %v), want paused and resumable", withQueue.State, withQueue.Resumable)
	}

	// Interrupted, not cancelled. Nobody cancelled this crawl — the app closed
	// on it. Saying "cancelled" told the user they had done something they had
	// not, and it is the one state that has to be distinguishable if history is
	// ever to offer picking a run back up.
	without, _ := loadRun(db, "run-without-queue")
	if without.State != StateInterrupted {
		t.Errorf("run with nothing queued = %s, want interrupted", without.State)
	}
}

// TestResumeFromCheckpointAfterRestart exercises the path where no live gate
// exists: the app was closed, so the run has to be rebuilt from SQLite.
func TestResumeFromCheckpointAfterRestart(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, db, _ := newTestService(t)

	opts := s.DefaultOptions()
	opts.Concurrency = 1
	// Slow enough that stopping after a couple of pages reliably leaves most of
	// the site still queued.
	opts.CrawlDelayMs = 250
	started, err := s.Start([]string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if st, err := s.Status(started.RunID); err == nil && st.Crawled >= 2 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Stop checkpoints the frontier on the way out. Dropping the live handle
	// afterwards is what a process restart looks like from Resume's side.
	s.Stop(started.RunID)
	waitForRun(t, s, started.RunID, StateStopped, StateCompleted)
	s.unregister(started.RunID)

	var queued, before int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_frontier WHERE run_id = ?`, started.RunID).Scan(&queued)
	if queued == 0 {
		t.Skip("crawl drained its queue before the stop landed; nothing to resume")
	}
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, started.RunID).Scan(&before)

	resumed, err := s.Resume(started.RunID)
	if err != nil {
		t.Fatalf("Resume from checkpoint: %v", err)
	}
	if resumed.JobID == resumed.RunID {
		t.Error("a checkpoint resume should get a fresh job id distinct from the run id")
	}
	waitForRun(t, s, started.RunID, StateCompleted, StateFailed, StateStopped)

	after := 0
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, started.RunID).Scan(&after)
	if after <= before {
		t.Errorf("%d pages after the checkpoint resume vs %d before — nothing continued", after, before)
	}

	// The run row's counters must keep counting across the resume: a fresh
	// coordinator once restarted crawled/found from zero and shrank them.
	final, _ := s.Status(started.RunID)
	if final.Crawled != after {
		t.Errorf("run counter says %d crawled but the page table has %d — the resume reset the count", final.Crawled, after)
	}
}
