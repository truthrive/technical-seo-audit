package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDRENDER003 = "AR-RENDER-003"
const expectedSummaryRENDER003 = "Meta robots directives are unchanged between raw server-rendered HTML and client-side rendered DOM."

// evaluateRENDER003 implements AR-RENDER-003: Robots directives change after render.
func evaluateRENDER003(
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
		pre := resolveRenderPrecondition(idx, subjectRef, ruleResultID)

		// 1. URL identity check
		if !pre.urlUsable {
			obsSummary := "Required URL identity evidence is unavailable."
			if pre.urlConflict {
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
				ExpectedSummary: expectedSummaryRENDER003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    pre.urlContextRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Precondition check: page selected for rendering
		if !pre.selectedForRender {
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
				ObservedSummary: "Page was not selected for JavaScript rendering.",
				ExpectedSummary: expectedSummaryRENDER003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    pre.urlContextRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. Render execution failure
		if pre.renderFailed {
			evidenceRefs := append([]audit.RuleEvidenceRef(nil), pre.urlContextRefs...)
			evidenceRefs = append(evidenceRefs, pre.renderContextRefs...)
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
				ObservedSummary: fmt.Sprintf("JavaScript rendering was requested but execution failed: %s.", pre.renderFailedMsg),
				ExpectedSummary: expectedSummaryRENDER003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Resolve raw and rendered meta robots observations
		rawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "meta_robots_raw")
		renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "meta_robots_rendered")

		evidenceRefs := append([]audit.RuleEvidenceRef(nil), pre.urlContextRefs...)
		evidenceRefs = append(evidenceRefs, pre.renderContextRefs...)
		for _, o := range rawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:meta_robots_raw", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "meta_robots_raw",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range renderedObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:meta_robots_rendered", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "meta_robots_rendered",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		if len(rawObs) == 0 || len(renderedObs) == 0 {
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
				ObservedSummary: "Rendering requested but required meta robots raw/rendered evidence is unavailable.",
				ExpectedSummary: expectedSummaryRENDER003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// Collect directive token sets
		var rawTokens []string
		for _, o := range rawObs {
			rawTokens = append(rawTokens, splitAndNormalizeTokens(o.Value)...)
		}
		var renderedTokens []string
		for _, o := range renderedObs {
			renderedTokens = append(renderedTokens, splitAndNormalizeTokens(o.Value)...)
		}

		rawSet := splitAndNormalizeTokens(strings.Join(rawTokens, ","))
		renderedSet := splitAndNormalizeTokens(strings.Join(renderedTokens, ","))

		// 5. Evaluate comparison
		var (
			status     audit.RuleResultStatus
			obsSummary string
		)
		if slicesEqual(rawSet, renderedSet) {
			status = audit.StatusPass
			if len(rawSet) == 0 {
				obsSummary = "No meta robots directives declared in both raw HTML and rendered DOM."
			} else {
				obsSummary = fmt.Sprintf("Meta robots directives are unchanged after JavaScript rendering: [%s].", strings.Join(rawSet, ", "))
			}
		} else {
			status = audit.StatusWarning
			obsSummary = fmt.Sprintf("Meta robots directives changed after JavaScript rendering (raw: [%s], rendered: [%s]).",
				strings.Join(rawSet, ", "), strings.Join(renderedSet, ", "))
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
			Status:          status,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeURL,
			ObservedSummary: obsSummary,
			ExpectedSummary: expectedSummaryRENDER003,
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
