package engine_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

type index001TestFixture struct {
	subjectRef      string
	urlValue        string
	urlValues       []string
	effectiveValue  string
	effectiveValues []string
	policyValue     string // "true", "false", "unspecified", or "" (missing)
	hasDirective    bool
}

func newINDEX001Snapshot(fixtures []index001TestFixture, auditRunID audit.AuditRunID, snapID audit.SnapshotID) (*audit.EvidenceSnapshot, *engine.PolicyIndex, error) {
	now := time.Now().UTC()
	var (
		obs      []audit.NormalizedObservation
		policies []audit.ProjectPolicyAssignment
	)

	for _, f := range fixtures {
		// 1. URL identity observations
		if len(f.urlValues) > 0 {
			for i, u := range f.urlValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:url:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "url_identity",
					Value:              u,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.urlValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:url:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "url_identity",
				Value:              f.urlValue,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// 2. effective_noindex observations
		if len(f.effectiveValues) > 0 {
			for i, ev := range f.effectiveValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:eff:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "effective_noindex",
					Value:              ev,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"page:test"},
					ObservedAt:         now,
				})
			}
		} else if f.effectiveValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:eff:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "effective_noindex",
				Value:              f.effectiveValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"page:test"},
				ObservedAt:         now,
			})
		}

		// 3. Optional underlying directive observations
		if f.hasDirective {
			dRef := fmt.Sprintf("directive:%s:1", f.subjectRef)
			refs := []string{dRef, "page:test"}
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "directive_source",
				Value:              "Meta",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: refs,
				ObservedAt:         now,
			})
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "directive_target",
				Value:              "*",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: refs,
				ObservedAt:         now,
			})
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:raw:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "directive_raw",
				Value:              "noindex, follow",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: refs,
				ObservedAt:         now,
			})
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tok:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "directive_tokens",
				Value:              "noindex, follow",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: refs,
				ObservedAt:         now,
			})
		}

		// 4. Policy assignment
		if f.policyValue != "" {
			policies = append(policies, audit.ProjectPolicyAssignment{
				PolicyAssignmentID: audit.PolicyAssignmentID(fmt.Sprintf("pol:%s:exp_idx", f.subjectRef)),
				AuditRunID:         auditRunID,
				PolicyKey:          audit.PolicyKeyExpectedIndexable,
				PolicyValue:        f.policyValue,
				Scope:              audit.PolicyScopeURL,
				TargetRef:          f.subjectRef,
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         now,
			})
		}
	}

	snap := &audit.EvidenceSnapshot{
		SnapshotID:             snapID,
		AuditRunID:             auditRunID,
		CreatedAt:              now,
		FrozenAt:               &now,
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.4.0",
		NormalizedObservations: obs,
	}

	polIdx, err := engine.NewPolicyIndexForRun(auditRunID, policies)
	if err != nil {
		return nil, nil, err
	}

	return snap, polIdx, nil
}

