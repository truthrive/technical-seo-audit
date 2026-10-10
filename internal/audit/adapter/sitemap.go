package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

// rawSitemapDoc holds raw document evidence from sitecrawl_sitemaps.
type rawSitemapDoc struct {
	id              int
	url             string
	discoverySource string
	parentID        int
	initialStatus   int
	finalStatus     int
	status          int
	fetchError      string
	redirectTo      string
	redirectHops    int
	fetchComplete   bool
	docType         string
	parseStatus     string
	parseError      string
	entryCount      int
	fetchedAt       string
}

// rawSitemapSource holds raw multi-source provenance from sitecrawl_sitemap_sources.
type rawSitemapSource struct {
	sitemapID int
	source    string
	parentID  int
}

// rawSitemapEntry holds raw entry evidence from sitecrawl_sitemap_entries.
type rawSitemapEntry struct {
	sitemapID  int
	seq        int
	urlID      int64
	loc        string
	lastMod    string
	changeFreq string
	priority   string
}

// rawSitemapDiscovery holds run-level acquisition telemetry from sitecrawl_sitemap_discovery.
type rawSitemapDiscovery struct {
	status        string
	sitemapsFound int
	entriesFound  int
	depthReached  int
	depthCapped   bool
	urlsCapped    bool
	byteCapped    bool
	stopReason    string
	diagnostics   string
	startedAt     string
	finishedAt    string
}

// sitemapRawEvidence bundles all raw persisted sitemap evidence for a crawl run.
type sitemapRawEvidence struct {
	isLegacy             bool
	hasDiscoveryRecord   bool
	discovery            rawSitemapDiscovery
	sitemaps             []rawSitemapDoc
	sources              []rawSitemapSource
	entries              []rawSitemapEntry
}

