package sitecrawl

import (
	"database/sql"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

func ftsDB(t *testing.T) *sql.DB {
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
	if !ensureFTS(db) {
		t.Skip("this build has no FTS5 support")
	}
	return db
}

func insertFTSPage(t *testing.T, db *sql.DB, runID string, urlID int, url, title, desc string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, title, meta_desc, data, crawled_at)
		VALUES(?,?,?,?,?,?,?)`, runID, urlID, url, title, desc, "{}", "2026-01-01")
	if err != nil {
		t.Fatalf("insertFTSPage: %v", err)
	}
}

func ftsHits(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_fts WHERE sitecrawl_fts MATCH ?`, q).Scan(&n); err != nil {
		t.Fatalf("ftsHits query (%q): %v", q, err)
	}
	return n
}

// TestFTSSurvivesCounterUpdates: search must still find a page after non-searchable
// columns are updated. The trigger must leave existing FTS entries alone.
func TestFTSSurvivesCounterUpdates(t *testing.T) {
	db := ftsDB(t)
	insertFTSPage(t, db, "run-1", 1, "https://example.com/pricing", "Pricing plans", "Our monthly pricing")

	if got := ftsHits(t, db, "pricing"); got != 1 {
		t.Fatalf("indexed %d rows after insert, want 1", got)
	}

	// Update acquisition-neutral / non-searchable columns
	for _, q := range []string{
		`UPDATE sitecrawl_pages SET inlinks = 7, inlinks_uniq = 5 WHERE run_id = 'run-1'`,
		`UPDATE sitecrawl_pages SET orphan = 1 WHERE run_id = 'run-1'`,
		`UPDATE sitecrawl_pages SET rendered = 1 WHERE run_id = 'run-1'`,
		`UPDATE sitecrawl_pages SET status = 200, status_class = 2 WHERE run_id = 'run-1'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	if got := ftsHits(t, db, "pricing"); got != 1 {
		t.Fatalf("search found %d rows after non-searchable updates, want 1 — page dropped from FTS index", got)
	}
}

// TestFTSStillFollowsSearchableEdits: updating url, title, or meta_desc must update the FTS entry.
func TestFTSStillFollowsSearchableEdits(t *testing.T) {
	db := ftsDB(t)
	insertFTSPage(t, db, "run-1", 1, "https://example.com/old", "Old unique title", "Old description")

	if got := ftsHits(t, db, "unique"); got != 1 {
		t.Fatalf("initial title search hits = %d, want 1", got)
	}

	// Update searchable column: title
	if _, err := db.Exec(`UPDATE sitecrawl_pages SET title = 'Brand new title' WHERE run_id = 'run-1'`); err != nil {
		t.Fatalf("update title: %v", err)
	}

	if got := ftsHits(t, db, "brand"); got != 1 {
		t.Fatalf("new title not searchable (%d hits), want 1", got)
	}
	if got := ftsHits(t, db, "unique"); got != 0 {
		t.Fatalf("old title still searchable (%d hits), want 0 (stale entry not removed)", got)
	}

	// Update searchable column: meta_desc
	if _, err := db.Exec(`UPDATE sitecrawl_pages SET meta_desc = 'Updated special summary' WHERE run_id = 'run-1'`); err != nil {
		t.Fatalf("update meta_desc: %v", err)
	}

	if got := ftsHits(t, db, "special"); got != 1 {
		t.Fatalf("new meta_desc not searchable (%d hits), want 1", got)
	}
}

// TestFTSFollowsDeletes: deleting the page must remove its FTS entry.
func TestFTSFollowsDeletes(t *testing.T) {
	db := ftsDB(t)
	insertFTSPage(t, db, "run-1", 1, "https://example.com/doomed", "Doomed page", "Will be deleted")

	if got := ftsHits(t, db, "doomed"); got != 1 {
		t.Fatalf("initial search hits = %d, want 1", got)
	}

	if _, err := db.Exec(`DELETE FROM sitecrawl_pages WHERE run_id = 'run-1'`); err != nil {
		t.Fatalf("delete page: %v", err)
	}

	if got := ftsHits(t, db, "doomed"); got != 0 {
		t.Fatalf("deleted page still searchable (%d hits), want 0", got)
	}
}
