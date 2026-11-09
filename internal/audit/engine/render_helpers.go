package engine

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

// renderPrecondition holds resolved URL identity and rendering execution state
// common to AR-RENDER-001 through AR-RENDER-006.
type renderPrecondition struct {
	urlUsable         bool
	urlConflict       bool
	urlVal            string
	urlContextRefs    []audit.RuleEvidenceRef
	selectedForRender bool
	renderFailed      bool
	renderFailedMsg   string
	renderContextRefs []audit.RuleEvidenceRef
}

// resolveRenderPrecondition evaluates whether the URL subject satisfies the
// precondition for rendering evaluation: page selected for rendering and render attempted.
func resolveRenderPrecondition(
	idx *EvidenceIndex,
	subjectRef string,
	ruleResultID audit.RuleResultID,
) renderPrecondition {
	res := renderPrecondition{}

	// 1. Resolve URL identity
	urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
	if len(urlObs) > 0 {
		firstURL := strings.TrimSpace(urlObs[0].Value)
		if firstURL != "" {
			res.urlVal = firstURL
			res.urlUsable = true
			for _, u := range urlObs[1:] {
				if strings.TrimSpace(u.Value) != firstURL {
					res.urlConflict = true
					res.urlUsable = false
					break
				}
			}
		}
	}

	for _, u := range urlObs {
		res.urlContextRefs = append(res.urlContextRefs, audit.RuleEvidenceRef{
			RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url", ruleResultID, u.ObservationID)),
			RuleResultID:      ruleResultID,
			EvidenceType:      "NORMALIZED_OBSERVATION",
			EvidenceRef:       string(u.ObservationID),
			Field:             "url",
			ObservedValue:     u.Value,
			Role:              audit.EvidenceRoleContext,
		})
	}

	if !res.urlUsable {
		return res
	}

	// 2. Check if page was selected for rendering
	reqObs := idx.GetObservations(audit.SubjectURL, subjectRef, "render_requested")
	renderedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "rendered")
	statusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "render_status")

	for _, o := range reqObs {
		res.renderContextRefs = append(res.renderContextRefs, audit.RuleEvidenceRef{
			RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:render_requested", ruleResultID, o.ObservationID)),
			RuleResultID:      ruleResultID,
			EvidenceType:      "NORMALIZED_OBSERVATION",
			EvidenceRef:       string(o.ObservationID),
			Field:             "render_requested",
			ObservedValue:     o.Value,
			Role:              audit.EvidenceRoleContext,
		})
	}
	for _, o := range renderedObs {
		res.renderContextRefs = append(res.renderContextRefs, audit.RuleEvidenceRef{
			RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:rendered", ruleResultID, o.ObservationID)),
			RuleResultID:      ruleResultID,
			EvidenceType:      "NORMALIZED_OBSERVATION",
			EvidenceRef:       string(o.ObservationID),
			Field:             "rendered",
			ObservedValue:     o.Value,
			Role:              audit.EvidenceRoleContext,
		})
	}
	for _, o := range statusObs {
		res.renderContextRefs = append(res.renderContextRefs, audit.RuleEvidenceRef{
			RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:render_status", ruleResultID, o.ObservationID)),
			RuleResultID:      ruleResultID,
			EvidenceType:      "NORMALIZED_OBSERVATION",
			EvidenceRef:       string(o.ObservationID),
			Field:             "render_status",
			ObservedValue:     o.Value,
			Role:              audit.EvidenceRoleContext,
		})
	}

	reqBool, reqUsable, reqConflict := resolveSingleBoolObservation(reqObs)
	reqMalformed := len(reqObs) > 0 && !reqUsable && !reqConflict

	renderedBool, renderedUsable, renderedConflict := resolveSingleBoolObservation(renderedObs)
	renderedMalformed := len(renderedObs) > 0 && !renderedUsable && !renderedConflict

	statusVal, statusUsable, statusConflict := resolveSingleStringObservation(statusObs)
	statusLower := strings.ToLower(statusVal)
	statusEmpty := len(statusObs) > 0 && statusUsable && statusLower == ""

	hasAnyRenderObs := len(reqObs) > 0 || len(renderedObs) > 0 || len(statusObs) > 0
	if !hasAnyRenderObs {
		res.selectedForRender = false
		return res
	}

	// Check for conflicts or malformed values in render state signals
	if reqConflict || renderedConflict || statusConflict {
		res.selectedForRender = true
		res.renderFailed = true
		res.renderFailedMsg = "conflicting render state observations"
		return res
	}
	if reqMalformed || renderedMalformed || statusEmpty {
		res.selectedForRender = true
		res.renderFailed = true
		res.renderFailedMsg = "malformed render state observation"
		return res
	}

	// Validate render_status vocabulary if present
	statusCompleted := false
	statusFailedOrIncomplete := false
	statusUnrecognized := false
	if statusUsable {
		switch statusLower {
		case "complete", "completed", "succeeded", "success", "ok", "true":
			statusCompleted = true
		case "failed", "error", "timeout", "pending", "in_progress", "skipped", "incomplete", "aborted", "cancelled":
			statusFailedOrIncomplete = true
		case "not_requested", "none", "false":
			//Explicitly not requested via status
		default:
			statusUnrecognized = true
		}
	}

	if statusUnrecognized {
		res.selectedForRender = true
		res.renderFailed = true
		res.renderFailedMsg = fmt.Sprintf("unrecognized render_status %q", statusVal)
		return res
	}

	// Detect contradictory signals across render_requested, rendered, and render_status
	if reqUsable && !reqBool {
		if (renderedUsable && renderedBool) || statusCompleted || statusFailedOrIncomplete {
			res.selectedForRender = true
			res.renderFailed = true
			res.renderFailedMsg = "contradictory render state signals (render_requested=false with active render state)"
			return res
		}
		res.selectedForRender = false
		return res
	}

	if renderedUsable && !renderedBool && statusCompleted {
		res.selectedForRender = true
		res.renderFailed = true
		res.renderFailedMsg = "contradictory render state signals (rendered=false with completed render_status)"
		return res
	}

	if reqUsable && reqBool && renderedUsable && !renderedBool && !statusFailedOrIncomplete {
		res.selectedForRender = true
		res.renderFailed = true
		res.renderFailedMsg = "render was requested (render_requested=true) but rendered=false"
		return res
	}

	// Determine if page was selected for rendering
	hasRenderSignal := (reqUsable && reqBool) || (renderedUsable && renderedBool) || statusCompleted || statusFailedOrIncomplete
	res.selectedForRender = hasRenderSignal

	if statusFailedOrIncomplete {
		res.renderFailed = true
		res.renderFailedMsg = fmt.Sprintf("render execution status %q", statusLower)
	}

	return res
}

