package sitecrawl

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"onescout/desktop/internal/core/credset"
	"onescout/desktop/internal/core/httpx"
)

// Connections is the one screen that manages credentials, and it stores every
// provider through credset — a JSON list, never a bare value. A tool that reads
// its own vault name with a plain Get() gets that JSON back and sends it to the
// provider as if it were the secret, which is how a key that the user can see
// in Connections produced "API key not valid" on every measurement.

func TestPageSpeedKeyFromConnections(t *testing.T) {
	v := newFakeVault()
	s := &Service{secrets: v}

	if _, err := credset.Add(v, VaultKeyPageSpeed, "work", "AIzaTestKey"); err != nil {
		t.Fatalf("add key: %v", err)
	}

	got, ok := s.psiKey()
	if !ok {
		t.Fatal("psiKey reports no key after Connections saved one")
	}
	if got != "AIzaTestKey" {
		t.Errorf("psiKey = %q, want the bare secret", got)
	}
}

// A key written by an older build sits in the vault as a bare string. credset
// decodes that shape too, so upgrading must not lose it.
func TestPageSpeedKeyLegacyBareValue(t *testing.T) {
	v := newFakeVault()
	s := &Service{secrets: v}
	if err := v.Set(VaultKeyPageSpeed, "AIzaLegacy"); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, ok := s.psiKey()
	if !ok || got != "AIzaLegacy" {
		t.Errorf("psiKey = %q/%v, want AIzaLegacy/true", got, ok)
	}
}

func TestPageSpeedKeyAbsent(t *testing.T) {
	s := &Service{secrets: newFakeVault()}
	if got, ok := s.psiKey(); ok {
		t.Errorf("psiKey = %q/true on an empty vault, want no key", got)
	}
}

// A disabled credential is one the user switched off in Connections; spending
// it anyway would ignore that switch.
func TestPageSpeedKeySkipsDisabled(t *testing.T) {
	v := newFakeVault()
	s := &Service{secrets: v}
	if err := credset.Save(v, VaultKeyPageSpeed, []credset.Credential{
		{ID: "a", Secret: "AIzaOff", Enabled: false},
		{ID: "b", Secret: "AIzaOn", Enabled: true},
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, ok := s.psiKey()
	if !ok || got != "AIzaOn" {
		t.Errorf("psiKey = %q/%v, want the enabled credential", got, ok)
	}
}

// The proxy is the same story: Connections writes it through credset, and
// crawling with a mangled proxy silently falls back to a direct connection —
// the crawl looks fine and leaks the user's real IP to the site being audited.
func TestProxyFromConnections(t *testing.T) {
	v := newFakeVault()
	s := &Service{secrets: v}
	if _, err := credset.Add(v, VaultKeyProxy, "", "http://user:pass@127.0.0.1:8080"); err != nil {
		t.Fatalf("add proxy: %v", err)
	}

	u := s.proxy()
	if u == nil {
		t.Fatal("proxy() = nil after Connections saved one")
	}
	if u.Host != "127.0.0.1:8080" {
		t.Errorf("proxy host = %q, want 127.0.0.1:8080", u.Host)
	}
}

// Having a proxy in Connections must not, on its own, send the crawl through it.
// It used to: a proxy saved for another tool re-routed every crawl silently.
// The switch is in Crawl configuration and starts off, like Screaming Frog's.
//
// Asserted on the transport as well as the func, because that is where the
// decision lands — httpx only installs Transport.Proxy when the func is non-nil,
// and a live request to 127.0.0.1 would prove nothing either way (httpx bypasses
// the proxy for local addresses).
func TestCrawlGoesDirectUnlessProxyEnabled(t *testing.T) {
	v := newFakeVault()
	s := &Service{secrets: v}
	if _, err := credset.Add(v, VaultKeyProxy, "", "http://127.0.0.1:8080"); err != nil {
		t.Fatalf("add proxy: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "https://example.com/", nil)

	if fn := s.crawlProxy(Options{}); fn != nil {
		t.Errorf("crawl would use proxy %v with the setting off", fn())
	}
	off, ok := httpx.NewFiltered(time.Second, s.crawlProxy(Options{}), true).Transport.(*http.Transport)
	if !ok {
		t.Fatal("httpx client is not *http.Transport")
	}
	if off.Proxy != nil {
		if u, _ := off.Proxy(req); u != nil {
			t.Errorf("crawl transport carries proxy %s with the setting off", u)
		}
	}

	// And on, it must actually reach the saved proxy — a switch that quietly does
	// nothing is worse than no switch.
	on, ok := httpx.NewFiltered(time.Second, s.crawlProxy(Options{UseProxy: true}), true).Transport.(*http.Transport)
	if !ok {
		t.Fatal("httpx client is not *http.Transport")
	}
	if on.Proxy == nil {
		t.Fatal("crawl transport has no proxy with the setting on")
	}
	u, _ := on.Proxy(req)
	if u == nil || u.Host != "127.0.0.1:8080" {
		t.Errorf("proxy with the setting on = %v, want 127.0.0.1:8080", u)
	}
}
