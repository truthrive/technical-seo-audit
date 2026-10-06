package adapter_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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
	expectedID := "disc:audit:run:start:1:START_URL"
	if string(disc.DiscoveryID) != expectedID {
		t.Errorf("expected DiscoveryID %q, got %q", expectedID, disc.DiscoveryID)
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
	if fetchObs.AcquisitionPurpose == nil || *fetchObs.AcquisitionPurpose != audit.PurposeRedirectTarget {
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

// 12. Render gap & truthful rendered HTML handling
func TestAdapter_RenderGap(t *testing.T) {
	db := newTestDB(t)
	runID := "run-render"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		Rendered:   true,
		Title:      "Rendered Title",
		XRobotsTag: "noindex",
		CrawledAt:  "2026-10-01T00:00:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, rendered, title, x_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 1, 'Rendered Title', 'noindex', '2026-10-01T00:00:00Z')`, runID, seed, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:render",
		SnapshotID: "snap:run:render",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Verify render gap was emitted with SubjectRef
	foundRenderGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapRenderedRawSourceUnavailable && g.SubjectRef == "url:audit:run:render:1" {
			foundRenderGap = true
			break
		}
	}
	if !foundRenderGap {
		t.Errorf("expected GapRenderedRawSourceUnavailable for rendered page with SubjectRef")
	}

	// Verify rendered title is NOT emitted as raw HTML title
	if len(res.HtmlObservations) != 1 {
		t.Fatalf("expected 1 HtmlObservation, got %d", len(res.HtmlObservations))
	}
	if res.HtmlObservations[0].Title != "" {
		t.Errorf("expected empty raw Title for rendered page, got %q", res.HtmlObservations[0].Title)
	}

	// Verify HTTP-header X-Robots-Tag evidence remains usable
	if len(res.HtmlObservations[0].XRobotsRaw) != 1 || res.HtmlObservations[0].XRobotsRaw[0] != "noindex" {
		t.Errorf("expected XRobotsRaw to retain HTTP header evidence, got %v", res.HtmlObservations[0].XRobotsRaw)
	}

	// Verify factual rendered=true observation is present, but NO raw title observation exists
	foundRenderedFact := false
	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.Field == "rendered" && obs.Value == "true" {
			foundRenderedFact = true
		}
		if obs.Field == "title" {
			t.Errorf("rendered title was illegally emitted as normalized title observation: %+v", obs)
		}
	}
	if !foundRenderedFact {
		t.Errorf("expected normalized observation for rendered=true")
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

// 15. Regression: List mode passive URL not supplied
func TestAdapter_ListModePassiveURLNotSupplied(t *testing.T) {
	db := newTestDB(t)
	runID := "run-list-passive"
	seedA := "https://example.com/pageA"
	targetB := "https://example.com/pageB"

	optsJSON, _ := json.Marshal(sitecrawl.Options{
		Mode: sitecrawl.ModeList,
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', ?, 'completed', '2026-10-01T00:00:00Z')`, runID, seedA, string(optsJSON))
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seedA)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 2, ?)`, runID, targetB)

	// Page A was crawled and had SourceManual
	pageAJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       seedA,
		Source:    sitecrawl.SourceManual,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, discovered_by, is_internal, status, crawled_at)
		VALUES(?, 1, ?, ?, 'manual', 1, 200, '2026-10-01T00:01:00Z')`, runID, seedA, string(pageAJSON))

	// Page A links to B (so B enters URL dictionary and link table), but B was NOT supplied and NOT crawled
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 2, 0, 1, 1, 'Link to B')`, runID)

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:list",
		SnapshotID: "snap:run:list",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Page A (url_id 1) was supplied so it receives SUPPLIED_URL_LIST
	foundASupplied := false
	for _, d := range res.DiscoveryRecords {
		if d.URLID == "url:audit:run:list:1" && d.DiscoveryType == audit.DiscoverySuppliedURLList {
			foundASupplied = true
		}
		if d.URLID == "url:audit:run:list:2" && d.DiscoveryType == audit.DiscoverySuppliedURLList {
			t.Fatalf("passive unvisited dictionary target B illegally received SUPPLIED_URL_LIST: %+v", d)
		}
	}
	if !foundASupplied {
		t.Errorf("expected Page A to have SUPPLIED_URL_LIST discovery record")
	}
}

// 16. Regression: Multiple discovery provenance on one URL
func TestAdapter_MultipleDiscoveryProvenance(t *testing.T) {
	db := newTestDB(t)
	runID := "run-multi-disc"
	seed := "https://example.com/"
	page2 := "https://example.com/p2"

	optsJSON, _ := json.Marshal(sitecrawl.Options{
		Mode: sitecrawl.ModeList,
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', ?, 'completed', '2026-10-01T00:00:00Z')`, runID, seed, string(optsJSON))
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 2, ?)`, runID, page2)

	// Seed URL is both START_URL and admitted from manual list (SourceManual)
	seedPageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       seed,
		Source:    sitecrawl.SourceManual,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, discovered_by, is_internal, status, crawled_at)
		VALUES(?, 1, ?, ?, 'manual', 1, 200, '2026-10-01T00:01:00Z')`, runID, seed, string(seedPageJSON))

	// Page 2 was discovered via sitemap, AND has an incoming internal link from seed
	p2JSON, _ := json.Marshal(sitecrawl.Page{
		URL:       page2,
		Source:    sitecrawl.SourceSitemap,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:30Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, discovered_by, is_internal, status, crawled_at)
		VALUES(?, 2, ?, ?, 'sitemap', 1, 200, '2026-10-01T00:01:30Z')`, runID, page2, string(p2JSON))
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 2, 0, 1, 1, 'Link to P2')`, runID)

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:multi",
		SnapshotID: "snap:run:multi",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// URL 1 should have START_URL and SUPPLIED_URL_LIST
	var types1 []audit.DiscoveryType
	for _, d := range res.DiscoveryRecords {
		if d.URLID == "url:audit:run:multi:1" {
			types1 = append(types1, d.DiscoveryType)
			expectedID := fmt.Sprintf("disc:audit:run:multi:1:%s", d.DiscoveryType)
			if string(d.DiscoveryID) != expectedID {
				t.Errorf("expected discovery ID %q, got %q", expectedID, d.DiscoveryID)
			}
		}
	}
	if len(types1) != 2 {
		t.Fatalf("expected 2 discovery records for URL 1 (START_URL + SUPPLIED_URL_LIST), got %d: %v", len(types1), types1)
	}

	// URL 2 should have SITEMAP and INTERNAL_LINK
	var types2 []audit.DiscoveryType
	for _, d := range res.DiscoveryRecords {
		if d.URLID == "url:audit:run:multi:2" {
			types2 = append(types2, d.DiscoveryType)
			expectedID := fmt.Sprintf("disc:audit:run:multi:2:%s", d.DiscoveryType)
			if string(d.DiscoveryID) != expectedID {
				t.Errorf("expected discovery ID %q, got %q", expectedID, d.DiscoveryID)
			}
		}
	}
	if len(types2) != 2 {
		t.Fatalf("expected 2 discovery records for URL 2 (SITEMAP + INTERNAL_LINK), got %d: %v", len(types2), types2)
	}
}

