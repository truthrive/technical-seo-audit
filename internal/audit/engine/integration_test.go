package engine_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// TestEngine_AR_ACC_004_EndToEndIntegration performs a hermetic integration test:
// httptest.Server -> sitecrawl.Runner -> completed crawl -> adapter.Build -> FROZEN snapshot -> engine.EvaluateRule -> RuleResult
func TestEngine_AR_ACC_004_EndToEndIntegration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home</title></head>
<body><nav><a href="/ok">OK Page</a> <a href="/server-error">Error Page</a></nav><h1>Home</h1></body></html>`)
	})
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>OK</title></head><body><h1>OK</h1></body></html>`)
	})
	mux.HandleFunc("/server-error", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, `500 Server Error`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Crawl via SiteCrawl
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

	// 2. Adapt to Frozen EvidenceSnapshot
	buildReq := adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:run:integration",
		SnapshotID: "snap:run:integration",
	}

	buildRes, err := adapter.Build(context.Background(), db, buildReq)
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := buildRes.EvidenceSnapshot
	if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Fatalf("expected non-nil FROZEN snapshot")
	}

	// 3. Initialize Rule Engine and evaluate AR-ACC-004
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	results, err := eng.EvaluateRule(context.Background(), snap, "AR-ACC-004")
	if err != nil {
		t.Fatalf("EvaluateRule AR-ACC-004 failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 RuleResults for 3 crawled pages, got %d", len(results))
	}

	// Map URL string to rule status
	resultsByURL := make(map[string]audit.RuleResult)
	for _, r := range results {
		// Find URL value from context evidence ref
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "url" {
				resultsByURL[ref.ObservedValue] = r
			}
		}
	}

	rootURL := srv.URL + "/"
	okURL := srv.URL + "/ok"
	errURL := srv.URL + "/server-error"

	// Verify / is PASS
	rRoot, ok := resultsByURL[rootURL]
	if !ok {
		t.Fatalf("missing result for root URL %s", rootURL)
	}
	if rRoot.Status != audit.StatusPass {
		t.Errorf("expected root URL PASS, got %s (observed: %s)", rRoot.Status, rRoot.ObservedSummary)
	}

	// Verify /ok is PASS
	rOK, ok := resultsByURL[okURL]
	if !ok {
		t.Fatalf("missing result for /ok URL %s", okURL)
	}
	if rOK.Status != audit.StatusPass {
		t.Errorf("expected /ok URL PASS, got %s (observed: %s)", rOK.Status, rOK.ObservedSummary)
	}

	// Verify /server-error is FAIL
	rErr, ok := resultsByURL[errURL]
	if !ok {
		t.Fatalf("missing result for /server-error URL %s", errURL)
	}
	if rErr.Status != audit.StatusFail {
		t.Errorf("expected /server-error URL FAIL, got %s (observed: %s)", rErr.Status, rErr.ObservedSummary)
	}

	// Validate metadata consistency across all results
	for _, r := range results {
		if r.RuleID != "AR-ACC-004" {
			t.Errorf("expected RuleID AR-ACC-004, got %q", r.RuleID)
		}
		if r.ParentCheck != "ACC-007" {
			t.Errorf("expected ParentCheck ACC-007, got %q", r.ParentCheck)
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
		if len(r.EvidenceRefs) < 2 {
			t.Errorf("expected at least 2 evidence refs (url context + fetch_status primary), got %d", len(r.EvidenceRefs))
		}
	}
}

// TestEngine_AR_INDEX_002_EndToEndIntegration performs a hermetic integration test:
// httptest.Server -> sitecrawl.Runner -> completed crawl -> adapter.Build -> FROZEN snapshot -> engine.EvaluateRule(AR-INDEX-002) -> RuleResults
func TestEngine_AR_INDEX_002_EndToEndIntegration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home</title></head>
<body><nav>
<a href="/no-directive">No Directive</a>
<a href="/meta-pass">Meta Pass</a>
<a href="/meta-conflict">Meta Conflict</a>
<a href="/cross-conflict">Cross Conflict</a>
<a href="/diff-scope">Diff Scope</a>
</nav><h1>Home</h1></body></html>`)
	})
	mux.HandleFunc("/no-directive", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>No Directive</title></head><body><h1>No Directive</h1></body></html>`)
	})
	mux.HandleFunc("/meta-pass", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Meta Pass</title><meta name="robots" content="index, follow"></head><body><h1>Meta Pass</h1></body></html>`)
	})
	mux.HandleFunc("/meta-conflict", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Meta Conflict</title><meta name="robots" content="index, noindex"></head><body><h1>Meta Conflict</h1></body></html>`)
	})
	mux.HandleFunc("/cross-conflict", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "noindex")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Cross Conflict</title><meta name="robots" content="index"></head><body><h1>Cross Conflict</h1></body></html>`)
	})
	mux.HandleFunc("/diff-scope", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Diff Scope</title><meta name="robots" content="index"><meta name="googlebot" content="noindex"></head><body><h1>Diff Scope</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Crawl via SiteCrawl
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

	// 2. Adapt to Frozen EvidenceSnapshot
	buildReq := adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:run:integration:index002",
		SnapshotID: "snap:run:integration:index002",
	}

	buildRes, err := adapter.Build(context.Background(), db, buildReq)
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := buildRes.EvidenceSnapshot
	if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Fatalf("expected non-nil FROZEN snapshot")
	}

	// 3. Initialize Rule Engine and evaluate AR-INDEX-002
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	results, err := eng.EvaluateRule(context.Background(), snap, "AR-INDEX-002")
	if err != nil {
		t.Fatalf("EvaluateRule AR-INDEX-002 failed: %v", err)
	}

	// Map URL string to rule result
	resultsByURL := make(map[string]audit.RuleResult)
	for _, r := range results {
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "url" {
				resultsByURL[ref.ObservedValue] = r
			}
		}
	}

	// Verify /no-directive is NOT_APPLICABLE
	if r, ok := resultsByURL[srv.URL+"/no-directive"]; !ok {
		t.Errorf("missing /no-directive result")
	} else if r.Status != audit.StatusNotApplicable {
		t.Errorf("expected /no-directive NOT_APPLICABLE, got %s", r.Status)
	}

	// Verify /meta-pass is PASS
	if r, ok := resultsByURL[srv.URL+"/meta-pass"]; !ok {
		t.Errorf("missing /meta-pass result")
	} else if r.Status != audit.StatusPass {
		t.Errorf("expected /meta-pass PASS, got %s", r.Status)
	}

	// Verify /meta-conflict is FAIL
	if r, ok := resultsByURL[srv.URL+"/meta-conflict"]; !ok {
		t.Errorf("missing /meta-conflict result")
	} else if r.Status != audit.StatusFail {
		t.Errorf("expected /meta-conflict FAIL, got %s", r.Status)
	}

	// Verify /cross-conflict is FAIL
	if r, ok := resultsByURL[srv.URL+"/cross-conflict"]; !ok {
		t.Errorf("missing /cross-conflict result")
	} else if r.Status != audit.StatusFail {
		t.Errorf("expected /cross-conflict FAIL, got %s", r.Status)
	}

	// Verify /diff-scope is PASS
	if r, ok := resultsByURL[srv.URL+"/diff-scope"]; !ok {
		t.Errorf("missing /diff-scope result")
	} else if r.Status != audit.StatusPass {
		t.Errorf("expected /diff-scope PASS, got %s", r.Status)
	}
}

// TestEngine_CanonicalBatch_EndToEndIntegration tests AR-CANON-004, AR-CANON-006, and AR-CANON-007
// through the full hermetic pipeline: httptest.Server -> sitecrawl.Runner -> adapter.Build -> engine.EvaluateRule.
func TestEngine_CanonicalBatch_EndToEndIntegration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home</title>
<link rel="canonical" href="/target-ok">
</head><body><a href="/target-ok">Target OK</a> <a href="/target-noindex">Target Noindex</a> <a href="/dup">Dup</a></body></html>`)
	})
	mux.HandleFunc("/target-ok", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "index, follow")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Target OK</title>
