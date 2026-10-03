package sitecrawl

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
)

// exec runs one of the post-crawl writes.
//
// None of these can fail the crawl: the pages are already fetched and stored,
// and the phase is idempotent by design (see finalizeCodes). What a dropped
// error costs instead is a report that is quietly incomplete — issues missing
// from a list the user reads as exhaustive — so it is recorded.
func (c *coordinator) exec(what, query string, args ...any) {
	if _, err := c.db.Exec(query, args...); err != nil {
		slog.Error("sitecrawl: "+what, "run", c.runID, "err", err)
	}
}

// finalizeCodes are every issue the post-crawl phase can add. Each rule deletes
// its own codes before recomputing, which is what makes the phase safe to
// re-run after a crash — there are no transactions in this codebase, so
// idempotence is the only way back to a consistent state.
var finalizeCodes = []string{
	IssueOrphan, IssueInternalRedirect, IssueBrokenImage,
	IssueCanonicalChain, IssueCanonicalLoop, IssueCanonicalNonOK, IssueCanonicalNoindex,
	IssueHreflangNoReturn, IssueHreflangNoSelf, IssueHreflangBadCode,
	IssueHreflangNonOK, IssueHreflangNoncanon,
}

// finalize computes everything that needs the whole crawl.
//
// Every step follows the same shape, forced by the workspace's single
// connection: drain narrow columns fully, close the cursor, compute in Go,
// write in batches. Iterating a cursor while updating deadlocks instantly.
func (c *coordinator) finalize(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	// Cleared first so a re-run after a crash recomputes rather than merges. If
	// this fails the rules below still run, and the report ends up holding both
	// the old findings and the new ones — worth knowing about.
	if err := clearIssues(c.db, c.runID, finalizeCodes); err != nil {
		slog.Error("sitecrawl: clear finalize issues", "run", c.runID, "err", err)
	}

	c.finalizeInlinks(ctx)
	c.finalizeOrphans(ctx)
	c.finalizeRedirectSources(ctx)
	c.finalizeBrokenImages(ctx)
	c.finalizeCanonicals(ctx)
	c.finalizeHreflang(ctx)

	if c.opts.duplicationOn() {
		c.setPhase(ctx, PhaseDuplicates)
		findExactDuplicates(c.db, c.runID)
		findNearDuplicates(c.db, c.runID, c.opts.DuplicationThreshold)
	}

	syncIssueCounts(c.db, c.runID)
	c.revision++
}

// finalizeInlinks counts incoming links per page.
func (c *coordinator) finalizeInlinks(ctx context.Context) {
	rows, err := c.db.Query(`
		SELECT dst_id, COUNT(*), COUNT(DISTINCT src_id)
		  FROM sitecrawl_links WHERE run_id = ? GROUP BY dst_id`, c.runID)
	if err != nil {
		return
	}
	type tally struct {
		id          int64
		total, uniq int
	}
	var list []tally
	for rows.Next() {
		var t tally
		if err := rows.Scan(&t.id, &t.total, &t.uniq); err == nil {
			list = append(list, t)
		}
	}
	rows.Close() // drain fully before writing — single-connection DB
	if err := rows.Err(); err != nil {
		// A truncated read leaves the pages it never reached at inlinks = 0,
		// and finalizeOrphans immediately reads that as "nothing links here" —
		// so a read error turns into pages falsely reported as orphans.
		slog.Error("sitecrawl: scan inlinks", "run", c.runID, "err", err)
	}

	// One statement per 500 pages: a CASE expression keyed on url_id updates the
	// whole batch without a round trip per row.
	chunked(len(list), 500, func(lo, hi int) error {
		var sb strings.Builder
		args := make([]any, 0, (hi-lo)*5+1)
		sb.WriteString(`UPDATE sitecrawl_pages SET inlinks = CASE url_id `)
		for i := lo; i < hi; i++ {
			sb.WriteString("WHEN ? THEN ? ")
			args = append(args, list[i].id, list[i].total)
		}
		sb.WriteString(`ELSE inlinks END, inlinks_uniq = CASE url_id `)
		for i := lo; i < hi; i++ {
			sb.WriteString("WHEN ? THEN ? ")
			args = append(args, list[i].id, list[i].uniq)
		}
		sb.WriteString(`ELSE inlinks_uniq END WHERE run_id = ? AND url_id IN (`)
		for i := lo; i < hi; i++ {
			if i > lo {
				sb.WriteByte(',')
			}
			sb.WriteByte('?')
		}
		sb.WriteByte(')')
		args = append(args, c.runID)
		for i := lo; i < hi; i++ {
			args = append(args, list[i].id)
		}
		_, err := c.db.Exec(sb.String(), args...)
		return err
	})
}

