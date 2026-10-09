package engine

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

// redirectHopPayload represents the JSON payload of a normalized "redirect_hop" observation.
type redirectHopPayload struct {
	HopIndex          int    `json:"hop_index"`
	SourceURL         string `json:"source_url"`
	Status            int    `json:"status"`
	ResolvedTargetURL string `json:"resolved_target_url"`
}

// redirectEvidenceBundle aggregates and validates all redirect-related observations
// for a single URL subject within a frozen EvidenceSnapshot.
type redirectEvidenceBundle struct {
	SubjectRef string

	// URL Identity
	URLIdentityObs []audit.NormalizedObservation
	URL            string
	URLUsable      bool
	URLConflict    bool

	// redirect_initial_observed
	InitialObs        []audit.NormalizedObservation
	InitialUsable     bool
	InitialConflict   bool
	IsInitialRedirect bool

	// redirect_hop_count
	HopCountObs       []audit.NormalizedObservation
	HopCountUsable    bool
	HopCountConflict  bool
	HopCountMalformed bool
	HopCount          int

	// redirect_hop
	HopObs       []audit.NormalizedObservation
	HopMalformed bool
	HopConflict  bool
	HopIndexDup  bool
	HopNonContig bool
	UniqueHops   []redirectHopPayload

	// redirect_traversal_complete
	CompleteObs       []audit.NormalizedObservation
	CompleteUsable    bool
	CompleteConflict  bool
	CompleteMalformed bool
	TraversalComplete bool

	// redirect_loop_detected
	LoopObs       []audit.NormalizedObservation
	LoopUsable    bool
	LoopConflict  bool
	LoopMalformed bool
	LoopDetected  bool

	// redirect_final_url
	FinalURLObs []audit.NormalizedObservation
	FinalURL    string
}

// extractRedirectEvidence extracts and validates all redirect-related observations for a subject.
func extractRedirectEvidence(idx *EvidenceIndex, subjectRef string) redirectEvidenceBundle {
	b := redirectEvidenceBundle{
		SubjectRef: subjectRef,
	}

	// 1. url_identity
	b.URLIdentityObs = idx.GetObservations(audit.SubjectURL, subjectRef, "url_identity")
	if len(b.URLIdentityObs) > 0 {
		first := strings.TrimSpace(b.URLIdentityObs[0].Value)
		if first != "" {
			b.URLUsable = true
			b.URL = first
			for _, o := range b.URLIdentityObs[1:] {
				if strings.TrimSpace(o.Value) != first {
					b.URLConflict = true
					b.URLUsable = false
					break
				}
			}
		}
	}

	// 2. redirect_initial_observed
	b.InitialObs = idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_initial_observed")
	if len(b.InitialObs) > 0 {
		first := strings.TrimSpace(b.InitialObs[0].Value)
		if first == "true" || first == "false" {
			b.InitialUsable = true
			b.IsInitialRedirect = (first == "true")
			for _, o := range b.InitialObs[1:] {
				if strings.TrimSpace(o.Value) != first {
					b.InitialConflict = true
					b.InitialUsable = false
					break
				}
			}
		} else {
			b.InitialConflict = true
			b.InitialUsable = false
		}
	}

	// 3. redirect_hop_count
	b.HopCountObs = idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_hop_count")
	if len(b.HopCountObs) > 0 {
		first := strings.TrimSpace(b.HopCountObs[0].Value)
		b.HopCountUsable = true
		for _, o := range b.HopCountObs[1:] {
			if strings.TrimSpace(o.Value) != first {
				b.HopCountConflict = true
				b.HopCountUsable = false
				break
			}
		}
		if !b.HopCountConflict {
			c, err := strconv.Atoi(first)
			if err != nil || c < 0 {
				b.HopCountMalformed = true
				b.HopCountUsable = false
			} else {
				b.HopCount = c
			}
		}
	}

	// 4. redirect_hop
	b.HopObs = idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_hop")
	if len(b.HopObs) > 0 {
		seenPayloads := make(map[string]redirectHopPayload)
		seenIndices := make(map[int]redirectHopPayload)

		for _, o := range b.HopObs {
			var p redirectHopPayload
			if err := json.Unmarshal([]byte(o.Value), &p); err != nil {
				b.HopMalformed = true
				break
			}
			if p.HopIndex < 0 || strings.TrimSpace(p.SourceURL) == "" || p.Status < 100 || p.Status >= 600 || strings.TrimSpace(p.ResolvedTargetURL) == "" {
				b.HopMalformed = true
				break
			}

			key := fmt.Sprintf("%d|%s|%d|%s", p.HopIndex, p.SourceURL, p.Status, p.ResolvedTargetURL)
			if existing, exists := seenIndices[p.HopIndex]; exists {
				existingKey := fmt.Sprintf("%d|%s|%d|%s", existing.HopIndex, existing.SourceURL, existing.Status, existing.ResolvedTargetURL)
				if existingKey != key {
					b.HopIndexDup = true
					b.HopConflict = true
					break
				}
				// Identical duplicate hop observation: accepted
			} else {
				seenIndices[p.HopIndex] = p
				seenPayloads[key] = p
			}
		}

		if !b.HopMalformed && !b.HopConflict && !b.HopIndexDup {
			hops := make([]redirectHopPayload, 0, len(seenPayloads))
			for _, p := range seenPayloads {
				hops = append(hops, p)
			}
			sort.Slice(hops, func(i, j int) bool {
				return hops[i].HopIndex < hops[j].HopIndex
			})
			b.UniqueHops = hops

			// Check contiguity: indices must be exactly 0, 1, ..., len(hops)-1
			for i, h := range hops {
				if h.HopIndex != i {
					b.HopNonContig = true
					break
				}
			}
		}
	}

	// 5. redirect_traversal_complete
	b.CompleteObs = idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_traversal_complete")
	if len(b.CompleteObs) > 0 {
		first := strings.TrimSpace(b.CompleteObs[0].Value)
		if first == "true" || first == "false" {
			b.CompleteUsable = true
			b.TraversalComplete = (first == "true")
			for _, o := range b.CompleteObs[1:] {
				if strings.TrimSpace(o.Value) != first {
					b.CompleteConflict = true
					b.CompleteUsable = false
					break
				}
			}
		} else {
			b.CompleteMalformed = true
		}
	}

	// 6. redirect_loop_detected
	b.LoopObs = idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_loop_detected")
	if len(b.LoopObs) > 0 {
		first := strings.TrimSpace(b.LoopObs[0].Value)
		if first == "true" || first == "false" {
			b.LoopUsable = true
			b.LoopDetected = (first == "true")
			for _, o := range b.LoopObs[1:] {
				if strings.TrimSpace(o.Value) != first {
					b.LoopConflict = true
					b.LoopUsable = false
					break
				}
			}
		} else {
			b.LoopMalformed = true
		}
	}

	// 7. redirect_final_url
	b.FinalURLObs = idx.GetObservations(audit.SubjectURL, subjectRef, "redirect_final_url")
	if len(b.FinalURLObs) > 0 {
		first := strings.TrimSpace(b.FinalURLObs[0].Value)
		if first != "" {
			b.FinalURL = first
		}
	}

	return b
}

