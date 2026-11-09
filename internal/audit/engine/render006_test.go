package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func TestEvaluateRENDER006_Preconditions(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()

	// Not selected for rendering -> NOT_APPLICABLE
	snapNotSelected := newRenderSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/page",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err := eng.EvaluateRule(context.Background(), snapNotSelected, "AR-RENDER-006")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusNotApplicable {
		t.Errorf("expected NOT_APPLICABLE when page not selected for rendering, got %v", results)
	}
}

func TestEvaluateRENDER006_PassAndWarning(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()

	// 1. Identical link sets -> PASS
	snapPass := newRenderSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/page",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:2",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "render_requested",
			Value:              "true",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:3",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "internal_links_raw",
			Value:              "https://example.com/a, https://example.com/b",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:4",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "internal_links_rendered",
			Value:              "https://example.com/b, https://example.com/a",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err := eng.EvaluateRule(context.Background(), snapPass, "AR-RENDER-006")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusPass {
		t.Errorf("expected PASS when link sets match, got %v", results)
	}

	// 2. Differing link sets (added/removed) -> WARNING
	snapWarn := newRenderSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/page",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:2",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "render_requested",
			Value:              "true",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:3",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "internal_links_raw",
			Value:              "https://example.com/static-link",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:4",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "internal_links_rendered",
			Value:              "https://example.com/static-link, https://example.com/dynamic-link",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err = eng.EvaluateRule(context.Background(), snapWarn, "AR-RENDER-006")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusWarning {
		t.Errorf("expected WARNING when link sets differ, got %v", results)
	}

	// 3. Relative link resolution and fragment normalization -> PASS
	snapRelLinks := newRenderSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/dir/page",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:2",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "render_requested",
			Value:              "true",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:3",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "internal_links_raw",
			Value:              "/about, sub",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:4",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "internal_links_rendered",
			Value:              "https://example.com/dir/sub#top, https://EXAMPLE.com:443/about",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err = eng.EvaluateRule(context.Background(), snapRelLinks, "AR-RENDER-006")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusPass {
		t.Errorf("expected PASS when relative internal links resolve to identical URLs, got %v", results)
	}

	// 4. Case-sensitive link path difference (/About vs /about) -> WARNING
	snapCaseDiff := newRenderSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/page",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:2",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "render_requested",
			Value:              "true",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:3",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "internal_links_raw",
			Value:              "https://example.com/About",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:4",
			AuditRunID:         "audit:render:run",
			SnapshotID:         "snap:render:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "internal_links_rendered",
			Value:              "https://example.com/about",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err = eng.EvaluateRule(context.Background(), snapCaseDiff, "AR-RENDER-006")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusWarning {
		t.Errorf("expected WARNING when internal link paths differ by case, got %v", results)
	}
}
