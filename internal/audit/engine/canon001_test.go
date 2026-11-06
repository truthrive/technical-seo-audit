package engine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForCANON001(
	subjectRef string,
	variantURL string,
	status int,
	finalURL string,
	redirectStatuses []int,
	preferredOriginObs string,
) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	if preferredOriginObs != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      "obs:pref:site",
			AuditRunID:         "audit:run:canon001",
			SnapshotID:         "snap:run:canon001",
			SubjectType:        audit.SubjectSite,
			SubjectRef:         "site",
			Field:              "preferred_origin",
			Value:              preferredOriginObs,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if variantURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:var:" + subjectRef),
			AuditRunID:         "audit:run:canon001",
			SnapshotID:         "snap:run:canon001",
			SubjectType:        audit.SubjectSiteConfiguration,
			SubjectRef:         subjectRef,
			Field:              "origin_variant",
			Value:              variantURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if status > 0 {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:st:" + subjectRef),
			AuditRunID:         "audit:run:canon001",
			SnapshotID:         "snap:run:canon001",
			SubjectType:        audit.SubjectSiteConfiguration,
			SubjectRef:         subjectRef,
			Field:              "variant_fetch_status",
			Value:              fmt.Sprintf("%d", status),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if finalURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:final:" + subjectRef),
			AuditRunID:         "audit:run:canon001",
			SnapshotID:         "snap:run:canon001",
			SubjectType:        audit.SubjectSiteConfiguration,
			SubjectRef:         subjectRef,
			Field:              "variant_final_url",
			Value:              finalURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	for i, rs := range redirectStatuses {
		hopJSON, _ := json.Marshal(map[string]any{
			"hop_index":           i,
			"source_url":          variantURL,
			"status":              rs,
			"resolved_target_url": finalURL,
		})
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID(fmt.Sprintf("obs:hop:%s:%d", subjectRef, i)),
			AuditRunID:         "audit:run:canon001",
			SnapshotID:         "snap:run:canon001",
			SubjectType:        audit.SubjectSiteConfiguration,
			SubjectRef:         subjectRef,
			Field:              "redirect_hop",
			Value:              string(hopJSON),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:canon001",
		AuditRunID:             "audit:run:canon001",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_CANON_001_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name             string
		policyOrigin     string
		obsOrigin        string
		variantURL       string
		status           int
		finalURL         string
		redirectStatuses []int
		expectedStatus   audit.RuleResultStatus
	}{
		// PASS: permanent redirect to preferred origin
		{
			name:             "PASS - 301 permanent redirect to preferred origin",
			policyOrigin:     "https://example.com",
			variantURL:       "http://example.com",
			status:           301,
			finalURL:         "https://example.com/",
			redirectStatuses: []int{301},
			expectedStatus:   audit.StatusPass,
		},
		{
			name:             "PASS - 308 permanent redirect www to non-www",
			policyOrigin:     "https://example.com",
			variantURL:       "https://www.example.com",
			status:           308,
			finalURL:         "https://example.com/",
			redirectStatuses: []int{308},
			expectedStatus:   audit.StatusPass,
		},

		// WARNING: temporary redirect to preferred origin
		{
			name:             "WARNING - 302 temporary redirect to preferred origin",
			policyOrigin:     "https://example.com",
			variantURL:       "http://example.com",
			status:           302,
			finalURL:         "https://example.com/",
			redirectStatuses: []int{302},
			expectedStatus:   audit.StatusWarning,
		},
		{
			name:             "WARNING - 307 temporary redirect to preferred origin",
			policyOrigin:     "https://example.com",
			variantURL:       "http://example.com",
			status:           307,
			finalURL:         "https://example.com/",
			redirectStatuses: []int{307},
			expectedStatus:   audit.StatusWarning,
		},

		// FAIL: remains 2xx or resolves to different origin
		{
			name:           "FAIL - variant remains HTTP 200 without redirect",
			policyOrigin:   "https://example.com",
			variantURL:     "http://example.com",
			status:         200,
			finalURL:       "http://example.com/",
			expectedStatus: audit.StatusFail,
		},
		{
			name:             "FAIL - variant redirects to completely different domain",
			policyOrigin:     "https://example.com",
			variantURL:       "http://example.com",
			status:           301,
			finalURL:         "https://another-domain.com/",
			redirectStatuses: []int{301},
			expectedStatus:   audit.StatusFail,
		},

		// NOT_APPLICABLE: evaluated origin is already preferred
		{
			name:           "NOT_APPLICABLE - evaluated origin is already preferred origin",
			policyOrigin:   "https://example.com",
			variantURL:     "https://example.com",
			status:         200,
			finalURL:       "https://example.com/",
			expectedStatus: audit.StatusNotApplicable,
		},

		// UNKNOWN: missing policy, missing status, or missing variant
		{
			name:           "UNKNOWN - preferred origin policy not specified",
			policyOrigin:   "",
			obsOrigin:      "",
			variantURL:     "http://example.com",
			status:         301,
			finalURL:       "https://example.com/",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "UNKNOWN - variant did not return usable HTTP response",
			policyOrigin:   "https://example.com",
			variantURL:     "http://example.com",
			status:         0,
			finalURL:       "",
			expectedStatus: audit.StatusUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snap := newSnapshotForCANON001("cfg:origin:variant", tc.variantURL, tc.status, tc.finalURL, tc.redirectStatuses, tc.obsOrigin)

			var policies *engine.PolicyIndex
			if tc.policyOrigin != "" {
				p, err := engine.NewPolicyIndex([]audit.ProjectPolicyAssignment{
					{
						PolicyAssignmentID: "pol:test:pref",
						AuditRunID:         "audit:run:canon001",
						PolicyKey:          audit.PolicyKeyPreferredOrigin,
						PolicyValue:        tc.policyOrigin,
						Scope:              audit.PolicyScopeSite,
						TargetRef:          "site",
						Source:             audit.PolicyProvenanceUserInput,
					},
				})
				if err != nil {
					t.Fatalf("failed to create policy index: %v", err)
				}
				policies = p
			} else {
				policies = engine.NewEmptyPolicyIndex()
			}

			results, err := eng.EvaluateRuleWithContext(ctx, engine.EvaluationContext{
				Snapshot: snap,
				Policies: policies,
			}, "AR-CANON-001")
			if err != nil {
				t.Fatalf("EvaluateRuleWithContext AR-CANON-001 failed: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 RuleResult, got %d", len(results))
			}
			r := results[0]
			if r.Status != tc.expectedStatus {
				t.Errorf("status = %v, want %v; summary: %s", r.Status, tc.expectedStatus, r.ObservedSummary)
			}
			if r.Severity != audit.SeverityP1 {
				t.Errorf("severity = %v, want P1", r.Severity)
			}
		})
	}
}

