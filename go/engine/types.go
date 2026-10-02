package sitecrawl

import "onescout/desktop/internal/core/runs"

import (
	"strings"
	"time"
)

// Crawl modes. Spider discovers URLs by following links; List crawls exactly
// the URLs it is handed and never follows anything (Screaming Frog's two modes).
const (
	ModeSpider = "spider"
	ModeList   = "list"
)

// PageSpeed device. One choice per run, like Screaming Frog: the two are
// separate measurements, and a run that mixed them could not be read as a
// single verdict.
const (
	StrategyMobile  = "mobile"
	StrategyDesktop = "desktop"
)

// Run states. "paused" is the one this tool adds over the other tools: the job
// goroutine is alive and holding the frontier, so it is neither running nor
// terminal. See RunStateEvent for why jobs.Progress cannot carry it.
const (
	StateRunning     = runs.StateRunning
	StatePaused      = runs.StatePaused
	StateCompleted   = runs.StateCompleted
	StateCancelled   = runs.StateCancelled
	StateStopped     = runs.StateStopped
	StateFailed      = runs.StateFailed
	StateInterrupted = runs.StateInterrupted
	// StateDeleting marks a run whose rows are still being removed in chunks.
	// Deleting a large crawl takes minutes, and the row used to stay in history
	// the whole time — visible, and clickable a second time into a duplicate
	// delete job. Claiming this state is what hides it and makes the claim
	// exclusive.
	StateDeleting = "deleting"
)

// Crawl phases, persisted on the run row so a crash mid-analysis can re-finalize.
const (
	PhasePreparing  = "preparing"
	PhaseCrawling   = "crawling"
	PhaseAnalyzing  = "analyzing"
	PhaseDuplicates = "duplicates"
	PhaseDone       = "done"
)

// Stop reasons, kebab-case slugs doubling as i18n keys.
const (
	StopMaxURLs         = "max-urls"
	StopMaxDepth        = "max-depth"
	StopSeedUnreachable = "seed-unreachable"
	// StopDNSBlocked: the seed's own name resolved into the machine's network.
	// Almost always a DNS server filtering the domain rather than a real address,
	// and the user can fix it by changing resolver — so it is worth naming apart
	// from a plain unreachable seed.
	StopDNSBlocked    = "dns-blocked"
	StopRobotsBlocked = "robots-blocked"
	StopUser          = "user"
	// StopSeedRedirect: the start URL 301s to another domain, so the crawl
	// stayed at one page. Screaming Frog's most-asked question — the UI must
	// say it and offer to crawl the target instead.
	StopSeedRedirect = "seed-redirect"
)

// Resource kinds, used as the grid's tab predicate.
const (
	KindHTML  = "html"
	KindImage = "image"
	KindCSS   = "css"
	KindJS    = "js"
	KindPDF   = "pdf"
	KindFont  = "font"
	KindMedia = "media"
	KindOther = "other"
)

// How a URL entered the crawl. Orphan detection needs to tell "found in the
// sitemap" from "found by following a link".
const (
	SourceSeed     = "seed"
	SourceLink     = "link"
	SourceSitemap  = "sitemap"
	SourceRedirect = "redirect"
	SourceManual   = "manual"
)

// Link placement, stored as a small int on the edge row.
const (
	PlacementBody   = 0
	PlacementNav    = 1
	PlacementFooter = 2
	PlacementHeader = 3
	PlacementImage  = 4
)

// Edge flag bits.
const (
	FlagInternal   uint32 = 1 << 0
	FlagNofollow   uint32 = 1 << 1
	FlagUGC        uint32 = 1 << 2
	FlagSponsored  uint32 = 1 << 3
	FlagImageLink  uint32 = 1 << 4
	FlagStylesheet uint32 = 1 << 5
	FlagScript     uint32 = 1 << 6
)

// Issue severities. Higher is worse; stored as an int so SQL can sort and
// MAX() them without a lookup table.
const (
	SeverityNotice   = 1
	SeverityWarning  = 2
	SeverityCritical = 3
)

