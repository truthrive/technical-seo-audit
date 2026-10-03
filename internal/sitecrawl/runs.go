package sitecrawl

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

// schemaStmts is the whole schema, declared as ordered migration steps applied
// via standalone.Migrate.
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

	// The URL dictionary. sitecrawl_links stores two integers instead of two strings.
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
	`CREATE INDEX IF NOT EXISTS sitecrawl_pages_inlinks ON sitecrawl_pages(run_id, inlinks DESC, url_id)`,

	`ALTER TABLE sitecrawl_runs ADD COLUMN total INTEGER NOT NULL DEFAULT 0`,
	`UPDATE sitecrawl_runs SET total = found`,
}

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
	`DROP TRIGGER IF EXISTS sitecrawl_fts_au`,
	`CREATE TRIGGER IF NOT EXISTS sitecrawl_fts_au
	 AFTER UPDATE OF url, title, meta_desc ON sitecrawl_pages BEGIN
		INSERT INTO sitecrawl_fts(sitecrawl_fts, rowid, url, title, meta_desc)
		VALUES ('delete', old.rowid, old.url, old.title, old.meta_desc);
		INSERT INTO sitecrawl_fts(rowid, url, title, meta_desc)
		VALUES (new.rowid, new.url, new.title, new.meta_desc);
	END`,
}

// ensureSchema runs the schema migration steps through standalone.Migrate.
func ensureSchema(db *sql.DB) error {
	return standalone.Migrate(db, "sitecrawl", schemaStmts)
}

// ensureFTS builds the search index, returning whether it succeeded.
func ensureFTS(db *sql.DB) bool {
	for _, stmt := range ftsStmts {
		if _, err := db.Exec(stmt); err != nil {
			return false
		}
	}
	return true
}

func nowStamp() string { return standalone.Now() }

func storableOptions(opts Options) Options {
	opts.CustomHeaders = nil
	return opts
}

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

func setCounters(db *sql.DB, runID string, found, crawled, issues int, nextURLID int64) {
	if _, err := db.Exec(`UPDATE sitecrawl_runs
		SET found = ?, total = ?, crawled = ?, issue_total = ?, next_url_id = ? WHERE id = ?`,
		found, found, crawled, issues, nextURLID, runID); err != nil {
		slog.Error("sitecrawl: set counters", "run", runID, "err", err)
	}
}

func pauseRun(db *sql.DB, runID string) {
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

func finishRun(db *sql.DB, runID, state, reason, errMsg string, resumable bool, startedAt time.Time) {
	res := 0
	if resumable {
		res = 1
	}
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

var errRunBusy = ErrRunBusy

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

// RecoverStaleRuns inspects the database for runs left in 'running' state from
// a previous session. Runs with a saved frontier checkpoint become StatePaused
// and resumable; runs without saved frontier become StateInterrupted.
func RecoverStaleRuns(db *sql.DB) error {
	rows, err := db.Query(`
		SELECT r.id, EXISTS(SELECT 1 FROM sitecrawl_frontier f WHERE f.run_id = r.id)
		  FROM sitecrawl_runs r WHERE r.state = ?`, StateRunning)
	if err != nil {
		return err
	}
	defer rows.Close()

	type stale struct {
		id       string
		hasQueue bool
	}
	var list []stale
	for rows.Next() {
		var s stale
		if err := rows.Scan(&s.id, &s.hasQueue); err != nil {
			return err
		}
		list = append(list, s)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, s := range list {
		if s.hasQueue {
			if _, err := db.Exec(`UPDATE sitecrawl_runs SET state = ?, resumable = 1 WHERE id = ?`,
				StatePaused, s.id); err != nil {
				return err
			}
			continue
		}
		if _, err := db.Exec(`UPDATE sitecrawl_runs SET state = ?, phase = ?, finished_at = ? WHERE id = ?`,
			StateInterrupted, PhaseDone, nowStamp(), s.id); err != nil {
			return err
		}
	}
	return nil
}

func recoverStaleRuns(db *sql.DB) {
	_ = RecoverStaleRuns(db)
}

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
	r.Options = storableOptions(r.Options)
	r.Mode = r.Options.Mode
	return r, nil
}

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

func runSizes(db *sql.DB) map[string]int64 {
	const (
		bytesPerPage = 2400
		bytesPerLink = 90
		bytesPerURL  = 90
	)
	out := map[string]int64{}
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
	if err := rows.Err(); err != nil {
		slog.Error("sitecrawl: measure run sizes", "err", err)
	}
	return out
}

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

var activeStates = "('" + StateRunning + "','" + StatePaused + "')"

func runIsActive(db *sql.DB, runID string) bool {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_runs WHERE id = ? AND state IN `+activeStates,
		runID).Scan(&n); err != nil {
		slog.Error("sitecrawl: check run is active", "run", runID, "err", err)
		return true
	}
	return n > 0
}

func anyRunActive(db *sql.DB) bool {
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_runs WHERE state IN ` + activeStates).Scan(&n); err != nil {
		slog.Error("sitecrawl: check for active runs", "err", err)
		return true
	}
	return n > 0
}

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
