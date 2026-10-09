# V1 Logical Data Model

**Status:** Frozen for V1 design
**Revision:** v1.4.4
**Depends on:** `07-v1-atomic-rule-manifest.md`, `08-system-architecture.md`
**Scope:** Logical data model only
**Database / ORM / storage engine:** Not selected

## 1. Purpose

Define the logical data contract required by V1:

```text
AuditRun
→ Raw Evidence
→ Normalized Observations
→ Evidence Snapshot
→ Atomic Rule Results
→ Aggregated Findings
```

This document defines entities, field ownership, relationships, identifiers, data lifecycle, normalization boundaries, evidence traceability and invariants. It does not prescribe SQL tables, ORM models, database technology, APIs, UI or crawler implementation.

## 2. Core modeling principles

### Observation and policy are separate

```text
effective_noindex = true     ← observed evidence
expected_indexable = true    ← explicit policy
AR-INDEX-001 = FAIL          ← rule conclusion
```

### URL identity is not fetch identity

One URL may be requested normally, under request profiles, as a canonical target, redirect target, probe or render source. Therefore:

```text
UrlResource ≠ FetchObservation
```

### URL identity is not discovery membership

A URL belongs to the website discovery universe only when it has legitimate discovery provenance.

```text
UrlResource + DiscoveryRecord → discovery membership
```

Fetches/probes alone do not create site graph membership.

### Raw and normalized evidence are separate

```text
Raw: X-Robots-Tag: noindex, nofollow
Normalized: tokens=[noindex,nofollow], effective_noindex=true
```

Rules consume normalized evidence; raw evidence is retained for reproduction/debugging.

### Rule results are immutable conclusions

A result evaluated against a frozen snapshot must preserve snapshot, rule/version, evidence, status, severity and evaluation time.

## 3. Top-level entity model

```text
AuditRun
│
├── ProjectPolicyAssignment[]
├── UrlResource[]
│   ├── DiscoveryRecord[]
│   ├── FetchObservation[]
│   │   └── RedirectHop[]
│   ├── HtmlObservation[]
│   ├── RobotsDecision[]
│   ├── LinkObservation[]
│   ├── StructuredDataBlock[]
│   └── RenderObservation[]
├── SitemapObservation[]
│   └── SitemapEntry[]
├── ProbeObservation[]
├── EvidenceSnapshot
│   └── NormalizedObservation[]
├── RuleResult[]
│   ├── RuleEvidenceRef[]
│   └── ManualReviewTask[]
└── Finding[]
    └── FindingMember[]
```

## 4. Identifier model

Logical identifiers:

```text
audit_run_id
url_id
policy_assignment_id
discovery_id
fetch_id
redirect_hop_id
html_observation_id
robots_decision_id
link_id
sitemap_id
sitemap_entry_id
structured_block_id
render_id
probe_id
snapshot_id
observation_id
rule_result_id
manual_review_task_id
finding_id
```

Format (UUID/ULID/database ID) is deferred.

## 5. AuditRun

Represents one complete audit execution.

Required logical fields:

```text
audit_run_id
start_url
normalized_start_url
started_at
completed_at
run_status
knowledge_snapshot
rule_manifest_version
execution_settings_ref
```

Workflow statuses:

```text
CREATED
ACQUIRING
NORMALIZING
SNAPSHOT_FROZEN
EVALUATING
AGGREGATING
COMPLETED
FAILED
CANCELLED
```

These are workflow statuses, not audit rule statuses.

Every run stores `knowledge_snapshot` and `rule_manifest_version`; each RuleResult also stores `rule_id` and `rule_version`.

## 6. ProjectPolicyAssignment

Policy is explicit and scoped rather than inferred.

```text
ProjectPolicyAssignment
├── policy_assignment_id
├── audit_run_id
├── policy_key
├── policy_value
├── scope
├── target_ref
├── source
└── supplied_at
```

V1 scopes:

```text
SITE
URL
```

Future DIRECTORY/TEMPLATE scopes are not required.

Site-level examples:

```text
preferred_origin
sitemap_expected
snippet_policy
googlebot_access_policy
oai_searchbot_access_policy
gptbot_training_policy
```

URL-level examples:

```text
expected_url_state
expected_crawlable
expected_indexable
priority_page
```

Policy provenance:

