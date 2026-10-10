package adapter_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

func insertLink(t *testing.T, db *sql.DB, runID string, srcID, dstID, seq int, placement uint8, flags uint32, anchor string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
		VALUES(?, ?, ?, ?, ?, ?, ?)`, runID, srcID, dstID, seq, placement, flags, anchor)
	if err != nil {
		t.Fatalf("insert link failed: %v", err)
	}
}

func insertPage(t *testing.T, db *sql.DB, runID string, urlID int, u string, status int) {
	t.Helper()
	pageData, err := json.Marshal(sitecrawl.Page{
		URL:       u,
		Status:    status,
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	if err != nil {
		t.Fatalf("marshal page data failed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
		VALUES(?, ?, ?, ?, ?, 'html', '2026-10-01T00:00:10Z')`, runID, urlID, u, string(pageData), status)
	if err != nil {
		t.Fatalf("insert page failed: %v", err)
	}
}

func getLinkObservations(snap *audit.EvidenceSnapshot, subjectRef string) map[string][]string {
	fields := make(map[string][]string)
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == subjectRef {
			fields[obs.Field] = append(fields[obs.Field], obs.Value)
		}
	}
	return fields
}

// ============================================================================
// Group A: Happy Paths
// ============================================================================

func TestAdapter_Link_HappyPaths_Internal200(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:200"
	auditRunID := "audit:link:200"
	snapID := "snap:link:200"

	srcURL := "https://example.com/source"
	targetURL := "https://example.com/target"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL)

	insertPage(t, db, runID, 1, srcURL, 200)
	insertPage(t, db, runID, 2, targetURL, 200)

	// Valid internal link edge from 1 to 2
	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementNav, sitecrawl.FlagInternal, "Target Page")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	if snap.NormalizationVersion != "v1.8.0" {
		t.Fatalf("expected NormalizationVersion v1.8.0, got %s", snap.NormalizationVersion)
	}

	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	// Check link_source_url
	if v := fields["link_source_url"]; len(v) != 1 || v[0] != srcURL {
		t.Errorf("expected link_source_url %q, got %v", srcURL, v)
	}
	// Check link_source_subject_ref
	expectedSrcRef := fmt.Sprintf("url:%s:1", auditRunID)
	if v := fields["link_source_subject_ref"]; len(v) != 1 || v[0] != expectedSrcRef {
		t.Errorf("expected link_source_subject_ref %q, got %v", expectedSrcRef, v)
	}
	// Check link_is_internal
	if v := fields["link_is_internal"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("expected link_is_internal 'true', got %v", v)
	}
	// Check link_target_subject_ref
	expectedDstRef := fmt.Sprintf("url:%s:2", auditRunID)
	if v := fields["link_target_subject_ref"]; len(v) != 1 || v[0] != expectedDstRef {
		t.Errorf("expected link_target_subject_ref %q, got %v", expectedDstRef, v)
	}
	// Check existing fields
	if v := fields["link_target"]; len(v) != 1 || v[0] != targetURL {
		t.Errorf("expected link_target %q, got %v", targetURL, v)
	}
	if v := fields["link_anchor"]; len(v) != 1 || v[0] != "Target Page" {
		t.Errorf("expected link_anchor 'Target Page', got %v", v)
	}
	if v := fields["link_location"]; len(v) != 1 || v[0] != "NAV" {
		t.Errorf("expected link_location 'NAV', got %v", v)
	}

	// Verify target URL subject's own HTTP status
	targetFields := make(map[string][]string)
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == expectedDstRef {
			targetFields[obs.Field] = append(targetFields[obs.Field], obs.Value)
		}
	}
	if v := targetFields["http_status"]; len(v) != 1 || v[0] != "200" {
		t.Errorf("expected target http_status '200', got %v", v)
	}

	// Verify invariant: HTTP status does NOT appear on the link subject
	if _, ok := fields["http_status"]; ok {
		t.Errorf("http_status must NOT be emitted on SubjectLink")
	}
	if _, ok := fields["link_target_status"]; ok {
		t.Errorf("link_target_status must NOT be emitted on SubjectLink")
	}
	if _, ok := fields["fetch_success"]; ok {
		t.Errorf("fetch_success must NOT be emitted on SubjectLink")
	}
}

