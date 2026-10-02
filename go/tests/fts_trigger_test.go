package sitecrawl

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func ftsDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", t.TempDir()+"/w.db")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	if !ensureFTS(db) {
		t.Skip("this build has no FTS5")
	}
	return db
}

func insertFTSPage(t *testing.T, db *sql.DB, runID string, urlID int, url, title string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, title, meta_desc, data, crawled_at)
		VALUES(?,?,?,?,?,?,?)`, runID, urlID, url, title, "desc", "{}", "2026-01-01")
	if err != nil {
		t.Fatal(err)
	}
}

func ftsHits(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_fts WHERE sitecrawl_fts MATCH ?`, q).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// Search must still find a page after the counter columns are rewritten — the
// narrowed trigger has to leave existing entries alone, not drop them.
func TestFTSSurvivesCounterUpdates(t *testing.T) {
	db := ftsDB(t)
	insertFTSPage(t, db, "run-1", 1, "https://site.vn/pricing", "Pricing page")

	if got := ftsHits(t, db, "pricing"); got != 1 {
		t.Fatalf("indexed %d rows after insert, want 1", got)
	}

	// Exactly what finalize and syncIssueCounts do: touch everything except the
	// three searchable columns.
	for _, q := range []string{
		`UPDATE sitecrawl_pages SET inlinks = 7, inlinks_uniq = 5 WHERE run_id = 'run-1'`,
		`UPDATE sitecrawl_pages SET orphan = 1 WHERE run_id = 'run-1'`,
		`UPDATE sitecrawl_pages SET issue_count = 3, issue_max_sev = 2 WHERE run_id = 'run-1'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	if got := ftsHits(t, db, "pricing"); got != 1 {
		t.Fatalf("search found %d rows after counter updates, want 1 — the page fell out of the index", got)
	}
}

// And it must still track a real edit to a searchable column, or search would
// silently return stale titles.
func TestFTSStillFollowsSearchableEdits(t *testing.T) {
	db := ftsDB(t)
	insertFTSPage(t, db, "run-1", 1, "https://site.vn/old", "Old title")

	if _, err := db.Exec(`UPDATE sitecrawl_pages SET title = 'Brand new title' WHERE run_id = 'run-1'`); err != nil {
		t.Fatal(err)
	}
	if got := ftsHits(t, db, "brand"); got != 1 {
		t.Fatalf("new title not searchable (%d hits) — the trigger is not firing on title", got)
	}
	if got := ftsHits(t, db, `"Old title"`); got != 0 {
		t.Fatalf("old title still searchable (%d hits) — the stale entry was not removed", got)
	}
}

// A delete must still remove the entry.
func TestFTSFollowsDeletes(t *testing.T) {
	db := ftsDB(t)
	insertFTSPage(t, db, "run-1", 1, "https://site.vn/gone", "Doomed page")
	if _, err := db.Exec(`DELETE FROM sitecrawl_pages WHERE run_id = 'run-1'`); err != nil {
		t.Fatal(err)
	}
	if got := ftsHits(t, db, "doomed"); got != 0 {
		t.Fatalf("deleted page still searchable (%d hits)", got)
	}
}
