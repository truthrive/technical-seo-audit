package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDRENDER006 = "AR-RENDER-006"
const expectedSummaryRENDER006 = "Crawlable internal-link set is unchanged between raw server-rendered HTML and client-side rendered DOM."

// evaluateRENDER006 implements AR-RENDER-006: Crawlable internal-link set changes after render.
func evaluateRENDER006(
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
				ExpectedSummary: expectedSummaryRENDER006,
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
				ExpectedSummary: expectedSummaryRENDER006,
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
				ExpectedSummary: expectedSummaryRENDER006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Resolve raw and rendered internal links observations
		rawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "internal_links_raw")
		renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "internal_links_rendered")

		evidenceRefs := append([]audit.RuleEvidenceRef(nil), pre.urlContextRefs...)
		evidenceRefs = append(evidenceRefs, pre.renderContextRefs...)
		for _, o := range rawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:internal_links_raw", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "internal_links_raw",
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range renderedObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:internal_links_rendered", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             "internal_links_rendered",
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
				ObservedSummary: "Rendering requested but required internal links raw/rendered evidence is unavailable.",
				ExpectedSummary: expectedSummaryRENDER006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// Collect and normalize internal link sets
		rawSet, rawOK := parseNormalizedURLSet(rawObs, pre.urlVal)
		renderedSet, renderedOK := parseNormalizedURLSet(renderedObs, pre.urlVal)
		if !rawOK || !renderedOK {
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
				ObservedSummary: "Rendering requested but internal links raw/rendered URL evidence is malformed or unresolvable.",
				ExpectedSummary: expectedSummaryRENDER006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		added := setDifference(renderedSet, rawSet)
		removed := setDifference(rawSet, renderedSet)

		// 5. Evaluate comparison
		var (
			status     audit.RuleResultStatus
			obsSummary string
		)
		if len(added) == 0 && len(removed) == 0 {
			status = audit.StatusPass
			if len(rawSet) == 0 {
				obsSummary = "No crawlable internal links discovered in both raw HTML and rendered DOM."
			} else {
				obsSummary = fmt.Sprintf("Crawlable internal-link set is unchanged after JavaScript rendering (%d links).", len(rawSet))
			}
		} else {
			status = audit.StatusWarning
			obsSummary = fmt.Sprintf("Crawlable internal-link set changed after JavaScript rendering (added: %d, removed: %d).",
				len(added), len(removed))
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
			ExpectedSummary: expectedSummaryRENDER006,
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