// Error classes recorded when a fetch never produced a response.
const (
	ErrDNS        = "dns-not-found"
	ErrTimeout    = "timeout"
	ErrRefused    = "connection-refused"
	ErrSSL        = "ssl-error"
	ErrConnection = "connection-error"
	ErrTooLarge   = "file-too-large"
	ErrCancelled  = "cancelled"
)

// Indexability reasons, shown in the Indexability Status column.
const (
	IndexNoindex       = "noindex"
	IndexCanonicalised = "canonicalised"
	IndexBlockedRobots = "blocked-by-robots"
	IndexNonOK         = "non-200"
	IndexRedirect      = "redirected"
)

// Options is the crawl configuration. It mirrors LibreCrawl's settings schema
// plus the gaps this tool fills.
//
// Following the house rule from indexcheck: the zero value means "default" for
// numbers and enums, which normalized() fills in and clamps — but bools are
// left alone, because a Go bool's zero value is false and there is no way to
// tell "off" from "unset". Anything that is on by default (RespectRobots,
// DiscoverSitemaps, FollowRedirects) has its default in the frontend's
// DEFAULT_OPTIONS, and the frontend mirrors every clamp below.
type Options struct {
	// --- crawler ---
	Mode             string `json:"mode"`             // spider | list
	MaxDepth         int    `json:"maxDepth"`         // 0 = default 3
	MaxURLs          int    `json:"maxURLs"`          // 0 = default 50000
	CrawlDelayMs     int    `json:"crawlDelayMs"`     // per host, 0 = no artificial delay
	FollowRedirects  bool   `json:"followRedirects"`  //
	CrawlExternal    bool   `json:"crawlExternal"`    // fetch external URLs (never follow their links)
	CrawlSubdomains  bool   `json:"crawlSubdomains"`  // treat sub.example.com as internal
	IgnoreQueryParam bool   `json:"ignoreQueryParam"` // collapse ?a=1 into the bare path
	// Fetch the resources pages reference, like Screaming Frog's Crawl
	// Images/CSS/JavaScript. Without these the Images/CSS/JS tabs can only ever
	// show resources someone linked with an <a href> — in practice, nothing.
	CrawlImages bool `json:"crawlImages"`
	CrawlCSS    bool `json:"crawlCSS"`
	CrawlJS     bool `json:"crawlJS"`

	// --- requests ---
	UserAgent      string `json:"userAgent"`      // preset id
	TimeoutSec     int    `json:"timeoutSec"`     // 0 = default 10
	Retries        int    `json:"retries"`        // 0 = default 1
	AcceptLanguage string `json:"acceptLanguage"` //
	Concurrency    int    `json:"concurrency"`    // 0 = default 5
	RespectRobots  bool   `json:"respectRobots"`  //
	// RespectCrawlDelay honours a robots.txt Crawl-delay. Off by default, which
	// is what Screaming Frog does: the directive spaces every request, so a
	// "Crawl-delay: 1" measured 1.1 URL/s where the same crawl otherwise ran at
	// 65 URL/s — and no thread count could recover it.
	RespectCrawlDelay bool              `json:"respectCrawlDelay"`
	DiscoverSitemaps  bool              `json:"discoverSitemaps"` //
	IgnoreSSL         bool              `json:"ignoreSSL"`        //
	CustomHeaders     map[string]string `json:"customHeaders"`    //
	// UseProxy routes crawl requests through the proxy saved in Connections. Off
	// by default, like Screaming Frog: the crawl goes direct until the user ticks
	// it on here. A proxy that applies to every crawl merely because it sits in
	// the vault is a surprise — and a bottleneck, since all threads then share
	// one connection.
	UseProxy bool `json:"useProxy"`

	// --- filters ---
	IncludeExtensions []string `json:"includeExtensions"`
	ExcludeExtensions []string `json:"excludeExtensions"`
	IncludePatterns   []string `json:"includePatterns"` // regex against the full URL
	ExcludePatterns   []string `json:"excludePatterns"`
	MaxFileSizeMB     int      `json:"maxFileSizeMB"` // 0 = default 50

	// --- issues ---
	IssueExclusions []string `json:"issueExclusions"` // path globs; nil = built-in defaults
	UseDefaultExcl  bool     `json:"useDefaultExcl"`  // prepend the built-in list

	// --- duplicates ---
	EnableDuplication    bool    `json:"enableDuplication"`
	DuplicationThreshold float64 `json:"duplicationThreshold"` // 0 = default 0.85

	// --- javascript rendering (phase 3) ---
	EnableJavaScript bool     `json:"enableJavaScript"`
	JSPatterns       []string `json:"jsPatterns"`   // only render URLs matching these; empty = all
	JSMaxPages       int      `json:"jsMaxPages"`   // hard cap enforced in Go, 0 = default 500
	JSWaitMs         int      `json:"jsWaitMs"`     // 0 = default 3000
	JSTimeoutSec     int      `json:"jsTimeoutSec"` // 0 = default 30
	JSViewportWidth  int      `json:"jsViewportWidth"`
	JSViewportHeight int      `json:"jsViewportHeight"`
	JSConcurrency    int      `json:"jsConcurrency"` // 0 = default 3

	// --- pagespeed ---
	// On, every internal HTML page that answered 2xx is measured through the
	// PageSpeed Insights API while the crawl runs, the way Screaming Frog does
	// it. Deliberately uncapped: the user's ruling was parity, and the cost is
	// stated in the Configuration dialog instead of being clipped in silence.
	EnablePageSpeed bool   `json:"enablePageSpeed"`
	PSIStrategy     string `json:"psiStrategy"` // mobile | desktop
}

