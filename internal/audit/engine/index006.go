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
	type probeRecord struct {
		probeRef    string
		probeURL    string
		status      int
		fingerprint string
		evidence    []audit.NormalizedObservation
	}

	var (
		known404Fingerprint string
		known404ProbeStatus int
		known404ProbeURL    string
		known404Evidence    []audit.NormalizedObservation
		probeConflict       bool
	)

	// Step 1a: Check SubjectSite ("site")
	siteFPObs := idx.GetObservations(audit.SubjectSite, "site", "known_404_content_fingerprint")
	siteStatusObs := idx.GetObservations(audit.SubjectSite, "site", "known_404_probe_status")
	siteURLObs := idx.GetObservations(audit.SubjectSite, "site", "known_404_probe_url")

	var (
		siteFP     string
		siteStatus int
		siteURL    string
	)
	if len(siteFPObs) > 0 {
		siteFP = strings.TrimSpace(siteFPObs[0].Value)
		for _, o := range siteFPObs {
			if strings.TrimSpace(o.Value) != siteFP {
				probeConflict = true
			}
			known404Evidence = append(known404Evidence, o)
		}
	}
	if len(siteStatusObs) > 0 {
		if s, err := strconv.Atoi(strings.TrimSpace(siteStatusObs[0].Value)); err == nil {
			siteStatus = s
		}
		for _, o := range siteStatusObs {
			if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err != nil || s != siteStatus {
				probeConflict = true
			}
			known404Evidence = append(known404Evidence, o)
		}
	}
	if len(siteURLObs) > 0 {
		siteURL = strings.TrimSpace(siteURLObs[0].Value)
		for _, o := range siteURLObs {
			if strings.TrimSpace(o.Value) != siteURL {
				probeConflict = true
			}
			known404Evidence = append(known404Evidence, o)
		}
	}

	if siteFP != "" && (siteStatus == 404 || siteStatus == 410) && !probeConflict {
		known404Fingerprint = siteFP
		known404ProbeStatus = siteStatus
		known404ProbeURL = siteURL
	}

	// Step 1b: If not on SubjectSite, check SubjectProbe subjects
	if known404Fingerprint == "" && !probeConflict {
		var validProbes []probeRecord
		var seenStatuses []int

		for _, pRef := range idx.SubjectRefs(audit.SubjectProbe) {
			var (
				pFP          string
				pStatus      int
				pURL         string
				pEvidence    []audit.NormalizedObservation
				pIntConflict bool
			)

			// Fingerprint for THIS probe
			fpObs := idx.GetObservations(audit.SubjectProbe, pRef, "content_fingerprint")
			if len(fpObs) == 0 {
				fpObs = idx.GetObservations(audit.SubjectProbe, pRef, "known_404_content_fingerprint")
			}
			if len(fpObs) > 0 {
				pFP = strings.TrimSpace(fpObs[0].Value)
				for _, o := range fpObs {
					pEvidence = append(pEvidence, o)
					if strings.TrimSpace(o.Value) != pFP {
						pIntConflict = true
					}
				}
			}

			// Status for THIS probe
			stObs := idx.GetObservations(audit.SubjectProbe, pRef, "probe_final_status")
			if len(stObs) == 0 {
				stObs = idx.GetObservations(audit.SubjectProbe, pRef, "http_status")
			}
			if len(stObs) == 0 {
				stObs = idx.GetObservations(audit.SubjectProbe, pRef, "known_404_probe_status")
			}
			if len(stObs) > 0 {
				if s, err := strconv.Atoi(strings.TrimSpace(stObs[0].Value)); err == nil {
					pStatus = s
					seenStatuses = append(seenStatuses, s)
				}
				for _, o := range stObs {
					pEvidence = append(pEvidence, o)
					if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err != nil || s != pStatus {
						pIntConflict = true
					}
				}
			}

			// URL for THIS probe
			uObs := idx.GetObservations(audit.SubjectProbe, pRef, "missing_probe_url")
			if len(uObs) == 0 {
				uObs = idx.GetObservations(audit.SubjectProbe, pRef, "probe_url")
			}
			if len(uObs) == 0 {
				uObs = idx.GetObservations(audit.SubjectProbe, pRef, "url_identity")
			}
			if len(uObs) > 0 {
				pURL = strings.TrimSpace(uObs[0].Value)
				for _, o := range uObs {
					pEvidence = append(pEvidence, o)
					if strings.TrimSpace(o.Value) != pURL {
						pIntConflict = true
					}
				}
			}

			if pIntConflict {
				probeConflict = true
				continue
			}

			// A probe candidate is valid ONLY if THIS same probe has BOTH status 404/410 AND a fingerprint
			if pFP != "" && (pStatus == 404 || pStatus == 410) {
				validProbes = append(validProbes, probeRecord{
					probeRef:    pRef,
					probeURL:    pURL,
					status:      pStatus,
					fingerprint: pFP,
					evidence:    pEvidence,
				})
			}
		}

		// Check if multiple probes have conflicting statuses
		if len(seenStatuses) > 1 {
			firstSt := seenStatuses[0]
			for _, st := range seenStatuses {
				if st != firstSt {
					probeConflict = true
					break
				}
			}
		}

		// Reconcile valid probes
		if len(validProbes) > 0 && !probeConflict {
			firstFP := validProbes[0].fingerprint
			firstStatus := validProbes[0].status
			for _, vp := range validProbes {
				if vp.fingerprint != firstFP || vp.status != firstStatus {
					probeConflict = true
					break
				}
			}
			if !probeConflict {
				known404Fingerprint = firstFP
				known404ProbeStatus = firstStatus
				known404ProbeURL = validProbes[0].probeURL
				known404Evidence = validProbes[0].evidence
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

		// Resolve 404 template for this subject (site/probe template vs validated URL override)
		urlFPObs := idx.GetObservations(audit.SubjectURL, subjectRef, "known_404_content_fingerprint")
		urlStatusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "known_404_probe_status")
		hasURLOverride := len(urlFPObs) > 0 || len(urlStatusObs) > 0

		var (
			subject404FP     string
			subject404Status int
			subject404URL    string
			overrideConflict bool
			overrideSummary  string
		)

		if hasURLOverride {
			// A URL-level override is valid ONLY if BOTH fingerprint and status are explicitly present on this URL,
			// internally non-conflicting, correlated to the same probe provenance, and non-conflicting with site evidence.
			var (
				urlFP      string
				urlStatus  int
				fpConflict bool
				stConflict bool
				fpSrcs     []string
				stSrcs     []string
			)

			if len(urlFPObs) > 0 {
				urlFP = strings.TrimSpace(urlFPObs[0].Value)
				for _, o := range urlFPObs {
					addEvidenceRef(o, "known_404_content_fingerprint", audit.EvidenceRoleSupporting)
					fpSrcs = append(fpSrcs, o.SourceEvidenceRefs...)
					if strings.TrimSpace(o.Value) != urlFP {
						fpConflict = true
					}
				}
			}

			if len(urlStatusObs) > 0 {
				if s, err := strconv.Atoi(strings.TrimSpace(urlStatusObs[0].Value)); err == nil {
					urlStatus = s
				}
				for _, o := range urlStatusObs {
					addEvidenceRef(o, "probe_status", audit.EvidenceRoleSupporting)
					stSrcs = append(stSrcs, o.SourceEvidenceRefs...)
					if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err != nil || s != urlStatus {
						stConflict = true
					}
				}
			}

			// Check for conflicting duplicate observations on URL
			if fpConflict || stConflict {
				overrideConflict = true
				overrideSummary = "Reliable soft-404 comparison unavailable: conflicting duplicate URL-level template observations recorded."
			} else if len(urlFPObs) == 0 || len(urlStatusObs) == 0 {
				// Uncoupled override: cannot mix URL fingerprint with site status, or URL status with site fingerprint
				overrideConflict = true
				overrideSummary = "Reliable soft-404 comparison unavailable: URL-level template override lacks coupled status and fingerprint evidence."
			} else if !hasSharedProvenance(fpSrcs, stSrcs) {
				// Provenance indicates different probes
				overrideConflict = true
				overrideSummary = "Reliable soft-404 comparison unavailable: URL-level template status and fingerprint belong to different probes."
			} else if known404Fingerprint != "" && (urlFP != known404Fingerprint || urlStatus != known404ProbeStatus) {
				// Conflicts with verified site-level probe template
				overrideConflict = true
				overrideSummary = "Reliable soft-404 comparison unavailable: URL-level template override conflicts with verified site probe template."
			} else {
				subject404FP = urlFP
				subject404Status = urlStatus
				subject404URL = known404ProbeURL
			}
		} else {
			// No URL override: use verified site/probe level template
			if probeConflict {
				overrideConflict = true
				overrideSummary = "Reliable soft-404 comparison unavailable: conflicting probe template fingerprints or statuses recorded across probes."
			} else {
				subject404FP = known404Fingerprint
				subject404Status = known404ProbeStatus
				subject404URL = known404ProbeURL

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
			}
		}

		if overrideConflict {
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
				ObservedSummary: overrideSummary,
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
			observedSummary = fmt.Sprintf("Target URL %s returns HTTP 200 but content matches known 404 template fingerprint (%s) (potential soft-404 template match).", urlVal, targetFingerprint)
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

// hasSharedProvenance verifies whether two sets of source evidence references share at least one probe identifier
// or are both unconstrained by explicit conflicting probe tags.
func hasSharedProvenance(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return true // No explicit conflicting source tags indicated
	}
	for _, x := range a {
		xTrim := strings.TrimSpace(x)
		if xTrim == "" || xTrim == "test" {
			continue
		}
		for _, y := range b {
			yTrim := strings.TrimSpace(y)
			if yTrim == "" || yTrim == "test" {
				continue
			}
			if xTrim == yTrim {
				return true
			}
		}
	}
	// If both had non-empty non-generic probe tags (e.g. "probe:A" vs "probe:B") and none matched:
	hasNonGenericA := false
	for _, x := range a {
		t := strings.TrimSpace(x)
		if t != "" && t != "test" {
			hasNonGenericA = true
			break
		}
	}
	hasNonGenericB := false
	for _, y := range b {
		t := strings.TrimSpace(y)
		if t != "" && t != "test" {
			hasNonGenericB = true
			break
		}
	}
	if hasNonGenericA && hasNonGenericB {
		return false
	}
	return true
}
