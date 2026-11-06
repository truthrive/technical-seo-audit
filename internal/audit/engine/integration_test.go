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
