# Technical Search & GEO Audit Tool — Product Workflow V1

**Status:** Approved Product Workflow V1
**Knowledge baseline:** v1.4
**Scope:** V1 Desktop / Standalone Product Contract
**Repository:** `truthrive/technical-seo-audit`

---

## 1. Document Purpose and Migration Context

This document defines the product workflow and user journey for the first usable version of the **Technical Search & GEO Audit Tool**. It acts as the product contract governing how backend acquisition, normalization, rule evaluation, and future UI presentation integrate to deliver user value.

### Migration Sequencing Relationship
- **No sequencing modification:** This specification does **not** alter the approved Checkpoint A0–A7 migration roadmap.
- **Immediate milestone:** Coordinated Checkpoint A3/A4 (engine reconstruction + minimal platform primitives) remains the next engineering milestone.
- **Deferred UI:** Frontend implementation remains strictly deferred until Checkpoint A7 (or post-core UI development).
- **Core Freeze prerequisite:** Execution of Audit V1 domain logic (`AR-*` rules) begins only after Standalone SiteCrawl Core Freeze (Checkpoint A6).
- **Guiding role:** This contract exists so that engine mechanics and data models developed during Checkpoints A3–A6 produce the exact technical evidence and aggregation contracts required by the final product experience.

---

## 2. Product Objective and Value Flow

The tool is not merely a raw crawler data viewer or an internal table explorer. Its primary value is transforming raw web acquisitions into reproducible evidence, deterministic and assisted rule verdicts, and organized, severity-aware, actionable findings.

```text
Crawl Website
      ↓
Reproducible Technical Evidence
      ↓
Frozen Atomic Rule Evaluation (47 AR-* Rules: 30 Deterministic / 16 Assisted / 1 Manual)
      ↓
Aggregated Findings
      ↓
Affected URLs & Evidence Deep Dive
      ↓
Recommended Remediation Action
```

### Core Architecture Invariants
Across all product surfaces, the workflow preserves five foundational architectural invariants:
1. `OBSERVATION ≠ CONCLUSION`: Technical evidence collected by acquisition modules never contains audit verdicts or assumed defect labels.
2. `PROJECT POLICY ≠ OBSERVED EVIDENCE`: Explicit user expectations (e.g. expected indexable, allowed origins) remain strictly separate from observed HTTP/HTML facts.
3. `ATOMIC RULE RESULT ≠ AGGREGATED FINDING`: Machine-evaluated atomic rules (`AR-*`) evaluate individual technical conditions; human-facing findings aggregate one or more rule results for presentation.
4. `RULE STATUS ≠ REPORT PRESENTATION`: Evaluated rule statuses (`PASS`, `WARNING`, `FAIL`, `MANUAL_REVIEW`, `UNKNOWN`, `NOT_APPLICABLE`) map into report presentation classes (`ISSUE`, `WARNING`, `MANUAL_REVIEW`, `INFORMATION`) without erasing underlying machine states. There is no universal 1:1 hard mapping between rule status/severity and presentation class.
5. `SEVERITY ≠ PRIORITY`: Frozen severity tiers (`P0` Critical, `P1` High, `P2` Medium, `P3` Low/Optional) reflect technical defect impact, not automated business priority. V1 does not introduce a numeric priority algorithm or rank-ordering score.

---

## 3. Low-Fidelity Navigation Model

The V1 application follows a hierarchical, task-driven navigation structure:

```text
Projects / Audits
   ↓
New Audit (Configuration)
   ↓
Crawl Progress
   ↓
Audit Overview
   ├── Findings List
   │      └── Finding Detail
   │              └── URL / Evidence Inspector
   │
   └── Raw Crawl Data (Secondary / Investigation)
```

---

## 4. The Six V1 Product Surfaces

### Surface 1: Projects / Audits
**Purpose:** Provide the top-level workspace to review historical audits, initiate a new audit run, or reopen a previously completed audit.

**Minimum Data Displayed:**
- **Audit Run ID:** Unique identifier for the audit execution.
- **Start URL / Target Domain:** Target website starting endpoint.
- **Run Status:** Current frozen workflow status:
  `CREATED` | `ACQUIRING` | `NORMALIZING` | `SNAPSHOT_FROZEN` | `EVALUATING` | `AGGREGATING` | `COMPLETED` | `FAILED` | `CANCELLED`
