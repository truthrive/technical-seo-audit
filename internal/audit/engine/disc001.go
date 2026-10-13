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

const (
	ruleIDDISC001          = "AR-DISC-001"
	expectedSummaryDISC001 = "One or more sitemap URLs discovered, or sitemap presence matches project policy."
)

// evaluateDISC001 implements the execution logic for:
// AR-DISC-001 — Sitemap presence matches project policy (parent check: DISC-001, P1, assisted).
//
// Subject: SubjectSite ("site")
// Unit of evaluation: The website's overall sitemap discovery state.
//
// Preconditions: root/robots sitemap discovery sources evaluated.
// Contract:
// - PASS: one or more sitemap URLs discovered (regardless of policy).
// - FAIL: not used.
// - WARNING: none found and sitemap_expected=true.
// - MANUAL_REVIEW: none found and policy unspecified (or missing).
// - UNKNOWN: discovery could not complete, was not attempted, or has missing/malformed/conflicting evidence.
// - NOT_APPLICABLE: none found and sitemap_expected=false.
//
// Non-negotiable invariants:
// MISSING POLICY != FALSE POLICY
// ZERO SITEMAPS FOUND != DISCOVERY COMPLETE
// DISCOVERY NOT ATTEMPTED != ZERO SITEMAPS DISCOVERED
// EXPLICIT POLICY != CRAWLER OBSERVATION
// POLICY UNSPECIFIED != SITEMAP NOT EXPECTED
// OBSERVATION != CONCLUSION
func evaluateDISC001(
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

	siteRefs := idx.SubjectRefs(audit.SubjectSite)
	if len(siteRefs) == 0 {
		return []audit.RuleResult{}, nil
	}

	results := make([]audit.RuleResult, 0, len(siteRefs))

	for _, subjectRef := range siteRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// 1. Validate SubjectSite identity
		if subjectRef != "site" {
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     audit.SubjectSite,
				SubjectRef:      subjectRef,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeSitewide,
				ObservedSummary: fmt.Sprintf("Invalid SubjectSite reference %q; expected %q.", subjectRef, "site"),
				ExpectedSummary: expectedSummaryDISC001,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    nil,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// Evidence reference collection
		var rawEvidenceRefs []audit.RuleEvidenceRef

		addObsRef := func(obs audit.NormalizedObservation, role audit.EvidenceRole) {
			rawEvidenceRefs = append(rawEvidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, obs.ObservationID, obs.Field)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             obs.Field,
				ObservedValue:     obs.Value,
				Role:              role,
			})
		}

		// 2. Resolve sitemaps_found_count
		countObs := idx.GetObservations(audit.SubjectSite, subjectRef, "sitemaps_found_count")
		for _, co := range countObs {
			addObsRef(co, audit.EvidenceRolePrimary)
		}

		var (
			countMissing   bool
			countMalformed bool
			countConflict  bool
			countNegative  bool
			sitemapsFound  int
		)
		if len(countObs) == 0 {
			countMissing = true
		} else {
			firstVal := strings.TrimSpace(countObs[0].Value)
			for _, co := range countObs[1:] {
				if strings.TrimSpace(co.Value) != firstVal {
					countConflict = true
					break
				}
			}
			if !countConflict {
				parsed, err := strconv.Atoi(firstVal)
				if err != nil {
					countMalformed = true
				} else if parsed < 0 {
					countNegative = true
				} else {
					sitemapsFound = parsed
				}
			}
		}

		// 3. Resolve sitemap_discovery_complete
		compObs := idx.GetObservations(audit.SubjectSite, subjectRef, "sitemap_discovery_complete")
		for _, co := range compObs {
			addObsRef(co, audit.EvidenceRoleSupporting)
		}

		var (
			compMissing          bool
			compMalformed        bool
			compConflict         bool
			obsDiscoveryComplete bool
		)
		if len(compObs) == 0 {
			compMissing = true
		} else {
			firstVal := strings.TrimSpace(compObs[0].Value)
			for _, co := range compObs[1:] {
				if strings.TrimSpace(co.Value) != firstVal {
					compConflict = true
					break
				}
			}
			if !compConflict {
				if firstVal == "true" {
					obsDiscoveryComplete = true
				} else if firstVal == "false" {
					obsDiscoveryComplete = false
				} else {
					compMalformed = true
				}
			}
		}

		// 4. Resolve sitemap_discovery_status
		statusObs := idx.GetObservations(audit.SubjectSite, subjectRef, "sitemap_discovery_status")
		for _, so := range statusObs {
			addObsRef(so, audit.EvidenceRoleContext)
		}

		var (
			statusConflict bool
			statusVal      string
		)
		if len(statusObs) > 0 {
			statusVal = strings.TrimSpace(statusObs[0].Value)
			for _, so := range statusObs[1:] {
				if strings.TrimSpace(so.Value) != statusVal {
					statusConflict = true
					break
				}
			}
		}

		// 5. Resolve discovery caps
		depthCappedObs := idx.GetObservations(audit.SubjectSite, subjectRef, "sitemap_depth_capped")
		urlsCappedObs := idx.GetObservations(audit.SubjectSite, subjectRef, "sitemap_urls_capped")
		byteCappedObs := idx.GetObservations(audit.SubjectSite, subjectRef, "sitemap_byte_capped")

		for _, o := range depthCappedObs {
			addObsRef(o, audit.EvidenceRoleContext)
		}
		for _, o := range urlsCappedObs {
			addObsRef(o, audit.EvidenceRoleContext)
		}
		for _, o := range byteCappedObs {
			addObsRef(o, audit.EvidenceRoleContext)
		}

		isCapped := func(obs []audit.NormalizedObservation) bool {
			for _, o := range obs {
				if strings.TrimSpace(o.Value) == "true" {
					return true
				}
			}
			return false
		}
		discoveryCapped := isCapped(depthCappedObs) || isCapped(urlsCappedObs) || isCapped(byteCappedObs)

		// 6. Contradiction checks
		snapshotContradiction := (!compMissing && !compConflict && !compMalformed && (obsDiscoveryComplete != snapshot.SitemapDiscoveryComplete))

		statusContradiction := false
		if !compMissing && !compConflict && !compMalformed && obsDiscoveryComplete {
			if discoveryCapped {
				statusContradiction = true
			}
			if statusVal == "NOT_ATTEMPTED" || statusVal == "ATTEMPTED_INCOMPLETE" || (statusVal != "" && statusVal != "COMPLETED") {
				statusContradiction = true
			}
		}

		// 7. Resolve explicit policy
		var (
			policyAssignment *audit.ProjectPolicyAssignment
			hasPolicy        bool
		)
		if policies != nil {
			policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeSite, "site", audit.PolicyKeySitemapExpected)
		}

		addPolicyRef := func(role audit.EvidenceRole) {
			if !hasPolicy || policyAssignment == nil {
				return
			}
			rawEvidenceRefs = append(rawEvidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_expected", ruleResultID, policyAssignment.PolicyAssignmentID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "PROJECT_POLICY_ASSIGNMENT",
				EvidenceRef:       string(policyAssignment.PolicyAssignmentID),
				Field:             "sitemap_expected",
				ObservedValue:     policyAssignment.PolicyValue,
				Role:              role,
			})
		}

		// 8. Determine evaluation status and observed summary
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		// Step A: Check for unknown/error conditions on discovery evidence
		if countMissing {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap discovery count observation is missing (sitemaps_found_count unavailable)."
			addPolicyRef(audit.EvidenceRoleContext)
		} else if countConflict {
			evalStatus = audit.StatusUnknown
			observedSummary = "Conflicting sitemap discovery counts observed on SubjectSite."
			addPolicyRef(audit.EvidenceRoleContext)
		} else if countMalformed {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap discovery count observation is malformed (cannot parse integer value)."
			addPolicyRef(audit.EvidenceRoleContext)
		} else if countNegative {
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("Sitemap discovery count observation is negative (%d).", sitemapsFound)
			addPolicyRef(audit.EvidenceRoleContext)
		} else if compMissing {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap discovery completion observation is missing (sitemap_discovery_complete unavailable)."
			addPolicyRef(audit.EvidenceRoleContext)
		} else if compConflict {
			evalStatus = audit.StatusUnknown
			observedSummary = "Conflicting sitemap discovery completion observations observed on SubjectSite."
			addPolicyRef(audit.EvidenceRoleContext)
		} else if compMalformed {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap discovery completion observation is malformed (expected 'true' or 'false')."
			addPolicyRef(audit.EvidenceRoleContext)
		} else if snapshotContradiction {
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("Contradictory discovery completion evidence: observation sitemap_discovery_complete=%v conflicts with snapshot.SitemapDiscoveryComplete=%v.", obsDiscoveryComplete, snapshot.SitemapDiscoveryComplete)
			addPolicyRef(audit.EvidenceRoleContext)
		} else if statusConflict {
			evalStatus = audit.StatusUnknown
			observedSummary = "Conflicting sitemap discovery status observations observed on SubjectSite."
			addPolicyRef(audit.EvidenceRoleContext)
		} else if statusContradiction {
			evalStatus = audit.StatusUnknown
			observedSummary = "Contradictory discovery status evidence: sitemap_discovery_complete is true but discovery was capped or status is not COMPLETED."
			addPolicyRef(audit.EvidenceRoleContext)
		} else if !obsDiscoveryComplete || statusVal == "NOT_ATTEMPTED" || statusVal == "ATTEMPTED_INCOMPLETE" || statusVal == "DISABLED" {
			evalStatus = audit.StatusUnknown
			if statusVal == "NOT_ATTEMPTED" {
				observedSummary = "Sitemap discovery was not attempted (status=NOT_ATTEMPTED); sitemap presence cannot be determined."
			} else if statusVal == "DISABLED" {
				observedSummary = "Sitemap discovery was disabled (status=DISABLED); sitemap presence cannot be determined."
			} else if statusVal == "ATTEMPTED_INCOMPLETE" || discoveryCapped {
				observedSummary = "Sitemap discovery could not complete (status=ATTEMPTED_INCOMPLETE or limit capped); sitemap presence cannot be determined."
			} else {
				observedSummary = "Sitemap discovery could not complete (sitemap_discovery_complete=false); sitemap presence cannot be determined."
			}
			addPolicyRef(audit.EvidenceRoleContext)
		} else {
			// Step B: Discovery completed cleanly without caps or contradictions
			if sitemapsFound > 0 {
				evalStatus = audit.StatusPass

				// Collect document-level discovery sources if available
				var docSources []string
				seenDocSrc := make(map[string]bool)
				for _, smRef := range idx.SubjectRefs(audit.SubjectSitemap) {
					for _, srcObs := range idx.GetObservations(audit.SubjectSitemap, smRef, "discovery_sources") {
						addObsRef(srcObs, audit.EvidenceRoleContext)
						for _, s := range strings.Split(srcObs.Value, ",") {
							sTrim := strings.TrimSpace(s)
							if sTrim != "" && !seenDocSrc[sTrim] {
								seenDocSrc[sTrim] = true
								docSources = append(docSources, sTrim)
							}
						}
					}
					for _, srcObs := range idx.GetObservations(audit.SubjectSitemap, smRef, "discovery_source") {
						addObsRef(srcObs, audit.EvidenceRoleContext)
						sTrim := strings.TrimSpace(srcObs.Value)
						if sTrim != "" && !seenDocSrc[sTrim] {
							seenDocSrc[sTrim] = true
							docSources = append(docSources, sTrim)
						}
					}
				}
				sort.Strings(docSources)

				if len(docSources) > 0 {
					observedSummary = fmt.Sprintf("One or more sitemap URLs discovered (%d sitemap document(s) discovered via %s).", sitemapsFound, strings.Join(docSources, ", "))
				} else {
					observedSummary = fmt.Sprintf("One or more sitemap URLs discovered (%d sitemap document(s) discovered).", sitemapsFound)
				}
				addPolicyRef(audit.EvidenceRoleContext)
			} else {
				// sitemapsFound == 0: policy governs verdict
				if hasPolicy && policyAssignment != nil && policyAssignment.PolicyValue == audit.PolicyValueTrue {
					evalStatus = audit.StatusWarning
					observedSummary = "No sitemaps discovered, but project policy explicitly expects a sitemap (sitemap_expected=true)."
					addPolicyRef(audit.EvidenceRolePrimary)
				} else if hasPolicy && policyAssignment != nil && policyAssignment.PolicyValue == audit.PolicyValueFalse {
					evalStatus = audit.StatusNotApplicable
					observedSummary = "No sitemaps discovered and project policy explicitly specifies that no sitemap is expected (sitemap_expected=false)."
					addPolicyRef(audit.EvidenceRolePrimary)
				} else if hasPolicy && policyAssignment != nil && policyAssignment.PolicyValue == audit.PolicyValueUnspecified {
					evalStatus = audit.StatusManualReview
					observedSummary = "No sitemaps discovered and project policy is explicitly unspecified (sitemap_expected=unspecified); review whether this website requires an XML sitemap."
					addPolicyRef(audit.EvidenceRolePrimary)
				} else {
					// Policy missing / not supplied
					evalStatus = audit.StatusManualReview
					observedSummary = "No sitemaps discovered and project policy is unspecified (no sitemap_expected assignment); review whether this website requires an XML sitemap."
				}
			}
		}

		// 9. Deduplicate and deterministically sort EvidenceRefs
		sort.Slice(rawEvidenceRefs, func(i, j int) bool {
			return rawEvidenceRefs[i].RuleEvidenceRefID < rawEvidenceRefs[j].RuleEvidenceRefID
		})
		var dedupedRefs []audit.RuleEvidenceRef
		seenRefIDs := make(map[audit.RuleEvidenceRefID]bool)
		for _, r := range rawEvidenceRefs {
			if !seenRefIDs[r.RuleEvidenceRefID] {
				seenRefIDs[r.RuleEvidenceRefID] = true
				dedupedRefs = append(dedupedRefs, r)
			}
		}

		// 10. Construct RuleResult
		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     audit.SubjectSite,
			SubjectRef:      subjectRef,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeSitewide,
			ObservedSummary: observedSummary,
			ExpectedSummary: expectedSummaryDISC001,
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
