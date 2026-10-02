# SiteCrawl Standalone Migration Map

**Status:** Approved (Sequencing updated post-A1)
**Repository:** `truthrive/technical-seo-audit`  
**Baseline tag:** `sitecrawl-pre-audit-v1`  
**Knowledge baseline:** v1.4  
**Primary goal:** Turn the extracted SiteCrawl module into a buildable, testable standalone acquisition foundation for Technical Audit V1, while preserving a clean path to merge the finished module back into 1Scout.

---

## 1. Context

The current repository contains an extracted reference copy of SiteCrawl from the larger 1Scout application.

The source itself states that:

- SiteCrawl was originally implemented as one `sitecrawl` package inside 1Scout;
- the current repository splits files into `go/engine`, `go/storage`, `go/pagespeed`, `go/app-glue`, and `go/tests` only for readability;
- several 1Scout platform packages are intentionally not included;
- the extracted repository therefore is not expected to build standalone in its current form.

This is not considered a defect in the original SiteCrawl implementation.

The migration goal is not to rebuild SiteCrawl from scratch.

The goal is:

```text
1Scout SiteCrawl reference
        ↓
standalone buildable acquisition core
        ↓
Technical Audit domain + Evidence Adapter
        ↓
47 AR-* audit rules
        ↓
stable standalone Technical Audit
        ↓
merge back into 1Scout through adapters
```

---

## 2. Migration Objective

Create a standalone development environment that:

1. preserves proven SiteCrawl acquisition behavior;
2. removes direct runtime dependency on the wider 1Scout application;
3. does not redesign the crawler while it is being extracted;
4. keeps legacy SEO issue behavior available only as compatibility/reference behavior;
5. creates a clean boundary for the new Technical Audit architecture;
6. remains easy to integrate back into 1Scout later.

The standalone migration is a **platform extraction task**, not yet an Audit V1 implementation task.

---

## 3. Non-Goals of the Standalone Migration

Do not use this migration to:

- implement the 47 `AR-*` rules;
- redesign `Page`;
- replace the legacy issue engine;
- redesign SiteCrawl UI;
- introduce a new SEO/GEO scoring model;
- refactor every Go file into a new package architecture;
- rewrite HTTP fetching;
- rewrite frontier logic;
- rewrite robots or sitemap behavior;
- rewrite rendering;
- port the entire 1Scout platform;
- add GSC, Bing, logs, analytics, or paid SEO APIs.

Those changes belong to later Technical Audit phases.

---

## 4. Current Repository State

Current Git history is intentionally clean:

```text
sitecrawl-pre-audit-v1
        │
        ▼
initial SiteCrawl baseline
        │
        ▼
knowledge v1.4
```

The baseline tag must remain unchanged throughout migration.

The extracted source currently contains:

```text
go/
├── engine/
├── storage/
├── pagespeed/
├── app-glue/
├── tests/
└── deps/

ui/
├── site-crawl/
└── bindings/

knowledge/
```

All Go source files originate from a package that was originally flat inside 1Scout.

Therefore the current directory split must not be interpreted as the intended standalone Go package structure.

---

## 5. Key Migration Decision

### Decision

Do **not** immediately split SiteCrawl into many new Go packages.

The current engine, storage, and related files rely heavily on package-private functions, types, and constants.

Trying to redesign them into:

```text
crawler/fetch
crawler/frontier
crawler/robots
crawler/sitemap
...
```

during extraction would combine two independent risks:

```text
platform extraction
+
architecture refactor
```

That would make regressions difficult to diagnose.

### Recommended first standalone shape

Reconstruct a buildable package close to the original package boundary:

```text
internal/sitecrawl/
```

and keep most migrated SiteCrawl files in that package until behavioral parity is established.

Refactoring into smaller packages may happen later if it provides a clear benefit.

---

## 6. Recommended Target Repository Shape

Initial target:

```text
technical-seo-audit/
├── go.mod
│
├── cmd/
│   └── sitecrawl-dev/
│       └── main.go
│
├── internal/
│   ├── sitecrawl/
│   │   ├── types.go
│   │   ├── normalize.go
│   │   ├── frontier.go
│   │   ├── politeness.go
│   │   ├── robots.go
│   │   ├── sitemap.go
│   │   ├── fetch.go
│   │   ├── render.go
│   │   ├── extract.go
│   │   ├── page.go
│   │   ├── links.go
│   │   ├── exclusions.go
│   │   ├── similarity.go
│   │   │
│   │   ├── crawler.go
│   │   ├── persist.go
│   │   ├── runs.go
│   │   ├── finalize.go
│   │   └── ...
│   │
│   └── platform/
│       └── standalone/
│           ├── database.go
│           ├── jobs.go
│           ├── events.go
│           └── config.go
│
├── tests/
│
├── knowledge/
├── docs/
└── ui/                     # migrated only after core is stable
```

This is a migration target, not a permanent architecture commitment.

---

## 7. Module Classification

### 7.1 Engine — Preserve / Reuse

| Current module | Migration class | Reason |
|---|---|---|
| `normalize.go` | Reuse with review | Mature URL identity logic; normalization assumptions must later be separated from Audit semantics |
| `frontier.go` | Reuse | BFS, URL dictionary, admission limits and checkpoint concepts are valuable |
| `politeness.go` | Reuse | Per-host pacing/backoff is acquisition behavior |
| `robots.go` | Reuse | Strong crawler infrastructure; later extended for Audit bot policies |
| `sitemap.go` | Reuse | Sitemap discovery/index handling already exists |
| `fetch.go` | Reuse | Redirects, retry, network errors and body limits are core evidence acquisition |
| `extract.go` | Reuse | Existing HTML tokenizer supplies many V1 observations |
| `links.go` | Reuse with future adapter | Strong link evidence but discovery semantics require later audit-specific separation |
| `render.go` | Reuse with later refactor | Browser discovery/render mechanics useful; raw/render evidence must later be separated |
| `exclusions.go` | Preserve initially | Existing SiteCrawl behavior; not Audit V1 authority |
| `similarity.go` | Preserve / defer | Useful legacy capability but not required by frozen Audit V1 |

### 7.2 Engine — Legacy Audit Semantics

| Current module | Migration class | Rule |
|---|---|---|
| `issues.go` | Legacy compatibility only | Must not become Audit V1 authority |
| indexability logic | Legacy compatibility only | Must not be mapped directly to `AR-*` outcomes |
| legacy severity constants | Legacy compatibility only | Audit V1 uses its own P0/P1/P2/P3 contract |

The migration must preserve these only if required for SiteCrawl compatibility.

They must not be used to define new Technical Audit behavior.

---

## 8. `Page` Migration Policy

The existing `Page` structure contains a mixture of:

```text
raw/parsed observations
derived crawl state
legacy SEO conclusions
presentation fields
```

Examples include technical fields such as:

- status;
- title;
- canonical;
- robots;
- headings;
- links;

alongside legacy conclusions such as:

- indexable;
- indexability reason;
- issues.

### Migration rule

Do **not** redesign `Page` during standalone extraction.

Checkpoint A should preserve it sufficiently to keep existing crawler behavior and tests understandable.

Later Technical Audit development introduces:

```text
SiteCrawl Page / stored crawl records
            ↓
Evidence Adapter
            ↓
Audit normalized evidence
```

The Evidence Adapter, not `Page`, becomes the migration boundary toward the frozen Audit V1 data model.

---

## 9. Discovery Semantics Risk

Current SiteCrawl has discovery-source concepts including:

```text
seed
link
sitemap
redirect
manual
```

This is appropriate for the existing crawler.

Technical Audit V1, however, requires a stronger distinction:

```text
legitimate website discovery
            ≠
evidence-only acquisition
```

Future evidence-only examples include:

- canonical target checks;
- redirect target continuation;
- known-missing URL probes;
- host/protocol probes;
- slash variants;
- bot-profile requests.

