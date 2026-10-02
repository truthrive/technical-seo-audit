package sitecrawl

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"onescout/desktop/internal/core/credset"
	"onescout/desktop/internal/core/httpx"

	"onescout/desktop/internal/core/safe"
	"onescout/desktop/internal/core/workspace"
)

// VaultKeyPageSpeed is where the user's PSI API key lives. The frontend only
// ever writes it; results come back through this service.
const VaultKeyPageSpeed = "pagespeed_api_key"

// psiConcurrency: the quota shown in Google Cloud Console for
// pagespeedonline.googleapis.com is 25,000 queries/day and 100 queries per 100
// seconds. One query is a real Lighthouse pass taking ~20s, so N workers issue
// N/20 queries a second: ten in flight is 50 per 100s — half the burst budget,
// and 1,440 a day at full tilt against a 25,000 ceiling.
//
// The ceiling was never the binding constraint. What actually broke a run was
// sending these through the user's crawl proxy (see psiClient).
const psiConcurrency = 10

// psiClient builds the HTTP client for PageSpeed. Deliberately WITHOUT the
// crawl proxy.
//
// A proxy exists so the site being crawled sees a different IP. PageSpeed is
// not a crawl request: it is a first-party call to Google authenticated by the
// user's API key, and Google identifies the caller by that key, not by address.
// Routing it through the proxy buys nothing and costs a lot — each measurement
// holds a proxy connection for ~20s, so ten of them alongside the crawler's own
// threads saturated one user's proxy and timed out 56 of 58 measurements.
//
// This function existed as `httpx.New(60s, s.proxy)` and was harmless only
// because s.proxy() was returning nil for everyone: it read the vault value
// raw, and Connections stores a credset list. Fixing that read turned a dead
// line into a live one, which is how the fault surfaced.
func psiClient() *http.Client {
	return httpx.New(60*time.Second, nil)
}

var psiEndpoint = "https://www.googleapis.com/pagespeedonline/v5/runPagespeed"

var errNoPSIKey = errors.New("missing-key")

// psiCategories are all four Lighthouse categories. Google returns them in the
// same response for the same ~20s of work, so asking for one and discarding the
// rest was throwing away three columns for free.
var psiCategories = []string{"performance", "accessibility", "best-practices", "seo"}

// PSIResult is one URL's PageSpeed verdict, as stored and as listed.
//
// Two kinds of number live here and must not be confused. The Lighthouse ones
// (Score, FCPMs…) are a simulated load Google runs on demand. The Crux ones are
// what real Chrome users measured over the last 28 days — that is the data
// Google ranks on, and it is absent for pages without enough traffic.
//
// -1 means "not measured" throughout, which is not 0: a page nobody visits and
// a page that genuinely scored 0 have to stay distinguishable.
type PSIResult struct {
	URL       string `json:"url"`
	Strategy  string `json:"strategy"`
	FetchedAt string `json:"fetchedAt"`

	// lab (Lighthouse)
	Score     int     `json:"score"` // performance, 0-100
	A11yScore int     `json:"a11yScore"`
	SEOScore  int     `json:"seoScore"`
	BPScore   int     `json:"bpScore"`
	FCPMs     int     `json:"fcpMs"`
	LCPMs     int     `json:"lcpMs"`
	CLS       float64 `json:"cls"`
	TBTMs     int     `json:"tbtMs"`
	SIMs      int     `json:"siMs"`

	// field (Chrome User Experience Report, 28 days)
	//
	// CruxSource says whose numbers these are: "url" for this page, "origin"
	// when the page itself lacked data and the whole domain's were used
	// instead. Showing borrowed numbers as if they were the page's own is the
	// one thing this must never do.
	CruxSource  string  `json:"cruxSource"`  // url | origin | ""
	CruxVerdict string  `json:"cruxVerdict"` // FAST | AVERAGE | SLOW | ""
	CruxLCPMs   int     `json:"cruxLcpMs"`
	CruxINPMs   int     `json:"cruxInpMs"`
	CruxCLS     float64 `json:"cruxCls"`
	CruxFCPMs   int     `json:"cruxFcpMs"`
	CruxTTFBMs  int     `json:"cruxTtfbMs"`

	Error string `json:"error,omitempty"`

	// Opps is stored in its own table and never sent to the grid: one URL can
	// carry ~20 of them, and the PageSpeed tab shows scores, not fixes.
	Opps []Opportunity `json:"-"`
}