- **Timestamp:** Started at and completed at dates/times.
- **Crawl Volume:** Total URLs discovered and total URLs crawled.
- **Finding Summary Counts:** Summary counts grouped strictly by `Finding.presentation_classification`:
  - Issues (`ISSUE`)
  - Warnings (`WARNING`)
  - Manual Review (`MANUAL_REVIEW`)
  - Information (`INFORMATION`)
  *(Note: Finding summary counts are sourced directly from the aggregated Finding entity, never inferred on-the-fly by the UI).*
- **Primary Actions:** `New Audit`, `Open Audit`, `Delete Audit`.

**V1 Boundary Constraints:**
- Single-user local desktop workstation scope.
- No multi-user roles, teams, organizations, or cloud synchronization.

---

### Surface 2: New Audit / Crawl Configuration
**Purpose:** Collect target inputs, crawler operational parameters, and explicit project policy before starting an acquisition run.

**Configuration Form Separation:**

#### A. Required Settings
- **Start URL:** Full seed URL (e.g., `https://example.com/`). Normalization validates scheme and host.

#### B. Advanced Crawl Settings
Configured within supported SiteCrawl engine parameters (authoritative constants from `go/engine/types.go`):
- **Max Crawl URLs:** Upper bound on crawled URLs (`DefaultMaxURLs = 50000`, `MaxMaxURLs = 500000`).
- **Max Crawl Depth:** Maximum link distance from seed URL (`DefaultMaxDepth = 3`, `MaxMaxDepth = 30`).
- **Respect robots.txt:** Toggle whether crawler strictly honors robots directives (default: `true`).
- **Sitemap Discovery:** Toggle automatic detection and parsing of `/sitemap.xml` and robots-declared sitemaps (default: `true`).
- **Rendering Mode:** Selection between:
  - `HTTP Only` (fast, raw HTML extraction, standard mode)
  - `Headless Chrome / Edge` (client JavaScript rendering where supported)
- **Politeness / Concurrency:** Maximum concurrent worker threads (`DefaultConcurrency = 5`) and per-host crawl delay (ms).

#### C. Explicit Project Policy Inputs
Explicit expectations supplied by the user via `ProjectPolicyAssignment`. The engine **never** infers business policy from page content, URL paths, or crawl observations.

##### 1. SITE-Scoped Policies
- `preferred_origin`: Target canonical origin (e.g., `https://example.com`).
- `sitemap_expected`: Boolean flag defining whether XML sitemap presence is expected for the site. *(Note: This controls whether sitemap absence is an issue; it does NOT assert that every live URL must appear in a sitemap).*
- `snippet_policy`: Expected snippet restriction policy (`allow_unrestricted` | `restrict` | `unspecified`).
- `googlebot_access_policy`: Expected crawler access for Googlebot (`allow` | `block` | `unspecified`).
- `oai_searchbot_access_policy`: Expected access policy for OpenAI SearchBot (`allow` | `block` | `unspecified`).
- `gptbot_training_policy`: Expected training scraper policy for GPTBot (`allow` | `block` | `unspecified`).

##### 2. URL-Scoped Policies
Explicit policy assignments mapped to specific supplied URLs:
- `expected_url_state`: Expected response state (`live` | `redirect` | `missing` | `unspecified`).
- `expected_crawlable`: Boolean flag defining whether specific URLs are explicitly expected to be crawlable.
- `expected_indexable`: Boolean flag defining whether specific URLs are explicitly intended for indexation.
- `priority_page`: Boolean flag marking high-value priority URLs for focused audit attention.

**Forbidden in V1 Configuration:**
- No cloud proxy network settings.
- No automated CAPTCHA bypass credentials.
- No third-party SEO tool API keys (Ahrefs, Semrush, Moz).
- No GSC / Bing Webmaster OAuth authentication prompts.

---

### Surface 3: Crawl Progress
**Purpose:** Provide real-time observability of evidence acquisition without blocking the user interface or making premature audit judgments.

**Live Metrics & State Displayed:**
- **Execution Phase:** Current frozen pipeline stage (`ACQUIRING`, `NORMALIZING`, `SNAPSHOT_FROZEN`, `EVALUATING`, `AGGREGATING`).
- **Frontier Status:**
  - Crawled URLs count
  - Queued / Discovered URLs count
  - In-flight active requests
- **Acquisition Health:**
  - Response breakdown counter: 2xx (Success), 3xx (Redirect), 4xx (Client Error), 5xx (Server Error)
  - Network timeouts and connection drops
- **Rate & Performance:**
  - Current crawl rate (URLs/sec)
  - Elapsed run time
