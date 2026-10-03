package sitecrawl

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Tabs. Each maps to an indexed predicate.
const (
	TabInternal   = "internal"
	TabExternal   = "external"
	TabResponse   = "response"
	TabTitles     = "titles"
	TabMeta       = "meta"
	TabH1         = "h1"
	TabH2         = "h2"
	TabImages     = "images"
	TabCanonicals = "canonicals"
	TabDirectives = "directives"
	TabHreflang   = "hreflang"
	TabSchema     = "schema"
	TabLinks      = "links"
	TabSitemaps   = "sitemaps"
	TabIssues     = "issues"
)

const (
	defaultLimit = 100
	maxLimit     = 500
)

// sortColumns is the ONLY place a user-supplied sort key becomes a column name.
// An unknown key falls back rather than erroring, which is what keeps the
// frontend's column list safe to drift ahead of Go.
var sortColumns = map[string]string{
	"url":          "url",
	"status":       "status",
	"kind":         "kind",
	"contentType":  "content_type",
	"size":         "size_bytes",
	"responseMs":   "response_ms",
	"depth":        "depth",
	"title":        "title",
	"titleLen":     "title_len",
	"metaDesc":     "meta_desc",
	"metaDescLen":  "meta_desc_len",
	"h1":           "h1",
	"h1Len":        "h1_len",
	"h1Count":      "h1_count",
	"h2":           "h2",
	"wordCount":    "word_count",
	"textRatio":    "text_ratio",
	"lang":         "lang",
	"canonical":    "canonical",
	"indexable":    "indexable",
	"indexability": "indexability",
	"inlinks":      "inlinks",
	"inlinksUniq":  "inlinks_uniq",
	"outlinks":     "outlinks",
	"outlinksExt":  "outlinks_ext",
	"images":       "images_count",
	"imagesNoAlt":  "images_noalt",
	"issues":       "issue_count",
	"severity":     "issue_max_sev",
	"redirectTo":   "redirect_to",
	"redirectHops": "redirect_hops",
	"lastMod":      "last_mod",
	"crawledAt":    "crawled_at",
}

// cellColumns maps a requested column id to the SQL expression that fills it.
// Anything not listed renders empty rather than failing the window.
var cellColumns = map[string]string{
	"url":          "url",
	"kind":         "kind",
	"contentType":  "content_type",
	"status":       "CAST(status AS TEXT)",
	"size":         "CAST(size_bytes AS TEXT)",
	"responseMs":   "CAST(response_ms AS TEXT)",
	"depth":        "CAST(depth AS TEXT)",
	"title":        "title",
	"titleLen":     "CAST(title_len AS TEXT)",
	"metaDesc":     "meta_desc",
	"metaDescLen":  "CAST(meta_desc_len AS TEXT)",
	"h1":           "h1",
	"h1Len":        "CAST(h1_len AS TEXT)",
	"h1Count":      "CAST(h1_count AS TEXT)",
	"h2":           "h2",
	"h2Count":      "CAST(h2_count AS TEXT)",
	"wordCount":    "CAST(word_count AS TEXT)",
	"textRatio":    "CAST(ROUND(text_ratio, 3) AS TEXT)",
	"lang":         "lang",
	"canonical":    "canonical",
	"metaRobots":   "meta_robots",
	"xRobots":      "x_robots",
	"indexability": "indexability",
	"robotsState":  "robots_state",
	"inlinks":      "CAST(inlinks AS TEXT)",
	"inlinksUniq":  "CAST(inlinks_uniq AS TEXT)",
	"outlinks":     "CAST(outlinks AS TEXT)",
	"outlinksExt":  "CAST(outlinks_ext AS TEXT)",
	"images":       "CAST(images_count AS TEXT)",
	"imagesNoAlt":  "CAST(images_noalt AS TEXT)",
	"issues":       "CAST(issue_count AS TEXT)",
	"redirectTo":   "redirect_to",
	"redirectHops": "CAST(redirect_hops AS TEXT)",
	"errorType":    "error_type",
	"discoveredBy": "discovered_by",
	"lastMod":      "last_mod",
	"crawledAt":    "crawled_at",
}

