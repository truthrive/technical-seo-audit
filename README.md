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
- **CLI Subcommands**: Checkpoint resume exists in the `Runner` API (`Runner.Resume`), and persisted run, page, and link data can be read through current `Runner` readback methods (`Runner.LoadRun`, `Runner.Pages`, `Runner.Links`, `Runner.URLs`). These lifecycle and readback operations are not exposed as CLI subcommands.

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

- **SiteCrawl Standalone Core**: **FREEZE v1 COMPLETE** (tagged `sitecrawl-standalone-core-v1` at commit `50760ed89f58b4e3abc68614363536c05516e07a`).
- **Audit V1.1 (Domain Contracts & Rule Registry)**: **COMPLETE / LOCKED** (in `internal/audit/`; domain entities, enums, policy contracts, and machine-readable 47-rule registry).
- **Audit V1.2 (Evidence Adapter & Frozen Snapshot Boundary)**: **COMPLETE / LOCKED** (in `internal/audit/adapter/`; translates completed SiteCrawl SQLite runs into normalized observations and frozen `EvidenceSnapshot`, reporting explicit `EvidenceGap` diagnostics; semantically hardened for truthful evidence representation and crawl completeness).
- **Audit V1.3a (Rule Engine Core + AR-ACC-004 Vertical Slice)**: **COMPLETE / LOCKED** (in `internal/audit/engine/`; evaluates frozen `EvidenceSnapshot` against `AR-ACC-004 — Unexpected server-error response` producing deterministic `RuleResult` and `RuleEvidenceRef` objects).
- **Audit V1.3b1 (Policy Boundary Core)**: **COMPLETE / LOCKED** (in `internal/audit/engine/`; introduces immutable `PolicyIndex`, `EvaluationContext`, and context-aware `EvaluateRuleWithContext` policy boundary; preserves `PROJECT POLICY != OBSERVED EVIDENCE`; backward-compatible `EvaluateRule` retained).
- **Audit V1.3b2 (Directive Source & Scope Hardening)**: **COMPLETE / LOCKED** (in `internal/audit/adapter/`; strict directive-scope semantics; unqualified `effective_noindex` suppressed when any agent or unknown scope exists; repeated generic meta preserved as generic scope; unknown header prefixes remain conservative unknown scope; `meta_robots_raw` and `x_robots_raw` restricted to proven generic evidence; scoped `<agent>_*_raw` fields separated).
- **Audit V1.3c (AR-INDEX-002 Vertical Slice)**: **COMPLETE / LOCKED** (in `internal/audit/engine/`; implements typed evaluator `AR-INDEX-002 — Conflicting index directives`; evaluates explicit contradictory `index` and `noindex` directives per exact scope without inheritance or precedence conflation; hardened evidence correlation requiring explicit `directive:*` references without fallback guessing; deterministic status precedence and traceable evidence refs; adapter and crawler remain unchanged).
- **Audit V1.3d (Googlebot Effective Noindex Normalization)**: **COMPLETE / LOCKED** (in `internal/audit/adapter/`; derives URL-level `effective_noindex` representing truthful Googlebot-effective indexability derived from generic `*` and explicit `googlebot` scopes with cumulative restrictive rules; typed `RobotsDirectiveObservation.EffectiveNoindex` aligned with Googlebot-effective semantics; unrelated bot scopes do not contaminate Googlebot indexability; ambiguous and incomplete/raw-render evidence safely withheld; NormalizationVersion remains `v1.4.0`; executable rule count remains exactly 2).
- **Audit V1.3e (AR-INDEX-001 Vertical Slice)**: **COMPLETE / LOCKED** (in `internal/audit/engine/`; implements typed evaluator `AR-INDEX-001 — Effective noindex conflicts with explicit indexability intent`; evaluates explicit `expected_indexable` policy from `PolicyIndex` against normalized `effective_noindex` without policy inference; deterministic subject set union over evidence and policy targets; executable rule count = 3 [`AR-ACC-004`, `AR-INDEX-001`, `AR-INDEX-002`]; adapter and frozen crawler remain unchanged).
- **Audit V1.4a (AR-CANON-003 Vertical Slice)**: **COMPLETE / LOCKED** (in `internal/audit/engine/`; implements typed evaluator `AR-CANON-003 — Canonical declaration missing`; evaluates 2xx HTML responses for presence of canonical declaration; 2xx HTML with canonical present evaluates to `PASS`; 2xx HTML without canonical on unrendered trustworthy extraction evaluates to `WARNING`; rendered 2xx HTML without raw canonical evaluates conservatively to `UNKNOWN`; non-HTML and non-2xx evaluate to `NOT_APPLICABLE`; no synthetic evidence refs generated; executable rule count = 4 [`AR-ACC-004`, `AR-CANON-003`, `AR-INDEX-001`, `AR-INDEX-002`]; canonical implementation has started; adapter and frozen crawler remain unchanged).
- **Audit V1.4b (Canonical Evidence Enablement)**: **COMPLETE / LOCKED** (in `internal/audit/adapter/`; derives truthful normalized canonical observations on non-rendered HTML: `canonical_count`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count`, and exact `canonical_target_subject_ref` correlation; preserves existing `canonical_resolved`; rendered pages safely withhold missing raw canonical evidence; target status and noindex are not duplicated onto source URLs; NormalizationVersion bumped to `v1.5.0`; enables upcoming `AR-CANON-004`, `AR-CANON-006`, and `AR-CANON-007`; executable rule count remains exactly 4).
- **Audit V1.4c (Canonical Evaluator Batch)**: **COMPLETE / LOCKED** (in `internal/audit/engine/`; implements typed evaluators for `AR-CANON-004 — Multiple canonical declarations`, `AR-CANON-006 — Canonical target returns final 200`, and `AR-CANON-007 — Canonical target is not noindex`; evaluates multiple declarations, distinct normalized targets, deterministic source-to-target subject correlation, target HTTP status, and target effective noindex state; executable rule count moves from 4 → 7 [`AR-ACC-004`, `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, `AR-CANON-007`, `AR-INDEX-001`, `AR-INDEX-002`]; adapter and frozen crawler remain unchanged).
- **Audit V1.5a (Redirect Evidence Enablement & Integrity Hardening)**: **COMPLETE** (in `internal/audit/adapter/`; enables truthful normalized redirect evidence: `redirect_initial_observed`, `redirect_hop_count`, deterministic ordered `redirect_hop`, explicit `redirect_traversal_complete`, truthful `redirect_loop_detected`, and normalized `redirect_final_url`; hardens redirect evidence integrity: validates promoted `redirect_hops` against `len(Page.Redirects)`, recording `GAP_REDIRECT_CHAIN_INCONSISTENT` and withholding completeness/loop-absence/final-target conclusions and `redirect_hop_count` on conflict; sets typed `RedirectHop.LocationRaw` to empty to reflect that raw Location header is not preserved by frozen SiteCrawl while preserving `ResolvedTargetURL`; corrects `FetchObservation.FinalURLID` to map only actual final destination on complete traversal and never first-hop `RedirectTo`; preserves final status boundary without fabricating unpersisted source-chain `FinalStatus`; marks `AR-CANON-008` and `AR-CANON-009` as READY_TO_IMPLEMENT; leaves `AR-CANON-010` and `AR-ACC-003` BLOCKED; NormalizationVersion remains `v1.6.0`; executable rule count remains exactly 7).
- **Phase 4 (First Vertical Slice: HTTP / Robots / Index / Canonical)**: **IN PROGRESS**. Exactly 7 rules are executable (`AR-ACC-004`, `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, `AR-CANON-007`, `AR-INDEX-001`, `AR-INDEX-002`); the other 40 rules remain registered but unimplemented. Readiness status: `AR-ACC-004` is EXECUTABLE, `AR-CANON-003` is EXECUTABLE, `AR-CANON-004` is EXECUTABLE, `AR-CANON-005` remains BLOCKED (raw declaration syntax unavailable in frozen crawler), `AR-CANON-006` is EXECUTABLE, `AR-CANON-007` is EXECUTABLE, `AR-CANON-008` is READY_TO_IMPLEMENT, `AR-CANON-009` is READY_TO_IMPLEMENT, `AR-CANON-010` remains BLOCKED (persisted source-chain `FinalStatus` unavailable), `AR-ACC-003` remains BLOCKED (same authoritative `final_status` limitation), `AR-INDEX-001` is EXECUTABLE, `AR-INDEX-002` is EXECUTABLE, `AR-INDEX-003` remains BLOCKED (no frozen directive registry). NormalizationVersion is `v1.6.0`. No Finding aggregation or Audit SQL persistence exists yet. Frozen crawler core remains unchanged.
