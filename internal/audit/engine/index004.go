package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const (
	ruleIDINDEX004          = "AR-INDEX-004"
	expectedSummaryINDEX004 = "Known-missing probe URL resolves to HTTP 404 or 410."
)

// evaluateINDEX004 implements the typed execution logic for:
// AR-INDEX-004 — Known-missing probe resolves to 404/410 (parent check: INDEX-004, P1, deterministic).
//
// Contract:
// - PASS: Final response is 404 or 410.
// - FAIL: Usable final response is obtained but final status is not 404/410.
// - UNKNOWN: Probe cannot complete or probe evidence is unavailable.
//
// Required inputs:
// - missing_probe_url
// - probe_final_status
// - probe_final_url
func evaluateINDEX004(
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

	type targetItem struct {
		st  audit.EvaluationSubjectType
		ref string
	}
	var targets []targetItem

	// 1. Check SubjectProbe
	for _, ref := range idx.SubjectRefs(audit.SubjectProbe) {
		targets = append(targets, targetItem{st: audit.SubjectProbe, ref: ref})
	}

	// 2. Check SubjectSite with missing_probe_url
	for _, ref := range idx.SubjectRefs(audit.SubjectSite) {
		obs := idx.GetObservations(audit.SubjectSite, ref, "missing_probe_url")
		if len(obs) > 0 {
			targets = append(targets, targetItem{st: audit.SubjectSite, ref: ref})
		}
	}

	// 3. Check SubjectURL with missing_probe_url
	for _, ref := range idx.SubjectRefs(audit.SubjectURL) {
		obs := idx.GetObservations(audit.SubjectURL, ref, "missing_probe_url")
		if len(obs) > 0 {
			targets = append(targets, targetItem{st: audit.SubjectURL, ref: ref})
		}
	}

	// Fallback to SubjectSite ("site") if no target found
	if len(targets) == 0 {
		siteRefs := idx.SubjectRefs(audit.SubjectSite)
		if len(siteRefs) > 0 {
			targets = append(targets, targetItem{st: audit.SubjectSite, ref: siteRefs[0]})
		} else {
			return []audit.RuleResult{}, nil
		}
	}

	results := make([]audit.RuleResult, 0, len(targets))

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, target.ref))

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

		// 1. missing_probe_url / probe_url
		probeURLObs := idx.GetObservations(target.st, target.ref, "missing_probe_url")
		if len(probeURLObs) == 0 {
			probeURLObs = idx.GetObservations(target.st, target.ref, "probe_url")
		}
		if len(probeURLObs) == 0 && target.st == audit.SubjectProbe {
			probeURLObs = idx.GetObservations(target.st, target.ref, "url_identity")
		}
		var (
			probeURL    string
			urlConflict bool
		)
		if len(probeURLObs) > 0 {
			probeURL = strings.TrimSpace(probeURLObs[0].Value)
			for _, o := range probeURLObs {
				addEvidenceRef(o, "probe_url", audit.EvidenceRoleContext)
				if strings.TrimSpace(o.Value) != probeURL {
					urlConflict = true
				}
			}
		}

		// 2. probe_final_status / http_status
		statusObs := idx.GetObservations(target.st, target.ref, "probe_final_status")
		if len(statusObs) == 0 {
			statusObs = idx.GetObservations(target.st, target.ref, "http_status")
		}
		var (
			finalStatus    int
			statusRecorded bool
			statusConflict bool
		)
		if len(statusObs) > 0 {
			firstVal := strings.TrimSpace(statusObs[0].Value)
			statusRecorded = true
			if sc, err := strconv.Atoi(firstVal); err == nil {
				finalStatus = sc
			}
			for _, o := range statusObs {
				addEvidenceRef(o, "final_status", audit.EvidenceRolePrimary)
				if strings.TrimSpace(o.Value) != firstVal {
					statusConflict = true
				}
			}
		}

		// 3. probe_final_url / redirect_final_url
		finalURLObs := idx.GetObservations(target.st, target.ref, "probe_final_url")
		if len(finalURLObs) == 0 {
			finalURLObs = idx.GetObservations(target.st, target.ref, "redirect_final_url")
		}
		var finalURL string
		if len(finalURLObs) > 0 {
			finalURL = strings.TrimSpace(finalURLObs[0].Value)
			for _, o := range finalURLObs {
				addEvidenceRef(o, "final_url", audit.EvidenceRoleSupporting)
			}
		} else {
			finalURL = probeURL
		}

		// 4. redirect_path / redirect_hop
		hopObs := idx.GetObservations(target.st, target.ref, "redirect_hop")
		for _, o := range hopObs {
			addEvidenceRef(o, "redirect_path", audit.EvidenceRoleSupporting)
		}

		scope := audit.ScopeSitewide
		if target.st == audit.SubjectURL {
			scope = audit.ScopeURL
		}

		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		if urlConflict || statusConflict {
			evalStatus = audit.StatusUnknown
			observedSummary = "Conflicting known-missing probe observations recorded."
		} else if probeURL == "" {
			evalStatus = audit.StatusUnknown
			observedSummary = "Known-missing probe cannot complete or probe URL evidence is unavailable."
		} else if !statusRecorded || finalStatus <= 0 {
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("Known-missing probe %s cannot complete: no usable HTTP response obtained.", probeURL)
		} else if finalStatus == 404 || finalStatus == 410 {
			evalStatus = audit.StatusPass
			observedSummary = fmt.Sprintf("Known-missing probe %s correctly resolves to HTTP %d.", probeURL, finalStatus)
		} else {
			evalStatus = audit.StatusFail
			if finalURL != "" && finalURL != probeURL {
				observedSummary = fmt.Sprintf("Known-missing probe %s redirected to %s and returned HTTP status %d instead of expected 404 or 410.", probeURL, finalURL, finalStatus)
			} else {
				observedSummary = fmt.Sprintf("Known-missing probe %s returned HTTP status %d instead of expected 404 or 410 (soft 404 or improper error handling).", probeURL, finalStatus)
			}
		}

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     target.st,
			SubjectRef:      target.ref,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           scope,
			ObservedSummary: observedSummary,
			ExpectedSummary: expectedSummaryINDEX004,
			EvaluatedAt:     evalTime,
			EvidenceRefs:    evidenceRefs,
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", target.ref, err)
		}
		results = append(results, rr)
	}

	return results, nil
}
