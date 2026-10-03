# SiteCrawl Standalone Buildability Inventory (Checkpoint A1)

- **Status:** Completed and Approved
- **Checkpoint:** A1 — Buildability Inventory
- **Upstream Map:** [`sitecrawl-standalone-migration-map.md`](sitecrawl-standalone-migration-map.md)
- **Baseline Git Tag:** `sitecrawl-pre-audit-v1` (`82c610d433203374473e1ea76edf4e6c165a3161`)
- **Nature of Work:** Strictly read-only analysis. No production code, tests, `go.mod`, or configuration files are modified during Checkpoint A1.

---

## 1. Executive Summary

This inventory evaluates the compile-time reality, dependency graph, and standalone buildability of the extracted SiteCrawl module from 1Scout Marketing (commit `2e2cbff`, 2026-09-25) in its current state within `truthrive/technical-seo-audit`. Checkpoint A1 is **completed and approved**, establishing the factual baseline and sequencing decisions for subsequent checkpoints.

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
     - `engine/page.go` relies on `nowStamp()` (in `storage/runs.go`) and `issue`, `severityOf`, `countMissingAlt` (in `engine/issues.go`).
4. **Toolchain & Runtime Reality**:
   - **Minimum Go Version**: **Go 1.22+** is strictly required, proven by standard library imports of `math/rand/v2` in `go/deps/httpx/httpx.go` and `log/slog` (Go 1.21+).
   - **Zero CGO**: The codebase contains zero `import "C"` calls. Tests use `modernc.org/sqlite` (pure Go). CGO is not required.
   - **Zero Build Tags**: Verified zero `//go:build` or `// +build` directives across all 58 files.
