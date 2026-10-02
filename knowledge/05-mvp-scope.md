# MVP Scope

## Purpose

Define the first shippable scope of the **Technical Search & GEO Audit Tool**.

This document is a delivery contract, not the full domain model.

The full audit knowledge remains in:

- `02-audit-check-catalog.md` — 100 long-term audit checks
- `03-rule-engine-spec.md` — result semantics and rule execution contract
- `04-ai-search-geo.md` — AI Search / GEO evidence boundaries

## MVP objective

Build a useful Technical Search audit engine that can:

1. crawl a website reliably;
2. collect reproducible technical evidence;
3. evaluate deterministic rules without LLM judgment;
4. surface contextual checks without false certainty;
5. return plain-language findings with evidence and recommended actions.

The MVP should prove the **audit engine**, not maximize feature count.

## Frozen V1 atomic contract

The planning targets in this document are refined by `07-v1-atomic-rule-manifest.md`.

For V1 execution semantics, that manifest defines the exact atomic IDs, `parent_check` mapping, automation class, required inputs, preconditions, statuses, evidence fields, source references, exact counts, and deferred selected catalog checks.

The catalog remains authoritative for broad domain definitions; the atomic manifest is authoritative for the frozen V1 decomposition.

Frozen V1 counts:

- **47 executable atomic rules**;
- **30 deterministic**;
- **16 assisted**;
- **1 manual**;
- **5 selected catalog checks fully deferred**.

The earlier `~28 deterministic + ~12 assisted` split remains planning history, not an execution contract.

---

# MVP model

The original MVP planning scope contains approximately **40 broad catalog-check candidates**, split into two layers. The frozen atomic executable count is defined separately in `07-v1-atomic-rule-manifest.md`:

## Layer A — Core Deterministic Engine

Target:

**~28 executable checks**

These should be decidable from normalized technical data using deterministic code.

Examples:

- HTTP response;
- redirect behavior;
- robots directives;
- canonical primitives;
- broken internal links;
- sitemap validity;
- structured-data syntax.

Atomic deterministic rules may return the statuses explicitly defined by their frozen contract, normally:

- `PASS`
- `WARNING`
- `FAIL`
- `UNKNOWN`
- `NOT_APPLICABLE`

`WARNING` is valid for a deterministic detector when the observed atomic condition is advisory rather than a hard technical failure. They should not require an LLM to decide the technical result.

## Layer B — Assisted / Contextual Audit

Target:

**~12 assisted checks**

These checks may collect evidence automatically, but the final interpretation may require:

- page intent;
- template role;
- business context;
- platform integration;
- manual review;
- external data not included in MVP.

These checks may return:

- `PASS`
- `WARNING`
- `FAIL` when explicit contextual preconditions or project policy are supplied and the result is reproducible from stored evidence;
- `MANUAL_REVIEW`
- `UNKNOWN`
- `NOT_APPLICABLE`

The tool must not force a `FAIL` when evidence or required context is insufficient.

---

# MVP scope boundaries

## Required for V1

The first version should work without:

- Google Search Console OAuth;
- Bing Webmaster API access;
- server-log upload;
- paid SEO APIs;
- distributed crawling infrastructure.

The core engine should operate primarily from:

- HTTP responses;
- robots.txt;
- raw HTML;
- internal links;
- canonical signals;
- XML sitemaps;
- selected rendered HTML when explicitly required.

## Optional external tools during validation

During development or QA, humans may compare results with:

- Google Search Console;
- Bing Webmaster Tools;
- browser DevTools;
- Screaming Frog / Sitebulb;
- Rich Results Test;
- Schema.org Validator.

These tools are **validation references**, not runtime dependencies for V1.

---

# Layer A — Core Deterministic Engine

The following catalog rules are candidate inputs to the deterministic MVP.

Some catalog checks are intentionally broad. Before implementation, they must be split into atomic executable rules where noted.

## 1. Access / HTTP

### Include

- `ACC-001` — HTTPS / connection availability
- `ACC-003` — robots.txt blocking state
- `ACC-006` — repeated 5xx / timeout evidence — **DEFER in frozen V1 manifest**
- `ACC-007` — HTTP response-state validation

### Implementation note

`ACC-007` must not remain one broad executable rule.

Split into atomic checks such as:

- successful page returns expected 2xx;
- broken URL returns 4xx;
- server failure returns 5xx;
- redirect returns 3xx;
- unexpected soft-success patterns require separate review.

