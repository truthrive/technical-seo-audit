package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func buildACC005Snapshot(urlIdentity, robotsAllowedGooglebot, matchedRule string) *audit.EvidenceSnapshot {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	var obs []audit.NormalizedObservation
	id := 1
	add := func(field, val string) {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:acc005:" + string(rune('0'+id))),
			AuditRunID:         "audit:acc005",
			SnapshotID:         "snap:acc005",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:audit:acc005:1",
			Field:              field,
			Value:              val,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:1"},
			ObservedAt:         now,
		})
		id++
	}
	if urlIdentity != "" {
		add("url_identity", urlIdentity)
	}
	if robotsAllowedGooglebot != "" {
		add("robots_allowed_googlebot", robotsAllowedGooglebot)
	}
	if matchedRule != "" {
		add("matched_robots_rule_googlebot", matchedRule)
	}
	add("robots_txt_url", "https://example.com/robots.txt")

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:acc005",
		AuditRunID:             "audit:acc005",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now,
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestEvaluateACC005_VerdictMatrix(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	ctx := context.Background()

	tests := []struct {
		name           string
		robotsAllowed  string
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
			snap := buildACC005Snapshot("https://example.com/page", tc.robotsAllowed, "Disallow: /page")
			polIdx := engine.NewEmptyPolicyIndex()
			if tc.policyValue != "" {
				var err error
				polIdx, err = engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
					{
						PolicyAssignmentID: "pol:acc005:1",
						AuditRunID:         "audit:acc005",
						PolicyKey:          audit.PolicyKeyGooglebotAccessPolicy,
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
			}, "AR-ACC-005")
			if err != nil {
				t.Fatalf("EvaluateRuleWithContext: %v", err)
			}
			if len(res) != 1 {
				t.Fatalf("expected 1 result, got %d", len(res))
			}
			if res[0].Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s (summary=%q)", tc.expectedStatus, res[0].Status, res[0].ObservedSummary)
			}
			if res[0].Severity != audit.SeverityP1 {
				t.Errorf("expected severity P1, got %s", res[0].Severity)
			}
		})
	}
}
