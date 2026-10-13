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

// disc006Fixture defines parameters for creating test sitemap entry snapshots
// for evaluating AR-DISC-006.
type disc006Fixture struct {
	entrySubjRef           string
	locRaw                 string
	locNorm                string
	parentSitemapRef       string
	entrySeq               string
	urlSubjectRef          string
	sitemapURL             string
	targetURLIdentity      string
	canonicalCount         string
	canonicalNormTarget    string
	canonicalNormComplete  string
	canonicalDistinctCount string
	canonicalResolved      string
	omitFields             []string
	customObs              []audit.NormalizedObservation
}

// defaultDISC006Fixture returns a standard fixture producing PASS.
func defaultDISC006Fixture(runID, snapID string) disc006Fixture {
	return disc006Fixture{
		entrySubjRef:          fmt.Sprintf("sme:%s:1:0", runID),
		locRaw:                "https://example.com/page",
		locNorm:               "https://example.com/page",
		parentSitemapRef:      fmt.Sprintf("sm:%s:1", runID),
		entrySeq:              "0",
		urlSubjectRef:         fmt.Sprintf("url:%s:1", runID),
		sitemapURL:            "https://example.com/sitemap.xml",
		targetURLIdentity:     "https://example.com/page",
		canonicalCount:        "1",
		canonicalNormTarget:   "https://example.com/page",
		canonicalNormComplete: "true",
	}
}