```text
USER_INPUT
PROJECT_CONFIGURATION
IMPORTED_CONFIGURATION
```

Do not use crawler or LLM inference as V1 policy provenance.

## 7. UrlResource

Represents URL identity, not a successful fetch or legitimate discovery.

```text
url_id
audit_run_id
url
normalized_url
scheme
host
port
path
query
fragment_removed
is_internal
origin
created_at
```

Normalization must be deterministic. HTTP fragments normally do not create distinct resource identities.

## 8. DiscoveryRecord

Represents legitimate entry into the site discovery universe.

```text
discovery_id
audit_run_id
url_id
discovery_type
source_url_id
source_ref
discovered_at
```

V1 types:

```text
START_URL
INTERNAL_LINK
SITEMAP
PAGINATION
SUPPLIED_URL_LIST
```

A URL can have multiple discovery records.

Only DiscoveryRecord grants graph membership. These must not create DiscoveryRecords by themselves:

```text
canonical target lookup
redirect continuation
slash counterpart test
host/protocol test
known-missing probe
bot-profile request
render resource request
```

## 9. FetchObservation

Represents one HTTP acquisition attempt.

```text
fetch_id
audit_run_id
url_id
acquisition_purpose
request_profile
requested_at
completed_at
fetch_attempted
status
final_url_id
content_type
response_time_ms
fetch_error_type
tls_valid
challenge_detected
response_headers_ref
body_artifact_ref
observed_at
```

Acquisition purpose examples:

```text
CRAWL
ROBOTS_FETCH
SITEMAP_FETCH
CANONICAL_TARGET
REDIRECT_TARGET
HOST_VARIANT_PROBE
SLASH_VARIANT_PROBE
KNOWN_MISSING_PROBE
BOT_PROFILE_COMPARE
RENDER_SOURCE
RENDER_RESOURCE
```

Request profiles:

```text
DEFAULT
GOOGLEBOT_PROFILE
OAI_SEARCHBOT_PROFILE
GPTBOT_PROFILE
CUSTOM_BOT_PROFILE
```

These do not represent verified crawler identity.

Fetch error examples:

```text
DNS_ERROR
TLS_ERROR
CONNECTION_ERROR
TIMEOUT
BODY_READ_ERROR
REDIRECT_LIMIT
UNKNOWN_NETWORK_ERROR
```

## 10. RedirectHop

Store redirect paths as ordered evidence:

```text
redirect_hop_id
fetch_id
hop_index
source_url
status
location_raw
resolved_target_url
observed_at
```

### Redirect acquisition and normalization semantics

1. **Redirect hop provenance and typed entity**:
   - Frozen SiteCrawl stores `Hop.Location` after `resolveLocation(...)`, which is already a resolved target URL and NOT the original raw Location header.
   - Frozen SiteCrawl does not preserve the original raw Location header.
   - In adapter-created `audit.RedirectHop`, `LocationRaw` remains empty (`""`) to prevent false provenance.
   - `ResolvedTargetURL` contains the persisted resolved hop target.
   - Raw Location is never reconstructed, and frozen domain field names are unchanged.
2. **Redirect hop normalized evidence**:
   - `redirect_initial_observed`: Boolean evidence emitted when the initial HTTP status is known (`status > 0`). Evaluates to `true` for 3xx responses (`300 <= status < 400`), and `false` for known non-3xx responses. Withheld if HTTP status is unavailable.
   - `redirect_hop_count`: Count of persisted redirect hops. Preserves `0` when the initial response is known and no hop was persisted. Withheld when promoted count conflicts with preserved chain length (`len(Page.Redirects) != pr.redirectHops`) to avoid falsely presenting conflicting persisted evidence as authoritative.
   - `redirect_hop`: One normalized observation per persisted hop in exact chronological order without reordering. Contains a deterministic machine-readable JSON payload containing exactly: `hop_index`, `source_url`, `status`, and `resolved_target_url`.
