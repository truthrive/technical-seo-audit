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

type canon003TestFixture struct {
	subjectRef      string
	urlValue        string
	urlValues       []string
	statusValue     string
	statusValues    []string
	contentType     string
	contentTypes    []string
	renderedValue   string
	renderedValues  []string
	canonicalValues []string
}

func newCANON003Snapshot(fixtures []canon003TestFixture, auditRunID audit.AuditRunID, snapID audit.SnapshotID) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	for _, f := range fixtures {
		// 1. URL identity
		if len(f.urlValues) > 0 {
			for i, u := range f.urlValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:url:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "url_identity",
					Value:              u,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.urlValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:url:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "url_identity",
				Value:              f.urlValue,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// 2. HTTP status
		if len(f.statusValues) > 0 {
			for i, s := range f.statusValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:status:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "http_status",
					Value:              s,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.statusValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:status:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "http_status",
				Value:              f.statusValue,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// 3. Content-Type
		if len(f.contentTypes) > 0 {
			for i, ct := range f.contentTypes {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:ct:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "content_type",
					Value:              ct,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.contentType != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:ct:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "content_type",
				Value:              f.contentType,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// 4. Rendered
		if len(f.renderedValues) > 0 {
			for i, rv := range f.renderedValues {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:rendered:%s:%d", f.subjectRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         f.subjectRef,
					Field:              "rendered",
					Value:              rv,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
					ObservedAt:         now,
				})
			}
		} else if f.renderedValue != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:rendered:%s", f.subjectRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "rendered",
				Value:              f.renderedValue,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
				ObservedAt:         now,
			})
		}

		// 5. Canonical observations
		for i, cv := range f.canonicalValues {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:canon:%s:%d", f.subjectRef, i)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         f.subjectRef,
				Field:              "canonical_resolved",
				Value:              cv,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{"page:test"},
				ObservedAt:         now,
			})
		}
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             snapID,
		AuditRunID:             auditRunID,
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.4.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