// 17. Regression: Robots-blocked FetchAttempted=false
func TestAdapter_RobotsBlockedFetchAttemptedFalse(t *testing.T) {
	db := newTestDB(t)
	runID := "run-robots-blocked"
	seed := "https://example.com/blocked"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:         seed,
		RobotsState: sitecrawl.RobotsBlocked,
		Status:      0,
		CrawledAt:   "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, robots_state, crawled_at)
		VALUES(?, 1, ?, ?, 0, 'blocked', '2026-10-01T00:01:00Z')`, runID, seed, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:rob",
		SnapshotID: "snap:run:rob",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.FetchObservations) != 1 {
		t.Fatalf("expected 1 FetchObservation, got %d", len(res.FetchObservations))
	}
	fetchObs := res.FetchObservations[0]
	if fetchObs.FetchAttempted {
		t.Errorf("expected FetchAttempted=false for robots-blocked page skipped before fetch")
	}
	if fetchObs.FetchErrorType != "" {
		t.Errorf("robots-blocked should not be classified as a fetch error, got %q", fetchObs.FetchErrorType)
	}
}

// 18. Regression: Network failure FetchAttempted=true
func TestAdapter_NetworkFailureFetchAttemptedTrue(t *testing.T) {
	db := newTestDB(t)
	runID := "run-net-fail"
	seed := "https://example.com/fail"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:         seed,
		Status:      0,
		ErrorType:   "dns-not-found",
		RobotsState: sitecrawl.RobotsAllowed,
		CrawledAt:   "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, error_type, robots_state, crawled_at)
		VALUES(?, 1, ?, ?, 0, 'dns-not-found', 'allowed', '2026-10-01T00:01:00Z')`, runID, seed, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:net",
		SnapshotID: "snap:run:net",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.FetchObservations) != 1 {
		t.Fatalf("expected 1 FetchObservation, got %d", len(res.FetchObservations))
	}
	fetchObs := res.FetchObservations[0]
	if !fetchObs.FetchAttempted {
		t.Errorf("expected FetchAttempted=true for network failure")
	}
	if fetchObs.FetchErrorType != "DNS_ERROR" {
		t.Errorf("expected FetchErrorType DNS_ERROR, got %q", fetchObs.FetchErrorType)
	}
}

