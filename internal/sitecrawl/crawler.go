package sitecrawl

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/truthrive/technical-seo-audit/go/deps/httpx"
	"github.com/truthrive/technical-seo-audit/go/deps/safe"
)

// Cadence knobs. Vars so tests can shrink them.
var (
	flushEvery    = 2 * time.Second
	progressEvery = 500 * time.Millisecond
	reportEvery   = 250 * time.Millisecond
	pauseGraceMax = 30 * time.Minute
	flushPages    = 200
	flushEdges    = 1000
)

// pauseGate is the crawl's stop/go signal.
type pauseGate struct {
	mu      sync.Mutex
	paused  bool
	resumed chan struct{}
	changed chan struct{}
}

func newPauseGate() *pauseGate {
	return &pauseGate{changed: make(chan struct{}, 1)}
}

func (g *pauseGate) Pause() {
	g.mu.Lock()
	if !g.paused {
		g.paused = true
		g.resumed = make(chan struct{})
	}
	g.mu.Unlock()
	g.nudge()
}

func (g *pauseGate) Resume() {
	g.mu.Lock()
	if g.paused {
		g.paused = false
		close(g.resumed)
		g.resumed = nil
	}
	g.mu.Unlock()
	g.nudge()
}

func (g *pauseGate) isPaused() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.paused
}

func (g *pauseGate) nudge() {
	select {
	case g.changed <- struct{}{}:
	default:
	}
}

// crawlResult is what a worker hands back. Workers never touch SQLite or the
// frontier; everything they produce is absorbed by the coordinator.
type crawlResult struct {
	item     frontierItem
	res      fetched
	robots   robotsVerdict
	skipped  bool
	rendered bool
}

// coordinator owns every piece of mutable crawl state, including the only
// database handle.
type coordinator struct {
	db       *sql.DB
	runID    string
	opts     Options
	seedHost string
	seedURL  string

	frontier *frontier
	hosts    *hostGate
	fetch    *fetcher
	robots   *robotsCache
	gate     *pauseGate

	rend      renderer
	rGate     *renderGate
	noBrowser bool

	emit   func(name string, data any)
	report func(done, total int, message string)

	pages []pageRow
	edges []edge
	urls  []urlEntry

	writeErr error

	crawled         int
	crawledInternal int
	inFlight        int
	statusTally     StatusTally
	revision        int64
	startedAt       time.Time
	current         string
	phase           string
	stopReason      string
	offsiteRedirect string

	seedUnreachable   bool
	seedRobotsBlocked bool
	dnsBlocked        bool

	deferred map[int64]bool

	robotsDelayAsked   time.Duration
	robotsDelayApplied time.Duration

	rate []time.Time
}

func newCoordinator(ctx context.Context, db *sql.DB, runID string, opts Options, seedURL string, proxy httpx.ProxyFunc,
	emit func(string, any), report func(int, int, string)) (*coordinator, error) {

	u, err := url.Parse(seedURL)
	if err != nil || u.Host == "" {
		return nil, errInvalidURL
	}

	allowPrivate, dnsBlocked := httpx.AllowPrivate(ctx, u.Hostname())

	f := newFetcher(opts, proxy, u.Hostname(), allowPrivate)
	c := &coordinator{
		db:         db,
		runID:      runID,
		opts:       opts,
		seedHost:   u.Hostname(),
		seedURL:    seedURL,
		frontier:   newFrontier(opts, u.Hostname()),
		hosts:      newHostGate(opts),
		fetch:      f,
		robots:     newRobotsCache(f.robotsClient(), opts.preset(), opts.timeout()),
		gate:       newPauseGate(),
		deferred:   map[int64]bool{},
		dnsBlocked: dnsBlocked,
		emit:       emit,
		report:     report,
		startedAt:  time.Now(),
		phase:      PhasePreparing,
	}
	if opts.EnableJavaScript {
		if cr := newChromeRenderer(opts); cr != nil {
			c.rend = cr
			c.rGate = newRenderGate(opts)
		} else {
			c.noBrowser = true
		}
	}
	return c, nil
}

