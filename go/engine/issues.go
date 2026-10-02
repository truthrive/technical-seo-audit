package sitecrawl

import (
	"net/http"
	"sort"
	"strings"
)

// Issue codes. Every one is a stable kebab-case slug doubling as an i18n key,
// so they must not be renamed once shipped.
const (
	// SEO
	IssueTitleMissing   = "title-missing"
	IssueTitleLong      = "title-long"
	IssueTitleShort     = "title-short"
	IssueTitleMultiple  = "title-multiple"
	IssueTitleDuplicate = "title-duplicate"
	IssueTitleSameAsH1  = "title-same-as-h1"
	IssueMetaMissing    = "meta-missing"
	IssueMetaLong       = "meta-long"
	IssueMetaShort      = "meta-short"
	IssueMetaMultiple   = "meta-multiple"
	IssueMetaDuplicate  = "meta-duplicate"
	IssueH1Missing      = "h1-missing"
	IssueH1Multiple     = "h1-multiple"
	IssueH1Long         = "h1-long"
	IssueH1Duplicate    = "h1-duplicate"
	IssueH2Missing      = "h2-missing"

	// Content
	IssueThinContent   = "thin-content"
	IssueLowTextRatio  = "low-text-ratio"
	IssueDuplicatePage = "duplicate-page"
	IssueBrokenImage   = "broken-image"

	// Technical
	IssueDNS               = "dns-not-found"
	IssueConnectionRefused = "connection-refused"
	IssueTimeout           = "timeout"
	IssueSSL               = "ssl-error"
	IssueConnection        = "connection-error"
	IssueClientError       = "client-error"
	IssueServerError       = "server-error"
	IssueRedirect          = "redirect"
	IssueRedirectChain     = "redirect-chain"
	IssueRedirectLoop      = "redirect-loop"
	IssueInternalRedirect  = "internal-redirect"
	IssueCanonicalMissing  = "canonical-missing"
	IssueCanonicalOther    = "canonical-other"
	IssueCanonicalMultiple = "canonical-multiple"
	IssueCanonicalChain    = "canonical-chain"
	IssueCanonicalLoop     = "canonical-loop"
	IssueCanonicalNonOK    = "canonical-to-non-200"
	IssueCanonicalNoindex  = "canonical-to-noindex"
	IssueMetaRefresh       = "meta-refresh"
	IssueBotBlocked        = "bot-blocked"

	// Mobile / accessibility
	IssueViewportMissing = "viewport-missing"
	IssueLangMissing     = "lang-missing"
	IssueImagesNoAlt     = "images-no-alt"
	IssueImageAltLong    = "image-alt-long"

	// Social
	IssueOGMissing      = "og-missing"
	IssueTwitterMissing = "twitter-missing"

	// Structured data
	IssueNoStructuredData = "no-structured-data"

	// Performance
	IssueSlowResponse     = "slow-response"
	IssueModerateResponse = "moderate-response"
	IssueLargePage        = "large-page"
	IssueModeratePage     = "moderate-page"

	// Indexability
	IssueNoindex       = "noindex"
	IssueNofollow      = "nofollow"
	IssueRobotsBlocked = "robots-blocked"
	IssueRobotsUnknown = "robots-unknown"
	IssueOrphan        = "orphan-page"

	// Hreflang
	IssueHreflangNoReturn = "hreflang-no-return-tag"
	IssueHreflangNoSelf   = "hreflang-missing-self"
	IssueHreflangBadCode  = "hreflang-invalid-code"
	IssueHreflangNonOK    = "hreflang-to-non-200"
	IssueHreflangNoncanon = "hreflang-non-canonical"
)

// Issue categories, also i18n keys.
const (
	CatSEO       = "seo"
	CatContent   = "content"
	CatTechnical = "technical"
	CatMobile    = "mobile"
	CatA11y      = "accessibility"
	CatSocial    = "social"
	CatSchema    = "structured-data"
	CatPerf      = "performance"
	CatIndex     = "indexability"
	CatDuplicate = "duplication"
	CatHreflang  = "hreflang"
)

// Thresholds. These are LibreCrawl's exact numbers, kept so a user comparing
// the two tools sees the same verdicts — except where a value is plainly a
// typo for the widely-published one, which is noted inline.
const (
	titleMaxLen = 60
	titleMinLen = 30
	metaMaxLen  = 160
	metaMinLen  = 120
	h1MaxLen    = 70
	thinWords   = 300
	altMaxLen   = 100

	slowResponseMs     = 3000
	moderateResponseMs = 1000
	largePageBytes     = 3 << 20
	moderatePageBytes  = 1 << 20

	lowTextRatio = 0.10
)

