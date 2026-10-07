package engine_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

type directiveFixture struct {
	refID        string
	source       string
	target       string
	raw          string
	tokens       string
	scopeUnknown bool
}

func newSnapshotWithDirectives(
	subjectRef string,
	urlStr string,
	directives []directiveFixture,
) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	obs := []audit.NormalizedObservation{}

	if urlStr != "" {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:url:" + subjectRef),
			AuditRunID:         "audit:run:index002",
			SnapshotID:         "snap:run:index002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "url_identity",
			Value:              urlStr,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		})
	}

	for i, d := range directives {
		refs := []string{d.refID, "page:test"}

		if d.source != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:src:%s:%d", subjectRef, i)),
				AuditRunID:         "audit:run:index002",
				SnapshotID:         "snap:run:index002",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         subjectRef,
				Field:              "directive_source",
				Value:              d.source,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: refs,
				ObservedAt:         now,
			})
		}

		// target observation can be empty string if unknown
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tgt:%s:%d", subjectRef, i)),
			AuditRunID:         "audit:run:index002",
			SnapshotID:         "snap:run:index002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              "directive_target",
			Value:              d.target,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: refs,
			ObservedAt:         now,
		})

		if d.raw != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:raw:%s:%d", subjectRef, i)),
				AuditRunID:         "audit:run:index002",
				SnapshotID:         "snap:run:index002",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         subjectRef,
				Field:              "directive_raw",
				Value:              d.raw,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: refs,
				ObservedAt:         now,
			})
		}

		if d.tokens != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:tok:%s:%d", subjectRef, i)),
				AuditRunID:         "audit:run:index002",
				SnapshotID:         "snap:run:index002",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         subjectRef,
				Field:              "directive_tokens",
				Value:              d.tokens,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: refs,
				ObservedAt:         now,
			})
		}

		if d.scopeUnknown {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:unk:%s:%d", subjectRef, i)),
				AuditRunID:         "audit:run:index002",
				SnapshotID:         "snap:run:index002",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         subjectRef,
				Field:              "directive_scope_unknown",
				Value:              "true",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: refs,
				ObservedAt:         now,
			})
		}
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:index002",
		AuditRunID:             "audit:run:index002",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.2.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestAR_INDEX_002_RequiredCases(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	t.Run("1. no directive -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", nil)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusNotApplicable {
			t.Errorf("expected NOT_APPLICABLE, got %s", r.Status)
		}
		if r.ObservedSummary != "No robots directive exists." {
			t.Errorf("unexpected summary: %s", r.ObservedSummary)
		}
		// Evidence refs: only URL context
		if len(r.EvidenceRefs) != 1 || r.EvidenceRefs[0].Role != audit.EvidenceRoleContext {
			t.Errorf("expected only URL context evidence ref, got %v", r.EvidenceRefs)
		}
	})

	t.Run("2. generic index -> PASS", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index, follow",
				tokens: "index, follow",
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", r.Status)
		}
		if len(r.EvidenceRefs) < 2 {
			t.Errorf("expected URL context + primary directive evidence, got %d refs", len(r.EvidenceRefs))
		}
	})

	t.Run("3. generic noindex -> PASS", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:header:0",
				source: "HTTP_HEADER",
				target: "*",
				raw:    "noindex",
				tokens: "noindex",
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", r.Status)
		}
	})

	t.Run("4. same generic directive contains index, noindex -> FAIL", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index, noindex",
				tokens: "index, noindex",
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", r.Status)
		}
		// Primary evidence must contain the deciding observations
		hasTokensPrimary := false
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "directive_tokens" && ref.Role == audit.EvidenceRolePrimary {
				hasTokensPrimary = true
			}
		}
		if !hasTokensPrimary {
			t.Errorf("expected directive_tokens primary evidence ref")
		}
	})

	t.Run("5. META generic index + X-Robots generic noindex -> FAIL", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index",
				tokens: "index",
			},
			{
				refID:  "directive:header:0",
				source: "HTTP_HEADER",
				target: "*",
				raw:    "noindex",
				tokens: "noindex",
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", r.Status)
		}
		// Evidence refs must include observations from both meta and header
		seenRefs := make(map[string]bool)
		for _, ref := range r.EvidenceRefs {
			seenRefs[ref.ObservedValue] = true
		}
		if !seenRefs["index"] || !seenRefs["noindex"] {
			t.Errorf("expected both index and noindex in evidence refs, got %v", seenRefs)
		}
	})

	t.Run("6. generic index + googlebot noindex -> PASS", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index",
				tokens: "index",
			},
			{
				refID:  "directive:meta:1",
				source: "META",
				target: "googlebot",
				raw:    "noindex",
				tokens: "noindex",
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS (different scopes), got %s", r.Status)
		}
	})

	t.Run("7. googlebot index + googlebot noindex -> FAIL", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "googlebot",
				raw:    "index",
				tokens: "index",
			},
			{
				refID:  "directive:header:0",
				source: "HTTP_HEADER",
				target: "googlebot",
				raw:    "noindex",
				tokens: "noindex",
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", r.Status)
		}
	})

	t.Run("8. unknown-scope directive, no confirmed conflict -> UNKNOWN", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:        "directive:meta:0",
				source:       "META",
				target:       "",
				raw:          "noindex",
				tokens:       "noindex",
				scopeUnknown: true,
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", r.Status)
		}
		// UNKNOWN must reference ambiguous directive evidence
		hasAmbiguousRef := false
		for _, ref := range r.EvidenceRefs {
			if ref.Role == audit.EvidenceRolePrimary {
				hasAmbiguousRef = true
			}
		}
		if !hasAmbiguousRef {
			t.Errorf("expected ambiguous primary evidence ref")
		}
	})

	t.Run("9. confirmed conflict + unrelated unknown-scope directive -> FAIL", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index",
				tokens: "index",
			},
			{
				refID:  "directive:meta:1",
				source: "META",
				target: "*",
				raw:    "noindex",
				tokens: "noindex",
			},
			{
				refID:        "directive:header:0",
				source:       "HTTP_HEADER",
				target:       "",
				raw:          "unrelated",
				tokens:       "unrelated",
				scopeUnknown: true,
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Errorf("expected FAIL (conflict takes precedence over ambiguous evidence), got %s", r.Status)
		}
		// Confirm unrelated unknown directive is NOT in primary evidence
		for _, ref := range r.EvidenceRefs {
			if ref.ObservedValue == "unrelated" {
				t.Errorf("unrelated unknown directive observation should not be in FAIL evidence refs")
			}
		}
	})

	t.Run("10. missing/conflicting URL identity -> UNKNOWN", func(t *testing.T) {
		// 10a. missing URL identity
		snapMissing := newSnapshotWithDirectives("url:1", "", []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index, noindex",
				tokens: "index, noindex",
			},
		})
		results, err := eng.EvaluateRule(ctx, snapMissing, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for missing URL identity, got %s", results[0].Status)
		}

		// 10b. conflicting URL identity
		snapConflict := newSnapshotWithDirectives("url:1", "https://example.com/a", nil)
		// add second conflicting URL observation
		now := time.Now().UTC()
		snapConflict.NormalizedObservations = append(snapConflict.NormalizedObservations, audit.NormalizedObservation{
			ObservationID:      "obs:url:conflict",
			AuditRunID:         "audit:run:index002",
			SnapshotID:         "snap:run:index002",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:1",
			Field:              "url_identity",
			Value:              "https://example.com/b",
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		})
		results, err = eng.EvaluateRule(ctx, snapConflict, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if results[0].Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting URL identity, got %s", results[0].Status)
		}
	})

	t.Run("11. reversed observation order -> identical semantic result", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index",
				tokens: "index",
			},
			{
				refID:  "directive:header:0",
				source: "HTTP_HEADER",
				target: "*",
				raw:    "noindex",
				tokens: "noindex",
			},
			{
				refID:  "directive:meta:1",
				source: "META",
				target: "googlebot",
				raw:    "noindex",
				tokens: "noindex",
			},
		}

		snapForward := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)

		// Create reversed snapshot
		snapReversed := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		revObs := make([]audit.NormalizedObservation, len(snapReversed.NormalizedObservations))
		for i, o := range snapReversed.NormalizedObservations {
			revObs[len(snapReversed.NormalizedObservations)-1-i] = o
		}
		snapReversed.NormalizedObservations = revObs

		resForward, err := eng.EvaluateRule(ctx, snapForward, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("forward failed: %v", err)
		}
		resReversed, err := eng.EvaluateRule(ctx, snapReversed, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("reversed failed: %v", err)
		}

		rf := resForward[0]
		rr := resReversed[0]

		if rf.Status != rr.Status {
			t.Errorf("status mismatch: %s vs %s", rf.Status, rr.Status)
		}
		if rf.ObservedSummary != rr.ObservedSummary {
			t.Errorf("summary mismatch: %q vs %q", rf.ObservedSummary, rr.ObservedSummary)
		}
		if len(rf.EvidenceRefs) != len(rr.EvidenceRefs) {
			t.Fatalf("evidence refs count mismatch: %d vs %d", len(rf.EvidenceRefs), len(rr.EvidenceRefs))
		}
		for i := range rf.EvidenceRefs {
			if !reflect.DeepEqual(rf.EvidenceRefs[i], rr.EvidenceRefs[i]) {
				t.Errorf("evidence ref %d mismatch:\nforward:  %+v\nreversed: %+v", i, rf.EvidenceRefs[i], rr.EvidenceRefs[i])
			}
		}
	})

	t.Run("12. evidence refs contain the deciding observations", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index",
				tokens: "index",
			},
			{
				refID:  "directive:meta:1",
				source: "META",
				target: "*",
				raw:    "noindex",
				tokens: "noindex",
			},
			{
				refID:  "directive:meta:2",
				source: "META",
				target: "*",
				raw:    "follow",
				tokens: "follow", // not deciding
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %s", r.Status)
		}

		foundIndexToken := false
		foundNoindexToken := false
		foundFollowToken := false

		for _, ref := range r.EvidenceRefs {
			if ref.Field == "directive_tokens" {
				if ref.ObservedValue == "index" {
					foundIndexToken = true
				}
				if ref.ObservedValue == "noindex" {
					foundNoindexToken = true
				}
				if ref.ObservedValue == "follow" {
					foundFollowToken = true
				}
			}
		}

		if !foundIndexToken {
			t.Errorf("expected deciding index token observation in evidence refs")
		}
		if !foundNoindexToken {
			t.Errorf("expected deciding noindex token observation in evidence refs")
		}
		if foundFollowToken {
			t.Errorf("non-deciding follow token observation should NOT be in FAIL evidence refs")
		}
	})

	t.Run("token semantics: do not reinterpret none as noindex", func(t *testing.T) {
		// generic index + generic none -> PASS (none is not reinterpreted as noindex)
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index",
				tokens: "index",
			},
			{
				refID:  "directive:meta:1",
				source: "META",
				target: "*",
				raw:    "none",
				tokens: "none",
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS (none must not be reinterpreted as noindex in AR-INDEX-002), got %s", r.Status)
		}
	})

	t.Run("token semantics: do not reinterpret nofollow or noarchive", func(t *testing.T) {
		directives := []directiveFixture{
			{
				refID:  "directive:meta:0",
				source: "META",
				target: "*",
				raw:    "index, nofollow, noarchive",
				tokens: "index, nofollow, noarchive",
			},
		}
		snap := newSnapshotWithDirectives("url:1", "https://example.com/test", directives)
		results, err := eng.EvaluateRule(ctx, snap, "AR-INDEX-002")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", r.Status)
		}
	})
}

