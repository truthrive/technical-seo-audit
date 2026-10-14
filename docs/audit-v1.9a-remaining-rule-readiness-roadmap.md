# Audit V1.9a: Remaining Atomic Rule Readiness & Implementation Roadmap

## 1. Executive Summary

- **Audit Milestone:** Audit V1.9a — Remaining Atomic Rule Readiness & Implementation Roadmap Review
- **Repository:** `truthrive/technical-seo-audit`
- **Branch:** `main`
- **Baseline Git HEAD:** `65bbbb01fa85bca1d2b1a0235d4ecf53b66735b1`
- **NormalizationVersion:** Strictly `v1.9.0` (unchanged)
- **Current Executable Rules:** Exactly `19 / 47` (40.4% coverage; unchanged in V1.9a)
- **Remaining Rules Assessed:** Exactly `28 / 47` (59.6% remaining)
- **Sitemap Rule Group:** `6 / 6` implemented (`100%` complete as of V1.8g)
- **Entity / Structured Data Rule Group:** `1 / 1` implemented (`100%` complete as of V1.7b)
- **Design-Only Assessment:** This milestone is strictly an architectural readiness review and implementation roadmap. Zero executable evaluator code, zero rule registrations, zero crawler modifications, zero SQLite schema diffs, zero adapter normalization alterations, and zero policy changes are introduced.

### Primary Milestone Conclusions

1. **Exact 47-Rule Inventory Reconciliation:** The frozen manifest (`knowledge/07-v1-atomic-rule-manifest.md`) and the machine-readable rule registry (`internal/audit/rules_v1.json`) agree on all 47 atomic rules. Across the codebase, exactly 19 rules are registered with typed executable evaluators in `internal/audit/engine/engine.go`, and exactly 28 rules remain unimplemented.
2. **Immediate Implementation Readiness:** Exactly **one** rule among the 28 remaining is classified as **`READY_TO_IMPLEMENT`** without any crawler, database, adapter, or policy modifications: **`AR-CANON-005 — Canonical URL is syntactically resolvable`**. 100% of required observations (`url_identity`, `canonical_count`, `canonical_resolved`, `canonical_normalization_complete`, `canonical_normalized_target`) are already extracted and normalized on `SubjectURL` by the V1.8c adapter.
3. **Primary Blocker Distribution Across the 28 Remaining Rules:**
   - **`READY_TO_IMPLEMENT` (1 rule):** `AR-CANON-005`.
   - **`NEEDS_ADAPTER_WIRING` (4 rules):** `AR-INDEX-003`, `AR-CANON-010`, `AR-LINK-005`, `AR-INDEX-005` (existing raw crawler/page data exists in SQLite, but normalized fields or target correlations are not yet emitted onto `EvidenceSnapshot`).
   - **`NEEDS_POLICY_WIRING` (2 rules):** `AR-ACC-003`, `AR-ACC-002` (policy models are frozen in `policy.go`, but explicit assignments must be wired to evaluators alongside minor adapter correlations).
   - **`MANUAL_REVIEW_DEPENDENT` (1 rule):** `AR-LINK-006` (requires manual review task generation and human review workflow; no automated pass/fail threshold exists by frozen contract).
   - **`BLOCKED` (1 rule):** `AR-ACC-007` (blocked by absence of sub-resource request tracking and render diff dependency proof).
   - **`NEEDS_ACQUISITION` (19 rules):** Diagnostic active probes (origin, trailing slash, 404 template), multi-agent robots AST parsing (Googlebot, OAI-SearchBot, GPTBot), dual raw/rendered DOM artifact storage, and multi-profile User-Agent comparative fetching are not supported by the standalone crawler.
4. **Recommended Next Implementation Target:** **Audit V1.9b — Implement `AR-CANON-005` (Canonical URL is syntactically resolvable)**. This completes the canonical declaration integrity trio (`AR-CANON-003`, `AR-CANON-004`, `AR-CANON-005`) and increases executable rule count from 19 to 20/47 (42.5%) with zero schema or adapter risk.

---

## 2. Verified Baseline and Rule Count

### 2.1 Git Preflight Verification

- Working tree status: clean (`git status` reports nothing to commit, working tree clean).
- Baseline SHA: `65bbbb01fa85bca1d2b1a0235d4ecf53b66735b1` (confirmed via `git rev-parse HEAD`).
- Active branch: `main`.

### 2.2 Authoritative Source Cross-Reference

This readiness review was synthesized by cross-referencing:
1. `knowledge/INDEX.md` — Document routing and precedence hierarchy.
2. `knowledge/00-product-principles.md` — Diagnostic integrity, zero false positives, explicit policy separation.
3. `knowledge/01-audit-framework.md` — Core lifecycle stages (Crawl, Discover, Render, Index, Consolidate).
4. `knowledge/02-audit-check-catalog.md` — Parent diagnostic check taxonomy (ACC, INDEX, CANON, LINK, DISC, RENDER, AI).
5. `knowledge/03-rule-engine-spec.md` — Hermetic engine execution, frozen snapshot immutability, PolicyIndex contract.
6. `knowledge/04-ai-search-geo.md` — AI policy models (OAI-SearchBot, GPTBot) and strict factual guardrails against speculative SEO.
7. `knowledge/05-mvp-scope.md` — V1 boundaries and deferred capabilities.
8. `knowledge/07-v1-atomic-rule-manifest.md` — Authoritative semantic definitions, allowed verdicts, preconditions, and evidence requirements.
9. `knowledge/08-system-architecture.md` — Architectural boundaries between crawler, SQLite, adapter, and engine.
10. `knowledge/09-data-model.md` — Domain entity schemas, observation types, policy vocabulary.
11. `knowledge/10-implementation-plan.md` — Phased sequencing and progression strategy.
12. `internal/audit/rules_v1.json` — Machine-readable rule registry defining the 47 frozen rules.
13. Source implementation in `internal/audit/engine/**`, `internal/audit/adapter/**`, and `internal/sitecrawl/**`.

### 2.3 Rule Count Reconciliation: 47 Total = 19 Implemented + 28 Remaining

The frozen atomic rule manifest establishes 47 rules. Verification against `internal/audit/engine/engine.go` confirms:

```
Total Frozen Atomic Rules:        47
Implemented Executable Evaluators: 19
Remaining Unimplemented Rules:     28
Discrepancy:                       0
```

#### Breakdown by Functional Category

