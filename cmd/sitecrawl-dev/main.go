package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

type progressPrinter struct{}

func (progressPrinter) OnProgress(data any) {
	if p, ok := data.(sitecrawl.ProgressEvent); ok {
		fmt.Fprintf(os.Stderr, "crawled=%d found=%d queued=%d rate=%.1f/s phase=%s\n",
			p.Crawled, p.Found, p.Queued, p.Rate, p.Phase)
	}
}

func main() {
	dbPath := flag.String("db", "crawl.db", "Path to SQLite database file")
	maxURLs := flag.Int("max-urls", sitecrawl.DefaultMaxURLs, "Maximum URLs to crawl")
	maxDepth := flag.Int("max-depth", sitecrawl.DefaultMaxDepth, "Maximum crawl depth")
	concurrency := flag.Int("concurrency", sitecrawl.DefaultConcurrency, "Number of concurrent workers")
	enableJS := flag.Bool("js", false, "Enable JavaScript rendering")
	verbose := flag.Bool("v", false, "Enable verbose progress output")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: sitecrawl-dev [options] <seed-url>\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	seedURL := args[0]

	db, err := standalone.OpenDB(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database at %q: %v", *dbPath, err)
	}
	defer db.Close()

	runner := sitecrawl.NewRunner(db)

	if *verbose {
		runner.Progress = progressPrinter{}
	}

	opts := sitecrawl.Options{
		Mode:               sitecrawl.ModeSpider,
		MaxURLs:            *maxURLs,
		MaxDepth:           *maxDepth,
		Concurrency:        *concurrency,
		FollowRedirects:    true,
		CrawlExternal:      false,
		CrawlImages:        true,
		RespectRobots:      true,
		DiscoverSitemaps:   true,
		EnableJavaScript:   *enableJS,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	summary, err := runner.Crawl(ctx, []string{seedURL}, opts)
	if err != nil && ctx.Err() == nil {
		log.Fatalf("crawl failed: %v", err)
	}

	out, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		log.Fatalf("failed to format summary: %v", err)
	}
	fmt.Println(string(out))
}
