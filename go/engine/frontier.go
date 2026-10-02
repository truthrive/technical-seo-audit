package sitecrawl

import (
	"net/url"
	"regexp"
	"strings"
)

// frontierItem is one URL waiting to be fetched.
type frontierItem struct {
	URL      string
	Depth    int
	Source   string
	ParentID int64
	// ID is the dictionary id, allocated at admission so the result can be
	// stored without another map lookup.
	ID int64
	// host is cached because the politeness gate needs it on every dispatch
	// decision, and re-parsing the URL each time is measurable at 50k URLs.
	host string
}

// frontier is the crawl queue plus the URL dictionary.
//
// A slice used as a FIFO with a head index gives breadth-first order for free:
// children are appended, so everything at depth N is dequeued before anything
// at depth N+1.
//
// seen doubles as the visited set AND the id allocator — one map lookup answers
// both "have I met this URL?" and "what is its dictionary id?". Doing that in
// SQL would be 5M single-connection reads interleaved with the crawl's writes.
type frontier struct {
	queue []frontierItem
	head  int

	seen map[string]int64
	// notQueued holds keys that have a dictionary id but never entered the
	// queue: link-graph targets recorded from a non-followed page, resources
	// whose crawl option is off, URLs that failed an admission check. admit
	// re-offers these — a URL's first sighting being passive must not mean it
	// is never crawled when a real link to it turns up later. (A resumed run
	// starts this set empty: forfeiting retry offers is the cost of not
	// persisting it, and matches the pre-checkpoint behaviour.)
	notQueued map[string]struct{}
	nextID    int64
	// pending are dictionary entries not yet flushed to sitecrawl_urls.
	pending []urlEntry

	opts     Options
	seedHost string

	includeExt map[string]bool
	excludeExt map[string]bool
	includeRe  []*regexp.Regexp
	excludeRe  []*regexp.Regexp

	// counters
	skipped int
	// hitURLCap / hitDepthCap record that a limit actually turned URLs away, so
	// a crawl that ends early can say which limit ended it instead of completing
	// with a blank reason and leaving the user to infer it from the count.
	hitURLCap   bool
	hitDepthCap bool
	// queuedTotal counts URLs that actually entered the queue. This — not
	// len(seen) — is what "found" means to the user: the dictionary also holds
	// every skipped external (CDN scripts, social links, foreign sitemap
	// entries), and counting those once showed "1 / 91 URLs" on a site whose
	// sitemap belonged to another domain.
	queuedTotal int
}

type urlEntry struct {
	ID  int64
	URL string
}

func newFrontier(opts Options, seedHost string) *frontier {
	f := &frontier{
		seen:      map[string]int64{},
		notQueued: map[string]struct{}{},
		nextID:    1,
		opts:      opts,
		seedHost:  seedHost,
	}
	f.includeExt = extSet(opts.IncludeExtensions)
	f.excludeExt = extSet(opts.ExcludeExtensions)
	f.includeRe = compilePatterns(opts.IncludePatterns)
	f.excludeRe = compilePatterns(opts.ExcludePatterns)
	return f
}

func extSet(list []string) map[string]bool {
	if len(list) == 0 {
		return nil
	}
	m := make(map[string]bool, len(list))
	for _, e := range list {
		e = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(e, ".")))
		if e != "" {
			m[e] = true
		}
	}
	return m
}

// compilePatterns compiles user regexes, silently dropping the invalid ones.
// A typo in one pattern must not abort a crawl the user already started.
func compilePatterns(list []string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range list {
		if re, err := regexp.Compile(p); err == nil {
			out = append(out, re)
		}
	}
	return out
}