- **Run Controls:**
  - `Pause`: Pauses frontier dispatch and permits active worker buffers to drain.
  - `Resume`: Resumes frontier scheduling.
  - `Cancel`: Immediately halts acquisition, persisting partial data up to the last clean buffer.
  *(Note: Pause and resume are operational crawler controls and do not alter the frozen AuditRun workflow status set).*

---

### Surface 4: Audit Overview
**Purpose:** High-level executive and technical summary of the completed audit. Focuses squarely on actionable audit findings rather than raw crawler tables.

**STRICT PROHIBITION — Score Models:**
The V1 Overview must **never** calculate or display:
- An "Overall SEO Score" (e.g., "78/100")
- A "GEO Optimization Score"
- A "Site Health Grade" (e.g., "B+")
- A percentage health index
- Speculative AI citation probabilities

**Presentation Summary (Four Report Classes):**
Audit findings are organized strictly into the frozen report presentation classifications:
1. **Issues (`ISSUE`):** Report items highlighting significant technical defects or non-compliant states (e.g., 5xx server responses, broken internal links, canonical target errors, contradictory robots directives, or explicit-policy noindex conflicts).
2. **Warnings (`WARNING`):** Report items highlighting advisory anomalies, sub-optimal technical configurations, or crawl friction (e.g., redirect chains, sitemap non-200 entries, render field discrepancies, or snippet restrictions without explicit policy).
3. **Manual Review (`MANUAL_REVIEW`):** Report items highlighting conditions where technical evidence was verified but conclusive evaluation requires human business context, page intent, or template interpretation (e.g., block-level `data-nosnippet` tags, complex navigation candidates lacking `<a href>`, or assisted render inspections).
4. **Information (`INFORMATION`):** Report items documenting verified configurations, structural architecture, and policy alignment confirmations (e.g., HTTPS endpoint verified, robots access matching explicit policy, sitemap discovered).

*(Note: Evaluated `RuleResult.status` remains distinct. A `FAIL` on an advisory rule may be classified as a Warning in presentation, and an assisted rule may generate a Manual Review presentation regardless of underlying status).*

**Separation of Crawler Metrics:**
Crawler metrics are displayed in an adjacent summary block, clearly separated from findings:
- Total URLs Discovered vs Crawled
- HTTP Status Distribution (2xx, 3xx, 4xx, 5xx)
- Maximum and Average Discovered Crawl Depth
- Protocol Breakdown (HTTPS vs HTTP)
- Total Acquisition Duration and Snapshot Timestamp

---

### Surface 5: Findings (List & Detail)
**Purpose:** The central analytical surface of the product, organizing evaluated rule results into understandable, actionable technical findings.

**Finding Record Contract:**
Each finding card/row in the list and detail view exposes:
- **Finding Title:** Clear, human-readable summary of the condition (e.g., *"Internal link targets 4xx client error"*).
- **Relevant Rule ID:** Singular atomic identifier from the frozen manifest (e.g., `AR-LINK-003`).
- **Parent Catalog Check:** Broader domain category reference (e.g., `LINK-002`).
- **Rule Result Status:** Underlying machine evaluation status:
  `PASS` | `WARNING` | `FAIL` | `MANUAL_REVIEW` | `NOT_APPLICABLE` | `UNKNOWN`
- **Default Severity:** Frozen severity tier: `P0` (Critical), `P1` (High), `P2` (Medium), `P3` (Low / Optional).
- **Presentation Classification:** `ISSUE` | `WARNING` | `MANUAL_REVIEW` | `INFORMATION`.
- **Automation Class:** `deterministic` | `assisted` | `manual`.
- **Affected Subject Count:** Number of unique URLs or subjects exhibiting this exact result.
- **Sample Affected URLs:** Quick-reference sample list of affected endpoints.
- **Observed Evidence Summary:** Concise factual statement of what was measured.
- **Expected State:** Normative technical baseline or explicit project policy expectation.
- **Recommended Action:** Clear, plain-language engineering guidance for remediation.

**Aggregation Rule:**
- Safe V1 aggregation is centered on a single atomic rule: Findings aggregate `RuleResult` instances sharing the same `rule_id`, `status`, and `severity`.
- `Finding` stores a singular `rule_id`, with `FindingMember` connecting individual `RuleResult` instances to the parent Finding.
- One Finding represents one atomic condition across multiple affected URLs; unrelated rules are never merged into a single Finding.
- Higher-level UI categorization represents report grouping, not a single Finding entity.
- Aggregations **never** guess root causes (e.g., attributing defects to CMS plugins or hosting providers) unless explicitly verified by deterministic rule evidence.

