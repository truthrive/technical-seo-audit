# V1 Implementation Plan — SiteCrawl-Based Migration

**Status:** Revised for SiteCrawl-based V1 implementation  
**Knowledge baseline:** v1.4  
**SiteCrawl reference baseline:** 1Scout Marketing Site Crawl, commit `2e2cbff` (2026-09-25), as documented in `sitecrawl-docs.zip`  
**Depends on:** `00-product-principles.md` through `09-data-model.md`  
**Scope:** Technical Search & GEO Audit Tool — V1  
**Executable contract:** 47 atomic rules  
**Automation split:** 30 deterministic / 16 assisted / 1 manual  
**Deferred broad checks:** 5  
**Production code:** Not included

---

## 1. Purpose

Define the implementation sequence for evolving the existing **SiteCrawl** codebase into the V1 Technical Search & GEO Audit Tool without rebuilding mature crawling capabilities from scratch and without importing SiteCrawl's legacy SEO verdict semantics into the new audit engine.

The target flow remains:

```text
SiteCrawl-derived acquisition
→ raw technical evidence
→ audit evidence adapter
→ normalized observations
→ frozen evidence snapshot
→ 47 AR-* rule evaluations
→ atomic RuleResults
→ aggregated Findings
→ audit report
```

Implementation should optimize for:

```text
reuse proven acquisition behavior
→ preserve crawler stability
→ separate evidence from verdicts
→ deterministic normalization
→ reproducible rule results
→ low false positives
→ usable reporting
```

not maximum feature count or maximum refactor depth.

---

## 2. SiteCrawl Role in V1

SiteCrawl becomes the **acquisition foundation** of the new product.

Capabilities expected to be reused or adapted include:

```text
URL normalization
BFS frontier
crawl limits
pause/resume
HTTP acquisition
redirect traversal
retry/backoff
host politeness
robots.txt
sitemap discovery/parsing
HTML extraction
internal link graph
structured-data extraction
selected JavaScript rendering
SQLite persistence foundations
existing crawl fixtures/tests
```

SiteCrawl must **not** remain the authority for audit verdict semantics.

The following legacy concepts are explicitly non-authoritative for V1:

```text
engine/issues.go issue taxonomy
legacy severity assignments
legacy indexability verdicts
Page.Issues as V1 audit output
heuristics such as title length / thin content / no-schema warnings
canonical-other as automatic error
noindex as automatic critical issue
```

The authority for new audit behavior is `/knowledge`, especially:

```text
03-rule-engine-spec.md
07-v1-atomic-rule-manifest.md
08-system-architecture.md
09-data-model.md
```

---

## 3. Migration Strategy

Do **not** perform a big-bang rewrite of SiteCrawl.

Preferred migration pattern:

```text
Existing SiteCrawl
      │
      ├── existing crawler behavior stays operational
      │
      ▼
Evidence Adapter
      │
      ▼
New Audit Domain
      │
      ▼
New AR-* Rule Engine
      │
      ▼
New Findings / Audit Report
```

During migration, legacy and new systems may coexist:

```text
SiteCrawl legacy Issues
→ existing UI / compatibility only

AR-* RuleResults
→ new Technical Audit report
```

Legacy code should be retired only after the corresponding new audit path is verified.

---

## 4. Core Implementation Principles

### 4.1 Preserve working crawler behavior

Do not rewrite frontier, HTTP, politeness, robots, sitemap, rendering or persistence merely to make the code resemble the new logical data model.

Refactor only where the frozen audit architecture requires a semantic boundary.

### 4.2 Adapt before replacing

Prefer:

```text
Existing Page / crawl data
→ Evidence Adapter
→ new normalized audit entities
```

before replacing the existing `Page` model throughout the crawler/UI/storage stack.

### 4.3 Evidence before verdict

SiteCrawl acquisition may report what happened technically.

It must not become the authority for whether that state is an SEO/GEO failure.

### 4.4 Project policy stays separate

Explicit context such as:

```text
expected_indexable
expected_crawlable
expected_url_state
priority_page
OAI/GPTBot policy
```

