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

type canon006TestFixture struct {
	sourceSubjectRef string
	sourceURL        string
	sourceURLs       []string
	countValue       string
	countValues      []string
	completeValue    string
	completeValues   []string
	distinctValue    string
	distinctValues   []string
	normTargetValue  string
	targetSubjRef    string
	targetSubjRefs   []string

	// Target URL observations
	targetSubjectRef string
	targetURL        string
	targetURLs       []string
	targetStatus     string
	targetStatuses   []string
}

func newCANON006Snapshot(fixtures []canon006TestFixture, auditRunID audit.AuditRunID, snapID audit.SnapshotID) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	for _, f := range fixtures {
		// --- Source Observations ---
		if len(f.sourceURLs) > 0 {
			for i, u := range f.sourceURLs {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_url:%s:%d", f.sourceSubjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.sourceSubjectRef,
					Field:              "url_identity",
					Value:              u,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.sourceURL != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_url:%s", f.sourceSubjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.sourceSubjectRef,
				Field:              "url_identity",
				Value:              f.sourceURL,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		if len(f.countValues) > 0 {
			for i, c := range f.countValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:count:%s:%d", f.sourceSubjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.sourceSubjectRef,
					Field:              "canonical_count",
					Value:              c,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.countValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:count:%s", f.sourceSubjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.sourceSubjectRef,
				Field:              "canonical_count",
				Value:              f.countValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		if len(f.completeValues) > 0 {
			for i, c := range f.completeValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:complete:%s:%d", f.sourceSubjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.sourceSubjectRef,
					Field:              "canonical_normalization_complete",
					Value:              c,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.completeValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:complete:%s", f.sourceSubjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.sourceSubjectRef,
				Field:              "canonical_normalization_complete",
				Value:              f.completeValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		if len(f.distinctValues) > 0 {
			for i, d := range f.distinctValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:distinct:%s:%d", f.sourceSubjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.sourceSubjectRef,
					Field:              "canonical_distinct_normalized_count",
					Value:              d,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.distinctValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:distinct:%s", f.sourceSubjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.sourceSubjectRef,
				Field:              "canonical_distinct_normalized_count",
				Value:              f.distinctValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		if f.normTargetValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:norm_target:%s", f.sourceSubjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.sourceSubjectRef,
				Field:              "canonical_normalized_target",
				Value:              f.normTargetValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		if len(f.targetSubjRefs) > 0 {
			for i, r := range f.targetSubjRefs {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:target_ref:%s:%d", f.sourceSubjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.sourceSubjectRef,
					Field:              "canonical_target_subject_ref",
					Value:              r,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.targetSubjRef != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:target_ref:%s", f.sourceSubjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.sourceSubjectRef,
				Field:              "canonical_target_subject_ref",
				Value:              f.targetSubjRef,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// --- Target Observations ---
		if f.targetSubjectRef != "" {
			if len(f.targetURLs) > 0 {
				for i, u := range f.targetURLs {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_url:%s:%d", f.targetSubjectRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         f.targetSubjectRef,
						Field:              "url_identity",
						Value:              u,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
						ObservedAt:         now,
					})
				}
			} else if f.targetURL != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_url:%s", f.targetSubjectRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.targetSubjectRef,
					Field:              "url_identity",
					Value:              f.targetURL,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}

			if len(f.targetStatuses) > 0 {
				for i, s := range f.targetStatuses {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_status:%s:%d", f.targetSubjectRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         f.targetSubjectRef,
						Field:              "http_status",
						Value:              s,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
						ObservedAt:         now,
					})
				}
			} else if f.targetStatus != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_status:%s", f.targetSubjectRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.targetSubjectRef,
					Field:              "http_status",
					Value:              f.targetStatus,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
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

func TestAR_CANON_006_RequiredCases(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	auditRunID := audit.AuditRunID("audit:run:canon006")
	snapID := audit.SnapshotID("snap:canon006")

	// 1. no canonical -> N/A
	t.Run("1. no canonical -> N/A", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "0",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
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

	// 2. multiple distinct targets -> N/A
	t.Run("2. multiple distinct targets -> N/A", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "2",
				completeValue:    "true",
				distinctValue:    "2",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
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

	// 3. unique target + status 200 -> PASS
	t.Run("3. unique target + status 200 -> PASS", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/tgt",
				targetSubjRef:    "url:test:tgt",
				targetSubjectRef: "url:test:tgt",
				targetURL:        "https://example.com/tgt",
				targetStatus:     "200",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		// Notice snapshot has 2 subjects (src and tgt). We check result for src.
		var srcRes *audit.RuleResult
		for i := range results {
			if results[i].SubjectRef == "url:test:src" {
				srcRes = &results[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("missing result for url:test:src")
		}
		if srcRes.Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", srcRes.Status)
		}
	})

	// 4. unique target + 301 -> FAIL
	t.Run("4. unique target + 301 -> FAIL", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/tgt",
				targetSubjRef:    "url:test:tgt",
				targetSubjectRef: "url:test:tgt",
				targetURL:        "https://example.com/tgt",
				targetStatus:     "301",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		var srcRes *audit.RuleResult
		for i := range results {
			if results[i].SubjectRef == "url:test:src" {
				srcRes = &results[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("missing result for url:test:src")
		}
		if srcRes.Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", srcRes.Status)
		}
	})

	// 5. unique target + 404 -> FAIL
	t.Run("5. unique target + 404 -> FAIL", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/tgt",
				targetSubjRef:    "url:test:tgt",
				targetSubjectRef: "url:test:tgt",
				targetURL:        "https://example.com/tgt",
				targetStatus:     "404",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		var srcRes *audit.RuleResult
		for i := range results {
			if results[i].SubjectRef == "url:test:src" {
				srcRes = &results[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("missing result for url:test:src")
		}
		if srcRes.Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", srcRes.Status)
		}
	})

	// 6. unique target + 500 -> FAIL
	t.Run("6. unique target + 500 -> FAIL", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/tgt",
				targetSubjRef:    "url:test:tgt",
				targetSubjectRef: "url:test:tgt",
				targetURL:        "https://example.com/tgt",
				targetStatus:     "500",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		var srcRes *audit.RuleResult
		for i := range results {
			if results[i].SubjectRef == "url:test:src" {
				srcRes = &results[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("missing result for url:test:src")
		}
		if srcRes.Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", srcRes.Status)
		}
	})

	// 7. target ref absent -> UNKNOWN
	t.Run("7. target ref absent -> UNKNOWN", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/external",
				targetSubjRef:    "", // absent
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
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

	// 8. target status missing -> UNKNOWN
	t.Run("8. target status missing -> UNKNOWN", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/tgt",
				targetSubjRef:    "url:test:tgt",
				targetSubjectRef: "url:test:tgt",
				targetURL:        "https://example.com/tgt",
				targetStatus:     "", // missing status
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		var srcRes *audit.RuleResult
		for i := range results {
			if results[i].SubjectRef == "url:test:src" {
				srcRes = &results[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("missing result for url:test:src")
		}
		if srcRes.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", srcRes.Status)
		}
	})

	// 9. target status malformed -> UNKNOWN
	t.Run("9. target status malformed -> UNKNOWN", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/tgt",
				targetSubjRef:    "url:test:tgt",
				targetSubjectRef: "url:test:tgt",
				targetURL:        "https://example.com/tgt",
				targetStatus:     "not_a_status",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		var srcRes *audit.RuleResult
		for i := range results {
			if results[i].SubjectRef == "url:test:src" {
				srcRes = &results[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("missing result for url:test:src")
		}
		if srcRes.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", srcRes.Status)
		}
	})

	// 10. target status conflicting -> UNKNOWN
	t.Run("10. target status conflicting -> UNKNOWN", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/tgt",
				targetSubjRef:    "url:test:tgt",
				targetSubjectRef: "url:test:tgt",
				targetURL:        "https://example.com/tgt",
				targetStatuses:   []string{"200", "404"},
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		var srcRes *audit.RuleResult
		for i := range results {
			if results[i].SubjectRef == "url:test:src" {
				srcRes = &results[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("missing result for url:test:src")
		}
		if srcRes.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", srcRes.Status)
		}
	})

	// 11. canonical uniqueness unavailable -> UNKNOWN
	t.Run("11. canonical uniqueness unavailable -> UNKNOWN", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "2",
				completeValue:    "false", // incomplete normalization
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
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

	// 12. exact source -> target evidence refs
	t.Run("12. exact source -> target evidence refs", func(t *testing.T) {
		snap := newCANON006Snapshot([]canon006TestFixture{
			{
				sourceSubjectRef: "url:test:src",
				sourceURL:        "https://example.com/src",
				countValue:       "1",
				completeValue:    "true",
				distinctValue:    "1",
				normTargetValue:  "https://example.com/tgt",
				targetSubjRef:    "url:test:tgt",
				targetSubjectRef: "url:test:tgt",
				targetURL:        "https://example.com/tgt",
				targetStatus:     "200",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		var srcRes *audit.RuleResult
		for i := range results {
			if results[i].SubjectRef == "url:test:src" {
				srcRes = &results[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("missing result for url:test:src")
		}

		// Verify roles and fields:
		// target http_status is PRIMARY
		// source url, count, complete, distinct, target_subj_ref, target_url are CONTEXT
		foundPrimaryStatus := false
		foundTargetURL := false
		foundSourceURL := false
		for _, ref := range srcRes.EvidenceRefs {
			if ref.Field == "http_status" && ref.Role == audit.EvidenceRolePrimary && ref.ObservedValue == "200" {
				foundPrimaryStatus = true
			}
			if ref.Field == "target_url" && ref.Role == audit.EvidenceRoleContext && ref.ObservedValue == "https://example.com/tgt" {
				foundTargetURL = true
			}
			if ref.Field == "url" && ref.Role == audit.EvidenceRoleContext && ref.ObservedValue == "https://example.com/src" {
				foundSourceURL = true
			}
		}

		if !foundPrimaryStatus {
			t.Errorf("expected target http_status to be PRIMARY evidence ref")
		}
		if !foundTargetURL {
			t.Errorf("expected target_url to be CONTEXT evidence ref")
		}
		if !foundSourceURL {
			t.Errorf("expected source url to be CONTEXT evidence ref")
		}
	})

	// Reversed order determinism check
	t.Run("reversed order determinism", func(t *testing.T) {
		f := canon006TestFixture{
			sourceSubjectRef: "url:test:src",
			sourceURL:        "https://example.com/src",
			countValue:       "1",
			completeValue:    "true",
			distinctValue:    "1",
			normTargetValue:  "https://example.com/tgt",
			targetSubjRef:    "url:test:tgt",
			targetSubjectRef: "url:test:tgt",
			targetURL:        "https://example.com/tgt",
			targetStatus:     "200",
		}
		snapFwd := newCANON006Snapshot([]canon006TestFixture{f}, auditRunID, snapID)
		snapRev := newCANON006Snapshot([]canon006TestFixture{f}, auditRunID, snapID)
		n := len(snapRev.NormalizedObservations)
		for i := 0; i < n/2; i++ {
			snapRev.NormalizedObservations[i], snapRev.NormalizedObservations[n-1-i] = snapRev.NormalizedObservations[n-1-i], snapRev.NormalizedObservations[i]
		}

		resFwd, err := eng.EvaluateRule(ctx, snapFwd, "AR-CANON-006")
		if err != nil {
			t.Fatalf("fwd failed: %v", err)
		}
		resRev, err := eng.EvaluateRule(ctx, snapRev, "AR-CANON-006")
		if err != nil {
			t.Fatalf("rev failed: %v", err)
		}
		if !reflect.DeepEqual(resFwd, resRev) {
			t.Errorf("expected deterministic results regardless of observation order")
		}
	})
}
