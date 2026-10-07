package engine_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

// Helper to construct a valid base frozen snapshot for engine testing.
func newValidFrozenSnapshot() *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	return &audit.EvidenceSnapshot{
		SnapshotID:           "snap:test:1",
		AuditRunID:           "audit:run:1",
		SnapshotStatus:       audit.SnapshotFrozen,
		NormalizationVersion: "v1.2.0",
		CrawlComplete:        true,
		CreatedAt:            now.Add(-1 * time.Minute),
		FrozenAt:             &now,
		NormalizedObservations: []audit.NormalizedObservation{
			{
				ObservationID:      "obs:test:1",
				AuditRunID:         "audit:run:1",
				SnapshotID:         "snap:test:1",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         "url:audit:run:1:1",
				Field:              "url_identity",
				Value:              "https://example.com/",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:run:1"},
				ObservedAt:         now,
			},
			{
				ObservationID:      "obs:test:2",
				AuditRunID:         "audit:run:1",
				SnapshotID:         "snap:test:1",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         "url:audit:run:1:1",
				Field:              "http_status",
				Value:              "200",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:run:1"},
				ObservedAt:         now,
			},
		},
	}
}

// 1. Snapshot validation guards
func TestEngine_SnapshotValidationGuards(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	// 1a. Nil snapshot
	_, err = eng.EvaluateRule(ctx, nil, "AR-ACC-004")
	if !errors.Is(err, engine.ErrNilSnapshot) {
		t.Errorf("expected ErrNilSnapshot, got %v", err)
	}

	// 1b. BUILDING snapshot
	snapBuilding := newValidFrozenSnapshot()
	snapBuilding.SnapshotStatus = audit.SnapshotBuilding
	_, err = eng.EvaluateRule(ctx, snapBuilding, "AR-ACC-004")
	if !errors.Is(err, engine.ErrSnapshotNotFrozen) {
		t.Errorf("expected ErrSnapshotNotFrozen, got %v", err)
	}

	// 1c. Missing FrozenAt
	snapNoFrozenAt := newValidFrozenSnapshot()
	snapNoFrozenAt.FrozenAt = nil
	_, err = eng.EvaluateRule(ctx, snapNoFrozenAt, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidSnapshot) {
		t.Errorf("expected ErrInvalidSnapshot for nil FrozenAt, got %v", err)
	}

	// 1d. Empty SnapshotID
	snapNoID := newValidFrozenSnapshot()
	snapNoID.SnapshotID = ""
	_, err = eng.EvaluateRule(ctx, snapNoID, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidSnapshot) {
		t.Errorf("expected ErrInvalidSnapshot for empty SnapshotID, got %v", err)
	}

	// 1e. Empty AuditRunID
	snapNoRunID := newValidFrozenSnapshot()
	snapNoRunID.AuditRunID = ""
	_, err = eng.EvaluateRule(ctx, snapNoRunID, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidSnapshot) {
		t.Errorf("expected ErrInvalidSnapshot for empty AuditRunID, got %v", err)
	}

	// 1f. Empty NormalizationVersion
	snapNoVer := newValidFrozenSnapshot()
	snapNoVer.NormalizationVersion = ""
	_, err = eng.EvaluateRule(ctx, snapNoVer, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidSnapshot) {
		t.Errorf("expected ErrInvalidSnapshot for empty NormalizationVersion, got %v", err)
	}
}

