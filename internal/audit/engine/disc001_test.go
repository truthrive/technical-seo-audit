package engine_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

type disc001SiteFixture struct {
	subjectRef         string // defaults to "site" if empty
	sitemapsFoundCount string // e.g. "1", "0"
	discoveryComplete  string // e.g. "true", "false"
	discoveryStatus    string // e.g. "COMPLETED", "NOT_ATTEMPTED", "DISABLED"
	depthCapped        string // e.g. "false", "true"
	urlsCapped         string // e.g. "false", "true"
	byteCapped         string // e.g. "false", "true"
	snapshotComplete   *bool  // if nil, defaults to (discoveryComplete == "true")
	omitFields         []string
	customObs          []audit.NormalizedObservation
	sitemapDocs        []string // optional SubjectSitemap refs to populate sitemap documents
}

func newDISC001Snapshot(fix disc001SiteFixture, runID, snapID string) *audit.EvidenceSnapshot {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	ref := fix.subjectRef
	if ref == "" {
		ref = "site"
	}

	omitted := make(map[string]bool)
	for _, o := range fix.omitFields {
		omitted[o] = true
	}

	var obs []audit.NormalizedObservation
	obsCounter := 0
	nextObsID := func() audit.ObservationID {
		obsCounter++
		return audit.ObservationID(fmt.Sprintf("obs:%s:%d", snapID, obsCounter))
	}

	if !omitted["sitemaps_found_count"] && fix.sitemapsFoundCount != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        audit.SubjectSite,
			SubjectRef:         ref,
			Field:              "sitemaps_found_count",
			Value:              fix.sitemapsFoundCount,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
			ObservedAt:         now,
		})
	}

	if !omitted["sitemap_discovery_complete"] && fix.discoveryComplete != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        audit.SubjectSite,
			SubjectRef:         ref,
			Field:              "sitemap_discovery_complete",
			Value:              fix.discoveryComplete,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
			ObservedAt:         now,
		})
	}

	if !omitted["sitemap_discovery_status"] && fix.discoveryStatus != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        audit.SubjectSite,
			SubjectRef:         ref,
			Field:              "sitemap_discovery_status",
			Value:              fix.discoveryStatus,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
			ObservedAt:         now,
		})
	}

	if !omitted["sitemap_depth_capped"] && fix.depthCapped != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        audit.SubjectSite,
			SubjectRef:         ref,
			Field:              "sitemap_depth_capped",
			Value:              fix.depthCapped,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
			ObservedAt:         now,
		})
	}

	if !omitted["sitemap_urls_capped"] && fix.urlsCapped != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        audit.SubjectSite,
			SubjectRef:         ref,
			Field:              "sitemap_urls_capped",
			Value:              fix.urlsCapped,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
			ObservedAt:         now,
		})
	}

	if !omitted["sitemap_byte_capped"] && fix.byteCapped != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        audit.SubjectSite,
			SubjectRef:         ref,
			Field:              "sitemap_byte_capped",
			Value:              fix.byteCapped,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
			ObservedAt:         now,
		})
	}

	// Optional SubjectSitemap context
	for _, smRef := range fix.sitemapDocs {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        audit.SubjectSitemap,
			SubjectRef:         smRef,
			Field:              "discovery_sources",
			Value:              "robots_txt",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_sitemap_sources:test"},
			ObservedAt:         now,
		})
	}

	obs = append(obs, fix.customObs...)

	snapComplete := (fix.discoveryComplete == "true")
	if fix.snapshotComplete != nil {
		snapComplete = *fix.snapshotComplete
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:               audit.SnapshotID(snapID),
		AuditRunID:               audit.AuditRunID(runID),
		CreatedAt:                now,
		FrozenAt:                 &now,
		SnapshotStatus:           audit.SnapshotFrozen,
		NormalizationVersion:     "v1.9.0",
		CrawlComplete:            true,
		SitemapDiscoveryComplete: snapComplete,
		NormalizedObservations:   obs,
	}
}

func makeSitePolicy(runID audit.AuditRunID, polID, targetRef, value string) audit.ProjectPolicyAssignment {
	return audit.ProjectPolicyAssignment{
		PolicyAssignmentID: audit.PolicyAssignmentID(polID),
		AuditRunID:         runID,
		PolicyKey:          audit.PolicyKeySitemapExpected,
		PolicyValue:        value,
		Scope:              audit.PolicyScopeSite,
		TargetRef:          targetRef,
		Source:             audit.PolicyProvenanceUserInput,
		SuppliedAt:         time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC),
	}
}

// ============================================================================
// Mandatory Unit Tests (Section 11)
// ============================================================================