3. **Persisted redirect count consistency & traversal completeness**:
   - When persisted redirect evidence is relevant, the adapter validates that promoted SQLite `redirect_hops` agrees with preserved `len(Page.Redirects)`.
   - If they conflict:
     - The adapter records an `EvidenceGap` (`GAP_REDIRECT_CHAIN_INCONSISTENT`);
     - Completeness remains unproven: authoritative `redirect_traversal_complete=true` is NOT emitted;
     - `redirect_loop_detected=false` is NOT emitted;
     - `redirect_final_url` is withheld;
     - `FetchObservation.FinalURLID` remains absent (`nil`);
     - `redirect_hop_count` is withheld;
     - Preserved `redirect_hop` observations continue to represent the actually preserved `Page.Redirects` chain.
   - `redirect_traversal_complete`: For an observed redirect without count conflict, evaluates to `true` only when all of the following hold:
     - `FollowRedirects = true`
     - Promoted `redirect_hops` equals `len(Page.Redirects)`
     - At least one hop is preserved
     - Crawler reached a terminal response
     - `Page.Error` is empty
   - Evaluates to `false` when incompleteness is positively known (and no conflicting counts exist), including:
     - Redirect following disabled (`FollowRedirects = false`)
     - Redirect loop (`Page.Error == "redirect loop"`)
     - Redirect-chain limit exceeded (`Page.Error == "redirect chain too long"`)
     - Invalid or missing Location header
     - Transport failure after a redirect (e.g. timeout or connection drop)
   - Unknown or conflicting states are never encoded as complete. Not emitted for non-redirect responses.
4. **Loop-state truthfulness**:
   - `redirect_loop_detected`: Emitted as `true` only when persisted crawler evidence explicitly records `Page.Error == "redirect loop"`.
   - Emitted as `false` only when redirect traversal is proven complete without a loop (`redirect_traversal_complete = true`).
   - For other incomplete traversals or conflicting persisted states, this field is withheld. A false loop state is never inferred from the mere absence of a loop error.
5. **Final URL semantics**:
   - `redirect_final_url`: Emitted only when `redirect_traversal_complete = true`. Uses the resolved target URL of the final persisted hop, normalized with Audit URL normalization rules. Conflicting promoted count vs preserved chain prevents final-target conclusions; `redirect_final_url` is withheld on conflict.
6. **Persisted final-status limitation**:
   - Runtime `fetched.FinalStatus` observed during redirect traversal is held in in-flight memory during crawl but is not persisted in the frozen `Page` model or SQLite schema.
   - Neither `final_status` nor `redirect_final_status` is emitted as a normalized observation, nor fabricated from a later independent fetch of the target URL (which constitutes a separate request and observation).
   - Consequently, `AR-CANON-010` and `AR-ACC-003` remain BLOCKED until authoritative source-chain final status is persisted.
7. **First-hop vs final-target distinction & `FetchObservation.FinalURLID` correction**:
   - `Page.RedirectTo` represents the first redirect target, not the final destination. The adapter must never map `RedirectTo` to `FetchObservation.FinalURLID`.
   - `FetchObservation.FinalURLID` is populated strictly when:
     (1) redirect traversal is complete (`redirect_traversal_complete = true`), and
     (2) no promoted count vs preserved chain conflict exists, and
     (3) the final normalized target URL maps uniquely to exactly one existing `UrlResource` in the snapshot.
   - If traversal is incomplete or conflicting, the final target is not in the snapshot, or the normalized URL identity is ambiguous, `FinalURLID` remains absent (`nil`).

## 11. RawArtifact

Large documents/payloads use an abstract artifact model:

```text
artifact_id
audit_run_id
artifact_type
content_hash
mime_type
byte_size
storage_ref
captured_at
```

Potential types:

```text
HTTP_BODY
RAW_HTML
ROBOTS_TXT
SITEMAP_XML
RENDERED_DOM
```

Physical storage is deferred.

## 12. RobotsDocumentObservation and RobotsDecision

Robots document:

```text
robots_document_id
audit_run_id
robots_url_id
fetch_id
parse_status
parse_error
raw_artifact_ref
observed_at
```

Effective decision:

```text
robots_decision_id
audit_run_id
url_id
robots_document_id
agent
allowed
matched_rule_type
matched_rule_pattern
observed_at
```

Agents:

```text
DEFAULT
GOOGLEBOT
OAI_SEARCHBOT
GPTBOT
```

## 13. HtmlObservation

```text
html_observation_id
audit_run_id
url_id
fetch_id
title
meta_description
h1_values[]
meta_robots_raw[]
x_robots_raw[]
canonical_raw_values[]
main_text_present
main_text_fingerprint
content_fingerprint
observed_at
```

`main_text_present` is technical evidence, not a content-quality verdict.

