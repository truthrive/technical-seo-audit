package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func TestEvaluateRENDER003_Preconditions(t *testing.T) {
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
	results, err := eng.EvaluateRule(context.Background(), snapNotSelected, "AR-RENDER-003")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusNotApplicable {
		t.Errorf("expected NOT_APPLICABLE when page not selected for rendering, got %v", results)
	}
}

func TestEvaluateRENDER003_PassAndWarning(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()

	// 1. Equivalent tokens in different order/casing -> PASS
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
			Field:              "meta_robots_raw",
			Value:              "index, follow",
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
			Field:              "meta_robots_rendered",
			Value:              "FOLLOW, INDEX",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err := eng.EvaluateRule(context.Background(), snapPass, "AR-RENDER-003")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusPass {
		t.Errorf("expected PASS when directives match, got %v", results)
	}

	// 2. Differing directives -> WARNING
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
			Field:              "meta_robots_raw",
			Value:              "noindex, nofollow",
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
			Field:              "meta_robots_rendered",
			Value:              "index, follow",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err = eng.EvaluateRule(context.Background(), snapWarn, "AR-RENDER-003")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusWarning {
		t.Errorf("expected WARNING when directives differ, got %v", results)
	}
}
