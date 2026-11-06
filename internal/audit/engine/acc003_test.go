package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForACC003(subjectRef, urlStr, statusVal string, extraObs ...audit.NormalizedObservation) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	obs := []audit.NormalizedObservation{
		{
			ObservationID:      audit.ObservationID("obs:url:" + subjectRef),
			AuditRunID:         "audit:run:acc003",
			SnapshotID:         "snap:run:acc003",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "url_identity",
			Value:              urlStr,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		},
	}

	if statusVal != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:status:" + subjectRef),
			AuditRunID:         "audit:run:acc003",
			SnapshotID:         "snap:run:acc003",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "http_status",
			Value:              statusVal,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		})
	}
	obs = append(obs, extraObs...)

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:acc003",
		AuditRunID:             "audit:run:acc003",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_ACC_003_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	runID := audit.AuditRunID("audit:run:acc003")
	subjectRef := "url:acc003:1"
	testURL := "https://example.com/page"

	t.Run("Policy expected_url_state not configured -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSnapshotForACC003(subjectRef, testURL, "200")
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected StatusNotApplicable, got %s", res[0].Status)
		}
	})

	t.Run("Policy live: 200 -> PASS", func(t *testing.T) {
		snap := newSnapshotForACC003(subjectRef, testURL, "200")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:acc003:1",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedURLState,
				PolicyValue:        "live",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-ACC-003")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusPass {
			t.Errorf("expected StatusPass, got %s", res[0].Status)
		}
	})

	t.Run("Policy live: 404 -> FAIL", func(t *testing.T) {
		snap := newSnapshotForACC003(subjectRef, testURL, "404")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:acc003:2",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedURLState,
				PolicyValue:        "live",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-ACC-003")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusFail {
			t.Errorf("expected StatusFail, got %s", res[0].Status)
		}
		if res[0].Severity != audit.SeverityP1 {
			t.Errorf("expected SeverityP1, got %s", res[0].Severity)
		}
	})

	t.Run("Policy redirect: 301 -> PASS", func(t *testing.T) {
		snap := newSnapshotForACC003(subjectRef, testURL, "301")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:acc003:3",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedURLState,
				PolicyValue:        "redirect",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-ACC-003")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusPass {
			t.Errorf("expected StatusPass, got %s", res[0].Status)
		}
	})

	t.Run("Policy redirect: 200 -> FAIL", func(t *testing.T) {
		snap := newSnapshotForACC003(subjectRef, testURL, "200")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:acc003:4",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedURLState,
				PolicyValue:        "redirect",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-ACC-003")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusFail {
			t.Errorf("expected StatusFail, got %s", res[0].Status)
		}
	})

	t.Run("Policy missing: 404 -> PASS", func(t *testing.T) {
		snap := newSnapshotForACC003(subjectRef, testURL, "404")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:acc003:5",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedURLState,
				PolicyValue:        "missing",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-ACC-003")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusPass {
			t.Errorf("expected StatusPass, got %s", res[0].Status)
		}
	})

	t.Run("Policy missing: 200 -> FAIL", func(t *testing.T) {
		snap := newSnapshotForACC003(subjectRef, testURL, "200")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:acc003:6",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedURLState,
				PolicyValue:        "missing",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-ACC-003")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusFail {
			t.Errorf("expected StatusFail, got %s", res[0].Status)
		}
	})

	t.Run("Policy live with conflicting http_status -> UNKNOWN", func(t *testing.T) {
		extra := audit.NormalizedObservation{
			ObservationID:      "obs:status:conflict",
			AuditRunID:         runID,
			SnapshotID:         "snap:run:acc003",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "http_status",
			Value:              "500",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test2"},
			ObservedAt:         time.Now().UTC(),
		}
		snap := newSnapshotForACC003(subjectRef, testURL, "200", extra)
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:acc003:7",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedURLState,
				PolicyValue:        "live",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-ACC-003")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected StatusUnknown, got %s", res[0].Status)
		}
	})
}
