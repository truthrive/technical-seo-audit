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
	ruleIDLINK002 = "AR-LINK-002"
	ruleIDLINK003 = "AR-LINK-003"
	ruleIDLINK004 = "AR-LINK-004"
)

// evaluateLINK002 implements the typed execution logic for:
// AR-LINK-002 — Internal link targets 3xx (parent check: LINK-002, P1, deterministic).
func evaluateLINK002(
	ctx context.Context,
	rule audit.RuleDefinition,
	idx *EvidenceIndex,
	policies *PolicyIndex,
	snapshot *audit.EvidenceSnapshot,
	evalTime time.Time,
) ([]audit.RuleResult, error) {
	return evaluateLinkTargetStatus(ctx, rule, idx, snapshot, evalTime, 3)
}

// evaluateLINK003 implements the typed execution logic for:
// AR-LINK-003 — Internal link targets 4xx (parent check: LINK-002, P1, deterministic).
func evaluateLINK003(
	ctx context.Context,
	rule audit.RuleDefinition,
	idx *EvidenceIndex,
	policies *PolicyIndex,
	snapshot *audit.EvidenceSnapshot,
	evalTime time.Time,
) ([]audit.RuleResult, error) {
	return evaluateLinkTargetStatus(ctx, rule, idx, snapshot, evalTime, 4)
}

// evaluateLINK004 implements the typed execution logic for:
// AR-LINK-004 — Internal link targets 5xx (parent check: LINK-002, P1, deterministic).
func evaluateLINK004(
	ctx context.Context,
	rule audit.RuleDefinition,
	idx *EvidenceIndex,
	policies *PolicyIndex,
	snapshot *audit.EvidenceSnapshot,
	evalTime time.Time,
) ([]audit.RuleResult, error) {
	return evaluateLinkTargetStatus(ctx, rule, idx, snapshot, evalTime, 5)
}

