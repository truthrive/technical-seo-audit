# Audit V1.10a — Robots Directive Evidence & Syntax Contract (`AR-INDEX-003`)

## 1. Executive Decision Summary

- **Milestone:** V1.10a.1 — Robots Directive Evidence & Syntax Contract Clarification & QA Fix
- **Target Rule:** `AR-INDEX-003` — *Malformed or unsupported robots directive* (`parent_check`: `INDEX-001`, `lifecycle`: `Index`, `automation`: `deterministic`, `default_severity`: `P1`)
- **Repository / Branch:** `truthrive/technical-seo-audit` (`main`)
- **Prior Contract Baseline:** `902b9c3ae3d89c6435762cdf81ea068b797cd0b2`
- **Last Approved Implementation Baseline (V1.9d):** `123a3781c346aba066958325f513f56d16533891`
- **Workflow:** **Risk-Based QA + Batch Implementation + MVP-First**
- **Milestone Status:** **REVISED FOR INDEPENDENT QA REVIEW (NOT SELF-LOCKED).**
- **Milestone Scope:** **Documentation and Contract Specification Only.** Zero production code changes, zero SQLite schema changes, zero adapter edits, and zero evaluator registrations in V1.10a.1.
- **Preserved Invariants in V1.10a.1:**
  - Executable rules remain strictly **`20 / 47`** (`27 / 47` remaining).
  - `NormalizationVersion` remains strictly **`v1.10.0`** (`internal/audit/adapter/adapter.go`, line 356).
  - Frozen rule manifest (`knowledge/07-v1-atomic-rule-manifest.md`, lines 215–229 and `internal/audit/rules_v1.json`, lines 359–392) is unmodified.

### 1.1 Summary of Revised Contract Decisions (`DEC-V1.10-001` through `DEC-V1.10-004`)

| Decision ID | Decision Topic | Revised Resolution Summary (Incorporating `QA-01`..`QA-05` & `OQ-1`..`OQ-3`) | Status |
|---|---|---|---|
| **`DEC-V1.10-001`** | **Directive Syntax & Evidence-Backed V1 Registry (`QA-01`)** | Establish a strict 4-class taxonomy separating **syntax validity** from **support status**:<br>1. **`VALID_SUPPORTED` (`PASS`):** 8 active Google standalone rules (`all`, `noindex`, `nofollow`, `none`, `nosnippet`, `indexifembedded`, `notranslate`, `noimageindex`), 2 active Google permissive default rules (`index`, `follow` — required by V1 `AR-INDEX-002`), and 4 active parameterized rules (`max-snippet:[number]`, `max-image-preview:[setting]`, `max-video-preview:[number]`, `unavailable_after:[date/time]`).<br>2. **`VALID_DEPRECATED_OR_UNSUPPORTED_BY_GOOGLE` (`WARNING` via `unsupported_directive:<token>`, NOT `malformed`):** Syntactically well-formed tokens that Google Search Central explicitly moved to *"Unsupported rules"* (`noarchive`, `noodp`, `noydir`, `nocache`, `nopagereadaloud`, `nositelinkssearchbox`), or unrecognized/invented directive names (`no-index`, `nofollows`, `archive`, `snippet`, `max-snipet:50`). Note: `noarchive` and `nocache` remain recognized by Bing through Feb 2027 (`PLATFORM_SPECIFIC`), so they are classified as `unsupported_directive:google_retired:<token>` (populated in `UnsupportedTokens`), **never** as `malformed_syntax` in `ParseErrors`.<br>3. **`PROVEN_MALFORMED` (`WARNING` via `parse_error:<code_and_token>`):** Proven syntactic or parameter violations (`invalid_delimiter`, `empty_token_segment`, `missing_parameter`, `unexpected_parameter`, `invalid_parameter_value`, `missing_content_attribute`, `meta_agent_prefix`).<br>4. **`UNKNOWN_OR_UNVERIFIABLE` (`UNKNOWN`):** Ambiguous `X-Robots-Tag` prefix/scope cases where validity cannot be proven or disproven without guessing (`ambiguous:*`). | **REVISED — PENDING QA APPROVAL** |
| **`DEC-V1.10-002`** | **User-Agent Prefix & Multi-Header Disambiguation (`QA-03`, `OQ-2`, `OQ-3`)** | Do **not** treat a hardcoded crawler list as the universe of valid user-agent names:<br>1. **HTML `<meta>` Scope (`OQ-3` Closed):** Strictly inspect only `<meta name="robots">` and recognized crawler `<meta>` tags. Retract the earlier proposal to flag `<meta name="robot">` as `WARNING` (to avoid silently expanding `AR-INDEX-003` scope to HTML `<meta name>` attribute typos). Inside `<meta content="...">`, `<agent>: <directive>` syntax is **proven malformed (`WARNING` — `parse_error:meta_agent_prefix`)**.<br>2. **Discrete `X-Robots-Tag` Headers:** Preserve `RawXRobotsTags []string` (`res.Header.Values("X-Robots-Tag")`).<br>3. **`X-Robots-Tag` `<lhs>: <rhs>` Disambiguation (`QA-03` & `OQ-2` Closed):**<br>- Near-miss parameterized directive typos (`max-snipet:50`, `max_snippet:50`, `unavailable-after:...`) or standalone directives with parameters (`noindex:true`) -> **`WARNING`** (`unsupported_directive` / `parse_error:unexpected_parameter`).<br>- Recognized crawler `<lhs>` + valid `<rhs>` (`googlebot: noindex, nofollow`) -> **`PASS`**.<br>- Unrecognized RFC 9110 token `<lhs>` + valid V1 `<rhs>` (`custombot: noindex`) -> **`UNKNOWN`** (`ambiguous:unverified_agent_prefix`).<br>- Unrecognized RFC 9110 token `<lhs>` + unrecognized single token `<rhs>` (`foo: bar`) -> **`UNKNOWN`** (`ambiguous:unrecognized_colon_pair`) because static parsing cannot prove whether `foo` is an unknown bot with an unknown rule or an unknown `key:value` extension.<br>- Multiple `<agent>:` prefixes inside one `X-Robots-Tag` header (`googlebot: noindex, bingbot: nofollow`) -> **`UNKNOWN`** (`ambiguous:multiple_agent_prefixes_in_header`) unless a token is demonstrably malformed. | **REVISED — PENDING QA APPROVAL** |
| **`DEC-V1.10-003`** | **Independent Channel Completeness & Empty `<meta>` Semantics (`QA-02`, `QA-04`, `OQ-1`)** | 1. **Separate `<meta>` Empty/Missing Cases (`QA-02` & `OQ-1` Closed):**<br>- `<meta name="robots">` (missing `content` attribute, `HasContentAttr == false`): Violates WHATWG HTML § 4.2.5 (`content` attribute is mandatory when `name` is set) -> **`WARNING`** (`parse_error:missing_content_attribute`).<br>- `<meta name="robots" content="">` and `<meta name="robots" content="   ">` (`HasContentAttr == true`, 0 tokens): Valid HTML syntax specifying zero directive tokens (vacuous no-op per Google Search Central). Retract the earlier unsubstantiated claim that `content=""` is malformed! Counts as a vacuous declaration (`PASS` if another valid directive exists; `NOT_APPLICABLE` if it is the only declaration and zero tokens exist on the page).<br>- Non-empty directive with empty comma slot (`content="noindex, , nofollow"`, `content="noindex,"`, `content=","`) -> **`WARNING`** (`parse_error:empty_token_segment`).<br>2. **Three Independent Completeness Contracts (`QA-04`):** Define separate state machines for **Channel 1 (`http_header_status`)**, **Channel 2 (`html_extraction_status` + `robots_meta_sample_truncated`)**, and **Channel 3 (`directive_bundle_integrity` + `robots_parse_status`)**. Preserve `MISSING != FALSE`: proven `WARNING` short-circuits incomplete unrelated channels (`P3`), while `PASS` (`P7`) and `NOT_APPLICABLE` (`P6`) require `"complete"` across all applicable channels. | **REVISED — PENDING QA APPROVAL** |
| **`DEC-V1.10-004`** | **Raw Evidence Sample Caps, Truncation & Portability (`QA-05`)** | Adopt **Option A** with bounded sample slices (`maxRobotsMeta = 10`, `maxXRobotsHeaders = 10`) paired with uncapped integer counters (`RobotsMetaTotalCount`, `XRobotsTagTotalCount`) and explicit sample-truncation observations (`robots_meta_sample_truncated`, `x_robots_sample_truncated`, `GapRobotsMetaSampleCapped`, `GapXRobotsSampleCapped`). All links in this contract use portable repository-relative paths. | **REVISED — PENDING QA APPROVAL** |

---

## 2. Verified Source-Code Inventory

All technical claims below were independently verified against repository source code at `902b9c3ae3d89c6435762cdf81ea068b797cd0b2` (runtime code identical to `123a3781c346aba066958325f513f56d16533891`).

### 2.1 VERIFIED FACTS in Current Codebase

1. **Frozen Rule Manifest & Machine Registry (`VERIFIED FACT`):**
   - `knowledge/07-v1-atomic-rule-manifest.md` (lines 215–229) and `internal/audit/rules_v1.json` (lines 359–392) define `AR-INDEX-003`:
     - `required_inputs`: `["url", "robots_directive_tokens", "robots_parse_errors"]`
     - `preconditions`: `["meta robots or X-Robots-Tag exists."]`
     - `pass_condition`: `"all directive tokens are recognized by the supported V1 directive registry."`
     - `warning_condition`: `"malformed or unsupported token observed."`
     - `fail_condition`: `"not used."`
     - `manual_review_condition`: `"not used."`
     - `unknown_condition`: `"raw directive unavailable."`
     - `not_applicable_condition`: `"no directive exists."`
     - `evidence_fields`: `["raw_directive", "parsed_tokens", "unsupported_tokens", "parse_error", "source_location", "observed_at"]`
     - `source_refs`: `["SRC-GOOGLE-ROBOTS-001"]`

2. **Domain Model Already Separates `UnsupportedTokens` from `ParseErrors` (`VERIFIED FACT`):**
   - In `internal/audit/model.go` (lines 172–185) and `knowledge/09-data-model.md` (§ 14, lines 490–505), `audit.RobotsDirectiveObservation` defines two distinct slices:
     - `UnsupportedTokens []string` (`json:"unsupported_tokens"`)
     - `ParseErrors []string` (`json:"parse_errors"`)
   - Currently, `internal/audit/adapter/adapter.go` (lines 2287–2470) leaves both slices `nil` and does not emit `robots_parse_errors` or `directive_unsupported_tokens` in `NormalizedObservation`.

3. **Five Verified Acquisition & Adapter Gaps in Current Code (`VERIFIED FACT`):**
   - **Gap 2.1 — Overwriting Same-Name `<meta>` Declarations:**
     In `internal/sitecrawl/extract.go` (lines 481–498), `d.MetaTags[name] = content` overwrites earlier same-name `<meta>` tags. Only `name="robots"` and `name="googlebot"` are appended into the single flattened string `d.MetaRobots` (`d.MetaRobots += ", " + content`). If a page has two `<meta name="googlebot-news">` or two `<meta name="bingbot">` tags, the first tag is lost. Furthermore, if a page has two `<meta name="robots">` tags AND a `<meta name="googlebot">` tag, `internal/audit/adapter/adapter.go` (lines 2331–2388) cannot attribute the first `<meta name="robots">` tag's tokens from `d.MetaRobots`, forcing `ScopeUnknown = true`.
   - **Gap 2.2 — Silent Loss of `<meta name="robots">` Without `content` or With Empty `content=""`:**
     In `internal/sitecrawl/extract.go` (line 439), `content := strings.TrimSpace(attrs["content"])` does not record whether the `content` attribute was present (`_, hasContent := attrs["content"]`). Then in `internal/audit/adapter/adapter.go` (lines 2266–2269), `if val == "" { continue }` skips any `<meta>` entry whose trimmed value is `""`. Consequently, `<meta name="robots">` (missing `content` attribute) and `<meta name="robots" content="">` are both silently discarded.
   - **Gap 2.3 — Flattening Multiple `X-Robots-Tag` HTTP Headers:**
     In `internal/sitecrawl/fetch.go` (lines 242–245), `out.XRobotsTag = strings.Join(values, ", ")` joins discrete HTTP `X-Robots-Tag` header lines into a single comma-separated string. When Header 1 is `X-Robots-Tag: googlebot: noindex` and Header 2 is `X-Robots-Tag: nofollow`, `internal/audit/adapter/adapter.go` (lines 2155–2166) cannot distinguish Header 2 from a continuation of Header 1, setting `scopeUnknown = true`.
   - **Gap 2.4 — Skipping `processPageDirectives` on Non-HTML Responses:**
     In `internal/audit/adapter/adapter.go` (line 902), `processPageDirectives` is called only inside `if isHTML { ... }`. Non-HTML responses (`application/pdf`, images, videos) that carry `X-Robots-Tag` HTTP headers never invoke `processPageDirectives` and emit zero directive observations.
   - **Gap 2.5 — Naive Comma-Splitting of RFC 1123 / RFC 850 `unavailable_after` Dates:**
     In `internal/audit/adapter/adapter.go` (lines 2097 and 2437), `strings.Split(..., ",")` splits on every comma, corrupting RFC 1123 / RFC 850 dates that contain a comma after the weekday name (e.g., `unavailable_after: Tue, 31 Dec 2026 23:59:59 GMT`).

