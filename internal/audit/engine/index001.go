package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDINDEX001 = "AR-INDEX-001"

// evaluateINDEX001 implements the typed execution logic for:
// AR-INDEX-001 — Effective noindex conflicts with explicit indexability intent
// (parent check: INDEX-001, P1, assisted).
//
// Contract:
// - Required inputs: url, effective_noindex, expected_indexable
// - Precondition: expected_indexable=true explicitly supplied.
// - PASS: expected_indexable=true and effective_noindex=false.
// - FAIL: expected_indexable=true and effective_noindex=true.
// - NOT_APPLICABLE: expected_indexable is missing, false, or unspecified.
// - UNKNOWN: expected_indexable=true, but URL identity or effective_noindex is unavailable/unusable.
//
// Subject set:
// The deterministic union of URL subject refs in snapshot evidence and
// URL policy target refs for expected_indexable.
func evaluateINDEX001(
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

	// 1. Determine subject set: deterministic union of evidence URL refs and policy expected_indexable targets
	subjectSet := make(map[string]struct{})
	for _, ref := range idx.SubjectRefs(audit.SubjectURL) {
		subjectSet[ref] = struct{}{}
	}
	if policies != nil {
		for _, ref := range policies.TargetRefs(audit.PolicyScopeURL) {
			if _, ok := policies.Get(audit.PolicyScopeURL, ref, audit.PolicyKeyExpectedIndexable); ok {
				subjectSet[ref] = struct{}{}
			}
		}
	}

	subjects := make([]string, 0, len(subjectSet))
	for ref := range subjectSet {
		subjects = append(subjects, ref)
	}
	sort.Strings(subjects)

	results := make([]audit.RuleResult, 0, len(subjects))

	for _, subjectRef := range subjects {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// 2. Resolve explicit policy for expected_indexable
		var (
			policyAssignment *audit.ProjectPolicyAssignment
			hasPolicy        bool
		)
		if policies != nil {
			policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeURL, subjectRef, audit.PolicyKeyExpectedIndexable)
		}

		// Helper to build URL context evidence refs from observations
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
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

		// Helper to build Policy context evidence ref
		buildPolicyContextRef := func() *audit.RuleEvidenceRef {
			if !hasPolicy || policyAssignment == nil {
				return nil
			}
			ref := audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:expected_indexable", ruleResultID, policyAssignment.PolicyAssignmentID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "PROJECT_POLICY_ASSIGNMENT",
				EvidenceRef:       string(policyAssignment.PolicyAssignmentID),
				Field:             "expected_indexable",
				ObservedValue:     policyAssignment.PolicyValue,
				Role:              audit.EvidenceRoleContext,
			}
			return &ref
		}

		// 3. Evaluate policy preconditions:
		// If expected_indexable is missing, false, or unspecified -> NOT_APPLICABLE
		if !hasPolicy || policyAssignment.PolicyValue != audit.PolicyValueTrue {
			var observedSummary string
			if !hasPolicy {
				observedSummary = "Explicit indexability intent is not supplied (policy expected_indexable is missing)."
			} else if policyAssignment.PolicyValue == audit.PolicyValueFalse {
				observedSummary = "Explicit indexability intent is false (policy expected_indexable=false)."
			} else if policyAssignment.PolicyValue == audit.PolicyValueUnspecified {
				observedSummary = "Explicit indexability intent is unspecified (policy expected_indexable=unspecified)."
			} else {
				observedSummary = fmt.Sprintf("Explicit indexability intent is %q (expected_indexable != true).", policyAssignment.PolicyValue)
			}

			evidenceRefs := buildURLContextRefs()
			if polRef := buildPolicyContextRef(); polRef != nil {
				evidenceRefs = append(evidenceRefs, *polRef)
			}
			sort.Slice(evidenceRefs, func(i, j int) bool {
				return evidenceRefs[i].RuleEvidenceRefID < evidenceRefs[j].RuleEvidenceRefID
			})

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
				ObservedSummary: observedSummary,
				ExpectedSummary: "URL expected to be indexable has no effective noindex.",
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Precondition met: expected_indexable = true.
		// Validate required inputs: url_identity and effective_noindex.

		// Validate URL identity:
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

		// Validate effective_noindex:
		effObs := idx.GetObservations(audit.SubjectURL, subjectRef, "effective_noindex")
		var (
			effUsable    bool
			effConflict  bool
			effMalformed bool
			effVal       bool
			firstEffStr  string
		)
		if len(effObs) > 0 {
			firstEffStr = strings.TrimSpace(effObs[0].Value)
			effUsable = true
			for _, eo := range effObs[1:] {
				if strings.TrimSpace(eo.Value) != firstEffStr {
					effConflict = true
					effUsable = false
					break
				}
			}
			if !effConflict {
				if firstEffStr == "true" {
					effVal = true
				} else if firstEffStr == "false" {
					effVal = false
				} else {
					effMalformed = true
					effUsable = false
				}
			}
		}

		// 5. Determine evaluation status and summary
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
			evidenceRefs    = buildURLContextRefs()
		)

		// Always include policy context ref when policy is present
		if polRef := buildPolicyContextRef(); polRef != nil {
			evidenceRefs = append(evidenceRefs, *polRef)
		}

		if !urlUsable {
			evalStatus = audit.StatusUnknown
			if urlConflict {
				observedSummary = "No usable URL identity is available: conflicting URL identity observations."
			} else if len(urlObs) == 0 {
				observedSummary = "Required URL identity evidence is unavailable."
			} else {
				observedSummary = "URL identity evidence is empty or invalid."
			}

			// Include effective_noindex if present
			for _, eo := range effObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:effective_noindex", ruleResultID, eo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(eo.ObservationID),
					Field:             "effective_noindex",
					ObservedValue:     eo.Value,
					Role:              audit.EvidenceRolePrimary,
				})
			}
		} else if !effUsable {
			evalStatus = audit.StatusUnknown
			if len(effObs) == 0 {
				observedSummary = "Required effective_noindex evidence is unavailable."
			} else if effConflict {
				observedSummary = "Effective noindex evidence is conflicting."
			} else if effMalformed {
				observedSummary = fmt.Sprintf("Effective noindex evidence is malformed: %q.", firstEffStr)
			}

			// Include all actual effective_noindex observations
			for _, eo := range effObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:effective_noindex", ruleResultID, eo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(eo.ObservationID),
					Field:             "effective_noindex",
					ObservedValue:     eo.Value,
					Role:              audit.EvidenceRolePrimary,
				})
			}
		} else {
			// Both URL identity and effective_noindex are usable
			// Add primary effective_noindex ref
			for _, eo := range effObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:effective_noindex", ruleResultID, eo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(eo.ObservationID),
					Field:             "effective_noindex",
					ObservedValue:     eo.Value,
					Role:              audit.EvidenceRolePrimary,
				})
			}

			if effVal {
				// FAIL: policy expected_indexable=true, but effective_noindex=true
				evalStatus = audit.StatusFail
				observedSummary = "Effective noindex is true, conflicting with explicit expected_indexable=true policy."

				// Include relevant underlying directive observations without guessing correlation
				underlyingObs := findDecidingNoindexDirectives(idx, subjectRef)
				for _, do := range underlyingObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, do.ObservationID, do.Field)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(do.ObservationID),
						Field:             do.Field,
						ObservedValue:     do.Value,
						Role:              audit.EvidenceRoleSupporting,
					})
				}
			} else {
				// PASS: policy expected_indexable=true and effective_noindex=false
				evalStatus = audit.StatusPass
				observedSummary = "Effective noindex is false, matching explicit expected_indexable=true policy."
			}
		}

		// Deduplicate evidence refs by RuleEvidenceRefID and sort deterministically
		seenRefIDs := make(map[audit.RuleEvidenceRefID]struct{}, len(evidenceRefs))
		dedupedRefs := make([]audit.RuleEvidenceRef, 0, len(evidenceRefs))
		for _, ref := range evidenceRefs {
			if _, exists := seenRefIDs[ref.RuleEvidenceRefID]; !exists {
				seenRefIDs[ref.RuleEvidenceRefID] = struct{}{}
				dedupedRefs = append(dedupedRefs, ref)
			}
		}
		sort.Slice(dedupedRefs, func(i, j int) bool {
			return dedupedRefs[i].RuleEvidenceRefID < dedupedRefs[j].RuleEvidenceRefID
		})

		// Guard: PASS or FAIL must have usable inputs and valid refs
		if evalStatus == audit.StatusPass || evalStatus == audit.StatusFail {
			if !urlUsable || !effUsable || !hasPolicy || policyAssignment.PolicyValue != audit.PolicyValueTrue {
				return nil, fmt.Errorf("audit engine: internal error: %s produced %s with invalid preconditions/inputs", ruleResultID, evalStatus)
			}
			hasURLContext := false
			hasPolicyContext := false
			hasEffectivePrimary := false
			for _, ref := range dedupedRefs {
				if ref.Field == "url" && ref.Role == audit.EvidenceRoleContext {
					hasURLContext = true
				}
				if ref.Field == "expected_indexable" && ref.Role == audit.EvidenceRoleContext && ref.EvidenceType == "PROJECT_POLICY_ASSIGNMENT" {
					hasPolicyContext = true
				}
				if ref.Field == "effective_noindex" && ref.Role == audit.EvidenceRolePrimary {
					hasEffectivePrimary = true
				}
			}
			if !hasURLContext || !hasPolicyContext || !hasEffectivePrimary {
				return nil, fmt.Errorf("audit engine: internal error: %s missing required evidence refs (url: %v, policy: %v, effective: %v)",
					ruleResultID, hasURLContext, hasPolicyContext, hasEffectivePrimary)
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
			ExpectedSummary: "URL expected to be indexable has no effective noindex.",
			EvaluatedAt:     evalTime,
			EvidenceRefs:    dedupedRefs,
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
		}

		results = append(results, rr)
	}

	return results, nil
}

