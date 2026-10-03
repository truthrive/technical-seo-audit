package sitecrawl

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/truthrive/technical-seo-audit/go/deps/httpx"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

var (
	// ErrCapabilityUnsupported is returned when an optional compatibility capability
	// (such as PageSpeed or duplicate analysis) is requested but not supported in
	// standalone acquisition core.
	ErrCapabilityUnsupported = errors.New("sitecrawl: capability unsupported in standalone acquisition core")

	// ErrProxyUnavailable is returned when UseProxy is enabled but no proxy provider function was supplied.
	ErrProxyUnavailable = errors.New("sitecrawl: proxy requested but no proxy function configured")

	// ErrRunNotResumable is returned when attempting to resume a run that is not
	// in an eligible resumable state (e.g. completed, failed, interrupted, deleting,
	// or non-resumable stopped).
	ErrRunNotResumable = errors.New("sitecrawl: run is not in a resumable state")

	// ErrRunBusy indicates a crawl is already running or claimed.
	ErrRunBusy = errors.New("sitecrawl: this crawl is already running")
)

// CrawlHandle represents an active or completed crawl execution.
type CrawlHandle struct {
	runID   string
	cancel  context.CancelFunc
	coord   *coordinator
	done    chan struct{}
	summary *RunSummary
	err     error
	mu      sync.Mutex
}

// RunID returns the crawl run ID.
func (h *CrawlHandle) RunID() string {
	return h.runID
}

// Pause pauses the running crawl, checkpointing current progress to SQLite.
func (h *CrawlHandle) Pause() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.done:
		return errors.New("sitecrawl: crawl is not running")
	default:
	}
	h.coord.gate.Pause()
	return nil
}

// Resume lifts the in-process pause and continues the crawl.
func (h *CrawlHandle) Resume() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.done:
		return errors.New("sitecrawl: crawl is not running")
	default:
	}
	h.coord.gate.Resume()
	return nil
}

// Stop stops the active crawl, flushing completed pages and checkpointing remaining frontier.
func (h *CrawlHandle) Stop() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.done:
		return nil
	default:
	}
	h.coord.gate.Resume() // lift pause so loop unblocks from select
	h.cancel()
	return nil
}

// Wait blocks until the crawl reaches a terminal state and returns the RunSummary.
func (h *CrawlHandle) Wait() (*RunSummary, error) {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.summary, h.err
}

// Runner is a lightweight standalone host entrypoint that coordinates crawling,
// database persistence, and optional event/progress sinks.
type Runner struct {
	DB       *sql.DB
	Events   standalone.EventSink
	Progress standalone.ProgressSink
	Proxy    httpx.ProxyFunc

	mu      sync.Mutex
	handles map[string]*CrawlHandle
}

// NewRunner creates a new Runner bound to db.
func NewRunner(db *sql.DB) *Runner {
	return &Runner{
		DB:      db,
		handles: make(map[string]*CrawlHandle),
	}
}

func (r *Runner) registerHandle(runID string, h *CrawlHandle) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.handles == nil {
		r.handles = make(map[string]*CrawlHandle)
	}
	r.handles[runID] = h
}

func (r *Runner) unregisterHandle(runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.handles, runID)
}

func (r *Runner) handleFor(runID string) *CrawlHandle {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.handles[runID]
}

// Handle returns the active in-process handle for runID, or nil if none.
func (r *Runner) Handle(runID string) *CrawlHandle {
	return r.handleFor(runID)
}

// Pause pauses an active crawl by run ID.
func (r *Runner) Pause(runID string) error {
	h := r.handleFor(runID)
	if h == nil {
		return errors.New("sitecrawl: this crawl is not running")
	}
	return h.Pause()
}

// Stop stops an active crawl by run ID.
func (r *Runner) Stop(runID string) error {
	h := r.handleFor(runID)
	if h == nil {
		return nil
	}
	return h.Stop()
}

// Status retrieves the current summary/status for a run by ID (alias to LoadRun).
func (r *Runner) Status(runID string) (RunSummary, error) {
	return r.LoadRun(runID)
}

// RecoverStaleRuns inspects the database and recovers any interrupted runs.
func (r *Runner) RecoverStaleRuns() error {
	return RecoverStaleRuns(r.DB)
}

// EnsureSchema initializes or updates the SiteCrawl database schema.
func (r *Runner) EnsureSchema() error {
	if err := ensureSchema(r.DB); err != nil {
		return err
	}
	ensureFTS(r.DB)
	return nil
}

