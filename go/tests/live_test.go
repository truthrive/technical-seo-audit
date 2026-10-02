package sitecrawl

// Live crawl probe. Gated behind CRAWL_LIVE so it never runs unattended — it
// sends real traffic to a real site. Used to answer "why is this particular
// crawl slow?", which a synthetic fixture cannot.
//
//	CRAWL_LIVE=https://example.com/ CRAWL_LIVE_MAX=300 go test -tags nodynamic \
//	  -run TestCrawlLive -v -timeout 900s ./internal/tools/sitecrawl/

import (
	"database/sql"
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"

	"onescout/desktop/internal/core/jobs"
	"onescout/desktop/internal/core/workspace"
)

// liveService is newTestService with one extra: CRAWL_LIVE_APPDATA points the
// run at an existing app data directory instead of an empty one.
//
// A fresh database is not the environment the app runs in. The user's workspace
// had 28 runs behind it — 16k pages and 248k link rows — and every flush of a
// new crawl inserts into those same indexed tables, on the goroutine that also
// dispatches work. Pointing a probe at a COPY of the real thing is the only way
// to measure that.
func liveService(t *testing.T) *Service {
	t.Helper()
	appData := os.Getenv("CRAWL_LIVE_APPDATA")
	if appData == "" {
		appData = t.TempDir()
	} else {
		t.Logf("using existing app data: %s", appData)
	}
	// HOME cũng phải trỏ vào đây: trên macOS os.UserConfigDir() bỏ qua cả
	// APPDATA lẫn XDG_CONFIG_HOME (xem internal/testutil/configdir.go).
	t.Setenv("APPDATA", appData)
	t.Setenv("XDG_CONFIG_HOME", appData)
	t.Setenv("HOME", appData)

	wm, err := workspace.NewManager()
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	t.Cleanup(wm.Close)
	s := &Service{
		Workspaces: wm,
		Jobs:       jobs.NewRunner(&stubEmitter{}),
		secrets:    newFakeVault(),
		Files:      savePath{dir: t.TempDir()},
	}
	if _, _, err := s.workspace(); err != nil {
		t.Fatalf("workspace init: %v", err)
	}
	return s
}