must be stored as `ProjectPolicyAssignment`, not inferred from SiteCrawl page state.

### 4.5 Probe traffic must not pollute discovery

Canonical targets, missing-page probes, host variants, slash variants and bot-profile comparisons may be fetched for evidence without becoming legitimate discovery records.

### 4.6 Snapshot before evaluation

AR-* rules evaluate a frozen normalized evidence snapshot, not mutable crawler state.

### 4.7 Legacy issues do not map automatically to AR rules

Do not create migrations such as:

```text
legacy noindex issue → AR-INDEX-* FAIL
legacy canonical-other → AR-CANON-* FAIL
legacy no-structured-data → AR-ENTITY-* FAIL
```

Each AR rule must be implemented directly from its frozen contract.

### 4.8 Do not implement deferred checks

These broad checks remain non-executable V1 scope:

```text
ACC-006 repeated instability
ACC-008 crawl traps
INDEX-005 index pollution
CANON-004 signal-consistency aggregation
CANON-005 parameter/filter strategy
```

---

## 5. Target Repository Direction

The existing SiteCrawl repository should remain the development base.

Conceptual structure:

```text
sitecrawl/
├── knowledge/                  # frozen domain source of truth
├── .ai-dev-system/             # agent workflow / execution rules
│
├── existing SiteCrawl code     # acquisition foundation
│   ├── engine/
│   ├── storage/
│   ├── pagespeed/
│   ├── app-glue/
│   └── ui/
│
├── audit/                      # new logical audit domain
│   ├── evidence/
│   ├── policy/
│   ├── normalize/
│   ├── rules/
│   ├── evaluation/
│   ├── aggregation/
│   └── report/
│
└── tests/
```

Exact folder names are an implementation decision. The required boundary is conceptual, not a mandated filesystem layout.

---

## 6. Phase Overview

| Phase | Focus | Main outcome |
|---|---|---|
| 0 | Freeze SiteCrawl baseline + project bootstrap | Safe migration starting point |
| 1 | SiteCrawl codebase mapping | Existing code mapped to new architecture/data model |
| 2 | Audit domain + 47-rule registry | New audit contracts exist without changing crawler behavior |
| 3 | Evidence Adapter + snapshot | SiteCrawl observations become normalized audit evidence |
| 4 | First vertical slice: HTTP / robots / index | First new end-to-end RuleResults |
| 5 | Canonical / redirect / probes | Consolidation rules on new evidence model |
| 6 | Link graph / sitemap / pagination | Discovery rules migrated |
| 7 | Structured data | `AR-ENTITY-001` migrated |
| 8 | Renderer refactor | Raw/render comparison preserved separately |
| 9 | Bot profiles + AI Search policy | OAI/GPTBot rules implemented safely |
| 10 | Complete 47-rule engine | Full manifest executable from snapshots |
| 11 | Aggregation + new audit report | RuleResults become usable Findings |
| 12 | Hardening + legacy retirement decision | Stable V1 release candidate |

---

# PHASE 0 — Freeze Existing SiteCrawl Baseline

## 7. Objective

Create a recoverable migration starting point before modifying the working crawler.

## Required actions

1. Use the actual SiteCrawl repository as the new product base.
2. Record the current working commit.
3. Create a baseline tag or equivalent recoverable reference, for example:

```text
sitecrawl-before-technical-audit-v1
```

4. Create a dedicated implementation branch, for example:

```text
feature/technical-audit-v1
```

5. Add `/knowledge` v1.4 to the repository.
6. Add `.ai-dev-system`.
7. Add `PROJECT_CONTEXT.md` defining authority boundaries.
8. Run the existing build/test suite before any migration change.

## `PROJECT_CONTEXT.md` must state

```text
/knowledge
= SEO/GEO domain authority

existing SiteCrawl code
= current implementation evidence and reusable acquisition foundation

legacy issues.go
= legacy behavior, not V1 audit authority

.ai-dev-system
= instructions for how agents plan, edit, test and review
```

## Baseline evidence

Capture at minimum:

```text
current commit
build command
unit/integration test command
number of passing tests
known failing tests if any
current app start/build state
```

