# Audit V1.9b: AR-CANON-005 Canonical Syntax Evidence Contract Review & Amended Specification

## 1. Executive Summary

- **Audit Milestone:** Audit V1.9b — Amended Canonical Evidence Contract & Implementation Specification (`AR-CANON-005`)
- **Repository:** `truthrive/technical-seo-audit`
- **Branch:** `main`
- **Prior V1.9b Commit Baseline:** `3449d003754c5c5b1f145aceaa63f5c54abb75cd`
- **Historical V1.9b Baselines:** `2f34429f3d86cdaf531fab1a14d191f7fc89c5d2`, `f6d38019f4b7a59f1b1164d57a5a97dc2fdafea4`
- **Last Locked Design Baseline (V1.9a):** `a82a0691c13ae753ee52a1dcba2ec8e5bff1f0be`
- **NormalizationVersion:** Strictly `v1.9.0` (unchanged in V1.9b; zero normalization changes executed in this milestone)
- **Executable Rules:** Strictly `19 / 47` (unchanged in V1.9b; zero evaluators implemented or registered in this milestone)
- **Remaining Rules:** Exactly `28 / 47`
- **Development Workflow:** **Risk-Based QA + Batch Implementation + MVP-First**
- **Milestone Type:** **Design-Only Contract Amendment & QA Gate Specification**. Zero crawler modifications, zero adapter modifications, zero evaluator registrations, zero SQLite schema changes, and zero `NormalizationVersion` changes in V1.9b.

### 1.1 Purpose of This Amendment

Following Independent QA review of `3449d003754c5c5b1f145aceaa63f5c54abb75cd`, this specification resolves the three remaining contract issues before any V1.9c/d implementation is authorized:

1. **Canonical Count & Sample Cap Separation (`DEC-V1.9A-003`):**
   - `canonical_count` must reflect the **true total number of `<link rel="canonical">` declarations** encountered in the document (including empty/whitespace `href` values), **never** `len(RawCanonicals)` where `RawCanonicals` is capped at 5 samples (`appendCapped(..., 5)`).
   - Define a separate uncapped integer counter (`CanonicalTotalCount` on `sitecrawl.Page`) alongside capped sample arrays (`RawCanonicals`, `Canonicals`, max 5), and emit explicit sample-cap truncation evidence (`canonical_sample_truncated = "true"` / `"false"` and `GapCanonicalSampleCapped`) when `CanonicalTotalCount > 5`.
   - Define exact behavior and acceptance criteria for pages with `> 5` canonical declarations across `AR-CANON-005` (`NOT_APPLICABLE` because `canonical_count > 1`) and `AR-CANON-004` (`FAIL` if the 5 retained samples already contain `>= 2` distinct valid targets; `UNKNOWN` if all 5 retained samples normalize to 1 target or fail normalization because unretained declarations 6..N cannot be verified identical).
2. **Invalid `<base href>` & Standards-Based Fallback Semantics — Scenario 6 (`DEC-V1.9A-001`):**
   - Re-evaluate `<base href>` under the **WHATWG HTML Standard (§ 4.2.3 `The base element` / `frozen base URL` algorithm)** and **RFC 3986 (§ 5.1 / § 5.2)**:
     - Under WHATWG HTML § 4.2.3, when a `<base href="...">` attribute fails URL parsing against the fallback base URL (or when no `<base href>` has a valid parseable URL), the element has no frozen base URL and the **document's fallback base URL (the request/response `FinalURL`) remains the document base URL**!
     - However, if `<base href="...">` **succeeds** in parsing into a frozen base URL with a non-HTTP(S) scheme that cannot serve as a relative base or resolves a relative canonical to a non-HTTP(S) scheme (e.g. `<base href="javascript:...">`, `<base href="mailto:...">`, `<base href="data:...">`, `<base href="ftp://example.com/">`), then resolving a relative canonical against that frozen base URL produces a non-HTTP(S) or unresolvable target (`FAIL`).
     - Furthermore, because Go's `url.Parse` does not implement the full WHATWG URL parser error-recovery state machine for malformed `<base href>` strings, when a relative canonical is paired with a `<base href>` that fails RFC 3986 `url.Parse` (or when multiple `<base href>` tags exist or fallback base URL is unavailable), the Adapter cannot prove whether a browser/Googlebot applied WHATWG error recovery to `<base href>` or fell back to the document URL (`UNKNOWN` — `insufficient_evidence:ambiguous_base_fallback`, **never** an automatic `FAIL`).
   - Distinguish:
     - **6a-i (Proven Resolution Failure -> `FAIL`):** Relative canonical + `<base href>` that parses and resolves the relative canonical to an unsupported non-HTTP(S) scheme (e.g. `<base href="ftp://files.example.com/dir/">` + `href="page.html"` -> `ftp://files.example.com/dir/page.html`) or empty host (`<base href="http:///dir/">`).
     - **6a-ii (Ambiguous / Unparseable `<base href>` with Relative Canonical -> `UNKNOWN`):** Relative canonical + `<base href>` that fails RFC 3986 `url.Parse` (e.g. `<base href="http://[::1">`) where Go's `url.Parse` cannot verify whether WHATWG URL parsing fell back to the document URL or failed.
     - **6b (Successful Fallback / Independent Resolution -> `PASS`):** (i) Any valid **absolute** HTTP(S) canonical (`href="https://example.com/page"`) regardless of `<base href>` state, OR (ii) a relative canonical where `<base>` has no `href` attribute (`<base target="_blank">`) or where `<base href="">` / `<base href="#frag">` resolves cleanly against the document's `FinalURL` to a valid HTTP(S) URL per WHATWG § 4.2.3 & RFC 3986 § 5.2.
     - **6c (Missing / Conflicting Base Context -> `UNKNOWN`):** Relative canonical + missing legacy base provenance (`missing_legacy`), unparseable page `url_identity`, or multiple conflicting `<base href>` declarations.
3. **Verifiable HTML Extraction Completeness (`DEC-V1.9A-002`):**
   - Withdraw the insufficient condition `counter.n < limit || extra == 0`. Explain why `extra == 0` is defective: if an HTML document is **exactly `limit` bytes** long (`counter.n == limit`), `io.LimitReader(res.Body, limit)` returns `io.EOF` before reading the underlying `res.Body` to EOF; if the subsequent `io.Copy` drain hits a transport read error on `res.Body` after reading `0` bytes (`extra == 0, drainErr != nil`), `counter.n < limit || extra == 0` would falsely report `"complete"`!
   - Replace `io.LimitReader(res.Body, limit)` with a **`limit + 1` probe reader** (`probeLimit := limit + 1`) that captures both byte count and the exact terminal `io.Reader` error (`readErr`), **and** capture `drainErr` during any post-parse drain of `res.Body`.
   - Prove clean EOF (`html_extraction_status = "complete"`) **if and only if** `probeReader.n <= limit`, `probeReader.err == io.EOF`, `z.Err() == io.EOF`, `drainErr == nil`, `pr.errorType == ""`, `!isRendered`, and `len(pr.page.Redirects) == 0`.

### 1.2 Governance Status

No contract decision is self-approved by the Implementation Agent. All amended decisions and acceptance criteria remain submitted for **INDEPENDENT QA REVIEW** prior to starting V1.9c/d.

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
- **Required Inputs:** `url`, `canonical_raw`, `canonical_resolved_url`, `canonical_parse_error`
- **Preconditions:** `exactly one canonical declaration exists` (`canonical_count == 1`).

### 2.2 Frozen Verdict Semantics

| Condition | Status | Non-Negotiable Contract Meaning |
|---|---|---|
| Proven complete HTML source (`html_extraction_status == "complete"`), `canonical_count == 1`, and declaration resolves to a valid absolute HTTP(S) URL | **`PASS`** | Authoritative canonical target is syntactically valid and unambiguously resolvable. |
| Proven complete HTML source (`html_extraction_status == "complete"`), `canonical_count == 1`, and declaration has a proven parser error or proven resolution failure | **`FAIL`** | Search engines cannot resolve the declared canonical target to a valid HTTP(S) URL; consolidation signal is broken. |
| Canonical source is incomplete/truncated/read-errored/legacy-unverified, or single-canonical resolution has ambiguous base fallback / insufficient evidence | **`UNKNOWN`** | Diagnostic evidence is insufficient to prove compliance or failure. Never emit a false `FAIL`. |
| Proven complete HTML source (`html_extraction_status == "complete"`) and `canonical_count == 0` or `canonical_count > 1` (including `> 5` declarations) | **`NOT_APPLICABLE`** | Precondition (`canonical_count == 1`) is proven unsatisfied. Absence is evaluated by `AR-CANON-003`; multiplicity is evaluated by `AR-CANON-004`. |

*Frozen Invariant:* `WARNING` and `MANUAL_REVIEW` are **`not used`** for `AR-CANON-005`.

### 2.3 Non-Negotiable Architectural Principles