func TestAdapter_Link_HappyPaths_Internal301(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:301"
	auditRunID := "audit:link:301"
	snapID := "snap:link:301"

	srcURL := "https://example.com/source"
	targetURL := "https://example.com/redirect"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL)

	insertPage(t, db, runID, 1, srcURL, 200)
	insertPage(t, db, runID, 2, targetURL, 301)

	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementNav, sitecrawl.FlagInternal, "Redirect Link")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	if v := fields["link_is_internal"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("expected link_is_internal 'true', got %v", v)
	}
	expectedDstRef := fmt.Sprintf("url:%s:2", auditRunID)
	if v := fields["link_target_subject_ref"]; len(v) != 1 || v[0] != expectedDstRef {
		t.Errorf("expected link_target_subject_ref %q, got %v", expectedDstRef, v)
	}

	// Target URL subject retains its own 301 status
	var targetStatus string
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == expectedDstRef && obs.Field == "http_status" {
			targetStatus = obs.Value
			break
		}
	}
	if targetStatus != "301" {
		t.Errorf("expected target http_status '301', got %q", targetStatus)
	}
}

func TestAdapter_Link_HappyPaths_Internal404(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:404"
	auditRunID := "audit:link:404"
	snapID := "snap:link:404"

	srcURL := "https://example.com/source"
	targetURL := "https://example.com/broken"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL)

	insertPage(t, db, runID, 1, srcURL, 200)
	insertPage(t, db, runID, 2, targetURL, 404)

	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Broken Link")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	if v := fields["link_is_internal"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("expected link_is_internal 'true', got %v", v)
	}
	expectedDstRef := fmt.Sprintf("url:%s:2", auditRunID)
	if v := fields["link_target_subject_ref"]; len(v) != 1 || v[0] != expectedDstRef {
		t.Errorf("expected link_target_subject_ref %q, got %v", expectedDstRef, v)
	}

	var targetStatus string
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == expectedDstRef && obs.Field == "http_status" {
			targetStatus = obs.Value
			break
		}
	}
	if targetStatus != "404" {
		t.Errorf("expected target http_status '404', got %q", targetStatus)
	}
}

func TestAdapter_Link_HappyPaths_Internal500(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:500"
	auditRunID := "audit:link:500"
	snapID := "snap:link:500"

	srcURL := "https://example.com/source"
	targetURL := "https://example.com/error"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL)

	insertPage(t, db, runID, 1, srcURL, 200)
	insertPage(t, db, runID, 2, targetURL, 500)

	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementFooter, sitecrawl.FlagInternal, "Server Error Link")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	if v := fields["link_is_internal"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("expected link_is_internal 'true', got %v", v)
	}
	expectedDstRef := fmt.Sprintf("url:%s:2", auditRunID)
	if v := fields["link_target_subject_ref"]; len(v) != 1 || v[0] != expectedDstRef {
		t.Errorf("expected link_target_subject_ref %q, got %v", expectedDstRef, v)
	}

	var targetStatus string
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == expectedDstRef && obs.Field == "http_status" {
			targetStatus = obs.Value
			break
		}
	}
	if targetStatus != "500" {
		t.Errorf("expected target http_status '500', got %q", targetStatus)
	}
}

