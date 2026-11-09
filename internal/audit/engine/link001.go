package engine

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDLINK001 = "AR-LINK-001"

// evaluateLINK001 implements the typed execution logic for:
// AR-LINK-001 — Internal navigation target lacks crawlable href (parent check: LINK-001, P1, assisted).
//
// Evaluates observed audit.SubjectLink entities against normalized navigation link evidence:
// - source_url / link_source_url
// - navigation_candidate
// - resolved_internal_target / link_target
// - element_tag
// - href
//
// Contract (per frozen manifest rules_v1.json):
// - NOT_APPLICABLE:
//   - navigation_candidate == "false" or verified external link (link_is_internal == "false").
// - PASS:
//   - navigation_candidate == "true", element_tag == "a", and href / resolved_internal_target
//     is a usable crawlable HTTP(S) URL.
// - WARNING:
//   - navigation_candidate == "true" with a resolvable internal target, but element_tag != "a"
//     (e.g. button/span/div) or href is missing, empty, "#", or "javascript:...".
// - UNKNOWN:
//   - source URL, navigation_candidate, element_tag, or target cannot be determined or conflicts.
// - FAIL and MANUAL_REVIEW are not used.
func evaluateLINK001(
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

	linkRefs := idx.SubjectRefs(audit.SubjectLink)
	results := make([]audit.RuleResult, 0, len(linkRefs))
	expectedSummary := "Internal navigation candidate is represented by an <a href> element with a crawlable URL."

	for _, subjectRef := range linkRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		sourceURLObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_source_url")
		if len(sourceURLObs) == 0 {
			sourceURLObs = idx.GetObservations(audit.SubjectLink, subjectRef, "source_url")
		}
		isInternalObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_is_internal")
		navCandObs := idx.GetObservations(audit.SubjectLink, subjectRef, "navigation_candidate")
		targetObs := idx.GetObservations(audit.SubjectLink, subjectRef, "resolved_internal_target")
		if len(targetObs) == 0 {
			targetObs = idx.GetObservations(audit.SubjectLink, subjectRef, "link_target")
		}
		tagObs := idx.GetObservations(audit.SubjectLink, subjectRef, "element_tag")
		hrefObs := idx.GetObservations(audit.SubjectLink, subjectRef, "href")
		locationObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_location")

		buildRefs := func(primaryField string) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			roleFor := func(f string) audit.EvidenceRole {
				if f == primaryField {
					return audit.EvidenceRolePrimary
				}
				return audit.EvidenceRoleContext
			}
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceURLObs, "source_url", roleFor("source_url"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", roleFor("link_is_internal"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, navCandObs, "navigation_candidate", roleFor("navigation_candidate"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetObs, "resolved_internal_target", roleFor("resolved_internal_target"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, tagObs, "element_tag", roleFor("element_tag"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, hrefObs, "href", roleFor("href"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, locationObs, "link_location", audit.EvidenceRoleContext)...)
			return refs
		}

		// 1. Validate source_url
		srcVal, srcUsable, srcConflict := extractSingleObsValue(sourceURLObs)
		if !srcUsable || srcConflict || srcVal == "" {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Required link source URL evidence is missing, conflicting, or malformed.", expectedSummary, evalTime, buildRefs("source_url")))
			continue
		}

		// 2. Check link_is_internal if present
		if len(isInternalObs) > 0 {
			intVal, intUsable, intConflict := extractSingleObsValue(isInternalObs)
			if !intUsable || intConflict {
				results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed link_is_internal evidence.", expectedSummary, evalTime, buildRefs("link_is_internal")))
				continue
			}
			intLower := strings.ToLower(intVal)
			if intLower != "true" && intLower != "false" {
				results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("Malformed link_is_internal value %q.", intVal), expectedSummary, evalTime, buildRefs("link_is_internal")))
				continue
			}
			if intLower == "false" {
				results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusNotApplicable, "Link is verified external; internal navigation crawlable href rule is not applicable.", expectedSummary, evalTime, buildRefs("link_is_internal")))
				continue
			}
		}

		// 3. Validate navigation_candidate
		if len(navCandObs) == 0 {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Navigation candidate classification (navigation_candidate) is unavailable.", expectedSummary, evalTime, buildRefs("navigation_candidate")))
			continue
		}
		navVal, navUsable, navConflict := extractSingleObsValue(navCandObs)
		if !navUsable || navConflict {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed navigation_candidate evidence.", expectedSummary, evalTime, buildRefs("navigation_candidate")))
			continue
		}
		navLower := strings.ToLower(navVal)
		if navLower != "true" && navLower != "false" {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("Malformed navigation_candidate value %q; expected 'true' or 'false'.", navVal), expectedSummary, evalTime, buildRefs("navigation_candidate")))
			continue
		}
		if navLower == "false" {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusNotApplicable, "Element is not an internal navigation candidate (navigation_candidate = false).", expectedSummary, evalTime, buildRefs("navigation_candidate")))
			continue
		}

		// 4. Validate resolved_internal_target
		targetVal, targetUsable, targetConflict := extractSingleObsValue(targetObs)
		if !targetUsable || targetConflict || targetVal == "" {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Resolved internal navigation target is missing, conflicting, or malformed.", expectedSummary, evalTime, buildRefs("resolved_internal_target")))
			continue
		}

		// 5. Validate element_tag
		if len(tagObs) == 0 {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Navigation element tag (element_tag) is unavailable.", expectedSummary, evalTime, buildRefs("element_tag")))
			continue
		}
		tagVal, tagUsable, tagConflict := extractSingleObsValue(tagObs)
		if !tagUsable || tagConflict {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed element_tag evidence.", expectedSummary, evalTime, buildRefs("element_tag")))
			continue
		}
		tagLower := strings.ToLower(tagVal)

		// 6. Validate href (allow empty string to be classified as WARNING when present or absent on a non-a or broken-a tag)
		hrefVal, _, hrefConflict := extractRawObsValue(hrefObs)
		if hrefConflict {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting href observations on navigation candidate.", expectedSummary, evalTime, buildRefs("href")))
			continue
		}
		hrefTrimmed := strings.TrimSpace(hrefVal)

		if tagLower != "a" {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, fmt.Sprintf("Internal navigation candidate targeting %q uses <%s> instead of a crawlable <a href>.", targetVal, tagLower), expectedSummary, evalTime, buildRefs("element_tag")))
			continue
		}

		hrefLower := strings.ToLower(hrefTrimmed)
		if hrefTrimmed == "" || strings.HasPrefix(hrefLower, "#") || strings.HasPrefix(hrefLower, "javascript:") {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, fmt.Sprintf("Internal navigation <a> candidate targeting %q lacks a crawlable href (%q).", targetVal, hrefTrimmed), expectedSummary, evalTime, buildRefs("href")))
			continue
		}

		parsedTarget, err := url.Parse(targetVal)
		if err != nil || parsedTarget.Host == "" || (strings.ToLower(parsedTarget.Scheme) != "http" && strings.ToLower(parsedTarget.Scheme) != "https") {
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, fmt.Sprintf("Internal navigation <a href=%q> does not resolve to a usable HTTP(S) URL.", hrefTrimmed), expectedSummary, evalTime, buildRefs("href")))
			continue
		}

		results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, fmt.Sprintf("Internal navigation target %q is represented by an <a href> with a crawlable URL.", targetVal), expectedSummary, evalTime, buildRefs("href")))
	}

	return results, nil
}
