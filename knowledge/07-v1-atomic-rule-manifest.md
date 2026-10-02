# V1 Atomic Rule Manifest

## Status

**Frozen after project approval — 2026-10-01**

This document freezes the V1 executable audit-rule semantics selected from `05-mvp-scope.md`.

It does not define application architecture or production implementation.

## Authority and relationship to other knowledge

- `02-audit-check-catalog.md` owns broad domain checks.
- `03-rule-engine-spec.md` owns generic status/evaluation semantics.
- `04-ai-search-geo.md` owns AI Search / GEO evidence guardrails.
- This document owns the frozen **V1 atomic decomposition and per-rule contract**.
- `06-research-sources.md` owns source IDs and authority levels.

## Resolved project decisions

1. Canonical lifecycle: `Discover → Crawl → Render → Index → Consolidate → Retrieve → Cite → Measure`.
2. `Access` is a Crawl precondition/cross-cutting concern, not a separate lifecycle stage.
3. Rule statuses are only `PASS`, `WARNING`, `FAIL`, `MANUAL_REVIEW`, `NOT_APPLICABLE`, `UNKNOWN`.
4. `Information` may be a report/presentation classification only; `INFO` is not a rule status.
5. Catalog checks and atomic executable rules are separate concepts.
6. Atomic executable IDs use the stable `AR-*` namespace and include `parent_check`.
7. Do not use suffix IDs such as `CANON-003a`.
8. The old `~28 deterministic + ~12 assisted` split is planning history; this manifest owns exact counts.
9. GPTBot policy is independent from OAI-SearchBot accessibility. Blocking GPTBot is not inherently an audit failure.
10. No page intent or business policy is inferred unless explicitly supplied.
11. V1 does not require GSC, Bing APIs, server logs, CMS history, paid APIs, or numeric SEO/GEO scoring.

## Automation vocabulary

- `deterministic`: final atomic condition can be decided from normalized technical evidence.
- `assisted`: deterministic evidence can be collected, but explicit project context/policy or human interpretation may be required.
- `manual`: evidence is collected for guided human judgment; no fabricated machine verdict.

An assisted rule may return `FAIL` only when its explicit contextual preconditions are supplied and the failure is reproducible entirely from stored evidence.

## Severity rule

Default severity is not project priority. `P0` is reserved for conditions that can materially block availability/crawling/indexing at significant scope. Scope and project context may later modify severity/priority, but this manifest introduces no numeric score.

---

# Atomic Rule Manifest

## Crawl / Access / HTTP

### AR-ACC-001 — HTTPS endpoint reachable
- `parent_check`: `ACC-001`
- `lifecycle`: Crawl
- `automation`: deterministic
- `required_inputs`: `site_https_url`, `fetch_attempted`, `tls_valid`, `fetch_error_type`, `fetch_status`
- `preconditions`: HTTPS origin can be derived from project origin.
- `PASS`: TLS connection succeeds and an HTTP response is obtained.
- `FAIL`: DNS, connection, or TLS/certificate failure prevents the HTTPS endpoint from returning an HTTP response.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: fetch was required but no usable acquisition result was produced.
- `NOT_APPLICABLE`: not used.
- `default_severity`: P0
- `evidence_fields`: `site_https_url`, `tls_valid`, `fetch_error_type`, `fetch_status`, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-ACC-002 — Robots access matches explicit crawl intent
- `parent_check`: `ACC-003`
- `lifecycle`: Crawl
- `automation`: assisted
- `required_inputs`: `url`, `robots_allowed_default`, `matched_robots_rule`, `expected_crawlable`
- `preconditions`: robots.txt parsed; `expected_crawlable=true` explicitly supplied.
- `PASS`: URL is allowed for the evaluated crawler.
- `FAIL`: explicit `expected_crawlable=true` and effective robots policy disallows the URL.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: effective robots decision cannot be obtained.
- `NOT_APPLICABLE`: crawl intent was not explicitly supplied.
- `default_severity`: P1
- `evidence_fields`: `url`, `expected_crawlable`, `robots_allowed_default`, `matched_robots_rule`, `robots_txt_url`, `observed_at`
- `source_refs`: `SRC-GOOGLE-ROBOTS-001`