// collapseWhitespace collapses runs of whitespace into a single space and trims leading/trailing whitespace.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// isRFCWeekdayPrefix checks whether a token ends with an RFC weekday prefix before a comma in unavailable_after.
func isRFCWeekdayPrefix(segment string) bool {
	lower := strings.ToLower(strings.TrimSpace(segment))
	idx := strings.Index(lower, "unavailable_after")
	if idx < 0 {
		return false
	}
	after := strings.TrimSpace(lower[idx+len("unavailable_after"):])
	if !strings.HasPrefix(after, ":") {
		return false
	}
	day := strings.TrimSpace(strings.TrimPrefix(after, ":"))
	switch day {
	case "mon", "tue", "wed", "thu", "fri", "sat", "sun",
		"monday", "tuesday", "wednesday", "thursday", "friday", "saturday", "sunday":
		return true
	default:
		return false
	}
}

// parseRobotsDirectiveSet parses a raw meta robots / X-Robots-Tag directive string
// into normalized lowercase tokens, preserving RFC dates in unavailable_after and
// returning ok=false if the string contains malformed directive syntax.
func parseRobotsDirectiveSet(raw string) ([]string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, true
	}
	// Reject invalid directive delimiters (semicolons or pipes)
	if strings.ContainsAny(trimmed, ";|") {
		return nil, false
	}

	rawParts := strings.Split(trimmed, ",")
	var mergedParts []string
	for i := 0; i < len(rawParts); i++ {
		part := strings.TrimSpace(rawParts[i])
		if part == "" {
			return nil, false
		}
		if isRFCWeekdayPrefix(part) && i+1 < len(rawParts) {
			nextPart := strings.TrimSpace(rawParts[i+1])
			if nextPart == "" {
				return nil, false
			}
			part = part + ", " + nextPart
			i++
		}
		mergedParts = append(mergedParts, part)
	}

	var tokens []string
	for _, p := range mergedParts {
		tok := collapseWhitespace(strings.ToLower(p))
		if strings.Contains(tok, ":") {
			colonIdx := strings.Index(tok, ":")
			lhs := strings.TrimSpace(tok[:colonIdx])
			rhs := strings.TrimSpace(tok[colonIdx+1:])
			if lhs == "" || rhs == "" || strings.ContainsAny(lhs, " \t") {
				return nil, false
			}
			tok = lhs + ":" + rhs
		} else {
			// Standalone directive token must not contain spaces
			if strings.ContainsAny(tok, " \t") {
				return nil, false
			}
		}
		tokens = append(tokens, tok)
	}

	return tokens, true
}