<link rel="canonical" href="/target-ok">
<meta name="robots" content="index, follow">
</head><body><h1>Target OK</h1></body></html>`)
	})
	mux.HandleFunc("/target-noindex", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "noindex")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Target Noindex</title>
<link rel="canonical" href="/target-noindex">
<meta name="robots" content="noindex">
</head><body><h1>Target Noindex</h1></body></html>`)
	})
	mux.HandleFunc("/dup", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Multiple declarations to same target
		fmt.Fprintf(w, `<!doctype html><html><head><title>Dup Canonical</title>
<link rel="canonical" href="/target-ok">
<link rel="canonical" href="/target-ok">
</head><body><h1>Dup</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

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

	buildReq := adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:run:canon_integration",
		SnapshotID: "snap:run:canon_integration",
	}

	buildRes, err := adapter.Build(context.Background(), db, buildReq)
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := buildRes.EvidenceSnapshot
	if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Fatalf("expected non-nil FROZEN snapshot")
	}

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	rootURL := srv.URL + "/"

	// 1. Evaluate AR-CANON-004
	res004, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-004")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-004 failed: %v", err)
	}
	map004 := make(map[string]audit.RuleResult)
	for _, r := range res004 {
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "url" {
				map004[ref.ObservedValue] = r
			}
		}
	}
	// Home has 1 canonical -> PASS
	if r, ok := map004[rootURL]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected home to PASS AR-CANON-004, got %v", r.Status)
	}
	// /dup has 2 canonicals pointing to same target -> WARNING
	if r, ok := map004[srv.URL+"/dup"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /dup to WARNING AR-CANON-004, got %v", r.Status)
	}

	// 2. Evaluate AR-CANON-006
	res006, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-006")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-006 failed: %v", err)
	}
	map006 := make(map[string]audit.RuleResult)
	for _, r := range res006 {
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "url" {
				map006[ref.ObservedValue] = r
			}
		}
	}
	// Home points to /target-ok which returned 200 -> PASS
	if r, ok := map006[rootURL]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected home to PASS AR-CANON-006, got %v", r.Status)
	}

	// 3. Evaluate AR-CANON-007
	res007, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-007")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-007 failed: %v", err)
	}
	map007 := make(map[string]audit.RuleResult)
	for _, r := range res007 {
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "url" {
				map007[ref.ObservedValue] = r
			}
		}
	}
	// Home points to /target-ok which has effective_noindex=false -> PASS
	if r, ok := map007[rootURL]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected home to PASS AR-CANON-007, got %v", r.Status)
	}
}

// TestEngine_Batch1_EndToEndIntegration verifies the entire pipeline:
// SiteCrawl -> Adapter -> Rule Engine -> Findings across all 6 Batch 1 rules:
// AR-INDEX-003, AR-ACC-002, AR-ACC-003, AR-CANON-010, AR-LINK-005, AR-LINK-006.
func TestEngine_Batch1_EndToEndIntegration(t *testing.T) {
	mux := http.NewServeMux()

	var srvURL string

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home</title></head>
<body>
<nav>
  <a href="%s/valid-robots">Valid Robots</a>
  <a href="%s/invalid-robots">Invalid Robots</a>
  <a href="%s/redirect-start">Redirect Page</a>
</nav>
<h1>Home</h1>
</body></html>`, srvURL, srvURL, srvURL)
	})

	mux.HandleFunc("/valid-robots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head>
<meta name="robots" content="noindex, follow">
<title>Valid Robots</title></head>
<body><h1>Valid</h1></body></html>`)
	})

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "User-agent: *\nAllow: /\nSitemap: %s/sitemap.xml\n", srvURL)
	})

	mux.HandleFunc("/sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/valid-robots</loc></url>
  <url><loc>%s/orphan-page</loc></url>
</urlset>`, srvURL, srvURL)
	})

	mux.HandleFunc("/invalid-robots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head>
<meta name="robots" content="max-snippet:invalid">
<title>Invalid Robots</title></head>
<body><h1>Invalid</h1></body></html>`)
	})

	mux.HandleFunc("/redirect-start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srvURL+"/redirect-target", http.StatusMovedPermanently)
	})

	mux.HandleFunc("/redirect-target", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Target</title></head><body>Final Target</body></html>`)
	})

	mux.HandleFunc("/orphan-page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Orphan</title></head><body>Orphan</body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	// 1. Crawl via SiteCrawl with native sitemap discovery (production data path)
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
		Mode:             sitecrawl.ModeSpider,
		MaxDepth:         3,
		MaxURLs:          20,
		Concurrency:      1,
		FollowRedirects:  true,
		RespectRobots:    true,
		DiscoverSitemaps: true, // Native sitemap discovery: populates sitemaps and admits frontier
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL + "/"}, opts)
	if err != nil {
		t.Fatalf("crawl execution failed: %v", err)
	}

	// 2. Adapt to Frozen EvidenceSnapshot
	buildReq := adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:run:batch1_integration",
		SnapshotID: "snap:run:batch1_integration",
	}

	buildRes, err := adapter.Build(context.Background(), db, buildReq)
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := buildRes.EvidenceSnapshot
	if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Fatalf("expected non-nil FROZEN snapshot")
	}

	// 3. Construct Policies for ACC-002, ACC-003, and LINK-006
	validRobotsURL := srv.URL + "/valid-robots"
	policies, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
		{
			PolicyAssignmentID: "pol:acc002",
			AuditRunID:         buildReq.AuditRunID,
			PolicyKey:          audit.PolicyKeyExpectedCrawlable,
			PolicyValue:        "true",
			Scope:              audit.PolicyScopeURL,
			TargetRef:          validRobotsURL,
			Source:             audit.PolicyProvenanceUserInput,
		},
		{
			PolicyAssignmentID: "pol:acc003",
			AuditRunID:         buildReq.AuditRunID,
			PolicyKey:          audit.PolicyKeyExpectedURLState,
			PolicyValue:        "live",
			Scope:              audit.PolicyScopeURL,
			TargetRef:          validRobotsURL,
			Source:             audit.PolicyProvenanceUserInput,
		},
		{
			PolicyAssignmentID: "pol:link006",
			AuditRunID:         buildReq.AuditRunID,
			PolicyKey:          audit.PolicyKeyPriorityPage,
			PolicyValue:        "true",
			Scope:              audit.PolicyScopeURL,
			TargetRef:          validRobotsURL,
			Source:             audit.PolicyProvenanceUserInput,
		},
	})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed: %v", err)
	}

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	evalCtx := engine.EvaluationContext{
		Snapshot: snap,
		Policies: policies,
	}

	// Helper to index results by URL
	mapResults := func(res []audit.RuleResult) map[string]audit.RuleResult {
		m := make(map[string]audit.RuleResult)
		for _, r := range res {
			for _, ref := range r.EvidenceRefs {
				if ref.Field == "url" {
					m[ref.ObservedValue] = r
					break
				}
			}
		}
		return m
	}

	// 1. Test AR-INDEX-003
	resIndex003, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-INDEX-003")
	if err != nil {
		t.Fatalf("EvaluateRule AR-INDEX-003 failed: %v", err)
	}
	mIndex003 := mapResults(resIndex003)
	if r, ok := mIndex003[validRobotsURL]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected valid-robots to PASS AR-INDEX-003, got %v", r.Status)
	}
	if r, ok := mIndex003[srv.URL+"/invalid-robots"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected invalid-robots to WARNING AR-INDEX-003, got %v", r.Status)
	}

	// 2. Test AR-CANON-010
	resCanon010, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-CANON-010")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-010 failed: %v", err)
	}
	mCanon010 := mapResults(resCanon010)
	if r, ok := mCanon010[srv.URL+"/redirect-start"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected redirect-start to PASS AR-CANON-010, got %v", r.Status)
	}

	// 3. Test AR-LINK-005
	resLink005, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-LINK-005")
	if err != nil {
		t.Fatalf("EvaluateRule AR-LINK-005 failed: %v", err)
	}
	mLink005 := mapResults(resLink005)
	if r, ok := mLink005[validRobotsURL]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected valid-robots (discovered in sitemap + inlinks > 0) to PASS AR-LINK-005, got %v", r.Status)
	}
	if r, ok := mLink005[srv.URL+"/orphan-page"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected orphan-page (discovered in sitemap + inlinks == 0) to WARNING AR-LINK-005, got %v", r.Status)
	}

	// 4. Test AR-LINK-006
	resLink006, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-LINK-006")
	if err != nil {
		t.Fatalf("EvaluateRule AR-LINK-006 failed: %v", err)
	}
	mLink006 := mapResults(resLink006)
	if r, ok := mLink006[validRobotsURL]; !ok || r.Status != audit.StatusManualReview {
		t.Errorf("expected valid-robots to be MANUAL_REVIEW for AR-LINK-006, got %v", r.Status)
	}

	// 5. Test AR-ACC-002
	resAcc002, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-ACC-002")
	if err != nil {
		t.Fatalf("EvaluateRule AR-ACC-002 failed: %v", err)
	}
	mAcc002 := mapResults(resAcc002)
	if r, ok := mAcc002[validRobotsURL]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected valid-robots to PASS AR-ACC-002, got %v", r.Status)
	}

	// 6. Test AR-ACC-003
	resAcc003, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-ACC-003")
	if err != nil {
		t.Fatalf("EvaluateRule AR-ACC-003 failed: %v", err)
	}
	mAcc003 := mapResults(resAcc003)
	if r, ok := mAcc003[validRobotsURL]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected valid-robots to PASS AR-ACC-003, got %v", r.Status)
	}
}