// Defaults and clamps. Kept as consts so tests and the frontend agree on one
// set of numbers.
const (
	DefaultMaxDepth = 3
	MaxMaxDepth     = 30

	DefaultMaxURLs = 50000
	MaxMaxURLs     = 500000

	// Screaming Frog and LibreCrawl both default to 5. design.md §245 asks batch
	// tools for 25, but that rule was written for tools whose threads each hit a
	// DIFFERENT domain — here every thread hits one host, and 25 concurrent
	// requests at a small site is indistinguishable from an attack.
	DefaultConcurrency = 5
	MaxConcurrency     = 50

	DefaultTimeoutSec = 10
	MaxTimeoutSec     = 120

	DefaultRetries = 1
	MaxRetries     = 5

	DefaultMaxFileSizeMB = 50
	MaxMaxFileSizeMB     = 1000

	DefaultDuplicationThreshold = 0.85

	DefaultJSMaxPages   = 500
	MaxJSMaxPages       = 10000
	DefaultJSWaitMs     = 3000
	DefaultJSTimeoutSec = 30
	DefaultJSViewportW  = 1920
	DefaultJSViewportH  = 1080
	DefaultJSConcurrenc = 3
	MaxJSConcurrency    = 10

	// HTML bodies are parsed by a streaming tokenizer, so the cap bounds the
	// tokenizer buffer rather than a materialised document. 4MB is already far
	// past any real page; MaxFileSizeMB governs non-HTML resources only.
	maxHTMLBytes = 4 << 20

	maxCrawlDelayMs = 60000
	// A CDN serving "Crawl-delay: 10" would turn a 50k crawl into 5.8 days.
	// Honour the directive, but cap it and say so in the UI.
	maxRobotsDelayMs = 5000
)

