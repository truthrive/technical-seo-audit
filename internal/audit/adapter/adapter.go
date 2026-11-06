package adapter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

// rawPageRecord represents a page row loaded from sitecrawl_pages.
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

	type adaptedLink struct {
		obs        audit.LinkObservation
		srcID      int
		dstID      int
		seq        int
		flags      uint32
		isInternal bool
	}

	var (
		evidenceGaps     []EvidenceGap
		linkObservations []audit.LinkObservation
		adaptedLinks     []adaptedLink
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

			// Dangling or invalid link source guard: src_id <= 0 or unallocated dictionary source
			if lr.srcID <= 0 || urlsByID[lr.srcID] == "" {
				evidenceGaps = append(evidenceGaps, EvidenceGap{
					GapCode:         GapUnresolvedLinkSourceUnavailable,
					SubjectRef:      fmt.Sprintf("link:%s:%d:%d", req.AuditRunID, lr.srcID, lr.seq),
					Field:           "source_url_id",
					Reason:          fmt.Sprintf("Link edge has source id %d which is not present in URL dictionary; source URL identity is unavailable.", lr.srcID),
					SourceComponent: "sitecrawl_links",
				})
			}

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
			adaptedLinks = append(adaptedLinks, adaptedLink{
				obs:        linkObs,
				srcID:      lr.srcID,
				dstID:      lr.dstID,
				seq:        lr.seq,
				flags:      lr.flags,
				isInternal: lr.flags&sitecrawl.FlagInternal != 0,
			})
		}
	}
	if err := linkRows.Err(); err != nil {
		return nil, fmt.Errorf("audit adapter: read links: %w", err)
	}

	// 5b. Load sitemap raw evidence within read transaction
	sitemapRaw, err := loadSitemapRawEvidence(ctx, tx, req.CrawlRunID)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: load sitemap evidence: %w", err)
	}

	// Commit read transaction after all source records are safely read
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("audit adapter: commit read tx: %w", err)
	}

	// Index sitemap entry URLs for discovery sources mapping
	sitemapEntryURLs := make(map[string]bool)
	if sitemapRaw != nil {
		for _, e := range sitemapRaw.entries {
			if norm, _, err := normalizeURL(e.loc); err == nil && norm != "" {
				sitemapEntryURLs[norm] = true
			}
		}
	}

	// 6. Initialize EvidenceSnapshot in BUILDING state
	snapshot := &audit.EvidenceSnapshot{
		SnapshotID:               req.SnapshotID,
		AuditRunID:               req.AuditRunID,
		CreatedAt:                time.Now().UTC(),
		SnapshotStatus:           audit.SnapshotBuilding,
		NormalizationVersion:     "v1.11.0",
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

	validURLResourceIDs := make(map[int]bool, len(urlsByID))
	for id, raw := range urlsByID {
		if id > 0 && raw != "" {
			validURLResourceIDs[id] = true
		}
	}

	// Build normalized URL to URLID mapping for exact target correlation
	normURLToSubjectRefs := make(map[string][]string)
	for _, ur := range urlResources {
		normURLToSubjectRefs[ur.NormalizedURL] = append(normURLToSubjectRefs[ur.NormalizedURL], string(ur.URLID))
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
		structuredDataBlocks        []audit.StructuredDataBlock
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

		// Redirect traversal completeness and final target evaluation
		isInitialRedirect := pr.status >= 300 && pr.status < 400
		redirectEvidenceRelevant := isInitialRedirect || len(pr.page.Redirects) > 0 || pr.redirectHops > 0
		redirectCountConflict := redirectEvidenceRelevant && (pr.redirectHops != len(pr.page.Redirects))
		if redirectCountConflict {
			evidenceGaps = append(evidenceGaps, EvidenceGap{
				GapCode:         GapRedirectChainInconsistent,
				SubjectRef:      string(urlIDStr),
				Field:           "redirect_hops",
				Reason:          fmt.Sprintf("Promoted redirect_hops count (%d) conflicts with preserved Page.Redirects chain length (%d); traversal completeness cannot be proven.", pr.redirectHops, len(pr.page.Redirects)),
				SourceComponent: "sitecrawl_pages",
			})
		}

		isObservedRedirect := redirectEvidenceRelevant
		hasHops := len(pr.page.Redirects) > 0
		noError := strings.TrimSpace(pr.page.Error) == ""
		traversalComplete := isObservedRedirect && opts.FollowRedirects && hasHops && noError && !redirectCountConflict

		var finalURLID *audit.URLID
		var normFinalURL string
		if traversalComplete {
			lastHop := pr.page.Redirects[len(pr.page.Redirects)-1]
			resolvedTarget := lastHop.Location
			if baseParsed, err := url.Parse(lastHop.URL); err == nil {
				if refParsed, err := url.Parse(lastHop.Location); err == nil {
					resolvedTarget = baseParsed.ResolveReference(refParsed).String()
				}
			}
			if nURL, _, err := normalizeURL(resolvedTarget); err == nil {
				normFinalURL = nURL
				if matchingSubjects := normURLToSubjectRefs[normFinalURL]; len(matchingSubjects) == 1 {
					uid := audit.URLID(matchingSubjects[0])
					finalURLID = &uid
				}
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

		// AR-LINK-005 & AR-LINK-006: is_start_url, crawl_inlink_count, discovery_sources
		isStartURL := (pr.url == seedURL || pr.page.Source == sitecrawl.SourceSeed)
		startURLVal := "false"
		if isStartURL {
			startURLVal = "true"
		}
		normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "is_start_url",
			Value:              startURLVal,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})

		inlinkCount := pr.page.Inlinks
		if inlinkCount == 0 && len(incomingAnchorEdges[urlID]) > 0 {
			inlinkCount = len(incomingAnchorEdges[urlID])
		}
		normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "crawl_inlink_count",
			Value:              strconv.Itoa(inlinkCount),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})

		normPageURL, _, _ := normalizeURL(pr.url)
		if sitemapEntryURLs[normPageURL] || pr.discoveredBy == sitecrawl.SourceSitemap || pr.page.Source == sitecrawl.SourceSitemap {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "discovery_sources",
				Value:              "sitemap",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}

		// AR-ACC-002: robots_allowed_default
		robotsAllowedVal := "unknown"
		if pr.robotsState == sitecrawl.RobotsAllowed || pr.page.RobotsState == sitecrawl.RobotsAllowed {
			robotsAllowedVal = "true"
		} else if pr.robotsState == sitecrawl.RobotsBlocked || pr.page.RobotsState == sitecrawl.RobotsBlocked {
			robotsAllowedVal = "false"
		}
		normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "robots_allowed_default",
			Value:              robotsAllowedVal,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})

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

		// 9b. Redirect Hops and Normalized Redirect Evidence
		if pr.status > 0 {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "redirect_initial_observed",
				Value:              strconv.FormatBool(isInitialRedirect),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}

		if (pr.status > 0 || len(pr.page.Redirects) > 0) && !redirectCountConflict {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "redirect_hop_count",
				Value:              strconv.Itoa(len(pr.page.Redirects)),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}

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
					LocationRaw:       "",
					ResolvedTargetURL: resolvedTarget,
					ObservedAt:        obsTime,
				})

				hopVal := redirectHopObservation{
					HopIndex:          hopIdx,
					SourceURL:         hop.URL,
					Status:            hop.Status,
					ResolvedTargetURL: resolvedTarget,
				}
				hopValBytes, _ := json.Marshal(hopVal)

				normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "redirect_hop",
					Value:              string(hopValBytes),
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				})
			}
		}

		if isObservedRedirect && !redirectCountConflict {
			var traversalVal string
			if traversalComplete {
				traversalVal = "true"
			} else if !opts.FollowRedirects || strings.TrimSpace(pr.page.Error) != "" || len(pr.page.Redirects) == 0 {
				traversalVal = "false"
			}
			if traversalVal != "" {
				normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "redirect_traversal_complete",
					Value:              traversalVal,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				})
			}
		}

		if strings.TrimSpace(pr.page.Error) == "redirect loop" {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "redirect_loop_detected",
				Value:              "true",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		} else if traversalComplete {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "redirect_loop_detected",
				Value:              "false",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}

		if traversalComplete && normFinalURL != "" {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "redirect_final_url",
				Value:              normFinalURL,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
			matchingSubjects := normURLToSubjectRefs[normFinalURL]
			if len(matchingSubjects) == 1 {
				normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "redirect_final_target_subject_ref",
					Value:              matchingSubjects[0],
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				})
			}
		}

		// Channel 1: HTTP Header X-Robots-Tag acquisition completeness
		var xRobotsRaw []string
		if len(pr.page.RawXRobotsTags) > 0 {
			xRobotsRaw = append(xRobotsRaw, pr.page.RawXRobotsTags...)
		} else if pr.page.XRobotsTag != "" {
			xRobotsRaw = append(xRobotsRaw, pr.page.XRobotsTag)
		} else if pr.xRobots != "" {
			xRobotsRaw = append(xRobotsRaw, pr.xRobots)
		}

		xRobotsSampleTruncated := false
		if pr.page.XRobotsTagTotalCount > len(pr.page.RawXRobotsTags) && len(pr.page.RawXRobotsTags) > 0 {
			xRobotsSampleTruncated = true
		}

		httpHeaderStatus := "unavailable"
		if pr.status >= 300 && pr.status < 400 || len(pr.page.Redirects) > 0 {
			httpHeaderStatus = "redirect_intermediate"
		} else if pr.status <= 0 || (pr.errorType != "" && pr.errorType != sitecrawl.ErrTooLarge) {
			httpHeaderStatus = "unavailable"
		} else if xRobotsSampleTruncated {
			httpHeaderStatus = "truncated_sample_cap"
		} else if pr.status >= 200 && pr.status < 600 {
			httpHeaderStatus = "complete"
		}

		normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "http_header_status",
			Value:              httpHeaderStatus,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})

		if httpHeaderStatus == "complete" || httpHeaderStatus == "truncated_sample_cap" || httpHeaderStatus == "redirect_intermediate" {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "x_robots_sample_truncated",
				Value:              strconv.FormatBool(xRobotsSampleTruncated),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
			if xRobotsSampleTruncated {
				evidenceGaps = append(evidenceGaps, EvidenceGap{
					GapCode:         GapXRobotsSampleCapped,
					SubjectRef:      string(urlIDStr),
					Field:           "x_robots_tag",
					Reason:          "HTTP X-Robots-Tag headers exceeded sample cap (10); subsequent headers were not retained.",
					SourceComponent: "sitecrawl_pages",
				})
			}
		}

		// 9c. HTML Observations
		isHTML := pr.kind == sitecrawl.KindHTML || strings.Contains(pr.contentType, "html")
		if isHTML {
			isRendered := (pr.rendered == 1 || pr.page.Rendered)
			htmlExtractionStatus := classifyHTMLExtractionStatus(pr, isRendered)
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "html_extraction_status",
				Value:              htmlExtractionStatus,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})

			if pr.page.RobotsMetaExtracted {
				metaTruncated := pr.page.RobotsMetaTotalCount > len(pr.page.RawRobotsMeta)
				normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "robots_meta_sample_truncated",
					Value:              strconv.FormatBool(metaTruncated),
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				})
				if metaTruncated {
					evidenceGaps = append(evidenceGaps, EvidenceGap{
						GapCode:         GapRobotsMetaSampleCapped,
						SubjectRef:      string(urlIDStr),
						Field:           "meta_robots",
						Reason:          "HTML robots meta tags exceeded sample cap (10); subsequent tags were not retained.",
						SourceComponent: "sitecrawl_pages",
					})
				}
			}

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

				var rawCanonSlice []string
				if len(pr.page.RawCanonicals) > 0 {
					rawCanonSlice = append([]string(nil), pr.page.RawCanonicals...)
				}

				var contentFingerprint string
				if titleVal != "" || metaDescVal != "" || len(h1Values) > 0 || pr.page.WordCount > 0 {
					h := sha256.New()
					h.Write([]byte(fmt.Sprintf("%s|%s|%s|%d", titleVal, metaDescVal, strings.Join(h1Values, ","), pr.page.WordCount)))
					contentFingerprint = fmt.Sprintf("%x", h.Sum(nil))
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
					CanonicalRawValues:  rawCanonSlice,
					MainTextPresent:     nil, // Not verified
					MainTextFingerprint: "",
					ContentFingerprint:  contentFingerprint,
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
				if contentFingerprint != "" {
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "content_fingerprint",
						Value:              contentFingerprint,
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

				hasRawProvenance := pr.page.ExtractionStatus != "" || pr.page.CanonicalTotalCount > 0 || len(pr.page.RawCanonicals) > 0
				totalCount := len(canonicals)
				if hasRawProvenance {
					totalCount = pr.page.CanonicalTotalCount
					if len(pr.page.RawCanonicals) > totalCount {
						totalCount = len(pr.page.RawCanonicals)
					}
					if len(canonicals) > totalCount {
						totalCount = len(canonicals)
					}
				}

				// canonical_count: count declarations preserved by SiteCrawl.
				// Duplicates and empty href declarations remain counted.
				// For trustworthy non-rendered HTML with no canonical: canonical_count = 0.
				normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "canonical_count",
					Value:              strconv.Itoa(totalCount),
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				})

				if totalCount > 0 {
					var targetIDs []audit.URLID
					for _, c := range canonicals {
						if tid, ok := idsByURL[c]; ok {
							targetIDs = append(targetIDs, audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, tid)))
						}
					}

					sampleTruncated := false
					if hasRawProvenance {
						sampleTruncated = totalCount > len(pr.page.RawCanonicals) && len(pr.page.RawCanonicals) > 0
						if totalCount > 5 && len(pr.page.RawCanonicals) <= 5 {
							sampleTruncated = true
						}
						normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
							ObservationID:      nextObsID(),
							AuditRunID:         req.AuditRunID,
							SnapshotID:         req.SnapshotID,
							SubjectType:        audit.SubjectURL,
							SubjectRef:         string(urlIDStr),
							Field:              "canonical_sample_truncated",
							Value:              strconv.FormatBool(sampleTruncated),
							DerivationType:     audit.DerivationDirect,
							SourceEvidenceRefs: []string{pageSrcRef},
							ObservedAt:         obsTime,
						})
						if sampleTruncated {
							evidenceGaps = append(evidenceGaps, EvidenceGap{
								GapCode:         GapCanonicalSampleCapped,
								SubjectRef:      string(urlIDStr),
								Field:           "canonical_raw",
								Reason:          fmt.Sprintf("Page declared %d canonical tags, exceeding the 5-item sample cap; only the first %d raw/resolved declarations were retained.", totalCount, len(pr.page.RawCanonicals)),
								SourceComponent: "sitecrawl_pages",
							})
						}
					}

					for _, rawC := range pr.page.RawCanonicals {
						normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
							ObservationID:      nextObsID(),
							AuditRunID:         req.AuditRunID,
							SnapshotID:         req.SnapshotID,
							SubjectType:        audit.SubjectURL,
							SubjectRef:         string(urlIDStr),
							Field:              "canonical_raw",
							Value:              rawC,
							DerivationType:     audit.DerivationDirect,
							SourceEvidenceRefs: []string{pageSrcRef},
							ObservedAt:         obsTime,
						})
					}

					// Keep existing canonical_resolved semantics unchanged (non-empty resolved strings only)
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

					var (
						allNormalized    = true
						normTargets      []string
						seenDistinctNorm = make(map[string]struct{})
						distinctTargets  []string
						parseErrors      []string
					)

					if totalCount == 1 && hasRawProvenance && len(pr.page.RawCanonicals) == 1 {
						syntaxStatus, parseErr, normTarget := classifySingleCanonical(
							pr.page.RawCanonicals[0],
							pr.page.BaseHrefRaw,
							pr.page.BaseHrefCount,
							pr.url,
						)
						normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
							ObservationID:      nextObsID(),
							AuditRunID:         req.AuditRunID,
							SnapshotID:         req.SnapshotID,
							SubjectType:        audit.SubjectURL,
							SubjectRef:         string(urlIDStr),
							Field:              "canonical_syntax_status",
							Value:              syntaxStatus,
							DerivationType:     audit.DerivationNormalized,
							SourceEvidenceRefs: []string{pageSrcRef},
							ObservedAt:         obsTime,
						})
						if parseErr != "" {
							parseErrors = append(parseErrors, parseErr)
							normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
								ObservationID:      nextObsID(),
								AuditRunID:         req.AuditRunID,
								SnapshotID:         req.SnapshotID,
								SubjectType:        audit.SubjectURL,
								SubjectRef:         string(urlIDStr),
								Field:              "canonical_parse_error",
								Value:              parseErr,
								DerivationType:     audit.DerivationNormalized,
								SourceEvidenceRefs: []string{pageSrcRef},
								ObservedAt:         obsTime,
							})
						}
						if syntaxStatus == "valid" && normTarget != "" {
							normTargets = append(normTargets, normTarget)
							distinctTargets = append(distinctTargets, normTarget)
							normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
								ObservationID:      nextObsID(),
								AuditRunID:         req.AuditRunID,
								SnapshotID:         req.SnapshotID,
								SubjectType:        audit.SubjectURL,
								SubjectRef:         string(urlIDStr),
								Field:              "canonical_normalized_target",
								Value:              normTarget,
								DerivationType:     audit.DerivationNormalized,
								SourceEvidenceRefs: []string{pageSrcRef},
								ObservedAt:         obsTime,
							})
						} else {
							allNormalized = false
						}
					} else if totalCount == 1 && !hasRawProvenance {
						// Legacy single-canonical row without RawCanonicals provenance
						for _, c := range canonicals {
							normTarget, err := normalizeCanonicalTarget(c, pr.url)
							if err != nil {
								allNormalized = false
								continue
							}
							normTargets = append(normTargets, normTarget)
							normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
								ObservationID:      nextObsID(),
								AuditRunID:         req.AuditRunID,
								SnapshotID:         req.SnapshotID,
								SubjectType:        audit.SubjectURL,
								SubjectRef:         string(urlIDStr),
								Field:              "canonical_normalized_target",
								Value:              normTarget,
								DerivationType:     audit.DerivationNormalized,
								SourceEvidenceRefs: []string{pageSrcRef},
								ObservedAt:         obsTime,
							})
							if _, exists := seenDistinctNorm[normTarget]; !exists {
								seenDistinctNorm[normTarget] = struct{}{}
								distinctTargets = append(distinctTargets, normTarget)
							}
						}
						if !allNormalized || len(normTargets) == 0 {
							parseErr := "insufficient_evidence:unclassified_normalization_failure"
							parseErrors = append(parseErrors, parseErr)
							normalizedObservations = append(normalizedObservations,
								audit.NormalizedObservation{
									ObservationID:      nextObsID(),
									AuditRunID:         req.AuditRunID,
									SnapshotID:         req.SnapshotID,
									SubjectType:        audit.SubjectURL,
									SubjectRef:         string(urlIDStr),
									Field:              "canonical_syntax_status",
									Value:              "insufficient_evidence",
									DerivationType:     audit.DerivationNormalized,
									SourceEvidenceRefs: []string{pageSrcRef},
									ObservedAt:         obsTime,
								},
								audit.NormalizedObservation{
									ObservationID:      nextObsID(),
									AuditRunID:         req.AuditRunID,
									SnapshotID:         req.SnapshotID,
									SubjectType:        audit.SubjectURL,
									SubjectRef:         string(urlIDStr),
									Field:              "canonical_parse_error",
									Value:              parseErr,
									DerivationType:     audit.DerivationNormalized,
									SourceEvidenceRefs: []string{pageSrcRef},
									ObservedAt:         obsTime,
								},
							)
						}
					} else {
						// Multi-canonical (totalCount > 1) or synthetic raw provenance without single RawCanonicals
						if hasRawProvenance && len(pr.page.RawCanonicals) > 0 {
							for _, rawC := range pr.page.RawCanonicals {
								status, parseErr, normTarget := classifySingleCanonical(
									rawC,
									pr.page.BaseHrefRaw,
									pr.page.BaseHrefCount,
									pr.url,
								)
								if parseErr != "" {
									parseErrors = append(parseErrors, parseErr)
								}
								if status != "valid" || normTarget == "" {
									allNormalized = false
									continue
								}
								normTargets = append(normTargets, normTarget)
								normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
									ObservationID:      nextObsID(),
									AuditRunID:         req.AuditRunID,
									SnapshotID:         req.SnapshotID,
									SubjectType:        audit.SubjectURL,
									SubjectRef:         string(urlIDStr),
									Field:              "canonical_normalized_target",
									Value:              normTarget,
									DerivationType:     audit.DerivationNormalized,
									SourceEvidenceRefs: []string{pageSrcRef},
									ObservedAt:         obsTime,
								})
								if _, exists := seenDistinctNorm[normTarget]; !exists {
									seenDistinctNorm[normTarget] = struct{}{}
									distinctTargets = append(distinctTargets, normTarget)
								}
							}
						} else {
							for _, c := range canonicals {
								normTarget, err := normalizeCanonicalTarget(c, pr.url)
								if err != nil {
									allNormalized = false
									continue
								}
								normTargets = append(normTargets, normTarget)
								normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
									ObservationID:      nextObsID(),
									AuditRunID:         req.AuditRunID,
									SnapshotID:         req.SnapshotID,
									SubjectType:        audit.SubjectURL,
									SubjectRef:         string(urlIDStr),
									Field:              "canonical_normalized_target",
									Value:              normTarget,
									DerivationType:     audit.DerivationNormalized,
									SourceEvidenceRefs: []string{pageSrcRef},
									ObservedAt:         obsTime,
								})
								if _, exists := seenDistinctNorm[normTarget]; !exists {
									seenDistinctNorm[normTarget] = struct{}{}
									distinctTargets = append(distinctTargets, normTarget)
								}
							}
						}
					}

					canonicalObservations = append(canonicalObservations, audit.CanonicalObservation{
						CanonicalObservationID: audit.CanonicalObservationID(fmt.Sprintf("canon:%s:%d", req.AuditRunID, urlID)),
						AuditRunID:             req.AuditRunID,
						URLID:                  urlIDStr,
						CanonicalCount:         totalCount,
						RawValues:              rawCanonSlice,
						NormalizedValues:       canonicals,
						ResolvedTargetURLIDs:   targetIDs,
						ParseErrors:            parseErrors,
						ObservedAt:             obsTime,
					})

					// canonical_normalization_complete:
					// When canonical_count > 0, emit true only if every declaration (including any
					// unretained declaration beyond the 5-item sample cap) can be normalized to a
					// valid HTTP(S) target. Otherwise emit false.
					completeVal := "false"
					if !sampleTruncated && allNormalized && len(normTargets) == totalCount {
						completeVal = "true"
					}
					normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
						ObservationID:      nextObsID(),
						AuditRunID:         req.AuditRunID,
						SnapshotID:         req.SnapshotID,
						SubjectType:        audit.SubjectURL,
						SubjectRef:         string(urlIDStr),
						Field:              "canonical_normalization_complete",
						Value:              completeVal,
						DerivationType:     audit.DerivationNormalized,
						SourceEvidenceRefs: []string{pageSrcRef},
						ObservedAt:         obsTime,
					})

					// canonical_distinct_normalized_count:
					// Emit only when canonical_normalization_complete = true.
					if completeVal == "true" {
						normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
							ObservationID:      nextObsID(),
							AuditRunID:         req.AuditRunID,
							SnapshotID:         req.SnapshotID,
							SubjectType:        audit.SubjectURL,
							SubjectRef:         string(urlIDStr),
							Field:              "canonical_distinct_normalized_count",
							Value:              strconv.Itoa(len(distinctTargets)),
							DerivationType:     audit.DerivationNormalized,
							SourceEvidenceRefs: []string{pageSrcRef},
							ObservedAt:         obsTime,
						})

						// Canonical target correlation:
						// When canonical evidence resolves to exactly one distinct valid normalized target:
						// try to map it to an existing URL subject.
						if len(distinctTargets) == 1 {
							targetNorm := distinctTargets[0]
							matchingSubjects := normURLToSubjectRefs[targetNorm]
							if len(matchingSubjects) == 1 {
								normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
									ObservationID:      nextObsID(),
									AuditRunID:         req.AuditRunID,
									SnapshotID:         req.SnapshotID,
									SubjectType:        audit.SubjectURL,
									SubjectRef:         string(urlIDStr),
									Field:              "canonical_target_subject_ref",
									Value:              matchingSubjects[0],
									DerivationType:     audit.DerivationNormalized,
									SourceEvidenceRefs: []string{pageSrcRef},
									ObservedAt:         obsTime,
								})
							}
						}
					}
				}
			}
		}

		// 9g. Structured data blocks and normalized observations
		sdBlocks, sdObs, sdGaps := buildStructuredDataEvidence(
			req, pr.page, urlID, urlIDStr, pr.url, obsTime, pageSrcRef, nextObsID,
		)
		structuredDataBlocks = append(structuredDataBlocks, sdBlocks...)
		normalizedObservations = append(normalizedObservations, sdObs...)
		evidenceGaps = append(evidenceGaps, sdGaps...)

		// 9h. Robots directive observations for all pages (both HTML and non-HTML)
		isRendered := (pr.rendered == 1 || pr.page.Rendered)
		isDiscreteHeaders := len(pr.page.RawXRobotsTags) > 0
		pageDirectives, dirNorms, dirGaps := processPageDirectives(
			req,
			urlID,
			urlIDStr,
			pageSrcRef,
			obsTime,
			isHTML,
			isRendered,
			pr.page.RobotsMetaExtracted,
			pr.page.RawRobotsMeta,
			pr.page.MetaTags,
			pr.page.MetaRobots,
			pr.metaRobots,
			xRobotsRaw,
			isDiscreteHeaders,
			nextObsID,
		)
		robotsDirectiveObservations = append(robotsDirectiveObservations, pageDirectives...)
		normalizedObservations = append(normalizedObservations, dirNorms...)
		evidenceGaps = append(evidenceGaps, dirGaps...)
	}

	// 10. Add normalized observations for links
	for _, al := range adaptedLinks {
		linkSrcRef := fmt.Sprintf("sitecrawl_links:%d:%d", al.srcID, al.seq)

		// 10a. Source identity and correlation
		sourceURL, srcOK := urlsByID[al.srcID]
		srcValid := srcOK && sourceURL != "" && al.srcID > 0 && validURLResourceIDs[al.srcID]

		if srcValid {
			normalizedObservations = append(normalizedObservations,
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         string(al.obs.LinkID),
					Field:              "link_source_url",
					Value:              sourceURL,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{linkSrcRef},
					ObservedAt:         al.obs.ObservedAt,
				},
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         string(al.obs.LinkID),
					Field:              "link_source_subject_ref",
					Value:              fmt.Sprintf("url:%s:%d", req.AuditRunID, al.srcID),
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{linkSrcRef},
					ObservedAt:         al.obs.ObservedAt,
				},
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         string(al.obs.LinkID),
					Field:              "link_is_internal",
					Value:              strconv.FormatBool(al.isInternal),
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{linkSrcRef},
					ObservedAt:         al.obs.ObservedAt,
				},
			)
		}

		// 10b. Resolved target URL
		if al.obs.TargetURLResolved != "" {
			normalizedObservations = append(normalizedObservations,
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectLink,
					SubjectRef:         string(al.obs.LinkID),
					Field:              "link_target",
					Value:              al.obs.TargetURLResolved,
					DerivationType:     audit.DerivationDirect,
					SourceEvidenceRefs: []string{linkSrcRef},
					ObservedAt:         al.obs.ObservedAt,
				},
			)
		}

		// 10c. Target subject correlation (link_target_subject_ref)
		if al.dstID > 0 {
			dstURL, dstOK := urlsByID[al.dstID]
			if dstOK && dstURL != "" && validURLResourceIDs[al.dstID] {
				if al.obs.TargetURLResolved == "" || al.obs.TargetURLResolved == dstURL {
					normalizedObservations = append(normalizedObservations,
						audit.NormalizedObservation{
							ObservationID:      nextObsID(),
							AuditRunID:         req.AuditRunID,
							SnapshotID:         req.SnapshotID,
							SubjectType:        audit.SubjectLink,
							SubjectRef:         string(al.obs.LinkID),
							Field:              "link_target_subject_ref",
							Value:              fmt.Sprintf("url:%s:%d", req.AuditRunID, al.dstID),
							DerivationType:     audit.DerivationNormalized,
							SourceEvidenceRefs: []string{linkSrcRef},
							ObservedAt:         al.obs.ObservedAt,
						},
					)
				}
			}
		}

		// 10d. Anchor text and location
		normalizedObservations = append(normalizedObservations,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         string(al.obs.LinkID),
				Field:              "link_anchor",
				Value:              al.obs.AnchorText,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{linkSrcRef},
				ObservedAt:         al.obs.ObservedAt,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectLink,
				SubjectRef:         string(al.obs.LinkID),
				Field:              "link_location",
				Value:              al.obs.LinkLocation,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{linkSrcRef},
				ObservedAt:         al.obs.ObservedAt,
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

	// AR-ACC-001: site_https_url and HTTPS reachability observations on SubjectSite
	if seedParsed, err := url.Parse(seedURL); err == nil && seedParsed != nil && seedParsed.Host != "" {
		siteHTTPSURL := "https://" + seedParsed.Host + "/"
		siteHTTPSNoSlash := "https://" + seedParsed.Host
		var (
			httpsPage      *rawPageRecord
			httpsAttempted bool
			httpsStatus    int
			httpsErrorType string
			httpsTLSValid  *bool
			pageSrcRef     string
		)
		if seedParsed.Scheme == "https" {
			if id, ok := idsByURL[seedURL]; ok {
				httpsPage = pagesByURLID[id]
			}
		}
		if httpsPage == nil {
			if id, ok := idsByURL[siteHTTPSURL]; ok {
				httpsPage = pagesByURLID[id]
			}
		}
		if httpsPage == nil {
			if id, ok := idsByURL[siteHTTPSNoSlash]; ok {
				httpsPage = pagesByURLID[id]
			}
		}

		srcRefs := []string{siteSrcRef}
		obsTime := runStartedAt
		if httpsPage != nil {
			httpsAttempted = true
			httpsStatus = httpsPage.status
			httpsErrorType = httpsPage.errorType
			siteHTTPSURL = httpsPage.url
			pageSrcRef = fmt.Sprintf("sitecrawl_pages:%s:%d", req.CrawlRunID, httpsPage.urlID)
			srcRefs = []string{pageSrcRef, siteSrcRef}
			if pt, err := time.Parse(time.RFC3339, httpsPage.crawledAt); err == nil {
				obsTime = pt
			}
			if httpsPage.status > 0 {
				v := true
				httpsTLSValid = &v
			} else if httpsPage.errorType == sitecrawl.ErrSSL || strings.Contains(httpsPage.errorType, "ssl") || strings.Contains(httpsPage.errorType, "tls") || strings.Contains(httpsPage.errorType, "cert") {
				v := false
				httpsTLSValid = &v
			}
		} else if seedParsed.Scheme == "https" {
			httpsAttempted = false
			httpsStatus = 0
		}

		normalizedObservations = append(normalizedObservations,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "site_https_url",
				Value:              siteHTTPSURL,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: srcRefs,
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "fetch_attempted",
				Value:              strconv.FormatBool(httpsAttempted),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: srcRefs,
				ObservedAt:         obsTime,
			},
		)
		if httpsAttempted || httpsStatus > 0 {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "fetch_status",
				Value:              strconv.Itoa(httpsStatus),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: srcRefs,
				ObservedAt:         obsTime,
			})
		}
		if httpsTLSValid != nil {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "tls_valid",
				Value:              strconv.FormatBool(*httpsTLSValid),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: srcRefs,
				ObservedAt:         obsTime,
			})
		}
		if httpsErrorType != "" {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "fetch_error_type",
				Value:              httpsErrorType,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: srcRefs,
				ObservedAt:         obsTime,
			})
		}
	}

	// 10c. Normalize sitemap evidence
	smObs, smEntries, smNormObs, smGaps, sitemapDiscoveryComplete := buildSitemapEvidence(
		req, sitemapRaw, urlsByID, urlIDs, pagesByURLID, runStartedAt, nextObsID,
	)
	snapshot.SitemapDiscoveryComplete = sitemapDiscoveryComplete
	normalizedObservations = append(normalizedObservations, smNormObs...)
	evidenceGaps = append(evidenceGaps, smGaps...)

	// 11. Append conditional and global capability-level evidence gaps
	hasLegacyMissingRawCanon := false
	for _, co := range canonicalObservations {
		if len(co.RawValues) == 0 {
			hasLegacyMissingRawCanon = true
			break
		}
	}
	if hasLegacyMissingRawCanon {
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
			GapCode:         GapRDFaAcquisitionUnavailable,
			Field:           "rdfa_observation",
			Reason:          "SiteCrawl does not acquire RDFa structured data attributes (typeof, property, vocab, resource).",
			SourceComponent: "sitecrawl",
		},
		EvidenceGap{
			GapCode:         GapMicrodataRawMarkupUnavailable,
			Field:           "microdata_raw_markup",
			Reason:          "SiteCrawl stores simplified SchemaOrg structures but does not preserve original raw Microdata HTML markup or attribute tokens.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapStructuredDataAbsenceUnprovable,
			Field:           "structured_data_absence",
			Reason:          "The frozen SiteCrawl acquisition pipeline does not provide complete structured-data format coverage, including RDFa. Therefore, absence of observed JSON-LD/Microdata cannot be treated as proof that a page contains no structured data.",
			SourceComponent: "sitecrawl",
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
		StructuredDataBlocks:        structuredDataBlocks,
		SitemapObservations:        smObs,
		SitemapEntries:             smEntries,
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

// redirectHopObservation represents the deterministic machine-readable payload
// for a normalized "redirect_hop" observation.
type redirectHopObservation struct {
	HopIndex          int    `json:"hop_index"`
	SourceURL         string `json:"source_url"`
	Status            int    `json:"status"`
	ResolvedTargetURL string `json:"resolved_target_url"`
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

// normalizeCanonicalTarget resolves raw canonical declaration against baseURL if relative,
// and normalizes it to a valid absolute HTTP(S) URL according to Audit V1 specifications.
func normalizeCanonicalTarget(rawCanon string, baseURL string) (string, error) {
	c := strings.TrimSpace(rawCanon)
	if c == "" {
		return "", errors.New("audit adapter: empty canonical declaration")
	}

	parsed, err := url.Parse(c)
	if err != nil {
		return "", fmt.Errorf("audit adapter: parse canonical %q: %w", c, err)
	}

	target := c
	if !parsed.IsAbs() {
		if baseURL == "" {
			return "", fmt.Errorf("audit adapter: cannot resolve relative canonical %q with empty base URL", c)
		}
		baseParsed, err := url.Parse(baseURL)
		if err != nil || !baseParsed.IsAbs() {
			return "", fmt.Errorf("audit adapter: invalid base URL %q for relative canonical %q", baseURL, c)
		}
		target = baseParsed.ResolveReference(parsed).String()
	}

	normURL, _, err := normalizeURL(target)
	if err != nil {
		return "", fmt.Errorf("audit adapter: normalize canonical URL %q: %w", target, err)
	}
	return normURL, nil
}

// classifyHTMLExtractionStatus maps persisted SiteCrawl page transport/extraction state
// to the four-value html_extraction_status vocabulary ("complete", "truncated_size_limit",
// "read_error", "missing_legacy").
func classifyHTMLExtractionStatus(pr *rawPageRecord, isRendered bool) string {
	if isRendered || len(pr.page.Redirects) > 0 || pr.redirectHops > 0 {
		return "missing_legacy"
	}
	if pr.errorType == sitecrawl.ErrTooLarge || pr.page.ExtractionStatus == sitecrawl.ExtractionStatusTruncatedSizeLimit {
		return "truncated_size_limit"
	}
	if pr.errorType != "" || pr.page.ExtractionStatus == sitecrawl.ExtractionStatusReadError {
		return "read_error"
	}
	if pr.status < 200 || pr.status >= 300 {
		return "missing_legacy"
	}
	if pr.page.ExtractionStatus == sitecrawl.ExtractionStatusComplete {
		return "complete"
	}
	return "missing_legacy"
}

// hasMalformedHostOrPort checks whether a parsed URL with an authority component has an
// unparseable or non-numeric port (e.g. "https://example.com:badport") or invalid characters.
func hasMalformedHostOrPort(u *url.URL) bool {
	if u == nil {
		return true
	}
	if strings.Contains(u.Host, ":") {
		h := u.Host
		// Check IPv6 bracket balance
		if strings.HasPrefix(h, "[") {
			closeIdx := strings.Index(h, "]")
			if closeIdx < 0 {
				return true
			}
			rest := h[closeIdx+1:]
			if strings.HasPrefix(rest, ":") {
				portStr := rest[1:]
				if portStr == "" {
					return false
				}
				if p, err := strconv.Atoi(portStr); err != nil || p <= 0 || p > 65535 {
					return true
				}
			} else if rest != "" {
				return true
			}
			return false
		}
		// Non-IPv6 host with colon
		colonIdx := strings.LastIndex(h, ":")
		portStr := h[colonIdx+1:]
		if portStr != "" {
			if p, err := strconv.Atoi(portStr); err != nil || p <= 0 || p > 65535 {
				return true
			}
		}
	}
	return false
}

// isNonHierarchicalScheme reports whether scheme is an opaque/non-hierarchical URI scheme
// that cannot serve as a hierarchical base for relative reference resolution.
func isNonHierarchicalScheme(scheme string) bool {
	switch strings.ToLower(scheme) {
	case "mailto", "javascript", "data", "tel", "urn", "about", "blob":
		return true
	default:
		return false
	}
}

// classifySingleCanonical evaluates a single raw canonical declaration against its
// document <base href> and page URL context under the approved V1.9b contract
// (DEC-V1.9A-001 & Scenario 6 sub-cases).
func classifySingleCanonical(rawHref string, baseHrefRaw string, baseHrefCount int, pageURL string) (syntaxStatus string, parseError string, normTarget string) {
	trimmed := strings.TrimSpace(rawHref)
	if trimmed == "" {
		return "parser_error", "parser_error:empty_href", ""
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return "parser_error", "parser_error:malformed_url", ""
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || hasMalformedHostOrPort(parsed) {
		return "parser_error", "parser_error:malformed_url", ""
	}

	// Case 1: Absolute URI with an explicit scheme (Scenario 2, 3, 6b-i, 7, 8)
	if parsed.IsAbs() {
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "http" && scheme != "https" {
			return "resolution_error", "resolution_error:unsupported_scheme", ""
		}
		if parsed.Hostname() == "" {
			return "resolution_error", "resolution_error:missing_host", ""
		}
		norm, _, err := normalizeURL(trimmed)
		if err != nil {
			return "parser_error", "parser_error:malformed_url", ""
		}
		return "valid", "", norm
	}

	// Case 2: Scheme-relative network-path reference (e.g. "//example.com/path" or "///path")
	if strings.HasPrefix(trimmed, "//") {
		if parsed.Hostname() == "" {
			return "resolution_error", "resolution_error:missing_host", ""
		}
	}

	// Case 3: Relative canonical reference (!parsed.IsAbs()) — requires base resolution
	if strings.TrimSpace(pageURL) == "" {
		return "insufficient_evidence", "insufficient_evidence:missing_base_context", ""
	}
	pageBaseParsed, err := url.Parse(pageURL)
	if err != nil || !pageBaseParsed.IsAbs() {
		return "insufficient_evidence", "insufficient_evidence:missing_base_context", ""
	}
	if _, _, err := normalizeURL(pageURL); err != nil {
		return "insufficient_evidence", "insufficient_evidence:missing_base_context", ""
	}

	// Scenario 6c: Multiple <base href> declarations with potentially conflicting bases
	if baseHrefCount > 1 {
		return "insufficient_evidence", "insufficient_evidence:missing_base_context", ""
	}

	baseTrimmed := strings.TrimSpace(baseHrefRaw)
	// Scenario 4 & Scenario 6b-ii: No <base href>, empty <base href="">, or fragment-only <base href="#frag">
	// deterministically falls back to the document's FinalURL (pageBaseParsed).
	if baseHrefCount == 0 || baseTrimmed == "" || strings.HasPrefix(baseTrimmed, "#") {
		resolved := pageBaseParsed.ResolveReference(parsed).String()
		norm, _, err := normalizeURL(resolved)
		if err != nil {
			return "resolution_error", "resolution_error:missing_host", ""
		}
		return "valid", "", norm
	}

	// Scenario 5, 6a-i, 6a-ii: Single non-empty, non-fragment <base href>
	if strings.ContainsAny(baseTrimmed, " \t\r\n") {
		return "insufficient_evidence", "insufficient_evidence:ambiguous_base_fallback", ""
	}
	baseParsed, err := url.Parse(baseTrimmed)
	if err != nil || hasMalformedHostOrPort(baseParsed) {
		return "insufficient_evidence", "insufficient_evidence:ambiguous_base_fallback", ""
	}

	var effectiveBase *url.URL
	if baseParsed.IsAbs() {
		baseScheme := strings.ToLower(baseParsed.Scheme)
		if baseScheme != "http" && baseScheme != "https" {
			// Scenario 6a-i: Valid non-HTTP(S) <base href>
			if baseParsed.Opaque != "" || isNonHierarchicalScheme(baseScheme) {
				return "resolution_error", "resolution_error:non_hierarchical_base", ""
			}
			return "resolution_error", "resolution_error:unsupported_scheme", ""
		}
		if baseParsed.Hostname() == "" {
			// Scenario 6a-ii: <base href="http:///dir/"> has empty host (malformed HTTP(S) base)
			return "insufficient_evidence", "insufficient_evidence:ambiguous_base_fallback", ""
		}
		if _, _, err := normalizeURL(baseParsed.String()); err != nil {
			return "insufficient_evidence", "insufficient_evidence:ambiguous_base_fallback", ""
		}
		effectiveBase = baseParsed
	} else {
		// Relative <base href> (e.g. "/dir/" or "//cdn.example.com/dir/")
		if strings.HasPrefix(baseTrimmed, "//") && baseParsed.Hostname() == "" {
			return "insufficient_evidence", "insufficient_evidence:ambiguous_base_fallback", ""
		}
		effectiveBase = pageBaseParsed.ResolveReference(baseParsed)
		if _, _, err := normalizeURL(effectiveBase.String()); err != nil {
			return "insufficient_evidence", "insufficient_evidence:ambiguous_base_fallback", ""
		}
	}

	resolved := effectiveBase.ResolveReference(parsed).String()
	norm, _, err := normalizeURL(resolved)
	if err != nil {
		return "resolution_error", "resolution_error:missing_host", ""
	}
	return "valid", "", norm
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

var (
	reValidIdentifier     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	reNearMissParam       = regexp.MustCompile(`^(?i)(max[-_]|unavailable[-_]?|no[-_])`)
	reUnavailableAfterDay = regexp.MustCompile(`(?i)^unavailable_after\s*:\s*(mon|tue|wed|thu|fri|sat|sun|monday|tuesday|wednesday|thursday|friday|saturday|sunday)$`)
)

var unavailableAfterLayouts = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05Z07:00",
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05",
	"2006-01-02",
	time.RFC1123,
	time.RFC1123Z,
	time.RFC822,
	time.RFC822Z,
	"02 Jan 2006 15:04:05 MST",
	"2 Jan 2006 15:04:05 MST",
	"02 Jan 2006 15:04:05 -0700",
	"2 Jan 2006 15:04:05 -0700",
	"02-Jan-2006 15:04:05 MST",
	"2-Jan-2006 15:04:05 MST",
	"02-Jan-2006",
	"02 Jan 2006",
	time.RFC850,
	"Monday, 02-Jan-2006 15:04:05 MST",
	"Monday, 2-Jan-2006 15:04:05 MST",
}

func isValidUnavailableAfterDate(val string) bool {
	v := strings.TrimSpace(val)
	if v == "" {
		return false
	}
	for _, layout := range unavailableAfterLayouts {
		if _, err := time.Parse(layout, v); err == nil {
			return true
		}
	}
	return false
}

func parseDirectiveTokens(raw string) []string {
	parts := splitDirectiveItems(raw)
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

func isParameterizedDirective(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "max-snippet", "max-image-preview", "max-video-preview", "unavailable_after":
		return true
	default:
		return false
	}
}

func isStandaloneSupportedDirective(tok string) bool {
	switch strings.ToLower(strings.TrimSpace(tok)) {
	case "all", "none", "index", "noindex", "follow", "nofollow",
		"nosnippet", "notranslate", "noimageindex", "indexifembedded",
		"noarchive":
		return true
	default:
		return false
	}
}

func isKnownUnsupportedDirective(tok string) bool {
	switch strings.ToLower(strings.TrimSpace(tok)) {
	case "noodp", "noydir", "nocache", "nopagereadaloud", "nositelinkssearchbox",
		"nopreview", "noav", "no-index", "no_index", "nofollows", "dofollow",
		"archive", "snippet":
		return true
	default:
		return false
	}
}

// isSupportedAgentToken returns the normalized agent name and true if the token is an
// explicit agent prefix recognized by the current frozen V1 vocabulary.
func isSupportedAgentToken(token string) (string, bool) {
	t := strings.ToLower(strings.TrimSpace(token))
	switch t {
	case "googlebot", "googlebot-image", "googlebot-news", "googlebot-video":
		return t, true
	case "oai-searchbot", "oaisearchbot":
		return "oai-searchbot", true
	case "gptbot":
		return "gptbot", true
	case "bingbot":
		return "bingbot", true
	default:
		if strings.HasPrefix(t, "googlebot-") || strings.HasPrefix(t, "googlebot_") {
			return t, true
		}
		return "", false
	}
}

func isRecognizedCrawlerAgent(name string) bool {
	t := strings.ToLower(strings.TrimSpace(name))
	switch t {
	case "robots", "googlebot", "googlebot-news", "googlebot-image", "googlebot-video",
		"adsbot-google", "adsbot-google-mobile", "mediapartners-google", "apis-google",
		"google-extended", "bingbot", "adidxbot", "msnbot", "oai-searchbot", "oaisearchbot",
		"gptbot", "chatgpt-user", "claudebot", "anthropic-ai", "perplexitybot",
		"applebot", "applebot-extended", "slurp", "duckduckbot", "baiduspider",
		"yandex", "facebookexternalhit", "meta-externalagent", "twitterbot",
		"linkedinbot", "bytespider", "ccbot":
		return true
	default:
		if strings.HasPrefix(t, "googlebot-") || strings.HasPrefix(t, "googlebot_") {
			return true
		}
		return false
	}
}

func splitDirectiveItems(raw string) []string {
	parts := strings.Split(raw, ",")
	var out []string
	for i := 0; i < len(parts); i++ {
		p := strings.TrimSpace(parts[i])
		if reUnavailableAfterDay.MatchString(p) && i+1 < len(parts) {
			merged := p + ", " + strings.TrimSpace(parts[i+1])
			out = append(out, merged)
			i++
		} else {
			out = append(out, p)
		}
	}
	return out
}

type itemValidationResult struct {
	token             string
	unsupportedTokens []string
	parseErrors       []string
}

func validateDirectiveItem(rawItem string, isHeader bool) itemValidationResult {
	item := strings.TrimSpace(rawItem)
	if item == "" {
		return itemValidationResult{
			parseErrors: []string{"parse_error:empty_token_segment"},
		}
	}

	if strings.Contains(item, ";") || strings.Contains(item, "|") {
		return itemValidationResult{
			parseErrors: []string{"parse_error:invalid_delimiter:" + item},
		}
	}

	colonIdx := strings.Index(item, ":")
	if colonIdx == -1 {
		if strings.Contains(item, " ") {
			return itemValidationResult{
				parseErrors: []string{"parse_error:invalid_delimiter:" + item},
			}
		}

		itemLower := strings.ToLower(item)
		if isStandaloneSupportedDirective(itemLower) {
			return itemValidationResult{
				token: itemLower,
			}
		}
		if isKnownUnsupportedDirective(itemLower) {
			return itemValidationResult{
				token:             itemLower,
				unsupportedTokens: []string{itemLower},
				parseErrors:       []string{"unsupported_directive:" + itemLower},
			}
		}
		if reValidIdentifier.MatchString(item) {
			return itemValidationResult{
				token:             itemLower,
				unsupportedTokens: []string{itemLower},
				parseErrors:       []string{"unsupported_directive:" + itemLower},
			}
		}

		return itemValidationResult{
			parseErrors: []string{"parse_error:invalid_syntax:" + item},
		}
	}

	lhs := strings.TrimSpace(item[:colonIdx])
	lhsLower := strings.ToLower(lhs)
	rhs := strings.TrimSpace(item[colonIdx+1:])
	rhsLower := strings.ToLower(rhs)

	if !isHeader && isRecognizedCrawlerAgent(lhsLower) && lhsLower != "robots" {
		return itemValidationResult{
			parseErrors: []string{"parse_error:meta_agent_prefix:" + item},
		}
	}

	if isStandaloneSupportedDirective(lhsLower) {
		return itemValidationResult{
			parseErrors: []string{"parse_error:unexpected_parameter:" + item},
		}
	}

	switch lhsLower {
	case "max-snippet":
		if rhs == "" {
			return itemValidationResult{
				parseErrors: []string{"parse_error:missing_parameter:max-snippet"},
			}
		}
		n, err := strconv.Atoi(rhs)
		if err != nil || n < -1 {
			return itemValidationResult{
				parseErrors: []string{"parse_error:invalid_parameter_value:max-snippet:" + rhs},
			}
		}
		return itemValidationResult{
			token: "max-snippet:" + rhs,
		}
	case "max-image-preview":
		if rhs == "" {
			return itemValidationResult{
				parseErrors: []string{"parse_error:missing_parameter:max-image-preview"},
			}
		}
		if rhsLower == "none" || rhsLower == "standard" || rhsLower == "large" {
			return itemValidationResult{
				token: "max-image-preview:" + rhsLower,
			}
		}
		return itemValidationResult{
			parseErrors: []string{"parse_error:invalid_parameter_value:max-image-preview:" + rhs},
		}
	case "max-video-preview":
		if rhs == "" {
			return itemValidationResult{
				parseErrors: []string{"parse_error:missing_parameter:max-video-preview"},
			}
		}
		n, err := strconv.Atoi(rhs)
		if err != nil || n < -1 {
			return itemValidationResult{
				parseErrors: []string{"parse_error:invalid_parameter_value:max-video-preview:" + rhs},
			}
		}
		return itemValidationResult{
			token: "max-video-preview:" + rhs,
		}
	case "unavailable_after":
		if rhs == "" {
			return itemValidationResult{
				parseErrors: []string{"parse_error:missing_parameter:unavailable_after"},
			}
		}
		if isValidUnavailableAfterDate(rhs) {
			return itemValidationResult{
				token: "unavailable_after:" + rhs,
			}
		}
		return itemValidationResult{
			parseErrors: []string{"parse_error:invalid_parameter_value:unavailable_after:" + rhs},
		}
	}

	if reNearMissParam.MatchString(lhsLower) {
		return itemValidationResult{
			token:             item,
			unsupportedTokens: []string{item},
			parseErrors:       []string{"unsupported_directive:" + item},
		}
	}

	return itemValidationResult{
		parseErrors: []string{"parse_error:unexpected_parameter:" + item},
	}
}

type parsedHeaderSegment struct {
	target            string
	scopeUnknown      bool
	rawValue          string
	tokens            []string
	unsupportedTokens []string
	parseErrors       []string
}

func parseXRobotsSegment(seg string, prevHadPrefix bool) (parsedHeaderSegment, bool) {
	colonIdx := strings.Index(seg, ":")
	if colonIdx == -1 {
		tokens := parseDirectiveTokens(seg)
		var pErrs []string
		var uToks []string
		for _, tok := range tokens {
			res := validateDirectiveItem(tok, true)
			uToks = append(uToks, res.unsupportedTokens...)
			pErrs = append(pErrs, res.parseErrors...)
		}

		if prevHadPrefix {
			if len(pErrs) == 0 {
				pErrs = []string{"ambiguous:joined_header_continuation"}
			}
			return parsedHeaderSegment{
				target:            "",
				scopeUnknown:      true,
				rawValue:          seg,
				tokens:            tokens,
				unsupportedTokens: uToks,
				parseErrors:       pErrs,
			}, true
		}
		return parsedHeaderSegment{
			target:            "*",
			scopeUnknown:      false,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: uToks,
			parseErrors:       pErrs,
		}, false
	}

	firstPart := strings.TrimSpace(seg[:colonIdx])
	firstPartLower := strings.ToLower(firstPart)
	rest := strings.TrimSpace(seg[colonIdx+1:])

	if isParameterizedDirective(firstPartLower) {
		res := validateDirectiveItem(seg, true)
		tokens := []string{strings.ToLower(seg)}
		if prevHadPrefix {
			pErrs := res.parseErrors
			if len(pErrs) == 0 {
				pErrs = []string{"ambiguous:joined_header_continuation"}
			}
			return parsedHeaderSegment{
				target:            "",
				scopeUnknown:      true,
				rawValue:          seg,
				tokens:            tokens,
				unsupportedTokens: res.unsupportedTokens,
				parseErrors:       pErrs,
			}, true
		}
		return parsedHeaderSegment{
			target:            "*",
			scopeUnknown:      false,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: res.unsupportedTokens,
			parseErrors:       res.parseErrors,
		}, false
	}

	if isStandaloneSupportedDirective(firstPartLower) || reNearMissParam.MatchString(firstPartLower) {
		res := validateDirectiveItem(seg, true)
		tokens := []string{strings.ToLower(seg)}
		if prevHadPrefix {
			pErrs := res.parseErrors
			if len(pErrs) == 0 {
				pErrs = []string{"ambiguous:joined_header_continuation"}
			}
			return parsedHeaderSegment{
				target:            "",
				scopeUnknown:      true,
				rawValue:          seg,
				tokens:            tokens,
				unsupportedTokens: res.unsupportedTokens,
				parseErrors:       pErrs,
			}, true
		}
		return parsedHeaderSegment{
			target:            "*",
			scopeUnknown:      false,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: res.unsupportedTokens,
			parseErrors:       res.parseErrors,
		}, false
	}

	if agentName, ok := isSupportedAgentToken(firstPartLower); ok {
		if rest == "" {
			return parsedHeaderSegment{
				target:       agentName,
				scopeUnknown: false,
				rawValue:     seg,
				parseErrors:  []string{"parse_error:missing_directive_after_prefix:" + firstPartLower + ":"},
			}, true
		}
		var tokens []string
		var uToks []string
		var pErrs []string
		restColonIdx := strings.Index(rest, ":")
		if restColonIdx != -1 {
			restFirst := strings.TrimSpace(rest[:restColonIdx])
			if isParameterizedDirective(restFirst) || isStandaloneSupportedDirective(restFirst) || reNearMissParam.MatchString(restFirst) {
				res := validateDirectiveItem(rest, true)
				tokens = []string{strings.ToLower(rest)}
				uToks = res.unsupportedTokens
				pErrs = res.parseErrors
			} else {
				tokens = parseDirectiveTokens(rest)
				for _, tok := range tokens {
					res := validateDirectiveItem(tok, true)
					uToks = append(uToks, res.unsupportedTokens...)
					pErrs = append(pErrs, res.parseErrors...)
				}
			}
		} else {
			tokens = parseDirectiveTokens(rest)
			for _, tok := range tokens {
				res := validateDirectiveItem(tok, true)
				uToks = append(uToks, res.unsupportedTokens...)
				pErrs = append(pErrs, res.parseErrors...)
			}
		}
		return parsedHeaderSegment{
			target:            agentName,
			scopeUnknown:      false,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: uToks,
			parseErrors:       pErrs,
		}, true
	}

	if isRecognizedCrawlerAgent(firstPartLower) {
		agentName := firstPartLower
		tokens := parseDirectiveTokens(rest)
		var uToks []string
		var pErrs []string
		for _, tok := range tokens {
			res := validateDirectiveItem(tok, true)
			uToks = append(uToks, res.unsupportedTokens...)
			pErrs = append(pErrs, res.parseErrors...)
		}
		return parsedHeaderSegment{
			target:            agentName,
			scopeUnknown:      false,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: uToks,
			parseErrors:       pErrs,
		}, true
	}

	// Unknown prefix-like syntax (e.g. foo: bar)
	tokens := parseDirectiveTokens(seg)
	var pErrs []string
	if reValidIdentifier.MatchString(firstPart) {
		restTokens := parseDirectiveTokens(rest)
		allValidV1 := len(restTokens) > 0
		for _, tok := range restTokens {
			if !isStandaloneSupportedDirective(tok) {
				allValidV1 = false
				break
			}
		}
		if allValidV1 {
			pErrs = []string{"ambiguous:unverified_agent_prefix:" + firstPartLower}
		} else if len(restTokens) == 1 && reValidIdentifier.MatchString(restTokens[0]) {
			pErrs = []string{"ambiguous:unrecognized_colon_pair:" + firstPart + ": " + rest}
		} else {
			pErrs = []string{"ambiguous:unverified_agent_prefix:" + firstPartLower}
		}
	} else {
		pErrs = []string{"parse_error:invalid_syntax:" + seg}
	}

	return parsedHeaderSegment{
		target:       "",
		scopeUnknown: true,
		rawValue:     seg,
		tokens:       tokens,
		parseErrors:  pErrs,
	}, true
}

func parseDiscreteXRobotsSegment(seg string, prevHadPrefix bool, currentAgent string) (parsedHeaderSegment, bool, string) {
	colonIdx := strings.Index(seg, ":")
	if colonIdx == -1 {
		tokens := parseDirectiveTokens(seg)
		var pErrs []string
		var uToks []string
		for _, tok := range tokens {
			res := validateDirectiveItem(tok, true)
			uToks = append(uToks, res.unsupportedTokens...)
			pErrs = append(pErrs, res.parseErrors...)
		}

		if prevHadPrefix && currentAgent != "" {
			return parsedHeaderSegment{
				target:            currentAgent,
				scopeUnknown:      false,
				rawValue:          seg,
				tokens:            tokens,
				unsupportedTokens: uToks,
				parseErrors:       pErrs,
			}, true, currentAgent
		}

		return parsedHeaderSegment{
			target:            "*",
			scopeUnknown:      false,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: uToks,
			parseErrors:       pErrs,
		}, false, ""
	}

	firstPart := strings.TrimSpace(seg[:colonIdx])
	firstPartLower := strings.ToLower(firstPart)
	rest := strings.TrimSpace(seg[colonIdx+1:])

	if isParameterizedDirective(firstPartLower) {
		res := validateDirectiveItem(seg, true)
		tokens := []string{strings.ToLower(seg)}
		target := "*"
		scopeUnknown := false
		if prevHadPrefix && currentAgent != "" {
			target = currentAgent
		}
		return parsedHeaderSegment{
			target:            target,
			scopeUnknown:      scopeUnknown,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: res.unsupportedTokens,
			parseErrors:       res.parseErrors,
		}, prevHadPrefix, currentAgent
	}

	if isStandaloneSupportedDirective(firstPartLower) || reNearMissParam.MatchString(firstPartLower) {
		res := validateDirectiveItem(seg, true)
		tokens := []string{strings.ToLower(seg)}
		return parsedHeaderSegment{
			target:            "*",
			scopeUnknown:      false,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: res.unsupportedTokens,
			parseErrors:       res.parseErrors,
		}, false, ""
	}

	var agentName string
	var isCrawler bool
	if supportedName, ok := isSupportedAgentToken(firstPartLower); ok {
		agentName = supportedName
		isCrawler = true
	} else if isRecognizedCrawlerAgent(firstPartLower) {
		agentName = firstPartLower
		isCrawler = true
	}

	if isCrawler {
		if prevHadPrefix && currentAgent != "" && currentAgent != agentName {
			tokens := parseDirectiveTokens(rest)
			var uToks []string
			var pErrs []string
			for _, tok := range tokens {
				res := validateDirectiveItem(tok, true)
				uToks = append(uToks, res.unsupportedTokens...)
				pErrs = append(pErrs, res.parseErrors...)
			}
			hasViolation := false
			for _, err := range pErrs {
				if strings.HasPrefix(err, "parse_error:") || strings.HasPrefix(err, "unsupported_directive:") {
					hasViolation = true
					break
				}
			}
			if !hasViolation {
				pErrs = []string{"ambiguous:multiple_agent_prefixes"}
			}
			return parsedHeaderSegment{
				target:            "",
				scopeUnknown:      true,
				rawValue:          seg,
				tokens:            tokens,
				unsupportedTokens: uToks,
				parseErrors:       pErrs,
			}, true, agentName
		}

		if rest == "" {
			return parsedHeaderSegment{
				target:       agentName,
				scopeUnknown: false,
				rawValue:     seg,
				parseErrors:  []string{"parse_error:missing_directive_after_prefix:" + firstPartLower + ":"},
			}, true, agentName
		}

		tokens := parseDirectiveTokens(rest)
		var uToks []string
		var pErrs []string
		for _, tok := range tokens {
			res := validateDirectiveItem(tok, true)
			uToks = append(uToks, res.unsupportedTokens...)
			pErrs = append(pErrs, res.parseErrors...)
		}
		return parsedHeaderSegment{
			target:            agentName,
			scopeUnknown:      false,
			rawValue:          seg,
			tokens:            tokens,
			unsupportedTokens: uToks,
			parseErrors:       pErrs,
		}, true, agentName
	}

	tokens := parseDirectiveTokens(seg)
	var pErrs []string
	if reValidIdentifier.MatchString(firstPart) {
		restTokens := parseDirectiveTokens(rest)
		allValidV1 := len(restTokens) > 0
		for _, tok := range restTokens {
			if !isStandaloneSupportedDirective(tok) {
				allValidV1 = false
				break
			}
		}
		if allValidV1 {
			pErrs = []string{"ambiguous:unverified_agent_prefix:" + firstPartLower}
		} else if len(restTokens) == 1 && reValidIdentifier.MatchString(restTokens[0]) {
			pErrs = []string{"ambiguous:unrecognized_colon_pair:" + firstPart + ": " + rest}
		} else {
			pErrs = []string{"ambiguous:unverified_agent_prefix:" + firstPartLower}
		}
	} else {
		pErrs = []string{"parse_error:invalid_syntax:" + seg}
	}

	return parsedHeaderSegment{
		target:       "",
		scopeUnknown: true,
		rawValue:     seg,
		tokens:       tokens,
		parseErrors:  pErrs,
	}, true, ""
}

func processPageDirectives(
	req BuildRequest,
	urlID int,
	urlIDStr audit.URLID,
	pageSrcRef string,
	obsTime time.Time,
	isHTML bool,
	isRendered bool,
	robotsMetaExtracted bool,
	rawRobotsMeta []sitecrawl.RawMetaDirective,
	pageMetaTags map[string]string,
	pageMetaRobots string,
	columnMetaRobots string,
	xRobotsRaw []string,
	isDiscreteHeaders bool,
	nextObsID func() audit.ObservationID,
) (
	directives []audit.RobotsDirectiveObservation,
	normalized []audit.NormalizedObservation,
	gaps []EvidenceGap,
) {
	metaDirIdx := 0
	var (
		genericMetaRaw string
		agentMetaRaw   = make(map[string]string)
	)

	// 1. Meta directives (suppressed on rendered pages or non-HTML)
	if isHTML && !isRendered {
		if robotsMetaExtracted {
			// V1.10b+ encounter-order extraction
			for _, rm := range rawRobotsMeta {
				agentName := strings.TrimSpace(rm.Name)
				if !isRecognizedCrawlerAgent(agentName) {
					continue
				}
				target := strings.ToLower(agentName)
				if target == "robots" {
					target = "*"
				}

				if !rm.HasContentAttr {
					// Case 2A: Missing content attribute
					parseErrs := []string{"parse_error:missing_content_attribute"}
					directives = append(directives, audit.RobotsDirectiveObservation{
						RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:meta:%d", req.AuditRunID, urlID, metaDirIdx)),
						AuditRunID:                   req.AuditRunID,
						URLID:                        urlIDStr,
						Source:                       audit.DirectiveSourceMeta,
						Target:                       target,
						ScopeUnknown:                 false,
						RawValue:                     "",
						ParsedTokens:                 nil,
						UnsupportedTokens:            nil,
						ParseErrors:                  parseErrs,
						EffectiveNoindex:             false,
						ObservedAt:                   obsTime,
					})
					metaDirIdx++
					continue
				}

				trimmedContent := strings.TrimSpace(rm.ContentRaw)
				if trimmedContent == "" {
					// Case 2B/2C: Vacuous 0-token declaration; do not emit bundle
					continue
				}

				// Check for meta agent prefix violation
				var (
					parsedTokens      []string
					unsupportedTokens []string
					parseErrors       []string
				)

				colonIdx := strings.Index(trimmedContent, ":")
				if colonIdx != -1 {
					lhs := strings.ToLower(strings.TrimSpace(trimmedContent[:colonIdx]))
					if isRecognizedCrawlerAgent(lhs) && lhs != "robots" {
						parseErrors = append(parseErrors, "parse_error:meta_agent_prefix:"+trimmedContent)
					}
				}

				if len(parseErrors) == 0 {
					items := splitDirectiveItems(rm.ContentRaw)
					for _, item := range items {
						res := validateDirectiveItem(item, false)
						if res.token != "" {
							parsedTokens = append(parsedTokens, res.token)
						}
						unsupportedTokens = append(unsupportedTokens, res.unsupportedTokens...)
						parseErrors = append(parseErrors, res.parseErrors...)
					}
				}

				isNoindex := false
				if (target == "*" || target == "googlebot") && (containsToken(parsedTokens, "noindex") || containsToken(parsedTokens, "none")) {
					isNoindex = true
				}

				directives = append(directives, audit.RobotsDirectiveObservation{
					RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:meta:%d", req.AuditRunID, urlID, metaDirIdx)),
					AuditRunID:                   req.AuditRunID,
					URLID:                        urlIDStr,
					Source:                       audit.DirectiveSourceMeta,
					Target:                       target,
					ScopeUnknown:                 false,
					RawValue:                     rm.ContentRaw,
					ParsedTokens:                 parsedTokens,
					UnsupportedTokens:            unsupportedTokens,
					ParseErrors:                  parseErrors,
					EffectiveNoindex:             isNoindex,
					ObservedAt:                   obsTime,
				})
				metaDirIdx++

				if target == "*" {
					if genericMetaRaw == "" {
						genericMetaRaw = rm.ContentRaw
					} else {
						genericMetaRaw += ", " + rm.ContentRaw
					}
				} else {
					if agentMetaRaw[target] == "" {
						agentMetaRaw[target] = rm.ContentRaw
					} else {
						agentMetaRaw[target] += ", " + rm.ContentRaw
					}
				}
			}
		} else {
			// Legacy SQLite fallback
			metaRobotsRawStr := ""
			if pageMetaRobots != "" {
				metaRobotsRawStr = pageMetaRobots
			} else if columnMetaRobots != "" {
				metaRobotsRawStr = columnMetaRobots
			}
			metaRobotsRawStr = strings.TrimSpace(metaRobotsRawStr)

			var (
				robotsVal string
				hasRobots bool
			)
			if pageMetaTags != nil {
				for k, v := range pageMetaTags {
					val := strings.TrimSpace(v)
					if val == "" {
						continue
					}
					kLower := strings.ToLower(strings.TrimSpace(k))
					switch kLower {
					case "robots":
						robotsVal = val
						hasRobots = true
					default:
						if agent, ok := isSupportedAgentToken(kLower); ok {
							agentMetaRaw[agent] = val
						}
					}
				}
			}

			if hasRobots || len(agentMetaRaw) > 0 {
				if hasRobots {
					var (
						parsedTokens      []string
						unsupportedTokens []string
						parseErrors       []string
					)
					items := splitDirectiveItems(robotsVal)
					for _, item := range items {
						res := validateDirectiveItem(item, false)
						if res.token != "" {
							parsedTokens = append(parsedTokens, res.token)
						}
						unsupportedTokens = append(unsupportedTokens, res.unsupportedTokens...)
						parseErrors = append(parseErrors, res.parseErrors...)
					}
					isNoindex := containsToken(parsedTokens, "noindex") || containsToken(parsedTokens, "none")
					directives = append(directives, audit.RobotsDirectiveObservation{
						RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:meta:%d", req.AuditRunID, urlID, metaDirIdx)),
						AuditRunID:                   req.AuditRunID,
						URLID:                        urlIDStr,
						Source:                       audit.DirectiveSourceMeta,
						Target:                       "*",
						ScopeUnknown:                 false,
						RawValue:                     robotsVal,
						ParsedTokens:                 parsedTokens,
						UnsupportedTokens:            unsupportedTokens,
						ParseErrors:                  parseErrors,
						EffectiveNoindex:             isNoindex,
						ObservedAt:                   obsTime,
					})
					metaDirIdx++
				}

				var agentKeys []string
				for k := range agentMetaRaw {
					agentKeys = append(agentKeys, k)
				}
				sort.Strings(agentKeys)
				for _, agent := range agentKeys {
					val := agentMetaRaw[agent]
					var (
						parsedTokens      []string
						unsupportedTokens []string
						parseErrors       []string
					)
					items := splitDirectiveItems(val)
					for _, item := range items {
						res := validateDirectiveItem(item, false)
						if res.token != "" {
							parsedTokens = append(parsedTokens, res.token)
						}
						unsupportedTokens = append(unsupportedTokens, res.unsupportedTokens...)
						parseErrors = append(parseErrors, res.parseErrors...)
					}
					isNoindex := false
					if agent == "googlebot" && (containsToken(parsedTokens, "noindex") || containsToken(parsedTokens, "none")) {
						isNoindex = true
					}
					directives = append(directives, audit.RobotsDirectiveObservation{
						RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:meta:%d", req.AuditRunID, urlID, metaDirIdx)),
						AuditRunID:                   req.AuditRunID,
						URLID:                        urlIDStr,
						Source:                       audit.DirectiveSourceMeta,
						Target:                       agent,
						ScopeUnknown:                 false,
						RawValue:                     val,
						ParsedTokens:                 parsedTokens,
						UnsupportedTokens:            unsupportedTokens,
						ParseErrors:                  parseErrors,
						EffectiveNoindex:             isNoindex,
						ObservedAt:                   obsTime,
					})
					metaDirIdx++
				}

				if metaRobotsRawStr != "" {
					allTokens := parseDirectiveTokens(metaRobotsRawStr)
					rTokens := parseDirectiveTokens(robotsVal)
					var agentTokens []string
					for _, val := range agentMetaRaw {
						agentTokens = append(agentTokens, parseDirectiveTokens(val)...)
					}
					var extraTokens []string
					for _, t := range allTokens {
						if !containsToken(rTokens, t) && !containsToken(agentTokens, t) {
							extraTokens = append(extraTokens, t)
						}
					}
					if len(extraTokens) > 0 {
						extraRaw := strings.Join(extraTokens, ", ")
						if hasRobots && len(agentMetaRaw) == 0 {
							isNoindex := containsToken(extraTokens, "noindex") || containsToken(extraTokens, "none")
							directives = append(directives, audit.RobotsDirectiveObservation{
								RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:meta:%d", req.AuditRunID, urlID, metaDirIdx)),
								AuditRunID:                   req.AuditRunID,
								URLID:                        urlIDStr,
								Source:                       audit.DirectiveSourceMeta,
								Target:                       "*",
								ScopeUnknown:                 false,
								RawValue:                     extraRaw,
								ParsedTokens:                 extraTokens,
								EffectiveNoindex:             isNoindex,
								ObservedAt:                   obsTime,
							})
							metaDirIdx++
						} else {
							directives = append(directives, audit.RobotsDirectiveObservation{
								RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:meta:%d", req.AuditRunID, urlID, metaDirIdx)),
								AuditRunID:                   req.AuditRunID,
								URLID:                        urlIDStr,
								Source:                       audit.DirectiveSourceMeta,
								Target:                       "",
								ScopeUnknown:                 true,
								RawValue:                     extraRaw,
								ParsedTokens:                 extraTokens,
								EffectiveNoindex:             false,
								ObservedAt:                   obsTime,
							})
							metaDirIdx++
							gaps = append(gaps, EvidenceGap{
								GapCode:         GapDirectiveScopeAmbiguous,
								SubjectRef:      string(urlIDStr),
								Field:           "meta_robots",
								Reason:          fmt.Sprintf("Flattened MetaRobots contains unrecovered extra tokens: %s", extraRaw),
								SourceComponent: "sitecrawl_pages",
							})
						}
					}
				}

				if hasRobots && len(agentMetaRaw) == 0 {
					genericMetaRaw = metaRobotsRawStr
					if genericMetaRaw == "" {
						genericMetaRaw = robotsVal
					}
				} else if hasRobots {
					genericMetaRaw = robotsVal
				}
			} else if metaRobotsRawStr != "" {
				tokens := parseDirectiveTokens(metaRobotsRawStr)
				directives = append(directives, audit.RobotsDirectiveObservation{
					RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:meta:%d", req.AuditRunID, urlID, metaDirIdx)),
					AuditRunID:                   req.AuditRunID,
					URLID:                        urlIDStr,
					Source:                       audit.DirectiveSourceMeta,
					Target:                       "",
					ScopeUnknown:                 true,
					RawValue:                     metaRobotsRawStr,
					ParsedTokens:                 tokens,
					EffectiveNoindex:             false,
					ObservedAt:                   obsTime,
				})
				metaDirIdx++
				gaps = append(gaps, EvidenceGap{
					GapCode:         GapDirectiveScopeAmbiguous,
					SubjectRef:      string(urlIDStr),
					Field:           "meta_robots",
					Reason:          "Meta robots directive scope cannot be recovered from legacy evidence without scoped MetaTags.",
					SourceComponent: "sitecrawl_pages",
				})
			}
		}
	}

	// 2. HTTP Header X-Robots-Tag directives
	headerDirIdx := 0
	var (
		genericHeaderRawSegments []string
		agentHeaderRawSegments   = make(map[string][]string)
	)

	if isDiscreteHeaders {
		for _, headerStr := range xRobotsRaw {
			trimmedHeader := strings.TrimSpace(headerStr)
			if trimmedHeader == "" {
				continue
			}
			segments := splitDirectiveItems(trimmedHeader)
			hadPrefix := false
			currentAgent := ""
			for _, seg := range segments {
				s := strings.TrimSpace(seg)
				if s == "" {
					continue
				}
				parsed, nextHadPrefix, nextAgent := parseDiscreteXRobotsSegment(s, hadPrefix, currentAgent)
				hadPrefix = nextHadPrefix
				currentAgent = nextAgent

				if parsed.target == "*" && !parsed.scopeUnknown {
					genericHeaderRawSegments = append(genericHeaderRawSegments, parsed.rawValue)
				} else if parsed.target != "" && !parsed.scopeUnknown {
					agentHeaderRawSegments[parsed.target] = append(agentHeaderRawSegments[parsed.target], parsed.rawValue)
				}

				isNoindex := false
				if (parsed.target == "*" || parsed.target == "googlebot") && !parsed.scopeUnknown && (containsToken(parsed.tokens, "noindex") || containsToken(parsed.tokens, "none")) {
					isNoindex = true
				}

				directives = append(directives, audit.RobotsDirectiveObservation{
					RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:header:%d", req.AuditRunID, urlID, headerDirIdx)),
					AuditRunID:                   req.AuditRunID,
					URLID:                        urlIDStr,
					Source:                       audit.DirectiveSourceHTTPHeader,
					Target:                       parsed.target,
					ScopeUnknown:                 parsed.scopeUnknown,
					RawValue:                     parsed.rawValue,
					ParsedTokens:                 parsed.tokens,
					UnsupportedTokens:            parsed.unsupportedTokens,
					ParseErrors:                  parsed.parseErrors,
					EffectiveNoindex:             isNoindex,
					ObservedAt:                   obsTime,
				})
				headerDirIdx++

				if parsed.scopeUnknown {
					gaps = append(gaps, EvidenceGap{
						GapCode:         GapDirectiveScopeAmbiguous,
						SubjectRef:      string(urlIDStr),
						Field:           "x_robots_tag",
						Reason:          fmt.Sprintf("X-Robots-Tag directive segment %q has ambiguous applicability scope due to multi-agent or unrecognized prefix.", s),
						SourceComponent: "sitecrawl_pages",
					})
				}
			}
		}
	} else {
		for _, headerStr := range xRobotsRaw {
			trimmedHeader := strings.TrimSpace(headerStr)
			if trimmedHeader == "" {
				continue
			}
			segments := splitDirectiveItems(trimmedHeader)
			hadPrefix := false
			for _, seg := range segments {
				s := strings.TrimSpace(seg)
				if s == "" {
					continue
				}
				parsed, nextHadPrefix := parseXRobotsSegment(s, hadPrefix)
				hadPrefix = nextHadPrefix

				if parsed.target == "*" && !parsed.scopeUnknown {
					genericHeaderRawSegments = append(genericHeaderRawSegments, parsed.rawValue)
				} else if parsed.target != "" && !parsed.scopeUnknown {
					agentHeaderRawSegments[parsed.target] = append(agentHeaderRawSegments[parsed.target], parsed.rawValue)
				}

				isNoindex := false
				if (parsed.target == "*" || parsed.target == "googlebot") && !parsed.scopeUnknown && (containsToken(parsed.tokens, "noindex") || containsToken(parsed.tokens, "none")) {
					isNoindex = true
				}

				directives = append(directives, audit.RobotsDirectiveObservation{
					RobotsDirectiveObservationID: audit.RobotsDirectiveID(fmt.Sprintf("directive:%s:%d:header:%d", req.AuditRunID, urlID, headerDirIdx)),
					AuditRunID:                   req.AuditRunID,
					URLID:                        urlIDStr,
					Source:                       audit.DirectiveSourceHTTPHeader,
					Target:                       parsed.target,
					ScopeUnknown:                 parsed.scopeUnknown,
					RawValue:                     parsed.rawValue,
					ParsedTokens:                 parsed.tokens,
					UnsupportedTokens:            parsed.unsupportedTokens,
					ParseErrors:                  parsed.parseErrors,
					EffectiveNoindex:             isNoindex,
					ObservedAt:                   obsTime,
				})
				headerDirIdx++

				if parsed.scopeUnknown {
					gaps = append(gaps, EvidenceGap{
						GapCode:         GapDirectiveScopeAmbiguous,
						SubjectRef:      string(urlIDStr),
						Field:           "x_robots_tag",
						Reason:          fmt.Sprintf("X-Robots-Tag directive segment %q has ambiguous applicability scope due to joined header boundaries.", s),
						SourceComponent: "sitecrawl_pages",
					})
				}
			}
		}
	}

	// 3. Emit NormalizedObservations for each directive observation
	for _, d := range directives {
		dirID := string(d.RobotsDirectiveObservationID)
		refs := []string{dirID, pageSrcRef}

		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "directive_source",
			Value:              string(d.Source),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: refs,
			ObservedAt:         obsTime,
		})

		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "directive_target",
			Value:              d.Target,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: refs,
			ObservedAt:         obsTime,
		})

		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "directive_raw",
			Value:              d.RawValue,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: refs,
			ObservedAt:         obsTime,
		})

		tokensVal := strings.Join(d.ParsedTokens, ", ")
		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "directive_tokens",
			Value:              tokensVal,
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: refs,
			ObservedAt:         obsTime,
		})

		if d.ScopeUnknown {
			normalized = append(normalized, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "directive_scope_unknown",
				Value:              "true",
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: refs,
				ObservedAt:         obsTime,
			})
		}

		// AR-INDEX-003: robots_directive_tokens
		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "robots_directive_tokens",
			Value:              tokensVal,
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: refs,
			ObservedAt:         obsTime,
		})

		// AR-INDEX-003: robots_parse_errors
		parseErrsVal := "none"
		if len(d.ParseErrors) > 0 {
			parseErrsVal = strings.Join(d.ParseErrors, ", ")
		}
		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "robots_parse_errors",
			Value:              parseErrsVal,
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: refs,
			ObservedAt:         obsTime,
		})

		// AR-INDEX-003: directive_unsupported_tokens
		if len(d.UnsupportedTokens) > 0 {
			normalized = append(normalized, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "directive_unsupported_tokens",
				Value:              strings.Join(d.UnsupportedTokens, ", "),
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: refs,
				ObservedAt:         obsTime,
			})
		}

		// AR-INDEX-002: robots_meta_tokens and x_robots_tokens for applicable generic scope
		if d.Target == "*" && !d.ScopeUnknown {
			if d.Source == audit.DirectiveSourceMeta {
				normalized = append(normalized, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "robots_meta_tokens",
					Value:              tokensVal,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: refs,
					ObservedAt:         obsTime,
				})
			} else if d.Source == audit.DirectiveSourceHTTPHeader && isHTML {
				normalized = append(normalized, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "x_robots_tokens",
					Value:              tokensVal,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: refs,
					ObservedAt:         obsTime,
				})
			}
		}
	}

	// 4. URL-level Raw Observations
	if genericMetaRaw != "" && !isRendered {
		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "meta_robots_raw",
			Value:              genericMetaRaw,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})
	}

	if !isRendered {
		var agentMetaKeys []string
		for k := range agentMetaRaw {
			agentMetaKeys = append(agentMetaKeys, k)
		}
		sort.Strings(agentMetaKeys)
		for _, agent := range agentMetaKeys {
			normalized = append(normalized, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              fmt.Sprintf("%s_meta_robots_raw", agent),
				Value:              agentMetaRaw[agent],
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}
	}

	if len(genericHeaderRawSegments) > 0 {
		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              "x_robots_raw",
			Value:              strings.Join(genericHeaderRawSegments, ", "),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})
	}

	var agentHeaderKeys []string
	for k := range agentHeaderRawSegments {
		agentHeaderKeys = append(agentHeaderKeys, k)
	}
	sort.Strings(agentHeaderKeys)
	for _, agent := range agentHeaderKeys {
		normalized = append(normalized, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         string(urlIDStr),
			Field:              fmt.Sprintf("%s_x_robots_raw", agent),
			Value:              strings.Join(agentHeaderRawSegments[agent], ", "),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{pageSrcRef},
			ObservedAt:         obsTime,
		})
	}

	// 5. Effective Noindex (Googlebot-effective page-level noindex normalization, HTML only)
	if isHTML {
		hasGooglebotApplicableDirective := false
		hasGooglebotNoindex := false
		hasAmbiguousDirective := false

		for _, d := range directives {
			if d.ScopeUnknown {
				hasAmbiguousDirective = true
			} else if d.Target == "*" || d.Target == "googlebot" {
				hasGooglebotApplicableDirective = true
				if containsToken(d.ParsedTokens, "noindex") || containsToken(d.ParsedTokens, "none") {
					hasGooglebotNoindex = true
				}
			}
		}

		canProveNoindex := hasGooglebotNoindex
		canProveIndexable := !isRendered && !hasAmbiguousDirective && !hasGooglebotNoindex && (hasGooglebotApplicableDirective || pageMetaTags != nil || robotsMetaExtracted)

		if canProveNoindex || canProveIndexable {
			noindexVal := "false"
			if canProveNoindex {
				noindexVal = "true"
			}
			normalized = append(normalized, audit.NormalizedObservation{
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

		// 6. Snippet Directives (AR-INDEX-005, HTML only)
		hasNosnippet := false
		maxSnippetVal := -1
		hasMaxSnippet := false

		for _, d := range directives {
			if !d.ScopeUnknown && (d.Target == "*" || d.Target == "googlebot") {
				if containsToken(d.ParsedTokens, "nosnippet") {
					hasNosnippet = true
				}
				for _, tok := range d.ParsedTokens {
					if strings.HasPrefix(tok, "max-snippet:") {
						valStr := strings.TrimPrefix(tok, "max-snippet:")
						if val, err := strconv.Atoi(valStr); err == nil {
							if !hasMaxSnippet || val < maxSnippetVal {
								maxSnippetVal = val
								hasMaxSnippet = true
							}
						}
					}
				}
			}
		}

		if pageMetaTags != nil || robotsMetaExtracted || len(directives) > 0 {
			normalized = append(normalized, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "nosnippet",
				Value:              strconv.FormatBool(hasNosnippet),
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})

			if hasMaxSnippet {
				normalized = append(normalized, audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         string(urlIDStr),
					Field:              "max_snippet",
					Value:              strconv.Itoa(maxSnippetVal),
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{pageSrcRef},
					ObservedAt:         obsTime,
				})
			}

			normalized = append(normalized, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "data_nosnippet_count",
				Value:              "0",
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}
	}

	return directives, normalized, gaps
}