// TestEngine_MVP_Batch2_EndToEndIntegration tests the 5 rules implemented in Fast-Track MVP Batch 2:
// AR-ACC-001, AR-CANON-001, AR-CANON-002, AR-INDEX-004, AR-INDEX-006.
func TestEngine_MVP_Batch2_EndToEndIntegration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home</title></head>
<body><nav>
<a href="/about/">About Slash</a>
<a href="/about">About Non-Slash</a>
<a href="/file.pdf">PDF File</a>
<a href="/soft404">Soft 404 Page</a>
<a href="/real-page">Real Page</a>
</nav><h1>Home</h1></body></html>`)
	})

	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/about/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/about/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>About</title></head><body><h1>About Us</h1></body></html>`)
	})
	mux.HandleFunc("/file.pdf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		fmt.Fprintf(w, `%%PDF-1.4 file content`)
	})
	mux.HandleFunc("/soft404", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Matches the 404 template title and content
		fmt.Fprintf(w, `<!doctype html><html><head><title>Page Not Found</title></head><body><h1>404 Error</h1><p>The page you requested could not be found.</p></body></html>`)
	})
	mux.HandleFunc("/real-page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Real Article</title></head><body><h1>Useful Real Content</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Crawl with SiteCrawl
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
		Mode:            sitecrawl.ModeSpider,
		MaxDepth:        2,
		MaxURLs:         15,
		Concurrency:     1,
		FollowRedirects: true,
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL + "/"}, opts)
	if err != nil {
		t.Fatalf("crawl execution failed: %v", err)
	}

	// 2. Build EvidenceSnapshot with Adapter
	buildReq := adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:run:batch2_integration",
		SnapshotID: "snap:run:batch2_integration",
	}

	buildRes, err := adapter.Build(context.Background(), db, buildReq)
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := buildRes.EvidenceSnapshot
	if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Fatalf("expected non-nil FROZEN snapshot")
	}

	// 3. Inject diagnostic probe and origin variant evidence into snapshot observations
	now := time.Now().UTC()

	// Find the native content fingerprint generated by adapter for /soft404
	var template404FP string
	for _, obs := range snap.NormalizedObservations {
		if obs.Field == "url_identity" && obs.Value == srv.URL+"/soft404" {
			for _, o := range snap.NormalizedObservations {
				if o.SubjectRef == obs.SubjectRef && o.Field == "content_fingerprint" {
					template404FP = o.Value
					break
				}
			}
			break
		}
	}
	if template404FP == "" {
		t.Fatalf("expected content_fingerprint for soft404 page from adapter")
	}

	snap.NormalizedObservations = append(snap.NormalizedObservations,
		// AR-INDEX-004 probe observations on SubjectProbe
		audit.NormalizedObservation{
			ObservationID:      "obs:probe:404:url",
			AuditRunID:         buildReq.AuditRunID,
			SnapshotID:         buildReq.SnapshotID,
			SubjectType:        audit.SubjectProbe,
			SubjectRef:         "probe:missing:404",
			Field:              "missing_probe_url",
			Value:              srv.URL + "/truthrive-missing-probe-404",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"probe_test"},
			ObservedAt:         now,
		},
		audit.NormalizedObservation{
			ObservationID:      "obs:probe:404:st",
			AuditRunID:         buildReq.AuditRunID,
			SnapshotID:         buildReq.SnapshotID,
			SubjectType:        audit.SubjectProbe,
			SubjectRef:         "probe:missing:404",
			Field:              "probe_final_status",
			Value:              "404",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"probe_test"},
			ObservedAt:         now,
		},
		// Global 404 template fingerprint on SubjectSite for AR-INDEX-006
		audit.NormalizedObservation{
			ObservationID:      "obs:site:k404:fp",
			AuditRunID:         buildReq.AuditRunID,
			SnapshotID:         buildReq.SnapshotID,
			SubjectType:        audit.SubjectSite,
			SubjectRef:         "site",
			Field:              "known_404_content_fingerprint",
			Value:              template404FP,
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"probe_test"},
			ObservedAt:         now,
		},
		audit.NormalizedObservation{
			ObservationID:      "obs:site:k404:st",
			AuditRunID:         buildReq.AuditRunID,
			SnapshotID:         buildReq.SnapshotID,
			SubjectType:        audit.SubjectSite,
			SubjectRef:         "site",
			Field:              "known_404_probe_status",
			Value:              "404",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"probe_test"},
			ObservedAt:         now,
		},
		// AR-CANON-001 origin variant observations on SubjectSiteConfiguration
		audit.NormalizedObservation{
			ObservationID:      "obs:canon001:var1",
			AuditRunID:         buildReq.AuditRunID,
			SnapshotID:         buildReq.SnapshotID,
			SubjectType:        audit.SubjectSiteConfiguration,
			SubjectRef:         "cfg:variant:http",
			Field:              "origin_variant",
			Value:              "http://example.com",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"variant_test"},
			ObservedAt:         now,
		},
		audit.NormalizedObservation{
			ObservationID:      "obs:canon001:st1",
			AuditRunID:         buildReq.AuditRunID,
			SnapshotID:         buildReq.SnapshotID,
			SubjectType:        audit.SubjectSiteConfiguration,
			SubjectRef:         "cfg:variant:http",
			Field:              "variant_fetch_status",
			Value:              "301",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"variant_test"},
			ObservedAt:         now,
		},
		audit.NormalizedObservation{
			ObservationID:      "obs:canon001:fin1",
			AuditRunID:         buildReq.AuditRunID,
			SnapshotID:         buildReq.SnapshotID,
			SubjectType:        audit.SubjectSiteConfiguration,
			SubjectRef:         "cfg:variant:http",
			Field:              "variant_final_url",
			Value:              "https://example.com/",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"variant_test"},
			ObservedAt:         now,
		},
	)

	// 4. Policy index with preferred_origin
	policies, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
		{
			PolicyAssignmentID: "pol:batch2:preforigin",
			AuditRunID:         buildReq.AuditRunID,
			PolicyKey:          audit.PolicyKeyPreferredOrigin,
			PolicyValue:        "https://example.com",
			Scope:              audit.PolicyScopeSite,
			TargetRef:          "site",
			Source:             audit.PolicyProvenanceUserInput,
		},
	})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed: %v", err)
	}

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	evalCtx := engine.EvaluationContext{
		Snapshot: snap,
		Policies: policies,
	}

	// Helper to index results by URL
	mapResults := func(res []audit.RuleResult) map[string]audit.RuleResult {
		m := make(map[string]audit.RuleResult)
		for _, r := range res {
			for _, ref := range r.EvidenceRefs {
				if ref.Field == "url" || ref.Field == "site_https_url" || ref.Field == "origin_variant" || ref.Field == "probe_url" {
					m[ref.ObservedValue] = r
					break
				}
			}
			if _, ok := m[string(r.SubjectRef)]; !ok {
				m[string(r.SubjectRef)] = r
			}
		}
		return m
	}

	// Test 1: AR-ACC-001 (HTTPS endpoint reachable)
	resAcc001, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-ACC-001")
	if err != nil {
		t.Fatalf("EvaluateRule AR-ACC-001 failed: %v", err)
	}
	if len(resAcc001) != 1 {
		t.Fatalf("expected 1 result for AR-ACC-001, got %d", len(resAcc001))
	}
	if resAcc001[0].Severity != audit.SeverityP0 {
		t.Errorf("AR-ACC-001 severity = %v, want P0", resAcc001[0].Severity)
	}

	// Test 2: AR-CANON-001 (Origin variants normalize to preferred origin)
	resCanon001, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-CANON-001")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-001 failed: %v", err)
	}
	mCanon001 := mapResults(resCanon001)
	if r, ok := mCanon001["http://example.com"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected http://example.com to PASS AR-CANON-001, got status %v", r.Status)
	}

	// Test 3: AR-CANON-002 (Trailing-slash pair is consistently normalized)
	resCanon002, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-CANON-002")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-002 failed: %v", err)
	}
	mCanon002 := mapResults(resCanon002)
	// /file.pdf should be NOT_APPLICABLE
	if r, ok := mCanon002[srv.URL+"/file.pdf"]; !ok || r.Status != audit.StatusNotApplicable {
		t.Errorf("expected /file.pdf to be NOT_APPLICABLE for AR-CANON-002, got %v", r.Status)
	}
	// /about and /about/ form a normalized pair where /about 301 redirects to /about/
	if r, ok := mCanon002[srv.URL+"/about/"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected /about/ to PASS AR-CANON-002, got %v", r.Status)
	}

	// Test 4: AR-INDEX-004 (Known-missing probe resolves to 404/410)
	resIndex004, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-INDEX-004")
	if err != nil {
		t.Fatalf("EvaluateRule AR-INDEX-004 failed: %v", err)
	}
	mIndex004 := mapResults(resIndex004)
	if r, ok := mIndex004["probe:missing:404"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected probe:missing:404 to PASS AR-INDEX-004, got %v", r.Status)
	}

	// Test 5: AR-INDEX-006 (200 response matches known 404 template)
	resIndex006, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-INDEX-006")
	if err != nil {
		t.Fatalf("EvaluateRule AR-INDEX-006 failed: %v", err)
	}
	mIndex006 := mapResults(resIndex006)
	if r, ok := mIndex006[srv.URL+"/soft404"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /soft404 to WARNING for AR-INDEX-006, got %v", r.Status)
	}
	if r, ok := mIndex006[srv.URL+"/real-page"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected /real-page to PASS for AR-INDEX-006, got %v", r.Status)
	}
}

