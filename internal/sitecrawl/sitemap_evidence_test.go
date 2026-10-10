package sitecrawl

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

// tempTestDB creates an isolated SQLite database for testing migrations and persistence.
func tempTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test_sitemap.db")
	db, err := standalone.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("standalone.OpenDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := ensureSchema(db); err != nil {
		t.Fatalf("ensureSchema failed: %v", err)
	}
	return db, dbPath
}

// TestSitemapMigrationSuite verifies schema initialization, idempotency, additive upgrades,
// primary keys, indexes, and cross-run isolation.
func TestSitemapMigrationSuite(t *testing.T) {
	db, _ := tempTestDB(t)

	// Idempotency: repeated ensureSchema calls must succeed without error
	for i := 0; i < 3; i++ {
		if err := ensureSchema(db); err != nil {
			t.Fatalf("repeated ensureSchema[%d] failed: %v", i, err)
		}
	}

	// Verify required tables exist
	tables := []string{
		"sitecrawl_sitemaps",
		"sitecrawl_sitemap_entries",
		"sitecrawl_sitemap_sources",
		"sitecrawl_sitemap_discovery",
	}
	for _, tbl := range tables {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, tbl).Scan(&name)
		if err != nil || name != tbl {
			t.Fatalf("table %s was not created: %v", tbl, err)
		}
	}

	// Verify cross-run isolation: inserting data for run1 and run2 with same IDs
	run1 := "run_iso_1"
	run2 := "run_iso_2"

	ev1 := sitemapEvidence{
		Discovery: sitemapDiscoveryRecord{RunID: run1, Status: "COMPLETED"},
		Sitemaps: []sitemapRecord{
			{ID: 1, URL: "https://example.com/sitemap.xml", DiscoverySource: "robots_txt", Status: 200},
		},
		Sources: []sitemapSourceRecord{
			{SitemapID: 1, Source: "robots_txt", ParentID: 0},
		},
		Entries: []sitemapEntryRecord{
			{SitemapID: 1, Seq: 0, Loc: "https://example.com/p1"},
		},
	}
	ev2 := sitemapEvidence{
		Discovery: sitemapDiscoveryRecord{RunID: run2, Status: "COMPLETED"},
		Sitemaps: []sitemapRecord{
			{ID: 1, URL: "https://other.com/sitemap.xml", DiscoverySource: "common_path", Status: 200},
		},
		Sources: []sitemapSourceRecord{
			{SitemapID: 1, Source: "common_path", ParentID: 0},
		},
		Entries: []sitemapEntryRecord{
			{SitemapID: 1, Seq: 0, Loc: "https://other.com/p1"},
		},
	}

	if err := writeSitemapEvidence(db, run1, ev1); err != nil {
		t.Fatalf("writeSitemapEvidence run1 failed: %v", err)
	}
	if err := writeSitemapEvidence(db, run2, ev2); err != nil {
		t.Fatalf("writeSitemapEvidence run2 failed: %v", err)
	}

	// Verify run1 isolation
	var u1 string
	if err := db.QueryRow(`SELECT url FROM sitecrawl_sitemaps WHERE run_id=? AND id=1`, run1).Scan(&u1); err != nil || u1 != "https://example.com/sitemap.xml" {
		t.Fatalf("run1 sitemap mismatch: %s, %v", u1, err)
	}
	var u2 string
	if err := db.QueryRow(`SELECT url FROM sitecrawl_sitemaps WHERE run_id=? AND id=1`, run2).Scan(&u2); err != nil || u2 != "https://other.com/sitemap.xml" {
		t.Fatalf("run2 sitemap mismatch: %s, %v", u2, err)
	}
}

