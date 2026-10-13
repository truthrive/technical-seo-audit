# Audit V1.8e: Sitemap Policy & HTTP Status Contract Review

## 1. Executive Summary

- **Milestone:** Audit V1.8e — Sitemap Policy & HTTP Status Contract Review (Correction)
- **Repository:** `truthrive/technical-seo-audit`
- **Branch:** `main`
- **Correction Baseline Git HEAD:** `f8ba9de1c590e475cba1ba6e4899a95a2dd6b344`
- **Current Milestone Phase:** Research, Contract Analysis, and Design Correction Only (Zero production Go code diff; no evaluator registrations; no SQLite schema modifications; no NormalizationVersion bump)
- **Executable Rule Count:** Strictly `17 / 47` (`AR-ACC-004`, `AR-CANON-003`, `AR-CANON-004`, `AR-CANON-006`, `AR-CANON-007`, `AR-CANON-008`, `AR-CANON-009`, `AR-DISC-003`, `AR-DISC-004`, `AR-DISC-005`, `AR-DISC-006`, `AR-ENTITY-001`, `AR-INDEX-001`, `AR-INDEX-002`, `AR-LINK-002`, `AR-LINK-003`, `AR-LINK-004`)
- **NormalizationVersion:** `v1.9.0` (frozen, unchanged)
- **Current Sitemap Rule Statuses:**
  - `AR-DISC-003`: LOCKED (`internal/audit/engine/disc003.go`)
  - `AR-DISC-004`: LOCKED (`internal/audit/engine/disc004.go`)
  - `AR-DISC-005`: LOCKED (`internal/audit/engine/disc005.go`)
  - `AR-DISC-006`: LOCKED (`internal/audit/engine/disc006.go`)
  - `AR-DISC-001`: `NEEDS_POLICY_WIRING` (Evaluator unimplemented; policy schema defined; canonical `"site"` targetRef wiring required)
  - `AR-DISC-002`: `BLOCKED` (Open contract decision: Initial vs. Terminal HTTP status semantics pending external product review)

### 1.1 Objective & Scope

This design document establishes the precise requirements, evidence dependencies, official documentation alignment, and contract decisions needed before implementing the two remaining sitemap rules in the frozen 47-rule V1 manifest:
1. `AR-DISC-001 — Sitemap presence matches project policy`
2. `AR-DISC-002 — Sitemap document returns 200`

This revision applies three specific contract calibrations:
1. **Site Policy TargetRef:** Standardizes `TargetRef = "site"` as the sole authoritative reference for site-level policy lookups in MVP without automatic origin fallback.
2. **RuleResult Subject & Scope:** Aligns the proposed contract with actual supported Go domain types (`audit.SubjectSite` and `audit.AuditReportScope`), eliminating invented scope identifiers and distinguishing policy scopes from report scopes.
3. **Evidence Quality for Redirect Claims:** Calibrates claims regarding search engine sitemap redirect behavior, distinguishing verified platform statements from engineering inferences and maintaining Option B as a design recommendation pending external approval.

---

## 2. AR-DISC-001 Policy Analysis

### 2.1 Rule Identity & Frozen Manifest Contract

- **Rule ID:** `AR-DISC-001`
- **Parent Check:** `DISC-001` (*Có XML sitemap cho các URL cần index không?*)
- **Rule Name:** Sitemap presence matches project policy
- **Lifecycle:** Discover
- **Category:** `sitemaps`
- **Automation Class:** `assisted` (Guided evaluation requiring user/project intent)
- **Default Severity:** `P1`
- **Required Inputs:** `discovered_sitemap_urls`, `sitemap_expected`, `sitemap_discovery_complete`
- **Preconditions:** Root/robots sitemap discovery sources evaluated.
- **Contract Outcomes:**
  - `PASS`: One or more sitemap URLs discovered (`len(discovered_sitemap_urls) > 0`).
  - `FAIL`: Not used.
  - `WARNING`: None found and `sitemap_expected=true`.
  - `MANUAL_REVIEW`: None found and policy unspecified.
  - `UNKNOWN`: Discovery could not complete (`sitemap_discovery_complete=false`).
  - `NOT_APPLICABLE`: `sitemap_expected=false`.
- **Evidence Fields:** `discovered_sitemap_list`, `discovery_sources_checked`, `explicit_policy`, `observed_at`
- **Source References:** `SRC-GOOGLE-SITEMAP-001`

### 2.2 Inspection of Existing Policy Infrastructure

A comprehensive code trace was performed across `internal/audit/policy.go`, `internal/audit/engine/policy.go`, `internal/audit/engine/engine.go`, and existing evaluators:

1. **Definition of `PolicyKeySitemapExpected`:**
   - Defined in [internal/audit/policy.go](../internal/audit/policy.go#L52):
     ```go
     PolicyKeySitemapExpected PolicyKey = "sitemap_expected"
     ```
   - Controlled values defined in lines 84-86:
     ```go
     PolicyValueTrue        = "true"
     PolicyValueFalse       = "false"
     PolicyValueUnspecified = "unspecified"
     ```
   - Schema validation is enforced in `ValidatePolicyKeyValue` (lines 135-144), which strictly enforces:
     - `scope == PolicyScopeSite` (returns error if `scope != PolicyScopeSite`).
     - `value` must be one of `"true"`, `"false"`, or `"unspecified"`.

2. **Storage of Project Policy Values:**
   - Project policy assignments are modeled by `ProjectPolicyAssignment` ([internal/audit/policy.go:91-100](../internal/audit/policy.go#L91-L100)):
     ```go
     type ProjectPolicyAssignment struct {
         PolicyAssignmentID PolicyAssignmentID `json:"policy_assignment_id"`
         AuditRunID         AuditRunID         `json:"audit_run_id"`
         PolicyKey          PolicyKey          `json:"policy_key"`
         PolicyValue        string             `json:"policy_value"`
         Scope              PolicyScope        `json:"scope"`
         TargetRef          string             `json:"target_ref"`
         Source             PolicyProvenance   `json:"source"`
         SuppliedAt         time.Time          `json:"supplied_at"`
     }
     ```
   - There is currently no database table storing policy records in SQLite. Policies are injected at audit run instantiation and stored in memory.

3. **Loading into `PolicyIndex`:**
   - Evaluators consume policies via `engine.PolicyIndex` ([internal/audit/engine/policy.go](../internal/audit/engine/policy.go)).
   - Built via `engine.NewPolicyIndex(assignments)` or `engine.NewPolicyIndexForRun(expectedRunID, assignments)`.
   - Validates all assignments, ensures uniform `AuditRunID`, rejects conflicting values (`ErrConflictingPolicy`), and builds an immutable 3-level map: `lookup[scope][targetRef][key]`.

4. **Access Pattern in Existing Evaluators:**
   - Evaluator signature ([internal/audit/engine/engine.go:21-28](../internal/audit/engine/engine.go#L21-L28)):
     ```go
     type Evaluator func(
         ctx context.Context,
         rule audit.RuleDefinition,
         evidence *EvidenceIndex,
         policies *PolicyIndex,
         snapshot *audit.EvidenceSnapshot,
         evalTime time.Time,
     ) ([]audit.RuleResult, error)
     ```
   - Currently, only `AR-INDEX-001` ([internal/audit/engine/index001.go:76](../internal/audit/engine/index001.go#L76)) accesses `policies`:
     ```go
     policyAssignment, hasPolicy = policies.Get(audit.PolicyScopeURL, subjectRef, audit.PolicyKeyExpectedIndexable)
     ```

5. **Existence in Runtime Policy Context:**
   - `PolicyKeySitemapExpected` is defined and validated in the schema, but is **not currently wired** to any active runtime evaluator or CLI flag.
   - When `engine.EvaluateRule()` is invoked, `NewEmptyPolicyIndex()` is passed by default. Policies only exist when caller explicitly uses `engine.EvaluateRuleWithContext()` with a populated `EvaluationContext.Policies`.

6. **Distinction of Policy States:**
   - `PolicyIndex.Get(scope, targetRef, key)` returns `(*ProjectPolicyAssignment, bool)`:
     - **Absent:** `ok == false` (or `policyAssignment == nil`).
     - **Explicit `true`:** `ok == true && policyAssignment.PolicyValue == audit.PolicyValueTrue`.
     - **Explicit `false`:** `ok == true && policyAssignment.PolicyValue == audit.PolicyValueFalse`.
     - **Explicit `unspecified`:** `ok == true && policyAssignment.PolicyValue == audit.PolicyValueUnspecified`.
     - **Malformed:** Rejected at policy index instantiation by `ValidatePolicyKeyValue()`. It is impossible for an invalid string (e.g. `"yes"`, `"1"`) to exist inside a valid `PolicyIndex`.
   - **Crucial Rule:** Missing policy (`ok == false`) and explicit unspecified (`PolicyValue == "unspecified"`) are semantically equivalent: both mean policy intent is not specified, which triggers `MANUAL_REVIEW` when no sitemaps are found. Missing policy must **never** default to `false` or `true`.

7. **Site Policy TargetRef & Conflict Handling (MVP Convention):**
   - In accordance with `PolicyIndex` architecture, assignments are indexed by the tuple `(PolicyScope, TargetRef, PolicyKey)`.
   - In normalized evidence, the subject identifier for `SubjectSite` observations is uniformly `"site"` ([internal/audit/adapter/sitemap.go:270](../internal/audit/adapter/sitemap.go#L270)).
   - For the MVP contract, we recommend that:
     - `PolicyScope = PolicyScopeSite`
     - `TargetRef = "site"`
     - `PolicyKey = PolicyKeySitemapExpected`
   - **No automatic root-origin fallback is permitted within the evaluator.** An evaluator must query `policies.Get(audit.PolicyScopeSite, "site", audit.PolicyKeySitemapExpected)`.
   - Policy states and conflict handling are strictly governed by the following table:

| Policy State in `PolicyIndex` | Evaluator Behavior for `AR-DISC-001` | Rationale |
|---|---|---|
| `"site"` assignment exists | Use its validated `PolicyValue` (`"true"`, `"false"`, or `"unspecified"`) | Authoritative MVP site assignment found. |
| `"site"` assignment missing | Treat policy as unspecified (`hasPolicy = false`) | No canonical site policy provided; missing does not imply `false`. |
| Origin-based assignment exists (e.g. `targetRef="https://example.com"`), `"site"` missing | Treat policy as unspecified (`hasPolicy = false`) | Evaluator does not silently adopt uncanonicalized origin assignments. |
| Both `"site"` and origin assignments exist with conflicting values | Evaluator queries `"site"`, ignores origin | `PolicyIndex` indexes distinct `TargetRef` values independently. Evaluator only reads authoritative `"site"`. |
| Multiple assignments for identical `(PolicyScopeSite, "site", PolicyKeySitemapExpected)` with conflicting values | Rejected at index build time (`ErrConflictingPolicy`) | Existing `PolicyIndex` invariant preserved; conflicting identical keys are forbidden. |

Converting external, legacy, or origin-based policy inputs into the canonical `"site"` target reference is an **upstream input normalization concern** (e.g. in CLI flag parsing or project configuration loading), not an evaluator runtime concern. The engine evaluator must not implement ad-hoc fallback logic that could mask configuration contradictions.

8. **Traceability through EvidenceRefs:**
   - Provenance is fully traceable:
     ```go
     ref := audit.RuleEvidenceRef{
         RuleEvidenceRefID: audit.RuleEvidenceRefID(fmt.Sprintf("ref:%s:%s:sitemap_expected", ruleResultID, policyAssignment.PolicyAssignmentID)),
         RuleResultID:      ruleResultID,
         EvidenceType:      "PROJECT_POLICY_ASSIGNMENT",
         EvidenceRef:       string(policyAssignment.PolicyAssignmentID),
         Field:             "sitemap_expected",
         ObservedValue:     policyAssignment.PolicyValue,
         Role:              audit.EvidenceRoleContext,
     }
     ```
   - `policyAssignment.Source` preserves whether policy came from `USER_INPUT`, `PROJECT_CONFIGURATION`, or `IMPORTED_CONFIGURATION`.

### 2.3 Inspection of Sitemap Discovery Evidence

The V1.8c adapter normalization layer exposes complete discovery telemetry on `SubjectSite` ([internal/audit/adapter/sitemap.go:264-379](../internal/audit/adapter/sitemap.go#L264-L379)):

| Normalized Field | Subject Type | Subject Ref | Source Evidence Ref | Purpose in AR-DISC-001 |
|---|---|---|---|---|
| `sitemaps_found_count` | `SubjectSite` | `"site"` | `sitecrawl_sitemap_discovery:<run_id>` | Discovered count (`>= 0`) |
| `sitemap_discovery_complete` | `SubjectSite` | `"site"` | `sitecrawl_sitemap_discovery:<run_id>` | Completeness boolean (`"true"`/`"false"`) |
| `sitemap_discovery_status` | `SubjectSite` | `"site"` | `sitecrawl_sitemap_discovery:<run_id>` | Status (`COMPLETED`, `NOT_ATTEMPTED`, `ATTEMPTED_INCOMPLETE`, `FAILED`) |
| `discovery_sources` | `SubjectSitemap` | `sm:<run_id>:<id>` | `sitecrawl_sitemaps:<run_id>:<id>` | Provenance channels (`robots_txt`, `common_path`, `sitemap_index`) |
| `sitemap_depth_capped` | `SubjectSite` | `"site"` | `sitecrawl_sitemap_discovery:<run_id>` | Truncation flag |
| `sitemap_urls_capped` | `SubjectSite` | `"site"` | `sitecrawl_sitemap_discovery:<run_id>` | Truncation flag |
| `sitemap_byte_capped` | `SubjectSite` | `"site"` | `sitecrawl_sitemap_discovery:<run_id>` | Truncation flag |

Additionally, `snapshot.SitemapDiscoveryComplete` is a top-level boolean on `EvidenceSnapshot`.

### 2.4 Evidence Differentiation Matrix

Current normalized evidence can deterministically distinguish all operational states:

| Operational State | `sitemap_discovery_complete` | `sitemaps_found_count` | `sitemap_discovery_status` | `sitemap_expected` Policy | Evaluator Outcome |
|---|---|---|---|---|---|
| **1. Complete, >=1 sitemaps found** | `true` | `>= 1` | `COMPLETED` | Any (`true`, `false`, `unspecified`, absent) | `PASS` |
| **2. Complete, 0 found, policy true** | `true` | `0` | `COMPLETED` | `true` | `WARNING` |
| **3. Complete, 0 found, policy unspecified** | `true` | `0` | `COMPLETED` | `unspecified` or absent | `MANUAL_REVIEW` |
| **4. Complete, 0 found, policy false** | `true` | `0` | `COMPLETED` | `false` | `NOT_APPLICABLE` |
| **5. Discovery incomplete (capped/timeout)** | `false` | Any | `ATTEMPTED_INCOMPLETE` | Any | `UNKNOWN` |
| **6. Discovery disabled (crawl option false)** | `false` | `0` | `NOT_ATTEMPTED` | Any | `UNKNOWN` |
| **7. Preconditions not met (sources not evaluated)** | `false` | `0` | `NOT_ATTEMPTED` / Legacy | Any | `UNKNOWN` |

### 2.5 Proposed Integration Contract

1. **Policy Key:** `audit.PolicyKeySitemapExpected` (`"sitemap_expected"`).
2. **Storage Source:** In-memory `ProjectPolicyAssignment` passed via `EvaluationContext.Policies`.
3. **Lookup API:**
   ```go
   policyAssignment, hasPolicy := policies.Get(audit.PolicyScopeSite, "site", audit.PolicyKeySitemapExpected)
   ```
4. **Missing-Policy Behavior:** If `!hasPolicy` or `policyAssignment.PolicyValue == audit.PolicyValueUnspecified`:
   - If `sitemaps_found_count > 0` -> `PASS`.
   - If `sitemaps_found_count == 0` -> `MANUAL_REVIEW`.
   - Never default missing policy to `false` (which would trigger `NOT_APPLICABLE`) or `true` (which would trigger `WARNING`).
5. **Result Subject and Scope Validation:**
   - **`SubjectType`:** `audit.SubjectSite` (defined constant `"SITE"` in [internal/audit/status.go:308](../internal/audit/status.go#L308), validated by `ValidateEvaluationSubjectType`).
   - **`SubjectRef`:** `"site"` (matches normalized evidence emitted by adapter).
   - **`Scope`:** `audit.ScopeSitewide` (defined constant `"SITEWIDE"` in [internal/audit/status.go:338](../internal/audit/status.go#L338), validated by `ValidateAuditReportScope`).
   - **Type Distinction:** `audit.PolicyScopeSite` (`"SITE"`) is a policy targeting scope; `audit.AuditReportScope` (`"SITEWIDE"`) is a report findings scope. They are distinct types in the domain model and must not be conflated.
   - **Why Site Level:** Sitemap presence evaluates whether the entire website publishes an XML sitemap. If zero sitemaps are found on the site, no `SubjectSitemap` entity exists. Creating a result on a non-existent document subject would require synthesizing a phantom subject, which violates data model invariants ([knowledge/09-data-model.md](../knowledge/09-data-model.md)). Thus, evaluation must occur on `SubjectSite`.
6. **Classification:** **`NEEDS_POLICY_WIRING`**
   - *Rationale:* All evidence is present and normalized; schema and validation exist in `internal/audit/policy.go`. The evaluator implementation `internal/audit/engine/disc001.go` is not yet written, and the canonical `TargetRef = "site"` wiring and unit tests must be implemented.

---

## 3. AR-DISC-002 HTTP Status Analysis

### 3.1 Rule Identity & Frozen Manifest Contract

- **Rule ID:** `AR-DISC-002`
- **Parent Check:** `DISC-002` (*Sitemap có chỉ chứa URL 200, canonical và indexable không?*)
- **Rule Name:** Sitemap document returns 200
- **Lifecycle:** Discover
- **Category:** `sitemaps`
- **Automation Class:** `deterministic`
- **Default Severity:** `P1`
- **Required Inputs:** `sitemap_url`, `sitemap_fetch_status`
- **Preconditions:** Sitemap discovered or explicitly supplied.
- **Contract Outcomes:**
  - `PASS`: Response is 200.
  - `FAIL`: HTTP response is non-200.
  - `UNKNOWN`: Request fails before HTTP response.
  - `WARNING`: Not used.
  - `MANUAL_REVIEW`: Not used.
  - `NOT_APPLICABLE`: Not used.
- **Evidence Fields:** `sitemap_url`, `fetch_status`, `fetch_error`, `observed_at`
- **Source References:** `SRC-GOOGLE-SITEMAP-001`

### 3.2 HTTP Evidence Trace

The end-to-end evidence pipeline is traced as follows:

```
Sitemap GET Request
      ↓
SiteCrawl [fetchSitemapWithTelemetry]
(records initialStatus, hops, redirectTo, finalStatus, fetchComplete, fetchError)
      ↓
SQLite [sitecrawl_sitemaps table]
(stores initial_status, final_status, redirect_hops, redirect_to, fetch_complete, fetch_error)
      ↓
Audit Adapter [internal/audit/adapter/sitemap.go]
(maps to SubjectSitemap observations: initial_http_status, final_http_status, sitemap_redirect_hops, sitemap_final_url, sitemap_fetch_error, sitemap_fetch_complete)
      ↓
EvidenceSnapshot & EvidenceIndex
(read-only indexed view consumed by Engine evaluators)
```

#### Meaning and Source of Telemetry Fields

1. `initial_http_status`:
   - *Source:* `SiteCrawl.fetchSitemapWithTelemetry` via `http.Client.CheckRedirect` inspecting `via[0].Response.StatusCode` ([internal/sitecrawl/sitemap.go:374-382](../internal/sitecrawl/sitemap.go#L374-L382)). If no redirect occurs (`hops == 0`), set to `res.StatusCode`.
   - *Meaning:* The HTTP status code returned by the server on the initial request to `sitemap_url`.
2. `final_http_status`:
   - *Source:* `res.StatusCode` from terminal HTTP response after following up to 10 redirects ([internal/sitecrawl/sitemap.go:407](../internal/sitecrawl/sitemap.go#L407)).
   - *Meaning:* The HTTP status code of the terminal document payload that was retrieved.
3. `sitemap_redirect_hops`:
   - *Source:* `len(via)` captured in `CheckRedirect`.
   - *Meaning:* Total number of HTTP redirects traversed (0 for direct response).
4. `sitemap_final_url`:
   - *Source:* `req.URL.String()` from the last redirect hop.
   - *Meaning:* The terminal destination URL. Emitted only when redirected (`hops > 0`).
5. `sitemap_fetch_error`:
   - *Source:* `err.Error()` from `http.NewRequestWithContext` or `reqClient.Do(req)`.
   - *Meaning:* Network, TLS, DNS, or timeout error string if the request failed before completing.
6. `sitemap_fetch_complete`:
   - *Source:* Set to `true` when `reqClient.Do(req)` returned without network error ([internal/sitecrawl/sitemap.go:406](../internal/sitecrawl/sitemap.go#L406)).
   - *Meaning:* Conclusive transport completion flag. **Does not mean HTTP 200.** (An HTTP 404 or 500 has `sitemap_fetch_complete=true`).

### 3.3 Representation of the 10 HTTP Scenarios

| Scenario | `initial_http_status` | `final_http_status` | `sitemap_redirect_hops` | `sitemap_final_url` | `sitemap_fetch_error` | `sitemap_fetch_complete` |
|---|---|---|---|---|---|---|
| **1. Direct HTTP 200** | `200` | `200` | `0` | `""` | `""` | `true` |
| **2. Direct HTTP 301 → final 200** | `301` | `200` | `1` | Destination URL | `""` | `true` |
| **3. Direct HTTP 302 → final 200** | `302` | `200` | `1` | Destination URL | `""` | `true` |
| **4. Redirect chain ending in 404** | `301` / `302` | `404` | `>= 1` | Destination URL | `""` | `true` |
| **5. Direct HTTP 404** | `404` | `404` | `0` | `""` | `""` | `true` |
| **6. Direct HTTP 500** | `500` | `500` | `0` | `""` | `""` | `true` |
| **7. Network failure before HTTP response** | `0` | `0` | `0` | `""` | `"dial tcp: ..."` | `false` |
| **8. Redirect loop** | `301` / `302` | `0` | `10` | Last URL reached | `"stopped after 10 redirects"` | `false` |
| **9. Redirect limit exceeded** | `301` / `302` | `0` | `10` | Last URL reached | `"stopped after 10 redirects"` | `false` |
| **10. Incomplete acquisition (canceled/timeout)** | `0` | `0` | `0` | `""` | `"context canceled"` | `false` |

*(Note: Scenarios 7, 8, 9, 10 cleanly produce `sitemap_fetch_complete=false` and `sitemap_fetch_error != ""`, which maps to `UNKNOWN` under the frozen contract).*

---

### 3.4 Analysis of the Open Contract Decision

The frozen manifest defines required input `sitemap_fetch_status`, PASS condition `response is 200`, and FAIL condition `HTTP response is non-200`. It leaves open whether the authoritative response is the **initial** response or the **terminal** response.

#### Option A — Initial Response Authoritative

Under Option A, `sitemap_fetch_status := initial_http_status`.
A sitemap URL returning `301 Moved Permanently` followed by `200 OK` evaluates as **`FAIL`**.

- **What this measures:** Direct endpoint response and strict transport cleanliness. Measures whether the URL declared in `robots.txt` or discovered at common paths serves the sitemap directly without redirect overhead.
- **Evidence required:** `initial_http_status`, `sitemap_url`.
- **Advantages:**
  1. Enforces strict infrastructure hygiene: flags obsolete declarations in `robots.txt` that force crawlers through redirect hops.
  2. Syntactically literal: evaluates the direct response code received for the requested sitemap URL.
  3. Conceptually mirrors `AR-DISC-004` (which fails 3xx on sitemap-listed URLs).
- **Limitations & Trade-offs:**
  1. Flags functioning sitemaps: If a website redirects `/sitemap.xml` to `/sitemap_index.xml` or HTTP to HTTPS, search engine crawlers follow the redirect and ingest the document. Option A labels this condition as a **P1 Critical Failure**, which may alarm users when search engines are successfully indexing the site.
  2. Contrast with `AR-DISC-003`: `AR-DISC-003` evaluates the retrieved XML body as `PASS` (parseable). Under Option A, an audit report would indicate that a sitemap failed its HTTP status check but passed XML parseability and URL extraction.

#### Option B — Terminal Response Authoritative

Under Option B, `sitemap_fetch_status := final_http_status`.
A sitemap URL returning `301 Moved Permanently` followed by `200 OK` evaluates as **`PASS`**, citing initial status, redirect hops, and destination URL as diagnostic context evidence.

- **What this measures:** Document retrievability and ingestibility. Measures whether the sitemap document can ultimately be reached and ingested by a bot following standard HTTP redirects.
- **Evidence required:** `final_http_status`, `sitemap_url`, with `initial_http_status`, `sitemap_redirect_hops`, and `sitemap_final_url` as diagnostic context.
- **Advantages:**
  1. Reflects bot ingestion reality: Avoids classifying a retrievable, fully functioning sitemap as a critical failure.
  2. Architectural alignment with `AR-DISC-003`: `AR-DISC-003` preconditions on sitemap returning 200 and body being retrieved. Under Option B, both rules agree that the document was successfully fetched.
  3. Preserves redirect awareness: `sitemap_redirect_hops`, `initial_http_status`, and `sitemap_final_url` remain visible in `EvidenceRefs` and `ObservedSummary`, maintaining full transparency.
- **Limitations:**
  - Does not emit a failure or warning for redirect latency within `AR-DISC-002` (since `WARNING` is "not used" in the frozen contract for `AR-DISC-002`).

---

## 4. Official Source Assessment

Authoritative sources from `knowledge/06-research-sources.md` and primary webmaster documentation were consulted:

### 4.1 Primary Platform Evidence & Qualification

1. **Google Search Central — Sitemaps Overview (`SRC-GOOGLE-SITEMAP-001`):**
   - *Source URL:* `https://developers.google.com/search/docs/crawling-indexing/sitemaps/overview`
   - *Explicit Requirement:* The sitemap must be available to crawlers and return a valid HTTP response with supported XML structure.
   - *Redirect Recommendation:* Google documentation recommends linking directly to the canonical sitemap URL in `robots.txt` to minimize latency and conserve crawl efficiency.
   - *Citing Redirects:* Googlebot adheres to standard HTTP redirect handling and follows redirects for discovered URLs, including sitemap URLs. Google does not specify HTTP 3xx redirection of a sitemap document as a fatal parsing error.
   - *Cross-Domain Qualification:* Google Search Central provides specific documentation for multi-site sitemaps ("Managing sitemaps for multiple sites"), requiring Search Console ownership verification when a sitemap on Domain A references target URLs on Domain B. However, applying this cross-domain multi-site rule to HTTP redirect hops of sitemap files is an **engineering inference**, not an explicit statement in primary sitemap documentation. We explicitly qualify this distinction.

2. **Google Search Central — Canonicalization & 301 Redirects (`SRC-GOOGLE-CANONICAL-001`):**
   - *Source URL:* `https://developers.google.com/search/docs/crawling-indexing/canonicalization`
   - Confirms that permanent 301 redirects are followed by Googlebot and transfer canonical signals.

3. **Sitemaps.org Protocol:**
   - *Source URL:* `https://www.sitemaps.org/protocol.html`
   - Governs XML schemas (`<urlset>`, `<sitemapindex>`), size limits (50MB, 50,000 URLs), and UTF-8 encoding.
   - Does not govern HTTP status codes or prohibit HTTP redirection.

### 4.2 Distinguishing Fact from Engineering Choices

| Topic | Explicit Official Requirement | Recommended Practice | Engineering / Product Contract Choice | Audit Engine Stance |
|---|---|---|---|---|
| **Sitemap Document Redirects** | Document must be retrievable and parseable. | Declare the direct, final 200 URL in `robots.txt`. | Whether `AR-DISC-002` evaluates initial vs. terminal HTTP response. | **Contract Decision `DEC-V1.8E-001`:** Option B recommended pending external review; Option A retained as valid alternative. |
| **Sitemap-listed URLs** | Listed URLs must be canonical 200 OK and indexable. | Sitemaps should contain only canonical targets. | N/A (Confirmed requirement). | **Enforced.** Handled by `AR-DISC-004` (fails 3xx) and `AR-DISC-005`/`006`. |
| **Absence of Sitemap** | Sitemaps are optional for small (<500 URLs), well-linked sites. | Useful for large, complex, or rapidly updating sites. | How missing sitemap is classified when policy is unspecified. | **Enforced.** Handled by `AR-DISC-001` (yields `MANUAL_REVIEW`, never FAIL). |

---

## 5. Cross-Rule Consistency Review

Compatibility between all six sitemap rules was verified:

```
[SubjectSite / Scope: SITEWIDE]
    AR-DISC-001: Sitemap presence matches project policy (Assisted)
        ├── Complete + Found >= 1    ──> PASS
        ├── Complete + 0 + Policy T  ──> WARNING
        ├── Complete + 0 + Policy ?  ──> MANUAL_REVIEW
        ├── Complete + 0 + Policy F  ──> NOT_APPLICABLE
        └── Incomplete / Disabled    ──> UNKNOWN

[SubjectSitemap / Scope: SITE_CONFIGURATION]
    AR-DISC-002: Sitemap document returns 200 (Deterministic)
        ├── Direct 200 / Final 200   ──> PASS (under Option B)
        ├── Non-200 (4xx/5xx/Final)  ──> FAIL
        └── Network / Timeout / Loop ──> UNKNOWN
    
    AR-DISC-003: Sitemap is parseable (Deterministic, LOCKED)
        ├── Valid URLSET / Index     ──> PASS
        ├── Malformed XML / Empty    ──> FAIL
        ├── Non-200 / Body not recv  ──> NOT_APPLICABLE
        └── Parser unavailable       ──> UNKNOWN

[SubjectSitemapEntry / Scope: URL]
    AR-DISC-004: Listed URL returns final 200 (Deterministic, LOCKED)
        ├── Direct 200 OK            ──> PASS
        ├── 3xx / 4xx / 5xx          ──> FAIL
        └── Uncrawled / Timeout      ──> UNKNOWN
    
    AR-DISC-005: Listed URL is not noindex (Deterministic, LOCKED)
        ├── 200 HTML, noindex=false  ──> PASS
        ├── 200 HTML, noindex=true   ──> FAIL
        └── Non-200 / Non-HTML       ──> NOT_APPLICABLE
    
    AR-DISC-006: Listed URL canonical self-reference (Deterministic, LOCKED)
        ├── 200 HTML, self-canonical ──> PASS
        ├── 200 HTML, foreign canon  ──> FAIL
        └── No canonical declared    ──> NOT_APPLICABLE
```

### 5.1 Verification of Structural Invariants

1. **Document Status vs. Listed URL Status:**
   - `AR-DISC-002` operates on `SubjectSitemap` (`sm:<run_id>:<id>`).
   - `AR-DISC-004` operates on `SubjectSitemapEntry` (`sme:<run_id>:<id>:<seq>`).
   - The status of a sitemap document never overwrites the status of URLs listed inside it.
2. **Parser Success vs. Fetch Success:**
   - `AR-DISC-002` evaluates HTTP transport.
   - `AR-DISC-003` evaluates XML schema parseability.
   - If HTTP fetch is 404 or 500, `AR-DISC-003` emits `NOT_APPLICABLE` (body not retrieved), preventing false syntax failure reports.
3. **Redirect Destination Status Preservation:**
   - Raw persistence and normalized evidence preserve `initial_http_status` and `final_http_status` as independent observations. Neither overwrites the other.
4. **Missing Sitemap is Never Automatic Error:**
   - `AR-DISC-001` emits `MANUAL_REVIEW` when policy is unspecified and `NOT_APPLICABLE` when `sitemap_expected=false`. It never emits `FAIL`.
5. **Missing Policy is Not False:**
   - Absence of policy is handled strictly as unspecified intent. It does not default to `false`.
6. **UNKNOWN is Not FAIL:**
   - Network timeouts, connection failures, and redirect loops produce `UNKNOWN`, honoring Product Principle 7 (*Do not manufacture certainty*).
7. **Zero Regression to Locked Rules:**
   - The four locked evaluators (`AR-DISC-003`, `AR-DISC-004`, `AR-DISC-005`, `AR-DISC-006`) remain completely unmodified.

---

## 6. Proposed Contracts

### 6.1 Proposed Contract for AR-DISC-001

```yaml
rule_id: AR-DISC-001
parent_check: DISC-001
rule_version: 1
name: Sitemap presence matches project policy
lifecycle: Discover
category: sitemaps
automation: assisted
default_severity: P1
subject_type: SITE
subject_ref: "site"
scope: SITEWIDE

required_inputs:
  - sitemaps_found_count
  - sitemap_discovery_complete
  - sitemap_expected

preconditions:
  - root/robots sitemap discovery sources evaluated (sitemap_discovery_status != "NOT_ATTEMPTED")

evaluation:
  pass:
    condition: sitemaps_found_count > 0
    summary: "One or more sitemaps discovered ({count} sitemap(s) found via {sources})."
  warning:
    condition: sitemaps_found_count == 0 && sitemap_expected == "true" && sitemap_discovery_complete == true
    summary: "No sitemaps discovered, but project policy explicitly expects a sitemap."
  manual_review:
    condition: sitemaps_found_count == 0 && (sitemap_expected == "unspecified" || sitemap_expected is missing) && sitemap_discovery_complete == true
    summary: "No sitemaps discovered and project policy is unspecified; review whether this website requires an XML sitemap."
  not_applicable:
    condition: sitemaps_found_count == 0 && sitemap_expected == "false" && sitemap_discovery_complete == true
    summary: "No sitemaps discovered and project policy explicitly specifies that no sitemap is expected."
  unknown:
    condition: sitemap_discovery_complete == false
    summary: "Sitemap discovery could not complete ({stop_reason}); sitemap presence cannot be determined."

evidence_refs:
  - field: sitemaps_found_count (Role: Primary)
  - field: sitemap_discovery_complete (Role: Prerequisite)
  - field: sitemap_expected (Role: Primary when sitemaps_found_count == 0; Role: Context otherwise)
  - field: discovery_sources (Role: Context)
```

---

### 6.2 Proposed Contract for AR-DISC-002 (Under Recommended Option B)

```yaml
rule_id: AR-DISC-002
parent_check: DISC-002
rule_version: 1
name: Sitemap document returns 200
lifecycle: Discover
category: sitemaps
automation: deterministic
default_severity: P1
subject_type: SITEMAP
subject_ref: sm:<audit_run_id>:<sitemap_id>
scope: SITE_CONFIGURATION

required_inputs:
  - sitemap_url
  - final_http_status
  - sitemap_fetch_complete

preconditions:
  - sitemap discovered or explicitly supplied

evaluation:
  pass:
    condition: sitemap_fetch_complete == true && final_http_status == 200
    summary: "Sitemap document returned HTTP 200 OK (terminal response)."
    context: if redirect_hops > 0, include "(redirected from {initial_status} via {redirect_hops} hop(s) to {final_url})"
  fail:
    condition: sitemap_fetch_complete == true && final_http_status != 200
    summary: "Sitemap document returned non-200 HTTP response ({final_status})."
  unknown:
    condition: sitemap_fetch_complete == false || final_http_status == 0
    summary: "Sitemap request failed before receiving a valid HTTP response ({fetch_error})."

evidence_refs:
  - field: final_http_status (Role: Primary)
  - field: sitemap_url (Role: Context)
  - field: initial_http_status (Role: Context)
  - field: sitemap_redirect_hops (Role: Context)
  - field: sitemap_final_url (Role: Context, if redirected)
  - field: sitemap_fetch_error (Role: Primary when UNKNOWN)
```

---

## 7. Decision Register

The following formal decision register details all policy and contract choices:

| Decision ID | Rule | Question | Options | Recommendation | Evidence / Rationale | Status & Approval Required |
|---|---|---|---|---|---|---|
| **`DEC-V1.8E-001`** | `AR-DISC-002` | What is the authoritative HTTP response status for sitemap documents? | **Option A:** Initial status (301 → FAIL).<br>**Option B:** Terminal status (301 → 200 → PASS). | **Option B (Terminal status authoritative)** with initial status and redirect hops cited as context EvidenceRefs. | Crawlers follow redirects; Option B evaluates document retrievability and aligns with `AR-DISC-003` XML parse success. Option A remains a valid alternative if strict endpoint hygiene is desired. | **PENDING EXTERNAL APPROVAL** |
| **`DEC-V1.8E-002`** | `AR-DISC-001` | What `TargetRef` convention should be used for `PolicyScopeSite` lookups? | **Option 1:** Fixed `"site"`.<br>**Option 2:** Root URL origin.<br>**Option 3:** Standardized `"site"` with fallback to origin. | **Option 1 (Canonical `TargetRef = "site"` only for MVP)**. No automatic origin fallback. | `PolicyIndex` indexes by `(Scope, TargetRef, Key)`. Eliminates ambiguity and conflict risks when both exist. Upstream normalization must map inputs to `"site"`. | **PENDING EXTERNAL APPROVAL** |
| **`DEC-V1.8E-003`** | `AR-DISC-001` | What is the supported `SubjectType` and `AuditReportScope` for `AR-DISC-001` RuleResult? | **Option 1:** `SubjectSite` (`"SITE"`) / `ScopeSitewide` (`"SITEWIDE"`).<br>**Option 2:** `SubjectSite` / `ScopeSiteConfiguration` (`"SITE_CONFIGURATION"`). | **Option 1 (`SubjectSite` / `ScopeSitewide`)**. | Uses exact supported constants from `internal/audit/status.go`. Preserves `"site"` subjectRef. Evaluates sitewide sitemap availability without phantom entities. | **PENDING EXTERNAL APPROVAL** |
| **`DEC-V1.8E-004`** | `AR-DISC-002` | Does the manifest or rule registry require text clarification? | **Option 1:** Retain existing wording.<br>**Option 2:** Clarify `pass_condition` to specify terminal 200. | **Option 1** (Retain frozen manifest and registry unchanged). | Manifest is frozen. If Option B is approved, manifest clarification is optional, but documentation clarity in evaluator docstring is mandatory. | **APPROVED CONTRACT PRESERVATION** |
| **`DEC-V1.8E-005`** | `AR-DISC-002` | Is a `NormalizationVersion` bump required? | **Option 1:** Retain `v1.9.0`.<br>**Option 2:** Bump to `v1.10.0`. | **Option 1 (Retain `v1.9.0`)**. | V1.8c already normalized `initial_http_status`, `final_http_status`, `redirect_hops`, and `fetch_complete`. Evaluator directly consumes existing fields with zero adapter diff. | **APPROVED CONTRACT PRESERVATION** |

---

## 8. Implementation Readiness

### 8.1 AR-DISC-001 Readiness

- **Status:** **`NEEDS_POLICY_WIRING`**
- **Exact Blockers:**
  1. No Go evaluator file (`internal/audit/engine/disc001.go`) exists.
  2. Engine registry does not have `AR-DISC-001` registered.
  3. Site-level policy lookup must be wired to canonical `TargetRef = "site"`.
- **Required Code Changes (when unblocked):**
  - Implement `internal/audit/engine/disc001.go` consuming `SubjectSite` and `policies.Get(PolicyScopeSite, "site", PolicyKeySitemapExpected)`.
  - Register `ruleIDDISC001` in `internal/audit/engine/engine.go`.
  - Add unit tests in `internal/audit/engine/disc001_test.go` covering all 7 operational states.
- **Dependencies:** `SubjectSite` observations (already available in `v1.9.0`).
- **Suggested Implementation Sequence:** Milestone V1.8f (Policy Wiring & AR-DISC-001 Evaluator).

### 8.2 AR-DISC-002 Readiness

- **Status:** **`BLOCKED`**
- **Exact Blockers:**
  1. Open Contract Decision `DEC-V1.8E-001` requires formal external approval.
  2. Cannot implement evaluator until authoritative status choice (Initial vs. Terminal) is locked.
- **Required Code Changes (when unblocked):**
  - Implement `internal/audit/engine/disc002.go` following the approved contract decision.
  - Register `ruleIDDISC002` in `internal/audit/engine/engine.go`.
  - Add unit tests in `internal/audit/engine/disc002_test.go` covering all 10 HTTP scenarios.
- **Dependencies:** External approval of `DEC-V1.8E-001`.
- **Suggested Implementation Sequence:** Milestone V1.8g (following external approval of `DEC-V1.8E-001`).

---

## 9. Explicit Non-Goals

This milestone explicitly does **not**:
1. Implement evaluators for `AR-DISC-001` or `AR-DISC-002`.
2. Register new rules in `internal/audit/engine/engine.go` (executable count strictly remains `17/47`).
3. Modify SQLite database tables, crawler fetch routines, or adapter normalization logic.
4. Bump `NormalizationVersion` (remains `v1.9.0`).
5. Alter any of the four locked sitemap evaluators (`AR-DISC-003`, `AR-DISC-004`, `AR-DISC-005`, `AR-DISC-006`).
6. Unilaterally declare `DEC-V1.8E-001`, `DEC-V1.8E-002`, or `DEC-V1.8E-003` locked without external review.
7. Introduce new values to `EvaluationSubjectType` or `AuditReportScope` enums.
8. Implement Finding grouping, issue aggregation, UI reporting, or score calculation.

---

## 10. External Approval Checklist

Before implementing the remaining sitemap rules in subsequent milestones, the following approvals are required:

- [ ] **Approval of `DEC-V1.8E-001`:** Confirm Option B (terminal HTTP response authoritative for `AR-DISC-002`, with initial status and redirect hops preserved as diagnostic context).
- [ ] **Approval of `DEC-V1.8E-002`:** Confirm canonical site policy lookup convention (`TargetRef = "site"` only for MVP; no automatic origin fallback).
- [ ] **Approval of `DEC-V1.8E-003`:** Confirm `AR-DISC-001` SubjectType as `SubjectSite` (`"SITE"`) and Scope as `ScopeSitewide` (`"SITEWIDE"`).
- [ ] **Approval of Implementation Sequencing:** Confirm proceeding with `AR-DISC-001` in V1.8f and `AR-DISC-002` in V1.8g upon resolution of `DEC-V1.8E-001`.

---

## 11. Conclusion & Next Step

Milestone **Audit V1.8e** is complete from an architectural and research standpoint. All evidence dependencies, platform documentation facts, cross-rule interactions, and contract options are systematically documented.

**Status:** **V1.8e DESIGN COMPLETE — PENDING EXTERNAL CONTRACT REVIEW**
