package engine_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

// Helper to build a frozen snapshot for testing AR-ACC-004 status cases.
func newSnapshotForStatus(subjectRef, urlStr string, statuses []string) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	obs := []audit.NormalizedObservation{
		{
			ObservationID:      audit.ObservationID("obs:url:" + subjectRef),
			AuditRunID:         "audit:run:status",
			SnapshotID:         "snap:run:status",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "url_identity",
			Value:              urlStr,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		},
	}

	for i, st := range statuses {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID(fmt.Sprintf("obs:st:%s:%d", subjectRef, i)),
			AuditRunID:         "audit:run:status",
			SnapshotID:         "snap:run:status",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "http_status",
			Value:              st,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		})
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:status",
		AuditRunID:             "audit:run:status",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.2.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_ACC_004_StatusEvaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name           string
		statuses       []string
		expectedStatus audit.RuleResultStatus
		expectedRole   audit.EvidenceRole
	}{
		// PASS cases (non-5xx responses)
		{name: "status 200", statuses: []string{"200"}, expectedStatus: audit.StatusPass},
		{name: "status 204", statuses: []string{"204"}, expectedStatus: audit.StatusPass},
		{name: "status 301", statuses: []string{"301"}, expectedStatus: audit.StatusPass},
		{name: "status 404", statuses: []string{"404"}, expectedStatus: audit.StatusPass},
		{name: "status 429", statuses: []string{"429"}, expectedStatus: audit.StatusPass},

		// FAIL cases (5xx server errors)
		{name: "status 500", statuses: []string{"500"}, expectedStatus: audit.StatusFail},
		{name: "status 502", statuses: []string{"502"}, expectedStatus: audit.StatusFail},
		{name: "status 503", statuses: []string{"503"}, expectedStatus: audit.StatusFail},
		{name: "status 599", statuses: []string{"599"}, expectedStatus: audit.StatusFail},

		// UNKNOWN cases (no usable HTTP response obtained)
		{name: "missing status", statuses: nil, expectedStatus: audit.StatusUnknown},
		{name: "status 0 (network failure / robots blocked)", statuses: []string{"0"}, expectedStatus: audit.StatusUnknown},
		{name: "malformed status string", statuses: []string{"invalid-http"}, expectedStatus: audit.StatusUnknown},
		{name: "conflicting statuses", statuses: []string{"200", "500"}, expectedStatus: audit.StatusUnknown},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			subjectRef := "url:audit:run:status:1"
			snap := newSnapshotForStatus(subjectRef, "https://example.com/test", tc.statuses)

			results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
			if err != nil {
				t.Fatalf("EvaluateRule failed: %v", err)
			}

			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}

			r := results[0]

			// Contract assertions
			if r.RuleID != "AR-ACC-004" {
				t.Errorf("expected RuleID AR-ACC-004, got %q", r.RuleID)
			}
			if r.ParentCheck != "ACC-007" {
				t.Errorf("expected ParentCheck ACC-007, got %q", r.ParentCheck)
			}
			if r.RuleVersion != 1 {
				t.Errorf("expected RuleVersion 1, got %d", r.RuleVersion)
			}
			if r.Severity != audit.SeverityP1 {
				t.Errorf("expected Severity P1, got %q", r.Severity)
			}
			if r.SubjectType != audit.SubjectURL {
				t.Errorf("expected SubjectType URL, got %q", r.SubjectType)
			}
			if r.Scope != audit.ScopeURL {
				t.Errorf("expected Scope URL, got %q", r.Scope)
			}
			if r.SubjectRef != subjectRef {
				t.Errorf("expected SubjectRef %q, got %q", subjectRef, r.SubjectRef)
			}
			if r.ExpectedSummary != "HTTP response status is not 5xx." {
				t.Errorf("expected ExpectedSummary 'HTTP response status is not 5xx.', got %q", r.ExpectedSummary)
			}

			// Status check
			if r.Status != tc.expectedStatus {
				t.Errorf("expected Status %q, got %q (ObservedSummary: %q)", tc.expectedStatus, r.Status, r.ObservedSummary)
			}

			// Explicit non-FAIL guardrails
			if tc.name == "status 404" && r.Status == audit.StatusFail {
				t.Errorf("CRITICAL: 404 must NOT be classified as FAIL for AR-ACC-004")
			}
			if tc.name == "status 429" && r.Status == audit.StatusFail {
				t.Errorf("CRITICAL: 429 must NOT be classified as FAIL for AR-ACC-004")
			}
			if (tc.name == "missing status" || tc.name == "status 0") && r.Status == audit.StatusFail {
				t.Errorf("CRITICAL: missing/0 status must NOT be classified as FAIL for AR-ACC-004")
			}

			// Evidence references check
			foundURLContext := false
			foundStatusPrimary := false

			for _, ref := range r.EvidenceRefs {
				if ref.Field == "url" && ref.Role == audit.EvidenceRoleContext && ref.ObservedValue == "https://example.com/test" {
					foundURLContext = true
				}
				if ref.Field == "fetch_status" && ref.Role == audit.EvidenceRolePrimary {
					foundStatusPrimary = true
				}
			}

			if !foundURLContext {
				t.Errorf("expected EvidenceRoleContext for field 'url'")
			}
			if len(tc.statuses) > 0 && !foundStatusPrimary {
				t.Errorf("expected EvidenceRolePrimary for field 'fetch_status'")
			}
		})
	}
}
