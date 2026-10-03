package sitecrawl

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"onescout/desktop/internal/core/httpx"
)

const (
	// maxRedirects caps how many hops are followed. Deliberately not
	// user-adjustable: past this a chain is broken, not long.
	maxRedirects = 5
	// drainCap is how much of a hop's body is read before closing, so the
	// connection returns to the pool for the next hop.
	drainCap = 4 << 10
)

// fetchRetryDelay is the pause before the single retry of a dropped connection.
// A var so tests can shrink it.
var fetchRetryDelay = 700 * time.Millisecond

var (
	errNoLocation  = errors.New("redirect without Location")
	errBadLocation = errors.New("invalid Location")
)

// fetched is one URL's transport-level outcome.
type fetched struct {
	FinalURL string
	// Status is what this URL itself answered — a 301 stays a 301.
	Status int
	// FinalStatus is what the end of its redirect chain answered, used to
	// decide whether the body is worth parsing.
	FinalStatus int
	Hops        []Hop
	RedirectTo  string
	ContentType string
	Kind        string
	Size        int64
	ElapsedMs   int
	LastMod     string
	XRobotsTag  string
	ErrorType   string
	Error       string
	// Doc is nil unless the terminal response was HTML that we read.
	Doc *document
	// BotBlocked records that the bot identity was refused and a browser
	// identity succeeded instead.
	BotBlocked bool
}

// fetcher performs one URL's HTTP work. Built fresh per run so a just-saved
// proxy or user agent takes effect (the vault is write-only from the frontend).
type fetcher struct {
	client *http.Client
	opts   Options
	ua     uaPreset
	// seedHost scopes CustomHeaders. They routinely carry a Cookie or
	// Authorization for a staging site, and this crawler walks redirects by hand
	// and fetches external URLs by default — so without a scope the credential
	// would be handed to every host a crawled page happens to point at.
	seedHost string
}

// newFetcher builds the run's HTTP client.
//
// allowPrivate reflects where the crawl was aimed: a seed on localhost or a LAN
// host may keep reaching that network, a seed on the public internet may not.
// See httpx.NewFiltered.
func newFetcher(opts Options, proxy httpx.ProxyFunc, seedHost string, allowPrivate bool) *fetcher {
	c := httpx.NewFiltered(opts.timeout(), proxy, !allowPrivate)
	// Redirects are walked by hand so every hop is observable — Go's automatic
	// following hides the chain, and the chain is a reported feature here.
	c.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if tr, ok := c.Transport.(*http.Transport); ok {
		// The shared default keeps 4 idle connections per host, which is sized for
		// tools that spread their threads over many domains. A site crawler aims
		// every thread at one host, so anything past the fourth returns its socket
		// to a full pool, gets it closed, and pays a fresh TCP and TLS handshake on
		// the next page. Measured at 50 threads: 496 connections for 1117 requests,
		// and a 3x throughput loss on a zero-latency local server — against a real
		// HTTPS host it is two extra round trips per page.
		tr.MaxIdleConnsPerHost = opts.Concurrency
		if tr.MaxIdleConns < opts.Concurrency {
			tr.MaxIdleConns = opts.Concurrency
		}
		if opts.IgnoreSSL {
			tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402 — opt-in per run
		}
	}
	return &fetcher{client: c, opts: opts, ua: opts.preset(), seedHost: seedHost}
}

// customHeadersFor returns the user's headers when rawURL belongs to the site
// the run was aimed at, and nil otherwise.
//
// net/http strips Authorization and Cookie on cross-host redirects for exactly
// this reason; walking the chain by hand opted out of that protection, so it is
// reinstated here — and it covers plain external fetches too, which the built-in
// rule never saw.
func (f *fetcher) customHeadersFor(rawURL string) map[string]string {
	if len(f.opts.CustomHeaders) == 0 {
		return nil
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return nil
	}
	if !sameSite(f.seedHost, u.Hostname(), f.opts.crawlSubdomains()) {
		return nil
	}
	return f.opts.CustomHeaders
}

