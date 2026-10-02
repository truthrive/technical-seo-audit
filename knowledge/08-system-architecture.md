# System Architecture V1

**Status:** Frozen for V1 design
**Knowledge baseline:** v1.3 / 2026-10-01
**Scope:** Technical Search & GEO Audit Tool — V1
**Executable rule contract:** 47 atomic rules
**Implementation:** This document defines architecture, not production code.

## 1. Architecture goal

V1 implements the product flow:

```text
website
→ evidence acquisition
→ normalized observations
→ atomic rule evaluation
→ rule results
→ aggregation
→ actionable audit report
```

Preserve these boundaries:

```text
OBSERVATION ≠ CONCLUSION
PROJECT POLICY ≠ OBSERVED EVIDENCE
ATOMIC RULE RESULT ≠ AGGREGATED FINDING
RULE STATUS ≠ REPORT PRESENTATION
```

Collectors gather evidence. The Rule Engine evaluates frozen rule semantics.

## 2. Architecture style

V1 uses a **pipeline-oriented modular monolith**.

```text
One application
├── acquisition modules
├── normalization modules
├── rule engine
├── aggregation
└── reporting
```

V1 does not require microservices, distributed workers, message brokers, a browser render farm, a distributed crawler, or a separate rule-engine service.

The system should remain modular enough that components can later be separated without redefining audit semantics.

## 3. Architecture principles

| Principle | Architectural implication |
|---|---|
| Evidence before conclusion | Acquisition modules never emit SEO/GEO verdicts |
| Deterministic before AI | Rule Engine uses deterministic logic whenever supported |
| No silent intent inference | Project policy remains separate from observations |
| Reproducible FAIL | Every deterministic FAIL references stored evidence |
| Acquisition failure ≠ audit failure | Missing evidence normally becomes `UNKNOWN` |
| Atomic rules | Rule Engine executes `AR-*`, not broad catalog checks |
| GEO guardrails | AI Search rules remain accessibility/policy checks, not scoring |
| No INFO status | Information exists only as report/presentation classification |
| Severity ≠ priority | No numeric priority model in V1 |
| Traceability | Result → atomic rule → parent check → evidence → source refs |

## 4. High-level system

```text
Audit Request
│  Start URL
│  Explicit Project Policy
│  Optional Discovery Inputs
│  Optional Render Selection
▼
Audit Run Orchestrator
▼
Evidence Acquisition Layer
│  Discovery Frontier
│  Evidence Probe Planner
│  HTTP Fetcher
│  Robots Collector / Parser
│  Sitemap Collector / Parser
│  HTML Extractor
│  Structured Data Extractor
│  Link Graph Builder
│  Optional Renderer
▼
Raw Evidence
▼
Normalization Layer
│  URL normalization
│  directive normalization
│  canonical resolution
│  response classification
│  graph derivation
│  render comparison
▼
Normalized Evidence Snapshot
▼
Rule Evaluation Layer
│  Rule Registry — 47 AR-* rules
│  Deterministic Evaluator
│  Assisted Evaluator
│  Manual Review Task Generator
▼
Atomic Rule Results
▼
Result Aggregator
▼
Audit Report Model
```

## 5. Core domain boundaries

V1 maintains six distinct concepts:

| Object | Meaning |
|---|---|
| `AuditRun` | One audit execution |
| `ProjectPolicy` | Explicit user/project expectations |
| `RawEvidence` | What collectors observed |
| `NormalizedObservation` | Evidence converted into consistent machine-readable fields |
| `RuleResult` | One atomic rule evaluation |
| `Finding` | Report-level aggregation of one or more rule results |

Example:

```text
meta robots = noindex       ← observation
expected_indexable = true   ← project policy
AR-INDEX-001 = FAIL         ← rule conclusion
```

The crawler must never independently emit “accidental noindex.”

## 6. AuditRun and ProjectPolicy

An AuditRun is the immutable context for one execution:

```text
AuditRun
├── run_id
├── started_at
├── start_url
├── rule_manifest_version
├── knowledge_snapshot
├── execution_settings
└── project_policy
```

Project policy fields frozen for V1 include:

```text
preferred_origin
expected_url_state
expected_crawlable
expected_indexable
sitemap_expected
snippet_policy
googlebot_access_policy
oai_searchbot_access_policy
gptbot_training_policy
priority_page
```

Architecture must not infer these fields from page content, URL patterns, or bot blocking.

## 7. Audit Run Orchestrator

The orchestrator coordinates the workflow but does not crawl, parse, render, or make audit decisions.

