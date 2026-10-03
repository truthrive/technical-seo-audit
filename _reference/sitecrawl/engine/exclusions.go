package sitecrawl

import (
	"net/url"
	"strings"
)

// defaultExclusions are the paths not worth reporting SEO issues about: admin
// panels, auth flows, checkout, build artefacts. Ported from LibreCrawl's
// default issueExclusionPatterns.
//
// They suppress ISSUES, not crawling — a 500 on /wp-admin still shows in the
// grid, it just does not add "missing meta description" to the report.
var defaultExclusions = []string{
	// WordPress
	"/wp-admin/*", "/wp-content/plugins/*", "/wp-content/themes/*",
	"/wp-content/uploads/*", "/wp-includes/*", "/wp-login.php", "/wp-cron.php",
	"/xmlrpc.php", "/wp-json/*", "/wp-activate.php", "/wp-signup.php",
	"/wp-trackback.php",
	// auth
	"/login*", "/signin*", "/sign-in*", "/log-in*", "/auth/*", "/authenticate/*",
	"/register*", "/signup*", "/sign-up*", "/registration/*", "/logout*",
	"/signout*", "/sign-out*", "/log-out*", "/forgot-password*",
	"/reset-password*", "/password-reset*", "/recover-password*",
	"/change-password*", "/account/password/*", "/user/password/*",
	"/activate/*", "/verification/*", "/verify/*", "/confirm/*",
	// admin panels
	"/admin/*", "/administrator/*", "/_admin/*", "/backend/*", "/dashboard/*",
	"/cpanel/*", "/phpmyadmin/*", "/pma/*", "/webmail/*", "/plesk/*",
	"/control-panel/*", "/manage/*", "/manager/*",
	// commerce
	"/checkout/*", "/cart/*", "/basket/*", "/payment/*", "/billing/*",
	"/order/*", "/orders/*", "/purchase/*",
	// account
	"/account/*", "/profile/*", "/settings/*", "/preferences/*",
	"/my-account/*", "/user/*", "/member/*", "/members/*",
	// cgi
	"/cgi-bin/*", "/cgi/*", "/fcgi-bin/*",
	// vcs / config
	"/.git/*", "/.svn/*", "/.hg/*", "/.bzr/*", "/.cvs/*", "/.env", "/.env.*",
	"/.htaccess", "/.htpasswd", "/web.config", "/app.config", "/composer.json",
	"/package.json",
	// build output
	"/node_modules/*", "/vendor/*", "/bower_components/*", "/jspm_packages/*",
	"/includes/*", "/lib/*", "/libs/*", "/src/*", "/dist/*", "/build/*",
	"/builds/*", "/_next/*", "/.next/*", "/out/*", "/_nuxt/*", "/.nuxt/*",
	// test / dev
	"/test/*", "/tests/*", "/spec/*", "/specs/*", "/__tests__/*", "/debug/*",
	"/dev/*", "/development/*", "/staging/*",
	// internal APIs
	"/api/internal/*", "/api/admin/*", "/api/private/*",
	// system
	"/private/*", "/system/*", "/core/*", "/internal/*", "/tmp/*", "/temp/*",
	"/cache/*", "/logs/*", "/log/*", "/backup/*", "/backups/*", "/old/*",
	"/archive/*", "/archives/*", "/config/*", "/configs/*", "/configuration/*",
	// uploads
	"/upload/*", "/uploads/*", "/uploader/*", "/file-upload/*",
	// search / filter / sort
	"/search*", "*/search/*", "?s=*", "?search=*", "*/filter/*", "?filter=*",
	"*/sort/*", "?sort=*",
	// alternate views
	"/print/*", "?print=*", "/preview/*", "?preview=*", "/embed/*", "?embed=*",
	"/amp/*", "/amp",
	// feeds
	"/feed/*", "/feeds/*", "/rss/*", "*.rss", "/atom/*", "*.atom",
	// file types
	"*.json", "*.xml", "*.yaml", "*.yml", "*.toml", "*.ini", "*.conf", "*.log",
	"*.txt", "*.csv", "*.sql", "*.db", "*.bak", "*.backup", "*.old", "*.orig",
	"*.tmp", "*.swp", "*.map", "*.min.js", "*.min.css",
}

