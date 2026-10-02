# Technical Search & GEO Audit Knowledge

## Purpose

This directory is the authoritative domain-knowledge layer for the planned **Technical Search & GEO Audit Tool**.

It is designed for ChatGPT Project, Codex, and later integration with the user's AI Dev System.

The knowledge base separates:

- domain facts;
- product decisions;
- implementation rules;
- emerging or experimental ideas.

Do not invent SEO, crawling, indexing, rendering, or AI Search requirements outside this knowledge base.

**Knowledge snapshot:** 2026-10-01

**Package revision:** v1.4 — SiteCrawl-based implementation plan

## Documents

| Document | Purpose |
|---|---|
| `00-product-principles.md` | Product boundaries, non-goals, and reasoning principles |
| `01-audit-framework.md` | Search lifecycle and audit layers |
| `02-audit-check-catalog.md` | Authoritative catalog of 100 audit checks with stable rule IDs |
| `03-rule-engine-spec.md` | PASS / WARNING / FAIL evaluation model and normalized rule contract |
| `04-ai-search-geo.md` | AI Search / GEO evidence levels, platform behavior, and anti-hype guardrails |
| `05-mvp-scope.md` | Initial product scope and first 40 candidate automated rules |
| `06-research-sources.md` | Primary sources, internal legacy inputs, and evidence registry |
| `07-v1-atomic-rule-manifest.md` | Frozen V1 executable rule decomposition, exact semantics, data contract, counts, and deferrals |
| `08-system-architecture.md` | Frozen V1 system boundaries, pipeline, module responsibilities, and architecture invariants |
| `09-data-model.md` | Frozen logical data entities, relationships, evidence ownership, snapshots, and result traceability |
| `10-implementation-plan.md` | Frozen SiteCrawl-based migration plan, phased implementation sequence, test gates, scope controls, and release criteria |

## Routing

### Always read before product or architecture decisions

- `00-product-principles.md`
- `01-audit-framework.md`

### Working on crawler, robots, discovery, or HTTP

Read:

- relevant `ACC-*` and `DISC-*` entries in `02-audit-check-catalog.md`
- `03-rule-engine-spec.md`

### Working on indexability or canonicalization

Read:

- relevant `INDEX-*` and `CANON-*` entries
- `03-rule-engine-spec.md`
- corresponding sources in `06-research-sources.md`

### Working on JavaScript rendering

Read:

- `RENDER-*` entries
- `01-audit-framework.md`
- `06-research-sources.md`

### Working on AI Search / GEO

Read:

- `04-ai-search-geo.md`
- relevant `AI-*` entries
- `06-research-sources.md`

### Working on MVP scope or V1 rule semantics

Read:

- `00-product-principles.md`
- `05-mvp-scope.md`
- `07-v1-atomic-rule-manifest.md`

### Working on system architecture

Read:

- `00-product-principles.md`
- `01-audit-framework.md`
- `07-v1-atomic-rule-manifest.md`
- `08-system-architecture.md`

### Working on normalized data, persistence contracts, or evidence traceability

Read:

- `03-rule-engine-spec.md`
- `07-v1-atomic-rule-manifest.md`
- `08-system-architecture.md`
- `09-data-model.md`

### Working on implementation sequencing or coding phases

Read:

- `07-v1-atomic-rule-manifest.md`
- `08-system-architecture.md`
- `09-data-model.md`
- `10-implementation-plan.md`

### Working on SiteCrawl integration or migration

Read:

- `08-system-architecture.md`
- `09-data-model.md`
- `10-implementation-plan.md`

Treat the existing SiteCrawl codebase as the V1 evidence-acquisition foundation. Preserve proven crawler behavior where compatible, introduce the new audit domain through adapters and explicit contracts, and do not treat the legacy issue engine as authority for V1 audit verdicts.

## Authority rules

1. Current source code and runtime behavior become authoritative for **implemented behavior** once the tool exists.
2. Broad domain audit definitions are owned by `02-audit-check-catalog.md`.
3. Generic rule execution/status semantics are owned by `03-rule-engine-spec.md`.
4. Frozen V1 atomic decomposition and per-rule execution semantics are owned by `07-v1-atomic-rule-manifest.md`; each executable rule references its broad catalog source through `parent_check`.
5. Frozen V1 architecture boundaries are owned by `08-system-architecture.md`.
6. Frozen V1 logical data contracts are owned by `09-data-model.md`.
7. Frozen V1 implementation sequencing and phase gates are owned by `10-implementation-plan.md`.
8. AI Search / GEO claims must follow evidence levels in `04-ai-search-geo.md`.
9. Primary platform documentation has more authority than third-party SEO commentary.
10. Experimental ideas must never silently become mandatory audit requirements.
11. If implementation and knowledge conflict, surface the conflict instead of guessing.
12. Do not change severity, status semantics, atomic rule behavior, or product claims without updating the relevant knowledge document.

## AI Dev System integration

When this knowledge is copied into the future local repository:

- keep it project-owned under `/knowledge`;
- keep `.ai-dev-system/` responsible for **how agents work**;
- keep `/knowledge` responsible for **what the audit domain means**;
- point project instructions or `PROJECT_CONTEXT.md` to this `INDEX.md`;
- do not copy SEO/GEO domain policy into the reusable AI Dev System core.

## Language convention

Control documents are English-first for coding agents.

The audit catalog preserves Vietnamese wording from the approved operational checklist so the same content can later support:

- Vietnamese audit reports;
- UI copy;
- issue explanations;
- implementation guidance.
