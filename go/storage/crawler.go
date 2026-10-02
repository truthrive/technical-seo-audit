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

	"onescout/desktop/internal/core/httpx"
	"onescout/desktop/internal/core/safe"
)

// Cadence knobs. Vars so tests can shrink them.
var (
	// flushEvery bounds how long a result can sit in a buffer before it is
	// durable. Anything not flushed is lost if the process dies.
	flushEvery = 2 * time.Second
	// progressEvery paces ProgressEvent. At 50k URLs with 20 workers, one event
	// per page would be 60+/sec across the Wails bridge; 2/sec carries the same
	// information because the grid reads rows through SQL anyway.
	progressEvery = 500 * time.Millisecond
	// reportEvery paces the shared job:progress event.
	reportEvery = 250 * time.Millisecond
	// pauseGraceMax releases a pause that nobody ever resumes. Without it a
	// crawl paused overnight holds a goroutine, an http.Client and a 400k-entry
	// map indefinitely.
	pauseGraceMax = 30 * time.Minute
	// flushPages / flushEdges are the buffer sizes that force an early flush.
	flushPages = 200
	flushEdges = 1000
	// flushPSI is small because measurements are: ten workers at ~20s each
	// produce a handful a minute, and waiting for 200 would mean waiting hours.
	flushPSI = 10
)

// pauseGate is the crawl's stop/go signal.
//
// Deliberately local to this package rather than a new primitive in
// internal/core/jobs: that package is shared by five shipped tools, and
// indexcheck's types.go already records the ruling that widening its state enum
// ripples into all of them. A tool that needs a state jobs.Progress cannot
// express emits its own event instead — this is the third time that pattern
// applies.
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
// database handle. Workers communicate with it exclusively over channels, which
// is what turns the workspace's single-connection limit from a rule people have
// to remember into a property of the structure.
type coordinator struct {
	db       *sql.DB
	runID    string
	opts     Options
	seedHost string
	seedURL  string

	frontier   *frontier
	hosts      *hostGate
	fetch      *fetcher
	robots     *robotsCache
	exclusions *exclusionSet
	gate       *pauseGate

	// JavaScript rendering. rend is nil unless the option is on AND a browser
	// was found; workers call maybeRender which tolerates nil. noBrowser
	// remembers that the option was on but nothing could honour it — the UI
	// has to say that, not silently crawl unrendered.
	rend      renderer
	rGate     *renderGate
	noBrowser bool

	// PageSpeed. psi is nil unless the option is on AND a key exists; results
	// arrive on psiOut and are written by this goroutine like everything else,
	// because the coordinator is the only writer.
	psi    *psiPump
	psiOut chan PSIResult

	emit   func(name string, data any)
	report func(done, total int, message string)

	// buffers, flushed together so a crash never leaves edges without pages
	pages   []pageRow
	edges   []edge
	issues  []issueRow
	urls    []urlEntry
	psiRows []PSIResult
	// writeErr holds the first persistence failure. Set by flush, read by the
	// dispatch loop, which stops the crawl rather than reporting success over
	// rows that never landed.
	writeErr error

	crawled int
	// crawledInternal is what "the crawl went somewhere" means: with external
	// checking on, a seed that 301s off-site still fetches its target, and that
	// external page must not mask the one-page-crawl explanation.
	crawledInternal int
	issueTally      IssueTally
	statusTally     StatusTally
	revision        int64
	startedAt       time.Time
	current         string
	phase           string
	stopReason      string
	// offsiteRedirect is where the seed 301'd when that target is another
	// site — the reason a crawl can legitimately end at one page.
	offsiteRedirect string
	// seedUnreachable / seedRobotsBlocked record why the start URL produced
	// nothing, so a one-page crawl can name its cause.
	seedUnreachable   bool
	seedRobotsBlocked bool
	// dnsBlocked records that a public seed name resolved into the machine's own
	// network — DNS filtering, not a dev site. It explains a crawl that reaches
	// nothing, and it is the reason this run kept its private-address guard.
	dnsBlocked bool

	// deferred holds the dictionary ids already sent back for their one retry,
	// so a host refusing twice ends the URL rather than looping. In memory only:
	// the frontier table has no column for it, and a resumed run forfeiting the
	// record costs at most one extra request per still-refused URL.
	deferred map[int64]bool

	// robotsDelayAsked / robotsDelayApplied are the seed host's Crawl-delay as
	// the file asked for it and as the crawl actually paces by. See prepare.
	robotsDelayAsked   time.Duration
	robotsDelayApplied time.Duration

	// rate is a 10s rolling window of completion timestamps.
	rate []time.Time
}