## Acceptance gate

```text
baseline commit recorded
existing tests executed
knowledge v1.4 present
agent instructions point to knowledge/INDEX.md
no crawler behavior changed
```

---

# PHASE 1 — SiteCrawl Codebase Mapping

## 8. Objective

Map the actual SiteCrawl codebase to `08-system-architecture.md` and `09-data-model.md` before editing core crawler behavior.

This phase is analysis/documentation plus tests where needed; it should not perform broad refactoring.

## Required mapping

At minimum map:

| Existing SiteCrawl | New architecture concept | Initial action |
|---|---|---|
| `engine/frontier.go` | Discovery Frontier | reuse/refactor only where provenance/probe separation requires |
| `engine/fetch.go` | HTTP acquisition | reuse |
| `engine/politeness.go` | host politeness | reuse |
| `engine/robots.go` | Robots collector/parser | reuse + extend agent-specific evidence |
| `engine/sitemap.go` | Sitemap acquisition/parser | reuse + expose observations |
| `engine/extract.go` | HTML evidence extractor | reuse + expose raw observations |
| `engine/links.go` | Link observations / graph input | reuse, review discovery semantics |
| `engine/render.go` | Optional Renderer | reuse mechanics, refactor output contract |
| `engine/types.go::Page` | legacy combined page object | adapt; do not immediately replace |
| `storage/crawler.go` | orchestration + persistence | preserve initially; identify verdict coupling |
| `storage/finalize.go` | legacy crawl-wide conclusions | review individually; not V1 authority |
| `engine/issues.go` | legacy rule engine | isolate / compatibility only |
| existing SQLite schema | legacy persistence | assess extension vs new audit tables |
| React crawl UI | current product surface | preserve until new audit reporting phase |

## Required deliverable

Create a codebase map such as:

```text
SITECRAWL-INTEGRATION-MAP.md
```

For each relevant module record:

```text
current responsibility
inputs
outputs
current tests
new architecture equivalent
reuse / adapt / replace / legacy
migration risk
required contract changes
```

## Critical questions this phase must answer

### Discovery provenance

Does the actual code distinguish:

```text
internal-link discovery
sitemap discovery
canonical target
redirect target
pagination
synthetic probe
```

well enough for the V1 data model?

### Verdict coupling

Identify every place where SiteCrawl currently calls or persists:

```text
evaluate()
indexabilityOf()
Issues
legacy severity
```

### Rendering

Confirm whether rendered HTML currently replaces the raw parsed document and identify the minimum change required to preserve both.

### Storage

Determine whether new audit entities can initially live beside existing tables without breaking current runs/history/UI.

## Acceptance gate

No production migration begins until:

```text
all relevant existing modules mapped
legacy verdict coupling locations known
probe/discovery conflicts known
render raw-vs-render gap confirmed
storage migration strategy selected at high level
existing tests still pass
```

---

# PHASE 2 — New Audit Domain & Atomic Rule Registry

## 9. Objective

Create the new audit contracts beside SiteCrawl without changing crawler outputs or existing UI behavior.

## Implement domain concepts

At minimum:

```text
AuditRun
ProjectPolicyAssignment
UrlResource
DiscoveryRecord
FetchObservation
RedirectHop
EvidenceSnapshot
NormalizedObservation
RuleDefinition
RuleResult
RuleEvidenceRef
ManualReviewTask
Finding
FindingMember
```

These are logical concepts from `09-data-model.md`; physical storage can be incremental.

## Rule registry

Create a machine-readable registry for all 47 `AR-*` rules.

Validate exactly:

```text
47 active rules
30 deterministic
16 assisted
1 manual
5 deferred broad checks remain non-executable
```

Every executable rule must preserve:

```text
rule_id
parent_check
rule_version
name
lifecycle
automation
required_inputs
preconditions
status conditions
default_severity
evidence_fields
source_refs
```

## Central statuses

Exactly:

```text
PASS
WARNING
FAIL
MANUAL_REVIEW
NOT_APPLICABLE
UNKNOWN
```

Reject `INFO` as a RuleResult status.

## Important constraint