// 19. Regression: Ambiguous AcquisitionPurpose=nil for canonical-only fetch target
func TestAdapter_AmbiguousAcquisitionPurposeNil(t *testing.T) {
	db := newTestDB(t)
	runID := "run-ambig-purp"
	seed := "https://example.com/"
	canonTarget := "https://example.com/canonical-target"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 2, ?)`, runID, canonTarget)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       canonTarget,
		Source:    sitecrawl.SourceLink,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	// Page 2 fetched as canonical target (no anchor link in sitecrawl_links)
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, discovered_by, is_internal, status, crawled_at)
		VALUES(?, 2, ?, ?, 'link', 1, 200, '2026-10-01T00:01:00Z')`, runID, canonTarget, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:ambig",
		SnapshotID: "snap:run:ambig",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	var targetFetch *audit.FetchObservation
	for i := range res.FetchObservations {
		if res.FetchObservations[i].URLID == "url:audit:run:ambig:2" {
			targetFetch = &res.FetchObservations[i]
			break
		}
	}
	if targetFetch == nil {
		t.Fatalf("expected FetchObservation for target 2")
	}
	if targetFetch.AcquisitionPurpose != nil {
		t.Errorf("expected AcquisitionPurpose to be nil for ambiguous reference fetch, got %v", *targetFetch.AcquisitionPurpose)
	}

	foundGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapAcquisitionPurposeAmbiguous && g.SubjectRef == "url:audit:run:ambig:2" {
			foundGap = true
			break
		}
	}
	if !foundGap {
		t.Errorf("expected GapAcquisitionPurposeAmbiguous with SubjectRef for target 2")
	}
}