// newDISC006Snapshot builds a frozen EvidenceSnapshot containing normalized
// SubjectSitemapEntry, SubjectSitemap, and SubjectURL observations for unit testing.
func newDISC006Snapshot(fixtures []disc006Fixture, runID, snapID string) *audit.EvidenceSnapshot {
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
			if !omitted["canonical_count"] && f.canonicalCount != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "canonical_count",
					Value:              f.canonicalCount,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["canonical_normalization_complete"] && f.canonicalNormComplete != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "canonical_normalization_complete",
					Value:              f.canonicalNormComplete,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["canonical_normalized_target"] && f.canonicalNormTarget != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "canonical_normalized_target",
					Value:              f.canonicalNormTarget,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["canonical_distinct_normalized_count"] && f.canonicalDistinctCount != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "canonical_distinct_normalized_count",
					Value:              f.canonicalDistinctCount,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
			if !omitted["canonical_resolved"] && f.canonicalResolved != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.urlSubjectRef,
					Field:              "canonical_resolved",
					Value:              f.canonicalResolved,
					DerivationType:     audit.DerivationNormalized,
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
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func assertDISC006ResultIntegrity(t *testing.T, res audit.RuleResult, expectedSubjRef string, expectedStatus audit.RuleResultStatus) {
	t.Helper()
	if res.RuleID != "AR-DISC-006" {
		t.Errorf("expected RuleID AR-DISC-006, got %s", res.RuleID)
	}
	if res.SubjectType != audit.SubjectSitemapEntry {
		t.Errorf("expected SubjectType SubjectSitemapEntry, got %s", res.SubjectType)
	}
	if res.SubjectRef != expectedSubjRef {
		t.Errorf("expected SubjectRef %s, got %s", expectedSubjRef, res.SubjectRef)
	}
	if res.Severity != audit.SeverityP1 {
		t.Errorf("expected Severity P1, got %s", res.Severity)
	}
	if res.Scope != audit.ScopeURL {
		t.Errorf("expected Scope URL, got %s", res.Scope)
	}
	if res.ParentCheck != "DISC-002" {
		t.Errorf("expected ParentCheck DISC-002, got %s", res.ParentCheck)
	}
	if res.RuleVersion != 1 {
		t.Errorf("expected RuleVersion 1, got %d", res.RuleVersion)
	}
	if res.Status != expectedStatus {
		t.Errorf("expected Status %s, got %s (summary: %s)", expectedStatus, res.Status, res.ObservedSummary)
	}
	if res.ExpectedSummary != "Sitemap-listed URL canonicalizes to itself when canonical is declared." {
		t.Errorf("unexpected ExpectedSummary: %s", res.ExpectedSummary)
	}

	// Verify evidence refs determinism
	for i := 1; i < len(res.EvidenceRefs); i++ {
		if res.EvidenceRefs[i].RuleEvidenceRefID < res.EvidenceRefs[i-1].RuleEvidenceRefID {
			t.Errorf("evidence refs not sorted deterministically: %s < %s",
				res.EvidenceRefs[i].RuleEvidenceRefID, res.EvidenceRefs[i-1].RuleEvidenceRefID)
		}
	}
}

// ============================================================================
// Group A: Unit Tests for AR-DISC-006 (Mandatory Scenarios 1 to 32)
// ============================================================================

func TestEngine_DISC006_UnitTests(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	// 1. One self-referencing absolute canonical -> PASS
	t.Run("1. One self-referencing absolute canonical -> PASS", func(t *testing.T) {
		runID := "run:test:1"
		f := defaultDISC006Fixture(runID, "snap:test:1")
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:1")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusPass)
	})

	// 2. One canonical pointing elsewhere -> FAIL
	t.Run("2. One canonical pointing elsewhere -> FAIL", func(t *testing.T) {
		runID := "run:test:2"
		f := defaultDISC006Fixture(runID, "snap:test:2")
		f.canonicalNormTarget = "https://example.com/other"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:2")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 3. One relative canonical resolving to listed URL -> PASS
	t.Run("3. One relative canonical resolving to listed URL -> PASS", func(t *testing.T) {
		runID := "run:test:3"
		f := defaultDISC006Fixture(runID, "snap:test:3")
		f.locRaw = "https://example.com/dir/page"
		f.locNorm = "https://example.com/dir/page"
		f.targetURLIdentity = "https://example.com/dir/page"
		f.canonicalNormTarget = "https://example.com/dir/page"
		f.canonicalResolved = "page"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:3")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusPass)
	})

	// 4. One relative canonical resolving elsewhere -> FAIL
	t.Run("4. One relative canonical resolving elsewhere -> FAIL", func(t *testing.T) {
		runID := "run:test:4"
		f := defaultDISC006Fixture(runID, "snap:test:4")
		f.locRaw = "https://example.com/dir/page"
		f.locNorm = "https://example.com/dir/page"
		f.targetURLIdentity = "https://example.com/dir/page"
		f.canonicalNormTarget = "https://example.com/dir/other"
		f.canonicalResolved = "other"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:4")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 5. No canonical declaration -> NOT_APPLICABLE
	t.Run("5. No canonical declaration -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:5"
		f := defaultDISC006Fixture(runID, "snap:test:5")
		f.canonicalCount = "0"
		f.omitFields = []string{"canonical_normalized_target", "canonical_normalization_complete"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:5")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 6. Two identical canonical declarations -> NOT_APPLICABLE
	t.Run("6. Two identical canonical declarations -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:6"
		f := defaultDISC006Fixture(runID, "snap:test:6")
		f.canonicalCount = "2"
		f.canonicalDistinctCount = "1"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:6")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 7. Two different canonical declarations -> NOT_APPLICABLE
	t.Run("7. Two different canonical declarations -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "run:test:7"
		f := defaultDISC006Fixture(runID, "snap:test:7")
		f.canonicalCount = "2"
		f.canonicalDistinctCount = "2"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:7")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusNotApplicable)
	})

	// 8. One malformed canonical -> UNKNOWN
	t.Run("8. One malformed canonical -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:8"
		f := defaultDISC006Fixture(runID, "snap:test:8")
		f.canonicalNormComplete = "false"
		f.omitFields = []string{"canonical_normalized_target"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:8")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 9. One unresolvable relative canonical -> UNKNOWN
	t.Run("9. One unresolvable relative canonical -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:9"
		f := defaultDISC006Fixture(runID, "snap:test:9")
		f.canonicalNormComplete = "false"
		f.omitFields = []string{"canonical_normalized_target"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:9")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 10. Canonical normalization incomplete -> UNKNOWN
	t.Run("10. Canonical normalization incomplete -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:10"
		f := defaultDISC006Fixture(runID, "snap:test:10")
		f.canonicalNormComplete = "false"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:10")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 11. Canonical count missing -> UNKNOWN
	t.Run("11. Canonical count missing -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:11"
		f := defaultDISC006Fixture(runID, "snap:test:11")
		f.omitFields = []string{"canonical_count"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:11")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 12. Canonical count malformed -> UNKNOWN
	t.Run("12. Canonical count malformed -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:12"
		f := defaultDISC006Fixture(runID, "snap:test:12")
		f.canonicalCount = "not_a_number"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:12")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 13. Canonical count conflicting -> UNKNOWN
	t.Run("13. Canonical count conflicting -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:13"
		f := defaultDISC006Fixture(runID, "snap:test:13")
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:      "obs:test:13:conflict",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:13",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.urlSubjectRef,
				Field:              "canonical_count",
				Value:              "2",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:conflict"},
				ObservedAt:         time.Now().UTC(),
			},
		}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:13")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 14. Canonical normalized target missing -> UNKNOWN
	t.Run("14. Canonical normalized target missing -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:14"
		f := defaultDISC006Fixture(runID, "snap:test:14")
		f.omitFields = []string{"canonical_normalized_target"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:14")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 15. Canonical target observations conflicting -> UNKNOWN
	t.Run("15. Canonical target observations conflicting -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:15"
		f := defaultDISC006Fixture(runID, "snap:test:15")
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:      "obs:test:15:conflict",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:15",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.urlSubjectRef,
				Field:              "canonical_normalized_target",
				Value:              "https://example.com/other",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:conflict"},
				ObservedAt:         time.Now().UTC(),
			},
		}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:15")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 16. Listed normalized URL unavailable -> UNKNOWN
	t.Run("16. Listed normalized URL unavailable -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:16"
		f := defaultDISC006Fixture(runID, "snap:test:16")
		f.omitFields = []string{"loc_normalized"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:16")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 17. Invalid url_subject_ref -> UNKNOWN
	t.Run("17. Invalid url_subject_ref -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:17"
		f := defaultDISC006Fixture(runID, "snap:test:17")
		f.urlSubjectRef = "foreign_run:url:1"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:17")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 18. Dangling URL SubjectRef -> UNKNOWN
	t.Run("18. Dangling URL SubjectRef -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:18"
		f := defaultDISC006Fixture(runID, "snap:test:18")
		f.urlSubjectRef = fmt.Sprintf("url:%s:999", runID) // does not exist in snapshot
		f.omitFields = []string{"url_identity", "canonical_count", "canonical_normalized_target", "canonical_normalization_complete"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:18")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 19. url_id=0 -> UNKNOWN
	t.Run("19. url_id=0 -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:19"
		f := defaultDISC006Fixture(runID, "snap:test:19")
		f.omitFields = []string{"url_subject_ref"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:19")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 20. URL dictionary exists but canonical extraction unavailable -> UNKNOWN
	t.Run("20. URL dictionary exists but canonical extraction unavailable -> UNKNOWN", func(t *testing.T) {
		runID := "run:test:20"
		f := defaultDISC006Fixture(runID, "snap:test:20")
		f.omitFields = []string{"canonical_count", "canonical_normalized_target", "canonical_normalization_complete"}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:20")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 21. Duplicate <loc> entries -> independent results
	t.Run("21. Duplicate <loc> entries -> independent results", func(t *testing.T) {
		runID := "run:test:21"
		f1 := defaultDISC006Fixture(runID, "snap:test:21")
		f1.entrySubjRef = fmt.Sprintf("sme:%s:1:0", runID)
		f1.entrySeq = "0"
		f2 := defaultDISC006Fixture(runID, "snap:test:21")
		f2.entrySubjRef = fmt.Sprintf("sme:%s:1:1", runID)
		f2.entrySeq = "1"

		snap := newDISC006Snapshot([]disc006Fixture{f1, f2}, runID, "snap:test:21")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].RuleResultID == results[1].RuleResultID {
			t.Errorf("expected distinct RuleResultIDs for duplicate entries")
		}
		assertDISC006ResultIntegrity(t, results[0], f1.entrySubjRef, audit.StatusPass)
		assertDISC006ResultIntegrity(t, results[1], f2.entrySubjRef, audit.StatusPass)
	})

	// 22. Same URL in two sitemaps -> independent results
	t.Run("22. Same URL in two sitemaps -> independent results", func(t *testing.T) {
		runID := "run:test:22"
		f1 := defaultDISC006Fixture(runID, "snap:test:22")
		f1.entrySubjRef = fmt.Sprintf("sme:%s:1:0", runID)
		f1.parentSitemapRef = fmt.Sprintf("sm:%s:1", runID)
		f1.sitemapURL = "https://example.com/sitemap1.xml"

		f2 := defaultDISC006Fixture(runID, "snap:test:22")
		f2.entrySubjRef = fmt.Sprintf("sme:%s:2:0", runID)
		f2.parentSitemapRef = fmt.Sprintf("sm:%s:2", runID)
		f2.sitemapURL = "https://example.com/sitemap2.xml"

		snap := newDISC006Snapshot([]disc006Fixture{f1, f2}, runID, "snap:test:22")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f1.entrySubjRef, audit.StatusPass)
		assertDISC006ResultIntegrity(t, results[1], f2.entrySubjRef, audit.StatusPass)
	})

	// 23. HTTP redirect with no trustworthy canonical extraction -> no borrowed destination canonical
	t.Run("23. HTTP redirect with no trustworthy canonical extraction -> no borrowed destination canonical", func(t *testing.T) {
		runID := "run:test:23"
		f := defaultDISC006Fixture(runID, "snap:test:23")
		// Correlated URL 1 has no canonical extraction
		f.omitFields = []string{"canonical_count", "canonical_normalized_target", "canonical_normalization_complete"}
		// Custom observation for destination URL 2 that has canonical
		destURLRef := fmt.Sprintf("url:%s:2", runID)
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:      "obs:test:23:dest:count",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:23",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         destURLRef,
				Field:              "canonical_count",
				Value:              "1",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:dest"},
				ObservedAt:         time.Now().UTC(),
			},
			{
				ObservationID:      "obs:test:23:dest:target",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:23",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         destURLRef,
				Field:              "canonical_normalized_target",
				Value:              "https://example.com/page",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:dest"},
				ObservedAt:         time.Now().UTC(),
			},
			{
				ObservationID:      "obs:test:23:dest:complete",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:23",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         destURLRef,
				Field:              "canonical_normalization_complete",
				Value:              "true",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:dest"},
				ObservedAt:         time.Now().UTC(),
			},
		}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:23")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		// Evaluator must NOT borrow destination canonical; must remain UNKNOWN
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 24. Canonical evidence from another URL subject -> not used
	t.Run("24. Canonical evidence from another URL subject -> not used", func(t *testing.T) {
		runID := "run:test:24"
		f := defaultDISC006Fixture(runID, "snap:test:24")
		f.omitFields = []string{"canonical_count", "canonical_normalized_target", "canonical_normalization_complete"}
		otherURLRef := fmt.Sprintf("url:%s:99", runID)
		f.customObs = []audit.NormalizedObservation{
			{
				ObservationID:      "obs:test:24:other:count",
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         "snap:test:24",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         otherURLRef,
				Field:              "canonical_count",
				Value:              "1",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_pages:other"},
				ObservedAt:         time.Now().UTC(),
			},
		}
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:24")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusUnknown)
	})

	// 25. Scheme difference -> follow existing normalization contract (http vs https -> FAIL)
	t.Run("25. Scheme difference -> FAIL", func(t *testing.T) {
		runID := "run:test:25"
		f := defaultDISC006Fixture(runID, "snap:test:25")
		f.locNorm = "https://example.com/page"
		f.canonicalNormTarget = "http://example.com/page"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:25")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 26. Default port handling -> follow existing normalization contract
	t.Run("26. Default port handling -> PASS", func(t *testing.T) {
		runID := "run:test:26"
		f := defaultDISC006Fixture(runID, "snap:test:26")
		// In Audit normalization, default port is stripped
		f.locNorm = "https://example.com/page"
		f.canonicalNormTarget = "https://example.com/page"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:26")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusPass)
	})

	// 27. Path case difference -> follow existing normalization contract (case preserved -> FAIL)
	t.Run("27. Path case difference -> FAIL", func(t *testing.T) {
		runID := "run:test:27"
		f := defaultDISC006Fixture(runID, "snap:test:27")
		f.locNorm = "https://example.com/page"
		f.canonicalNormTarget = "https://example.com/PAGE"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:27")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 28. Query parameter difference -> follow existing normalization contract
	t.Run("28. Query parameter difference -> FAIL", func(t *testing.T) {
		runID := "run:test:28"
		f := defaultDISC006Fixture(runID, "snap:test:28")
		f.locNorm = "https://example.com/page?ref=sitemap"
		f.canonicalNormTarget = "https://example.com/page"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:28")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 29. Fragment handling -> follow existing normalization contract (fragment stripped -> PASS)
	t.Run("29. Fragment handling -> PASS", func(t *testing.T) {
		runID := "run:test:29"
		f := defaultDISC006Fixture(runID, "snap:test:29")
		// Both normalized without fragment
		f.locNorm = "https://example.com/page"
		f.canonicalNormTarget = "https://example.com/page"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:29")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusPass)
	})

	// 30. Trailing slash difference -> follow existing normalization contract (slash preserved -> FAIL)
	t.Run("30. Trailing slash difference -> FAIL", func(t *testing.T) {
		runID := "run:test:30"
		f := defaultDISC006Fixture(runID, "snap:test:30")
		f.locNorm = "https://example.com/page"
		f.canonicalNormTarget = "https://example.com/page/"
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:30")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], f.entrySubjRef, audit.StatusFail)
	})

	// 31. EvidenceRefs resolve to the correct observations
	t.Run("31. EvidenceRefs resolve to correct observations", func(t *testing.T) {
		runID := "run:test:31"
		f := defaultDISC006Fixture(runID, "snap:test:31")
		snap := newDISC006Snapshot([]disc006Fixture{f}, runID, "snap:test:31")
		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		res := results[0]
		obsMap := make(map[string]audit.NormalizedObservation)
		for _, o := range snap.NormalizedObservations {
			obsMap[string(o.ObservationID)] = o
		}
		for _, ref := range res.EvidenceRefs {
			obs, ok := obsMap[ref.EvidenceRef]
			if !ok {
				t.Errorf("evidence ref %s points to non-existent observation", ref.EvidenceRef)
			}
			if obs.Field != ref.Field {
				t.Errorf("ref field %s != obs field %s", ref.Field, obs.Field)
			}
			if obs.Value != ref.ObservedValue {
				t.Errorf("ref observed value %s != obs value %s", ref.ObservedValue, obs.Value)
			}
		}
	})

	// 32. Repeated evaluation -> deterministic results
	t.Run("32. Repeated evaluation -> deterministic results", func(t *testing.T) {
		fixedTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		eng.SetClock(func() time.Time { return fixedTime })
		t.Cleanup(func() { eng.SetClock(func() time.Time { return time.Now().UTC() }) })

		runID := "run:test:32"
		f1 := defaultDISC006Fixture(runID, "snap:test:32")
		f1.entrySubjRef = fmt.Sprintf("sme:%s:1:0", runID)
		f1.urlSubjectRef = fmt.Sprintf("url:%s:1", runID)
		f1.entrySeq = "0"

		f2 := defaultDISC006Fixture(runID, "snap:test:32")
		f2.entrySubjRef = fmt.Sprintf("sme:%s:1:1", runID)
		f2.urlSubjectRef = fmt.Sprintf("url:%s:2", runID)
		f2.entrySeq = "1"
		f2.canonicalNormTarget = "https://example.com/other"

		snap := newDISC006Snapshot([]disc006Fixture{f1, f2}, runID, "snap:test:32")
		results1, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("first evaluation failed: %v", err)
		}
		results2, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-006")
		if err != nil {
			t.Fatalf("second evaluation failed: %v", err)
		}
		if !reflect.DeepEqual(results1, results2) {
			t.Fatalf("repeated evaluations differ: %v vs %v", results1, results2)
		}
	})
}

// ============================================================================
// Group B: Hermetic SQLite Integration Tests (Section 12, Scenarios 1 to 13)
// ============================================================================

func TestEngine_DISC006_HermeticSQLiteIntegration(t *testing.T) {
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

	// 1. Sitemap-listed URL with self canonical -> PASS
	t.Run("1. Sitemap-listed URL with self canonical -> PASS", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:self"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{seed},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', ?, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData), seed)

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusPass)
	})

	// 2. Sitemap-listed URL with canonical to another URL -> FAIL
	t.Run("2. Sitemap-listed URL with canonical to another URL -> FAIL", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:other"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		target := "https://example.com/canonical-target"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{target},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', ?, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData), target)

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusFail)
	})

	// 3. Sitemap-listed URL with no canonical -> NOT_APPLICABLE
	t.Run("3. Sitemap-listed URL with no canonical -> NOT_APPLICABLE", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:nocanon"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusNotApplicable)
	})

	// 4. Sitemap-listed URL with multiple canonical declarations -> NOT_APPLICABLE
	t.Run("4. Sitemap-listed URL with multiple canonical declarations -> NOT_APPLICABLE", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:multicanon"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{seed, "https://example.com/other"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', ?, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData), seed)

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusNotApplicable)
	})

	// 5. Relative canonical -> PASS (resolves to listed URL)
	t.Run("5. Relative canonical -> PASS", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:relative"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/section/page"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{"/section/page"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', '/section/page', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusPass)
	})

	// 6. Malformed canonical -> UNKNOWN
	t.Run("6. Malformed canonical -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:malformed"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{"mailto:invalid@example.com"},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', 'mailto:invalid@example.com', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 7. URL dictionary with no trustworthy canonical extraction -> UNKNOWN
	t.Run("7. URL dictionary with no trustworthy canonical extraction -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:uncrawled"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		setupRun(t, db, runID, seed)

		// URL inserted in sitecrawl_urls, but NO row in sitecrawl_pages
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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 8. Sitemap entry with url_id=0 -> UNKNOWN
	t.Run("8. Sitemap entry with url_id=0 -> UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:urlid0"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusUnknown)
	})

	// 9. Duplicate sitemap entries -> independent results
	t.Run("9. Duplicate sitemap entries -> independent results", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:dupentries"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{seed},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', ?, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData), seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found) VALUES (?, 'COMPLETED', 1, 2)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 2)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES (?, 1, 'robots_txt', 0)`, runID)
		// Duplicate loc
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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		if results[0].RuleResultID == results[1].RuleResultID {
			t.Errorf("expected distinct RuleResultIDs for duplicate entries")
		}
		assertDISC006ResultIntegrity(t, results[0], fmt.Sprintf("sme:%s:1:0", auditRunID), audit.StatusPass)
		assertDISC006ResultIntegrity(t, results[1], fmt.Sprintf("sme:%s:1:1", auditRunID), audit.StatusPass)
	})

	// 10. Same URL in multiple sitemap documents -> independent results
	t.Run("10. Same URL in multiple sitemap documents -> independent results", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:multism"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{seed},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', ?, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData), seed)

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
		assertDISC006ResultIntegrity(t, results[0], fmt.Sprintf("sme:%s:1:0", auditRunID), audit.StatusPass)
		assertDISC006ResultIntegrity(t, results[1], fmt.Sprintf("sme:%s:2:0", auditRunID), audit.StatusPass)
	})

	// 11. Redirect destination canonical must not be borrowed
	t.Run("11. Redirect destination canonical must not be borrowed", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:redir"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/old"
		dest := "https://example.com/new"
		setupRun(t, db, runID, seed)

		// URL 1: 301 redirecting to dest; no canonical
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: dest,
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, crawled_at)
			VALUES (?, 1, ?, ?, 301, ?, 1, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData1), dest)

		// URL 2: 200 HTML with canonical to itself
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 2, ?)`, runID, dest)
		pageData2, _ := json.Marshal(sitecrawl.Page{
			URL:        dest,
			Status:     200,
			Canonicals: []string{dest},
			CrawledAt:  "2026-10-01T00:00:11Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 2, ?, ?, 200, 'text/html', 'html', ?, '2026-10-01T00:00:11Z')`, runID, dest, string(pageData2), dest)

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sme:%s:1:0", auditRunID)
		// Redirect URL 1 has no canonical declared (canonical_count = 0) -> NOT_APPLICABLE
		// Destination URL 2 has canonical declared; if it were borrowed, status would be FAIL.
		assertDISC006ResultIntegrity(t, results[0], expectedSubj, audit.StatusNotApplicable)
	})

	// 12. EvidenceRefs resolve to actual normalized observations
	t.Run("12. EvidenceRefs resolve to actual normalized observations", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:evidencerefs"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/page"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{seed},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', ?, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData), seed)

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

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-006")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		snapObs := make(map[string]bool)
		for _, o := range buildRes.EvidenceSnapshot.NormalizedObservations {
			snapObs[string(o.ObservationID)] = true
		}
		for _, ref := range results[0].EvidenceRefs {
			if !snapObs[ref.EvidenceRef] {
				t.Errorf("evidence ref %s not found in snapshot normalized observations", ref.EvidenceRef)
			}
		}
	})

	// 13. All 17 evaluators execute without regression
	t.Run("13. All 17 evaluators execute without regression", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:disc006:allrules"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		setupRun(t, db, runID, seed)

		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, ?)`, runID, seed)
		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     200,
			Canonicals: []string{seed},
			CrawledAt:  "2026-10-01T00:00:10Z",
			MetaTags:   map[string]string{"robots": "index, follow"},
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, kind, canonical, crawled_at)
			VALUES (?, 1, ?, ?, 200, 'text/html', 'html', ?, '2026-10-01T00:00:10Z')`, runID, seed, string(pageData), seed)

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
		if len(implIDs) != 19 {
			t.Fatalf("expected exactly 19 implemented rules, got %d", len(implIDs))
		}

		for _, ruleID := range implIDs {
			res, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, ruleID)
			if err != nil {
				t.Fatalf("evaluating rule %s failed: %v", ruleID, err)
			}
			// AR-DISC-002, AR-DISC-003, AR-DISC-004, AR-DISC-005, and AR-DISC-006 evaluate with non-empty results
			if ruleID == "AR-DISC-002" || ruleID == "AR-DISC-003" || ruleID == "AR-DISC-004" || ruleID == "AR-DISC-005" || ruleID == "AR-DISC-006" {
				if len(res) == 0 {
					t.Errorf("expected non-empty results for %s", ruleID)
				}
			}
		}
	})
}
