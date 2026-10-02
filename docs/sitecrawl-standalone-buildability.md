# SiteCrawl Standalone Buildability Inventory (Checkpoint A1)

- **Status:** Completed
- **Checkpoint:** A1 — Buildability Inventory
- **Upstream Map:** [`sitecrawl-standalone-migration-map.md`](sitecrawl-standalone-migration-map.md)
- **Baseline Git Tag:** `sitecrawl-pre-audit-v1` (`82c610d433203374473e1ea76edf4e6c165a3161`)
- **Nature of Work:** Strictly read-only analysis. No production code, tests, `go.mod`, or configuration files are modified during Checkpoint A1.

---

## 1. Executive Summary

This inventory evaluates the compile-time reality, dependency graph, and standalone buildability of the extracted SiteCrawl module from 1Scout Marketing (commit `2e2cbff`, 2026-09-25) in its current state within `truthrive/technical-seo-audit`.

### Key Findings
1. **Reconciled File Count**: Exactly **58 Go source files** across 6 directories:
   - `go/engine/`: 15 files (~5,000 lines) — crawler mechanics and data extraction.
   - `go/storage/`: 9 files (~3,900 lines) — SQLite persistence, query, export, run lifecycle, and post-crawl finalization.
   - `go/pagespeed/`: 3 files (~1,050 lines) — PageSpeed Insights API integration and measurement pump.
   - `go/app-glue/`: 1 file (623 lines) — Wails v3 desktop application service bridge.
   - `go/deps/`: 3 files (`safe`: 1, `httpx`: 2) — standalone utility packages bundled with the extraction.
   - `go/tests/`: 27 files (~5,000 lines) — test suite using `httptest.Server` and SQLite fixtures.
2. **Current Standalone Build Status**: **Non-buildable**.
   - No `go.mod` exists in the repository.
   - 16 of the 58 files import from `onescout/...` (12 files import unbundled 1Scout packages, 4 import only bundled `core/httpx` or `core/safe`).
   - 42 of the 58 files contain zero `onescout/...` imports.
3. **Compile-Time Dependency Closures**:
   - The original implementation was a single flat package `package sitecrawl`. Files across directories share unexported structs, functions, and receiver types.
   - Crucially, files cannot be arbitrarily moved or excluded without breaking compile-time symbol closure:
     - `storage/crawler.go` relies on symbols in `go/pagespeed/` (`psiPump`, `PSIResult`, `newPSIPump`, `psiClient`, `savePSI`).
     - `engine/issues.go`, `storage/query.go`, `storage/export.go`, `storage/graph.go`, and `go/pagespeed/*.go` define methods on `(s *Service)`, which is declared in `go/app-glue/service.go`.
     - `engine/page.go` relies on `nowStamp()` (in `storage/runs.go`) and `issue`, `severityOf()`, `countMissingAlt()` (in `engine/issues.go`).
4. **Toolchain & Runtime Reality**:
   - **Minimum Go Version**: **Go 1.22+** is strictly required, proven by standard library imports of `math/rand/v2` in `go/deps/httpx/httpx.go` and `log/slog` (Go 1.21+).
   - **Zero CGO**: The codebase contains zero `import "C"` calls. Tests use `modernc.org/sqlite` (pure Go). CGO is not required.
   - **Zero Build Tags**: Verified zero `//go:build` or `// +build` directives across all 58 files.
5. **Checkpoint A2 / A3 Sequencing Conflict**:
   - Checkpoint A2 specifies running `go list ./...` against `go.mod` before source files are reorganized in Checkpoint A3.
   - Because existing source files contain unresolved `onescout/...` imports, running `go list ./...` on the current raw directory structure will fail. Decision options are documented for review before A2 begins.

---

## 2. Reconciled File & Dependency Metrics

Programmatically verified from workspace source code:

| Metric | Count | Reconciled Files |
|---|---|---|
| **Total Go Files** | **58** | 15 engine + 9 storage + 3 pagespeed + 1 app-glue + 3 deps + 27 tests |
| **Declared `package sitecrawl`** | **55** | All engine (15), storage (9), pagespeed (3), app-glue (1), tests (27) |
| **Declared `package safe`** | **1** | `go/deps/safe/safe.go` |
| **Declared `package httpx`** | **2** | `go/deps/httpx/httpx.go`, `go/deps/httpx/delivery.go` |
| **Files with ANY `onescout/...` Import** | **16** | 4 bundled-only + 8 unbundled-only + 4 both |
| **Files with Unbundled 1Scout Imports** | **12** | `types.go`, `export.go`, `runs.go`, `service.go`, `pagespeed.go`, `cancel_test.go`, `crawler_test.go`, `credentials_test.go`, `history_test.go`, `live_test.go`, `pagespeed_test.go`, `schema_golden_test.go` |
| **Files with Bundled `core/httpx` or `core/safe` Imports** | **8** | `fetch.go`, `sitemap.go`, `pagespeed_pump.go`, `crawler.go`, `service.go`, `pagespeed.go`, `credentials_test.go`, `pagespeed_test.go` |
| **Files with Bundled Imports ONLY** | **4** | `go/engine/fetch.go`, `go/engine/sitemap.go`, `go/pagespeed/pagespeed_pump.go`, `go/storage/crawler.go` |
| **Files with Unbundled Imports ONLY** | **8** | `go/engine/types.go`, `go/storage/export.go`, `go/storage/runs.go`, `go/tests/cancel_test.go`, `go/tests/crawler_test.go`, `go/tests/history_test.go`, `go/tests/live_test.go`, `go/tests/schema_golden_test.go` |
| **Files with BOTH Bundled and Unbundled Imports** | **4** | `go/app-glue/service.go`, `go/pagespeed/pagespeed.go`, `go/tests/credentials_test.go`, `go/tests/pagespeed_test.go` |
| **Files with ZERO `onescout/...` Imports** | **42** | 12 engine + 6 storage + 1 pagespeed + 0 app-glue + 3 deps + 20 tests |

### Directory Breakdown

