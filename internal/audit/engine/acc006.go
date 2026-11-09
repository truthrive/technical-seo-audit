package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDACC006 = "AR-ACC-006"

// evaluateACC006 implements the typed execution logic for:
// AR-ACC-006 — Googlebot profile response anomaly (parent check: ACC-002, P1, assisted).
//
// Evaluates observed audit.SubjectURL entities against comparative fetch evidence:
// - url / url_identity
// - default_profile_status
// - googlebot_profile_status
// - default_challenge_detected
// - googlebot_challenge_detected
//
// Contract (per frozen manifest rules_v1.json):
// - NOT_APPLICABLE:
//   - Profile comparison not enabled for the URL (no googlebot profile observations and
//     profile_comparison_enabled != "true").
// - UNKNOWN:
//   - Missing/conflicting/malformed URL identity, or profile comparison started/enabled
//     with incomplete, conflicting, or malformed comparison evidence.
// - WARNING:
//   - Googlebot-like request receives 403/429 (or 5xx anomaly) or challenge response
//     while normal request succeeds without challenge.
// - PASS:
//   - Both profiles receive equivalent accessible response classes without bot-only challenge.
func evaluateACC006(
	ctx context.Context,
	rule audit.RuleDefinition,
	idx *EvidenceIndex,
	policies *PolicyIndex,
	snapshot *audit.EvidenceSnapshot,
	evalTime time.Time,
) ([]audit.RuleResult, error) {
	return evaluateBotProfileComparison(
		ctx,
		rule,
		idx,
		snapshot,
		evalTime,
		"Googlebot",
		"googlebot_profile_status",
		"googlebot_challenge_detected",
		"",
		"Googlebot profile receives an equivalent accessible response class without bot-only challenge.",
	)
}

