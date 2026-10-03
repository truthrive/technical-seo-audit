package sitecrawl

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"onescout/desktop/internal/core/runs"
	"onescout/desktop/internal/core/schema"
)

// schemaStmts is the whole schema, declared as idempotent DDL and replayed on
// every workspace open (guarded by schemaOnce — see Service.workspace).
//
// Two deliberate departures from the other tools' "one JSON blob plus a couple
// of promoted columns" rule, both earned:
//
//   - sitecrawl_pages promotes far more. Here the grid IS the product: every tab
//     is an indexed predicate, and every column the grid renders must be
//     readable without touching `data` — 50 rows × a 20KB blob is 1MB per
//     window against ~15KB of promoted columns.
//   - issues are a separate table. The sidebar needs GROUP BY code and the
//     "only pages with issue X" filter needs an indexed join; a text column
//     would force LIKE '% x %' and a full scan for every count.
var schemaStmts = []string{
	`CREATE TABLE IF NOT EXISTS sitecrawl_runs (
		id            TEXT PRIMARY KEY,
		seed_url      TEXT NOT NULL,
		host          TEXT NOT NULL,
		options       TEXT NOT NULL,
		state         TEXT NOT NULL,
		phase         TEXT NOT NULL DEFAULT '',
		stop_reason   TEXT NOT NULL DEFAULT '',
		resumable     INTEGER NOT NULL DEFAULT 0,
		found         INTEGER NOT NULL DEFAULT 0,
		crawled       INTEGER NOT NULL DEFAULT 0,
		issue_total   INTEGER NOT NULL DEFAULT 0,
		next_url_id   INTEGER NOT NULL DEFAULT 1,
		error         TEXT NOT NULL DEFAULT '',
		started_at    TEXT NOT NULL,
		finished_at   TEXT,
		duration_ms   INTEGER NOT NULL DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_runs_started ON sitecrawl_runs(started_at DESC)`,

	// The URL dictionary. sitecrawl_links stores two integers instead of two
	// ~80-byte strings because of it: at 5M edges that is ~250MB rather than 1GB.
	`CREATE TABLE IF NOT EXISTS sitecrawl_urls (
		run_id TEXT NOT NULL,
		id     INTEGER NOT NULL,
		url    TEXT NOT NULL,
		PRIMARY KEY (run_id, id)
	)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_urls_lookup ON sitecrawl_urls(run_id, url)`,

	`CREATE TABLE IF NOT EXISTS sitecrawl_pages (
		run_id        TEXT    NOT NULL,
		url_id        INTEGER NOT NULL,
		url           TEXT    NOT NULL,
		data          TEXT    NOT NULL,

		kind          TEXT    NOT NULL DEFAULT 'html',
		is_internal   INTEGER NOT NULL DEFAULT 1,
		depth         INTEGER NOT NULL DEFAULT 0,
		discovered_by TEXT    NOT NULL DEFAULT 'link',

		status        INTEGER NOT NULL DEFAULT 0,
		status_class  INTEGER NOT NULL DEFAULT 0,
		content_type  TEXT    NOT NULL DEFAULT '',
		size_bytes    INTEGER NOT NULL DEFAULT 0,
		response_ms   INTEGER NOT NULL DEFAULT 0,
		redirect_to   TEXT    NOT NULL DEFAULT '',
		redirect_hops INTEGER NOT NULL DEFAULT 0,
		error_type    TEXT    NOT NULL DEFAULT '',

		title         TEXT    NOT NULL DEFAULT '',
		title_len     INTEGER NOT NULL DEFAULT 0,
		meta_desc     TEXT    NOT NULL DEFAULT '',
		meta_desc_len INTEGER NOT NULL DEFAULT 0,
		h1            TEXT    NOT NULL DEFAULT '',
		h1_len        INTEGER NOT NULL DEFAULT 0,
		h1_count      INTEGER NOT NULL DEFAULT 0,
		h2            TEXT    NOT NULL DEFAULT '',
		h2_count      INTEGER NOT NULL DEFAULT 0,
		word_count    INTEGER NOT NULL DEFAULT 0,
		text_ratio    REAL    NOT NULL DEFAULT 0,
		lang          TEXT    NOT NULL DEFAULT '',
		canonical     TEXT    NOT NULL DEFAULT '',
		meta_robots   TEXT    NOT NULL DEFAULT '',
		x_robots      TEXT    NOT NULL DEFAULT '',
		indexable     INTEGER NOT NULL DEFAULT 1,
		indexability  TEXT    NOT NULL DEFAULT '',
		robots_state  TEXT    NOT NULL DEFAULT 'unknown',
		last_mod      TEXT    NOT NULL DEFAULT '',

		outlinks      INTEGER NOT NULL DEFAULT 0,
		outlinks_ext  INTEGER NOT NULL DEFAULT 0,
		inlinks       INTEGER NOT NULL DEFAULT 0,
		inlinks_uniq  INTEGER NOT NULL DEFAULT 0,
		images_count  INTEGER NOT NULL DEFAULT 0,
		images_noalt  INTEGER NOT NULL DEFAULT 0,

		issue_count   INTEGER NOT NULL DEFAULT 0,
		issue_max_sev INTEGER NOT NULL DEFAULT 0,
		dupe_group    INTEGER NOT NULL DEFAULT 0,
		rendered      INTEGER NOT NULL DEFAULT 0,
		orphan        INTEGER NOT NULL DEFAULT 0,
		crawled_at    TEXT    NOT NULL,
		PRIMARY KEY (run_id, url_id)
	)`,
	// Indexed: the predicates every tab lands on. Sort-only columns
	// (title_len, word_count, size_bytes…) are deliberately NOT indexed — a
	// 50k-row sort is one scan at ~40-80ms, while twenty more indexes would be
	// paid on every insert of the crawl.
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_url    ON sitecrawl_pages(run_id, url)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_status ON sitecrawl_pages(run_id, status_class, status)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_kind   ON sitecrawl_pages(run_id, is_internal, kind)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_index  ON sitecrawl_pages(run_id, indexable)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_issues ON sitecrawl_pages(run_id, issue_count DESC)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_depth  ON sitecrawl_pages(run_id, depth)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_dupe   ON sitecrawl_pages(run_id, dupe_group)`,

	`CREATE TABLE IF NOT EXISTS sitecrawl_issues (
		run_id   TEXT    NOT NULL,
		url_id   INTEGER NOT NULL,
		code     TEXT    NOT NULL,
		severity INTEGER NOT NULL,
		category TEXT    NOT NULL,
		detail   TEXT    NOT NULL DEFAULT '',
		PRIMARY KEY (run_id, url_id, code)
	)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_issues_code ON sitecrawl_issues(run_id, code, url_id)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_issues_sev  ON sitecrawl_issues(run_id, severity DESC, code)`,

	`CREATE TABLE IF NOT EXISTS sitecrawl_links (
		run_id    TEXT    NOT NULL,
		src_id    INTEGER NOT NULL,
		dst_id    INTEGER NOT NULL,
		seq       INTEGER NOT NULL DEFAULT 0,
		placement INTEGER NOT NULL DEFAULT 0,
		flags     INTEGER NOT NULL DEFAULT 0,
		anchor    TEXT    NOT NULL DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_links_src ON sitecrawl_links(run_id, src_id)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_links_dst ON sitecrawl_links(run_id, dst_id)`,

	// The pause / crash checkpoint. Holding the frontier only in memory would
	// make "close the app mid-crawl" lose everything not yet fetched.
	`CREATE TABLE IF NOT EXISTS sitecrawl_frontier (
		run_id    TEXT    NOT NULL,
		url       TEXT    NOT NULL,
		depth     INTEGER NOT NULL DEFAULT 0,
		source    TEXT    NOT NULL DEFAULT 'link',
		parent_id INTEGER NOT NULL DEFAULT 0,
		seq       INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (run_id, url)
	)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_frontier_order ON sitecrawl_frontier(run_id, seq)`,

	`CREATE TABLE IF NOT EXISTS sitecrawl_dupes (
		run_id   TEXT    NOT NULL,
		group_id INTEGER NOT NULL,
		url_id   INTEGER NOT NULL,
		kind     TEXT    NOT NULL DEFAULT 'page',
		score    REAL    NOT NULL DEFAULT 1,
		PRIMARY KEY (run_id, url_id, kind)
	)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_dupes_group ON sitecrawl_dupes(run_id, kind, group_id)`,

	`CREATE TABLE IF NOT EXISTS sitecrawl_psi (
		run_id     TEXT    NOT NULL,
		url        TEXT    NOT NULL,
		strategy   TEXT    NOT NULL DEFAULT 'mobile',
		fetched_at TEXT    NOT NULL DEFAULT '',
		score      INTEGER NOT NULL DEFAULT -1,
		fcp_ms     INTEGER NOT NULL DEFAULT 0,
		lcp_ms     INTEGER NOT NULL DEFAULT 0,
		cls        REAL    NOT NULL DEFAULT 0,
		tbt_ms     INTEGER NOT NULL DEFAULT 0,
		si_ms      INTEGER NOT NULL DEFAULT 0,
		error      TEXT    NOT NULL DEFAULT '',
		PRIMARY KEY (run_id, url, strategy)
	)`,

	// PageSpeed grew from "performance score plus five lab numbers" to what
	// Screaming Frog reports: all four Lighthouse categories, the Chrome User
	// Experience Report's field data, and per-URL opportunities. Appended as
	// ALTER steps because schema.Migrate runs each step exactly once per
	// database — a rewritten CREATE would be a no-op on every file that already
	// has the table, and then every read here would fail with "no such column".
	//
	// -1 is "not measured", which is not 0: a page with no field data and a page
	// that really scored 0 have to stay distinguishable.
	`ALTER TABLE sitecrawl_psi ADD COLUMN a11y_score INTEGER NOT NULL DEFAULT -1`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN seo_score INTEGER NOT NULL DEFAULT -1`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN bp_score INTEGER NOT NULL DEFAULT -1`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN crux_source TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN crux_verdict TEXT NOT NULL DEFAULT ''`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN crux_lcp_ms INTEGER NOT NULL DEFAULT -1`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN crux_inp_ms INTEGER NOT NULL DEFAULT -1`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN crux_cls REAL NOT NULL DEFAULT -1`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN crux_fcp_ms INTEGER NOT NULL DEFAULT -1`,
	`ALTER TABLE sitecrawl_psi ADD COLUMN crux_ttfb_ms INTEGER NOT NULL DEFAULT -1`,

	// One row per (URL, audit): the Opportunities tab groups them site-wide,
	// and the detail view lists the URLs behind one audit.
	`CREATE TABLE IF NOT EXISTS sitecrawl_psi_opps (
		run_id        TEXT    NOT NULL,
		url           TEXT    NOT NULL,
		strategy      TEXT    NOT NULL DEFAULT 'mobile',
		audit_id      TEXT    NOT NULL,
		title         TEXT    NOT NULL DEFAULT '',
		savings_ms    INTEGER NOT NULL DEFAULT 0,
		savings_bytes INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (run_id, url, strategy, audit_id)
	)`,
	`CREATE INDEX IF NOT EXISTS sitecrawl_psi_opps_audit ON sitecrawl_psi_opps(run_id, strategy, audit_id)`,
	// The link-graph view asks for the top 3,000 pages by inlinks. Without this
	// the database read every page of the run and sorted the lot to find them;
	// with it the answer is the first 3,000 index entries. Measured on a 60,000
	// page run: 27ms → 6.7ms, and it grows with the crawl, so the users who feel
	// it are the ones with the most to look at.
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_inlinks ON sitecrawl_pages(run_id, inlinks DESC, url_id)`,

	// --- Reshaped onto core/runs. This tool already had five of the six shared
	// columns; total was the missing one. It mirrors found, because "how big
	// was this run" is the number a history row wants, while the tool's own
	// screens keep reading found and crawled separately.
	//
	// Nothing is dropped here. A crawl's tables hold tens of thousands of rows
	// the user paid for in time, and one ADD COLUMN reaches them all.
	`ALTER TABLE sitecrawl_runs ADD COLUMN total INTEGER NOT NULL DEFAULT 0`,
	`UPDATE sitecrawl_runs SET total = found`,
}

// ftsStmts create the full-text index the grid's search box uses.
//
// Separate from schemaStmts because FTS5 is a compile-time option: it is
// present in modernc.org/sqlite, but a failure here must degrade to LIKE rather
// than take the whole tool down.
var ftsStmts = []string{
	`CREATE VIRTUAL TABLE IF NOT EXISTS sitecrawl_fts USING fts5(
		url, title, meta_desc,
		content='sitecrawl_pages', content_rowid='rowid',
		tokenize='unicode61 remove_diacritics 2'
	)`,
	`CREATE TRIGGER IF NOT EXISTS sitecrawl_fts_ai AFTER INSERT ON sitecrawl_pages BEGIN
		INSERT INTO sitecrawl_fts(rowid, url, title, meta_desc)
		VALUES (new.rowid, new.url, new.title, new.meta_desc);
	END`,
	`CREATE TRIGGER IF NOT EXISTS sitecrawl_fts_ad AFTER DELETE ON sitecrawl_pages BEGIN
		INSERT INTO sitecrawl_fts(sitecrawl_fts, rowid, url, title, meta_desc)
		VALUES ('delete', old.rowid, old.url, old.title, old.meta_desc);
	END`,
	// Dropped and recreated rather than IF NOT EXISTS: the first version fired on
	// ANY update to the row, and installs that already have it would otherwise
	// keep it forever. ensureFTS replays this list on every open, so this is the
	// migration.
	`DROP TRIGGER IF EXISTS sitecrawl_fts_au`,
	// UPDATE OF is the whole point. Nothing in the crawl ever edits a page's url,
	// title or description after insert — but three things update the row:
	// inlinks and orphan during finalize, and issue counts, which rewrites EVERY
	// page of the run. Unscoped, each of those deleted and reinserted the search
	// entry for every page, so finishing a 5,000-page crawl rebuilt the entire
	// full-text index three times over for no change at all.
	`CREATE TRIGGER IF NOT EXISTS sitecrawl_fts_au
	 AFTER UPDATE OF url, title, meta_desc ON sitecrawl_pages BEGIN
		INSERT INTO sitecrawl_fts(sitecrawl_fts, rowid, url, title, meta_desc)
		VALUES ('delete', old.rowid, old.url, old.title, old.meta_desc);
		INSERT INTO sitecrawl_fts(rowid, url, title, meta_desc)
		VALUES (new.rowid, new.url, new.title, new.meta_desc);
	END`,
}

// ensureSchema replays the DDL. Every statement is idempotent, so this is safe
// to call on any entry point.
// schemaStmts are append-only migration steps: see core/schema. Never edit or
// reorder a statement that has shipped — add a new one at the end.
func ensureSchema(db *sql.DB) error {
	return schema.Migrate(db, "sitecrawl", schemaStmts)
}

// ensureFTS builds the search index, reporting whether it is usable. A build
// without FTS5 falls back to LIKE rather than failing the tool.
func ensureFTS(db *sql.DB) bool {
	for _, stmt := range ftsStmts {
		if _, err := db.Exec(stmt); err != nil {
			return false
		}
	}
	return true
}

// nowStamp is the shared stamp format. Kept as a local name only because this
// package calls it from a dozen files; the implementation is one copy now.
func nowStamp() string { return runs.Now() }

// storableOptions strips the fields that must not be written to disk.
//
// CustomHeaders is where a user puts a Cookie or Authorization to crawl a site
// behind a login. Persisting it put that credential in cleartext in the
// workspace database and — since loadRun/listRuns hand Options straight back —
// returned it to the frontend, which the project's own rule says must never be
// able to read a secret back.
//
// The cost is deliberate: replaying an old run from history no longer replays
// its headers, so the user retypes them. A credential in a plain file is worse.
func storableOptions(opts Options) Options {
	opts.CustomHeaders = nil
	return opts
}

// insertRun creates the run row. Called from the Start handshake before the job
// body is allowed to touch anything.
func insertRun(db *sql.DB, runID, seedURL, host string, opts Options) error {
	blob, err := json.Marshal(storableOptions(opts))
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO sitecrawl_runs (id, seed_url, host, options, state, phase, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		runID, seedURL, host, string(blob), StateRunning, PhasePreparing, nowStamp())
	return err
}

func setPhase(db *sql.DB, runID, phase string) {
	if _, err := db.Exec(`UPDATE sitecrawl_runs SET phase = ? WHERE id = ?`, phase, runID); err != nil {
		slog.Error("sitecrawl: set phase", "run", runID, "phase", phase, "err", err)
	}
}

// setCounters keeps the run header in step with the rows, so history and a
// resumed run agree without recounting.
func setCounters(db *sql.DB, runID string, found, crawled, issues int, nextURLID int64) {
	// total mirrors found for the shared history table; the tool's own screens
	// keep reading found/crawled, which mean different things to a crawl.
	if _, err := db.Exec(`UPDATE sitecrawl_runs
		SET found = ?, total = ?, crawled = ?, issue_total = ?, next_url_id = ? WHERE id = ?`,
		found, found, crawled, issues, nextURLID, runID); err != nil {
		slog.Error("sitecrawl: set counters", "run", runID, "err", err)
	}
}

// pauseRun records a pause. The job goroutine stays alive holding the frontier,
// but the row has to say so too: closing the app must leave something resumable.
func pauseRun(db *sql.DB, runID string) {
	// If this never lands the crawl looks like it is still running, so the
	// resume the user is about to reach for is refused and the frontier is
	// stranded.
	if _, err := db.Exec(`UPDATE sitecrawl_runs SET state = ?, resumable = 1 WHERE id = ?`,
		StatePaused, runID); err != nil {
		slog.Error("sitecrawl: pause run", "run", runID, "err", err)
	}
}

func resumeRunState(db *sql.DB, runID string) {
	if _, err := db.Exec(`UPDATE sitecrawl_runs SET state = ?, resumable = 0 WHERE id = ?`,
		StateRunning, runID); err != nil {
		slog.Error("sitecrawl: resume run state", "run", runID, "err", err)
	}
}

// finishRun closes a run out. resumable is set when the frontier still holds
// work, so the history row can offer Resume instead of only Recrawl.
func finishRun(db *sql.DB, runID, state, reason, errMsg string, resumable bool, startedAt time.Time) {
	res := 0
	if resumable {
		res = 1
	}
	// A closing state that never lands leaves the crawl looking busy for ever:
	// it cannot be deleted, cannot be cleared, and nothing says why.
	if _, err := db.Exec(`
		UPDATE sitecrawl_runs
		   SET state = ?, phase = ?, stop_reason = ?, error = ?, resumable = ?,
		       finished_at = ?, duration_ms = ?
		 WHERE id = ?`,
		state, PhaseDone, reason, errMsg, res, nowStamp(),
		time.Since(startedAt).Milliseconds(), runID); err != nil {
		slog.Error("sitecrawl: finish run", "run", runID, "state", state, "err", err)
	}
}

var errRunBusy = errors.New("sitecrawl: this crawl is already running")

// claimRun takes ownership of an existing run for a resume.
//
// A guarded UPDATE rather than a read-then-write: two Resume clicks a
// millisecond apart would both see "paused" and both start a job, and there are
// no transactions in this codebase to lean on.
func claimRun(db *sql.DB, runID string) error {
	res, err := db.Exec(`
		UPDATE sitecrawl_runs SET state = ?, resumable = 0, finished_at = NULL
		 WHERE id = ? AND state != ?`, StateRunning, runID, StateRunning)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errRunBusy
	}
	return nil
}

// recoverStaleRuns runs once per workspace, on first DB touch.
//
// A job dies with the process, so any row still saying "running" is from a
// previous session. Rows with a frontier left are marked paused-and-resumable
// rather than cancelled: the user closed the app mid-crawl and expects to pick
// it back up.
func recoverStaleRuns(db *sql.DB) {
	rows, err := db.Query(`
		SELECT r.id, EXISTS(SELECT 1 FROM sitecrawl_frontier f WHERE f.run_id = r.id)
		  FROM sitecrawl_runs r WHERE r.state = ?`, StateRunning)
	if err != nil {
		return
	}
	type stale struct {
		id       string
		hasQueue bool
	}
	var list []stale
	for rows.Next() {
		var s stale
		if err := rows.Scan(&s.id, &s.hasQueue); err != nil {
			slog.Error("sitecrawl: scan stale run", "err", err)
			break
		}
		list = append(list, s)
	}
	// A cursor that breaks mid-drain returns a short list, and every run it
	// missed stays "running" for ever: undeletable, unclearable, unresumable.
	// rankcheck already checks this in the same place; the fix never reached
	// here, which is the whole reason this layer is being shared.
	if err := rows.Err(); err != nil {
		slog.Error("sitecrawl: find stale runs", "err", err)
	}
	// Drain fully before writing: the workspace DB allows a single connection,
	// so holding this cursor open while updating would deadlock.
	rows.Close()

	for _, s := range list {
		if s.hasQueue {
			// Paused, not interrupted: there is a frontier to pick up and the
			// screen already offers Resume for exactly this shape.
			if _, err := db.Exec(`UPDATE sitecrawl_runs SET state = ?, resumable = 1 WHERE id = ?`,
				StatePaused, s.id); err != nil {
				slog.Error("sitecrawl: mark stale run resumable", "run", s.id, "err", err)
			}
			continue
		}
		// Nothing left to resume. Interrupted rather than cancelled — the user
		// did not stop this, the app closed on it.
		if _, err := db.Exec(`UPDATE sitecrawl_runs SET state = ?, phase = ?, finished_at = ? WHERE id = ?`,
			StateInterrupted, PhaseDone, nowStamp(), s.id); err != nil {
			slog.Error("sitecrawl: close stale run", "run", s.id, "err", err)
		}
	}
}

// loadRun reads one run header.
func loadRun(db *sql.DB, runID string) (RunSummary, error) {
	var (
		r          RunSummary
		optsBlob   string
		finishedAt sql.NullString
		resumable  int
	)
	err := db.QueryRow(`
		SELECT id, seed_url, host, options, state, phase, stop_reason, resumable,
		       found, crawled, issue_total, started_at, finished_at, duration_ms
		  FROM sitecrawl_runs WHERE id = ?`, runID).
		Scan(&r.ID, &r.SeedURL, &r.Host, &optsBlob, &r.State, &r.Phase, &r.StopReason,
			&resumable, &r.Found, &r.Crawled, &r.Issues, &r.StartedAt, &finishedAt, &r.DurationMs)
	if err != nil {
		return RunSummary{}, err
	}
	r.Resumable = resumable == 1
	r.FinishedAt = finishedAt.String
	json.Unmarshal([]byte(optsBlob), &r.Options)
	// Rows written before insertRun started stripping them still carry headers.
	r.Options = storableOptions(r.Options)
	r.Mode = r.Options.Mode
	return r, nil
}

// listRuns returns the history, newest first.
func listRuns(db *sql.DB) ([]RunSummary, error) {
	rows, err := db.Query(`
		SELECT id, seed_url, host, options, state, phase, stop_reason, resumable,
		       found, crawled, issue_total, started_at, finished_at, duration_ms
		  FROM sitecrawl_runs WHERE state != ? ORDER BY started_at DESC`, StateDeleting)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []RunSummary{}
	for rows.Next() {
		var (
			r          RunSummary
			optsBlob   string
			finishedAt sql.NullString
			resumable  int
		)
		if err := rows.Scan(&r.ID, &r.SeedURL, &r.Host, &optsBlob, &r.State, &r.Phase,
			&r.StopReason, &resumable, &r.Found, &r.Crawled, &r.Issues,
			&r.StartedAt, &finishedAt, &r.DurationMs); err != nil {
			return nil, err
		}
		r.Resumable = resumable == 1
		r.FinishedAt = finishedAt.String
		json.Unmarshal([]byte(optsBlob), &r.Options)
		r.Options = storableOptions(r.Options)
		r.Mode = r.Options.Mode
		out = append(out, r)
	}
	return out, rows.Err()
}

// runSizes estimates each run's share of the database file.
//
// SQLite cannot report per-row storage without the dbstat extension, so this
// costs the row counts and multiplies by measured averages. It only has to be
// good enough to answer "which crawl is eating my disk?" — the History row
// labels it as approximate.
func runSizes(db *sql.DB) map[string]int64 {
	const (
		bytesPerPage = 2400 // the JSON blob dominates
		bytesPerLink = 90
		bytesPerURL  = 90
	)
	out := map[string]int64{}
	// These look like 3 × (number of runs) table scans and are not: every one of
	// these tables is indexed by run_id, so each subquery counts index entries
	// in a range. Measured against a version that pre-aggregated with GROUP BY
	// and joined — 120 runs × 1200 pages — this form won, 38ms to 48ms, because
	// the alternative has to materialise three temporary tables. Left alone.
	rows, err := db.Query(`
		SELECT r.id,
		       (SELECT COUNT(*) FROM sitecrawl_pages p WHERE p.run_id = r.id),
		       (SELECT COUNT(*) FROM sitecrawl_links l WHERE l.run_id = r.id),
		       (SELECT COUNT(*) FROM sitecrawl_urls u WHERE u.run_id = r.id)
		  FROM sitecrawl_runs r`)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var pages, links, urls int64
		if err := rows.Scan(&id, &pages, &links, &urls); err != nil {
			slog.Error("sitecrawl: scan run size", "err", err)
			break
		}
		out[id] = pages*bytesPerPage + links*bytesPerLink + urls*bytesPerURL
	}
	// Without this a broken cursor simply stopped early and the runs it never
	// reached showed "0 B" — a number, not a gap, so nothing looked wrong.
	if err := rows.Err(); err != nil {
		slog.Error("sitecrawl: measure run sizes", "err", err)
	}
	return out
}

// deleteRunChunk removes up to `limit` rows of one run, reporting whether any
// were left.
//
// Chunked because a single DELETE over 5M link rows holds the workspace's only
// connection for seconds, freezing every other tool's grid.
func deleteRunChunk(db *sql.DB, runID string, limit int) (bool, error) {
	for _, table := range []string{"sitecrawl_links", "sitecrawl_issues", "sitecrawl_dupes",
		"sitecrawl_frontier", "sitecrawl_psi_opps", "sitecrawl_psi", "sitecrawl_pages", "sitecrawl_urls"} {
		res, err := db.Exec(fmt.Sprintf(
			`DELETE FROM %s WHERE rowid IN (SELECT rowid FROM %s WHERE run_id = ? LIMIT ?)`,
			table, table), runID, limit)
		if err != nil {
			return false, err
		}
		if n, _ := res.RowsAffected(); n > 0 {
			return true, nil
		}
	}
	_, err := db.Exec(`DELETE FROM sitecrawl_runs WHERE id = ?`, runID)
	return false, err
}

// claimForDelete flips a finished run to "deleting" and reports whether this
// caller is the one that got it. The WHERE guard is what makes it atomic: a run
// that is running, paused, already being deleted or already gone yields false.
func claimForDelete(db *sql.DB, runID string) (bool, error) {
	res, err := db.Exec(
		`UPDATE sitecrawl_runs SET state = ? WHERE id = ? AND state NOT IN `+
			activeStates+` AND state != ?`,
		StateDeleting, runID, StateDeleting)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// activeStates is the one SQL fragment every "is something running?" guard
// shares, so they cannot drift apart.
var activeStates = "('" + StateRunning + "','" + StatePaused + "')"

// runIsActive fails CLOSED. When the query cannot run at all, the honest answer
// is "assume it is busy": reading a failed scan as zero is how a crawl that is
// still writing gets deleted out from under its own job.
func runIsActive(db *sql.DB, runID string) bool {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_runs WHERE id = ? AND state IN `+activeStates,
		runID).Scan(&n); err != nil {
		slog.Error("sitecrawl: check run is active", "run", runID, "err", err)
		return true
	}
	return n > 0
}

