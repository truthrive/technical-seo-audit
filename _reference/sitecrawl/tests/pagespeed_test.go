package sitecrawl

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"onescout/desktop/internal/core/credset"
	"onescout/desktop/internal/core/httpx"
)

// psiFixture is a PSI v5 response with everything this tool reads: all four
// Lighthouse categories, a full CrUX block, and three audits of which only two
// are opportunities worth listing.
const psiFixture = `{
  "loadingExperience": {
    "overall_category": "AVERAGE",
    "metrics": {
      "LARGEST_CONTENTFUL_PAINT_MS": {"percentile": 2800, "category": "AVERAGE"},
      "INTERACTION_TO_NEXT_PAINT": {"percentile": 210, "category": "AVERAGE"},
      "CUMULATIVE_LAYOUT_SHIFT_SCORE": {"percentile": 12, "category": "AVERAGE"},
      "FIRST_CONTENTFUL_PAINT_MS": {"percentile": 1900, "category": "AVERAGE"},
      "EXPERIMENTAL_TIME_TO_FIRST_BYTE": {"percentile": 800, "category": "AVERAGE"}
    }
  },
  "originLoadingExperience": {
    "overall_category": "FAST",
    "metrics": {"LARGEST_CONTENTFUL_PAINT_MS": {"percentile": 1200, "category": "FAST"}}
  },
  "lighthouseResult": {
    "categories": {
      "performance":    {"score": 0.62},
      "accessibility":  {"score": 0.88},
      "best-practices": {"score": 0.75},
      "seo":            {"score": 1}
    },
    "audits": {
      "first-contentful-paint":    {"numericValue": 1500},
      "largest-contentful-paint":  {"numericValue": 3100},
      "cumulative-layout-shift":   {"numericValue": 0.04},
      "total-blocking-time":       {"numericValue": 320},
      "speed-index":               {"numericValue": 2600},
      "uses-optimized-images":     {"title": "Efficiently encode images",
        "details": {"type": "opportunity", "overallSavingsMs": 900, "overallSavingsBytes": 340000}},
      "render-blocking-resources": {"title": "Eliminate render-blocking resources",
        "details": {"type": "opportunity", "overallSavingsMs": 600, "overallSavingsBytes": 0}},
      "unused-css-rules":          {"title": "Reduce unused CSS",
        "details": {"type": "opportunity", "overallSavingsMs": 0, "overallSavingsBytes": 0}},
      "uses-long-cache-ttl":       {"title": "Serve static assets with an efficient cache policy",
        "details": {"type": "table", "overallSavingsBytes": 99999}}
    }
  }
}`

// psiServer stands in for Google. body is returned for every request; the
// handler records what was asked for.
func psiServer(t *testing.T, body string) (*httptest.Server, *int32, *[]string) {
	t.Helper()
	var calls int32
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		mu.Lock()
		seen = append(seen, r.URL.Query().Get("url"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(body))
	}))
	old := psiEndpoint
	psiEndpoint = srv.URL
	t.Cleanup(func() {
		psiEndpoint = old
		srv.Close()
	})
	return srv, &calls, &seen
}

