package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDCANON008 = "AR-CANON-008"

// evaluateCANON008 implements the typed execution logic for:
// AR-CANON-008 — Redirect chain exceeds one hop (parent check: CANON-007, P1, deterministic).
//
// Precondition: Initial response redirects.
//
// Status semantics:
// - Initial redirect=false with no conflicting redirect evidence -> NOT_APPLICABLE
// - Initial redirect=true, complete=true, verified hop count=1 -> PASS
// - Initial redirect=true, complete=true, verified hop count>=2 -> FAIL
// - Complete=true + loop=true -> UNKNOWN (contradictory evidence)
// - Redirect traversal incomplete=false -> UNKNOWN
// - Completeness missing -> UNKNOWN
// - Hop count missing/malformed/conflicting -> UNKNOWN
// - Hop observations missing/malformed/conflicting/noncontiguous/discontinuous -> UNKNOWN
// - Initial redirect unknown or contradictory -> UNKNOWN
//
// An incomplete redirect traversal is never marked FAIL merely because multiple hops were observed.
// Hop count must agree with contiguous, continuous, verified redirect_hop observations.
func evaluateCANON008(
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
		b := extractRedirectEvidence(idx, subjectRef)

		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
			evidenceRefs    []audit.RuleEvidenceRef
		)

		if !b.URLUsable {
			evalStatus = audit.StatusUnknown
			observedSummary = "URL identity is missing or conflicting."
			evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
		} else if !b.InitialUsable || len(b.InitialObs) == 0 {
			evalStatus = audit.StatusUnknown
			observedSummary = "Initial redirect state is unavailable, malformed, or contradictory."
			evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRolePrimary)...)
		} else if !b.IsInitialRedirect {
			if b.hasConflictingRedirectEvidence() {
				evalStatus = audit.StatusUnknown
				observedSummary = "Initial response observed as non-redirect, but contradictory redirect evidence is present."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
			} else {
				evalStatus = audit.StatusNotApplicable
				observedSummary = "URL does not redirect; initial response is not a redirect."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				if len(b.HopCountObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
				}
			}
		} else {
			// Precondition met: Initial response redirects
			if b.CompleteConflict || b.CompleteMalformed {
				evalStatus = audit.StatusUnknown
				observedSummary = "Contradictory or malformed redirect traversal completeness evidence."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
			} else if len(b.CompleteObs) == 0 {
				evalStatus = audit.StatusUnknown
				observedSummary = "Redirect traversal completeness evidence is missing; completeness cannot be verified."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleSupporting)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
			} else if b.CompleteUsable && b.TraversalComplete && b.LoopUsable && b.LoopDetected {
				// Contradictory: complete=true + loop=true -> UNKNOWN
				evalStatus = audit.StatusUnknown
				observedSummary = "Contradictory redirect evidence: redirect traversal marked complete while redirect loop is detected."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
				if len(b.HopCountObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleSupporting)...)
				}
				if len(b.HopObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
				}
			} else if !b.TraversalComplete {
				evalStatus = audit.StatusUnknown
				observedSummary = "Redirect traversal is incomplete; total redirect chain length cannot be determined."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleSupporting)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
			} else {
				// Traversal is complete: validate hop count and individual hop observations
				if len(b.HopCountObs) == 0 {
					evalStatus = audit.StatusUnknown
					observedSummary = "Redirect hop count evidence is missing."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
				} else if b.HopCountConflict || b.HopCountMalformed {
					evalStatus = audit.StatusUnknown
					observedSummary = "Redirect hop count evidence is conflicting or malformed."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRolePrimary)...)
				} else if b.HopCount < 1 {
					evalStatus = audit.StatusUnknown
					observedSummary = fmt.Sprintf("Redirect hop count (%d) is invalid for a complete redirect traversal.", b.HopCount)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRolePrimary)...)
				} else if len(b.HopObs) == 0 {
					evalStatus = audit.StatusUnknown
					observedSummary = "Redirect traversal marked complete, but individual redirect hop observations are missing."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRolePrimary)...)
				} else if b.HopMalformed {
					evalStatus = audit.StatusUnknown
					observedSummary = "Redirect hop observation payload is malformed or invalid."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
				} else if b.HopConflict || b.HopIndexDup {
					evalStatus = audit.StatusUnknown
					observedSummary = "Contradictory or duplicate redirect hop indices detected."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
				} else if b.HopNonContig {
					evalStatus = audit.StatusUnknown
					observedSummary = "Noncontiguous redirect hop indices detected."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
				} else if b.HopChainInconsistent {
					evalStatus = audit.StatusUnknown
					observedSummary = "Redirect chain evidence is discontinuous or contradicts URL identity."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
					if len(b.FinalURLObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.FinalURLObs, "redirect_final_url", audit.EvidenceRoleContext)...)
					}
				} else if len(b.UniqueHops) != b.HopCount {
					evalStatus = audit.StatusUnknown
					observedSummary = fmt.Sprintf("Redirect hop count (%d) does not match usable redirect hop observations (%d).", b.HopCount, len(b.UniqueHops))
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
				} else if b.HopCount == 1 {
					evalStatus = audit.StatusPass
					observedSummary = "Redirect chain completed in exactly one hop."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
					if len(b.FinalURLObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.FinalURLObs, "redirect_final_url", audit.EvidenceRoleContext)...)
					}
				} else {
					evalStatus = audit.StatusFail
					observedSummary = fmt.Sprintf("Redirect chain exceeds one hop (%d hops observed before final destination).", b.HopCount)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
					if len(b.FinalURLObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.FinalURLObs, "redirect_final_url", audit.EvidenceRoleContext)...)
					}
				}
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
			ExpectedSummary: "Initial redirect completes in exactly one hop without chaining.",
			EvaluatedAt:     evalTime,
			EvidenceRefs:    dedupeAndSortEvidenceRefs(evidenceRefs),
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: generated invalid rule result for %s in %s: %w", subjectRef, rule.RuleID, err)
		}

		results = append(results, rr)
	}

	return results, nil
}
