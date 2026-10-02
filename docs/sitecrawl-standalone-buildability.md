# SiteCrawl Standalone Buildability Inventory (Checkpoint A1)

- **Status:** Completed
- **Checkpoint:** A1 — Buildability Inventory
- **Upstream Map:** [`sitecrawl-standalone-migration-map.md`](sitecrawl-standalone-migration-map.md)
- **Baseline Git Tag:** `sitecrawl-pre-audit-v1` (`82c610d433203374473e1ea76edf4e6c165a3161`)
- **Nature of Work:** Strictly read-only analysis. No production code, tests, `go.mod`, or configuration files are modified during Checkpoint A1.

---

## 1. Executive Summary

This inventory evaluates the buildability of the extracted SiteCrawl module from 1Scout Marketing (commit `2e2cbff`, 2026-09-25) in its current state within `truthrive/technical-seo-audit`.

### Key Findings
1. **Total Go Files Inspected**: Exactly **58 Go source files** across 6 directories:
   - `go/engine/`: 15 files (~5,000 lines) — core crawler mechanics.
   - `go/storage/`: 9 files (~3,900 lines) — SQLite persistence, query, export, run lifecycle, and post-crawl finalization.
   - `go/pagespeed/`: 3 files (~1,050 lines) — PageSpeed Insights API integration and parallel measurement pump.
   - `go/app-glue/`: 1 file (623 lines) — Wails v3 desktop application service bridge.
   - `go/deps/`: 3 files (`safe`: 1, `httpx`: 2) — standalone utility packages bundled with the extraction.
   - `go/tests/`: 27 files (~5,000 lines) — test suite using `httptest.Server` and SQLite fixtures.
2. **Current Standalone Build Status**: **Non-buildable**.
   - No `go.mod` exists in the repository.
   - 19 of the 58 Go files import 8 unbundled 1Scout platform packages under `onescout/desktop/internal/...`.
   - External dependencies (`golang.org/x/net/html`, `github.com/temoto/robotstxt`, `github.com/chromedp/chromedp`, `modernc.org/sqlite`, `github.com/wailsapp/wails/v3`) are undeclared in a module file.
3. **Core Acquisition Isolation**:
   - The acquisition engine (`go/engine/` and core `go/storage/`) is remarkably independent of 1Scout platform logic.
   - 11 of the 15 `go/engine/` files require **only standard library** plus 3 standard external modules (`html`, `robotstxt`, `chromedp`).
   - The heavy 1Scout platform coupling is concentrated in `go/app-glue/service.go` (8 platform imports + Wails v3) and `go/pagespeed/` (4 platform imports).
   - Neither `go/app-glue/` nor `go/pagespeed/` is required for the standalone web crawler acquisition core.
4. **Toolchain & CGO Requirements**:
   - **Minimum Go Version**: **Go 1.22+** is strictly required, proven by standard library imports of `math/rand/v2` in `go/deps/httpx/httpx.go` and `log/slog` (Go 1.21+).
   - **Zero CGO**: The codebase contains zero `import "C"` calls. The tests use `modernc.org/sqlite` (pure Go). CGO is not required.
   - **Zero Build Tags**: No `//go:build` or `// +build` directives exist anywhere in the code.
5. **Flat-Package Coupling**:
   - All 55 files in `engine/`, `storage/`, `pagespeed/`, `app-glue/`, and `tests/` declare `package sitecrawl`.
   - They extensively share unexported functions, structs, and variables across directory boundaries. Fracturing them into separate Go subpackages during initial extraction is hazardous and must not be attempted in Checkpoints A2/A3.

---

## 2. Comprehensive File Inventory