// run executes the whole crawl: prepare, crawl, finalizeAcquisition.
func (c *coordinator) run(ctx context.Context, seeds []string, resume bool) error {
	defer c.flush()
	if c.rend != nil {
		defer c.rend.close()
	}
	if c.noBrowser && c.emit != nil {
		c.emit(EventRender, RenderStateEvent{RunID: c.runID, State: "no-browser"})
	}

	if !resume {
		if err := c.prepare(ctx, seeds); err != nil {
			return err
		}
	}

	c.setPhase(ctx, PhaseCrawling)
	c.loop(ctx)
	c.flush()

	if c.writeErr != nil {
		return c.writeErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if c.gate.isPaused() {
		return nil
	}

	if c.stopReason == "" {
		switch {
		case c.crawledInternal <= 1 && c.dnsBlocked:
			c.stopReason = StopDNSBlocked
		case c.crawledInternal <= 1 && c.seedRobotsBlocked:
			c.stopReason = StopRobotsBlocked
		case c.crawledInternal <= 1 && c.seedUnreachable:
			c.stopReason = StopSeedUnreachable
		case c.offsiteRedirect != "" && c.crawledInternal <= 1:
			c.stopReason = StopSeedRedirect
		case c.frontier.hitURLCap:
			c.stopReason = StopMaxURLs
		case c.frontier.hitDepthCap:
			c.stopReason = StopMaxDepth
		}
	}

	c.setPhase(ctx, PhaseAnalyzing)
	c.finalizeAcquisition(ctx)
	c.setPhase(ctx, PhaseDone)
	return nil
}

func (c *coordinator) prepare(ctx context.Context, seeds []string) error {
	origin := originOf(c.seedURL)

	if c.opts.RespectRobots && origin != "" {
		capped, asked := c.robots.Delay(ctx, origin)
		c.robotsDelayAsked = asked
		if capped > 0 && c.opts.RespectCrawlDelay {
			c.robotsDelayApplied = capped
			c.hosts.setRobotsDelay(strings.ToLower(hostOf(c.seedURL)), capped, asked)
		}
	}

	if c.opts.listMode() {
		for _, s := range seeds {
			c.frontier.admit(s, 0, SourceManual, 0)
		}
		return nil
	}

	for _, s := range seeds {
		c.frontier.admit(s, 0, SourceSeed, 0)
	}

	if c.opts.DiscoverSitemaps && origin != "" {
		declared := c.robots.Sitemaps(ctx, origin)
		entries, _ := discoverSitemaps(ctx, c.fetch.robotsClient(), c.opts.preset(), origin,
			declared, c.opts.Concurrency)
		for _, e := range entries {
			if ctx.Err() != nil {
				break
			}
			if u, err := url.Parse(e.Loc); err != nil ||
				!sameSite(c.seedHost, u.Hostname(), c.opts.crawlSubdomains()) {
				continue
			}
			c.frontier.admit(e.Loc, 0, SourceSitemap, 0)
		}
	}
	return nil
}

func (c *coordinator) loop(ctx context.Context) {
	work := make(chan frontierItem)
	results := make(chan crawlResult, c.opts.Concurrency*2)

	var wg sync.WaitGroup
	for i := 0; i < c.opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				results <- safe.Call(
					"sitecrawl/doOne",
					func() crawlResult { return c.doOne(ctx, item) },
					func(p any) crawlResult {
						return crawlResult{item: item, res: fetched{
							ErrorType: ErrConnection,
							Error:     fmt.Sprintf("internal error while reading this page (%v)", p),
						}}
					},
				)
			}
		}()
	}
	defer func() {
		close(work)
		go func() { wg.Wait(); close(results) }()
		for r := range results {
			c.absorb(r)
		}
	}()

	flushTick := time.NewTicker(flushEvery)
	defer flushTick.Stop()
	progressTick := time.NewTicker(progressEvery)
	defer progressTick.Stop()
	reportTick := time.NewTicker(reportEvery)
	defer reportTick.Stop()

	waitTimer := time.NewTimer(time.Hour)
	waitTimer.Stop()
	defer waitTimer.Stop()

	var pauseSince time.Time

	for {
		if ctx.Err() != nil {
			return
		}
		if c.writeErr != nil {
			return
		}
		if c.frontier.empty() && c.inFlight == 0 {
			return
		}

		var out chan<- frontierItem
		var next frontierItem
		nextIdx := -1
		if !c.gate.isPaused() && c.inFlight < c.opts.Concurrency {
			if i := c.frontier.peekReady(c.hosts); i >= 0 {
				nextIdx = i
				next = c.frontier.at(i)
				out = work
			}
		}

		var wake <-chan time.Time
		if out == nil && !c.frontier.empty() && c.inFlight < c.opts.Concurrency && !c.gate.isPaused() {
			d := c.hosts.nextWake()
			if d <= 0 {
				d = 10 * time.Millisecond
			}
			waitTimer.Reset(d)
			wake = waitTimer.C
		}

		select {
		case out <- next:
			c.frontier.take(nextIdx)
			c.hosts.reserve(next.host)
			c.current = next.URL
			c.inFlight++

		case r := <-results:
			c.inFlight--
			c.absorb(r)
			if len(c.pages) >= flushPages || len(c.edges) >= flushEdges {
				c.flush()
			}

		case <-flushTick.C:
			c.flush()

		case <-progressTick.C:
			c.emitProgress()

		case <-reportTick.C:
			if c.report != nil {
				c.report(c.crawled, c.frontier.discovered(), c.current)
			}

		case <-c.gate.changed:
			if c.gate.isPaused() {
				pauseSince = time.Now()
				c.flush()
				c.checkpoint()
				pauseRun(c.db, c.runID)
				c.emitState(StatePaused, "", true, nil)
			} else {
				pauseSince = time.Time{}
				resumeRunState(c.db, c.runID)
				clearFrontier(c.db, c.runID)
				c.emitState(StateRunning, "", false, nil)
			}

		case <-wake:

		case <-ctx.Done():
			return
		}

		if !pauseSince.IsZero() && time.Since(pauseSince) > pauseGraceMax {
			return
		}
	}
}

