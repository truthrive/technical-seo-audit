# Technical Search & GEO Audit Tool — Product Workflow V1

**Status:** Approved Product Workflow Specification
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

The tool is not merely a raw crawler data viewer or an internal table explorer. Its primary value is transforming raw web acquisitions into reproducible evidence, deterministic audit rule verdicts, and prioritized, actionable remediation.

```text
Crawl Website
      ↓
Reproducible Technical Evidence
      ↓
Frozen Atomic Rule Evaluation (47 AR-* Rules)
      ↓
Aggregated Findings
      ↓
Affected URLs & Evidence Deep Dive
      ↓
Recommended Remediation Action
```

### Core Architecture Invariants
Across all product surfaces, the workflow preserves the four foundational architectural invariants:
1. `OBSERVATION ≠ CONCLUSION`: Technical evidence collected by acquisition modules never contains audit verdicts or assumed defect labels.
2. `PROJECT POLICY ≠ OBSERVED EVIDENCE`: Explicit user expectations (e.g. expected indexable, allowed origins) remain strictly separate from observed HTTP/HTML facts.
3. `ATOMIC RULE RESULT ≠ AGGREGATED FINDING`: Machine-evaluated atomic rules (`AR-*`) evaluate individual technical conditions; human-facing findings aggregate one or more rule results for presentation.
4. `RULE STATUS ≠ REPORT PRESENTATION`: Evaluated rule statuses (`PASS`, `WARNING`, `FAIL`, `MANUAL_REVIEW`, `UNKNOWN`, `NOT_APPLICABLE`) map into report presentation classes (`Issue`, `Warning`, `Manual Review`, `Information`) without erasing underlying machine states.

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
- **Run Status:** Current lifecycle state (`CREATED`, `ACQUIRING`, `NORMALIZING`, `SNAPSHOT_FROZEN`, `EVALUATING`, `AGGREGATING`, `COMPLETED`, `FAILED`, `CANCELLED`).
- **Timestamp:** Started at and completed at dates/times.
- **Crawl Volume:** Total URLs discovered and total URLs crawled.
- **Finding Summary Counts:** Categorized counts by presentation classification:
  - Issues (`FAIL` results on critical/high rules)
  - Warnings (`WARNING` results or advisory failures)
  - Manual Review (`MANUAL_REVIEW` conditions or assisted reviews)
  - Information (`PASS` confirmations or structural observations)
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
- **Max Crawl URLs:** Upper bound on crawled URLs (default: 500 for initial tests; up to 10,000 for standard site audits).
- **Max Crawl Depth:** Maximum link distance from seed URL (default: 5).
- **Respect robots.txt:** Toggle whether crawler strictly honors robots directives (default: `true`).
- **Sitemap Discovery:** Toggle automatic detection and parsing of `/sitemap.xml` and robots-declared sitemaps (default: `true`).
- **Rendering Mode:** Selection between:
  - `HTTP Only` (fast, raw HTML extraction, standard mode)
  - `Headless Chrome / Edge` (executes client JavaScript where browser environment is available)
- **Politeness / Concurrency:** Request delay (ms) per host and maximum concurrent worker threads.

#### C. Explicit Project Policy Inputs
Explicit expectations supplied by the user. The engine **never** infers business policy from page content or URL paths.
- **Preferred Origin:** Canonical host/protocol expectation (e.g. `https://example.com`).
- **Expected Indexability Policy:** Default assumption for discovered content (`All URLs Expected Indexable` vs `Seed Only` vs `Manual`).
- **Sitemap Invariant Expected:** Expectation that all canonical live URLs should exist in XML sitemaps (default: `true`).
- **Snippet Policy:** Expected snippet restriction policy (`ALLOW_SNIPPET` vs restricted).
- **Googlebot Access Policy:** Target policy for Googlebot (`ALLOW` vs `DISALLOW`).
- **OAI-SearchBot Access Policy:** Target policy for OpenAI SearchBot (`ALLOW` vs `DISALLOW`).
- **GPTBot Training Policy:** Target policy for OpenAI training scraper (`ALLOW` vs `DISALLOW`).
- **Priority URLs (Optional):** Explicit list of business-critical URLs where failures trigger elevated review.