// PSIEvent is one finished measurement, emitted as sitecrawl:psi-result.
// Per-item events are fine here — a batch is ≤50 slow items, not 50k fast ones.
type PSIEvent struct {
	RunID  string    `json:"runId"`
	Result PSIResult `json:"result"`
}

// psiKey returns the key to spend, which Connections owns like every other
// credential in this app. It must be read through credset: Connections stores a
// labelled list, so a plain Get() here returns the JSON envelope and sends
// *that* to Google as the key — every measurement then fails with "API key not
// valid" while the key sits in Connections looking perfectly fine.
func (s *Service) psiKey() (string, bool) {
	st := s.store()
	if st == nil {
		return "", false
	}
	c, ok := credset.First(credset.Load(st, VaultKeyPageSpeed))
	if !ok {
		return "", false
	}
	return strings.TrimSpace(c.Secret), true
}

// PageSpeed measures the given URLs as a background job and returns its id.
func (s *Service) PageSpeed(runID string, urls []string, strategy string) (string, error) {
	if err := s.Licence.Check(); err != nil {
		return "", err
	}
	_, db, err := s.workspace()
	if err != nil {
		return "", err
	}
	read, err := s.readDB()
	if err != nil {
		return "", err
	}
	key, ok := s.psiKey()
	if !ok || key == "" {
		return "", errNoPSIKey
	}
	if strategy != StrategyDesktop {
		strategy = StrategyMobile
	}
	urls = dedupe(urls)

	// A second Run while the first is still going would double every
	// measurement and spend the user's quota twice.
	if existing := s.psiJobID(runID); existing != "" {
		return existing, nil
	}

	client := psiClient()
	pj := &psiJob{}
	s.setPSIJob(runID, pj)

	jobID := s.Jobs.Start(func(ctx context.Context, report func(done, total int, message string)) error {
		defer s.clearPSIJob(runID)

		// No crawl is running here, so this pump owns the write handle. It is
		// also sealed from the start: nothing new will be crawled while it works.
		pump := newPSIPump(read, runID, strategy, key, client,
			func(name string, data any) { s.Jobs.Emit(name, data) },
			func(r PSIResult) {
				savePSI(db, runID, r)
				s.Jobs.Emit(EventPSI, PSIEvent{RunID: runID, Result: r})
			})
		if len(urls) > 0 {
			pump.withURLs(urls)
		} else {
			pump.Seal()
		}
		pj.set(pump)

		pump.Start(ctx, nil)
		// Keep the shared job list honest while the pump reports the detail
		// through its own event.
		stopReport := make(chan struct{})
		// Outlives the crawl by hours, so a panic here would take the app down
		// long after the user stopped watching.
		safe.Go("sitecrawl/psiProgress", func() {
			t := time.NewTicker(psiProgressEvery)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					done, total := pump.progressNumbers()
					report(done, total, "")
				case <-stopReport:
					return
				}
			}
		})
		pump.Wait()
		close(stopReport)
		return ctx.Err()
	})
	pj.setJobID(jobID)
	return jobID, nil
}

// psiJob is the standalone PageSpeed pass, tracked so StopPageSpeed can reach
// a pump that was created inside the job goroutine.
type psiJob struct {
	mu    sync.Mutex
	jobID string
	pump  *psiPump
}

func (j *psiJob) set(p *psiPump)     { j.mu.Lock(); j.pump = p; j.mu.Unlock() }
func (j *psiJob) setJobID(id string) { j.mu.Lock(); j.jobID = id; j.mu.Unlock() }

func (j *psiJob) stop() {
	j.mu.Lock()
	p := j.pump
	j.mu.Unlock()
	if p != nil {
		p.Stop()
	}
}