// psiSetup is what the coordinator needs to run PageSpeed alongside the crawl:
// a read-only handle for the queue query and the user's key. Both zero means
// the pass is off, which is the normal case.
type psiSetup struct {
	read *sql.DB
	key  string
}

func newCoordinator(ctx context.Context, db *sql.DB, runID string, opts Options, seedURL string, proxy httpx.ProxyFunc,
	emit func(string, any), report func(int, int, string), psi psiSetup) (*coordinator, error) {

	u, err := url.Parse(seedURL)
	if err != nil || u.Host == "" {
		return nil, errInvalidURL
	}
	// Where the crawl was aimed decides whether it may reach the machine's own
	// network. Pointing the tool at a dev site on localhost is a first-class
	// workflow, but a public seed following an <img src="http://192.168.1.1/...">
	// into the user's LAN never is — and the page that wrote that address is not
	// the user. Deciding it from the seed keeps the workflow and drops the pivot,
	// without another option for the user to understand.
	//
	// Resolving privately is not on its own enough to grant that: an ISP blocking
	// a site by answering 127.0.0.1 for it makes an ordinary public domain look
	// exactly like a dev box. The NAME has to say local too — see httpx.IsLocalName.
	// A public name that resolves inward is DNS filtering, and the run says so
	// instead of quietly dropping its own guard.
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
		exclusions: newExclusionSet(opts.IssueExclusions, opts.UseDefaultExcl),
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
	// No key means no pass. The Configuration dialog already warns about that;
	// failing every measurement to make the same point would only fill the
	// results table with errors.
	if opts.EnablePageSpeed && psi.key != "" && psi.read != nil {
		c.psiOut = make(chan PSIResult, 64)
		// psiClient(), not the crawl's proxied client: see psiClient.
		c.psi = newPSIPump(psi.read, runID, opts.PSIStrategy, psi.key,
			psiClient(), emit,
			// Runs on a pump goroutine, so it may only hand the result over —
			// touching the buffers here would race the coordinator.
			//
			// The stop case is not optional. run() returns early on a write
			// failure without draining psiOut, and ctx is NOT cancelled on that
			// path: a worker parked on the send would stay parked for the life
			// of the app, and Wait() would never return.
			func(r PSIResult) {
				select {
				case c.psiOut <- r:
				case <-c.psi.stop:
				case <-ctx.Done():
				}
			})
	}
	return c, nil
}