Do not infer business intent from status code alone.

---

## 2. Index directives

### Include

- `INDEX-001` — indexability directives
- `INDEX-004` — 404 / 410 response behavior
- `INDEX-007` — snippet restrictions

### Required split before coding

`INDEX-001` should become atomic evidence checks such as:

- meta robots contains `noindex`;
- X-Robots-Tag contains `noindex`;
- conflicting robots directives;
- multiple robots directives;
- malformed or unsupported directives where detectable.

Important:

`noindex detected` is deterministic evidence.

`noindex is wrong` is contextual unless page intent is known.

Therefore:

- detection may be deterministic;
- accidental-noindex classification belongs to the assisted layer unless the project supplies `expected_indexable=true`.

---

## 3. URL normalization / redirects / canonical primitives

### Include

- `CANON-001` — protocol / host normalization
- `CANON-002` — trailing-slash consistency
- `CANON-003` — canonical implementation
- `CANON-007` — redirect chain / loop behavior

### Required split before coding

`CANON-003` is too broad for one executable rule.

Split into atomic checks such as:

- canonical missing;
- multiple canonical declarations;
- malformed canonical;
- canonical target non-200;
- canonical target redirect;
- canonical target 4xx / 5xx;
- canonical target noindex when that state is known;
- self-canonical presence as an informational/contextual check where appropriate.

Do not implement:

`canonical != current URL → FAIL`

Canonical intent depends on consolidation strategy.

### Redirect primitives

`CANON-007` should support at least:

- redirect chain;
- redirect loop;
- redirect target unavailable;
- internal URL linking to redirect destination where applicable.

---

## 4. Internal links

### Include

- `LINK-001` — crawlable `<a href>` links
- `LINK-002` — internal links to non-200 targets

### Required split before coding

`LINK-002` should be separated into:

- internal link → 3xx;
- internal link → 4xx;
- internal link → 5xx.

These cases have different meaning and severity.

The crawler should store:

- source URL;
- target URL;
- anchor;
- HTTP status;
- link location where available.

---

## 5. Sitemap fundamentals

### Include

- `DISC-001` — sitemap discovery
- `DISC-002` — sitemap URL validity

### Interpretation rule

Missing sitemap is **not automatically a technical FAIL**.

It should normally be evaluated as `WARNING`, `MANUAL_REVIEW`, or `NOT_APPLICABLE` depending on explicit project policy and available evidence.

`Information` may later be used as a report/presentation classification, but `INFO` is not a rule status.

`DISC-002` is more suitable for deterministic checks.

Split into:

- sitemap URL returns non-200;
- listed URL returns non-200;
- listed URL is noindex;
- listed URL canonicalizes elsewhere;
- malformed sitemap;
- sitemap cannot be parsed.

---

## 6. Structured-data syntax

### Include

- `ENTITY-001` — structured-data syntax / parse validity

### MVP boundary

V1 may detect and parse:

- JSON-LD;
- Microdata;
- RDFa where feasible.

The tool may report:

- malformed syntax;
- invalid JSON-LD;
- duplicate/contradictory graph nodes where deterministic;
- missing required fields only when a supported schema rule is explicitly implemented.

Do not attempt full page-type appropriateness judgment in the deterministic layer.

---

## 7. Rendering primitives

### Include

- selected technical parts of `RENDER-001`
- selected technical parts of `RENDER-002`

### Deterministic comparison targets

For selected URLs, compare raw HTML vs rendered DOM for:

- title;
- canonical;
- meta robots;
- H1;
- main text presence;
- internal links.

Possible findings:

- metadata changed after render;
- canonical changed after render;
- robots directive changed after render;
- important links appear only after render;
- primary content appears only after render.

Do not claim:

`Google cannot render this page`

unless platform-specific evidence proves that claim.

The MVP may only claim:

`The tool observed a material difference between raw and rendered output.`

---

# Layer B — Assisted / Contextual Audit

The following checks are useful in MVP, but should not be presented as fully deterministic failures.

## 1. Googlebot accessibility interpretation

- `ACC-002`

The crawler can test:

- robots policy;
- HTTP response;
- common Googlebot user-agent profiles.

But V1 should not claim that spoofing a user-agent proves Googlebot can or cannot access the site.

Output should focus on observed access conditions.

---

## 2. CSS / JavaScript resource blocking

- `ACC-004`

The tool can identify blocked resources.

Whether a blocked resource materially harms rendering requires rendered-output evidence.

Default result:

- `WARNING`
- or `MANUAL_REVIEW`

unless a direct rendered-content failure is demonstrated.

---

## 3. CDN / WAF / bot protection

- `ACC-005`

Move out of deterministic V1.

Without verified bot identity, server/CDN logs, or provider-specific verification, the tool should not make strong crawler-blocking claims.

V1 may surface:

- 403;
- 429;
- challenge pages;
- inconsistent responses.

But classification should remain contextual.

---

## 4. Crawl traps / infinite URL spaces

- `ACC-008` — **DEFER in frozen V1 manifest**

Keep as a heuristic check.

Possible evidence:

- rapidly expanding parameter combinations;
- repeated patterns;
- calendar URLs;
- filter/sort permutations;
- high duplicate ratio.

Default result:

`WARNING`

Do not classify parameterized URLs as errors simply because parameters exist.

---

## 5. Soft 404

- `INDEX-003`

Keep as assisted.

Without Search Console or a validated classifier, soft 404 detection is probabilistic.

Possible signals:

- 200 response;
- very low/empty content;
- error language;
- template similarity to known 404 page.

Default result:

`WARNING`

---

## 6. Index pollution / index bloat

- `INDEX-005` — **DEFER in frozen V1 manifest**

Do not classify as deterministic in V1.

The crawler can detect candidate low-value URL families.

Actual index bloat requires an index dataset or external index evidence.

Default result:

`MANUAL_REVIEW`

---

## 7. Signal consistency

- `CANON-004` — **DEFER in frozen V1 manifest**

Useful as an aggregation rule.

The engine may detect conflicts among:

- internal links;
- canonical;
- redirects;
- sitemap.

However, the preferred canonical intent may still require context.

Default result:

`WARNING`

---

## 8. Filter / parameter strategy

- `CANON-005` — **DEFER in frozen V1 manifest**

Move to assisted review.

The tool may cluster parameter patterns and display:

- indexability;
- canonical destination;
- internal links;
- sitemap presence;
- duplicate signatures.

Do not automatically canonicalize all filters to the parent page.

---

## 9. Pagination

- `CANON-006`

Keep as contextual.

The tool may detect:

- paginated URLs;
- crawlable next-page links;
- canonical patterns;
- noindex;
- infinite-scroll behavior.

Do not require deprecated `rel=prev/next`.

Do not require all pagination implementations to follow one universal pattern.

---

## 10. Orphan candidates

- `LINK-003`

Keep, but label precisely.

V1 may report:

`orphan relative to supplied discovery sources`

Examples of discovery sources:

- crawl graph;
- sitemap;
- imported URL list.

Do not claim a URL is globally orphaned unless all relevant sources are available.

---

## 11. Crawl depth

- `LINK-004`

Treat crawl depth as a diagnostic metric.

Do not use a universal threshold such as:

`depth > 3 = FAIL`

Frozen V1 result:

`MANUAL_REVIEW` when a page is explicitly supplied as a priority page and crawl depth is available.

Do not create an `INFO` rule status. `Information` may be used later only as a presentation classification.

---

## 12. AI Search policy checks

### Include

- `AI-001` — OAI-SearchBot accessibility
- `AI-002` — GPTBot policy separation

These are valuable MVP checks, but they should primarily be:

- policy;
- accessibility;
- informational / warning.

### OAI-SearchBot

The tool may detect:

- explicit robots allow/disallow;
- observed HTTP blocking;
- response differences.

Do not claim crawler access guarantees ChatGPT visibility.

### GPTBot

The tool must keep GPTBot policy separate from OAI-SearchBot.

`GPTBot blocked` is not automatically an error.

---

# Explicitly excluded from V1 runtime dependencies

The following catalog checks remain in long-term knowledge but should not require runtime integrations in V1.

## Search Console dependent

- `INDEX-006`
- any rule requiring actual Google index-state confirmation

## Bing API / dashboard dependent

- `DISC-004`
- automated IndexNow dashboard verification

## CMS / historical-data dependent

- `DISC-005` accurate `lastmod` verification against actual edit history

## Server-log dependent

- `OBS-*` log-analysis rules

## Analytics dependent

- GA4 / GTM validation rules

These may enter V1.5 or V2.

---

# V1 atomic ID convention

Broad catalog checks and executable rules are separate concepts.

- Catalog IDs such as `CANON-003` remain stable domain identifiers.
- Atomic executable rules use stable `AR-*` IDs.
- Every atomic rule stores its broad source ID in `parent_check`.
- Do not create suffix IDs such as `CANON-003a`.