Phase 2 must not make `engine/issues.go` call the new registry or vice versa.

The systems remain separate until the Evidence Adapter is available.

## Acceptance gate

```text
47 rules load and validate
all new audit contracts compile
no network/browser dependency in rule package
legacy crawler behavior unchanged
existing SiteCrawl tests still pass
new registry tests pass
```

---

# PHASE 3 — Evidence Adapter & Snapshot Boundary

## 10. Objective

Translate existing SiteCrawl crawl output into the normalized evidence model without first rewriting the crawler internals.

## Initial migration pattern

```text
SiteCrawl Page / crawl records
        ↓
Evidence Adapter
        ↓
UrlResource
FetchObservation
HtmlObservation
RobotsDirectiveObservation
CanonicalObservation
LinkObservation
...
        ↓
NormalizedObservation
        ↓
EvidenceSnapshot
```

## Adapter responsibilities

Translate only evidence that actually exists.

Do not manufacture absent fields to satisfy a rule.

Examples:

```text
Page.Status
→ FetchObservation.status

Page.Canonical(s)
→ CanonicalObservation

Page.MetaRobots / headers
→ RobotsDirectiveObservation

current crawl links
→ LinkObservation
```

## Explicit policy layer

Implement scoped `ProjectPolicyAssignment` independently from SiteCrawl page state.

Initial scope:

```text
SITE
URL
```

## Snapshot

Implement:

```text
BUILDING
→ FROZEN
```

Rules must reject mutable snapshots.

## Evidence gaps

If SiteCrawl does not currently preserve required raw evidence, mark the gap and update acquisition in the later relevant phase.

Do not solve gaps by inference.

## Acceptance gate

A completed SiteCrawl run can be converted to a frozen audit evidence snapshot without evaluating AR rules and without changing the legacy SiteCrawl report.

---

# PHASE 4 — First Vertical Slice: HTTP / Robots / Index

## 11. Objective

Prove the new architecture end-to-end using existing SiteCrawl acquisition wherever possible.

Target flow:

```text
SiteCrawl acquisition
→ Evidence Adapter
→ normalized snapshot
→ AR rule
→ RuleResult
→ evidence refs
```

## SiteCrawl reuse

Prefer reuse from:

```text
engine/fetch.go
engine/robots.go
engine/extract.go
engine/types.go
```

Refactor only when required evidence is not retained.

## Initial rule set

Prioritize the HTTP/robots/index rules whose evidence is already strongest in SiteCrawl.

The exact rule order must follow `07-v1-atomic-rule-manifest.md`, but the phase should cover the practical first slice across:

```text
AR-ACC-001 ... AR-ACC-004 where evidence is ready
AR-INDEX-* directive/status primitives
```

Rules requiring later probe/profile behavior may remain pending until their dependency phase.

## Critical semantic differences from legacy issues

Tests must prove:

```text
noindex observed
≠ automatic FAIL

robots block observed
≠ automatic FAIL without required policy

fetch acquisition failure
≠ automatic technical FAIL when rule contract says UNKNOWN
```

## Acceptance gate

At least one controlled crawl produces reproducible new `RuleResult` objects independently of `Page.Issues`.

---

# PHASE 5 — Canonical, Redirect & Evidence Probe Separation

## 12. Objective

Migrate consolidation evidence while fixing the main architectural mismatch between SiteCrawl frontier behavior and the new audit model.

## Reuse

Reuse redirect acquisition/traversal from existing fetcher.

Reuse canonical extraction from existing parser.

## Required refactor: discovery vs evidence target

SiteCrawl currently fetches non-link references such as canonical targets so legacy rules can inspect them.

V1 must distinguish:

```text
legitimate discovery
```

from:

```text
evidence-only acquisition
```

Introduce explicit acquisition purpose/provenance for at least:

```text
CANONICAL_TARGET
REDIRECT_TARGET
HOST_VARIANT_PROBE
SLASH_VARIANT_PROBE
KNOWN_MISSING_PROBE
BOT_PROFILE_COMPARE
```

Evidence-only targets must not automatically alter:

```text
crawl depth
internal inlink count
orphan calculation
site URL count
discovery-source reporting
```

