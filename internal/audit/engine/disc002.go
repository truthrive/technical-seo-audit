package engine

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDDISC002 = "AR-DISC-002"

// evaluateDISC002 implements the typed execution logic for:
// AR-DISC-002 — Sitemap document returns 200 (parent check: DISC-002, P1, deterministic).
//
// Subject: SubjectSitemap (sm:<audit_run_id>:<sitemap_id>)
// Unit of evaluation: The individual sitemap document.
//
// Contract:
// - PASS: Terminal HTTP response is 200 (direct 200, 301->200, 302->200, multi-hop->200).
// - FAIL: Terminal HTTP response is non-200 (direct 404/500/403/429/201/204/304, or redirect chain ending in non-200).
// - UNKNOWN: Request fails before terminal HTTP response (network/DNS/timeout/TLS failure, redirect loop/exhaustion,
//            missing/zero/malformed terminal status, missing sitemap URL identity, run identity mismatch, or contradictory evidence).
// - WARNING: not used.
// - MANUAL_REVIEW: not used.
// - NOT_APPLICABLE: not used.
//
// Decision DEC-V1.8E-001:
// Terminal HTTP response is authoritative. Initial status, redirect count, final destination URL,
// and fetch error remain contextual diagnostic evidence.
func evaluateDISC002(
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

	sitemapRefs := idx.SubjectRefs(audit.SubjectSitemap)
	if len(sitemapRefs) == 0 {
		return nil, nil
	}

	results := make([]audit.RuleResult, 0, len(sitemapRefs))
	expectedRunPrefix := fmt.Sprintf("sm:%s:", snapshot.AuditRunID)

	for _, subjectRef := range sitemapRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// 1. Validate subject run identity
		runIdentityMismatch := !strings.HasPrefix(subjectRef, expectedRunPrefix)

		// 2. Resolve sitemap_url
		urlObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_url")
		var (
			urlMissing  bool
			urlEmpty    bool
			urlConflict bool
		)
		if len(urlObs) == 0 {
			urlMissing = true
		} else {
			firstURL := strings.TrimSpace(urlObs[0].Value)
			if firstURL == "" {
				urlEmpty = true
			}
			for _, uo := range urlObs[1:] {
				if strings.TrimSpace(uo.Value) != firstURL {
					urlConflict = true
					break
				}
			}
		}

		// 3. Resolve final_http_status
		finalStatusObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "final_http_status")
		var (
			finalStatusMissing   bool
			finalStatusConflict  bool
			finalStatusMalformed bool
			finalStatusZero      bool
			finalStatusCode      int
		)
		if len(finalStatusObs) == 0 {
			finalStatusMissing = true
		} else {
			firstFS := strings.TrimSpace(finalStatusObs[0].Value)
			for _, fso := range finalStatusObs[1:] {
				if strings.TrimSpace(fso.Value) != firstFS {
					finalStatusConflict = true
					break
				}
			}
			if !finalStatusConflict {
				code, err := strconv.Atoi(firstFS)
				if err != nil {
					finalStatusMalformed = true
				} else if code <= 0 {
					finalStatusZero = true
					finalStatusCode = code
				} else {
					finalStatusCode = code
				}
			}
		}

		// 4. Resolve sitemap_fetch_complete
		fetchCompleteObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_fetch_complete")
		var (
			fetchCompleteMissing   bool
			fetchCompleteConflict  bool
			fetchCompleteMalformed bool
			fetchCompleteVal       bool
			hasFetchComplete       bool
		)
		if len(fetchCompleteObs) == 0 {
			fetchCompleteMissing = true
		} else {
			firstFC := strings.TrimSpace(fetchCompleteObs[0].Value)
			for _, fco := range fetchCompleteObs[1:] {
				if strings.TrimSpace(fco.Value) != firstFC {
					fetchCompleteConflict = true
					break
				}
			}
			if !fetchCompleteConflict {
				b, err := strconv.ParseBool(firstFC)
				if err != nil {
					fetchCompleteMalformed = true
				} else {
					fetchCompleteVal = b
					hasFetchComplete = true
				}
			}
		}

		// 5. Resolve initial_http_status (optional context)
		initStatusObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "initial_http_status")
		var (
			initStatusConflict bool
			initStatusCode     int
			hasInitStatus      bool
		)
		if len(initStatusObs) > 0 {
			firstIS := strings.TrimSpace(initStatusObs[0].Value)
			for _, iso := range initStatusObs[1:] {
				if strings.TrimSpace(iso.Value) != firstIS {
					initStatusConflict = true
					break
				}
			}
			if !initStatusConflict {
				if code, err := strconv.Atoi(firstIS); err == nil && code > 0 {
					initStatusCode = code
					hasInitStatus = true
				}
			}
		}

		// 6. Resolve sitemap_redirect_hops (optional context)
		hopsObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_redirect_hops")
		var (
			hopsConflict bool
			redirectHops int
		)
		if len(hopsObs) > 0 {
			firstHops := strings.TrimSpace(hopsObs[0].Value)
			for _, ho := range hopsObs[1:] {
				if strings.TrimSpace(ho.Value) != firstHops {
					hopsConflict = true
					break
				}
			}
			if !hopsConflict {
				if h, err := strconv.Atoi(firstHops); err == nil && h >= 0 {
					redirectHops = h
				}
			}
		}

		// 7. Resolve sitemap_final_url (optional context)
		finalURLObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_final_url")
		var finalURLConflict bool
		if len(finalURLObs) > 0 {
			firstFURL := strings.TrimSpace(finalURLObs[0].Value)
			for _, fuo := range finalURLObs[1:] {
				if strings.TrimSpace(fuo.Value) != firstFURL {
					finalURLConflict = true
					break
				}
			}
		}

		// 8. Resolve sitemap_fetch_error (optional context)
		fetchErrObs := idx.GetObservations(audit.SubjectSitemap, subjectRef, "sitemap_fetch_error")
		var (
			fetchErrConflict bool
			fetchErrVal      string
		)
		if len(fetchErrObs) > 0 {
			firstFE := strings.TrimSpace(fetchErrObs[0].Value)
			fetchErrVal = firstFE
			for _, feo := range fetchErrObs[1:] {
				if strings.TrimSpace(feo.Value) != firstFE {
					fetchErrConflict = true
					break
				}
			}
		}

		// Contradiction and consistency checks
		hasConflict := urlConflict || finalStatusConflict || fetchCompleteConflict ||
			initStatusConflict || hopsConflict || finalURLConflict || fetchErrConflict

		statusContradiction := (hasFetchComplete && !fetchCompleteVal && finalStatusCode > 0) ||
			(hasFetchComplete && fetchCompleteVal && (finalStatusZero || finalStatusCode == 0)) ||
			(finalStatusCode > 0 && fetchErrVal != "") ||
			(hasFetchComplete && fetchCompleteVal && fetchErrVal != "")

		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		if runIdentityMismatch {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap subject does not match audit run identity."
		} else if hasConflict {
			evalStatus = audit.StatusUnknown
			observedSummary = "Conflicting observations observed for sitemap."
		} else if urlMissing || urlEmpty {
			evalStatus = audit.StatusUnknown
			observedSummary = "Required sitemap URL identity evidence is unavailable."
		} else if finalStatusMissing {
			evalStatus = audit.StatusUnknown
			observedSummary = "Required terminal HTTP status evidence is unavailable."
		} else if finalStatusMalformed {
			evalStatus = audit.StatusUnknown
			observedSummary = "Required terminal HTTP status evidence is malformed."
		} else if statusContradiction {
			evalStatus = audit.StatusUnknown
			if hasFetchComplete && !fetchCompleteVal && finalStatusCode > 0 {
				observedSummary = fmt.Sprintf("Contradictory evidence: sitemap returned HTTP status %d but sitemap_fetch_complete is false.", finalStatusCode)
			} else if hasFetchComplete && fetchCompleteVal && (finalStatusZero || finalStatusCode == 0) {
				observedSummary = "Contradictory evidence: sitemap_fetch_complete is true but terminal HTTP status is 0."
			} else if finalStatusCode > 0 && fetchErrVal != "" {
				observedSummary = fmt.Sprintf("Contradictory evidence: sitemap returned HTTP %d but reported fetch error %q.", finalStatusCode, fetchErrVal)
			} else {
				observedSummary = fmt.Sprintf("Contradictory evidence: sitemap_fetch_complete is true but reported fetch error %q.", fetchErrVal)
			}
		} else if fetchCompleteMissing {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap fetch completion evidence is unavailable."
		} else if fetchCompleteMalformed {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap fetch completion evidence is malformed."
		} else if finalStatusZero || finalStatusCode == 0 || !fetchCompleteVal {
			evalStatus = audit.StatusUnknown
			if fetchErrVal != "" {
				observedSummary = fmt.Sprintf("Sitemap document fetch failed before receiving an HTTP response (%s).", fetchErrVal)
			} else {
				observedSummary = "Sitemap document fetch did not produce a trustworthy terminal HTTP response."
			}
		} else if finalStatusCode == http.StatusOK {
			evalStatus = audit.StatusPass
			if redirectHops > 0 {
				if hasInitStatus && initStatusCode > 0 {
					observedSummary = fmt.Sprintf("Sitemap document returned terminal HTTP 200 after redirecting from initial HTTP %d.", initStatusCode)
				} else {
					observedSummary = fmt.Sprintf("Sitemap document returned terminal HTTP 200 after %d redirect hop(s).", redirectHops)
				}
			} else {
				observedSummary = "Sitemap document returned terminal HTTP 200 without redirects."
			}
		} else {
			evalStatus = audit.StatusFail
			if redirectHops > 0 {
				if hasInitStatus && initStatusCode > 0 {
					observedSummary = fmt.Sprintf("Sitemap document returned terminal HTTP %d after redirecting from initial HTTP %d.", finalStatusCode, initStatusCode)
				} else {
					observedSummary = fmt.Sprintf("Sitemap document returned terminal HTTP %d after %d redirect hop(s).", finalStatusCode, redirectHops)
				}
			} else {
				observedSummary = fmt.Sprintf("Sitemap document returned terminal HTTP %d.", finalStatusCode)
			}
		}

		// Assemble traceable evidence references
		var evidenceRefs []audit.RuleEvidenceRef

		// Primary: final_http_status
		for _, obs := range finalStatusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:final_http_status", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "final_http_status",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		// Context: sitemap_url
		for _, obs := range urlObs {
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

		// Context: sitemap_fetch_complete
		for _, obs := range fetchCompleteObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_fetch_complete", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_fetch_complete",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// Context: initial_http_status
		for _, obs := range initStatusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:initial_http_status", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "initial_http_status",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// Context: sitemap_redirect_hops
		for _, obs := range hopsObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_redirect_hops", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_redirect_hops",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// Context: sitemap_final_url
		for _, obs := range finalURLObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_final_url", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_final_url",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// Context: sitemap_fetch_error
		for _, obs := range fetchErrObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_fetch_error", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_fetch_error",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
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

		// PASS/FAIL integrity guard:
		// Requires valid sitemap subject, valid sitemap URL, trustworthy terminal HTTP response,
		// and all required primary/context evidence refs.
		if evalStatus == audit.StatusPass || evalStatus == audit.StatusFail {
			var (
				hasFinalStatusPrimary   bool
				hasURLContext           bool
				hasFetchCompleteContext bool
			)
			for _, ref := range dedupedRefs {
				if ref.Field == "final_http_status" && ref.Role == audit.EvidenceRolePrimary {
					hasFinalStatusPrimary = true
				}
				if ref.Field == "sitemap_url" && ref.Role == audit.EvidenceRoleContext {
					hasURLContext = true
				}
				if ref.Field == "sitemap_fetch_complete" && ref.Role == audit.EvidenceRoleContext {
					hasFetchCompleteContext = true
				}
			}
			if !hasFinalStatusPrimary || !hasURLContext || !hasFetchCompleteContext ||
				urlMissing || urlEmpty || urlConflict || finalStatusMissing || finalStatusMalformed ||
				finalStatusZero || finalStatusConflict || !hasFetchComplete || !fetchCompleteVal ||
				statusContradiction || hasConflict || runIdentityMismatch || fetchErrVal != "" {
				evalStatus = audit.StatusUnknown
				observedSummary = "Sitemap document fetch did not produce trustworthy terminal HTTP response evidence."
			}
		}

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     audit.SubjectSitemap,
			SubjectRef:      subjectRef,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeSiteConfiguration,
			ObservedSummary: observedSummary,
			ExpectedSummary: "Sitemap document returns HTTP 200.",
			EvaluatedAt:     evalTime,
			EvidenceRefs:    dedupedRefs,
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
		}

		results = append(results, rr)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].SubjectRef < results[j].SubjectRef
	})

	return results, nil
}
