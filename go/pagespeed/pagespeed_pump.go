package sitecrawl

import (
	"context"
	"database/sql"
	"net/http"
	"sync"
	"time"

	"onescout/desktop/internal/core/safe"
)

// The PageSpeed pump: measures every internal HTML page of a run, the way
// Screaming Frog's API integration does — no button, no selection, it simply
// runs alongside the crawl and finishes after it.
//
// The queue comes from SQL, not from the crawl. One query answers "which pages
// of this run still have no measurement", and that same query serves all three
// entry points: a crawl in progress, a crawl resumed after a pause, and a
// finished run the user asks to fill in later. Feeding the pump from the crawl
// loop instead would need a second, unbounded in-memory queue (absorb runs on
// the coordinator goroutine — blocking it stalls the whole crawl) plus its own
// checkpoint to survive a pause, and the three paths would each drift.
//
// Nothing here writes to SQLite during a crawl. Results go out through sink,
// which the coordinator points at its own buffer, because the coordinator is
// the only writer the workspace's single connection allows.

var (
	// psiRefillEvery is how often the pump looks for newly crawled pages.
	// Slower than the crawl's flush on purpose: a measurement takes ~20s, so
	// finding work a second late costs nothing.
	psiRefillEvery = 2 * time.Second
	// psiProgressEvery paces the pump's own progress event.
	psiProgressEvery = 500 * time.Millisecond
	// psiQueueBatch bounds one refill. Big enough that the workers never idle,
	// small enough that a 50k crawl does not build a 50k-string slice.
	psiQueueBatch = 200
)

type psiPump struct {
	read     *sql.DB
	runID    string
	strategy string
	key      string
	client   *http.Client
	emit     func(string, any)
	sink     func(PSIResult)

	// fixed is a caller-supplied URL list (the "measure exactly these" path).
	// When set the pump never queries for more work.
	fixed []string

	mu    sync.Mutex
	queue []string
	// sent holds URLs handed to a worker whose result has not landed yet; the
	// refill query cannot see them, because their row does not exist yet.
	sent map[string]bool
	// attempted holds every URL this pump has already measured, successful or
	// not. A URL Google keeps rejecting stays in the pending query for ever —
	// without this the pump would measure it again on every refill and never
	// finish.
	attempted map[string]bool
	// sealed means the crawl has stopped producing pages, so an empty refill
	// really is the end. Until then an empty queue only means "the crawl has
	// not flushed anything new yet".
	sealed    bool
	lastAdded int
	// done/total are the fixed-list counters; the query path reads its numbers
	// from SQL instead, so a resumed run does not restart the bar at zero.
	done       int
	total      int
	measured   int
	measurable int

	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func newPSIPump(read *sql.DB, runID, strategy, key string, client *http.Client,
	emit func(string, any), sink func(PSIResult)) *psiPump {
	if strategy != StrategyDesktop {
		strategy = StrategyMobile
	}
	return &psiPump{
		read: read, runID: runID, strategy: strategy, key: key,
		client: client, emit: emit, sink: sink,
		sent:      map[string]bool{},
		attempted: map[string]bool{},
		stop:      make(chan struct{}),
	}
}

// withURLs pins the pump to an explicit list instead of the pending query.
// Such a pump has nothing to wait for, so it is sealed from the start.
func (p *psiPump) withURLs(urls []string) *psiPump {
	p.fixed = urls
	p.queue = append([]string(nil), urls...)
	p.total = len(urls)
	p.sealed = true
	return p
}

// Seal tells the pump that no further pages will be crawled, which is what lets
// an empty queue mean "finished" rather than "waiting". The coordinator calls
// it when the crawl loop ends.
func (p *psiPump) Seal() {
	p.mu.Lock()
	p.sealed = true
	p.mu.Unlock()
}

// Start launches the workers and the refill loop. It does not block.
//
// gate may be nil (no crawl to follow). When it is not, a paused crawl pauses
// the measurements too: the user's Pause means "stop using my bandwidth", and a
// PageSpeed batch that kept running through it would make that button a lie.
func (p *psiPump) Start(ctx context.Context, gate *pauseGate) {
	for i := 0; i < psiConcurrency; i++ {
		p.wg.Add(1)
		// Decodes PageSpeed replies for hours after the crawl itself ends.
		safe.Go("sitecrawl/psiWorker", func() { p.worker(ctx, gate) })
	}
	p.wg.Add(1)
	safe.Go("sitecrawl/psiRefill", func() { p.refillLoop(ctx) })
}

func (p *psiPump) worker(ctx context.Context, gate *pauseGate) {
	defer p.wg.Done()
	for {
		if !p.waitWhilePaused(ctx, gate) {
			return
		}
		u, ok := p.take()
		if !ok {
			// Nothing queued yet — the crawl may not have flushed a page since
			// the last refill. Idle briefly rather than spin.
			if !p.idle(ctx, 500*time.Millisecond) {
				return
			}
			continue
		}
		r := fetchPSI(ctx, p.client, p.key, u, p.strategy)
		// A cancelled context produces a meaningless result; recording it would
		// mark the URL measured and it would never be retried.
		select {
		case <-ctx.Done():
			return
		default:
		}
		p.sink(r)
		p.finish(u)
	}
}

// waitWhilePaused blocks until the crawl resumes. false means "give up".
func (p *psiPump) waitWhilePaused(ctx context.Context, gate *pauseGate) bool {
	for gate != nil && gate.isPaused() {
		if !p.idle(ctx, 200*time.Millisecond) {
			return false
		}
	}
	select {
	case <-ctx.Done():
		return false
	case <-p.stop:
		return false
	default:
		return true
	}
}

// idle sleeps unless the pump is being torn down. false means "give up".
func (p *psiPump) idle(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-p.stop:
		return false
	case <-ctx.Done():
		return false
	}
}

