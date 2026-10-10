package engine_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

type blockFixture struct {
	subjectRef string // e.g. "sdb:run:1:jsonld:0"

	// SubjectStructuredDataBlock observations
	blockID         string
	blockIDs        []string
	format          string
	formats         []string
	url             string
	urls            []string
	urlSubjRef      string
	urlSubjRefs     []string
	raw             *string
	raws            []string
	parseStatus     string
	parseStatuses   []string
	parseError      string
	parseErrors     []string
	omitBlockID     bool
	omitFormat      bool
	omitURL         bool
	omitURLSubjRef  bool
	omitParseStatus bool

	// Containing SubjectURL observations
	parentURLSubjRef   string
	parentURLIdent     string
	parentURLIdents    []string
	parentBlockRefs    []string
	omitParentSubject  bool
	omitParentURLIdent bool
	omitParentBlockRef bool
}

func newStructuredBlockSnapshot(fixtures []blockFixture, auditRunID audit.AuditRunID, snapID audit.SnapshotID) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	var obs []audit.NormalizedObservation

	createdParents := make(map[string]bool)

	for _, f := range fixtures {
		subRef := f.subjectRef
		parentRef := f.parentURLSubjRef
		if parentRef == "" {
			parentRef = f.urlSubjRef
		}
		if parentRef == "" {
			parentRef = fmt.Sprintf("url:%s:1", auditRunID)
		}

		pageSrcRef := "sitecrawl_pages:run:1"

		// 1. SubjectStructuredDataBlock observations
		if !f.omitBlockID {
			bID := f.blockID
			if bID == "" && len(f.blockIDs) == 0 {
				bID = subRef
			}
			if len(f.blockIDs) > 0 {
				for i, id := range f.blockIDs {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:bid:%s:%d", subRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectStructuredDataBlock,
						SubjectRef:         subRef,
						Field:              "structured_block_id",
						Value:              id,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         now,
					})
				}
			} else {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:bid:%s", subRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectStructuredDataBlock,
					SubjectRef:         subRef,
					Field:              "structured_block_id",
					Value:              bID,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         now,
				})
			}
		}

		if !f.omitFormat {
			fmtVal := f.format
			if fmtVal == "" && len(f.formats) == 0 {
				fmtVal = string(audit.FormatJSONLD)
			}
			if len(f.formats) > 0 {
				for i, fv := range f.formats {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:fmt:%s:%d", subRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectStructuredDataBlock,
						SubjectRef:         subRef,
						Field:              "structured_format",
						Value:              fv,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         now,
					})
				}
			} else {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:fmt:%s", subRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectStructuredDataBlock,
					SubjectRef:         subRef,
					Field:              "structured_format",
					Value:              fmtVal,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         now,
				})
			}
		}

		if !f.omitURL {
			uVal := f.url
			if uVal == "" && len(f.urls) == 0 {
				uVal = "https://example.com/page"
			}
			if len(f.urls) > 0 {
				for i, uv := range f.urls {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:url:%s:%d", subRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectStructuredDataBlock,
						SubjectRef:         subRef,
						Field:              "url",
						Value:              uv,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         now,
					})
				}
			} else {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:url:%s", subRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectStructuredDataBlock,
					SubjectRef:         subRef,
					Field:              "url",
					Value:              uVal,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         now,
				})
			}
		}

		if !f.omitURLSubjRef {
			refVal := f.urlSubjRef
			if refVal == "" && len(f.urlSubjRefs) == 0 {
				refVal = parentRef
			}
			if len(f.urlSubjRefs) > 0 {
				for i, rv := range f.urlSubjRefs {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:urlref:%s:%d", subRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectStructuredDataBlock,
						SubjectRef:         subRef,
						Field:              "url_subject_ref",
						Value:              rv,
						DerivationType:     audit.DerivationNormalized,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         now,
					})
				}
			} else {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:urlref:%s", subRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectStructuredDataBlock,
					SubjectRef:         subRef,
					Field:              "url_subject_ref",
					Value:              refVal,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         now,
				})
			}
		}

		if len(f.raws) > 0 {
			for i, rv := range f.raws {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:raw:%s:%d", subRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectStructuredDataBlock,
					SubjectRef:         subRef,
					Field:              "structured_raw",
					Value:              rv,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         now,
				})
			}
		} else if f.raw != nil {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:raw:%s", subRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         subRef,
				Field:              "structured_raw",
				Value:              *f.raw,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         now,
			})
		}

		if !f.omitParseStatus {
			pStat := f.parseStatus
			if pStat == "" && len(f.parseStatuses) == 0 {
				pStat = "PARSE_SUCCESS"
			}
			if len(f.parseStatuses) > 0 {
				for i, ps := range f.parseStatuses {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:status:%s:%d", subRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectStructuredDataBlock,
						SubjectRef:         subRef,
						Field:              "structured_parse_status",
						Value:              ps,
						DerivationType:     audit.DerivationDerived,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         now,
					})
				}
			} else {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:status:%s", subRef)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectStructuredDataBlock,
					SubjectRef:         subRef,
					Field:              "structured_parse_status",
					Value:              pStat,
					DerivationType:     audit.DerivationDerived,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         now,
				})
			}
		}

		if len(f.parseErrors) > 0 {
			for i, pe := range f.parseErrors {
				obs = append(obs, audit.NormalizedObservation{
					ObservationID:      audit.ObservationID(fmt.Sprintf("obs:err:%s:%d", subRef, i)),
					AuditRunID:         auditRunID,
					SnapshotID:         snapID,
					SubjectType:        audit.SubjectStructuredDataBlock,
					SubjectRef:         subRef,
					Field:              "structured_parse_error",
					Value:              pe,
					DerivationType:     audit.DerivationDerived,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         now,
				})
			}
		} else if f.parseError != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID(fmt.Sprintf("obs:err:%s", subRef)),
				AuditRunID:         auditRunID,
				SnapshotID:         snapID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         subRef,
				Field:              "structured_parse_error",
				Value:              f.parseError,
				DerivationType:     audit.DerivationDerived,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         now,
			})
		}

		// 2. Containing SubjectURL observations
		if !f.omitParentSubject && !createdParents[parentRef] {
			createdParents[parentRef] = true

			pIdent := f.parentURLIdent
			if pIdent == "" && len(f.parentURLIdents) == 0 {
				pIdent = f.url
				if pIdent == "" {
					pIdent = "https://example.com/page"
				}
			}

			if !f.omitParentURLIdent {
				if len(f.parentURLIdents) > 0 {
					for i, pid := range f.parentURLIdents {
						obs = append(obs, audit.NormalizedObservation{
							ObservationID:      audit.ObservationID(fmt.Sprintf("obs:pident:%s:%d", parentRef, i)),
							AuditRunID:         auditRunID,
							SnapshotID:         snapID,
							SubjectType:        audit.SubjectURL,
							SubjectRef:         parentRef,
							Field:              "url_identity",
							Value:              pid,
							DerivationType:     audit.DerivationDirect,
							SourceEvidenceRefs: []string{pageSrcRef},
							ObservedAt:         now,
						})
					}
				} else {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:pident:%s", parentRef)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         parentRef,
						Field:              "url_identity",
						Value:              pIdent,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         now,
					})
				}
			}

			if !f.omitParentBlockRef {
				bRefs := f.parentBlockRefs
				if len(bRefs) == 0 {
					bRefs = []string{subRef}
				}
				for i, br := range bRefs {
					obs = append(obs, audit.NormalizedObservation{
						ObservationID:      audit.ObservationID(fmt.Sprintf("obs:pblockref:%s:%d", parentRef, i)),
						AuditRunID:         auditRunID,
						SnapshotID:         snapID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         parentRef,
						Field:              "structured_block_ref",
						Value:              br,
						DerivationType:     audit.DerivationNormalized,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         now,
					})
				}
			}
		}
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             snapID,
		AuditRunID:             auditRunID,
		NormalizationVersion:   "v1.8.0",
		SnapshotStatus:         audit.SnapshotFrozen,
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func strPtr(s string) *string {
	return &s
}