## Known-missing probe

Add collision-resistant 404/410 evidence probing required by the frozen manifest.

## Implement canonical family

Implement `AR-CANON-*` from normalized evidence only.

Do not translate legacy:

```text
canonical-other
canonical-missing
canonical-chain
```

directly into new statuses without applying each AR contract.

## Acceptance gate

Canonical/redirect rules are reproducible and probe-only targets cannot pollute the discovery graph.

---

# PHASE 6 — Link Graph, Sitemap & Pagination

## 13. Objective

Adapt SiteCrawl's mature discovery mechanics to the new provenance-aware evidence model.

## Link graph

Reuse existing link extraction/edge mechanics.

Expose normalized evidence required for:

```text
source URL
target URL
anchor
element
href
link placement
crawl depth
inlink count
shortest path
```

## Provenance hardening

Ensure graph metrics use legitimate `DiscoveryRecord` membership, not every fetched UrlResource.

## Sitemap

Reuse existing support for:

```text
robots-declared sitemap
common sitemap paths
sitemap index
urlset
lastmod collection
```

But retain document-level and entry-level evidence instead of only using sitemap data to seed the frontier.

## Pagination

Expose enough evidence for the frozen pagination-related canonical rule without building a universal pagination strategy.

## Implement

```text
AR-LINK-*
AR-DISC-*
remaining pagination-dependent AR-CANON rule(s)
```

## Acceptance gate

The audit can distinguish:

```text
internal-link discovered
sitemap-only
supplied
pagination discovered
evidence-only fetched
```

and produce stable link/sitemap results.

---

# PHASE 7 — Structured Data Syntax

## 14. Objective

Adapt existing structured-data extraction to the narrow V1 syntax contract.

SiteCrawl already extracts JSON-LD and microdata. Verify actual support required by the V1 manifest, including RDFa if required by the frozen rule.

Store per block:

```text
format
block identity/index
raw evidence
parse status
parse error
```

Implement:

```text
AR-ENTITY-001
```

Do not carry forward the legacy semantic:

```text
no structured data → warning
```

unless a future approved rule explicitly defines it.

## Acceptance gate

Structured-data syntax can be evaluated deterministically without external APIs and without inferring schema appropriateness or GEO benefit.

---

# PHASE 8 — Renderer Refactor: Preserve Raw and Rendered Evidence

## 15. Objective

Reuse SiteCrawl's Chrome/Edge rendering mechanics while changing its evidence contract.

## Existing behavior to change

SiteCrawl rendering currently treats rendered output as the effective parsed page state.

V1 needs both:

```text
raw HTML observations
+
rendered DOM observations
```

Preserve them independently.

## Compare normalized fields

At minimum:

```text
TITLE
CANONICAL
META_ROBOTS
H1
MAIN_TEXT_PRESENCE
INTERNAL_LINK_SET
```

Produce `RenderFieldComparison` evidence.

## Resource evidence

Collect only the CSS/JS evidence required for the approved access/render rule.

Do not interpret every blocked asset as an SEO failure.

## Implement

```text
AR-RENDER-001 ... AR-RENDER-006
render-dependent AR-ACC rule(s)
```

## Required tests

```text
not selected → rule-specific NOT_APPLICABLE
selected + renderer failure → UNKNOWN
successful + unchanged → PASS where defined
successful + material difference → WARNING where defined
```

No test may equate renderer failure with “Google cannot render”.

---

# PHASE 9 — Request Profiles & AI Search Policy

## 16. Objective

Extend SiteCrawl's existing user-agent/profile mechanics for safe AI Search evidence collection.

## Reuse

SiteCrawl already has user-agent presets and bot-vs-browser behavior detection concepts.

Refactor profile testing so baseline and comparison responses are both preserved instead of collapsing them into a boolean verdict.

Required normalized comparison evidence includes:

```text
baseline status
comparison status
baseline challenge state
comparison challenge state
response fingerprints
```

## Add AI-specific policy support

Support independent evidence/policy for:

```text
OAI-SearchBot
GPTBot
```

