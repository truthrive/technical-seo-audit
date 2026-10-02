package sitecrawl

import (
	"net/http"
	"time"
)

// Adaptive backoff bounds. Vars so tests can shrink them.
var (
	// penaltyStep is added on the first throttle signal, then doubled.
	penaltyStep = 500 * time.Millisecond
	penaltyMax  = 10 * time.Second
	// penaltyClearAfter is how many consecutive clean responses drop the penalty
	// to zero. It replaces a fixed 250ms pay-back per success, which recovered
	// linearly from a penalty that grew geometrically: five 503s took the penalty
	// to 8s and then needed 32 more responses to clear, each one spaced by the
	// 8s still in force. Measured on a 111-page crawl, that turned 0.5s into
	// 1m57s — see the perf harness.
	penaltyClearAfter = 3
)

// throttleBurst is how many requests may go out together while a host is under
// an adaptive penalty. The penalty exists to cut the request RATE, so spacing
// every single request was overkill: it collapsed the host to one request at a
// time and made the user's thread count irrelevant for as long as it lasted.
const throttleBurst = 3

// hostState is one host's pacing.
type hostState struct {
	nextAllowed time.Time
	inFlight    int
	delay       time.Duration
	penalty     time.Duration
	// clean counts consecutive good responses toward clearing the penalty.
	clean int
	// burst counts dispatches inside the current penalty window.
	burst int
	// robotsAsked records what robots.txt requested before capping, so the UI
	// can explain the difference instead of silently obeying or ignoring it.
	robotsAsked time.Duration
}

// hostGate paces requests per host.
//
// Nothing shared exists for this in the repo — the other tools spread their
// threads across many domains, so concurrency alone was pacing enough. A site
// crawler aims every thread at one host, so it needs a real gate.
//
// LibreCrawl builds a RateLimiter and calls update_rate on it, but acquire() is
// only ever reached from its JavaScript path: on a plain HTTP crawl the
// configured delay does nothing at all. Putting the gate in the dispatcher is
// what makes it impossible to bypass here.
type hostGate struct {
	hosts       map[string]*hostState
	defaultWait time.Duration
	now         func() time.Time
	perHost     int
}

// newHostGate paces one crawl.
//
// perHost is the user's thread count, unclamped. A hidden cap of 8 used to sit
// here: the Threads field offers up to 50, but a crawl of one site — the normal
// case — never went past 8, so 20 threads and 50 threads measured identically.
// Protecting the user's own server from a number the user typed is the config
// dialog's job, and it warns there instead.
func newHostGate(opts Options) *hostGate {
	return &hostGate{
		hosts:       map[string]*hostState{},
		defaultWait: opts.delay(),
		now:         time.Now,
		perHost:     opts.Concurrency,
	}
}

func (g *hostGate) state(host string) *hostState {
	s, ok := g.hosts[host]
	if !ok {
		s = &hostState{delay: g.defaultWait}
		g.hosts[host] = s
	}
	return s
}

// setRobotsDelay applies a host's Crawl-delay, already capped by the caller.
func (g *hostGate) setRobotsDelay(host string, capped, asked time.Duration) {
	s := g.state(host)
	s.robotsAsked = asked
	if capped > s.delay {
		s.delay = capped
	}
}

// robotsAsked reports the uncapped Crawl-delay a host requested, if any.
func (g *hostGate) robotsAsked(host string) time.Duration {
	if s, ok := g.hosts[host]; ok {
		return s.robotsAsked
	}
	return 0
}

// ready reports whether a request to host may be dispatched now.
func (g *hostGate) ready(host string) bool {
	s := g.state(host)
	if s.inFlight >= g.perHost {
		return false
	}
	return !g.now().Before(s.nextAllowed)
}

// reserve records a dispatch.
//
// nextAllowed advances at dispatch time, not on completion: that is what makes
// the delay a real floor on request rate. Advancing it on completion would let
// N workers fire N requests simultaneously and only then start spacing.
func (g *hostGate) reserve(host string) {
	s := g.state(host)
	s.inFlight++
	// The user's own crawl delay is a hard floor: it spaces every single request,
	// penalty included, because that is what the setting says it does.
	if s.delay > 0 {
		s.nextAllowed = g.now().Add(s.delay + s.penalty)
		return
	}
	// An adaptive penalty is a rate limit, not a serialization order, so a short
	// burst may go out inside one window. See throttleBurst.
	if s.penalty > 0 {
		s.burst++
		if s.burst >= throttleBurst {
			s.burst = 0
			s.nextAllowed = g.now().Add(s.penalty)
		}
	}
}

// release records a completed request and adjusts the penalty.
func (g *hostGate) release(host string, status int, errType string) {
	s := g.state(host)
	if s.inFlight > 0 {
		s.inFlight--
	}
	switch {
	case status == http.StatusTooManyRequests, status == http.StatusServiceUnavailable,
		errType == ErrTimeout, errType == ErrRefused:
		// The site is telling us to slow down. Back off multiplicatively and
		// hold off the next request to this host immediately, not just the one
		// after it.
		s.clean = 0
		if s.penalty == 0 {
			s.penalty = penaltyStep
		} else if s.penalty < penaltyMax {
			s.penalty *= 2
		}
		if s.penalty > penaltyMax {
			s.penalty = penaltyMax
		}
		if next := g.now().Add(s.penalty); next.After(s.nextAllowed) {
			s.nextAllowed = next
		}
	case status >= 200 && status < 400 && s.penalty > 0:
		// A few clean responses in a row mean the blip is over — clear outright.
		// A site that is genuinely struggling never reaches the count, because
		// each new throttle signal resets it and doubles the penalty again.
		s.clean++
		if s.clean >= penaltyClearAfter {
			s.penalty = 0
			s.burst = 0
			s.clean = 0
			// Drop the outstanding cool-off too. Leaving it would idle the host
			// for up to penaltyMax after it has already proven healthy — and a
			// user delay, if any, is re-applied on the next reserve.
			if s.delay == 0 {
				s.nextAllowed = time.Time{}
			}
		}
	}
}

// nextWake reports how long until some host becomes ready, so the coordinator
// can sleep instead of spinning. Zero means something is ready now.
func (g *hostGate) nextWake() time.Duration {
	now := g.now()
	best := time.Duration(-1)
	for _, s := range g.hosts {
		if s.inFlight >= g.perHost {
			continue
		}
		d := s.nextAllowed.Sub(now)
		if d <= 0 {
			return 0
		}
		if best < 0 || d < best {
			best = d
		}
	}
	if best < 0 {
		return 0
	}
	return best
}