// filterClause turns a tab and filter slug into an indexed predicate.
// Returns the SQL fragment and its arguments.
func filterClause(tab, filter string) (string, []any) {
	var (
		where []string
		args  []any
	)
	add := func(frag string, a ...any) {
		where = append(where, frag)
		args = append(args, a...)
	}

	// "issue:<code>" works on every page tab: it is how the Issues tab and the
	// Overview tree's issue rows jump to the affected URLs, without giving each
	// of the 55 codes its own named filter.
	if strings.HasPrefix(filter, "issue:") {
		add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)",
			strings.TrimPrefix(filter, "issue:"))
		filter = ""
	}

	switch tab {
	case TabExternal:
		add("is_internal = 0")
	case TabResponse:
		// every URL, internal and external
	case TabImages:
		add("kind = ?", KindImage)
	case TabLinks, TabIssues:
		// handled by their own query builders
	case TabSitemaps:
		add("discovered_by = ?", SourceSitemap)
	default:
		add("is_internal = 1")
	}

	switch tab {
	case TabTitles:
		add("kind = ? AND status_class = 2", KindHTML)
		switch filter {
		case "missing":
			add("title = ''")
		case "duplicate":
			add("EXISTS(SELECT 1 FROM sitecrawl_dupes d WHERE d.run_id = p.run_id AND d.url_id = p.url_id AND d.kind = ?)", DupeTitle)
		case "long":
			add("title_len > ?", titleMaxLen)
		case "short":
			add("title_len > 0 AND title_len < ?", titleMinLen)
		case "multiple":
			add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)", IssueTitleMultiple)
		case "sameAsH1":
			add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)", IssueTitleSameAsH1)
		}
	case TabMeta:
		add("kind = ? AND status_class = 2", KindHTML)
		switch filter {
		case "missing":
			add("meta_desc = ''")
		case "duplicate":
			add("EXISTS(SELECT 1 FROM sitecrawl_dupes d WHERE d.run_id = p.run_id AND d.url_id = p.url_id AND d.kind = ?)", DupeMeta)
		case "long":
			add("meta_desc_len > ?", metaMaxLen)
		case "short":
			add("meta_desc_len > 0 AND meta_desc_len < ?", metaMinLen)
		case "multiple":
			add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)", IssueMetaMultiple)
		}
	case TabH1:
		add("kind = ? AND status_class = 2", KindHTML)
		switch filter {
		case "missing":
			add("h1_count = 0")
		case "duplicate":
			add("EXISTS(SELECT 1 FROM sitecrawl_dupes d WHERE d.run_id = p.run_id AND d.url_id = p.url_id AND d.kind = ?)", DupeH1)
		case "multiple":
			add("h1_count > 1")
		case "long":
			add("h1_len > ?", h1MaxLen)
		}
	case TabH2:
		add("kind = ? AND status_class = 2", KindHTML)
		switch filter {
		case "missing":
			add("h2_count = 0")
		case "multiple":
			add("h2_count > 1")
		}
	case TabCanonicals:
		add("kind = ? AND status_class = 2", KindHTML)
		switch filter {
		case "missing":
			add("canonical = ''")
		case "self":
			add("canonical != '' AND indexability != ?", IndexCanonicalised)
		case "canonicalised":
			add("indexability = ?", IndexCanonicalised)
		}
	case TabDirectives:
		add("kind = ?", KindHTML)
		switch filter {
		case "noindex":
			add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)", IssueNoindex)
		case "nofollow":
			add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)", IssueNofollow)
		case "refresh":
			add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)", IssueMetaRefresh)
		}
	case TabResponse:
		switch filter {
		case "success":
			add("status_class = 2")
		case "redirect":
			add("status_class = 3")
		case "clientError":
			add("status_class = 4")
		case "serverError":
			add("status_class = 5")
		case "noResponse":
			add("status = 0 AND error_type != ?", ErrTooLarge)
		case "blocked":
			add("robots_state = ?", RobotsBlocked)
		}
	case TabHreflang:
		add("kind = ? AND status_class = 2", KindHTML)
		issueFilter := map[string]string{
			"noReturn":     IssueHreflangNoReturn,
			"missingSelf":  IssueHreflangNoSelf,
			"invalidCode":  IssueHreflangBadCode,
			"nonOK":        IssueHreflangNonOK,
			"nonCanonical": IssueHreflangNoncanon,
		}
		if code, ok := issueFilter[filter]; ok {
			add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)", code)
		}
	case TabSchema:
		add("kind = ? AND status_class = 2", KindHTML)
		if filter == "missing" {
			add("EXISTS(SELECT 1 FROM sitecrawl_issues i WHERE i.run_id = p.run_id AND i.url_id = p.url_id AND i.code = ?)", IssueNoStructuredData)
		}
	case TabSitemaps:
		switch filter {
		case "orphan":
			add("orphan = 1")
		case "nonIndexable":
			add("indexable = 0")
		}
	default:
		// Internal / External share the resource-kind filters.
		switch filter {
		case "html", "css", "js", "pdf":
			add("kind = ?", filter)
		case "images":
			add("kind = ?", KindImage)
		case "other":
			// Fonts and media have no chip of their own; "other" is everything
			// that is not one of the named chips, or they would be invisible.
			add("kind IN (?, ?, ?)", KindOther, KindFont, KindMedia)
		case "indexable":
			add("indexable = 1")
		case "nonIndexable":
			add("indexable = 0")
		case "orphan":
			add("orphan = 1")
		case "issues":
			add("issue_count > 0")
		}
	}

	if len(where) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(where, " AND "), args
}