GPTBot remains a training-control policy and must not be treated as a synonym for ChatGPT Search accessibility.

## Implement

Relevant remaining:

```text
AR-ACC-* assisted bot/profile checks
AR-AI-001
AR-AI-002
AR-AI-003
```

## Regression guardrails

Tests must prohibit:

```text
GPTBot blocked → ChatGPT Search blocked
crawler allowed → AI citation guaranteed
schema present → AI visibility guaranteed
profile UA response → verified crawler identity
```

## Acceptance gate

All AI Search V1 findings are explainable using stored technical evidence plus explicit project policy only.

---

# PHASE 10 — Complete 47-Rule Evaluation

## 17. Objective

Verify the complete frozen manifest through one common evaluation contract.

Final flow:

```text
RuleDefinition
+
Frozen EvidenceSnapshot
+
Explicit ProjectPolicy
        ↓
applicability
        ↓
required inputs
        ↓
condition evaluation
        ↓
status + severity
        ↓
RuleEvidenceRef[]
        ↓
RuleResult
```

## Hard dependency rules

The new Rule Engine must not call:

```text
HTTP Fetcher
Renderer
Search Console
Bing API
server logs
LLM
external SEO APIs
```

Rules must not evaluate from another rule's status.

## Registry verification

Assert exactly:

```text
47 unique executable AR-* IDs
30 deterministic
16 assisted
1 manual
```

and zero executable rules corresponding to the five deferred broad checks.

## Fixture requirements

Each rule must have fixtures appropriate to its contract, including applicable paths for:

```text
PASS
FAIL
WARNING
MANUAL_REVIEW
UNKNOWN
NOT_APPLICABLE
missing policy
missing evidence
malformed evidence
edge cases
```

Not every rule needs every status; only statuses valid for its frozen contract.

## Acceptance gate

All 47 rules evaluate from fixture snapshots with zero network/browser activity during evaluation.

---

# PHASE 11 — Aggregation & New Audit Report

## 18. Objective

Build the new audit result surface without forcing immediate removal of the existing SiteCrawl UI.

## Aggregation

Initial grouping should primarily use:

```text
rule_id
status
severity
```

Atomic results remain immutable.

## Finding model

Expose:

```text
rule ID
parent check
lifecycle
status
severity
automation
summary
expected state
affected count
sample targets
evidence
recommended action
```

## Presentation

Allowed report classifications may include:

```text
ISSUE
WARNING
MANUAL_REVIEW
INFORMATION
```

`INFORMATION` is presentation only and must never become a RuleResult status.

## Legacy UI strategy

During this phase choose one:

```text
A. add a new Technical Audit view beside existing SiteCrawl results
or
B. progressively replace legacy issue panels with new Findings
```

Do not rewrite the entire React crawl UI before the new result contract is stable.

## No scoring

V1 report must not compute:

```text
SEO health score
GEO score
AI readiness score
ranking probability
citation probability
```

## Acceptance gate

A completed crawl can produce a report solely from stored new `RuleResult` data without invoking `engine/issues.go`.

---

# PHASE 12 — Hardening & Legacy Retirement Decision

## 19. Objective

Validate the new product and determine what legacy SiteCrawl behavior should remain, be isolated or be removed.

## Integration fixture scenarios

Maintain controlled scenarios for:

```text
clean baseline
HTTP 4xx/5xx
redirect/chain/loop
robots policy
noindex / directive conflicts
canonical target redirect/4xx/noindex
broken internal links
sitemap-only URL
orphan candidate
render differences
render failure
OAI-SearchBot policy
GPTBot policy
profile-specific response anomalies
```

## Reproducibility

Verify:

```text
same frozen snapshot
+
same rule versions
→ same deterministic RuleResults
```

## Probe pollution

Explicitly test that:

```text
known-missing probes
canonical targets
host variants
slash variants
bot profiles
```

cannot change discovery-derived metrics unless they were independently legitimately discovered.

## Security baseline

Review at minimum:

```text
user-supplied URL validation
redirect safety
localhost/private-network behavior
response size limits
XML parsing safety
HTML parsing safety
renderer isolation
request/resource limits
artifact size limits
```

