package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDAI003 = "AR-AI-003"

// evaluateAI003 implements the typed execution logic for:
// AR-AI-003 — GPTBot configuration matches explicit training policy (parent check: AI-002, P2, assisted).
//
// Evaluates observed audit.SubjectURL entities (or audit.SubjectSite when URL subjects are absent)
// against normalized GPTBot and OAI-SearchBot robots.txt evaluation evidence and optional
// gptbot_training_policy.
//
// Contract (per frozen manifest rules_v1.json):
// - PASS:
//   - Policy allow & robots_allowed_gptbot == "true"
//   - Policy block & robots_allowed_gptbot == "false"
// - WARNING:
//   - Policy allow & robots_allowed_gptbot == "false"
//   - Policy block & robots_allowed_gptbot == "true"
// - MANUAL_REVIEW:
//   - Policy unspecified (or not supplied) while robots_allowed_gptbot is known ("true" or "false")
// - UNKNOWN:
//   - Missing/conflicting/malformed url_identity, robots_allowed_gptbot, robots_allowed_oai_searchbot, or policy input.
//   - robots_allowed_gptbot == "unknown".
// - FAIL and NOT_APPLICABLE are not used.
func evaluateAI003(
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

	subjectType := audit.SubjectURL
	scope := audit.ScopeURL
	subjectRefs := idx.SubjectRefs(audit.SubjectURL)
	if len(subjectRefs) == 0 {
		siteRefs := idx.SubjectRefs(audit.SubjectSite)
		if len(siteRefs) > 0 {
			subjectType = audit.SubjectSite
			scope = audit.ScopeSitewide
			subjectRefs = siteRefs
		}
	}

	results := make([]audit.RuleResult, 0, len(subjectRefs))
	expectedSummary := "GPTBot robots.txt configuration matches explicit AI training policy."

	makeRes := func(subjectRef string, ruleResultID audit.RuleResultID, status audit.RuleResultStatus, summary string, refs []audit.RuleEvidenceRef) audit.RuleResult {
		return audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     subjectType,
			SubjectRef:      subjectRef,
			Status:          status,
			Severity:        rule.DefaultSeverity,
			Scope:           scope,
			ObservedSummary: summary,
			ExpectedSummary: expectedSummary,
			EvaluatedAt:     evalTime,
			EvidenceRefs:    dedupeAndSortEvidenceRefs(refs),
		}
	}

	for _, subjectRef := range subjectRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		urlIdentityObs := idx.GetObservations(subjectType, subjectRef, "url_identity")
		robotsGPTObs := idx.GetObservations(subjectType, subjectRef, "robots_allowed_gptbot")
		robotsOAIObs := idx.GetObservations(subjectType, subjectRef, "robots_allowed_oai_searchbot")
		matchedRuleGPTObs := idx.GetObservations(subjectType, subjectRef, "matched_robots_rule_gptbot")
		matchedRuleOAIObs := idx.GetObservations(subjectType, subjectRef, "matched_robots_rule_oai_searchbot")
		matchedRuleDefaultObs := idx.GetObservations(subjectType, subjectRef, "matched_robots_rule")
		robotsTxtURLObs := idx.GetObservations(subjectType, subjectRef, "robots_txt_url")

		buildContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			if len(urlIdentityObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			}
			if len(robotsOAIObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, robotsOAIObs, "robots_allowed_oai_searchbot", audit.EvidenceRoleSupporting)...)
			}
			if len(matchedRuleGPTObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchedRuleGPTObs, "matched_robots_rule", audit.EvidenceRoleSupporting)...)
			} else if len(matchedRuleDefaultObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchedRuleDefaultObs, "matched_robots_rule", audit.EvidenceRoleSupporting)...)
			}
			if len(matchedRuleOAIObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, matchedRuleOAIObs, "matched_robots_rule_oai_searchbot", audit.EvidenceRoleContext)...)
			}
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsTxtURLObs, "robots_txt_url", audit.EvidenceRoleContext)...)
			return refs
		}

		// 1. Validate URL identity when evaluating SubjectURL
		var urlVal string
		if subjectType == audit.SubjectURL {
			if len(urlIdentityObs) == 0 {
				refs := buildContextRefs()
				refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRoleContext)...)
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Required URL identity observation is missing.", refs))
				continue
			}
			var urlUsable, urlConflict bool
			urlVal, urlUsable, urlConflict = extractSingleObsValue(urlIdentityObs)
			if urlConflict {
				refs := buildContextRefs()
				refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRoleContext)...)
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting URL identity observations on audited URL.", refs))
				continue
			}
			if !urlUsable {
				refs := buildContextRefs()
				refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRoleContext)...)
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "URL identity observation is empty or malformed.", refs))
				continue
			}
		}

		// 2. Validate robots_allowed_oai_searchbot if present (must not be conflicting/malformed)
		if len(robotsOAIObs) > 0 {
			oaiVal, oaiUsable, oaiConflict := extractSingleObsValue(robotsOAIObs)
			if oaiConflict || !oaiUsable {
				refs := buildContextRefs()
				refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRoleContext)...)
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed robots_allowed_oai_searchbot evidence.", refs))
				continue
			}
			oaiLower := strings.ToLower(oaiVal)
			if oaiLower != "true" && oaiLower != "false" && oaiLower != "unknown" {
				refs := buildContextRefs()
				refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRoleContext)...)
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("robots_allowed_oai_searchbot value %q is malformed.", oaiVal), refs))
				continue
			}
		}

		// 3. Resolve gptbot_training_policy
		policyVal := resolveAccessPolicyValue(policies, subjectRef, urlVal, audit.PolicyKeyGPTBotTrainingPolicy)
		if policyVal == "invalid" {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRoleContext)...)
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "GPTBot training policy value is outside supported enum {allow, block, unspecified}.", refs))
			continue
		}

		// 4. Validate robots_allowed_gptbot
		if len(robotsGPTObs) == 0 {
			refs := buildContextRefs()
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "GPTBot robots.txt evaluation evidence (robots_allowed_gptbot) is unavailable.", refs))
			continue
		}

		robotsVal, robotsUsable, robotsConflict := extractSingleObsValue(robotsGPTObs)
		if robotsConflict {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRolePrimary)...)
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting robots_allowed_gptbot observations.", refs))
			continue
		}
		if !robotsUsable {
			refs := buildContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRolePrimary)...)
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "robots_allowed_gptbot observation is empty or malformed.", refs))
			continue
		}

		robotsValLower := strings.ToLower(robotsVal)
		refs := buildContextRefs()
		refs = append(refs, buildEvidenceRefs(ruleResultID, robotsGPTObs, "robots_allowed_gptbot", audit.EvidenceRolePrimary)...)

		if robotsValLower == "unknown" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "GPTBot robots.txt evaluation is unknown (robots.txt unavailable or unparseable).", refs))
			continue
		}
		if robotsValLower != "true" && robotsValLower != "false" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("robots_allowed_gptbot value %q is malformed; expected 'true', 'false', or 'unknown'.", robotsVal), refs))
			continue
		}

		allowed := robotsValLower == "true"

		switch policyVal {
		case audit.PolicyValueAccessAllow:
			if allowed {
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusPass, "GPTBot is allowed by robots.txt, matching explicit allow training policy.", refs))
			} else {
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusWarning, "Explicit training policy is allow, but GPTBot is disallowed by robots.txt.", refs))
			}
		case audit.PolicyValueAccessBlock:
			if !allowed {
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusPass, "GPTBot is disallowed by robots.txt, matching explicit block training policy.", refs))
			} else {
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusWarning, "Explicit training policy is block, but GPTBot is allowed by robots.txt.", refs))
			}
		default: // unspecified
			stateDesc := "allowed"
			if !allowed {
				stateDesc = "disallowed"
			}
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusManualReview, fmt.Sprintf("GPTBot is %s by robots.txt, but explicit gptbot_training_policy is not supplied; manual review required to confirm alignment with intended AI training policy.", stateDesc), refs))
		}
	}

	return results, nil
}
