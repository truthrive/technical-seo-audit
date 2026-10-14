# Audit V1.9b: AR-CANON-005 Canonical Syntax Evidence Contract Review

## 1. Executive Summary

- **Audit Milestone:** Audit V1.9b — AR-CANON-005 Canonical Syntax Evidence Contract Review
- **Repository:** `truthrive/technical-seo-audit`
- **Branch:** `main`
- **Approved Baseline:** `a82a0691c13ae753ee52a1dcba2ec8e5bff1f0be`
- **NormalizationVersion:** Strictly `v1.9.0` (unchanged; no normalization changes authorized in V1.9b)
- **Executable Rules:** Strictly `19 / 47` (unchanged; no evaluator implemented in V1.9b)
- **Remaining Rules:** Exactly `28 / 47`
- **Milestone Type:** **Design-Only Contract Review**. Zero executable evaluator code, zero rule registrations, zero crawler modifications, zero SQLite schema diffs, zero adapter normalization alterations, and zero policy changes.

### 1.1 Objective & Strategic Context

Following the Audit V1.9a readiness review and targeted correction, `AR-CANON-005 — Canonical URL is syntactically resolvable` was correctly reclassified from `READY_TO_IMPLEMENT` to `NEEDS_CONTRACT_DECISION`.

This design review establishes an evidence-backed implementation contract for `AR-CANON-005` by:
1. Tracing the full end-to-end evidence pipeline from HTML source bytes through SiteCrawl extraction, SQLite persistence, and Adapter normalization into the Engine's `EvidenceSnapshot`.
2. Formulating and analyzing the three binding contract decisions (DEC-V1.9A-001, DEC-V1.9A-002, and DEC-V1.9A-003).
3. Constructing a complete, 22-scenario testable verdict decision matrix.
4. Defining rigorous implementation boundaries for conditional follow-up milestones (V1.9c Evidence Wiring and V1.9d Evaluator Implementation).

### 1.2 Status of Decisions

In strict adherence to governance principles, all contract options evaluated in this document remain **PENDING EXTERNAL APPROVAL**. No contract decision is self-approved by the agent.

---

## 2. Baseline & Frozen Contract

### 2.1 Frozen Rule Definition

From `knowledge/07-v1-atomic-rule-manifest.md` (lines 346–360) and `internal/audit/rules_v1.json` (lines 571–606):

- **Rule ID:** `AR-CANON-005`
- **Name:** Canonical URL is syntactically resolvable
- **Parent Check:** `CANON-003`
- **Lifecycle:** Consolidate
- **Automation:** Deterministic
- **Default Severity:** P1
- **Source References:** `SRC-GOOGLE-CANONICAL-001`
- **Required Inputs:**
  1. `url` (Evaluation subject URL identity)
  2. `canonical_raw` (Raw un-normalized canonical declaration attribute string)
  3. `canonical_resolved_url` (Normalized/resolved HTTP(S) canonical target URL)
  4. `canonical_parse_error` (Diagnostic error string if parsing or resolution fails)
- **Preconditions:** Exactly one canonical declaration exists (`canonical_count == 1`).

### 2.2 Frozen Verdict Semantics

| Condition | Status | Non-Negotiable Contract Meaning |
|---|---|---|
| Exactly one canonical declaration exists, and the value successfully resolves to a valid absolute HTTP(S) URL | **`PASS`** | Authoritative canonical target is syntactically valid and unambiguous. |
| Exactly one canonical declaration exists, but the value cannot be resolved as a valid canonical URL due to proven syntax/resolution failure | **`FAIL`** | Search engines cannot resolve the declared canonical target; search consolidation breaks. |
| Canonical source cannot be read, extraction is unverified, or required evidence is contradictory / missing | **`UNKNOWN`** | Diagnostic evidence is insufficient to prove either compliance or failure. |
| No single canonical declaration exists (`canonical_count == 0` or `canonical_count > 1`) | **`NOT_APPLICABLE`** | Precondition unsatisfied. Multiplicity is handled by `AR-CANON-004`; absence is handled by `AR-CANON-003`. |