## Legacy retirement review

Only now decide the future of:

```text
engine/issues.go
indexabilityOf()
legacy issue storage
legacy issue UI
legacy finalize rules
```

Possible outcomes:

```text
retain as separate crawler diagnostics
keep compatibility-only
migrate specific non-conflicting features
remove after replacement
```

Do not delete legacy behavior merely because an AR rule with a similar name exists.

## Acceptance gate

V1 release candidate satisfies the full release contract below.

---

## 20. V1 Release Acceptance Criteria

V1 is ready when all are true:

```text
SiteCrawl acquisition remains stable
existing crawler regression suite passes

47 executable AR-* rules validate
30 deterministic
16 assisted
1 manual
5 deferred checks remain non-executable

new Rule Engine is independent from legacy issues.go
same snapshot produces stable deterministic results
every deterministic FAIL has reproducible evidence refs

crawler/acquisition does not own SEO/GEO verdict semantics
Rule Engine performs no network/browser acquisition
project policy remains explicit

UrlResource / DiscoveryRecord / FetchObservation semantics are preserved
probe-only acquisitions do not pollute discovery metrics

raw and rendered evidence remain independently available
render failure does not mean search-engine render failure

request profiles are not represented as verified bot identities
GPTBot and OAI-SearchBot remain separate

no INFO rule status exists
no numeric SEO/GEO/AI citation score exists

no GSC/Bing/log/paid SEO API dependency is required

audit report can be built from stored RuleResults
```

---

## 21. Testing Strategy

Maintain five testing layers.

### Existing SiteCrawl regression tests

Must continue to protect mature crawler behavior such as:

```text
frontier
pause/resume
politeness
retry/defer
sitemap
rendering
custom headers
crawl limits
storage/history
```

### New domain/unit tests

Test:

```text
policy resolution
normalized entities
snapshot immutability
rule registry validation
rule condition evaluation
aggregation
```

### Evidence Adapter tests

Given stable SiteCrawl objects/records, assert exact normalized audit evidence.

This layer is critical during migration.

### Rule fixtures

Test every `AR-*` independently from normalized evidence.

### End-to-end audit fixtures

Test:

```text
seed URL
→ SiteCrawl acquisition
→ adapter
→ snapshot
→ 47-rule evaluation
→ Findings
```

---

## 22. Definition of Done for One Migrated Rule

A rule is complete only when:

```text
RuleDefinition matches 07-v1-atomic-rule-manifest.md
required SiteCrawl evidence source is identified
Evidence Adapter mapping exists
normalized fields are deterministic
missing evidence behavior is explicit
project policy dependencies are explicit
status paths are tested
evidence refs are stored
rule version is recorded
rule fixture passes
integration fixture passes
legacy issue semantics are not silently reused
```

---

## 23. Definition of Done for One Migration Phase

A phase is complete only when:

```text
planned migration behavior implemented
new tests pass
relevant existing SiteCrawl tests still pass
knowledge contract preserved
no deferred scope introduced
no unsupported GEO claims introduced
no crawler regression is knowingly accepted without explicit decision
acceptance gate passes
```

---

## 24. Change-Control During Migration

If existing SiteCrawl behavior conflicts with the frozen audit model:

```text
identify conflict
↓
classify as implementation vs domain conflict
↓
keep existing behavior isolated if possible
↓
review knowledge contract
↓
change knowledge only if explicitly approved
↓
version affected rule if semantics change
↓
update tests
↓
resume migration
```

Do not silently change audit semantics to match legacy code.

Equally, do not delete mature crawler behavior merely because it is outside the current 47-rule manifest.

---

## 25. AI Dev System Usage

The AI Dev System should be applied from Phase 0 onward.

For each implementation task, agents should receive:

```text
purpose
exact phase
relevant knowledge files
relevant SiteCrawl source files
allowed files to modify
explicit non-goals
acceptance tests
```

Avoid prompts such as:

```text
"upgrade SiteCrawl into the new technical audit tool"
```

Prefer narrow tasks such as:

```text
"Phase 2 only: add the new RuleDefinition registry and registry validation tests. Do not change crawler, storage schema, UI or legacy issues.go."
```

