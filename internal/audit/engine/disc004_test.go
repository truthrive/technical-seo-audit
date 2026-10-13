package engine_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// sitemapEntryFixture defines input fields to synthesize normalized observations
// for evaluating AR-DISC-004.
type sitemapEntryFixture struct {
	entrySubjRef           string
	entrySeq               string
	locRaw                 string
	locNorm                string
	parentSitemapRef       string
	urlSubjectRef          string
	sitemapURL             string
	targetURLIdentity      string
	httpStatus             string
	redirectInitialObserved string
	redirectHopCount       string
	fetchErrorType         string
	omitFields             []string
	customObs              []audit.NormalizedObservation
}

// newDISC004Snapshot builds a frozen EvidenceSnapshot containing normalized
// SubjectSitemapEntry, SubjectSitemap, and SubjectURL observations for unit testing.
func newDISC004Snapshot(fixtures []sitemapEntryFixture, runID, snapID string) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation
	obsCounter := 0
	nextObsID := func() audit.ObservationID {
		obsCounter++
		return audit.ObservationID(fmt.Sprintf("obs:%s:%d", snapID, obsCounter))
	}

	for _, f := range fixtures {
		omitted := make(map[string]bool)
		for _, o := range f.omitFields {
			omitted[o] = true
		}

		// 1. SubjectSitemapEntry observations
		if !omitted["sitemap_entry_id"] && f.entrySubjRef != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         f.entrySubjRef,
				Field:              "sitemap_entry_id",
				Value:              f.entrySubjRef,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemap_entries:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["loc_raw"] && f.locRaw != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         f.entrySubjRef,
				Field:              "loc_raw",
				Value:              f.locRaw,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemap_entries:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["loc_normalized"] && f.locNorm != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         f.entrySubjRef,
				Field:              "loc_normalized",
				Value:              f.locNorm,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_sitemap_entries:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["parent_sitemap_ref"] && f.parentSitemapRef != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         f.entrySubjRef,
				Field:              "parent_sitemap_ref",
				Value:              f.parentSitemapRef,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemap_entries:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["entry_seq"] && f.entrySeq != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         f.entrySubjRef,
				Field:              "entry_seq",
				Value:              f.entrySeq,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemap_entries:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["url_subject_ref"] && f.urlSubjectRef != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         f.entrySubjRef,
				Field:              "url_subject_ref",
				Value:              f.urlSubjectRef,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemap_entries:test"},
				ObservedAt:         now,
			})
		}

		// 2. SubjectSitemap observation (parent sitemap URL)
		if !omitted["sitemap_url"] && f.parentSitemapRef != "" && f.sitemapURL != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.parentSitemapRef,
				Field:              "sitemap_url",
				Value:              f.sitemapURL,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}

		// 3. SubjectURL observations
		if f.urlSubjectRef != "" {
			if !omitted["url_identity"] && f.targetURLIdentity != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "url_identity",
					Value:              f.targetURLIdentity,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_urls:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["http_status"] && f.httpStatus != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "http_status",
					Value:              f.httpStatus,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["redirect_initial_observed"] && f.redirectInitialObserved != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "redirect_initial_observed",
					Value:              f.redirectInitialObserved,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["redirect_hop_count"] && f.redirectHopCount != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "redirect_hop_count",
					Value:              f.redirectHopCount,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["fetch_error_type"] && f.fetchErrorType != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "fetch_error_type",
					Value:              f.fetchErrorType,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		}

		// Append custom observations
		obs = append(obs, f.customObs...)
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             audit.SnapshotID(snapID),
		AuditRunID:             audit.AuditRunID(runID),
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.9.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-10 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

