package engine_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

func TestEngine_RedirectBatch_HermeticIntegration(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	existingSevenRules := []string{
		"AR-ACC-004",
		"AR-CANON-003",
		"AR-CANON-004",
		"AR-CANON-006",
		"AR-CANON-007",
		"AR-INDEX-001",
		"AR-INDEX-002",
	}

	// 1. Hermetic One-Hop Redirect: /start -> /target (200 OK)
	t.Run("1. hermetic one-hop redirect: CANON-008 PASS, CANON-009 PASS", func(t *testing.T) {
		mux := http.NewServeMux()
		var srvURL string
		mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srvURL+"/target", http.StatusMovedPermanently)
		})
		mux.HandleFunc("/target", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head><title>Target</title><link rel="canonical" href="%s/target"></head><body><h1>Target</h1></body></html>`, srvURL)
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()
		srvURL = srv.URL

		db, err := standalone.OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open memory db failed: %v", err)
		}
		defer db.Close()

		runner := sitecrawl.NewRunner(db)
		if err := runner.EnsureSchema(); err != nil {
			t.Fatalf("ensure schema failed: %v", err)
		}

		opts := sitecrawl.Options{
			Mode:            sitecrawl.ModeSpider,
			MaxDepth:        2,
			MaxURLs:         10,
			Concurrency:     1,
			FollowRedirects: true,
		}

		started, err := runner.Crawl(context.Background(), []string{srv.URL + "/start"}, opts)
		if err != nil {
			t.Fatalf("crawl execution failed: %v", err)
		}

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: started.ID,
			AuditRunID: "audit:run:onehop",
			SnapshotID: "snap:run:onehop",
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		snap := buildRes.EvidenceSnapshot
		if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
			t.Fatalf("expected non-nil FROZEN snapshot")
		}

		// Evaluate CANON-008
		res008, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-008 failed: %v", err)
		}
		res008BySubj := mapResultsBySubject(res008)

		// Start URL should PASS CANON-008 (exactly 1 hop)
		startRef := findSubjectRefByObservedURL(snap, srv.URL+"/start")
		if startRef == "" {
			t.Fatalf("could not find subject ref for start URL")
		}
		r008Start, ok := res008BySubj[startRef]
		if !ok {
			t.Fatalf("missing CANON-008 result for start URL %s", startRef)
		}
		if r008Start.Status != audit.StatusPass {
			t.Errorf("expected CANON-008 PASS for one-hop redirect, got %s (observed: %s)", r008Start.Status, r008Start.ObservedSummary)
		}
		validateEvidenceRefsTraceable(t, snap, r008Start)

		// Evaluate CANON-009
		res009, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-009 failed: %v", err)
		}
		res009BySubj := mapResultsBySubject(res009)
		r009Start, ok := res009BySubj[startRef]
		if !ok {
			t.Fatalf("missing CANON-009 result for start URL %s", startRef)
		}
		if r009Start.Status != audit.StatusPass {
			t.Errorf("expected CANON-009 PASS for one-hop redirect without loop, got %s (observed: %s)", r009Start.Status, r009Start.ObservedSummary)
		}
		validateEvidenceRefsTraceable(t, snap, r009Start)

		// Target URL should be NOT_APPLICABLE for both (no redirect)
		targetRef := findSubjectRefByObservedURL(snap, srv.URL+"/target")
		if targetRef != "" {
			if r008Target, ok := res008BySubj[targetRef]; ok && r008Target.Status != audit.StatusNotApplicable {
				t.Errorf("expected target URL NOT_APPLICABLE for CANON-008, got %s", r008Target.Status)
			}
			if r009Target, ok := res009BySubj[targetRef]; ok && r009Target.Status != audit.StatusNotApplicable {
				t.Errorf("expected target URL NOT_APPLICABLE for CANON-009, got %s", r009Target.Status)
			}
		}

		// Regress existing 7 evaluators
		for _, ruleID := range existingSevenRules {
			results, err := eng.EvaluateRule(context.Background(), snap, ruleID)
			if err != nil {
				t.Errorf("regression error on %s: %v", ruleID, err)
			}
			for _, r := range results {
				validateEvidenceRefsTraceable(t, snap, r)
			}
		}
	})

	// 2. Hermetic Multi-Hop Redirect: /hop0 -> /hop1 -> /final (200 OK)
	t.Run("2. hermetic multi-hop redirect: CANON-008 FAIL, CANON-009 PASS", func(t *testing.T) {
		mux := http.NewServeMux()
		var srvURL string
		mux.HandleFunc("/hop0", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srvURL+"/hop1", http.StatusMovedPermanently)
		})
		mux.HandleFunc("/hop1", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srvURL+"/final", http.StatusFound)
		})
		mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!doctype html><html><head><title>Final</title></head><body><h1>Final</h1></body></html>`)
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()
		srvURL = srv.URL

		db, err := standalone.OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open memory db failed: %v", err)
		}
		defer db.Close()

		runner := sitecrawl.NewRunner(db)
		if err := runner.EnsureSchema(); err != nil {
			t.Fatalf("ensure schema failed: %v", err)
		}

		opts := sitecrawl.Options{
			Mode:            sitecrawl.ModeSpider,
			MaxDepth:        3,
			MaxURLs:         10,
			Concurrency:     1,
			FollowRedirects: true,
		}

		started, err := runner.Crawl(context.Background(), []string{srv.URL + "/hop0"}, opts)
		if err != nil {
			t.Fatalf("crawl execution failed: %v", err)
		}

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: started.ID,
			AuditRunID: "audit:run:multihop",
			SnapshotID: "snap:run:multihop",
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		snap := buildRes.EvidenceSnapshot
		if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
			t.Fatalf("expected non-nil FROZEN snapshot")
		}

		startRef := findSubjectRefByObservedURL(snap, srv.URL+"/hop0")
		if startRef == "" {
			t.Fatalf("could not find subject ref for hop0 URL")
		}

		// Evaluate CANON-008 -> FAIL (2 hops >= 2)
		res008, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-008 failed: %v", err)
		}
		res008BySubj := mapResultsBySubject(res008)
		r008Hop0, ok := res008BySubj[startRef]
		if !ok {
			t.Fatalf("missing CANON-008 result for hop0")
		}
		if r008Hop0.Status != audit.StatusFail {
			t.Errorf("expected CANON-008 FAIL for multi-hop, got %s (observed: %s)", r008Hop0.Status, r008Hop0.ObservedSummary)
		}
		validateEvidenceRefsTraceable(t, snap, r008Hop0)

		// Evaluate CANON-009 -> PASS (complete, no loop)
		res009, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-009 failed: %v", err)
		}
		res009BySubj := mapResultsBySubject(res009)
		r009Hop0, ok := res009BySubj[startRef]
		if !ok {
			t.Fatalf("missing CANON-009 result for hop0")
		}
		if r009Hop0.Status != audit.StatusPass {
			t.Errorf("expected CANON-009 PASS for multi-hop, got %s (observed: %s)", r009Hop0.Status, r009Hop0.ObservedSummary)
		}
		validateEvidenceRefsTraceable(t, snap, r009Hop0)

		// Regress existing 7 evaluators
		for _, ruleID := range existingSevenRules {
			results, err := eng.EvaluateRule(context.Background(), snap, ruleID)
			if err != nil {
				t.Errorf("regression error on %s: %v", ruleID, err)
			}
			for _, r := range results {
				validateEvidenceRefsTraceable(t, snap, r)
			}
		}
	})

	// 3. Hermetic Redirect Loop: /loop-a <-> /loop-b
	t.Run("3. hermetic redirect loop: CANON-008 UNKNOWN, CANON-009 FAIL", func(t *testing.T) {
		mux := http.NewServeMux()
		var srvURL string
		mux.HandleFunc("/loop-a", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srvURL+"/loop-b", http.StatusFound)
		})
		mux.HandleFunc("/loop-b", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, srvURL+"/loop-a", http.StatusFound)
		})

		srv := httptest.NewServer(mux)
		defer srv.Close()
		srvURL = srv.URL

		db, err := standalone.OpenDB(":memory:")
		if err != nil {
			t.Fatalf("open memory db failed: %v", err)
		}
		defer db.Close()

		runner := sitecrawl.NewRunner(db)
		if err := runner.EnsureSchema(); err != nil {
			t.Fatalf("ensure schema failed: %v", err)
		}

		opts := sitecrawl.Options{
			Mode:            sitecrawl.ModeSpider,
			MaxDepth:        5,
			MaxURLs:         10,
			Concurrency:     1,
			FollowRedirects: true,
		}

		started, err := runner.Crawl(context.Background(), []string{srv.URL + "/loop-a"}, opts)
		if err != nil {
			t.Fatalf("crawl execution failed: %v", err)
		}

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: started.ID,
			AuditRunID: "audit:run:loop",
			SnapshotID: "snap:run:loop",
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		snap := buildRes.EvidenceSnapshot
		if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
			t.Fatalf("expected non-nil FROZEN snapshot")
		}

		loopRef := findSubjectRefByObservedURL(snap, srv.URL+"/loop-a")
		if loopRef == "" {
			t.Fatalf("could not find subject ref for loop-a URL")
		}

		// Evaluate CANON-008 -> UNKNOWN (incomplete traversal must NOT falsely fail)
		res008, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-008")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-008 failed: %v", err)
		}
		res008BySubj := mapResultsBySubject(res008)
		r008Loop, ok := res008BySubj[loopRef]
		if !ok {
			t.Fatalf("missing CANON-008 result for loop-a")
		}
		if r008Loop.Status != audit.StatusUnknown {
			t.Errorf("expected CANON-008 UNKNOWN for incomplete loop chain, got %s (observed: %s)", r008Loop.Status, r008Loop.ObservedSummary)
		}
		validateEvidenceRefsTraceable(t, snap, r008Loop)

		// Evaluate CANON-009 -> FAIL (confirmed loop detected)
		res009, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-009")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-009 failed: %v", err)
		}
		res009BySubj := mapResultsBySubject(res009)
		r009Loop, ok := res009BySubj[loopRef]
		if !ok {
			t.Fatalf("missing CANON-009 result for loop-a")
		}
		if r009Loop.Status != audit.StatusFail {
			t.Errorf("expected CANON-009 FAIL for loop, got %s (observed: %s)", r009Loop.Status, r009Loop.ObservedSummary)
		}
		validateEvidenceRefsTraceable(t, snap, r009Loop)

		// Regress existing 7 evaluators
		for _, ruleID := range existingSevenRules {
			results, err := eng.EvaluateRule(context.Background(), snap, ruleID)
			if err != nil {
				t.Errorf("regression error on %s: %v", ruleID, err)
			}
			for _, r := range results {
				validateEvidenceRefsTraceable(t, snap, r)
			}
		}
	})
}