// run executes the whole crawl: prepare, crawl, finalize.
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

	if c.psi != nil {
		defer c.psi.Stop()
		c.psi.Start(ctx, c.gate)
	}

	c.setPhase(ctx, PhaseCrawling)
	c.loop(ctx)
	// No more pages will appear, so an empty PageSpeed queue now means finished
	// rather than "the crawl has not flushed anything yet".
	if c.psi != nil {
		c.psi.Seal()
	}
	// The loop's drain absorbs the last in-flight results into the buffers, so
	// they have to reach SQLite before finalize reads — every graph rule works
	// from the tables, not from memory.
	c.flush()

	// Reported before ctx: a cancelled run that also failed to save is a failure,
	// and finalize below reads the tables this was supposed to fill.
	if c.writeErr != nil {
		return c.writeErr
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if c.gate.isPaused() {
		return nil // paused runs stop here with the frontier checkpointed
	}

	// A crawl that ends earlier than the user expected has to say why. These
	// were declared as constants but never assigned, so hitting MaxURLs on a
	// 5000-page site completed with a blank reason and left the user to infer it
	// from the count.
	//
	// Order is by explanatory power: what happened to the seed beats a limit,
	// because a one-page crawl is about the seed. Within the limits, the URL cap
	// wins — it is the one users set low by accident.
	if c.stopReason == "" {
		switch {
		// Ahead of "unreachable": DNS filtering is WHY it was unreachable, and it
		// is the one cause here the user can actually fix.
		case c.crawledInternal <= 1 && c.dnsBlocked:
			c.stopReason = StopDNSBlocked
		case c.crawledInternal <= 1 && c.seedRobotsBlocked:
			c.stopReason = StopRobotsBlocked
		case c.crawledInternal <= 1 && c.seedUnreachable:
			c.stopReason = StopSeedUnreachable
		// One page and an off-site seed redirect: the crawl is over before it
		// began, and completing silently reads as a bug. Name the reason so the
		// UI can offer to crawl the target instead.
		case c.offsiteRedirect != "" && c.crawledInternal <= 1:
			c.stopReason = StopSeedRedirect
		case c.frontier.hitURLCap:
			c.stopReason = StopMaxURLs
		case c.frontier.hitDepthCap:
			c.stopReason = StopMaxDepth
		}
	}

	c.setPhase(ctx, PhaseAnalyzing)
	c.finalize(ctx)
	// PageSpeed last, and after finalize on purpose: measuring 800 pages takes
	// far longer than the crawl did, and the grid must be complete and usable
	// while it finishes rather than held back by it.
	c.drainPSI(ctx)
	c.setPhase(ctx, PhaseDone)
	return nil
}

// drainPSI waits out the measurements the crawl started, writing results as
// they land. Returns as soon as the pump retires, the user stops it, or the run
// is cancelled.
func (c *coordinator) drainPSI(ctx context.Context) {
	if c.psi == nil {
		return
	}
	finished := make(chan struct{})
	go func() { c.psi.Wait(); close(finished) }()

	tick := time.NewTicker(flushEvery)
	defer tick.Stop()
	for waiting := true; waiting; {
		select {
		case r := <-c.psiOut:
			c.psiRows = append(c.psiRows, r)
			if len(c.psiRows) >= flushPSI {
				c.flush()
			}
		case <-tick.C:
			c.flush()
		case <-finished:
			waiting = false
		case <-ctx.Done():
			waiting = false
		}
	}
	// Results already handed over but not yet read. Dropping them would lose
	// measurements the user watched complete.
	for {
		select {
		case r := <-c.psiOut:
			c.psiRows = append(c.psiRows, r)
		default:
			c.flush()
			return
		}
	}
}

// prepare seeds the frontier: robots.txt, sitemaps, then the seed URLs.
func (c *coordinator) prepare(ctx context.Context, seeds []string) error {
	origin := originOf(c.seedURL)

	// Crawl-delay is read whenever robots.txt is read, but only APPLIED when the
	// user asked for it — see Options.RespectCrawlDelay. Reading it either way is
	// what lets the status strip say "robots.txt asks 2s, ignoring" rather than
	// leave an unexplained speed on screen.
	if c.opts.RespectRobots && origin != "" {
		capped, asked := c.robots.Delay(ctx, origin)
		c.robotsDelayAsked = asked
		if capped > 0 && c.opts.RespectCrawlDelay {
			c.robotsDelayApplied = capped
			c.hosts.setRobotsDelay(strings.ToLower(hostOf(c.seedURL)), capped, asked)
		}
	}

	// List mode takes exactly what it was given and never discovers anything.
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
			// Only entries on the crawl's own site. The sitemap fetch follows
			// redirects, so a domain that 301s elsewhere serves the TARGET's
			// sitemap — ingesting those foreign URLs once inflated "found" to 91
			// on a site whose crawl could only ever reach 1 page.
			if u, err := url.Parse(e.Loc); err != nil ||
				!sameSite(c.seedHost, u.Hostname(), c.opts.crawlSubdomains()) {
				continue
			}
			// Sitemap URLs enter at depth 0. That is what makes an orphan
			// detectable — but it also means sitemap discovery quietly defeats
			// maxDepth, which the UI has to say out loud rather than let the
			// user discover from the URL count.
			c.frontier.admit(e.Loc, 0, SourceSitemap, 0)
		}
	}
	return nil
}