func (s *Service) setPSIJob(runID string, j *psiJob) {
	s.mu.Lock()
	if s.psiJobs == nil {
		s.psiJobs = map[string]*psiJob{}
	}
	s.psiJobs[runID] = j
	s.mu.Unlock()
}

func (s *Service) clearPSIJob(runID string) {
	s.mu.Lock()
	delete(s.psiJobs, runID)
	s.mu.Unlock()
}

func (s *Service) psiJobID(runID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.psiJobs[runID]; j != nil {
		j.mu.Lock()
		defer j.mu.Unlock()
		return j.jobID
	}
	return ""
}

// StopPageSpeed ends the measurements without touching the crawl.
//
// Stopping the pump is enough on both paths: the crawl's drain sees it retire,
// and the standalone job's Wait returns. Everything measured so far is kept,
// and the pending query means a later Run picks up exactly where this left off.
func (s *Service) StopPageSpeed(runID string) error {
	if h := s.handleFor(runID); h != nil && h.psi != nil {
		h.psi.Stop()
	}
	s.mu.Lock()
	j := s.psiJobs[runID]
	s.mu.Unlock()
	if j != nil {
		j.stop()
	}
	return nil
}

// psiExperience is one CrUX block: the field metrics for this URL, or for the
// whole origin when the URL itself has too little traffic to report.
type psiExperience struct {
	OverallCategory string `json:"overall_category"`
	Metrics         map[string]struct {
		Percentile int    `json:"percentile"`
		Category   string `json:"category"`
	} `json:"metrics"`
}

// newPSIResult starts every measurable field at "not measured". Doing it here
// rather than at each assignment is what keeps a missing field from reading as
// a real zero.
func newPSIResult(rawURL, strategy string) PSIResult {
	return PSIResult{
		URL: rawURL, Strategy: strategy,
		FetchedAt: time.Now().UTC().Format(time.RFC3339),
		Score:     -1, A11yScore: -1, SEOScore: -1, BPScore: -1,
		CruxLCPMs: -1, CruxINPMs: -1, CruxCLS: -1, CruxFCPMs: -1, CruxTTFBMs: -1,
	}
}

