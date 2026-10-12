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

const ruleIDDISC006 = "AR-DISC-006"

// evaluateDISC006 implements the typed execution logic for:
// AR-DISC-006 — Sitemap-listed URL canonicalizes to itself when canonical is declared
// (parent check: DISC-002, P1, deterministic).
//
// Subject: SubjectSitemapEntry (sme:<audit_run_id>:<sitemap_id>:<seq>)
// Unit of evaluation: The individual sitemap entry.
//
// Preconditions: listed URL declares exactly one valid canonical.
// Contract:
// - PASS: normalized canonical equals listed URL.
// - FAIL: normalized canonical points elsewhere.
// - NOT_APPLICABLE: no usable canonical declaration (canonical_count = 0,
//                   or multiple canonical declarations).
// - UNKNOWN: canonical cannot be resolved (normalization incomplete, malformed target,
//            missing target, missing listed URL normalization, conflicting observations,
//            missing canonical count, uncrawled entry, or missing/invalid URL correlation).
// - WARNING: not used.
// - MANUAL_REVIEW: not used.
//
// Core distinctions:
// MISSING != FALSE
// NO CANONICAL != CANONICAL MISMATCH
// CANONICAL DECLARATION != GOOGLE-SELECTED CANONICAL
// URL IDENTITY != HTTP FETCH
// UNRESOLVED CANONICAL != CANONICAL POINTING ELSEWHERE
// OBSERVATION != CONCLUSION
func evaluateDISC006(
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

	entryRefs := idx.SubjectRefs(audit.SubjectSitemapEntry)
	if len(entryRefs) == 0 {
		return []audit.RuleResult{}, nil
	}

	results := make([]audit.RuleResult, 0, len(entryRefs))

	for _, subjectRef := range entryRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// Check subjectRef format and run identity
		expectedSmePrefix := fmt.Sprintf("sme:%s:", snapshot.AuditRunID)
		subjectRefValid := strings.HasPrefix(subjectRef, expectedSmePrefix)

		// 1. Resolve loc_raw (Context)
		locRawObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "loc_raw")
		var (
			locRawUsable   bool
			locRawConflict bool
			locRawVal      string
		)
		if len(locRawObs) > 0 {
			locRawVal = strings.TrimSpace(locRawObs[0].Value)
			if locRawVal != "" {
				locRawUsable = true
				for _, lo := range locRawObs[1:] {
					if strings.TrimSpace(lo.Value) != locRawVal {
						locRawConflict = true
						locRawUsable = false
						break
					}
				}
			}
		}

		// 2. Resolve loc_normalized (Context)
		locNormObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "loc_normalized")
		var (
			locNormUsable   bool
			locNormConflict bool
			locNormVal      string
		)
		if len(locNormObs) > 0 {
			locNormVal = strings.TrimSpace(locNormObs[0].Value)
			if locNormVal != "" {
				locNormUsable = true
				for _, lno := range locNormObs[1:] {
					if strings.TrimSpace(lno.Value) != locNormVal {
						locNormConflict = true
						locNormUsable = false
						break
					}
				}
			}
		}

		// 3. Resolve parent_sitemap_ref (Context)
		parentRefObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "parent_sitemap_ref")
		var (
			parentRefUsable   bool
			parentRefConflict bool
			parentRefVal      string
		)
		expectedSMPrefix := fmt.Sprintf("sm:%s:", snapshot.AuditRunID)
		if len(parentRefObs) > 0 {
			parentRefVal = strings.TrimSpace(parentRefObs[0].Value)
			if parentRefVal != "" && strings.HasPrefix(parentRefVal, expectedSMPrefix) {
				parentRefUsable = true
				for _, po := range parentRefObs[1:] {
					if strings.TrimSpace(po.Value) != parentRefVal {
						parentRefConflict = true
						parentRefUsable = false
						break
					}
				}
			}
		}

		// 4. Resolve sitemap_url from parent sitemap document (Context, optional)
		var (
			smURLObs      []audit.NormalizedObservation
			smURLConflict bool
		)
		if parentRefUsable {
			smURLObs = idx.GetObservations(audit.SubjectSitemap, parentRefVal, "sitemap_url")
			if len(smURLObs) > 1 {
				firstSmURL := strings.TrimSpace(smURLObs[0].Value)
				for _, smo := range smURLObs[1:] {
					if strings.TrimSpace(smo.Value) != firstSmURL {
						smURLConflict = true
						break
					}
				}
			}
		}

		// 5. Resolve url_subject_ref (Context)
		urlSubjObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "url_subject_ref")
		var (
			urlSubjUsable   bool
			urlSubjConflict bool
			urlSubjVal      string
		)
		expectedURLPrefix := fmt.Sprintf("url:%s:", snapshot.AuditRunID)
		if len(urlSubjObs) > 0 {
			urlSubjVal = strings.TrimSpace(urlSubjObs[0].Value)
			if urlSubjVal != "" && strings.HasPrefix(urlSubjVal, expectedURLPrefix) {
				urlSubjUsable = true
				for _, uo := range urlSubjObs[1:] {
					if strings.TrimSpace(uo.Value) != urlSubjVal {
						urlSubjConflict = true
						urlSubjUsable = false
						break
					}
				}
			}
		}

		// 6. Resolve entry_seq (Context, optional)
		entrySeqObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "entry_seq")
		var entrySeqConflict bool
		if len(entrySeqObs) > 1 {
			firstSeq := strings.TrimSpace(entrySeqObs[0].Value)
			for _, eso := range entrySeqObs[1:] {
				if strings.TrimSpace(eso.Value) != firstSeq {
					entrySeqConflict = true
					break
				}
			}
		}

		// 7. Resolve target URL observations on SubjectURL
		var (
			targetURLObs      []audit.NormalizedObservation
			targetURLUsable   bool
			targetURLConflict bool

			countObs       []audit.NormalizedObservation
			countUsable    bool
			countConflict  bool
			countMalformed bool
			firstCountStr  string
			canonCount     int

			completeObs       []audit.NormalizedObservation
			completeUsable    bool
			completeConflict  bool
			completeMalformed bool
			firstCompleteStr  string
			completeVal       bool

			normTargetObs      []audit.NormalizedObservation
			normTargetUsable   bool
			normTargetConflict bool
			firstTargetVal     string

			distinctObs []audit.NormalizedObservation
			canonResObs []audit.NormalizedObservation
		)

		if urlSubjUsable {
			// Resolve url_identity
			targetURLObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "url_identity")
			if len(targetURLObs) > 0 {
				tuVal := strings.TrimSpace(targetURLObs[0].Value)
				if tuVal != "" {
					targetURLUsable = true
					for _, tuo := range targetURLObs[1:] {
						if strings.TrimSpace(tuo.Value) != tuVal {
							targetURLConflict = true
							targetURLUsable = false
							break
						}
					}
				}
			}

			// Resolve canonical_count
			countObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "canonical_count")
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

			// Resolve canonical_normalization_complete
			completeObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "canonical_normalization_complete")
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

			// Resolve canonical_normalized_target
			normTargetObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "canonical_normalized_target")
			if len(normTargetObs) > 0 {
				firstTargetVal = strings.TrimSpace(normTargetObs[0].Value)
				if firstTargetVal != "" {
					normTargetUsable = true
					if canonCount == 1 {
						for _, no := range normTargetObs[1:] {
							if strings.TrimSpace(no.Value) != firstTargetVal {
								normTargetConflict = true
								normTargetUsable = false
								break
							}
						}
					}
				}
			}

			// Resolve optional canonical_distinct_normalized_count
			distinctObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "canonical_distinct_normalized_count")

			// Resolve optional canonical_resolved
			canonResObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "canonical_resolved")
		}

		// 8. Evaluation determination
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
			countRole       audit.EvidenceRole = audit.EvidenceRoleContext
			completeRole    audit.EvidenceRole = audit.EvidenceRoleContext
			normTargetRole  audit.EvidenceRole = audit.EvidenceRoleContext
		)

		hasInternalContradiction := locRawConflict || locNormConflict || parentRefConflict ||
			urlSubjConflict || entrySeqConflict || smURLConflict || targetURLConflict ||
			countConflict

		if !subjectRefValid {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap entry subject reference is invalid or belongs to a foreign audit run."
		} else if hasInternalContradiction {
			evalStatus = audit.StatusUnknown
			observedSummary = "Contradictory observations for sitemap entry or referenced URL subject."
		} else if !locRawUsable {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap entry loc_raw evidence is unavailable."
		} else if !parentRefUsable {
			evalStatus = audit.StatusUnknown
			if parentRefConflict {
				observedSummary = "Sitemap entry parent_sitemap_ref evidence is conflicting."
			} else {
				observedSummary = "Sitemap entry parent_sitemap_ref evidence is unavailable or invalid."
			}
		} else if !urlSubjUsable {
			evalStatus = audit.StatusUnknown
			if urlSubjConflict {
				observedSummary = "Sitemap entry url_subject_ref evidence is conflicting."
			} else {
				observedSummary = "Sitemap entry url_subject_ref is unavailable or invalid (uncrawled entry or url_id=0)."
			}
		} else if !targetURLUsable {
			evalStatus = audit.StatusUnknown
			if targetURLConflict {
				observedSummary = "Correlated URL identity observations are conflicting."
			} else {
				observedSummary = "Correlated URL subject does not exist in evidence snapshot (dangling URL subject reference)."
			}
		} else if !countUsable {
			evalStatus = audit.StatusUnknown
			if len(countObs) == 0 {
				observedSummary = "Required canonical count evidence is unavailable for sitemap-listed URL (canonical extraction unavailable)."
			} else if countConflict {
				observedSummary = "Canonical count evidence is conflicting for sitemap-listed URL."
			} else if countMalformed {
				observedSummary = fmt.Sprintf("Canonical count evidence is malformed for sitemap-listed URL: %q.", firstCountStr)
			}
		} else if canonCount == 0 {
			// Precondition not met: no usable canonical declaration
			evalStatus = audit.StatusNotApplicable
			observedSummary = "Sitemap-listed URL has no canonical declarations (canonical_count = 0)."
			countRole = audit.EvidenceRolePrimary
		} else if canonCount > 1 {
			// Precondition requires exactly one valid canonical declaration
			evalStatus = audit.StatusNotApplicable
			observedSummary = fmt.Sprintf("Sitemap-listed URL declares %d canonical declarations; rule requires exactly one valid canonical declaration.", canonCount)
			countRole = audit.EvidenceRolePrimary
		} else {
			// canonCount == 1: exactly one declaration
			if !completeUsable {
				evalStatus = audit.StatusUnknown
				if len(completeObs) == 0 {
					observedSummary = "One canonical declaration exists, but canonical normalization completeness evidence is unavailable."
				} else if completeConflict {
					observedSummary = "Canonical normalization completeness evidence is conflicting."
				} else if completeMalformed {
					observedSummary = fmt.Sprintf("Canonical normalization completeness evidence is malformed: %q.", firstCompleteStr)
				}
			} else if !completeVal {
				// completeVal == false -> unresolvable or malformed canonical
				evalStatus = audit.StatusUnknown
				observedSummary = "One canonical declaration exists, but canonical cannot be resolved (normalization failed or malformed syntax)."
				countRole = audit.EvidenceRolePrimary
				completeRole = audit.EvidenceRolePrimary
			} else {
				// completeVal == true: normalization succeeded
				if !normTargetUsable {
					evalStatus = audit.StatusUnknown
					if len(normTargetObs) == 0 {
						observedSummary = "Canonical normalization is marked complete, but canonical normalized target is missing."
					} else if normTargetConflict {
						observedSummary = "Canonical normalized target observations are conflicting."
					} else {
						observedSummary = "Canonical normalized target is empty or unusable."
					}
				} else if !locNormUsable {
					evalStatus = audit.StatusUnknown
					if len(locNormObs) == 0 {
						observedSummary = "Sitemap entry loc_normalized evidence is unavailable for comparison."
					} else if locNormConflict {
						observedSummary = "Sitemap entry loc_normalized evidence is conflicting."
					} else {
						observedSummary = "Sitemap entry loc_normalized is empty or unusable."
					}
				} else {
					// Both firstTargetVal and locNormVal are available and valid!
					countRole = audit.EvidenceRolePrimary
					completeRole = audit.EvidenceRolePrimary
					normTargetRole = audit.EvidenceRolePrimary

					if firstTargetVal == locNormVal {
						evalStatus = audit.StatusPass
						observedSummary = fmt.Sprintf("Sitemap-listed URL canonical target %q equals listed URL %q.", firstTargetVal, locNormVal)
					} else {
						evalStatus = audit.StatusFail
						observedSummary = fmt.Sprintf("Sitemap-listed URL canonical target %q does not match listed URL %q.", firstTargetVal, locNormVal)
					}
				}
			}
		}

		// 9. Build EvidenceRefs
		var evidenceRefs []audit.RuleEvidenceRef

		// SubjectSitemapEntry: loc_raw (Context)
		for _, obs := range locRawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:loc_raw", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "loc_raw",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: loc_normalized (Context)
		for _, obs := range locNormObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:loc_normalized", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "loc_normalized",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: parent_sitemap_ref (Context)
		for _, obs := range parentRefObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:parent_sitemap_ref", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "parent_sitemap_ref",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemap: sitemap_url (Context)
		for _, obs := range smURLObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_url", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_url",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: url_subject_ref (Context)
		for _, obs := range urlSubjObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url_subject_ref", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "url_subject_ref",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: entry_seq (Context)
		for _, obs := range entrySeqObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:entry_seq", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "entry_seq",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectURL: url_identity (Context)
		for _, obs := range targetURLObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url_identity", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "url_identity",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectURL: canonical_count
		for _, obs := range countObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_count", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "canonical_count",
				ObservedValue:     obs.Value,
				Role:              countRole,
			})
		}

		// SubjectURL: canonical_normalization_complete
		for _, obs := range completeObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_normalization_complete", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "canonical_normalization_complete",
				ObservedValue:     obs.Value,
				Role:              completeRole,
			})
		}

		// SubjectURL: canonical_normalized_target
		for _, obs := range normTargetObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_normalized_target", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "canonical_normalized_target",
				ObservedValue:     obs.Value,
				Role:              normTargetRole,
			})
		}

		// SubjectURL: canonical_distinct_normalized_count (Context)
		for _, obs := range distinctObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_distinct_normalized_count", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "canonical_distinct_normalized_count",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectURL: canonical_resolved (Context)
		for _, obs := range canonResObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:canonical_resolved", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "canonical_resolved",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// Deduplicate and sort evidence refs deterministically by RuleEvidenceRefID
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

		// Guard: PASS or FAIL requires usable inputs and affirmative evidence
		if evalStatus == audit.StatusPass || evalStatus == audit.StatusFail {
			if !locRawUsable || !locNormUsable || !parentRefUsable || !urlSubjUsable || !targetURLUsable || !countUsable || canonCount != 1 || !completeUsable || !completeVal || !normTargetUsable {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s produced %s without usable inputs", ruleResultID, evalStatus)
			}
			var hasLocRawContext, hasLocNormContext, hasParentRefContext, hasURLSubjContext, hasCountPrimary, hasCompletePrimary, hasNormTargetPrimary bool
			for _, ref := range dedupedRefs {
				if ref.Field == "loc_raw" && ref.Role == audit.EvidenceRoleContext {
					hasLocRawContext = true
				}
				if ref.Field == "loc_normalized" && ref.Role == audit.EvidenceRoleContext {
					hasLocNormContext = true
				}
				if ref.Field == "parent_sitemap_ref" && ref.Role == audit.EvidenceRoleContext {
					hasParentRefContext = true
				}
				if ref.Field == "url_subject_ref" && ref.Role == audit.EvidenceRoleContext {
					hasURLSubjContext = true
				}
				if ref.Field == "canonical_count" && ref.Role == audit.EvidenceRolePrimary {
					hasCountPrimary = true
				}
				if ref.Field == "canonical_normalization_complete" && ref.Role == audit.EvidenceRolePrimary {
					hasCompletePrimary = true
				}
				if ref.Field == "canonical_normalized_target" && ref.Role == audit.EvidenceRolePrimary {
					hasNormTargetPrimary = true
				}
			}
			if !hasLocRawContext || !hasLocNormContext || !hasParentRefContext || !hasURLSubjContext || !hasCountPrimary || !hasCompletePrimary || !hasNormTargetPrimary {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s missing required evidence refs (loc_raw: %v, loc_normalized: %v, parent_sitemap_ref: %v, url_subject_ref: %v, canonical_count: %v, canonical_normalization_complete: %v, canonical_normalized_target: %v)",
					ruleResultID, hasLocRawContext, hasLocNormContext, hasParentRefContext, hasURLSubjContext, hasCountPrimary, hasCompletePrimary, hasNormTargetPrimary)
			}
		}

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     audit.SubjectSitemapEntry,
			SubjectRef:      subjectRef,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeURL,
			ObservedSummary: observedSummary,
			ExpectedSummary: "Sitemap-listed URL canonicalizes to itself when canonical is declared.",
			EvaluatedAt:     evalTime,
			EvidenceRefs:    dedupedRefs,
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
		}

		results = append(results, rr)
	}

	// Deterministic result ordering
	sort.Slice(results, func(i, j int) bool {
		return results[i].RuleResultID < results[j].RuleResultID
	})

	return results, nil
}
