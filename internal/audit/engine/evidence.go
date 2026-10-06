package engine

import (
	"fmt"
	"sort"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

// EvidenceIndex provides a read-only, indexed view over a frozen EvidenceSnapshot's
// normalized observations, enabling deterministic rule evaluation lookups by subject and field.
type EvidenceIndex struct {
	// bySubject indexes observations by: [SubjectType][SubjectRef][Field][]NormalizedObservation
	bySubject map[audit.EvaluationSubjectType]map[string]map[string][]audit.NormalizedObservation

	// subjectRefs stores deterministically sorted unique SubjectRefs per SubjectType
	subjectRefs map[audit.EvaluationSubjectType][]string
}

// NewEvidenceIndex validates the frozen snapshot and builds an immutable, deterministic lookup index.
func NewEvidenceIndex(snapshot *audit.EvidenceSnapshot) (*EvidenceIndex, error) {
	if snapshot == nil {
		return nil, ErrNilSnapshot
	}
	if snapshot.SnapshotStatus != audit.SnapshotFrozen {
		return nil, fmt.Errorf("%w: snapshot %q status is %q (expected %q)",
			ErrSnapshotNotFrozen, snapshot.SnapshotID, snapshot.SnapshotStatus, audit.SnapshotFrozen)
	}
	if snapshot.SnapshotID == "" {
		return nil, fmt.Errorf("%w: snapshot_id cannot be empty", ErrInvalidSnapshot)
	}
	if snapshot.AuditRunID == "" {
		return nil, fmt.Errorf("%w: audit_run_id cannot be empty", ErrInvalidSnapshot)
	}
	if snapshot.FrozenAt == nil {
		return nil, fmt.Errorf("%w: frozen_at timestamp cannot be nil for frozen snapshot", ErrInvalidSnapshot)
	}
	if snapshot.NormalizationVersion == "" {
		return nil, fmt.Errorf("%w: normalization_version cannot be empty", ErrInvalidSnapshot)
	}

	bySubject := make(map[audit.EvaluationSubjectType]map[string]map[string][]audit.NormalizedObservation)
	subjectSet := make(map[audit.EvaluationSubjectType]map[string]struct{})

	for i, obs := range snapshot.NormalizedObservations {
		// Validate observation integrity
		if obs.ObservationID == "" {
			return nil, fmt.Errorf("%w: observation at index %d has empty observation_id", ErrInvalidObservation, i)
		}
		if obs.AuditRunID != snapshot.AuditRunID {
			return nil, fmt.Errorf("%w: observation %q audit_run_id %q does not match snapshot audit_run_id %q",
				ErrInvalidObservation, obs.ObservationID, obs.AuditRunID, snapshot.AuditRunID)
		}
		if obs.SnapshotID != snapshot.SnapshotID {
			return nil, fmt.Errorf("%w: observation %q snapshot_id %q does not match snapshot snapshot_id %q",
				ErrInvalidObservation, obs.ObservationID, obs.SnapshotID, snapshot.SnapshotID)
		}
		if err := audit.ValidateEvaluationSubjectType(obs.SubjectType); err != nil {
			return nil, fmt.Errorf("%w: observation %q subject_type invalid: %w", ErrInvalidObservation, obs.ObservationID, err)
		}
		if err := audit.ValidateDerivationType(obs.DerivationType); err != nil {
			return nil, fmt.Errorf("%w: observation %q derivation_type invalid: %w", ErrInvalidObservation, obs.ObservationID, err)
		}
		if obs.SubjectRef == "" {
			return nil, fmt.Errorf("%w: observation %q subject_ref cannot be empty", ErrInvalidObservation, obs.ObservationID)
		}
		if obs.Field == "" {
			return nil, fmt.Errorf("%w: observation %q field cannot be empty", ErrInvalidObservation, obs.ObservationID)
		}

		// Group observation into index
		if bySubject[obs.SubjectType] == nil {
			bySubject[obs.SubjectType] = make(map[string]map[string][]audit.NormalizedObservation)
			subjectSet[obs.SubjectType] = make(map[string]struct{})
		}
		if bySubject[obs.SubjectType][obs.SubjectRef] == nil {
			bySubject[obs.SubjectType][obs.SubjectRef] = make(map[string][]audit.NormalizedObservation)
		}
		bySubject[obs.SubjectType][obs.SubjectRef][obs.Field] = append(
			bySubject[obs.SubjectType][obs.SubjectRef][obs.Field], obs)

		subjectSet[obs.SubjectType][obs.SubjectRef] = struct{}{}
	}

	// Produce deterministically sorted subject ref slices
	sortedSubjects := make(map[audit.EvaluationSubjectType][]string, len(subjectSet))
	for st, set := range subjectSet {
		refs := make([]string, 0, len(set))
		for ref := range set {
			refs = append(refs, ref)
		}
		sort.Strings(refs)
		sortedSubjects[st] = refs
	}

	return &EvidenceIndex{
		bySubject:   bySubject,
		subjectRefs: sortedSubjects,
	}, nil
}

// SubjectRefs returns a copy of all sorted unique SubjectRef identifiers for a given SubjectType.
func (idx *EvidenceIndex) SubjectRefs(st audit.EvaluationSubjectType) []string {
	refs, ok := idx.subjectRefs[st]
	if !ok {
		return nil
	}
	out := make([]string, len(refs))
	copy(out, refs)
	return out
}

// GetObservations returns all normalized observations matching the given subject type, subject ref, and field.
func (idx *EvidenceIndex) GetObservations(st audit.EvaluationSubjectType, subjectRef, field string) []audit.NormalizedObservation {
	refsMap, ok := idx.bySubject[st]
	if !ok {
		return nil
	}
	fieldsMap, ok := refsMap[subjectRef]
	if !ok {
		return nil
	}
	obsList, ok := fieldsMap[field]
	if !ok {
		return nil
	}
	out := make([]audit.NormalizedObservation, len(obsList))
	copy(out, obsList)
	return out
}

// GetFirstObservation returns the first observation matching the subject and field, if present.
func (idx *EvidenceIndex) GetFirstObservation(st audit.EvaluationSubjectType, subjectRef, field string) (*audit.NormalizedObservation, bool) {
	obsList := idx.GetObservations(st, subjectRef, field)
	if len(obsList) == 0 {
		return nil, false
	}
	return &obsList[0], true
}
