package sitecrawl

import (
	"strings"
	"testing"
)

// TestCustomHeadersStayOnTheSeedSite pins the scope of the Custom Headers
// option. It is where a user puts a Cookie or Authorization to crawl a site
// behind a login, and this crawler both walks redirect chains by hand and
// fetches external URLs by default — so an unscoped header reaches every host a
// crawled page points at, including one a 301 sends it to.
func TestCustomHeadersStayOnTheSeedSite(t *testing.T) {
	opts := Options{
		CustomHeaders: map[string]string{"Cookie": "session=secret"},
	}.normalized()
	f := newFetcher(opts, nil, "example.com", true)

	same := []string{
		"https://example.com/page",
		"http://example.com/other",
		"https://example.com:8443/port",
	}
	for _, u := range same {
		if got := f.customHeadersFor(u); len(got) == 0 {
			t.Errorf("customHeadersFor(%q) dropped the headers for the site being crawled", u)
		}
	}

	other := []string{
		"https://attacker.tld/",          // where a 301 would send the chain
		"https://cdn.other.com/logo.png", // an ordinary external resource
		"https://sub.example.com/page",   // subdomains are off by default
		"not a url",
	}
	for _, u := range other {
		if got := f.customHeadersFor(u); len(got) != 0 {
			t.Errorf("customHeadersFor(%q) leaked %v to a host that is not the crawl target", u, got)
		}
	}

	// Turning on subdomain crawling widens the site, and the credential with it.
	opts.CrawlSubdomains = true
	f = newFetcher(opts, nil, "example.com", true)
	if got := f.customHeadersFor("https://sub.example.com/page"); len(got) == 0 {
		t.Error("with CrawlSubdomains on, a subdomain is part of the site and should get the headers")
	}
}

// TestCustomHeadersAreNotPersisted: run options are stored as one JSON blob and
// handed straight back to the frontend by Runs(), so anything secret in them
// lands in cleartext on disk and becomes readable from JS.
func TestCustomHeadersAreNotPersisted(t *testing.T) {
	s, db, _ := newTestService(t)
	opts := s.DefaultOptions()
	opts.CustomHeaders = map[string]string{"Authorization": "Bearer super-secret"}

	if err := insertRun(db, "run1", "https://example.com/", "example.com", opts); err != nil {
		t.Fatalf("insertRun: %v", err)
	}

	var blob string
	if err := db.QueryRow(`SELECT options FROM sitecrawl_runs WHERE id = ?`, "run1").Scan(&blob); err != nil {
		t.Fatalf("read options: %v", err)
	}
	if strings.Contains(blob, "super-secret") {
		t.Errorf("the stored options blob contains the credential: %s", blob)
	}

	run, err := loadRun(db, "run1")
	if err != nil {
		t.Fatalf("loadRun: %v", err)
	}
	if len(run.Options.CustomHeaders) != 0 {
		t.Errorf("loadRun handed %v back to the frontend", run.Options.CustomHeaders)
	}

	list, err := listRuns(db)
	if err != nil {
		t.Fatalf("listRuns: %v", err)
	}
	for _, r := range list {
		if len(r.Options.CustomHeaders) != 0 {
			t.Errorf("listRuns handed %v back to the frontend", r.Options.CustomHeaders)
		}
	}
}

// TestLoadRunRedactsLegacyHeaders covers databases written before insertRun
// started stripping: the rows are already on disk, so the read path has to
// redact them too.
func TestLoadRunRedactsLegacyHeaders(t *testing.T) {
	s, db, _ := newTestService(t)
	if err := insertRun(db, "old", "https://example.com/", "example.com", s.DefaultOptions()); err != nil {
		t.Fatalf("insertRun: %v", err)
	}
	legacy := `{"customHeaders":{"Cookie":"session=leaked"},"mode":"spider"}`
	if _, err := db.Exec(`UPDATE sitecrawl_runs SET options = ? WHERE id = ?`, legacy, "old"); err != nil {
		t.Fatalf("stage legacy row: %v", err)
	}

	run, err := loadRun(db, "old")
	if err != nil {
		t.Fatalf("loadRun: %v", err)
	}
	if len(run.Options.CustomHeaders) != 0 {
		t.Errorf("loadRun returned a legacy credential: %v", run.Options.CustomHeaders)
	}
}