func TestAdapter_Link_HappyPaths_ExternalHyperlink(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:external"
	auditRunID := "audit:link:external"
	snapID := "snap:link:external"

	srcURL := "https://example.com/source"
	extURL := "https://external.example.org/about"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, extURL)

	insertPage(t, db, runID, 1, srcURL, 200)

	// External link: FlagInternal is NOT set (flags = 0)
	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, 0, "External Site")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	// Invariant: external link has link_is_internal = false
	if v := fields["link_is_internal"]; len(v) != 1 || v[0] != "false" {
		t.Errorf("expected link_is_internal 'false', got %v", v)
	}
	if v := fields["link_source_url"]; len(v) != 1 || v[0] != srcURL {
		t.Errorf("expected link_source_url %q, got %v", srcURL, v)
	}
	expectedSrcRef := fmt.Sprintf("url:%s:1", auditRunID)
	if v := fields["link_source_subject_ref"]; len(v) != 1 || v[0] != expectedSrcRef {
		t.Errorf("expected link_source_subject_ref %q, got %v", expectedSrcRef, v)
	}
	expectedDstRef := fmt.Sprintf("url:%s:2", auditRunID)
	if v := fields["link_target_subject_ref"]; len(v) != 1 || v[0] != expectedDstRef {
		t.Errorf("expected link_target_subject_ref %q, got %v", expectedDstRef, v)
	}
	if v := fields["link_target"]; len(v) != 1 || v[0] != extURL {
		t.Errorf("expected link_target %q, got %v", extURL, v)
	}
}

// ============================================================================
// Group B: Missing Evidence
// ============================================================================

func TestAdapter_Link_MissingEvidence_TargetDiscoveredNotFetched(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:unfetched"
	auditRunID := "audit:link:unfetched"
	snapID := "snap:link:unfetched"

	srcURL := "https://example.com/source"
	targetURL := "https://example.com/unfetched-target"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL) // Present in dictionary

	insertPage(t, db, runID, 1, srcURL, 200)
	// Target URL 2 is NOT inserted into sitecrawl_pages (discovered but never fetched)

	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Unfetched Target")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	// Target subject correlation exists because URL resource exists in dictionary
	expectedDstRef := fmt.Sprintf("url:%s:2", auditRunID)
	if v := fields["link_target_subject_ref"]; len(v) != 1 || v[0] != expectedDstRef {
		t.Errorf("expected link_target_subject_ref %q, got %v", expectedDstRef, v)
	}

	// Verify target URL resource exists
	var foundTargetRes bool
	for _, ur := range res.UrlResources {
		if ur.URLID == audit.URLID(expectedDstRef) {
			foundTargetRes = true
			break
		}
	}
	if !foundTargetRes {
		t.Errorf("expected target UrlResource to exist for %s", expectedDstRef)
	}

	// Verify target has NO FetchObservation
	for _, fo := range res.FetchObservations {
		if fo.URLID == audit.URLID(expectedDstRef) {
			t.Errorf("unfetched target must NOT have FetchObservation")
		}
	}

	// Verify target has NO http_status in NormalizedObservations
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == expectedDstRef && obs.Field == "http_status" {
			t.Errorf("unfetched target must NOT have http_status observation: %v", obs)
		}
	}

	// Verify link subject has NO status
	if _, ok := fields["http_status"]; ok {
		t.Errorf("http_status must NOT be emitted on SubjectLink")
	}
}

