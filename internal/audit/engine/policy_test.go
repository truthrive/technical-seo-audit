package engine_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

// Helper to construct a base valid ProjectPolicyAssignment.
func newBasePolicy(id audit.PolicyAssignmentID, runID audit.AuditRunID, scope audit.PolicyScope, targetRef string, key audit.PolicyKey, val string) audit.ProjectPolicyAssignment {
	return audit.ProjectPolicyAssignment{
		PolicyAssignmentID: id,
		AuditRunID:         runID,
		PolicyKey:          key,
		PolicyValue:        val,
		Scope:              scope,
		TargetRef:          targetRef,
		Source:             audit.PolicyProvenanceUserInput,
		SuppliedAt:         time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
	}
}

// 1. Valid policy lookup tests
func TestPolicyIndex_ValidPolicyLookup(t *testing.T) {
	runID := audit.AuditRunID("audit:run:pol:1")
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

	sitePol := audit.ProjectPolicyAssignment{
		PolicyAssignmentID: "pol:site:origin",
		AuditRunID:         runID,
		PolicyKey:          audit.PolicyKeyPreferredOrigin,
		PolicyValue:        "https://example.com",
		Scope:              audit.PolicyScopeSite,
		TargetRef:          "https://example.com",
		Source:             audit.PolicyProvenanceUserInput,
		SuppliedAt:         now,
	}

	urlPol := audit.ProjectPolicyAssignment{
		PolicyAssignmentID: "pol:url:1:indexable",
		AuditRunID:         runID,
		PolicyKey:          audit.PolicyKeyExpectedIndexable,
		PolicyValue:        "true",
		Scope:              audit.PolicyScopeURL,
		TargetRef:          "url:audit:run:pol:1:1",
		Source:             audit.PolicyProvenanceProjectConfiguration,
		SuppliedAt:         now,
	}

	idx, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{sitePol, urlPol})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed on valid policies: %v", err)
	}

	if idx.Len() != 2 {
		t.Errorf("expected Len() == 2, got %d", idx.Len())
	}
	if idx.AuditRunID() != runID {
		t.Errorf("expected AuditRunID %q, got %q", runID, idx.AuditRunID())
	}

	// 1a. SITE policy lookup & intact fields verification
	gotSite, ok := idx.Get(audit.PolicyScopeSite, "https://example.com", audit.PolicyKeyPreferredOrigin)
	if !ok || gotSite == nil {
		t.Fatalf("expected to find SITE policy")
	}
	if gotSite.PolicyAssignmentID != sitePol.PolicyAssignmentID ||
		gotSite.AuditRunID != sitePol.AuditRunID ||
		gotSite.PolicyKey != sitePol.PolicyKey ||
		gotSite.PolicyValue != sitePol.PolicyValue ||
		gotSite.Scope != sitePol.Scope ||
		gotSite.TargetRef != sitePol.TargetRef ||
		gotSite.Source != sitePol.Source ||
		!gotSite.SuppliedAt.Equal(sitePol.SuppliedAt) {
		t.Errorf("SITE policy fields not returned intact: got %+v, expected %+v", *gotSite, sitePol)
	}

	// 1b. URL policy lookup & intact fields verification
	gotURL, ok := idx.Get(audit.PolicyScopeURL, "url:audit:run:pol:1:1", audit.PolicyKeyExpectedIndexable)
	if !ok || gotURL == nil {
		t.Fatalf("expected to find URL policy")
	}
	if gotURL.PolicyAssignmentID != urlPol.PolicyAssignmentID ||
		gotURL.AuditRunID != urlPol.AuditRunID ||
		gotURL.PolicyKey != urlPol.PolicyKey ||
		gotURL.PolicyValue != urlPol.PolicyValue ||
		gotURL.Scope != urlPol.Scope ||
		gotURL.TargetRef != urlPol.TargetRef ||
		gotURL.Source != urlPol.Source ||
		!gotURL.SuppliedAt.Equal(urlPol.SuppliedAt) {
		t.Errorf("URL policy fields not returned intact: got %+v, expected %+v", *gotURL, urlPol)
	}

	// 1c. Non-existent lookups
	if _, ok := idx.Get(audit.PolicyScopeURL, "url:nonexistent", audit.PolicyKeyExpectedIndexable); ok {
		t.Errorf("expected not found for nonexistent targetRef")
	}
	if _, ok := idx.Get(audit.PolicyScopeURL, "url:audit:run:pol:1:1", audit.PolicyKeyExpectedCrawlable); ok {
		t.Errorf("expected not found for nonexistent policy key")
	}
	if _, ok := idx.Get(audit.PolicyScopeSite, "url:audit:run:pol:1:1", audit.PolicyKeyExpectedIndexable); ok {
		t.Errorf("expected not found when querying with wrong scope")
	}
}