// findDecidingNoindexDirectives extracts normalized observations belonging to
// directive bundles that contribute to Googlebot-effective noindex state
// (target is generic or googlebot, scope is known, tokens contain noindex or none).
// Only explicit directive references (directive:*) are correlated.
func findDecidingNoindexDirectives(idx *EvidenceIndex, subjectRef string) []audit.NormalizedObservation {
	var candidateObs []audit.NormalizedObservation
	candidateObs = append(candidateObs, idx.GetObservations(audit.SubjectURL, subjectRef, "directive_source")...)
	candidateObs = append(candidateObs, idx.GetObservations(audit.SubjectURL, subjectRef, "directive_target")...)
	candidateObs = append(candidateObs, idx.GetObservations(audit.SubjectURL, subjectRef, "directive_raw")...)
	candidateObs = append(candidateObs, idx.GetObservations(audit.SubjectURL, subjectRef, "directive_tokens")...)
	candidateObs = append(candidateObs, idx.GetObservations(audit.SubjectURL, subjectRef, "directive_scope_unknown")...)
	candidateObs = append(candidateObs, idx.GetObservations(audit.SubjectURL, subjectRef, "robots_directive_tokens")...)
	candidateObs = append(candidateObs, idx.GetObservations(audit.SubjectURL, subjectRef, "robots_meta_tokens")...)
	candidateObs = append(candidateObs, idx.GetObservations(audit.SubjectURL, subjectRef, "x_robots_tokens")...)

	type dirBundle struct {
		target       string
		scopeUnknown bool
		tokens       string
		allObs       []audit.NormalizedObservation
	}

	bundleMap := make(map[string]*dirBundle)
	var bundleKeys []string

	for _, obs := range candidateObs {
		dRef := extractDirectiveRef(obs.SourceEvidenceRefs)
		if dRef == "" {
			continue
		}
		b, exists := bundleMap[dRef]
		if !exists {
			b = &dirBundle{}
			bundleMap[dRef] = b
			bundleKeys = append(bundleKeys, dRef)
		}
		b.allObs = append(b.allObs, obs)
		switch obs.Field {
		case "directive_target":
			b.target = strings.TrimSpace(obs.Value)
		case "directive_scope_unknown":
			if strings.TrimSpace(strings.ToLower(obs.Value)) == "true" {
				b.scopeUnknown = true
			}
		case "directive_tokens":
			b.tokens = obs.Value
		}
	}

	sort.Strings(bundleKeys)

	var decidingObs []audit.NormalizedObservation
	for _, bKey := range bundleKeys {
		b := bundleMap[bKey]
		if b.scopeUnknown {
			continue
		}
		normTgt := strings.ToLower(b.target)
		if normTgt != "*" && normTgt != "googlebot" {
			continue
		}
		hasNoindex := false
		for _, part := range strings.Split(b.tokens, ",") {
			tok := strings.ToLower(strings.TrimSpace(part))
			if tok == "noindex" || tok == "none" {
				hasNoindex = true
				break
			}
		}
		if hasNoindex {
			decidingObs = append(decidingObs, b.allObs...)
		}
	}

	return decidingObs
}