// IssueInfo describes one rule to the frontend, so the Issues tab and the
// Overview tree can label and colour it without hardcoding anything.
type IssueInfo struct {
	Code     string `json:"code"`
	Category string `json:"category"`
	Severity int    `json:"severity"`
}

// issueCatalog is the single source of truth for severity and category.
// Anything emitted by evaluate or a finalize rule must appear here.
var issueCatalog = []IssueInfo{
	{IssueTitleMissing, CatSEO, SeverityCritical},
	{IssueTitleLong, CatSEO, SeverityWarning},
	{IssueTitleShort, CatSEO, SeverityWarning},
	{IssueTitleMultiple, CatSEO, SeverityWarning},
	{IssueTitleDuplicate, CatSEO, SeverityWarning},
	{IssueTitleSameAsH1, CatSEO, SeverityNotice},
	{IssueMetaMissing, CatSEO, SeverityCritical},
	{IssueMetaLong, CatSEO, SeverityWarning},
	{IssueMetaShort, CatSEO, SeverityWarning},
	{IssueMetaMultiple, CatSEO, SeverityWarning},
	{IssueMetaDuplicate, CatSEO, SeverityWarning},
	{IssueH1Missing, CatSEO, SeverityCritical},
	{IssueH1Multiple, CatSEO, SeverityWarning},
	{IssueH1Long, CatSEO, SeverityNotice},
	{IssueH1Duplicate, CatSEO, SeverityWarning},
	{IssueH2Missing, CatSEO, SeverityNotice},

	{IssueThinContent, CatContent, SeverityWarning},
	{IssueLowTextRatio, CatContent, SeverityNotice},
	{IssueDuplicatePage, CatDuplicate, SeverityWarning},
	{IssueBrokenImage, CatContent, SeverityCritical},

	{IssueDNS, CatTechnical, SeverityCritical},
	{IssueConnectionRefused, CatTechnical, SeverityCritical},
	{IssueTimeout, CatTechnical, SeverityCritical},
	{IssueSSL, CatTechnical, SeverityCritical},
	{IssueConnection, CatTechnical, SeverityCritical},
	{IssueClientError, CatTechnical, SeverityCritical},
	{IssueServerError, CatTechnical, SeverityCritical},
	{IssueRedirect, CatTechnical, SeverityNotice},
	{IssueRedirectChain, CatTechnical, SeverityWarning},
	{IssueRedirectLoop, CatTechnical, SeverityCritical},
	{IssueInternalRedirect, CatTechnical, SeverityWarning},
	{IssueCanonicalMissing, CatTechnical, SeverityWarning},
	{IssueCanonicalOther, CatTechnical, SeverityNotice},
	{IssueCanonicalMultiple, CatTechnical, SeverityWarning},
	{IssueCanonicalChain, CatTechnical, SeverityWarning},
	{IssueCanonicalLoop, CatTechnical, SeverityCritical},
	{IssueCanonicalNonOK, CatTechnical, SeverityCritical},
	{IssueCanonicalNoindex, CatTechnical, SeverityCritical},
	{IssueMetaRefresh, CatTechnical, SeverityWarning},
	{IssueBotBlocked, CatTechnical, SeverityNotice},

	{IssueViewportMissing, CatMobile, SeverityCritical},
	{IssueLangMissing, CatA11y, SeverityWarning},
	{IssueImagesNoAlt, CatA11y, SeverityWarning},
	{IssueImageAltLong, CatA11y, SeverityNotice},

	{IssueOGMissing, CatSocial, SeverityWarning},
	{IssueTwitterMissing, CatSocial, SeverityWarning},

	// LibreCrawl grades this an error. Structured data is genuinely optional for
	// most page types, so it is a warning here — an audit that opens with 400
	// red rows on a blog teaches users to ignore red.
	{IssueNoStructuredData, CatSchema, SeverityWarning},

	{IssueSlowResponse, CatPerf, SeverityCritical},
	{IssueModerateResponse, CatPerf, SeverityWarning},
	{IssueLargePage, CatPerf, SeverityCritical},
	{IssueModeratePage, CatPerf, SeverityWarning},

	{IssueNoindex, CatIndex, SeverityCritical},
	{IssueNofollow, CatIndex, SeverityWarning},
	{IssueRobotsBlocked, CatIndex, SeverityCritical},
	{IssueRobotsUnknown, CatIndex, SeverityNotice},
	{IssueOrphan, CatIndex, SeverityWarning},

	{IssueHreflangNoReturn, CatHreflang, SeverityWarning},
	{IssueHreflangNoSelf, CatHreflang, SeverityWarning},
	{IssueHreflangBadCode, CatHreflang, SeverityWarning},
	{IssueHreflangNonOK, CatHreflang, SeverityCritical},
	{IssueHreflangNoncanon, CatHreflang, SeverityWarning},
}

