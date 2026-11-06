package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDLINK006 = "AR-LINK-006"
const expectedSummaryLINK006 = "Priority page crawl depth and discovery path surfaced for human manual review."

// evaluateLINK006 implements AR-LINK-006: Priority-page crawl depth review.
// Evaluation produces StatusManualReview when preconditions are met (no automated PASS/FAIL threshold).
func evaluateLINK006(
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
				ExpectedSummary: expectedSummaryLINK006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    buildURLContextRefs(),
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Resolve priority_page policy
		var (
			policyAssignment *audit.ProjectPolicyAssignment
			hasPolicy        bool
		)
		if policies != nil {
			policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeURL, subjectRef, audit.PolicyKeyPriorityPage)
			if !hasPolicy && urlVal != "" {
				policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeURL, urlVal, audit.PolicyKeyPriorityPage)
			}
		}

		buildPolicyRef := func(role audit.EvidenceRole) *audit.RuleEvidenceRef {
			if !hasPolicy || policyAssignment == nil {
				return nil
			}
			return &audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:priority_page", ruleResultID, policyAssignment.PolicyAssignmentID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "PROJECT_POLICY_ASSIGNMENT",
				EvidenceRef:       string(policyAssignment.PolicyAssignmentID),
				Field:             "priority_page",
				ObservedValue:     policyAssignment.PolicyValue,
				Role:              role,
			}
		}

		// Precondition: priority_page=true explicitly supplied
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
				ObservedSummary: "Page is not explicitly designated as a priority page (priority_page=true not assigned).",
				ExpectedSummary: expectedSummaryLINK006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. Resolve crawl_depth observation
		depthObs := idx.GetObservations(audit.SubjectURL, subjectRef, "crawl_depth")
		var (
			depthMissing  bool
			depthConflict bool
			crawlDepth    int
		)
		if len(depthObs) == 0 {
			depthMissing = true
		} else {
			firstVal := strings.TrimSpace(depthObs[0].Value)
			for _, o := range depthObs[1:] {
				if strings.TrimSpace(o.Value) != firstVal {
					depthConflict = true
					break
				}
			}
			if !depthConflict {
				n, err := strconv.Atoi(firstVal)
				if err != nil || n < 0 {
					depthMissing = true
				} else {
					crawlDepth = n
				}
			}
		}

		evidenceRefs := buildURLContextRefs()
		if pref := buildPolicyRef(audit.EvidenceRolePrimary); pref != nil {
			evidenceRefs = append(evidenceRefs, *pref)
		}
		for _, o := range depthObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:crawl_depth", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             o.Field,
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		if depthMissing || depthConflict {
			obsSummary := "Crawl depth observation is unavailable."
			if depthConflict {
				obsSummary = "Conflicting crawl depth observations on SubjectURL."
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
				ExpectedSummary: expectedSummaryLINK006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Return StatusManualReview
		// Guardrail: no universal "depth > 3 = FAIL" threshold. Decision rests with human reviewer.
		obsSummary := fmt.Sprintf("Priority page observed at crawl depth %d; requires manual reviewer review.", crawlDepth)

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     audit.SubjectURL,
			SubjectRef:      subjectRef,
			Status:          audit.StatusManualReview,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeURL,
			ObservedSummary: obsSummary,
			ExpectedSummary: expectedSummaryLINK006,
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