// fetchPSI runs one PSI query and flattens the response to the stored shape.
func fetchPSI(ctx context.Context, client *http.Client, key, rawURL, strategy string) PSIResult {
	out := newPSIResult(rawURL, strategy)

	q := url.Values{}
	q.Set("url", rawURL)
	q.Set("strategy", strategy)
	// Repeated, not comma-joined: the API takes `category` once per category.
	for _, c := range psiCategories {
		q.Add("category", c)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, psiEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	// The key goes in a header, not the query string. PSI accepts both, but a
	// *url.Error prints the URL it was given, and out.Error is stored in
	// sitecrawl_psi.error and rendered in the UI — every other provider in this
	// codebase already keeps its credential in a header.
	req.Header.Set("X-goog-api-key", key)
	res, err := client.Do(req)
	if err != nil {
		out.Error = classifyError(ctx, err)
		return out
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 8<<20))

	if res.StatusCode != http.StatusOK {
		out.Error = psiAPIError(res.StatusCode, body)
		return out
	}

	var parsed struct {
		LoadingExperience       psiExperience `json:"loadingExperience"`
		OriginLoadingExperience psiExperience `json:"originLoadingExperience"`
		LighthouseResult        struct {
			// A map, not a struct per category: Google omits a category it could
			// not run, and Score is a pointer so "null" stays apart from 0.
			Categories map[string]struct {
				Score *float64 `json:"score"`
			} `json:"categories"`
			Audits map[string]struct {
				Title        string  `json:"title"`
				NumericValue float64 `json:"numericValue"`
				Details      struct {
					Type                string  `json:"type"`
					OverallSavingsMs    float64 `json:"overallSavingsMs"`
					OverallSavingsBytes float64 `json:"overallSavingsBytes"`
				} `json:"details"`
			} `json:"audits"`
		} `json:"lighthouseResult"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		out.Error = "bad-response"
		return out
	}
	lr := parsed.LighthouseResult

	score := func(id string) int {
		c, ok := lr.Categories[id]
		if !ok || c.Score == nil {
			return -1
		}
		return int(*c.Score*100 + 0.5)
	}
	out.Score = score("performance")
	out.A11yScore = score("accessibility")
	out.BPScore = score("best-practices")
	out.SEOScore = score("seo")

	out.FCPMs = int(lr.Audits["first-contentful-paint"].NumericValue)
	out.LCPMs = int(lr.Audits["largest-contentful-paint"].NumericValue)
	out.CLS = lr.Audits["cumulative-layout-shift"].NumericValue
	out.TBTMs = int(lr.Audits["total-blocking-time"].NumericValue)
	out.SIMs = int(lr.Audits["speed-index"].NumericValue)

	// This page's own field data first; the origin's only as a labelled
	// fallback, never silently.
	exp, src := parsed.LoadingExperience, "url"
	if len(exp.Metrics) == 0 {
		exp, src = parsed.OriginLoadingExperience, "origin"
	}
	if len(exp.Metrics) > 0 {
		out.CruxSource = src
		out.CruxVerdict = exp.OverallCategory
		metric := func(name string) int {
			m, ok := exp.Metrics[name]
			if !ok {
				return -1
			}
			return m.Percentile
		}
		out.CruxLCPMs = metric("LARGEST_CONTENTFUL_PAINT_MS")
		out.CruxINPMs = metric("INTERACTION_TO_NEXT_PAINT")
		out.CruxFCPMs = metric("FIRST_CONTENTFUL_PAINT_MS")
		out.CruxTTFBMs = metric("EXPERIMENTAL_TIME_TO_FIRST_BYTE")
		// CrUX reports CLS as an integer hundred times the score: 12 is 0.12.
		if p := metric("CUMULATIVE_LAYOUT_SHIFT_SCORE"); p >= 0 {
			out.CruxCLS = float64(p) / 100
		}
	}

	// Opportunities are the audits Lighthouse costs in saved time or bytes.
	// Anything that saves neither is a diagnostic, not a fix worth listing.
	for id, a := range lr.Audits {
		if a.Details.Type != "opportunity" {
			continue
		}
		ms, bytes := int64(a.Details.OverallSavingsMs), int64(a.Details.OverallSavingsBytes)
		if ms <= 0 && bytes <= 0 {
			continue
		}
		out.Opps = append(out.Opps, Opportunity{
			AuditID: id, Title: a.Title, SavingsMs: ms, SavingsBytes: bytes,
		})
	}
	return out
}

// psiAPIError turns Google's error envelope into a short slug + message.
func psiAPIError(status int, body []byte) string {
	var e struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return fmt.Sprintf("HTTP %d: %s", status, truncate(e.Error.Message, 200))
	}
	return fmt.Sprintf("HTTP %d", status)
}

func savePSI(db *sql.DB, runID string, r PSIResult) {
	// One transaction for the whole result. Each statement here used to commit
	// on its own, so a page with ten opportunities cost twelve commits — twelve
	// WAL writes and twelve trips through the writer lock, for one measurement.
	//
	// It also used to be hand-rolled and, alone among this repo's transactions,
	// had no deferred rollback: a panic anywhere below left the transaction open
	// on the workspace's single writer connection, so every later write in every
	// tool waited on a connection that was never coming back.
	//
	// The context is deliberately not the crawl's. This row records an answer
	// the PageSpeed API has already given; cancelling the fetch is free, but
	// cancelling the row that says what came back throws away a measurement that
	// costs another round trip to get again. Step 2.4 swaps this for
	// runs.Durable(c.ctx) — the same promise with a deadline on it.
	err := workspace.WithTx(context.Background(), db, func(tx *sql.Tx) error {
		return savePSIExec(tx, runID, r)
	})
	if err != nil {
		// Swallowing this is how a measurement disappears with the page still
		// listed as measured, which reads as "PageSpeed had nothing to say".
		slog.Error("sitecrawl: save pagespeed result",
			"run", runID, "url", r.URL, "strategy", r.Strategy, "err", err)
	}
}

func savePSIExec(db *sql.Tx, runID string, r PSIResult) error {
	if _, err := db.Exec(`
		INSERT INTO sitecrawl_psi(run_id, url, strategy, fetched_at, score, fcp_ms, lcp_ms, cls, tbt_ms, si_ms, error,
			a11y_score, seo_score, bp_score, crux_source, crux_verdict,
			crux_lcp_ms, crux_inp_ms, crux_cls, crux_fcp_ms, crux_ttfb_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(run_id, url, strategy) DO UPDATE SET
			fetched_at = excluded.fetched_at, score = excluded.score,
			fcp_ms = excluded.fcp_ms, lcp_ms = excluded.lcp_ms, cls = excluded.cls,
			tbt_ms = excluded.tbt_ms, si_ms = excluded.si_ms, error = excluded.error,
			a11y_score = excluded.a11y_score, seo_score = excluded.seo_score,
			bp_score = excluded.bp_score, crux_source = excluded.crux_source,
			crux_verdict = excluded.crux_verdict, crux_lcp_ms = excluded.crux_lcp_ms,
			crux_inp_ms = excluded.crux_inp_ms, crux_cls = excluded.crux_cls,
			crux_fcp_ms = excluded.crux_fcp_ms, crux_ttfb_ms = excluded.crux_ttfb_ms`,
		runID, r.URL, r.Strategy, r.FetchedAt, r.Score, r.FCPMs, r.LCPMs, r.CLS, r.TBTMs, r.SIMs, r.Error,
		r.A11yScore, r.SEOScore, r.BPScore, r.CruxSource, r.CruxVerdict,
		r.CruxLCPMs, r.CruxINPMs, r.CruxCLS, r.CruxFCPMs, r.CruxTTFBMs); err != nil {
		return fmt.Errorf("insert measurement: %w", err)
	}

	// Replace rather than merge: a re-measure that fixed an opportunity must
	// drop the old row, or the Opportunities tab keeps reporting work already
	// done.
	if _, err := db.Exec(`DELETE FROM sitecrawl_psi_opps WHERE run_id = ? AND url = ? AND strategy = ?`,
		runID, r.URL, r.Strategy); err != nil {
		return fmt.Errorf("clear old opportunities: %w", err)
	}
	for _, o := range r.Opps {
		if _, err := db.Exec(`
			INSERT INTO sitecrawl_psi_opps(run_id, url, strategy, audit_id, title, savings_ms, savings_bytes)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(run_id, url, strategy, audit_id) DO UPDATE SET
				title = excluded.title, savings_ms = excluded.savings_ms,
				savings_bytes = excluded.savings_bytes`,
			runID, r.URL, r.Strategy, o.AuditID, o.Title, o.SavingsMs, o.SavingsBytes); err != nil {
			return fmt.Errorf("insert opportunity %s: %w", o.AuditID, err)
		}
	}
	return nil
}

// PageSpeedResults lists every stored measurement for a run, newest first.
func (s *Service) PageSpeedResults(runID string) ([]PSIResult, error) {
	db, err := s.readDB()
	if err != nil {
		return nil, err
	}
	out := []PSIResult{}
	if runID == "" {
		return out, nil
	}
	rows, err := db.Query(`
		SELECT url, strategy, fetched_at, score, fcp_ms, lcp_ms, cls, tbt_ms, si_ms, error,
		       a11y_score, seo_score, bp_score, crux_source, crux_verdict,
		       crux_lcp_ms, crux_inp_ms, crux_cls, crux_fcp_ms, crux_ttfb_ms
		  FROM sitecrawl_psi WHERE run_id = ? ORDER BY fetched_at DESC, url`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var r PSIResult
		if err := rows.Scan(&r.URL, &r.Strategy, &r.FetchedAt, &r.Score,
			&r.FCPMs, &r.LCPMs, &r.CLS, &r.TBTMs, &r.SIMs, &r.Error,
			&r.A11yScore, &r.SEOScore, &r.BPScore, &r.CruxSource, &r.CruxVerdict,
			&r.CruxLCPMs, &r.CruxINPMs, &r.CruxCLS, &r.CruxFCPMs, &r.CruxTTFBMs); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