// anyRunActive fails CLOSED for the same reason as runIsActive: this one guards
// Clear history, so a wrong "no" wipes every crawl the user has.
func anyRunActive(db *sql.DB) bool {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_runs WHERE state IN ` + activeStates).Scan(&n); err != nil {
		slog.Error("sitecrawl: check for active runs", "err", err)
		return true
	}
	return n > 0
}

// --- frontier checkpoint ---

// saveFrontier replaces the stored queue for a run.
//
// Chunked at 500 rows per statement: one statement for a 30k-item queue would
// block reads for as long as it runs, and SQLite's bind-variable limit is real.
func saveFrontier(db *sql.DB, runID string, items []frontierItem) error {
	if _, err := db.Exec(`DELETE FROM sitecrawl_frontier WHERE run_id = ?`, runID); err != nil {
		return err
	}
	const batch = 500
	for start := 0; start < len(items); start += batch {
		end := start + batch
		if end > len(items) {
			end = len(items)
		}
		var sb strings.Builder
		args := make([]any, 0, (end-start)*6)
		sb.WriteString(`INSERT OR REPLACE INTO sitecrawl_frontier(run_id, url, depth, source, parent_id, seq) VALUES `)
		for i, it := range items[start:end] {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString("(?,?,?,?,?,?)")
			args = append(args, runID, it.URL, it.Depth, it.Source, it.ParentID, start+i)
		}
		if _, err := db.Exec(sb.String(), args...); err != nil {
			return err
		}
	}
	return nil
}

func loadFrontier(db *sql.DB, runID string) ([]frontierItem, error) {
	rows, err := db.Query(`
		SELECT url, depth, source, parent_id FROM sitecrawl_frontier
		 WHERE run_id = ? ORDER BY seq`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []frontierItem
	for rows.Next() {
		var it frontierItem
		if err := rows.Scan(&it.URL, &it.Depth, &it.Source, &it.ParentID); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func clearFrontier(db *sql.DB, runID string) {
	db.Exec(`DELETE FROM sitecrawl_frontier WHERE run_id = ?`, runID)
}

// loadSeen rebuilds the visited set for a resumed run: every URL the dictionary
// already knows, with its id, so ids stay stable across the restart.
//
// The map must be keyed the same way the live frontier keys it — by frontierKey,
// not by the raw URL stored in the column. They differ for any URL with a
// fragment, a tracking param, several query params or an uppercase host, and a
// mismatch is silent: restore's lookup misses, the item resumes with ID 0, and
// writePages upserts every one of them onto the same (run_id, url_id) row.
//
// ignoreQuery must be the resumed run's own option, since it changes what
// frontierKey considers the same URL.
func loadSeen(db *sql.DB, runID string, ignoreQuery bool) (map[string]int64, int64, error) {
	rows, err := db.Query(`SELECT id, url FROM sitecrawl_urls WHERE run_id = ?`, runID)
	if err != nil {
		return nil, 1, err
	}
	defer rows.Close()
	seen := map[string]int64{}
	var maxID int64
	for rows.Next() {
		var id int64
		var u string
		if err := rows.Scan(&id, &u); err != nil {
			return nil, 1, err
		}
		seen[frontierKey(u, ignoreQuery)] = id
		if id > maxID {
			maxID = id
		}
	}
	return seen, maxID + 1, rows.Err()
}
