package httpx

import (
	"context"
	"errors"
	"net"
	"time"
)

// privateLookupTimeout bounds the one DNS lookup AllowPrivate makes. A resolver
// that is not answering must not hold up the run that is asking.
const privateLookupTimeout = 5 * time.Second

// AllowPrivate decides whether a run aimed at host may reach private addresses,
// and reports separately when the answer was "no" for a suspicious reason.
//
// Both halves are required, and that is the whole point:
//
//   - IsPrivateHost alone cannot tell a dev box from a public domain whose DNS
//     is lying. An ISP or VPN that blocks a site by answering with a private
//     address (observed on a Vietnamese ISP, and on ProtonVPN, which routes all
//     lookups through 10.2.0.1) would hand the run the user's whole LAN.
//   - IsLocalName alone would let a name that merely looks local aim anywhere.
//
// dnsBlocked is true when the name is public but resolved inward. Nothing is
// granted in that case; the caller surfaces it so the user sees "your resolver
// is redirecting this domain" instead of an unexplained connection failure.
//
// This lives here rather than in each tool because the pairing was copied into
// three call sites and two of them kept only the first half — which is exactly
// the private-network access this function exists to refuse.
func AllowPrivate(ctx context.Context, host string) (allow, dnsBlocked bool) {
	lookupCtx, cancel := context.WithTimeout(ctx, privateLookupTimeout)
	resolvesPrivate := IsPrivateHost(lookupCtx, host)
	cancel()
	return decideAllowPrivate(host, resolvesPrivate)
}

// decideAllowPrivate is the rule itself, kept apart from the lookup so it can be
// tested for the case that matters most and is hardest to arrange for real: a
// public name that resolves to a private address.
func decideAllowPrivate(host string, resolvesPrivate bool) (allow, dnsBlocked bool) {
	if !resolvesPrivate {
		return false, false
	}
	if IsLocalName(host) {
		return true, false
	}
	return false, true
}

// NeverSent reports whether err proves the request never reached the server.
//
// This is the question every paid, non-idempotent call has to answer before it
// retries. Providers that bill on acceptance — DataForSEO charges the moment it
// queues a task, Sinbyte the moment it accepts a batch — turn a wrong guess
// into a double charge: resending a request that actually arrived bills the
// user twice for the same work.
//
// Only two things are proof. DNS never resolved, so no connection was even
// attempted; or the dial itself failed, so no bytes were written. Everything
// else — a read timeout, a reset, an unexpected EOF, any 5xx — means the
// request may well have been processed and only the answer was lost.
//
// It deliberately answers "no" when unsure. Reporting a genuinely undelivered
// request as possibly-delivered costs nothing but a reconciliation lookup;
// the opposite costs the user money.
func NeverSent(err error) bool {
	if err == nil {
		return false
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) && opErr.Op == "dial" {
		return true
	}
	return false
}
