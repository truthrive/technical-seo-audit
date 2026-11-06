package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDACC003 = "AR-ACC-003"
const expectedSummaryACC003 = "HTTP response status matches explicit expected URL state (live -> 2xx, redirect -> 3xx, missing -> 404/410)."

// evaluateACC003 implements AR-ACC-003: HTTP status matches explicit expected URL state.
func evaluateACC003(
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

		// 1. Resolve source URL identity
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		var (
			urlUsable   bool
			urlConflict bool
			urlVal      string
		)
		if len(urlObs) > 0 {
			urlVal = strings.TrimSpace(urlObs[0].Value)
			if urlVal != "" {
				urlUsable = true
				for _, u := range urlObs[1:] {
					if strings.TrimSpace(u.Value) != urlVal {
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

		if !urlUsable {
			obsSummary := "Required URL identity evidence is unavailable."
			if urlConflict {
				obsSummary = "No usable URL identity is available: conflicting URL identity observations."
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
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryACC003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    buildURLContextRefs(),
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Resolve expected_url_state policy
		var (
			policyAssignment *audit.ProjectPolicyAssignment
			hasPolicy        bool
		)
		if policies != nil {
			policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeURL, subjectRef, audit.PolicyKeyExpectedURLState)
			if !hasPolicy && urlVal != "" {
				policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeURL, urlVal, audit.PolicyKeyExpectedURLState)
			}
		}

		buildPolicyRef := func(role audit.EvidenceRole) *audit.RuleEvidenceRef {
			if !hasPolicy || policyAssignment == nil {
				return nil
			}
			return &audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:expected_url_state", ruleResultID, policyAssignment.PolicyAssignmentID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "PROJECT_POLICY_ASSIGNMENT",
				EvidenceRef:       string(policyAssignment.PolicyAssignmentID),
				Field:             "expected_url_state",
				ObservedValue:     policyAssignment.PolicyValue,
				Role:              role,
			}
		}

		if !hasPolicy || policyAssignment == nil || policyAssignment.PolicyValue == "" || policyAssignment.PolicyValue == audit.PolicyValueURLStateUnspecified {
			evidenceRefs := buildURLContextRefs()
			if pref := buildPolicyRef(audit.EvidenceRoleContext); pref != nil {
				evidenceRefs = append(evidenceRefs, *pref)
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
				Status:          audit.StatusNotApplicable,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: "Expected URL state was not explicitly supplied.",
				ExpectedSummary: expectedSummaryACC003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		expectedState := strings.ToLower(strings.TrimSpace(policyAssignment.PolicyValue))
		if expectedState != audit.PolicyValueURLStateLive && expectedState != audit.PolicyValueURLStateRedirect && expectedState != audit.PolicyValueURLStateMissing {
			evidenceRefs := buildURLContextRefs()
			if pref := buildPolicyRef(audit.EvidenceRolePrimary); pref != nil {
				evidenceRefs = append(evidenceRefs, *pref)
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
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: fmt.Sprintf("Invalid expected_url_state policy value %q; expected 'live', 'redirect', or 'missing'.", expectedState),
				ExpectedSummary: expectedSummaryACC003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. Resolve initial response status
		statusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "http_status")
		var (
			statusMissing   bool
			statusConflict  bool
			statusMalformed bool
			statusCode      int
		)
		if len(statusObs) == 0 {
			statusMissing = true
		} else {
			firstVal := strings.TrimSpace(statusObs[0].Value)
			for _, s := range statusObs[1:] {
				if strings.TrimSpace(s.Value) != firstVal {
					statusConflict = true
					break
				}
			}
			if !statusConflict {
				n, err := strconv.Atoi(firstVal)
				if err != nil || n <= 0 {
					statusMalformed = true
				} else {
					statusCode = n
				}
			}
		}

		if statusMissing || statusConflict || statusMalformed {
			evidenceRefs := buildURLContextRefs()
			if pref := buildPolicyRef(audit.EvidenceRoleContext); pref != nil {
				evidenceRefs = append(evidenceRefs, *pref)
			}
			for _, s := range statusObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:http_status", ruleResultID, s.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(s.ObservationID),
					Field:             "http_status",
					ObservedValue:     s.Value,
					Role:              audit.EvidenceRolePrimary,
				})
			}
			obsSummary := "HTTP response status evidence is unavailable."
			if statusConflict {
				obsSummary = "Conflicting HTTP response status observations on SubjectURL."
			} else if statusMalformed {
				obsSummary = "Malformed HTTP response status observation on SubjectURL."
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
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryACC003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Evaluate against expected state
		evidenceRefs := buildURLContextRefs()
		if pref := buildPolicyRef(audit.EvidenceRolePrimary); pref != nil {
			evidenceRefs = append(evidenceRefs, *pref)
		}
		for _, s := range statusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:http_status", ruleResultID, s.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(s.ObservationID),
				Field:             "http_status",
				ObservedValue:     s.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		var (
			evalStatus audit.RuleResultStatus
			obsSummary string
		)

		switch expectedState {
		case audit.PolicyValueURLStateLive:
			if statusCode >= 200 && statusCode < 300 {
				evalStatus = audit.StatusPass
				obsSummary = fmt.Sprintf("HTTP status %d matches expected 'live' state (2xx).", statusCode)
			} else {
				evalStatus = audit.StatusFail
				obsSummary = fmt.Sprintf("HTTP status %d does not match expected 'live' state (expected 2xx).", statusCode)
			}

		case audit.PolicyValueURLStateRedirect:
			if statusCode >= 300 && statusCode < 400 {
				evalStatus = audit.StatusPass
				obsSummary = fmt.Sprintf("HTTP status %d matches expected 'redirect' state (3xx).", statusCode)
			} else {
				evalStatus = audit.StatusFail
				obsSummary = fmt.Sprintf("HTTP status %d does not match expected 'redirect' state (expected 3xx).", statusCode)
			}

		case audit.PolicyValueURLStateMissing:
			// If redirected, check destination status
			if statusCode >= 300 && statusCode < 400 {
				targetRefObs := idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_final_target_subject_ref")
				if len(targetRefObs) > 0 {
					targetSubj := strings.TrimSpace(targetRefObs[0].Value)
					targetStatusObs := idx.GetObservations(audit.SubjectURL, targetSubj, "http_status")
					if len(targetStatusObs) > 0 {
						for _, tso := range targetStatusObs {
							evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
								RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:target_http_status", ruleResultID, tso.ObservationID)),
								RuleResultID:      ruleResultID,
								EvidenceType:      "NORMALIZED_OBSERVATION",
								EvidenceRef:       string(tso.ObservationID),
								Field:             "http_status",
								ObservedValue:     tso.Value,
								Role:              audit.EvidenceRolePrimary,
							})
						}
						targetStatus, _ := strconv.Atoi(strings.TrimSpace(targetStatusObs[0].Value))
						if targetStatus == 404 || targetStatus == 410 {
							evalStatus = audit.StatusPass
							obsSummary = fmt.Sprintf("Redirected to final status %d, matching expected 'missing' state (404/410).", targetStatus)
						} else {
							evalStatus = audit.StatusFail
							obsSummary = fmt.Sprintf("Redirected to final status %d, does not match expected 'missing' state (expected 404/410).", targetStatus)
						}
					} else {
						evalStatus = audit.StatusUnknown
						obsSummary = "Redirect final destination status cannot be obtained."
					}
				} else {
					evalStatus = audit.StatusUnknown
					obsSummary = "Redirect destination is outside crawl boundary or final status cannot be obtained."
				}
			} else if statusCode == 404 || statusCode == 410 {
				evalStatus = audit.StatusPass
				obsSummary = fmt.Sprintf("HTTP status %d matches expected 'missing' state (404/410).", statusCode)
			} else {
				evalStatus = audit.StatusFail
				obsSummary = fmt.Sprintf("HTTP status %d does not match expected 'missing' state (expected 404/410).", statusCode)
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
			ObservedSummary: obsSummary,
			ExpectedSummary: expectedSummaryACC003,
			EvaluatedAt:     evalTime,
			EvidenceRefs:    evidenceRefs,
		}
		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
		}
		results = append(results, rr)
	}

	return results, nil
}
