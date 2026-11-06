package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const (
	ruleIDCANON002          = "AR-CANON-002"
	expectedSummaryCANON002 = "Trailing-slash pair is consistently normalized with one canonical 2xx variant and permanent redirect."
)

var staticFileExts = map[string]bool{
	".html": true, ".htm": true, ".php": true, ".asp": true, ".aspx": true,
	".pdf": true, ".xml": true, ".css": true, ".js": true, ".json": true,
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
	".webp": true, ".ico": true, ".txt": true, ".zip": true, ".gz": true,
	".tar": true, ".mp3": true, ".mp4": true, ".wav": true, ".woff": true,
	".woff2": true, ".ttf": true, ".eot": true,
}

// evaluateCANON002 implements the typed execution logic for:
// AR-CANON-002 — Trailing-slash pair is consistently normalized (parent check: CANON-002, P1, deterministic).
//
// Contract:
// - PASS: One variant is final 2xx and alternative permanently redirects directly to it (301/308).
// - FAIL: Both variants are 2xx with identical normalized content hashes (duplicate content).
// - WARNING: Both are 2xx but hashes differ, or normalization is inconsistent without proving duplication.
// - NOT_APPLICABLE: No meaningful slash pair (e.g. root origin '/', or static file URLs).
// - UNKNOWN: Either variant cannot be evaluated.
//
// Required inputs:
// - slash_url
// - non_slash_url
// - slash_response_state
// - non_slash_response_state
// - redirect_paths
// - normalized_content_hash
func evaluateCANON002(
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
	probeRefs := idx.SubjectRefs(audit.SubjectProbe)

	type subjectRefTarget struct {
		st  audit.EvaluationSubjectType
		ref string
	}
	var targets []subjectRefTarget
	for _, ref := range urlRefs {
		targets = append(targets, subjectRefTarget{st: audit.SubjectURL, ref: ref})
	}
	for _, ref := range probeRefs {
		targets = append(targets, subjectRefTarget{st: audit.SubjectProbe, ref: ref})
	}

	if len(targets) == 0 {
		return []audit.RuleResult{}, nil
	}

	// Build map of normalized URL to subjectRef for pairing lookups across SubjectURL
	urlToRef := make(map[string]string)
	for _, ref := range urlRefs {
		obs := idx.GetObservations(audit.SubjectURL, ref, "url_identity")
		if len(obs) > 0 {
			u := strings.TrimSpace(obs[0].Value)
			if u != "" {
				urlToRef[cleanURLForMatching(u)] = ref
			}
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

		// 1. Resolve current subject URL
		urlObs := idx.GetObservations(target.st, target.ref, "url_identity")
		var currentURL string
		if len(urlObs) > 0 {
			currentURL = strings.TrimSpace(urlObs[0].Value)
			for _, o := range urlObs {
				addEvidenceRef(o, "url", audit.EvidenceRoleContext)
			}
		}

		// Check explicit slash_url / non_slash_url observations on the subject
		slashURLObs := idx.GetObservations(target.st, target.ref, "slash_url")
		nonSlashURLObs := idx.GetObservations(target.st, target.ref, "non_slash_url")
		var explicitSlashURL, explicitNonSlashURL string
		if len(slashURLObs) > 0 {
			explicitSlashURL = strings.TrimSpace(slashURLObs[0].Value)
			for _, o := range slashURLObs {
				addEvidenceRef(o, "slash_url", audit.EvidenceRoleContext)
			}
		}
		if len(nonSlashURLObs) > 0 {
			explicitNonSlashURL = strings.TrimSpace(nonSlashURLObs[0].Value)
			for _, o := range nonSlashURLObs {
				addEvidenceRef(o, "non_slash_url", audit.EvidenceRoleContext)
			}
		}

		evalURL := currentURL
		if evalURL == "" && explicitSlashURL != "" {
			evalURL = explicitSlashURL
		} else if evalURL == "" && explicitNonSlashURL != "" {
			evalURL = explicitNonSlashURL
		}

		if evalURL == "" {
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     target.st,
				SubjectRef:      target.ref,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: "URL identity evidence is unavailable for trailing-slash evaluation.",
				ExpectedSummary: expectedSummaryCANON002,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", target.ref, err)
			}
			results = append(results, rr)
			continue
		}

		parsed, err := url.Parse(evalURL)
		if err != nil {
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     target.st,
				SubjectRef:      target.ref,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: fmt.Sprintf("URL %q is syntactically invalid.", evalURL),
				ExpectedSummary: expectedSummaryCANON002,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", target.ref, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Precondition check: NOT_APPLICABLE for root URL or static files
		urlPath := parsed.Path
		if urlPath == "" || urlPath == "/" {
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     target.st,
				SubjectRef:      target.ref,
				Status:          audit.StatusNotApplicable,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: fmt.Sprintf("Root URL %q has no meaningful trailing-slash pair.", evalURL),
				ExpectedSummary: expectedSummaryCANON002,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", target.ref, err)
			}
			results = append(results, rr)
			continue
		}

		ext := strings.ToLower(path.Ext(urlPath))
		if staticFileExts[ext] {
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     target.st,
				SubjectRef:      target.ref,
				Status:          audit.StatusNotApplicable,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: fmt.Sprintf("Static file URL %q has no meaningful trailing-slash pair.", evalURL),
				ExpectedSummary: expectedSummaryCANON002,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", target.ref, err)
			}
			results = append(results, rr)
			continue
		}

		// 3. Resolve slash and non-slash URL variants
		slashURL := explicitSlashURL
		nonSlashURL := explicitNonSlashURL

		if slashURL == "" || nonSlashURL == "" {
			if strings.HasSuffix(evalURL, "/") {
				slashURL = evalURL
				nonSlashURL = strings.TrimSuffix(evalURL, "/")
			} else {
				nonSlashURL = evalURL
				slashURL = evalURL + "/"
			}
		}

		// 4. Resolve states for both variants
		var (
			slashStatus         int
			nonSlashStatus      int
			slashFinalURL       string
			nonSlashFinalURL    string
			slashRedirectStatus int
			nonSlashRedirStatus int
			slashHash           string
			nonSlashHash        string
			normalizedHash      string
		)

		// Check explicit observations on target subject first
		for _, o := range idx.GetObservations(target.st, target.ref, "slash_response_state") {
			addEvidenceRef(o, "statuses", audit.EvidenceRolePrimary)
			if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err == nil {
				slashStatus = s
			}
		}
		for _, o := range idx.GetObservations(target.st, target.ref, "non_slash_response_state") {
			addEvidenceRef(o, "statuses", audit.EvidenceRolePrimary)
			if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err == nil {
				nonSlashStatus = s
			}
		}
		for _, o := range idx.GetObservations(target.st, target.ref, "slash_final_url") {
			addEvidenceRef(o, "redirect_paths", audit.EvidenceRoleSupporting)
			slashFinalURL = strings.TrimSpace(o.Value)
		}
		for _, o := range idx.GetObservations(target.st, target.ref, "non_slash_final_url") {
			addEvidenceRef(o, "redirect_paths", audit.EvidenceRoleSupporting)
			nonSlashFinalURL = strings.TrimSpace(o.Value)
		}
		for _, o := range idx.GetObservations(target.st, target.ref, "normalized_content_hash") {
			addEvidenceRef(o, "normalized_hashes", audit.EvidenceRoleSupporting)
			normalizedHash = strings.TrimSpace(o.Value)
		}

		// If states not fully resolved from explicit observations, inspect index via urlToRef
		extractSubjectState := func(ref string) (status int, finalURL string, redirStatus int, hash string) {
			for _, o := range idx.GetObservations(audit.SubjectURL, ref, "http_status") {
				if s, err := strconv.Atoi(strings.TrimSpace(o.Value)); err == nil {
					status = s
					addEvidenceRef(o, "statuses", audit.EvidenceRolePrimary)
				}
			}
			for _, o := range idx.GetObservations(audit.SubjectURL, ref, "redirect_final_url") {
				finalURL = strings.TrimSpace(o.Value)
				addEvidenceRef(o, "redirect_paths", audit.EvidenceRoleSupporting)
			}
			for _, o := range idx.GetObservations(audit.SubjectURL, ref, "redirect_hop") {
				var rh redirectHopPayload
				if err := json.Unmarshal([]byte(o.Value), &rh); err == nil && rh.Status > 0 && redirStatus == 0 {
					redirStatus = rh.Status
					addEvidenceRef(o, "redirect_paths", audit.EvidenceRoleSupporting)
				}
			}
			if redirStatus == 0 && (status == 301 || status == 308 || status == 302 || status == 307) {
				redirStatus = status
			}
			for _, o := range idx.GetObservations(audit.SubjectURL, ref, "content_fingerprint") {
				hash = strings.TrimSpace(o.Value)
				addEvidenceRef(o, "normalized_hashes", audit.EvidenceRoleSupporting)
			}
			return
		}

		if slashStatus == 0 {
			if sRef, ok := urlToRef[cleanURLForMatching(slashURL)]; ok {
				slashStatus, slashFinalURL, slashRedirectStatus, slashHash = extractSubjectState(sRef)
			}
		}
		if nonSlashStatus == 0 {
			if nsRef, ok := urlToRef[cleanURLForMatching(nonSlashURL)]; ok {
				nonSlashStatus, nonSlashFinalURL, nonSlashRedirStatus, nonSlashHash = extractSubjectState(nsRef)
			}
		}

		// If current subject is one of the variants, set its state directly
		if evalURL == slashURL && slashStatus == 0 {
			slashStatus, slashFinalURL, slashRedirectStatus, slashHash = extractSubjectState(target.ref)
		} else if evalURL == nonSlashURL && nonSlashStatus == 0 {
			nonSlashStatus, nonSlashFinalURL, nonSlashRedirStatus, nonSlashHash = extractSubjectState(target.ref)
		}

		// 5. Preconditions: Are both variants evaluable?
		if slashStatus <= 0 || nonSlashStatus <= 0 {
			obsSummary := fmt.Sprintf("Trailing-slash pair cannot be evaluated: missing response for one or both variants (%s status: %d, %s status: %d).",
				slashURL, slashStatus, nonSlashURL, nonSlashStatus)
			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     target.st,
				SubjectRef:      target.ref,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryCANON002,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", target.ref, err)
			}
			results = append(results, rr)
			continue
		}

		// 6. Evaluate verdicts
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		isSlash2xx := slashStatus >= 200 && slashStatus <= 299
		isNonSlash2xx := nonSlashStatus >= 200 && nonSlashStatus <= 299

		if isSlash2xx && isNonSlash2xx {
			// Both variants return 2xx!
			hashesMatch := false
			if normalizedHash != "" {
				hashesMatch = true
			} else if slashHash != "" && nonSlashHash != "" && slashHash == nonSlashHash {
				hashesMatch = true
			}

			if hashesMatch {
				evalStatus = audit.StatusFail
				observedSummary = fmt.Sprintf("Both trailing-slash variants return HTTP 2xx with identical content hash (%s and %s), causing duplicate content.", slashURL, nonSlashURL)
			} else {
				evalStatus = audit.StatusWarning
				observedSummary = fmt.Sprintf("Both trailing-slash variants return HTTP 2xx (%s and %s) without redirecting to a canonical version.", slashURL, nonSlashURL)
			}
		} else if isSlash2xx && (nonSlashStatus == 301 || nonSlashStatus == 308 || nonSlashRedirStatus == 301 || nonSlashRedirStatus == 308) {
			// Slash is 2xx, non-slash redirects permanently
			if cleanURLForMatching(nonSlashFinalURL) == cleanURLForMatching(slashURL) {
				evalStatus = audit.StatusPass
				observedSummary = fmt.Sprintf("Trailing-slash pair is consistently normalized: non-slash variant permanently redirects (HTTP %d) to canonical %s.", nonSlashStatus, slashURL)
			} else {
				evalStatus = audit.StatusWarning
				observedSummary = fmt.Sprintf("Non-slash variant permanently redirects to %s instead of canonical slash variant %s.", nonSlashFinalURL, slashURL)
			}
		} else if isNonSlash2xx && (slashStatus == 301 || slashStatus == 308 || slashRedirectStatus == 301 || slashRedirectStatus == 308) {
			// Non-slash is 2xx, slash redirects permanently
			if cleanURLForMatching(slashFinalURL) == cleanURLForMatching(nonSlashURL) {
				evalStatus = audit.StatusPass
				observedSummary = fmt.Sprintf("Trailing-slash pair is consistently normalized: slash variant permanently redirects (HTTP %d) to canonical %s.", slashStatus, nonSlashURL)
			} else {
				evalStatus = audit.StatusWarning
				observedSummary = fmt.Sprintf("Slash variant permanently redirects to %s instead of canonical non-slash variant %s.", slashFinalURL, nonSlashURL)
			}
		} else if isSlash2xx && (nonSlashStatus == 302 || nonSlashStatus == 307 || nonSlashRedirStatus == 302 || nonSlashRedirStatus == 307) {
			evalStatus = audit.StatusWarning
			observedSummary = fmt.Sprintf("Trailing-slash normalization is inconsistent: non-slash variant %s uses temporary redirect (HTTP %d) to %s.", nonSlashURL, nonSlashStatus, nonSlashFinalURL)
		} else if isNonSlash2xx && (slashStatus == 302 || slashStatus == 307 || slashRedirectStatus == 302 || slashRedirectStatus == 307) {
			evalStatus = audit.StatusWarning
			observedSummary = fmt.Sprintf("Trailing-slash normalization is inconsistent: slash variant %s uses temporary redirect (HTTP %d) to %s.", slashURL, slashStatus, slashFinalURL)
		} else {
			evalStatus = audit.StatusWarning
			observedSummary = fmt.Sprintf("Trailing-slash normalization is inconsistent between %s (HTTP %d) and %s (HTTP %d).", slashURL, slashStatus, nonSlashURL, nonSlashStatus)
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
			Scope:           audit.ScopeURL,
			ObservedSummary: observedSummary,
			ExpectedSummary: expectedSummaryCANON002,
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

// cleanURLForMatching normalizes URL scheme/host and trailing slash for reliable pair matching.
func cleanURLForMatching(raw string) string {
	raw = strings.TrimSpace(raw)
	parsed, err := url.Parse(raw)
	if err != nil {
		return strings.ToLower(raw)
	}
	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Host)
	path := parsed.Path
	if path == "" {
		path = "/"
	}
	return scheme + "://" + host + path
}