// 20. Regression: Deterministic multi-URL and link ordering
func TestAdapter_DeterministicMultiURLOrdering(t *testing.T) {
	db := newTestDB(t)
	runID := "run-det-multi"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)

	urls := []string{
		"https://example.com/",
		"https://example.com/b",
		"https://example.com/c",
		"https://example.com/d",
		"https://example.com/e",
	}
	for i, u := range urls {
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, ?, ?)`, runID, i+1, u)
		pageJSON, _ := json.Marshal(sitecrawl.Page{
			URL:       u,
			Title:     fmt.Sprintf("Title %d", i+1),
			Status:    200,
			CrawledAt: fmt.Sprintf("2026-10-01T00:0%d:00Z", i+1),
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, title, crawled_at)
			VALUES(?, ?, ?, ?, 'html', 200, ?, ?)`, runID, i+1, u, string(pageJSON), fmt.Sprintf("Title %d", i+1), fmt.Sprintf("2026-10-01T00:0%d:00Z", i+1))
	}

	// Add links in various orders
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 2, 0, 1, 1, 'Link 1-2')`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 3, 1, 1, 1, 'Link 1-3')`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 2, 4, 0, 1, 1, 'Link 2-4')`, runID)

	req := adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:detmulti",
		SnapshotID: "snap:run:detmulti",
	}

	res1, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("build 1 failed: %v", err)
	}
	res2, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("build 2 failed: %v", err)
	}

	// Compare UrlResources
	if len(res1.UrlResources) != len(res2.UrlResources) {
		t.Fatalf("UrlResources length mismatch: %d vs %d", len(res1.UrlResources), len(res2.UrlResources))
	}
	for i := range res1.UrlResources {
		u1 := res1.UrlResources[i]
		u2 := res2.UrlResources[i]
		if u1.URLID != u2.URLID || u1.AuditRunID != u2.AuditRunID || u1.URL != u2.URL ||
			u1.NormalizedURL != u2.NormalizedURL || u1.Scheme != u2.Scheme || u1.Host != u2.Host ||
			u1.Port != u2.Port || u1.Path != u2.Path || u1.Query != u2.Query ||
			u1.FragmentRemoved != u2.FragmentRemoved || u1.Origin != u2.Origin {
			t.Fatalf("UrlResources field mismatch at index %d: %+v vs %+v", i, u1, u2)
		}
		if (u1.IsInternal == nil) != (u2.IsInternal == nil) ||
			(u1.IsInternal != nil && *u1.IsInternal != *u2.IsInternal) {
			t.Fatalf("UrlResources IsInternal mismatch at index %d: %v vs %v", i, u1.IsInternal, u2.IsInternal)
		}
	}

	// Compare DiscoveryRecords
	if len(res1.DiscoveryRecords) != len(res2.DiscoveryRecords) {
		t.Fatalf("DiscoveryRecords length mismatch: %d vs %d", len(res1.DiscoveryRecords), len(res2.DiscoveryRecords))
	}
	for i := range res1.DiscoveryRecords {
		d1 := res1.DiscoveryRecords[i]
		d2 := res2.DiscoveryRecords[i]
		if d1.DiscoveryID != d2.DiscoveryID || d1.URLID != d2.URLID || d1.DiscoveryType != d2.DiscoveryType ||
			d1.SourceRef != d2.SourceRef || !d1.DiscoveredAt.Equal(d2.DiscoveredAt) {
			t.Fatalf("DiscoveryRecords mismatch at %d: %+v vs %+v", i, d1, d2)
		}
		if (d1.SourceURLID == nil) != (d2.SourceURLID == nil) ||
			(d1.SourceURLID != nil && *d1.SourceURLID != *d2.SourceURLID) {
			t.Fatalf("DiscoveryRecords SourceURLID mismatch at %d", i)
		}
	}

	// Compare FetchObservations
	if len(res1.FetchObservations) != len(res2.FetchObservations) {
		t.Fatalf("FetchObservations length mismatch: %d vs %d", len(res1.FetchObservations), len(res2.FetchObservations))
	}
	for i := range res1.FetchObservations {
		f1 := res1.FetchObservations[i]
		f2 := res2.FetchObservations[i]
		if f1.FetchID != f2.FetchID || f1.URLID != f2.URLID || f1.Status != f2.Status ||
			f1.RequestProfile != f2.RequestProfile || f1.FetchAttempted != f2.FetchAttempted ||
			f1.FetchErrorType != f2.FetchErrorType || !f1.ObservedAt.Equal(f2.ObservedAt) {
			t.Fatalf("FetchObservations mismatch at index %d: %+v vs %+v", i, f1, f2)
		}
		if (f1.AcquisitionPurpose == nil) != (f2.AcquisitionPurpose == nil) ||
			(f1.AcquisitionPurpose != nil && *f1.AcquisitionPurpose != *f2.AcquisitionPurpose) {
			t.Fatalf("FetchObservations AcquisitionPurpose mismatch at index %d: %v vs %v", i, f1.AcquisitionPurpose, f2.AcquisitionPurpose)
		}
	}

	// Compare LinkObservations
	if len(res1.LinkObservations) != len(res2.LinkObservations) {
		t.Fatalf("LinkObservations length mismatch: %d vs %d", len(res1.LinkObservations), len(res2.LinkObservations))
	}
	for i := range res1.LinkObservations {
		l1 := res1.LinkObservations[i]
		l2 := res2.LinkObservations[i]
		if l1.LinkID != l2.LinkID || l1.SourceURLID != l2.SourceURLID ||
			l1.TargetURLResolved != l2.TargetURLResolved || l1.AnchorText != l2.AnchorText ||
			l1.LinkLocation != l2.LinkLocation || !l1.ObservedAt.Equal(l2.ObservedAt) {
			t.Fatalf("LinkObservations mismatch at %d: %+v vs %+v", i, l1, l2)
		}
	}

	// Compare NormalizedObservations
	if len(res1.EvidenceSnapshot.NormalizedObservations) != len(res2.EvidenceSnapshot.NormalizedObservations) {
		t.Fatalf("NormalizedObservations length mismatch: %d vs %d",
			len(res1.EvidenceSnapshot.NormalizedObservations), len(res2.EvidenceSnapshot.NormalizedObservations))
	}
	for i := range res1.EvidenceSnapshot.NormalizedObservations {
		o1 := res1.EvidenceSnapshot.NormalizedObservations[i]
		o2 := res2.EvidenceSnapshot.NormalizedObservations[i]
		if o1.ObservationID != o2.ObservationID || o1.SubjectType != o2.SubjectType ||
			o1.SubjectRef != o2.SubjectRef || o1.Field != o2.Field || o1.Value != o2.Value ||
			o1.DerivationType != o2.DerivationType || !o1.ObservedAt.Equal(o2.ObservedAt) {
			t.Fatalf("NormalizedObservations mismatch at %d: %+v vs %+v", i, o1, o2)
		}
		if len(o1.SourceEvidenceRefs) != len(o2.SourceEvidenceRefs) {
			t.Fatalf("SourceEvidenceRefs length mismatch at %d: %d vs %d", i, len(o1.SourceEvidenceRefs), len(o2.SourceEvidenceRefs))
		}
		for j := range o1.SourceEvidenceRefs {
			if o1.SourceEvidenceRefs[j] != o2.SourceEvidenceRefs[j] {
				t.Fatalf("SourceEvidenceRefs mismatch at %d,%d: %q vs %q", i, j, o1.SourceEvidenceRefs[j], o2.SourceEvidenceRefs[j])
			}
		}
	}
}

// 21. Regression: Malformed source data rejected
func TestAdapter_MalformedSourceRejected(t *testing.T) {
	db := newTestDB(t)

	// 1. Malformed started_at in sitecrawl_runs
	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES('run-bad-started', 'https://example.com/', 'example.com', '{}', 'completed', 'invalid-time')`)
	_, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: "run-bad-started",
		AuditRunID: "audit:run:test",
		SnapshotID: "snap:run:test",
	})
	if !errors.Is(err, adapter.ErrMalformedRunTimestamp) {
		t.Errorf("expected ErrMalformedRunTimestamp, got %v", err)
	}

	// 2. Malformed options JSON in sitecrawl_runs
	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES('run-bad-options', 'https://example.com/', 'example.com', '{invalid-json', 'completed', '2026-10-01T00:00:00Z')`)
	_, err = adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: "run-bad-options",
		AuditRunID: "audit:run:test",
		SnapshotID: "snap:run:test",
	})
	if !errors.Is(err, adapter.ErrMalformedOptionsJSON) {
		t.Errorf("expected ErrMalformedOptionsJSON, got %v", err)
	}

	// 3. Malformed crawled_at in sitecrawl_pages
	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES('run-bad-crawled', 'https://example.com/', 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES('run-bad-crawled', 1, 'https://example.com/')`)
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
		VALUES('run-bad-crawled', 1, 'https://example.com/', '{}', 200, 'not-a-timestamp')`)
	_, err = adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: "run-bad-crawled",
		AuditRunID: "audit:run:test",
		SnapshotID: "snap:run:test",
	})
	if !errors.Is(err, adapter.ErrMalformedPageTimestamp) {
		t.Errorf("expected ErrMalformedPageTimestamp, got %v", err)
	}
}

// 22. Regression: Truthful URL normalization
func TestAdapter_URLNormalization(t *testing.T) {
	db := newTestDB(t)
	runID := "run-norm-url"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, 'https://example.com/', 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID)

	testCases := []struct {
		id              int
		rawURL          string
		expectedNormURL string
		fragRemoved     bool
		expectedHost    string
		expectedPort    int
		expectedPath    string
	}{
		{1, "HTTPS://EXAMPLE.COM/Page", "https://example.com/Page", false, "example.com", 0, "/Page"},
		{2, "https://example.com", "https://example.com/", false, "example.com", 0, "/"},
		{3, "https://example.com/section#heading", "https://example.com/section", true, "example.com", 0, "/section"},
		{4, "http://example.com:80/about", "http://example.com/about", false, "example.com", 0, "/about"},
		{5, "https://example.com:443/about", "https://example.com/about", false, "example.com", 0, "/about"},
		{6, "https://example.com:8443/custom", "https://example.com:8443/custom", false, "example.com", 8443, "/custom"},
	}

	for _, tc := range testCases {
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, ?, ?)`, runID, tc.id, tc.rawURL)
	}

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:norm",
		SnapshotID: "snap:run:norm",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.UrlResources) != len(testCases) {
		t.Fatalf("expected %d UrlResources, got %d", len(testCases), len(res.UrlResources))
	}

	for i, tc := range testCases {
		ur := res.UrlResources[i]
		if ur.NormalizedURL != tc.expectedNormURL {
			t.Errorf("case %d: expected NormalizedURL %q, got %q", tc.id, tc.expectedNormURL, ur.NormalizedURL)
		}
		if ur.FragmentRemoved != tc.fragRemoved {
			t.Errorf("case %d: expected FragmentRemoved %v, got %v", tc.id, tc.fragRemoved, ur.FragmentRemoved)
		}
		if ur.Host != tc.expectedHost {
			t.Errorf("case %d: expected Host %q, got %q", tc.id, tc.expectedHost, ur.Host)
		}
		if ur.Port != tc.expectedPort {
			t.Errorf("case %d: expected Port %d, got %d", tc.id, tc.expectedPort, ur.Port)
		}
		if ur.Path != tc.expectedPath {
			t.Errorf("case %d: expected Path %q, got %q", tc.id, tc.expectedPath, ur.Path)
		}
	}
}