### Migration rule

Do not solve this during the standalone extraction.

Preserve current behavior first.

Document every future insertion point where an Evidence Probe Planner must bypass normal DiscoveryRecord creation.

This becomes part of the Evidence Adapter / Audit acquisition phase after standalone parity.

---

## 10. Renderer Migration Risk

The current renderer is valuable because it already:

- locates an installed Chrome/Edge browser;
- uses `chromedp`;
- limits render volume;
- respects configured render concurrency.

However the Technical Audit architecture requires:

```text
raw evidence
+
rendered evidence
+
field comparison
```

rather than a single post-render page state.

### Standalone migration

Preserve current renderer mechanics.

### Later Audit refactor

Modify the acquisition contract so rendering produces a separate observation instead of destroying or replacing the raw observation required by:

```text
AR-RENDER-001
...
AR-RENDER-006
```

Do not combine this refactor with standalone extraction.

---

## 11. Storage Classification

### `crawler.go`

**Reuse with platform extraction.**

High-value behavior includes:

- single coordinator / single writer;
- worker isolation;
- batching;
- pause/resume;
- 429/503 deferral;
- crawl state;
- frontier checkpoints;
- render coordination.

The coordinator is one of the strongest parts of the current implementation and should not be rewritten without evidence.

### `persist.go`

**Reuse with local database adaptation.**

Keep batching/persistence mechanics where possible.

Remove assumptions tied specifically to the 1Scout workspace layer.

### `runs.go`

**Refactor for standalone run storage.**

The standalone application needs local run lifecycle persistence without depending on 1Scout's shared run infrastructure.

### `finalize.go`

**Mixed.**

Reusable computations:

- inlink counts;
- graph facts;
- canonical graph traversal;
- other crawl-wide observations.

Legacy behavior to isolate:

```text
calculation
→ directly insert legacy issue
```

Future Audit architecture requires:

```text
calculation
→ normalized evidence
→ AR-* rule evaluation
```

Do not convert finalize logic during standalone extraction.

Mark legacy issue writes as compatibility behavior.

### `query.go`, `graph.go`, `export.go`

**Defer until a standalone/report UI needs them.**

They are product presentation infrastructure, not required to prove acquisition parity.

### `duplicates.go`

**Defer / compatibility module.**

Not required by frozen Technical Audit V1.

### `agent_local.go`

**Defer.**

Not required for the standalone crawler or V1 audit engine.

---

## 12. 1Scout Dependency Map

The extracted SiteCrawl currently references wider 1Scout platform responsibilities.

These responsibilities should be replaced by **minimal standalone adapters**, not by copying the whole 1Scout core.

### 12.1 Run state / history

Current responsibility:

```text
core/runs
```

Standalone replacement:

```text
local run state constants
+
standalone run repository
```

Do not make the Audit domain depend on 1Scout run types.

### 12.2 Workspace / database ownership

Current responsibility:

```text
core/workspace
```

Standalone replacement:

```text
database path/config
+
direct SQLite lifecycle
```

The standalone application needs only enough abstraction to create/open its own database.

### 12.3 Schema management

Current responsibility:

```text
core/schema
```

Standalone replacement:

```text
local deterministic migrations
```

Do not port unrelated shared 1Scout schemas.

### 12.4 Jobs / progress / events

Current responsibility:

```text
core/jobs
```

Standalone replacement should expose only what SiteCrawl needs:

```text
context cancellation
progress callback
event sink
job state
```

Suggested conceptual interfaces:

```text
ProgressReporter
EventSink
JobController
```

Do not reproduce the entire 1Scout job framework.

### 12.5 Credentials

Current responsibility:

```text
core/credset
```

This is primarily relevant to optional external integrations such as PageSpeed.

Standalone Audit V1 does not require PageSpeed.

Therefore credential migration should not block the core crawler.

### 12.6 License

Current responsibility:

```text
core/license
```

Standalone development does not need 1Scout license enforcement.

Keep licensing outside the crawler/audit domain.

