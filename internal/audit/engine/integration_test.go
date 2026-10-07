package engine_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

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