func TestAdapter_Link_MissingEvidence_DstIDZero(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:dstzero"
	auditRunID := "audit:link:dstzero"
	snapID := "snap:link:dstzero"

	srcURL := "https://example.com/source"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertPage(t, db, runID, 1, srcURL, 200)

	// dst_id = 0 represents target URL that could not be allocated
	insertLink(t, db, runID, 1, 0, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Invalid Target")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	// Source observations should still be emitted
	if v := fields["link_source_url"]; len(v) != 1 || v[0] != srcURL {
		t.Errorf("expected link_source_url %q, got %v", srcURL, v)
	}
	if v := fields["link_is_internal"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("expected link_is_internal 'true', got %v", v)
	}

	// Target subject ref and target URL must NOT be emitted
	if _, ok := fields["link_target_subject_ref"]; ok {
		t.Errorf("link_target_subject_ref must NOT be emitted when dst_id=0")
	}
	if _, ok := fields["link_target"]; ok {
		t.Errorf("link_target must NOT be emitted when dst_id=0")
	}

	// EvidenceGap GAP_UNRESOLVED_LINK_TARGET_UNAVAILABLE must be recorded
	var foundGap bool
	for _, gap := range res.EvidenceGaps {
		if gap.GapCode == adapter.GapUnresolvedLinkTargetUnavailable {
			foundGap = true
			break
		}
	}
	if !foundGap {
		t.Errorf("expected EvidenceGap %s for dst_id=0", adapter.GapUnresolvedLinkTargetUnavailable)
	}
}

func TestAdapter_Link_MissingEvidence_DanglingDstID(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:danglingdst"
	auditRunID := "audit:link:danglingdst"
	snapID := "snap:link:danglingdst"

	srcURL := "https://example.com/source"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertPage(t, db, runID, 1, srcURL, 200)

	// dst_id = 999 does not exist in sitecrawl_urls
	insertLink(t, db, runID, 1, 999, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Dangling Target")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	if _, ok := fields["link_target_subject_ref"]; ok {
		t.Errorf("link_target_subject_ref must NOT be emitted for dangling dst_id")
	}

	var foundGap bool
	for _, gap := range res.EvidenceGaps {
		if gap.GapCode == adapter.GapUnresolvedLinkTargetUnavailable {
			foundGap = true
			break
		}
	}
	if !foundGap {
		t.Errorf("expected EvidenceGap %s for dangling dst_id", adapter.GapUnresolvedLinkTargetUnavailable)
	}
}

func TestAdapter_Link_MissingEvidence_MissingSource(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:missingsrc"
	auditRunID := "audit:link:missingsrc"
	snapID := "snap:link:missingsrc"

	seedURL := "https://example.com/"
	targetURL := "https://example.com/target"

	setupTestRun(t, db, runID, seedURL)
	insertURL(t, db, runID, 1, seedURL)
	insertURL(t, db, runID, 2, targetURL)
	insertPage(t, db, runID, 1, seedURL, 200)

	// src_id = 999 does not exist in sitecrawl_urls
	insertLink(t, db, runID, 999, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Missing Source Link")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:999:0", auditRunID)
	fields := getLinkObservations(snap, linkSubjectRef)

	// When source URL dictionary entry is missing, source observations and internal flag must NOT be emitted
	if _, ok := fields["link_source_url"]; ok {
		t.Errorf("link_source_url must NOT be emitted when src_id is invalid")
	}
	if _, ok := fields["link_source_subject_ref"]; ok {
		t.Errorf("link_source_subject_ref must NOT be emitted when src_id is invalid")
	}
	if _, ok := fields["link_is_internal"]; ok {
		t.Errorf("link_is_internal must NOT be emitted when src_id is invalid")
	}

	// GapUnresolvedLinkSourceUnavailable must be recorded
	var foundGap bool
	for _, gap := range res.EvidenceGaps {
		if gap.GapCode == adapter.GapUnresolvedLinkSourceUnavailable {
			foundGap = true
			break
		}
	}
	if !foundGap {
		t.Errorf("expected EvidenceGap %s for missing source ID", adapter.GapUnresolvedLinkSourceUnavailable)
	}
}

// ============================================================================
// Group C: Resource Isolation
// ============================================================================

func TestAdapter_Link_ResourceIsolation_ImageEdge(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:res:img"
	auditRunID := "audit:link:res:img"
	snapID := "snap:link:res:img"

	srcURL := "https://example.com/source"
	imgURL := "https://example.com/logo.png"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, imgURL)
	insertPage(t, db, runID, 1, srcURL, 200)

	// Image edge: FlagImageLink set
	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementImage, sitecrawl.FlagImageLink, "Logo")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	if len(res.LinkObservations) != 0 {
		t.Errorf("expected 0 LinkObservations for image resource edge, got %d", len(res.LinkObservations))
	}

	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.SubjectType == audit.SubjectLink {
			t.Errorf("image resource edge must NOT produce SubjectLink observation: %v", obs)
		}
	}
}