### AR-ACC-003 — HTTP status matches explicit expected URL state
- `parent_check`: `ACC-007`
- `lifecycle`: Crawl
- `automation`: deterministic
- `required_inputs`: `url`, `expected_url_state`, `fetch_status`, `final_status`
- `preconditions`: `expected_url_state` is explicitly `live`, `redirect`, or `missing`.
- `PASS`: `live → 2xx`; `redirect → 3xx`; `missing → final 404/410`.
- `FAIL`: obtained response does not match the explicitly supplied expected state.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: response state cannot be obtained.
- `NOT_APPLICABLE`: expected state unspecified.
- `default_severity`: P1
- `evidence_fields`: `url`, `expected_url_state`, `fetch_status`, `final_status`, `final_url`, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-ACC-004 — Unexpected server-error response
- `parent_check`: `ACC-007`
- `lifecycle`: Crawl
- `automation`: deterministic
- `required_inputs`: `url`, `fetch_status`
- `preconditions`: HTTP response obtained.
- `PASS`: status is not 5xx.
- `FAIL`: status is 5xx.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: no HTTP response obtained.
- `NOT_APPLICABLE`: not used.
- `default_severity`: P1
- `evidence_fields`: `url`, `fetch_status`, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-ACC-005 — Googlebot robots access matches explicit policy
- `parent_check`: `ACC-002`
- `lifecycle`: Crawl
- `automation`: assisted
- `required_inputs`: `url`, `robots_allowed_googlebot`, `googlebot_access_policy`, `matched_robots_rule`
- `preconditions`: Googlebot robots policy is parseable.
- `PASS`: explicit allow/block policy matches effective robots behavior.
- `FAIL`: explicit Googlebot access policy exists and effective robots behavior contradicts it.
- `WARNING`: Googlebot is disallowed while project policy is unspecified.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: effective Googlebot robots state cannot be calculated.
- `NOT_APPLICABLE`: not used.
- `default_severity`: P1
- `evidence_fields`: `url`, `googlebot_access_policy`, `robots_allowed_googlebot`, `matched_robots_rule`, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-ACC-006 — Googlebot profile response anomaly
- `parent_check`: `ACC-002`
- `lifecycle`: Crawl
- `automation`: assisted
- `required_inputs`: `url`, `default_profile_status`, `googlebot_profile_status`, `default_challenge_detected`, `googlebot_challenge_detected`
- `preconditions`: normal and Googlebot-profile requests attempted.
- `PASS`: equivalent accessible response classes and no bot-only challenge observed.
- `FAIL`: not used.
- `WARNING`: Googlebot-profile request receives 403/429/challenge not observed for normal request.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: comparison evidence incomplete.
- `NOT_APPLICABLE`: profile comparison not enabled.
- `default_severity`: P1
- `evidence_fields`: request profiles, statuses, challenge markers, captured response evidence, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`
- `guardrail`: this is an observed profile difference, not proof of actual Googlebot access.

### AR-ACC-007 — Blocked CSS/JS resource affects rendered output
- `parent_check`: `ACC-004`
- `lifecycle`: Render
- `automation`: assisted
- `required_inputs`: `page_url`, `resource_url`, `resource_type`, `resource_robots_allowed`, `render_dependency_observed`, `render_diff_evidence`
- `preconditions`: CSS/JS resource referenced by selected rendered page.
- `PASS`: resource accessible, or blocked without observed material audited-output dependency.
- `FAIL`: resource blocked and direct render evidence proves loss/change of audited main content or metadata because the resource is unavailable.
- `WARNING`: resource blocked but material rendering impact cannot be established.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: renderer/resource evidence unavailable.
- `NOT_APPLICABLE`: no relevant CSS/JS resource.
- `default_severity`: P1
- `evidence_fields`: page/resource URLs, resource type, robots rule, render evidence, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-ACC-008 — Bot-profile access restriction observed
- `parent_check`: `ACC-005`
- `lifecycle`: Crawl
- `automation`: assisted
- `required_inputs`: `url`, `default_profile_status`, `bot_profile_status`, `default_challenge_detected`, `bot_challenge_detected`, `tested_user_agent`
- `preconditions`: normal and alternate bot-profile requests completed.
- `PASS`: no bot-specific access restriction observed.
- `FAIL`: not used.
- `WARNING`: alternate profile receives 403/429/challenge while normal profile remains accessible.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: comparison evidence incomplete.
- `NOT_APPLICABLE`: profile comparison disabled.
- `default_severity`: P1
- `evidence_fields`: request profiles, statuses, challenge markers, relevant headers, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`
- `guardrail`: no verified crawler-blocking claim without verified bot identity/log evidence.

## Index

### AR-INDEX-001 — Effective noindex conflicts with explicit indexability intent
- `parent_check`: `INDEX-001`
- `lifecycle`: Index
- `automation`: assisted
- `required_inputs`: `url`, `effective_noindex`, `expected_indexable`
- `preconditions`: `expected_indexable=true` explicitly supplied.
- `PASS`: no effective `noindex`.
- `FAIL`: effective `noindex` exists.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: directives unavailable.
- `NOT_APPLICABLE`: expected indexability false or unspecified.
- `default_severity`: P1
- `evidence_fields`: URL, explicit intent, raw/normalized meta and X-Robots directives, effective state, `observed_at`
- `source_refs`: `SRC-GOOGLE-ROBOTS-001`

### AR-INDEX-002 — Conflicting index directives
- `parent_check`: `INDEX-001`
- `lifecycle`: Index
- `automation`: deterministic
- `required_inputs`: `url`, `robots_meta_tokens`, `x_robots_tokens`
- `preconditions`: at least one applicable directive parsed.
- `PASS`: no contradictory index/noindex intent in the same applicable scope.
- `FAIL`: contradictory `index` and `noindex` directives explicitly present in the same applicable scope.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: directives present but not reliably parseable.
- `NOT_APPLICABLE`: no robots directive exists.
- `default_severity`: P1
- `evidence_fields`: original directive strings, parsed tokens, source location/header, URL, `observed_at`
- `source_refs`: `SRC-GOOGLE-ROBOTS-001`

### AR-INDEX-003 — Malformed or unsupported robots directive
- `parent_check`: `INDEX-001`
- `lifecycle`: Index
- `automation`: deterministic
- `required_inputs`: `url`, `robots_directive_tokens`, `robots_parse_errors`
- `preconditions`: meta robots or X-Robots-Tag exists.
- `PASS`: all directive tokens are recognized by the supported V1 directive registry.
- `FAIL`: not used.
- `WARNING`: malformed or unsupported token observed.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: raw directive unavailable.
- `NOT_APPLICABLE`: no directive exists.
- `default_severity`: P1
- `evidence_fields`: raw directive, parsed/unsupported tokens, parse error, source location, `observed_at`
- `source_refs`: `SRC-GOOGLE-ROBOTS-001`

