package sitecrawl

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

func setupTestRun(t *testing.T, runID, seedURL string, opts Options) (*Runner, string) {
	t.Helper()
	if seedURL == "" {
		seedURL = "http://127.0.0.1:0/"
	}
	u, _ := url.Parse(seedURL)
	host := u.Hostname()
	if host == "" {
		host = "127.0.0.1"
	}

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	runner := NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	if err := insertRun(db, runID, seedURL, host, opts); err != nil {
		t.Fatalf("insertRun: %v", err)
	}

	return runner, runID
}

func addFrontierItems(t *testing.T, runner *Runner, runID, seedURL string) {
	t.Helper()
	items := []frontierItem{
		{URL: seedURL + "page1", Depth: 1, Source: "link"},
	}
	if err := saveFrontier(runner.DB, runID, items); err != nil {
		t.Fatalf("saveFrontier: %v", err)
	}
}

// TestResumeEligibility_CompletedProhibited verifies that a completed run cannot be resumed,
// does not mutate state or clear the frontier, and registers no handle.
func TestResumeEligibility_CompletedProhibited(t *testing.T) {
	runner, runID := setupTestRun(t, "run-completed", "http://127.0.0.1:8080/", Options{})
	finishRun(runner.DB, runID, StateCompleted, "", "", false, time.Now())
	addFrontierItems(t, runner, runID, "http://127.0.0.1:8080/")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err == nil {
		t.Fatal("expected Resume to fail for completed run, got nil error")
	}
	if !errors.Is(err, ErrRunNotResumable) {
		t.Errorf("got error %v, want wrapping ErrRunNotResumable", err)
	}
	if handle != nil {
		t.Errorf("expected nil handle on rejected resume, got %v", handle)
	}

	// Verify state and resumable flag unchanged
	run, err := runner.LoadRun(runID)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if run.State != StateCompleted {
		t.Errorf("run state mutated to %s, want %s", run.State, StateCompleted)
	}
	if run.Resumable {
		t.Error("run.Resumable mutated to true, want false")
	}

	// Verify frontier not cleared
	items, err := loadFrontier(runner.DB, runID)
	if err != nil {
		t.Fatalf("loadFrontier: %v", err)
	}
	if len(items) == 0 {
		t.Error("frontier was cleared on rejected resume")
	}

	// Verify no handle registered
	if runner.Handle(runID) != nil {
		t.Error("active handle registered on rejected resume")
	}
}

// TestResumeEligibility_NonResumableStoppedProhibited verifies that a stopped run with resumable=0
// cannot be resumed.
func TestResumeEligibility_NonResumableStoppedProhibited(t *testing.T) {
	runner, runID := setupTestRun(t, "run-stopped-nonresumable", "http://127.0.0.1:8080/", Options{})
	finishRun(runner.DB, runID, StateStopped, StopUser, "", false, time.Now())
	addFrontierItems(t, runner, runID, "http://127.0.0.1:8080/")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err == nil {
		t.Fatal("expected Resume to fail for non-resumable stopped run, got nil error")
	}
	if !errors.Is(err, ErrRunNotResumable) {
		t.Errorf("got error %v, want wrapping ErrRunNotResumable", err)
	}
	if handle != nil {
		t.Errorf("expected nil handle on rejected resume, got %v", handle)
	}

	run, err := runner.LoadRun(runID)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if run.State != StateStopped {
		t.Errorf("run state mutated to %s, want %s", run.State, StateStopped)
	}
	if run.Resumable {
		t.Error("run.Resumable mutated to true, want false")
	}

	items, err := loadFrontier(runner.DB, runID)
	if err != nil {
		t.Fatalf("loadFrontier: %v", err)
	}
	if len(items) == 0 {
		t.Error("frontier was cleared on rejected resume")
	}

	if runner.Handle(runID) != nil {
		t.Error("active handle registered on rejected resume")
	}
}