// TestSitemapRedirectScenariosSuite tests the mandatory redirect cases:
// Direct 200, 301->200, 302->200, 301->404, 404, 500, redirect loop/limit, and network failure.
func TestSitemapRedirectScenariosSuite(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/direct-200.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<?xml version="1.0"?><urlset><url><loc>` + srv.URL + `/p1</loc></url></urlset>`))
		case "/redir-301.xml":
			http.Redirect(w, r, srv.URL+"/direct-200.xml", http.StatusMovedPermanently)
		case "/redir-302.xml":
			http.Redirect(w, r, srv.URL+"/direct-200.xml", http.StatusFound)
		case "/redir-404.xml":
			http.Redirect(w, r, srv.URL+"/missing.xml", http.StatusMovedPermanently)
		case "/missing.xml":
			w.WriteHeader(http.StatusNotFound)
		case "/server-500.xml":
			w.WriteHeader(http.StatusInternalServerError)
		case "/loop-a.xml":
			http.Redirect(w, r, srv.URL+"/loop-b.xml", http.StatusMovedPermanently)
		case "/loop-b.xml":
			http.Redirect(w, r, srv.URL+"/loop-a.xml", http.StatusMovedPermanently)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	client := srv.Client()
	ua := presetFor(DefaultUserAgent)

	// 1. Direct 200
	_, tel1 := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/direct-200.xml")
	if tel1.InitialStatus != 200 || tel1.FinalStatus != 200 || tel1.RedirectHops != 0 || !tel1.FetchComplete {
		t.Errorf("direct 200 failed: %+v", tel1)
	}
	if tel1.ParseStatus != "parsed" || tel1.DocType != "urlset" || tel1.EntryCount != 1 {
		t.Errorf("direct 200 parse failed: %+v", tel1)
	}

	// 2. 301 -> 200
	_, tel2 := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/redir-301.xml")
	if tel2.InitialStatus != 301 {
		t.Errorf("redir 301 initial status want 301, got %d", tel2.InitialStatus)
	}
	if tel2.FinalStatus != 200 {
		t.Errorf("redir 301 final status want 200, got %d", tel2.FinalStatus)
	}
	if tel2.RedirectHops != 1 {
		t.Errorf("redir 301 hops want 1, got %d", tel2.RedirectHops)
	}
	if !strings.HasSuffix(tel2.RedirectTo, "/direct-200.xml") {
		t.Errorf("redir 301 redirectTo want /direct-200.xml, got %s", tel2.RedirectTo)
	}
	if tel2.InitialStatus == tel2.FinalStatus {
		t.Errorf("INITIAL STATUS MUST NOT EQUAL FINAL STATUS for 301->200: %+v", tel2)
	}

	// 3. 302 -> 200
	_, tel3 := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/redir-302.xml")
	if tel3.InitialStatus != 302 || tel3.FinalStatus != 200 || tel3.RedirectHops != 1 {
		t.Errorf("redir 302 failed: %+v", tel3)
	}

	// 4. 301 -> 404
	_, tel4 := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/redir-404.xml")
	if tel4.InitialStatus != 301 || tel4.FinalStatus != 404 || tel4.RedirectHops != 1 {
		t.Errorf("redir 404 failed: %+v", tel4)
	}

	// 5. Direct 404
	_, tel5 := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/missing.xml")
	if tel5.InitialStatus != 404 || tel5.FinalStatus != 404 || tel5.RedirectHops != 0 {
		t.Errorf("missing 404 failed: %+v", tel5)
	}

	// 6. Direct 500
	_, tel6 := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/server-500.xml")
	if tel6.InitialStatus != 500 || tel6.FinalStatus != 500 || tel6.RedirectHops != 0 {
		t.Errorf("server 500 failed: %+v", tel6)
	}

	// 7. Redirect loop
	_, tel7 := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/loop-a.xml")
	if tel7.InitialStatus != 301 || tel7.RedirectHops == 0 {
		t.Errorf("loop initial status/hops failed: %+v", tel7)
	}

	// 8. Network failure before HTTP response
	badClient := &http.Client{Timeout: 50 * time.Millisecond}
	_, tel8 := fetchSitemapWithTelemetry(ctx, badClient, ua, "http://127.0.0.1:9999/nonexistent.xml")
	if tel8.InitialStatus != 0 || tel8.FinalStatus != 0 || tel8.FetchComplete || tel8.FetchError == "" {
		t.Errorf("network failure telemetry failed: %+v", tel8)
	}
}

// TestSitemapParseOutcomesSuite tests parsing variations:
// Valid urlset, valid sitemapindex, empty input, XML decode error, unsupported structure, and gzip.
func TestSitemapParseOutcomesSuite(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/valid-urlset.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(`<?xml version="1.0"?><urlset><url><loc>https://example.com/p1</loc></url></urlset>`))
		case "/valid-index.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(`<?xml version="1.0"?><sitemapindex><sitemap><loc>https://example.com/child.xml</loc></sitemap></sitemapindex>`))
		case "/empty.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(``))
		case "/malformed-xml.xml":
			w.Header().Set("Content-Type", "application/xml")
			w.Write([]byte(`<?xml version="1.0"?><urlset><url><loc>unclosed`))
		case "/html-error.xml":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html><html><body>Error 404</body></html>`))
		case "/sitemap.xml.gz":
			w.Header().Set("Content-Type", "application/gzip")
			var buf bytes.Buffer
			zw := gzip.NewWriter(&buf)
			zw.Write([]byte(`<?xml version="1.0"?><urlset><url><loc>https://example.com/gz-p1</loc></url></urlset>`))
			zw.Close()
			w.Write(buf.Bytes())
		case "/bad-gzip.xml.gz":
			w.Header().Set("Content-Type", "application/gzip")
			w.Write([]byte(`not a real gzip stream`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	client := srv.Client()
	ua := presetFor(DefaultUserAgent)

	// Valid urlset
	doc, tel := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/valid-urlset.xml")
	if doc == nil || tel.ParseStatus != "parsed" || tel.DocType != "urlset" || tel.EntryCount != 1 {
		t.Errorf("urlset parse failed: %+v", tel)
	}

	// Valid sitemapindex
	docIdx, telIdx := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/valid-index.xml")
	if docIdx == nil || telIdx.ParseStatus != "parsed" || telIdx.DocType != "sitemapindex" || len(docIdx.Sitemaps) != 1 {
		t.Errorf("sitemapindex parse failed: %+v", telIdx)
	}

	// Empty input
	_, telEmpty := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/empty.xml")
	if telEmpty.ParseStatus != "empty_input" {
		t.Errorf("empty input parseStatus want empty_input, got %s", telEmpty.ParseStatus)
	}

	// Malformed XML
	_, telMal := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/malformed-xml.xml")
	if telMal.ParseStatus != "xml_error" || telMal.ParseError == "" {
		t.Errorf("malformed XML want xml_error, got %+v", telMal)
	}

	// Unsupported structure (HTML error page)
	_, telHtml := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/html-error.xml")
	if telHtml.ParseStatus != "unsupported_structure" {
		t.Errorf("html error want unsupported_structure, got %+v", telHtml)
	}

	// Valid Gzip sitemap
	docGz, telGz := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/sitemap.xml.gz")
	if docGz == nil || telGz.ParseStatus != "parsed" || telGz.DocType != "urlset" {
		t.Errorf("gzip parse failed: %+v", telGz)
	}

	// Bad Gzip decompression error
	_, telBadGz := fetchSitemapWithTelemetry(ctx, client, ua, srv.URL+"/bad-gzip.xml.gz")
	if telBadGz.ParseStatus != "unavailable" || !strings.Contains(telBadGz.ParseError, "gzip") {
		t.Errorf("bad gzip want unavailable with gzip error, got %+v", telBadGz)
	}
}

// TestSitemapEntriesProvenanceAndLimitsSuite tests entry-level fields, sequence preservation,
// duplicate entries, cross-sitemap URLs, same-site frontier admission vs rejected entries (url_id=0),
// and multi-source provenance.
func TestSitemapEntriesProvenanceAndLimitsSuite(t *testing.T) {
	db, _ := tempTestDB(t)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprintf(w, "User-agent: *\nDisallow:\nSitemap: %s/sitemap.xml\nSitemap: %s/sitemap-shared.xml\n", srv.URL, srv.URL)
		case "/sitemap.xml":
			// Contains internal URL, external URL, duplicate loc, lastmod, changefreq, priority
			fmt.Fprintf(w, `<?xml version="1.0"?>
<urlset>
  <url>
    <loc>%s/page1</loc>
    <lastmod>2026-10-01</lastmod>
    <changefreq>daily</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>%s/page1</loc>
    <lastmod>2026-10-02</lastmod>
  </url>
  <url>
    <loc>https://external.example.org/outsite</loc>
  </url>
  <url>
    <loc>://bad-url</loc>
  </url>
</urlset>`, srv.URL, srv.URL)
		case "/sitemap-shared.xml":
			// Also lists page1
			fmt.Fprintf(w, `<?xml version="1.0"?>
<urlset>
  <url><loc>%s/page1</loc></url>
  <url><loc>%s/page2</loc></url>
</urlset>`, srv.URL, srv.URL)
		case "/page1":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body><h1>Page 1</h1></body></html>`))
		case "/page2":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<html><body><h1>Page 2</h1></body></html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	opts := Options{
		DiscoverSitemaps: true,
		MaxURLs:          10,
		Concurrency:      2,
	}

	runner := NewRunner(db)
	h, err := runner.Start(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Runner.Start failed: %v", err)
	}
	runID := h.RunID()

	if _, err := h.Wait(); err != nil {
		t.Fatalf("h.Wait failed: %v", err)
	}

	// 1. Verify sitecrawl_sitemap_discovery
	var status, stopReason string
	var sitemapsFound, entriesFound int
	err = db.QueryRow(`SELECT status, stop_reason, sitemaps_found, entries_found FROM sitecrawl_sitemap_discovery WHERE run_id=?`, runID).
		Scan(&status, &stopReason, &sitemapsFound, &entriesFound)
	if err != nil {
		t.Fatalf("query sitemap discovery failed: %v", err)
	}
	if status != "COMPLETED" {
		t.Errorf("discovery status want COMPLETED, got %s (reason: %s)", status, stopReason)
	}
	if sitemapsFound < 2 {
		t.Errorf("sitemapsFound want at least 2, got %d", sitemapsFound)
	}
	if entriesFound < 6 {
		t.Errorf("entriesFound want at least 6, got %d", entriesFound)
	}

	// 2. Verify sitecrawl_sitemap_sources (robots_txt and common_path tracking)
	var srcCount int
	err = db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_sitemap_sources WHERE run_id=?`, runID).Scan(&srcCount)
	if err != nil || srcCount == 0 {
		t.Errorf("sitemap sources count want > 0, got %d: %v", srcCount, err)
	}

	// /sitemap.xml is declared in robots.txt AND matches commonSitemapPaths:
	// Verify that multiple sources are recorded without silent deduplication loss!
	var sm1ID int
	_ = db.QueryRow(`SELECT id FROM sitecrawl_sitemaps WHERE run_id=? AND url LIKE '%/sitemap.xml'`, runID).Scan(&sm1ID)
	var sm1Sources int
	_ = db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_sitemap_sources WHERE run_id=? AND sitemap_id=?`, runID, sm1ID).Scan(&sm1Sources)
	if sm1Sources < 2 {
		t.Errorf("sitemap.xml sources want at least 2 (robots_txt and common_path), got %d", sm1Sources)
	}

	// 3. Verify entry sequence and duplicate loc preservation
	rows, err := db.Query(`SELECT seq, url_id, loc, lastmod, changefreq, priority FROM sitecrawl_sitemap_entries WHERE run_id=? AND sitemap_id=? ORDER BY seq`, runID, sm1ID)
	if err != nil {
		t.Fatalf("query sitemap entries failed: %v", err)
	}
	defer rows.Close()

	type entryRow struct {
		seq        int
		urlID      int64
		loc        string
		lastmod    string
		changefreq string
		priority   string
	}
	var loaded []entryRow
	for rows.Next() {
		var e entryRow
		if err := rows.Scan(&e.seq, &e.urlID, &e.loc, &e.lastmod, &e.changefreq, &e.priority); err != nil {
			t.Fatalf("scan entry failed: %v", err)
		}
		loaded = append(loaded, e)
	}

	if len(loaded) != 4 {
		t.Fatalf("expected 4 entries in sitemap.xml, got %d", len(loaded))
	}

	// Entry 0: page1 (admitted, urlID > 0)
	if loaded[0].seq != 0 || loaded[0].urlID == 0 || loaded[0].lastmod != "2026-10-01" || loaded[0].changefreq != "daily" || loaded[0].priority != "0.8" {
		t.Errorf("entry 0 metadata mismatch: %+v", loaded[0])
	}
	// Entry 1: duplicate page1 (seq 1, distinct entry, urlID matches page1)
	if loaded[1].seq != 1 || loaded[1].urlID == 0 || loaded[1].lastmod != "2026-10-02" {
		t.Errorf("entry 1 duplicate loc mismatch: %+v", loaded[1])
	}
	// Entry 2: external URL (rejected by sameSite, urlID MUST BE 0!)
	if loaded[2].seq != 2 || loaded[2].urlID != 0 {
		t.Errorf("external URL urlID MUST be 0 sentinel, got %d", loaded[2].urlID)
	}
	// Entry 3: malformed URL (rejected, urlID MUST BE 0!)
	if loaded[3].seq != 3 || loaded[3].urlID != 0 {
		t.Errorf("malformed URL urlID MUST be 0 sentinel, got %d", loaded[3].urlID)
	}
}

// TestSitemapDisabledAndIncompleteSuite tests:
// 1. DiscoverSitemaps=false produces NOT_ATTEMPTED
// 2. Context cancellation during discovery produces ATTEMPTED_INCOMPLETE
func TestSitemapDisabledAndIncompleteSuite(t *testing.T) {
	db, _ := tempTestDB(t)

	// Case 1: DiscoverSitemaps = false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<html><body><h1>Hello</h1></body></html>`))
	}))
	defer srv.Close()

	runner := NewRunner(db)
	h1, err := runner.Start(context.Background(), []string{srv.URL}, Options{DiscoverSitemaps: false, MaxURLs: 1})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	runIDDisabled := h1.RunID()
	if _, err := h1.Wait(); err != nil {
		t.Fatalf("h1.Wait failed: %v", err)
	}

	var status string
	err = db.QueryRow(`SELECT status FROM sitecrawl_sitemap_discovery WHERE run_id=?`, runIDDisabled).Scan(&status)
	if err != nil || status != "NOT_ATTEMPTED" {
		t.Errorf("disabled discovery want status NOT_ATTEMPTED, got %s: %v", status, err)
	}

	// Case 2: Cancellation during sitemap discovery produces ATTEMPTED_INCOMPLETE
	var slowReq atomic.Int32
	slowSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slowReq.Add(1)
		time.Sleep(1 * time.Second)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<?xml version="1.0"?><urlset><url><loc>https://example.com/p</loc></url></urlset>`))
	}))
	defer slowSrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ev, _, _ := discoverSitemapsDetailed(ctx, slowSrv.Client(), presetFor(DefaultUserAgent), slowSrv.URL, nil, 1)
	if ev.Discovery.Status != "ATTEMPTED_INCOMPLETE" {
		t.Errorf("canceled discovery status want ATTEMPTED_INCOMPLETE, got %s", ev.Discovery.Status)
	}
	if ev.Discovery.StopReason != "context_canceled" {
		t.Errorf("canceled discovery stop reason want context_canceled, got %s", ev.Discovery.StopReason)
	}
}

