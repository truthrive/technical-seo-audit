package engine

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDCANON011 = "AR-CANON-011"

// evaluateCANON011 implements the typed execution logic for:
// AR-CANON-011 — Pagination exposes crawlable next-page link (parent check: CANON-006, P1, assisted).
//
// Evaluates observed audit.SubjectURL entities against normalized pagination evidence:
// - pagination_detected
// - has_next_page
// - next_page_url
// - next_page_element_tag
// - next_page_href
//
// Contract (per frozen manifest rules_v1.json):
// - NOT_APPLICABLE:
//   - pagination_detected == "false"
//   - pagination_detected == "true" and has_next_page == "false" (final page)
// - PASS:
//   - pagination_detected == "true" and has_next_page == "true", and next page is exposed
//     through a crawlable <a href> resolving to a valid HTTP(S) URL.
// - WARNING:
//   - pagination_detected == "true" and has_next_page == "true", and next page depends only
//     on script/event/infinite-scroll interaction (e.g., element tag is not "a", or href is
//     missing, empty, "#", or "javascript:...").
// - UNKNOWN:
//   - pagination/link evidence unavailable, conflicting, or malformed.
// - FAIL and MANUAL_REVIEW are not used.
func evaluateCANON011(
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

	subjectRefs := idx.SubjectRefs(audit.SubjectURL)
	results := make([]audit.RuleResult, 0, len(subjectRefs))
	expectedSummary := "Paginated sequence exposes next page through a crawlable <a href> link."

	for _, subjectRef := range subjectRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		urlIdentityObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url")
		if len(urlIdentityObs) == 0 && len(urlObs) > 0 {
			urlIdentityObs = urlObs
		}

		pagDetectedObs := idx.GetObservations(audit.SubjectURL, subjectRef, "pagination_detected")
		hasNextObs := idx.GetObservations(audit.SubjectURL, subjectRef, "has_next_page")
		nextURLObs := idx.GetObservations(audit.SubjectURL, subjectRef, "next_page_url")
		nextTagObs := idx.GetObservations(audit.SubjectURL, subjectRef, "next_page_element_tag")
		nextHrefObs := idx.GetObservations(audit.SubjectURL, subjectRef, "next_page_href")

		buildAllRefs := func(primaryField string) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			roleFor := func(f string) audit.EvidenceRole {
				if f == primaryField {
					return audit.EvidenceRolePrimary
				}
				return audit.EvidenceRoleSupporting
			}
			refs = append(refs, buildEvidenceRefs(ruleResultID, pagDetectedObs, "pagination_detected", roleFor("pagination_detected"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, hasNextObs, "has_next_page", roleFor("has_next_page"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, nextURLObs, "next_page_url", roleFor("next_page_url"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, nextTagObs, "next_page_element_tag", roleFor("next_page_element_tag"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, nextHrefObs, "next_page_href", roleFor("next_page_href"))...)
			return refs
		}

		// 1. Validate URL identity
		if len(urlIdentityObs) == 0 {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Required URL identity observation is missing.", expectedSummary, evalTime, buildAllRefs("")))
			continue
		}
		_, urlUsable, urlConflict := extractSingleObsValue(urlIdentityObs)
		if urlConflict || !urlUsable {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "URL identity observation is conflicting or malformed.", expectedSummary, evalTime, buildAllRefs("")))
			continue
		}

		// 2. Validate pagination_detected
		if len(pagDetectedObs) == 0 {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Pagination detection evidence (pagination_detected) is unavailable.", expectedSummary, evalTime, buildAllRefs("")))
			continue
		}
		pagVal, pagUsable, pagConflict := extractSingleObsValue(pagDetectedObs)
		if pagConflict || !pagUsable {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed pagination_detected observation.", expectedSummary, evalTime, buildAllRefs("pagination_detected")))
			continue
		}
		pagLower := strings.ToLower(pagVal)
		if pagLower != "true" && pagLower != "false" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("pagination_detected value %q is malformed; expected 'true' or 'false'.", pagVal), expectedSummary, evalTime, buildAllRefs("pagination_detected")))
			continue
		}

		if pagLower == "false" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusNotApplicable, "No pagination detected on audited URL.", expectedSummary, evalTime, buildAllRefs("pagination_detected")))
			continue
		}

		// 3. Validate has_next_page
		if len(hasNextObs) == 0 {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Pagination next-page status (has_next_page) is unavailable.", expectedSummary, evalTime, buildAllRefs("pagination_detected")))
			continue
		}
		hasNextVal, hasNextUsable, hasNextConflict := extractSingleObsValue(hasNextObs)
		if hasNextConflict || !hasNextUsable {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed has_next_page observation.", expectedSummary, evalTime, buildAllRefs("has_next_page")))
			continue
		}
		hasNextLower := strings.ToLower(hasNextVal)
		if hasNextLower != "true" && hasNextLower != "false" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("has_next_page value %q is malformed; expected 'true' or 'false'.", hasNextVal), expectedSummary, evalTime, buildAllRefs("has_next_page")))
			continue
		}

		if hasNextLower == "false" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusNotApplicable, "Audited URL is the final page of the paginated sequence (has_next_page = false).", expectedSummary, evalTime, buildAllRefs("has_next_page")))
			continue
		}

		// 4. Check for conflicting observations on next_page_element_tag, next_page_href, next_page_url
		tagVal, _, tagConflict := extractRawObsValue(nextTagObs)
		hrefVal, _, hrefConflict := extractRawObsValue(nextHrefObs)
		nextURLVal, _, nextURLConflict := extractRawObsValue(nextURLObs)
		if tagConflict || hrefConflict || nextURLConflict {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting next-page element or link evidence on paginated URL.", expectedSummary, evalTime, buildAllRefs("next_page_element_tag")))
			continue
		}

		// If neither element tag nor href nor next_page_url was captured at all -> UNKNOWN (evidence unavailable)
		if len(nextTagObs) == 0 && len(nextHrefObs) == 0 && len(nextURLObs) == 0 {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Next-page element and href evidence are unavailable for paginated URL.", expectedSummary, evalTime, buildAllRefs("has_next_page")))
			continue
		}

		tagLower := strings.ToLower(strings.TrimSpace(tagVal))
		hrefTrimmed := strings.TrimSpace(hrefVal)
		nextURLTrimmed := strings.TrimSpace(nextURLVal)

		// If element tag is explicitly non-anchor (e.g. "button", "span", "div") -> WARNING
		if tagLower != "a" {
			summary := fmt.Sprintf("Paginated next page control uses non-anchor element <%s> instead of a crawlable <a href>.", tagLower)
			if tagLower == "" {
				summary = "Paginated next page is not exposed through a body <a href> anchor element."
			}
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, summary, expectedSummary, evalTime, buildAllRefs("next_page_element_tag")))
			continue
		}

		// Element tag is "a": check href and resolved URL
		hrefLower := strings.ToLower(hrefTrimmed)
		if hrefTrimmed == "" || strings.HasPrefix(hrefLower, "#") || strings.HasPrefix(hrefLower, "javascript:") {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, fmt.Sprintf("Paginated next-page <a> element lacks a crawlable href (%q); relies on script or event interaction.", hrefTrimmed), expectedSummary, evalTime, buildAllRefs("next_page_href")))
			continue
		}

		// Validate resolved next_page_url (or href if already absolute HTTP(S))
		candidateURL := nextURLTrimmed
		if candidateURL == "" {
			candidateURL = hrefTrimmed
		}
		parsedNext, err := url.Parse(candidateURL)
		if err != nil || parsedNext.Host == "" || (strings.ToLower(parsedNext.Scheme) != "http" && strings.ToLower(parsedNext.Scheme) != "https") {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, fmt.Sprintf("Paginated next-page <a href=%q> does not resolve to a crawlable HTTP(S) URL.", hrefTrimmed), expectedSummary, evalTime, buildAllRefs("next_page_href")))
			continue
		}

		results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, fmt.Sprintf("Paginated next page is exposed through a crawlable <a href> link (%s).", candidateURL), expectedSummary, evalTime, buildAllRefs("next_page_href")))
	}

	return results, nil
}
