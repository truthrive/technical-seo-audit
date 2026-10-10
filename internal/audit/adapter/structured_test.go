package adapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// Helper to query observations for a specific subject ref.
func getStructuredDataObservations(snap *audit.EvidenceSnapshot, subjectRef string) map[string][]string {
	fields := make(map[string][]string)
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == subjectRef {
			fields[obs.Field] = append(fields[obs.Field], obs.Value)
		}
	}
	return fields
}

// ============================================================================
// Group A: JSON-LD Tests (Section 13.A)
// ============================================================================

func TestAdapter_StructuredData_JSONLD(t *testing.T) {
	t.Run("Valid object", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:obj"
		auditRunID := audit.AuditRunID("audit:sd:obj")
		snapshotID := audit.SnapshotID("snap:sd:obj")
		seed := "https://example.com/page"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		rawJSON := `{"@context": "https://schema.org", "@type": "Organization", "name": "Acme Corp"}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{rawJSON},
		})
		_, err := db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))
		if err != nil {
			t.Fatalf("insert page failed: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 1 {
			t.Fatalf("expected 1 StructuredDataBlock, got %d", len(res.StructuredDataBlocks))
		}
		b := res.StructuredDataBlocks[0]
		expectedID := audit.StructuredBlockID(fmt.Sprintf("sdb:%s:1:jsonld:0", auditRunID))
		if b.StructuredBlockID != expectedID {
			t.Errorf("expected block ID %s, got %s", expectedID, b.StructuredBlockID)
		}
		if b.Format != audit.FormatJSONLD {
			t.Errorf("expected format %s, got %s", audit.FormatJSONLD, b.Format)
		}
		if b.ParseStatus != adapter.ParseStatusSuccess {
			t.Errorf("expected parse status %s, got %s", adapter.ParseStatusSuccess, b.ParseStatus)
		}
		if b.ParseError != "" {
			t.Errorf("expected empty parse error, got %q", b.ParseError)
		}

		obs := getStructuredDataObservations(res.EvidenceSnapshot, string(expectedID))
		if v := obs["structured_parse_status"]; len(v) != 1 || v[0] != adapter.ParseStatusSuccess {
			t.Errorf("expected structured_parse_status %s, got %v", adapter.ParseStatusSuccess, v)
		}
		if v := obs["structured_raw"]; len(v) != 1 || v[0] != rawJSON {
			t.Errorf("expected structured_raw to match raw JSON, got %v", v)
		}
		if v := obs["structured_format"]; len(v) != 1 || v[0] != string(audit.FormatJSONLD) {
			t.Errorf("expected structured_format %s, got %v", audit.FormatJSONLD, v)
		}
		if v := obs["structured_parse_error"]; len(v) != 0 {
			t.Errorf("expected no structured_parse_error observation for valid JSON, got %v", v)
		}
	})

	t.Run("Valid array", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:arr"
		auditRunID := audit.AuditRunID("audit:sd:arr")
		snapshotID := audit.SnapshotID("snap:sd:arr")
		seed := "https://example.com/array"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		rawJSON := `[{"@type": "Person", "name": "Alice"}, {"@type": "Person", "name": "Bob"}]`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{rawJSON},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 1 {
			t.Fatalf("expected 1 StructuredDataBlock, got %d", len(res.StructuredDataBlocks))
		}
		b := res.StructuredDataBlocks[0]
		if b.ParseStatus != adapter.ParseStatusSuccess {
			t.Errorf("expected parse status %s, got %s", adapter.ParseStatusSuccess, b.ParseStatus)
		}
	})

	t.Run("Valid @graph JSON", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:graph"
		auditRunID := audit.AuditRunID("audit:sd:graph")
		snapshotID := audit.SnapshotID("snap:sd:graph")
		seed := "https://example.com/graph"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		rawJSON := `{"@context": "https://schema.org", "@graph": [{"@type": "WebSite", "name": "My Site"}, {"@type": "WebPage", "name": "Home"}]}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{rawJSON},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(res.StructuredDataBlocks))
		}
		if res.StructuredDataBlocks[0].ParseStatus != adapter.ParseStatusSuccess {
			t.Errorf("expected %s, got %s", adapter.ParseStatusSuccess, res.StructuredDataBlocks[0].ParseStatus)
		}
	})

	t.Run("Multiple valid blocks on one page", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:multi"
		auditRunID := audit.AuditRunID("audit:sd:multi")
		snapshotID := audit.SnapshotID("snap:sd:multi")
		seed := "https://example.com/multi"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		raw1 := `{"@type": "BreadcrumbList", "itemListElement": []}`
		raw2 := `{"@type": "Organization", "name": "Acme"}`
		raw3 := `{"@type": "WebPage", "headline": "Multi Block"}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{raw1, raw2, raw3},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 3 {
			t.Fatalf("expected 3 blocks, got %d", len(res.StructuredDataBlocks))
		}
		for i, b := range res.StructuredDataBlocks {
			if b.BlockIndex != i {
				t.Errorf("block %d has index %d", i, b.BlockIndex)
			}
			expectedID := audit.StructuredBlockID(fmt.Sprintf("sdb:%s:1:jsonld:%d", auditRunID, i))
			if b.StructuredBlockID != expectedID {
				t.Errorf("block %d expected ID %s, got %s", i, expectedID, b.StructuredBlockID)
			}
			if b.ParseStatus != adapter.ParseStatusSuccess {
				t.Errorf("block %d expected success, got %s", i, b.ParseStatus)
			}
		}

		// Verify containing SubjectURL carries all 3 structured_block_ref observations
		urlObs := getStructuredDataObservations(res.EvidenceSnapshot, fmt.Sprintf("url:%s:1", auditRunID))
		refs := urlObs["structured_block_ref"]
		if len(refs) != 3 {
			t.Errorf("expected 3 structured_block_ref on URL, got %d (%v)", len(refs), refs)
		}
	})

	t.Run("Malformed JSON", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:malformed"
		auditRunID := audit.AuditRunID("audit:sd:malformed")
		snapshotID := audit.SnapshotID("snap:sd:malformed")
		seed := "https://example.com/malformed"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		rawJSON := `{name: "unquoted keys without proper json syntax"}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{rawJSON},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		b := res.StructuredDataBlocks[0]
		if b.ParseStatus != adapter.ParseStatusError {
			t.Errorf("expected parse status %s, got %s", adapter.ParseStatusError, b.ParseStatus)
		}
		if b.ParseError == "" {
			t.Errorf("expected non-empty parse error for malformed JSON")
		}

		obs := getStructuredDataObservations(res.EvidenceSnapshot, string(b.StructuredBlockID))
		if v := obs["structured_parse_status"]; len(v) != 1 || v[0] != adapter.ParseStatusError {
			t.Errorf("expected structured_parse_status %s, got %v", adapter.ParseStatusError, v)
		}
		if v := obs["structured_parse_error"]; len(v) != 1 || v[0] == "" {
			t.Errorf("expected structured_parse_error observation, got %v", v)
		}
	})

	t.Run("Invalid trailing comma", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:trailingcomma"
		auditRunID := audit.AuditRunID("audit:sd:trailingcomma")
		snapshotID := audit.SnapshotID("snap:sd:trailingcomma")
		seed := "https://example.com/trailingcomma"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		rawJSON := `{"@type": "Person", "name": "Alice",}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{rawJSON},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		b := res.StructuredDataBlocks[0]
		if b.ParseStatus != adapter.ParseStatusError {
			t.Errorf("expected parse status %s, got %s", adapter.ParseStatusError, b.ParseStatus)
		}
		if b.ParseError == "" {
			t.Errorf("expected non-empty parse error for trailing comma")
		}
	})

	t.Run("Incomplete JSON", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:incomplete"
		auditRunID := audit.AuditRunID("audit:sd:incomplete")
		snapshotID := audit.SnapshotID("snap:sd:incomplete")
		seed := "https://example.com/incomplete"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		rawJSON := `{"@type": "Person", "name": "Trun`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{rawJSON},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		b := res.StructuredDataBlocks[0]
		if b.ParseStatus != adapter.ParseStatusError {
			t.Errorf("expected parse status %s, got %s", adapter.ParseStatusError, b.ParseStatus)
		}
	})

	t.Run("Empty or whitespace-only input in persisted fixture", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:empty"
		auditRunID := audit.AuditRunID("audit:sd:empty")
		snapshotID := audit.SnapshotID("snap:sd:empty")
		seed := "https://example.com/empty"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{"   \n\t  "},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		b := res.StructuredDataBlocks[0]
		if b.ParseStatus != adapter.ParseStatusEmptyInput {
			t.Errorf("expected parse status %s, got %s", adapter.ParseStatusEmptyInput, b.ParseStatus)
		}
		if b.ParseError == "" {
			t.Errorf("expected non-empty parse error for empty input")
		}
	})

	t.Run("Valid JSON with unusual Schema.org properties", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:unusual"
		auditRunID := audit.AuditRunID("audit:sd:unusual")
		snapshotID := audit.SnapshotID("snap:sd:unusual")
		seed := "https://example.com/unusual"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		rawJSON := `{"@context": "https://custom-vocabulary.org", "unknownProp123": 42, "geoScoreAI": "bogus"}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{rawJSON},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		b := res.StructuredDataBlocks[0]
		if b.ParseStatus != adapter.ParseStatusSuccess {
			t.Errorf("expected parse status %s for valid syntax with unusual properties, got %s", adapter.ParseStatusSuccess, b.ParseStatus)
		}
	})

	t.Run("Valid syntax without @context", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:nocontext"
		auditRunID := audit.AuditRunID("audit:sd:nocontext")
		snapshotID := audit.SnapshotID("snap:sd:nocontext")
		seed := "https://example.com/nocontext"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		rawJSON := `{"@type": "Thing", "name": "Item without context"}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{rawJSON},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		b := res.StructuredDataBlocks[0]
		if b.ParseStatus != adapter.ParseStatusSuccess {
			t.Errorf("expected parse status %s, got %s", adapter.ParseStatusSuccess, b.ParseStatus)
		}
	})

	t.Run("Duplicate blocks with distinct deterministic block IDs", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:dup"
		auditRunID := audit.AuditRunID("audit:sd:dup")
		snapshotID := audit.SnapshotID("snap:sd:dup")
		seed := "https://example.com/dup"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		identicalBlock := `{"@type": "Person", "name": "Identical"}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{identicalBlock, identicalBlock},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(res.StructuredDataBlocks))
		}
		b0 := res.StructuredDataBlocks[0]
		b1 := res.StructuredDataBlocks[1]
		if b0.StructuredBlockID == b1.StructuredBlockID {
			t.Errorf("duplicate blocks must receive distinct IDs: %s vs %s", b0.StructuredBlockID, b1.StructuredBlockID)
		}
		if b0.BlockIndex != 0 || b1.BlockIndex != 1 {
			t.Errorf("expected indices 0 and 1, got %d and %d", b0.BlockIndex, b1.BlockIndex)
		}
	})

	t.Run("Valid @graph with string or null value", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:graph:special"
		auditRunID := audit.AuditRunID("audit:sd:graph:special")
		snapshotID := audit.SnapshotID("snap:sd:graph:special")
		seed := "https://example.com/graph-special"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		raw1 := `{"@graph": "unexpected"}`
		raw2 := `{"@graph": null}`
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{raw1, raw2},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(res.StructuredDataBlocks))
		}
		for i, b := range res.StructuredDataBlocks {
			if b.ParseStatus != adapter.ParseStatusSuccess {
				t.Errorf("block %d expected %s, got %s (err: %s)", i, adapter.ParseStatusSuccess, b.ParseStatus, b.ParseError)
			}
			if b.ParseError != "" {
				t.Errorf("block %d expected empty parse error, got %q", i, b.ParseError)
			}
		}
	})

	t.Run("Valid top-level primitive types and payload preservation", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:primitives"
		auditRunID := audit.AuditRunID("audit:sd:primitives")
		snapshotID := audit.SnapshotID("snap:sd:primitives")
		seed := "https://example.com/primitives"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		primitives := []string{
			`"plain string"`,
			`123`,
			`true`,
			`null`,
		}
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    primitives,
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != len(primitives) {
			t.Fatalf("expected %d blocks, got %d", len(primitives), len(res.StructuredDataBlocks))
		}
		for i, b := range res.StructuredDataBlocks {
			if b.ParseStatus != adapter.ParseStatusSuccess {
				t.Errorf("primitive %d (%s) expected %s, got %s (err: %s)", i, primitives[i], adapter.ParseStatusSuccess, b.ParseStatus, b.ParseError)
			}
			if b.ParseError != "" {
				t.Errorf("primitive %d expected empty parse error, got %q", i, b.ParseError)
			}
			// Verify structured_raw preserves exact persisted payload
			obs := getStructuredDataObservations(res.EvidenceSnapshot, string(b.StructuredBlockID))
			rawVals := obs["structured_raw"]
			if len(rawVals) != 1 || rawVals[0] != primitives[i] {
				t.Errorf("primitive %d expected structured_raw %q, got %v", i, primitives[i], rawVals)
			}
		}
	})

	t.Run("Invalid syntax with unquoted token, concatenated documents, or trailing garbage", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:invalid:tokens"
		auditRunID := audit.AuditRunID("audit:sd:invalid:tokens")
		snapshotID := audit.SnapshotID("snap:sd:invalid:tokens")
		seed := "https://example.com/invalid-tokens"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		invalidInputs := []string{
			`{"name":}`,
			`{"name":"A"} {"name":"B"}`,
			`{"name":"A"} trailing garbage`,
		}
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    invalidInputs,
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != len(invalidInputs) {
			t.Fatalf("expected %d blocks, got %d", len(invalidInputs), len(res.StructuredDataBlocks))
		}
		for i, b := range res.StructuredDataBlocks {
			if b.ParseStatus != adapter.ParseStatusError {
				t.Errorf("invalid input %d (%s) expected %s, got %s", i, invalidInputs[i], adapter.ParseStatusError, b.ParseStatus)
			}
			if b.ParseError == "" {
				t.Errorf("invalid input %d expected non-empty parse error", i)
			}
		}
	})
}

// ============================================================================
// Group B: Microdata Tests (Section 13.B)
// ============================================================================

func TestAdapter_StructuredData_Microdata(t *testing.T) {
	t.Run("Simplified Microdata schema persisted in Page.SchemaOrg", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:microdata"
		auditRunID := audit.AuditRunID("audit:sd:microdata")
		snapshotID := audit.SnapshotID("snap:sd:microdata")
		seed := "https://example.com/microdata"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			SchemaOrg: []sitecrawl.Schema{
				{
					Type: "https://schema.org/Product",
					Properties: map[string]string{
						"name":  "Smartphone",
						"price": "799",
					},
				},
			},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 1 {
			t.Fatalf("expected 1 block, got %d", len(res.StructuredDataBlocks))
		}
		b := res.StructuredDataBlocks[0]
		expectedID := audit.StructuredBlockID(fmt.Sprintf("sdb:%s:1:microdata:0", auditRunID))
		if b.StructuredBlockID != expectedID {
			t.Errorf("expected block ID %s, got %s", expectedID, b.StructuredBlockID)
		}
		if b.Format != audit.FormatMicrodata {
			t.Errorf("expected format %s, got %s", audit.FormatMicrodata, b.Format)
		}
		if b.ParseStatus != adapter.ParseStatusPartialAcquisition {
			t.Errorf("expected %s, got %s", adapter.ParseStatusPartialAcquisition, b.ParseStatus)
		}
		if b.RawArtifactOrValueRef != "" {
			t.Errorf("expected empty RawArtifactOrValueRef (withheld), got %q", b.RawArtifactOrValueRef)
		}

		obs := getStructuredDataObservations(res.EvidenceSnapshot, string(expectedID))
		if _, hasRaw := obs["structured_raw"]; hasRaw {
			t.Errorf("structured_raw must be withheld for Microdata since raw HTML is unavailable")
		}
		if v := obs["structured_parse_status"]; len(v) != 1 || v[0] != adapter.ParseStatusPartialAcquisition {
			t.Errorf("expected structured_parse_status %s, got %v", adapter.ParseStatusPartialAcquisition, v)
		}
		if v := obs["structured_parse_error"]; len(v) != 1 || v[0] == "" {
			t.Errorf("expected non-empty parse limitation for Microdata, got %v", v)
		}
	})

	t.Run("Multiple schemas and empty properties or missing itemtype", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:microdata:multi"
		auditRunID := audit.AuditRunID("audit:sd:microdata:multi")
		snapshotID := audit.SnapshotID("snap:sd:microdata:multi")
		seed := "https://example.com/microdata-multi"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			SchemaOrg: []sitecrawl.Schema{
				{
					Type:       "https://schema.org/EmptyProps",
					Properties: map[string]string{}, // empty properties
				},
				{
					Type:       "", // missing itemtype
					Properties: map[string]string{"foo": "bar"},
				},
			},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 2 {
			t.Fatalf("expected 2 Microdata blocks, got %d", len(res.StructuredDataBlocks))
		}
		for i, b := range res.StructuredDataBlocks {
			if b.Format != audit.FormatMicrodata {
				t.Errorf("block %d expected format Microdata, got %s", i, b.Format)
			}
			if b.ParseStatus != adapter.ParseStatusPartialAcquisition {
				t.Errorf("block %d expected %s, got %s", i, adapter.ParseStatusPartialAcquisition, b.ParseStatus)
			}
		}
	})
}

// ============================================================================
// Group C: RDFa Tests (Section 13.C)
// ============================================================================

func TestAdapter_StructuredData_RDFa(t *testing.T) {
	t.Run("No RDFa block fabricated and limitation represented truthfully", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:rdfa"
		auditRunID := audit.AuditRunID("audit:sd:rdfa")
		snapshotID := audit.SnapshotID("snap:sd:rdfa")
		seed := "https://example.com/rdfa"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		// Verify zero RDFa blocks fabricated
		for _, b := range res.StructuredDataBlocks {
			if b.Format == audit.FormatRDFa {
				t.Fatalf("no RDFa block should be fabricated, found %v", b)
			}
		}

		// Verify RDFa capability gap is recorded
		hasRDFaGap := false
		for _, g := range res.EvidenceGaps {
			if g.GapCode == adapter.GapRDFaAcquisitionUnavailable {
				hasRDFaGap = true
				break
			}
		}
		if !hasRDFaGap {
			t.Errorf("expected global EvidenceGap %s to be present", adapter.GapRDFaAcquisitionUnavailable)
		}
	})
}

// ============================================================================
// Group D: Coverage Tests (Section 13.D)
// ============================================================================

func TestAdapter_StructuredData_Coverage(t *testing.T) {
	t.Run("JSON-LD block cap generates diagnostic gap", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:cap:jsonld"
		auditRunID := audit.AuditRunID("audit:sd:cap:jsonld")
		snapshotID := audit.SnapshotID("snap:sd:cap:jsonld")
		seed := "https://example.com/cap-jsonld"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		var blocks []string
		for i := 0; i < 20; i++ {
			blocks = append(blocks, fmt.Sprintf(`{"@type": "Item", "index": %d}`, i))
		}
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    blocks,
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 20 {
			t.Fatalf("expected 20 blocks, got %d", len(res.StructuredDataBlocks))
		}

		hasCapGap := false
		for _, g := range res.EvidenceGaps {
			if g.GapCode == adapter.GapJSONLDExtractionCapped {
				hasCapGap = true
				break
			}
		}
		if !hasCapGap {
			t.Errorf("expected gap %s for page with 20 JSON-LD blocks", adapter.GapJSONLDExtractionCapped)
		}
	})

	t.Run("Microdata extraction cap generates diagnostic gap", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:cap:microdata"
		auditRunID := audit.AuditRunID("audit:sd:cap:microdata")
		snapshotID := audit.SnapshotID("snap:sd:cap:microdata")
		seed := "https://example.com/cap-microdata"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		var schemas []sitecrawl.Schema
		for i := 0; i < 40; i++ {
			schemas = append(schemas, sitecrawl.Schema{
				Type:       "https://schema.org/Thing",
				Properties: map[string]string{"idx": fmt.Sprintf("%d", i)},
			})
		}
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			SchemaOrg: schemas,
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		hasCapGap := false
		for _, g := range res.EvidenceGaps {
			if g.GapCode == adapter.GapMicrodataExtractionCapped {
				hasCapGap = true
				break
			}
		}
		if !hasCapGap {
			t.Errorf("expected gap %s for page with 40 Microdata schemas", adapter.GapMicrodataExtractionCapped)
		}
	})

	t.Run("Page with both JSON-LD and Microdata", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:both"
		auditRunID := audit.AuditRunID("audit:sd:both")
		snapshotID := audit.SnapshotID("snap:sd:both")
		seed := "https://example.com/both"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{`{"@type": "Organization", "name": "Org"}`},
			SchemaOrg: []sitecrawl.Schema{
				{Type: "https://schema.org/Product", Properties: map[string]string{"name": "Prod"}},
			},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(res.StructuredDataBlocks))
		}
		b0 := res.StructuredDataBlocks[0]
		b1 := res.StructuredDataBlocks[1]
		if b0.Format != audit.FormatJSONLD || b1.Format != audit.FormatMicrodata {
			t.Errorf("expected JSON_LD then MICRODATA, got %s and %s", b0.Format, b1.Format)
		}
		if b0.StructuredBlockID == b1.StructuredBlockID {
			t.Errorf("block IDs must not collide: %s vs %s", b0.StructuredBlockID, b1.StructuredBlockID)
		}
	})

	t.Run("Page with neither format and non-HTML response", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:none"
		auditRunID := audit.AuditRunID("audit:sd:none")
		snapshotID := audit.SnapshotID("snap:sd:none")
		seed := "https://example.com/image.png"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "image",
			CrawledAt: "2026-10-01T12:00:00Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'image', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 0 {
			t.Errorf("expected 0 structured data blocks for image, got %d", len(res.StructuredDataBlocks))
		}

		// Verify GapStructuredDataAbsenceUnprovable is recorded
		hasAbsenceGap := false
		for _, g := range res.EvidenceGaps {
			if g.GapCode == adapter.GapStructuredDataAbsenceUnprovable {
				hasAbsenceGap = true
				break
			}
		}
		if !hasAbsenceGap {
			t.Errorf("expected gap %s to be present", adapter.GapStructuredDataAbsenceUnprovable)
		}
	})

	t.Run("Page not fetched produces no blocks", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:unfetched"
		auditRunID := audit.AuditRunID("audit:sd:unfetched")
		snapshotID := audit.SnapshotID("snap:sd:unfetched")
		seed := "https://example.com/"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)
		insertURL(t, db, runID, 2, "https://example.com/unfetched") // url present, no page row

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 0 {
			t.Errorf("expected 0 blocks, got %d", len(res.StructuredDataBlocks))
		}
	})
}

// ============================================================================
// Group C2: EvidenceGap Semantics Tests (Section 7.C)
// ============================================================================

func TestAdapter_StructuredData_EvidenceGapSemantics(t *testing.T) {
	t.Run("Scenario 1 — Structured data present: Global capability gap exists without asserting site-level absence", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:gap:present"
		auditRunID := audit.AuditRunID("audit:sd:gap:present")
		snapshotID := audit.SnapshotID("snap:sd:gap:present")
		seed := "https://example.com/present"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    []string{`{"@context": "https://schema.org", "@type": "WebPage", "name": "Present"}`},
			SchemaOrg: []sitecrawl.Schema{{Type: "https://schema.org/Thing", Properties: map[string]string{"name": "Thing"}}},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 2 {
			t.Fatalf("expected 2 blocks, got %d", len(res.StructuredDataBlocks))
		}

		var absenceGap *adapter.EvidenceGap
		for _, g := range res.EvidenceGaps {
			if g.GapCode == adapter.GapStructuredDataAbsenceUnprovable {
				gCopy := g
				absenceGap = &gCopy
				break
			}
		}
		if absenceGap == nil {
			t.Fatalf("expected global gap %s to be present", adapter.GapStructuredDataAbsenceUnprovable)
		}

		// Verify it does NOT attach a URL-specific subject
		if absenceGap.SubjectRef != "" {
			t.Errorf("global capability gap must not attach URL-specific SubjectRef, got %q", absenceGap.SubjectRef)
		}

		// Verify reason is accurate and does not assert that the site/page lacks structured data
		expectedReason := "The frozen SiteCrawl acquisition pipeline does not provide complete structured-data format coverage, including RDFa. Therefore, absence of observed JSON-LD/Microdata cannot be treated as proof that a page contains no structured data."
		if absenceGap.Reason != expectedReason {
			t.Errorf("expected reason %q, got %q", expectedReason, absenceGap.Reason)
		}

		// Verify no fabricated page-level absence observations or verdicts
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.Field == "structured_data_absence" || obs.Field == "has_structured_data" {
				t.Errorf("unexpected page-level absence observation emitted: %+v", obs)
			}
		}
	})

	t.Run("Scenario 2 — No structured data observed: Capability gap describes incomplete coverage without concluding absence", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:gap:empty"
		auditRunID := audit.AuditRunID("audit:sd:gap:empty")
		snapshotID := audit.SnapshotID("snap:sd:gap:empty")
		seed := "https://example.com/no-sd"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: "2026-10-01T12:00:00Z",
			JSONLD:    nil,
			SchemaOrg: nil,
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T12:00:00Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if len(res.StructuredDataBlocks) != 0 {
			t.Fatalf("expected 0 blocks, got %d", len(res.StructuredDataBlocks))
		}

		var absenceGap *adapter.EvidenceGap
		for _, g := range res.EvidenceGaps {
			if g.GapCode == adapter.GapStructuredDataAbsenceUnprovable {
				gCopy := g
				absenceGap = &gCopy
				break
			}
		}
		if absenceGap == nil {
			t.Fatalf("expected global gap %s to be present", adapter.GapStructuredDataAbsenceUnprovable)
		}

		if absenceGap.SubjectRef != "" {
			t.Errorf("global capability gap must not attach URL-specific SubjectRef, got %q", absenceGap.SubjectRef)
		}

		expectedReason := "The frozen SiteCrawl acquisition pipeline does not provide complete structured-data format coverage, including RDFa. Therefore, absence of observed JSON-LD/Microdata cannot be treated as proof that a page contains no structured data."
		if absenceGap.Reason != expectedReason {
			t.Errorf("expected reason %q, got %q", expectedReason, absenceGap.Reason)
		}

		// Verify no automatic NOT_APPLICABLE, PASS, or FAIL rule results produced for AR-ENTITY-001 when no blocks exist
		eng, err := engine.New()
		if err != nil {
			t.Fatalf("engine.New failed: %v", err)
		}
		results, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule AR-ENTITY-001 failed: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results for AR-ENTITY-001 on page without structured data, got %d", len(results))
		}
	})
}

// ============================================================================
// Group E: Evidence Integrity Tests (Section 13.E)
// ============================================================================

func TestAdapter_StructuredData_EvidenceIntegrity(t *testing.T) {
	t.Run("Stable block IDs, stable ObservationIDs, and immutability", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:integrity"
		auditRunID := audit.AuditRunID("audit:sd:integrity")
		snapshotID := audit.SnapshotID("snap:sd:integrity")
		seed := "https://example.com/integrity"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		crawledAt := "2026-10-01T12:34:56Z"
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			Kind:      "html",
			CrawledAt: crawledAt,
			JSONLD:    []string{`{"@type": "Item", "name": "Test"}`},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', ?)`, runID, seed, string(pageData), crawledAt)

		res1, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build 1 failed: %v", err)
		}

		res2, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build 2 failed: %v", err)
		}

		if len(res1.StructuredDataBlocks) != len(res2.StructuredDataBlocks) {
			t.Fatalf("mismatched block lengths")
		}
		if res1.StructuredDataBlocks[0].StructuredBlockID != res2.StructuredDataBlocks[0].StructuredBlockID {
			t.Errorf("block ID not stable: %s vs %s", res1.StructuredDataBlocks[0].StructuredBlockID, res2.StructuredDataBlocks[0].StructuredBlockID)
		}

		// Verify grounded timestamp (not time.Now())
		expectedTime, _ := time.Parse(time.RFC3339, crawledAt)
		if !res1.StructuredDataBlocks[0].ObservedAt.Equal(expectedTime) {
			t.Errorf("expected ObservedAt %v, got %v", expectedTime, res1.StructuredDataBlocks[0].ObservedAt)
		}

		// Verify snapshot status
		if res1.EvidenceSnapshot.SnapshotStatus != audit.SnapshotFrozen {
			t.Errorf("expected snapshot to be FROZEN, got %s", res1.EvidenceSnapshot.SnapshotStatus)
		}
		if res1.EvidenceSnapshot.FrozenAt == nil {
			t.Errorf("expected non-nil FrozenAt")
		}
		if res1.EvidenceSnapshot.NormalizationVersion != "v1.8.0" {
			t.Errorf("expected NormalizationVersion v1.8.0, got %s", res1.EvidenceSnapshot.NormalizationVersion)
		}

		// Verify index construction succeeds without any error
		idx, err := engine.NewEvidenceIndex(res1.EvidenceSnapshot)
		if err != nil {
			t.Fatalf("NewEvidenceIndex failed: %v", err)
		}
		blockRefs := idx.SubjectRefs(audit.SubjectStructuredDataBlock)
		if len(blockRefs) != 1 {
			t.Errorf("expected 1 block ref in index, got %d", len(blockRefs))
		}
	})
}

