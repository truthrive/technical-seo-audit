package engine

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDINDEX003 = "AR-INDEX-003"
const expectedSummaryINDEX003 = "All observed robots directive tokens are recognized and valid."

type directiveBundle struct {
	directiveRef      string
	sources           []audit.NormalizedObservation
	raws              []audit.NormalizedObservation
	robotsTokens      []audit.NormalizedObservation
	parseErrors       []audit.NormalizedObservation
	targets           []audit.NormalizedObservation
	unsupportedTokens []audit.NormalizedObservation
	scopeUnknowns     []audit.NormalizedObservation
	tokens            []audit.NormalizedObservation
}

func (b *directiveBundle) allObservations() []audit.NormalizedObservation {
	var all []audit.NormalizedObservation
	all = append(all, b.sources...)
	all = append(all, b.targets...)
	all = append(all, b.raws...)
	all = append(all, b.tokens...)
	all = append(all, b.robotsTokens...)
	all = append(all, b.parseErrors...)
	all = append(all, b.unsupportedTokens...)
	all = append(all, b.scopeUnknowns...)
	return all
}

func isBundleIndependentlyValid(b *directiveBundle) bool {
	// 1. Explicit Bundle Identity: every observation carries the same directive:* ref
	for _, o := range b.allObservations() {
		hasRef := false
		for _, r := range o.SourceEvidenceRefs {
			if r == b.directiveRef {
				hasRef = true
				break
			}
		}
		if !hasRef {
			return false
		}
	}

	// 2. Single non-conflicting directive_source in {"META", "HTTP_HEADER"}
	if len(b.sources) == 0 {
		return false
	}
	srcVal := strings.TrimSpace(b.sources[0].Value)
	if srcVal != string(audit.DirectiveSourceMeta) && srcVal != string(audit.DirectiveSourceHTTPHeader) {
		return false
	}
	for _, s := range b.sources[1:] {
		if strings.TrimSpace(s.Value) != srcVal {
			return false
		}
	}

	// 3. Single non-conflicting directive_raw
	if len(b.raws) == 0 {
		return false
	}
	rawVal := b.raws[0].Value
	for _, r := range b.raws[1:] {
		if r.Value != rawVal {
			return false
		}
	}

	// 4. Single non-conflicting robots_directive_tokens
	if len(b.robotsTokens) == 0 {
		return false
	}
	tokVal := strings.TrimSpace(b.robotsTokens[0].Value)
	for _, t := range b.robotsTokens[1:] {
		if strings.TrimSpace(t.Value) != tokVal {
			return false
		}
	}

	// 5. Single non-conflicting robots_parse_errors
	if len(b.parseErrors) == 0 {
		return false
	}
	errVal := strings.TrimSpace(b.parseErrors[0].Value)
	if errVal == "" {
		return false
	}
	for _, p := range b.parseErrors[1:] {
		if strings.TrimSpace(p.Value) != errVal {
			return false
		}
	}

	// Validate structure of codes in errVal
	errCodes := strings.Split(errVal, ",")
	hasNone := false
	hasViolation := false
	hasAmbiguous := false
	for _, rawCode := range errCodes {
		c := strings.TrimSpace(rawCode)
		switch {
		case c == "none":
			hasNone = true
		case strings.HasPrefix(c, "parse_error:"):
			hasViolation = true
		case strings.HasPrefix(c, "unsupported_directive:"):
			hasViolation = true
		case strings.HasPrefix(c, "ambiguous:"):
			hasAmbiguous = true
		default:
			return false
		}
	}
	if hasNone && (hasViolation || hasAmbiguous) {
		return false
	}

	// 6. No internal conflict in optional fields
	if len(b.targets) > 1 {
		tgtVal := strings.TrimSpace(b.targets[0].Value)
		for _, t := range b.targets[1:] {
			if strings.TrimSpace(t.Value) != tgtVal {
				return false
			}
		}
	}
	if len(b.unsupportedTokens) > 1 {
		uVal := strings.TrimSpace(b.unsupportedTokens[0].Value)
		for _, u := range b.unsupportedTokens[1:] {
			if strings.TrimSpace(u.Value) != uVal {
				return false
			}
		}
	}
	if len(b.scopeUnknowns) > 0 {
		suVal := strings.TrimSpace(b.scopeUnknowns[0].Value)
		if suVal != "true" && suVal != "false" {
			return false
		}
		for _, s := range b.scopeUnknowns[1:] {
			if strings.TrimSpace(s.Value) != suVal {
				return false
			}
		}
	}
	if len(b.tokens) > 1 {
		dtVal := strings.TrimSpace(b.tokens[0].Value)
		for _, dt := range b.tokens[1:] {
			if strings.TrimSpace(dt.Value) != dtVal {
				return false
			}
		}
	}

	return true
}

