package engine_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

type linkTestFixture struct {
	subjectRef string // e.g. "link:audit:run:1:0"

	// Link observations
	sourceURL      string
	sourceURLs     []string
	sourceSubjRef  string
	sourceSubjRefs []string
	isInternal     string
	isInternals    []string
	targetURL      string
	targetURLs     []string
	targetSubjRef  string
	targetSubjRefs []string
	anchor         string
	location       string

	// Source URL observations on sourceSubjRef
	sourceURLIdent     string
	sourceURLIdents    []string
	sourceStatus       string
	omitSourceSubject  bool
	omitSourceURLIdent bool

	// Target URL observations on targetSubjRef
	targetURLIdent     string
	targetURLIdents    []string
	targetStatus       string
	targetStatuses     []string
	targetFinalURL     string
	omitTargetSubject  bool
	omitTargetURLIdent bool
}

func newLinkSnapshot(fixtures []linkTestFixture, auditRunID audit.AuditRunID, snapID audit.SnapshotID) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	seenSources := make(map[string]bool)
	seenTargets := make(map[string]bool)
	for _, f := range fixtures {
		// --- Link Observations ---
		if len(f.sourceURLs) > 0 {
			for i, u := range f.sourceURLs {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_url:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         f.subjectRef,
					Field:              "link_source_url",
					Value:              u,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_urls:1"},
					ObservedAt:         now,
				})
			}
		} else if f.sourceURL != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_url:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         f.subjectRef,
				Field:              "link_source_url",
				Value:              f.sourceURL,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_urls:1"},
				ObservedAt:         now,
			})
		}

		if len(f.sourceSubjRefs) > 0 {
			for i, r := range f.sourceSubjRefs {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_ref:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         f.subjectRef,
					Field:              "link_source_subject_ref",
					Value:              r,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_urls:1"},
					ObservedAt:         now,
				})
			}
		} else if f.sourceSubjRef != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_ref:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         f.subjectRef,
				Field:              "link_source_subject_ref",
				Value:              f.sourceSubjRef,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_urls:1"},
				ObservedAt:         now,
			})
		}

		if len(f.isInternals) > 0 {
			for i, v := range f.isInternals {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:is_int:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         f.subjectRef,
					Field:              "link_is_internal",
					Value:              v,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
					ObservedAt:         now,
				})
			}
		} else if f.isInternal != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:is_int:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         f.subjectRef,
				Field:              "link_is_internal",
				Value:              f.isInternal,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
				ObservedAt:         now,
			})
		}

		if len(f.targetURLs) > 0 {
			for i, u := range f.targetURLs {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_url:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         f.subjectRef,
					Field:              "link_target",
					Value:              u,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
					ObservedAt:         now,
				})
			}
		} else if f.targetURL != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_url:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         f.subjectRef,
				Field:              "link_target",
				Value:              f.targetURL,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
				ObservedAt:         now,
			})
		}

		if len(f.targetSubjRefs) > 0 {
			for i, r := range f.targetSubjRefs {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_ref:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         f.subjectRef,
					Field:              "link_target_subject_ref",
					Value:              r,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
					ObservedAt:         now,
				})
			}
		} else if f.targetSubjRef != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_ref:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         f.subjectRef,
				Field:              "link_target_subject_ref",
				Value:              f.targetSubjRef,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
				ObservedAt:         now,
			})
		}

		if f.anchor != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:anchor:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         f.subjectRef,
				Field:              "link_anchor",
				Value:              f.anchor,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
				ObservedAt:         now,
			})
		}

		if f.location != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:loc:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         f.subjectRef,
				Field:              "link_location",
				Value:              f.location,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
				ObservedAt:         now,
			})
		}

		// --- Source Observations on Source Subject Ref ---
		if f.sourceSubjRef != "" && !seenSources[f.sourceSubjRef] && !f.omitSourceSubject {
			seenSources[f.sourceSubjRef] = true
			if len(f.sourceURLIdents) > 0 {
				for i, u := range f.sourceURLIdents {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_ident:%s:%d", f.sourceSubjRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         f.sourceSubjRef,
						Field:              "url_identity",
						Value:              u,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"sitecrawl_urls:1"},
						ObservedAt:         now,
					})
				}
			} else if !f.omitSourceURLIdent {
				srcIdent := f.sourceURLIdent
				if srcIdent == "" {
					srcIdent = f.sourceURL
				}
				if srcIdent != "" {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_ident:%s", f.sourceSubjRef)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         f.sourceSubjRef,
						Field:              "url_identity",
						Value:              srcIdent,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"sitecrawl_urls:1"},
						ObservedAt:         now,
					})
				}
			}

			if f.sourceStatus != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src_stat:%s", f.sourceSubjRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.sourceSubjRef,
					Field:              "http_status",
					Value:              f.sourceStatus,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:1"},
					ObservedAt:         now,
				})
			}
		}

		// --- Target Observations on Target Subject Ref ---
		if f.targetSubjRef != "" && !seenTargets[f.targetSubjRef] && !f.omitTargetSubject {
			seenTargets[f.targetSubjRef] = true
			if len(f.targetURLIdents) > 0 {
				for i, u := range f.targetURLIdents {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_ident:%s:%d", f.targetSubjRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         f.targetSubjRef,
						Field:              "url_identity",
						Value:              u,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"sitecrawl_urls:2"},
						ObservedAt:         now,
					})
				}
			} else if !f.omitTargetURLIdent {
				if f.targetURLIdent != "" {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_ident:%s", f.targetSubjRef)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         f.targetSubjRef,
						Field:              "url_identity",
						Value:              f.targetURLIdent,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"sitecrawl_urls:2"},
						ObservedAt:         now,
					})
				}
			}

			if len(f.targetStatuses) > 0 {
				for i, s := range f.targetStatuses {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_stat:%s:%d", f.targetSubjRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         f.targetSubjRef,
						Field:              "http_status",
						Value:              s,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{"sitecrawl_pages:2"},
						ObservedAt:         now,
					})
				}
			} else if f.targetStatus != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_stat:%s", f.targetSubjRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.targetSubjRef,
					Field:              "http_status",
					Value:              f.targetStatus,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:2"},
					ObservedAt:         now,
				})
			}

			if f.targetFinalURL != "" {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt_final:%s", f.targetSubjRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.targetSubjRef,
					Field:              "redirect_final_url",
					Value:              f.targetFinalURL,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{"sitecrawl_redirects:2"},
					ObservedAt:         now,
				})
			}
		}
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             snapID,
		AuditRunID:             auditRunID,
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.7.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-10 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

