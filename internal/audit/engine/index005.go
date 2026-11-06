package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDINDEX005 = "AR-INDEX-005"
const expectedSummaryINDEX005 = "Snippet configuration conforms to project policy (unrestricted search snippet display or permitted restriction)."

// evaluateINDEX005 implements AR-INDEX-005: Snippet restriction conflicts with snippet policy.
func evaluateINDEX005(
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

	// Resolve site-level snippet policy
	var (
		policyAssignment *audit.ProjectPolicyAssignment
		hasSitePolicy    bool
	)
	if policies != nil {
		policyAssignment, hasSitePolicy = policies.Get(audit.PolicyScopeSite, "site", audit.PolicyKeySnippetPolicy)
		if !hasSitePolicy {
			for _, target := range policies.TargetRefs(audit.PolicyScopeSite) {
				if pol, ok := policies.Get(audit.PolicyScopeSite, target, audit.PolicyKeySnippetPolicy); ok {
					policyAssignment = pol
					hasSitePolicy = true
					break
				}
			}
		}
	}

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
				ExpectedSummary: expectedSummaryINDEX005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    buildURLContextRefs(),
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Resolve URL-level or site-level policy
		effPolicyAssignment := policyAssignment
		hasPolicy := hasSitePolicy
		if policies != nil {
			if urlPol, ok := policies.Get(audit.PolicyScopeURL, subjectRef, audit.PolicyKeySnippetPolicy); ok {
				effPolicyAssignment = urlPol
				hasPolicy = true
			} else if urlVal != "" {
				if urlPol, ok := policies.Get(audit.PolicyScopeURL, urlVal, audit.PolicyKeySnippetPolicy); ok {
					effPolicyAssignment = urlPol
					hasPolicy = true
				}
			}
		}

		policyValue := audit.PolicyValueSnippetUnspecified
		if hasPolicy && effPolicyAssignment != nil {
			policyValue = strings.TrimSpace(effPolicyAssignment.PolicyValue)
		}

		buildPolicyRef := func(role audit.EvidenceRole) *audit.RuleEvidenceRef {
			if !hasPolicy || effPolicyAssignment == nil {
				return nil
			}
			return &audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:snippet_policy", ruleResultID, effPolicyAssignment.PolicyAssignmentID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "PROJECT_POLICY_ASSIGNMENT",
				EvidenceRef:       string(effPolicyAssignment.PolicyAssignmentID),
				Field:             "snippet_policy",
				ObservedValue:     effPolicyAssignment.PolicyValue,
				Role:              role,
			}
		}

		// 3. Extract snippet directives evidence
		nosnippetObs := idx.GetObservations(audit.SubjectURL, subjectRef, "nosnippet")
		maxSnippetObs := idx.GetObservations(audit.SubjectURL, subjectRef, "max_snippet")
		dataNosnippetObs := idx.GetObservations(audit.SubjectURL, subjectRef, "data_nosnippet_count")
		metaRawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "meta_robots_raw")
		xRobotsRawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "x_robots_raw")

		evidenceRefs := buildURLContextRefs()
		if pref := buildPolicyRef(audit.EvidenceRoleContext); pref != nil {
			evidenceRefs = append(evidenceRefs, *pref)
		}

		for _, o := range nosnippetObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:nosnippet", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "nosnippet",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range maxSnippetObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:max_snippet", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "max_snippet",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range dataNosnippetObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:data_nosnippet_count", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "data_nosnippet_count",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range metaRawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:meta_robots_raw", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "meta_robots_raw",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}
		for _, o := range xRobotsRawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:x_robots_raw", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "x_robots_raw",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// Check if directives are available
		hasDirectiveEvidence := len(nosnippetObs) > 0 || len(maxSnippetObs) > 0 || len(dataNosnippetObs) > 0 ||
			len(metaRawObs) > 0 || len(xRobotsRawObs) > 0

		if !hasDirectiveEvidence {
			// Check if http_header_status indicates unavailable
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
				ObservedSummary: "Snippet directives are unavailable or unextractable for SubjectURL.",
				ExpectedSummary: expectedSummaryINDEX005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// Check for observation conflicts
		nosnippetBool, nosnippetUsable, nosnippetConflict := resolveSingleBoolObservation(nosnippetObs)
		maxSnippetStr, maxSnippetUsable, maxSnippetConflict := resolveSingleStringObservation(maxSnippetObs)
		dataNosnippetStr, dataNosnippetUsable, dataNosnippetConflict := resolveSingleStringObservation(dataNosnippetObs)

		if nosnippetConflict || maxSnippetConflict || dataNosnippetConflict {
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
				ObservedSummary: "Conflicting snippet directive observations observed on SubjectURL.",
				ExpectedSummary: expectedSummaryINDEX005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// Determine directive values (combining direct observations and raw headers/meta if needed)
		hasNosnippet := nosnippetUsable && nosnippetBool
		maxSnippetVal := -1
		hasMaxSnippet := false
		if maxSnippetUsable {
			if v, err := strconv.Atoi(maxSnippetStr); err == nil {
				maxSnippetVal = v
				if maxSnippetVal >= 0 {
					hasMaxSnippet = true
				}
			}
		}

		dataNosnippetCount := 0
		if dataNosnippetUsable {
			if v, err := strconv.Atoi(dataNosnippetStr); err == nil && v > 0 {
				dataNosnippetCount = v
			}
		}

		// Fallback to raw meta/x-robots tokens if direct observations were absent
		if !nosnippetUsable && !maxSnippetUsable {
			var rawTokens []string
			for _, o := range metaRawObs {
				rawTokens = append(rawTokens, splitAndNormalizeTokens(o.Value)...)
			}
			for _, o := range xRobotsRawObs {
				rawTokens = append(rawTokens, splitAndNormalizeTokens(o.Value)...)
			}
			for _, tok := range rawTokens {
				if tok == "nosnippet" {
					hasNosnippet = true
				}
				if strings.HasPrefix(tok, "max-snippet:") {
					numStr := strings.TrimPrefix(tok, "max-snippet:")
					if n, err := strconv.Atoi(numStr); err == nil {
						maxSnippetVal = n
						if maxSnippetVal >= 0 {
							hasMaxSnippet = true
						}
					}
				}
			}
		}

		hasDataNosnippet := (dataNosnippetCount > 0)
		hasRestriction := hasNosnippet || hasMaxSnippet || hasDataNosnippet

		// 4. Evaluate against policy
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		if !hasRestriction {
			// Precondition: snippet directives extracted. PASS when no restriction exists.
			evalStatus = audit.StatusPass
			observedSummary = "No snippet restrictions observed; complies with snippet policy."
		} else {
			// A restriction exists.
			switch policyValue {
			case audit.PolicyValueSnippetAllowUnrestricted:
				evalStatus = audit.StatusFail
				var reasons []string
				if hasNosnippet {
					reasons = append(reasons, "nosnippet directive present")
				}
				if hasMaxSnippet {
					reasons = append(reasons, fmt.Sprintf("max-snippet:%d length restriction present", maxSnippetVal))
				}
				if hasDataNosnippet {
					reasons = append(reasons, fmt.Sprintf("data-nosnippet elements present (%d)", dataNosnippetCount))
				}
				observedSummary = fmt.Sprintf("Snippet restriction observed (%s) while project policy specifies allow_unrestricted.",
					strings.Join(reasons, ", "))

			case audit.PolicyValueSnippetRestrict:
				evalStatus = audit.StatusPass
				observedSummary = "Explicit snippet policy 'restrict' permits observed snippet restriction."

			default: // PolicyValueSnippetUnspecified or no policy
				if hasDataNosnippet && !hasNosnippet && !hasMaxSnippet {
					evalStatus = audit.StatusManualReview
					observedSummary = fmt.Sprintf("Block-level data-nosnippet restriction observed (%d elements); manual review required to evaluate search snippet impact.", dataNosnippetCount)
				} else {
					evalStatus = audit.StatusWarning
					var reasons []string
					if hasNosnippet {
						reasons = append(reasons, "nosnippet")
					}
					if hasMaxSnippet {
						reasons = append(reasons, fmt.Sprintf("max-snippet:%d", maxSnippetVal))
					}
					observedSummary = fmt.Sprintf("Snippet restriction observed (%s) while snippet policy is unspecified.",
						strings.Join(reasons, ", "))
				}
			}
		}

		// Update policy evidence role to primary if it drove FAIL/PASS/WARNING
		if pref := buildPolicyRef(audit.EvidenceRolePrimary); pref != nil && hasPolicy {
			for k, r := range evidenceRefs {
				if r.Field == "snippet_policy" {
					evidenceRefs[k].Role = audit.EvidenceRolePrimary
					break
				}
			}
		}

		// Ensure non-nil slices for serializability
		if evidenceRefs == nil {
			evidenceRefs = []audit.RuleEvidenceRef{}
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
			ExpectedSummary: expectedSummaryINDEX005,
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