| Category | Parent Checklist | Total Rules | Implemented Rules | Remaining Rules | Implemented Rule IDs | Remaining Rule IDs |
|---|---|---:|---:|---:|---|---|
| **Access & Transport** | `ACC-001` to `ACC-007` | 8 | 1 | 7 | `AR-ACC-004` | `AR-ACC-001`, `AR-ACC-002`, `AR-ACC-003`, `AR-ACC-005`, `AR-ACC-006`, `AR-ACC-007`, `AR-ACC-008` |
| **Indexability Directives** | `INDEX-001` to `INDEX-007` | 6 | 2 | 4 | `AR-INDEX-001`, `AR-INDEX-002` | `AR-INDEX-003`, `AR-INDEX-004`, `AR-INDEX-005`, `AR-INDEX-006` |
| **Canonicalization & Redirects** | `CANON-001` to `CANON-007` | 11 | 6 | 5 | `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, `AR-CANON-007`, `AR-CANON-008`, `AR-CANON-009` | `AR-CANON-001`, `AR-CANON-002`, `AR-CANON-005`, `AR-CANON-010`, `AR-CANON-011` |
| **Internal Links & Graph** | `LINK-001` to `LINK-004` | 6 | 3 | 3 | `AR-LINK-002`, `AR-LINK-003`, `AR-LINK-004` | `AR-LINK-001`, `AR-LINK-005`, `AR-LINK-006` |
| **Sitemaps & Discovery** | `DISC-001` to `DISC-006` | 6 | 6 | 0 | `AR-DISC-001`, `AR-DISC-002`, `AR-DISC-003`, `AR-DISC-004`, `AR-DISC-005`, `AR-DISC-006` | *(None — 100% implemented)* |
| **Structured Data & Entity** | `ENTITY-001` | 1 | 1 | 0 | `AR-ENTITY-001` | *(None — 100% implemented)* |
| **JavaScript Rendering** | `RENDER-001`, `RENDER-002` | 6 | 0 | 6 | *(None)* | `AR-RENDER-001`, `AR-RENDER-002`, `AR-RENDER-003`, `AR-RENDER-004`, `AR-RENDER-005`, `AR-RENDER-006` |
| **AI Search & Policy** | `AI-001`, `AI-002` | 3 | 0 | 3 | *(None)* | `AR-AI-001`, `AR-AI-002`, `AR-AI-003` |
| **Total** | | **47** | **19** | **28** | | |

---

## 3. Complete 47-Rule Inventory

The following table provides the full inventory of all 47 frozen atomic rules, identifying their lifecycle, automation level, required inputs, preconditions, allowed verdicts, default severity, implementation status, and registered evaluator.

| Rule ID | Name | Parent Check | Lifecycle | Auto Type | Preconditions | Allowed Verdicts | Def Sev | Status | Evaluator Function |
|---|---|---|---|---|---|---|---|---|---|
| `AR-ACC-001` | HTTPS endpoint reachable | `ACC-001` | Crawl | deterministic | HTTPS origin derived from project origin | PASS, FAIL, UNKNOWN | P0 | Remaining | *(Unimplemented)* |
| `AR-ACC-002` | Robots access matches explicit crawl intent | `ACC-003` | Crawl | assisted | robots.txt parsed; expected_crawlable=true | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-ACC-003` | HTTP status matches explicit expected URL state | `ACC-007` | Crawl | deterministic | expected_url_state is live, redirect, or missing | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-ACC-004` | Unexpected server-error response | `ACC-006` | Crawl | deterministic | HTTP response obtained | PASS, FAIL, UNKNOWN | P1 | **Implemented** | `evaluateACC004` (`acc004.go`) |
| `AR-ACC-005` | Googlebot robots access matches explicit policy | `ACC-002` | Crawl | assisted | Googlebot robots policy parseable | PASS, WARNING, FAIL, UNKNOWN | P1 | Remaining | *(Unimplemented)* |
| `AR-ACC-006` | Googlebot profile response anomaly | `ACC-002` | Crawl | assisted | normal and Googlebot-profile requests attempted | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-ACC-007` | Blocked CSS/JS resource affects rendered output | `ACC-004` | Render | assisted | CSS/JS referenced by rendered page | PASS, WARNING, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-ACC-008` | Bot-profile access restriction observed | `ACC-005` | Crawl | assisted | normal and alternate bot-profile requests completed | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-INDEX-001` | Effective noindex conflicts with explicit indexability intent | `INDEX-002` | Index | assisted | directive evaluation completed; expected_indexable explicitly supplied | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateINDEX001` (`index001.go`) |
| `AR-INDEX-002` | Conflicting index directives | `INDEX-001` | Index | deterministic | robots directives present | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateINDEX002` (`index002.go`) |
| `AR-INDEX-003` | Malformed or unsupported robots directive | `INDEX-001` | Index | deterministic | meta robots or X-Robots-Tag exists | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-INDEX-004` | Known-missing probe resolves to 404/410 | `INDEX-004` | Index | deterministic | collision-resistant URL generated | PASS, FAIL, UNKNOWN | P1 | Remaining | *(Unimplemented)* |
| `AR-INDEX-005` | Snippet restriction conflicts with snippet policy | `INDEX-007` | Index | assisted | snippet directives extracted | PASS, WARNING, FAIL, MANUAL_REVIEW, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-INDEX-006` | 200 response matches known 404 template | `INDEX-003` | Index | assisted | target is 200; missing probe returned 404/410 with fingerprint | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-CANON-001` | Origin variants normalize to preferred origin | `CANON-001` | Consolidate | deterministic | explicit preferred origin; variant returns HTTP response | PASS, WARNING, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-CANON-002` | Trailing-slash pair is consistently normalized | `CANON-002` | Consolidate | deterministic | both variants evaluable | PASS, WARNING, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-CANON-003` | Canonical declaration missing | `CANON-003` | Consolidate | assisted | final response is 2xx HTML | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateCANON003` (`canon003.go`) |
| `AR-CANON-004` | Multiple canonical declarations | `CANON-003` | Consolidate | deterministic | at least one canonical declaration exists | PASS, WARNING, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateCANON004` (`canon004.go`) |
| `AR-CANON-005` | Canonical URL is syntactically resolvable | `CANON-003` | Consolidate | deterministic | exactly one canonical declaration exists | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-CANON-006` | Canonical target returns final 200 | `CANON-003` | Consolidate | deterministic | exactly one valid target; fetch attempted | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateCANON006` (`canon006.go`) |
| `AR-CANON-007` | Canonical target is not noindex | `CANON-003` | Consolidate | deterministic | exactly one valid target; target returns 200 | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateCANON007` (`canon007.go`) |
| `AR-CANON-008` | Redirect chain exceeds one hop | `CANON-007` | Consolidate | deterministic | initial response redirects | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateCANON008` (`canon008.go`) |
| `AR-CANON-009` | Redirect loop detected | `CANON-007` | Consolidate | deterministic | at least one redirect occurs | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateCANON009` (`canon009.go`) |
| `AR-CANON-010` | Redirect final target is available | `CANON-007` | Consolidate | deterministic | redirect exists and no loop detected | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-CANON-011` | Pagination exposes crawlable next-page link | `CANON-006` | Discover | assisted | paginated sequence identified; not final page | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-LINK-001` | Internal navigation target lacks crawlable href | `LINK-001` | Discover | assisted | internal navigation candidate has resolvable target | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-LINK-002` | Internal link targets 3xx | `LINK-002` | Discover | deterministic | resolved target is internal and fetchable | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateLINK002` (`link.go`) |
| `AR-LINK-003` | Internal link targets 4xx | `LINK-002` | Discover | deterministic | resolved target is internal and fetchable | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateLINK003` (`link.go`) |
| `AR-LINK-004` | Internal link targets 5xx | `LINK-002` | Discover | deterministic | resolved target is internal and fetchable | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateLINK004` (`link.go`) |
| `AR-LINK-005` | Orphan candidate relative to supplied discovery sources | `LINK-003` | Discover | assisted | URL in at least one supplied non-crawl source and not start URL | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-LINK-006` | Priority-page crawl depth review | `LINK-004` | Discover | manual | priority_page=true explicitly supplied | PASS, FAIL, MANUAL_REVIEW, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-DISC-001` | Sitemap presence matches project policy | `DISC-001` | Discover | assisted | root/robots sitemap discovery sources evaluated | PASS, WARNING, MANUAL_REVIEW, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateDISC001` (`disc001.go`) |
| `AR-DISC-002` | Sitemap document returns 200 | `DISC-002` | Discover | deterministic | sitemap discovered via robots.txt, common path, or index | PASS, FAIL, UNKNOWN | P1 | **Implemented** | `evaluateDISC002` (`disc002.go`) |
| `AR-DISC-003` | Sitemap is parseable | `DISC-003` | Discover | deterministic | sitemap fetch completed with response body | PASS, FAIL, UNKNOWN | P1 | **Implemented** | `evaluateDISC003` (`disc003.go`) |
| `AR-DISC-004` | Sitemap-listed URL returns final 200 | `DISC-004` | Discover | deterministic | sitemap entry parsed with resolvable loc | PASS, FAIL, UNKNOWN | P1 | **Implemented** | `evaluateDISC004` (`disc004.go`) |
| `AR-DISC-005` | Sitemap-listed URL is not noindex | `DISC-005` | Discover | deterministic | sitemap entry loc resolved and target returns 200 | PASS, FAIL, UNKNOWN | P1 | **Implemented** | `evaluateDISC005` (`disc005.go`) |
| `AR-DISC-006` | Sitemap-listed URL canonicalizes to itself | `DISC-006` | Discover | deterministic | sitemap entry target returns 200 HTML and canonical declared | PASS, FAIL, UNKNOWN, NOT_APPLICABLE | P1 | **Implemented** | `evaluateDISC006` (`disc006.go`) |
| `AR-ENTITY-001` | Structured-data block parses successfully | `ENTITY-001` | Discover | deterministic | JSON-LD / Microdata block extracted | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P2 | **Implemented** | `evaluateENTITY001` (`entity001.go`) |
| `AR-RENDER-001` | Title changes after render | `RENDER-002` | Render | deterministic | raw and rendered observations available | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-RENDER-002` | Canonical changes after render | `RENDER-002` | Render | deterministic | raw and rendered observations available | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-RENDER-003` | Robots directives change after render | `RENDER-002` | Render | deterministic | raw and rendered observations available | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-RENDER-004` | H1 set changes after render | `RENDER-001` | Render | deterministic | raw and rendered observations available | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-RENDER-005` | Main-text presence changes after render | `RENDER-001` | Render | deterministic | raw and rendered observations available | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-RENDER-006` | Crawlable internal-link set changes after render | `RENDER-001` | Render | deterministic | raw and rendered observations available | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P1 | Remaining | *(Unimplemented)* |
| `AR-AI-001` | OAI-SearchBot robots access matches project policy | `AI-001` | Crawl | assisted | OAI-SearchBot robots policy can be evaluated | PASS, WARNING, FAIL, UNKNOWN | P2 | Remaining | *(Unimplemented)* |
| `AR-AI-002` | OAI-SearchBot profile encounters access anomaly | `AI-001` | Crawl | assisted | default and OAI-profile requests attempted | PASS, WARNING, UNKNOWN, NOT_APPLICABLE | P2 | Remaining | *(Unimplemented)* |
| `AR-AI-003` | GPTBot configuration matches explicit training policy | `AI-002` | Crawl | assisted | GPTBot robots state can be evaluated | PASS, WARNING, MANUAL_REVIEW, UNKNOWN | P2 | Remaining | *(Unimplemented)* |