func TestFetchPSIParsesAllCategories(t *testing.T) {
	psiServer(t, psiFixture)

	r := fetchPSI(context.Background(), http.DefaultClient, "k", "https://example.com/", StrategyMobile)
	if r.Error != "" {
		t.Fatalf("unexpected error: %s", r.Error)
	}

	for _, c := range []struct {
		name string
		got  int
		want int
	}{
		{"performance", r.Score, 62},
		{"accessibility", r.A11yScore, 88},
		{"best-practices", r.BPScore, 75},
		{"seo", r.SEOScore, 100},
	} {
		if c.got != c.want {
			t.Errorf("%s score = %d, want %d", c.name, c.got, c.want)
		}
	}

	// Lab numbers must survive the rewrite unchanged.
	if r.LCPMs != 3100 || r.TBTMs != 320 || r.SIMs != 2600 {
		t.Errorf("lab metrics = lcp %d tbt %d si %d", r.LCPMs, r.TBTMs, r.SIMs)
	}

	if r.CruxSource != "url" {
		t.Errorf("cruxSource = %q, want url", r.CruxSource)
	}
	if r.CruxVerdict != "AVERAGE" {
		t.Errorf("cruxVerdict = %q, want AVERAGE", r.CruxVerdict)
	}
	if r.CruxLCPMs != 2800 || r.CruxINPMs != 210 || r.CruxFCPMs != 1900 || r.CruxTTFBMs != 800 {
		t.Errorf("crux = lcp %d inp %d fcp %d ttfb %d",
			r.CruxLCPMs, r.CruxINPMs, r.CruxFCPMs, r.CruxTTFBMs)
	}
	// CrUX reports CLS as an integer hundred times the score.
	if r.CruxCLS != 0.12 {
		t.Errorf("cruxCls = %v, want 0.12", r.CruxCLS)
	}

	// Two of the four audits qualify: one saves nothing, one is not an
	// opportunity at all.
	if len(r.Opps) != 2 {
		t.Fatalf("opportunities = %d, want 2: %+v", len(r.Opps), r.Opps)
	}
	byID := map[string]Opportunity{}
	for _, o := range r.Opps {
		byID[o.AuditID] = o
	}
	if o := byID["uses-optimized-images"]; o.SavingsMs != 900 || o.SavingsBytes != 340000 ||
		o.Title != "Efficiently encode images" {
		t.Errorf("uses-optimized-images = %+v", o)
	}
	if _, ok := byID["unused-css-rules"]; ok {
		t.Error("an opportunity saving nothing was listed")
	}
	if _, ok := byID["uses-long-cache-ttl"]; ok {
		t.Error("a diagnostic was listed as an opportunity")
	}
}

func TestFetchPSIRequestsFourCategories(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()["category"]
		w.Write([]byte(psiFixture))
	}))
	defer srv.Close()
	old := psiEndpoint
	psiEndpoint = srv.URL
	defer func() { psiEndpoint = old }()

	fetchPSI(context.Background(), http.DefaultClient, "k", "https://example.com/", StrategyMobile)
	if len(got) != 4 {
		t.Fatalf("category params = %v, want all four", got)
	}
}

// A page with too little traffic has no field data of its own. Borrowing the
// origin's is fine as long as the result says so.
func TestFetchPSIOriginFallback(t *testing.T) {
	const body = `{
	  "loadingExperience": {"metrics": {}},
	  "originLoadingExperience": {
	    "overall_category": "FAST",
	    "metrics": {"LARGEST_CONTENTFUL_PAINT_MS": {"percentile": 1200}}
	  },
	  "lighthouseResult": {"categories": {"performance": {"score": 0.9}}, "audits": {}}
	}`
	psiServer(t, body)

	r := fetchPSI(context.Background(), http.DefaultClient, "k", "https://example.com/x", StrategyMobile)
	if r.CruxSource != "origin" {
		t.Errorf("cruxSource = %q, want origin", r.CruxSource)
	}
	if r.CruxLCPMs != 1200 {
		t.Errorf("cruxLcpMs = %d, want 1200", r.CruxLCPMs)
	}
	// Absent inside a present block still means "not measured", not zero.
	if r.CruxINPMs != -1 {
		t.Errorf("cruxInpMs = %d, want -1", r.CruxINPMs)
	}
}

func TestFetchPSINoFieldData(t *testing.T) {
	const body = `{"lighthouseResult": {"categories": {"performance": {"score": 0.5}}, "audits": {}}}`
	psiServer(t, body)

	r := fetchPSI(context.Background(), http.DefaultClient, "k", "https://example.com/", StrategyMobile)
	if r.CruxSource != "" {
		t.Errorf("cruxSource = %q, want empty", r.CruxSource)
	}
	for name, v := range map[string]int{
		"lcp": r.CruxLCPMs, "inp": r.CruxINPMs, "fcp": r.CruxFCPMs, "ttfb": r.CruxTTFBMs,
	} {
		if v != -1 {
			t.Errorf("crux %s = %d, want -1", name, v)
		}
	}
	if r.CruxCLS != -1 {
		t.Errorf("cruxCls = %v, want -1", r.CruxCLS)
	}
	// A category Google did not run must not read as a real zero.
	if r.A11yScore != -1 || r.SEOScore != -1 || r.BPScore != -1 {
		t.Errorf("absent categories = a11y %d seo %d bp %d", r.A11yScore, r.SEOScore, r.BPScore)
	}
}

