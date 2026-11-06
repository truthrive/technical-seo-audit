package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func TestEvaluateRENDER005_Preconditions(t *testing.T) {
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
	results, err := eng.EvaluateRule(context.Background(), snapNotSelected, "AR-RENDER-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusNotApplicable {
		t.Errorf("expected NOT_APPLICABLE when page not selected for rendering, got %v", results)
	}
}

func TestEvaluateRENDER005_PassAndWarning(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()

	// 1. Both true -> PASS
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
			Field:              "main_text_present_raw",
			Value:              "true",
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
			Field:              "main_text_present_rendered",
			Value:              "true",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err := eng.EvaluateRule(context.Background(), snapPass, "AR-RENDER-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusPass {
		t.Errorf("expected PASS when main-text presence matches, got %v", results)
	}

	// 2. Differing states -> WARNING
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
			Field:              "main_text_present_raw",
			Value:              "false",
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
			Field:              "main_text_present_rendered",
			Value:              "true",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err = eng.EvaluateRule(context.Background(), snapWarn, "AR-RENDER-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusWarning {
		t.Errorf("expected WARNING when main-text presence differs, got %v", results)
	}
}
