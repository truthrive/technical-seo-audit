package adapter_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// Helper to create an in-memory database with initialized SiteCrawl schema.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	runner := sitecrawl.NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("failed to ensure schema: %v", err)
	}
	return db
}

// 1. Completed crawl builds frozen snapshot
func TestAdapter_CompletedCrawlBuildsFrozenSnapshot(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home Page</title><meta name="description" content="Home meta"></head>
<body><nav><a href="/about">About Us</a></nav><h1>Welcome</h1></body></html>`)
	})
	mux.HandleFunc("/about", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>About Page</title><meta name="robots" content="noindex, follow"></head>
<body><h1>About</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	db := newTestDB(t)
	runner := sitecrawl.NewRunner(db)

	opts := sitecrawl.Options{
		Mode:        sitecrawl.ModeSpider,
		MaxDepth:    2,
		MaxURLs:     10,
		Concurrency: 1,
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	req := adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:run:1",
		SnapshotID: "snap:run:1",
		ProjectPolicyAssignments: []audit.ProjectPolicyAssignment{
			{
				PolicyAssignmentID: "pol:1",
				AuditRunID:         "audit:run:1",
				PolicyKey:          audit.PolicyKeyGooglebotAccessPolicy,
				PolicyValue:        audit.PolicyValueAccessAllow,
				Scope:              audit.PolicyScopeSite,
				TargetRef:          "site",
				Source:             audit.PolicyProvenanceUserInput,
				SuppliedAt:         time.Now().UTC(),
			},
		},
	}

	res, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	// Verify Snapshot boundary
	snap := res.EvidenceSnapshot
	if snap == nil {
		t.Fatalf("expected non-nil EvidenceSnapshot")
	}
	if snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Errorf("expected SnapshotStatus %q, got %q", audit.SnapshotFrozen, snap.SnapshotStatus)
	}
	if snap.FrozenAt == nil {
		t.Errorf("expected FrozenAt to be non-nil")
	}
	if snap.NormalizationVersion == "" {
		t.Errorf("expected non-empty NormalizationVersion")
	}
	if !snap.CrawlComplete {
		t.Errorf("expected CrawlComplete = true")
	}
	if snap.SitemapDiscoveryComplete || snap.RenderSelectionComplete || snap.ProbeCollectionComplete {
		t.Errorf("expected unsupported completion flags to remain false")
	}

	// Verify UrlResources & FetchObservations
	if len(res.UrlResources) < 2 {
		t.Errorf("expected at least 2 UrlResources, got %d", len(res.UrlResources))
	}
	if len(res.FetchObservations) < 2 {
		t.Errorf("expected at least 2 FetchObservations, got %d", len(res.FetchObservations))
	}
	if len(res.HtmlObservations) < 2 {
		t.Errorf("expected at least 2 HtmlObservations, got %d", len(res.HtmlObservations))
	}
	if len(snap.NormalizedObservations) == 0 {
		t.Errorf("expected non-empty NormalizedObservations")
	}
	if len(res.EvidenceGaps) == 0 {
		t.Errorf("expected documented EvidenceGaps to be present")
	}
}

// 2. Incomplete crawl rejected
func TestAdapter_IncompleteCrawlRejected(t *testing.T) {
	db := newTestDB(t)

	invalidStates := []string{
		sitecrawl.StateRunning,
		sitecrawl.StatePaused,
		sitecrawl.StateStopped,
		sitecrawl.StateFailed,
		sitecrawl.StateInterrupted,
		sitecrawl.StateDeleting,
	}

	for _, st := range invalidStates {
		runID := "run-" + st
		_, err := db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
			VALUES(?, 'http://test.local', 'test.local', '{}', ?, '2026-10-01T00:00:00Z')`, runID, st)
		if err != nil {
			t.Fatalf("insert run failed: %v", err)
		}

		req := adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:test",
			SnapshotID: "snap:test",
		}

		_, err = adapter.Build(context.Background(), db, req)
		if err == nil {
			t.Errorf("expected error for state %q, got nil", st)
		}
	}

	// Missing run
	req := adapter.BuildRequest{
		CrawlRunID: "non-existent-run",
		AuditRunID: "audit:test",
		SnapshotID: "snap:test",
	}
	_, err := adapter.Build(context.Background(), db, req)
	if err == nil {
		t.Errorf("expected error for non-existent run, got nil")
	}
}

