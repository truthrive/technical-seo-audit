// Package sitecrawl crawls a whole site and reports what an SEO audit needs:
// status codes, titles and meta, headings, canonicals, hreflang, the internal
// link graph, duplicates and orphans.
//
// It is a Go port of LibreCrawl (MIT, Python/Flask) — its field list, issue
// rules and thresholds are the specification — restructured around this app's
// constraints and extended with the parts Screaming Frog has and LibreCrawl
// lacks: redirect chains, duplicate title/meta/H1, rel=nofollow, orphan pages,
// hreflang reciprocity and canonical chains.
package sitecrawl

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"

	"onescout/desktop/internal/core/credset"
	"onescout/desktop/internal/core/httpx"
	"onescout/desktop/internal/core/jobs"
	"onescout/desktop/internal/core/license"
	"onescout/desktop/internal/core/runs"
	"onescout/desktop/internal/core/schema"
	"onescout/desktop/internal/core/workspace"
	"onescout/desktop/internal/tools"
)

// VaultKeyProxy is app-wide, shared with the other tools.
const VaultKeyProxy = "proxy_url"

// FilePicker is the native Save dialog, injected so this package never imports
// the Wails application package.
type FilePicker interface {
	SaveFile(defaultName, filterName, filterPattern string) (string, error)
}

// keyStore is the narrow slice of the vault this tool needs, so tests never
// touch the OS keychain.
type keyStore interface {
	Get(name string) (string, error)
	Set(name, secret string) error
	Delete(name string) error
}

// handle tracks a live crawl so Pause and Resume can reach its gate, and so
// StopPageSpeed can stop the measurements without ending the crawl. psi is nil
// whenever the PageSpeed pass is off.
type handle struct {
	jobID string
	gate  *pauseGate
	psi   *psiPump
}

type Service struct {
	Workspaces *workspace.Manager
	Vault      credset.Store
	Jobs       *jobs.Runner
	// Licence refuses new work when the licence is not in good standing.
	// Reading and exporting past runs never asks it (plan D12).
	Licence *license.Gate
	Files   FilePicker

	// ready guards the DDL per workspace. This tool has forty statements and
	// Rows() runs on every keystroke, so replaying them each time would put the
	// schema on the hot path.
	ready schema.Ready
	// Per workspace, like ready above: a bare sync.Once skipped recovery
	// for every workspace opened after the first.
	stale    runs.Gate
	secrets  keyStore
	ftsReady bool

	mu   sync.Mutex
	live map[string]*handle
	// revision mirrors each run's flush counter so a window read can tell the
	// frontend whether it is looking at fresh rows.
	revisions map[string]int64
	// psiJobs tracks a PageSpeed pass running WITHOUT a crawl — the "fill in a
	// stored run" path, which has no handle to hang off.
	psiJobs map[string]*psiJob
}

// ToolID is this tool's stable Go identity, independent of the frontend id.
const ToolID = "siteCrawl"

func init() {
	tools.Register(tools.Factory{ID: ToolID, New: func(d tools.Deps) application.Service {
		return application.NewService(&Service{Workspaces: d.Workspaces, Vault: d.Vault, Jobs: d.Jobs, Licence: d.Licence, Files: d.Files})
	}})
}

func (s *Service) store() keyStore {
	if s.secrets != nil {
		return s.secrets
	}
	if s.Vault == nil {
		return nil
	}
	return s.Vault
}

// proxy reads the app-wide proxy setting. Built per run so a just-saved value
// takes effect without a restart.
func (s *Service) proxy() *url.URL {
	st := s.store()
	if st == nil {
		return nil
	}
	// Through credset, like every other reader of this secret: Connections
	// stores a labelled list, and parsing that JSON as a URL yields nil — the
	// crawl would then run direct while the user believes it is proxied.
	c, ok := credset.First(credset.Load(st, VaultKeyProxy))
	if !ok {
		return nil
	}
	return parseProxy(c.Secret)
}

// crawlProxy decides whether this run's fetches go through the proxy saved in
// Connections. Nil means a direct connection.
//
// Opt-in per run rather than "on whenever the vault holds a proxy": under that
// older rule, a proxy saved for another tool re-routed every crawl without
// anyone asking for it. Screaming Frog puts its proxy behind a setting for the
// same reason, so this one lives in Crawl configuration.
func (s *Service) crawlProxy(opts Options) httpx.ProxyFunc {
	if !opts.UseProxy {
		return nil
	}
	return func() *url.URL { return s.proxy() }
}

// parseProxy accepts a full URL or a bare login:pass@host:port.
func parseProxy(raw string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil
	}
	return u
}