func TestAR_INDEX_002_MetadataAndGuards(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	snap := newSnapshotWithDirectives("url:meta", "https://example.com/test", []directiveFixture{
		{
			refID:  "directive:meta:0",
			source: "META",
			target: "*",
			raw:    "index",
			tokens: "index",
		},
	})

	results, err := eng.EvaluateRule(context.Background(), snap, "AR-INDEX-002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := results[0]

	if r.RuleID != "AR-INDEX-002" {
		t.Errorf("expected RuleID AR-INDEX-002, got %q", r.RuleID)
	}
	if r.ParentCheck != "INDEX-001" {
		t.Errorf("expected ParentCheck INDEX-001, got %q", r.ParentCheck)
	}
	if r.RuleVersion != 1 {
		t.Errorf("expected RuleVersion 1, got %d", r.RuleVersion)
	}
	if r.Severity != audit.SeverityP1 {
		t.Errorf("expected Severity P1, got %q", r.Severity)
	}
	if r.SubjectType != audit.SubjectURL {
		t.Errorf("expected SubjectType URL, got %q", r.SubjectType)
	}
	if r.Scope != audit.ScopeURL {
		t.Errorf("expected Scope URL, got %q", r.Scope)
	}
	if r.ExpectedSummary != "No contradictory index and noindex directives in the same applicable scope." {
		t.Errorf("unexpected ExpectedSummary: %q", r.ExpectedSummary)
	}
}
