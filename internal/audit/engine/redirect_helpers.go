package engine

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
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
	InitialMalformed  bool
	IsInitialRedirect bool

	// redirect_hop_count
	HopCountObs       []audit.NormalizedObservation
	HopCountUsable    bool
	HopCountConflict  bool
	HopCountMalformed bool
	HopCount          int

	// redirect_hop
	HopObs                []audit.NormalizedObservation
	HopMalformed          bool
	HopConflict           bool
	HopIndexDup           bool
	HopNonContig          bool
	HopChainInconsistent  bool
	UniqueHops            []redirectHopPayload

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

// isValidHTTPURL validates that a raw URL string is a valid, absolute HTTP or HTTPS URL.
func isValidHTTPURL(raw string) bool {
	if raw == "" || strings.ContainsAny(raw, " \t\r\n") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return false
	}
	if parsed.Host == "" {
		return false
	}
	return true
}

// parseAndValidateHopJSON parses a single "redirect_hop" observation JSON string.
// Enforces required keys, exact expected types, 3xx status, and valid HTTP(S) URLs.
func parseAndValidateHopJSON(val string) (redirectHopPayload, bool) {
	valBytes := []byte(strings.TrimSpace(val))
	if len(valBytes) == 0 {
		return redirectHopPayload{}, false
	}

	var rawMap map[string]json.RawMessage
	if err := json.Unmarshal(valBytes, &rawMap); err != nil || rawMap == nil {
		return redirectHopPayload{}, false
	}

	// 1. Validate required JSON keys explicitly
	rawIndex, hasIndex := rawMap["hop_index"]
	rawSource, hasSource := rawMap["source_url"]
	rawStatus, hasStatus := rawMap["status"]
	rawTarget, hasTarget := rawMap["resolved_target_url"]
	if !hasIndex || !hasSource || !hasStatus || !hasTarget {
		return redirectHopPayload{}, false
	}

	// 2. Validate hop_index: must be an exact integer >= 0
	var rawIndexVal any
	decIndex := json.NewDecoder(bytes.NewReader(rawIndex))
	decIndex.UseNumber()
	if err := decIndex.Decode(&rawIndexVal); err != nil {
		return redirectHopPayload{}, false
	}
	indexNum, ok := rawIndexVal.(json.Number)
	if !ok {
		return redirectHopPayload{}, false
	}
	indexInt64, err := indexNum.Int64()
	if err != nil || indexInt64 < 0 || indexInt64 > math.MaxInt {
		return redirectHopPayload{}, false
	}
	hopIndex := int(indexInt64)

	// 3. Validate status: must be an exact integer and 3xx (300 <= status <= 399)
	var rawStatusVal any
	decStatus := json.NewDecoder(bytes.NewReader(rawStatus))
	decStatus.UseNumber()
	if err := decStatus.Decode(&rawStatusVal); err != nil {
		return redirectHopPayload{}, false
	}
	statusNum, ok := rawStatusVal.(json.Number)
	if !ok {
		return redirectHopPayload{}, false
	}
	statusInt64, err := statusNum.Int64()
	if err != nil || statusInt64 < 300 || statusInt64 > 399 {
		return redirectHopPayload{}, false
	}
	status := int(statusInt64)

	// 4. Validate source_url: must be string and valid HTTP(S) URL
	var rawSourceVal any
	if err := json.Unmarshal(rawSource, &rawSourceVal); err != nil {
		return redirectHopPayload{}, false
	}
	sourceStr, ok := rawSourceVal.(string)
	if !ok {
		return redirectHopPayload{}, false
	}
	sourceStr = strings.TrimSpace(sourceStr)
	if !isValidHTTPURL(sourceStr) {
		return redirectHopPayload{}, false
	}

	// 5. Validate resolved_target_url: must be string and valid HTTP(S) URL
	var rawTargetVal any
	if err := json.Unmarshal(rawTarget, &rawTargetVal); err != nil {
		return redirectHopPayload{}, false
	}
	targetStr, ok := rawTargetVal.(string)
	if !ok {
		return redirectHopPayload{}, false
	}
	targetStr = strings.TrimSpace(targetStr)
	if !isValidHTTPURL(targetStr) {
		return redirectHopPayload{}, false
	}

	return redirectHopPayload{
		HopIndex:          hopIndex,
		SourceURL:         sourceStr,
		Status:            status,
		ResolvedTargetURL: targetStr,
	}, true
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
			b.InitialMalformed = true
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
			p, valid := parseAndValidateHopJSON(o.Value)
			if !valid {
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

	// 8. Verify full URL-to-URL continuity of redirect chain
	if !b.HopMalformed && !b.HopConflict && !b.HopIndexDup && !b.HopNonContig && len(b.UniqueHops) > 0 {
		// hop[0].source_url must match URL identity exactly (no fuzzy matching, no variant collapsing)
		if b.URLUsable && b.UniqueHops[0].SourceURL != b.URL {
			b.HopChainInconsistent = true
		}

		// hop[i].resolved_target_url must match hop[i+1].source_url exactly
		for i := 0; i < len(b.UniqueHops)-1; i++ {
			if b.UniqueHops[i].ResolvedTargetURL != b.UniqueHops[i+1].SourceURL {
				b.HopChainInconsistent = true
				break
			}
		}

		// If traversal complete and redirect_final_url is present, final hop target must match it
		if b.CompleteUsable && b.TraversalComplete && len(b.FinalURLObs) > 0 && b.FinalURL != "" {
			if b.UniqueHops[len(b.UniqueHops)-1].ResolvedTargetURL != b.FinalURL {
				b.HopChainInconsistent = true
			}
		}
	}

	return b
}

// isHopEvidenceValidAndConsistent checks if all required redirect hop and count observations
// are present, unconflicted, fully parsed, contiguous, and continuous.
func (b *redirectEvidenceBundle) isHopEvidenceValidAndConsistent() bool {
	if len(b.HopCountObs) == 0 || !b.HopCountUsable || b.HopCountConflict || b.HopCountMalformed || b.HopCount < 1 {
		return false
	}
	if len(b.HopObs) == 0 || b.HopMalformed || b.HopConflict || b.HopIndexDup || b.HopNonContig || b.HopChainInconsistent {
		return false
	}
	if len(b.UniqueHops) != b.HopCount {
		return false
	}
	return true
}

// hasConflictingRedirectEvidence returns true if redirect observations exist that contradict
// an initial non-redirect verdict (e.g. hops recorded, traversal completed, or loop detected).
func (b *redirectEvidenceBundle) hasConflictingRedirectEvidence() bool {
	return len(b.HopObs) > 0 ||
		(len(b.HopCountObs) > 0 && (!b.HopCountUsable || b.HopCount > 0)) ||
		(len(b.CompleteObs) > 0 && (!b.CompleteUsable || b.TraversalComplete)) ||
		(len(b.LoopObs) > 0 && (!b.LoopUsable || b.LoopDetected)) ||
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
