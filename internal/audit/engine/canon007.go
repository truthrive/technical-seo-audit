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

const ruleIDCANON007 = "AR-CANON-007"

// evaluateCANON007 implements the typed execution logic for:
// AR-CANON-007 — Canonical target is not noindex (parent check: CANON-003, P1, deterministic).
//
// Contract:
// - Required inputs: canonical_resolved_url, canonical_target_status, canonical_target_effective_noindex
// - Preconditions: canonical target is 200 and index directives extracted.
// - Semantics:
//   - no usable single canonical target -> NOT_APPLICABLE
//   - target subject cannot be correlated -> NOT_APPLICABLE
//   - target status unavailable/conflicting -> UNKNOWN
//   - target status != 200 -> NOT_APPLICABLE
//   - target status = 200 + effective_noindex unavailable -> UNKNOWN
//   - target status = 200 + effective_noindex=false -> PASS
//   - target status = 200 + effective_noindex=true -> FAIL
//   - malformed or conflicting effective state -> UNKNOWN
//   - Do not infer absent effective_noindex=false.
// - No WARNING or MANUAL_REVIEW.
func evaluateCANON007(
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

		// 6. Resolve canonical_target_subject_ref
		targetRefObs := idx.GetObservations(audit.SubjectURL, subjectRef, "canonical_target_subject_ref")
		var (
			targetRefUsable    bool
			targetRefConflict  bool
			targetRefMalformed bool
			targetSubjectRef   string
		)
		if len(targetRefObs) > 0 {
			targetSubjectRef = strings.TrimSpace(targetRefObs[0].Value)
			if targetSubjectRef != "" {
				targetRefUsable = true
				for _, to := range targetRefObs[1:] {
					if strings.TrimSpace(to.Value) != targetSubjectRef {
						targetRefConflict = true
						targetRefUsable = false
						break
					}
				}
			} else {
				targetRefMalformed = true
			}
		}

		buildTargetSubjectContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, to := range targetRefObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_target_subject_ref", ruleResultID, to.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(to.ObservationID),
					Field:             "canonical_target_subject_ref",
					ObservedValue:     to.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			return refs
		}

		// 7. Evaluate status, summary, and evidence refs
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
		} else if !countUsable || canonCount == 0 {
			// no usable single canonical target -> NOT_APPLICABLE
			evalStatus = audit.StatusNotApplicable
			if countMalformed {
				observedSummary = fmt.Sprintf("Canonical count evidence is malformed: %q; no usable single canonical target.", firstCountStr)
			} else if countConflict {
				observedSummary = "Canonical count evidence is conflicting; no usable single canonical target."
			} else {
				observedSummary = "Page has no usable single canonical target (no canonical declarations)."
			}
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
		} else if !completeUsable || !completeVal || !distinctUsable || distinctCount != 1 {
			// multiple distinct targets or incomplete normalization -> NOT_APPLICABLE (no usable single canonical target)
			evalStatus = audit.StatusNotApplicable
			if !completeUsable || !completeVal {
				if completeMalformed {
					observedSummary = fmt.Sprintf("Canonical normalization completeness evidence is malformed: %q; no usable single canonical target.", firstCompleteStr)
				} else if completeConflict {
					observedSummary = "Canonical normalization completeness evidence is conflicting; no usable single canonical target."
				} else {
					observedSummary = "Canonical normalization is incomplete or declarations cannot be normalized; no usable single canonical target."
				}
			} else {
				if distinctMalformed {
					observedSummary = fmt.Sprintf("Distinct canonical target count evidence is malformed: %q; no usable single canonical target.", firstDistinctStr)
				} else if distinctConflict {
					observedSummary = "Distinct canonical target count evidence is conflicting; no usable single canonical target."
				} else {
					observedSummary = fmt.Sprintf("Page has %d distinct canonical targets; rule applies only to single canonical targets.", distinctCount)
				}
			}
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
			if len(completeObs) > 0 {
				evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
			}
			if len(distinctObs) > 0 {
				evidenceRefs = append(evidenceRefs, buildDistinctRefs(audit.EvidenceRoleContext)...)
			}
			evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
		} else if !targetRefUsable || targetRefMalformed || targetRefConflict {
			// target subject cannot be correlated -> NOT_APPLICABLE
			evalStatus = audit.StatusNotApplicable
			observedSummary = "Canonical target subject cannot be correlated to snapshot (external or unvisited target)."
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildDistinctRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
			if len(targetRefObs) > 0 {
				evidenceRefs = append(evidenceRefs, buildTargetSubjectContextRefs()...)
			}
		} else {
			// Usable single canonical target and target subject correlated!
			evidenceRefs = append(evidenceRefs, buildCountRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildCompleteRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildDistinctRefs(audit.EvidenceRoleContext)...)
			evidenceRefs = append(evidenceRefs, buildNormTargetContextRefs()...)
			evidenceRefs = append(evidenceRefs, buildTargetSubjectContextRefs()...)

			// Target URL identity
			targetURLObs := idx.GetObservations(audit.SubjectURL, targetSubjectRef, "url_identity")
			for _, tu := range targetURLObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:target_url", ruleResultID, tu.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(tu.ObservationID),
					Field:             "target_url",
					ObservedValue:     tu.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}

			// Target HTTP status
			targetStatusObs := idx.GetObservations(audit.SubjectURL, targetSubjectRef, "http_status")
			var (
				targetStatusUsable    bool
				targetStatusConflict  bool
				targetStatusMalformed bool
				targetStatusStr       string
				targetStatusCode      int
			)
			if len(targetStatusObs) > 0 {
				targetStatusStr = strings.TrimSpace(targetStatusObs[0].Value)
				targetStatusUsable = true
				for _, so := range targetStatusObs[1:] {
					if strings.TrimSpace(so.Value) != targetStatusStr {
						targetStatusConflict = true
						targetStatusUsable = false
						break
					}
				}
				if !targetStatusConflict {
					sc, err := strconv.Atoi(targetStatusStr)
					if err != nil || sc <= 0 {
						targetStatusMalformed = true
						targetStatusUsable = false
					} else {
						targetStatusCode = sc
					}
				}
			}

			buildTargetStatusContextRefs := func() []audit.RuleEvidenceRef {
				var refs []audit.RuleEvidenceRef
				for _, so := range targetStatusObs {
					refs = append(refs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:http_status", ruleResultID, so.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(so.ObservationID),
						Field:             "http_status",
						ObservedValue:     so.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
				return refs
			}

			if !targetStatusUsable {
				// target status unavailable/conflicting -> UNKNOWN
				evalStatus = audit.StatusUnknown
				if len(targetStatusObs) == 0 {
					observedSummary = "Canonical target subject exists, but target HTTP status evidence is unavailable."
				} else if targetStatusConflict {
					observedSummary = "Canonical target HTTP status evidence is conflicting."
				} else if targetStatusMalformed {
					observedSummary = fmt.Sprintf("Canonical target HTTP status evidence is malformed: %q.", targetStatusStr)
				}
				evidenceRefs = append(evidenceRefs, buildTargetStatusContextRefs()...)
			} else if targetStatusCode != 200 {
				// target status != 200 -> NOT_APPLICABLE
				evalStatus = audit.StatusNotApplicable
				observedSummary = fmt.Sprintf("Canonical target returns HTTP %d (precondition requires target status 200).", targetStatusCode)
				evidenceRefs = append(evidenceRefs, buildTargetStatusContextRefs()...)
			} else {
				// target status == 200! Precondition met: inspect target effective_noindex
				evidenceRefs = append(evidenceRefs, buildTargetStatusContextRefs()...)

				effObs := idx.GetObservations(audit.SubjectURL, targetSubjectRef, "effective_noindex")
				var (
					effUsable    bool
					effConflict  bool
					effMalformed bool
					firstEffStr  string
					effVal       bool
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

				buildTargetEffectiveRefs := func(role audit.EvidenceRole) []audit.RuleEvidenceRef {
					var refs []audit.RuleEvidenceRef
					for _, eo := range effObs {
						refs = append(refs, audit.RuleEvidenceRef{
							RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:effective_noindex", ruleResultID, eo.ObservationID)),
							RuleResultID:      ruleResultID,
							EvidenceType:      "NORMALIZED_OBSERVATION",
							EvidenceRef:       string(eo.ObservationID),
							Field:             "effective_noindex",
							ObservedValue:     eo.Value,
							Role:              role,
						})
					}
					return refs
				}

				if !effUsable {
					// target status = 200 + effective_noindex unavailable/malformed/conflicting -> UNKNOWN
					evalStatus = audit.StatusUnknown
					if len(effObs) == 0 {
						observedSummary = "Canonical target returns HTTP 200, but target effective_noindex evidence is unavailable."
					} else if effConflict {
						observedSummary = "Canonical target effective_noindex evidence is conflicting."
					} else if effMalformed {
						observedSummary = fmt.Sprintf("Canonical target effective_noindex evidence is malformed: %q.", firstEffStr)
					}
					evidenceRefs = append(evidenceRefs, buildTargetEffectiveRefs(audit.EvidenceRolePrimary)...)
				} else if !effVal {
					// target status = 200 + effective_noindex=false -> PASS
					evalStatus = audit.StatusPass
					observedSummary = "Canonical target returns HTTP 200 and has effective_noindex=false."
					evidenceRefs = append(evidenceRefs, buildTargetEffectiveRefs(audit.EvidenceRolePrimary)...)
				} else {
					// target status = 200 + effective_noindex=true -> FAIL
					evalStatus = audit.StatusFail
					observedSummary = "Canonical target returns HTTP 200 but has effective_noindex=true."
					evidenceRefs = append(evidenceRefs, buildTargetEffectiveRefs(audit.EvidenceRolePrimary)...)
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
			ExpectedSummary: "Canonical target does not have an effective noindex directive.",
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