---

## 4. Remaining 28-Rule Evidence Readiness Matrix

Each of the 28 unimplemented rules has been evaluated through the end-to-end trace:  
`Frozen rule contract → required inputs → SiteCrawl acquisition → SQLite → Adapter → EvidenceSnapshot → Engine`.

The 12-column matrix below details the exact readiness state for every remaining rule.

| Rule ID | Rule Name | Automation | Required Evidence | Available Evidence | Missing Evidence | Policy Dependency | Contract Dependency | Code References | Readiness | Recommended Phase | Rationale |
|---|---|---|---|---|---|---|---|---|---|---|---|
| `AR-ACC-001` | HTTPS endpoint reachable | deterministic | `site_https_url`, `fetch_attempted`, `tls_valid`, `fetch_error_type`, `fetch_status` | `url_identity`, `http_status`, `fetch_error_type` on crawled pages | Dedicated HTTPS origin probe; TLS certificate validity verification | None | None | `internal/audit/adapter/adapter.go:640` (`tls_valid: nil`); `sitecrawl_pages` | `NEEDS_ACQUISITION` | V2.0a (Active Probes) | Crawled pages record HTTP status, but origin-level HTTPS reachability probe with explicit TLS handshake verification does not exist. |
| `AR-ACC-002` | Robots access matches explicit crawl intent | assisted | `url`, `robots_allowed_default`, `matched_robots_rule`, `expected_crawlable` | `sitecrawl_pages.robots_state` (allowed/blocked/unknown) | Normalized `robots_allowed_default` and `matched_robots_rule` on `SubjectURL`; granular matched pattern AST | `expected_crawlable` (URL scope) | None | `internal/sitecrawl/robots.go:28`; `internal/audit/adapter/adapter.go:2075`; `policy.go:60` | `NEEDS_ADAPTER_WIRING` | V1.15a (Crawler Intent & Robots) | `sitecrawl_pages` stores crawler UA outcome (`robots_state`), but the adapter does not emit `robots_allowed_default`, and the matched line/rule pattern is not persisted in SQLite. Secondary policy dependency. |
| `AR-ACC-003` | HTTP status matches explicit expected URL state | deterministic | `url`, `expected_url_state`, `fetch_status`, `final_status` | `url_identity`, `http_status` on `SubjectURL` | Correlation of destination `final_status` across redirects; policy index evaluation | `expected_url_state` (URL scope: `live`, `redirect`, `missing`) | None | `internal/audit/policy.go:59,169`; `internal/audit/adapter/adapter.go:673` | `NEEDS_POLICY_WIRING` | V1.13b (Expected URL State) | `expected_url_state` is fully modeled in `policy.go`. `http_status` is available. For redirecting pages, terminal status correlation is needed, but the primary task is connecting URL policy to the evaluation path. |
| `AR-ACC-005` | Googlebot robots access matches explicit policy | assisted | `url`, `robots_allowed_googlebot`, `googlebot_access_policy`, `matched_robots_rule` | Generic robots directives (`robots_directive_tokens`, `googlebot_meta_robots_raw`) | Googlebot-specific robots.txt AST parse and URL evaluation; matched directive line | `googlebot_access_policy` (Site scope: `allow`, `block`) | None | `internal/sitecrawl/robots.go:42`; `internal/audit/policy.go:54` | `NEEDS_ACQUISITION` | V2.1a (Robots Multi-Agent) | SiteCrawl only evaluates robots.txt for its own crawling User-Agent. It does not parse or evaluate a Googlebot-specific robots group against crawled URLs. |
| `AR-ACC-006` | Googlebot profile response anomaly | assisted | `url`, `default_profile_status`, `googlebot_profile_status`, `default_challenge_detected`, `googlebot_challenge_detected` | Single-profile fetch observations only | Dual-profile comparative HTTP fetch (default vs Googlebot UA); challenge header detection | None | Comparative threshold definition | `internal/sitecrawl/fetch.go`; `internal/audit/model.go:30` (`ProfileComparisonID`) | `NEEDS_ACQUISITION` | V2.2a (Multi-Profile Transport) | Requires issuing a secondary HTTP request with Googlebot User-Agent for comparison. Single-profile crawling cannot detect UA-specific blocking or cloaking. |
| `AR-ACC-007` | Blocked CSS/JS resource affects rendered output | assisted | `page_url`, `resource_url`, `resource_type`, `resource_robots_allowed`, `render_dependency_observed`, `render_diff_evidence` | None | Sub-resource request network log during render; sub-resource robots check; render output diff proof | None | Materiality threshold for rendered visual/content loss | `internal/sitecrawl/render.go`; `internal/audit/model.go:281` (`RenderResourceObservation`) | `BLOCKED` | Deferred (Post-V2) | Standalone crawler does not capture sub-resource network events or verify sub-resource robots.txt. Proving material output change caused by blocked CSS/JS requires DOM dependency diffing. |
| `AR-ACC-008` | Bot-profile access restriction observed | assisted | `url`, `default_profile_status`, `bot_profile_status`, `default_challenge_detected`, `bot_challenge_detected`, `tested_user_agent` | `sitecrawl_pages.data.botBlocked` (Boolean flag) | Multi-profile structured comparative HTTP fetch observations; WAF challenge marker taxonomy | None | None | `internal/sitecrawl/types.go:415`; `internal/audit/adapter/adapter.go:640` | `NEEDS_ACQUISITION` | V2.2a (Multi-Profile Transport) | While `botBlocked` exists in `Page` JSON, it is an opportunistic fallback marker rather than a hermetic comparative multi-profile test across controlled user-agents. |
| `AR-INDEX-003` | Malformed or unsupported robots directive | deterministic | `url`, `robots_directive_tokens`, `robots_parse_errors` | `robots_directive_tokens`, `meta_robots_raw`, `x_robots_raw` on `SubjectURL` | Explicit `robots_parse_errors` or `unsupported_tokens` normalized observation | None | **Authoritative V1 supported directive token vocabulary registry** | `internal/audit/adapter/adapter.go:2146`; `internal/audit/rules_v1.json:344` | `NEEDS_ADAPTER_WIRING` | V1.10b (Directive Syntax Integrity) | Raw tokens are emitted on `SubjectURL`. However, the repository lacks an approved frozen registry defining exact supported tokens vs malformed syntax. Primary blocker is contract decision on token taxonomy. |
| `AR-INDEX-004` | Known-missing probe resolves to 404/410 | deterministic | `missing_probe_url`, `probe_final_status`, `probe_final_url` | None | Active diagnostic probe generator for non-existent probe URL; terminal status recording | None | None | `internal/audit/model.go:296` (`ProbeObservation`); `internal/sitecrawl/crawler.go` | `NEEDS_ACQUISITION` | V2.0a (Active Probes) | Crawling only visits discovered links and sitemaps. A synthetic collision-resistant probe request (e.g. `/sitecrawl-probe-nonexistent-404-uuid`) must be executed to verify server 404 handling. |
| `AR-INDEX-005` | Snippet restriction conflicts with snippet policy | assisted | `url`, `nosnippet`, `max_snippet`, `data_nosnippet_count`, `snippet_policy` | Directives parsed (`directive_tokens`); meta robots raw | Normalized snippet token observations (`nosnippet`, `max_snippet`); HTML `data-nosnippet` count | `snippet_policy` (Site scope: `allow_unrestricted`, `restrict`) | None | `internal/audit/policy.go:53,146`; `internal/audit/adapter/adapter.go:1701` | `NEEDS_ADAPTER_WIRING` | V1.16a (Snippet Policy & Directives) | `snippet_policy` is modeled in `policy.go`. Directive tokens are split in adapter, but snippet-specific tokens (`nosnippet`, `max-snippet`) and HTML `data-nosnippet` elements are not normalized onto `SubjectURL`. |
| `AR-INDEX-006` | 200 response matches known 404 template | assisted | `url`, `fetch_status`, `content_fingerprint`, `known_404_content_fingerprint`, `known_404_probe_status` | `url_identity`, `http_status` on `SubjectURL` | Known-missing 404 probe execution; DOM structural/text content fingerprinting of 404 template | None | Fingerprint comparison threshold (exact hash vs DOM tree similarity) | `internal/audit/model.go:167` (`ContentFingerprint`); `internal/audit/model.go:296` | `NEEDS_ACQUISITION` | V2.0b (Soft 404 Detection) | Depends on `AR-INDEX-004` probe acquisition and normalized DOM content hashing/fingerprinting across 200 responses to detect soft-404s. |
| `AR-CANON-001` | Origin variants normalize to preferred origin | deterministic | `preferred_origin`, `origin_variant`, `variant_fetch_status`, `variant_final_url`, `redirect_hops` | None | Multi-origin active variant probes (http vs https, www vs non-www); redirect chain capture | `preferred_origin` (Site scope) | None | `internal/audit/policy.go:51,126`; `internal/audit/model.go:296` | `NEEDS_ACQUISITION` | V2.0a (Active Probes) | SiteCrawl only crawls URLs from seed frontier. Probing all 4 origin variants (`http://`, `https://`, `http://www.`, `https://www.`) requires dedicated origin probe runner. Secondary policy dependency. |
| `AR-CANON-002` | Trailing-slash pair is consistently normalized | deterministic | `slash_url`, `non_slash_url`, both response states, redirect paths, `normalized_content_hash` | Crawled URLs have status and content | Paired slash/non-slash request execution; normalized DOM content hash comparison | None | Pair selection heuristic (seed URL vs sampled internal URLs) | `internal/sitecrawl/crawler.go`; `internal/audit/model.go:296` | `NEEDS_ACQUISITION` | V2.0a (Active Probes) | Crawler canonicalizes URLs before fetch to avoid infinite loops, preventing dual fetch of slash/non-slash pairs without dedicated diagnostic probe runner. |
| `AR-CANON-005` | Canonical URL is syntactically resolvable | deterministic | `url`, `canonical_raw`, `canonical_resolved_url`, `canonical_parse_error` | `url_identity`, `canonical_count`, `canonical_resolved`, `canonical_normalization_complete`, `canonical_normalized_target` | **None** | None | None | `internal/audit/adapter/adapter.go:1087-1225,1596` (`normalizeCanonicalTarget`) | **`READY_TO_IMPLEMENT`** | **V1.9b (Canonical Syntax Integrity)** | **All required evidence is fully extracted, normalized, and indexed on `SubjectURL`. When `canonical_count == 1`, `canonical_normalization_complete` proves valid resolution. Evaluator logic is 100% implementable.** |
| `AR-CANON-010` | Redirect final target is available | deterministic | `url`, `redirect_hops`, `final_status`, `redirect_loop_detected` | `redirect_initial_observed`, `redirect_hop_count`, `redirect_hop`, `redirect_traversal_complete`, `redirect_loop_detected`, `redirect_final_url` | Terminal `final_status` observation on `SubjectURL`; target subject correlation for terminal hop | None | Contract for external redirect target terminal status | `internal/sitecrawl/fetch.go:190`; `internal/audit/adapter/adapter.go:886`; `internal/audit/engine/redirect_helpers.go` | `NEEDS_ADAPTER_WIRING` | V1.11b (Redirect Target Availability) | Hops and final URL are normalized. But `fetch.go` terminal status (`out.FinalStatus`) was not written to `sitecrawl_pages`. If target was crawled internally, adapter can correlate; if offsite, status is missing. Primary blocker: adapter correlation. |
| `AR-CANON-011` | Pagination exposes crawlable next-page link | assisted | `url`, `pagination_detected`, `has_next_page`, `next_page_url`, `next_page_element_tag`, `next_page_href` | `sitecrawl_pages.data.relNext` (raw string) | Pagination sequence identification; crawlable `<a href>` extraction; infinite scroll detection | None | Definition of pagination candidate elements beyond `<link rel=next>` | `internal/sitecrawl/types.go:435`; `internal/sitecrawl/extract.go` | `NEEDS_ACQUISITION` | V2.3a (Pagination Sequences) | Manifest guardrail explicitly forbids relying solely on deprecated `rel=next/prev` tags. HTML sequence identification and markup link extraction are required. |
| `AR-LINK-001` | Internal navigation target lacks crawlable href | assisted | `source_url`, `navigation_candidate`, `resolved_internal_target`, `element_tag`, `href` | Valid hyperlinks extracted in `sitecrawl_links` | Extraction of non-anchor navigation candidates (`<button>`, `<div onclick>`, script navigation) | None | Navigation candidate element selector contract | `internal/sitecrawl/extract.go`; `internal/audit/model.go:212` (`NavigationCandidate`) | `NEEDS_ACQUISITION` | V2.4a (JavaScript Navigation Discovery) | Crawler currently only extracts `<a href>` and `<link href>` tags. Elements functioning as internal navigation without valid `href` are not captured in SQLite. |
| `AR-LINK-005` | Orphan candidate relative to supplied discovery sources | assisted | `url`, `discovery_sources`, `crawl_inlink_count`, `is_start_url`, `crawl_complete` | `sitecrawl_pages.inlinks`; `sitecrawl_sitemap_entries.listed_url_raw` | Normalized `crawl_inlink_count` and `discovery_sources` on `SubjectURL` | None | None | `internal/sitecrawl/runs.go:88` (`inlinks`); `internal/audit/adapter/adapter.go:640` | `NEEDS_ADAPTER_WIRING` | V1.12b (Orphan URL Discovery) | Raw inlinks are counted in SQLite (`sitecrawl_pages.inlinks`) and sitemap entries exist in `sitecrawl_sitemap_entries`. The adapter does not emit `crawl_inlink_count` or supplemental source flags on `SubjectURL`. |
| `AR-LINK-006` | Priority-page crawl depth review | manual | `url`, `crawl_depth`, `priority_page` | `url_identity`, `crawl_depth` on `SubjectURL` | Manual review task generation; reviewer decision workflow | `priority_page` (URL scope) | Contract for manual review task records and human verdict entry | `internal/audit/adapter/adapter.go:706`; `internal/audit/policy.go:62,191`; `internal/audit/model.go:35` | `MANUAL_REVIEW_DEPENDENT` | V1.14b (Priority Crawl Depth Review) | `crawl_depth` is already normalized. `priority_page` is modeled in `policy.go`. By frozen manifest contract, this rule has no automated pass/fail threshold and produces `MANUAL_REVIEW`. Secondary policy dependency. |
| `AR-RENDER-001` | Title changes after render | deterministic | `url`, `title_raw`, `title_rendered` | Post-render `title` (when render enabled) or raw `title` (when disabled) | Concurrent dual-capture: pre-render raw server HTML title AND post-render headless browser title | None | None | `internal/sitecrawl/page.go:54`; `internal/audit/adapter/adapter.go:906` (`GapRenderedRawSourceUnavailable`) | `NEEDS_ACQUISITION` | V2.5a (Render Comparison Engine) | When rendering is active, SiteCrawl replaces raw server HTML with rendered DOM. Raw HTML title is discarded, preventing raw-vs-rendered comparison. |
| `AR-RENDER-002` | Canonical changes after render | deterministic | `url`, `canonical_raw`, `canonical_rendered` | Post-render or pre-render canonical declarations | Concurrent dual-capture of raw server canonical set AND rendered DOM canonical set | None | None | `internal/sitecrawl/render.go`; `internal/audit/adapter/adapter.go:906` | `NEEDS_ACQUISITION` | V2.5a (Render Comparison Engine) | Standalone crawler does not store raw server HTML alongside rendered DOM. Both sets must be persisted to detect client-side canonical modification. |
| `AR-RENDER-003` | Robots directives change after render | deterministic | `url`, `meta_robots_raw`, `meta_robots_rendered` | Post-render or pre-render meta robots directives; HTTP X-Robots-Tag | Concurrent dual-capture of raw server meta robots AND rendered DOM meta robots | None | None | `internal/sitecrawl/render.go`; `internal/audit/adapter/adapter.go:906` | `NEEDS_ACQUISITION` | V2.5a (Render Comparison Engine) | Dual capture required. Directives injected or altered by client-side JavaScript cannot be diffed without retaining pre-render server HTML. |
| `AR-RENDER-004` | H1 set changes after render | deterministic | `url`, `h1_raw`, `h1_rendered` | Post-render or pre-render H1 values | Concurrent dual-capture of raw server H1 set AND rendered DOM H1 set | None | None | `internal/sitecrawl/render.go`; `internal/audit/adapter/adapter.go:906` | `NEEDS_ACQUISITION` | V2.5a (Render Comparison Engine) | Dual capture required. Requires preserving both pre-render and post-render H1 heading arrays. |
| `AR-RENDER-005` | Main-text presence changes after render | deterministic | `url`, `main_text_present_raw`, `main_text_present_rendered` | Content word count / text ratio | Main-text presence signal extraction on raw HTML AND rendered DOM; text signal evidence | None | Main-text presence heuristic / word count threshold | `internal/sitecrawl/render.go`; `internal/audit/model.go:165` (`MainTextPresent`) | `NEEDS_ACQUISITION` | V2.5a (Render Comparison Engine) | Requires dual extraction and structured main-text presence detection on both raw server response and post-render DOM. |
| `AR-RENDER-006` | Crawlable internal-link set changes after render | deterministic | `url`, `internal_links_raw`, `internal_links_rendered` | Post-render or pre-render link set | Concurrent dual-capture of raw HTML crawlable internal links AND rendered DOM link set | None | None | `internal/sitecrawl/render.go`; `internal/audit/adapter/adapter.go:906` | `NEEDS_ACQUISITION` | V2.5a (Render Comparison Engine) | Requires diffing links found in initial server HTML vs links injected by client-side hydration. Standalone crawler replaces link set rather than retaining both. |
| `AR-AI-001` | OAI-SearchBot robots access matches project policy | assisted | `url`, `robots_allowed_oai_searchbot`, `oai_searchbot_access_policy`, `matched_robots_rule` | Generic robots directives | OAI-SearchBot-specific robots.txt group evaluation; matched directive line | `oai_searchbot_access_policy` (Site scope: `allow`, `block`) | None | `internal/sitecrawl/robots.go`; `internal/audit/policy.go:55` | `NEEDS_ACQUISITION` | V2.1a (Robots Multi-Agent) | SiteCrawl does not evaluate robots.txt against OpenAI search bot tokens (`OAI-SearchBot`). Secondary policy dependency. |
| `AR-AI-002` | OAI-SearchBot profile encounters access anomaly | assisted | `url`, `default_profile_status`, `oai_profile_status`, `default_challenge_detected`, `oai_challenge_detected` | Single-profile fetch observations only | Dual-profile comparative HTTP fetch (default vs OAI-SearchBot UA); challenge header detection | None | Comparative anomaly threshold | `internal/sitecrawl/fetch.go`; `internal/audit/model.go:30` | `NEEDS_ACQUISITION` | V2.2a (Multi-Profile Transport) | Standalone crawler only fetches once per URL using configured UA. Comparative differential fetch with OAI-SearchBot profile is not performed. |
| `AR-AI-003` | GPTBot configuration matches explicit training policy | assisted | `robots_allowed_gptbot`, `gptbot_training_policy`, `robots_allowed_oai_searchbot` | Generic robots directives | GPTBot-specific robots.txt group evaluation; training vs search rule precedence verification | `gptbot_training_policy` (Site scope: `allow`, `block`) | Precedence resolution contract between training (GPTBot) and search (OAI-SearchBot) | `internal/sitecrawl/robots.go`; `internal/audit/policy.go:56` | `NEEDS_ACQUISITION` | V2.1a (Robots Multi-Agent) | Requires evaluating robots.txt specifically for `GPTBot` user-agent and verifying compliance with explicit project training policy. Secondary policy dependency. |

