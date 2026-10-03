package sitecrawl

import (
	"onescout/desktop/internal/core/runs"
	"strings"
	"testing"
)

// pageOf reads one crawled page back out of the database.
func pageOf(t *testing.T, s *Service, runID, path string, site *fixtureSite) Page {
	t.Helper()
	p, err := s.Page(runID, site.URL+path)
	if err != nil {
		t.Fatalf("Page(%s): %v", path, err)
	}
	return p
}

func issueCodes(t *testing.T, s *Service, runID, url string) map[string]bool {
	t.Helper()
	db, err := s.readDB()
	if err != nil {
		t.Fatalf("readDB: %v", err)
	}
	rows, err := db.Query(`
		SELECT i.code FROM sitecrawl_issues i
		  JOIN sitecrawl_pages p ON p.run_id = i.run_id AND p.url_id = i.url_id
		 WHERE i.run_id = ? AND p.url = ?`, runID, url)
	if err != nil {
		t.Fatalf("issues: %v", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var c string
		rows.Scan(&c)
		out[c] = true
	}
	return out
}

func TestCrawlFixtureSite(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, emitter := newTestService(t)

	runID := crawlFixture(t, s, site, nil)

	// --- the crawl found the site ---
	facets, err := s.Facets(runID)
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	if facets[TabInternal] < 12 {
		t.Errorf("only %d internal URLs crawled, expected the whole fixture: %v", facets[TabInternal], facets)
	}

	// --- status codes land in the right buckets ---
	if facets[TabResponse+".clientError"] == 0 {
		t.Error("the deliberate 404 was not recorded as a client error")
	}
	if facets[TabResponse+".redirect"] == 0 {
		t.Error("the 301 chain was not recorded as redirects")
	}

	// --- robots.txt is honoured ---
	if site.count("/private/secret") != 0 {
		t.Errorf("/private/secret was fetched %d times despite robots.txt Disallow", site.count("/private/secret"))
	}

	// --- the redirect chain is populated, which LibreCrawl never does ---
	hop := pageOf(t, s, runID, "/hop1", site)
	if len(hop.Redirects) == 0 {
		t.Error("redirect chain is empty — the hops were not recorded")
	}
	if hop.RedirectTo == "" {
		t.Error("redirectTo is empty on a 301")
	}

	// --- a redirect loop is caught rather than followed forever ---
	loop := pageOf(t, s, runID, "/loop-a", site)
	if loop.Error == "" && len(loop.Redirects) > maxRedirects+1 {
		t.Errorf("redirect loop was followed %d times", len(loop.Redirects))
	}

	// --- content rules fire where they should ---
	home := pageOf(t, s, runID, "/", site)
	if home.Title == "" {
		t.Error("homepage title not extracted")
	}
	if home.WordCount < 300 {
		t.Errorf("homepage word count = %d, the filler should have pushed it past 300", home.WordCount)
	}
	if !home.Indexable {
		t.Errorf("homepage reported non-indexable (%s)", home.Indexability)
	}

	// --- noindex is detected ---
	noindex := pageOf(t, s, runID, "/noindex", site)
	if noindex.Indexable {
		t.Error("the noindex page was reported as indexable")
	}
	if !issueCodes(t, s, runID, site.URL+"/noindex")[IssueNoindex] {
		t.Error("noindex issue not recorded")
	}

	// --- duplicates: exact title and description grouping ---
	dupIssues := issueCodes(t, s, runID, site.URL+"/dup-a")
	if !dupIssues[IssueTitleDuplicate] {
		t.Error("duplicate title not flagged — this is one of the gaps LibreCrawl leaves open")
	}
	if !dupIssues[IssueMetaDuplicate] {
		t.Error("duplicate meta description not flagged")
	}

	// --- canonical chain ---
	if !issueCodes(t, s, runID, site.URL+"/canon-a")[IssueCanonicalChain] {
		t.Error("canonical chain a->b->c not detected")
	}

	// --- hreflang reciprocity: EN points at VI, VI never points back ---
	if !issueCodes(t, s, runID, site.URL+"/en/page")[IssueHreflangNoReturn] {
		t.Error("missing hreflang return tag not detected")
	}

	// --- orphan: reachable only from the sitemap ---
	if !issueCodes(t, s, runID, site.URL+"/orphan")[IssueOrphan] {
		t.Error("orphan page not detected")
	}

	// --- the run reported itself as finished exactly once ---
	states := emitter.runStates()
	if len(states) == 0 || states[len(states)-1] != StateCompleted {
		t.Errorf("run state events = %v, want the last to be completed", states)
	}
	if emitter.count(EventProgress) == 0 {
		t.Error("no progress events emitted")
	}
}

// TestCrawlFetchesPageResources is the regression test for the empty
// CSS/JS/Images tabs: images were recorded as link-graph edges but never
// admitted to the frontier, and stylesheets/scripts were never collected at
// all, so no row with kind css/js/image could ever exist.
func TestCrawlFetchesPageResources(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	runID := crawlFixture(t, s, site, nil)

	facets, err := s.Facets(runID)
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	for _, key := range []string{"internal.css", "internal.js", "internal.images"} {
		if facets[key] == 0 {
			t.Errorf("facet %q = 0 — that resource kind was never fetched: %v", key, facets)
		}
	}
	if site.count("/style.css") != 1 {
		t.Errorf("/style.css fetched %d times, want 1 — <link rel=stylesheet> discovery", site.count("/style.css"))
	}
	if site.count("/app.js") != 1 {
		t.Errorf("/app.js fetched %d times, want 1 — <script src> discovery", site.count("/app.js"))
	}
	if site.count("/logo.png") != 1 {
		t.Errorf("/logo.png fetched %d times, want 1 — <img> admission", site.count("/logo.png"))
	}
	if site.count("/lazy.png") != 1 {
		t.Errorf("/lazy.png fetched %d times, want 1 — a data: placeholder src must fall back to data-src", site.count("/lazy.png"))
	}

	// The grid filters see them.
	rows, err := s.Rows(RowQuery{RunID: runID, Tab: TabInternal, Filter: "css", Cols: []string{"url"}, Limit: 10, WantTotal: true})
	if err != nil {
		t.Fatalf("Rows(css): %v", err)
	}
	if rows.Total == 0 {
		t.Error("Internal>CSS filter returns nothing after crawling stylesheets")
	}

	// A 404 image flags the page that embeds it, not just the image.
	if !issueCodes(t, s, runID, site.URL+"/")[IssueBrokenImage] {
		t.Error("broken-image issue missing on the page embedding the 404 image")
	}
}

// TestCrawlResourceOptionsOff proves the switches actually gate the fetches.
func TestCrawlResourceOptionsOff(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	crawlFixture(t, s, site, func(o *Options) {
		o.CrawlImages = false
		o.CrawlCSS = false
		o.CrawlJS = false
	})

	if n := site.count("/style.css"); n != 0 {
		t.Errorf("/style.css fetched %d times with CrawlCSS off", n)
	}
	if n := site.count("/app.js"); n != 0 {
		t.Errorf("/app.js fetched %d times with CrawlJS off", n)
	}
	if n := site.count("/logo.png"); n != 0 {
		t.Errorf("/logo.png fetched %d times with CrawlImages off", n)
	}
}

// TestCrawlIssuesOneRequestPerURL is the regression test for LibreCrawl's
// pre-flight HEAD.
//
// LibreCrawl runs session.head() before every GET whenever max_file_size > 0 —
// which is its default — so every crawl hits the user's own server twice per
// URL. Content-Length on the GET answers the same question for free.
func TestCrawlIssuesOneRequestPerURL(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	runID := crawlFixture(t, s, site, func(o *Options) {
		o.Concurrency = 1
		o.MaxFileSizeMB = DefaultMaxFileSizeMB // the condition that triggers LibreCrawl's HEAD
	})

	db, _ := s.readDB()
	var crawled int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, runID).Scan(&crawled)

	// robots.txt and sitemap.xml are fetched once each on top of the pages, and
	// a redirect chain costs one request per hop.
	total := site.totalRequests()
	if total > crawled*2 {
		t.Errorf("%d HTTP requests for %d crawled URLs — that ratio means a pre-flight request per URL", total, crawled)
	}

	// One page that nothing redirects to is the clearest single case: exactly
	// one GET, no HEAD. (/about would be a bad choice — the 301 chain ends
	// there, and following a redirect legitimately fetches its target.)
	if n := site.count("/contact"); n != 1 {
		t.Errorf("/contact was requested %d times, want exactly 1", n)
	}
}

