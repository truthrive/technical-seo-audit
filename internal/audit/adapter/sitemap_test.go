package adapter_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// Helper to query observations for any subject ref.
func getObservations(snap *audit.EvidenceSnapshot, subjectRef string) map[string][]string {
	fields := make(map[string][]string)
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == subjectRef {
			fields[obs.Field] = append(fields[obs.Field], obs.Value)
		}
	}
	return fields
}

// Helper to check if an EvidenceGap code exists in gaps.
func hasGap(gaps []adapter.EvidenceGap, code string) bool {
	for _, g := range gaps {
		if g.GapCode == code {
			return true
		}
	}
	return false
}

// Helper to find an EvidenceGap by code and optional subject ref.
func findGap(gaps []adapter.EvidenceGap, code string, subjectRef string) *adapter.EvidenceGap {
	for _, g := range gaps {
		if g.GapCode == code && (subjectRef == "" || g.SubjectRef == subjectRef) {
			return &g
		}
	}
	return nil
}

// Helper to insert a minimal crawl run record for sitemap tests.
func setupSitemapTestRun(t *testing.T, db *sql.DB, runID, seed string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`INSERT INTO sitecrawl_runs(
		id, state, stop_reason, seed_url, host, options, started_at, finished_at, found, crawled, total
	) VALUES (?, 'completed', '', ?, 'example.com', '{}', ?, ?, 1, 1, 1)`,
		runID, seed, now, now)
	if err != nil {
		t.Fatalf("setupSitemapTestRun failed: %v", err)
	}
}

// ============================================================================
// Group A: Document Evidence Tests (Section 15.A)
// ============================================================================

