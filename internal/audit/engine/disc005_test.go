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

// disc005Fixture defines parameters for creating test sitemap entry snapshots
// for evaluating AR-DISC-005.
type disc005Fixture struct {
	entrySubjRef            string
	locRaw                  string
	locNorm                 string
	parentSitemapRef        string
	entrySeq                string
	urlSubjectRef           string
	sitemapURL              string
	targetURLIdentity       string
	httpStatus              string
	redirectInitialObserved string
	contentType             string
	effectiveNoindex        string
	fetchError              string
	metaRobots              string
	xRobots                 string
	omitFields              []string
	customObs               []audit.NormalizedObservation
}

// newDISC005Snapshot builds a frozen EvidenceSnapshot containing normalized
// SubjectSitemapEntry, SubjectSitemap, and SubjectURL observations for unit testing.
func newDISC005Snapshot(fixtures []disc005Fixture, runID, snapID string) *audit.EvidenceSnapshot {
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
			if !omitted["content_type"] && f.contentType != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "content_type",
					Value:              f.contentType,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["effective_noindex"] && f.effectiveNoindex != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "effective_noindex",
					Value:              f.effectiveNoindex,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["fetch_error"] && f.fetchError != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "fetch_error",
					Value:              f.fetchError,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["meta_robots"] && f.metaRobots != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "meta_robots",
					Value:              f.metaRobots,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["x_robots"] && f.xRobots != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "x_robots",
					Value:              f.xRobots,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		}

		// Append any custom observations for this fixture
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

func assertDISC005ResultIntegrity(t *testing.T, res audit.RuleResult, expectedSubjectRef string, expectedStatus audit.RuleResultStatus) {
	t.Helper()
	if res.RuleID != "AR-DISC-005" {
		t.Errorf("expected RuleID AR-DISC-005, got %s", res.RuleID)
	}
	if res.ParentCheck != "DISC-002" {
		t.Errorf("expected ParentCheck DISC-002, got %s", res.ParentCheck)
	}
	if res.SubjectType != audit.SubjectSitemapEntry {
		t.Errorf("expected SubjectType audit.SubjectSitemapEntry, got %s", res.SubjectType)
	}
	if res.SubjectRef != expectedSubjectRef {
		t.Errorf("expected SubjectRef %s, got %s", expectedSubjectRef, res.SubjectRef)
	}
	if res.Status != expectedStatus {
		t.Errorf("expected Status %s, got %s (summary: %q)", expectedStatus, res.Status, res.ObservedSummary)
	}
	if res.Severity != audit.SeverityP1 {
		t.Errorf("expected Severity P1, got %s", res.Severity)
	}
	if res.Scope != audit.ScopeURL {
		t.Errorf("expected Scope audit.ScopeURL, got %s", res.Scope)
	}
	if res.ExpectedSummary != "Sitemap-listed URL does not contain effective noindex directive." {
		t.Errorf("expected ExpectedSummary 'Sitemap-listed URL does not contain effective noindex directive.', got %q", res.ExpectedSummary)
	}
	if len(res.EvidenceRefs) == 0 {
		t.Errorf("expected non-empty EvidenceRefs")
	}

	seen := make(map[audit.RuleEvidenceRefID]bool)
	for _, ref := range res.EvidenceRefs {
		if seen[ref.RuleEvidenceRefID] {
			t.Errorf("duplicate EvidenceRef: %s", ref.RuleEvidenceRefID)
		}
		seen[ref.RuleEvidenceRefID] = true
	}

	// For PASS and FAIL, verify that effective_noindex is primary evidence
	if res.Status == audit.StatusPass || res.Status == audit.StatusFail {
		var hasEffectivePrimary bool
		for _, ref := range res.EvidenceRefs {
			if ref.Field == "effective_noindex" && ref.Role == audit.EvidenceRolePrimary {
				hasEffectivePrimary = true
			}
		}
		if !hasEffectivePrimary {
			t.Errorf("expected effective_noindex as primary evidence for %s result", res.Status)
		}
	}
}

