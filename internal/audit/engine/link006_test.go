package engine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForLINK006(subjectRef, urlStr, depthVal string) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	obs := []audit.NormalizedObservation{
		{
			ObservationID:      audit.ObservationID("obs:url:" + subjectRef),
			AuditRunID:         "audit:run:link006",
			SnapshotID:         "snap:run:link006",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "url_identity",
			Value:              urlStr,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		},
	}

	if depthVal != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:depth:" + subjectRef),
			AuditRunID:         "audit:run:link006",
			SnapshotID:         "snap:run:link006",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "crawl_depth",
			Value:              depthVal,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		})
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:link006",
		AuditRunID:             "audit:run:link006",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_LINK_006_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	runID := audit.AuditRunID("audit:run:link006")
	subjectRef := "url:link006:1"
	testURL := "https://example.com/priority-pricing"

	t.Run("Priority page policy not assigned -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSnapshotForLINK006(subjectRef, testURL, "2")
		res, err := eng.EvaluateRule(ctx, snap, "AR-LINK-006")
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

	t.Run("Priority page policy=false -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSnapshotForLINK006(subjectRef, testURL, "2")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:link006:false",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyPriorityPage,
				PolicyValue:        "false",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-LINK-006")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected StatusNotApplicable, got %s", res[0].Status)
		}
	})

	t.Run("Priority page policy=true with crawl_depth=4 -> MANUAL_REVIEW", func(t *testing.T) {
		snap := newSnapshotForLINK006(subjectRef, testURL, "4")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:link006:true",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyPriorityPage,
				PolicyValue:        "true",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-LINK-006")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusManualReview {
			t.Errorf("expected StatusManualReview, got %s", res[0].Status)
		}
		if !strings.Contains(res[0].ObservedSummary, "depth 4") {
			t.Errorf("expected summary to contain 'depth 4', got %q", res[0].ObservedSummary)
		}
	})

	t.Run("Priority page policy=true with crawl_depth missing -> UNKNOWN", func(t *testing.T) {
		snap := newSnapshotForLINK006(subjectRef, testURL, "")
		pol, _ := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:link006:true",
				AuditRunID:         runID,
				PolicyKey:          audit.PolicyKeyPriorityPage,
				PolicyValue:        "true",
				Scope:              audit.PolicyScopeURL,
				TargetRef:          subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pol}, "AR-LINK-006")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed: %v", err)
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected StatusUnknown, got %s", res[0].Status)
		}
	})
}
