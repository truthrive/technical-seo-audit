// Package safe stops one goroutine's panic from killing the whole app.
//
// Go has no per-goroutine recovery: a panic anywhere unwinds that goroutine
// and then takes the process down with it. jobs.Runner works this out for the
// goroutine it starts, and says so:
//
//	Job bodies decode untrusted images and parse HTML from crawled sites, so a
//	codec panic is a realistic input rather than only a programming mistake.
//
// The trouble is that the decoding it describes does not happen on that
// goroutine. Every tool fans work out to a pool, and those children were
// outside the guard — so a single malformed PNG or a hostile page took the
// window away with no dialog, losing every other tool's in-flight run too.
//
// The three shapes here match the three the tools actually have:
//
//	Go    — a long-lived background goroutine (a ticker, a scheduler, a socket)
//	Do    — a unit of work that returns nothing (kwcluster's shard workers)
//	Call  — a unit of work that must produce a value (every mapPool item)
//
// Recovering is not the same as pretending nothing happened: every recovery is
// logged with its stack, and Call makes the caller decide what a failed item
// looks like so the batch reports a failure instead of a silent hole.
package safe

import (
	"log/slog"
	"runtime/debug"
)

// Go runs fn on a new goroutine, surviving a panic in it.
//
// For goroutines that outlive a single call — the updater's poll ticker, the
// rank-check scheduler, the shortlink live socket. Losing one of those to a
// panic used to end the process; now it ends only that loop, and leaves a
// record saying which one and why.
func Go(name string, fn func()) {
	go func() { Do(name, fn) }()
}

// Do runs fn, converting a panic into a logged event. It reports whether fn
// panicked, so a caller that needs to count failures can.
//
// Put the pool's own bookkeeping OUTSIDE the call:
//
//	go func() {
//		defer wg.Done()          // runs after Do has finished logging
//		Do("pool/item", func() { … })
//	}()
//
// Inside fn, `defer wg.Done()` runs while the panic is still unwinding — the
// waiter is released before Do has recovered and written the record, so the
// caller can observe the work as complete while the report is still in flight.
func Do(name string, fn func()) (panicked bool) {
	defer func() {
		if p := recover(); p != nil {
			panicked = true
			logPanic(name, p)
		}
	}()
	fn()
	return false
}

// Recovered is the deferred form: `defer safe.Recovered("name")` at the top of
// a goroutine body.
//
// For bodies whose control flow makes a closure awkward — a worker loop with
// several `return`s, where wrapping would silently turn "leave the goroutine"
// into "leave the closure and carry on". recover() works here because
// Recovered is itself the deferred function.
func Recovered(name string) {
	if p := recover(); p != nil {
		logPanic(name, p)
	}
}

// Call runs fn and returns its result. If fn panics, the panic is logged and
// onPanic supplies the result in its place.
//
// onPanic is required rather than optional: this is used on the item path of
// every worker pool, and a zero value there would quietly mean "this URL was
// fine" or "this image needed no work". The caller has to say what a failure
// looks like in its own result type.
func Call[R any](name string, fn func() R, onPanic func(recovered any) R) (result R) {
	defer func() {
		if p := recover(); p != nil {
			logPanic(name, p)
			result = onPanic(p)
		}
	}()
	return fn()
}

// logPanic records a recovered panic. It goes through slog, so on a shipped
// build it lands in the log file rather than the stderr that -H windowsgui
// leaves nowhere.
func logPanic(name string, p any) {
	slog.Error("recovered panic",
		"where", name,
		"panic", p,
		"stack", string(debug.Stack()),
	)
}