*Rule Invariant:* The rule does **not** use `WARNING` or `MANUAL_REVIEW`.

### 2.3 Non-Negotiable Architectural Principles

- `MISSING != FALSE`: Absence of an observation does not constitute an observed negative value.
- `CANONICAL ABSENT != INVALID`: A page lacking a canonical tag is evaluated by `AR-CANON-003` (warning/not-applicable), not `AR-CANON-005` (fail).
- `SOURCE UNREADABLE != INVALID CANONICAL`: An unreadable HTML document or headless-rendered DOM without raw head retention cannot support a `FAIL` verdict; it must evaluate to `UNKNOWN`.
- `NORMALIZATION FAILURE != AUTOMATIC SYNTAX FAILURE`: An aggregate failure during URL normalization does not prove that the underlying declaration contains invalid syntax.
- `OBSERVATION != CONCLUSION`: Raw facts must remain separate from diagnostic verdicts.

---

## 3. Source-to-Evidence Provenance Trace

To determine exact evidence availability, the complete production path was inspected across the codebase.

```mermaid
flowchart TD
    A[HTTP Response Body text/html] -->|Streaming Tokenizer| B[internal/sitecrawl/extract.go: onLink]
    B -->|rel=canonical & href!=''| C[d.Canonicals []string]
    C -->|extract.go: resolve| D[extract.go: absolutize]
    D -->|Overwrites in-place| E[Page.Canonicals []string post-absolutized]
    E -->|JSON Serialization| F[(SQLite: sitecrawl_pages.data)]
    F -->|Adapter Query| G[internal/audit/adapter/adapter.go]
    G -->|len canonicals| H[NormalizedObservation: canonical_count]
    G -->|Raw string| I[NormalizedObservation: canonical_resolved]
    G -->|normalizeCanonicalTarget| J[NormalizedObservation: canonical_normalized_target]
    G -->|Aggregate check| K[NormalizedObservation: canonical_normalization_complete]
    G -->|Constructs| L[audit.CanonicalObservation RawValues=nil, ParseErrors=nil]
    K -->|EvidenceSnapshot| M[Engine: EvidenceIndex]
```

### 3.1 Step 1: HTML Tokenization & Extraction (`internal/sitecrawl/extract.go`)

- **Location:** `internal/sitecrawl/extract.go:500–525` (`onLink` callback during streaming HTML tokenization).
- **Rel Matching:** Attribute `rel` is normalized with `strings.ToLower(strings.TrimSpace(attrs["rel"]))`. It splits on whitespace via `strings.Fields(rel)`, correctly matching compound values like `rel="canonical alternate"`.
- **Href Handling:** `href := strings.TrimSpace(attrs["href"])`.
- **Empty & Whitespace-Only Declarations:**
  *Verified Fact:* Line 502 specifies:
  ```go
  if rel == "" || href == "" {
      return
  }
  ```
  If `<link rel="canonical">` has no `href` attribute, or `href=""`, or `href="   "`, `onLink` **returns immediately without recording the tag**.
  *Consequence:* Empty canonical declarations are completely dropped during extraction. They are never appended to `d.Canonicals`. Downstream systems cannot distinguish a page with `<link rel="canonical" href="">` from a page with no canonical tag at all.
- **Multiple Canonical Tags:** Handled via `appendCapped(d.Canonicals, href, 5)`. Capped at 5 declarations.

### 3.2 Step 2: Document Resolution & Absolutization (`internal/sitecrawl/extract.go`)

- **Location:** `internal/sitecrawl/extract.go:560–570` (`resolve(base *url.URL)`) and lines 595–604 (`absolutize`).
- **Base HREF Handling:** If `<base href="...">` is present and valid (`d.BaseHref != ""`), `effective` is updated:
  ```go
  if b, err := base.Parse(d.BaseHref); err == nil && b.Host != "" {
      effective = b
  }
  ```
