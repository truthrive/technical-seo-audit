package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDINDEX002 = "AR-INDEX-002"

// rawDirectiveBundle aggregates normalized observations belonging to one distinct
// directive evidence reference.
type rawDirectiveBundle struct {
	directiveRef  string
	sources       []audit.NormalizedObservation
	targets       []audit.NormalizedObservation
	raws          []audit.NormalizedObservation
	tokens        []audit.NormalizedObservation
	scopeUnknowns []audit.NormalizedObservation

	hasIndex   bool
	hasNoindex bool
}

// allObservations returns all normalized observations in this bundle.
func (b *rawDirectiveBundle) allObservations() []audit.NormalizedObservation {
	var all []audit.NormalizedObservation
	all = append(all, b.tokens...)
	all = append(all, b.raws...)
	all = append(all, b.targets...)
	all = append(all, b.sources...)
	all = append(all, b.scopeUnknowns...)
	return all
}

// extractDirectiveRef extracts the stable directive reference identifier from
// an observation's SourceEvidenceRefs slice.
func extractDirectiveRef(refs []string) string {
	for _, r := range refs {
		if strings.HasPrefix(r, "directive:") {
			return r
		}
	}
	for _, r := range refs {
		if !strings.HasPrefix(r, "page:") && !strings.HasPrefix(r, "sitecrawl_pages:") {
			return r
		}
	}
	if len(refs) > 0 {
		return refs[0]
	}
	return ""
}