// insertPage puts one row straight into the page table, which is all the pump's
// queue query reads.
func insertPage(t *testing.T, db *sql.DB, runID string, id int, url, kind string, statusClass, internal, inlinks int) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, is_internal, status, status_class, inlinks, crawled_at)
		VALUES (?, ?, ?, '{}', ?, ?, ?, ?, ?, '')`,
		runID, id, url, kind, internal, statusClass*100, statusClass, inlinks)
	if err != nil {
		t.Fatalf("insert page: %v", err)
	}
}

// The pump must measure internal HTML pages that answered, and nothing else:
// an image or a 404 would cost 20 seconds to learn nothing.
func TestPSIPumpMeasuresPendingOnly(t *testing.T) {
	s, db, _ := newTestService(t)
	_, calls, seen := psiServer(t, psiFixture)

	const runID = "run-pending"
	insertPage(t, db, runID, 1, "https://example.com/", "html", 2, 1, 10)
	insertPage(t, db, runID, 2, "https://example.com/blog", "html", 2, 1, 5)
	insertPage(t, db, runID, 3, "https://example.com/old", "html", 2, 1, 1)
	insertPage(t, db, runID, 4, "https://example.com/a.png", "image", 2, 1, 3)
	insertPage(t, db, runID, 5, "https://example.com/gone", "html", 4, 1, 2)
	insertPage(t, db, runID, 6, "https://other.com/", "html", 2, 0, 9)
	// /old was measured on an earlier pass and must not be measured again.
	savePSI(db, runID, PSIResult{URL: "https://example.com/old", Strategy: StrategyMobile, Score: 80})

	read, err := s.readDB()
	if err != nil {
		t.Fatalf("readDB: %v", err)
	}
	if n, _ := countMeasurable(read, runID); n != 3 {
		t.Fatalf("measurable = %d, want 3 (html, internal, 2xx)", n)
	}

	var mu sync.Mutex
	var saved []string
	pump := newPSIPump(read, runID, StrategyMobile, "k", http.DefaultClient, nil,
		func(r PSIResult) {
			mu.Lock()
			saved = append(saved, r.URL)
			mu.Unlock()
		})
	pump.Seal()
	pump.Start(context.Background(), nil)
	pump.Wait()

	if got := atomic.LoadInt32(calls); got != 2 {
		t.Errorf("PSI calls = %d, want 2; asked for %v", got, *seen)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(saved) != 2 {
		t.Fatalf("saved = %v, want the two unmeasured pages", saved)
	}
	for _, u := range saved {
		if u == "https://example.com/a.png" || u == "https://example.com/gone" ||
			u == "https://other.com/" || u == "https://example.com/old" {
			t.Errorf("measured a page it should have skipped: %s", u)
		}
	}
}

// A URL Google keeps rejecting stays in the pending query for ever. Within one
// pump it must be tried once, or the run never ends.
func TestPSIPumpDoesNotLoopOnFailure(t *testing.T) {
	s, db, _ := newTestService(t)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"message":"quota"}}`))
	}))
	defer srv.Close()
	old := psiEndpoint
	psiEndpoint = srv.URL
	defer func() { psiEndpoint = old }()

	const runID = "run-fail"
	insertPage(t, db, runID, 1, "https://example.com/", "html", 2, 1, 1)

	read, _ := s.readDB()
	pump := newPSIPump(read, runID, StrategyMobile, "k", http.DefaultClient, nil,
		func(r PSIResult) { savePSI(db, runID, r) })
	pump.Seal()

	done := make(chan struct{})
	go func() { pump.Start(context.Background(), nil); pump.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		pump.Stop()
		t.Fatal("pump never finished — a permanently failing URL was retried in a loop")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1", got)
	}
}

