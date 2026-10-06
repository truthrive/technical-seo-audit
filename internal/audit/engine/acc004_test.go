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

// Helper to construct a frozen snapshot with arbitrary URL and status observations for testing input resolution.
func newSnapshotForInputs(subjectRef string, urls []string, statuses []string) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	for i, u := range urls {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID(fmt.Sprintf("obs:url:%s:%d", subjectRef, i)),
			AuditRunID:         "audit:run:inputs",
			SnapshotID:         "snap:run:inputs",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "url_identity",
			Value:              u,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		})
	}

	for i, st := range statuses {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID(fmt.Sprintf("obs:st:%s:%d", subjectRef, i)),
			AuditRunID:         "audit:run:inputs",
			SnapshotID:         "snap:run:inputs",
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
		SnapshotID:             "snap:run:inputs",
		AuditRunID:             "audit:run:inputs",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.2.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_ACC_004_URLInputResolution(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	subjectRef := "url:audit:run:inputs:1"

	t.Run("missing url_identity with status 200 -> UNKNOWN", func(t *testing.T) {
		snap := newSnapshotForInputs(subjectRef, nil, []string{"200"})
		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for missing URL, got %s", r.Status)
		}
		if r.Status == audit.StatusPass || r.Status == audit.StatusFail {
			t.Errorf("missing URL must NEVER produce PASS or FAIL")
		}
		if r.ObservedSummary != "Required URL identity evidence is unavailable." {
			t.Errorf("expected summary 'Required URL identity evidence is unavailable.', got %q", r.ObservedSummary)
		}
		// Must not contain fake URL evidence, but must contain status evidence
		if len(r.EvidenceRefs) != 1 || r.EvidenceRefs[0].Field != "fetch_status" {
			t.Errorf("expected 1 status evidence ref, got %+v", r.EvidenceRefs)
		}
	})

	t.Run("missing url_identity with status 503 -> UNKNOWN", func(t *testing.T) {
		snap := newSnapshotForInputs(subjectRef, nil, []string{"503"})
		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for missing URL with 503, got %s", r.Status)
		}
		if r.Status == audit.StatusFail {
			t.Errorf("missing URL with 503 must NEVER produce FAIL")
		}
	})

	t.Run("one usable URL + 200 -> PASS", func(t *testing.T) {
		snap := newSnapshotForInputs(subjectRef, []string{"https://example.com/ok"}, []string{"200"})
		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", r.Status)
		}
		if r.ObservedSummary != "HTTP status 200 is not a server-error response." {
			t.Errorf("unexpected summary: %q", r.ObservedSummary)
		}
		if len(r.EvidenceRefs) != 2 {
			t.Fatalf("expected 2 evidence refs, got %d", len(r.EvidenceRefs))
		}
	})

	t.Run("one usable URL + 503 -> FAIL", func(t *testing.T) {
		snap := newSnapshotForInputs(subjectRef, []string{"https://example.com/err"}, []string{"503"})
		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", r.Status)
		}
		if r.ObservedSummary != "HTTP status 503 is a server-error response." {
			t.Errorf("unexpected summary: %q", r.ObservedSummary)
		}
		if len(r.EvidenceRefs) != 2 {
			t.Fatalf("expected 2 evidence refs, got %d", len(r.EvidenceRefs))
		}
	})

	t.Run("conflicting URL identities -> UNKNOWN", func(t *testing.T) {
		snap := newSnapshotForInputs(subjectRef, []string{"https://example.com/a", "https://example.com/b"}, []string{"200"})
		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting URLs, got %s", r.Status)
		}
		if r.Status == audit.StatusPass || r.Status == audit.StatusFail {
			t.Errorf("conflicting URLs must NEVER produce PASS or FAIL")
		}
		if r.ObservedSummary != "No usable URL identity is available: conflicting URL identity observations." {
			t.Errorf("unexpected summary: %q", r.ObservedSummary)
		}
		// Must include all conflicting URL evidence refs plus status ref
		if len(r.EvidenceRefs) != 3 {
			t.Fatalf("expected 3 evidence refs (2 conflicting URLs + 1 status), got %d", len(r.EvidenceRefs))
		}
		urlRefsCount := 0
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "url" {
				urlRefsCount++
			}
		}
		if urlRefsCount != 2 {
			t.Errorf("expected 2 URL context refs, got %d", urlRefsCount)
		}
	})

	t.Run("duplicate identical URL identities with 200 -> PASS", func(t *testing.T) {
		snap := newSnapshotForInputs(subjectRef, []string{"https://example.com/dup", "https://example.com/dup"}, []string{"200"})
		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS for duplicate identical URLs with 200, got %s", r.Status)
		}
		// Both source observations should be retained in evidence refs
		if len(r.EvidenceRefs) != 3 {
			t.Fatalf("expected 3 evidence refs (2 source identical URLs + 1 status), got %d", len(r.EvidenceRefs))
		}
		// Verify distinct RuleEvidenceRef IDs
		refIDs := make(map[audit.RuleEvidenceRefID]struct{})
		for _, ref := range r.EvidenceRefs {
			if _, exists := refIDs[ref.RuleEvidenceRefID]; exists {
				t.Errorf("duplicate RuleEvidenceRefID detected: %q", ref.RuleEvidenceRefID)
			}
			refIDs[ref.RuleEvidenceRefID] = struct{}{}
		}
	})

	t.Run("duplicate identical URL identities with 503 -> FAIL", func(t *testing.T) {
		snap := newSnapshotForInputs(subjectRef, []string{"https://example.com/dup", "https://example.com/dup"}, []string{"503"})
		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Errorf("expected FAIL for duplicate identical URLs with 503, got %s", r.Status)
		}
		if len(r.EvidenceRefs) != 3 {
			t.Fatalf("expected 3 evidence refs, got %d", len(r.EvidenceRefs))
		}
	})
}
