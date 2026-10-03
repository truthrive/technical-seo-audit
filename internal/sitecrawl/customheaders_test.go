package sitecrawl

import (
	"strings"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

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
			t.Errorf("customHeadersFor(%q) dropped headers for the site being crawled", u)
		}
	}

	other := []string{
		"https://attacker.tld/",          // where a 301 would send the chain
		"https://cdn.other.com/logo.png", // external resource
		"https://sub.example.com/page",   // subdomains off by default
		"not a url",
	}
	for _, u := range other {
		if got := f.customHeadersFor(u); len(got) != 0 {
			t.Errorf("customHeadersFor(%q) leaked %v to host that is not the crawl target", u, got)
		}
	}

	opts.CrawlSubdomains = true
	f = newFetcher(opts, nil, "example.com", true)
	if got := f.customHeadersFor("https://sub.example.com/page"); len(got) == 0 {
		t.Error("with CrawlSubdomains on, subdomain should receive custom headers")
	}
}

func TestCustomHeadersAreNotPersisted(t *testing.T) {
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	opts := Options{
		Mode:          ModeSpider,
		CustomHeaders: map[string]string{"Authorization": "Bearer super-secret"},
	}

	if err := insertRun(db, "run1", "https://example.com/", "example.com", opts); err != nil {
		t.Fatalf("insertRun: %v", err)
	}

	var blob string
	if err := db.QueryRow(`SELECT options FROM sitecrawl_runs WHERE id = ?`, "run1").Scan(&blob); err != nil {
		t.Fatalf("read options blob: %v", err)
	}
	if strings.Contains(blob, "super-secret") {
		t.Errorf("the stored options blob contains the secret credential: %s", blob)
	}

	run, err := runner.LoadRun("run1")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if len(run.Options.CustomHeaders) != 0 {
		t.Errorf("LoadRun returned custom headers: %v", run.Options.CustomHeaders)
	}

	list, err := runner.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	for _, r := range list {
		if len(r.Options.CustomHeaders) != 0 {
			t.Errorf("ListRuns returned custom headers: %v", r.Options.CustomHeaders)
		}
	}
}

func TestLoadRunRedactsLegacyHeaders(t *testing.T) {
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	opts := Options{Mode: ModeSpider}
	if err := insertRun(db, "old", "https://example.com/", "example.com", opts); err != nil {
		t.Fatalf("insertRun: %v", err)
	}

	legacy := `{"customHeaders":{"Cookie":"session=leaked"},"mode":"spider"}`
	if _, err := db.Exec(`UPDATE sitecrawl_runs SET options = ? WHERE id = ?`, legacy, "old"); err != nil {
		t.Fatalf("stage legacy row: %v", err)
	}

	run, err := runner.LoadRun("old")
	if err != nil {
		t.Fatalf("LoadRun: %v", err)
	}
	if len(run.Options.CustomHeaders) != 0 {
		t.Errorf("LoadRun returned a legacy credential: %v", run.Options.CustomHeaders)
	}

	list, err := runner.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	for _, r := range list {
		if r.ID == "old" && len(r.Options.CustomHeaders) != 0 {
			t.Errorf("ListRuns returned a legacy credential: %v", r.Options.CustomHeaders)
		}
	}
}