### 2.1 `go/engine/` (15 Files)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Coupling | Standalone Feasibility | Checkpoint | Risk |
|---|---|---|---|---|---|---|---|
| [`exclusions.go`](../go/engine/exclusions.go) | `sitecrawl` | None | None | `shouldExclude`, `defaultExclusions` (used by `crawler.go`, `types.go`) | Portable as-is (100% stdlib) | A3 | Low |
| [`extract.go`](../go/engine/extract.go) | `sitecrawl` | `golang.org/x/net/html` | None | `extractDocument`, `extractedDoc`, `SchemaItem`, `Image`, `Hreflang`, `LinkEdge` (used by `page.go`, `crawler.go`) | Portable with `x/net/html` | A3 | Low |
| [`fetch.go`](../go/engine/fetch.go) | `sitecrawl` | None | `core/httpx` | `newFetcher`, `fetcher`, `fetchResult`, `fetchOutcome` (used by `crawler.go`, `page.go`) | Needs import path redirect to local `httpx` | A3 | Low |
| [`frontier.go`](../go/engine/frontier.go) | `sitecrawl` | None | None | `newFrontier`, `frontier`, `frontierItem`, `admit`, `peekReady`, `snapshot`, `restore` (used by `crawler.go`, `runs.go`) | Portable as-is (100% stdlib) | A3 | Low |
| [`issues.go`](../go/engine/issues.go) | `sitecrawl` | None | None | `evaluate(p *Page)`, `indexabilityOf(p *Page)`, `issue`, `issueCatalog`, `severityOf` (used by `crawler.go`, `finalize.go`, `persist.go`). Method `(s *Service) IssueCatalog()` attaches to `Service` | Portable as-is. Legacy compatibility only; not Audit V1 authority | A3 | Low |
| [`links.go`](../go/engine/links.go) | `sitecrawl` | None | None | `collectLinks`, `linkEdge`, `classifyRel` (used by `crawler.go`) | Portable as-is (100% stdlib) | A3 | Low |
| [`normalize.go`](../go/engine/normalize.go) | `sitecrawl` | None | None | `normalizeURL`, `frontierKey`, `isSameSite`, `rootDomain`, `stripTracking` (used across all crawler modules) | Portable as-is (100% stdlib) | A3 | Low |
| [`page.go`](../go/engine/page.go) | `sitecrawl` | None | None | `buildPage(item, res, doc)`, `Page` struct methods (used by `crawler.go`, `persist.go`) | Portable as-is (100% stdlib) | A3 | Low |
| [`politeness.go`](../go/engine/politeness.go) | `sitecrawl` | None | None | `newHostGate`, `hostGate`, `onSuccess`, `onRetryable`, `onTerminal` (used by `crawler.go`) | Portable as-is (100% stdlib) | A3 | Low |
| [`render.go`](../go/engine/render.go) | `sitecrawl` | `github.com/chromedp/chromedp` | None | `findBrowser`, `renderPage`, `renderResult` (used by `crawler.go`) | Portable with `chromedp`; requires local Chrome/Edge binary | A3 | Medium |
| [`robots.go`](../go/engine/robots.go) | `sitecrawl` | `github.com/temoto/robotstxt` | None | `newRobots`, `robotsCache`, `Check`, `CrawlDelay`, `Sitemaps` (used by `crawler.go`) | Portable with `robotstxt` | A3 | Low |
| [`similarity.go`](../go/engine/similarity.go) | `sitecrawl` | None | None | `simHash64`, `seqRatio` (used by `storage/duplicates.go`) | Portable as-is (100% stdlib) | A3 | Low |
| [`sitemap.go`](../go/engine/sitemap.go) | `sitecrawl` | None | `core/safe` | `discoverSitemaps`, `parseSitemap`, `sitemapEntry` (used by `crawler.go`) | Needs import path redirect to local `safe` | A3 | Low |
| [`types.go`](../go/engine/types.go) | `sitecrawl` | None | `core/runs` | Core domain types: `Options`, `Page`, `ProgressEvent`, `RunStateEvent`, crawl modes, stop reasons | Needs `runs.State*` constants decoupled or aliased | A3 | Low |
| [`useragents.go`](../go/engine/useragents.go) | `sitecrawl` | None | None | `defaultUserAgent`, `UserAgentPresets` (used by `types.go`, `crawler.go`) | Portable as-is (100% stdlib) | A3 | Low |

---

### 2.2 `go/storage/` (9 Files)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Coupling | Standalone Feasibility | Checkpoint | Risk |
|---|---|---|---|---|---|---|---|
| [`crawler.go`](../go/storage/crawler.go) | `sitecrawl` | None | `core/httpx`, `core/safe` | `coordinator`, `newCoordinator`, `prepare`, `loop`, `doOne`, `absorb`, `flush`, `checkpoint`, `finalize` (orchestrates all engine modules) | Needs `httpx` & `safe` import redirects. Takes pure `*sql.DB` | A3 | Medium |
| [`persist.go`](../go/storage/persist.go) | `sitecrawl` | None | None | `persistBuffer`, `writeBatch`, `writePage`, `writeLinks`, `writeIssues` (called by `crawler.go`) | Portable as-is (100% stdlib + `database/sql`) | A3 | Low |
| [`runs.go`](../go/storage/runs.go) | `sitecrawl` | None | `core/runs`, `core/schema` | `schemaStmts`, `initRun`, `finishRun`, `saveFrontier`, `loadFrontier`, `loadSeen`, `cleanupDeadRuns`, `ensureFTS` | Needs `runs.Now()` and `schema.Migrate()` replacement | A3 | Medium |
| [`finalize.go`](../go/storage/finalize.go) | `sitecrawl` | None | None | `c.finalize(ctx)`, `finalizeCodes`, `finalizeInlinks`, `finalizeOrphans`, `finalizeCanonicals`, `finalizeHreflang`, `clearIssues` | Portable as-is. Legacy issue generation logic | A3 | Low |
| [`duplicates.go`](../go/storage/duplicates.go) | `sitecrawl` | None | None | `findExactDuplicates`, `findNearDuplicates` (calls `simHash64`, `seqRatio` from `engine/similarity.go`) | Portable as-is (100% stdlib + `database/sql`) | A3 | Low |
| [`query.go`](../go/storage/query.go) | `sitecrawl` | None | None | `queryPages`, `countPages`, `queryInlinks`, `queryOutlinks`, `safeOrderBy` | Portable as-is (100% stdlib + `database/sql`) | A3 | Low |
| [`export.go`](../go/storage/export.go) | `sitecrawl` | None | `core/runs` | `exportPagesCSV`, `exportPagesJSON`, `exportPagesXML` (calls `runs.BOMUTF8`, `runs.CSVGuard`) | Needs local constants/helpers for BOM and CSV escaping | A3 | Low |
| [`graph.go`](../go/storage/graph.go) | `sitecrawl` | None | None | `buildGraphData(db, runID, maxNodes)` (generates Visualization graph) | Portable as-is | A3 | Low |
| [`agent_local.go`](../go/storage/agent_local.go) | `sitecrawl` | None | None | `LookupLocalPages` (read-only search query for 1Scout AI Writer) | Portable as-is (can be preserved or deferred) | A3 / Defer | Low |