---

### Surface 6: URL / Evidence Inspector
**Purpose:** Deep technical inspection of an individual URL, displaying normalized evidence collected alongside evaluated rule verdicts.

**Foundational Rule:**
`OBSERVATION ≠ CONCLUSION`. The inspector renders observed facts and rule conclusions in separate, distinct UI panels.

**Ten Evidence Domains Displayed:**
1. **HTTP Response:** Fetch status code, TLS certificate validity, response time (ms), fetch error type, and optional raw response header references (`FetchObservation`).
2. **Redirects & Hops:** Full ordered redirect chain (`RedirectHop[]`), HTTP status per hop, `Location` header targets, redirect loop flag, resolved final destination URL.
3. **Robots Directives & Decisions:** Document fetch status for `/robots.txt`, effective line matched, allow/disallow decisions evaluated across specific bot profiles (`DEFAULT`, `GOOGLEBOT`, `OAI_SEARCHBOT`, `GPTBOT`) (`RobotsDecision`).
4. **Page Index Directives:** Raw `<meta name="robots">` tags and `X-Robots-Tag` HTTP headers, parsed token list (including any `nofollow` tokens), unsupported tokens, parse errors, and calculated `effective_noindex` flag (`RobotsDirectiveObservation`).
5. **Canonical Configuration:** Raw `<link rel="canonical">` tag values, resolved absolute URL, canonical target fetch status, target indexability state, and self-canonical determination (`CanonicalObservation`).
6. **Title, Meta Description & Headings:** Raw title text, meta description text, array of `<h1>` values, and main text presence indicator (`HtmlObservation`). *(Note: Pixel length calculations and duplicate heading detection are excluded from V1).*
7. **Internal Link Graph:** Inbound link count (`crawl_inlink_count`), discovered crawl depth (`crawl_depth`), list of source inlinks with anchor text and DOM location context (`MAIN`, `NAV`, `FOOTER`, `HEADER`), list of outbound internal links (`LinkObservation`).
8. **Sitemap Presence:** Presence in XML sitemaps, sitemap document URL, raw and normalized `<lastmod>` values (`SitemapEntry`).
9. **Structured Data:** Extracted blocks by format (`JSON_LD`, `MICRODATA`, `RDFA`), raw block value/artifact ref, syntax parse status, and parse error messages (`StructuredDataBlock`). *(Note: Schema type appropriateness and GEO quality judgments are excluded from V1).*
10. **Normalized Raw-vs-Rendered Field Comparison:** When JavaScript rendering is enabled, normalized comparisons across the frozen `RenderFieldComparison` fields: `TITLE`, `CANONICAL`, `META_ROBOTS`, `H1`, `MAIN_TEXT_PRESENCE`, `INTERNAL_LINK_SET` (`RenderObservation`, `RenderFieldComparison`). *(Note: Pixel/screenshot visual diffs are excluded).*

**Evaluated Rule Results Panel:**
A dedicated panel showing all atomic rules evaluated for this URL:
- Table listing `rule_id`, rule name, `status`, `severity`, primary/supporting evidence references (`RuleEvidenceRef`), and link back to the parent finding.

---

## 5. Finding Example Flows

The following flows illustrate how raw technical observations translate into reproducible rule results, aggregated findings, and recommended actions without false certainty.

### Flow 1: Internal Link Target Returns 404 (Broken Link)
- **Observed Evidence:**
  - Crawler fetches `https://example.com/about`.
  - HTML parser discovers `<a href="/team-bios">` located in `MAIN` content.
  - Discovery frontier schedules fetch for `https://example.com/team-bios` (`FetchObservation`).
  - Fetch attempt returns HTTP response status `404 Not Found`.
- **Rule Evaluation:**
  - Evaluator executes `AR-LINK-003` (Internal link targets 4xx).
  - Parent: `LINK-002`.
  - Automation: `deterministic`.
  - Precondition: resolved target is internal and fetchable.
  - Condition: target status is 4xx.
  - Result: `RuleResult` status `FAIL`, default severity `P1`, subject `https://example.com/team-bios`.
  - Evidence Ref: Primary = `FetchObservation(status=404)`, Supporting = `LinkObservation(source=/about, anchor="Meet the Team")`.
- **Finding Model:**
  - Title: *"Internal link targets 4xx client error"*
  - Rule ID: `AR-LINK-003` (Parent: `LINK-002`)
  - Classification: `ISSUE`
  - Severity: `P1`
  - Affected Subjects: 1 link target (`/team-bios`), referenced by 3 source pages.
