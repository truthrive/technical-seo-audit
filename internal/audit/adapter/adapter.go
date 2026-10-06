package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// Build processes a completed SiteCrawl run from SQLite and constructs the
// normalized audit evidence and a frozen EvidenceSnapshot.
func Build(ctx context.Context, db *sql.DB, req BuildRequest) (*BuildResult, error) {
	// 1. Validate inputs
	if req.CrawlRunID == "" {
		return nil, ErrEmptyCrawlRunID
	}
	if req.AuditRunID == "" {
		return nil, ErrEmptyAuditRunID
	}
	if req.SnapshotID == "" {
		return nil, ErrEmptySnapshotID
	}

	for i, policy := range req.ProjectPolicyAssignments {
		if policy.AuditRunID != req.AuditRunID {
			return nil, fmt.Errorf("%w: policy at index %d has audit_run_id %q, expected %q",
				ErrPolicyRunMismatch, i, policy.AuditRunID, req.AuditRunID)
		}
		if err := policy.Validate(); err != nil {
			return nil, fmt.Errorf("%w: policy at index %d (%s) failed validation: %v",
				ErrInvalidPolicy, i, policy.PolicyKey, err)
		}
	}

	// 2. Open single read-only transaction for consistent source view
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("audit adapter: begin read tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Query and verify crawl run state
	var (
		runState, stopReason, seedURL, host, optJSON, startedAtStr string
		finishedAtStr                                             sql.NullString
		foundCount, crawledCount                                  int
	)
	err = tx.QueryRowContext(ctx,
		`SELECT state, stop_reason, seed_url, host, options, started_at, finished_at, found, crawled
		 FROM sitecrawl_runs WHERE id = ?`, req.CrawlRunID).
		Scan(&runState, &stopReason, &seedURL, &host, &optJSON, &startedAtStr, &finishedAtStr, &foundCount, &crawledCount)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("%w: %q", ErrRunNotFound, req.CrawlRunID)
	}
	if err != nil {
		return nil, fmt.Errorf("audit adapter: query run: %w", err)
	}

	if runState != sitecrawl.StateCompleted {
		return nil, fmt.Errorf("%w: current run state is %q (only %q runs can be audited)",
			ErrRunNotCompleted, runState, sitecrawl.StateCompleted)
	}

	// Crawl completeness semantics:
	// Snapshot metadata (EvidenceSnapshot.CrawlComplete) is the canonical source for
	// crawl completeness. In Audit V1.2, CrawlComplete is true if and only if
	// the crawl run reached StateCompleted AND stop_reason is empty (i.e. frontier
	// was fully exhausted without hitting limits such as max-urls, max-depth, etc.).
	// Completed runs with a non-empty stop_reason still produce a frozen snapshot
	// with CrawlComplete = false and emit a crawl_stop_reason observation.
	crawlComplete := (runState == sitecrawl.StateCompleted && strings.TrimSpace(stopReason) == "")

	var opts sitecrawl.Options
	if optJSON != "" && optJSON != "{}" {
		if err := json.Unmarshal([]byte(optJSON), &opts); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrMalformedOptionsJSON, err)
		}
	}

	runStartedAt, err := time.Parse(time.RFC3339, startedAtStr)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformedRunTimestamp, err)
	}

	// 3. Load URL dictionary
	urlRows, err := tx.QueryContext(ctx,
		`SELECT id, url FROM sitecrawl_urls WHERE run_id = ? ORDER BY id ASC`, req.CrawlRunID)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: query urls: %w", err)
	}
	defer urlRows.Close()

	urlsByID := make(map[int]string)
	idsByURL := make(map[string]int)
	for urlRows.Next() {
		var (
			id  int
			raw string
		)
		if err := urlRows.Scan(&id, &raw); err != nil {
			return nil, fmt.Errorf("audit adapter: scan url: %w", err)
		}
		urlsByID[id] = raw
		idsByURL[raw] = id
	}
	if err := urlRows.Err(); err != nil {
		return nil, fmt.Errorf("audit adapter: read urls: %w", err)
	}

	// Extract and sort numeric URL IDs to guarantee deterministic ordering
	urlIDs := make([]int, 0, len(urlsByID))
	for id := range urlsByID {
		urlIDs = append(urlIDs, id)
	}
	sort.Ints(urlIDs)

	// 4. Load page rows
	type rawPageRecord struct {
		urlID        int
		url          string
		dataJSON     string
		kind         string
		isInternal   int
		depth        int
		discoveredBy string
		status       int
		contentType  string
		sizeBytes    int64
		responseMs   int
		redirectTo   string
		redirectHops int
		errorType    string
		title        string
		metaDesc     string
		h1           string
		lang         string
		canonical    string
		metaRobots   string
		xRobots      string
		rendered     int
		crawledAt    string
		robotsState  string
		page         sitecrawl.Page
	}

	pageRows, err := tx.QueryContext(ctx,
		`SELECT url_id, url, data, kind, is_internal, depth, discovered_by,
		        status, content_type, size_bytes, response_ms, redirect_to,
		        redirect_hops, error_type, title, meta_desc, h1, lang, canonical,
		        meta_robots, x_robots, rendered, crawled_at, robots_state
		 FROM sitecrawl_pages WHERE run_id = ? ORDER BY url_id ASC`, req.CrawlRunID)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: query pages: %w", err)
	}
	defer pageRows.Close()

	pagesByURLID := make(map[int]*rawPageRecord)
	for pageRows.Next() {
		var pr rawPageRecord
		if err := pageRows.Scan(
			&pr.urlID, &pr.url, &pr.dataJSON, &pr.kind, &pr.isInternal, &pr.depth, &pr.discoveredBy,
			&pr.status, &pr.contentType, &pr.sizeBytes, &pr.responseMs, &pr.redirectTo,
			&pr.redirectHops, &pr.errorType, &pr.title, &pr.metaDesc, &pr.h1, &pr.lang, &pr.canonical,
			&pr.metaRobots, &pr.xRobots, &pr.rendered, &pr.crawledAt, &pr.robotsState,
		); err != nil {
			return nil, fmt.Errorf("audit adapter: scan page: %w", err)
		}

		if pr.crawledAt == "" {
			return nil, fmt.Errorf("%w: page url_id %d has empty crawled_at", ErrMalformedPageTimestamp, pr.urlID)
		}
		if _, err := time.Parse(time.RFC3339, pr.crawledAt); err != nil {
			return nil, fmt.Errorf("%w: page url_id %d crawled_at %q: %v", ErrMalformedPageTimestamp, pr.urlID, pr.crawledAt, err)
		}

		if pr.dataJSON != "" {
			if err := json.Unmarshal([]byte(pr.dataJSON), &pr.page); err != nil {
				return nil, fmt.Errorf("%w: page url_id %d: %v", ErrMalformedPageJSON, pr.urlID, err)
			}
		}
		pagesByURLID[pr.urlID] = &pr
	}
	if err := pageRows.Err(); err != nil {
		return nil, fmt.Errorf("audit adapter: read pages: %w", err)
	}

	// 5. Load links and build incoming anchor graph & LinkObservations
	type linkRow struct {
		srcID     int
		dstID     int
		seq       int
		placement int
		flags     uint32
		anchor    string
	}

	linkRows, err := tx.QueryContext(ctx,
		`SELECT src_id, dst_id, seq, placement, flags, anchor
		 FROM sitecrawl_links WHERE run_id = ? ORDER BY src_id ASC, seq ASC`, req.CrawlRunID)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: query links: %w", err)
	}
	defer linkRows.Close()

	var (
		evidenceGaps     []EvidenceGap
		linkObservations []audit.LinkObservation
	)
	incomingAnchorEdges := make(map[int][]linkRow)

	for linkRows.Next() {
		var lr linkRow
		if err := linkRows.Scan(&lr.srcID, &lr.dstID, &lr.seq, &lr.placement, &lr.flags, &lr.anchor); err != nil {
			return nil, fmt.Errorf("audit adapter: scan link: %w", err)
		}

		isResource := (lr.flags&(sitecrawl.FlagImageLink|sitecrawl.FlagStylesheet|sitecrawl.FlagScript) != 0) ||
			(lr.placement == sitecrawl.PlacementImage)

		// Record legitimate internal anchor links for discovery provenance (only if dstID > 0)
		if !isResource && (lr.flags&sitecrawl.FlagInternal != 0) && lr.dstID > 0 {
			incomingAnchorEdges[lr.dstID] = append(incomingAnchorEdges[lr.dstID], lr)
		}

		// Build LinkObservation only for non-resource anchor edges
		if !isResource {
			var (
				targetURLID       *audit.URLID
				targetURLResolved string
			)

			if lr.dstID > 0 {
				if resolved, ok := urlsByID[lr.dstID]; ok && resolved != "" {
					tid := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, lr.dstID))
					targetURLID = &tid
					targetURLResolved = resolved
				}
			}

			// Dangling link target guard: dst_id == 0 or unallocated dictionary target
			if targetURLID == nil {
				evidenceGaps = append(evidenceGaps, EvidenceGap{
					GapCode:         GapUnresolvedLinkTargetUnavailable,
					SubjectRef:      fmt.Sprintf("link:%s:%d:%d", req.AuditRunID, lr.srcID, lr.seq),
					Field:           "target_url_id",
					Reason:          fmt.Sprintf("Link edge from url %d at seq %d points to destination id %d which is not present in URL dictionary (e.g. allocation cap reached); target URL ID is unavailable.", lr.srcID, lr.seq, lr.dstID),
					SourceComponent: "sitecrawl_links",
				})
			}

			var location string
			switch lr.placement {
			case sitecrawl.PlacementNav:
				location = "NAV"
			case sitecrawl.PlacementHeader:
				location = "HEADER"
			case sitecrawl.PlacementFooter:
				location = "FOOTER"
			case sitecrawl.PlacementBody:
				location = "UNKNOWN"
			default:
				location = "UNKNOWN"
			}

			var obsTime time.Time
			if p, ok := pagesByURLID[lr.srcID]; ok {
				t, err := time.Parse(time.RFC3339, p.crawledAt)
				if err == nil {
					obsTime = t
				}
			}
			if obsTime.IsZero() {
				obsTime = runStartedAt
			}

			linkObs := audit.LinkObservation{
				LinkID:              audit.LinkID(fmt.Sprintf("link:%s:%d:%d", req.AuditRunID, lr.srcID, lr.seq)),
				AuditRunID:          req.AuditRunID,
				SourceURLID:         audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, lr.srcID)),
				TargetURLID:         targetURLID,
				TargetURLRaw:        "", // Raw href not persisted by SiteCrawl
				TargetURLResolved:   targetURLResolved,
				ElementTag:          "a",
				HREFRaw:             "", // Raw href not persisted by SiteCrawl
				AnchorText:          lr.anchor,
				LinkLocation:        location,
				NavigationCandidate: false,
				ObservedAt:          obsTime,
			}
			linkObservations = append(linkObservations, linkObs)
		}
	}
	if err := linkRows.Err(); err != nil {
		return nil, fmt.Errorf("audit adapter: read links: %w", err)
	}

	// Commit read transaction after all source records are safely read
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("audit adapter: commit read tx: %w", err)
	}

	// 6. Initialize EvidenceSnapshot in BUILDING state
	snapshot := &audit.EvidenceSnapshot{
		SnapshotID:               req.SnapshotID,
		AuditRunID:               req.AuditRunID,
		CreatedAt:                time.Now().UTC(),
		SnapshotStatus:           audit.SnapshotBuilding,
		NormalizationVersion:     "v1.2.0",
		CrawlComplete:            crawlComplete,
		SitemapDiscoveryComplete: false,
		RenderSelectionComplete:  false,
		ProbeCollectionComplete:  false,
	}

	// 7. Build UrlResource objects in deterministic order
	var urlResources []audit.UrlResource
	for _, urlID := range urlIDs {
		rawURL := urlsByID[urlID]
		normURL, fragRemoved, err := normalizeURL(rawURL)
		if err != nil {
			return nil, fmt.Errorf("audit adapter: url %d: %w", urlID, err)
		}
		normParsed, _ := url.Parse(normURL)

		var portInt int
		if p := normParsed.Port(); p != "" {
			portInt, _ = strconv.Atoi(p)
		}
		origin := normParsed.Scheme + "://" + normParsed.Host

		var isInternal *bool
		if pr, exists := pagesByURLID[urlID]; exists {
			v := pr.isInternal == 1
			isInternal = &v
		} else if edges, exists := incomingAnchorEdges[urlID]; exists && len(edges) > 0 {
			v := true
			isInternal = &v
		}

		ur := audit.UrlResource{
			URLID:           audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, urlID)),
			AuditRunID:      req.AuditRunID,
			URL:             rawURL,
			NormalizedURL:   normURL,
			Scheme:          normParsed.Scheme,
			Host:            normParsed.Hostname(),
			Port:            portInt,
			Path:            normParsed.EscapedPath(),
			Query:           normParsed.RawQuery,
			FragmentRemoved: fragRemoved,
			IsInternal:      isInternal,
			Origin:          origin,
			CreatedAt:       nil, // Not persisted in SQLite
		}
		urlResources = append(urlResources, ur)
	}

	// 8. Build DiscoveryRecords with strict provenance and multiple-record support
	var discoveryRecords []audit.DiscoveryRecord

	for _, urlID := range urlIDs {
		rawURL := urlsByID[urlID]
		urlIDStr := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, urlID))
		pr := pagesByURLID[urlID]
		provenanceCount := 0

		// 8a. Seed URL discovery
		if rawURL == seedURL {
			discoveryRecords = append(discoveryRecords, audit.DiscoveryRecord{
				DiscoveryID:   audit.DiscoveryID(fmt.Sprintf("disc:%s:%d:%s", req.AuditRunID, urlID, audit.DiscoveryStartURL)),
				AuditRunID:    req.AuditRunID,
				URLID:         urlIDStr,
				DiscoveryType: audit.DiscoveryStartURL,
				SourceURLID:   nil,
				SourceRef:     "sitecrawl_runs:" + req.CrawlRunID,
				DiscoveredAt:  runStartedAt,
			})
			provenanceCount++
		}

		// 8b. Supplied URL list discovery (only when proven by SourceManual / list admission)
		if pr != nil && (pr.discoveredBy == sitecrawl.SourceManual || pr.page.Source == sitecrawl.SourceManual) {
			discoveryRecords = append(discoveryRecords, audit.DiscoveryRecord{
				DiscoveryID:   audit.DiscoveryID(fmt.Sprintf("disc:%s:%d:%s", req.AuditRunID, urlID, audit.DiscoverySuppliedURLList)),
				AuditRunID:    req.AuditRunID,
				URLID:         urlIDStr,
				DiscoveryType: audit.DiscoverySuppliedURLList,
				SourceURLID:   nil,
				SourceRef:     "sitecrawl_runs:" + req.CrawlRunID + ":list",
				DiscoveredAt:  runStartedAt,
			})
			provenanceCount++
		}

		// 8c. Sitemap discovery
		if pr != nil && (pr.discoveredBy == sitecrawl.SourceSitemap || pr.page.Source == sitecrawl.SourceSitemap) {
			obsTime, _ := time.Parse(time.RFC3339, pr.crawledAt)
			if obsTime.IsZero() {
				obsTime = runStartedAt
			}
			discoveryRecords = append(discoveryRecords, audit.DiscoveryRecord{
				DiscoveryID:   audit.DiscoveryID(fmt.Sprintf("disc:%s:%d:%s", req.AuditRunID, urlID, audit.DiscoverySitemap)),
				AuditRunID:    req.AuditRunID,
				URLID:         urlIDStr,
				DiscoveryType: audit.DiscoverySitemap,
				SourceURLID:   nil,
				SourceRef:     fmt.Sprintf("sitecrawl_pages:%s:%d", req.CrawlRunID, urlID),
				DiscoveredAt:  obsTime,
			})
			provenanceCount++
		}

		// 8d. Internal link discovery (MUST be verified by an incoming non-resource anchor link)
		if edges, ok := incomingAnchorEdges[urlID]; ok && len(edges) > 0 {
			firstEdge := edges[0]
			srcURLID := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, firstEdge.srcID))
			obsTime := runStartedAt
			if p, ok := pagesByURLID[firstEdge.srcID]; ok {
				t, err := time.Parse(time.RFC3339, p.crawledAt)
				if err == nil {
					obsTime = t
				}
			}

			discoveryRecords = append(discoveryRecords, audit.DiscoveryRecord{
				DiscoveryID:   audit.DiscoveryID(fmt.Sprintf("disc:%s:%d:%s", req.AuditRunID, urlID, audit.DiscoveryInternalLink)),
				AuditRunID:    req.AuditRunID,
				URLID:         urlIDStr,
				DiscoveryType: audit.DiscoveryInternalLink,
				SourceURLID:   &srcURLID,
				SourceRef:     fmt.Sprintf("sitecrawl_links:%s:%d:%d", req.CrawlRunID, firstEdge.srcID, firstEdge.seq),
				DiscoveredAt:  obsTime,
			})
			provenanceCount++
		}

		// 8e. Ambiguous or non-anchor provenance
		if provenanceCount == 0 && pr != nil && (pr.discoveredBy == sitecrawl.SourceLink || pr.page.Source == sitecrawl.SourceLink) {
			evidenceGaps = append(evidenceGaps, EvidenceGap{
				GapCode:         GapDiscoveryProvenanceAmbiguous,
				SubjectRef:      string(urlIDStr),
				Field:           "discovery_record",
				Reason:          "Page is marked with source 'link' but has no supporting internal anchor edge in sitecrawl_links (e.g. canonical or hreflang reference).",
				SourceComponent: "sitecrawl_pages",
			})
		}
	}

	// 9. Build FetchObservations, RedirectHops, HtmlObservations, Directives, Canonicals in deterministic order
	var (
		fetchObservations           []audit.FetchObservation
		redirectHops                []audit.RedirectHop
		htmlObservations            []audit.HtmlObservation
		robotsDirectiveObservations []audit.RobotsDirectiveObservation
		canonicalObservations       []audit.CanonicalObservation
		normalizedObservations      []audit.NormalizedObservation
		obsSeq                      int
	)

	nextObsID := func() audit.ObservationID {
		obsSeq++
		return audit.ObservationID(fmt.Sprintf("obs:%s:%d", req.SnapshotID, obsSeq))
	}

	configuredProfile := mapRequestProfile(opts.UserAgent)

	for _, urlID := range urlIDs {
		pr, exists := pagesByURLID[urlID]
		if !exists {
			continue
		}
		urlIDStr := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, urlID))
		fetchID := audit.FetchID(fmt.Sprintf("fetch:%s:%d", req.AuditRunID, urlID))

		obsTime, _ := time.Parse(time.RFC3339, pr.crawledAt)

		// Acquisition purpose determination
		var purpose *audit.AcquisitionPurpose
		if pr.discoveredBy == sitecrawl.SourceRedirect || pr.page.Source == sitecrawl.SourceRedirect {
			p := audit.PurposeRedirectTarget
			purpose = &p
		} else if pr.url == seedURL ||
			pr.discoveredBy == sitecrawl.SourceSitemap || pr.page.Source == sitecrawl.SourceSitemap ||
			pr.discoveredBy == sitecrawl.SourceManual || pr.page.Source == sitecrawl.SourceManual ||
			len(incomingAnchorEdges[urlID]) > 0 {
			p := audit.PurposeCrawl
			purpose = &p
		} else {
			purpose = nil
			evidenceGaps = append(evidenceGaps, EvidenceGap{
				GapCode:         GapAcquisitionPurposeAmbiguous,
				SubjectRef:      string(urlIDStr),
				Field:           "acquisition_purpose",
				Reason:          "Page fetch occurred without confirmed anchor, seed, sitemap, or redirect provenance; acquisition purpose is unavailable.",
				SourceComponent: "sitecrawl_pages",
			})
		}

		// FetchAttempted determination
		fetchAttempted := true
		isRobotsBlocked := (pr.robotsState == sitecrawl.RobotsBlocked || pr.page.RobotsState == sitecrawl.RobotsBlocked)
		if isRobotsBlocked && pr.status == 0 && pr.errorType == "" {
			fetchAttempted = false
		}

		// Fetch error normalization
		fetchError, errGap := normalizeFetchError(pr.errorType, string(urlIDStr))
		if errGap != nil {
			evidenceGaps = append(evidenceGaps, *errGap)
		}

		// Effective request profile: account for bot-blocked Chrome fallback
		effectiveProfile := configuredProfile
		if pr.page.BotBlocked {
			effectiveProfile = audit.ProfileDefault
			evidenceGaps = append(evidenceGaps, EvidenceGap{
				GapCode:         GapBotResponseNotPreserved,
				SubjectRef:      string(urlIDStr),
				Field:           "request_profile",
				Reason:          fmt.Sprintf("Configured bot request (%s) was refused with 403 and retried with browser fallback (%s); the original bot response was discarded by crawler and is not preserved as a FetchObservation.", configuredProfile, audit.ProfileDefault),
				SourceComponent: "sitecrawl_fetch",
			})
		}

		var finalURLID *audit.URLID
		if pr.redirectTo != "" {
			if fid, ok := idsByURL[pr.redirectTo]; ok {
				fidStr := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, fid))
				finalURLID = &fidStr
			}
		} else if len(pr.page.Redirects) > 0 {
			lastHop := pr.page.Redirects[len(pr.page.Redirects)-1]
			if fid, ok := idsByURL[lastHop.Location]; ok {
				fidStr := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, fid))
				finalURLID = &fidStr
			}
		}

		fetchObs := audit.FetchObservation{
			FetchID:             fetchID,
			AuditRunID:          req.AuditRunID,
			URLID:               urlIDStr,
			AcquisitionPurpose:  purpose,
			RequestProfile:      effectiveProfile,
			RequestedAt:         nil, // Not stored
			CompletedAt:         nil, // Not stored
			FetchAttempted:      fetchAttempted,
			Status:              pr.status,
			FinalURLID:          finalURLID,
			ContentType:         pr.contentType,
			ResponseTimeMs:      int64(pr.responseMs),
			FetchErrorType:      fetchError,
			TLSValid:            nil, // Not verified
			ChallengeDetected:   nil, // Not verified
			ResponseHeadersRef:  "",
			BodyArtifactRef:     "",
			ObservedAt:          obsTime,
		}
		fetchObservations = append(fetchObservations, fetchObs)

		pageSrcRef := fmt.Sprintf("sitecrawl_pages:%s:%d", req.CrawlRunID, urlID)

		// Normalized observations for transport / HTTP
		normalizedObservations = append(normalizedObservations,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "url_identity",
				Value:              pr.url,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
		)

		if fetchAttempted || pr.status > 0 {
			normalizedObservations = append(normalizedObservations,
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "http_status",
					Value:              strconv.Itoa(pr.status),
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				},
			)
		}

		if pr.contentType != "" {
			normalizedObservations = append(normalizedObservations,
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "content_type",
					Value:              pr.contentType,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				},
			)
		}

		normalizedObservations = append(normalizedObservations,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "crawl_depth",
				Value:              strconv.Itoa(pr.depth),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			},
		)

		if pr.responseMs > 0 {
			normalizedObservations = append(normalizedObservations,
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "response_time_ms",
					Value:              strconv.Itoa(pr.responseMs),
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				},
			)
		}

		if fetchError != "" {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "fetch_error_type",
				Value:              fetchError,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}

		if purpose != nil {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "acquisition_purpose",
				Value:              string(*purpose),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}

		// 9b. Redirect Hops
		if len(pr.page.Redirects) > 0 {
			for hopIdx, hop := range pr.page.Redirects {
				resolvedTarget := hop.Location
				if baseParsed, err := url.Parse(hop.URL); err == nil {
					if refParsed, err := url.Parse(hop.Location); err == nil {
						resolvedTarget = baseParsed.ResolveReference(refParsed).String()
					}
				}

				redirectHops = append(redirectHops, audit.RedirectHop{
					RedirectHopID:     audit.RedirectHopID(fmt.Sprintf("hop:%s:%d:%d", req.AuditRunID, urlID, hopIdx)),
					FetchID:           fetchID,
					HopIndex:          hopIdx,
					SourceURL:         hop.URL,
					Status:            hop.Status,
					LocationRaw:       hop.Location,
					ResolvedTargetURL: resolvedTarget,
					ObservedAt:        obsTime,
				})
			}
		}

		// 9c. HTML Observations
		isHTML := pr.kind == sitecrawl.KindHTML || strings.Contains(pr.contentType, "html")
		if isHTML {
			isRendered := (pr.rendered == 1 || pr.page.Rendered)
			if isRendered {
				evidenceGaps = append(evidenceGaps, EvidenceGap{
					GapCode:         GapRenderedRawSourceUnavailable,
					SubjectRef:      string(urlIDStr),
					Field:           "raw_html_evidence",
					Reason:          "Page was rendered via headless browser; raw server-rendered HTML was replaced and is unavailable.",
					SourceComponent: "sitecrawl_pages",
				})

				normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "rendered",
					Value:              "true",
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				})
			}

			var xRobotsRaw []string
			if pr.page.XRobotsTag != "" {
				xRobotsRaw = append(xRobotsRaw, pr.page.XRobotsTag)
			} else if pr.xRobots != "" {
				xRobotsRaw = append(xRobotsRaw, pr.xRobots)
			}

			if isRendered {
				// Rendered page: DO NOT emit raw-style title, meta description, H1, meta robots, or canonicals.
				// Preserved: X-Robots-Tag (HTTP response header) and rendered = true fact.
				htmlObs := audit.HtmlObservation{
					HTMLObservationID:   audit.HtmlObservationID(fmt.Sprintf("html:%s:%d", req.AuditRunID, urlID)),
					AuditRunID:          req.AuditRunID,
					URLID:               urlIDStr,
					FetchID:             fetchID,
					Title:               "",
					MetaDescription:     "",
					H1Values:            nil,
					MetaRobotsRaw:       nil,
					XRobotsRaw:          xRobotsRaw,
					CanonicalRawValues:  nil,
					MainTextPresent:     nil,
					MainTextFingerprint: "",
					ContentFingerprint:  "",
					ObservedAt:          obsTime,
				}
				htmlObservations = append(htmlObservations, htmlObs)

				if len(xRobotsRaw) > 0 {
					hasHeaderNoindex := false
					for dirIdx, raw := range xRobotsRaw {
						tokens := parseDirectiveTokens(raw)
						isNoindex := containsToken(tokens, "noindex")
						if isNoindex {
							hasHeaderNoindex = true
						}
						robotsDirectiveObservations = append(robotsDirectiveObservations, audit.RobotsDirectiveObservation{
							RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:header:%d", req.AuditRunID, urlID, dirIdx)),
							AuditRunID:                   req.AuditRunID,
							URLID:                        urlIDStr,
							Source:                       audit.DirectiveSourceHTTPHeader,
							RawValue:                     raw,
							ParsedTokens:                 tokens,
							EffectiveNoindex:             isNoindex,
							ObservedAt:                   obsTime,
						})
					}
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "x_robots_raw",
						Value:              strings.Join(xRobotsRaw, ", "),
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})
					noindexVal := "false"
					if hasHeaderNoindex {
						noindexVal = "true"
					}
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "effective_noindex",
						Value:              noindexVal,
						DerivationType:     audit.DerivationNormalized,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})
				}
			} else {
				// Non-rendered page: extract verified raw HTML fields
				h1Values := pr.page.H1
				if len(h1Values) == 0 && pr.h1 != "" {
					h1Values = []string{pr.h1}
				}

				var metaRobotsRaw []string
				if pr.page.MetaRobots != "" {
					metaRobotsRaw = append(metaRobotsRaw, pr.page.MetaRobots)
				} else if pr.metaRobots != "" {
					metaRobotsRaw = append(metaRobotsRaw, pr.metaRobots)
				}

				titleVal := pr.page.Title
				if titleVal == "" {
					titleVal = pr.title
				}
				metaDescVal := pr.page.MetaDesc
				if metaDescVal == "" {
					metaDescVal = pr.metaDesc
				}

				htmlObs := audit.HtmlObservation{
					HTMLObservationID:   audit.HtmlObservationID(fmt.Sprintf("html:%s:%d", req.AuditRunID, urlID)),
					AuditRunID:          req.AuditRunID,
					URLID:               urlIDStr,
					FetchID:             fetchID,
					Title:               titleVal,
					MetaDescription:     metaDescVal,
					H1Values:            h1Values,
					MetaRobotsRaw:       metaRobotsRaw,
					XRobotsRaw:          xRobotsRaw,
					CanonicalRawValues:  nil, // Raw canonical syntax unavailable
					MainTextPresent:     nil, // Not verified
					MainTextFingerprint: "",
					ContentFingerprint:  "",
					ObservedAt:          obsTime,
				}
				htmlObservations = append(htmlObservations, htmlObs)

				// Normalized HTML fields
				if titleVal != "" {
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "title",
						Value:              titleVal,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})
				}
				if metaDescVal != "" {
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "meta_description",
						Value:              metaDescVal,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})
				}
				for _, h1 := range h1Values {
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "h1",
						Value:              h1,
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})
				}

				// Robots directives
				hasNoindex := false
				if len(metaRobotsRaw) > 0 {
					for dirIdx, raw := range metaRobotsRaw {
						tokens := parseDirectiveTokens(raw)
						isNoindex := containsToken(tokens, "noindex")
						if isNoindex {
							hasNoindex = true
						}
						robotsDirectiveObservations = append(robotsDirectiveObservations, audit.RobotsDirectiveObservation{
							RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:meta:%d", req.AuditRunID, urlID, dirIdx)),
							AuditRunID:                   req.AuditRunID,
							URLID:                        urlIDStr,
							Source:                       audit.DirectiveSourceMeta,
							RawValue:                     raw,
							ParsedTokens:                 tokens,
							EffectiveNoindex:             isNoindex,
							ObservedAt:                   obsTime,
						})
					}
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "meta_robots_raw",
						Value:              strings.Join(metaRobotsRaw, ", "),
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})
				}

				if len(xRobotsRaw) > 0 {
					for dirIdx, raw := range xRobotsRaw {
						tokens := parseDirectiveTokens(raw)
						isNoindex := containsToken(tokens, "noindex")
						if isNoindex {
							hasNoindex = true
						}
						robotsDirectiveObservations = append(robotsDirectiveObservations, audit.RobotsDirectiveObservation{
							RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:header:%d", req.AuditRunID, urlID, dirIdx)),
							AuditRunID:                   req.AuditRunID,
							URLID:                        urlIDStr,
							Source:                       audit.DirectiveSourceHTTPHeader,
							RawValue:                     raw,
							ParsedTokens:                 tokens,
							EffectiveNoindex:             isNoindex,
							ObservedAt:                   obsTime,
						})
					}
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "x_robots_raw",
						Value:              strings.Join(xRobotsRaw, ", "),
						DerivationType:     audit.DerivationDirect,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})
				}

				if len(metaRobotsRaw) > 0 || len(xRobotsRaw) > 0 {
					noindexVal := "false"
					if hasNoindex {
						noindexVal = "true"
					}
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "effective_noindex",
						Value:              noindexVal,
						DerivationType:     audit.DerivationNormalized,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})
				}

				// Canonical observations
				canonicals := pr.page.Canonicals
				if len(canonicals) == 0 && (pr.page.Canonical != "" || pr.canonical != "") {
					c := pr.page.Canonical
					if c == "" {
						c = pr.canonical
					}
					canonicals = []string{c}
				}

				if len(canonicals) > 0 {
					var targetIDs []audit.URLID
					for _, c := range canonicals {
						if tid, ok := idsByURL[c]; ok {
							targetIDs = append(targetIDs, audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, tid)))
						}
					}

					canonicalObservations = append(canonicalObservations, audit.CanonicalObservation{
						CanonicalObservationID: audit.CanonicalObservationID(fmt.Sprintf("canon:%s:%d", req.AuditRunID, urlID)),
						AuditRunID:             req.AuditRunID,
						URLID:                  urlIDStr,
						CanonicalCount:         len(canonicals),
						RawValues:              nil, // Raw canonical syntax unavailable
						NormalizedValues:       canonicals,
						ResolvedTargetURLIDs:   targetIDs,
						ParseErrors:            nil,
						ObservedAt:             obsTime,
					})

					for _, c := range canonicals {
						normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
							ObservationID:      nextObsID(),
							AuditRunID:         req.AuditRunID,
							SnapshotID:         req.SnapshotID,
							SubjectType:        audit.SubjectURL,
							SubjectRef:         string(urlIDStr),
							Field:              "canonical_resolved",
							Value:              c,
							DerivationType:     audit.DerivationNormalized,
							SourceEvidenceRefs: []string{pageSrcRef},
							ObservedAt:         obsTime,
						})
					}
				}
			}
		}
	}

	// 10. Add normalized observations for links
	for _, lo := range linkObservations {
		linkSrcRef := fmt.Sprintf("sitecrawl_links:%s", strings.TrimPrefix(string(lo.LinkID), "link:"+string(req.AuditRunID)+":"))
		if lo.TargetURLResolved != "" {
			normalizedObservations = append(normalizedObservations,
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         string(lo.LinkID),
					Field:              "link_target",
					Value:              lo.TargetURLResolved,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{linkSrcRef},
					ObservedAt:         lo.ObservedAt,
				},
			)
		}
		normalizedObservations = append(normalizedObservations,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         string(lo.LinkID),
				Field:              "link_anchor",
				Value:              lo.AnchorText,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{linkSrcRef},
				ObservedAt:         lo.ObservedAt,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         string(lo.LinkID),
				Field:              "link_location",
				Value:              lo.LinkLocation,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{linkSrcRef},
				ObservedAt:         lo.ObservedAt,
			},
		)
	}

	// 10b. Site-level normalized observations for crawl completeness and stop reason
	siteSrcRef := fmt.Sprintf("sitecrawl_runs:%s", req.CrawlRunID)
	normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
		ObservationID:      nextObsID(),
		AuditRunID:         req.AuditRunID,
		SnapshotID:         req.SnapshotID,
		SubjectType:        audit.SubjectSite,
		SubjectRef:         "site",
		Field:              "crawl_complete",
		Value:              strconv.FormatBool(crawlComplete),
		DerivationType:     audit.DerivationDirect,
		SourceEvidenceRefs: []string{siteSrcRef},
		ObservedAt:         runStartedAt,
	})

	if strings.TrimSpace(stopReason) != "" {
		normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectSite,
			SubjectRef:         "site",
			Field:              "crawl_stop_reason",
			Value:              stopReason,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{siteSrcRef},
			ObservedAt:         runStartedAt,
		})
	}

	// 11. Append conditional and global capability-level evidence gaps
	if len(canonicalObservations) > 0 {
		evidenceGaps = append(evidenceGaps, EvidenceGap{
			GapCode:         GapRawCanonicalUnavailable,
			Field:           "raw_canonical_values",
			Reason:          "SiteCrawl normalizes and resolves canonical references before SQLite persistence; raw canonical declaration syntax is unavailable.",
			SourceComponent: "sitecrawl_pages",
		})
	}
	if len(linkObservations) > 0 {
		evidenceGaps = append(evidenceGaps, EvidenceGap{
			GapCode:         GapRawHrefUnavailable,
			Field:           "href_raw",
			Reason:          "SiteCrawl resolves link targets before SQLite persistence; original raw href attributes are not preserved.",
			SourceComponent: "sitecrawl_links",
		})
	}

	evidenceGaps = append(evidenceGaps,
		EvidenceGap{
			GapCode:         GapFetchTimingUnavailable,
			Field:           "requested_at / completed_at",
			Reason:          "SiteCrawl records crawled_at and response_ms duration, but does not preserve discrete HTTP request start/end timestamps.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapRetryHistoryUnavailable,
			Field:           "retry_attempts",
			Reason:          "SiteCrawl persists only the final attempt result per URL, without individual retry attempt history.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapTLSValidityUnavailable,
			Field:           "tls_valid",
			Reason:          "SiteCrawl does not record TLS certificate validity checks; ssl-error is recorded only on hard connection failure.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapChallengeDetectionUnavailable,
			Field:           "challenge_detected",
			Reason:          "BotBlocked flag records bot-profile fallback, not generic challenge detection response telemetry.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapResponseHeadersUnavailable,
			Field:           "response_headers_ref",
			Reason:          "SiteCrawl does not store complete HTTP response header payloads in SQLite.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapBodyArtifactUnavailable,
			Field:           "body_artifact_ref",
			Reason:          "Raw HTML response bodies are parsed via streaming tokenization and discarded without SQLite persistence.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapMainTextUnavailable,
			Field:           "main_text_present",
			Reason:          "SiteCrawl stores word_count and text_ratio, but does not execute semantic main text block extraction.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapRobotsDocumentUnavailable,
			Field:           "robots_document_observation",
			Reason:          "SiteCrawl does not preserve complete raw robots.txt artifacts or parsed AST records in SQLite.",
			SourceComponent: "sitecrawl",
		},
		EvidenceGap{
			GapCode:         GapRobotsDecisionUnavailable,
			Field:           "robots_decision",
			Reason:          "SiteCrawl stores RobotsState string without granular per-agent matched pattern and rule index telemetry.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapSitemapDocumentUnavailable,
			Field:           "sitemap_observation",
			Reason:          "SiteCrawl consumes sitemaps during frontier scheduling but does not persist formal SitemapObservation or SitemapEntry entities.",
			SourceComponent: "sitecrawl",
		},
		EvidenceGap{
			GapCode:         GapStructuredDataStatusUnavailable,
			Field:           "structured_data_block",
			Reason:          "SiteCrawl stores JSONLD and SchemaOrg JSON blobs without formal V1 parse-status evaluation contracts.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapRenderComparisonUnavailable,
			Field:           "render_field_comparison",
			Reason:          "SiteCrawl rendered pages overwrite raw extraction fields; independent raw and rendered HTML states are not co-preserved.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapProbeObservationUnavailable,
			Field:           "probe_observation",
			Reason:          "No synthetic diagnostic probe suite (known-missing, host-variant) exists in standalone SiteCrawl.",
			SourceComponent: "sitecrawl",
		},
		EvidenceGap{
			GapCode:         GapProfileComparisonUnavailable,
			Field:           "profile_comparison_observation",
			Reason:          "Comparative multi-bot profile differential requests are not executed by the standalone crawler.",
			SourceComponent: "sitecrawl",
		},
	)

	// 12. Finalize snapshot lifecycle: BUILDING -> FROZEN
	finalizeSnapshot(snapshot, normalizedObservations)

	return &BuildResult{
		UrlResources:                urlResources,
		DiscoveryRecords:            discoveryRecords,
		FetchObservations:           fetchObservations,
		RedirectHops:                redirectHops,
		HtmlObservations:            htmlObservations,
		RobotsDirectiveObservations: robotsDirectiveObservations,
		CanonicalObservations:       canonicalObservations,
		LinkObservations:            linkObservations,
		EvidenceSnapshot:            snapshot,
		EvidenceGaps:                evidenceGaps,
	}, nil
}