**Forbidden in V1 Configuration:**
- No cloud proxy network settings.
- No automated CAPTCHA bypass credentials.
- No third-party SEO tool API keys (Ahrefs, Semrush, Moz).
- No GSC / Bing Webmaster OAuth authentication prompts.

---

### Surface 3: Crawl Progress
**Purpose:** Provide real-time observability of evidence acquisition without blocking the user interface or making premature audit judgments.

**Live Metrics & State Displayed:**
- **Execution Phase:** Current pipeline stage:
  - `ACQUIRING` (HTTP requests, robots parsing, sitemap discovery, frontier expansion)
  - `NORMALIZING` (URL normalization, directive resolution, canonical mapping, graph metrics derivation)
  - `SNAPSHOT_FROZEN` (Evidence immutability lock)
  - `EVALUATING` (Rule engine evaluating 47 `AR-*` rules against snapshot)
  - `AGGREGATING` (Synthesizing findings and report presentation)
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
  - Estimated time remaining (heuristic based on frontier queue)
- **Run Controls:**
  - `Pause`: Pauses frontier dispatch and permits active buffers to drain.
  - `Resume`: Resumes frontier scheduling.
  - `Cancel`: Immediately halts acquisition, persisting partial data up to the last clean buffer.

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
1. **Issues:** Deterministic and critical failures requiring immediate technical remediation (e.g., broken links, 5xx server errors, canonical targets failing, accidental `noindex`).
2. **Warnings:** Advisory anomalies, non-optimal implementations, or crawl friction that do not constitute fatal access failures (e.g., trailing slash inconsistencies, redirect chains, missing meta descriptions, unoptimized snippet directives).
3. **Manual Review:** Findings where technical evidence was verified but conclusive evaluation requires human business context or template intent (e.g., soft-404 patterns, ambiguous canonical consolidation, client-rendered content discrepancies).
4. **Information:** Verified structural architecture, successful crawl confirmations, and neutral configuration facts (e.g., sitemap discovered, HTTPS enforced, robots.txt valid).

**Separation of Crawler Metrics:**
Crawler metrics are displayed in an adjacent summary block, clearly separated from findings:
- Total URLs Discovered vs Crawled
- HTTP Status Distribution (2xx, 3xx, 4xx, 5xx)
- Maximum and Average Discovered Crawl Depth
- Protocol Breakdown (HTTPS vs HTTP)
- Total Acquisition Duration and Snapshot Timestamp

---

### Surface 5: Findings (List & Detail)
**Purpose:** The central analytical surface of the product, organizing evaluated rule results into understandable, prioritized technical findings.

**Finding Record Contract:**
Each finding card/row in the list and detail view exposes:
- **Finding Title:** Clear, human-readable summary of the condition (e.g., *"Canonical target returns 404 Not Found"*).
- **Relevant Rule ID(s):** Exact atomic identifier(s) from the frozen manifest (e.g., `AR-CANON-002`).
- **Parent Catalog Check:** Broader domain category reference (e.g., `CANON-003`).
- **Rule Result Status:** Underlying machine evaluation status:
  - `FAIL` | `WARNING` | `MANUAL_REVIEW` | `PASS` | `UNKNOWN` | `NOT_APPLICABLE`
- **Default Severity:** Frozen severity tier: `P0` (Critical blocker), `P1` (Major defect), `P2` (Moderate issue), `P3` (Minor advisory).
- **Presentation Classification:** `Issue` | `Warning` | `Manual Review` | `Information`.
- **Automation Class:** `deterministic` | `assisted` | `manual`.
- **Affected URL Count:** Number of unique URLs exhibiting this exact result.
- **Sample Affected URLs:** Quick-reference sample list of affected endpoints.
- **Observed Evidence Summary:** Concise factual statement of what was measured.
- **Expected State:** Normative technical baseline or explicit project policy expectation.
- **Recommended Action:** Clear, plain-language engineering guidance for remediation.

**Aggregation Rule:**
- Findings aggregate atomic `RuleResult` instances across URLs sharing the same `rule_id`, `status`, and `severity`.
- Aggregations **never** guess root causes (e.g., *"This is caused by WordPress plugin X"*) unless explicitly verified by deterministic rule evidence.

