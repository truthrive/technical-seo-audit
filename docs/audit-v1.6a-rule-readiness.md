# Audit V1.6a — Phase 4 Rule Readiness & Next Batch Selection

## 1. Executive Summary

- **Audit Milestone**: Audit V1.6a (Analysis & Planning)
- **Baseline Git HEAD**: `a3da82e94eba4404202c335fc907106ad88fc19a`
- **Knowledge Revision**: `v1.4.4`
- **NormalizationVersion**: `v1.6.0`
- **Current Executable Rules**: Exactly `9 / 47` (`AR-ACC-004`, `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, `AR-CANON-007`, `AR-CANON-008`, `AR-CANON-009`, `AR-INDEX-001`, `AR-INDEX-002`)
- **Unimplemented Rules Assessed**: `38 / 47`
- **Currently READY_TO_IMPLEMENT Rules**: `0 / 47` (no unimplemented rule has complete normalized evidence and correlation boundaries in the existing frozen snapshot)
- **Phase 4 Status**: **IN PROGRESS**
- **Recommended Next Batch**: **Audit V1.6b — Link Evidence Enablement & Target Correlation Hardening** (enabling normalized evidence for `AR-LINK-002`, `AR-LINK-003`, and `AR-LINK-004` without implementing premature evaluators, mirroring the proven V1.4b canonical and V1.5a redirect enablement milestones).

---

## 2. Readiness Classification Summary

| Classification | Count | Description |
|---|---:|---|
| **EXECUTABLE** | 9 | Fully implemented, registered, verified by hermetic and unit tests. |
| **READY_TO_IMPLEMENT** | 0 | All required evidence and correlation boundaries exist; frozen semantics evaluable without adapter/acquisition changes. |
| **NEEDS_ADAPTER** | 4 | Crawler persists sufficient raw evidence, but Audit Evidence Adapter normalized observations or subject correlation boundaries are missing (`AR-LINK-002`, `AR-LINK-003`, `AR-LINK-004`, `AR-ENTITY-001`). |
| **NEEDS_ACQUISITION** | 28 | Required technical evidence is not acquired or persisted by frozen SiteCrawl (diagnostic probes, TLS verification, raw DOM retention, sitemap documents, multi-bot comparisons). |
| **NEEDS_POLICY** | 1 | (Primary hurdle is explicit project/user policy; several acquisition rules also have secondary policy dependencies). |
| **MANUAL_OR_ASSISTED** | 1 | Requires guided human review and manual review task infrastructure by frozen contract (`AR-LINK-006`). |
| **BLOCKED** | 4 | Missing authoritative evidence blocked by frozen architectural boundaries (`AR-ACC-003`, `AR-CANON-005`, `AR-CANON-010`, `AR-INDEX-003`). |
| **Total** | **47** | Complete accounting of all registered atomic rules in `internal/audit/rules_v1.json`. |

---

## 3. Comprehensive 47-Rule Readiness Matrix

| Rule ID | Rule name | Automation | Current readiness | Required evidence | Available evidence | Missing dependency | Source path / field | Recommended next action |
|---|---|---|---|---|---|---|---|---|
| `AR-ACC-001` | HTTPS endpoint reachable | deterministic | NEEDS_ACQUISITION | `site_https_url`, `fetch_attempted`, `tls_valid`, `fetch_error_type`, `fetch_status` | Partial (`fetch_status`, `fetch_error_type` on crawled pages) | Dedicated HTTPS origin probe; TLS certificate validity verification | `internal/audit/adapter/adapter.go:1296` (`GapTLSValidityUnavailable`) | Design standalone origin probe suite in future acquisition round |
| `AR-ACC-002` | Robots access matches explicit crawl intent | assisted | NEEDS_ACQUISITION | `url`, `robots_allowed_default`, `matched_robots_rule`, `expected_crawlable` | Partial (`robots_state` on `sitecrawl_pages`) | Explicit `expected_crawlable` policy; granular matched robots pattern/line telemetry | `internal/audit/adapter/adapter.go:1332` (`GapRobotsDecisionUnavailable`) | Persist granular robots.txt AST matched rules before implementing |
| `AR-ACC-003` | HTTP status matches explicit expected URL state | deterministic | BLOCKED | `url`, `expected_url_state`, `fetch_status`, `final_status` | Partial (`url`, `fetch_status`) | Authoritative source-chain `final_status` not persisted by frozen SiteCrawl; explicit policy | `internal/sitecrawl/page.go:23-24`; `knowledge/07-v1-atomic-rule-manifest.md:87` | Preserve frozen boundary; keep BLOCKED |
| `AR-ACC-004` | Unexpected server-error response | deterministic | EXECUTABLE | `url`, `fetch_status` | Present (`url_identity`, `http_status`) | None | `internal/audit/engine/acc004.go` | Maintain existing regression suite |
| `AR-ACC-005` | Googlebot robots access matches explicit policy | assisted | NEEDS_ACQUISITION | `url`, `robots_allowed_googlebot`, `googlebot_access_policy`, `matched_robots_rule` | Missing | Crawler only evaluates default user-agent; no Googlebot AST parse; explicit policy required | `internal/sitecrawl/crawler.go`; `internal/audit/adapter/adapter.go:1332` | Design multi-agent robots evaluation pipeline |
| `AR-ACC-006` | Googlebot profile response anomaly | assisted | NEEDS_ACQUISITION | `url`, `default_profile_status`, `googlebot_profile_status`, `default_challenge_detected`, `googlebot_challenge_detected` | Missing | Comparative multi-bot differential HTTP requests not executed by standalone crawler | `internal/audit/adapter/adapter.go:1362` (`GapProfileComparisonUnavailable`) | Defer to post-V1 advanced probe acquisition |
| `AR-ACC-007` | Blocked CSS/JS resource affects rendered output | assisted | NEEDS_ACQUISITION | `page_url`, `resource_url`, `resource_type`, `resource_robots_allowed`, `render_dependency_observed`, `render_diff_evidence` | Missing | Sub-resource dependency accessibility and render diff telemetry not tracked in SQLite | `internal/audit/adapter/adapter.go:1350` (`GapRenderComparisonUnavailable`) | Defer to rendering pipeline enhancement phase |
| `AR-ACC-008` | Bot-profile access restriction observed | assisted | NEEDS_ACQUISITION | `url`, `default_profile_status`, `bot_profile_status`, `default_challenge_detected`, `bot_challenge_detected`, `tested_user_agent` | Missing | Multi-profile comparative request telemetry not executed | `internal/audit/adapter/adapter.go:1362` (`GapProfileComparisonUnavailable`) | Defer to post-V1 advanced probe acquisition |
| `AR-INDEX-001` | Effective noindex conflicts with explicit indexability intent | assisted | EXECUTABLE | `url`, `effective_noindex`, `expected_indexable` | Present (`url_identity`, `effective_noindex`, `PolicyIndex.ExpectedIndexable`) | None | `internal/audit/engine/index001.go` | Maintain existing regression suite |
| `AR-INDEX-002` | Conflicting index directives | deterministic | EXECUTABLE | `url`, `robots_meta_tokens`, `x_robots_tokens` | Present (`url_identity`, `robots_meta_raw`, `x_robots_raw`, `robots_directive_tokens`) | None | `internal/audit/engine/index002.go` | Maintain existing regression suite |
| `AR-INDEX-003` | Malformed or unsupported robots directive | deterministic | BLOCKED | `url`, `robots_directive_tokens`, `robots_parse_errors` | Partial (`robots_directive_tokens` present) | No frozen authoritative V1 supported directive registry in `/knowledge` | `internal/audit/adapter/directives.go`; `knowledge/07-v1-atomic-rule-manifest.md:215` | Preserve frozen boundary; keep BLOCKED |
| `AR-INDEX-004` | Known-missing probe resolves to 404/410 | deterministic | NEEDS_ACQUISITION | `missing_probe_url`, `probe_final_status`, `probe_final_url` | Missing | Synthetic diagnostic missing-probe request generator does not exist in standalone SiteCrawl | `internal/audit/adapter/adapter.go:1356` (`GapProbeObservationUnavailable`) | Design diagnostic probe runner module |
| `AR-INDEX-005` | Snippet restriction conflicts with snippet policy | assisted | NEEDS_ACQUISITION | `url`, `nosnippet`, `max_snippet`, `data_nosnippet_count`, `snippet_policy` | Partial (`nosnippet` / `max_snippet` directive tokens) | HTML `data-nosnippet` element count not extracted by SiteCrawl; explicit policy required | `internal/sitecrawl/extract.go` | Plan extraction and policy support in future indexability slice |
| `AR-INDEX-006` | 200 response matches known 404 template | assisted | NEEDS_ACQUISITION | `url`, `fetch_status`, `content_fingerprint`, `known_404_content_fingerprint`, `known_404_probe_status` | Missing | Requires missing-probe acquisition and normalized DOM content fingerprinting | `internal/audit/adapter/adapter.go:1356` (`GapProbeObservationUnavailable`) | Defer to soft-404 acquisition milestone |
| `AR-CANON-001` | Origin variants normalize to preferred origin | deterministic | NEEDS_ACQUISITION | `preferred_origin`, `origin_variant`, `variant_fetch_status`, `variant_final_url`, `redirect_hops` | Missing | Origin variant probes (http/https, www/non-www) not executed; explicit `preferred_origin` required | `internal/audit/adapter/adapter.go:1356` (`GapProbeObservationUnavailable`) | Group with diagnostic probe acquisition round |
| `AR-CANON-002` | Trailing-slash pair is consistently normalized | deterministic | NEEDS_ACQUISITION | `slash_url`, `non_slash_url`, both response states, redirect paths, `normalized_content_hash` | Missing | Crawler normalizes frontier; does not dual-probe slash pairs or persist content hashes | `internal/sitecrawl/crawler.go`; `internal/audit/adapter/adapter.go:1356` | Group with diagnostic probe acquisition round |
| `AR-CANON-003` | Canonical declaration missing | assisted | EXECUTABLE | `url`, `fetch_status`, `content_type`, `canonical_values` | Present (`url_identity`, `http_status`, `content_type`, `canonical_count`) | None | `internal/audit/engine/canon003.go` | Maintain existing regression suite |
| `AR-CANON-004` | Multiple canonical declarations | deterministic | EXECUTABLE | `url`, `canonical_values`, `normalized_canonical_values` | Present (`canonical_count`, `canonical_normalized_target`, `canonical_distinct_normalized_count`, `canonical_normalization_complete`) | None | `internal/audit/engine/canon004.go` | Maintain existing regression suite |
| `AR-CANON-005` | Canonical URL is syntactically resolvable | deterministic | BLOCKED | `url`, `canonical_raw`, `canonical_resolved_url`, `canonical_parse_error` | Partial (resolved target preserved) | Original raw canonical declaration syntax and parse errors not preserved by frozen SiteCrawl | `internal/sitecrawl/page.go:75-76`; `internal/audit/adapter/adapter.go:1266` (`GapRawCanonicalUnavailable`) | Preserve frozen boundary; keep BLOCKED |
| `AR-CANON-006` | Canonical target returns final 200 | deterministic | EXECUTABLE | `url`, `canonical_resolved_url`, `canonical_target_status` | Present (`canonical_target_subject_ref`, correlated target `http_status`) | None | `internal/audit/engine/canon006.go` | Maintain existing regression suite |
| `AR-CANON-007` | Canonical target is not noindex | deterministic | EXECUTABLE | `canonical_resolved_url`, `canonical_target_status`, `canonical_target_effective_noindex` | Present (`canonical_target_subject_ref`, target `effective_noindex`, target `http_status`) | None | `internal/audit/engine/canon007.go` | Maintain existing regression suite |
| `AR-CANON-008` | Redirect chain exceeds one hop | deterministic | EXECUTABLE | `url`, `redirect_hops` | Present (`redirect_initial_observed`, `redirect_hop_count`, `redirect_hop`, `redirect_traversal_complete`, `redirect_final_url`) | None | `internal/audit/engine/canon008.go` | Maintain existing regression suite |
| `AR-CANON-009` | Redirect loop detected | deterministic | EXECUTABLE | `url`, `redirect_hops`, `redirect_loop_detected` | Present (`redirect_initial_observed`, `redirect_loop_detected`, `redirect_traversal_complete`, `redirect_hop`) | None | `internal/audit/engine/canon009.go` | Maintain existing regression suite |
| `AR-CANON-010` | Redirect final target is available | deterministic | BLOCKED | `url`, `redirect_hops`, `final_status`, `redirect_loop_detected` | Partial (`redirect_hops`, `redirect_loop_detected`, `redirect_final_url`) | Source-chain `final_status` not persisted by frozen SiteCrawl; independent target fetch cannot substitute | `internal/sitecrawl/page.go:23`; `internal/audit/adapter/redirect.go` | Preserve frozen boundary; keep BLOCKED |
| `AR-CANON-011` | Pagination exposes crawlable next-page link | assisted | NEEDS_ACQUISITION | `url`, `pagination_detected`, `has_next_page`, `next_page_url`, `next_page_element_tag`, `next_page_href` | Missing (crawler extracts deprecated `rel=next/prev`, not markup sequence links) | Pagination sequence identification and crawlable `<a href>` extraction | `internal/sitecrawl/page.go:79-80`; `knowledge/07-v1-atomic-rule-manifest.md:457` | Plan markup-based pagination acquisition in future discovery phase |
| `AR-LINK-001` | Internal navigation target lacks crawlable href | assisted | NEEDS_ACQUISITION | `source_url`, `navigation_candidate`, `resolved_internal_target`, `element_tag`, `href` | Missing | Non-href navigation candidate detection (JS click handlers, wrappers) not implemented | `internal/sitecrawl/extract.go` | Defer to post-V1 JS navigation inspection |
| `AR-LINK-002` | Internal link targets 3xx | deterministic | NEEDS_ADAPTER | `source_url`, `target_url`, `target_status`, `anchor_text`, `link_location` | Partial (SiteCrawl stores `sitecrawl_links` edges and `sitecrawl_pages` status; snapshot has `link_target`, `link_anchor`, `link_location`) | Adapter does not emit `source_url`, `link_is_internal`, or `link_target_subject_ref` correlation boundary | `internal/sitecrawl/runs.go:121`; `internal/audit/adapter/adapter.go:1187-1231` | Implement Link Evidence Enablement (`Audit V1.6b`) |
| `AR-LINK-003` | Internal link targets 4xx | deterministic | NEEDS_ADAPTER | `source_url`, `target_url`, `target_status`, `anchor_text`, `link_location` | Partial (same as AR-LINK-002) | Adapter does not emit `source_url`, `link_is_internal`, or `link_target_subject_ref` correlation boundary | `internal/sitecrawl/runs.go:121`; `internal/audit/adapter/adapter.go:1187-1231` | Implement in evaluator batch (`Audit V1.6c`) following V1.6b enablement |
| `AR-LINK-004` | Internal link targets 5xx | deterministic | NEEDS_ADAPTER | `source_url`, `target_url`, `target_status`, `anchor_text`, `link_location` | Partial (same as AR-LINK-002) | Adapter does not emit `source_url`, `link_is_internal`, or `link_target_subject_ref` correlation boundary | `internal/sitecrawl/runs.go:121`; `internal/audit/adapter/adapter.go:1187-1231` | Implement in evaluator batch (`Audit V1.6c`) following V1.6b enablement |
| `AR-LINK-005` | Orphan candidate relative to supplied discovery sources | assisted | NEEDS_ACQUISITION | `url`, `discovery_sources`, `crawl_inlink_count`, `is_start_url`, `crawl_complete` | Partial (`sitecrawl_pages.inlinks`, `crawl_complete` in snapshot metadata) | Adapter does not emit normalized `crawl_inlink_count`; supplemental discovery sources require external policy/ingestion | `internal/sitecrawl/page.go:88-89`; `internal/audit/adapter/adapter.go` | Defer until supplemental discovery sources and inlink graph normalization are established |
| `AR-LINK-006` | Priority-page crawl depth review | manual | MANUAL_OR_ASSISTED | `url`, `crawl_depth`, `priority_page` | Partial (`crawl_depth` observation present on `SubjectURL`) | Explicit `priority_page=true` project policy required; manual review task infrastructure | `internal/audit/adapter/adapter.go:661` (`crawl_depth`); `knowledge/07-v1-atomic-rule-manifest.md:542` | Defer until manual review task orchestration is established |
| `AR-DISC-001` | Sitemap presence matches project policy | assisted | NEEDS_ACQUISITION | `discovered_sitemap_urls`, `sitemap_expected`, `sitemap_discovery_complete` | Missing | Sitemap discovery artifacts not persisted in SQLite; explicit `sitemap_expected` policy required | `internal/audit/adapter/adapter.go:1338` (`GapSitemapDocumentUnavailable`) | Design sitemap persistence tables in crawler/platform layer |
| `AR-DISC-002` | Sitemap document returns 200 | deterministic | NEEDS_ACQUISITION | `sitemap_url`, `sitemap_fetch_status` | Missing | Sitemap HTTP fetch attempts and status codes not persisted in SQLite | `internal/audit/adapter/adapter.go:1338` (`GapSitemapDocumentUnavailable`) | Add sitemap fetch tracking to acquisition architecture |
| `AR-DISC-003` | Sitemap is parseable | deterministic | NEEDS_ACQUISITION | `sitemap_url`, `sitemap_fetch_status`, `sitemap_parse_status`, `sitemap_parse_error` | Missing | Sitemap XML parse status and error diagnostics not persisted in SQLite | `internal/audit/adapter/adapter.go:1338` (`GapSitemapDocumentUnavailable`) | Add sitemap parse diagnostic persistence |
| `AR-DISC-004` | Sitemap-listed URL returns final 200 | deterministic | NEEDS_ACQUISITION | `sitemap_url`, `listed_url`, `listed_url_status` | Missing | Formal `SitemapEntry` correlation linking sitemap documents to listed URLs not persisted | `internal/audit/adapter/adapter.go:1338` (`GapSitemapDocumentUnavailable`) | Implement sitemap entry entity mapping |
| `AR-DISC-005` | Sitemap-listed URL is not noindex | deterministic | NEEDS_ACQUISITION | `sitemap_url`, `listed_url`, `listed_url_status`, `listed_url_effective_noindex` | Missing | Formal `SitemapEntry` correlation linking sitemap documents to listed URLs not persisted | `internal/audit/adapter/adapter.go:1338` (`GapSitemapDocumentUnavailable`) | Implement sitemap entry entity mapping |
| `AR-DISC-006` | Sitemap-listed URL canonicalizes to itself when canonical is declared | deterministic | NEEDS_ACQUISITION | `sitemap_url`, `listed_url`, `listed_url_canonical` | Missing | Formal `SitemapEntry` correlation linking sitemap documents to listed URLs not persisted | `internal/audit/adapter/adapter.go:1338` (`GapSitemapDocumentUnavailable`) | Implement sitemap entry entity mapping |
| `AR-ENTITY-001` | Structured-data block parses successfully | deterministic | NEEDS_ADAPTER | `url`, `structured_block_id`, `structured_format`, `structured_raw`, `structured_parse_status`, `structured_parse_error` | Partial (SiteCrawl stores raw `JSONLD` script strings in `sitecrawl_pages.data`) | Microdata/RDFa not extracted; block indices and formal V1 parse validation contracts not normalized | `internal/sitecrawl/page.go:32`; `internal/audit/adapter/adapter.go:1344` (`GapStructuredDataStatusUnavailable`) | Define structured data parse contract in dedicated vertical slice |
| `AR-RENDER-001` | Title changes after render | deterministic | NEEDS_ACQUISITION | `url`, `title_raw`, `title_rendered` | Missing | SiteCrawl overwrites raw title when rendered; pre-render and post-render HTML not co-preserved | `internal/audit/adapter/adapter.go:1350` (`GapRenderComparisonUnavailable`) | Refactor renderer to preserve raw DOM before headless execution |
| `AR-RENDER-002` | Canonical changes after render | deterministic | NEEDS_ACQUISITION | `url`, `canonical_raw`, `canonical_rendered` | Missing | Pre-render and post-render canonical declarations not co-preserved | `internal/audit/adapter/adapter.go:1350` (`GapRenderComparisonUnavailable`) | Refactor renderer to preserve raw DOM before headless execution |
| `AR-RENDER-003` | Robots directives change after render | deterministic | NEEDS_ACQUISITION | `url`, `meta_robots_raw`, `meta_robots_rendered` | Missing | Pre-render and post-render robots directives not co-preserved | `internal/audit/adapter/adapter.go:1350` (`GapRenderComparisonUnavailable`) | Refactor renderer to preserve raw DOM before headless execution |
| `AR-RENDER-004` | H1 set changes after render | deterministic | NEEDS_ACQUISITION | `url`, `h1_raw`, `h1_rendered` | Missing | Pre-render and post-render H1 sets not co-preserved | `internal/audit/adapter/adapter.go:1350` (`GapRenderComparisonUnavailable`) | Refactor renderer to preserve raw DOM before headless execution |
| `AR-RENDER-005` | Main-text presence changes after render | deterministic | NEEDS_ACQUISITION | `url`, `main_text_present_raw`, `main_text_present_rendered` | Missing | Pre-render and post-render main-text presence not co-preserved; main-text block detector missing | `internal/audit/adapter/adapter.go:1320`, `1350` (`GapMainTextUnavailable`, `GapRenderComparisonUnavailable`) | Refactor renderer and implement semantic main-text extraction |
| `AR-RENDER-006` | Crawlable internal-link set changes after render | deterministic | NEEDS_ACQUISITION | `url`, `internal_links_raw`, `internal_links_rendered` | Missing | Pre-render and post-render link graphs not co-preserved | `internal/audit/adapter/adapter.go:1350` (`GapRenderComparisonUnavailable`) | Refactor renderer to preserve raw DOM before headless execution |
| `AR-AI-001` | OAI-SearchBot robots access matches project policy | assisted | NEEDS_ACQUISITION | `url`, `robots_allowed_oai_searchbot`, `oai_searchbot_access_policy`, `matched_robots_rule` | Missing | Robots.txt not evaluated for OAI-SearchBot; explicit policy required | `internal/sitecrawl/crawler.go`; `internal/audit/adapter/adapter.go:1332` | Add OAI-SearchBot evaluation in AI Search acquisition phase |
| `AR-AI-002` | OAI-SearchBot profile encounters access anomaly | assisted | NEEDS_ACQUISITION | `url`, `default_profile_status`, `oai_profile_status`, `default_challenge_detected`, `oai_challenge_detected` | Missing | Comparative multi-bot profile differential requests not executed by standalone crawler | `internal/audit/adapter/adapter.go:1362` (`GapProfileComparisonUnavailable`) | Defer to post-V1 advanced probe acquisition |
| `AR-AI-003` | GPTBot configuration matches explicit training policy | assisted | NEEDS_ACQUISITION | `robots_allowed_gptbot`, `gptbot_training_policy`, `robots_allowed_oai_searchbot` | Missing | Robots.txt not evaluated for GPTBot; explicit training policy required | `internal/sitecrawl/crawler.go`; `internal/audit/adapter/adapter.go:1332` | Add GPTBot evaluation in AI Search acquisition phase |

---

## 4. In-Depth Candidate Analysis

### 4.1 Crawl / Access / HTTP Candidates

#### `AR-ACC-001 — HTTPS endpoint reachable`
- **Frozen Contract**: P0 deterministic check evaluating whether the site's HTTPS origin returns a successful HTTP response.
- **Evidence Gap**: `GapTLSValidityUnavailable` (`adapter.go:1296`). Standalone SiteCrawl only records connection-level `ssl-error` when a fetch fails completely; it does not perform or record formal TLS certificate validation (`tls_valid`). Furthermore, SiteCrawl crawls the supplied seed URL and does not maintain a dedicated origin-level probe suite (`SubjectSite` or `SubjectOrigin`).
- **Readiness Verdict**: **`NEEDS_ACQUISITION`**.

#### `AR-ACC-002 — Robots access matches explicit crawl intent`
- **Frozen Contract**: P1 assisted check requiring `expected_crawlable=true` and comparing against effective robots decision.
- **Evidence Gap**: `GapRobotsDecisionUnavailable` (`adapter.go:1332`) and `GapRobotsDocumentUnavailable` (`adapter.go:1326`). SiteCrawl evaluates robots during frontier filtering and stores only a coarse `RobotsState` enum string (`allowed`, `blocked`, `unknown`) on `sitecrawl_pages`. It does not preserve the parsed robots.txt AST, line numbers, or specific matched directive patterns required for `matched_robots_rule`. In addition, `expected_crawlable` is an explicit project policy assignment that must not be inferred.
- **Readiness Verdict**: **`NEEDS_ACQUISITION`** (with secondary `NEEDS_POLICY`).

#### `AR-ACC-003 — HTTP status matches explicit expected URL state`
- **Frozen Contract**: P1 deterministic check comparing actual response state against explicit expectation (`live → 2xx`, `redirect → 3xx`, `missing → 404/410`).
- **Evidence Gap**: **Preserved Known Blocker**. For redirected URLs, frozen SiteCrawl persists only the initial 3xx hop and first-hop `RedirectTo`, but does not record the authoritative source-chain `FinalStatus`. Independent target resource fetches cannot be substituted for the source chain's actual resolution.
- **Readiness Verdict**: **`BLOCKED`**.

#### `AR-ACC-005 — Googlebot robots access matches explicit policy`
- **Frozen Contract**: P1 assisted check verifying whether Googlebot robots accessibility matches an explicit project policy.
- **Evidence Gap**: Standalone SiteCrawl parses robots.txt solely against its configured crawler User-Agent (`opts.UserAgent`, defaulting to `SiteCrawl/1.0`). It does not run a secondary Googlebot evaluation pass, nor does it record Googlebot-specific matched rules. `googlebot_access_policy` is an explicit policy.
- **Readiness Verdict**: **`NEEDS_ACQUISITION`** (with secondary `NEEDS_POLICY`).

---

### 4.2 Index Candidates

#### `AR-INDEX-003 — Malformed or unsupported robots directive`
- **Frozen Contract**: P1 deterministic check flagging unrecognized directive tokens in `meta robots` or `X-Robots-Tag`.
- **Evidence Gap**: **Preserved Known Blocker**. Although `adapter.go` extracts raw directive tokens, `/knowledge` does not yet define an authoritative, frozen V1 supported directive registry. Evaluating this rule without a frozen registry would require inventing an ad-hoc grammar.
- **Readiness Verdict**: **`BLOCKED`**.

#### `AR-INDEX-004 — Known-missing probe resolves to 404/410`
- **Frozen Contract**: P1 deterministic check verifying whether a collision-resistant probe expected not to exist returns 404/410.
- **Evidence Gap**: `GapProbeObservationUnavailable` (`adapter.go:1356`). Standalone SiteCrawl only crawls URLs discovered in the frontier; it has no synthetic diagnostic probe generation or execution subsystem.
- **Readiness Verdict**: **`NEEDS_ACQUISITION`**.

#### `AR-INDEX-005 — Snippet restriction conflicts with snippet policy`
- **Frozen Contract**: P1 assisted check evaluating whether snippet controls (`nosnippet`, `max-snippet`, `data-nosnippet`) conflict with `snippet_policy`.
- **Evidence Gap**: SiteCrawl extracts `nosnippet` and `max-snippet` tokens from meta/headers, but does not extract or count HTML element-level `data-nosnippet` attributes (`data_nosnippet_count`). In addition, `snippet_policy` is an explicit project policy assignment that cannot be inferred.
- **Readiness Verdict**: **`NEEDS_ACQUISITION`** (with secondary `NEEDS_POLICY`).

---

### 4.3 Consolidate / Canonical Candidates

#### `AR-CANON-001 — Origin variants normalize to preferred origin`
- **Frozen Contract**: P1 deterministic check verifying that non-preferred protocol and subdomain variants permanently redirect to the preferred origin.
- **Evidence Gap**: `GapProbeObservationUnavailable` (`adapter.go:1356`). Requires executing diagnostic probe requests across origin variants (HTTP, HTTPS, www, non-www) outside the crawl frontier, plus an explicit `preferred_origin` policy assignment.
- **Readiness Verdict**: **`NEEDS_ACQUISITION`** (with secondary `NEEDS_POLICY`).

#### `AR-CANON-002 — Trailing-slash pair is consistently normalized`
- **Frozen Contract**: P1 deterministic check verifying that slash and non-slash URL variants are normalized consistently without duplication.
- **Evidence Gap**: SiteCrawl normalizes URLs in the frontier prior to fetching and does not systematically dual-probe slash variants. Furthermore, `normalized_content_hash` is not computed or persisted.
- **Readiness Verdict**: **`NEEDS_ACQUISITION`**.

#### `AR-CANON-005 — Canonical URL is syntactically resolvable`
- **Frozen Contract**: P1 deterministic check verifying that a raw canonical declaration resolves to a valid HTTP(S) URL.
- **Evidence Gap**: **Preserved Known Blocker**. Frozen SiteCrawl resolves relative canonical URLs during HTML extraction and persists only the resolved URL string in `Page.Canonicals` / `sitecrawl_pages`. Original raw declaration syntax (`href_raw`) and raw syntax errors are discarded.
- **Readiness Verdict**: **`BLOCKED`**.

#### `AR-CANON-010 — Redirect final target is available`
- **Frozen Contract**: P1 deterministic check verifying that the final destination of a redirect chain returns 2xx.
- **Evidence Gap**: **Preserved Known Blocker**. Frozen SiteCrawl persists intermediate redirect hops (`Page.Redirects`) without recording the chain's authoritative `FinalStatus`. Independent fetches of the destination URL cannot be substituted for the source chain's actual transport conclusion.
- **Readiness Verdict**: **`BLOCKED`**.

---

### 4.4 Discover / Internal Links Candidates

#### `AR-LINK-002 — Internal link targets 3xx`
#### `AR-LINK-003 — Internal link targets 4xx`
#### `AR-LINK-004 — Internal link targets 5xx`
- **Frozen Contract**: P1 deterministic checks evaluating whether internal hyperlinks target 3xx (redirects), 4xx (client errors), or 5xx (server errors).
- **Preconditions**: "resolved target is internal and fetchable. Preconditions fail/NOT_APPLICABLE for external/non-HTTP links."
- **Current Evidence Inspection**:
  - **SiteCrawl SQLite Persistence**: Complete! SiteCrawl stores every discovered hyperlink in `sitecrawl_links` with columns `(run_id, src_id, dst_id, seq, placement, flags, anchor)`, where `flags & FlagInternal` distinguishes internal links, and `dst_id` references the target URL in `sitecrawl_urls`. For all crawled targets, `sitecrawl_pages.status` stores the exact HTTP status.
  - **Current Snapshot Evidence**: **INCOMPLETE**. The Evidence Adapter currently builds typed `LinkObservation` objects, but in `adapter.go:1187-1231`, it translates them to `NormalizedObservation`s on `SubjectLink` with ONLY three fields: `link_target` (string), `link_anchor` (string), and `link_location` (string).
  - **Missing Adapter Boundaries**:
    1. **Source URL identity**: No `source_url` (or `link_source_url`) observation or reference to the source URL exists on `SubjectLink`.
    2. **Internal link flag**: `flags & FlagInternal` is not emitted as a normalized observation (`link_is_internal`).
    3. **Target subject correlation**: No `link_target_subject_ref` is emitted connecting `SubjectLink` to `url:<audit_run_id>:<dst_id>`.
    4. **Target fetch status boundary**: The Rule Engine cannot correlate the link edge to the target URL's `http_status` or distinguish an un-crawled link target (e.g. stopped at `max-urls`) from an unavailable response.
- **Readiness Verdict**: **`NEEDS_ADAPTER`**.

---

## 5. Next Batch Selection & Recommendation

### 5.1 Recommendation

**Recommend ONE Evidence-Enablement Slice: `Audit V1.6b — Link Evidence Enablement & Target Correlation Hardening`.**

No atomic rule is currently `READY_TO_IMPLEMENT`. Attempting to implement evaluators for `AR-LINK-002`, `AR-LINK-003`, or `AR-LINK-004` today would result in evaluators returning `UNKNOWN` for every URL or guessing un-normalized relationships.

Following the proven, low-risk architectural precedent of:
- **Canonical Slice**: V1.4b (Canonical Evidence Enablement) → V1.4c (Canonical Evaluators)
- **Redirect Slice**: V1.5a (Redirect Evidence Enablement) → V1.5b (Redirect Evaluators)

the project must first establish truthful normalized link evidence and subject correlation in the adapter before implementing the evaluators.

### 5.2 Milestone Specification: Audit V1.6b

- **Proposed Milestone ID**: `Audit V1.6b (Link Evidence Enablement & Target Correlation Hardening)`
- **Selected Rules to Enable**:
  - `AR-LINK-002` (Internal link targets 3xx)
  - `AR-LINK-003` (Internal link targets 4xx)
  - `AR-LINK-004` (Internal link targets 5xx)
- **Why This Slice is Selected**:
  1. **Zero Crawler Changes**: SiteCrawl already persists all link edges in `sitecrawl_links` with source ID, target ID, placement, internal flags, and anchor text.
  2. **High SEO Value**: Broken links (4xx/5xx) and internal redirect links (3xx) are universally prioritized SEO audit checks.
  3. **Architectural Consistency**: Reuses the exact subject correlation pattern (`canonical_target_subject_ref`) developed in V1.4b.
  4. **Low False-Positive Risk**: Distinguishes un-crawled internal targets (stopped by limits) from confirmed 4xx/5xx targets without guessing.
- **Exact Evidence Field Mapping for V1.6b**:
  - `link_source_url`: source URL string derived from `urlsByID[src_id]`
  - `link_source_subject_ref`: `url:<audit_run_id>:<src_id>`
  - `link_is_internal`: boolean derived strictly from `flags & FlagInternal != 0`
  - `link_target`: resolved target URL string
  - `link_anchor`: anchor text
  - `link_location`: `NAV`, `HEADER`, `FOOTER`, `UNKNOWN`
  - `link_target_subject_ref`: `url:<audit_run_id>:<dst_id>` emitted when `dst_id > 0` maps to an existing `UrlResource` in the snapshot
- **Required Subject Correlation**:
  - Subject Type: `SubjectLink` (`audit.SubjectLink`), with `SubjectRef` formatted as `link:<audit_run_id>:<src_id>:<seq>`.
  - Target Correlation: `link_target_subject_ref` points directly to the target's `SubjectURL` (`url:<audit_run_id>:<dst_id>`).
  - Target Status Boundary: Evaluators in V1.6c will traverse `link_target_subject_ref` to inspect the target's own `http_status` observation. If the target was not crawled or cannot be resolved, the status is safely treated as unavailable (`UNKNOWN`). Target status is NOT duplicated onto `SubjectLink`.
- **Expected PASS/FAIL/UNKNOWN Boundaries in V1.6c**:
  - `PASS`: Target URL response is verified non-3xx (for LINK-002), non-4xx (for LINK-003), non-5xx (for LINK-004).
  - `FAIL`: Target URL response is verified 3xx (for LINK-002), 4xx (for LINK-003), 5xx (for LINK-004).
  - `UNKNOWN`: Target response unavailable (e.g. un-crawled internal link target, crawl halted by limits, missing fetch observation).
  - `NOT_APPLICABLE`: Link is external (`link_is_internal = false`) or non-HTTP scheme.
- **Target Executable Count**:
  - During V1.6b: Remains strictly **`9 / 47`**.
  - During V1.6c: Moves from **`9 → 12 / 47`** (`AR-LINK-002`, `AR-LINK-003`, `AR-LINK-004`).
- **Required Implementation Files for V1.6b**:
  - `internal/audit/adapter/adapter.go` (link observation extraction and normalization)
  - `internal/audit/adapter/adapter_test.go` (focused link normalization regression tests)
- **Protected Boundaries**:
  - `internal/sitecrawl/**` (zero diff)
  - `internal/audit/engine/**` (zero diff during V1.6b)
  - `internal/audit/rules_v1.json` (zero diff)
  - `knowledge/**` (zero diff)
  - `docs/product-workflow-v1.md` (zero diff)
- **Remaining Blockers Preserved**:
  - `AR-ACC-003` (BLOCKED)
  - `AR-CANON-005` (BLOCKED)
  - `AR-CANON-010` (BLOCKED)
  - `AR-INDEX-003` (BLOCKED)

---

## 6. Implementation Sequence Roadmap

### Phase 4 Extension Roadmap:

```text
Audit V1.6a (Current) — Rule Readiness Review & Batch Selection
│   • Analysis and planning only; zero code changes
│   • Executable rules: 9 / 47
│
├── Audit V1.6b — Link Evidence Enablement & Target Correlation Hardening
│   • Adapter-only milestone; normalize link source, internal flag, and target correlation
│   • Bumps NormalizationVersion to v1.7.0
│   • Zero evaluator changes; executable rules remain 9 / 47
│
├── Audit V1.6c — Internal Link Evaluator Batch (AR-LINK-002, AR-LINK-003, AR-LINK-004)
│   • Implement typed evaluators in internal/audit/engine/
│   • Zero adapter or crawler changes
│   • Executable rules advance: 9 → 12 / 47
│
└── Audit V1.7 — Phase 4 Review or Vertical Expansion
    • Review Phase 4 completion criteria
    • Evaluate Structured Data (AR-ENTITY-001) or Sitemap acquisition foundations
```

---

## 7. Boundary & Constraint Verification

- **No New Evaluators**: Verified; executable rule count remains exactly 9.
- **No Schema Changes**: Verified; no SQLite or domain schema modifications.
- **No New Dependencies**: Verified; Go module graph unchanged.
- **No Version Modifications in V1.6a**:
  - `NormalizationVersion` remains `v1.6.0`.
  - `Knowledge revision` remains `v1.4.4`.
- **Known Blockers Preserved**: `AR-ACC-003`, `AR-CANON-005`, `AR-CANON-010`, `AR-INDEX-003` remain strictly BLOCKED.