// finalizeSnapshot completes the BUILDING -> FROZEN lifecycle transition for an EvidenceSnapshot.
func finalizeSnapshot(s *audit.EvidenceSnapshot, normalizedObs []audit.NormalizedObservation) {
	now := time.Now().UTC()
	s.FrozenAt = &now
	s.SnapshotStatus = audit.SnapshotFrozen
	s.NormalizedObservations = normalizedObs
}

// normalizeURL performs truthful deterministic URL normalization according to Audit V1 specifications.
func normalizeURL(rawURL string) (string, bool, error) {
	if rawURL == "" {
		return "", false, errors.New("audit adapter: empty URL")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", false, fmt.Errorf("audit adapter: parse URL %q: %w", rawURL, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", false, fmt.Errorf("audit adapter: URL %q has unsupported scheme %q (only http and https supported)", rawURL, parsed.Scheme)
	}

	hostName := strings.ToLower(parsed.Hostname())
	if hostName == "" {
		return "", false, fmt.Errorf("audit adapter: URL %q has empty host", rawURL)
	}

	port := parsed.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}

	host := hostName
	if port != "" {
		host = hostName + ":" + port
	}

	fragRemoved := parsed.Fragment != "" || strings.Contains(rawURL, "#")

	path := parsed.EscapedPath()
	if path == "" {
		path = "/"
	}

	var b strings.Builder
	b.WriteString(scheme)
	b.WriteString("://")
	b.WriteString(host)
	b.WriteString(path)
	if parsed.RawQuery != "" {
		b.WriteString("?")
		b.WriteString(parsed.RawQuery)
	}

	return b.String(), fragRemoved, nil
}