- `MISSING != FALSE`: Absence of an observation is never an observed negative value.
- `OBSERVATION != CONCLUSION`: Raw crawler and adapter observations remain strictly separated from rule verdicts.
- `SAMPLE CAP != TOTAL COUNT`: A bounded sample array (`appendCapped(..., 5)`) must never be used as the total declaration count when > 5 declarations exist.
- `URL DICTIONARY != FETCH`: Presence of a URL in `sitecrawl_urls` does not imply a completed page fetch.
- `INITIAL HTTP STATUS != TERMINAL HTTP STATUS`: Redirected pages (`len(res.Hops) > 0`) record initial 3xx status on `Page.Status` and skip document copying in `buildPage` (`internal/sitecrawl/page.go:57`).
- `SOURCE UNREADABLE != INVALID`: An unreadable, truncated, read-errored, or legacy-unverified HTML source must evaluate to `UNKNOWN`, never `FAIL`.
- `NORMALIZATION FAILURE != PROVEN SYNTAX FAILURE`: Aggregate normalization failure (`canonical_normalization_complete=false`) or an invalid `<base href>` that does not prevent valid canonical resolution (or whose fallback behavior is ambiguous) must never automatically produce `FAIL`.

---

## 3. Source-to-Evidence Provenance & Four-Layer Completeness Analysis

Inspection of `internal/sitecrawl/fetch.go`, `internal/sitecrawl/extract.go`, `internal/sitecrawl/page.go`, `internal/sitecrawl/types.go`, and `internal/audit/adapter/adapter.go` establishes the four distinct completeness layers and current implementation gaps:

| Layer | Technical Responsibility | Verified Source Location | Current Implementation Gap |
|---|---|---|---|
| **Layer 1: HTTP Response / Transport Eligibility** | Server returned an initial 2xx HTML response (`http_status` 200–299, `isHTMLContentType`, `!rendered`, `len(res.Hops) == 0`). | `fetch.go:235–253`, `page.go:57`, `adapter.go:665–926` | Redirected rows (`len(res.Hops) > 0`) skip `buildPage` document copying (`page.go:57`). Rendered pages replace raw server HTML (`adapter.go:904–954`). |
| **Layer 2: HTML Body Read Completeness** | The HTTP response body stream (`res.Body`) was read to clean `io.EOF` without mid-stream or post-parse drain transport errors. | `extract.go:172–176`, `fetch.go:280–287` | **Gap 3.1:** `extractDocument` breaks on `html.ErrorToken` without inspecting `z.Err()`.<br>**Gap 3.2:** `fetch.go:285` ignores the error return of `io.Copy(io.Discard, io.LimitReader(res.Body, drainCap))`. |
| **Layer 3: HTML Extraction Completeness** | Full HTML stream was tokenized within configured size limits without pre-parse rejection (`ErrTooLarge`) or mid-stream byte-cap truncation. | `fetch.go:245–249, 276–287`, `adapter.go:679–696, 1640–1647` | **Gap 3.3:** `io.LimitReader(res.Body, limit)` caps HTML reads at `limit = min(cap, maxHTMLBytes)`. If a document is `>= limit` bytes, `io.LimitReader` synthesizes `io.EOF` at `limit`, making a clean `limit`-byte document indistinguishable from a truncated `> limit`-byte document if the drain step fails or is not probed via `limit + 1`.<br>**Gap 3.4:** `normalizeFetchError` (`adapter.go:1640`) maps `"file-too-large"` to `fetch_error_type == ""` + `GapFetchErrorUnmappable`. |
| **Layer 4: Canonical Declaration Readability & True Count** | All `<link rel="canonical">` tags (including empty/whitespace `href`) are counted accurately without cap distortion, and raw `href` + `<base href>` attributes are preserved. | `extract.go:322–326, 500–511, 560–604` | **Gap 3.5:** `readLink` (`extract.go:502`) drops `<link rel="canonical">` when `href == ""` or whitespace-only.<br>**Gap 3.6:** `readLink` (`extract.go:510`) caps `d.Canonicals` at 5 items via `appendCapped(d.Canonicals, href, 5)` without an uncapped total counter.<br>**Gap 3.7:** `d.resolve` (`extract.go:568`) overwrites `d.Canonicals[i]` in-place and does not persist `d.BaseHref` (`page.go:60–93`). |

---

## 4. Amended Contract Decisions (`DEC-V1.9A-001`, `DEC-V1.9A-002`, `DEC-V1.9A-003`)

### 4.1 DEC-V1.9A-001 — Canonical Failure Semantics & `<base href>` Fallback (`REVISED`)

