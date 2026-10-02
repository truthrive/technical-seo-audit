package sitecrawl

import (
	"errors"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	errEmptyURL          = errors.New("empty URL")
	errInvalidURL        = errors.New("invalid URL")
	errUnsupportedScheme = errors.New("unsupported scheme")
)

// trackingParams never identify a different page, so they are dropped before a
// URL becomes a frontier key. Without this, one campaign link per page turns a
// 500-page site into a 5000-page crawl.
var trackingParams = map[string]bool{
	"gclid": true, "fbclid": true, "yclid": true, "msclkid": true,
	"mc_eid": true, "mc_cid": true, "igshid": true, "_ga": true,
	"gbraid": true, "wbraid": true, "dclid": true, "ttclid": true,
}

const trackingPrefix = "utm_"

func isTrackingParam(key string) bool {
	k := strings.ToLower(key)
	return trackingParams[k] || strings.HasPrefix(k, trackingPrefix)
}

// A local copy of indexcheck's scheme disambiguation: the tool packages stay
// independent of each other by design, so shared plumbing is duplicated rather
// than extracted.
//
//   - schemeAuthorityRe needs the "//", so it only ever matches a real scheme.
//   - opaqueSchemeRe catches authority-less schemes (mailto:, tel:, javascript:)
//     by requiring a non-digit after the colon, which is what keeps
//     "example.com:8080/a" out of it.
var (
	schemeAuthorityRe = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*://`)
	opaqueSchemeRe    = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.\-]*:(?:[^0-9/]|$)`)
)

// normalizeInput turns a typed or pasted line into an absolute http(s) URL.
func normalizeInput(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errEmptyURL
	}
	switch {
	case strings.HasPrefix(s, "//"):
		s = "https:" + s
	case schemeAuthorityRe.MatchString(s):
	case opaqueSchemeRe.MatchString(s):
		return "", errUnsupportedScheme
	default:
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", errInvalidURL
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return "", errUnsupportedScheme
	}
	if u.Host == "" {
		return "", errInvalidURL
	}
	if u.Path == "" {
		u.Path = "/"
	}
	u.Fragment = ""
	return u.String(), nil
}

// frontierKey is this crawler's identity for a URL: the visited set, the URL
// dictionary and the "have I queued this?" check all key on it.
//
// It is deliberately NOT indexcheck's matchKey. That one folds away the scheme,
// a leading "www." and the trailing slash because to Google those are the same
// page — but they are precisely the differences a site audit exists to surface.
// Fold them here and http://x/ and https://www.x collapse into one node, hiding
// every http→https and www-canonicalisation problem on the site.
//
// So: scheme kept, host lowercased but "www." kept, default port dropped,
// path case AND its trailing slash significant, fragment dropped, tracking
// params dropped, remaining params sorted.
func frontierKey(rawURL string, ignoreQuery bool) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		// Unparseable URLs must never accidentally equal each other.
		return "\x00" + strings.TrimSpace(rawURL)
	}

	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	if port := u.Port(); port != "" && !isDefaultPort(scheme, port) {
		host += ":" + port
	}

	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}

	key := scheme + "://" + host + p
	if ignoreQuery {
		return key
	}
	q := u.Query()
	for name := range q {
		if isTrackingParam(name) {
			q.Del(name)
		}
	}
	if encoded := q.Encode(); encoded != "" { // Encode sorts by key
		key += "?" + encoded
	}
	return key
}

// redirectKey identifies a URL for redirect-loop detection only.
//
// It keeps everything a redirect can legitimately change — scheme, www, the
// trailing slash — because folding those reports an ordinary apex→www hop as a
// loop and truncates the chain at the first 301.
func redirectKey(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return strings.TrimSpace(rawURL)
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	u.Fragment = "" // never sent, so it cannot be what a server redirects to
	return u.String()
}

func isDefaultPort(scheme, port string) bool {
	switch strings.ToLower(scheme) {
	case "http":
		return port == "80"
	case "https":
		return port == "443"
	}
	return false
}

// registrableHost strips a leading "www." so apex and www count as one site.
func registrableHost(host string) string {
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}

// sameSite decides whether a URL counts as internal to the crawl.
//
// Default is LibreCrawl's rule — same host ignoring "www." — which makes
// blog.example.com external. subdomains widens it to any host under the same
// registrable domain, which is Screaming Frog's "Crawl All Subdomains".
func sameSite(seedHost, host string, subdomains bool) bool {
	a, b := registrableHost(seedHost), registrableHost(host)
	if a == b {
		return true
	}
	if !subdomains {
		return false
	}
	root := rootDomain(a)
	return root != "" && rootDomain(b) == root
}

// rootDomain returns the last two labels of a host, plus a third for the common
// multi-part public suffixes.
//
// This is a heuristic, not a public-suffix-list lookup: pulling in a PSL adds a
// dependency and a periodically-stale data file to decide something the user
// can already override with the subdomains switch.
func rootDomain(host string) string {
	host = strings.TrimSuffix(host, ".")
	parts := strings.Split(host, ".")
	if len(parts) < 2 {
		return host
	}
	last2 := strings.Join(parts[len(parts)-2:], ".")
	if len(parts) >= 3 && multiPartSuffix[last2] {
		return strings.Join(parts[len(parts)-3:], ".")
	}
	return last2
}

// multiPartSuffix covers the suffixes common enough that getting them wrong
// would be visible — notably the Vietnamese second-level domains.
var multiPartSuffix = map[string]bool{
	"com.vn": true, "net.vn": true, "org.vn": true, "edu.vn": true,
	"gov.vn": true, "biz.vn": true, "info.vn": true, "name.vn": true,
	"co.uk": true, "org.uk": true, "ac.uk": true, "gov.uk": true,
	"com.au": true, "net.au": true, "org.au": true, "edu.au": true,
	"co.jp": true, "or.jp": true, "ne.jp": true, "ac.jp": true,
	"com.br": true, "com.cn": true, "com.tw": true, "co.kr": true,
	"com.sg": true, "com.my": true, "co.th": true, "co.id": true,
	"com.hk": true, "co.nz": true, "com.mx": true, "co.in": true,
}

// resolveLink turns an href found on base into an absolute http(s) URL.
// Empty result means the link is not crawlable (fragment-only, mailto:, data:,
// javascript:, or unparseable).
func resolveLink(base *url.URL, href string) string {
	h := strings.TrimSpace(href)
	if h == "" || strings.HasPrefix(h, "#") {
		return ""
	}
	switch {
	case strings.HasPrefix(strings.ToLower(h), "mailto:"),
		strings.HasPrefix(strings.ToLower(h), "tel:"),
		strings.HasPrefix(strings.ToLower(h), "javascript:"),
		strings.HasPrefix(strings.ToLower(h), "data:"),
		strings.HasPrefix(strings.ToLower(h), "sms:"),
		strings.HasPrefix(strings.ToLower(h), "ftp:"):
		return ""
	}
	ref, err := url.Parse(h)
	if err != nil {
		return ""
	}
	abs := base.ResolveReference(ref)
	switch strings.ToLower(abs.Scheme) {
	case "http", "https":
	default:
		return ""
	}
	if abs.Host == "" {
		return ""
	}
	abs.Fragment = ""
	return abs.String()
}

// urlExtension returns the lowercased extension of a URL's path, without the
// dot. "" when the path has none.
func urlExtension(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	ext := strings.ToLower(path.Ext(u.Path))
	return strings.TrimPrefix(ext, ".")
}

// dedupe removes duplicate input lines, first occurrence winning, order kept.
// Only scheme and host are case-folded: paths are case-sensitive, so /Foo and
// /foo have to survive as two entries.
func dedupe(urls []string) []string {
	seen := make(map[string]struct{}, len(urls))
	out := make([]string, 0, len(urls))
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" {
			continue
		}
		key := dedupeKey(u)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, u)
	}
	return out
}

func dedupeKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return raw
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return u.String()
}