func TestCANON001_QA02Remediation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()

	obsHelper := func(runID, snapID, id string, st audit.EvaluationSubjectType, ref, field, val string) audit.NormalizedObservation {
		return audit.NormalizedObservation{
			ObservationID:      audit.ObservationID(id),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        st,
			SubjectRef:         ref,
			Field:              field,
			Value:              val,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		}
	}

	t.Run("Ordinary crawled URL without origin_variant is not treated as origin variant probe", func(t *testing.T) {
		runID := "run:test:canon001:regular-url"
		snapID := "snap:test:canon001:regular-url"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:pref:site", audit.SubjectSite, "site", "preferred_origin", "https://example.com"),
				// Crawled URL has url_identity, http_status, final_url, but NO origin_variant
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "http://example.com/regular-page"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "301"),
				obsHelper(runID, snapID, "obs:fin:1", audit.SubjectURL, "url:1", "final_url", "https://example.com/regular-page"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-001")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-001 failed: %v", err)
		}
		// Since url:1 has no origin_variant, it falls back to SubjectSite ("site"), which also has no origin_variant -> returns UNKNOWN for site, NOT a false PASS on url:1!
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].SubjectType == audit.SubjectURL {
			t.Errorf("expected SubjectURL without origin_variant not to be evaluated as origin variant, got SubjectType = %v", results[0].SubjectType)
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN when no origin variant exists, got %v (summary: %s)", results[0].Status, results[0].ObservedSummary)
		}
	})

	t.Run("Conflicting destination URLs on origin variant returns UNKNOWN", func(t *testing.T) {
		runID := "run:test:canon001:conflict-dest"
		snapID := "snap:test:canon001:conflict-dest"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:pref:site", audit.SubjectSite, "site", "preferred_origin", "https://example.com"),
				obsHelper(runID, snapID, "obs:var:1", audit.SubjectSiteConfiguration, "cfg:1", "origin_variant", "http://example.com"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectSiteConfiguration, "cfg:1", "variant_fetch_status", "301"),
				obsHelper(runID, snapID, "obs:dest:1", audit.SubjectSiteConfiguration, "cfg:1", "variant_final_url", "https://example.com"),
				obsHelper(runID, snapID, "obs:dest:2", audit.SubjectSiteConfiguration, "cfg:1", "variant_final_url", "https://other.com"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-001")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-001 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting destinations, got %v (summary: %s)", results[0].Status, results[0].ObservedSummary)
		}
	})

	t.Run("Temporary redirect 303 returns WARNING", func(t *testing.T) {
		runID := "run:test:canon001:303"
		snapID := "snap:test:canon001:303"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:pref:site", audit.SubjectSite, "site", "preferred_origin", "https://example.com"),
				obsHelper(runID, snapID, "obs:var:1", audit.SubjectSiteConfiguration, "cfg:1", "origin_variant", "http://example.com"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectSiteConfiguration, "cfg:1", "variant_fetch_status", "303"),
				obsHelper(runID, snapID, "obs:dest:1", audit.SubjectSiteConfiguration, "cfg:1", "variant_final_url", "https://example.com"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-001")
		if err != nil {
			t.Fatalf("EvaluateRule AR-CANON-001 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusWarning {
			t.Errorf("expected WARNING for 303 redirect, got %v (summary: %s)", results[0].Status, results[0].ObservedSummary)
		}
	})
}
