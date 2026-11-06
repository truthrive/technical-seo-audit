package engine

import (
	"fmt"
	"sort"
	"strconv"
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

	hasRenderSignal := false
	if len(reqObs) > 0 {
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
			v := strings.ToLower(strings.TrimSpace(o.Value))
			if v == "true" || v == "1" {
				hasRenderSignal = true
			}
		}
	} else if len(renderedObs) > 0 {
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
			v := strings.ToLower(strings.TrimSpace(o.Value))
			if v == "true" || v == "1" {
				hasRenderSignal = true
			}
		}
	}

	// Also check if render_status is explicitly present
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
		v := strings.ToLower(strings.TrimSpace(o.Value))
		if v != "" {
			hasRenderSignal = true
			if v == "failed" || v == "error" || v == "timeout" {
				res.renderFailed = true
				res.renderFailedMsg = fmt.Sprintf("render execution status %q", v)
			}
		}
	}

	res.selectedForRender = hasRenderSignal
	return res
}

// splitAndNormalizeTokens parses a comma- or whitespace-separated string into a sorted, unique token slice.
func splitAndNormalizeTokens(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == '\n' || r == '\r'
	})
	tokenMap := make(map[string]struct{}, len(parts))
	for _, p := range parts {
		tok := strings.ToLower(strings.TrimSpace(p))
		if tok != "" {
			tokenMap[tok] = struct{}{}
		}
	}
	tokens := make([]string, 0, len(tokenMap))
	for t := range tokenMap {
		tokens = append(tokens, t)
	}
	sort.Strings(tokens)
	return tokens
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

// resolveSingleBoolObservation resolves a single boolean field, returning usable=false on conflict or unparseable.
func resolveSingleBoolObservation(obs []audit.NormalizedObservation) (val bool, usable bool, conflict bool) {
	s, ok, conf := resolveSingleStringObservation(obs)
	if !ok || conf {
		return false, ok, conf
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false, false, false
	}
	return b, true, false
}