### AR-INDEX-004 — Known-missing probe resolves to 404/410
- `parent_check`: `INDEX-004`
- `lifecycle`: Index
- `automation`: deterministic
- `required_inputs`: `missing_probe_url`, `probe_final_status`, `probe_final_url`
- `preconditions`: audit generated a collision-resistant URL expected not to exist.
- `PASS`: final response is 404 or 410.
- `FAIL`: usable final response is obtained but final status is not 404/410.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: probe cannot complete.
- `NOT_APPLICABLE`: not used.
- `default_severity`: P1
- `evidence_fields`: probe URL, redirect path, final URL/status, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-INDEX-005 — Snippet restriction conflicts with snippet policy
- `parent_check`: `INDEX-007`
- `lifecycle`: Index
- `automation`: assisted
- `required_inputs`: `url`, `nosnippet`, `max_snippet`, `data_nosnippet_count`, `snippet_policy`
- `preconditions`: snippet directives extracted.
- `PASS`: no restriction exists, or explicit policy permits observed restriction.
- `FAIL`: `snippet_policy=allow_unrestricted` and a snippet restriction is present.
- `WARNING`: restriction exists while snippet policy is unspecified.
- `MANUAL_REVIEW`: restriction scope requires human interpretation, e.g. block-level `data-nosnippet`.
- `UNKNOWN`: snippet directives unavailable.
- `NOT_APPLICABLE`: explicit policy intentionally requires observed restriction.
- `default_severity`: P1
- `evidence_fields`: raw directives, normalized restriction/value, affected element/count where available, explicit policy, `observed_at`
- `source_refs`: `SRC-GOOGLE-ROBOTS-001`
- `guardrail`: relaxing snippet controls never guarantees AI citation.

### AR-INDEX-006 — 200 response matches known 404 template
- `parent_check`: `INDEX-003`
- `lifecycle`: Index
- `automation`: assisted
- `required_inputs`: `url`, `fetch_status`, `content_fingerprint`, `known_404_content_fingerprint`, `known_404_probe_status`
- `preconditions`: target is 200; known missing probe returned 404/410 and its fingerprint was captured.
- `PASS`: target fingerprint differs from known 404-template fingerprint.
- `FAIL`: not used.
- `WARNING`: target returns 200 and exactly matches known 404-template fingerprint.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: reliable comparison unavailable.
- `NOT_APPLICABLE`: target is not 200.
- `default_severity`: P1
- `evidence_fields`: URL/status, both fingerprints, probe URL/status, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

## Consolidate / Canonical / Redirects

### AR-CANON-001 — Origin variants normalize to preferred origin
- `parent_check`: `CANON-001`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `preferred_origin`, `origin_variant`, `variant_fetch_status`, `variant_final_url`, `redirect_hops`
- `preconditions`: explicit preferred origin; evaluated non-preferred variant returns an HTTP response.
- `PASS`: non-preferred variant permanently redirects via 301/308 to preferred origin.
- `FAIL`: non-preferred variant remains 2xx or resolves to a different origin.
- `WARNING`: preferred origin reached only through temporary redirect.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: preferred origin/variant cannot be evaluated.
- `NOT_APPLICABLE`: evaluated origin already preferred.
- `default_severity`: P1
- `evidence_fields`: origins, full redirect path/statuses, final URL, `observed_at`
- `source_refs`: `SRC-GOOGLE-CANONICAL-001`

### AR-CANON-002 — Trailing-slash pair is consistently normalized
- `parent_check`: `CANON-002`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `slash_url`, `non_slash_url`, both response states, redirect paths, `normalized_content_hash`
- `preconditions`: both variants evaluable.
- `PASS`: one variant is final 2xx and the alternative permanently redirects directly to it.
- `FAIL`: both variants are 2xx with identical normalized content hashes.
- `WARNING`: both are 2xx but hashes differ, or normalization is inconsistent without proving duplication.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: either variant cannot be evaluated.
- `NOT_APPLICABLE`: no meaningful slash pair.
- `default_severity`: P1
- `evidence_fields`: both URLs/statuses, redirect paths, normalized hashes, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-CANON-003 — Canonical declaration missing
- `parent_check`: `CANON-003`
- `lifecycle`: Consolidate
- `automation`: assisted
- `required_inputs`: `url`, `fetch_status`, `content_type`, `canonical_values`
- `preconditions`: final response is 2xx HTML.
- `PASS`: at least one canonical declaration present.
- `FAIL`: not used.
- `WARNING`: no canonical declaration present.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: HTML/head extraction failed.
- `NOT_APPLICABLE`: non-HTML or non-2xx.
- `default_severity`: P1
- `evidence_fields`: URL/status, canonical count/values/source, `observed_at`
- `source_refs`: `SRC-GOOGLE-CANONICAL-001`

### AR-CANON-004 — Multiple canonical declarations
- `parent_check`: `CANON-003`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `url`, `canonical_values`, `normalized_canonical_values`
- `preconditions`: at least one canonical declaration exists.
- `PASS`: exactly one declaration.
- `FAIL`: more than one distinct normalized canonical target.
- `WARNING`: multiple declarations normalize to the same target.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: declarations cannot be normalized.
- `NOT_APPLICABLE`: no canonical exists.
- `default_severity`: P1
- `evidence_fields`: raw/normalized declarations, count, `observed_at`
- `source_refs`: `SRC-GOOGLE-CANONICAL-001`