// workspace resolves the workspace the user currently has open and its writable
// handle.
//
// Callers hold the returned handle for as long as their work lasts. That is what
// lets a crawl keep writing into the right client after the user switches away:
// nothing re-resolves the workspace mid-job.
func (s *Service) workspace() (string, *sql.DB, error) {
	if s.Workspaces == nil {
		return "", nil, errors.New("sitecrawl: no workspace manager")
	}
	id, err := s.Workspaces.Active()
	if err != nil {
		return "", nil, err
	}
	db, err := s.Workspaces.DB(id)
	if err != nil {
		return "", nil, err
	}

	if err := s.ready.Do(id, func() error {
		if err := ensureSchema(db); err != nil {
			return err
		}
		s.ftsReady = ensureFTS(db)
		return nil
	}); err != nil {
		return "", nil, err
	}

	s.stale.Do(id, func() { recoverStaleRuns(db) })
	return id, db, nil
}

// readDB returns the multi-connection read-only handle.
//
// Grid reads go through it so they do not queue behind the crawl's writes: a
// 50k crawl holds the single writable connection for tens of minutes, and
// every other tool's table would stall with it.
func (s *Service) readDB() (*sql.DB, error) {
	id, _, err := s.workspace()
	if err != nil {
		return nil, err
	}
	return s.Workspaces.DBReadOnly(id)
}

func (s *Service) revisionOf(runID string) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revisions[runID]
}

func (s *Service) setRevision(runID string, rev int64) {
	s.mu.Lock()
	if s.revisions == nil {
		s.revisions = map[string]int64{}
	}
	s.revisions[runID] = rev
	s.mu.Unlock()
}

func (s *Service) register(runID string, h *handle) {
	s.mu.Lock()
	if s.live == nil {
		s.live = map[string]*handle{}
	}
	s.live[runID] = h
	s.mu.Unlock()
}

func (s *Service) unregister(runID string) {
	s.mu.Lock()
	delete(s.live, runID)
	s.mu.Unlock()
}

func (s *Service) handleFor(runID string) *handle {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.live[runID]
}

// DefaultOptions gives the frontend the same numbers Go clamps to, so the two
// never disagree about what "default" means.
func (s *Service) DefaultOptions() Options {
	return Options{
		Mode:                 ModeSpider,
		MaxDepth:             DefaultMaxDepth,
		MaxURLs:              DefaultMaxURLs,
		Concurrency:          DefaultConcurrency,
		TimeoutSec:           DefaultTimeoutSec,
		Retries:              DefaultRetries,
		MaxFileSizeMB:        DefaultMaxFileSizeMB,
		UserAgent:            DefaultUserAgent,
		FollowRedirects:      true,
		CrawlExternal:        true,
		CrawlImages:          true,
		CrawlCSS:             true,
		CrawlJS:              true,
		RespectRobots:        true,
		DiscoverSitemaps:     true,
		UseDefaultExcl:       true,
		EnableDuplication:    true,
		DuplicationThreshold: DefaultDuplicationThreshold,
		JSMaxPages:           DefaultJSMaxPages,
		JSWaitMs:             DefaultJSWaitMs,
		JSTimeoutSec:         DefaultJSTimeoutSec,
		JSViewportWidth:      DefaultJSViewportW,
		JSViewportHeight:     DefaultJSViewportH,
		JSConcurrency:        DefaultJSConcurrenc,
		PSIStrategy:          StrategyMobile,
	}
}

// Start begins a crawl.
//
// seeds is one URL in spider mode, or the whole list in list mode.
func (s *Service) Start(seeds []string, opts Options) (StartedCrawl, error) {
	if err := s.Licence.Check(); err != nil {
		return StartedCrawl{}, err
	}
	_, db, err := s.workspace()
	if err != nil {
		return StartedCrawl{}, err
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
		return StartedCrawl{}, errors.New("sitecrawl: no valid URL to crawl")
	}
	seedURL := clean[0]
	host := hostOf(seedURL)

	// The run id IS the job id for a fresh crawl, but Start only learns the job
	// id after Jobs.Start returns — so the body waits on `ready` until the run
	// row exists. Without it the first flush could write rows for a run that is
	// not in the table yet.
	var (
		runID string
		coord *coordinator
	)
	ready := make(chan error, 1)
	jobID := s.Jobs.Start(func(ctx context.Context, report func(done, total int, message string)) error {
		if err := <-ready; err != nil {
			return err
		}
		return s.runCrawl(ctx, db, coordSetup{
			runID: runID, jobID: runID, seeds: clean, seedURL: seedURL, opts: opts,
			report: report,
		}, &coord)
	})
	runID = jobID

	err = insertRun(db, runID, seedURL, host, opts)
	ready <- err
	if err != nil {
		return StartedCrawl{}, err
	}
	return StartedCrawl{JobID: jobID, RunID: runID}, nil
}