// assertResultIntegrity checks all structural and invariant requirements for a RuleResult.
func assertDISC004ResultIntegrity(t *testing.T, res audit.RuleResult, expectedSubjRef string, expectedStatus audit.RuleResultStatus) {
	t.Helper()

	if res.RuleID != "AR-DISC-004" {
		t.Errorf("expected RuleID AR-DISC-004, got %s", res.RuleID)
	}
	if res.ParentCheck != "DISC-002" {
		t.Errorf("expected ParentCheck DISC-002, got %s", res.ParentCheck)
	}
	if res.SubjectType != audit.SubjectSitemapEntry {
		t.Errorf("expected SubjectType SubjectSitemapEntry, got %s", res.SubjectType)
	}
	if res.SubjectRef != expectedSubjRef {
		t.Errorf("expected SubjectRef %s, got %s", expectedSubjRef, res.SubjectRef)
	}
	if res.Status != expectedStatus {
		t.Errorf("expected Status %s, got %s (observed summary: %q)", expectedStatus, res.Status, res.ObservedSummary)
	}
	if res.Severity != audit.SeverityP1 {
		t.Errorf("expected Severity P1, got %s", res.Severity)
	}
	if res.Scope != audit.ScopeURL {
		t.Errorf("expected Scope ScopeURL, got %s", res.Scope)
	}
	if res.ExpectedSummary != "Sitemap-listed URL returns HTTP 200." {
		t.Errorf("expected ExpectedSummary 'Sitemap-listed URL returns HTTP 200.', got %q", res.ExpectedSummary)
	}
	if res.ObservedSummary == "" {
		t.Errorf("ObservedSummary must not be empty")
	}

	// Verify deterministic sorting of evidence refs
	for i := 1; i < len(res.EvidenceRefs); i++ {
		if res.EvidenceRefs[i-1].RuleEvidenceRefID >= res.EvidenceRefs[i].RuleEvidenceRefID {
			t.Errorf("EvidenceRefs not strictly sorted: %s >= %s",
				res.EvidenceRefs[i-1].RuleEvidenceRefID, res.EvidenceRefs[i].RuleEvidenceRefID)
		}
	}

	// For PASS/FAIL, verify required context and primary evidence refs
	if expectedStatus == audit.StatusPass || expectedStatus == audit.StatusFail {
		var hasLocRaw, hasParentRef, hasURLSubj, hasStatusPrimary bool
		for _, ref := range res.EvidenceRefs {
			if ref.Field == "loc_raw" && ref.Role == audit.EvidenceRoleContext {
				hasLocRaw = true
			}
			if ref.Field == "parent_sitemap_ref" && ref.Role == audit.EvidenceRoleContext {
				hasParentRef = true
			}
			if ref.Field == "url_subject_ref" && ref.Role == audit.EvidenceRoleContext {
				hasURLSubj = true
			}
			if ref.Field == "http_status" && ref.Role == audit.EvidenceRolePrimary {
				hasStatusPrimary = true
			}
		}
		if !hasLocRaw || !hasParentRef || !hasURLSubj || !hasStatusPrimary {
			t.Errorf("missing required evidence refs for status %s (loc_raw: %v, parent_ref: %v, url_subj_ref: %v, http_status: %v)",
				expectedStatus, hasLocRaw, hasParentRef, hasURLSubj, hasStatusPrimary)
		}
	}
}

// ============================================================================
// Section 11: Mandatory Unit Tests (Cases 1-22)
// ============================================================================

