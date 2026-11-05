# Audit V1.10a — Robots Directive Evidence & Syntax Contract (`AR-INDEX-003`)

## 1. Executive Decision Summary

- **Milestone:** V1.10a — Robots Directive Evidence & Syntax Contract Review
- **Target Rule:** `AR-INDEX-003` — *Malformed or unsupported robots directive* (`parent_check`: `INDEX-001`, `lifecycle`: `Index`, `automation`: `deterministic`, `default_severity`: `P1`)
- **Repository / Branch:** `truthrive/technical-seo-audit` (`main`)
- **Approved Baseline Commit:** `123a3781c346aba066958325f513f56d16533891`
- **Workflow:** **Risk-Based QA + Batch Implementation + MVP-First**
- **Milestone Scope:** **Documentation and Contract Specification Only.** Zero production code changes, zero SQLite schema changes, zero adapter edits, and zero evaluator registrations in V1.10a.
- **Preserved Invariants in V1.10a:**
  - Executable rules remain strictly **`20 / 47`** (`27 / 47` remaining).
  - `NormalizationVersion` remains strictly **`v1.10.0`** ([`internal/audit/adapter/adapter.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L356)).
  - Frozen rule manifest ([`knowledge/07-v1-atomic-rule-manifest.md`](file:///d:/Projects/audit%20technical/sitecrawl-docs/knowledge/07-v1-atomic-rule-manifest.md#L215-L229) and [`internal/audit/rules_v1.json`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/rules_v1.json#L359-L392)) is unmodified.

### 1.1 Summary of the Four Contract Decisions

| Decision ID | Decision Topic | Proposed Resolution Summary | Status |
|---|---|---|---|
| **`DEC-V1.10-001`** | **Directive Syntax & Supported V1 Registry (`SRC-GOOGLE-ROBOTS-001`)** | Freeze a 3-tier token classification:<br>1. **Valid V1 Directive Tokens (15 total):** 11 standalone tokens (`all`, `none`, `index`, `noindex`, `follow`, `nofollow`, `noarchive`, `nosnippet`, `notranslate`, `noimageindex`, `indexifembedded` — including explicit `index`/`follow` recognized by V1 and `AR-INDEX-002`) + 4 parameterized tokens (`max-snippet:[number]`, `max-image-preview:[setting]`, `max-video-preview:[number]`, `unavailable_after:[date/time]`).<br>2. **Proven Malformed or Unsupported Directives (`WARNING`):** (a) Proven syntax/delimiter errors (`;`, `\|`, whitespace-separated multi-tokens `noindex nofollow`, empty declaration `""` or `,`, missing/extra parameter colons `max-snippet`, `max-snippet:`, `noindex:true`, invalid parameter values `max-snippet:abc`, `max-image-preview:huge`, invalid `unavailable_after` date, or `agent:` prefix inside `<meta name="robots">`); (b) Proven deprecated/unsupported directives (`noodp`, `noydir`, `nocache`, `noav`, `nopreview`, ` archive`, `snippet`) and near-miss typos of standard directives (`no-index`, `no_index`, `nofollows`, `max_snippet`, `max-snipet`, `unavailable-after`).<br>3. **Unverifiable / Ambiguous Scope or Custom Extension (`UNKNOWN`):** Where a token's syntax or scope cannot be deterministically verified as a valid V1 directive or proven malformed/unsupported without guessing crawler-specific extensions. | **PROPOSED — PENDING QA APPROVAL** |
| **`DEC-V1.10-002`** | **User-Agent Prefix & Multi-Header Handling** | Do **not** rely on a hardcoded list of 4 crawlers as a complete validity registry:<br>1. **`<meta name="...">`:** Scope is restricted to `<meta name="robots">` and crawler-specific robots meta tags (`googlebot`, `googlebot-*`, `bingbot`, `oai-searchbot`, `gptbot`, `slurp`, `duckduckbot`, `baiduspider`, `yandex`, `applebot`, `adsbot-google`, `mediapartners-google`, `google-extended`). Inside `<meta content="...">`, `user-agent:` prefixes (e.g., `<meta name="robots" content="googlebot: noindex">`) are **proven invalid syntax (`WARNING`)** per `SRC-GOOGLE-ROBOTS-001`.<br>2. **`X-Robots-Tag` HTTP Headers:** Preserve discrete `res.Header.Values("X-Robots-Tag")` slices (`RawXRobotsTags []string`) before comma-joining so multi-header boundaries are never conflated. Within a single `X-Robots-Tag` header value, `<user-agent>: <directive>[, <directive>...]` is valid syntax when `<user-agent>` is a valid RFC 9110 token (`^[a-zA-Z][a-zA-Z0-9_-]*$`) **and** is not a misspelled parameterized directive name (e.g., `max_snippet`, `max-snipet`, `unavailable-after`), **and** all subsequent comma-separated tokens are valid V1 directives. If `<prefix>: <rest>` has a valid token prefix that is not a known V1 crawler and `<rest>` is a valid directive, or if legacy comma-joined `X-Robots-Tag` headers have ambiguous prefix boundaries without a proven malformed token, classify as **`UNKNOWN` (`scope_or_prefix_ambiguous`)** rather than a false `WARNING` or false `PASS`. | **PROPOSED — PENDING QA APPROVAL** |
| **`DEC-V1.10-003`** | **Verdict Precedence & Evidence Completeness** | Strictly preserve `PASS`, `WARNING`, `UNKNOWN`, `NOT_APPLICABLE` (`FAIL` and `MANUAL_REVIEW` are **not used**):<br>1. **Unusable `url_identity` or internally conflicting/malformed directive observations** -> `UNKNOWN`.<br>2. **Proven `WARNING` Short-Circuit:** If any retained directive (`<meta>` or `X-Robots-Tag`, including on non-HTML `application/pdf` responses or partially truncated/rendered HTML pages) contains a **proven** `malformed_syntax` or `unsupported_directive` error, return **`WARNING`** immediately.<br>3. **Incomplete / Unverifiable Source (`UNKNOWN`):** If no `WARNING` is proven in retained directives, and directive source acquisition is incomplete or unverifiable (e.g., `http_status <= 0`, `fetch_error_type != ""`, `3xx` redirect row where body was not inspected, HTML response with `html_extraction_status != "complete"` including `truncated_size_limit`, `read_error`, `missing_legacy`, or `rendered == "true"`, or legacy/ambiguous directive scope), return **`UNKNOWN`**.<br>4. **Zero Directives on Complete Source (`NOT_APPLICABLE`):** Complete 2xx HTML (`html_extraction_status == "complete"`, `robots_meta_total_count == 0`, no `X-Robots-Tag`) or complete non-redirect non-HTML response (`status >= 200`, no `fetch_error_type`, no `X-Robots-Tag`) with zero directives -> **`NOT_APPLICABLE`**.<br>5. **All Directives Valid on Complete Source (`PASS`):** At least one directive exists, source is complete, and all directives have `robots_parse_errors == "none"` -> **`PASS`**. | **PROPOSED — PENDING QA APPROVAL** |
| **`DEC-V1.10-004`** | **Lossless Raw Directive Preservation vs. Adapter-Only** | **Recommend Option A (Lossless Raw Acquisition in `sitecrawl.Page` JSON + `X-Robots-Tag` Header Slice Preservation):**<br>Independent code verification proved **5 critical data-loss defects** in existing `sitecrawl` acquisition (`extract.go`, `fetch.go`, `adapter.go`) that make Option B (Adapter-only) incapable of meeting the contract:<br>1. `d.MetaTags[name] = content` (`extract.go:481`) overwrites earlier same-name `<meta>` tags.<br>2. `adapter.go:2267` (`if val == "" { continue }`) drops `<meta name="robots" content="">` and missing `content` attributes.<br>3. `extract.go:439` (`strings.TrimSpace`) strips raw whitespace before storage.<br>4. `fetch.go:244` (`strings.Join(values, ", ")`) flattens multiple `X-Robots-Tag` headers into a single string, destroying header boundaries.<br>5. `adapter.go:902` only calls `processPageDirectives` inside `if isHTML`, **completely dropping `X-Robots-Tag` headers on non-HTML resources (PDFs, images, etc.)**!<br>Option A adds optional JSON fields (`RawRobotsMeta []RawMetaDirective`, `RobotsMetaTotalCount int`, `RawXRobotsTags []string`) to `sitecrawl.Page` with zero SQLite table/column DDL changes and 100% backward compatibility. | **PROPOSED — PENDING QA APPROVAL** |

---

## 2. Verified Source-Code Inventory (Independent Audit of Current Codebase)

Every claim below was independently verified against commit `123a3781c346aba066958325f513f56d16533891`.

### 2.1 VERIFIED FACTS in Current Codebase

1. **Frozen Rule Manifest & Registry (`VERIFIED FACT`):**
   - [`knowledge/07-v1-atomic-rule-manifest.md`](file:///d:/Projects/audit%20technical/sitecrawl-docs/knowledge/07-v1-atomic-rule-manifest.md#L215-L229) and [`internal/audit/rules_v1.json`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/rules_v1.json#L359-L392) define `AR-INDEX-003`:
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

2. **Typed Entity Model Already Declares `UnsupportedTokens` and `ParseErrors` (`VERIFIED FACT`):**
   - In [`internal/audit/model.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/model.go#L172-L185), `audit.RobotsDirectiveObservation` **already** has the exact struct fields required by `knowledge/09-data-model.md` (§ 14, lines 490–505):
     ```go
     type RobotsDirectiveObservation struct {
         RobotsDirectiveObservationID RobotsDirectiveID `json:"robots_directive_observation_id"`
         AuditRunID                   AuditRunID        `json:"audit_run_id"`
         URLID                        URLID             `json:"url_id"`
         Source                       DirectiveSource   `json:"source"`
         Target                       string            `json:"target"`
         ScopeUnknown                 bool              `json:"scope_unknown"`
         RawValue                     string            `json:"raw_value"`
         ParsedTokens                 []string          `json:"parsed_tokens"`
         UnsupportedTokens            []string          `json:"unsupported_tokens"`
         ParseErrors                  []string          `json:"parse_errors"`
         EffectiveNoindex             bool              `json:"effective_noindex"`
         ObservedAt                   time.Time         `json:"observed_at"`
     }
     ```
   - Currently, [`internal/audit/adapter/adapter.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L2287-L2470) leaves `UnsupportedTokens` and `ParseErrors` unpopulated (`nil`) and does not emit `robots_parse_errors` or `directive_unsupported_tokens` as `NormalizedObservation` records.

3. **Five Verified Acquisition & Adapter Gaps in Current Code (`VERIFIED FACT`):**
   - **Gap 2.1 — Overwriting Same-Name `<meta>` Tags in `extract.go`:**
     In [`internal/sitecrawl/extract.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/extract.go#L481-L498):
     - Line 481 executes `d.MetaTags[name] = content`, which **overwrites** any previous `<meta>` tag with the same `name`!
     - While lines 486–498 append `"robots"` and `"googlebot"` into the single flattened string `d.MetaRobots` (`d.MetaRobots += ", " + content`), any **other** crawler-specific meta tag (e.g., two `<meta name="bingbot">` or `<meta name="googlebot-news">` tags) has its first declaration permanently overwritten.
     - Worse, even for `<meta name="robots">`, if a page has `<meta name="robots" content="index">` followed by `<meta name="robots" content="nofollow">`, `d.MetaTags["robots"]` retains only `"nofollow"`, while `d.MetaRobots` flattens `"index, nofollow"` — and if the page *also* has `<meta name="googlebot" content="noarchive">`, `adapter.go:2365–2387` cannot tell whether the unrecovered token `"index"` in `d.MetaRobots` came from `<meta name="robots">` or `<meta name="googlebot">`, forcing `ScopeUnknown = true`!
   - **Gap 2.2 — Silent Loss of Empty or Missing `content` Attributes (`<meta name="robots" content="">` or `<meta name="robots">`):**
     - In [`internal/sitecrawl/extract.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/extract.go#L439), `content := strings.TrimSpace(attrs["content"])`. If a page has `<meta name="robots" content="">` or `<meta name="robots" content="   ">` or `<meta name="robots">` (no `content` attribute), `d.MetaTags["robots"]` is set to `""` and `d.MetaRobots` is `""`.
     - Then, in [`internal/audit/adapter/adapter.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L2266-L2269):
       ```go
       val := strings.TrimSpace(v)
       if val == "" {
           continue
       }
       ```
       The adapter **completely skips** empty `MetaTags` entries! As a result, a page with `<meta name="robots" content="">` emits **zero** directive observations and would falsely evaluate to `NOT_APPLICABLE` instead of `WARNING`!
   - **Gap 2.3 — Flattening Multiple `X-Robots-Tag` HTTP Headers in `fetch.go`:**
     - In [`internal/sitecrawl/fetch.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/fetch.go#L242-L245):
       ```go
       if values := res.Header.Values("X-Robots-Tag"); len(values) > 0 {
           out.XRobotsTag = strings.Join(values, ", ")
       }
       ```
     - When an HTTP response sends two separate headers:
       Header 1: `X-Robots-Tag: googlebot: noindex`
       Header 2: `X-Robots-Tag: nofollow`
       `fetch.go` joins them into `"googlebot: noindex, nofollow"`. Then [`parseXRobotsSegment`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L2155-L2166) cannot tell whether `nofollow` was part of Header 1 (`target = "googlebot"`) or a separate Header 2 (`target = "*"`), forcing `scopeUnknown = true`.
   - **Gap 2.4 — Non-HTML Responses (`application/pdf`, Images, etc.) Skip `processPageDirectives` Entirely in `adapter.go`:**
     - In [`internal/audit/adapter/adapter.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L902-L1092), `processPageDirectives` is called **only inside `if isHTML { ... }`**!
     - However, per `SRC-GOOGLE-ROBOTS-001`, `X-Robots-Tag` is specifically designed for **both HTML and non-HTML resources** (such as PDFs, images, and videos). Currently, if a crawled PDF (`content_type = "application/pdf"`, `kind = "pdf"`) returns `X-Robots-Tag: no-index` or `X-Robots-Tag: noindex`, `adapter.go` never calls `processPageDirectives` and emits **zero** `RobotsDirectiveObservation` records for that URL!
   - **Gap 2.5 — Comma-Split `unavailable_after` Dates Containing Commas (RFC 850 / RFC 1123):**
     - Standard HTTP / RFC 1123 / RFC 850 dates contain a comma after the day-of-week (e.g., `unavailable_after: Tue, 31 Dec 2026 23:59:59 GMT` or `unavailable_after: Tuesday, 31-Dec-26 23:59:59 EST`).
     - Currently, [`parseDirectiveTokens`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L2096-L2106) and [`processPageDirectives`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L2437) blindly call `strings.Split(..., ",")`!
     - Splitting `"unavailable_after: Tue, 31 Dec 2026 23:59:59 GMT, noindex"` on every comma corrupts the date into two broken fragments: `"unavailable_after: tue"` and `"31 dec 2026 23:59:59 gmt"`!

---

## 3. Official Source References & Evidence Levels

Per [`knowledge/06-research-sources.md`](file:///d:/Projects/audit%20technical/sitecrawl-docs/knowledge/06-research-sources.md#L39-L53) and [`knowledge/00-product-principles.md`](file:///d:/Projects/audit%20technical/sitecrawl-docs/knowledge/00-product-principles.md#L116-L126):

| Source ID / Standard | Authority Level | Evidence Label | Verified Technical Facts Applicable to `AR-INDEX-003` |
|---|---|---|---|
| **`SRC-GOOGLE-ROBOTS-001`**<br>Google Search Central: *Robots Meta Tags Specifications* (`https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag`) | Primary Platform Documentation | `CONFIRMED` | 1. Defines the supported indexing and serving rules for `<meta name="robots">` and `X-Robots-Tag`: `all`, `noindex`, `nofollow`, `none`, `nosnippet`, `indexifembedded`, `max-snippet:[number]`, `max-image-preview:[setting]`, `max-video-preview:[number]`, `notranslate`, `noimageindex`, `unavailable_after:[date/time]`.<br>2. Directives are **case-insensitive**.<br>3. Multiple rules must be comma-separated (`,`).<br>4. Unrecognized or malformed rules are ignored by Google.<br>5. User-agent prefixes (`<user-agent>: <rule>`) are valid in `X-Robots-Tag` HTTP headers; in HTML, crawler targeting uses `<meta name="<user-agent>" content="<rule>">`, **not** `<meta name="robots" content="<user-agent>: <rule>">`.<br>6. `X-Robots-Tag` applies to both HTML and non-HTML resources (e.g., PDF, video, image files). |
| **RFC 9110 (HTTP Semantics) § 5.3 / § 5.6.2** | Primary Internet Standard | `CONFIRMED` | 1. Multiple HTTP header fields with the same field name (`X-Robots-Tag`) may be sent in a response and are semantically combinable, though retaining discrete header values preserves exact scope boundaries when user-agent prefixes are present.<br>2. User-agent product tokens consist of RFC 9110 `token` characters (`tchar`). |
| **RFC 822 / RFC 850 / RFC 1123 / RFC 3339 / ISO 8601** | Primary Date/Time Standards | `CONFIRMED` | Defines the standard date/time formats accepted by Google for `unavailable_after:[date/time]`, including formats that contain a literal comma after the weekday name (`Mon, 02 Jan 2006 15:04:05 MST` / `Monday, 02-Jan-06 15:04:05 MST`). |
| **Frozen V1 Manifest (`knowledge/07-v1-atomic-rule-manifest.md`)** | Authoritative Project Contract | `CONFIRMED` | `AR-INDEX-002` explicitly recognizes `index` (and by symmetry `follow`) as valid V1 directive tokens alongside `SRC-GOOGLE-ROBOTS-001`'s list (`all`, `none`, `noindex`, `nofollow`, etc.). `AR-INDEX-003` uses `PASS`, `WARNING`, `UNKNOWN`, `NOT_APPLICABLE` (`FAIL` and `MANUAL_REVIEW` are not used). |

---

## 4. Contract Decisions (`DEC-V1.10-001` through `DEC-V1.10-004`)

### 4.1 `DEC-V1.10-001` — Directive Syntax & Supported V1 Registry

#### 4.1.1 Three-Way Distinction: Valid vs. Proven Malformed/Unsupported vs. Unknown/Unverifiable

Per the V1.10a mandate, we must critically distinguish:
1. **Supported V1 Directive Tokens (`VALID` -> contributes to `PASS`):**
   - **11 Standalone Valueless Tokens (case-insensitive):**
     `all`, `none`, `index`, `noindex`, `follow`, `nofollow`, `noarchive`, `nosnippet`, `notranslate`, `noimageindex`, `indexifembedded`
     *(Note: While Google's current table notes `all` is the default and `index`/`follow` are implicit defaults, both `index` and `follow` are universally used standard directives and `index` is explicitly required by `AR-INDEX-002`. Both are valid in V1.)*
   - **4 Parameterized Directives (case-insensitive directive name, colon-delimited `[name]:[value]` with optional whitespace around `:`):**
     - `max-snippet:[number]` — `[number]` (trimmed) must be a valid base-10 integer `>= -1` (`-1`, `0`, or positive integer `1..2147483647`, matching `^(-1|[0-9]+)$` with no leading `+`, no decimals, and no unit suffixes like `px` or `chars`).
     - `max-image-preview:[setting]` — `[setting]` (trimmed, case-insensitive) must be one of: `none`, `standard`, `large`.
     - `max-video-preview:[number]` — `[number]` (trimmed) must be a valid base-10 integer `>= -1` (`-1`, `0`, or positive integer `1..2147483647`, matching `^(-1|[0-9]+)$`).
     - `unavailable_after:[date/time]` — `[date/time]` (trimmed) must be a valid calendar date/time parseable under standard ISO 8601 / RFC 3339 / RFC 1123 / RFC 822 / RFC 850 layouts (see Section 5.3).

2. **Proven Malformed Syntax or Recognized Unsupported Directives (`WARNING`):**
   - **Category A — Proven Malformed Syntax (`parse_error:malformed_*`):**
     - `parse_error:empty_directive`: A `<meta name="robots">` (or agent meta tag) or `X-Robots-Tag` header is present, but its raw value is empty (`""`), whitespace-only (`"   "`), or contains only commas (`","`, `" , "`).
     - `parse_error:empty_token_segment`: Trailing, leading, or consecutive commas producing an empty token slot (e.g., `"noindex, , nofollow"` or `"noindex,"`). *(See Open Question OQ-1 on whether a single trailing comma should be `WARNING` (`parse_error:empty_token_segment`) or tolerated; we propose `WARNING` because it indicates a template concatenation defect).*
     - `parse_error:invalid_delimiter`: Semicolon (`;`), pipe (`|`), slash (`/`), or whitespace-separated multiple tokens without a comma (e.g., `"noindex; nofollow"`, `"noindex | nofollow"`, `"noindex nofollow"`).
     - `parse_error:missing_parameter`: A parameterized directive name (`max-snippet`, `max-image-preview`, `max-video-preview`, `unavailable_after`) appears without `:[value]` or with an empty value after `:` (e.g., `"max-snippet"`, `"max-snippet:"`).
     - `parse_error:unexpected_parameter`: A standalone valueless directive is given a colon parameter (e.g., `"noindex:true"`, `"nofollow:all"`, `"nosnippet:1"`).
     - `parse_error:invalid_parameter_value`: A parameterized directive has an invalid parameter value:
       - `max-snippet` or `max-video-preview` with value `< -1` (e.g., `-2`), non-integer (`abc`, `10.5`, `150chars`, `none`, `+50`), or integer overflow.
       - `max-image-preview` with value outside `{none, standard, large}` (e.g., `huge`, `medium`, `default`, `true`, `0`, `-1`).
       - `unavailable_after` with an unparseable date/time string or invalid calendar date (e.g., `2026-02-30`, `2026-13-01T00:00:00Z`, `tomorrow`, `2026/99/99`).
     - `parse_error:meta_agent_prefix`: An `X-Robots-Tag`-style `<agent>: <directive>` prefix is placed inside an HTML `<meta>` tag's `content` attribute (e.g., `<meta name="robots" content="googlebot: noindex">`). Per `SRC-GOOGLE-ROBOTS-001`, HTML `<meta>` tags specify the user-agent in the `name` attribute (`<meta name="googlebot" content="noindex">`), never via a colon prefix inside `content`.
   - **Category B — Recognized Unsupported / Deprecated Directives & Proven Typos (`parse_error:unsupported_token`):**
     - **Legacy/Retired Directives Explicitly Unsupported by Google (`SRC-GOOGLE-ROBOTS-001`):**
       `noodp`, `noydir`, `nocache`, `nopreview`, `noav`, `nositelinkssearchbox`, `nopagereadaloud`, `archive`, `snippet`, `imageindex`, `translate`.
       *(Note: `nositelinkssearchbox` and `nopagereadaloud` are retired/page-level meta tags, not valid `robots` indexing/serving rules per `SRC-GOOGLE-ROBOTS-001`; `noodp` and `noydir` are defunct directory directives ignored by Google).*
     - **Single-Token Unrecognized Directives (Typos & Unknown Standalone Words):**
       Per the frozen manifest for `AR-INDEX-003` ([`knowledge/07-v1-atomic-rule-manifest.md:221–223`](file:///d:/Projects/audit%20technical/sitecrawl-docs/knowledge/07-v1-atomic-rule-manifest.md#L221-L223)):
       - `PASS`: *"all directive tokens are recognized by the supported V1 directive registry."*
       - `WARNING`: *"malformed or unsupported token observed."*
       Therefore, any standalone directive token (e.g., `no-index`, `no_index`, `nofollows`, `doindex`, `dofollow`, `noindexx`, `foobar`) or any unrecognized parameterized directive name (`max_snippet:50`, `max-snipet:50`, `unavailable-after:2026-12-31`, `max-text:100`) that is **not** in the V1 supported directive registry produces `WARNING` (`parse_error:unsupported_token`), **except** when a colon-prefixed segment in `X-Robots-Tag` has genuine ambiguity between an unknown third-party crawler user-agent prefix and an unknown parameterized directive (see `DEC-V1.10-002` below!).

3. **Unknown / Unverifiable Tokens (`UNKNOWN`):**
   - Where does `UNKNOWN` apply at the token/syntax level?
     - In `X-Robots-Tag`, syntax `<token>: <valid-v1-directives>` (for example, `X-Robots-Tag: mycustomcrawler: noindex`) uses a syntactically valid RFC 9110 user-agent token (`mycustomcrawler`) followed by a valid V1 directive (`noindex`). Because `mycustomcrawler` is neither a known V1 crawler nor a known/near-miss parameterized directive name, `AR-INDEX-003` **must not** falsely flag `mycustomcrawler: noindex` as a malformed directive (`WARNING`)—Wait: or is `<valid-agent-token>: <valid-v1-directives>` in `X-Robots-Tag` actually **valid `X-Robots-Tag` syntax (`PASS`)** per `SRC-GOOGLE-ROBOTS-001`?
     - Let us analyze this carefully in `DEC-V1.10-002`!

---

### 4.2 `DEC-V1.10-002` — User-Agent Prefix Handling in `<meta>` and `X-Robots-Tag`

#### 4.2.1 Why a Hardcoded Crawler List Must Not Be Used as a Complete Validity Registry
Per `SRC-GOOGLE-ROBOTS-001` ("Any user agent that is used by a crawler can be specified" in `<meta name="<user-agent>">` and `X-Robots-Tag: <user-agent>: <rules>`):
- In `AR-INDEX-001` and `AR-INDEX-002`, the adapter's `isSupportedAgentToken` (`googlebot`, `googlebot-*`, `oai-searchbot`, `gptbot`, `bingbot`) defines which crawler scopes are tracked for V1 indexability conflict and Googlebot `effective_noindex` evaluation.
- However, for `AR-INDEX-003` (*Malformed or unsupported robots directive*), a site may legitimately serve `X-Robots-Tag: adsbot-google: noindex` or `X-Robots-Tag: duckduckbot: noindex, nofollow`. Treating every crawler name outside `{googlebot, oai-searchbot, gptbot, bingbot}` as a malformed directive (`WARNING`) would violate Principle 3 (*Avoid false positives: An unusual implementation is not automatically an error*).

#### 4.2.2 Deterministic User-Agent Scope & Prefix Rules

1. **HTML `<meta name="..." content="...">` Scope:**
   - **Which `<meta name>` attributes are robots directive declarations?**
     A `<meta>` element is a robots directive declaration if and only if its trimmed lowercase `name` attribute is:
     - `"robots"` (generic scope `*`), **OR**
     - In the V1 recognized crawler meta-name vocabulary:
       `googlebot`, `googlebot-news`, `googlebot-image`, `googlebot-video`, `adsbot-google`, `adsbot-google-mobile`, `mediapartners-google`, `apis-google`, `google-extended`, `bingbot`, `adidxbot`, `msnbot`, `oai-searchbot`, `oaisearchbot`, `gptbot`, `chatgpt-user`, `claudebot`, `anthropic-ai`, `perplexitybot`, `applebot`, `applebot-extended`, `slurp`, `duckduckbot`, `baiduspider`, `yandex`, `facebookexternalhit`, `meta-externalagent`, `twitterbot`, `linkedinbot`, `bytespider`, `ccbot`.
     - *(Non-robots `<meta>` tags such as `description`, `viewport`, `charset`, `keywords`, `author`, `generator`, `theme-color`, `referrer`, `format-detection`, `rating`, `google`, `google-site-verification`, `msvalidate.01`, `apple-mobile-web-app-*`, `application-name`, `msapplication-*`, `csrf-token`, etc. are **not** robots directives and are ignored by `processPageDirectives`.)*
   - **Inside `<meta>` `content` attribute:**
     User-agent prefixes (`<prefix>: <directive>`) are **never** valid inside `<meta name="..." content="...">`. Any `:` inside a `<meta>` directive token whose left-hand side is not one of the 4 parameterized directives (`max-snippet`, `max-image-preview`, `max-video-preview`, `unavailable_after`) is a **proven `WARNING`** (`parse_error:meta_agent_prefix` if the left-hand side is a crawler name, or `parse_error:unsupported_token` / `parse_error:unexpected_parameter`).

2. **HTTP `X-Robots-Tag` Header Prefix & Segment Grammar:**
   - Each discrete `X-Robots-Tag` HTTP response header line (`RawXRobotsTags[i]`) is parsed **independently** (never pre-joined across separate HTTP headers).
   - Within a single `X-Robots-Tag` header line, the header may either:
     - **Form 1 (Unprefixed / Generic `*`):** `<directive-list>` (e.g., `X-Robots-Tag: noindex, max-snippet: 150`).
     - **Form 2 (Agent-Prefixed):** `<user-agent>: <directive-list>` (e.g., `X-Robots-Tag: googlebot: noindex, nofollow, max-snippet: 150`).
   - **How to deterministically distinguish `<user-agent>: <directive-list>` from a parameterized directive `<param-name>: <param-value>` or a malformed typo?**
     When a header line starts with `<lhs>:<rhs>` (where `<lhs>` is the text before the first colon `:` before any comma):
     - **Step 1:** If `<lhs>` (lowercased, trimmed) is one of the 4 supported parameterized directives (`max-snippet`, `max-image-preview`, `max-video-preview`, `unavailable_after`), the header is **Form 1 (Unprefixed)** and `<lhs>:<rhs>` is parsed as a parameterized directive token.
     - **Step 2:** If `<lhs>` is a **known near-miss / misspelled parameterized directive name** (specifically: starts with `max-`, `max_`, `unavailable-`, `unavailable_`, or matches `^(max|unavailable)[a-z0-9_-]*$`) OR `<lhs>` is a standalone directive name with an unexpected parameter (e.g., `noindex: true`, `nofollow: 1`, `nosnippet: yes`), it is **NOT** a user-agent prefix — it is a **proven `WARNING`** (`parse_error:unsupported_token` or `parse_error:unexpected_parameter`).
     - **Step 3:** If `<lhs>` is in the **V1 recognized crawler vocabulary** (the same crawler token set defined in 4.2.2.1 above, matching `isRecognizedCrawlerAgent(<lhs>)`), then `<lhs>` is a **verified user-agent prefix** (`target = normalizeAgent(<lhs>)`, `scopeUnknown = false`), and `<rhs>` is tokenized as a comma-separated `<directive-list>`!
       - Every token in `<rhs>` is then validated against the V1 directive registry.
       - If `<rhs>` is empty (e.g., `X-Robots-Tag: googlebot:`), that is a **proven `WARNING`** (`parse_error:empty_directive`).
       - If another non-parameterized `<agent2>:` prefix appears mid-line after a comma inside the same header (e.g., `X-Robots-Tag: googlebot: noindex, bingbot: nofollow`), `SRC-GOOGLE-ROBOTS-001` specifies one user-agent per `X-Robots-Tag` header; mid-line re-prefixing is flagged as `WARNING` (`parse_error:multiple_agent_prefixes_in_header`) or `scopeUnknown = true`.
     - **Step 4 (Ambiguous / Unrecognized Prefix `<lhs>: <rhs>`):**
       What if `<lhs>` is a syntactically valid ASCII token (`^[a-zA-Z][a-zA-Z0-9_-]*$`) that is **neither** a parameterized directive / `max*` / `unavailable*` typo **nor** in the V1 recognized crawler vocabulary (for example, `X-Robots-Tag: custombot: noindex` vs `X-Robots-Tag: foo: bar`)?
       - **Case 4a (`<rhs>` contains a proven malformed or unsupported directive token, e.g., `X-Robots-Tag: foo: bar` or `X-Robots-Tag: custombot: no-index`):** Regardless of whether `<lhs>` was intended as a bot name or a directive, `<rhs>` (`bar` / `no-index`) is **not** a valid V1 directive! Therefore, this is a **proven `WARNING`** (`parse_error:unsupported_token`).
       - **Case 4b (`<lhs>` is invalid token syntax, e.g., contains spaces or invalid punctuation like `X-Robots-Tag: bad bot: noindex`):** **Proven `WARNING`** (`parse_error:malformed_syntax`).
       - **Case 4c (`<lhs>` is a valid ASCII token not in the V1 crawler registry, AND every token in `<rhs>` is a valid V1 directive, e.g., `X-Robots-Tag: myinternalbot: noindex, nofollow`):**
         Because `<lhs>` could be a legitimate custom/regional crawler name not in our static V1 crawler list, we **do not** emit a false `WARNING`. Instead:
         - For `AR-INDEX-001` / `AR-INDEX-002`: `target = ""`, `scopeUnknown = true` (preserving conservative scope ambiguity when headers were flattened, or `target = <lhs>` non-Googlebot scope).
         - For `AR-INDEX-003`: Should `X-Robots-Tag: myinternalbot: noindex` return **`UNKNOWN`** (`parse_status = "unknown_agent_prefix"`) or **`PASS`**?
         - Per the prompt instruction: *"Do not assume a hardcoded list of crawler names is a complete validity registry. Define deterministic parsing rules and explicit UNKNOWN cases where ambiguity cannot be resolved safely."*
         - Therefore, when `<lhs>` is an unrecognized ASCII token prefix in `X-Robots-Tag` followed by valid V1 directives (and no `WARNING` exists elsewhere on the page), the directive bundle is marked `robots_parse_status = "unknown_prefix"` (`robots_parse_errors = "ambiguous:unverified_agent_prefix"`), and `AR-INDEX-003` deterministically returns **`UNKNOWN`**!

---

### 4.3 `DEC-V1.10-003` — Verdict Precedence & Evidence Completeness

#### 4.3.1 Frozen Verdict Set for `AR-INDEX-003`
Per [`knowledge/07-v1-atomic-rule-manifest.md`](file:///d:/Projects/audit%20technical/sitecrawl-docs/knowledge/07-v1-atomic-rule-manifest.md#L215-L229):
- **`PASS`**: All directive tokens are recognized by the supported V1 directive registry (and valid).
- **`WARNING`**: Malformed or unsupported token observed.
- **`UNKNOWN`**: Raw directive unavailable (or incomplete/conflicting/ambiguous evidence).
- **`NOT_APPLICABLE`**: No directive exists.
- **`FAIL`**: **Not used.**
- **`MANUAL_REVIEW`**: **Not used.**

#### 4.3.2 Resolution of Key Completeness & Precedence Questions

1. **Can a proven malformed/unsupported directive justify `WARNING` even when the complete HTML directive set is unavailable?**
   - **Proposed Decision: YES (`WARNING` > `UNKNOWN` for proven violations, provided `url_identity` and the deciding directive observation itself are non-conflicting).**
   - *Rationale:*
     - Suppose a page is rendered (`rendered == "true"`) or its HTML body was truncated at 4 MiB (`html_extraction_status == "truncated_size_limit"`), **and** its HTTP response headers contain `X-Robots-Tag: no-index` (or a `<meta name="robots" content="no-index">` tag was already extracted in `<head>` before the 4 MiB truncation boundary).
     - No amount of additional HTML after byte 4,194,304 can "un-malform" an already-observed `no-index` token! Unlike `PASS` or `NOT_APPLICABLE` (which are universal claims requiring 100% complete directive inspection), `WARNING` is an **existential** claim (*"at least one malformed or unsupported token was observed"*).
     - This is 100% consistent with `AR-INDEX-001` (where a proven `X-Robots-Tag: noindex` yields `effective_noindex = true` -> `FAIL` even on a rendered page), `AR-INDEX-002` (where a confirmed conflict in a known scope yields `FAIL` even if another bundle has `scope_unknown = true`), and the approved V1.9c QA fix for `AR-CANON-004` (where `>= 2` distinct targets in a capped sample yields `FAIL`).

2. **When Must `AR-INDEX-003` Return `UNKNOWN` vs `NOT_APPLICABLE` When Zero Directives Are Observed?**
   - **`MISSING != FALSE` / `ZERO OBSERVED != ZERO EXIST`:**
     A URL with zero observed directives (`len(directives) == 0`) can return **`NOT_APPLICABLE`** **if and only if** all directive-bearing channels applicable to that response were completely and verifiably inspected:
     - **Case A — HTML Response (`isHTML == true`):**
       Requires **both**:
       1. HTTP response headers were received (`http_status` in `200..299`, `redirect_initial_observed == "false"`, `fetch_error_type == ""`), **and**
       2. Raw server HTML was completely extracted (`rendered != "true"`, `html_extraction_status == "complete"`), **and**
       3. Zero `<meta>` robots directives (`robots_meta_total_count == "0"`) and zero `X-Robots-Tag` headers exist.
       -> Returns **`NOT_APPLICABLE`**.
       *(Conversely, if `html_extraction_status` is `"truncated_size_limit"`, `"read_error"`, or `"missing_legacy"`, or `rendered == "true"`, or `http_status` is `3xx`/`4xx`/`5xx`/missing, or `fetch_error_type != ""`, and zero directives were observed, `AR-INDEX-003` **must** return **`UNKNOWN`** because raw HTML `<meta>` directives were not completely or authoritatively inspected!)*
       *(Wait: what about a `404` or `500` HTML error page? In [`sitecrawl/fetch.go:259`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/fetch.go#L259), `if !html || !wantBody || res.StatusCode < 200 || res.StatusCode >= 300`, `sitecrawl` does **not** run `extractDocument` on non-2xx HTML responses (`html_extraction_status == "missing_legacy"`). Therefore, on a `4xx`/`5xx` HTML response without a proven malformed `X-Robots-Tag`, raw `<meta name="robots">` in the error page body was never parsed, so returning `UNKNOWN` is strictly truthful!)*
     - **Case B — Non-HTML Response (e.g., `application/pdf`, `image/png`, `video/mp4` where `isHTML == false`):**
       Non-HTML resources do not have HTML `<meta>` tags; their **only** robots directive channel is the HTTP `X-Robots-Tag` response header (`SRC-GOOGLE-ROBOTS-001`).
       - If the non-HTML resource returned a terminal HTTP response (`http_status >= 200 && http_status < 300`, `redirect_initial_observed == "false"`, `fetch_error_type == ""`) and has **zero** `X-Robots-Tag` headers -> Returns **`NOT_APPLICABLE`**.
       - If the non-HTML resource has a valid `X-Robots-Tag: noindex` -> Returns **`PASS`**.
       - If the non-HTML resource has a malformed `X-Robots-Tag: no-index` -> Returns **`WARNING`**.
       - If the non-HTML resource failed to fetch (`http_status <= 0` or `fetch_error_type != ""`) or is an unfollowed/initial `3xx` redirect row -> Returns **`UNKNOWN`**.

3. **What if `robots_meta_sample_truncated == "true"` (More Than Cap `<meta>` Robots Declarations)?**
   - If a page contains more than the sample cap (e.g., `> 10` `<meta>` robots declarations, `robots_meta_sample_truncated == "true"`):
     - If any retained directive in the sample (or `X-Robots-Tag`) has a proven `WARNING`, return **`WARNING`**.
     - If all retained directives in the sample are valid (`robots_parse_errors == "none"`), the unretained `<meta>` declarations (`11..N`) could contain a malformed token; therefore, return **`UNKNOWN`** (never a false `PASS`).

---

### 4.4 `DEC-V1.10-004` — Raw Meta & Header Preservation Contract (Option A vs. Option B)

#### 4.4.1 Explicit Comparison of Option A and Option B

| Criterion | Option A: Preserve Raw Declarations in `sitecrawl.Page` JSON + Adapter (Recommended) | Option B: Adapter-Only Using Existing `Page.MetaTags` / `Page.MetaRobots` / `Page.XRobotsTag` |
|---|---|---|
| **Multiple same-name `<meta>` tags** (e.g., two `<meta name="robots">` or two `<meta name="googlebot">` tags) | **100% Preserved:** Every declaration is recorded in document order with its exact `name` and raw `content`. | **Broken / Lossy:** `Page.MetaTags[name]` overwrites earlier tags (`extract.go:481`). If both `robots` and `googlebot` exist and one is repeated, `adapter.go:2365` marks extra tokens `ScopeUnknown = true`. |
| **Empty `content=""` or missing `content` attribute** (`<meta name="robots" content="">` or `<meta name="robots">`) | **100% Preserved:** Recorded as `RawMetaDirective{Name: "robots", ContentRaw: "", HasContentAttr: true/false}` and counted in `RobotsMetaTotalCount`. Evaluates to `WARNING` (`parse_error:empty_directive`). | **Broken:** `extract.go:439` and `adapter.go:2267` silently drop empty `content=""`, causing `<meta name="robots" content="">` to evaluate to a false `NOT_APPLICABLE`! |
| **Raw values before normalization & source ordering** | **100% Preserved:** Untrimmed `ContentRaw` and exact 0-based declaration index are preserved. | **Broken:** `MetaTags` is an unordered Go `map[string]string` with pre-trimmed values. |
| **Multiple `X-Robots-Tag` HTTP response headers** | **100% Preserved:** `Page.RawXRobotsTags []string` stores `res.Header.Values("X-Robots-Tag")` as discrete strings while keeping `Page.XRobotsTag` joined for legacy compatibility. | **Broken:** `fetch.go:244` joins headers with `", "`, causing `X-Robots-Tag: googlebot: noindex` + `X-Robots-Tag: nofollow` to lose header boundary provenance (`ScopeUnknown = true`). |
| **SQLite Schema DDL Impact** | **Zero DDL Changes:** Stored inside the existing `sitecrawl_pages.data` JSON column (`json:",omitempty"`), identical to V1.9c `RawCanonicals` and `ExtractionStatus`. | **Zero DDL Changes.** |
| **Backward Compatibility with Existing Rules (`AR-INDEX-001`, `AR-INDEX-002`, `AR-CANON-007`, `AR-DISC-005`)** | **100% Compatible:** `Page.MetaTags`, `Page.MetaRobots`, and `Page.XRobotsTag` remain populated identically for all existing callers; legacy SQLite rows without `RawRobotsMeta` fall back cleanly to `MetaTags`/`MetaRobots`/`XRobotsTag`. | **Partial:** Cannot fix false `ScopeUnknown` on multi-tag pages or multi-header `X-Robots-Tag` responses. |

#### 4.4.2 Recommendation: Option A (Lossless Raw Acquisition in `sitecrawl.Page` JSON)

We strongly recommend **Option A**, following the exact architectural pattern proven in Milestone V1.9c (`RawCanonicals` / `CanonicalTotalCount` / `ExtractionStatus`):

1. **New Optional JSON Fields on `sitecrawl.Page` ([`internal/sitecrawl/types.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/types.go#L444-L455)):**
   ```go
   // RawMetaDirective preserves one raw <meta> robots/crawler directive declaration
   // in document order before trimming or map deduplication.
   type RawMetaDirective struct {
       Name           string `json:"name"`                        // lowercased trimmed name attribute
       NameRaw        string `json:"name_raw,omitempty"`          // original raw name attribute
       ContentRaw     string `json:"content_raw"`                 // untouched raw content attribute value
       HasContentAttr bool   `json:"has_content_attr"`            // true if content attribute was present on <meta>
   }
   ```
   And on `Page`:
   ```go
   RobotsMetaTotalCount int                `json:"robots_meta_total_count,omitempty"`
   RawRobotsMeta        []RawMetaDirective `json:"raw_robots_meta,omitempty"`
   RawXRobotsTags       []string           `json:"raw_x_robots_tags,omitempty"`
   ```
   - **Sample Cap & Uncapped Total Counter:**
     - `RobotsMetaTotalCount` is incremented (**uncapped**, `0..N`) for every `<meta>` tag whose `name` is in the recognized robots/crawler meta vocabulary (`isRobotsMetaName(name)`).
     - `RawRobotsMeta` retains up to `maxRobotsMeta = 10` raw declarations in document order (`appendCapped`).
     - `RawXRobotsTags` retains up to `maxXRobotsHeaders = 10` discrete `res.Header.Values("X-Robots-Tag")` strings (and `XRobotsTagTotalCount int` tracks the uncapped header count).
   - **Preservation of Existing Fields:**
     - `Page.MetaTags`, `Page.MetaRobots`, and `Page.XRobotsTag` continue to be populated identically so existing UI/crawler code and legacy tests are 100% unaffected.

---

## 5. Supported Token & Parameter Grammar Specification

### 5.1 Formal EBNF-Style Grammar for V1 Robots Directives

```ebnf
(* HTML <meta> content attribute grammar *)
meta_directive_string = directive_list ;

(* Single HTTP X-Robots-Tag header field-value grammar *)
x_robots_header_value = [ agent_prefix , ":" ] , directive_list ;

agent_prefix          = token ; (* RFC 9110 tchar token, validated per DEC-V1.10-002 *)

(* Comma-separated list of 1 or more directives; commas inside unavailable_after dates are protected *)
directive_list        = directive_item , { "," , directive_item } ;

directive_item        = optional_ws , ( standalone_directive | parameterized_directive ) , optional_ws ;

standalone_directive  = "all" | "none" | "index" | "noindex" | "follow" | "nofollow"
                      | "noarchive" | "nosnippet" | "notranslate" | "noimageindex"
                      | "indexifembedded" ; (* case-insensitive *)

parameterized_directive = max_snippet_rule
                        | max_image_preview_rule
                        | max_video_preview_rule
                        | unavailable_after_rule ;

max_snippet_rule        = "max-snippet" , optional_ws , ":" , optional_ws , signed_int_ge_neg1 ;
max_image_preview_rule  = "max-image-preview" , optional_ws , ":" , optional_ws , ( "none" | "standard" | "large" ) ;
max_video_preview_rule  = "max-video-preview" , optional_ws , ":" , optional_ws , signed_int_ge_neg1 ;
unavailable_after_rule  = "unavailable_after" , optional_ws , ":" , optional_ws , valid_date_time ;

signed_int_ge_neg1      = "-1" | ( digit , { digit } ) ;
```

### 5.2 Comma-Aware Tokenizer for `unavailable_after` Dates

Because RFC 850 / RFC 1123 dates contain a comma after the weekday name (e.g., `unavailable_after: Tue, 31 Dec 2026 23:59:59 GMT, noindex`), the directive splitter **must not** blindly run `strings.Split(raw, ",")`.

**Deterministic Tokenization Algorithm (`splitDirectiveItems`):**
1. Scan left-to-right across the directive list string.
2. Split on `,` **except** when the current segment (trimmed, lowercased) begins with `"unavailable_after"` followed by `:` and the text after `:` matches a weekday prefix (`^(mon|tue|wed|thu|fri|sat|sun|monday|tuesday|wednesday|thursday|friday|saturday|sunday)$` case-insensitively) and has not yet consumed the date's post-weekday comma!
3. Specifically, when a segment starts with `unavailable_after:` and the portion before the first `,` is a weekday name (`Mon`..`Sunday`), merge that comma and the following segment (`"Tue, 31 Dec 2026 23:59:59 GMT"`) into the single `unavailable_after` token before continuing comma-splitting for subsequent directives.

### 5.3 Accepted Date/Time Formats for `unavailable_after`

Per `SRC-GOOGLE-ROBOTS-001` (which specifies RFC 822, RFC 850, and ISO 8601 date/time formats), a trimmed `[date/time]` value in `unavailable_after:[date/time]` is valid if and only if it parses cleanly under one of the following deterministic Go `time.Parse` layouts (and represents a valid calendar date):

1. **ISO 8601 / RFC 3339 (with timezone or date-only):**
   - `time.RFC3339` (`"2006-01-02T15:04:05Z07:00"`)
   - `"2006-01-02T15:04:05Z"`
   - `"2006-01-02T15:04:05"` (ISO 8601 local/unspecified timezone)
   - `"2006-01-02 15:04:05Z07:00"` / `"2006-01-02 15:04:05 UTC"` / `"2006-01-02 15:04:05 GMT"` / `"2006-01-02 15:04:05"`
   - `"2006-01-02"` (ISO 8601 date-only, e.g., `unavailable_after: 2026-12-31`)
2. **RFC 1123 / RFC 822 (with or without weekday):**
   - `time.RFC1123` (`"Mon, 02 Jan 2006 15:04:05 MST"`)
   - `time.RFC1123Z` (`"Mon, 02 Jan 2006 15:04:05 -0700"`)
   - `time.RFC822` (`"02 Jan 06 15:04:05 MST"`)
   - `time.RFC822Z` (`"02 Jan 06 15:04:05 -0700"`)
   - `"02 Jan 2006 15:04:05 MST"` / `"2 Jan 2006 15:04:05 MST"` / `"02 Jan 2006 15:04:05 -0700"`
   - `"02-Jan-2006 15:04:05 MST"` / `"2-Jan-2006 15:04:05 MST"` (Google Search Central example format: `25-Aug-2007 15:00:00 EST`)
   - `"02-Jan-2006"` / `"02 Jan 2006"`
3. **RFC 850:**
   - `time.RFC850` (`"Monday, 02-Jan-06 15:04:05 MST"`)
   - `"Monday, 02-Jan-2006 15:04:05 MST"` / `"Monday, 2-Jan-2006 15:04:05 MST"`

*(Note: Whether the parsed `unavailable_after` timestamp is in the past or future is **not** a syntax error in `AR-INDEX-003`; a syntactically valid past date is a timed noindex, not a malformed token).*

---

## 6. Evidence Acquisition & Normalization Contract

### 6.1 Per-Directive Bundle Observations (`directive:*`)

For each observed `<meta>` robots/crawler declaration and each `X-Robots-Tag` header segment, [`processPageDirectives`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L2227-L2601) constructs an `audit.RobotsDirectiveObservation` with a stable ID (`directive:<audit_run_id>:<url_id>:meta:<idx>` or `directive:<audit_run_id>:<url_id>:header:<idx>`) and emits the following `NormalizedObservation` fields on `SubjectURL` with `SourceEvidenceRefs: []string{dirID, pageSrcRef}`:

| Normalized Observation Field | Value Format | Existing or New | Purpose & Consumers |
|---|---|---|---|
| `directive_source` | `"META"` or `"HTTP_HEADER"` | Existing (V1.3b2) | Identifies whether directive came from HTML `<meta>` or HTTP `X-Robots-Tag` (`AR-INDEX-002`, `AR-INDEX-003`). |
| `directive_target` | `"*"` (generic), `"<agent>"` (e.g. `"googlebot"`), or `""` (when `scope_unknown == true`) | Existing (V1.3b2) | Identifies target crawler scope (`AR-INDEX-001`, `AR-INDEX-002`, `AR-INDEX-003`). |
| `directive_raw` | Exact raw string of the `<meta content>` or `X-Robots-Tag` header | Existing (V1.3b2) | Preserves original raw directive string (`AR-INDEX-002`, `AR-INDEX-003`). |
| `directive_tokens` | Comma-joined normalized tokens (`"noindex, nofollow"`) | Existing (V1.3b2) | Consumed by `AR-INDEX-002` and `AR-INDEX-001`. |
| `directive_scope_unknown` | `"true"` (emitted only when `ScopeUnknown == true`) | Existing (V1.3b2) | Consumed by `AR-INDEX-002`. |
| `robots_directive_tokens` | Comma-joined normalized tokens (`"noindex, nofollow"`), or `""` when raw directive is empty | Existing (V1.3b2) | Required input `robots_directive_tokens` for `AR-INDEX-003`. |
| **`robots_parse_errors`** | `"none"` when all tokens and syntax in this directive bundle are valid V1 directives; otherwise comma-joined structured error codes (e.g., `"unsupported_token:no-index"`, `"invalid_parameter_value:max-snippet:abc"`, `"malformed_delimiter:noindex; nofollow"`, `"empty_directive"`, `"ambiguous:unverified_agent_prefix:custombot"`) | **New in V1.10c** | Required input `robots_parse_errors` for `AR-INDEX-003`. Explicit `"none"` prevents `MISSING != FALSE` violations! |
| **`directive_unsupported_tokens`** | Comma-joined list of offending raw tokens/segments (emitted when `len(d.UnsupportedTokens) > 0`) | **New in V1.10c** | Populates `unsupported_tokens` evidence field for `AR-INDEX-003` `WARNING` results. |

### 6.2 URL-Level Completeness Observations on `SubjectURL`

| Normalized Observation Field | Value Format | Purpose for `AR-INDEX-003` |
|---|---|---|
| `url_identity` | Raw page URL | Required input `url` (`Role: CONTEXT`). |
| `http_status` | Integer string (`"200"`, `"404"`, etc.) | Verifies terminal HTTP response availability. |
| `content_type` | MIME type (`"text/html"`, `"application/pdf"`) | Distinguishes HTML pages (which require both `<meta>` and `X-Robots-Tag` completeness) from non-HTML resources (which only use `X-Robots-Tag`). |
| `redirect_initial_observed` | `"true"` or `"false"` | Identifies initial `3xx` redirect rows where terminal body was not extracted on this row. |
| `fetch_error_type` | Error slug (when fetch failed) | Identifies failed fetches (`DNS_ERROR`, `TIMEOUT`, etc.). |
| `rendered` | `"true"` (emitted when rendered) | Identifies rendered pages where raw `<meta>` tags were suppressed. |
| `html_extraction_status` | `"complete"`, `"truncated_size_limit"`, `"read_error"`, `"missing_legacy"` | Proven HTML extraction completeness (from V1.9c). |
| **`robots_meta_sample_truncated`** | `"true"` or `"false"` (emitted on HTML pages with raw provenance when `RobotsMetaTotalCount > 0`) | Tracks whether `RobotsMetaTotalCount > len(RawRobotsMeta)` (`> 10` meta robots declarations). |

---

## 7. Verdict Precedence Decision Table (`AR-INDEX-003`)

`evaluateINDEX003` evaluates each `SubjectURL` in deterministic order using the following strict precedence table:

| Priority | Condition on `SubjectURL` | Verdict | `ObservedSummary` Template | Required `EvidenceRefs` Roles |
|---|---|---|---|---|
| **P1** | `url_identity` is missing, empty, or has conflicting observations | **`UNKNOWN`** | `"Required URL identity evidence is unavailable."` (or conflicting) | `url` (`CONTEXT`, if any) |
| **P2** | Any directive bundle observation (`directive_source`, `directive_target`, `directive_raw`, `robots_directive_tokens`, `robots_parse_errors`) is unreferenced (missing `directive:*` ref), has conflicting values within the same bundle, or is missing `robots_parse_errors` (`MISSING != FALSE`), OR URL-level control fields (`http_status`, `content_type`, `html_extraction_status`, `rendered`, `redirect_initial_observed`, `robots_meta_sample_truncated`) have conflicting/malformed values | **`UNKNOWN`** | `"Robots directive evidence is incomplete, malformed, or conflicting."` | `url` (`CONTEXT`) + conflicting/incomplete directive observations (`PRIMARY`) |
| **P3** | At least one valid directive bundle has a proven syntax or unsupported-token error in `robots_parse_errors` (`robots_parse_errors != "none"` and not solely `"ambiguous:*"`) | **`WARNING`** | `"Malformed or unsupported robots directive observed: <details>."` | `url` (`CONTEXT`) + all observations of each offending `directive:*` bundle (`PRIMARY`: `directive_raw`, `robots_directive_tokens`, `robots_parse_errors`, `directive_unsupported_tokens`, `directive_source`, `directive_target`) |
| **P4** | Any directive bundle has an ambiguous/unverifiable error code (`strings.HasPrefix(err, "ambiguous:")` or `directive_scope_unknown == "true"` without proven `WARNING`) | **`UNKNOWN`** | `"Robots directive exists but scope or prefix syntax cannot be reliably verified."` | `url` (`CONTEXT`) + ambiguous `directive:*` bundle observations (`PRIMARY`) |
| **P5** | Directive source acquisition is incomplete or unverifiable:<br>- `fetch_error_type != ""` or `http_status` missing or `< 200` or `>= 300` or `redirect_initial_observed == "true"`<br>- OR `content_type` is missing/empty<br>- OR on an HTML response (`isHTML`): `rendered == "true"` or `html_extraction_status != "complete"` or `robots_meta_sample_truncated == "true"` | **`UNKNOWN`** | `"Raw robots directive source is unavailable or incomplete (<reason>)."` | `url` (`CONTEXT`) + deciding completeness/status observations (`PRIMARY`: `html_extraction_status`, `rendered`, `http_status`, `fetch_error_type`, `robots_meta_sample_truncated`, plus any retained directive bundles as `SUPPORTING`) |
| **P6** | Source is complete (Priority P5 passed) and **zero** directive bundles exist (`len(bundles) == 0`) | **`NOT_APPLICABLE`** | `"No robots directive exists."` | `url` (`CONTEXT`) + completeness context (`html_extraction_status` / `http_status` / `content_type` as `CONTEXT`) |
| **P7** | Source is complete (Priority P5 passed), `len(bundles) >= 1`, and **every** bundle has `robots_parse_errors == "none"` | **`PASS`** | `"All observed robots directive tokens are recognized and valid."` | `url` (`CONTEXT`) + all evaluated `directive:*` bundle observations (`PRIMARY`: `directive_raw`, `robots_directive_tokens`, `robots_parse_errors`, `directive_source`, `directive_target`) |

---

## 8. Missing & Conflicting Evidence Handling (`MISSING != FALSE`)

1. **Missing `robots_parse_errors` on a Directive Bundle:**
   - If a snapshot (e.g., a synthetic test fixture or pre-V1.10c snapshot) contains `directive_raw` / `robots_directive_tokens` for a bundle `directive:X` but **lacks** a `robots_parse_errors` observation for `directive:X`, `evaluateINDEX003` **must not** assume `robots_parse_errors == "none"`. By Priority P2 (`MISSING != FALSE`), it must return **`UNKNOWN`**.
2. **Empty `robots_directive_tokens` vs. Missing Observation:**
   - When a `<meta name="robots" content="">` tag has an empty `content` attribute, the adapter emits `directive_raw = ""`, `robots_directive_tokens = ""`, and `robots_parse_errors = "empty_directive"`. Because `robots_parse_errors` is explicitly `"empty_directive"`, Priority P3 deterministically returns **`WARNING`**.
3. **Unreferenced Directive Observations:**
   - Identical to [`evaluateINDEX002`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/engine/index002.go#L40-L52), every per-directive observation is correlated strictly via its explicit `"directive:*"` reference in `SourceEvidenceRefs`. Any directive observation lacking a `"directive:*"` reference is treated as unreferenced/unreliable evidence (`UNKNOWN` unless another valid bundle already proves `WARNING`).
4. **Multiple Conflicting Observations for the Same Field:**
   - If a single bundle `directive:X` has two `robots_parse_errors` observations with different values (e.g., `"none"` and `"unsupported_token:foo"`), or if `SubjectURL` has conflicting `html_extraction_status` observations (`"complete"` and `"read_error"`), Priority P2 returns **`UNKNOWN`**.

---

## 9. Versioning, Legacy Compatibility & Regression Assessment

### 9.1 `NormalizationVersion` Policy
- **In V1.10a (this milestone):** `NormalizationVersion` remains **`"v1.10.0"`** (no code changes).
- **In V1.10b (SiteCrawl acquisition):** `NormalizationVersion` remains **`"v1.10.0"`** if V1.10b only adds raw fields to `sitecrawl.Page` without changing adapter output.
- **In V1.10c (Adapter normalization):** Because V1.10c adds new normalized observation fields (`robots_parse_errors`, `directive_unsupported_tokens`, `robots_meta_sample_truncated`) and extends `processPageDirectives` to non-HTML `X-Robots-Tag` responses and comma-containing `unavailable_after` dates, `NormalizationVersion` **must be bumped from `"v1.10.0"` to `"v1.11.0"`** in V1.10c.

### 9.2 Legacy SQLite Data Fallback Behavior
When `adapter.Build` reads a legacy `sitecrawl_pages` row created before V1.10b (where `pr.page.RawRobotsMeta == nil`, `pr.page.RobotsMetaTotalCount == 0`, and `pr.page.RawXRobotsTags == nil`):
1. **Header Fallback:** If `len(pr.page.RawXRobotsTags) == 0` and `pr.page.XRobotsTag != ""` (or `pr.xRobots != ""`), fall back to splitting the single joined `XRobotsTag` string using the existing `prevHadPrefix` state machine in `parseXRobotsSegment`.
2. **Meta Fallback:** If `len(pr.page.RawRobotsMeta) == 0` and `pr.page.ExtractionStatus == ""`:
   - Fall back to `pr.page.MetaTags` and `pr.page.MetaRobots` / `pr.metaRobots` exactly as V1.10.0 does today.
   - Note that for legacy rows (`pr.page.ExtractionStatus == ""`), `html_extraction_status` is `"missing_legacy"`. Therefore, in `AR-INDEX-003`:
     - If a legacy row has a **proven** malformed/unsupported token in `MetaTags` or `X-Robots-Tag` (e.g., `noindex; nofollow`), `AR-INDEX-003` returns **`WARNING`** (Priority P3).
     - If a legacy HTML row has only valid tokens or no tokens, `html_extraction_status == "missing_legacy"` causes `AR-INDEX-003` to return **`UNKNOWN`** (Priority P5) because raw extraction completeness and empty-meta absence cannot be verified on legacy rows.

### 9.3 Regression Risk Assessment for Existing Rules

| Existing Rule | Consumed Observations | Impact of Proposed V1.10b/c Changes & Mitigation | Regression Risk |
|---|---|---|---|
| **`AR-INDEX-001`** *(Effective noindex conflicts with explicit indexability intent)* | `url_identity`, `effective_noindex`, `expected_indexable` policy, plus supporting `directive_*` refs when `effective_noindex == "true"` ([`index001.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/engine/index001.go)) | - For HTML pages, `effective_noindex` calculation in `processPageDirectives` (`adapter.go:2685-2720`) is preserved.<br>- **Critical Guardrail for Non-HTML Responses:** When V1.10c enables `X-Robots-Tag` processing for non-HTML responses (e.g. PDFs), should `effective_noindex` also be emitted on non-HTML responses? Wait: on a PDF (`content_type = "application/pdf"`), if `X-Robots-Tag: noindex` is present, emitting `effective_noindex = "true"` (and `"false"` when no `X-Robots-Tag` noindex is present on a 200 response) is truthful; however, let's check `AR-CANON-007` and `AR-DISC-005`: `AR-DISC-005` already checks `isHTML` (`disc005.go:205`) and returns `NOT_APPLICABLE` for non-HTML URLs! And in existing tests, no test expects `effective_noindex` to be withheld on valid pages. To guarantee zero unexpected changes in `AR-INDEX-001`, we will verify all existing `index001_test.go` and `adapter_test.go` assertions. | **Low** |
| **`AR-INDEX-002`** *(Conflicting index directives)* | `url_identity`, `directive_source`, `directive_target`, `directive_raw`, `directive_tokens`, `directive_scope_unknown` ([`index002.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/engine/index002.go)) | - `index002.go` ignores `robots_parse_errors` and `directive_unsupported_tokens`.<br>- Preserving discrete `RawRobotsMeta` and `RawXRobotsTags` actually **improves** scope precision on multi-tag/multi-header pages while preserving 100% of existing `index002_test.go` and `adapter_test.go` cases.<br>- **Important Guardrail:** If `<meta name="robots" content="">` (empty content) is present, should it emit a `directive:*` bundle with `directive_tokens = ""`? Look at [`index002.go:338`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/engine/index002.go#L338):<br>`isKnownAndReliable := !isScopeUnknown && !targetConflict && targetVal != "" && !tokensConflict && len(b.tokens) > 0`<br>And in [`index002_test.go:101`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/engine/index002_test.go#L101), if `len(b.tokens) == 0` (or if `directive_tokens` has `Value: ""`), wait: in `adapter.go:2529`, `directive_tokens` is always emitted (`len(b.tokens) == 1`), even when `tokensVal == ""`! If `tokensVal == ""`, `index002.go:327` splits `""` by `,`, gets `[""]`, sees neither `index` nor `noindex`, and treats the bundle as `isKnownAndReliable = true` with no conflict! However, for `effective_noindex` in `adapter.go:2689`, an empty `<meta name="robots" content="">` has `Target: "*"`, `ScopeUnknown: false`, and no `noindex`, so `canProveIndexable` remains `true`. | **Low** (Verified against `index002.go:327-338`) |
| **`AR-CANON-007`** *(Canonical target is not noindex)* | `canonical_target_subject_ref` -> target `http_status == 200` and target `effective_noindex` ([`canon007.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/engine/canon007.go)) | Consumes only `effective_noindex` on the target URL. Unaffected by `robots_parse_errors`. | **Zero** |
| **`AR-DISC-005`** *(Sitemap-listed URL is not noindex)* | `SubjectSitemapEntry` -> target `http_status == 200`, `isHTML`, `effective_noindex`, plus supporting `meta_robots_raw`, `x_robots_raw`, `googlebot_meta_robots_raw`, `googlebot_x_robots_raw` ([`disc005.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/engine/disc005.go)) | Consumes `effective_noindex` and `*_robots_raw` fields on 200 HTML URLs. All 4 raw fields remain emitted with identical field names and semantics. | **Zero** |

---

## 10. High-Risk QA Scenarios Matrix (30 Scenarios)

The following 30 high-risk scenarios define the mandatory verification matrix for `AR-INDEX-003`:

| # | Scenario Description | Input Evidence (`<meta>` / `X-Robots-Tag` / Completeness) | Expected `robots_parse_errors` | Expected `AR-INDEX-003` Verdict |
|---|---|---|---|---|
| **1** | **Clean 200 HTML page with no robots meta tag and no `X-Robots-Tag`** | `http_status=200`, `content_type="text/html"`, `html_extraction_status="complete"`, 0 directives | *(no bundles)* | **`NOT_APPLICABLE`** |
| **2** | **Standard valid `<meta name="robots" content="index, follow">`** | `html_extraction_status="complete"`, 1 meta bundle (`target="*"`, `tokens="index, follow"`) | `"none"` | **`PASS`** |
| **3** | **All 11 standalone V1 tokens in mixed case & whitespace** | `<meta name="ROBOTS" content=" ALL, None, Index, NoIndex, Follow, NoFollow, NoArchive, NoSnippet, NoTranslate, NoImageIndex, IndexIfEmbedded ">` (`html_extraction_status="complete"`) | `"none"` | **`PASS`** *(Note: `AR-INDEX-002` returns `FAIL` for `index`+`noindex`, while `AR-INDEX-003` returns `PASS` because every token's syntax is valid)* |
| **4** | **Valid parameterized directives (`max-snippet`, `max-image-preview`, `max-video-preview`)** | `<meta name="robots" content="max-snippet: -1, max-image-preview: LARGE, max-video-preview: 0">` (`html_extraction_status="complete"`) | `"none"` | **`PASS`** |
| **5** | **Valid `unavailable_after` with ISO 8601 / RFC 3339 date** | `<meta name="robots" content="noindex, unavailable_after: 2026-12-31T23:59:59Z">` (`html_extraction_status="complete"`) | `"none"` | **`PASS`** |
| **6** | **Valid `unavailable_after` with comma-containing RFC 1123 / RFC 850 date** | `X-Robots-Tag: unavailable_after: Tue, 31 Dec 2026 23:59:59 GMT, noarchive` (`html_extraction_status="complete"`) | `"none"` | **`PASS`** *(Comma-aware splitter preserves date intact)* |
| **7** | **Near-miss spelling typo in standalone directive (`no-index`, `nofollows`, `no_index`)** | `<meta name="robots" content="no-index, follow">` (even if `html_extraction_status="truncated_size_limit"`) | `"unsupported_token:no-index"` | **`WARNING`** |
| **8** | **Deprecated/unsupported legacy directive (`noodp`, `noydir`, `nocache`)** | `<meta name="robots" content="index, follow, noodp, noydir">` | `"unsupported_token:noodp, unsupported_token:noydir"` | **`WARNING`** |
| **9** | **Semicolon or pipe delimiter instead of comma (`noindex; nofollow`, `noindex\|nofollow`)** | `<meta name="robots" content="noindex; nofollow">` | `"malformed_delimiter:noindex; nofollow"` | **`WARNING`** |
| **10** | **Whitespace-only separation between tokens (`noindex nofollow`)** | `<meta name="robots" content="noindex nofollow">` | `"malformed_delimiter:noindex nofollow"` | **`WARNING`** |
| **11** | **Empty `<meta name="robots" content="">` or whitespace `<meta name="robots" content="   ">` or `<meta name="robots">`** | `<meta name="robots" content="">` | `"empty_directive"` | **`WARNING`** |
| **12** | **Empty token slot from consecutive/trailing commas (`noindex, , nofollow` or `noindex,`)** | `<meta name="robots" content="noindex, , nofollow">` | `"empty_token_segment"` | **`WARNING`** |
| **13** | **Invalid `max-snippet` parameter (`max-snippet:abc`, `max-snippet:-2`, `max-snippet:100px`, `max-snippet:`)** | `<meta name="robots" content="max-snippet:-2">` | `"invalid_parameter_value:max-snippet:-2"` | **`WARNING`** |
| **14** | **Invalid `max-image-preview` parameter (`max-image-preview:huge`, `max-image-preview:true`, `max-image-preview:100`)** | `<meta name="robots" content="max-image-preview:huge">` | `"invalid_parameter_value:max-image-preview:huge"` | **`WARNING`** |
| **15** | **Invalid `max-video-preview` parameter (`max-video-preview:none`, `max-video-preview:-5`)** | `<meta name="robots" content="max-video-preview:none">` | `"invalid_parameter_value:max-video-preview:none"` | **`WARNING`** |
| **16** | **Invalid `unavailable_after` date (`unavailable_after:2026-02-30`, `unavailable_after:next-week`, `unavailable_after:`)** | `<meta name="robots" content="unavailable_after:not-a-date">` | `"invalid_parameter_value:unavailable_after:not-a-date"` | **`WARNING`** |
| **17** | **Unexpected parameter on standalone directive (`noindex:true`, `nofollow:all`)** | `<meta name="robots" content="noindex:true">` | `"unexpected_parameter:noindex:true"` | **`WARNING`** |
| **18** | **Misspelled parameterized directive name in `X-Robots-Tag` (`max-snipet: 50`, `max_snippet: 50`, `unavailable-after: 2026-12-31`)** | `X-Robots-Tag: max-snipet: 50` | `"unsupported_token:max-snipet: 50"` | **`WARNING`** *(Caught by Step 2 of `DEC-V1.10-002`; never mistaken for a user-agent prefix)* |
| **19** | **User-agent prefix placed inside HTML `<meta name="robots" content="googlebot: noindex">`** | `<meta name="robots" content="googlebot: noindex">` | `"meta_agent_prefix:googlebot: noindex"` | **`WARNING`** |
| **20** | **Valid agent-specific `<meta name="googlebot" content="noindex, max-snippet:120">` and `<meta name="OAI-SearchBot" content="noindex">`** | `html_extraction_status="complete"`, 2 meta bundles | `"none"` on both | **`PASS`** |
| **21** | **Multiple same-name `<meta name="googlebot">` tags where the first is malformed and the second is valid** | Tag 1: `<meta name="googlebot" content="no-index">`, Tag 2: `<meta name="googlebot" content="nofollow">` | Bundle 0: `"unsupported_token:no-index"`, Bundle 1: `"none"` | **`WARNING`** *(Proves why Option A in `DEC-V1.10-004` is required; Option B would overwrite Tag 1)* |
| **22** | **Valid multi-token `X-Robots-Tag` with user-agent prefix (`X-Robots-Tag: googlebot: noindex, nofollow, max-image-preview:large`)** | Discrete header in `RawXRobotsTags`, `html_extraction_status="complete"` | `"none"` | **`PASS`** |
| **23** | **Two separate `X-Robots-Tag` HTTP headers (`X-Robots-Tag: googlebot: noindex` + `X-Robots-Tag: nofollow`)** | `RawXRobotsTags = ["googlebot: noindex", "nofollow"]`, `html_extraction_status="complete"` | Bundle 0 (`googlebot`): `"none"`, Bundle 1 (`*`): `"none"` | **`PASS`** *(Proves why `RawXRobotsTags` slice preservation is required)* |
| **24** | **Unrecognized ASCII token prefix in `X-Robots-Tag` followed by valid directive (`X-Robots-Tag: custominternalbot: noindex`)** | `X-Robots-Tag: custominternalbot: noindex`, no other errors | `"ambiguous:unverified_agent_prefix:custominternalbot"` | **`UNKNOWN`** *(Per `DEC-V1.10-002` Step 4c)* |
| **25** | **Unrecognized prefix in `X-Robots-Tag` followed by invalid directive (`X-Robots-Tag: foo: bar`)** | `X-Robots-Tag: foo: bar` | `"unsupported_token:foo: bar"` | **`WARNING`** *(Per `DEC-V1.10-002` Step 4a)* |
| **26** | **Non-HTML resource (`application/pdf`) with `X-Robots-Tag`**<br>- **26a:** `200 application/pdf` with no `X-Robots-Tag`<br>- **26b:** `200 application/pdf` with `X-Robots-Tag: noindex, noarchive`<br>- **26c:** `200 application/pdf` with `X-Robots-Tag: no-index` | `http_status=200`, `content_type="application/pdf"` | - **26a:** *(no bundles)*<br>- **26b:** `"none"`<br>- **26c:** `"unsupported_token:no-index"` | - **26a:** **`NOT_APPLICABLE`**<br>- **26b:** **`PASS`**<br>- **26c:** **`WARNING`** |
| **27** | **Truncated or read-errored 200 HTML page (`html_extraction_status` = `"truncated_size_limit"` or `"read_error"`)**<br>- **27a:** Zero directives or only valid directives retained before truncation<br>- **27b:** Proven malformed directive (`<meta name="robots" content="no-index">` or `X-Robots-Tag: no-index`) already retained | `html_extraction_status="truncated_size_limit"` | - **27a:** `"none"` (or 0 bundles)<br>- **27b:** `"unsupported_token:no-index"` | - **27a:** **`UNKNOWN`**<br>- **27b:** **`WARNING`** |
| **28** | **Rendered HTML page (`rendered == "true"`)**<br>- **28a:** No `X-Robots-Tag` or valid `X-Robots-Tag: noindex`<br>- **28b:** Malformed `X-Robots-Tag: no-index` in HTTP response header | `rendered="true"`, `html_extraction_status="missing_legacy"` | - **28a:** `"none"` (or 0 bundles)<br>- **28b:** `"unsupported_token:no-index"` | - **28a:** **`UNKNOWN`** *(Raw HTML meta directives were replaced by DOM)*<br>- **28b:** **`WARNING`** *(HTTP header already proves violation)* |
| **29** | **Page with `> 10` `<meta>` robots declarations (`robots_meta_sample_truncated == "true"`)**<br>- **29a:** All 10 retained samples are valid<br>- **29b:** Sample #3 contains `noindex; nofollow` | `RobotsMetaTotalCount=12`, `len(RawRobotsMeta)=10`, `robots_meta_sample_truncated="true"` | - **29a:** `"none"` on all 10<br>- **29b:** `"malformed_delimiter:..."` on bundle 2 | - **29a:** **`UNKNOWN`** *(Tags 11–12 uninspected)*<br>- **29b:** **`WARNING`** |
| **30** | **Missing `robots_parse_errors` observation on a directive bundle (`MISSING != FALSE`) or conflicting `robots_parse_errors` values** | Bundle `directive:1` has `robots_directive_tokens="noindex"` but no `robots_parse_errors` observation | *(missing or conflicting)* | **`UNKNOWN`** |

---

## 11. Open Questions Requiring Independent QA Approval

While we have provided concrete recommendations for every decision above, the following specific boundary choices are highlighted for Independent QA confirmation:

1. **OQ-1 (Trailing/Empty Comma Segments — Scenario 12):**
   - When a directive contains a trailing comma or empty segment (e.g., `<meta name="robots" content="noindex, ">` or `"noindex, , nofollow"`), our proposal classifies this as **`WARNING`** (`parse_error:empty_token_segment`), whereas `parseDirectiveTokens` in V1.3b2 silently ignored empty segments between commas. Does Independent QA approve flagging empty comma segments as `WARNING` in `AR-INDEX-003` (while keeping `AR-INDEX-001` and `AR-INDEX-002` token extraction tolerant of empty segments)?
2. **OQ-2 (Unrecognized ASCII User-Agent Prefix in `X-Robots-Tag` with Valid Directives — Scenario 24):**
   - For `X-Robots-Tag: custominternalbot: noindex` (where `custominternalbot` is a valid RFC 9110 token that is neither in the V1 crawler list nor a `max*`/`unavailable*` typo, and `noindex` is a valid V1 directive), our proposal returns **`UNKNOWN`** (`ambiguous:unverified_agent_prefix`). Does Independent QA approve `UNKNOWN` for unverified user-agent prefixes in `X-Robots-Tag`, or should any syntactically valid `<rfc9110-token>: <valid-v1-directives>` in `X-Robots-Tag` evaluate to `PASS`?
3. **OQ-3 (Scope of `<meta name="...">` Attributes Inspected — Section 4.2.2.1):**
   - Our proposal inspects `<meta name="robots">` plus the 31 recognized search/AI crawler agent names (`isRobotsMetaName(name)`), ignoring non-robots `<meta>` tags (`viewport`, `description`, `referrer`, `google-site-verification`, etc.). What if a page has a typo in the `<meta name>` attribute itself, such as `<meta name="robot" content="noindex">` or `<meta name="google-bot" content="noindex">`?
   - **Proposed handling:** Include common robots `<meta name>` misspellings (`"robot"`, `"google-bot"`, `"google_bot"`, `"bing-bot"`) in `isRobotsMetaName` and emit `robots_parse_errors = "invalid_meta_name:<name>"` -> **`WARNING`** so that `<meta name="robot" content="noindex">` is caught as a malformed robots directive rather than silently ignored as an unrelated HTML meta tag! Does Independent QA approve catching `name="robot"` / `name="google-bot"` as `WARNING`?

---

## 12. Proposed Phased Implementation Plan (Post-Approval)

Following **Risk-Based QA + Batch Implementation + MVP-First**, implementation after V1.10a contract approval is divided into two or three tight, verifiable milestones:

### Milestone V1.10b — Acquisition & Evidence Preservation (`internal/sitecrawl/`)
- **Scope:**
  1. Add `RawMetaDirective`, `RobotsMetaTotalCount int`, `RawRobotsMeta []RawMetaDirective`, and `RawXRobotsTags []string` to `sitecrawl.Page` in [`internal/sitecrawl/types.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/types.go).
  2. Update [`internal/sitecrawl/extract.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/extract.go#L438-L510) (`readMeta`) to capture raw untruncated/untrimmed `RawMetaDirective` entries (up to cap 10) and increment `RobotsMetaTotalCount` for all robots/crawler `<meta>` declarations (including empty `content=""` and missing `content`).
  3. Update [`internal/sitecrawl/fetch.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/fetch.go#L242-L245) and [`internal/sitecrawl/page.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/sitecrawl/page.go#L26) to preserve `res.Header.Values("X-Robots-Tag")` in `Page.RawXRobotsTags` (while keeping `Page.XRobotsTag` joined for backward compatibility).
  4. Add unit tests in `internal/sitecrawl/extract_test.go` and `fetch_test.go`.

### Milestone V1.10c — Adapter Normalization & Directive Syntax Validator (`internal/audit/adapter/`)
- **Scope:**
  1. Bump `NormalizationVersion` from `"v1.10.0"` to `"v1.11.0"` in [`internal/audit/adapter/adapter.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/adapter/adapter.go#L356).
  2. Update `adapter.go` so `processPageDirectives` runs for both HTML pages (meta + `X-Robots-Tag`) and non-HTML responses (`X-Robots-Tag` only).
  3. Implement the V1 directive syntax/token/parameter validator (`DEC-V1.10-001` & `DEC-V1.10-002`), including comma-aware `unavailable_after` date splitting and validation, populating `RobotsDirectiveObservation.UnsupportedTokens` and `ParseErrors`, and emitting `robots_parse_errors`, `directive_unsupported_tokens`, and `robots_meta_sample_truncated`.
  4. Verify zero regressions across `AR-INDEX-001`, `AR-INDEX-002`, `AR-CANON-007`, and `AR-DISC-005`.

### Milestone V1.10d — `AR-INDEX-003` Evaluator & Integration Tests (`internal/audit/engine/`)
- **Scope:**
  1. Implement `evaluateINDEX003` in `internal/audit/engine/index003.go` enforcing the Section 7 precedence table.
  2. Register `AR-INDEX-003` in [`internal/audit/engine/engine.go`](file:///d:/Projects/audit%20technical/sitecrawl-docs/internal/audit/engine/engine.go), advancing executable rules from **`20 / 47` to `21 / 47`**.
  3. Add comprehensive unit and hermetic SQLite -> Adapter -> Engine integration tests in `internal/audit/engine/index003_test.go` covering all 30 QA scenarios.