- **Affected URL Inspector:**
  - User inspects `/team-bios`: Under HTTP Response, sees status `404 Not Found`.
  - Under Internal Link Graph, identifies exact source referring pages (`/about`, `/company`, `/careers`) and anchor text.
- **Recommended Action:**
  - *"Update or remove internal link(s) pointing to `/team-bios` across referring pages, or restore the destination URL to return 200 OK."*

---

### Flow 2: Canonical Target Returns Non-200 Status
- **Observed Evidence:**
  - Page `https://example.com/products/widget-blue` returns `200 OK`.
  - HTML contains `<link rel="canonical" href="https://example.com/products/widget-all">`.
  - Evidence planner schedules probe fetch for canonical target `https://example.com/products/widget-all`.
  - Canonical target returns `404 Not Found`.
- **Rule Evaluation:**
  - Evaluator executes `AR-CANON-006` (Canonical target returns final 200).
  - Parent: `CANON-003`.
  - Automation: `deterministic`.
  - Preconditions: exactly one valid canonical target and target fetch attempted.
  - Condition: valid single canonical target was fetched and target does not directly return 200 (returns 404).
  - Result: `RuleResult` status `FAIL`, default severity `P1`, subject `/products/widget-blue`.
  - Evidence Ref: Primary = `CanonicalObservation(resolved_url=https://example.com/products/widget-all)`, Supporting = `FetchObservation(url=/products/widget-all, status=404)`.
- **Finding Model:**
  - Title: *"Canonical target does not return 200 OK"*
  - Rule ID: `AR-CANON-006` (Parent: `CANON-003`)
  - Classification: `ISSUE`
  - Severity: `P1`
  - Affected Subjects: 12 product variant pages declaring `/products/widget-all`.
- **Affected URL Inspector:**
  - User inspects `/products/widget-blue`: Under Canonical Configuration, sees declared canonical URL, resolved URL, and red target fetch badge `404 Not Found`.
- **Recommended Action:**
  - *"Update canonical link tag on affected page(s) to reference an existing 200 OK destination, or restore the canonical target URL."*

---

### Flow 3: Conflicting Robots Index Directives
- **Observed Evidence:**
  - URL `https://example.com/checkout/confirmation` is crawled.
  - HTTP response header includes: `X-Robots-Tag: noindex`.
  - HTML `<head>` includes: `<meta name="robots" content="index">`.
- **Rule Evaluation:**
  - Normalizer extracts parsed tokens: `robots_meta_tokens=[index]`, `x_robots_tokens=[noindex]`.
  - Evaluator executes `AR-INDEX-002` (Conflicting robots directives).
  - Parent: `INDEX-001`.
  - Automation: `deterministic`.
  - Precondition: at least one applicable directive parsed.
  - Condition: contradictory `index` and `noindex` directives explicitly present in the same applicable scope.
  - Result: `RuleResult` status `FAIL`, default severity `P1`.
  - Evidence Ref: `RobotsDirectiveObservation(meta=[index], header=[noindex])`.
- **Finding Model:**
  - Title: *"Contradictory index and noindex directives in same scope"*
  - Rule ID: `AR-INDEX-002` (Parent: `INDEX-001`)
  - Classification: `ISSUE`
  - Severity: `P1`
  - Affected Subjects: 1 URL.
- **Affected URL Inspector:**
  - Page Index Directives section displays:
    - Raw `X-Robots-Tag`: `noindex` (HTTP Header)
    - Raw `meta robots`: `index` (HTML `<head>`)
    - Directive tokens and contradictory conflict indicator.
- **Recommended Action:**
  - *"Align robots index directives across HTTP response headers and HTML `<head>`. Remove contradictory index and noindex directives to ensure consistent indexing signals."*

---

### Flow 4: Snippet Restriction Conflicts With Snippet Policy (Assisted)
- **Observed Evidence:**
  - URL `https://example.com/guide/pricing` returns `200 OK`.
  - HTML contains `<meta name="robots" content="max-snippet:20">`.
  - Explicit project policy supplies: `snippet_policy = allow_unrestricted`.
