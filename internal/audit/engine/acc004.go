package engine

import (
	"context"
	"fmt"
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
// - UNKNOWN: No usable HTTP response obtained (missing status, status 0, malformed, or conflicting).
//
// Field projection:
// - url          <- url_identity
// - fetch_status <- http_status
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

		// 1. Project inputs from normalized observations
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		statusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "http_status")

		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		if len(statusObs) == 0 {
			// No HTTP status observation recorded
			evalStatus = audit.StatusUnknown
			observedSummary = "No usable HTTP response status is available."
		} else {
			// Check for conflicting status observations
			hasConflict := false
			firstVal := statusObs[0].Value
			for _, so := range statusObs[1:] {
				if so.Value != firstVal {
					hasConflict = true
					break
				}
			}

			if hasConflict {
				evalStatus = audit.StatusUnknown
				observedSummary = "No usable HTTP response status is available: conflicting status observations."
			} else {
				valTrimmed := strings.TrimSpace(firstVal)
				code, err := strconv.Atoi(valTrimmed)
				if err != nil {
					// Malformed integer
					evalStatus = audit.StatusUnknown
					observedSummary = fmt.Sprintf("No usable HTTP response status is available: malformed status %q.", valTrimmed)
				} else if code <= 0 {
					// Zero or negative status indicates no response or network/acquisition failure
					evalStatus = audit.StatusUnknown
					observedSummary = "No usable HTTP response status is available."
				} else {
					if code >= 500 && code <= 599 {
						evalStatus = audit.StatusFail
						observedSummary = fmt.Sprintf("HTTP status %d is a server-error response.", code)
					} else {
						evalStatus = audit.StatusPass
						observedSummary = fmt.Sprintf("HTTP status %d is not a server-error response.", code)
					}
				}
			}
		}

		// 2. Build traceable evidence references
		var evidenceRefs []audit.RuleEvidenceRef

		// URL Context Evidence
		if len(urlObs) > 0 {
			u := urlObs[0]
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

		// Status Primary Evidence
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
