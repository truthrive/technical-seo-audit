package engine_test

import (
	"context"
	"database/sql"
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

// sitemapDocFixture defines test fixture inputs for a SubjectSitemap document.
type sitemapDocFixture struct {
	subjectRef    string
	url           string
	finalStatus   string
	initialStatus string
	fetchComplete string
	parseStatus   string
	docType       string
	parseError    string
	omitFields    []string
	customObs     []audit.NormalizedObservation
}

// newSitemapSnapshot constructs a frozen EvidenceSnapshot containing normalized
// SubjectSitemap observations for unit testing AR-DISC-003.
func newSitemapSnapshot(fixtures []sitemapDocFixture, runID, snapID string) *audit.EvidenceSnapshot {
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

		if !omitted["sitemap_url"] && f.url != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "sitemap_url",
				Value:              f.url,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["final_http_status"] && f.finalStatus != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "final_http_status",
				Value:              f.finalStatus,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["initial_http_status"] && f.initialStatus != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "initial_http_status",
				Value:              f.initialStatus,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["sitemap_fetch_complete"] && f.fetchComplete != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "sitemap_fetch_complete",
				Value:              f.fetchComplete,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["sitemap_parse_status"] && f.parseStatus != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "sitemap_parse_status",
				Value:              f.parseStatus,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["sitemap_document_type"] && f.docType != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "sitemap_document_type",
				Value:              f.docType,
				DerivationType:     audit.DerivationDerived,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}
		if !omitted["sitemap_parse_error"] && f.parseError != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "sitemap_parse_error",
				Value:              f.parseError,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}
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

// assertResultIntegrity checks the frozen contract invariants on a RuleResult.
func assertResultIntegrity(t *testing.T, rr audit.RuleResult, expectedRuleID, expectedSubjectRef string, expectedStatus audit.RuleResultStatus) {
	t.Helper()
	if rr.RuleID != expectedRuleID {
		t.Errorf("RuleID want %q, got %q", expectedRuleID, rr.RuleID)
	}
	if rr.ParentCheck != "DISC-002" {
		t.Errorf("ParentCheck want 'DISC-002', got %q", rr.ParentCheck)
	}
	if rr.RuleVersion != 1 {
		t.Errorf("RuleVersion want 1, got %d", rr.RuleVersion)
	}
	if rr.SubjectType != audit.SubjectSitemap {
		t.Errorf("SubjectType want SITEMAP, got %q", rr.SubjectType)
	}
	if rr.SubjectRef != expectedSubjectRef {
		t.Errorf("SubjectRef want %q, got %q", expectedSubjectRef, rr.SubjectRef)
	}
	if rr.Status != expectedStatus {
		t.Errorf("Status want %q, got %q", expectedStatus, rr.Status)
	}
	if rr.Severity != audit.SeverityP1 {
		t.Errorf("Severity want P1, got %q", rr.Severity)
	}
	if rr.Scope != audit.ScopeSiteConfiguration {
		t.Errorf("Scope want SITE_CONFIGURATION, got %q", rr.Scope)
	}
	if rr.ObservedSummary == "" {
		t.Errorf("ObservedSummary is empty")
	}
	if rr.ExpectedSummary == "" {
		t.Errorf("ExpectedSummary is empty")
	}
	if len(rr.EvidenceRefs) == 0 {
		t.Errorf("EvidenceRefs is empty")
	}
	// Verify sorting of evidence refs
	for i := 1; i < len(rr.EvidenceRefs); i++ {
		if rr.EvidenceRefs[i].RuleEvidenceRefID < rr.EvidenceRefs[i-1].RuleEvidenceRefID {
			t.Errorf("EvidenceRefs not sorted deterministically: index %d < index %d", i, i-1)
		}
	}
}

// ============================================================================
// Section 10: Mandatory Unit Tests (15 Cases)
// ============================================================================