**Amendment Mandate (QA Issue #2 Resolution):**
1. Return `FAIL` **only** for proven canonical syntax or resolution errors.
2. Verify `<base href>` fallback behavior against **WHATWG HTML Standard § 4.2.3** and **RFC 3986 § 5.1 / § 5.2**: an invalid `<base href>` does **not** automatically make a relative canonical unresolvable, and must never automatically produce `FAIL`.
3. Strictly distinguish **proven resolution failure (`FAIL`)**, **successful fallback or independent resolution (`PASS`)**, and **ambiguous / insufficient evidence (`UNKNOWN`)**.

#### 4.1.1 Standards Verification: How `<base href>` Interacts with Canonical Resolution

1. **WHATWG HTML Standard § 4.2.3 (`The base element` — setting the frozen base URL):**
   - A document's **fallback base URL** is the document's address (the terminal fetched URL `FinalURL`, which is guaranteed to be a valid absolute HTTP(S) URL for any crawled 2xx HTML page).
   - When the first `<base href="...">` element with an `href` attribute is processed, the user agent parses the `href` attribute value relative to the document's fallback base URL:
     - If parsing **succeeds**, the resulting URL Record becomes the document's **frozen base URL** (unless prohibited by scheme/data rules).
     - If parsing **fails**, the document's base URL **falls back to the document's fallback base URL (`FinalURL`)**!
2. **RFC 3986 § 5.2 (`Relative Resolution`):**
   - If the target reference `R` (`<link rel="canonical" href="R">`) has a defined scheme (`R.IsAbs() == true`), `ResolveReference` ignores the base URI entirely.
   - If `R` is a relative reference (`R.IsAbs() == false`) and `<base href="">` (empty) or `<base href="#frag">` (fragment-only) is present, resolving `<base href>` against `FinalURL` yields `FinalURL` itself, and resolving `R` against `FinalURL` succeeds cleanly.
3. **Why Go's `url.Parse` vs. WHATWG URL Parser Creates an Ambiguity Zone on Malformed `<base href>`:**
   - Suppose a page at `https://example.com/dir/page.html` declares `<base href="http://[::1">` (or `<base href="http://example.com:badport/base/">`) and a relative canonical `<link rel="canonical" href="target.html">`.
   - Under WHATWG HTML § 4.2.3, a browser/Googlebot parses `<base href="http://[::1">`, detects a URL parse failure, **ignores `<base>`**, falls back to `https://example.com/dir/page.html`, and resolves `target.html` to `https://example.com/dir/target.html`!
   - Conversely, if `<base href="https://cdn.example.com/assets/ ">` contains characters or syntax where Go's `net/url` and WHATWG URL parsing diverge, or if the author intended `<base href>` to rebase relative URLs to another directory/host and `<base href>` failed to parse, the audit engine cannot prove whether the resulting target matches the effective canonical URL or failed resolution.
   - Therefore, when a **relative** canonical is paired with a `<base href>` that fails RFC 3986 `url.Parse` (or produces an invalid host under `net/url`), emitting `FAIL` would falsely condemn a canonical that browsers/Googlebot resolve via fallback to `FinalURL`, while emitting `PASS` via fallback would silently mask a broken `<base href>` rebase! The only truthful verdict is **`UNKNOWN` (`insufficient_evidence:ambiguous_base_fallback`)**.

#### 4.1.2 Complete `<base href>` & Canonical Resolution Matrix (Scenario 6 Sub-Cases)

| Sub-Case | Raw Canonical `href` | Document `<base href>` State | Resolution & Standards Outcome | `canonical_syntax_status` / `canonical_parse_error` | `AR-CANON-005` Verdict |
|---|---|---|---|---|---|
| **6a-i: Proven Base-Induced Resolution Failure** | Relative reference (`href="sub/page.html"` or `href="/page.html"`) | `<base href="...">` parses cleanly as a URI with a **non-HTTP(S) scheme** (e.g. `<base href="ftp://files.example.com/dir/">`, `<base href="javascript:void(0)">`, `<base href="mailto:user@example.com">`, `<base href="data:text/html,hi">`, `<base href="file:///path/">`) | `<base href>` is a valid URI under RFC 3986 / WHATWG, so it sets the document's frozen base URL to a non-HTTP(S) scheme (or opaque path cannot-be-a-base URL). Resolving the relative canonical against that frozen base URL produces a non-HTTP(S) target (`ftp://...`) or cannot-be-a-base resolution failure. | `resolution_error` / `resolution_error:unsupported_scheme` (or `resolution_error:non_hierarchical_base`) | **`FAIL`** |
| **6a-ii: Unparseable / Malformed `<base href>` with Relative Canonical** | Relative reference (`href="sub/page.html"`) | `<base href="...">` is non-empty and **fails RFC 3986 `url.Parse` or host validation** (e.g. `<base href="http://[::1">`, `<base href="https://example.com:badport/">`, `<base href="http:///path/">`) | Under WHATWG § 4.2.3, a failed `<base href>` parse falls back to the document `FinalURL`, whereas under strict static base resolution the intended base rebase failed. Static RFC 3986 parsing cannot prove whether effective resolution succeeded via fallback or broke the intended target. | `insufficient_evidence` / `insufficient_evidence:ambiguous_base_fallback` | **`UNKNOWN`** *(Never automatic `FAIL`)* |
| **6b-i: Absolute Canonical with Any Invalid `<base href>`** | Valid absolute HTTP(S) URL (`href="https://example.com/canonical"`) | Any invalid, malformed, or non-HTTP(S) `<base href>` (`<base href="http://[::1">`, `<base href="javascript:alert(1)">`, `<base href="ftp://...">`) | Per RFC 3986 § 5.2 and WHATWG URL standard, an absolute URL with `http`/`https` scheme and valid host ignores `<base href>` completely. | `valid` / *(none)* | **`PASS`** |
| **6b-ii: Relative Canonical with Empty / Fragment / No-Href `<base>` (Deterministic Fallback)** | Valid relative reference (`href="sub/page.html"` or `href="/about"`) | `<base target="_blank">` (no `href`), `<base href="">`, or `<base href="#section">` on a page with valid HTTP(S) `FinalURL` | Per RFC 3986 § 5.2 and WHATWG § 4.2.3, resolving `""` or `"#section"` against `FinalURL` yields the document's `FinalURL` (stripped of fragment). Resolving the relative canonical against that base deterministically produces a valid HTTP(S) URL. | `valid` / *(none)* | **`PASS`** |
| **6c: Relative Canonical with Missing / Conflicting Base Context** | Relative reference (`href="sub/page.html"`) | Legacy SQLite row lacking `BaseHrefRaw` provenance (`missing_legacy`), unparseable page `url_identity`, or multiple `<base href>` tags with conflicting values (`BaseHrefCount > 1`) | Base context is missing or contradictory; deterministic resolution cannot be verified. | `insufficient_evidence` / `insufficient_evidence:missing_base_context` | **`UNKNOWN`** |

#### 4.1.3 Three-Tier Diagnostic Classification (`canonical_syntax_status` & `canonical_parse_error`)

When `html_extraction_status == "complete"` and `canonical_count == 1`, single-canonical evaluation classifies the declaration into:

| `canonical_syntax_status` | `canonical_parse_error` Code | Proven Trigger Condition | `AR-CANON-005` Verdict |
|---|---|---|---|
| **`valid`** | *(none / omitted)* | Canonical `href` is non-empty, parses cleanly, and resolves (directly if absolute `http`/`https`, or via valid `<base href>` / deterministic `FinalURL` fallback if relative) to a valid HTTP(S) URL with non-empty host. | **`PASS`** |
| **`parser_error`** | `parser_error:empty_href` | `<link rel="canonical">` tag is present (`canonical_count == 1`), and raw `href` attribute is missing, `""`, or whitespace-only. | **`FAIL`** |
| **`parser_error`** | `parser_error:malformed_url` | Raw canonical `href` is non-empty and fails RFC 3986 `url.Parse` (e.g. control chars, bad port `https://example.com:abc`, unclosed IPv6 `http://[::1`, invalid percent-encoding `%ZZ`). | **`FAIL`** |
| **`resolution_error`** | `resolution_error:unsupported_scheme` | Canonical `href` (or its resolved target via a valid non-HTTP(S) `<base href>` such as `ftp://`) has a scheme other than `http` or `https` (`javascript:`, `mailto:`, `tel:`, `ftp:`, `data:`, `file:`). | **`FAIL`** |
| **`resolution_error`** | `resolution_error:missing_host` | Canonical `href` has `http`/`https` scheme or scheme-relative `//` syntax with an empty or invalid host (`https:///path`, `http://:80/path`, `///path`). | **`FAIL`** |
| **`resolution_error`** | `resolution_error:non_hierarchical_base` | Canonical `href` is relative (`!parsed.IsAbs()`) and `<base href>` is a valid URI with an opaque/non-hierarchical cannot-be-a-base scheme (`mailto:a@b`, `javascript:void(0)`, `data:text/plain,x`) against which RFC 3986 / WHATWG relative resolution cannot produce a valid URL. | **`FAIL`** |
| **`insufficient_evidence`** | `insufficient_evidence:ambiguous_base_fallback` | Canonical `href` is relative (`!parsed.IsAbs()`) and `<base href>` is non-empty but fails RFC 3986 `url.Parse` or host validation (e.g. `<base href="http://[::1">` or `<base href="http:///dir/">`), creating ambiguity between WHATWG fallback to `FinalURL` and broken base rebase. | **`UNKNOWN`** |
| **`insufficient_evidence`** | `insufficient_evidence:missing_base_context` | Canonical `href` is relative and page `url_identity` is unparseable/non-absolute, or multiple conflicting `<base href>` tags exist (`BaseHrefCount > 1`), or legacy snapshot lacks base provenance. | **`UNKNOWN`** |
| **`insufficient_evidence`** | `insufficient_evidence:unclassified_normalization_failure` | `canonical_normalization_complete == "false"` on a single canonical declaration without any proven `parser_error:*` or `resolution_error:*`. | **`UNKNOWN`** |

---

### 4.2 DEC-V1.9A-002 — Verifiable HTML Extraction Completeness (`REVISED`)

**Amendment Mandate (QA Issue #3 Resolution):**
1. Replace the insufficient condition `counter.n < limit || extra == 0`.
2. Define a verifiable mechanism to distinguish **clean EOF**, **size-limit truncation**, **read errors**, and **missing legacy evidence**.
3. Explicitly account for errors while reading/draining the remaining response body.
4. Preserve `UNKNOWN` for incomplete or unverified evidence.

#### 4.2.1 Why `counter.n < limit || extra == 0` Was Insufficient

In `internal/sitecrawl/fetch.go:280–287`:
```go
counter := &countingReader{r: io.LimitReader(res.Body, limit)}
doc := extractDocument(counter, base)
out.Doc = doc
out.Size = counter.n
if rest, _ := io.Copy(io.Discard, io.LimitReader(res.Body, drainCap)); rest > 0 {
    out.Size += rest
}
```
The condition `counter.n < limit || extra == 0` had two fatal flaws:
1. **Boundary EOF Blindspot (`counter.n == limit` + Drain Read Error):** When the HTML body is `>= limit` bytes, `io.LimitReader(res.Body, limit)` stops reading `res.Body` at `limit` bytes and returns a synthetic `io.EOF` to `html.Tokenizer`. If the subsequent `io.Copy(io.Discard, ...)` immediately encounters a network error (e.g. TCP reset, timeout, or chunked-encoding framing error) before reading its first byte, `io.Copy` returns `rest = 0` (`extra == 0`) and a non-nil `err` (which `fetch.go:285` ignored with `_`). Under `counter.n < limit || extra == 0`, `extra == 0` evaluated to `true`, falsely classifying a truncated + read-errored stream as `"complete"`!
2. **`countingReader` Error Blindspot:** `countingReader` in `fetch.go` only counted bytes (`n int64`) without recording the terminal `err` returned by the underlying `res.Body.Read`.

#### 4.2.2 Verifiable `limit + 1` Bounded Probe Reader & Drain Error Contract

To distinguish **clean EOF**, **size-limit truncation**, and **read errors** with mathematical certainty, V1.9c must implement the following two-part mechanism in `internal/sitecrawl/fetch.go` and `internal/sitecrawl/extract.go`:

1. **`boundedProbeReader` (`limit + 1` Probe on `res.Body`):**
   Instead of `io.LimitReader(res.Body, limit)`, wrap `res.Body` in a `boundedProbeReader` configured with `maxBytes = limit`:
   - `boundedProbeReader.Read(p)` reads from `res.Body` up to `limit + 1` total bytes:
     - Tracks `n int64` (bytes delivered to caller, capped at `limit`).
     - Tracks `exceededLimit bool`: if `res.Body` delivers the `(limit + 1)`-th byte, sets `exceededLimit = true` and returns `io.EOF` to the tokenizer so the tokenizer never processes more than `limit` bytes.
     - Tracks `readErr error`: records any non-nil error returned by `res.Body.Read`. When `res.Body.Read` returns `io.EOF` with `n <= limit` and `!exceededLimit`, sets `reachedEOF = true`.
   - Why `limit + 1` solves the boundary case: If a document is **exactly `limit` bytes** long, the tokenizer reads the `limit` bytes and calls `Read` one more time to get the next token; `boundedProbeReader` attempts to read the `(limit + 1)`-th byte from `res.Body` and receives either:
     - `(0, io.EOF)` -> `reachedEOF = true`, `exceededLimit = false`, `readErr = io.EOF` (**Proven Clean EOF**).
     - `(>=1, _)` -> `exceededLimit = true` (**Proven Size-Limit Truncation**).
     - `(0, nonEOFError)` -> `readErr = nonEOFError` (**Proven Read Error**).
2. **Explicit Tokenizer `z.Err()` & Post-Parse Drain Error Accounting:**
   - In `extractDocument` (`extract.go:172–176`), when `tt == html.ErrorToken`, record `d.TokenizerErr = z.Err()`.
   - In `fetch.go:285–287`, capture both `rest` and `drainErr`:
     ```go
     var drainErr error
     if !probe.reachedEOF {
         var rest int64
         rest, drainErr = io.Copy(io.Discard, io.LimitReader(res.Body, drainCap))
         out.Size += rest
     }
     ```
     *(Note: When `probe.reachedEOF` is already `true`, `res.Body` has already returned `io.EOF`, so no post-parse drain read is needed; if a drain runs when `!probe.reachedEOF`, any non-nil `drainErr` is captured explicitly.)*

#### 4.2.3 Deterministic State Transition Table for `ExtractionStatus` / `html_extraction_status`

| `html_extraction_status` | Exact Verifiable Condition | Meaning | `AR-CANON-005` Verdict |
|---|---|---|---|
| **`"complete"`** *(Proven Clean EOF)* | **All** of the following hold:<br>1. `res.StatusCode >= 200 && res.StatusCode < 300` and `isHTML(out.ContentType)` and `!opts.Render` and `len(res.Hops) == 0`.<br>2. Pre-parse size gate did not trigger (`res.ContentLength <= cap`).<br>3. `probe.exceededLimit == false` (`res.Body` did not have a `(limit + 1)`-th byte).<br>4. `probe.reachedEOF == true` and `(probe.readErr == nil \|\| probe.readErr == io.EOF)`.<br>5. `doc.TokenizerErr == io.EOF`.<br>6. `drainErr == nil` and `out.ErrorType == ""`. | Every byte of the HTTP 2xx HTML response body was read to genuine `res.Body` `io.EOF` and tokenized without size-cap truncation or transport error. | Eligible for `PASS`, `FAIL`, or `NOT_APPLICABLE`. |
| **`"truncated_size_limit"`** *(Proven Size-Limit Truncation)* | Either:<br>1. Pre-parse `Content-Length` gate triggered: `res.ContentLength > cap` (`out.ErrorType == ErrTooLarge` / `"file-too-large"`), **OR**<br>2. Stream probe read a `(limit + 1)`-th byte: `probe.exceededLimit == true` (with no preceding transport read error). | HTML response body exceeded `min(cap, maxHTMLBytes)` (`10 MiB` or configured `MaxSize`), so HTML extraction was skipped or truncated. | **`UNKNOWN`** |
| **`"read_error"`** *(Proven Read / Transport Error)* | Any of the following occurred:<br>1. Transport fetch error before body read (`out.ErrorType != ""` and `out.ErrorType != ErrTooLarge`), **OR**<br>2. Mid-stream read error on `res.Body`: `probe.readErr != nil && probe.readErr != io.EOF`, **OR**<br>3. Tokenizer exited with non-EOF error: `doc.TokenizerErr != nil && doc.TokenizerErr != io.EOF`, **OR**<br>4. Post-parse drain encountered a read error: `drainErr != nil` when `!probe.reachedEOF`, **OR**<br>5. `!probe.reachedEOF && !probe.exceededLimit` (stream ended without clean `io.EOF`). | Network or stream error interrupted reading or draining the response body. | **`UNKNOWN`** |
| **`"missing_legacy"`** *(Missing Legacy / Unextracted Evidence)* | `pr.page.ExtractionStatus == ""` (legacy SQLite row created before V1.9c), OR `rendered == "true"`, OR `len(pr.page.Redirects) > 0` (initial 3xx redirect row), OR non-2xx / non-HTML response where static HTML extraction did not run. | Static HTML extraction completeness was not recorded or is inapplicable. | **`UNKNOWN`** |

---

### 4.3 DEC-V1.9A-003 — Raw Canonical Provenance, True Total Count & Sample Cap (`ACCEPTED WITH CONDITIONS`)

**Amendment Mandate (QA Issue #1 Resolution):**
1. Fix the contract so `canonical_count` reflects the **actual total number of `<link rel="canonical">` declarations** in the document, including empty `href` values.
2. **Never** use `len(RawCanonicals)` (which is capped at 5 samples) as `canonical_count`.
3. Define a separate uncapped declaration counter (`CanonicalTotalCount`) and preserve truncation/sample-cap evidence (`canonical_sample_truncated`, `GapCanonicalSampleCapped`).
4. Define explicit acceptance criteria and cross-rule behavior for pages with **more than 5 canonical declarations** (`canonical_count > 5`).

#### 4.3.1 Separation of Uncapped Total Count from Capped Sample Slices

In `internal/sitecrawl/extract.go:510`, SiteCrawl bounds per-page slice allocations to prevent pathological HTML documents from bloating SQLite JSON payloads (`const maxCanonicals = 5`). To keep memory/storage bounded while preserving 100% counting accuracy:

| `sitecrawl.Page` JSON Field | Type & Cap | Exact Semantics | Adapter Mapping (`SubjectURL` / `EvidenceSnapshot`) |
|---|---|---|---|
| **`CanonicalTotalCount`** (`json:"canonical_total_count,omitempty"`) | `int` (**Uncapped**: `0..N`) | Incremented by `1` on **every** `<link>` element whose `rel` token list contains `"canonical"` (including missing `href`, `href=""`, whitespace-only `href="   "`, relative, absolute, or malformed), with **no upper cap**. | Emitted as **`canonical_count`** (`strconv.Itoa(pr.page.CanonicalTotalCount)`) when `pr.page.ExtractionStatus == "complete"`, and stored in `CanonicalObservation.CanonicalCount`. |
| **`RawCanonicals`** (`json:"raw_canonicals,omitempty"`) | `[]string` (**Capped at 5 samples**) | Stores the raw `attrs["href"]` string (with missing/empty stored as `""`) for the first up to `5` `<link rel="canonical">` declarations in document order (`appendCapped(d.RawCanonicals, rawHref, 5)`). | Stored in `CanonicalObservation.RawValues`. When `CanonicalTotalCount == 1`, `RawCanonicals[0]` is emitted as **`canonical_raw`**. |
| **`Canonicals`** (`json:"canonicals,omitempty"`) & **`Canonical`** (`json:"canonical,omitempty"`) | `[]string` (**Capped at 5 non-empty samples**) & `string` | Preserves existing SiteCrawl behavior: stores up to `5` **non-empty** post-absolutized canonical strings (`Page.Canonical = first(d.Canonicals)`). | Emitted as **`canonical_resolved`** observations (consumed by `AR-CANON-003`). |
| **`BaseHrefRaw`** (`json:"base_href_raw,omitempty"`) & **`BaseHrefCount`** (`json:"base_href_count,omitempty"`) | `string` & `int` (**Uncapped count**) | `BaseHrefCount` counts every `<base>` tag with an `href` attribute in the document (`0..N`); `BaseHrefRaw` stores the raw `href` of the first `<base href>` tag. | Used by Adapter's canonical resolution classifier to evaluate Scenario 6a/6b/6c. |
| **`ExtractionStatus`** (`json:"extraction_status,omitempty"`) | `string` | `"complete"`, `"truncated_size_limit"`, or `"read_error"`. | Emitted as **`html_extraction_status`** on `SubjectURL` (`"missing_legacy"` if empty). |

#### 4.3.2 Sample-Cap Truncation Evidence When `CanonicalTotalCount > 5`

When a page contains more than 5 `<link rel="canonical">` declarations (`pr.page.CanonicalTotalCount > 5`, so `len(pr.page.RawCanonicals) == 5 < pr.page.CanonicalTotalCount`):
1. **`canonical_count` Observation:** Emits the exact uncapped total (e.g. `"6"`, `"12"`), **never** `"5"`.
2. **`canonical_sample_truncated` Observation:** Adapter emits `canonical_sample_truncated = "true"` on `SubjectURL` (and `"false"` when `1 <= CanonicalTotalCount <= 5`).
3. **`EvidenceGap` Record:** Adapter appends an `EvidenceGap` with `GapCode: GapCanonicalSampleCapped` (`"GAP_CANONICAL_SAMPLE_CAPPED"`), recording that `CanonicalTotalCount` declarations were observed while only the first 5 raw/resolved samples were retained in `Page`.
4. **Effect on `canonical_normalization_complete` & Downstream Rules (`CanonicalTotalCount > 5`):**
   - Because declarations `6..N` were not retained in `RawCanonicals` / `Canonicals`, the Adapter **cannot** verify whether declarations `6..N` normalized cleanly or to the same URL as declarations `1..5`, and must never fabricate `canonical_normalization_complete = "true"` for unretained declarations.
   - Instead, the Adapter truthfully emits `canonical_normalization_complete = "false"`, omits `canonical_distinct_normalized_count`, and emits `canonical_normalized_target` for each valid normalized target among the up to 5 retained samples.
   - In `AR-CANON-004` (`internal/audit/engine/canon004.go`):
     - **Case A (`> 5` tags, and retained samples contain `>= 2` distinct valid `canonical_normalized_target` values):** Multiplicity of distinct canonical targets (`>= 2`) is already proven by the retained sample regardless of what unretained tags `6..N` contain. `AR-CANON-004` returns **`FAIL`** (while `AR-CANON-006` returns `UNKNOWN`, `AR-CANON-007` returns `NOT_APPLICABLE`, and `AR-CANON-005` returns `NOT_APPLICABLE`).
     - **Case B (`> 5` tags, and retained samples contain `< 2` distinct valid `canonical_normalized_target` values, e.g. all 5 retained samples point to the same target):** Because tag #6 was not retained, `AR-CANON-004` cannot determine whether all targets are identical (`WARNING`) or conflicting (`FAIL`), and therefore returns **`UNKNOWN`**.

---

## 5. Regression Requirements for `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, and `AR-CANON-007`

| Rule ID | Consumed Observations (`internal/audit/engine/`) | Behavioral Invariant to Preserve | Required Regression Test Cases in V1.9c/d |
|---|---|---|---|
| **`AR-CANON-003`** *(Canonical declaration missing)* | `url_identity`, `http_status`, `content_type`, `rendered`, `canonical_resolved` (`canon003.go:65–238`) | - `PASS` when 2xx HTML has >= 1 non-blank `canonical_resolved`.<br>- `WARNING` when unrendered 2xx HTML has 0 non-blank `canonical_resolved`.<br>- `UNKNOWN` when `rendered == "true"` and no raw canonical.<br>- `NOT_APPLICABLE` on non-2xx or non-HTML. | 1. Existing `canon003_test.go` suite passes unchanged.<br>2. Single empty `<link rel="canonical" href="">`: `canonical_resolved` has 0 non-blank entries -> `AR-CANON-003` returns `WARNING` while `AR-CANON-005` returns `FAIL`.<br>3. Page with `6` canonical tags (`> 5` cap) -> at least 1 non-blank `canonical_resolved` -> `AR-CANON-003` returns `PASS`. |
| **`AR-CANON-004`** *(Multiple canonical declarations)* | `url_identity`, `canonical_count`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count` (`canon004.go:58–310`) | - `NOT_APPLICABLE` when `canonical_count == 0`.<br>- `PASS` when `canonical_count == 1`.<br>- `WARNING` when `1 < canonical_count <= 5`, `complete == "true"`, `distinct == 1`.<br>- `FAIL` when (`1 < canonical_count <= 5`, `complete == "true"`, `distinct > 1`) OR (`canonical_count > 1`, `complete == "false"`, and retained `canonical_normalized_target` observations already contain `>= 2` distinct targets).<br>- `UNKNOWN` when `canonical_count > 1`, `complete == "false"`, and retained `canonical_normalized_target` observations contain `< 2` distinct targets. | 1. Existing `canon004_test.go` suite passes unchanged.<br>2. Single empty canonical (`href=""`) -> `canonical_count == 1` -> `AR-CANON-004` returns `PASS`.<br>3. Two canonicals (`[valid, ""]`) -> `canonical_count == 2`, `complete == "false"`, 1 distinct target -> `AR-CANON-004` returns `UNKNOWN`.<br>4. Six identical canonicals (`CanonicalTotalCount == 6`, sample capped at 5, 1 distinct target in sample) -> `canonical_count == 6`, `canonical_sample_truncated == "true"`, `complete == "false"` -> `AR-CANON-004` returns `UNKNOWN` (never a false `WARNING`).<br>5. Six canonicals with `>= 2` distinct valid targets in the 5 retained samples -> `canonical_count == 6`, `canonical_sample_truncated == "true"`, `complete == "false"` -> `AR-CANON-004` returns `FAIL`.<br>6. Legacy SQLite row without `CanonicalTotalCount` -> falls back to `len(Page.Canonicals)` with zero regression. |
| **`AR-CANON-006`** *(Canonical target returns final 200)* | `url_identity`, `canonical_count`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count`, `canonical_target_subject_ref`, target `http_status` (`canon006.go:61–480`) | - `NOT_APPLICABLE` when `canonical_count == 0` or (`complete == "true"` and `distinct > 1`).<br>- `UNKNOWN` when `canonical_count > 0` and `complete == "false"`, or `canonical_target_subject_ref` absent.<br>- `PASS` when unique target `http_status == 200`.<br>- `FAIL` when unique target `http_status` in `300..599`. | 1. Existing `canon006_test.go` suite passes unchanged.<br>2. Single invalid/empty canonical (`canonical_count == 1`, `complete == "false"`) -> `AR-CANON-006` returns `UNKNOWN`.<br>3. Page with `6` canonicals (`CanonicalTotalCount == 6`, `complete == "false"`) -> `AR-CANON-006` returns `UNKNOWN`.<br>4. Valid absolute canonical on page with invalid `<base href>` -> `complete == "true"`, `distinct == 1` -> `AR-CANON-006` evaluates target status normally. |
| **`AR-CANON-007`** *(Canonical target is not noindex)* | `url_identity`, `canonical_count`, `canonical_normalized_target`, `canonical_normalization_complete`, `canonical_distinct_normalized_count`, `canonical_target_subject_ref`, target `http_status`, target `effective_noindex` (`canon007.go:56–511`) | - `NOT_APPLICABLE` when `canonical_count == 0`, `complete == "false"`, `distinct != 1`, `canonical_target_subject_ref` absent, or target `http_status != 200`.<br>- `PASS` when target `http_status == 200` and `effective_noindex == "false"`.<br>- `FAIL` when target `http_status == 200` and `effective_noindex == "true"`. | 1. Existing `canon007_test.go` suite passes unchanged.<br>2. Single invalid/empty canonical (`canonical_count == 1`, `complete == "false"`) -> `AR-CANON-007` returns `NOT_APPLICABLE`.<br>3. Page with `6` canonicals (`CanonicalTotalCount == 6`, `complete == "false"`) -> `AR-CANON-007` returns `NOT_APPLICABLE`. |

---

## 6. Corrected 22-Scenario QA & Verdict Decision Matrix

The 22-scenario matrix below incorporates all three QA fixes (uncapped `canonical_count` + `> 5` declarations in Scenarios 11 & 12, standards-based `<base href>` resolution in Scenarios 6a-i / 6a-ii / 6b-i / 6b-ii / 6c, and `limit + 1` probe + drain error checking in Scenarios 13–15):

| # | Scenario | Expected Verdict (Amended Contract) | Required Evidence State (`SubjectURL`) | Current Availability at `3449d003` | Evidence Currently Missing | Required V1.9c/d Prerequisite |
|---|---|---|---|---|---|---|
| **1** | No canonical declaration (`CanonicalTotalCount == 0` in proven clean-EOF 2xx HTML) | **`NOT_APPLICABLE`** | `html_extraction_status == "complete"`, `canonical_count == "0"` | Partial (`canonical_count == "0"`, but clean EOF and empty-href absence unproven) | `html_extraction_status == "complete"` via `limit+1` probe, empty-href counting in `CanonicalTotalCount` | V1.9c: `boundedProbeReader` + `CanonicalTotalCount` |
| **2** | Exactly one valid absolute HTTP canonical URL (`http://example.com/page`) | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "http://example.com/page"`, `canonical_syntax_status == "valid"`, `canonical_normalized_target == "http://example.com/page"`, `canonical_normalization_complete == "true"` | Partial (`canonical_normalized_target` and `complete == "true"` available) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status` | V1.9c + V1.9d (`evaluateCANON005`) |
| **3** | Exactly one valid absolute HTTPS canonical URL (`https://example.com/page`) | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "https://example.com/page"`, `canonical_syntax_status == "valid"`, `canonical_normalized_target == "https://example.com/page"`, `canonical_normalization_complete == "true"` | Partial (`canonical_normalized_target` and `complete == "true"` available) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status` | V1.9c + V1.9d (`evaluateCANON005`) |
| **4** | Exactly one valid relative canonical URL (`href="/about"`) with no `<base href>` | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "/about"`, `canonical_syntax_status == "valid"`, `canonical_normalized_target == "https://example.com/about"`, `canonical_normalization_complete == "true"` | Partial (resolved target available; raw `"/about"` overwritten at `extract.go:568`) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status` | V1.9c: Preserve `RawCanonicals` before `resolve()` |
| **5** | Relative canonical URL (`href="sub"`) with valid HTTP(S) `<base href="https://example.com/dir/">` | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "sub"`, `base_href_raw == "https://example.com/dir/"`, `canonical_syntax_status == "valid"`, `canonical_normalized_target == "https://example.com/dir/sub"` | Partial (resolved target available; `BaseHref` and raw `"sub"` not persisted) | `html_extraction_status`, `canonical_raw`, `base_href_raw`, `canonical_syntax_status` | V1.9c: Persist `RawCanonicals`, `BaseHrefRaw`, `BaseHrefCount` |
| **6** | **Base-Href Standards & Fallback Suite (5 sub-cases per QA Issue #2):**<br>- **6a-i (Proven Non-HTTP(S) Base Resolution Failure):** Relative canonical (`href="sub.html"`) + valid non-HTTP(S) `<base href="ftp://files.example.com/dir/">` or opaque `<base href="mailto:a@b">` / `<base href="javascript:void(0)">`<br>- **6a-ii (Unparseable `<base href>` with Relative Canonical):** Relative canonical (`href="sub.html"`) + malformed `<base href="http://[::1">` or `<base href="http:///dir/">` (WHATWG fallback vs broken rebase ambiguity)<br>- **6b-i (Absolute Canonical + Any Invalid `<base href>`):** Absolute canonical (`href="https://example.com/page"`) + `<base href="http://[::1">` or `<base href="javascript:void(0)">`<br>- **6b-ii (Deterministic Fallback to `FinalURL`):** Relative canonical (`href="sub.html"`) + `<base target="_blank">` (no `href`), `<base href="">`, or `<base href="#frag">`<br>- **6c (Missing / Conflicting Base Context):** Relative canonical (`href="sub.html"`) + `BaseHrefCount > 1` (conflicting bases), unparseable page `url_identity`, or legacy snapshot | - **6a-i:** **`FAIL`**<br>- **6a-ii:** **`UNKNOWN`**<br>- **6b-i:** **`PASS`**<br>- **6b-ii:** **`PASS`**<br>- **6c:** **`UNKNOWN`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, plus:<br>- **6a-i:** `canonical_syntax_status == "resolution_error"`, `canonical_parse_error` in `{"resolution_error:unsupported_scheme", "resolution_error:non_hierarchical_base"}`<br>- **6a-ii:** `canonical_syntax_status == "insufficient_evidence"`, `canonical_parse_error == "insufficient_evidence:ambiguous_base_fallback"`<br>- **6b-i & 6b-ii:** `canonical_syntax_status == "valid"`, `canonical_normalized_target != ""`<br>- **6c:** `canonical_syntax_status == "insufficient_evidence"`, `canonical_parse_error == "insufficient_evidence:missing_base_context"` | **No** (`extract.go:563` silently falls back to `base` on `err != nil \|\| b.Host == ""`, turning 6a-i with `mailto:`/`javascript:` or 6a-ii with `http://[::1` into an unverified `PASS`!) | `BaseHrefRaw`, `BaseHrefCount`, Adapter base-resolution classifier distinguishing 6a-i (`FAIL`), 6a-ii (`UNKNOWN`), 6b-i/ii (`PASS`), and 6c (`UNKNOWN`) | V1.9c: Persist `BaseHrefRaw` & `BaseHrefCount`; implement standards-compliant base classifier in Adapter |
| **7** | Canonical directly using unsupported scheme (`javascript:void(0)`, `mailto:a@b.com`, `ftp://files.example.com/a`) | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_syntax_status == "resolution_error"`, `canonical_parse_error == "resolution_error:unsupported_scheme"` | Partial (only untyped `canonical_normalization_complete == "false"` is emitted) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status`, `canonical_parse_error` | V1.9c: Classifier emits `resolution_error:unsupported_scheme` |
| **8** | Canonical containing malformed URL syntax (`http://[::1`, `https://example.com:badport`) or missing host (`http:///path`, `///path`) | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_syntax_status` in `{"parser_error", "resolution_error"}`, `canonical_parse_error` in `{"parser_error:malformed_url", "resolution_error:missing_host"}` | Partial (only untyped `canonical_normalization_complete == "false"` is emitted) | `html_extraction_status`, `canonical_raw`, `canonical_syntax_status`, `canonical_parse_error` | V1.9c: Classifier emits `parser_error:malformed_url` / `resolution_error:missing_host` |
| **9** | Empty canonical declaration (`<link rel="canonical" href="">` or `<link rel="canonical">`) | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == ""`, `canonical_syntax_status == "parser_error"`, `canonical_parse_error == "parser_error:empty_href"` | **No** (`extract.go:502` drops empty `href`, emitting `canonical_count == "0"`) | `CanonicalTotalCount` & `RawCanonicals` capture of empty `href`, `parser_error:empty_href` | V1.9c: Increment `CanonicalTotalCount` and append `""` to `RawCanonicals` |
| **10** | Whitespace-only canonical declaration (`<link rel="canonical" href="   ">`) | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_raw == "   "`, `canonical_syntax_status == "parser_error"`, `canonical_parse_error == "parser_error:empty_href"` | **No** (`extract.go:501–502` trims and drops whitespace `href`, emitting `canonical_count == "0"`) | `CanonicalTotalCount` & `RawCanonicals` capture of whitespace `href`, `parser_error:empty_href` | V1.9c: Increment `CanonicalTotalCount` and append raw whitespace string to `RawCanonicals` |
| **11** | Multiple distinct canonical declarations:<br>- **11a:** `2..5` declarations (including `[valid, ""]` or `[valid1, valid2]`)<br>- **11b:** `> 5` declarations (`CanonicalTotalCount == 7`, sample capped at 5) | **`NOT_APPLICABLE`** | `html_extraction_status == "complete"`, `canonical_count == "2"` (11a) or `canonical_count == "7"` + `canonical_sample_truncated == "true"` (11b) | Partial (broken today when `> 5` tags exist because count is capped at 5, or when 1 of 2 tags has `href=""`) | Uncapped `CanonicalTotalCount`, empty-href counting, `canonical_sample_truncated` | V1.9c (`CanonicalTotalCount`) + V1.9d (`canonical_count > 1` -> `NOT_APPLICABLE`) |
| **12** | Duplicate identical canonical declarations:<br>- **12a:** `2..5` identical declarations<br>- **12b:** `> 5` declarations where first 5 samples are identical (`CanonicalTotalCount == 6`, sample capped at 5) | **`NOT_APPLICABLE`** *(in `AR-CANON-005`; while `AR-CANON-004` returns `WARNING` for 12a and `UNKNOWN` for 12b)* | `html_extraction_status == "complete"`, `canonical_count > "1"` (`"6"` for 12b with `canonical_sample_truncated == "true"` and `canonical_normalization_complete == "false"`) | Partial (today 12b emits `canonical_count == "5"` and false `WARNING` in `AR-CANON-004`!) | Uncapped `CanonicalTotalCount`, `canonical_sample_truncated`, `complete = "false"` when `CanonicalTotalCount > 5` | V1.9c + V1.9d |
| **13** | Canonical source unavailable or legacy-unverified:<br>- **13a:** Non-HTML `content_type` (`application/pdf`, `image/png`)<br>- **13b:** Legacy SQLite row lacking `ExtractionStatus` (`html_extraction_status == "missing_legacy"`) | **`UNKNOWN`** | `html_extraction_status == "missing_legacy"` (or non-HTML / missing `html_extraction_status`) | Partial (`content_type` available; legacy vs complete distinction absent) | `html_extraction_status == "missing_legacy"` | V1.9c + V1.9d: Require `html_extraction_status == "complete"` |
| **14** | HTTP fetch failed (DNS/TLS/timeout/connect error, HTTP 3xx/4xx/5xx, or redirect hop row) | **`UNKNOWN`** | `fetch_error_type != ""` or `http_status` not in `200..299` or `html_extraction_status` in `{"read_error", "missing_legacy"}` | Yes for mappable errors and non-200 status | `html_extraction_status` integration | V1.9c + V1.9d: Return `UNKNOWN` when `html_extraction_status != "complete"` |
| **15** | HTTP 200 HTML response, but extraction incomplete (3 sub-cases per QA Issue #3):<br>- **15a:** Mid-stream `res.Body` / `z.Err()` read error (`html_extraction_status == "read_error"`)<br>- **15b:** Size-limit truncation via `ContentLength > cap` (`file-too-large`) or `limit + 1` probe (`probe.exceededLimit == true`) (`html_extraction_status == "truncated_size_limit"`)<br>- **15c:** Exact `limit`-byte boundary or post-parse drain error (`drainErr != nil`) (`html_extraction_status == "read_error"`) | **`UNKNOWN`** | `http_status == "200"`, `html_extraction_status == "read_error"` (15a, 15c) or `"truncated_size_limit"` (15b) | **No** (All three sub-cases are indistinguishable from a clean 200 HTML page today!) | `boundedProbeReader` (`limit + 1`), `z.Err()` check (`extract.go:172`), `drainErr` check (`fetch.go:285`), `"file-too-large"` check | V1.9c: Implement `boundedProbeReader`, `z.Err()`, and `drainErr` checks |
| **16** | Rendered page without retained raw server HTML (`rendered == "true"`) | **`UNKNOWN`** | `rendered == "true"`, `html_extraction_status == "missing_legacy"` | Yes (`rendered == "true"` emitted at `adapter.go:920`) | `evaluateCANON005` check | V1.9d: Return `UNKNOWN` when `rendered == "true"` |
| **17** | Single canonical where normalization succeeds (`canonical_normalization_complete == "true"`) and resolution is proven valid | **`PASS`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_syntax_status == "valid"`, `canonical_normalization_complete == "true"`, `canonical_normalized_target != ""` | Partial (requires `html_extraction_status == "complete"` and `canonical_syntax_status == "valid"`) | `html_extraction_status`, `canonical_syntax_status`, `canonical_raw` | V1.9c + V1.9d |
| **18** | Single canonical where normalization fails with a proven `parser_error` or `resolution_error` | **`FAIL`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_normalization_complete == "false"`, `canonical_syntax_status` in `{"parser_error", "resolution_error"}`, `canonical_parse_error != ""` | **No** (Only untyped `canonical_normalization_complete == "false"` exists today) | `canonical_syntax_status`, `canonical_parse_error`, `html_extraction_status` | V1.9c + V1.9d |
| **19** | Single canonical where normalization fails **without** a proven syntax/resolution cause (`insufficient_evidence`, including Scenario 6a-ii & 6c) | **`UNKNOWN`** | `html_extraction_status == "complete"`, `canonical_count == "1"`, `canonical_normalization_complete == "false"`, `canonical_syntax_status == "insufficient_evidence"` (or `canonical_parse_error` absent) | Yes (Conservative fallback design) | `evaluateCANON005` safeguard implementation | V1.9c + V1.9d: Return `UNKNOWN` whenever `parser_error`/`resolution_error` is not proven |
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
     CanonicalTotalCount int      `json:"canonical_total_count,omitempty"`
     RawCanonicals       []string `json:"raw_canonicals,omitempty"`
     BaseHrefRaw         string   `json:"base_href_raw,omitempty"`
     BaseHrefCount       int      `json:"base_href_count,omitempty"`
     ExtractionStatus    string   `json:"extraction_status,omitempty"`
     ```
2. **`internal/sitecrawl/extract.go`:**
   - Update `document` struct to track `CanonicalTotalCount int`, `RawCanonicals []string`, `BaseHrefRaw string`, `BaseHrefCount int`, and `TokenizerErr error`.
   - In `extractDocument` (`extract.go:172–176`): when `tt == html.ErrorToken`, record `d.TokenizerErr = z.Err()`.
   - In `<base>` token handling (`extract.go:322–326`): whenever `<base>` has an `href` key in `attrs` (`_, hasHref := attrs["href"]`), increment `d.BaseHrefCount++`; on the first `<base>` with `hasHref`, record `d.BaseHrefRaw = attrs["href"]`.
   - In `readLink` (`extract.go:499–525`): check `rel` tokens before checking `href == ""`. When a token in `strings.Fields(rel)` is `"canonical"`:
     - Increment `d.CanonicalTotalCount++` (**uncapped**).
     - Append `attrs["href"]` (which is `""` if missing/empty) to `d.RawCanonicals = appendCapped(d.RawCanonicals, attrs["href"], 5)`.
     - Only when trimmed `href != ""`, append to `d.Canonicals = appendCapped(d.Canonicals, href, 5)`.
   - In `d.resolve(base)` (`extract.go:560–569`): leave `d.RawCanonicals` untouched so raw values are never overwritten.
3. **`internal/sitecrawl/fetch.go` & `internal/sitecrawl/page.go`:**
   - Implement `boundedProbeReader` in `fetch.go` reading up to `limit + 1` bytes from `res.Body` (delivering at most `limit` bytes to `extractDocument`):
     - When `res.ContentLength > cap` (`ErrTooLarge`), set `out.ExtractionStatus = "truncated_size_limit"`.
     - Otherwise, run `extractDocument(probe, base)`. If `!probe.reachedEOF`, drain up to `drainCap` bytes via `rest, drainErr := io.Copy(io.Discard, io.LimitReader(res.Body, drainCap))` and add `rest` to `out.Size`.
     - Classify `out.ExtractionStatus` strictly per Section 4.2.3:
       - If `probe.readErr != nil && probe.readErr != io.EOF`, or `doc.TokenizerErr != nil && doc.TokenizerErr != io.EOF`, or `drainErr != nil`: set `out.ExtractionStatus = "read_error"` (and set `out.ErrorType = classifyError(...)` when a read error occurred).
       - Else if `probe.exceededLimit`: set `out.ExtractionStatus = "truncated_size_limit"`.
       - Else if `probe.reachedEOF && doc.TokenizerErr == io.EOF && drainErr == nil && out.ErrorType == ""`: set `out.ExtractionStatus = "complete"`.
       - Else: set `out.ExtractionStatus = "read_error"`.
   - In `buildPage` (`page.go:57–93`): when `len(res.Hops) == 0`, copy `p.ExtractionStatus = res.ExtractionStatus`, and when `d != nil`, copy `p.CanonicalTotalCount = d.CanonicalTotalCount`, `p.RawCanonicals = d.RawCanonicals`, `p.BaseHrefRaw = d.BaseHrefRaw`, and `p.BaseHrefCount = d.BaseHrefCount`.

#### B. Evidence Adapter Changes (`internal/audit/adapter/adapter.go` & `types.go`)
1. **Add `GapCanonicalSampleCapped` Constant (`internal/audit/adapter/types.go`):**
   - `GapCanonicalSampleCapped = "GAP_CANONICAL_SAMPLE_CAPPED"`.
2. **Emit `html_extraction_status` on `SubjectURL`:**
   - Emit `"complete"`, `"truncated_size_limit"`, `"read_error"`, or `"missing_legacy"` per Section 4.2.3.
3. **Emit True `canonical_count` & Sample-Cap Truncation Evidence (`DEC-V1.9A-003`):**
   - When `pr.page.ExtractionStatus != ""` (V1.9c+ crawl run), set `totalCount := pr.page.CanonicalTotalCount`. (For legacy rows where `pr.page.ExtractionStatus == ""`, fall back to `len(canonicals)`).
   - Emit `canonical_count = strconv.Itoa(totalCount)`.
   - When `totalCount > len(pr.page.RawCanonicals)` (`totalCount > 5`), emit `canonical_sample_truncated = "true"`, append `GapCanonicalSampleCapped`, and set `canonical_normalization_complete = "false"` whenever `len(distinctTargets) <= 1` or any sample failed normalization (so `AR-CANON-004` never emits a false `WARNING` when tags `6..N` are unverified).
   - Keep `canonical_resolved` emitted only for non-empty `pr.page.Canonicals` entries so `AR-CANON-003` is 100% preserved.
4. **Standards-Based Single-Canonical Syntax & Resolution Classifier (`DEC-V1.9A-001`):**
   - When `totalCount == 1` and `len(pr.page.RawCanonicals) == 1`:
     - Emit `canonical_raw = pr.page.RawCanonicals[0]`.
     - Classify `(rawHref, pr.page.BaseHrefRaw, pr.page.BaseHrefCount, pr.url)` per Section 4.1.2 & Section 4.1.3 into `"valid"`, `"parser_error"`, `"resolution_error"`, or `"insufficient_evidence"`, emitting `canonical_syntax_status` and (when not `"valid"`) `canonical_parse_error`.
     - Specifically:
       - If `rawHref` is a valid absolute HTTP(S) URL (`parsed.IsAbs()` with `http`/`https` scheme and valid host), emit `canonical_syntax_status = "valid"` regardless of `BaseHrefRaw` / `BaseHrefCount` (**Scenario 6b-i**).
       - If `rawHref` is a valid relative reference and `BaseHrefCount == 0` (or `BaseHrefCount == 1` and `strings.TrimSpace(BaseHrefRaw)` is `""` or fragment-only `"#..."`), resolve against `pr.url` (`FinalURL`) and emit `canonical_syntax_status = "valid"` (**Scenario 4 & Scenario 6b-ii**).
       - If `rawHref` is a relative reference and `BaseHrefCount == 1` with a non-empty `BaseHrefRaw`:
         - If `BaseHrefRaw` parses cleanly as a URI (or resolves against `pr.url`) with a **non-HTTP(S) scheme** (`ftp:`, `mailto:`, `javascript:`, `data:`, `file:`, `tel:`), emit `canonical_syntax_status = "resolution_error"` and `canonical_parse_error = "resolution_error:unsupported_scheme"` (or `"resolution_error:non_hierarchical_base"`) (**Scenario 6a-i -> `FAIL`**).
         - If `BaseHrefRaw` fails RFC 3986 `url.Parse` or resolves with an empty/invalid host (`http://[::1`, `http:///dir/`), emit `canonical_syntax_status = "insufficient_evidence"` and `canonical_parse_error = "insufficient_evidence:ambiguous_base_fallback"` (**Scenario 6a-ii -> `UNKNOWN`**), and set `canonical_normalization_complete = "false"`.
       - If `rawHref` is a relative reference and `BaseHrefCount > 1` (with distinct `<base href>` values) or `pr.url` is unparseable, emit `canonical_syntax_status = "insufficient_evidence"` and `canonical_parse_error = "insufficient_evidence:missing_base_context"` (**Scenario 6c -> `UNKNOWN`**).
5. **Bump `NormalizationVersion` (in V1.9c only):**
   - Update `NormalizationVersion` from `"v1.9.0"` to `"v1.10.0"` in `internal/audit/adapter/adapter.go:356` when V1.9c is implemented. *(In V1.9b, `NormalizationVersion` remains strictly `"v1.9.0"`.)*

### 7.2 Phase 2 (V1.9d — Medium Risk): `AR-CANON-005` Evaluator Implementation

1. **`internal/audit/engine/canon005.go`:**
   - Implement `evaluateCANON005` enforcing the amended 22-scenario matrix:
     - Validate `url_identity` (missing/conflicting -> `UNKNOWN`).
     - Validate single-valued observation consistency (`html_extraction_status`, `canonical_count`, `canonical_syntax_status`, `canonical_parse_error`, `canonical_normalization_complete`, `rendered`). Any conflict -> `UNKNOWN`.
     - If `rendered == "true"` or `html_extraction_status != "complete"` -> return `UNKNOWN`.
     - Parse `canonical_count`: if `0` or `> 1` (including `> 5`) -> return `NOT_APPLICABLE`.
     - For `canonical_count == 1`:
       - If `canonical_syntax_status == "valid"` and `canonical_normalization_complete == "true"` and `canonical_normalized_target != ""` -> return `PASS`.
       - If `canonical_syntax_status` in `{"parser_error", "resolution_error"}` and `canonical_parse_error != ""` -> return `FAIL`.
       - Otherwise (including `canonical_syntax_status == "insufficient_evidence"`, covering Scenarios 6a-ii, 6c, and 19) -> return `UNKNOWN`.
2. **`internal/audit/engine/engine.go`:**
   - Register `e.register(ruleIDCANON005, evaluateCANON005)` (`19 / 47 -> 20 / 47` in V1.9d).

### 7.3 Risk-Based QA Gates for V1.9c/d

| QA Gate ID | Risk Tier | Gate Requirement | Verification Command / Artifact |
|---|---|---|---|
| **GATE-V1.9CD-01** | **High Risk** *(Acquisition: Completeness & Uncapped Count)* | Unit tests in `internal/sitecrawl/extract_test.go` and `fetch_test.go` verifying:<br>1. Clean EOF (`< limit` and **exact `limit`-byte boundary** where `(limit+1)`-th read returns `io.EOF`) sets `ExtractionStatus = "complete"`.<br>2. Stream with `>= limit + 1` bytes sets `probe.exceededLimit = true` and `ExtractionStatus = "truncated_size_limit"`.<br>3. Mid-stream reader error or **exact `limit`-byte stream followed by a drain read error (`drainErr != nil`)** sets `ExtractionStatus = "read_error"`.<br>4. Pre-parse `ContentLength > cap` sets `ExtractionStatus = "truncated_size_limit"`.<br>5. Page with `7` `<link rel="canonical">` tags (including empty `href=""`) records `CanonicalTotalCount = 7` and `len(RawCanonicals) = 5`. | `go test -v -count=1 ./internal/sitecrawl/...` |
| **GATE-V1.9CD-02** | **High Risk** *(Adapter: Base Fallback & Sample Cap)* | Adapter tests in `internal/audit/adapter/adapter_test.go` and `canonical_test.go` verifying:<br>1. All four `html_extraction_status` states (`complete`, `truncated_size_limit`, `read_error`, `missing_legacy`).<br>2. Scenario 6a-i (relative canonical + `<base href="ftp://...">` / `<base href="mailto:...">`) emits `resolution_error`.<br>3. Scenario 6a-ii (relative canonical + malformed `<base href="http://[::1">`) emits `insufficient_evidence:ambiguous_base_fallback`.<br>4. Scenario 6b-i (absolute canonical + `<base href="http://[::1">`) and 6b-ii (relative canonical + `<base href="">` / `<base href="#frag">`) emit `canonical_syntax_status = "valid"`.<br>5. Page with `> 5` canonicals (`CanonicalTotalCount = 6`) emits `canonical_count = "6"`, `canonical_sample_truncated = "true"`, and `GapCanonicalSampleCapped`. | `go test -v -count=1 ./internal/audit/adapter/...` |
| **GATE-V1.9CD-03** | **High Risk** *(Cross-Rule Regression)* | Regression test suite proving zero behavioral regressions across `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, and `AR-CANON-007` (including empty `href=""`, `[valid, ""]`, `> 5` canonical declarations, and all Scenario 6 `<base href>` sub-cases). | `go test -v -count=1 -run "TestEvaluateCANON00(3|4|6|7)" ./internal/audit/engine/...` |
| **GATE-V1.9CD-04** | **Medium Risk** *(Evaluator & 22-Scenario Matrix)* | Exhaustive table-driven unit tests in `internal/audit/engine/canon005_test.go` covering all 22 scenarios (including sub-cases 6a-i, 6a-ii, 6b-i, 6b-ii, 6c, 11a, 11b, 12a, 12b, 13a, 13b, 15a, 15b, 15c) plus hermetic end-to-end SQLite -> Adapter -> Engine integration tests. | `go test -v -count=1 ./internal/audit/...` |
| **GATE-V1.9CD-05** | **MVP-First Pipeline Gate** | Full repository build, vet, test, and diff check passing cleanly; unimplemented rules (`27 / 47` after V1.9d) remain explicitly un-registered / `NOT EVALUATED` and never silently `PASS`. | `go test -count=1 ./... && go vet ./... && go build ./cmd/sitecrawl-dev && git diff --check` |

---

## 8. Remaining Risks & Mitigations for QA Review

1. **WHATWG vs. RFC 3986 (`net/url`) Parser Divergence on Edge-Case `<base href>` Strings:**
   - *Risk:* Go's standard library `net/url.Parse` implements RFC 3986 rather than the full WHATWG URL Standard state machine. On malformed `<base href>` strings (Scenario 6a-ii), a browser might either recover a host or fall back to the document URL (`FinalURL`).
   - *Mitigation:* By routing unparseable `<base href>` strings paired with relative canonicals (Scenario 6a-ii) to **`UNKNOWN`** (`insufficient_evidence:ambiguous_base_fallback`) rather than `FAIL` or `PASS`, the contract guarantees zero false `FAIL` verdicts while never silently assuming an unverified target URL.
2. **`AR-CANON-004` on Pages with `> 5` Canonical Declarations Where First 5 Samples Have `>= 2` Distinct Valid Targets:**
   - *Risk:* When `CanonicalTotalCount > 5` (e.g. `6` canonical tags), only 5 samples are retained in `RawCanonicals`. If all 5 retained samples are identical (`len(distinctTargets) == 1`), `AR-CANON-004` must return `UNKNOWN` because tag #6 is unverified. If the 5 retained samples already contain `>= 2` distinct valid targets (`len(distinctTargets) >= 2`), multiplicity of distinct targets is already proven regardless of tag #6 (`AR-CANON-005` returns `NOT_APPLICABLE` in both cases because `canonical_count == 6 > 1`).
   - *Mitigation:* Section 4.3.2 and `GATE-V1.9CD-03` explicitly test `CanonicalTotalCount > 5` across both `AR-CANON-005` (`NOT_APPLICABLE`) and `AR-CANON-004` (`UNKNOWN` when retained samples are identical).
3. **Legacy SQLite Crawl Databases (`html_extraction_status == "missing_legacy"`):**
   - *Risk:* SQLite databases generated prior to V1.9c lack `extraction_status`, `canonical_total_count`, and `raw_canonicals` in `sitecrawl_pages.data`.
   - *Mitigation:* `AR-CANON-005` safely evaluates legacy SQLite rows to `UNKNOWN` (`html_extraction_status == "missing_legacy"`), while `AR-CANON-003/004/006/007` fall back to `Page.Canonicals` without regression.
4. **Pre-existing `.ai-dev-system/docs/INDEX.md` Local Link Gate Failures:**
   - *Risk:* `.ai-dev-system/gates/run.ps1` (`docs.local-links`) reports 5 pre-existing unresolved template links in `.ai-dev-system/docs/INDEX.md` (`architecture/`, `decisions/`, `../tests/run.ps1`, `../README.md`, `../CHANGELOG.md`), which are outside the V1.9b scope boundary.

---

## 9. Contract Acceptance Criteria & QA Sign-Off Checklist

Before V1.9c/d implementation begins, independent QA sign-off is requested against these acceptance criteria:

- [ ] **AC-1 (`DEC-V1.9A-001` Revised — Failure Semantics & Scenario 6 `<base href>` Fallback):**
  - `AR-CANON-005` emits `FAIL` **only** on proven `parser_error` (`empty_href`, `malformed_url`) or proven `resolution_error` (`unsupported_scheme`, `missing_host`, `non_hierarchical_base`).
  - Invalid `<base href>` alone **never** automatically produces `FAIL`:
    - Absolute HTTP(S) canonical + any invalid `<base href>` -> **`PASS`** (Scenario 6b-i).
    - Relative canonical + `<base>` without `href`, `<base href="">`, or `<base href="#frag">` -> **`PASS`** via deterministic fallback to `FinalURL` (Scenario 6b-ii).
    - Relative canonical + valid non-HTTP(S) `<base href="ftp://...">` / `<base href="mailto:...">` / `<base href="javascript:...">` -> **`FAIL`** (Scenario 6a-i).
    - Relative canonical + malformed/unparseable `<base href="http://[::1">` -> **`UNKNOWN`** (`insufficient_evidence:ambiguous_base_fallback`, Scenario 6a-ii).
    - Relative canonical + missing/conflicting base context (`BaseHrefCount > 1`) -> **`UNKNOWN`** (`insufficient_evidence:missing_base_context`, Scenario 6c).
  *(Status: **PENDING INDEPENDENT QA APPROVAL**)*
- [ ] **AC-2 (`DEC-V1.9A-002` Revised — Verifiable HTML Extraction Completeness via `limit + 1` Probe & Drain Error Check):**
  - Replaces `counter.n < limit || extra == 0` with `boundedProbeReader` (`limit + 1` probe) + `z.Err() == io.EOF` + `drainErr == nil` check.
  - Distinguishes `"complete"` (clean EOF: `probe.reachedEOF && !probe.exceededLimit && probe.readErr == io.EOF && doc.TokenizerErr == io.EOF && drainErr == nil && out.ErrorType == ""`), `"truncated_size_limit"` (`ContentLength > cap` or `probe.exceededLimit`), `"read_error"` (mid-stream read error, tokenizer error, or post-parse `drainErr != nil`), and `"missing_legacy"`.
  - Any state other than `"complete"` evaluates to **`UNKNOWN`** in `AR-CANON-005`.
  *(Status: **PENDING INDEPENDENT QA APPROVAL**)*
- [ ] **AC-3 (`DEC-V1.9A-003` Accepted with Conditions — Uncapped `CanonicalTotalCount`, Sample Cap & `> 5` Declarations):**
  - `CanonicalTotalCount` counts all `<link rel="canonical">` tags (including empty/whitespace `href`) without cap and drives `canonical_count` (never `len(RawCanonicals)`).
  - `RawCanonicals` and `Canonicals` remain capped at 5 samples; when `CanonicalTotalCount > 5`, Adapter emits `canonical_count = strconv.Itoa(CanonicalTotalCount)` (e.g. `"6"`), `canonical_sample_truncated = "true"`, and `GapCanonicalSampleCapped`.
  - When `CanonicalTotalCount > 5`: `AR-CANON-005` returns **`NOT_APPLICABLE`** (`canonical_count > 1`), and `AR-CANON-004` returns **`UNKNOWN`** whenever the 5 retained samples normalize to a single target (since unretained tags `6..N` cannot be proven identical).
  - Backward compatibility with `Page.Canonical`, `Page.Canonicals`, SQLite `sitecrawl_pages`, and `AR-CANON-003/004/006/007` is preserved.
  *(Status: **PENDING INDEPENDENT QA APPROVAL**)*
- [ ] **AC-4 (V1.9c/d Scope, 22-Scenario Matrix & 5 QA Gates):**
  - The updated 22-scenario QA matrix (Section 6) and QA gates `GATE-V1.9CD-01` through `GATE-V1.9CD-05` (Section 7.3) are approved to govern V1.9c/d implementation.
  *(Status: **PENDING INDEPENDENT QA APPROVAL**)*