### AR-CANON-005 — Canonical URL is syntactically resolvable
- `parent_check`: `CANON-003`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `url`, `canonical_raw`, `canonical_resolved_url`, `canonical_parse_error`
- `preconditions`: exactly one canonical declaration exists.
- `PASS`: value resolves to valid HTTP(S) URL.
- `FAIL`: value cannot be resolved as a valid canonical URL.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: canonical source cannot be read.
- `NOT_APPLICABLE`: no single canonical declaration.
- `default_severity`: P1
- `evidence_fields`: raw value, base URL, resolved value, parse error, `observed_at`
- `source_refs`: `SRC-GOOGLE-CANONICAL-001`

### AR-CANON-006 — Canonical target returns final 200
- `parent_check`: `CANON-003`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `url`, `canonical_resolved_url`, `canonical_target_status`
- `preconditions`: exactly one valid canonical target and target fetch attempted.
- `PASS`: target directly returns 200.
- `FAIL`: target returns 3xx, 4xx, or 5xx.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: target cannot be fetched.
- `NOT_APPLICABLE`: no valid canonical target.
- `default_severity`: P1
- `evidence_fields`: source URL, canonical URL, target status/final URL/fetch error, `observed_at`
- `source_refs`: `SRC-GOOGLE-CANONICAL-001`

### AR-CANON-007 — Canonical target is not noindex
- `parent_check`: `CANON-003`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `canonical_resolved_url`, `canonical_target_status`, `canonical_target_effective_noindex`
- `preconditions`: canonical target is 200 and index directives extracted.
- `PASS`: target has no effective noindex.
- `FAIL`: target has effective noindex.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: target directives unavailable.
- `NOT_APPLICABLE`: no usable target.
- `default_severity`: P1
- `evidence_fields`: target URL/status, raw directives, effective state, `observed_at`
- `source_refs`: `SRC-GOOGLE-CANONICAL-001`

### AR-CANON-008 — Redirect chain exceeds one hop
- `parent_check`: `CANON-007`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `url`, `redirect_hops`
- `preconditions`: initial response redirects.
- `PASS`: exactly one redirect reaches final destination.
- `FAIL`: two or more redirect hops occur before final destination.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: redirect traversal incomplete.
- `NOT_APPLICABLE`: URL does not redirect.
- `default_severity`: P1
- `evidence_fields`: complete redirect chain/status per hop, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-CANON-009 — Redirect loop detected
- `parent_check`: `CANON-007`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `url`, `redirect_hops`, `redirect_loop_detected`
- `preconditions`: at least one redirect occurs.
- `PASS`: no URL repeats in traversal.
- `FAIL`: traversal enters loop.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: traversal incomplete before loop state determined.
- `NOT_APPLICABLE`: no redirect.
- `default_severity`: P1
- `evidence_fields`: redirect chain, repeated URL/hop, statuses, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-CANON-010 — Redirect final target is available
- `parent_check`: `CANON-007`
- `lifecycle`: Consolidate
- `automation`: deterministic
- `required_inputs`: `url`, `redirect_hops`, `final_status`, `redirect_loop_detected`
- `preconditions`: redirect exists and no loop detected.
- `PASS`: final destination is 2xx.
- `FAIL`: final destination is 4xx or 5xx.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: final destination cannot be resolved.
- `NOT_APPLICABLE`: no initial redirect.
- `default_severity`: P1
- `evidence_fields`: source URL, chain, final URL/status/fetch error, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-CANON-011 — Pagination exposes crawlable next-page link
- `parent_check`: `CANON-006`
- `lifecycle`: Discover
- `automation`: assisted
- `required_inputs`: `url`, `pagination_detected`, `has_next_page`, `next_page_url`, `next_page_element_tag`, `next_page_href`
- `preconditions`: paginated sequence identified and current URL not final page.
- `PASS`: next page exposed through crawlable `<a href>`.
- `FAIL`: not used.
- `WARNING`: next page appears to depend only on script/event/infinite-scroll interaction.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: pagination/link evidence unavailable.
- `NOT_APPLICABLE`: no pagination or final page.
- `default_severity`: P1
- `evidence_fields`: sequence/current/next URLs, tag/href, discovery evidence, `observed_at`
- `source_refs`: `SRC-GOOGLE-CANONICAL-001`
- `guardrail`: deprecated `rel=prev/next` is not required.

## Discover / Internal Links

### AR-LINK-001 — Internal navigation target lacks crawlable href
- `parent_check`: `LINK-001`
- `lifecycle`: Discover
- `automation`: assisted
- `required_inputs`: `source_url`, `navigation_candidate`, `resolved_internal_target`, `element_tag`, `href`
- `preconditions`: internal navigation candidate has resolvable target.
- `PASS`: target represented by `<a href>` with usable URL.
- `FAIL`: not used.
- `WARNING`: navigation works through another mechanism but no crawlable href exposed.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: target/action cannot be determined.
- `NOT_APPLICABLE`: not a navigation candidate.
- `default_severity`: P1
- `evidence_fields`: source URL, target, tag, href, element context/candidate reason, `observed_at`
- `source_refs`: `SRC-GOOGLE-JS-001`