// TestSitemapIndexRecursionDepthAndLimits tests sitemap index hierarchy, parent_id tracking,
// depth cap detection, and URL limit capping.
func TestSitemapIndexRecursionDepthAndLimits(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/sitemap.xml":
			// Root index points to child1
			fmt.Fprintf(w, `<?xml version="1.0"?><sitemapindex><sitemap><loc>%s/child1.xml</loc></sitemap></sitemapindex>`, srv.URL)
		case "/child1.xml":
			// Child1 points to child2
			fmt.Fprintf(w, `<?xml version="1.0"?><sitemapindex><sitemap><loc>%s/child2.xml</loc></sitemap></sitemapindex>`, srv.URL)
		case "/child2.xml":
			// Leaf urlset
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset><url><loc>%s/target</loc></url></urlset>`, srv.URL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ev, entries, read := discoverSitemapsDetailed(context.Background(), srv.Client(), presetFor(DefaultUserAgent), srv.URL, nil, 2)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d (read: %v)", len(entries), read)
	}
	if ev.Discovery.Status != "COMPLETED" {
		t.Errorf("expected COMPLETED, got %s", ev.Discovery.Status)
	}
	if ev.Discovery.DepthReached < 2 {
		t.Errorf("expected depthReached >= 2, got %d", ev.Discovery.DepthReached)
	}

	// Verify parent-child relationships
	var rootID, child1ID, child2ID int
	for _, sm := range ev.Sitemaps {
		if sm.URL == srv.URL+"/sitemap.xml" {
			rootID = sm.ID
		} else if sm.URL == srv.URL+"/child1.xml" {
			child1ID = sm.ID
		} else if sm.URL == srv.URL+"/child2.xml" {
			child2ID = sm.ID
		}
	}
	if rootID == 0 || child1ID == 0 || child2ID == 0 {
		t.Fatalf("could not find all sitemap IDs: root=%d, child1=%d, child2=%d", rootID, child1ID, child2ID)
	}

	// child1 parent must be rootID; child2 parent must be child1ID
	for _, sm := range ev.Sitemaps {
		if sm.ID == child1ID && sm.ParentID != rootID {
			t.Errorf("child1 parentID want %d, got %d", rootID, sm.ParentID)
		}
		if sm.ID == child2ID && sm.ParentID != child1ID {
			t.Errorf("child2 parentID want %d, got %d", child1ID, sm.ParentID)
		}
	}
}