// 3. Start URL discovery
func TestAdapter_StartURLDiscovery(t *testing.T) {
	db := newTestDB(t)
	runID := "run-start-url"
	seed := "https://example.com/"

	_, err := db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	if err != nil {
		t.Fatalf("insert run failed: %v", err)
	}

	_, err = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)
	if err != nil {
		t.Fatalf("insert url failed: %v", err)
	}

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:start",
		SnapshotID: "snap:run:start",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.DiscoveryRecords) != 1 {
		t.Fatalf("expected 1 DiscoveryRecord, got %d", len(res.DiscoveryRecords))
	}
	disc := res.DiscoveryRecords[0]
	if disc.DiscoveryType != audit.DiscoveryStartURL {
		t.Errorf("expected DiscoveryType START_URL, got %v", disc.DiscoveryType)
	}
	if disc.SourceRef != "sitecrawl_runs:"+runID {
		t.Errorf("expected SourceRef sitecrawl_runs:%s, got %s", runID, disc.SourceRef)
	}
	if disc.SourceURLID != nil {
		t.Errorf("expected SourceURLID = nil for START_URL, got %v", *disc.SourceURLID)
	}
}

// 4. Real anchor discovery
func TestAdapter_RealAnchorDiscovery(t *testing.T) {
	db := newTestDB(t)
	runID := "run-real-anchor"
	seed := "https://example.com/"
	target := "https://example.com/target"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 2, ?)`, runID, target)

	// Valid internal link edge from 1 to 2
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 2, 0, 1, 1, 'Target Anchor')`, runID) // placement=Nav(1), flags=Internal(1)

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:anchor",
		SnapshotID: "snap:run:anchor",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	var targetDisc *audit.DiscoveryRecord
	for i := range res.DiscoveryRecords {
		if res.DiscoveryRecords[i].URLID == "url:audit:run:anchor:2" {
			targetDisc = &res.DiscoveryRecords[i]
			break
		}
	}
	if targetDisc == nil {
		t.Fatalf("expected discovery record for target URL 2")
	}
	if targetDisc.DiscoveryType != audit.DiscoveryInternalLink {
		t.Errorf("expected INTERNAL_LINK, got %v", targetDisc.DiscoveryType)
	}
	if targetDisc.SourceURLID == nil || *targetDisc.SourceURLID != "url:audit:run:anchor:1" {
		t.Errorf("expected SourceURLID url:audit:run:anchor:1, got %v", targetDisc.SourceURLID)
	}
}

// 5. Canonical-only target does NOT become internal-link discovery
func TestAdapter_CanonicalOnlyTargetNotInternalLinkDiscovery(t *testing.T) {
	db := newTestDB(t)
	runID := "run-canon-only"
	seed := "https://example.com/"
	canonTarget := "https://example.com/canonical-only"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 2, ?)`, runID, canonTarget)

	// Page 2 was fetched by crawler with discovered_by = "link" because of a canonical reference,
	// but there is NO link in sitecrawl_links.
	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       canonTarget,
		Source:    sitecrawl.SourceLink,
		Internal:  true,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, discovered_by, is_internal, status, crawled_at)
		VALUES(?, 2, ?, ?, 'link', 1, 200, '2026-10-01T00:01:00Z')`, runID, canonTarget, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:canon",
		SnapshotID: "snap:run:canon",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	for _, d := range res.DiscoveryRecords {
		if d.URLID == "url:audit:run:canon:2" {
			t.Fatalf("canonical-only target incorrectly received discovery membership: %+v", d)
		}
	}

	// Verify gap was recorded
	foundGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDiscoveryProvenanceAmbiguous && g.SubjectRef == "url:audit:run:canon:2" {
			foundGap = true
			break
		}
	}
	if !foundGap {
		t.Errorf("expected GapDiscoveryProvenanceAmbiguous for non-anchor page 2")
	}
}

// 6. Redirect target does NOT gain discovery membership
func TestAdapter_RedirectTargetNotDiscoveryMembership(t *testing.T) {
	db := newTestDB(t)
	runID := "run-redirect-target"
	seed := "https://example.com/src"
	target := "https://example.com/dst"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 2, ?)`, runID, target)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       target,
		Source:    sitecrawl.SourceRedirect,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, discovered_by, is_internal, status, crawled_at)
		VALUES(?, 2, ?, ?, 'redirect', 1, 200, '2026-10-01T00:01:00Z')`, runID, target, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:redir",
		SnapshotID: "snap:run:redir",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Target 2 must NOT have discovery record
	for _, d := range res.DiscoveryRecords {
		if d.URLID == "url:audit:run:redir:2" {
			t.Fatalf("redirect destination incorrectly gained discovery record: %+v", d)
		}
	}

	// But target 2 MUST have a FetchObservation with PurposeRedirectTarget
	var fetchObs *audit.FetchObservation
	for i := range res.FetchObservations {
		if res.FetchObservations[i].URLID == "url:audit:run:redir:2" {
			fetchObs = &res.FetchObservations[i]
			break
		}
	}
	if fetchObs == nil {
		t.Fatalf("expected FetchObservation for redirect target 2")
	}
	if fetchObs.AcquisitionPurpose != audit.PurposeRedirectTarget {
		t.Errorf("expected AcquisitionPurpose REDIRECT_TARGET, got %v", fetchObs.AcquisitionPurpose)
	}
}