// evaluateINDEX002 implements the typed execution logic for:
// AR-INDEX-002 — Conflicting index directives (parent check: INDEX-001, P1, deterministic).
//
// Contract:
// - PASS: No contradictory index and noindex directives in the same applicable scope.
// - FAIL: Contradictory index and noindex directives explicitly present in the same applicable scope.
// - UNKNOWN: Directives exist but applicability or parsing is not reliable, or URL identity is unusable.
// - NOT_APPLICABLE: No robots directive exists.
//
// Status precedence:
//  1. Missing or conflicting URL identity -> UNKNOWN
//  2. No directive evidence -> NOT_APPLICABLE
//  3. Confirmed known-scope index + noindex -> FAIL
//  4. No confirmed conflict, but any directive evidence has unknown/unreliable scope -> UNKNOWN
//  5. Otherwise -> PASS
func evaluateINDEX002(
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

	urlRefs := idx.SubjectRefs(audit.SubjectURL)
	results := make([]audit.RuleResult, 0, len(urlRefs))

	for _, subjectRef := range urlRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// 1. Resolve URL identity input
		urlObs := idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
		var (
			urlUsable   bool
			urlConflict bool
			firstURL    string
		)

		if len(urlObs) > 0 {
			firstURL = strings.TrimSpace(urlObs[0].Value)
			if firstURL != "" {
				urlUsable = true
				for _, u := range urlObs[1:] {
					if strings.TrimSpace(u.Value) != firstURL {
						urlConflict = true
						urlUsable = false
						break
					}
				}
			}
		}

		if !urlUsable {
			var observedSummary string
			if urlConflict {
				observedSummary = "No usable URL identity is available: conflicting URL identity observations."
			} else {
				observedSummary = "Required URL identity evidence is unavailable."
			}

			var evidenceRefs []audit.RuleEvidenceRef
			for _, u := range urlObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url", ruleResultID, u.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(u.ObservationID),
					Field:             "url",
					ObservedValue:     u.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			sort.Slice(evidenceRefs, func(i, j int) bool {
				return evidenceRefs[i].RuleEvidenceRefID < evidenceRefs[j].RuleEvidenceRefID
			})

			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     audit.SubjectURL,
				SubjectRef:      subjectRef,
				Status:          audit.StatusUnknown,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: observedSummary,
				ExpectedSummary: "No contradictory index and noindex directives in the same applicable scope.",
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// 2. Resolve V1.3b2 directive bundle observations
		srcObs := idx.GetObservations(audit.SubjectURL, subjectRef, "directive_source")
		tgtObs := idx.GetObservations(audit.SubjectURL, subjectRef, "directive_target")
		rawObs := idx.GetObservations(audit.SubjectURL, subjectRef, "directive_raw")
		tokObs := idx.GetObservations(audit.SubjectURL, subjectRef, "directive_tokens")
		unkObs := idx.GetObservations(audit.SubjectURL, subjectRef, "directive_scope_unknown")

		totalDirectiveObs := len(srcObs) + len(tgtObs) + len(rawObs) + len(tokObs) + len(unkObs)

		// URL context evidence helper
		buildURLContextRefs := func() []audit.RuleEvidenceRef {
			var refs []audit.RuleEvidenceRef
			for _, u := range urlObs {
				refs = append(refs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url", ruleResultID, u.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(u.ObservationID),
					Field:             "url",
					ObservedValue:     u.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			return refs
		}

		if totalDirectiveObs == 0 {
			// Status precedence 2: No directive evidence -> NOT_APPLICABLE
			evidenceRefs := buildURLContextRefs()
			sort.Slice(evidenceRefs, func(i, j int) bool {
				return evidenceRefs[i].RuleEvidenceRefID < evidenceRefs[j].RuleEvidenceRefID
			})

			rr := audit.RuleResult{
				RuleResultID:    ruleResultID,
				AuditRunID:      snapshot.AuditRunID,
				SnapshotID:      snapshot.SnapshotID,
				RuleID:          rule.RuleID,
				ParentCheck:     rule.ParentCheck,
				RuleVersion:     rule.RuleVersion,
				SubjectType:     audit.SubjectURL,
				SubjectRef:      subjectRef,
				Status:          audit.StatusNotApplicable,
				Severity:        rule.DefaultSeverity,
				Scope:           audit.ScopeURL,
				ObservedSummary: "No robots directive exists.",
				ExpectedSummary: "No contradictory index and noindex directives in the same applicable scope.",
				EvaluatedAt:     evalTime,
				EvidenceRefs:    evidenceRefs,
			}
			if err := validateRuleResult(rr); err != nil {
				return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
			}
			results = append(results, rr)
			continue
		}

		// Reconstruct directive bundles using stable directive evidence reference
		bundleMap := make(map[string]*rawDirectiveBundle)
		var bundleKeys []string
		var unreferencedObs []audit.NormalizedObservation

		getOrCreateBundle := func(ref string) *rawDirectiveBundle {
			b, ok := bundleMap[ref]
			if !ok {
				b = &rawDirectiveBundle{directiveRef: ref}
				bundleMap[ref] = b
				bundleKeys = append(bundleKeys, ref)
			}
			return b
		}

		for _, obs := range srcObs {
			ref := extractDirectiveRef(obs.SourceEvidenceRefs)
			if ref == "" {
				unreferencedObs = append(unreferencedObs, obs)
			} else {
				getOrCreateBundle(ref).sources = append(getOrCreateBundle(ref).sources, obs)
			}
		}
		for _, obs := range tgtObs {
			ref := extractDirectiveRef(obs.SourceEvidenceRefs)
			if ref == "" {
				unreferencedObs = append(unreferencedObs, obs)
			} else {
				getOrCreateBundle(ref).targets = append(getOrCreateBundle(ref).targets, obs)
			}
		}
		for _, obs := range rawObs {
			ref := extractDirectiveRef(obs.SourceEvidenceRefs)
			if ref == "" {
				unreferencedObs = append(unreferencedObs, obs)
			} else {
				getOrCreateBundle(ref).raws = append(getOrCreateBundle(ref).raws, obs)
			}
		}
		for _, obs := range tokObs {
			ref := extractDirectiveRef(obs.SourceEvidenceRefs)
			if ref == "" {
				unreferencedObs = append(unreferencedObs, obs)
			} else {
				getOrCreateBundle(ref).tokens = append(getOrCreateBundle(ref).tokens, obs)
			}
		}
		for _, obs := range unkObs {
			ref := extractDirectiveRef(obs.SourceEvidenceRefs)
			if ref == "" {
				unreferencedObs = append(unreferencedObs, obs)
			} else {
				getOrCreateBundle(ref).scopeUnknowns = append(getOrCreateBundle(ref).scopeUnknowns, obs)
			}
		}

		sort.Strings(bundleKeys)

		var (
			hasAmbiguousDirectives bool
			ambiguousObs           []audit.NormalizedObservation
			knownScopeBundles      = make(map[string][]*rawDirectiveBundle)
			evaluatedKnownBundles  []*rawDirectiveBundle
		)

		if len(unreferencedObs) > 0 {
			hasAmbiguousDirectives = true
			ambiguousObs = append(ambiguousObs, unreferencedObs...)
		}

		for _, bKey := range bundleKeys {
			b := bundleMap[bKey]

			// Check scope unknown flag
			isScopeUnknown := false
			for _, uo := range b.scopeUnknowns {
				if strings.TrimSpace(strings.ToLower(uo.Value)) == "true" {
					isScopeUnknown = true
					break
				}
			}

			// Validate target
			var targetVal string
			var targetConflict bool
			if len(b.targets) > 0 {
				firstTgt := strings.TrimSpace(b.targets[0].Value)
				targetVal = firstTgt
				for _, to := range b.targets[1:] {
					if strings.TrimSpace(to.Value) != firstTgt {
						targetConflict = true
						break
					}
				}
			}

			// Validate tokens
			var tokensConflict bool
			if len(b.tokens) > 0 {
				firstTok := strings.TrimSpace(b.tokens[0].Value)
				for _, to := range b.tokens[1:] {
					if strings.TrimSpace(to.Value) != firstTok {
						tokensConflict = true
						break
					}
				}
			}

			// Parse tokens for explicit index/noindex tokens
			if len(b.tokens) > 0 && !tokensConflict {
				for _, part := range strings.Split(b.tokens[0].Value, ",") {
					t := strings.ToLower(strings.TrimSpace(part))
					if t == "index" {
						b.hasIndex = true
					} else if t == "noindex" {
						b.hasNoindex = true
					}
				}
			}

			isKnownAndReliable := !isScopeUnknown && !targetConflict && targetVal != "" && !tokensConflict && len(b.tokens) > 0

			if isKnownAndReliable {
				normScope := strings.ToLower(targetVal)
				knownScopeBundles[normScope] = append(knownScopeBundles[normScope], b)
				evaluatedKnownBundles = append(evaluatedKnownBundles, b)
			} else {
				hasAmbiguousDirectives = true
				ambiguousObs = append(ambiguousObs, b.allObservations()...)
			}
		}

		// Evaluate conflicts per known scope
		conflictingScopes := make(map[string][]*rawDirectiveBundle)
		for scopeName, bundles := range knownScopeBundles {
			scopeHasIndex := false
			scopeHasNoindex := false
			var decidingBundles []*rawDirectiveBundle

			for _, b := range bundles {
				if b.hasIndex {
					scopeHasIndex = true
				}
				if b.hasNoindex {
					scopeHasNoindex = true
				}
				if b.hasIndex || b.hasNoindex {
					decidingBundles = append(decidingBundles, b)
				}
			}

			if scopeHasIndex && scopeHasNoindex {
				conflictingScopes[scopeName] = decidingBundles
			}
		}

		hasConfirmedConflict := len(conflictingScopes) > 0

		// Status precedence evaluation
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
			evidenceRefs    = buildURLContextRefs()
		)

		if hasConfirmedConflict {
			// Precedence 3: Confirmed known-scope conflict -> FAIL
			evalStatus = audit.StatusFail

			var confScopeNames []string
			for sc := range conflictingScopes {
				confScopeNames = append(confScopeNames, sc)
			}
			sort.Strings(confScopeNames)

			if len(confScopeNames) == 1 {
				observedSummary = fmt.Sprintf("Contradictory index and noindex directives observed in scope %q.", confScopeNames[0])
			} else {
				observedSummary = fmt.Sprintf("Contradictory index and noindex directives observed in scopes: %s.", strings.Join(confScopeNames, ", "))
			}

			// FAIL must reference the observations that prove both index and noindex
			// inside the conflicting scope (deciding bundles only).
			seenObs := make(map[audit.ObservationID]struct{})
			for _, sc := range confScopeNames {
				for _, db := range conflictingScopes[sc] {
					for _, obs := range db.allObservations() {
						if _, exists := seenObs[obs.ObservationID]; !exists {
							seenObs[obs.ObservationID] = struct{}{}
							evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
								RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, obs.ObservationID, obs.Field)),
								RuleResultID:      ruleResultID,
								EvidenceType:      "NORMALIZED_OBSERVATION",
								EvidenceRef:       string(obs.ObservationID),
								Field:             obs.Field,
								ObservedValue:     obs.Value,
								Role:              audit.EvidenceRolePrimary,
							})
						}
					}
				}
			}
		} else if hasAmbiguousDirectives {
			// Precedence 4: No confirmed conflict, but ambiguous directive evidence exists -> UNKNOWN
			evalStatus = audit.StatusUnknown
			observedSummary = "Directives exist but applicability or parsing is not reliable."

			seenObs := make(map[audit.ObservationID]struct{})
			for _, obs := range ambiguousObs {
				if _, exists := seenObs[obs.ObservationID]; !exists {
					seenObs[obs.ObservationID] = struct{}{}
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, obs.ObservationID, obs.Field)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(obs.ObservationID),
						Field:             obs.Field,
						ObservedValue:     obs.Value,
						Role:              audit.EvidenceRolePrimary,
					})
				}
			}
		} else {
			// Precedence 5: Otherwise -> PASS
			evalStatus = audit.StatusPass
			observedSummary = "No contradictory index and noindex directives observed in the same applicable scope."

			seenObs := make(map[audit.ObservationID]struct{})
			for _, kb := range evaluatedKnownBundles {
				for _, obs := range kb.allObservations() {
					if _, exists := seenObs[obs.ObservationID]; !exists {
						seenObs[obs.ObservationID] = struct{}{}
						evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
							RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, obs.ObservationID, obs.Field)),
							RuleResultID:      ruleResultID,
							EvidenceType:      "NORMALIZED_OBSERVATION",
							EvidenceRef:       string(obs.ObservationID),
							Field:             obs.Field,
							ObservedValue:     obs.Value,
							Role:              audit.EvidenceRolePrimary,
						})
					}
				}
			}
		}

		sort.Slice(evidenceRefs, func(i, j int) bool {
			return evidenceRefs[i].RuleEvidenceRefID < evidenceRefs[j].RuleEvidenceRefID
		})

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     audit.SubjectURL,
			SubjectRef:      subjectRef,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeURL,
			ObservedSummary: observedSummary,
			ExpectedSummary: "No contradictory index and noindex directives in the same applicable scope.",
			EvaluatedAt:     evalTime,
			EvidenceRefs:    evidenceRefs,
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
		}

		results = append(results, rr)
	}

	return results, nil
}