- **In-Place Mutation:** Lines 567–569 iterate through `d.Canonicals`:
  ```go
  for i := range d.Canonicals {
      d.Canonicals[i] = absolutize(effective, d.Canonicals[i])
  }
  ```
  *Verified Fact:* `absolutize` converts relative URLs (e.g. `/category/product` or `../product`) into absolute URLs (e.g. `https://example.com/category/product`). **The original raw relative declaration is permanently overwritten.**
- **Suppression of Parse Errors:** In lines 595–604:
  ```go
  func absolutize(base *url.URL, raw string) string {
      if raw == "" {
          return ""
      }
      ref, err := url.Parse(strings.TrimSpace(raw))
      if err != nil {
          return raw
      }
      return base.ResolveReference(ref).String()
  }
  ```
  *Verified Fact:* If `url.Parse` fails on a malformed declaration, `err` is silently discarded, and the unparsed `raw` string is returned. **No parse error diagnostics are recorded.**

### 3.3 Step 3: Domain Assembly & SQLite Persistence (`internal/sitecrawl/page.go`, `persist.go`)

- **Location:** `internal/sitecrawl/page.go:75–76`.
- **Assignment:**
  ```go
  p.Canonicals = d.Canonicals
  p.Canonical = first(d.Canonicals)
  ```
- **SQLite Storage:** In `sitecrawl_pages` table (`internal/sitecrawl/runs.go:46–100` and `persist.go:65`):
  - Column `canonical`: Stores `p.Canonical` (only the first declaration, post-absolutized).
  - Column `data`: Stores the entire `Page` struct serialized as JSON, containing `p.Canonicals []string` (all post-absolutized).
  *Verified Fact:* SQLite does not store raw un-absolutized canonical declarations or parse error messages.

### 3.4 Step 4: Adapter Normalization (`internal/audit/adapter/adapter.go`)

- **Location:** `internal/audit/adapter/adapter.go:1074–1225`.
- **Declaration Recovery:**
  ```go
  canonicals := pr.page.Canonicals
  if len(canonicals) == 0 && (pr.page.Canonical != "" || pr.canonical != "") {
      c := pr.page.Canonical
      if c == "" {
          c = pr.canonical
      }
      canonicals = []string{c}
  }
  ```
- **Observation Construction:**
  1. `canonical_count`: Emitted on `SubjectURL` with value `strconv.Itoa(len(canonicals))`.
  2. `audit.CanonicalObservation`: Appended to domain observations:
     ```go
     audit.CanonicalObservation{
         CanonicalObservationID: audit.CanonicalObservationID(fmt.Sprintf("canon:%s:%d", req.AuditRunID, urlID)),
         AuditRunID:             req.AuditRunID,
         URLID:                  urlIDStr,
         CanonicalCount:         len(canonicals),
         RawValues:              nil, // Raw canonical syntax unavailable
         NormalizedValues:       canonicals,
         ResolvedTargetURLIDs:   targetIDs,
         ParseErrors:            nil,
         ObservedAt:             obsTime,
     }
     ```
     *Verified Fact:* The Adapter explicitly sets `RawValues: nil` and `ParseErrors: nil` because raw syntax and parse errors are absent from the crawler output.
  3. `canonical_resolved`: Emitted for each declaration in `canonicals`. *Verified Fact:* This value is the post-absolutized string, not the raw HTML attribute string.
  4. `canonical_normalized_target`: Produced by passing each declaration to `normalizeCanonicalTarget(c, pr.url)` (`adapter.go:1596`). If normalization succeeds, it emits the normalized URL.
  5. `canonical_normalization_complete`: Line 1171 sets `completeVal := "false"`. Only if `allNormalized && len(normTargets) == len(canonicals)` does it emit `"true"`. Otherwise, it emits `"false"`.
  6. `canonical_distinct_normalized_count`: Emitted only when `canonical_normalization_complete == "true"`.

### 3.5 Step 5: Headless Render Impact