func newTestEngine(t *testing.T) *engine.Engine {
	t.Helper()
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}
	return eng
}

// ============================================================================
// Group A: JSON-LD Success Tests (Section 12.A)
// ============================================================================

func TestEngine_Entity001_JSONLD_Success(t *testing.T) {
	eng := newTestEngine(t)
	auditRunID := audit.AuditRunID("audit:ent:success")
	snapID := audit.SnapshotID("snap:ent:success")

	t.Run("Valid JSON object", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@context": "https://schema.org", "@type": "Organization", "name": "Acme"}`),
				parseStatus: "PARSE_SUCCESS",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusPass {
			t.Errorf("expected PASS, got %s (observed: %s)", r.Status, r.ObservedSummary)
		}
		if r.SubjectType != audit.SubjectStructuredDataBlock {
			t.Errorf("expected subject type STRUCTURED_DATA_BLOCK, got %s", r.SubjectType)
		}
		if r.SubjectRef != "sdb:1:jsonld:0" {
			t.Errorf("expected subject ref sdb:1:jsonld:0, got %s", r.SubjectRef)
		}
		if !strings.Contains(r.ObservedSummary, "syntactically valid JSON") {
			t.Errorf("expected summary to state syntactically valid JSON, got %q", r.ObservedSummary)
		}
	})

	t.Run("Valid JSON array", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`[{"@type": "Person", "name": "Alice"}, {"@type": "Person", "name": "Bob"}]`),
				parseStatus: "PARSE_SUCCESS",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusPass {
			t.Fatalf("expected 1 PASS, got %v", results)
		}
	})

	t.Run("Valid @graph JSON", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@context": "https://schema.org", "@graph": [{"@type": "WebSite"}]}`),
				parseStatus: "PARSE_SUCCESS",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusPass {
			t.Fatalf("expected 1 PASS, got %v", results)
		}
	})

	t.Run("Valid JSON without @context", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@type": "Thing", "name": "NoContext"}`),
				parseStatus: "PARSE_SUCCESS",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusPass {
			t.Fatalf("expected 1 PASS, got %v", results)
		}
	})

	t.Run("Unusual Schema.org properties", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@context": "https://custom.org", "unknownProp": 42}`),
				parseStatus: "PARSE_SUCCESS",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusPass {
			t.Fatalf("expected 1 PASS, got %v", results)
		}
	})

	t.Run("Valid top-level JSON primitives", func(t *testing.T) {
		prims := []string{
			`"plain string"`,
			`123`,
			`true`,
			`null`,
		}
		for _, p := range prims {
			snap := newStructuredBlockSnapshot([]blockFixture{
				{
					subjectRef:  "sdb:1:jsonld:prim",
					raw:         strPtr(p),
					parseStatus: "PARSE_SUCCESS",
				},
			}, auditRunID, snapID)

			results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
			if err != nil {
				t.Fatalf("EvaluateRule failed: %v", err)
			}
			if len(results) != 1 || results[0].Status != audit.StatusPass {
				t.Fatalf("expected PASS for primitive %q, got %v", p, results)
			}
		}
	})

	t.Run("Multiple valid blocks on one page evaluated independently", func(t *testing.T) {
		parentURLSubj := fmt.Sprintf("url:%s:1", auditRunID)
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:       "sdb:1:jsonld:0",
				parentURLSubjRef: parentURLSubj,
				parentBlockRefs:  []string{"sdb:1:jsonld:0", "sdb:1:jsonld:1", "sdb:1:jsonld:2"},
				raw:              strPtr(`{"@type": "Organization"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			{
				subjectRef:       "sdb:1:jsonld:1",
				parentURLSubjRef: parentURLSubj,
				parentBlockRefs:  []string{"sdb:1:jsonld:0", "sdb:1:jsonld:1", "sdb:1:jsonld:2"},
				raw:              strPtr(`{"@type": "WebPage"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			{
				subjectRef:       "sdb:1:jsonld:2",
				parentURLSubjRef: parentURLSubj,
				parentBlockRefs:  []string{"sdb:1:jsonld:0", "sdb:1:jsonld:1", "sdb:1:jsonld:2"},
				raw:              strPtr(`{"@type": "BreadcrumbList"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 results, got %d", len(results))
		}
		for i, r := range results {
			if r.Status != audit.StatusPass {
				t.Errorf("block %d expected PASS, got %s", i, r.Status)
			}
		}
	})
}

// ============================================================================
// Group B: JSON-LD Failure Tests (Section 12.B)
// ============================================================================

func TestEngine_Entity001_JSONLD_Failure(t *testing.T) {
	eng := newTestEngine(t)
	auditRunID := audit.AuditRunID("audit:ent:fail")
	snapID := audit.SnapshotID("snap:ent:fail")

	t.Run("Malformed JSON with parse error", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{unquoted: "syntax"}`),
				parseStatus: "PARSE_ERROR",
				parseError:  "invalid character 'u' looking for beginning of object key string",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Errorf("expected FAIL, got %s", r.Status)
		}
		if !strings.Contains(r.ObservedSummary, "failed deterministic JSON syntax parsing") {
			t.Errorf("expected summary to indicate syntax parse failure, got %q", r.ObservedSummary)
		}
		if !strings.Contains(r.ObservedSummary, "invalid character 'u'") {
			t.Errorf("expected summary to contain error detail, got %q", r.ObservedSummary)
		}
	})

	t.Run("Invalid trailing comma", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"name": "Alice",}`),
				parseStatus: "PARSE_ERROR",
				parseError:  "invalid character '}' looking for beginning of object key string",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected 1 FAIL, got %v", results)
		}
	})

	t.Run("Incomplete JSON", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"name": "Trun`),
				parseStatus: "PARSE_ERROR",
				parseError:  "unexpected EOF",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected 1 FAIL, got %v", results)
		}
	})

	t.Run("Multiple top-level JSON documents", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"name":"A"} {"name":"B"}`),
				parseStatus: "PARSE_ERROR",
				parseError:  "multiple top-level JSON values detected",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusFail {
			t.Fatalf("expected 1 FAIL, got %v", results)
		}
	})

	t.Run("Observed empty JSON-LD block (EMPTY_INPUT)", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr("   \n\t  "),
				parseStatus: "EMPTY_INPUT",
				parseError:  "empty or whitespace-only structured data block",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusFail {
			t.Errorf("expected FAIL for EMPTY_INPUT, got %s", r.Status)
		}
		if !strings.Contains(r.ObservedSummary, "empty or contains only whitespace") {
			t.Errorf("expected empty input summary, got %q", r.ObservedSummary)
		}
	})
}

// ============================================================================
// Group C: Incomplete Evidence & Correlation Tests (Section 12.C)
// ============================================================================

func TestEngine_Entity001_IncompleteEvidence(t *testing.T) {
	eng := newTestEngine(t)
	auditRunID := audit.AuditRunID("audit:ent:incomp")
	snapID := audit.SnapshotID("snap:ent:incomp")

	tests := []struct {
		name    string
		fixture blockFixture
	}{
		{
			name: "Missing parse status",
			fixture: blockFixture{
				subjectRef:      "sdb:1:jsonld:0",
				raw:             strPtr(`{"@type": "Thing"}`),
				omitParseStatus: true,
			},
		},
		{
			name: "Conflicting parse statuses",
			fixture: blockFixture{
				subjectRef:    "sdb:1:jsonld:0",
				raw:           strPtr(`{"@type": "Thing"}`),
				parseStatuses: []string{"PARSE_SUCCESS", "PARSE_ERROR"},
			},
		},
		{
			name: "Missing format",
			fixture: blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@type": "Thing"}`),
				parseStatus: "PARSE_SUCCESS",
				omitFormat:  true,
			},
		},
		{
			name: "Conflicting formats",
			fixture: blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@type": "Thing"}`),
				parseStatus: "PARSE_SUCCESS",
				formats:     []string{string(audit.FormatJSONLD), string(audit.FormatMicrodata)},
			},
		},
		{
			name: "Missing block ID",
			fixture: blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@type": "Thing"}`),
				parseStatus: "PARSE_SUCCESS",
				omitBlockID: true,
			},
		},
		{
			name: "Block ID mismatch",
			fixture: blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				blockID:     "sdb:1:jsonld:different",
				raw:         strPtr(`{"@type": "Thing"}`),
				parseStatus: "PARSE_SUCCESS",
			},
		},
		{
			name: "Missing containing URL",
			fixture: blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@type": "Thing"}`),
				parseStatus: "PARSE_SUCCESS",
				omitURL:     true,
			},
		},
		{
			name: "Missing URL subject ref",
			fixture: blockFixture{
				subjectRef:     "sdb:1:jsonld:0",
				raw:            strPtr(`{"@type": "Thing"}`),
				parseStatus:    "PARSE_SUCCESS",
				omitURLSubjRef: true,
			},
		},
		{
			name: "Dangling URL subject ref",
			fixture: blockFixture{
				subjectRef:        "sdb:1:jsonld:0",
				urlSubjRef:        "url:audit:dangling",
				raw:               strPtr(`{"@type": "Thing"}`),
				parseStatus:       "PARSE_SUCCESS",
				omitParentSubject: true,
			},
		},
		{
			name: "URL identity mismatch",
			fixture: blockFixture{
				subjectRef:     "sdb:1:jsonld:0",
				url:            "https://example.com/one",
				parentURLIdent: "https://example.com/other",
				raw:            strPtr(`{"@type": "Thing"}`),
				parseStatus:    "PARSE_SUCCESS",
			},
		},
		{
			name: "Missing structured_block_ref on parent URL",
			fixture: blockFixture{
				subjectRef:         "sdb:1:jsonld:0",
				raw:                strPtr(`{"@type": "Thing"}`),
				parseStatus:        "PARSE_SUCCESS",
				omitParentBlockRef: true,
			},
		},
		{
			name: "Parent structured_block_ref points to another block",
			fixture: blockFixture{
				subjectRef:      "sdb:1:jsonld:0",
				raw:             strPtr(`{"@type": "Thing"}`),
				parseStatus:     "PARSE_SUCCESS",
				parentBlockRefs: []string{"sdb:1:jsonld:alien"},
			},
		},
		{
			name: "Missing raw JSON-LD evidence",
			fixture: blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         nil, // omitted structured_raw
				parseStatus: "PARSE_SUCCESS",
			},
		},
		{
			name: "PARSE_ERROR without parse-error detail",
			fixture: blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{malformed}`),
				parseStatus: "PARSE_ERROR",
				parseError:  "", // missing error detail
			},
		},
		{
			name: "PARSE_SUCCESS with contradictory parse error",
			fixture: blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         strPtr(`{"@type": "Thing"}`),
				parseStatus: "PARSE_SUCCESS",
				parseError:  "unexpected syntax error", // contradiction
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snap := newStructuredBlockSnapshot([]blockFixture{tc.fixture}, auditRunID, snapID)
			results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
			if err != nil {
				t.Fatalf("EvaluateRule failed: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			if results[0].Status != audit.StatusUnknown {
				t.Errorf("expected UNKNOWN for %s, got %s (observed: %s)", tc.name, results[0].Status, results[0].ObservedSummary)
			}
		})
	}
}

// ============================================================================
// Group D: Format Boundaries Tests (Section 12.D)
// ============================================================================

func TestEngine_Entity001_FormatBoundaries(t *testing.T) {
	eng := newTestEngine(t)
	auditRunID := audit.AuditRunID("audit:ent:format")
	snapID := audit.SnapshotID("snap:ent:format")

	t.Run("Microdata PARTIAL_ACQUISITION produces UNKNOWN", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:microdata:0",
				format:      string(audit.FormatMicrodata),
				parseStatus: "PARTIAL_ACQUISITION",
				parseError:  "raw Microdata markup not preserved by acquisition",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Status != audit.StatusUnknown {
			t.Errorf("expected UNKNOWN for Microdata, got %s", r.Status)
		}
		if !strings.Contains(r.ObservedSummary, "Microdata syntax validation is unavailable") {
			t.Errorf("expected Microdata unavailable summary, got %q", r.ObservedSummary)
		}
	})

	t.Run("RDFa unsupported acquisition produces UNKNOWN", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:rdfa:0",
				format:      string(audit.FormatRDFa),
				parseStatus: "PARSER_UNAVAILABLE",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for RDFa, got %v", results)
		}
	})

	t.Run("Unknown format produces UNKNOWN", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:other:0",
				format:      "TURTLE",
				parseStatus: "PARSE_SUCCESS",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for unknown format, got %v", results)
		}
	})

	t.Run("Unsupported parse status produces UNKNOWN", func(t *testing.T) {
		snap := newStructuredBlockSnapshot([]blockFixture{
			{
				subjectRef:  "sdb:1:jsonld:0",
				format:      string(audit.FormatJSONLD),
				raw:         strPtr(`{}`),
				parseStatus: "PARSE_INDETERMINATE_UNKNOWN",
			},
		}, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 1 || results[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN for unsupported parse status, got %v", results)
		}
	})
}

// ============================================================================
// Group E: No-Block Handling Tests (Section 12.E)
// ============================================================================

func TestEngine_Entity001_NoBlockHandling(t *testing.T) {
	eng := newTestEngine(t)
	auditRunID := audit.AuditRunID("audit:ent:noblock")
	snapID := audit.SnapshotID("snap:ent:noblock")

	t.Run("Snapshot with 0 structured-data block subjects returns empty result set", func(t *testing.T) {
		snap := newStructuredBlockSnapshot(nil, auditRunID, snapID)

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("Snapshot with URL observations but zero structured-data blocks returns empty result set", func(t *testing.T) {
		now := time.Now().UTC()
		snap := &audit.EvidenceSnapshot{
			SnapshotID:           snapID,
			AuditRunID:           auditRunID,
			NormalizationVersion: "v1.8.0",
			SnapshotStatus:       audit.SnapshotFrozen,
			FrozenAt:             &now,
			NormalizedObservations: []audit.NormalizedObservation{
				{
					ObservationID:  "obs:url:1",
					AuditRunID:     auditRunID,
					SnapshotID:     snapID,
					SubjectType:    audit.SubjectURL,
					SubjectRef:     "url:audit:1",
					Field:          "url_identity",
					Value:          "https://example.com/no-blocks",
					DerivationType: audit.DerivationDirect,
					ObservedAt:     now,
				},
			},
		}

		results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(results) != 0 {
			t.Fatalf("expected 0 results (no fabricated NOT_APPLICABLE), got %d", len(results))
		}
	})
}

// ============================================================================
// Group F: Determinism & Immutability Tests (Section 12.F)
// ============================================================================

func TestEngine_Entity001_Determinism(t *testing.T) {
	eng := newTestEngine(t)
	auditRunID := audit.AuditRunID("audit:ent:deter")
	snapID := audit.SnapshotID("snap:ent:deter")

	fixtures := []blockFixture{
		{
			subjectRef:  "sdb:1:jsonld:0",
			raw:         strPtr(`{"@type": "Organization"}`),
			parseStatus: "PARSE_SUCCESS",
		},
		{
			subjectRef:  "sdb:1:jsonld:1",
			raw:         strPtr(`{"invalid":`),
			parseStatus: "PARSE_ERROR",
			parseError:  "unexpected EOF",
		},
	}

	t.Run("Repeat evaluation produces identical RuleResults", func(t *testing.T) {
		snap := newStructuredBlockSnapshot(fixtures, auditRunID, snapID)

		res1, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("run 1 failed: %v", err)
		}
		res2, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
		if err != nil {
			t.Fatalf("run 2 failed: %v", err)
		}

		if len(res1) != len(res2) {
			t.Fatalf("result lengths differ: %d vs %d", len(res1), len(res2))
		}
		for i := range res1 {
			if res1[i].RuleResultID != res2[i].RuleResultID {
				t.Errorf("result %d ID mismatch: %s vs %s", i, res1[i].RuleResultID, res2[i].RuleResultID)
			}
			if res1[i].Status != res2[i].Status {
				t.Errorf("result %d status mismatch: %s vs %s", i, res1[i].Status, res2[i].Status)
			}
			if len(res1[i].EvidenceRefs) != len(res2[i].EvidenceRefs) {
				t.Errorf("result %d evidence refs length mismatch", i)
			}
		}
	})

	t.Run("Reversed normalized observations order produces identical deterministic results", func(t *testing.T) {
		snap1 := newStructuredBlockSnapshot(fixtures, auditRunID, snapID)
		snap2 := newStructuredBlockSnapshot(fixtures, auditRunID, snapID)

		// Reverse snap2 observations
		n := len(snap2.NormalizedObservations)
		for i := 0; i < n/2; i++ {
			snap2.NormalizedObservations[i], snap2.NormalizedObservations[n-1-i] = snap2.NormalizedObservations[n-1-i], snap2.NormalizedObservations[i]
		}

		res1, _ := eng.EvaluateRule(context.Background(), snap1, "AR-ENTITY-001")
		res2, _ := eng.EvaluateRule(context.Background(), snap2, "AR-ENTITY-001")

		if len(res1) != len(res2) {
			t.Fatalf("lengths differ: %d vs %d", len(res1), len(res2))
		}
		for i := range res1 {
			if res1[i].RuleResultID != res2[i].RuleResultID {
				t.Errorf("reversed order produced different result ordering at %d: %s vs %s", i, res1[i].RuleResultID, res2[i].RuleResultID)
			}
		}
	})

	t.Run("Context cancellation returns error immediately", func(t *testing.T) {
		snap := newStructuredBlockSnapshot(fixtures, auditRunID, snapID)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := eng.EvaluateRule(ctx, snap, "AR-ENTITY-001")
		if err == nil {
			t.Fatalf("expected error on cancelled context")
		}
	})
}

// ============================================================================
// Group G: Hermetic SQLite Integration Test (Section 12.G)
// ============================================================================

func TestEngine_Entity001_HermeticIntegration(t *testing.T) {
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB failed: %v", err)
	}
	defer db.Close()

	runner := sitecrawl.NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	crawlRunID := "run:ent:integ"
	auditRunID := audit.AuditRunID("audit:ent:integ")
	snapID := audit.SnapshotID("snap:ent:integ")
	seedURL := "https://example.com/page1"

	// Setup run
	nowStr := "2026-10-01T12:00:00Z"
	_, err = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', ?)`, crawlRunID, seedURL, nowStr)
	if err != nil {
		t.Fatalf("insert run failed: %v", err)
	}

	// Insert Page 1: valid JSON-LD and malformed JSON-LD
	insertURL := func(urlID int, u string) {
		_, err := db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url)
			VALUES(?, ?, ?)`, crawlRunID, urlID, u)
		if err != nil {
			t.Fatalf("insert url failed: %v", err)
		}
	}
	insertPage := func(urlID int, u string, p sitecrawl.Page) {
		dataBytes, _ := json.Marshal(p)
		_, err := db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, ?, ?, ?, 200, 'html', ?)`, crawlRunID, urlID, u, string(dataBytes), nowStr)
		if err != nil {
			t.Fatalf("insert page failed: %v", err)
		}
	}

	insertURL(1, seedURL)
	insertPage(1, seedURL, sitecrawl.Page{
		URL:       seedURL,
		Status:    200,
		Kind:      "html",
		CrawledAt: nowStr,
		JSONLD: []string{
			`{"@context": "https://schema.org", "@type": "Organization", "name": "Acme"}`,
			`{"name": "malformed", trailing comma}`,
			`   `, // empty input
		},
	})

	// Insert Page 2: Microdata
	page2URL := "https://example.com/page2"
	insertURL(2, page2URL)
	insertPage(2, page2URL, sitecrawl.Page{
		URL:       page2URL,
		Status:    200,
		Kind:      "html",
		CrawledAt: nowStr,
		SchemaOrg: []sitecrawl.Schema{
			{Type: "https://schema.org/Product", Properties: map[string]string{"name": "Widget"}},
		},
	})

	// Insert Page 3: No structured data
	page3URL := "https://example.com/page3"
	insertURL(3, page3URL)
	insertPage(3, page3URL, sitecrawl.Page{
		URL:       page3URL,
		Status:    200,
		Kind:      "html",
		CrawledAt: nowStr,
	})

	// Build snapshot through Adapter
	buildRes, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: crawlRunID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	snap := buildRes.EvidenceSnapshot
	if snap == nil {
		t.Fatalf("expected non-nil snapshot")
	}

	// Index snapshot observations for verification
	obsByID := make(map[string]bool)
	for _, o := range snap.NormalizedObservations {
		obsByID[string(o.ObservationID)] = true
	}

	eng := newTestEngine(t)
	results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
	if err != nil {
		t.Fatalf("EvaluateRule AR-ENTITY-001 failed: %v", err)
	}

	// Total blocks expected: 3 from page 1 + 1 from page 2 = 4 blocks! Page 3 has 0 blocks.
	if len(results) != 4 {
		t.Fatalf("expected 4 results across the crawl, got %d", len(results))
	}

	// Verify status distribution:
	// Block 0 (page 1): valid JSON-LD -> PASS
	// Block 1 (page 1): malformed JSON-LD -> FAIL
	// Block 2 (page 1): empty JSON-LD -> FAIL
	// Block 3 (page 2): Microdata -> UNKNOWN
	expectedStatuses := []audit.RuleResultStatus{
		audit.StatusPass,
		audit.StatusFail,
		audit.StatusFail,
		audit.StatusUnknown,
	}

	for i, exp := range expectedStatuses {
		if results[i].Status != exp {
			t.Errorf("result %d expected status %s, got %s (summary: %s)", i, exp, results[i].Status, results[i].ObservedSummary)
		}
		// Verify all evidence refs resolve to real observations in the snapshot
		if len(results[i].EvidenceRefs) == 0 {
			t.Errorf("result %d has no evidence refs", i)
		}
		for _, ref := range results[i].EvidenceRefs {
			if !obsByID[ref.EvidenceRef] {
				t.Errorf("result %d evidence ref %s does not resolve to a real snapshot observation", i, ref.EvidenceRef)
			}
		}
	}
}