// exclusionSet is a compiled matcher.
//
// The naive shape — iterate 200 compiled regexes per URL — is 10 million
// evaluations on a 50k crawl. Splitting by pattern shape means the common cases
// ("/admin/*", "*.json") are a map lookup or a suffix compare, and only the
// handful of patterns with an interior wildcard need a real scan.
type exclusionSet struct {
	exact    map[string]bool
	prefixes []string
	suffixes []string
	globs    []string
	// wantQuery records that at least one pattern mentions a query string, so
	// the matcher only pays for building the query form when it can matter.
	wantQuery bool
}

func newExclusionSet(patterns []string, includeDefaults bool) *exclusionSet {
	e := &exclusionSet{exact: map[string]bool{}}
	if includeDefaults {
		e.add(defaultExclusions)
	}
	e.add(patterns)
	return e
}

func (e *exclusionSet) add(patterns []string) {
	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		// "#" opens a comment line, as in LibreCrawl's settings textarea.
		if p == "" || strings.HasPrefix(p, "#") {
			continue
		}
		if strings.ContainsAny(p, "?&") {
			e.wantQuery = true
		}
		star := strings.Count(p, "*")
		switch {
		case star == 0:
			e.exact[p] = true
		case star == 1 && strings.HasSuffix(p, "*"):
			e.prefixes = append(e.prefixes, strings.TrimSuffix(p, "*"))
		case star == 1 && strings.HasPrefix(p, "*"):
			e.suffixes = append(e.suffixes, strings.TrimPrefix(p, "*"))
		default:
			e.globs = append(e.globs, p)
		}
	}
}

func (e *exclusionSet) empty() bool {
	return e == nil || (len(e.exact) == 0 && len(e.prefixes) == 0 && len(e.suffixes) == 0 && len(e.globs) == 0)
}

// match reports whether a URL's path should have its issues suppressed.
//
// LibreCrawl matches against the path alone, which makes its own "?s=*" and
// "?print=*" defaults dead patterns that can never fire. Here a pattern
// mentioning a query is matched against path+query so those actually work.
func (e *exclusionSet) match(rawURL string) bool {
	if e.empty() {
		return false
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	subjects := [...]string{path, ""}
	n := 1
	if e.wantQuery && u.RawQuery != "" {
		subjects[1] = path + "?" + u.RawQuery
		n = 2
	}

	for i := 0; i < n; i++ {
		s := subjects[i]
		if e.exact[s] {
			return true
		}
		for _, p := range e.prefixes {
			if strings.HasPrefix(s, p) {
				return true
			}
		}
		for _, suf := range e.suffixes {
			if strings.HasSuffix(s, suf) {
				return true
			}
		}
		for _, g := range e.globs {
			if globMatch(g, s) {
				return true
			}
		}
	}
	// A bare "?s=*" style pattern has no path part, so also try it against the
	// query alone — that is how a user reading the default list expects it to
	// behave.
	if e.wantQuery && u.RawQuery != "" {
		q := "?" + u.RawQuery
		for _, p := range e.prefixes {
			if strings.HasPrefix(p, "?") && strings.HasPrefix(q, p) {
				return true
			}
		}
	}
	return false
}

// globMatch matches a pattern with any number of `*` wildcards against a
// string. Anchored at both ends unless the pattern starts or ends with `*`.
func globMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if head := parts[0]; head != "" {
		if !strings.HasPrefix(s, head) {
			return false
		}
		s = s[len(head):]
	}
	tail := parts[len(parts)-1]
	middle := parts[1 : len(parts)-1]
	for _, part := range middle {
		if part == "" {
			continue
		}
		i := strings.Index(s, part)
		if i < 0 {
			return false
		}
		s = s[i+len(part):]
	}
	if tail == "" {
		return true
	}
	return strings.HasSuffix(s, tail)
}