// TestResumeEligibility_FailedProhibited verifies that a failed run cannot be resumed.
func TestResumeEligibility_FailedProhibited(t *testing.T) {
	runner, runID := setupTestRun(t, "run-failed", "http://127.0.0.1:8080/", Options{})
	finishRun(runner.DB, runID, StateFailed, "", "crawl network error", false, time.Now())
	addFrontierItems(t, runner, runID, "http://127.0.0.1:8080/")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err == nil {
		t.Fatal("expected Resume to fail for failed run, got nil error")
	}
	if !errors.Is(err, ErrRunNotResumable) {
		t.Errorf("got error %v, want wrapping ErrRunNotResumable", err)
	}
	if handle != nil {
		t.Errorf("expected nil handle, got %v", handle)
	}

	run, err := runner.LoadRun(runID)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if run.State != StateFailed {
		t.Errorf("run state mutated to %s, want %s", run.State, StateFailed)
	}

	items, err := loadFrontier(runner.DB, runID)
	if err != nil {
		t.Fatalf("loadFrontier: %v", err)
	}
	if len(items) == 0 {
		t.Error("frontier was cleared on rejected resume")
	}

	if runner.Handle(runID) != nil {
		t.Error("active handle registered on rejected resume")
	}
}

// TestResumeEligibility_RunningNoHandleProhibited verifies that a run marked 'running' with no live
// handle cannot be resumed silently and fails with ErrRunBusy.
func TestResumeEligibility_RunningNoHandleProhibited(t *testing.T) {
	runner, runID := setupTestRun(t, "run-running-orphan", "http://127.0.0.1:8080/", Options{})
	// Run is in StateRunning by default from insertRun

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err == nil {
		t.Fatal("expected Resume to fail for running run without active handle, got nil error")
	}
	if !errors.Is(err, ErrRunBusy) {
		t.Errorf("got error %v, want wrapping ErrRunBusy", err)
	}
	if handle != nil {
		t.Errorf("expected nil handle, got %v", handle)
	}

	run, err := runner.LoadRun(runID)
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if run.State != StateRunning {
		t.Errorf("run state mutated to %s, want %s", run.State, StateRunning)
	}

	if runner.Handle(runID) != nil {
		t.Error("active handle registered on rejected resume")
	}
}

// TestResumeEligibility_PausedResumableAllowed proves that a paused resumable run can be resumed.
func TestResumeEligibility_PausedResumableAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body>Resumed paused test</body></html>`)
	}))
	t.Cleanup(srv.Close)

	runner, runID := setupTestRun(t, "run-paused-resumable", srv.URL+"/", Options{})
	pauseRun(runner.DB, runID) // sets StatePaused, resumable=1
	addFrontierItems(t, runner, runID, srv.URL+"/")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err != nil {
		t.Fatalf("Resume failed for paused resumable run: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle on successful resume")
	}

	if runner.Handle(runID) == nil {
		t.Error("expected active handle to be registered")
	}

	_ = handle.Stop()
	_, _ = handle.Wait()
}

// TestResumeEligibility_StoppedResumableAllowed proves that a stopped resumable run can be resumed.
func TestResumeEligibility_StoppedResumableAllowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><body>Resumed stopped test</body></html>`)
	}))
	t.Cleanup(srv.Close)

	runner, runID := setupTestRun(t, "run-stopped-resumable", srv.URL+"/", Options{})
	finishRun(runner.DB, runID, StateStopped, StopUser, "", true, time.Now()) // resumable=1
	addFrontierItems(t, runner, runID, srv.URL+"/")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	handle, err := runner.Resume(ctx, runID)
	if err != nil {
		t.Fatalf("Resume failed for stopped resumable run: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle on successful resume")
	}

	if runner.Handle(runID) == nil {
		t.Error("expected active handle to be registered")
	}

	_ = handle.Stop()
	_, _ = handle.Wait()
}