// ============================================================================
// Group H: Regression Tests (Section 12.H)
// ============================================================================

func TestEngine_Entity001_Regression(t *testing.T) {
	eng := newTestEngine(t)

	// 1. Exactly 13 implemented rules
	ruleIDs := eng.ImplementedRuleIDs()
	if len(ruleIDs) != 13 {
		t.Fatalf("expected exactly 13 implemented rules, got %d: %v", len(ruleIDs), ruleIDs)
	}

	// 2. Frozen registry contains 47 rules
	reg, err := audit.LoadV1Registry()
	if err != nil {
		t.Fatalf("LoadV1Registry failed: %v", err)
	}
	allRules := reg.All()
	if len(allRules) != 47 {
		t.Fatalf("expected 47 rules in registry, got %d", len(allRules))
	}

	// 3. Confirm AR-ENTITY-001 is registered in both registry and engine
	hasInReg := false
	for _, r := range allRules {
		if r.RuleID == "AR-ENTITY-001" {
			hasInReg = true
			if r.DefaultSeverity != audit.SeverityP2 {
				t.Errorf("expected default severity P2, got %s", r.DefaultSeverity)
			}
			if r.Lifecycle != audit.LifecycleRetrieve {
				t.Errorf("expected lifecycle Retrieve, got %s", r.Lifecycle)
			}
		}
	}
	if !hasInReg {
		t.Errorf("AR-ENTITY-001 not found in registry")
	}

	hasInEngine := false
	for _, id := range ruleIDs {
		if id == "AR-ENTITY-001" {
			hasInEngine = true
		}
	}
	if !hasInEngine {
		t.Errorf("AR-ENTITY-001 not implemented in engine")
	}
}