// evaluateLinkTargetStatus provides the shared, deterministic evaluation logic for
// internal link target status rules (AR-LINK-002, AR-LINK-003, AR-LINK-004).
//
// Target class parameter:
// - 3: tests for 3xx redirect status (300 <= status <= 399)
// - 4: tests for 4xx client error status (400 <= status <= 499)
// - 5: tests for 5xx server error status (500 <= status <= 599)
func evaluateLinkTargetStatus(
	ctx context.Context,
	rule audit.RuleDefinition,
	idx *EvidenceIndex,
	snapshot *audit.EvidenceSnapshot,
	evalTime time.Time,
	targetClass int,
) ([]audit.RuleResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	linkRefs := idx.SubjectRefs(audit.SubjectLink)
	results := make([]audit.RuleResult, 0, len(linkRefs))

	expectedURLPrefix := fmt.Sprintf("url:%s:", snapshot.AuditRunID)

	var expectedSummary string
	switch targetClass {
	case 3:
		expectedSummary = "Internal link target is not 3xx."
	case 4:
		expectedSummary = "Internal link target is not 4xx."
	case 5:
		expectedSummary = "Internal link target is not 5xx."
	default:
		expectedSummary = "Internal link target status is valid."
	}

	for _, subjectRef := range linkRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// 1. Collect all SubjectLink observations
		sourceURLObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_source_url")
		sourceRefObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_source_subject_ref")
		isInternalObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_is_internal")
		targetURLObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_target")
		targetRefObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_target_subject_ref")
		anchorObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_anchor")
		locationObs := idx.GetObservations(audit.SubjectLink, subjectRef, "link_location")

		// Helper to build context evidence refs for all link-level observations
		buildLinkContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceURLObs, "link_source_url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceRefObs, "link_source_subject_ref", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetURLObs, "link_target", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetRefObs, "link_target_subject_ref", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, anchorObs, "link_anchor", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, locationObs, "link_location", audit.EvidenceRoleContext)...)
			return refs
		}

		// 2. Check internal classification
		isInternalVal, isInternalUsable, isInternalConflict := extractSingleObsValue(isInternalObs)
		if !isInternalUsable {
			var summary string
			if len(isInternalObs) == 0 {
				summary = "Required internal link classification evidence (link_is_internal) is unavailable."
			} else if isInternalConflict {
				summary = "Conflicting internal link classification evidence (link_is_internal)."
			} else {
				summary = "Malformed internal link classification evidence (link_is_internal)."
			}
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		isInternalValLower := strings.ToLower(isInternalVal)
		if isInternalValLower != "true" && isInternalValLower != "false" {
			summary := fmt.Sprintf("Internal link classification value %q is malformed; expected 'true' or 'false'.", isInternalVal)
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// Verified external link -> NOT_APPLICABLE
		if isInternalValLower == "false" {
			summary := "Link is verified external (link_is_internal = false); internal link target status rule is not applicable."
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRolePrimary)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusNotApplicable, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 3. Check target URL evidence and scheme
		targetURLVal, targetURLUsable, targetURLConflict := extractSingleObsValue(targetURLObs)
		if !targetURLUsable {
			var summary string
			if len(targetURLObs) == 0 {
				summary = "Required target URL evidence (link_target) is unavailable."
			} else if targetURLConflict {
				summary = "Conflicting target URL evidence (link_target)."
			} else {
				summary = "Malformed target URL evidence (link_target)."
			}
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		isNonHTTP, scheme := checkNonHTTPScheme(targetURLVal)
		if isNonHTTP {
			summary := fmt.Sprintf("Link target URL %q has non-HTTP scheme %q; internal HTTP link rule is not applicable.", targetURLVal, scheme)
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetURLObs, "link_target", audit.EvidenceRolePrimary)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusNotApplicable, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 4. Check source URL identity and source subject reference
		sourceURLVal, sourceURLUsable, sourceURLConflict := extractSingleObsValue(sourceURLObs)
		sourceRefVal, sourceRefUsable, sourceRefConflict := extractSingleObsValue(sourceRefObs)
		if !sourceURLUsable || !sourceRefUsable {
			var summary string
			if len(sourceURLObs) == 0 || len(sourceRefObs) == 0 {
				summary = "Required link source identity evidence is unavailable."
			} else if sourceURLConflict || sourceRefConflict {
				summary = "Link source identity evidence is conflicting."
			} else {
				summary = "Link source identity evidence is malformed."
			}
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		sourceValidation := validateReferencedURLSubject(idx, snapshot, "Source", sourceRefVal, sourceURLVal, expectedURLPrefix)
		if !sourceValidation.valid {
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			if len(sourceValidation.urlIdentityObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, sourceValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			}
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, sourceValidation.summary, expectedSummary, evalTime, refs))
			continue
		}

		// 5. Check target subject reference correlation
		targetRefVal, targetRefUsable, targetRefConflict := extractSingleObsValue(targetRefObs)
		if !targetRefUsable {
			var summary string
			if len(targetRefObs) == 0 {
				summary = "Target URL subject correlation is unavailable (e.g. dst_id = 0, unvisited, or unallocated target)."
			} else if targetRefConflict {
				summary = "Target URL subject correlation is conflicting."
			} else {
				summary = "Target URL subject correlation is malformed."
			}
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		targetValidation := validateReferencedURLSubject(idx, snapshot, "Target", targetRefVal, targetURLVal, expectedURLPrefix)
		if !targetValidation.valid {
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			if len(targetValidation.urlIdentityObs) > 0 {
				refs = append(refs, buildEvidenceRefs(ruleResultID, targetValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			}
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, targetValidation.summary, expectedSummary, evalTime, refs))
			continue
		}

		// 6. Traverse to target URL subject and inspect target's own HTTP status
		targetStatusObs := idx.GetObservations(audit.SubjectURL, targetRefVal, "http_status")
		if len(targetStatusObs) == 0 {
			summary := "Target URL was discovered but not fetched; HTTP status observation is unavailable."
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		statusVal, statusUsable, statusConflict := extractSingleObsValue(targetStatusObs)
		if !statusUsable {
			var summary string
			if statusConflict {
				summary = "Conflicting HTTP status observations on target URL."
			} else {
				summary = "Malformed HTTP status observation on target URL."
			}
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetStatusObs, "http_status", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		statusCode, err := strconv.Atoi(statusVal)
		if err != nil || statusCode <= 0 {
			summary := fmt.Sprintf("Target HTTP status observation %q is malformed.", statusVal)
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetStatusObs, "http_status", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		if statusCode < 100 || statusCode > 599 {
			summary := fmt.Sprintf("Target HTTP status code %d is outside valid HTTP range 100-599.", statusCode)
			refs := buildLinkContextRefs()
			refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, sourceValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetStatusObs, "http_status", audit.EvidenceRoleContext)...)
			results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, summary, expectedSummary, evalTime, refs))
			continue
		}

		// 7. Status Decision Matrix: evaluate against targetClass
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		switch targetClass {
		case 3:
			if statusCode >= 300 && statusCode <= 399 {
				evalStatus = audit.StatusFail
				observedSummary = fmt.Sprintf("Internal link targets HTTP %d redirect response.", statusCode)
			} else {
				evalStatus = audit.StatusPass
				observedSummary = fmt.Sprintf("Internal link target returned HTTP status %d (not 3xx).", statusCode)
			}
		case 4:
			if statusCode >= 400 && statusCode <= 499 {
				evalStatus = audit.StatusFail
				observedSummary = fmt.Sprintf("Internal link targets HTTP %d client error response.", statusCode)
			} else {
				evalStatus = audit.StatusPass
				observedSummary = fmt.Sprintf("Internal link target returned HTTP status %d (not 4xx).", statusCode)
			}
		case 5:
			if statusCode >= 500 && statusCode <= 599 {
				evalStatus = audit.StatusFail
				observedSummary = fmt.Sprintf("Internal link targets HTTP %d server error response.", statusCode)
			} else {
				evalStatus = audit.StatusPass
				observedSummary = fmt.Sprintf("Internal link target returned HTTP status %d (not 5xx).", statusCode)
			}
		}

		// Build final evidence refs
		refs := buildLinkContextRefs()
		refs = append(refs, buildEvidenceRefs(ruleResultID, isInternalObs, "link_is_internal", audit.EvidenceRoleContext)...)
		refs = append(refs, buildEvidenceRefs(ruleResultID, sourceValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
		refs = append(refs, buildEvidenceRefs(ruleResultID, targetValidation.urlIdentityObs, "url", audit.EvidenceRoleContext)...)
		refs = append(refs, buildEvidenceRefs(ruleResultID, targetStatusObs, "http_status", audit.EvidenceRolePrimary)...)

		targetFinalURLObs := idx.GetObservations(audit.SubjectURL, targetRefVal, "redirect_final_url")
		if len(targetFinalURLObs) > 0 {
			refs = append(refs, buildEvidenceRefs(ruleResultID, targetFinalURLObs, "redirect_final_url", audit.EvidenceRoleContext)...)
		}

		results = append(results, makeLinkResult(rule, snapshot, subjectRef, ruleResultID, evalStatus, observedSummary, expectedSummary, evalTime, refs))
	}

	return results, nil
}

// subjectIdentityValidation holds the result of validating a referenced URL subject's identity.
type subjectIdentityValidation struct {
	valid          bool
	summary        string
	urlIdentityObs []audit.NormalizedObservation
}

// validateReferencedURLSubject verifies that a subject reference:
// 1. Is well-formed and belongs to the current audit run (url:<audit_run_id>:<id>).
// 2. Exists as an actual SubjectURL in the frozen snapshot.
// 3. Has usable url_identity evidence (present, non-empty, non-conflicting).
// 4. Matches the expected normalized URL string exactly.
func validateReferencedURLSubject(
	idx *EvidenceIndex,
	snapshot *audit.EvidenceSnapshot,
	subjectRole string, // "Source" or "Target"
	refVal string,
	expectedURL string,
	expectedURLPrefix string,
) subjectIdentityValidation {
	if !strings.HasPrefix(refVal, "url:") {
		return subjectIdentityValidation{
			valid:   false,
			summary: fmt.Sprintf("%s subject reference %q is not a valid URL subject.", subjectRole, refVal),
		}
	}

	if !strings.HasPrefix(refVal, expectedURLPrefix) {
		return subjectIdentityValidation{
			valid:   false,
			summary: fmt.Sprintf("%s subject reference %q does not match current audit run %q.", subjectRole, refVal, snapshot.AuditRunID),
		}
	}

	if refVal == expectedURLPrefix {
		return subjectIdentityValidation{
			valid:   false,
			summary: fmt.Sprintf("%s subject reference %q is malformed; missing subject identifier.", subjectRole, refVal),
		}
	}

	// 2. Verify referenced SubjectURL exists in snapshot
	if idx.bySubject == nil || idx.bySubject[audit.SubjectURL] == nil || idx.bySubject[audit.SubjectURL][refVal] == nil {
		return subjectIdentityValidation{
			valid:   false,
			summary: fmt.Sprintf("Referenced %s URL subject %q does not exist in snapshot.", strings.ToLower(subjectRole), refVal),
		}
	}

	// 3. Inspect url_identity observations on the referenced SubjectURL
	urlIdentObs := idx.GetObservations(audit.SubjectURL, refVal, "url_identity")
	if len(urlIdentObs) == 0 {
		return subjectIdentityValidation{
			valid:   false,
			summary: fmt.Sprintf("Referenced %s URL subject %q has no url_identity observation.", strings.ToLower(subjectRole), refVal),
		}
	}

	identVal, identUsable, identConflict := extractSingleObsValue(urlIdentObs)
	if identConflict {
		return subjectIdentityValidation{
			valid:          false,
			summary:        fmt.Sprintf("Referenced %s URL subject %q has conflicting url_identity observations.", strings.ToLower(subjectRole), refVal),
			urlIdentityObs: urlIdentObs,
		}
	}
	if !identUsable {
		return subjectIdentityValidation{
			valid:          false,
			summary:        fmt.Sprintf("Referenced %s URL subject %q has malformed url_identity observation.", strings.ToLower(subjectRole), refVal),
			urlIdentityObs: urlIdentObs,
		}
	}

	// 4. Verify url_identity matches expected URL exactly
	if identVal != expectedURL {
		return subjectIdentityValidation{
			valid:          false,
			summary:        fmt.Sprintf("Referenced %s URL subject %q identity %q does not match link %s %q.", strings.ToLower(subjectRole), refVal, identVal, strings.ToLower(subjectRole), expectedURL),
			urlIdentityObs: urlIdentObs,
		}
	}

	return subjectIdentityValidation{
		valid:          true,
		urlIdentityObs: urlIdentObs,
	}
}

// checkNonHTTPScheme determines whether a target URL is verified to have a non-HTTP scheme.
func checkNonHTTPScheme(rawURL string) (bool, string) {
	trimmed := strings.TrimSpace(rawURL)
	lower := strings.ToLower(trimmed)

	// Known common non-HTTP prefixes
	for _, prefix := range []string{"mailto:", "tel:", "javascript:", "data:", "file:", "sms:", "ftp:"} {
		if strings.HasPrefix(lower, prefix) {
			return true, strings.TrimSuffix(prefix, ":")
		}
	}

	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Scheme != "" {
		schemeLower := strings.ToLower(parsed.Scheme)
		if schemeLower != "http" && schemeLower != "https" {
			return true, schemeLower
		}
	}

	return false, ""
}

// extractSingleObsValue extracts and validates a unique string value from a slice of NormalizedObservations.
func extractSingleObsValue(obs []audit.NormalizedObservation) (string, bool, bool) {
	if len(obs) == 0 {
		return "", false, false
	}
	firstVal := strings.TrimSpace(obs[0].Value)
	if firstVal == "" {
		return "", false, false
	}
	for _, o := range obs[1:] {
		if strings.TrimSpace(o.Value) != firstVal {
			return "", false, true // conflict
		}
	}
	return firstVal, true, false
}

// makeLinkResult constructs an audit.RuleResult for a link subject with deterministic evidence refs.
func makeLinkResult(
	rule audit.RuleDefinition,
	snapshot *audit.EvidenceSnapshot,
	subjectRef string,
	ruleResultID audit.RuleResultID,
	status audit.RuleResultStatus,
	observedSummary string,
	expectedSummary string,
	evalTime time.Time,
	evidenceRefs []audit.RuleEvidenceRef,
) audit.RuleResult {
	return audit.RuleResult{
		RuleResultID:    ruleResultID,
		AuditRunID:      snapshot.AuditRunID,
		SnapshotID:      snapshot.SnapshotID,
		RuleID:          rule.RuleID,
		ParentCheck:     rule.ParentCheck,
		RuleVersion:     rule.RuleVersion,
		SubjectType:     audit.SubjectLink,
		SubjectRef:      subjectRef,
		Status:          status,
		Severity:        rule.DefaultSeverity,
		Scope:           audit.ScopeURL,
		ObservedSummary: observedSummary,
		ExpectedSummary: expectedSummary,
		EvaluatedAt:     evalTime,
		EvidenceRefs:    dedupeAndSortEvidenceRefs(evidenceRefs),
	}
}