---

### 2.3 `go/pagespeed/` (3 Files)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Coupling | Standalone Feasibility | Checkpoint | Risk |
|---|---|---|---|---|---|---|---|
| [`pagespeed.go`](../go/pagespeed/pagespeed.go) | `sitecrawl` | None | `core/credset`, `core/httpx`, `core/safe`, `core/workspace` | `psiClient`, `measureURL`, `persistPSIResult`, `runPSIJob` | High platform coupling (`credset`, `workspace.WithTx`). Non-core feature | Defer | High (if included now) / Low (if deferred) |
| [`pagespeed_pump.go`](../go/pagespeed/pagespeed_pump.go) | `sitecrawl` | None | `core/safe` | `psiPump`, `newPSIPump`, `start`, `stop` (called by `crawler.go` if PSI enabled) | Coupled to `pagespeed.go` | Defer | Low |
| [`pagespeed_opps.go`](../go/pagespeed/pagespeed_opps.go) | `sitecrawl` | None | None | `queryOpportunities`, `oppsRow` | Depends on PSI database tables | Defer | Low |

---

### 2.4 `go/app-glue/` (1 File)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Coupling | Standalone Feasibility | Checkpoint | Risk |
|---|---|---|---|---|---|---|---|
| [`service.go`](../go/app-glue/service.go) | `sitecrawl` | `github.com/wailsapp/wails/v3` | `core/credset`, `core/httpx`, `core/jobs`, `core/license`, `core/runs`, `core/schema`, `core/workspace`, `tools` | Connects UI to `newCoordinator`, `initRun`, `DefaultOptions`, etc. Implements Wails service methods | Severe desktop platform coupling. Not required for standalone engine/CLI | Defer / Exclude from Core | High (if included now) / Zero (if excluded from core) |

---

### 2.5 `go/deps/` (3 Files)

| File | Package | External Imports | 1Scout Imports | Role / Purpose | Standalone Feasibility | Checkpoint | Risk |
|---|---|---|---|---|---|---|---|
| [`safe/safe.go`](../go/deps/safe/safe.go) | `safe` | None | None | Panic recovery utilities: `safe.Go`, `safe.Do`, `safe.Call` | Portable as-is (100% stdlib: `log/slog`, `runtime/debug`) | A2 / A3 | Low |
| [`httpx/httpx.go`](../go/deps/httpx/httpx.go) | `httpx` | None | None | Hardened HTTP client: timeouts, proxy hook, User-Agent pool, DNS filtering | Portable as-is (100% stdlib, requires Go 1.22+ for `math/rand/v2`) | A2 / A3 | Low |
| [`httpx/delivery.go`](../go/deps/httpx/delivery.go) | `httpx` | None | None | Custom dialer preventing private network SSRF/rebinding | Portable as-is (100% stdlib) | A2 / A3 | Low |

---

### 2.6 `go/tests/` (27 Files)