// robotsClient is a second client over the same transport that DOES follow
// redirects: an apex robots.txt commonly 301s to www.
func (f *fetcher) robotsClient() *http.Client {
	return &http.Client{
		Transport: f.client.Transport,
		Timeout:   f.client.Timeout,
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}

// fetch walks the redirect chain and, for an HTML terminal response, streams the
// body straight into the extractor.
func (f *fetcher) fetch(ctx context.Context, rawURL string, wantBody bool) fetched {
	start := time.Now()
	out := f.chain(ctx, rawURL, f.ua, wantBody)

	// A bot identity refused outright says nothing about the page, so retry as a
	// browser: Cloudflare and friends routinely 403 an unverified crawler UA from
	// a residential IP, and without this fallback whole domains report a false
	// client error.
	if out.Status == http.StatusForbidden && f.ua.Bot && ctx.Err() == nil {
		retry := f.chain(ctx, rawURL, presetFor(fallbackUserAgent), wantBody)
		if retry.Status >= 200 && retry.Status < 400 {
			retry.BotBlocked = true
			out = retry
		}
	}

	out.ElapsedMs = int(time.Since(start).Milliseconds())
	return out
}

func (f *fetcher) chain(ctx context.Context, startURL string, ua uaPreset, wantBody bool) fetched {
	out := fetched{FinalURL: startURL, Kind: KindOther}
	current := startURL
	seen := map[string]bool{redirectKey(startURL): true}

	for hop := 0; ; hop++ {
		if hop > maxRedirects {
			out.ErrorType = ErrConnection
			out.Error = "redirect chain too long"
			return out
		}

		res, err := f.doRetrying(ctx, current, ua)
		if err != nil {
			out.ErrorType = classifyError(ctx, err)
			out.Error = err.Error()
			out.FinalURL = current
			return out
		}

		// A URL's own status is what IT answered, not what the end of its chain
		// answered. Overwriting it each hop would report a 301 as a 200 and hide
		// every redirect from the Response Codes tab.
		if hop == 0 {
			out.Status = res.StatusCode
		}
		out.FinalStatus = res.StatusCode
		out.FinalURL = current

		if res.StatusCode >= 300 && res.StatusCode < 400 {
			location := res.Header.Get("Location")
			io.Copy(io.Discard, io.LimitReader(res.Body, drainCap))
			res.Body.Close()

			next, err := resolveLocation(current, location)
			if err != nil {
				out.ErrorType = ErrConnection
				out.Error = err.Error()
				return out
			}
			out.Hops = append(out.Hops, Hop{URL: current, Status: res.StatusCode, Location: next})
			if out.RedirectTo == "" {
				out.RedirectTo = next
			}
			if !f.opts.followRedirects() {
				return out
			}
			if seen[redirectKey(next)] {
				out.FinalURL = next
				out.ErrorType = ErrConnection
				out.Error = "redirect loop"
				return out
			}
			seen[redirectKey(next)] = true
			current = next
			continue
		}

		f.readTerminal(res, &out, wantBody)
		res.Body.Close()
		return out
	}
}

// readTerminal records the terminal response and, when it is HTML we want,
// streams it into the extractor.
//
// LibreCrawl runs a HEAD before every GET to enforce max_file_size, doubling the
// request count against the user's own site. Content-Length on the GET answers
// the same question for free, and a counting reader covers the servers that
// omit it.
func (f *fetcher) readTerminal(res *http.Response, out *fetched, wantBody bool) {
	out.ContentType = strings.TrimSpace(strings.SplitN(res.Header.Get("Content-Type"), ";", 2)[0])
	out.Kind = kindOf(out.ContentType, out.FinalURL)
	out.LastMod = res.Header.Get("Last-Modified")
	if values := res.Header.Values("X-Robots-Tag"); len(values) > 0 {
		// X-Robots-Tag is legitimately repeatable, so keep every value.
		out.XRobotsTag = strings.Join(values, ", ")
	}

	cap := f.opts.maxFileSize()
	if declared := res.ContentLength; declared > 0 && declared > cap {
		out.Size = declared
		out.ErrorType = ErrTooLarge
		io.Copy(io.Discard, io.LimitReader(res.Body, drainCap))
		return
	}

	html := isHTML(out.ContentType)
	if !html || !wantBody || res.StatusCode < 200 || res.StatusCode >= 300 {
		// Nothing to parse: drain through a counter so the size is real without
		// ever buffering the resource.
		n, _ := io.Copy(io.Discard, io.LimitReader(res.Body, cap+1))
		out.Size = n
		if n > cap {
			out.ErrorType = ErrTooLarge
		}
		return
	}

	// Go rewrites res.Request.URL to whatever was actually fetched, which is what
	// a relative href or canonical resolves against.
	base := res.Request.URL
	if base == nil {
		base, _ = url.Parse(out.FinalURL)
	}
	if base == nil {
		return
	}

	// HTML is parsed by a streaming tokenizer, so the cap bounds the tokenizer's
	// buffer rather than a materialised document.
	limit := cap
	if limit > maxHTMLBytes {
		limit = maxHTMLBytes
	}
	counter := &countingReader{r: io.LimitReader(res.Body, limit)}
	doc := extractDocument(counter, base)
	out.Doc = doc
	out.Size = counter.n
	// Whatever the tokenizer left unread still counts toward the size.
	if rest, _ := io.Copy(io.Discard, io.LimitReader(res.Body, drainCap)); rest > 0 {
		out.Size += rest
	}
}

// doRetrying performs one hop, retrying a dropped connection once.
//
// A busy site under load will close a connection without answering — Go reports
// that as a bare EOF. Only transport failures are retried: a real HTTP answer
// (even a 429) is the site talking, and asking again immediately would only add
// load.
func (f *fetcher) doRetrying(ctx context.Context, rawURL string, ua uaPreset) (*http.Response, error) {
	var res *http.Response
	var err error
	for attempt := 0; ; attempt++ {
		res, err = f.doOnce(ctx, rawURL, ua)
		if err == nil || attempt >= f.opts.Retries || !retryableFetchErr(ctx, err) {
			return res, err
		}
		if !sleepCtx(ctx, fetchRetryDelay) {
			return nil, err // ctx ended — report the original failure, not the wait
		}
	}
}

func (f *fetcher) doOnce(ctx context.Context, rawURL string, ua uaPreset) (*http.Response, error) {
	reqCtx, cancel := context.WithTimeout(ctx, f.opts.timeout())
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, rawURL, nil)
	if err != nil {
		cancel()
		return nil, err
	}
	ua.apply(req, f.opts.acceptLanguage(), f.customHeadersFor(rawURL))

	res, err := f.client.Do(req)
	if err != nil {
		cancel()
		return nil, err
	}
	// The body must outlive this function, so the timeout is released when the
	// body closes rather than on return.
	res.Body = &cancelReadCloser{ReadCloser: res.Body, cancel: cancel}
	return res, nil
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   bool
}