When merged back into 1Scout, the host application can reapply its license gate.

### 12.7 Tool registry

Current responsibility:

```text
tools
```

Not required in a standalone repository.

The standalone executable itself is the selected tool.

---

## 13. Included Dependencies

The extracted repository already includes utility code equivalent to:

```text
safe
httpx
```

These should be migrated into the standalone module under local import paths.

Avoid unnecessary rewrites before tests establish parity.

External libraries used by the source include:

```text
golang.org/x/net/html
github.com/temoto/robotstxt
github.com/chromedp/chromedp
```

Wails should not be required to prove the standalone crawl core.

---

## 14. PageSpeed Migration

PageSpeed is a mature SiteCrawl feature, but it is not required by the frozen Technical Audit V1 runtime.

Therefore:

### Checkpoint A

Do not let PageSpeed block standalone crawler compilation.

Either:

- temporarily exclude PageSpeed from the first buildable core; or
- preserve it behind an optional adapter if this requires little work.

### Later

Reintroduce PageSpeed as a compatibility/optional feature only after the standalone crawl engine is stable.

PageSpeed must not become a dependency of the Audit Rule Engine.

---

## 15. UI Migration

The existing UI is not self-contained.

It relies on:

- Wails-generated bindings;
- shared application components;
- wider 1Scout UI infrastructure.

Therefore the first standalone checkpoint should **not** attempt to reconstruct the complete React UI.

### Recommended temporary development interface

Create a minimal executable:

```text
cmd/sitecrawl-dev
```

capable of:

```text
input start URL
→ execute crawl
→ write/read local crawl data
→ print basic run summary
```

JSON output is sufficient.

This gives developers an end-to-end executable without coupling migration to UI work.

### UI migration gate

Begin standalone UI work only after:

```text
crawler compiles
crawler tests pass
fixture crawl passes
storage works
pause/cancel behavior is stable
```

---

## 16. Test Migration Strategy

Existing tests are a critical migration asset.

### Tier 1 — Pure/near-pure engine tests

Prioritize tests for:

- URL normalization;
- frontier;
- extraction;
- politeness;
- sitemap;
- similarity;
- redirects;
- rendering where browser availability permits.

These should be migrated with minimal semantic change.

### Tier 2 — Coordinator integration tests

Port tests that exercise:

- fixture crawl;
- 429/503 deferral;
- pause/resume;
- cancellation;
- frontier limits;
- seed redirects;
- storage behavior.

Replace the current 1Scout-specific test service setup with a standalone SQLite/test harness.

### Tier 3 — Legacy issue regression tests

Keep these as **compatibility tests**, not Audit V1 tests.

Example:

```text
legacy noindex issue still behaves like old SiteCrawl
```

does not imply:

```text
AR-INDEX-* must produce the same verdict
```

The two systems may intentionally differ.

### Tier 4 — Live tests

Keep environment-gated behavior.

Live-network tests must not become normal CI requirements.

---

## 17. Recommended Migration Checkpoints

### Checkpoint A0 — Preserve Baseline

Already complete.

Requirements:

```text
baseline commit exists
baseline tag exists
knowledge v1.4 exists
```

No baseline history rewriting.

### Checkpoint A1 — Buildability Inventory

**Status:** Completed and Approved.

Deliverable:

```text
docs/sitecrawl-standalone-buildability.md
```

Inventory completed with zero code changes: 58 Go files cataloged, 16 files with `onescout/...` imports, compile-time coupling knots documented, proxy boundary corrected, behavioral contracts defined, and sequencing conflicts resolved into approved execution decisions.

### Checkpoint A2 — Create Standalone Go Module (Scoped Bootstrap)

Create root:

```text
go.mod
```

- **Module Path**: `github.com/truthrive/technical-seo-audit`
- **Go Version**: `go 1.22`
- **Scoped Verification Strategy**: Initial acceptance verifies only genuinely standalone packages without failing on un-migrated legacy directories:
  ```text
  go list ./go/deps/...
  ```
