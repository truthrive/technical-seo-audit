package engine

import (
	"context"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDAI002 = "AR-AI-002"

// evaluateAI002 implements the typed execution logic for:
// AR-AI-002 — OAI-SearchBot profile encounters access anomaly (parent check: AI-001, P2, assisted).
//
// Evaluates observed audit.SubjectURL entities against comparative OAI-SearchBot profile fetch evidence:
// - url / url_identity
// - default_profile_status
// - oai_profile_status
// - default_challenge_detected
// - oai_challenge_detected
func evaluateAI002(
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
		"OAI-SearchBot profile",
		"oai_profile_status",
		"oai_challenge_detected",
		"",
		"OAI-SearchBot profile does not encounter access anomaly compared to normal profile.",
	)
}