| Test File | Package | External Imports | 1Scout Imports | Focus Area / Coverage | Dependencies / Prerequisites | Checkpoint |
|---|---|---|---|---|---|---|
| [`agent_local_test.go`](../go/tests/agent_local_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | AST import validation, local HTML-only search | Pure SQLite in-memory | A3 |
| [`cancel_test.go`](../go/tests/cancel_test.go) | `sitecrawl` | None | `core/runs` | Crawl cancellation lifecycle | Uses `s.Jobs.Cancel`, `runs.Terminal` via `newTestService` | A3 (with test harness) |
| [`crawler_test.go`](../go/tests/crawler_test.go) | `sitecrawl` | None | `core/runs` | Core fixture crawl: facets, status codes, canonicals, robots, duplicates | Uses `newTestService`, `runs.BOMUTF8` | A3 (with test harness) |
| [`credentials_test.go`](../go/tests/credentials_test.go) | `sitecrawl` | None | `core/credset`, `core/httpx` | PageSpeed API key storage encryption/vault | Coupled to PageSpeed / `credset` | Defer |
| [`customheaders_test.go`](../go/tests/customheaders_test.go) | `sitecrawl` | None | None | Custom HTTP headers forwarding during crawl | Uses `newTestService` | A3 |
| [`deferred_test.go`](../go/tests/deferred_test.go) | `sitecrawl` | None | None | 429 Too Many Requests & 503 retry/deferral loop | Uses `newTestService` | A3 |
| [`extract_test.go`](../go/tests/extract_test.go) | `sitecrawl` | None | None | HTML extraction: title, meta, canonical, hreflang, schema | Pure unit test (100% stdlib) | A3 |
| [`fixture_test.go`](../go/tests/fixture_test.go) | `sitecrawl` | None | None | Synthetic site fixture with redirect chains, loops, canonicals | Helper file for all crawler tests | A3 |
| [`frontier_cap_test.go`](../go/tests/frontier_cap_test.go) | `sitecrawl` | None | None | `MaxURLs` admission boundary enforcement | Pure unit test (100% stdlib) | A3 |
| [`frontier_test.go`](../go/tests/frontier_test.go) | `sitecrawl` | None | None | FIFO queue, BFS depth order, tracking param stripping | Pure unit test (100% stdlib) | A3 |
| [`fts_trigger_test.go`](../go/tests/fts_trigger_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | SQLite FTS5 index update triggers | Skips if SQLite build lacks FTS5 | A3 |
| [`history_test.go`](../go/tests/history_test.go) | `sitecrawl` | None | `core/jobs`, `core/workspace`, `testutil` | Test harness definition (`newTestService`), run history queries | Core test helper definition | A3 (with test harness) |
| [`inspectdb_test.go`](../go/tests/inspectdb_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | Manual crawl DB inspection utility | Environment-gated (`CRAWL_DB=<path>`) | A3 |
| [`inspectopts_test.go`](../go/tests/inspectopts_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | Manual options inspection utility | Environment-gated (`CRAWL_DB=<path>`) | A3 |
| [`live_test.go`](../go/tests/live_test.go) | `sitecrawl` | None | `core/jobs`, `core/workspace` | Live web crawl test against external sites | Environment-gated (`CRAWL_LIVE=<url>`) | Defer / A3 |
| [`pagespeed_test.go`](../go/tests/pagespeed_test.go) | `sitecrawl` | None | `core/credset`, `core/httpx` | PageSpeed pump, concurrency pacing, error handling | Coupled to PageSpeed | Defer |
| [`pause_test.go`](../go/tests/pause_test.go) | `sitecrawl` | None | None | Pause and resume lifecycle, frontier preservation | Uses `newTestService` | A3 |
| [`perf_test.go`](../go/tests/perf_test.go) | `sitecrawl` | None | None | High-concurrency synthetic crawl benchmark | Environment-gated (`CRAWL_PERF=1`) | A3 |
| [`politeness_test.go`](../go/tests/politeness_test.go) | `sitecrawl` | None | None | Per-host pacing, backoff upon error, `Crawl-delay` | Uses `newTestService` | A3 |
| [`psi_live_test.go`](../go/tests/psi_live_test.go) | `sitecrawl` | None | None | Live Google PSI API request test | Environment-gated (`PSI_LIVE_KEY=<key>`) | Defer |
| [`query_test.go`](../go/tests/query_test.go) | `sitecrawl` | None | None | Grid query SQL construction and SQL injection safety | Pure unit test (100% stdlib) | A3 |
| [`render_test.go`](../go/tests/render_test.go) | `sitecrawl` | None | None | Chrome/Edge headless rendering of JS content | Skips if no local Chrome/Edge binary | A3 |
| [`resume_ids_test.go`](../go/tests/resume_ids_test.go) | `sitecrawl` | None | None | URL ID stability across crawl pause and restart | Uses `newTestService` | A3 |
| [`schema_golden_test.go`](../go/tests/schema_golden_test.go) | `sitecrawl` | None | `testutil` | SQLite schema migration regression test against golden file | Uses `testutil.SchemaGolden` | A3 (with adapter) |
| [`seedredirect_test.go`](../go/tests/seedredirect_test.go) | `sitecrawl` | None | None | Initial seed URL 301/302 redirect resolution | Uses `newTestService` | A3 |
| [`similarity_test.go`](../go/tests/similarity_test.go) | `sitecrawl` | None | None | SimHash duplicate detection and content ratio tests | Pure unit test (100% stdlib) | A3 |
| [`sitemap_test.go`](../go/tests/sitemap_test.go) | `sitecrawl` | None | None | Sitemap index, gzip sitemaps, invalid XML tolerance | Pure unit test (100% stdlib) | A3 |

---

## 3. 1Scout Platform Dependency Analysis

The table below catalogs every reference to unbundled 1Scout platform packages across the codebase:

| 1Scout Package | Importing Files | Exact Symbols / Types Used | Purpose in SiteCrawl | Core Crawler Need? | Recommended Strategy |
|---|---|---|---|---|---|
| **`core/runs`** | `engine/types.go`, `storage/runs.go`, `storage/export.go`, `app-glue/service.go`, `tests/cancel_test.go`, `tests/crawler_test.go` | `StateRunning`, `StatePaused`, `StateCompleted`, `StateCancelled`, `StateStopped`, `StateFailed`, `StateInterrupted`, `Now()`, `BOMUTF8`, `CSVGuard()`, `Gate`, `Terminal()` | Run status string constants, RFC3339 timestamps, CSV export UTF-8 BOM and formula guard, terminal state check | **No** (trivial utility concepts) | **Replace by minimal adapter / local definitions**. Define string constants, `time.Now().UTC().Format(time.RFC3339)`, and CSV helpers in `internal/sitecrawl/` without importing `core/runs`. |
| **`core/schema`** | `storage/runs.go`, `app-glue/service.go` | `schema.Migrate(db, "sitecrawl", stmts)`, `schema.Ready` | Executes sequential SQLite DDL migration statements and records versions in a migration table | **Yes** (database schema init) | **Replace by minimal standalone adapter**. A 25-line SQLite migration helper that executes the existing `schemaStmts` array and records applied hashes/versions in a `_schema_migrations` table. |
| **`core/workspace`** | `app-glue/service.go`, `pagespeed/pagespeed.go`, `tests/history_test.go`, `tests/live_test.go` | `workspace.Manager`, `workspace.NewManager()`, `workspace.WithTx()` | Multi-workspace file management, path resolution, and transaction retry wrapper | **No** (standalone crawler operates on standard `*sql.DB` or local SQLite file) | **Exclude from Core / Replace with local `*sql.DB` opener**. Standalone core only needs standard `sql.Open("sqlite", path)`. For tests, a minimal `openTestDB(t)` replaces `workspace.NewManager()`. |
| **`core/jobs`** | `app-glue/service.go`, `tests/cancel_test.go`, `tests/history_test.go`, `tests/live_test.go` | `jobs.Runner`, `jobs.NewRunner()`, `Jobs.Start()`, `Jobs.Emit()`, `Jobs.Cancel()` | Background goroutine supervisor and desktop UI event emitter | **No** (desktop UI background task framework) | **Exclude from Core**. The crawler coordinator already manages its own worker pool and cancellation via standard Go channels and `context.Context`. |
| **`core/credset`** | `app-glue/service.go`, `pagespeed/pagespeed.go`, `tests/credentials_test.go`, `tests/pagespeed_test.go` | `credset.Store`, `credset.First()`, `credset.Add()`, `credset.Save()` | Encrypted desktop vault for Google PageSpeed API keys | **No** (desktop secrets storage for non-core PSI feature) | **Exclude from Core / Defer**. Unneeded for web crawling or technical SEO audit. PageSpeed integration is deferred. |
| **`core/license`** | `app-glue/service.go` | `license.Gate` | Software license verification gate | **No** (commercial licensing) | **Exclude completely**. Has zero function in the audit tool. |
| **`tools`** | `app-glue/service.go` | `tools.Register()`, `tools.Factory`, `tools.Starter` | 1Scout desktop plugin registry | **No** (desktop app plugin mechanism) | **Exclude completely**. Standalone module will use CLI or Go programmatic API. |
| **`testutil`** | `tests/history_test.go`, `tests/schema_golden_test.go` | `testutil.RedirectConfigDir(t)`, `testutil.SchemaGolden(t, "sitecrawl", stmts)` | Test environment config redirection and schema golden file validator | **No** | **Replace by local test helpers**. Replace `RedirectConfigDir` with `t.TempDir()`; adapt `SchemaGolden` locally or verify DDL directly. |
| **`core/safe`** *(bundled)* | `engine/sitemap.go`, `storage/crawler.go`, `pagespeed/pagespeed.go`, `pagespeed_pump.go` | `safe.Do()`, `safe.Call()`, `safe.Go()` | Per-goroutine panic recovery | **Yes** | **Preserved**. Code is already present in `go/deps/safe/safe.go`. Only needs package import path updated in Checkpoint A3. |
| **`core/httpx`** *(bundled)* | `engine/fetch.go`, `storage/crawler.go`, `pagespeed/pagespeed.go`, `app-glue/service.go`, `tests/credentials_test.go`, `tests/pagespeed_test.go` | `httpx.NewFiltered()`, `httpx.ProxyFunc`, `httpx.IsLocalName`, `httpx.AllowPrivate` | Hardened HTTP transport, proxy rotation, SSRF dialer | **Yes** | **Preserved**. Code is already present in `go/deps/httpx/`. Only needs package import path updated in Checkpoint A3. |

---

## 4. Cross-File Coupling & Flat-Package Analysis

All 55 Go files in `engine/`, `storage/`, `pagespeed/`, `app-glue/`, and `tests/` originally resided in a single flat package `desktop/internal/tools/sitecrawl/`. The current directory layout is purely an aesthetic partition in documentation.

### 4.1 Concrete Evidence of Package-Private Coupling Across Directories
The Go compiler enforces that unexported symbols (identifiers starting with lowercase letters) can only be accessed across files in the **same directory** belonging to the **same package**.

The extracted code relies heavily on unexported cross-directory calls:
1. **Coordinator to Engine**:
   - In `storage/crawler.go` (line 675):
     ```go
     p := buildPage(r.item, r.res, r.doc, c.opts)
     ```
     `buildPage` is unexported in `engine/page.go`. It consumes `frontierItem` (unexported in `engine/frontier.go`), `fetchResult` (unexported in `engine/fetch.go`), and `extractedDoc` (unexported in `engine/extract.go`).
   - In `storage/crawler.go` (lines 116–120):
     ```go
     type coordinator struct {
         frontier   *frontier     // unexported in engine/frontier.go
         hosts      *hostGate     // unexported in engine/politeness.go
         fetch      *fetcher      // unexported in engine/fetch.go
         robots     *robotsCache  // unexported in engine/robots.go
         exclusions *exclusionSet // unexported in engine/exclusions.go
         rend       renderer      // unexported in engine/render.go
     ```
2. **Coordinator to Storage**:
   - In `storage/crawler.go`:
     - Calls `persistBuffer` and `writeBatch` (unexported in `storage/persist.go`).
     - Calls `checkpoint`, `saveFrontier`, `loadFrontier`, `loadSeen`, `ensureFTS` (unexported in `storage/runs.go`).
     - Calls `c.finalize(ctx)`, `finalizeCodes`, `clearIssues` (unexported in `storage/finalize.go`).
3. **Storage to Engine (Duplicates)**:
   - In `storage/duplicates.go` (lines 178, 252):
     Calls `simHash64` and `seqRatio` (both unexported in `engine/similarity.go`).
4. **Coordinator to Issues (Legacy)**:
   - In `storage/crawler.go` (line 676):
     Calls `evaluate(p)` and `indexabilityOf(p)` (unexported in `engine/issues.go`).
5. **Tests to Engine & Storage**:
   - Test files directly invoke unexported types and constructors: `newFrontier()`, `frontierKey()`, `normalizeURL()`, `newHostGate()`, `readDB()`, `parseSitemap()`, `findBrowser()`.

### 4.2 Invariant for Checkpoints A2 and A3
Attempting to restructure `engine/`, `storage/`, etc., into separate Go packages (`crawler/frontier`, `crawler/fetch`, `crawler/storage`) during standalone extraction would require:
- Exporting hundreds of internal fields, methods, and types.
- Rewriting method signatures and call sites across 55 files.
- Introducing circular dependency cycles (e.g. `coordinator` needs `page`, `page` needs `fetchResult`, `fetcher` needs options, `finalize` needs `coordinator`).

**Mandatory Conclusion**: The first buildable package in Checkpoint A3 must reconstruct the single, unified flat package:
```text
internal/sitecrawl/
```
preserving package-private cohesion and zero runtime disruption.

---

## 5. Build Metadata & Runtime Constraints

### 5.1 Build Tags
- **Directives Found**: None.
- Verified across all 58 files: exactly **zero** `//go:build` or `// +build` directives exist.

### 5.2 Generated Code & Wails Bindings
- **Directives Found**: Zero `//go:generate` directives exist in Go source files.
- **Wails Bindings**: Files in `ui/bindings/` (`models.ts`, `service.ts`) are generated TypeScript bindings produced by `wails3 generate bindings` from `app-glue/service.go`. They affect frontend compilation only and do not impact standalone Go engine buildability.

### 5.3 CGO Sensitivity
- **Directives Found**: Zero `import "C"` calls exist.
- **Database Driver**: The test suite imports `modernc.org/sqlite`. This is a pure-Go transpiled SQLite implementation. It does **not** require CGO.
- **Conclusion**: The standalone crawler can be compiled and tested with `CGO_ENABLED=0` across all supported platforms (Windows, Linux, macOS).

### 5.4 External Go Modules
When Checkpoint A2 creates `go.mod`, only the following external modules are required for the acquisition core:
1. `golang.org/x/net` (specifically `golang.org/x/net/html`) — used for HTML document tokenization.
2. `github.com/temoto/robotstxt` — used for robots.txt parsing and directive matching.
3. `github.com/chromedp/chromedp` — used for optional browser rendering.
4. `modernc.org/sqlite` — used for pure-Go SQLite persistence and testing.

*(Note: `github.com/wailsapp/wails/v3` is only needed if `app-glue/service.go` is retained. It should be excluded from the acquisition core).*

### 5.5 Runtime & Browser Requirements
- **JavaScript Rendering**: `go/engine/render.go` does not bundle Chromium; it searches for an installed system browser:
  - Windows: Google Chrome, Microsoft Edge, Brave.
  - macOS: Google Chrome, Microsoft Edge, Brave, Chromium.
  - Linux: Google Chrome, Chromium, Brave.
- When no browser is present, headless rendering tests skip gracefully (`render_test.go` line 16: `t.Skip("no Chrome/Edge installed")`). The crawler itself functions normally for HTTP/HTML acquisition without a browser.

### 5.6 Environment-Gated Tests
The following tests in `go/tests/` conditionally skip unless specific environment variables are set:
- `CRAWL_LIVE=<url>`: in `go/tests/live_test.go` (crawls an external live URL).
- `PSI_LIVE_KEY=<key>`: in `go/tests/psi_live_test.go` (calls live Google PageSpeed API).
- `CRAWL_PERF=1`: in `go/tests/perf_test.go` (runs long-running high-concurrency benchmarks).
- `CRAWL_DB=<path>`: in `go/tests/inspectdb_test.go` and `go/tests/inspectopts_test.go` (opens manual workspace DB files).

### 5.7 Go Version Constraint
- **Minimum Proven Version**: **Go 1.22**.
- **Proof**: `go/deps/httpx/httpx.go` (line 11) imports standard library `"math/rand/v2"`, which was introduced in Go 1.22. Additionally, `log/slog` (used in `safe.go`, `crawler.go`, `runs.go`) requires at least Go 1.21.
- `go.mod` in Checkpoint A2 should specify `go 1.22` or `go 1.23`.

---

## 6. First Buildable Package Proposal (Checkpoint A3)

### 6.1 Package Architecture
The first buildable package will recreate the original flat acquisition core inside:
```text
internal/sitecrawl/
```
accompanied by a lightweight internal platform utility package for `safe` and `httpx`:
```text
internal/platform/safe/
internal/platform/httpx/
```
*(or kept as internal subpackages within `internal/sitecrawl/deps/`)*.

### 6.2 Classification of Files for Checkpoint A3

#### Category 1: Required Now (Core Acquisition Engine — 22 Files)
These files represent the actual crawling, extraction, and persistence engine and must enter `internal/sitecrawl/` immediately:
- `go/engine/normalize.go` — URL identity, tracking param stripping, domain scoping.
- `go/engine/frontier.go` — BFS queue, admission gates, depth tracking.
- `go/engine/politeness.go` — Per-host concurrency, delay, backoff pacing.
- `go/engine/robots.go` — Robots.txt fetching, parsing, caching.
- `go/engine/sitemap.go` — Sitemap discovery and XML parsing.
- `go/engine/fetch.go` — HTTP client, redirect traversal, transport error handling.
- `go/engine/extract.go` — HTML tokenizer, meta/links/canonical/schema extraction.
- `go/engine/page.go` — Document and page record assembly.
- `go/engine/links.go` — Link edge collection and relationship classification.
- `go/engine/render.go` — Headless browser management and DOM extraction.
- `go/engine/exclusions.go` — URL pattern exclusions.
- `go/engine/similarity.go` — SimHash and duplicate content algorithms.
- `go/engine/types.go` — Core types (`Options`, `Page`, events, crawl modes).
- `go/engine/useragents.go` — Preset crawler user-agents.
- `go/storage/crawler.go` — Coordinator, crawl loop, worker orchestration.
- `go/storage/persist.go` — Batch SQLite persistence of URLs, pages, and links.
- `go/storage/runs.go` — SQLite schema DDL, run records, frontier save/load.
- `go/storage/duplicates.go` — Content duplication detection queries.
- `go/storage/query.go` — Crawl results querying, filtering, and tab sorting.
- `go/storage/export.go` — CSV/JSON/XML data export.
- `go/deps/safe/safe.go` — Panic recovery helper.
- `go/deps/httpx/httpx.go` & `delivery.go` — HTTP client and SSRF protection.

#### Category 2: Legacy Compatibility Only (2 Files)
These files are preserved during extraction strictly so existing crawler behavior and test assertions remain functional, but they hold **zero authority** over Audit V1:
- `go/engine/issues.go` — 55 legacy kebab-case issue rules and `indexabilityOf`.
- `go/storage/finalize.go` — Post-crawl legacy issue generation (broken images, orphans, etc.).

#### Category 3: Optional / Defer (4 Files)
These files provide auxiliary functionality that can be excluded from the first buildable acquisition core without harming web crawling capabilities:
- `go/storage/agent_local.go` — AI Writer local lookup query (optional; can be included if desired as it only uses `database/sql`).
- `go/storage/graph.go` — Visualization data builder (optional; uses `database/sql` only).
- `go/pagespeed/pagespeed.go` — Google PageSpeed Insights API client (deferred).
- `go/pagespeed/pagespeed_pump.go` — Background PSI measurement pump (deferred).
- `go/pagespeed/pagespeed_opps.go` — PageSpeed opportunities rollup (deferred).

#### Category 4: Exclude from Standalone Core (1 File)
- `go/app-glue/service.go` — Wails v3 desktop application bridge. Highly coupled to 8 unbundled 1Scout platform packages and the desktop UI. Replaced in standalone mode by a lightweight CLI entrypoint (`cmd/sitecrawl-dev/main.go`).

#### Category 5: Test Suite (27 Files)
- **20 Tests Portable Immediately**: `agent_local_test.go`, `customheaders_test.go`, `deferred_test.go`, `extract_test.go`, `fixture_test.go`, `frontier_cap_test.go`, `frontier_test.go`, `fts_trigger_test.go`, `inspectdb_test.go`, `inspectopts_test.go`, `pause_test.go`, `perf_test.go`, `politeness_test.go`, `query_test.go`, `render_test.go`, `resume_ids_test.go`, `seedredirect_test.go`, `similarity_test.go`, `sitemap_test.go`.
- **4 Tests Requiring Minimal Test Harness**: `cancel_test.go`, `crawler_test.go`, `history_test.go`, `schema_golden_test.go`.
- **3 Tests Deferred (PageSpeed)**: `credentials_test.go`, `pagespeed_test.go`, `psi_live_test.go`.

---

## 7. Required Conclusions

### 1. What prevents the extracted repository from building standalone today?
- Absence of a `go.mod` file defining the module boundary and dependency versions.
- Import statements referencing unbundled 1Scout platform packages (`onescout/desktop/internal/...`).
- Missing external Go module declarations for `golang.org/x/net`, `github.com/temoto/robotstxt`, `github.com/chromedp/chromedp`, and `modernc.org/sqlite`.

### 2. Which files form the smallest useful acquisition core?
- The 22 files identified in **Category 1** (engine mechanics + storage coordinator + bundled deps) plus the 2 legacy compatibility files in **Category 2** (`issues.go`, `finalize.go`). This 24-file bundle provides full URL discovery, politeness, robots/sitemap parsing, HTTP fetching, JS rendering, HTML extraction, link graph building, and SQLite storage.

### 3. Which 1Scout dependencies block that core?
Only three 1Scout packages touch the 24 core files:
1. `core/runs`: Used in `types.go`, `runs.go`, and `export.go` for status strings, timestamps, and CSV formatting.
2. `core/schema`: Used in `storage/runs.go` for `schema.Migrate()`.
3. `core/httpx` & `core/safe`: Imported using full `onescout/...` paths, but their source code is **already present** in `go/deps/`.

*(All other platform dependencies—`core/workspace`, `core/jobs`, `core/credset`, `core/license`, `tools`—belong to `service.go` and `pagespeed/`, which are outside the acquisition core).*

### 4. What minimal standalone adapters will eventually be required?
To make the 24 core files build cleanly in Checkpoint A3, only three trivial shims are required:
1. **Local Run Constants & Helpers**: Replace `runs.State*` constants in `types.go` with native string constants, replace `runs.Now()` with `time.Now().UTC().Format(time.RFC3339)`, and define `BOMUTF8` / formula escaping directly in `export.go`.
2. **Minimal SQLite Schema Migrator**: Implement a lightweight standalone function (~25 lines of Go) in `runs.go` that iterates over `schemaStmts` and applies unapplied migrations against `*sql.DB`.
3. **Internal Module Paths**: Update import paths for `safe` and `httpx` from `onescout/desktop/internal/core/...` to the local standalone module path.

### 5. Which dependencies/features can be deferred?
- **Google PageSpeed Insights (`pagespeed/`)**: Deferred entirely. It is not part of crawling or core SEO audit evidence.
- **Wails Desktop Service (`app-glue/service.go`)**: Excluded from the standalone acquisition core.
- **Platform Licensing & Jobs (`license`, `jobs`, `tools`, `credset`)**: Excluded entirely.

### 6. Which package-private dependencies make early package splitting unsafe?
The tight coupling between `storage/crawler.go` (the coordinator) and `engine/` types (`frontierItem`, `fetchResult`, `buildPage`, `hostGate`, `robotsCache`, `simHash64`, `seqRatio`, `evaluate`) relies completely on package-private visibility. Any premature package split would require mass-exporting types and refactoring signatures, introducing significant regression risks.

### 7. What exact scope should Checkpoint A2 (`go.mod`) contain?
Checkpoint A2 must:
- Initialize `go.mod` with a chosen module path (e.g. `truthrive/technical-seo-audit`).
- Declare `go 1.22`.
- Require only the verified external packages:
  - `golang.org/x/net`
  - `github.com/temoto/robotstxt`
  - `github.com/chromedp/chromedp`
  - `modernc.org/sqlite`
- Verify with `go list ./...`.

### 8. What should explicitly NOT be done in A2/A3?
- Do NOT implement any of the 47 `AR-*` Audit V1 rules.
- Do NOT rewrite or redesign the crawler loop, frontier, or HTTP fetching.
- Do NOT redesign the `Page` struct.
- Do NOT split `sitecrawl` into multiple fine-grained subpackages.
- Do NOT port or reimplement 1Scout desktop application frameworks (`jobs.Runner`, `license.Gate`, `tools.Register`, `credset.Vault`, Wails v3).
- Do NOT delete or modify legacy `issues.go` rules prematurely (preserve them as reference/compatibility behavior).