---

### Surface 6: URL / Evidence Inspector
**Purpose:** Deep technical inspection of an individual URL, displaying every piece of normalized evidence collected alongside every evaluated rule verdict.

**Foundational Rule:**
`OBSERVATION ≠ CONCLUSION`. The inspector renders observed facts and rule conclusions in separate, distinct UI panels.

**Ten Evidence Domains Displayed:**
1. **HTTP Response:** Fetch status code, protocol (HTTP/1.1, HTTP/2), TLS certificate validity, response time (ms), server headers (Content-Type, Cache-Control, Date).
2. **Redirects & Hops:** Full ordered redirect chain (`RedirectHop[]`), HTTP status per hop, `Location` header targets, redirect loop flag, final destination URL.
3. **Robots Directives & Decisions:** Document fetch status for `/robots.txt`, effective line matched, allow/disallow decisions evaluated across specific bot profiles (`Default`, `Googlebot`, `OAI-SearchBot`, `GPTBot`).
4. **Page Index Directives:** Raw `<meta name="robots">` tags and `X-Robots-Tag` HTTP headers, parsed token list, unsupported tokens, and calculated `effective_noindex` / `effective_nofollow` flags.
5. **Canonical Configuration:** Raw `<link rel="canonical">` tag values, resolved absolute URL, canonical target fetch status, target indexability state, and self-canonical determination.
6. **Title, Meta Description & Headings:** Raw title text, meta description text, array of `<h1>` values, character/pixel length metrics, duplicate heading detections.
7. **Internal Link Graph:** Inbound link count (`crawl_inlink_count`), discovered crawl depth (`crawl_depth`), list of source inlinks with anchor text and DOM location context (`MAIN`, `NAV`, `FOOTER`, `HEADER`), list of outbound internal links.
8. **Sitemap Presence:** Presence in XML sitemaps, sitemap document URL, raw and normalized `<lastmod>` values.
9. **Structured Data:** Extracted blocks by format (`JSON_LD`, `MICRODATA`, `RDFA`), schema types declared, syntax validation state, and JSON parsing error messages.
10. **Raw vs Rendered DOM Comparison:** When rendered HTML is available, visual diff of normalized fields: raw vs rendered title, raw vs rendered canonical, raw vs rendered meta robots, and JavaScript-injected internal links.

**Evaluated Rule Results Panel:**
A dedicated panel showing all atomic rules evaluated for this URL:
- Table listing `rule_id`, rule name, `status`, `severity`, primary evidence reference, and link back to the parent finding.

---

## 5. Finding Example Flows

The following flows illustrate how raw technical observations translate into reproducible rule results, aggregated findings, and recommended actions without false certainty.

### Flow 1: Internal Link Target Returns 404 (Broken Link)
- **Observed Evidence:**
  - Crawler fetches `https://example.com/about`.
  - HTML parser discovers `<a href="/team-bios">` located in `MAIN` content.
  - Discovery frontier schedules `https://example.com/team-bios` (`FetchObservation`).
  - Fetch attempt returns HTTP response status `404 Not Found`.
- **Rule Evaluation:**
  - Evaluator executes `AR-LINK-001` (Broken internal link destination).
  - Condition: internal link target returns 4xx/5xx status.
  - Result: `RuleResult` status `FAIL`, severity `P1`, subject `https://example.com/team-bios`.
  - Evidence Ref: Primary = `FetchObservation(status=404)`, Supporting = `LinkObservation(source=/about, anchor="Meet the Team")`.
- **Finding Model:**
  - Title: *"Internal link points to broken page (404 Not Found)"*
  - Rule ID: `AR-LINK-001` (Parent: `LINK-001`)
  - Classification: `Issue`
  - Severity: `P1`
  - Affected URLs: 1 link target (`/team-bios`), referenced by 3 source pages.
- **Affected URL Inspector:**
  - User inspects `/team-bios`: Sees HTTP status 404, zero content length.
  - User reviews inlinks: Identifies exact source pages (`/about`, `/company`, `/careers`) and anchor text.
