package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDACC002 = "AR-ACC-002"
const expectedSummaryACC002 = "URL robots access policy matches explicit crawl intent (expected_crawlable=true)."

// evaluateACC002 implements AR-ACC-002: Robots access matches explicit crawl intent.
func evaluateACC002(
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
				ExpectedSummary: expectedSummaryACC002,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    buildURLContextRefs(),
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Resolve expected_crawlable policy
		var (
			policyAssignment *audit.ProjectPolicyAssignment
			hasPolicy        bool
		)
		if policies != nil {
			policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeURL, subjectRef, audit.PolicyKeyExpectedCrawlable)
			if !hasPolicy && urlVal != "" {
				policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeURL, urlVal, audit.PolicyKeyExpectedCrawlable)
			}
		}

		buildPolicyRef := func(role audit.EvidenceRole) *audit.RuleEvidenceRef {
			if !hasPolicy || policyAssignment == nil {
				return nil
			}
			return &audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:expected_crawlable", ruleResultID, policyAssignment.PolicyAssignmentID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "PROJECT_POLICY_ASSIGNMENT",
				EvidenceRef:       string(policyAssignment.PolicyAssignmentID),
				Field:             "expected_crawlable",
				ObservedValue:     policyAssignment.PolicyValue,
				Role:              role,
			}
		}

		// Precondition: explicit expected_crawlable=true required
		if !hasPolicy || policyAssignment == nil || strings.ToLower(strings.TrimSpace(policyAssignment.PolicyValue)) != "true" {
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
				ObservedSummary: "Explicit crawl intent was not supplied (expected_crawlable=true not assigned).",
				ExpectedSummary: expectedSummaryACC002,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. Resolve robots_allowed_default observation
		allowedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "robots_allowed_default")
		var (
			allowedMissing  bool
			allowedConflict bool
			allowedVal      string
		)
		if len(allowedObs) == 0 {
			allowedMissing = true
		} else {
			firstVal := strings.TrimSpace(allowedObs[0].Value)
			allowedVal = firstVal
			for _, o := range allowedObs[1:] {
				if strings.TrimSpace(o.Value) != firstVal {
					allowedConflict = true
					break
				}
			}
		}

		evidenceRefs := buildURLContextRefs()
		if pref := buildPolicyRef(audit.EvidenceRolePrimary); pref != nil {
			evidenceRefs = append(evidenceRefs, *pref)
		}
		for _, o := range allowedObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:robots_allowed_default", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "robots_allowed_default",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		if allowedMissing || allowedConflict || allowedVal == "unknown" || (allowedVal != "true" && allowedVal != "false") {
			obsSummary := "Effective robots decision cannot be obtained."
			if allowedConflict {
				obsSummary = "Conflicting robots access observations on SubjectURL."
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
				ExpectedSummary: expectedSummaryACC002,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Evaluate Pass/Fail
		var (
			evalStatus audit.RuleResultStatus
			obsSummary string
		)
		if allowedVal == "true" {
			evalStatus = audit.StatusPass
			obsSummary = "URL is allowed for crawler access under effective robots policy, matching explicit crawl intent."
		} else {
			evalStatus = audit.StatusFail
			obsSummary = "URL is disallowed by effective robots policy despite explicit crawl intent (expected_crawlable=true)."
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
			ExpectedSummary: expectedSummaryACC002,
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