---

## 5. Acquisition / SQLite / Adapter Gaps

Detailed audit of the codebase reveals specific, actionable technical gaps across the three foundational layers.

### 5.1 Acquisition Gaps (`internal/sitecrawl/`)

1. **Active Diagnostic Probes Engine (Missing):**
   - *Affected Rules:* `AR-ACC-001` (HTTPS endpoint), `AR-CANON-001` (Origin variants), `AR-CANON-002` (Trailing slash pair), `AR-INDEX-004` (Known-missing 404 probe), `AR-INDEX-006` (Soft 404 template matching).
   - *Mechanism:* SiteCrawl currently operates purely on frontier graph traversal (seed URL -> links -> sitemaps). It does not execute synthetic, non-graph diagnostic probes.
   - *Technical Requirement:* A dedicated diagnostic probing subsystem that generates collision-resistant non-existent paths, executes the 4 canonical origin combinations (`http://`, `https://`, `http://www.`, `https://www.`), and fetches slash/non-slash pairs without polluting the crawl discovery universe.
2. **Robots.txt Multi-Agent AST Parsing Engine (Missing):**
   - *Affected Rules:* `AR-ACC-002`, `AR-ACC-005` (Googlebot), `AR-AI-001` (OAI-SearchBot), `AR-AI-003` (GPTBot).
   - *Mechanism:* `robotsCache` (`internal/sitecrawl/robots.go:40`) uses `temoto/robotstxt` to evaluate only the user-agent configured for the active crawl. It does not parse or evaluate separate groups for Googlebot, OAI-SearchBot, or GPTBot, nor does it persist the matched line/rule pattern to SQLite.
   - *Technical Requirement:* Persist parsed robots.txt AST and evaluate target URLs against all supported V1 agent tokens (`*`, `googlebot`, `oai-searchbot`, `gptbot`), recording allowed/disallowed state and matched rule strings.