// loadSitemapRawEvidence queries the 4 sitemap tables from SQLite within the read transaction.
// If the tables do not exist (legacy runs), it returns an evidence bundle with isLegacy = true.
func loadSitemapRawEvidence(ctx context.Context, tx *sql.Tx, crawlRunID string) (*sitemapRawEvidence, error) {
	// 1. Check if sitecrawl_sitemaps table exists in SQLite schema
	var tableExists int
	err := tx.QueryRowContext(ctx,
		`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='sitecrawl_sitemaps'`).Scan(&tableExists)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: check sitemap table existence: %w", err)
	}
	if tableExists == 0 {
		return &sitemapRawEvidence{isLegacy: true}, nil
	}

	result := &sitemapRawEvidence{}

	// 2. Query sitecrawl_sitemap_discovery
	var (
		discStatus, stopReason, diagnostics, startedAt, finishedAt string
		sitemapsFound, entriesFound, depthReached                  int
		depthCapped, urlsCapped, byteCapped                        int
	)
	row := tx.QueryRowContext(ctx,
		`SELECT status, sitemaps_found, entries_found, depth_reached,
		        depth_capped, urls_capped, byte_capped, stop_reason,
		        diagnostics, started_at, finished_at
		 FROM sitecrawl_sitemap_discovery WHERE run_id = ?`, crawlRunID)
	err = row.Scan(
		&discStatus, &sitemapsFound, &entriesFound, &depthReached,
		&depthCapped, &urlsCapped, &byteCapped, &stopReason,
		&diagnostics, &startedAt, &finishedAt,
	)
	if err == sql.ErrNoRows {
		result.hasDiscoveryRecord = false
	} else if err != nil {
		return nil, fmt.Errorf("audit adapter: query sitemap discovery: %w", err)
	} else {
		result.hasDiscoveryRecord = true
		result.discovery = rawSitemapDiscovery{
			status:        discStatus,
			sitemapsFound: sitemapsFound,
			entriesFound:  entriesFound,
			depthReached:  depthReached,
			depthCapped:   depthCapped != 0,
			urlsCapped:    urlsCapped != 0,
			byteCapped:    byteCapped != 0,
			stopReason:    stopReason,
			diagnostics:   diagnostics,
			startedAt:     startedAt,
			finishedAt:    finishedAt,
		}
	}

	// 3. Query sitecrawl_sitemaps in deterministic order (ORDER BY id ASC)
	smRows, err := tx.QueryContext(ctx,
		`SELECT id, url, discovery_source, parent_id, initial_status,
		        final_status, status, fetch_error, redirect_to, redirect_hops,
		        fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at
		 FROM sitecrawl_sitemaps WHERE run_id = ? ORDER BY id ASC`, crawlRunID)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: query sitemaps: %w", err)
	}
	defer smRows.Close()

	for smRows.Next() {
		var (
			doc rawSitemapDoc
			fc  int
		)
		if err := smRows.Scan(
			&doc.id, &doc.url, &doc.discoverySource, &doc.parentID,
			&doc.initialStatus, &doc.finalStatus, &doc.status, &doc.fetchError,
			&doc.redirectTo, &doc.redirectHops, &fc, &doc.docType,
			&doc.parseStatus, &doc.parseError, &doc.entryCount, &doc.fetchedAt,
		); err != nil {
			return nil, fmt.Errorf("audit adapter: scan sitemap doc: %w", err)
		}
		doc.fetchComplete = (fc != 0)
		result.sitemaps = append(result.sitemaps, doc)
	}
	if err := smRows.Err(); err != nil {
		return nil, fmt.Errorf("audit adapter: read sitemaps: %w", err)
	}

	// 4. Query sitecrawl_sitemap_sources in deterministic order (ORDER BY sitemap_id ASC, source ASC, parent_id ASC)
	srcRows, err := tx.QueryContext(ctx,
		`SELECT sitemap_id, source, parent_id
		 FROM sitecrawl_sitemap_sources WHERE run_id = ?
		 ORDER BY sitemap_id ASC, source ASC, parent_id ASC`, crawlRunID)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: query sitemap sources: %w", err)
	}
	defer srcRows.Close()

	for srcRows.Next() {
		var src rawSitemapSource
		if err := srcRows.Scan(&src.sitemapID, &src.source, &src.parentID); err != nil {
			return nil, fmt.Errorf("audit adapter: scan sitemap source: %w", err)
		}
		result.sources = append(result.sources, src)
	}
	if err := srcRows.Err(); err != nil {
		return nil, fmt.Errorf("audit adapter: read sitemap sources: %w", err)
	}

	// 5. Query sitecrawl_sitemap_entries in deterministic order (ORDER BY sitemap_id ASC, seq ASC)
	entRows, err := tx.QueryContext(ctx,
		`SELECT sitemap_id, seq, url_id, loc, lastmod, changefreq, priority
		 FROM sitecrawl_sitemap_entries WHERE run_id = ?
		 ORDER BY sitemap_id ASC, seq ASC`, crawlRunID)
	if err != nil {
		return nil, fmt.Errorf("audit adapter: query sitemap entries: %w", err)
	}
	defer entRows.Close()

	for entRows.Next() {
		var ent rawSitemapEntry
		if err := entRows.Scan(
			&ent.sitemapID, &ent.seq, &ent.urlID, &ent.loc,
			&ent.lastMod, &ent.changeFreq, &ent.priority,
		); err != nil {
			return nil, fmt.Errorf("audit adapter: scan sitemap entry: %w", err)
		}
		result.entries = append(result.entries, ent)
	}
	if err := entRows.Err(); err != nil {
		return nil, fmt.Errorf("audit adapter: read sitemap entries: %w", err)
	}

	return result, nil
}

