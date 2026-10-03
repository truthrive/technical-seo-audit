package sitecrawl

import (
	"database/sql"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"onescout/desktop/internal/core/jobs"
	"onescout/desktop/internal/core/workspace"
	"onescout/desktop/internal/testutil"
)

// stubEmitter records every event a run emits so tests can assert on the wire
// shape rather than on internal state.
type stubEmitter struct {
	mu     sync.Mutex
	events []recorded
}

type recorded struct {
	name string
	data any
}

func (e *stubEmitter) Emit(name string, data ...any) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var payload any
	if len(data) > 0 {
		payload = data[0]
	}
	e.events = append(e.events, recorded{name: name, data: payload})
}

func (e *stubEmitter) count(name string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, ev := range e.events {
		if ev.name == name {
			n++
		}
	}
	return n
}

// runStates returns every RunStateEvent state in order.
func (e *stubEmitter) runStates() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []string
	for _, ev := range e.events {
		if ev.name != EventRunState {
			continue
		}
		if rs, ok := ev.data.(RunStateEvent); ok {
			out = append(out, rs.State)
		}
	}
	return out
}

// terminal reports the last job:progress state, which is what the shared
// progress UI keys on.
func (e *stubEmitter) terminal() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := len(e.events) - 1; i >= 0; i-- {
		if e.events[i].name != jobs.EventName {
			continue
		}
		if p, ok := e.events[i].data.(jobs.Progress); ok && p.State != "running" {
			return p.State
		}
	}
	return ""
}

// fakeVault is an in-memory keyStore so tests never reach the OS keychain.
type fakeVault struct {
	mu sync.Mutex
	m  map[string]string
}

func newFakeVault() *fakeVault { return &fakeVault{m: map[string]string{}} }

func (v *fakeVault) Get(name string) (string, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	s, ok := v.m[name]
	if !ok {
		return "", sql.ErrNoRows
	}
	return s, nil
}

func (v *fakeVault) Set(name, secret string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.m[name] = secret
	return nil
}

func (v *fakeVault) Delete(name string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	delete(v.m, name)
	return nil
}

// savePath is a FilePicker that answers with a fixed path, so ExportCSV runs
// without a native dialog.
type savePath struct{ dir string }

func (s savePath) SaveFile(defaultName, _, _ string) (string, error) {
	return filepath.Join(s.dir, defaultName), nil
}

// newTestService stands up a Service against a real SQLite file in a temp dir.
// Both env vars are set so the workspace manager finds it on Windows and Unix.
func newTestService(t *testing.T) (*Service, *sql.DB, *stubEmitter) {
	t.Helper()
	testutil.RedirectConfigDir(t)

	wm, err := workspace.NewManager()
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	t.Cleanup(wm.Close)

	emitter := &stubEmitter{}
	s := &Service{
		Workspaces: wm,
		Jobs:       jobs.NewRunner(emitter),
		secrets:    newFakeVault(),
		Files:      savePath{dir: t.TempDir()},
	}
	_, db, err := s.workspace()
	if err != nil {
		t.Fatalf("workspace init: %v", err)
	}
	return s, db, emitter
}

// fastCrawl shrinks every pacing constant so a test crawl finishes in
// milliseconds, restoring them afterwards. This is why those are vars.
func fastCrawl(t *testing.T) {
	t.Helper()
	oldFlush, oldProgress, oldReport, oldGrace := flushEvery, progressEvery, reportEvery, pauseGraceMax
	oldRetry := fetchRetryDelay
	flushEvery = 20 * time.Millisecond
	progressEvery = 20 * time.Millisecond
	reportEvery = 20 * time.Millisecond
	pauseGraceMax = 2 * time.Second
	fetchRetryDelay = 5 * time.Millisecond
	t.Cleanup(func() {
		flushEvery, progressEvery, reportEvery, pauseGraceMax = oldFlush, oldProgress, oldReport, oldGrace
		fetchRetryDelay = oldRetry
	})
}

// waitForRun polls until the run reaches a terminal or paused state.
func waitForRun(t *testing.T, s *Service, runID string, want ...string) RunStatus {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		st, err := s.Status(runID)
		if err == nil {
			for _, w := range want {
				if st.State == w {
					return st
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	st, _ := s.Status(runID)
	t.Fatalf("run %s stuck in state %q, wanted one of %v", runID, st.State, want)
	return RunStatus{}
}

// crawlFixture runs a whole crawl of the fixture site and returns the run id.
func crawlFixture(t *testing.T, s *Service, site *fixtureSite, mutate func(*Options)) string {
	t.Helper()
	opts := s.DefaultOptions()
	opts.Concurrency = 4
	// The default is on, but a unit test must never resolve a real hostname:
	// the fixture's external link would otherwise leave the machine.
	opts.CrawlExternal = false
	if mutate != nil {
		mutate(&opts)
	}
	started, err := s.Start([]string{site.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, s, started.RunID, StateCompleted, StateFailed, StateStopped)
	return started.RunID
}

// readFile is a small helper so export assertions read cleanly.
func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
