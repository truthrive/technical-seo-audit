package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForACC002(subjectRef, urlStr, robotsVal string, extraObs ...audit.NormalizedObservation) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	obs := []audit.NormalizedObservation{
		{
			ObservationID:      audit.ObservationID("obs:url:" + subjectRef),
			AuditRunID:         "audit:run:acc002",
			SnapshotID:         "snap:run:acc002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "url_identity",
			Value:              urlStr,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		},
	}

	if robotsVal != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:robots:" + subjectRef),
			AuditRunID:         "audit:run:acc002",
			SnapshotID:         "snap:run:acc002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "robots_allowed_default",
			Value:              robotsVal,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		})
	}
	obs = append(obs, extraObs...)

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:acc002",
		AuditRunID:             "audit:run:acc002",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_ACC_002_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	runID := audit.AuditRunID("audit:run:acc002")
	subjectRef := "url:acc002:1"
	testURL := "https://example.com/section/page"

	t.Run("Policy expected_crawlable=true and robots_allowed_default=true -> PASS", func(t *testing.T) {
		snap := newSnapshotForACC002(subjectRef, testURL, "true")
		pol, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:1",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedCrawlable,
				PolicyValue:        "true",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		if err != nil {
			t.Fatalf("failed to create policy index: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: pol,
		}, "AR-ACC-002")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusPass {
			t.Errorf("expected StatusPass, got %s", res[0].Status)
		}
		if len(res[0].EvidenceRefs) == 0 {
			t.Errorf("expected evidence refs attached")
		}
	})

	t.Run("Policy expected_crawlable=true and robots_allowed_default=false -> FAIL", func(t *testing.T) {
		snap := newSnapshotForACC002(subjectRef, testURL, "false")
		pol, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:2",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedCrawlable,
				PolicyValue:        "true",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		if err != nil {
			t.Fatalf("failed to create policy index: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: pol,
		}, "AR-ACC-002")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusFail {
			t.Errorf("expected StatusFail, got %s", res[0].Status)
		}
		if res[0].Severity != audit.SeverityP1 {
			t.Errorf("expected SeverityP1, got %s", res[0].Severity)
		}
	})

	t.Run("Policy expected_crawlable=true and robots_allowed_default missing -> UNKNOWN", func(t *testing.T) {
		snap := newSnapshotForACC002(subjectRef, testURL, "")
		pol, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:3",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedCrawlable,
				PolicyValue:        "true",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		if err != nil {
			t.Fatalf("failed to create policy index: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: pol,
		}, "AR-ACC-002")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected StatusUnknown, got %s", res[0].Status)
		}
	})

	t.Run("Policy expected_crawlable=true with conflicting robots observations -> UNKNOWN", func(t *testing.T) {
		extra := audit.NormalizedObservation{
			ObservationID:      "obs:robots:conflict",
			AuditRunID:         runID,
			SnapshotID:         "snap:run:acc002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "robots_allowed_default",
			Value:              "false",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test2"},
			ObservedAt:         time.Now().UTC(),
		}
		snap := newSnapshotForACC002(subjectRef, testURL, "true", extra)
		pol, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:4",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedCrawlable,
				PolicyValue:        "true",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		if err != nil {
			t.Fatalf("failed to create policy index: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: pol,
		}, "AR-ACC-002")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected StatusUnknown, got %s", res[0].Status)
		}
	})

	t.Run("Policy expected_crawlable not set -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSnapshotForACC002(subjectRef, testURL, "true")
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-002")
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

	t.Run("Policy expected_crawlable=false -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSnapshotForACC002(subjectRef, testURL, "true")
		pol, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:5",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyExpectedCrawlable,
				PolicyValue:        "false",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		if err != nil {
			t.Fatalf("failed to create policy index: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: pol,
		}, "AR-ACC-002")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected StatusNotApplicable, got %s", res[0].Status)
		}
	})
}