// 2. Observation validation guards
func TestEngine_ObservationValidationGuards(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	// 2a. AuditRunID mismatch in observation
	snapRunMismatch := newValidFrozenSnapshot()
	snapRunMismatch.NormalizedObservations[0].AuditRunID = "other-run-id"
	_, err = eng.EvaluateRule(ctx, snapRunMismatch, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidObservation) {
		t.Errorf("expected ErrInvalidObservation for AuditRunID mismatch, got %v", err)
	}

	// 2b. SnapshotID mismatch in observation
	snapSnapMismatch := newValidFrozenSnapshot()
	snapSnapMismatch.NormalizedObservations[0].SnapshotID = "other-snap-id"
	_, err = eng.EvaluateRule(ctx, snapSnapMismatch, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidObservation) {
		t.Errorf("expected ErrInvalidObservation for SnapshotID mismatch, got %v", err)
	}

	// 2c. Invalid SubjectType
	snapBadSubject := newValidFrozenSnapshot()
	snapBadSubject.NormalizedObservations[0].SubjectType = "INVALID_SUBJECT"
	_, err = eng.EvaluateRule(ctx, snapBadSubject, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidObservation) {
		t.Errorf("expected ErrInvalidObservation for invalid SubjectType, got %v", err)
	}

	// 2d. Invalid DerivationType
	snapBadDerivation := newValidFrozenSnapshot()
	snapBadDerivation.NormalizedObservations[0].DerivationType = "INVALID_DERIVATION"
	_, err = eng.EvaluateRule(ctx, snapBadDerivation, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidObservation) {
		t.Errorf("expected ErrInvalidObservation for invalid DerivationType, got %v", err)
	}

	// 2e. Duplicate ObservationID
	snapDupObs := newValidFrozenSnapshot()
	snapDupObs.NormalizedObservations = append(snapDupObs.NormalizedObservations, audit.NormalizedObservation{
		ObservationID:      snapDupObs.NormalizedObservations[0].ObservationID, // duplicate!
		AuditRunID:         "audit:run:1",
		SnapshotID:         "snap:test:1",
		SubjectType:        audit.SubjectURL,
		SubjectRef:         "url:audit:run:1:2",
		Field:              "http_status",
		Value:              "200",
		DerivationType:     audit.DerivationDirect,
		SourceEvidenceRefs: []string{"sitecrawl_pages:run:1"},
		ObservedAt:         time.Now().UTC(),
	})
	_, err = eng.EvaluateRule(ctx, snapDupObs, "AR-ACC-004")
	if !errors.Is(err, engine.ErrInvalidObservation) {
		t.Errorf("expected ErrInvalidObservation for duplicate ObservationID, got %v", err)
	}
}