func TestAR_DISC_003_MandatoryUnitSuite(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	// 1. Valid urlset -> PASS
	t.Run("1. Valid urlset -> PASS", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:1",
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "parsed",
				docType:       "URLSET",
			},
		}, "run:test:1", "snap:test:1")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:1", audit.StatusPass)
	})

	// 2. Valid sitemapindex -> PASS
	t.Run("2. Valid sitemapindex -> PASS", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:2",
				url:           "https://example.com/sitemap_index.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "parsed",
				docType:       "SITEMAP_INDEX",
			},
		}, "run:test:2", "snap:test:2")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:2", audit.StatusPass)
	})

	// 3. xml_error -> FAIL
	t.Run("3. xml_error -> FAIL", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:3",
				url:           "https://example.com/malformed.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "xml_error",
				docType:       "UNKNOWN",
				parseError:    "XML syntax error on line 3: unclosed tag",
			},
		}, "run:test:3", "snap:test:3")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:3", audit.StatusFail)
	})

	// 4. unsupported_structure -> FAIL
	t.Run("4. unsupported_structure -> FAIL", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:4",
				url:           "https://example.com/html_page.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "unsupported_structure",
				docType:       "UNKNOWN",
			},
		}, "run:test:4", "snap:test:4")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:4", audit.StatusFail)
	})

	// 5. empty_input -> FAIL (frozen-contract-consistent result)
	t.Run("5. empty_input -> FAIL", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:5",
				url:           "https://example.com/empty.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "empty_input",
				docType:       "UNKNOWN",
			},
		}, "run:test:5", "snap:test:5")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:5", audit.StatusFail)
	})

	// 6. unavailable -> UNKNOWN
	t.Run("6. unavailable -> UNKNOWN", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:6",
				url:           "https://example.com/corrupt_gzip.xml.gz",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "unavailable",
				docType:       "UNKNOWN",
				parseError:    "gzip: invalid header",
			},
		}, "run:test:6", "snap:test:6")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:6", audit.StatusUnknown)
	})

	// 7. not_attempted with no body -> NOT_APPLICABLE
	t.Run("7. not_attempted with no body -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:7",
				url:           "https://example.com/server_error.xml",
				finalStatus:   "500",
				initialStatus: "500",
				fetchComplete: "true",
				parseStatus:   "not_attempted",
				docType:       "UNKNOWN",
			},
		}, "run:test:7", "snap:test:7")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:7", audit.StatusNotApplicable)
	})

	// 8. HTTP 404 without body -> NOT_APPLICABLE
	t.Run("8. HTTP 404 without body -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:8",
				url:           "https://example.com/not_found.xml",
				finalStatus:   "404",
				initialStatus: "404",
				fetchComplete: "true",
				parseStatus:   "not_attempted",
				docType:       "UNKNOWN",
			},
		}, "run:test:8", "snap:test:8")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:8", audit.StatusNotApplicable)
	})

	// 9. Network failure before body -> NOT_APPLICABLE
	t.Run("9. Network failure before body -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:9",
				url:           "https://example.com/network_down.xml",
				finalStatus:   "0",
				initialStatus: "0",
				fetchComplete: "false",
				parseStatus:   "not_attempted",
				docType:       "UNKNOWN",
			},
		}, "run:test:9", "snap:test:9")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:9", audit.StatusNotApplicable)
	})

	// 10. HTTP 200 with unavailable parser evidence -> UNKNOWN
	t.Run("10. HTTP 200 with unavailable parser evidence -> UNKNOWN", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:10",
				url:           "https://example.com/read_timeout.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "unavailable",
				docType:       "UNKNOWN",
				parseError:    "unexpected EOF while reading body",
			},
		}, "run:test:10", "snap:test:10")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:10", audit.StatusUnknown)
	})

	// 11. Missing parse observation -> UNKNOWN
	t.Run("11. Missing parse observation -> UNKNOWN", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:11",
				url:           "https://example.com/missing_parse.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				omitFields:    []string{"sitemap_parse_status"},
			},
		}, "run:test:11", "snap:test:11")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		assertResultIntegrity(t, results[0], "AR-DISC-003", "sm:run:11", audit.StatusUnknown)
	})

	// 12. Contradictory evidence -> UNKNOWN
	t.Run("12. Contradictory evidence -> UNKNOWN", func(t *testing.T) {
		// Example A: Two conflicting parse statuses
		now := time.Now().UTC()
		snapA := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:12a",
				url:           "https://example.com/conflict.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "parsed",
				docType:       "URLSET",
				customObs: []audit.NormalizedObservation{
					{
						ObservationID:      "obs:conflict:ps",
						AuditRunID:         "run:test:12a",
						SnapshotID:         "snap:test:12a",
						SubjectType:        audit.SubjectSitemap,
						SubjectRef:         "sm:run:12a",
						Field:              "sitemap_parse_status",
						Value:              "xml_error",
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
						ObservedAt:         now,
					},
				},
			},
		}, "run:test:12a", "snap:test:12a")

		resultsA, err := eng.EvaluateRule(context.Background(), snapA, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule conflict A failed: %v", err)
		}
		if len(resultsA) != 1 {
			t.Fatalf("expected 1 result, got %d", len(resultsA))
		}
		assertResultIntegrity(t, resultsA[0], "AR-DISC-003", "sm:run:12a", audit.StatusUnknown)

		// Example B: HTTP 404 with parsed status (impossible/contradictory)
		snapB := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:12b",
				url:           "https://example.com/conflict_status.xml",
				finalStatus:   "404",
				initialStatus: "404",
				fetchComplete: "true",
				parseStatus:   "parsed",
				docType:       "URLSET",
			},
		}, "run:test:12b", "snap:test:12b")

		resultsB, err := eng.EvaluateRule(context.Background(), snapB, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule conflict B failed: %v", err)
		}
		if len(resultsB) != 1 {
			t.Fatalf("expected 1 result, got %d", len(resultsB))
		}
		assertResultIntegrity(t, resultsB[0], "AR-DISC-003", "sm:run:12b", audit.StatusUnknown)
	})

	// 13. Multiple sitemap documents -> separate RuleResults
	t.Run("13. Multiple sitemap documents -> separate RuleResults", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:13a",
				url:           "https://example.com/sitemap1.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "parsed",
				docType:       "URLSET",
			},
			{
				subjectRef:    "sm:run:13b",
				url:           "https://example.com/sitemap2.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "xml_error",
				docType:       "UNKNOWN",
				parseError:    "corrupt XML",
			},
		}, "run:test:13", "snap:test:13")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}

		resBySubj := make(map[string]audit.RuleResult)
		for _, r := range results {
			resBySubj[r.SubjectRef] = r
		}

		res1, ok1 := resBySubj["sm:run:13a"]
		if !ok1 {
			t.Fatalf("missing result for sm:run:13a")
		}
		assertResultIntegrity(t, res1, "AR-DISC-003", "sm:run:13a", audit.StatusPass)

		res2, ok2 := resBySubj["sm:run:13b"]
		if !ok2 {
			t.Fatalf("missing result for sm:run:13b")
		}
		assertResultIntegrity(t, res2, "AR-DISC-003", "sm:run:13b", audit.StatusFail)
	})

	// 14. No sitemap subjects -> no fabricated document results
	t.Run("14. No sitemap subjects -> no fabricated document results", func(t *testing.T) {
		snap := newSitemapSnapshot(nil, "run:test:14", "snap:test:14")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results when no sitemaps exist, got %d", len(results))
		}
	})

	// 15. Duplicate requested URLs with distinct document subject identities -> independent evaluation
	t.Run("15. Duplicate requested URLs with distinct document identities", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:15:doc1",
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "parsed",
				docType:       "URLSET",
			},
			{
				subjectRef:    "sm:run:15:doc2",
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "xml_error",
				docType:       "UNKNOWN",
				parseError:    "corrupted retry artifact",
			},
		}, "run:test:15", "snap:test:15")

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 separate results for 2 distinct subjects, got %d", len(results))
		}

		resBySubj := make(map[string]audit.RuleResult)
		for _, r := range results {
			resBySubj[r.SubjectRef] = r
		}

		res1, ok1 := resBySubj["sm:run:15:doc1"]
		if !ok1 {
			t.Fatalf("missing result for sm:run:15:doc1")
		}
		assertResultIntegrity(t, res1, "AR-DISC-003", "sm:run:15:doc1", audit.StatusPass)

		res2, ok2 := resBySubj["sm:run:15:doc2"]
		if !ok2 {
			t.Fatalf("missing result for sm:run:15:doc2")
		}
		assertResultIntegrity(t, res2, "AR-DISC-003", "sm:run:15:doc2", audit.StatusFail)
	})

	// Determinism: Repeated evaluation yields identical results
	t.Run("Determinism check", func(t *testing.T) {
		snap := newSitemapSnapshot([]sitemapDocFixture{
			{
				subjectRef:    "sm:run:det",
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "200",
				fetchComplete: "true",
				parseStatus:   "parsed",
				docType:       "URLSET",
			},
		}, "run:test:det", "snap:test:det")

		res1, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("run 1 failed: %v", err)
		}
		res2, err := eng.EvaluateRule(context.Background(), snap, "AR-DISC-003")
		if err != nil {
			t.Fatalf("run 2 failed: %v", err)
		}

		if len(res1) != 1 || len(res2) != 1 {
			t.Fatalf("expected 1 result per run")
		}
		if !reflect.DeepEqual(res1[0].EvidenceRefs, res2[0].EvidenceRefs) {
			t.Errorf("evidence refs mismatch across repeated runs")
		}
		if res1[0].Status != res2[0].Status {
			t.Errorf("status mismatch: %s vs %s", res1[0].Status, res2[0].Status)
		}
		if res1[0].RuleResultID != res2[0].RuleResultID {
			t.Errorf("rule result ID mismatch: %s vs %s", res1[0].RuleResultID, res2[0].RuleResultID)
		}
	})
}