func TestAdapter_Sitemap_DocumentEvidence(t *testing.T) {
	t.Run("Direct sitemap HTTP 200", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:doc:200"
		auditRunID := audit.AuditRunID("audit:sm:doc:200")
		snapshotID := audit.SnapshotID("snap:sm:doc:200")
		seed := "https://example.com/"
		setupSitemapTestRun(t, db, runID, seed)

		// Insert sitemap discovery record
		_, err := db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at, finished_at
		) VALUES (?, 'COMPLETED', 1, 2, 0, 0, 0, 0, '2026-10-01T10:00:00Z', '2026-10-01T10:00:05Z')`, runID)
		if err != nil {
			t.Fatalf("insert discovery: %v", err)
		}

		// Insert sitemap document (direct 200, parsed, 2 entries)
		smURL := "https://example.com/sitemap.xml"
		_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status, final_status, status,
			fetch_error, redirect_to, redirect_hops, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		) VALUES (?, 1, ?, 'common_path', 0, 200, 200, 200, '', '', 0, 1, 'urlset', 'parsed', '', 2, '2026-10-01T10:00:01Z')`,
			runID, smURL)
		if err != nil {
			t.Fatalf("insert sitemap: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.SitemapObservations) != 1 {
			t.Fatalf("expected 1 SitemapObservation, got %d", len(res.SitemapObservations))
		}
		so := res.SitemapObservations[0]
		expectedDocRef := fmt.Sprintf("sm:%s:1", auditRunID)
		if string(so.SitemapID) != expectedDocRef {
			t.Errorf("expected SitemapID %s, got %s", expectedDocRef, so.SitemapID)
		}
		if so.DocumentType != audit.SitemapDocumentURLSet {
			t.Errorf("expected DocumentType URLSET, got %s", so.DocumentType)
		}
		if so.ParseStatus != "parsed" {
			t.Errorf("expected ParseStatus 'parsed', got %s", so.ParseStatus)
		}

		// Verify emitted observations on SubjectSitemap
		obs := getObservations(res.EvidenceSnapshot, expectedDocRef)
		if v := obs["sitemap_url"]; len(v) != 1 || v[0] != smURL {
			t.Errorf("expected sitemap_url %q, got %v", smURL, v)
		}
		if v := obs["initial_http_status"]; len(v) != 1 || v[0] != "200" {
			t.Errorf("expected initial_http_status '200', got %v", v)
		}
		if v := obs["final_http_status"]; len(v) != 1 || v[0] != "200" {
			t.Errorf("expected final_http_status '200', got %v", v)
		}
		if v := obs["sitemap_fetch_status"]; len(v) != 0 {
			t.Errorf("expected no sitemap_fetch_status observation, got %v", v)
		}
		for _, o := range res.EvidenceSnapshot.NormalizedObservations {
			if o.Field == "sitemap_fetch_status" {
				t.Fatalf("forbidden normalized observation sitemap_fetch_status found: %+v", o)
			}
		}
		if v := obs["sitemap_document_type"]; len(v) != 1 || v[0] != "URLSET" {
			t.Errorf("expected sitemap_document_type 'URLSET', got %v", v)
		}
		if v := obs["sitemap_entry_count"]; len(v) != 1 || v[0] != "2" {
			t.Errorf("expected sitemap_entry_count '2', got %v", v)
		}

		// Ensure no unconditional GapSitemapDocumentUnavailable is emitted
		if hasGap(res.EvidenceGaps, adapter.GapSitemapDocumentUnavailable) {
			t.Errorf("unexpected GapSitemapDocumentUnavailable emitted when valid sitemap was normalized")
		}
	})

	t.Run("Initial 301 to final 200 preserves independent statuses", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:doc:redirect:200"
		auditRunID := audit.AuditRunID("audit:sm:doc:redirect:200")
		snapshotID := audit.SnapshotID("snap:sm:doc:redirect:200")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, err := db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at, finished_at
		) VALUES (?, 'COMPLETED', 1, 5, 0, 0, 0, 0, '2026-10-01T10:00:00Z', '2026-10-01T10:00:05Z')`, runID)
		if err != nil {
			t.Fatalf("insert discovery: %v", err)
		}

		smURL := "https://example.com/sitemap.xml"
		finalURL := "https://example.com/sitemap_index.xml"
		_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status, final_status, status,
			fetch_error, redirect_to, redirect_hops, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		) VALUES (?, 1, ?, 'robots_txt', 0, 301, 200, 200, '', ?, 1, 1, 'sitemapindex', 'parsed', '', 5, '2026-10-01T10:00:01Z')`,
			runID, smURL, finalURL)
		if err != nil {
			t.Fatalf("insert sitemap: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedDocRef := fmt.Sprintf("sm:%s:1", auditRunID)
		obs := getObservations(res.EvidenceSnapshot, expectedDocRef)

		// Verify independent initial status 301 and final status 200
		if v := obs["initial_http_status"]; len(v) != 1 || v[0] != "301" {
			t.Errorf("expected initial_http_status '301', got %v", v)
		}
		if v := obs["final_http_status"]; len(v) != 1 || v[0] != "200" {
			t.Errorf("expected final_http_status '200', got %v", v)
		}
		if v := obs["sitemap_final_url"]; len(v) != 1 || v[0] != finalURL {
			t.Errorf("expected sitemap_final_url %q, got %v", finalURL, v)
		}
		if v := obs["sitemap_redirect_hops"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("expected sitemap_redirect_hops '1', got %v", v)
		}
		if v := obs["sitemap_document_type"]; len(v) != 1 || v[0] != "SITEMAP_INDEX" {
			t.Errorf("expected sitemap_document_type 'SITEMAP_INDEX', got %v", v)
		}
	})

	t.Run("Initial 301 to final 404 preserves redirect failure", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:doc:redirect:404"
		auditRunID := audit.AuditRunID("audit:sm:doc:redirect:404")
		snapshotID := audit.SnapshotID("snap:sm:doc:redirect:404")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, err := db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at, finished_at
		) VALUES (?, 'COMPLETED', 1, 0, 0, 0, 0, 0, '2026-10-01T10:00:00Z', '2026-10-01T10:00:05Z')`, runID)
		if err != nil {
			t.Fatalf("insert discovery: %v", err)
		}

		smURL := "https://example.com/sitemap.xml"
		deadURL := "https://example.com/missing.xml"
		_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status, final_status, status,
			fetch_error, redirect_to, redirect_hops, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		) VALUES (?, 1, ?, 'common_path', 0, 301, 404, 404, '', ?, 1, 1, 'unknown', 'not_attempted', '', 0, '2026-10-01T10:00:01Z')`,
			runID, smURL, deadURL)
		if err != nil {
			t.Fatalf("insert sitemap: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedDocRef := fmt.Sprintf("sm:%s:1", auditRunID)
		obs := getObservations(res.EvidenceSnapshot, expectedDocRef)
		if v := obs["initial_http_status"]; len(v) != 1 || v[0] != "301" {
			t.Errorf("expected initial_http_status '301', got %v", v)
		}
		if v := obs["final_http_status"]; len(v) != 1 || v[0] != "404" {
			t.Errorf("expected final_http_status '404', got %v", v)
		}
		if v := obs["sitemap_final_url"]; len(v) != 1 || v[0] != deadURL {
			t.Errorf("expected sitemap_final_url %q, got %v", deadURL, v)
		}
	})

	t.Run("Fetch failure before response emits GapSitemapFetchFailed", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:doc:fetchfail"
		auditRunID := audit.AuditRunID("audit:sm:doc:fetchfail")
		snapshotID := audit.SnapshotID("snap:sm:doc:fetchfail")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, err := db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at, finished_at
		) VALUES (?, 'COMPLETED', 1, 0, 0, 0, 0, 0, '2026-10-01T10:00:00Z', '2026-10-01T10:00:05Z')`, runID)
		if err != nil {
			t.Fatalf("insert discovery: %v", err)
		}

		smURL := "https://example.com/sitemap.xml"
		_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status, final_status, status,
			fetch_error, redirect_to, redirect_hops, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		) VALUES (?, 1, ?, 'common_path', 0, 0, 0, 0, 'dial tcp: connection refused', '', 0, 0, 'unknown', 'not_attempted', '', 0, '2026-10-01T10:00:01Z')`,
			runID, smURL)
		if err != nil {
			t.Fatalf("insert sitemap: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedDocRef := fmt.Sprintf("sm:%s:1", auditRunID)
		obs := getObservations(res.EvidenceSnapshot, expectedDocRef)
		if v := obs["sitemap_fetch_error"]; len(v) != 1 || v[0] != "dial tcp: connection refused" {
			t.Errorf("expected fetch error observation, got %v", v)
		}

		gap := findGap(res.EvidenceGaps, adapter.GapSitemapFetchFailed, expectedDocRef)
		if gap == nil {
			t.Errorf("expected GapSitemapFetchFailed gap for connection refused sitemap")
		}
	})

	t.Run("Parse error emits GapSitemapParseFailed", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:doc:parsefail"
		auditRunID := audit.AuditRunID("audit:sm:doc:parsefail")
		snapshotID := audit.SnapshotID("snap:sm:doc:parsefail")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, err := db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at, finished_at
		) VALUES (?, 'COMPLETED', 1, 0, 0, 0, 0, 0, '2026-10-01T10:00:00Z', '2026-10-01T10:00:05Z')`, runID)
		if err != nil {
			t.Fatalf("insert discovery: %v", err)
		}

		smURL := "https://example.com/sitemap.xml"
		parseErr := "XML syntax error on line 4: element <loc> not closed"
		_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status, final_status, status,
			fetch_error, redirect_to, redirect_hops, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		) VALUES (?, 1, ?, 'common_path', 0, 200, 200, 200, '', '', 0, 1, 'urlset', 'xml_error', ?, 0, '2026-10-01T10:00:01Z')`,
			runID, smURL, parseErr)
		if err != nil {
			t.Fatalf("insert sitemap: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedDocRef := fmt.Sprintf("sm:%s:1", auditRunID)
		obs := getObservations(res.EvidenceSnapshot, expectedDocRef)
		if v := obs["sitemap_parse_status"]; len(v) != 1 || v[0] != "xml_error" {
			t.Errorf("expected parse_status 'xml_error', got %v", v)
		}
		if v := obs["sitemap_parse_error"]; len(v) != 1 || v[0] != parseErr {
			t.Errorf("expected parse_error %q, got %v", parseErr, v)
		}

		gap := findGap(res.EvidenceGaps, adapter.GapSitemapParseFailed, expectedDocRef)
		if gap == nil {
			t.Errorf("expected GapSitemapParseFailed gap")
		}
	})

	t.Run("Unsupported document structure", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:doc:unsupported"
		auditRunID := audit.AuditRunID("audit:sm:doc:unsupported")
		snapshotID := audit.SnapshotID("snap:sm:doc:unsupported")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, err := db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at, finished_at
		) VALUES (?, 'COMPLETED', 1, 0, 0, 0, 0, 0, '2026-10-01T10:00:00Z', '2026-10-01T10:00:05Z')`, runID)
		if err != nil {
			t.Fatalf("insert discovery: %v", err)
		}

		smURL := "https://example.com/sitemap.xml"
		_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status, final_status, status,
			fetch_error, redirect_to, redirect_hops, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		) VALUES (?, 1, ?, 'common_path', 0, 200, 200, 200, '', '', 0, 1, 'unknown', 'unsupported_structure', 'root tag <rss> not sitemap', 0, '2026-10-01T10:00:01Z')`,
			runID, smURL)
		if err != nil {
			t.Fatalf("insert sitemap: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedDocRef := fmt.Sprintf("sm:%s:1", auditRunID)
		obs := getObservations(res.EvidenceSnapshot, expectedDocRef)
		if v := obs["sitemap_document_type"]; len(v) != 1 || v[0] != "UNKNOWN" {
			t.Errorf("expected sitemap_document_type UNKNOWN, got %v", v)
		}
		if v := obs["sitemap_parse_status"]; len(v) != 1 || v[0] != "unsupported_structure" {
			t.Errorf("expected parse_status 'unsupported_structure', got %v", v)
		}
	})

	t.Run("Missing timestamp falls back safely without panic", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:doc:notime"
		auditRunID := audit.AuditRunID("audit:sm:doc:notime")
		snapshotID := audit.SnapshotID("snap:sm:doc:notime")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, err := db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at, finished_at
		) VALUES (?, 'COMPLETED', 1, 0, 0, 0, 0, 0, '', '')`, runID)
		if err != nil {
			t.Fatalf("insert discovery: %v", err)
		}

		_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status, final_status, status,
			fetch_error, redirect_to, redirect_hops, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		) VALUES (?, 1, 'https://example.com/sitemap.xml', 'common_path', 0, 200, 200, 200, '', '', 0, 1, 'urlset', 'parsed', '', 0, '')`,
			runID)
		if err != nil {
			t.Fatalf("insert sitemap: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}
		if len(res.SitemapObservations) != 1 {
			t.Fatalf("expected 1 SitemapObservation, got %d", len(res.SitemapObservations))
		}
		if res.SitemapObservations[0].ObservedAt.IsZero() {
			t.Errorf("expected non-zero fallback observed_at")
		}
	})
}

// ============================================================================
// Group B: Entry Evidence Tests (Section 15.B)
// ============================================================================