func (c *coordinator) doOne(ctx context.Context, item frontierItem) crawlResult {
	out := crawlResult{item: item}

	budget := c.opts.urlBudget()
	if c.rend != nil {
		budget += time.Duration(c.opts.JSTimeoutSec) * time.Second
	}
	urlCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	if c.opts.RespectRobots {
		out.robots = c.robots.Check(urlCtx, item.URL)
		if out.robots.State == RobotsBlocked {
			out.skipped = true
			return out
		}
	}

	wantBody := sameSite(c.seedHost, hostnameOf(item.URL), c.opts.crawlSubdomains())
	out.res = c.fetch.fetch(urlCtx, item.URL, wantBody)
	out.rendered = maybeRender(urlCtx, c.rend, c.rGate, &out.res, wantBody)
	return out
}

func (c *coordinator) absorb(r crawlResult) {
	c.hosts.release(r.item.host, r.res.Status, r.res.ErrorType)

	if c.shouldDefer(r) {
		c.frontier.requeue(r.item)
		c.deferred[r.item.ID] = true
		return
	}

	p := buildPage(r.item, r.res, c.seedHost, c.opts)
	p.Rendered = r.rendered
	p.RobotsState = r.robots.State
	if r.robots.State == "" {
		p.RobotsState = RobotsUnknown
	}

	if r.item.Source == SourceSeed {
		c.seedRobotsBlocked = r.robots.State == RobotsBlocked
		c.seedUnreachable = r.res.ErrorType != "" || r.res.Status == 0
	}

	if p.RedirectTo != "" && p.Internal {
		c.frontier.admit(p.RedirectTo, p.Depth, SourceRedirect, r.item.ID)
		if r.item.Source == SourceSeed {
			if u, err := url.Parse(p.RedirectTo); err == nil && u.Host != "" &&
				!sameSite(c.seedHost, u.Hostname(), c.opts.crawlSubdomains()) {
				c.offsiteRedirect = p.RedirectTo
			}
		}
	}

	if !r.skipped && r.res.Doc != nil {
		targets := collectLinks(c.frontier, r.item.ID, p, r.res.Doc, c.opts)
		c.edges = append(c.edges, targets.Edges...)
		if p.Internal {
			for _, ref := range referencedURLs(p) {
				c.frontier.admit(ref, p.Depth+1, SourceLink, r.item.ID)
			}
		}
	}

	p.Indexable, p.Indexability = indexabilityOf(p)

	blob, err := json.Marshal(p)
	if err != nil {
		blob = []byte("{}")
	}
	c.pages = append(c.pages, projectRow(r.item.ID, p, string(blob)))
	c.urls = append(c.urls, c.frontier.takePending()...)

	c.crawled++
	if p.Internal {
		c.crawledInternal++
	}
	c.tallyStatus(p, r.robots.State)
	c.noteRate()
	c.current = r.item.URL
}

func (c *coordinator) shouldDefer(r crawlResult) bool {
	if r.skipped || c.deferred[r.item.ID] {
		return false
	}
	return r.res.Status == http.StatusTooManyRequests ||
		r.res.Status == http.StatusServiceUnavailable
}

func (c *coordinator) tallyStatus(p *Page, robotsState string) {
	switch {
	case robotsState == RobotsBlocked:
		c.statusTally.Blocked++
	case p.Status == 0:
		c.statusTally.Failed++
	case p.Status >= 500:
		c.statusTally.ServerErr++
	case p.Status >= 400:
		c.statusTally.ClientErr++
	case p.Status >= 300:
		c.statusTally.Redirect++
	default:
		c.statusTally.OK++
	}
}

