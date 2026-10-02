package sitecrawl

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixtureSite is a small site with one of every shape the crawler has to get
// right. Every test in this package crawls it.
type fixtureSite struct {
	*httptest.Server
	mu    sync.Mutex
	hits  map[string]int
	total int
	// order records request arrival for the politeness test.
	order []hit
}

type hit struct {
	path string
	at   int64 // nanoseconds since the server started
}

func (f *fixtureSite) count(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.hits[path]
}

func (f *fixtureSite) totalRequests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.total
}

func (f *fixtureSite) requestLog() []hit {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]hit, len(f.order))
	copy(out, f.order)
	return out
}

// page renders a minimal but complete HTML document.
func page(title, meta, h1, body string, extraHead string) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>%s</title>
<meta name="description" content="%s">
<meta name="viewport" content="width=device-width, initial-scale=1">
%s
</head>
<body>
<header><a href="/">Home</a></header>
<nav><a href="/about">About</a><a href="/contact">Contact</a></nav>
<main><h1>%s</h1><h2>Section</h2>%s</main>
<footer><a href="/privacy">Privacy</a></footer>
</body></html>`, title, meta, extraHead, h1, body)
}

func filler(words int) string {
	return "<p>" + strings.Repeat("alpha beta gamma delta epsilon zeta eta theta ", words/8) + "</p>"
}

func newFixtureSite(t *testing.T) *fixtureSite {
	t.Helper()
	f := &fixtureSite{hits: map[string]int{}}

	// The server has to exist before the handlers are registered: several of
	// them embed f.URL in absolute canonical and hreflang hrefs, which is
	// exactly what the real rules compare against.
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	f.Server = srv
	t.Cleanup(srv.Close)
	start := time.Now()
	record := func(h http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			f.hits[r.URL.Path]++
			f.total++
			f.order = append(f.order, hit{path: r.URL.Path, at: int64(time.Since(start))})
			f.mu.Unlock()
			h(w, r)
		}
	}
	html := func(body string) http.HandlerFunc {
		return record(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(body))
		})
	}

	mux.HandleFunc("/robots.txt", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "User-agent: *\nDisallow: /private/\nSitemap: %s/sitemap.xml\n", f.URL)
	}))
	mux.HandleFunc("/sitemap.xml", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>%s/</loc></url>
  <url><loc>%s/orphan</loc><lastmod>2026-01-01</lastmod></url>
</urlset>`, f.URL, f.URL)
	}))

	// The homepage links to everything interesting.
	mux.HandleFunc("/", record(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(page("Not found", "", "404", "", "")))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(page(
			"Home page of the fixture site",
			"A description that is comfortably inside the recommended range for a meta description on this page.",
			"Welcome",
			filler(400)+
				`<a href="/about">About us</a>
				 <a href="/missing">Broken link</a>
				 <a href="/hop1">Redirect chain</a>
				 <a href="/loop-a">Loop</a>
				 <a href="/private/secret">Blocked</a>
				 <a href="/asset.css">Stylesheet</a>
				 <a href="/dup-a">Dup A</a>
				 <a href="/dup-b">Dup B</a>
				 <a href="/canon-a">Canonical chain</a>
				 <a href="/en/page">EN</a>
				 <a href="/vi/page">VI</a>
				 <a href="/noindex" rel="nofollow">Noindex</a>
				 <a href="https://external.example.org/somewhere">External</a>
				 <img src="/logo.png" alt="Logo">
				 <img src="/broken.png" alt="Broken">
				 <img src="data:image/gif;base64,R0lGODlhAQABAAAAACw=" data-src="/lazy.png" alt="Lazy">`,
			`<link rel="canonical" href="`+f.URL+`/">
			 <link rel="stylesheet" href="/style.css">
			 <script src="/app.js"></script>`)))
	}))

	for _, p := range []string{"/about", "/contact", "/privacy"} {
		mux.HandleFunc(p, html(page(
			"Page "+p+" of the fixture site here",
			"Another description of a sensible length so that the meta description rule does not fire on it.",
			"Heading "+p, filler(400), "")))
	}

	// A three-hop 301 chain ending on a real page.
	for i, next := range map[string]string{"/hop1": "/hop2", "/hop2": "/hop3", "/hop3": "/about"} {
		from, to := i, next
		mux.HandleFunc(from, record(func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, to, http.StatusMovedPermanently)
		}))
	}

	// A redirect loop.
	mux.HandleFunc("/loop-a", record(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop-b", http.StatusFound)
	}))
	mux.HandleFunc("/loop-b", record(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/loop-a", http.StatusFound)
	}))

	mux.HandleFunc("/private/secret", html(page("Secret", "", "Secret", "", "")))

	mux.HandleFunc("/asset.css", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte("body{color:red}"))
	}))
	mux.HandleFunc("/logo.png", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))
	mux.HandleFunc("/broken.png", record(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	// Reachable only through a lazy-loader's data-src: the real src attribute
	// holds a data: placeholder.
	mux.HandleFunc("/lazy.png", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write([]byte{0x89, 'P', 'N', 'G'})
	}))
	// Referenced only by <link rel=stylesheet> / <script src> — never <a href> —
	// so these are only reachable when resource crawling fetches them.
	mux.HandleFunc("/style.css", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte("main{margin:0}"))
	}))
	mux.HandleFunc("/app.js", record(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte("console.log(1)"))
	}))

	// Two pages sharing a title and description.
	dup := page("Duplicated title", "Duplicated description that is long enough to avoid the short-description rule entirely.",
		"Same heading", filler(400), "")
	mux.HandleFunc("/dup-a", html(dup))
	mux.HandleFunc("/dup-b", html(dup))

	// A canonical chain: canon-a -> canon-b -> canon-c.
	mux.HandleFunc("/canon-a", html(page("Canonical A page title here", "Description for canonical A that runs to a reasonable length for the rule.",
		"Canon A", filler(400), `<link rel="canonical" href="`+f.URL+`/canon-b">`)))
	mux.HandleFunc("/canon-b", html(page("Canonical B page title here", "Description for canonical B that runs to a reasonable length for the rule.",
		"Canon B", filler(400), `<link rel="canonical" href="`+f.URL+`/canon-c">`)))
	mux.HandleFunc("/canon-c", html(page("Canonical C page title here", "Description for canonical C that runs to a reasonable length for the rule.",
		"Canon C", filler(400), `<link rel="canonical" href="`+f.URL+`/canon-c">`)))

	// An hreflang pair where the VI side never links back.
	mux.HandleFunc("/en/page", html(page("English page title goes here", "English description that runs to a reasonable length for the meta rule.",
		"English", filler(400),
		`<link rel="canonical" href="`+f.URL+`/en/page">
		 <link rel="alternate" hreflang="en" href="`+f.URL+`/en/page">
		 <link rel="alternate" hreflang="vi" href="`+f.URL+`/vi/page">`)))
	mux.HandleFunc("/vi/page", html(page("Vietnamese page title here", "Vietnamese description that runs to a reasonable length for the meta rule.",
		"Tiếng Việt", filler(400),
		`<link rel="canonical" href="`+f.URL+`/vi/page">
		 <link rel="alternate" hreflang="vi" href="`+f.URL+`/vi/page">`)))

	mux.HandleFunc("/noindex", html(page("Noindex page title goes here", "Description of the noindex page, long enough to pass the meta rule cleanly.",
		"Noindex", filler(400), `<meta name="robots" content="noindex, follow">`)))

	// Reachable only from the sitemap: the orphan.
	mux.HandleFunc("/orphan", html(page("Orphan page title goes here", "Description of the orphan page, long enough to pass the meta rule cleanly.",
		"Orphan", filler(400), "")))

	return f
}