```text
create audit run
→ initialize discovery
→ coordinate acquisition
→ request evidence probes
→ request selected rendering
→ finalize normalized snapshot
→ execute rule evaluation
→ aggregate results
→ produce report model
```

## 8. Discovery Frontier

The frontier maintains the known candidate URL set.

Legitimate discovery sources:

```text
Start URL
XML sitemap
internal <a href>
pagination
optional supplied URL list
```

Discovery provenance must be preserved.

### Crawl candidates vs evidence targets

A critical V1 distinction:

**Crawl candidates** are real site URLs discovered from valid discovery sources.

**Evidence targets** are fetched because rules need additional technical evidence, for example:

```text
canonical target
redirect target
slash/non-slash variant
host/protocol variant
known-missing 404 probe
bot-profile request
```

Evidence targets must not automatically enter the site discovery graph.

## 9. Discover ↔ Crawl runtime loop

The conceptual lifecycle remains:

```text
Discover → Crawl → Render → Index → Consolidate → Retrieve → Cite → Measure
```

Runtime acquisition is iterative:

```text
Start URL
→ Discover
→ Crawl
→ Extract links
→ Discover new URLs
→ Crawl new URLs
→ ...
```

This does not redefine the lifecycle.

## 10. Evidence Acquisition Planner

A logical planner determines what evidence is still required without evaluating SEO rules.

Examples:

```text
canonical exists → schedule canonical target fetch
redirect detected → continue redirect traversal
sitemap discovered → fetch sitemap
sitemap lists URL → schedule evidence fetch
known-missing rule enabled → create probe
profile comparison enabled → schedule alternate request profile
render selected → schedule browser rendering
```

The Rule Engine never performs network requests.

## 11. HTTP Acquisition Module

Collects transport evidence:

```text
requested URL
HTTP status
final URL
redirect hops
response headers
content type
response time
fetch error
TLS result
response body reference
timestamp
```

It may execute request-profile comparisons, but those are observations only and never proof of verified crawler identity.

## 12. Robots Module

Responsibilities:

```text
fetch robots.txt
parse groups
normalize directives
evaluate effective rules for configured agents
retain matched-rule evidence
```

V1 needs decisions for:

```text
default crawler
Googlebot
OAI-SearchBot
GPTBot
```

The module emits facts such as `robots_allowed_oai_searchbot = false`, not “ChatGPT Search visibility problem.”

## 13. HTML Extraction Module

Extract technical observations required by V1:

```text
title
meta description
H1 set
meta robots
canonical
internal links
structured-data blocks
main-text signal
```

For links retain source, target, anchor, element, href and location when available.

## 14. Sitemap Module

Responsibilities:

```text
discover sitemap URLs
fetch sitemap documents
identify sitemap / sitemap-index type
parse listed URLs
collect lastmod when present
store parse errors
```

Preserve `sitemap → listed URL` relationships.

V1 may collect `lastmod`; it does not verify it against CMS history.

## 15. Structured Data Module

V1 needs syntax/extraction support for:

```text
JSON-LD
Microdata
RDFa where feasible
```

It may parse and store errors. It must not infer ideal schema type, “GEO schema,” or AI citation impact.

## 16. Link Graph Builder

The graph derives relationships from legitimate crawlable link evidence:

```text
source URL
├── anchor
├── location
└── target URL
```

Derived data may include:

```text
inlink count
crawl depth
shortest discovered path
discovery source
```

It must not infer business importance or SEO priority.

## 17. Evidence Probe System

Rules may require requests normal crawling would not make:

```text
HTTPS endpoint
host/protocol variants
slash counterpart
known-missing URL
canonical target
redirect destination
crawler-profile comparison
```

These observations must remain distinguishable from website discovery evidence.

A synthetic 404 probe must never become an orphan candidate, broken site URL, or site URL count member.

## 18. Optional Renderer

Rendering is separate from normal HTTP acquisition and is selective.

```text
selected URL
├── raw HTML
└── browser render
    └── rendered DOM
```

V1 compares:

```text
title
canonical
meta robots
H1
main-text presence
crawlable internal links
```

Renderer states:

```text
not selected
selected + successful
selected + failed
```

Implication examples:

```text
not selected → rule-specific NOT_APPLICABLE
selected but failed → UNKNOWN
render successful + field differs → WARNING where defined
```

Renderer failure must never become “Google cannot render this page.”

## 19. Raw Evidence Layer

Collectors first preserve raw evidence such as:

```text
HTTP response
robots.txt
HTML
sitemap XML
rendered DOM
structured-data block
response headers
redirect sequence
```

Each item should retain acquisition source, target, timestamp, observed value and acquisition error.

## 20. Normalization Layer

The Rule Engine does not parse arbitrary raw documents.

```text
raw evidence
→ normalizers
→ normalized observations
```

Examples:

```text
raw URL → normalized URL
robots string → directive tokens
canonical href → resolved canonical URL
redirect responses → redirect_hops[]
meta/header directives → effective index state
link elements → normalized links
raw/rendered DOM → render comparison
```

Derived evidence such as `effective_noindex`, `crawl_depth`, `redirect_loop_detected` or render link differences must remain reproducible from underlying evidence.

## 21. Normalized Evidence Snapshot

V1 uses snapshot-based evaluation.

```text
acquisition
→ normalization
→ evidence completion
→ SNAPSHOT FREEZE
→ rule evaluation
```

The same frozen evidence plus the same rule version should produce the same deterministic result.

Every RuleResult must be attributable to:

```text
audit_run + evidence_snapshot + rule_version
```

## 22. Rule Registry

The machine-readable registry represents `07-v1-atomic-rule-manifest.md`.

V1 contract:

```text
47 executable rules
30 deterministic
16 assisted
1 manual
```

Each definition owns:

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

Five deferred broad checks must not silently enter the executable registry.

## 23. Rule Evaluation Engine

The evaluator consumes only:

```text
Rule Definition
+ Normalized Evidence
+ Explicit Project Policy
```

It must not call HTTP, renderer, GSC, Bing, logs, external SEO APIs or LLMs during evaluation.

Logical flow:

```text
load atomic rule
→ check applicability
→ check required evidence
→ resolve explicit policy/context
→ evaluate condition
→ assign status
→ attach evidence
→ create RuleResult
```

Atomic rules must not depend on other RuleResult statuses. They may share normalized evidence.

## 24. Deterministic evaluator

Handles 30 deterministic rules. Output depends entirely on frozen normalized evidence.

No model inference is permitted.

## 25. Assisted evaluator

Handles 16 assisted rules. These use deterministic evidence plus explicit context/policy where required.

Missing policy must never be fabricated.

## 26. Manual review

Manual review is a first-class RuleResult. V1 contains one manual atomic rule.

Manual review output should include rule ID, target, evidence and review question/context.

## 27. RuleResult

Conceptually:

```text
RuleResult
├── audit_run
├── snapshot_id
├── rule_id
├── parent_check
├── rule_version
├── lifecycle
├── automation
├── target
├── status
├── severity
├── evidence refs
├── expected condition
└── evaluated_at
```

## 28. Aggregation Layer

Atomic results are grouped for usability after evaluation.

Example:

```text
87 individual AR-LINK-003 FAIL results
→ one report Finding
→ affected_count = 87
```

V1 may calculate safe aggregation facts but must not infer template/root-cause defects without evidence.

## 29. Audit Report Model

```text
RuleResult[]
→ Aggregation
→ Finding[]
→ Report
```

Each finding retains rule ID, parent check, status, severity, automation type, evidence, expected state, affected count, samples and recommended action.

Presentation may classify findings as Issue, Warning, Manual review or Information. `Information` never becomes a rule status.

No universal SEO score or GEO score is produced.

## 30. Lifecycle mapping

| Lifecycle | V1 architecture responsibility |
|---|---|
| Discover | Frontier, sitemap parser, link graph, pagination/orphan evidence |
| Crawl | HTTP, TLS, robots, redirects, request profiles |
| Render | Optional renderer and raw/render comparison |
| Index | Rules over directives and known-404 evidence |
| Consolidate | Canonical, redirects, host/slash normalization |
| Retrieve | Structured data and machine-readable evidence |
| Cite | Framework preserved; no dedicated citation-readiness rule family in V1 |
| Measure | Framework preserved; GSC/Bing/log/analytics integrations excluded |

Do not create empty Cite or Measure services merely because the lifecycle contains those stages.

## 31. AI Search / GEO placement

AI Search V1 reuses the same robots/access evidence infrastructure.

```text
robots.txt
├── Googlebot
├── OAI-SearchBot
└── GPTBot
```

There is no AI Rank Engine, GEO Score Engine, or Citation Probability Engine in V1.

## 32. LLM placement

No LLM exists in the audit verdict path.

Forbidden:

```text
evidence → LLM → PASS/FAIL
```

