package engine_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForINDEX006(
	subjectRef string,
	urlStr string,
	status int,
	targetFingerprint string,
	known404Fingerprint string,
	known404Status int,
) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	// Target URL observations
	if urlStr != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:url:" + subjectRef),
			AuditRunID:         "audit:run:index006",
			SnapshotID:         "snap:run:index006",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "url_identity",
			Value:              urlStr,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if status > 0 {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:st:" + subjectRef),
			AuditRunID:         "audit:run:index006",
			SnapshotID:         "snap:run:index006",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "http_status",
			Value:              fmt.Sprintf("%d", status),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if targetFingerprint != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:fp:" + subjectRef),
			AuditRunID:         "audit:run:index006",
			SnapshotID:         "snap:run:index006",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "content_fingerprint",
			Value:              targetFingerprint,
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	// Global / site known 404 template observations
	if known404Fingerprint != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      "obs:k404:fp",
			AuditRunID:         "audit:run:index006",
			SnapshotID:         "snap:run:index006",
			SubjectType:        audit.SubjectSite,
			SubjectRef:         "site",
			Field:              "known_404_content_fingerprint",
			Value:              known404Fingerprint,
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if known404Status > 0 {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      "obs:k404:st",
			AuditRunID:         "audit:run:index006",
			SnapshotID:         "snap:run:index006",
			SubjectType:        audit.SubjectSite,
			SubjectRef:         "site",
			Field:              "known_404_probe_status",
			Value:              fmt.Sprintf("%d", known404Status),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:index006",
		AuditRunID:             "audit:run:index006",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_INDEX_006_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name                string
		urlStr              string
		status              int
		targetFingerprint   string
		known404Fingerprint string
		known404Status      int
		expectedStatus      audit.RuleResultStatus
	}{
		// PASS: target is 200 and differs from known 404 template
		{
			name:                "PASS - 200 response differs from known 404 template",
			urlStr:              "https://example.com/valid-article",
			status:              200,
			targetFingerprint:   "fp-article-content-999",
			known404Fingerprint: "fp-404-template-001",
			known404Status:      404,
			expectedStatus:      audit.StatusPass,
		},

		// WARNING: target is 200 and matches known 404 template (soft 404)
		{
			name:                "WARNING - 200 response matches known 404 template (soft 404)",
			urlStr:              "https://example.com/fake-page-200",
			status:              200,
			targetFingerprint:   "fp-404-template-001",
			known404Fingerprint: "fp-404-template-001",
			known404Status:      404,
			expectedStatus:      audit.StatusWarning,
		},

		// NOT_APPLICABLE: target is not 200
		{
			name:                "NOT_APPLICABLE - target is HTTP 404",
			urlStr:              "https://example.com/not-found",
			status:              404,
			targetFingerprint:   "fp-404-template-001",
			known404Fingerprint: "fp-404-template-001",
			known404Status:      404,
			expectedStatus:      audit.StatusNotApplicable,
		},
		{
			name:                "NOT_APPLICABLE - target is HTTP 301",
			urlStr:              "https://example.com/redirect",
			status:              301,
			targetFingerprint:   "",
			known404Fingerprint: "fp-404-template-001",
			known404Status:      404,
			expectedStatus:      audit.StatusNotApplicable,
		},
		{
			name:                "NOT_APPLICABLE - target is HTTP 500",
			urlStr:              "https://example.com/server-error",
			status:              500,
			targetFingerprint:   "fp-500",
			known404Fingerprint: "fp-404-template-001",
			known404Status:      404,
			expectedStatus:      audit.StatusNotApplicable,
		},

		// UNKNOWN: missing probe template evidence or probe failed
		{
			name:                "UNKNOWN - known 404 template fingerprint missing",
			urlStr:              "https://example.com/page",
			status:              200,
			targetFingerprint:   "fp-page-123",
			known404Fingerprint: "",
			known404Status:      404,
			expectedStatus:      audit.StatusUnknown,
		},
		{
			name:                "UNKNOWN - probe returned 200 instead of 404/410",
			urlStr:              "https://example.com/page",
			status:              200,
			targetFingerprint:   "fp-page-123",
			known404Fingerprint: "fp-probe-template",
			known404Status:      200,
			expectedStatus:      audit.StatusUnknown,
		},
		{
			name:                "UNKNOWN - target content fingerprint unavailable",
			urlStr:              "https://example.com/page",
			status:              200,
			targetFingerprint:   "",
			known404Fingerprint: "fp-404-template-001",
			known404Status:      404,
			expectedStatus:      audit.StatusUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snap := newSnapshotForINDEX006(
				"url:test:1",
				tc.urlStr,
				tc.status,
				tc.targetFingerprint,
				tc.known404Fingerprint,
				tc.known404Status,
			)

			results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
			if err != nil {
				t.Fatalf("EvaluateRule AR-INDEX-006 failed: %v", err)
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

func TestINDEX006_ProbeProvenanceAndConflict(t *testing.T) {
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

	t.Run("Split probes - Probe A has fingerprint but Probe B has status 404", func(t *testing.T) {
		runID := "run:test:split-probe"
		snapID := "snap:test:split-probe"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/live-page"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "fingerprint-A"),
				// Probe A: only content fingerprint, no status
				obsHelper(runID, snapID, "obs:pA:fp", audit.SubjectProbe, "probe:A", "content_fingerprint", "fingerprint-A"),
				// Probe B: only status 404, no fingerprint
				obsHelper(runID, snapID, "obs:pB:st", audit.SubjectProbe, "probe:B", "probe_final_status", "404"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil {
			t.Fatalf("EvaluateRule AR-INDEX-006 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN when probe status and fingerprint belong to different probes, got %v (summary: %s)",
				results[0].Status, results[0].ObservedSummary)
		}
	})

	t.Run("Conflicting probes - Probe 1 and Probe 2 have different fingerprints", func(t *testing.T) {
		runID := "run:test:conflict-probe-fp"
		snapID := "snap:test:conflict-probe-fp"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/live-page"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "fingerprint-1"),
				// Probe 1: status 404, fingerprint 1
				obsHelper(runID, snapID, "obs:p1:st", audit.SubjectProbe, "probe:1", "probe_final_status", "404"),
				obsHelper(runID, snapID, "obs:p1:fp", audit.SubjectProbe, "probe:1", "content_fingerprint", "fingerprint-1"),
				// Probe 2: status 404, fingerprint 2
				obsHelper(runID, snapID, "obs:p2:st", audit.SubjectProbe, "probe:2", "probe_final_status", "404"),
				obsHelper(runID, snapID, "obs:p2:fp", audit.SubjectProbe, "probe:2", "content_fingerprint", "fingerprint-2"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil {
			t.Fatalf("EvaluateRule AR-INDEX-006 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting probe fingerprints, got %v (summary: %s)",
				results[0].Status, results[0].ObservedSummary)
		}
	})

	t.Run("Conflicting probes - Probe 1 (status 404) and Probe 2 (status 200)", func(t *testing.T) {
		runID := "run:test:conflict-probe-st"
		snapID := "snap:test:conflict-probe-st"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/live-page"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "fingerprint-1"),
				// Probe 1: status 404, fingerprint 1
				obsHelper(runID, snapID, "obs:p1:st", audit.SubjectProbe, "probe:1", "probe_final_status", "404"),
				obsHelper(runID, snapID, "obs:p1:fp", audit.SubjectProbe, "probe:1", "content_fingerprint", "fingerprint-1"),
				// Probe 2: status 200, fingerprint 1
				obsHelper(runID, snapID, "obs:p2:st", audit.SubjectProbe, "probe:2", "probe_final_status", "200"),
				obsHelper(runID, snapID, "obs:p2:fp", audit.SubjectProbe, "probe:2", "content_fingerprint", "fingerprint-1"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil {
			t.Fatalf("EvaluateRule AR-INDEX-006 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting probe statuses, got %v (summary: %s)",
				results[0].Status, results[0].ObservedSummary)
		}
	})

	t.Run("Probe returns 200 rather than 404/410", func(t *testing.T) {
		runID := "run:test:probe-200"
		snapID := "snap:test:probe-200"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/live-page"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "fingerprint-1"),
				// Probe returns 200
				obsHelper(runID, snapID, "obs:p1:st", audit.SubjectProbe, "probe:1", "probe_final_status", "200"),
				obsHelper(runID, snapID, "obs:p1:fp", audit.SubjectProbe, "probe:1", "content_fingerprint", "fingerprint-1"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil {
			t.Fatalf("EvaluateRule AR-INDEX-006 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN when probe returned 200 instead of 404/410, got %v (summary: %s)",
				results[0].Status, results[0].ObservedSummary)
		}
	})
}

func TestINDEX006_URLOverridesAndProbeCorrelation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	ctx := context.Background()
	now := time.Now().UTC()

	obsHelper := func(runID, snapID, id string, st audit.EvaluationSubjectType, ref, field, val string, srcRefs ...string) audit.NormalizedObservation {
		if len(srcRefs) == 0 {
			srcRefs = []string{"test"}
		}
		return audit.NormalizedObservation{
			ObservationID:      audit.ObservationID(id),
			AuditRunID:         audit.AuditRunID(runID),
			SnapshotID:         audit.SnapshotID(snapID),
			SubjectType:        st,
			SubjectRef:         ref,
			Field:              field,
			Value:              val,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: srcRefs,
			ObservedAt:         now,
		}
	}

	// Fixture 1: Valid site-level probe evidence with no URL overrides
	t.Run("Fixture 1 - Valid site-level probe evidence with no URL overrides", func(t *testing.T) {
		runID := "run:f1"
		snapID := "snap:f1"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:site:fp", audit.SubjectSite, "site", "known_404_content_fingerprint", "site-404-fp"),
				obsHelper(runID, snapID, "obs:site:st", audit.SubjectSite, "site", "known_404_probe_status", "404"),
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/p1"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "site-404-fp"),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusWarning {
			t.Errorf("expected WARNING, got %v", res[0].Status)
		}
	})

	// Fixture 2: URL-level fingerprint override only (lacks coupled status)
	t.Run("Fixture 2 - URL-level fingerprint override only", func(t *testing.T) {
		runID := "run:f2"
		snapID := "snap:f2"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:site:fp", audit.SubjectSite, "site", "known_404_content_fingerprint", "site-404-fp"),
				obsHelper(runID, snapID, "obs:site:st", audit.SubjectSite, "site", "known_404_probe_status", "404"),
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/p1"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "url-override-fp"),
				// URL-level fingerprint override ONLY
				obsHelper(runID, snapID, "obs:url:override:fp", audit.SubjectURL, "url:1", "known_404_content_fingerprint", "url-override-fp"),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for uncoupled URL-level fingerprint override, got %v (summary: %s)", res[0].Status, res[0].ObservedSummary)
		}
	})

	// Fixture 3: URL-level status override only (lacks coupled fingerprint)
	t.Run("Fixture 3 - URL-level status override only", func(t *testing.T) {
		runID := "run:f3"
		snapID := "snap:f3"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:site:fp", audit.SubjectSite, "site", "known_404_content_fingerprint", "site-404-fp"),
				obsHelper(runID, snapID, "obs:site:st", audit.SubjectSite, "site", "known_404_probe_status", "404"),
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/p1"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "site-404-fp"),
				// URL-level status override ONLY
				obsHelper(runID, snapID, "obs:url:override:st", audit.SubjectURL, "url:1", "known_404_probe_status", "404"),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for uncoupled URL-level status override, got %v (summary: %s)", res[0].Status, res[0].ObservedSummary)
		}
	})

	// Fixture 4: URL-level fingerprint and status from the same verified probe
	t.Run("Fixture 4 - URL-level fingerprint and status from same verified probe", func(t *testing.T) {
		runID := "run:f4"
		snapID := "snap:f4"
		probeSrc := "probe:custom:1"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/p1"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "custom-404-fp"),
				// Both from the same verified probe probe:custom:1
				obsHelper(runID, snapID, "obs:url:override:fp", audit.SubjectURL, "url:1", "known_404_content_fingerprint", "custom-404-fp", probeSrc),
				obsHelper(runID, snapID, "obs:url:override:st", audit.SubjectURL, "url:1", "known_404_probe_status", "404", probeSrc),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusWarning {
			t.Errorf("expected WARNING for matching custom template, got %v (summary: %s)", res[0].Status, res[0].ObservedSummary)
		}
	})

	// Fixture 5: URL-level fingerprint and status from different probes
	t.Run("Fixture 5 - URL-level fingerprint and status from different probes", func(t *testing.T) {
		runID := "run:f5"
		snapID := "snap:f5"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/p1"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "custom-404-fp"),
				// From probe:A
				obsHelper(runID, snapID, "obs:url:override:fp", audit.SubjectURL, "url:1", "known_404_content_fingerprint", "custom-404-fp", "probe:A"),
				// From probe:B
				obsHelper(runID, snapID, "obs:url:override:st", audit.SubjectURL, "url:1", "known_404_probe_status", "404", "probe:B"),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for URL overrides from different probes, got %v (summary: %s)", res[0].Status, res[0].ObservedSummary)
		}
	})

	// Fixture 6: Conflicting duplicate URL-level observations
	t.Run("Fixture 6 - Conflicting duplicate URL-level observations", func(t *testing.T) {
		runID := "run:f6"
		snapID := "snap:f6"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/p1"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "custom-404-fp"),
				// Two conflicting fingerprints on the same URL
				obsHelper(runID, snapID, "obs:url:fp:1", audit.SubjectURL, "url:1", "known_404_content_fingerprint", "custom-404-fp"),
				obsHelper(runID, snapID, "obs:url:fp:2", audit.SubjectURL, "url:1", "known_404_content_fingerprint", "other-404-fp"),
				obsHelper(runID, snapID, "obs:url:st:1", audit.SubjectURL, "url:1", "known_404_probe_status", "404"),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting duplicate URL observations, got %v (summary: %s)", res[0].Status, res[0].ObservedSummary)
		}
	})

	// Fixture 7: URL-level override conflicting with site-level evidence
	t.Run("Fixture 7 - URL-level override conflicting with site-level evidence", func(t *testing.T) {
		runID := "run:f7"
		snapID := "snap:f7"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				// Site-level template: site-404-fp
				obsHelper(runID, snapID, "obs:site:fp", audit.SubjectSite, "site", "known_404_content_fingerprint", "site-404-fp"),
				obsHelper(runID, snapID, "obs:site:st", audit.SubjectSite, "site", "known_404_probe_status", "404"),

				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/p1"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "site-404-fp"),

				// URL override conflicting with site template: diff-404-fp
				obsHelper(runID, snapID, "obs:url:override:fp", audit.SubjectURL, "url:1", "known_404_content_fingerprint", "diff-404-fp"),
				obsHelper(runID, snapID, "obs:url:override:st", audit.SubjectURL, "url:1", "known_404_probe_status", "404"),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for URL override conflicting with site template, got %v (summary: %s)", res[0].Status, res[0].ObservedSummary)
		}
	})

	// Fixture 8: Missing probe identity or provenance
	t.Run("Fixture 8 - Missing probe identity or provenance", func(t *testing.T) {
		runID := "run:f8"
		snapID := "snap:f8"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/p1"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "fp-1"),
				// No probe observations anywhere
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for missing probe template, got %v", res[0].Status)
		}
	})

	// Fixture 9: Target HTTP 200 matching the verified template
	t.Run("Fixture 9 - Target HTTP 200 matching the verified template", func(t *testing.T) {
		runID := "run:f9"
		snapID := "snap:f9"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:site:fp", audit.SubjectSite, "site", "known_404_content_fingerprint", "matching-fp"),
				obsHelper(runID, snapID, "obs:site:st", audit.SubjectSite, "site", "known_404_probe_status", "404"),
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/soft404"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "matching-fp"),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusWarning {
			t.Errorf("expected WARNING, got %v", res[0].Status)
		}
	})

	// Fixture 10: Target HTTP 200 differing from the verified template
	t.Run("Fixture 10 - Target HTTP 200 differing from the verified template", func(t *testing.T) {
		runID := "run:f10"
		snapID := "snap:f10"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:site:fp", audit.SubjectSite, "site", "known_404_content_fingerprint", "verified-404-fp"),
				obsHelper(runID, snapID, "obs:site:st", audit.SubjectSite, "site", "known_404_probe_status", "404"),
				obsHelper(runID, snapID, "obs:url:1", audit.SubjectURL, "url:1", "url_identity", "https://example.com/valid-article"),
				obsHelper(runID, snapID, "obs:st:1", audit.SubjectURL, "url:1", "http_status", "200"),
				obsHelper(runID, snapID, "obs:fp:1", audit.SubjectURL, "url:1", "content_fingerprint", "distinct-article-fp"),
			},
		}
		res, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-006")
		if err != nil || len(res) != 1 {
			t.Fatalf("unexpected error %v, count %d", err, len(res))
		}
		if res[0].Status != audit.StatusPass {
			t.Errorf("expected PASS, got %v", res[0].Status)
		}
	})
}