// buildSitemapEvidence normalizes persisted sitemap acquisition evidence into typed
// observations, subject entities, and deterministic EvidenceSnapshot metadata.
func buildSitemapEvidence(
	req BuildRequest,
	raw *sitemapRawEvidence,
	urlsByID map[int]string,
	urlIDs []int,
	pagesByURLID map[int]*rawPageRecord,
	runStartedAt time.Time,
	nextObsID func() audit.ObservationID,
) (
	sitemapObs []audit.SitemapObservation,
	sitemapEntries []audit.SitemapEntry,
	normalizedObs []audit.NormalizedObservation,
	gaps []EvidenceGap,
	sitemapDiscoveryComplete bool,
) {
	// A. Legacy crawl with no persisted sitemap evidence tables
	if raw == nil || raw.isLegacy {
		gaps = append(gaps, EvidenceGap{
			GapCode:         GapSitemapDocumentUnavailable,
			Field:           "sitemap_observation",
			Reason:          "Legacy crawl run lacks sitemap evidence tables or records; sitemap acquisition was not performed under the V1.8+ evidence schema.",
			SourceComponent: "sitecrawl_sitemaps",
		})
		return nil, nil, nil, gaps, false
	}

	// B. Run-level discovery completeness and site observations
	disc := raw.discovery
	if !raw.hasDiscoveryRecord {
		gaps = append(gaps, EvidenceGap{
			GapCode:         GapSitemapMetadataInconsistent,
			Field:           "sitecrawl_sitemap_discovery",
			Reason:          "Run lacks a sitemap discovery telemetry record in sitecrawl_sitemap_discovery.",
			SourceComponent: "sitecrawl_sitemap_discovery",
		})
	} else {
		// SitemapDiscoveryComplete is true ONLY when acquisition reached COMPLETED without hitting caps
		sitemapDiscoveryComplete = (disc.status == "COMPLETED" && !disc.depthCapped && !disc.urlsCapped && !disc.byteCapped)

		var discObsTime time.Time
		if disc.startedAt != "" {
			if t, err := time.Parse(time.RFC3339, disc.startedAt); err == nil {
				discObsTime = t
			}
		}
		if discObsTime.IsZero() {
			discObsTime = runStartedAt
		}

		siteSrcRef := fmt.Sprintf("sitecrawl_sitemap_discovery:%s", req.CrawlRunID)

		// Emit Site-level normalized observations
		normalizedObs = append(normalizedObs,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemap_discovery_status",
				Value:              disc.status,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemap_discovery_complete",
				Value:              strconv.FormatBool(sitemapDiscoveryComplete),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemaps_found_count",
				Value:              strconv.Itoa(disc.sitemapsFound),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemap_entries_found_count",
				Value:              strconv.Itoa(disc.entriesFound),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemap_depth_capped",
				Value:              strconv.FormatBool(disc.depthCapped),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemap_urls_capped",
				Value:              strconv.FormatBool(disc.urlsCapped),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemap_byte_capped",
				Value:              strconv.FormatBool(disc.byteCapped),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			},
		)

		if disc.stopReason != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemap_discovery_stop_reason",
				Value:              disc.stopReason,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			})
		}

		if disc.diagnostics != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSite,
				SubjectRef:         "site",
				Field:              "sitemap_discovery_diagnostics",
				Value:              disc.diagnostics,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{siteSrcRef},
				ObservedAt:         discObsTime,
			})
		}

		// Emit discovery-level evidence gaps
		if disc.status == "NOT_ATTEMPTED" {
			gaps = append(gaps, EvidenceGap{
				GapCode:         GapSitemapDiscoveryDisabled,
				Field:           "sitemap_discovery",
				Reason:          "Sitemap discovery was disabled in crawl options (DiscoverSitemaps=false); sitemap presence or absence cannot be determined from crawler evidence.",
				SourceComponent: "sitecrawl",
			})
		} else if disc.status == "ATTEMPTED_INCOMPLETE" {
			gaps = append(gaps, EvidenceGap{
				GapCode:         GapSitemapDiscoveryIncomplete,
				Field:           "sitemap_discovery",
				Reason:          fmt.Sprintf("Sitemap discovery was incomplete (status: %s, stop_reason: %q); sitemap evidence coverage is partial.", disc.status, disc.stopReason),
				SourceComponent: "sitecrawl_sitemap_discovery",
			})
		}

		if disc.depthCapped || disc.urlsCapped || disc.byteCapped {
			gaps = append(gaps, EvidenceGap{
				GapCode:         GapSitemapTruncated,
				Field:           "sitemap_discovery_caps",
				Reason:          fmt.Sprintf("Sitemap acquisition hit safety limits (depth_capped=%t, urls_capped=%t, byte_capped=%t); complete tree may not be reflected.", disc.depthCapped, disc.urlsCapped, disc.byteCapped),
				SourceComponent: "sitecrawl_sitemap_discovery",
			})
		}

		if disc.status == "COMPLETED" && len(raw.sitemaps) == 0 {
			gaps = append(gaps, EvidenceGap{
				GapCode:         GapSitemapDocumentUnavailable,
				Field:           "sitemap_observation",
				Reason:          "Completed sitemap discovery found 0 sitemap documents on tested robots.txt and common paths.",
				SourceComponent: "sitecrawl_sitemaps",
			})
		}
	}

	// C. Index raw sources by sitemap ID
	sourcesByDocID := make(map[int][]rawSitemapSource)
	for _, src := range raw.sources {
		sourcesByDocID[src.sitemapID] = append(sourcesByDocID[src.sitemapID], src)
	}

	// Index sitemap documents by ID
	sitemapsByID := make(map[int]*rawSitemapDoc)
	for i := range raw.sitemaps {
		doc := &raw.sitemaps[i]
		sitemapsByID[doc.id] = doc
	}

	// Invert dictionary for fast URL ID lookup
	idsByURL := make(map[string]int, len(urlsByID))
	for id, u := range urlsByID {
		idsByURL[u] = id
	}

	// D. Normalize sitemap documents (SubjectSitemap)
	for _, doc := range raw.sitemaps {
		smIDRef := fmt.Sprintf("sm:%s:%d", req.AuditRunID, doc.id)
		smSrcRef := fmt.Sprintf("sitecrawl_sitemaps:%s:%d", req.CrawlRunID, doc.id)

		var obsTime time.Time
		if doc.fetchedAt != "" {
			if t, err := time.Parse(time.RFC3339, doc.fetchedAt); err == nil {
				obsTime = t
			}
		}
		if obsTime.IsZero() {
			obsTime = runStartedAt
		}

		// Map document type
		var docType audit.SitemapDocumentType
		switch strings.ToLower(doc.docType) {
		case "urlset":
			docType = audit.SitemapDocumentURLSet
		case "sitemapindex":
			docType = audit.SitemapDocumentSitemapIndex
		default:
			docType = audit.SitemapDocumentUnknown
		}

		// Optional URLID resolution
		var sitemapURLID audit.URLID
		if dictID, ok := idsByURL[doc.url]; ok && dictID > 0 {
			sitemapURLID = audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, dictID))
		}

		// Construct typed SitemapObservation
		so := audit.SitemapObservation{
			SitemapID:      audit.SitemapID(smIDRef),
			AuditRunID:     req.AuditRunID,
			SitemapURLID:   sitemapURLID,
			FetchID:        audit.FetchID(fmt.Sprintf("fetch:sitemap:%s:%d", req.AuditRunID, doc.id)),
			DocumentType:   docType,
			ParseStatus:    doc.parseStatus,
			ParseError:     doc.parseError,
			RawArtifactRef: smSrcRef,
			ObservedAt:     obsTime,
		}
		sitemapObs = append(sitemapObs, so)

		// Resolve multi-source provenance
		var distinctSources []string
		seenSrc := make(map[string]bool)
		for _, s := range sourcesByDocID[doc.id] {
			if !seenSrc[s.source] {
				seenSrc[s.source] = true
				distinctSources = append(distinctSources, s.source)
			}
		}
		sort.Strings(distinctSources)

		// Validate parent sitemap reference if parent_id > 0
		var parentRef string
		if doc.parentID > 0 {
			if _, exists := sitemapsByID[doc.parentID]; exists {
				parentRef = fmt.Sprintf("sm:%s:%d", req.AuditRunID, doc.parentID)
			} else {
				gaps = append(gaps, EvidenceGap{
					GapCode:         GapSitemapMetadataInconsistent,
					SubjectRef:      smIDRef,
					Field:           "parent_id",
					Reason:          fmt.Sprintf("Sitemap document %d references parent sitemap ID %d which does not exist in the crawl run.", doc.id, doc.parentID),
					SourceComponent: "sitecrawl_sitemaps",
				})
			}
		}

		// Emit document observations
		normalizedObs = append(normalizedObs,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_id",
				Value:              smIDRef,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_url",
				Value:              doc.url,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "discovery_source",
				Value:              doc.discoverySource,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
		)

		if len(distinctSources) > 0 {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "discovery_sources",
				Value:              strings.Join(distinctSources, ","),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			})
		}

		if parentRef != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "parent_sitemap_ref",
				Value:              parentRef,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			})
		}

		// Independent HTTP statuses and redirect provenance
		normalizedObs = append(normalizedObs,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "initial_http_status",
				Value:              strconv.Itoa(doc.initialStatus),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "final_http_status",
				Value:              strconv.Itoa(doc.finalStatus),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_fetch_status",
				Value:              strconv.Itoa(doc.status),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_redirect_hops",
				Value:              strconv.Itoa(doc.redirectHops),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
		)

		if doc.redirectTo != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_final_url",
				Value:              doc.redirectTo,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			})
		}

		if doc.fetchError != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_fetch_error",
				Value:              doc.fetchError,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			})
		}

		normalizedObs = append(normalizedObs,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_fetch_complete",
				Value:              strconv.FormatBool(doc.fetchComplete),
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_document_type",
				Value:              string(docType),
				DerivationType:     audit.DerivationDerived,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_parse_status",
				Value:              doc.parseStatus,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			},
		)

		if doc.parseError != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "sitemap_parse_error",
				Value:              doc.parseError,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			})
		}

		normalizedObs = append(normalizedObs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectSitemap,
			SubjectRef:         smIDRef,
			Field:              "sitemap_entry_count",
			Value:              strconv.Itoa(doc.entryCount),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{smSrcRef},
			ObservedAt:         obsTime,
		})

		if sitemapURLID != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemap,
				SubjectRef:         smIDRef,
				Field:              "url_subject_ref",
				Value:              string(sitemapURLID),
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{smSrcRef},
				ObservedAt:         obsTime,
			})
		}

		// Emit document-level fetch and parse gaps
		if doc.fetchError != "" || (!doc.fetchComplete && doc.initialStatus == 0) {
			gaps = append(gaps, EvidenceGap{
				GapCode:         GapSitemapFetchFailed,
				SubjectRef:      smIDRef,
				Field:           "fetch_error",
				Reason:          fmt.Sprintf("Sitemap document fetch failed: %s", doc.fetchError),
				SourceComponent: "sitecrawl_sitemaps",
			})
		}
		if doc.parseStatus == "xml_error" || doc.parseStatus == "unsupported_structure" || doc.parseStatus == "empty_input" {
			gaps = append(gaps, EvidenceGap{
				GapCode:         GapSitemapParseFailed,
				SubjectRef:      smIDRef,
				Field:           "parse_status",
				Reason:          fmt.Sprintf("Sitemap XML parse failed with status %q: %s", doc.parseStatus, doc.parseError),
				SourceComponent: "sitecrawl_sitemaps",
			})
		}
	}

	// E. Normalize sitemap entries (SubjectSitemapEntry) and prepare bilateral URL links
	type entryCorrelation struct {
		entryRef  string
		docRef    string
		docURL    string
		entrySeq  int
		sitemapID int
	}
	correlationsByURLID := make(map[int][]entryCorrelation)

	for _, entry := range raw.entries {
		smeIDRef := fmt.Sprintf("sme:%s:%d:%d", req.AuditRunID, entry.sitemapID, entry.seq)
		entSrcRef := fmt.Sprintf("sitecrawl_sitemap_entries:%s:%d:%d", req.CrawlRunID, entry.sitemapID, entry.seq)

		parentDoc, parentExists := sitemapsByID[entry.sitemapID]
		var (
			parentRef string
			obsTime   time.Time
		)
		if parentExists {
			parentRef = fmt.Sprintf("sm:%s:%d", req.AuditRunID, entry.sitemapID)
			if parentDoc.fetchedAt != "" {
				if t, err := time.Parse(time.RFC3339, parentDoc.fetchedAt); err == nil {
					obsTime = t
				}
			}
		} else {
			// Orphan sitemap entry referencing missing document
			gaps = append(gaps, EvidenceGap{
				GapCode:         GapOrphanSitemapEntry,
				SubjectRef:      smeIDRef,
				Field:           "parent_sitemap_id",
				Reason:          fmt.Sprintf("Sitemap entry at seq %d references non-existent parent sitemap id %d.", entry.seq, entry.sitemapID),
				SourceComponent: "sitecrawl_sitemap_entries",
			})
		}
		if obsTime.IsZero() {
			obsTime = runStartedAt
		}

		// URL normalization on entry.loc
		normListedURL, _, normErr := normalizeURL(entry.loc)

		// Correlate with URL dictionary record under Section 9 integrity rules
		var targetURLSubjectRef audit.URLID
		if entry.urlID > 0 {
			dictURL, existsInDict := urlsByID[int(entry.urlID)]
			if !existsInDict {
				// Dangling URL dictionary ID
				gaps = append(gaps, EvidenceGap{
					GapCode:         GapInvalidSitemapURLCorrelation,
					SubjectRef:      smeIDRef,
					Field:           "url_id",
					Reason:          fmt.Sprintf("Sitemap entry loc %q has url_id %d which does not exist in URL dictionary.", entry.loc, entry.urlID),
					SourceComponent: "sitecrawl_sitemap_entries",
				})
			} else {
				// Verify listed URL and dictionary target identity consistency
				normDict, _, normDictErr := normalizeURL(dictURL)
				match := false
				if entry.loc == dictURL {
					match = true
				} else if normErr == nil && normDictErr == nil && normListedURL == normDict {
					match = true
				}

				if !match {
					// Contradictory correlation
					gaps = append(gaps, EvidenceGap{
						GapCode:         GapInvalidSitemapURLCorrelation,
						SubjectRef:      smeIDRef,
						Field:           "url_subject_ref",
						Reason:          fmt.Sprintf("Sitemap entry loc %q has url_id %d pointing to dictionary URL %q which does not match listed URL under normalization.", entry.loc, entry.urlID, dictURL),
						SourceComponent: "sitecrawl_sitemap_entries",
					})
				} else {
					targetURLSubjectRef = audit.URLID(fmt.Sprintf("url:%s:%d", req.AuditRunID, entry.urlID))
					if parentExists {
						correlationsByURLID[int(entry.urlID)] = append(correlationsByURLID[int(entry.urlID)], entryCorrelation{
							entryRef:  smeIDRef,
							docRef:    parentRef,
							docURL:    parentDoc.url,
							entrySeq:  entry.seq,
							sitemapID: entry.sitemapID,
						})
					}
				}
			}
		}

		// Parse lastmod if present
		var parsedLastmod *time.Time
		if entry.lastMod != "" {
			if t, err := time.Parse(time.RFC3339, entry.lastMod); err == nil {
				parsedLastmod = &t
			} else if t, err := time.Parse("2006-01-02", entry.lastMod); err == nil {
				parsedLastmod = &t
			}
		}

		// Construct typed SitemapEntry
		se := audit.SitemapEntry{
			SitemapEntryID:    audit.SitemapEntryID(smeIDRef),
			AuditRunID:        req.AuditRunID,
			SitemapID:         audit.SitemapID(parentRef),
			ListedURLID:       targetURLSubjectRef,
			ListedURLRaw:      entry.loc,
			LastmodRaw:        entry.lastMod,
			LastmodNormalized: parsedLastmod,
			ObservedAt:        obsTime,
		}
		sitemapEntries = append(sitemapEntries, se)

		// Emit SubjectSitemapEntry observations
		normalizedObs = append(normalizedObs,
			audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         smeIDRef,
				Field:              "sitemap_entry_id",
				Value:              smeIDRef,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{entSrcRef},
				ObservedAt:         obsTime,
			},
		)

		if parentRef != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         smeIDRef,
				Field:              "parent_sitemap_ref",
				Value:              parentRef,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{entSrcRef},
				ObservedAt:         obsTime,
			})
		}

		normalizedObs = append(normalizedObs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectSitemapEntry,
			SubjectRef:         smeIDRef,
			Field:              "loc_raw",
			Value:              entry.loc,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{entSrcRef},
			ObservedAt:         obsTime,
		})

		if normErr == nil {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         smeIDRef,
				Field:              "loc_normalized",
				Value:              normListedURL,
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{entSrcRef},
				ObservedAt:         obsTime,
			})
		}

		normalizedObs = append(normalizedObs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectSitemapEntry,
			SubjectRef:         smeIDRef,
			Field:              "entry_seq",
			Value:              strconv.Itoa(entry.seq),
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{entSrcRef},
			ObservedAt:         obsTime,
		})

		if entry.lastMod != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         smeIDRef,
				Field:              "lastmod_raw",
				Value:              entry.lastMod,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{entSrcRef},
				ObservedAt:         obsTime,
			})
		}

		if entry.changeFreq != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         smeIDRef,
				Field:              "changefreq_raw",
				Value:              entry.changeFreq,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{entSrcRef},
				ObservedAt:         obsTime,
			})
		}

		if entry.priority != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         smeIDRef,
				Field:              "priority_raw",
				Value:              entry.priority,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{entSrcRef},
				ObservedAt:         obsTime,
			})
		}

		if targetURLSubjectRef != "" {
			normalizedObs = append(normalizedObs, audit.NormalizedObservation{
				ObservationID:      nextObsID(),
				AuditRunID:         req.AuditRunID,
				SnapshotID:         req.SnapshotID,
				SubjectType:        audit.SubjectSitemapEntry,
				SubjectRef:         smeIDRef,
				Field:              "url_subject_ref",
				Value:              string(targetURLSubjectRef),
				DerivationType:     audit.DerivationNormalized,
				SourceEvidenceRefs: []string{entSrcRef},
				ObservedAt:         obsTime,
			})
		}
	}

	// F. Emit bilateral observations on SubjectURL in deterministic sorted urlID order
	for _, urlID := range urlIDs {
		corrs, ok := correlationsByURLID[urlID]
		if !ok || len(corrs) == 0 {
			continue
		}

		urlSubjRef := fmt.Sprintf("url:%s:%d", req.AuditRunID, urlID)

		var obsTime time.Time
		if p, ok := pagesByURLID[urlID]; ok && p.crawledAt != "" {
			if t, err := time.Parse(time.RFC3339, p.crawledAt); err == nil {
				obsTime = t
			}
		}
		if obsTime.IsZero() {
			obsTime = runStartedAt
		}

		// Emit listed_in_sitemap = "true" once per correlated URL subject
		normalizedObs = append(normalizedObs, audit.NormalizedObservation{
			ObservationID:      nextObsID(),
			AuditRunID:         req.AuditRunID,
			SnapshotID:         req.SnapshotID,
			SubjectType:        audit.SubjectURL,
			SubjectRef:         urlSubjRef,
			Field:              "listed_in_sitemap",
			Value:              "true",
			DerivationType:     audit.DerivationNormalized,
			SourceEvidenceRefs: []string{fmt.Sprintf("sitecrawl_urls:%s:%d", req.CrawlRunID, urlID)},
			ObservedAt:         obsTime,
		})

		for _, corr := range corrs {
			entSrcRef := fmt.Sprintf("sitecrawl_sitemap_entries:%s:%d:%d", req.CrawlRunID, corr.sitemapID, corr.entrySeq)
			normalizedObs = append(normalizedObs,
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         urlSubjRef,
					Field:              "sitemap_entry_ref",
					Value:              corr.entryRef,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{entSrcRef},
					ObservedAt:         obsTime,
				},
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         urlSubjRef,
					Field:              "sitemap_document_ref",
					Value:              corr.docRef,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{entSrcRef},
					ObservedAt:         obsTime,
				},
				audit.NormalizedObservation{
					ObservationID:      nextObsID(),
					AuditRunID:         req.AuditRunID,
					SnapshotID:         req.SnapshotID,
					SubjectType:        audit.SubjectURL,
					SubjectRef:         urlSubjRef,
					Field:              "sitemap_url",
					Value:              corr.docURL,
					DerivationType:     audit.DerivationNormalized,
					SourceEvidenceRefs: []string{entSrcRef},
					ObservedAt:         obsTime,
				},
			)
		}
	}

	return sitemapObs, sitemapEntries, normalizedObs, gaps, sitemapDiscoveryComplete
}