func mapResultsBySubject(results []audit.RuleResult) map[string]audit.RuleResult {
	m := make(map[string]audit.RuleResult, len(results))
	for _, r := range results {
		m[r.SubjectRef] = r
	}
	return m
}

func findSubjectRefByObservedURL(snap *audit.EvidenceSnapshot, targetURL string) string {
	for _, obs := range snap.NormalizedObservations {
		if obs.Field == "url_identity" && strings.TrimSpace(obs.Value) == targetURL {
			return obs.SubjectRef
		}
	}
	return ""
}

func validateEvidenceRefsTraceable(t *testing.T, snap *audit.EvidenceSnapshot, r audit.RuleResult) {
	t.Helper()

	// Check metadata contracts
	if r.RuleResultID == "" {
		t.Errorf("empty RuleResultID on result %+v", r)
	}
	if r.ParentCheck == "" {
		t.Errorf("empty ParentCheck on result %+v", r)
	}
	if (r.RuleID == "AR-CANON-008" || r.RuleID == "AR-CANON-009") && r.ParentCheck != "CANON-007" {
		t.Errorf("unexpected ParentCheck for redirect rule %s: %s", r.RuleID, r.ParentCheck)
	}
	if r.Scope != audit.ScopeURL {
		t.Errorf("expected ScopeURL, got %s", r.Scope)
	}

	// Verify all EvidenceRefs point to actual ObservationIDs in snapshot
	obsMap := make(map[string]bool, len(snap.NormalizedObservations))
	for _, o := range snap.NormalizedObservations {
		obsMap[string(o.ObservationID)] = true
	}

	seenRefIDs := make(map[audit.RuleEvidenceRefID]bool)
	var lastRefID audit.RuleEvidenceRefID
	for i, ref := range r.EvidenceRefs {
		if !obsMap[ref.EvidenceRef] {
			t.Errorf("evidence ref %s points to non-existent observation %s", ref.RuleEvidenceRefID, ref.EvidenceRef)
		}
		if seenRefIDs[ref.RuleEvidenceRefID] {
			t.Errorf("duplicate RuleEvidenceRefID %s in result %s", ref.RuleEvidenceRefID, r.RuleResultID)
		}
		seenRefIDs[ref.RuleEvidenceRefID] = true

		if i > 0 && ref.RuleEvidenceRefID <= lastRefID {
			t.Errorf("evidence refs not strictly sorted: %s followed by %s", lastRefID, ref.RuleEvidenceRefID)
		}
		lastRefID = ref.RuleEvidenceRefID
	}
}
