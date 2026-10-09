package engine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

type redirectTestFixture struct {
	subjectRef     string
	urlValue       string
	urlValues      []string
	initialValue   string
	initialValues  []string
	countValue     string
	countValues    []string
	completeValue  string
	completeValues []string
	loopValue      string
	loopValues     []string
	hopValues      []string
	finalURL       string
	finalURLs      []string
}

func hopJSON(index int, src string, status int, target string) string {
	b, _ := json.Marshal(map[string]any{
		"hop_index":           index,
		"source_url":          src,
		"status":              status,
		"resolved_target_url": target,
	})
	return string(b)
}

func newRedirectSnapshot(fixtures []redirectTestFixture, auditRunID audit.AuditRunID, snapID audit.SnapshotID) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation
	obsSeq := 0

	nextID := func() audit.ObservationID {
		obsSeq++
		return audit.ObservationID(fmt.Sprintf("obs:%s:%d", snapID, obsSeq))
	}

	for _, f := range fixtures {
		// URL identity
		if len(f.urlValues) > 0 {
			for _, u := range f.urlValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextID(),
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
				ObservationID:      nextID(),
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

		// redirect_initial_observed
		if len(f.initialValues) > 0 {
			for _, v := range f.initialValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextID(),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "redirect_initial_observed",
					Value:              v,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.initialValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextID(),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "redirect_initial_observed",
				Value:              f.initialValue,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// redirect_hop_count
		if len(f.countValues) > 0 {
			for _, v := range f.countValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextID(),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "redirect_hop_count",
					Value:              v,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.countValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextID(),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "redirect_hop_count",
				Value:              f.countValue,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// redirect_traversal_complete
		if len(f.completeValues) > 0 {
			for _, v := range f.completeValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextID(),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "redirect_traversal_complete",
					Value:              v,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.completeValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextID(),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "redirect_traversal_complete",
				Value:              f.completeValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// redirect_loop_detected
		if len(f.loopValues) > 0 {
			for _, v := range f.loopValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextID(),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "redirect_loop_detected",
					Value:              v,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.loopValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextID(),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "redirect_loop_detected",
				Value:              f.loopValue,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// redirect_hop
		for _, h := range f.hopValues {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextID(),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "redirect_hop",
				Value:              h,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// redirect_final_url
		if len(f.finalURLs) > 0 {
			for _, u := range f.finalURLs {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextID(),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "redirect_final_url",
					Value:              u,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.finalURL != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextID(),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "redirect_final_url",
				Value:              f.finalURL,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             snapID,
		AuditRunID:             auditRunID,
		NormalizationVersion:   "v1.6.0",
		SnapshotStatus:         audit.SnapshotFrozen,
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_CANON_008_RequiredCases(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	// 1. No redirect -> NOT_APPLICABLE
	t.Run("1. no redirect -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:   "url:1",
				urlValue:     "https://example.com/200",
				initialValue: "false",
				countValue:   "0",
			},
		}, "audit:1", "snap:1")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusNotApplicable {
			t.Fatalf("expected NOT_APPLICABLE, got %+v", results)
		}
	})

	// 2. Complete one-hop -> PASS
	t.Run("2. complete one-hop -> PASS", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:2", "snap:2")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %+v", results)
		}
	})

	// 3. Complete two-hop -> FAIL
	t.Run("3. complete two-hop -> FAIL", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/h0",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/h0", 301, "https://example.com/h1"),
					hopJSON(1, "https://example.com/h1", 302, "https://example.com/h2"),
				},
				finalURL: "https://example.com/h2",
			},
		}, "audit:3", "snap:3")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %+v", results)
		}
	})

	// 4. Complete three-hop -> FAIL
	t.Run("4. complete three-hop -> FAIL", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/h0",
				initialValue:  "true",
				countValue:    "3",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/h0", 301, "https://example.com/h1"),
					hopJSON(1, "https://example.com/h1", 302, "https://example.com/h2"),
					hopJSON(2, "https://example.com/h2", 307, "https://example.com/h3"),
				},
				finalURL: "https://example.com/h3",
			},
		}, "audit:4", "snap:4")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %+v", results)
		}
	})

	// 5. Incomplete one-hop -> UNKNOWN
	t.Run("5. incomplete one-hop -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:5", "snap:5")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for incomplete one-hop, got %+v", results)
		}
	})

	// 6. Incomplete multiple hops -> UNKNOWN
	t.Run("6. incomplete multiple hops -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/h0",
				initialValue:  "true",
				countValue:    "5",
				completeValue: "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/h0", 301, "https://example.com/h1"),
					hopJSON(1, "https://example.com/h1", 301, "https://example.com/h2"),
					hopJSON(2, "https://example.com/h2", 301, "https://example.com/h3"),
					hopJSON(3, "https://example.com/h3", 301, "https://example.com/h4"),
					hopJSON(4, "https://example.com/h4", 301, "https://example.com/h5"),
				},
			},
		}, "audit:6", "snap:6")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN (must not fail incomplete traversal even with multiple hops), got %+v", results)
		}
	})

	// 7. Missing completeness -> UNKNOWN
	t.Run("7. missing completeness -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:   "url:1",
				urlValue:     "https://example.com/start",
				initialValue: "true",
				countValue:   "1",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:7", "snap:7")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for missing completeness, got %+v", results)
		}
	})

	// 8. Missing/malformed hop count -> UNKNOWN
	t.Run("8. missing or malformed hop count -> UNKNOWN", func(t *testing.T) {
		// 8a: Missing hop count
		snap1 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:8a", "snap:8a")

		res1, err := eng.EvaluateRule(context.Background(), snap1, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res1) != 1 || res1[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for missing hop count, got %+v", res1)
		}

		// 8b: Malformed hop count (non-integer)
		snap2 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "not-a-number",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:8b", "snap:8b")

		res2, err := eng.EvaluateRule(context.Background(), snap2, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res2) != 1 || res2[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for malformed hop count, got %+v", res2)
		}
	})

	// 9. Count/hop observation mismatch -> UNKNOWN
	t.Run("9. count and hop observation mismatch -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2", // claims 2 hops, but only 1 hop observation provided
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:9", "snap:9")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for count/hop mismatch, got %+v", results)
		}
	})

	// 10. Missing/conflicting initial state -> UNKNOWN
	t.Run("10. missing or conflicting initial state -> UNKNOWN", func(t *testing.T) {
		// 10a: Missing initial state
		snap1 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				countValue:    "1",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:10a", "snap:10a")

		res1, err := eng.EvaluateRule(context.Background(), snap1, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res1) != 1 || res1[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for missing initial state, got %+v", res1)
		}

		// 10b: Conflicting initial state (true and false)
		snap2 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValues: []string{"true", "false"},
				countValue:    "1",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:10b", "snap:10b")

		res2, err := eng.EvaluateRule(context.Background(), snap2, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res2) != 1 || res2[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for conflicting initial state, got %+v", res2)
		}
	})

	// 11. Malformed/duplicate hop index -> UNKNOWN
	t.Run("11. malformed or duplicate hop index -> UNKNOWN", func(t *testing.T) {
		// 11a: Malformed JSON
		snap1 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				hopValues: []string{
					"invalid-json",
				},
			},
		}, "audit:11a", "snap:11a")

		res1, err := eng.EvaluateRule(context.Background(), snap1, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res1) != 1 || res1[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for malformed hop JSON, got %+v", res1)
		}

		// 11b: Duplicate conflicting hop index 0
		snap2 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest1"),
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest2"),
				},
			},
		}, "audit:11b", "snap:11b")

		res2, err := eng.EvaluateRule(context.Background(), snap2, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res2) != 1 || res2[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for duplicate conflicting hop index, got %+v", res2)
		}

		// 11c: Noncontiguous hop indices (0 and 2 with count=2)
		snap3 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/h1"),
					hopJSON(2, "https://example.com/h1", 301, "https://example.com/h2"),
				},
			},
		}, "audit:11c", "snap:11c")

		res3, err := eng.EvaluateRule(context.Background(), snap3, "AR-CANON-008")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res3) != 1 || res3[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for noncontiguous hop indices, got %+v", res3)
		}
	})

	// 12. Reversed observation ordering deterministic
	t.Run("12. reversed observation ordering deterministic", func(t *testing.T) {
		snapForward := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/h1"),
					hopJSON(1, "https://example.com/h1", 302, "https://example.com/h2"),
				},
				finalURL: "https://example.com/h2",
			},
		}, "audit:12", "snap:12f")

		// Create reversed observations
		revObs := make([]audit.NormalizedObservation, len(snapForward.NormalizedObservations))
		for i, o := range snapForward.NormalizedObservations {
			revObs[len(snapForward.NormalizedObservations)-1-i] = o
		}
		now := time.Now().UTC()
		snapReversed := &audit.EvidenceSnapshot{
			SnapshotID:             "snap:12f",
			AuditRunID:             "audit:12",
			NormalizationVersion:   "v1.6.0",
			SnapshotStatus:         audit.SnapshotFrozen,
			FrozenAt:               &now,
			NormalizedObservations: revObs,
		}

		resForward, err := eng.EvaluateRule(context.Background(), snapForward, "AR-CANON-008")
		if err != nil {
			t.Fatalf("forward error: %v", err)
		}
		resReversed, err := eng.EvaluateRule(context.Background(), snapReversed, "AR-CANON-008")
		if err != nil {
			t.Fatalf("reversed error: %v", err)
		}

		if len(resForward) != 1 || len(resReversed) != 1 {
			t.Fatalf("expected 1 result each, got %d and %d", len(resForward), len(resReversed))
		}
		if resForward[0].Status != resReversed[0].Status {
			t.Errorf("status mismatch: %s vs %s", resForward[0].Status, resReversed[0].Status)
		}
		if resForward[0].ObservedSummary != resReversed[0].ObservedSummary {
			t.Errorf("summary mismatch: %q vs %q", resForward[0].ObservedSummary, resReversed[0].ObservedSummary)
		}
	})
}
