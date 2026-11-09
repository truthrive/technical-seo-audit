package engine

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDACC007 = "AR-ACC-007"

// evaluateACC007 implements the typed execution logic for:
// AR-ACC-007 — Blocked CSS/JS resource affects rendered output (parent check: ACC-004, P1, assisted).
//
// Evaluates observed audit.SubjectResource entities (or audit.SubjectURL entities when resource
// subjects are absent) against normalized resource robots and render dependency evidence:
// - page_url / url_identity
// - resource_url
// - resource_type
// - resource_robots_allowed
// - render_dependency_observed
// - render_diff_evidence
//
// Contract (per frozen manifest rules_v1.json):
// - NOT_APPLICABLE:
//   - Page not selected for rendering or resource is not a CSS/JS render resource (or URL has
//     no relevant CSS/JS resource).
// - PASS:
//   - Resource is allowed by robots.txt (resource_robots_allowed == "true"), OR
//   - Resource is blocked by robots.txt (resource_robots_allowed == "false") and
//     render_dependency_observed == "false" (no material dependency).
// - WARNING:
//   - Resource is blocked by robots.txt (resource_robots_allowed == "false"), but material impact
//     cannot be established (e.g., render_dependency_observed is "unknown"/missing, or
//     render_dependency_observed == "true" without direct render_diff_evidence).
// - FAIL:
//   - Resource is blocked by robots.txt (resource_robots_allowed == "false"),
//     render_dependency_observed == "true", and direct render_diff_evidence proves loss or change
//     of audited main content or metadata.
// - UNKNOWN:
//   - Missing, conflicting, or malformed resource/robots/render evidence.
func evaluateACC007(
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

	subjectType := audit.SubjectResource
	subjectRefs := idx.SubjectRefs(audit.SubjectResource)
	if len(subjectRefs) == 0 {
		subjectType = audit.SubjectURL
		subjectRefs = idx.SubjectRefs(audit.SubjectURL)
	}

	results := make([]audit.RuleResult, 0, len(subjectRefs))
	expectedSummary := "CSS/JS resources required for rendering audited content are allowed by robots.txt or do not cause material rendered output loss."

	makeRes := func(subjectRef string, ruleResultID audit.RuleResultID, status audit.RuleResultStatus, summary string, refs []audit.RuleEvidenceRef) audit.RuleResult {
		return audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     subjectType,
			SubjectRef:      subjectRef,
			Status:          status,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeURL,
			ObservedSummary: summary,
			ExpectedSummary: expectedSummary,
			EvaluatedAt:     evalTime,
			EvidenceRefs:    dedupeAndSortEvidenceRefs(refs),
		}
	}

	for _, subjectRef := range subjectRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		pageURLObs := idx.GetObservations(subjectType, subjectRef, "page_url")
		if len(pageURLObs) == 0 {
			pageURLObs = idx.GetObservations(subjectType, subjectRef, "url_identity")
		}
		if len(pageURLObs) == 0 {
			pageURLObs = idx.GetObservations(subjectType, subjectRef, "url")
		}

		resURLObs := idx.GetObservations(subjectType, subjectRef, "resource_url")
		resTypeObs := idx.GetObservations(subjectType, subjectRef, "resource_type")
		resRobotsObs := idx.GetObservations(subjectType, subjectRef, "resource_robots_allowed")
		depObs := idx.GetObservations(subjectType, subjectRef, "render_dependency_observed")
		diffObs := idx.GetObservations(subjectType, subjectRef, "render_diff_evidence")
		renderReqObs := idx.GetObservations(subjectType, subjectRef, "render_requested")
		renderedObs := idx.GetObservations(subjectType, subjectRef, "rendered")

		buildRefs := func(primaryField string) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			roleFor := func(f string) audit.EvidenceRole {
				if f == primaryField {
					return audit.EvidenceRolePrimary
				}
				return audit.EvidenceRoleSupporting
			}
			refs = append(refs, buildEvidenceRefs(ruleResultID, pageURLObs, "page_url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, resURLObs, "resource_url", roleFor("resource_url"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, resTypeObs, "resource_type", roleFor("resource_type"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, resRobotsObs, "resource_robots_allowed", roleFor("resource_robots_allowed"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, depObs, "render_dependency_observed", roleFor("render_dependency_observed"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, diffObs, "render_diff_evidence", roleFor("render_diff_evidence"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, renderReqObs, "render_requested", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, renderedObs, "rendered", audit.EvidenceRoleContext)...)
			return refs
		}

		// 1. Validate page_url
		if len(pageURLObs) == 0 {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Required page_url identity observation is missing.", buildRefs("")))
			continue
		}
		_, pageUsable, pageConflict := extractSingleObsValue(pageURLObs)
		if pageConflict || !pageUsable {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed page_url identity observation.", buildRefs("")))
			continue
		}

		// 2. If evaluating on SubjectURL without resource observations:
		//    - If page was not selected for rendering (render_requested == "false" or no render/resource evidence), return NOT_APPLICABLE.
		if subjectType == audit.SubjectURL && len(resURLObs) == 0 && len(resTypeObs) == 0 && len(resRobotsObs) == 0 {
			if len(renderReqObs) > 0 {
				reqVal, reqUsable, reqConflict := extractSingleObsValue(renderReqObs)
				if reqConflict || !reqUsable {
					results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed render_requested observation.", buildRefs("render_requested")))
					continue
				}
				if strings.ToLower(reqVal) == "false" {
					results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusNotApplicable, "Page was not selected for rendering.", buildRefs("render_requested")))
					continue
				}
			} else if len(renderedObs) == 0 {
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusNotApplicable, "Page was not selected for rendering and has no CSS/JS render resource observations.", buildRefs("")))
				continue
			}
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "CSS/JS resource robots and render dependency evidence is unavailable for rendered page.", buildRefs("")))
			continue
		}

		// Check if render_requested is explicitly false
		if len(renderReqObs) > 0 {
			reqVal, reqUsable, reqConflict := extractSingleObsValue(renderReqObs)
			if reqConflict || !reqUsable {
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed render_requested observation.", buildRefs("render_requested")))
				continue
			}
			if strings.ToLower(reqVal) == "false" {
				results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusNotApplicable, "Containing page was not selected for rendering.", buildRefs("render_requested")))
				continue
			}
		}

		// 3. Validate resource_type
		if len(resTypeObs) == 0 {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Required resource_type observation is missing.", buildRefs("resource_type")))
			continue
		}
		resTypeVal, resTypeUsable, resTypeConflict := extractSingleObsValue(resTypeObs)
		if resTypeConflict || !resTypeUsable {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed resource_type observation.", buildRefs("resource_type")))
			continue
		}
		resTypeUpper := strings.ToUpper(resTypeVal)
		if resTypeUpper != "CSS" && resTypeUpper != "STYLE" && resTypeUpper != "STYLESHEET" &&
			resTypeUpper != "JS" && resTypeUpper != "JAVASCRIPT" && resTypeUpper != "SCRIPT" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusNotApplicable, fmt.Sprintf("Resource type %q is not a CSS or JS render resource.", resTypeVal), buildRefs("resource_type")))
			continue
		}

		// 4. Validate resource_url
		resURLVal, resURLUsable, resURLConflict := extractSingleObsValue(resURLObs)
		if !resURLUsable || resURLConflict || resURLVal == "" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Required resource_url observation is missing, conflicting, or malformed.", buildRefs("resource_url")))
			continue
		}

		// 5. Validate resource_robots_allowed
		if len(resRobotsObs) == 0 {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Required resource_robots_allowed observation is missing.", buildRefs("resource_robots_allowed")))
			continue
		}
		robotsVal, robotsUsable, robotsConflict := extractSingleObsValue(resRobotsObs)
		if !robotsUsable || robotsConflict {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed resource_robots_allowed observation.", buildRefs("resource_robots_allowed")))
			continue
		}
		robotsLower := strings.ToLower(robotsVal)
		if robotsLower == "unknown" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Resource robots.txt evaluation is unknown.", buildRefs("resource_robots_allowed")))
			continue
		}
		if robotsLower != "true" && robotsLower != "false" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("Malformed resource_robots_allowed value %q; expected 'true' or 'false'.", robotsVal), buildRefs("resource_robots_allowed")))
			continue
		}

		// If resource is allowed by robots.txt -> PASS
		if robotsLower == "true" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusPass, fmt.Sprintf("%s resource %q is allowed by robots.txt.", resTypeUpper, resURLVal), buildRefs("resource_robots_allowed")))
			continue
		}

		// Resource is blocked by robots.txt (resource_robots_allowed == "false"):
		// Inspect render_dependency_observed and render_diff_evidence
		if len(depObs) == 0 {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusWarning, fmt.Sprintf("%s resource %q is blocked by robots.txt, but render_dependency_observed evidence is unavailable to establish material rendering impact.", resTypeUpper, resURLVal), buildRefs("resource_robots_allowed")))
			continue
		}
		depVal, depUsable, depConflict := extractSingleObsValue(depObs)
		if depConflict || !depUsable {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed render_dependency_observed observation.", buildRefs("render_dependency_observed")))
			continue
		}
		depLower := strings.ToLower(depVal)
		if depLower == "unknown" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusWarning, fmt.Sprintf("%s resource %q is blocked by robots.txt, and material rendering impact is unknown.", resTypeUpper, resURLVal), buildRefs("resource_robots_allowed")))
			continue
		}
		if depLower != "true" && depLower != "false" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("Malformed render_dependency_observed value %q.", depVal), buildRefs("render_dependency_observed")))
			continue
		}

		if depLower == "false" {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusPass, fmt.Sprintf("%s resource %q is blocked by robots.txt, but no material dependency on audited rendered output was observed.", resTypeUpper, resURLVal), buildRefs("render_dependency_observed")))
			continue
		}

		// render_dependency_observed == "true": check render_diff_evidence
		diffVal, diffUsable, diffConflict := extractRawObsValue(diffObs)
		if diffConflict {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting render_diff_evidence observations.", buildRefs("render_diff_evidence")))
			continue
		}
		diffTrimmed := strings.TrimSpace(diffVal)
		if !diffUsable || diffTrimmed == "" || strings.EqualFold(diffTrimmed, "none") || strings.EqualFold(diffTrimmed, "unknown") {
			results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusWarning, fmt.Sprintf("%s resource %q is blocked by robots.txt and flagged as a render dependency, but direct render diff evidence is unavailable to prove material loss of main content or metadata.", resTypeUpper, resURLVal), buildRefs("render_dependency_observed")))
			continue
		}

		results = append(results, makeRes(subjectRef, ruleResultID, audit.StatusFail, fmt.Sprintf("%s resource %q is blocked by robots.txt and direct render evidence proves material impact on rendered output (%s).", resTypeUpper, resURLVal, diffTrimmed), buildRefs("render_diff_evidence")))
	}

	return results, nil
}