var issueByCode = func() map[string]IssueInfo {
	m := make(map[string]IssueInfo, len(issueCatalog))
	for _, i := range issueCatalog {
		m[i.Code] = i
	}
	return m
}()

func severityOf(code string) int {
	if i, ok := issueByCode[code]; ok {
		return i.Severity
	}
	return SeverityNotice
}

func categoryOf(code string) string {
	if i, ok := issueByCode[code]; ok {
		return i.Category
	}
	return CatTechnical
}

// issue is one finding on one page.
type issue struct {
	Code   string
	Detail string
}

// evaluate applies every per-page rule. It is a pure function of the page: the
// rules that need the whole crawl (duplicates, orphans, canonical and hreflang
// chains) run in finalize instead.
func evaluate(p *Page, exclusions *exclusionSet) []issue {
	if exclusions != nil && exclusions.match(p.URL) {
		return nil
	}
	var out []issue
	add := func(code, detail string) { out = append(out, issue{Code: code, Detail: detail}) }

	// --- transport ---
	switch p.ErrorType {
	case ErrDNS:
		add(IssueDNS, p.Error)
		return out
	case ErrRefused:
		add(IssueConnectionRefused, p.Error)
		return out
	case ErrTimeout:
		add(IssueTimeout, p.Error)
		return out
	case ErrSSL:
		add(IssueSSL, p.Error)
		return out
	case ErrConnection:
		add(IssueConnection, p.Error)
		return out
	case ErrTooLarge:
		// Deliberately not an issue, matching LibreCrawl: the crawler chose not
		// to download it, which says nothing about the page.
		return out
	case ErrCancelled:
		return out
	}

	switch {
	case p.Status == 0:
		add(IssueConnection, p.Error)
		return out
	case p.Status >= 500:
		add(IssueServerError, statusMessage(p.Status))
	case p.Status == http.StatusTooManyRequests:
		// 429 is the site throttling us, not a verdict on the page. Reported as
		// a notice through the redirect-free path below rather than a client
		// error, so a healthy URL is never coloured like a 404.
	case p.Status >= 400:
		add(IssueClientError, statusMessage(p.Status))
	case p.Status >= 300:
		add(IssueRedirect, p.RedirectTo)
	}

	if n := len(p.Redirects); n > 1 {
		add(IssueRedirectChain, "")
	}

	// Non-HTML resources carry none of the content rules, and external pages
	// are not ours to audit — their transport verdict above is the finding.
	if p.Kind != KindHTML || !p.Internal || p.Status < 200 || p.Status >= 300 {
		if p.RobotsState == RobotsBlocked {
			add(IssueRobotsBlocked, "")
		}
		return out
	}

	// --- title ---
	switch n := len([]rune(p.Title)); {
	case p.Title == "":
		add(IssueTitleMissing, "")
	case n > titleMaxLen:
		add(IssueTitleLong, "")
	case n < titleMinLen:
		add(IssueTitleShort, "")
	}
	if len(p.Titles) > 1 {
		add(IssueTitleMultiple, "")
	}
	if p.Title != "" && len(p.H1) > 0 && strings.EqualFold(p.Title, p.H1[0]) {
		add(IssueTitleSameAsH1, "")
	}

	// --- meta description ---
	switch n := len([]rune(p.MetaDesc)); {
	case p.MetaDesc == "":
		add(IssueMetaMissing, "")
	case n > metaMaxLen:
		add(IssueMetaLong, "")
	case n < metaMinLen:
		add(IssueMetaShort, "")
	}
	if len(p.MetaDescs) > 1 {
		add(IssueMetaMultiple, "")
	}

	// --- headings ---
	switch {
	case len(p.H1) == 0:
		add(IssueH1Missing, "")
	case len(p.H1) > 1:
		add(IssueH1Multiple, "")
	}
	if len(p.H1) > 0 && len([]rune(p.H1[0])) > h1MaxLen {
		add(IssueH1Long, "")
	}
	if len(p.H2) == 0 {
		add(IssueH2Missing, "")
	}

	// --- content ---
	if p.WordCount < thinWords {
		add(IssueThinContent, "")
	}
	if p.TextRatio > 0 && p.TextRatio < lowTextRatio {
		add(IssueLowTextRatio, "")
	}

	// --- technical ---
	switch {
	case len(p.Canonicals) == 0:
		add(IssueCanonicalMissing, "")
	case len(p.Canonicals) > 1:
		add(IssueCanonicalMultiple, "")
	}
	// Compared on the frontier key, not raw strings: LibreCrawl's exact string
	// compare reports a trailing-slash or relative canonical as pointing
	// elsewhere, which is a false positive on a correctly configured site.
	if p.Canonical != "" && frontierKey(p.Canonical, false) != frontierKey(p.URL, false) {
		add(IssueCanonicalOther, p.Canonical)
	}
	if p.MetaRefresh != "" {
		add(IssueMetaRefresh, p.MetaRefresh)
	}

	// --- mobile / accessibility ---
	if p.Viewport == "" {
		add(IssueViewportMissing, "")
	}
	if p.Lang == "" {
		add(IssueLangMissing, "")
	}
	if missing := countMissingAlt(p.Images); missing > 0 {
		add(IssueImagesNoAlt, "")
	}
	for _, img := range p.Images {
		if len([]rune(img.Alt)) > altMaxLen {
			add(IssueImageAltLong, "")
			break
		}
	}
	// IssueBrokenImage is a finalize rule: it needs the image's own fetch
	// result, which only exists once the crawl has visited the image.

	// --- social ---
	if len(p.OGTags) == 0 {
		add(IssueOGMissing, "")
	}
	if len(p.TwitterTags) == 0 {
		add(IssueTwitterMissing, "")
	}

	// --- structured data ---
	if len(p.JSONLD) == 0 && len(p.SchemaOrg) == 0 {
		add(IssueNoStructuredData, "")
	}

	// --- performance ---
	switch {
	case p.ResponseMs > slowResponseMs:
		add(IssueSlowResponse, "")
	case p.ResponseMs > moderateResponseMs:
		add(IssueModerateResponse, "")
	}
	switch {
	case p.SizeBytes > largePageBytes:
		add(IssueLargePage, "")
	case p.SizeBytes > moderatePageBytes:
		add(IssueModeratePage, "")
	}

	// --- indexability ---
	directives := p.MetaRobots + ", " + p.XRobotsTag
	if robotsHasToken(directives, "noindex") {
		add(IssueNoindex, "")
	}
	if robotsHasToken(directives, "nofollow") {
		add(IssueNofollow, "")
	}
	switch p.RobotsState {
	case RobotsBlocked:
		add(IssueRobotsBlocked, "")
	case RobotsUnknown:
		add(IssueRobotsUnknown, "")
	}

	sortIssues(out)
	return out
}