// TestCrawlRespectsMaxDepth also documents that sitemap discovery bypasses it:
// sitemap URLs enter at depth 0 by design, which is what makes orphans
// detectable but silently widens a shallow crawl.
func TestCrawlRespectsMaxDepth(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	runID := crawlFixture(t, s, site, func(o *Options) {
		o.MaxDepth = 1
		o.DiscoverSitemaps = false
	})

	db, _ := s.readDB()
	var maxDepth int
	db.QueryRow(`SELECT COALESCE(MAX(depth), 0) FROM sitecrawl_pages WHERE run_id = ?`, runID).Scan(&maxDepth)
	if maxDepth > 1 {
		t.Errorf("crawled to depth %d with maxDepth = 1", maxDepth)
	}
}

func TestCrawlListModeDoesNotDiscover(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)

	opts := s.DefaultOptions()
	opts.Mode = ModeList
	opts.Concurrency = 2
	started, err := s.Start([]string{site.URL + "/about", site.URL + "/contact"}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, s, started.RunID, StateCompleted, StateFailed)

	db, _ := s.readDB()
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, started.RunID).Scan(&n)
	if n != 2 {
		t.Errorf("list mode crawled %d pages, want exactly the 2 it was given", n)
	}
}

func TestRowsWindowAndSearch(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	runID := crawlFixture(t, s, site, nil)

	cols := []string{"url", "status", "title", "wordCount"}
	page, err := s.Rows(RowQuery{RunID: runID, Tab: TabInternal, Cols: cols, Limit: 5, WantTotal: true})
	if err != nil {
		t.Fatalf("Rows: %v", err)
	}
	if page.Total <= 0 {
		t.Errorf("total = %d, want the internal page count", page.Total)
	}
	if len(page.Rows) == 0 || len(page.Rows) > 5 {
		t.Errorf("got %d rows for limit 5", len(page.Rows))
	}
	for _, r := range page.Rows {
		if len(r.Cells) != len(cols) {
			t.Fatalf("row has %d cells for %d requested columns", len(r.Cells), len(cols))
		}
	}

	// Paging must not repeat a row.
	first, _ := s.Rows(RowQuery{RunID: runID, Tab: TabInternal, Cols: cols, Limit: 3, Offset: 0})
	second, _ := s.Rows(RowQuery{RunID: runID, Tab: TabInternal, Cols: cols, Limit: 3, Offset: 3})
	seen := map[string]bool{}
	for _, r := range append(first.Rows, second.Rows...) {
		if seen[r.ID] {
			t.Errorf("row %s appeared on both pages — paging is unstable", r.ID)
		}
		seen[r.ID] = true
	}

	// Search reaches the URL.
	found, err := s.Rows(RowQuery{RunID: runID, Tab: TabInternal, Cols: cols, Search: "canon", Limit: 50})
	if err != nil {
		t.Fatalf("Rows(search): %v", err)
	}
	if len(found.Rows) == 0 {
		t.Error("searching for 'canon' returned nothing")
	}
	for _, r := range found.Rows {
		if !strings.Contains(r.ID, "canon") {
			t.Errorf("search returned unrelated row %s", r.ID)
		}
	}
}