// Tests covering all contract requirements for AR-CANON-003:
// 1. 200 HTML + one canonical -> PASS
// 2. 200 HTML + no canonical -> WARNING
// 3. 200 HTML + multiple canonicals -> PASS
// 4. 200 non-HTML -> NOT_APPLICABLE
// 5. 301 -> NOT_APPLICABLE
// 6. 404 -> NOT_APPLICABLE
// 7. missing status -> UNKNOWN
// 8. malformed status -> UNKNOWN
// 9. conflicting status -> UNKNOWN
// 10. 200 + missing content type -> UNKNOWN
// 11. conflicting content type -> UNKNOWN
// 12. rendered 200 HTML + no raw canonical -> UNKNOWN
// 13. missing URL identity -> UNKNOWN
// 14. conflicting URL identity -> UNKNOWN
// 15. reversed observation order -> identical semantic result
// 16. PASS references canonical evidence
// 17. WARNING contains no synthetic evidence
func TestEngine_AR_CANON_003(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	ctx := context.Background()
	runID := audit.AuditRunID("audit:run:canon003")
	snapID := audit.SnapshotID("snap:run:canon003")

	t.Run("1. 200 HTML + one canonical -> PASS", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/page",
				statusValue:     "200",
				contentType:     "text/html; charset=utf-8",
				canonicalValues: []string{"https://example.com/page"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", r.Status)
		}
		if r.Severity != audit.SeverityP1 {
			t.Errorf("expected P1 severity, got %s", r.Severity)
		}
		if r.ParentCheck != "CANON-003" {
			t.Errorf("expected parent_check CANON-003, got %q", r.ParentCheck)
		}
	})

	t.Run("2. 200 HTML + no canonical -> WARNING", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/no-canon",
				statusValue:     "200",
				contentType:     "text/html",
				canonicalValues: nil,
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusWarning {
			t.Errorf("expected WARNING, got %s", r.Status)
		}
	})

	t.Run("3. 200 HTML + multiple canonicals -> PASS", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/multi",
				statusValue:     "200",
				contentType:     "application/xhtml+xml",
				canonicalValues: []string{"https://example.com/a", "https://example.com/b"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s", r.Status)
		}
		// Multiplicity is not evaluated here: multiple still counts as present
	})

	t.Run("4. 200 non-HTML -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/doc.pdf",
				statusValue:     "200",
				contentType:     "application/pdf",
				canonicalValues: []string{"https://example.com/doc.pdf"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
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
	})

	t.Run("5. 301 -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/redirect",
				statusValue:     "301",
				contentType:     "text/html",
				canonicalValues: nil,
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
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
	})

	t.Run("6. 404 -> NOT_APPLICABLE", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/missing",
				statusValue:     "404",
				contentType:     "text/html",
				canonicalValues: nil,
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
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
	})

	t.Run("7. missing status -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/no-status",
				statusValue:     "",
				contentType:     "text/html",
				canonicalValues: []string{"https://example.com/no-status"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", r.Status)
		}
	})

	t.Run("8. malformed status -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/bad-status",
				statusValue:     "invalid_status",
				contentType:     "text/html",
				canonicalValues: []string{"https://example.com/bad-status"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", r.Status)
		}
	})

	t.Run("9. conflicting status -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/conflict-status",
				statusValues:    []string{"200", "500"},
				contentType:     "text/html",
				canonicalValues: []string{"https://example.com/conflict-status"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", r.Status)
		}
	})

	t.Run("10. 200 + missing content type -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/no-ct",
				statusValue:     "200",
				contentType:     "",
				canonicalValues: []string{"https://example.com/no-ct"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", r.Status)
		}
	})

	t.Run("11. conflicting content type -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/conflict-ct",
				statusValue:     "200",
				contentTypes:    []string{"text/html", "application/pdf"},
				canonicalValues: []string{"https://example.com/conflict-ct"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN, got %s", r.Status)
		}
	})

	t.Run("12. rendered 200 HTML + no raw canonical -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/rendered-page",
				statusValue:     "200",
				contentType:     "text/html; charset=utf-8",
				renderedValue:   "true",
				canonicalValues: nil,
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for rendered page without canonical, got %s", r.Status)
		}
		// Ensure rendered context evidence is present
		hasRenderedRef := false
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "rendered" && ref.Role == audit.EvidenceRoleContext {
				hasRenderedRef = true
			}
		}
		if !hasRenderedRef {
			t.Errorf("expected rendered context evidence reference")
		}
	})

	t.Run("12b. rendered 200 HTML + canonical present -> PASS", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/rendered-page-with-canon",
				statusValue:     "200",
				contentType:     "text/html; charset=utf-8",
				renderedValue:   "true",
				canonicalValues: []string{"https://example.com/canonical"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS when canonical present even if rendered, got %s", r.Status)
		}
	})

	t.Run("12c. conflicting rendered value + no canonical -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/rendered-conflict",
				statusValue:     "200",
				contentType:     "text/html",
				renderedValues:  []string{"true", "false"},
				canonicalValues: nil,
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting rendered state without canonical, got %s", r.Status)
		}
	})

	t.Run("13. missing URL identity -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "",
				statusValue:     "200",
				contentType:     "text/html",
				canonicalValues: []string{"https://example.com/"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for missing URL identity, got %s", r.Status)
		}
	})

	t.Run("14. conflicting URL identity -> UNKNOWN", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValues:       []string{"https://example.com/a", "https://example.com/b"},
				statusValue:     "200",
				contentType:     "text/html",
				canonicalValues: []string{"https://example.com/a"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for conflicting URL identity, got %s", r.Status)
		}
	})

	t.Run("14b. identical duplicate URL identity -> PASS", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValues:       []string{"https://example.com/page", "https://example.com/page"},
				statusValue:     "200",
				contentType:     "text/html",
				canonicalValues: []string{"https://example.com/page"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS with identical duplicate URL identity, got %s", r.Status)
		}
	})

	t.Run("15. reversed observation order -> identical semantic result", func(t *testing.T) {
		snapFwd := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/page",
				statusValue:     "200",
				contentType:     "text/html",
				canonicalValues: []string{"https://example.com/page"},
			},
		}, runID, snapID)

		// Create reversed observations slice
		snapRev := &audit.EvidenceSnapshot{
			SnapshotID:           snapID,
			AuditRunID:           runID,
			SnapshotStatus:       audit.SnapshotFrozen,
			NormalizationVersion: "v1.4.0",
			CrawlComplete:        true,
			CreatedAt:            snapFwd.CreatedAt,
			FrozenAt:             snapFwd.FrozenAt,
		}
		for i := len(snapFwd.NormalizedObservations) - 1; i >= 0; i-- {
			snapRev.NormalizedObservations = append(snapRev.NormalizedObservations, snapFwd.NormalizedObservations[i])
		}

		fixedTime := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
		eng.SetClock(func() time.Time { return fixedTime })

		resFwd, err := eng.EvaluateRule(ctx, snapFwd, "AR-CANON-003")
		if err != nil {
			t.Fatalf("eval fwd failed: %v", err)
		}
		resRev, err := eng.EvaluateRule(ctx, snapRev, "AR-CANON-003")
		if err != nil {
			t.Fatalf("eval rev failed: %v", err)
		}

		if !reflect.DeepEqual(resFwd, resRev) {
			t.Errorf("reversed observations produced differing results: fwd %+v != rev %+v", resFwd, resRev)
		}
	})

	t.Run("16. PASS references canonical evidence", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/canonical-test",
				statusValue:     "200",
				contentType:     "text/html",
				canonicalValues: []string{"https://example.com/canonical-target"},
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %s", r.Status)
		}

		var (
			hasURLContext   bool
			hasStatusContext bool
			hasCTContext    bool
			hasCanonPrimary bool
		)
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "url" && ref.Role == audit.EvidenceRoleContext {
				hasURLContext = true
			}
			if ref.Field == "fetch_status" && ref.Role == audit.EvidenceRoleContext {
				hasStatusContext = true
			}
			if ref.Field == "content_type" && ref.Role == audit.EvidenceRoleContext {
				hasCTContext = true
			}
			if ref.Field == "canonical_resolved" && ref.Role == audit.EvidenceRolePrimary {
				hasCanonPrimary = true
				if ref.ObservedValue != "https://example.com/canonical-target" {
					t.Errorf("expected canonical target observed value, got %q", ref.ObservedValue)
				}
			}
		}

		if !hasURLContext {
			t.Errorf("missing url context reference")
		}
		if !hasStatusContext {
			t.Errorf("missing status context reference")
		}
		if !hasCTContext {
			t.Errorf("missing content_type context reference")
		}
		if !hasCanonPrimary {
			t.Errorf("missing canonical_resolved primary reference")
		}
	})

	t.Run("17. WARNING contains no synthetic evidence", func(t *testing.T) {
		snap := newCANON003Snapshot([]canon003TestFixture{
			{
				subjectRef:      "url:1",
				urlValue:        "https://example.com/warning-test",
				statusValue:     "200",
				contentType:     "text/html",
				canonicalValues: nil,
			},
		}, runID, snapID)

		results, err := eng.EvaluateRule(ctx, snap, "AR-CANON-003")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusWarning {
			t.Fatalf("expected WARNING, got %s", r.Status)
		}

		// Ensure all referenced evidence corresponds to real normalized observations
		obsIDs := make(map[string]bool)
		for _, o := range snap.NormalizedObservations {
			obsIDs[string(o.ObservationID)] = true
		}

		for _, ref := range r.EvidenceRefs {
			if !obsIDs[ref.EvidenceRef] {
				t.Errorf("evidence ref %s points to non-existent or synthetic observation ID %q", ref.RuleEvidenceRefID, ref.EvidenceRef)
			}
			if ref.Field == "canonical_resolved" {
				t.Errorf("WARNING must not include canonical_resolved evidence ref: got %v", ref)
			}
			if ref.Role != audit.EvidenceRoleContext {
				t.Errorf("WARNING should only contain context refs, got role %s for field %s", ref.Role, ref.Field)
			}
		}
	})
}