### AR-LINK-002 — Internal link targets 3xx
- `parent_check`: `LINK-002`
- `lifecycle`: Discover
- `automation`: deterministic
- `required_inputs`: `source_url`, `target_url`, `target_status`, `anchor_text`, `link_location`
- `preconditions`: resolved target is internal and fetchable.
- `PASS`: target is not 3xx.
- `FAIL`: target is 3xx.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: target response unavailable.
- `NOT_APPLICABLE`: external/non-HTTP link.
- `default_severity`: P1
- `evidence_fields`: required inputs, redirect target/final URL, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-LINK-003 — Internal link targets 4xx
- `parent_check`: `LINK-002`
- `lifecycle`: Discover
- `automation`: deterministic
- `required_inputs`: `source_url`, `target_url`, `target_status`, `anchor_text`, `link_location`
- `preconditions`: resolved target is internal and fetchable.
- `PASS`: target is not 4xx.
- `FAIL`: target is 4xx.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: target response unavailable.
- `NOT_APPLICABLE`: external/non-HTTP link.
- `default_severity`: P1
- `evidence_fields`: required inputs, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-LINK-004 — Internal link targets 5xx
- `parent_check`: `LINK-002`
- `lifecycle`: Discover
- `automation`: deterministic
- `required_inputs`: `source_url`, `target_url`, `target_status`, `anchor_text`, `link_location`
- `preconditions`: resolved target is internal and fetchable.
- `PASS`: target is not 5xx.
- `FAIL`: target is 5xx.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: target response unavailable.
- `NOT_APPLICABLE`: external/non-HTTP link.
- `default_severity`: P1
- `evidence_fields`: required inputs, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`

### AR-LINK-005 — Orphan candidate relative to supplied discovery sources
- `parent_check`: `LINK-003`
- `lifecycle`: Discover
- `automation`: assisted
- `required_inputs`: `url`, `discovery_sources`, `crawl_inlink_count`, `is_start_url`, `crawl_complete`
- `preconditions`: URL is in at least one supplied non-crawl source and is not crawl start URL.
- `PASS`: at least one crawlable internal inlink found.
- `FAIL`: not used.
- `WARNING`: zero crawlable internal inlinks found.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: crawl graph incomplete.
- `NOT_APPLICABLE`: no supplied supplemental discovery source.
- `default_severity`: P1
- `evidence_fields`: URL, source memberships, inlink count/samples, crawl completeness, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`
- `guardrail`: wording must remain “orphan relative to supplied discovery sources”.

### AR-LINK-006 — Priority-page crawl depth review
- `parent_check`: `LINK-004`
- `lifecycle`: Discover
- `automation`: manual
- `required_inputs`: `url`, `crawl_depth`, `priority_page`
- `preconditions`: `priority_page=true` explicitly supplied.
- `PASS`: reviewer decision only.
- `FAIL`: reviewer decision only.
- `WARNING`: not used.
- `MANUAL_REVIEW`: preconditions met; surface depth/path with no universal threshold.
- `UNKNOWN`: depth unavailable due incomplete discovery/crawl evidence.
- `NOT_APPLICABLE`: page not explicitly marked priority.
- `default_severity`: P1
- `evidence_fields`: URL, depth, shortest discovered path, priority flag, `observed_at`
- `source_refs`: `SRC-INTERNAL-CHECKLIST-001`
- `guardrail`: no universal `depth > 3 = FAIL` rule.

## Discover / Sitemaps

### AR-DISC-001 — Sitemap presence matches project policy
- `parent_check`: `DISC-001`
- `lifecycle`: Discover
- `automation`: assisted
- `required_inputs`: `discovered_sitemap_urls`, `sitemap_expected`, `sitemap_discovery_complete`
- `preconditions`: root/robots sitemap discovery sources evaluated.
- `PASS`: one or more sitemap URLs discovered.
- `FAIL`: not used.
- `WARNING`: none found and `sitemap_expected=true`.
- `MANUAL_REVIEW`: none found and policy unspecified.
- `UNKNOWN`: discovery could not complete.
- `NOT_APPLICABLE`: `sitemap_expected=false`.
- `default_severity`: P1
- `evidence_fields`: discovered sitemap list, discovery sources checked, explicit policy, `observed_at`
- `source_refs`: `SRC-GOOGLE-SITEMAP-001`

### AR-DISC-002 — Sitemap document returns 200
- `parent_check`: `DISC-002`
- `lifecycle`: Discover
- `automation`: deterministic
- `required_inputs`: `sitemap_url`, `sitemap_fetch_status`
- `preconditions`: sitemap discovered or explicitly supplied.
- `PASS`: response is 200.
- `FAIL`: HTTP response is non-200.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: request fails before HTTP response.
- `NOT_APPLICABLE`: not used.
- `default_severity`: P1
- `evidence_fields`: sitemap URL/status/fetch error, `observed_at`
- `source_refs`: `SRC-GOOGLE-SITEMAP-001`

### AR-DISC-003 — Sitemap is parseable
- `parent_check`: `DISC-002`
- `lifecycle`: Discover
- `automation`: deterministic
- `required_inputs`: `sitemap_url`, `sitemap_fetch_status`, `sitemap_parse_status`, `sitemap_parse_error`
- `preconditions`: sitemap returned 200 and body retrieved.
- `PASS`: parses as supported sitemap or sitemap index.
- `FAIL`: retrieved document is malformed/unparseable.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: parser could not execute.
- `NOT_APPLICABLE`: body not successfully retrieved.
- `default_severity`: P1
- `evidence_fields`: URL, parser result/error/document type, `observed_at`
- `source_refs`: `SRC-GOOGLE-SITEMAP-001`