// ============================================================================
// Section 11: Mandatory Hermetic Integration Tests
// Full path: SQLite -> adapter.Build() -> EvidenceSnapshot -> engine.Evaluate()
// ============================================================================

func TestAR_DISC_003_HermeticSQLiteIntegrationSuite(t *testing.T) {
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

	// 1. Parsed URLSET produces PASS
	t.Run("Parsed URLSET produces PASS", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:urlset"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found)
			VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count, fetched_at)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 'robots_txt', 0, 200, 200, 200, 1, 'urlset', 'parsed', 1, '2026-10-01T00:00:00Z')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
			VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc)
			VALUES (?, 1, 0, 1, 'https://example.com/page1')`, runID)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sm:%s:1", auditRunID)
		assertResultIntegrity(t, results[0], "AR-DISC-003", expectedSubj, audit.StatusPass)
	})

	// 2. Parsed sitemap index produces PASS
	t.Run("Parsed sitemap index produces PASS", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:smindex"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found)
			VALUES (?, 'COMPLETED', 1, 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count, fetched_at)
			VALUES (?, 2, 'https://example.com/sitemap_index.xml', 'common_path', 0, 200, 200, 200, 1, 'sitemapindex', 'parsed', 0, '2026-10-01T00:00:00Z')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
			VALUES (?, 2, 'common_path', 0)`, runID)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sm:%s:2", auditRunID)
		assertResultIntegrity(t, results[0], "AR-DISC-003", expectedSubj, audit.StatusPass)
	})

	// 3. Malformed XML produces FAIL
	t.Run("Malformed XML produces FAIL", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:malxml"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found)
			VALUES (?, 'COMPLETED', 1, 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at)
			VALUES (?, 3, 'https://example.com/bad.xml', 'robots_txt', 0, 200, 200, 200, 1, 'unknown', 'xml_error', 'syntax error', 0, '2026-10-01T00:00:00Z')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
			VALUES (?, 3, 'robots_txt', 0)`, runID)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sm:%s:3", auditRunID)
		assertResultIntegrity(t, results[0], "AR-DISC-003", expectedSubj, audit.StatusFail)
	})

	// 4. No retrieved body produces NOT_APPLICABLE
	t.Run("No retrieved body produces NOT_APPLICABLE", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:nobody"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found)
			VALUES (?, 'COMPLETED', 1, 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count, fetched_at)
			VALUES (?, 4, 'https://example.com/notfound.xml', 'common_path', 0, 404, 404, 404, 1, 'unknown', 'not_attempted', 0, '2026-10-01T00:00:00Z')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
			VALUES (?, 4, 'common_path', 0)`, runID)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sm:%s:4", auditRunID)
		assertResultIntegrity(t, results[0], "AR-DISC-003", expectedSubj, audit.StatusNotApplicable)
	})

	// 5. Parser unavailable produces UNKNOWN
	t.Run("Parser unavailable produces UNKNOWN", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:unavail"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found)
			VALUES (?, 'COMPLETED', 1, 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at)
			VALUES (?, 5, 'https://example.com/corrupt.gz', 'common_path', 0, 200, 200, 200, 1, 'unknown', 'unavailable', 'gzip read error', 0, '2026-10-01T00:00:00Z')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
			VALUES (?, 5, 'common_path', 0)`, runID)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		expectedSubj := fmt.Sprintf("sm:%s:5", auditRunID)
		assertResultIntegrity(t, results[0], "AR-DISC-003", expectedSubj, audit.StatusUnknown)
	})

	// 6. Multiple documents remain isolated
	t.Run("Multiple documents remain isolated", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:multi"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupRun(t, db, runID, "https://example.com/")

		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found)
			VALUES (?, 'COMPLETED', 3, 1)`, runID)
		// doc 1: 200 URLSET -> PASS
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count, fetched_at)
			VALUES (?, 1, 'https://example.com/sm1.xml', 'robots_txt', 0, 200, 200, 200, 1, 'urlset', 'parsed', 1, '2026-10-01T00:00:00Z')`, runID)
		// doc 2: 404 -> NOT_APPLICABLE
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count, fetched_at)
			VALUES (?, 2, 'https://example.com/sm2.xml', 'common_path', 0, 404, 404, 404, 1, 'unknown', 'not_attempted', 0, '2026-10-01T00:00:00Z')`, runID)
		// doc 3: xml_error -> FAIL
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at)
			VALUES (?, 3, 'https://example.com/sm3.xml', 'common_path', 0, 200, 200, 200, 1, 'unknown', 'xml_error', 'syntax error', 0, '2026-10-01T00:00:00Z')`, runID)

		for i := 1; i <= 3; i++ {
			_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
				VALUES (?, ?, 'robots_txt', 0)`, runID, i)
		}

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 results for 3 documents, got %d", len(results))
		}

		resBySubj := make(map[string]audit.RuleResult)
		for _, r := range results {
			resBySubj[r.SubjectRef] = r
		}

		subj1 := fmt.Sprintf("sm:%s:1", auditRunID)
		subj2 := fmt.Sprintf("sm:%s:2", auditRunID)
		subj3 := fmt.Sprintf("sm:%s:3", auditRunID)

		if resBySubj[subj1].Status != audit.StatusPass {
			t.Errorf("doc 1 status want PASS, got %s", resBySubj[subj1].Status)
		}
		if resBySubj[subj2].Status != audit.StatusNotApplicable {
			t.Errorf("doc 2 status want NOT_APPLICABLE, got %s", resBySubj[subj2].Status)
		}
		if resBySubj[subj3].Status != audit.StatusFail {
			t.Errorf("doc 3 status want FAIL, got %s", resBySubj[subj3].Status)
		}
	})

	// 7. EvidenceRefs trace back to the correct sitemap records
	t.Run("EvidenceRefs trace back to the correct sitemap records", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:trace"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupRun(t, db, runID, "https://example.com/")

		smURL := "https://example.com/sitemap_traced.xml"
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found)
			VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count, fetched_at)
			VALUES (?, 10, ?, 'robots_txt', 0, 200, 200, 200, 1, 'urlset', 'parsed', 1, '2026-10-01T00:00:00Z')`, runID, smURL)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
			VALUES (?, 10, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc)
			VALUES (?, 10, 0, 1, 'https://example.com/page1')`, runID)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		results, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-DISC-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}

		rr := results[0]
		expectedSubj := fmt.Sprintf("sm:%s:10", auditRunID)
		if rr.SubjectRef != expectedSubj {
			t.Errorf("subject ref want %s, got %s", expectedSubj, rr.SubjectRef)
		}

		fieldsFound := make(map[string]string)
		for _, ref := range rr.EvidenceRefs {
			fieldsFound[ref.Field] = ref.ObservedValue
		}

		if fieldsFound["sitemap_url"] != smURL {
			t.Errorf("traced sitemap_url want %q, got %q", smURL, fieldsFound["sitemap_url"])
		}
		if fieldsFound["final_http_status"] != "200" {
			t.Errorf("traced final_http_status want '200', got %q", fieldsFound["final_http_status"])
		}
		if fieldsFound["sitemap_parse_status"] != "parsed" {
			t.Errorf("traced sitemap_parse_status want 'parsed', got %q", fieldsFound["sitemap_parse_status"])
		}
		if fieldsFound["sitemap_document_type"] != "URLSET" {
			t.Errorf("traced sitemap_document_type want 'URLSET', got %q", fieldsFound["sitemap_document_type"])
		}
	})

	// 8. No unrelated rule verdict changes
	t.Run("No unrelated rule verdict changes", func(t *testing.T) {
		db := newIntegrationDB(t)
		runID := "run:int:unrelated"
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		setupRun(t, db, runID, "https://example.com/")

		// Insert 1 page and 1 link
		_, _ = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES (?, 1, 'https://example.com/')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, status, kind, crawled_at, data)
			VALUES (?, 1, 'https://example.com/', 200, 'html', '2026-10-01T00:00:00Z', '{}')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor)
			VALUES (?, 1, 1, 0, 1, 1, 'Self')`, runID)

		// Insert valid sitemap
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found)
			VALUES (?, 'COMPLETED', 1, 1)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, discovery_source, parent_id,
			initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count, fetched_at)
			VALUES (?, 1, 'https://example.com/sitemap.xml', 'robots_txt', 0, 200, 200, 200, 1, 'urlset', 'parsed', 1, '2026-10-01T00:00:00Z')`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
			VALUES (?, 1, 'robots_txt', 0)`, runID)
		_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc)
			VALUES (?, 1, 0, 1, 'https://example.com/')`, runID)

		buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		// Evaluate AR-ACC-004 (should be PASS for 200 page)
		accResults, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-ACC-004")
		if err != nil {
			t.Fatalf("EvaluateRule AR-ACC-004 failed: %v", err)
		}
		if len(accResults) != 1 || accResults[0].Status != audit.StatusPass {
			t.Errorf("expected AR-ACC-004 to remain PASS, got %v", accResults)
		}

		// Evaluate AR-LINK-002 (should be PASS for 200 self link)
		linkResults, err := eng.EvaluateRule(context.Background(), buildRes.EvidenceSnapshot, "AR-LINK-002")
		if err != nil {
			t.Fatalf("EvaluateRule AR-LINK-002 failed: %v", err)
		}
		if len(linkResults) != 1 || linkResults[0].Status != audit.StatusPass {
			t.Errorf("expected AR-LINK-002 to remain PASS, got %v", linkResults)
		}
	})
}