- **Recommended Action:**
  - *"Update or remove broken internal links pointing to `/team-bios` across 3 referring pages, or restore the destination URL with a 200 OK response."*

---

### Flow 2: Canonical Target Failure
- **Observed Evidence:**
  - Page `https://example.com/products/widget-blue` returns `200 OK`.
  - HTML contains `<link rel="canonical" href="https://example.com/products/widget-all">`.
  - Evidence planner schedules probe fetch for canonical target `https://example.com/products/widget-all`.
  - Canonical target returns `404 Not Found`.
- **Rule Evaluation:**
  - Evaluator executes `AR-CANON-002` (Canonical target status failure).
  - Precondition: canonical declared and target fetched.
  - Condition: target status is 4xx.
  - Result: `RuleResult` status `FAIL`, severity `P0`, subject `/products/widget-blue`.
  - Evidence Ref: Primary = `CanonicalObservation(target=/products/widget-all)`, Supporting = `FetchObservation(status=404)`.
- **Finding Model:**
  - Title: *"Canonical URL target does not exist (returns 404)"*
  - Rule ID: `AR-CANON-002` (Parent: `CANON-003`)
  - Classification: `Issue`
  - Severity: `P0`
  - Affected URLs: 12 product variant pages declaring `/products/widget-all`.
- **Affected URL Inspector:**
  - User inspects `/products/widget-blue`: Under Canonical Domain, sees raw declared canonical URL, resolved URL, and red target fetch badge `404 Not Found`.
- **Recommended Action:**
  - *"Correct the canonical link tag on affected pages to point to an accessible, valid 200 OK canonical URL, or recreate the canonical destination page."*

---

### Flow 3: Conflicting Robots Index Directives
- **Observed Evidence:**
  - URL `https://example.com/checkout/confirmation` is crawled.
  - HTTP response header includes: `X-Robots-Tag: noindex, nofollow`.
  - HTML `<head>` includes: `<meta name="robots" content="index, follow">`.
  - Project policy: `expected_indexable` is not specified.
- **Rule Evaluation:**
  - Normalizer calculates: `effective_noindex = true` (strictest restriction prevails in search engine processing), but detects directive conflict.
  - Evaluator executes `AR-INDEX-003` (Conflicting robots directives).
  - Condition: contradictory index/noindex signals across meta and headers.
  - Result: `RuleResult` status `WARNING`, severity `P2`.
- **Finding Model:**
  - Title: *"Conflicting robots directives between HTTP header and HTML meta tag"*
  - Rule ID: `AR-INDEX-003` (Parent: `INDEX-001`)
  - Classification: `Warning`
  - Severity: `P2`
  - Affected URLs: 1 URL.
- **Affected URL Inspector:**
  - Page Directives section shows:
    - `X-Robots-Tag`: `noindex, nofollow` (HTTP Header)
    - `meta robots`: `index, follow` (HTML `<head>`)
    - Effective calculated state: `noindex` (Search engines prioritize restriction).
- **Recommended Action:**
  - *"Align index directives between the server HTTP header and the HTML `<head>`. Remove contradictory directives to ensure predictable indexing behavior."*

---

### Flow 4: Assisted / Contextual Review (Snippet Restrictions on Core Content)
- **Observed Evidence:**
  - URL `https://example.com/guide/pricing` returns `200 OK`.
  - HTML contains: `<meta name="robots" content="max-snippet:20">`.
  - Page is identified in sitemap and has 25 internal inlinks.
  - Project policy supplies: `snippet_policy = ALLOW_SNIPPET`.
- **Rule Evaluation:**
  - Evaluator executes `AR-INDEX-007` (Snippet restrictions restrict quotation on expected snippet-eligible page).
  - Automation: `assisted`.
  - Precondition: explicit snippet policy supplied.
  - Condition: `max-snippet:20` severely truncates textual snippet extraction in search and AI answer engines.
  - Result: `RuleResult` status `WARNING`, severity `P2`.
- **Finding Model:**
  - Title: *"Severe snippet length restriction on key content page"*
  - Rule ID: `AR-INDEX-007` (Parent: `INDEX-007`)
  - Classification: `Warning`
  - Severity: `P2`