// evaluateBotProfileComparison provides the shared deterministic evaluation logic for
// comparative bot-profile rules (AR-ACC-006, AR-ACC-008, AR-AI-002).
func evaluateBotProfileComparison(
	ctx context.Context,
	rule audit.RuleDefinition,
	idx *EvidenceIndex,
	snapshot *audit.EvidenceSnapshot,
	evalTime time.Time,
	profileLabel string,
	botStatusField string,
	botChallengeField string,
	testedUAField string,
	expectedSummary string,
) ([]audit.RuleResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	subjectRefs := idx.SubjectRefs(audit.SubjectURL)
	results := make([]audit.RuleResult, 0, len(subjectRefs))

	for _, subjectRef := range subjectRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		urlIdentityObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		if len(urlIdentityObs) == 0 {
			urlIdentityObs = idx.GetObservations(audit.SubjectURL, subjectRef, "url")
		}

		enabledObs := idx.GetObservations(audit.SubjectURL, subjectRef, "profile_comparison_enabled")
		if len(enabledObs) == 0 {
			enabledObs = idx.GetObservations(audit.SubjectSite, "site", "profile_comparison_enabled")
		}

		defStatusObs := idx.GetObservations(audit.SubjectURL, subjectRef, "default_profile_status")
		botStatusObs := idx.GetObservations(audit.SubjectURL, subjectRef, botStatusField)
		defChalObs := idx.GetObservations(audit.SubjectURL, subjectRef, "default_challenge_detected")
		botChalObs := idx.GetObservations(audit.SubjectURL, subjectRef, botChallengeField)
		var testedUAObs []audit.NormalizedObservation
		if testedUAField != "" {
			testedUAObs = idx.GetObservations(audit.SubjectURL, subjectRef, testedUAField)
		}

		buildRefs := func(primaryField string) []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			roleFor := func(f string) audit.EvidenceRole {
				if f == primaryField {
					return audit.EvidenceRolePrimary
				}
				return audit.EvidenceRoleSupporting
			}
			refs = append(refs, buildEvidenceRefs(ruleResultID, urlIdentityObs, "url", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, enabledObs, "profile_comparison_enabled", audit.EvidenceRoleContext)...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, defStatusObs, "default_profile_status", roleFor("default_profile_status"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, botStatusObs, botStatusField, roleFor(botStatusField))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, defChalObs, "default_challenge_detected", roleFor("default_challenge_detected"))...)
			refs = append(refs, buildEvidenceRefs(ruleResultID, botChalObs, botChallengeField, roleFor(botChallengeField))...)
			if testedUAField != "" {
				refs = append(refs, buildEvidenceRefs(ruleResultID, testedUAObs, testedUAField, roleFor(testedUAField))...)
			}
			return refs
		}

		// 1. Validate URL identity
		if len(urlIdentityObs) == 0 {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Required URL identity observation is missing.", expectedSummary, evalTime, buildRefs("")))
			continue
		}
		_, urlUsable, urlConflict := extractSingleObsValue(urlIdentityObs)
		if urlConflict || !urlUsable {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "URL identity observation is conflicting or malformed.", expectedSummary, evalTime, buildRefs("")))
			continue
		}

		// 2. Check if profile comparison is enabled / attempted
		hasBotEvidence := len(botStatusObs) > 0 || len(botChalObs) > 0 || len(testedUAObs) > 0 || len(defStatusObs) > 0 || len(defChalObs) > 0
		if len(enabledObs) > 0 {
			enVal, enUsable, enConflict := extractSingleObsValue(enabledObs)
			if enConflict || !enUsable {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Conflicting or malformed profile_comparison_enabled observation.", expectedSummary, evalTime, buildRefs("profile_comparison_enabled")))
				continue
			}
			enLower := strings.ToLower(enVal)
			if enLower != "true" && enLower != "false" {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("Malformed profile_comparison_enabled value %q.", enVal), expectedSummary, evalTime, buildRefs("profile_comparison_enabled")))
				continue
			}
			if enLower == "false" {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusNotApplicable, fmt.Sprintf("%s profile comparison is not enabled for audited URL.", profileLabel), expectedSummary, evalTime, buildRefs("profile_comparison_enabled")))
				continue
			}
		} else if !hasBotEvidence {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusNotApplicable, fmt.Sprintf("%s profile comparison is not enabled for audited URL.", profileLabel), expectedSummary, evalTime, buildRefs("")))
			continue
		}

		// 3. Validate tested_user_agent when required
		var testedUAVal string
		if testedUAField != "" {
			uaVal, uaUsable, uaConflict := extractSingleObsValue(testedUAObs)
			if !uaUsable || uaConflict || uaVal == "" {
				results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("Required %s evidence is missing, conflicting, or malformed.", testedUAField), expectedSummary, evalTime, buildRefs(testedUAField)))
				continue
			}
			testedUAVal = uaVal
		}

		// 4. Validate default_profile_status and botStatusField
		defStatusStr, defStatusUsable, defStatusConflict := extractSingleObsValue(defStatusObs)
		if !defStatusUsable || defStatusConflict {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Required default_profile_status evidence is missing, conflicting, or malformed.", expectedSummary, evalTime, buildRefs("default_profile_status")))
			continue
		}
		defStatus, err := strconv.Atoi(defStatusStr)
		if err != nil || defStatus < 100 || defStatus > 599 {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("default_profile_status %q is outside valid HTTP status range 100-599.", defStatusStr), expectedSummary, evalTime, buildRefs("default_profile_status")))
			continue
		}

		botStatusStr, botStatusUsable, botStatusConflict := extractSingleObsValue(botStatusObs)
		if !botStatusUsable || botStatusConflict {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("Required %s evidence is missing, conflicting, or malformed.", botStatusField), expectedSummary, evalTime, buildRefs(botStatusField)))
			continue
		}
		botStatus, err := strconv.Atoi(botStatusStr)
		if err != nil || botStatus < 100 || botStatus > 599 {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("%s %q is outside valid HTTP status range 100-599.", botStatusField, botStatusStr), expectedSummary, evalTime, buildRefs(botStatusField)))
			continue
		}

		// 5. Validate default_challenge_detected and botChallengeField
		defChalStr, defChalUsable, defChalConflict := extractSingleObsValue(defChalObs)
		if !defChalUsable || defChalConflict {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, "Required default_challenge_detected evidence is missing, conflicting, or malformed.", expectedSummary, evalTime, buildRefs("default_challenge_detected")))
			continue
		}
		defChalLower := strings.ToLower(defChalStr)
		if defChalLower != "true" && defChalLower != "false" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("default_challenge_detected value %q is malformed; expected 'true' or 'false'.", defChalStr), expectedSummary, evalTime, buildRefs("default_challenge_detected")))
			continue
		}
		defChallenge := defChalLower == "true"

		botChalStr, botChalUsable, botChalConflict := extractSingleObsValue(botChalObs)
		if !botChalUsable || botChalConflict {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("Required %s evidence is missing, conflicting, or malformed.", botChallengeField), expectedSummary, evalTime, buildRefs(botChallengeField)))
			continue
		}
		botChalLower := strings.ToLower(botChalStr)
		if botChalLower != "true" && botChalLower != "false" {
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusUnknown, fmt.Sprintf("%s value %q is malformed; expected 'true' or 'false'.", botChallengeField, botChalStr), expectedSummary, evalTime, buildRefs(botChallengeField)))
			continue
		}
		botChallenge := botChalLower == "true"

		// 6. Evaluate response anomaly
		defaultAccessible := defStatus >= 200 && defStatus < 400 && !defChallenge
		botRestrictedStatus := botStatus == 403 || botStatus == 429 || (botStatus >= 500 && defStatus < 400)
		botOnlyChallenge := botChallenge && !defChallenge

		label := profileLabel
		if testedUAVal != "" {
			label = fmt.Sprintf("%s (%s)", profileLabel, testedUAVal)
		}

		if defaultAccessible && (botRestrictedStatus || botOnlyChallenge) {
			primaryField := botStatusField
			if botOnlyChallenge && !botRestrictedStatus {
				primaryField = botChallengeField
			}
			summary := fmt.Sprintf("%s request received HTTP %d (challenge=%t) while default profile succeeded with HTTP %d (challenge=%t).", label, botStatus, botChallenge, defStatus, defChallenge)
			results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusWarning, summary, expectedSummary, evalTime, buildRefs(primaryField)))
			continue
		}

		summary := fmt.Sprintf("%s request received HTTP %d (challenge=%t), consistent with default profile HTTP %d (challenge=%t).", label, botStatus, botChallenge, defStatus, defChallenge)
		results = append(results, makeURLResult(rule, snapshot, subjectRef, ruleResultID, audit.StatusPass, summary, expectedSummary, evalTime, buildRefs(botStatusField)))
	}

	return results, nil
}
