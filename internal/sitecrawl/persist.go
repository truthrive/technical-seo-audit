package sitecrawl

import (
	"database/sql"
	"fmt"
	"strings"
)

// Batch sizes. One statement per batch keeps well under SQLite's bind-variable
// limit while not paying a round trip per row. Pages carry ~45 columns so their
// batch is smaller than the link batch by design.
const (
	pageBatch = 100
	linkBatch = 400
	urlBatch  = 500
)

// writeURLs flushes new dictionary entries.
func writeURLs(db *sql.DB, runID string, entries []urlEntry) error {
	return chunked(len(entries), urlBatch, func(lo, hi int) error {
		var sb strings.Builder
		args := make([]any, 0, (hi-lo)*3)
		sb.WriteString(`INSERT OR IGNORE INTO sitecrawl_urls(run_id, id, url) VALUES `)
		for i := lo; i < hi; i++ {
			if i > lo {
				sb.WriteByte(',')
			}
			sb.WriteString("(?,?,?)")
			args = append(args, runID, entries[i].ID, entries[i].URL)
		}
		_, err := db.Exec(sb.String(), args...)
		return err
	})
}

const pageColumns = `run_id, url_id, url, data, kind, is_internal, depth, discovered_by,
	status, status_class, content_type, size_bytes, response_ms, redirect_to, redirect_hops,
	error_type, title, title_len, meta_desc, meta_desc_len, h1, h1_len, h1_count, h2, h2_count,
	word_count, text_ratio, lang, canonical, meta_robots, x_robots, indexable, indexability,
	robots_state, last_mod, outlinks, outlinks_ext, images_count, images_noalt,
	issue_count, issue_max_sev, rendered, crawled_at`

const pagePlaceholders = `(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`

// writePages upserts crawled pages.
//
// An upsert rather than a plain insert so a recrawl of the same run overwrites
// cleanly, and so a resumed run that re-fetches a URL it already had does not
// fail on the primary key.
func writePages(db *sql.DB, runID string, rows []pageRow) error {
	return chunked(len(rows), pageBatch, func(lo, hi int) error {
		var sb strings.Builder
		args := make([]any, 0, (hi-lo)*43)
		sb.WriteString(`INSERT INTO sitecrawl_pages(` + pageColumns + `) VALUES `)
		for i := lo; i < hi; i++ {
			if i > lo {
				sb.WriteByte(',')
			}
			sb.WriteString(pagePlaceholders)
			r := rows[i]
			args = append(args,
				runID, r.URLID, r.URL, r.Data, r.Kind, r.IsInternal, r.Depth, r.DiscoveredBy,
				r.Status, r.StatusClass, r.ContentType, r.SizeBytes, r.ResponseMs, r.RedirectTo, r.RedirectHops,
				r.ErrorType, r.Title, r.TitleLen, r.MetaDesc, r.MetaDescLen, r.H1, r.H1Len, r.H1Count, r.H2, r.H2Count,
				r.WordCount, r.TextRatio, r.Lang, r.Canonical, r.MetaRobots, r.XRobots, r.Indexable, r.Indexability,
				r.RobotsState, r.LastMod, r.Outlinks, r.OutlinksExt, r.ImagesCount, r.ImagesNoAlt,
				r.IssueCount, r.IssueMaxSev, r.Rendered, r.CrawledAt)
		}
		sb.WriteString(` ON CONFLICT(run_id, url_id) DO UPDATE SET
			data = excluded.data, kind = excluded.kind, is_internal = excluded.is_internal,
			depth = excluded.depth, discovered_by = excluded.discovered_by,
			status = excluded.status, status_class = excluded.status_class,
			content_type = excluded.content_type, size_bytes = excluded.size_bytes,
			response_ms = excluded.response_ms, redirect_to = excluded.redirect_to,
			redirect_hops = excluded.redirect_hops, error_type = excluded.error_type,
			title = excluded.title, title_len = excluded.title_len,
			meta_desc = excluded.meta_desc, meta_desc_len = excluded.meta_desc_len,
			h1 = excluded.h1, h1_len = excluded.h1_len, h1_count = excluded.h1_count,
			h2 = excluded.h2, h2_count = excluded.h2_count, word_count = excluded.word_count,
			text_ratio = excluded.text_ratio, lang = excluded.lang, canonical = excluded.canonical,
			meta_robots = excluded.meta_robots, x_robots = excluded.x_robots,
			indexable = excluded.indexable, indexability = excluded.indexability,
			robots_state = excluded.robots_state, last_mod = excluded.last_mod,
			outlinks = excluded.outlinks, outlinks_ext = excluded.outlinks_ext,
			images_count = excluded.images_count, images_noalt = excluded.images_noalt,
			issue_count = excluded.issue_count, issue_max_sev = excluded.issue_max_sev,
			rendered = excluded.rendered, crawled_at = excluded.crawled_at`)
		_, err := db.Exec(sb.String(), args...)
		return err
	})
}