- **Location:** `internal/audit/adapter/adapter.go:904–950`.
- **Render Replacement:** When rendering is active (`isRendered = true`), SiteCrawl replaces server-delivered HTML with the post-hydration browser DOM (`GapRenderedRawSourceUnavailable`).
- **Canonical Suppression:** Lines 935–950 deliberately **suppress** HTML canonical observations on rendered pages to avoid attributing client-side injected tags to server canonical declarations without raw proof.

---

## 4. Current Evidence Availability & Gaps

| Manifest Required Input | Corresponding Engine Field / Observation | Actual Value / Provenance | Integrity Status | Specific Gap Identified |
|---|---|---|---|---|
| `url` | `SubjectURL`, `url_identity` | Requested URL string | **Complete** | None. URL identity is fully preserved and indexed. |
| `canonical_raw` | `CanonicalObservation.RawValues`, `canonical_raw` | **Missing / Overwritten** | **Incomplete** | SiteCrawl overwrites raw relative declarations with absolutized URLs (`extract.go:568`). Adapter emits `RawValues: nil`. |
| `canonical_resolved_url` | `canonical_resolved`, `canonical_normalized_target` | Post-absolutized string and audit-normalized URL | **Complete for PASS** | Available when normalization succeeds. Missing when normalization fails. |
| `canonical_parse_error` | `CanonicalObservation.ParseErrors`, `canonical_parse_error` | **Missing** | **Absent** | Neither SiteCrawl nor Adapter records or emits explicit parse error diagnostics. `ParseErrors: nil`. |

### Summary of Foundational Gaps

1. **Failure Diagnostics Gap:** An evaluator reading `canonical_normalization_complete=false` cannot inspect a typed error to determine *why* normalization failed (e.g. malformed host vs unsupported scheme vs relative resolution against empty base).
2. **Raw vs Absolutized Divergence:** The system cannot verify whether the author wrote `<link rel="canonical" href="https://example.com/page">` or `<link rel="canonical" href="/page">`.
3. **Empty Tag Blindspot:** `<link rel="canonical" href="">` is discarded during crawling, resulting in `canonical_count=0` rather than `canonical_count=1` with an empty syntax failure.

---

## 5. DEC-V1.9A-001 Option Analysis: Canonical Failure Semantics

**Core Question:** Can `canonical_normalization_complete=false` prove syntactic non-resolvability (`FAIL`) for a page with exactly one canonical declaration?

### Option A — Use Aggregate Normalization Outcome (Derived Failure)

- **Mechanics:** When `canonical_count == 1`:
  - If `canonical_normalization_complete == "true"`: Return `PASS`.
  - If `canonical_normalization_complete == "false"`: Return `FAIL`.
  - If canonical source is unreadable: Return `UNKNOWN`.
- **Compliance with Manifest:** Low. Manifest requires `canonical_parse_error` and proves invalid canonical syntax. Normalization failure encompasses non-syntax conditions (e.g. valid URI with unsupported scheme).
- **False-Positive Risk:** High. If a canonical URL fails normalization due to an internal timeout, scheme restriction, or normalizer bug, the page receives a false `FAIL` verdict.
- **Implementation Complexity:** Low (Engine evaluator only; zero Adapter changes).
- **NormalizationVersion Impact:** Zero (`v1.9.0` preserved).

### Option B — Explicit Syntax Diagnostics (Recommended for Long-Term Integrity)

- **Mechanics:** Enhance the Adapter (or Engine helper) to emit an explicit `canonical_parse_error` or `canonical_syntax_valid` observation on `SubjectURL`.
  - When `canonical_count == 1`:
    - Valid HTTP(S) resolution -> `PASS`.
    - Proven parsing/syntax error -> `FAIL` (with explicit error string).
    - Normalization incomplete without proven syntax error -> `UNKNOWN`.
- **Compliance with Manifest:** Strict. Directly fulfills the manifest's `canonical_parse_error` input and auditable failure proof.
- **False-Positive Risk:** Near Zero. Only proven syntax or reference resolution failures produce `FAIL`.
- **Implementation Complexity:** Medium (Requires Adapter wiring in Milestone V1.9c before V1.9d).
- **NormalizationVersion Impact:** Requires evaluating whether adding `canonical_parse_error` constitutes a version bump.

