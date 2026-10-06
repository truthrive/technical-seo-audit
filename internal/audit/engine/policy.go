package engine

import (
	"fmt"
	"sort"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

// PolicyIndex provides an immutable, deterministic, read-only lookup view over
// explicit ProjectPolicyAssignments for a given audit run.
//
// In accordance with the foundational architectural principle:
//   PROJECT POLICY != OBSERVED EVIDENCE
// PolicyIndex is strictly decoupled from EvidenceSnapshot and NormalizedObservations.
type PolicyIndex struct {
	auditRunID audit.AuditRunID

	// lookup maps: [PolicyScope][TargetRef][PolicyKey]ProjectPolicyAssignment
	lookup map[audit.PolicyScope]map[string]map[audit.PolicyKey]audit.ProjectPolicyAssignment

	// assignments stores the canonical assignments, deterministically sorted by PolicyAssignmentID
	assignments []audit.ProjectPolicyAssignment
}

// NewEmptyPolicyIndex constructs an empty, immutable PolicyIndex.
func NewEmptyPolicyIndex() *PolicyIndex {
	return &PolicyIndex{
		lookup: make(map[audit.PolicyScope]map[string]map[audit.PolicyKey]audit.ProjectPolicyAssignment),
	}
}

// NewPolicyIndex creates an immutable PolicyIndex from a slice of ProjectPolicyAssignments.
//
// Validation rules:
//  1. Every assignment is strictly validated using ProjectPolicyAssignment.Validate().
//  2. If assignments are non-empty, all assignments must share the same non-empty AuditRunID.
//  3. If multiple assignments target the same (scope, targetRef, key):
//     - Identical values are accepted as semantically equivalent with deterministic canonical selection.
//     - Conflicting values are rejected with ErrConflictingPolicy.
func NewPolicyIndex(assignments []audit.ProjectPolicyAssignment) (*PolicyIndex, error) {
	return NewPolicyIndexForRun("", assignments)
}

// NewPolicyIndexForRun constructs a PolicyIndex and verifies that all assignments match
// the explicitly provided expected AuditRunID.
func NewPolicyIndexForRun(expectedRunID audit.AuditRunID, assignments []audit.ProjectPolicyAssignment) (*PolicyIndex, error) {
	if len(assignments) == 0 {
		return &PolicyIndex{
			auditRunID: expectedRunID,
			lookup:     make(map[audit.PolicyScope]map[string]map[audit.PolicyKey]audit.ProjectPolicyAssignment),
		}, nil
	}

	runID := expectedRunID

	// Temporary map to track (scope, targetRef, key) -> canonical assignment
	type policyKey struct {
		scope     audit.PolicyScope
		targetRef string
		key       audit.PolicyKey
	}

	canonicalMap := make(map[policyKey]audit.ProjectPolicyAssignment, len(assignments))

	for i, p := range assignments {
		// 1. Strict schema & controlled-vocabulary validation
		if err := p.Validate(); err != nil {
			return nil, fmt.Errorf("%w: assignment at index %d (%q): %w",
				ErrInvalidPolicy, i, p.PolicyAssignmentID, err)
		}

		// 2. AuditRunID consistency check
		if runID == "" {
			runID = p.AuditRunID
		} else if p.AuditRunID != runID {
			return nil, fmt.Errorf("%w: assignment %q has audit_run_id %q, expected %q",
				ErrPolicyRunMismatch, p.PolicyAssignmentID, p.AuditRunID, runID)
		}

		// 3. Duplicate and conflict check
		pk := policyKey{
			scope:     p.Scope,
			targetRef: p.TargetRef,
			key:       p.PolicyKey,
		}

		existing, exists := canonicalMap[pk]
		if exists {
			if existing.PolicyValue != p.PolicyValue {
				return nil, fmt.Errorf("%w: conflicting values for %s %q %s: %q vs %q",
					ErrConflictingPolicy, p.Scope, p.TargetRef, p.PolicyKey, existing.PolicyValue, p.PolicyValue)
			}
			// Identical values: accept as semantically equivalent.
			// Deterministically select the canonical assignment by smallest PolicyAssignmentID
			// to guarantee input order independence.
			if p.PolicyAssignmentID < existing.PolicyAssignmentID {
				canonicalMap[pk] = p
			}
		} else {
			canonicalMap[pk] = p
		}
	}

	// 4. Build immutable lookup structure and deterministically sorted canonical list
	lookup := make(map[audit.PolicyScope]map[string]map[audit.PolicyKey]audit.ProjectPolicyAssignment)
	canonicalSlice := make([]audit.ProjectPolicyAssignment, 0, len(canonicalMap))

	for pk, p := range canonicalMap {
		if lookup[pk.scope] == nil {
			lookup[pk.scope] = make(map[string]map[audit.PolicyKey]audit.ProjectPolicyAssignment)
		}
		if lookup[pk.scope][pk.targetRef] == nil {
			lookup[pk.scope][pk.targetRef] = make(map[audit.PolicyKey]audit.ProjectPolicyAssignment)
		}
		lookup[pk.scope][pk.targetRef][pk.key] = p
		canonicalSlice = append(canonicalSlice, p)
	}

	// Sort canonical assignments by PolicyAssignmentID for deterministic iteration
	sort.Slice(canonicalSlice, func(i, j int) bool {
		return canonicalSlice[i].PolicyAssignmentID < canonicalSlice[j].PolicyAssignmentID
	})

	return &PolicyIndex{
		auditRunID:  runID,
		lookup:      lookup,
		assignments: canonicalSlice,
	}, nil
}

// Get returns the explicit ProjectPolicyAssignment for the specified scope, targetRef, and policy key.
// Returns a pointer to a copy of the canonical assignment, and true if found; (nil, false) otherwise.
// The complete assignment is returned so downstream consumers can trace provenance and assignment ID.
func (idx *PolicyIndex) Get(scope audit.PolicyScope, targetRef string, key audit.PolicyKey) (*audit.ProjectPolicyAssignment, bool) {
	if idx == nil || idx.lookup == nil {
		return nil, false
	}
	byScope, ok := idx.lookup[scope]
	if !ok {
		return nil, false
	}
	byTarget, ok := byScope[targetRef]
	if !ok {
		return nil, false
	}
	assignment, ok := byTarget[key]
	if !ok {
		return nil, false
	}
	cp := assignment
	return &cp, true
}

// Len returns the count of unique, canonical policy assignments in the index.
func (idx *PolicyIndex) Len() int {
	if idx == nil {
		return 0
	}
	return len(idx.assignments)
}

// AuditRunID returns the AuditRunID associated with the policies in this index.
func (idx *PolicyIndex) AuditRunID() audit.AuditRunID {
	if idx == nil {
		return ""
	}
	return idx.auditRunID
}

// Assignments returns a copy of all canonical policy assignments, deterministically
// sorted by PolicyAssignmentID.
func (idx *PolicyIndex) Assignments() []audit.ProjectPolicyAssignment {
	if idx == nil || len(idx.assignments) == 0 {
		return nil
	}
	out := make([]audit.ProjectPolicyAssignment, len(idx.assignments))
	copy(out, idx.assignments)
	return out
}

// TargetRefs returns a deterministically sorted slice of all unique TargetRef identifiers
// present in the index for a given scope.
func (idx *PolicyIndex) TargetRefs(scope audit.PolicyScope) []string {
	if idx == nil || idx.lookup == nil {
		return nil
	}
	byScope, ok := idx.lookup[scope]
	if !ok {
		return nil
	}
	refs := make([]string, 0, len(byScope))
	for ref := range byScope {
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}

// ValidateForRun validates that this index's assignments conform to schemas and
// match the expected evaluation AuditRunID.
func (idx *PolicyIndex) ValidateForRun(expectedRunID audit.AuditRunID) error {
	if idx == nil || len(idx.assignments) == 0 {
		return nil
	}
	if expectedRunID != "" && idx.auditRunID != "" && idx.auditRunID != expectedRunID {
		return fmt.Errorf("%w: policy audit_run_id %q does not match expected run_id %q",
			ErrPolicyRunMismatch, idx.auditRunID, expectedRunID)
	}
	for i, p := range idx.assignments {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("%w: assignment %d (%q): %w", ErrInvalidPolicy, i, p.PolicyAssignmentID, err)
		}
		if expectedRunID != "" && p.AuditRunID != expectedRunID {
			return fmt.Errorf("%w: assignment %q has audit_run_id %q, expected %q",
				ErrPolicyRunMismatch, p.PolicyAssignmentID, p.AuditRunID, expectedRunID)
		}
	}
	return nil
}
