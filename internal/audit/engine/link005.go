package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDLINK005 = "AR-LINK-005"
const expectedSummaryLINK005 = "URL discovered via supplemental source is linked by internal crawl graph (not an orphan relative to supplied discovery sources)."

// evaluateLINK005 implements AR-LINK-005: Orphan candidate relative to supplied discovery sources.
func evaluateLINK005(
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
				ExpectedSummary: expectedSummaryLINK005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    buildURLContextRefs(),
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Resolve discovery_sources (precondition: URL in at least one supplied non-crawl source)
		discoveryObs := idx.GetObservations(audit.SubjectURL, subjectRef, "discovery_sources")
		if len(discoveryObs) == 0 {
			evidenceRefs := buildURLContextRefs()
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
				ObservedSummary: "No supplied supplemental discovery source for SubjectURL.",
				ExpectedSummary: expectedSummaryLINK005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. Resolve is_start_url (precondition: not crawl start URL)
		startURLObs := idx.GetObservations(audit.SubjectURL, subjectRef, "is_start_url")
		var (
			startMissing  bool
			startConflict bool
			isStartURL    bool
		)
		if len(startURLObs) == 0 {
			startMissing = true
		} else {
			firstVal := strings.TrimSpace(startURLObs[0].Value)
			for _, o := range startURLObs[1:] {
				if strings.TrimSpace(o.Value) != firstVal {
					startConflict = true
					break
				}
			}
			if !startConflict {
				isStartURL = (firstVal == "true")
			}
		}

		evidenceRefs := buildURLContextRefs()
		for _, o := range discoveryObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:discovery_sources", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             o.Field,
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}
		for _, o := range startURLObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:is_start_url", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             o.Field,
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		if startMissing || startConflict {
			obsSummary := "Start URL status is unavailable."
			if startConflict {
				obsSummary = "Conflicting start URL observations on SubjectURL."
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
				ExpectedSummary: expectedSummaryLINK005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		if isStartURL {
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
				ObservedSummary: "SubjectURL is a crawl start URL; orphan evaluation is not applicable.",
				ExpectedSummary: expectedSummaryLINK005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Resolve crawl completeness
		if !snapshot.CrawlComplete {
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
				ObservedSummary: "Crawl graph incomplete; crawl frontier was not cleanly exhausted.",
				ExpectedSummary: expectedSummaryLINK005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 5. Resolve crawl_inlink_count
		inlinkObs := idx.GetObservations(audit.SubjectURL, subjectRef, "crawl_inlink_count")
		var (
			inlinkMissing  bool
			inlinkConflict bool
			inlinkCount    int
		)
		if len(inlinkObs) == 0 {
			inlinkMissing = true
		} else {
			firstVal := strings.TrimSpace(inlinkObs[0].Value)
			for _, o := range inlinkObs[1:] {
				if strings.TrimSpace(o.Value) != firstVal {
					inlinkConflict = true
					break
				}
			}
			if !inlinkConflict {
				n, err := strconv.Atoi(firstVal)
				if err != nil || n < 0 {
					inlinkMissing = true
				} else {
					inlinkCount = n
				}
			}
		}

		for _, o := range inlinkObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:crawl_inlink_count", ruleResultID, o.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(o.ObservationID),
				Field:             o.Field,
				ObservedValue:     o.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		if inlinkMissing || inlinkConflict {
			obsSummary := "Internal crawl inlink count observation is unavailable."
			if inlinkConflict {
				obsSummary = "Conflicting internal crawl inlink observations on SubjectURL."
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
				ExpectedSummary: expectedSummaryLINK005,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 6. Evaluate Pass vs Warning
		// Guardrail: wording must remain "orphan relative to supplied discovery sources"
		var (
			evalStatus audit.RuleResultStatus
			obsSummary string
		)
		if inlinkCount > 0 {
			evalStatus = audit.StatusPass
			obsSummary = fmt.Sprintf("At least one crawlable internal inlink found (%d inlinks); not an orphan relative to supplied discovery sources.", inlinkCount)
		} else {
			evalStatus = audit.StatusWarning
			obsSummary = "Zero crawlable internal inlinks found; orphan candidate relative to supplied discovery sources."
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
			ExpectedSummary: expectedSummaryLINK005,
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