---

## 26. Recommended Git Strategy

Preserve the pre-audit state with a tag/release reference.

Then use small migration commits, for example:

```text
chore: add technical audit knowledge and project context

docs: map sitecrawl modules to audit architecture

audit: add domain contracts and AR rule registry

audit: adapt crawl page evidence into frozen snapshots

audit: add HTTP and index rule vertical slice

audit: separate canonical probes from discovery graph

audit: adapt sitemap and link evidence

audit: preserve raw and rendered observations

audit: add OAI and GPTBot policy evaluation

audit: complete 47-rule manifest coverage

audit-ui: add findings report
```

Before each commit:

```text
git status
git diff
build
tests
stage intended files only
commit
push
```

Do not stage secrets, environment files, build artifacts, caches or temporary outputs.

---

## 27. What Not to Build During V1

Do not use migration as an excuse to expand toward:

```text
full Screaming Frog replacement
GSC OAuth
Bing integration
server-log analytics
GA4/GTM validation
mandatory PageSpeed integration
enterprise distributed crawl
cloud render farm
multi-user collaboration
automatic business priority
automatic page-role classification
LLM-generated technical verdicts
AI citation tracking
SEO score
GEO score
citation probability
mandatory llms.txt auditing
```

Existing SiteCrawl capabilities outside V1 may remain in the application, but they do not become part of the new audit contract automatically.

---

## 28. Revised Milestones

### Milestone A — Safe Migration Foundation

Contains:

```text
Phase 0
Phase 1
Phase 2
```

Outcome:

```text
working SiteCrawl baseline
+
codebase migration map
+
new 47-rule audit domain
```

No major crawler rewrite yet.

### Milestone B — Evidence Bridge

Contains:

```text
Phase 3
Phase 4
Phase 5
```

Outcome:

```text
SiteCrawl evidence
→ normalized snapshot
→ first reproducible new RuleResults
→ discovery/probe boundary enforced
```

### Milestone C — Technical Search Coverage

Contains:

```text
Phase 6
Phase 7
Phase 8
```

Outcome:

```text
link/sitemap
structured-data syntax
raw/render comparison
```

### Milestone D — AI Search + Full Rule Engine

Contains:

```text
Phase 9
Phase 10
```

Outcome:

```text
OAI/GPTBot evidence
+
all 47 AR-* rules executable
```

### Milestone E — Productization

Contains:

```text
Phase 11
Phase 12
```

Outcome:

```text
new audit findings/report
+
V1 hardening
+
legacy retirement decision
```

---

## 29. First Implementation Task After Approval

Do **not** begin by changing the crawler.

The first execution task should be:

**Phase 0 — Freeze SiteCrawl baseline and integrate knowledge/AI Dev System.**

The first technical analysis task should then be:

**Phase 1 — SiteCrawl Codebase Mapping.**

Only after the mapping is reviewed should Codex create the new audit domain in Phase 2.

Recommended first implementation boundary:

```text
allowed:
- add knowledge/
- add/update PROJECT_CONTEXT.md
- add .ai-dev-system integration
- record baseline/test commands
- create migration mapping document

not allowed:
- rewrite Page
- change frontier behavior
- change persistence schema
- change React UI
- delete issues.go
- implement AR rules
```

This keeps the first change low-risk and provides a verified map for all later work.

---

## 30. Final Implementation Direction

V1 is not a new crawler project.

It is:

```text
Existing SiteCrawl acquisition engine
            +
Frozen Technical Search & GEO knowledge
            +
Evidence Adapter
            +
Normalized Evidence Snapshot
            +
47-rule AR-* engine
            +
Findings/reporting layer
```

The migration strategy is therefore:

```text
PRESERVE
mature crawler mechanics

ADAPT
existing observations into explicit evidence contracts

SEPARATE
policy, evidence and verdicts

REPLACE GRADUALLY
legacy SEO issue semantics

VERIFY
with both SiteCrawl regression tests and new AR-* rule fixtures
```

This is the default implementation contract for V1 unless a later approved knowledge revision changes it.