// ============================================================================
// Group A: Status-class matrix
// ============================================================================

func TestEngine_Link_StatusClassMatrix(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:matrix")
	snapID := audit.SnapshotID("snap:link:matrix")

	testCases := []struct {
		statusCode  string
		expected002 audit.RuleResultStatus
		expected003 audit.RuleResultStatus
		expected004 audit.RuleResultStatus
	}{
		// 2xx
		{"200", audit.StatusPass, audit.StatusPass, audit.StatusPass},
		{"204", audit.StatusPass, audit.StatusPass, audit.StatusPass},
		// 3xx
		{"300", audit.StatusFail, audit.StatusPass, audit.StatusPass},
		{"301", audit.StatusFail, audit.StatusPass, audit.StatusPass},
		{"302", audit.StatusFail, audit.StatusPass, audit.StatusPass},
		{"307", audit.StatusFail, audit.StatusPass, audit.StatusPass},
		{"308", audit.StatusFail, audit.StatusPass, audit.StatusPass},
		{"399", audit.StatusFail, audit.StatusPass, audit.StatusPass},
		// 4xx
		{"400", audit.StatusPass, audit.StatusFail, audit.StatusPass},
		{"404", audit.StatusPass, audit.StatusFail, audit.StatusPass},
		{"410", audit.StatusPass, audit.StatusFail, audit.StatusPass},
		{"499", audit.StatusPass, audit.StatusFail, audit.StatusPass},
		// 5xx
		{"500", audit.StatusPass, audit.StatusPass, audit.StatusFail},
		{"502", audit.StatusPass, audit.StatusPass, audit.StatusFail},
		{"503", audit.StatusPass, audit.StatusPass, audit.StatusFail},
		{"599", audit.StatusPass, audit.StatusPass, audit.StatusFail},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("status_%s", tc.statusCode), func(t *testing.T) {
			fixture := linkTestFixture{
				subjectRef:     "link:run:link:matrix:1:0",
				sourceURL:      "https://example.com/source",
				sourceSubjRef:  "url:run:link:matrix:1",
				isInternal:     "true",
				targetURL:      "https://example.com/target",
				targetSubjRef:  "url:run:link:matrix:2",
				anchor:         "Target Anchor",
				location:       "NAV",
				targetURLIdent: "https://example.com/target",
				targetStatus:   tc.statusCode,
			}

			snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

			// AR-LINK-002
			res002, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-002")
			if err != nil {
				t.Fatalf("EvaluateRule AR-LINK-002 failed: %v", err)
			}
			if len(res002) != 1 || res002[0].Status != tc.expected002 {
				t.Errorf("AR-LINK-002 status for HTTP %s: expected %s, got %v", tc.statusCode, tc.expected002, res002[0].Status)
			}

			// AR-LINK-003
			res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
			if err != nil {
				t.Fatalf("EvaluateRule AR-LINK-003 failed: %v", err)
			}
			if len(res003) != 1 || res003[0].Status != tc.expected003 {
				t.Errorf("AR-LINK-003 status for HTTP %s: expected %s, got %v", tc.statusCode, tc.expected003, res003[0].Status)
			}

			// AR-LINK-004
			res004, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-004")
			if err != nil {
				t.Fatalf("EvaluateRule AR-LINK-004 failed: %v", err)
			}
			if len(res004) != 1 || res004[0].Status != tc.expected004 {
				t.Errorf("AR-LINK-004 status for HTTP %s: expected %s, got %v", tc.statusCode, tc.expected004, res004[0].Status)
			}
		})
	}
}

// ============================================================================
// Group B: Applicability
// ============================================================================

