package sitecrawl

import "net/http"

// DefaultUserAgent identifies the tool honestly, the way Screaming Frog does.
// A site owner auditing their own site should be able to find us in their access
// log, and a crawler that lies about being Googlebot by default would poison
// their analytics.
const DefaultUserAgent = "sitecrawl"

// fallbackUserAgent is used for the one silent retry after a bot identity is
// refused outright — see fetch's bot-blocked handling.
const fallbackUserAgent = "chrome"

// uaPreset is one selectable identity.
//
// Bot presets deliberately send a minimal header set: a crawler that also
// announces Sec-Fetch-* and Accept-Language is obviously not a crawler, and the
// point of the preset is to trigger a site's UA-conditional rules.
//
// A local copy of indexcheck's preset table: the tool packages stay independent
// of each other by design, so shared plumbing is duplicated rather than
// extracted. The set differs — this tool adds its own identity and the
// smartphone crawler.
type uaPreset struct {
	ID  string
	UA  string
	Bot bool
	// RobotsToken is the User-agent name matched against robots.txt groups.
	RobotsToken string
	// Mobile drives the viewport used when JavaScript rendering is on.
	Mobile bool
}

var uaPresets = map[string]uaPreset{
	"sitecrawl": {
		ID:          "sitecrawl",
		UA:          "Mozilla/5.0 (compatible; 1ScoutSiteCrawl/1.0; +https://1scout.marketing/bot)",
		Bot:         true,
		RobotsToken: "1ScoutSiteCrawl",
	},
	"googlebot": {
		ID:          "googlebot",
		UA:          "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		Bot:         true,
		RobotsToken: "Googlebot",
	},
	"googlebot-mobile": {
		ID:          "googlebot-mobile",
		UA:          "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		Bot:         true,
		RobotsToken: "Googlebot",
		Mobile:      true,
	},
	"bingbot": {
		ID:          "bingbot",
		UA:          "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
		Bot:         true,
		RobotsToken: "bingbot",
	},
	"chrome": {
		ID:          "chrome",
		UA:          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
		RobotsToken: "1ScoutSiteCrawl",
	},
	"chrome-mobile": {
		ID:          "chrome-mobile",
		UA:          "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36",
		RobotsToken: "1ScoutSiteCrawl",
		Mobile:      true,
	},
}

func presetFor(id string) uaPreset {
	if p, ok := uaPresets[id]; ok {
		return p
	}
	return uaPresets[DefaultUserAgent]
}

// apply sets the request headers for this identity, then layers the user's own
// headers on top so a custom Authorization or Cookie always wins.
//
// Accept-Encoding is deliberately NOT set: Go's transport adds gzip itself and
// transparently decompresses the response, but only while the caller leaves the
// header alone. Setting it by hand would hand the tokenizer compressed bytes.
func (p uaPreset) apply(req *http.Request, acceptLang string, custom map[string]string) {
	req.Header.Set("User-Agent", p.UA)
	if p.Bot {
		req.Header.Set("Accept", "*/*")
	} else {
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		if p.ID == "chrome" {
			req.Header.Set("Sec-Fetch-Dest", "document")
			req.Header.Set("Sec-Fetch-Mode", "navigate")
			req.Header.Set("Sec-Fetch-Site", "none")
			req.Header.Set("Sec-Fetch-User", "?1")
			req.Header.Set("Sec-CH-UA", `"Chromium";v="131", "Not_A Brand";v="24", "Google Chrome";v="131"`)
			req.Header.Set("Sec-CH-UA-Mobile", "?0")
			req.Header.Set("Sec-CH-UA-Platform", `"Windows"`)
		}
	}
	if acceptLang != "" {
		req.Header.Set("Accept-Language", acceptLang)
	} else if !p.Bot {
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	}
	for k, v := range custom {
		if k != "" {
			req.Header.Set(k, v)
		}
	}
}

// robotsTokens are the User-agent names a robots.txt group may address us by.
// "*" is handled by the matcher itself.
func (p uaPreset) robotsTokens() []string {
	if p.RobotsToken == "" {
		return []string{"1ScoutSiteCrawl"}
	}
	return []string{p.RobotsToken}
}