3. **Dual Raw / Rendered HTML DOM Retention (Missing):**
   - *Affected Rules:* `AR-RENDER-001` through `AR-RENDER-006` (Title, Canonical, Directives, H1, Main Text, Links).
   - *Mechanism:* When rendering is enabled (`internal/sitecrawl/render.go`), the headless browser DOM overwrites the server-delivered HTML (`internal/sitecrawl/page.go:54`). The raw HTML is discarded, triggering `GapRenderedRawSourceUnavailable` in the adapter (`internal/audit/adapter/adapter.go:906`).
   - *Technical Requirement:* When rendering is active, retain both the raw HTTP response body and the post-hydration browser DOM in SQLite as distinct artifacts (`RawArtifact`), extracting separate raw and rendered property sets.
4. **Comparative Multi-Profile Transport Probing (Missing):**
   - *Affected Rules:* `AR-ACC-006` (Googlebot anomaly), `AR-ACC-008` (Bot-profile anomaly), `AR-AI-002` (OAI-SearchBot anomaly).
   - *Mechanism:* SiteCrawl issues exactly one HTTP request per URL using its configured user-agent. `Page.BotBlocked` (`internal/sitecrawl/types.go:415`) is merely an opportunistic browser fallback flag, not a controlled comparative probe.
   - *Technical Requirement:* A secondary request pipeline that re-fetches sampled URLs with designated bot profile headers to compare status codes, header discrepancies, and WAF challenge markers.
5. **Non-Anchor Navigation & Sequence Discovery (Missing):**
   - *Affected Rules:* `AR-LINK-001` (Non-href navigation), `AR-CANON-011` (Pagination sequences).
   - *Mechanism:* `extract.go` extracts standard `<a href>` and `<link href>` tags. It does not inspect interactive elements (`<button>`, `<div onclick>`) or detect paginated series markup beyond deprecated `<link rel=next>` headers.

### 5.2 SQLite Schema Gaps

1. **Terminal Redirect Status on Source Rows:**
   - *Table:* `sitecrawl_pages`
   - *Missing Column:* `final_status INTEGER NOT NULL DEFAULT 0`
   - *Impact on `AR-CANON-010` and `AR-ACC-003`:* In `internal/sitecrawl/fetch.go:190`, `out.FinalStatus` captures the terminal status (e.g. 200, 404, 500) at the end of a redirect chain. However, `sitecrawl_pages` only stores `status` (the initial 3xx code). Unless the target was separately crawled as an internal page, the terminal status is lost.