func (c *coordinator) noteRate() {
	now := time.Now()
	c.rate = append(c.rate, now)
	cutoff := now.Add(-10 * time.Second)
	i := 0
	for ; i < len(c.rate) && c.rate[i].Before(cutoff); i++ {
	}
	c.rate = c.rate[i:]
}

func (c *coordinator) flush() {
	if len(c.urls) == 0 && len(c.pages) == 0 && len(c.edges) == 0 {
		return
	}
	if len(c.urls) > 0 {
		if err := writeURLs(c.db, c.runID, c.urls); err != nil {
			c.failWrite(err)
			return
		}
		c.urls = c.urls[:0]
	}
	if len(c.pages) > 0 {
		if err := writePages(c.db, c.runID, c.pages); err != nil {
			c.failWrite(err)
			return
		}
		c.pages = c.pages[:0]
	}
	if len(c.edges) > 0 {
		if err := writeLinks(c.db, c.runID, c.edges); err != nil {
			c.failWrite(err)
			return
		}
		c.edges = c.edges[:0]
	}
	setCounters(c.db, c.runID, c.frontier.discovered(), c.crawled, 0, c.frontier.nextID)
	c.revision++
}

func (c *coordinator) failWrite(err error) {
	if c.writeErr == nil {
		c.writeErr = fmt.Errorf("sitecrawl: saving results failed: %w", err)
	}
}

func (c *coordinator) checkpoint() {
	if pending := c.frontier.takePending(); len(pending) > 0 {
		if err := writeURLs(c.db, c.runID, pending); err != nil {
			slog.Error("sitecrawl: checkpoint url dictionary", "run", c.runID, "err", err)
		}
	}
	if err := saveFrontier(c.db, c.runID, c.frontier.snapshot()); err != nil {
		slog.Error("sitecrawl: checkpoint frontier", "run", c.runID, "err", err)
	}
	setCounters(c.db, c.runID, c.frontier.discovered(), c.crawled, 0, c.frontier.nextID)
}

func (c *coordinator) setPhase(ctx context.Context, phase string) {
	c.phase = phase
	setPhase(c.db, c.runID, phase)
	c.emitProgress()
}

func (c *coordinator) emitProgress() {
	if c.emit == nil {
		return
	}
	elapsed := time.Since(c.startedAt)
	var rate float64
	if len(c.rate) > 1 {
		span := c.rate[len(c.rate)-1].Sub(c.rate[0]).Seconds()
		if span > 0 {
			rate = float64(len(c.rate)-1) / span
		}
	}
	eta := -1
	if rate > 0 {
		if remaining := c.frontier.queued(); remaining > 0 {
			eta = int(float64(remaining) / rate)
		}
	}
	c.emit(EventProgress, ProgressEvent{
		RunID:     c.runID,
		Phase:     c.phase,
		Found:     c.frontier.discovered(),
		Crawled:   c.crawled,
		Queued:    c.frontier.queued(),
		InFlight:  c.inFlight,
		Skipped:   c.frontier.skipped,
		Status:    c.statusTally,
		Rate:      rate,
		ElapsedMs: elapsed.Milliseconds(),
		ETASec:    eta,
		Revision:  c.revision,
		Current:   c.current,

		CrawlDelayAskedMs:   int(c.robotsDelayAsked.Milliseconds()),
		CrawlDelayAppliedMs: int(c.robotsDelayApplied.Milliseconds()),
	})
}

func (c *coordinator) emitState(state, reason string, resumable bool, err error) {
	if c.emit == nil {
		return
	}
	ev := RunStateEvent{
		RunID:      c.runID,
		State:      state,
		Phase:      c.phase,
		Reason:     reason,
		Resumable:  resumable,
		Found:      c.frontier.discovered(),
		Crawled:    c.crawled,
		DurationMs: time.Since(c.startedAt).Milliseconds(),
	}
	if reason == StopSeedRedirect {
		ev.RedirectTarget = c.offsiteRedirect
	}
	if err != nil {
		ev.Error = err.Error()
	}
	c.emit(EventRunState, ev)
}

func (c *coordinator) finalizeAcquisition(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	c.finalizeInlinks(ctx)
	c.revision++
}

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
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.Error("sitecrawl: scan inlinks", "run", c.runID, "err", err)
	}

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

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

func hostnameOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