func TestCrawlLive(t *testing.T) {
	seed := os.Getenv("CRAWL_LIVE")
	if seed == "" {
		t.Skip("set CRAWL_LIVE=<url> to probe a real site")
	}
	maxURLs := 300
	if v := os.Getenv("CRAWL_LIVE_MAX"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			maxURLs = n
		}
	}
	threads := 20
	if v := os.Getenv("CRAWL_LIVE_THREADS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			threads = n
		}
	}

	svc := liveService(t)
	opts := svc.DefaultOptions()
	opts.MaxURLs = maxURLs
	opts.Concurrency = threads
	if v := os.Getenv("CRAWL_LIVE_DEPTH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			opts.MaxDepth = n
		}
	}
	// Defaults otherwise — including duplication analysis, which is on in the
	// app and is a whole phase a probe that switches it off never measures.
	if os.Getenv("CRAWL_LIVE_NODUP") != "" {
		opts.EnableDuplication = false
	}

	start := time.Now()
	started, err := svc.Start([]string{seed}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Optionally reproduce what the open Site Crawl screen does while a crawl
	// runs: refetch the facet counts and the visible row window every time the
	// revision moves. A headless probe never pays this, and it is the one cost
	// that grows with the number of rows already crawled.
	stopUI := make(chan struct{})
	if os.Getenv("CRAWL_LIVE_UI") != "" {
		go func() {
			var n int
			var facetTotal, rowsTotal time.Duration
			for {
				select {
				case <-stopUI:
					if n > 0 {
						t.Logf("UI load: %d refreshes, avg Facets=%v avg Rows=%v",
							n, facetTotal/time.Duration(n), rowsTotal/time.Duration(n))
					}
					return
				case <-time.After(500 * time.Millisecond):
				}
				a := time.Now()
				svc.Facets(started.RunID)
				fd := time.Since(a)
				b := time.Now()
				svc.Rows(RowQuery{RunID: started.RunID, Tab: TabInternal, Limit: 100, WantTotal: true})
				rd := time.Since(b)
				n++
				facetTotal += fd
				rowsTotal += rd
				if n%20 == 0 {
					st, _ := svc.Status(started.RunID)
					t.Logf("  [UI] after %-5d rows: Facets=%-8v Rows=%v", st.Crawled,
						fd.Round(time.Millisecond), rd.Round(time.Millisecond))
				}
			}
		}()
	}

	// Sample the rate while it runs, so a crawl that starts fast and then
	// collapses is distinguishable from one that was slow throughout.
	done := make(chan struct{})
	go func() {
		defer close(done)
		last, lastAt := 0, time.Now()
		for {
			select {
			case <-done:
				return
			case <-time.After(2 * time.Second):
			}
			st, err := svc.Status(started.RunID)
			if err != nil {
				return
			}
			now := time.Now()
			t.Logf("  t=%-6s phase=%-10s crawled=%-5d found=%-5d  (%.1f/s over the last %s)",
				now.Sub(start).Round(time.Second), st.Phase, st.Crawled, st.Found,
				float64(st.Crawled-last)/now.Sub(lastAt).Seconds(),
				now.Sub(lastAt).Round(time.Second))
			last, lastAt = st.Crawled, now
			switch st.State {
			case StateCompleted, StateFailed, StateStopped:
				return
			}
		}
	}()

	waitLong(t, svc, started.RunID, 15*time.Minute)
	<-done
	close(stopUI)
	elapsed := time.Since(start)

	st, _ := svc.Status(started.RunID)
	t.Logf("FINISHED %s in %s — %.1f URL/s (%d threads, cap %d)",
		st.State, elapsed.Round(time.Millisecond),
		float64(st.Crawled)/elapsed.Seconds(), threads, maxURLs)

	db, err := svc.readDB()
	if err != nil {
		t.Fatalf("readDB: %v", err)
	}
	var reason, runErr string
	db.QueryRow(`SELECT COALESCE(stop_reason,''), COALESCE(error,'') FROM sitecrawl_runs WHERE id = ?`,
		started.RunID).Scan(&reason, &runErr)
	t.Logf("stopReason=%q error=%q", reason, runErr)
	reportGroup(t, db, "status", `SELECT CAST(status AS TEXT), COUNT(*), CAST(AVG(response_ms) AS INT)
		FROM sitecrawl_pages WHERE run_id = ? GROUP BY status ORDER BY COUNT(*) DESC`, started.RunID)
	reportGroup(t, db, "error", `SELECT error_type, COUNT(*), CAST(AVG(response_ms) AS INT)
		FROM sitecrawl_pages WHERE run_id = ? AND error_type <> '' GROUP BY error_type
		ORDER BY COUNT(*) DESC`, started.RunID)
	reportGroup(t, db, "kind", `SELECT kind, COUNT(*), CAST(AVG(response_ms) AS INT)
		FROM sitecrawl_pages WHERE run_id = ? GROUP BY kind ORDER BY COUNT(*) DESC`, started.RunID)
	reportGroup(t, db, "internal", `SELECT CASE is_internal WHEN 1 THEN 'own-site' ELSE 'external' END,
		COUNT(*), CAST(AVG(response_ms) AS INT)
		FROM sitecrawl_pages WHERE run_id = ? GROUP BY is_internal`, started.RunID)

	// Every distinct external host costs one robots.txt fetch, on a worker, with
	// the full timeout available to a domain that no longer answers.
	var extHosts int
	db.QueryRow(`SELECT COUNT(DISTINCT SUBSTR(url, 1, INSTR(SUBSTR(url, 9), '/') + 7))
		FROM sitecrawl_pages WHERE run_id = ? AND is_internal = 0`, started.RunID).Scan(&extHosts)
	t.Logf("distinct external hosts: %d", extHosts)

	// Where the wall-clock actually went, by host. One dead or slow third-party
	// domain can cost more than the whole site put together.
	rows2, err := db.Query(`SELECT
			SUBSTR(url, 1, INSTR(SUBSTR(url, 9), '/') + 7) AS host,
			COUNT(*), SUM(response_ms)
		FROM sitecrawl_pages WHERE run_id = ?
		GROUP BY host ORDER BY SUM(response_ms) DESC LIMIT 10`, started.RunID)
	if err == nil {
		defer rows2.Close()
		t.Log("time spent by host (sum of response times):")
		for rows2.Next() {
			var host string
			var n, total int
			rows2.Scan(&host, &n, &total)
			t.Logf("  %8.1fs  %5d req  %s", float64(total)/1000, n, trim(host, 70))
		}
	}

	// The slowest individual responses: one hanging endpoint can hold a whole
	// worker for the request timeout, over and over.
	rows, err := db.Query(`SELECT url, status, response_ms FROM sitecrawl_pages
		WHERE run_id = ? ORDER BY response_ms DESC LIMIT 8`, started.RunID)
	if err == nil {
		defer rows.Close()
		t.Log("slowest responses:")
		for rows.Next() {
			var u string
			var status, ms int
			rows.Scan(&u, &status, &ms)
			t.Logf("  %6dms  %3d  %s", ms, status, trim(u, 90))
		}
	}
}

func reportGroup(t *testing.T, db *sql.DB, label, query, runID string) {
	t.Helper()
	rows, err := db.Query(query, runID)
	if err != nil {
		t.Logf("%s: query failed: %v", label, err)
		return
	}
	defer rows.Close()
	var lines []string
	for rows.Next() {
		var key string
		var n, avg int
		if err := rows.Scan(&key, &n, &avg); err != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s=%d (avg %dms)", key, n, avg))
	}
	sort.Strings(lines)
	if len(lines) > 0 {
		t.Logf("%s: %v", label, lines)
	}
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