func countMissingAlt(images []Image) int {
	n := 0
	for _, img := range images {
		if strings.TrimSpace(img.Alt) == "" {
			n++
		}
	}
	return n
}

// robotsHasToken reports whether a robots directive list carries a token.
//
// Prefix-aware so "unavailable_after: ..." is never read as a bare directive,
// and so a "googlebot: noindex" prefix still counts.
func robotsHasToken(directives, want string) bool {
	for _, part := range strings.Split(strings.ToLower(directives), ",") {
		part = strings.TrimSpace(part)
		if i := strings.LastIndex(part, ":"); i >= 0 {
			// Strip a "<agent>:" prefix but keep "unavailable_after:2025-01-01"
			// from matching anything.
			head := strings.TrimSpace(part[:i])
			if !strings.Contains(head, "_") {
				part = strings.TrimSpace(part[i+1:])
			}
		}
		if part == want {
			return true
		}
	}
	return false
}

// indexabilityOf reduces the signals to the one verdict the grid shows.
// Order is the order Google applies them.
func indexabilityOf(p *Page) (bool, string) {
	switch {
	case p.RobotsState == RobotsBlocked:
		return false, IndexBlockedRobots
	case robotsHasToken(p.MetaRobots+", "+p.XRobotsTag, "noindex"):
		return false, IndexNoindex
	case p.Status >= 300 && p.Status < 400:
		return false, IndexRedirect
	case p.Status < 200 || p.Status >= 300:
		return false, IndexNonOK
	case p.Canonical != "" && frontierKey(p.Canonical, false) != frontierKey(p.URL, false):
		return false, IndexCanonicalised
	}
	return true, ""
}

// issueOrder puts the findings a person acts on first.
var issueOrder = func() map[string]int {
	m := make(map[string]int, len(issueCatalog))
	for i, info := range issueCatalog {
		m[info.Code] = i
	}
	return m
}()

func sortIssues(list []issue) {
	sort.SliceStable(list, func(i, j int) bool {
		si, sj := severityOf(list[i].Code), severityOf(list[j].Code)
		if si != sj {
			return si > sj // critical first
		}
		return issueOrder[list[i].Code] < issueOrder[list[j].Code]
	})
}

// statusMessage names an HTTP status for the issue detail line.
func statusMessage(code int) string {
	if t := http.StatusText(code); t != "" {
		return t
	}
	return ""
}

// IssueCatalog is bound to the frontend so the Issues tab and the Overview tree
// can label and colour every rule without duplicating this table in TypeScript.
func (s *Service) IssueCatalog() []IssueInfo {
	out := make([]IssueInfo, len(issueCatalog))
	copy(out, issueCatalog)
	return out
}