// parseRobotsObservationSet parses a slice of meta_robots observations into a sorted, deduplicated token set.
func parseRobotsObservationSet(obs []audit.NormalizedObservation) ([]string, bool) {
	tokenMap := make(map[string]struct{})
	for _, o := range obs {
		toks, ok := parseRobotsDirectiveSet(o.Value)
		if !ok {
			return nil, false
		}
		for _, t := range toks {
			tokenMap[t] = struct{}{}
		}
	}
	res := make([]string, 0, len(tokenMap))
	for t := range tokenMap {
		res = append(res, t)
	}
	sort.Strings(res)
	return res, true
}

// normalizeRenderURL resolves rawURL against baseURL and normalizes scheme/host case,
// default ports, and fragments while preserving case-sensitive path and query components.
func normalizeRenderURL(rawURL string, baseURL string) (string, bool) {
	s := strings.TrimSpace(rawURL)
	if s == "" || strings.ContainsAny(s, " \t\r\n") {
		return "", false
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", false
	}

	if baseURL != "" {
		base, err := url.Parse(strings.TrimSpace(baseURL))
		if err == nil && base.Scheme != "" && base.Host != "" {
			u = base.ResolveReference(u)
		}
	}

	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false
	}
	if u.Host == "" {
		return "", false
	}

	u.Scheme = scheme
	host := strings.ToLower(u.Host)
	if scheme == "http" && strings.HasSuffix(host, ":80") {
		host = strings.TrimSuffix(host, ":80")
	} else if scheme == "https" && strings.HasSuffix(host, ":443") {
		host = strings.TrimSuffix(host, ":443")
	}
	u.Host = host
	u.Fragment = ""
	u.RawFragment = ""

	if u.Path == "" {
		u.Path = "/"
	}

	return u.String(), true
}