### Option C — Conservative UNKNOWN (Non-Breaking Intermediate)

- **Mechanics:** When `canonical_count == 1`:
  - If `canonical_normalization_complete == "true"`: Return `PASS`.
  - If `canonical_normalization_complete == "false"`: Return `UNKNOWN` (reason: "Canonical normalization incomplete; explicit syntax failure unverified").
- **Compliance with Manifest:** Medium. Never produces an unsupported `FAIL`, but leaves `FAIL` unexecutable until diagnostic wiring is implemented.
- **False-Positive Risk:** Zero.
- **Implementation Complexity:** Low.
- **NormalizationVersion Impact:** Zero (`v1.9.0` preserved).

### Recommendation

**Option B (Explicit Syntax Diagnostics)** is the only technically defensible option that satisfies the frozen manifest contract without diagnostic corruption.
*Status:* **PENDING EXTERNAL APPROVAL**.

---

## 6. DEC-V1.9A-002 Option Analysis: Canonical Source Completeness

**Core Question:** What evidence is sufficient to distinguish an invalid canonical declaration (`FAIL`) from an unreadable canonical source (`UNKNOWN`)?

### Critical Source Limitations Uncovered

The previously proposed heuristic:
`http_status == 200 AND content_type contains html AND rendered != true`
is **insufficient** on its own because:
1. `http_status == 200` does not guarantee the HTML response body was read to completion; network drops mid-stream can truncate `<head>`.
2. Truncated HTML is not recorded as a specific error flag in `Page`.
3. An author declaring `<link rel="canonical" href="">` is dropped by `extract.go:502`, causing `canonical_count=0` instead of a syntax error.

### Option Analysis

- **Option A (Derived Source Completeness):** Treat source as readable if:
  `http_status == 200` AND `content_type contains html` AND `rendered != true` AND `fetch_error_type == ""`.
  *Trade-off:* Relies on existing observations, but cannot detect mid-stream truncation or dropped empty `href` tags.
- **Option B (Explicit Typed Completeness Evidence):** Add `html_head_complete` or `canonical_source_readable` observation emitted by the Adapter based on parser EOF verification.
  *Trade-off:* Requires crawler parser changes.
- **Option C (Conservative Unknown on Ambiguity):** If canonical count is 0 or normalization fails on any document where response time, body size, or render state is anomalous, evaluate as `UNKNOWN`.

### Recommendation

**Option A augmented with `fetch_error_type == ""`** is recommended for V1 scope, with strict `UNKNOWN` fallback on any transport error or rendered document.
*Status:* **PENDING EXTERNAL APPROVAL**.

---

## 7. DEC-V1.9A-003 Option Analysis: Raw Canonical Provenance

**Core Question:** May `canonical_resolved` replace `canonical_raw` in V1?

### Information Loss Analysis

| Scenario | Raw Declaration | Absolutized Value (`extract.go`) | Adapter Output (`canonical_resolved`) | Information Lost? | Impact on AR-CANON-005 Verdict |
|---|---|---|---|---|---|
| Valid Relative URL | `href="/about"` | `https://example.com/about` | `https://example.com/about` | Yes (relative syntax lost) | None for PASS. Target resolves validly. |
| Relative with Base | `href="sub"` + `<base href="/dir/">` | `https://example.com/dir/sub` | `https://example.com/dir/sub` | Yes (base dependency lost) | None for PASS. Target resolves validly. |
| Invalid Scheme | `href="javascript:void(0)"` | `javascript:void(0)` | `javascript:void(0)` | No | Proven failure can be detected. |
| Malformed Syntax | `href="https://:bad"` | `https://:bad` | `https://:bad` | No | Proven syntax error can be detected. |
| Empty Href | `href=""` | *(Dropped during parse)* | *(None — count = 0)* | **Critical** | Converted to missing canonical instead of syntax failure! |