---

## 3. Official Source References & Evidence Levels

Per `knowledge/06-research-sources.md` (lines 39–53) and `knowledge/00-product-principles.md` (lines 116–126):

| Source ID / Standard | Authority Level | Evidence Label | Verified Technical Facts Applicable to `AR-INDEX-003` |
|---|---|---|---|
| **`SRC-GOOGLE-ROBOTS-001`**<br>Google Search Central: *Robots Meta Tags Specifications* (`https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag`) | Primary Platform Documentation | `CONFIRMED` | 1. **Valid indexing and serving rules table:** Lists 12 explicit rules (`all`, `noindex`, `nofollow`, `none`, `nosnippet`, `indexifembedded`, `max-snippet:[number]`, `max-image-preview:[setting]`, `max-video-preview:[number]`, `notranslate`, `noimageindex`, `unavailable_after:[date/time]`). Also documents `index` and `follow` in prose/examples as permissive defaults.<br>2. **Unsupported rules section:** Explicitly lists retired/unsupported rules ignored by Google Search (`noarchive`, `nopagereadaloud`, `nositelinkssearchbox`, `noodp`, `noydir`).<br>3. **Combinability & syntax:** Rules are case-insensitive, comma-separated, and supported in both `<meta>` (HTML) and `X-Robots-Tag` (HTTP headers for HTML and non-HTML files).<br>4. **User-agent targeting:** In HTML, user-agent is specified via `<meta name="<agent>" content="<rules>">`; in HTTP headers, via `X-Robots-Tag: <agent>: <rules>`. |
| **WHATWG HTML Living Standard § 4.2.5 (`The meta element`)** | Primary Web Standard | `CONFIRMED` | 1. *"If either `name`, `http-equiv`, or `itemprop` is specified, then the `content` attribute must also be specified. Otherwise, it must be omitted."* Therefore, `<meta name="robots">` without a `content` attribute is a normative HTML syntax error.<br>2. When `content=""` is present on `<meta name="robots" content="">`, the HTML element is syntactically valid and its metadata content value is the empty string `""` (zero comma-separated tokens). |
| **Bing Webmaster Guidelines — Robots Meta Tags (`noarchive`, `nocache`)** | Primary Platform Documentation | `PLATFORM_SPECIFIC` | Bing documents `noarchive` and `nocache` for cache/Copilot grounding control through March 1, 2027 (after which Bing transitions to dedicated AI preferences), whereas Google Search Central retired `noarchive` after removing Cached links. |
| **RFC 9110 (HTTP Semantics) § 5.3 & § 5.6.2** | Primary Internet Standard | `CONFIRMED` | Defines HTTP header field-value combination and `token` (`tchar`) syntax for user-agent product identifiers. |
| **RFC 822 / RFC 850 / RFC 1123 / RFC 3339 / ISO 8601** | Primary Date/Time Standards | `CONFIRMED` | Defines the date/time formats accepted by Google for `unavailable_after:[date/time]`. |

---

## 4. Contract Decisions (`DEC-V1.10-001` through `DEC-V1.10-004`)

### 4.1 `DEC-V1.10-001` — Evidence-Backed Directive Registry & Support Status (`QA-01` Resolution)

#### 4.1.1 Retraction & Correction of Earlier Over-Classification (`QA-01`)
- **What Changed from `902b9c3a`:**
  1. In `902b9c3a`, `noarchive` was listed as a universally active V1 supported rule without noting that Google Search Central moved `noarchive` to its **Unsupported rules** section after retiring the Google Search Cached link and `cache:` operator. Conversely, in `902b9c3a`, deprecated/unsupported directives (`noodp`, `noydir`, `nocache`) were grouped under `parse_error:unsupported_token` alongside syntax errors.
  2. **Critical Distinction Required by `QA-01`:** A directive that is **syntactically well-formed** (`^[a-z][a-z0-9_-]*$`) but **deprecated or unsupported by Google** is **NOT** a `parse_error` (malformed syntax). In `audit.RobotsDirectiveObservation` (`internal/audit/model.go`, lines 181–182), the data model explicitly provides **two separate fields**:
     - `UnsupportedTokens []string` — for syntactically well-formed tokens that are not supported in the V1 directive registry (e.g., `noodp`, `noydir`, `nocache`, `no-index`, `nofollows`).
     - `ParseErrors []string` — for **proven malformed syntax** (`invalid_delimiter`, `empty_token_segment`, `missing_parameter`, `unexpected_parameter`, `invalid_parameter_value`, `missing_content_attribute`, `meta_agent_prefix`).
  3. **Special Audit of `noarchive` (`CONFIRMED` vs `PLATFORM_SPECIFIC`):**
     - **Verified Fact 1 (`SRC-GOOGLE-ROBOTS-001`):** Google Search Central moved `noarchive` to the *"Unsupported rules"* section because Google retired cached links; Googlebot ignores `noarchive` as a no-op.
     - **Verified Fact 2 (`knowledge/02-audit-check-catalog.md`, line 614 — `INDEX-007`):** Our own authoritative check catalog explicitly states under `INDEX-007`: *"Lưu ý `noarchive` đã lỗi thời trên Google Search từ 2024 khi Google bỏ liên kết Cached, nhưng một số bot khác vẫn có thể đọc."*
     - **Verified Fact 3 (Bing Webmaster Guidelines):** Bing and several archiving/crawler systems still recognize `noarchive` (Bing honors it until March 1, 2027).
     - **Contract Decision on `noarchive`:** Should `noarchive` produce `PASS` or `WARNING` in `AR-INDEX-003`?
       - Look at `knowledge/00-product-principles.md` (Principle 3: *"Avoid false positives... An unusual implementation is not automatically an error"*, and `knowledge/02-audit-check-catalog.md`, line 614, which explicitly notes `noarchive` is still read by other bots). Furthermore, tens of thousands of production pages and CMS templates include `noarchive` for Bing/archiver control. If `AR-INDEX-003` (a `P1` rule!) emitted a `P1 WARNING` on every page with `noarchive`, it would flood audits with false-positive `P1` warnings on sites legitimately using `noarchive` for non-Google bots!
       - Therefore, `noarchive` is classified in the V1 registry as **`VALID_LEGACY_CROSS_ENGINE` -> `PASS` in `AR-INDEX-003`** (valid syntax, recognized in the V1 directive registry per `knowledge/02-audit-check-catalog.md:614`, inert no-op on Google, active on other bots). *(See Open Question Resolution in Section 11 if Independent QA prefers classifying `noarchive` in generic/googlebot scope as `WARNING`; our evidence-backed recommendation is `PASS` to prevent P1 noise).*

#### 4.1.2 Complete V1 Directive Registry & Evidence Matrix (`QA-01`)