// 2. Validation error tests
func TestPolicyIndex_ValidationErrors(t *testing.T) {
	runID := audit.AuditRunID("audit:run:val:1")

	// 2a. Invalid scope rejected
	t.Run("invalid scope", func(t *testing.T) {
		p := newBasePolicy("pol:1", runID, audit.PolicyScope("INVALID_SCOPE"), "ref:1", audit.PolicyKeyExpectedIndexable, "true")
		_, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{p})
		if !errors.Is(err, engine.ErrInvalidPolicy) {
			t.Errorf("expected ErrInvalidPolicy for invalid scope, got %v", err)
		}
	})

	// 2b. Invalid provenance rejected (e.g. prohibited crawler or LLM inference)
	t.Run("invalid provenance", func(t *testing.T) {
		p := newBasePolicy("pol:1", runID, audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "true")
		p.Source = audit.PolicyProvenance("CRAWLER_INFERENCE")
		_, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{p})
		if !errors.Is(err, engine.ErrInvalidPolicy) {
			t.Errorf("expected ErrInvalidPolicy for invalid provenance, got %v", err)
		}
	})

	// 2c. Invalid controlled value rejected
	t.Run("invalid controlled value", func(t *testing.T) {
		p := newBasePolicy("pol:1", runID, audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "maybe")
		_, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{p})
		if !errors.Is(err, engine.ErrInvalidPolicy) {
			t.Errorf("expected ErrInvalidPolicy for invalid controlled value, got %v", err)
		}
	})

	// 2d. Empty assignment ID rejected
	t.Run("empty assignment ID", func(t *testing.T) {
		p := newBasePolicy("", runID, audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "true")
		_, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{p})
		if !errors.Is(err, engine.ErrInvalidPolicy) {
			t.Errorf("expected ErrInvalidPolicy for empty assignment ID, got %v", err)
		}
	})

	// 2e. AuditRunID mismatch within assignment slice rejected
	t.Run("AuditRunID mismatch within slice", func(t *testing.T) {
		p1 := newBasePolicy("pol:1", "run:A", audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "true")
		p2 := newBasePolicy("pol:2", "run:B", audit.PolicyScopeURL, "ref:2", audit.PolicyKeyExpectedIndexable, "true")
		_, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{p1, p2})
		if !errors.Is(err, engine.ErrPolicyRunMismatch) {
			t.Errorf("expected ErrPolicyRunMismatch for differing run IDs in slice, got %v", err)
		}
	})

	// 2f. AuditRunID mismatch with expected run ID rejected
	t.Run("AuditRunID mismatch with expected run ID", func(t *testing.T) {
		p := newBasePolicy("pol:1", "run:actual", audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "true")
		_, err := engine.NewPolicyIndexForRun("run:expected", []audit.ProjectPolicyAssignment{p})
		if !errors.Is(err, engine.ErrPolicyRunMismatch) {
			t.Errorf("expected ErrPolicyRunMismatch when assignment runID differs from expected, got %v", err)
		}
	})
}