func TestRowsUnknownSortFallsBack(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	runID := crawlFixture(t, s, site, nil)

	// An unknown sort key must fall back rather than error: the frontend's
	// column list is allowed to drift ahead of Go.
	if _, err := s.Rows(RowQuery{
		RunID: runID, Tab: TabInternal, Cols: []string{"url"},
		Sort: "definitely-not-a-column'; DROP TABLE sitecrawl_pages; --", Limit: 5,
	}); err != nil {
		t.Fatalf("unknown sort key errored instead of falling back: %v", err)
	}
	var n int
	db, _ := s.readDB()
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, runID).Scan(&n)
	if n == 0 {
		t.Fatal("pages table is gone — the sort key reached SQL unescaped")
	}
}

func TestInlinksAndOutlinks(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	runID := crawlFixture(t, s, site, nil)

	in, err := s.Inlinks(runID, site.URL+"/about", 0, 100)
	if err != nil {
		t.Fatalf("Inlinks: %v", err)
	}
	if len(in) == 0 {
		t.Error("/about has no recorded inlinks, but every page's nav links to it")
	}

	out, err := s.Outlinks(runID, site.URL+"/", 0, 100)
	if err != nil {
		t.Fatalf("Outlinks: %v", err)
	}
	if len(out) < 5 {
		t.Errorf("homepage has %d outlinks recorded, expected many more", len(out))
	}

	// Anchor text and placement survive to the detail pane.
	var sawAnchor, sawFooter bool
	for _, l := range out {
		if l.Anchor != "" {
			sawAnchor = true
		}
		if l.Placement == PlacementFooter {
			sawFooter = true
		}
	}
	if !sawAnchor {
		t.Error("no anchor text recorded on any outlink")
	}
	if !sawFooter {
		t.Error("no footer-placed link recorded, but the fixture has one")
	}
}