// 23. Regression: Fetch error vocabulary normalization
func TestAdapter_FetchErrorNormalization(t *testing.T) {
	db := newTestDB(t)
	runID := "run-err-norm"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, 'https://example.com/', 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID)

	testCases := []struct {
		id                int
		sourceError       string
		expectedNormError string
		unmappable        bool
	}{
		{1, "dns-not-found", "DNS_ERROR", false},
		{2, "timeout", "TIMEOUT", false},
		{3, "connection-refused", "CONNECTION_ERROR", false},
		{4, "connection-error", "CONNECTION_ERROR", false},
		{5, "ssl-error", "TLS_ERROR", false},
		{6, "file-too-large", "", true},
	}

	for _, tc := range testCases {
		u := fmt.Sprintf("https://example.com/err/%d", tc.id)
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, ?, ?)`, runID, tc.id, u)
		pageJSON, _ := json.Marshal(sitecrawl.Page{
			URL:       u,
			Status:    0,
			ErrorType: tc.sourceError,
			CrawledAt: "2026-10-01T00:01:00Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, error_type, crawled_at)
			VALUES(?, ?, ?, ?, 0, ?, '2026-10-01T00:01:00Z')`, runID, tc.id, u, string(pageJSON), tc.sourceError)
	}

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:errnorm",
		SnapshotID: "snap:run:errnorm",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	for _, tc := range testCases {
		targetID := audit.URLID(fmt.Sprintf("url:audit:run:errnorm:%d", tc.id))
		var fetchObs *audit.FetchObservation
		for i := range res.FetchObservations {
			if res.FetchObservations[i].URLID == targetID {
				fetchObs = &res.FetchObservations[i]
				break
			}
		}
		if fetchObs == nil {
			t.Fatalf("missing FetchObservation for %s", targetID)
		}
		if fetchObs.FetchErrorType != tc.expectedNormError {
			t.Errorf("case %d (%s): expected FetchErrorType %q, got %q",
				tc.id, tc.sourceError, tc.expectedNormError, fetchObs.FetchErrorType)
		}
		if tc.unmappable {
			foundUnmappableGap := false
			for _, g := range res.EvidenceGaps {
				if g.GapCode == adapter.GapFetchErrorUnmappable && g.SubjectRef == string(targetID) {
					foundUnmappableGap = true
					break
				}
			}
			if !foundUnmappableGap {
				t.Errorf("case %d (%s): expected GapFetchErrorUnmappable for unmappable source error", tc.id, tc.sourceError)
			}
		}
	}
}

