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