5. **Approved Sequencing Decisions**:
   - **A2 Scoped Verification**: A2 creates root `go.mod` (`github.com/truthrive/technical-seo-audit`, `go 1.22`) and verifies genuine standalone packages via scoped target `go list ./go/deps/...` without temporary `replace` directives. External dependencies are introduced just-in-time. Full repository `go list ./...` is enforced by Checkpoint A6.
   - **Coordinated A3/A4 Milestone**: Package reconstruction and minimal real platform contracts land together without stubbing legacy issue semantics. Initial A3 engine boundary prioritizes 13 engine mechanics files and bundled `safe`/`httpx`; `page.go` is deferred to A5 due to cross-file coupling with `nowStamp`, `issue`, `severityOf`, and `countMissingAlt`.
   - **Open Decision Gates Preserved**: A5 PageSpeed decoupling and later Service receiver decoupling remain open decisions to be resolved during their respective checkpoints.

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
| [`exclusions.go`](../_reference/sitecrawl/engine/exclusions.go) | `sitecrawl` | None | None | `shouldExclude`, `defaultExclusions` (used by `crawler.go`, `types.go`) | Checkpoint A3 (Engine Core) | Low |
| [`extract.go`](../_reference/sitecrawl/engine/extract.go) | `sitecrawl` | `golang.org/x/net/html` | None | `extractDocument`, `extractedDoc`, `SchemaItem`, `Image`, `Hreflang`, `LinkEdge` (used by `page.go`, `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`fetch.go`](../_reference/sitecrawl/engine/fetch.go) | `sitecrawl` | None | `core/httpx` (bundled) | `newFetcher`, `fetcher`, `fetchResult`, `fetchOutcome` (used by `crawler.go`, `page.go`) | Checkpoint A3 (Engine Core) | Low |
| [`frontier.go`](../_reference/sitecrawl/engine/frontier.go) | `sitecrawl` | None | None | `newFrontier`, `frontier`, `frontierItem`, `admit`, `peekReady`, `snapshot`, `restore` (used by `crawler.go`, `runs.go`) | Checkpoint A3 (Engine Core) | Low |
| [`issues.go`](../_reference/sitecrawl/engine/issues.go) | `sitecrawl` | None | None | Defines `evaluate(p *Page)`, `indexabilityOf(p *Page)`, `issue`, `severityOf`, `countMissingAlt`. Defines method `(s *Service) IssueCatalog()` | Checkpoint A7 (Optional Compatibility Features) | Medium (depends on `Service`) |
| [`links.go`](../_reference/sitecrawl/engine/links.go) | `sitecrawl` | None | None | `collectLinks`, `linkEdge`, `classifyRel` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`normalize.go`](../_reference/sitecrawl/engine/normalize.go) | `sitecrawl` | None | None | `normalizeURL`, `frontierKey`, `isSameSite`, `rootDomain`, `stripTracking` (used across all crawler modules) | Checkpoint A3 (Engine Core) | Low |
| [`page.go`](../_reference/sitecrawl/engine/page.go) | `sitecrawl` | None | None | Defines `buildPage`, `Page`, `pageRow`. Calls `nowStamp()` (in `storage/runs.go`), uses `issue`, `severityOf`, `countMissingAlt` (in `engine/issues.go`) | Checkpoint A3 (Engine Core) | Medium (cross-file symbol dependencies) |
| [`politeness.go`](../_reference/sitecrawl/engine/politeness.go) | `sitecrawl` | None | None | `newHostGate`, `hostGate`, `onSuccess`, `onRetryable`, `onTerminal` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`render.go`](../_reference/sitecrawl/engine/render.go) | `sitecrawl` | `github.com/chromedp/chromedp` | None | `findBrowser`, `renderPage`, `renderResult` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Medium (runtime browser dependency) |
| [`robots.go`](../_reference/sitecrawl/engine/robots.go) | `sitecrawl` | `github.com/temoto/robotstxt` | None | `newRobots`, `robotsCache`, `Check`, `CrawlDelay`, `Sitemaps` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`similarity.go`](../_reference/sitecrawl/engine/similarity.go) | `sitecrawl` | None | None | `simHash64`, `seqRatio` (used by `storage/duplicates.go`) | Checkpoint A3 (Engine Core) | Low |
| [`sitemap.go`](../_reference/sitecrawl/engine/sitemap.go) | `sitecrawl` | None | `core/safe` (bundled) | `discoverSitemaps`, `parseSitemap`, `sitemapEntry` (used by `crawler.go`) | Checkpoint A3 (Engine Core) | Low |
| [`types.go`](../_reference/sitecrawl/engine/types.go) | `sitecrawl` | None | `core/runs` (unbundled) | Core types (`Options`, `Page`, events, crawl modes). Uses `runs.State*` constants | Checkpoint A3 (Engine Core) | Low (needs status constants decoupled) |
| [`useragents.go`](../_reference/sitecrawl/engine/useragents.go) | `sitecrawl` | None | None | `defaultUserAgent`, `UserAgentPresets` (used by `types.go`, `crawler.go`) | Checkpoint A3 (Engine Core) | Low |

---

### 3.2 `_reference/sitecrawl/storage/` (9 Files)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Symbol Coupling | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`crawler.go`](../_reference/sitecrawl/storage/crawler.go) | `sitecrawl` | None | `core/httpx`, `core/safe` (both bundled) | `coordinator`, `newCoordinator`, `prepare`, `loop`, `doOne`, `absorb`, `flush`. Uses `psiPump`, `PSIResult`, `newPSIPump`, `psiClient`, `savePSI` from `pagespeed/`; calls `c.finalize()` in `finalize.go` | Checkpoint A5 (Coordinator & Persistence) | High (compile-coupled to `pagespeed/` and `finalize.go`) |
| [`persist.go`](../_reference/sitecrawl/storage/persist.go) | `sitecrawl` | None | None | `persistBuffer`, `writeBatch`, `writePage`, `writeLinks`, `writeIssues` (called by `crawler.go`) | Checkpoint A5 (Coordinator & Persistence) | Low |
| [`runs.go`](../_reference/sitecrawl/storage/runs.go) | `sitecrawl` | None | `core/runs`, `core/schema` (both unbundled) | `schemaStmts`, `nowStamp`, `initRun`, `finishRun`, `saveFrontier`, `loadFrontier`, `loadSeen`, `ensureFTS`. Calls `runs.Now()`, `schema.Migrate()` | Checkpoint A5 (Coordinator & Persistence) | Medium (requires schema migration and timestamp adapter) |
| [`finalize.go`](../_reference/sitecrawl/storage/finalize.go) | `sitecrawl` | None | None | Method `(c *coordinator) finalize(ctx)`, `finalizeCodes`, `finalizeInlinks`, `finalizeOrphans`, `finalizeCanonicals`, `clearIssues`. Uses `issues.go` codes | Checkpoint A7 (Optional Compatibility Features) | Medium (coupled to `coordinator` and `issues.go`) |
| [`duplicates.go`](../_reference/sitecrawl/storage/duplicates.go) | `sitecrawl` | None | None | `findExactDuplicates`, `findNearDuplicates` (calls `simHash64`, `seqRatio` from `engine/similarity.go`) | Checkpoint A7 (Optional Compatibility Features) | Low |
| [`query.go`](../_reference/sitecrawl/storage/query.go) | `sitecrawl` | None | None | Defines 9 methods on `(s *Service)`: `Rows`, `searchClause`, `issueRows`, `linkTabRows`, `Facets`, `Page`, `Inlinks`, `Outlinks`, `links` | Checkpoint A7 (Optional Compatibility Features) | High (coupled to desktop `Service`) |
| [`export.go`](../_reference/sitecrawl/storage/export.go) | `sitecrawl` | None | `core/runs` (unbundled) | Defines 5 methods on `(s *Service)`: `Export`, `eachWindow`, `exportCSV`, `exportJSON`, `exportXML`. Uses `s.Files` (`FilePicker`), `runs.BOMUTF8`, `runs.CSVGuard` | Checkpoint A7 (Optional Compatibility Features) | High (coupled to desktop `Service` and `FilePicker`) |
| [`graph.go`](../_reference/sitecrawl/storage/graph.go) | `sitecrawl` | None | None | Defines method `(s *Service) Graph(runID string, maxNodes int)` | Checkpoint A7 (Optional Compatibility Features) | Medium (coupled to desktop `Service`) |
| [`agent_local.go`](../_reference/sitecrawl/storage/agent_local.go) | `sitecrawl` | None | None | `LookupLocalPages` (read-only search query for 1Scout AI Writer) | Checkpoint A7 (Optional Compatibility Features) | Low |

---

### 3.3 `_reference/sitecrawl/pagespeed/` (3 Files)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Symbol Coupling | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`pagespeed.go`](../_reference/sitecrawl/pagespeed/pagespeed.go) | `sitecrawl` | None | `core/credset`, `core/httpx`, `core/safe`, `core/workspace` | Defines `PSIResult`, `psiClient`, `savePSI`. Defines 7 methods on `(s *Service)`. Imported by `crawler.go` | Checkpoint A7 (Optional Compatibility Features) | High (coupled to `crawler.go`, `Service`, `workspace`, `credset`) |
| [`pagespeed_pump.go`](../_reference/sitecrawl/pagespeed/pagespeed_pump.go) | `sitecrawl` | None | `core/safe` (bundled) | Defines `psiPump`, `newPSIPump`. Defines method `(s *Service) PendingPageSpeed`. Used directly by `crawler.go` | Checkpoint A7 (Optional Compatibility Features) | High (coupled to `crawler.go` and `Service`) |
| [`pagespeed_opps.go`](../_reference/sitecrawl/pagespeed/pagespeed_opps.go) | `sitecrawl` | None | None | Defines 2 methods on `(s *Service)`: `Opportunities`, `OpportunityPages` | Checkpoint A7 (Optional Compatibility Features) | Medium (coupled to `Service` and PSI schema) |

---

### 3.4 `_reference/sitecrawl/app-glue/` (1 File)

| File | Package | External Imports | 1Scout Imports | Important Package-Private Symbol Coupling | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`service.go`](../_reference/sitecrawl/app-glue/service.go) | `sitecrawl` | `github.com/wailsapp/wails/v3` | `core/credset`, `core/httpx`, `core/jobs`, `core/license`, `core/runs`, `core/schema`, `core/workspace`, `tools` | Defines `Service`, `FilePicker`, `handle`. Declares 24 methods on `(s *Service)`. Connects desktop UI to coordinator | Post-core optional / Excluded from standalone core | High (heavy 1Scout platform coupling; not needed for standalone core) |

---

### 3.5 `go/deps/` (3 Files)

| File | Package | External Imports | 1Scout Imports | Role / Purpose | Checkpoint Mapping | Migration Risk |
|---|---|---|---|---|---|---|
| [`safe/safe.go`](../go/deps/safe/safe.go) | `safe` | None | None | Panic recovery utilities: `safe.Go`, `safe.Do`, `safe.Call` (100% stdlib: `log/slog`, `runtime/debug`) | Checkpoint A3 (Bundled Dependency) | Low |
| [`httpx/httpx.go`](../go/deps/httpx/httpx.go) | `httpx` | None | None | Hardened HTTP client: timeouts, proxy hook, User-Agent pool, DNS filtering (100% stdlib, requires Go 1.22+ for `math/rand/v2`) | Checkpoint A3 (Bundled Dependency) | Low |
| [`httpx/delivery.go`](../go/deps/httpx/delivery.go) | `httpx` | None | None | Custom dialer preventing private network SSRF/rebinding | Checkpoint A3 (Bundled Dependency) | Low |

---

### 3.6 `_reference/sitecrawl/tests/` (27 Files)

| Test File | Package | External Imports | 1Scout Direct Imports | Test Harness & Runtime Coupling | Test Classification | Checkpoint Mapping |
|---|---|---|---|---|---|---|
| [`agent_local_test.go`](../_reference/sitecrawl/tests/agent_local_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | In-memory SQLite search test and AST import validation | Pure/near-pure engine test | Checkpoint A6 |
| [`cancel_test.go`](../_reference/sitecrawl/tests/cancel_test.go) | `sitecrawl` | None | `core/runs` (`runs.Terminal`) | Crawl cancellation lifecycle; uses `newTestService`; contains `t.Skip` | Transitive test-harness dependency + direct import | Checkpoint A6 |
| [`crawler_test.go`](../_reference/sitecrawl/tests/crawler_test.go) | `sitecrawl` | None | `core/runs` (`runs.BOMUTF8`) | Core fixture crawl: facets, status codes, canonicals; uses `newTestService` | Transitive test-harness dependency + direct import | Checkpoint A6 |
| [`credentials_test.go`](../_reference/sitecrawl/tests/credentials_test.go) | `sitecrawl` | None | `core/credset`, `core/httpx` | PageSpeed API key storage vault tests | Direct import dependency (PageSpeed) | Checkpoint A7 |
| [`customheaders_test.go`](../_reference/sitecrawl/tests/customheaders_test.go) | `sitecrawl` | None | None | Custom HTTP headers forwarding during crawl; uses `newTestService` | Transitive test-harness dependency | Checkpoint A6 |
| [`deferred_test.go`](../_reference/sitecrawl/tests/deferred_test.go) | `sitecrawl` | None | None | 429 Too Many Requests & 503 retry/deferral loop; uses `newTestService` | Transitive test-harness dependency | Checkpoint A6 |
| [`extract_test.go`](../_reference/sitecrawl/tests/extract_test.go) | `sitecrawl` | None | None | HTML extraction: title, meta, canonical, hreflang, schema (100% stdlib) | Pure/near-pure engine test | Checkpoint A3 / A6 |
| [`fixture_test.go`](../_reference/sitecrawl/tests/fixture_test.go) | `sitecrawl` | None | None | Synthetic site fixture with redirect chains, loops, canonicals | Test fixture helper | Checkpoint A3 / A6 |
| [`frontier_cap_test.go`](../_reference/sitecrawl/tests/frontier_cap_test.go) | `sitecrawl` | None | None | `MaxURLs` admission boundary enforcement (100% stdlib) | Pure/near-pure engine test | Checkpoint A3 / A6 |
| [`frontier_test.go`](../_reference/sitecrawl/tests/frontier_test.go) | `sitecrawl` | None | None | FIFO queue, BFS depth order, tracking param stripping (100% stdlib) | Pure/near-pure engine test | Checkpoint A3 / A6 |
| [`fts_trigger_test.go`](../_reference/sitecrawl/tests/fts_trigger_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | SQLite FTS5 index update triggers; contains `t.Skip` if FTS5 missing | Pure/near-pure engine test | Checkpoint A6 |
| [`history_test.go`](../_reference/sitecrawl/tests/history_test.go) | `sitecrawl` | None | `core/jobs`, `core/workspace`, `testutil` | Defines `newTestService`; tests run history queries | Transitive test-harness definition + direct import | Checkpoint A6 |
| [`inspectdb_test.go`](../_reference/sitecrawl/tests/inspectdb_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | Manual crawl DB inspection utility; contains `t.Skip` (`CRAWL_DB=<path>`) | Environment-gated utility | Checkpoint A6 |
| [`inspectopts_test.go`](../_reference/sitecrawl/tests/inspectopts_test.go) | `sitecrawl` | `modernc.org/sqlite` | None | Manual options inspection utility; contains `t.Skip` (`CRAWL_DB=<path>`) | Environment-gated utility | Checkpoint A6 |
| [`live_test.go`](../_reference/sitecrawl/tests/live_test.go) | `sitecrawl` | None | `core/jobs`, `core/workspace` | Live web crawl test against external sites; contains `t.Skip` (`CRAWL_LIVE`) | Environment-gated + direct import | Checkpoint A6 |
| [`pagespeed_test.go`](../_reference/sitecrawl/tests/pagespeed_test.go) | `sitecrawl` | None | `core/credset`, `core/httpx` | PageSpeed pump, concurrency pacing, error handling; uses `newTestService` | Transitive test-harness dependency + direct import | Checkpoint A7 |
| [`pause_test.go`](../_reference/sitecrawl/tests/pause_test.go) | `sitecrawl` | None | None | Pause and resume lifecycle, frontier preservation; uses `newTestService`; contains `t.Skip` | Transitive test-harness dependency | Checkpoint A6 |
| [`perf_test.go`](../_reference/sitecrawl/tests/perf_test.go) | `sitecrawl` | None | None | High-concurrency synthetic crawl benchmark; uses `newTestService`; contains `t.Skip` (`CRAWL_PERF=1`) | Transitive test-harness dependency + environment-gated | Checkpoint A6 |
| [`politeness_test.go`](../_reference/sitecrawl/tests/politeness_test.go) | `sitecrawl` | None | None | Per-host pacing, backoff upon error, `Crawl-delay`; uses `newTestService` | Transitive test-harness dependency | Checkpoint A6 |
| [`psi_live_test.go`](../_reference/sitecrawl/tests/psi_live_test.go) | `sitecrawl` | None | None | Live Google PSI API request test; contains `t.Skip` (`PSI_LIVE_KEY=<key>`) | Environment-gated PageSpeed test | Checkpoint A7 |
| [`query_test.go`](../_reference/sitecrawl/tests/query_test.go) | `sitecrawl` | None | None | Grid query SQL construction and SQL injection safety; uses `newTestService` | Transitive test-harness dependency | Checkpoint A6 |
| [`render_test.go`](../_reference/sitecrawl/tests/render_test.go) | `sitecrawl` | None | None | Chrome/Edge headless rendering; uses `newTestService`; contains `t.Skip` if no browser | Transitive test-harness dependency + environment-gated | Checkpoint A6 |
| [`resume_ids_test.go`](../_reference/sitecrawl/tests/resume_ids_test.go) | `sitecrawl` | None | None | URL ID stability across crawl pause and restart; uses `newTestService` | Transitive test-harness dependency | Checkpoint A6 |
| [`schema_golden_test.go`](../_reference/sitecrawl/tests/schema_golden_test.go) | `sitecrawl` | None | `testutil` (`SchemaGolden`) | SQLite schema migration regression test against golden file | Direct import dependency | Checkpoint A6 |
| [`seedredirect_test.go`](../_reference/sitecrawl/tests/seedredirect_test.go) | `sitecrawl` | None | None | Initial seed URL 301/302 redirect resolution; uses `newTestService`; contains `t.Skip` | Transitive test-harness dependency | Checkpoint A6 |
| [`similarity_test.go`](../_reference/sitecrawl/tests/similarity_test.go) | `sitecrawl` | None | None | SimHash duplicate detection tests; uses `newTestService` | Transitive test-harness dependency | Checkpoint A6 |
| [`sitemap_test.go`](../_reference/sitecrawl/tests/sitemap_test.go) | `sitecrawl` | None | None | Sitemap index, gzip sitemaps, invalid XML tolerance (100% stdlib) | Pure/near-pure engine test | Checkpoint A3 / A6 |

#### Summary of Test Classifications (27 Files Total)
- **Pure / Near-Pure Engine & Fixture Unit Tests (7 files)**: `agent_local_test.go`, `extract_test.go`, `fixture_test.go`, `frontier_cap_test.go`, `frontier_test.go`, `fts_trigger_test.go`, `sitemap_test.go`.
- **Environment-Gated PageSpeed Integration Test (1 file)**: `psi_live_test.go`.
- **Transitive Test-Harness Dependencies without Direct 1Scout Imports (10 files)**: `customheaders_test.go`, `deferred_test.go`, `pause_test.go`, `perf_test.go`, `politeness_test.go`, `query_test.go`, `render_test.go`, `resume_ids_test.go`, `seedredirect_test.go`, `similarity_test.go`.
- **Direct 1Scout Import Dependencies with Transitive Test-Harness (5 files)**: `cancel_test.go`, `crawler_test.go`, `history_test.go`, `live_test.go`, `pagespeed_test.go`.
- **Direct 1Scout Import Dependencies without Test-Harness (2 files)**: `credentials_test.go`, `schema_golden_test.go`.
- **Manual DB Inspection Utilities (2 files)**: `inspectdb_test.go`, `inspectopts_test.go`.
*(Note: Exactly 10 test files contain `t.Skip` directives: 7 are environment/build/browser gates [`live_test.go`, `perf_test.go`, `inspectdb_test.go`, `inspectopts_test.go`, `psi_live_test.go`, `render_test.go`, `fts_trigger_test.go`], and 3 are timing/convergence skips [`cancel_test.go`, `pause_test.go`, `seedredirect_test.go`]).*

---

## 4. 1Scout Platform Dependency Contracts (Observable Requirements Only)

This section records the exact symbols used from unbundled 1Scout packages and the concrete behavior observable from SiteCrawl source code, without inventing unverified adapter implementations or internal details:

| Package | Importing Files | Exact Symbols / Types Used | Observable Behavior Required by SiteCrawl | Standalone Equivalent Needed? | Implementation Status |
|---|---|---|---|---|---|
| **`core/runs`** | `types.go`, `runs.go`, `export.go`, `service.go`, `cancel_test.go`, `crawler_test.go` | `runs.StateRunning`, `runs.StatePaused`, `runs.StateCompleted`, `runs.StateCancelled`, `runs.StateStopped`, `runs.StateFailed`, `runs.StateInterrupted`, `runs.Now()`, `runs.BOMUTF8`, `runs.CSVGuard(string)`, `runs.Gate`, `runs.Terminal(string)` | - Run state string constants.<br>- RFC3339 formatted UTC timestamp string.<br>- UTF-8 BOM byte sequence `[]byte{0xEF, 0xBB, 0xBF}` for CSV output.<br>- CSV formula character escaping.<br>- Boolean check whether a run state string is terminal.<br>- Per-workspace run synchronization gate in `service.go`. | **Yes (for acquisition core & persistence)** | Standalone equivalents needed for run-state constants (A3/A4), timestamps (A5), terminal state checks (A5/A6), and CSV helpers (A7). Implementation details = TBD. `runs.Gate` excluded with `service.go`. |
| **`core/schema`** | `runs.go`, `service.go` | `schema.Migrate(db *sql.DB, tool string, stmts []string) error`, `schema.Ready` | - In `runs.go`: Executes sequential SQLite DDL migration statements (`schemaStmts`) against `*sql.DB` ensuring statements are applied once.<br>- In `service.go`: Synchronization gate (`ready.Do(...)`) to avoid replaying DDL checks on every read action. | **Yes (for standalone persistence)** | Standalone mechanism needed in Checkpoint A5 to execute `schemaStmts` against `*sql.DB` when `storage/runs.go` is migrated. Implementation details = TBD. `schema.Ready` excluded with `service.go`. |
| **`core/workspace`** | `service.go`, `pagespeed.go`, `history_test.go`, `live_test.go` | `workspace.Manager`, `workspace.NewManager()`, `workspace.WithTx(ctx, db, ...)` | - In `service.go`: Resolves workspace directory and opens SQLite DB handle.<br>- In `pagespeed.go`: Wraps SQLite operations in transactions with backoff retry.<br>- In tests: Initializes a test workspace database. | **No (for standalone acquisition core)** | Excluded from acquisition core. The core crawler operates on standard `database/sql` handles (`*sql.DB`). Standalone tests require equivalent isolated database opener behavior (exact implementation = TBD). |
| **`core/jobs`** | `service.go`, `cancel_test.go`, `history_test.go`, `live_test.go` | `jobs.Runner`, `jobs.NewRunner(...)`, `Jobs.Start(...)`, `Jobs.Emit(...)`, `Jobs.Cancel(...)`, `jobs.EventName`, `jobs.Progress` | - In `service.go`: Desktop supervisor for background goroutines and Wails event emission.<br>- In tests: Used to construct test `Service` instances. | **No (for crawler core)** | Excluded. Do NOT port the 1Scout jobs framework. Coordinator cancellation uses standard `context.Context`. However, Checkpoint A4 acceptance requires progress/events to be observable, which requires an equivalent minimal event sink. |
| **`core/credset`** | `service.go`, `pagespeed.go`, `credentials_test.go`, `pagespeed_test.go` | `credset.Store`, `credset.First(...)`, `credset.Add(...)`, `credset.Save(...)` | - In `service.go` & `pagespeed.go`: Reads encrypted API keys for Google PageSpeed Insights and proxy configuration. | **No (for acquisition core)** | Excluded with `service.go` and `pagespeed/`. See Section 5 for the proxy boundary. |
| **`core/license`** | `service.go` | `license.Gate` | - Verifies commercial application license. | **No** | Excluded completely. Has zero function in the audit tool. |
| **`tools`** | `service.go` | `tools.Register(...)`, `tools.Factory`, `tools.Starter` | - 1Scout desktop plugin registry hook (`tools.Register(tools.Factory{...})`). | **No** | Excluded with `service.go`. Standalone tool uses CLI or Go API. |
| **`testutil`** | `history_test.go`, `schema_golden_test.go` | `testutil.RedirectConfigDir(t *testing.T)`, `testutil.SchemaGolden(t *testing.T, tool string, stmts []string)` | - `RedirectConfigDir`: Redirects config directory to temporary directory.<br>- `SchemaGolden`: Asserts that DDL output matches golden schema file. | **No** | Standalone tests require equivalent isolated configuration-directory behavior and schema regression verification (exact implementation = TBD). |
| **`core/safe`** *(bundled)* | `sitemap.go`, `crawler.go`, `pagespeed.go`, `pagespeed_pump.go` | `safe.Do()`, `safe.Call()`, `safe.Go()` | - Panic recovery wrapper for worker goroutines. | **Yes (Preserved)** | Source code is bundled in `go/deps/safe/safe.go`. |
| **`core/httpx`** *(bundled)* | `fetch.go`, `crawler.go`, `service.go`, `pagespeed.go`, `credentials_test.go`, `pagespeed_test.go` | `httpx.NewFiltered(...)`, `httpx.ProxyFunc`, `httpx.IsLocalName`, `httpx.AllowPrivate` | - HTTP client with safe dialing, DNS checks, and proxy rotation hooks. | **Yes (Preserved)** | Source code is bundled in `go/deps/httpx/`. |

---

## 5. Source Proxy Boundary Reality

Inspection of `go/storage/crawler.go` (lines 198–200) and `go/app-glue/service.go` (lines 109–158) confirms:
1. `Options.UseProxy` is purely a boolean flag indicating whether proxying is enabled for the run.
2. `newCoordinator(...)` receives the actual proxy implementation as an argument of type:
   ```go
   proxy httpx.ProxyFunc
   ```
3. In 1Scout, `service.go` queries `credset.Store` for saved proxy settings and constructs the `httpx.ProxyFunc` callback passed into `newCoordinator`.
4. Therefore, `credset` is properly excluded from the crawler core; however, preserving proxy capabilities in standalone mode requires an optional standalone host/provider capable of supplying `httpx.ProxyFunc` (e.g. from environment variables `HTTP_PROXY`/`HTTPS_PROXY` or CLI flags). `Options.UseProxy` alone does not supply proxy resolution.

---

## 6. Compile-Time Symbol Dependency Closures & Cross-File Knots

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

## 7. Migration Checkpoint Reconciliation & Sequencing Conflicts

Reconciling the source code dependencies with [`sitecrawl-standalone-migration-map.md`](sitecrawl-standalone-migration-map.md):

### 7.1 Checkpoint Sequence per Approved Migration Map
- **Coordinated Checkpoint A3 / A4 — Reconstruct `sitecrawl` Engine & Minimal Platform Primitives**:
  - Target: `internal/sitecrawl/`
  - Intended scope: 13 engine mechanics files (`normalize`, `frontier`, `politeness`, `robots`, `sitemap`, `fetch`, `extract`, `links`, `render`, `exclusions`, `similarity`, `types`, `useragents`), bundled `safe` and `httpx`, minimum run-state contract in `types.go`, standalone DB opener primitive, `context.Context` cancellation primitive, and minimal progress/event sink primitives.
  - Acceptance: `internal/sitecrawl` engine package compiles; portable engine tests pass; bundled utilities resolve without 1Scout; minimal host primitives compile and are testable; no temporary issue stubs exist.
- **Checkpoint A5 — Port Coordinator, Persistence & Page Assembly**:
  - Target: `storage/crawler.go`, `storage/persist.go`, `storage/runs.go`, `engine/page.go`.
  - Intended scope: Full crawl coordinator loop, run persistence, SiteCrawl schema migration (`schemaStmts`), page projection, fixture crawl execution, and cancellation/progress integration.
  - Acceptance: Coordinator can start and finish a crawl; local SQLite can persist one run; context cancellation works through coordinator; progress/events can be observed during a crawl; fixture site can be crawled end-to-end; results survive process-level readback.
- **Checkpoint A6 — Regression Parity & Full Module Gate**:
  - Port applicable tests, establish regression parity, satisfy full module verification (`go list ./...`), create freeze tag `sitecrawl-standalone-core-v1`.
- **Checkpoint A7 — Optional Compatibility Features**:
  - Migrate legacy issues (`issues.go`), post-crawl finalize (`finalize.go`), duplicates (`duplicates.go`), exports (`export.go`), query (`query.go`), graph (`graph.go`), and PageSpeed (`pagespeed/`). Post-core optional desktop UI integration remains excluded from the acquisition core.

### 7.2 The Explicit Sequencing Conflicts

#### Conflict 1: Checkpoint A2 Feasibility Conflict
`sitecrawl-standalone-migration-map.md` defines Checkpoint A2 acceptance as:
> `go list ./...` must enumerate intended standalone packages without relying on the 1Scout repository.

However, in the current repository:
1. All 58 Go files reside in subdirectories under `go/`.
2. 16 of these files contain import statements pointing to `onescout/desktop/internal/...`.
3. If `go.mod` is created at the repository root and `go list ./...` is executed:
   - Go attempts to load every package in every subdirectory.
   - Go fails immediately with unresolvable package errors for `onescout/desktop/internal/...`.
   - Therefore, `go.mod` + external dependencies alone cannot satisfy `go list ./...` against the current un-migrated source tree.

#### Conflict 2: Checkpoint A3 / A4 Packaging Conflict
Checkpoint A3 acceptance states:
> Core package compiles; portable engine tests pass.

However, the prioritized A3 file set contains compile-time dependencies that cross into later checkpoints:
1. `engine/types.go` (A3) directly imports unbundled `onescout/desktop/internal/core/runs` (scheduled for adapter replacement in A4).
2. `engine/page.go` (A3) requires `nowStamp()` (located in `storage/runs.go` — A5) and `issue`, `severityOf`, `countMissingAlt` (located in `engine/issues.go` — A7).
3. Therefore, A3 cannot compile in isolation without either:
   - bringing forward minimal adapter/compatibility definitions into A3;
   - combining selected A3/A4 work; or
   - revising the A3 package boundary.

---

## 8. Build Metadata & Runtime Constraints

- **Minimum Go Version**: **Go 1.22+**. Proven by standard library import `"math/rand/v2"` in `go/deps/httpx/httpx.go` line 11.
- **Pure Go / Zero CGO**: Verified zero `import "C"` calls. Tests use `modernc.org/sqlite`, allowing compilation and testing with `CGO_ENABLED=0`.
- **Zero Build Tags**: Verified zero `//go:build` or `// +build` directives across all 58 files.
- **External Go Modules & Timing**:
  1. `golang.org/x/net` (specifically `golang.org/x/net/html`) — introduced in A3 when `extract.go` is migrated.
  2. `github.com/temoto/robotstxt` — introduced in A3 when `robots.go` is migrated.
  3. `github.com/chromedp/chromedp` — introduced in A3 when `render.go` is migrated.
  4. `modernc.org/sqlite` — introduced in A4/A5 when active persistence and test harnesses require SQLite.
  *(Note: External dependencies are introduced just-in-time when active migrated packages require them, not predeclared in A2. `github.com/wailsapp/wails/v3` is only needed if desktop `service.go` is compiled; it is excluded from the standalone acquisition core).*
- **Environment & Runtime Skips (`t.Skip` across 10 files)**:
  - **Environment / Build / Browser Gates (7 files)**:
    - `CRAWL_LIVE=<url>`: in `go/tests/live_test.go` (live network probe)
    - `PSI_LIVE_KEY=<key>`: in `go/tests/psi_live_test.go` (live PageSpeed API call & CrUX check)
    - `CRAWL_PERF=1`: in `go/tests/perf_test.go` (high-throughput benchmark)
    - `CRAWL_DB=<path>`: in `go/tests/inspectdb_test.go` and `go/tests/inspectopts_test.go` (manual DB inspection)
    - Browser presence: `go/tests/render_test.go` (skips if Chrome/Edge binary is missing)
    - Build capability: `go/tests/fts_trigger_test.go` (skips if SQLite FTS5 capability is missing)
  - **Timing & Convergence Skips (3 files)**:
    - Cancellation race: `go/tests/cancel_test.go` (skips if fixture crawl finishes before cancel lands)
    - Pause/drain race: `go/tests/pause_test.go` (skips if crawl finishes/queue drains before pause lands)
    - Convergence state: `go/tests/seedredirect_test.go` (skips if run ends before convergence assertions)

---

## 9. Required Conclusions

### 1. What prevents the extracted repository from building standalone today?
- Absence of a `go.mod` file defining module boundaries and external dependency versions.
- 16 files importing `onescout/desktop/internal/...` (12 files importing unbundled platform packages).
- Undeclared external modules (`x/net/html`, `robotstxt`, `chromedp`, `modernc.org/sqlite`).
- Missing compile-time closure when directories are partitioned (e.g. `page.go` needing symbols from `runs.go` and `issues.go`).

### 2. Which files form the smallest useful acquisition core?
- Per the coordinated A3/A4 milestone: the core engine mechanics files in `go/engine/` (`normalize.go`, `frontier.go`, `politeness.go`, `robots.go`, `sitemap.go`, `fetch.go`, `extract.go`, `links.go`, `render.go`, `exclusions.go`, `similarity.go`, `types.go`, `useragents.go`) plus the bundled dependencies in `go/deps/` (`safe.go`, `httpx.go`, `delivery.go`).
- *Note*: `page.go` is deferred to Checkpoint A5 because its current implementation couples page assembly and projection to `nowStamp()` (in `storage/runs.go`) and legacy issue semantics (`issue`, `severityOf`, `countMissingAlt` in `engine/issues.go`). This sequencing adjustment preserves the clean engine boundary without inventing fake issue stubs.

### 3. Which 1Scout dependencies block that core?
- For the A3 engine core: only `core/runs` (status string constants in `types.go`). `core/httpx` and `core/safe` are already bundled in `go/deps/`.
- For A5 coordinator + persistence: `core/schema` (`schema.Migrate`) and `core/runs` (`runs.Now()`).
- All other platform dependencies (`core/workspace`, `core/jobs`, `core/credset`, `core/license`, `tools`) belong to `service.go` and `pagespeed/`, which are outside the acquisition core.

### 4. What minimal standalone adapters will eventually be required?
- **Run State Constants (A3/A4)**: Standalone run-state string constants required by `types.go`.
- **Database Lifecycle & Event Host Primitives (A3/A4)**: Minimal isolated SQLite database lifecycle/opener primitive for tests, `context.Context` cancellation primitive, and event sink primitives for observing crawl progress/cancellation.
- **Internal Module Path Redirects (A3/A4)**: Updating imports for `safe` and `httpx` from `onescout/desktop/internal/core/...` to local module paths.
- **Run Persistence & Timestamp Adapter (A5)**: Standalone run lifecycle persistence and timestamp generation (`nowStamp()`, `runs.Now()`) required by `storage/runs.go` and `engine/page.go`.
- **SiteCrawl Schema Migration (A5)**: Standalone migration runner executing `schemaStmts` against `*sql.DB` when `storage/runs.go` is migrated to initialize crawl persistence.

### 5. Which dependencies/features can be deferred?
- **Google PageSpeed Insights (`pagespeed/`)**: Deferred to Checkpoint A7.
- **Wails Desktop Service (`app-glue/service.go`)**: Excluded from the standalone core (post-core optional).
- **Platform Licensing & Jobs (`license`, `jobs`, `tools`, `credset`)**: Excluded completely.

### 6. Which package-private dependencies make early package splitting unsafe?
- The extensive sharing of unexported structs and functions across `crawler.go`, `page.go`, `frontier.go`, `fetch.go`, `extract.go`, `persist.go`, and `runs.go` makes splitting `sitecrawl` into subpackages (`crawler/frontier`, `crawler/fetch`, etc.) hazardous. The unified `internal/sitecrawl/` package structure must be preserved.

### 7. What exact scope should Checkpoint A2 (`go.mod`) contain?
- Declare the standalone module path: `github.com/truthrive/technical-seo-audit`.
- Declare `go 1.22`.
- Avoid temporary `replace` directives or stub modules.
- Use scoped verification (`go list ./go/deps/...`) rather than raw `./...` across legacy directories.
- Introduce external dependencies just-in-time as migrated packages require them, rather than predeclaring them in A2.

### 8. What should explicitly NOT be done in A2/A3?
- Do NOT implement any of the 47 `AR-*` Audit V1 rules.
- Do NOT rewrite or redesign the crawler loop, frontier, or HTTP fetching.
- Do NOT redesign the `Page` struct.
- Do NOT split `sitecrawl` into multiple subpackages.
- Do NOT attempt to port 1Scout desktop frameworks (`jobs.Runner`, `license.Gate`, `tools.Register`, Wails v3).
- Do NOT delete legacy issues logic (`issues.go`) prematurely.

---

## 10. Approved Sequencing Decisions & Open Decision Gates

Following Checkpoint A1 review, the sequencing decisions for Checkpoints A2–A4 are officially approved. Decisions for Checkpoint A5 and later remain intentionally open.

### Approved Decision 1: Checkpoint A2 Scoped Module Bootstrap
- **Strategy**: A2 uses **scoped verification**.
- **Module Identity**: Root `go.mod` declared with module path `github.com/truthrive/technical-seo-audit` and `go 1.22`.
- **No Temporary Replace Directives**: Avoid `replace onescout/desktop/internal/... => ...` directives or stub module trees.
- **Scoped Verification Target**: Initial A2 acceptance is verified via `go list ./go/deps/...`, verifying only genuine standalone packages without failing on un-migrated legacy directories.
- **Just-In-Time External Dependencies**: Do not predeclare external modules (`x/net/html`, `robotstxt`, `chromedp`, `modernc.org/sqlite`) in A2; introduce them when migrated active packages actually require them.
- **Temporary Scope**: Scoped verification is explicitly temporary. By Checkpoint A6 (Standalone Core Freeze), the active repository must satisfy full repository `go list ./...`.

### Approved Decision 2: Coordinated A3/A4 Migration Milestone
- **Strategy**: A3 (`internal/sitecrawl/` reconstruction) and A4 (standalone platform adapters) retain their conceptual responsibilities but will be executed as **one coordinated migration milestone**. This ensures package reconstruction and minimal real platform contracts land together without broken intermediate states.
- **Initial A3 Engine Boundary**: Prioritize the 13 engine mechanics files (`normalize.go`, `frontier.go`, `politeness.go`, `robots.go`, `sitemap.go`, `fetch.go`, `extract.go`, `links.go`, `render.go`, `exclusions.go`, `similarity.go`, `types.go`, `useragents.go`) plus bundled `safe` and `httpx`.
- **`page.go` Deferred to A5**: `page.go` is deferred to Checkpoint A5 because current source couples page projection to `nowStamp()`, `issue`, `severityOf`, and `countMissingAlt`. This is a sequencing adjustment, not a redesign of `Page`.
- **No Fake Issue Stubs**: Do NOT introduce temporary fake/stub definitions for `issue`, `severityOf`, `countMissingAlt`, or legacy issue evaluation.
- **Minimal Real Platform Primitives (A4 Responsibility)**: Introduce only real contracts and primitives required by migrated code: minimal run-state constants in `types.go`, standalone SQLite database lifecycle/opener primitive, `context.Context` cancellation primitive, and minimal progress/event interfaces or sink primitives. External dependencies required by the engine (`golang.org/x/net/html`, `github.com/temoto/robotstxt`, `github.com/chromedp/chromedp`) are added. Do NOT port the 1Scout jobs framework.
- **No Premature End-to-End Crawl**: A3/A4 must NOT require an end-to-end crawl, SiteCrawl run persistence, or `schemaStmts` execution yet (deferred to A5 with `storage/runs.go`).
- **Coordinated A3/A4 Acceptance Criteria**:
  1. `internal/sitecrawl` engine package compiles.
  2. Migrated portable engine tests pass.
  3. Local bundled dependencies resolve without 1Scout.
  4. Minimal host/platform primitives compile and are independently testable.
  5. No temporary legacy-issue stubs exist.

### Approved Checkpoint A5 Scope & Acceptance (Coordinator, Persistence & Page Assembly)
- **Scope**: Port `storage/crawler.go`, `storage/persist.go`, `storage/runs.go`, and `engine/page.go`. Owns SiteCrawl schema initialization (`schemaStmts`), run lifecycle persistence, timestamp generation (`nowStamp()` / `runs.Now()`), coordinator integration, fixture crawl execution, and cancellation/progress integration through the A4 primitives.
- **A5 Acceptance Criteria**:
  1. Coordinator can start and finish a crawl.
  2. Local SQLite can persist one run.
  3. Context cancellation works through the coordinator.
  4. Progress/events can be observed during a crawl.
  5. Fixture site can be crawled end-to-end.
  6. Results survive process-level readback.

### Full Module Gate by Checkpoint A6
- Scoped verification is temporary and must not become a permanent exception.
- By the Checkpoint A6 Standalone Core Freeze, the repository must pass full module verification (`go list ./...`).
- Legacy extracted Go reference directories under `go/` must not permanently poison module traversal. Once their required behavior and source have been preserved into `internal/sitecrawl/`, they may be migrated, removed after parity, or isolated from active module traversal.
- The immutable Git tag `sitecrawl-pre-audit-v1` remains the permanent, recoverable reference.

### Later Open Decision Gates (Intentionally Deferred)

The following decisions remain intentionally open and will be resolved during their respective checkpoints:

1. **Checkpoint A5 PageSpeed Compile-Time Decoupling Strategy**:
   - *Option A*: Preserve PSI types inside `internal/sitecrawl/` temporarily alongside `crawler.go`.
   - *Option B*: Decouple coordinator via an internal interface / null object collector pattern (`type psiCollector interface { ... }`).
   - *Status*: Open. To be decided during Checkpoint A5.

2. **Later Service Receiver Decoupling Strategy**:
   - *Option A*: Refactor receiver methods on `(s *Service)` in `issues.go`, `query.go`, `export.go`, and `graph.go` into functional/repository signatures taking `*sql.DB`.
   - *Option B*: Define a minimal standalone `type Service struct { DB *sql.DB }` facade in `internal/sitecrawl/`.
   - *Status*: Open. To be decided during Checkpoint A7 / UI integration.