// hasConflictingRedirectEvidence returns true if redirect observations exist that contradict
// an initial non-redirect verdict (e.g. hops recorded, traversal completed, or loop detected).
func (b *redirectEvidenceBundle) hasConflictingRedirectEvidence() bool {
	return len(b.HopObs) > 0 ||
		(b.HopCountUsable && b.HopCount > 0) ||
		(b.CompleteUsable && b.TraversalComplete) ||
		(b.LoopUsable && b.LoopDetected) ||
		len(b.FinalURLObs) > 0
}

// buildEvidenceRefs converts normalized observations into RuleEvidenceRefs with the specified role.
func buildEvidenceRefs(ruleResultID audit.RuleResultID, observations []audit.NormalizedObservation, field string, role audit.EvidenceRole) []audit.RuleEvidenceRef {
	refs := make([]audit.RuleEvidenceRef, 0, len(observations))
	for _, obs := range observations {
		refs = append(refs, audit.RuleEvidenceRef{
			RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:%s", ruleResultID, obs.ObservationID, field)),
			RuleResultID:      ruleResultID,
			EvidenceType:      "NORMALIZED_OBSERVATION",
			EvidenceRef:       string(obs.ObservationID),
			Field:             field,
			ObservedValue:     obs.Value,
			Role:              role,
		})
	}
	return refs
}

// dedupeAndSortEvidenceRefs removes duplicate RuleEvidenceRefs by ID and sorts them deterministically.
func dedupeAndSortEvidenceRefs(refs []audit.RuleEvidenceRef) []audit.RuleEvidenceRef {
	seen := make(map[audit.RuleEvidenceRefID]struct{}, len(refs))
	out := make([]audit.RuleEvidenceRef, 0, len(refs))
	for _, r := range refs {
		if _, exists := seen[r.RuleEvidenceRefID]; !exists {
			seen[r.RuleEvidenceRefID] = struct{}{}
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RuleEvidenceRefID < out[j].RuleEvidenceRefID
	})
	return out
}