// normalized fills defaults and clamps every numeric field. Unknown enum values
// fall back rather than erroring, which is what keeps the frontend's option
// lists safe to drift.
func (o Options) normalized() Options {
	n := o

	if n.Mode != ModeList {
		n.Mode = ModeSpider
	}
	n.MaxDepth = clamp(n.MaxDepth, DefaultMaxDepth, 0, MaxMaxDepth)
	n.MaxURLs = clamp(n.MaxURLs, DefaultMaxURLs, 1, MaxMaxURLs)
	n.CrawlDelayMs = clamp(n.CrawlDelayMs, 0, 0, maxCrawlDelayMs)
	n.Concurrency = clamp(n.Concurrency, DefaultConcurrency, 1, MaxConcurrency)
	n.TimeoutSec = clamp(n.TimeoutSec, DefaultTimeoutSec, 1, MaxTimeoutSec)
	n.Retries = clamp(n.Retries, DefaultRetries, 0, MaxRetries)
	n.MaxFileSizeMB = clamp(n.MaxFileSizeMB, DefaultMaxFileSizeMB, 1, MaxMaxFileSizeMB)

	if _, ok := uaPresets[n.UserAgent]; !ok {
		n.UserAgent = DefaultUserAgent
	}
	if n.PSIStrategy != StrategyDesktop {
		n.PSIStrategy = StrategyMobile
	}
	if n.DuplicationThreshold <= 0 || n.DuplicationThreshold > 1 {
		n.DuplicationThreshold = DefaultDuplicationThreshold
	}

	n.JSMaxPages = clamp(n.JSMaxPages, DefaultJSMaxPages, 1, MaxJSMaxPages)
	n.JSWaitMs = clamp(n.JSWaitMs, DefaultJSWaitMs, 0, 30000)
	n.JSTimeoutSec = clamp(n.JSTimeoutSec, DefaultJSTimeoutSec, 5, 120)
	n.JSViewportWidth = clamp(n.JSViewportWidth, DefaultJSViewportW, 320, 4000)
	n.JSViewportHeight = clamp(n.JSViewportHeight, DefaultJSViewportH, 320, 3000)
	n.JSConcurrency = clamp(n.JSConcurrency, DefaultJSConcurrenc, 1, MaxJSConcurrency)

	n.IncludeExtensions = cleanList(n.IncludeExtensions)
	n.ExcludeExtensions = cleanList(n.ExcludeExtensions)
	n.IncludePatterns = cleanList(n.IncludePatterns)
	n.ExcludePatterns = cleanList(n.ExcludePatterns)
	n.JSPatterns = cleanList(n.JSPatterns)
	n.IssueExclusions = cleanList(n.IssueExclusions)
	return n
}

func (o Options) timeout() time.Duration { return time.Duration(o.TimeoutSec) * time.Second }
func (o Options) delay() time.Duration   { return time.Duration(o.CrawlDelayMs) * time.Millisecond }
func (o Options) maxFileSize() int64     { return int64(o.MaxFileSizeMB) << 20 }
func (o Options) jsWait() time.Duration  { return time.Duration(o.JSWaitMs) * time.Millisecond }
func (o Options) listMode() bool         { return o.Mode == ModeList }
func (o Options) preset() uaPreset       { return presetFor(o.UserAgent) }
func (o Options) acceptLanguage() string { return strings.TrimSpace(o.AcceptLanguage) }
func (o Options) duplicationOn() bool    { return o.EnableDuplication }
func (o Options) followRedirects() bool  { return o.FollowRedirects }
func (o Options) crawlSubdomains() bool  { return o.CrawlSubdomains }
func (o Options) urlBudget() time.Duration {
	// One URL may walk a redirect chain plus a retry before the coordinator
	// gives up on it, so its deadline has to cover more than one round trip.
	return time.Duration(o.Retries+maxRedirects+1)*o.timeout() + 10*time.Second
}