// Rows returns one window of grid rows.
func (s *Service) Rows(q RowQuery) (RowPage, error) {
	db, err := s.readDB()
	if err != nil {
		return RowPage{}, err
	}
	if q.RunID == "" {
		return RowPage{Rows: []Row{}, Total: 0}, nil
	}
	if q.Limit <= 0 {
		q.Limit = defaultLimit
	}
	if q.Limit > maxLimit {
		q.Limit = maxLimit
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	if q.Tab == TabIssues {
		return s.issueRows(db, q)
	}
	if q.Tab == TabLinks {
		return s.linkTabRows(db, q)
	}

	where, args := filterClause(q.Tab, q.Filter)
	searchSQL, searchArgs := s.searchClause(q.Search)
	idSQL, idArgs := idClause(q.IDs)
	base := ` FROM sitecrawl_pages p WHERE p.run_id = ?` + where + searchSQL + idSQL

	out := RowPage{Rows: []Row{}, Offset: q.Offset, Total: -1, Revision: s.revisionOf(q.RunID)}

	if q.WantTotal {
		countArgs := append(append(append([]any{q.RunID}, args...), searchArgs...), idArgs...)
		if err := db.QueryRow(`SELECT COUNT(*)`+base, countArgs...).Scan(&out.Total); err != nil {
			return RowPage{}, err
		}
	}

	// The tiebreaker keeps paging stable while the crawl is still inserting.
	order := sortColumns[q.Sort]
	if order == "" {
		order = "url_id"
	}
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}

	sel := make([]string, 0, len(q.Cols))
	for _, c := range q.Cols {
		if expr, ok := cellColumns[c]; ok {
			sel = append(sel, "COALESCE("+expr+", '')")
		} else {
			sel = append(sel, "''")
		}
	}
	if len(sel) == 0 {
		sel = append(sel, "url")
	}

	query := `SELECT url, status, is_internal, indexable, issue_count, rendered, orphan, ` +
		strings.Join(sel, ", ") + base +
		fmt.Sprintf(` ORDER BY %s %s, url_id %s LIMIT ? OFFSET ?`, order, dir, dir)

	rowArgs := append(append(append([]any{q.RunID}, args...), searchArgs...), idArgs...)
	rowArgs = append(rowArgs, q.Limit, q.Offset)

	rows, err := db.Query(query, rowArgs...)
	if err != nil {
		return RowPage{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                                                string
			status                                            int
			internal, indexable, issueCount, rendered, orphan int
		)
		cells := make([]string, len(sel))
		scan := make([]any, 0, 7+len(cells))
		scan = append(scan, &id, &status, &internal, &indexable, &issueCount, &rendered, &orphan)
		for i := range cells {
			scan = append(scan, &cells[i])
		}
		if err := rows.Scan(scan...); err != nil {
			return RowPage{}, err
		}
		out.Rows = append(out.Rows, Row{
			ID:     id,
			Cells:  cells,
			Status: status,
			Flags:  rowFlags(internal, indexable, issueCount, rendered, orphan),
		})
	}
	return out, rows.Err()
}