// finalizeOrphans flags pages nothing links to.
//
// Only sitemap- and list-sourced pages qualify: a page found by following a
// link has an inlink by definition, and the seed is not an orphan either.
// LibreCrawl has the data for this and never computes it.
func (c *coordinator) finalizeOrphans(ctx context.Context) {
	if _, err := c.db.Exec(`
		UPDATE sitecrawl_pages SET orphan = 1
		 WHERE run_id = ? AND is_internal = 1 AND status_class = 2
		   AND inlinks = 0 AND discovered_by IN (?, ?)`,
		c.runID, SourceSitemap, SourceManual); err != nil {
		return
	}
	c.exec("flag orphans", `
		INSERT INTO sitecrawl_issues(run_id, url_id, code, severity, category, detail)
		SELECT run_id, url_id, ?, ?, ?, ''
		  FROM sitecrawl_pages WHERE run_id = ? AND orphan = 1
		ON CONFLICT(run_id, url_id, code) DO NOTHING`,
		IssueOrphan, severityOf(IssueOrphan), categoryOf(IssueOrphan), c.runID)
}

// finalizeRedirectSources flags the pages that LINK to a redirect.
//
// The redirect itself is already reported, but it is not the actionable row:
// the fix is on the page holding the stale href, and that page is what a person
// opens to edit.
func (c *coordinator) finalizeRedirectSources(ctx context.Context) {
	c.exec("flag redirect sources", `
		INSERT INTO sitecrawl_issues(run_id, url_id, code, severity, category, detail)
		SELECT DISTINCT l.run_id, l.src_id, ?, ?, ?, ''
		  FROM sitecrawl_links l
		  JOIN sitecrawl_pages p ON p.run_id = l.run_id AND p.url_id = l.dst_id
		 WHERE l.run_id = ? AND p.status_class = 3 AND p.is_internal = 1
		ON CONFLICT(run_id, url_id, code) DO NOTHING`,
		IssueInternalRedirect, severityOf(IssueInternalRedirect), categoryOf(IssueInternalRedirect), c.runID)
}

// finalizeBrokenImages flags the pages that EMBED an image whose own fetch
// failed. Like redirect sources, the actionable row is the page holding the
// <img>, not the image. Only possible at all because CrawlImages fetches the
// images — the per-page rules see the tag but never the image's status.
func (c *coordinator) finalizeBrokenImages(ctx context.Context) {
	c.exec("flag broken images", `
		INSERT INTO sitecrawl_issues(run_id, url_id, code, severity, category, detail)
		SELECT DISTINCT l.run_id, l.src_id, ?, ?, ?, p.url
		  FROM sitecrawl_links l
		  JOIN sitecrawl_pages p ON p.run_id = l.run_id AND p.url_id = l.dst_id
		 WHERE l.run_id = ? AND (l.flags & ?) != 0
		   AND (p.status >= 400 OR (p.status = 0 AND p.error_type NOT IN ('', ?)))
		ON CONFLICT(run_id, url_id, code) DO NOTHING`,
		IssueBrokenImage, severityOf(IssueBrokenImage), categoryOf(IssueBrokenImage),
		c.runID, FlagImageLink, ErrTooLarge)
}

// pageFacts is the narrow projection every graph rule works from.
type pageFacts struct {
	id        int64
	url       string
	canonical string
	status    int
	indexable bool
	internal  bool
}

// loadFacts drains the columns the graph rules need and closes the cursor.
// At 50k pages this is roughly 18MB, which is the price of not deadlocking.
func (c *coordinator) loadFacts() ([]pageFacts, map[string]*pageFacts) {
	rows, err := c.db.Query(`
		SELECT url_id, url, canonical, status, indexable, is_internal
		  FROM sitecrawl_pages WHERE run_id = ?`, c.runID)
	if err != nil {
		return nil, nil
	}
	var list []pageFacts
	for rows.Next() {
		var f pageFacts
		var indexable, internal int
		if err := rows.Scan(&f.id, &f.url, &f.canonical, &f.status, &indexable, &internal); err != nil {
			continue
		}
		f.indexable = indexable == 1
		f.internal = internal == 1
		list = append(list, f)
	}
	rows.Close()

	byKey := make(map[string]*pageFacts, len(list))
	for i := range list {
		byKey[frontierKey(list[i].url, false)] = &list[i]
	}
	return list, byKey
}