- **Rule Evaluation:**
  - Evaluator executes `AR-INDEX-005` (Snippet restriction conflicts with snippet policy).
  - Parent: `INDEX-007`.
  - Automation: `assisted`.
  - Precondition: snippet directives extracted.
  - Condition: explicit `snippet_policy=allow_unrestricted` and snippet restriction (`max-snippet:20`) is present.
  - Result: `RuleResult` status `FAIL`, default severity `P1`.
    *(Note: If policy had been unspecified, status would be `WARNING`. If block-level `data-nosnippet` required contextual interpretation, status would be `MANUAL_REVIEW`).*
  - Evidence Ref: `RobotsDirectiveObservation(max_snippet=20)`, `ProjectPolicyAssignment(snippet_policy=allow_unrestricted)`.
- **Finding Model:**
  - Title: *"Snippet restriction conflicts with explicit unrestricted snippet policy"*
  - Rule ID: `AR-INDEX-005` (Parent: `INDEX-007`)
  - Classification: `ISSUE`
  - Severity: `P1`
  - Affected Subjects: 1 URL.
- **Affected URL Inspector:**
  - Page Index Directives section displays extracted snippet directive: `max-snippet:20`.
  - Rule evaluation panel notes policy conflict against supplied `snippet_policy = allow_unrestricted`.
- **Recommended Action:**
  - *"Review the `max-snippet:20` directive against project snippet policy. If full text previews and quotations are intended, remove or adjust the restriction."*
  *(Guardrail: Relaxing snippet controls permits snippet quotation but does NOT guarantee search or AI citation).*

---

### Flow 5: AI Search Crawler Policy Evaluations (Two Separate Atomic Findings)

#### 5A. OAI-SearchBot Access Policy
- **Observed Evidence:**
  - `/robots.txt` contains:
    ```text
    User-agent: OAI-SearchBot
    Allow: /
    ```
  - Explicit project policy supplies: `oai_searchbot_access_policy = allow`.
- **Rule Evaluation:**
  - Evaluator executes `AR-AI-001` (OAI-SearchBot robots access matches project policy).
  - Parent: `AI-001`.
  - Automation: `assisted`.
  - Precondition: OAI-SearchBot robots policy can be evaluated.
  - Condition: explicit `allow` policy matches effective robots behavior (`ALLOWED`).
  - Result: `RuleResult` status `PASS`, default severity `P2`.
- **Finding Model:**
  - Title: *"OAI-SearchBot robots access matches project policy"*
  - Rule ID: `AR-AI-001` (Parent: `AI-001`)
  - Classification: `INFORMATION`
  - Severity: `P2`
- **Recommended Action:**
  - *"OAI-SearchBot robots policy matches the explicit project policy and does not disallow this crawler."*
  *(Guardrail: Robots allow is policy/access evidence; actual HTTP/profile accessibility may require separate evidence. Accessibility does not guarantee ChatGPT inclusion, ranking, quotation, or citation).*

#### 5B. GPTBot Training Scraper Policy
- **Observed Evidence:**
  - `/robots.txt` contains:
    ```text
    User-agent: GPTBot
    Disallow: /
    ```
  - Explicit project policy supplies: `gptbot_training_policy = block`.
- **Rule Evaluation:**
  - Evaluator executes `AR-AI-003` (GPTBot configuration matches explicit training policy).
  - Parent: `AI-002`.
  - Automation: `assisted`.
  - Precondition: GPTBot robots state can be evaluated.
  - Condition: explicit `block` training policy matches effective GPTBot robots behavior (`DISALLOWED`).
  - Result: `RuleResult` status `PASS`, default severity `P2`.
- **Finding Model:**
  - Title: *"GPTBot robots configuration matches training policy"*
  - Rule ID: `AR-AI-003` (Parent: `AI-002`)
  - Classification: `INFORMATION`
  - Severity: `P2`
- **Recommended Action:**
  - *"GPTBot robots policy matches the explicit project training policy and disallows GPTBot crawling under robots.txt."*
  *(Guardrail: GPTBot blocked is an intentional training protection policy; it does NOT mean ChatGPT Search is blocked).*

---

## 6. AI Search & GEO Product Guardrails

V1 implements rigorous domain guardrails for AI Search and Generative Engine Optimization (GEO). The tool must never promise speculative ranking outcomes.

### Strict Guardrail Principles
1. **OAI-SearchBot vs GPTBot Independence:**
   - Disallowing `GPTBot` must **never** be reported as a failure to appear in ChatGPT Search.
   - They are distinct bots with separate platform purposes (training scraper vs search crawler).
2. **Access ≠ Citation Guarantee:**
   - Permitting crawler access to AI bots guarantees only retrieval eligibility.
   - The UI and findings must explicitly state that crawler accessibility does not guarantee AI answer inclusion, quotation, or citation ranking.
