package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDAI001 = "AR-AI-001"

// evaluateAI001 implements the typed execution logic for:
// AR-AI-001 — OAI-SearchBot robots access matches project policy (parent check: AI-001, P2, assisted).
//
// Evaluates observed audit.SubjectURL entities against normalized OAI-SearchBot robots.txt
// evaluation evidence and optional oai_searchbot_access_policy.
// Never claims ChatGPT ranking or citation impact.
func evaluateAI001(
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
	expectedSummary := "OAI-SearchBot robots.txt access matches explicit policy or does not block important URLs without explicit policy."

	for _, subjectRef := range subjectRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		urlIdentityObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		robotsOAIObs := idx.GetObservations(audit.SubjectURL, subjectRef, "robots_allowed_oai_searchbot")
		matchedRuleOAIObs := idx.GetObservations(audit.SubjectURL, subjectRef, "matched_robots_rule_oai_searchbot")
		matchedRuleDefaultObs := idx.GetObservations(audit.SubjectURL, subjectRef, "matched_robots_rule")
		robotsTxtURLObs := idx.GetObservations(audit.SubjectURL, subjectRef, "robots_txt_url")

		buildContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			if len(matchedRuleOAIObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchedRuleOAIObs, "matched_robots_rule", audit.EvidenceRoleSupporting)...)
			} else if len(matchedRuleDefaultObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchedRuleDefaultObs, "matched_robots_rule", audit.EvidenceRoleSupporting)...)
			}
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsTxtURLObs, "robots_txt_url", audit.EvidenceRoleContext)...)
			return refs
		}

		// 1. Validate URL identity
		if len(urlIdentityObs) == 0 {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsOAIObs, "robots_allowed_oai_searchbot", audit.EvidenceRoleContext)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Required URL identity observation is missing.", expectedSummary, evalTime, refs))
			continue
		}
		urlVal, urlUsable, urlConflict := extractSingleObsValue(urlIdentityObs)
		if urlConflict {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsOAIObs, "robots_allowed_oai_searchbot", audit.EvidenceRoleContext)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting URL identity observations on audited URL.", expectedSummary, evalTime, refs))
			continue
		}
		if !urlUsable {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsOAIObs, "robots_allowed_oai_searchbot", audit.EvidenceRoleContext)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "URL identity observation is empty or malformed.", expectedSummary, evalTime, refs))
			continue
		}

		// 2. Resolve oai_searchbot_access_policy (URL-scoped policy overrides site-scoped policy)
		policyVal := resolveAccessPolicyValue(policies, subjectRef, urlVal, audit.PolicyKeyOAISearchbotAccessPolicy)
		if policyVal == "invalid" {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsOAIObs, "robots_allowed_oai_searchbot", audit.EvidenceRoleContext)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "OAI-SearchBot access policy value is outside supported enum {allow, block, unspecified}.", expectedSummary, evalTime, refs))
			continue
		}

		// 3. Validate robots_allowed_oai_searchbot
		if len(robotsOAIObs) == 0 {
			refs := buildContextRefs()
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "OAI-SearchBot robots.txt evaluation evidence (robots_allowed_oai_searchbot) is unavailable.", expectedSummary, evalTime, refs))
			continue
		}

		robotsVal, robotsUsable, robotsConflict := extractSingleObsValue(robotsOAIObs)
		if robotsConflict {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsOAIObs, "robots_allowed_oai_searchbot", audit.EvidenceRolePrimary)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting robots_allowed_oai_searchbot observations on audited URL.", expectedSummary, evalTime, refs))
			continue
		}
		if !robotsUsable {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsOAIObs, "robots_allowed_oai_searchbot", audit.EvidenceRolePrimary)...)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "robots_allowed_oai_searchbot observation is empty or malformed.", expectedSummary, evalTime, refs))
			continue
		}

		robotsValLower := strings.ToLower(robotsVal)
		refs := buildContextRefs()
		refs = append(refs, buildEvidenceRefs(ruleResultID, robotsOAIObs, "robots_allowed_oai_searchbot", audit.EvidenceRolePrimary)...)

		if robotsValLower == "unknown" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "OAI-SearchBot robots.txt evaluation is unknown (robots.txt unavailable or unparseable).", expectedSummary, evalTime, refs))
			continue
		}
		if robotsValLower != "true" && robotsValLower != "false" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("robots_allowed_oai_searchbot value %q is malformed; expected 'true', 'false', or 'unknown'.", robotsVal), expectedSummary, evalTime, refs))
			continue
		}

		allowed := robotsValLower == "true"

		switch policyVal {
		case audit.PolicyValueAccessAllow:
			if allowed {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, "OAI-SearchBot is allowed by robots.txt, matching explicit allow policy.", expectedSummary, evalTime, refs))
			} else {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusFail, "Explicit policy requires OAI-SearchBot access, but URL is blocked by robots.txt for OAI-SearchBot.", expectedSummary, evalTime, refs))
			}
		case audit.PolicyValueAccessBlock:
			if !allowed {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, "OAI-SearchBot is disallowed by robots.txt, matching explicit block policy.", expectedSummary, evalTime, refs))
			} else {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusFail, "Explicit policy requires blocking OAI-SearchBot, but URL is allowed by robots.txt for OAI-SearchBot.", expectedSummary, evalTime, refs))
			}
		default: // unspecified
			if allowed {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, "OAI-SearchBot is allowed by robots.txt (no explicit OAI-SearchBot access policy supplied).", expectedSummary, evalTime, refs))
			} else {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, "OAI-SearchBot is disallowed by robots.txt on audited URL while OAI-SearchBot access policy is unspecified.", expectedSummary, evalTime, refs))
			}
		}
	}

	return results, nil
}
