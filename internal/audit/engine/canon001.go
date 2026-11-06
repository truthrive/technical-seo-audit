package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const (
	ruleIDCANON001          = "AR-CANON-001"
	expectedSummaryCANON001 = "Origin variants permanently redirect (301/308) to the preferred origin."
)

// evaluateCANON001 implements the typed execution logic for:
// AR-CANON-001 — Origin variants normalize to preferred origin (parent check: CANON-001, P1, deterministic).
//
// Contract:
// - PASS: Non-preferred variant permanently redirects via 301/308 to preferred origin.
// - WARNING: Preferred origin reached only through temporary redirect (302/307).
// - FAIL: Non-preferred variant remains 2xx or resolves to a different origin.
// - NOT_APPLICABLE: Evaluated origin already preferred.
// - UNKNOWN: Preferred origin/variant cannot be evaluated.
//
// Required inputs:
// - preferred_origin
// - origin_variant
// - variant_fetch_status
// - variant_final_url
// - redirect_hops
func evaluateCANON001(
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

	// 1. Resolve preferred_origin from PolicyIndex or EvidenceSnapshot
	var (
		preferredOrigin        string
		preferredOriginPolicy  *audit.ProjectPolicyAssignment
		preferredOriginObsList []audit.NormalizedObservation
	)

	if pol, ok := policies.Get(audit.PolicyScopeSite, "site", audit.PolicyKeyPreferredOrigin); ok {
		preferredOrigin = strings.TrimSpace(pol.PolicyValue)
		preferredOriginPolicy = pol
	} else {
		// Look for preferred_origin observation on SubjectSite ("site")
		preferredOriginObsList = idx.GetObservations(audit.SubjectSite, "site", "preferred_origin")
		if len(preferredOriginObsList) > 0 {
			preferredOrigin = strings.TrimSpace(preferredOriginObsList[0].Value)
		}
	}

	// 2. Discover target subjects with origin_variant observations
	type subjectItem struct {
		st  audit.EvaluationSubjectType
		ref string
	}
	var targetSubjects []subjectItem

	subjectTypeCandidates := []audit.EvaluationSubjectType{
		audit.SubjectSiteConfiguration,
		audit.SubjectProbe,
		audit.SubjectURL,
		audit.SubjectSite,
	}

	for _, st := range subjectTypeCandidates {
		for _, ref := range idx.SubjectRefs(st) {
			obs := idx.GetObservations(st, ref, "origin_variant")
			if len(obs) > 0 {
				targetSubjects = append(targetSubjects, subjectItem{st: st, ref: ref})
			}
		}
	}

	// Fallback: If no subjects have explicit origin_variant observations, but SubjectSite exists
	if len(targetSubjects) == 0 {
		siteRefs := idx.SubjectRefs(audit.SubjectSite)
		if len(siteRefs) > 0 {
			targetSubjects = append(targetSubjects, subjectItem{st: audit.SubjectSite, ref: siteRefs[0]})
		} else {
			return []audit.RuleResult{}, nil
		}
	}

	results := make([]audit.RuleResult, 0, len(targetSubjects))

	for _, ts := range targetSubjects {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, ts.ref))

		var evidenceRefs []audit.RuleEvidenceRef
		addEvidenceRef := func(obs audit.NormalizedObservation, field string, role audit.EvidenceRole) {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, obs.ObservationID, field)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             field,
				ObservedValue:     obs.Value,
				Role:              role,
			})
		}

		// Add preferred_origin evidence
		if len(preferredOriginObsList) > 0 {
			for _, o := range preferredOriginObsList {
				addEvidenceRef(o, "preferred_origin", audit.EvidenceRoleContext)
			}
		}

		// Check preferred origin precondition
		if preferredOrigin == "" {
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     ts.st,
				SubjectRef:      ts.ref,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeSiteConfiguration,
				ObservedSummary: "Explicit preferred origin policy is not specified or unavailable.",
				ExpectedSummary: expectedSummaryCANON001,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", ts.ref, err)
			}
			results = append(results, rr)
			continue
		}

		// 1. origin_variant
		variantObs := idx.GetObservations(ts.st, ts.ref, "origin_variant")
		if len(variantObs) == 0 && ts.st == audit.SubjectURL {
			variantObs = idx.GetObservations(ts.st, ts.ref, "url_identity")
		}
		var (
			variantURL      string
			variantConflict bool
		)
		if len(variantObs) > 0 {
			variantURL = strings.TrimSpace(variantObs[0].Value)
			for _, o := range variantObs {
				addEvidenceRef(o, "origin_variant", audit.EvidenceRoleContext)
				if strings.TrimSpace(o.Value) != variantURL {
					variantConflict = true
				}
			}
		}

		if variantConflict || variantURL == "" {
			obsSummary := "Origin variant evidence is unavailable for evaluation."
			if variantConflict {
				obsSummary = "Conflicting origin variant observations recorded."
			}
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     ts.st,
				SubjectRef:      ts.ref,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeSiteConfiguration,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryCANON001,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", ts.ref, err)
			}
			results = append(results, rr)
			continue
		}

		normPref := normalizeOriginStr(preferredOrigin)
		normVariant := normalizeOriginStr(variantURL)

		// 2. Precondition: Is evaluated origin already preferred?
		if normVariant == normPref {
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     ts.st,
				SubjectRef:      ts.ref,
				Status:          audit.StatusNotApplicable,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeSiteConfiguration,
				ObservedSummary: fmt.Sprintf("Evaluated origin variant %q is already the preferred origin.", variantURL),
				ExpectedSummary: expectedSummaryCANON001,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", ts.ref, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. variant_fetch_status
		statusObs := idx.GetObservations(ts.st, ts.ref, "variant_fetch_status")
		if len(statusObs) == 0 {
			statusObs = idx.GetObservations(ts.st, ts.ref, "http_status")
		}
		var (
			variantStatus  int
			statusRecorded bool
			statusConflict bool
		)
		if len(statusObs) > 0 {
			firstStatus := strings.TrimSpace(statusObs[0].Value)
			statusRecorded = true
			if sc, err := strconv.Atoi(firstStatus); err == nil {
				variantStatus = sc
			}
			for _, o := range statusObs {
				addEvidenceRef(o, "variant_fetch_status", audit.EvidenceRolePrimary)
				if strings.TrimSpace(o.Value) != firstStatus {
					statusConflict = true
				}
			}
		}

		if statusConflict || !statusRecorded || variantStatus <= 0 {
			obsSummary := fmt.Sprintf("Origin variant %q did not produce a usable HTTP response.", variantURL)
			if statusConflict {
				obsSummary = "Conflicting HTTP status observations recorded for origin variant."
			}
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     ts.st,
				SubjectRef:      ts.ref,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeSiteConfiguration,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryCANON001,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", ts.ref, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. variant_final_url & redirect_hops
		finalURLObs := idx.GetObservations(ts.st, ts.ref, "variant_final_url")
		if len(finalURLObs) == 0 {
			finalURLObs = idx.GetObservations(ts.st, ts.ref, "redirect_final_url")
		}
		var finalURL string
		if len(finalURLObs) > 0 {
			finalURL = strings.TrimSpace(finalURLObs[0].Value)
			for _, o := range finalURLObs {
				addEvidenceRef(o, "variant_final_url", audit.EvidenceRolePrimary)
			}
		} else {
			finalURL = variantURL
		}

		// Redirect hops
		hopObs := idx.GetObservations(ts.st, ts.ref, "redirect_hop")
		var hopStatuses []int
		for _, o := range hopObs {
			addEvidenceRef(o, "redirect_hops", audit.EvidenceRoleSupporting)
			var rh redirectHopPayload
			if err := json.Unmarshal([]byte(o.Value), &rh); err == nil && rh.Status > 0 {
				hopStatuses = append(hopStatuses, rh.Status)
			}
		}

		// If no hop observation objects, check initial status or redirect_hop_count
		if len(hopStatuses) == 0 && (variantStatus == 301 || variantStatus == 308 || variantStatus == 302 || variantStatus == 307) {
			hopStatuses = append(hopStatuses, variantStatus)
		}

		normFinalOrigin := normalizeOriginStr(finalURL)

		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		if normFinalOrigin == normPref {
			// Destination matches preferred origin! Check if redirect was permanent or temporary
			hasTemporary := false
			allPermanent := len(hopStatuses) > 0
			for _, s := range hopStatuses {
				if s == 302 || s == 307 {
					hasTemporary = true
					allPermanent = false
				} else if s != 301 && s != 308 {
					allPermanent = false
				}
			}

			if allPermanent {
				evalStatus = audit.StatusPass
				observedSummary = fmt.Sprintf("Origin variant %q permanently redirects (301/308) to preferred origin %q.", variantURL, preferredOrigin)
			} else if hasTemporary {
				evalStatus = audit.StatusWarning
				observedSummary = fmt.Sprintf("Preferred origin %q reached from %q only through temporary redirect (302/307).", preferredOrigin, variantURL)
			} else if len(hopStatuses) == 0 && variantStatus >= 200 && variantStatus <= 299 {
				evalStatus = audit.StatusFail
				observedSummary = fmt.Sprintf("Origin variant %q returned HTTP %d without redirecting to preferred origin %q.", variantURL, variantStatus, preferredOrigin)
			} else {
				evalStatus = audit.StatusWarning
				observedSummary = fmt.Sprintf("Origin variant %q reached preferred origin %q via mixed or non-standard redirect.", variantURL, preferredOrigin)
			}
		} else {
			// Final origin does NOT match preferred origin (either 2xx on wrong origin, or redirects to different origin)
			evalStatus = audit.StatusFail
			if finalURL == variantURL {
				observedSummary = fmt.Sprintf("Non-preferred origin variant %q remains HTTP %d without redirecting to preferred origin %q.", variantURL, variantStatus, preferredOrigin)
			} else {
				observedSummary = fmt.Sprintf("Non-preferred origin variant %q redirects to %q instead of preferred origin %q.", variantURL, finalURL, preferredOrigin)
			}
		}

		_ = preferredOriginPolicy // preserved for contract traceability

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     ts.st,
			SubjectRef:      ts.ref,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeSiteConfiguration,
			ObservedSummary: observedSummary,
			ExpectedSummary: expectedSummaryCANON001,
			EvaluatedAt:     evalTime,
			EvidenceRefs:    evidenceRefs,
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", ts.ref, err)
		}
		results = append(results, rr)
	}

	return results, nil
}

// normalizeOriginStr normalizes an origin string (scheme://hostname[:port]) for comparison.
func normalizeOriginStr(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(raw)
	}
	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		return scheme + "://" + host + ":" + port
	}
	return scheme + "://" + host
}
