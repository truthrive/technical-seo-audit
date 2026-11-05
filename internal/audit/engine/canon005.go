package engine

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDCANON005 = "AR-CANON-005"

// evaluateCANON005 implements the typed execution logic for:
// AR-CANON-005 — Canonical URL is syntactically resolvable (parent check: CANON-003, P1, deterministic).
//
// Contract (Approved V1.9b/c/d Specification):
//   - Required inputs: url, canonical_raw, canonical_resolved_url, canonical_parse_error
//     (via url_identity, html_extraction_status, rendered, canonical_count, canonical_raw,
//     canonical_syntax_status, canonical_parse_error, canonical_normalized_target,
//     canonical_normalization_complete)
//   - Preconditions: html_extraction_status == "complete" and canonical_count == 1.
//   - PASS: html_extraction_status == "complete", canonical_count == 1,
//     canonical_syntax_status == "valid", canonical_normalization_complete == "true",
//     and exactly one non-empty canonical_normalized_target.
//   - FAIL: html_extraction_status == "complete", canonical_count == 1,
//     canonical_syntax_status in {"parser_error", "resolution_error"}, and
//     canonical_parse_error carries a proven parser_error:* or resolution_error:* code.
//   - UNKNOWN: html_extraction_status != "complete" (missing_legacy, truncated_size_limit,
//     read_error, or absent), rendered == "true", missing/conflicting/malformed observations,
//     or canonical_syntax_status == "insufficient_evidence" / unproven normalization failure.
//     Never infer FAIL solely from canonical_normalization_complete == "false".
//   - NOT_APPLICABLE: html_extraction_status == "complete" and (canonical_count == 0 or canonical_count > 1).
//   - No WARNING or MANUAL_REVIEW.
func evaluateCANON005(
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

	urlRefs := idx.SubjectRefs(audit.SubjectURL)
	results := make([]audit.RuleResult, 0, len(urlRefs))

	for _, subjectRef := range urlRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// 1. Resolve URL identity
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		var (
			urlUsable   bool
			urlConflict bool
			firstURL    string
		)
		if len(urlObs) > 0 {
			firstURL = strings.TrimSpace(urlObs[0].Value)
			if firstURL != "" {
				urlUsable = true
				for _, u := range urlObs[1:] {
					if strings.TrimSpace(u.Value) != firstURL {
						urlConflict = true
						urlUsable = false
						break
					}
				}
			}
		}

		buildURLContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, u := range urlObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url", ruleResultID, u.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(u.ObservationID),
					Field:             "url",
					ObservedValue:     u.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			return refs
		}

		// 2. Resolve rendered state
		renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "rendered")
		var (
			isRendered         bool
			renderedConflict   bool
			renderedUnreliable bool
		)
		if len(renderedObs) > 0 {
			firstRend := strings.TrimSpace(renderedObs[0].Value)
			for _, ro := range renderedObs[1:] {
				if strings.TrimSpace(ro.Value) != firstRend {
					renderedConflict = true
					renderedUnreliable = true
					break
				}
			}
			if !renderedConflict {
				switch firstRend {
				case "true":
					isRendered = true
				case "false":
					isRendered = false
				default:
					renderedUnreliable = true
				}
			}
		}

		buildRenderedRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, ro := range renderedObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:rendered", ruleResultID, ro.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(ro.ObservationID),
					Field:             "rendered",
					ObservedValue:     ro.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 3. Resolve html_extraction_status
		extractObs := idx.GetObservations(audit.SubjectURL, subjectRef, "html_extraction_status")
		var (
			extractUsable    bool
			extractConflict  bool
			extractMalformed bool
			extractStatus    string
		)
		if len(extractObs) > 0 {
			extractStatus = strings.TrimSpace(extractObs[0].Value)
			for _, eo := range extractObs[1:] {
				if strings.TrimSpace(eo.Value) != extractStatus {
					extractConflict = true
					break
				}
			}
			if !extractConflict {
				switch extractStatus {
				case "complete", "truncated_size_limit", "read_error", "missing_legacy":
					extractUsable = true
				default:
					extractMalformed = true
				}
			}
		}

		buildExtractRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, eo := range extractObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:html_extraction_status", ruleResultID, eo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(eo.ObservationID),
					Field:             "html_extraction_status",
					ObservedValue:     eo.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 4. Resolve canonical_count
		countObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_count")
		var (
			countUsable    bool
			countConflict  bool
			countMalformed bool
			firstCountStr  string
			canonCount     int
		)
		if len(countObs) > 0 {
			firstCountStr = strings.TrimSpace(countObs[0].Value)
			countUsable = true
			for _, co := range countObs[1:] {
				if strings.TrimSpace(co.Value) != firstCountStr {
					countConflict = true
					countUsable = false
					break
				}
			}
			if !countConflict {
				c, err := strconv.Atoi(firstCountStr)
				if err != nil || c < 0 {
					countMalformed = true
					countUsable = false
				} else {
					canonCount = c
				}
			}
		}

		buildCountRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, co := range countObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_count", ruleResultID, co.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(co.ObservationID),
					Field:             "canonical_count",
					ObservedValue:     co.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 5. Resolve canonical_sample_truncated (context when >5 declarations)
		sampleTruncObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_sample_truncated")
		var (
			sampleTruncConflict  bool
			sampleTruncMalformed bool
			sampleTruncVal       bool
		)
		if len(sampleTruncObs) > 0 {
			firstTrunc := strings.TrimSpace(sampleTruncObs[0].Value)
			for _, to := range sampleTruncObs[1:] {
				if strings.TrimSpace(to.Value) != firstTrunc {
					sampleTruncConflict = true
					break
				}
			}
			if !sampleTruncConflict {
				switch firstTrunc {
				case "true":
					sampleTruncVal = true
				case "false":
					sampleTruncVal = false
				default:
					sampleTruncMalformed = true
				}
			}
		}

		buildSampleTruncRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, to := range sampleTruncObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_sample_truncated", ruleResultID, to.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(to.ObservationID),
					Field:             "canonical_sample_truncated",
					ObservedValue:     to.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 6. Resolve canonical_raw
		rawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_raw")
		var (
			rawUsable   bool
			rawConflict bool
			firstRaw    string
		)
		if len(rawObs) > 0 {
			firstRaw = rawObs[0].Value
			rawUsable = true
			for _, ro := range rawObs[1:] {
				if ro.Value != firstRaw {
					rawConflict = true
					rawUsable = false
					break
				}
			}
		}

		buildRawRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, ro := range rawObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_raw", ruleResultID, ro.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(ro.ObservationID),
					Field:             "canonical_raw",
					ObservedValue:     ro.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 7. Resolve canonical_syntax_status
		syntaxObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_syntax_status")
		var (
			syntaxUsable    bool
			syntaxConflict  bool
			syntaxMalformed bool
			syntaxStatus    string
		)
		if len(syntaxObs) > 0 {
			syntaxStatus = strings.TrimSpace(syntaxObs[0].Value)
			for _, so := range syntaxObs[1:] {
				if strings.TrimSpace(so.Value) != syntaxStatus {
					syntaxConflict = true
					break
				}
			}
			if !syntaxConflict {
				switch syntaxStatus {
				case "valid", "parser_error", "resolution_error", "insufficient_evidence":
					syntaxUsable = true
				default:
					syntaxMalformed = true
				}
			}
		}

		buildSyntaxRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, so := range syntaxObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_syntax_status", ruleResultID, so.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(so.ObservationID),
					Field:             "canonical_syntax_status",
					ObservedValue:     so.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 8. Resolve canonical_parse_error
		parseErrObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_parse_error")
		var (
			parseErrUsable   bool
			parseErrConflict bool
			parseErrVal      string
		)
		if len(parseErrObs) > 0 {
			parseErrVal = strings.TrimSpace(parseErrObs[0].Value)
			if parseErrVal != "" {
				parseErrUsable = true
				for _, po := range parseErrObs[1:] {
					if strings.TrimSpace(po.Value) != parseErrVal {
						parseErrConflict = true
						parseErrUsable = false
						break
					}
				}
			}
		}

		buildParseErrRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, po := range parseErrObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_parse_error", ruleResultID, po.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(po.ObservationID),
					Field:             "canonical_parse_error",
					ObservedValue:     po.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 9. Resolve canonical_normalized_target
		normTargetObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_normalized_target")
		var (
			normTargetUsable   bool
			normTargetConflict bool
			firstNormTarget    string
		)
		if len(normTargetObs) > 0 {
			firstNormTarget = strings.TrimSpace(normTargetObs[0].Value)
			if firstNormTarget != "" {
				normTargetUsable = true
				for _, no := range normTargetObs[1:] {
					if strings.TrimSpace(no.Value) != firstNormTarget {
						normTargetConflict = true
						normTargetUsable = false
						break
					}
				}
			}
		}

		buildNormTargetRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, no := range normTargetObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_resolved_url", ruleResultID, no.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(no.ObservationID),
					Field:             "canonical_resolved_url",
					ObservedValue:     no.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 10. Resolve canonical_normalization_complete
		completeObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_normalization_complete")
		var (
			completeUsable    bool
			completeConflict  bool
			completeMalformed bool
			firstCompleteStr  string
			completeVal       bool
		)
		if len(completeObs) > 0 {
			firstCompleteStr = strings.TrimSpace(completeObs[0].Value)
			completeUsable = true
			for _, co := range completeObs[1:] {
				if strings.TrimSpace(co.Value) != firstCompleteStr {
					completeConflict = true
					completeUsable = false
					break
				}
			}
			if !completeConflict {
				switch firstCompleteStr {
				case "true":
					completeVal = true
				case "false":
					completeVal = false
				default:
					completeMalformed = true
					completeUsable = false
				}
			}
		}

		buildCompleteRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, co := range completeObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_normalization_complete", ruleResultID, co.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(co.ObservationID),
					Field:             "canonical_normalization_complete",
					ObservedValue:     co.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 11. Determine status, summary, and evidence refs
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
			evidenceRefs    = buildURLContextRefs()
		)

		if !urlUsable {
			evalStatus = audit.StatusUnknown
			if urlConflict {
				observedSummary = "No usable URL identity is available: conflicting URL identity observations."
			} else {
				observedSummary = "Required URL identity evidence is unavailable."
			}
			evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if renderedUnreliable {
			evalStatus = audit.StatusUnknown
			if renderedConflict {
				observedSummary = "Rendered state evidence is conflicting."
			} else {
				observedSummary = "Rendered state evidence is malformed."
			}
			evidenceRefs = append(evidenceRefs, buildRenderedRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
		} else if isRendered {
			evalStatus = audit.StatusUnknown
			observedSummary = "Page was rendered via headless browser without retained raw server HTML; static canonical syntax cannot be verified."
			evidenceRefs = append(evidenceRefs, buildRenderedRefs(audit.EvidenceRolePrimary)...)
			evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
		} else if !extractUsable {
			evalStatus = audit.StatusUnknown
			if len(extractObs) == 0 {
				observedSummary = "Required HTML extraction completeness evidence is unavailable."
			} else if extractConflict {
				observedSummary = "HTML extraction completeness evidence is conflicting."
			} else if extractMalformed {
				observedSummary = fmt.Sprintf("HTML extraction completeness evidence is malformed: %q.", extractStatus)
			}
			evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if extractStatus != "complete" {
			evalStatus = audit.StatusUnknown
			switch extractStatus {
			case "truncated_size_limit":
				observedSummary = "HTML response body exceeded the extraction size limit; canonical source completeness is unverified."
			case "read_error":
				observedSummary = "HTML response stream encountered a transport or read error; canonical source completeness is unverified."
			default:
				observedSummary = "Static HTML extraction completeness is unavailable or legacy-unverified (missing_legacy)."
			}
			evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRolePrimary)...)
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if !countUsable {
			evalStatus = audit.StatusUnknown
			if len(countObs) == 0 {
				observedSummary = "Required canonical count evidence is unavailable."
			} else if countConflict {
				observedSummary = "Canonical count evidence is conflicting."
			} else if countMalformed {
				observedSummary = fmt.Sprintf("Canonical count evidence is malformed: %q.", firstCountStr)
			}
			evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if sampleTruncConflict || sampleTruncMalformed {
			evalStatus = audit.StatusUnknown
			if sampleTruncConflict {
				observedSummary = "Canonical sample truncation evidence is conflicting."
			} else {
				observedSummary = "Canonical sample truncation evidence is malformed."
			}
			evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildSampleTruncRefs(audit.EvidenceRoleContext)...)
		} else if canonCount == 0 {
			evalStatus = audit.StatusNotApplicable
			observedSummary = "Page has no canonical declarations (canonical_count = 0)."
			evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if canonCount > 1 {
			if len(sampleTruncObs) > 0 && sampleTruncVal && canonCount <= 5 {
				evalStatus = audit.StatusUnknown
				observedSummary = "Canonical sample truncation evidence conflicts with canonical_count <= 5."
				evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildSampleTruncRefs(audit.EvidenceRoleContext)...)
			} else {
				evalStatus = audit.StatusNotApplicable
				observedSummary = fmt.Sprintf("Page has %d canonical declarations; single-canonical syntax check is not applicable.", canonCount)
				evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildSampleTruncRefs(audit.EvidenceRoleContext)...)
			}
		} else {
			// html_extraction_status == "complete" and canonCount == 1
			if rawConflict || syntaxConflict || syntaxMalformed || parseErrConflict || normTargetConflict || completeConflict || completeMalformed {
				evalStatus = audit.StatusUnknown
				observedSummary = "Single canonical syntax or normalization evidence is conflicting or malformed."
				evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildRawRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildSyntaxRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildParseErrRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildNormTargetRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
			} else if len(sampleTruncObs) > 0 && sampleTruncVal {
				evalStatus = audit.StatusUnknown
				observedSummary = "Canonical sample truncation evidence conflicts with canonical_count = 1."
				evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildSampleTruncRefs(audit.EvidenceRoleContext)...)
			} else if !syntaxUsable || !rawUsable {
				evalStatus = audit.StatusUnknown
				observedSummary = "Single canonical declaration exists, but raw canonical or syntax classification evidence is unavailable."
				evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildRawRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildSyntaxRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildParseErrRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildNormTargetRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
			} else if syntaxStatus == "valid" {
				if parseErrUsable || !completeUsable || !completeVal || !normTargetUsable || len(normTargetObs) != 1 {
					evalStatus = audit.StatusUnknown
					observedSummary = "Canonical syntax is marked valid, but resolved target or normalization completeness evidence is missing or contradictory."
					evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildRawRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildSyntaxRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildParseErrRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildNormTargetRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
				} else {
					evalStatus = audit.StatusPass
					observedSummary = fmt.Sprintf("Page has a single canonical declaration (%q) that resolves to a valid HTTP(S) URL (%s).", firstRaw, firstNormTarget)
					evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildRawRefs(audit.EvidenceRoleSupporting)...)
					evidenceRefs = append(evidenceRefs, buildSyntaxRefs(audit.EvidenceRoleSupporting)...)
					evidenceRefs = append(evidenceRefs, buildNormTargetRefs(audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
				}
			} else if syntaxStatus == "parser_error" || syntaxStatus == "resolution_error" {
				expectedPrefix := syntaxStatus + ":"
				if !parseErrUsable || !strings.HasPrefix(parseErrVal, expectedPrefix) || (completeUsable && completeVal) || normTargetUsable {
					evalStatus = audit.StatusUnknown
					observedSummary = "Canonical syntax error status lacks a consistent parse error code or conflicts with resolved target evidence."
					evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildRawRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildSyntaxRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildParseErrRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildNormTargetRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
				} else {
					evalStatus = audit.StatusFail
					observedSummary = fmt.Sprintf("Page has a single canonical declaration (%q) with a proven %s (%s).", firstRaw, strings.ReplaceAll(syntaxStatus, "_", " "), parseErrVal)
					evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildRawRefs(audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildSyntaxRefs(audit.EvidenceRoleSupporting)...)
					evidenceRefs = append(evidenceRefs, buildParseErrRefs(audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
				}
			} else {
				// syntaxStatus == "insufficient_evidence" (including Scenario 6a-ii, 6c, and 19)
				evalStatus = audit.StatusUnknown
				if parseErrUsable {
					observedSummary = fmt.Sprintf("Single canonical declaration (%q) cannot be deterministically verified due to insufficient evidence (%s).", firstRaw, parseErrVal)
				} else {
					observedSummary = fmt.Sprintf("Single canonical declaration (%q) cannot be deterministically verified due to insufficient evidence.", firstRaw)
				}
				evidenceRefs = append(evidenceRefs, buildExtractRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildRawRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildSyntaxRefs(audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildParseErrRefs(audit.EvidenceRoleSupporting)...)
				evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
			}
		}

		// Deduplicate and sort evidence refs deterministically
		seenRefIDs := make(map[audit.RuleEvidenceRefID]struct{}, len(evidenceRefs))
		dedupedRefs := make([]audit.RuleEvidenceRef, 0, len(evidenceRefs))
		for _, ref := range evidenceRefs {
			if _, exists := seenRefIDs[ref.RuleEvidenceRefID]; !exists {
				seenRefIDs[ref.RuleEvidenceRefID] = struct{}{}
				dedupedRefs = append(dedupedRefs, ref)
			}
		}
		sort.Slice(dedupedRefs, func(i, j int) bool {
			return dedupedRefs[i].RuleEvidenceRefID < dedupedRefs[j].RuleEvidenceRefID
		})

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     audit.SubjectURL,
			SubjectRef:      subjectRef,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeURL,
			ObservedSummary: observedSummary,
			ExpectedSummary: "Single canonical declaration is syntactically valid and resolves to an HTTP(S) URL.",
			EvaluatedAt:     evalTime,
			EvidenceRefs:    dedupedRefs,
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
		}

		results = append(results, rr)
	}

	return results, nil
}