// Stop must retire every goroutine; a pump per crawl that leaks ten workers
// would accumulate for the life of the app.
func TestPSIPumpStopIsClean(t *testing.T) {
	s, db, _ := newTestService(t)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
		w.Write([]byte(psiFixture))
	}))
	defer srv.Close()
	old := psiEndpoint
	psiEndpoint = srv.URL
	defer func() { psiEndpoint = old }()

	const runID = "run-stop"
	for i := 1; i <= 30; i++ {
		insertPage(t, db, runID, i, "https://example.com/p"+string(rune('a'+i%26)), "html", 2, 1, i)
	}

	before := runtime.NumGoroutine()
	read, _ := s.readDB()
	pump := newPSIPump(read, runID, StrategyMobile, "k", http.DefaultClient, nil, func(PSIResult) {})
	pump.Start(context.Background(), nil)
	time.Sleep(100 * time.Millisecond)

	pump.Stop()
	close(release)
	waited := make(chan struct{})
	go func() { pump.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-time.After(10 * time.Second):
		t.Fatal("Wait did not return after Stop")
	}

	// Stop twice must not panic on a closed channel.
	pump.Stop()

	time.Sleep(200 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before+3 {
		t.Errorf("goroutines %d -> %d, workers leaked", before, after)
	}
}

// A resumed run must show 50/76, not start the bar over at zero.
func TestPSIPumpProgressCountsPriorWork(t *testing.T) {
	s, db, _ := newTestService(t)
	const runID = "run-progress"
	for i := 1; i <= 4; i++ {
		insertPage(t, db, runID, i, "https://example.com/p"+string(rune('a'+i)), "html", 2, 1, i)
	}
	savePSI(db, runID, PSIResult{URL: "https://example.com/pb", Strategy: StrategyMobile})
	savePSI(db, runID, PSIResult{URL: "https://example.com/pc", Strategy: StrategyMobile})

	read, _ := s.readDB()
	pump := newPSIPump(read, runID, StrategyMobile, "k", http.DefaultClient, nil, func(PSIResult) {})
	pump.refill()
	done, total := pump.progressNumbers()
	if done != 2 || total != 4 {
		t.Errorf("progress = %d/%d, want 2/4", done, total)
	}

	if n, err := s.PendingPageSpeedCount(runID, StrategyMobile); err != nil || n != 2 {
		t.Errorf("PendingPageSpeedCount = %d (%v), want 2", n, err)
	}
}

// An errored row is not a measurement: a later Run has to pick it up again.
func TestPendingIgnoresErroredRows(t *testing.T) {
	s, db, _ := newTestService(t)
	const runID = "run-errored"
	insertPage(t, db, runID, 1, "https://example.com/", "html", 2, 1, 1)
	savePSI(db, runID, PSIResult{URL: "https://example.com/", Strategy: StrategyMobile, Error: "HTTP 429: quota"})

	read, _ := s.readDB()
	urls, err := pendingPageSpeed(read, runID, StrategyMobile, 10)
	if err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(urls) != 1 {
		t.Errorf("pending = %v, want the errored URL back", urls)
	}
}

