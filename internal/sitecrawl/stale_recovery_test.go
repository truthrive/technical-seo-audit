package sitecrawl

import (
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

// TestRecoverStaleRunsMarksResumable tests recovery of orphaned runs from prior crashed/closed sessions:
// - A running run WITH a saved frontier checkpoint becomes StatePaused and resumable.
// - A running run WITHOUT a saved frontier becomes StateInterrupted (not cancelled).
func TestRecoverStaleRunsMarksResumable(t *testing.T) {
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	opts := Options{}.normalized()

	if err := insertRun(db, "run-with-queue", "https://example.com/", "example.com", opts); err != nil {
		t.Fatalf("insertRun: %v", err)
	}
	if err := insertRun(db, "run-without-queue", "https://example.com/", "example.com", opts); err != nil {
		t.Fatalf("insertRun: %v", err)
	}
	if err := saveFrontier(db, "run-with-queue", []frontierItem{{URL: "https://example.com/a", Depth: 1}}); err != nil {
		t.Fatalf("saveFrontier: %v", err)
	}

	// Trigger stale run recovery
	if err := runner.RecoverStaleRuns(); err != nil {
		t.Fatalf("RecoverStaleRuns: %v", err)
	}

	withQueue, err := runner.LoadRun("run-with-queue")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if withQueue.State != StatePaused || !withQueue.Resumable {
		t.Errorf("run with frontier checkpoint state = %s (resumable = %v), want StatePaused and resumable=true",
			withQueue.State, withQueue.Resumable)
	}

	withoutQueue, err := runner.LoadRun("run-without-queue")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if withoutQueue.State != StateInterrupted {
		t.Errorf("run without frontier checkpoint state = %s, want StateInterrupted", withoutQueue.State)
	}
	if withoutQueue.Resumable {
		t.Errorf("run without frontier checkpoint resumable = %v, want false", withoutQueue.Resumable)
	}
}
