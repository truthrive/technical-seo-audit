package engine_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForINDEX004(
	subjectRef string,
	probeURL string,
	status int,
	finalURL string,
) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	if probeURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:purl:" + subjectRef),
			AuditRunID:         "audit:run:index004",
			SnapshotID:         "snap:run:index004",
			SubjectType:        audit.SubjectProbe,
			SubjectRef:         subjectRef,
			Field:              "missing_probe_url",
			Value:              probeURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if status > 0 {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:pst:" + subjectRef),
			AuditRunID:         "audit:run:index004",
			SnapshotID:         "snap:run:index004",
			SubjectType:        audit.SubjectProbe,
			SubjectRef:         subjectRef,
			Field:              "probe_final_status",
			Value:              fmt.Sprintf("%d", status),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if finalURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:pfin:" + subjectRef),
			AuditRunID:         "audit:run:index004",
			SnapshotID:         "snap:run:index004",
			SubjectType:        audit.SubjectProbe,
			SubjectRef:         subjectRef,
			Field:              "probe_final_url",
			Value:              finalURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:index004",
		AuditRunID:             "audit:run:index004",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_INDEX_004_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name           string
		probeURL       string
		status         int
		finalURL       string
		expectedStatus audit.RuleResultStatus
	}{
		// PASS: resolves to 404 or 410
		{
			name:           "PASS - resolves to HTTP 404",
			probeURL:       "https://example.com/sitecrawl-probe-404-nonexistent",
			status:         404,
			finalURL:       "https://example.com/sitecrawl-probe-404-nonexistent",
			expectedStatus: audit.StatusPass,
		},
		{
			name:           "PASS - resolves to HTTP 410 (Gone)",
			probeURL:       "https://example.com/sitecrawl-probe-410-nonexistent",
			status:         410,
			finalURL:       "https://example.com/sitecrawl-probe-410-nonexistent",
			expectedStatus: audit.StatusPass,
		},

		// FAIL: returns non-404/410 status
		{
			name:           "FAIL - returns HTTP 200 (soft 404)",
			probeURL:       "https://example.com/sitecrawl-probe-404-nonexistent",
			status:         200,
			finalURL:       "https://example.com/sitecrawl-probe-404-nonexistent",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "FAIL - redirects to homepage and returns 200",
			probeURL:       "https://example.com/sitecrawl-probe-404-nonexistent",
			status:         200,
			finalURL:       "https://example.com/",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "FAIL - returns HTTP 500 server error",
			probeURL:       "https://example.com/sitecrawl-probe-404-nonexistent",
			status:         500,
			finalURL:       "https://example.com/sitecrawl-probe-404-nonexistent",
			expectedStatus: audit.StatusFail,
		},

		// UNKNOWN: probe failed or evidence missing
		{
			name:           "UNKNOWN - probe fetch failed with status 0",
			probeURL:       "https://example.com/sitecrawl-probe-404-nonexistent",
			status:         0,
			finalURL:       "",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "UNKNOWN - missing probe URL evidence",
			probeURL:       "",
			status:         404,
			finalURL:       "",
			expectedStatus: audit.StatusUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snap := newSnapshotForINDEX004("probe:missing:404", tc.probeURL, tc.status, tc.finalURL)

			results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-004")
			if err != nil {
				t.Fatalf("EvaluateRule AR-INDEX-004 failed: %v", err)
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

func TestINDEX004_QA04Remediation(t *testing.T) {
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

	t.Run("Probe fetch not attempted returns UNKNOWN", func(t *testing.T) {
		runID := "run:test:index004:not-attempted"
		snapID := "snap:test:index004:not-attempted"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:p:url", audit.SubjectProbe, "probe:1", "missing_probe_url", "https://example.com/probe-404"),
				obsHelper(runID, snapID, "obs:p:att", audit.SubjectProbe, "probe:1", "fetch_attempted", "false"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-004")
		if err != nil {
			t.Fatalf("EvaluateRule AR-INDEX-004 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN when probe not attempted, got %v (summary: %s)", results[0].Status, results[0].ObservedSummary)
		}
	})

	t.Run("Conflicting destination URLs on probe returns UNKNOWN", func(t *testing.T) {
		runID := "run:test:index004:conflict-dest"
		snapID := "snap:test:index004:conflict-dest"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:p:url", audit.SubjectProbe, "probe:1", "missing_probe_url", "https://example.com/probe-404"),
				obsHelper(runID, snapID, "obs:p:st", audit.SubjectProbe, "probe:1", "probe_final_status", "301"),
				obsHelper(runID, snapID, "obs:p:dest:1", audit.SubjectProbe, "probe:1", "probe_final_url", "https://example.com/dest1"),
				obsHelper(runID, snapID, "obs:p:dest:2", audit.SubjectProbe, "probe:1", "probe_final_url", "https://example.com/dest2"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-004")
		if err != nil {
			t.Fatalf("EvaluateRule AR-INDEX-004 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting final URLs, got %v (summary: %s)", results[0].Status, results[0].ObservedSummary)
		}
	})
}