// idFor returns the dictionary id for a key, allocating one on first sight.
// A fresh entry starts as not-queued; admit clears that when it queues.
//
// It answers 0 when the dictionary is full. MaxURLs has to be enforced here
// rather than in admit, because collectLinks calls idFor directly for every
// link, image, stylesheet and script it does not follow — up to 5900 per page.
// Capping only the queue left the maps growing one entry per distinct URL ever
// sighted, with nothing trimming them, so a link farm or a calendar with endless
// query permutations exhausted memory long after the crawl stopped queueing.
// Callers already tolerate a 0 dst id: it is what an uncrawlable target gets.
func (f *frontier) idFor(key, rawURL string) (int64, bool) {
	if id, ok := f.seen[key]; ok {
		return id, false
	}
	if len(f.seen) >= f.opts.MaxURLs {
		return 0, false
	}
	id := f.nextID
	f.nextID++
	f.seen[key] = id
	f.notQueued[key] = struct{}{}
	f.pending = append(f.pending, urlEntry{ID: id, URL: rawURL})
	return id, true
}

// takePending hands the coordinator the dictionary rows to flush.
func (f *frontier) takePending() []urlEntry {
	if len(f.pending) == 0 {
		return nil
	}
	out := f.pending
	f.pending = nil
	return out
}

// admit offers a discovered URL to the queue, reporting whether it was queued
// and the dictionary id it was given (allocated even when not queued, so the
// link graph can still record an edge to it).
//
// Checks run cheapest-first. robots.txt is deliberately NOT consulted here: it
// costs a network fetch, and doing it at admission time would block discovery
// behind it. The fetch path checks instead.
func (f *frontier) admit(rawURL string, depth int, source string, parentID int64) (id int64, queued bool) {
	key := frontierKey(rawURL, f.opts.IgnoreQueryParam)
	id, fresh := f.idFor(key, rawURL)
	if id == 0 {
		f.skipped++ // dictionary full; ids start at 1, so 0 can only mean that
		f.hitURLCap = true
		return 0, false
	}
	if !fresh {
		if _, retry := f.notQueued[key]; !retry {
			return id, false // already queued or crawled
		}
	}
	// skipped counts each URL once, at first sight; a re-offer that fails again
	// stays counted, one that succeeds stays counted too (the discovery WAS
	// declined once — the counter reports declined offers, not final fates).
	decline := func() (int64, bool) {
		if fresh {
			f.skipped++
		}
		return id, false
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return decline()
	}
	internal := sameSite(f.seedHost, u.Hostname(), f.opts.crawlSubdomains())

	switch {
	case depth > f.opts.MaxDepth:
		f.hitDepthCap = true
		return decline()
	case !internal && !f.opts.CrawlExternal:
		// External URLs are still recorded so the External tab can report their
		// status later if the user turns the option on; they are just not queued.
		return decline()
	case !f.allowedByExtension(rawURL):
		return decline()
	case !f.allowedByPattern(rawURL):
		return decline()
	}

	delete(f.notQueued, key)
	f.queue = append(f.queue, frontierItem{
		URL: rawURL, Depth: depth, Source: source, ParentID: parentID,
		ID: id, host: strings.ToLower(u.Host),
	})
	f.queuedTotal++
	return id, true
}

// allowedByExtension applies the include/exclude extension lists.
//
// An empty include list means "anything", which is what makes the default
// behaviour crawl a whole site rather than only .html.
func (f *frontier) allowedByExtension(rawURL string) bool {
	ext := urlExtension(rawURL)
	if f.excludeExt[ext] {
		return false
	}
	if len(f.includeExt) == 0 {
		return true
	}
	// An extension-less URL is a page, not a resource, so it always passes an
	// include list — otherwise "html,htm,php" would exclude the homepage.
	if ext == "" {
		return true
	}
	return f.includeExt[ext]
}

func (f *frontier) allowedByPattern(rawURL string) bool {
	for _, re := range f.excludeRe {
		if re.MatchString(rawURL) {
			return false
		}
	}
	if len(f.includeRe) == 0 {
		return true
	}
	for _, re := range f.includeRe {
		if re.MatchString(rawURL) {
			return true
		}
	}
	return false
}