// 3. Duplicate and conflict handling tests
func TestPolicyIndex_DuplicatesAndConflicts(t *testing.T) {
	runID := audit.AuditRunID("audit:run:dup:1")

	// 3a. Identical duplicates accepted and deduplicated
	t.Run("identical duplicates accepted", func(t *testing.T) {
		p1 := newBasePolicy("pol:dup:2", runID, audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "true")
		p2 := newBasePolicy("pol:dup:1", runID, audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "true")

		idx, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{p1, p2})
		if err != nil {
			t.Fatalf("expected identical duplicates to be accepted, got error: %v", err)
		}

		if idx.Len() != 1 {
			t.Errorf("expected Len() == 1 for identical duplicates, got %d", idx.Len())
		}

		got, ok := idx.Get(audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable)
		if !ok || got == nil {
			t.Fatalf("expected to find policy")
		}
		if got.PolicyValue != "true" {
			t.Errorf("expected value 'true', got %q", got.PolicyValue)
		}
		// Canonical choice selects smallest ID ("pol:dup:1") deterministically
		if got.PolicyAssignmentID != "pol:dup:1" {
			t.Errorf("expected canonical selection 'pol:dup:1', got %q", got.PolicyAssignmentID)
		}
	})

	// 3b. Conflicting duplicates rejected
	t.Run("conflicting duplicates rejected", func(t *testing.T) {
		p1 := newBasePolicy("pol:conflict:1", runID, audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "true")
		p2 := newBasePolicy("pol:conflict:2", runID, audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "false")

		_, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{p1, p2})
		if err == nil {
			t.Fatalf("expected error for conflicting policies, got nil")
		}
		if !errors.Is(err, engine.ErrConflictingPolicy) {
			t.Errorf("expected ErrConflictingPolicy, got %v", err)
		}
	})

	// 3c. Reversed input order behaves deterministically
	t.Run("reversed input order determinism", func(t *testing.T) {
		p1 := newBasePolicy("pol:a", runID, audit.PolicyScopeURL, "ref:1", audit.PolicyKeyExpectedIndexable, "true")
		p2 := newBasePolicy("pol:b", runID, audit.PolicyScopeURL, "ref:2", audit.PolicyKeyExpectedCrawlable, "false")
		p3 := newBasePolicy("pol:c", runID, audit.PolicyScopeSite, "site:1", audit.PolicyKeyPreferredOrigin, "https://example.com")

		sliceForward := []audit.ProjectPolicyAssignment{p1, p2, p3}
		sliceReversed := []audit.ProjectPolicyAssignment{p3, p2, p1}

		idx1, err := engine.NewPolicyIndex(sliceForward)
		if err != nil {
			t.Fatalf("forward NewPolicyIndex failed: %v", err)
		}
		idx2, err := engine.NewPolicyIndex(sliceReversed)
		if err != nil {
			t.Fatalf("reversed NewPolicyIndex failed: %v", err)
		}

		if idx1.Len() != idx2.Len() {
			t.Fatalf("length mismatch: %d vs %d", idx1.Len(), idx2.Len())
		}

		for _, p := range sliceForward {
			got1, ok1 := idx1.Get(p.Scope, p.TargetRef, p.PolicyKey)
			got2, ok2 := idx2.Get(p.Scope, p.TargetRef, p.PolicyKey)
			if !ok1 || !ok2 {
				t.Fatalf("lookup failed for %s/%s/%s", p.Scope, p.TargetRef, p.PolicyKey)
			}
			if got1.PolicyAssignmentID != got2.PolicyAssignmentID || got1.PolicyValue != got2.PolicyValue {
				t.Errorf("lookup mismatch between forward and reversed order for %s/%s/%s", p.Scope, p.TargetRef, p.PolicyKey)
			}
		}

		assignments1 := idx1.Assignments()
		assignments2 := idx2.Assignments()
		if len(assignments1) != len(assignments2) {
			t.Fatalf("Assignments() length mismatch")
		}
		for i := range assignments1 {
			if assignments1[i].PolicyAssignmentID != assignments2[i].PolicyAssignmentID {
				t.Errorf("Assignments() order mismatch at index %d: %q vs %q",
					i, assignments1[i].PolicyAssignmentID, assignments2[i].PolicyAssignmentID)
			}
		}
	})
}