func (p *psiPump) take() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.queue) == 0 {
		return "", false
	}
	u := p.queue[0]
	p.queue = p.queue[1:]
	p.sent[u] = true
	p.attempted[u] = true
	return u, true
}

func (p *psiPump) finish(u string) {
	p.mu.Lock()
	delete(p.sent, u)
	p.done++
	p.mu.Unlock()
}

// refillLoop keeps the queue stocked and reports progress. It owns the pump's
// lifetime: once the crawl is sealed and there is provably nothing left, it
// closes stop, which retires the workers too.
func (p *psiPump) refillLoop(ctx context.Context) {
	defer p.wg.Done()

	refill := time.NewTicker(psiRefillEvery)
	defer refill.Stop()
	progress := time.NewTicker(psiProgressEvery)
	defer progress.Stop()

	p.refill()
	p.emitProgress(true, false)

	for {
		select {
		case <-refill.C:
			p.refill()
			if p.exhausted() {
				p.emitProgress(false, false)
				p.Stop()
				return
			}
		case <-progress.C:
			p.emitProgress(true, false)
		case <-p.stop:
			p.emitProgress(false, true)
			return
		case <-ctx.Done():
			p.emitProgress(false, true)
			return
		}
	}
}

// exhausted reports that the run is sealed, nothing is queued or in flight, and
// the last refill found no work left.
func (p *psiPump) exhausted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sealed && len(p.queue) == 0 && len(p.sent) == 0 && p.lastAdded == 0
}

// refill tops the queue up from the pending query. A fixed list never refills.
func (p *psiPump) refill() {
	if p.fixed != nil || p.read == nil {
		return
	}

	// Counts first, so the bar keeps moving even while the queue is full.
	//
	// The measurable total is the exception: once the crawl is sealed no page
	// can be added, so the answer cannot change. Re-running it was a scan of the
	// pages table every two seconds for the whole of a PSI run — and a PSI run
	// is long by nature, because the API is rate limited.
	p.mu.Lock()
	settled := p.sealed && p.measurable > 0
	p.mu.Unlock()
	if !settled {
		if n, err := countMeasurable(p.read, p.runID); err == nil {
			p.mu.Lock()
			p.measurable = n
			p.mu.Unlock()
		}
	}
	if n, err := countMeasured(p.read, p.runID, p.strategy); err == nil {
		p.mu.Lock()
		p.measured = n
		p.mu.Unlock()
	}

	p.mu.Lock()
	needs := len(p.queue) == 0
	p.mu.Unlock()
	if !needs {
		return
	}

	urls, err := pendingPageSpeed(p.read, p.runID, p.strategy, psiQueueBatch)
	if err != nil {
		return
	}
	p.mu.Lock()
	added := 0
	for _, u := range urls {
		if p.sent[u] || p.attempted[u] {
			continue
		}
		p.queue = append(p.queue, u)
		added++
	}
	p.lastAdded = added
	p.mu.Unlock()
}

