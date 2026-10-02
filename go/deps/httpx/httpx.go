// Package httpx provides the shared HTTP client used by tool packages.
// It centralises timeouts, a desktop-browser User-Agent pool, and an optional
// per-request proxy rotation hook (e.g. DataImpulse for the Wayback tool, so
// each call exits from a fresh IP and avoids Internet Archive rate limits).
package httpx

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:127.0) Gecko/20100101 Firefox/127.0",
}

// ProxyFunc returns the proxy URL to use for a request, or nil for a direct
// connection. Tools receive it from user settings (vault) — rotation providers
// like DataImpulse hand out a new IP per connection automatically.
type ProxyFunc func() *url.URL

// ErrPrivateAddress is returned when a client built with BlockPrivate resolves a
// host to an address inside the user's own network.
var ErrPrivateAddress = errors.New("refusing to connect to a private or link-local address")

// New builds an *http.Client for talking to a hardcoded provider host.
//
// Use NewFiltered for anything that fetches a URL the user or a crawled page
// supplied — this constructor applies no address filtering.
func New(timeout time.Duration, proxy ProxyFunc) *http.Client {
	return NewFiltered(timeout, proxy, false)
}

// NewLongWait is New for a provider that answers only once the work is done: an
// image API sends its headers together with the finished picture, a minute or
// more after the request. The 30-second header wait NewFiltered sets — there so
// a dead host fails fast — cut every such call off at 30s whatever timeout was
// asked for (OpenAI's Best quality and Gemini, measured 2026-09-23). Here the
// client timeout bounds the whole call; a host that cannot be reached still
// fails at the dial and TLS timeouts.
func NewLongWait(timeout time.Duration, proxy ProxyFunc) *http.Client {
	c := NewFiltered(timeout, proxy, false)
	c.Transport.(*http.Transport).ResponseHeaderTimeout = 0
	return c
}

// NewFiltered builds an *http.Client. proxy may be nil (direct connection).
// The transport carries its own connect-phase timeouts so an unreachable host
// (e.g. an ISP null-routing web.archive.org) fails in seconds — the overall
// client timeout only has to bound large body reads.
//
// blockPrivate refuses connections to the machine's own network. It matters
// because a crawler follows addresses written by whoever owns the page it is
// reading: an <img src="http://192.168.1.1/..."> makes the app issue that
// request from inside the user's network, where devices routinely skip
// authentication. The check runs in Dialer.Control, which sees the address
// *after* resolution, so it closes the DNS-rebinding gap for free.
//
// Callers decide the flag from where the run started, not from a setting:
// crawling a dev site on localhost is a first-class use case, while a public
// seed has no business reaching a LAN address. See IsPrivateHost.
//
// Requests aimed at the machine's own network never go through the proxy — see
// bypassProxy. Once they are direct, Control sees the real target address, so
// the only remaining gap is a proxy running on localhost.
func NewFiltered(timeout time.Duration, proxy ProxyFunc, blockPrivate bool) *http.Client {
	dialer := &net.Dialer{
		Timeout:   15 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	if blockPrivate {
		dialer.Control = func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip, err := netip.ParseAddr(host)
			if err != nil {
				return err
			}
			if isPrivateAddr(ip) {
				return fmt.Errorf("%w: %s", ErrPrivateAddress, ip)
			}
			return nil
		}
	}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
		// A transport is built per run and never explicitly closed. Without these
		// the pool keeps every keep-alive socket alive forever: one crawl over 500
		// hosts holds ~1000 sockets after the run ends, and a few runs exhaust
		// Windows handles and ephemeral ports.
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 4,
	}
	if proxy != nil {
		transport.Proxy = func(req *http.Request) (*url.URL, error) {
			if bypassProxy(req.URL.Hostname()) {
				return nil, nil
			}
			return proxy(), nil
		}
	}
	return &http.Client{Timeout: timeout, Transport: transport}
}

// bypassProxy reports whether host must be reached directly. A proxy cannot
// route to the user's own network, so sending it there turns "crawl my dev site
// on localhost" — a first-class use case — into an unexplained 403 from the
// proxy; it also hands internal hostnames and paths to a third party. Go's own
// ProxyFromEnvironment makes the same exemption, and so do browsers.
//
// Only literal addresses and the localhost names are checked: Transport.Proxy
// runs per request and must not resolve DNS. Anything else takes the proxy, and
// blockPrivate still catches it at dial time when the run is not allowed inward.
func bypassProxy(host string) bool {
	if host == "" {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return isPrivateAddr(ip)
	}
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// isPrivateAddr reports whether ip belongs to the machine's own network rather
// than the public internet.
func isPrivateAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	switch {
	case ip.IsLoopback(), ip.IsPrivate(), ip.IsLinkLocalUnicast(),
		ip.IsLinkLocalMulticast(), ip.IsUnspecified(), ip.IsMulticast():
		return true
	}
	// Carrier-grade NAT (100.64.0.0/10) is not covered by IsPrivate, and Alibaba
	// Cloud's metadata endpoint lives there. AWS/GCP/Azure use 169.254.169.254,
	// which IsLinkLocalUnicast already catches.
	if ip.Is4() && netip.PrefixFrom(netip.AddrFrom4([4]byte{100, 64, 0, 0}), 10).Contains(ip) {
		return true
	}
	// IPv6 unique local addresses (fc00::/7).
	if ip.Is6() && ip.As16()[0]&0xfe == 0xfc {
		return true
	}
	return false
}

// localSuffixes are the name endings reserved for, or conventionally used for,
// names that never leave a local network.
var localSuffixes = []string{
	".localhost", ".local", ".internal", ".intranet", ".lan",
	".home", ".home.arpa", ".test", ".localdomain",
}

// IsLocalName reports whether a host was plainly AIMED at the local network: an
// address literal, a name reserved for local use, or a single label with no
// domain part at all.
//
// This is deliberately a different question from IsPrivateHost. "Does it resolve
// to a private address?" cannot tell a dev site apart from a public domain a DNS
// server is filtering — an ISP blocking a site by answering 127.0.0.1 for it is
// routine, and a Vietnamese ISP was observed doing exactly that. Callers that
// grant a run permission to reach the local network must require BOTH, or a
// crawl the user aimed at the public internet silently gains that permission the
// moment someone else's DNS lies about the address.
func IsLocalName(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" {
		return false
	}
	// An address literal is as explicit as aiming gets. Callers pair this with
	// IsPrivateHost, so a public literal still grants nothing.
	if _, err := netip.ParseAddr(host); err == nil {
		return true
	}
	// A single label has no public registry behind it.
	if !strings.Contains(host, ".") {
		return true
	}
	for _, suffix := range localSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

// IsPrivateHost reports whether host resolves to an address on the machine's own
// network. Tools call it once on the seed URL to decide a run's address policy:
// a run the user aimed at localhost or a LAN host is allowed to stay there,
// while a run aimed at the public internet is not allowed to wander inward.
//
// A host that cannot be resolved is treated as public — the dialler will fail on
// it anyway, and guessing "private" here would silently widen the allowance.
func IsPrivateHost(ctx context.Context, host string) bool {
	if host == "" {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		return isPrivateAddr(ip)
	}
	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil || len(addrs) == 0 {
		return false
	}
	for _, a := range addrs {
		if isPrivateAddr(a) {
			return true
		}
	}
	return false
}

// BrowserHeaders sets a randomized desktop-browser User-Agent on a request.
func BrowserHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgents[rand.IntN(len(userAgents))])
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
}
