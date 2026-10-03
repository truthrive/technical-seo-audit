package sitecrawl

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

// TestNetworkFailure_ConnectionRefused tests handling of a closed local listener:
// The page records ErrRefused or ErrConnection with status 0, and the crawl
// stops with StopSeedUnreachable.
func TestNetworkFailure_ConnectionRefused(t *testing.T) {
	fastCrawl(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := l.Addr().String()
	l.Close() // closed immediately

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}.normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	summary, err := runner.Crawl(ctx, []string{"http://" + addr}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	if summary.StopReason != StopSeedUnreachable {
		t.Errorf("stop reason = %s, want %s", summary.StopReason, StopSeedUnreachable)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page recorded for unreachable seed")
	}
	p := pages[0]
	if p.Status != 0 {
		t.Errorf("status = %d, want 0", p.Status)
	}
	if p.ErrorType != ErrRefused && p.ErrorType != ErrConnection {
		t.Errorf("error type = %s, want ErrRefused or ErrConnection", p.ErrorType)
	}
	if p.Error == "" {
		t.Error("expected error message to be recorded")
	}
}

// TestNetworkFailure_Timeout tests fetch timeout:
// When server response exceeds the timeout budget, the fetch records ErrTimeout.
func TestNetworkFailure_Timeout(t *testing.T) {
	fastCrawl(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>Delayed</body></html>`)
	}))
	t.Cleanup(srv.Close)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		TimeoutSec:       1,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}.normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	summary, err := runner.Crawl(ctx, []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("expected page recorded for timeout")
	}
	p := pages[0]
	if p.ErrorType != ErrTimeout {
		t.Errorf("error type = %s, want %s", p.ErrorType, ErrTimeout)
	}
}

// TestNetworkFailure_TLSCertificate tests SSL certificate error handling:
// Without IgnoreSSL, connecting to an untrusted local TLS server records ErrSSL.
func TestNetworkFailure_TLSCertificate(t *testing.T) {
	fastCrawl(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>TLS content</body></html>`)
	}))
	t.Cleanup(srv.Close)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		IgnoreSSL:        false,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}.normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	summary, err := runner.Crawl(ctx, []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("expected page recorded for SSL error")
	}
	p := pages[0]
	if p.ErrorType != ErrSSL {
		t.Errorf("error type = %s, want %s", p.ErrorType, ErrSSL)
	}
	if p.Status != 0 {
		t.Errorf("status = %d, want 0", p.Status)
	}
}

// TestNetworkFailure_IgnoreSSL tests successful fetch when explicitly ignoring SSL errors.
func TestNetworkFailure_IgnoreSSL(t *testing.T) {
	fastCrawl(t)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>TLS content</body></html>`)
	}))
	t.Cleanup(srv.Close)

	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	opts := Options{
		IgnoreSSL:        true,
		RespectRobots:    false,
		DiscoverSitemaps: false,
	}.normalized()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	summary, err := runner.Crawl(ctx, []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Crawl: %v", err)
	}

	pages, err := runner.Pages(summary.ID)
	if err != nil {
		t.Fatalf("Pages: %v", err)
	}
	if len(pages) == 0 {
		t.Fatal("expected page recorded for IgnoreSSL")
	}
	p := pages[0]
	if p.Status != 200 {
		t.Errorf("status = %d, want 200", p.Status)
	}
	if p.ErrorType != "" {
		t.Errorf("error type = %s, want empty", p.ErrorType)
	}
}