func TestNofollowRecorded(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	runID := crawlFixture(t, s, site, nil)

	out, err := s.Outlinks(runID, site.URL+"/", 0, 200)
	if err != nil {
		t.Fatalf("Outlinks: %v", err)
	}
	found := false
	for _, l := range out {
		if strings.HasSuffix(l.Target, "/noindex") && l.Flags&FlagNofollow != 0 {
			found = true
		}
	}
	if !found {
		t.Error("rel=nofollow was not recorded — LibreCrawl never reads rel at all, and this is the gap being closed")
	}
}

func TestExportCSVRoundTrip(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	runID := crawlFixture(t, s, site, nil)

	path, err := s.Export(FormatCSV, RowQuery{
		RunID: runID, Tab: TabInternal, Cols: []string{"url", "status", "title"},
	})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	if path == "" {
		t.Fatal("export returned an empty path")
	}
	data := readFile(t, path)
	if !strings.HasPrefix(data, string(runs.BOMUTF8)) {
		t.Error("CSV is missing the UTF-8 BOM Excel needs for Vietnamese")
	}
	if !strings.Contains(data, "\r\n") {
		t.Error("CSV is not using CRLF line endings")
	}
	if !strings.Contains(data, site.URL) {
		t.Error("CSV does not contain any crawled URL")
	}
}

func TestExportJSONAndXML(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	runID := crawlFixture(t, s, site, nil)

	for _, format := range []string{FormatJSON, FormatXML} {
		path, err := s.Export(format, RowQuery{
			RunID: runID, Tab: TabInternal, Cols: []string{"url", "status"},
		})
		if err != nil {
			t.Fatalf("Export(%s): %v", format, err)
		}
		data := readFile(t, path)
		if !strings.Contains(data, site.URL) {
			t.Errorf("%s export contains no crawled URL", format)
		}
	}
}