The exact frozen mapping is defined in `07-v1-atomic-rule-manifest.md`.

---

# Rule atomicity requirement

Before implementation begins, every deterministic MVP rule must satisfy:

1. one primary technical condition;
2. explicit required inputs;
3. explicit preconditions;
4. explicit `PASS` state;
5. explicit `FAIL` state when a hard failure is valid for that atomic condition, otherwise document that `FAIL` is not used;
6. explicit `WARNING` state when the deterministic condition is advisory;
7. explicit `UNKNOWN` state;
8. explicit `NOT_APPLICABLE` state where relevant;
9. explicit evidence fields;
10. fixtures for every supported result state and edge case.

Broad catalog rules are domain knowledge.

They must not be copied directly into code as oversized boolean checks.

---

# Target V1 architecture

The MVP should support the following pipeline:

```text
Start URL
   ↓
URL discovery
   ↓
HTTP crawler
   ↓
robots parser
   ↓
HTML parser
   ↓
internal-link graph
   ↓
canonical / robots / metadata extraction
   ↓
sitemap parser
   ↓
optional selected-page renderer
   ↓
normalized observations
   ↓
deterministic rule engine
   ↓
assisted/contextual evaluation layer
   ↓
issue aggregation
   ↓
audit report
```

---

# Minimum normalized data model

The exact implementation is not prescribed yet, but V1 should be able to represent at least:

## URL observation

- URL
- normalized URL
- HTTP status
- redirect target
- content type
- response time
- fetch error
- robots.txt allow/block state

## HTML observation

- title
- meta description
- H1
- meta robots
- X-Robots-Tag
- canonical
- internal links
- structured-data blocks
- main-text signal

## Sitemap observation

- sitemap URL
- listed URL
- lastmod when present
- parse errors

## Render comparison

Where rendering is enabled:

- raw value
- rendered value
- difference type

---

# MVP output requirements

Every issue must contain:

- rule ID;
- result status;
- default severity;
- observed evidence;
- expected state;
- affected URL count;
- sample affected URLs;
- recommended action;
- automation type.

The report should distinguish:

- **Issue**
- **Warning**
- **Manual review**
- **Information**

Do not present every observation as an SEO error.

---

# MVP success criteria

The MVP is useful when all of the following are true:

1. it can crawl a small-to-medium public site reliably;
2. deterministic rules produce stable results from the same inputs;
3. every deterministic FAIL contains reproducible evidence;
4. contextual checks do not fabricate certainty;
5. broad domain rules have been decomposed before implementation;
6. audit output can be understood without reading raw crawler data;
7. AI Search checks do not overclaim ranking/citation impact;
8. no Google/Bing OAuth or paid SEO API is required for core use;
9. rule behavior is covered by fixtures and tests;
10. the product can distinguish technical root causes from affected URL counts where evidence permits.

---

# Out of MVP

Do not require the following for first release:

- Google Search Console OAuth;
- Bing Webmaster API integration;
- server-log upload;
- distributed crawler;
- browser render farm;
- scheduled cloud crawling;
- multi-user workspace;
- automatic AI citation tracking;
- universal GEO score;
- LLM-only technical verdicts;
- enterprise crawl scheduling;
- automatic business-priority inference;
- automatic page-intent classification as a hard dependency.

---

# Likely V1.5 additions

Candidates:

- expanded internal-link graph analysis;
- stronger duplicate-content signatures;
- title / description / H1 duplicate rules;
- hreflang;
- richer structured-data validation;
- PageSpeed / CrUX integration;
- configurable page roles;
- explicit indexability-intent configuration;
- export to Excel / CSV / JSON;
- project-level ignore / exception policies.

---

# Likely V2 additions

Candidates:

- Google Search Console integration;
- Bing integration;
- server/CDN log analysis;
- template/root-cause clustering;
- crawl comparison between releases;
- migration mode;
- AI crawler observability;
- citation sampling;
- recurring audits;
- team workflows.

---

# Scope-control rule

A rule belongs to the 100-check catalog because it is useful domain knowledge.

That does **not** mean it belongs in the first executable release.

Use this separation:

```text
100-rule catalog
    ↓
~40 MVP audit checks
    ↓
~28 deterministic engine checks
+
~12 assisted/contextual checks
```

The MVP should optimize for:

**reliability → evidence → low false positives → clear action**

not raw rule count.