2. **Granular Robots Directive Line and Rule Pattern:**
   - *Table:* `sitecrawl_pages`
   - *Missing Columns:* `robots_rule TEXT NOT NULL DEFAULT ''`, `robots_line INTEGER NOT NULL DEFAULT 0`
   - *Impact on `AR-ACC-002`:* `robots_state` (allowed/blocked/unknown) is stored, but the matching directive line (e.g. `Disallow: /checkout`) is not persisted.
3. **Raw vs Rendered Artifact References:**
   - *Table:* `sitecrawl_pages`
   - *Missing Columns:* `raw_html_artifact_ref TEXT`, `rendered_dom_artifact_ref TEXT`
   - *Impact on Rendering Suite:* Essential for dual-artifact persistence without bloating the SQLite row data column.

### 5.3 Adapter Normalization Gaps (`internal/audit/adapter/adapter.go`)

1. **Orphan Discovery & Inlink Count (`AR-LINK-005`):**
   - `sitecrawl_pages.inlinks` contains the raw internal inlink count.
   - `sitecrawl_sitemap_entries` contains URLs discovered via sitemaps.
   - *Gap:* The adapter does not emit `crawl_inlink_count` or `discovery_sources` normalized observations onto `SubjectURL`.
2. **Terminal Redirect Correlation (`AR-CANON-010`):**
   - The adapter emits `redirect_hop` and `redirect_final_url`.
   - *Gap:* It does not emit `redirect_final_status` or correlate the destination URL to its corresponding `SubjectURL` to extract `http_status`.
3. **Snippet Directives Extraction (`AR-INDEX-005`):**
   - The adapter parses generic tokens in `processPageDirectives`.
   - *Gap:* It does not extract parameterized directives (`max-snippet`, `data-nosnippet`) into normalized observations on `SubjectURL`.
4. **Default Robots Allowed State (`AR-ACC-002`):**
   - `pr.robotsState` is available in `sitecrawl_pages`.
   - *Gap:* The adapter does not normalize `robots_allowed_default` on `SubjectURL`.

---

## 6. Policy and Contract Dependencies

### 6.1 Explicit Project Policy Dependencies

All project policies must conform to the foundational guardrail: **PROJECT POLICY != OBSERVED EVIDENCE**. The domain model (`internal/audit/policy.go`) has already implemented strict schema validation for all 10 V1 policy keys. The table below identifies which remaining rules depend on explicit policies:

| Policy Key | Scope | Controlled Vocabulary | Dependent Rule | Policy Status in Code | Required Wiring |
|---|---|---|---|---|---|
| `preferred_origin` | SITE | Absolute URI string (`https://example.com`) | `AR-CANON-001` | Implemented in `policy.go:51` | Pass via `PolicyIndex` to active probe evaluator |
| `sitemap_expected` | SITE | `true`, `false`, `unspecified` | *(Implemented in `AR-DISC-001`)* | Implemented in `policy.go:52` | Fully wired and tested |
| `snippet_policy` | SITE | `allow_unrestricted`, `restrict`, `unspecified` | `AR-INDEX-005` | Implemented in `policy.go:53` | Pass via `PolicyIndex` to snippet evaluator |
| `googlebot_access_policy` | SITE | `allow`, `block`, `unspecified` | `AR-ACC-005` | Implemented in `policy.go:54` | Pass via `PolicyIndex` to Googlebot robots evaluator |
| `oai_searchbot_access_policy` | SITE | `allow`, `block`, `unspecified` | `AR-AI-001` | Implemented in `policy.go:55` | Pass via `PolicyIndex` to OAI-SearchBot evaluator |
| `gptbot_training_policy` | SITE | `allow`, `block`, `unspecified` | `AR-AI-003` | Implemented in `policy.go:56` | Pass via `PolicyIndex` to GPTBot evaluator |
| `expected_url_state` | URL | `live`, `redirect`, `missing`, `unspecified` | `AR-ACC-003` | Implemented in `policy.go:59` | Pass URL policy assignment to `AR-ACC-003` evaluator |
| `expected_crawlable` | URL | `true`, `false`, `unspecified` | `AR-ACC-002` | Implemented in `policy.go:60` | Pass URL policy assignment to `AR-ACC-002` evaluator |
| `expected_indexable` | URL | `true`, `false`, `unspecified` | *(Implemented in `AR-INDEX-001`)* | Implemented in `policy.go:61` | Fully wired and tested |
| `priority_page` | URL | `true`, `false` | `AR-LINK-006` | Implemented in `policy.go:62` | Pass URL policy assignment to `AR-LINK-006` evaluator |

### 6.2 Open Contract Decisions

1. **`AR-INDEX-003` Supported Directive Token Vocabulary:**
   - *Decision Needed:* Authoritative list of recognized robots directive tokens for V1.
   - *Recommended Contract:* `index`, `noindex`, `follow`, `nofollow`, `none`, `all`, `noarchive`, `nosnippet`, `notranslate`, `noimageindex`, `nocache`, and parameterized prefixes (`max-snippet`, `max-image-preview`, `max-video-preview`, `unavailable_after`). Any other token triggers `WARNING`.
2. **`AR-CANON-010` Offsite Destination Terminal Status Contract:**
   - *Decision Needed:* If a redirect leads to an external URL (not crawled), should the rule return `UNKNOWN` or should the crawler be permitted to capture the terminal HTTP status without crawling further?
   - *Recommended Contract:* Crawler should persist `FinalStatus` from the initial transport walk into `sitecrawl_pages.final_status`. If present, evaluate directly; if absent and destination is uncrawled, evaluate as `UNKNOWN`.
3. **`AR-LINK-006` Manual Review Task Contract:**
   - *Decision Needed:* Data contract for emitting `ManualReviewTask` records and how reviewer verdicts are recorded.

---

## 7. Cross-Rule Dependency Map

The 47 rules partition into cohesive functional clusters that share normalized evidence models and derivation paths.

```mermaid
graph TD
    subgraph Canonical_Cluster ["Canonical Integrity Cluster (7/11 Complete)"]
        CANON003["AR-CANON-003 (Missing) [EXECUTABLE]"]
        CANON004["AR-CANON-004 (Multiple) [EXECUTABLE]"]
        CANON005["AR-CANON-005 (Syntax) [READY]"]
        CANON006["AR-CANON-006 (200 Target) [EXECUTABLE]"]
        CANON007["AR-CANON-007 (Not Noindex) [EXECUTABLE]"]
        CANON005 -.->|Shares canonical_count & normalization| CANON004
        CANON006 -.->|Requires valid target from| CANON005
    end

    subgraph Redirect_Cluster ["Redirect Cluster (2/3 Complete)"]
        CANON008["AR-CANON-008 (Hop Count) [EXECUTABLE]"]
        CANON009["AR-CANON-009 (Loop) [EXECUTABLE]"]
        CANON010["AR-CANON-010 (Available) [NEEDS ADAPTER]"]
        CANON010 -.->|Reuses redirectEvidenceBundle| CANON008
    end

    subgraph Directives_Cluster ["Robots Directives Cluster (2/3 Complete)"]
        INDEX001["AR-INDEX-001 (Intent) [EXECUTABLE]"]
        INDEX002["AR-INDEX-002 (Conflict) [EXECUTABLE]"]
        INDEX003["AR-INDEX-003 (Syntax) [NEEDS CONTRACT]"]
        INDEX003 -.->|Reuses rawDirectiveBundle| INDEX002
    end

    subgraph Link_Cluster ["Internal Links & Graph (3/6 Complete)"]
        LINK002["AR-LINK-002 (3xx Target) [EXECUTABLE]"]
        LINK003["AR-LINK-003 (4xx Target) [EXECUTABLE]"]
        LINK004["AR-LINK-004 (5xx Target) [EXECUTABLE]"]
        LINK005["AR-LINK-005 (Orphan) [NEEDS ADAPTER]"]
        LINK006["AR-LINK-006 (Depth) [MANUAL]"]
        LINK005 -.->|Reuses sitecrawl_links & pages| LINK002
    end

    subgraph Probing_Cluster ["Active Diagnostic Probes (0/5 Complete)"]
        ACC001["AR-ACC-001 (HTTPS Reachable)"]
        CANON001["AR-CANON-001 (Origin Normalize)"]
        CANON002["AR-CANON-002 (Trailing Slash)"]
        INDEX004["AR-INDEX-004 (404 Probe)"]
        INDEX006["AR-INDEX-006 (Soft 404)"]
        INDEX006 -.->|Depends on 404 template from| INDEX004
    end

    subgraph MultiAgent_Cluster ["Robots Multi-Agent AST (0/4 Complete)"]
        ACC002["AR-ACC-002 (Crawl Intent)"]
        ACC005["AR-ACC-005 (Googlebot Robots)"]
        AI001["AR-AI-001 (OAI-SearchBot)"]
        AI003["AR-AI-003 (GPTBot Training)"]
    end

    subgraph Render_Cluster ["Dual Raw/Render DOM Diff (0/6 Complete)"]
        RENDER001["AR-RENDER-001 (Title)"]
        RENDER002["AR-RENDER-002 (Canonical)"]
        RENDER003["AR-RENDER-003 (Robots)"]
        RENDER004["AR-RENDER-004 (H1)"]
        RENDER005["AR-RENDER-005 (Text)"]
        RENDER006["AR-RENDER-006 (Links)"]
    end
```

