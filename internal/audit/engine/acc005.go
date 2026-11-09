package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDACC005 = "AR-ACC-005"

// evaluateACC005 implements the typed execution logic for:
// AR-ACC-005 — Googlebot robots access matches explicit policy (parent check: ACC-002, P1, assisted).
//
// Evaluates observed audit.SubjectURL entities against normalized Googlebot robots.txt
// evaluation evidence and optional googlebot_access_policy.
//
// Contract:
// - PASS:
//   - Policy allow & robots_allowed_googlebot == "true"
//   - Policy block & robots_allowed_googlebot == "false"
//   - Policy unspecified & robots_allowed_googlebot == "true"
// - FAIL:
//   - Policy allow & robots_allowed_googlebot == "false"
//   - Policy block & robots_allowed_googlebot == "true"
// - WARNING:
//   - Policy unspecified & robots_allowed_googlebot == "false"
// - UNKNOWN:
//   - Missing/conflicting/malformed url_identity, robots_allowed_googlebot, or policy input.
//   - robots_allowed_googlebot == "unknown".
func evaluateACC005(
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

	subjectRefs := idx.SubjectRefs(audit.SubjectURL)
	results := make([]audit.RuleResult, 0, len(subjectRefs))
	expectedSummary := "Googlebot robots.txt outcome matches explicit policy or does not block important URLs without explicit policy."

	for _, subjectRef := range subjectRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		urlIdentityObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		robotsGooglebotObs := idx.GetObservations(audit.SubjectURL, subjectRef, "robots_allowed_googlebot")
		robotsDefaultObs := idx.GetObservations(audit.SubjectURL, subjectRef, "robots_allowed_default")
		matchedRuleGooglebotObs := idx.GetObservations(audit.SubjectURL, subjectRef, "matched_robots_rule_googlebot")
		matchedRuleDefaultObs := idx.GetObservations(audit.SubjectURL, subjectRef, "matched_robots_rule")
		robotsTxtURLObs := idx.GetObservations(audit.SubjectURL, subjectRef, "robots_txt_url")

		buildContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			if len(matchedRuleGooglebotObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchedRuleGooglebotObs, "matched_robots_rule", audit.EvidenceRoleSupporting)...)
			} else if len(matchedRuleDefaultObs) > 0 && len(robotsDefaultObs) == 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchedRuleDefaultObs, "matched_robots_rule", audit.EvidenceRoleSupporting)...)
			}
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsTxtURLObs, "robots_txt_url", audit.EvidenceRoleContext)...)
			return refs
		}

		// 1. Validate URL identity
		if len(urlIdentityObs) == 0 {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGooglebotObs, "robots_allowed_googlebot", audit.EvidenceRoleContext)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Required URL identity observation is missing.", expectedSummary, evalTime, refs))
			continue
		}
		urlVal, urlUsable, urlConflict := extractSingleObsValue(urlIdentityObs)
		if urlConflict {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGooglebotObs, "robots_allowed_googlebot", audit.EvidenceRoleContext)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting URL identity observations on audited URL.", expectedSummary, evalTime, refs))
			continue
		}
		if !urlUsable {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGooglebotObs, "robots_allowed_googlebot", audit.EvidenceRoleContext)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "URL identity observation is empty or malformed.", expectedSummary, evalTime, refs))
			continue
		}

		// 2. Resolve googlebot_access_policy (URL-scoped policy overrides site-scoped policy)
		policyVal := resolveAccessPolicyValue(policies, subjectRef, urlVal, audit.PolicyKeyGooglebotAccessPolicy)
		if policyVal == "invalid" {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGooglebotObs, "robots_allowed_googlebot", audit.EvidenceRoleContext)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Googlebot access policy value is outside supported enum {allow, block, unspecified}.", expectedSummary, evalTime, refs))
			continue
		}

		// 3. Validate robots_allowed_googlebot
		if len(robotsGooglebotObs) == 0 {
			refs := buildContextRefs()
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Googlebot robots.txt evaluation evidence (robots_allowed_googlebot) is unavailable.", expectedSummary, evalTime, refs))
			continue
		}

		robotsVal, robotsUsable, robotsConflict := extractSingleObsValue(robotsGooglebotObs)
		if robotsConflict {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGooglebotObs, "robots_allowed_googlebot", audit.EvidenceRolePrimary)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting robots_allowed_googlebot observations on audited URL.", expectedSummary, evalTime, refs))
			continue
		}
		if !robotsUsable {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGooglebotObs, "robots_allowed_googlebot", audit.EvidenceRolePrimary)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "robots_allowed_googlebot observation is empty or malformed.", expectedSummary, evalTime, refs))
			continue
		}

		robotsValLower := strings.ToLower(robotsVal)
		refs := buildContextRefs()
		refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGooglebotObs, "robots_allowed_googlebot", audit.EvidenceRolePrimary)...)

		if robotsValLower == "unknown" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Googlebot robots.txt evaluation is unknown (robots.txt unavailable or unparseable).", expectedSummary, evalTime, refs))
			continue
		}
		if robotsValLower != "true" && robotsValLower != "false" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("robots_allowed_googlebot value %q is malformed; expected 'true', 'false', or 'unknown'.", robotsVal), expectedSummary, evalTime, refs))
			continue
		}

		allowed := robotsValLower == "true"

		switch policyVal {
		case audit.PolicyValueAccessAllow:
			if allowed {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, "Googlebot is allowed by robots.txt, matching explicit allow policy.", expectedSummary, evalTime, refs))
			} else {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusFail, "Explicit policy requires Googlebot access, but URL is blocked by robots.txt for Googlebot.", expectedSummary, evalTime, refs))
			}
		case audit.PolicyValueAccessBlock:
			if !allowed {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, "Googlebot is disallowed by robots.txt, matching explicit block policy.", expectedSummary, evalTime, refs))
			} else {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusFail, "Explicit policy requires blocking Googlebot, but URL is allowed by robots.txt for Googlebot.", expectedSummary, evalTime, refs))
			}
		default: // unspecified
			if allowed {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, "Googlebot is allowed by robots.txt (no explicit Googlebot access policy supplied).", expectedSummary, evalTime, refs))
			} else {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, "Googlebot is disallowed by robots.txt on audited URL while Googlebot access policy is unspecified.", expectedSummary, evalTime, refs))
			}
		}
	}

	return results, nil
}

