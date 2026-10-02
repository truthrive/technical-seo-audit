package sitecrawl

import (
	"strings"
	"testing"
)

// TestLinksTabRows: the Links tab lists edges, not URLs — one row per link
// with its own fixed cell shape, filterable by internal/external/nofollow.
func TestLinksTabRows(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	runID := crawlFixture(t, s, site, nil)

	page, err := s.Rows(RowQuery{RunID: runID, Tab: TabLinks, Limit: 500, WantTotal: true})
	if err != nil {
		t.Fatalf("Rows(links): %v", err)
	}
	if page.Total <= 0 || len(page.Rows) == 0 {
		t.Fatalf("links tab returned %d rows (total %d), want the fixture's edges", len(page.Rows), page.Total)
	}
	for _, r := range page.Rows[:3] {
		if len(r.Cells) != 5 {
			t.Fatalf("link row has %d cells, want the fixed 5 [source, target, anchor, placement, targetStatus]", len(r.Cells))
		}
		if !strings.HasPrefix(r.Cells[0], "http") || r.Cells[1] == "" {
			t.Errorf("link row cells look wrong: %v", r.Cells)
		}
	}

	// The fixture's homepage carries one rel=nofollow link (to /noindex).
	nofollow, err := s.Rows(RowQuery{RunID: runID, Tab: TabLinks, Filter: "nofollow", Limit: 100, WantTotal: true})
	if err != nil {
		t.Fatalf("Rows(links, nofollow): %v", err)
	}
	if nofollow.Total < 1 {
		t.Error("nofollow filter found nothing; the fixture has a rel=nofollow link")
	}
	for _, r := range nofollow.Rows {
		if r.Flags&FlagNofollow == 0 {
			t.Errorf("row %v passed the nofollow filter without the nofollow flag", r.Cells)
		}
	}

	// External edges exist (the fixture links to external.example.org).
	external, err := s.Rows(RowQuery{RunID: runID, Tab: TabLinks, Filter: "external", Limit: 100, WantTotal: true})
	if err != nil {
		t.Fatalf("Rows(links, external): %v", err)
	}
	if external.Total < 1 {
		t.Error("external filter found nothing; the fixture links off-site")
	}

	// Facets carry the same counts for the toolbar chips.
	facets, err := s.Facets(runID)
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	if facets[TabLinks] != page.Total {
		t.Errorf("facets links total %d != rows total %d", facets[TabLinks], page.Total)
	}
	if facets[TabLinks+".nofollow"] != nofollow.Total {
		t.Errorf("facets nofollow %d != filter total %d", facets[TabLinks+".nofollow"], nofollow.Total)
	}
}

// TestIssueCodeFilter: "issue:<code>" is how the Issues tab and the Overview
// tree jump to affected URLs — it must return exactly the pages carrying that
// code, and agree with the facet count.
func TestIssueCodeFilter(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	runID := crawlFixture(t, s, site, nil)

	facets, err := s.Facets(runID)
	if err != nil {
		t.Fatalf("Facets: %v", err)
	}
	if facets["issue."+IssueTitleDuplicate] < 2 {
		t.Fatalf("fixture should have duplicate titles (dup-a/dup-b), facets = %d", facets["issue."+IssueTitleDuplicate])
	}

	page, err := s.Rows(RowQuery{
		RunID: runID, Tab: TabInternal, Filter: "issue:" + IssueTitleDuplicate,
		Cols: []string{"url", "title"}, Limit: 100, WantTotal: true,
	})
	if err != nil {
		t.Fatalf("Rows(issue filter): %v", err)
	}
	if page.Total != facets["issue."+IssueTitleDuplicate] {
		t.Errorf("issue filter total %d != facet count %d", page.Total, facets["issue."+IssueTitleDuplicate])
	}
	for _, r := range page.Rows {
		if !strings.Contains(r.ID, "/dup-") {
			t.Errorf("unexpected page in title-duplicate filter: %s", r.ID)
		}
	}
}
