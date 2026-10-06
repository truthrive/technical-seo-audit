package engine

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

// EvaluationContext bundles the frozen EvidenceSnapshot with an immutable PolicyIndex,
// forming the explicit, normalized boundary for rule evaluation.
type EvaluationContext struct {
	Snapshot *audit.EvidenceSnapshot
	Policies *PolicyIndex
}

// Evaluator is the typed signature for an atomic rule execution handler.
type Evaluator func(
	ctx context.Context,
	rule audit.RuleDefinition,
	evidence *EvidenceIndex,
	policies *PolicyIndex,
	snapshot *audit.EvidenceSnapshot,
	evalTime time.Time,
) ([]audit.RuleResult, error)

// Engine coordinates the execution of atomic audit rules against frozen EvidenceSnapshots.
// It relies on the machine-readable rule registry for metadata and enforces strict
// architectural boundaries: no HTTP calls, no SQLite queries, no Finding aggregation,
// and no mutation of snapshots or policy inference.
type Engine struct {
	registry   *audit.Registry
	evaluators map[string]Evaluator
	clock      func() time.Time
}

// New creates and initializes an Engine loaded with the frozen 47-rule V1 registry
// and all currently implemented typed rule evaluators.
func New() (*Engine, error) {
	reg, err := audit.LoadV1Registry()
	if err != nil {
		return nil, fmt.Errorf("audit engine: failed to load registry: %w", err)
	}
	return NewWithRegistry(reg)
}

// NewV1 is an alias for New, constructing the standard V1 engine.
func NewV1() (*Engine, error) {
	return New()
}

// NewWithRegistry initializes an Engine using a pre-loaded audit rule registry.
func NewWithRegistry(reg *audit.Registry) (*Engine, error) {
	if reg == nil {
		return nil, errors.New("audit engine: registry cannot be nil")
	}

	e := &Engine{
		registry:   reg,
		evaluators: make(map[string]Evaluator),
		clock:      func() time.Time { return time.Now().UTC() },
	}

	// Register implemented V1.3a evaluators
	e.register(ruleIDACC004, evaluateACC004)

	return e, nil
}

// SetClock allows injecting a custom time provider, primarily for deterministic testing.
func (e *Engine) SetClock(clock func() time.Time) {
	if clock != nil {
		e.clock = clock
	}
}

// register binds an executable typed evaluator to a specific rule ID.
func (e *Engine) register(ruleID string, fn Evaluator) {
	e.evaluators[ruleID] = fn
}