func TestEngine_Link_Applicability(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:app")
	snapID := audit.SnapshotID("snap:link:app")

	t.Run("verified external link -> NOT_APPLICABLE", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:app:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:app:1",
			isInternal:     "false", // External
			targetURL:      "https://external.example.org/page",
			targetSubjRef:  "url:run:link:app:2",
			targetURLIdent: "https://external.example.org/page",
			targetStatus:   "404", // Even if external status is 404, rule is not applicable!
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
				t.Errorf("%s: expected NOT_APPLICABLE for external link, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("mailto: hyperlink -> NOT_APPLICABLE", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:    "link:run:link:app:1:1",
			sourceURL:     "https://example.com/source",
			sourceSubjRef: "url:run:link:app:1",
			isInternal:    "true",
			targetURL:     "mailto:support@example.com",
			targetSubjRef: "url:run:link:app:3",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
				t.Errorf("%s: expected NOT_APPLICABLE for mailto:, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("tel: hyperlink -> NOT_APPLICABLE", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:    "link:run:link:app:1:2",
			sourceURL:     "https://example.com/source",
			sourceSubjRef: "url:run:link:app:1",
			isInternal:    "true",
			targetURL:     "tel:+1234567890",
			targetSubjRef: "url:run:link:app:4",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
				t.Errorf("%s: expected NOT_APPLICABLE for tel:, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("javascript: hyperlink -> NOT_APPLICABLE", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:    "link:run:link:app:1:3",
			sourceURL:     "https://example.com/source",
			sourceSubjRef: "url:run:link:app:1",
			isInternal:    "true",
			targetURL:     "javascript:void(0)",
			targetSubjRef: "url:run:link:app:5",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
				t.Errorf("%s: expected NOT_APPLICABLE for javascript:, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("missing link_is_internal -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:app:1:4",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:app:1",
			isInternal:     "", // Missing
			targetURL:      "https://example.com/page",
			targetSubjRef:  "url:run:link:app:6",
			targetURLIdent: "https://example.com/page",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for missing link_is_internal, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("conflicting link_is_internal -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:app:1:5",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:app:1",
			isInternals:    []string{"true", "false"}, // Conflicting
			targetURL:      "https://example.com/page",
			targetSubjRef:  "url:run:link:app:6",
			targetURLIdent: "https://example.com/page",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for conflicting link_is_internal, got %v", rID, res[0].Status)
			}
		}
	})
}

// ============================================================================
// Group C: Missing evidence
// ============================================================================

func TestEngine_Link_MissingEvidence(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:miss")
	snapID := audit.SnapshotID("snap:link:miss")

	t.Run("target discovered but not fetched -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:miss:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:miss:1",
			isInternal:     "true",
			targetURL:      "https://example.com/unfetched",
			targetSubjRef:  "url:run:link:miss:2",
			targetURLIdent: "https://example.com/unfetched",
			targetStatus:   "", // Unfetched, no status
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for unfetched target, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("missing target subject reference -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:    "link:run:link:miss:1:1",
			sourceURL:     "https://example.com/source",
			sourceSubjRef: "url:run:link:miss:1",
			isInternal:    "true",
			targetURL:     "https://example.com/target",
			targetSubjRef: "", // Missing target correlation
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for missing target ref, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("invalid HTTP status value -> UNKNOWN", func(t *testing.T) {
		invalidStatuses := []string{"abc", "-1", "0", "999"}
		for _, inv := range invalidStatuses {
			fixture := linkTestFixture{
				subjectRef:     "link:run:link:miss:1:2",
				sourceURL:      "https://example.com/source",
				sourceSubjRef:  "url:run:link:miss:1",
				isInternal:     "true",
				targetURL:      "https://example.com/target",
				targetSubjRef:  "url:run:link:miss:2",
				targetURLIdent: "https://example.com/target",
				targetStatus:   inv,
			}
			snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

			for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
				res, err := eng.EvaluateRule(context.Background(), snap, rID)
				if err != nil {
					t.Fatalf("%s failed: %v", rID, err)
				}
				if len(res) != 1 || res[0].Status != audit.StatusUnknown {
					t.Errorf("%s: expected UNKNOWN for status %q, got %v", rID, inv, res[0].Status)
				}
			}
		}
	})

	t.Run("conflicting target status observations -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:miss:1:3",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:miss:1",
			isInternal:     "true",
			targetURL:      "https://example.com/target",
			targetSubjRef:  "url:run:link:miss:2",
			targetURLIdent: "https://example.com/target",
			targetStatuses: []string{"200", "404"}, // Conflicting
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for conflicting status, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("missing source identity -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:miss:1:4",
			sourceURL:      "", // Missing
			sourceSubjRef:  "", // Missing
			isInternal:     "true",
			targetURL:      "https://example.com/target",
			targetSubjRef:  "url:run:link:miss:2",
			targetURLIdent: "https://example.com/target",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for missing source identity, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("missing anchor and location does NOT cause UNKNOWN when status is conclusive", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:miss:1:5",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:miss:1",
			isInternal:     "true",
			targetURL:      "https://example.com/target",
			targetSubjRef:  "url:run:link:miss:2",
			anchor:         "", // Missing supporting context
			location:       "", // Missing supporting context
			targetURLIdent: "https://example.com/target",
			targetStatus:   "404",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
		if err != nil {
			t.Fatalf("EvaluateRule AR-LINK-003 failed: %v", err)
		}
		// 404 must evaluate to FAIL on AR-LINK-003 despite missing anchor/location
		if len(res003) != 1 || res003[0].Status != audit.StatusFail {
			t.Errorf("expected FAIL for AR-LINK-003 with conclusive 404, got %v", res003[0].Status)
		}
	})
}

// ============================================================================
// Group D: Correlation integrity
// ============================================================================

func TestEngine_Link_CorrelationIntegrity(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:integ")
	snapID := audit.SnapshotID("snap:link:integ")

	t.Run("two links to one target are evaluated independently", func(t *testing.T) {
		f1 := linkTestFixture{
			subjectRef:     "link:run:link:integ:1:0",
			sourceURL:      "https://example.com/page1",
			sourceSubjRef:  "url:run:link:integ:1",
			isInternal:     "true",
			targetURL:      "https://example.com/shared-broken",
			targetSubjRef:  "url:run:link:integ:3",
			targetURLIdent: "https://example.com/shared-broken",
			targetStatus:   "404",
		}
		f2 := linkTestFixture{
			subjectRef:     "link:run:link:integ:2:0",
			sourceURL:      "https://example.com/page2",
			sourceSubjRef:  "url:run:link:integ:2",
			isInternal:     "true",
			targetURL:      "https://example.com/shared-broken",
			targetSubjRef:  "url:run:link:integ:3",
			targetURLIdent: "https://example.com/shared-broken",
			targetStatus:   "404",
		}
		snap := newLinkSnapshot([]linkTestFixture{f1, f2}, runID, snapID)

		res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
		if err != nil {
			t.Fatalf("EvaluateRule AR-LINK-003 failed: %v", err)
		}
		if len(res003) != 2 {
			t.Fatalf("expected exactly 2 independent results, got %d", len(res003))
		}
		if res003[0].SubjectRef != "link:run:link:integ:1:0" || res003[0].Status != audit.StatusFail {
			t.Errorf("result 0 mismatch: %v", res003[0])
		}
		if res003[1].SubjectRef != "link:run:link:integ:2:0" || res003[1].Status != audit.StatusFail {
			t.Errorf("result 1 mismatch: %v", res003[1])
		}
	})

	t.Run("source and target subjects from different audit runs -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:integ:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:integ:1",
			isInternal:     "true",
			targetURL:      "https://example.com/target",
			targetSubjRef:  "url:run:OTHER_RUN:2", // Cross-run target subject reference!
			targetURLIdent: "https://example.com/target",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for cross-run target ref, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("incorrect target subject type -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:integ:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:integ:1",
			isInternal:     "true",
			targetURL:      "https://example.com/target",
			targetSubjRef:  "site:run:link:integ:2", // Non-URL subject type!
			targetURLIdent: "https://example.com/target",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for non-URL target ref, got %v", rID, res[0].Status)
			}
		}
	})
}

// ============================================================================
// Group E: Redirect boundary
// ============================================================================

func TestEngine_Link_RedirectBoundary(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:redir")
	snapID := audit.SnapshotID("snap:link:redir")

	t.Run("link to 301 whose destination independently returns 200: AR-LINK-002 still FAILS", func(t *testing.T) {
		// Target URL's own status is 301, with redirect_final_url = "https://example.com/final"
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:redir:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:redir:1",
			isInternal:     "true",
			targetURL:      "https://example.com/redirect",
			targetSubjRef:  "url:run:link:redir:2",
			targetURLIdent: "https://example.com/redirect",
			targetStatus:   "301",
			targetFinalURL: "https://example.com/final",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		// AR-LINK-002 tests the linked target's own response (301), NOT the destination's 200!
		res002, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-002")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res002) != 1 || res002[0].Status != audit.StatusFail {
			t.Errorf("AR-LINK-002 must FAIL when linked target returns 301, got %v", res002[0].Status)
		}

		// AR-LINK-003 and AR-LINK-004 evaluate to PASS
		res003, _ := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
		if len(res003) != 1 || res003[0].Status != audit.StatusPass {
			t.Errorf("AR-LINK-003 must PASS for 301 target, got %v", res003[0].Status)
		}
		res004, _ := eng.EvaluateRule(context.Background(), snap, "AR-LINK-004")
		if len(res004) != 1 || res004[0].Status != audit.StatusPass {
			t.Errorf("AR-LINK-004 must PASS for 301 target, got %v", res004[0].Status)
		}
	})
}

// ============================================================================
// Group F: Determinism
// ============================================================================

func TestEngine_Link_Determinism(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:determ")
	snapID := audit.SnapshotID("snap:link:determ")

	f1 := linkTestFixture{
		subjectRef:     "link:run:link:determ:1:0",
		sourceURL:      "https://example.com/page1",
		sourceSubjRef:  "url:run:link:determ:1",
		isInternal:     "true",
		targetURL:      "https://example.com/broken",
		targetSubjRef:  "url:run:link:determ:3",
		targetURLIdent: "https://example.com/broken",
		targetStatus:   "404",
	}
	f2 := linkTestFixture{
		subjectRef:     "link:run:link:determ:2:0",
		sourceURL:      "https://example.com/page2",
		sourceSubjRef:  "url:run:link:determ:2",
		isInternal:     "true",
		targetURL:      "https://example.com/ok",
		targetSubjRef:  "url:run:link:determ:4",
		targetURLIdent: "https://example.com/ok",
		targetStatus:   "200",
	}
	snap := newLinkSnapshot([]linkTestFixture{f1, f2}, runID, snapID)

	res1, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
	if err != nil {
		t.Fatalf("first eval failed: %v", err)
	}
	res2, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
	if err != nil {
		t.Fatalf("second eval failed: %v", err)
	}

	if len(res1) != len(res2) {
		t.Fatalf("result counts differ: %d vs %d", len(res1), len(res2))
	}

	for i := range res1 {
		if res1[i].RuleResultID != res2[i].RuleResultID {
			t.Errorf("result[%d] ID mismatch: %q vs %q", i, res1[i].RuleResultID, res2[i].RuleResultID)
		}
		if res1[i].Status != res2[i].Status {
			t.Errorf("result[%d] Status mismatch: %q vs %q", i, res1[i].Status, res2[i].Status)
		}
		if res1[i].SubjectRef != res2[i].SubjectRef {
			t.Errorf("result[%d] SubjectRef mismatch: %q vs %q", i, res1[i].SubjectRef, res2[i].SubjectRef)
		}
		if len(res1[i].EvidenceRefs) != len(res2[i].EvidenceRefs) {
			t.Fatalf("result[%d] EvidenceRefs count mismatch", i)
		}
		for j := range res1[i].EvidenceRefs {
			if res1[i].EvidenceRefs[j] != res2[i].EvidenceRefs[j] {
				t.Errorf("result[%d] evidence[%d] mismatch", i, j)
			}
		}
	}

	// Context cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = eng.EvaluateRule(ctx, snap, "AR-LINK-002")
	if err == nil {
		t.Errorf("expected error for canceled context, got nil")
	}
}

// ============================================================================
// Group G: Regression
// ============================================================================

func TestEngine_Link_Regression_Rules(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	implementedIDs := eng.ImplementedRuleIDs()
	expectedIDs := []string{
		"AR-ACC-004",
		"AR-CANON-003",
		"AR-CANON-004",
		"AR-CANON-006",
		"AR-CANON-007",
		"AR-CANON-008",
		"AR-CANON-009",
		"AR-INDEX-001",
		"AR-INDEX-002",
		"AR-LINK-002",
		"AR-LINK-003",
		"AR-LINK-004",
	}

	if len(implementedIDs) != 12 {
		t.Fatalf("expected exactly 12 implemented rules, got %d: %v", len(implementedIDs), implementedIDs)
	}
	if !reflect.DeepEqual(implementedIDs, expectedIDs) {
		t.Fatalf("expected implemented IDs %v, got %v", expectedIDs, implementedIDs)
	}

	// Verify that future link rules beyond LINK-002/003/004 are NOT implemented
	for _, futureRule := range []string{"AR-LINK-001", "AR-LINK-005", "AR-LINK-006"} {
		for _, id := range implementedIDs {
			if id == futureRule {
				t.Errorf("rule %s must NOT be implemented in V1.6c", futureRule)
			}
		}
	}
}

// ============================================================================
// Group H: Integration (SQLite -> Adapter -> Frozen Snapshot -> Engine)
// ============================================================================

func TestEngine_Link_HermeticSQLiteIntegration(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	var srvURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/home", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Home</title></head><body>
<a href="%s/target200">OK Page</a>
<a href="%s/target301">Redirect Page</a>
<a href="%s/target404">Broken Page</a>
<a href="%s/target500">Error Page</a>
<a href="https://external.example.org/out">External Link</a>
</body></html>`, srvURL, srvURL, srvURL, srvURL)
	})
	mux.HandleFunc("/target200", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>200 OK</title></head><body>OK</body></html>`)
	})
	mux.HandleFunc("/target301", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srvURL+"/target200", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/target404", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})
	mux.HandleFunc("/target500", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Server Error", http.StatusInternalServerError)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open memory db failed: %v", err)
	}
	defer db.Close()

	runner := sitecrawl.NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("ensure schema failed: %v", err)
	}

	started, err := runner.Crawl(context.Background(), []string{srvURL + "/home"}, sitecrawl.Options{
		Mode:            sitecrawl.ModeSpider,
		MaxDepth:        2,
		MaxURLs:         10,
		Concurrency:     1,
		FollowRedirects: true,
	})
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	auditRunID := "audit:run:link:integ"
	snapID := "snap:run:link:integ"

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: audit.AuditRunID(auditRunID),
		SnapshotID: audit.SnapshotID(snapID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	if snap == nil {
		t.Fatalf("expected non-nil EvidenceSnapshot")
	}

	// Evaluate all three rules
	res002, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-002")
	if err != nil {
		t.Fatalf("Evaluate AR-LINK-002 failed: %v", err)
	}
	res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
	if err != nil {
		t.Fatalf("Evaluate AR-LINK-003 failed: %v", err)
	}
	res004, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-004")
	if err != nil {
		t.Fatalf("Evaluate AR-LINK-004 failed: %v", err)
	}

	// Count statuses
	statusCount := func(results []audit.RuleResult) map[audit.RuleResultStatus]int {
		counts := make(map[audit.RuleResultStatus]int)
		for _, r := range results {
			counts[r.Status]++
		}
		return counts
	}

	counts002 := statusCount(res002)
	counts003 := statusCount(res003)
	counts004 := statusCount(res004)

	// In our crawl:
	// - 1 link to 301 (LINK-002 FAIL, LINK-003 PASS, LINK-004 PASS)
	// - 1 link to 404 (LINK-002 PASS, LINK-003 FAIL, LINK-004 PASS)
	// - 1 link to 500 (LINK-002 PASS, LINK-003 PASS, LINK-004 FAIL)
	// - 1 link to 200 (LINK-002 PASS, LINK-003 PASS, LINK-004 PASS)
	// - 1 link to external (LINK-002 NOT_APPLICABLE, LINK-003 NOT_APPLICABLE, LINK-004 NOT_APPLICABLE)

	if counts002[audit.StatusFail] != 1 {
		t.Errorf("AR-LINK-002 expected 1 FAIL (for 301 target), got %d: %v", counts002[audit.StatusFail], counts002)
	}
	if counts003[audit.StatusFail] != 1 {
		t.Errorf("AR-LINK-003 expected 1 FAIL (for 404 target), got %d: %v", counts003[audit.StatusFail], counts003)
	}
	if counts004[audit.StatusFail] != 1 {
		t.Errorf("AR-LINK-004 expected 1 FAIL (for 500 target), got %d: %v", counts004[audit.StatusFail], counts004)
	}

	if counts002[audit.StatusNotApplicable] != 1 {
		t.Errorf("AR-LINK-002 expected 1 NOT_APPLICABLE (for external), got %d", counts002[audit.StatusNotApplicable])
	}
	if counts003[audit.StatusNotApplicable] != 1 {
		t.Errorf("AR-LINK-003 expected 1 NOT_APPLICABLE (for external), got %d", counts003[audit.StatusNotApplicable])
	}
	if counts004[audit.StatusNotApplicable] != 1 {
		t.Errorf("AR-LINK-004 expected 1 NOT_APPLICABLE (for external), got %d", counts004[audit.StatusNotApplicable])
	}

	// Verify all RuleEvidenceRefs point to real snapshot observations
	obsMap := make(map[audit.ObservationID]bool)
	for _, o := range snap.NormalizedObservations {
		obsMap[o.ObservationID] = true
	}

	allResults := append(append(res002, res003...), res004...)
	for _, r := range allResults {
		for _, ref := range r.EvidenceRefs {
			if !obsMap[audit.ObservationID(ref.EvidenceRef)] {
				t.Errorf("result %s evidence ref %s points to non-existent observation %s",
					r.RuleResultID, ref.RuleEvidenceRefID, ref.EvidenceRef)
			}
		}
	}
}