// 24. Regression: Request profile mapping
func TestAdapter_RequestProfileMapping(t *testing.T) {
	testProfiles := []struct {
		ua              string
		expectedProfile audit.RequestProfile
	}{
		{"googlebot", audit.ProfileGooglebot},
		{"googlebot-mobile", audit.ProfileGooglebot},
		{"bingbot", audit.ProfileCustomBot},
		{"sitecrawl", audit.ProfileDefault},
		{"chrome", audit.ProfileDefault},
		{"", audit.ProfileDefault},
	}

	for _, tc := range testProfiles {
		t.Run("profile_"+tc.ua, func(t *testing.T) {
			db := newTestDB(t)
			runID := "run-profile-" + tc.ua
			if tc.ua == "" {
				runID = "run-profile-default"
			}
			optsJSON, _ := json.Marshal(sitecrawl.Options{
				UserAgent: tc.ua,
			})
			_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
				VALUES(?, 'https://example.com/', 'example.com', ?, 'completed', '2026-10-01T00:00:00Z')`, runID, string(optsJSON))
			_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, 'https://example.com/')`, runID)
			pageJSON, _ := json.Marshal(sitecrawl.Page{
				URL:       "https://example.com/",
				Status:    200,
				CrawledAt: "2026-10-01T00:01:00Z",
			})
			_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
				VALUES(?, 1, 'https://example.com/', ?, 200, '2026-10-01T00:01:00Z')`, runID, string(pageJSON))

			res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
				CrawlRunID: runID,
				AuditRunID: "audit:run:prof",
				SnapshotID: "snap:run:prof",
			})
			if err != nil {
				t.Fatalf("build failed: %v", err)
			}

			if len(res.FetchObservations) != 1 {
				t.Fatalf("expected 1 FetchObservation, got %d", len(res.FetchObservations))
			}
			if res.FetchObservations[0].RequestProfile != tc.expectedProfile {
				t.Errorf("UA %q: expected RequestProfile %q, got %q",
					tc.ua, tc.expectedProfile, res.FetchObservations[0].RequestProfile)
			}
		})
	}
}

// 25. Regression: EvidenceGap precision
func TestAdapter_EvidenceGapPrecision(t *testing.T) {
	db := newTestDB(t)
	runID := "run-gap-prec"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, 'https://example.com/', 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, 'https://example.com/')`, runID)
	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       "https://example.com/",
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	// No canonical and no links in this crawl run
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
		VALUES(?, 1, 'https://example.com/', ?, 200, '2026-10-01T00:01:00Z')`, runID, string(pageJSON))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:prec",
		SnapshotID: "snap:run:prec",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapRawCanonicalUnavailable {
			t.Errorf("GapRawCanonicalUnavailable was emitted for run with NO canonical evidence")
		}
		if g.GapCode == adapter.GapRawHrefUnavailable {
			t.Errorf("GapRawHrefUnavailable was emitted for run with NO hyperlink evidence")
		}
	}
}

// 26. Snapshot lifecycle: BUILDING -> FROZEN and no leak on failure
func TestAdapter_SnapshotLifecycle(t *testing.T) {
	db := newTestDB(t)
	runID := "run-lifecycle"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       seed,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
		VALUES(?, 1, ?, ?, 200, '2026-10-01T00:01:00Z')`, runID, seed, string(pageJSON))

	before := time.Now().UTC().Add(-1 * time.Second)

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:life",
		SnapshotID: "snap:run:life",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	if snap == nil {
		t.Fatalf("expected non-nil snapshot")
	}
	if snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Errorf("expected SnapshotFrozen, got %s", snap.SnapshotStatus)
	}
	if snap.FrozenAt == nil {
		t.Fatalf("expected FrozenAt to be non-nil")
	}
	if snap.FrozenAt.Before(before) {
		t.Errorf("expected FrozenAt after %v, got %v", before, *snap.FrozenAt)
	}

	// On error, no partial snapshot must be returned to the caller
	badRes, badErr := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: "non-existent-run",
		AuditRunID: "audit:run:bad",
		SnapshotID: "snap:run:bad",
	})
	if badErr == nil {
		t.Errorf("expected error for non-existent run, got nil")
	}
	if badRes != nil {
		t.Errorf("expected nil BuildResult on error, got %+v", badRes)
	}
}