func (f *frontier) empty() bool { return f.head >= len(f.queue) }
func (f *frontier) queued() int { return len(f.queue) - f.head }

// discovered is the user-facing "found" number: URLs the crawl has fetched or
// will fetch. On a clean completion it converges to exactly the crawled count.
func (f *frontier) discovered() int { return f.queuedTotal }

// peekReady returns the index of the first queued item whose host is ready to
// be hit, or -1.
//
// Scanning forward rather than only looking at the head is what keeps one slow
// host from blocking a crawl that spans several (external resources, CDNs).
func (f *frontier) peekReady(gate *hostGate) int {
	for i := f.head; i < len(f.queue); i++ {
		if gate.ready(f.queue[i].host) {
			return i
		}
	}
	return -1
}

// at reads a queued item without removing it. The dispatcher peeks with this
// and only calls take once the hand-off to a worker has actually happened.
func (f *frontier) at(i int) frontierItem { return f.queue[i] }

// requeue puts an already-dequeued item back at the end of the queue, for the
// one retry a temporarily-refused URL gets. It skips every admission check and
// the dictionary on purpose: this URL has already passed both, and re-admitting
// it would be rejected as a duplicate. queuedTotal is not touched either — the
// URL was counted as found the first time, and counting it twice would make
// "found" climb while nothing new was discovered.
func (f *frontier) requeue(item frontierItem) {
	f.queue = append(f.queue, item)
}

// take removes the item at index i, preserving queue order.
func (f *frontier) take(i int) frontierItem {
	item := f.queue[i]
	if i == f.head {
		f.queue[f.head] = frontierItem{} // release the strings
		f.head++
	} else {
		copy(f.queue[f.head+1:i+1], f.queue[f.head:i])
		f.queue[f.head] = frontierItem{}
		f.head++
	}
	f.compact()
	return item
}

// compact reclaims the consumed prefix once it dominates the slice, so a long
// crawl does not hold every URL it ever dequeued.
func (f *frontier) compact() {
	if f.head < 1024 || f.head < len(f.queue)/2 {
		return
	}
	rest := f.queue[f.head:]
	f.queue = append(f.queue[:0], rest...)
	f.head = 0
}

// snapshot returns the outstanding queue for the checkpoint.
func (f *frontier) snapshot() []frontierItem {
	if f.empty() {
		return nil
	}
	out := make([]frontierItem, f.queued())
	copy(out, f.queue[f.head:])
	return out
}

// restore rebuilds a frontier from a checkpoint plus the stored dictionary.
// priorCrawled seeds queuedTotal so "found" keeps counting pages fetched
// before the pause on top of what is still queued.
func (f *frontier) restore(seen map[string]int64, nextID int64, items []frontierItem, priorCrawled int) {
	f.seen = seen
	if nextID > 1 {
		f.nextID = nextID
	}
	f.queue = f.queue[:0]
	f.head = 0
	f.queuedTotal = priorCrawled + len(items)
	for _, it := range items {
		if u, err := url.Parse(it.URL); err == nil {
			it.host = strings.ToLower(u.Host)
		}
		// The dictionary already knows these URLs, so reuse the stored id — a
		// fresh one would orphan every edge recorded before the pause.
		key := frontierKey(it.URL, f.opts.IgnoreQueryParam)
		if id, ok := f.seen[key]; ok {
			it.ID = id
		} else {
			// A queued URL with no dictionary row means the two were written out
			// of step. Allocating rather than storing 0 keeps every page on its own
			// row: writePages upserts on (run_id, url_id), so a shared 0 would make
			// them overwrite each other.
			it.ID = f.nextID
			f.nextID++
			f.seen[key] = it.ID
			f.pending = append(f.pending, urlEntry{ID: it.ID, URL: it.URL})
		}
		f.queue = append(f.queue, it)
	}
}