// ============================================================================
// Group I: Mandatory Correction Tests — Source Identity (Section 8.A)
// ============================================================================

func TestEngine_Link_Correction_SourceIdentity(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:src:ident")
	snapID := audit.SnapshotID("snap:link:src:ident")

	t.Run("valid source subject and matching URL -> evaluation proceeds", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:src:ident:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:src:ident:1",
			isInternal:     "true",
			targetURL:      "https://example.com/target",
			targetSubjRef:  "url:run:link:src:ident:2",
			targetURLIdent: "https://example.com/target",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusPass {
				t.Errorf("%s: expected PASS for valid source and 200 target, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("source subject reference has correct prefix but subject does not exist -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:        "link:run:link:src:ident:1:1",
			sourceURL:         "https://example.com/source",
			sourceSubjRef:     "url:run:link:src:ident:99", // Well-formed ref but subject not in snapshot
			omitSourceSubject: true,
			isInternal:        "true",
			targetURL:         "https://example.com/target",
			targetSubjRef:     "url:run:link:src:ident:2",
			targetURLIdent:    "https://example.com/target",
			targetStatus:      "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when source subject does not exist, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("source subject exists but url_identity differs from link_source_url -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:      "link:run:link:src:ident:1:2",
			sourceURL:       "https://example.com/source-a",
			sourceSubjRef:   "url:run:link:src:ident:1",
			sourceURLIdent:  "https://example.com/source-b", // Mismatch!
			isInternal:      "true",
			targetURL:       "https://example.com/target",
			targetSubjRef:   "url:run:link:src:ident:2",
			targetURLIdent:  "https://example.com/target",
			targetStatus:    "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when source url_identity differs from link_source_url, got %v", rID, res[0].Status)
			}
			// Verify relevant identity evidence was preserved even on UNKNOWN
			hasSrcIdent := false
			for _, ref := range res[0].EvidenceRefs {
				if ref.EvidenceRef == "obs:src_ident:url:run:link:src:ident:1" {
					hasSrcIdent = true
					break
				}
			}
			if !hasSrcIdent {
				t.Errorf("%s: expected source url_identity to be cited in evidence refs on UNKNOWN result", rID)
			}
		}
	})

	t.Run("source subject has no url_identity -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:         "link:run:link:src:ident:1:3",
			sourceURL:          "https://example.com/source",
			sourceSubjRef:      "url:run:link:src:ident:1",
			omitSourceURLIdent: true,
			sourceStatus:       "200", // Subject exists via http_status observation, but has no url_identity!
			isInternal:         "true",
			targetURL:          "https://example.com/target",
			targetSubjRef:      "url:run:link:src:ident:2",
			targetURLIdent:     "https://example.com/target",
			targetStatus:       "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when source subject has no url_identity, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("source subject has conflicting url_identity observations -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:      "link:run:link:src:ident:1:4",
			sourceURL:       "https://example.com/source",
			sourceSubjRef:   "url:run:link:src:ident:1",
			sourceURLIdents: []string{"https://example.com/source", "https://example.com/source-conflict"},
			isInternal:      "true",
			targetURL:       "https://example.com/target",
			targetSubjRef:   "url:run:link:src:ident:2",
			targetURLIdent:  "https://example.com/target",
			targetStatus:    "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when source subject has conflicting url_identity, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("source subject reference belongs to another audit run -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:src:ident:1:5",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:OTHER_RUN:1", // Cross-run source reference!
			isInternal:     "true",
			targetURL:      "https://example.com/target",
			targetSubjRef:  "url:run:link:src:ident:2",
			targetURLIdent: "https://example.com/target",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when source subject reference is from another run, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("malformed source subject reference -> UNKNOWN", func(t *testing.T) {
		for _, badRef := range []string{"not-a-url-ref", "url:", "url:run:link:src:ident:"} {
			fixture := linkTestFixture{
				subjectRef:     "link:run:link:src:ident:1:6",
				sourceURL:      "https://example.com/source",
				sourceSubjRef:  badRef,
				isInternal:     "true",
				targetURL:      "https://example.com/target",
				targetSubjRef:  "url:run:link:src:ident:2",
				targetURLIdent: "https://example.com/target",
				targetStatus:   "200",
			}
			snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

			for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
				res, err := eng.EvaluateRule(context.Background(), snap, rID)
				if err != nil {
					t.Fatalf("%s failed with bad ref %q: %v", rID, badRef, err)
				}
				if len(res) != 1 || res[0].Status != audit.StatusUnknown {
					t.Errorf("%s: expected UNKNOWN for malformed source ref %q, got %v", rID, badRef, res[0].Status)
				}
			}
		}
	})
}