func TestAR_DISC_004_MandatoryUnitTests(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	fixedTime := time.Date(2026, 10, 12, 12, 0, 0, 0, time.UTC)
	eng.SetClock(func() time.Time { return fixedTime })

	// Helper to create a single-entry standard fixture
	stdFixture := func(runID, entrySubj, parentRef, urlRef, status, redir string) sitemapEntryFixture {
		return sitemapEntryFixture{
			entrySubjRef:            entrySubj,
			entrySeq:                "0",
			locRaw:                  "https://example.com/target",
			locNorm:                 "https://example.com/target",
			parentSitemapRef:        parentRef,
			urlSubjectRef:           urlRef,
			sitemapURL:              "https://example.com/sitemap.xml",
			targetURLIdentity:       "https://example.com/target",
			httpStatus:              status,
			redirectInitialObserved: redir,
		}
	}

	// 1. Direct HTTP 200 -> PASS
	t.Run("1. Direct HTTP 200 -> PASS", func(t *testing.T) {
		runID := "run:test:1"
		snap := newDISC004Snapshot([]sitemapEntryFixture{
			stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "200", "false"),
		}, runID, "snap:test:1")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusPass)
	})

	// 2. Initial 301 -> final 200 -> FAIL
	t.Run("2. Initial 301 -> final 200 -> FAIL", func(t *testing.T) {
		runID := "run:test:2"
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "301", "true")
		f.redirectHopCount = "1"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:2")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusFail)
	})

	// 3. Initial 302 -> final 200 -> FAIL
	t.Run("3. Initial 302 -> final 200 -> FAIL", func(t *testing.T) {
		runID := "run:test:3"
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "302", "true")
		f.redirectHopCount = "1"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:3")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusFail)
	})

	// 4. Initial 307 -> final 200 -> FAIL
	t.Run("4. Initial 307 -> final 200 -> FAIL", func(t *testing.T) {
		runID := "run:test:4"
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "307", "true")
		f.redirectHopCount = "1"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:4")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusFail)
	})

	// 5. Initial 308 -> final 200 -> FAIL
	t.Run("5. Initial 308 -> final 200 -> FAIL", func(t *testing.T) {
		runID := "run:test:5"
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "308", "true")
		f.redirectHopCount = "1"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:5")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusFail)
	})

	// 6. Direct HTTP 404 -> FAIL
	t.Run("6. Direct HTTP 404 -> FAIL", func(t *testing.T) {
		runID := "run:test:6"
		snap := newDISC004Snapshot([]sitemapEntryFixture{
			stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "404", "false"),
		}, runID, "snap:test:6")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusFail)
	})

	// 7. Direct HTTP 410 -> FAIL
	t.Run("7. Direct HTTP 410 -> FAIL", func(t *testing.T) {
		runID := "run:test:7"
		snap := newDISC004Snapshot([]sitemapEntryFixture{
			stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "410", "false"),
		}, runID, "snap:test:7")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusFail)
	})

	// 8. Direct HTTP 500 -> FAIL
	t.Run("8. Direct HTTP 500 -> FAIL", func(t *testing.T) {
		runID := "run:test:8"
		snap := newDISC004Snapshot([]sitemapEntryFixture{
			stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "500", "false"),
		}, runID, "snap:test:8")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusFail)
	})

	// 9. Timeout before response -> UNKNOWN
	t.Run("9. Timeout before response -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:9"
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "0", "false")
		f.fetchErrorType = "timeout"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:9")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 10. DNS/TLS failure before response -> UNKNOWN
	t.Run("10. DNS/TLS failure before response -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:10"
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "0", "false")
		f.fetchErrorType = "dns_failure"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:10")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 11. URL present but not fetched -> UNKNOWN
	t.Run("11. URL present but not fetched -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:11"
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "", "")
		f.omitFields = []string{"http_status"}
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:11")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 12. URL dictionary entry without HTTP observation -> UNKNOWN
	t.Run("12. URL dictionary entry without HTTP observation -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:12"
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "", "")
		f.omitFields = []string{"http_status", "redirect_initial_observed"}
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:12")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 13. Valid entry with url_id = 0 -> UNKNOWN
	t.Run("13. Valid entry with url_id = 0 -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:13"
		f := sitemapEntryFixture{
			entrySubjRef:     "sme:" + runID + ":1:0",
			entrySeq:         "0",
			locRaw:           "https://example.com/orphan",
			locNorm:          "https://example.com/orphan",
			parentSitemapRef: "sm:" + runID + ":1",
			sitemapURL:       "https://example.com/sitemap.xml",
			omitFields:       []string{"url_subject_ref"},
		}
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:13")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 14. Missing URL SubjectRef -> UNKNOWN
	t.Run("14. Missing URL SubjectRef -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:14"
		f := sitemapEntryFixture{
			entrySubjRef:     "sme:" + runID + ":1:0",
			entrySeq:         "0",
			locRaw:           "https://example.com/missingref",
			locNorm:          "https://example.com/missingref",
			parentSitemapRef: "sm:" + runID + ":1",
			sitemapURL:       "https://example.com/sitemap.xml",
			omitFields:       []string{"url_subject_ref"},
		}
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:14")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 15. Invalid/dangling URL SubjectRef -> UNKNOWN
	t.Run("15. Invalid/dangling URL SubjectRef -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:15"
		f := sitemapEntryFixture{
			entrySubjRef:     "sme:" + runID + ":1:0",
			entrySeq:         "0",
			locRaw:           "https://example.com/dangling",
			locNorm:          "https://example.com/dangling",
			parentSitemapRef: "sm:" + runID + ":1",
			sitemapURL:       "https://example.com/sitemap.xml",
			urlSubjectRef:    "url:" + runID + ":9999", // non-existent URL subject in snapshot
		}
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:15")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 16. Conflicting HTTP observations -> UNKNOWN
	t.Run("16. Conflicting HTTP observations -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:16"
		now := time.Now().UTC()
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "200", "false")
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:      "obs:test:16:conflict",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:16",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         "url:" + runID + ":1",
				Field:              "http_status",
				Value:              "500", // conflicting with 200
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:conflict"},
				ObservedAt:         now,
			},
		}
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:16")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 17. Duplicate <loc> in one sitemap -> independent results
	t.Run("17. Duplicate <loc> in one sitemap -> independent results", func(t *testing.T) {
		runID := "run:test:17"
		f1 := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "200", "false")
		f2 := stdFixture(runID, "sme:"+runID+":1:1", "sm:"+runID+":1", "url:"+runID+":1", "200", "false")
		f2.entrySeq = "1"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f1, f2}, runID, "snap:test:17")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 distinct results for 2 sitemap entries, got %d", len(results))
		}
		if results[0].SubjectRef == results[1].SubjectRef {
			t.Errorf("subject references must be distinct: %s == %s", results[0].SubjectRef, results[1].SubjectRef)
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusPass)
		assertDISC004ResultIntegrity(t, results[1], "sme:"+runID+":1:1", audit.StatusPass)
	})

	// 18. Same <loc> in two sitemaps -> independent results
	t.Run("18. Same <loc> in two sitemaps -> independent results", func(t *testing.T) {
		runID := "run:test:18"
		f1 := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "200", "false")
		f1.sitemapURL = "https://example.com/sitemap1.xml"

		f2 := stdFixture(runID, "sme:"+runID+":2:0", "sm:"+runID+":2", "url:"+runID+":1", "200", "false")
		f2.sitemapURL = "https://example.com/sitemap2.xml"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f1, f2}, runID, "snap:test:18")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 distinct results across separate sitemaps, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusPass)
		assertDISC004ResultIntegrity(t, results[1], "sme:"+runID+":2:0", audit.StatusPass)
	})

	// 19. Sitemap document HTTP 200 without listed-URL fetch -> UNKNOWN
	t.Run("19. Sitemap document HTTP 200 without listed-URL fetch -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:19"
		now := time.Now().UTC()
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "", "")
		f.omitFields = []string{"http_status"}
		// Parent sitemap document has HTTP 200, but listed URL has no fetch observation
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:      "obs:test:19:sm:status",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:19",
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         "sm:" + runID + ":1",
				Field:              "final_http_status",
				Value:              "200",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			},
		}
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:19")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 20. Redirect destination HTTP 200 without initial listed-URL status -> UNKNOWN
	t.Run("20. Redirect destination HTTP 200 without initial listed-URL status -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:20"
		now := time.Now().UTC()
		f := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "", "")
		f.omitFields = []string{"http_status"}
		// Destination URL (url:test:20:2) exists and returned 200, but initial URL (url:test:20:1) lacks status
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:      "obs:test:20:dest:status",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:20",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         "url:" + runID + ":2",
				Field:              "http_status",
				Value:              "200",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:dest"},
				ObservedAt:         now,
			},
		}
		snap := newDISC004Snapshot([]sitemapEntryFixture{f}, runID, "snap:test:20")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC004ResultIntegrity(t, results[0], "sme:"+runID+":1:0", audit.StatusUnknown)
	})

	// 21. Multiple sitemap entries -> stable result ordering
	t.Run("21. Multiple sitemap entries -> stable result ordering", func(t *testing.T) {
		runID := "run:test:21"
		f1 := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "200", "false")
		f2 := stdFixture(runID, "sme:"+runID+":1:1", "sm:"+runID+":1", "url:"+runID+":2", "301", "true")
		f2.entrySeq = "1"
		f3 := stdFixture(runID, "sme:"+runID+":1:2", "sm:"+runID+":1", "url:"+runID+":3", "404", "false")
		f3.entrySeq = "2"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f3, f1, f2}, runID, "snap:test:21")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}

		// Results must be ordered deterministically by SubjectRef
		expectedOrder := []string{
			"sme:" + runID + ":1:0",
			"sme:" + runID + ":1:1",
			"sme:" + runID + ":1:2",
		}
		for i, exp := range expectedOrder {
			if results[i].SubjectRef != exp {
				t.Errorf("at index %d: expected subject %s, got %s", i, exp, results[i].SubjectRef)
			}
		}
	})

	// 22. Repeated evaluation -> deterministic results
	t.Run("22. Repeated evaluation -> deterministic results", func(t *testing.T) {
		runID := "run:test:22"
		f1 := stdFixture(runID, "sme:"+runID+":1:0", "sm:"+runID+":1", "url:"+runID+":1", "200", "false")
		f2 := stdFixture(runID, "sme:"+runID+":1:1", "sm:"+runID+":1", "url:"+runID+":2", "301", "true")
		f2.entrySeq = "1"
		snap := newDISC004Snapshot([]sitemapEntryFixture{f1, f2}, runID, "snap:test:22")

		res1, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("first evaluation failed: %v", err)
		}
		res2, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-004")
		if err != nil {
			t.Fatalf("second evaluation failed: %v", err)
		}

		if !reflect.DeepEqual(res1, res2) {
			t.Fatalf("repeated evaluation produced non-deterministic results")
		}
	})
}