// ============================================================================
// Group F: Regression Integration Tests (Section 13.F)
// ============================================================================

func TestAdapter_StructuredData_Regression(t *testing.T) {
	t.Run("Canonical, Link, and Engine rules remain green with Structured Data present", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:sd:regression"
		auditRunID := audit.AuditRunID("audit:sd:regression")
		snapshotID := audit.SnapshotID("snap:sd:regression")
		srcURL := "https://example.com/source"
		dstURL := "https://example.com/target"

		setupTestRun(t, db, runID, srcURL)
		insertURL(t, db, runID, 1, srcURL)
		insertURL(t, db, runID, 2, dstURL)

		// Insert source page with canonical and JSON-LD
		srcPage, _ := json.Marshal(sitecrawl.Page{
			URL:         srcURL,
			Status:      200,
			ContentType: "text/html",
			Kind:        "html",
			CrawledAt:   "2026-10-01T12:00:00Z",
			Canonical:   srcURL,
			Canonicals:  []string{srcURL},
			JSONLD:      []string{`{"@context": "https://schema.org", "@type": "WebPage", "name": "Source Page"}`},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, content_type, crawled_at, canonical)
			VALUES(?, 1, ?, ?, 200, 'html', 'text/html', '2026-10-01T12:00:00Z', ?)`, runID, srcURL, string(srcPage), srcURL)

		// Insert target page with 200 and Microdata
		dstPage, _ := json.Marshal(sitecrawl.Page{
			URL:         dstURL,
			Status:      200,
			ContentType: "text/html",
			Kind:        "html",
			CrawledAt:   "2026-10-01T12:00:05Z",
			Canonical:   dstURL,
			Canonicals:  []string{dstURL},
			SchemaOrg:   []sitecrawl.Schema{{Type: "https://schema.org/AboutPage", Properties: map[string]string{}}},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, content_type, crawled_at, canonical)
			VALUES(?, 2, ?, ?, 200, 'html', 'text/html', '2026-10-01T12:00:05Z', ?)`, runID, dstURL, string(dstPage), dstURL)

		// Insert link from 1 to 2
		insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Target Link")

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		// Verify 2 structured data blocks exist (1 JSON-LD, 1 Microdata)
		if len(res.StructuredDataBlocks) != 2 {
			t.Fatalf("expected 2 structured data blocks, got %d", len(res.StructuredDataBlocks))
		}

		// Verify Snapshot validation passes
		idx, err := engine.NewEvidenceIndex(res.EvidenceSnapshot)
		if err != nil {
			t.Fatalf("NewEvidenceIndex failed: %v", err)
		}

		// Run engine evaluator registry against snapshot
		eng, err := engine.New()
		if err != nil {
			t.Fatalf("engine.New failed: %v", err)
		}
		ruleIDs := eng.ImplementedRuleIDs()
		if len(ruleIDs) != 13 {
			t.Fatalf("expected exactly 13 implemented rules, got %d", len(ruleIDs))
		}

		for _, ruleID := range ruleIDs {
			results, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, ruleID)
			if err != nil {
				t.Fatalf("engine EvaluateRule for %s failed: %v", ruleID, err)
			}
			for _, r := range results {
				if r.RuleID == "AR-LINK-002" || r.RuleID == "AR-LINK-003" || r.RuleID == "AR-LINK-004" {
					if r.Status != audit.StatusPass {
						t.Errorf("rule %s expected PASS on 200 internal target, got %s", r.RuleID, r.Status)
					}
				}
				if r.RuleID == "AR-CANON-003" {
					if r.Status != audit.StatusPass {
						t.Errorf("rule AR-CANON-003 expected PASS on self-canonical, got %s", r.Status)
					}
				}
			}
		}

		// Ensure no index corruption
		if len(idx.SubjectRefs(audit.SubjectURL)) != 2 {
			t.Errorf("expected 2 URLs in index, got %d", len(idx.SubjectRefs(audit.SubjectURL)))
		}
		if len(idx.SubjectRefs(audit.SubjectLink)) != 1 {
			t.Errorf("expected 1 Link in index, got %d", len(idx.SubjectRefs(audit.SubjectLink)))
		}
		if len(idx.SubjectRefs(audit.SubjectStructuredDataBlock)) != 2 {
			t.Errorf("expected 2 StructuredDataBlocks in index, got %d", len(idx.SubjectRefs(audit.SubjectStructuredDataBlock)))
		}
	})
}