// evaluateINDEX003 implements AR-INDEX-003: Malformed or unsupported robots directive.
// Evaluator applies the 5-tier (P1 -> P5) precedence model with bundle isolation.
func evaluateINDEX003(
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

		// 1. Resolve source URL identity (P1: Tier A)
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		var (
			urlUsable   bool
			urlConflict bool
			urlVal      string
		)
		if len(urlObs) > 0 {
			urlVal = strings.TrimSpace(urlObs[0].Value)
			if urlVal != "" {
				urlUsable = true
				for _, u := range urlObs[1:] {
					if strings.TrimSpace(u.Value) != urlVal {
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
				ExpectedSummary: expectedSummaryINDEX003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    buildURLContextRefs(),
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Reconstruct directive bundles on SubjectURL
		directiveFields := []string{
			"directive_source",
			"directive_target",
			"directive_raw",
			"directive_tokens",
			"directive_scope_unknown",
			"robots_directive_tokens",
			"robots_parse_errors",
			"directive_unsupported_tokens",
		}

		bundleMap := make(map[string]*directiveBundle)
		var unreferencedObs []audit.NormalizedObservation

		for _, field := range directiveFields {
			obsList := idx.GetObservations(audit.SubjectURL, subjectRef, field)
			for _, o := range obsList {
				var dirRef string
				for _, r := range o.SourceEvidenceRefs {
					if strings.HasPrefix(r, "directive:") {
						dirRef = r
						break
					}
				}
				if dirRef == "" {
					unreferencedObs = append(unreferencedObs, o)
					continue
				}

				b, exists := bundleMap[dirRef]
				if !exists {
					b = &directiveBundle{directiveRef: dirRef}
					bundleMap[dirRef] = b
				}

				switch field {
				case "directive_source":
					b.sources = append(b.sources, o)
				case "directive_target":
					b.targets = append(b.targets, o)
				case "directive_raw":
					b.raws = append(b.raws, o)
				case "directive_tokens":
					b.tokens = append(b.tokens, o)
				case "directive_scope_unknown":
					b.scopeUnknowns = append(b.scopeUnknowns, o)
				case "robots_directive_tokens":
					b.robotsTokens = append(b.robotsTokens, o)
				case "robots_parse_errors":
					b.parseErrors = append(b.parseErrors, o)
				case "directive_unsupported_tokens":
					b.unsupportedTokens = append(b.unsupportedTokens, o)
				}
			}
		}

		// Sort bundle keys deterministically
		var bundleKeys []string
		for k := range bundleMap {
			bundleKeys = append(bundleKeys, k)
		}
		sort.Strings(bundleKeys)

		var (
			validViolationBundles []*directiveBundle
			validAmbiguousBundles []*directiveBundle
			validCleanBundles     []*directiveBundle
			invalidBundles        []*directiveBundle
		)

		for _, k := range bundleKeys {
			b := bundleMap[k]
			if !isBundleIndependentlyValid(b) {
				invalidBundles = append(invalidBundles, b)
				continue
			}

			errVal := strings.TrimSpace(b.parseErrors[0].Value)
			isScopeUnknown := false
			if len(b.scopeUnknowns) > 0 && strings.TrimSpace(b.scopeUnknowns[0].Value) == "true" {
				isScopeUnknown = true
			}

			hasViolation := strings.Contains(errVal, "parse_error:") || strings.Contains(errVal, "unsupported_directive:")
			hasAmbiguous := strings.Contains(errVal, "ambiguous:") || isScopeUnknown

			if hasViolation {
				validViolationBundles = append(validViolationBundles, b)
			} else if hasAmbiguous {
				validAmbiguousBundles = append(validAmbiguousBundles, b)
			} else {
				validCleanBundles = append(validCleanBundles, b)
			}
		}

		// 3. Evaluate P2 (Tier C: Independently Proven Malformed or Unsupported Directive — Short-Circuit)
		if len(validViolationBundles) > 0 {
			hasSyntaxParseError := false
			var violationDetails []string
			for _, b := range validViolationBundles {
				errVal := strings.TrimSpace(b.parseErrors[0].Value)
				codes := strings.Split(errVal, ",")
				for _, c := range codes {
					trimmed := strings.TrimSpace(c)
					if strings.HasPrefix(trimmed, "parse_error:") {
						hasSyntaxParseError = true
						violationDetails = append(violationDetails, trimmed)
					} else if strings.HasPrefix(trimmed, "unsupported_directive:") {
						violationDetails = append(violationDetails, trimmed)
					}
				}
			}

			summaryPrefix := "Unsupported robots directive observed"
			if hasSyntaxParseError {
				summaryPrefix = "Malformed robots directive observed"
			}
			obsSummary := fmt.Sprintf("%s: %s.", summaryPrefix, strings.Join(violationDetails, ", "))

			// Deciding evidence: only observations of validViolationBundles
			evidenceRefs := buildURLContextRefs()
			for _, b := range validViolationBundles {
				for _, o := range b.allObservations() {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(o.ObservationID),
						Field:             o.Field,
						ObservedValue:     o.Value,
						Role:              audit.EvidenceRolePrimary,
					})
				}
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
				Status:          audit.StatusWarning,
				Severity:        audit.SeverityP1,
				Scope:           audit.ScopeURL,
				ObservedSummary: obsSummary,
				ExpectedSummary: expectedSummaryINDEX003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 4. Resolve URL-level completeness observations for P3/P4/P5
		httpHeaderStatusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "http_header_status")
		xRobotsTruncObs := idx.GetObservations(audit.SubjectURL, subjectRef, "x_robots_sample_truncated")
		httpStatusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "http_status")
		contentTypeObs := idx.GetObservations(audit.SubjectURL, subjectRef, "content_type")
		htmlStatusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "html_extraction_status")
		metaTruncObs := idx.GetObservations(audit.SubjectURL, subjectRef, "robots_meta_sample_truncated")
		renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "rendered")

		hasConflict := func(obs []audit.NormalizedObservation) bool {
			if len(obs) <= 1 {
				return false
			}
			first := strings.TrimSpace(obs[0].Value)
			for _, o := range obs[1:] {
				if strings.TrimSpace(o.Value) != first {
					return true
				}
			}
			return false
		}

		firstVal := func(obs []audit.NormalizedObservation) string {
			if len(obs) == 0 {
				return ""
			}
			return strings.TrimSpace(obs[0].Value)
		}

		urlCompletenessConflict := hasConflict(httpHeaderStatusObs) || hasConflict(xRobotsTruncObs) ||
			hasConflict(httpStatusObs) || hasConflict(contentTypeObs) ||
			hasConflict(htmlStatusObs) || hasConflict(metaTruncObs) || hasConflict(renderedObs)

		statusCode := 0
		if len(httpStatusObs) > 0 && !hasConflict(httpStatusObs) {
			statusCode, _ = strconv.Atoi(firstVal(httpStatusObs))
		}

		ctVal := firstVal(contentTypeObs)
		isHTML := strings.Contains(strings.ToLower(ctVal), "html") || ctVal == ""

		channel1Complete := !hasConflict(httpHeaderStatusObs) && firstVal(httpHeaderStatusObs) == "complete" &&
			!hasConflict(xRobotsTruncObs) && firstVal(xRobotsTruncObs) == "false" &&
			statusCode > 0 && ctVal != ""

		channel2Complete := true
		if isHTML {
			rawStatusComplete := !hasConflict(htmlStatusObs) && firstVal(htmlStatusObs) == "complete"
			renderedWithoutRaw := firstVal(renderedObs) == "true" && !rawStatusComplete
			channel2Complete = statusCode >= 200 && statusCode < 300 &&
				!renderedWithoutRaw &&
				rawStatusComplete &&
				!hasConflict(metaTruncObs) && firstVal(metaTruncObs) == "false"
		}

		// 5. Evaluate P3 (Tier B & Tier D: Invalid evidence, ambiguous scope, or incomplete channel)
		tierB := len(invalidBundles) > 0 || len(unreferencedObs) > 0
		tierD1 := len(validAmbiguousBundles) > 0
		tierD2 := urlCompletenessConflict
		tierD3 := !channel1Complete
		tierD4 := isHTML && !channel2Complete

		if tierB || tierD1 || tierD2 || tierD3 || tierD4 {
			var obsSummary string
			switch {
			case tierB || tierD2:
				obsSummary = "Robots directive evidence is incomplete, malformed, or conflicting."
			case tierD1:
				obsSummary = "Robots directive exists but user-agent prefix or scope syntax cannot be reliably verified."
			case tierD3:
				obsSummary = "Raw robots directive source is unavailable or incomplete (HTTP headers incomplete or unavailable)."
			default:
				obsSummary = "Raw robots directive source is unavailable or incomplete (HTML body extraction incomplete or rendered)."
			}

			evidenceRefs := buildURLContextRefs()
			// Attach deciding primary evidence
			for _, b := range invalidBundles {
				for _, o := range b.allObservations() {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(o.ObservationID),
						Field:             o.Field,
						ObservedValue:     o.Value,
						Role:              audit.EvidenceRolePrimary,
					})
				}
			}
			for _, o := range unreferencedObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(o.ObservationID),
					Field:             o.Field,
					ObservedValue:     o.Value,
					Role:              audit.EvidenceRolePrimary,
				})
			}
			for _, b := range validAmbiguousBundles {
				for _, o := range b.allObservations() {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(o.ObservationID),
						Field:             o.Field,
						ObservedValue:     o.Value,
						Role:              audit.EvidenceRolePrimary,
					})
				}
			}
			// Channel completeness observations
			for _, o := range httpHeaderStatusObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(o.ObservationID),
					Field:             o.Field,
					ObservedValue:     o.Value,
					Role:              audit.EvidenceRolePrimary,
				})
			}
			for _, o := range htmlStatusObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(o.ObservationID),
					Field:             o.Field,
					ObservedValue:     o.Value,
					Role:              audit.EvidenceRolePrimary,
				})
			}
			// Supporting clean bundles
			for _, b := range validCleanBundles {
				for _, o := range b.allObservations() {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(o.ObservationID),
						Field:             o.Field,
						ObservedValue:     o.Value,
						Role:              audit.EvidenceRoleSupporting,
					})
				}
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
				ExpectedSummary: expectedSummaryINDEX003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 6. Evaluate P4 (Tier E.1: Complete channels & 0 directives -> NOT_APPLICABLE)
		if len(validCleanBundles) == 0 {
			evidenceRefs := buildURLContextRefs()
			addContextObs := func(obs []audit.NormalizedObservation) {
				for _, o := range obs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(o.ObservationID),
						Field:             o.Field,
						ObservedValue:     o.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
			}
			addContextObs(httpHeaderStatusObs)
			addContextObs(xRobotsTruncObs)
			addContextObs(httpStatusObs)
			addContextObs(contentTypeObs)
			if isHTML {
				addContextObs(htmlStatusObs)
				addContextObs(metaTruncObs)
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
				ObservedSummary: "No robots directive exists.",
				ExpectedSummary: expectedSummaryINDEX003,
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 7. Evaluate P5 (Tier E.2: Complete channels & all directives clean -> PASS)
		evidenceRefs := buildURLContextRefs()
		for _, b := range validCleanBundles {
			for _, o := range b.allObservations() {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(o.ObservationID),
					Field:             o.Field,
					ObservedValue:     o.Value,
					Role:              audit.EvidenceRolePrimary,
				})
			}
		}
		addContextObs := func(obs []audit.NormalizedObservation) {
			for _, o := range obs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, o.ObservationID, o.Field)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(o.ObservationID),
					Field:             o.Field,
					ObservedValue:     o.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
		}
		addContextObs(httpHeaderStatusObs)
		addContextObs(xRobotsTruncObs)
		addContextObs(httpStatusObs)
		addContextObs(contentTypeObs)
		if isHTML {
			addContextObs(htmlStatusObs)
			addContextObs(metaTruncObs)
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
			Status:          audit.StatusPass,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeURL,
			ObservedSummary: "All observed robots directive tokens are recognized and valid.",
			ExpectedSummary: expectedSummaryINDEX003,
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