func rowFlags(internal, indexable, issues, rendered, orphan int) uint32 {
	var f uint32
	if internal == 1 {
		f |= RowInternal
	}
	if indexable == 1 {
		f |= RowIndexable
	}
	if issues > 0 {
		f |= RowHasIssues
	}
	if rendered == 1 {
		f |= RowRendered
	}
	if orphan == 1 {
		f |= RowOrphan
	}
	return f
}

// maxSelectedIDs caps the IN list so a pathological caller cannot build a
// 50k-variable statement.
const maxSelectedIDs = 2000

// idClause pins the query to an explicit URL set (export selected, PageSpeed
// on selected). Empty means "no pin", not "match nothing".
func idClause(ids []string) (string, []any) {
	if len(ids) == 0 {
		return "", nil
	}
	if len(ids) > maxSelectedIDs {
		ids = ids[:maxSelectedIDs]
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return " AND p.url IN (?" + strings.Repeat(",?", len(ids)-1) + ")", args
}

// searchClause prefers FTS5 and falls back to LIKE.
//
// LIKE '%x%' cannot use an index, so at 50k rows it is a 50-150ms scan on every
// debounced keystroke — while the crawler is writing to the same file. FTS5
// ships with modernc.org/sqlite, so this costs no dependency.
func (s *Service) searchClause(search string) (string, []any) {
	term := strings.TrimSpace(search)
	if term == "" {
		return "", nil
	}
	if s.ftsReady {
		return ` AND p.rowid IN (SELECT rowid FROM sitecrawl_fts WHERE sitecrawl_fts MATCH ?)`,
			[]any{ftsQuery(term)}
	}
	like := "%" + term + "%"
	return ` AND (p.url LIKE ? OR p.title LIKE ? OR p.meta_desc LIKE ?)`, []any{like, like, like}
}

// ftsQuery turns typed text into an FTS5 prefix query.
//
// Every token is quoted and suffixed with *, so "san pham" matches "san-pham"
// mid-URL and a stray quote or hyphen cannot become FTS syntax.
func ftsQuery(term string) string {
	fields := strings.FieldsFunc(term, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '/' || r == '-' || r == '_' || r == '?' || r == '&' || r == '='
	})
	if len(fields) == 0 {
		return `""`
	}
	parts := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, `""`)
		parts = append(parts, `"`+f+`"*`)
	}
	return strings.Join(parts, " ")
}

// issueRows powers the Issues tab, which lists rules rather than URLs.
func (s *Service) issueRows(db *sql.DB, q RowQuery) (RowPage, error) {
	out := RowPage{Rows: []Row{}, Offset: q.Offset, Total: -1, Revision: s.revisionOf(q.RunID)}

	where := ""
	args := []any{q.RunID}
	switch q.Filter {
	case "critical":
		where = " AND severity = ?"
		args = append(args, SeverityCritical)
	case "warning":
		where = " AND severity = ?"
		args = append(args, SeverityWarning)
	case "notice":
		where = " AND severity = ?"
		args = append(args, SeverityNotice)
	}

	if q.WantTotal {
		db.QueryRow(`SELECT COUNT(DISTINCT code) FROM sitecrawl_issues WHERE run_id = ?`+where, args...).Scan(&out.Total)
	}

	order := "urls DESC"
	if q.Sort == "code" {
		order = "code ASC"
	} else if q.Sort == "severity" {
		order = "severity DESC, urls DESC"
	}

	rows, err := db.Query(`
		SELECT code, severity, category, COUNT(*) AS urls
		  FROM sitecrawl_issues WHERE run_id = ?`+where+`
		 GROUP BY code, severity, category
		 ORDER BY `+order+` LIMIT ? OFFSET ?`,
		append(args, q.Limit, q.Offset)...)
	if err != nil {
		return RowPage{}, err
	}
	defer rows.Close()

	var total int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, q.RunID).Scan(&total)

	for rows.Next() {
		var code, category string
		var severity, urls int
		if err := rows.Scan(&code, &severity, &category, &urls); err != nil {
			return RowPage{}, err
		}
		pct := "0"
		if total > 0 {
			pct = strconv.FormatFloat(float64(urls)*100/float64(total), 'f', 1, 64)
		}
		out.Rows = append(out.Rows, Row{
			ID:     code,
			Cells:  []string{code, category, strconv.Itoa(severity), strconv.Itoa(urls), pct},
			Status: 0,
			Flags:  0,
		})
	}
	return out, rows.Err()
}

