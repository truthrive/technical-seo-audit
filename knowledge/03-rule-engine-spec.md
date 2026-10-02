# Rule Engine Specification

## Purpose

Define how normalized technical observations become consistent audit results.

This document describes the evaluation contract.

It does not prescribe the final programming language or storage technology.

## Status model

Every evaluated rule returns one of:

- `PASS`
- `WARNING`
- `FAIL`
- `MANUAL_REVIEW`
- `NOT_APPLICABLE`
- `UNKNOWN`

## Semantics

### PASS

Observed state satisfies the rule.

### WARNING

A potential issue exists, but available context is insufficient for a deterministic failure.

Use WARNING when:

- the pattern is suspicious but not inherently wrong;
- impact depends on page role;
- thresholds are advisory rather than absolute;
- incomplete contextual evidence exists.

### FAIL

Observed state clearly violates a defined technical requirement.

FAIL should be reproducible from stored evidence.

### MANUAL_REVIEW

The tool can collect useful evidence but should not make the final judgment.

Examples:

- source credibility;
- information architecture quality;
- claim-source relationships;
- whether a UX implementation is genuinely disruptive.

### NOT_APPLICABLE

The rule does not apply to the target.

Examples:

- hreflang on a single-language site;
- video sitemap checks on a site with no indexable video assets.

### UNKNOWN

The rule would apply, but required technical data could not be obtained.

Examples:

- canonical target could not be fetched;
- renderer failed;
- external API was unavailable;
- access logs were not supplied.

## Evaluation pipeline

`Raw observations → Normalization → Preconditions → Context → Rule evaluation → Exceptions → Status → Evidence → Aggregation`

## Rule contract

Every executable rule should eventually expose a machine-readable definition similar to:

```yaml
rule_id: AR-CANON-006
parent_check: CANON-003
rule_version: 1
name: Canonical target returns final 200
category: canonical
lifecycle: consolidate
automation: deterministic
default_severity: P1

required_inputs:
  - source_url
  - canonical_url
  - canonical_status

preconditions:
  - canonical_url != null
  - canonical_status is available

evaluate:
  pass: canonical_status == 200
  fail: canonical_status >= 300
  unknown: canonical target fetch did not produce usable evidence

evidence_fields:
  - source_url
  - canonical_url
  - canonical_status

source_refs:
  - SRC-GOOGLE-CANONICAL-001
```

Markdown remains the human/agent knowledge layer.

A later YAML/JSON rule registry should become the executable configuration layer.

## Required result object

A rule result should support at least:

```json
{
  "rule_id": "AR-CANON-006",
  "parent_check": "CANON-003",
  "rule_version": 1,
  "status": "FAIL",
  "severity": "P1",
  "scope": "template",
  "summary": "Canonical target returns a redirect.",
  "affected_count": 487,
  "evidence": [
    {
      "url": "https://example.com/a",
      "observed": "canonical -> /b -> 301",
      "expected": "canonical target returns 200"
    }
  ],
  "recommended_fix": "Point the canonical directly to the final 200 URL."
}
```

## Severity model

### P0 — Critical

Use only for conditions that can materially block site availability, crawling, or indexing at a large or strategically important scope.

Examples:

- production site inaccessible;
- sitewide accidental noindex;
- widespread Googlebot blocking;
- critical server failures.

### P1 — High

Strong technical impact or substantial consolidation/discovery risk.

Examples:

- canonical implementation broken at template scale;
- widespread broken internal links;
- redirect loops;
- sitemap dominated by non-indexable URLs.

### P2 — Medium

Optimization issue, partial impact, or problem requiring context.

### P3 — Low / Optional

Best practice, informational check, or experimental item.

## Severity modifiers

Default severity may be modified by:

- affected URL count;
- percentage of indexable URLs affected;
- template scope;
- page business importance;
- traffic;
- migration status;
- whether issue blocks an earlier lifecycle layer.

## Priority model

Do not equate severity with priority.

A future priority model may consider:

```text
priority =
  severity weight
  × scope weight
  × affected URL weight
  × business importance
  × confidence
```

Do not implement a numeric score until weights are explicitly approved and tested.

## Scope model

Supported scopes:

- `URL`
- `TEMPLATE`
- `DIRECTORY`
- `SITEWIDE`
- `SITE_CONFIGURATION`

The engine should aggregate URL-level evidence into likely shared root causes where possible.

## Rule-design requirements

### One rule, one primary condition

Avoid broad rules such as:

`CANONICAL_PROBLEMS`

Prefer:

- missing canonical;
- multiple canonical;
- canonical target non-200;
- canonical target noindex;
- canonical conflict with sitemap.

### Store the evidence used to decide

Do not return only:

`FAIL`

Return enough evidence to reproduce the decision.

### Do not infer intent silently

Example:

`noindex` is not inherently an error.

A FAIL requires evidence that the page is intended to be indexable or a project-level policy says so.

### Separate acquisition failure from audit failure

Renderer failure should not automatically mean:

`page rendering FAIL`

It may mean:

`UNKNOWN — renderer could not obtain evidence`.

## Automation classes

Executable rules use these automation classes:

### deterministic

The final technical condition can be decided from deterministic normalized fields. A deterministic rule may still return `WARNING` when the atomic condition itself is advisory rather than a hard failure.

### assisted

The tool collects deterministic evidence, but page intent, explicit project policy, business context, or reviewer interpretation may be required. An assisted rule may return `FAIL` only when its explicit contextual preconditions are supplied and the failure remains reproducible from stored evidence.

### manual

The tool creates a guided review task and evidence checklist rather than fabricating a machine verdict.

Catalog labels map conceptually as `Full → deterministic`, `Partial → assisted`, and `Manual → manual`, but the catalog label is not a V1 execution contract. The frozen V1 automation class is defined by `07-v1-atomic-rule-manifest.md`.

## Versioning

Catalog check IDs and executable rule IDs are separate stable identifiers.

Executable rules use stable `AR-*` IDs and retain the broad source definition through `parent_check`.

Do not derive executable IDs by suffixing a catalog ID such as `CANON-003a`.

When executable rule behavior materially changes:

- keep the same executable rule ID and `parent_check`;
- increment `rule_version`;
- document the reason;
- preserve historical result compatibility where practical.

## Testing contract

Each deterministic rule should eventually have fixtures covering:

- every result state that the atomic rule actually supports;
- `FAIL` only when the atomic condition defines a valid hard-failure state;
- `WARNING` when the deterministic condition is advisory;
- missing input / `UNKNOWN`;
- `NOT_APPLICABLE` where relevant;
- malformed input;
- edge cases and exceptions.

Each assisted rule should test:

- deterministic evidence states;
- explicit contextual/policy preconditions;
- any valid policy-driven `FAIL`;
- escalation to `WARNING`, `MANUAL_REVIEW`, `UNKNOWN`, or `NOT_APPLICABLE` when required context/evidence is absent.

## Knowledge-to-code workflow

Before implementing a rule:

1. read the catalog entry;
2. read referenced primary sources;
3. define exact normalized inputs;
4. define explicit preconditions;
5. define machine conditions;
6. define exceptions;
7. create fixtures;
8. implement;
9. verify result evidence;
10. compare implementation back to the knowledge entry.

Do not silently reinterpret the domain rule during coding.