// validateRuntimeCapabilities checks if the requested crawl options rely on capabilities
// that are unsupported in standalone core or missing necessary runtime providers (e.g. proxy func).
func validateRuntimeCapabilities(opts Options, proxy httpx.ProxyFunc) error {
	if opts.EnablePageSpeed {
		return fmt.Errorf("%w: PageSpeed is not part of acquisition core", ErrCapabilityUnsupported)
	}
	if opts.EnableDuplication {
		return fmt.Errorf("%w: duplicate analysis is deferred", ErrCapabilityUnsupported)
	}
	if opts.UseProxy && proxy == nil {
		return ErrProxyUnavailable
	}
	return nil
}

// Start begins an asynchronous crawl, returning a lightweight CrawlHandle immediately.
func (r *Runner) Start(ctx context.Context, seeds []string, opts Options) (*CrawlHandle, error) {
	if err := validateRuntimeCapabilities(opts, r.Proxy); err != nil {
		return nil, err
	}

	opts = opts.normalized()

	clean := make([]string, 0, len(seeds))
	for _, raw := range dedupe(seeds) {
		n, err := normalizeInput(raw)
		if err != nil {
			continue
		}
		clean = append(clean, n)
	}
	if len(clean) == 0 {
		return nil, errors.New("sitecrawl: no valid URL to crawl")
	}
	seedURL := clean[0]
	host := hostOf(seedURL)

	if err := r.EnsureSchema(); err != nil {
		return nil, fmt.Errorf("sitecrawl: ensure schema: %w", err)
	}

	runID, err := generateRunID()
	if err != nil {
		return nil, fmt.Errorf("sitecrawl: generate run id: %w", err)
	}
	if err := insertRun(r.DB, runID, seedURL, host, opts); err != nil {
		return nil, fmt.Errorf("sitecrawl: insert run: %w", err)
	}

	crawlCtx, cancel := context.WithCancel(ctx)

	emit := func(name string, data any) {
		if r.Events != nil {
			r.Events.Emit(name, data)
		}
		if name == EventProgress && r.Progress != nil {
			r.Progress.OnProgress(data)
		}
	}
	report := func(done, total int, message string) {}

	coord, err := newCoordinator(crawlCtx, r.DB, runID, opts, seedURL, r.Proxy, emit, report)
	if err != nil {
		cancel()
		finishRun(r.DB, runID, StateFailed, "", err.Error(), false, time.Now())
		return nil, err
	}

	handle := &CrawlHandle{
		runID:  runID,
		cancel: cancel,
		coord:  coord,
		done:   make(chan struct{}),
	}
	r.registerHandle(runID, handle)

	go r.runCrawlLoop(crawlCtx, coord, handle, clean, false)

	return handle, nil
}

// Resume continues a previously stopped or paused crawl from its SQLite checkpoint.
// If the crawl is already live in-process, it lifts the pause gate.
func (r *Runner) Resume(ctx context.Context, runID string) (*CrawlHandle, error) {
	if h := r.handleFor(runID); h != nil {
		if err := h.Resume(); err != nil {
			return nil, err
		}
		return h, nil
	}

	if err := r.EnsureSchema(); err != nil {
		return nil, fmt.Errorf("sitecrawl: ensure schema: %w", err)
	}

	run, err := loadRun(r.DB, runID)
	if err != nil {
		return nil, fmt.Errorf("sitecrawl: load run for resume: %w", err)
	}

	if err := validateRuntimeCapabilities(run.Options, r.Proxy); err != nil {
		return nil, err
	}

	if run.State == StateRunning {
		return nil, fmt.Errorf("%w: run %s is marked running with no active in-process handle", ErrRunBusy, runID)
	}
	if !run.Resumable || (run.State != StatePaused && run.State != StateStopped) {
		return nil, fmt.Errorf("%w: run %s is in state %q (resumable=%v)", ErrRunNotResumable, runID, run.State, run.Resumable)
	}

	seen, nextID, err := loadSeen(r.DB, runID, run.Options.IgnoreQueryParam)
	if err != nil {
		return nil, fmt.Errorf("sitecrawl: load seen: %w", err)
	}

	items, err := loadFrontier(r.DB, runID)
	if err != nil {
		return nil, fmt.Errorf("sitecrawl: load frontier: %w", err)
	}

	var prior int
	if err := r.DB.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, runID).Scan(&prior); err != nil {
		return nil, fmt.Errorf("sitecrawl: count prior pages: %w", err)
	}

	crawlCtx, cancel := context.WithCancel(ctx)

	emit := func(name string, data any) {
		if r.Events != nil {
			r.Events.Emit(name, data)
		}
		if name == EventProgress && r.Progress != nil {
			r.Progress.OnProgress(data)
		}
	}
	report := func(done, total int, message string) {}

	coord, err := newCoordinator(crawlCtx, r.DB, runID, run.Options.normalized(), run.SeedURL, r.Proxy, emit, report)
	if err != nil {
		cancel()
		return nil, err
	}

	coord.crawled = prior
	coord.crawledInternal = prior
	coord.frontier.restore(seen, nextID, items, prior)

	if err := claimRun(r.DB, runID); err != nil {
		cancel()
		return nil, err
	}

	clearFrontier(r.DB, runID)

	handle := &CrawlHandle{
		runID:  runID,
		cancel: cancel,
		coord:  coord,
		done:   make(chan struct{}),
	}
	r.registerHandle(runID, handle)

	go r.runCrawlLoop(crawlCtx, coord, handle, nil, true)

	return handle, nil
}