// finalizeCanonicals walks canonical pointers looking for chains, loops and
// targets that cannot carry the signal. None of this exists in LibreCrawl.
func (c *coordinator) finalizeCanonicals(ctx context.Context) {
	list, byKey := c.loadFacts()
	if len(list) == 0 {
		return
	}
	var out []issueRow
	for i := range list {
		p := &list[i]
		if !p.internal || p.canonical == "" {
			continue
		}
		key := frontierKey(p.canonical, false)
		if key == frontierKey(p.url, false) {
			continue // self-referencing, the healthy case
		}
		target, ok := byKey[key]
		if !ok {
			continue // outside the crawl; nothing verifiable
		}
		switch {
		case target.status < 200 || target.status >= 300:
			out = append(out, issueRow{URLID: p.id, issue: issue{Code: IssueCanonicalNonOK, Detail: target.url}})
		case !target.indexable:
			out = append(out, issueRow{URLID: p.id, issue: issue{Code: IssueCanonicalNoindex, Detail: target.url}})
		}

		// Walk on: a canonical pointing at a page that itself canonicalises
		// elsewhere is a chain, and Google follows at most one hop reliably.
		seen := map[string]bool{frontierKey(p.url, false): true, key: true}
		cur := target
		for hops := 0; cur != nil && cur.canonical != "" && hops < maxRedirects; hops++ {
			nextKey := frontierKey(cur.canonical, false)
			if nextKey == frontierKey(cur.url, false) {
				break // target is self-canonical, the chain ends properly
			}
			if seen[nextKey] {
				out = append(out, issueRow{URLID: p.id, issue: issue{Code: IssueCanonicalLoop, Detail: cur.url}})
				break
			}
			out = append(out, issueRow{URLID: p.id, issue: issue{Code: IssueCanonicalChain, Detail: cur.canonical}})
			seen[nextKey] = true
			cur = byKey[nextKey]
		}
	}
	writeIssues(c.db, c.runID, out)
}

// finalizeHreflang checks that alternates point at live, canonical pages and
// link back. A missing return tag makes the whole cluster invalid to Google,
// and it is invisible without crawling both sides.
func (c *coordinator) finalizeHreflang(ctx context.Context) {
	rows, err := c.db.Query(`
		SELECT url_id, url, data FROM sitecrawl_pages
		 WHERE run_id = ? AND is_internal = 1 AND kind = ? AND status_class = 2`,
		c.runID, KindHTML)
	if err != nil {
		return
	}
	type entry struct {
		id   int64
		url  string
		alts []Hreflang
	}
	var list []entry
	for rows.Next() {
		var e entry
		var blob string
		if err := rows.Scan(&e.id, &e.url, &blob); err != nil {
			continue
		}
		var p Page
		if err := unmarshalPage(blob, &p); err != nil || len(p.Hreflang) == 0 {
			continue
		}
		e.alts = p.Hreflang
		list = append(list, e)
	}
	rows.Close()
	if len(list) == 0 {
		return
	}

	_, byKey := c.loadFacts()
	alts := make(map[string][]Hreflang, len(list))
	for _, e := range list {
		alts[frontierKey(e.url, false)] = e.alts
	}

	var out []issueRow
	for _, e := range list {
		selfKey := frontierKey(e.url, false)
		hasSelf := false
		for _, h := range e.alts {
			if !validLangCode(h.Lang) {
				out = append(out, issueRow{URLID: e.id, issue: issue{Code: IssueHreflangBadCode, Detail: h.Lang}})
			}
			targetKey := frontierKey(h.URL, false)
			if targetKey == selfKey {
				hasSelf = true
				continue
			}
			if target, ok := byKey[targetKey]; ok {
				if target.status < 200 || target.status >= 300 {
					out = append(out, issueRow{URLID: e.id, issue: issue{Code: IssueHreflangNonOK, Detail: h.URL}})
					continue
				}
				if target.canonical != "" && frontierKey(target.canonical, false) != targetKey {
					out = append(out, issueRow{URLID: e.id, issue: issue{Code: IssueHreflangNoncanon, Detail: h.URL}})
				}
			}
			// Reciprocity: the alternate must point back at us.
			if back, ok := alts[targetKey]; ok {
				found := false
				for _, b := range back {
					if frontierKey(b.URL, false) == selfKey {
						found = true
						break
					}
				}
				if !found {
					out = append(out, issueRow{URLID: e.id, issue: issue{Code: IssueHreflangNoReturn, Detail: h.URL}})
				}
			}
		}
		if !hasSelf {
			out = append(out, issueRow{URLID: e.id, issue: issue{Code: IssueHreflangNoSelf}})
		}
	}
	writeIssues(c.db, c.runID, out)
}

// validLangCode accepts the shapes Google accepts: "en", "en-GB", "zh-Hant-TW",
// and the literal "x-default".
func validLangCode(code string) bool {
	code = strings.TrimSpace(code)
	if code == "" {
		return false
	}
	if strings.EqualFold(code, "x-default") {
		return true
	}
	parts := strings.Split(code, "-")
	if len(parts) > 3 {
		return false
	}
	lang := parts[0]
	if len(lang) < 2 || len(lang) > 3 || !isAlpha(lang) {
		return false
	}
	for _, p := range parts[1:] {
		switch len(p) {
		case 2: // region
			if !isAlpha(p) && !isDigits(p) {
				return false
			}
		case 3: // UN M.49 region
			if !isDigits(p) {
				return false
			}
		case 4: // script
			if !isAlpha(p) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func isAlpha(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func unmarshalPage(blob string, p *Page) error {
	return json.Unmarshal([]byte(blob), p)
}
