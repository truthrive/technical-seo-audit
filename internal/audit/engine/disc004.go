package engine

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDDISC004 = "AR-DISC-004"

// evaluateDISC004 implements the typed execution logic for:
// AR-DISC-004 — Sitemap-listed URL returns final 200 (parent check: DISC-002, P1, deterministic).
//
// Subject: SubjectSitemapEntry (sme:<audit_run_id>:<sitemap_id>:<seq>)
// Unit of evaluation: The individual sitemap entry.
//
// Preconditions: URL parsed from sitemap.
// Contract:
// - PASS: The listed URL directly returns HTTP 200.
// - FAIL: The listed URL returns 3xx redirect or 4xx/5xx client/server error.
// - UNKNOWN: The listed URL cannot be fetched, HTTP status is missing/malformed/conflicting,
//            or URL correlation is missing/invalid.
// - WARNING: not used.
// - MANUAL_REVIEW: not used.
// - NOT_APPLICABLE: not used.
//
// Evidence boundaries:
// INITIAL HTTP STATUS != TERMINAL REDIRECT DESTINATION STATUS
// SITEMAP DOCUMENT STATUS != SITEMAP-LISTED URL STATUS
// URL DICTIONARY PRESENCE != HTTP FETCH EVIDENCE
// MISSING != 404
// TIMEOUT != 500
// OBSERVATION != CONCLUSION
func evaluateDISC004(
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

	entryRefs := idx.SubjectRefs(audit.SubjectSitemapEntry)
	if len(entryRefs) == 0 {
		return []audit.RuleResult{}, nil
	}

	results := make([]audit.RuleResult, 0, len(entryRefs))

	for _, subjectRef := range entryRefs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ruleResultID := audit.RuleResultID(fmt.Sprintf("rr:%s:%s:%s", snapshot.SnapshotID, rule.RuleID, subjectRef))

		// Check subjectRef format and run identity
		expectedSmePrefix := fmt.Sprintf("sme:%s:", snapshot.AuditRunID)
		subjectRefValid := strings.HasPrefix(subjectRef, expectedSmePrefix)

		// 1. Resolve loc_raw (Context)
		locRawObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "loc_raw")
		var (
			locRawUsable   bool
			locRawConflict bool
			locRawVal      string
		)
		if len(locRawObs) > 0 {
			locRawVal = strings.TrimSpace(locRawObs[0].Value)
			if locRawVal != "" {
				locRawUsable = true
				for _, lo := range locRawObs[1:] {
					if strings.TrimSpace(lo.Value) != locRawVal {
						locRawConflict = true
						locRawUsable = false
						break
					}
				}
			}
		}

		// 2. Resolve loc_normalized (Context, optional)
		locNormObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "loc_normalized")
		var (
			locNormConflict bool
			locNormVal      string
		)
		if len(locNormObs) > 0 {
			locNormVal = strings.TrimSpace(locNormObs[0].Value)
			for _, lno := range locNormObs[1:] {
				if strings.TrimSpace(lno.Value) != locNormVal {
					locNormConflict = true
					break
				}
			}
		}

		// 3. Resolve parent_sitemap_ref (Context)
		parentRefObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "parent_sitemap_ref")
		var (
			parentRefUsable   bool
			parentRefConflict bool
			parentRefVal      string
		)
		expectedSMPrefix := fmt.Sprintf("sm:%s:", snapshot.AuditRunID)
		if len(parentRefObs) > 0 {
			parentRefVal = strings.TrimSpace(parentRefObs[0].Value)
			if parentRefVal != "" && strings.HasPrefix(parentRefVal, expectedSMPrefix) {
				parentRefUsable = true
				for _, po := range parentRefObs[1:] {
					if strings.TrimSpace(po.Value) != parentRefVal {
						parentRefConflict = true
						parentRefUsable = false
						break
					}
				}
			}
		}

		// 4. Resolve sitemap_url from parent sitemap document (Context, optional)
		var (
			smURLObs      []audit.NormalizedObservation
			smURLConflict bool
		)
		if parentRefUsable {
			smURLObs = idx.GetObservations(audit.SubjectSitemap, parentRefVal, "sitemap_url")
			if len(smURLObs) > 1 {
				firstSmURL := strings.TrimSpace(smURLObs[0].Value)
				for _, smo := range smURLObs[1:] {
					if strings.TrimSpace(smo.Value) != firstSmURL {
						smURLConflict = true
						break
					}
				}
			}
		}

		// 5. Resolve url_subject_ref (Context)
		urlSubjObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "url_subject_ref")
		var (
			urlSubjUsable   bool
			urlSubjConflict bool
			urlSubjVal      string
		)
		expectedURLPrefix := fmt.Sprintf("url:%s:", snapshot.AuditRunID)
		if len(urlSubjObs) > 0 {
			urlSubjVal = strings.TrimSpace(urlSubjObs[0].Value)
			if urlSubjVal != "" && strings.HasPrefix(urlSubjVal, expectedURLPrefix) {
				urlSubjUsable = true
				for _, uo := range urlSubjObs[1:] {
					if strings.TrimSpace(uo.Value) != urlSubjVal {
						urlSubjConflict = true
						urlSubjUsable = false
						break
					}
				}
			}
		}

		// 6. Resolve entry_seq (Context, optional)
		entrySeqObs := idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "entry_seq")
		var entrySeqConflict bool
		if len(entrySeqObs) > 1 {
			firstSeq := strings.TrimSpace(entrySeqObs[0].Value)
			for _, eso := range entrySeqObs[1:] {
				if strings.TrimSpace(eso.Value) != firstSeq {
					entrySeqConflict = true
					break
				}
			}
		}

		// 7. Resolve target URL observations on SubjectURL
		var (
			targetURLObs     []audit.NormalizedObservation
			targetURLUsable  bool
			targetURLConflict bool

			statusObs       []audit.NormalizedObservation
			statusUsable    bool
			statusConflict  bool
			statusMalformed bool
			statusCode      int
			statusVal       string

			redirectObs      []audit.NormalizedObservation
			redirectConflict bool
			isRedirect       bool

			hopCountObs []audit.NormalizedObservation
			hopObs      []audit.NormalizedObservation
			fetchErrObs []audit.NormalizedObservation
		)

		if urlSubjUsable {
			// Resolve url_identity
			targetURLObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "url_identity")
			if len(targetURLObs) > 0 {
				tuVal := strings.TrimSpace(targetURLObs[0].Value)
				if tuVal != "" {
					targetURLUsable = true
					for _, tuo := range targetURLObs[1:] {
						if strings.TrimSpace(tuo.Value) != tuVal {
							targetURLConflict = true
							targetURLUsable = false
							break
						}
					}
				}
			}

			// Resolve http_status
			statusObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "http_status")
			if len(statusObs) > 0 {
				statusVal = strings.TrimSpace(statusObs[0].Value)
				statusUsable = true
				for _, so := range statusObs[1:] {
					if strings.TrimSpace(so.Value) != statusVal {
						statusConflict = true
						statusUsable = false
						break
					}
				}
				if !statusConflict {
					sc, err := strconv.Atoi(statusVal)
					if err != nil {
						statusMalformed = true
						statusUsable = false
					} else if sc <= 0 {
						// Status 0 indicates fetch failure before receiving HTTP response
						statusUsable = false
					} else {
						statusCode = sc
					}
				}
			}

			// Resolve redirect_initial_observed
			redirectObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "redirect_initial_observed")
			if len(redirectObs) > 0 {
				firstRedir := strings.TrimSpace(redirectObs[0].Value)
				if firstRedir == "true" {
					isRedirect = true
				}
				for _, ro := range redirectObs[1:] {
					if strings.TrimSpace(ro.Value) != firstRedir {
						redirectConflict = true
						break
					}
				}
			}

			// Contextual redirect and fetch error observations
			hopCountObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "redirect_hop_count")
			hopObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "redirect_hop")
			fetchErrObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "fetch_error_type")
		}

		// 8. Determine verdict and summary
		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		hasInternalContradiction := locRawConflict || locNormConflict || parentRefConflict ||
			urlSubjConflict || entrySeqConflict || smURLConflict || targetURLConflict ||
			statusConflict || redirectConflict

		if !subjectRefValid {
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap entry subject reference does not match current audit run ID."
		} else if hasInternalContradiction {
			evalStatus = audit.StatusUnknown
			observedSummary = "Contradictory observations for sitemap entry or referenced URL subject."
		} else if !locRawUsable {
			evalStatus = audit.StatusUnknown
			observedSummary = "Required sitemap entry loc evidence is unavailable."
		} else if !parentRefUsable {
			evalStatus = audit.StatusUnknown
			observedSummary = "Required parent sitemap reference is unavailable or invalid."
		} else if !urlSubjUsable {
			// No valid url_subject_ref: entry with url_id=0, orphan entry, dangling dictionary ID, or uncrawled
			evalStatus = audit.StatusUnknown
			if len(urlSubjObs) == 0 {
				observedSummary = "Sitemap entry does not correlate to a validated URL dictionary subject."
			} else {
				observedSummary = "Sitemap entry URL subject correlation reference is unusable."
			}
		} else if !targetURLUsable {
			// Target URL subject does not have usable url_identity
			evalStatus = audit.StatusUnknown
			if len(targetURLObs) == 0 {
				observedSummary = "Referenced URL subject does not exist in snapshot."
			} else {
				observedSummary = "Referenced URL subject identity evidence is unusable."
			}
		} else if len(statusObs) == 0 {
			// URL dictionary entry exists, but no fetch observation (never fetched)
			evalStatus = audit.StatusUnknown
			observedSummary = "Sitemap-listed URL has not been fetched or HTTP status evidence is unavailable."
		} else if !statusUsable {
			evalStatus = audit.StatusUnknown
			if statusMalformed {
				observedSummary = fmt.Sprintf("Sitemap-listed URL HTTP status evidence is malformed: %q.", statusVal)
			} else {
				// statusCode <= 0: fetch failed before HTTP response
				if len(fetchErrObs) > 0 && strings.TrimSpace(fetchErrObs[0].Value) != "" {
					observedSummary = fmt.Sprintf("Sitemap-listed URL fetch failed before receiving an HTTP response (%s).", strings.TrimSpace(fetchErrObs[0].Value))
				} else {
					observedSummary = "Sitemap-listed URL fetch failed before receiving an HTTP response."
				}
			}
		} else if statusCode == http.StatusOK {
			// Check if redirect evidence contradicts direct 200
			if isRedirect {
				evalStatus = audit.StatusUnknown
				observedSummary = "Contradictory evidence: sitemap-listed URL reported HTTP 200 but indicated initial redirect."
			} else {
				evalStatus = audit.StatusPass
				observedSummary = "Sitemap-listed URL directly returns HTTP 200."
			}
		} else if statusCode >= 300 && statusCode <= 399 {
			evalStatus = audit.StatusFail
			observedSummary = fmt.Sprintf("Sitemap-listed URL returned HTTP %d redirect (expected 200).", statusCode)
		} else if statusCode >= 400 && statusCode <= 599 {
			evalStatus = audit.StatusFail
			observedSummary = fmt.Sprintf("Sitemap-listed URL returned HTTP %d (expected 200).", statusCode)
		} else {
			// Other positive codes outside 200/3xx/4xx/5xx (e.g. 201, 202, 204, etc.)
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("Sitemap-listed URL returned unclassified HTTP status %d.", statusCode)
		}

		// 9. Build traceable evidence references
		var evidenceRefs []audit.RuleEvidenceRef

		// SubjectSitemapEntry: sitemap_entry_id (Context)
		for _, obs := range idx.GetObservations(audit.SubjectSitemapEntry, subjectRef, "sitemap_entry_id") {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_entry_id", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_entry_id",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: loc_raw (Context)
		for _, obs := range locRawObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:loc_raw", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "loc_raw",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: loc_normalized (Context)
		for _, obs := range locNormObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:loc_normalized", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "loc_normalized",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: parent_sitemap_ref (Context)
		for _, obs := range parentRefObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:parent_sitemap_ref", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "parent_sitemap_ref",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemap: sitemap_url (Context)
		for _, obs := range smURLObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_url", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "sitemap_url",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: url_subject_ref (Context)
		for _, obs := range urlSubjObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url_subject_ref", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "url_subject_ref",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectSitemapEntry: entry_seq (Context)
		for _, obs := range entrySeqObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:entry_seq", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "entry_seq",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectURL: url_identity (Context)
		for _, obs := range targetURLObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url_identity", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "url_identity",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectURL: http_status (Primary)
		for _, obs := range statusObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:http_status", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "http_status",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRolePrimary,
			})
		}

		// SubjectURL: redirect_initial_observed (Context)
		for _, obs := range redirectObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:redirect_initial_observed", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "redirect_initial_observed",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectURL: redirect_hop_count (Context)
		for _, obs := range hopCountObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:redirect_hop_count", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "redirect_hop_count",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectURL: redirect_hop (Context)
		for _, obs := range hopObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:redirect_hop", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "redirect_hop",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// SubjectURL: fetch_error_type (Context)
		for _, obs := range fetchErrObs {
			evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
				RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:fetch_error_type", ruleResultID, obs.ObservationID)),
				RuleResultID:      ruleResultID,
				EvidenceType:      "NORMALIZED_OBSERVATION",
				EvidenceRef:       string(obs.ObservationID),
				Field:             "fetch_error_type",
				ObservedValue:     obs.Value,
				Role:              audit.EvidenceRoleContext,
			})
		}

		// Deduplicate and sort evidence refs deterministically by RuleEvidenceRefID
		seenRefIDs := make(map[audit.RuleEvidenceRefID]struct{}, len(evidenceRefs))
		dedupedRefs := make([]audit.RuleEvidenceRef, 0, len(evidenceRefs))
		for _, ref := range evidenceRefs {
			if _, exists := seenRefIDs[ref.RuleEvidenceRefID]; !exists {
				seenRefIDs[ref.RuleEvidenceRefID] = struct{}{}
				dedupedRefs = append(dedupedRefs, ref)
			}
		}
		sort.Slice(dedupedRefs, func(i, j int) bool {
			return dedupedRefs[i].RuleEvidenceRefID < dedupedRefs[j].RuleEvidenceRefID
		})

		// Guard: PASS or FAIL requires usable inputs and affirmative evidence
		if evalStatus == audit.StatusPass || evalStatus == audit.StatusFail {
			if !locRawUsable || !parentRefUsable || !urlSubjUsable || !targetURLUsable || !statusUsable {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s produced %s without usable inputs", ruleResultID, evalStatus)
			}
			var hasLocRawContext, hasParentRefContext, hasURLSubjContext, hasStatusPrimary bool
			for _, ref := range dedupedRefs {
				if ref.Field == "loc_raw" && ref.Role == audit.EvidenceRoleContext {
					hasLocRawContext = true
				}
				if ref.Field == "parent_sitemap_ref" && ref.Role == audit.EvidenceRoleContext {
					hasParentRefContext = true
				}
				if ref.Field == "url_subject_ref" && ref.Role == audit.EvidenceRoleContext {
					hasURLSubjContext = true
				}
				if ref.Field == "http_status" && ref.Role == audit.EvidenceRolePrimary {
					hasStatusPrimary = true
				}
			}
			if !hasLocRawContext || !hasParentRefContext || !hasURLSubjContext || !hasStatusPrimary {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s missing required evidence refs (loc_raw: %v, parent_sitemap_ref: %v, url_subject_ref: %v, http_status: %v)",
					ruleResultID, hasLocRawContext, hasParentRefContext, hasURLSubjContext, hasStatusPrimary)
			}
		}

		rr := audit.RuleResult{
			RuleResultID:    ruleResultID,
			AuditRunID:      snapshot.AuditRunID,
			SnapshotID:      snapshot.SnapshotID,
			RuleID:          rule.RuleID,
			ParentCheck:     rule.ParentCheck,
			RuleVersion:     rule.RuleVersion,
			SubjectType:     audit.SubjectSitemapEntry,
			SubjectRef:      subjectRef,
			Status:          evalStatus,
			Severity:        rule.DefaultSeverity,
			Scope:           audit.ScopeURL,
			ObservedSummary: observedSummary,
			ExpectedSummary: "Sitemap-listed URL returns HTTP 200.",
			EvaluatedAt:     evalTime,
			EvidenceRefs:    dedupedRefs,
		}

		if err := validateRuleResult(rr); err != nil {
			return nil, fmt.Errorf("audit engine: generated invalid rule result for %s: %w", subjectRef, err)
		}

		results = append(results, rr)
	}

	return results, nil
}
