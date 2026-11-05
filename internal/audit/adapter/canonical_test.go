package adapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
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

func TestAdapter_V19c_CanonicalSyntaxAndProvenanceContract(t *testing.T) {
	// 1. All 4 html_extraction_status states: complete, truncated_size_limit, read_error, missing_legacy
	t.Run("1. html_extraction_status states", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:v19c:extract:status"
		u1 := "https://example.com/complete"
		u2 := "https://example.com/truncated"
		u3 := "https://example.com/read-error"
		u4 := "https://example.com/legacy"
		setupTestRun(t, db, runID, u1)
		insertURL(t, db, runID, 1, u1)
		insertURL(t, db, runID, 2, u2)
		insertURL(t, db, runID, 3, u3)
		insertURL(t, db, runID, 4, u4)

		p1, _ := json.Marshal(sitecrawl.Page{
			URL:              u1,
			Status:           200,
			Kind:             sitecrawl.KindHTML,
			ExtractionStatus: sitecrawl.ExtractionStatusComplete,
			CrawledAt:        "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 200, 'text/html', '2026-10-01T00:00:10Z')`, runID, u1, string(p1))

		p2, _ := json.Marshal(sitecrawl.Page{
			URL:              u2,
			Status:           200,
			Kind:             sitecrawl.KindHTML,
			ExtractionStatus: sitecrawl.ExtractionStatusTruncatedSizeLimit,
			CrawledAt:        "2026-10-01T00:00:11Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
			VALUES(?, 2, ?, ?, 'html', 200, 'text/html', '2026-10-01T00:00:11Z')`, runID, u2, string(p2))

		p3, _ := json.Marshal(sitecrawl.Page{
			URL:              u3,
			Status:           200,
			Kind:             sitecrawl.KindHTML,
			ExtractionStatus: sitecrawl.ExtractionStatusReadError,
			CrawledAt:        "2026-10-01T00:00:12Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
			VALUES(?, 3, ?, ?, 'html', 200, 'text/html', '2026-10-01T00:00:12Z')`, runID, u3, string(p3))

		p4, _ := json.Marshal(sitecrawl.Page{
			URL:       u4,
			Status:    200,
			Kind:      sitecrawl.KindHTML,
			CrawledAt: "2026-10-01T00:00:13Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
			VALUES(?, 4, ?, ?, 'html', 200, 'text/html', '2026-10-01T00:00:13Z')`, runID, u4, string(p4))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:v19c:extract",
			SnapshotID: "snap:v19c:extract",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		if res.EvidenceSnapshot.NormalizationVersion != "v1.10.0" {
			t.Errorf("NormalizationVersion = %q, want v1.10.0", res.EvidenceSnapshot.NormalizationVersion)
		}

		statusBySubj := make(map[string]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.Field == "html_extraction_status" {
				statusBySubj[obs.SubjectRef] = obs.Value
			}
		}

		want := map[string]string{
			"url:audit:v19c:extract:1": "complete",
			"url:audit:v19c:extract:2": "truncated_size_limit",
			"url:audit:v19c:extract:3": "read_error",
			"url:audit:v19c:extract:4": "missing_legacy",
		}
		for subj, w := range want {
			if got := statusBySubj[subj]; got != w {
				t.Errorf("%s html_extraction_status = %q, want %q", subj, got, w)
			}
		}
	})

	// 2. Empty and whitespace canonical declarations + parser_error / resolution_error classification
	t.Run("2. single canonical syntax and resolution error classification", func(t *testing.T) {
		cases := []struct {
			name           string
			rawHref        string
			resolvedSlice  []string
			baseHrefRaw    string
			baseHrefCount  int
			wantStatus     string
			wantParseError string
			wantNormTarget string
			wantComplete   string
		}{
			{
				name:           "Scenario 9: empty href",
				rawHref:        "",
				resolvedSlice:  nil,
				wantStatus:     "parser_error",
				wantParseError: "parser_error:empty_href",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 10: whitespace-only href",
				rawHref:        "   \t  ",
				resolvedSlice:  nil,
				wantStatus:     "parser_error",
				wantParseError: "parser_error:empty_href",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 8a: malformed URL unclosed IPv6",
				rawHref:        "http://[::1",
				resolvedSlice:  []string{"http://[::1"},
				wantStatus:     "parser_error",
				wantParseError: "parser_error:malformed_url",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 8b: malformed URL bad port",
				rawHref:        "https://example.com:badport/page",
				resolvedSlice:  []string{"https://example.com:badport/page"},
				wantStatus:     "parser_error",
				wantParseError: "parser_error:malformed_url",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 8c: missing host http:///path",
				rawHref:        "http:///path",
				resolvedSlice:  []string{"http:///path"},
				wantStatus:     "resolution_error",
				wantParseError: "resolution_error:missing_host",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 7: unsupported scheme mailto",
				rawHref:        "mailto:seo@example.com",
				resolvedSlice:  []string{"mailto:seo@example.com"},
				wantStatus:     "resolution_error",
				wantParseError: "resolution_error:unsupported_scheme",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 5: relative canonical with valid HTTP(S) base href",
				rawHref:        "sub/page.html",
				resolvedSlice:  []string{"https://cdn.example.com/dir/sub/page.html"},
				baseHrefRaw:    "https://cdn.example.com/dir/",
				baseHrefCount:  1,
				wantStatus:     "valid",
				wantParseError: "",
				wantNormTarget: "https://cdn.example.com/dir/sub/page.html",
				wantComplete:   "true",
			},
			{
				name:           "Scenario 6a-i: relative canonical with valid ftp base href",
				rawHref:        "sub.html",
				resolvedSlice:  []string{"ftp://files.example.com/dir/sub.html"},
				baseHrefRaw:    "ftp://files.example.com/dir/",
				baseHrefCount:  1,
				wantStatus:     "resolution_error",
				wantParseError: "resolution_error:unsupported_scheme",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 6a-i: relative canonical with opaque mailto base href",
				rawHref:        "sub.html",
				resolvedSlice:  []string{"https://example.com/dir/sub.html"},
				baseHrefRaw:    "mailto:admin@example.com",
				baseHrefCount:  1,
				wantStatus:     "resolution_error",
				wantParseError: "resolution_error:non_hierarchical_base",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 6a-ii: relative canonical with malformed base href http://[::1",
				rawHref:        "sub.html",
				resolvedSlice:  []string{"https://example.com/dir/sub.html"},
				baseHrefRaw:    "http://[::1",
				baseHrefCount:  1,
				wantStatus:     "insufficient_evidence",
				wantParseError: "insufficient_evidence:ambiguous_base_fallback",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 6a-ii: relative canonical with empty-host base href http:///dir/",
				rawHref:        "sub.html",
				resolvedSlice:  []string{"https://example.com/dir/sub.html"},
				baseHrefRaw:    "http:///dir/",
				baseHrefCount:  1,
				wantStatus:     "insufficient_evidence",
				wantParseError: "insufficient_evidence:ambiguous_base_fallback",
				wantComplete:   "false",
			},
			{
				name:           "Scenario 6b-i: absolute HTTP(S) canonical ignores malformed base href",
				rawHref:        "https://example.com/canonical-target",
				resolvedSlice:  []string{"https://example.com/canonical-target"},
				baseHrefRaw:    "http://[::1",
				baseHrefCount:  1,
				wantStatus:     "valid",
				wantParseError: "",
				wantNormTarget: "https://example.com/canonical-target",
				wantComplete:   "true",
			},
			{
				name:           "Scenario 6b-ii: relative canonical with empty base href falls back to page URL",
				rawHref:        "sub.html",
				resolvedSlice:  []string{"https://example.com/dir/sub.html"},
				baseHrefRaw:    "",
				baseHrefCount:  1,
				wantStatus:     "valid",
				wantParseError: "",
				wantNormTarget: "https://example.com/dir/sub.html",
				wantComplete:   "true",
			},
			{
				name:           "Scenario 6b-ii: relative canonical with fragment-only base href falls back to page URL",
				rawHref:        "sub.html",
				resolvedSlice:  []string{"https://example.com/dir/sub.html"},
				baseHrefRaw:    "#section",
				baseHrefCount:  1,
				wantStatus:     "valid",
				wantParseError: "",
				wantNormTarget: "https://example.com/dir/sub.html",
				wantComplete:   "true",
			},
			{
				name:           "Scenario 6c: relative canonical with multiple conflicting base href declarations",
				rawHref:        "sub.html",
				resolvedSlice:  []string{"https://example.com/dir/sub.html"},
				baseHrefRaw:    "https://example.com/b1/",
				baseHrefCount:  2,
				wantStatus:     "insufficient_evidence",
				wantParseError: "insufficient_evidence:missing_base_context",
				wantComplete:   "false",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				db := newTestDB(t)
				runID := "run:v19c:case"
				pageURL := "https://example.com/dir/page.html"
				setupTestRun(t, db, runID, pageURL)
				insertURL(t, db, runID, 1, pageURL)

				pData, _ := json.Marshal(sitecrawl.Page{
					URL:                 pageURL,
					Status:              200,
					Kind:                sitecrawl.KindHTML,
					ExtractionStatus:    sitecrawl.ExtractionStatusComplete,
					CanonicalTotalCount: 1,
					RawCanonicals:       []string{tc.rawHref},
					Canonicals:          tc.resolvedSlice,
					BaseHrefRaw:         tc.baseHrefRaw,
					BaseHrefCount:       tc.baseHrefCount,
					CrawledAt:           "2026-10-01T00:00:10Z",
				})
				_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
					VALUES(?, 1, ?, ?, 'html', 200, 'text/html', '2026-10-01T00:00:10Z')`, runID, pageURL, string(pData))

				res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
					CrawlRunID: runID,
					AuditRunID: "audit:v19c:case",
					SnapshotID: "snap:v19c:case",
				})
				if err != nil {
					t.Fatalf("build failed: %v", err)
				}

				fields := make(map[string][]string)
				for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
					if obs.SubjectRef == "url:audit:v19c:case:1" {
						fields[obs.Field] = append(fields[obs.Field], obs.Value)
					}
				}

				if v := fields["canonical_count"]; len(v) != 1 || v[0] != "1" {
					t.Errorf("canonical_count = %v, want [1]", v)
				}
				if v := fields["canonical_raw"]; len(v) != 1 || v[0] != tc.rawHref {
					t.Errorf("canonical_raw = %v, want [%q]", v, tc.rawHref)
				}
				if v := fields["canonical_syntax_status"]; len(v) != 1 || v[0] != tc.wantStatus {
					t.Errorf("canonical_syntax_status = %v, want [%q]", v, tc.wantStatus)
				}
				if tc.wantParseError == "" {
					if v, ok := fields["canonical_parse_error"]; ok {
						t.Errorf("unexpected canonical_parse_error = %v", v)
					}
				} else {
					if v := fields["canonical_parse_error"]; len(v) != 1 || v[0] != tc.wantParseError {
						t.Errorf("canonical_parse_error = %v, want [%q]", v, tc.wantParseError)
					}
				}
				if tc.wantNormTarget == "" {
					if v, ok := fields["canonical_normalized_target"]; ok {
						t.Errorf("unexpected canonical_normalized_target = %v", v)
					}
				} else {
					if v := fields["canonical_normalized_target"]; len(v) != 1 || v[0] != tc.wantNormTarget {
						t.Errorf("canonical_normalized_target = %v, want [%q]", v, tc.wantNormTarget)
					}
				}
				if v := fields["canonical_normalization_complete"]; len(v) != 1 || v[0] != tc.wantComplete {
					t.Errorf("canonical_normalization_complete = %v, want [%q]", v, tc.wantComplete)
				}
			})
		}
	})

	// 3. > 5 canonical declarations (uncapped count, sample truncation, GapCanonicalSampleCapped, and AR-CANON-003/004/006/007 compatibility)
	t.Run("3. more than 5 canonical declarations and cross-rule compatibility", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:v19c:gt5"
		u1 := "https://example.com/six-identical-in-sample"
		u2 := "https://example.com/empty-canon-only"
		u3 := "https://example.com/two-with-one-empty"
		setupTestRun(t, db, runID, u1)
		insertURL(t, db, runID, 1, u1)
		insertURL(t, db, runID, 2, u2)
		insertURL(t, db, runID, 3, u3)

		// u1: 6 canonical tags, first 5 retained samples are identical
		p1, _ := json.Marshal(sitecrawl.Page{
			URL:                 u1,
			Status:              200,
			Kind:                sitecrawl.KindHTML,
			ExtractionStatus:    sitecrawl.ExtractionStatusComplete,
			CanonicalTotalCount: 6,
			RawCanonicals:       []string{u1, u1, u1, u1, u1},
			Canonicals:          []string{u1, u1, u1, u1, u1},
			CrawledAt:           "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
			VALUES(?, 1, ?, ?, 'html', 200, 'text/html', '2026-10-01T00:00:10Z')`, runID, u1, string(p1))

		// u2: 1 empty canonical tag (<link rel="canonical" href="">)
		p2, _ := json.Marshal(sitecrawl.Page{
			URL:                 u2,
			Status:              200,
			Kind:                sitecrawl.KindHTML,
			ExtractionStatus:    sitecrawl.ExtractionStatusComplete,
			CanonicalTotalCount: 1,
			RawCanonicals:       []string{""},
			Canonicals:          nil,
			CrawledAt:           "2026-10-01T00:00:11Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
			VALUES(?, 2, ?, ?, 'html', 200, 'text/html', '2026-10-01T00:00:11Z')`, runID, u2, string(p2))

		// u3: 2 canonical tags ([u3, ""])
		p3, _ := json.Marshal(sitecrawl.Page{
			URL:                 u3,
			Status:              200,
			Kind:                sitecrawl.KindHTML,
			ExtractionStatus:    sitecrawl.ExtractionStatusComplete,
			CanonicalTotalCount: 2,
			RawCanonicals:       []string{u3, ""},
			Canonicals:          []string{u3},
			CrawledAt:           "2026-10-01T00:00:12Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, crawled_at)
			VALUES(?, 3, ?, ?, 'html', 200, 'text/html', '2026-10-01T00:00:12Z')`, runID, u3, string(p3))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:v19c:gt5",
			SnapshotID: "snap:v19c:gt5",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		// Check u1 observations and GapCanonicalSampleCapped
		u1Fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:v19c:gt5:1" {
				u1Fields[obs.Field] = append(u1Fields[obs.Field], obs.Value)
			}
		}
		if v := u1Fields["canonical_count"]; len(v) != 1 || v[0] != "6" {
			t.Errorf("u1 canonical_count = %v, want [6]", v)
		}
		if v := u1Fields["canonical_sample_truncated"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("u1 canonical_sample_truncated = %v, want [true]", v)
		}
		if v := u1Fields["canonical_normalization_complete"]; len(v) != 1 || v[0] != "false" {
			t.Errorf("u1 canonical_normalization_complete = %v, want [false] when sample is truncated", v)
		}

		foundSampleGap := false
		for _, g := range res.EvidenceGaps {
			if g.GapCode == adapter.GapCanonicalSampleCapped && g.SubjectRef == "url:audit:v19c:gt5:1" {
				foundSampleGap = true
			}
			if g.GapCode == adapter.GapRawCanonicalUnavailable {
				t.Errorf("GapRawCanonicalUnavailable must not be emitted when all CanonicalObservations have RawValues")
			}
		}
		if !foundSampleGap {
			t.Errorf("expected GapCanonicalSampleCapped for u1 with 6 canonical tags")
		}

		// Evaluate existing canonical rules (AR-CANON-003, AR-CANON-004, AR-CANON-006, AR-CANON-007)
		eng, err := engine.New()
		if err != nil {
			t.Fatalf("engine.New failed: %v", err)
		}
		if gotImpl := len(eng.ImplementedRuleIDs()); gotImpl != 19 {
			t.Fatalf("expected executable rule count to remain 19 in V1.9c, got %d", gotImpl)
		}

		evalStatusMap := func(ruleID string) map[string]audit.RuleResultStatus {
			rrs, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, ruleID)
			if err != nil {
				t.Fatalf("EvaluateRule(%s) failed: %v", ruleID, err)
			}
			m := make(map[string]audit.RuleResultStatus)
			for _, r := range rrs {
				m[r.SubjectRef] = r.Status
			}
			return m
		}

		// AR-CANON-003: u1 -> PASS (has non-empty canonical_resolved), u2 -> WARNING (empty href has no canonical_resolved), u3 -> PASS
		c003 := evalStatusMap("AR-CANON-003")
		if c003["url:audit:v19c:gt5:1"] != audit.StatusPass {
			t.Errorf("AR-CANON-003 u1 = %s, want PASS", c003["url:audit:v19c:gt5:1"])
		}
		if c003["url:audit:v19c:gt5:2"] != audit.StatusWarning {
			t.Errorf("AR-CANON-003 u2 (empty href) = %s, want WARNING", c003["url:audit:v19c:gt5:2"])
		}
		if c003["url:audit:v19c:gt5:3"] != audit.StatusPass {
			t.Errorf("AR-CANON-003 u3 = %s, want PASS", c003["url:audit:v19c:gt5:3"])
		}

		// AR-CANON-004: u1 (6 tags, sample truncated) -> UNKNOWN; u2 (1 empty tag) -> PASS; u3 (2 tags, 1 empty) -> UNKNOWN
		c004 := evalStatusMap("AR-CANON-004")
		if c004["url:audit:v19c:gt5:1"] != audit.StatusUnknown {
			t.Errorf("AR-CANON-004 u1 (6 tags, sample truncated) = %s, want UNKNOWN", c004["url:audit:v19c:gt5:1"])
		}
		if c004["url:audit:v19c:gt5:2"] != audit.StatusPass {
			t.Errorf("AR-CANON-004 u2 (1 empty tag) = %s, want PASS", c004["url:audit:v19c:gt5:2"])
		}
		if c004["url:audit:v19c:gt5:3"] != audit.StatusUnknown {
			t.Errorf("AR-CANON-004 u3 (2 tags, 1 empty) = %s, want UNKNOWN", c004["url:audit:v19c:gt5:3"])
		}

		// AR-CANON-006: u1, u2, u3 all have canonical_normalization_complete=false -> UNKNOWN
		c006 := evalStatusMap("AR-CANON-006")
		for i := 1; i <= 3; i++ {
			subj := "url:audit:v19c:gt5:" + string(rune('0'+i))
			if c006[subj] != audit.StatusUnknown {
				t.Errorf("AR-CANON-006 %s = %s, want UNKNOWN", subj, c006[subj])
			}
		}

		// AR-CANON-007: u1, u2, u3 all have canonical_normalization_complete=false -> NOT_APPLICABLE
		c007 := evalStatusMap("AR-CANON-007")
		for i := 1; i <= 3; i++ {
			subj := "url:audit:v19c:gt5:" + string(rune('0'+i))
			if c007[subj] != audit.StatusNotApplicable {
				t.Errorf("AR-CANON-007 %s = %s, want NOT_APPLICABLE", subj, c007[subj])
			}
		}
	})

	// 4. Hermetic end-to-end SiteCrawl -> SQLite -> Adapter -> Engine pipeline test
	t.Run("4. hermetic end-to-end crawl to adapter canonical evidence", func(t *testing.T) {
		mux := http.NewServeMux()
		var srvURL string

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head>
<link rel="canonical" href="/valid-rel">
</head><body>
<a href="/valid-rel">Valid Rel</a>
<a href="/empty-href">Empty Href</a>
<a href="/six-tags">Six Tags</a>
<a href="/ftp-base">FTP Base</a>
<a href="/bad-base-abs-canon">Bad Base Abs Canon</a>
</body></html>`)
		})

		mux.HandleFunc("/valid-rel", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head><link rel="canonical" href="%s/valid-rel"></head><body>Target</body></html>`, srvURL)
		})

		mux.HandleFunc("/empty-href", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html><head><link rel="canonical" href="   "></head><body>Empty</body></html>`)
		})

		mux.HandleFunc("/six-tags", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html><head>
<link rel="canonical" href="/valid-rel">
<link rel="canonical" href="/valid-rel">
<link rel="canonical" href="/valid-rel">
<link rel="canonical" href="/valid-rel">
<link rel="canonical" href="/valid-rel">
<link rel="canonical" href="/valid-rel">
</head><body>Six</body></html>`)
		})

		mux.HandleFunc("/ftp-base", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, `<!doctype html><html><head>
<base href="ftp://files.example.com/dir/">
<link rel="canonical" href="sub.html">
</head><body>FTP Base</body></html>`)
		})

		mux.HandleFunc("/bad-base-abs-canon", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head>
<base href="http://[::1">
<link rel="canonical" href="%s/valid-rel">
</head><body>Abs Canon with Bad Base</body></html>`, srvURL)
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()
		srvURL = srv.URL

		db, err := standalone.OpenDB(":memory:")
		if err != nil {
			t.Fatalf("OpenDB: %v", err)
		}
		defer db.Close()

		runner := sitecrawl.NewRunner(db)
		if err := runner.EnsureSchema(); err != nil {
			t.Fatalf("EnsureSchema: %v", err)
		}

		summary, err := runner.Crawl(context.Background(), []string{srv.URL + "/"}, sitecrawl.Options{
			Mode:             sitecrawl.ModeSpider,
			MaxDepth:         2,
			MaxURLs:          20,
			Concurrency:      1,
			RespectRobots:    false,
			DiscoverSitemaps: false,
		})
		if err != nil {
			t.Fatalf("Crawl: %v", err)
		}

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: summary.ID,
			AuditRunID: "audit:hermetic:v19c",
			SnapshotID: "snap:hermetic:v19c",
		})
		if err != nil {
			t.Fatalf("adapter.Build: %v", err)
		}

		obsByURL := make(map[string]map[string][]string)
		subjToURL := make(map[string]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectType == audit.SubjectURL && obs.Field == "url_identity" {
				subjToURL[obs.SubjectRef] = obs.Value
				if obsByURL[obs.Value] == nil {
					obsByURL[obs.Value] = make(map[string][]string)
				}
			}
		}
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if u, ok := subjToURL[obs.SubjectRef]; ok {
				obsByURL[u][obs.Field] = append(obsByURL[u][obs.Field], obs.Value)
			}
		}

		// Verify /empty-href
		emptyObs := obsByURL[srv.URL+"/empty-href"]
		if v := emptyObs["html_extraction_status"]; len(v) != 1 || v[0] != "complete" {
			t.Errorf("/empty-href html_extraction_status = %v, want [complete]", v)
		}
		if v := emptyObs["canonical_count"]; len(v) != 1 || v[0] != "1" {
			t.Errorf("/empty-href canonical_count = %v, want [1]", v)
		}
		if v := emptyObs["canonical_raw"]; len(v) != 1 || v[0] != "   " {
			t.Errorf("/empty-href canonical_raw = %v, want ['   ']", v)
		}
		if v := emptyObs["canonical_syntax_status"]; len(v) != 1 || v[0] != "parser_error" {
			t.Errorf("/empty-href canonical_syntax_status = %v, want [parser_error]", v)
		}
		if v := emptyObs["canonical_parse_error"]; len(v) != 1 || v[0] != "parser_error:empty_href" {
			t.Errorf("/empty-href canonical_parse_error = %v, want [parser_error:empty_href]", v)
		}

		// Verify /six-tags
		sixObs := obsByURL[srv.URL+"/six-tags"]
		if v := sixObs["canonical_count"]; len(v) != 1 || v[0] != "6" {
			t.Errorf("/six-tags canonical_count = %v, want [6]", v)
		}
		if v := sixObs["canonical_sample_truncated"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("/six-tags canonical_sample_truncated = %v, want [true]", v)
		}
		if v := sixObs["canonical_normalization_complete"]; len(v) != 1 || v[0] != "false" {
			t.Errorf("/six-tags canonical_normalization_complete = %v, want [false]", v)
		}

		// Verify /ftp-base (Scenario 6a-i)
		ftpObs := obsByURL[srv.URL+"/ftp-base"]
		if v := ftpObs["canonical_syntax_status"]; len(v) != 1 || v[0] != "resolution_error" {
			t.Errorf("/ftp-base canonical_syntax_status = %v, want [resolution_error]", v)
		}
		if v := ftpObs["canonical_parse_error"]; len(v) != 1 || v[0] != "resolution_error:unsupported_scheme" {
			t.Errorf("/ftp-base canonical_parse_error = %v, want [resolution_error:unsupported_scheme]", v)
		}

		// Verify /bad-base-abs-canon (Scenario 6b-i)
		absObs := obsByURL[srv.URL+"/bad-base-abs-canon"]
		if v := absObs["canonical_syntax_status"]; len(v) != 1 || v[0] != "valid" {
			t.Errorf("/bad-base-abs-canon canonical_syntax_status = %v, want [valid]", v)
		}
		if v := absObs["canonical_normalization_complete"]; len(v) != 1 || v[0] != "true" {
			t.Errorf("/bad-base-abs-canon canonical_normalization_complete = %v, want [true]", v)
		}
	})
}