### Option Analysis

- **Option A (Derived-Evidence Contract):** Formally approve in the design contract that `canonical_resolved` satisfies `canonical_raw` for V1, acknowledging that relative vs absolute declaration distinction is deferred to V2.
- **Option B (Strict Raw Provenance):** Modify SiteCrawl `extract.go` to collect `RawCanonicals []string` before `resolve()` runs, persist in SQLite `sitecrawl_pages`, and emit `canonical_raw` in Adapter.
- **Option C (Hybrid Approach):** Use `canonical_resolved` to satisfy `PASS` (where valid resolution is proven), but require explicit syntax error verification on the resolved string before emitting `FAIL`.

### Recommendation

**Option C (Hybrid Approach)** is recommended. It allows `PASS` to execute safely while ensuring `FAIL` is only emitted when the preserved declaration string is definitively unparseable.
*Status:* **PENDING EXTERNAL APPROVAL**.

---

## 8. Complete Verdict Decision Matrix

The following 22-scenario matrix defines the exact deterministic behavior required for `AR-CANON-005`.

| # | Input Scenario | Required Evidence State | Expected Verdict | Currently Available? | Remaining Gap | Contract Dependency |
|---|---|---|---|---|---|---|
| 1 | No canonical declaration | `canonical_count == 0` | **`NOT_APPLICABLE`** | **Yes** | None | Covered by frozen manifest |
| 2 | Exactly one valid absolute HTTP URL | `count == 1`, target valid HTTP, `complete == true` | **`PASS`** | **Yes** | None | DEC-V1.9A-003 |
| 3 | Exactly one valid absolute HTTPS URL | `count == 1`, target valid HTTPS, `complete == true` | **`PASS`** | **Yes** | None | DEC-V1.9A-003 |
| 4 | Exactly one valid relative canonical URL | `count == 1`, resolved to valid HTTP(S), `complete == true` | **`PASS`** | **Yes** | Raw relative syntax lost, but target valid | DEC-V1.9A-003 (Option C) |
| 5 | Relative canonical with valid `<base href>` | `count == 1`, resolved via base, `complete == true` | **`PASS`** | **Yes** | Base provenance lost, but target valid | DEC-V1.9A-003 (Option C) |
| 6 | Relative canonical with missing/invalid base | `count == 1`, base resolution fails, target unresolvable | **`FAIL`** | Partial | `complete == false` emitted, but error untyped | DEC-V1.9A-001 (Option B) |
| 7 | Canonical using unsupported scheme (`javascript:`, `ftp:`) | `count == 1`, scheme != http/https | **`FAIL`** | Partial | `complete == false` emitted, but error untyped | DEC-V1.9A-001 (Option B) |
| 8 | Canonical containing malformed URL syntax (e.g. `http://[::1`) | `count == 1`, `url.Parse` error | **`FAIL`** | Partial | `complete == false` emitted; parse error dropped | DEC-V1.9A-001 (Option B) |
| 9 | Empty canonical `href=""` | `count == 1`, declaration empty | **`FAIL`** | **No** | Dropped by crawler (`count == 0`) | Crawler extraction gap |
| 10 | Whitespace-only canonical `href="   "` | `count == 1`, declaration whitespace | **`FAIL`** | **No** | Dropped by crawler (`count == 0`) | Crawler extraction gap |
| 11 | Multiple distinct canonical declarations | `canonical_count > 1`, distinct count > 1 | **`NOT_APPLICABLE`** | **Yes** | None (Handled by `AR-CANON-004`) | Covered by frozen manifest |
| 12 | Duplicate identical canonical declarations | `canonical_count > 1`, distinct count == 1 | **`NOT_APPLICABLE`** | **Yes** | None (Handled by `AR-CANON-004`) | Covered by frozen manifest |
| 13 | Canonical source unavailable (document unreadable) | Source completeness unproven | **`UNKNOWN`** | Partial | Heuristic check possible | DEC-V1.9A-002 |
| 14 | HTTP fetch failed (connection error, timeout, 5xx) | `fetch_error_type != ""` or `status != 200` | **`UNKNOWN`** | **Yes** | None | DEC-V1.9A-002 |
| 15 | HTTP 200 but HTML extraction incomplete / corrupted | Document body truncated or parse aborted | **`UNKNOWN`** | **No** | Crawler does not flag body truncation | DEC-V1.9A-002 |
| 16 | Rendered page without retained raw HTML | `rendered == true` | **`UNKNOWN`** | **Yes** | None (`GapRenderedRawSourceUnavailable`) | DEC-V1.9A-002 |
| 17 | Canonical normalization complete == true | `count == 1`, `complete == true`, target != "" | **`PASS`** | **Yes** | None | DEC-V1.9A-001 |
| 18 | Normalization failed with proven syntax error | `count == 1`, `complete == false`, syntax invalid | **`FAIL`** | Partial | Requires typed error observation | DEC-V1.9A-001 (Option B) |
| 19 | Normalization failed without proven cause | `count == 1`, `complete == false`, syntax unverified | **`UNKNOWN`** | **Yes** | Safeguard against false FAIL | DEC-V1.9A-001 (Option C) |
| 20 | Conflicting canonical observations on SubjectURL | Contradictory `canonical_count` or `complete` | **`UNKNOWN`** | **Yes** | Standard Engine index conflict handling | Handled by Engine spec |
| 21 | Missing required evidence (no URL identity) | `url_identity` missing or conflicting | **`UNKNOWN`** | **Yes** | Standard Engine identity handling | Handled by Engine spec |
| 22 | Cross-run or invalid subject correlation | `AuditRunID` or `SnapshotID` mismatch | **`UNKNOWN`** | **Yes** | Handled by EvidenceIndex validation | Handled by Engine spec |