// ImplementedRuleIDs returns the deterministically sorted list of rule IDs currently
// supported with an executable evaluator. For Audit V1.3a / V1.3b1, this is exactly ["AR-ACC-004"].
func (e *Engine) ImplementedRuleIDs() []string {
	ids := make([]string, 0, len(e.evaluators))
	for id := range e.evaluators {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// EvaluateRule evaluates a single atomic rule against a frozen EvidenceSnapshot
// with an empty policy set. This is a backward-compatible wrapper around EvaluateRuleWithContext.
// Returns ErrRuleNotFound if the rule is not in the registry,
// or ErrRuleNotImplemented if the rule is registered but lacks a typed evaluator in this slice.
func (e *Engine) EvaluateRule(
	ctx context.Context,
	snapshot *audit.EvidenceSnapshot,
	ruleID string,
) ([]audit.RuleResult, error) {
	return e.EvaluateRuleWithContext(ctx, EvaluationContext{
		Snapshot: snapshot,
		Policies: NewEmptyPolicyIndex(),
	}, ruleID)
}

// EvaluateRuleWithContext evaluates a single atomic rule against a frozen EvidenceSnapshot
// and explicit ProjectPolicyAssignments bundled within an EvaluationContext.
//
// Validation and execution order:
//  1. Validate frozen snapshot and build read-only indexed evidence view (Steps 1 & 2)
//  2. Normalize and validate policies (Step 3)
//  3. Validate AuditRunID consistency between policies and snapshot (Step 4)
//  4. Resolve rule metadata from registry (Step 5)
//  5. Resolve typed evaluator
//  6. Execute the typed evaluator with snapshot, evidence index, and policies (Step 6)
//  7. Structural validation of generated results
func (e *Engine) EvaluateRuleWithContext(
	ctx context.Context,
	eval EvaluationContext,
	ruleID string,
) ([]audit.RuleResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// 1. Validate frozen snapshot and build read-only indexed evidence view
	idx, err := NewEvidenceIndex(eval.Snapshot)
	if err != nil {
		return nil, err
	}

	// 2. Normalize policies (PolicyIndex must never be nil internally)
	policies := eval.Policies
	if policies == nil {
		policies = NewEmptyPolicyIndex()
	}

	// 3. Validate AuditRunID consistency between policies and snapshot
	if err := policies.ValidateForRun(eval.Snapshot.AuditRunID); err != nil {
		return nil, err
	}

	// 4. Resolve rule metadata from registry
	ruleDef, exists := e.registry.Get(ruleID)
	if !exists {
		return nil, fmt.Errorf("%w: %q", ErrRuleNotFound, ruleID)
	}

	// 5. Resolve typed evaluator
	evaluator, implemented := e.evaluators[ruleID]
	if !implemented {
		return nil, fmt.Errorf("%w: %q", ErrRuleNotImplemented, ruleID)
	}

	evalTime := e.clock()

	// 6. Execute the typed evaluator
	results, err := evaluator(ctx, ruleDef, idx, policies, eval.Snapshot, evalTime)
	if err != nil {
		return nil, err
	}

	// 7. Structural validation of generated results
	for i, rr := range results {
		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: result %d failed validation: %w", i, err)
		}
	}

	return results, nil
}

// validateRuleResult performs strict structural validation on a generated RuleResult.
func validateRuleResult(rr audit.RuleResult) error {
	if rr.RuleResultID == "" {
		return errors.New("empty rule_result_id")
	}
	if rr.AuditRunID == "" {
		return errors.New("empty audit_run_id")
	}
	if rr.SnapshotID == "" {
		return errors.New("empty snapshot_id")
	}
	if rr.RuleID == "" {
		return errors.New("empty rule_id")
	}
	if rr.ParentCheck == "" {
		return errors.New("empty parent_check")
	}
	if rr.RuleVersion <= 0 {
		return errors.New("invalid rule_version")
	}
	if rr.SubjectRef == "" {
		return errors.New("empty subject_ref")
	}
	if err := audit.ValidateRuleResultStatus(rr.Status); err != nil {
		return err
	}
	if err := audit.ValidateSeverity(rr.Severity); err != nil {
		return err
	}
	if err := audit.ValidateEvaluationSubjectType(rr.SubjectType); err != nil {
		return err
	}
	if err := audit.ValidateAuditReportScope(rr.Scope); err != nil {
		return err
	}
	if rr.ObservedSummary == "" {
		return errors.New("empty observed_summary")
	}
	if rr.ExpectedSummary == "" {
		return errors.New("empty expected_summary")
	}
	if rr.EvaluatedAt.IsZero() {
		return errors.New("zero evaluated_at timestamp")
	}

	for j, ref := range rr.EvidenceRefs {
		if ref.RuleEvidenceRefID == "" {
			return fmt.Errorf("evidence ref at index %d has empty rule_evidence_ref_id", j)
		}
		if ref.RuleResultID != rr.RuleResultID {
			return fmt.Errorf("evidence ref at index %d rule_result_id %q does not match parent %q",
				j, ref.RuleResultID, rr.RuleResultID)
		}
		if ref.EvidenceType == "" {
			return fmt.Errorf("evidence ref at index %d has empty evidence_type", j)
		}
		if ref.EvidenceRef == "" {
			return fmt.Errorf("evidence ref at index %d has empty evidence_ref", j)
		}
		if ref.Field == "" {
			return fmt.Errorf("evidence ref at index %d has empty field", j)
		}
		if err := audit.ValidateEvidenceRole(ref.Role); err != nil {
			return fmt.Errorf("evidence ref at index %d has invalid role: %w", j, err)
		}
	}

	return nil
}