## 14. RobotsDirectiveObservation

```text
robots_directive_observation_id
audit_run_id
url_id
source
target
scope_unknown
raw_value
parsed_tokens[]
unsupported_tokens[]
parse_errors[]
effective_noindex
observed_at
```

Source values:

```text
META
HTTP_HEADER
RENDERED_META
```

Target values:
- generic: `*`
- explicit agent: normalized user-agent token (e.g. `googlebot`)
- unknown applicability: empty string with `scope_unknown = true`

Preserve source-level evidence before deriving effective state. Do not infer scope. On `RobotsDirectiveObservation`, `effective_noindex` indicates whether that individual directive source contributes a Googlebot-effective noindex restriction (`target` is `*` or `googlebot`, scope is known, tokens contain `noindex` or `none`). URL-level `effective_noindex` represents the final accumulated Googlebot-effective page-level state derived from:
- generic scope (`*`);
- explicit googlebot scope (`googlebot`);
- cumulative restrictive-rule accumulation (an applicable `noindex` or `none` cannot be cancelled by `index`).
Other agent scopes (e.g. GPTBot, OAI-SearchBot, Bingbot) do not affect Googlebot text search indexability. Unknown evidence, ambiguous scopes, and incomplete/raw-render evidence must remain absent (do not encode unknown as false).

## 15. CanonicalObservation

```text
canonical_observation_id
audit_run_id
url_id
canonical_count
raw_values[]
normalized_values[]
resolved_target_url_ids[]
parse_errors[]
observed_at
```

### Canonical acquisition and normalization semantics

1. **Raw canonical syntax is unavailable**: Frozen SiteCrawl resolves relative canonical references before persistence. Original raw `href` declaration syntax and raw syntax errors are not preserved; `raw_values` remains empty and `parse_errors` is not fabricated.
2. **SiteCrawl values are already resolved**: Values preserved by SiteCrawl represent crawler-resolved URLs, not raw declaration strings.
3. **Adapter-level normalized target semantics**: For trustworthy non-rendered HTML, the adapter normalizes each preserved declaration through Audit V1 URL normalization rules (`normalizeURL`), verifying valid absolute HTTP(S) schemes, hostnames, default port stripping, fragment removal, and case normalization. Each valid declaration emits `canonical_normalized_target`.
4. **Canonical count vs. distinct target count**:
   - `canonical_count`: Total number of canonical declarations preserved by SiteCrawl for the URL (including duplicates). Emits `canonical_count = 0` for non-rendered HTML with no canonical declarations. Rendered pages where raw head evidence is unavailable do not emit `canonical_count`.
   - `canonical_normalization_complete`: Emitted when `canonical_count > 0`. Evaluates to `true` only if every preserved declaration successfully normalizes to a valid HTTP(S) target; otherwise `false`.
   - `canonical_distinct_normalized_count`: Emitted only when `canonical_normalization_complete = true`. Represents the count of unique, distinct normalized targets (e.g. 2 declarations pointing to the same normalized target yield count=2, distinct=1).
5. **Source → target subject correlation**:
   - When canonical evidence resolves to exactly one distinct valid normalized target (`canonical_normalization_complete = true` and `canonical_distinct_normalized_count = 1`), the adapter attempts to correlate the target with an existing `UrlResource` in the snapshot using deterministic Audit URL normalization.
   - If exactly one URL resource matches, the adapter emits `canonical_target_subject_ref` with that resource's subject ref (`url:<audit_run_id>:<target_url_id>`).
   - If the target does not exist in the snapshot, no subject ref is synthesized.
   - If matching is ambiguous (multiple URL resources normalize to the same URL), `canonical_target_subject_ref` is withheld.
   - Target HTTP status and indexability are NOT duplicated onto the source URL; subsequent evaluators (`AR-CANON-006`, `AR-CANON-007`) traverse `canonical_target_subject_ref` to inspect the target's own evidence.

Target status is sourced from FetchObservation for the target URL.

## 16. LinkObservation

```text
link_id
audit_run_id
source_url_id
target_url_id
target_url_raw
target_url_resolved
element_tag
href_raw
anchor_text
link_location
navigation_candidate
navigation_candidate_reason
observed_at
```

Suggested normalized locations:

```text
HEAD
HEADER
NAV
MAIN
ARTICLE
ASIDE
FOOTER
UNKNOWN
```

