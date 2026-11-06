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
	ruleIDINDEX006          = "AR-INDEX-006"
	expectedSummaryINDEX006 = "HTTP 200 response content differs from known 404 error template."
)

// evaluateINDEX006 implements the typed execution logic for:
// AR-INDEX-006 — 200 response matches known 404 template (parent check: INDEX-003, P1, assisted).
//
// Contract:
// - PASS: Target fingerprint differs from known 404-template fingerprint.
// - WARNING: Target returns 200 and exactly matches known 404-template fingerprint (soft 404).
// - NOT_APPLICABLE: Target is not 200.
// - UNKNOWN: Reliable comparison unavailable (probe failed, fingerprint missing, or conflicting).
//
// Required inputs:
// - url
// - fetch_status
// - content_fingerprint
// - known_404_content_fingerprint
// - known_404_probe_status
func evaluateINDEX006(
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
	if len(urlRefs) == 0 {
		return []audit.RuleResult{}, nil
	}

	// 1. Resolve known 404 probe template evidence across the snapshot
	var (
		known404Fingerprint string
		known404ProbeStatus int
		known404ProbeURL    string
		known404Evidence    []audit.NormalizedObservation
	)

	// Check SubjectSite ("site")
	for _, o := range idx.GetObservations(audit.SubjectSite, "site", "known_404_content_fingerprint") {
		known404Fingerprint = strings.TrimSpace(o.Value)
		known404Evidence = append(known404Evidence, o)
	}
	for _, o := range idx.GetObservations(audit.SubjectSite, "site", "known_404_probe_status") {
		if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err == nil {
			known404ProbeStatus = s
		}
		known404Evidence = append(known404Evidence, o)
	}
	for _, o := range idx.GetObservations(audit.SubjectSite, "site", "known_404_probe_url") {
		known404ProbeURL = strings.TrimSpace(o.Value)
		known404Evidence = append(known404Evidence, o)
	}

	// If not on SubjectSite, check SubjectProbe subjects
	if known404Fingerprint == "" || known404ProbeStatus == 0 {
		for _, pRef := range idx.SubjectRefs(audit.SubjectProbe) {
			for _, o := range idx.GetObservations(audit.SubjectProbe, pRef, "content_fingerprint") {
				if known404Fingerprint == "" {
					known404Fingerprint = strings.TrimSpace(o.Value)
					known404Evidence = append(known404Evidence, o)
				}
			}
			for _, o := range idx.GetObservations(audit.SubjectProbe, pRef, "probe_final_status") {
				if known404ProbeStatus == 0 {
					if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err == nil {
						known404ProbeStatus = s
						known404Evidence = append(known404Evidence, o)
					}
				}
			}
			if known404ProbeStatus == 0 {
				for _, o := range idx.GetObservations(audit.SubjectProbe, pRef, "http_status") {
					if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err == nil {
						known404ProbeStatus = s
						known404Evidence = append(known404Evidence, o)
					}
				}
			}
			for _, o := range idx.GetObservations(audit.SubjectProbe, pRef, "missing_probe_url") {
				if known404ProbeURL == "" {
					known404ProbeURL = strings.TrimSpace(o.Value)
					known404Evidence = append(known404Evidence, o)
				}
			}
		}
	}

	results := make([]audit.RuleResult, 0, len(urlRefs))

	for _, subjectRef := range urlRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

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

		// 1. Resolve URL identity
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		var (
			urlVal      string
			urlConflict bool
		)
		if len(urlObs) > 0 {
			urlVal = strings.TrimSpace(urlObs[0].Value)
			for _, o := range urlObs {
				addEvidenceRef(o, "url", audit.EvidenceRoleContext)
				if strings.TrimSpace(o.Value) != urlVal {
					urlConflict = true
				}
			}
		}

		if urlConflict || urlVal == "" {
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
				ExpectedSummary: expectedSummaryINDEX006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Resolve fetch_status
		statusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "http_status")
		if len(statusObs) == 0 {
			statusObs = idx.GetObservations(audit.SubjectURL, subjectRef, "fetch_status")
		}
		var (
			statusCode     int
			statusRecorded bool
			statusConflict bool
		)
		if len(statusObs) > 0 {
			firstVal := strings.TrimSpace(statusObs[0].Value)
			statusRecorded = true
			if sc, err := strconv.Atoi(firstVal); err == nil {
				statusCode = sc
			}
			for _, o := range statusObs {
				addEvidenceRef(o, "status", audit.EvidenceRolePrimary)
				if strings.TrimSpace(o.Value) != firstVal {
					statusConflict = true
				}
			}
		}

		if statusConflict || !statusRecorded || statusCode <= 0 {
			obsSummary := fmt.Sprintf("Target URL %s did not produce a usable HTTP response.", urlVal)
			if statusConflict {
				obsSummary = "Conflicting HTTP status observations recorded."
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
				ExpectedSummary: expectedSummaryINDEX006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// Precondition: Rule applies only to HTTP 200 responses
		if statusCode != 200 {
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
				ObservedSummary: fmt.Sprintf("Target URL returned HTTP status %d (rule applies only to HTTP 200 responses).", statusCode),
				ExpectedSummary: expectedSummaryINDEX006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// Check local overrides on subject if present
		subject404FP := known404Fingerprint
		subject404Status := known404ProbeStatus
		subject404URL := known404ProbeURL
		for _, o := range idx.GetObservations(audit.SubjectURL, subjectRef, "known_404_content_fingerprint") {
			subject404FP = strings.TrimSpace(o.Value)
			addEvidenceRef(o, "known_404_content_fingerprint", audit.EvidenceRoleSupporting)
		}
		for _, o := range idx.GetObservations(audit.SubjectURL, subjectRef, "known_404_probe_status") {
			if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err == nil {
				subject404Status = s
			}
			addEvidenceRef(o, "probe_status", audit.EvidenceRoleSupporting)
		}

		// Add global probe evidence refs if used
		for _, o := range known404Evidence {
			field := o.Field
			if field == "known_404_content_fingerprint" || field == "content_fingerprint" {
				addEvidenceRef(o, "known_404_content_fingerprint", audit.EvidenceRoleSupporting)
			} else if field == "known_404_probe_status" || field == "probe_final_status" || field == "http_status" {
				addEvidenceRef(o, "probe_status", audit.EvidenceRoleSupporting)
			} else if field == "known_404_probe_url" || field == "missing_probe_url" {
				addEvidenceRef(o, "probe_url", audit.EvidenceRoleContext)
			}
		}

		// Precondition: Known-missing probe must have returned 404/410 with fingerprint
		if (subject404Status != 404 && subject404Status != 410) || subject404FP == "" {
			obsSummary := "Reliable soft-404 comparison unavailable: known-missing 404 template fingerprint is unavailable or probe did not return 404/410."
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
				ExpectedSummary: expectedSummaryINDEX006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. Resolve target content_fingerprint
		var (
			targetFingerprint string
			fpConflict        bool
		)
		fpObs := idx.GetObservations(audit.SubjectURL, subjectRef, "content_fingerprint")
		if len(fpObs) > 0 {
			targetFingerprint = strings.TrimSpace(fpObs[0].Value)
			for _, o := range fpObs {
				addEvidenceRef(o, "content_fingerprint", audit.EvidenceRolePrimary)
				if strings.TrimSpace(o.Value) != targetFingerprint {
					fpConflict = true
				}
			}
		}

		if fpConflict || targetFingerprint == "" {
			obsSummary := fmt.Sprintf("Target URL %s content fingerprint is unavailable for 404 template comparison.", urlVal)
			if fpConflict {
				obsSummary = "Conflicting content fingerprint observations recorded for target URL."
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
				ExpectedSummary: expectedSummaryINDEX006,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		_ = subject404URL

		// 4. Compare fingerprints
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		if targetFingerprint == subject404FP {
			evalStatus = audit.StatusWarning
			observedSummary = fmt.Sprintf("Target URL %s returns HTTP 200 but content matches known 404 template fingerprint (%s) (soft 404 detected).", urlVal, targetFingerprint)
		} else {
			evalStatus = audit.StatusPass
			observedSummary = fmt.Sprintf("Target URL %s returns HTTP 200 and content fingerprint (%s) differs from known 404 template fingerprint (%s).", urlVal, targetFingerprint, subject404FP)
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
			ObservedSummary: observedSummary,
			ExpectedSummary: expectedSummaryINDEX006,
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
