package engine

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

const ruleIDDISC005 = "AR-DISC-005"

// evaluateDISC005 implements the typed execution logic for:
// AR-DISC-005 — Sitemap-listed URL is not noindex (parent check: DISC-002, P1, deterministic).
//
// Subject: SubjectSitemapEntry (sme:<audit_run_id>:<sitemap_id>:<seq>)
// Unit of evaluation: The individual sitemap entry.
//
// Preconditions: listed URL is 200 and directives extractable.
// Contract:
// - PASS: direct HTTP 200 + confirmed HTML + extractable directives + effective_noindex=false.
// - FAIL: direct HTTP 200 + confirmed HTML + effective_noindex=true.
// - UNKNOWN: direct HTTP 200 + confirmed HTML + directives unavailable / missing / conflicting / malformed,
//            OR HTTP status missing/conflicting/malformed,
//            OR Content-Type missing/conflicting on 200,
//            OR URL correlation is missing/invalid/uncrawled.
// - NOT_APPLICABLE: listed URL not usable 200 HTML target
//                   (non-200 status, initial redirect, non-HTML content type, network failure before response).
// - WARNING: not used.
// - MANUAL_REVIEW: not used.
//
// Required distinctions:
// MISSING != FALSE
// HTTP 200 != HTML CONFIRMED
// NO NOINDEX OBSERVED != NOINDEX CONFIRMED ABSENT
// ROBOTS.TXT DISALLOW != META/X-ROBOTS NOINDEX
// REDIRECT DESTINATION INDEXABILITY != LISTED URL INDEXABILITY
// OBSERVATION != CONCLUSION
func evaluateDISC005(
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
			targetURLObs      []audit.NormalizedObservation
			targetURLUsable   bool
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

			ctObs      []audit.NormalizedObservation
			ctUsable   bool
			ctConflict bool
			firstCT    string
			isHTML     bool

			effObs       []audit.NormalizedObservation
			effUsable    bool
			effConflict  bool
			effMalformed bool
			firstEffStr  string
			effVal       bool

			fetchErrObs []audit.NormalizedObservation
			fetchAttObs []audit.NormalizedObservation
			metaRobObs  []audit.NormalizedObservation
			xRobObs     []audit.NormalizedObservation
			gbotMetaObs []audit.NormalizedObservation
			gbotXRobObs []audit.NormalizedObservation
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
				for _, ro := range redirectObs[1:] {
					if strings.TrimSpace(ro.Value) != firstRedir {
						redirectConflict = true
						break
					}
				}
				if !redirectConflict {
					if firstRedir == "true" {
						isRedirect = true
					}
				}
			}

			// Resolve content_type
			ctObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "content_type")
			if len(ctObs) > 0 {
				firstCT = strings.TrimSpace(ctObs[0].Value)
				if firstCT != "" {
					ctUsable = true
					for _, co := range ctObs[1:] {
						if strings.TrimSpace(co.Value) != firstCT {
							ctConflict = true
							ctUsable = false
							break
						}
					}
					if !ctConflict {
						isHTML = isHTMLContentType(firstCT)
					}
				}
			}

			// Resolve effective_noindex
			effObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "effective_noindex")
			if len(effObs) > 0 {
				firstEffStr = strings.TrimSpace(effObs[0].Value)
				effUsable = true
				for _, eo := range effObs[1:] {
					if strings.TrimSpace(eo.Value) != firstEffStr {
						effConflict = true
						effUsable = false
						break
					}
				}
				if !effConflict {
					if firstEffStr == "true" {
						effVal = true
					} else if firstEffStr == "false" {
						effVal = false
					} else {
						effMalformed = true
						effUsable = false
					}
				}
			}

			// Supplemental observations for context
			fetchErrObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "fetch_error")
			fetchAttObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "fetch_attempted")
			metaRobObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "meta_robots")
			xRobObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "x_robots")
			gbotMetaObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "googlebot_meta_robots_raw")
			gbotXRobObs = idx.GetObservations(audit.SubjectURL, urlSubjVal, "googlebot_x_robots_raw")
		}

		// Evidence reference builders
		var evidenceRefs []audit.RuleEvidenceRef

		buildEntryRefs := func() {
			if len(locRawObs) > 0 {
				for _, lo := range locRawObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:loc_raw", ruleResultID, lo.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(lo.ObservationID),
						Field:             "loc_raw",
						ObservedValue:     lo.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
			}
			if len(locNormObs) > 0 && !locNormConflict {
				for _, lno := range locNormObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:loc_normalized", ruleResultID, lno.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(lno.ObservationID),
						Field:             "loc_normalized",
						ObservedValue:     lno.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
			}
			if len(parentRefObs) > 0 {
				for _, po := range parentRefObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:parent_sitemap_ref", ruleResultID, po.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(po.ObservationID),
						Field:             "parent_sitemap_ref",
						ObservedValue:     po.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
			}
			if len(smURLObs) > 0 && !smURLConflict {
				for _, smo := range smURLObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_url", ruleResultID, smo.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(smo.ObservationID),
						Field:             "sitemap_url",
						ObservedValue:     smo.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
			}
			if len(urlSubjObs) > 0 {
				for _, uo := range urlSubjObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url_subject_ref", ruleResultID, uo.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(uo.ObservationID),
						Field:             "url_subject_ref",
						ObservedValue:     uo.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
			}
			if len(entrySeqObs) > 0 && !entrySeqConflict {
				for _, eso := range entrySeqObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:entry_seq", ruleResultID, eso.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(eso.ObservationID),
						Field:             "entry_seq",
						ObservedValue:     eso.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
			}
		}

		buildTargetURLRefs := func() {
			if len(targetURLObs) > 0 {
				for _, tuo := range targetURLObs {
					evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
						RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:url_identity", ruleResultID, tuo.ObservationID)),
						RuleResultID:      ruleResultID,
						EvidenceType:      "NORMALIZED_OBSERVATION",
						EvidenceRef:       string(tuo.ObservationID),
						Field:             "url_identity",
						ObservedValue:     tuo.Value,
						Role:              audit.EvidenceRoleContext,
					})
				}
			}
		}

		buildStatusRefs := func(role audit.EvidenceRole) {
			for _, so := range statusObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:http_status", ruleResultID, so.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(so.ObservationID),
					Field:             "http_status",
					ObservedValue:     so.Value,
					Role:              role,
				})
			}
		}

		buildContentTypeRefs := func(role audit.EvidenceRole) {
			for _, co := range ctObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:content_type", ruleResultID, co.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(co.ObservationID),
					Field:             "content_type",
					ObservedValue:     co.Value,
					Role:              role,
				})
			}
		}

		buildEffectiveNoindexRefs := func(role audit.EvidenceRole) {
			for _, eo := range effObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:effective_noindex", ruleResultID, eo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(eo.ObservationID),
					Field:             "effective_noindex",
					ObservedValue:     eo.Value,
					Role:              role,
				})
			}
		}

		buildSupplementalRefs := func() {
			for _, ro := range redirectObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:redirect_initial_observed", ruleResultID, ro.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(ro.ObservationID),
					Field:             "redirect_initial_observed",
					ObservedValue:     ro.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			for _, fo := range fetchErrObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:fetch_error", ruleResultID, fo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(fo.ObservationID),
					Field:             "fetch_error",
					ObservedValue:     fo.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			for _, fa := range fetchAttObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:fetch_attempted", ruleResultID, fa.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(fa.ObservationID),
					Field:             "fetch_attempted",
					ObservedValue:     fa.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			for _, mo := range metaRobObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:meta_robots", ruleResultID, mo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(mo.ObservationID),
					Field:             "meta_robots",
					ObservedValue:     mo.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			for _, xo := range xRobObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:x_robots", ruleResultID, xo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(xo.ObservationID),
					Field:             "x_robots",
					ObservedValue:     xo.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			for _, gmo := range gbotMetaObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:googlebot_meta_robots_raw", ruleResultID, gmo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(gmo.ObservationID),
					Field:             "googlebot_meta_robots_raw",
					ObservedValue:     gmo.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
			for _, gxo := range gbotXRobObs {
				evidenceRefs = append(evidenceRefs, audit.RuleEvidenceRef{
					RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:googlebot_x_robots_raw", ruleResultID, gxo.ObservationID)),
					RuleResultID:      ruleResultID,
					EvidenceType:      "NORMALIZED_OBSERVATION",
					EvidenceRef:       string(gxo.ObservationID),
					Field:             "googlebot_x_robots_raw",
					ObservedValue:     gxo.Value,
					Role:              audit.EvidenceRoleContext,
				})
			}
		}

		var (
			evalStatus      audit.RuleResultStatus
			observedSummary string
		)

		// Always attach entry-level evidence refs
		buildEntryRefs()

		// ---------------------------------------------------------------------
		// Step 1: Validate entry subject and core entry references
		// ---------------------------------------------------------------------
		if !subjectRefValid {
			evalStatus = audit.StatusUnknown
			observedSummary = fmt.Sprintf("Sitemap entry subject %s is malformed or belongs to an invalid audit run.", subjectRef)
		} else if !locRawUsable {
			evalStatus = audit.StatusUnknown
			if locRawConflict {
				observedSummary = "Sitemap entry loc_raw evidence is conflicting."
			} else {
				observedSummary = "Sitemap entry loc_raw evidence is missing or invalid."
			}
		} else if !parentRefUsable {
			evalStatus = audit.StatusUnknown
			if parentRefConflict {
				observedSummary = "Sitemap entry parent_sitemap_ref evidence is conflicting."
			} else {
				observedSummary = "Sitemap entry parent_sitemap_ref is missing, conflicting, or references a non-existent sitemap."
			}
		} else if !urlSubjUsable {
			evalStatus = audit.StatusUnknown
			if urlSubjConflict {
				observedSummary = "Sitemap entry url_subject_ref evidence is conflicting."
			} else if len(urlSubjObs) == 0 {
				observedSummary = "Sitemap entry has url_id=0 or unvalidated correlation; target URL subject is unavailable."
			} else {
				observedSummary = fmt.Sprintf("Sitemap entry url_subject_ref %q is invalid or cannot be correlated to a URL subject.", urlSubjVal)
			}
		} else if !targetURLUsable {
			evalStatus = audit.StatusUnknown
			if targetURLConflict {
				observedSummary = "Sitemap-listed target URL identity evidence is conflicting."
			} else {
				observedSummary = "Sitemap-listed target URL identity evidence is unavailable."
			}
			buildTargetURLRefs()
		} else {
			// Subject correlation verified! Attach target URL references
			buildTargetURLRefs()

			// -----------------------------------------------------------------
			// Step 2: Evaluate HTTP fetch outcome & status
			// -----------------------------------------------------------------
			if statusConflict {
				evalStatus = audit.StatusUnknown
				observedSummary = "Sitemap-listed URL HTTP status evidence is conflicting."
				buildStatusRefs(audit.EvidenceRoleContext)
			} else if statusMalformed {
				evalStatus = audit.StatusUnknown
				observedSummary = fmt.Sprintf("Sitemap-listed URL HTTP status evidence is malformed: %q.", statusVal)
				buildStatusRefs(audit.EvidenceRoleContext)
			} else if !statusUsable {
				// Status unavailable or <= 0
				if len(statusObs) == 0 {
					// URL was never fetched
					evalStatus = audit.StatusUnknown
					observedSummary = "Sitemap-listed URL has not been fetched or HTTP status evidence is unavailable."
				} else {
					// Fetch failed before receiving HTTP response (status <= 0)
					evalStatus = audit.StatusNotApplicable
					if len(fetchErrObs) > 0 && strings.TrimSpace(fetchErrObs[0].Value) != "" {
						observedSummary = fmt.Sprintf("Sitemap-listed URL fetch failed before receiving an HTTP response (%s); target is unusable.", strings.TrimSpace(fetchErrObs[0].Value))
					} else {
						observedSummary = "Sitemap-listed URL fetch failed before receiving an HTTP response; target is unusable."
					}
					buildStatusRefs(audit.EvidenceRoleContext)
					buildSupplementalRefs()
				}
			} else if isRedirect || (statusCode >= 300 && statusCode < 400) {
				// Initial redirect observed -> NOT_APPLICABLE (not a direct 200 HTML target)
				evalStatus = audit.StatusNotApplicable
				observedSummary = fmt.Sprintf("Sitemap-listed URL returned HTTP %d redirect (precondition requires direct HTTP 200 HTML target).", statusCode)
				buildStatusRefs(audit.EvidenceRoleContext)
				buildSupplementalRefs()
			} else if statusCode >= 400 && statusCode < 600 {
				// Client or server error -> NOT_APPLICABLE (not a usable 200 HTML target)
				evalStatus = audit.StatusNotApplicable
				observedSummary = fmt.Sprintf("Sitemap-listed URL returned HTTP %d (precondition requires direct HTTP 200 HTML target).", statusCode)
				buildStatusRefs(audit.EvidenceRoleContext)
				buildSupplementalRefs()
			} else if statusCode != 200 {
				// Any other non-200 status -> NOT_APPLICABLE
				evalStatus = audit.StatusNotApplicable
				observedSummary = fmt.Sprintf("Sitemap-listed URL returned HTTP %d (precondition requires direct HTTP 200 HTML target).", statusCode)
				buildStatusRefs(audit.EvidenceRoleContext)
				buildSupplementalRefs()
			} else {
				// Direct HTTP 200! Precondition partially met: inspect Content-Type
				buildStatusRefs(audit.EvidenceRoleContext)

				// -------------------------------------------------------------
				// Step 3: Evaluate Content-Type / HTML Applicability
				// -------------------------------------------------------------
				if ctConflict {
					evalStatus = audit.StatusUnknown
					observedSummary = "Sitemap-listed URL Content-Type evidence is conflicting."
					buildContentTypeRefs(audit.EvidenceRoleContext)
				} else if !ctUsable {
					evalStatus = audit.StatusUnknown
					observedSummary = "Sitemap-listed URL returns HTTP 200, but Content-Type evidence is unavailable to confirm HTML."
					buildContentTypeRefs(audit.EvidenceRoleContext)
				} else if !isHTML {
					// Usable non-HTML (XML, JSON, image, PDF, etc.) -> NOT_APPLICABLE
					evalStatus = audit.StatusNotApplicable
					observedSummary = fmt.Sprintf("Sitemap-listed URL returns non-HTML content type %q (precondition requires usable 200 HTML target).", firstCT)
					buildContentTypeRefs(audit.EvidenceRoleContext)
				} else {
					// Confirmed direct HTTP 200 HTML target!
					buildContentTypeRefs(audit.EvidenceRoleContext)

					// ---------------------------------------------------------
					// Step 4: Evaluate effective_noindex directives state
					// ---------------------------------------------------------
					if !effUsable {
						evalStatus = audit.StatusUnknown
						if len(effObs) == 0 {
							observedSummary = "Sitemap-listed URL returns HTTP 200 HTML, but effective_noindex evidence is unavailable."
						} else if effConflict {
							observedSummary = "Sitemap-listed URL effective_noindex evidence is conflicting."
						} else if effMalformed {
							observedSummary = fmt.Sprintf("Sitemap-listed URL effective_noindex evidence is malformed: %q.", firstEffStr)
						}
						buildEffectiveNoindexRefs(audit.EvidenceRolePrimary)
						buildSupplementalRefs()
					} else if !effVal {
						// effective_noindex = false -> PASS
						evalStatus = audit.StatusPass
						observedSummary = "Sitemap-listed URL returns HTTP 200 HTML and has effective_noindex=false."
						buildEffectiveNoindexRefs(audit.EvidenceRolePrimary)
						buildSupplementalRefs()
					} else {
						// effective_noindex = true -> FAIL
						evalStatus = audit.StatusFail
						observedSummary = "Sitemap-listed URL returns HTTP 200 HTML but has effective_noindex=true."
						buildEffectiveNoindexRefs(audit.EvidenceRolePrimary)
						buildSupplementalRefs()
					}
				}
			}
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
			if !locRawUsable || !parentRefUsable || !urlSubjUsable || !targetURLUsable || !statusUsable || statusCode != 200 || isRedirect || !ctUsable || !isHTML || !effUsable {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s produced %s without usable inputs", ruleResultID, evalStatus)
			}
			var hasLocRawContext, hasParentRefContext, hasURLSubjContext, hasStatusContext, hasCTContext, hasEffectivePrimary bool
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
				if ref.Field == "http_status" && ref.Role == audit.EvidenceRoleContext {
					hasStatusContext = true
				}
				if ref.Field == "content_type" && ref.Role == audit.EvidenceRoleContext {
					hasCTContext = true
				}
				if ref.Field == "effective_noindex" && ref.Role == audit.EvidenceRolePrimary {
					hasEffectivePrimary = true
				}
			}
			if !hasLocRawContext || !hasParentRefContext || !hasURLSubjContext || !hasStatusContext || !hasCTContext || !hasEffectivePrimary {
				return nil, fmt.Errorf("audit engine: internal error: rule result %s missing required evidence refs (loc_raw: %v, parent_sitemap_ref: %v, url_subject_ref: %v, http_status: %v, content_type: %v, effective_noindex: %v)",
					ruleResultID, hasLocRawContext, hasParentRefContext, hasURLSubjContext, hasStatusContext, hasCTContext, hasEffectivePrimary)
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
			ExpectedSummary: "Sitemap-listed URL does not contain effective noindex directive.",
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
