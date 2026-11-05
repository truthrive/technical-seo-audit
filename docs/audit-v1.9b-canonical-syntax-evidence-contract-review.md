# Audit V1.9b: AR-CANON-005 Canonical Syntax Evidence Contract Review & Amended Specification

## 1. Executive Summary

- **Audit Milestone:** Audit V1.9b — Amended Canonical Evidence Contract & Implementation Specification (`AR-CANON-005`)
- **Repository:** `truthrive/technical-seo-audit`
- **Branch:** `main`
- **Prior V1.9b Commit Baseline:** `2f34429f3d86cdaf531fab1a14d191f7fc89c5d2`
- **Historical V1.9b Draft Baseline:** `f6d38019f4b7a59f1b1164d57a5a97dc2fdafea4`
- **Last Locked Design Baseline (V1.9a):** `a82a0691c13ae753ee52a1dcba2ec8e5bff1f0be`
- **NormalizationVersion:** Strictly `v1.9.0` (unchanged in V1.9b; zero normalization changes executed in this milestone)
- **Executable Rules:** Strictly `19 / 47` (unchanged in V1.9b; zero evaluators implemented or registered in this milestone)
- **Remaining Rules:** Exactly `28 / 47`
- **Development Workflow:** **Risk-Based QA + Batch Implementation + MVP-First**
- **Milestone Type:** **Design-Only Contract Amendment & QA Gate Specification**. Zero crawler modifications, zero adapter modifications, zero evaluator registrations, zero SQLite schema changes, and zero `NormalizationVersion` changes in V1.9b.

### 1.1 Purpose of This Amendment

Following independent technical review of the `2f34429f3d86cdaf531fab1a14d191f7fc89c5d2` V1.9b contract proposal, three binding contract amendments are incorporated before any V1.9c/d implementation work is authorized:

1. **`DEC-V1.9A-001` (REVISED — Canonical Failure Semantics):**
   - Return `FAIL` **only** for proven canonical syntax or resolution errors.
   - An invalid `<base href>` alone **must not** automatically produce `FAIL` (an absolute canonical URL does not depend on `<base href>`, and RFC 3986 / WHATWG fallback or unverified base resolution context must distinguish proven resolution failure from valid absolute resolution or insufficient evidence).
   - Strictly separate **parser errors**, **resolution failures**, and **insufficient evidence** (`UNKNOWN`).
2. **`DEC-V1.9A-002` (REVISED — Provenance-Backed HTML Extraction Completeness):**
   - Require provenance-backed HTML extraction completeness (`html_extraction_status`).
   - Explicitly distinguish four terminal extraction states: **clean EOF (`complete`)**, **size-limit truncation (`truncated_size_limit`)**, **stream/transport read errors (`read_error`)**, and **missing legacy evidence (`missing_legacy` / `unextracted`)**.
   - Any missing, incomplete, truncated, or legacy-unverified evidence must produce **`UNKNOWN`**, never a false `FAIL` or false `NOT_APPLICABLE`.
3. **`DEC-V1.9A-003` (ACCEPTED WITH CONDITIONS — Raw Canonical Provenance & Backward Compatibility):**
   - Preserve raw canonical declarations (`RawCanonicals`) and `<base href>` context (`BaseHref`, `BaseHrefCount`).
   - Count declarations accurately, including empty or whitespace-only `href` values (`<link rel="canonical" href="">` or `<link rel="canonical">`).
   - Enforce strict backward compatibility with existing canonical fields (`Page.Canonical`, `Page.Canonicals`, `canonical_resolved`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count`, `canonical_target_subject_ref`), existing SQLite data (`sitecrawl_pages`), and existing evaluators (`AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, `AR-CANON-007`).

### 1.2 Governance Status

In accordance with project governance rules, no contract decision is self-approved by the Implementation Agent. All amended decisions and acceptance criteria in this document are submitted for **INDEPENDENT QA / EXTERNAL REVIEW** prior to executing V1.9c/d code changes.

---

## 2. Baseline & Frozen Rule Contract

### 2.1 Frozen Rule Definition (`AR-CANON-005`)

From `knowledge/07-v1-atomic-rule-manifest.md` (lines 346–360) and `internal/audit/rules_v1.json` (lines 655–692):

- **Rule ID:** `AR-CANON-005`
- **Name:** Canonical URL is syntactically resolvable
- **Parent Check:** `CANON-003`
- **Category:** `canonicalization`
- **Scope:** `url` (`EvaluationSubjectType`: `SubjectURL`)
- **Lifecycle:** `consolidate`
- **Automation:** `deterministic` (`PolicyMode`: `GLOBAL_DEFAULT`, `PolicyDependency`: `none`)
- **Default Severity:** `P1`
- **Source References:** `SRC-GOOGLE-CANONICAL-001`
- **Required Inputs:**
  1. `url` (Evaluation subject URL identity)
  2. `canonical_raw` (Raw un-normalized canonical declaration attribute string)
  3. `canonical_resolved_url` (Normalized/resolved HTTP(S) canonical target URL)
  4. `canonical_parse_error` (Diagnostic error classification/message when parsing or resolution fails)
- **Preconditions:** `exactly one canonical declaration exists` (`canonical_count == 1`).

### 2.2 Frozen Verdict Semantics

| Condition | Status | Non-Negotiable Contract Meaning |
|---|---|---|
| Proven complete HTML source, `canonical_count == 1`, and declaration resolves to a valid absolute HTTP(S) URL | **`PASS`** | Authoritative canonical target is syntactically valid and unambiguously resolvable. |
| Proven complete HTML source, `canonical_count == 1`, and declaration has a proven parser error or proven resolution failure | **`FAIL`** | Search engines cannot resolve the declared canonical target; consolidation signal is broken. |
| Canonical source is incomplete/truncated/read-errored/legacy-unverified, or single-canonical resolution lacks sufficient evidence | **`UNKNOWN`** | Evidence is insufficient to prove compliance or failure. Never emit a false `FAIL`. |
| Proven complete HTML source and `canonical_count == 0` or `canonical_count > 1` | **`NOT_APPLICABLE`** | Precondition (`canonical_count == 1`) is proven unsatisfied. Absence is evaluated by `AR-CANON-003`; multiplicity is evaluated by `AR-CANON-004`. |

*Frozen Invariant:* `WARNING` and `MANUAL_REVIEW` are **`not used`** for `AR-CANON-005`.

### 2.3 Non-Negotiable Architectural Principles

- `MISSING != FALSE`: Absence of an observation is never an observed negative value.
- `OBSERVATION != CONCLUSION`: Raw crawler and adapter observations remain strictly separated from rule verdicts.
- `URL DICTIONARY != FETCH`: Presence of a URL in `sitecrawl_urls` does not imply a completed page fetch.
- `INITIAL HTTP STATUS != TERMINAL HTTP STATUS`: Redirected pages (`len(res.Hops) > 0`) record initial 3xx status on `Page.Status` and skip document copying in `buildPage` (`internal/sitecrawl/page.go:57`).
- `SOURCE UNREADABLE != INVALID`: An unreadable, truncated, read-errored, or legacy-unverified HTML source must evaluate to `UNKNOWN`, never `FAIL`.
- `NORMALIZATION FAILURE != PROVEN SYNTAX FAILURE`: Aggregate normalization failure (`canonical_normalization_complete=false`) or an invalid `<base href>` that does not prevent valid canonical resolution must never automatically produce `FAIL`.

---

## 3. Source-to-Evidence Provenance & Four-Layer Completeness Analysis