// writeLinks appends link edges between URLs.
func writeLinks(db *sql.DB, runID string, edges []edge) error {
	return chunked(len(edges), linkBatch, func(lo, hi int) error {
		var sb strings.Builder
		args := make([]any, 0, (hi-lo)*7)
		sb.WriteString(`INSERT INTO sitecrawl_links(run_id, src_id, dst_id, seq, placement, flags, anchor) VALUES `)
		for i := lo; i < hi; i++ {
			if i > lo {
				sb.WriteByte(',')
			}
			sb.WriteString("(?,?,?,?,?,?,?)")
			e := edges[i]
			args = append(args, runID, e.SrcID, e.DstID, e.Seq, e.Placement, e.Flags, e.Anchor)
		}
		_, err := db.Exec(sb.String(), args...)
		return err
	})
}

// chunked runs fn over [0,n) in slices of at most size.
func chunked(n, size int, fn func(lo, hi int) error) error {
	for lo := 0; lo < n; lo += size {
		hi := lo + size
		if hi > n {
			hi = n
		}
		if err := fn(lo, hi); err != nil {
			return err
		}
	}
	return nil
}

// sitemapRecord holds raw sitemap document evidence to persist.
type sitemapRecord struct {
	ID              int
	URL             string
	DiscoverySource string
	ParentID        int
	InitialStatus   int
	FinalStatus     int
	Status          int
	FetchError      string
	RedirectTo      string
	RedirectHops    int
	FetchComplete   bool
	DocType         string
	ParseStatus     string
	ParseError      string
	EntryCount      int
	FetchedAt       string
}

// sitemapSourceRecord preserves multi-source and parent discovery provenance.
type sitemapSourceRecord struct {
	SitemapID int
	Source    string
	ParentID  int
}

// sitemapEntryRecord holds raw sitemap URL entry evidence.
type sitemapEntryRecord struct {
	SitemapID  int
	Seq        int
	URLID      int64
	Loc        string
	LastMod    string
	ChangeFreq string
	Priority   string
}

// sitemapDiscoveryRecord tracks run-level acquisition telemetry and limits.
type sitemapDiscoveryRecord struct {
	RunID         string
	Status        string // NOT_ATTEMPTED, ATTEMPTED_INCOMPLETE, COMPLETED, FAILED, UNKNOWN
	SitemapsFound int
	EntriesFound  int
	DepthReached  int
	DepthCapped   bool
	URLsCapped    bool
	ByteCapped    bool
	StopReason    string
	Diagnostics   string
	StartedAt     string
	FinishedAt    string
}

// sitemapEvidence bundles complete sitemap acquisition evidence for transactional persistence.
type sitemapEvidence struct {
	Discovery sitemapDiscoveryRecord
	Sitemaps  []sitemapRecord
	Sources   []sitemapSourceRecord
	Entries   []sitemapEntryRecord
}

