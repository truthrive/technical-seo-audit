package adapter

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// parseJSONLDSyntax validates JSON-LD raw script content against strict JSON syntax rules
// without invoking Schema.org semantic rules, external validators, or Google APIs.
//
// It returns:
// - parseStatus: PARSE_SUCCESS, PARSE_ERROR, or EMPTY_INPUT.
// - parseError: error message if invalid, or empty string on successful parse.
func parseJSONLDSyntax(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "\ufeff")
	trimmed = strings.TrimSpace(trimmed)
	if trimmed == "" {
		return ParseStatusEmptyInput, "empty or whitespace-only structured data block"
	}

	var val any
	dec := json.NewDecoder(strings.NewReader(trimmed))
	if err := dec.Decode(&val); err != nil {
		return ParseStatusError, err.Error()
	}

	// Ensure no trailing tokens exist after the top-level JSON value
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return ParseStatusError, "multiple top-level JSON values detected"
		}
		return ParseStatusError, fmt.Sprintf("trailing data after top-level JSON value: %v", err)
	}

	switch v := val.(type) {
	case map[string]any:
		// Check @graph if present at top level
		if graphVal, hasGraph := v["@graph"]; hasGraph {
			switch graphVal.(type) {
			case []any, map[string]any:
				// Valid @graph structure at JSON syntax level
			case nil:
				return ParseStatusError, "@graph value cannot be null"
			default:
				return ParseStatusError, "@graph value must be a JSON object or array"
			}
		}
		return ParseStatusSuccess, ""
	case []any:
		return ParseStatusSuccess, ""
	case nil:
		return ParseStatusError, "JSON-LD top-level value must be an object or array, got null"
	default:
		return ParseStatusError, fmt.Sprintf("JSON-LD top-level value must be an object or array, got %T", val)
	}
}

// buildStructuredDataEvidence processes Page.JSONLD and Page.SchemaOrg for a crawled page record
// and constructs typed audit.StructuredDataBlock entities and audit.NormalizedObservation records.
func buildStructuredDataEvidence(
	req BuildRequest,
	page sitecrawl.Page,
	urlID int,
	urlIDStr audit.URLID,
	rawURL string,
	obsTime time.Time,
	pageSrcRef string,
	nextObsID func() audit.ObservationID,
) ([]audit.StructuredDataBlock, []audit.NormalizedObservation, []EvidenceGap) {
	var (
		blocks []audit.StructuredDataBlock
		obs    []audit.NormalizedObservation
		gaps   []EvidenceGap
	)

	// 1. Process JSON-LD blocks
	maxJSONLD := 20
	if len(page.JSONLD) >= maxJSONLD {
		gaps = append(gaps, EvidenceGap{
			GapCode:         GapJSONLDExtractionCapped,
			SubjectRef:      string(urlIDStr),
			Field:           "jsonld_extraction_cap",
			Reason:          fmt.Sprintf("Page JSON-LD block count reached SiteCrawl extraction limit of %d blocks; additional blocks may exist.", maxJSONLD),
			SourceComponent: "sitecrawl_extract",
		})
	}

	for idx, rawBlock := range page.JSONLD {
		blockID := audit.StructuredBlockID(fmt.Sprintf("sdb:%s:%d:jsonld:%d", req.AuditRunID, urlID, idx))
		parseStatus, parseError := parseJSONLDSyntax(rawBlock)

		block := audit.StructuredDataBlock{
			StructuredBlockID:     blockID,
			AuditRunID:            req.AuditRunID,
			URLID:                 urlIDStr,
			Format:                audit.FormatJSONLD,
			BlockIndex:            idx,
			RawArtifactOrValueRef: fmt.Sprintf("sitecrawl_pages:%s:%d:jsonld:%d", req.CrawlRunID, urlID, idx),
			ParseStatus:           parseStatus,
			ParseError:            parseError,
			ObservedAt:            obsTime,
		}
		blocks = append(blocks, block)

		// Emitted observations on SubjectStructuredDataBlock
		obs = append(obs,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "url",
				Value:              rawURL,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "url_subject_ref",
				Value:              string(urlIDStr),
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_block_id",
				Value:              string(blockID),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_format",
				Value:              string(audit.FormatJSONLD),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_raw",
				Value:              rawBlock,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_parse_status",
				Value:              parseStatus,
				DerivationType:     audit.DerivationDerived,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
		)

		if parseError != "" {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_parse_error",
				Value:              parseError,
				DerivationType:     audit.DerivationDerived,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}

		// Reference on containing SubjectURL
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "structured_block_ref",
			Value:              string(blockID),
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})
	}

	// 2. Process Microdata blocks
	maxSchemas := 40
	if len(page.SchemaOrg) >= maxSchemas {
		gaps = append(gaps, EvidenceGap{
			GapCode:         GapMicrodataExtractionCapped,
			SubjectRef:      string(urlIDStr),
			Field:           "microdata_extraction_cap",
			Reason:          fmt.Sprintf("Page Microdata item count reached SiteCrawl extraction limit of %d schemas; additional items may exist.", maxSchemas),
			SourceComponent: "sitecrawl_extract",
		})
	}

	for idx := range page.SchemaOrg {
		blockID := audit.StructuredBlockID(fmt.Sprintf("sdb:%s:%d:microdata:%d", req.AuditRunID, urlID, idx))
		parseStatus := ParseStatusPartialAcquisition
		parseError := "raw Microdata markup not preserved by acquisition; syntax validation unavailable"

		block := audit.StructuredDataBlock{
			StructuredBlockID:     blockID,
			AuditRunID:            req.AuditRunID,
			URLID:                 urlIDStr,
			Format:                audit.FormatMicrodata,
			BlockIndex:            idx,
			RawArtifactOrValueRef: "", // Withheld because raw HTML is not preserved
			ParseStatus:           parseStatus,
			ParseError:            parseError,
			ObservedAt:            obsTime,
		}
		blocks = append(blocks, block)

		// Emitted observations on SubjectStructuredDataBlock
		obs = append(obs,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "url",
				Value:              rawURL,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "url_subject_ref",
				Value:              string(urlIDStr),
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_block_id",
				Value:              string(blockID),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_format",
				Value:              string(audit.FormatMicrodata),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			// Note: structured_raw is WITHHELD because raw HTML was not preserved
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_parse_status",
				Value:              parseStatus,
				DerivationType:     audit.DerivationDerived,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectStructuredDataBlock,
				SubjectRef:         string(blockID),
				Field:              "structured_parse_error",
				Value:              parseError,
				DerivationType:     audit.DerivationDerived,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
		)

		// Reference on containing SubjectURL
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "structured_block_ref",
			Value:              string(blockID),
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})
	}

	return blocks, obs, gaps
}