func TestAdapter_Sitemap_EntryEvidence(t *testing.T) {
	t.Run("Single and multiple entries normalized with metadata", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:ent:meta"
		auditRunID := audit.AuditRunID("audit:sm:ent:meta")
		snapshotID := audit.SnapshotID("snap:sm:ent:meta")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		// Insert 2 URLs into dictionary and crawl results
		insertURL(t, db, runID, 1, "https://example.com/")
		insertURL(t, db, runID, 2, "https://example.com/products")
		insertPage(t, db, runID, 1, "https://example.com/", 200)
		insertPage(t, db, runID, 2, "https://example.com/products", 200)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at, finished_at
		) VALUES (?, 'COMPLETED', 1, 2, 0, 0, 0, 0, '2026-10-01T10:00:00Z', '2026-10-01T10:00:05Z')`, runID)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status, final_status, status,
			fetch_error, redirect_to, redirect_hops, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		) VALUES (?, 1, 'https://example.com/sitemap.xml', 'common_path', 0, 200, 200, 200, '', '', 0, 1, 'urlset', 'parsed', '', 2, '2026-10-01T10:00:01Z')`, runID)

		// Entry 0: https://example.com/ (url_id=1)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(
			run_id, sitemap_id, seq, url_id, loc, lastmod, changefreq, priority
		) VALUES (?, 1, 0, 1, 'https://example.com/', '2026-09-15T08:30:00Z', 'daily', '1.0')`, runID)

		// Entry 1: https://example.com/products (url_id=2)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(
			run_id, sitemap_id, seq, url_id, loc, lastmod, changefreq, priority
		) VALUES (?, 1, 1, 2, 'https://example.com/products', '2026-09-10', 'weekly', '0.8')`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.SitemapEntries) != 2 {
			t.Fatalf("expected 2 SitemapEntries, got %d", len(res.SitemapEntries))
		}

		e0 := res.SitemapEntries[0]
		expectedE0ID := fmt.Sprintf("sme:%s:1:0", auditRunID)
		if string(e0.SitemapEntryID) != expectedE0ID {
			t.Errorf("expected entry ID %s, got %s", expectedE0ID, e0.SitemapEntryID)
		}
		if e0.ListedURLRaw != "https://example.com/" {
			t.Errorf("expected listed URL https://example.com/, got %s", e0.ListedURLRaw)
		}
		if e0.LastmodNormalized == nil {
			t.Errorf("expected parsed lastmod normalized, got nil")
		}

		// Observations on entry 0
		obs0 := getObservations(res.EvidenceSnapshot, expectedE0ID)
		if v := obs0["loc_raw"]; len(v) != 1 || v[0] != "https://example.com/" {
			t.Errorf("expected loc_raw, got %v", v)
		}
		if v := obs0["loc_normalized"]; len(v) != 1 || v[0] != "https://example.com/" {
			t.Errorf("expected loc_normalized, got %v", v)
		}
		if v := obs0["entry_seq"]; len(v) != 1 || v[0] != "0" {
			t.Errorf("expected entry_seq '0', got %v", v)
		}
		if v := obs0["lastmod_raw"]; len(v) != 1 || v[0] != "2026-09-15T08:30:00Z" {
			t.Errorf("expected lastmod_raw, got %v", v)
		}
		if v := obs0["changefreq_raw"]; len(v) != 1 || v[0] != "daily" {
			t.Errorf("expected changefreq_raw 'daily', got %v", v)
		}
		if v := obs0["priority_raw"]; len(v) != 1 || v[0] != "1.0" {
			t.Errorf("expected priority_raw '1.0', got %v", v)
		}
		if v := obs0["url_subject_ref"]; len(v) != 1 || v[0] != fmt.Sprintf("url:%s:1", auditRunID) {
			t.Errorf("expected url_subject_ref, got %v", v)
		}

		// Bilateral check on URL subject url:<audit_run_id>:1
		urlObs := getObservations(res.EvidenceSnapshot, fmt.Sprintf("url:%s:1", auditRunID))
		if v := urlObs["listed_in_sitemap"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected listed_in_sitemap 'true', got %v", v)
		}
		if v := urlObs["sitemap_entry_ref"]; len(v) != 1 || v[0] != expectedE0ID {
			t.Errorf("expected sitemap_entry_ref on URL, got %v", v)
		}
		if v := urlObs["sitemap_document_ref"]; len(v) != 1 || v[0] != fmt.Sprintf("sm:%s:1", auditRunID) {
			t.Errorf("expected sitemap_document_ref on URL, got %v", v)
		}
	})

	t.Run("Duplicate loc within one sitemap retains separate identities", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:ent:dupe:loc"
		auditRunID := audit.AuditRunID("audit:sm:ent:dupe:loc")
		snapshotID := audit.SnapshotID("snap:sm:ent:dupe:loc")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		insertURL(t, db, runID, 1, "https://example.com/duplicate")
		insertPage(t, db, runID, 1, "https://example.com/duplicate", 200)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 2)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 2)`, runID)

		// Same loc inserted twice with seq 0 and seq 1
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, 'https://example.com/duplicate')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 1, 1, 'https://example.com/duplicate')`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.SitemapEntries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(res.SitemapEntries))
		}
		id0 := string(res.SitemapEntries[0].SitemapEntryID)
		id1 := string(res.SitemapEntries[1].SitemapEntryID)
		if id0 == id1 {
			t.Errorf("duplicate loc must retain distinct identities, both had %s", id0)
		}
		if id0 != fmt.Sprintf("sme:%s:1:0", auditRunID) || id1 != fmt.Sprintf("sme:%s:1:1", auditRunID) {
			t.Errorf("unexpected identities: %s, %s", id0, id1)
		}

		// Bilateral URL check: URL subject receives both entry refs
		urlObs := getObservations(res.EvidenceSnapshot, fmt.Sprintf("url:%s:1", auditRunID))
		if v := urlObs["sitemap_entry_ref"]; len(v) != 2 {
			t.Errorf("expected 2 sitemap_entry_ref entries on URL, got %v", v)
		}
	})

	t.Run("Same loc in multiple sitemaps creates separate entry subjects", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:ent:multi:docs"
		auditRunID := audit.AuditRunID("audit:sm:ent:multi:docs")
		snapshotID := audit.SnapshotID("snap:sm:ent:multi:docs")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		insertURL(t, db, runID, 1, "https://example.com/common")
		insertPage(t, db, runID, 1, "https://example.com/common", 200)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 2, 2)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap_a.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 2, 'https://example.com/sitemap_b.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, 'https://example.com/common')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 2, 0, 1, 'https://example.com/common')`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.SitemapEntries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(res.SitemapEntries))
		}
		expectedA := fmt.Sprintf("sme:%s:1:0", auditRunID)
		expectedB := fmt.Sprintf("sme:%s:2:0", auditRunID)
		if string(res.SitemapEntries[0].SitemapEntryID) != expectedA || string(res.SitemapEntries[1].SitemapEntryID) != expectedB {
			t.Errorf("unexpected entry IDs: %s, %s", res.SitemapEntries[0].SitemapEntryID, res.SitemapEntries[1].SitemapEntryID)
		}

		// Observations check parent references
		obsA := getObservations(res.EvidenceSnapshot, expectedA)
		if v := obsA["parent_sitemap_ref"]; len(v) != 1 || v[0] != fmt.Sprintf("sm:%s:1", auditRunID) {
			t.Errorf("expected parent sm:1, got %v", v)
		}
		obsB := getObservations(res.EvidenceSnapshot, expectedB)
		if v := obsB["parent_sitemap_ref"]; len(v) != 1 || v[0] != fmt.Sprintf("sm:%s:2", auditRunID) {
			t.Errorf("expected parent sm:2, got %v", v)
		}
	})

	t.Run("External listed URL with url_id=0 does not create URL subject", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:ent:external"
		auditRunID := audit.AuditRunID("audit:sm:ent:external")
		snapshotID := audit.SnapshotID("snap:sm:ent:external")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)

		extURL := "https://otherdomain.org/about"
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 0, ?)`, runID, extURL)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.SitemapEntries) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(res.SitemapEntries))
		}
		if res.SitemapEntries[0].ListedURLID != "" {
			t.Errorf("expected empty ListedURLID for external URL with url_id=0, got %s", res.SitemapEntries[0].ListedURLID)
		}

		entObs := getObservations(res.EvidenceSnapshot, fmt.Sprintf("sme:%s:1:0", auditRunID))
		if v := entObs["url_subject_ref"]; len(v) != 0 {
			t.Errorf("expected no url_subject_ref for external url_id=0, got %v", v)
		}
		if v := entObs["loc_raw"]; len(v) != 1 || v[0] != extURL {
			t.Errorf("expected loc_raw %q, got %v", extURL, v)
		}
	})

	t.Run("Malformed listed URL retains loc_raw without panic", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:ent:malformed"
		auditRunID := audit.AuditRunID("audit:sm:ent:malformed")
		snapshotID := audit.SnapshotID("snap:sm:ent:malformed")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)

		badURL := "http://[invalid-ipv6"
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 0, ?)`, runID, badURL)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		entObs := getObservations(res.EvidenceSnapshot, fmt.Sprintf("sme:%s:1:0", auditRunID))
		if v := entObs["loc_raw"]; len(v) != 1 || v[0] != badURL {
			t.Errorf("expected loc_raw to preserve %q, got %v", badURL, v)
		}
		if v := entObs["loc_normalized"]; len(v) != 0 {
			t.Errorf("expected no loc_normalized for malformed url, got %v", v)
		}
	})

	t.Run("Orphan sitemap entry references missing sitemap document", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:ent:orphan"
		auditRunID := audit.AuditRunID("audit:sm:ent:orphan")
		snapshotID := audit.SnapshotID("snap:sm:ent:orphan")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		orphanURL := "https://example.com/orphan"
		insertURL(t, db, runID, 1, orphanURL)
		insertPage(t, db, runID, 1, orphanURL, 200)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 0, 1)`, runID)
		// No sitemap doc with ID 99 inserted! Entry has valid url_id = 1 pointing to valid dictionary URL
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 99, 0, 1, ?)`, runID, orphanURL)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedEID := fmt.Sprintf("sme:%s:99:0", auditRunID)
		gap := findGap(res.EvidenceGaps, adapter.GapOrphanSitemapEntry, expectedEID)
		if gap == nil {
			t.Errorf("expected GapOrphanSitemapEntry gap for entry with missing parent document")
		}

		// Entry subject exists and preserves raw loc
		if len(res.SitemapEntries) != 1 {
			t.Fatalf("expected 1 sitemap entry, got %d", len(res.SitemapEntries))
		}
		se := res.SitemapEntries[0]
		if string(se.SitemapEntryID) != expectedEID {
			t.Errorf("expected SitemapEntryID %s, got %s", expectedEID, se.SitemapEntryID)
		}
		if se.ListedURLRaw != orphanURL {
			t.Errorf("expected raw loc %q preserved, got %q", orphanURL, se.ListedURLRaw)
		}
		// Typed ListedURLID must be empty
		if se.ListedURLID != "" {
			t.Errorf("expected empty ListedURLID for orphan entry, got %s", se.ListedURLID)
		}

		entObs := getObservations(res.EvidenceSnapshot, expectedEID)
		if v := entObs["parent_sitemap_ref"]; len(v) != 0 {
			t.Errorf("expected no parent_sitemap_ref for orphan entry, got %v", v)
		}
		if v := entObs["url_subject_ref"]; len(v) != 0 {
			t.Errorf("expected no url_subject_ref for orphan entry, got %v", v)
		}
		if v := entObs["loc_raw"]; len(v) != 1 || v[0] != orphanURL {
			t.Errorf("expected loc_raw preserved, got %v", v)
		}

		// No bilateral sitemap references appear on the URL subject
		urlSubjRef := fmt.Sprintf("url:%s:1", auditRunID)
		urlObs := getObservations(res.EvidenceSnapshot, urlSubjRef)
		if v := urlObs["listed_in_sitemap"]; len(v) != 0 {
			t.Errorf("expected no listed_in_sitemap on URL subject for orphan entry, got %v", v)
		}
		if v := urlObs["sitemap_entry_ref"]; len(v) != 0 {
			t.Errorf("expected no sitemap_entry_ref on URL subject for orphan entry, got %v", v)
		}
		if v := urlObs["sitemap_document_ref"]; len(v) != 0 {
			t.Errorf("expected no sitemap_document_ref on URL subject for orphan entry, got %v", v)
		}
		if v := urlObs["sitemap_url"]; len(v) != 0 {
			t.Errorf("expected no sitemap_url on URL subject for orphan entry, got %v", v)
		}
	})

	t.Run("Entry referencing nonexistent URL dictionary ID emits gap", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:ent:badurlid"
		auditRunID := audit.AuditRunID("audit:sm:ent:badurlid")
		snapshotID := audit.SnapshotID("snap:sm:ent:badurlid")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)

		// url_id = 9999 does not exist in sitecrawl_urls!
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 9999, 'https://example.com/page')`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedEID := fmt.Sprintf("sme:%s:1:0", auditRunID)
		gap := findGap(res.EvidenceGaps, adapter.GapInvalidSitemapURLCorrelation, expectedEID)
		if gap == nil {
			t.Errorf("expected GapInvalidSitemapURLCorrelation gap for nonexistent URL dictionary id")
		}

		entObs := getObservations(res.EvidenceSnapshot, expectedEID)
		if v := entObs["url_subject_ref"]; len(v) != 0 {
			t.Errorf("expected no url_subject_ref for invalid correlation, got %v", v)
		}
	})
}

// ============================================================================
// Group C: Provenance & Multi-Source Tests (Section 15.C)
// ============================================================================

func TestAdapter_Sitemap_Provenance(t *testing.T) {
	t.Run("Multi-source discovery provenance on single sitemap", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:prov:multi"
		auditRunID := audit.AuditRunID("audit:sm:prov:multi")
		snapshotID := audit.SnapshotID("snap:sm:prov:multi")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 'robots_txt', 200, 200, 200, 1, 'urlset', 'parsed', 0)`, runID)

		// Insert both robots_txt and common_path as sources
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'common_path', 0)`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedDocRef := fmt.Sprintf("sm:%s:1", auditRunID)
		obs := getObservations(res.EvidenceSnapshot, expectedDocRef)
		if v := obs["discovery_source"]; len(v) != 1 || v[0] != "robots_txt" {
			t.Errorf("expected primary discovery_source 'robots_txt', got %v", v)
		}
		if v := obs["discovery_sources"]; len(v) != 1 || v[0] != "common_path,robots_txt" {
			t.Errorf("expected sorted multi-source 'common_path,robots_txt', got %v", v)
		}
	})

	t.Run("Sitemap index parent relationship", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:prov:index"
		auditRunID := audit.AuditRunID("audit:sm:prov:index")
		snapshotID := audit.SnapshotID("snap:sm:prov:index")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 2, 0)`, runID)
		// Parent index (ID 1)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap_index.xml', 'robots_txt', 0, 200, 200, 200, 1, 'sitemapindex', 'parsed', 1)`, runID)
		// Child sitemap (ID 2) with parent_id 1
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 2, 'https://example.com/child.xml', 'sitemap_index', 1, 200, 200, 200, 1, 'urlset', 'parsed', 0)`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		childObs := getObservations(res.EvidenceSnapshot, fmt.Sprintf("sm:%s:2", auditRunID))
		if v := childObs["parent_sitemap_ref"]; len(v) != 1 || v[0] != fmt.Sprintf("sm:%s:1", auditRunID) {
			t.Errorf("expected parent_sitemap_ref sm:1, got %v", v)
		}
	})

	t.Run("Cross-run isolation", func(t *testing.T) {
		db := newTestDB(t)
		runA := "run:sm:prov:runA"
		runB := "run:sm:prov:runB"
		setupSitemapTestRun(t, db, runA, "https://example.com/")
		setupSitemapTestRun(t, db, runB, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found) VALUES (?, 'COMPLETED', 1)`, runA)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found) VALUES (?, 'COMPLETED', 1)`, runB)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status)
			VALUES (?, 1, 'https://example.com/sitemapA.xml', 200, 200, 200, 1, 'urlset', 'parsed')`, runA)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status)
			VALUES (?, 1, 'https://example.com/sitemapB.xml', 200, 200, 200, 1, 'urlset', 'parsed')`, runB)

		resA, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runA,
			AuditRunID: "audit:runA",
			SnapshotID: "snap:runA",
		})
		if err != nil {
			t.Fatalf("build A failed: %v", err)
		}

		if len(resA.SitemapObservations) != 1 {
			t.Fatalf("expected 1 sitemap for runA, got %d", len(resA.SitemapObservations))
		}
		obsA := getObservations(resA.EvidenceSnapshot, "sm:audit:runA:1")
		if v := obsA["sitemap_url"]; len(v) != 1 || v[0] != "https://example.com/sitemapA.xml" {
			t.Errorf("runA leaked sitemap from runB: %v", v)
		}
	})
}

// ============================================================================
// Group D: Completeness & Limits Tests (Section 15.D)
// ============================================================================

func TestAdapter_Sitemap_Completeness(t *testing.T) {
	t.Run("Discovery disabled (NOT_ATTEMPTED) emits gap and complete=false", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:comp:disabled"
		auditRunID := audit.AuditRunID("audit:sm:comp:disabled")
		snapshotID := audit.SnapshotID("snap:sm:comp:disabled")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped
		) VALUES (?, 'NOT_ATTEMPTED', 0, 0, 0, 0, 0, 0)`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if res.EvidenceSnapshot.SitemapDiscoveryComplete {
			t.Errorf("expected SitemapDiscoveryComplete to be false when discovery disabled")
		}

		siteObs := getObservations(res.EvidenceSnapshot, "site")
		if v := siteObs["sitemap_discovery_status"]; len(v) != 1 || v[0] != "NOT_ATTEMPTED" {
			t.Errorf("expected NOT_ATTEMPTED, got %v", v)
		}
		if v := siteObs["sitemap_discovery_complete"]; len(v) != 1 || v[0] != "false" {
			t.Errorf("expected sitemap_discovery_complete 'false', got %v", v)
		}

		if !hasGap(res.EvidenceGaps, adapter.GapSitemapDiscoveryDisabled) {
			t.Errorf("expected GapSitemapDiscoveryDisabled gap")
		}
	})

	t.Run("Completed discovery with zero sitemaps found", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:comp:zero"
		auditRunID := audit.AuditRunID("audit:sm:comp:zero")
		snapshotID := audit.SnapshotID("snap:sm:comp:zero")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped
		) VALUES (?, 'COMPLETED', 0, 0, 0, 0, 0, 0)`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if !res.EvidenceSnapshot.SitemapDiscoveryComplete {
			t.Errorf("expected SitemapDiscoveryComplete true when discovery completed cleanly")
		}

		siteObs := getObservations(res.EvidenceSnapshot, "site")
		if v := siteObs["sitemaps_found_count"]; len(v) != 1 || v[0] != "0" {
			t.Errorf("expected sitemaps_found_count '0', got %v", v)
		}

		if !hasGap(res.EvidenceGaps, adapter.GapSitemapDocumentUnavailable) {
			t.Errorf("expected GapSitemapDocumentUnavailable gap when completed discovery found 0 documents")
		}
	})

	t.Run("Partial discovery (ATTEMPTED_INCOMPLETE) emits gap", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:comp:partial"
		auditRunID := audit.AuditRunID("audit:sm:comp:partial")
		snapshotID := audit.SnapshotID("snap:sm:comp:partial")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, stop_reason
		) VALUES (?, 'ATTEMPTED_INCOMPLETE', 1, 50, 'context_canceled')`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if res.EvidenceSnapshot.SitemapDiscoveryComplete {
			t.Errorf("expected SitemapDiscoveryComplete false for partial discovery")
		}

		siteObs := getObservations(res.EvidenceSnapshot, "site")
		if v := siteObs["sitemap_discovery_stop_reason"]; len(v) != 1 || v[0] != "context_canceled" {
			t.Errorf("expected stop reason context_canceled, got %v", v)
		}

		if !hasGap(res.EvidenceGaps, adapter.GapSitemapDiscoveryIncomplete) {
			t.Errorf("expected GapSitemapDiscoveryIncomplete gap")
		}
	})

	t.Run("URL capped, Depth capped, Byte capped emit GapSitemapTruncated", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sm:comp:caps"
		auditRunID := audit.AuditRunID("audit:sm:comp:caps")
		snapshotID := audit.SnapshotID("snap:sm:comp:caps")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(
			run_id, status, sitemaps_found, entries_found, depth_capped, urls_capped, byte_capped, stop_reason
		) VALUES (?, 'ATTEMPTED_INCOMPLETE', 5, 200000, 1, 1, 1, 'urls_capped')`, runID)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if res.EvidenceSnapshot.SitemapDiscoveryComplete {
			t.Errorf("expected SitemapDiscoveryComplete false when caps hit")
		}

		siteObs := getObservations(res.EvidenceSnapshot, "site")
		if v := siteObs["sitemap_depth_capped"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected sitemap_depth_capped true, got %v", v)
		}
		if v := siteObs["sitemap_urls_capped"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected sitemap_urls_capped true, got %v", v)
		}
		if v := siteObs["sitemap_byte_capped"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected sitemap_byte_capped true, got %v", v)
		}

		if !hasGap(res.EvidenceGaps, adapter.GapSitemapTruncated) {
			t.Errorf("expected GapSitemapTruncated gap")
		}
	})

	t.Run("Legacy run without sitemap tables handled safely", func(t *testing.T) {
		// Open memory DB WITHOUT running sitecrawl schema initialization
		db, err := standalone.OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open memory DB: %v", err)
		}
		defer db.Close()

		// Create only minimal legacy tables: sitecrawl_runs, sitecrawl_urls, sitecrawl_pages, sitecrawl_links
		_, err = db.Exec(`
			CREATE TABLE sitecrawl_runs (
				id TEXT PRIMARY KEY, state TEXT, stop_reason TEXT, seed_url TEXT,
				host TEXT, options TEXT, started_at TEXT, finished_at TEXT, found INTEGER, crawled INTEGER, total INTEGER
			);
			CREATE TABLE sitecrawl_urls (id INTEGER PRIMARY KEY, run_id TEXT, url TEXT);
			CREATE TABLE sitecrawl_pages (
				run_id TEXT, url_id INTEGER, url TEXT, data TEXT, kind TEXT, is_internal INTEGER DEFAULT 1,
				depth INTEGER DEFAULT 0, discovered_by TEXT DEFAULT '', status INTEGER DEFAULT 200,
				content_type TEXT DEFAULT 'text/html', size_bytes INTEGER DEFAULT 0, response_ms INTEGER DEFAULT 0,
				redirect_to TEXT DEFAULT '', redirect_hops INTEGER DEFAULT 0, error_type TEXT DEFAULT '',
				title TEXT DEFAULT '', meta_desc TEXT DEFAULT '', h1 TEXT DEFAULT '', lang TEXT DEFAULT '',
				canonical TEXT DEFAULT '', meta_robots TEXT DEFAULT '', x_robots TEXT DEFAULT '',
				rendered INTEGER DEFAULT 0, crawled_at TEXT, robots_state TEXT DEFAULT 'unknown',
				PRIMARY KEY (run_id, url_id)
			);
			CREATE TABLE sitecrawl_links (
				run_id TEXT, src_id INTEGER, dst_id INTEGER, seq INTEGER DEFAULT 0,
				placement INTEGER DEFAULT 0, flags INTEGER DEFAULT 0, anchor TEXT DEFAULT ''
			);
		`)
		if err != nil {
			t.Fatalf("create legacy tables: %v", err)
		}

		runID := "run:sm:legacy"
		auditRunID := audit.AuditRunID("audit:sm:legacy")
		snapshotID := audit.SnapshotID("snap:sm:legacy")
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build should succeed on legacy DB, got error: %v", err)
		}

		if res.EvidenceSnapshot.SitemapDiscoveryComplete {
			t.Errorf("expected SitemapDiscoveryComplete false on legacy run")
		}
		if !hasGap(res.EvidenceGaps, adapter.GapSitemapDocumentUnavailable) {
			t.Errorf("expected GapSitemapDocumentUnavailable gap on legacy run")
		}
	})
}

// ============================================================================
// Group E: Determinism & Integrity Tests (Section 15.E)
// ============================================================================

func TestAdapter_Sitemap_Determinism(t *testing.T) {
	db := newTestDB(t)
	runID := "run:sm:det"
	auditRunID := audit.AuditRunID("audit:sm:det")
	snapshotID := audit.SnapshotID("snap:sm:det")
	setupSitemapTestRun(t, db, runID, "https://example.com/")

	insertURL(t, db, runID, 1, "https://example.com/")
	insertURL(t, db, runID, 2, "https://example.com/about")
	insertPage(t, db, runID, 1, "https://example.com/", 200)
	insertPage(t, db, runID, 2, "https://example.com/about", 200)

	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 2, 2)`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
		VALUES (?, 1, 'https://example.com/sitemap1.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
		VALUES (?, 2, 'https://example.com/sitemap2.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)

	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 2, 'common_path', 0)`, runID)

	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, 'https://example.com/')`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 2, 0, 2, 'https://example.com/about')`, runID)

	res1, err1 := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapshotID,
	})
	if err1 != nil {
		t.Fatalf("build 1 failed: %v", err1)
	}

	res2, err2 := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapshotID,
	})
	if err2 != nil {
		t.Fatalf("build 2 failed: %v", err2)
	}

	// 1. Observation counts match
	obs1 := res1.EvidenceSnapshot.NormalizedObservations
	obs2 := res2.EvidenceSnapshot.NormalizedObservations
	if len(obs1) != len(obs2) {
		t.Fatalf("observation count mismatch: %d vs %d", len(obs1), len(obs2))
	}

	// 2. Exact field-by-field observation equality
	for i := range obs1 {
		o1 := obs1[i]
		o2 := obs2[i]
		if o1.ObservationID != o2.ObservationID {
			t.Errorf("obs[%d] ID mismatch: %s vs %s", i, o1.ObservationID, o2.ObservationID)
		}
		if o1.SubjectType != o2.SubjectType || o1.SubjectRef != o2.SubjectRef {
			t.Errorf("obs[%d] subject mismatch: (%s, %s) vs (%s, %s)", i, o1.SubjectType, o1.SubjectRef, o2.SubjectType, o2.SubjectRef)
		}
		if o1.Field != o2.Field || o1.Value != o2.Value {
			t.Errorf("obs[%d] field/value mismatch: (%s=%s) vs (%s=%s)", i, o1.Field, o1.Value, o2.Field, o2.Value)
		}
	}

	// 3. No duplicate ObservationIDs
	seenIDs := make(map[audit.ObservationID]bool)
	for _, o := range obs1 {
		if seenIDs[o.ObservationID] {
			t.Errorf("duplicate observation ID detected: %s", o.ObservationID)
		}
		seenIDs[o.ObservationID] = true
	}
}

// ============================================================================
// Group F: Existing 13-Rule Regression Tests (Section 15.F)
// ============================================================================

func TestAdapter_Sitemap_ExistingRuleRegression(t *testing.T) {
	db := newTestDB(t)
	runID := "run:sm:regr"
	auditRunID := audit.AuditRunID("audit:sm:regr")
	snapshotID := audit.SnapshotID("snap:sm:regr")
	seed := "https://example.com/"
	setupSitemapTestRun(t, db, runID, seed)

	insertURL(t, db, runID, 1, seed)

	rawJSON := `{"@context":"https://schema.org","@type":"WebSite","name":"Test"}`
	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:       seed,
		Status:    200,
		Kind:      "html",
		CrawledAt: "2026-10-01T00:00:10Z",
		JSONLD:    []string{rawJSON},
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
		VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	insertLink(t, db, runID, 1, 1, 0, 1, 1, "Home")

	// Add valid sitemap evidence
	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
		VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapshotID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// 1. Verify NormalizationVersion is v1.9.0
	if res.EvidenceSnapshot.NormalizationVersion != "v1.9.0" {
		t.Errorf("expected NormalizationVersion v1.9.0, got %s", res.EvidenceSnapshot.NormalizationVersion)
	}

	// 2. Run rule engine against the snapshot
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	// Verify exactly 19 evaluators implemented
	implIDs := eng.ImplementedRuleIDs()
	if len(implIDs) != 19 {
		t.Errorf("expected exactly 19 implemented rules, got %d: %v", len(implIDs), implIDs)
	}

	// Execute all 19 evaluators
	for _, ruleID := range implIDs {
		results, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, ruleID)
		if err != nil {
			t.Fatalf("evaluating rule %s failed: %v", ruleID, err)
		}
		if len(results) == 0 {
			t.Errorf("expected rule %s to evaluate to results, got 0", ruleID)
		}
	}

	// Verify all 6 sitemap rules (AR-DISC-001 through AR-DISC-006) are implemented
	for _, discRuleID := range []string{"AR-DISC-001", "AR-DISC-002", "AR-DISC-003", "AR-DISC-004", "AR-DISC-005", "AR-DISC-006"} {
		_, err = eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, discRuleID)
		if err != nil {
			t.Errorf("sitemap rule %s must be implemented in V1.8g: %v", discRuleID, err)
		}
	}
}

// ============================================================================
// Group G: V1.8c Correction Mandatory Regression Tests (Section 6)
// ============================================================================

func TestAdapter_Sitemap_V1_8c_MandatoryRegressions(t *testing.T) {
	// A. Fetch-status semantics
	t.Run("A. Fetch-status semantics across 4 redirect/fetch scenarios", func(t *testing.T) {
		scenarios := []struct {
			name          string
			initStatus    int
			finalStatus   int
			redirectHops  int
			redirectTo    string
			fetchError    string
			fetchComplete int
		}{
			{
				name:          "1. Direct HTTP 200 sitemap",
				initStatus:    200,
				finalStatus:   200,
				redirectHops:  0,
				redirectTo:    "",
				fetchError:    "",
				fetchComplete: 1,
			},
			{
				name:          "2. Initial 301 to final 200",
				initStatus:    301,
				finalStatus:   200,
				redirectHops:  1,
				redirectTo:    "https://example.com/sitemap_target.xml",
				fetchError:    "",
				fetchComplete: 1,
			},
			{
				name:          "3. Initial 301 to final 404",
				initStatus:    301,
				finalStatus:   404,
				redirectHops:  1,
				redirectTo:    "https://example.com/missing.xml",
				fetchError:    "",
				fetchComplete: 1,
			},
			{
				name:          "4. Fetch failure before HTTP response",
				initStatus:    0,
				finalStatus:   0,
				redirectHops:  0,
				redirectTo:    "",
				fetchError:    "dial tcp: connection refused",
				fetchComplete: 0,
			},
		}

		eng, err := engine.New()
		if err != nil {
			t.Fatalf("engine.New failed: %v", err)
		}

		for _, tc := range scenarios {
			t.Run(tc.name, func(t *testing.T) {
				db := newTestDB(t)
				runID := fmt.Sprintf("run:v18c:fetch:%d_%d", tc.initStatus, tc.finalStatus)
				auditRunID := audit.AuditRunID("audit:" + runID)
				snapshotID := audit.SnapshotID("snap:" + runID)
				setupSitemapTestRun(t, db, runID, "https://example.com/")

				_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 0)`, runID)
				_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(
					run_id, id, url, initial_status, final_status, status, redirect_hops, redirect_to, fetch_error, fetch_complete, doc_type, parse_status
				) VALUES (?, 1, 'https://example.com/sitemap.xml', ?, ?, ?, ?, ?, ?, ?, 'urlset', 'parsed')`,
					runID, tc.initStatus, tc.finalStatus, tc.finalStatus, tc.redirectHops, tc.redirectTo, tc.fetchError, tc.fetchComplete)

				res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
					CrawlRunID: runID,
					AuditRunID: auditRunID,
					SnapshotID: snapshotID,
				})
				if err != nil {
					t.Fatalf("build failed: %v", err)
				}

				docRef := fmt.Sprintf("sm:%s:1", auditRunID)
				obs := getObservations(res.EvidenceSnapshot, docRef)

				// Initial and final HTTP status observations are independent
				if v := obs["initial_http_status"]; len(v) != 1 || v[0] != fmt.Sprintf("%d", tc.initStatus) {
					t.Errorf("expected initial_http_status %d, got %v", tc.initStatus, v)
				}
				if v := obs["final_http_status"]; len(v) != 1 || v[0] != fmt.Sprintf("%d", tc.finalStatus) {
					t.Errorf("expected final_http_status %d, got %v", tc.finalStatus, v)
				}

				// Redirect evidence is retained
				if v := obs["sitemap_redirect_hops"]; len(v) != 1 || v[0] != fmt.Sprintf("%d", tc.redirectHops) {
					t.Errorf("expected sitemap_redirect_hops %d, got %v", tc.redirectHops, v)
				}
				if tc.redirectTo != "" {
					if v := obs["sitemap_final_url"]; len(v) != 1 || v[0] != tc.redirectTo {
						t.Errorf("expected sitemap_final_url %q, got %v", tc.redirectTo, v)
					}
				}
				if tc.fetchError != "" {
					if v := obs["sitemap_fetch_error"]; len(v) != 1 || v[0] != tc.fetchError {
						t.Errorf("expected sitemap_fetch_error %q, got %v", tc.fetchError, v)
					}
				}

				// Negative assertion: no normalized sitemap_fetch_status field is emitted anywhere
				if v := obs["sitemap_fetch_status"]; len(v) != 0 {
					t.Errorf("forbidden sitemap_fetch_status observation emitted on document: %v", v)
				}
				for _, o := range res.EvidenceSnapshot.NormalizedObservations {
					if o.Field == "sitemap_fetch_status" {
						t.Fatalf("forbidden normalized observation sitemap_fetch_status found in snapshot on subject %s", o.SubjectRef)
					}
				}

				// AR-DISC-002 is executable in V1.8g
				_, err = eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, "AR-DISC-002")
				if err != nil {
					t.Errorf("AR-DISC-002 must be executable in V1.8g: %v", err)
				}

				// All implemented sitemap rules evaluate without error
				for _, rID := range eng.ImplementedRuleIDs() {
					results, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, rID)
					if err != nil {
						t.Fatalf("rule %s evaluation failed: %v", rID, err)
					}
					for _, r := range results {
						if len(r.RuleID) >= 7 && r.RuleID[:7] == "AR-DISC" &&
							r.RuleID != "AR-DISC-001" && r.RuleID != "AR-DISC-002" &&
							r.RuleID != "AR-DISC-003" && r.RuleID != "AR-DISC-004" &&
							r.RuleID != "AR-DISC-005" && r.RuleID != "AR-DISC-006" {
							t.Fatalf("unexpected sitemap RuleResult generated: %s", r.RuleID)
						}
					}
				}
			})
		}
	})

	// B. Orphan entry correlation
	t.Run("B. Orphan entry with valid url_id and matching dictionary URL", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:v18c:orphan:validurl"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		pageURL := "https://example.com/orphan-page"
		insertURL(t, db, runID, 42, pageURL)
		insertPage(t, db, runID, 42, pageURL, 200)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 0, 1)`, runID)
		// Missing parent sitemap document 77; valid url_id = 42
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 77, 0, 42, ?)`, runID, pageURL)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedEID := fmt.Sprintf("sme:%s:77:0", auditRunID)

		// 1. Entry subject exists
		if len(res.SitemapEntries) != 1 {
			t.Fatalf("expected 1 sitemap entry, got %d", len(res.SitemapEntries))
		}
		se := res.SitemapEntries[0]
		if string(se.SitemapEntryID) != expectedEID {
			t.Errorf("expected entry ID %s, got %s", expectedEID, se.SitemapEntryID)
		}

		// 2. GAP_ORPHAN_SITEMAP_ENTRY exists
		gap := findGap(res.EvidenceGaps, adapter.GapOrphanSitemapEntry, expectedEID)
		if gap == nil {
			t.Errorf("expected GapOrphanSitemapEntry gap for orphan entry")
		}

		// 3. No parent_sitemap_ref
		entObs := getObservations(res.EvidenceSnapshot, expectedEID)
		if v := entObs["parent_sitemap_ref"]; len(v) != 0 {
			t.Errorf("expected no parent_sitemap_ref on orphan entry, got %v", v)
		}

		// 4. No url_subject_ref
		if v := entObs["url_subject_ref"]; len(v) != 0 {
			t.Errorf("expected no url_subject_ref on orphan entry, got %v", v)
		}

		// 5. Typed SitemapEntry.ListedURLID is empty
		if se.ListedURLID != "" {
			t.Errorf("expected empty ListedURLID on orphan entry, got %q", se.ListedURLID)
		}

		// 6. No bilateral sitemap references appear on the URL subject
		urlSubjRef := fmt.Sprintf("url:%s:42", auditRunID)
		urlObs := getObservations(res.EvidenceSnapshot, urlSubjRef)
		if v := urlObs["listed_in_sitemap"]; len(v) != 0 {
			t.Errorf("expected no listed_in_sitemap on URL subject for orphan entry, got %v", v)
		}
		if v := urlObs["sitemap_entry_ref"]; len(v) != 0 {
			t.Errorf("expected no sitemap_entry_ref on URL subject for orphan entry, got %v", v)
		}
		if v := urlObs["sitemap_document_ref"]; len(v) != 0 {
			t.Errorf("expected no sitemap_document_ref on URL subject for orphan entry, got %v", v)
		}
		if v := urlObs["sitemap_url"]; len(v) != 0 {
			t.Errorf("expected no sitemap_url on URL subject for orphan entry, got %v", v)
		}

		// 7. Raw loc remains preserved
		if se.ListedURLRaw != pageURL {
			t.Errorf("expected ListedURLRaw %q, got %q", pageURL, se.ListedURLRaw)
		}
		if v := entObs["loc_raw"]; len(v) != 1 || v[0] != pageURL {
			t.Errorf("expected loc_raw %q, got %v", pageURL, v)
		}
	})

	// C. Valid correlation non-regression
	t.Run("C. Valid correlation non-regression", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:v18c:valid:corr"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		pageURL := "https://example.com/valid-page"
		insertURL(t, db, runID, 10, pageURL)
		insertPage(t, db, runID, 10, pageURL, 200)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 10, ?)`, runID, pageURL)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		expectedDocRef := fmt.Sprintf("sm:%s:1", auditRunID)
		expectedEID := fmt.Sprintf("sme:%s:1:0", auditRunID)
		expectedURLRef := fmt.Sprintf("url:%s:10", auditRunID)

		if len(res.SitemapEntries) != 1 {
			t.Fatalf("expected 1 sitemap entry, got %d", len(res.SitemapEntries))
		}
		se := res.SitemapEntries[0]

		// parent_sitemap_ref exists
		entObs := getObservations(res.EvidenceSnapshot, expectedEID)
		if v := entObs["parent_sitemap_ref"]; len(v) != 1 || v[0] != expectedDocRef {
			t.Errorf("expected parent_sitemap_ref %s, got %v", expectedDocRef, v)
		}

		// url_subject_ref exists
		if v := entObs["url_subject_ref"]; len(v) != 1 || v[0] != expectedURLRef {
			t.Errorf("expected url_subject_ref %s, got %v", expectedURLRef, v)
		}

		// Typed ListedURLID is correct
		if string(se.ListedURLID) != expectedURLRef {
			t.Errorf("expected ListedURLID %s, got %s", expectedURLRef, se.ListedURLID)
		}

		// Bilateral URL observations reference the correct entry
		urlObs := getObservations(res.EvidenceSnapshot, expectedURLRef)
		if v := urlObs["listed_in_sitemap"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected listed_in_sitemap 'true', got %v", v)
		}
		if v := urlObs["sitemap_entry_ref"]; len(v) != 1 || v[0] != expectedEID {
			t.Errorf("expected sitemap_entry_ref %s, got %v", expectedEID, v)
		}
		if v := urlObs["sitemap_document_ref"]; len(v) != 1 || v[0] != expectedDocRef {
			t.Errorf("expected sitemap_document_ref %s, got %v", expectedDocRef, v)
		}

		// No inappropriate orphan or correlation gaps
		if hasGap(res.EvidenceGaps, adapter.GapOrphanSitemapEntry) {
			t.Errorf("unexpected GapOrphanSitemapEntry gap")
		}
		if hasGap(res.EvidenceGaps, adapter.GapInvalidSitemapURLCorrelation) {
			t.Errorf("unexpected GapInvalidSitemapURLCorrelation gap")
		}
	})

	// D. Duplicate behavior
	t.Run("D. Duplicate entries and orphan isolation", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:v18c:dupe:orphan"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		commonURL := "https://example.com/duplicate"
		insertURL(t, db, runID, 5, commonURL)
		insertPage(t, db, runID, 5, commonURL, 200)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 3)`, runID)
		// Valid sitemap doc 1
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 2)`, runID)

		// Two duplicate entries in valid doc 1 (seq 0 and seq 1)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 5, ?)`, runID, commonURL)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 1, 5, ?)`, runID, commonURL)

		// One orphan entry in non-existent sitemap doc 99 (seq 0) referencing same URL
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 99, 0, 5, ?)`, runID, commonURL)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.SitemapEntries) != 3 {
			t.Fatalf("expected 3 entries, got %d", len(res.SitemapEntries))
		}

		eid0 := fmt.Sprintf("sme:%s:1:0", auditRunID)
		eid1 := fmt.Sprintf("sme:%s:1:1", auditRunID)
		orphanEID := fmt.Sprintf("sme:%s:99:0", auditRunID)
		docRef := fmt.Sprintf("sm:%s:1", auditRunID)
		urlRef := fmt.Sprintf("url:%s:5", auditRunID)

		// Preserve distinct entry identities
		if string(res.SitemapEntries[0].SitemapEntryID) != eid0 || string(res.SitemapEntries[1].SitemapEntryID) != eid1 {
			t.Errorf("duplicate entries must preserve distinct identities: %s, %s", res.SitemapEntries[0].SitemapEntryID, res.SitemapEntries[1].SitemapEntryID)
		}

		// Both duplicates retain correct parent document references and URL correlation
		for _, eid := range []string{eid0, eid1} {
			obs := getObservations(res.EvidenceSnapshot, eid)
			if v := obs["parent_sitemap_ref"]; len(v) != 1 || v[0] != docRef {
				t.Errorf("entry %s expected parent_sitemap_ref %s, got %v", eid, docRef, v)
			}
			if v := obs["url_subject_ref"]; len(v) != 1 || v[0] != urlRef {
				t.Errorf("entry %s expected url_subject_ref %s, got %v", eid, urlRef, v)
			}
		}

		// Orphan entry does not borrow correlation
		orphanObs := getObservations(res.EvidenceSnapshot, orphanEID)
		if v := orphanObs["parent_sitemap_ref"]; len(v) != 0 {
			t.Errorf("orphan entry must not have parent_sitemap_ref, got %v", v)
		}
		if v := orphanObs["url_subject_ref"]; len(v) != 0 {
			t.Errorf("orphan entry must not have url_subject_ref, got %v", v)
		}
		if res.SitemapEntries[2].ListedURLID != "" {
			t.Errorf("orphan entry must have empty ListedURLID, got %s", res.SitemapEntries[2].ListedURLID)
		}

		// Bilateral URL subject references exactly the 2 valid entries, NOT the orphan entry
		urlObs := getObservations(res.EvidenceSnapshot, urlRef)
		if v := urlObs["sitemap_entry_ref"]; len(v) != 2 {
			t.Fatalf("expected exactly 2 sitemap_entry_ref on URL subject, got %d: %v", len(v), v)
		}
		for _, ref := range urlObs["sitemap_entry_ref"] {
			if ref == orphanEID {
				t.Errorf("URL subject must NOT reference orphan entry %s", orphanEID)
			}
		}
	})

	// E. Determinism
	t.Run("E. Determinism across two builds", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:v18c:determinism"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupSitemapTestRun(t, db, runID, "https://example.com/")

		insertURL(t, db, runID, 1, "https://example.com/p1")
		insertPage(t, db, runID, 1, "https://example.com/p1", 200)
		insertURL(t, db, runID, 2, "https://example.com/p2")
		insertPage(t, db, runID, 2, "https://example.com/p2", 200)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 2, 3)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap_a.xml', 200, 200, 200, 1, 'urlset', 'parsed', 2)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, redirect_hops, redirect_to, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 2, 'https://example.com/sitemap_b.xml', 301, 200, 200, 1, 'https://example.com/sitemap_target.xml', 1, 'urlset', 'parsed', 1)`, runID)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, 'https://example.com/p1')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 1, 1, 'https://example.com/p1')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 2, 0, 2, 'https://example.com/p2')`, runID)
		// Orphan entry
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 99, 0, 1, 'https://example.com/p1')`, runID)

		res1, err1 := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err1 != nil {
			t.Fatalf("build 1 failed: %v", err1)
		}

		res2, err2 := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err2 != nil {
			t.Fatalf("build 2 failed: %v", err2)
		}

		// NormalizationVersion remains v1.9.0
		if res1.EvidenceSnapshot.NormalizationVersion != "v1.9.0" {
			t.Errorf("expected NormalizationVersion v1.9.0, got %s", res1.EvidenceSnapshot.NormalizationVersion)
		}

		// Observation counts and ordering match
		obs1 := res1.EvidenceSnapshot.NormalizedObservations
		obs2 := res2.EvidenceSnapshot.NormalizedObservations
		if len(obs1) != len(obs2) {
			t.Fatalf("observation count mismatch: %d vs %d", len(obs1), len(obs2))
		}
		for i := range obs1 {
			if obs1[i].ObservationID != obs2[i].ObservationID {
				t.Fatalf("obs[%d] ID mismatch: %s vs %s", i, obs1[i].ObservationID, obs2[i].ObservationID)
			}
			if obs1[i].SubjectRef != obs2[i].SubjectRef || obs1[i].Field != obs2[i].Field || obs1[i].Value != obs2[i].Value {
				t.Fatalf("obs[%d] mismatch: (%s, %s, %s) vs (%s, %s, %s)",
					i, obs1[i].SubjectRef, obs1[i].Field, obs1[i].Value,
					obs2[i].SubjectRef, obs2[i].Field, obs2[i].Value)
			}
		}

		// Verify no dangling references and no sitemap_fetch_status in either snapshot
		for _, o := range obs1 {
			if o.Field == "sitemap_fetch_status" {
				t.Errorf("forbidden observation sitemap_fetch_status found in determinism snapshot")
			}
		}
	})
}
