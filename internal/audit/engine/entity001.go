package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const (
	ruleIDENTITY001 = "AR-ENTITY-001"

	parseStatusSuccess            = "PARSE_SUCCESS"
	parseStatusError              = "PARSE_ERROR"
	parseStatusEmptyInput         = "EMPTY_INPUT"
	parseStatusPartialAcquisition = "PARTIAL_ACQUISITION"
	parseStatusParserUnavailable  = "PARSER_UNAVAILABLE"
)

// evaluateENTITY001 implements the typed execution logic for:
// AR-ENTITY-001 — Structured-data block parses successfully (parent check: ENTITY-001, P2, deterministic).
//
// Evaluates observed audit.SubjectStructuredDataBlock entities against V1.7a normalized evidence.
// Evaluates each block independently.
//
// Contract:
// - PASS: JSON-LD block with verified PARSE_SUCCESS syntax.
// - FAIL: JSON-LD block with reproducible PARSE_ERROR or EMPTY_INPUT.
// - UNKNOWN: Microdata (PARTIAL_ACQUISITION / raw markup unavailable), RDFa (unsupported acquisition),
//            PARSER_UNAVAILABLE, missing/conflicting/malformed identity or correlation evidence.
// - No synthetic page-level NOT_APPLICABLE: if no blocks observed, returns empty result set.
func evaluateENTITY001(
	ctx context.Context,
	rule audit.RuleDefinition,
	idx *EvidenceIndex,
	policies *PolicyIndex,
	snapshot *audit.EvidenceSnapshot,
	evalTime time.Time,
) ([]audit.RuleResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	blockRefs := idx.SubjectRefs(audit.SubjectStructuredDataBlock)
	if len(blockRefs) == 0 {
		return []audit.RuleResult{}, nil
	}

	results := make([]audit.RuleResult, 0, len(blockRefs))
	expectedSummary := "Structured-data block parses successfully in supported parser."

	for _, subjectRef := range blockRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// 1. Collect all SubjectStructuredDataBlock observations
		blockIDObs := idx.GetObservations(audit.SubjectStructuredDataBlock, subjectRef, "structured_block_id")
		formatObs := idx.GetObservations(audit.SubjectStructuredDataBlock, subjectRef, "structured_format")
		urlObs := idx.GetObservations(audit.SubjectStructuredDataBlock, subjectRef, "url")
		urlSubjRefObs := idx.GetObservations(audit.SubjectStructuredDataBlock, subjectRef, "url_subject_ref")
		rawObs := idx.GetObservations(audit.SubjectStructuredDataBlock, subjectRef, "structured_raw")
		parseStatusObs := idx.GetObservations(audit.SubjectStructuredDataBlock, subjectRef, "structured_parse_status")
		parseErrorObs := idx.GetObservations(audit.SubjectStructuredDataBlock, subjectRef, "structured_parse_error")

		buildBlockContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			refs = append(refs, buildEvidenceRefs(ruleResultID, blockIDObs, "structured_block_id", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, formatObs, "structured_format", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlSubjRefObs, "url_subject_ref", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, rawObs, "structured_raw", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parseStatusObs, "structured_parse_status", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parseErrorObs, "structured_parse_error", audit.EvidenceRoleContext)...)
			return refs
		}

		// 2. Validate structured_block_id
		blockIDVal, blockIDUsable, blockIDConflict := extractSingleObsValue(blockIDObs)
		if !blockIDUsable {
			var summary string
			if len(blockIDObs) == 0 {
				summary = "Required structured_block_id evidence is missing."
			} else if blockIDConflict {
				summary = "Conflicting structured_block_id evidence."
			} else {
				summary = "Malformed structured_block_id evidence."
			}
			refs := buildBlockContextRefs()
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}
		if blockIDVal != subjectRef {
			summary := fmt.Sprintf("structured_block_id %q does not match subject reference %q.", blockIDVal, subjectRef)
			refs := buildBlockContextRefs()
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 3. Validate url_subject_ref
		urlSubjRefVal, urlSubjRefUsable, urlSubjRefConflict := extractSingleObsValue(urlSubjRefObs)
		if !urlSubjRefUsable {
			var summary string
			if len(urlSubjRefObs) == 0 {
				summary = "Required url_subject_ref evidence is missing."
			} else if urlSubjRefConflict {
				summary = "Conflicting url_subject_ref evidence."
			} else {
				summary = "Malformed url_subject_ref evidence."
			}
			refs := buildBlockContextRefs()
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// Ensure url_subject_ref resolves to a real SubjectURL in the frozen snapshot
		parentURLRefs := idx.SubjectRefs(audit.SubjectURL)
		foundParent := false
		for _, ref := range parentURLRefs {
			if ref == urlSubjRefVal {
				foundParent = true
				break
			}
		}
		if !foundParent {
			summary := fmt.Sprintf("Referenced containing url_subject_ref %q is not found in the frozen snapshot.", urlSubjRefVal)
			refs := buildBlockContextRefs()
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 4. Validate containing URL identity
		parentURLIdentObs := idx.GetObservations(audit.SubjectURL, urlSubjRefVal, "url_identity")
		parentURLVal, parentURLUsable, parentURLConflict := extractSingleObsValue(parentURLIdentObs)
		if !parentURLUsable {
			var summary string
			if len(parentURLIdentObs) == 0 {
				summary = fmt.Sprintf("Containing URL %q lacks required url_identity evidence.", urlSubjRefVal)
			} else if parentURLConflict {
				summary = fmt.Sprintf("Containing URL %q has conflicting url_identity evidence.", urlSubjRefVal)
			} else {
				summary = fmt.Sprintf("Containing URL %q has malformed url_identity evidence.", urlSubjRefVal)
			}
			refs := buildBlockContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 5. Validate block's containing URL matches SubjectURL identity
		blockURLVal, blockURLUsable, blockURLConflict := extractSingleObsValue(urlObs)
		if !blockURLUsable {
			var summary string
			if len(urlObs) == 0 {
				summary = "Required containing url evidence on block is missing."
			} else if blockURLConflict {
				summary = "Conflicting containing url evidence on block."
			} else {
				summary = "Malformed containing url evidence on block."
			}
			refs := buildBlockContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}
		if blockURLVal != parentURLVal {
			summary := fmt.Sprintf("Block url %q does not match containing URL identity %q.", blockURLVal, parentURLVal)
			refs := buildBlockContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 6. Validate containing SubjectURL correlates back to this block via structured_block_ref
		parentBlockRefObs := idx.GetObservations(audit.SubjectURL, urlSubjRefVal, "structured_block_ref")
		var matchingBlockRefObs []audit.NormalizedObservation
		for _, pbr := range parentBlockRefObs {
			if strings.TrimSpace(pbr.Value) == subjectRef {
				matchingBlockRefObs = append(matchingBlockRefObs, pbr)
			}
		}
		if len(matchingBlockRefObs) == 0 {
			var summary string
			if len(parentBlockRefObs) == 0 {
				summary = fmt.Sprintf("Containing URL %q lacks structured_block_ref pointing to block %q.", urlSubjRefVal, subjectRef)
			} else {
				summary = fmt.Sprintf("Containing URL %q structured_block_ref does not correlate with block %q.", urlSubjRefVal, subjectRef)
			}
			refs := buildBlockContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 7. Validate structured_format
		formatVal, formatUsable, formatConflict := extractSingleObsValue(formatObs)
		if !formatUsable {
			var summary string
			if len(formatObs) == 0 {
				summary = "Required structured_format evidence is missing."
			} else if formatConflict {
				summary = "Conflicting structured_format evidence."
			} else {
				summary = "Malformed structured_format evidence."
			}
			refs := buildBlockContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 8. Validate structured_parse_status
		statusVal, statusUsable, statusConflict := extractSingleObsValue(parseStatusObs)
		if !statusUsable {
			var summary string
			if len(parseStatusObs) == 0 {
				summary = "Required structured_parse_status evidence is missing."
			} else if statusConflict {
				summary = "Conflicting structured_parse_status evidence."
			} else {
				summary = "Malformed structured_parse_status evidence."
			}
			refs := buildBlockContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 9. Format-specific evaluation
		switch formatVal {
		case string(audit.FormatJSONLD):
			rawVal, rawUsable, rawConflict := extractRawObsValue(rawObs)
			if !rawUsable {
				var summary string
				if len(rawObs) == 0 {
					summary = "Required structured_raw evidence is missing for JSON-LD block."
				} else if rawConflict {
					summary = "Conflicting structured_raw evidence for JSON-LD block."
				} else {
					summary = "Malformed structured_raw evidence for JSON-LD block."
				}
				refs := buildBlockContextRefs()
				refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
				results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
				continue
			}

			switch statusVal {
			case parseStatusSuccess:
				if len(parseErrorObs) > 0 {
					summary := "JSON-LD block has PARSE_SUCCESS status but contains contradictory structured_parse_error evidence."
					refs := buildBlockContextRefs()
					refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
					refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
					results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
					continue
				}

				var refs []audit.RuleEvidenceRef
				refs = append(refs, buildEvidenceRefs(ruleResultID, parseStatusObs, "structured_parse_status", audit.EvidenceRolePrimary)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, formatObs, "structured_format", audit.EvidenceRolePrimary)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, blockIDObs, "structured_block_id", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, urlObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, urlSubjRefObs, "url_subject_ref", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, rawObs, "structured_raw", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)

				summary := "Observed JSON-LD block contains syntactically valid JSON."
				results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, summary, expectedSummary, evalTime, refs))

			case parseStatusError:
				errVal, errUsable, errConflict := extractSingleObsValue(parseErrorObs)
				if !errUsable {
					var summary string
					if len(parseErrorObs) == 0 {
						summary = "JSON-LD block failed parsing but lacks required structured_parse_error detail."
					} else if errConflict {
						summary = "Conflicting structured_parse_error evidence for JSON-LD parse error."
					} else {
						summary = "Malformed structured_parse_error evidence for JSON-LD parse error."
					}
					refs := buildBlockContextRefs()
					refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
					refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
					results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
					continue
				}

				var refs []audit.RuleEvidenceRef
				refs = append(refs, buildEvidenceRefs(ruleResultID, parseStatusObs, "structured_parse_status", audit.EvidenceRolePrimary)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, parseErrorObs, "structured_parse_error", audit.EvidenceRolePrimary)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, formatObs, "structured_format", audit.EvidenceRolePrimary)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, blockIDObs, "structured_block_id", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, urlObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, urlSubjRefObs, "url_subject_ref", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, rawObs, "structured_raw", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)

				summary := fmt.Sprintf("Observed JSON-LD block failed deterministic JSON syntax parsing: %s", errVal)
				results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusFail, summary, expectedSummary, evalTime, refs))

			case parseStatusEmptyInput:
				if strings.TrimSpace(rawVal) != "" {
					summary := "JSON-LD block marked EMPTY_INPUT contains non-empty raw content."
					refs := buildBlockContextRefs()
					refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
					refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
					results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
					continue
				}

				var refs []audit.RuleEvidenceRef
				refs = append(refs, buildEvidenceRefs(ruleResultID, parseStatusObs, "structured_parse_status", audit.EvidenceRolePrimary)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, formatObs, "structured_format", audit.EvidenceRolePrimary)...)
				if len(parseErrorObs) > 0 {
					refs = append(refs, buildEvidenceRefs(ruleResultID, parseErrorObs, "structured_parse_error", audit.EvidenceRolePrimary)...)
				}
				refs = append(refs, buildEvidenceRefs(ruleResultID, blockIDObs, "structured_block_id", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, urlObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, urlSubjRefObs, "url_subject_ref", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, rawObs, "structured_raw", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)

				summary := "Observed JSON-LD block is empty or contains only whitespace."
				results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusFail, summary, expectedSummary, evalTime, refs))

			case parseStatusParserUnavailable:
				summary := "JSON-LD parser execution was unavailable or indeterminate."
				refs := buildBlockContextRefs()
				refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
				results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))

			default:
				summary := fmt.Sprintf("Unsupported structured data parse status vocabulary: %q.", statusVal)
				refs := buildBlockContextRefs()
				refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
				results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			}

		case string(audit.FormatMicrodata):
			summary := "Microdata syntax validation is unavailable because raw markup is not preserved by acquisition."
			var refs []audit.RuleEvidenceRef
			refs = append(refs, buildEvidenceRefs(ruleResultID, formatObs, "structured_format", audit.EvidenceRolePrimary)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parseStatusObs, "structured_parse_status", audit.EvidenceRolePrimary)...)
			if len(parseErrorObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, parseErrorObs, "structured_parse_error", audit.EvidenceRoleContext)...)
			}
			refs = append(refs, buildEvidenceRefs(ruleResultID, blockIDObs, "structured_block_id", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlSubjRefObs, "url_subject_ref", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))

		case string(audit.FormatRDFa):
			summary := "RDFa structured data parsing is unsupported by current acquisition capabilities."
			var refs []audit.RuleEvidenceRef
			refs = append(refs, buildEvidenceRefs(ruleResultID, formatObs, "structured_format", audit.EvidenceRolePrimary)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parseStatusObs, "structured_parse_status", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, blockIDObs, "structured_block_id", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlSubjRefObs, "url_subject_ref", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))

		default:
			summary := fmt.Sprintf("Structured data format %q is unsupported.", formatVal)
			var refs []audit.RuleEvidenceRef
			refs = append(refs, buildEvidenceRefs(ruleResultID, formatObs, "structured_format", audit.EvidenceRolePrimary)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parseStatusObs, "structured_parse_status", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, blockIDObs, "structured_block_id", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlSubjRefObs, "url_subject_ref", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, parentURLIdentObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, matchingBlockRefObs, "structured_block_ref", audit.EvidenceRoleContext)...)
			results = append(results, makeBlockResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
		}
	}

	return results, nil
}