Location does not imply business importance.

Derived graph metrics for legitimately discovered URLs:

```text
crawl_inlink_count
crawl_depth
shortest_discovered_path
```

Probe-only URLs do not participate.

## 17. SitemapObservation and SitemapEntry

Sitemap document:

```text
sitemap_id
audit_run_id
sitemap_url_id
fetch_id
document_type
parse_status
parse_error
raw_artifact_ref
observed_at
```

Document types:

```text
URLSET
SITEMAP_INDEX
UNKNOWN
```

Entry:

```text
sitemap_entry_id
audit_run_id
sitemap_id
listed_url_id
listed_url_raw
lastmod_raw
lastmod_normalized
observed_at
```

V1 may store `lastmod` but does not validate it against CMS history.

## 18. StructuredDataBlock

```text
structured_block_id
audit_run_id
url_id
format
block_index
raw_artifact_or_value_ref
parse_status
parse_error
observed_at
```

Formats:

```text
JSON_LD
MICRODATA
RDFA
```

This entity supports syntax/extraction only and does not imply page-type appropriateness or GEO value.

## 19. RenderObservation and normalized render evidence

```text
render_id
audit_run_id
url_id
render_requested
render_status
render_error
rendered_dom_artifact_ref
rendered_at
```

Status:

```text
NOT_SELECTED
SUCCESS
FAILED
```

When successful, normalize:

```text
title_rendered
canonical_rendered[]
meta_robots_rendered[]
h1_rendered[]
main_text_present_rendered
internal_links_rendered[]
```

## 20. RenderFieldComparison

```text
render_comparison_id
audit_run_id
url_id
render_id
field
raw_value
rendered_value
difference_type
observed_at
```

Fields:

```text
TITLE
CANONICAL
META_ROBOTS
H1
MAIN_TEXT_PRESENCE
INTERNAL_LINK_SET
```

Difference types:

```text
UNCHANGED
ADDED
REMOVED
CHANGED
SET_CHANGED
```

`AR-RENDER-*` rules consume these normalized comparisons.

## 21. RenderResourceObservation

Required for CSS/JS blocking-assisted semantics:

```text
render_resource_observation_id
audit_run_id
page_url_id
resource_url_id
resource_type
robots_allowed
load_status
render_dependency_observed
render_diff_evidence_ref
observed_at
```

Resource types:

```text
CSS
JAVASCRIPT
OTHER
```

## 22. ProbeObservation

```text
probe_id
audit_run_id
probe_type
target_url_id
related_url_id
expected_probe_state
fetch_id
created_at
```

V1 types:

```text
KNOWN_MISSING
HOST_VARIANT
SLASH_VARIANT
BOT_PROFILE
```

Known-missing probes preserve generated URL, final status/final URL and content fingerprint and never create legitimate DiscoveryRecords.

## 23. ProfileComparisonObservation

Derived from multiple FetchObservations:

```text
profile_comparison_id
audit_run_id
url_id
baseline_fetch_id
comparison_fetch_id
profile
baseline_status
comparison_status
baseline_challenge_detected
comparison_challenge_detected
baseline_response_fingerprint
comparison_response_fingerprint
observed_at
```

This means response comparison only, not verified crawler identity.

## 24. NormalizedObservation

Generic conceptual Rule Engine boundary; it need not be one physical table.

```text
observation_id
audit_run_id
snapshot_id
subject_type
subject_ref
field
value
derivation_type
source_evidence_refs[]
observed_at
```

Derivation type:

```text
DIRECT
NORMALIZED
DERIVED
```

Derived observations retain source evidence references.

## 25. EvidenceSnapshot

```text
snapshot_id
audit_run_id
created_at
frozen_at
snapshot_status
normalization_version
```

Statuses:

```text
BUILDING
FROZEN
```

Rule evaluation is permitted only on FROZEN snapshots.

V1 may use one final snapshot per AuditRun. Frozen evidence must not change in place.

## 26. RuleDefinition

Logical machine-readable representation of one `AR-*` entry:

```text
rule_id
parent_check
rule_version
name
lifecycle
automation
required_inputs[]
preconditions[]
pass_condition
fail_condition
warning_condition
manual_review_condition
unknown_condition
not_applicable_condition
default_severity
evidence_fields[]
source_refs[]
```