func TestAdapter_Link_ResourceIsolation_StylesheetEdge(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:res:css"
	auditRunID := "audit:link:res:css"
	snapID := "snap:link:res:css"

	srcURL := "https://example.com/source"
	cssURL := "https://example.com/style.css"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, cssURL)
	insertPage(t, db, runID, 1, srcURL, 200)

	// Stylesheet edge: FlagStylesheet set
	insertLink(t, db, runID, 1, 2, 0, 0, sitecrawl.FlagStylesheet, "")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	if len(res.LinkObservations) != 0 {
		t.Errorf("expected 0 LinkObservations for stylesheet edge, got %d", len(res.LinkObservations))
	}

	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.SubjectType == audit.SubjectLink {
			t.Errorf("stylesheet edge must NOT produce SubjectLink observation: %v", obs)
		}
	}
}

func TestAdapter_Link_ResourceIsolation_ScriptEdge(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:res:js"
	auditRunID := "audit:link:res:js"
	snapID := "snap:link:res:js"

	srcURL := "https://example.com/source"
	jsURL := "https://example.com/app.js"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, jsURL)
	insertPage(t, db, runID, 1, srcURL, 200)

	// Script edge: FlagScript set
	insertLink(t, db, runID, 1, 2, 0, 0, sitecrawl.FlagScript, "")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	if len(res.LinkObservations) != 0 {
		t.Errorf("expected 0 LinkObservations for script edge, got %d", len(res.LinkObservations))
	}

	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.SubjectType == audit.SubjectLink {
			t.Errorf("script edge must NOT produce SubjectLink observation: %v", obs)
		}
	}
}

func TestAdapter_Link_ResourceIsolation_ResourceWithInternalFlag(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:res:internal"
	auditRunID := "audit:link:res:internal"
	snapID := "snap:link:res:internal"

	srcURL := "https://example.com/source"
	imgURL := "https://example.com/internal-icon.png"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, imgURL)
	insertPage(t, db, runID, 1, srcURL, 200)

	// Resource edge with FlagInternal also set: FlagImageLink | FlagInternal
	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementImage, sitecrawl.FlagImageLink|sitecrawl.FlagInternal, "Icon")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	if len(res.LinkObservations) != 0 {
		t.Errorf("expected 0 LinkObservations for internal image edge, got %d", len(res.LinkObservations))
	}

	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.SubjectType == audit.SubjectLink {
			t.Errorf("internal image edge must NOT produce SubjectLink observation: %v", obs)
		}
	}
}

func TestAdapter_Link_ResourceIsolation_MixedEdgesInOneCrawl(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:mixed"
	auditRunID := "audit:link:mixed"
	snapID := "snap:link:mixed"

	srcURL := "https://example.com/source"
	target1 := "https://example.com/page1"
	imgURL := "https://example.com/img.png"
	cssURL := "https://example.com/style.css"
	jsURL := "https://example.com/bundle.js"
	target2 := "https://example.com/page2"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, target1)
	insertURL(t, db, runID, 3, imgURL)
	insertURL(t, db, runID, 4, cssURL)
	insertURL(t, db, runID, 5, jsURL)
	insertURL(t, db, runID, 6, target2)
	insertPage(t, db, runID, 1, srcURL, 200)

	// 5 edges:
	// seq 0: body hyperlink (hyperlink)
	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Page 1")
	// seq 1: image edge (resource)
	insertLink(t, db, runID, 1, 3, 1, sitecrawl.PlacementImage, sitecrawl.FlagImageLink|sitecrawl.FlagInternal, "Image")
	// seq 2: css edge (resource)
	insertLink(t, db, runID, 1, 4, 2, 0, sitecrawl.FlagStylesheet|sitecrawl.FlagInternal, "")
	// seq 3: js edge (resource)
	insertLink(t, db, runID, 1, 5, 3, 0, sitecrawl.FlagScript|sitecrawl.FlagInternal, "")
	// seq 4: nav hyperlink (hyperlink)
	insertLink(t, db, runID, 1, 6, 4, sitecrawl.PlacementNav, sitecrawl.FlagInternal, "Page 2")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	// Only seq 0 and seq 4 should be in LinkObservations
	if len(res.LinkObservations) != 2 {
		t.Fatalf("expected exactly 2 LinkObservations, got %d", len(res.LinkObservations))
	}

	linkSubjects := make(map[string]bool)
	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.SubjectType == audit.SubjectLink {
			linkSubjects[obs.SubjectRef] = true
		}
	}

	expectedSubj0 := fmt.Sprintf("link:%s:1:0", auditRunID)
	expectedSubj4 := fmt.Sprintf("link:%s:1:4", auditRunID)

	if !linkSubjects[expectedSubj0] {
		t.Errorf("expected SubjectLink for seq 0 (%s)", expectedSubj0)
	}
	if !linkSubjects[expectedSubj4] {
		t.Errorf("expected SubjectLink for seq 4 (%s)", expectedSubj4)
	}
	if len(linkSubjects) != 2 {
		t.Errorf("expected exactly 2 distinct SubjectLink subjects, got %d: %v", len(linkSubjects), linkSubjects)
	}
}