- **No Temporary Replace Directives**: Avoid `replace onescout/desktop/internal/... => ...` directives or stub modules.
- **No Premature Dependency Predeclaration**: Do not require `go mod tidy` across legacy files, and do not predeclare external modules (`x/net/html`, `robotstxt`, `chromedp`, `modernc.org/sqlite`) before migrated active packages actually require them.
- **Temporary Scope**: Scoped verification is explicitly temporary. Full module verification (`go list ./...`) is enforced at Checkpoint A6.

### Coordinated Checkpoint A3 / A4 — Reconstruct `sitecrawl` Package & Minimal Platform Contracts

Checkpoints A3 and A4 retain their conceptual responsibilities but are executed as **one coordinated migration milestone** so package reconstruction and minimal real platform contracts land together without broken intermediate states.

**Target Package**:

```text
internal/sitecrawl/
```

#### Initial A3 Engine Boundary

Move and adapt the core engine mechanics files that have clean boundaries:

```text
normalize.go
frontier.go
politeness.go
robots.go
sitemap.go
fetch.go
extract.go
links.go
render.go
exclusions.go
similarity.go
types.go
useragents.go
go/deps/safe/
go/deps/httpx/
```

- **`page.go` Deferred to A5**: `page.go` is intentionally deferred to Checkpoint A5 because current source couples page projection to `nowStamp()` (in `runs.go`) and legacy issue semantics (`issue`, `severityOf`, `countMissingAlt` in `issues.go`). This is a sequencing adjustment, not a redesign of `Page`.
- **No Fake Issue Stubs**: Do NOT introduce temporary fake/stub definitions for `issue`, `severityOf`, `countMissingAlt`, or legacy issue evaluation.
- **External Dependencies**: Introduce `golang.org/x/net/html`, `github.com/temoto/robotstxt`, and `github.com/chromedp/chromedp` as required by the migrated engine files.

#### Minimal Real Platform Contracts (A4 Responsibility)

During the coordinated milestone, introduce only the real primitives and contracts required by migrated code:

- Run state string constants and timestamp generation required by `types.go`.
- Standalone SQLite database lifecycle/opener primitive (`modernc.org/sqlite`).
- Cancellation primitive based on standard `context.Context`.
- Minimal progress reporter and event sink primitives/interfaces.
- External dependencies required by the migrated engine (`golang.org/x/net/html`, `github.com/temoto/robotstxt`, `github.com/chromedp/chromedp`).
- Do NOT port the 1Scout jobs framework.
- Does NOT require an end-to-end crawl, SiteCrawl run persistence, or `schemaStmts` execution yet (deferred to A5 with `storage/runs.go`).

**Coordinated Milestone Acceptance**:

```text
internal/sitecrawl engine package compiles
migrated portable engine tests pass
local bundled dependencies resolve without 1Scout
minimal host/platform primitives compile and are independently testable
no temporary legacy-issue stubs exist
```

### Checkpoint A5 — Port Coordinator, Persistence & Page Assembly

Migrate the high-value coordinator and storage behavior:

```text
storage/crawler.go
storage/persist.go
storage/runs.go
engine/page.go
```

Preserve:

```text
single-writer coordinator
buffers
retry/defer behavior
checkpoint concepts
crawl state
page assembly and persistence projection
```

Responsibilities:

- Required SiteCrawl schema initialization (`schemaStmts`).
- Run lifecycle persistence.
- Coordinator integration.
- Fixture crawl end-to-end.
- Process-level readback.
- Cancellation and progress integration through the A4 primitives.
- **PageSpeed Decoupling Gate (Open Decision)**: Resolve whether to preserve PSI types inside `internal/sitecrawl/` temporarily (Option A) or decouple the coordinator via an internal interface / null object collector pattern (Option B).

**Acceptance**:

```text
coordinator can start and finish a crawl
local SQLite can persist one run
context cancellation works through the coordinator
progress/events can be observed during a crawl
fixture site can be crawled end-to-end
results survive process-level readback
```