type coordSetup struct {
	runID string
	// jobID addresses the goroutine. It equals runID for a fresh crawl and
	// differs for a checkpoint resume, and Stop needs it — cancelling by run id
	// would silently do nothing.
	jobID   string
	seeds   []string
	seedURL string
	opts    Options
	report  func(done, total int, message string)
	resume  bool
}

// runCrawl is the job body shared by Start and Resume.
func (s *Service) runCrawl(ctx context.Context, db *sql.DB, cfg coordSetup, out **coordinator) error {
	// Resolved here rather than inside the coordinator so that package has no
	// opinion about vaults or workspaces. Both may be empty — the pass is then
	// simply off.
	var psi psiSetup
	if cfg.opts.EnablePageSpeed {
		psi.read, _ = s.readDB()
		psi.key, _ = s.psiKey()
	}

	c, err := newCoordinator(ctx, db, cfg.runID, cfg.opts, cfg.seedURL, s.crawlProxy(cfg.opts),
		func(name string, data any) { s.Jobs.Emit(name, data) }, cfg.report, psi)
	if err != nil {
		finishRun(db, cfg.runID, StateFailed, "", err.Error(), false, time.Now())
		return err
	}
	if out != nil {
		*out = c
	}

	s.register(cfg.runID, &handle{jobID: cfg.jobID, gate: c.gate, psi: c.psi})
	defer s.unregister(cfg.runID)

	started := time.Now()
	if cfg.resume {
		seen, nextID, err := loadSeen(db, cfg.runID, cfg.opts.IgnoreQueryParam)
		if err != nil {
			return err
		}
		items, err := loadFrontier(db, cfg.runID)
		if err != nil {
			return err
		}
		// The page table is the authority on what the paused run already did —
		// without this both "crawled" and "found" would restart from zero and
		// shrink the run row's counters on the next flush.
		var prior int
		db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, cfg.runID).Scan(&prior)
		c.crawled = prior
		c.frontier.restore(seen, nextID, items, prior)
		clearFrontier(db, cfg.runID)
	}

	runErr := c.run(ctx, cfg.seeds, cfg.resume)
	c.flush()
	s.setRevision(cfg.runID, c.revision)

	switch {
	case ctx.Err() != nil:
		// Always persist, even mid-cancel: a stopped crawl keeps everything it
		// found, and the frontier is checkpointed so Resume can continue.
		c.checkpoint()
		finishRun(db, cfg.runID, StateStopped, StopUser, "", c.frontier.queued() > 0, started)
		c.emitState(StateStopped, StopUser, c.frontier.queued() > 0, nil)
	case c.gate.isPaused():
		c.checkpoint()
		pauseRun(db, cfg.runID)
		c.emitState(StatePaused, "", true, nil)
	case runErr != nil:
		finishRun(db, cfg.runID, StateFailed, "", runErr.Error(), false, started)
		c.emitState(StateFailed, "", false, runErr)
	default:
		clearFrontier(db, cfg.runID)
		finishRun(db, cfg.runID, StateCompleted, c.stopReason, "", false, started)
		c.emitState(StateCompleted, c.stopReason, false, nil)
	}
	return runErr
}

// Pause stops dispatching without ending the run. The job goroutine stays alive
// holding the frontier, so Resume continues exactly where it left off.
func (s *Service) Pause(runID string) error {
	h := s.handleFor(runID)
	if h == nil {
		return errors.New("sitecrawl: this crawl is not running")
	}
	h.gate.Pause()
	return nil
}

// Resume continues a paused crawl.
//
// Two paths: a live gate is simply lifted, while a run paused by closing the
// app is restarted from its checkpoint — in which case the job id differs from
// the run id, exactly as in indexcheck's rerun.
func (s *Service) Resume(runID string) (StartedCrawl, error) {
	if err := s.Licence.Check(); err != nil {
		return StartedCrawl{}, err
	}
	if h := s.handleFor(runID); h != nil {
		h.gate.Resume()
		return StartedCrawl{JobID: h.jobID, RunID: runID}, nil
	}

	_, db, err := s.workspace()
	if err != nil {
		return StartedCrawl{}, err
	}
	run, err := loadRun(db, runID)
	if err != nil {
		return StartedCrawl{}, err
	}
	if err := claimRun(db, runID); err != nil {
		return StartedCrawl{}, err
	}

	// The body must not run before jobID is known, or the registered handle
	// would carry an empty job id and Stop would cancel nothing.
	var jobID string
	ready := make(chan struct{})
	id := s.Jobs.Start(func(ctx context.Context, report func(done, total int, message string)) error {
		<-ready
		return s.runCrawl(ctx, db, coordSetup{
			runID: runID, jobID: jobID, seedURL: run.SeedURL, opts: run.Options.normalized(),
			report: report, resume: true,
		}, nil)
	})
	jobID = id
	close(ready)
	return StartedCrawl{JobID: id, RunID: runID}, nil
}