// ============================================================================
// Group J: Mandatory Correction Tests — Target Identity (Section 8.B)
// ============================================================================

func TestEngine_Link_Correction_TargetIdentity(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:tgt:ident")
	snapID := audit.SnapshotID("snap:link:tgt:ident")

	t.Run("valid target subject and matching URL -> status evaluation proceeds", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:tgt:ident:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:tgt:ident:1",
			isInternal:     "true",
			targetURL:      "https://example.com/target-404",
			targetSubjRef:  "url:run:link:tgt:ident:2",
			targetURLIdent: "https://example.com/target-404",
			targetStatus:   "404",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
		if err != nil {
			t.Fatalf("Evaluate AR-LINK-003 failed: %v", err)
		}
		if len(res003) != 1 || res003[0].Status != audit.StatusFail {
			t.Errorf("AR-LINK-003: expected FAIL for valid target with 404, got %v", res003[0].Status)
		}
	})

	t.Run("target ref has correct prefix but subject does not exist -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:        "link:run:link:tgt:ident:1:1",
			sourceURL:         "https://example.com/source",
			sourceSubjRef:     "url:run:link:tgt:ident:1",
			isInternal:        "true",
			targetURL:         "https://example.com/target",
			targetSubjRef:     "url:run:link:tgt:ident:99", // Referenced target subject does not exist
			omitTargetSubject: true,
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when target subject does not exist, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("target subject exists but url_identity differs from link_target -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:tgt:ident:1:2",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:tgt:ident:1",
			isInternal:     "true",
			targetURL:      "https://example.com/target-a",
			targetSubjRef:  "url:run:link:tgt:ident:2",
			targetURLIdent: "https://example.com/target-b", // Mismatch!
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when target url_identity differs from link_target, got %v", rID, res[0].Status)
			}
			// Verify relevant identity evidence was preserved even on UNKNOWN
			hasTgtIdent := false
			for _, ref := range res[0].EvidenceRefs {
				if ref.EvidenceRef == "obs:tgt_ident:url:run:link:tgt:ident:2" {
					hasTgtIdent = true
					break
				}
			}
			if !hasTgtIdent {
				t.Errorf("%s: expected target url_identity to be cited in evidence refs on UNKNOWN result", rID)
			}
		}
	})

	t.Run("target subject has no url_identity -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:         "link:run:link:tgt:ident:1:3",
			sourceURL:          "https://example.com/source",
			sourceSubjRef:      "url:run:link:tgt:ident:1",
			isInternal:         "true",
			targetURL:          "https://example.com/target",
			targetSubjRef:      "url:run:link:tgt:ident:2",
			omitTargetURLIdent: true,
			targetStatus:       "200", // Target subject exists via http_status, but has no url_identity!
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when target subject has no url_identity, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("target subject has conflicting url_identity observations -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:      "link:run:link:tgt:ident:1:4",
			sourceURL:       "https://example.com/source",
			sourceSubjRef:   "url:run:link:tgt:ident:1",
			isInternal:      "true",
			targetURL:       "https://example.com/target",
			targetSubjRef:   "url:run:link:tgt:ident:2",
			targetURLIdents: []string{"https://example.com/target", "https://example.com/target-conflict"},
			targetStatus:    "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when target subject has conflicting url_identity, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("target subject reference belongs to another audit run -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:tgt:ident:1:5",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:tgt:ident:1",
			isInternal:     "true",
			targetURL:      "https://example.com/target",
			targetSubjRef:  "url:run:OTHER_RUN:2", // Cross-run target reference!
			targetURLIdent: "https://example.com/target",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when target subject reference belongs to another run, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("malformed target subject reference -> UNKNOWN", func(t *testing.T) {
		for _, badRef := range []string{"invalid-target-ref", "url:", "url:run:link:tgt:ident:"} {
			fixture := linkTestFixture{
				subjectRef:     "link:run:link:tgt:ident:1:6",
				sourceURL:      "https://example.com/source",
				sourceSubjRef:  "url:run:link:tgt:ident:1",
				isInternal:     "true",
				targetURL:      "https://example.com/target",
				targetSubjRef:  badRef,
				targetURLIdent: "https://example.com/target",
				targetStatus:   "200",
			}
			snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

			for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
				res, err := eng.EvaluateRule(context.Background(), snap, rID)
				if err != nil {
					t.Fatalf("%s failed with bad ref %q: %v", rID, badRef, err)
				}
				if len(res) != 1 || res[0].Status != audit.StatusUnknown {
					t.Errorf("%s: expected UNKNOWN for malformed target ref %q, got %v", rID, badRef, res[0].Status)
				}
			}
		}
	})
}