Physical YAML/JSON format is deferred.

## 27. RuleResult

```text
rule_result_id
audit_run_id
snapshot_id
rule_id
parent_check
rule_version
subject_type
subject_ref
status
severity
scope
observed_summary
expected_summary
evaluated_at
```

Allowed rule statuses exactly:

```text
PASS
WARNING
FAIL
MANUAL_REVIEW
NOT_APPLICABLE
UNKNOWN
```

There is no `INFO` RuleResult status.

Suggested evaluation subject types:

```text
SITE
URL
LINK
SITEMAP
SITEMAP_ENTRY
STRUCTURED_DATA_BLOCK
RESOURCE
RENDER_COMPARISON
PROBE
SITE_CONFIGURATION
```

Audit report scope remains separate:

```text
URL
TEMPLATE
DIRECTORY
SITEWIDE
SITE_CONFIGURATION
```

Automatic template/directory inference is not required.

Severity values:

```text
P0
P1
P2
P3
```

No numeric priority/SEO/GEO score is stored.

## 28. RuleEvidenceRef

Every result must trace to exact evidence:

```text
rule_evidence_ref_id
rule_result_id
evidence_type
evidence_ref
field
observed_value
role
```

Roles:

```text
PRIMARY
SUPPORTING
CONTEXT
```

Evidence types may include FetchObservation, RobotsDecision, HtmlObservation, RobotsDirective, CanonicalObservation, LinkObservation, Sitemap entities, StructuredDataBlock, RenderComparison, ProfileComparison, ProbeObservation, ProjectPolicy and NormalizedObservation.

A deterministic result must be reproducible from rule/version, snapshot, policy refs and evidence refs. Hidden runtime or LLM context is forbidden.

## 29. ManualReviewTask and optional resolution

```text
manual_review_task_id
audit_run_id
rule_result_id
review_question
evidence_refs[]
review_status
created_at
```

Statuses:

```text
OPEN
RESOLVED
```

Optional human resolution is stored separately and does not rewrite the original machine RuleResult.

## 30. Finding and FindingMember

Finding is report-level aggregation:

```text
finding_id
audit_run_id
rule_id
parent_check
status
severity
presentation_classification
affected_count
summary
expected_state
recommended_action
created_at
```

Presentation classification:

```text
ISSUE
WARNING
MANUAL_REVIEW
INFORMATION
```

This does not change underlying rule status.

Membership:

```text
finding_member_id
finding_id
rule_result_id
```

Safe V1 aggregation is primarily by rule/status/severity. Do not automatically infer root cause/template/CMS ownership.

Recommendations originate from rule/domain knowledge rather than aggregator invention.

## 31. Key relationships

```text
AuditRun 1 → N UrlResource
AuditRun 1 → N ProjectPolicyAssignment
AuditRun 1 → 1 EvidenceSnapshot (V1)
AuditRun 1 → N RuleResult
AuditRun 1 → N Finding

UrlResource 1 → N DiscoveryRecord
UrlResource 1 → N FetchObservation
UrlResource 1 → N HtmlObservation
UrlResource 1 → N RobotsDecision
UrlResource 1 → N StructuredDataBlock
UrlResource 1 → N RenderObservation

SitemapObservation 1 → N SitemapEntry
RuleDefinition 1 → N RuleResult
RuleResult 1 → N RuleEvidenceRef
Finding N ↔ N RuleResult through FindingMember
```

## 32. Logical normalized URL snapshot

Rule execution should be able to project a consolidated view resembling:

```text
UrlSnapshot
├── identity
├── discovery_sources[]
├── crawl_depth / inlinks
├── default fetch/final state
├── redirect evidence
├── robots decisions
├── HTML metadata
├── page directives / effective_noindex
├── canonical evidence / target state
├── internal links[]
├── structured blocks[]
├── render state/comparisons[]
└── explicit policy refs[]
```

It may later be implemented as an object, query projection, in-memory model or database view.

## 33. Rule-family data dependencies

`AR-ACC-*` primarily consumes UrlResource, FetchObservation, RobotsDecision, ProfileComparison, RenderResource and ProjectPolicy.

`AR-INDEX-*` consumes RobotsDirective, ProjectPolicy, Probe, Fetch and Html observations.

`AR-CANON-*` consumes CanonicalObservation, FetchObservation, RedirectHop, LinkObservation, pagination evidence and ProjectPolicy.

