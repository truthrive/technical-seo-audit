package engine

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const (
	ruleIDACC001          = "AR-ACC-001"
	expectedSummaryACC001 = "HTTPS endpoint is reachable with valid TLS and returns an HTTP response."
)

// evaluateACC001 implements the typed execution logic for:
// AR-ACC-001 — HTTPS endpoint reachable (parent check: ACC-001, P0, deterministic).
//
// Contract:
// - PASS: TLS connection succeeds and an HTTP response is obtained (status > 0, no fatal transport/TLS failure).
// - FAIL: DNS, connection, or TLS/certificate failure prevents the HTTPS endpoint from returning an HTTP response.
// - UNKNOWN: Fetch was required but no usable acquisition result was produced.
//
// Required inputs:
// - site_https_url
// - fetch_attempted
// - tls_valid
// - fetch_error_type
// - fetch_status
func evaluateACC001(
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

	// 1. Identify evaluation subjects.
	// Primary subject is SubjectSite ("site"). If not found, evaluate SubjectURL subjects with site_https_url.
	siteRefs := idx.SubjectRefs(audit.SubjectSite)
	var targetSubjects []struct {
		st  audit.EvaluationSubjectType
		ref string
	}

	if len(siteRefs) > 0 {
		for _, ref := range siteRefs {
			targetSubjects = append(targetSubjects, struct {
				st  audit.EvaluationSubjectType
				ref string
			}{st: audit.SubjectSite, ref: ref})
		}
	} else {
		for _, ref := range idx.SubjectRefs(audit.SubjectURL) {
			obs := idx.GetObservations(audit.SubjectURL, ref, "site_https_url")
			if len(obs) > 0 {
				targetSubjects = append(targetSubjects, struct {
					st  audit.EvaluationSubjectType
					ref string
				}{st: audit.SubjectURL, ref: ref})
			}
		}
	}

	if len(targetSubjects) == 0 {
		return []audit.RuleResult{}, nil
	}

	results := make([]audit.RuleResult, 0, len(targetSubjects))

	for _, ts := range targetSubjects {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, ts.ref))

		// Evidence collector helper
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

		// 1. site_https_url
		urlObs := idx.GetObservations(ts.st, ts.ref, "site_https_url")
		if len(urlObs) == 0 && ts.st == audit.SubjectURL {
			urlObs = idx.GetObservations(ts.st, ts.ref, "url_identity")
		}
		var (
			httpsURL    string
			urlConflict bool
		)
		if len(urlObs) > 0 {
			httpsURL = strings.TrimSpace(urlObs[0].Value)
			for _, o := range urlObs {
				addEvidenceRef(o, "site_https_url", audit.EvidenceRoleContext)
				if strings.TrimSpace(o.Value) != httpsURL {
					urlConflict = true
				}
			}
		}

		// 2. fetch_attempted
		attemptObs := idx.GetObservations(ts.st, ts.ref, "fetch_attempted")
		var (
			fetchAttempted  bool
			attemptExplicit bool
			attemptConflict bool
		)
		if len(attemptObs) > 0 {
			firstVal := strings.TrimSpace(attemptObs[0].Value)
			attemptExplicit = true
			if parsed, err := strconv.ParseBool(firstVal); err == nil {
				fetchAttempted = parsed
			}
			for _, o := range attemptObs {
				addEvidenceRef(o, "fetch_attempted", audit.EvidenceRoleSupporting)
				if strings.TrimSpace(o.Value) != firstVal {
					attemptConflict = true
				}
			}
		}

		// 3. fetch_status / http_status
		statusObs := idx.GetObservations(ts.st, ts.ref, "fetch_status")
		if len(statusObs) == 0 {
			statusObs = idx.GetObservations(ts.st, ts.ref, "http_status")
		}
		var (
			statusCode     int
			statusRecorded bool
			statusConflict bool
		)
		if len(statusObs) > 0 {
			firstStatus := strings.TrimSpace(statusObs[0].Value)
			statusRecorded = true
			if sc, err := strconv.Atoi(firstStatus); err == nil {
				statusCode = sc
			}
			for _, o := range statusObs {
				addEvidenceRef(o, "fetch_status", audit.EvidenceRolePrimary)
				if strings.TrimSpace(o.Value) != firstStatus {
					statusConflict = true
				}
			}
		}

		// 4. tls_valid
		tlsObs := idx.GetObservations(ts.st, ts.ref, "tls_valid")
		var (
			tlsValidExplicit *bool
			tlsConflict      bool
		)
		if len(tlsObs) > 0 {
			firstTLS := strings.TrimSpace(tlsObs[0].Value)
			if parsed, err := strconv.ParseBool(firstTLS); err == nil {
				tlsValidExplicit = &parsed
			}
			for _, o := range tlsObs {
				addEvidenceRef(o, "tls_valid", audit.EvidenceRolePrimary)
				if strings.TrimSpace(o.Value) != firstTLS {
					tlsConflict = true
				}
			}
		}

		// 5. fetch_error_type
		errObs := idx.GetObservations(ts.st, ts.ref, "fetch_error_type")
		var (
			errorType   string
			errConflict bool
		)
		if len(errObs) > 0 {
			errorType = strings.TrimSpace(errObs[0].Value)
			for _, o := range errObs {
				addEvidenceRef(o, "fetch_error_type", audit.EvidenceRolePrimary)
				if strings.TrimSpace(o.Value) != errorType {
					errConflict = true
				}
			}
		}

		// Determine scope
		scope := audit.ScopeSitewide
		if ts.st == audit.SubjectURL {
			scope = audit.ScopeURL
		}

		// Evaluation Logic
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		if urlConflict || attemptConflict || statusConflict || tlsConflict || errConflict {
			evalStatus = audit.StatusUnknown
			observedSummary = "Conflicting HTTPS reachability evidence observations recorded."
		} else if httpsURL == "" {
			evalStatus = audit.StatusUnknown
			observedSummary = "HTTPS origin cannot be derived or required HTTPS URL evidence is unavailable."
		} else if parsedURL, err := url.Parse(httpsURL); err != nil || parsedURL.Scheme != "https" {
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("Supplied site_https_url %q is not a valid HTTPS URL.", httpsURL)
		} else if attemptExplicit && !fetchAttempted {
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("Fetch was required for HTTPS endpoint %s but was not attempted.", httpsURL)
		} else if !statusRecorded && errorType == "" {
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("Fetch was required for HTTPS endpoint %s but no usable acquisition result was produced.", httpsURL)
		} else if (tlsValidExplicit != nil && !*tlsValidExplicit) || errorType == "ssl-error" {
			evalStatus = audit.StatusFail
			observedSummary = fmt.Sprintf("TLS certificate or handshake verification failed for HTTPS endpoint %s (error: %s).", httpsURL, errorType)
		} else if errorType == "dns-not-found" {
			evalStatus = audit.StatusFail
			observedSummary = fmt.Sprintf("DNS resolution failure prevented connection to HTTPS endpoint %s.", httpsURL)
		} else if errorType == "connection-refused" {
			evalStatus = audit.StatusFail
			observedSummary = fmt.Sprintf("TCP connection refused by server for HTTPS endpoint %s.", httpsURL)
		} else if errorType == "timeout" {
			evalStatus = audit.StatusFail
			observedSummary = fmt.Sprintf("Connection timeout connecting to HTTPS endpoint %s.", httpsURL)
		} else if errorType != "" && statusCode <= 0 {
			evalStatus = audit.StatusFail
			observedSummary = fmt.Sprintf("Transport connection failure (%s) prevented HTTP response from HTTPS endpoint %s.", errorType, httpsURL)
		} else if statusCode > 0 {
			// TLS handshake succeeded and an HTTP response was obtained
			evalStatus = audit.StatusPass
			observedSummary = fmt.Sprintf("TLS connection succeeded and an HTTP response was obtained for %s (HTTP %d).", httpsURL, statusCode)
		} else {
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("HTTPS endpoint %s did not produce a usable response or verified error.", httpsURL)
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
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           scope,
			ObservedSummary: observedSummary,
			ExpectedSummary: expectedSummaryACC001,
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