// 7. Resource edge does NOT become normal page discovery
func TestAdapter_ResourceEdgeNotPageDiscovery(t *testing.T) {
	db := newTestDB(t)
	runID := "run-resource-edge"
	seed := "https://example.com/"
	css := "https://example.com/style.css"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 2, ?)`, runID, css)

	// Resource link edge with FlagStylesheet
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 2, 0, 0, ?, '')`, runID, sitecrawl.FlagStylesheet|sitecrawl.FlagInternal)

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:res",
		SnapshotID: "snap:run:res",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	for _, d := range res.DiscoveryRecords {
		if d.URLID == "url:audit:run:res:2" {
			t.Fatalf("stylesheet resource edge incorrectly produced DiscoveryRecord: %+v", d)
		}
	}
	// Resource edge must not be included in LinkObservations
	for _, l := range res.LinkObservations {
		if l.TargetURLID != nil && *l.TargetURLID == "url:audit:run:res:2" {
			t.Fatalf("stylesheet resource edge incorrectly included in LinkObservations: %+v", l)
		}
	}
}

// 8. Policy separation
func TestAdapter_PolicySeparation(t *testing.T) {
	db := newTestDB(t)
	runID := "run-policy"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	validPolicy := audit.ProjectPolicyAssignment{
		PolicyAssignmentID: "pol:valid",
		AuditRunID:         "audit:run:pol",
		PolicyKey:          audit.PolicyKeySnippetPolicy,
		PolicyValue:        audit.PolicyValueSnippetAllowUnrestricted,
		Scope:              audit.PolicyScopeSite,
		TargetRef:          "site",
		Source:             audit.PolicyProvenanceUserInput,
		SuppliedAt:         time.Now().UTC(),
	}

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:pol",
		SnapshotID: "snap:run:pol",
		ProjectPolicyAssignments: []audit.ProjectPolicyAssignment{
			validPolicy,
		},
	})
	if err != nil {
		t.Fatalf("build with valid policy failed: %v", err)
	}

	// Verify policy is NOT mixed into NormalizedObservation
	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.Field == string(audit.PolicyKeySnippetPolicy) {
			t.Fatalf("project policy was illegally encoded as a NormalizedObservation: %+v", obs)
		}
	}

	// Test policy with mismatched AuditRunID
	mismatched := validPolicy
	mismatched.AuditRunID = "other-run-id"
	_, err = adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID:               runID,
		AuditRunID:               "audit:run:pol",
		SnapshotID:               "snap:run:pol",
		ProjectPolicyAssignments: []audit.ProjectPolicyAssignment{mismatched},
	})
	if err == nil {
		t.Errorf("expected error for mismatched policy AuditRunID")
	}
}