- **Affected URL Inspector:**
  - Inspector displays exact meta tag content: `max-snippet:20`.
  - Displays context note: *"Page is configured with a 20-character snippet limit, preventing search engines and AI assistants from generating informative preview text."*
- **Recommended Action:**
  - *"Review whether the 20-character snippet limit is intentional for this guide page. Remove `max-snippet:20` or increase the character threshold to allow standard search previews and AI citations."*

---

### Flow 5: AI Search Crawler Policy Evaluation
- **Observed Evidence:**
  - Site `/robots.txt` contains:
    ```text
    User-agent: GPTBot
    Disallow: /

    User-agent: OAI-SearchBot
    Allow: /
    ```
  - Project policy supplies:
    - `oai_searchbot_access_policy = ALLOW`
    - `gptbot_training_policy = DISALLOW`
- **Rule Evaluation:**
  - Evaluator executes `AR-AI-001` (OAI-SearchBot access allowed when AI Search visibility expected).
    - Condition: `OAI-SearchBot` decision is `ALLOWED` matching policy `ALLOW`.
    - Result: `RuleResult` status `PASS`.
  - Evaluator executes `AR-AI-002` (GPTBot access aligns with explicit AI training policy).
    - Condition: `GPTBot` decision is `DISALLOWED` matching policy `DISALLOW`.
    - Result: `RuleResult` status `PASS`.
- **Finding Model:**
  - Title: *"OpenAI search crawler allowed while training bot restricted"*
  - Rule IDs: `AR-AI-001`, `AR-AI-002` (Parent: `AI-001`)
  - Classification: `Information`
  - Severity: `P3`
- **Affected URL Inspector:**
  - Robots decision panel displays:
    - `OAI-SearchBot`: `Allowed` (matches intent to appear in ChatGPT Search).
    - `GPTBot`: `Disallowed` (matches intent to prevent model training scraping).
- **Recommended Action:**
  - *"Configuration is consistent with project policy: content is accessible for search retrieval while protected from generative model training."*

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
3. **No Universal "GEO Score":**
   - V1 strictly rejects arbitrary composite metrics (e.g. *"GEO Readiness: 64%"*).
   - Findings in this domain are strictly factual: access availability, snippet quotation permissions, machine text extractability, and valid schema syntax.
4. **No Unsupported Probability Claims:**
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
- **DOM Render Comparisons:** Raw vs rendered extraction diff ledger.

### Product Positioning
- Raw crawler views are accessible via the `Raw Crawl Data` tab.
- They serve technical debugging and investigative validation.
- They do **not** replace the primary audit workflow (`Audit Overview` → `Findings` → `Evidence Inspector`).

---

## 8. Data-to-Product Mapping

The product surfaces map directly to the frozen logical entities defined in `knowledge/09-data-model.md`:

| Product Surface | Required Logical Entities / Data Elements | Produced By Layer / Module |
|---|---|---|
| **1. Projects / Audits** | `AuditRun`, finding counts by presentation classification, crawl counters | Orchestrator & Persistence Repository |
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
- Rule evaluation logic is deterministic, reproducible, and self-contained.
- Evaluators output `RuleResult` instances linked to `RuleEvidenceRef`.
- Result aggregation synthesizes findings without altering atomic rule statuses.

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

## 11. Verification Checklist

Before accepting this workflow specification into the project documentation:
- [x] Verified consistency with MVP scope in `knowledge/05-mvp-scope.md`.
- [x] Verified terminology matches frozen contracts in `knowledge/08-system-architecture.md` and `knowledge/09-data-model.md`.
- [x] Confirmed all rule references align with `knowledge/07-v1-atomic-rule-manifest.md`.
- [x] Confirmed absolute exclusion of score models (SEO score, GEO score, health grade).
- [x] Confirmed strict separation of Observation vs Conclusion and Rule Status vs Report Presentation.
- [x] Confirmed AI Search guardrails (OAI-SearchBot vs GPTBot distinction, no citation guarantee).
- [x] Confirmed that current engineering migration sequencing (A2 -> A3/A4 -> A5 -> A6 -> A7) is completely preserved.