// splitURLListString splits an observation value that may contain one URL or multiple URLs
// separated by newlines or commas (preserving commas inside a single URL path/query unless
// the comma precedes a URL or path delimiter).
func splitURLListString(raw string) ([]string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, true
	}

	lines := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == '\n' || r == '\r'
	})

	var candidates []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.Contains(line, ",") {
			candidates = append(candidates, line)
			continue
		}
		parts := strings.Split(line, ",")
		// Determine if commas are acting as list delimiters between multiple URLs
		// vs part of a single URL's path/query.
		isMultiURLList := true
		for idx, p := range parts {
			pTrim := strings.TrimSpace(p)
			if pTrim == "" {
				return nil, false
			}
			if idx > 0 {
				lowerP := strings.ToLower(pTrim)
				hasCommaDelimiter := strings.HasPrefix( p, " ") ||
					strings.HasPrefix(lowerP, "http://") ||
					strings.HasPrefix(lowerP, "https://") ||
					strings.HasPrefix(lowerP, "/")
				if !hasCommaDelimiter {
					isMultiURLList = false
					break
				}
			}
		}
		if isMultiURLList {
			for _, p := range parts {
				candidates = append(candidates, strings.TrimSpace(p))
			}
		} else {
			candidates = append(candidates, line)
		}
	}

	return candidates, true
}

// parseNormalizedURLSet parses observations containing URLs, resolves and normalizes each URL,
// and returns a sorted, deduplicated slice of normalized URLs. Returns ok=false if any URL is invalid.
func parseNormalizedURLSet(obs []audit.NormalizedObservation, baseURL string) ([]string, bool) {
	urlMap := make(map[string]struct{})
	for _, o := range obs {
		rawURLs, ok := splitURLListString(o.Value)
		if !ok {
			return nil, false
		}
		for _, r := range rawURLs {
			norm, ok := normalizeRenderURL(r, baseURL)
			if !ok {
				return nil, false
			}
			urlMap[norm] = struct{}{}
		}
	}
	res := make([]string, 0, len(urlMap))
	for u := range urlMap {
		res = append(res, u)
	}
	sort.Strings(res)
	return res, true
}

// parseNormalizedH1Set parses H1 observations without splitting on commas (preserving commas
// and case in H1 heading text while collapsing whitespace runs).
func parseNormalizedH1Set(obs []audit.NormalizedObservation) []string {
	h1Map := make(map[string]struct{})
	for _, o := range obs {
		lines := strings.FieldsFunc(o.Value, func(r rune) bool {
			return r == '\n' || r == '\r'
		})
		for _, line := range lines {
			norm := collapseWhitespace(line)
			if norm != "" {
				h1Map[norm] = struct{}{}
			}
		}
	}
	res := make([]string, 0, len(h1Map))
	for h := range h1Map {
		res = append(res, h)
	}
	sort.Strings(res)
	return res
}

// slicesEqual returns true if two sorted slices of strings are identical.
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// setDifference returns elements present in a but missing from b.
func setDifference(a, b []string) []string {
	bMap := make(map[string]struct{}, len(b))
	for _, x := range b {
		bMap[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, ok := bMap[x]; !ok {
			diff = append(diff, x)
		}
	}
	sort.Strings(diff)
	return diff
}

// resolveSingleStringObservation resolves a single string field, returning usable=false on conflict.
func resolveSingleStringObservation(obs []audit.NormalizedObservation) (val string, usable bool, conflict bool) {
	if len(obs) == 0 {
		return "", false, false
	}
	first := strings.TrimSpace(obs[0].Value)
	for _, o := range obs[1:] {
		if strings.TrimSpace(o.Value) != first {
			return "", false, true
		}
	}
	return first, true, false
}

// resolveSingleBoolObservation resolves a single boolean field ("true" or "false"),
// returning usable=false on conflict or non-boolean value.
func resolveSingleBoolObservation(obs []audit.NormalizedObservation) (val bool, usable bool, conflict bool) {
	if len(obs) == 0 {
		return false, false, false
	}
	first := strings.ToLower(strings.TrimSpace(obs[0].Value))
	for _, o := range obs[1:] {
		if strings.ToLower(strings.TrimSpace(o.Value)) != first {
			return false, false, true
		}
	}
	switch first {
	case "true":
		return true, true, false
	case "false":
		return false, true, false
	default:
		return false, false, false
	}
}