func defaultDISC005Fixture(runID string) disc005Fixture {
	return disc005Fixture{
		entrySubjRef:      fmt.Sprintf("sme:%s:1:0", runID),
		locRaw:            "https://example.com/page",
		locNorm:           "https://example.com/page",
		parentSitemapRef:  fmt.Sprintf("sm:%s:1", runID),
		entrySeq:          "0",
		urlSubjectRef:     fmt.Sprintf("url:%s:1", runID),
		sitemapURL:        "https://example.com/sitemap.xml",
		targetURLIdentity: "https://example.com/page",
		httpStatus:        "200",
		contentType:       "text/html; charset=utf-8",
		effectiveNoindex:  "false",
	}
}

// ============================================================================
// Section 11: Mandatory Unit Tests (30 Cases)
// ============================================================================

func TestAR_DISC_005_MandatoryUnitSuite(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	// 1. Direct 200 HTML + effective_noindex=false -> PASS
	t.Run("1. Direct 200 HTML + effective_noindex=false -> PASS", func(t *testing.T) {
		runID := "run:test:1"
		f := defaultDISC005Fixture(runID)
		f.effectiveNoindex = "false"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:1")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusPass)
	})

	// 2. Direct 200 HTML + effective_noindex=true -> FAIL
	t.Run("2. Direct 200 HTML + effective_noindex=true -> FAIL", func(t *testing.T) {
		runID := "run:test:2"
		f := defaultDISC005Fixture(runID)
		f.effectiveNoindex = "true"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:2")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 3. Direct 200 HTML + missing effective_noindex -> UNKNOWN
	t.Run("3. Direct 200 HTML + missing effective_noindex -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:3"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"effective_noindex"}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:3")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 4. Direct 200 HTML + conflicting effective_noindex -> UNKNOWN
	t.Run("4. Direct 200 HTML + conflicting effective_noindex -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:4"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"effective_noindex"}
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:  "obs:conf:1",
				AuditRunID:     audit.AuditRunID(runID),
				SnapshotID:     "snap:test:4",
				SubjectType:    audit.SubjectURL,
				SubjectRef:     f.urlSubjectRef,
				Field:          "effective_noindex",
				Value:          "false",
				DerivationType: audit.DerivationNormalized,
			},
			{
				ObservationID:  "obs:conf:2",
				AuditRunID:     audit.AuditRunID(runID),
				SnapshotID:     "snap:test:4",
				SubjectType:    audit.SubjectURL,
				SubjectRef:     f.urlSubjectRef,
				Field:          "effective_noindex",
				Value:          "true",
				DerivationType: audit.DerivationNormalized,
			},
		}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:4")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 5. Direct 200 HTML + directives extraction unavailable -> UNKNOWN
	t.Run("5. Direct 200 HTML + directives extraction unavailable -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:5"
		f := defaultDISC005Fixture(runID)
		f.effectiveNoindex = "malformed_unknown_token"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:5")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 6. HTTP 200 XML -> NOT_APPLICABLE
	t.Run("6. HTTP 200 XML -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:6"
		f := defaultDISC005Fixture(runID)
		f.contentType = "application/xml"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:6")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 7. HTTP 200 JSON -> NOT_APPLICABLE
	t.Run("7. HTTP 200 JSON -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:7"
		f := defaultDISC005Fixture(runID)
		f.contentType = "application/json"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:7")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 8. HTTP 200 PDF/image -> NOT_APPLICABLE
	t.Run("8. HTTP 200 PDF/image -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:8"
		f := defaultDISC005Fixture(runID)
		f.contentType = "application/pdf"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:8")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 9. HTTP 200 + missing content type -> UNKNOWN
	t.Run("9. HTTP 200 + missing content type -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:9"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"content_type"}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:9")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 10. HTTP 200 + incomplete HTML evidence (conflicting content type) -> UNKNOWN
	t.Run("10. HTTP 200 + conflicting content type -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:10"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"content_type"}
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:  "obs:ct:1",
				AuditRunID:     audit.AuditRunID(runID),
				SnapshotID:     "snap:test:10",
				SubjectType:    audit.SubjectURL,
				SubjectRef:     f.urlSubjectRef,
				Field:          "content_type",
				Value:          "text/html",
				DerivationType: audit.DerivationDirect,
			},
			{
				ObservationID:  "obs:ct:2",
				AuditRunID:     audit.AuditRunID(runID),
				SnapshotID:     "snap:test:10",
				SubjectType:    audit.SubjectURL,
				SubjectRef:     f.urlSubjectRef,
				Field:          "content_type",
				Value:          "application/pdf",
				DerivationType: audit.DerivationDirect,
			},
		}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:10")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 11. HTTP 301 -> terminal 200 HTML -> NOT_APPLICABLE
	t.Run("11. HTTP 301 -> terminal 200 HTML -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:11"
		f := defaultDISC005Fixture(runID)
		f.httpStatus = "301"
		f.redirectInitialObserved = "true"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:11")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 12. HTTP 302/307/308 -> NOT_APPLICABLE
	t.Run("12. HTTP 302/307/308 -> NOT_APPLICABLE", func(t *testing.T) {
		for _, st := range []string{"302", "307", "308"} {
			runID := "run:test:12:" + st
			f := defaultDISC005Fixture(runID)
			f.httpStatus = st
			f.redirectInitialObserved = "true"
			snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:12:"+st)

			results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
			if err != nil {
				t.Fatalf("EvaluateRule failed for %s: %v", st, err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result for %s, got %d", st, len(results))
			}
			assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
		}
	})

	// 13. HTTP 404 -> NOT_APPLICABLE
	t.Run("13. HTTP 404 -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:13"
		f := defaultDISC005Fixture(runID)
		f.httpStatus = "404"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:13")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 14. HTTP 500 -> NOT_APPLICABLE
	t.Run("14. HTTP 500 -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:14"
		f := defaultDISC005Fixture(runID)
		f.httpStatus = "500"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:14")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 15. Network failure before HTTP response -> frozen-contract-consistent result (NOT_APPLICABLE)
	t.Run("15. Network failure before HTTP response -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:15"
		f := defaultDISC005Fixture(runID)
		f.httpStatus = "0"
		f.fetchError = "connection_refused"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:15")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 16. Missing HTTP status -> UNKNOWN
	t.Run("16. Missing HTTP status -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:16"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"http_status"}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:16")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 17. Contradictory status observations -> UNKNOWN
	t.Run("17. Contradictory status observations -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:17"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"http_status"}
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:  "obs:st:1",
				AuditRunID:     audit.AuditRunID(runID),
				SnapshotID:     "snap:test:17",
				SubjectType:    audit.SubjectURL,
				SubjectRef:     f.urlSubjectRef,
				Field:          "http_status",
				Value:          "200",
				DerivationType: audit.DerivationDirect,
			},
			{
				ObservationID:  "obs:st:2",
				AuditRunID:     audit.AuditRunID(runID),
				SnapshotID:     "snap:test:17",
				SubjectType:    audit.SubjectURL,
				SubjectRef:     f.urlSubjectRef,
				Field:          "http_status",
				Value:          "404",
				DerivationType: audit.DerivationDirect,
			},
		}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:17")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 18. URL dictionary exists but not fetched -> UNKNOWN
	t.Run("18. URL dictionary exists but not fetched -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:18"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"http_status", "content_type", "effective_noindex"}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:18")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 19. url_id=0 -> UNKNOWN when applicability cannot be determined
	t.Run("19. url_id=0 -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:19"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"url_subject_ref"}
		f.urlSubjectRef = ""
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:19")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 20. Invalid/dangling URL reference -> UNKNOWN
	t.Run("20. Invalid/dangling URL reference -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:20"
		f := defaultDISC005Fixture(runID)
		f.urlSubjectRef = "url:other_run:999" // foreign audit run reference
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:20")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 21. Duplicate <loc> in one sitemap -> separate results
	t.Run("21. Duplicate loc in one sitemap -> separate results", func(t *testing.T) {
		runID := "run:test:21"
		f1 := defaultDISC005Fixture(runID)
		f1.entrySubjRef = fmt.Sprintf("sme:%s:1:0", runID)
		f1.entrySeq = "0"
		f1.effectiveNoindex = "false"

		f2 := defaultDISC005Fixture(runID)
		f2.entrySubjRef = fmt.Sprintf("sme:%s:1:1", runID)
		f2.entrySeq = "1"
		f2.effectiveNoindex = "false"

		snap := newDISC005Snapshot([]disc005Fixture{f1, f2}, runID, "snap:test:21")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].RuleResultID == results[1].RuleResultID {
			t.Fatalf("expected distinct RuleResult IDs for duplicate loc entries")
		}
		assertDISC005ResultIntegrity(t, results[0], f1.entrySubjRef, audit.StatusPass)
		assertDISC005ResultIntegrity(t, results[1], f2.entrySubjRef, audit.StatusPass)
	})

	// 22. Same <loc> in two sitemap documents -> separate results
	t.Run("22. Same loc in two sitemap documents -> separate results", func(t *testing.T) {
		runID := "run:test:22"
		f1 := defaultDISC005Fixture(runID)
		f1.entrySubjRef = fmt.Sprintf("sme:%s:1:0", runID)
		f1.parentSitemapRef = fmt.Sprintf("sm:%s:1", runID)
		f1.sitemapURL = "https://example.com/sitemap1.xml"

		f2 := defaultDISC005Fixture(runID)
		f2.entrySubjRef = fmt.Sprintf("sme:%s:2:0", runID)
		f2.parentSitemapRef = fmt.Sprintf("sm:%s:2", runID)
		f2.sitemapURL = "https://example.com/sitemap2.xml"

		snap := newDISC005Snapshot([]disc005Fixture{f1, f2}, runID, "snap:test:22")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f1.entrySubjRef, audit.StatusPass)
		assertDISC005ResultIntegrity(t, results[1], f2.entrySubjRef, audit.StatusPass)
	})

	// 23. robots.txt disallow without effective noindex -> no invented FAIL (PASS)
	t.Run("23. robots.txt disallow without effective noindex -> PASS", func(t *testing.T) {
		runID := "run:test:23"
		f := defaultDISC005Fixture(runID)
		f.effectiveNoindex = "false"
		// Disallow does not create effective_noindex=true
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:  "obs:rob:1",
				AuditRunID:     audit.AuditRunID(runID),
				SnapshotID:     "snap:test:23",
				SubjectType:    audit.SubjectURL,
				SubjectRef:     f.urlSubjectRef,
				Field:          "robots_txt_allow_googlebot",
				Value:          "false",
				DerivationType: audit.DerivationNormalized,
			},
		}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:23")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusPass)
	})

	// 24. Meta robots noindex -> FAIL when normalized effective state confirms it
	t.Run("24. Meta robots noindex -> FAIL", func(t *testing.T) {
		runID := "run:test:24"
		f := defaultDISC005Fixture(runID)
		f.effectiveNoindex = "true"
		f.metaRobots = "noindex, follow"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:24")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 25. X-Robots-Tag noindex -> FAIL when normalized effective state confirms it
	t.Run("25. X-Robots-Tag noindex -> FAIL", func(t *testing.T) {
		runID := "run:test:25"
		f := defaultDISC005Fixture(runID)
		f.effectiveNoindex = "true"
		f.xRobots = "noindex"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:25")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 26. User-agent-specific directives -> respect existing resolved effective state
	t.Run("26. User-agent-specific directives -> respect resolved effective state", func(t *testing.T) {
		runID := "run:test:26"
		f := defaultDISC005Fixture(runID)
		f.effectiveNoindex = "true"
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:  "obs:gbot:1",
				AuditRunID:     audit.AuditRunID(runID),
				SnapshotID:     "snap:test:26",
				SubjectType:    audit.SubjectURL,
				SubjectRef:     f.urlSubjectRef,
				Field:          "googlebot_meta_robots_raw",
				Value:          "noindex",
				DerivationType: audit.DerivationDirect,
			},
		}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:26")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 27. Conflicting directive source observations -> handle according to existing Adapter contract (UNKNOWN)
	t.Run("27. Conflicting directive source observations -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:27"
		f := defaultDISC005Fixture(runID)
		// When adapter encounters ambiguous directives, effective_noindex is withheld
		f.omitFields = []string{"effective_noindex"}
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:27")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 28. Missing HTML evidence -> do not manufacture PASS (UNKNOWN)
	t.Run("28. Missing HTML evidence -> do not manufacture PASS", func(t *testing.T) {
		runID := "run:test:28"
		f := defaultDISC005Fixture(runID)
		f.omitFields = []string{"content_type"}
		f.effectiveNoindex = "false"
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:28")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC005ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 29. Multiple subjects -> deterministic ordering
	t.Run("29. Multiple subjects -> deterministic ordering", func(t *testing.T) {
		runID := "run:test:29"
		f1 := defaultDISC005Fixture(runID)
		f1.entrySubjRef = fmt.Sprintf("sme:%s:1:0", runID)
		f1.entrySeq = "0"
		f1.effectiveNoindex = "false"

		f2 := defaultDISC005Fixture(runID)
		f2.entrySubjRef = fmt.Sprintf("sme:%s:1:1", runID)
		f2.entrySeq = "1"
		f2.effectiveNoindex = "true"

		snap := newDISC005Snapshot([]disc005Fixture{f1, f2}, runID, "snap:test:29")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].SubjectRef != f1.entrySubjRef || results[1].SubjectRef != f2.entrySubjRef {
			t.Fatalf("unexpected ordering: got %s, %s", results[0].SubjectRef, results[1].SubjectRef)
		}
	})

	// 30. Repeated evaluation -> deterministic results
	t.Run("30. Repeated evaluation -> deterministic results", func(t *testing.T) {
		runID := "run:test:30"
		f := defaultDISC005Fixture(runID)
		snap := newDISC005Snapshot([]disc005Fixture{f}, runID, "snap:test:30")

		res1, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
		if err != nil {
			t.Fatalf("first evaluation failed: %v", err)
		}
		res2, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-005")
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

func TestAR_DISC_005_HermeticSQLiteIntegrationSuite(t *testing.T) {
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

	// 1. Sitemap-listed direct 200 HTML with confirmed effective_noindex=false -> PASS
	t.Run("1. Direct 200 HTML with effective_noindex=false -> PASS", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:pass"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		// URL and page records (HTML, 200, indexable)
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
			MetaTags: map[string]string{
				"robots": "index, follow",
			},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusPass)
	})

	// 2. Sitemap-listed direct 200 HTML with effective_noindex=true -> FAIL
	t.Run("2. Direct 200 HTML with effective_noindex=true -> FAIL", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:fail"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/noindex"
		setupRun(t, db, runID, seed)

		// URL and page records (HTML, 200, meta_robots noindex)
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			CrawledAt:  "2026-10-01T00:00:10Z",
			MetaRobots: "noindex, follow",
			MetaTags: map[string]string{
				"robots": "noindex, follow",
			},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, meta_robots, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', 'noindex, follow', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusFail)
	})

	// 3. HTTP 301 -> destination 200 HTML -> NOT_APPLICABLE
	t.Run("3. HTTP 301 -> destination 200 HTML -> NOT_APPLICABLE", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:redir"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/redirect"
		dest := "https://example.com/final"
		setupRun(t, db, runID, seed)

		// Seed URL 1: 301 redirecting to dest
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: dest,
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, crawled_at)
			VALUES (?, 1, ?, ?, 301, ?, 1, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData1), dest)

		// Dest URL 2: 200 HTML
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 2, ?)`, runID, dest)
		pageData2, _ := json.Marshal(sitecrawl.Page{
			URL:       dest,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:11Z",
			MetaTags:  map[string]string{"robots": "index, follow"},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, crawled_at)
			VALUES (?, 2, ?, ?, 200, 'text/html', 'html', '2026-10-01T00:00:11Z')`, runID, dest, string(pageData2))

		// Sitemap entry lists seed URL (id 1)
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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusNotApplicable)
	})

	// 4. Direct HTTP 404 -> NOT_APPLICABLE
	t.Run("4. Direct HTTP 404 -> NOT_APPLICABLE", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:404"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/not-found"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    404,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, crawled_at)
			VALUES (?, 1, ?, ?, 404, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusNotApplicable)
	})

	// 5. Direct HTTP 200 non-HTML -> NOT_APPLICABLE
	t.Run("5. Direct HTTP 200 non-HTML -> NOT_APPLICABLE", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:xml"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/feed.xml"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'application/xml', 'xml', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusNotApplicable)
	})

	// 6. HTML page with missing directives evidence -> UNKNOWN
	t.Run("6. HTML page with missing directives evidence -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:nodir"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/no-directives"
		setupRun(t, db, runID, seed)

		// Page record with empty/unparsed MetaTags and rendered=true so effective_noindex is withheld
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, rendered, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', 1, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 7. URL dictionary record without successful fetch -> UNKNOWN
	t.Run("7. URL dictionary record without successful fetch -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:unfetched"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/unfetched"
		setupRun(t, db, runID, seed)

		// URL exists in sitecrawl_urls, but NO record in sitecrawl_pages
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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 8. Sitemap entry with url_id=0 -> UNKNOWN
	t.Run("8. Sitemap entry with url_id=0 -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:url0"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		// url_id = 0
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 0, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 9. Duplicate sitemap entries -> separate results
	t.Run("9. Duplicate sitemap entries -> separate results", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:dup"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
			MetaTags:  map[string]string{"robots": "index, follow"},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 2)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 2)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		// Two entries with identical loc
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 1, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		subj0 := fmt.Sprintf("sme:%s:1:0", auditRunID)
		subj1 := fmt.Sprintf("sme:%s:1:1", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], subj0, audit.StatusPass)
		assertDISC005ResultIntegrity(t, results[1], subj1, audit.StatusPass)
	})

	// 10. Multiple sitemap documents listing the same URL -> separate results
	t.Run("10. Multiple sitemap documents listing the same URL -> separate results", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:multidoc"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
			MetaTags:  map[string]string{"robots": "index, follow"},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 2, 2)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap1.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 2, 'https://example.com/sitemap2.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 2, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 1, 0, 1, ?)`, runID, seed)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc) VALUES (?, 2, 0, 1, ?)`, runID, seed)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		subj1 := fmt.Sprintf("sme:%s:1:0", auditRunID)
		subj2 := fmt.Sprintf("sme:%s:2:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], subj1, audit.StatusPass)
		assertDISC005ResultIntegrity(t, results[1], subj2, audit.StatusPass)
	})

	// 11. Meta robots noindex -> FAIL
	t.Run("11. Meta robots noindex -> FAIL", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:metarob"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/meta-noindex"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			CrawledAt:  "2026-10-01T00:00:10Z",
			MetaRobots: "noindex, follow",
			MetaTags: map[string]string{
				"robots": "noindex, follow",
			},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, meta_robots, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', 'noindex, follow', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusFail)
	})

	// 12. X-Robots-Tag noindex -> FAIL
	t.Run("12. X-Robots-Tag noindex -> FAIL", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:xrob"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/xrob-noindex"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			CrawledAt:  "2026-10-01T00:00:10Z",
			XRobotsTag: "noindex, follow",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, x_robots, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', 'noindex, follow', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC005ResultIntegrity(t, results[0], expectedSubj, audit.StatusFail)
	})

	// 13. EvidenceRefs resolve correctly in snapshot
	t.Run("13. EvidenceRefs resolve correctly", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:refs"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
			MetaTags:  map[string]string{"robots": "index, follow"},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		snapObsByID := make(map[audit.ObservationID]bool)
		for _, o := range buildRes.EvidenceSnapshot.NormalizedObservations {
			snapObsByID[o.ObservationID] = true
		}

		for _, ref := range results[0].EvidenceRefs {
			if ref.EvidenceType == "NORMALIZED_OBSERVATION" {
				if !snapObsByID[audit.ObservationID(ref.EvidenceRef)] {
					t.Errorf("evidence reference %s (%s) does not resolve to an observation in snapshot", ref.EvidenceRef, ref.Field)
				}
			}
		}
	})

	// 14. All 17 evaluators remain green and unaffected
	t.Run("14. All 17 evaluators execute without regression", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc005:allrules"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
			MetaTags:  map[string]string{"robots": "index, follow"},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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