func TestAR_INDEX_001_RequiredCases(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to initialize engine: %v", err)
	}

	auditRunID := audit.AuditRunID("audit:run:index001:req")
	snapID := audit.SnapshotID("snap:run:index001:req")

	// 1. no policy -> NOT_APPLICABLE
	t.Run("1. no policy -> NOT_APPLICABLE", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:1",
				urlValue:       "https://example.com/1",
				effectiveValue: "false",
				policyValue:    "", // no policy
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected NOT_APPLICABLE, got %s", res[0].Status)
		}
		// Confirm no synthetic policy ref
		for _, ref := range res[0].EvidenceRefs {
			if ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" {
				t.Errorf("expected no policy evidence ref when policy is absent")
			}
		}
	})

	// 2. policy=false -> NOT_APPLICABLE
	t.Run("2. policy=false -> NOT_APPLICABLE", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:2",
				urlValue:       "https://example.com/2",
				effectiveValue: "true",
				policyValue:    "false",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected NOT_APPLICABLE, got %s", res[0].Status)
		}
		hasPolRef := false
		for _, ref := range res[0].EvidenceRefs {
			if ref.Field == "expected_indexable" && ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" && ref.ObservedValue == "false" {
				hasPolRef = true
			}
		}
		if !hasPolRef {
			t.Errorf("expected policy evidence ref for expected_indexable=false")
		}
	})

	// 3. policy=unspecified -> NOT_APPLICABLE
	t.Run("3. policy=unspecified -> NOT_APPLICABLE", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:3",
				urlValue:       "https://example.com/3",
				effectiveValue: "true",
				policyValue:    "unspecified",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected NOT_APPLICABLE, got %s", res[0].Status)
		}
		hasPolRef := false
		for _, ref := range res[0].EvidenceRefs {
			if ref.Field == "expected_indexable" && ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" && ref.ObservedValue == "unspecified" {
				hasPolRef = true
			}
		}
		if !hasPolRef {
			t.Errorf("expected policy evidence ref for expected_indexable=unspecified")
		}
	})

	// 4. policy=true + effective=false -> PASS
	t.Run("4. policy=true + effective=false -> PASS", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:4",
				urlValue:       "https://example.com/4",
				effectiveValue: "false",
				policyValue:    "true",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", res[0].Status)
		}

		var hasURL, hasPol, hasEff bool
		for _, ref := range res[0].EvidenceRefs {
			if ref.Field == "url" && ref.Role == audit.EvidenceRoleContext {
				hasURL = true
			}
			if ref.Field == "expected_indexable" && ref.Role == audit.EvidenceRoleContext && ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" {
				hasPol = true
			}
			if ref.Field == "effective_noindex" && ref.Role == audit.EvidenceRolePrimary && ref.ObservedValue == "false" {
				hasEff = true
			}
		}
		if !hasURL || !hasPol || !hasEff {
			t.Errorf("expected URL context (%v), policy context (%v), effective_noindex primary (%v)", hasURL, hasPol, hasEff)
		}
	})

	// 5. policy=true + effective=true -> FAIL
	t.Run("5. policy=true + effective=true -> FAIL", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:5",
				urlValue:       "https://example.com/5",
				effectiveValue: "true",
				policyValue:    "true",
				hasDirective:    true,
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", res[0].Status)
		}

		var hasURL, hasPol, hasEff, hasDirSupporting bool
		for _, ref := range res[0].EvidenceRefs {
			if ref.Field == "url" && ref.Role == audit.EvidenceRoleContext {
				hasURL = true
			}
			if ref.Field == "expected_indexable" && ref.Role == audit.EvidenceRoleContext && ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" {
				hasPol = true
			}
			if ref.Field == "effective_noindex" && ref.Role == audit.EvidenceRolePrimary && ref.ObservedValue == "true" {
				hasEff = true
			}
			if ref.Role == audit.EvidenceRoleSupporting {
				hasDirSupporting = true
			}
		}
		if !hasURL || !hasPol || !hasEff || !hasDirSupporting {
			t.Errorf("expected URL (%v), policy (%v), effective_noindex (%v), directive supporting (%v)", hasURL, hasPol, hasEff, hasDirSupporting)
		}
	})

	// 6. policy=true + effective missing -> UNKNOWN
	t.Run("6. policy=true + effective missing -> UNKNOWN", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:6",
				urlValue:       "https://example.com/6",
				effectiveValue: "", // missing
				policyValue:    "true",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", res[0].Status)
		}
	})

	// 7. malformed effective value -> UNKNOWN
	t.Run("7. malformed effective value -> UNKNOWN", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:7",
				urlValue:       "https://example.com/7",
				effectiveValue: "invalid_val",
				policyValue:    "true",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", res[0].Status)
		}
	})

	// 8. conflicting effective values -> UNKNOWN
	t.Run("8. conflicting effective values -> UNKNOWN", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:      "url:8",
				urlValue:        "https://example.com/8",
				effectiveValues: []string{"true", "false"},
				policyValue:     "true",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", res[0].Status)
		}
	})

	// 9. missing URL identity -> UNKNOWN
	t.Run("9. missing URL identity -> UNKNOWN", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:9",
				urlValue:       "", // missing URL identity
				effectiveValue: "true",
				policyValue:    "true",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", res[0].Status)
		}
	})

	// 10. conflicting URL identity -> UNKNOWN
	t.Run("10. conflicting URL identity -> UNKNOWN", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:10",
				urlValues:      []string{"https://example.com/10a", "https://example.com/10b"},
				effectiveValue: "true",
				policyValue:    "true",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", res[0].Status)
		}
	})

	// 11. policy-only target + true -> UNKNOWN
	t.Run("11. policy-only target + true -> UNKNOWN", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:  "url:orphan",
				policyValue: "true",
				// zero observations in snapshot for this subjectRef
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result for policy target, got %d", len(res))
		}
		if res[0].SubjectRef != "url:orphan" {
			t.Errorf("expected subject_ref url:orphan, got %s", res[0].SubjectRef)
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for policy-only target, got %s", res[0].Status)
		}
	})

	// 12. EvaluateRule without policies -> N/A
	t.Run("12. EvaluateRule without policies -> N/A", func(t *testing.T) {
		snap, _, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:12a",
				urlValue:       "https://example.com/12a",
				effectiveValue: "true",
			},
			{
				subjectRef:     "url:12b",
				urlValue:       "https://example.com/12b",
				effectiveValue: "false",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		// Backward-compatible EvaluateRule provides empty PolicyIndex
		res, err := eng.EvaluateRule(context.Background(), snap, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res))
		}
		for _, r := range res {
			if r.Status != audit.StatusNotApplicable {
				t.Errorf("expected NOT_APPLICABLE without policies, got %s for %s", r.Status, r.SubjectRef)
			}
		}
	})

	// 13. policy provenance appears in refs
	t.Run("13. policy provenance appears in refs", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:13",
				urlValue:       "https://example.com/13",
				effectiveValue: "false",
				policyValue:    "true",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}

		foundPol := false
		for _, ref := range res[0].EvidenceRefs {
			if ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" &&
				ref.EvidenceRef == "pol:url:13:exp_idx" &&
				ref.Field == "expected_indexable" &&
				ref.Role == audit.EvidenceRoleContext {
				foundPol = true
			}
		}
		if !foundPol {
			t.Errorf("expected policy provenance in evidence refs")
		}
	})

	// 14. PASS/FAIL include effective state ref
	t.Run("14. PASS/FAIL include effective state ref", func(t *testing.T) {
		snap, polIdx, err := newINDEX001Snapshot([]index001TestFixture{
			{
				subjectRef:     "url:14_pass",
				urlValue:       "https://example.com/14_pass",
				effectiveValue: "false",
				policyValue:    "true",
			},
			{
				subjectRef:     "url:14_fail",
				urlValue:       "https://example.com/14_fail",
				effectiveValue: "true",
				policyValue:    "true",
			},
		}, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		res, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("evaluation failed: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res))
		}

		for _, r := range res {
			hasEffRef := false
			for _, ref := range r.EvidenceRefs {
				if ref.EvidenceType == "NORMALIZED_OBSERVATION" &&
					ref.Field == "effective_noindex" &&
					ref.Role == audit.EvidenceRolePrimary {
					hasEffRef = true
				}
			}
			if !hasEffRef {
				t.Errorf("%s (status %s) missing primary effective_noindex ref", r.SubjectRef, r.Status)
			}
		}
	})

	// 15. reversed input order is deterministic
	t.Run("15. reversed input order is deterministic", func(t *testing.T) {
		fixturesForward := []index001TestFixture{
			{subjectRef: "url:15a", urlValue: "https://example.com/15a", effectiveValue: "true", policyValue: "true"},
			{subjectRef: "url:15b", urlValue: "https://example.com/15b", effectiveValue: "false", policyValue: "true"},
			{subjectRef: "url:15c", urlValue: "https://example.com/15c", effectiveValue: "false", policyValue: "false"},
			{subjectRef: "url:15d", urlValue: "https://example.com/15d", effectiveValue: "true", policyValue: ""},
		}
		fixturesReverse := []index001TestFixture{
			{subjectRef: "url:15d", urlValue: "https://example.com/15d", effectiveValue: "true", policyValue: ""},
			{subjectRef: "url:15c", urlValue: "https://example.com/15c", effectiveValue: "false", policyValue: "false"},
			{subjectRef: "url:15b", urlValue: "https://example.com/15b", effectiveValue: "false", policyValue: "true"},
			{subjectRef: "url:15a", urlValue: "https://example.com/15a", effectiveValue: "true", policyValue: "true"},
		}

		fixedTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		eng.SetClock(func() time.Time { return fixedTime })

		snapFwd, polFwd, err := newINDEX001Snapshot(fixturesForward, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup fwd failed: %v", err)
		}
		resFwd, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snapFwd, Policies: polFwd}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("eval fwd failed: %v", err)
		}

		snapRev, polRev, err := newINDEX001Snapshot(fixturesReverse, auditRunID, snapID)
		if err != nil {
			t.Fatalf("setup rev failed: %v", err)
		}
		// Also reverse the NormalizedObservations slice inside snapshot
		for i, j := 0, len(snapRev.NormalizedObservations)-1; i < j; i, j = i+1, j-1 {
			snapRev.NormalizedObservations[i], snapRev.NormalizedObservations[j] = snapRev.NormalizedObservations[j], snapRev.NormalizedObservations[i]
		}

		resRev, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snapRev, Policies: polRev}, "AR-INDEX-001")
		if err != nil {
			t.Fatalf("eval rev failed: %v", err)
		}

		if len(resFwd) != len(resRev) {
			t.Fatalf("result length mismatch: fwd=%d, rev=%d", len(resFwd), len(resRev))
		}

		for i := range resFwd {
			if !reflect.DeepEqual(resFwd[i], resRev[i]) {
				t.Errorf("result %d mismatch between forward and reversed order:\nfwd: %+v\nrev: %+v", i, resFwd[i], resRev[i])
			}
		}
	})
}

