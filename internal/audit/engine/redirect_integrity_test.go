package engine_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// TestRedirectIntegrity_RequiredRegressionCases tests all 18 required regression cases
// for V1.5b Redirect Evaluator Integrity Correction.
func TestRedirectIntegrity_RequiredRegressionCases(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	// 1. Missing hop_index -> UNKNOWN
	t.Run("1. missing hop_index -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					`{"source_url":"https://example.com/start","status":301,"resolved_target_url":"https://example.com/dest"}`,
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:1", "snap:1")

		res008, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval 008 failed: %v", err)
		}
		if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-008 expected UNKNOWN for missing hop_index, got %+v", res008)
		}

		res009, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("eval 009 failed: %v", err)
		}
		if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-009 expected UNKNOWN for missing hop_index, got %+v", res009)
		}
	})

	// 2. hop_index wrong type -> UNKNOWN
	t.Run("2. hop_index wrong type -> UNKNOWN", func(t *testing.T) {
		testBadIndices := []string{
			`{"hop_index":"0","source_url":"https://example.com/start","status":301,"resolved_target_url":"https://example.com/dest"}`,
			`{"hop_index":1.5,"source_url":"https://example.com/start","status":301,"resolved_target_url":"https://example.com/dest"}`,
			`{"hop_index":true,"source_url":"https://example.com/start","status":301,"resolved_target_url":"https://example.com/dest"}`,
			`{"hop_index":-1,"source_url":"https://example.com/start","status":301,"resolved_target_url":"https://example.com/dest"}`,
		}

		for i, badHop := range testBadIndices {
			snap := newRedirectSnapshot([]redirectTestFixture{
				{
					subjectRef:    "url:1",
					urlValue:      "https://example.com/start",
					initialValue:  "true",
					countValue:    "1",
					completeValue: "true",
					loopValue:     "false",
					hopValues:     []string{badHop},
					finalURL:      "https://example.com/dest",
				},
			}, audit.AuditRunID(fmt.Sprintf("audit:2:%d", i)), audit.SnapshotID(fmt.Sprintf("snap:2:%d", i)))

			res008, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
			if err != nil {
				t.Fatalf("eval 008 failed: %v", err)
			}
			if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
				t.Errorf("[%d] CANON-008 expected UNKNOWN, got %+v", i, res008)
			}

			res009, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
			if err != nil {
				t.Fatalf("eval 009 failed: %v", err)
			}
			if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
				t.Errorf("[%d] CANON-009 expected UNKNOWN, got %+v", i, res009)
			}
		}
	})

	// 3. Missing required hop property -> UNKNOWN
	t.Run("3. missing required hop property -> UNKNOWN", func(t *testing.T) {
		missingProps := []string{
			`{"hop_index":0,"source_url":"https://example.com/start","resolved_target_url":"https://example.com/dest"}`, // missing status
			`{"hop_index":0,"status":301,"resolved_target_url":"https://example.com/dest"}`,                             // missing source_url
			`{"hop_index":0,"source_url":"https://example.com/start","status":301}`,                                     // missing resolved_target_url
		}

		for i, badHop := range missingProps {
			snap := newRedirectSnapshot([]redirectTestFixture{
				{
					subjectRef:    "url:1",
					urlValue:      "https://example.com/start",
					initialValue:  "true",
					countValue:    "1",
					completeValue: "true",
					loopValue:     "false",
					hopValues:     []string{badHop},
					finalURL:      "https://example.com/dest",
				},
			}, audit.AuditRunID(fmt.Sprintf("audit:3:%d", i)), audit.SnapshotID(fmt.Sprintf("snap:3:%d", i)))

			res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
			if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
				t.Errorf("[%d] CANON-008 expected UNKNOWN, got %+v", i, res008)
			}

			res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
			if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
				t.Errorf("[%d] CANON-009 expected UNKNOWN, got %+v", i, res009)
			}
		}
	})

	// 4. Non-redirect hop status -> UNKNOWN
	t.Run("4. non-redirect hop status -> UNKNOWN", func(t *testing.T) {
		nonRedirectStatuses := []int{200, 204, 404, 500, 299, 400}
		for _, st := range nonRedirectStatuses {
			snap := newRedirectSnapshot([]redirectTestFixture{
				{
					subjectRef:    "url:1",
					urlValue:      "https://example.com/start",
					initialValue:  "true",
					countValue:    "1",
					completeValue: "true",
					loopValue:     "false",
					hopValues: []string{
						hopJSON(0, "https://example.com/start", st, "https://example.com/dest"),
					},
					finalURL: "https://example.com/dest",
				},
			}, audit.AuditRunID(fmt.Sprintf("audit:4:%d", st)), audit.SnapshotID(fmt.Sprintf("snap:4:%d", st)))

			res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
			if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
				t.Errorf("status %d CANON-008 expected UNKNOWN, got %+v", st, res008)
			}

			res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
			if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
				t.Errorf("status %d CANON-009 expected UNKNOWN, got %+v", st, res009)
			}
		}
	})

	// 5. Invalid hop URL -> UNKNOWN
	t.Run("5. invalid hop URL -> UNKNOWN", func(t *testing.T) {
		invalidURLs := []struct {
			src string
			dst string
		}{
			{"ftp://example.com/start", "https://example.com/dest"},
			{"https://example.com/start", "ftp://example.com/dest"},
			{"/relative/start", "https://example.com/dest"},
			{"https://example.com/start", "/relative/dest"},
			{"not a url", "https://example.com/dest"},
			{"https://example.com/start", "not a url"},
		}

		for i, u := range invalidURLs {
			snap := newRedirectSnapshot([]redirectTestFixture{
				{
					subjectRef:    "url:1",
					urlValue:      "https://example.com/start",
					initialValue:  "true",
					countValue:    "1",
					completeValue: "true",
					loopValue:     "false",
					hopValues: []string{
						hopJSON(0, u.src, 301, u.dst),
					},
					finalURL: u.dst,
				},
			}, audit.AuditRunID(fmt.Sprintf("audit:5:%d", i)), audit.SnapshotID(fmt.Sprintf("snap:5:%d", i)))

			res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
			if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
				t.Errorf("[%d] CANON-008 expected UNKNOWN for invalid URL, got %+v", i, res008)
			}

			res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
			if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
				t.Errorf("[%d] CANON-009 expected UNKNOWN for invalid URL, got %+v", i, res009)
			}
		}
	})

	// 6. hop[0] source != URL identity -> UNKNOWN
	t.Run("6. hop[0] source != URL identity -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/mismatched-source", 301, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:6", "snap:6")

		res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-008 expected UNKNOWN when hop[0].src != url, got %+v", res008)
		}

		res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-009 expected UNKNOWN when hop[0].src != url, got %+v", res009)
		}
	})

	// 7. Discontinuous redirect chain -> UNKNOWN
	t.Run("7. discontinuous redirect chain -> UNKNOWN", func(t *testing.T) {
		// Hop 0 target is /mid-A, but Hop 1 source is /mid-B
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/mid-A"),
					hopJSON(1, "https://example.com/mid-B", 302, "https://example.com/final"),
				},
				finalURL: "https://example.com/final",
			},
		}, "audit:7", "snap:7")

		res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-008 expected UNKNOWN for discontinuous chain, got %+v", res008)
		}

		res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-009 expected UNKNOWN for discontinuous chain, got %+v", res009)
		}
	})

	// 8. Complete=true + loop=true -> UNKNOWN
	t.Run("8. complete=true + loop=true -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				loopValue:     "true", // Contradiction: traversal completed AND loop detected
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:8", "snap:8")

		res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-008 expected UNKNOWN for complete=true + loop=true, got %+v", res008)
		}

		res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-009 expected UNKNOWN for complete=true + loop=true, got %+v", res009)
		}
	})

	// 9. CANON-009 malformed hop + loop=false + complete=true -> UNKNOWN
	t.Run("9. CANON-009 malformed hop + loop=false + complete=true -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					`{not-valid-json}`,
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:9", "snap:9")

		res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-009 expected UNKNOWN when hop is malformed, got %+v", res009)
		}
	})

	// 10. CANON-009 count/hop mismatch -> UNKNOWN
	t.Run("10. CANON-009 count/hop mismatch -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2", // claims 2 hops, but only 1 hop emitted
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:10", "snap:10")

		res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if len(res009) != 1 || res009[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-009 expected UNKNOWN when count/hop mismatch, got %+v", res009)
		}
	})

	// 11. CANON-009 valid complete chain -> PASS
	t.Run("11. CANON-009 valid complete chain -> PASS", func(t *testing.T) {
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
		}, "audit:11", "snap:11")

		res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if len(res009) != 1 || res009[0].Status != audit.StatusPass {
			t.Errorf("CANON-009 expected PASS for valid complete chain, got %+v", res009)
		}
	})

	// 12. Confirmed loop + incomplete -> FAIL
	t.Run("12. confirmed loop + incomplete -> FAIL", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "false", // incomplete traversal
				loopValue:     "true",  // confirmed loop
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/loop"),
				},
			},
		}, "audit:12", "snap:12")

		res009, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if len(res009) != 1 || res009[0].Status != audit.StatusFail {
			t.Errorf("CANON-009 expected FAIL for confirmed loop + incomplete, got %+v", res009)
		}
	})

	// 13. CANON-008 one-hop -> PASS
	t.Run("13. CANON-008 one-hop -> PASS", func(t *testing.T) {
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
		}, "audit:13", "snap:13")

		res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if len(res008) != 1 || res008[0].Status != audit.StatusPass {
			t.Errorf("CANON-008 expected PASS for one-hop, got %+v", res008)
		}
	})

	// 14. CANON-008 multi-hop -> FAIL
	t.Run("14. CANON-008 multi-hop -> FAIL", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/h1"),
					hopJSON(1, "https://example.com/h1", 302, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:14", "snap:14")

		res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if len(res008) != 1 || res008[0].Status != audit.StatusFail {
			t.Errorf("CANON-008 expected FAIL for multi-hop, got %+v", res008)
		}
	})

	// 15. Incomplete chain -> UNKNOWN
	t.Run("15. incomplete chain -> UNKNOWN", func(t *testing.T) {
		// Even with multiple hops captured, incomplete traversal is NEVER failed
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "false", // incomplete
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/h1"),
					hopJSON(1, "https://example.com/h1", 302, "https://example.com/h2"),
				},
			},
		}, "audit:15", "snap:15")

		res008, _ := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if len(res008) != 1 || res008[0].Status != audit.StatusUnknown {
			t.Errorf("CANON-008 expected UNKNOWN for incomplete chain, got %+v", res008)
		}
	})

	// 16. Reversed observation order deterministic
	t.Run("16. reversed observation order deterministic", func(t *testing.T) {
		snapFwd := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/h1"),
					hopJSON(1, "https://example.com/h1", 302, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:16", "snap:16f")

		revObs := make([]audit.NormalizedObservation, len(snapFwd.NormalizedObservations))
		for i, o := range snapFwd.NormalizedObservations {
			revObs[len(snapFwd.NormalizedObservations)-1-i] = o
		}
		now := time.Now().UTC()
		snapRev := &audit.EvidenceSnapshot{
			SnapshotID:             "snap:16f",
			AuditRunID:             "audit:16",
			NormalizationVersion:   "v1.6.0",
			SnapshotStatus:         audit.SnapshotFrozen,
			FrozenAt:               &now,
			NormalizedObservations: revObs,
		}

		fixedTime := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
		eng.SetClock(func() time.Time { return fixedTime })

		for _, ruleID := range []string{"AR-CANON-008", "AR-CANON-009"} {
			resFwd, err := eng.EvaluateRule(ctx, snapFwd, ruleID)
			if err != nil {
				t.Fatalf("eval fwd %s failed: %v", ruleID, err)
			}
			resRev, err := eng.EvaluateRule(ctx, snapRev, ruleID)
			if err != nil {
				t.Fatalf("eval rev %s failed: %v", ruleID, err)
			}

			if len(resFwd) != len(resRev) {
				t.Fatalf("%s result len mismatch: %d vs %d", ruleID, len(resFwd), len(resRev))
			}
			if resFwd[0].Status != resRev[0].Status {
				t.Errorf("%s status mismatch: %s vs %s", ruleID, resFwd[0].Status, resRev[0].Status)
			}
			if resFwd[0].ObservedSummary != resRev[0].ObservedSummary {
				t.Errorf("%s summary mismatch: %q vs %q", ruleID, resFwd[0].ObservedSummary, resRev[0].ObservedSummary)
			}
		}
	})

	// 17. Existing seven evaluators remain green
	t.Run("17. existing seven evaluators remain green", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:   "url:1",
				urlValue:     "https://example.com/page",
				initialValue: "false",
			},
		}, "audit:17", "snap:17")

		existingSeven := []string{
			"AR-ACC-004",
			"AR-CANON-003",
			"AR-CANON-004",
			"AR-CANON-006",
			"AR-CANON-007",
			"AR-INDEX-001",
			"AR-INDEX-002",
		}

		for _, ruleID := range existingSeven {
			results, err := eng.EvaluateRule(ctx, snap, ruleID)
			if err != nil {
				t.Errorf("eval %s failed: %v", ruleID, err)
			}
			for _, r := range results {
				if r.Status == audit.StatusFail {
					t.Errorf("rule %s failed unexpectedly on clean baseline: %+v", ruleID, r)
				}
			}
		}
	})

	// 18. Hermetic redirect integration remains green
	t.Run("18. hermetic redirect integration remains green", func(t *testing.T) {
		mux := http.NewServeMux()
		var srvURL string
		mux.HandleFunc("/entry", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srvURL+"/dest", http.StatusMovedPermanently)
		})
		mux.HandleFunc("/dest", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head><title>Dest</title><link rel="canonical" href="%s/dest"></head><body><h1>Dest</h1></body></html>`, srvURL)
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()
		srvURL = srv.URL

		db, err := standalone.OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open db failed: %v", err)
		}
		defer db.Close()

		runner := sitecrawl.NewRunner(db)
		if err := runner.EnsureSchema(); err != nil {
			t.Fatalf("schema failed: %v", err)
		}

		started, err := runner.Crawl(ctx, []string{srv.URL + "/entry"}, sitecrawl.Options{
			Mode:            sitecrawl.ModeSpider,
			MaxDepth:        2,
			MaxURLs:         5,
			Concurrency:     1,
			FollowRedirects: true,
		})
		if err != nil {
			t.Fatalf("crawl failed: %v", err)
		}

		buildRes, err := adapter.Build(ctx, db, adapter.BuildRequest{
			CrawlRunID: started.ID,
			AuditRunID: "audit:run:hermetic18",
			SnapshotID: "snap:run:hermetic18",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		snap := buildRes.EvidenceSnapshot
		res008, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("008 failed: %v", err)
		}
		res009, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("009 failed: %v", err)
		}

		entrySubj := findSubjectRefByObservedURL(snap, srv.URL+"/entry")
		if entrySubj == "" {
			t.Fatalf("entry subject not found")
		}

		var found008, found009 bool
		for _, r := range res008 {
			if r.SubjectRef == entrySubj {
				found008 = true
				if r.Status != audit.StatusPass {
					t.Errorf("expected 008 PASS, got %s", r.Status)
				}
			}
		}
		for _, r := range res009 {
			if r.SubjectRef == entrySubj {
				found009 = true
				if r.Status != audit.StatusPass {
					t.Errorf("expected 009 PASS, got %s", r.Status)
				}
			}
		}

		if !found008 || !found009 {
			t.Fatalf("missing results for entry: found008=%v, found009=%v", found008, found009)
		}
	})
}

// TestFinalScalarIntegrity_RequiredCases tests all 15 required focused cases
// for V1.5b Final Scalar Integrity Correction.
func TestFinalScalarIntegrity_RequiredCases(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	// 1. CANON-008 conflicting loop=true/false + otherwise complete one-hop -> UNKNOWN
	t.Run("1. CANON-008 conflicting loop=true/false + complete one-hop -> UNKNOWN", func(t *testing.T) {
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
				finalURL: "https://example.com/dest",
			},
		}, "audit:scalar:1", "snap:scalar:1")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval CANON-008 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for conflicting loop evidence, got %+v", results)
		}

		var hasLoopRef bool
		for _, ref := range results[0].EvidenceRefs {
			for _, obs := range snap.NormalizedObservations {
				if string(obs.ObservationID) == ref.EvidenceRef && obs.Field == "redirect_loop_detected" {
					hasLoopRef = true
				}
			}
		}
		if !hasLoopRef {
			t.Errorf("expected evidence ref pointing to redirect_loop_detected observation")
		}
	})

	// 2. CANON-008 conflicting loop=true/false + otherwise complete multi-hop -> UNKNOWN
	t.Run("2. CANON-008 conflicting loop=true/false + complete multi-hop -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				loopValues:    []string{"true", "false"},
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/mid"),
					hopJSON(1, "https://example.com/mid", 302, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:scalar:2", "snap:scalar:2")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval CANON-008 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for conflicting loop evidence on multi-hop, got %+v", results)
		}
	})

	// 3. CANON-009 loop=true + conflicting completeness -> UNKNOWN
	t.Run("3. CANON-009 loop=true + conflicting completeness -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:     "url:1",
				urlValue:       "https://example.com/start",
				initialValue:   "true",
				countValue:     "1",
				completeValues: []string{"true", "false"},
				loopValue:      "true",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/start"),
				},
			},
		}, "audit:scalar:3", "snap:scalar:3")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("eval CANON-009 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for loop=true + conflicting completeness, got %+v", results)
		}
	})

	// 4. CANON-009 loop=false + conflicting completeness -> UNKNOWN
	t.Run("4. CANON-009 loop=false + conflicting completeness -> UNKNOWN", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:     "url:1",
				urlValue:       "https://example.com/start",
				initialValue:   "true",
				countValue:     "1",
				completeValues: []string{"true", "false"},
				loopValue:      "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:scalar:4", "snap:scalar:4")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("eval CANON-009 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for loop=false + conflicting completeness, got %+v", results)
		}
	})

	// 5. CANON-008 conflicting final URLs + complete chain -> UNKNOWN
	t.Run("5. CANON-008 conflicting final URLs + complete chain -> UNKNOWN", func(t *testing.T) {
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
				finalURLs: []string{"https://example.com/dest", "https://example.com/conflicting"},
			},
		}, "audit:scalar:5", "snap:scalar:5")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval CANON-008 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for conflicting final URLs, got %+v", results)
		}
	})

	// 6. CANON-009 conflicting final URLs + complete chain -> UNKNOWN
	t.Run("6. CANON-009 conflicting final URLs + complete chain -> UNKNOWN", func(t *testing.T) {
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
				finalURLs: []string{"https://example.com/dest", "https://example.com/conflicting"},
			},
		}, "audit:scalar:6", "snap:scalar:6")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("eval CANON-009 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for conflicting final URLs, got %+v", results)
		}
	})

	// 7. Identical duplicate final URLs preserve original result
	t.Run("7. identical duplicate final URLs preserve original result", func(t *testing.T) {
		// CANON-008 one-hop should PASS
		snap008 := newRedirectSnapshot([]redirectTestFixture{
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
				finalURLs: []string{"https://example.com/dest", "https://example.com/dest"},
			},
		}, "audit:scalar:7a", "snap:scalar:7a")

		res008, err := eng.EvaluateRule(ctx, snap008, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval CANON-008 failed: %v", err)
		}
		if len(res008) != 1 || res008[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS for identical duplicate final URLs, got %+v", res008)
		}

		// CANON-009 complete no loop should PASS
		res009, err := eng.EvaluateRule(ctx, snap008, "AR-CANON-009")
		if err != nil {
			t.Fatalf("eval CANON-009 failed: %v", err)
		}
		if len(res009) != 1 || res009[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS for identical duplicate final URLs, got %+v", res009)
		}
	})

	// 8. Missing optional final URL preserves original result
	t.Run("8. missing optional final URL preserves original result", func(t *testing.T) {
		// CANON-008 one-hop without finalURL should PASS
		snap008One := newRedirectSnapshot([]redirectTestFixture{
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
				finalURL: "", // missing optional final URL
			},
		}, "audit:scalar:8a", "snap:scalar:8a")

		res008One, err := eng.EvaluateRule(ctx, snap008One, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval CANON-008 failed: %v", err)
		}
		if len(res008One) != 1 || res008One[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS for one-hop with missing final URL, got %+v", res008One)
		}

		// CANON-009 complete no loop without finalURL should PASS
		res009One, err := eng.EvaluateRule(ctx, snap008One, "AR-CANON-009")
		if err != nil {
			t.Fatalf("eval CANON-009 failed: %v", err)
		}
		if len(res009One) != 1 || res009One[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS for CANON-009 with missing final URL, got %+v", res009One)
		}

		// CANON-008 multi-hop without finalURL should FAIL
		snap008Multi := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/mid"),
					hopJSON(1, "https://example.com/mid", 302, "https://example.com/dest"),
				},
				finalURL: "",
			},
		}, "audit:scalar:8b", "snap:scalar:8b")

		res008Multi, err := eng.EvaluateRule(ctx, snap008Multi, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval CANON-008 failed: %v", err)
		}
		if len(res008Multi) != 1 || res008Multi[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL for multi-hop with missing final URL, got %+v", res008Multi)
		}
	})

	// 9. Explicit loop + incomplete -> FAIL
	t.Run("9. explicit loop + incomplete -> FAIL", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "1",
				completeValue: "false", // incomplete
				loopValue:     "true",  // explicit loop
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/start"),
				},
			},
		}, "audit:scalar:9", "snap:scalar:9")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("eval CANON-009 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL for explicit loop + incomplete, got %+v", results)
		}
	})

	// 10. Valid one-hop -> PASS CANON-008
	t.Run("10. valid one-hop -> PASS CANON-008", func(t *testing.T) {
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
		}, "audit:scalar:10", "snap:scalar:10")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval CANON-008 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS for valid one-hop, got %+v", results)
		}
	})

	// 11. Valid multi-hop -> FAIL CANON-008
	t.Run("11. valid multi-hop -> FAIL CANON-008", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/mid"),
					hopJSON(1, "https://example.com/mid", 302, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:scalar:11", "snap:scalar:11")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("eval CANON-008 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL for valid multi-hop, got %+v", results)
		}
	})

	// 12. Valid complete no-loop -> PASS CANON-009
	t.Run("12. valid complete no-loop -> PASS CANON-009", func(t *testing.T) {
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
		}, "audit:scalar:12", "snap:scalar:12")

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("eval CANON-009 failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS for valid complete no-loop, got %+v", results)
		}
	})

	// 13. Reversed observation order deterministic
	t.Run("13. reversed observation order deterministic", func(t *testing.T) {
		snapFwd := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:    "url:1",
				urlValue:      "https://example.com/start",
				initialValue:  "true",
				countValue:    "2",
				completeValue: "true",
				loopValue:     "false",
				hopValues: []string{
					hopJSON(0, "https://example.com/start", 301, "https://example.com/mid"),
					hopJSON(1, "https://example.com/mid", 302, "https://example.com/dest"),
				},
				finalURL: "https://example.com/dest",
			},
		}, "audit:scalar:13", "snap:scalar:13f")

		revObs := make([]audit.NormalizedObservation, len(snapFwd.NormalizedObservations))
		for i, o := range snapFwd.NormalizedObservations {
			revObs[len(snapFwd.NormalizedObservations)-1-i] = o
		}
		now := time.Now().UTC()
		snapRev := &audit.EvidenceSnapshot{
			SnapshotID:             "snap:scalar:13f",
			AuditRunID:             "audit:scalar:13",
			NormalizationVersion:   "v1.6.0",
			SnapshotStatus:         audit.SnapshotFrozen,
			FrozenAt:               &now,
			NormalizedObservations: revObs,
		}

		fixedTime := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
		eng.SetClock(func() time.Time { return fixedTime })

		for _, ruleID := range []string{"AR-CANON-008", "AR-CANON-009"} {
			resFwd, err := eng.EvaluateRule(ctx, snapFwd, ruleID)
			if err != nil {
				t.Fatalf("eval fwd %s failed: %v", ruleID, err)
			}
			resRev, err := eng.EvaluateRule(ctx, snapRev, ruleID)
			if err != nil {
				t.Fatalf("eval rev %s failed: %v", ruleID, err)
			}

			if len(resFwd) != len(resRev) {
				t.Fatalf("%s result length mismatch: %d vs %d", ruleID, len(resFwd), len(resRev))
			}
			if resFwd[0].Status != resRev[0].Status {
				t.Errorf("%s status mismatch: %s vs %s", ruleID, resFwd[0].Status, resRev[0].Status)
			}
			if resFwd[0].ObservedSummary != resRev[0].ObservedSummary {
				t.Errorf("%s summary mismatch: %q vs %q", ruleID, resFwd[0].ObservedSummary, resRev[0].ObservedSummary)
			}
			if len(resFwd[0].EvidenceRefs) != len(resRev[0].EvidenceRefs) {
				t.Errorf("%s evidence refs length mismatch: %d vs %d", ruleID, len(resFwd[0].EvidenceRefs), len(resRev[0].EvidenceRefs))
			}
		}
	})

	// 14. Existing seven rules remain green
	t.Run("14. existing seven rules remain green", func(t *testing.T) {
		snap := newRedirectSnapshot([]redirectTestFixture{
			{
				subjectRef:   "url:1",
				urlValue:     "https://example.com/page",
				initialValue: "false",
			},
		}, "audit:scalar:14", "snap:scalar:14")

		existingSeven := []string{
			"AR-ACC-004",
			"AR-CANON-003",
			"AR-CANON-004",
			"AR-CANON-006",
			"AR-CANON-007",
			"AR-INDEX-001",
			"AR-INDEX-002",
		}

		for _, ruleID := range existingSeven {
			results, err := eng.EvaluateRule(ctx, snap, ruleID)
			if err != nil {
				t.Errorf("eval %s failed: %v", ruleID, err)
			}
			for _, r := range results {
				if r.Status == audit.StatusFail {
					t.Errorf("rule %s failed unexpectedly: %+v", ruleID, r)
				}
			}
		}
	})

	// 15. Hermetic redirect integration green
	t.Run("15. hermetic redirect integration green", func(t *testing.T) {
		mux := http.NewServeMux()
		var srvURL string
		mux.HandleFunc("/entry", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srvURL+"/dest", http.StatusMovedPermanently)
		})
		mux.HandleFunc("/dest", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head><title>Dest</title><link rel="canonical" href="%s/dest"></head><body><h1>Dest</h1></body></html>`, srvURL)
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()
		srvURL = srv.URL

		db, err := standalone.OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open db failed: %v", err)
		}
		defer db.Close()

		runner := sitecrawl.NewRunner(db)
		if err := runner.EnsureSchema(); err != nil {
			t.Fatalf("schema failed: %v", err)
		}

		started, err := runner.Crawl(ctx, []string{srv.URL + "/entry"}, sitecrawl.Options{
			Mode:            sitecrawl.ModeSpider,
			MaxDepth:        2,
			MaxURLs:         5,
			Concurrency:     1,
			FollowRedirects: true,
		})
		if err != nil {
			t.Fatalf("crawl failed: %v", err)
		}

		buildRes, err := adapter.Build(ctx, db, adapter.BuildRequest{
			CrawlRunID: started.ID,
			AuditRunID: "audit:run:hermetic15",
			SnapshotID: "snap:run:hermetic15",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		snap := buildRes.EvidenceSnapshot
		res008, err := eng.EvaluateRule(ctx, snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("008 failed: %v", err)
		}
		res009, err := eng.EvaluateRule(ctx, snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("009 failed: %v", err)
		}

		entrySubj := findSubjectRefByObservedURL(snap, srv.URL+"/entry")
		if entrySubj == "" {
			t.Fatalf("entry subject not found")
		}

		var found008, found009 bool
		for _, r := range res008 {
			if r.SubjectRef == entrySubj {
				found008 = true
				if r.Status != audit.StatusPass {
					t.Errorf("expected 008 PASS, got %s", r.Status)
				}
			}
		}
		for _, r := range res009 {
			if r.SubjectRef == entrySubj {
				found009 = true
				if r.Status != audit.StatusPass {
					t.Errorf("expected 009 PASS, got %s", r.Status)
				}
			}
		}

		if !found008 || !found009 {
			t.Fatalf("missing results for entry: found008=%v, found009=%v", found008, found009)
		}
	})
}