3. **Bot Profile ≠ Verified Identity:**
   - Simulated bot request profiles in tests do not represent cryptographically verified crawler identity.
4. **No Universal "GEO Score":**
   - V1 strictly rejects arbitrary composite metrics (e.g. *"GEO Readiness: 64%"*).
   - Findings in this domain are strictly factual: access availability, snippet quotation permissions, machine text extractability, and valid schema syntax.
5. **No Unsupported Probability Claims:**
   - Do not claim predictive likelihood of LLM citation (e.g. *"90% probability of citation"*).

---

## 7. Role of Raw Crawler Data

The product maintains standard raw crawler data views, but places them in a secondary, supporting role.

### Supported Raw Data Tables
- **All Crawled URLs:** Filterable table by scheme, host, path, status, MIME type, depth.
- **Status Code Explorer:** Direct aggregation by HTTP response class (2xx, 3xx, 4xx, 5xx, timeouts).
- **Redirects & Chains:** Complete table of redirect hops, loop flags, and destinations.
- **Canonical Ledger:** URL vs canonical URL mapping table.
- **Internal Link Matrix:** Full directional link edge ledger with anchor texts.
- **Sitemap Index & Entries:** Listed URLs vs crawled URLs reconciliation table.
- **Structured Data Ledger:** JSON-LD/Microdata block inventories.
- **DOM Render Comparisons:** Normalized raw vs rendered field diff ledger.

### Product Positioning
- Raw crawler views are accessible via the `Raw Crawl Data` tab.
- They serve technical debugging and investigative validation.
- They do **not** replace the primary audit workflow (`Audit Overview` → `Findings` → `Evidence Inspector`).

---

## 8. Data-to-Product Mapping

The product surfaces map directly to the frozen logical entities defined in `knowledge/09-data-model.md`:

| Product Surface | Required Logical Entities / Data Elements | Produced By Layer / Module |
|---|---|---|
| **1. Projects / Audits** | `AuditRun`, finding counts by `presentation_classification`, crawl counters | Orchestrator & Persistence Repository |
| **2. New Audit Config** | `AuditRun`, `ProjectPolicyAssignment` | UI Configuration & Orchestrator |
| **3. Crawl Progress** | `AuditRun.run_status`, live crawl metrics, frontier queue counters, buffer stats | Discovery Frontier, HTTP Fetcher, Buffer Sink |
| **4. Audit Overview** | `Finding` summary, presentation class rollups, `AuditRun` crawl summary | Result Aggregator & Normalizer |
| **5. Findings (List/Detail)** | `Finding`, `FindingMember`, `RuleResult`, `RuleDefinition`, `ManualReviewTask` | Rule Engine (`AR-*`) & Result Aggregator |
| **6. URL / Evidence Inspector** | `UrlResource`, `DiscoveryRecord`, `FetchObservation`, `RedirectHop`, `RobotsDecision`, `HtmlObservation`, `RobotsDirectiveObservation`, `CanonicalObservation`, `LinkObservation`, `SitemapEntry`, `StructuredDataBlock`, `RenderObservation`, `RenderFieldComparison`, `RuleResult`, `RuleEvidenceRef` | Normalization Layer, Evidence Snapshot, Rule Engine |
| **Secondary: Raw Crawl Data** | Raw observation tables (`FetchObservation`, `RedirectHop`, `LinkObservation`, `SitemapObservation`, `StructuredDataBlock`) | Acquisition Modules & SQLite Storage |

---

## 9. Backend and Architecture Implications

To fulfill this product workflow, subsequent implementation phases must respect strict architectural boundaries:

```text
┌────────────────────────────────────────────────────────┐
│ UI Layer                                               │
│ - Collects Start URL and explicit ProjectPolicy        │
│ - Displays live progress and run controls              │
│ - Renders Overview, Findings, and Evidence Inspector   │
└───────────────────────────▲────────────────────────────┘
                            │ (Reads findings & evidence)
┌───────────────────────────┴────────────────────────────┐
│ Audit Responsibility (Post-Crawl)                      │
│ - Normalization into EvidenceSnapshot                  │
│ - Freeze snapshot immutability                         │
│ - Evaluate 47 AR-* atomic rules                        │
│   (30 deterministic / 16 assisted / 1 manual)          │
│ - Synthesize RuleResults into Findings                 │
│ - Generate ManualReviewTasks                           │
└───────────────────────────▲────────────────────────────┘
                            │ (Consumes raw crawl data)
┌───────────────────────────┴────────────────────────────┐
│ SiteCrawl / Acquisition Responsibility (Crawl Phase)   │
│ - BFS Frontier scheduling & Host Politeness            │
│ - HTTP fetching & redirects                            │
│ - Robots.txt & XML Sitemap parsing                     │
│ - Raw HTML & Structured Data extraction                │
│ - Link graph discovery & optional headless rendering   │
│ - SQLite raw persistence                               │
└────────────────────────────────────────────────────────┘
```

