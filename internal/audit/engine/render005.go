package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDRENDER005 = "AR-RENDER-005"
const expectedSummaryRENDER005 = "Main-text presence state is unchanged between raw server-rendered HTML and client-side rendered DOM."

// evaluateRENDER005 implements AR-RENDER-005: Main-text presence changes after render.
func evaluateRENDER005(
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
				ExpectedSummary: expectedSummaryRENDER005,
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
				ExpectedSummary: expectedSummaryRENDER005,
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
				ExpectedSummary: expectedSummaryRENDER005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Resolve raw and rendered main-text presence observations
		rawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "main_text_present_raw")
		renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "main_text_present_rendered")

		rawBool, rawUsable, rawConflict := resolveSingleBoolObservation(rawObs)
		renderedBool, renderedUsable, renderedConflict := resolveSingleBoolObservation(renderedObs)

		evidenceRefs := append([]audit.RuleEvidenceRef(nil), pre.urlContextRefs...)
		evidenceRefs = append(evidenceRefs, pre.renderContextRefs...)
		for _, o := range rawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:main_text_present_raw", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "main_text_present_raw",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range renderedObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:main_text_present_rendered", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "main_text_present_rendered",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		if !rawUsable || !renderedUsable {
			obsSummary := "Rendering requested but required main-text presence raw/rendered evidence is unavailable."
			if rawConflict || renderedConflict {
				obsSummary = "Rendering requested but conflicting main-text presence evidence was observed."
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
				ExpectedSummary: expectedSummaryRENDER005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 5. Evaluate comparison
		var (
			status     audit.RuleResultStatus
			obsSummary string
		)
		if rawBool == renderedBool {
			status = audit.StatusPass
			obsSummary = fmt.Sprintf("Main-text presence state is identical between raw HTML and rendered DOM (%v).", rawBool)
		} else {
			status = audit.StatusWarning
			obsSummary = fmt.Sprintf("Main-text presence differs between raw HTML (%v) and rendered DOM (%v).", rawBool, renderedBool)
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
			ExpectedSummary: expectedSummaryRENDER005,
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
