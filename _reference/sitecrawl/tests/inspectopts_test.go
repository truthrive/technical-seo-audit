package sitecrawl

// Prints the options a stored run actually used. Every run snapshots them, so
// this settles "what was the app configured with?" without asking anyone to
// remember.

import (
	"database/sql"
	"encoding/json"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestInspectRunOptions(t *testing.T) {
	path := os.Getenv("CRAWL_DB")
	if path == "" {
		t.Skip("set CRAWL_DB=<workspace.db>")
	}
	db, err := sql.Open("sqlite", path+"?_pragma=query_only(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT id, seed_url, started_at, duration_ms, crawled, options
		  FROM sitecrawl_runs ORDER BY started_at DESC LIMIT 4`)
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, seed, started, optsJSON string
		var durMs int64
		var crawled int
		if err := rows.Scan(&id, &seed, &started, &durMs, &crawled, &optsJSON); err != nil {
			t.Logf("scan: %v", err)
			continue
		}
		var o Options
		if err := json.Unmarshal([]byte(optsJSON), &o); err != nil {
			t.Logf("%s: options unreadable: %v", started, err)
			continue
		}
		t.Logf("%s  %s  crawled=%d  took=%.1fs", started[:19], trim(seed, 34), crawled,
			float64(durMs)/1000)
		t.Logf("    threads=%d timeout=%ds retries=%d delayMs=%d maxDepth=%d maxURLs=%d",
			o.Concurrency, o.TimeoutSec, o.Retries, o.CrawlDelayMs, o.MaxDepth, o.MaxURLs)
		t.Logf("    js=%v jsWaitMs=%d jsConcurrency=%d jsMaxPages=%d | external=%v images=%v css=%v js=%v",
			o.EnableJavaScript, o.JSWaitMs, o.JSConcurrency, o.JSMaxPages,
			o.CrawlExternal, o.CrawlImages, o.CrawlCSS, o.CrawlJS)
		t.Logf("    robots=%v sitemaps=%v dup=%v ua=%q headers=%d",
			o.RespectRobots, o.DiscoverSitemaps, o.EnableDuplication, o.UserAgent, len(o.CustomHeaders))

		// Rendering is charged outside the fetch timer, so a slow render shows up
		// as wall-clock with no response_ms to explain it.
		var rendered, fetched int
		var sumMs, avgMs float64
		db.QueryRow(`SELECT COALESCE(SUM(rendered),0), COUNT(*), COALESCE(SUM(response_ms),0),
				COALESCE(AVG(response_ms),0)
			  FROM sitecrawl_pages WHERE run_id = ? AND status > 0`, id).
			Scan(&rendered, &fetched, &sumMs, &avgMs)
		t.Logf("    fetched=%d rendered=%d avgResponse=%.0fms sumResponse=%.0fs",
			fetched, rendered, avgMs, sumMs/1000)
		if fetched < 100 {
			continue
		}
		// Where the slow requests actually are: an average hides whether every
		// request was slow or a minority timed out.
		buckets, err := db.Query(`SELECT
				CASE WHEN response_ms < 500 THEN 'a <0.5s'
				     WHEN response_ms < 2000 THEN 'b 0.5-2s'
				     WHEN response_ms < 5000 THEN 'c 2-5s'
				     WHEN response_ms < 10000 THEN 'd 5-10s'
				     ELSE 'e >=10s' END AS bucket,
				COUNT(*), CAST(SUM(response_ms)/1000 AS INT)
			  FROM sitecrawl_pages WHERE run_id = ? AND status > 0
			 GROUP BY bucket ORDER BY bucket`, id)
		if err == nil {
			for buckets.Next() {
				var b string
				var n, secs int
				buckets.Scan(&b, &n, &secs)
				t.Logf("      %-9s %5d requests, %5ds total", b[2:], n, secs)
			}
			buckets.Close()
		}
		slow, err := db.Query(`SELECT url, status, COALESCE(error_type,''), response_ms
			  FROM sitecrawl_pages WHERE run_id = ? AND response_ms >= 10000
			 ORDER BY response_ms DESC LIMIT 10`, id)
		if err == nil {
			for slow.Next() {
				var u, et string
				var st, ms int
				slow.Scan(&u, &st, &et, &ms)
				t.Logf("      SLOW %6dms status=%-3d %-18s %s", ms, st, et, trim(u, 72))
			}
			slow.Close()
		}
		var timeouts int
		db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages
			 WHERE run_id = ? AND error_type = ?`, id, ErrTimeout).Scan(&timeouts)
		t.Logf("      pages recorded as timeout: %d", timeouts)

		kinds, err := db.Query(`SELECT kind, COUNT(*), CAST(AVG(response_ms) AS INT)
			  FROM sitecrawl_pages WHERE run_id = ? AND status > 0
			 GROUP BY kind ORDER BY AVG(response_ms) DESC`, id)
		if err == nil {
			for kinds.Next() {
				var k string
				var n, avg int
				kinds.Scan(&k, &n, &avg)
				t.Logf("      kind %-6s %5d requests, avg %dms", k, n, avg)
			}
			kinds.Close()
		}
	}
}