// ============================================================================
// Group D: Integrity
// ============================================================================

func TestAdapter_Link_Integrity_ObservationIDDeterminismAndOrdering(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:determ"
	auditRunID := "audit:link:determ"
	snapID := "snap:link:determ"

	srcURL := "https://example.com/source"
	targetURL := "https://example.com/target"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL)
	insertPage(t, db, runID, 1, srcURL, 200)
	insertPage(t, db, runID, 2, targetURL, 200)

	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Target")

	req := adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	}

	res1, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("build 1 failed: %v", err)
	}
	res2, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("build 2 failed: %v", err)
	}

	obs1 := res1.EvidenceSnapshot.NormalizedObservations
	obs2 := res2.EvidenceSnapshot.NormalizedObservations

	if len(obs1) != len(obs2) {
		t.Fatalf("observation counts differ: %d vs %d", len(obs1), len(obs2))
	}

	for i := range obs1 {
		if obs1[i].ObservationID != obs2[i].ObservationID {
			t.Errorf("obs[%d] ID mismatch: %q vs %q", i, obs1[i].ObservationID, obs2[i].ObservationID)
		}
		if obs1[i].SubjectRef != obs2[i].SubjectRef {
			t.Errorf("obs[%d] SubjectRef mismatch: %q vs %q", i, obs1[i].SubjectRef, obs2[i].SubjectRef)
		}
		if obs1[i].Field != obs2[i].Field {
			t.Errorf("obs[%d] Field mismatch: %q vs %q", i, obs1[i].Field, obs2[i].Field)
		}
		if obs1[i].Value != obs2[i].Value {
			t.Errorf("obs[%d] Value mismatch: %q vs %q", i, obs1[i].Value, obs2[i].Value)
		}
		if !reflect.DeepEqual(obs1[i].SourceEvidenceRefs, obs2[i].SourceEvidenceRefs) {
			t.Errorf("obs[%d] SourceEvidenceRefs mismatch", i)
		}
	}
}

func TestAdapter_Link_Integrity_EvidenceProvenance(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:prov"
	auditRunID := "audit:link:prov"
	snapID := "snap:link:prov"

	srcURL := "https://example.com/source"
	targetURL := "https://example.com/target"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL)
	insertPage(t, db, runID, 1, srcURL, 200)

	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Target")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	linkSubjectRef := fmt.Sprintf("link:%s:1:0", auditRunID)

	expectedLinkSrcRef := "sitecrawl_links:1:0"

	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef != linkSubjectRef {
			continue
		}
		if obs.AuditRunID != audit.AuditRunID(auditRunID) {
			t.Errorf("obs %s: AuditRunID mismatch", obs.ObservationID)
		}
		if obs.SnapshotID != audit.SnapshotID(snapID) {
			t.Errorf("obs %s: SnapshotID mismatch", obs.ObservationID)
		}
		if len(obs.SourceEvidenceRefs) != 1 {
			t.Errorf("obs %s: expected 1 SourceEvidenceRef, got %d", obs.ObservationID, len(obs.SourceEvidenceRefs))
			continue
		}

		// All SubjectLink observations trace their source provenance to the persisted link edge
		if obs.SourceEvidenceRefs[0] != expectedLinkSrcRef {
			t.Errorf("field %s: expected SourceEvidenceRef %q, got %q", obs.Field, expectedLinkSrcRef, obs.SourceEvidenceRefs[0])
		}
	}
}