// ============================================================================
// V1.7b Correction Tests (Section 7.A & 7.B)
// ============================================================================

func TestEngine_Entity001_Correction_RawStatusConsistency(t *testing.T) {
	eng := newTestEngine(t)
	auditRunID := audit.AuditRunID("audit:ent:rawstatus")
	snapID := audit.SnapshotID("snap:ent:rawstatus")

	tests := []struct {
		name           string
		raw            *string
		raws           []string
		parseStatus    string
		parseError     string
		expectedStatus audit.RuleResultStatus
	}{
		{
			name:           "Valid JSON + PARSE_SUCCESS -> PASS",
			raw:            strPtr(`{"@type": "Organization", "name": "Acme"}`),
			parseStatus:    "PARSE_SUCCESS",
			expectedStatus: audit.StatusPass,
		},
		{
			name:           "Malformed JSON + PARSE_ERROR + error -> FAIL",
			raw:            strPtr(`{"unquoted": value}`),
			parseStatus:    "PARSE_ERROR",
			parseError:     "invalid character 'v' looking for beginning of value",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "Empty string + EMPTY_INPUT -> FAIL",
			raw:            strPtr(""),
			parseStatus:    "EMPTY_INPUT",
			parseError:     "empty or whitespace-only structured data block",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "Whitespace-only + EMPTY_INPUT -> FAIL",
			raw:            strPtr("   \n\t  "),
			parseStatus:    "EMPTY_INPUT",
			parseError:     "empty or whitespace-only structured data block",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "BOM-only + EMPTY_INPUT -> FAIL",
			raw:            strPtr("\ufeff"),
			parseStatus:    "EMPTY_INPUT",
			parseError:     "empty or whitespace-only structured data block",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "BOM + whitespace + EMPTY_INPUT -> FAIL",
			raw:            strPtr("\ufeff   \n\t  "),
			parseStatus:    "EMPTY_INPUT",
			parseError:     "empty or whitespace-only structured data block",
			expectedStatus: audit.StatusFail,
		},
		{
			name:           "Empty string + PARSE_SUCCESS -> UNKNOWN",
			raw:            strPtr(""),
			parseStatus:    "PARSE_SUCCESS",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "Whitespace-only + PARSE_SUCCESS -> UNKNOWN",
			raw:            strPtr("   \t\n  "),
			parseStatus:    "PARSE_SUCCESS",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "BOM + whitespace + PARSE_SUCCESS -> UNKNOWN",
			raw:            strPtr("\ufeff  \n  "),
			parseStatus:    "PARSE_SUCCESS",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "Empty string + PARSE_ERROR + error -> UNKNOWN",
			raw:            strPtr(""),
			parseStatus:    "PARSE_ERROR",
			parseError:     "unexpected EOF",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "Whitespace-only + PARSE_ERROR + error -> UNKNOWN",
			raw:            strPtr("    "),
			parseStatus:    "PARSE_ERROR",
			parseError:     "unexpected EOF",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "BOM + whitespace + PARSE_ERROR + error -> UNKNOWN",
			raw:            strPtr("\ufeff  "),
			parseStatus:    "PARSE_ERROR",
			parseError:     "unexpected EOF",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "Non-empty raw + EMPTY_INPUT -> UNKNOWN",
			raw:            strPtr(`{"@type": "Thing"}`),
			parseStatus:    "EMPTY_INPUT",
			parseError:     "empty or whitespace-only structured data block",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "Missing raw observation -> UNKNOWN",
			raw:            nil,
			parseStatus:    "PARSE_SUCCESS",
			expectedStatus: audit.StatusUnknown,
		},
		{
			name:           "Conflicting raw observations -> UNKNOWN",
			raws:           []string{`{"a": 1}`, `{"b": 2}`},
			parseStatus:    "PARSE_SUCCESS",
			expectedStatus: audit.StatusUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fix := blockFixture{
				subjectRef:  "sdb:1:jsonld:0",
				raw:         tc.raw,
				raws:        tc.raws,
				parseStatus: tc.parseStatus,
				parseError:  tc.parseError,
			}
			snap := newStructuredBlockSnapshot([]blockFixture{fix}, auditRunID, snapID)
			results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
			if err != nil {
				t.Fatalf("EvaluateRule failed: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			r := results[0]
			if r.Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s (observed: %s)", tc.expectedStatus, r.Status, r.ObservedSummary)
			}
			if tc.expectedStatus == audit.StatusUnknown {
				if r.Status == audit.StatusPass || r.Status == audit.StatusFail {
					t.Errorf("contradictory raw/status must not produce PASS or FAIL; got %s", r.Status)
				}
			}
		})
	}
}

