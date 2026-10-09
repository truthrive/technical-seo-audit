package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func TestAR_CANON_009_RequiredCases(t *testing.T) {
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

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusNotApplicable {
			t.Fatalf("expected NOT_APPLICABLE, got %+v", results)
		}
	})

	// 2. Complete redirect + loop=false -> PASS
	t.Run("2. complete redirect + loop=false -> PASS", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:2", "snap:2")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %+v", results)
		}
	})

	// 3. Explicit loop=true -> FAIL
	t.Run("3. explicit loop=true -> FAIL", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/loop1",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "false",
				loopValue:     "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/loop1", 301, "https://example.com/loop2"),
					hopJSON(1, "https://example.com/loop2", 301, "https://example.com/loop1"),
				},
			},
		}, "audit:3", "snap:3")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL for explicit loop=true, got %+v", results)
		}
	})

	// 4. Loop=true + traversal incomplete -> FAIL
	t.Run("4. loop=true + traversal incomplete -> FAIL", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/loop1",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "false", // traversal incomplete
				loopValue:     "true",  // but explicit positive loop evidence exists
				hopValues: []string{
					hopJSON(0, "https://example.com/loop1", 301, "https://example.com/loop2"),
					hopJSON(1, "https://example.com/loop2", 301, "https://example.com/loop1"),
				},
			},
		}, "audit:4", "snap:4")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL (loop is positive evidence even when traversal incomplete), got %+v", results)
		}
	})

	// 5. Incomplete chain + loop missing -> UNKNOWN
	t.Run("5. incomplete chain + loop missing -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/h0",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "false",
				// loopValue intentionally missing
				hopValues: []string{
					hopJSON(0, "https://example.com/h0", 301, "https://example.com/h1"),
				},
			},
		}, "audit:5", "snap:5")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for incomplete chain with missing loop state, got %+v", results)
		}
	})

	// 6. FollowRedirects=false evidence -> UNKNOWN
	t.Run("6. FollowRedirects=false evidence -> UNKNOWN", func(t *testing.T) {
		// When FollowRedirects=false: initial redirect=true, complete=false, loop state unavailable
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/nofollow",
				initialValue:  "true",
				countValue:    "0",
				completeValue: "false",
				// loop state withheld
			},
		}, "audit:6", "snap:6")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN when FollowRedirects=false, got %+v", results)
		}
	})

	// 7. Redirect limit without loop proof -> UNKNOWN
	t.Run("7. redirect limit without loop proof -> UNKNOWN", func(t *testing.T) {
		// Chain too long error: complete=false, loop state withheld (not an explicit loop)
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/h0",
				initialValue:  "true",
				countValue:    "5",
				completeValue: "false",
				// loop state withheld
				hopValues: []string{
					hopJSON(0, "https://example.com/h0", 301, "https://example.com/h1"),
					hopJSON(1, "https://example.com/h1", 301, "https://example.com/h2"),
					hopJSON(2, "https://example.com/h2", 301, "https://example.com/h3"),
					hopJSON(3, "https://example.com/h3", 301, "https://example.com/h4"),
					hopJSON(4, "https://example.com/h4", 301, "https://example.com/h5"),
				},
			},
		}, "audit:7", "snap:7")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for chain limit without loop proof, got %+v", results)
		}
	})

	// 8. Missing loop state -> UNKNOWN
	t.Run("8. missing loop state on complete traversal -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				// loopValue intentionally missing
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:8", "snap:8")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN (never infer loop=false when evidence missing), got %+v", results)
		}
	})

	// 9. Conflicting loop states -> UNKNOWN
	t.Run("9. conflicting loop states -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				loopValues:    []string{"true", "false"},
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:9", "snap:9")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for conflicting loop states, got %+v", results)
		}
	})

	// 10. Malformed loop state -> UNKNOWN
	t.Run("10. malformed loop state -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				loopValue:     "not-a-boolean",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
			},
		}, "audit:10", "snap:10")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for malformed loop state, got %+v", results)
		}
	})

	// 11. Contradictory initial redirect -> UNKNOWN
	t.Run("11. contradictory initial redirect -> UNKNOWN", func(t *testing.T) {
		// 11a: Initial says false, but loop=true is present
		snap1 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:   "url:1",
				urlValue:     "https://example.com/start",
				initialValue: "false",
				loopValue:    "true",
			},
		}, "audit:11a", "snap:11a")

		res1, err := eng.EvaluateRule(context.Background(), snap1, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res1) != 1 || res1[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for contradictory initial redirect (false + loop=true), got %+v", res1)
		}

		// 11b: Initial observed has conflicting values [true, false]
		snap2 := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValues: []string{"true", "false"},
				loopValue:     "false",
				completeValue: "true",
			},
		}, "audit:11b", "snap:11b")

		res2, err := eng.EvaluateRule(context.Background(), snap2, "AR-CANON-009")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(res2) != 1 || res2[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for conflicting initial redirect, got %+v", res2)
		}
	})

	// 12. Reversed observation ordering deterministic
	t.Run("12. reversed observation ordering deterministic", func(t *testing.T) {
		snapForward := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:12", "snap:12f")

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

		resForward, err := eng.EvaluateRule(context.Background(), snapForward, "AR-CANON-009")
		if err != nil {
			t.Fatalf("forward error: %v", err)
		}
		resReversed, err := eng.EvaluateRule(context.Background(), snapReversed, "AR-CANON-009")
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