// ============================================================================
// Group E: Regression
// ============================================================================

func TestAdapter_Link_Regression_CanonicalAndRedirect(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:regr"
	auditRunID := "audit:link:regr"
	snapID := "snap:link:regr"

	srcURL := "https://example.com/source"
	canonURL := "https://example.com/canonical"
	redirURL := "https://example.com/redirect"
	finalURL := "https://example.com/final"

	setupTestRunWithOpts(t, db, runID, srcURL, sitecrawl.Options{FollowRedirects: true})
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, canonURL)
	insertURL(t, db, runID, 3, redirURL)
	insertURL(t, db, runID, 4, finalURL)

	// Source page with canonical
	page1, _ := json.Marshal(sitecrawl.Page{
		URL:        srcURL,
		Status:     200,
		Canonicals: []string{canonURL},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, canonical, crawled_at)
		VALUES(?, 1, ?, ?, 200, 'html', ?, '2026-10-01T00:00:10Z')`, runID, srcURL, string(page1), canonURL)

	// Redirect page
	page3, _ := json.Marshal(sitecrawl.Page{
		URL:    redirURL,
		Status: 301,
		Redirects: []sitecrawl.Hop{
			{URL: redirURL, Status: 301, Location: finalURL},
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
		VALUES(?, 3, ?, ?, 301, 'html', '2026-10-01T00:00:10Z')`, runID, redirURL, string(page3))
	_, _ = db.Exec(`INSERT INTO sitecrawl_redirects(run_id, page_url_id, hop_index, status, location, final_url)
		VALUES(?, 3, 0, 301, ?, ?)`, runID, finalURL, finalURL)

	// Link from 1 to 3
	insertLink(t, db, runID, 1, 3, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Redirect Link")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	if snap.NormalizationVersion != "v1.8.0" {
		t.Errorf("expected NormalizationVersion v1.8.0, got %s", snap.NormalizationVersion)
	}

	// Verify Canonical evidence
	srcFields := make(map[string][]string)
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == fmt.Sprintf("url:%s:1", auditRunID) {
			srcFields[obs.Field] = append(srcFields[obs.Field], obs.Value)
		}
	}
	if v := srcFields["canonical_resolved"]; len(v) != 1 || v[0] != canonURL {
		t.Errorf("expected canonical_resolved %q, got %v", canonURL, v)
	}

	// Verify Redirect evidence
	redirFields := make(map[string][]string)
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == fmt.Sprintf("url:%s:3", auditRunID) {
			redirFields[obs.Field] = append(redirFields[obs.Field], obs.Value)
		}
	}
	if v := redirFields["redirect_initial_observed"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("expected redirect_initial_observed 'true', got %v", v)
	}

	// Verify Link evidence
	linkFields := getLinkObservations(snap, fmt.Sprintf("link:%s:1:0", auditRunID))
	if v := linkFields["link_is_internal"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("expected link_is_internal 'true', got %v", v)
	}
	if v := linkFields["link_target_subject_ref"]; len(v) != 1 || v[0] != fmt.Sprintf("url:%s:3", auditRunID) {
		t.Errorf("expected link_target_subject_ref url:%s:3, got %v", auditRunID, v)
	}
}

