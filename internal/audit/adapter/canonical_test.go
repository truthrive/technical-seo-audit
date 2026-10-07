package adapter_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// Tests covering all V1.4b Canonical Evidence Enablement requirements:
// 1. no canonical -> count 0
// 2. one absolute canonical
// 3. one relative canonical becomes normalized
// 4. duplicate declarations same target
// 5. multiple declarations different targets
// 6. fragment/default-port/case normalization
// 7. invalid/non-HTTP canonical -> completeness false
// 8. complete set -> distinct count emitted
// 9. incomplete set -> distinct count absent
// 10. one target maps to URL subject
// 11. target not in snapshot -> subject ref absent
// 12. ambiguous normalized URL identity -> subject ref absent
// 13. rendered page does not manufacture canonical_count=0
// 14. deterministic observation ordering
// 15. existing CANON-003 integration remains green
// 16. existing INDEX rules remain green
func TestAdapter_CanonicalEvidence(t *testing.T) {
	// 1. no canonical -> count 0
	t.Run("1. no canonical -> count 0", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:none"
		seed := "https://example.com/"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, crawled_at)
			VALUES(?, 1, ?, ?, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:none",
			SnapshotID: "snap:run:canon:none",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:none:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if countVals, ok := fields["canonical_count"]; !ok || len(countVals) != 1 || countVals[0] != "0" {
			t.Errorf("expected canonical_count = '0', got %v", countVals)
		}
		if _, ok := fields["canonical_resolved"]; ok {
			t.Errorf("canonical_resolved must not be emitted when no canonical exists")
		}
		if _, ok := fields["canonical_normalized_target"]; ok {
			t.Errorf("canonical_normalized_target must not be emitted when no canonical exists")
		}
		if _, ok := fields["canonical_normalization_complete"]; ok {
			t.Errorf("canonical_normalization_complete must not be emitted when no canonical exists")
		}
		if _, ok := fields["canonical_distinct_normalized_count"]; ok {
			t.Errorf("canonical_distinct_normalized_count must not be emitted when no canonical exists")
		}
		if _, ok := fields["canonical_target_subject_ref"]; ok {
			t.Errorf("canonical_target_subject_ref must not be emitted when no canonical exists")
		}
	})

	// 2. one absolute canonical
	t.Run("2. one absolute canonical", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:one:abs"
		seed := "https://example.com/source"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Canonicals: []string{"https://example.com/target"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 'https://example.com/target', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:one:abs",
			SnapshotID: "snap:run:canon:one:abs",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:one:abs:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if v := fields["canonical_count"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("expected canonical_count = '1', got %v", v)
		}
		if v := fields["canonical_resolved"]; len(v) != 1 || v[0] != "https://example.com/target" {
			t.Errorf("expected canonical_resolved = 'https://example.com/target', got %v", v)
		}
		if v := fields["canonical_normalized_target"]; len(v) != 1 || v[0] != "https://example.com/target" {
			t.Errorf("expected canonical_normalized_target = 'https://example.com/target', got %v", v)
		}
		if v := fields["canonical_normalization_complete"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected canonical_normalization_complete = 'true', got %v", v)
		}
		if v := fields["canonical_distinct_normalized_count"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("expected canonical_distinct_normalized_count = '1', got %v", v)
		}
	})

	// 3. one relative canonical becomes normalized
	t.Run("3. one relative canonical becomes normalized", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:one:rel"
		seed := "https://example.com/dir/page"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Canonicals: []string{"/target"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', '/target', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:one:rel",
			SnapshotID: "snap:run:canon:one:rel",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:one:rel:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if v := fields["canonical_count"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("expected canonical_count = '1', got %v", v)
		}
		if v := fields["canonical_resolved"]; len(v) != 1 || v[0] != "/target" {
			t.Errorf("expected canonical_resolved = '/target', got %v", v)
		}
		if v := fields["canonical_normalized_target"]; len(v) != 1 || v[0] != "https://example.com/target" {
			t.Errorf("expected canonical_normalized_target = 'https://example.com/target', got %v", v)
		}
		if v := fields["canonical_normalization_complete"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected canonical_normalization_complete = 'true', got %v", v)
		}
		if v := fields["canonical_distinct_normalized_count"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("expected canonical_distinct_normalized_count = '1', got %v", v)
		}
	})

	// 4. duplicate declarations same target
	t.Run("4. duplicate declarations same target", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:dup"
		seed := "https://example.com/page"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Canonicals: []string{"https://example.com/target", "https://example.com/target"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 'https://example.com/target', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:dup",
			SnapshotID: "snap:run:canon:dup",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:dup:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if v := fields["canonical_count"]; len(v) != 1 || v[0] != "2" {
			t.Errorf("expected canonical_count = '2', got %v", v)
		}
		if v := fields["canonical_resolved"]; len(v) != 2 {
			t.Errorf("expected 2 canonical_resolved observations, got %v", v)
		}
		if v := fields["canonical_normalized_target"]; len(v) != 2 || v[0] != "https://example.com/target" || v[1] != "https://example.com/target" {
			t.Errorf("expected 2 canonical_normalized_target observations, got %v", v)
		}
		if v := fields["canonical_normalization_complete"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected canonical_normalization_complete = 'true', got %v", v)
		}
		if v := fields["canonical_distinct_normalized_count"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("expected distinct count = '1' for duplicate declarations, got %v", v)
		}
	})

	// 5. multiple declarations different targets
	t.Run("5. multiple declarations different targets", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:multi"
		seed := "https://example.com/page"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Canonicals: []string{"https://example.com/target1", "https://example.com/target2"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 'https://example.com/target1', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:multi",
			SnapshotID: "snap:run:canon:multi",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:multi:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if v := fields["canonical_count"]; len(v) != 1 || v[0] != "2" {
			t.Errorf("expected canonical_count = '2', got %v", v)
		}
		if v := fields["canonical_normalization_complete"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("expected canonical_normalization_complete = 'true', got %v", v)
		}
		if v := fields["canonical_distinct_normalized_count"]; len(v) != 1 || v[0] != "2" {
			t.Errorf("expected distinct count = '2', got %v", v)
		}
		if _, ok := fields["canonical_target_subject_ref"]; ok {
			t.Errorf("canonical_target_subject_ref must NOT be emitted when distinct count > 1")
		}
	})

	// 6. fragment/default-port/case normalization
	t.Run("6. fragment/default-port/case normalization", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:norm"
		seed := "https://example.com/page"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Canonicals: []string{"HTTPS://EXAMPLE.COM:443/target#section"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 'HTTPS://EXAMPLE.COM:443/target#section', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:norm",
			SnapshotID: "snap:run:canon:norm",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:norm:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if v := fields["canonical_normalized_target"]; len(v) != 1 || v[0] != "https://example.com/target" {
			t.Errorf("expected normalized target 'https://example.com/target', got %v", v)
		}
		if v := fields["canonical_distinct_normalized_count"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("expected distinct count '1', got %v", v)
		}
	})

	// 7. invalid/non-HTTP canonical -> completeness false
	// 8. complete set -> distinct count emitted
	// 9. incomplete set -> distinct count absent
	t.Run("7,8,9. invalid/non-HTTP canonical -> completeness false and distinct count absent", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:invalid"
		seed := "https://example.com/page"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Canonicals: []string{"https://example.com/target", "mailto:someone@example.com"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 'https://example.com/target', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:invalid",
			SnapshotID: "snap:run:canon:invalid",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:invalid:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if v := fields["canonical_count"]; len(v) != 1 || v[0] != "2" {
			t.Errorf("expected canonical_count = '2', got %v", v)
		}
		if v := fields["canonical_normalized_target"]; len(v) != 1 || v[0] != "https://example.com/target" {
			t.Errorf("expected only 1 valid canonical_normalized_target, got %v", v)
		}
		if v := fields["canonical_normalization_complete"]; len(v) != 1 || v[0] != "false" {
			t.Errorf("expected canonical_normalization_complete = 'false', got %v", v)
		}
		if _, ok := fields["canonical_distinct_normalized_count"]; ok {
			t.Errorf("canonical_distinct_normalized_count must be absent when normalization is incomplete")
		}
		if _, ok := fields["canonical_target_subject_ref"]; ok {
			t.Errorf("canonical_target_subject_ref must be absent when normalization is incomplete")
		}
	})

	// 10. one target maps to URL subject
	t.Run("10. one target maps to URL subject", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:correlate"
		sourceURL := "https://example.com/source"
		targetURL := "https://example.com/target"
		setupTestRun(t, db, runID, sourceURL)
		insertURL(t, db, runID, 1, sourceURL)
		insertURL(t, db, runID, 2, targetURL)

		sourceData, _ := json.Marshal(sitecrawl.Page{
			URL:        sourceURL,
			Canonicals: []string{targetURL},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', ?, '2026-10-01T00:00:10Z')`, runID, sourceURL, string(sourceData), targetURL)

		targetData, _ := json.Marshal(sitecrawl.Page{
			URL:       targetURL,
			CrawledAt: "2026-10-01T00:00:15Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, crawled_at)
			VALUES(?, 2, ?, ?, 'html', '2026-10-01T00:00:15Z')`, runID, targetURL, string(targetData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:correlate",
			SnapshotID: "snap:run:canon:correlate",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:correlate:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		targetRef := fields["canonical_target_subject_ref"]
		if len(targetRef) != 1 || targetRef[0] != "url:audit:run:canon:correlate:2" {
			t.Errorf("expected canonical_target_subject_ref = 'url:audit:run:canon:correlate:2', got %v", targetRef)
		}
	})

	// 11. target not in snapshot -> subject ref absent
	t.Run("11. target not in snapshot -> subject ref absent", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:external"
		sourceURL := "https://example.com/source"
		setupTestRun(t, db, runID, sourceURL)
		insertURL(t, db, runID, 1, sourceURL)

		sourceData, _ := json.Marshal(sitecrawl.Page{
			URL:        sourceURL,
			Canonicals: []string{"https://other.com/external"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 'https://other.com/external', '2026-10-01T00:00:10Z')`, runID, sourceURL, string(sourceData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:external",
			SnapshotID: "snap:run:canon:external",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:external:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if _, ok := fields["canonical_target_subject_ref"]; ok {
			t.Errorf("canonical_target_subject_ref must be absent when target is not in snapshot")
		}
		if v := fields["canonical_distinct_normalized_count"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("expected distinct count = '1', got %v", v)
		}
	})

	// 12. ambiguous normalized URL identity -> subject ref absent
	t.Run("12. ambiguous normalized URL identity -> subject ref absent", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:ambig"
		sourceURL := "https://example.com/source"
		targetURL1 := "https://example.com/target"
		targetURL2 := "https://example.com/target#fragment"
		setupTestRun(t, db, runID, sourceURL)
		insertURL(t, db, runID, 1, sourceURL)
		insertURL(t, db, runID, 2, targetURL1)
		insertURL(t, db, runID, 3, targetURL2)

		sourceData, _ := json.Marshal(sitecrawl.Page{
			URL:        sourceURL,
			Canonicals: []string{targetURL1},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', ?, '2026-10-01T00:00:10Z')`, runID, sourceURL, string(sourceData), targetURL1)

		page2Data, _ := json.Marshal(sitecrawl.Page{URL: targetURL1, CrawledAt: "2026-10-01T00:00:15Z"})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, crawled_at)
			VALUES(?, 2, ?, ?, 'html', '2026-10-01T00:00:15Z')`, runID, targetURL1, string(page2Data))

		page3Data, _ := json.Marshal(sitecrawl.Page{URL: targetURL2, CrawledAt: "2026-10-01T00:00:20Z"})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, crawled_at)
			VALUES(?, 3, ?, ?, 'html', '2026-10-01T00:00:20Z')`, runID, targetURL2, string(page3Data))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:ambig",
			SnapshotID: "snap:run:canon:ambig",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:ambig:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if _, ok := fields["canonical_target_subject_ref"]; ok {
			t.Errorf("canonical_target_subject_ref must be withheld when multiple URL resources match the normalized target")
		}
	})

	// 13. rendered page does not manufacture canonical_count=0
	t.Run("13. rendered page does not manufacture canonical_count=0", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:rendered"
		seed := "https://example.com/rendered"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Rendered:  true,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, rendered, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 1, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:rendered",
			SnapshotID: "snap:run:canon:rendered",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:canon:rendered:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if _, ok := fields["canonical_count"]; ok {
			t.Errorf("rendered page must NOT emit canonical_count when head extraction completeness is unverified")
		}
	})

	// 14. deterministic observation ordering
	t.Run("14. deterministic observation ordering", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:order"
		seed := "https://example.com/source"
		target := "https://example.com/target"
		setupTestRun(t, db, runID, seed)
		insertURL(t, db, runID, 1, seed)
		insertURL(t, db, runID, 2, target)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Canonicals: []string{target},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
			VALUES(?, 1, ?, ?, 'html', ?, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData), target)

		targetData, _ := json.Marshal(sitecrawl.Page{URL: target, CrawledAt: "2026-10-01T00:00:15Z"})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, crawled_at)
			VALUES(?, 2, ?, ?, 'html', '2026-10-01T00:00:15Z')`, runID, target, string(targetData))

		res1, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:order",
			SnapshotID: "snap:run:canon:order",
		})
		if err != nil {
			t.Fatalf("build 1 failed: %v", err)
		}

		res2, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:order",
			SnapshotID: "snap:run:canon:order",
		})
		if err != nil {
			t.Fatalf("build 2 failed: %v", err)
		}

		if len(res1.EvidenceSnapshot.NormalizedObservations) != len(res2.EvidenceSnapshot.NormalizedObservations) {
			t.Fatalf("length mismatch: %d != %d", len(res1.EvidenceSnapshot.NormalizedObservations), len(res2.EvidenceSnapshot.NormalizedObservations))
		}

		for i := range res1.EvidenceSnapshot.NormalizedObservations {
			o1 := res1.EvidenceSnapshot.NormalizedObservations[i]
			o2 := res2.EvidenceSnapshot.NormalizedObservations[i]
			if o1.Field != o2.Field || o1.Value != o2.Value || o1.SubjectRef != o2.SubjectRef {
				t.Fatalf("order/value mismatch at %d: %+v != %+v", i, o1, o2)
			}
		}
	})

	// 15. existing CANON-003 integration remains green
	t.Run("15. existing CANON-003 integration remains green", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:engine:canon003"
		url1 := "https://example.com/has-canon"
		url2 := "https://example.com/no-canon"
		setupTestRun(t, db, runID, url1)
		insertURL(t, db, runID, 1, url1)
		insertURL(t, db, runID, 2, url2)

		p1, _ := json.Marshal(sitecrawl.Page{
			URL:        url1,
			Canonicals: []string{url1},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, status, content_type, crawled_at)
			VALUES(?, 1, ?, ?, 'html', ?, 200, 'text/html; charset=utf-8', '2026-10-01T00:00:10Z')`, runID, url1, string(p1), url1)

		p2, _ := json.Marshal(sitecrawl.Page{
			URL:       url2,
			CrawledAt: "2026-10-01T00:00:15Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
			VALUES(?, 2, ?, ?, 'html', 200, 'text/html; charset=utf-8', '2026-10-01T00:00:15Z')`, runID, url2, string(p2))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:engine:canon003",
			SnapshotID: "snap:run:canon:engine:canon003",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		eng, err := engine.New()
		if err != nil {
			t.Fatalf("engine.New failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, "AR-CANON-003")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-003 failed: %v", err)
		}

		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		statusBySubj := make(map[string]audit.RuleResultStatus)
		for _, r := range results {
			statusBySubj[r.SubjectRef] = r.Status
		}

		if s := statusBySubj["url:audit:run:canon:engine:canon003:1"]; s != audit.StatusPass {
			t.Errorf("expected PASS for url 1 with canonical, got %s", s)
		}
		if s := statusBySubj["url:audit:run:canon:engine:canon003:2"]; s != audit.StatusWarning {
			t.Errorf("expected WARNING for url 2 without canonical, got %s", s)
		}
	})

	// 16. existing INDEX rules remain green
	t.Run("16. existing INDEX rules remain green", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:canon:engine:index"
		url1 := "https://example.com/noindex"
		setupTestRun(t, db, runID, url1)
		insertURL(t, db, runID, 1, url1)

		p1, _ := json.Marshal(sitecrawl.Page{
			URL:        url1,
			MetaRobots: "noindex",
			MetaTags: map[string]string{
				"robots": "noindex",
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, status, content_type, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 'noindex', 200, 'text/html', '2026-10-01T00:00:10Z')`, runID, url1, string(p1))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:canon:engine:index",
			SnapshotID: "snap:run:canon:engine:index",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		eng, err := engine.New()
		if err != nil {
			t.Fatalf("engine.New failed: %v", err)
		}

		// AR-INDEX-002
		results002, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("AR-INDEX-002 failed: %v", err)
		}
		if len(results002) != 1 || results002[0].Status != audit.StatusPass {
			t.Errorf("expected PASS for single noindex in AR-INDEX-002, got %+v", results002)
		}

		// AR-INDEX-001 with policy expected_indexable = true
		polIdx, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:exp:1",
				AuditRunID:         "audit:run:canon:engine:index",
				PolicyKey:          audit.PolicyKeyExpectedIndexable,
				PolicyValue:        "true",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          "url:audit:run:canon:engine:index:1",
				Source:             audit.PolicyProvenanceUserInput,
			},
		})
		if err != nil {
			t.Fatalf("NewPolicyIndex failed: %v", err)
		}

		results001, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{
			Snapshot: res.EvidenceSnapshot,
			Policies: polIdx,
		}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("AR-INDEX-001 failed: %v", err)
		}
		if len(results001) != 1 || results001[0].Status != audit.StatusFail {
			t.Errorf("expected FAIL for AR-INDEX-001 when effective_noindex conflicts with expected_indexable, got %+v", results001)
		}
	})
}
