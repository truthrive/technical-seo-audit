package adapter

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
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

	// 2. Query and verify crawl run state
	var (
		runState, seedURL, host, optJSON, startedAtStr string
		finishedAtStr                                 sql.NullString
		foundCount, crawledCount                      int
	)
	err := db.QueryRowContext(ctx,
		`SELECT state, seed_url, host, options, started_at, finished_at, found, crawled
		 FROM sitecrawl_runs WHERE id = ?`, req.CrawlRunID).
		Scan(&runState, &seedURL, &host, &optJSON, &startedAtStr, &finishedAtStr, &foundCount, &crawledCount)
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

	var opts sitecrawl.Options
	if optJSON != "" {
		_ = json.Unmarshal([]byte(optJSON), &opts)
	}

	runStartedAt, err := time.Parse(time.RFC3339, startedAtStr)
	if err != nil {
		runStartedAt = time.Now().UTC()
	}

	// 3. Load URL dictionary
	urlRows, err := db.QueryContext(ctx,
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
		page         sitecrawl.Page
	}

	pageRows, err := db.QueryContext(ctx,
		`SELECT url_id, url, data, kind, is_internal, depth, discovered_by,
		        status, content_type, size_bytes, response_ms, redirect_to,
		        redirect_hops, error_type, title, meta_desc, h1, lang, canonical,
		        meta_robots, x_robots, rendered, crawled_at
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
			&pr.metaRobots, &pr.xRobots, &pr.rendered, &pr.crawledAt,
		); err != nil {
			return nil, fmt.Errorf("audit adapter: scan page: %w", err)
		}

		if pr.dataJSON != "" {
			_ = json.Unmarshal([]byte(pr.dataJSON), &pr.page)
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

	linkRows, err := db.QueryContext(ctx,
		`SELECT src_id, dst_id, seq, placement, flags, anchor
		 FROM sitecrawl_links WHERE run_id = ? ORDER BY src_id ASC, seq ASC`, req.CrawlRunID)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: query links: %w", err)
	}
	defer linkRows.Close()

	incomingAnchorEdges := make(map[int][]linkRow)
	var linkObservations []audit.LinkObservation

	for linkRows.Next() {
		var lr linkRow
		if err := linkRows.Scan(&lr.srcID, &lr.dstID, &lr.seq, &lr.placement, &lr.flags, &lr.anchor); err != nil {
			return nil, fmt.Errorf("audit adapter: scan link: %w", err)
		}

		isResource := (lr.flags&(sitecrawl.FlagImageLink|sitecrawl.FlagStylesheet|sitecrawl.FlagScript) != 0) ||
			(lr.placement == sitecrawl.PlacementImage)

		// Record legitimate internal anchor links for discovery provenance
		if !isResource && (lr.flags&sitecrawl.FlagInternal != 0) {
			incomingAnchorEdges[lr.dstID] = append(incomingAnchorEdges[lr.dstID], lr)
		}

		// Build LinkObservation only for non-resource anchor edges
		if !isResource {
			targetURLResolved := urlsByID[lr.dstID]
			targetURLID := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, lr.dstID))

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
				TargetURLID:         &targetURLID,
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

	// 6. Build UrlResource objects
	var urlResources []audit.UrlResource
	for urlID, rawURL := range urlsByID {
		parsed, err := url.Parse(rawURL)
		var (
			scheme, host, pathStr, query, origin string
			port                                int
		)
		if err == nil {
			scheme = parsed.Scheme
			host = parsed.Hostname()
			pathStr = parsed.EscapedPath()
			query = parsed.RawQuery
			origin = parsed.Scheme + "://" + parsed.Host
			if p := parsed.Port(); p != "" {
				port, _ = strconv.Atoi(p)
			}
		} else {
			origin = rawURL
		}

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
			NormalizedURL:   rawURL,
			Scheme:          scheme,
			Host:            host,
			Port:            port,
			Path:            pathStr,
			Query:           query,
			FragmentRemoved: false,
			IsInternal:      isInternal,
			Origin:          origin,
			CreatedAt:       nil, // Not persisted in SQLite
		}
		urlResources = append(urlResources, ur)
	}

	// 7. Build DiscoveryRecords with strict provenance
	var (
		discoveryRecords []audit.DiscoveryRecord
		evidenceGaps     []EvidenceGap
	)

	// Pre-populate standard global gaps
	evidenceGaps = append(evidenceGaps,
		EvidenceGap{
			GapCode:         GapRawCanonicalUnavailable,
			Field:           "raw_canonical_values",
			Reason:          "SiteCrawl normalizes and resolves canonical references before SQLite persistence; raw canonical declaration syntax is unavailable.",
			SourceComponent: "sitecrawl_pages",
		},
		EvidenceGap{
			GapCode:         GapRawHrefUnavailable,
			Field:           "href_raw",
			Reason:          "SiteCrawl resolves link targets before SQLite persistence; original raw href attributes are not preserved.",
			SourceComponent: "sitecrawl_links",
		},
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

	for urlID, rawURL := range urlsByID {
		urlIDStr := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, urlID))
		pr := pagesByURLID[urlID]

		// 7a. Seed URL discovery
		if rawURL == seedURL {
			discoveryRecords = append(discoveryRecords, audit.DiscoveryRecord{
				DiscoveryID:   audit.DiscoveryID(fmt.Sprintf("disc:%s:%d", req.AuditRunID, urlID)),
				AuditRunID:    req.AuditRunID,
				URLID:         urlIDStr,
				DiscoveryType: audit.DiscoveryStartURL,
				SourceURLID:   nil,
				SourceRef:     "sitecrawl_runs:" + req.CrawlRunID,
				DiscoveredAt:  runStartedAt,
			})
			continue
		}

		// 7b. List mode manual list discovery
		if opts.Mode == sitecrawl.ModeList {
			discoveryRecords = append(discoveryRecords, audit.DiscoveryRecord{
				DiscoveryID:   audit.DiscoveryID(fmt.Sprintf("disc:%s:%d", req.AuditRunID, urlID)),
				AuditRunID:    req.AuditRunID,
				URLID:         urlIDStr,
				DiscoveryType: audit.DiscoverySuppliedURLList,
				SourceURLID:   nil,
				SourceRef:     "sitecrawl_runs:" + req.CrawlRunID + ":list",
				DiscoveredAt:  runStartedAt,
			})
			continue
		}

		// 7c. Sitemap discovery
		if pr != nil && (pr.discoveredBy == sitecrawl.SourceSitemap || pr.page.Source == sitecrawl.SourceSitemap) {
			obsTime, _ := time.Parse(time.RFC3339, pr.crawledAt)
			if obsTime.IsZero() {
				obsTime = runStartedAt
			}
			discoveryRecords = append(discoveryRecords, audit.DiscoveryRecord{
				DiscoveryID:   audit.DiscoveryID(fmt.Sprintf("disc:%s:%d", req.AuditRunID, urlID)),
				AuditRunID:    req.AuditRunID,
				URLID:         urlIDStr,
				DiscoveryType: audit.DiscoverySitemap,
				SourceURLID:   nil,
				SourceRef:     fmt.Sprintf("sitecrawl_pages:%s:%d", req.CrawlRunID, urlID),
				DiscoveredAt:  obsTime,
			})
			continue
		}

		// 7d. Internal link discovery (MUST be verified by an incoming non-resource anchor link)
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
				DiscoveryID:   audit.DiscoveryID(fmt.Sprintf("disc:%s:%d", req.AuditRunID, urlID)),
				AuditRunID:    req.AuditRunID,
				URLID:         urlIDStr,
				DiscoveryType: audit.DiscoveryInternalLink,
				SourceURLID:   &srcURLID,
				SourceRef:     fmt.Sprintf("sitecrawl_links:%s:%d:%d", req.CrawlRunID, firstEdge.srcID, firstEdge.seq),
				DiscoveredAt:  obsTime,
			})
			continue
		}

		// 7e. Ambiguous or non-anchor provenance
		if pr != nil && (pr.discoveredBy == sitecrawl.SourceLink || pr.page.Source == sitecrawl.SourceLink) {
			evidenceGaps = append(evidenceGaps, EvidenceGap{
				GapCode:         GapDiscoveryProvenanceAmbiguous,
				SubjectRef:      string(urlIDStr),
				Field:           "discovery_record",
				Reason:          "Page is marked with source 'link' but has no supporting internal anchor edge in sitecrawl_links (e.g. canonical or hreflang reference).",
				SourceComponent: "sitecrawl_pages",
			})
		}
		// Redirect continuations (SourceRedirect) deliberately do not produce discovery records.
	}

	// 8. Build FetchObservations, RedirectHops, HtmlObservations, Directives, Canonicals
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

	for urlID, pr := range pagesByURLID {
		urlIDStr := audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, urlID))
		fetchID := audit.FetchID(fmt.Sprintf("fetch:%s:%d", req.AuditRunID, urlID))

		obsTime, err := time.Parse(time.RFC3339, pr.crawledAt)
		if err != nil {
			obsTime = runStartedAt
		}

		// Acquisition purpose determination
		var purpose audit.AcquisitionPurpose
		if pr.discoveredBy == sitecrawl.SourceRedirect || pr.page.Source == sitecrawl.SourceRedirect {
			purpose = audit.PurposeRedirectTarget
		} else if pr.url == seedURL || pr.discoveredBy == sitecrawl.SourceSitemap || len(incomingAnchorEdges[urlID]) > 0 {
			purpose = audit.PurposeCrawl
		} else {
			purpose = audit.PurposeCrawl
			evidenceGaps = append(evidenceGaps, EvidenceGap{
				GapCode:         GapAcquisitionPurposeAmbiguous,
				SubjectRef:      string(urlIDStr),
				Field:           "acquisition_purpose",
				Reason:          "Page fetch occurred without confirmed anchor discovery provenance; defaulted to CRAWL.",
				SourceComponent: "sitecrawl_pages",
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
			RequestProfile:      audit.ProfileDefault,
			RequestedAt:         nil, // Not stored
			CompletedAt:         nil, // Not stored
			FetchAttempted:      true,
			Status:              pr.status,
			FinalURLID:          finalURLID,
			ContentType:         pr.contentType,
			ResponseTimeMs:      int64(pr.responseMs),
			FetchErrorType:      pr.errorType,
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

		if pr.errorType != "" {
			normalizedObservations = append(normalizedObservations, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectURL,
				SubjectRef:         string(urlIDStr),
				Field:              "fetch_error_type",
				Value:              pr.errorType,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{pageSrcRef},
				ObservedAt:         obsTime,
			})
		}

		// 8b. Redirect Hops
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

		// 8c. HTML Observations
		isHTML := pr.kind == sitecrawl.KindHTML || strings.Contains(pr.contentType, "html")
		if isHTML {
			if pr.rendered == 1 || pr.page.Rendered {
				evidenceGaps = append(evidenceGaps, EvidenceGap{
					GapCode:         GapRenderedRawSourceUnavailable,
					SubjectRef:      string(urlIDStr),
					Field:           "raw_html_evidence",
					Reason:          "Page was rendered via headless browser; raw server-rendered HTML was replaced and is unavailable.",
					SourceComponent: "sitecrawl_pages",
				})
			}

			h1Values := pr.page.H1
			if len(h1Values) == 0 && pr.h1 != "" {
				h1Values = []string{pr.h1}
			}

			var metaRobotsRaw, xRobotsRaw []string
			if pr.page.MetaRobots != "" {
				metaRobotsRaw = append(metaRobotsRaw, pr.page.MetaRobots)
			} else if pr.metaRobots != "" {
				metaRobotsRaw = append(metaRobotsRaw, pr.metaRobots)
			}
			if pr.page.XRobotsTag != "" {
				xRobotsRaw = append(xRobotsRaw, pr.page.XRobotsTag)
			} else if pr.xRobots != "" {
				xRobotsRaw = append(xRobotsRaw, pr.xRobots)
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
				CanonicalRawValues:  nil, // Raw canonical unavailable
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

			// 8d. Robots directives
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

			// 8e. Canonical observations
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
					RawValues:              nil, // Raw canonical unavailable
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

	// 9. Add normalized observations for links
	for _, lo := range linkObservations {
		linkSrcRef := fmt.Sprintf("sitecrawl_links:%s", strings.TrimPrefix(string(lo.LinkID), "link:"+string(req.AuditRunID)+":"))
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

	// 10. Freeze EvidenceSnapshot
	frozenTime := time.Now().UTC()
	snapshot := &audit.EvidenceSnapshot{
		SnapshotID:               req.SnapshotID,
		AuditRunID:               req.AuditRunID,
		CreatedAt:                frozenTime,
		FrozenAt:                 &frozenTime,
		SnapshotStatus:           audit.SnapshotFrozen,
		NormalizationVersion:     "v1.2.0",
		CrawlComplete:            true,
		SitemapDiscoveryComplete: false,
		RenderSelectionComplete:  false,
		ProbeCollectionComplete:  false,
		NormalizedObservations:   normalizedObservations,
	}

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