---

## 9. NormalizationVersion Assessment

### 9.1 Architectural Principles Governing NormalizationVersion

According to `knowledge/08-system-architecture.md` and `knowledge/09-data-model.md`:
1. **Evaluator Changes Only:** Adding or updating rule evaluators in `internal/audit/engine/` does **not** change `NormalizationVersion`.
2. **Backward-Compatible Field Additions:** If an observation field is added to `SubjectURL` that does not alter the semantics, types, or derivation logic of existing observations, the version **remains unchanged**.
3. **Semantic or Structural Changes:** Modifying existing observation field meanings, removing fields, altering subject identity derivation, or changing derivation types requires bumping `NormalizationVersion` (e.g. `v1.9.0` -> `v1.10.0`).

### 9.2 Versioning Recommendation for Milestone V1.9c

- **If Option B is approved (Adding `canonical_parse_error` to `SubjectURL`):**
  *Recommendation:* Bumping `NormalizationVersion` from `v1.9.0` to `v1.10.0` is recommended to signal a formal contract enhancement in canonical diagnostic evidence, ensuring full traceability across cached snapshots.
- **Current Milestone (V1.9b):** Strictly **no change**. `NormalizationVersion` remains `v1.9.0`.

---

## 10. Proposed Implementation Boundaries: V1.9c & V1.9d

### 10.1 Milestone V1.9c — Canonical Evidence Wiring (Conditional)

*Precondition:* External approval of DEC-V1.9A-001 (Option B), DEC-V1.9A-002, and DEC-V1.9A-003.

- **Objective:** Enhance Adapter normalization to emit typed syntax diagnostic observations.
- **Scope of Changes:**
  1. `internal/audit/adapter/adapter.go`:
     - Update `normalizeCanonicalTarget` to return typed error classifications (`ErrEmptyCanonical`, `ErrMalformedURL`, `ErrUnsupportedScheme`, `ErrEmptyHost`).
     - Emit `canonical_syntax_valid` (`"true"` / `"false"`) on `SubjectURL`.
     - When syntax is invalid, emit `canonical_parse_error` with the specific diagnostic error string.
     - Emit `canonical_raw` preserving the extracted declaration string.
  2. `NormalizationVersion`: Bump to `v1.10.0`.
  3. **Zero Evaluator Implementation:** Executable rule count remains strictly `19 / 47`.
