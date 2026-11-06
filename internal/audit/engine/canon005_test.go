package engine_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
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

type canon005Fixture struct {
	subjectRef          string
	urlValues           []string
	renderedValues      []string
	extractValues       []string
	countValues         []string
	sampleTruncValues   []string
	rawValues           []string
	syntaxValues        []string
	parseErrValues      []string
	normTargetValues    []string
	completeValues      []string
	httpStatusValues    []string
	contentTypeValues   []string
	fetchErrorValues    []string
	canonicalResolved   []string
}

func newCANON005Snapshot(fixtures []canon005Fixture, auditRunID audit.AuditRunID, snapID audit.SnapshotID) *audit.EvidenceSnapshot {
	now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	var obs []audit.NormalizedObservation
	seq := 0
	nextID := func(prefix string) audit.ObservationID {
		seq++
		return audit.ObservationID(fmt.Sprintf("obs:%s:%d", prefix, seq))
	}

	addObs := func(subj, field, val string, deriv audit.DerivationType) {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextID(field),
			AuditRunID:         auditRunID,
			SnapshotID:         snapID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subj,
			Field:              field,
			Value:              val,
			DerivationType:     deriv,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		})
	}

	for _, f := range fixtures {
		for _, v := range f.urlValues {
			addObs(f.subjectRef, "url_identity", v, audit.DerivationDirect)
		}
		for _, v := range f.renderedValues {
			addObs(f.subjectRef, "rendered", v, audit.DerivationDirect)
		}
		for _, v := range f.extractValues {
			addObs(f.subjectRef, "html_extraction_status", v, audit.DerivationDirect)
		}
		for _, v := range f.countValues {
			addObs(f.subjectRef, "canonical_count", v, audit.DerivationNormalized)
		}
		for _, v := range f.sampleTruncValues {
			addObs(f.subjectRef, "canonical_sample_truncated", v, audit.DerivationDirect)
		}
		for _, v := range f.rawValues {
			addObs(f.subjectRef, "canonical_raw", v, audit.DerivationDirect)
		}
		for _, v := range f.syntaxValues {
			addObs(f.subjectRef, "canonical_syntax_status", v, audit.DerivationNormalized)
		}
		for _, v := range f.parseErrValues {
			addObs(f.subjectRef, "canonical_parse_error", v, audit.DerivationNormalized)
		}
		for _, v := range f.normTargetValues {
			addObs(f.subjectRef, "canonical_normalized_target", v, audit.DerivationNormalized)
		}
		for _, v := range f.completeValues {
			addObs(f.subjectRef, "canonical_normalization_complete", v, audit.DerivationNormalized)
		}
		for _, v := range f.httpStatusValues {
			addObs(f.subjectRef, "http_status", v, audit.DerivationDirect)
		}
		for _, v := range f.contentTypeValues {
			addObs(f.subjectRef, "content_type", v, audit.DerivationDirect)
		}
		for _, v := range f.fetchErrorValues {
			addObs(f.subjectRef, "fetch_error_type", v, audit.DerivationNormalized)
		}
		for _, v := range f.canonicalResolved {
			addObs(f.subjectRef, "canonical_resolved", v, audit.DerivationNormalized)
		}
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             snapID,
		AuditRunID:             auditRunID,
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.10.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-5 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_CANON_005_22ScenarioMatrix(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}
	fixedTime := time.Date(2026, 10, 1, 12, 30, 0, 0, time.UTC)
	eng.SetClock(func() time.Time { return fixedTime })

	ctx := context.Background()
	auditRunID := audit.AuditRunID("audit:run:canon005:matrix")
	snapID := audit.SnapshotID("snap:canon005:matrix")

	cases := []struct {
		name               string
		fixture            canon005Fixture
		wantStatus         audit.RuleResultStatus
		wantPrimaryField   string
		wantSummarySubstr  string
	}{
		{
			name: "Scenario 1: No canonical declaration in proven complete 2xx HTML -> NOT_APPLICABLE",
			fixture: canon005Fixture{
				subjectRef:    "url:test:1",
				urlValues:     []string{"https://example.com/no-canon"},
				extractValues: []string{"complete"},
				countValues:   []string{"0"},
			},
			wantStatus:        audit.StatusNotApplicable,
			wantSummarySubstr: "canonical_count = 0",
		},
		{
			name: "Scenario 2: Exactly one valid absolute HTTP canonical URL -> PASS",
			fixture: canon005Fixture{
				subjectRef:       "url:test:2",
				urlValues:        []string{"http://example.com/page"},
				extractValues:    []string{"complete"},
				countValues:      []string{"1"},
				rawValues:        []string{"http://example.com/page"},
				syntaxValues:     []string{"valid"},
				normTargetValues: []string{"http://example.com/page"},
				completeValues:   []string{"true"},
			},
			wantStatus:       audit.StatusPass,
			wantPrimaryField: "canonical_resolved_url",
		},
		{
			name: "Scenario 3: Exactly one valid absolute HTTPS canonical URL -> PASS",
			fixture: canon005Fixture{
				subjectRef:       "url:test:3",
				urlValues:        []string{"https://example.com/page"},
				extractValues:    []string{"complete"},
				countValues:      []string{"1"},
				rawValues:        []string{"https://example.com/page"},
				syntaxValues:     []string{"valid"},
				normTargetValues: []string{"https://example.com/page"},
				completeValues:   []string{"true"},
			},
			wantStatus:       audit.StatusPass,
			wantPrimaryField: "canonical_resolved_url",
		},
		{
			name: "Scenario 4: Exactly one valid relative canonical URL (/about) with no base href -> PASS",
			fixture: canon005Fixture{
				subjectRef:       "url:test:4",
				urlValues:        []string{"https://example.com/source"},
				extractValues:    []string{"complete"},
				countValues:      []string{"1"},
				rawValues:        []string{"/about"},
				syntaxValues:     []string{"valid"},
				normTargetValues: []string{"https://example.com/about"},
				completeValues:   []string{"true"},
			},
			wantStatus:       audit.StatusPass,
			wantPrimaryField: "canonical_resolved_url",
		},
		{
			name: "Scenario 5: Relative canonical URL (sub) with valid HTTP(S) base href -> PASS",
			fixture: canon005Fixture{
				subjectRef:       "url:test:5",
				urlValues:        []string{"https://example.com/page"},
				extractValues:    []string{"complete"},
				countValues:      []string{"1"},
				rawValues:        []string{"sub"},
				syntaxValues:     []string{"valid"},
				normTargetValues: []string{"https://example.com/dir/sub"},
				completeValues:   []string{"true"},
			},
			wantStatus:       audit.StatusPass,
			wantPrimaryField: "canonical_resolved_url",
		},
		{
			name: "Scenario 6a-i (ftp base): Relative canonical + valid non-HTTP(S) ftp base href -> FAIL",
			fixture: canon005Fixture{
				subjectRef:     "url:test:6a1:ftp",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"sub.html"},
				syntaxValues:   []string{"resolution_error"},
				parseErrValues: []string{"resolution_error:unsupported_scheme"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusFail,
			wantPrimaryField:  "canonical_parse_error",
			wantSummarySubstr: "resolution_error:unsupported_scheme",
		},
		{
			name: "Scenario 6a-i (opaque mailto base): Relative canonical + opaque mailto base href -> FAIL",
			fixture: canon005Fixture{
				subjectRef:     "url:test:6a1:mailto",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"sub.html"},
				syntaxValues:   []string{"resolution_error"},
				parseErrValues: []string{"resolution_error:non_hierarchical_base"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusFail,
			wantPrimaryField:  "canonical_parse_error",
			wantSummarySubstr: "resolution_error:non_hierarchical_base",
		},
		{
			name: "Scenario 6a-ii: Relative canonical + malformed base href (http://[::1) -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:     "url:test:6a2",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"sub.html"},
				syntaxValues:   []string{"insufficient_evidence"},
				parseErrValues: []string{"insufficient_evidence:ambiguous_base_fallback"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusUnknown,
			wantPrimaryField:  "canonical_syntax_status",
			wantSummarySubstr: "insufficient_evidence:ambiguous_base_fallback",
		},
		{
			name: "Scenario 6b-i: Absolute HTTP(S) canonical + malformed base href -> PASS",
			fixture: canon005Fixture{
				subjectRef:       "url:test:6b1",
				urlValues:        []string{"https://example.com/page"},
				extractValues:    []string{"complete"},
				countValues:      []string{"1"},
				rawValues:        []string{"https://example.com/page"},
				syntaxValues:     []string{"valid"},
				normTargetValues: []string{"https://example.com/page"},
				completeValues:   []string{"true"},
			},
			wantStatus:       audit.StatusPass,
			wantPrimaryField: "canonical_resolved_url",
		},
		{
			name: "Scenario 6b-ii: Relative canonical + empty or fragment base href (deterministic FinalURL fallback) -> PASS",
			fixture: canon005Fixture{
				subjectRef:       "url:test:6b2",
				urlValues:        []string{"https://example.com/dir/page.html"},
				extractValues:    []string{"complete"},
				countValues:      []string{"1"},
				rawValues:        []string{"sub.html"},
				syntaxValues:     []string{"valid"},
				normTargetValues: []string{"https://example.com/dir/sub.html"},
				completeValues:   []string{"true"},
			},
			wantStatus:       audit.StatusPass,
			wantPrimaryField: "canonical_resolved_url",
		},
		{
			name: "Scenario 6c: Relative canonical + conflicting/missing base context -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:     "url:test:6c",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"sub.html"},
				syntaxValues:   []string{"insufficient_evidence"},
				parseErrValues: []string{"insufficient_evidence:missing_base_context"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusUnknown,
			wantPrimaryField:  "canonical_syntax_status",
			wantSummarySubstr: "insufficient_evidence:missing_base_context",
		},
		{
			name: "Scenario 7: Canonical directly using unsupported scheme (javascript:/mailto:/ftp:) -> FAIL",
			fixture: canon005Fixture{
				subjectRef:     "url:test:7",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"javascript:void(0)"},
				syntaxValues:   []string{"resolution_error"},
				parseErrValues: []string{"resolution_error:unsupported_scheme"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusFail,
			wantPrimaryField:  "canonical_parse_error",
			wantSummarySubstr: "resolution_error:unsupported_scheme",
		},
		{
			name: "Scenario 8a: Canonical containing malformed URL syntax (http://[::1) -> FAIL",
			fixture: canon005Fixture{
				subjectRef:     "url:test:8a",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"http://[::1"},
				syntaxValues:   []string{"parser_error"},
				parseErrValues: []string{"parser_error:malformed_url"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusFail,
			wantPrimaryField:  "canonical_parse_error",
			wantSummarySubstr: "parser_error:malformed_url",
		},
		{
			name: "Scenario 8b: Canonical with missing host (http:///path) -> FAIL",
			fixture: canon005Fixture{
				subjectRef:     "url:test:8b",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"http:///path"},
				syntaxValues:   []string{"resolution_error"},
				parseErrValues: []string{"resolution_error:missing_host"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusFail,
			wantPrimaryField:  "canonical_parse_error",
			wantSummarySubstr: "resolution_error:missing_host",
		},
		{
			name: "Scenario 9: Empty canonical declaration (href=\"\") -> FAIL",
			fixture: canon005Fixture{
				subjectRef:     "url:test:9",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{""},
				syntaxValues:   []string{"parser_error"},
				parseErrValues: []string{"parser_error:empty_href"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusFail,
			wantPrimaryField:  "canonical_parse_error",
			wantSummarySubstr: "parser_error:empty_href",
		},
		{
			name: "Scenario 10: Whitespace-only canonical declaration (href=\"   \") -> FAIL",
			fixture: canon005Fixture{
				subjectRef:     "url:test:10",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"   "},
				syntaxValues:   []string{"parser_error"},
				parseErrValues: []string{"parser_error:empty_href"},
				completeValues: []string{"false"},
			},
			wantStatus:        audit.StatusFail,
			wantPrimaryField:  "canonical_parse_error",
			wantSummarySubstr: "parser_error:empty_href",
		},
		{
			name: "Scenario 11a: Multiple distinct canonical declarations (2..5 declarations) -> NOT_APPLICABLE",
			fixture: canon005Fixture{
				subjectRef:        "url:test:11a",
				urlValues:         []string{"https://example.com/page"},
				extractValues:     []string{"complete"},
				countValues:       []string{"2"},
				sampleTruncValues: []string{"false"},
				rawValues:         []string{"https://example.com/a", "https://example.com/b"},
			},
			wantStatus:        audit.StatusNotApplicable,
			wantSummarySubstr: "2 canonical declarations",
		},
		{
			name: "Scenario 11b: Multiple distinct canonical declarations (>5 declarations, sample capped at 5) -> NOT_APPLICABLE",
			fixture: canon005Fixture{
				subjectRef:        "url:test:11b",
				urlValues:         []string{"https://example.com/page"},
				extractValues:     []string{"complete"},
				countValues:       []string{"7"},
				sampleTruncValues: []string{"true"},
				rawValues:         []string{"https://example.com/1", "https://example.com/2", "https://example.com/3", "https://example.com/4", "https://example.com/5"},
			},
			wantStatus:        audit.StatusNotApplicable,
			wantSummarySubstr: "7 canonical declarations",
		},
		{
			name: "Scenario 12a: Duplicate identical canonical declarations (2..5 identical declarations) -> NOT_APPLICABLE",
			fixture: canon005Fixture{
				subjectRef:        "url:test:12a",
				urlValues:         []string{"https://example.com/page"},
				extractValues:     []string{"complete"},
				countValues:       []string{"3"},
				sampleTruncValues: []string{"false"},
				rawValues:         []string{"https://example.com/page", "https://example.com/page", "https://example.com/page"},
			},
			wantStatus:        audit.StatusNotApplicable,
			wantSummarySubstr: "3 canonical declarations",
		},
		{
			name: "Scenario 12b: Duplicate identical canonical declarations (>5 declarations, sample capped at 5) -> NOT_APPLICABLE",
			fixture: canon005Fixture{
				subjectRef:        "url:test:12b",
				urlValues:         []string{"https://example.com/page"},
				extractValues:     []string{"complete"},
				countValues:       []string{"6"},
				sampleTruncValues: []string{"true"},
				rawValues:         []string{"https://example.com/page", "https://example.com/page", "https://example.com/page", "https://example.com/page", "https://example.com/page"},
			},
			wantStatus:        audit.StatusNotApplicable,
			wantSummarySubstr: "6 canonical declarations",
		},
		{
			name: "Scenario 13a: Non-HTML content_type (application/pdf, html_extraction_status absent) -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:        "url:test:13a",
				urlValues:         []string{"https://example.com/doc.pdf"},
				httpStatusValues:  []string{"200"},
				contentTypeValues: []string{"application/pdf"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "HTML extraction completeness evidence is unavailable",
		},
		{
			name: "Scenario 13b: Legacy SQLite row lacking ExtractionStatus (missing_legacy) -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:    "url:test:13b",
				urlValues:     []string{"https://example.com/legacy"},
				extractValues: []string{"missing_legacy"},
				countValues:   []string{"1"},
			},
			wantStatus:        audit.StatusUnknown,
			wantPrimaryField:  "html_extraction_status",
			wantSummarySubstr: "missing_legacy",
		},
		{
			name: "Scenario 14: HTTP fetch failed (DNS/TLS/timeout or non-200 status) -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:       "url:test:14",
				urlValues:        []string{"https://example.com/fail"},
				httpStatusValues: []string{"500"},
				extractValues:    []string{"missing_legacy"},
				fetchErrorValues: []string{"TIMEOUT"},
			},
			wantStatus:       audit.StatusUnknown,
			wantPrimaryField: "html_extraction_status",
		},
		{
			name: "Scenario 15a: HTTP 200 HTML response with mid-stream read error (read_error) -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:       "url:test:15a",
				urlValues:        []string{"https://example.com/read-err"},
				httpStatusValues: []string{"200"},
				extractValues:    []string{"read_error"},
				countValues:      []string{"1"},
			},
			wantStatus:        audit.StatusUnknown,
			wantPrimaryField:  "html_extraction_status",
			wantSummarySubstr: "read error",
		},
		{
			name: "Scenario 15b: HTTP 200 HTML response with size-limit truncation (truncated_size_limit) -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:       "url:test:15b",
				urlValues:        []string{"https://example.com/truncated"},
				httpStatusValues: []string{"200"},
				extractValues:    []string{"truncated_size_limit"},
				countValues:      []string{"1"},
			},
			wantStatus:        audit.StatusUnknown,
			wantPrimaryField:  "html_extraction_status",
			wantSummarySubstr: "size limit",
		},
		{
			name: "Scenario 15c: Exact limit-byte boundary with post-parse drain error (read_error) -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:       "url:test:15c",
				urlValues:        []string{"https://example.com/drain-err"},
				httpStatusValues: []string{"200"},
				extractValues:    []string{"read_error"},
				countValues:      []string{"0"},
			},
			wantStatus:        audit.StatusUnknown,
			wantPrimaryField:  "html_extraction_status",
			wantSummarySubstr: "read error",
		},
		{
			name: "Scenario 16: Rendered page without retained raw server HTML (rendered == true) -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:     "url:test:16",
				urlValues:      []string{"https://example.com/spa"},
				renderedValues: []string{"true"},
				extractValues:  []string{"missing_legacy"},
			},
			wantStatus:        audit.StatusUnknown,
			wantPrimaryField:  "rendered",
			wantSummarySubstr: "rendered via headless browser",
		},
		{
			name: "Scenario 17: Single canonical where normalization succeeds and resolution is proven valid -> PASS",
			fixture: canon005Fixture{
				subjectRef:       "url:test:17",
				urlValues:        []string{"https://example.com/page"},
				extractValues:    []string{"complete"},
				countValues:      []string{"1"},
				rawValues:        []string{"https://EXAMPLE.com:443/page#frag"},
				syntaxValues:     []string{"valid"},
				normTargetValues: []string{"https://example.com/page"},
				completeValues:   []string{"true"},
			},
			wantStatus:       audit.StatusPass,
			wantPrimaryField: "canonical_resolved_url",
		},
		{
			name: "Scenario 18: Single canonical where normalization fails with proven parser/resolution error -> FAIL",
			fixture: canon005Fixture{
				subjectRef:     "url:test:18",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"ftp://files.example.com/doc"},
				syntaxValues:   []string{"resolution_error"},
				parseErrValues: []string{"resolution_error:unsupported_scheme"},
				completeValues: []string{"false"},
			},
			wantStatus:       audit.StatusFail,
			wantPrimaryField: "canonical_parse_error",
		},
		{
			name: "Scenario 19a: Single canonical where normalization fails with unclassified insufficient_evidence -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:     "url:test:19a",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"sub/page"},
				syntaxValues:   []string{"insufficient_evidence"},
				parseErrValues: []string{"insufficient_evidence:unclassified_normalization_failure"},
				completeValues: []string{"false"},
			},
			wantStatus:       audit.StatusUnknown,
			wantPrimaryField: "canonical_syntax_status",
		},
		{
			name: "Scenario 19b: Single canonical with canonical_normalization_complete=false alone (no syntax/parse_error) -> UNKNOWN (never FAIL)",
			fixture: canon005Fixture{
				subjectRef:     "url:test:19b",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				completeValues: []string{"false"},
			},
			wantStatus: audit.StatusUnknown,
		},
		{
			name: "Scenario 19c: Proven parser_error/resolution_error but missing canonical_normalization_complete -> UNKNOWN (MISSING != FALSE)",
			fixture: canon005Fixture{
				subjectRef:     "url:test:19c",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"http://[::1"},
				syntaxValues:   []string{"parser_error"},
				parseErrValues: []string{"parser_error:malformed_url"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "canonical normalization completeness evidence is unavailable",
		},
		{
			name: "Scenario 20a: Conflicting canonical_count observations -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:    "url:test:20a",
				urlValues:     []string{"https://example.com/page"},
				extractValues: []string{"complete"},
				countValues:   []string{"1", "2"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "conflicting",
		},
		{
			name: "Scenario 20b: Conflicting html_extraction_status observations -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:    "url:test:20b",
				urlValues:     []string{"https://example.com/page"},
				extractValues: []string{"complete", "read_error"},
				countValues:   []string{"1"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "conflicting",
		},
		{
			name: "Scenario 20c: Conflicting canonical_syntax_status observations -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:     "url:test:20c",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"https://example.com/page"},
				syntaxValues:   []string{"valid", "parser_error"},
				completeValues: []string{"true"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "conflicting",
		},
		{
			name: "Scenario 20d: Conflicting canonical_normalization_complete on syntax error -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:     "url:test:20d",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"http://[::1"},
				syntaxValues:   []string{"parser_error"},
				parseErrValues: []string{"parser_error:malformed_url"},
				completeValues: []string{"false", "true"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "conflicting or malformed",
		},
		{
			name: "Scenario 20e: Malformed canonical_normalization_complete on syntax error -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:     "url:test:20e",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"http://[::1"},
				syntaxValues:   []string{"parser_error"},
				parseErrValues: []string{"parser_error:malformed_url"},
				completeValues: []string{"not-a-bool"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "conflicting or malformed",
		},
		{
			name: "Scenario 20f: Contradictory canonical_normalization_complete=true on syntax error -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:     "url:test:20f",
				urlValues:      []string{"https://example.com/page"},
				extractValues:  []string{"complete"},
				countValues:    []string{"1"},
				rawValues:      []string{"http://[::1"},
				syntaxValues:   []string{"parser_error"},
				parseErrValues: []string{"parser_error:malformed_url"},
				completeValues: []string{"true"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "conflicts with normalization completeness",
		},
		{
			name: "Scenario 21a: Missing subject url_identity -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:    "url:test:21a",
				extractValues: []string{"complete"},
				countValues:   []string{"1"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "URL identity evidence is unavailable",
		},
		{
			name: "Scenario 21b: Conflicting subject url_identity -> UNKNOWN",
			fixture: canon005Fixture{
				subjectRef:    "url:test:21b",
				urlValues:     []string{"https://example.com/a", "https://example.com/b"},
				extractValues: []string{"complete"},
				countValues:   []string{"1"},
			},
			wantStatus:        audit.StatusUnknown,
			wantSummarySubstr: "conflicting URL identity",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := newCANON005Snapshot([]canon005Fixture{tc.fixture}, auditRunID, snapID)
			results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-005")
			if err != nil {
				t.Fatalf("EvaluateRule(AR-CANON-005) failed: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 RuleResult, got %d", len(results))
			}
			rr := results[0]
			if rr.Status != tc.wantStatus {
				t.Errorf("Status = %s, want %s (summary: %q)", rr.Status, tc.wantStatus, rr.ObservedSummary)
			}
			if rr.Status == audit.StatusWarning || rr.Status == audit.StatusManualReview {
				t.Errorf("AR-CANON-005 must never return WARNING or MANUAL_REVIEW, got %s", rr.Status)
			}
			if tc.wantSummarySubstr != "" && !strings.Contains(rr.ObservedSummary, tc.wantSummarySubstr) {
				t.Errorf("ObservedSummary = %q, want substring %q", rr.ObservedSummary, tc.wantSummarySubstr)
			}
			if tc.wantPrimaryField != "" {
				foundPrimary := false
				for _, ref := range rr.EvidenceRefs {
					if ref.Field == tc.wantPrimaryField && ref.Role == audit.EvidenceRolePrimary {
						foundPrimary = true
						break
					}
				}
				if !foundPrimary {
					t.Errorf("expected primary EvidenceRef with Field=%q, got refs=%+v", tc.wantPrimaryField, rr.EvidenceRefs)
				}
			}
		})
	}

	// Scenario 22: Cross-run or invalid subject correlation (AuditRunID / SnapshotID mismatch)
	t.Run("Scenario 22: Cross-run or SnapshotID mismatch rejected by EvidenceIndex", func(t *testing.T) {
		snapMismatchRun := newCANON005Snapshot([]canon005Fixture{
			{
				subjectRef:    "url:test:22",
				urlValues:     []string{"https://example.com/page"},
				extractValues: []string{"complete"},
				countValues:   []string{"1"},
			},
		}, auditRunID, snapID)
		snapMismatchRun.NormalizedObservations[0].AuditRunID = "audit:run:other"

		_, err := eng.EvaluateRule(ctx, snapMismatchRun, "AR-CANON-005")
		if !errors.Is(err, engine.ErrInvalidObservation) {
			t.Errorf("expected ErrInvalidObservation on AuditRunID mismatch, got %v", err)
		}

		snapMismatchSnap := newCANON005Snapshot([]canon005Fixture{
			{
				subjectRef:    "url:test:22",
				urlValues:     []string{"https://example.com/page"},
				extractValues: []string{"complete"},
				countValues:   []string{"1"},
			},
		}, auditRunID, snapID)
		snapMismatchSnap.NormalizedObservations[0].SnapshotID = "snap:other"

		_, err = eng.EvaluateRule(ctx, snapMismatchSnap, "AR-CANON-005")
		if !errors.Is(err, engine.ErrInvalidObservation) {
			t.Errorf("expected ErrInvalidObservation on SnapshotID mismatch, got %v", err)
		}
	})

	// Determinism under reversed observation order
	t.Run("Determinism: reversed observation order produces identical RuleResults", func(t *testing.T) {
		var allFixtures []canon005Fixture
		for _, tc := range cases {
			allFixtures = append(allFixtures, tc.fixture)
		}
		snapFwd := newCANON005Snapshot(allFixtures, auditRunID, snapID)
		snapRev := newCANON005Snapshot(allFixtures, auditRunID, snapID)
		n := len(snapRev.NormalizedObservations)
		for i := 0; i < n/2; i++ {
			snapRev.NormalizedObservations[i], snapRev.NormalizedObservations[n-1-i] = snapRev.NormalizedObservations[n-1-i], snapRev.NormalizedObservations[i]
		}

		resFwd, err := eng.EvaluateRule(ctx, snapFwd, "AR-CANON-005")
		if err != nil {
			t.Fatalf("EvaluateRule fwd failed: %v", err)
		}
		resRev, err := eng.EvaluateRule(ctx, snapRev, "AR-CANON-005")
		if err != nil {
			t.Fatalf("EvaluateRule rev failed: %v", err)
		}
		if !reflect.DeepEqual(resFwd, resRev) {
			t.Errorf("expected deterministic identical results regardless of observation order")
		}
	})
}

func TestAR_CANON_005_HermeticEndToEndPipeline(t *testing.T) {
	mux := http.NewServeMux()
	var srvURL string

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head>
<link rel="canonical" href="%s/">
</head><body>
<a href="/no-canon">No Canon</a>
<a href="/rel-canon">Rel Canon</a>
<a href="/empty-canon">Empty Canon</a>
<a href="/ws-canon">Whitespace Canon</a>
<a href="/malformed-canon">Malformed Canon</a>
<a href="/ftp-base-rel-canon">FTP Base Rel Canon</a>
<a href="/bad-base-rel-canon">Bad Base Rel Canon</a>
<a href="/bad-base-abs-canon">Bad Base Abs Canon</a>
<a href="/empty-base-rel-canon">Empty Base Rel Canon</a>
<a href="/multi-base-rel-canon">Multi Base Rel Canon</a>
<a href="/two-canons">Two Canons</a>
<a href="/six-canons">Six Canons</a>
</body></html>`, srvURL)
	})

	mux.HandleFunc("/no-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><title>No Canon</title></head><body>No Canon</body></html>`)
	})

	mux.HandleFunc("/rel-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="canonical" href="/rel-canon"></head><body>Rel Canon</body></html>`)
	})

	mux.HandleFunc("/empty-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="canonical" href=""></head><body>Empty Canon</body></html>`)
	})

	mux.HandleFunc("/ws-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="canonical" href="   "></head><body>WS Canon</body></html>`)
	})

	mux.HandleFunc("/malformed-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><link rel="canonical" href="http://[::1"></head><body>Malformed Canon</body></html>`)
	})

	mux.HandleFunc("/ftp-base-rel-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><base href="ftp://files.example.com/dir/"><link rel="canonical" href="sub.html"></head><body>FTP Base</body></html>`)
	})

	mux.HandleFunc("/bad-base-rel-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><base href="http://[::1"><link rel="canonical" href="sub.html"></head><body>Bad Base Rel</body></html>`)
	})

	mux.HandleFunc("/bad-base-abs-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><base href="http://[::1"><link rel="canonical" href="%s/bad-base-abs-canon"></head><body>Bad Base Abs</body></html>`, srvURL)
	})

	mux.HandleFunc("/empty-base-rel-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><base href=""><link rel="canonical" href="/empty-base-rel-canon"></head><body>Empty Base Rel</body></html>`)
	})

	mux.HandleFunc("/multi-base-rel-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><base href="%s/a/"><base href="%s/b/"><link rel="canonical" href="sub.html"></head><body>Multi Base Rel</body></html>`, srvURL, srvURL)
	})

	mux.HandleFunc("/two-canons", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><link rel="canonical" href="%s/two-canons"><link rel="canonical" href=""></head><body>Two Canons</body></html>`, srvURL)
	})

	mux.HandleFunc("/six-canons", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head>
<link rel="canonical" href="/rel-canon">
<link rel="canonical" href="/rel-canon">
<link rel="canonical" href="/rel-canon">
<link rel="canonical" href="/rel-canon">
<link rel="canonical" href="/rel-canon">
<link rel="canonical" href="/rel-canon">
</head><body>Six Canons</body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()
	srvURL = srv.URL

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := sitecrawl.NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	summary, err := runner.Crawl(context.Background(), []string{srv.URL + "/"}, sitecrawl.Options{
		Mode:             sitecrawl.ModeSpider,
		MaxDepth:         2,
		MaxURLs:          30,
		Concurrency:      1,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	})
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	// Also insert a legacy SQLite row (no ExtractionStatus / RawCanonicals) to verify legacy handling
	legacyURL := srv.URL + "/legacy-row"
	_, err = db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, 999, ?)`, summary.ID, legacyURL)
	if err != nil {
		t.Fatalf("insert legacy URL: %v", err)
	}
	legacyPageJSON, _ := json.Marshal(sitecrawl.Page{
		URL:        legacyURL,
		Status:     200,
		Kind:       sitecrawl.KindHTML,
		Canonical:  legacyURL,
		Canonicals: []string{legacyURL},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, err = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, status, content_type, canonical, crawled_at)
		VALUES(?, 999, ?, ?, 'html', 200, 'text/html', ?, '2026-10-01T00:00:10Z')`,
		summary.ID, legacyURL, string(legacyPageJSON), legacyURL)
	if err != nil {
		t.Fatalf("insert legacy page: %v", err)
	}

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: summary.ID,
		AuditRunID: "audit:e2e:canon005",
		SnapshotID: "snap:e2e:canon005",
	})
	if err != nil {
		t.Fatalf("adapter.Build: %v", err)
	}

	if res.EvidenceSnapshot.NormalizationVersion != "v1.11.0" {
		t.Errorf("NormalizationVersion = %q, want v1.11.0", res.EvidenceSnapshot.NormalizationVersion)
	}

	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	if got := len(eng.ImplementedRuleIDs()); got != 26 {
		t.Fatalf("ImplementedRuleIDs count = %d, want 26", got)
	}

	subjToURL := make(map[string]string)
	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.SubjectType == audit.SubjectURL && obs.Field == "url_identity" {
			subjToURL[obs.SubjectRef] = obs.Value
		}
	}

	evalByURL := func(ruleID string) map[string]audit.RuleResultStatus {
		rrs, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, ruleID)
		if err != nil {
			t.Fatalf("EvaluateRule(%s): %v", ruleID, err)
		}
		m := make(map[string]audit.RuleResultStatus, len(rrs))
		for _, rr := range rrs {
			if u, ok := subjToURL[rr.SubjectRef]; ok {
				m[u] = rr.Status
			}
		}
		return m
	}

	c005 := evalByURL("AR-CANON-005")
	wantC005 := map[string]audit.RuleResultStatus{
		srv.URL + "/":                     audit.StatusPass,
		srv.URL + "/no-canon":             audit.StatusNotApplicable,
		srv.URL + "/rel-canon":            audit.StatusPass,
		srv.URL + "/empty-canon":          audit.StatusFail,
		srv.URL + "/ws-canon":             audit.StatusFail,
		srv.URL + "/malformed-canon":      audit.StatusFail,
		srv.URL + "/ftp-base-rel-canon":   audit.StatusFail,
		srv.URL + "/bad-base-rel-canon":   audit.StatusUnknown,
		srv.URL + "/bad-base-abs-canon":   audit.StatusPass,
		srv.URL + "/empty-base-rel-canon": audit.StatusPass,
		srv.URL + "/multi-base-rel-canon": audit.StatusUnknown,
		srv.URL + "/two-canons":           audit.StatusNotApplicable,
		srv.URL + "/six-canons":           audit.StatusNotApplicable,
		legacyURL:                         audit.StatusUnknown,
	}
	for u, want := range wantC005 {
		if got, ok := c005[u]; !ok || got != want {
			t.Errorf("AR-CANON-005(%s) = %s (present=%v), want %s", u, got, ok, want)
		}
	}

	// Verify AR-CANON-003/004/006/007 regression on the same end-to-end snapshot
	c003 := evalByURL("AR-CANON-003")
	if c003[srv.URL+"/empty-canon"] != audit.StatusWarning {
		t.Errorf("AR-CANON-003(/empty-canon) = %s, want WARNING", c003[srv.URL+"/empty-canon"])
	}
	if c003[srv.URL+"/rel-canon"] != audit.StatusPass {
		t.Errorf("AR-CANON-003(/rel-canon) = %s, want PASS", c003[srv.URL+"/rel-canon"])
	}

	c004 := evalByURL("AR-CANON-004")
	if c004[srv.URL+"/empty-canon"] != audit.StatusPass {
		t.Errorf("AR-CANON-004(/empty-canon) = %s, want PASS", c004[srv.URL+"/empty-canon"])
	}
	if c004[srv.URL+"/two-canons"] != audit.StatusUnknown {
		t.Errorf("AR-CANON-004(/two-canons) = %s, want UNKNOWN", c004[srv.URL+"/two-canons"])
	}
	if c004[srv.URL+"/six-canons"] != audit.StatusUnknown {
		t.Errorf("AR-CANON-004(/six-canons) = %s, want UNKNOWN", c004[srv.URL+"/six-canons"])
	}

	c006 := evalByURL("AR-CANON-006")
	if c006[srv.URL+"/rel-canon"] != audit.StatusPass {
		t.Errorf("AR-CANON-006(/rel-canon) = %s, want PASS", c006[srv.URL+"/rel-canon"])
	}
	if c006[srv.URL+"/empty-canon"] != audit.StatusUnknown {
		t.Errorf("AR-CANON-006(/empty-canon) = %s, want UNKNOWN", c006[srv.URL+"/empty-canon"])
	}

	c007 := evalByURL("AR-CANON-007")
	if c007[srv.URL+"/rel-canon"] != audit.StatusPass {
		t.Errorf("AR-CANON-007(/rel-canon) = %s, want PASS", c007[srv.URL+"/rel-canon"])
	}
	if c007[srv.URL+"/empty-canon"] != audit.StatusNotApplicable {
		t.Errorf("AR-CANON-007(/empty-canon) = %s, want NOT_APPLICABLE", c007[srv.URL+"/empty-canon"])
	}
}
