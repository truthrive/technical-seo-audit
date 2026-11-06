package engine_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForACC001(
	subjectRef string,
	httpsURL string,
	attempted *bool,
	status *int,
	tlsValid *bool,
	errorType string,
) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	if httpsURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:url:" + subjectRef),
			AuditRunID:         "audit:run:acc001",
			SnapshotID:         "snap:run:acc001",
			SubjectType:        audit.SubjectSite,
			SubjectRef:         subjectRef,
			Field:              "site_https_url",
			Value:              httpsURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_runs:test"},
			ObservedAt:         now,
		})
	}

	if attempted != nil {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:att:" + subjectRef),
			AuditRunID:         "audit:run:acc001",
			SnapshotID:         "snap:run:acc001",
			SubjectType:        audit.SubjectSite,
			SubjectRef:         subjectRef,
			Field:              "fetch_attempted",
			Value:              fmt.Sprintf("%t", *attempted),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_runs:test"},
			ObservedAt:         now,
		})
	}

	if status != nil {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:st:" + subjectRef),
			AuditRunID:         "audit:run:acc001",
			SnapshotID:         "snap:run:acc001",
			SubjectType:        audit.SubjectSite,
			SubjectRef:         subjectRef,
			Field:              "fetch_status",
			Value:              fmt.Sprintf("%d", *status),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_runs:test"},
			ObservedAt:         now,
		})
	}

	if tlsValid != nil {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:tls:" + subjectRef),
			AuditRunID:         "audit:run:acc001",
			SnapshotID:         "snap:run:acc001",
			SubjectType:        audit.SubjectSite,
			SubjectRef:         subjectRef,
			Field:              "tls_valid",
			Value:              fmt.Sprintf("%t", *tlsValid),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_runs:test"},
			ObservedAt:         now,
		})
	}

	if errorType != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:err:" + subjectRef),
			AuditRunID:         "audit:run:acc001",
			SnapshotID:         "snap:run:acc001",
			SubjectType:        audit.SubjectSite,
			SubjectRef:         subjectRef,
			Field:              "fetch_error_type",
			Value:              errorType,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_runs:test"},
			ObservedAt:         now,
		})
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:acc001",
		AuditRunID:             "audit:run:acc001",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int    { return &i }

func TestAR_ACC_001_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name           string
		httpsURL       string
		attempted      *bool
		status         *int
		tlsValid       *bool
		errorType      string
		expectedStatus audit.RuleResultStatus
	}{
		// PASS cases: TLS succeeds and an HTTP response is obtained
		{
			name:           "PASS - status 200 with valid TLS",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(200),
			tlsValid:       boolPtr(true),
			expectedStatus: audit.StatusPass,
		},
		{
			name:           "PASS - status 301 redirect with valid TLS",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(301),
			tlsValid:       boolPtr(true),
			expectedStatus: audit.StatusPass,
		},
		{
			name:           "PASS - status 404 response obtained via TLS",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(404),
			tlsValid:       boolPtr(true),
			expectedStatus: audit.StatusPass,
		},
		{
			name:           "PASS - status 500 response obtained via TLS (reachable)",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(500),
			tlsValid:       boolPtr(true),
			expectedStatus: audit.StatusPass,
		},

		// FAIL cases: DNS, connection, or TLS/certificate failure prevents HTTP response
		{
			name:           "FAIL - TLS certificate validation failure",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(0),
			tlsValid:       boolPtr(false),
			errorType:      "ssl-error",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "FAIL - DNS resolution failure",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(0),
			errorType:      "dns-not-found",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "FAIL - Connection refused",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(0),
			errorType:      "connection-refused",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "FAIL - Connection timeout",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(0),
			errorType:      "timeout",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "FAIL - Generic connection error",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(0),
			errorType:      "connection-error",
			expectedStatus: audit.StatusFail,
		},

		// UNKNOWN cases
		{
			name:           "UNKNOWN - fetch was required but not attempted",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(false),
			status:         intPtr(0),
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "UNKNOWN - missing status and error type",
			httpsURL:       "https://example.com/",
			attempted:      boolPtr(true),
			status:         nil,
			errorType:      "",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "UNKNOWN - non-HTTPS URL provided",
			httpsURL:       "http://example.com/",
			attempted:      boolPtr(true),
			status:         intPtr(200),
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "UNKNOWN - empty HTTPS URL",
			httpsURL:       "",
			attempted:      boolPtr(true),
			status:         intPtr(200),
			expectedStatus: audit.StatusUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snap := newSnapshotForACC001("site", tc.httpsURL, tc.attempted, tc.status, tc.tlsValid, tc.errorType)
			results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-001")
			if err != nil {
				t.Fatalf("EvaluateRule AR-ACC-001 failed: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 RuleResult, got %d", len(results))
			}
			r := results[0]
			if r.Status != tc.expectedStatus {
				t.Errorf("status = %v, want %v; summary: %s", r.Status, tc.expectedStatus, r.ObservedSummary)
			}
			if r.Severity != audit.SeverityP0 {
				t.Errorf("severity = %v, want P0", r.Severity)
			}
			if r.Scope != audit.ScopeSitewide {
				t.Errorf("scope = %v, want SITEWIDE", r.Scope)
			}
		})
	}
}

func TestACC001_QA01Remediation(t *testing.T) {
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

	t.Run("Conflicting TLS validity observations return UNKNOWN", func(t *testing.T) {
		runID := "run:test:acc001:conflict-tls"
		snapID := "snap:test:acc001:conflict-tls"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:site:url", audit.SubjectSite, "site", "site_https_url", "https://example.com/"),
				obsHelper(runID, snapID, "obs:site:att", audit.SubjectSite, "site", "fetch_attempted", "true"),
				obsHelper(runID, snapID, "obs:site:st", audit.SubjectSite, "site", "fetch_status", "200"),
				obsHelper(runID, snapID, "obs:site:tls:1", audit.SubjectSite, "site", "tls_valid", "true"),
				obsHelper(runID, snapID, "obs:site:tls:2", audit.SubjectSite, "site", "tls_valid", "false"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-001")
		if err != nil {
			t.Fatalf("EvaluateRule AR-ACC-001 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting TLS validity, got %v (summary: %s)", results[0].Status, results[0].ObservedSummary)
		}
	})

	t.Run("No HTTPS fetch attempted returns UNKNOWN", func(t *testing.T) {
		runID := "run:test:acc001:no-attempt"
		snapID := "snap:test:acc001:no-attempt"
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           audit.SnapshotID(snapID),
			AuditRunID:           audit.AuditRunID(runID),
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.11.0",
			CrawlComplete:        true,
			CreatedAt:            now.Add(-1 * time.Minute),
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				obsHelper(runID, snapID, "obs:site:url", audit.SubjectSite, "site", "site_https_url", "https://example.com/"),
				obsHelper(runID, snapID, "obs:site:att", audit.SubjectSite, "site", "fetch_attempted", "false"),
			},
		}

		results, err := eng.EvaluateRule(ctx, snap, "AR-ACC-001")
		if err != nil {
			t.Fatalf("EvaluateRule AR-ACC-001 failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN when fetch not attempted, got %v (summary: %s)", results[0].Status, results[0].ObservedSummary)
		}
	})
}
