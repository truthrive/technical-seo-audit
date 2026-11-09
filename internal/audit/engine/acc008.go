package engine

import (
	"context"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDACC008 = "AR-ACC-008"

// evaluateACC008 implements the typed execution logic for:
// AR-ACC-008 — Bot-profile access restriction observed (parent check: ACC-005, P1, assisted).
//
// Evaluates observed audit.SubjectURL entities against comparative bot-profile fetch evidence:
// - url / url_identity
// - default_profile_status
// - bot_profile_status
// - default_challenge_detected
// - bot_challenge_detected
// - tested_user_agent
func evaluateACC008(
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
		"Bot profile",
		"bot_profile_status",
		"bot_challenge_detected",
		"tested_user_agent",
		"Alternate bot profile does not encounter access restriction compared to normal profile.",
	)
}
