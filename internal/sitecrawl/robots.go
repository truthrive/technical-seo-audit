package sitecrawl

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

// Robots verdicts.
const (
	RobotsAllowed = "allowed"
	RobotsBlocked = "blocked"
	RobotsUnknown = "unknown"
)

// robotsByteCap bounds a robots.txt read. Real files are a few KB; a site that
// serves half a gigabyte there is not worth parsing.
const robotsByteCap = 512 << 10

// robotsVerdict is one URL's robots.txt outcome.
type robotsVerdict struct {
	State string
	// Rule is the matched directive, for display only ("Disallow: /admin").
	Rule string
}

// robotsCache fetches each host's robots.txt exactly once per run.
//
// A local copy of indexcheck's cache — the tool packages stay independent of
// each other by design, so shared plumbing is duplicated rather than extracted.
// Two things are added here that a crawler needs and an index checker does not:
// Crawl-delay, and the Sitemap: directives that seed the frontier.
type robotsCache struct {
	client *http.Client
	// tokens are the User-agent names to evaluate as, most specific first.
	tokens []string
	// timeout bounds one robots.txt fetch on its own, independently of whichever
	// URL happened to trigger it.
	timeout time.Duration
	ua      uaPreset

	mu    sync.Mutex
	hosts map[string]*robotsEntry
}

type robotsEntry struct {
	once sync.Once
	data *robotstxt.RobotsData
	raw  []byte
	// lines is raw split once, at fetch time. matchedDisallow needs it on every
	// blocked URL purely to name the rule for the UI, and robotsByteCap allows a
	// 512KB file — re-splitting per URL meant allocating and scanning half a
	// megabyte up to MaxURLs times against a site that blocks everything.
	lines []string
	// state is RobotsUnknown when the file could not be read (5xx or network).
	// Google treats 5xx as a temporary full disallow, but reporting a flaky
	// robots.txt as "blocked" in a diagnostic tool would actively mislead —
	// unknown is the honest answer.
	state string
	// delay is the Crawl-delay the most specific matching group asked for,
	// already capped. Zero means the file said nothing.
	delay time.Duration
	// delayRaw is what the file actually asked for, so the UI can say
	// "robots.txt asks 10s, capped at 5s" instead of silently doing either.
	delayRaw time.Duration
	sitemaps []string
}

func newRobotsCache(client *http.Client, ua uaPreset, timeout time.Duration) *robotsCache {
	return &robotsCache{
		client:  client,
		tokens:  ua.robotsTokens(),
		timeout: timeout,
		ua:      ua,
		hosts:   map[string]*robotsEntry{},
	}
}

func (c *robotsCache) entryFor(host string) *robotsEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.hosts[host]
	if !ok {
		e = &robotsEntry{}
		c.hosts[host] = e
	}
	return e
}

func originOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
}

// load fetches and parses the entry, exactly once per origin.
func (c *robotsCache) load(ctx context.Context, origin string) *robotsEntry {
	e := c.entryFor(origin)
	e.once.Do(func() { e.fetch(ctx, c.client, c.ua, origin, c.timeout) })
	return e
}

// Check reports whether the URL may be crawled.
func (c *robotsCache) Check(ctx context.Context, rawURL string) robotsVerdict {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return robotsVerdict{State: RobotsUnknown}
	}
	e := c.load(ctx, strings.ToLower(u.Scheme)+"://"+strings.ToLower(u.Host))

	if e.state != "" {
		return robotsVerdict{State: e.state}
	}
	if e.data == nil {
		return robotsVerdict{State: RobotsUnknown}
	}

	p := u.EscapedPath()
	if p == "" {
		p = "/"
	}
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}

	for _, token := range c.tokens {
		if !e.data.TestAgent(p, token) {
			return robotsVerdict{State: RobotsBlocked, Rule: matchedDisallow(e.lines, token, p)}
		}
	}
	return robotsVerdict{State: RobotsAllowed}
}

// Delay returns the capped Crawl-delay for a host, and what the file asked for
// before capping.
func (c *robotsCache) Delay(ctx context.Context, origin string) (capped, asked time.Duration) {
	e := c.load(ctx, origin)
	return e.delay, e.delayRaw
}

// Sitemaps returns the Sitemap: URLs declared for a host.
func (c *robotsCache) Sitemaps(ctx context.Context, origin string) []string {
	return c.load(ctx, origin).sitemaps
}