| Directory | Total Files | With Unbundled 1Scout | With Bundled 1Scout Only | Zero `onescout` Imports |
|---|---|---|---|---|
| `go/engine/` | 15 | 1 (`types.go`) | 2 (`fetch.go`, `sitemap.go`) | 12 |
| `go/storage/` | 9 | 2 (`export.go`, `runs.go`) | 1 (`crawler.go`) | 6 |
| `go/pagespeed/` | 3 | 1 (`pagespeed.go`) | 1 (`pagespeed_pump.go`) | 1 (`pagespeed_opps.go`) |
| `go/app-glue/` | 1 | 1 (`service.go`) | 0 | 0 |
| `go/deps/` | 3 | 0 | 0 | 3 |
| `go/tests/` | 27 | 7 (2 also bundled) | 0 | 20 |
| **Total** | **58** | **12** | **4** | **42** |

---

## 3. Comprehensive 58-File Inventory

### 3.1 `go/engine/` (15 Files)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Symbol Coupling | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`exclusions.go`](../go/engine/exclusions.go) | `sitecrawl` | None | None | `shouldExclude`, `defaultExclusions` (used by `crawler.go`, `types.go`) | Checkpoint A3 (Engine Core) | Low |
| [`extract.go`](../go/engine/extract.go) | `sitecrawl` | `golang.org/x/net/html` | None | `extractDocument`, `extractedDoc`, `SchemaItem`, `Image`, `Hreflang`, `LinkEdge` (used by `page.go`, `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`fetch.go`](../go/engine/fetch.go) | `sitecrawl` | None | `core/httpx` (bundled) | `newFetcher`, `fetcher`, `fetchResult`, `fetchOutcome` (used by `crawler.go`, `page.go`) | Checkpoint A3 (Engine Core) | Low |
| [`frontier.go`](../go/engine/frontier.go) | `sitecrawl` | None | None | `newFrontier`, `frontier`, `frontierItem`, `admit`, `peekReady`, `snapshot`, `restore` (used by `crawler.go`, `runs.go`) | Checkpoint A3 (Engine Core) | Low |
| [`issues.go`](../go/engine/issues.go) | `sitecrawl` | None | None | Defines `evaluate(p *Page)`, `indexabilityOf(p *Page)`, `issue`, `severityOf`, `countMissingAlt`. Defines method `(s *Service) IssueCatalog()` | Checkpoint A7 (Legacy Compatibility) / Compile dependency for `page.go` | Medium (depends on `Service`) |
| [`links.go`](../go/engine/links.go) | `sitecrawl` | None | None | `collectLinks`, `linkEdge`, `classifyRel` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`normalize.go`](../go/engine/normalize.go) | `sitecrawl` | None | None | `normalizeURL`, `frontierKey`, `isSameSite`, `rootDomain`, `stripTracking` (used across all crawler modules) | Checkpoint A3 (Engine Core) | Low |
| [`page.go`](../go/engine/page.go) | `sitecrawl` | None | None | Defines `buildPage`, `Page`, `pageRow`. Calls `nowStamp()` (in `storage/runs.go`), uses `issue`, `severityOf`, `countMissingAlt` (in `engine/issues.go`) | Checkpoint A3 (Engine Core) | Medium (cross-file symbol dependencies) |
| [`politeness.go`](../go/engine/politeness.go) | `sitecrawl` | None | None | `newHostGate`, `hostGate`, `onSuccess`, `onRetryable`, `onTerminal` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`render.go`](../go/engine/render.go) | `sitecrawl` | `github.com/chromedp/chromedp` | None | `findBrowser`, `renderPage`, `renderResult` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Medium (runtime browser dependency) |
| [`robots.go`](../go/engine/robots.go) | `sitecrawl` | `github.com/temoto/robotstxt` | None | `newRobots`, `robotsCache`, `Check`, `CrawlDelay`, `Sitemaps` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`similarity.go`](../go/engine/similarity.go) | `sitecrawl` | None | None | `simHash64`, `seqRatio` (used by `storage/duplicates.go`) | Checkpoint A3 (Engine Core) | Low |
| [`sitemap.go`](../go/engine/sitemap.go) | `sitecrawl` | None | `core/safe` (bundled) | `discoverSitemaps`, `parseSitemap`, `sitemapEntry` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`types.go`](../go/engine/types.go) | `sitecrawl` | None | `core/runs` (unbundled) | Core types (`Options`, `Page`, events, crawl modes). Uses `runs.State*` constants | Checkpoint A3 (Engine Core) | Low (needs status constants decoupled) |
| [`useragents.go`](../go/engine/useragents.go) | `sitecrawl` | None | None | `defaultUserAgent`, `UserAgentPresets` (used by `types.go`, `crawler.go`) | Checkpoint A3 (Engine Core) | Low |

---

### 3.2 `go/storage/` (9 Files)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Symbol Coupling | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`crawler.go`](../go/storage/crawler.go) | `sitecrawl` | None | `core/httpx`, `core/safe` (both bundled) | `coordinator`, `newCoordinator`, `prepare`, `loop`, `doOne`, `absorb`, `flush`. Uses `psiPump`, `PSIResult`, `newPSIPump`, `psiClient`, `savePSI` from `pagespeed/`; calls `c.finalize()` in `finalize.go` | Checkpoint A5 (Coordinator & Persistence) | High (compile-coupled to `pagespeed/` and `finalize.go`) |
| [`persist.go`](../go/storage/persist.go) | `sitecrawl` | None | None | `persistBuffer`, `writeBatch`, `writePage`, `writeLinks`, `writeIssues` (called by `crawler.go`) | Checkpoint A5 (Coordinator & Persistence) | Low |
| [`runs.go`](../go/storage/runs.go) | `sitecrawl` | None | `core/runs`, `core/schema` (both unbundled) | `schemaStmts`, `nowStamp`, `initRun`, `finishRun`, `saveFrontier`, `loadFrontier`, `loadSeen`, `ensureFTS`. Calls `runs.Now()`, `schema.Migrate()` | Checkpoint A5 (Coordinator & Persistence) | Medium (requires schema migration and timestamp adapter) |
| [`finalize.go`](../go/storage/finalize.go) | `sitecrawl` | None | None | Method `(c *coordinator) finalize(ctx)`, `finalizeCodes`, `finalizeInlinks`, `finalizeOrphans`, `finalizeCanonicals`, `clearIssues`. Uses `issues.go` codes | Checkpoint A7 (Optional Compatibility Features) | Medium (coupled to `coordinator` and `issues.go`) |
| [`duplicates.go`](../go/storage/duplicates.go) | `sitecrawl` | None | None | `findExactDuplicates`, `findNearDuplicates` (calls `simHash64`, `seqRatio` from `engine/similarity.go`) | Checkpoint A7 (Optional Compatibility Features) | Low |
| [`query.go`](../go/storage/query.go) | `sitecrawl` | None | None | Defines 9 methods on `(s *Service)`: `Rows`, `searchClause`, `issueRows`, `linkTabRows`, `Facets`, `Page`, `Inlinks`, `Outlinks`, `links` | Checkpoint A7 (Optional Compatibility Features) | High (coupled to desktop `Service`) |
| [`export.go`](../go/storage/export.go) | `sitecrawl` | None | `core/runs` (unbundled) | Defines 5 methods on `(s *Service)`: `Export`, `eachWindow`, `exportCSV`, `exportJSON`, `exportXML`. Uses `s.Files` (`FilePicker`), `runs.BOMUTF8`, `runs.CSVGuard` | Checkpoint A7 (Optional Compatibility Features) | High (coupled to desktop `Service` and `FilePicker`) |
| [`graph.go`](../go/storage/graph.go) | `sitecrawl` | None | None | Defines method `(s *Service) Graph(runID string, maxNodes int)` | Checkpoint A7 (Optional Compatibility Features) | Medium (coupled to desktop `Service`) |
| [`agent_local.go`](../go/storage/agent_local.go) | `sitecrawl` | None | None | `LookupLocalPages` (read-only search query for 1Scout AI Writer) | Checkpoint A7 (Optional Compatibility Features) | Low |

---

### 3.3 `go/pagespeed/` (3 Files)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Symbol Coupling | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`pagespeed.go`](../go/pagespeed/pagespeed.go) | `sitecrawl` | None | `core/credset`, `core/httpx`, `core/safe`, `core/workspace` | Defines `PSIResult`, `psiClient`, `savePSI`. Defines 7 methods on `(s *Service)`. Imported by `crawler.go` | Checkpoint A7 (Optional Compatibility Features) | High (coupled to `crawler.go`, `Service`, `workspace`, `credset`) |
| [`pagespeed_pump.go`](../go/pagespeed/pagespeed_pump.go) | `sitecrawl` | None | `core/safe` (bundled) | Defines `psiPump`, `newPSIPump`. Defines method `(s *Service) PendingPageSpeed`. Used directly by `crawler.go` | Checkpoint A7 (Optional Compatibility Features) | High (coupled to `crawler.go` and `Service`) |
| [`pagespeed_opps.go`](../go/pagespeed/pagespeed_opps.go) | `sitecrawl` | None | None | Defines 2 methods on `(s *Service)`: `Opportunities`, `OpportunityPages` | Checkpoint A7 (Optional Compatibility Features) | Medium (coupled to `Service` and PSI schema) |

---

### 3.4 `go/app-glue/` (1 File)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Symbol Coupling | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`service.go`](../go/app-glue/service.go) | `sitecrawl` | `github.com/wailsapp/wails/v3` | `core/credset`, `core/httpx`, `core/jobs`, `core/license`, `core/runs`, `core/schema`, `core/workspace`, `tools` | Defines `Service`, `FilePicker`, `handle`. Declares 24 methods on `(s *Service)`. Connects desktop UI to coordinator | Checkpoint A8 (UI / Service Bridge) | High (heavy 1Scout platform coupling; not needed for standalone core) |

---

### 3.5 `go/deps/` (3 Files)

| File | Package | External Imports | 1Scout Imports | Role / Purpose | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`safe/safe.go`](../go/deps/safe/safe.go) | `safe` | None | None | Panic recovery utilities: `safe.Go`, `safe.Do`, `safe.Call` (100% stdlib: `log/slog`, `runtime/debug`) | Checkpoint A3 (Bundled Dependency) | Low |
| [`httpx/httpx.go`](../go/deps/httpx/httpx.go) | `httpx` | None | None | Hardened HTTP client: timeouts, proxy hook, User-Agent pool, DNS filtering (100% stdlib, requires Go 1.22+ for `math/rand/v2`) | Checkpoint A3 (Bundled Dependency) | Low |
| [`httpx/delivery.go`](../go/deps/httpx/delivery.go) | `httpx` | None | None | Custom dialer preventing private network SSRF/rebinding | Checkpoint A3 (Bundled Dependency) | Low |

---

### 3.6 `go/tests/` (27 Files)

| Test File | Package | External Imports | 1Scout Imports | Test Focus & Coupling | Checkpoint Mapping |
|---|---|---|---|---|---|
| [`agent_local_test.go`](../go/tests/agent_local_test.go) | `sitecrawl` | `modernc.org/sqlite` | None (AST check only) | AST import validation, local HTML-only search | Checkpoint A6 |
| [`cancel_test.go`](../go/tests/cancel_test.go) | `sitecrawl` | None | `core/runs` (`runs.Terminal`) | Crawl cancellation lifecycle (uses `newTestService`) | Checkpoint A6 |
| [`crawler_test.go`](../go/tests/crawler_test.go) | `sitecrawl` | None | `core/runs` (`runs.BOMUTF8`) | Core fixture crawl: facets, status codes, canonicals (uses `newTestService`) | Checkpoint A6 |
| [`credentials_test.go`](../go/tests/credentials_test.go) | `sitecrawl` | None | `core/credset`, `core/httpx` | PageSpeed API key storage vault | Checkpoint A7 |
| [`customheaders_test.go`](../go/tests/customheaders_test.go) | `sitecrawl` | None | None | Custom HTTP headers forwarding during crawl | Checkpoint A6 |
| [`deferred_test.go`](../go/tests/deferred_test.go) | `sitecrawl` | None | None | 429 Too Many Requests & 503 retry/deferral loop | Checkpoint A6 |
| [`extract_test.go`](../go/tests/extract_test.go) | `sitecrawl` | None | None | HTML extraction: title, meta, canonical, hreflang, schema (pure stdlib unit test) | Checkpoint A3 / A6 |
| [`fixture_test.go`](../go/tests/fixture_test.go) | `sitecrawl` | None | None | Synthetic site fixture with redirect chains, loops, canonicals | Checkpoint A3 / A6 |
| [`frontier_cap_test.go`](../go/tests/frontier_cap_test.go) | `sitecrawl` | None | None | `MaxURLs` admission boundary enforcement (pure stdlib unit test) | Checkpoint A3 / A6 |
| [`frontier_test.go`](../go/tests/frontier_test.go) | `sitecrawl` | None | None | FIFO queue, BFS depth order, tracking param stripping (pure stdlib unit test) | Checkpoint A3 / A6 |
| [`fts_trigger_test.go`](../go/tests/fts_trigger_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | SQLite FTS5 index update triggers | Checkpoint A6 |
| [`history_test.go`](../go/tests/history_test.go) | `sitecrawl` | None | `core/jobs`, `core/workspace`, `testutil` | Defines `newTestService`, tests run history queries | Checkpoint A6 |
| [`inspectdb_test.go`](../go/tests/inspectdb_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | Manual crawl DB inspection utility (`CRAWL_DB=<path>`) | Checkpoint A6 |
| [`inspectopts_test.go`](../go/tests/inspectopts_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | Manual options inspection utility (`CRAWL_DB=<path>`) | Checkpoint A6 |
| [`live_test.go`](../go/tests/live_test.go) | `sitecrawl` | None | `core/jobs`, `core/workspace` | Live web crawl test against external sites (`CRAWL_LIVE=<url>`) | Checkpoint A6 |
| [`pagespeed_test.go`](../go/tests/pagespeed_test.go) | `sitecrawl` | None | `core/credset`, `core/httpx` | PageSpeed pump, concurrency pacing, error handling | Checkpoint A7 |
| [`pause_test.go`](../go/tests/pause_test.go) | `sitecrawl` | None | None | Pause and resume lifecycle, frontier preservation | Checkpoint A6 |
| [`perf_test.go`](../go/tests/perf_test.go) | `sitecrawl` | None | None | High-concurrency synthetic crawl benchmark (`CRAWL_PERF=1`) | Checkpoint A6 |
| [`politeness_test.go`](../go/tests/politeness_test.go) | `sitecrawl` | None | None | Per-host pacing, backoff upon error, `Crawl-delay` | Checkpoint A6 |
| [`psi_live_test.go`](../go/tests/psi_live_test.go) | `sitecrawl` | None | None | Live Google PSI API request test (`PSI_LIVE_KEY=<key>`) | Checkpoint A7 |
| [`query_test.go`](../go/tests/query_test.go) | `sitecrawl` | None | None | Grid query SQL construction and SQL injection safety (pure unit test) | Checkpoint A6 |
| [`render_test.go`](../go/tests/render_test.go) | `sitecrawl` | None | None | Chrome/Edge headless rendering of JS content (skips if no browser) | Checkpoint A6 |
| [`resume_ids_test.go`](../go/tests/resume_ids_test.go) | `sitecrawl` | None | None | URL ID stability across crawl pause and restart | Checkpoint A6 |
| [`schema_golden_test.go`](../go/tests/schema_golden_test.go) | `sitecrawl` | None | `testutil` (`SchemaGolden`) | SQLite schema migration regression test against golden file | Checkpoint A6 |
| [`seedredirect_test.go`](../go/tests/seedredirect_test.go) | `sitecrawl` | None | None | Initial seed URL 301/302 redirect resolution | Checkpoint A6 |
| [`similarity_test.go`](../go/tests/similarity_test.go) | `sitecrawl` | None | None | SimHash duplicate detection and content ratio tests (pure unit test) | Checkpoint A3 / A6 |
| [`sitemap_test.go`](../go/tests/sitemap_test.go) | `sitecrawl` | None | None | Sitemap index, gzip sitemaps, invalid XML tolerance (pure unit test) | Checkpoint A3 / A6 |

---

## 4. 1Scout Platform Dependency Analysis (Observable Requirements)

This section documents the exact symbols used from unbundled 1Scout platform packages and the concrete behavior observable from SiteCrawl source code:

| Package | Importing Files | Exact Symbols / Types Used | Observable Requirement in SiteCrawl Source | Standalone Replacement Needed? |
|---|---|---|---|---|
| **`core/runs`** | `types.go`, `runs.go`, `export.go`, `service.go`, `cancel_test.go`, `crawler_test.go` | `runs.StateRunning`, `runs.StatePaused`, `runs.StateCompleted`, `runs.StateCancelled`, `runs.StateStopped`, `runs.StateFailed`, `runs.StateInterrupted`, `runs.Now()`, `runs.BOMUTF8`, `runs.CSVGuard(string)`, `runs.Gate`, `runs.Terminal(string)` | - Provides run state string constants.<br>- Provides RFC3339 formatted UTC timestamp string.<br>- Provides byte sequence `[]byte{0xEF, 0xBB, 0xBF}` for CSV output.<br>- Provides CSV cell formula escaping.<br>- Checks if a state string is terminal.<br>- Provides per-workspace run gate in `service.go`. | **Yes (for standalone acquisition core)**: Replace constants, timestamp helper, and CSV helpers with native Go definitions in `internal/sitecrawl/`. `runs.Gate` is excluded with `service.go`. |
| **`core/schema`** | `runs.go`, `service.go` | `schema.Migrate(db *sql.DB, tool string, stmts []string) error`, `schema.Ready` | - In `runs.go`: Executes sequential SQLite DDL statements (`schemaStmts`) and tracks execution so statements are applied once.<br>- In `service.go`: Provides a synchronization gate (`ready.Do(...)`) to avoid replaying migrations on every user action. | **Yes (for standalone persistence)**: A standalone migration runner executing `schemaStmts` against `*sql.DB`. `schema.Ready` is excluded with `service.go`. |
| **`core/workspace`** | `service.go`, `pagespeed.go`, `history_test.go`, `live_test.go` | `workspace.Manager`, `workspace.NewManager()`, `workspace.WithTx(ctx, db, ...)` | - In `service.go`: Resolves workspace directory and opens SQLite DB handle.<br>- In `pagespeed.go`: Wraps SQLite transactions with backoff retry.<br>- In tests: Initializes a test workspace database. | **No (for acquisition core)**: The core crawler operates on standard `*sql.DB`. Desktop workspace management is excluded. Tests can open SQLite files directly via standard `database/sql` and `modernc.org/sqlite`. |
| **`core/jobs`** | `service.go`, `cancel_test.go`, `history_test.go`, `live_test.go` | `jobs.Runner`, `jobs.NewRunner(...)`, `Jobs.Start(...)`, `Jobs.Emit(...)`, `Jobs.Cancel(...)`, `jobs.EventName`, `jobs.Progress` | - In `service.go`: Desktop supervisor for background goroutines and Wails event emission.<br>- In tests: Used to construct `Service` instances. | **No**: Excluded with `service.go`. Crawler worker concurrency is natively managed via standard goroutines, channels, and `context.Context`. |
| **`core/credset`** | `service.go`, `pagespeed.go`, `credentials_test.go`, `pagespeed_test.go` | `credset.Store`, `credset.First(...)`, `credset.Add(...)`, `credset.Save(...)` | - In `service.go` & `pagespeed.go`: Reads encrypted API keys for Google PageSpeed Insights and proxy configuration. | **No**: Excluded with `service.go` and `pagespeed/`. Crawler proxy settings can be passed directly via `Options.UseProxy`. |
| **`core/license`** | `service.go` | `license.Gate` | - Verifies commercial application license. | **No**: Excluded completely. Has zero function in the audit tool. |
| **`tools`** | `service.go` | `tools.Register(...)`, `tools.Factory`, `tools.Starter` | - 1Scout desktop plugin registry hook (`tools.Register(tools.Factory{...})`). | **No**: Excluded with `service.go`. Standalone tool uses CLI or Go API. |
| **`testutil`** | `history_test.go`, `schema_golden_test.go` | `testutil.RedirectConfigDir(t *testing.T)`, `testutil.SchemaGolden(t *testing.T, tool string, stmts []string)` | - `RedirectConfigDir`: Redirects config directory to temporary directory.<br>- `SchemaGolden`: Asserts that DDL output matches golden schema file. | **No**: Replace `RedirectConfigDir` with `t.TempDir()`. `SchemaGolden` can be adapted or deferred. |
| **`core/safe`** *(bundled)* | `sitemap.go`, `crawler.go`, `pagespeed.go`, `pagespeed_pump.go` | `safe.Do()`, `safe.Call()`, `safe.Go()` | - Panic recovery wrapper for worker goroutines. | **Yes (Preserved)**: Source code is bundled in `go/deps/safe/safe.go`. |
| **`core/httpx`** *(bundled)* | `fetch.go`, `crawler.go`, `service.go`, `pagespeed.go`, `credentials_test.go`, `pagespeed_test.go` | `httpx.NewFiltered(...)`, `httpx.ProxyFunc`, `httpx.IsLocalName`, `httpx.AllowPrivate` | - HTTP client with safe dialing, DNS checks, and proxy rotation hooks. | **Yes (Preserved)**: Source code is bundled in `go/deps/httpx/`. |

---

## 5. Compile-Time Symbol Dependency Closures & Cross-File Knots

Because all non-utility files declare `package sitecrawl`, unexported symbols are freely shared across the current directories. This creates several tight compile-time dependency closures that prevent arbitrary file partitioning:

```text
┌────────────────────────────────────────────────────────────────────────┐
│                        COMPILE-TIME COUPLING KNOTS                     │
├────────────────────────────────────────────────────────────────────────┤
│ 1. Crawler → PageSpeed:                                               │
│    storage/crawler.go  ──uses──►  psiPump, PSIResult, newPSIPump,     │
│                                  psiClient, savePSI (pagespeed/*.go)   │
│                                                                        │
│ 2. Crawler → Finalize:                                                │
│    storage/crawler.go  ──calls─►  (c *coordinator) finalize(ctx)      │
│                                  (storage/finalize.go)                 │
│                                                                        │
│ 3. Page → Runs + Issues:                                              │
│    engine/page.go      ──calls─►  nowStamp() (storage/runs.go)        │
│                        ──uses──►  issue, severityOf, countMissingAlt   │
│                                  (engine/issues.go)                    │
│                                                                        │
│ 4. Service Receiver Methods:                                           │
│    app-glue/service.go ──declares► type Service struct { ... }        │
│                               ▲                                        │
│                               │ attached to Service                    │
│    ├── engine/issues.go       ┴── (s *Service) IssueCatalog()          │
│    ├── storage/query.go       ─── 9 methods: Rows, Facets, Page...     │
│    ├── storage/export.go      ─── 5 methods: Export, exportCSV...      │
│    ├── storage/graph.go       ─── 1 method: Graph                      │
│    └── pagespeed/*.go         ─── 10 methods on Service                │
└────────────────────────────────────────────────────────────────────────┘
```

### Detailed Compile-Time Closures

1. **`storage/crawler.go` ↔ `go/pagespeed/*.go`**:
   - `storage/crawler.go` cannot compile without `go/pagespeed/`:
     - Line 134: `psi *psiPump` (`psiPump` declared in `pagespeed_pump.go`).
     - Line 135, 145: `psiOut chan PSIResult`, `psiRows []PSIResult` (`PSIResult` declared in `pagespeed.go`).
     - Line 253: calls `newPSIPump(...)` (declared in `pagespeed_pump.go`).
     - Line 254: calls `psiClient()` (declared in `pagespeed.go`).
     - Line 851: calls `savePSI(...)` (declared in `pagespeed.go`).
2. **`storage/crawler.go` ↔ `storage/finalize.go`**:
   - Line 835: calls `c.finalize(ctx)`, which is a method defined on `*coordinator` in `storage/finalize.go`.
   - `finalize.go` in turn references `IssueOrphan`, `IssueInternalRedirect`, etc., declared in `engine/issues.go`.
3. **`engine/page.go` ↔ `storage/runs.go` & `engine/issues.go`**:
   - `engine/page.go` cannot compile alone without:
     - Line 40: `nowStamp()`, declared in `storage/runs.go` line 300.
     - Lines 34, 154: `issues []issue`, declared in `engine/issues.go` line 125.
     - Line 157: `severityOf(is.Code)`, declared in `engine/issues.go` line 144.
     - Line 197: `countMissingAlt(p.Images)`, declared in `engine/issues.go` line 437.
4. **`Service` Receiver Method Scattering**:
   - If `app-glue/service.go` is excluded to avoid 1Scout platform dependencies, any file defining a method on `(s *Service)` fails to compile (`undefined: Service`):
     - `engine/issues.go` line 516: `func (s *Service) IssueCatalog() []IssueInfo`
     - `storage/query.go`: 9 methods on `*Service`
     - `storage/export.go`: 5 methods on `*Service` (and uses `FilePicker` from `service.go`)
     - `storage/graph.go`: `func (s *Service) Graph(...)`
     - `pagespeed/*.go`: 10 methods on `*Service`

---

## 6. Migration Checkpoint Reconciliation & Package Proposals

Reconciling the source code dependencies with [`sitecrawl-standalone-migration-map.md`](sitecrawl-standalone-migration-map.md):

### 6.1 Checkpoint Sequence per Migration Map
- **Checkpoint A3 — Reconstruct `sitecrawl` Package**:
  - Target: `internal/sitecrawl/`
  - Intended scope: Core acquisition engine (`normalize`, `frontier`, `politeness`, `robots`, `sitemap`, `fetch`, `extract`, `page`, `links`, `render`, `deps` + `exclusions`, `similarity`, `types`, `useragents`).
  - Acceptance: Core package compiles; portable engine tests pass.
- **Checkpoint A4 — Standalone Platform Adapters**:
  - Implement standalone replacements for `runs`, `schema`, `workspace/database`, `jobs/events`.
- **Checkpoint A5 — Port Coordinator + Persistence**:
  - Port `storage/crawler.go`, `storage/persist.go`, `storage/runs.go`.
  - Acceptance: Fixture site can be crawled end-to-end; results survive process-level readback.
- **Checkpoint A6 — Regression Parity**:
  - Port applicable tests, establish regression parity, create freeze tag `sitecrawl-standalone-core-v1`.
- **Checkpoint A7 — Optional Compatibility Features**:
  - Migrate legacy issues (`issues.go`), post-crawl finalize (`finalize.go`), duplicates (`duplicates.go`), exports (`export.go`), query (`query.go`), graph (`graph.go`), and PageSpeed (`pagespeed/`).

### 6.2 The Cross-Checkpoint Compile Conflicts
The compile-time dependencies identified in Section 5 create specific conflicts across the planned checkpoints:

1. **A3 Engine Core depends on A5 and A7 symbols**:
   - `page.go` (A3) requires `nowStamp()` (in `runs.go` — A5) and `issue`, `severityOf`, `countMissingAlt` (in `issues.go` — A7).
   - *Conflict*: A3 cannot compile purely from the prioritized A3 file list without resolving these four symbols.
2. **A5 Coordinator depends on A7 symbols**:
   - `crawler.go` (A5) requires `psiPump`, `PSIResult`, `newPSIPump`, `psiClient`, `savePSI` (in `pagespeed/` — A7) and `c.finalize()` (in `finalize.go` — A7).
   - *Conflict*: A5 cannot compile without either including `pagespeed/` and `finalize.go`, or introducing stubs/interfaces for them.
3. **A7 Compatibility files depend on A8 Desktop `Service`**:
   - `issues.go`, `query.go`, `export.go`, `graph.go`, and `pagespeed/*.go` define methods on `(s *Service)`.
   - *Conflict*: If `service.go` is deferred or excluded, these A7 files cannot compile in their current form.

### 6.3 Reconciled Checkpoint File Placement

| File Path | Original Group | Planned Checkpoint | Compile-Time Blockers to Resolve at that Checkpoint |
|---|---|---|---|
| `go/engine/exclusions.go` | engine | **A3** | None |
| `go/engine/extract.go` | engine | **A3** | None (requires `golang.org/x/net/html`) |
| `go/engine/fetch.go` | engine | **A3** | Redirect import from `onescout/.../core/httpx` to local `httpx` |
| `go/engine/frontier.go` | engine | **A3** | None |
| `go/engine/links.go` | engine | **A3** | None |
| `go/engine/normalize.go` | engine | **A3** | None |
| `go/engine/politeness.go` | engine | **A3** | None |
| `go/engine/render.go` | engine | **A3** | None (requires `github.com/chromedp/chromedp`) |
| `go/engine/robots.go` | engine | **A3** | None (requires `github.com/temoto/robotstxt`) |
| `go/engine/similarity.go` | engine | **A3** | None |
| `go/engine/sitemap.go` | engine | **A3** | Redirect import from `onescout/.../core/safe` to local `safe` |
| `go/engine/types.go` | engine | **A3** | Decouple `runs.State*` constants (replace with local constants or A4 adapter) |
| `go/engine/useragents.go` | engine | **A3** | None |
| `go/deps/safe/safe.go` | deps | **A3** | None |
| `go/deps/httpx/httpx.go` | deps | **A3** | None |
| `go/deps/httpx/delivery.go` | deps | **A3** | None |
| `go/engine/page.go` | engine | **A3** | **Knot 3**: Needs resolution of `nowStamp` (A5) and `issue`/`severityOf`/`countMissingAlt` (A7) |
| `go/engine/issues.go` | engine | **A7** (or pull stub to A3) | **Knot 4**: Method `IssueCatalog()` requires `Service` |
| `go/storage/persist.go` | storage | **A5** | Uses `issue` type from `issues.go` |
| `go/storage/runs.go` | storage | **A5** | Requires `core/schema` and `core/runs` adapters (from A4) |
| `go/storage/crawler.go` | storage | **A5** | **Knots 1 & 2**: Requires PSI symbols (`pagespeed/`) and `c.finalize()` (`finalize.go`) |
| `go/storage/finalize.go` | storage | **A7** (or pull to A5) | Requires `coordinator` and `issues.go` |
| `go/storage/duplicates.go` | storage | **A7** | None (uses `similarity.go` and `*sql.DB`) |
| `go/storage/query.go` | storage | **A7** | **Knot 4**: 9 methods on `*Service` |
| `go/storage/export.go` | storage | **A7** | **Knot 4**: 5 methods on `*Service`, `FilePicker`, and `core/runs` helpers |
| `go/storage/graph.go` | storage | **A7** | **Knot 4**: Method on `*Service` |
| `go/storage/agent_local.go` | storage | **A7** | None |
| `go/pagespeed/pagespeed.go` | pagespeed | **A7** | **Knot 4**: 7 methods on `*Service`, `credset`, `workspace` |
| `go/pagespeed/pagespeed_pump.go` | pagespeed | **A7** | **Knot 4**: Method on `*Service` |
| `go/pagespeed/pagespeed_opps.go` | pagespeed | **A7** | **Knot 4**: 2 methods on `*Service` |
| `go/app-glue/service.go` | app-glue | **A8 / Exclude** | 1Scout platform framework coupling |
| `go/tests/*` (27 files) | tests | **A3 / A6 / A7** | 20 unit tests portable; 4 need test harness (A6); 3 PSI deferred (A7) |

---

## 7. Re-evaluation of PageSpeed and Service Decisions

### 7.1 PageSpeed Coupling in Coordinator
In `storage/crawler.go`, PageSpeed is not merely an optional flag evaluated at runtime; it is baked into the `coordinator` struct definition, channel loops, and flush cycles:
- Fields: `c.psi *psiPump`, `c.psiOut chan PSIResult`, `c.psiRows []PSIResult`.
- Methods called: `newPSIPump(...)`, `c.psi.Start(...)`, `c.psi.Stop()`, `c.psi.Seal()`, `c.drainPSI(ctx)`, `savePSI(c.db, c.runID, r)`.

**Decision Options for A5**:
1. **Option A (Preserve PSI Types in A5)**: Move `pagespeed_pump.go` and `pagespeed.go` into `internal/sitecrawl/` along with `crawler.go` in Checkpoint A5, providing no-op/mock credentials when PageSpeed is disabled.
2. **Option B (Coordinator Interface / Null Object Pattern)**: In Checkpoint A5, define a clean internal interface on `coordinator` (e.g. `type psiCollector interface { ... }`) with a no-op implementation, decoupling the coordinator from concrete PageSpeed code.
3. **Option C (Postpone Coordinator Extraction to A5 with Decision Gate)**: Do not attempt to separate PageSpeed in A3; keep analysis read-only and select Option A or B as an explicit design task at the start of Checkpoint A5.

### 7.2 Service Coupling in Storage & Issues
`Service` is the central desktop facade from 1Scout. Attaching methods to `(s *Service)` across `issues.go`, `query.go`, `export.go`, `graph.go`, and `pagespeed/*.go` binds these storage and catalog features directly to the desktop UI glue.

**Decision Options for Later Checkpoints**:
1. **Option A (Decouple Receiver Methods)**: In Checkpoint A7, refactor query and export methods from `func (s *Service) QueryPages(...)` into standalone functions taking an `*sql.DB` or repository struct (e.g. `func QueryPages(db *sql.DB, q RowQuery) ...`), and decouple `IssueCatalog()` from `Service`.
2. **Option B (Standalone Service Struct)**: Define a minimal standalone `type Service struct { DB *sql.DB }` in `internal/sitecrawl/` that provides the receiver target without importing Wails or 1Scout platform packages.

---

## 8. Checkpoint A2 Feasibility Analysis & Sequencing Conflict

### 8.1 The Conflict
`sitecrawl-standalone-migration-map.md` defines Checkpoint A2 as:
> **Checkpoint A2 — Create Standalone Go Module**
> Create: `go.mod`
> Introduce only the dependencies required by the acquisition core.
> Acceptance: `go list ./...` must enumerate intended standalone packages without relying on the 1Scout repository.

However, in the current repository:
1. All 58 Go files reside in subdirectories under `go/` (`go/engine/`, `go/storage/`, `go/app-glue/`, etc.).
2. 16 of these files contain import statements pointing to `onescout/desktop/internal/...`.
3. If `go.mod` is created at the repository root and `go list ./...` is executed:
   - Go attempts to load every package in every subdirectory.
   - Go fails immediately with errors such as:
     `cannot find module providing package onescout/desktop/internal/core/httpx`
     `cannot find module providing package onescout/desktop/internal/core/runs`
     `cannot find module providing package onescout/desktop/internal/core/schema`

Therefore, `go.mod` + external dependencies alone **cannot satisfy `go list ./...` against the current un-migrated source tree**.

### 8.2 Decision Options for Review Before A2
Before proceeding to Checkpoint A2, one of the following approaches must be selected:

- **Option 1 (Scoped Package Verification)**:
  - In Checkpoint A2, create `go.mod` at the repository root.
  - Evaluate `go list` not with `./...` (which scans un-migrated legacy directories), but specifically against the intended standalone target directory or bundled dependencies (e.g. `go list ./go/deps/...` or once `internal/sitecrawl` is created).
- **Option 2 (Combine A2 and A3 into a Single Atomic Transition)**:
  - Initialize `go.mod` and populate `internal/sitecrawl/` with the reconstructed package and updated import paths in one coordinated step, ensuring `go list ./internal/...` immediately resolves cleanly.
- **Option 3 (Temporary Local Module Shims in A2)**:
  - In Checkpoint A2, add `replace onescout/desktop/internal/... => ./go/deps/...` directives or minimal stub packages in a temporary directory so `go list ./...` succeeds across all legacy files.

---

## 9. Build Metadata & Runtime Constraints

- **Minimum Go Version**: **Go 1.22+**. Proven by standard library import `"math/rand/v2"` in `go/deps/httpx/httpx.go` line 11.
- **Pure Go / Zero CGO**: Verified zero `import "C"` calls. Tests use `modernc.org/sqlite`, allowing compilation and testing with `CGO_ENABLED=0`.
- **Zero Build Tags**: Verified zero `//go:build` or `// +build` directives across all 58 files.
- **External Go Modules**:
  1. `golang.org/x/net` (specifically `golang.org/x/net/html`)
  2. `github.com/temoto/robotstxt`
  3. `github.com/chromedp/chromedp`
  4. `modernc.org/sqlite`
  *(Note: `github.com/wailsapp/wails/v3` is only needed if desktop `service.go` is compiled; it is excluded from the standalone acquisition core).*
- **Environment-Gated Tests**:
  - `CRAWL_LIVE=<url>`: in `go/tests/live_test.go`
  - `PSI_LIVE_KEY=<key>`: in `go/tests/psi_live_test.go`
  - `CRAWL_PERF=1`: in `go/tests/perf_test.go`
  - `CRAWL_DB=<path>`: in `go/tests/inspectdb_test.go` and `go/tests/inspectopts_test.go`
  - Browser presence: `go/tests/render_test.go` skips if no local Chrome/Edge binary is detected.

---

## 10. Required Conclusions

### 1. What prevents the extracted repository from building standalone today?
- Absence of a `go.mod` file defining module boundaries and external dependency versions.
- 16 files importing `onescout/desktop/internal/...` (12 files importing unbundled platform packages).
- Undeclared external modules (`x/net/html`, `robotstxt`, `chromedp`, `modernc.org/sqlite`).
- Missing compile-time closure when directories are partitioned (e.g. `page.go` needing symbols from `runs.go` and `issues.go`).

### 2. Which files form the smallest useful acquisition core?
- Per Checkpoint A3 of the migration map: the core engine mechanics files in `go/engine/` (`normalize.go`, `frontier.go`, `politeness.go`, `robots.go`, `sitemap.go`, `fetch.go`, `extract.go`, `page.go`, `links.go`, `render.go`, `exclusions.go`, `similarity.go`, `types.go`, `useragents.go`) plus the bundled dependencies in `go/deps/` (`safe.go`, `httpx.go`, `delivery.go`).
- *Note*: To achieve complete compile-time symbol closure, the four cross-file symbols required by `page.go` (`nowStamp`, `issue`, `severityOf`, `countMissingAlt`) must be resolved (either by defining local shims or pulling in the required definitions).

### 3. Which 1Scout dependencies block that core?
- For the A3 engine core: only `core/runs` (status string constants in `types.go`). `core/httpx` and `core/safe` are already bundled in `go/deps/`.
- For A5 coordinator + persistence: `core/schema` (`schema.Migrate`) and `core/runs` (`runs.Now()`).
- All other platform dependencies (`core/workspace`, `core/jobs`, `core/credset`, `core/license`, `tools`) belong to `service.go` and `pagespeed/`, which are outside the acquisition core.

### 4. What minimal standalone adapters will eventually be required?
- **Run Constants & Timestamp Adapter (A4)**: Local definitions for run states (`StateRunning = "running"`, etc.), UTC RFC3339 timestamp helper (`nowStamp()`), and CSV export helpers (`BOMUTF8`, formula escaping).
- **Schema Migration Adapter (A4)**: A standalone migration runner executing `schemaStmts` against `*sql.DB`.
- **Internal Module Path Redirects (A3)**: Updating imports for `safe` and `httpx` from `onescout/desktop/internal/core/...` to local module paths.

### 5. Which dependencies/features can be deferred?
- **Google PageSpeed Insights (`pagespeed/`)**: Can be deferred to Checkpoint A7.
- **Wails Desktop Service (`app-glue/service.go`)**: Excluded from the standalone core (deferred to A8 or replaced by a CLI harness in A7).
- **Platform Licensing & Jobs (`license`, `jobs`, `tools`, `credset`)**: Excluded completely.

### 6. Which package-private dependencies make early package splitting unsafe?
- The extensive sharing of unexported structs and functions across `crawler.go`, `page.go`, `frontier.go`, `fetch.go`, `extract.go`, `persist.go`, and `runs.go` makes splitting `sitecrawl` into subpackages (`crawler/frontier`, `crawler/fetch`, etc.) hazardous. The unified `internal/sitecrawl/` package structure must be preserved.

### 7. What exact scope should Checkpoint A2 (`go.mod`) contain?
- Declare the standalone module path (e.g. `truthrive/technical-seo-audit`).
- Declare `go 1.22`.
- Require the 4 verified external packages (`golang.org/x/net`, `github.com/temoto/robotstxt`, `github.com/chromedp/chromedp`, `modernc.org/sqlite`).
- Resolve the A2/A3 sequencing conflict (e.g. verify `go list` against scoped targets).

### 8. What should explicitly NOT be done in A2/A3?
- Do NOT implement any of the 47 `AR-*` Audit V1 rules.
- Do NOT rewrite or redesign the crawler loop, frontier, or HTTP fetching.
- Do NOT redesign the `Page` struct.
- Do NOT split `sitecrawl` into multiple subpackages.
- Do NOT attempt to port 1Scout desktop frameworks (`jobs.Runner`, `license.Gate`, `tools.Register`, Wails v3).
- Do NOT delete legacy issues logic (`issues.go`) prematurely.
