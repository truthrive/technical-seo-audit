# Technical SEO Audit — SiteCrawl Standalone Core

A high-performance, standalone technical SEO web crawler and persistence engine built in Go.

This repository hosts the frozen **SiteCrawl Standalone Core (v1)**, which serves as the deterministic crawling and extraction foundation for the upcoming Technical SEO Audit engine.

## Provenance & Attribution

The crawler foundation derives from the **LibreCrawl** project (MIT License, Python/Flask specifications for crawl rules, link extraction, and thresholds) and the historical **1Scout SiteCrawl** port (desktop crawler integration).

All historical reference code and lineage are preserved under `_reference/sitecrawl/` and across Git history. The pre-audit reference state is anchored at immutable tag `sitecrawl-pre-audit-v1`.

## Architecture & Package Layout

The root Go module contains only active, buildable packages:

- `internal/sitecrawl/`: The core standalone crawler engine, coordinator, extraction pipeline, frontier scheduler, robots/sitemap parser, and lifecycle manager.
- `internal/platform/standalone/`: Standalone SQLite storage drivers, schema migration engine (`Migrate`), and persistence helpers.
- `cmd/sitecrawl-dev/`: Developer CLI tool for running standalone crawls and inspecting progress.
- `go/deps/httpx/`: Resilient HTTP client utilities, proxy hooks, and private network guards.
- `go/deps/safe/`: Panic recovery wrappers protecting crawler goroutines.
- `_reference/sitecrawl/`: Isolated historical reference source (`engine/`, `storage/`, `pagespeed/`, `app-glue/`, `tests/`), ignored by Go toolchain builds.

## Developer CLI (`cmd/sitecrawl-dev`)

The developer CLI provides local execution and debugging capabilities for starting standalone crawls.

Flags must precede the positional seed URL per standard Go `flag` parsing:

```bash
# Build the CLI
go build ./cmd/sitecrawl-dev

# Run a basic crawl
go run ./cmd/sitecrawl-dev -db crawl.db https://example.com

# Expanded invocation with crawler options
go run ./cmd/sitecrawl-dev -db crawl.db -max-urls 500 -max-depth 3 -concurrency 5 -js -v https://example.com
```

### CLI Capabilities & Lifecycle

- **Start Crawl**: The CLI currently starts a crawl from the provided seed URL and prints the JSON summary to stdout. The `-v` flag streams live progress events to stderr.
- **Lifecycle API**: The core `Runner` API (`internal/sitecrawl`) supports lifecycle operations including in-process pause/resume, graceful stop, and SQLite checkpoint resumption (`Runner.Start`, `CrawlHandle.Pause`, `CrawlHandle.Resume`, `CrawlHandle.Stop`, `Runner.Resume`).
- **CLI Subcommands**: Checkpoint resume, database inspection, and benchmarking exist in the programmatic core and test suites but are not yet exposed as CLI subcommands.

### Persistence & Schema

- **Engine**: Pure-Go SQLite (`modernc.org/sqlite`) with WAL mode enabled.
- **Schema**: 38 deterministic, append-only migration steps validated by golden digest tests (`209ab3ee14d732cc16a77c09bc75829f4445d09f680ee708268d69b30717dab0`).
- **Data Model**: URL dictionary encoding (`sitecrawl_urls`), page attributes (`sitecrawl_pages`), internal/external link graph (`sitecrawl_links`), and full-text search index (`sitecrawl_fts`).

### Checkpoint & Pause/Resume

- **Durable Checkpointing**: Active frontier URLs and seen sets are persisted to SQLite on pause, stop, or periodic checkpoint intervals.
- **Process Restart Resume**: Resumes can be initiated in-process or from a new process against the database.
- **Eligibility & Safety**: Checkpoint resume verifies run eligibility (`paused` or `stopped` with `resumable=1`). State verification, frontier loading, and coordinator setup are completed prior to claiming the run in SQLite, preventing corrupted or stale claims.

### Rendering

- Headless rendering uses locally available Google Chrome or Microsoft Edge via `chromedp`.
- Rendering is governed by pattern filters (`Include/ExcludePatterns`), per-run page caps (`JSMaxPages`), and concurrency throttles (`JSConcurrency`).
- Operates gracefully in headless CLI environments, falling back to direct HTTP extraction if no supported browser binary is detected.

### Optional & Deferred Capabilities

- **PageSpeed Insights**: Deferred from the standalone acquisition core. Resuming or starting with `EnablePageSpeed=true` is rejected with `ErrCapabilityUnsupported`.
- **Duplicate Analysis**: Deferred. Resuming or starting with `EnableDuplication=true` is rejected with `ErrCapabilityUnsupported`.
- **Proxy**: Requires an explicit `httpx.ProxyFunc` runtime provider; runs specifying `UseProxy=true` without a proxy provider return `ErrProxyUnavailable`.

## Project Status & Audit V1 Roadmap

- **SiteCrawl Standalone Core**: **FREEZE v1 COMPLETE** (tagged `sitecrawl-standalone-core-v1`).
- **Audit V1**: **NOT STARTED**. Audit V1 implementation (Evidence Adapter, EvidenceSnapshot, Rule Engine, 47 `AR-*` audit rules, Project Policy, scoring) will begin on top of this frozen core in subsequent milestones.