Inspection of `internal/sitecrawl/fetch.go`, `internal/sitecrawl/extract.go`, `internal/sitecrawl/page.go`, `internal/sitecrawl/types.go`, and `internal/audit/adapter/adapter.go` establishes the four distinct completeness layers and current implementation gaps:

| Layer | Technical Responsibility | Verified Source Location | Current Implementation Gap |
|---|---|---|---|
| **Layer 1: HTTP Response / Transport Eligibility** | Server returned an initial 2xx HTML response (`http_status` 200–299, `isHTMLContentType`, `!rendered`, `len(res.Hops) == 0`). | `fetch.go:235–253`, `page.go:57`, `adapter.go:665–926` | Redirected rows (`len(res.Hops) > 0`) skip `buildPage` document copying (`page.go:57`). Rendered pages replace raw server HTML (`adapter.go:904–954`). |
| **Layer 2: HTML Body Read Completeness** | The HTTP response body stream (`res.Body`) was read to clean `io.EOF` without mid-stream transport errors. | `extract.go:172–176` | **Gap 4.1:** `extractDocument` breaks on `html.ErrorToken` without inspecting `z.Err()`. Mid-stream TCP resets or read timeouts after HTTP 200 headers are silently ignored (`z.Err() != io.EOF`), returning a partial `document` with `out.ErrorType == ""`. |
| **Layer 3: HTML Extraction Completeness** | Full HTML stream was tokenized within configured size limits without pre-parse rejection (`ErrTooLarge`) or mid-stream `io.LimitReader` truncation. | `fetch.go:245–249, 276–287`, `adapter.go:679–696, 1640–1647` | **Gap 4.2:** `io.LimitReader(res.Body, limit)` caps HTML reads at `min(cap, maxHTMLBytes)` and drains `extra` bytes (`fetch.go:286`) without flagging truncation when `counter.n == limit && extra > 0`.<br>**Gap 4.3:** When `res.ContentLength > cap`, `fetch.go:246` sets `out.ErrorType = "file-too-large"` and skips `extractDocument`, while `normalizeFetchError` (`adapter.go:1640`) maps `"file-too-large"` to `fetch_error_type == ""` + `GapFetchErrorUnmappable`. |
| **Layer 4: Canonical Declaration Readability** | All `<link rel="canonical">` tags (including empty/whitespace `href`) and `<base href>` attributes are captured without silent dropping or destructive in-place overwrite. | `extract.go:322–326, 500–511, 560–604` | **Gap 4.4:** `readLink` (`extract.go:502`) drops `<link rel="canonical">` when `href == ""` or whitespace-only, causing `canonical_count == 0` instead of `1`.<br>**Gap 4.5:** `d.resolve` (`extract.go:568`) overwrites `d.Canonicals[i]` in-place via `absolutize`, discards `url.Parse` errors (`extract.go:599`), and does not persist `d.BaseHref` to `Page` (`page.go:60–93`). |

---

## 4. Amended Contract Decisions (`DEC-V1.9A-001`, `DEC-V1.9A-002`, `DEC-V1.9A-003`)

### 4.1 DEC-V1.9A-001 — Canonical Failure Semantics (`REVISED`)

**Amendment Mandate:**
1. Return `FAIL` **only** for proven canonical syntax or resolution errors.
2. Invalid `<base href>` alone **must not** automatically produce `FAIL`.
3. Strictly distinguish **parser errors**, **resolution failures**, and **insufficient evidence** (`UNKNOWN`).

#### 4.1.1 Why Invalid `<base href>` Alone Must Not Automatically Produce `FAIL`

Under RFC 3986 Section 5.2 and the WHATWG HTML Standard (`<base>` element and URL resolution rules), `<base href>` interacts with `<link rel="canonical" href="...">` in three distinct ways depending on the canonical declaration itself:

| Case | Raw Canonical `href` | Document `<base href>` State | Technical Resolution Reality | Required `AR-CANON-005` Verdict |
|---|---|---|---|---|
| **Case 1: Absolute Canonical + Invalid `<base href>`** | Valid absolute HTTP(S) URL (`https://example.com/page`) | Present and malformed (`<base href="http://[::1">`) or non-HTTP(S)/hostless (`<base href="javascript:void(0)">`) | An absolute URI reference (`parsed.IsAbs() == true`) **never uses the base URI** during RFC 3986 reference resolution (`ResolveReference`). The canonical URL resolves unambiguously to `https://example.com/page`. | **`PASS`** (Invalid `<base href>` is irrelevant to an absolute canonical declaration; emitting `FAIL` here would be a false positive). |
| **Case 2: Relative Canonical + Proven Unusable `<base href>`** | Relative reference (`href="sub/page.html"` or `href="/page.html"`) | Explicitly present in `<head>`, and proven malformed (`url.Parse` error), relative-without-authority when document base is unavailable, or non-HTTP(S) scheme (`<base href="mailto:user@example.com">` / `<base href="javascript:...">`) | A relative canonical declaration depends on the effective base URI. When `<base href>` is explicitly declared with an invalid/non-HTTP(S) URI or fails resolution, the author's intended canonical target cannot be deterministically resolved as a valid HTTP(S) URL (or if `<base href>` is malformed while fallback behavior is ambiguous, see distinction below). | **`FAIL`** (`resolution_error:invalid_base_for_relative_canonical`) when `<base href>` resolves to a non-HTTP(S)/unresolvable target or explicitly breaks relative resolution; **`UNKNOWN`** if base context evidence is missing/conflicting. |
| **Case 3: Relative Canonical + Missing / Unverified Base Context** | Relative reference (`href="sub/page.html"`) | Base context was not captured (e.g., legacy SQLite snapshot lacking `BaseHref` provenance) or subject `url_identity` cannot be parsed as a valid base URL | The system lacks proven base evidence to determine whether resolution failed due to author syntax or missing audit context. | **`UNKNOWN`** (`insufficient_evidence:base_context_unavailable`). Never emit `FAIL` when base evidence is missing. |

#### 4.1.2 Three-Tier Diagnostic Classification: Parser Error vs. Resolution Failure vs. Insufficient Evidence

When `canonical_count == 1` and HTML extraction completeness is proven (`html_extraction_status == "complete"`), single-canonical evaluation must classify outcomes into one of four categories:

| Diagnostic Category (`canonical_syntax_status`) | Sub-Classification (`canonical_parse_error`) | Trigger Condition (Proven Facts Required) | `AR-CANON-005` Verdict |
|---|---|---|---|
| **`valid`** | *(none / omitted)* | Raw canonical declaration is non-empty, parses cleanly under RFC 3986 (`url.Parse`), resolves (directly if absolute, or against valid effective base if relative) to an absolute URL with scheme `http` or `https` and a non-empty valid host, and passes `normalizeURL`. | **`PASS`** |
| **`parser_error`** *(Proven Canonical Syntax Error)* | `parser_error:empty_href` | `<link rel="canonical">` tag is present (`canonical_count == 1`), and raw `href` attribute is missing, `""`, or whitespace-only. | **`FAIL`** |
| **`parser_error`** *(Proven Canonical Syntax Error)* | `parser_error:malformed_url` | Raw canonical `href` is non-empty and fails `url.Parse` (e.g., control characters, invalid port `https://example.com:abc`, unclosed IPv6 literal `http://[::1`, invalid percent-encoding `https://example.com/%ZZ`), or contains raw whitespace/illegal URI characters that prevent valid parsing. | **`FAIL`** |
| **`resolution_error`** *(Proven Canonical Resolution Failure)* | `resolution_error:unsupported_scheme` | Canonical `href` parses as a URI, but its declared scheme (or resolved scheme via `<base href>`) is not `http` or `https` (e.g., `javascript:void(0)`, `mailto:info@example.com`, `tel:+123`, `ftp://files.example.com/a`, `data:text/html,hi`). | **`FAIL`** |
| **`resolution_error`** *(Proven Canonical Resolution Failure)* | `resolution_error:missing_host` | Canonical `href` parses with `http` or `https` scheme (e.g., `https:///path-only`, `http://:8080/path`) or scheme-relative `//` with empty host (`///path`), resulting in an empty or invalid host (`host == ""` or host starts/ends with `.` / invalid port). | **`FAIL`** |
| **`resolution_error`** *(Proven Canonical Resolution Failure)* | `resolution_error:invalid_base_href` | Canonical `href` is a **relative reference** (`!parsed.IsAbs()`), the document explicitly declares `<base href="...">`, and that `<base href>` fails `url.Parse`, resolves to a non-HTTP(S) scheme, or resolves with an empty/invalid host—making the relative canonical unresolvable as intended. *(Note: If the canonical `href` is an absolute `http`/`https` URL, an invalid `<base href>` is ignored and evaluates to **`PASS`**.)* | **`FAIL`** |
| **`insufficient_evidence`** *(Unproven / Ambiguous State)* | `insufficient_evidence:missing_base_context` | Canonical `href` is relative, and either the page `url_identity` is unparseable/non-absolute or multiple conflicting `<base href>` declarations exist (`BaseHrefCount > 1` with distinct values) or raw/base provenance is absent in a legacy snapshot. | **`UNKNOWN`** |
| **`insufficient_evidence`** *(Unproven / Ambiguous State)* | `insufficient_evidence:unclassified_normalization_failure` | `canonical_normalization_complete == "false"` on a single canonical declaration, but no `parser_error:*` or `resolution_error:*` code is proven. | **`UNKNOWN`** |

#### 4.1.3 Summary of `DEC-V1.9A-001` (`REVISED`)

- **Decision State:** **REVISED — PENDING EXTERNAL / QA APPROVAL**
- **Contract Rule:** `AR-CANON-005` returns `FAIL` **if and only if** `html_extraction_status == "complete"`, `canonical_count == 1`, and `canonical_syntax_status` is `"parser_error"` or `"resolution_error"` with a non-empty `canonical_parse_error` code. Invalid `<base href>` when the canonical `href` is a valid absolute HTTP(S) URL evaluates to **`PASS`**. Any normalization failure lacking a proven `parser_error` or `resolution_error` evaluates to **`UNKNOWN`**.

---

### 4.2 DEC-V1.9A-002 — Provenance-Backed HTML Extraction Completeness (`REVISED`)

**Amendment Mandate:**
1. Require provenance-backed HTML extraction completeness.
2. Explicitly distinguish **clean EOF**, **size-limit truncation**, **read errors**, and **missing legacy evidence**.
3. Missing or incomplete evidence must produce **`UNKNOWN`**, never a false `FAIL` (and never a false `NOT_APPLICABLE` when `canonical_count == 0` is merely an artifact of unextracted/truncated HTML).

#### 4.2.1 Four-State HTML Extraction Completeness Taxonomy

A simple boolean cannot distinguish why HTML extraction was incomplete or whether a SQLite database came from a legacy crawl run. SiteCrawl (`internal/sitecrawl/fetch.go`, `extract.go`, `page.go`, `types.go`) and the Evidence Adapter (`internal/audit/adapter/adapter.go`) must record and emit a typed `html_extraction_status` observation on `SubjectURL` for HTML candidate pages:

| `html_extraction_status` Value | Proven Acquisition Condition | Underlying Cause Distinguished | `AR-CANON-005` Verdict |
|---|---|---|---|
| **`"complete"`** *(Clean EOF)* | 1. HTTP status is `200–299` and `isHTMLContentType(content_type)` is true.<br>2. `!isRendered` (`rendered != "true"`).<br>3. `len(pr.page.Redirects) == 0` (direct terminal 2xx response, not an initial 3xx hop row where `buildPage` discards `res.Doc`).<br>4. Raw `pr.errorType == ""` (no `"file-too-large"` or transport error).<br>5. Tokenizer reached `html.ErrorToken` with `z.Err() == io.EOF` (`extract.go:172`).<br>6. `io.LimitReader` did **not** truncate the stream (`counter.n < limit` OR `extra == 0` in `fetch.go:279–286`).<br>7. `pr.page.ExtractionStatus == "complete"` is explicitly persisted in `sitecrawl_pages.data`. | **Clean EOF:** The entire HTML document stream delivered by the server was tokenized without transport drop, byte-cap truncation, or DOM replacement. | Proceed to evaluate `canonical_count` (`PASS`, `FAIL`, or `NOT_APPLICABLE`). |
| **`"truncated_size_limit"`** *(Size-Limit Truncation)* | Either:<br>- Pre-parse size gate triggered: `res.ContentLength > cap` -> `pr.errorType == "file-too-large"` (`fetch.go:246`), OR<br>- Stream cap triggered during parse: `counter.n == limit && extra > 0` (`fetch.go:279–286`). | **Size-Limit Truncation:** The HTML body exceeded `maxHTMLBytes` (`10 MiB`) or configured `MaxSize`, so `<head>` / document extraction was skipped or cut off before EOF. | **`UNKNOWN`** |
| **`"read_error"`** *(Stream / Transport Read Error)* | Either:<br>- Fetch failed with a transport error (`pr.errorType != ""` and `pr.errorType != "file-too-large"`), OR<br>- `extractDocument` exited on `html.ErrorToken` with `z.Err() != nil && z.Err() != io.EOF` (mid-stream TCP reset, TLS alert, or body read timeout after HTTP 200 headers). | **Read Error:** Network or stream failure prevented reading the full HTML response body. | **`UNKNOWN`** |
| **`"missing_legacy"`** *(Missing Legacy Evidence / Unextracted)* | `sitecrawl_pages.data` JSON was persisted by an older crawler build that lacks `ExtractionStatus` (`pr.page.ExtractionStatus == ""`), OR the page was rendered (`rendered == "true"`), OR the row represents a redirect hop (`len(pr.page.Redirects) > 0`) or non-2xx / non-HTML response where static HTML extraction did not run. *(May be sub-classified or paired with `rendered == "true"`, `http_status`, `content_type`.)* | **Missing Legacy / Unextracted Evidence:** Provenance is absent because the crawl run predates extraction-completeness tracking, or raw HTML was replaced by headless rendering. | **`UNKNOWN`** |

#### 4.2.2 Why Missing Legacy Evidence Must Evaluate to `UNKNOWN`

