package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func TestEvaluateRENDER002_Preconditions(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()

	// 1. Missing URL identity
	snapNoURL := newRenderSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
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
	})
	results, err := eng.EvaluateRule(context.Background(), snapNoURL, "AR-RENDER-002")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusUnknown {
		t.Errorf("expected UNKNOWN for missing URL identity, got %v", results)
	}

	// 2. Not selected for rendering -> NOT_APPLICABLE
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
	results, err = eng.EvaluateRule(context.Background(), snapNotSelected, "AR-RENDER-002")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusNotApplicable {
		t.Errorf("expected NOT_APPLICABLE when page not selected for rendering, got %v", results)
	}

	// 3. Missing raw/rendered evidence -> UNKNOWN
	snapMissing := newRenderSnapshot([]audit.NormalizedObservation{
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
	})
	results, err = eng.EvaluateRule(context.Background(), snapMissing, "AR-RENDER-002")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusUnknown {
		t.Errorf("expected UNKNOWN when canonical raw/rendered evidence is missing, got %v", results)
	}
}

func TestEvaluateRENDER002_PassAndWarning(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()

	// 1. Identical canonicals -> PASS
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
			Field:              "canonical_raw",
			Value:              "https://example.com/canonical",
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
			Field:              "canonical_rendered",
			Value:              "https://example.com/canonical",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err := eng.EvaluateRule(context.Background(), snapPass, "AR-RENDER-002")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusPass {
		t.Errorf("expected PASS when canonicals match, got %v", results)
	}

	// 2. Differing canonicals -> WARNING
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
			Field:              "canonical_raw",
			Value:              "https://example.com/page",
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
			Field:              "canonical_rendered",
			Value:              "https://example.com/other",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})
	results, err = eng.EvaluateRule(context.Background(), snapWarn, "AR-RENDER-002")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusWarning {
		t.Errorf("expected WARNING when canonicals differ, got %v", results)
	}
}