// loop is the dispatcher. One select, one writer, no locks.
func (c *coordinator) loop(ctx context.Context) {
	work := make(chan frontierItem)
	results := make(chan crawlResult, c.opts.Concurrency*2)

	var wg sync.WaitGroup
	for i := 0; i < c.opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				// doOne parses third-party HTML (and, when rendering is on, a
				// browser-built DOM). A tokenizer or codec giving up on
				// hostile markup is realistic input, not a programming
				// mistake — and jobs.Runner's recover cannot reach this
				// goroutine, only the one that started the job. Without this
				// the process died with no dialog, taking every buffered page
				// with it and every other tool's run besides.
				//
				// The URL is recorded as a failed fetch so the crawl reports
				// it, rather than the row silently never arriving.
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
		// Drain whatever is still in flight so nothing is dropped mid-write.
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

	// Reused by the politeness wait below, so the wait costs one timer for the
	// whole crawl instead of one per iteration. Since Go 1.23 timer channels are
	// unbuffered and Reset discards a pending send, so no drain is needed here.
	waitTimer := time.NewTimer(time.Hour)
	waitTimer.Stop()
	defer waitTimer.Stop()

	var (
		inFlight   int
		pauseSince time.Time
	)

	for {
		if ctx.Err() != nil {
			return
		}
		// Results that cannot be persisted are not results. Continuing would only
		// grow the kept buffers and end in a run marked "completed" over rows that
		// never landed.
		if c.writeErr != nil {
			return
		}
		if c.frontier.empty() && inFlight == 0 {
			return
		}

		// Decide whether a dispatch is possible this iteration. A nil channel
		// disables the send case, which is what lets one select cover "ready to
		// dispatch" and "nothing to dispatch" without spinning.
		//
		// The item is only PEEKED here. Removing it before the select would lose
		// it whenever the select picks a different case — a result arriving or a
		// tick is enough — and the URL would silently never be crawled.
		var out chan<- frontierItem
		var next frontierItem
		nextIdx := -1
		if !c.gate.isPaused() && inFlight < c.opts.Concurrency {
			if i := c.frontier.peekReady(c.hosts); i >= 0 {
				nextIdx = i
				next = c.frontier.at(i)
				out = work
			}
		}

		// When nothing is dispatchable but work remains, wake when the politeness
		// gate says a host is free again rather than busy-looping.
		//
		// One timer is reused across iterations. This branch is the normal state
		// of a single-host crawl once maxPerHost is reached, so a per-iteration
		// `defer t.Stop()` piled up ~360k deferred closures and live timers over
		// an hour — none released until loop returned.
		var wake <-chan time.Time
		if out == nil && !c.frontier.empty() && inFlight < c.opts.Concurrency && !c.gate.isPaused() {
			d := c.hosts.nextWake()
			if d <= 0 {
				d = 10 * time.Millisecond
			}
			waitTimer.Reset(d)
			wake = waitTimer.C
		}

		select {
		case out <- next:
			// Only now is it safe to remove it from the queue.
			c.frontier.take(nextIdx)
			c.hosts.reserve(next.host)
			c.current = next.URL
			inFlight++

		case r := <-results:
			inFlight--
			c.absorb(r)
			if len(c.pages) >= flushPages || len(c.edges) >= flushEdges {
				c.flush()
			}

		// A nil channel blocks for ever, which is exactly right when the
		// PageSpeed pass is off: the case simply never fires.
		case r := <-c.psiOut:
			c.psiRows = append(c.psiRows, r)
			if len(c.psiRows) >= flushPSI {
				c.flush()
			}

		case <-flushTick.C:
			c.flush()

		case <-progressTick.C:
			c.emitProgress()

		case <-reportTick.C:
			c.report(c.crawled, c.frontier.discovered(), c.current)

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

		// A pause nobody lifts must not hold the goroutine forever.
		if !pauseSince.IsZero() && time.Since(pauseSince) > pauseGraceMax {
			return
		}
	}
}

// doOne is the worker body: robots, fetch, parse. It touches no shared state
// except the robots cache, which is safe for concurrent use.
func (c *coordinator) doOne(ctx context.Context, item frontierItem) crawlResult {
	out := crawlResult{item: item}

	// Bound one URL on its own so a hung host cannot stall a worker for the
	// whole run. Rendering rides in the same budget, so it must be included.
	budget := c.opts.urlBudget()
	if c.rend != nil {
		budget += time.Duration(c.opts.JSTimeoutSec) * time.Second
	}
	urlCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()

	if c.opts.RespectRobots {
		out.robots = c.robots.Check(urlCtx, item.URL)
		if out.robots.State == RobotsBlocked {
			// Blocked URLs are still recorded — "this page is blocked" is the
			// finding — but never fetched.
			out.skipped = true
			return out
		}
	}
	// External pages are fetched for their status only, never parsed: their
	// links are not followed anyway, and extracting them would pour another
	// site's URLs into the dictionary.
	wantBody := sameSite(c.seedHost, hostnameOf(item.URL), c.opts.crawlSubdomains())
	out.res = c.fetch.fetch(urlCtx, item.URL, wantBody)
	out.rendered = maybeRender(urlCtx, c.rend, c.rGate, &out.res, wantBody)
	return out
}

// absorb folds one result into the buffers. Runs on the coordinator goroutine,
// so it may touch the frontier and the counters freely.
func (c *coordinator) absorb(r crawlResult) {
	c.hosts.release(r.item.host, r.res.Status, r.res.ErrorType)

	// A 429 or 503 is the site saying "not now" — it is not a verdict on the
	// page. Put the URL back at the end of the queue and try it once more when
	// the crawl is winding down and the host has had time to recover.
	//
	// Recording the refusal instead left a real crawl reporting ~100 pages as
	// failures that were fine, and silently lost every page reachable only
	// through one of them: a refused page has no body, so none of its links were
	// ever discovered. Nothing is written for the first attempt — the retry's
	// result is the page's result.
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
	if r.res.BotBlocked {
		p.Issues = append(p.Issues, IssueBotBlocked)
	}

	if r.item.Source == SourceSeed {
		c.seedRobotsBlocked = r.robots.State == RobotsBlocked
		c.seedUnreachable = r.res.ErrorType != "" || r.res.Status == 0
	}

	// A redirect target is not linked from anywhere in the markup, so it has to
	// be queued explicitly or a page reachable only through a redirect would
	// never be crawled.
	if p.RedirectTo != "" && p.Internal {
		c.frontier.admit(p.RedirectTo, p.Depth, SourceRedirect, r.item.ID)
		// The seed pointing off-site is Screaming Frog's most-asked "why only
		// one URL" — remember the target so the completion can explain itself.
		if r.item.Source == SourceSeed {
			if u, err := url.Parse(p.RedirectTo); err == nil && u.Host != "" &&
				!sameSite(c.seedHost, u.Hostname(), c.opts.crawlSubdomains()) {
				c.offsiteRedirect = p.RedirectTo
			}
		}
	}

	// Links are collected here, not in the worker, because admitting to the
	// frontier mutates the dictionary and the queue.
	if !r.skipped && r.res.Doc != nil {
		targets := collectLinks(c.frontier, r.item.ID, p, r.res.Doc, c.opts)
		c.edges = append(c.edges, targets.Edges...)
		// Canonical, hreflang and pagination targets are queued too. None of
		// them is an <a href>, so a page referenced only that way would never be
		// crawled — and then the rules that check whether a canonical points at
		// a live, indexable page would have nothing to check against.
		if p.Internal {
			for _, ref := range referencedURLs(p) {
				c.frontier.admit(ref, p.Depth+1, SourceLink, r.item.ID)
			}
		}
	}

	// Settle indexability before the blob is marshalled: the stored record and
	// the promoted column have to agree, and the column is what the grid reads.
	p.Indexable, p.Indexability = indexabilityOf(p)

	found := evaluate(p, c.exclusions)
	if r.res.BotBlocked {
		found = append(found, issue{Code: IssueBotBlocked})
	}
	for _, is := range found {
		p.Issues = append(p.Issues, is.Code)
		c.issues = append(c.issues, issueRow{URLID: r.item.ID, issue: is})
		switch severityOf(is.Code) {
		case SeverityCritical:
			c.issueTally.Critical++
		case SeverityWarning:
			c.issueTally.Warning++
		default:
			c.issueTally.Notice++
		}
	}

	blob, err := json.Marshal(p)
	if err != nil {
		blob = []byte("{}")
	}
	c.pages = append(c.pages, projectRow(r.item.ID, p, string(blob), found))
	c.urls = append(c.urls, c.frontier.takePending()...)

	c.crawled++
	if p.Internal {
		c.crawledInternal++
	}
	c.tallyStatus(p, r.robots.State)
	c.noteRate()
	c.current = r.item.URL
}

// shouldDefer reports whether a result is a temporary refusal worth one retry
// later in the run.
//
// Only 429 and 503 qualify. A timeout or a dropped connection is already
// retried inside the fetch itself, and anything else — a 404, a 500, a DNS
// failure — is an answer about the page that asking again will not change.
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

// flush writes every buffer, in dependency order, then bumps the revision the
// frontend watches.
//
// Order matters: URLs before pages and edges, so a foreign-key-shaped read
// never sees an edge whose endpoint is unknown. Pages before issues so the
// grid never shows a count without the row.
// A write failure is recorded in c.writeErr and the buffer is kept rather than
// truncated. Discarding it silently — which is what ignoring these four errors
// amounted to — dropped up to 200 pages and 1000 edges while the run still
// reported "completed", and left sitecrawl_pages rows pointing at dictionary
// ids that never landed, so those URLs render blank in the grid forever.
func (c *coordinator) flush() {
	if len(c.urls) == 0 && len(c.pages) == 0 && len(c.edges) == 0 && len(c.issues) == 0 &&
		len(c.psiRows) == 0 {
		return
	}
	// Dependency order: URLs before pages and edges so a read never sees an edge
	// whose endpoint is unknown. A failure stops the sequence for the same reason
	// — writing pages after the dictionary failed would create exactly that.
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
	if len(c.issues) > 0 {
		if err := writeIssues(c.db, c.runID, c.issues); err != nil {
			c.failWrite(err)
			return
		}
		c.issues = c.issues[:0]
	}
	// PageSpeed rows are independent of the page/edge graph — a failure to save
	// one is not a reason to stop the crawl. savePSI logs its own failures
	// rather than reporting them here.
	if len(c.psiRows) > 0 {
		for _, r := range c.psiRows {
			savePSI(c.db, c.runID, r)
		}
		c.psiRows = c.psiRows[:0]
	}
	setCounters(c.db, c.runID, c.frontier.discovered(), c.crawled,
		c.issueTally.Critical+c.issueTally.Warning+c.issueTally.Notice, c.frontier.nextID)
	c.revision++
}

// failWrite records the first persistence failure. The dispatch loop stops on
// it: retrying against a full disk or a closed handle only grows the buffers,
// and finishing the crawl would report success over missing rows.
func (c *coordinator) failWrite(err error) {
	if c.writeErr == nil {
		c.writeErr = fmt.Errorf("sitecrawl: saving results failed: %w", err)
	}
}

// checkpoint persists the outstanding queue so a closed app or a crash can
// resume rather than start over.
func (c *coordinator) checkpoint() {
	// Pending dictionary entries have to land first, or the resumed run would
	// re-issue ids that the frontier already handed out.
	// A checkpoint that fails is not visible until the user tries to resume and
	// the crawl restarts from the beginning — by which point there is nothing
	// left to diagnose it with.
	if pending := c.frontier.takePending(); len(pending) > 0 {
		if err := writeURLs(c.db, c.runID, pending); err != nil {
			slog.Error("sitecrawl: checkpoint url dictionary", "run", c.runID, "err", err)
		}
	}
	if err := saveFrontier(c.db, c.runID, c.frontier.snapshot()); err != nil {
		slog.Error("sitecrawl: checkpoint frontier", "run", c.runID, "err", err)
	}
	setCounters(c.db, c.runID, c.frontier.discovered(), c.crawled,
		c.issueTally.Critical+c.issueTally.Warning+c.issueTally.Notice, c.frontier.nextID)
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
		Skipped:   c.frontier.skipped,
		Issues:    c.issueTally,
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

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Host
}

// hostnameOf is hostOf without the port, the shape sameSite compares.
func hostnameOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
