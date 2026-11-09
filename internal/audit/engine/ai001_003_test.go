package engine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func buildAI001And003Snapshot(robotsOAI, robotsGPT string) *audit.EvidenceSnapshot {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	var obs []audit.NormalizedObservation
	id := 1
	add := func(field, val string) {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:ai:" + string(rune('0'+id))),
			AuditRunID:         "audit:ai",
			SnapshotID:         "snap:ai",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:audit:ai:1",
			Field:              field,
			Value:              val,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:1"},
			ObservedAt:         now,
		})
		id++
	}
	add("url_identity", "https://example.com/article")
	if robotsOAI != "" {
		add("robots_allowed_oai_searchbot", robotsOAI)
	}
	if robotsGPT != "" {
		add("robots_allowed_gptbot", robotsGPT)
	}
	add("robots_txt_url", "https://example.com/robots.txt")

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:ai",
		AuditRunID:             "audit:ai",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now,
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestEvaluateAI001_VerdictMatrix(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	ctx := context.Background()

	tests := []struct {
		name           string
		robotsOAI      string
		policyValue    string
		expectedStatus audit.RuleResultStatus
	}{
		{"Unspecified_Allowed_PASS", "true", "", audit.StatusPass},
		{"Unspecified_Blocked_WARNING", "false", "", audit.StatusWarning},
		{"AllowPolicy_Allowed_PASS", "true", "allow", audit.StatusPass},
		{"AllowPolicy_Blocked_FAIL", "false", "allow", audit.StatusFail},
		{"BlockPolicy_Blocked_PASS", "false", "block", audit.StatusPass},
		{"BlockPolicy_Allowed_FAIL", "true", "block", audit.StatusFail},
		{"UnknownRobots_UNKNOWN", "unknown", "allow", audit.StatusUnknown},
		{"MissingRobots_UNKNOWN", "", "allow", audit.StatusUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snap := buildAI001And003Snapshot(tc.robotsOAI, "true")
			polIdx := engine.NewEmptyPolicyIndex()
			if tc.policyValue != "" {
				polIdx, err = engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
					{
						PolicyAssignmentID: "pol:ai001:1",
						AuditRunID:         "audit:ai",
						PolicyKey:          audit.PolicyKeyOAISearchbotAccessPolicy,
						Scope:              audit.PolicyScopeSite,
						TargetRef:          "site",
						PolicyValue:        tc.policyValue,
						Source:             audit.PolicyProvenanceUserInput,
					},
				})
				if err != nil {
					t.Fatalf("NewPolicyIndex: %v", err)
				}
			}

			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
				Snapshot: snap,
				Policies: polIdx,
			}, "AR-AI-001")
			if err != nil {
				t.Fatalf("EvaluateRuleWithContext: %v", err)
			}
			if len(res) != 1 {
				t.Fatalf("expected 1 result, got %d", len(res))
			}
			if res[0].Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s (summary=%q)", tc.expectedStatus, res[0].Status, res[0].ObservedSummary)
			}
			if res[0].Severity != audit.SeverityP2 {
				t.Errorf("expected severity P2, got %s", res[0].Severity)
			}
			if strings.Contains(strings.ToLower(res[0].ObservedSummary), "ranking") || strings.Contains(strings.ToLower(res[0].ObservedSummary), "citation") {
				t.Errorf("observed summary must not claim ranking or citation impact: %q", res[0].ObservedSummary)
			}
		})
	}
}

func TestEvaluateAI003_VerdictMatrix(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	ctx := context.Background()

	tests := []struct {
		name           string
		robotsGPT      string
		policyValue    string
		expectedStatus audit.RuleResultStatus
	}{
		{"Unspecified_Allowed_MANUAL_REVIEW", "true", "", audit.StatusManualReview},
		{"Unspecified_Blocked_MANUAL_REVIEW", "false", "", audit.StatusManualReview},
		{"ExplicitUnspecified_MANUAL_REVIEW", "false", "unspecified", audit.StatusManualReview},
		{"AllowPolicy_Allowed_PASS", "true", "allow", audit.StatusPass},
		{"AllowPolicy_Blocked_WARNING", "false", "allow", audit.StatusWarning},
		{"BlockPolicy_Blocked_PASS", "false", "block", audit.StatusPass},
		{"BlockPolicy_Allowed_WARNING", "true", "block", audit.StatusWarning},
		{"UnknownRobots_UNKNOWN", "unknown", "block", audit.StatusUnknown},
		{"MissingRobots_UNKNOWN", "", "block", audit.StatusUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snap := buildAI001And003Snapshot("true", tc.robotsGPT)
			polIdx := engine.NewEmptyPolicyIndex()
			if tc.policyValue != "" {
				polIdx, err = engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
					{
						PolicyAssignmentID: "pol:ai003:1",
						AuditRunID:         "audit:ai",
						PolicyKey:          audit.PolicyKeyGPTBotTrainingPolicy,
						Scope:              audit.PolicyScopeSite,
						TargetRef:          "site",
						PolicyValue:        tc.policyValue,
						Source:             audit.PolicyProvenanceUserInput,
					},
				})
				if err != nil {
					t.Fatalf("NewPolicyIndex: %v", err)
				}
			}

			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
				Snapshot: snap,
				Policies: polIdx,
			}, "AR-AI-003")
			if err != nil {
				t.Fatalf("EvaluateRuleWithContext: %v", err)
			}
			if len(res) != 1 {
				t.Fatalf("expected 1 result, got %d", len(res))
			}
			if res[0].Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s (summary=%q)", tc.expectedStatus, res[0].Status, res[0].ObservedSummary)
			}
			if res[0].Severity != audit.SeverityP2 {
				t.Errorf("expected severity P2, got %s", res[0].Severity)
			}
		})
	}
}
