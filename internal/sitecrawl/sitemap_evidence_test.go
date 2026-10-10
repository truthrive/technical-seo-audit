package sitecrawl

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

// TestSitemapURLCapBoundarySuite tests:
// 1. Under cap: fewer than cap observed entries -> all admitted, urls_capped=false
// 2. Exact cap: exactly cap entries -> all admitted, urls_capped=false
// 3. Over cap: more than cap entries -> bounded entries offered to frontier, urls_capped=true,
//    unadmitted entries retain url_id = 0 even if duplicate of admitted URL
func TestSitemapURLCapBoundarySuite(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/sitemap.xml":
			// 5 entries: p1, p2, p3, p4, p1 (duplicate at end)
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset>
				<url><loc>%s/p1</loc></url>
				<url><loc>%s/p2</loc></url>
				<url><loc>%s/p3</loc></url>
				<url><loc>%s/p4</loc></url>
				<url><loc>%s/p1</loc></url>
			</urlset>`, srv.URL, srv.URL, srv.URL, srv.URL, srv.URL)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	// 1. Under cap: cap = 10, entries = 5
	evUnder, entriesUnder, _ := discoverSitemapsDetailedWithLimits(context.Background(), srv.Client(), presetFor(DefaultUserAgent), srv.URL, nil, 1, 10, 10)
	if len(entriesUnder) != 5 {
		t.Errorf("under-cap entries count want 5, got %d", len(entriesUnder))
	}
	if len(evUnder.Entries) != 5 {
		t.Errorf("under-cap evidence entries want 5, got %d", len(evUnder.Entries))
	}
	if evUnder.Discovery.URLsCapped {
		t.Errorf("under-cap urls_capped want false, got true")
	}
	if evUnder.Discovery.Status != "COMPLETED" {
		t.Errorf("under-cap status want COMPLETED, got %s", evUnder.Discovery.Status)
	}

	// 2. Exact cap: cap = 5, entries = 5
	evExact, entriesExact, _ := discoverSitemapsDetailedWithLimits(context.Background(), srv.Client(), presetFor(DefaultUserAgent), srv.URL, nil, 1, 5, 10)
	if len(entriesExact) != 5 {
		t.Errorf("exact-cap entries count want 5, got %d", len(entriesExact))
	}
	if len(evExact.Entries) != 5 {
		t.Errorf("exact-cap evidence entries want 5, got %d", len(evExact.Entries))
	}
	if evExact.Discovery.URLsCapped {
		t.Errorf("exact-cap urls_capped want false, got true")
	}
	if evExact.Discovery.Status != "COMPLETED" {
		t.Errorf("exact-cap status want COMPLETED, got %s", evExact.Discovery.Status)
	}

	// 3. Over cap: cap = 3, entries = 5
	evOver, entriesOver, _ := discoverSitemapsDetailedWithLimits(context.Background(), srv.Client(), presetFor(DefaultUserAgent), srv.URL, nil, 1, 3, 10)
	if len(entriesOver) != 3 {
		t.Errorf("over-cap entries count want 3, got %d", len(entriesOver))
	}
	if len(evOver.Entries) != 5 {
		t.Errorf("over-cap evidence entries want 5, got %d", len(evOver.Entries))
	}
	if !evOver.Discovery.URLsCapped {
		t.Errorf("over-cap urls_capped want true, got false")
	}
	if evOver.Discovery.Status != "ATTEMPTED_INCOMPLETE" {
		t.Errorf("over-cap status want ATTEMPTED_INCOMPLETE, got %s", evOver.Discovery.Status)
	}
	if evOver.Discovery.StopReason != "urls_capped" {
		t.Errorf("over-cap stop_reason want urls_capped, got %s", evOver.Discovery.StopReason)
	}

	// Simulate admission loop from coordinator.prepare on evOver
	opts := Options{CrawlSubdomains: false, MaxURLs: 100}
	u, _ := url.Parse(srv.URL)
	front := newFrontier(opts, u.Hostname())
	for _, e := range entriesOver {
		parsed, err := url.Parse(e.Loc)
		if err != nil || !sameSite(u.Hostname(), parsed.Hostname(), opts.crawlSubdomains()) {
			continue
		}
		id, _ := front.admit(e.Loc, 0, SourceSitemap, 0)
		if e.evidenceIndex >= 0 && e.evidenceIndex < len(evOver.Entries) {
			evOver.Entries[e.evidenceIndex].URLID = id
		}
	}

	// Entries 0, 1, 2 must have URLID > 0
	for i := 0; i < 3; i++ {
		if evOver.Entries[i].URLID == 0 {
			t.Errorf("entry %d within cap want urlID > 0, got 0", i)
		}
	}
	// Entries 3, 4 (beyond cap) must have URLID == 0
	// Entry 4 has loc = p1 (same as entry 0), must NOT borrow entry 0's URLID!
	if evOver.Entries[3].URLID != 0 {
		t.Errorf("entry 3 beyond cap want urlID == 0, got %d", evOver.Entries[3].URLID)
	}
	if evOver.Entries[4].URLID != 0 {
		t.Errorf("entry 4 beyond cap (duplicate p1) want urlID == 0, got %d", evOver.Entries[4].URLID)
	}
	// Frontier queue must contain exactly 3 items
	if front.discovered() != 3 {
		t.Errorf("frontier discovered count want 3, got %d", front.discovered())
	}
}

// TestSitemapDuplicateEntryIdentitySuite tests:
// 1. Duplicate URLs within one sitemap retain distinct entry sequence and both receive verified url_id.
// 2. Duplicate URLs across two sitemaps retain distinct sitemap document identity.
// 3. Repeated sitemap discovery sources are preserved.
func TestSitemapDuplicateEntryIdentitySuite(t *testing.T) {
	db, _ := tempTestDB(t)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprintf(w, "User-agent: *\nSitemap: %s/sitemap.xml\n", srv.URL)
		case "/sitemap.xml":
			// Two duplicates of /page1 in the same document
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset>
				<url><loc>%s/page1</loc><lastmod>2026-10-01</lastmod></url>
				<url><loc>%s/page1</loc><lastmod>2026-10-02</lastmod></url>
			</urlset>`, srv.URL, srv.URL)
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body>Page</body></html>`))
		}
	}))
	defer srv.Close()

	runner := NewRunner(db)
	h, err := runner.Start(context.Background(), []string{srv.URL}, Options{DiscoverSitemaps: true, MaxURLs: 10})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if _, err := h.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// 1. Verify two distinct entries preserved in SQLite
	rows, err := db.Query(`SELECT seq, url_id, loc, lastmod FROM sitecrawl_sitemap_entries WHERE run_id=? ORDER BY seq`, h.RunID())
	if err != nil {
		t.Fatalf("query entries failed: %v", err)
	}
	defer rows.Close()

	type ent struct {
		seq     int
		urlID   int64
		loc     string
		lastmod string
	}
	var entries []ent
	for rows.Next() {
		var e ent
		if err := rows.Scan(&e.seq, &e.urlID, &e.loc, &e.lastmod); err != nil {
			t.Fatalf("scan entry failed: %v", err)
		}
		entries = append(entries, e)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 distinct entries in SQLite, got %d", len(entries))
	}
	if entries[0].seq != 0 || entries[1].seq != 1 {
		t.Errorf("entry sequence mismatch: seq0=%d, seq1=%d", entries[0].seq, entries[1].seq)
	}
	if entries[0].urlID == 0 || entries[1].urlID == 0 {
		t.Errorf("expected both entries to have valid url_id, got %d and %d", entries[0].urlID, entries[1].urlID)
	}
	if entries[0].urlID != entries[1].urlID {
		t.Errorf("both duplicate entries should map to the same dictionary url_id, got %d != %d", entries[0].urlID, entries[1].urlID)
	}
	if entries[0].lastmod != "2026-10-01" || entries[1].lastmod != "2026-10-02" {
		t.Errorf("lastmod mismatch: %s vs %s", entries[0].lastmod, entries[1].lastmod)
	}

	// 2. Verify frontier deduplication: /page1 was only crawled once!
	var page1Crawls int
	err = db.QueryRow(`SELECT count(*) FROM sitecrawl_pages p JOIN sitecrawl_urls u ON p.url_id=u.id WHERE p.run_id=? AND u.url LIKE '%/page1'`, h.RunID()).Scan(&page1Crawls)
	if err != nil || page1Crawls != 1 {
		t.Errorf("page1 should be crawled exactly once, got %d crawls", page1Crawls)
	}

	// 3. Verify repeated discovery sources: sitemap.xml discovered via robots.txt AND common_path
	var srcCount int
	err = db.QueryRow(`SELECT count(*) FROM sitecrawl_sitemap_sources WHERE run_id=?`, h.RunID()).Scan(&srcCount)
	if err != nil || srcCount < 2 {
		t.Errorf("expected at least 2 discovery sources for dual-discovered sitemap, got %d: %v", srcCount, err)
	}
}

// TestSitemapRejectedEntriesSuite tests that external and malformed URLs
// do not receive frontier admission or fabricated URL IDs.
func TestSitemapRejectedEntriesSuite(t *testing.T) {
	db, _ := tempTestDB(t)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/sitemap.xml":
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset>
				<url><loc>%s/valid1</loc></url>
				<url><loc>https://external.example.com/external1</loc></url>
				<url><loc>http://[invalid:host/bad</loc></url>
				<url><loc>%s/valid2</loc></url>
			</urlset>`, srv.URL, srv.URL)
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body>OK</body></html>`))
		}
	}))
	defer srv.Close()

	runner := NewRunner(db)
	h, err := runner.Start(context.Background(), []string{srv.URL}, Options{DiscoverSitemaps: true, MaxURLs: 10})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if _, err := h.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	rows, err := db.Query(`SELECT seq, url_id, loc FROM sitecrawl_sitemap_entries WHERE run_id=? ORDER BY seq`, h.RunID())
	if err != nil {
		t.Fatalf("query entries failed: %v", err)
	}
	defer rows.Close()

	type ent struct {
		seq   int
		urlID int64
		loc   string
	}
	var entries []ent
	for rows.Next() {
		var e ent
		if err := rows.Scan(&e.seq, &e.urlID, &e.loc); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		entries = append(entries, e)
	}

	if len(entries) != 4 {
		t.Fatalf("expected 4 entries in SQLite, got %d", len(entries))
	}
	// Entry 0 (/valid1): admitted -> urlID > 0
	if entries[0].urlID == 0 {
		t.Errorf("valid1 should have urlID > 0, got 0")
	}
	// Entry 1 (external): rejected -> urlID == 0
	if entries[1].urlID != 0 {
		t.Errorf("external entry should have urlID == 0, got %d", entries[1].urlID)
	}
	// Entry 2 (malformed): rejected -> urlID == 0
	if entries[2].urlID != 0 {
		t.Errorf("malformed entry should have urlID == 0, got %d", entries[2].urlID)
	}
	// Entry 3 (/valid2): admitted -> urlID > 0
	if entries[3].urlID == 0 {
		t.Errorf("valid2 should have urlID > 0, got 0")
	}

	// Verify external URL not in sitecrawl_urls
	var externalInDict int
	err = db.QueryRow(`SELECT count(*) FROM sitecrawl_urls WHERE run_id=? AND url LIKE '%external.example.com%'`, h.RunID()).Scan(&externalInDict)
	if err != nil || externalInDict != 0 {
		t.Errorf("external URL must not exist in sitecrawl_urls, got %d", externalInDict)
	}
}

// TestSitemapFrontierParitySuite tests that frontier admission order,
// deduplication, and source attribution match the baseline contract.
func TestSitemapFrontierParitySuite(t *testing.T) {
	db, _ := tempTestDB(t)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch r.URL.Path {
		case "/robots.txt":
			fmt.Fprintf(w, "User-agent: *\nSitemap: %s/sitemap.xml\n", srv.URL)
		case "/sitemap.xml":
			fmt.Fprintf(w, `<?xml version="1.0"?><urlset>
				<url><loc>%s/alpha</loc></url>
				<url><loc>%s/beta</loc></url>
				<url><loc>%s/alpha</loc></url>
				<url><loc>https://other.com/gamma</loc></url>
				<url><loc>%s/delta</loc></url>
			</urlset>`, srv.URL, srv.URL, srv.URL, srv.URL)
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body>OK</body></html>`))
		}
	}))
	defer srv.Close()

	runner := NewRunner(db)
	h, err := runner.Start(context.Background(), []string{srv.URL}, Options{DiscoverSitemaps: true, MaxURLs: 20})
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if _, err := h.Wait(); err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	// 1. Verify frontier admission semantics on candidate URLs:
	// - candidate order preserved
	// - attribution is SourceSitemap
	// - duplicate /alpha is deduped
	// - external /gamma is rejected
	u, _ := url.Parse(srv.URL)
	fOpts := Options{CrawlSubdomains: false, MaxURLs: 20}
	front := newFrontier(fOpts, u.Hostname())
	front.admit(srv.URL, 0, SourceSeed, 0)

	_, entries, _ := discoverSitemapsDetailed(context.Background(), srv.Client(), presetFor(DefaultUserAgent), srv.URL, nil, 1)
	for _, e := range entries {
		parsed, err := url.Parse(e.Loc)
		if err != nil || !sameSite(u.Hostname(), parsed.Hostname(), fOpts.crawlSubdomains()) {
			continue
		}
		front.admit(e.Loc, 0, SourceSitemap, 0)
	}

	snap := front.snapshot()
	if len(snap) != 4 {
		t.Fatalf("expected 4 frontier items in queue, got %d", len(snap))
	}
	if snap[0].URL != srv.URL || snap[0].Source != SourceSeed {
		t.Errorf("item 0 want seed (%s), got %+v", SourceSeed, snap[0])
	}
	if snap[1].URL != srv.URL+"/alpha" || snap[1].Source != SourceSitemap {
		t.Errorf("item 1 want alpha (%s), got %+v", SourceSitemap, snap[1])
	}
	if snap[2].URL != srv.URL+"/beta" || snap[2].Source != SourceSitemap {
		t.Errorf("item 2 want beta (%s), got %+v", SourceSitemap, snap[2])
	}
	if snap[3].URL != srv.URL+"/delta" || snap[3].Source != SourceSitemap {
		t.Errorf("item 3 want delta (%s), got %+v", SourceSitemap, snap[3])
	}

	// 2. Verify complete crawl output in SQLite:
	// - seed, alpha, beta, delta crawled (4 pages)
	// - external gamma was not crawled or admitted
	var pageCount int
	err = db.QueryRow(`SELECT count(*) FROM sitecrawl_pages WHERE run_id=?`, h.RunID()).Scan(&pageCount)
	if err != nil || pageCount != 4 {
		t.Errorf("crawled pages count want 4, got %d: %v", pageCount, err)
	}

	var gammaInUrls int
	err = db.QueryRow(`SELECT count(*) FROM sitecrawl_urls WHERE run_id=? AND url LIKE '%other.com%'`, h.RunID()).Scan(&gammaInUrls)
	if err != nil || gammaInUrls != 0 {
		t.Errorf("rejected external gamma must not exist in sitecrawl_urls, got %d", gammaInUrls)
	}

	// 3. Verify sitemap entries in SQLite:
	// 5 entries total, alpha duplicate preserved with matching urlID, gamma has urlID=0
	rows, err := db.Query(`SELECT seq, url_id, loc FROM sitecrawl_sitemap_entries WHERE run_id=? ORDER BY seq`, h.RunID())
	if err != nil {
		t.Fatalf("query sitemap entries failed: %v", err)
	}
	defer rows.Close()

	type smRow struct {
		seq   int
		urlID int64
		loc   string
	}
	var smEntries []smRow
	for rows.Next() {
		var r smRow
		if err := rows.Scan(&r.seq, &r.urlID, &r.loc); err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		smEntries = append(smEntries, r)
	}
	if len(smEntries) != 5 {
		t.Fatalf("expected 5 sitemap entry rows, got %d", len(smEntries))
	}
	// alpha (seq 0) and alpha dup (seq 2) must have same non-zero urlID
	if smEntries[0].urlID == 0 || smEntries[2].urlID == 0 || smEntries[0].urlID != smEntries[2].urlID {
		t.Errorf("alpha entries urlID mismatch: seq0=%d, seq2=%d", smEntries[0].urlID, smEntries[2].urlID)
	}
	// gamma (seq 3) must have urlID == 0
	if smEntries[3].urlID != 0 {
		t.Errorf("external gamma want urlID=0, got %d", smEntries[3].urlID)
	}
}