// linkTabRows powers the Links tab, where one row is one edge, not one URL.
//
// Cells are a fixed projection — [source, target, anchor, placement,
// targetStatus] — because edges share no columns with pages; the frontend's
// links TabDef mirrors this order. Rows keep crawl order (seq); Sort is
// ignored on purpose, an edge list has no meaningful per-column sort cheaper
// than the filters that exist.
func (s *Service) linkTabRows(db *sql.DB, q RowQuery) (RowPage, error) {
	out := RowPage{Rows: []Row{}, Offset: q.Offset, Total: -1, Revision: s.revisionOf(q.RunID)}

	where := ""
	args := []any{q.RunID}
	switch q.Filter {
	case "internal":
		where += " AND (l.flags & 1) != 0"
	case "external":
		where += " AND (l.flags & 1) = 0"
	case "nofollow":
		where += " AND (l.flags & 2) != 0"
	}
	if term := strings.TrimSpace(q.Search); term != "" {
		like := "%" + term + "%"
		where += " AND (su.url LIKE ? OR du.url LIKE ? OR l.anchor LIKE ?)"
		args = append(args, like, like, like)
	}

	joins := ` FROM sitecrawl_links l
	  JOIN sitecrawl_urls su ON su.run_id = l.run_id AND su.id = l.src_id
	  JOIN sitecrawl_urls du ON du.run_id = l.run_id AND du.id = l.dst_id`
	cond := ` WHERE l.run_id = ?` + where

	if q.WantTotal {
		if err := db.QueryRow(`SELECT COUNT(*)`+joins+cond, args...).Scan(&out.Total); err != nil {
			return RowPage{}, err
		}
	}

	rows, err := db.Query(`
		SELECT l.rowid, su.url, du.url, l.anchor, l.placement, l.flags, COALESCE(p.status, 0)`+
		joins+` LEFT JOIN sitecrawl_pages p ON p.run_id = l.run_id AND p.url_id = l.dst_id`+
		cond+` ORDER BY l.rowid LIMIT ? OFFSET ?`,
		append(append([]any{}, args...), q.Limit, q.Offset)...)
	if err != nil {
		return RowPage{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			rowid                  int64
			src, dst, anchor       string
			placement, targetState int
			flags                  uint32
		)
		if err := rows.Scan(&rowid, &src, &dst, &anchor, &placement, &flags, &targetState); err != nil {
			return RowPage{}, err
		}
		out.Rows = append(out.Rows, Row{
			ID:     strconv.FormatInt(rowid, 10),
			Cells:  []string{src, dst, anchor, strconv.Itoa(placement), strconv.Itoa(targetState)},
			Status: targetState,
			Flags:  flags,
		})
	}
	return out, rows.Err()
}

// Facets returns every tab and filter count in one call, so the Overview tree
// never has to poll.
func (s *Service) Facets(runID string) (map[string]int, error) {
	db, err := s.readDB()
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	if runID == "" {
		return out, nil
	}

	rows, err := db.Query(`
		SELECT is_internal, kind, status_class, indexable, orphan,
		       COUNT(*), SUM(CASE WHEN issue_count > 0 THEN 1 ELSE 0 END)
		  FROM sitecrawl_pages WHERE run_id = ?
		 GROUP BY is_internal, kind, status_class, indexable, orphan`, runID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var internal, statusClass, indexable, orphan, n, withIssues int
		var kind string
		if err := rows.Scan(&internal, &kind, &statusClass, &indexable, &orphan, &n, &withIssues); err != nil {
			continue
		}
		side := TabInternal
		if internal == 0 {
			side = TabExternal
		}
		out[side] += n
		// Facet keys are FILTER slugs, not kind values: the chips look up
		// "<tab>.<filter>", so "image" must land under "images" and the chipless
		// kinds under "other" — mirroring filterClause exactly.
		switch kind {
		case KindImage:
			out[side+".images"] += n
			out[TabImages] += n // the Content > Images tab lists every image
		case KindFont, KindMedia:
			out[side+".other"] += n
		default:
			out[side+"."+kind] += n
		}
		out[TabResponse+"."+statusBucket(statusClass)] += n
		out[TabResponse] += n
		if indexable == 0 {
			out[side+".nonIndexable"] += n
		} else {
			out[side+".indexable"] += n
		}
		if orphan == 1 {
			out[TabSitemaps+".orphan"] += n
		}
		out[side+".issues"] += withIssues
	}
	rows.Close()

	// Link counts feed the Links tab's filter chips and its tree row.
	var lTotal, lInternal, lNofollow sql.NullInt64
	if err := db.QueryRow(`
		SELECT COUNT(*), SUM(CASE WHEN flags & 1 THEN 1 ELSE 0 END), SUM(CASE WHEN flags & 2 THEN 1 ELSE 0 END)
		  FROM sitecrawl_links WHERE run_id = ?`, runID).
		Scan(&lTotal, &lInternal, &lNofollow); err == nil && lTotal.Int64 > 0 {
		out[TabLinks] = int(lTotal.Int64)
		out[TabLinks+".internal"] = int(lInternal.Int64)
		out[TabLinks+".external"] = int(lTotal.Int64 - lInternal.Int64)
		out[TabLinks+".nofollow"] = int(lNofollow.Int64)
	}

	// Issue counts drive both the Issues tab and the tree's problem rows.
	irows, err := db.Query(`SELECT code, COUNT(*) FROM sitecrawl_issues WHERE run_id = ? GROUP BY code`, runID)
	if err == nil {
		for irows.Next() {
			var code string
			var n int
			if err := irows.Scan(&code, &n); err == nil {
				out["issue."+code] = n
			}
		}
		irows.Close()
	}
	return out, nil
}