// Stop ends a crawl, keeping everything it found.
func (s *Service) Stop(runID string) error {
	h := s.handleFor(runID)
	if h == nil {
		return nil // already finished
	}
	// A paused crawl is parked inside its select; lift the gate so the
	// cancellation is noticed immediately instead of at the grace timeout.
	h.gate.Resume()
	s.Jobs.Cancel(h.jobID)
	return nil
}

// Cancel is the jobId-addressed form the shared progress UI uses.
func (s *Service) Cancel(jobID string) { s.Jobs.Cancel(jobID) }

// Recrawl starts a fresh run with the same seed and options.
func (s *Service) Recrawl(runID string) (StartedCrawl, error) {
	if err := s.Licence.Check(); err != nil {
		return StartedCrawl{}, err
	}
	_, db, err := s.workspace()
	if err != nil {
		return StartedCrawl{}, err
	}
	run, err := loadRun(db, runID)
	if err != nil {
		return StartedCrawl{}, err
	}
	return s.Start([]string{run.SeedURL}, run.Options)
}

// Status is the poll fallback for a frontend that missed an event.
//
// Read-only on purpose, and this is the call where it matters most: the screen
// polls it every second or two for the whole length of a crawl, which is
// exactly when the crawl is holding the single writer connection. Every counter
// it reports is written by a committed statement, so the read-only handle sees
// the same numbers.
func (s *Service) Status(runID string) (RunStatus, error) {
	db, err := s.readDB()
	if err != nil {
		return RunStatus{}, err
	}
	run, err := loadRun(db, runID)
	if err != nil {
		return RunStatus{}, err
	}
	st := RunStatus{
		RunID: runID, State: run.State, Phase: run.Phase,
		Found: run.Found, Crawled: run.Crawled, Revision: s.revisionOf(runID),
	}
	if h := s.handleFor(runID); h != nil {
		st.JobID = h.jobID
	}
	return st, nil
}

// Runs returns the crawl history, newest first, with each run's approximate
// share of the workspace database.
func (s *Service) Runs() ([]RunSummary, error) {
	db, err := s.readDB()
	if err != nil {
		return nil, err
	}
	list, err := listRuns(db)
	if err != nil {
		return nil, err
	}
	sizes := runSizes(db)
	for i := range list {
		list[i].SizeBytes = sizes[list[i].ID]
	}
	return list, nil
}

// DeleteRun removes a run's rows in chunks, returning the job id doing it.
//
// A single DELETE over a 5M-row link table holds the workspace's only writable
// connection for seconds, freezing every other tool's grid — so the work is
// chunked and backgrounded, and the run row is hidden immediately.
func (s *Service) DeleteRun(runID string) (string, error) {
	_, db, err := s.workspace()
	if err != nil {
		return "", err
	}
	// Claiming "deleting" both hides the row from history straight away — the
	// chunked delete below can run for minutes — and makes the claim exclusive,
	// so a second click cannot start a duplicate delete job.
	claimed, err := claimForDelete(db, runID)
	if err != nil {
		return "", err
	}
	if !claimed {
		if runIsActive(db, runID) {
			return "", errors.New("sitecrawl: stop this crawl before deleting it")
		}
		// Already gone, or already being deleted — either way there is nothing
		// to start.
		return "", nil
	}
	jobID := s.Jobs.Start(func(ctx context.Context, report func(done, total int, message string)) error {
		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			more, err := deleteRunChunk(db, runID, 20000)
			if err != nil {
				return err
			}
			if !more {
				return nil
			}
			report(0, 0, "")
		}
	})
	return jobID, nil
}

// ClearHistory removes every stored crawl.
func (s *Service) ClearHistory() (string, error) {
	_, db, err := s.workspace()
	if err != nil {
		return "", err
	}
	if anyRunActive(db) {
		return "", errors.New("sitecrawl: stop the running crawl before clearing history")
	}
	list, err := listRuns(db)
	if err != nil {
		return "", err
	}
	jobID := s.Jobs.Start(func(ctx context.Context, report func(done, total int, message string)) error {
		for i, run := range list {
			for {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				more, err := deleteRunChunk(db, run.ID, 20000)
				if err != nil {
					return err
				}
				if !more {
					break
				}
			}
			report(i+1, len(list), "")
		}
		return nil
	})
	return jobID, nil
}

// OnLaunch is the tools.Starter hook: the shell calls it on every built tool
// that has one, so main never names this tool.
func (s *Service) OnLaunch() { s.RecoverRuns() }

// RecoverRuns is called once at startup so a crawl interrupted by closing the
// app shows as resumable rather than running.
func (s *Service) RecoverRuns() {
	if _, _, err := s.workspace(); err != nil {
		return
	}
}