// 27. Single consistent SQLite read view
func TestAdapter_ConsistentReadView(t *testing.T) {
	db := newTestDB(t)
	runID := "run-readview"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       seed,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
		VALUES(?, 1, ?, ?, 200, '2026-10-01T00:01:00Z')`, runID, seed, string(pageJSON))

	// Verify successful acquisition under read-only transaction
	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:rv",
		SnapshotID: "snap:run:rv",
	})
	if err != nil {
		t.Fatalf("build with read view failed: %v", err)
	}
	if res == nil || res.EvidenceSnapshot == nil {
		t.Fatalf("expected non-nil result from consistent read view")
	}

	// Verify cancelled context aborts and rolls back read transaction cleanly
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel
	_, cancelErr := adapter.Build(ctx, db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:rv2",
		SnapshotID: "snap:run:rv2",
	})
	if cancelErr == nil {
		t.Errorf("expected error when context is pre-cancelled")
	}
}

// 28. Bot-profile fallback provenance
func TestAdapter_BotFallbackProfileProvenance(t *testing.T) {
	tests := []struct {
		name            string
		configuredUA    string
		botBlocked      bool
		expectedProfile audit.RequestProfile
		expectGap       bool
	}{
		{
			name:            "googlebot_without_fallback",
			configuredUA:    "googlebot",
			botBlocked:      false,
			expectedProfile: audit.ProfileGooglebot,
			expectGap:       false,
		},
		{
			name:            "googlebot_with_bot_blocked_fallback",
			configuredUA:    "googlebot",
			botBlocked:      true,
			expectedProfile: audit.ProfileDefault, // Chrome fallback response
			expectGap:       true,
		},
		{
			name:            "default_sitecrawl_profile",
			configuredUA:    "",
			botBlocked:      false,
			expectedProfile: audit.ProfileDefault,
			expectGap:       false,
		},
		{
			name:            "custom_bot_fallback",
			configuredUA:    "bingbot",
			botBlocked:      true,
			expectedProfile: audit.ProfileDefault, // Chrome fallback response
			expectGap:       true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			runID := "run-" + tc.name
			optsJSON, _ := json.Marshal(sitecrawl.Options{
				UserAgent: tc.configuredUA,
			})
			_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
				VALUES(?, 'https://example.com/', 'example.com', ?, 'completed', '2026-10-01T00:00:00Z')`, runID, string(optsJSON))
			_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, 'https://example.com/')`, runID)

			page := sitecrawl.Page{
				URL:        "https://example.com/",
				Status:     200,
				BotBlocked: tc.botBlocked,
				CrawledAt:  "2026-10-01T00:01:00Z",
			}
			pageJSON, _ := json.Marshal(page)
			_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
				VALUES(?, 1, 'https://example.com/', ?, 200, '2026-10-01T00:01:00Z')`, runID, string(pageJSON))

			res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
				CrawlRunID: runID,
				AuditRunID: audit.AuditRunID("audit:" + runID),
				SnapshotID: audit.SnapshotID("snap:" + runID),
			})
			if err != nil {
				t.Fatalf("build failed: %v", err)
			}

			if len(res.FetchObservations) != 1 {
				t.Fatalf("expected 1 FetchObservation, got %d", len(res.FetchObservations))
			}
			obs := res.FetchObservations[0]
			if obs.RequestProfile != tc.expectedProfile {
				t.Errorf("expected RequestProfile %q, got %q", tc.expectedProfile, obs.RequestProfile)
			}

			foundGap := false
			for _, g := range res.EvidenceGaps {
				if g.GapCode == adapter.GapBotResponseNotPreserved && g.SubjectRef == string(obs.URLID) {
					foundGap = true
					break
				}
			}
			if foundGap != tc.expectGap {
				t.Errorf("expected GapBotResponseNotPreserved = %v, got %v", tc.expectGap, foundGap)
			}
		})
	}
}

// 29. Reject malformed persisted Page JSON
func TestAdapter_RejectMalformedPageJSON(t *testing.T) {
	db := newTestDB(t)

	// 1. Invalid JSON in sitecrawl_pages.data
	runID := "run-malformed-page"
	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, 'https://example.com/', 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, 'https://example.com/')`, runID)
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
		VALUES(?, 1, 'https://example.com/', '{invalid-json-content', 200, '2026-10-01T00:01:00Z')`, runID)

	_, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:malformed",
		SnapshotID: "snap:run:malformed",
	})
	if err == nil {
		t.Fatalf("expected error for malformed Page JSON, got nil")
	}
	if !errors.Is(err, adapter.ErrMalformedPageJSON) {
		t.Errorf("expected ErrMalformedPageJSON, got %v", err)
	}

	// 2. Valid empty JSON object "{}" must be accepted
	runID2 := "run-valid-empty-json"
	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, 'https://example.com/', 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID2)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, 'https://example.com/')`, runID2)
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
		VALUES(?, 1, 'https://example.com/', '{}', 200, '2026-10-01T00:01:00Z')`, runID2)

	res2, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID2,
		AuditRunID: "audit:run:emptyjson",
		SnapshotID: "snap:run:emptyjson",
	})
	if err != nil {
		t.Fatalf("expected valid '{}' to succeed, got %v", err)
	}
	if res2 == nil {
		t.Fatalf("expected non-nil BuildResult")
	}
}

