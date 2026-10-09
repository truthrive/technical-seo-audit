package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDCANON009 = "AR-CANON-009"

// evaluateCANON009 implements the typed execution logic for:
// AR-CANON-009 — Redirect loop detected (parent check: CANON-007, P1, deterministic).
//
// Precondition: At least one redirect occurs.
//
// Status semantics:
// - Verified no redirect -> NOT_APPLICABLE
// - Confirmed redirect + explicit redirect_loop_detected=true -> FAIL
// - Loop=true + complete=true is contradictory -> UNKNOWN
// - Confirmed redirect + redirect_traversal_complete=true + redirect_loop_detected=false
//   PASSes ONLY when required redirect chain evidence is internally consistent -> PASS
// - Loop=false + complete=true with missing/malformed/conflicting hop/count evidence -> UNKNOWN
// - Complete traversal with conflicting/malformed final URL evidence -> UNKNOWN
// - Redirect occurred but loop state unavailable -> UNKNOWN
// - Incomplete traversal without confirmed loop -> UNKNOWN
// - Conflicting/malformed loop evidence -> UNKNOWN
// - Conflicting/malformed completeness evidence -> UNKNOWN
// - Contradictory redirect precondition -> UNKNOWN
//
// A confirmed loop may FAIL even when traversal is incomplete, because the loop itself is positive evidence.
// Loop=false is never inferred merely because no loop evidence was emitted.
func evaluateCANON009(
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
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
				if len(b.FinalURLObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.FinalURLObs, "redirect_final_url", audit.EvidenceRolePrimary)...)
				}
			} else {
				evalStatus = audit.StatusNotApplicable
				observedSummary = "URL does not redirect; redirect loop check is not applicable."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				if len(b.HopCountObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
				}
			}
		} else {
			// Precondition met: Initial response redirects
			if b.LoopConflict || b.LoopMalformed {
				evalStatus = audit.StatusUnknown
				observedSummary = "Contradictory or malformed redirect loop detection evidence."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
			} else if len(b.LoopObs) == 0 {
				evalStatus = audit.StatusUnknown
				observedSummary = "Redirect occurred, but redirect loop detection evidence is unavailable."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				if len(b.CompleteObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
				}
				if len(b.HopCountObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
				}
				if len(b.HopObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
				}
			} else if b.CompleteConflict || b.CompleteMalformed {
				// Conflicting or malformed completeness -> UNKNOWN (do not evaluate to FAIL or PASS)
				evalStatus = audit.StatusUnknown
				observedSummary = "Contradictory or malformed redirect traversal completeness evidence."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
			} else if b.LoopDetected && b.CompleteUsable && b.TraversalComplete {
				// Contradictory: loop=true + complete=true -> UNKNOWN
				evalStatus = audit.StatusUnknown
				observedSummary = "Contradictory redirect evidence: redirect traversal marked complete while redirect loop is detected."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
				if len(b.HopCountObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleSupporting)...)
				}
				if len(b.HopObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
				}
			} else if b.LoopDetected {
				// Confirmed loop with incomplete traversal remains FAIL if positive loop evidence is otherwise trustworthy
				evalStatus = audit.StatusFail
				observedSummary = "Redirect traversal entered a loop."
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
				if len(b.CompleteObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
				}
				if len(b.HopCountObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
				}
				if len(b.HopObs) > 0 {
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
				}
			} else {
				// LoopDetected == false: verify that traversal completeness is proven
				if !b.CompleteUsable || !b.TraversalComplete {
					evalStatus = audit.StatusUnknown
					observedSummary = "Redirect traversal is incomplete without a confirmed loop; loop-absence cannot be proven."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
					if len(b.CompleteObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
					}
					if len(b.HopCountObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
					}
					if len(b.HopObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
					}
				} else if b.FinalURLConflict || b.FinalURLMalformed {
					// Complete traversal: conflicting or malformed final URL evidence -> UNKNOWN
					evalStatus = audit.StatusUnknown
					observedSummary = "Redirect traversal completed, but final URL evidence is conflicting or malformed."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					if len(b.HopCountObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
					}
					if len(b.HopObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleContext)...)
					}
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.FinalURLObs, "redirect_final_url", audit.EvidenceRolePrimary)...)
				} else if !b.isHopEvidenceValidAndConsistent() {
					// loop=false + complete=true may PASS ONLY when required redirect chain evidence is internally consistent.
					// Missing, malformed or conflicting hop/count evidence must not produce false PASS.
					evalStatus = audit.StatusUnknown
					observedSummary = "Redirect traversal completed with loop=false, but redirect chain evidence is missing, malformed, or inconsistent."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRolePrimary)...)
					if len(b.HopCountObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleSupporting)...)
					}
					if len(b.HopObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
					}
				} else {
					evalStatus = audit.StatusPass
					observedSummary = "Redirect traversal completed without detecting a redirect loop."
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.URLIdentityObs, "url", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.InitialObs, "redirect_initial_observed", audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.LoopObs, "redirect_loop_detected", audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.CompleteObs, "redirect_traversal_complete", audit.EvidenceRoleContext)...)
					if len(b.HopCountObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopCountObs, "redirect_hop_count", audit.EvidenceRoleContext)...)
					}
					if len(b.HopObs) > 0 {
						evidenceRefs = append(evidenceRefs, buildEvidenceRefs(ruleResultID, b.HopObs, "redirect_hop", audit.EvidenceRoleSupporting)...)
					}
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
			ExpectedSummary: "Redirect chain completes without encountering cyclical redirect loops.",
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