// ============================================================================
// Group K: Mandatory Correction Tests — False-Positive Prevention (Section 8.C)
// ============================================================================

func TestEngine_Link_Correction_FalsePositivePrevention(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:fp:prev")
	snapID := audit.SnapshotID("snap:link:fp:prev")

	t.Run("mismatched target subject returning HTTP 404 does NOT produce FAIL -> UNKNOWN", func(t *testing.T) {
		// Link target: https://example.com/a
		// Target subject reference points to URL /b
		// URL /b has verified HTTP 404
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:fp:prev:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:fp:prev:1",
			isInternal:     "true",
			targetURL:      "https://example.com/a",
			targetSubjRef:  "url:run:link:fp:prev:2",
			targetURLIdent: "https://example.com/b", // URL /b identity mismatch!
			targetStatus:   "404",                    // 404 belongs to /b, not /a!
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		// AR-LINK-002 -> UNKNOWN
		res002, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-002")
		if err != nil {
			t.Fatalf("AR-LINK-002 failed: %v", err)
		}
		if len(res002) != 1 || res002[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-002 expected UNKNOWN, got %v", res002[0].Status)
		}

		// AR-LINK-003 -> UNKNOWN (CRITICAL: Must NOT be FAIL because 404 belongs to /b!)
		res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
		if err != nil {
			t.Fatalf("AR-LINK-003 failed: %v", err)
		}
		if len(res003) != 1 || res003[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-003 expected UNKNOWN, got %v (must NOT conclude /a returns 404!)", res003[0].Status)
		}

		// AR-LINK-004 -> UNKNOWN
		res004, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-004")
		if err != nil {
			t.Fatalf("AR-LINK-004 failed: %v", err)
		}
		if len(res004) != 1 || res004[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-004 expected UNKNOWN, got %v", res004[0].Status)
		}
	})

	t.Run("mismatched target subject returning HTTP 301 does NOT produce FAIL -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:fp:prev:1:1",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:fp:prev:1",
			isInternal:     "true",
			targetURL:      "https://example.com/a",
			targetSubjRef:  "url:run:link:fp:prev:2",
			targetURLIdent: "https://example.com/b", // URL /b identity mismatch!
			targetStatus:   "301",                    // 301 belongs to /b, not /a!
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		// AR-LINK-002 -> UNKNOWN (Must NOT be FAIL!)
		res002, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-002")
		if err != nil {
			t.Fatalf("AR-LINK-002 failed: %v", err)
		}
		if len(res002) != 1 || res002[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-002 expected UNKNOWN, got %v (must NOT conclude /a returns 301!)", res002[0].Status)
		}

		// AR-LINK-003 -> UNKNOWN
		res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
		if err != nil {
			t.Fatalf("AR-LINK-003 failed: %v", err)
		}
		if len(res003) != 1 || res003[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-003 expected UNKNOWN, got %v", res003[0].Status)
		}

		// AR-LINK-004 -> UNKNOWN
		res004, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-004")
		if err != nil {
			t.Fatalf("AR-LINK-004 failed: %v", err)
		}
		if len(res004) != 1 || res004[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-004 expected UNKNOWN, got %v", res004[0].Status)
		}
	})

	t.Run("mismatched target subject returning HTTP 500 does NOT produce FAIL -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:fp:prev:1:2",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:fp:prev:1",
			isInternal:     "true",
			targetURL:      "https://example.com/a",
			targetSubjRef:  "url:run:link:fp:prev:2",
			targetURLIdent: "https://example.com/b", // URL /b identity mismatch!
			targetStatus:   "500",                    // 500 belongs to /b, not /a!
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		// AR-LINK-002 -> UNKNOWN
		res002, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-002")
		if err != nil {
			t.Fatalf("AR-LINK-002 failed: %v", err)
		}
		if len(res002) != 1 || res002[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-002 expected UNKNOWN, got %v", res002[0].Status)
		}

		// AR-LINK-003 -> UNKNOWN
		res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
		if err != nil {
			t.Fatalf("AR-LINK-003 failed: %v", err)
		}
		if len(res003) != 1 || res003[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-003 expected UNKNOWN, got %v", res003[0].Status)
		}

		// AR-LINK-004 -> UNKNOWN (Must NOT be FAIL!)
		res004, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-004")
		if err != nil {
			t.Fatalf("AR-LINK-004 failed: %v", err)
		}
		if len(res004) != 1 || res004[0].Status != audit.StatusUnknown {
			t.Errorf("AR-LINK-004 expected UNKNOWN, got %v (must NOT conclude /a returns 500!)", res004[0].Status)
		}
	})
}