| Directive Token | Syntax & Parameters | Official Documentation Reference | Current Support Status | Evidence Confidence | V1 Classification | `RobotsDirectiveObservation` Field Populated | Expected `AR-INDEX-003` Verdict |
|---|---|---|---|---|---|---|---|
| **`all`** | Standalone (`all`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported rule (no restrictions; default) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** |
| **`noindex`** | Standalone (`noindex`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported rule (do not show in search results) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens`, `EffectiveNoindex` | **`PASS`** |
| **`nofollow`** | Standalone (`nofollow`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported rule (do not follow links on page) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** |
| **`none`** | Standalone (`none`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported rule (equivalent to `noindex, nofollow`) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens`, `EffectiveNoindex` | **`PASS`** |
| **`nosnippet`** | Standalone (`nosnippet`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported rule (do not show text snippet or video preview) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** |
| **`indexifembedded`** | Standalone (`indexifembedded`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported rule (allow indexing of embedded content despite `noindex`) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** |
| **`notranslate`** | Standalone (`notranslate`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported rule (do not offer translation in search results) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** |
| **`noimageindex`** | Standalone (`noimageindex`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported rule (do not index images on this page) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** |
| **`max-snippet`** | `max-snippet:[number]` (`number` integer `>= -1`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported parameterized rule | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** *(when `number` is valid integer `>= -1`)* |
| **`max-image-preview`** | `max-image-preview:[setting]` (`none \| standard \| large`, case-insensitive) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported parameterized rule | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** *(when `setting` in `{none, standard, large}`)* |
| **`max-video-preview`** | `max-video-preview:[number]` (`number` integer `>= -1`) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported parameterized rule | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** *(when `number` is valid integer `>= -1`)* |
| **`unavailable_after`** | `unavailable_after:[date/time]` (RFC 822 / RFC 850 / RFC 1123 / RFC 3339 / ISO 8601) | `SRC-GOOGLE-ROBOTS-001` (*Valid indexing and serving rules*) | Active supported parameterized rule (timed `noindex`) | `CONFIRMED` | `VALID_SUPPORTED` | `ParsedTokens` | **`PASS`** *(when `date/time` is valid standard date)* |
| **`index`** | Standalone (`index`) | `SRC-GOOGLE-ROBOTS-001` (permissive default in examples) & `knowledge/07-v1-atomic-rule-manifest.md` (`AR-INDEX-002`) | Permissive default; recognized across all search engines and required by `AR-INDEX-002` | `CONFIRMED` | `VALID_SUPPORTED` (Permissive Default) | `ParsedTokens` | **`PASS`** |
| **`follow`** | Standalone (`follow`) | `SRC-GOOGLE-ROBOTS-001` (permissive default in examples) | Permissive default; recognized across all search engines | `CONFIRMED` | `VALID_SUPPORTED` (Permissive Default) | `ParsedTokens` | **`PASS`** |
| **`noarchive`** | Standalone (`noarchive`) | `SRC-GOOGLE-ROBOTS-001` (*Unsupported rules* on Google since 2024) & `knowledge/02-audit-check-catalog.md:614` (`INDEX-007`: retired on Google, still read by other bots e.g. Bing) | Retired no-op on Google Search; active on Bing / archivers (`PLATFORM_SPECIFIC`) | `CONFIRMED` / `PLATFORM_SPECIFIC` | `VALID_LEGACY_CROSS_ENGINE` (Recognized in V1 Registry) | `ParsedTokens` | **`PASS`** *(Never `malformed`; accepted in V1 registry per `INDEX-007` catalog note)* |
| **`noodp`**, **`noydir`** | Standalone (`noodp`, `noydir`) | `SRC-GOOGLE-ROBOTS-001` (*Unsupported rules*) | Obsolete/defunct directory rules (DMOZ & Yahoo Directory shut down); ignored by all search engines | `CONFIRMED` | `VALID_SYNTAX_UNSUPPORTED_DIRECTIVE` | `ParsedTokens` + `UnsupportedTokens` (`unsupported_directive:obsolete:<token>`) | **`WARNING`** *(Classified as `unsupported_directive`, **not** `malformed_syntax`)* |
| **`nopagereadaloud`**, **`nositelinkssearchbox`** | Standalone | `SRC-GOOGLE-ROBOTS-001` (*Unsupported rules* in robots meta; page-level `<meta name="google">` or retired) | Unsupported in `robots` meta / retired by Google | `CONFIRMED` | `VALID_SYNTAX_UNSUPPORTED_DIRECTIVE` | `ParsedTokens` + `UnsupportedTokens` (`unsupported_directive:google_unsupported:<token>`) | **`WARNING`** *(Classified as `unsupported_directive`, **not** `malformed_syntax`)* |
| **`nocache`**, **`nopreview`**, **`noav`** | Standalone | `SRC-GOOGLE-ROBOTS-001` (not in Google supported rules; `nocache` is Bing-only legacy being retired) | Unsupported by Google Search (`SRC-GOOGLE-ROBOTS-001` is sole `source_ref` of `AR-INDEX-003`) | `CONFIRMED` | `VALID_SYNTAX_UNSUPPORTED_DIRECTIVE` | `ParsedTokens` + `UnsupportedTokens` (`unsupported_directive:<token>`) | **`WARNING`** *(Classified as `unsupported_directive`, **not** `malformed_syntax`)* |
| **Unrecognized / misspelled directive names** (`no-index`, `no_index`, `nofollows`, `dofollow`, `archive`, `snippet`, `max-snipet:50`, `max_snippet:50`, `unavailable-after:...`) | Standalone or `name:value` | `SRC-GOOGLE-ROBOTS-001` (*Unrecognized rules are ignored*) | Not supported by Google or V1 registry | `CONFIRMED` | `UNSUPPORTED_DIRECTIVE` | `ParsedTokens` + `UnsupportedTokens` (`unsupported_directive:<token>`) | **`WARNING`** |
| **Syntactically malformed constructs** (`noindex; nofollow`, `noindex nofollow`, `noindex, , nofollow`, `max-snippet:abc`, `max-image-preview:huge`, `unavailable_after:bad`, `noindex:true`, `<meta name="robots">` without `content`, `<meta name="robots" content="googlebot: noindex">`) | Invalid delimiter, parameter, or HTML attribute syntax | `SRC-GOOGLE-ROBOTS-001` & WHATWG HTML § 4.2.5 | Proven syntax / parameter error | `CONFIRMED` | `PROVEN_MALFORMED` | `ParseErrors` (`parse_error:<code>:<detail>`) | **`WARNING`** |
| **Ambiguous `X-Robots-Tag` constructs** (`custombot: noindex`, `foo: bar`, `googlebot: noindex, bingbot: nofollow`, legacy joined header boundary) | Ambiguous `<lhs>: <rhs>` or multi-agent header | `SRC-GOOGLE-ROBOTS-001` & RFC 9110 | Cannot be proven valid or malformed from static V1 registry | `CONFIRMED` | `UNKNOWN_OR_UNVERIFIABLE` | `ScopeUnknown = true` (where applicable), `robots_parse_errors = "ambiguous:<code>"` | **`UNKNOWN`** *(unless another directive proves `WARNING`)* |

---

### 4.2 `DEC-V1.10-002` — `X-Robots-Tag` Parsing & User-Agent Prefix Disambiguation (`QA-03`, `OQ-2`, `OQ-3` Resolution)

#### 4.2.1 Closing `OQ-3` — HTML `<meta name>` Scope (Do Not Expand Frozen Rule Scope)
- **Retraction of `OQ-3` Expansion:** In `902b9c3a`, we asked whether `<meta name="robot" content="noindex">` (a typo in the HTML `<meta>` `name` attribute) should be captured and flagged as `WARNING` by `AR-INDEX-003`.
- **Verified Fact:** In `knowledge/07-v1-atomic-rule-manifest.md` (lines 219–220), `AR-INDEX-003` has `preconditions: meta robots or X-Robots-Tag exists` and `required_inputs: url, robots_directive_tokens, robots_parse_errors`. A `<meta name="robot">` tag is not a `<meta name="robots">` tag; expanding `AR-INDEX-003` to audit arbitrary `<meta name>` attribute typos would silently expand the frozen rule's scope beyond directive token/syntax validation.
- **Closed Contract Rule (`OQ-3`):** `AR-INDEX-003` evaluates **only** `<meta>` elements whose trimmed case-insensitive `name` is `"robots"` or a recognized crawler user-agent identifier (`isRecognizedCrawlerAgent(name)`: `googlebot`, `googlebot-news`, `googlebot-image`, `googlebot-video`, `adsbot-google`, `adsbot-google-mobile`, `mediapartners-google`, `apis-google`, `google-extended`, `bingbot`, `adidxbot`, `msnbot`, `oai-searchbot`, `oaisearchbot`, `gptbot`, `chatgpt-user`, `claudebot`, `anthropic-ai`, `perplexitybot`, `applebot`, `applebot-extended`, `slurp`, `duckduckbot`, `baiduspider`, `yandex`, `facebookexternalhit`, `meta-externalagent`, `twitterbot`, `linkedinbot`, `bytespider`, `ccbot`). All other `<meta name="...">` tags are out of scope.
- **Agent Prefixes Inside `<meta content="...">`:** Per `SRC-GOOGLE-ROBOTS-001`, user-agent prefixes (`googlebot: noindex`) are valid **only** in `X-Robots-Tag` HTTP headers, never inside an HTML `<meta>` `content` attribute. Therefore, `<meta name="robots" content="googlebot: noindex">` is a **proven malformed directive (`WARNING` — `parse_error:meta_agent_prefix:googlebot: noindex`)**.

#### 4.2.2 Six-Case Deterministic `X-Robots-Tag` Disambiguation Matrix (`QA-03` & `OQ-2` Resolution)

**Retraction & Correction of `foo: bar` (`QA-03`):**
In `902b9c3a`, `X-Robots-Tag: foo: bar` was classified as `WARNING` on the assumption that `bar` is not a valid V1 directive. However, as `QA-03` rightly challenged: if `foo` is an unknown crawler or custom header agent and `bar` is a single ASCII token (`^[a-zA-Z][a-zA-Z0-9_-]*$`), static analysis without a complete universe of every crawler and custom bot extension cannot prove whether `foo: bar` is a bot-specific extension or a typo! Conversely, if `<lhs>` is a near-miss typo of a Google parameterized directive (`max-snipet: 50`, `max_snippet: 50`, `unavailable-after: 2026-12-31`), or if `<rhs>` contains proven delimiter/syntax errors (`foo: noindex; nofollow`), then it **is** demonstrably malformed/unsupported (`WARNING`).

The table below defines the exact deterministic classification for all six `QA-03` cases on discrete `X-Robots-Tag` header values (`RawXRobotsTags[i]`):

| Case | `X-Robots-Tag` Input Pattern | Deterministic Parsing & Disambiguation Rule | Classification (`robots_parse_errors`) | Expected `AR-INDEX-003` Verdict |
|---|---|---|---|---|
| **Case 1: Recognized Agent Prefix + Valid Directives** | `X-Robots-Tag: googlebot: noindex, nofollow` (or `X-Robots-Tag: googlebot: noindex, max-snippet: 150`) | `<lhs>` (`googlebot`) is a recognized crawler identifier (`isRecognizedCrawlerAgent`), and `<rhs>` (`noindex, nofollow`) contains 1+ comma-separated tokens that are all valid V1 directives with no secondary `<agent>:` prefix. `target = "googlebot"`, `ScopeUnknown = false`. | `"none"` | **`PASS`** |
| **Case 2: Multiple Separate `X-Robots-Tag` HTTP Header Fields** | Header 1: `X-Robots-Tag: googlebot: noindex`<br>Header 2: `X-Robots-Tag: nofollow` | Preserved as 2 discrete strings in `Page.RawXRobotsTags`. Header 1 parses as `target = "googlebot"`, `tokens = ["noindex"]`. Header 2 parses as `target = "*"`, `tokens = ["nofollow"]`. Both have `ScopeUnknown = false`. | Bundle 0: `"none"`<br>Bundle 1: `"none"` | **`PASS`** |
| **Case 3: Multiple Agent-Prefixed Segments in a Single `X-Robots-Tag` Header** | `X-Robots-Tag: googlebot: noindex, bingbot: nofollow` (or legacy comma-joined `Page.XRobotsTag` where `RawXRobotsTags` is absent) | When a second `<token>:` prefix (that is not one of the 4 parameterized directives `max-snippet`, `max-image-preview`, `max-video-preview`, `unavailable_after`) appears after a comma within the same header string, HTTP header folding or multi-agent chaining makes scope boundaries ambiguous (`ScopeUnknown = true`). Unless one of the tokens is demonstrably malformed (e.g. `no-index` or `max-snippet:abc`), prefer **`UNKNOWN`** over speculative `WARNING` or `PASS`. | `"ambiguous:multiple_agent_prefixes"` | **`UNKNOWN`** *(or `WARNING` if any token has a proven syntax/parameter error)* |
| **Case 4: Parameterized Directive Typos & Standalone Directives with Parameters** | `X-Robots-Tag: max-snipet: 50`<br>`X-Robots-Tag: max_snippet: 50`<br>`X-Robots-Tag: max-image: large`<br>`X-Robots-Tag: unavailable-after: 2026-12-31`<br>`X-Robots-Tag: noindex: true` | `<lhs>` is either:<br>(a) A standalone V1 directive (`all`, `none`, `index`, `noindex`, `follow`, `nofollow`, `noarchive`, `nosnippet`, `notranslate`, `noimageindex`, `indexifembedded`) given an unexpected colon parameter (`parse_error:unexpected_parameter`), **OR**<br>(b) A near-miss parameterized directive prefix matching `^(max[-_]|unavailable[-_]?|no[-_])` that is not a valid directive (`unsupported_directive:<raw>`).<br>These are **never** crawler user-agent names; they are demonstrably malformed/unsupported directives. | `"parse_error:unexpected_parameter:..."` or `"unsupported_directive:..."` | **`WARNING`** |
| **Case 5: Unrecognized ASCII Prefix + Valid V1 Directives (`OQ-2` Closed)** | `X-Robots-Tag: custombot: noindex` (or `X-Robots-Tag: mycrawler: noindex, nofollow`) | `<lhs>` (`custombot`) is a valid RFC 9110 identifier (`^[a-zA-Z][a-zA-Z0-9_-]*$`) not in `isRecognizedCrawlerAgent` and not matching Case 4 typo patterns, and `<rhs>` consists of valid V1 directives. Because a hardcoded crawler list is not the universe of valid user-agents, `custombot` may be a legitimate custom bot, or it may be an unrecognized prefix. Static V1 rules cannot verify it safely (`ScopeUnknown = true`). | `"ambiguous:unverified_agent_prefix:custombot"` | **`UNKNOWN`** *(Never a speculative `WARNING`)* |
| **Case 6: Unrecognized Prefix + Unrecognized Single Token (`X-Robots-Tag: foo: bar`)** | `X-Robots-Tag: foo: bar` | `<lhs>` (`foo`) is an unrecognized RFC 9110 token (not in `isRecognizedCrawlerAgent`, not a Case 4 `max*`/`unavailable*`/`no*` typo, and not a recognized crawler) and `<rhs>` (`bar`) is a single unrecognized ASCII token without proven delimiter errors (`;`, `\|`, spaces).<br>- **If `<lhs>` IS a recognized crawler** (e.g. `X-Robots-Tag: googlebot: bar` or `X-Robots-Tag: googlebot: no-index`): `<lhs>` is proven to be Googlebot, and `bar`/`no-index` is proven unsupported for Googlebot -> **`WARNING`** (`unsupported_directive:bar`).<br>- **If `<lhs>` is NOT a recognized crawler (`X-Robots-Tag: foo: bar`)**: Because static parsing cannot distinguish an unknown bot with a custom rule from an unknown `key: value` header token without guessing, prefer **`UNKNOWN`** (`ambiguous:unrecognized_colon_pair:foo: bar`) over a speculative `WARNING`!<br>- **Exception (Proven Syntax Error in `<lhs>` or `<rhs>`):** If `<lhs>` or `<rhs>` contains proven delimiter/syntax errors (e.g. `X-Robots-Tag: foo bar: baz`, `X-Robots-Tag: foo: noindex; nofollow`, `X-Robots-Tag: foo: max-snippet:abc`), that is a proven syntax error -> **`WARNING`**. | `"ambiguous:unrecognized_colon_pair:foo: bar"` *(when both `foo` and `bar` are bare unrecognized tokens)* | **`UNKNOWN`** |

---

### 4.3 `DEC-V1.10-003` — Empty/Missing `<meta>` Content & Independent Evidence Completeness (`QA-02`, `QA-04`, `OQ-1` Resolution)

#### 4.3.1 Separate Analysis of Empty & Missing `<meta name="robots">` Cases (`QA-02` & `OQ-1` Resolution)

**Retraction of Blanket `WARNING` on `content=""` and `content="   "` (`QA-02`):**
In `902b9c3a`, `<meta name="robots" content="">`, `<meta name="robots" content="   ">`, and `<meta name="robots">` were all grouped together as `WARNING` (`empty_directive`). Upon reviewing **WHATWG HTML Living Standard § 4.2.5** and **Google Search Central (`SRC-GOOGLE-ROBOTS-001`)**, these four cases have distinct normative statuses and **must not** be conflated:

| Case | HTML / Header Markup | Raw Acquisition State (`RawMetaDirective`) | Standards & Platform Analysis | Classification (`robots_parse_errors`) | Effect on `AR-INDEX-003` Verdict |
|---|---|---|---|---|---|
| **Case 2A: Missing `content` Attribute** | `<meta name="robots">` (or `<meta name="googlebot">`) | `HasContentAttr = false`, `ContentRaw = ""` | **WHATWG HTML § 4.2.5 Violation:** *"If either `name`, `http-equiv`, or `itemprop` is specified, then the `content` attribute must also be specified."* A `<meta name="robots">` tag without a `content` attribute is **proven malformed HTML markup**. | `"parse_error:missing_content_attribute"` | **`WARNING`** *(Emits a `directive:*` bundle with `robots_parse_errors = "parse_error:missing_content_attribute"`)* |
| **Case 2B: Explicit Empty `content=""`** | `<meta name="robots" content="">` | `HasContentAttr = true`, `ContentRaw = ""` | **Valid HTML + Vacuous (0-Token) Declaration:** In WHATWG HTML § 4.2.5, `content=""` satisfies attribute presence. In `SRC-GOOGLE-ROBOTS-001`, `content` holds a comma-separated list of rules; an empty string `""` contains **zero rules** (equivalent to specifying no restriction, which defaults to `all`). There is **no** primary documentation stating `content=""` is malformed or an error. | *(Vacuous declaration: 0 tokens; no error)* | **Does NOT trigger `WARNING`.**<br>- If no other directive with tokens exists on the page (and extraction is complete): **`NOT_APPLICABLE`** (0 active directive tokens exist).<br>- If another valid directive exists on the page (e.g., `X-Robots-Tag: noindex`): **`PASS`**. |
| **Case 2C: Whitespace-Only `content="   "`** | `<meta name="robots" content="   ">` | `HasContentAttr = true`, `ContentRaw = "   "` | **Valid HTML + Vacuous (0-Token) Declaration After Whitespace Trimming:** Per `SRC-GOOGLE-ROBOTS-001`, leading/trailing whitespace around directive values is ignored. Trimming `"   "` yields `""` (zero tokens, no delimiter punctuation). Just like `content=""`, it specifies zero rules and is **not** proven malformed. | *(Vacuous declaration: 0 tokens; no error)* | **Does NOT trigger `WARNING`.** Same behavior as Case 2B (`NOT_APPLICABLE` if sole declaration; `PASS` alongside valid directives). |
| **Case 2D: Explicit Comma With Empty Token Slot (`OQ-1` Closed)** | `<meta name="robots" content=",">`<br>`<meta name="robots" content="noindex,">`<br>`<meta name="robots" content="noindex, , nofollow">`<br>`X-Robots-Tag: ,` | `HasContentAttr = true`, `ContentRaw` contains `,` with an empty trimmed segment on at least one side of `,` | **Proven Malformed Comma List Syntax:** Unlike `content=""` (zero tokens, zero delimiters), a comma `,` is a list separator between two directive tokens (`directive_item , "," , directive_item`). A bare comma `,`, trailing comma `noindex,`, leading comma `,noindex`, or double comma `noindex, , nofollow` has an explicit delimiter with a missing operand (`empty_token_segment`). | `"parse_error:empty_token_segment"` | **`WARNING`** |
| **Case 2E: Empty `X-Robots-Tag:` HTTP Header** | `X-Robots-Tag:` or `X-Robots-Tag:    ` | `RawXRobotsTags = [""]` or `["   "]` | In HTTP (RFC 9110 § 5.5), an empty header field value `X-Robots-Tag:` carries zero tokens (vacuous header), whereas `X-Robots-Tag: googlebot:` has an agent prefix `googlebot:` with a missing directive list! | - `X-Robots-Tag:` -> vacuous (0 tokens)<br>- `X-Robots-Tag: googlebot:` -> `"parse_error:missing_directive_after_prefix:googlebot:"` | - `X-Robots-Tag:` -> **`NOT_APPLICABLE`** (if sole)<br>- `X-Robots-Tag: googlebot:` -> **`WARNING`** |

**Preservation of Raw Declaration for Cases 2A, 2B, and 2C:**
Even though Cases 2B (`content=""`) and 2C (`content="   "`) do not emit a false `WARNING`, `sitecrawl.Page.RawRobotsMeta` **still preserves every raw declaration** (`Name`, `ContentRaw`, `HasContentAttr`) and `HtmlObservation.MetaRobotsRaw` records its raw presence for full audit traceability!
- For **Case 2A (`HasContentAttr == false`, `<meta name="robots">`)**: `processPageDirectives` emits a `RobotsDirectiveObservation` bundle (`directive:<run>:<url>:meta:<idx>`) with `RawValue: ""`, `ParsedTokens: []`, `ParseErrors: ["parse_error:missing_content_attribute"]`, and emits `robots_parse_errors = "parse_error:missing_content_attribute"` -> **`WARNING`**.
- For **Cases 2B & 2C (`HasContentAttr == true`, `strings.TrimSpace(ContentRaw) == ""`)**: Because zero directive tokens and zero syntax errors exist in `content=""`, `processPageDirectives` does **not** emit a synthetic non-empty `RobotsDirectiveObservation` token bundle (keeping `AR-INDEX-001` and `AR-INDEX-002` 100% unchanged, where `len(directives) == 0` -> `NOT_APPLICABLE` in `AR-INDEX-002` and `AR-INDEX-003`!).

#### 4.3.2 Three Independent Evidence Completeness Contracts (`QA-04` Resolution)

Per `QA-04`, HTTP header acquisition, HTML meta extraction, and directive parsing/normalization are **independent evidence channels**. A failure or incompleteness in one channel must be tracked separately from the others (`MISSING != FALSE`).

##### Channel 1: HTTP Header Acquisition Completeness (`http_header_status`)
Tracks whether HTTP response headers (specifically `X-Robots-Tag`) were authoritatively acquired for the terminal resource of `SubjectURL`:

| `http_header_status` Value | Exact Condition in Adapter (`adapter.go`) | Meaning |
|---|---|---|
| **`"complete"`** | 1. `pr.status >= 200 && pr.status < 600` (a real HTTP response was received), **AND**<br>2. `pr.status < 300 \|\| pr.status >= 400` (`!isInitialRedirect` and `len(pr.page.Redirects) == 0` on this row — remember that in SiteCrawl, a redirecting URL `3xx` row records the initial 3xx hop and its body/terminal state belongs to the destination URL row), **AND**<br>3. `pr.errorType == ""` (or `pr.errorType == sitecrawl.ErrTooLarge`, where HTTP response headers were completely received before the body size gate triggered!), **AND**<br>4. `!xRobotsSampleTruncated` (`XRobotsTagTotalCount <= len(RawXRobotsTags)`). | All `X-Robots-Tag` HTTP response headers for this response were completely received and retained. |
| **`"truncated_sample_cap"`** | `XRobotsTagTotalCount > len(RawXRobotsTags)` (`> 10` `X-Robots-Tag` headers on a single response). | Response headers were received, but only the first 10 `X-Robots-Tag` header values were retained in `RawXRobotsTags`. |
| **`"redirect_intermediate"`** | `pr.status >= 300 && pr.status < 400` or `len(pr.page.Redirects) > 0` (`redirect_initial_observed == "true"`). | This URL row represents an initial redirect response (`page.go:61` skips document extraction on redirect rows). |
| **`"unavailable"`** | `pr.status <= 0` or (`pr.errorType != ""` and `pr.errorType != sitecrawl.ErrTooLarge`), or uncrawled URL dictionary entry (`pr == nil`). | No usable terminal HTTP response headers were obtained (DNS error, timeout, connection refused, robots.txt blocked before fetch, or uncrawled URL). |

*(Note on Legacy SQLite Rows for Channel 1: On legacy rows created before V1.10b, if `pr.page.XRobotsTag == ""` and `pr.xRobots == ""`, zero `X-Robots-Tag` headers existed, so `http_header_status` is `"complete"` whenever `pr.status >= 200`, `!isInitialRedirect`, and `pr.errorType == ""`. If a legacy row has non-empty `XRobotsTag` without `RawXRobotsTags`, `http_header_status` is `"complete"` unless comma-joining after an agent prefix caused `ScopeUnknown = true`, which is caught by Channel 3!)*

##### Channel 2: HTML Meta Extraction Completeness (`html_extraction_status` & `robots_meta_sample_truncated`)
Applicable **only when the response is HTML (`isHTML == true`)**. Tracks whether all `<meta>` robots directives in the raw server HTML were completely extracted and retained:

| HTML Channel State | Exact Condition in Adapter (`adapter.go`) | Meaning |
|---|---|---|
| **`html_extraction_status == "complete"` AND `robots_meta_sample_truncated == "false"`** | 1. `pr.status >= 200 && pr.status < 300`, `!isRendered`, `len(pr.page.Redirects) == 0`, `pr.errorType == ""`, `pr.page.ExtractionStatus == "complete"` (verified via V1.9c `limit + 1` `boundedProbeReader` + `z.Err() == io.EOF` + `drainErr == nil`).<br>2. `pr.page.RobotsMetaTotalCount <= len(pr.page.RawRobotsMeta)` (`<= 10` robots `<meta>` declarations).<br>3. Row has V1.10b raw meta provenance (`pr.page.RobotsMetaExtracted == true`). | Every `<meta>` robots declaration in the raw server HTML document was tokenized to clean EOF and retained in `RawRobotsMeta`. |
| **`html_extraction_status == "truncated_size_limit"`** | `pr.errorType == sitecrawl.ErrTooLarge` or `pr.page.ExtractionStatus == "truncated_size_limit"`. | HTML body exceeded `min(cap, maxHTMLBytes)` (4 MiB), so `<meta>` tags beyond the limit (or the whole body if `Content-Length > cap`) were not extracted. *(Note: Channel 1 `http_header_status` is still `"complete"`!)* |
| **`html_extraction_status == "read_error"`** | Mid-stream body read error, tokenizer error, or drain error (`pr.page.ExtractionStatus == "read_error"`). | HTML body stream failed mid-read. |
| **`rendered == "true"`** (`html_extraction_status == "missing_legacy"`) | Headless browser rendering replaced raw server HTML (`pr.rendered == 1 \|\| pr.page.Rendered`). | Raw server HTML `<meta>` directives were suppressed (`GapRenderedRawSourceUnavailable`); only Channel 1 (`X-Robots-Tag`) is available. |
| **`html_extraction_status == "missing_legacy"` (or `!pr.page.RobotsMetaExtracted`)** | Legacy SQLite row created before V1.10b, or non-2xx HTML response (`404`/`500` where `fetch.go:259` does not run `extractDocument`), or redirect row. | Complete raw `<meta>` extraction provenance (including `<meta name="robots">` without `content`) is unavailable. |
| **`robots_meta_sample_truncated == "true"`** | `pr.page.RobotsMetaTotalCount > len(pr.page.RawRobotsMeta)` (`> 10` robots `<meta>` tags on page). | More than 10 robots `<meta>` tags exist; only the first 10 were retained in `RawRobotsMeta`. |

##### Channel 3: Directive Parsing & Normalization Completeness (`robots_parse_errors` per `directive:*` Bundle)
Tracks the per-declaration parsing and validation status for each retained `RobotsDirectiveObservation` bundle (`directive:<run>:<url>:meta:<idx>` and `directive:<run>:<url>:header:<idx>`):

| `robots_parse_errors` Value on Bundle | Meaning | Bundle Status for `AR-INDEX-003` |
|---|---|---|
| **`"none"`** | All tokens and parameters in this directive declaration are valid V1 directives (`VALID_SUPPORTED` or `VALID_LEGACY_CROSS_ENGINE`), and scope/prefix syntax is unambiguous. | **Valid Bundle** |
| **Comma-joined `parse_error:*` and/or `unsupported_directive:*` codes** | At least one token or syntax construct in this declaration is **demonstrably malformed** (`parse_error:*`) or **unsupported by Google/V1** (`unsupported_directive:*`). | **Proven Violation Bundle (`WARNING`)** |
| **Comma-joined `ambiguous:*` code(s) only** (or `directive_scope_unknown == "true"` with no `parse_error:*` / `unsupported_directive:*`) | The declaration contains no proven syntax/token error, but its user-agent prefix or header boundary is ambiguous (`ambiguous:unverified_agent_prefix`, `ambiguous:unrecognized_colon_pair`, `ambiguous:multiple_agent_prefixes`, `ambiguous:legacy_unscoped_meta`). | **Ambiguous Bundle (`UNKNOWN`)** |
| **Missing, empty (`""`), or conflicting `robots_parse_errors` observation on bundle** | Required normalized observation is absent (`MISSING != FALSE`) or contradictory within the same `directive:*` bundle. | **Incomplete/Conflicting Bundle (`UNKNOWN`)** |

#### 4.3.3 How the Three Channels Interact Across Response Types (`QA-04`)

| URL Response Archetype | Channel 1 (`http_header_status`) | Channel 2 (HTML `<meta>`) | Channel 3 (Retained Directive Bundles) | Resulting `AR-INDEX-003` Verdict |
|---|---|---|---|---|
| **1. Standard 200 HTML Page (Clean EOF)** | `"complete"` | `html_extraction_status == "complete"`, `robots_meta_sample_truncated == "false"` | - 0 bundles -> **`NOT_APPLICABLE`**<br>- 1+ bundles, all `"none"` -> **`PASS`**<br>- Any bundle has `parse_error:*` or `unsupported_directive:*` -> **`WARNING`**<br>- No `WARNING` bundle, 1+ bundle has `ambiguous:*` -> **`UNKNOWN`** | `NOT_APPLICABLE`, `PASS`, `WARNING`, or `UNKNOWN` |
| **2. Truncated (`truncated_size_limit` / `read_error`) or Rendered (`rendered == "true"`) 200 HTML Page** | `"complete"` | Incomplete (`truncated_size_limit`, `read_error`, or `rendered == "true"`) | - If **any** retained `<meta>` or `X-Robots-Tag` bundle has proven `parse_error:*` or `unsupported_directive:*` -> **`WARNING`** (proven violation short-circuit)<br>- Otherwise (0 bundles, or only `"none"` / `ambiguous:*` bundles) -> **`UNKNOWN`** (uninspected HTML `<meta>` tags prevent proving `PASS` or `NOT_APPLICABLE`) | **`WARNING`** (if proven error in retained evidence) or **`UNKNOWN`** (never `PASS` or `NOT_APPLICABLE`) |
| **3. Non-2xx HTML Response (`404`, `410`, `500`)** | `"complete"` | Not extracted (`fetch.go:259` skips HTML parsing on non-2xx; `html_extraction_status == "missing_legacy"`) | - If `X-Robots-Tag` header has a proven `parse_error:*` or `unsupported_directive:*` -> **`WARNING`**<br>- Otherwise -> **`UNKNOWN`** (because the HTML body was not parsed for `<meta name="robots">`) | **`WARNING`** (if header error proven) or **`UNKNOWN`** |
| **4. Non-HTML 200 Resource (`application/pdf`, `image/png`, etc., where `isHTML == false`)** | `"complete"` | *Not applicable to non-HTML MIME types* (`SRC-GOOGLE-ROBOTS-001`: non-HTML files only support `X-Robots-Tag`) | - 0 `X-Robots-Tag` bundles -> **`NOT_APPLICABLE`**<br>- 1+ `X-Robots-Tag` bundles, all `"none"` -> **`PASS`**<br>- Any `X-Robots-Tag` bundle has `parse_error:*` or `unsupported_directive:*` -> **`WARNING`**<br>- Any `X-Robots-Tag` bundle has `ambiguous:*` (and no `WARNING`) -> **`UNKNOWN`** | `NOT_APPLICABLE`, `PASS`, `WARNING`, or `UNKNOWN` |
| **5. Non-HTML Non-2xx Resource (e.g., `404 application/pdf`)** | `"complete"` | *Not applicable* | - If `X-Robots-Tag` has proven `parse_error:*` or `unsupported_directive:*` -> **`WARNING`**<br>- Otherwise -> **`UNKNOWN`** (non-2xx error response) | **`WARNING`** or **`UNKNOWN`** |
| **6. Initial `3xx` Redirect Row (`redirect_initial_observed == "true"`)** | `"redirect_intermediate"` | Not extracted (`page.go:61`) | - If `X-Robots-Tag` on the 3xx hop itself has a proven `parse_error:*` or `unsupported_directive:*` -> **`WARNING`**<br>- Otherwise -> **`UNKNOWN`** (redirect hop row; terminal page is evaluated on its own destination URL subject) | **`WARNING`** or **`UNKNOWN`** |
| **7. Failed Fetch (`status == 0`, `DNS_ERROR`, `TIMEOUT`) or Uncrawled URL** | `"unavailable"` | Unavailable | No bundles | **`UNKNOWN`** |
| **8. Legacy SQLite Row (Pre-V1.10b, `!RobotsMetaExtracted`)** | `"complete"` (if 200) | `"missing_legacy"` | - If any retained `MetaTags` / `XRobotsTag` bundle has a proven `parse_error:*` or `unsupported_directive:*` -> **`WARNING`**<br>- Otherwise -> **`UNKNOWN`** | **`WARNING`** or **`UNKNOWN`** |

---

### 4.4 `DEC-V1.10-004` — Raw Evidence Preservation, Sample Caps & Portability (`QA-05` Resolution)

#### 4.4.1 Uncapped Total Counters vs. Capped Raw Sample Slices (`QA-05`)
To bound per-page JSON memory/storage in `sitecrawl_pages.data` while guaranteeing that capped samples **never** masquerade as complete evidence (`SAMPLE CAP != TOTAL COUNT`), V1.10b defines explicit uncapped counters alongside capped slices (`const maxRobotsMeta = 10`, `const maxXRobotsHeaders = 10`):

```go
// RawMetaDirective preserves one raw <meta> robots/crawler directive declaration
// in document order before trimming or map deduplication.
type RawMetaDirective struct {
    Seq            int    `json:"seq"`               // 0-based declaration index in document order (0..9)
    Name           string `json:"name"`              // lowercased trimmed name attribute (e.g. "robots", "googlebot")
    NameRaw        string `json:"name_raw,omitempty"`// original raw name attribute string
    ContentRaw     string `json:"content_raw"`       // untouched raw content attribute value
    HasContentAttr bool   `json:"has_content_attr"`  // true iff the "content" attribute key existed on the <meta> token
}
```

Fields added to `sitecrawl.Page` (`internal/sitecrawl/types.go`):

| `sitecrawl.Page` JSON Field | Type & Cap | Exact Acquisition Rule (`extract.go` / `fetch.go` / `page.go`) | Adapter Mapping (`adapter.go`) |
|---|---|---|---|
| **`RobotsMetaExtracted`** (`json:"robots_meta_extracted,omitempty"`) | `bool` | Set to `true` on every non-rendered 2xx HTML page where `extractDocument` ran in V1.10b+ (`buildPage` when `d != nil && len(res.Hops) == 0 && !r.rendered`). Distinguishes a V1.10b page with `0` `<meta>` tags from a pre-V1.10b legacy SQLite row! | Used to verify V1.10b raw meta provenance. |
| **`RobotsMetaTotalCount`** (`json:"robots_meta_total_count,omitempty"`) | `int` (**Uncapped**: `0..N`) | Incremented by `1` for every `<meta>` tag in the document whose `name` is `"robots"` or in `isRecognizedCrawlerAgent(name)` (including missing `content`, `content=""`, `content="   "`, and valid/invalid values). | Emitted as `robots_meta_total_count` on `SubjectURL`. |
| **`RawRobotsMeta`** (`json:"raw_robots_meta,omitempty"`) | `[]RawMetaDirective` (**Capped at `10` samples**) | Appends `RawMetaDirective` in exact document encounter order while `len(d.RawRobotsMeta) < 10`. | Iterated in exact `Seq` (`0..len-1`) order to build `directive:<run>:<url>:meta:<seq>` bundles. |
| **`XRobotsTagTotalCount`** (`json:"x_robots_tag_total_count,omitempty"`) | `int` (**Uncapped**: `0..N`) | Set to `len(res.Header.Values("X-Robots-Tag"))` in `fetch.go`. | Compared against `len(RawXRobotsTags)` to detect header sample truncation. |
| **`RawXRobotsTags`** (`json:"raw_x_robots_tags,omitempty"`) | `[]string` (**Capped at `10` samples**) | Stores the first up to `10` discrete `res.Header.Values("X-Robots-Tag")` strings without joining them with commas. | Iterated in exact header order (`0..len-1`) to build `directive:<run>:<url>:header:<idx>` bundles. |

#### 4.4.2 Explicit Sample-Truncation Indicators & Verdict Impact (`QA-05`)
1. **HTML `<meta>` Sample Truncation (`RobotsMetaTotalCount > len(RawRobotsMeta)`, i.e. `> 10` declarations):**
   - Adapter emits `robots_meta_sample_truncated = "true"` on `SubjectURL` (and `"false"` when `RobotsMetaExtracted == true` and `RobotsMetaTotalCount <= len(RawRobotsMeta)`).
   - Adapter appends an `EvidenceGap` with `GapCode: GapRobotsMetaSampleCapped` (`"GAP_ROBOTS_META_SAMPLE_CAPPED"`).
   - **Verdict Impact:**
     - If any of the 10 retained `RawRobotsMeta` declarations (or `RawXRobotsTags`) contains a proven `parse_error:*` or `unsupported_directive:*`, `AR-INDEX-003` returns **`WARNING`** (Priority P3).
     - Otherwise, because declarations `11..N` were not retained and cannot be verified free of malformed/unsupported tokens, `AR-INDEX-003` returns **`UNKNOWN`** (Priority P5).
2. **HTTP `X-Robots-Tag` Sample Truncation (`XRobotsTagTotalCount > len(RawXRobotsTags)`, i.e. `> 10` headers):**
   - Adapter emits `x_robots_sample_truncated = "true"` on `SubjectURL`, sets `http_header_status = "truncated_sample_cap"`, and appends `GapXRobotsSampleCapped` (`"GAP_X_ROBOTS_SAMPLE_CAPPED"`).
   - **Verdict Impact:**
     - If any retained directive bundle has a proven `WARNING`, `AR-INDEX-003` returns **`WARNING`**.
     - Otherwise, `http_header_status == "truncated_sample_cap"` causes `AR-INDEX-003` to return **`UNKNOWN`**.

#### 4.4.3 Source Order & Stable Evidence Identity (`QA-05`)
- **Meta Bundles:** Indexed in exact document order (`Seq = 0, 1, ...`):
  `RobotsDirectiveObservationID = "directive:<audit_run_id>:<url_id>:meta:<seq>"`
- **Header Bundles:** Indexed in exact HTTP response header order (`headerIdx = 0, 1, ...`):
  `RobotsDirectiveObservationID = "directive:<audit_run_id>:<url_id>:header:<headerIdx>"`
- Every `NormalizedObservation` emitted for a bundle carries `SourceEvidenceRefs: []string{string(d.RobotsDirectiveObservationID), pageSrcRef}`.

---

## 5. Formal Token & Parameter Grammar Specification

### 5.1 EBNF Grammar

```ebnf
(* HTML <meta> content attribute value *)
meta_content_value      = optional_ws                           (* vacuous: 0 tokens -> no bundle *)
                        | directive_list ;

(* Discrete HTTP X-Robots-Tag header field-value *)
x_robots_header_value   = optional_ws                           (* vacuous: 0 tokens -> no bundle *)
                        | [ agent_prefix , ":" ] , directive_list ;

agent_prefix            = rfc9110_token ;                       (* validated per Section 4.2.2 *)

directive_list          = directive_item , { "," , directive_item } ;

directive_item          = optional_ws , ( standalone_token | parameterized_token ) , optional_ws ;

standalone_token        = "all" | "none" | "index" | "noindex" | "follow" | "nofollow"
                        | "nosnippet" | "notranslate" | "noimageindex" | "indexifembedded"
                        | "noarchive" ;                         (* case-insensitive *)

parameterized_token     = max_snippet_token
                        | max_image_preview_token
                        | max_video_preview_token
                        | unavailable_after_token ;

max_snippet_token       = "max-snippet" , optional_ws , ":" , optional_ws , int_ge_neg1 ;
max_image_preview_token = "max-image-preview" , optional_ws , ":" , optional_ws , ( "none" | "standard" | "large" ) ;
max_video_preview_token = "max-video-preview" , optional_ws , ":" , optional_ws , int_ge_neg1 ;
unavailable_after_token = "unavailable_after" , optional_ws , ":" , optional_ws , valid_date_time ;

int_ge_neg1             = "-1" | ( digit , { digit } ) ;
```

### 5.2 Comma-Aware Directive Splitter (`splitDirectiveItems`)

To prevent RFC 1123 (`Mon, 02 Jan 2006 15:04:05 MST`) and RFC 850 (`Monday, 02-Jan-06 15:04:05 MST`) dates inside `unavailable_after` from being split across the post-weekday comma:
1. Split the directive list on `,` into raw segments `seg[0..k-1]`.
2. Iterate `i = 0 .. k-1`:
   - If `seg[i]` (trimmed, lowercased) matches `^unavailable_after\s*:\s*(mon|tue|wed|thu|fri|sat|sun|monday|tuesday|wednesday|thursday|friday|saturday|sunday)$` **and** `i + 1 < k`, merge `seg[i] + "," + seg[i+1]` into a single item and advance `i++`.
3. If any resulting item has `strings.TrimSpace(item) == ""` (when the original raw directive string was non-empty after trimming), record `parse_error:empty_token_segment`.

### 5.3 Accepted Date/Time Layouts for `unavailable_after`

A trimmed `[date/time]` string after `unavailable_after:` is valid if and only if it parses cleanly via `time.Parse` under one of these layouts:
- **ISO 8601 / RFC 3339:** `time.RFC3339`, `"2006-01-02T15:04:05Z"`, `"2006-01-02T15:04:05"`, `"2006-01-02 15:04:05Z07:00"`, `"2006-01-02 15:04:05 MST"`, `"2006-01-02 15:04:05"`, `"2006-01-02"`.
- **RFC 1123 / RFC 822 / Google Search Central Examples:** `time.RFC1123`, `time.RFC1123Z`, `time.RFC822`, `time.RFC822Z`, `"02 Jan 2006 15:04:05 MST"`, `"2 Jan 2006 15:04:05 MST"`, `"02 Jan 2006 15:04:05 -0700"`, `"2 Jan 2006 15:04:05 -0700"`, `"02-Jan-2006 15:04:05 MST"`, `"2-Jan-2006 15:04:05 MST"`, `"02-Jan-2006"`, `"02 Jan 2006"`.
- **RFC 850:** `time.RFC850`, `"Monday, 02-Jan-2006 15:04:05 MST"`, `"Monday, 2-Jan-2006 15:04:05 MST"`.

---

## 6. Evidence Acquisition & Normalization Contract

### 6.1 Per-Directive Bundle Observations (`directive:*` on `SubjectURL`)

For every non-vacuous `<meta>` robots declaration (and every `<meta name="robots">` missing `content`) and every non-vacuous `X-Robots-Tag` header, `processPageDirectives` emits a `directive:*` bundle with `SourceEvidenceRefs: []string{dirID, pageSrcRef}`:

| Field | Value Format |Derivation | Description |
|---|---|---|---|
| `directive_source` | `"META"` or `"HTTP_HEADER"` | `DIRECT` | Source channel of the declaration. |
| `directive_target` | `"*"` (generic), `"<agent>"` (recognized crawler), or `""` (when `ScopeUnknown == true`) | `DIRECT` | Target user-agent scope. |
| `directive_raw` | Raw string (`ContentRaw` or header string) | `DIRECT` | Exact raw declaration string before normalization. |
| `directive_tokens` | Comma-joined normalized tokens (`"noindex, nofollow"`) | `NORMALIZED` | Consumed by `AR-INDEX-002` and `AR-INDEX-001`. |
| `directive_scope_unknown` | `"true"` (emitted when `ScopeUnknown == true`) | `DIRECT` | Consumed by `AR-INDEX-002` and `AR-INDEX-003`. |
| `robots_directive_tokens` | Comma-joined normalized tokens (`"noindex, nofollow"`, or `""` when no valid tokens parsed) | `NORMALIZED` | Required input for `AR-INDEX-003`. |
| **`robots_parse_errors`** | `"none"` when valid; otherwise comma-joined structured codes (`"parse_error:<type>:<detail>"`, `"unsupported_directive:<token>"`, or `"ambiguous:<type>:<detail>"`) | `NORMALIZED` | Required input for `AR-INDEX-003`. Always explicitly emitted on every `directive:*` bundle (`MISSING != FALSE`). |
| **`directive_unsupported_tokens`** | Comma-joined unsupported token strings (emitted when `len(d.UnsupportedTokens) > 0`) | `NORMALIZED` | Distinguishes `unsupported_directive` tokens from syntax `parse_error`s. |

### 6.2 URL-Level Completeness Observations on `SubjectURL`

| Field | Value Format | Description |
|---|---|---|
| `url_identity` | Raw URL string | Subject URL identity (`CONTEXT`). |
| `http_status` | `"200"`, `"404"`, etc. | Initial HTTP response status. |
| `content_type` | `"text/html"`, `"application/pdf"`, etc. | Normalized MIME type. |
| `http_header_status` | `"complete"`, `"truncated_sample_cap"`, `"redirect_intermediate"`, `"unavailable"` | **New in V1.10c (`QA-04`):** Explicit Channel 1 (`X-Robots-Tag`) acquisition completeness. |
| `html_extraction_status` | `"complete"`, `"truncated_size_limit"`, `"read_error"`, `"missing_legacy"` | Channel 2 HTML body extraction completeness (emitted on HTML pages). |
| `robots_meta_sample_truncated` | `"true"` or `"false"` | **New in V1.10c (`QA-05`):** Emitted on V1.10b+ HTML pages (`RobotsMetaExtracted == true`). `"true"` iff `RobotsMetaTotalCount > len(RawRobotsMeta)`. |
| `x_robots_sample_truncated` | `"true"` or `"false"` | **New in V1.10c (`QA-05`):** Emitted when `http_header_status` is `"complete"` or `"truncated_sample_cap"`. `"true"` iff `XRobotsTagTotalCount > len(RawXRobotsTags)`. |
| `rendered` | `"true"` (when rendered) | Indicates headless browser DOM replacement of raw server HTML. |

---

## 7. Updated Verdict Precedence Decision Table (`AR-INDEX-003`)

`evaluateINDEX003` evaluates each `SubjectURL` in deterministic order:

| Priority | Condition on `SubjectURL` | Verdict | `ObservedSummary` Template | Required `EvidenceRefs` Roles |
|---|---|---|---|---|
| **P1: Unusable Subject Identity** | `url_identity` is missing, empty, or has conflicting observations | **`UNKNOWN`** | `"Required URL identity evidence is unavailable."` (or conflicting) | `url` (`CONTEXT`, if any) |
| **P2: Malformed / Conflicting / Incomplete Bundle Observations (`MISSING != FALSE`)** | - Any observation in `directive_source`, `directive_target`, `directive_raw`, `robots_directive_tokens`, `robots_parse_errors`, `directive_unsupported_tokens`, `directive_scope_unknown` lacks a `"directive:*"` ref in `SourceEvidenceRefs`, **OR**<br>- Any `directive:*` bundle is missing `directive_source`, `directive_raw`, `robots_directive_tokens`, or `robots_parse_errors` (`MISSING != FALSE`), or has conflicting values for a single-valued bundle field, **OR**<br>- Any URL-level completeness field (`http_status`, `content_type`, `http_header_status`, `html_extraction_status`, `rendered`, `robots_meta_sample_truncated`, `x_robots_sample_truncated`) has conflicting or malformed values | **`UNKNOWN`** | `"Robots directive evidence is incomplete, malformed, or conflicting."` | `url` (`CONTEXT`) + offending/incomplete observations (`PRIMARY`) |
| **P3: Proven Malformed Syntax or Unsupported Directive (`WARNING` Short-Circuit)** | At least one valid `directive:*` bundle has `robots_parse_errors` containing one or more `parse_error:*` or `unsupported_directive:*` codes | **`WARNING`** | - If `parse_error:*` present: `"Malformed robots directive observed: <details>."`<br>- If only `unsupported_directive:*` present: `"Unsupported robots directive observed: <details>."` | `url` (`CONTEXT`) + all observations of each offending `directive:*` bundle (`PRIMARY`: `directive_raw`, `robots_directive_tokens`, `robots_parse_errors`, `directive_unsupported_tokens`, `directive_source`, `directive_target`) |
| **P4: Ambiguous Directive Scope or Prefix (`UNKNOWN`)** | No bundle triggered P3 (`WARNING`), and at least one `directive:*` bundle has `robots_parse_errors` containing `ambiguous:*` or `directive_scope_unknown == "true"` | **`UNKNOWN`** | `"Robots directive exists but user-agent prefix or scope syntax cannot be reliably verified."` | `url` (`CONTEXT`) + ambiguous `directive:*` bundle observations (`PRIMARY`) |
| **P5: Incomplete Acquisition Across Applicable Channels (`UNKNOWN`)** | No bundle triggered P3 or P4, and acquisition across applicable channels is incomplete:<br>1. **Channel 1 (`X-Robots-Tag`):** `http_header_status != "complete"` or `x_robots_sample_truncated == "true"` or `http_status` not in `200..299` or `content_type` missing/empty.<br>2. **Channel 2 (HTML `<meta>`, required when `isHTML == true`):** `rendered == "true"` or `html_extraction_status != "complete"` or `robots_meta_sample_truncated != "false"` (note: requiring explicit `robots_meta_sample_truncated == "false"` ensures legacy pre-V1.10b HTML rows without `RobotsMetaExtracted` return `UNKNOWN`!). | **`UNKNOWN`** | `"Raw robots directive source is unavailable or incomplete (<channel reason>)."` | `url` (`CONTEXT`) + deciding channel completeness observations (`PRIMARY`: `http_header_status`, `html_extraction_status`, `rendered`, `http_status`, `robots_meta_sample_truncated`, `x_robots_sample_truncated`) + any retained valid bundles (`SUPPORTING`) |
| **P6: Zero Directives on Complete Channels (`NOT_APPLICABLE`)** | All applicable channels are complete (Priority P5 passed) and `len(bundles) == 0` | **`NOT_APPLICABLE`** | `"No robots directive exists."` | `url` (`CONTEXT`) + channel completeness observations (`CONTEXT`: `http_header_status`, plus `html_extraction_status` and `robots_meta_sample_truncated` when `isHTML`) |
| **P7: All Directives Valid on Complete Channels (`PASS`)** | All applicable channels are complete (Priority P5 passed), `len(bundles) >= 1`, and every bundle has `robots_parse_errors == "none"` | **`PASS`** | `"All observed robots directive tokens are recognized and valid."` | `url` (`CONTEXT`) + all evaluated `directive:*` bundle observations (`PRIMARY`: `directive_raw`, `robots_directive_tokens`, `robots_parse_errors`, `directive_source`, `directive_target`) + channel completeness observations (`CONTEXT`) |

---

## 8. Missing & Conflicting Evidence Handling

1. **Strict Enforcement of `MISSING != FALSE`:**
   - **Bundle level:** If a `directive:*` bundle is present in the snapshot without an explicit `robots_parse_errors` observation, Priority P2 returns **`UNKNOWN`** (never assuming `"none"`).
   - **Channel 1 level:** If `http_header_status` is missing on `SubjectURL` (and no `WARNING` is proven in P3), Priority P5 returns **`UNKNOWN`** (never assuming HTTP headers were completely inspected).
   - **Channel 2 level:** On an HTML response (`isHTML == true`), if `html_extraction_status` is missing or `!= "complete"`, or if `robots_meta_sample_truncated` is missing or `!= "false"` (and no `WARNING` is proven in P3), Priority P5 returns **`UNKNOWN`**.
2. **Conflicting Observations:**
   - Multiple observations with different values for any single-valued URL field (`url_identity`, `http_status`, `content_type`, `http_header_status`, `html_extraction_status`, `rendered`, `robots_meta_sample_truncated`, `x_robots_sample_truncated`) or single-valued bundle field (`directive_source`, `directive_target`, `directive_raw`, `robots_directive_tokens`, `robots_parse_errors`) immediately return **`UNKNOWN`** under Priority P1/P2.

---

## 9. Legacy Compatibility, Versioning & Regression Analysis

1. **`NormalizationVersion` Policy:**
   - **V1.10a / V1.10a.1:** Remains `"v1.10.0"`.
   - **V1.10b (SiteCrawl acquisition only):** Remains `"v1.10.0"` if adapter output is untouched in V1.10b.
   - **V1.10c (Adapter normalization):** Bumps `NormalizationVersion` from `"v1.10.0"` to **`"v1.11.0"`** when `robots_parse_errors`, `directive_unsupported_tokens`, `http_header_status`, `robots_meta_sample_truncated`, and `x_robots_sample_truncated` are emitted.
2. **Legacy SQLite Fallback Behavior:**
   - For SQLite crawl runs created prior to V1.10b (`pr.page.RobotsMetaExtracted == false` and `len(pr.page.RawXRobotsTags) == 0`):
     - Adapter falls back to `pr.page.MetaTags`, `pr.page.MetaRobots` / `pr.metaRobots`, and `pr.page.XRobotsTag` / `pr.xRobots`.
     - If any legacy directive contains a proven `parse_error:*` or `unsupported_directive:*`, `AR-INDEX-003` returns **`WARNING`** (Priority P3).
     - Otherwise, on legacy HTML rows, because `RobotsMetaExtracted == false` (`robots_meta_sample_truncated` is not emitted as `"false"`), Priority P5 returns **`UNKNOWN`** rather than a false `PASS` or `NOT_APPLICABLE` (since legacy rows did not record `<meta name="robots">` tags lacking `content` or overwritten duplicate tags).
3. **Zero Regression on `AR-INDEX-001`, `AR-INDEX-002`, `AR-CANON-007`, and `AR-DISC-005`:**
   - Vacuous `<meta name="robots" content="">` and `<meta name="robots" content="   ">` (`HasContentAttr == true`, 0 tokens) do not emit synthetic `directive:*` bundles, preserving `len(directives) == 0` -> `NOT_APPLICABLE` in `AR-INDEX-002` and `effective_noindex = "false"` in `AR-INDEX-001` / `AR-CANON-007` / `AR-DISC-005`.
   - Missing-attribute `<meta name="robots">` (`HasContentAttr == false`) emits a `directive:*` bundle with `ParsedTokens = nil` (`directive_tokens = ""`) and `robots_parse_errors = "parse_error:missing_content_attribute"`; in `AR-INDEX-002` (`internal/audit/engine/index002.go`, lines 327–338), `directive_tokens = ""` has no `index`/`noindex` conflict and does not break `AR-INDEX-002`.
   - Non-HTML `X-Robots-Tag` processing in `processPageDirectives` emits `directive_*` and `robots_*` observations on non-HTML URLs while keeping `effective_noindex` emission gated on `isHTML` so `AR-INDEX-001` and `AR-CANON-007` behavior on non-HTML resources is 100% unchanged.

---

## 10. Updated High-Risk QA Scenario Matrix (34 Scenarios)

| # | Scenario Description | Input Evidence (`<meta>` / `X-Robots-Tag` / Channel States) | Expected `robots_parse_errors` / `UnsupportedTokens` | Expected `AR-INDEX-003` Verdict |
|---|---|---|---|---|
| **1** | **Clean 200 HTML page with zero `<meta>` robots tags and zero `X-Robots-Tag` headers** | `http_header_status="complete"`, `html_extraction_status="complete"`, `robots_meta_sample_truncated="false"`, 0 bundles | *(0 bundles)* | **`NOT_APPLICABLE`** |
| **2** | **Standard valid `<meta name="robots" content="index, follow">`** | Both channels `"complete"`, 1 meta bundle (`target="*"`, `tokens="index, follow"`) | `"none"` / `[]` | **`PASS`** |
| **3** | **All 10 active Google standalone tokens + `noarchive` in mixed case & whitespace** | `<meta name="ROBOTS" content=" ALL, None, Index, NoIndex, Follow, NoFollow, NoArchive, NoSnippet, NoTranslate, NoImageIndex, IndexIfEmbedded ">` | `"none"` / `[]` | **`PASS`** *(Valid syntax in `AR-INDEX-003`; contradiction evaluated by `AR-INDEX-002`)* |
| **4** | **Valid parameterized directives (`max-snippet`, `max-image-preview`, `max-video-preview`)** | `<meta name="robots" content="max-snippet: -1, max-image-preview: LARGE, max-video-preview: 0">` | `"none"` / `[]` | **`PASS`** |
| **5** | **Valid `unavailable_after` with ISO 8601 / RFC 3339 date** | `<meta name="robots" content="noindex, unavailable_after: 2026-12-31T23:59:59Z">` | `"none"` / `[]` | **`PASS`** |
| **6** | **Valid `unavailable_after` with comma-containing RFC 1123 / RFC 850 date** | `X-Robots-Tag: unavailable_after: Tue, 31 Dec 2026 23:59:59 GMT, noarchive` | `"none"` / `[]` | **`PASS`** |
| **7** | **`<meta name="robots" content="">` (explicit empty `content=""`, `HasContentAttr=true`) (`QA-02`)** | Clean 200 HTML; `HasContentAttr=true`, `ContentRaw=""`, no other directives | *(Vacuous declaration: 0 tokens, 0 bundles)* | **`NOT_APPLICABLE`** *(or `PASS` if paired with a valid `X-Robots-Tag: noindex`; **never** `WARNING`)* |
| **8** | **`<meta name="robots" content="   ">` (whitespace-only `content`, `HasContentAttr=true`) (`QA-02`)** | Clean 200 HTML; `HasContentAttr=true`, `ContentRaw="   "`, no other directives | *(Vacuous declaration: 0 tokens, 0 bundles)* | **`NOT_APPLICABLE`** *(or `PASS` if paired with a valid directive; **never** `WARNING`)* |
| **9** | **`<meta name="robots">` (missing `content` attribute, `HasContentAttr=false`) (`QA-02`)** | Clean 200 HTML; `HasContentAttr=false` | `"parse_error:missing_content_attribute"` / `[]` | **`WARNING`** *(Violates WHATWG HTML § 4.2.5)* |
| **10** | **Empty comma segment (`content="noindex, , nofollow"`, `content="noindex,"`, `content=","`) (`OQ-1`)** | `<meta name="robots" content="noindex, , nofollow">` | `"parse_error:empty_token_segment"` / `[]` | **`WARNING`** |
| **11** | **Deprecated/obsolete directory directives (`noodp`, `noydir`) (`QA-01`)** | `<meta name="robots" content="index, follow, noodp, noydir">` | `"unsupported_directive:obsolete:noodp, unsupported_directive:obsolete:noydir"` / `["noodp", "noydir"]` | **`WARNING`** *(Reported as `unsupported_directive`, **not** `malformed`)* |
| **12** | **Unsupported Google rules (`nopagereadaloud`, `nositelinkssearchbox`, `nocache`, `nopreview`) (`QA-01`)** | `<meta name="robots" content="noindex, nocache">` | `"unsupported_directive:nocache"` / `["nocache"]` | **`WARNING`** *(Reported as `unsupported_directive`, **not** `malformed`)* |
| **13** | **Near-miss spelling typo in standalone directive (`no-index`, `nofollows`, `no_index`, `archive`, `snippet`)** | `<meta name="robots" content="no-index, follow">` | `"unsupported_directive:no-index"` / `["no-index"]` | **`WARNING`** |
| **14** | **Semicolon or pipe delimiter instead of comma (`noindex; nofollow`, `noindex\|nofollow`)** | `<meta name="robots" content="noindex; nofollow">` | `"parse_error:invalid_delimiter:noindex; nofollow"` / `[]` | **`WARNING`** |
| **15** | **Whitespace-only separation between multiple tokens (`noindex nofollow`)** | `<meta name="robots" content="noindex nofollow">` | `"parse_error:invalid_delimiter:noindex nofollow"` / `[]` | **`WARNING`** |
| **16** | **Invalid `max-snippet` parameter (`max-snippet:abc`, `max-snippet:-2`, `max-snippet:100px`, `max-snippet:`, `max-snippet`)** | `<meta name="robots" content="max-snippet:-2">` | `"parse_error:invalid_parameter_value:max-snippet:-2"` / `[]` | **`WARNING`** |
| **17** | **Invalid `max-image-preview` parameter (`max-image-preview:huge`, `max-image-preview:true`, `max-image-preview:`)** | `<meta name="robots" content="max-image-preview:huge">` | `"parse_error:invalid_parameter_value:max-image-preview:huge"` / `[]` | **`WARNING`** |
| **18** | **Invalid `max-video-preview` parameter (`max-video-preview:none`, `max-video-preview:-5`, `max-video-preview:`)** | `<meta name="robots" content="max-video-preview:none">` | `"parse_error:invalid_parameter_value:max-video-preview:none"` / `[]` | **`WARNING`** |
| **19** | **Invalid `unavailable_after` date (`unavailable_after:2026-02-30`, `unavailable_after:not-a-date`, `unavailable_after:`)** | `<meta name="robots" content="unavailable_after:not-a-date">` | `"parse_error:invalid_parameter_value:unavailable_after:not-a-date"` / `[]` | **`WARNING`** |
| **20** | **Unexpected parameter on standalone directive (`noindex:true`, `nofollow:all`, `nosnippet:1`)** | `<meta name="robots" content="noindex:true">` | `"parse_error:unexpected_parameter:noindex:true"` / `[]` | **`WARNING`** |
| **21** | **Misspelled parameterized directive name in `X-Robots-Tag` (`max-snipet: 50`, `max_snippet: 50`, `unavailable-after: 2026-12-31`) (`QA-03`)** | `X-Robots-Tag: max-snipet: 50` | `"unsupported_directive:max-snipet: 50"` / `["max-snipet: 50"]` | **`WARNING`** |
| **22** | **User-agent prefix placed inside HTML `<meta name="robots" content="googlebot: noindex">`** | `<meta name="robots" content="googlebot: noindex">` | `"parse_error:meta_agent_prefix:googlebot: noindex"` / `[]` | **`WARNING`** |
| **23** | **Valid agent-specific `<meta name="googlebot" content="noindex, max-snippet:120">` and `<meta name="OAI-SearchBot" content="noindex">`** | Both channels `"complete"`, 2 meta bundles | `"none"` on both | **`PASS`** |
| **24** | **Multiple same-name `<meta name="googlebot">` tags where Tag 1 is malformed (`no-index`) and Tag 2 is valid (`nofollow`) (`QA-05`)** | `RawRobotsMeta` retains both in `Seq` 0 and 1 | Bundle 0: `"unsupported_directive:no-index"`<br>Bundle 1: `"none"` | **`WARNING`** |
| **25** | **Valid multi-token `X-Robots-Tag` with recognized user-agent prefix (`X-Robots-Tag: googlebot: noindex, nofollow`) (`QA-03`)** | Discrete header in `RawXRobotsTags`, both channels `"complete"` | `"none"` / `[]` | **`PASS`** |
| **26** | **Two separate `X-Robots-Tag` HTTP header fields (`X-Robots-Tag: googlebot: noindex` + `X-Robots-Tag: nofollow`) (`QA-03`)** | `RawXRobotsTags = ["googlebot: noindex", "nofollow"]`, both channels `"complete"` | Bundle 0 (`googlebot`): `"none"`<br>Bundle 1 (`*`): `"none"` | **`PASS`** |
| **27** | **Multiple agent-prefixed segments inside a single `X-Robots-Tag` header (`X-Robots-Tag: googlebot: noindex, bingbot: nofollow`) (`QA-03`)** | Single header string containing two `<agent>:` prefixes | `"ambiguous:multiple_agent_prefixes"` | **`UNKNOWN`** *(or `WARNING` if any token has a proven syntax error)* |
| **28** | **Unrecognized RFC 9110 user-agent prefix + valid V1 directive (`X-Robots-Tag: custombot: noindex`) (`QA-03`, `OQ-2`)** | `X-Robots-Tag: custombot: noindex` | `"ambiguous:unverified_agent_prefix:custombot"` | **`UNKNOWN`** |
| **29** | **Unrecognized RFC 9110 prefix + unrecognized bare token (`X-Robots-Tag: foo: bar`) vs. recognized agent + unrecognized token (`X-Robots-Tag: googlebot: bar`) (`QA-03`)** | - **29a:** `X-Robots-Tag: foo: bar`<br>- **29b:** `X-Robots-Tag: googlebot: bar`<br>- **29c:** `X-Robots-Tag: foo: noindex; nofollow` | - **29a:** `"ambiguous:unrecognized_colon_pair:foo: bar"`<br>- **29b:** `"unsupported_directive:bar"`<br>- **29c:** `"parse_error:invalid_delimiter:noindex; nofollow"` | - **29a:** **`UNKNOWN`**<br>- **29b:** **`WARNING`**<br>- **29c:** **`WARNING`** |
| **30** | **Non-HTML resource (`200 application/pdf`) with `X-Robots-Tag` (`QA-04`)**<br>- **30a:** No `X-Robots-Tag`<br>- **30b:** `X-Robots-Tag: noindex, noarchive`<br>- **30c:** `X-Robots-Tag: no-index` | `http_status=200`, `content_type="application/pdf"`, `http_header_status="complete"` | - **30a:** *(0 bundles)*<br>- **30b:** `"none"`<br>- **30c:** `"unsupported_directive:no-index"` | - **30a:** **`NOT_APPLICABLE`**<br>- **30b:** **`PASS`**<br>- **30c:** **`WARNING`** |
| **31** | **Truncated (`truncated_size_limit` / `read_error`) or Rendered (`rendered == "true"`) 200 HTML page (`QA-04`)**<br>- **31a:** Zero directives or only valid directives retained<br>- **31b:** Proven malformed/unsupported directive retained in `<meta>` or `X-Robots-Tag` | `http_header_status="complete"`, `html_extraction_status="truncated_size_limit"` (or `rendered="true"`) | - **31a:** `"none"` (or 0 bundles)<br>- **31b:** `"unsupported_directive:no-index"` | - **31a:** **`UNKNOWN`**<br>- **31b:** **`WARNING`** |
| **32** | **Redirect row (`301` / `redirect_initial_observed == "true"`), non-2xx HTML row (`404`), or failed fetch (`status == 0`) (`QA-04`)**<br>- **32a:** No proven malformed `X-Robots-Tag` on the response<br>- **32b:** `404` or `301` response carries `X-Robots-Tag: no-index` | `http_header_status="redirect_intermediate"` or `html_extraction_status="missing_legacy"` or `http_header_status="unavailable"` | - **32a:** 0 or `"none"` bundles<br>- **32b:** `"unsupported_directive:no-index"` | - **32a:** **`UNKNOWN`**<br>- **32b:** **`WARNING`** |
| **33** | **Sample cap exceeded (`> 10` `<meta>` tags `robots_meta_sample_truncated == "true"` or `> 10` `X-Robots-Tag` headers `x_robots_sample_truncated == "true"`) (`QA-05`)**<br>- **33a:** All 10 retained samples are `"none"`<br>- **33b:** Sample #4 has `noindex; nofollow` | `RobotsMetaTotalCount=12`, `len(RawRobotsMeta)=10`, `robots_meta_sample_truncated="true"` | - **33a:** `"none"` on all 10<br>- **33b:** `"parse_error:invalid_delimiter:..."` | - **33a:** **`UNKNOWN`**<br>- **33b:** **`WARNING`** |
| **34** | **Missing `robots_parse_errors` on a bundle (`MISSING != FALSE`), unreferenced directive observation, or conflicting observations (`QA-04`)** | Bundle `directive:1` has `robots_directive_tokens="noindex"` but no `robots_parse_errors` | *(missing or conflicting)* | **`UNKNOWN`** |

---

## 11. Resolution of Outstanding Contract Questions (`OQ-1`, `OQ-2`, `OQ-3`)

All three open questions from V1.10a have been explicitly resolved using primary standards and conservative V1 guardrails:

| Question ID | Question Summary | Final V1.10a.1 Resolution | Justification & Authority |
|---|---|---|---|
| **`OQ-1` (CLOSED)** | Empty comma segments (`"noindex, , nofollow"`, `"noindex,"`, `","`) vs. empty meta declarations (`content=""`, `content="   "`, and missing `content`). | 1. `<meta name="robots" content="">` and `<meta name="robots" content="   ">` (`HasContentAttr == true`): **Vacuous 0-token declarations (`NOT_APPLICABLE` if sole; `PASS` alongside valid directives; never `WARNING`)**.<br>2. `<meta name="robots">` (`HasContentAttr == false`): **`WARNING` (`parse_error:missing_content_attribute`)**.<br>3. Non-empty directive with empty comma slot (`"noindex, , nofollow"`, `"noindex,"`, `","`): **`WARNING` (`parse_error:empty_token_segment`)**. | WHATWG HTML § 4.2.5 requires `content` when `name` is present (`<meta name="robots">` without `content` is invalid HTML), while `content=""` is valid HTML with 0 tokens. Explicit commas with missing operands are malformed comma-separated lists. |
| **`OQ-2` (CLOSED)** | Unknown/custom user-agent prefixes in `X-Robots-Tag` (`X-Robots-Tag: custombot: noindex` and `X-Robots-Tag: foo: bar`). | **Return `UNKNOWN`** (`ambiguous:unverified_agent_prefix` for `custombot: noindex`; `ambiguous:unrecognized_colon_pair` for `foo: bar`) unless `<lhs>` is a known `max*`/`unavailable*`/`no*` directive typo or standalone directive with parameter (`WARNING`), or `<lhs>` is a recognized crawler with an invalid `<rhs>` (`googlebot: bar` -> `WARNING`), or `<rhs>` has a proven delimiter/parameter syntax error (`WARNING`). | `SRC-GOOGLE-ROBOTS-001` allows any crawler user-agent in `X-Robots-Tag`. A hardcoded crawler list is not a complete validity registry; returning `UNKNOWN` avoids both false `WARNING`s on custom bots and false `PASS`es on unverified prefixes. |
| **`OQ-3` (CLOSED)** | Suspected `<meta name>` typos such as `<meta name="robot" content="noindex">`. | **Out of Scope for `AR-INDEX-003` (Retracted).** Do **not** inspect `<meta name="robot">` or other non-robots `<meta name>` attributes in `AR-INDEX-003`. | Preserves the frozen `AR-INDEX-003` manifest boundary (`preconditions: meta robots or X-Robots-Tag exists`). Auditing HTML `<meta name>` attribute typos would silently expand the frozen rule's scope. |

---

## 12. V1.10b Implementation Readiness Assessment & Phased Plan

### 12.1 Readiness Assessment for Milestone V1.10b
- **Contract Completeness:** All 5 Independent QA findings (`QA-01`..`QA-05`) and all 3 open questions (`OQ-1`..`OQ-3`) are resolved with exact data structures, state machines, and a 34-scenario verification matrix.
- **Architectural Risk:** **Low.** V1.10b requires zero SQLite table DDL changes and zero changes to existing `Page.MetaTags`, `Page.MetaRobots`, or `Page.XRobotsTag` fields.
- **Recommendation:** Ready to proceed to **Milestone V1.10b** immediately upon Independent QA sign-off of this revised V1.10a.1 contract.

### 12.2 Phased Implementation Plan (Awaiting QA Approval)

1. **Milestone V1.10b — Raw Directive Acquisition & Preservation (`internal/sitecrawl/`):**
   - Add `RawMetaDirective`, `RobotsMetaExtracted bool`, `RobotsMetaTotalCount int`, `RawRobotsMeta []RawMetaDirective` (cap 10), `XRobotsTagTotalCount int`, and `RawXRobotsTags []string` (cap 10) to `sitecrawl.Page` in `internal/sitecrawl/types.go`.
   - Update `internal/sitecrawl/extract.go` (`readMeta`), `internal/sitecrawl/fetch.go` (`readTerminal`), `internal/sitecrawl/page.go` (`buildPage`), and `internal/sitecrawl/crawler.go` (`absorb`) to populate these fields losslessly while preserving existing fields.
   - Add unit tests in `internal/sitecrawl/extract_test.go` and `internal/sitecrawl/fetch_test.go`.
2. **Milestone V1.10c — Adapter Normalization & Directive Validator (`internal/audit/adapter/`):**
   - Bump `NormalizationVersion` from `"v1.10.0"` to `"v1.11.0"` in `internal/audit/adapter/adapter.go`.
   - Emit Channel 1 (`http_header_status`, `x_robots_sample_truncated`), Channel 2 (`robots_meta_sample_truncated`), and Channel 3 (`robots_parse_errors`, `directive_unsupported_tokens`) observations across both HTML and non-HTML responses.
   - Verify zero regressions on `AR-INDEX-001`, `AR-INDEX-002`, `AR-CANON-007`, and `AR-DISC-005`.
3. **Milestone V1.10d — `AR-INDEX-003` Evaluator & 34-Scenario Test Suite (`internal/audit/engine/`):**
   - Implement `evaluateINDEX003` in `internal/audit/engine/index003.go` and register `AR-INDEX-003` in `internal/audit/engine/engine.go` (`20 / 47 -> 21 / 47`).
   - Add unit and hermetic SQLite -> Adapter -> Engine tests in `internal/audit/engine/index003_test.go` covering all 34 QA scenarios.

---

## 13. Change Log Mapping (`QA-01` through `QA-05` -> Document Sections)

| QA Finding | Summary of Required Fix | Document Sections Updated in V1.10a.1 |
|---|---|---|
| **`QA-01`** | Audit every token in the V1 registry (`noarchive`, `index`, `follow`, legacy directives); separate `unsupported_directive` from `parse_error` (malformed syntax); provide full evidence matrix. | Section 1.1 (`DEC-V1.10-001`), Section 3, Section 4.1 (`4.1.1` retraction/clarification & `4.1.2` 21-row evidence matrix), Section 6.1, Section 10 (Scenarios 3, 11, 12, 13). |
| **`QA-02`** | Review `<meta name="robots" content="">`, `<meta name="robots">`, and `<meta name="robots" content="   ">` separately; do not force `WARNING` on `content=""` or `content="   "` without authority; preserve raw declaration & `HasContentAttr`. | Section 1.1 (`DEC-V1.10-003`), Section 3 (WHATWG HTML § 4.2.5), Section 4.3.1 (Cases 2A–2E table), Section 4.4.1 (`RawMetaDirective.HasContentAttr`), Section 10 (Scenarios 7, 8, 9, 10), Section 11 (`OQ-1`). |
| **`QA-03`** | Reassess `foo: bar`, `custombot: noindex`, `googlebot: noindex, nofollow`, multiple `X-Robots-Tag` headers, multiple agent prefixes, and parameterized directive typos; prefer `UNKNOWN` over speculative `WARNING`. | Section 1.1 (`DEC-V1.10-002`), Section 4.2 (`4.2.1` & `4.2.2` 6-case disambiguation matrix), Section 10 (Scenarios 21, 25, 26, 27, 28, 29a/b/c), Section 11 (`OQ-2`, `OQ-3`). |
| **`QA-04`** | Define separate evidence completeness contracts for (1) HTTP header acquisition, (2) HTML meta extraction, and (3) directive parsing/normalization; enforce `MISSING != FALSE`; address redirects, non-HTML, rendered, truncated, legacy, and conflicts. | Section 1.1 (`DEC-V1.10-003`), Section 4.3.2 (Channels 1, 2, 3 tables), Section 4.3.3 (8 response archetypes), Section 6.2 (`http_header_status`), Section 7 (Precedence P1–P7), Section 8, Section 10 (Scenarios 30–34). |
| **`QA-05`** | Specify uncapped total counts, capped raw samples, explicit sample-truncation indicators, source order/identity, verdict impact of incomplete samples, legacy fallback, and replace all machine-local absolute file URI links with portable repository-relative paths. | Section 1.1 (`DEC-V1.10-004`), Section 4.4 (`4.4.1`–`4.4.3`), Section 6.2 (`robots_meta_sample_truncated`, `x_robots_sample_truncated`), Section 9.2, Section 10 (Scenarios 24, 33a/b), and all file references throughout the document. |