### Key Reuse Insights

1. **`AR-CANON-005` Reuses Existing Canonical Infrastructure:** `AR-CANON-005` operates entirely on observations already produced for `AR-CANON-004` and `AR-CANON-006`. Zero new observation fields are required.
2. **`AR-CANON-010` Reuses `redirectEvidenceBundle`:** The engine already has `internal/audit/engine/redirect_helpers.go` parsing hops, loops, and traversal completeness. Once terminal status is wired, `AR-CANON-010` seamlessly plugs in.
3. **`AR-INDEX-003` Reuses `rawDirectiveBundle`:** `internal/audit/engine/index002.go` established the directive token aggregation model. `AR-INDEX-003` inspects those same tokens against the token registry.
4. **`AR-LINK-005` Cross-Correlates Sitemap and Crawl Graph:** `AR-LINK-005` leverages discovery records from the fully implemented Sitemap group (`AR-DISC-001` through `006`) to identify orphan pages with zero internal inlinks.

---

## 8. Prioritized Implementation Roadmap

Following the project principles (governed progression, small independently testable milestones, zero breaking changes to frozen contracts), the remaining 28 rules are organized into sequential, phased implementation batches.

### Phase 1: Pure Engine Evaluator Implementation (No Acquisition or Adapter Changes)

#### Milestone V1.9b: Target `AR-CANON-005` (Canonical URL is syntactically resolvable)
- **Target Rule:** `AR-CANON-005`
- **Current Readiness:** `READY_TO_IMPLEMENT`
- **Expected Evidence Changes:** None. Uses existing `url_identity`, `canonical_count`, `canonical_resolved`, `canonical_normalization_complete`, `canonical_normalized_target`.
- **Expected Evaluator Changes:** Implement `evaluateCANON005` in `internal/audit/engine/canon005.go` and register in `engine.go`.
- **Required Tests:** Unit tests covering PASS (valid absolute/relative URL), FAIL (invalid syntax, unsupported scheme, invalid host), NOT_APPLICABLE (count 0 or >1), UNKNOWN (missing identity, normalization incomplete, rendered source unavailable).
- **Frozen Boundaries:** Zero adapter diff; zero crawler diff; `NormalizationVersion` remains `v1.9.0`.
- **Expected Executable Rule Count:** **`20 / 47`** (42.5%).
- **External Approval Needed:** Standard post-implementation PR review.

---

### Phase 2: Adapter Wiring & Contract Decision Milestones

#### Milestone V1.10a: V1 Robots Directive Token Registry Contract Decision
- **Objective:** Approve and lock the authoritative V1 supported directive token vocabulary in `knowledge/`.
- **Scope:** Documentation/contract review only. No code.

#### Milestone V1.10b: Implement `AR-INDEX-003` (Malformed or unsupported robots directive)
- **Target Rule:** `AR-INDEX-003`
- **Current Readiness:** `NEEDS_ADAPTER_WIRING` / `NEEDS_CONTRACT_DECISION`
- **Expected Changes:** Implement `evaluateINDEX003` in `internal/audit/engine/index003.go` validating `directive_tokens` against the approved vocabulary.
- **Expected Executable Rule Count:** **`21 / 47`** (44.7%).

#### Milestone V1.11a: Redirect Terminal Status Adapter Wiring
- **Objective:** Correlate terminal redirect status (`redirect_final_status`) in `internal/audit/adapter/` when target is present in crawl.
- **Expected Changes:** Adapter emits `redirect_final_status` on `SubjectURL`. Bump `NormalizationVersion` to `v1.10.0`.

#### Milestone V1.11b: Implement `AR-CANON-010` (Redirect final target is available)
- **Target Rule:** `AR-CANON-010`
- **Expected Evaluator Changes:** Implement `evaluateCANON010` in `internal/audit/engine/canon010.go`.
- **Expected Executable Rule Count:** **`22 / 47`** (46.8%).

#### Milestone V1.12a: Inlink & Discovery Sources Adapter Wiring
- **Objective:** Expose `crawl_inlink_count` from `sitecrawl_pages.inlinks` and supplemental source flags onto `SubjectURL`.
- **Expected Changes:** Adapter normalization enhancement.

#### Milestone V1.12b: Implement `AR-LINK-005` (Orphan candidate relative to supplied discovery sources)
- **Target Rule:** `AR-LINK-005`
- **Expected Evaluator Changes:** Implement `evaluateLINK005` in `internal/audit/engine/link005.go`.
- **Expected Executable Rule Count:** **`23 / 47`** (48.9%).

---

### Phase 3: Policy-Wiring & Assisted / Manual Milestones

#### Milestone V1.13: Wire `expected_url_state` & Implement `AR-ACC-003`
- **Target Rule:** `AR-ACC-003` (HTTP status matches explicit expected URL state)
- **Expected Changes:** Wire `expected_url_state` policy lookup in evaluator; implement `evaluateACC003`.
- **Expected Executable Rule Count:** **`24 / 47`** (51.1%).

#### Milestone V1.14: Manual Review Task Engine & `AR-LINK-006`
- **Target Rule:** `AR-LINK-006` (Priority-page crawl depth review)
- **Expected Changes:** Wire `priority_page` policy; implement manual review task generator; implement `evaluateLINK006`.
- **Expected Executable Rule Count:** **`25 / 47`** (53.2%).

#### Milestone V1.15: Crawler Intent & `AR-ACC-002`
- **Target Rule:** `AR-ACC-002` (Robots access matches explicit crawl intent)
- **Expected Changes:** Expose `robots_allowed_default` in adapter; wire `expected_crawlable` policy; implement `evaluateACC002`.
- **Expected Executable Rule Count:** **`26 / 47`** (55.3%).

#### Milestone V1.16: Snippet Policy & Directives (`AR-INDEX-005`)
- **Target Rule:** `AR-INDEX-005` (Snippet restriction conflicts with snippet policy)
- **Expected Changes:** Extract snippet directive tokens and HTML `data-nosnippet`; wire `snippet_policy`; implement `evaluateINDEX005`.
- **Expected Executable Rule Count:** **`27 / 47`** (57.4%).

---

### Phase 4: Advanced Acquisition Milestones (Major Architecture Enhancements)

#### Milestone V2.0: Active Diagnostic Probes Engine
- **Target Rules (5):** `AR-ACC-001`, `AR-CANON-001`, `AR-CANON-002`, `AR-INDEX-004`, `AR-INDEX-006`.
- **Scope:** Build synthetic diagnostic probe runner in `internal/sitecrawl/probe/`.
- **Expected Executable Rule Count:** **`32 / 47`** (68.1%).

#### Milestone V2.1: Multi-Agent Robots.txt AST Engine
- **Target Rules (3):** `AR-ACC-005`, `AR-AI-001`, `AR-AI-003`.
- **Scope:** Multi-agent robots parsing and persistence in SQLite.
- **Expected Executable Rule Count:** **`35 / 47`** (74.5%).

#### Milestone V2.2: Multi-Profile Transport Probing
- **Target Rules (3):** `AR-ACC-006`, `AR-ACC-008`, `AR-AI-002`.
- **Scope:** Comparative bot-profile HTTP fetcher.
- **Expected Executable Rule Count:** **`38 / 47`** (80.9%).

#### Milestone V2.3 to V2.4: Advanced Discovery (Pagination & JS Navigation)
- **Target Rules (2):** `AR-CANON-011`, `AR-LINK-001`.
- **Expected Executable Rule Count:** **`40 / 47`** (85.1%).

#### Milestone V2.5: Headless Raw/Render Dual Retention Engine
- **Target Rules (6):** `AR-RENDER-001` through `AR-RENDER-006`.
- **Scope:** Retain both raw HTTP response and rendered DOM artifacts in SQLite.
- **Expected Executable Rule Count:** **`46 / 47`** (97.9%).