func TestOpportunitiesAggregate(t *testing.T) {
	s, db, _ := newTestService(t)
	const runID = "run-opps"
	insertPage(t, db, runID, 1, "https://example.com/a", "html", 2, 1, 1)
	insertPage(t, db, runID, 2, "https://example.com/b", "html", 2, 1, 1)

	savePSI(db, runID, PSIResult{URL: "https://example.com/a", Strategy: StrategyMobile, Opps: []Opportunity{
		{AuditID: "uses-optimized-images", Title: "Efficiently encode images", SavingsMs: 900, SavingsBytes: 340000},
		{AuditID: "render-blocking-resources", Title: "Eliminate render-blocking", SavingsMs: 600},
	}})
	savePSI(db, runID, PSIResult{URL: "https://example.com/b", Strategy: StrategyMobile, Opps: []Opportunity{
		{AuditID: "uses-optimized-images", Title: "Efficiently encode images", SavingsMs: 400, SavingsBytes: 120000},
	}})

	list, err := s.Opportunities(runID)
	if err != nil {
		t.Fatalf("Opportunities: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("opportunities = %d, want 2", len(list))
	}
	top := list[0]
	if top.AuditID != "uses-optimized-images" {
		t.Errorf("heaviest audit = %s", top.AuditID)
	}
	if top.Pages != 2 || top.SavingsMs != 1300 || top.SavingsBytes != 460000 {
		t.Errorf("aggregate = %+v", top)
	}

	pages, err := s.OpportunityPages(runID, "uses-optimized-images")
	if err != nil {
		t.Fatalf("OpportunityPages: %v", err)
	}
	if len(pages) != 2 || pages[0].URL != "https://example.com/a" {
		t.Errorf("pages = %+v", pages)
	}
}

// Re-measuring must drop the opportunities the user has already fixed.
func TestSavePSIReplacesOpportunities(t *testing.T) {
	_, db, _ := newTestService(t)
	const runID = "run-replace"
	savePSI(db, runID, PSIResult{URL: "https://example.com/a", Strategy: StrategyMobile, Opps: []Opportunity{
		{AuditID: "uses-optimized-images", SavingsMs: 900},
		{AuditID: "render-blocking-resources", SavingsMs: 600},
	}})
	savePSI(db, runID, PSIResult{URL: "https://example.com/a", Strategy: StrategyMobile, Opps: []Opportunity{
		{AuditID: "uses-optimized-images", SavingsMs: 100},
	}})

	var n int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_psi_opps WHERE run_id = ?`, runID).Scan(&n)
	if n != 1 {
		t.Errorf("opportunity rows = %d, want 1 after the re-measure", n)
	}
}

// The whole point of the feature: turn the option on, crawl, and the
// measurements are simply there — no button, no selection, like Screaming Frog.
func TestCrawlMeasuresPageSpeedAutomatically(t *testing.T) {
	fastCrawl(t)
	// The pump's own cadence has to shrink too, or the crawl finishes and the
	// drain waits out the 2s refill tick several times.
	oldRefill, oldProgress := psiRefillEvery, psiProgressEvery
	psiRefillEvery, psiProgressEvery = 50*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { psiRefillEvery, psiProgressEvery = oldRefill, oldProgress })

	site := newFixtureSite(t)
	s, _, emitter := newTestService(t)
	_, calls, _ := psiServer(t, psiFixture)

	// The key lives in Connections, so store it the way Connections does.
	if _, err := credset.Add(s.store().(*fakeVault), VaultKeyPageSpeed, "", "AIzaTest"); err != nil {
		t.Fatalf("add key: %v", err)
	}

	runID := crawlFixture(t, s, site, func(o *Options) {
		o.EnablePageSpeed = true
		o.PSIStrategy = StrategyMobile
	})

	results, err := s.PageSpeedResults(runID)
	if err != nil {
		t.Fatalf("PageSpeedResults: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("the crawl finished with no PageSpeed measurements at all")
	}
	if got := int(atomic.LoadInt32(calls)); got != len(results) {
		t.Errorf("%d PSI calls produced %d stored rows", got, len(results))
	}

	// Every measured URL must be an internal HTML page of this run.
	pending, err := s.PendingPageSpeedCount(runID, StrategyMobile)
	if err != nil {
		t.Fatalf("PendingPageSpeedCount: %v", err)
	}
	if pending != 0 {
		t.Errorf("%d pages left unmeasured after a completed crawl", pending)
	}

	// The parsed fields survived the trip through the coordinator's buffer.
	r := results[0]
	if r.A11yScore != 88 || r.CruxINPMs != 210 || r.CruxSource != "url" {
		t.Errorf("stored row lost its new fields: %+v", r)
	}

	// Opportunities landed in their own table.
	opps, err := s.Opportunities(runID)
	if err != nil {
		t.Fatalf("Opportunities: %v", err)
	}
	if len(opps) != 2 {
		t.Errorf("opportunities = %d, want 2", len(opps))
	}

	// The status strip needs the progress event to have been emitted.
	if emitter.count(EventPSIProgress) == 0 {
		t.Error("no sitecrawl:psi-progress event — the PageSpeed bar would never appear")
	}
}

// With the option off, nothing must reach Google.
func TestCrawlSkipsPageSpeedWhenOff(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	_, calls, _ := psiServer(t, psiFixture)
	credset.Add(s.store().(*fakeVault), VaultKeyPageSpeed, "", "AIzaTest")

	runID := crawlFixture(t, s, site, nil)

	if got := atomic.LoadInt32(calls); got != 0 {
		t.Errorf("PSI was called %d times with the option off", got)
	}
	if results, _ := s.PageSpeedResults(runID); len(results) != 0 {
		t.Errorf("stored %d measurements with the option off", len(results))
	}
}

// A key that is not there must not fail the crawl — it must just skip the pass.
func TestCrawlWithoutKeyStillCrawls(t *testing.T) {
	fastCrawl(t)
	site := newFixtureSite(t)
	s, _, _ := newTestService(t)
	_, calls, _ := psiServer(t, psiFixture)

	runID := crawlFixture(t, s, site, func(o *Options) { o.EnablePageSpeed = true })

	if got := atomic.LoadInt32(calls); got != 0 {
		t.Errorf("PSI was called %d times without a key", got)
	}
	facets, err := s.Facets(runID)
	if err != nil || facets[TabInternal] < 12 {
		t.Errorf("the crawl itself was damaged: %v %v", facets, err)
	}
}

// PageSpeed must NOT go through the crawl proxy.
//
// The proxy is there so the crawled site sees a different IP; PageSpeed is a
// first-party API call Google authenticates by key. Sending it through the
// proxy once held a proxy connection for ~20s per measurement and timed out 56
// of 58 of them. This pins the decision: with an unroutable proxy configured,
// a PageSpeed call must still succeed.
// Asserted on the transport, not on a live request: httpx bypasses the proxy
// for localhost, so a test server on 127.0.0.1 succeeds whether the proxy is
// wired in or not — an earlier version of this test passed either way and
// proved nothing.
func TestPageSpeedIgnoresCrawlProxy(t *testing.T) {
	s, _, _ := newTestService(t)
	v := s.store().(*fakeVault)
	if _, err := credset.Add(v, VaultKeyProxy, "", "http://127.0.0.1:9"); err != nil {
		t.Fatalf("add proxy: %v", err)
	}

	// The API host, deliberately not a local address.
	req, err := http.NewRequest(http.MethodGet, "https://www.googleapis.com/pagespeedonline/v5/runPagespeed", nil)
	if err != nil {
		t.Fatal(err)
	}

	psiTr, ok := psiClient().Transport.(*http.Transport)
	if !ok {
		t.Fatal("psiClient transport is not *http.Transport")
	}
	if psiTr.Proxy != nil {
		if u, _ := psiTr.Proxy(req); u != nil {
			t.Errorf("PageSpeed would go through the crawl proxy (%s)", u)
		}
	}

	// The crawl's own client MUST still use it, otherwise this test would pass
	// simply because the proxy never loads and the assertion above is empty.
	crawlTr, ok := httpx.New(time.Second, func() *url.URL { return s.proxy() }).Transport.(*http.Transport)
	if !ok {
		t.Fatal("crawl transport is not *http.Transport")
	}
	if crawlTr.Proxy == nil {
		t.Fatal("test setup is wrong: the crawl client has no proxy to bypass")
	}
	if u, _ := crawlTr.Proxy(req); u == nil {
		t.Fatal("test setup is wrong: the crawl proxy did not load from the vault")
	}
}

// The new columns have to exist on a database built from the full migration.
func TestSchemaHasPageSpeedColumns(t *testing.T) {
	_, db, _ := newTestService(t)
	var (
		a11y, inp int
		cls       float64
	)
	err := db.QueryRow(`SELECT a11y_score, crux_inp_ms, crux_cls FROM sitecrawl_psi LIMIT 1`).
		Scan(&a11y, &inp, &cls)
	if err != nil && err != sql.ErrNoRows {
		t.Fatalf("new columns missing: %v", err)
	}
}