### Checkpoint A6 — Regression Parity & Full Module Gate

Port applicable existing tests to establish behavioral parity.

Required regression families:

```text
frontier
robots
sitemap
redirects
network failures
429/503
resource crawling
pause/resume
cancel
render
crawl limits
```

- **Full Module Gate**: By Checkpoint A6, the active repository module must support full module verification:
  ```text
  go list ./...
  ```
  Legacy extracted Go reference directories under `go/` must not permanently poison module traversal. Once their required behavior and source have been preserved into `internal/sitecrawl/`, they may be migrated, removed after parity, or isolated from active module traversal.
- The immutable Git tag `sitecrawl-pre-audit-v1` remains the permanent, recoverable reference.

**Acceptance**:

```text
all migrated core tests green
go list ./... succeeds across the active repository module
no intentional crawler behavior change remains undocumented
```

This is the **Standalone SiteCrawl Core Freeze**. Create the freeze tag:

```text
sitecrawl-standalone-core-v1
```

only after this gate passes.

### Checkpoint A7 — Optional Compatibility Features

Only after core freeze:

- legacy issue engine (`issues.go`);
- duplicate analysis (`duplicates.go`);
- PageSpeed integration (`pagespeed/`);
- query grid (`query.go`);
- exports (`export.go`);
- graph UI data (`graph.go`);
- post-core optional desktop UI integration (`service.go` and `ui/`).

Receiver decoupling strategy on `(s *Service)` remains an open decision (functional/repository signatures vs minimal standalone `Service` facade). These may be migrated selectively according to Technical Audit needs.

---

## 18. Technical Audit Entry Point

Only after Checkpoint A6 should Audit V1 implementation begin.

New flow:

```text
Standalone SiteCrawl
        ↓
Evidence Adapter
        ↓
Normalized Evidence
        ↓
Evidence Snapshot
        ↓
AR-* Rule Registry
        ↓
Rule Engine
        ↓
RuleResult
```

At this point `/knowledge` becomes the authority for new audit behavior.

Legacy `issues.go` remains non-authoritative.

---

## 19. Future Package Direction

After standalone parity, Technical Audit may introduce:

```text
internal/
├── sitecrawl/
│
├── audit/
│   ├── evidence/
│   ├── policy/
│   ├── rules/
│   ├── evaluation/
│   └── reporting/
│
└── platform/
    └── standalone/
```

The first major boundary should be:

```text
sitecrawl
→ evidence adapter
→ audit
```

not dozens of new crawler micro-packages.

---

## 20. Reintegration Strategy for 1Scout

Standalone development must avoid coupling new Audit domain logic to standalone infrastructure.

When the module is ready to return to 1Scout:

### Keep

```text
SiteCrawl acquisition improvements
Evidence Adapter
Audit evidence model
ProjectPolicy
47-rule registry
Rule Engine
Finding aggregation
Audit tests
```

### Replace

```text
standalone DB lifecycle
standalone job controller
standalone event sink
standalone credential provider
standalone executable shell
```

with existing 1Scout platform implementations.

Conceptually:

```text
                   ┌─ StandalonePlatform
SiteCrawl + Audit ─┤
                   └─ OneScoutPlatform
```

The crawler/audit domain should not need to know which host is active.

---

## 21. High-Risk Migration Areas

| Risk | Level | Mitigation |
|---|---:|---|
| Splitting the original flat package into many packages | High | Keep one `internal/sitecrawl` package initially |
| Replacing coordinator while extracting | High | Reuse coordinator behavior, adapt platform dependencies |
| Changing URL normalization | High | Preserve behavior and port tests before Audit normalization changes |
| Discovery/probe semantics | High later | Defer to Evidence Adapter phase |
| Renderer raw/render overwrite | High later | Preserve now; refactor only during Audit render phase |
| Legacy issue semantics leaking into AR rules | High | Mark legacy engine non-authoritative |
| Rebuilding full Wails UI early | Medium/High | Use dev CLI/test harness first |
| PageSpeed credentials/infrastructure blocking build | Medium | Make PageSpeed optional/deferred |
| SQLite shared-core assumptions | Medium | Introduce minimal standalone DB adapter |
| Losing 1Scout reintegration path | Medium | Keep standalone platform concerns outside crawler/audit domain |