func (c *cancelReadCloser) Close() error {
	err := c.ReadCloser.Close()
	if !c.once {
		c.once = true
		c.cancel()
	}
	return err
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// retryableFetchErr reports whether asking again could plausibly help.
func retryableFetchErr(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return false
	}
	switch classifyError(ctx, err) {
	case ErrConnection, ErrTimeout, ErrRefused:
		return true
	}
	return false
}

// netMsgRe catches the transport failures Go reports only as text.
var netMsgRe = regexp.MustCompile(`(?i)connection reset|network is unreachable|host is unreachable|forcibly closed|wsarecv|wsasend|connectex|broken pipe|server closed|unexpected EOF|malformed HTTP`)

// classifyError maps a transport failure to one of the ErrX slugs.
//
// Order matters: a dial timeout is both a net.Error timeout and a *net.OpError
// and must report as a timeout; errors.As unwraps the *url.Error that
// http.Client returns.
func classifyError(ctx context.Context, err error) string {
	if err == nil {
		return ""
	}
	if ctx != nil && ctx.Err() != nil {
		return ErrCancelled
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrTimeout
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ErrTimeout
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ErrDNS
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "no such host"):
		return ErrDNS
	case strings.Contains(lower, "connection refused"):
		return ErrRefused
	case strings.Contains(lower, "x509"), strings.Contains(lower, "certificate"),
		strings.Contains(lower, "tls"), strings.Contains(lower, "handshake"):
		return ErrSSL
	case netMsgRe.MatchString(msg), strings.Contains(lower, "eof"):
		return ErrConnection
	}
	return ErrConnection
}

// sleepCtx waits for d, reporting false if the context ended first.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// resolveLocation turns a Location header into an absolute URL.
func resolveLocation(current, location string) (string, error) {
	location = strings.TrimSpace(location)
	if location == "" {
		return "", errNoLocation
	}
	base, err := url.Parse(current)
	if err != nil {
		return "", errInvalidURL
	}
	next, err := base.Parse(location)
	if err != nil {
		return "", errBadLocation
	}
	switch strings.ToLower(next.Scheme) {
	case "http", "https":
	default:
		return "", errUnsupportedScheme
	}
	next.Fragment = ""
	return next.String(), nil
}

// isHTML reports whether a Content-Type is markup we should parse. An empty
// type is treated as HTML: servers omit it, and guessing "not a page" would
// silently drop real pages from the crawl.
func isHTML(contentType string) bool {
	if contentType == "" {
		return true
	}
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mt = strings.ToLower(strings.TrimSpace(contentType))
	}
	switch mt {
	case "text/html", "application/xhtml+xml", "application/xhtml", "text/xhtml":
		return true
	}
	return false
}

// kindOf buckets a resource for the grid's tab predicate, preferring the
// declared Content-Type and falling back to the URL extension.
func kindOf(contentType, rawURL string) string {
	mt := strings.ToLower(strings.TrimSpace(contentType))
	switch {
	case mt == "":
		// fall through to the extension
	case isHTML(mt):
		return KindHTML
	case strings.HasPrefix(mt, "image/"):
		return KindImage
	case mt == "text/css":
		return KindCSS
	case mt == "application/pdf":
		return KindPDF
	case strings.HasPrefix(mt, "font/"), mt == "application/font-woff",
		mt == "application/vnd.ms-fontobject", mt == "application/x-font-ttf":
		return KindFont
	case strings.HasPrefix(mt, "video/"), strings.HasPrefix(mt, "audio/"):
		return KindMedia
	case mt == "application/javascript", mt == "text/javascript",
		mt == "application/x-javascript", mt == "module":
		return KindJS
	}

	switch urlExtension(rawURL) {
	case "html", "htm", "php", "asp", "aspx", "jsp", "shtml", "":
		return KindHTML
	case "css":
		return KindCSS
	case "js", "mjs", "cjs":
		return KindJS
	case "pdf":
		return KindPDF
	case "jpg", "jpeg", "png", "gif", "webp", "svg", "avif", "bmp", "ico", "tif", "tiff":
		return KindImage
	case "woff", "woff2", "ttf", "otf", "eot":
		return KindFont
	case "mp4", "webm", "mp3", "wav", "ogg", "mov", "avi", "m4a", "m4v":
		return KindMedia
	}
	return KindOther
}
