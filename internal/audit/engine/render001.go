package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDRENDER001 = "AR-RENDER-001"
const expectedSummaryRENDER001 = "Document title is unchanged between raw server-rendered HTML and client-side rendered DOM."

// evaluateRENDER001 implements AR-RENDER-001: Title changes after render.
func evaluateRENDER001(
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
				ExpectedSummary: expectedSummaryRENDER001,
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
				ExpectedSummary: expectedSummaryRENDER001,
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
				ExpectedSummary: expectedSummaryRENDER001,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Resolve raw and rendered title observations
		rawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "title_raw")
		renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "title_rendered")

		rawVal, rawUsable, rawConflict := resolveSingleStringObservation(rawObs)
		renderedVal, renderedUsable, renderedConflict := resolveSingleStringObservation(renderedObs)

		evidenceRefs := append([]audit.RuleEvidenceRef(nil), pre.urlContextRefs...)
		evidenceRefs = append(evidenceRefs, pre.renderContextRefs...)
		for _, o := range rawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:title_raw", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "title_raw",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range renderedObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:title_rendered", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "title_rendered",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		if !rawUsable || !renderedUsable {
			obsSummary := "Rendering requested but required title raw/rendered evidence is unavailable."
			if rawConflict || renderedConflict {
				obsSummary = "Rendering requested but conflicting title evidence was observed."
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
				ExpectedSummary: expectedSummaryRENDER001,
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
		rawNorm := collapseWhitespace(rawVal)
		renderedNorm := collapseWhitespace(renderedVal)
		if rawNorm == renderedNorm {
			status = audit.StatusPass
			obsSummary = fmt.Sprintf("Title is unchanged after JavaScript rendering (%q).", rawNorm)
		} else {
			status = audit.StatusWarning
			obsSummary = fmt.Sprintf("Title changed after JavaScript rendering (raw: %q, rendered: %q).", rawNorm, renderedNorm)
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
			ExpectedSummary: expectedSummaryRENDER001,
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
