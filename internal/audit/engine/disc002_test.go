package engine_test

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

type disc002DocFixture struct {
	subjectRef    string
	url           string
	finalStatus   string
	initialStatus string
	redirectHops  string
	finalURL      string
	fetchError    string
	fetchComplete string
	omitFields    []string
	customObs     []audit.NormalizedObservation
}

func newDISC002Snapshot(fixtures []disc002DocFixture, runID, snapID string) *audit.EvidenceSnapshot {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
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

		if !omitted["sitemap_redirect_hops"] && f.redirectHops != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "sitemap_redirect_hops",
				Value:              f.redirectHops,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}

		if !omitted["sitemap_final_url"] && f.finalURL != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "sitemap_final_url",
				Value:              f.finalURL,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_sitemaps:test"},
				ObservedAt:         now,
			})
		}

		if !omitted["sitemap_fetch_error"] && f.fetchError != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         audit.AuditRunID(runID),
				SnapshotID:         audit.SnapshotID(snapID),
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         f.subjectRef,
				Field:              "sitemap_fetch_error",
				Value:              f.fetchError,
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

		obs = append(obs, f.customObs...)
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:               audit.SnapshotID(snapID),
		AuditRunID:               audit.AuditRunID(runID),
		CreatedAt:                now,
		FrozenAt:                 &now,
		SnapshotStatus:           audit.SnapshotFrozen,
		NormalizationVersion:     "v1.9.0",
		CrawlComplete:            true,
		SitemapDiscoveryComplete: true,
		NormalizedObservations:   obs,
	}
}

// ============================================================================
// Section 10: Mandatory Unit Tests (Scenarios 1 - 36)
// ============================================================================