Optional future LLM assistance belongs downstream of frozen RuleResults for explanation, summarization, hypotheses or manual-review assistance.

## 33. Error handling model

| Situation | Behavior |
|---|---|
| Obtained state clearly violates rule | rule-specific `FAIL` |
| Required acquisition cannot produce evidence | usually `UNKNOWN` |
| Renderer not selected | rule-specific `NOT_APPLICABLE` |
| Renderer selected but fails | `UNKNOWN` for dependent rules |
| Explicit policy missing | rule-specific WARNING / MANUAL_REVIEW / N/A |
| Sitemap malformed | parsing rule may `FAIL` |
| Structured block malformed | syntax rule may `FAIL` |
| Bot profile differs | observed anomaly only |
| GPTBot intentionally blocked | not inherently a failure |
| Unsupported GEO concept | no mandatory rule |

## 34. Logical storage boundaries

Architecture requires logical persistence for:

```text
Audit Runs
Project Policy
Raw Evidence
Normalized Observations
Rule Definitions
Rule Results
Manual Review Results
Aggregated Findings
```

Storage technology is deferred to technical design after the logical data model.

## 35. V1 execution sequence

```text
1. Initialize AuditRun + ProjectPolicy
2. Bootstrap discovery
3. Iterative discovery + crawl
4. Complete evidence probes/targets
5. Selected rendering
6. Finalize normalization and graph
7. Freeze Evidence Snapshot
8. Evaluate 47 atomic rules
9. Create RuleResults
10. Aggregate
11. Build Audit Report Model
```

## 36. Architecture invariants

1. Acquisition modules cannot emit SEO/GEO verdicts.
2. Rule Engine cannot perform network/browser acquisition.
3. Rule Engine consumes normalized evidence, not raw HTML directly.
4. Project policy remains separate from observed evidence.
5. Atomic rules cannot depend on other RuleResult statuses.
6. Every deterministic FAIL is reproducible.
7. Acquisition failure does not automatically become FAIL.
8. Render failure does not mean search-engine render failure.
9. Request-profile tests do not equal verified bot identity.
10. GPTBot and OAI-SearchBot remain separate.
11. No `INFO` rule status exists.
12. No numeric SEO/GEO score exists.
13. No automatic page-intent/business-value inference exists.
14. Deferred checks do not silently enter execution.
15. Every AuditRun evaluates against a known rule-manifest version.

## 37. Explicitly outside V1 architecture

Do not introduce:

```text
Google Search Console integration
Bing Webmaster API integration
server-log analysis
GA4 / GTM validation
paid SEO APIs
distributed crawler/job infrastructure
browser render farm
scheduled recurring crawling
multi-user collaboration
automatic template/root-cause inference
automatic business-priority inference
automatic AI citation tracking
GEO / AI ranking / citation scores
llms.txt mandatory checks
LLM-generated technical verdicts
```

## 38. Decisions deferred to technical design

The following are implementation decisions, not audit semantics:

```text
programming language
web framework
crawler library
database/storage technology
crawl concurrency
timeouts/retries
crawl limits
same-origin/subdomain scope
parser libraries
fingerprint algorithm
main-text detection
challenge detection
renderer technology
report UI/export format
deployment target
```

## 39. Component dependency direction

```text
ProjectPolicy
    │
AuditRun → Orchestrator
            ↓
      Acquisition Layer
            ↓
        Raw Evidence
            ↓
     Normalization Layer
            ↓
Normalized Evidence Snapshot
        ↓           ↓
   Rule Registry   ProjectPolicy
        └─────┬─────┘
              ↓
         Rule Engine
              ↓
         Rule Results
              ↓
          Aggregator
              ↓
         Report Model
```

Forbidden dependency directions:

```text
Rule Engine ─X→ HTTP Fetcher
Rule Engine ─X→ Renderer
Report ─X→ mutate RuleResult
LLM ─X→ mutate RuleResult
```

## 40. Architecture freeze gate

V1 architecture is frozen when these decisions are accepted:

1. modular monolith for V1;
2. discovery URLs and evidence targets/probes remain separate;
3. Rule Engine performs no acquisition;
4. ProjectPolicy remains separate from evidence;
5. normalized snapshot is the Rule Engine boundary;
6. rule evaluation is snapshot-based;
7. `AR-*` rules do not depend on other rule statuses;
8. renderer is selective and optional;
9. aggregation occurs after atomic evaluation;
10. Cite/Measure do not receive artificial V1 modules;
11. AI Search reuses technical evidence infrastructure;
12. no LLM exists in the technical verdict path.