- **Verification:** Hermetic unit and integration tests in `internal/audit/adapter/adapter_test.go` and `canonical_test.go`.

### 10.2 Milestone V1.9d — AR-CANON-005 Evaluator Implementation (Conditional)

*Precondition:* Successful completion and verification of Milestone V1.9c.

- **Objective:** Implement typed rule evaluator `evaluateCANON005`.
- **Scope of Changes:**
  1. Create `internal/audit/engine/canon005.go`:
     - Implements `evaluateCANON005`.
     - Strictly enforces the 22-case decision matrix from Section 8.
     - PASS: `count == 1` AND `canonical_syntax_valid == "true"` AND `target != ""`.
     - FAIL: `count == 1` AND `canonical_syntax_valid == "false"` AND source readable.
     - NOT_APPLICABLE: `count == 0` or `count > 1`.
     - UNKNOWN: Source unreadable, rendered without raw head, or missing identity.
  2. Register evaluator in `internal/audit/engine/engine.go`:
     ```go
     e.register(ruleIDCANON005, evaluateCANON005)
     ```
  3. Tests: Create `internal/audit/engine/canon005_test.go` with complete hermetic test coverage across all verdict branches.
  4. Executable Rule Progression: **`19 / 47 → 20 / 47`** (42.5%).

---

## 11. Risks, Limitations & Regression Strategy

| Risk Category | Specific Failure Mode | Severity | Mitigation & Regression Strategy |
|---|---|---|---|
| **False-Positive Risk** | Emitting `FAIL` on valid relative canonicals whose base resolution failed due to missing base URL | High | Require explicit syntax diagnostics; return `UNKNOWN` if base URL is absent rather than claiming author syntax error. |
| **False-Negative Risk** | Missing empty canonical tags (`<link rel="canonical" href="">`) because crawler drops them | Medium | Document as known crawler acquisition limitation for V1; evaluate as `NOT_APPLICABLE` (count 0) until crawler parser is upgraded in V2. |
| **Regression Risk** | Modifying canonical normalization breaks existing evaluators (`AR-CANON-004`, `AR-CANON-006`, `AR-CANON-007`) | High | Maintain strict backward compatibility for `canonical_count`, `canonical_resolved`, `canonical_normalized_target`, and `canonical_normalization_complete`. Run full test suite (`go test ./...`). |
| **Render Blindspot** | Emitting `FAIL` on rendered pages where raw server HTML was replaced | High | Enforce strict `isRendered` check: any rendered page without raw head retention MUST evaluate to `UNKNOWN`. |

---

## 12. Exact External Approval Questions

The following questions require formal external approval from the Project Owner before Milestone V1.9c can begin:

1. **Approval of DEC-V1.9A-001 (Failure Semantics):**
   *Question:* Is **Option B (Explicit Syntax Diagnostics)** approved, requiring Adapter-emitted `canonical_parse_error` to prove `FAIL`, or should Option C (Conservative UNKNOWN) be used to avoid Adapter changes?
2. **Approval of DEC-V1.9A-002 (Source Completeness):**
   *Question:* Is the source completeness predicate (`status == 200` AND `content_type contains html` AND `rendered != true` AND `fetch_error_type == ""`) approved for V1 evaluation?
3. **Approval of DEC-V1.9A-003 (Raw Provenance Substitution):**
   *Question:* Is the hybrid derived-evidence contract (using `canonical_resolved` for PASS, but requiring explicit unparseable syntax proof for FAIL) approved for V1, deferring pre-absolutized crawler storage to V2?
4. **Approval of Phased Sequence (V1.9c -> V1.9d):**
   *Question:* Is the two-stage implementation plan (Milestone V1.9c for Adapter evidence wiring, followed by Milestone V1.9d for Engine evaluator implementation) approved?