// Hermetic crawl -> adapter -> engine integration test for AR-CANON-003.
func TestEngine_AR_CANON_003_HermeticIntegration(t *testing.T) {
	mux := http.NewServeMux()

	// 1. Page with canonical -> PASS
	mux.HandleFunc("/canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Canon</title>
<link rel="canonical" href="/canon" />
</head><body><nav><a href="/no-canon">No Canon</a> <a href="/not-html">Not HTML</a></nav><h1>Canonical Present</h1></body></html>`)
	})

	// 2. Page without canonical -> WARNING
	mux.HandleFunc("/no-canon", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head><title>No Canon</title></head><body><h1>Missing Canonical</h1></body></html>`)
	})

	// 3. Non-HTML page -> NOT_APPLICABLE
	mux.HandleFunc("/not-html", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "Plain text content")
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	// 1. Crawl via SiteCrawl
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	runner := sitecrawl.NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("failed to ensure schema: %v", err)
	}

	opts := sitecrawl.Options{
		Mode:        sitecrawl.ModeSpider,
		MaxDepth:    2,
		MaxURLs:     10,
		Concurrency: 1,
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL + "/canon"}, opts)
	if err != nil {
		t.Fatalf("crawl execution failed: %v", err)
	}

	// 2. Adapt to Frozen EvidenceSnapshot
	buildReq := adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:run:hermetic:canon003",
		SnapshotID: "snap:run:hermetic:canon003",
	}

	buildRes, err := adapter.Build(context.Background(), db, buildReq)
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := buildRes.EvidenceSnapshot
	if snap == nil || snap.SnapshotStatus != audit.SnapshotFrozen {
		t.Fatalf("expected non-nil FROZEN snapshot")
	}

	// 3. Initialize Rule Engine and evaluate AR-CANON-003
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New failed: %v", err)
	}

	results, err := eng.EvaluateRule(context.Background(), snap, "AR-CANON-003")
	if err != nil {
		t.Fatalf("EvaluateRule AR-CANON-003 failed: %v", err)
	}

	if len(results) < 3 {
		t.Fatalf("expected at least 3 RuleResults, got %d", len(results))
	}

	// Map URL string to rule status
	resultsByURL := make(map[string]audit.RuleResult)
	for _, r := range results {
		for _, ref := range r.EvidenceRefs {
			if ref.Field == "url" {
				resultsByURL[ref.ObservedValue] = r
			}
		}
	}

	// Verify /canon -> PASS
	canonRes, ok := resultsByURL[srv.URL+"/canon"]
	if !ok {
		t.Errorf("missing result for %s", srv.URL+"/canon")
	} else if canonRes.Status != audit.StatusPass {
		t.Errorf("expected PASS for /canon, got %s (summary: %s)", canonRes.Status, canonRes.ObservedSummary)
	}

	// Verify /no-canon -> WARNING
	noCanonRes, ok := resultsByURL[srv.URL+"/no-canon"]
	if !ok {
		t.Errorf("missing result for %s", srv.URL+"/no-canon")
	} else if noCanonRes.Status != audit.StatusWarning {
		t.Errorf("expected WARNING for /no-canon, got %s (summary: %s)", noCanonRes.Status, noCanonRes.ObservedSummary)
	}

	// Verify /not-html -> NOT_APPLICABLE
	notHTMLRes, ok := resultsByURL[srv.URL+"/not-html"]
	if !ok {
		t.Errorf("missing result for %s", srv.URL+"/not-html")
	} else if notHTMLRes.Status != audit.StatusNotApplicable {
		t.Errorf("expected NOT_APPLICABLE for /not-html, got %s (summary: %s)", notHTMLRes.Status, notHTMLRes.ObservedSummary)
	}
}