// TestEngine_Batch3_EndToEndIntegration tests Fast-Track MVP Batch 3 rules:
// - AR-INDEX-005: Snippet restriction conflicts with snippet policy
// - AR-RENDER-001 through AR-RENDER-006: Render comparison rules
func TestEngine_Batch3_EndToEndIntegration(t *testing.T) {
	mux := http.NewServeMux()

	// 1. Unrestricted snippet page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Unrestricted Home</title>
<meta name="robots" content="index, follow">
</head><body><nav>
<a href="/restricted">Restricted Page</a>
<a href="/x-max-snippet">Header Max Snippet</a>
<a href="/googlebot-zero">Googlebot Max Snippet Zero</a>
<a href="/block-nosnippet">Block Data Nosnippet</a>
<a href="/rendered-page">Rendered Page</a>
</nav><h1>Home Page</h1></body></html>`)
	})

	// 2. Page with meta robots nosnippet directive
	mux.HandleFunc("/restricted", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Restricted Page</title>
<meta name="robots" content="index, nosnippet">
</head><body><h1>Restricted Snippet</h1></body></html>`)
	})

	// 3. Page with X-Robots-Tag: max-snippet:50 HTTP header
	mux.HandleFunc("/x-max-snippet", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Add("X-Robots-Tag", "max-snippet:50")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Header Max Snippet</title>
</head><body><h1>Header Max Snippet 50</h1></body></html>`)
	})

	// 4. Page with googlebot meta max-snippet:0 and generic max-snippet:-1 (tests precedence)
	mux.HandleFunc("/googlebot-zero", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Googlebot Zero</title>
<meta name="robots" content="max-snippet:-1">
<meta name="googlebot" content="max-snippet:0">
</head><body><h1>Googlebot Max Snippet Zero</h1></body></html>`)
	})

	// 5. Page with block-level data-nosnippet elements only
	mux.HandleFunc("/block-nosnippet", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Block Data Nosnippet</title>
<meta name="robots" content="index, follow">
</head><body><h1>Article</h1><div data-nosnippet>Sidebar disclaimer</div><span data-nosnippet="true">Promo</span></body></html>`)
	})

	// 6. Page that will be marked rendered=1 in SQLite (simulating SiteCrawl JS rendering overwriting raw HTML)
	mux.HandleFunc("/rendered-page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Rendered Page</title>
</head><body><h1>Rendered Content</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

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
		MaxURLs:     15,
		Concurrency: 1,
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("crawl execution failed: %v", err)
	}

	// Mark /rendered-page as rendered=1 in sitecrawl_pages to verify real adapter handling of rendered pages
	if _, err := db.Exec(`UPDATE sitecrawl_pages SET rendered = 1 WHERE run_id = ? AND url = ?`, started.ID, srv.URL+"/rendered-page"); err != nil {
		t.Fatalf("failed to mark /rendered-page as rendered=1: %v", err)
	}

	buildReq := adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:run:batch3",
		SnapshotID: "snap:run:batch3",
	}

	buildRes, err := adapter.Build(context.Background(), db, buildReq)
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := buildRes.EvidenceSnapshot
	if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Fatalf("expected non-nil FROZEN snapshot")
	}

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	now := time.Now().UTC()

	// Helper to map rule results by URL
	mapByURL := func(results []audit.RuleResult) map[string]audit.RuleResult {
		m := make(map[string]audit.RuleResult, len(results))
		for _, r := range results {
			for _, ref := range r.EvidenceRefs {
				if ref.Field == "url" {
					m[ref.ObservedValue] = r
					break
				}
			}
		}
		return m
	}

	// Sub-test A: AR-INDEX-005 with unspecified snippet policy
	resIndex005Unspecified, err := eng.EvaluateRule(context.Background(), snap, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule AR-INDEX-005 unspecified failed: %v", err)
	}
	mIndex005 := mapByURL(resIndex005Unspecified)
	if r, ok := mIndex005[srv.URL+"/"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected Home to PASS AR-INDEX-005 with unspecified policy, got %v", r)
	}
	if r, ok := mIndex005[srv.URL+"/restricted"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /restricted to WARNING for AR-INDEX-005 with unspecified policy, got %v", r)
	}
	if r, ok := mIndex005[srv.URL+"/x-max-snippet"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /x-max-snippet to WARNING for AR-INDEX-005 with unspecified policy, got %v", r)
	}
	if r, ok := mIndex005[srv.URL+"/googlebot-zero"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /googlebot-zero to WARNING for AR-INDEX-005 with unspecified policy, got %v", r)
	}
	if r, ok := mIndex005[srv.URL+"/block-nosnippet"]; !ok || r.Status != audit.StatusManualReview {
		t.Errorf("expected /block-nosnippet to MANUAL_REVIEW for AR-INDEX-005 with unspecified policy, got %v", r)
	}
	if r, ok := mIndex005[srv.URL+"/rendered-page"]; !ok || r.Status != audit.StatusUnknown {
		t.Errorf("expected /rendered-page to UNKNOWN for AR-INDEX-005 (raw HTML overwritten by render), got %v", r)
	}

	// Sub-test B: AR-INDEX-005 with policy allow_unrestricted
	polAllow, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
		{
			PolicyAssignmentID: "pol:batch3:allow",
			AuditRunID:         "audit:run:batch3",
			PolicyKey:          audit.PolicyKeySnippetPolicy,
			PolicyValue:        audit.PolicyValueSnippetAllowUnrestricted,
			Scope:              audit.PolicyScopeSite,
			TargetRef:          "site",
			Source:             audit.PolicyProvenanceUserInput,
			SuppliedAt:         now,
		},
	})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed: %v", err)
	}
	resIndex005Allow, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polAllow}, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule AR-INDEX-005 allow failed: %v", err)
	}
	mIndex005Allow := mapByURL(resIndex005Allow)
	if r, ok := mIndex005Allow[srv.URL+"/"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected Home to PASS for AR-INDEX-005 under allow_unrestricted policy, got %v", r)
	}
	if r, ok := mIndex005Allow[srv.URL+"/restricted"]; !ok || r.Status != audit.StatusFail {
		t.Errorf("expected /restricted to FAIL for AR-INDEX-005 under allow_unrestricted policy, got %v", r)
	}
	if r, ok := mIndex005Allow[srv.URL+"/x-max-snippet"]; !ok || r.Status != audit.StatusFail {
		t.Errorf("expected /x-max-snippet to FAIL for AR-INDEX-005 under allow_unrestricted policy, got %v", r)
	}
	if r, ok := mIndex005Allow[srv.URL+"/googlebot-zero"]; !ok || r.Status != audit.StatusFail {
		t.Errorf("expected /googlebot-zero to FAIL for AR-INDEX-005 under allow_unrestricted policy, got %v", r)
	}
	if r, ok := mIndex005Allow[srv.URL+"/block-nosnippet"]; !ok || r.Status != audit.StatusFail {
		t.Errorf("expected /block-nosnippet to FAIL for AR-INDEX-005 under allow_unrestricted policy, got %v", r)
	}

	// Sub-test C: AR-INDEX-005 with policy restrict
	polRestrict, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
		{
			PolicyAssignmentID: "pol:batch3:restrict",
			AuditRunID:         "audit:run:batch3",
			PolicyKey:          audit.PolicyKeySnippetPolicy,
			PolicyValue:        audit.PolicyValueSnippetRestrict,
			Scope:              audit.PolicyScopeSite,
			TargetRef:          "site",
			Source:             audit.PolicyProvenanceUserInput,
			SuppliedAt:         now,
		},
	})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed: %v", err)
	}
	resIndex005Restrict, err := eng.EvaluateRuleWithContext(context.Background(), engine.EvaluationContext{Snapshot: snap, Policies: polRestrict}, "AR-INDEX-005")
	if err != nil {
		t.Fatalf("EvaluateRule AR-INDEX-005 restrict failed: %v", err)
	}
	mIndex005Restrict := mapByURL(resIndex005Restrict)
	if r, ok := mIndex005Restrict[srv.URL+"/restricted"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected /restricted to PASS for AR-INDEX-005 under restrict policy, got %v", r)
	}
	if r, ok := mIndex005Restrict[srv.URL+"/x-max-snippet"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected /x-max-snippet to PASS for AR-INDEX-005 under restrict policy, got %v", r)
	}
	if r, ok := mIndex005Restrict[srv.URL+"/block-nosnippet"]; !ok || r.Status != audit.StatusManualReview {
		t.Errorf("expected /block-nosnippet to MANUAL_REVIEW for AR-INDEX-005 under restrict policy, got %v", r)
	}

	// Sub-test D: AR-RENDER-001 through AR-RENDER-006 on real adapter snapshot:
	// - Unrendered pages evaluate to NOT_APPLICABLE
	// - Rendered page (/rendered-page, rendered=1 in SQLite) evaluates to UNKNOWN because SiteCrawl
	//   overwrites raw HTML fields and does not preserve raw vs rendered comparison pairs.
	renderRuleIDs := []string{
		"AR-RENDER-001",
		"AR-RENDER-002",
		"AR-RENDER-003",
		"AR-RENDER-004",
		"AR-RENDER-005",
		"AR-RENDER-006",
	}
	for _, ruleID := range renderRuleIDs {
		renderResults, err := eng.EvaluateRule(context.Background(), snap, ruleID)
		if err != nil {
			t.Fatalf("EvaluateRule %s failed: %v", ruleID, err)
		}
		mRender := mapByURL(renderResults)
		if r, ok := mRender[srv.URL+"/"]; !ok || r.Status != audit.StatusNotApplicable {
			t.Errorf("expected unrendered Home to be NOT_APPLICABLE under %s, got %v", ruleID, r)
		}
		if r, ok := mRender[srv.URL+"/rendered-page"]; !ok || r.Status != audit.StatusUnknown {
			t.Errorf("expected rendered /rendered-page to be UNKNOWN under %s (raw/rendered pair unavailable), got %v", ruleID, r)
		}
	}

	// Sub-test E: Synthetic Rendered Snapshot testing comparison logic
	var renderedObs []audit.NormalizedObservation
	synthSnapID := audit.SnapshotID("snap:synth:batch3")
	for _, o := range snap.NormalizedObservations {
		oCopy := o
		oCopy.SnapshotID = synthSnapID
		renderedObs = append(renderedObs, oCopy)
	}

	// Find URL subject ref for Home
	homeRef := ""
	for _, obs := range snap.NormalizedObservations {
		if obs.Field == "url_identity" && obs.Value == srv.URL+"/" {
			homeRef = obs.SubjectRef
			break
		}
	}
	if homeRef == "" {
		t.Fatalf("could not find home URL subject ref")
	}

	// Add render signal and comparison observations
	renderedObs = append(renderedObs,
		audit.NormalizedObservation{
			ObservationID:      "obs:render:req",
			AuditRunID:         "audit:run:batch3",
			SnapshotID:         synthSnapID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         homeRef,
			Field:              "render_requested",
			Value:              "true",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		},
		audit.NormalizedObservation{
			ObservationID:      "obs:title:raw",
			AuditRunID:         "audit:run:batch3",
			SnapshotID:         synthSnapID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         homeRef,
			Field:              "title_raw",
			Value:              "Unrestricted Home",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		},
		audit.NormalizedObservation{
			ObservationID:      "obs:title:rendered",
			AuditRunID:         "audit:run:batch3",
			SnapshotID:         synthSnapID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         homeRef,
			Field:              "title_rendered",
			Value:              "Unrestricted Home",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		},
		audit.NormalizedObservation{
			ObservationID:      "obs:canon:raw",
			AuditRunID:         "audit:run:batch3",
			SnapshotID:         synthSnapID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         homeRef,
			Field:              "canonical_raw",
			Value:              srv.URL + "/",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		},
		audit.NormalizedObservation{
			ObservationID:      "obs:canon:rendered",
			AuditRunID:         "audit:run:batch3",
			SnapshotID:         synthSnapID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         homeRef,
			Field:              "canonical_rendered",
			Value:              srv.URL + "/client-changed",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		},
	)

	synthSnap := &audit.EvidenceSnapshot{
		SnapshotID:             synthSnapID,
		AuditRunID:             "audit:run:batch3",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: renderedObs,
	}

	// AR-RENDER-001 should PASS for Home (equal titles)
	resR1, err := eng.EvaluateRule(context.Background(), synthSnap, "AR-RENDER-001")
	if err != nil {
		t.Fatalf("EvaluateRule AR-RENDER-001 failed: %v", err)
	}
	for _, r := range resR1 {
		if r.SubjectRef == homeRef {
			if r.Status != audit.StatusPass {
				t.Errorf("expected Home to PASS AR-RENDER-001, got %s", r.Status)
			}
		}
	}

	// AR-RENDER-002 should WARNING for Home (differing canonicals)
	resR2, err := eng.EvaluateRule(context.Background(), synthSnap, "AR-RENDER-002")
	if err != nil {
		t.Fatalf("EvaluateRule AR-RENDER-002 failed: %v", err)
	}
	for _, r := range resR2 {
		if r.SubjectRef == homeRef {
			if r.Status != audit.StatusWarning {
				t.Errorf("expected Home to WARNING for AR-RENDER-002, got %s", r.Status)
			}
		}
	}
}

// TestEngine_Batch4_EndToEndIntegration verifies the end-to-end pipeline:
// SiteCrawl -> Adapter -> Rule Engine -> Findings across all 9 Batch 4 rules
// (AR-ACC-005, AR-AI-001, AR-AI-003, AR-CANON-011, AR-LINK-001, AR-ACC-006,
// AR-ACC-007, AR-ACC-008, AR-AI-002) and verifies dual raw/rendered HTML comparison
// persistence and adapter normalization for AR-RENDER-001..006.
func TestEngine_Batch4_EndToEndIntegration(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, `User-agent: Googlebot
Disallow: /google-blocked

User-agent: OAI-SearchBot
Disallow: /oai-blocked

User-agent: GPTBot
Disallow: /
Allow: /public-ai

User-agent: *
Allow: /
`)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home</title></head>
<body>
<nav>
  <a href="%s/blog?page=1">Blog Page 1</a>
  <a href="%s/google-blocked">Google Blocked</a>
  <a href="%s/oai-blocked">OAI Blocked</a>
  <a href="%s/public-ai">Public AI</a>
</nav>
<main><h1>Welcome Home</h1><p>Main content on home page.</p></main>
</body></html>`, srvURL, srvURL, srvURL, srvURL)
	})

	// Paginated page 1: crawlable <a rel="next" href="..."> -> AR-CANON-011 PASS
	mux.HandleFunc("/blog", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page := r.URL.Query().Get("page")
		switch page {
		case "1":
			fmt.Fprintf(w, `<!doctype html><html><head><title>Blog Page 1</title></head>
<body>
<h1>Blog Page 1</h1>
<a rel="next" href="%s/blog?page=2">Next Page</a>
<a href="%s/blog?page=3">Jump to Page 3</a>
</body></html>`, srvURL, srvURL)
		case "2":
			// Paginated page 2: non-anchor <button rel="next"> without crawlable href -> AR-CANON-011 WARNING
			fmt.Fprintf(w, `<!doctype html><html><head><title>Blog Page 2</title></head>
<body>
<h1>Blog Page 2</h1>
<a rel="prev" href="%s/blog?page=1">Prev Page</a>
<button rel="next" data-next-page="3">Next Page</button>
</body></html>`, srvURL)
		case "3":
			// Paginated page 3 (last page): only rel="prev", no next page -> AR-CANON-011 NOT_APPLICABLE
			fmt.Fprintf(w, `<!doctype html><html><head><title>Blog Page 3</title></head>
<body>
<h1>Blog Page 3</h1>
<a rel="prev" href="%s/blog?page=2">Prev Page</a>
</body></html>`, srvURL)
		default:
			http.NotFound(w, r)
		}
	})

	mux.HandleFunc("/google-blocked", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Google Blocked</title></head><body><h1>Blocked for Googlebot</h1></body></html>`)
	})

	mux.HandleFunc("/oai-blocked", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>OAI Blocked</title></head><body><h1>Blocked for OAI-SearchBot</h1></body></html>`)
	})

	mux.HandleFunc("/public-ai", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Public AI</title></head><body><h1>Allowed for GPTBot</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	// 1. Crawl via SiteCrawl
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
		Mode:            sitecrawl.ModeSpider,
		MaxDepth:        3,
		MaxURLs:         20,
		Concurrency:     1,
		FollowRedirects: true,
		RespectRobots:   true,
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL + "/"}, opts)
	if err != nil {
		t.Fatalf("crawl execution failed: %v", err)
	}

	// Simulate a page with dual raw/rendered HTML comparison persisted via sitecrawl.RenderComparisonData
	renderedCompJSON := `{
		"render_requested": true,
		"render_status": "complete",
		"title_raw": "SSR Title",
		"title_rendered": "SSR Title",
		"canonical_raw": "` + srv.URL + `/public-ai",
		"canonical_rendered": "` + srv.URL + `/public-ai-changed",
		"meta_robots_raw": "index,follow",
		"meta_robots_rendered": "noindex,follow",
		"h1_raw": ["Initial Heading"],
		"h1_rendered": ["Initial Heading"],
		"main_text_present_raw": false,
		"main_text_present_rendered": true,
		"internal_links_raw": ["` + srv.URL + `/"],
		"internal_links_rendered": ["` + srv.URL + `/", "` + srv.URL + `/blog?page=1"]
	}`
	_, err = db.Exec(`UPDATE sitecrawl_pages
		SET rendered = 1,
		    data = json_set(data, '$.rendered', json('true'), '$.render_comparison', json(?))
		WHERE run_id = ? AND url = ?`,
		renderedCompJSON, started.ID, srv.URL+"/public-ai")
	if err != nil {
		t.Fatalf("failed to update /public-ai with RenderComparisonData: %v", err)
	}

	// 2. Adapt to Frozen EvidenceSnapshot
	auditRunID := audit.AuditRunID("audit:run:batch4_integration")
	snapID := audit.SnapshotID("snap:run:batch4_integration")
	buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}
	snap := buildRes.EvidenceSnapshot

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	// Verify 47/47 rules registered
	if got := len(eng.ImplementedRuleIDs()); got != 47 {
		t.Fatalf("expected 47 implemented rules, got %d", got)
	}

	mapByURL := func(res []audit.RuleResult) map[string]audit.RuleResult {
		m := make(map[string]audit.RuleResult)
		for _, r := range res {
			for _, ref := range r.EvidenceRefs {
				if ref.Field == "url" {
					m[ref.ObservedValue] = r
					break
				}
			}
		}
		return m
	}

	// 3. Test Priority A: Robots & AI Crawler Policy Rules (AR-ACC-005, AR-AI-001, AR-AI-003)
	// Without policy:
	// - AR-ACC-005: PASS when allowed, WARNING when disallowed (/google-blocked)
	// - AR-AI-001: PASS when allowed, WARNING when disallowed (/oai-blocked)
	// - AR-AI-003: MANUAL_REVIEW when policy is unspecified
	resAcc005NoPol, err := eng.EvaluateRule(context.Background(), snap, "AR-ACC-005")
	if err != nil {
		t.Fatalf("EvaluateRule AR-ACC-005 (no policy) failed: %v", err)
	}
	mAcc005NoPol := mapByURL(resAcc005NoPol)
	if r, ok := mAcc005NoPol[srv.URL+"/"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("AR-ACC-005 without policy on Home expected PASS, got %v", r)
	}
	if r, ok := mAcc005NoPol[srv.URL+"/google-blocked"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("AR-ACC-005 without policy on /google-blocked expected WARNING, got %v", r)
	}

	resAI001NoPol, err := eng.EvaluateRule(context.Background(), snap, "AR-AI-001")
	if err != nil {
		t.Fatalf("EvaluateRule AR-AI-001 (no policy) failed: %v", err)
	}
	mAI001NoPol := mapByURL(resAI001NoPol)
	if r, ok := mAI001NoPol[srv.URL+"/"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("AR-AI-001 without policy on Home expected PASS, got %v", r)
	}
	if r, ok := mAI001NoPol[srv.URL+"/oai-blocked"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("AR-AI-001 without policy on /oai-blocked expected WARNING, got %v", r)
	}

	resAI003NoPol, err := eng.EvaluateRule(context.Background(), snap, "AR-AI-003")
	if err != nil {
		t.Fatalf("EvaluateRule AR-AI-003 (no policy) failed: %v", err)
	}
	for _, r := range resAI003NoPol {
		if r.Status != audit.StatusManualReview {
			t.Errorf("AR-AI-003 without policy expected MANUAL_REVIEW, got %s", r.Status)
		}
	}

	// With explicit policy:
	// - Googlebot expected allow -> / PASS, /google-blocked FAIL
	// - OAI-SearchBot expected allow -> / PASS, /oai-blocked WARNING
	// - GPTBot expected block -> /google-blocked PASS (disallowed by /), /public-ai WARNING (allowed by /public-ai)
	policies, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
		{
			PolicyAssignmentID: "pol:googlebot",
			AuditRunID:         auditRunID,
			PolicyKey:          audit.PolicyKeyGooglebotAccessPolicy,
			PolicyValue:        audit.PolicyValueAccessAllow,
			Scope:              audit.PolicyScopeSite,
			TargetRef:          string(audit.ScopeSitewide),
			Source:             audit.PolicyProvenanceUserInput,
		},
		{
			PolicyAssignmentID: "pol:oai",
			AuditRunID:         auditRunID,
			PolicyKey:          audit.PolicyKeyOAISearchbotAccessPolicy,
			PolicyValue:        audit.PolicyValueAccessAllow,
			Scope:              audit.PolicyScopeSite,
			TargetRef:          string(audit.ScopeSitewide),
			Source:             audit.PolicyProvenanceUserInput,
		},
		{
			PolicyAssignmentID: "pol:gptbot",
			AuditRunID:         auditRunID,
			PolicyKey:          audit.PolicyKeyGPTBotTrainingPolicy,
			PolicyValue:        audit.PolicyValueAccessBlock,
			Scope:              audit.PolicyScopeSite,
			TargetRef:          string(audit.ScopeSitewide),
			Source:             audit.PolicyProvenanceUserInput,
		},
	})
	if err != nil {
		t.Fatalf("NewPolicyIndex failed: %v", err)
	}

	evalCtx := engine.EvaluationContext{
		Snapshot: snap,
		Policies: policies,
	}

	resAcc005, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-ACC-005")
	if err != nil {
		t.Fatalf("EvaluateRuleWithContext AR-ACC-005 failed: %v", err)
	}
	mAcc005 := mapByURL(resAcc005)
	if r, ok := mAcc005[srv.URL+"/"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected Home to PASS AR-ACC-005, got %v", r)
	}
	if r, ok := mAcc005[srv.URL+"/google-blocked"]; !ok || r.Status != audit.StatusFail {
		t.Errorf("expected /google-blocked to FAIL AR-ACC-005, got %v", r)
	}

	resAI001, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-AI-001")
	if err != nil {
		t.Fatalf("EvaluateRuleWithContext AR-AI-001 failed: %v", err)
	}
	mAI001 := mapByURL(resAI001)
	if r, ok := mAI001[srv.URL+"/"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected Home to PASS AR-AI-001, got %v", r)
	}
	if r, ok := mAI001[srv.URL+"/oai-blocked"]; !ok || r.Status != audit.StatusFail {
		t.Errorf("expected /oai-blocked to FAIL AR-AI-001 under allow policy, got %v", r)
	}

	resAI003, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, "AR-AI-003")
	if err != nil {
		t.Fatalf("EvaluateRuleWithContext AR-AI-003 failed: %v", err)
	}
	mAI003 := mapByURL(resAI003)
	if r, ok := mAI003[srv.URL+"/google-blocked"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected /google-blocked (disallowed for GPTBot) to PASS AR-AI-003, got %v", r)
	}
	if r, ok := mAI003[srv.URL+"/public-ai"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /public-ai (allowed for GPTBot when policy=block) to WARNING AR-AI-003, got %v", r)
	}

	// 4. Test Priority B: AR-CANON-011 & AR-LINK-001
	resCanon011, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-011")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-011 failed: %v", err)
	}
	mCanon011 := mapByURL(resCanon011)
	if r, ok := mCanon011[srv.URL+"/"]; !ok || r.Status != audit.StatusNotApplicable {
		t.Errorf("expected unpaginated Home to be NOT_APPLICABLE under AR-CANON-011, got %v", r)
	}
	if r, ok := mCanon011[srv.URL+"/blog?page=1"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected /blog?page=1 to PASS AR-CANON-011, got %v", r)
	}
	if r, ok := mCanon011[srv.URL+"/blog?page=2"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /blog?page=2 (<button rel=next>) to WARNING AR-CANON-011, got %v", r)
	}
	if r, ok := mCanon011[srv.URL+"/blog?page=3"]; !ok || r.Status != audit.StatusNotApplicable {
		t.Errorf("expected /blog?page=3 (last page, only rel=prev) to be NOT_APPLICABLE under AR-CANON-011, got %v", r)
	}

	resLink001, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-001")
	if err != nil {
		t.Fatalf("EvaluateRule AR-LINK-001 failed: %v", err)
	}
	passedNavCount := 0
	for _, r := range resLink001 {
		if r.Status == audit.StatusPass {
			passedNavCount++
		}
	}
	if passedNavCount < 4 {
		t.Errorf("expected at least 4 internal <nav> links on Home to PASS AR-LINK-001, got %d", passedNavCount)
	}

	// 5. Test End-to-End Dual Raw/Rendered HTML Comparison for AR-RENDER-001..006
	// On /public-ai:
	// - AR-RENDER-001 (title equal): PASS
	// - AR-RENDER-002 (canonical changed): WARNING
	// - AR-RENDER-003 (meta robots changed to noindex): WARNING
	// - AR-RENDER-004 (h1 equal): PASS
	// - AR-RENDER-005 (main text absent in raw, present in rendered): WARNING
	// - AR-RENDER-006 (internal links added in rendered): WARNING
	renderExpectations := map[string]audit.RuleResultStatus{
		"AR-RENDER-001": audit.StatusPass,
		"AR-RENDER-002": audit.StatusWarning,
		"AR-RENDER-003": audit.StatusWarning,
		"AR-RENDER-004": audit.StatusPass,
		"AR-RENDER-005": audit.StatusWarning,
		"AR-RENDER-006": audit.StatusWarning,
	}
	for ruleID, wantStatus := range renderExpectations {
		resR, err := eng.EvaluateRule(context.Background(), snap, ruleID)
		if err != nil {
			t.Fatalf("EvaluateRule %s failed: %v", ruleID, err)
		}
		mR := mapByURL(resR)
		if r, ok := mR[srv.URL+"/public-ai"]; !ok || r.Status != wantStatus {
			t.Errorf("expected /public-ai under %s to be %s, got %v", ruleID, wantStatus, r)
		}
	}

	// 6. Verify all 47 registered rules evaluate without error on the Batch 4 snapshot
	emptyAllowed := map[string]bool{
		"AR-ACC-007":    true, // requires SubjectResource
		"AR-CANON-001":  true, // requires SubjectSiteConfiguration
		"AR-INDEX-004":  true, // requires SubjectProbe
		"AR-DISC-001":   true, // requires SubjectSitemap
		"AR-DISC-002":   true, // requires SubjectSitemap
		"AR-DISC-003":   true, // requires SubjectSitemap
		"AR-DISC-004":   true, // requires SubjectSitemapEntry
		"AR-DISC-005":   true, // requires SubjectSitemapEntry
		"AR-DISC-006":   true, // requires SubjectSitemapEntry
		"AR-ENTITY-001": true, // requires SubjectSchemaEntity
	}
	for _, ruleID := range eng.ImplementedRuleIDs() {
		results, err := eng.EvaluateRuleWithContext(context.Background(), evalCtx, ruleID)
		if err != nil {
			t.Fatalf("EvaluateRuleWithContext %s failed: %v", ruleID, err)
		}
		if len(results) == 0 && !emptyAllowed[ruleID] {
			t.Errorf("expected non-empty results for %s on Batch 4 crawl", ruleID)
		}
	}
}

