package sitecrawl

// Read-only inspector for a real workspace database, so a "why is MY crawl
// slow?" question can be answered from the user's own data instead of a
// synthetic site. Gated behind CRAWL_DB; opens query_only so it can never write
// to a database the running app owns.
//
//	CRAWL_DB="C:\path\to\workspace.db" go test -tags nodynamic -run TestInspectDB -v ./internal/tools/sitecrawl/

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestInspectDB(t *testing.T) {
	path := os.Getenv("CRAWL_DB")
	if path == "" {
		t.Skip("set CRAWL_DB=<workspace.db> to inspect a real workspace")
	}
	db, err := sql.Open("sqlite",
		path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=query_only(1)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	if fi, err := os.Stat(path); err == nil {
		t.Logf("database size: %.1f MB", float64(fi.Size())/(1<<20))
	}

	for _, tbl := range []string{"sitecrawl_runs", "sitecrawl_pages", "sitecrawl_links",
		"sitecrawl_issues", "sitecrawl_urls", "sitecrawl_frontier"} {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + tbl).Scan(&n); err != nil {
			t.Logf("%-20s (unreadable: %v)", tbl, err)
			continue
		}
		t.Logf("%-20s %d rows", tbl, n)
	}

	cols, _ := db.Query(`SELECT name FROM pragma_table_info('sitecrawl_runs')`)
	if cols != nil {
		var names []string
		for cols.Next() {
			var n string
			cols.Scan(&n)
			names = append(names, n)
		}
		cols.Close()
		t.Logf("sitecrawl_runs columns: %v", names)
	}

	rows, err := db.Query(`SELECT id, seed_url, state, phase, COALESCE(stop_reason,''),
			found, crawled, started_at,
			CAST((julianday(COALESCE(finished_at, started_at)) - julianday(started_at)) * 86400 AS INT)
		  FROM sitecrawl_runs ORDER BY started_at DESC LIMIT 12`)
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	defer rows.Close()
	t.Log("recent runs (newest first):")
	for rows.Next() {
		var id, seed, state, phase, reason, started string
		var found, crawled, secs int
		if err := rows.Scan(&id, &seed, &state, &phase, &reason, &found, &crawled, &started, &secs); err != nil {
			t.Logf("  scan: %v", err)
			continue
		}
		var pages int
		db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, id).Scan(&pages)
		t.Logf("  %-19s %-9s %-16s took=%-8s crawled=%-6d pages=%-6d %s",
			started[:19], state, reason, (time.Duration(secs) * time.Second).String(),
			crawled, pages, trim(seed, 40))

		// The discriminator: wall-clock spent INSIDE requests versus the crawl's
		// total. A tiny share means the time went somewhere other than the network.
		if secs > 60 && pages > 100 {
			var sumMs, avgMs, maxMs, fetched int
			db.QueryRow(`SELECT COALESCE(SUM(response_ms),0), COALESCE(AVG(response_ms),0),
					COALESCE(MAX(response_ms),0), COUNT(*)
				  FROM sitecrawl_pages WHERE run_id = ? AND status > 0`, id).
				Scan(&sumMs, &avgMs, &maxMs, &fetched)
			t.Logf("      fetched=%d  avg=%dms  max=%dms  total time inside requests=%s of %s (%.1f%%)",
				fetched, avgMs, maxMs,
				(time.Duration(sumMs) * time.Millisecond).Round(time.Second),
				(time.Duration(secs) * time.Second),
				100*float64(sumMs)/float64(secs*1000))
		}
	}
}

var _ = fmt.Sprint