// 30. Dangling link guard: dst_id = 0 or unallocated target ID
func TestAdapter_DanglingLinkGuard(t *testing.T) {
	db := newTestDB(t)
	runID := "run-dangling-link"
	seed := "https://example.com/"

	_, _ = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

	pageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:       seed,
		Status:    200,
		CrawledAt: "2026-10-01T00:01:00Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
		VALUES(?, 1, ?, ?, 200, '2026-10-01T00:01:00Z')`, runID, seed, string(pageJSON))

	// Link 1: dst_id = 0 (URL dictionary cap hit in crawler)
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 0, 0, 1, 1, 'Capped Link')`, runID)

	// Link 2: dst_id = 999 (target ID not in sitecrawl_urls)
	_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, 1, 999, 1, 1, 1, 'Missing Link')`, runID)

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:run:dang",
		SnapshotID: "snap:run:dang",
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.LinkObservations) != 2 {
		t.Fatalf("expected 2 LinkObservations, got %d", len(res.LinkObservations))
	}

	for _, l := range res.LinkObservations {
		if l.TargetURLID != nil {
			t.Errorf("expected TargetURLID = nil for unresolved link, got %v", *l.TargetURLID)
		}
		if l.TargetURLResolved != "" {
			t.Errorf("expected TargetURLResolved to be empty, got %q", l.TargetURLResolved)
		}
	}

	// Verify no fake DiscoveryRecords were created for dst_id 0 or 999
	for _, d := range res.DiscoveryRecords {
		if d.URLID == "url:audit:run:dang:0" || d.URLID == "url:audit:run:dang:999" {
			t.Errorf("illegal discovery record created for dangling target: %+v", d)
		}
	}

	// Verify EvidenceGap was emitted for unresolved targets
	gapCount := 0
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapUnresolvedLinkTargetUnavailable && strings.HasPrefix(g.SubjectRef, "link:audit:run:dang:1:") {
			gapCount++
		}
	}
	if gapCount != 2 {
		t.Errorf("expected 2 GapUnresolvedLinkTargetUnavailable gaps, got %d", gapCount)
	}
}

// 31. Crawl completeness semantics based on state and stop_reason
func TestAdapter_CrawlCompletenessSemantics(t *testing.T) {
	testCases := []struct {
		name                string
		stopReason          string
		expectCrawlComplete bool
		expectStopReasonObs bool
	}{
		{
			name:                "clean_completion",
			stopReason:          "",
			expectCrawlComplete: true,
			expectStopReasonObs: false,
		},
		{
			name:                "max_urls_limit",
			stopReason:          sitecrawl.StopMaxURLs, // "max-urls"
			expectCrawlComplete: false,
			expectStopReasonObs: true,
		},
		{
			name:                "max_depth_limit",
			stopReason:          sitecrawl.StopMaxDepth, // "max-depth"
			expectCrawlComplete: false,
			expectStopReasonObs: true,
		},
		{
			name:                "seed_unreachable",
			stopReason:          sitecrawl.StopSeedUnreachable, // "seed-unreachable"
			expectCrawlComplete: false,
			expectStopReasonObs: true,
		},
		{
			name:                "robots_blocked",
			stopReason:          sitecrawl.StopRobotsBlocked, // "robots-blocked"
			expectCrawlComplete: false,
			expectStopReasonObs: true,
		},
		{
			name:                "seed_redirect",
			stopReason:          sitecrawl.StopSeedRedirect, // "seed-redirect"
			expectCrawlComplete: false,
			expectStopReasonObs: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db := newTestDB(t)
			runID := "run-complete-" + tc.name
			seed := "https://example.com/"

			_, err := db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, stop_reason, started_at)
				VALUES(?, ?, 'example.com', '{}', 'completed', ?, '2026-10-01T00:00:00Z')`, runID, seed, tc.stopReason)
			if err != nil {
				t.Fatalf("insert run failed: %v", err)
			}
			_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 1, ?)`, runID, seed)

			pageJSON, _ := json.Marshal(sitecrawl.Page{
				URL:       seed,
				Status:    200,
				CrawledAt: "2026-10-01T00:01:00Z",
			})
			_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
				VALUES(?, 1, ?, ?, 200, '2026-10-01T00:01:00Z')`, runID, seed, string(pageJSON))

			res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
				CrawlRunID: runID,
				AuditRunID: audit.AuditRunID("audit:" + runID),
				SnapshotID: audit.SnapshotID("snap:" + runID),
			})
			if err != nil {
				t.Fatalf("expected snapshot build to succeed, got error: %v", err)
			}

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

			// Verify canonical metadata CrawlComplete
			if snap.CrawlComplete != tc.expectCrawlComplete {
				t.Errorf("expected CrawlComplete = %v, got %v", tc.expectCrawlComplete, snap.CrawlComplete)
			}

			// Verify normalized observations for site
			var (
				foundCompleteObs   bool
				crawlCompleteValue string
				foundStopReasonObs bool
				stopReasonValue    string
			)

			for _, obs := range snap.NormalizedObservations {
				if obs.SubjectType == audit.SubjectSite && obs.SubjectRef == "site" {
					if obs.Field == "crawl_complete" {
						foundCompleteObs = true
						crawlCompleteValue = obs.Value
					}
					if obs.Field == "crawl_stop_reason" {
						foundStopReasonObs = true
						stopReasonValue = obs.Value
					}
				}
			}

			if !foundCompleteObs {
				t.Errorf("expected crawl_complete normalized observation")
			} else {
				expectedVal := fmt.Sprintf("%v", tc.expectCrawlComplete)
				if crawlCompleteValue != expectedVal {
					t.Errorf("expected crawl_complete observation %q, got %q", expectedVal, crawlCompleteValue)
				}
			}

			if foundStopReasonObs != tc.expectStopReasonObs {
				t.Errorf("expected crawl_stop_reason observation presence %v, got %v", tc.expectStopReasonObs, foundStopReasonObs)
			}
			if tc.expectStopReasonObs && stopReasonValue != tc.stopReason {
				t.Errorf("expected crawl_stop_reason observation value %q, got %q", tc.stopReason, stopReasonValue)
			}
		})
	}
}
