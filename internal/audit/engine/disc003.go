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

const ruleIDDISC003 = "AR-DISC-003"

// evaluateDISC003 implements the typed execution logic for:
// AR-DISC-003 — Sitemap is parseable (parent check: DISC-002, P1, deterministic).
//
// Subject: SubjectSitemap (sm:<audit_run_id>:<sitemap_id>)
// Unit of evaluation: The sitemap document.
//
// Preconditions: sitemap returned 200 and body retrieved.
// Contract:
// - PASS: A retrieved sitemap document is successfully parsed as a supported sitemap (URLSET)
//         or sitemap index (SITEMAP_INDEX).
// - FAIL: A retrieved sitemap document is malformed or cannot be parsed as a supported sitemap format
//         (xml_error, unsupported_structure, or empty_input).
// - UNKNOWN: Parser execution could not be completed or its outcome is unavailable
//            (unavailable, missing parse status, missing/conflicting URL identity, or contradictory evidence).
// - NOT_APPLICABLE: The sitemap body was not retrieved
//                   (not_attempted with non-2xx status, HTTP 404/500, network failure before response body).
//
// Consumed observations on SubjectSitemap:
// - sitemap_url            (Context)
// - sitemap_document_type  (Primary)
// - sitemap_parse_status   (Primary)
// - sitemap_parse_error    (Primary, optional)
// - sitemap_fetch_complete (Context)
// - initial_http_status    (Context)
// - final_http_status      (Context)
//
// Evidence boundaries:
// HTTP STATUS != XML PARSE RESULT
// XML PARSE RESULT != SEO RANKING SIGNAL
// MISSING != FALSE
// OBSERVATION != CONCLUSION
func evaluateDISC003(
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

	sitemapRefs := idx.SubjectRefs(audit.SubjectSitemap)
	if len(sitemapRefs) == 0 {
		return nil, nil
	}

	results := make([]audit.RuleResult, 0, len(sitemapRefs))

	for _, subjectRef := range sitemapRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// 1. Resolve sitemap_url (URL identity context)
		urlObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_url")
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

		// 2. Resolve final_http_status
		finalStatusObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "final_http_status")
		var (
			finalStatusUsable    bool
			finalStatusConflict  bool
			finalStatusCode      int
			finalStatusMalformed bool
			firstFinalStatusVal  string
		)
		if len(finalStatusObs) > 0 {
			firstFinalStatusVal = strings.TrimSpace(finalStatusObs[0].Value)
			finalStatusUsable = true
			for _, fso := range finalStatusObs[1:] {
				if strings.TrimSpace(fso.Value) != firstFinalStatusVal {
					finalStatusConflict = true
					finalStatusUsable = false
					break
				}
			}
			if !finalStatusConflict {
				code, err := strconv.Atoi(firstFinalStatusVal)
				if err != nil {
					finalStatusMalformed = true
					finalStatusUsable = false
				} else {
					finalStatusCode = code
				}
			}
		}

		// 3. Resolve initial_http_status (optional context)
		initStatusObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "initial_http_status")
		var initStatusConflict bool
		if len(initStatusObs) > 1 {
			firstInit := strings.TrimSpace(initStatusObs[0].Value)
			for _, iso := range initStatusObs[1:] {
				if strings.TrimSpace(iso.Value) != firstInit {
					initStatusConflict = true
					break
				}
			}
		}

		// 4. Resolve sitemap_fetch_complete
		fetchCompleteObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_fetch_complete")
		var (
			fetchCompleteUsable   bool
			fetchCompleteConflict bool
			fetchCompleteVal      bool
		)
		if len(fetchCompleteObs) > 0 {
			firstFC := strings.TrimSpace(fetchCompleteObs[0].Value)
			fetchCompleteUsable = true
			for _, fco := range fetchCompleteObs[1:] {
				if strings.TrimSpace(fco.Value) != firstFC {
					fetchCompleteConflict = true
					fetchCompleteUsable = false
					break
				}
			}
			if !fetchCompleteConflict {
				b, err := strconv.ParseBool(firstFC)
				if err == nil {
					fetchCompleteVal = b
				} else {
					fetchCompleteUsable = false
				}
			}
		}

		// 5. Resolve sitemap_parse_status
		parseStatusObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_parse_status")
		var (
			parseStatusUsable   bool
			parseStatusConflict bool
			parseStatusVal      string
		)
		if len(parseStatusObs) > 0 {
			firstPS := strings.ToLower(strings.TrimSpace(parseStatusObs[0].Value))
			parseStatusVal = firstPS
			parseStatusUsable = true
			for _, pso := range parseStatusObs[1:] {
				if strings.ToLower(strings.TrimSpace(pso.Value)) != firstPS {
					parseStatusConflict = true
					parseStatusUsable = false
					break
				}
			}
		}

		// 6. Resolve sitemap_document_type
		docTypeObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_document_type")
		var (
			docTypeUsable   bool
			docTypeConflict bool
			docTypeVal      string
		)
		if len(docTypeObs) > 0 {
			firstDT := strings.ToUpper(strings.TrimSpace(docTypeObs[0].Value))
			docTypeVal = firstDT
			docTypeUsable = true
			for _, dto := range docTypeObs[1:] {
				if strings.ToUpper(strings.TrimSpace(dto.Value)) != firstDT {
					docTypeConflict = true
					docTypeUsable = false
					break
				}
			}
		}

		// 7. Resolve sitemap_parse_error (optional)
		parseErrObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_parse_error")
		var (
			parseErrConflict bool
			parseErrVal      string
		)
		if len(parseErrObs) > 0 {
			firstPE := strings.TrimSpace(parseErrObs[0].Value)
			parseErrVal = firstPE
			for _, peo := range parseErrObs[1:] {
				if strings.TrimSpace(peo.Value) != firstPE {
					parseErrConflict = true
					break
				}
			}
		}

		// Determine verdict and summary
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		hasInternalContradiction := urlConflict || finalStatusConflict || initStatusConflict ||
			fetchCompleteConflict || parseStatusConflict || docTypeConflict || parseErrConflict

		if hasInternalContradiction {
			evalStatus = audit.StatusUnknown
			observedSummary = "Inconsistent or contradictory observations for sitemap."
		} else if !urlUsable {
			evalStatus = audit.StatusUnknown
			observedSummary = "Required sitemap URL identity evidence is unavailable."
		} else if len(finalStatusObs) == 0 || finalStatusMalformed {
			evalStatus = audit.StatusUnknown
			observedSummary = "Required final HTTP status evidence is unavailable or malformed."
		} else if len(parseStatusObs) == 0 {
			// Parse status observation is missing entirely
			if finalStatusCode >= 200 && finalStatusCode <= 299 {
				evalStatus = audit.StatusUnknown
				observedSummary = "Required sitemap parse status evidence is unavailable."
			} else {
				evalStatus = audit.StatusNotApplicable
				observedSummary = "Sitemap document body was not retrieved."
			}
		} else {
			// We have a single consistent parseStatusVal and finalStatusCode
			is2xx := finalStatusCode >= 200 && finalStatusCode <= 299

			switch parseStatusVal {
			case "parsed":
				if !is2xx {
					evalStatus = audit.StatusUnknown
					observedSummary = fmt.Sprintf("Contradictory evidence: sitemap returned HTTP %d but reported parsed status.", finalStatusCode)
				} else if docTypeUsable && docTypeVal == "URLSET" {
					evalStatus = audit.StatusPass
					observedSummary = "Sitemap document was successfully parsed as a supported urlset."
				} else if docTypeUsable && docTypeVal == "SITEMAP_INDEX" {
					evalStatus = audit.StatusPass
					observedSummary = "Sitemap document was successfully parsed as a supported sitemap index."
				} else {
					evalStatus = audit.StatusUnknown
					observedSummary = fmt.Sprintf("Contradictory evidence: sitemap parse status is parsed but document type %q is unsupported.", docTypeVal)
				}

			case "xml_error":
				if !is2xx {
					evalStatus = audit.StatusUnknown
					observedSummary = fmt.Sprintf("Contradictory evidence: sitemap returned HTTP %d but reported xml_error parse status.", finalStatusCode)
				} else if docTypeVal == "URLSET" || docTypeVal == "SITEMAP_INDEX" {
					evalStatus = audit.StatusUnknown
					observedSummary = "Contradictory evidence: sitemap reported xml_error but indicated a supported document type."
				} else {
					evalStatus = audit.StatusFail
					if parseErrVal != "" {
						observedSummary = fmt.Sprintf("Sitemap document is malformed XML: %s.", parseErrVal)
					} else {
						observedSummary = "Sitemap document is malformed XML."
					}
				}

			case "unsupported_structure":
				if !is2xx {
					evalStatus = audit.StatusUnknown
					observedSummary = fmt.Sprintf("Contradictory evidence: sitemap returned HTTP %d but reported unsupported_structure parse status.", finalStatusCode)
				} else if docTypeVal == "URLSET" || docTypeVal == "SITEMAP_INDEX" {
					evalStatus = audit.StatusUnknown
					observedSummary = "Contradictory evidence: sitemap reported unsupported_structure but indicated a supported document type."
				} else {
					evalStatus = audit.StatusFail
					observedSummary = "Sitemap document structure is unsupported; document contains no valid URL or sitemap entries."
				}

			case "empty_input":
				if !is2xx {
					evalStatus = audit.StatusUnknown
					observedSummary = fmt.Sprintf("Contradictory evidence: sitemap returned HTTP %d but reported empty_input parse status.", finalStatusCode)
				} else if docTypeVal == "URLSET" || docTypeVal == "SITEMAP_INDEX" {
					evalStatus = audit.StatusUnknown
					observedSummary = "Contradictory evidence: sitemap reported empty_input but indicated a supported document type."
				} else {
					evalStatus = audit.StatusFail
					observedSummary = "Retrieved sitemap document body is empty."
				}

			case "unavailable":
				evalStatus = audit.StatusUnknown
				if parseErrVal != "" {
					observedSummary = fmt.Sprintf("Sitemap parser execution could not be completed: %s.", parseErrVal)
				} else {
					observedSummary = "Sitemap parser execution could not be completed or parser outcome is unavailable."
				}

			case "not_attempted":
				if !is2xx || (fetchCompleteUsable && !fetchCompleteVal) || finalStatusCode == 0 {
					evalStatus = audit.StatusNotApplicable
					if finalStatusCode > 0 {
						observedSummary = fmt.Sprintf("Sitemap document body was not retrieved (HTTP status %d).", finalStatusCode)
					} else {
						observedSummary = "Sitemap document body was not retrieved due to acquisition failure."
					}
				} else {
					// 2xx response but parsing was not attempted
					evalStatus = audit.StatusUnknown
					observedSummary = "HTTP response 200 was received but sitemap parser execution was not attempted."
				}

			default:
				evalStatus = audit.StatusUnknown
				observedSummary = fmt.Sprintf("Unrecognized sitemap parse status %q.", parseStatusVal)
			}
		}

		// Build traceable evidence references
		var evidenceRefs []audit.RuleEvidenceRef

		// sitemap_url (Context)
		for _, obs := range urlObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_url", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_url",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// final_http_status (Context)
		for _, obs := range finalStatusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:final_http_status", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "final_http_status",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// initial_http_status (Context)
		for _, obs := range initStatusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:initial_http_status", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "initial_http_status",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// sitemap_fetch_complete (Context)
		for _, obs := range fetchCompleteObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_fetch_complete", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_fetch_complete",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// sitemap_parse_status (Primary)
		for _, obs := range parseStatusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_parse_status", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_parse_status",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		// sitemap_document_type (Primary)
		for _, obs := range docTypeObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_document_type", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_document_type",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		// sitemap_parse_error (Primary)
		for _, obs := range parseErrObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_parse_error", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_parse_error",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		// Deduplicate and sort evidence refs deterministically by RuleEvidenceRefID
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

		// Guard: PASS or FAIL requires usable inputs
		if evalStatus == audit.StatusPass || evalStatus == audit.StatusFail {
			if !urlUsable || !finalStatusUsable || !parseStatusUsable || (evalStatus == audit.StatusPass && !docTypeUsable) {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s produced %s without usable inputs", ruleResultID, evalStatus)
			}
			var hasURLContext, hasFinalStatusContext, hasParseStatusPrimary, hasDocTypePrimary bool
			for _, ref := range dedupedRefs {
				if ref.Field == "sitemap_url" && ref.Role == audit.EvidenceRoleContext {
					hasURLContext = true
				}
				if ref.Field == "final_http_status" && ref.Role == audit.EvidenceRoleContext {
					hasFinalStatusContext = true
				}
				if ref.Field == "sitemap_parse_status" && ref.Role == audit.EvidenceRolePrimary {
					hasParseStatusPrimary = true
				}
				if ref.Field == "sitemap_document_type" && ref.Role == audit.EvidenceRolePrimary {
					hasDocTypePrimary = true
				}
			}
			if !hasURLContext || !hasFinalStatusContext || !hasParseStatusPrimary || (evalStatus == audit.StatusPass && !hasDocTypePrimary) {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s missing required evidence refs (url: %v, status: %v, parse: %v, docType: %v)",
					ruleResultID, hasURLContext, hasFinalStatusContext, hasParseStatusPrimary, hasDocTypePrimary)
			}
		}

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     audit.SubjectSitemap,
			SubjectRef:      subjectRef,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeSiteConfiguration,
			ObservedSummary: observedSummary,
			ExpectedSummary: "Sitemap document is successfully parsed as a supported sitemap or sitemap index.",
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