func TestEngine_AR_DISC_001_Unit(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := "run:disc001:test"
	snapID := "snap:disc001:test"
	ctx := context.Background()

	// ------------------------------------------------------------------------
	// A. Policy outcomes (Scenarios 1 - 8)
	// ------------------------------------------------------------------------
	t.Run("A. Policy outcomes", func(t *testing.T) {
		tests := []struct {
			name           string
			foundCount     string
			policyVal      string // "", "true", "false", "unspecified"
			expectedStatus audit.RuleResultStatus
		}{
			// 1. Complete + found count 1 + policy true -> PASS
			{"1. complete + count 1 + policy true", "1", "true", audit.StatusPass},
			// 2. Complete + found count 1 + policy false -> PASS
			{"2. complete + count 1 + policy false", "1", "false", audit.StatusPass},
			// 3. Complete + found count 1 + policy unspecified -> PASS
			{"3. complete + count 1 + policy unspecified", "1", "unspecified", audit.StatusPass},
			// 4. Complete + found count 1 + policy missing -> PASS
			{"4. complete + count 1 + policy missing", "1", "", audit.StatusPass},
			// 5. Complete + count 0 + policy true -> WARNING
			{"5. complete + count 0 + policy true", "0", "true", audit.StatusWarning},
			// 6. Complete + count 0 + policy false -> NOT_APPLICABLE
			{"6. complete + count 0 + policy false", "0", "false", audit.StatusNotApplicable},
			// 7. Complete + count 0 + policy unspecified -> MANUAL_REVIEW
			{"7. complete + count 0 + policy unspecified", "0", "unspecified", audit.StatusManualReview},
			// 8. Complete + count 0 + policy missing -> MANUAL_REVIEW
			{"8. complete + count 0 + policy missing", "0", "", audit.StatusManualReview},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				snap := newDISC001Snapshot(disc001SiteFixture{
					sitemapsFoundCount: tc.foundCount,
					discoveryComplete:  "true",
					discoveryStatus:    "COMPLETED",
					depthCapped:        "false",
					urlsCapped:         "false",
					byteCapped:         "false",
					sitemapDocs:        []string{"sm:1"},
				}, runID, snapID)

				var polAssignments []audit.ProjectPolicyAssignment
				if tc.policyVal != "" {
					polAssignments = append(polAssignments, makeSitePolicy(audit.AuditRunID(runID), "pol:site:sm_exp", "site", tc.policyVal))
				}
				polIdx, err := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), polAssignments)
				if err != nil {
					t.Fatalf("NewPolicyIndexForRun failed: %v", err)
				}

				res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
					Snapshot: snap,
					Policies: polIdx,
				}, "AR-DISC-001")
				if err != nil {
					t.Fatalf("EvaluateRuleWithContext failed: %v", err)
				}
				if len(res) != 1 {
					t.Fatalf("expected exactly 1 result, got %d", len(res))
				}
				if res[0].Status != tc.expectedStatus {
					t.Errorf("expected status %s, got %s (summary: %q)", tc.expectedStatus, res[0].Status, res[0].ObservedSummary)
				}
			})
		}
	})

	// ------------------------------------------------------------------------
	// B. Policy TargetRef (Scenarios 9 - 14)
	// ------------------------------------------------------------------------
	t.Run("B. Policy TargetRef", func(t *testing.T) {
		snapZero := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			discoveryStatus:    "COMPLETED",
			depthCapped:        "false",
			urlsCapped:         "false",
			byteCapped:         "false",
		}, runID, snapID)

		// 9. Only canonical "site" policy supplied -> correct value used
		t.Run("9. only canonical site policy supplied", func(t *testing.T) {
			polIdx, err := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
				makeSitePolicy(audit.AuditRunID(runID), "pol:site", "site", "true"),
			})
			if err != nil {
				t.Fatalf("policy index build failed: %v", err)
			}
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapZero, Policies: polIdx}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusWarning {
				t.Errorf("expected WARNING, got %v", res)
			}
		})

		// 10. Only origin-based policy supplied -> treated as unspecified (MANUAL_REVIEW)
		t.Run("10. only origin-based policy supplied", func(t *testing.T) {
			polIdx, err := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
				makeSitePolicy(audit.AuditRunID(runID), "pol:origin", "https://example.com", "true"),
			})
			if err != nil {
				t.Fatalf("policy index build failed: %v", err)
			}
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapZero, Policies: polIdx}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusManualReview {
				t.Errorf("expected MANUAL_REVIEW when only origin policy is supplied, got %v", res)
			}
		})

		// 11. Both canonical "site" and origin policy supplied -> canonical "site" governs
		t.Run("11. canonical site policy overrides origin policy", func(t *testing.T) {
			polIdx, err := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
				makeSitePolicy(audit.AuditRunID(runID), "pol:site", "site", "false"),
				makeSitePolicy(audit.AuditRunID(runID), "pol:origin", "https://example.com", "true"),
			})
			if err != nil {
				t.Fatalf("policy index build failed: %v", err)
			}
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapZero, Policies: polIdx}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
				t.Errorf("expected NOT_APPLICABLE from canonical site policy false, got %v", res)
			}
		})

		// 12. Conflicting duplicate exact-key policy assignments -> rejected by PolicyIndex
		t.Run("12. conflicting duplicate exact-key assignments rejected by PolicyIndex", func(t *testing.T) {
			_, err := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
				makeSitePolicy(audit.AuditRunID(runID), "pol:1", "site", "true"),
				makeSitePolicy(audit.AuditRunID(runID), "pol:2", "site", "false"),
			})
			if err == nil {
				t.Fatalf("expected ErrConflictingPolicy, got nil")
			}
			if !errors.Is(err, engine.ErrConflictingPolicy) {
				t.Errorf("expected ErrConflictingPolicy, got %v", err)
			}
		})

		// 13. Foreign AuditRunID policy -> rejected
		t.Run("13. foreign AuditRunID policy rejected", func(t *testing.T) {
			polIdx, err := engine.NewPolicyIndexForRun("run:foreign", []audit.ProjectPolicyAssignment{
				makeSitePolicy("run:foreign", "pol:foreign", "site", "true"),
			})
			if err != nil {
				t.Fatalf("policy index build failed: %v", err)
			}
			_, err = eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapZero, Policies: polIdx}, "AR-DISC-001")
			if err == nil {
				t.Fatalf("expected error for mismatched policy AuditRunID, got nil")
			}
			if !errors.Is(err, engine.ErrPolicyRunMismatch) {
				t.Errorf("expected ErrPolicyRunMismatch, got %v", err)
			}
		})

		// 14. Explicit unspecified distinguished from missing policy
		t.Run("14. explicit unspecified distinguished from missing policy", func(t *testing.T) {
			polIdxUnspec, err := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
				makeSitePolicy(audit.AuditRunID(runID), "pol:unspec", "site", "unspecified"),
			})
			if err != nil {
				t.Fatalf("policy index build failed: %v", err)
			}
			resUnspec, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapZero, Policies: polIdxUnspec}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}

			polIdxMissing := engine.NewEmptyPolicyIndex()
			resMissing, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapZero, Policies: polIdxMissing}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}

			if resUnspec[0].Status != audit.StatusManualReview || resMissing[0].Status != audit.StatusManualReview {
				t.Fatalf("expected both to be MANUAL_REVIEW, got %s and %s", resUnspec[0].Status, resMissing[0].Status)
			}
			// They must be distinguished in ObservedSummary and EvidenceRefs
			if resUnspec[0].ObservedSummary == resMissing[0].ObservedSummary {
				t.Errorf("expected distinct ObservedSummary between explicit unspecified and missing policy")
			}
			hasPolicyRef := false
			for _, ref := range resUnspec[0].EvidenceRefs {
				if ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" {
					hasPolicyRef = true
					break
				}
			}
			if !hasPolicyRef {
				t.Errorf("expected PROJECT_POLICY_ASSIGNMENT EvidenceRef for explicit unspecified policy")
			}
			for _, ref := range resMissing[0].EvidenceRefs {
				if ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" {
					t.Errorf("unexpected PROJECT_POLICY_ASSIGNMENT EvidenceRef for missing policy")
				}
			}
		})
	})

	// ------------------------------------------------------------------------
	// C. Discovery completeness (Scenarios 15 - 24)
	// ------------------------------------------------------------------------
	t.Run("C. Discovery completeness", func(t *testing.T) {
		polTrue, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:1", "site", "true"),
		})
		polFalse, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:2", "site", "false"),
		})

		// 15. Discovery incomplete + policy true -> UNKNOWN
		t.Run("15. discovery incomplete + policy true -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "0",
				discoveryComplete:  "false",
				discoveryStatus:    "ATTEMPTED_INCOMPLETE",
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 16. Discovery incomplete + policy false -> UNKNOWN
		t.Run("16. discovery incomplete + policy false -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "0",
				discoveryComplete:  "false",
				discoveryStatus:    "ATTEMPTED_INCOMPLETE",
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polFalse}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 17. Discovery incomplete + positive sitemap count -> UNKNOWN
		t.Run("17. discovery incomplete + positive count -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "2",
				discoveryComplete:  "false",
				discoveryStatus:    "ATTEMPTED_INCOMPLETE",
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 18. Discovery not attempted -> UNKNOWN
		t.Run("18. discovery not attempted -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "0",
				discoveryComplete:  "false",
				discoveryStatus:    "NOT_ATTEMPTED",
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 19. Discovery disabled -> UNKNOWN
		t.Run("19. discovery disabled -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "0",
				discoveryComplete:  "false",
				discoveryStatus:    "DISABLED",
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 20. Missing completion evidence -> UNKNOWN
		t.Run("20. missing completion evidence -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "1",
				omitFields:         []string{"sitemap_discovery_complete"},
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 21. Malformed completion evidence -> UNKNOWN
		t.Run("21. malformed completion evidence -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "1",
				discoveryComplete:  "not_a_boolean",
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 22. Conflicting completion evidence -> UNKNOWN
		t.Run("22. conflicting completion evidence -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "1",
				discoveryComplete:  "true",
				customObs: []audit.NormalizedObservation{
					{
						ObservationID:      "obs:conf:complete",
						AuditRunID:         audit.AuditRunID(runID),
						SnapshotID:         audit.SnapshotID(snapID),
						SubjectType:        audit.SubjectSite,
						SubjectRef:         "site",
						Field:              "sitemap_discovery_complete",
						Value:              "false",
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"test"},
						ObservedAt:         time.Now().UTC(),
					},
				},
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 23. Snapshot and observation completion contradictions -> UNKNOWN
		t.Run("23. snapshot and observation completion contradiction -> UNKNOWN", func(t *testing.T) {
			snapFalse := false
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "1",
				discoveryComplete:  "true",
				snapshotComplete:   &snapFalse,
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 24. Discovery source preconditions not satisfied / capped -> UNKNOWN
		t.Run("24. discovery capped -> UNKNOWN", func(t *testing.T) {
			for _, capField := range []string{"depth", "urls", "byte"} {
				fix := disc001SiteFixture{
					sitemapsFoundCount: "0",
					discoveryComplete:  "true",
					discoveryStatus:    "COMPLETED",
				}
				switch capField {
				case "depth":
					fix.depthCapped = "true"
				case "urls":
					fix.urlsCapped = "true"
				case "byte":
					fix.byteCapped = "true"
				}
				snap := newDISC001Snapshot(fix, runID, snapID)
				res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
				if err != nil {
					t.Fatalf("eval failed: %v", err)
				}
				if len(res) != 1 || res[0].Status != audit.StatusUnknown {
					t.Errorf("cap %s: expected UNKNOWN, got %v", capField, res)
				}
			}
		})
	})

	// ------------------------------------------------------------------------
	// D. Count integrity (Scenarios 25 - 30)
	// ------------------------------------------------------------------------
	t.Run("D. Count integrity", func(t *testing.T) {
		polTrue, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:1", "site", "true"),
		})

		// 25. Missing count -> UNKNOWN
		t.Run("25. missing count -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				discoveryComplete: "true",
				omitFields:        []string{"sitemaps_found_count"},
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 26. Malformed count -> UNKNOWN
		t.Run("26. malformed count -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "abc",
				discoveryComplete:  "true",
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 27. Negative count -> UNKNOWN
		t.Run("27. negative count -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "-1",
				discoveryComplete:  "true",
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 28. Conflicting count -> UNKNOWN
		t.Run("28. conflicting count -> UNKNOWN", func(t *testing.T) {
			snap := newDISC001Snapshot(disc001SiteFixture{
				sitemapsFoundCount: "1",
				discoveryComplete:  "true",
				customObs: []audit.NormalizedObservation{
					{
						ObservationID:      "obs:conf:count",
						AuditRunID:         audit.AuditRunID(runID),
						SnapshotID:         audit.SnapshotID(snapID),
						SubjectType:        audit.SubjectSite,
						SubjectRef:         "site",
						Field:              "sitemaps_found_count",
						Value:              "2",
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"test"},
						ObservedAt:         time.Now().UTC(),
					},
				},
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 29. Zero count must not be inferred from no SubjectSitemap entities
		t.Run("29. zero count must not be inferred from absence of SubjectSitemap entities", func(t *testing.T) {
			// Count is missing, but no sitemaps present -> must be UNKNOWN, NOT inferred zero
			snap := newDISC001Snapshot(disc001SiteFixture{
				discoveryComplete: "true",
				omitFields:        []string{"sitemaps_found_count"},
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})

		// 30. Positive count must not be inferred solely from SubjectSitemap entity presence
		t.Run("30. positive count must not be inferred solely from SubjectSitemap presence", func(t *testing.T) {
			// Count observation is missing, but SubjectSitemap entities exist -> must be UNKNOWN, NOT inferred PASS
			snap := newDISC001Snapshot(disc001SiteFixture{
				discoveryComplete: "true",
				omitFields:        []string{"sitemaps_found_count"},
				sitemapDocs:       []string{"sm:1", "sm:2"},
			}, runID, snapID)
			res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polTrue}, "AR-DISC-001")
			if err != nil {
				t.Fatalf("eval failed: %v", err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN, got %v", res)
			}
		})
	})

	// ------------------------------------------------------------------------
	// E. Result integrity (Scenarios 31 - 40)
	// ------------------------------------------------------------------------
	t.Run("E. Result integrity", func(t *testing.T) {
		snapPass := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "true",
			discoveryStatus:    "COMPLETED",
			sitemapDocs:        []string{"sm:1"},
		}, runID, snapID)
		polTrue, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:1", "site", "true"),
		})

		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapPass, Policies: polTrue}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		rr := res[0]

		// 31. SubjectType is SubjectSite
		if rr.SubjectType != audit.SubjectSite {
			t.Errorf("31. expected SubjectSite, got %s", rr.SubjectType)
		}

		// 32. SubjectRef is "site"
		if rr.SubjectRef != "site" {
			t.Errorf("32. expected 'site', got %s", rr.SubjectRef)
		}

		// 33. Scope is ScopeSitewide
		if rr.Scope != audit.ScopeSitewide {
			t.Errorf("33. expected ScopeSitewide, got %s", rr.Scope)
		}

		// 34. Default severity is P1
		if rr.Severity != audit.SeverityP1 {
			t.Errorf("34. expected SeverityP1, got %s", rr.Severity)
		}

		// 35. Never emits FAIL across varied fixtures
		var allStatuses []audit.RuleResultStatus
		allStatuses = append(allStatuses, rr.Status)
		for _, s := range []string{"0", "1", "2"} {
			for _, p := range []string{"true", "false", "unspecified", ""} {
				snap := newDISC001Snapshot(disc001SiteFixture{
					sitemapsFoundCount: s,
					discoveryComplete:  "true",
					discoveryStatus:    "COMPLETED",
				}, runID, snapID)
				var pols []audit.ProjectPolicyAssignment
				if p != "" {
					pols = append(pols, makeSitePolicy(audit.AuditRunID(runID), "pol:"+p, "site", p))
				}
				pidx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), pols)
				out, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: pidx}, "AR-DISC-001")
				if err != nil {
					t.Fatalf("eval failed: %v", err)
				}
				for _, r := range out {
					allStatuses = append(allStatuses, r.Status)
					if r.Status == audit.StatusFail {
						t.Errorf("35. AR-DISC-001 must NEVER emit FAIL, but emitted FAIL for count=%s policy=%s", s, p)
					}
				}
			}
		}

		// 36. Never emits unsupported statuses
		for _, st := range allStatuses {
			if err := audit.ValidateRuleResultStatus(st); err != nil {
				t.Errorf("36. invalid status emitted: %v", err)
			}
		}

		// 37. Every EvidenceRef resolves
		idx, _ := engine.NewEvidenceIndex(snapPass)
		for _, ref := range rr.EvidenceRefs {
			if ref.EvidenceType == "NORMALIZED_OBSERVATION" {
				obsList := idx.GetObservations(audit.SubjectSite, "site", ref.Field)
				found := false
				for _, o := range obsList {
					if string(o.ObservationID) == ref.EvidenceRef {
						found = true
						break
					}
				}
				// Also check SubjectSitemap observations if correlated
				if !found {
					for _, smRef := range idx.SubjectRefs(audit.SubjectSitemap) {
						for _, o := range idx.GetObservations(audit.SubjectSitemap, smRef, ref.Field) {
							if string(o.ObservationID) == ref.EvidenceRef {
								found = true
								break
							}
						}
					}
				}
				if !found {
					t.Errorf("37. EvidenceRef %s (%s) did not resolve to a real observation", ref.EvidenceRef, ref.Field)
				}
			}
		}

		// 38. Policy provenance remains separate from observations
		snapZero := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			discoveryStatus:    "COMPLETED",
		}, runID, snapID)
		resWarn, _ := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapZero, Policies: polTrue}, "AR-DISC-001")
		for _, ref := range resWarn[0].EvidenceRefs {
			if ref.Field == "sitemap_expected" {
				if ref.EvidenceType != "PROJECT_POLICY_ASSIGNMENT" {
					t.Errorf("38. policy ref evidence_type must be PROJECT_POLICY_ASSIGNMENT, got %s", ref.EvidenceType)
				}
				if ref.EvidenceRef != "pol:1" {
					t.Errorf("38. policy ref evidence_ref must match PolicyAssignmentID, got %s", ref.EvidenceRef)
				}
			}
		}

		// 39. Repeated evaluation produces stable results
		res2, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snapPass, Policies: polTrue}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("repeated eval failed: %v", err)
		}
		if res[0].RuleResultID != res2[0].RuleResultID {
			t.Errorf("39. RuleResultID not stable: %s vs %s", res[0].RuleResultID, res2[0].RuleResultID)
		}
		if len(res[0].EvidenceRefs) != len(res2[0].EvidenceRefs) {
			t.Errorf("39. EvidenceRefs length changed: %d vs %d", len(res[0].EvidenceRefs), len(res2[0].EvidenceRefs))
		}
		for i := range res[0].EvidenceRefs {
			if res[0].EvidenceRefs[i].RuleEvidenceRefID != res2[0].EvidenceRefs[i].RuleEvidenceRefID {
				t.Errorf("39. EvidenceRef ordering unstable at %d", i)
			}
		}

		// 40. Other eighteen evaluators remain unaffected
		allIDs := eng.ImplementedRuleIDs()
		if len(allIDs) != 19 {
			t.Fatalf("40. expected 19 implemented rules, got %d", len(allIDs))
		}
		for _, id := range allIDs {
			if id == "AR-DISC-001" {
				continue
			}
			// Each other evaluator evaluates without error
			_, err := eng.EvaluateRule(ctx, snapPass, id)
			if err != nil {
				t.Errorf("40. rule %s evaluation failed: %v", id, err)
			}
		}
	})
}