// Hermetic adapter -> PolicyIndex -> engine integration test
func TestEngine_AR_INDEX_001_HermeticIntegration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home</title></head>
<body><nav>
<a href="/indexed">Indexed</a>
<a href="/blocked">Blocked</a>
<a href="/unspecified">Unspecified</a>
<a href="/not-expected">Not Expected</a>
</nav><h1>Home</h1></body></html>`)
	})
	mux.HandleFunc("/indexed", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><meta name="robots" content="index, follow"><title>Indexed</title></head><body><h1>Indexed</h1></body></html>`)
	})
	mux.HandleFunc("/blocked", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><meta name="robots" content="noindex, follow"><title>Blocked</title></head><body><h1>Blocked</h1></body></html>`)
	})
	mux.HandleFunc("/unspecified", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><meta name="robots" content="noindex"><title>Unspecified</title></head><body><h1>Unspecified</h1></body></html>`)
	})
	mux.HandleFunc("/not-expected", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><meta name="robots" content="noindex"><title>Not Expected</title></head><body><h1>Not Expected</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Crawl
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	runner := sitecrawl.NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("failed to ensure schema: %v", err)
	}

	opts := sitecrawl.Options{
		Mode:        sitecrawl.ModeSpider,
		MaxDepth:    2,
		MaxURLs:     10,
		Concurrency: 1,
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("crawl execution failed: %v", err)
	}

	// 2. Build frozen snapshot via Adapter
	auditRunID := audit.AuditRunID("audit:run:hermetic:index001")
	snapID := audit.SnapshotID("snap:run:hermetic:index001")
	buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}
	snap := buildRes.EvidenceSnapshot

	// Map URL string to subject_ref
	subjByURL := make(map[string]string)
	for _, obs := range snap.NormalizedObservations {
		if obs.Field == "url_identity" {
			subjByURL[obs.Value] = obs.SubjectRef
		}
	}

	// 3. Define explicit policies for each URL
	indexedSubj := subjByURL[srv.URL+"/indexed"]
	blockedSubj := subjByURL[srv.URL+"/blocked"]
	notExpSubj := subjByURL[srv.URL+"/not-expected"]
	// /unspecified will have NO policy

	now := time.Now().UTC()
	var policies []audit.ProjectPolicyAssignment

	if indexedSubj != "" {
		policies = append(policies, audit.ProjectPolicyAssignment{
			PolicyAssignmentID: "pol:hermetic:indexed",
			AuditRunID:         auditRunID,
			PolicyKey:          audit.PolicyKeyExpectedIndexable,
			PolicyValue:        "true",
			Scope:              audit.PolicyScopeURL,
			TargetRef:          indexedSubj,
			Source:             audit.PolicyProvenanceUserInput,
			SuppliedAt:         now,
		})
	}
	if blockedSubj != "" {
		policies = append(policies, audit.ProjectPolicyAssignment{
			PolicyAssignmentID: "pol:hermetic:blocked",
			AuditRunID:         auditRunID,
			PolicyKey:          audit.PolicyKeyExpectedIndexable,
			PolicyValue:        "true",
			Scope:              audit.PolicyScopeURL,
			TargetRef:          blockedSubj,
			Source:             audit.PolicyProvenanceUserInput,
			SuppliedAt:         now,
		})
	}
	if notExpSubj != "" {
		policies = append(policies, audit.ProjectPolicyAssignment{
			PolicyAssignmentID: "pol:hermetic:not-exp",
			AuditRunID:         auditRunID,
			PolicyKey:          audit.PolicyKeyExpectedIndexable,
			PolicyValue:        "false",
			Scope:              audit.PolicyScopeURL,
			TargetRef:          notExpSubj,
			Source:             audit.PolicyProvenanceUserInput,
			SuppliedAt:         now,
		})
	}

	polIdx, err := engine.NewPolicyIndexForRun(auditRunID, policies)
	if err != nil {
		t.Fatalf("NewPolicyIndexForRun failed: %v", err)
	}

	// 4. Evaluate AR-INDEX-001
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	results, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{
		Snapshot: snap,
		Policies: polIdx,
	}, "AR-INDEX-001")
	if err != nil {
		t.Fatalf("EvaluateRuleWithContext failed: %v", err)
	}

	resultsBySubj := make(map[string]audit.RuleResult)
	for _, r := range results {
		resultsBySubj[r.SubjectRef] = r
	}

	// Verify /indexed: policy=true, effective_noindex=false -> PASS
	if r, ok := resultsBySubj[indexedSubj]; !ok {
		t.Errorf("missing /indexed result")
	} else if r.Status != audit.StatusPass {
		t.Errorf("expected /indexed PASS, got %s", r.Status)
	}

	// Verify /blocked: policy=true, effective_noindex=true -> FAIL
	if r, ok := resultsBySubj[blockedSubj]; !ok {
		t.Errorf("missing /blocked result")
	} else if r.Status != audit.StatusFail {
		t.Errorf("expected /blocked FAIL, got %s", r.Status)
	}

	// Verify /unspecified: no policy -> NOT_APPLICABLE
	unspecSubj := subjByURL[srv.URL+"/unspecified"]
	if r, ok := resultsBySubj[unspecSubj]; !ok {
		t.Errorf("missing /unspecified result")
	} else if r.Status != audit.StatusNotApplicable {
		t.Errorf("expected /unspecified NOT_APPLICABLE, got %s", r.Status)
	}

	// Verify /not-expected: policy=false -> NOT_APPLICABLE
	if r, ok := resultsBySubj[notExpSubj]; !ok {
		t.Errorf("missing /not-expected result")
	} else if r.Status != audit.StatusNotApplicable {
		t.Errorf("expected /not-expected NOT_APPLICABLE, got %s", r.Status)
	}
}