// extractRawObsValue extracts the raw structured data string, permitting empty/whitespace strings for EMPTY_INPUT.
func extractRawObsValue(obs []audit.NormalizedObservation) (string, bool, bool) {
	if len(obs) == 0 {
		return "", false, false // missing
	}
	firstVal := obs[0].Value
	for _, o := range obs[1:] {
		if o.Value != firstVal {
			return "", false, true // conflict
		}
	}
	return firstVal, true, false // usable
}

// makeBlockResult constructs an audit.RuleResult for a structured data block subject with deterministic evidence refs.
func makeBlockResult(
	rule audit.RuleDefinition,
	snapshot *audit.EvidenceSnapshot,
	subjectRef string,
	ruleResultID audit.RuleResultID,
	status audit.RuleResultStatus,
	observedSummary string,
	expectedSummary string,
	evalTime time.Time,
	evidenceRefs []audit.RuleEvidenceRef,
) audit.RuleResult {
	return audit.RuleResult{
		RuleResultID:    ruleResultID,
		AuditRunID:      snapshot.AuditRunID,
		SnapshotID:      snapshot.SnapshotID,
		RuleID:          rule.RuleID,
		ParentCheck:     rule.ParentCheck,
		RuleVersion:     rule.RuleVersion,
		SubjectType:     audit.SubjectStructuredDataBlock,
		SubjectRef:      subjectRef,
		Status:          status,
		Severity:        rule.DefaultSeverity,
		Scope:           audit.ScopeURL,
		ObservedSummary: observedSummary,
		ExpectedSummary: expectedSummary,
		EvaluatedAt:     evalTime,
		EvidenceRefs:    dedupeAndSortEvidenceRefs(evidenceRefs),
	}
}
