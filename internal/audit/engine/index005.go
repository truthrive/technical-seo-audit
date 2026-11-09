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
		policyInvalid := false
		if hasPolicy && effPolicyAssignment != nil {
			policyValue = strings.TrimSpace(effPolicyAssignment.PolicyValue)
			switch policyValue {
			case audit.PolicyValueSnippetAllowUnrestricted,
				audit.PolicyValueSnippetRestrict,
				audit.PolicyValueSnippetUnspecified,
				"":
				if policyValue == "" {
					policyValue = audit.PolicyValueSnippetUnspecified
				}
			default:
				policyInvalid = true
			}
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
		googlebotMetaRawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "googlebot_meta_robots_raw")
		xRobotsRawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "x_robots_raw")
		googlebotXRobotsRawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "googlebot_x_robots_raw")

		hasNormalizedSnippetObs := len(nosnippetObs) > 0 || len(maxSnippetObs) > 0 || len(dataNosnippetObs) > 0
		rawEvidenceRole := audit.EvidenceRoleContext
		if !hasNormalizedSnippetObs {
			rawEvidenceRole = audit.EvidenceRolePrimary
		}

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
				Role:              rawEvidenceRole,
			})
		}
		for _, o := range googlebotMetaRawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:googlebot_meta_robots_raw", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "googlebot_meta_robots_raw",
				ObservedValue:     o.Value,
				Role:              rawEvidenceRole,
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
				Role:              rawEvidenceRole,
			})
		}
		for _, o := range googlebotXRobotsRawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:googlebot_x_robots_raw", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "googlebot_x_robots_raw",
				ObservedValue:     o.Value,
				Role:              rawEvidenceRole,
			})
		}

		if policyInvalid {
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
				ObservedSummary: fmt.Sprintf("Unrecognized snippet_policy value %q supplied for SubjectURL.", policyValue),
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

		// Check if directives are available
		hasDirectiveEvidence := hasNormalizedSnippetObs ||
			len(metaRawObs) > 0 || len(googlebotMetaRawObs) > 0 ||
			len(xRobotsRawObs) > 0 || len(googlebotXRobotsRawObs) > 0

		if !hasDirectiveEvidence {
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

		// Check for observation conflicts and malformed values
		nosnippetBool, nosnippetUsable, nosnippetConflict := resolveSingleBoolObservation(nosnippetObs)
		nosnippetMalformed := len(nosnippetObs) > 0 && !nosnippetUsable && !nosnippetConflict

		maxSnippetStr, maxSnippetUsable, maxSnippetConflict := resolveSingleStringObservation(maxSnippetObs)
		dataNosnippetStr, dataNosnippetUsable, dataNosnippetConflict := resolveSingleStringObservation(dataNosnippetObs)

		maxSnippetVal := -1
		hasMaxSnippet := false
		maxSnippetMalformed := false
		if len(maxSnippetObs) > 0 {
			if !maxSnippetUsable && !maxSnippetConflict {
				maxSnippetMalformed = true
			} else if maxSnippetUsable {
				if maxSnippetStr == "" {
					maxSnippetMalformed = true
				} else if v, err := strconv.Atoi(maxSnippetStr); err != nil || v < -1 {
					maxSnippetMalformed = true
				} else {
					maxSnippetVal = v
					if maxSnippetVal >= 0 {
						hasMaxSnippet = true
					}
				}
			}
		}

		dataNosnippetCount := 0
		dataNosnippetMalformed := false
		if len(dataNosnippetObs) > 0 {
			if !dataNosnippetUsable && !dataNosnippetConflict {
				dataNosnippetMalformed = true
			} else if dataNosnippetUsable {
				if dataNosnippetStr == "" {
					dataNosnippetMalformed = true
				} else if v, err := strconv.Atoi(dataNosnippetStr); err != nil || v < 0 {
					dataNosnippetMalformed = true
				} else {
					dataNosnippetCount = v
				}
			}
		}

		if nosnippetConflict || maxSnippetConflict || dataNosnippetConflict ||
			nosnippetMalformed || maxSnippetMalformed || dataNosnippetMalformed {
			obsSummary := "Conflicting snippet directive observations observed on SubjectURL."
			if nosnippetMalformed || maxSnippetMalformed || dataNosnippetMalformed {
				obsSummary = "Malformed snippet directive observation value on SubjectURL."
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

		// Check if raw observations or directive bundles have malformed snippet tokens or ambiguous scope
		hasRawAmbiguity := false
		scopeUnknownObs := idx.GetObservations(audit.SubjectURL, subjectRef, "directive_scope_unknown")
		for _, o := range scopeUnknownObs {
			if strings.EqualFold(strings.TrimSpace(o.Value), "true") {
				hasRawAmbiguity = true
				break
			}
		}
		parseErrObs := idx.GetObservations(audit.SubjectURL, subjectRef, "robots_parse_errors")
		for _, o := range parseErrObs {
			if strings.Contains(o.Value, "max-snippet") || strings.Contains(o.Value, "nosnippet") {
				hasRawAmbiguity = true
				break
			}
		}

		// Always inspect raw meta/x-robots tokens if present to detect malformed snippet tokens
		// and as fallback when normalized nosnippet/max_snippet observations are absent.
		rawFallbackUsed := !nosnippetUsable && !maxSnippetUsable
		var rawObsList []audit.NormalizedObservation
		rawObsList = append(rawObsList, metaRawObs...)
		rawObsList = append(rawObsList, googlebotMetaRawObs...)
		rawObsList = append(rawObsList, xRobotsRawObs...)
		rawObsList = append(rawObsList, googlebotXRobotsRawObs...)

		for _, o := range rawObsList {
			tokens, ok := parseRobotsDirectiveSet(o.Value)
			if !ok {
				hasRawAmbiguity = true
				continue
			}
			for _, tok := range tokens {
				if tok == "nosnippet" {
					if rawFallbackUsed {
						hasNosnippet = true
					}
				}
				if strings.HasPrefix(tok, "max-snippet") {
					if !strings.HasPrefix(tok, "max-snippet:") {
						hasRawAmbiguity = true
						continue
					}
					numStr := strings.TrimPrefix(tok, "max-snippet:")
					n, err := strconv.Atoi(numStr)
					if err != nil || n < -1 {
						hasRawAmbiguity = true
						continue
					}
					if rawFallbackUsed {
						if n >= 0 {
							if !hasMaxSnippet || n < maxSnippetVal {
								maxSnippetVal = n
							}
							hasMaxSnippet = true
						} else if n == -1 && !hasMaxSnippet {
							maxSnippetVal = -1
						}
					}
				}
			}
		}

		hasDataNosnippet := (dataNosnippetCount > 0)
		hasRestriction := hasNosnippet || hasMaxSnippet || hasDataNosnippet

		// If no restriction was proven, ensure we actually have non-ambiguous evidence of absence
		if !hasRestriction {
			// Check if HTML or HTTP header extraction was incomplete when raw fallback or partial evidence is used
			htmlStatusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "html_extraction_status")
			headerStatusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "http_header_status")
			renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "rendered")

			channelIncomplete := false
			for _, o := range renderedObs {
				if strings.EqualFold(strings.TrimSpace(o.Value), "true") {
					channelIncomplete = true
				}
			}
			for _, o := range htmlStatusObs {
				st := strings.TrimSpace(o.Value)
				if st != "" && st != "complete" {
					channelIncomplete = true
				}
			}
			for _, o := range headerStatusObs {
				st := strings.TrimSpace(o.Value)
				if st != "" && st != "complete" {
					channelIncomplete = true
				}
			}

			// If only data_nosnippet_count=0 was provided without any directive evidence (nosnippet/max_snippet/raw),
			// directive state is incomplete.
			onlyDataNosnippetZero := !nosnippetUsable && !maxSnippetUsable && len(rawObsList) == 0 && dataNosnippetUsable && dataNosnippetCount == 0

			if hasRawAmbiguity || channelIncomplete || onlyDataNosnippetZero {
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
					ObservedSummary: "Snippet directive evidence is incomplete or ambiguous on SubjectURL; cannot prove absence of restriction.",
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
		}

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
				if hasDataNosnippet && !hasNosnippet && !hasMaxSnippet {
					evalStatus = audit.StatusManualReview
					observedSummary = fmt.Sprintf("Block-level data-nosnippet restriction observed (%d elements); manual review required to evaluate restriction scope.", dataNosnippetCount)
				} else {
					evalStatus = audit.StatusPass
					observedSummary = "Explicit snippet policy 'restrict' permits observed snippet restriction."
				}

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