// ============================================================================
// Mandatory Hermetic Integration Tests (Section 12)
// SQLite -> adapter.Build() -> EvidenceSnapshot -> PolicyIndex -> engine.EvaluateRuleWithContext()
// ============================================================================

func TestAdapter_Engine_AR_DISC_001_Integration(t *testing.T) {
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

	setupIntegrationRun := func(t *testing.T, runID string, discStatus string, sitemapsFound int, addSitemapDoc bool) (*sql.DB, *audit.EvidenceSnapshot) {
		t.Helper()
		db := newIntegrationDB(t)
		auditRunID := audit.AuditRunID("audit:" + runID)
		snapshotID := audit.SnapshotID("snap:" + runID)
		seed := "https://example.com/"

		setupRun(t, db, runID, seed)

		now := time.Now().UTC().Format(time.RFC3339)

		// Insert sitemap discovery
		if discStatus != "" {
			_, err = db.Exec(`INSERT INTO sitecrawl_sitemap_discovery(run_id, status, sitemaps_found, entries_found, depth_reached, depth_capped, urls_capped, byte_capped, started_at)
				VALUES(?, ?, ?, ?, 0, 0, 0, 0, ?)`, runID, discStatus, sitemapsFound, sitemapsFound, now)
			if err != nil {
				t.Fatalf("insert discovery failed: %v", err)
			}
		}

		if addSitemapDoc {
			_, err = db.Exec(`INSERT INTO sitecrawl_sitemaps(run_id, id, url, initial_status, final_status, status, fetch_complete, doc_type, parse_status, entry_count, fetched_at)
				VALUES(?, 1, 'https://example.com/sitemap.xml', 200, 200, 200, 1, 'urlset', 'parsed', 1, ?)`, runID, now)
			if err != nil {
				t.Fatalf("insert sitemap failed: %v", err)
			}
			_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id)
				VALUES(?, 1, 'robots_txt', 0)`, runID)
			_, _ = db.Exec(`INSERT INTO sitecrawl_sitemap_entries(run_id, sitemap_id, seq, url_id, loc)
				VALUES(?, 1, 0, 1, ?)`, runID, seed)
		}

		buildRes, err := adapter.Build(ctx, db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: auditRunID,
			SnapshotID: snapshotID,
		})
		if err != nil {
			t.Fatalf("adapter.Build failed: %v", err)
		}

		return db, buildRes.EvidenceSnapshot
	}

	// 1. Completed discovery with one sitemap -> PASS
	t.Run("1. completed discovery with one sitemap -> PASS", func(t *testing.T) {
		runID := "int:disc001:1_pass"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 1, true)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Errorf("expected PASS, got %v", res)
		}
	})

	// 2. Completed discovery with zero sitemaps + expected true -> WARNING
	t.Run("2. completed discovery with zero sitemaps + expected true -> WARNING", func(t *testing.T) {
		runID := "int:disc001:2_warn"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 0, false)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID("audit:"+runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:1", "site", "true"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusWarning {
			t.Errorf("expected WARNING, got %v", res)
		}
	})

	// 3. Completed discovery with zero sitemaps + expected false -> NOT_APPLICABLE
	t.Run("3. completed discovery with zero sitemaps + expected false -> NOT_APPLICABLE", func(t *testing.T) {
		runID := "int:disc001:3_na"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 0, false)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID("audit:"+runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:1", "site", "false"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected NOT_APPLICABLE, got %v", res)
		}
	})

	// 4. Completed discovery with zero sitemaps + unspecified -> MANUAL_REVIEW
	t.Run("4. completed discovery with zero sitemaps + unspecified -> MANUAL_REVIEW", func(t *testing.T) {
		runID := "int:disc001:4_mr_unspec"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 0, false)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID("audit:"+runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:1", "site", "unspecified"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusManualReview {
			t.Errorf("expected MANUAL_REVIEW, got %v", res)
		}
	})

	// 5. Completed discovery with zero sitemaps + no assignment -> MANUAL_REVIEW
	t.Run("5. completed discovery with zero sitemaps + no assignment -> MANUAL_REVIEW", func(t *testing.T) {
		runID := "int:disc001:5_mr_none"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 0, false)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusManualReview {
			t.Errorf("expected MANUAL_REVIEW, got %v", res)
		}
	})

	// 6. Incomplete discovery -> UNKNOWN
	t.Run("6. incomplete discovery -> UNKNOWN", func(t *testing.T) {
		runID := "int:disc001:6_incomp"
		_, snap := setupIntegrationRun(t, runID, "ATTEMPTED_INCOMPLETE", 0, false)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID("audit:"+runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:1", "site", "true"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 7. Discovery disabled -> UNKNOWN
	t.Run("7. discovery disabled -> UNKNOWN", func(t *testing.T) {
		runID := "int:disc001:7_disab"
		_, snap := setupIntegrationRun(t, runID, "DISABLED", 0, false)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID("audit:"+runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:1", "site", "true"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 8. Origin-only policy must not affect the result
	t.Run("8. origin-only policy does not affect the result", func(t *testing.T) {
		runID := "int:disc001:8_origin"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 0, false)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID("audit:"+runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:orig", "https://example.com", "true"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		// Since origin-only is treated as unspecified, result is MANUAL_REVIEW, NOT WARNING
		if len(res) != 1 || res[0].Status != audit.StatusManualReview {
			t.Errorf("expected MANUAL_REVIEW (origin policy ignored for site scope), got %v", res)
		}
	})

	// 9. Canonical "site" policy overrides unrelated origin-targeted policy
	t.Run("9. canonical site policy overrides origin policy", func(t *testing.T) {
		runID := "int:disc001:9_override"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 0, false)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID("audit:"+runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:site", "site", "false"),
			makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:orig", "https://example.com", "true"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected NOT_APPLICABLE from canonical site policy, got %v", res)
		}
	})

	// 10. Missing/corrupt discovery evidence -> UNKNOWN
	t.Run("10. missing discovery record -> UNKNOWN", func(t *testing.T) {
		runID := "int:disc001:10_nodisc"
		// discStatus == "" means sitecrawl_sitemap_discovery record is completely omitted
		_, snap := setupIntegrationRun(t, runID, "", 0, false)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		// If there is no site subject on snapshot, returns empty slice; if site subject present with missing count -> UNKNOWN
		for _, r := range res {
			if r.Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN for corrupt/missing discovery evidence, got %v", r.Status)
			}
		}
	})

	// 11. Policy provenance retained
	t.Run("11. policy provenance retained in EvidenceRefs", func(t *testing.T) {
		runID := "int:disc001:11_prov"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 0, false)
		polAssignment := makeSitePolicy(audit.AuditRunID("audit:"+runID), "pol:prov_test", "site", "true")
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID("audit:"+runID), []audit.ProjectPolicyAssignment{polAssignment})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		foundProv := false
		for _, ref := range res[0].EvidenceRefs {
			if ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" && ref.EvidenceRef == "pol:prov_test" {
				foundProv = true
				if ref.ObservedValue != "true" || ref.Field != "sitemap_expected" {
					t.Errorf("unexpected policy ref fields: %+v", ref)
				}
				break
			}
		}
		if !foundProv {
			t.Errorf("expected policy assignment provenance retained in EvidenceRefs")
		}
	})

	// 12. EvidenceRefs resolve to existing normalized observations
	t.Run("12. EvidenceRefs resolve to existing normalized observations", func(t *testing.T) {
		runID := "int:disc001:12_resolve"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 1, true)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
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
					t.Errorf("EvidenceRef field/value mismatch: ref=%+v, obs=%+v", ref, o)
				}
			}
		}
	})

	// 13. All previously implemented evaluators remain unchanged
	t.Run("13. all previously implemented evaluators evaluate without error", func(t *testing.T) {
		runID := "int:disc001:13_all"
		_, snap := setupIntegrationRun(t, runID, "COMPLETED", 1, true)
		allIDs := eng.ImplementedRuleIDs()
		if len(allIDs) != 19 {
			t.Fatalf("expected 19 rules, got %d", len(allIDs))
		}
		for _, rID := range allIDs {
			results, err := eng.EvaluateRule(ctx, snap, rID)
			if err != nil {
				t.Fatalf("rule %s failed: %v", rID, err)
			}
			// When run with valid pages, links, and sitemaps, sitemap rules evaluate non-empty
			if rID == "AR-DISC-001" || rID == "AR-DISC-002" || rID == "AR-DISC-003" || rID == "AR-DISC-004" || rID == "AR-DISC-005" {
				if len(results) == 0 {
					t.Errorf("expected non-empty results for %s", rID)
				}
			}
		}
	})
}

// ============================================================================
// Audit V1.8f Targeted Correction Regression Tests (Section 6)
// ============================================================================

func TestEngine_AR_DISC_001_TargetedCorrection(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := "run:disc001:corr"
	snapID := "snap:disc001:corr"
	ctx := context.Background()
	snapTrue := true
	snapFalse := false

	// 1. Missing discovery status + completion=true + snapshot=true + count=1 -> UNKNOWN
	t.Run("1. missing discovery status + completion=true + snapshot=true + count=1 -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			omitFields:         []string{"sitemap_discovery_status"},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 2. Missing discovery status + completion=true + snapshot=true + count=0 + policy=true -> UNKNOWN
	t.Run("2. missing discovery status + completion=true + snapshot=true + count=0 + policy=true -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			omitFields:         []string{"sitemap_discovery_status"},
		}, runID, snapID)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:sm_exp_true", "site", "true"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 3. Missing discovery status + completion=true + snapshot=true + count=0 + policy=false -> UNKNOWN
	t.Run("3. missing discovery status + completion=true + snapshot=true + count=0 + policy=false -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			omitFields:         []string{"sitemap_discovery_status"},
		}, runID, snapID)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:sm_exp_false", "site", "false"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 4. Missing discovery status + completion=true + snapshot=true + count=0 + policy missing -> UNKNOWN
	t.Run("4. missing discovery status + completion=true + snapshot=true + count=0 + policy missing -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			omitFields:         []string{"sitemap_discovery_status"},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 5. Empty discovery status -> UNKNOWN
	t.Run("5. empty discovery status -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			omitFields:         []string{"sitemap_discovery_status"},
			customObs: []audit.NormalizedObservation{
				{
					ObservationID:      "obs:corr:empty_status",
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectSite,
					SubjectRef:         "site",
					Field:              "sitemap_discovery_status",
					Value:              "",
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
					ObservedAt:         time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
				},
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 6. Whitespace-only discovery status -> UNKNOWN
	t.Run("6. whitespace-only discovery status -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			omitFields:         []string{"sitemap_discovery_status"},
			customObs: []audit.NormalizedObservation{
				{
					ObservationID:      "obs:corr:ws_status",
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectSite,
					SubjectRef:         "site",
					Field:              "sitemap_discovery_status",
					Value:              "   \t  ",
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
					ObservedAt:         time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
				},
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 7. Unrecognized discovery status -> UNKNOWN
	t.Run("7. unrecognized discovery status -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "PENDING_INSPECTION",
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 8. Conflicting status observations -> UNKNOWN
	t.Run("8. conflicting status observations -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "COMPLETED",
			customObs: []audit.NormalizedObservation{
				{
					ObservationID:      "obs:corr:status_conflict",
					AuditRunID:         audit.AuditRunID(runID),
					SnapshotID:         audit.SnapshotID(snapID),
					SubjectType:        audit.SubjectSite,
					SubjectRef:         "site",
					Field:              "sitemap_discovery_status",
					Value:              "NOT_ATTEMPTED",
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_sitemap_discovery:test"},
					ObservedAt:         time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC),
				},
			},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 9. NOT_ATTEMPTED + complete=true -> UNKNOWN
	t.Run("9. NOT_ATTEMPTED + complete=true -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "NOT_ATTEMPTED",
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 10. DISABLED + complete=true -> UNKNOWN
	t.Run("10. DISABLED + complete=true -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "DISABLED",
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 11. ATTEMPTED_INCOMPLETE + complete=true -> UNKNOWN
	t.Run("11. ATTEMPTED_INCOMPLETE + complete=true -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "ATTEMPTED_INCOMPLETE",
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 12. COMPLETED + complete=false -> UNKNOWN
	t.Run("12. COMPLETED + complete=false -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "false",
			snapshotComplete:   &snapFalse,
			discoveryStatus:    "COMPLETED",
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 13. COMPLETED + snapshot completion=false -> UNKNOWN
	t.Run("13. COMPLETED + snapshot completion=false -> UNKNOWN", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "true",
			snapshotComplete:   &snapFalse,
			discoveryStatus:    "COMPLETED",
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %v", res)
		}
	})

	// 14. COMPLETED + capped discovery -> UNKNOWN
	t.Run("14. COMPLETED + capped discovery -> UNKNOWN", func(t *testing.T) {
		for _, capType := range []string{"depth", "urls", "byte"} {
			fix := disc001SiteFixture{
				sitemapsFoundCount: "1",
				discoveryComplete:  "true",
				snapshotComplete:   &snapTrue,
				discoveryStatus:    "COMPLETED",
			}
			switch capType {
			case "depth":
				fix.depthCapped = "true"
			case "urls":
				fix.urlsCapped = "true"
			case "byte":
				fix.byteCapped = "true"
			}
			snap := newDISC001Snapshot(fix, runID, snapID)
			res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
			if err != nil {
				t.Fatalf("cap %s eval failed: %v", capType, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("cap %s: expected UNKNOWN, got %v", capType, res)
			}
		}
	})

	// 15. Valid COMPLETED + count=1 -> PASS
	t.Run("15. valid COMPLETED + count=1 -> PASS", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "1",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "COMPLETED",
			depthCapped:        "false",
			urlsCapped:         "false",
			byteCapped:         "false",
			sitemapDocs:        []string{"sm:1"},
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Errorf("expected PASS, got %v", res)
		}
	})

	// 16. Valid COMPLETED + count=0 + policy=true -> WARNING
	t.Run("16. valid COMPLETED + count=0 + policy=true -> WARNING", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "COMPLETED",
			depthCapped:        "false",
			urlsCapped:         "false",
			byteCapped:         "false",
		}, runID, snapID)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:sm_exp_true", "site", "true"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusWarning {
			t.Errorf("expected WARNING, got %v", res)
		}
	})

	// 17. Valid COMPLETED + count=0 + policy=false -> NOT_APPLICABLE
	t.Run("17. valid COMPLETED + count=0 + policy=false -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "COMPLETED",
			depthCapped:        "false",
			urlsCapped:         "false",
			byteCapped:         "false",
		}, runID, snapID)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:sm_exp_false", "site", "false"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected NOT_APPLICABLE, got %v", res)
		}
	})

	// 18. Valid COMPLETED + count=0 + policy unspecified -> MANUAL_REVIEW
	t.Run("18. valid COMPLETED + count=0 + policy unspecified -> MANUAL_REVIEW", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "0",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "COMPLETED",
			depthCapped:        "false",
			urlsCapped:         "false",
			byteCapped:         "false",
		}, runID, snapID)
		polIdx, _ := engine.NewPolicyIndexForRun(audit.AuditRunID(runID), []audit.ProjectPolicyAssignment{
			makeSitePolicy(audit.AuditRunID(runID), "pol:sm_exp_unspec", "site", "unspecified"),
		})
		res, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{Snapshot: snap, Policies: polIdx}, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusManualReview {
			t.Errorf("expected MANUAL_REVIEW, got %v", res)
		}
	})

	// 19. Negative count -1 -> UNKNOWN, summary contains -1
	t.Run("19. negative count -1 -> UNKNOWN, summary contains -1", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "-1",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "COMPLETED",
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
		if !strings.Contains(res[0].ObservedSummary, "-1") {
			t.Errorf("expected ObservedSummary to contain '-1', got %q", res[0].ObservedSummary)
		}
	})

	// 20. Negative count -5 -> UNKNOWN, summary contains -5
	t.Run("20. negative count -5 -> UNKNOWN, summary contains -5", func(t *testing.T) {
		snap := newDISC001Snapshot(disc001SiteFixture{
			sitemapsFoundCount: "-5",
			discoveryComplete:  "true",
			snapshotComplete:   &snapTrue,
			discoveryStatus:    "COMPLETED",
		}, runID, snapID)
		res, err := eng.EvaluateRule(ctx, snap, "AR-DISC-001")
		if err != nil {
			t.Fatalf("eval failed: %v", err)
		}
		if len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v", res)
		}
		if !strings.Contains(res[0].ObservedSummary, "-5") {
			t.Errorf("expected ObservedSummary to contain '-5', got %q", res[0].ObservedSummary)
		}
	})
}