// ============================================================================
// Group L: Mandatory Correction Tests — Identity Edge Cases (Section 8.D)
// ============================================================================

func TestEngine_Link_Correction_IdentityEdgeCases(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	runID := audit.AuditRunID("run:link:edge")
	snapID := audit.SnapshotID("snap:link:edge")

	t.Run("same hostname, different paths -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:edge:1:0",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:edge:1",
			isInternal:     "true",
			targetURL:      "https://example.com/path-one",
			targetSubjRef:  "url:run:link:edge:2",
			targetURLIdent: "https://example.com/path-two",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for different paths, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("same path, different query parameters -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:edge:1:1",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:edge:1",
			isInternal:     "true",
			targetURL:      "https://example.com/search?q=foo",
			targetSubjRef:  "url:run:link:edge:2",
			targetURLIdent: "https://example.com/search?q=bar",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for different query params, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("HTTP vs HTTPS scheme difference -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:edge:1:2",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:edge:1",
			isInternal:     "true",
			targetURL:      "http://example.com/page",
			targetSubjRef:  "url:run:link:edge:2",
			targetURLIdent: "https://example.com/page",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for HTTP vs HTTPS mismatch, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("trailing slash difference -> UNKNOWN", func(t *testing.T) {
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:edge:1:3",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:edge:1",
			isInternal:     "true",
			targetURL:      "https://example.com/about",
			targetSubjRef:  "url:run:link:edge:2",
			targetURLIdent: "https://example.com/about/",
			targetStatus:   "200",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN for trailing slash mismatch, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("target redirect destination is not treated as original URL identity -> UNKNOWN", func(t *testing.T) {
		// Link target is /original, but target subject identity is /redirected-target
		fixture := linkTestFixture{
			subjectRef:     "link:run:link:edge:1:4",
			sourceURL:      "https://example.com/source",
			sourceSubjRef:  "url:run:link:edge:1",
			isInternal:     "true",
			targetURL:      "https://example.com/original",
			targetSubjRef:  "url:run:link:edge:2",
			targetURLIdent: "https://example.com/redirected-target", // Redirect destination cannot substitute original identity!
			targetStatus:   "200",
			targetFinalURL: "https://example.com/redirected-target",
		}
		snap := newLinkSnapshot([]linkTestFixture{fixture}, runID, snapID)

		for _, rID := range []string{"AR-LINK-002", "AR-LINK-003", "AR-LINK-004"} {
			res, err := eng.EvaluateRule(context.Background(), snap, rID)
			if err != nil {
				t.Fatalf("%s failed: %v", rID, err)
			}
			if len(res) != 1 || res[0].Status != audit.StatusUnknown {
				t.Errorf("%s: expected UNKNOWN when target redirect destination differs from original target identity, got %v", rID, res[0].Status)
			}
		}
	})

	t.Run("two distinct link subjects referencing the same valid target -> both evaluated properly", func(t *testing.T) {
		f1 := linkTestFixture{
			subjectRef:     "link:run:link:edge:1:5",
			sourceURL:      "https://example.com/source-1",
			sourceSubjRef:  "url:run:link:edge:1",
			isInternal:     "true",
			targetURL:      "https://example.com/shared-target",
			targetSubjRef:  "url:run:link:edge:3",
			targetURLIdent: "https://example.com/shared-target",
			targetStatus:   "404",
		}
		f2 := linkTestFixture{
			subjectRef:     "link:run:link:edge:2:5",
			sourceURL:      "https://example.com/source-2",
			sourceSubjRef:  "url:run:link:edge:2",
			isInternal:     "true",
			targetURL:      "https://example.com/shared-target",
			targetSubjRef:  "url:run:link:edge:3",
			targetURLIdent: "https://example.com/shared-target",
			targetStatus:   "404",
		}
		snap := newLinkSnapshot([]linkTestFixture{f1, f2}, runID, snapID)

		res003, err := eng.EvaluateRule(context.Background(), snap, "AR-LINK-003")
		if err != nil {
			t.Fatalf("EvaluateRule AR-LINK-003 failed: %v", err)
		}
		if len(res003) != 2 {
			t.Fatalf("expected 2 results, got %d", len(res003))
		}
		if res003[0].Status != audit.StatusFail || res003[1].Status != audit.StatusFail {
			t.Errorf("both links to 404 target must evaluate to FAIL on AR-LINK-003: %v, %v", res003[0].Status, res003[1].Status)
		}
		if res003[0].RuleResultID == res003[1].RuleResultID {
			t.Errorf("rule result IDs must be distinct: %s vs %s", res003[0].RuleResultID, res003[1].RuleResultID)
		}
	})
}