// 4. Engine integration tests
func TestEngine_PolicyIntegration(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	snap := &audit.EvidenceSnapshot{
		SnapshotID:           "snap:policy:test:1",
		AuditRunID:           "audit:run:pol:test:1",
		SnapshotStatus:       audit.SnapshotFrozen,
		NormalizationVersion: "v1.2.0",
		CrawlComplete:        true,
		CreatedAt:            now.Add(-1 * time.Minute),
		FrozenAt:             &now,
		NormalizedObservations: []audit.NormalizedObservation{
			{
				ObservationID:      "obs:url:1",
				AuditRunID:         "audit:run:pol:test:1",
				SnapshotID:         "snap:policy:test:1",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         "url:audit:run:pol:test:1:1",
				Field:              "url_identity",
				Value:              "https://example.com/ok",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			},
			{
				ObservationID:      "obs:st:1",
				AuditRunID:         "audit:run:pol:test:1",
				SnapshotID:         "snap:policy:test:1",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         "url:audit:run:pol:test:1:1",
				Field:              "http_status",
				Value:              "200",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			},
		},
	}

	// 4a. AR-ACC-004 still works through the backward-compatible EvaluateRule API
	t.Run("backward compatible EvaluateRule API", func(t *testing.T) {
		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusPass {
			t.Errorf("expected PASS for status 200, got %s", results[0].Status)
		}
	})

	// 4b. AR-ACC-004 works through EvaluationContext with empty policies
	t.Run("EvaluationContext with nil and empty policies", func(t *testing.T) {
		// Nil policies
		resNil, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: nil,
		}, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed with nil policies: %v", err)
		}
		if len(resNil) != 1 || resNil[0].Status != audit.StatusPass {
			t.Errorf("unexpected result with nil policies")
		}

		// Empty PolicyIndex
		resEmpty, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: engine.NewEmptyPolicyIndex(),
		}, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed with empty policies: %v", err)
		}
		if len(resEmpty) != 1 || resEmpty[0].Status != audit.StatusPass {
			t.Errorf("unexpected result with empty policies")
		}
	})

	// 4c. AR-ACC-004 works when unrelated valid policies are supplied
	t.Run("EvaluationContext with unrelated valid policies", func(t *testing.T) {
		polList := []audit.ProjectPolicyAssignment{
			newBasePolicy("pol:1", snap.AuditRunID, audit.PolicyScopeSite, "site", audit.PolicyKeyPreferredOrigin, "https://example.com"),
			newBasePolicy("pol:2", snap.AuditRunID, audit.PolicyScopeURL, "url:audit:run:pol:test:1:1", audit.PolicyKeyExpectedIndexable, "true"),
		}
		policyIdx, err := engine.NewPolicyIndex(polList)
		if err != nil {
			t.Fatalf("NewPolicyIndex failed: %v", err)
		}

		resWithPol, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: policyIdx,
		}, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext failed with valid policies: %v", err)
		}
		if len(resWithPol) != 1 || resWithPol[0].Status != audit.StatusPass {
			t.Errorf("unexpected result with unrelated policies")
		}
	})

	// 4d. AuditRunID mismatch between policies and snapshot prevents evaluation
	t.Run("mismatched AuditRunID between policy and snapshot prevents evaluation", func(t *testing.T) {
		polList := []audit.ProjectPolicyAssignment{
			newBasePolicy("pol:1", "other:run:id", audit.PolicyScopeSite, "site", audit.PolicyKeyPreferredOrigin, "https://example.com"),
		}
		policyIdx, err := engine.NewPolicyIndex(polList)
		if err != nil {
			t.Fatalf("NewPolicyIndex failed: %v", err)
		}

		_, err = eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
			Snapshot: snap,
			Policies: policyIdx,
		}, "AR-ACC-004")
		if err == nil {
			t.Fatalf("expected error for mismatched policy AuditRunID, got nil")
		}
		if !errors.Is(err, engine.ErrPolicyRunMismatch) {
			t.Errorf("expected ErrPolicyRunMismatch, got %v", err)
		}
	})

	// 4e. Implemented rule set remains strictly ["AR-ACC-004", "AR-CANON-003", "AR-CANON-004", "AR-CANON-006", "AR-CANON-007", "AR-INDEX-001", "AR-INDEX-002"]
	t.Run("implemented rules remain strictly AR-ACC-004, AR-CANON-003, AR-CANON-004, AR-CANON-006, AR-CANON-007, AR-INDEX-001, and AR-INDEX-002", func(t *testing.T) {
		ids := eng.ImplementedRuleIDs()
		expected := []string{"AR-ACC-004", "AR-CANON-003", "AR-CANON-004", "AR-CANON-006", "AR-CANON-007", "AR-INDEX-001", "AR-INDEX-002"}
		if len(ids) != len(expected) {
			t.Fatalf("expected exactly %v, got %v", expected, ids)
		}
		for i, exp := range expected {
			if ids[i] != exp {
				t.Fatalf("at index %d: expected %q, got %q", i, exp, ids[i])
			}
		}
	})
}