// progressNumbers: the fixed list counts itself, the query path trusts SQL so a
// resumed run shows 50/76 rather than starting over at 0/76.
func (p *psiPump) progressNumbers() (done, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fixed != nil {
		return p.done, p.total
	}
	done, total = p.measured, p.measurable
	// Results already fetched but not yet flushed by the coordinator are real
	// work the user watched happen; counting them keeps the bar from stalling.
	if inFlight := p.done - p.measured; inFlight > 0 {
		done = p.measured + inFlight
	}
	if done > total {
		done = total
	}
	return done, total
}

func (p *psiPump) emitProgress(running, stopped bool) {
	if p.emit == nil {
		return
	}
	done, total := p.progressNumbers()
	p.emit(EventPSIProgress, PSIProgressEvent{
		RunID: p.runID, Done: done, Total: total, Running: running, Stopped: stopped,
	})
}

// Stop ends the pump. Safe to call twice and from any goroutine.
func (p *psiPump) Stop() {
	p.stopOnce.Do(func() { close(p.stop) })
}

// Wait blocks until every goroutine has retired.
func (p *psiPump) Wait() { p.wg.Wait() }

// --- the queue query, in one place ---

// measurableWhere is what PageSpeed can be run against: this run's own pages,
// on this site, that are HTML and actually answered. Measuring an image, a
// redirect or a 404 would spend 20 seconds to learn nothing.
const measurableWhere = `p.run_id = ? AND p.is_internal = 1 AND p.kind = 'html' AND p.status_class = 2`

// pendingPageSpeed lists measurable URLs with no usable result yet.
//
// A stored row whose error is non-empty does NOT count as measured: a quota
// rejection or a dropped connection has to be retryable on a later run, or one
// bad minute would blank those pages permanently. Within a single pump the
// attempted set stops that from becoming a loop.
func pendingPageSpeed(db *sql.DB, runID, strategy string, limit int) ([]string, error) {
	if db == nil || runID == "" {
		return nil, nil
	}
	rows, err := db.Query(`
		SELECT p.url FROM sitecrawl_pages p
		 WHERE `+measurableWhere+`
		   AND NOT EXISTS (SELECT 1 FROM sitecrawl_psi s
		                    WHERE s.run_id = p.run_id AND s.url = p.url
		                      AND s.strategy = ? AND s.error = '')
		 ORDER BY p.inlinks DESC, p.depth ASC
		 LIMIT ?`, runID, strategy, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// countMeasurable is the denominator of the progress bar: every page PageSpeed
// could run on, measured or not.
func countMeasurable(db *sql.DB, runID string) (int, error) {
	if db == nil || runID == "" {
		return 0, nil
	}
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages p WHERE `+measurableWhere, runID).Scan(&n)
	return n, err
}

// countMeasured is the numerator: pages with a stored result that worked.
func countMeasured(db *sql.DB, runID, strategy string) (int, error) {
	if db == nil || runID == "" {
		return 0, nil
	}
	var n int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM sitecrawl_psi s
		 WHERE s.run_id = ? AND s.strategy = ? AND s.error = ''
		   AND EXISTS (SELECT 1 FROM sitecrawl_pages p
		                WHERE `+measurableWhere+` AND p.url = s.url)`,
		runID, strategy, runID).Scan(&n)
	return n, err
}

// PendingPageSpeedCount is what the PageSpeed tab's Run button counts: how many
// pages of this run still have no measurement for this device.
func (s *Service) PendingPageSpeedCount(runID, strategy string) (int, error) {
	db, err := s.readDB()
	if err != nil {
		return 0, err
	}
	if strategy != StrategyDesktop {
		strategy = StrategyMobile
	}
	measurable, err := countMeasurable(db, runID)
	if err != nil {
		return 0, err
	}
	measured, err := countMeasured(db, runID, strategy)
	if err != nil {
		return 0, err
	}
	if n := measurable - measured; n > 0 {
		return n, nil
	}
	return 0, nil
}
