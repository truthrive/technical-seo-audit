package engine

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDCANON004 = "AR-CANON-004"

// evaluateCANON004 implements the typed execution logic for:
// AR-CANON-004 — Multiple canonical declarations (parent check: CANON-003, P1, deterministic).
//
// Contract:
// - Required inputs: url, canonical_values, normalized_canonical_values
//   (via url_identity, canonical_count, canonical_normalization_complete,
//    canonical_distinct_normalized_count, canonical_normalized_target)
// - Preconditions: at least one canonical declaration exists.
// - PASS: canonical_count = 1.
//   (Do not require normalization to PASS when declaration count is exactly 1.
//    Do not evaluate canonical syntax.)
// - WARNING: canonical_count > 1, normalization complete=true, distinct count=1.
// - FAIL: canonical_count > 1, normalization complete=true, distinct count > 1.
// - UNKNOWN: canonical_count > 1, normalization incomplete/unavailable;
//            or missing, malformed, or conflicting canonical_count;
//            or malformed/conflicting completeness or distinct-count evidence;
//            or missing/conflicting url_identity.
// - NOT_APPLICABLE: canonical_count = 0.
// - No MANUAL_REVIEW.
func evaluateCANON004(
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

		// 1. Resolve URL identity
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
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

		// 2. Resolve canonical_count
		countObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_count")
		var (
			countUsable    bool
			countConflict  bool
			countMalformed bool
			firstCountStr  string
			canonCount     int
		)
		if len(countObs) > 0 {
			firstCountStr = strings.TrimSpace(countObs[0].Value)
			countUsable = true
			for _, co := range countObs[1:] {
				if strings.TrimSpace(co.Value) != firstCountStr {
					countConflict = true
					countUsable = false
					break
				}
			}
			if !countConflict {
				c, err := strconv.Atoi(firstCountStr)
				if err != nil || c < 0 {
					countMalformed = true
					countUsable = false
				} else {
					canonCount = c
				}
			}
		}

		buildCountRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, co := range countObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_count", ruleResultID, co.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(co.ObservationID),
					Field:             "canonical_count",
					ObservedValue:     co.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 3. Resolve canonical_normalized_target
		normTargetObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_normalized_target")
		buildNormTargetContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, no := range normTargetObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_normalized_target", ruleResultID, no.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(no.ObservationID),
					Field:             "canonical_normalized_target",
					ObservedValue:     no.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			return refs
		}

		// 4. Resolve canonical_normalization_complete
		completeObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_normalization_complete")
		var (
			completeUsable    bool
			completeConflict  bool
			completeMalformed bool
			firstCompleteStr  string
			completeVal       bool
		)
		if len(completeObs) > 0 {
			firstCompleteStr = strings.TrimSpace(completeObs[0].Value)
			completeUsable = true
			for _, co := range completeObs[1:] {
				if strings.TrimSpace(co.Value) != firstCompleteStr {
					completeConflict = true
					completeUsable = false
					break
				}
			}
			if !completeConflict {
				if firstCompleteStr == "true" {
					completeVal = true
				} else if firstCompleteStr == "false" {
					completeVal = false
				} else {
					completeMalformed = true
					completeUsable = false
				}
			}
		}

		buildCompleteRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, co := range completeObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_normalization_complete", ruleResultID, co.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(co.ObservationID),
					Field:             "canonical_normalization_complete",
					ObservedValue:     co.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 5. Resolve canonical_distinct_normalized_count
		distinctObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_distinct_normalized_count")
		var (
			distinctUsable    bool
			distinctConflict  bool
			distinctMalformed bool
			firstDistinctStr  string
			distinctCount     int
		)
		if len(distinctObs) > 0 {
			firstDistinctStr = strings.TrimSpace(distinctObs[0].Value)
			distinctUsable = true
			for _, do := range distinctObs[1:] {
				if strings.TrimSpace(do.Value) != firstDistinctStr {
					distinctConflict = true
					distinctUsable = false
					break
				}
			}
			if !distinctConflict {
				d, err := strconv.Atoi(firstDistinctStr)
				if err != nil || d <= 0 {
					distinctMalformed = true
					distinctUsable = false
				} else {
					distinctCount = d
				}
			}
		}

		buildDistinctRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, do := range distinctObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_distinct_normalized_count", ruleResultID, do.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(do.ObservationID),
					Field:             "canonical_distinct_normalized_count",
					ObservedValue:     do.Value,
					Role:              role,
				})
			}
			return refs
		}

		// 6. Determine status, summary, and evidence refs
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
			evidenceRefs    = buildURLContextRefs()
		)

		if !urlUsable {
			evalStatus = audit.StatusUnknown
			if urlConflict {
				observedSummary = "No usable URL identity is available: conflicting URL identity observations."
			} else {
				observedSummary = "Required URL identity evidence is unavailable."
			}
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if !countUsable {
			evalStatus = audit.StatusUnknown
			if len(countObs) == 0 {
				observedSummary = "Required canonical count evidence is unavailable."
			} else if countConflict {
				observedSummary = "Canonical count evidence is conflicting."
			} else if countMalformed {
				observedSummary = fmt.Sprintf("Canonical count evidence is malformed: %q.", firstCountStr)
			}
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if canonCount == 0 {
			// canonical_count = 0 -> NOT_APPLICABLE
			evalStatus = audit.StatusNotApplicable
			observedSummary = "Page has no canonical declarations (canonical_count = 0)."
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if canonCount == 1 {
			// canonical_count = 1 -> PASS (syntax/normalization not required to PASS)
			evalStatus = audit.StatusPass
			observedSummary = "Page has exactly one canonical declaration."
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRolePrimary)...)
			evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
		} else {
			// canonCount > 1
			if !completeUsable {
				evalStatus = audit.StatusUnknown
				if len(completeObs) == 0 {
					observedSummary = "Multiple canonical declarations exist, but normalization completeness evidence is unavailable."
				} else if completeConflict {
					observedSummary = "Canonical normalization completeness evidence is conflicting."
				} else if completeMalformed {
					observedSummary = fmt.Sprintf("Canonical normalization completeness evidence is malformed: %q.", firstCompleteStr)
				}
				evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
			} else if !completeVal {
				// complete = false -> UNKNOWN
				evalStatus = audit.StatusUnknown
				observedSummary = "Multiple canonical declarations exist, but one or more declarations cannot be normalized to a valid HTTP(S) target."
				evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
				evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRolePrimary)...)
				evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
			} else {
				// complete = true -> inspect distinct count
				if !distinctUsable {
					evalStatus = audit.StatusUnknown
					if len(distinctObs) == 0 {
						observedSummary = "Multiple canonical declarations exist with normalization complete, but distinct target count evidence is unavailable."
					} else if distinctConflict {
						observedSummary = "Distinct canonical target count evidence is conflicting."
					} else if distinctMalformed {
						observedSummary = fmt.Sprintf("Distinct canonical target count evidence is malformed: %q.", firstDistinctStr)
					}
					evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildDistinctRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
				} else if distinctCount == 1 {
					// multiple declarations normalize to same target -> WARNING
					evalStatus = audit.StatusWarning
					observedSummary = fmt.Sprintf("Page has %d canonical declarations that normalize to the same target.", canonCount)
					evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildDistinctRefs(audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
				} else {
					// multiple distinct targets -> FAIL
					evalStatus = audit.StatusFail
					observedSummary = fmt.Sprintf("Page has %d canonical declarations with %d distinct normalized targets.", canonCount, distinctCount)
					evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
					evidenceRefs = append(evidenceRefs, buildDistinctRefs(audit.EvidenceRolePrimary)...)
					evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
				}
			}
		}

		// Deduplicate and sort evidence refs deterministically
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
			ExpectedSummary: "Page has exactly one canonical declaration or multiple declarations normalizing to the same target.",
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
