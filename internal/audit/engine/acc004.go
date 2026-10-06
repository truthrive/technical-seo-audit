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

const ruleIDACC004 = "AR-ACC-004"

// evaluateACC004 implements the typed execution logic for:
// AR-ACC-004 — Unexpected server-error response (parent check: ACC-007, P1, deterministic).
//
// Contract:
// - PASS: HTTP response obtained and status is not 5xx (e.g. 200, 204, 301, 404, 429).
// - FAIL: HTTP response obtained and status is 5xx (500 <= status <= 599).
// - UNKNOWN: No usable HTTP response obtained (missing status, status 0, malformed, or conflicting),
//            or missing/conflicting URL identity evidence.
//
// Required inputs:
// - url          <- url_identity
// - fetch_status <- http_status
// A PASS or FAIL is produced only when both required inputs are usable.
func evaluateACC004(
	ctx context.Context,
	rule audit.RuleDefinition,
	idx *EvidenceIndex,
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

		// 1. Resolve URL identity input
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

		// 2. Resolve fetch_status input
		statusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "http_status")
		var (
			statusUsable    bool
			statusConflict  bool
			statusCode      int
			statusVal       string
			statusMalformed bool
		)

		if len(statusObs) > 0 {
			firstStatus := strings.TrimSpace(statusObs[0].Value)
			statusVal = firstStatus
			statusUsable = true
			for _, so := range statusObs[1:] {
				if strings.TrimSpace(so.Value) != firstStatus {
					statusConflict = true
					statusUsable = false
					break
				}
			}
			if !statusConflict {
				code, err := strconv.Atoi(firstStatus)
				if err != nil {
					statusMalformed = true
					statusUsable = false
				} else if code <= 0 {
					statusUsable = false
				} else {
					statusCode = code
				}
			}
		}

		// 3. Determine evaluation status and observed summary
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		if !urlUsable {
			evalStatus = audit.StatusUnknown
			if urlConflict {
				observedSummary = "No usable URL identity is available: conflicting URL identity observations."
			} else {
				observedSummary = "Required URL identity evidence is unavailable."
			}
		} else if !statusUsable {
			evalStatus = audit.StatusUnknown
			if len(statusObs) == 0 {
				observedSummary = "No usable HTTP response status is available."
			} else if statusConflict {
				observedSummary = "No usable HTTP response status is available: conflicting status observations."
			} else if statusMalformed {
				observedSummary = fmt.Sprintf("No usable HTTP response status is available: malformed status %q.", statusVal)
			} else {
				// code <= 0
				observedSummary = "No usable HTTP response status is available."
			}
		} else {
			// Both required inputs are usable
			if statusCode >= 500 && statusCode <= 599 {
				evalStatus = audit.StatusFail
				observedSummary = fmt.Sprintf("HTTP status %d is a server-error response.", statusCode)
			} else {
				evalStatus = audit.StatusPass
				observedSummary = fmt.Sprintf("HTTP status %d is not a server-error response.", statusCode)
			}
		}

		// 4. Build traceable evidence references
		var evidenceRefs []audit.RuleEvidenceRef

		// URL Context Evidence: include all actual URL observations
		for _, u := range urlObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url", ruleResultID, u.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(u.ObservationID),
				Field:             "url",
				ObservedValue:     u.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// Status Primary Evidence: include all actual status observations
		for _, st := range statusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:fetch_status", ruleResultID, st.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(st.ObservationID),
				Field:             "fetch_status",
				ObservedValue:     st.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		// Sort evidence refs deterministically by RuleEvidenceRefID
		sort.Slice(evidenceRefs, func(i, j int) bool {
			return evidenceRefs[i].RuleEvidenceRefID < evidenceRefs[j].RuleEvidenceRefID
		})

		// Guard: PASS or FAIL requires both usable inputs
		if evalStatus == audit.StatusPass || evalStatus == audit.StatusFail {
			if !urlUsable || !statusUsable {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s produced %s without both usable inputs", ruleResultID, evalStatus)
			}
			hasURLContext := false
			hasStatusPrimary := false
			for _, ref := range evidenceRefs {
				if ref.Field == "url" && ref.Role == audit.EvidenceRoleContext {
					hasURLContext = true
				}
				if ref.Field == "fetch_status" && ref.Role == audit.EvidenceRolePrimary {
					hasStatusPrimary = true
				}
			}
			if !hasURLContext || !hasStatusPrimary {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s missing required evidence refs (url context: %v, fetch_status primary: %v)", ruleResultID, hasURLContext, hasStatusPrimary)
			}
		}

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
			ExpectedSummary: "HTTP response status is not 5xx.",
			EvaluatedAt:     evalTime,
			EvidenceRefs:    evidenceRefs,
		}

		// Validate result before appending
		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
		}

		results = append(results, rr)
	}

	return results, nil
}