// 9. Legacy conclusions ignored
func TestAdapter_LegacyConclusionsIgnored(t *testing.T) {
	db := newTestDB(t)
	runID := "run-legacy"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:          seed,
		Indexable:    false,
		Indexability: "noindex",
		Issues:       []string{"legacy-issue-1", "legacy-issue-2"},
		CrawledAt:    "2026-10-01T00:00:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, is_internal, status, indexable, indexability, issue_count, issue_max_sev, crawled_at)
		VALUES(?, 1, ?, ?, 1, 200, 0, 'noindex', 2, 3, '2026-10-01T00:00:00Z')`, runID, seed, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:legacy",
		SnapshotID: "snap:run:legacy",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Ensure no NormalizedObservation carries legacy issue codes
	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.Field == "issues" || obs.Field == "indexability" || obs.Value == "legacy-issue-1" {
			t.Fatalf("legacy conclusion field leaked into normalized observations: %+v", obs)
		}
	}
}

// 10. Raw canonical gap & 11. Raw href gap
func TestAdapter_RawEvidenceGaps(t *testing.T) {
	db := newTestDB(t)
	runID := "run-gaps"
	seed := "https://example.com/"
	target := "https://example.com/target"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 2, ?)`, runID, target)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		Canonical:  "https://example.com/canonical",
		Canonicals: []string{"https://example.com/canonical"},
		CrawledAt:  "2026-10-01T00:00:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, canonical, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'https://example.com/canonical', '2026-10-01T00:00:00Z')`, runID, seed, string(pageJSON))

	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 2, 0, 1, 1, 'Link')`, runID)

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:gaps",
		SnapshotID: "snap:run:gaps",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// 10. Raw canonical gap
	if len(res.CanonicalObservations) != 1 {
		t.Fatalf("expected 1 CanonicalObservation, got %d", len(res.CanonicalObservations))
	}
	canon := res.CanonicalObservations[0]
	if len(canon.RawValues) != 0 {
		t.Errorf("expected empty RawValues for CanonicalObservation, got %v", canon.RawValues)
	}
	if len(canon.NormalizedValues) != 1 || canon.NormalizedValues[0] != "https://example.com/canonical" {
		t.Errorf("expected NormalizedValues with resolved target, got %v", canon.NormalizedValues)
	}

	// 11. Raw href gap
	if len(res.LinkObservations) != 1 {
		t.Fatalf("expected 1 LinkObservation, got %d", len(res.LinkObservations))
	}
	link := res.LinkObservations[0]
	if link.HREFRaw != "" || link.TargetURLRaw != "" {
		t.Errorf("expected empty HREFRaw and TargetURLRaw, got %q, %q", link.HREFRaw, link.TargetURLRaw)
	}
	if link.TargetURLResolved != target {
		t.Errorf("expected TargetURLResolved %q, got %q", target, link.TargetURLResolved)
	}
}

// 12. Render gap
func TestAdapter_RenderGap(t *testing.T) {
	db := newTestDB(t)
	runID := "run-render"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       seed,
		Rendered:  true,
		Title:     "Rendered Title",
		CrawledAt: "2026-10-01T00:00:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, rendered, title, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 1, 'Rendered Title', '2026-10-01T00:00:00Z')`, runID, seed, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:render",
		SnapshotID: "snap:run:render",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	foundRenderGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapRenderedRawSourceUnavailable && g.SubjectRef == "url:audit:run:render:1" {
			foundRenderGap = true
			break
		}
	}
	if !foundRenderGap {
		t.Errorf("expected GapRenderedRawSourceUnavailable for rendered page")
	}
}

// 13. Snapshot detached from DB
func TestAdapter_SnapshotDetachedFromDB(t *testing.T) {
	db := newTestDB(t)
	runID := "run-detach"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       seed,
		Title:     "Original Title",
		Status:    200,
		CrawledAt: "2026-10-01T00:00:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, title, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 200, 'Original Title', '2026-10-01T00:00:00Z')`, runID, seed, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:detach",
		SnapshotID: "snap:run:detach",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Mutate DB rows directly
	_, _ = db.Exec(`UPDATE sitecrawl_pages SET title = 'MUTATED TITLE', status = 500 WHERE run_id = ?`, runID)
	_, _ = db.Exec(`DELETE FROM sitecrawl_urls WHERE run_id = ?`, runID)

	// Assert original snapshot and observation models are completely unaffected
	if res.HtmlObservations[0].Title != "Original Title" {
		t.Errorf("snapshot was affected by DB mutation: title is %q", res.HtmlObservations[0].Title)
	}
	if res.FetchObservations[0].Status != 200 {
		t.Errorf("snapshot was affected by DB mutation: status is %d", res.FetchObservations[0].Status)
	}
	if len(res.UrlResources) != 1 {
		t.Errorf("snapshot was affected by DB deletion: UrlResources length is %d", len(res.UrlResources))
	}
}

// 14. Stable deterministic output
func TestAdapter_DeterministicOutput(t *testing.T) {
	db := newTestDB(t)
	runID := "run-det"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       seed,
		Title:     "Deterministic Title",
		Status:    200,
		CrawledAt: "2026-10-01T00:00:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, title, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 200, 'Deterministic Title', '2026-10-01T00:00:00Z')`, runID, seed, string(pageJSON))

	req := adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:det",
		SnapshotID: "snap:run:det",
	}

	res1, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	res2, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}

	if len(res1.UrlResources) != len(res2.UrlResources) {
		t.Errorf("UrlResources count mismatch")
	}
	if len(res1.FetchObservations) != len(res2.FetchObservations) {
		t.Errorf("FetchObservations count mismatch")
	}
	if len(res1.EvidenceSnapshot.NormalizedObservations) != len(res2.EvidenceSnapshot.NormalizedObservations) {
		t.Errorf("NormalizedObservations count mismatch")
	}

	for i := range res1.EvidenceSnapshot.NormalizedObservations {
		o1 := res1.EvidenceSnapshot.NormalizedObservations[i]
		o2 := res2.EvidenceSnapshot.NormalizedObservations[i]
		if o1.ObservationID != o2.ObservationID || o1.Field != o2.Field || o1.Value != o2.Value {
			t.Errorf("observation mismatch at %d: %+v vs %+v", i, o1, o2)
		}
	}
}