// ResumeRun resumes a run from checkpoint and waits synchronously for completion.
func (r *Runner) ResumeRun(ctx context.Context, runID string) (*RunSummary, error) {
	h, err := r.Resume(ctx, runID)
	if err != nil {
		return nil, err
	}
	return h.Wait()
}

func (r *Runner) runCrawlLoop(ctx context.Context, coord *coordinator, handle *CrawlHandle, seeds []string, resume bool) {
	runID := handle.runID
	defer r.unregisterHandle(runID)
	defer close(handle.done)

	started := time.Now()
	runErr := coord.run(ctx, seeds, resume)
	coord.flush()

	switch {
	case ctx.Err() != nil:
		coord.checkpoint()
		finishRun(r.DB, runID, StateStopped, StopUser, "", coord.frontier.queued() > 0, started)
		coord.emitState(StateStopped, StopUser, coord.frontier.queued() > 0, nil)
	case coord.gate.isPaused():
		coord.checkpoint()
		pauseRun(r.DB, runID)
		coord.emitState(StatePaused, "", true, nil)
	case runErr != nil:
		finishRun(r.DB, runID, StateFailed, "", runErr.Error(), false, started)
		coord.emitState(StateFailed, "", false, runErr)
	default:
		clearFrontier(r.DB, runID)
		finishRun(r.DB, runID, StateCompleted, coord.stopReason, "", false, started)
		coord.emitState(StateCompleted, coord.stopReason, false, nil)
	}

	summary, err := loadRun(r.DB, runID)
	handle.mu.Lock()
	if err == nil {
		handle.summary = &summary
	}
	if ctx.Err() != nil {
		handle.err = ctx.Err()
	} else if runErr != nil {
		handle.err = runErr
	}
	handle.mu.Unlock()
}

// Crawl executes an end-to-end crawl for the given seeds and options synchronously.
func (r *Runner) Crawl(ctx context.Context, seeds []string, opts Options) (*RunSummary, error) {
	h, err := r.Start(ctx, seeds, opts)
	if err != nil {
		return nil, err
	}
	return h.Wait()
}

// LoadRun loads the summary for a run by ID.
func (r *Runner) LoadRun(runID string) (RunSummary, error) {
	return loadRun(r.DB, runID)
}

// ListRuns lists all runs in the database, newest first.
func (r *Runner) ListRuns() ([]RunSummary, error) {
	return listRuns(r.DB)
}

// Page retrieves a single Page record by run ID and URL ID.
func (r *Runner) Page(runID string, urlID int64) (*Page, error) {
	var blob string
	err := r.DB.QueryRow(`SELECT data FROM sitecrawl_pages WHERE run_id = ? AND url_id = ?`, runID, urlID).Scan(&blob)
	if err != nil {
		return nil, err
	}
	var p Page
	if err := json.Unmarshal([]byte(blob), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Pages retrieves all Page records for a run.
func (r *Runner) Pages(runID string) ([]Page, error) {
	rows, err := r.DB.Query(`SELECT data FROM sitecrawl_pages WHERE run_id = ? ORDER BY url_id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Page
	for rows.Next() {
		var blob string
		if err := rows.Scan(&blob); err != nil {
			return nil, err
		}
		var p Page
		if err := json.Unmarshal([]byte(blob), &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Links retrieves all edges stored for a run.
func (r *Runner) Links(runID string) ([]edge, error) {
	rows, err := r.DB.Query(`SELECT src_id, dst_id, seq, placement, flags, anchor FROM sitecrawl_links WHERE run_id = ? ORDER BY src_id, seq`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []edge
	for rows.Next() {
		var e edge
		if err := rows.Scan(&e.SrcID, &e.DstID, &e.Seq, &e.Placement, &e.Flags, &e.Anchor); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// URLs retrieves all dictionary URL entries for a run.
func (r *Runner) URLs(runID string) ([]urlEntry, error) {
	rows, err := r.DB.Query(`SELECT id, url FROM sitecrawl_urls WHERE run_id = ? ORDER BY id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []urlEntry
	for rows.Next() {
		var u urlEntry
		if err := rows.Scan(&u.ID, &u.URL); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func generateRunID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("sitecrawl: generate random run id: %w", err)
	}
	return fmt.Sprintf("crawl_%s_%s", time.Now().UTC().Format("20060102150405"), hex.EncodeToString(b[:])), nil
}