func clamp(v, def, lo, hi int) int {
	if v == 0 {
		v = def
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Image is one <img> found on a page.
type Image struct {
	Src    string `json:"src"`
	Alt    string `json:"alt"`
	Width  string `json:"width"`
	Height string `json:"height"`
	Status int    `json:"status,omitempty"` // filled only when image checking is on
}

// Hreflang is one rel=alternate declaration.
type Hreflang struct {
	Lang string `json:"lang"`
	URL  string `json:"url"`
}

// Schema is one microdata itemscope.
type Schema struct {
	Type       string            `json:"type"`
	Properties map[string]string `json:"properties"`
}

// Analytics records which tracking scripts a page carries.
type Analytics struct {
	GoogleAnalytics bool   `json:"googleAnalytics"`
	Gtag            bool   `json:"gtag"`
	GA4ID           string `json:"ga4Id,omitempty"`
	GTMID           string `json:"gtmId,omitempty"`
	FacebookPixel   bool   `json:"facebookPixel"`
	Hotjar          bool   `json:"hotjar"`
	Mixpanel        bool   `json:"mixpanel"`
}

// Hop is one step of a redirect chain. LibreCrawl declares this field and never
// fills it; populating it is one of the gaps this port closes.
type Hop struct {
	URL      string `json:"url"`
	Status   int    `json:"status"`
	Location string `json:"location"`
}

// Page is everything known about one crawled URL. It is stored as a JSON blob;
// the columns SQL needs to filter or sort on are promoted alongside it (see
// schemaStmts) and derived from this struct by pageRow.
type Page struct {
	URL      string `json:"url"`
	Depth    int    `json:"depth"`
	Source   string `json:"source"`
	Internal bool   `json:"internal"`

	// transport
	Status      int    `json:"status"`
	ContentType string `json:"contentType"`
	Kind        string `json:"kind"`
	SizeBytes   int64  `json:"sizeBytes"`
	ResponseMs  int    `json:"responseMs"`
	ErrorType   string `json:"errorType,omitempty"`
	Error       string `json:"error,omitempty"`
	Redirects   []Hop  `json:"redirects"`
	RedirectTo  string `json:"redirectTo,omitempty"`
	LastMod     string `json:"lastMod,omitempty"`

	// head
	Title       string   `json:"title"`
	Titles      []string `json:"titles"` // every <title>, to flag multiples
	MetaDesc    string   `json:"metaDesc"`
	MetaDescs   []string `json:"metaDescs"`
	H1          []string `json:"h1"`
	H2          []string `json:"h2"`
	H3          []string `json:"h3"`
	WordCount   int      `json:"wordCount"`
	TextRatio   float64  `json:"textRatio"`
	Lang        string   `json:"lang"`
	Charset     string   `json:"charset"`
	Viewport    string   `json:"viewport"`
	Canonical   string   `json:"canonical"`
	Canonicals  []string `json:"canonicals"`
	MetaRobots  string   `json:"metaRobots"`
	XRobotsTag  string   `json:"xRobotsTag"`
	MetaRefresh string   `json:"metaRefresh,omitempty"`
	RelNext     string   `json:"relNext,omitempty"`
	RelPrev     string   `json:"relPrev,omitempty"`
	Author      string   `json:"author,omitempty"`
	Keywords    string   `json:"keywords,omitempty"`
	Generator   string   `json:"generator,omitempty"`
	ThemeColor  string   `json:"themeColor,omitempty"`

	MetaTags    map[string]string `json:"metaTags"`
	OGTags      map[string]string `json:"ogTags"`
	TwitterTags map[string]string `json:"twitterTags"`
	JSONLD      []string          `json:"jsonLd"` // raw script bodies, parsed on demand
	SchemaOrg   []Schema          `json:"schemaOrg"`
	Hreflang    []Hreflang        `json:"hreflang"`
	Analytics   Analytics         `json:"analytics"`
	Images      []Image           `json:"images"`

	// graph, filled during the crawl and corrected in finalize
	OutlinksInternal int `json:"outlinksInternal"`
	OutlinksExternal int `json:"outlinksExternal"`
	Inlinks          int `json:"inlinks"`
	InlinksUnique    int `json:"inlinksUnique"`

	// verdicts
	Indexable    bool     `json:"indexable"`
	Indexability string   `json:"indexability,omitempty"`
	RobotsState  string   `json:"robotsState"`
	Rendered     bool     `json:"rendered"`
	Issues       []string `json:"issues"` // issue codes, for the detail pane
	CrawledAt    string   `json:"crawledAt"`
}

// Row is the grid's projection of a page: exactly the columns the current tab
// asked for, positionally aligned to RowQuery.Cols.
//
// A map would be the obvious shape, but Wails renders Go maps as
// `{[_ in string]?: T}` — every value optional and needing narrowing — and at
// 25 columns × 50k rows the keys roughly double the payload.
type Row struct {
	ID     string   `json:"id"` // the URL; stable across sorts and live inserts
	Cells  []string `json:"cells"`
	Status int      `json:"status"`
	Flags  uint32   `json:"flags"`
}

// Row flag bits. These stay outside Cells so badge colour and indexability
// decisions remain typed, and so all formatting and i18n stay in the frontend.
const (
	RowInternal  uint32 = 1 << 0
	RowIndexable uint32 = 1 << 1
	RowHasIssues uint32 = 1 << 2
	RowRendered  uint32 = 1 << 3
	RowOrphan    uint32 = 1 << 4
)

// RowQuery is one grid window request.
type RowQuery struct {
	RunID  string   `json:"runId"`
	Tab    string   `json:"tab"`
	Filter string   `json:"filter"`
	Search string   `json:"search"`
	Cols   []string `json:"cols"`
	Sort   string   `json:"sort"`
	Desc   bool     `json:"desc"`
	Offset int      `json:"offset"`
	Limit  int      `json:"limit"`
	// IDs pins the window to an explicit URL set — how "export selected"
	// works: the grid's multi-select rides the same query as everything else.
	IDs []string `json:"ids"`
	// WantTotal asks for COUNT(*). The grid only needs it when the filter
	// changed, and skipping it turns every scroll into an index-only read.
	WantTotal bool `json:"wantTotal"`
}

// RowPage is one window of grid rows.
type RowPage struct {
	Rows     []Row `json:"rows"`
	Offset   int   `json:"offset"`
	Total    int   `json:"total"`    // -1 when not requested
	Revision int64 `json:"revision"` // matches ProgressEvent.Revision
}

// StartedCrawl is what Start/Resume hand back.
//
// JobID and RunID are the same string for a fresh crawl and differ for a resume,
// exactly as in indexcheck: the job id addresses the goroutine (progress,
// cancel) while the run id addresses the rows in SQLite.
type StartedCrawl struct {
	JobID string `json:"jobId"`
	RunID string `json:"runId"`
}

// RunSummary is one row of the history list.
type RunSummary struct {
	ID         string  `json:"id"`
	SeedURL    string  `json:"seedUrl"`
	Host       string  `json:"host"`
	Mode       string  `json:"mode"`
	State      string  `json:"state"`
	Phase      string  `json:"phase"`
	StopReason string  `json:"stopReason,omitempty"`
	Resumable  bool    `json:"resumable"`
	Found      int     `json:"found"`
	Crawled    int     `json:"crawled"`
	Issues     int     `json:"issues"`
	StartedAt  string  `json:"startedAt"`
	FinishedAt string  `json:"finishedAt,omitempty"`
	DurationMs int64   `json:"durationMs"`
	Options    Options `json:"options"`
	// SizeBytes is this run's share of the workspace database. Storing the full
	// link graph is what makes the Links tab possible and it is not cheap, so
	// the cost is shown rather than hidden.
	SizeBytes int64 `json:"sizeBytes"`
}

// RunStatus is the poll fallback for a frontend that missed an event.
type RunStatus struct {
	RunID    string `json:"runId"`
	JobID    string `json:"jobId,omitempty"`
	State    string `json:"state"`
	Phase    string `json:"phase"`
	Found    int    `json:"found"`
	Crawled  int    `json:"crawled"`
	Revision int64  `json:"revision"`
}

// OverviewNode is one row of the right-hand tree.
type OverviewNode struct {
	Tab    string `json:"tab"`
	Filter string `json:"filter,omitempty"`
	Count  int    `json:"count"`
}

// --- events ---

const (
	EventProgress    = "sitecrawl:progress"
	EventRunState    = "sitecrawl:run-state"
	EventPSI         = "sitecrawl:psi-result"
	EventPSIProgress = "sitecrawl:psi-progress"
	EventRender      = "sitecrawl:render-state"
)

// PSIProgressEvent is the PageSpeed pass's own heartbeat.
//
// Separate from ProgressEvent because the two do not share a lifetime: PageSpeed
// keeps measuring after the crawl has finished, and it also runs with no crawl
// at all when the user fills in a stored run. One event serving both is what
// lets the status strip and the PageSpeed tab read the same numbers.
//
// Total grows while the crawl is still finding pages — that is honest, not a
// bug, and Screaming Frog's API bar behaves the same way.
type PSIProgressEvent struct {
	RunID   string `json:"runId"`
	Done    int    `json:"done"`
	Total   int    `json:"total"`
	Running bool   `json:"running"`
	Stopped bool   `json:"stopped"`
}

// IssueTally and StatusTally are the headline counters the status strip shows.
type IssueTally struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Notice   int `json:"notice"`
}

type StatusTally struct {
	OK        int `json:"ok"`
	Redirect  int `json:"redirect"`
	ClientErr int `json:"clientErr"`
	ServerErr int `json:"serverErr"`
	Failed    int `json:"failed"`
	Blocked   int `json:"blocked"`
}

// ProgressEvent is the aggregate heartbeat, emitted on a ticker rather than per
// URL.
//
// It carries no rows on purpose. At 50k URLs with 20 workers, one event per
// crawled page is 60+ events/sec across the Wails bridge, and the frontend
// would have to hold the whole set in memory to apply them. Instead the grid
// reads windows through Rows(), and Revision — bumped on every committed flush
// — is the only signal it needs to know a refetch is worthwhile. The Overview
// tree's counts follow the same rule: the frontend refreshes Facets() when
// Revision moves, rather than this event carrying the whole map.
type ProgressEvent struct {
	RunID     string      `json:"runId"`
	Phase     string      `json:"phase"`
	Found     int         `json:"found"`
	Crawled   int         `json:"crawled"`
	Queued    int         `json:"queued"`
	InFlight  int         `json:"inFlight"`
	Skipped   int         `json:"skipped"`
	Issues    IssueTally  `json:"issues"`
	Status    StatusTally `json:"status"`
	Rate      float64     `json:"rate"`
	ElapsedMs int64       `json:"elapsedMs"`
	ETASec    int         `json:"etaSec"` // -1 when unknown
	Revision  int64       `json:"revision"`
	Current   string      `json:"current,omitempty"`
	// CrawlDelayAskedMs is what the seed's robots.txt asked for, and
	// CrawlDelayAppliedMs is what the crawl actually paces by. They differ when
	// RespectCrawlDelay is off (applied 0) or when the cap bit, and the status
	// strip says so — obeying or ignoring the directive in silence both leave the
	// user staring at a speed with no explanation.
	CrawlDelayAskedMs   int `json:"crawlDelayAskedMs"`
	CrawlDelayAppliedMs int `json:"crawlDelayAppliedMs"`
}

// RunStateEvent reports state and phase transitions.
//
// It exists for the same reason indexcheck's StoppedEvent does: jobs.Progress
// only knows running|completed|cancelled|failed, and widening that shared enum
// would ripple into every other tool. "Paused, frontier intact, resumable" is
// none of the four.
//
// The frontend must derive "is this crawl running?" from State here, never from
// job:progress — during a pause the job goroutine is alive and jobs.Progress
// still reads "running", so a UI keyed on it shows a spinner over a stopped
// crawl.
// RenderStateEvent reports the JavaScript renderer's condition. "no-browser"
// means the option was on but no Chrome/Edge exists to honour it — the UI must
// say so rather than let the crawl silently run unrendered.
type RenderStateEvent struct {
	RunID string `json:"runId"`
	State string `json:"state"`
}

type RunStateEvent struct {
	RunID      string `json:"runId"`
	JobID      string `json:"jobId"`
	State      string `json:"state"`
	Phase      string `json:"phase"`
	Reason     string `json:"reason,omitempty"`
	Resumable  bool   `json:"resumable"`
	Found      int    `json:"found"`
	Crawled    int    `json:"crawled"`
	DurationMs int64  `json:"durationMs"`
	Error      string `json:"error,omitempty"`
	// RedirectTarget carries where the seed pointed when Reason is
	// seed-redirect, so the UI's "crawl that instead" button knows the URL.
	RedirectTarget string `json:"redirectTarget,omitempty"`
}