`AR-LINK-*` consumes LinkObservation, FetchObservation, DiscoveryRecord, graph metrics and ProjectPolicy.

`AR-DISC-*` consumes Sitemap entities, FetchObservation, RobotsDirective, CanonicalObservation and ProjectPolicy.

`AR-ENTITY-*` consumes StructuredDataBlock.

`AR-RENDER-*` consumes HtmlObservation, RenderObservation and RenderFieldComparison.

`AR-AI-*` consumes RobotsDecision, ProfileComparison and ProjectPolicy. No AI ranking dataset exists.

## 34. Normalization ownership

Collectors own raw observations; normalizers own normalized/derived fields; Rule Engine evaluates those fields.

Examples:

```text
HTTP Fetcher → status=301
Redirect normalizer → hop_count=2, loop=false
HTML parser → canonical_raw="../b"
URL normalizer → canonical_resolved_url=https://example.com/b
Rule Engine → evaluates frozen rule condition
```

The Rule Engine must not duplicate parsing/normalization logic.

## 35. Null, UNKNOWN and NOT_APPLICABLE

Storage absence and rule statuses are different.

If canonical target fetch was required but timed out:

```text
canonical_target_status = null
fetch_error = timeout
→ rule may return UNKNOWN
```

If no canonical exists and a target-status rule only applies when canonical exists:

```text
→ NOT_APPLICABLE
```

`null` does not automatically mean `UNKNOWN`.

Missing policy must remain absent/UNSPECIFIED, not false.

## 36. Timestamps and snapshot consistency

Evidence should preserve relevant timestamps including run start, fetch/parse/render time, snapshot freeze and rule evaluation.

One RuleResult cannot mix evidence from different snapshots.

## 37. Fingerprints

Logical fingerprint fields may include:

```text
content_fingerprint
main_text_fingerprint
response_fingerprint
```

Used for known-404 comparison, slash duplication evidence and request-profile comparison. Algorithm is deferred but must be deterministic/reproducible for the rule use case.

## 38. Evidence completeness

Snapshot metadata should be able to express:

```text
crawl_complete
sitemap_discovery_complete
render_selection_complete
probe_collection_complete
```

Exact meaning of `crawl_complete` depends on later crawl-limit decisions.

## 39. Data that must not exist as inferred V1 facts

Do not automatically create:

```text
seo_importance
business_value
search_intent
page_quality
content_quality_score
geo_score
ai_rank_score
citation_probability
google_indexed
googlebot_verified
oai_searchbot_verified
template_root_cause
```

unless later knowledge explicitly introduces evidence and semantics.

## 40. Unsupported V1 runtime dependencies

The data model must not require GSC index state, Bing dashboards, server/CDN logs, GA4/GTM, CMS revision history, paid SEO APIs or AI citation tracking.

## 41. Logical retention categories

Lightweight structured data:

```text
IDs
statuses
URLs
normalized fields
rules/results/relationships
```

Potentially large evidence:

```text
raw HTML
rendered DOM
sitemap XML
robots.txt
response bodies
```

This distinction may influence storage later but does not require separate infrastructure in V1.

## 42. Suggested aggregate boundaries

```text
Audit Aggregate:
AuditRun + ProjectPolicyAssignment + EvidenceSnapshot

URL Evidence Aggregate:
UrlResource + discovery/fetch/html/robots/canonical/link/render evidence

Sitemap Aggregate:
SitemapObservation + SitemapEntry

Evaluation Aggregate:
RuleDefinition + RuleResult + RuleEvidenceRef + ManualReviewTask

Reporting Aggregate:
Finding + FindingMember
```

These are logical boundaries, not required transaction boundaries.

## 43. Rule evaluation input contract

```text
RuleEvaluationContext
├── audit_run_id
├── snapshot_id
├── RuleDefinition
├── subject
├── normalized_inputs
└── matching_project_policy
```

Output:

```text
RuleResult + RuleEvidenceRef[]
```

Crawler/browser objects do not belong in RuleEvaluationContext.

## 44. Representative examples

### Canonical target redirect

```text
URL A canonical → URL B
URL B status = 301
→ AR-CANON-006 = FAIL
```

Evidence refs point to CanonicalObservation A and FetchObservation B.

### Explicit noindex intent