// resolveAccessPolicyValue resolves a site-level or URL-level crawler access/training policy
// ("allow", "block", "unspecified", or "invalid").
func resolveAccessPolicyValue(policies *PolicyIndex, subjectRef, urlVal string, key audit.PolicyKey) string {
	if policies == nil {
		return audit.PolicyValueAccessUnspecified
	}
	var assign *audit.ProjectPolicyAssignment
	if p, ok := policies.Get(audit.PolicyScopeURL, subjectRef, key); ok {
		assign = p
	} else if urlVal != "" {
		if p, ok := policies.Get(audit.PolicyScopeURL, urlVal, key); ok {
			assign = p
		}
	}
	if assign == nil {
		if p, ok := policies.Get(audit.PolicyScopeSite, "site", key); ok {
			assign = p
		} else {
			for _, target := range policies.TargetRefs(audit.PolicyScopeSite) {
				if p, ok := policies.Get(audit.PolicyScopeSite, target, key); ok {
					assign = p
					break
				}
			}
		}
	}
	if assign == nil {
		return audit.PolicyValueAccessUnspecified
	}
	val := strings.ToLower(strings.TrimSpace(assign.PolicyValue))
	switch val {
	case audit.PolicyValueAccessAllow, audit.PolicyValueAccessBlock, audit.PolicyValueAccessUnspecified, "":
		if val == "" {
			return audit.PolicyValueAccessUnspecified
		}
		return val
	default:
		return "invalid"
	}
}

// makeURLResult constructs an audit.RuleResult for a SubjectURL subject with deterministic evidence refs.
func makeURLResult(
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
		SubjectType:     audit.SubjectURL,
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