func TestEngine_AR_DISC_002_Unit(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}
	fixedTime := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	eng.SetClock(func() time.Time { return fixedTime })

	runID := "run:disc002:test"
	snapID := "snap:disc002:test"
	smRef := fmt.Sprintf("sm:%s:1", runID)
	ctx := context.Background()

	// 1. Direct HTTP 200 -> PASS
	t.Run("1. direct HTTP 200 -> PASS", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "200",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v", res)
		}
		if res[0].ObservedSummary != "Sitemap document returned terminal HTTP 200 without redirects." {
			t.Errorf("unexpected summary: %s", res[0].ObservedSummary)
		}
	})

	// 2. HTTP 301 -> terminal 200 -> PASS
	t.Run("2. HTTP 301 -> terminal 200 -> PASS", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "http://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "301",
				redirectHops:  "1",
				finalURL:      "https://example.com/sitemap.xml",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v", res)
		}
		if res[0].ObservedSummary != "Sitemap document returned terminal HTTP 200 after redirecting from initial HTTP 301." {
			t.Errorf("unexpected summary: %s", res[0].ObservedSummary)
		}
	})

	// 3. HTTP 302 -> terminal 200 -> PASS
	t.Run("3. HTTP 302 -> terminal 200 -> PASS", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "302",
				redirectHops:  "1",
				finalURL:      "https://example.com/sitemap_index.xml",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v", res)
		}
		if res[0].ObservedSummary != "Sitemap document returned terminal HTTP 200 after redirecting from initial HTTP 302." {
			t.Errorf("unexpected summary: %s", res[0].ObservedSummary)
		}
	})

	// 4. Multi-hop redirect -> terminal 200 -> PASS
	t.Run("4. multi-hop redirect -> terminal 200 -> PASS", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "http://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "301",
				redirectHops:  "3",
				finalURL:      "https://example.com/final_sitemap.xml",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v", res)
		}
		if res[0].ObservedSummary != "Sitemap document returned terminal HTTP 200 after redirecting from initial HTTP 301." {
			t.Errorf("unexpected summary: %s", res[0].ObservedSummary)
		}
	})

	// 5. HTTP 301 -> terminal 404 -> FAIL
	t.Run("5. HTTP 301 -> terminal 404 -> FAIL", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "http://example.com/sitemap.xml",
				finalStatus:   "404",
				initialStatus: "301",
				redirectHops:  "1",
				finalURL:      "https://example.com/missing_sitemap.xml",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
		if res[0].ObservedSummary != "Sitemap document returned terminal HTTP 404 after redirecting from initial HTTP 301." {
			t.Errorf("unexpected summary: %s", res[0].ObservedSummary)
		}
	})

	// 6. Direct HTTP 404 -> FAIL
	t.Run("6. direct HTTP 404 -> FAIL", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "404",
				initialStatus: "404",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
		if res[0].ObservedSummary != "Sitemap document returned terminal HTTP 404." {
			t.Errorf("unexpected summary: %s", res[0].ObservedSummary)
		}
	})

	// 7. Direct HTTP 500 -> FAIL
	t.Run("7. direct HTTP 500 -> FAIL", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "500",
				initialStatus: "500",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 8. HTTP 403 -> FAIL
	t.Run("8. HTTP 403 -> FAIL", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "403",
				initialStatus: "403",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 9. HTTP 429 -> FAIL
	t.Run("9. HTTP 429 -> FAIL", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "429",
				initialStatus: "429",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 10. HTTP 503 -> FAIL
	t.Run("10. HTTP 503 -> FAIL", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "503",
				initialStatus: "503",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 11. HTTP 201 -> FAIL (non-200)
	t.Run("11. HTTP 201 -> FAIL", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "201",
				initialStatus: "201",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 12. HTTP 204 -> FAIL (non-200)
	t.Run("12. HTTP 204 -> FAIL", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "204",
				initialStatus: "204",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 13. HTTP 304 terminal response -> FAIL when trustworthy
	t.Run("13. HTTP 304 terminal response -> FAIL when trustworthy", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "304",
				initialStatus: "304",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 14. DNS failure -> UNKNOWN
	t.Run("14. DNS failure -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://invalid.example.test/sitemap.xml",
				finalStatus:   "0",
				initialStatus: "0",
				fetchError:    "dial tcp: lookup invalid.example.test: no such host",
				fetchComplete: "false",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
		if !strings.Contains(res[0].ObservedSummary, "no such host") {
			t.Errorf("expected summary to contain error, got %s", res[0].ObservedSummary)
		}
	})

	// 15. Timeout -> UNKNOWN
	t.Run("15. timeout -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "0",
				initialStatus: "0",
				fetchError:    "context deadline exceeded (Client.Timeout exceeded)",
				fetchComplete: "false",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 16. TLS failure -> UNKNOWN
	t.Run("16. TLS failure -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "0",
				initialStatus: "0",
				fetchError:    "remote error: tls: handshake failure",
				fetchComplete: "false",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 17. Redirect loop -> UNKNOWN
	t.Run("17. redirect loop -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "0",
				initialStatus: "301",
				redirectHops:  "10",
				finalURL:      "https://example.com/sitemap.xml",
				fetchError:    "stopped after 10 redirects",
				fetchComplete: "false",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 18. Redirect limit exceeded -> UNKNOWN
	t.Run("18. redirect limit exceeded -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "0",
				initialStatus: "302",
				redirectHops:  "10",
				fetchError:    "stopped after 10 redirects",
				fetchComplete: "false",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 19. Missing final status -> UNKNOWN
	t.Run("19. missing final status -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				omitFields:    []string{"final_http_status"},
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 20. Final status 0 -> UNKNOWN
	t.Run("20. final status 0 -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "0",
				fetchComplete: "false",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 21. Malformed final status -> UNKNOWN
	t.Run("21. malformed final status -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "not_a_number",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 22. Conflicting final status observations -> UNKNOWN
	t.Run("22. conflicting final status observations -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				fetchComplete: "true",
				customObs: []audit.NormalizedObservation{
					{
						ObservationID:      "obs:conf:status",
						AuditRunID:         audit.AuditRunID(runID),
						SnapshotID:         audit.SnapshotID(snapID),
						SubjectType:        audit.SubjectSitemap,
						SubjectRef:         smRef,
						Field:              "final_http_status",
						Value:              "404",
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"test"},
						ObservedAt:         time.Now().UTC(),
					},
				},
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 23. Missing sitemap URL -> UNKNOWN
	t.Run("23. missing sitemap URL -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				omitFields:    []string{"sitemap_url"},
				finalStatus:   "200",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 24. Conflicting sitemap URL -> UNKNOWN
	t.Run("24. conflicting sitemap URL -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap1.xml",
				finalStatus:   "200",
				fetchComplete: "true",
				customObs: []audit.NormalizedObservation{
					{
						ObservationID:      "obs:conf:url",
						AuditRunID:         audit.AuditRunID(runID),
						SnapshotID:         audit.SnapshotID(snapID),
						SubjectType:        audit.SubjectSitemap,
						SubjectRef:         smRef,
						Field:              "sitemap_url",
						Value:              "https://example.com/sitemap2.xml",
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"test"},
						ObservedAt:         time.Now().UTC(),
					},
				},
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 25. Invalid sitemap subject run identity -> UNKNOWN
	t.Run("25. invalid sitemap subject run identity -> UNKNOWN", func(t *testing.T) {
		foreignRef := "sm:foreign_run:1"
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    foreignRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 26. Missing initial status with trustworthy terminal status -> proven contract behavior
	t.Run("26. missing initial status with trustworthy terminal status -> PASS", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				omitFields:    []string{"initial_http_status"},
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v", res)
		}
	})

	// 27. Missing fetch-complete evidence -> UNKNOWN
	t.Run("27. missing fetch-complete evidence -> UNKNOWN", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				omitFields:    []string{"sitemap_fetch_complete"},
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 28. Fetch-complete/status contradiction -> UNKNOWN
	t.Run("28. fetch-complete/status contradiction -> UNKNOWN", func(t *testing.T) {
		// HTTP 200 reported but fetch_complete is false
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				fetchComplete: "false",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}

		// Status 0 reported but fetch_complete is true
		snap2 := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res2, err := eng.EvaluateRule(ctx, snap2, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res2) != 1 || res2[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res2)
		}
	})

	// 29. One sitemap document -> one RuleResult
	t.Run("29. one sitemap document -> one RuleResult", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected exactly 1 result, got %d", len(res))
		}
	})

	// 30. Two sitemap documents -> separate RuleResults
	t.Run("30. two sitemap documents -> separate RuleResults", func(t *testing.T) {
		smRef2 := fmt.Sprintf("sm:%s:2", runID)
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap1.xml",
				finalStatus:   "200",
				fetchComplete: "true",
			},
			{
				subjectRef:    smRef2,
				url:           "https://example.com/sitemap2.xml",
				finalStatus:   "404",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected exactly 2 results, got %d", len(res))
		}
		if res[0].SubjectRef != smRef || res[0].Status != audit.StatusPass {
			t.Errorf("expected res[0] smRef PASS, got %v", res[0])
		}
		if res[1].SubjectRef != smRef2 || res[1].Status != audit.StatusFail {
			t.Errorf("expected res[1] smRef2 FAIL, got %v", res[1])
		}
	})

	// 31. Multiple subjects -> deterministic ordering
	t.Run("31. multiple subjects -> deterministic ordering", func(t *testing.T) {
		smRef1 := fmt.Sprintf("sm:%s:1", runID)
		smRef2 := fmt.Sprintf("sm:%s:2", runID)
		smRef3 := fmt.Sprintf("sm:%s:3", runID)
		// Insert in reverse order
		snap := newDISC002Snapshot([]disc002DocFixture{
			{subjectRef: smRef3, url: "https://example.com/sitemap3.xml", finalStatus: "200", fetchComplete: "true"},
			{subjectRef: smRef1, url: "https://example.com/sitemap1.xml", finalStatus: "200", fetchComplete: "true"},
			{subjectRef: smRef2, url: "https://example.com/sitemap2.xml", finalStatus: "200", fetchComplete: "true"},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 3 {
			t.Fatalf("expected 3 results, got %d", len(res))
		}
		if res[0].SubjectRef != smRef1 || res[1].SubjectRef != smRef2 || res[2].SubjectRef != smRef3 {
			t.Errorf("results not deterministically sorted: %s, %s, %s", res[0].SubjectRef, res[1].SubjectRef, res[2].SubjectRef)
		}
	})

	// 32. Repeated evaluation -> identical results
	t.Run("32. repeated evaluation -> identical results", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "http://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "301",
				redirectHops:  "1",
				finalURL:      "https://example.com/sitemap.xml",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res1, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval 1 failed: %v", err)
		}
		res2, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval 2 failed: %v", err)
		}
		if !reflect.DeepEqual(res1, res2) {
			t.Errorf("repeated evaluation yielded different results: res1=%+v, res2=%+v", res1, res2)
		}
	})

	// 33. Primary EvidenceRefs resolve correctly
	t.Run("33. primary EvidenceRefs resolve correctly", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		var foundPrimary bool
		for _, ref := range res[0].EvidenceRefs {
			if ref.Field == "final_http_status" && ref.Role == audit.EvidenceRolePrimary && ref.ObservedValue == "200" {
				foundPrimary = true
				break
			}
		}
		if !foundPrimary {
			t.Errorf("expected primary final_http_status EvidenceRef in %+v", res[0].EvidenceRefs)
		}
	})

	// 34. Redirect context EvidenceRefs resolve correctly
	t.Run("34. redirect context EvidenceRefs resolve correctly", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "http://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "301",
				redirectHops:  "1",
				finalURL:      "https://example.com/sitemap.xml",
				fetchComplete: "true",
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		var hasInitStatus, hasHops, hasFinalURL bool
		for _, ref := range res[0].EvidenceRefs {
			if ref.Field == "initial_http_status" && ref.Role == audit.EvidenceRoleContext && ref.ObservedValue == "301" {
				hasInitStatus = true
			}
			if ref.Field == "sitemap_redirect_hops" && ref.Role == audit.EvidenceRoleContext && ref.ObservedValue == "1" {
				hasHops = true
			}
			if ref.Field == "sitemap_final_url" && ref.Role == audit.EvidenceRoleContext && ref.ObservedValue == "https://example.com/sitemap.xml" {
				hasFinalURL = true
			}
		}
		if !hasInitStatus || !hasHops || !hasFinalURL {
			t.Errorf("missing redirect context refs: init=%t, hops=%t, finalURL=%t", hasInitStatus, hasHops, hasFinalURL)
		}
	})

	// 35. No WARNING/MANUAL_REVIEW/NOT_APPLICABLE emitted
	t.Run("35. no WARNING/MANUAL_REVIEW/NOT_APPLICABLE emitted", func(t *testing.T) {
		testCases := []struct {
			status string
			fc     string
		}{
			{"200", "true"},
			{"404", "true"},
			{"500", "true"},
			{"0", "false"},
			{"not_number", "true"},
		}
		for _, tc := range testCases {
			snap := newDISC002Snapshot([]disc002DocFixture{
				{
					subjectRef:    smRef,
					url:           "https://example.com/sitemap.xml",
					finalStatus:   tc.status,
					fetchComplete: tc.fc,
				},
			}, runID, snapID)
			res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			for _, r := range res {
				if r.Status == audit.StatusWarning || r.Status == audit.StatusManualReview || r.Status == audit.StatusNotApplicable {
					t.Errorf("forbidden status emitted: %s", r.Status)
				}
			}
		}
	})

	// 36. Prior five sitemap evaluators remain unaffected
	t.Run("36. prior five sitemap evaluators remain unaffected", func(t *testing.T) {
		snap := newDISC002Snapshot([]disc002DocFixture{
			{
				subjectRef:    smRef,
				url:           "https://example.com/sitemap.xml",
				finalStatus:   "200",
				initialStatus: "200",
				redirectHops:  "0",
				fetchComplete: "true",
			},
		}, runID, snapID)
		priorRules := []string{"AR-DISC-001", "AR-DISC-003", "AR-DISC-004", "AR-DISC-005", "AR-DISC-006"}
		for _, rID := range priorRules {
			_, err := eng.EvaluateRule(ctx, snap, rID)
			if err != nil {
				t.Errorf("prior evaluator %s failed: %v", rID, err)
			}
		}
	})
}

// ============================================================================
// Section 11: Hermetic SQLite Integration Tests (Scenarios 1 - 13)
// SQLite -> adapter.Build() -> EvidenceSnapshot -> engine.EvaluateRule()
// ============================================================================

func TestAdapter_Engine_AR_DISC_002_Integration(t *testing.T) {
	ctx := context.Background()
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

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

	type sitemapRow struct {
		id           int
		url          string
		initStatus   int
		finalStatus  int
		redirectHops int
		redirectTo   string
		fetchError   string
		fetchComp    int
	}

	setupIntegrationRun := func(t *testing.T, runID string, discStatus string, sitemaps []sitemapRow) (*sql.DB, *audit.EvidenceSnapshot) {
		t.Helper()
		db := newIntegrationDB(t)
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"
		now := time.Now().UTC().Format(time.RFC3339)

		// 1. Insert sitecrawl_runs
		_, err := db.Exec(`INSERT INTO sitecrawl_runs(
			id, state, stop_reason, seed_url, host, options, started_at, finished_at, found, crawled, total
		) VALUES (?, 'completed', '', ?, 'example.com', '{}', ?, ?, 1, 1, 1)`,
			runID, seed, now, now)
		if err != nil {
			t.Fatalf("insert run failed: %v", err)
		}

		// 2. Insert sitecrawl_sitemap_discovery
		if discStatus != "" {
			_, err = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at)
				VALUES(?, ?, ?, ?, 0, 0, 0, 0, ?)`, runID, discStatus, len(sitemaps), len(sitemaps), now)
			if err != nil {
				t.Fatalf("insert discovery failed: %v", err)
			}
		}

		// 3. Insert sitemaps
		for _, sm := range sitemaps {
			_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(
				run_id, id, url, initial_status, final_status, status, fetch_complete,
				redirect_hops, redirect_to, fetch_error, doc_type, parse_status, entry_count, fetched_at
			) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'urlset', 'parsed', 1, ?)`,
				runID, sm.id, sm.url, sm.initStatus, sm.finalStatus, sm.finalStatus, sm.fetchComp,
				sm.redirectHops, sm.redirectTo, sm.fetchError, now)
			if err != nil {
				t.Fatalf("insert sitemap failed: %v", err)
			}
		}

		// 4. Run adapter.Build
		res, err := adapter.Build(ctx, db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		return db, res.EvidenceSnapshot
	}

	// 1. Direct sitemap HTTP 200 -> PASS
	t.Run("1. direct sitemap HTTP 200 -> PASS", func(t *testing.T) {
		runID := "int:disc002:1_200"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://example.com/sitemap.xml", initStatus: 200, finalStatus: 200, redirectHops: 0, fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v", res)
		}
	})

	// 2. Sitemap HTTP 301 -> terminal 200 -> PASS
	t.Run("2. sitemap HTTP 301 -> terminal 200 -> PASS", func(t *testing.T) {
		runID := "int:disc002:2_301_200"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "http://example.com/sitemap.xml", initStatus: 301, finalStatus: 200, redirectHops: 1, redirectTo: "https://example.com/sitemap.xml", fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v", res)
		}
	})

	// 3. Sitemap HTTP 302 -> terminal 200 -> PASS
	t.Run("3. sitemap HTTP 302 -> terminal 200 -> PASS", func(t *testing.T) {
		runID := "int:disc002:3_302_200"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://example.com/sitemap.xml", initStatus: 302, finalStatus: 200, redirectHops: 1, redirectTo: "https://example.com/sitemap_index.xml", fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v", res)
		}
	})

	// 4. Sitemap HTTP 301 -> terminal 404 -> FAIL
	t.Run("4. sitemap HTTP 301 -> terminal 404 -> FAIL", func(t *testing.T) {
		runID := "int:disc002:4_301_404"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "http://example.com/sitemap.xml", initStatus: 301, finalStatus: 404, redirectHops: 1, redirectTo: "https://example.com/missing.xml", fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 5. Direct HTTP 404 -> FAIL
	t.Run("5. direct HTTP 404 -> FAIL", func(t *testing.T) {
		runID := "int:disc002:5_404"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://example.com/sitemap.xml", initStatus: 404, finalStatus: 404, redirectHops: 0, fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 6. Direct HTTP 500 -> FAIL
	t.Run("6. direct HTTP 500 -> FAIL", func(t *testing.T) {
		runID := "int:disc002:6_500"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://example.com/sitemap.xml", initStatus: 500, finalStatus: 500, redirectHops: 0, fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v", res)
		}
	})

	// 7. Network failure with no terminal response -> UNKNOWN
	t.Run("7. network failure with no terminal response -> UNKNOWN", func(t *testing.T) {
		runID := "int:disc002:7_neterr"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://invalid.example.test/sitemap.xml", initStatus: 0, finalStatus: 0, redirectHops: 0, fetchError: "dial tcp: lookup failed", fetchComp: 0},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 8. Redirect exhaustion -> UNKNOWN
	t.Run("8. redirect exhaustion -> UNKNOWN", func(t *testing.T) {
		runID := "int:disc002:8_redirex"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://example.com/sitemap.xml", initStatus: 301, finalStatus: 0, redirectHops: 10, redirectTo: "https://example.com/loop.xml", fetchError: "stopped after 10 redirects", fetchComp: 0},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 9. Missing/unavailable terminal HTTP status -> UNKNOWN
	t.Run("9. missing/unavailable terminal HTTP status -> UNKNOWN", func(t *testing.T) {
		runID := "int:disc002:9_noterm"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://example.com/sitemap.xml", initStatus: 0, finalStatus: 0, redirectHops: 0, fetchComp: 0},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
	})

	// 10. Multiple sitemap documents -> separate results
	t.Run("10. multiple sitemap documents -> separate results", func(t *testing.T) {
		runID := "int:disc002:10_multi"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://example.com/sitemap1.xml", initStatus: 200, finalStatus: 200, redirectHops: 0, fetchComp: 1},
			{id: 2, url: "https://example.com/sitemap2.xml", initStatus: 404, finalStatus: 404, redirectHops: 0, fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res))
		}
		if res[0].Status != audit.StatusPass || res[1].Status != audit.StatusFail {
			t.Errorf("expected [PASS, FAIL], got [%s, %s]", res[0].Status, res[1].Status)
		}
	})

	// 11. EvidenceRefs resolve to actual normalized observations
	t.Run("11. EvidenceRefs resolve to actual normalized observations", func(t *testing.T) {
		runID := "int:disc002:11_resolve"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "http://example.com/sitemap.xml", initStatus: 301, finalStatus: 200, redirectHops: 1, redirectTo: "https://example.com/sitemap.xml", fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		obsMap := make(map[string]audit.NormalizedObservation)
		for _, o := range snap.NormalizedObservations {
			obsMap[string(o.ObservationID)] = o
		}
		for _, ref := range res[0].EvidenceRefs {
			if ref.EvidenceType == "NORMALIZED_OBSERVATION" {
				o, exists := obsMap[ref.EvidenceRef]
				if !exists {
					t.Errorf("EvidenceRef %s does not exist on snapshot", ref.EvidenceRef)
				} else if o.Field != ref.Field || o.Value != ref.ObservedValue {
					t.Errorf("field/value mismatch: ref=%+v, obs=%+v", ref, o)
				}
			}
		}
	})

	// 12. No initial/terminal status conflation
	t.Run("12. no initial/terminal status conflation", func(t *testing.T) {
		runID := "int:disc002:12_noconflate"
		// 301 initial -> 200 terminal MUST be PASS, not FAIL or WARNING
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "http://example.com/sitemap.xml", initStatus: 301, finalStatus: 200, redirectHops: 1, redirectTo: "https://example.com/sitemap.xml", fetchComp: 1},
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-002")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("initial 301 with terminal 200 MUST evaluate to PASS, got %v", res)
		}
	})

	// 13. All 19 implemented evaluators execute without regression
	t.Run("13. all 19 implemented evaluators execute without regression", func(t *testing.T) {
		runID := "int:disc002:13_all19"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", []sitemapRow{
			{id: 1, url: "https://example.com/sitemap.xml", initStatus: 200, finalStatus: 200, redirectHops: 0, fetchComp: 1},
		})
		allIDs := eng.ImplementedRuleIDs()
		if len(allIDs) != 19 {
			t.Fatalf("expected 19 implemented rules, got %d", len(allIDs))
		}
		for _, rID := range allIDs {
			results, err := eng.EvaluateRule(ctx, snap, rID)
			if err != nil {
				t.Fatalf("rule %s failed: %v", rID, err)
			}
			// AR-DISC-001, AR-DISC-002, AR-DISC-003, AR-DISC-004 evaluate on sitemap/discovery context
			if rID == "AR-DISC-001" || rID == "AR-DISC-002" || rID == "AR-DISC-003" {
				if len(results) == 0 {
					t.Errorf("expected non-empty results for %s", rID)
				}
			}
		}
	})
}