#### Milestone V2.6: Sub-Resource Render Dependency Tracking (`AR-ACC-007`)
- **Target Rule (1):** `AR-ACC-007`.
- **Expected Executable Rule Count:** **`47 / 47`** (100.0%).

---

## 9. Recommended Next Milestone

### Primary Recommendation: Audit V1.9b — Implement `AR-CANON-005`

**Rule ID:** `AR-CANON-005`  
**Name:** `Canonical URL is syntactically resolvable`  
**Parent Check:** `CANON-003`  
**Category:** `canonical`  
**Lifecycle:** `Consolidate`  
**Automation:** `deterministic`  
**Default Severity:** `P1`

#### Rigorous Justification

1. **Evidence Completeness (100% Ready):**
   - In `internal/audit/adapter/adapter.go:1087-1225`, the V1.8c adapter already executes `normalizeCanonicalTarget(c, pr.url)` for every preserved canonical declaration.
   - When exactly one canonical declaration exists (`canonical_count == 1`):
     - If it parses and resolves to a valid absolute HTTP(S) URL: `canonical_normalization_complete` is `"true"` and `canonical_normalized_target` contains the normalized URL.
     - If it fails syntactic parsing (e.g. empty, invalid characters, unsupported scheme like `javascript:`, malformed host): `canonical_normalization_complete` is `"false"`.
   - All required observations are already indexed on `SubjectURL`: `url_identity`, `canonical_count`, `canonical_resolved`, `canonical_normalization_complete`, `canonical_normalized_target`.
2. **Contract Clarity:**
   - Manifest contract (`knowledge/07-v1-atomic-rule-manifest.md:346-360`):
     - `Preconditions`: Exactly one canonical declaration exists (`canonical_count == 1`).
     - `PASS`: Value resolves to valid HTTP(S) URL (`canonical_normalization_complete == "true"`).
     - `FAIL`: Value cannot be resolved as a valid canonical URL (`canonical_normalization_complete == "false"`).
     - `NOT_APPLICABLE`: No single canonical declaration (`canonical_count != 1`, i.e., 0 or >1).
     - `UNKNOWN`: Missing/conflicting URL identity, missing/malformed `canonical_count`, or raw HTML canonical evidence unavailable (e.g. rendered page where raw HTML was replaced).
3. **Engineering Complexity: Minimal:**
   - Evaluator follows the exact typed evaluator pattern established in `internal/audit/engine/canon004.go` and `internal/audit/engine/canon006.go`.
   - Requires zero crawler changes, zero SQLite changes, zero adapter changes, and zero policy wiring.
   - `NormalizationVersion` remains `v1.9.0`.
4. **Risk of False Positives: Near Zero:**
   - Relies on standard Go `net/url` parsing and frozen RFC-compliant URL normalization locked in V1.3c and V1.4b.
5. **Impact on Audit System:**
   - Completes the canonical declaration triage sequence:
     - `AR-CANON-003`: Is a canonical present?
     - `AR-CANON-004`: Is there only one declaration?
     - `AR-CANON-005`: Is that single declaration syntactically valid?
     - `AR-CANON-006`: Does the target return 200?
     - `AR-CANON-007`: Is the target indexable?
   - Advances executable rule count from **19** to **20/47** (42.5%).

#### Alternative Candidates Evaluated

1. **Alternative 1: `AR-CANON-010` (Redirect final target is available)**
   - *Why rejected for V1.9b:* In `internal/sitecrawl/fetch.go:190`, `out.FinalStatus` was captured during the initial HTTP fetch walk, but it was not persisted into `sitecrawl_pages`. The adapter does not emit `redirect_final_status`. If a redirect leads offsite or to an uncrawled URL, its terminal status is unavailable in `EvidenceSnapshot`. Implementing `AR-CANON-010` requires adapter wiring and contract resolution first.
2. **Alternative 2: `AR-INDEX-003` (Malformed or unsupported robots directive)**
   - *Why rejected for V1.9b:* While `robots_directive_tokens` is emitted on `SubjectURL`, the exact supported V1 directive token vocabulary has not been locked in an authoritative contract decision. Implementing without locking the token list risks false warnings on valid directives (e.g. `max-image-preview:large` or `unavailable_after`).
3. **Alternative 3: `AR-LINK-005` (Orphan candidate relative to supplied discovery sources)**
   - *Why rejected for V1.9b:* SQLite stores inlink counts in `sitecrawl_pages.inlinks`, but the adapter does not emit `crawl_inlink_count` on `SubjectURL`. Requires adapter wiring and a `NormalizationVersion` bump before engine implementation.

---

## 10. Risks and Open Decisions

| Risk / Decision | Severity | Architectural Impact | Mitigation Strategy |
|---|---|---|---|
| **Confusing `final_status` from separate crawl with transport terminal status** | Medium | Incorrect redirect verdicts for offsite or uncrawled destinations | Enforce that redirect terminal status must originate from the authoritative transport hop walk (`sitecrawl_pages.final_status` or explicit terminal hop) rather than guessing from independent URL records. |
| **Robots directive token set divergence** | Medium | False warnings in `AR-INDEX-003` | Execute Milestone V1.10a contract decision to freeze the V1 directive token list before coding `AR-INDEX-003`. |
| **Conflating Policy with Evidence** | High | Diagnostic corruption; violation of core principles | Keep all `PolicyIndex` lookups strictly isolated from `EvidenceSnapshot`. Prohibit crawler or LLM policy inference. |
| **Render Dual-Retention DB Bloat** | Medium | Large SQLite database size when caching full HTML and DOM | Use compressed content-addressed blob storage (`RawArtifact`) with hash deduplication rather than storing redundant full text in `sitecrawl_pages.data`. |
| **Premature Active Probing in Crawl Loop** | High | Crawl frontier pollution; infinite loops | Ensure diagnostic probe requests run in an isolated probe runner, generating `ProbeObservation` records that never create `DiscoveryRecord`s. |

---

## 11. Verification and Source References

### 11.1 Verification Commands Executed

```powershell
# 1. Clean working tree and baseline HEAD check
git status
git rev-parse HEAD

# 2. Whitespace check
git diff --check

# 3. Test suite execution
go test -count=1 ./...

# 4. Binary build verification
go build ./cmd/sitecrawl-dev

# 5. Static code analysis
go vet ./...

# 6. AI Dev System gate runner
powershell -NoProfile -File .ai-dev-system/gates/run.ps1 -ProjectRoot .
```

*Verification Results:*
- Tests: `go test -count=1 ./...` PASSED across all packages (`internal/audit`, `internal/audit/adapter`, `internal/audit/engine`, `internal/platform/standalone`, `internal/sitecrawl`).
- Build: `go build ./cmd/sitecrawl-dev` succeeded with 0 errors.
- Vet: `go vet ./...` completed clean with 0 warnings.
- Gates: `git.diff-check` PASSED. `docs.local-links` reported pre-existing failure on `.ai-dev-system/docs/INDEX.md` relative links to unimplemented template directories.

### 11.2 Authoritative Document References

- `knowledge/07-v1-atomic-rule-manifest.md` — Authoritative rule contracts.
- `knowledge/09-data-model.md` — Frozen domain entity definitions and policy vocabulary.
- `internal/audit/rules_v1.json` — Machine-readable 47-rule registry.
- `internal/audit/engine/engine.go` — Registration of the 19 implemented evaluators.
- `internal/audit/adapter/adapter.go` — V1.8c normalization logic and evidence catalog.
- `docs/audit-v1.6a-rule-readiness.md` — Historical Phase 4 rule readiness assessment.
- `docs/audit-v1.8c-sitemap-rule-readiness.md` — V1.8c sitemap readiness review.

---

## 12. External Approval Checklist

Before proceeding to implementation in Milestone V1.9b, external review should confirm:

- [ ] Exact rule reconciliation confirmed: 47 frozen = 19 implemented + 28 remaining.
- [ ] Readiness classifications and primary blockers for all 28 remaining rules accepted.
- [ ] Classification of `AR-CANON-005` as the sole immediately implementable (`READY_TO_IMPLEMENT`) rule confirmed.
- [ ] Recommended next target: Audit V1.9b to implement `AR-CANON-005 — Canonical URL is syntactically resolvable`.
- [ ] Proposed phased roadmap (Phase 1 pure evaluators, Phase 2 adapter wiring, Phase 3 policy wiring, Phase 4 advanced acquisition) accepted as the strategic implementation plan.