```text
effective_noindex = true
expected_indexable = true
→ AR-INDEX-001 = FAIL
```

No intent is inferred when policy is absent.

### GPTBot intentionally blocked

```text
robots_allowed_gptbot = false
gptbot_training_policy = block
→ AR-AI-003 = PASS
```

No `chatgpt_visibility=false` field is created.

### OAI request-profile anomaly

```text
DEFAULT = 200
OAI_SEARCHBOT_PROFILE = 403
→ ProfileComparisonObservation
→ AR-AI-002 may WARNING
```

This is not verified OAI crawler blocking.

### Render failure

```text
render_requested=true
render_status=FAILED
→ dependent evidence unavailable
→ UNKNOWN where defined
```

No `google_render_failed=true` field exists.

### Known-missing probe

Probe URL receives UrlResource + ProbeObservation + FetchObservation but no DiscoveryRecord; it cannot affect site URL count, orphan state or graph metrics.

### Sitemap URL

SitemapEntry may legitimately create DiscoveryRecord(type=SITEMAP) because sitemap is a real discovery source.

## 45. Data model invariants

1. One UrlResource represents URL identity, not one request.
2. One URL may have many FetchObservations.
3. Only DiscoveryRecord grants discovery membership.
4. Audit probes never automatically create DiscoveryRecords.
5. Project policies are explicit and never crawler-inferred.
6. Raw evidence remains separate from normalized observations.
7. Derived observations retain source evidence references.
8. Rule evaluation uses only a frozen EvidenceSnapshot.
9. One RuleResult cannot combine multiple snapshots.
10. Every deterministic FAIL has sufficient evidence refs for reproduction.
11. Rule status is exactly one of six frozen statuses.
12. INFORMATION is report presentation only.
13. Request profile is not verified crawler identity.
14. GPTBot and OAI-SearchBot policies remain independent.
15. RuleResults are atomic; Findings are aggregations.
16. Aggregation cannot rewrite RuleResult status.
17. Human manual resolution remains separate from machine evaluation.
18. No numeric SEO/GEO/AI citation score exists.
19. No external V1 integration is required by the data contract.
20. Historical results retain knowledge/rule version identity.

## 46. Minimum V1 data model

At minimum implementation must support:

```text
AuditRun
ProjectPolicyAssignment
UrlResource
DiscoveryRecord
FetchObservation
RedirectHop
RobotsDecision
HtmlObservation
RobotsDirectiveObservation
CanonicalObservation
LinkObservation
SitemapObservation
SitemapEntry
StructuredDataBlock
RenderObservation
RenderFieldComparison
RenderResourceObservation
ProbeObservation
ProfileComparisonObservation
EvidenceSnapshot
NormalizedObservation
RuleDefinition
RuleResult
RuleEvidenceRef
ManualReviewTask
Finding
FindingMember
```

Removing a concept requires proving all 47 rules remain representable and reproducible.

## 47. Decisions deferred to implementation design

```text
relational vs document database
schema/table names
JSON vs normalized columns
UUID vs ULID
artifact persistence/compression
indexes/foreign keys/cascade behavior
crawl frontier persistence
batch-write strategy
transaction boundaries
cache layer
retention duration
snapshot serialization
rule-registry file format
```

## 48. Data model freeze gate

V1 data model is frozen when accepted that:

1. AuditRun is the execution boundary.
2. ProjectPolicyAssignment is explicit and scoped.
3. UrlResource, DiscoveryRecord and FetchObservation are separate.
4. Probe URLs cannot pollute the site graph.
5. Raw and normalized evidence remain separate.
6. Rules evaluate a frozen EvidenceSnapshot.
7. RuleResult is atomic; Finding is aggregation.
8. Deterministic FAILs have evidence refs.
9. Manual resolution does not rewrite machine history.
10. Request profiles are not verified bot identities.
11. GPTBot and OAI-SearchBot remain separate.
12. No external API/log/analytics runtime dependency is required.
13. No numeric SEO/GEO score exists.
14. Storage technology remains undecided.

## 49. Final logical boundary

```text
WHAT THE SITE DID
≠ WHAT THE PROJECT EXPECTED
≠ WHAT THE RULE CONCLUDED
≠ HOW THE REPORT PRESENTED IT
```

Keeping these concepts independent preserves reproducibility and reduces false positives.