// TestEngine_Batch4_QARemediationAndHeadlessRender verifies:
// 1. Real headless browser JavaScript rendering (AR-RENDER-001..006) mutating title, canonical,
//    meta robots, H1, main text, and internal links end-to-end (SiteCrawl -> Adapter -> Engine).
// 2. Redirected URL (301) is NOT marked rendered when EnableJavaScript is true (AR-RENDER-001..006 -> NOT_APPLICABLE).
// 3. Multi-agent robots.txt specificity: when a specific User-agent group (e.g. Googlebot) exists,
//    wildcard User-agent: * Disallow rules do NOT contaminate Googlebot's matched_robots_rule or verdict,
//    and default-crawler matched_robots_rule does NOT contaminate Googlebot/OAI-SearchBot/GPTBot evidence.
// 4. Pagination edge cases (AR-CANON-011):
//    - <link rel="next"> in <head> with matching body <a href> -> PASS
//    - <link rel="next"> in <head> with NO body <a href> -> WARNING
//    - Conflicting <a rel="next"> targets on the same page -> UNKNOWN
//    - Ordinary non-pagination UI button (<button aria-label="Next slide">) -> NOT_APPLICABLE
// 5. Navigation link edge cases (AR-LINK-001):
//    - Non-crawlable <button data-href="/internal"> and <a href="javascript:navigate('/internal')"> in <nav> -> WARNING
//    - Ordinary UI <button type="button">Toggle Menu</button> in <nav> without target -> NOT emitted as WARNING
func TestEngine_Batch4_QARemediationAndHeadlessRender(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// Googlebot has its own specific group blocking ONLY /only-google-blocked.
		// Wildcard * blocks /wildcard-blocked (which Googlebot must IGNORE per RFC 9309,
		// while OAI-SearchBot inherits the wildcard block).
		// Default crawler 1ScoutSiteCrawl has Allow: / so it can crawl all test pages.
		fmt.Fprintf(w, `User-agent: 1ScoutSiteCrawl
Allow: /

User-agent: Googlebot
Disallow: /only-google-blocked

User-agent: GPTBot
Disallow: /gpt-blocked
Allow: /gpt-blocked/allowed-subpath

User-agent: *
Disallow: /wildcard-blocked
`)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head>
<title>Raw Home Title</title>
<link rel="canonical" href="%s/">
<meta name="robots" content="index, follow">
</head>
<body>
<nav>
  <a href="%s/wildcard-blocked">Wildcard Blocked</a>
  <a href="%s/head-rel-next-with-anchor">Head Rel Next With Anchor</a>
  <a href="%s/head-rel-next-only">Head Rel Next Only</a>
  <a href="%s/conflict-next">Conflict Next</a>
  <a href="%s/redirect-start">Redirect Start</a>
  <button type="button" aria-label="Toggle Menu">Menu</button>
  <button type="button" data-href="/nav-button-target">Button Nav</button>
  <a href="javascript:navigate('/nav-js-target')">JS Nav</a>
</nav>
<div id="carousel"><button type="button" aria-label="Next slide">Next Slide</button></div>
<div id="app"></div>
<script>
  document.title = "Rendered Home Title";
  var canon = document.querySelector('link[rel="canonical"]');
  if (canon) { canon.setAttribute("href", "%s/rendered-canonical"); }
  var robots = document.querySelector('meta[name="robots"]');
  if (robots) { robots.setAttribute("content", "noindex, follow"); }
  document.getElementById("app").innerHTML =
    '<h1>Rendered Dynamic H1</h1>' +
    '<p>This main body text was injected by client-side JavaScript during headless rendering.</p>' +
    '<a href="%s/js-discovered">JS Discovered Link</a>';
</script>
</body></html>`, srvURL, srvURL, srvURL, srvURL, srvURL, srvURL, srvURL, srvURL)
	})

	mux.HandleFunc("/wildcard-blocked", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Wildcard Blocked</title></head><body><h1>Wildcard</h1><p>Body text</p></body></html>`)
	})

	mux.HandleFunc("/head-rel-next-with-anchor", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head>
<title>Head Rel Next With Body Anchor</title>
<link rel="next" href="%s/wildcard-blocked">
</head><body>
<h1>Paginated With Body Link</h1>
<a href="%s/wildcard-blocked">Page 2</a>
</body></html>`, srvURL, srvURL)
	})

	mux.HandleFunc("/head-rel-next-only", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head>
<title>Head Rel Next Only</title>
<link rel="next" href="%s/unlinked-page-2">
</head><body>
<h1>Paginated Without Body Anchor</h1>
</body></html>`, srvURL)
	})

	mux.HandleFunc("/conflict-next", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Conflicting Next</title></head><body>
<h1>Conflicting Next Links</h1>
<a rel="next" href="%s/page-a">Next A</a>
<a rel="next" href="%s/page-b">Next B</a>
</body></html>`, srvURL, srvURL)
	})

	mux.HandleFunc("/redirect-start", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srvURL+"/wildcard-blocked", http.StatusMovedPermanently)
	})

	mux.HandleFunc("/js-discovered", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>JS Discovered</title></head><body><h1>JS Discovered</h1><p>Text</p></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	runner := sitecrawl.NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	opts := sitecrawl.Options{
		Mode:             sitecrawl.ModeSpider,
		MaxDepth:         3,
		MaxURLs:          30,
		Concurrency:      2,
		FollowRedirects:  true,
		RespectRobots:    true,
		EnableJavaScript: true,
		JSPatterns:       []string{`^` + srvURL + `/$`},
		JSMaxPages:       5,
		JSWaitMs:         400,
		JSTimeoutSec:     25,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	started, err := runner.Crawl(ctx, []string{srv.URL + "/"}, opts)
	if err != nil {
		t.Fatalf("Crawl failed: %v", err)
	}

	auditRunID := audit.AuditRunID("audit:run:batch4_qa_headless")
	snapID := audit.SnapshotID("snap:run:batch4_qa_headless")
	buildRes, err := adapter.Build(ctx, db, adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}
	snap := buildRes.EvidenceSnapshot

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	mapByURL := func(res []audit.RuleResult) map[string]audit.RuleResult {
		m := make(map[string]audit.RuleResult)
		for _, r := range res {
			for _, ref := range r.EvidenceRefs {
				if ref.Field == "url" {
					m[ref.ObservedValue] = r
					break
				}
			}
		}
		return m
	}

	// 1. Verify QA-A: Googlebot ignores wildcard Disallow when User-agent: Googlebot group exists,
	// and does not attach wildcard/default Disallow rule as supporting evidence.
	resAcc005, err := eng.EvaluateRule(ctx, snap, "AR-ACC-005")
	if err != nil {
		t.Fatalf("EvaluateRule AR-ACC-005: %v", err)
	}
	mAcc005 := mapByURL(resAcc005)
	if r, ok := mAcc005[srv.URL+"/wildcard-blocked"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected /wildcard-blocked to PASS AR-ACC-005 for Googlebot (specific group overrides *), got %v", r)
	} else {
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "matched_robots_rule" && strings.Contains(ref.ObservedValue, "Disallow") {
				t.Errorf("unexpected Disallow rule attached to Googlebot ALLOWED result: %+v", ref)
			}
		}
	}

	// Meanwhile OAI-SearchBot has no specific group so it inherits User-agent: * Disallow: /wildcard-blocked -> WARNING
	resAI001, err := eng.EvaluateRule(ctx, snap, "AR-AI-001")
	if err != nil {
		t.Fatalf("EvaluateRule AR-AI-001: %v", err)
	}
	mAI001 := mapByURL(resAI001)
	if r, ok := mAI001[srv.URL+"/wildcard-blocked"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /wildcard-blocked to WARNING AR-AI-001 for OAI-SearchBot (inherits *), got %v", r)
	}

	// 2. Verify QA-C: AR-CANON-011 pagination edge cases
	resCanon011, err := eng.EvaluateRule(ctx, snap, "AR-CANON-011")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-011: %v", err)
	}
	mCanon011 := mapByURL(resCanon011)
	// Home page has <button aria-label="Next slide"> which must NOT trigger pagination
	if r, ok := mCanon011[srv.URL+"/"]; !ok || r.Status != audit.StatusNotApplicable {
		t.Errorf("expected Home with carousel 'Next slide' button to be NOT_APPLICABLE under AR-CANON-011, got %v", r)
	}
	// <link rel="next"> + matching body <a href> -> PASS
	if r, ok := mCanon011[srv.URL+"/head-rel-next-with-anchor"]; !ok || r.Status != audit.StatusPass {
		t.Errorf("expected /head-rel-next-with-anchor to PASS AR-CANON-011, got %v", r)
	}
	// <link rel="next"> only in <head> with no body <a href> -> WARNING
	if r, ok := mCanon011[srv.URL+"/head-rel-next-only"]; !ok || r.Status != audit.StatusWarning {
		t.Errorf("expected /head-rel-next-only to WARNING AR-CANON-011, got %v", r)
	}
	// Conflicting <a rel="next"> targets -> UNKNOWN
	if r, ok := mCanon011[srv.URL+"/conflict-next"]; !ok || r.Status != audit.StatusUnknown {
		t.Errorf("expected /conflict-next to be UNKNOWN under AR-CANON-011, got %v", r)
	}

	// 3. Verify QA-C: AR-LINK-001 navigation candidates
	resLink001, err := eng.EvaluateRule(ctx, snap, "AR-LINK-001")
	if err != nil {
		t.Fatalf("EvaluateRule AR-LINK-001: %v", err)
	}
	warningTargets := make(map[string]bool)
	for _, r := range resLink001 {
		if r.Status == audit.StatusWarning {
			for _, ref := range r.EvidenceRefs {
				if ref.Field == "resolved_internal_target" {
					warningTargets[ref.ObservedValue] = true
				}
			}
		}
	}
	if !warningTargets[srv.URL+"/nav-button-target"] {
		t.Errorf("expected <button data-href='/nav-button-target'> in <nav> to emit AR-LINK-001 WARNING, got %v", warningTargets)
	}
	if !warningTargets[srv.URL+"/nav-js-target"] {
		t.Errorf("expected <a href=\"javascript:navigate('/nav-js-target')\"> in <nav> to emit AR-LINK-001 WARNING, got %v", warningTargets)
	}
	if len(warningTargets) != 2 {
		t.Errorf("expected exactly 2 AR-LINK-001 WARNING targets (ordinary menu button excluded), got %v", warningTargets)
	}

	// 4. Verify QA-B: Real headless browser render on Home ("/") and non-rendered 301 ("/redirect-start")
	for _, ruleID := range []string{"AR-RENDER-001", "AR-RENDER-002", "AR-RENDER-003", "AR-RENDER-004", "AR-RENDER-005", "AR-RENDER-006"} {
		resR, err := eng.EvaluateRule(ctx, snap, ruleID)
		if err != nil {
			t.Fatalf("EvaluateRule %s: %v", ruleID, err)
		}
		mR := mapByURL(resR)
		// Redirecting URL must be NOT_APPLICABLE (never marked rendered)
		if r, ok := mR[srv.URL+"/redirect-start"]; !ok || r.Status != audit.StatusNotApplicable {
			t.Errorf("expected 301 /redirect-start under %s to be NOT_APPLICABLE, got %v", ruleID, r)
		}
		// Home page underwent real headless Chrome/Edge rendering where JS mutated title, canonical,
		// meta robots, H1, main text, and internal links -> all 6 rules must emit WARNING!
		if r, ok := mR[srv.URL+"/"]; !ok || r.Status != audit.StatusWarning {
			t.Errorf("expected real headless-rendered Home under %s to be WARNING, got %v", ruleID, r)
		}
	}
}