// writeSitemapDiscovery stores run-level sitemap acquisition telemetry.
func writeSitemapDiscovery(db *sql.DB, runID string, disc sitemapDiscoveryRecord) error {
	if db == nil {
		return nil
	}
	var depthCapped, urlsCapped, byteCapped int
	if disc.DepthCapped {
		depthCapped = 1
	}
	if disc.URLsCapped {
		urlsCapped = 1
	}
	if disc.ByteCapped {
		byteCapped = 1
	}
	_, err := db.Exec(`INSERT OR REPLACE INTO sitecrawl_sitemap_discovery(
		run_id, status, sitemaps_found, entries_found, depth_reached,
		depth_capped, urls_capped, byte_capped, stop_reason, diagnostics,
		started_at, finished_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		runID, disc.Status, disc.SitemapsFound, disc.EntriesFound,
		disc.DepthReached, depthCapped, urlsCapped, byteCapped,
		disc.StopReason, disc.Diagnostics, disc.StartedAt, disc.FinishedAt)
	return err
}

// writeSitemapEvidence writes sitemap discovery, documents, sources, and entries in an atomic transaction.
func writeSitemapEvidence(db *sql.DB, runID string, ev sitemapEvidence) error {
	if db == nil {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Insert sitemap discovery record
	var depthCapped, urlsCapped, byteCapped int
	if ev.Discovery.DepthCapped {
		depthCapped = 1
	}
	if ev.Discovery.URLsCapped {
		urlsCapped = 1
	}
	if ev.Discovery.ByteCapped {
		byteCapped = 1
	}
	_, err = tx.Exec(`INSERT OR REPLACE INTO sitecrawl_sitemap_discovery(
		run_id, status, sitemaps_found, entries_found, depth_reached,
		depth_capped, urls_capped, byte_capped, stop_reason, diagnostics,
		started_at, finished_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		runID, ev.Discovery.Status, ev.Discovery.SitemapsFound, ev.Discovery.EntriesFound,
		ev.Discovery.DepthReached, depthCapped, urlsCapped, byteCapped,
		ev.Discovery.StopReason, ev.Discovery.Diagnostics,
		ev.Discovery.StartedAt, ev.Discovery.FinishedAt)
	if err != nil {
		return fmt.Errorf("sitecrawl: insert sitemap discovery: %w", err)
	}

	// 2. Insert sitemap documents in batches of 50
	const smBatch = 50
	for lo := 0; lo < len(ev.Sitemaps); lo += smBatch {
		hi := lo + smBatch
		if hi > len(ev.Sitemaps) {
			hi = len(ev.Sitemaps)
		}
		var sb strings.Builder
		args := make([]any, 0, (hi-lo)*17)
		sb.WriteString(`INSERT OR REPLACE INTO sitecrawl_sitemaps(
			run_id, id, url, discovery_source, parent_id, initial_status,
			final_status, status, fetch_error, redirect_to, redirect_hops,
			fetch_complete, doc_type, parse_status, parse_error, entry_count, fetched_at) VALUES `)
		for i := lo; i < hi; i++ {
			if i > lo {
				sb.WriteByte(',')
			}
			sb.WriteString("(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)")
			sm := ev.Sitemaps[i]
			fc := 0
			if sm.FetchComplete {
				fc = 1
			}
			args = append(args, runID, sm.ID, sm.URL, sm.DiscoverySource, sm.ParentID,
				sm.InitialStatus, sm.FinalStatus, sm.Status, sm.FetchError, sm.RedirectTo,
				sm.RedirectHops, fc, sm.DocType, sm.ParseStatus, sm.ParseError,
				sm.EntryCount, sm.FetchedAt)
		}
		if _, err := tx.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("sitecrawl: insert sitemaps batch: %w", err)
		}
	}

	// 3. Insert sitemap sources in batches of 100
	const srcBatch = 100
	for lo := 0; lo < len(ev.Sources); lo += srcBatch {
		hi := lo + srcBatch
		if hi > len(ev.Sources) {
			hi = len(ev.Sources)
		}
		var sb strings.Builder
		args := make([]any, 0, (hi-lo)*4)
		sb.WriteString(`INSERT OR IGNORE INTO sitecrawl_sitemap_sources(run_id, sitemap_id, source, parent_id) VALUES `)
		for i := lo; i < hi; i++ {
			if i > lo {
				sb.WriteByte(',')
			}
			sb.WriteString("(?,?,?,?)")
			s := ev.Sources[i]
			args = append(args, runID, s.SitemapID, s.Source, s.ParentID)
		}
		if _, err := tx.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("sitecrawl: insert sitemap sources batch: %w", err)
		}
	}

	// 4. Insert sitemap entries in batches of 100
	const entBatch = 100
	for lo := 0; lo < len(ev.Entries); lo += entBatch {
		hi := lo + entBatch
		if hi > len(ev.Entries) {
			hi = len(ev.Entries)
		}
		var sb strings.Builder
		args := make([]any, 0, (hi-lo)*8)
		sb.WriteString(`INSERT OR REPLACE INTO sitecrawl_sitemap_entries(
			run_id, sitemap_id, seq, url_id, loc, lastmod, changefreq, priority) VALUES `)
		for i := lo; i < hi; i++ {
			if i > lo {
				sb.WriteByte(',')
			}
			sb.WriteString("(?,?,?,?,?,?,?,?)")
			e := ev.Entries[i]
			args = append(args, runID, e.SitemapID, e.Seq, e.URLID, e.Loc, e.LastMod, e.ChangeFreq, e.Priority)
		}
		if _, err := tx.Exec(sb.String(), args...); err != nil {
			return fmt.Errorf("sitecrawl: insert sitemap entries batch: %w", err)
		}
	}

	return tx.Commit()
}