### 1. SiteCrawl Acquisition Boundary
- The crawler’s sole responsibility is high-integrity acquisition and storage of web observations.
- The crawler **must never evaluate SEO rules**, calculate health scores, or emit audit verdicts.

### 2. Audit Normalization & Rule Engine Boundary
- Operates strictly on a frozen `EvidenceSnapshot`.
- The Rule Engine executes the 47 frozen atomic rules (`AR-*`):
  - **30 Deterministic rules:** evaluate normalized evidence against frozen conditions without external inference.
  - **16 Assisted rules:** evaluate technical conditions combined with explicit `ProjectPolicy` or structured contextual preconditions.
  - **1 Manual rule:** collects structured evidence for guided human evaluation.
- Evaluators output immutable `RuleResult` instances linked to `RuleEvidenceRef`.
- Result aggregation synthesizes findings without altering atomic rule statuses.
- LLM-only verdicts are strictly forbidden.

### 3. UI Presentation Boundary
- The UI renders data models supplied by the backend.
- The UI does not execute crawler mechanics, does not infer missing project policy, and does not invent audit logic.

---

## 10. V1 Product Exclusions (Out of Scope)

To maintain disciplined MVP delivery, the following capabilities are explicitly excluded from V1:
- **No Google Search Console (GSC) API integration:** No OAuth flows or Search Analytics API dependencies.
- **No Bing Webmaster Tools API:** No external search engine verification APIs.
- **No Server Log Ingestion:** No log file parsing (Apache/Nginx/CDN access logs).
- **No Cloud Crawling Infrastructure:** No distributed crawler nodes, proxies, or remote clusters.
- **No Scheduled / Recurring Crawls:** Audits are executed on-demand locally.
- **No Multi-User / Workspace Collaboration:** No user authentication, permissions, or team workspaces.
- **No Automated SERP Tracking:** No rank tracking or automated search engine scraping.
- **No Hard PageSpeed API Dependency:** PageSpeed Insights is optional/deferred and not part of core audit.
- **No Synthetic Scores:** No composite percentage grades, health indices, or SEO scores.
- **No LLM Auto-Verdicts:** No unverified generative AI assertions or non-reproducible evaluations.

---

## 11. V1 Product Decisions on Open Questions

### 1. Manual Review Task Persistence
- In V1, `ManualReviewTask` is scoped strictly to `audit_run_id` with statuses `OPEN` and `RESOLVED`.
- Optional human resolution is stored separately and does not rewrite the original `RuleResult`.
- Manual review resolution remains run-scoped in V1.
- Re-crawling a site initiates a new `AuditRun`, and previously resolved manual review decisions are **not** automatically carried forward.
- Cross-run resolution carry-forward is a candidate for future versions once explicit entity-identity and policy-versioning semantics are established.

### 2. Headless Execution Output
- Product-level export (CSV, Excel, formatted PDF reports) is not required for the initial V1 workflow and is deferred.
- The standalone development harness (`cmd/sitecrawl-dev`) emits structured JSON / debug log output sufficient for automated verification, regression testing, and CI parity validation during Checkpoints A3–A6.
- Headless output formatting must not block engineering milestones A3–A6.

---

## 12. Verification Checklist

Before accepting this workflow specification into the project documentation:
- [x] Verified consistency with MVP scope in `knowledge/05-mvp-scope.md`.
- [x] Verified terminology matches frozen contracts in `knowledge/08-system-architecture.md` and `knowledge/09-data-model.md`.
- [x] Confirmed all rule references align with `knowledge/07-v1-atomic-rule-manifest.md` (exact 47 rules: 30 deterministic, 16 assisted, 1 manual).
- [x] Confirmed absolute exclusion of score models (SEO score, GEO score, health grade, priority score).
- [x] Confirmed strict separation of Observation vs Conclusion, Rule Status vs Report Presentation, and Severity vs Priority.
- [x] Confirmed AI Search guardrails (OAI-SearchBot vs GPTBot distinction, no citation guarantee).
- [x] Confirmed that current engineering migration sequencing (A2 -> A3/A4 -> A5 -> A6 -> A7) is completely preserved.