func TestEngine_Entity001_Correction_URLReferenceIntegrity(t *testing.T) {
	eng := newTestEngine(t)
	auditRunID := audit.AuditRunID("audit:ent:urlref")
	snapID := audit.SnapshotID("snap:ent:urlref")

	validParentSubj := fmt.Sprintf("url:%s:1", auditRunID)

	tests := []struct {
		name           string
		fixture        blockFixture
		expectedStatus audit.RuleResultStatus
	}{
		{
			name: "Valid URL reference matching current audit run -> PASS",
			fixture: blockFixture{
				subjectRef:       "sdb:1:jsonld:0",
				urlSubjRef:       validParentSubj,
				parentURLSubjRef: validParentSubj,
				raw:              strPtr(`{"@type": "Thing"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			expectedStatus: audit.StatusPass,
		},
		{
			name: "Reference from a different audit run -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:       "sdb:1:jsonld:0",
				urlSubjRef:       "url:foreign_run_123:1",
				parentURLSubjRef: validParentSubj,
				raw:              strPtr(`{"@type": "Thing"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			expectedStatus: audit.StatusUnknown,
		},
		{
			name: "Malformed reference prefix -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:       "sdb:1:jsonld:0",
				urlSubjRef:       "noturl:prefix:1",
				parentURLSubjRef: validParentSubj,
				raw:              strPtr(`{"@type": "Thing"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			expectedStatus: audit.StatusUnknown,
		},
		{
			name: "Missing URL ID component (empty trailing) -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:       "sdb:1:jsonld:0",
				urlSubjRef:       fmt.Sprintf("url:%s:", auditRunID),
				parentURLSubjRef: validParentSubj,
				raw:              strPtr(`{"@type": "Thing"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			expectedStatus: audit.StatusUnknown,
		},
		{
			name: "Missing URL ID component (no trailing colon) -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:       "sdb:1:jsonld:0",
				urlSubjRef:       fmt.Sprintf("url:%s", auditRunID),
				parentURLSubjRef: validParentSubj,
				raw:              strPtr(`{"@type": "Thing"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			expectedStatus: audit.StatusUnknown,
		},
		{
			name: "Missing URL ID component (whitespace only) -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:       "sdb:1:jsonld:0",
				urlSubjRef:       fmt.Sprintf("url:%s:   ", auditRunID),
				parentURLSubjRef: validParentSubj,
				raw:              strPtr(`{"@type": "Thing"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			expectedStatus: audit.StatusUnknown,
		},
		{
			name: "Correct prefix but dangling URL subject -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:        "sdb:1:jsonld:0",
				urlSubjRef:        fmt.Sprintf("url:%s:999", auditRunID),
				parentURLSubjRef:  validParentSubj,
				raw:               strPtr(`{"@type": "Thing"}`),
				parseStatus:       "PARSE_SUCCESS",
				omitParentSubject: true,
			},
			expectedStatus: audit.StatusUnknown,
		},
		{
			name: "Referenced URL subject missing url_identity -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:         "sdb:1:jsonld:0",
				urlSubjRef:         validParentSubj,
				parentURLSubjRef:   validParentSubj,
				raw:                strPtr(`{"@type": "Thing"}`),
				parseStatus:        "PARSE_SUCCESS",
				omitParentURLIdent: true,
			},
			expectedStatus: audit.StatusUnknown,
		},
		{
			name: "URL identity mismatch -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:       "sdb:1:jsonld:0",
				url:              "https://example.com/one",
				urlSubjRef:       validParentSubj,
				parentURLSubjRef: validParentSubj,
				parentURLIdent:   "https://example.com/different",
				raw:              strPtr(`{"@type": "Thing"}`),
				parseStatus:      "PARSE_SUCCESS",
			},
			expectedStatus: audit.StatusUnknown,
		},
		{
			name: "Missing reverse structured_block_ref on containing URL -> UNKNOWN",
			fixture: blockFixture{
				subjectRef:         "sdb:1:jsonld:0",
				urlSubjRef:         validParentSubj,
				parentURLSubjRef:   validParentSubj,
				raw:                strPtr(`{"@type": "Thing"}`),
				parseStatus:        "PARSE_SUCCESS",
				omitParentBlockRef: true,
			},
			expectedStatus: audit.StatusUnknown,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snap := newStructuredBlockSnapshot([]blockFixture{tc.fixture}, auditRunID, snapID)
			results, err := eng.EvaluateRule(context.Background(), snap, "AR-ENTITY-001")
			if err != nil {
				t.Fatalf("EvaluateRule failed: %v", err)
			}
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			r := results[0]
			if r.Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s (observed: %s)", tc.expectedStatus, r.Status, r.ObservedSummary)
			}
		})
	}
}
