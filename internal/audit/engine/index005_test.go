package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnippetSnapshot(obs []audit.NormalizedObservation) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:snippet:test",
		AuditRunID:             "audit:snippet:run",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestEvaluateINDEX005_Preconditions(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()

	// 1. Missing URL identity
	snapNoURL := newSnippetSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "nosnippet",
			Value:              "false",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})

	results, err := eng.EvaluateRule(context.Background(), snapNoURL, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusUnknown {
		t.Errorf("expected UNKNOWN for missing URL identity, got %v", results)
	}

	// 2. Conflicting URL identity
	snapConflictURL := newSnippetSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/a",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:2",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/b",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})

	results, err = eng.EvaluateRule(context.Background(), snapConflictURL, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusUnknown {
		t.Errorf("expected UNKNOWN for conflicting URL identity, got %v", results)
	}

	// 3. Missing snippet directives
	snapNoDirectives := newSnippetSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/page",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})

	results, err = eng.EvaluateRule(context.Background(), snapNoDirectives, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusUnknown {
		t.Errorf("expected UNKNOWN for missing snippet directives, got %v", results)
	}
}

func TestEvaluateINDEX005_NoRestriction(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()
	snap := newSnippetSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
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
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "nosnippet",
			Value:              "false",
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:3",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "data_nosnippet_count",
			Value:              "0",
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})

	// Case A: Unspecified policy
	results, err := eng.EvaluateRule(context.Background(), snap, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusPass {
		t.Errorf("expected PASS for no restriction with unspecified policy, got %v", results)
	}

	// Case B: Explicit allow_unrestricted policy
	policies, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
		{
			PolicyAssignmentID: "pol:1",
			AuditRunID:         "audit:snippet:run",
			PolicyKey:          audit.PolicyKeySnippetPolicy,
			PolicyValue:        audit.PolicyValueSnippetAllowUnrestricted,
			Scope:              audit.PolicyScopeSite,
			TargetRef:          "site",
			Source:             audit.PolicyProvenanceUserInput,
			SuppliedAt:         now,
		},
	})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed: %v", err)
	}

	evalCtx := engine.EvaluationContext{Snapshot: snap, Policies: policies}
	results, err = eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRuleWithContext failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusPass {
		t.Errorf("expected PASS for no restriction with allow_unrestricted policy, got %v", results)
	}
}

func TestEvaluateINDEX005_WithRestrictions(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	now := time.Now().UTC()

	// Base snapshot with nosnippet=true
	baseObs := []audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
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
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "nosnippet",
			Value:              "true",
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	}
	snap := newSnippetSnapshot(baseObs)

	// 1. Unspecified policy -> WARNING
	results, err := eng.EvaluateRule(context.Background(), snap, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusWarning {
		t.Errorf("expected WARNING for nosnippet with unspecified policy, got %v", results)
	}

	// 2. Policy allow_unrestricted -> FAIL
	polAllow, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
		{
			PolicyAssignmentID: "pol:1",
			AuditRunID:         "audit:snippet:run",
			PolicyKey:          audit.PolicyKeySnippetPolicy,
			PolicyValue:        audit.PolicyValueSnippetAllowUnrestricted,
			Scope:              audit.PolicyScopeSite,
			TargetRef:          "site",
			Source:             audit.PolicyProvenanceUserInput,
			SuppliedAt:         now,
		},
	})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed: %v", err)
	}
	results, err = eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polAllow}, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRuleWithContext failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusFail {
		t.Errorf("expected FAIL for nosnippet with allow_unrestricted policy, got %v", results)
	}

	// 3. Policy restrict -> PASS
	polRestrict, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
		{
			PolicyAssignmentID: "pol:2",
			AuditRunID:         "audit:snippet:run",
			PolicyKey:          audit.PolicyKeySnippetPolicy,
			PolicyValue:        audit.PolicyValueSnippetRestrict,
			Scope:              audit.PolicyScopeSite,
			TargetRef:          "site",
			Source:             audit.PolicyProvenanceUserInput,
			SuppliedAt:         now,
		},
	})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed: %v", err)
	}
	results, err = eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polRestrict}, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRuleWithContext failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusPass {
		t.Errorf("expected PASS for nosnippet with restrict policy, got %v", results)
	}

	// 4. Block-level data-nosnippet only with unspecified policy -> MANUAL_REVIEW
	snapDataNosnippet := newSnippetSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
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
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "nosnippet",
			Value:              "false",
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:3",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "data_nosnippet_count",
			Value:              "3",
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})

	results, err = eng.EvaluateRule(context.Background(), snapDataNosnippet, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusManualReview {
		t.Errorf("expected MANUAL_REVIEW for data-nosnippet with unspecified policy, got %v", results)
	}

	// 5. max-snippet:0 with allow_unrestricted -> FAIL
	snapMaxSnippet := newSnippetSnapshot([]audit.NormalizedObservation{
		{
			ObservationID:      "obs:1",
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
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
			AuditRunID:         "audit:snippet:run",
			SnapshotID:         "snap:snippet:test",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "max_snippet",
			Value:              "0",
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"src:1"},
			ObservedAt:         now,
		},
	})

	results, err = eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snapMaxSnippet, Policies: polAllow}, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRuleWithContext failed: %v", err)
	}
	if len(results) != 1 || results[0].Status != audit.StatusFail {
		t.Errorf("expected FAIL for max-snippet:0 with allow_unrestricted policy, got %v", results)
	}
}