---

## 22. Files That Must Not Become Audit Authority

Explicitly treat these behaviors as legacy compatibility:

```text
go/engine/issues.go
legacy severity constants
legacy indexability verdicts
legacy issue writes in finalize.go
UI issue taxonomy / translated issue descriptions
```

They can be useful regression references.

They cannot override:

```text
knowledge/03-rule-engine-spec.md
knowledge/07-v1-atomic-rule-manifest.md
```

---

## 23. Files With Highest Reuse Value

Based on the current SiteCrawl source, prioritize preserving behavior in:

```text
normalize.go
frontier.go
politeness.go
robots.go
sitemap.go
fetch.go
extract.go
links.go
render.go

crawler.go
persist.go
```

These collectively represent the most expensive crawler engineering already completed.

---

## 24. Migration Invariants

### M1

Do not rewrite the crawler from scratch.

### M2

Do not change SiteCrawl behavior and platform extraction in the same commit unless the behavior change is required and explicitly documented.

### M3

Keep the baseline tag immutable.

### M4

Do not treat the extracted directory layout as the desired Go package layout.

### M5

Preserve one-package behavior first; modularize later.

### M6

Do not port the full 1Scout platform.

### M7

Introduce minimal host adapters only.

### M8

Legacy SEO issues remain separate from Audit V1.

### M9

Do not introduce `AR-*` rules before standalone parity.

### M10

Do not reconstruct the full UI before crawler parity.

### M11

Do not let PageSpeed block the standalone acquisition core.

### M12

The standalone architecture must preserve a low-cost merge path back to 1Scout.

---

## 25. Definition of Standalone Core Done

The SiteCrawl standalone core is considered ready for Technical Audit development when:

```text
repository has a valid Go module

core SiteCrawl package builds independently from 1Scout

no runtime import requires the 1Scout repository for the core crawl path

fixture crawl runs end-to-end

HTTP/robots/sitemap/link extraction works

redirect behavior works

politeness and 429/503 handling work

local persistence works

pause/cancel behavior required by the standalone core works

rendering either works or is explicitly isolated as an optional capability

migrated acquisition regression tests pass

legacy issue behavior is clearly separated

knowledge v1.4 remains untouched

baseline tag remains recoverable
```

At that point create a standalone-core freeze before beginning Audit V1 implementation.

---

## 26. Migration Status & Next Task

Checkpoint A1 (Buildability Inventory) is **completed and approved** in [`docs/sitecrawl-standalone-buildability.md`](sitecrawl-standalone-buildability.md).

### Next Task: Checkpoint A2 — Scoped Go Module Bootstrap

Create root `go.mod` using:

```text
module github.com/truthrive/technical-seo-audit
go 1.22
```

- Verify genuine standalone packages using scoped verification target:
  ```text
  go list ./go/deps/...
  ```
- Avoid temporary `replace` directives or stub modules.
- Introduce external dependencies just-in-time when migrated active packages require them.
- Followed by coordinated Checkpoint A3/A4 milestone (`internal/sitecrawl/` reconstruction + minimal real platform contracts).

---

# Freeze Decision

The recommended migration strategy is:

```text
REFERENCE SITECRAWL
        ↓
DEPENDENCY INVENTORY
        ↓
ONE BUILDABLE SITECRAWL PACKAGE
        ↓
MINIMAL STANDALONE PLATFORM ADAPTERS
        ↓
CRAWLER REGRESSION PARITY
        ↓
STANDALONE CORE FREEZE
        ↓
AUDIT EVIDENCE ADAPTER
        ↓
TECHNICAL AUDIT V1
        ↓
1SCOUT REINTEGRATION
```

This sequencing deliberately separates **extraction risk** from **Audit architecture risk**.