### AR-DISC-004 — Sitemap-listed URL returns final 200
- `parent_check`: `DISC-002`
- `lifecycle`: Discover
- `automation`: deterministic
- `required_inputs`: `sitemap_url`, `listed_url`, `listed_url_status`
- `preconditions`: URL parsed from sitemap.
- `PASS`: listed URL directly returns 200.
- `FAIL`: listed URL returns 3xx/4xx/5xx.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: listed URL cannot be fetched.
- `NOT_APPLICABLE`: not used.
- `default_severity`: P1
- `evidence_fields`: sitemap/listed URL, status, redirect/fetch evidence, `observed_at`
- `source_refs`: `SRC-GOOGLE-SITEMAP-001`, `SRC-GOOGLE-CANONICAL-001`

### AR-DISC-005 — Sitemap-listed URL is not noindex
- `parent_check`: `DISC-002`
- `lifecycle`: Discover
- `automation`: deterministic
- `required_inputs`: `sitemap_url`, `listed_url`, `listed_url_status`, `listed_url_effective_noindex`
- `preconditions`: listed URL is 200 and directives extractable.
- `PASS`: no effective noindex.
- `FAIL`: effective noindex exists.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: directives unavailable.
- `NOT_APPLICABLE`: listed URL not usable 200 HTML target.
- `default_severity`: P1
- `evidence_fields`: sitemap/listed URL, robots sources/tokens/effective state, `observed_at`
- `source_refs`: `SRC-GOOGLE-SITEMAP-001`, `SRC-GOOGLE-CANONICAL-001`

### AR-DISC-006 — Sitemap-listed URL canonicalizes to itself when canonical is declared
- `parent_check`: `DISC-002`
- `lifecycle`: Discover
- `automation`: deterministic
- `required_inputs`: `sitemap_url`, `listed_url`, `listed_url_canonical`
- `preconditions`: listed URL declares exactly one valid canonical.
- `PASS`: normalized canonical equals listed URL.
- `FAIL`: normalized canonical points elsewhere.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: canonical cannot be resolved.
- `NOT_APPLICABLE`: no usable canonical declaration.
- `default_severity`: P1
- `evidence_fields`: sitemap URL, listed URL, canonical raw/resolved value, `observed_at`
- `source_refs`: `SRC-GOOGLE-SITEMAP-001`, `SRC-GOOGLE-CANONICAL-001`

## Retrieve / Structured Data

### AR-ENTITY-001 — Structured-data block parses successfully
- `parent_check`: `ENTITY-001`
- `lifecycle`: Retrieve
- `automation`: deterministic
- `required_inputs`: `url`, `structured_block_id`, `structured_format`, `structured_raw`, `structured_parse_status`, `structured_parse_error`
- `preconditions`: JSON-LD, Microdata, or RDFa block detected.
- `PASS`: block parses successfully in supported parser.
- `FAIL`: reproducible syntax/extraction errors prevent parsing.
- `WARNING`: not used.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: parser cannot execute.
- `NOT_APPLICABLE`: no structured-data block exists.
- `default_severity`: P2
- `evidence_fields`: block ID/format/raw location, parser status/error, `observed_at`
- `source_refs`: `SRC-GOOGLE-STRUCTURED-001`, `SRC-SCHEMA-VALIDATOR-001`
- `guardrail`: V1 does not infer page-type appropriateness or special GEO schema requirements.

## Render

Common contract for AR-RENDER-001 through AR-RENDER-006:

- `lifecycle`: Render
- `automation`: deterministic
- `default_severity`: P1
- `source_refs`: `SRC-GOOGLE-JS-001`
- `preconditions`: raw and rendered observations are available for a URL selected for rendering.
- `FAIL`: not used; these are deterministic difference detectors, not platform-renderability verdicts.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: rendering requested but required raw/rendered evidence unavailable.
- `NOT_APPLICABLE`: page not selected for rendering.
- `guardrail`: never convert an observed raw/render difference into “Google cannot render this page” without platform-specific evidence.

### AR-RENDER-001 — Title changes after render
- `parent_check`: `RENDER-002`
- `required_inputs`: `url`, `title_raw`, `title_rendered`
- `PASS`: normalized title values equal.
- `WARNING`: title values differ.
- `evidence_fields`: raw/rendered title, `observed_at`

### AR-RENDER-002 — Canonical changes after render
- `parent_check`: `RENDER-002`
- `required_inputs`: `url`, `canonical_raw`, `canonical_rendered`
- `PASS`: normalized canonical sets equal.
- `WARNING`: canonical sets differ.
- `evidence_fields`: raw/rendered canonical sets, `observed_at`

### AR-RENDER-003 — Robots directives change after render
- `parent_check`: `RENDER-002`
- `required_inputs`: `url`, `meta_robots_raw`, `meta_robots_rendered`
- `PASS`: normalized effective directive sets equal.
- `WARNING`: directive sets differ.
- `evidence_fields`: raw/rendered directives and normalized tokens, `observed_at`

### AR-RENDER-004 — H1 set changes after render
- `parent_check`: `RENDER-001`
- `required_inputs`: `url`, `h1_raw`, `h1_rendered`
- `PASS`: normalized H1 sets equal.
- `WARNING`: H1 sets differ.
- `evidence_fields`: raw/rendered H1 values, `observed_at`

