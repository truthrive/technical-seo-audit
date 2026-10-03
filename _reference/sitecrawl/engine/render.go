package sitecrawl

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chromedp/chromedp"
)

// renderer turns a URL into its post-JavaScript DOM. The crawl only ever talks
// to this interface: when rendering is off or no browser exists, noopRenderer
// keeps every call site branch-free.
type renderer interface {
	render(ctx context.Context, rawURL string) (string, error)
	close()
}

type noopRenderer struct{}

func (noopRenderer) render(context.Context, string) (string, error) { return "", errNoRenderer }
func (noopRenderer) close()                                         {}

var errNoRenderer = &renderError{"no renderer"}

type renderError struct{ msg string }

func (e *renderError) Error() string { return e.msg }

// detectBrowser finds an installed Chromium-family browser to launch.
//
// The app never ships one: rendering borrows the Chrome or Edge already on the
// machine, started with its own --user-data-dir so the user's profile —
// sessions, extensions, history — is never touched and never touches us.
func detectBrowser() string {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			base := os.Getenv(env)
			if base == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(base, `Google\Chrome\Application\chrome.exe`),
				filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`),
				filepath.Join(base, `Chromium\Application\chrome.exe`),
			)
		}
	case "darwin":
		// Quét cả /Applications lẫn ~/Applications: cài Chrome mà không có quyền
		// admin thì macOS đặt vào thư mục của user, và đó là trường hợp rất phổ
		// biến trên máy công ty. Thiếu nhánh này thì app kết luận "không có
		// trình duyệt" dù Chrome đang nằm ngay đó.
		roots := []string{"/Applications"}
		if home, err := os.UserHomeDir(); err == nil {
			roots = append(roots, filepath.Join(home, "Applications"))
		}
		// Thứ tự ưu tiên: Chrome bản thường trước, rồi các bản Chromium khác.
		// Tên là đường dẫn tương đối tính từ mỗi root.
		apps := []string{
			"Google Chrome.app/Contents/MacOS/Google Chrome",
			"Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"Chromium.app/Contents/MacOS/Chromium",
			"Brave Browser.app/Contents/MacOS/Brave Browser",
			"Vivaldi.app/Contents/MacOS/Vivaldi",
			"Opera.app/Contents/MacOS/Opera",
			"Arc.app/Contents/MacOS/Arc",
			"Google Chrome Beta.app/Contents/MacOS/Google Chrome Beta",
			"Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
		}
		// Vòng ngoài là app, vòng trong là root: ưu tiên ĐÚNG TRÌNH DUYỆT quan
		// trọng hơn ưu tiên vị trí. Chrome ở ~/Applications vẫn phải thắng Arc ở
		// /Applications — Googlebot chạy Chromium nên Chrome là bản render sát
		// thực tế nhất.
		for _, app := range apps {
			for _, root := range roots {
				candidates = append(candidates, filepath.Join(root, app))
			}
		}
	default:
		for _, name := range []string{"google-chrome", "chromium", "chromium-browser", "microsoft-edge"} {
			if p, err := exec.LookPath(name); err == nil {
				return p
			}
		}
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// chromeRenderer drives one headless browser process; each render is one tab.
type chromeRenderer struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	opts        Options
	// sem caps concurrent tabs at JSConcurrency — a tab is a real renderer
	// process, and 20 of them is a machine on its knees.
	sem     chan struct{}
	userDir string
}

// newChromeRenderer starts the allocator (the browser launches lazily on the
// first tab). Returns nil when no browser is installed.
func newChromeRenderer(opts Options) *chromeRenderer {
	execPath := detectBrowser()
	if execPath == "" {
		return nil
	}
	userDir, err := os.MkdirTemp("", "onescout-render-*")
	if err != nil {
		return nil
	}
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(execPath),
		chromedp.UserDataDir(userDir),
		chromedp.WindowSize(opts.JSViewportWidth, opts.JSViewportHeight),
		chromedp.UserAgent(opts.preset().UA),
		// The crawl already respects robots and politeness; the browser only
		// re-fetches pages the HTTP path was allowed to fetch.
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("mute-audio", true),
		// chromedp's defaults switch off site-per-process. This browser executes
		// JavaScript from pages we did not write, so the isolation it provides is
		// worth keeping; the flag is re-set here without it, and a later flag wins.
		// (The OS sandbox is untouched — no --no-sandbox anywhere — and the
		// throwaway user-data-dir above means no profile or cookies are present.)
		chromedp.Flag("disable-features", "Translate,BlinkGenPropertyTrees"),
	)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOpts...)
	return &chromeRenderer{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		opts:        opts,
		sem:         make(chan struct{}, opts.JSConcurrency),
		userDir:     userDir,
	}
}

// render loads the URL in a fresh tab, waits the settle delay, and returns the
// serialized DOM.
func (r *chromeRenderer) render(ctx context.Context, rawURL string) (string, error) {
	select {
	case r.sem <- struct{}{}:
		defer func() { <-r.sem }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	tabCtx, cancel := chromedp.NewContext(r.allocCtx)
	defer cancel()
	tabCtx, cancelTimeout := context.WithTimeout(tabCtx, time.Duration(r.opts.JSTimeoutSec)*time.Second)
	defer cancelTimeout()

	// The crawl's own ctx (pause/stop) must also end the tab.
	go func() {
		select {
		case <-ctx.Done():
			cancel()
		case <-tabCtx.Done():
		}
	}()

	var html string
	err := chromedp.Run(tabCtx,
		chromedp.Navigate(rawURL),
		chromedp.Sleep(r.opts.jsWait()),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		return "", err
	}
	return html, nil
}

func (r *chromeRenderer) close() {
	r.allocCancel()
	// The browser holds the dir until it exits; give it a moment, then sweep.
	// Best-effort — a leftover temp dir is untidy, not incorrect.
	go func() {
		time.Sleep(2 * time.Second)
		os.RemoveAll(r.userDir)
	}()
}

// renderGate decides which URLs are worth a browser: internal HTML documents
// matching the user's patterns (none = all), under a hard cap enforced here in
// Go — a UI-only cap would be a suggestion.
type renderGate struct {
	patterns []*regexp.Regexp
	maxPages int64
	rendered atomic.Int64
}

func newRenderGate(opts Options) *renderGate {
	return &renderGate{
		patterns: compilePatterns(opts.JSPatterns),
		maxPages: int64(opts.JSMaxPages),
	}
}

// admit reports whether this URL may render, consuming one slot when it does.
func (g *renderGate) admit(rawURL string) bool {
	if len(g.patterns) > 0 {
		ok := false
		for _, re := range g.patterns {
			if re.MatchString(rawURL) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	// Reserve, then roll back on overshoot: two workers racing past the cap
	// would otherwise both render.
	if g.rendered.Add(1) > g.maxPages {
		g.rendered.Add(-1)
		return false
	}
	return true
}

func (g *renderGate) release() { g.rendered.Add(-1) }

// maybeRender re-extracts an already-fetched HTML page from its post-JS DOM,
// reporting whether it did. Runs on the WORKER: a render takes seconds and the
// coordinator goroutine is the one thing that must never block that long.
//
// The HTTP fetch stays the source of transport truth (status, redirect chain,
// headers, size); rendering only replaces the parsed document. That means an
// eligible page costs two fetches — the same trade Screaming Frog makes, and
// far simpler than teaching the browser to report hops and X-Robots-Tag.
func maybeRender(ctx context.Context, r renderer, gate *renderGate, res *fetched, internal bool) bool {
	if r == nil || gate == nil {
		return false
	}
	if res.Kind != KindHTML || !internal || res.FinalStatus < 200 || res.FinalStatus >= 300 || res.Doc == nil {
		return false
	}
	if !gate.admit(res.FinalURL) {
		return false
	}
	html, err := r.render(ctx, res.FinalURL)
	if err != nil || strings.TrimSpace(html) == "" {
		gate.release() // a failed render should not eat the cap
		return false
	}
	base, err := url.Parse(res.FinalURL)
	if err != nil || base.Host == "" {
		gate.release()
		return false
	}
	res.Doc = extractDocument(strings.NewReader(html), base)
	return true
}