// ============================================================================
// Section 12: Mandatory Hermetic SQLite Integration Suite
// ============================================================================

func TestAR_DISC_004_HermeticSQLiteIntegrationSuite(t *testing.T) {
	newIntegrationDB := func(t *testing.T) *sql.DB {
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

	setupRun := func(t *testing.T, db *sql.DB, runID, seed string) {
		t.Helper()
		now := time.Now().UTC().Format(time.RFC3339)
		_, err := db.Exec(`INSERT INTO sitecrawl_runs(
			id, state, stop_reason, seed_url, host, options, started_at, finished_at, found, crawled, total
		) VALUES (?, 'completed', '', ?, 'example.com', '{}', ?, ?, 1, 1, 1)`,
			runID, seed, now, now)
		if err != nil {
			t.Fatalf("setupRun failed: %v", err)
		}
	}

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	// 1. Sitemap entry correlated with directly fetched HTTP 200 URL -> PASS
	t.Run("1. Sitemap entry with directly fetched 200 URL -> PASS", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:pass"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		// URL and page records
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		// Sitemap records
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC004ResultIntegrity(t, results[0], expectedSubj, audit.StatusPass)
	})

	// 2. Sitemap entry whose listed URL redirects to final 200 URL -> FAIL
	t.Run("2. Sitemap entry with redirecting URL -> FAIL", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:redir"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/old"
		target := "https://example.com/new"
		setupRun(t, db, runID, seed)

		// Page 1: 301 redirecting to target
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: target,
			Redirects: []sitecrawl.Hop{
				{URL: seed, Status: 301, Location: target},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES (?, 1, ?, ?, 301, ?, 1, 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), target)

		// Page 2: target 200
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 2, ?)`, runID, target)
		page2, _ := json.Marshal(sitecrawl.Page{
			URL:       target,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:11Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES (?, 2, ?, ?, 200, 'html', '2026-10-01T00:00:11Z')`, runID, target, string(page2))

		// Sitemap listing the redirecting URL (url_id=1)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC004ResultIntegrity(t, results[0], expectedSubj, audit.StatusFail)
	})

	// 3. Sitemap entry whose listed URL returns 404 -> FAIL
	t.Run("3. Sitemap entry with 404 URL -> FAIL", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:404"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/missing"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    404,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES (?, 1, ?, ?, 404, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC004ResultIntegrity(t, results[0], expectedSubj, audit.StatusFail)
	})

	// 4. Sitemap entry with URL dictionary record but no successful fetch -> UNKNOWN
	t.Run("4. Sitemap entry with URL dictionary but no successful fetch -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:nofetch"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/notfetched"
		setupRun(t, db, runID, seed)

		// URL exists in sitecrawl_urls but NO page record in sitecrawl_pages
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC004ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 5. Sitemap entry with url_id = 0 -> UNKNOWN
	t.Run("5. Sitemap entry with url_id = 0 -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:urlid0"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/orphan"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 0, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC004ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 6. Multiple sitemap documents listing the same URL -> separate independent results
	t.Run("6. Multiple sitemaps listing same URL -> independent results", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:multidoc"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/shared"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 2, 2)`, runID)
		// Sitemap 1
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap1.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

		// Sitemap 2
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 2, 'https://example.com/sitemap2.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 2, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 2, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results across distinct sitemap entries, got %d", len(results))
		}

		subj1 := fmt.Sprintf("sme:%s:1:0", auditRunID)
		subj2 := fmt.Sprintf("sme:%s:2:0", auditRunID)
		assertDISC004ResultIntegrity(t, results[0], subj1, audit.StatusPass)
		assertDISC004ResultIntegrity(t, results[1], subj2, audit.StatusPass)
	})

	// 7. Sitemap document with HTTP 200 but listed URL never fetched -> UNKNOWN
	t.Run("7. Sitemap document 200 with uncrawled listed URL -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:sm200_urlunfetched"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/uncrawled"
		setupRun(t, db, runID, seed)

		// Sitemap document returned 200 and parsed
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		// Entry has url_id=0 because it was never fetched or admitted to frontier
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 0, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC004ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 8. Fetch/redirect observations retain correct request identity
	t.Run("8. Fetch/redirect observations retain correct request identity", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:identity"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page-ident"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		// Ensure evidence refs point to url:audit:run:int:disc004:identity:1
		expectedURLSubject := fmt.Sprintf("url:%s:1", auditRunID)
		var foundTargetURLIdent bool
		for _, ref := range results[0].EvidenceRefs {
			if ref.Field == "url_identity" && ref.ObservedValue == seed {
				foundTargetURLIdent = true
			}
		}
		if !foundTargetURLIdent {
			t.Errorf("expected url_identity observation for %s not found in evidence refs", expectedURLSubject)
		}
	})

	// 9. EvidenceRefs resolve to actual normalized observations
	t.Run("9. EvidenceRefs resolve to actual normalized observations", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:refs"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/check-refs"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-004")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		obsIndex := make(map[audit.ObservationID]audit.NormalizedObservation)
		for _, obs := range buildRes.EvidenceSnapshot.NormalizedObservations {
			obsIndex[obs.ObservationID] = obs
		}

		for _, ref := range results[0].EvidenceRefs {
			obs, exists := obsIndex[audit.ObservationID(ref.EvidenceRef)]
			if !exists {
				t.Fatalf("EvidenceRef %s refers to non-existent observation %s", ref.RuleEvidenceRefID, ref.EvidenceRef)
			}
			if obs.Field != ref.Field {
				t.Errorf("field mismatch: ref has %s, observation has %s", ref.Field, obs.Field)
			}
			if obs.Value != ref.ObservedValue {
				t.Errorf("value mismatch: ref has %s, observation has %s", ref.ObservedValue, obs.Value)
			}
		}
	})

	// 10. Existing fourteen evaluators remain unaffected
	t.Run("10. Existing fourteen evaluators remain unaffected", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc004:regression"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		implIDs := eng.ImplementedRuleIDs()
		if len(implIDs) != 18 {
			t.Fatalf("expected exactly 18 implemented rules, got %d", len(implIDs))
		}

		for _, ruleID := range implIDs {
			res, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, ruleID)
			if err != nil {
				t.Fatalf("evaluating rule %s failed: %v", ruleID, err)
			}
			// AR-DISC-003, AR-DISC-004, AR-DISC-005, and AR-DISC-006 evaluate with non-empty results
			if ruleID == "AR-DISC-003" || ruleID == "AR-DISC-004" || ruleID == "AR-DISC-005" || ruleID == "AR-DISC-006" {
				if len(res) == 0 {
					t.Errorf("expected non-empty results for %s", ruleID)
				}
			}
		}
	})
}