### AR-RENDER-005 — Main-text presence changes after render
- `parent_check`: `RENDER-001`
- `required_inputs`: `url`, `main_text_present_raw`, `main_text_present_rendered`
- `PASS`: main-text presence state equal.
- `WARNING`: main-text presence differs.
- `evidence_fields`: raw/rendered boolean states and text-signal evidence, `observed_at`

### AR-RENDER-006 — Crawlable internal-link set changes after render
- `parent_check`: `RENDER-001`
- `required_inputs`: `url`, `internal_links_raw`, `internal_links_rendered`
- `PASS`: normalized crawlable internal-link sets equal.
- `WARNING`: link sets differ.
- `evidence_fields`: raw/rendered sets, added/removed URLs, `observed_at`

## AI Search / GEO policy

### AR-AI-001 — OAI-SearchBot robots access matches project policy
- `parent_check`: `AI-001`
- `lifecycle`: Crawl
- `automation`: assisted
- `required_inputs`: `url`, `robots_allowed_oai_searchbot`, `oai_searchbot_access_policy`, `matched_robots_rule`
- `preconditions`: OAI-SearchBot robots policy can be evaluated.
- `PASS`: explicit allow/block policy matches effective robots behavior.
- `FAIL`: explicit project policy exists and robots configuration contradicts it.
- `WARNING`: OAI-SearchBot blocked while project policy unspecified.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: effective robots state unavailable.
- `NOT_APPLICABLE`: not used.
- `default_severity`: P2
- `evidence_fields`: URL, project policy, effective rule, robots source, `observed_at`
- `source_refs`: `SRC-OPENAI-BOTS-001`
- `guardrail`: FAIL means configuration contradicts explicit project policy; it is not a ChatGPT ranking/citation failure.

### AR-AI-002 — OAI-SearchBot profile encounters access anomaly
- `parent_check`: `AI-001`
- `lifecycle`: Crawl
- `automation`: assisted
- `required_inputs`: `url`, `default_profile_status`, `oai_profile_status`, `default_challenge_detected`, `oai_challenge_detected`
- `preconditions`: default and OAI-profile requests attempted.
- `PASS`: no OAI-specific anomaly observed.
- `FAIL`: not used.
- `WARNING`: OAI profile receives 403/429/challenge while normal request does not.
- `MANUAL_REVIEW`: not used.
- `UNKNOWN`: comparison incomplete.
- `NOT_APPLICABLE`: profile testing disabled.
- `default_severity`: P2
- `evidence_fields`: request profiles, statuses, challenge markers, relevant headers, `observed_at`
- `source_refs`: `SRC-OPENAI-BOTS-001`
- `guardrail`: user-agent profile is not verified crawler identity.

### AR-AI-003 — GPTBot configuration matches explicit training policy
- `parent_check`: `AI-002`
- `lifecycle`: Crawl
- `automation`: assisted
- `required_inputs`: `robots_allowed_gptbot`, `gptbot_training_policy`, `robots_allowed_oai_searchbot`
- `preconditions`: GPTBot robots state can be evaluated.
- `PASS`: explicit allow/block training policy matches effective GPTBot robots behavior.
- `FAIL`: not used.
- `WARNING`: explicit training policy conflicts with actual GPTBot configuration.
- `MANUAL_REVIEW`: training policy not supplied.
- `UNKNOWN`: GPTBot robots state unavailable.
- `NOT_APPLICABLE`: not used.
- `default_severity`: P2
- `evidence_fields`: training policy, GPTBot effective rule, OAI-SearchBot effective rule for comparison, robots source, `observed_at`
- `source_refs`: `SRC-OPENAI-BOTS-001`
- `guardrail`: `GPTBot blocked` can be PASS when policy is to block; it never implies ChatGPT Search is blocked.

---

# Deferred catalog checks

## Fully deferred selected V1 candidates

### ACC-006 — repeated 5xx / timeout instability
`DEFER` because the knowledge does not yet freeze attempt count, retry interval, observation window, or transient-failure policy. A single 5xx remains covered by `AR-ACC-004`.

### ACC-008 — crawl traps / effectively infinite URL spaces
`DEFER` because no approved growth/duplication threshold or heuristic contract exists. Parameters alone are not errors.

### INDEX-005 — index pollution / index bloat
`DEFER` because actual bloat requires index evidence V1 does not have; “low value” URL-family semantics are not sufficiently frozen.

### CANON-004 — canonical signal consistency aggregation
`DEFER` as an executable atomic rule because it is an aggregation concept over atomic link/canonical/redirect/sitemap evidence. Aggregation must not become another oversized boolean rule.

### CANON-005 — parameter/filter canonical strategy
`DEFER` because correct handling depends on content equivalence, page intent, and project strategy. Do not automatically canonicalize filters to parent pages.

## Partially deferred sub-scopes

- `INDEX-003`: fuzzy low-content/error-language soft-404 classifiers deferred; exact known-404-template match remains V1.
- `CANON-006`: canonical/noindex strategy for paginated pages deferred; crawlable next-page discovery remains V1.
- `ENTITY-001`: graph-node contradiction semantics and schema-type required fields deferred until explicit contracts exist.
- `ACC-005`: verified crawler identity/WAF conclusion deferred without logs/provider verification.
- `AI-001`: verified OAI-SearchBot IP/WAF conclusion deferred without logs/provider verification.
- `RENDER-001/002`: platform claim “Google cannot render” explicitly excluded; only observed raw/render differences are evaluated.