If an existing SQLite database created before V1.9c is passed into `Adapter.BuildSnapshot`, `pr.page.ExtractionStatus` will be `""` (empty string) and `pr.page.RawCanonicals` will be `nil`.
- Under `MISSING != FALSE` and `SOURCE UNREADABLE != INVALID`, the Adapter must **not** assume that `pr.page.ExtractionStatus == ""` means `"complete"`.
- Instead, when `pr.page.ExtractionStatus == ""`, the Adapter emits `html_extraction_status = "missing_legacy"`.
- `AR-CANON-005` inspects `html_extraction_status`: because it is `"missing_legacy"` (not `"complete"`), `AR-CANON-005` deterministically returns **`UNKNOWN`** (never a false `FAIL` and never an unverified `PASS` or `NOT_APPLICABLE`), while existing rules (`AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, `AR-CANON-007`) continue to read their existing observations without regression!

#### 4.2.3 Summary of `DEC-V1.9A-002` (`REVISED`)

- **Decision State:** **REVISED — PENDING EXTERNAL / QA APPROVAL**
- **Contract Rule:** SiteCrawl must record explicit extraction completeness (`ExtractionStatus`: `"complete"`, `"truncated_size_limit"`, `"read_error"`) on `sitecrawl.Page`, and the Adapter must emit `html_extraction_status` (`"complete"`, `"truncated_size_limit"`, `"read_error"`, `"missing_legacy"`) on `SubjectURL`. `AR-CANON-005` may only emit `PASS`, `FAIL`, or `NOT_APPLICABLE` when `html_extraction_status == "complete"`. All other states (`"truncated_size_limit"`, `"read_error"`, `"missing_legacy"`, or missing/conflicting observation) must evaluate to **`UNKNOWN`**.

---

### 4.3 DEC-V1.9A-003 — Raw Canonical Provenance & Backward Compatibility (`ACCEPTED WITH CONDITIONS`)

**Amendment Mandate:**
1. Preserve raw canonical declarations and `<base href>` context.
2. Count canonical declarations accurately, including empty `href` values.
3. Preserve strict backward compatibility with existing canonical fields and SQLite data.

#### 4.3.1 Required Conditions for Acceptance

To satisfy all three conditions simultaneously without breaking existing SQLite databases, crawler callers, or `AR-CANON-003/004/006/007`:

##### Condition 1: Preserve Raw Canonical Declarations & Base Context in `sitecrawl.Page` JSON (Zero SQLite DDL Change)
In `internal/sitecrawl/extract.go`, `internal/sitecrawl/page.go`, and `internal/sitecrawl/types.go`:
- Add optional JSON fields to `sitecrawl.Page` (`internal/sitecrawl/types.go`):
  - `RawCanonicals []string` (`json:"raw_canonicals,omitempty"`): Stores the raw attribute string `attrs["href"]` (untrimmed or trimmed of leading/trailing whitespace only after recording raw presence, with empty/missing `href` stored as `""`) for every `<link rel="canonical">` tag encountered in document order (capped at 5 to match `Canonicals`).
  - `BaseHrefRaw string` (`json:"base_href_raw,omitempty"`): Stores the raw `href` attribute of the first `<base href>` tag in the document (even if empty or malformed).
  - `BaseHrefCount int` (`json:"base_href_count,omitempty"`): Counts `<base href>` tags in the document so multiple conflicting `<base href>` declarations can be detected.
  - `ExtractionStatus string` (`json:"extraction_status,omitempty"`): `"complete"`, `"truncated_size_limit"`, or `"read_error"`.
- **SQLite Compatibility:** Because `sitecrawl_pages` persists `Page` as serialized JSON in the existing `data` TEXT column (`internal/sitecrawl/persist.go:65`) and unmarshals it in `internal/audit/adapter/adapter.go:467`, **zero SQLite DDL (`CREATE TABLE` / `ALTER TABLE`) changes are required**. Both old and new SQLite files remain 100% readable.

##### Condition 2: Accurate Declaration Counting (Including Empty `href=""`)
In `internal/sitecrawl/extract.go:500–511` (`readLink`):
- Currently, `if rel == "" || href == ""` returns early before checking `rel` tokens.
- Under Condition 2, `readLink` must first parse `rel := strings.ToLower(strings.TrimSpace(attrs["rel"]))`. When any token in `strings.Fields(rel)` is `"canonical"`:
  - Always append the raw `attrs["href"]` (which is `""` when `href` is missing or empty) to `d.RawCanonicals = appendCapped(d.RawCanonicals, attrs["href"], 5)`.
  - For non-empty `href != ""` (`strings.TrimSpace(attrs["href"]) != ""`), continue appending to `d.Canonicals = appendCapped(d.Canonicals, href, 5)` so `d.Canonicals` and `Page.Canonical = first(d.Canonicals)` retain only non-empty resolved URLs for legacy crawler consumers!
- In `internal/audit/adapter/adapter.go`:
  - When `pr.page.ExtractionStatus != ""` (new provenance-backed crawl run), the total number of canonical declarations on the page is `len(pr.page.RawCanonicals)` (which includes empty `href=""` declarations).
  - When `pr.page.ExtractionStatus == ""` and `len(pr.page.RawCanonicals) == 0` (legacy SQLite crawl run), the Adapter falls back to `len(canonicals)` from `pr.page.Canonicals` / `pr.page.Canonical` / `pr.canonical` for backward compatibility.

##### Condition 3: Backward Compatibility with Existing Canonical Fields & Rules (`AR-CANON-003/004/006/007`)
Let us verify how each existing field and rule behaves when `RawCanonicals` includes empty `href=""` tags:
1. **`Page.Canonical` and `Page.Canonicals` (`internal/sitecrawl/types.go`):**
   - Remain populated **only** with non-empty post-absolutized canonical strings, preserving 100% backward compatibility with external SiteCrawl consumers and SQLite `sitecrawl_pages.canonical` column queries.
2. **`canonical_resolved` (`SubjectURL` observation):**
   - Consumed by `AR-CANON-003` (`internal/audit/engine/canon003.go:231–238`), which filters `if strings.TrimSpace(co.Value) != ""` to set `hasCanonical := len(nonBlankCanonObs) > 0`.
   - Even if `canonical_resolved` is emitted only for non-empty resolved values (or if empty values are filtered by `canon003.go:234`), a page with only `<link rel="canonical" href="">` has `len(nonBlankCanonObs) == 0` -> `AR-CANON-003` returns `WARNING` ("no usable non-empty canonical declaration"), while `AR-CANON-005` sees `canonical_count == 1` and `canonical_parse_error == "parser_error:empty_href"` -> returns `FAIL`!
3. **`canonical_count` (`SubjectURL` observation):**
   - Consumed by `AR-CANON-004`, `AR-CANON-005`, `AR-CANON-006`, and `AR-CANON-007`:
   - **Single empty canonical (`[" "]` or `[""]`):** `canonical_count == 1`, `canonical_normalization_complete == "false"`.
     - `AR-CANON-004`: `canonCount == 1` -> `PASS` (no multiple canonicals).
     - `AR-CANON-005`: `canonCount == 1` + `parser_error:empty_href` -> `FAIL` (syntactically unresolvable).
     - `AR-CANON-006`: `canonCount == 1` + `complete == false` -> `UNKNOWN` (cannot establish valid target to check HTTP 200).
     - `AR-CANON-007`: `canonCount == 1` + `complete == false` -> `NOT_APPLICABLE` (no usable single canonical target).
   - **Two canonicals: one valid, one empty (`["https://example.com/a", ""]`):** `canonical_count == 2`, `canonical_normalization_complete == "false"`.
     - `AR-CANON-004`: `canonCount == 2` + `complete == false` -> `UNKNOWN` (multiple canonical declarations exist, one unnormalizable).
     - `AR-CANON-005`: `canonCount == 2` -> `NOT_APPLICABLE` (precondition `canonical_count == 1` not met).
     - `AR-CANON-006`: `canonCount == 2` + `complete == false` -> `UNKNOWN`.
     - `AR-CANON-007`: `canonCount == 2` + `complete == false` -> `NOT_APPLICABLE`.

#### 4.3.2 Summary of `DEC-V1.9A-003` (`ACCEPTED WITH CONDITIONS`)

- **Decision State:** **ACCEPTED WITH CONDITIONS — PENDING EXTERNAL / QA APPROVAL**
- **Contract Rule:** Preserve `RawCanonicals`, `BaseHrefRaw`, `BaseHrefCount`, and `ExtractionStatus` in `sitecrawl.Page` JSON (`sitecrawl_pages.data`); count all `<link rel="canonical">` tags (including empty `href=""`) in `canonical_count`; emit `canonical_raw`, `canonical_syntax_status`, `canonical_parse_error`, and `html_extraction_status` in the Adapter; and keep `Page.Canonical`, `Page.Canonicals`, `canonical_resolved`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count`, and `canonical_target_subject_ref` backward-compatible for existing SQLite data and rules `AR-CANON-003/004/006/007`.

---

## 5. Regression Requirements for `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, and `AR-CANON-007`

Because `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-005`, `AR-CANON-006`, and `AR-CANON-007` all share parent check `CANON-003` and consume canonical observations on `SubjectURL`, V1.9c/d implementation must satisfy the following cross-rule regression contract:

| Rule ID | Consumed Observations (`internal/audit/engine/`) | Behavioral Invariant to Preserve | Required Regression Test Cases in V1.9c/d |
|---|---|---|---|
| **`AR-CANON-003`** *(Canonical declaration missing)* | `url_identity`, `http_status`, `content_type`, `rendered`, `canonical_resolved` (`canon003.go:65–238`) | - `PASS` when 2xx HTML has >= 1 non-blank `canonical_resolved`.<br>- `WARNING` when unrendered 2xx HTML has 0 non-blank `canonical_resolved`.<br>- `UNKNOWN` when `rendered == "true"` and no raw canonical.<br>- `NOT_APPLICABLE` on non-2xx or non-HTML. | 1. All existing `canon003_test.go` cases pass unchanged.<br>2. Page with valid canonical -> `PASS`.<br>3. Page with no canonical -> `WARNING`.<br>4. Page with empty `<link rel="canonical" href="">`: `canonical_resolved` must not contain a non-blank entry, so `AR-CANON-003` continues to emit `WARNING` while `AR-CANON-005` emits `FAIL`. |
| **`AR-CANON-004`** *(Multiple canonical declarations)* | `url_identity`, `canonical_count`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count` (`canon004.go:58–310`) | - `NOT_APPLICABLE` when `canonical_count == 0`.<br>- `PASS` when `canonical_count == 1` (regardless of whether `canonical_normalization_complete` is `"true"` or `"false"`).<br>- `WARNING` when `canonical_count > 1`, `complete == "true"`, `distinct == 1`.<br>- `FAIL` when `canonical_count > 1`, `complete == "true"`, `distinct > 1`.<br>- `UNKNOWN` when `canonical_count > 1` and `complete == "false"`. | 1. All existing `canon004_test.go` cases pass unchanged.<br>2. Single empty canonical (`href=""`) -> `canonical_count == 1` -> `AR-CANON-004` returns `PASS` (delegating single-tag syntax failure to `AR-CANON-005`).<br>3. Two canonicals (`href="https://example.com/a"` and `href=""`) -> `canonical_count == 2`, `complete == "false"` -> `AR-CANON-004` returns `UNKNOWN`.<br>4. Legacy SQLite row without `RawCanonicals` -> falls back to `len(Page.Canonicals)` with zero regression. |
| **`AR-CANON-006`** *(Canonical target returns final 200)* | `url_identity`, `canonical_count`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count`, `canonical_target_subject_ref`, target `http_status` (`canon006.go:61–480`) | - `NOT_APPLICABLE` when `canonical_count == 0` or (`complete == "true"` and `distinct > 1`).<br>- `UNKNOWN` when `canonical_count > 0` and `complete == "false"`, or `canonical_target_subject_ref` absent.<br>- `PASS` when unique target `http_status == 200`.<br>- `FAIL` when unique target `http_status` in `300..599`. | 1. All existing `canon006_test.go` cases pass unchanged.<br>2. Single invalid/empty canonical (`canonical_count == 1`, `complete == "false"`) -> `AR-CANON-006` returns `UNKNOWN`.<br>3. Valid absolute canonical on page with invalid `<base href>` -> `complete == "true"`, `distinct == 1` -> `AR-CANON-006` correlates target normally (`PASS`/`FAIL` by target status). |
| **`AR-CANON-007`** *(Canonical target is not noindex)* | `url_identity`, `canonical_count`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count`, `canonical_target_subject_ref`, target `http_status`, target `effective_noindex` (`canon007.go:56–511`) | - `NOT_APPLICABLE` when `canonical_count == 0`, `complete == "false"`, `distinct != 1`, `canonical_target_subject_ref` absent, or target `http_status != 200`.<br>- `PASS` when target `http_status == 200` and `effective_noindex == "false"`.<br>- `FAIL` when target `http_status == 200` and `effective_noindex == "true"`. | 1. All existing `canon007_test.go` cases pass unchanged.<br>2. Single invalid/empty canonical (`canonical_count == 1`, `complete == "false"`) -> `AR-CANON-007` returns `NOT_APPLICABLE`.<br>3. Valid single canonical target with 200 + `effective_noindex=false` -> `PASS`. |

---

## 6. Corrected 22-Scenario QA & Verdict Decision Matrix

The 22-scenario matrix below incorporates all three contract amendments (`DEC-V1.9A-001` REVISED, `DEC-V1.9A-002` REVISED, and `DEC-V1.9A-003` ACCEPTED WITH CONDITIONS), explicitly distinguishing Scenario 6's sub-cases (relative canonical vs. absolute canonical with invalid `<base href>`, and missing base context) and Scenario 13–16's four-state extraction completeness taxonomy:

| # | Scenario | Expected Verdict (Amended Contract) | Required Evidence State (`SubjectURL`) | Current Availability at `2f34429f` | Evidence Currently Missing | Required V1.9c/d Prerequisite |
|---|---|---|---|---|---|---|
| **1** | No canonical declaration (`0` `<link rel="canonical">` tags in clean-EOF 2xx HTML) | **`NOT_APPLICABLE`** | `html_extraction_status == "complete"`, `canonical_count == "0"` | Partial (`canonical_count == "0"`, but completeness & empty-tag absence unproven) | `html_extraction_status == "complete"`, empty-href capture in `extract.go:502` | V1.9c: `ExtractionStatus` + `RawCanonicals` capture |
| **2** | Exactly one valid absolute HTTP canonical URL (`http://example.com/page`) | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "http://example.com/page"`, `canonical_syntax_status == "valid"`, `canonical_normalized_target == "http://example.com/page"`, `canonical_normalization_complete == "true"` | Partial (`canonical_normalized_target` and `complete == "true"` available) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status` | V1.9c + V1.9d (`evaluateCANON005`) |
| **3** | Exactly one valid absolute HTTPS canonical URL (`https://example.com/page`) | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "https://example.com/page"`, `canonical_syntax_status == "valid"`, `canonical_normalized_target == "https://example.com/page"`, `canonical_normalization_complete == "true"` | Partial (`canonical_normalized_target` and `complete == "true"` available) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status` | V1.9c + V1.9d (`evaluateCANON005`) |
| **4** | Exactly one valid relative canonical URL (`href="/about"`) with no `<base href>` | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "/about"`, `canonical_syntax_status == "valid"`, `canonical_normalized_target == "https://example.com/about"`, `canonical_normalization_complete == "true"` | Partial (resolved target available; raw `"/about"` overwritten at `extract.go:568`) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status` | V1.9c: Preserve `RawCanonicals` before `resolve()` |
| **5** | Relative canonical URL (`href="sub"`) with valid `<base href="https://example.com/dir/">` | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "sub"`, `base_href_raw == "https://example.com/dir/"`, `canonical_syntax_status == "valid"`, `canonical_normalized_target == "https://example.com/dir/sub"` | Partial (resolved target available; `BaseHref` and raw `"sub"` not persisted) | `html_extraction_status`, `canonical_raw`, `base_href_raw`, `canonical_syntax_status` | V1.9c: Persist `RawCanonicals` & `BaseHrefRaw` on `Page` |
| **6** | **Base-Href Interaction Suite (3 sub-cases per `DEC-V1.9A-001` Revision):**<br>- **6a:** Relative canonical (`href="sub"`) + explicitly invalid/non-HTTP(S) `<base href="http://[::1">` or `<base href="mailto:a@b">`<br>- **6b:** **Absolute** canonical (`href="https://example.com/page"`) + invalid `<base href="http://[::1">`<br>- **6c:** Relative canonical (`href="sub"`) + missing/conflicting base context (`BaseHrefCount > 1` or unparseable page `url_identity`) | - **6a:** **`FAIL`**<br>- **6b:** **`PASS`**<br>- **6c:** **`UNKNOWN`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, plus:<br>- **6a:** `canonical_syntax_status == "resolution_error"`, `canonical_parse_error == "resolution_error:invalid_base_href"`<br>- **6b:** `canonical_syntax_status == "valid"`, `canonical_normalized_target == "https://example.com/page"`<br>- **6c:** `canonical_syntax_status == "insufficient_evidence"`, `canonical_parse_error == "insufficient_evidence:missing_base_context"` | **No** (In `extract.go:563`, 6a silently falls back to request URL and produces a **false `PASS`**!) | `BaseHrefRaw`, `BaseHrefCount`, relative-vs-absolute distinction in Adapter resolution, typed `resolution_error` vs `insufficient_evidence` | V1.9c: Persist `BaseHrefRaw` & `BaseHrefCount`; enforce 6a (`FAIL`), 6b (`PASS`), 6c (`UNKNOWN`) |
| **7** | Canonical using unsupported scheme (`javascript:void(0)`, `mailto:a@b.com`, `ftp://files.example.com/a`) | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_syntax_status == "resolution_error"`, `canonical_parse_error == "resolution_error:unsupported_scheme"` | Partial (only untyped `canonical_normalization_complete == "false"` is emitted) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status`, `canonical_parse_error` | V1.9c: Classifier emits `resolution_error:unsupported_scheme` |
| **8** | Canonical containing malformed URL syntax (`http://[::1`, `https://example.com:badport`, `http:///path`) | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_syntax_status == "parser_error"` (or `"resolution_error"` for `missing_host`), `canonical_parse_error == "parser_error:malformed_url"` (or `"resolution_error:missing_host"`) | Partial (only untyped `canonical_normalization_complete == "false"` is emitted) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status`, `canonical_parse_error` | V1.9c: Classifier emits `parser_error:malformed_url` / `resolution_error:missing_host` |
| **9** | Empty canonical declaration (`<link rel="canonical" href="">` or `<link rel="canonical">`) | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == ""`, `canonical_syntax_status == "parser_error"`, `canonical_parse_error == "parser_error:empty_href"` | **No** (`extract.go:502` drops empty `href`, emitting `canonical_count == "0"`) | Empty `href` capture in `readLink`, `canonical_parse_error == "parser_error:empty_href"` | V1.9c: Capture empty `href` in `RawCanonicals` (`extract.go:500`) |
| **10** | Whitespace-only canonical declaration (`<link rel="canonical" href="   ">`) | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "   "`, `canonical_syntax_status == "parser_error"`, `canonical_parse_error == "parser_error:empty_href"` | **No** (`extract.go:501–502` trims and drops whitespace `href`, emitting `canonical_count == "0"`) | Whitespace `href` capture in `readLink`, `canonical_parse_error == "parser_error:empty_href"` | V1.9c: Capture whitespace `href` in `RawCanonicals` (`extract.go:500`) |
| **11** | Multiple distinct canonical declarations (`canonical_count > 1`, including `[valid, ""]` or `[valid1, valid2]`) | **`NOT_APPLICABLE`** | `html_extraction_status == "complete"`, `canonical_count > "1"` | Partial (works when all tags are non-empty; fails when 1 of 2 tags has `href=""`) | `html_extraction_status == "complete"`, empty-href counting in `canonical_count` | V1.9c + V1.9d (`canonical_count > 1` -> `NOT_APPLICABLE`) |
| **12** | Duplicate identical canonical declarations (`canonical_count > 1`, identical targets) | **`NOT_APPLICABLE`** | `html_extraction_status == "complete"`, `canonical_count > "1"`, `canonical_distinct_normalized_count == "1"` | Partial (`canonical_count > "1"` available; completeness unproven) | `html_extraction_status == "complete"` | V1.9c + V1.9d (`canonical_count > 1` -> `NOT_APPLICABLE`) |
| **13** | Canonical source unavailable or legacy-unverified:<br>- **13a:** Non-HTML `content_type` (`application/pdf`, `image/png`)<br>- **13b:** Legacy SQLite row lacking `ExtractionStatus` (`html_extraction_status == "missing_legacy"`) | **`UNKNOWN`** *(for 13b; and `UNKNOWN` / non-applicable per frozen manifest when source cannot be read)* | `html_extraction_status == "missing_legacy"` (or non-HTML / missing `html_extraction_status`) | Partial (`content_type` available; legacy vs complete distinction absent) | `html_extraction_status` (`"missing_legacy"`) | V1.9c + V1.9d: Require `html_extraction_status == "complete"` |
| **14** | HTTP fetch failed (DNS/TLS/timeout/connect error, HTTP 3xx/4xx/5xx, or redirect hop row) | **`UNKNOWN`** | `fetch_error_type != ""` or `http_status` not in `200..299` or `html_extraction_status == "read_error"` / `"missing_legacy"` | Yes for mappable errors and non-200 status | `html_extraction_status` integration | V1.9c + V1.9d: Return `UNKNOWN` when `html_extraction_status != "complete"` |
| **15** | HTTP 200 HTML response, but extraction incomplete:<br>- **15a:** Mid-stream `z.Err()` read error (`html_extraction_status == "read_error"`)<br>- **15b:** Size-limit truncation via `io.LimitReader` or `file-too-large` (`html_extraction_status == "truncated_size_limit"`) | **`UNKNOWN`** | `http_status == "200"`, `html_extraction_status == "read_error"` (15a) or `"truncated_size_limit"` (15b) | **No** (Currently indistinguishable from a clean 200 HTML page with 0 canonicals!) | `z.Err()` check (`extract.go:172`), `io.LimitReader` cap check (`fetch.go:279`), `"file-too-large"` check (`adapter.go:679`) | V1.9c: Populate `ExtractionStatus` (`"read_error"` / `"truncated_size_limit"`) |
| **16** | Rendered page without retained raw server HTML (`rendered == "true"`) | **`UNKNOWN`** | `rendered == "true"`, `html_extraction_status == "missing_legacy"` (or not `"complete"`) | Yes (`rendered == "true"` emitted at `adapter.go:920`) | `evaluateCANON005` check | V1.9d: Return `UNKNOWN` when `rendered == "true"` |
| **17** | Single canonical where normalization succeeds (`canonical_normalization_complete == "true"`) and resolution is proven valid | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_syntax_status == "valid"`, `canonical_normalization_complete == "true"`, `canonical_normalized_target != ""` | Partial (requires `html_extraction_status == "complete"` and `canonical_syntax_status == "valid"`) | `html_extraction_status`, `canonical_syntax_status`, `canonical_raw` | V1.9c + V1.9d |
| **18** | Single canonical where normalization fails with a proven `parser_error` or `resolution_error` | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_normalization_complete == "false"`, `canonical_syntax_status` in `{"parser_error", "resolution_error"}`, `canonical_parse_error != ""` | **No** (Only untyped `canonical_normalization_complete == "false"` exists today) | `canonical_syntax_status`, `canonical_parse_error`, `html_extraction_status` | V1.9c + V1.9d |
| **19** | Single canonical where normalization fails **without** a proven syntax/resolution cause (`insufficient_evidence`) | **`UNKNOWN`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_normalization_complete == "false"`, `canonical_syntax_status == "insufficient_evidence"` (or `canonical_parse_error` absent) | Yes (Conservative fallback design) | `evaluateCANON005` safeguard implementation | V1.9c + V1.9d: Return `UNKNOWN` whenever `parser_error`/`resolution_error` is not proven |
| **20** | Conflicting observations on `SubjectURL` (contradictory `canonical_count`, `html_extraction_status`, `canonical_syntax_status`, `canonical_parse_error`, or `canonical_normalized_target`) | **`UNKNOWN`** | Multiple conflicting `NormalizedObservation` values for the same field on `SubjectURL` | Yes (`EvidenceIndex` preserves all observations per field) | `evaluateCANON005` conflict checks | V1.9d: Conflict detection across all required fields |
| **21** | Missing or conflicting subject URL identity (`url_identity` missing, empty, or conflicting on `SubjectURL`) | **`UNKNOWN`** | `url_identity` observation absent, blank, or contradictory | Yes (`EvidenceIndex` pattern established) | `evaluateCANON005` identity check | V1.9d: Standard `url_identity` check |
| **22** | Cross-run or invalid subject correlation (`AuditRunID` or `SnapshotID` mismatch) | **`UNKNOWN`** | Handled by `EvidenceIndex` validation in `internal/audit/engine/evidence.go` | Yes | None | Existing `EvidenceIndex` invariant + `canon005_test.go` verification |

---

## 7. Proposed V1.9c/d Implementation Scope & QA Gates

Under the **Risk-Based QA + Batch Implementation + MVP-First** workflow, V1.9c (High-Risk Acquisition & Adapter Wiring) and V1.9d (Medium-Risk Evaluator Implementation) are specified below with explicit QA gates. **No code from this section is executed in V1.9b.**

### 7.1 Phase 1 (V1.9c — High Risk): SiteCrawl Acquisition & Adapter Evidence Wiring

#### A. SiteCrawl Acquisition Changes (`internal/sitecrawl/`)
1. **`internal/sitecrawl/types.go`:**
   - Extend `Page` struct with backward-compatible optional JSON fields:
     ```go
     RawCanonicals    []string `json:"raw_canonicals,omitempty"`
     BaseHrefRaw      string   `json:"base_href_raw,omitempty"`
     BaseHrefCount    int      `json:"base_href_count,omitempty"`
     ExtractionStatus string   `json:"extraction_status,omitempty"`
     ```
2. **`internal/sitecrawl/extract.go`:**
   - Update `document` struct to track `RawCanonicals []string`, `BaseHrefRaw string`, `BaseHrefCount int`, and `ReadErr error`.
   - In `extractDocument` (`extract.go:172–176`): when `tt == html.ErrorToken`, inspect `err := z.Err()`. If `err != nil && err != io.EOF`, store `d.ReadErr = err` and return `(d, err)`.
   - In `<base>` token handling (`extract.go:258–263`): increment `d.BaseHrefCount++` whenever `<base>` has an `href` attribute (or on `<base href="...">`), and capture the first `d.BaseHrefRaw = attrs["href"]` before trimming.
   - In `readLink` (`extract.go:499–525`): check `rel` tokens before checking `href == ""`. When a token is `"canonical"`, append `attrs["href"]` to `d.RawCanonicals = appendCapped(d.RawCanonicals, attrs["href"], 5)`. Only append to `d.Canonicals` when trimmed `href != ""`.
   - In `d.resolve(base)` (`extract.go:560–569`): leave `d.RawCanonicals` untouched so raw values are never overwritten.
3. **`internal/sitecrawl/fetch.go` & `internal/sitecrawl/page.go`:**
   - In `fetch.go:245–288`:
     - When `res.ContentLength > cap` (`ErrTooLarge`), set `out.ExtractionStatus = "truncated_size_limit"`.
     - When `extractDocument` returns a non-nil `err`, set `out.ExtractionStatus = "read_error"` and `out.ErrorType = classifyError(err)`.
     - When `counter.n == limit && extra > 0`, set `out.ExtractionStatus = "truncated_size_limit"`.
     - When `err == nil` and not truncated (`counter.n < limit || extra == 0`), set `out.ExtractionStatus = "complete"`.
   - In `buildPage` (`page.go:57–93`): copy `p.ExtractionStatus = res.ExtractionStatus` (when `len(res.Hops) == 0`), `p.RawCanonicals = d.RawCanonicals`, `p.BaseHrefRaw = d.BaseHrefRaw`, and `p.BaseHrefCount = d.BaseHrefCount`.

#### B. Evidence Adapter Changes (`internal/audit/adapter/adapter.go`)
1. **Emit `html_extraction_status` on `SubjectURL`:**
   - Emit `"complete"`, `"truncated_size_limit"`, `"read_error"`, or `"missing_legacy"` according to Section 4.2.1.
2. **Accurate `canonical_count` & Backward-Compatible `canonical_resolved`:**
   - When `pr.page.ExtractionStatus == "complete"`, set `canonical_count` to `len(pr.page.RawCanonicals)`.
   - Keep `canonical_resolved` emitted only for non-empty resolved entries so `AR-CANON-003` is unaffected.
   - Populate `CanonicalObservation.RawValues` and `CanonicalObservation.ParseErrors`.
3. **Single-Canonical Syntax & Resolution Classifier:**
   - When `len(pr.page.RawCanonicals) == 1` (or legacy fallback count == 1), emit `canonical_raw` and classify using `(rawHref, pr.page.BaseHrefRaw, pr.page.BaseHrefCount, pr.url)` into:
     - `canonical_syntax_status = "valid"` (when `rawHref` is a valid absolute HTTP(S) URL—ignoring any invalid `<base href>`—or when `rawHref` is a relative reference that resolves cleanly via a valid `<base href>` or page URL to a valid HTTP(S) URL).
     - `canonical_syntax_status = "parser_error"` with `canonical_parse_error` in `{"parser_error:empty_href", "parser_error:malformed_url"}`.
     - `canonical_syntax_status = "resolution_error"` with `canonical_parse_error` in `{"resolution_error:unsupported_scheme", "resolution_error:missing_host", "resolution_error:invalid_base_href"}`.
     - `canonical_syntax_status = "insufficient_evidence"` with `canonical_parse_error` in `{"insufficient_evidence:missing_base_context", "insufficient_evidence:unclassified_normalization_failure"}`.
4. **Bump `NormalizationVersion` (in V1.9c only):**
   - Update `NormalizationVersion` from `"v1.9.0"` to `"v1.10.0"` in `internal/audit/adapter/adapter.go:356` when V1.9c is implemented. *(In V1.9b, `NormalizationVersion` remains strictly `"v1.9.0"`.)*

### 7.2 Phase 2 (V1.9d — Medium Risk): `AR-CANON-005` Evaluator Implementation

1. **`internal/audit/engine/canon005.go`:**
   - Implement `evaluateCANON005` enforcing the amended 22-scenario matrix:
     - Validate `url_identity` (missing/conflicting -> `UNKNOWN`).
     - Validate single-valued observation consistency (`html_extraction_status`, `canonical_count`, `canonical_syntax_status`, `canonical_parse_error`, `canonical_normalization_complete`, `rendered`). Any conflict -> `UNKNOWN`.
     - If `rendered == "true"` or `html_extraction_status != "complete"` -> return `UNKNOWN`.
     - Parse `canonical_count`: if `0` or `> 1` -> return `NOT_APPLICABLE`.
     - For `canonical_count == 1`:
       - If `canonical_syntax_status == "valid"` and `canonical_normalization_complete == "true"` and `canonical_normalized_target != ""` -> return `PASS`.
       - If `canonical_syntax_status` in `{"parser_error", "resolution_error"}` and `canonical_parse_error != ""` -> return `FAIL`.
       - Otherwise (including `canonical_syntax_status == "insufficient_evidence"`) -> return `UNKNOWN`.
2. **`internal/audit/engine/engine.go`:**
   - Register `e.register(ruleIDCANON005, evaluateCANON005)` (`19 / 47 -> 20 / 47` in V1.9d).

### 7.3 Risk-Based QA Gates for V1.9c/d

| QA Gate ID | Risk Tier | Gate Requirement | Verification Command / Artifact |
|---|---|---|---|
| **GATE-V1.9CD-01** | **High Risk** *(Acquisition)* | Unit tests in `internal/sitecrawl/extract_test.go` and `fetch_test.go` verifying: (a) clean EOF sets `ExtractionStatus = "complete"`, (b) mid-stream reader error sets `ExtractionStatus = "read_error"`, (c) `io.LimitReader` cap sets `ExtractionStatus = "truncated_size_limit"`, (d) `ContentLength > cap` sets `ExtractionStatus = "truncated_size_limit"`, (e) `<link rel="canonical" href="">` is captured in `RawCanonicals` while `Page.Canonicals` and `Page.Canonical` remain backward-compatible. | `go test -v -count=1 ./internal/sitecrawl/...` |
| **GATE-V1.9CD-02** | **High Risk** *(Adapter & Legacy SQLite)* | Adapter tests in `internal/audit/adapter/adapter_test.go` verifying: (a) all four `html_extraction_status` states (`complete`, `truncated_size_limit`, `read_error`, `missing_legacy`), (b) legacy SQLite rows without `ExtractionStatus` emit `html_extraction_status = "missing_legacy"`, (c) Case 6a (`relative` + invalid `<base href>`) emits `resolution_error:invalid_base_href`, (d) Case 6b (`absolute` + invalid `<base href>`) emits `canonical_syntax_status = "valid"`. | `go test -v -count=1 ./internal/audit/adapter/...` |
| **GATE-V1.9CD-03** | **High Risk** *(Cross-Rule Regression)* | Regression test suite proving zero behavioral regressions across `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, and `AR-CANON-007` (including pages with empty `href=""`, `[valid, ""]`, and invalid `<base href>`). | `go test -v -count=1 -run "TestEvaluateCANON00(3|4|6|7)" ./internal/audit/engine/...` |
| **GATE-V1.9CD-04** | **Medium Risk** *(Evaluator & 22-Scenario Matrix)* | Exhaustive table-driven unit tests in `internal/audit/engine/canon005_test.go` covering all 22 scenarios (including sub-cases 6a/6b/6c, 13a/13b, 15a/15b) plus hermetic end-to-end SQLite -> Adapter -> Engine integration tests. | `go test -v -count=1 ./internal/audit/...` |
| **GATE-V1.9CD-05** | **MVP-First Pipeline Gate** | Full repository build, vet, test, and diff check passing cleanly; unimplemented rules (`27 / 47` after V1.9d) remain explicitly un-registered / `NOT EVALUATED` and never silently `PASS`. | `go test -count=1 ./... && go vet ./... && go build ./cmd/sitecrawl-dev && git diff --check` |

---

## 8. Unresolved Risks & Open Items for QA Review

1. **Multiple `<base href>` Declarations in Malformed HTML (`BaseHrefCount > 1`):**
   - *Risk:* According to the HTML specification, only the first `<base href>` element in `<head>` is obeyed by browsers, whereas a page containing multiple conflicting `<base href>` tags with a relative canonical may indicate broken template injection.
   - *Proposed Handling:* In `extract.go:322`,SiteCrawl already captures the first `<base href>` (`if d.BaseHref == ""`). The contract records `BaseHrefCount`; if QA prefers strictly following browser first-base semantics (using the first `<base href>`) vs. returning `UNKNOWN` when multiple distinct `<base href>` tags exist alongside a relative canonical, this should be confirmed during QA sign-off.
2. **Legacy SQLite Snapshot Re-Evaluation Behavior (`missing_legacy`):**
   - *Risk:* Any SQLite crawl database generated prior to V1.9c lacks `extraction_status` and `raw_canonicals` in `sitecrawl_pages.data`. Under `DEC-V1.9A-002` (REVISED), running `AR-CANON-005` against a legacy SQLite file will deterministically return `UNKNOWN` (`html_extraction_status == "missing_legacy"`) for all URLs until a fresh crawl is run with the V1.9c crawler.
   - *Mitigation:* This is intentional and required by `SOURCE UNREADABLE != INVALID` and `MISSING != FALSE`. Existing rules (`AR-CANON-003/004/006/007`) remain 100% functional on legacy SQLite files.
3. **Pre-existing `.ai-dev-system/docs/INDEX.md` Local Link Gate Failures:**
   - *Risk:* `.ai-dev-system/gates/run.ps1` (`docs.local-links`) reports 5 pre-existing broken template links in `.ai-dev-system/docs/INDEX.md` (`architecture/`, `decisions/`, `../tests/run.ps1`, `../README.md`, `../CHANGELOG.md`). Because `.ai-dev-system/` is outside the V1.9b scope boundary, those template links remain untouched.

---

## 9. Contract Acceptance Criteria & QA Sign-Off Checklist

Before V1.9c/d implementation begins, independent QA / Project Owner sign-off is requested against these acceptance criteria:

- [ ] **AC-1 (`DEC-V1.9A-001` Revised):** `AR-CANON-005` emits `FAIL` only on proven `parser_error` (`empty_href`, `malformed_url`) or proven `resolution_error` (`unsupported_scheme`, `missing_host`, `invalid_base_href` on a relative canonical). An invalid `<base href>` paired with a valid **absolute** HTTP(S) canonical URL evaluates to `PASS`. Unproven normalization failures or missing base context evaluate to `UNKNOWN`. *(Status: **PENDING QA / EXTERNAL APPROVAL**)*
- [ ] **AC-2 (`DEC-V1.9A-002` Revised):** SiteCrawl and Adapter distinguish `"complete"` (clean EOF), `"truncated_size_limit"`, `"read_error"`, and `"missing_legacy"` via `html_extraction_status`. Any state other than `"complete"` evaluates to `UNKNOWN` in `AR-CANON-005`. *(Status: **PENDING QA / EXTERNAL APPROVAL**)*
- [ ] **AC-3 (`DEC-V1.9A-003` Accepted with Conditions):** `RawCanonicals`, `BaseHrefRaw`, `BaseHrefCount`, and `ExtractionStatus` are persisted in `sitecrawl_pages.data` JSON (zero SQLite DDL changes); empty `href=""` canonical tags are counted in `canonical_count`; and `Page.Canonical`, `Page.Canonicals`, and `AR-CANON-003/004/006/007` maintain 100% backward compatibility. *(Status: **PENDING QA / EXTERNAL APPROVAL**)*
- [ ] **AC-4 (V1.9c/d Scope & QA Gates):** The 22-scenario QA matrix (Section 6) and the 5 QA gates (`GATE-V1.9CD-01` through `GATE-V1.9CD-05`, Section 7.3) are approved to govern V1.9c/d execution. *(Status: **PENDING QA / EXTERNAL APPROVAL**)*
