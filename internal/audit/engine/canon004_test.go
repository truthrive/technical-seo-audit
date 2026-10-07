package engine_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

type canon004TestFixture struct {
	subjectRef       string
	urlValue         string
	urlValues        []string
	countValue       string
	countValues      []string
	completeValue    string
	completeValues   []string
	distinctValue    string
	distinctValues   []string
	normTargetValues []string
}

func newCANON004Snapshot(fixtures []canon004TestFixture, auditRunID audit.AuditRunID, snapID audit.SnapshotID) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	for _, f := range fixtures {
		// 1. URL identity
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

		// 2. canonical_count
		if len(f.countValues) > 0 {
			for i, c := range f.countValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:count:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "canonical_count",
					Value:              c,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.countValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:count:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "canonical_count",
				Value:              f.countValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// 3. canonical_normalization_complete
		if len(f.completeValues) > 0 {
			for i, c := range f.completeValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:complete:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "canonical_normalization_complete",
					Value:              c,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.completeValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:complete:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "canonical_normalization_complete",
				Value:              f.completeValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// 4. canonical_distinct_normalized_count
		if len(f.distinctValues) > 0 {
			for i, d := range f.distinctValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:distinct:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "canonical_distinct_normalized_count",
					Value:              d,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.distinctValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:distinct:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "canonical_distinct_normalized_count",
				Value:              f.distinctValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// 5. canonical_normalized_target
		for i, nt := range f.normTargetValues {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:norm_target:%s:%d", f.subjectRef, i)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "canonical_normalized_target",
				Value:              nt,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             snapID,
		AuditRunID:             auditRunID,
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.5.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-10 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_CANON_004_RequiredCases(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	auditRunID := audit.AuditRunID("audit:run:canon004")
	snapID := audit.SnapshotID("snap:canon004")

	// 1. count=0 -> N/A
	t.Run("1. count=0 -> N/A", func(t *testing.T) {
		snap := newCANON004Snapshot([]canon004TestFixture{
			{
				subjectRef: "url:test:1",
				urlValue:   "https://example.com/page1",
				countValue: "0",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected NOT_APPLICABLE, got %s", results[0].Status)
		}
	})

	// 2. count=1 -> PASS
	t.Run("2. count=1 -> PASS", func(t *testing.T) {
		snap := newCANON004Snapshot([]canon004TestFixture{
			{
				subjectRef:       "url:test:1",
				urlValue:         "https://example.com/page1",
				countValue:       "1",
				normTargetValues: []string{"https://example.com/target1"},
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", results[0].Status)
		}
	})

	// 3. duplicate declarations same target -> WARNING
	t.Run("3. duplicate declarations same target -> WARNING", func(t *testing.T) {
		snap := newCANON004Snapshot([]canon004TestFixture{
			{
				subjectRef:       "url:test:1",
				urlValue:         "https://example.com/page1",
				countValue:       "2",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValues: []string{"https://example.com/target", "https://example.com/target"},
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusWarning {
			t.Errorf("expected WARNING, got %s", results[0].Status)
		}
	})

	// 4. multiple distinct targets -> FAIL
	t.Run("4. multiple distinct targets -> FAIL", func(t *testing.T) {
		snap := newCANON004Snapshot([]canon004TestFixture{
			{
				subjectRef:       "url:test:1",
				urlValue:         "https://example.com/page1",
				countValue:       "2",
				completeValue:    "true",
				distinctValue:    "2",
				normTargetValues: []string{"https://example.com/target1", "https://example.com/target2"},
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", results[0].Status)
		}
	})

	// 5. multiple + normalization incomplete -> UNKNOWN
	t.Run("5. multiple + normalization incomplete -> UNKNOWN", func(t *testing.T) {
		snap := newCANON004Snapshot([]canon004TestFixture{
			{
				subjectRef:       "url:test:1",
				urlValue:         "https://example.com/page1",
				countValue:       "2",
				completeValue:    "false",
				normTargetValues: []string{"https://example.com/target1"},
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", results[0].Status)
		}
	})

	// 6. malformed count -> UNKNOWN
	t.Run("6. malformed count -> UNKNOWN", func(t *testing.T) {
		snap := newCANON004Snapshot([]canon004TestFixture{
			{
				subjectRef: "url:test:1",
				urlValue:   "https://example.com/page1",
				countValue: "invalid_count",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", results[0].Status)
		}
	})

	// 7. conflicting count -> UNKNOWN
	t.Run("7. conflicting count -> UNKNOWN", func(t *testing.T) {
		snap := newCANON004Snapshot([]canon004TestFixture{
			{
				subjectRef:  "url:test:1",
				urlValue:    "https://example.com/page1",
				countValues: []string{"1", "2"},
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", results[0].Status)
		}
	})

	// 8. malformed distinct count -> UNKNOWN
	t.Run("8. malformed distinct count -> UNKNOWN", func(t *testing.T) {
		snap := newCANON004Snapshot([]canon004TestFixture{
			{
				subjectRef:    "url:test:1",
				urlValue:      "https://example.com/page1",
				countValue:    "2",
				completeValue: "true",
				distinctValue: "not_a_number",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", results[0].Status)
		}
	})

	// 9. reversed observation order deterministic
	t.Run("9. reversed observation order deterministic", func(t *testing.T) {
		f := canon004TestFixture{
			subjectRef:       "url:test:1",
			urlValue:         "https://example.com/page1",
			countValue:       "2",
			completeValue:    "true",
			distinctValue:    "1",
			normTargetValues: []string{"https://example.com/target", "https://example.com/target"},
		}
		snapFwd := newCANON004Snapshot([]canon004TestFixture{f}, auditRunID, snapID)

		// Create reverse order snapshot
		snapRev := newCANON004Snapshot([]canon004TestFixture{f}, auditRunID, snapID)
		n := len(snapRev.NormalizedObservations)
		for i := 0; i < n/2; i++ {
			snapRev.NormalizedObservations[i], snapRev.NormalizedObservations[n-1-i] = snapRev.NormalizedObservations[n-1-i], snapRev.NormalizedObservations[i]
		}

		resFwd, err := eng.EvaluateRule(ctx, snapFwd, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule fwd failed: %v", err)
		}
		resRev, err := eng.EvaluateRule(ctx, snapRev, "AR-CANON-004")
		if err != nil {
			t.Fatalf("EvaluateRule rev failed: %v", err)
		}

		if !reflect.DeepEqual(resFwd, resRev) {
			t.Errorf("expected deterministic identical results regardless of observation ordering")
		}
	})
}