---

# Exact counts

| Class | Count |
|---|---:|
| deterministic | 30 |
| assisted | 16 |
| manual | 1 |
| **total executable atomic rules** | **47** |
| fully deferred selected catalog checks | 5 |

Selected broad catalog candidates in `05-mvp-scope.md`: **31**.

- represented by executable V1 atomic rules: **26 parent checks**;
- fully deferred: **5 parent checks**.

---

# Required normalized data fields

This is a logical evidence contract, not a storage/database architecture.

## Core URL / fetch
`url`, `normalized_url`, `is_start_url`, `fetch_attempted`, `fetch_status`, `final_url`, `final_status`, `fetch_error_type`, `content_type`, `response_time_ms`, `observed_at`

## HTTPS / transport
`site_https_url`, `tls_valid`

## Redirect
`redirect_hops[]`, `redirect_loop_detected`

Each hop: `source_url`, `status`, `location`, `resolved_target_url`.

## Explicit project context — never inferred
`preferred_origin`, `expected_url_state`, `expected_crawlable`, `expected_indexable`, `sitemap_expected`, `snippet_policy`, `googlebot_access_policy`, `oai_searchbot_access_policy`, `gptbot_training_policy`, `priority_page`

Controlled values:
- `expected_url_state`: `live | redirect | missing | unspecified`
- `snippet_policy`: `allow_unrestricted | restrict | unspecified`
- crawler access policies: `allow | block | unspecified`
- GPTBot training policy: `allow | block | unspecified`

## robots.txt
`robots_txt_url`, `robots_txt_fetch_status`, `robots_txt_parse_status`, `robots_allowed_default`, `robots_allowed_googlebot`, `robots_allowed_oai_searchbot`, `robots_allowed_gptbot`, `matched_robots_rule`

## Page robots / index directives
`robots_meta_raw[]`, `x_robots_raw[]`, `robots_meta_tokens[]`, `x_robots_tokens[]`, `robots_directive_tokens[]`, `robots_parse_errors[]`, `effective_noindex`, `directive_source`

## Canonical
`canonical_values[]`, `normalized_canonical_values[]`, `canonical_raw`, `canonical_resolved_url`, `canonical_parse_error`, `canonical_target_status`, `canonical_target_effective_noindex`

## HTML
`title`, `meta_description`, `h1[]`, `main_text_present`, `normalized_content_hash`, `internal_links[]`

## Internal links
Per link: `source_url`, `target_url`, `resolved_target_url`, `target_status`, `anchor_text`, `link_location`, `element_tag`, `href`, `navigation_candidate`, `navigation_candidate_reason`

Graph: `crawl_inlink_count`, `crawl_depth`, `crawl_complete`, `discovery_sources[]`

## Sitemap
`discovered_sitemap_urls[]`, `sitemap_url`, `sitemap_fetch_status`, `sitemap_parse_status`, `sitemap_parse_error`, `sitemap_discovery_complete`, `listed_url`, `listed_url_status`, `listed_url_effective_noindex`, `listed_url_canonical`

`lastmod` may be collected but is not verified against CMS history in V1.

## Structured data
`structured_block_id`, `structured_format`, `structured_raw`, `structured_parse_status`, `structured_parse_error`

Supported candidate formats: `json_ld`, `microdata`, `rdfa`.

## Rendering
Raw: `title_raw`, `canonical_raw[]`, `meta_robots_raw`, `h1_raw[]`, `main_text_present_raw`, `internal_links_raw[]`

Rendered: `title_rendered`, `canonical_rendered[]`, `meta_robots_rendered`, `h1_rendered[]`, `main_text_present_rendered`, `internal_links_rendered[]`

Execution: `render_requested`, `render_succeeded`, `render_error`

## Resource dependency
`page_url`, `resource_url`, `resource_type`, `resource_robots_allowed`, `render_dependency_observed`, `render_diff_evidence`

## HTTP profile comparison
`tested_user_agent`, `default_profile_status`, `googlebot_profile_status`, `oai_profile_status`, `bot_profile_status`, `default_challenge_detected`, `googlebot_challenge_detected`, `oai_challenge_detected`, `bot_challenge_detected`, `default_response_fingerprint`, `bot_response_fingerprint`

These are observed request profiles, never verified crawler identity.

## Soft 404
`missing_probe_url`, `probe_final_status`, `known_404_content_fingerprint`, `content_fingerprint`

## Pagination
`pagination_detected`, `page_sequence_id`, `has_next_page`, `next_page_url`, `next_page_element_tag`, `next_page_href`

---

# GEO guardrails preserved

The V1 manifest must not claim:

- guaranteed ChatGPT ranking;
- universal GEO score;
- numeric AI citation probability without a validated model;
- mandatory special GEO schema;
- allowing a crawler guarantees visibility/citation;
- blocking GPTBot blocks ChatGPT Search;
- perfect CWV guarantees AI citation;
- `llms.txt` is mandatory.

OAI-SearchBot and GPTBot remain separate controls. Bot-profile requests are observations only unless bot identity is independently verified by supported evidence.

---

# Freeze rule

Architecture and implementation may consume this manifest, but must not silently reinterpret its rule semantics.

If implementation requires a semantic change:

1. identify the affected `AR-*` rule;
2. update this manifest;
3. increment `rule_version` when executable behavior materially changes;
4. update fixtures/tests together;
5. preserve `parent_check` traceability.
