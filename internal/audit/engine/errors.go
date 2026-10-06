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
)