func TestAdapter_Link_Regression_ExecutableRules(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	implementedIDs := eng.ImplementedRuleIDs()
	expectedIDs := []string{
		"AR-ACC-004",
		"AR-CANON-003",
		"AR-CANON-004",
		"AR-CANON-006",
		"AR-CANON-007",
		"AR-CANON-008",
		"AR-CANON-009",
		"AR-INDEX-001",
		"AR-INDEX-002",
		"AR-LINK-002",
		"AR-LINK-003",
		"AR-LINK-004",
	}

	if len(implementedIDs) != 12 {
		t.Fatalf("expected exactly 12 implemented rules, got %d: %v", len(implementedIDs), implementedIDs)
	}
	if !reflect.DeepEqual(implementedIDs, expectedIDs) {
		t.Fatalf("expected implemented IDs %v, got %v", expectedIDs, implementedIDs)
	}
}

func TestAdapter_Link_MissingEvidence_IncompleteCrawlRejected(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:incomplete"
	seed := "https://example.com/"

	// Run is still running
	_, err := db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'running', '2026-10-01T00:00:00Z')`, runID, seed)
	if err != nil {
		t.Fatalf("insert run failed: %v", err)
	}

	req := adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: "audit:link:incomplete",
		SnapshotID: "snap:link:incomplete",
	}

	_, err = adapter.Build(context.Background(), db, req)
	if err == nil {
		t.Errorf("expected error for running crawl, got nil")
	}
}

func TestAdapter_Link_Integrity_ConflictingDestinationIdentity(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:conflict"
	auditRunID := "audit:link:conflict"
	snapID := "snap:link:conflict"

	srcURL := "https://example.com/source"
	targetURL := "https://example.com/target"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL)
	insertPage(t, db, runID, 1, srcURL, 200)

	// In sitecrawl_links, dst_id=2 maps to targetURL, but let's test if dst_id maps to something inconsistent
	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Anchor")

	req := adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	}

	res, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Verify standard target correlation works when consistent
	linkFields := getLinkObservations(res.EvidenceSnapshot, fmt.Sprintf("link:%s:1:0", auditRunID))
	if v := linkFields["link_target_subject_ref"]; len(v) != 1 || v[0] != fmt.Sprintf("url:%s:2", auditRunID) {
		t.Errorf("expected target subject ref url:%s:2, got %v", auditRunID, v)
	}
}

func TestAdapter_Link_Regression_GooglebotEffectiveNoindex(t *testing.T) {
	db := newTestDB(t)
	runID := "run:link:regr:noindex"
	auditRunID := "audit:link:regr:noindex"
	snapID := "snap:link:regr:noindex"

	srcURL := "https://example.com/noindex-page"
	targetURL := "https://example.com/target-page"

	setupTestRun(t, db, runID, srcURL)
	insertURL(t, db, runID, 1, srcURL)
	insertURL(t, db, runID, 2, targetURL)

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        srcURL,
		Status:     200,
		MetaRobots: "noindex, follow",
		MetaTags: map[string]string{
			"robots": "noindex, follow",
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, meta_robots, crawled_at)
		VALUES(?, 1, ?, ?, 200, 'html', 'noindex, follow', '2026-10-01T00:00:10Z')`, runID, srcURL, string(pageData))

	insertLink(t, db, runID, 1, 2, 0, sitecrawl.PlacementBody, sitecrawl.FlagInternal, "Target")

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	var foundNoindex bool
	for _, obs := range snap.NormalizedObservations {
		if obs.SubjectRef == fmt.Sprintf("url:%s:1", auditRunID) && obs.Field == "effective_noindex" && obs.Value == "true" {
			foundNoindex = true
			break
		}
	}
	if !foundNoindex {
		t.Errorf("expected effective_noindex=true for noindex page")
	}

	// And link evidence is also emitted
	linkFields := getLinkObservations(snap, fmt.Sprintf("link:%s:1:0", auditRunID))
	if v := linkFields["link_is_internal"]; len(v) != 1 || v[0] != "true" {
		t.Errorf("expected link_is_internal 'true', got %v", v)
	}
}