// normalizeFetchError maps persisted SiteCrawl error slugs into Audit evidence vocabulary.
func normalizeFetchError(errSlug string, urlIDStr string) (string, *EvidenceGap) {
	if errSlug == "" {
		return "", nil
	}
	switch errSlug {
	case "dns-not-found":
		return "DNS_ERROR", nil
	case "timeout":
		return "TIMEOUT", nil
	case "connection-refused", "connection-error":
		return "CONNECTION_ERROR", nil
	case "ssl-error":
		return "TLS_ERROR", nil
	case "file-too-large":
		return "", &EvidenceGap{
			GapCode:         GapFetchErrorUnmappable,
			SubjectRef:      urlIDStr,
			Field:           "fetch_error_type",
			Reason:          fmt.Sprintf("Source error %q is a size limit error, not a network/transport error; mapped to unavailable in Audit evidence vocabulary.", errSlug),
			SourceComponent: "sitecrawl_pages",
		}
	default:
		return "", &EvidenceGap{
			GapCode:         GapFetchErrorUnmappable,
			SubjectRef:      urlIDStr,
			Field:           "fetch_error_type",
			Reason:          fmt.Sprintf("Source error %q cannot be truthfully mapped to Audit network error vocabulary.", errSlug),
			SourceComponent: "sitecrawl_pages",
		}
	}
}

// mapRequestProfile maps configured SiteCrawl user agent preset into Audit RequestProfile.
func mapRequestProfile(userAgentPreset string) audit.RequestProfile {
	ua := strings.ToLower(strings.TrimSpace(userAgentPreset))
	switch {
	case strings.Contains(ua, "googlebot"):
		return audit.ProfileGooglebot
	case ua == "bingbot":
		return audit.ProfileCustomBot
	case ua == "oai-searchbot" || ua == "oaisearchbot":
		return audit.ProfileOAISearchbot
	case ua == "gptbot":
		return audit.ProfileGPTBot
	case ua == "" || ua == "sitecrawl" || ua == "chrome" || ua == "chrome-mobile":
		return audit.ProfileDefault
	default:
		return audit.ProfileCustomBot
	}
}

func parseDirectiveTokens(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		token := strings.TrimSpace(strings.ToLower(p))
		if token != "" {
			out = append(out, token)
		}
	}
	return out
}

func containsToken(tokens []string, target string) bool {
	for _, t := range tokens {
		if t == target {
			return true
		}
	}
	return false
}
