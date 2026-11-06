package engine_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForINDEX003(observations []audit.NormalizedObservation) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:index003",
		AuditRunID:             "audit:run:index003",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: observations,
	}
}

func TestAR_INDEX_003_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	subjectRef := "url:index003:1"
	testURL := "https://example.com/test"

	makeObs := func(field, val string, refs ...string) audit.NormalizedObservation {
		srcRefs := []string{"sitecrawl_pages:test"}
		if len(refs) > 0 {
			srcRefs = refs
		}
		return audit.NormalizedObservation{
			ObservationID:      audit.ObservationID(fmt.Sprintf("obs:%s:%s:%d", field, strings.ReplaceAll(val, " ", "_"), time.Now().UnixNano())),
			AuditRunID:         "audit:run:index003",
			SnapshotID:         "snap:run:index003",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              field,
			Value:              val,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: srcRefs,
			ObservedAt:         now,
		}
	}

	baseCompleteObs := func() []audit.NormalizedObservation {
		return []audit.NormalizedObservation{
			makeObs("url_identity", testURL),
			makeObs("http_status", "200"),
			makeObs("content_type", "text/html"),
			makeObs("rendered", "false"),
			makeObs("http_header_status", "complete"),
			makeObs("x_robots_sample_truncated", "false"),
			makeObs("html_extraction_status", "complete"),
			makeObs("robots_meta_sample_truncated", "false"),
		}
	}

	makeBundle := func(dirRef, source, raw, tokens, parseErrors string, unsupported string, scopeUnknown string) []audit.NormalizedObservation {
		var bundle []audit.NormalizedObservation
		bundle = append(bundle, makeObs("directive_source", source, dirRef))
		bundle = append(bundle, makeObs("directive_raw", raw, dirRef))
		bundle = append(bundle, makeObs("robots_directive_tokens", tokens, dirRef))
		bundle = append(bundle, makeObs("robots_parse_errors", parseErrors, dirRef))
		if unsupported != "" {
			bundle = append(bundle, makeObs("directive_unsupported_tokens", unsupported, dirRef))
		}
		if scopeUnknown != "" {
			bundle = append(bundle, makeObs("directive_scope_unknown", scopeUnknown, dirRef))
		}
		return bundle
	}

	t.Run("P1 Tier A: Conflicting URL identities -> UNKNOWN", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs("url_identity", "https://example.com/a"),
			makeObs("url_identity", "https://example.com/b"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForINDEX003(obs), "AR-INDEX-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected StatusUnknown, got %s", res[0].Status)
		}
	})

	t.Run("P2 Tier C: Malformed directive (syntax parse error) -> WARNING", func(t *testing.T) {
		obs := baseCompleteObs()
		dirRef := "directive:meta:0"
		obs = append(obs, makeBundle(dirRef, "META", "max-snippet:invalid", "max-snippet:invalid", "parse_error:invalid_integer_max_snippet", "", "")...)

		res, err := eng.EvaluateRule(ctx, newSnapshotForINDEX003(obs), "AR-INDEX-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if res[0].Status != audit.StatusWarning {
			t.Errorf("expected StatusWarning, got %s", res[0].Status)
		}
		if res[0].Severity != audit.SeverityP1 {
			t.Errorf("expected SeverityP1, got %s", res[0].Severity)
		}
		if !strings.Contains(res[0].ObservedSummary, "Malformed robots directive") {
			t.Errorf("expected summary to indicate malformed directive, got %q", res[0].ObservedSummary)
		}
	})

	t.Run("P2 Tier C: Unsupported directive token -> WARNING", func(t *testing.T) {
		obs := baseCompleteObs()
		dirRef := "directive:meta:0"
		obs = append(obs, makeBundle(dirRef, "META", "indexifembedded", "indexifembedded", "unsupported_directive:indexifembedded", "indexifembedded", "")...)

		res, err := eng.EvaluateRule(ctx, newSnapshotForINDEX003(obs), "AR-INDEX-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if res[0].Status != audit.StatusWarning {
			t.Errorf("expected StatusWarning, got %s", res[0].Status)
		}
		if !strings.Contains(res[0].ObservedSummary, "Unsupported robots directive") {
			t.Errorf("expected summary to indicate unsupported directive, got %q", res[0].ObservedSummary)
		}
	})

	t.Run("P2 Tier C: Short-circuiting P2 even if channel 2 is incomplete -> WARNING", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs("url_identity", testURL),
			makeObs("http_status", "200"),
			makeObs("content_type", "text/html"),
			makeObs("rendered", "false"),
			makeObs("http_header_status", "complete"),
			makeObs("x_robots_sample_truncated", "false"),
			makeObs("html_extraction_status", "truncated"), // Incomplete HTML!
			makeObs("robots_meta_sample_truncated", "true"),
		}
		dirRef := "directive:header:0"
		obs = append(obs, makeBundle(dirRef, "HTTP_HEADER", "max-image-preview:huge", "max-image-preview:huge", "parse_error:invalid_enum_max_image_preview", "", "")...)

		res, err := eng.EvaluateRule(ctx, newSnapshotForINDEX003(obs), "AR-INDEX-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if res[0].Status != audit.StatusWarning {
			t.Errorf("expected StatusWarning short-circuit, got %s", res[0].Status)
		}
	})

	t.Run("P3 Tier D: Ambiguous scope token -> UNKNOWN", func(t *testing.T) {
		obs := baseCompleteObs()
		dirRef := "directive:header:0"
		obs = append(obs, makeBundle(dirRef, "HTTP_HEADER", "googlebot:noindex", "noindex", "ambiguous:unsupported_prefix", "", "true")...)

		res, err := eng.EvaluateRule(ctx, newSnapshotForINDEX003(obs), "AR-INDEX-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected StatusUnknown, got %s", res[0].Status)
		}
		if !strings.Contains(res[0].ObservedSummary, "scope syntax cannot be reliably verified") {
			t.Errorf("expected summary to note ambiguous scope, got %q", res[0].ObservedSummary)
		}
	})

	t.Run("P3 Tier D: Incomplete HTTP headers -> UNKNOWN", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs("url_identity", testURL),
			makeObs("http_status", "200"),
			makeObs("content_type", "text/html"),
			makeObs("rendered", "false"),
			makeObs("http_header_status", "truncated"), // Truncated headers!
			makeObs("x_robots_sample_truncated", "true"),
			makeObs("html_extraction_status", "complete"),
			makeObs("robots_meta_sample_truncated", "false"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForINDEX003(obs), "AR-INDEX-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected StatusUnknown, got %s", res[0].Status)
		}
	})

	t.Run("P4 Tier E.1: Complete channels & 0 directives -> NOT_APPLICABLE", func(t *testing.T) {
		obs := baseCompleteObs()
		res, err := eng.EvaluateRule(ctx, newSnapshotForINDEX003(obs), "AR-INDEX-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected StatusNotApplicable, got %s", res[0].Status)
		}
	})

	t.Run("P5 Tier E.2: Complete channels & all directives recognized and valid -> PASS", func(t *testing.T) {
		obs := baseCompleteObs()
		dirRef := "directive:meta:0"
		obs = append(obs, makeBundle(dirRef, "META", "noindex, follow", "noindex,follow", "none", "", "false")...)

		res, err := eng.EvaluateRule(ctx, newSnapshotForINDEX003(obs), "AR-INDEX-003")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if res[0].Status != audit.StatusPass {
			t.Errorf("expected StatusPass, got %s", res[0].Status)
		}
		if !strings.Contains(res[0].ObservedSummary, "recognized and valid") {
			t.Errorf("expected summary to note valid and recognized directives, got %q", res[0].ObservedSummary)
		}
	})
}