// 3. Rule selection behavior
func TestEngine_RuleSelection(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	snap := newValidFrozenSnapshot()

	// 3a. Unknown rule not in registry
	_, err = eng.EvaluateRule(ctx, snap, "AR-UNKNOWN-999")
	if !errors.Is(err, engine.ErrRuleNotFound) {
		t.Errorf("expected ErrRuleNotFound, got %v", err)
	}

	// 3b. Known in registry but unimplemented rule
	unimplementedRules := []string{"AR-ACC-001", "AR-ACC-002", "AR-INDEX-003", "AR-CANON-001"}
	for _, ruleID := range unimplementedRules {
		_, err = eng.EvaluateRule(ctx, snap, ruleID)
		if !errors.Is(err, engine.ErrRuleNotImplemented) {
			t.Errorf("rule %s: expected ErrRuleNotImplemented, got %v", ruleID, err)
		}
	}

	// 3c. Implemented rule AR-ACC-004
	results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-004")
	if err != nil {
		t.Fatalf("expected AR-ACC-004 to execute successfully, got: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// 3d. Implemented rule AR-INDEX-001
	results, err = eng.EvaluateRule(ctx, snap, "AR-INDEX-001")
	if err != nil {
		t.Fatalf("expected AR-INDEX-001 to execute successfully, got: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// 4. Implemented rule set API
func TestEngine_ImplementedRuleIDs(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ids := eng.ImplementedRuleIDs()
	if len(ids) != 3 || ids[0] != "AR-ACC-004" || ids[1] != "AR-INDEX-001" || ids[2] != "AR-INDEX-002" {
		t.Errorf("expected exactly [\"AR-ACC-004\", \"AR-INDEX-001\", \"AR-INDEX-002\"], got %v", ids)
	}
}

// 5. Context cancellation
func TestEngine_ContextCancellation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	snap := newValidFrozenSnapshot()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel context

	_, err = eng.EvaluateRule(ctx, snap, "AR-ACC-004")
	if err == nil {
		t.Fatalf("expected context error on cancelled context, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// 6. Determinism: Same snapshot produces identical results
func TestEngine_EvaluationDeterminism(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	fixedTime := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	eng.SetClock(func() time.Time { return fixedTime })

	snap := newValidFrozenSnapshot()

	// Add multiple URL subjects in non-sorted observation order
	now := time.Now().UTC()
	extraObs := []audit.NormalizedObservation{
		{
			ObservationID:      "obs:test:3",
			AuditRunID:         "audit:run:1",
			SnapshotID:         "snap:test:1",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:audit:run:1:3",
			Field:              "url_identity",
			Value:              "https://example.com/z",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:run:3"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:test:4",
			AuditRunID:         "audit:run:1",
			SnapshotID:         "snap:test:1",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:audit:run:1:3",
			Field:              "http_status",
			Value:              "500",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:run:3"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:test:5",
			AuditRunID:         "audit:run:1",
			SnapshotID:         "snap:test:1",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:audit:run:1:2",
			Field:              "url_identity",
			Value:              "https://example.com/a",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:run:2"},
			ObservedAt:         now,
		},
		{
			ObservationID:      "obs:test:6",
			AuditRunID:         "audit:run:1",
			SnapshotID:         "snap:test:1",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:audit:run:1:2",
			Field:              "http_status",
			Value:              "404",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:run:2"},
			ObservedAt:         now,
		},
	}
	snap.NormalizedObservations = append(snap.NormalizedObservations, extraObs...)

	snap1 := snap

	// Construct snap2 with the same observations in reversed order to assert order independence
	snap2 := *snap
	reversedObs := make([]audit.NormalizedObservation, len(snap.NormalizedObservations))
	for i, o := range snap.NormalizedObservations {
		reversedObs[len(snap.NormalizedObservations)-1-i] = o
	}
	snap2.NormalizedObservations = reversedObs

	ctx := context.Background()

	res1, err := eng.EvaluateRule(ctx, snap1, "AR-ACC-004")
	if err != nil {
		t.Fatalf("evaluation on snap1 failed: %v", err)
	}

	res2, err := eng.EvaluateRule(ctx, &snap2, "AR-ACC-004")
	if err != nil {
		t.Fatalf("evaluation on snap2 (reversed observation order) failed: %v", err)
	}

	if len(res1) != len(res2) {
		t.Fatalf("results length mismatch: %d vs %d", len(res1), len(res2))
	}

	for i := range res1 {
		r1 := res1[i]
		r2 := res2[i]

		if r1.RuleResultID != r2.RuleResultID {
			t.Errorf("item %d RuleResultID mismatch: %q vs %q", i, r1.RuleResultID, r2.RuleResultID)
		}
		if r1.SubjectRef != r2.SubjectRef {
			t.Errorf("item %d SubjectRef mismatch: %q vs %q", i, r1.SubjectRef, r2.SubjectRef)
		}
		if r1.Status != r2.Status {
			t.Errorf("item %d Status mismatch: %q vs %q", i, r1.Status, r2.Status)
		}
		if r1.Severity != r2.Severity {
			t.Errorf("item %d Severity mismatch: %q vs %q", i, r1.Severity, r2.Severity)
		}
		if r1.ObservedSummary != r2.ObservedSummary {
			t.Errorf("item %d ObservedSummary mismatch: %q vs %q", i, r1.ObservedSummary, r2.ObservedSummary)
		}
		if !r1.EvaluatedAt.Equal(r2.EvaluatedAt) {
			t.Errorf("item %d EvaluatedAt mismatch: %v vs %v", i, r1.EvaluatedAt, r2.EvaluatedAt)
		}
		if len(r1.EvidenceRefs) != len(r2.EvidenceRefs) {
			t.Fatalf("item %d EvidenceRefs length mismatch", i)
		}
		for j := range r1.EvidenceRefs {
			ref1 := r1.EvidenceRefs[j]
			ref2 := r2.EvidenceRefs[j]
			if ref1.RuleEvidenceRefID != ref2.RuleEvidenceRefID ||
				ref1.Field != ref2.Field ||
				ref1.ObservedValue != ref2.ObservedValue ||
				ref1.Role != ref2.Role {
				t.Errorf("item %d evidence ref %d mismatch: %+v vs %+v", i, j, ref1, ref2)
			}
		}
	}
}

// 7. Registry drift guard for AR-ACC-004
func TestEngine_RegistryDriftGuardACC004(t *testing.T) {
	reg, err := audit.LoadV1Registry()
	if err != nil {
		t.Fatalf("failed to load registry: %v", err)
	}

	def, exists := reg.Get("AR-ACC-004")
	if !exists {
		t.Fatalf("AR-ACC-004 missing from V1 registry")
	}

	if def.RuleID != "AR-ACC-004" {
		t.Errorf("expected RuleID AR-ACC-004, got %q", def.RuleID)
	}
	if def.ParentCheck != "ACC-007" {
		t.Errorf("expected ParentCheck ACC-007, got %q", def.ParentCheck)
	}
	if def.Automation != audit.AutomationDeterministic {
		t.Errorf("expected Automation deterministic, got %q", def.Automation)
	}
	if def.DefaultSeverity != audit.SeverityP1 {
		t.Errorf("expected DefaultSeverity P1, got %q", def.DefaultSeverity)
	}
	if def.RuleVersion != 1 {
		t.Errorf("expected RuleVersion 1, got %d", def.RuleVersion)
	}

	hasURL := false
	hasFetchStatus := false
	for _, in := range def.RequiredInputs {
		if in == "url" {
			hasURL = true
		}
		if in == "fetch_status" {
			hasFetchStatus = true
		}
	}
	if !hasURL || !hasFetchStatus {
		t.Errorf("expected required_inputs to contain 'url' and 'fetch_status', got %v", def.RequiredInputs)
	}
}
