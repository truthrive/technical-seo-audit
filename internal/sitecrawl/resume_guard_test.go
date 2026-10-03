package sitecrawl

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

func setupResumableRun(t *testing.T, runID string, opts Options) (*Runner, string) {
	t.Helper()
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	runner := NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	if err := insertRun(db, runID, "https://example.com/", "example.com", opts); err != nil {
		t.Fatalf("insertRun: %v", err)
	}
	pauseRun(db, runID) // mark as StatePaused, resumable=1

	items := []frontierItem{
		{URL: "https://example.com/page1", Depth: 1, Source: "link"},
	}
	if err := saveFrontier(db, runID, items); err != nil {
		t.Fatalf("saveFrontier: %v", err)
	}

	return runner, runID
}

// TestResumePageSpeedGuard verifies that resuming a persisted run with EnablePageSpeed=true
// is blocked with ErrCapabilityUnsupported, leaves the run state and frontier intact,
// and does not register an active handle.
func TestResumePageSpeedGuard(t *testing.T) {
	opts := Options{
		EnablePageSpeed: true,
	}
	runner, runID := setupResumableRun(t, "run-psi-guard", opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err == nil {
		t.Fatal("expected Resume to fail for EnablePageSpeed=true, got nil")
	}
	if !errors.Is(err, ErrCapabilityUnsupported) {
		t.Errorf("got error %v, want wrapping ErrCapabilityUnsupported", err)
	}
	if handle != nil {
		t.Errorf("handle = %v, want nil on failed validation", handle)
	}

	// Verify run state remains StatePaused
	run, err := runner.LoadRun(runID)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if run.State != StatePaused {
		t.Errorf("run state mutated to %s on failed resume, want %s", run.State, StatePaused)
	}
	if !run.Resumable {
		t.Error("run.Resumable was mutated to false, want true")
	}

	// Verify frontier remains intact
	items, err := loadFrontier(runner.DB, runID)
	if err != nil {
		t.Fatalf("loadFrontier: %v", err)
	}
	if len(items) == 0 {
		t.Error("frontier was cleared on failed resume validation")
	}

	// Verify no active handle registered
	if runner.Handle(runID) != nil {
		t.Error("active handle was registered despite failed validation")
	}
}

// TestResumeDuplicateAnalysisGuard verifies that resuming a persisted run with EnableDuplication=true
// is blocked with ErrCapabilityUnsupported, leaves run state and frontier intact, and registers no handle.
func TestResumeDuplicateAnalysisGuard(t *testing.T) {
	opts := Options{
		EnableDuplication: true,
	}
	runner, runID := setupResumableRun(t, "run-dupe-guard", opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err == nil {
		t.Fatal("expected Resume to fail for EnableDuplication=true, got nil")
	}
	if !errors.Is(err, ErrCapabilityUnsupported) {
		t.Errorf("got error %v, want wrapping ErrCapabilityUnsupported", err)
	}
	if handle != nil {
		t.Errorf("handle = %v, want nil on failed validation", handle)
	}

	run, err := runner.LoadRun(runID)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if run.State != StatePaused {
		t.Errorf("run state mutated to %s on failed resume, want %s", run.State, StatePaused)
	}

	items, err := loadFrontier(runner.DB, runID)
	if err != nil {
		t.Fatalf("loadFrontier: %v", err)
	}
	if len(items) == 0 {
		t.Error("frontier was cleared on failed resume validation")
	}

	if runner.Handle(runID) != nil {
		t.Error("active handle was registered despite failed validation")
	}
}

// TestResumeProxyGuard verifies that resuming a persisted run with UseProxy=true
// on a Runner without a configured proxy function fails with ErrProxyUnavailable,
// leaving run state and frontier untouched.
func TestResumeProxyGuard(t *testing.T) {
	opts := Options{
		UseProxy: true,
	}
	runner, runID := setupResumableRun(t, "run-proxy-guard", opts)
	runner.Proxy = nil // ensure no proxy configured

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err == nil {
		t.Fatal("expected Resume to fail for UseProxy=true with nil Proxy, got nil")
	}
	if !errors.Is(err, ErrProxyUnavailable) {
		t.Errorf("got error %v, want ErrProxyUnavailable", err)
	}
	if handle != nil {
		t.Errorf("handle = %v, want nil on failed validation", handle)
	}

	run, err := runner.LoadRun(runID)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if run.State != StatePaused {
		t.Errorf("run state mutated to %s on failed resume, want %s", run.State, StatePaused)
	}

	items, err := loadFrontier(runner.DB, runID)
	if err != nil {
		t.Fatalf("loadFrontier: %v", err)
	}
	if len(items) == 0 {
		t.Error("frontier was cleared on failed resume validation")
	}

	if runner.Handle(runID) != nil {
		t.Error("active handle was registered despite failed validation")
	}
}

// TestResumeProxyAvailable verifies that a persisted UseProxy=true run passes
// the capability gate when resumed on a Runner with a configured proxy function.
func TestResumeProxyAvailable(t *testing.T) {
	opts := Options{
		UseProxy: true,
	}
	runner, runID := setupResumableRun(t, "run-proxy-available", opts)

	// Configure a non-nil proxy function
	runner.Proxy = func() *url.URL {
		return nil // direct connection
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err != nil {
		t.Fatalf("expected Resume to pass capability gate with configured Proxy, got err: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle after successful Resume")
	}

	// Verify handle registered
	if runner.Handle(runID) == nil {
		t.Error("expected active handle to be registered")
	}

	_ = handle.Stop()
	_, _ = handle.Wait()
}
