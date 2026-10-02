package sitecrawl

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
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
)

// Runner is a lightweight standalone host entrypoint that coordinates crawling,
// database persistence, and optional event/progress sinks.
type Runner struct {
	DB       *sql.DB
	Events   standalone.EventSink
	Progress standalone.ProgressSink
	Proxy    httpx.ProxyFunc
}

// NewRunner creates a new Runner bound to db.
func NewRunner(db *sql.DB) *Runner {
	return &Runner{DB: db}
}

// EnsureSchema initializes or updates the SiteCrawl database schema.
func (r *Runner) EnsureSchema() error {
	if err := ensureSchema(r.DB); err != nil {
		return err
	}
	ensureFTS(r.DB)
	return nil
}

// Crawl executes an end-to-end crawl for the given seeds and options.
func (r *Runner) Crawl(ctx context.Context, seeds []string, opts Options) (*RunSummary, error) {
	if opts.EnablePageSpeed {
		return nil, fmt.Errorf("%w: PageSpeed is not part of acquisition core", ErrCapabilityUnsupported)
	}
	if opts.EnableDuplication {
		return nil, fmt.Errorf("%w: duplicate analysis is deferred", ErrCapabilityUnsupported)
	}
	if opts.UseProxy && r.Proxy == nil {
		return nil, ErrProxyUnavailable
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

	runID := generateRunID()
	if err := insertRun(r.DB, runID, seedURL, host, opts); err != nil {
		return nil, fmt.Errorf("sitecrawl: insert run: %w", err)
	}

	emit := func(name string, data any) {
		if r.Events != nil {
			r.Events.Emit(name, data)
		}
		if name == EventProgress && r.Progress != nil {
			r.Progress.OnProgress(data)
		}
	}
	report := func(done, total int, message string) {}

	coord, err := newCoordinator(ctx, r.DB, runID, opts, seedURL, r.Proxy, emit, report)
	if err != nil {
		finishRun(r.DB, runID, StateFailed, "", err.Error(), false, time.Now())
		return nil, err
	}

	started := time.Now()
	runErr := coord.run(ctx, clean, false)
	coord.flush()

	switch {
	case ctx.Err() != nil:
		coord.checkpoint()
		finishRun(r.DB, runID, StateStopped, StopUser, "", coord.frontier.queued() > 0, started)
		coord.emitState(StateStopped, StopUser, coord.frontier.queued() > 0, nil)
	case runErr != nil:
		finishRun(r.DB, runID, StateFailed, "", runErr.Error(), false, started)
		coord.emitState(StateFailed, "", false, runErr)
	default:
		clearFrontier(r.DB, runID)
		finishRun(r.DB, runID, StateCompleted, coord.stopReason, "", false, started)
		coord.emitState(StateCompleted, coord.stopReason, false, nil)
	}

	summary, err := loadRun(r.DB, runID)
	if err != nil {
		return nil, fmt.Errorf("sitecrawl: load finished run: %w", err)
	}

	if ctx.Err() != nil {
		return &summary, ctx.Err()
	}
	if runErr != nil {
		return &summary, runErr
	}
	return &summary, nil
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

func generateRunID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("crawl_%s_%x", time.Now().UTC().Format("20060102150405"), binary.BigEndian.Uint64(b[:])&0xffff)
}
