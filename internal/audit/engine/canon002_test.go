package engine_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForCANON002Explicit(
	subjectRef string,
	urlStr string,
	slashURL string,
	nonSlashURL string,
	slashState int,
	nonSlashState int,
	slashFinalURL string,
	nonSlashFinalURL string,
	normHash string,
) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	if urlStr != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:url:" + subjectRef),
			AuditRunID:         "audit:run:canon002",
			SnapshotID:         "snap:run:canon002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "url_identity",
			Value:              urlStr,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if slashURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:surl:" + subjectRef),
			AuditRunID:         "audit:run:canon002",
			SnapshotID:         "snap:run:canon002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "slash_url",
			Value:              slashURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if nonSlashURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:nsurl:" + subjectRef),
			AuditRunID:         "audit:run:canon002",
			SnapshotID:         "snap:run:canon002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "non_slash_url",
			Value:              nonSlashURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if slashState > 0 {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:sst:" + subjectRef),
			AuditRunID:         "audit:run:canon002",
			SnapshotID:         "snap:run:canon002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "slash_response_state",
			Value:              fmt.Sprintf("%d", slashState),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if nonSlashState > 0 {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:nsst:" + subjectRef),
			AuditRunID:         "audit:run:canon002",
			SnapshotID:         "snap:run:canon002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "non_slash_response_state",
			Value:              fmt.Sprintf("%d", nonSlashState),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if slashFinalURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:sfin:" + subjectRef),
			AuditRunID:         "audit:run:canon002",
			SnapshotID:         "snap:run:canon002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "slash_final_url",
			Value:              slashFinalURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if nonSlashFinalURL != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:nsfin:" + subjectRef),
			AuditRunID:         "audit:run:canon002",
			SnapshotID:         "snap:run:canon002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "non_slash_final_url",
			Value:              nonSlashFinalURL,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	if normHash != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:hash:" + subjectRef),
			AuditRunID:         "audit:run:canon002",
			SnapshotID:         "snap:run:canon002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "normalized_content_hash",
			Value:              normHash,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"test"},
			ObservedAt:         now,
		})
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:canon002",
		AuditRunID:             "audit:run:canon002",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_CANON_002_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	testCases := []struct {
		name             string
		urlStr           string
		slashURL         string
		nonSlashURL      string
		slashState       int
		nonSlashState    int
		slashFinalURL    string
		nonSlashFinalURL string
		normHash         string
		expectedStatus   audit.RuleResultStatus
	}{
		// PASS: one is 2xx, the other permanently redirects (301) directly to it
		{
			name:             "PASS - non-slash 301 redirects to slash 200",
			urlStr:           "https://example.com/about/",
			slashURL:         "https://example.com/about/",
			nonSlashURL:      "https://example.com/about",
			slashState:       200,
			nonSlashState:    301,
			nonSlashFinalURL: "https://example.com/about/",
			expectedStatus:   audit.StatusPass,
		},
		{
			name:             "PASS - slash 308 redirects to non-slash 200",
			urlStr:           "https://example.com/about",
			slashURL:         "https://example.com/about/",
			nonSlashURL:      "https://example.com/about",
			slashState:       308,
			nonSlashState:    200,
			slashFinalURL:    "https://example.com/about",
			expectedStatus:   audit.StatusPass,
		},

		// FAIL: both return 200 with identical content hash (duplicate content)
		{
			name:           "FAIL - both return 200 with identical content hash",
			urlStr:         "https://example.com/about",
			slashURL:       "https://example.com/about/",
			nonSlashURL:    "https://example.com/about",
			slashState:     200,
			nonSlashState:  200,
			normHash:       "hash-abc-123",
			expectedStatus: audit.StatusFail,
		},

		// WARNING: both return 200 with differing or missing content hash
		{
			name:           "WARNING - both return 200 without duplicate hash proof",
			urlStr:         "https://example.com/about",
			slashURL:       "https://example.com/about/",
			nonSlashURL:    "https://example.com/about",
			slashState:     200,
			nonSlashState:  200,
			normHash:       "",
			expectedStatus: audit.StatusWarning,
		},

		// WARNING: temporary redirect 302
		{
			name:             "WARNING - non-slash uses temporary 302 redirect",
			urlStr:           "https://example.com/about/",
			slashURL:         "https://example.com/about/",
			nonSlashURL:      "https://example.com/about",
			slashState:       200,
			nonSlashState:    302,
			nonSlashFinalURL: "https://example.com/about/",
			expectedStatus:   audit.StatusWarning,
		},

		// NOT_APPLICABLE: root URL
		{
			name:           "NOT_APPLICABLE - root URL",
			urlStr:         "https://example.com/",
			expectedStatus: audit.StatusNotApplicable,
		},
		{
			name:           "NOT_APPLICABLE - root URL without slash",
			urlStr:         "https://example.com",
			expectedStatus: audit.StatusNotApplicable,
		},

		// NOT_APPLICABLE: static file extensions
		{
			name:           "NOT_APPLICABLE - pdf file",
			urlStr:         "https://example.com/report.pdf",
			expectedStatus: audit.StatusNotApplicable,
		},
		{
			name:           "NOT_APPLICABLE - html file",
			urlStr:         "https://example.com/index.html",
			expectedStatus: audit.StatusNotApplicable,
		},

		// UNKNOWN: missing alternative variant response
		{
			name:           "UNKNOWN - missing alternative response state",
			urlStr:         "https://example.com/about",
			slashURL:       "https://example.com/about/",
			nonSlashURL:    "https://example.com/about",
			slashState:     0,
			nonSlashState:  200,
			expectedStatus: audit.StatusUnknown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			snap := newSnapshotForCANON002Explicit(
				"url:test:1",
				tc.urlStr,
				tc.slashURL,
				tc.nonSlashURL,
				tc.slashState,
				tc.nonSlashState,
				tc.slashFinalURL,
				tc.nonSlashFinalURL,
				tc.normHash,
			)

			results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-002")
			if err != nil {
				t.Fatalf("EvaluateRule AR-CANON-002 failed: %v", err)
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