func (e *robotsEntry) fetch(ctx context.Context, client *http.Client, ua uaPreset, origin string, timeout time.Duration) {
	// Detached from the caller's per-URL deadline: this entry is cached for the
	// whole run, so one slow URL timing out must not leave every later URL on
	// the same host reporting an unreadable robots.txt.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/robots.txt", nil)
	if err != nil {
		e.state = RobotsUnknown
		return
	}
	ua.apply(req, "", nil)

	res, err := client.Do(req)
	if err != nil {
		e.state = RobotsUnknown
		return
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, robotsByteCap))

	switch {
	case res.StatusCode >= 200 && res.StatusCode < 300:
		data, err := robotstxt.FromBytes(body)
		if err != nil || data == nil {
			e.state = RobotsUnknown
			return
		}
		e.data = data
		e.raw = body
		e.lines = strings.Split(string(body), "\n")
	case res.StatusCode >= 400 && res.StatusCode < 500:
		// Google: 4xx means no robots.txt exists — full allow.
		e.state = RobotsAllowed
	default:
		e.state = RobotsUnknown
	}

	// Sitemap: and Crawl-delay are read off the raw bytes either way: a 4xx
	// leaves nothing to read, but a parsed file may still carry both.
	if len(body) > 0 && res.StatusCode < 400 {
		e.sitemaps = parseSitemapLines(body)
		e.delayRaw = parseCrawlDelay(body, ua.robotsTokens())
		e.delay = e.delayRaw
		if e.delay > maxRobotsDelayMs*time.Millisecond {
			e.delay = maxRobotsDelayMs * time.Millisecond
		}
	}
}

// parseSitemapLines pulls the Sitemap: directives out of a robots.txt.
//
// The value is everything after the FIRST colon, which is the field separator —
// splitting on every colon would truncate "https://…" to "//…".
func parseSitemapLines(raw []byte) []string {
	var out []string
	for _, line := range strings.Split(string(raw), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(line[:colon]), "sitemap") {
			continue
		}
		if v := strings.TrimSpace(line[colon+1:]); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// parseCrawlDelay reads the Crawl-delay of the most specific matching group.
// A named group always beats the "*" group.
//
// temoto/robotstxt exposes this per-group, but only through a Group lookup that
// does not tell us whether the match was named or wildcard — and the directive
// is non-standard enough that scanning for it directly is clearer than trusting
// a library's interpretation of a field it does not otherwise use.
func parseCrawlDelay(raw []byte, tokens []string) time.Duration {
	var (
		best     float64 = -1
		specific bool
		inGroup  bool
		starOnly bool
	)
	for _, line := range strings.Split(string(raw), "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		field := strings.ToLower(strings.TrimSpace(line[:colon]))
		value := strings.TrimSpace(line[colon+1:])

		switch field {
		case "user-agent":
			agent := strings.ToLower(value)
			starOnly = agent == "*"
			inGroup = starOnly
			for _, t := range tokens {
				if strings.HasPrefix(strings.ToLower(t), agent) {
					inGroup = true
					break
				}
			}
		case "crawl-delay":
			if !inGroup {
				continue
			}
			secs, err := strconv.ParseFloat(value, 64)
			if err != nil || secs < 0 {
				continue
			}
			named := !starOnly
			if named && !specific {
				best, specific = secs, true
			} else if named == specific && (best < 0 || secs > best) {
				best = secs
			}
		}
	}
	if best <= 0 {
		return 0
	}
	return time.Duration(best * float64(time.Second))
}

// matchedDisallow finds the Disallow line that most likely produced a block, so
// the UI can say *which* rule blocked the URL.
//
// The library keeps its rules private, so this is an independent scan used for
// display only — the verdict itself always comes from the library's
// spec-correct matcher. It picks the longest matching Disallow pattern in the
// most specific group, mirroring how the real matcher breaks ties.
// lines is the robots.txt already split, cached on the entry — see robotsEntry.
func matchedDisallow(lines []string, token, path string) string {
	if len(lines) == 0 {
		return ""
	}
	token = strings.ToLower(token)

	var (
		best      string
		bestLen   = -1
		inGroup   bool
		groupStar bool
		// bestSpecific records whether `best` came from a named group, which
		// always outranks a rule from the "*" group.
		bestSpecific bool
	)

	for _, line := range lines {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		field := strings.ToLower(strings.TrimSpace(line[:colon]))
		value := strings.TrimSpace(line[colon+1:])

		switch field {
		case "user-agent":
			agent := strings.ToLower(value)
			inGroup = agent == "*" || strings.HasPrefix(token, agent)
			groupStar = agent == "*"
		case "disallow":
			if !inGroup || value == "" {
				continue
			}
			if !robotsPatternMatches(value, path) {
				continue
			}
			specific := !groupStar
			if (specific && !bestSpecific) || (specific == bestSpecific && len(value) > bestLen) {
				best, bestLen, bestSpecific = value, len(value), specific
			}
		}
	}
	if best == "" {
		return ""
	}
	return "Disallow: " + best
}

// robotsPatternMatches implements the robots.txt path pattern rules: prefix
// match, `*` as any sequence, `$` anchoring the end.
func robotsPatternMatches(pattern, path string) bool {
	anchored := strings.HasSuffix(pattern, "$")
	if anchored {
		pattern = strings.TrimSuffix(pattern, "$")
	}
	parts := strings.Split(pattern, "*")

	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			if !strings.HasPrefix(path[pos:], part) {
				return false
			}
			pos += len(part)
			continue
		}
		idx := strings.Index(path[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	if anchored {
		// The last literal must land exactly at the end, unless the pattern
		// ended with a wildcard (which can absorb the remainder).
		if last := parts[len(parts)-1]; last != "" {
			return pos == len(path)
		}
	}
	return true
}