func statusBucket(class int) string {
	switch class {
	case 2:
		return "success"
	case 3:
		return "redirect"
	case 4:
		return "clientError"
	case 5:
		return "serverError"
	}
	return "noResponse"
}

// Page returns the full stored record for one URL — the only read that touches
// the JSON blob, and only ever one row of it.
func (s *Service) Page(runID, rawURL string) (Page, error) {
	db, err := s.readDB()
	if err != nil {
		return Page{}, err
	}
	var blob string
	if err := db.QueryRow(`SELECT data FROM sitecrawl_pages WHERE run_id = ? AND url = ?`,
		runID, rawURL).Scan(&blob); err != nil {
		return Page{}, err
	}
	var p Page
	if err := unmarshalPage(blob, &p); err != nil {
		return Page{}, err
	}
	return p, nil
}

// LinkRow is one edge as the detail pane shows it.
type LinkRow struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	Anchor      string `json:"anchor"`
	Placement   int    `json:"placement"`
	Flags       uint32 `json:"flags"`
	TargetState int    `json:"targetStatus"`
}

// Inlinks lists the pages linking to a URL.
func (s *Service) Inlinks(runID, rawURL string, offset, limit int) ([]LinkRow, error) {
	return s.links(runID, rawURL, "dst", offset, limit)
}

// Outlinks lists what a page links to.
func (s *Service) Outlinks(runID, rawURL string, offset, limit int) ([]LinkRow, error) {
	return s.links(runID, rawURL, "src", offset, limit)
}

func (s *Service) links(runID, rawURL, side string, offset, limit int) ([]LinkRow, error) {
	db, err := s.readDB()
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}
	var anchorCol, otherCol string
	if side == "dst" {
		anchorCol, otherCol = "dst_id", "src_id"
	} else {
		anchorCol, otherCol = "src_id", "dst_id"
	}

	rows, err := db.Query(`
		SELECT su.url, du.url, l.anchor, l.placement, l.flags, COALESCE(p.status, 0)
		  FROM sitecrawl_links l
		  JOIN sitecrawl_urls  su ON su.run_id = l.run_id AND su.id = l.src_id
		  JOIN sitecrawl_urls  du ON du.run_id = l.run_id AND du.id = l.dst_id
		  LEFT JOIN sitecrawl_pages p ON p.run_id = l.run_id AND p.url_id = l.`+otherCol+`
		 WHERE l.run_id = ? AND l.`+anchorCol+` = (
		       SELECT id FROM sitecrawl_urls WHERE run_id = ? AND url = ? LIMIT 1)
		 ORDER BY l.seq LIMIT ? OFFSET ?`,
		runID, runID, rawURL, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []LinkRow{}
	for rows.Next() {
		var l LinkRow
		if err := rows.Scan(&l.Source, &l.Target, &l.Anchor, &l.Placement, &l.Flags, &l.TargetState); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
