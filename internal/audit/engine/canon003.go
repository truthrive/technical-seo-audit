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

const ruleIDCANON003 = "AR-CANON-003"

// isHTMLContentType returns true if the Content-Type header specifies an HTML media type.
func isHTMLContentType(raw string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(raw))
	if idx := strings.Index(mediaType, ";"); idx != -1 {
		mediaType = strings.TrimSpace(mediaType[:idx])
	}
	switch mediaType {
	case "text/html", "application/xhtml+xml", "application/xhtml", "text/xhtml":
		return true
	default:
		return strings.Contains(mediaType, "html")
	}
}

// evaluateCANON003 implements the typed execution logic for:
// AR-CANON-003 — Canonical declaration missing (parent check: CANON-003, P1, assisted).
//
// Contract:
// - Required inputs: url, fetch_status, content_type, canonical_values
// - Precondition: final response is 2xx HTML.
// - PASS: 2xx HTML + at least one non-empty canonical declaration present.
// - WARNING: 2xx HTML + no canonical declaration present (on unrendered/reliable head extraction).
// - UNKNOWN: HTML/head extraction unavailable (e.g. rendered=true with no raw canonical,
//            missing/conflicting/malformed URL identity, status, or content type on 2xx).
// - NOT_APPLICABLE: non-HTML or non-2xx.
// - No FAIL or MANUAL_REVIEW.
func evaluateCANON003(
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

		// Helper to build URL context evidence refs
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

		// 2. Resolve HTTP status
		statusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "http_status")
		var (
			statusUsable    bool
			statusConflict  bool
			statusMalformed bool
			statusVal       string
			statusCode      int
		)
		if len(statusObs) > 0 {
			statusVal = strings.TrimSpace(statusObs[0].Value)
			statusUsable = true
			for _, so := range statusObs[1:] {
				if strings.TrimSpace(so.Value) != statusVal {
					statusConflict = true
					statusUsable = false
					break
				}
			}
			if !statusConflict {
				code, err := strconv.Atoi(statusVal)
				if err != nil || code <= 0 {
					statusMalformed = true
					statusUsable = false
				} else {
					statusCode = code
				}
			}
		}

		buildStatusContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, so := range statusObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:fetch_status", ruleResultID, so.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(so.ObservationID),
					Field:             "fetch_status",
					ObservedValue:     so.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			return refs
		}

		// 3. Resolve Content-Type
		ctObs := idx.GetObservations(audit.SubjectURL, subjectRef, "content_type")
		var (
			ctUsable   bool
			ctConflict bool
			firstCT    string
			isHTML     bool
		)
		if len(ctObs) > 0 {
			firstCT = strings.TrimSpace(ctObs[0].Value)
			if firstCT != "" {
				ctUsable = true
				for _, co := range ctObs[1:] {
					if strings.TrimSpace(co.Value) != firstCT {
						ctConflict = true
						ctUsable = false
						break
					}
				}
				if !ctConflict {
					isHTML = isHTMLContentType(firstCT)
				}
			}
		}

		buildContentTypeContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, co := range ctObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:content_type", ruleResultID, co.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(co.ObservationID),
					Field:             "content_type",
					ObservedValue:     co.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			return refs
		}

		// 4. Resolve Rendered state
		renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "rendered")
		var (
			isRendered         bool
			renderedUnreliable bool
		)
		if len(renderedObs) > 0 {
			firstRend := strings.TrimSpace(renderedObs[0].Value)
			for _, ro := range renderedObs[1:] {
				if strings.TrimSpace(ro.Value) != firstRend {
					renderedUnreliable = true
					break
				}
			}
			if !renderedUnreliable {
				if firstRend == "true" {
					isRendered = true
				} else if firstRend == "false" {
					isRendered = false
				} else {
					renderedUnreliable = true
				}
			}
		}

		buildRenderedContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, ro := range renderedObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:rendered", ruleResultID, ro.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(ro.ObservationID),
					Field:             "rendered",
					ObservedValue:     ro.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			return refs
		}

		// 5. Resolve Canonical declarations
		canonObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_resolved")
		var nonBlankCanonObs []audit.NormalizedObservation
		for _, co := range canonObs {
			if strings.TrimSpace(co.Value) != "" {
				nonBlankCanonObs = append(nonBlankCanonObs, co)
			}
		}
		hasCanonical := len(nonBlankCanonObs) > 0

		// 6. Determine status, summary, and evidence refs
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
			evidenceRefs = append(evidenceRefs, buildStatusContextRefs()...)
		} else if !statusUsable {
			evalStatus = audit.StatusUnknown
			if len(statusObs) == 0 {
				observedSummary = "Required HTTP status evidence is unavailable."
			} else if statusConflict {
				observedSummary = "No usable HTTP response status is available: conflicting status observations."
			} else if statusMalformed {
				observedSummary = fmt.Sprintf("No usable HTTP response status is available: malformed status %q.", statusVal)
			}
			evidenceRefs = append(evidenceRefs, buildStatusContextRefs()...)
		} else if statusCode < 200 || statusCode > 299 {
			// Non-2xx -> NOT_APPLICABLE
			evalStatus = audit.StatusNotApplicable
			observedSummary = fmt.Sprintf("HTTP status %d is not a 2xx response.", statusCode)
			evidenceRefs = append(evidenceRefs, buildStatusContextRefs()...)
		} else if !ctUsable {
			// Missing, empty, or conflicting Content-Type on a 2xx response -> UNKNOWN
			evalStatus = audit.StatusUnknown
			if len(ctObs) == 0 || firstCT == "" {
				observedSummary = "Required Content-Type evidence is unavailable on a 2xx response."
				evidenceRefs = append(evidenceRefs, buildStatusContextRefs()...)
				if len(ctObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildContentTypeContextRefs()...)
				}
			} else if ctConflict {
				observedSummary = "No usable Content-Type is available: conflicting content type observations."
				evidenceRefs = append(evidenceRefs, buildStatusContextRefs()...)
				evidenceRefs = append(evidenceRefs, buildContentTypeContextRefs()...)
			}
		} else if !isHTML {
			// Known non-HTML -> NOT_APPLICABLE
			evalStatus = audit.StatusNotApplicable
			observedSummary = fmt.Sprintf("Content-Type %q is non-HTML.", firstCT)
			evidenceRefs = append(evidenceRefs, buildStatusContextRefs()...)
			evidenceRefs = append(evidenceRefs, buildContentTypeContextRefs()...)
		} else {
			// 2xx HTML response verified
			evidenceRefs = append(evidenceRefs, buildStatusContextRefs()...)
			evidenceRefs = append(evidenceRefs, buildContentTypeContextRefs()...)
			if len(renderedObs) > 0 {
				evidenceRefs = append(evidenceRefs, buildRenderedContextRefs()...)
			}

			if hasCanonical {
				// Canonical declaration present -> PASS
				evalStatus = audit.StatusPass
				observedSummary = fmt.Sprintf("2xx HTML page has %d canonical declaration(s).", len(nonBlankCanonObs))

				// Include canonical observations as PRIMARY
				for _, co := range nonBlankCanonObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_resolved", ruleResultID, co.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(co.ObservationID),
						Field:             "canonical_resolved",
						ObservedValue:     co.Value,
						Role:              audit.EvidenceRolePrimary,
					})
				}
			} else {
				// No canonical declaration found
				if isRendered || renderedUnreliable {
					// Rendered page with no raw canonical -> UNKNOWN
					evalStatus = audit.StatusUnknown
					if renderedUnreliable {
						observedSummary = "Rendered state evidence is conflicting or malformed, head extraction completeness is unknown."
					} else {
						observedSummary = "Page was rendered via headless browser and raw head evidence is unavailable; canonical extraction completeness is unknown."
					}
				} else {
					// Unrendered 2xx HTML with verified extraction completeness and no canonical -> WARNING
					evalStatus = audit.StatusWarning
					observedSummary = "2xx HTML response has no canonical declaration."
					// WARNING has no synthetic "canonical missing" observation.
				}
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
			ExpectedSummary: "2xx HTML page has at least one canonical declaration.",
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
