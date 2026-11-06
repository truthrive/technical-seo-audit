package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDCANON010 = "AR-CANON-010"
const expectedSummaryCANON010 = "Redirect final destination is available (returns HTTP 2xx)."

// evaluateCANON010 implements AR-CANON-010: Redirect final target is available.
func evaluateCANON010(
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
		)
		if len(urlObs) > 0 {
			firstURL := strings.TrimSpace(urlObs[0].Value)
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

		if !urlUsable {
			obsSummary := "Required URL identity evidence is unavailable."
			if urlConflict {
				obsSummary = "No usable URL identity is available: conflicting URL identity observations."
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
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryCANON010,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    buildURLContextRefs(),
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Resolve redirect_initial_observed
		initRedirectObs := idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_initial_observed")
		isRedirect := false
		for _, o := range initRedirectObs {
			if strings.TrimSpace(o.Value) == "true" {
				isRedirect = true
				break
			}
		}

		if !isRedirect {
			evidenceRefs := buildURLContextRefs()
			for _, o := range initRedirectObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:redirect_initial_observed", ruleResultID, o.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(o.ObservationID),
					Field:             o.Field,
					ObservedValue:     o.Value,
					Role:              audit.EvidenceRoleContext,
				})
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
				Status:          audit.StatusNotApplicable,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: "URL does not redirect.",
				ExpectedSummary: expectedSummaryCANON010,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. Resolve redirect_loop_detected
		loopObs := idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_loop_detected")
		var (
			loopConflict bool
			hasLoop      bool
		)
		if len(loopObs) > 0 {
			firstVal := strings.TrimSpace(loopObs[0].Value)
			for _, o := range loopObs[1:] {
				if strings.TrimSpace(o.Value) != firstVal {
					loopConflict = true
					break
				}
			}
			if !loopConflict && firstVal == "true" {
				hasLoop = true
			}
		}

		evidenceRefs := buildURLContextRefs()
		for _, o := range initRedirectObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:redirect_initial_observed", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             o.Field,
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range loopObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:redirect_loop_detected", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             o.Field,
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		if loopConflict {
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     audit.SubjectURL,
				SubjectRef:      subjectRef,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: "Conflicting redirect loop observations on SubjectURL.",
				ExpectedSummary: expectedSummaryCANON010,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		if hasLoop {
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
				ObservedSummary: "Redirect loop detected; no final destination was reached.",
				ExpectedSummary: expectedSummaryCANON010,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Resolve final destination status
		targetRefObs := idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_final_target_subject_ref")
		finalURLObs := idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_final_url")

		for _, o := range finalURLObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:redirect_final_url", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             o.Field,
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRoleSupporting,
			})
		}

		if len(targetRefObs) == 0 {
			obsSummary := "Redirect final destination cannot be resolved."
			if len(finalURLObs) > 0 {
				obsSummary = fmt.Sprintf("Redirect final destination %q was not crawled in this audit run.", finalURLObs[0].Value)
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
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryCANON010,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		targetSubj := strings.TrimSpace(targetRefObs[0].Value)
		targetStatusObs := idx.GetObservations(audit.SubjectURL, targetSubj, "http_status")
		for _, o := range targetStatusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:final_http_status", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "http_status",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		var (
			targetMissing  bool
			targetConflict bool
			targetStatus   int
		)
		if len(targetStatusObs) == 0 {
			targetMissing = true
		} else {
			firstVal := strings.TrimSpace(targetStatusObs[0].Value)
			for _, o := range targetStatusObs[1:] {
				if strings.TrimSpace(o.Value) != firstVal {
					targetConflict = true
					break
				}
			}
			if !targetConflict {
				n, err := strconv.Atoi(firstVal)
				if err != nil || n <= 0 {
					targetMissing = true
				} else {
					targetStatus = n
				}
			}
		}

		if targetMissing || targetConflict {
			obsSummary := "Redirect final destination status cannot be obtained."
			if targetConflict {
				obsSummary = "Conflicting HTTP status observations for redirect final destination."
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
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryCANON010,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 5. Evaluate final destination status
		var (
			evalStatus audit.RuleResultStatus
			obsSummary string
		)
		if targetStatus >= 200 && targetStatus < 300 {
			evalStatus = audit.StatusPass
			obsSummary = fmt.Sprintf("Redirect final destination returned HTTP %d (available).", targetStatus)
		} else if targetStatus >= 400 && targetStatus < 600 {
			evalStatus = audit.StatusFail
			obsSummary = fmt.Sprintf("Redirect final destination returned HTTP %d (unavailable).", targetStatus)
		} else {
			evalStatus = audit.StatusUnknown
			obsSummary = fmt.Sprintf("Redirect final destination returned intermediate/unsupported status %d.", targetStatus)
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
			ObservedSummary: obsSummary,
			ExpectedSummary: expectedSummaryCANON010,
			EvaluatedAt:     evalTime,
			EvidenceRefs:    evidenceRefs,
		}
		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
		}
		results = append(results, rr)
	}

	return results, nil
}
