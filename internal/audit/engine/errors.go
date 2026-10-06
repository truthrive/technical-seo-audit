package engine

import "errors"

var (
	// ErrNilSnapshot is returned when a nil EvidenceSnapshot is passed for evaluation.
	ErrNilSnapshot = errors.New("audit engine: snapshot cannot be nil")

	// ErrSnapshotNotFrozen is returned when an EvidenceSnapshot is not in FROZEN status.
	ErrSnapshotNotFrozen = errors.New("audit engine: snapshot must be in FROZEN status")

	// ErrInvalidSnapshot is returned when an EvidenceSnapshot fails mandatory structural validation.
	ErrInvalidSnapshot = errors.New("audit engine: invalid snapshot")

	// ErrInvalidObservation is returned when a consumed observation violates snapshot integrity contracts.
	ErrInvalidObservation = errors.New("audit engine: invalid observation")

	// ErrRuleNotFound is returned when the requested rule ID does not exist in the V1 rule registry.
	ErrRuleNotFound = errors.New("audit engine: rule not found in registry")

	// ErrRuleNotImplemented is returned when a rule is defined in the registry but has no executable evaluator.
	ErrRuleNotImplemented = errors.New("audit engine: rule is defined in registry but not implemented")

	// ErrConflictingPolicy is returned when contradictory policy assignments exist for the same scope, target, and key.
	ErrConflictingPolicy = errors.New("audit engine: conflicting policy assignments")

	// ErrInvalidPolicy is returned when a ProjectPolicyAssignment fails schema or controlled-vocabulary validation.
	ErrInvalidPolicy = errors.New("audit engine: invalid project policy assignment")

	// ErrPolicyRunMismatch is returned when a policy assignment's AuditRunID does not match the evaluation run ID.
	ErrPolicyRunMismatch = errors.New("audit engine: policy assignment audit run id does not match evaluation run id")
)
