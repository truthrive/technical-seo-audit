# Audit Framework

## Purpose

Provide the shared mental model used to organize technical checks and feature development.

## Search lifecycle

The audit lifecycle is:

`Discover → Crawl → Render → Index → Consolidate → Retrieve → Cite → Measure`

A page may fail at any layer.

Later-layer optimization has limited value when an earlier layer is broken.

Example:

If a page cannot be crawled, citation-readiness improvements are not the immediate technical priority.

## Layer 1 — Discover

**Question:** Can crawlers discover important URLs through reliable paths?

Covers:

- internal links;
- crawlable anchors;
- sitemap;
- pagination;
- orphan URLs;
- feeds;
- IndexNow where applicable.

Primary rule prefixes:

- `LINK-*`
- `DISC-*`

## Layer 2 — Crawl

**Question:** Can an approved crawler reach and fetch a discovered resource reliably?

`Access` is a precondition and cross-cutting concern of Crawl, not a separate lifecycle stage.

Covers:

- DNS and HTTPS;
- robots.txt;
- authentication;
- CDN / WAF / bot protection;
- HTTP status;
- timeouts;
- server errors;
- crawl traps.

Primary rule prefixes:

- `ACC-*`

## Layer 3 — Render

**Question:** Can the intended content and metadata be obtained after rendering?

Covers:

- JavaScript;
- hydration;
- API failures;
- lazy loading;
- raw HTML vs rendered DOM;
- client-side metadata;
- infinite scroll.

Primary rule prefixes:

- `RENDER-*`

## Layer 4 — Index

**Question:** Is the URL eligible to be indexed?

Covers:

- meta robots;
- X-Robots-Tag;
- `noindex`;
- soft 404;
- index pollution;
- Search Console index state;
- snippet controls.

Primary rule prefixes:

- `INDEX-*`

## Layer 5 — Consolidate

**Question:** Which URL should represent the content?

Covers:

- canonical;
- redirects;
- duplicate URLs;
- protocol / host normalization;
- parameter URLs;
- pagination strategy;
- hreflang / canonical interaction.

Primary rule prefixes:

- `CANON-*`
- `INTL-*`

## Layer 6 — Retrieve

**Question:** Can machines extract and understand the useful information on the page?

Covers:

- semantic HTML;
- headings;
- structured data;
- entity consistency;
- text availability;
- tables and lists;
- media discoverability.

Primary rule prefixes:

- `RETR-*`
- `ENTITY-*`
- `MEDIA-*`

## Layer 7 — Cite

**Question:** Can important claims be attributed and cited clearly?

Covers:

- claim-source relationships;
- author / entity visibility;
- dates;
- source links;
- extractable facts.

This layer contains emerging evidence and must not be presented as a confirmed universal AI ranking system.

Primary rule prefixes:

- selected `AI-*`
- selected `RETR-*`

## Layer 8 — Measure

**Question:** Can we observe crawler, indexing, performance, and visibility behavior?

Covers:

- Google Search Console;
- Bing Webmaster Tools;
- server / CDN logs;
- Core Web Vitals;
- bot verification;
- post-deploy validation;
- referral and citation observation.

Primary rule prefixes:

- `PERF-*`
- `OBS-*`

## Cross-cutting layer — Experience & Infrastructure

Some checks support several lifecycle stages rather than a single stage.

Examples:

- server reliability;
- response time;
- compression;
- cache behavior;
- mobile usability.

These are tracked mainly under:

- `PERF-*`

## Rule families

| Prefix | Domain |
|---|---|
| `ACC` | Access, robots, server accessibility |
| `INDEX` | Indexability and snippet controls |
| `CANON` | URL normalization, redirects, canonicalization |
| `LINK` | Internal linking and discovery |
| `DISC` | Sitemap and freshness |
| `RETR` | Machine-readable content and extractability |
| `ENTITY` | Structured data and entity consistency |
| `RENDER` | JavaScript and rendered output |
| `PERF` | Performance, infrastructure, experience |
| `MEDIA` | Images and video |
| `INTL` | International / hreflang |
| `AI` | AI Search / GEO readiness |
| `OBS` | Logs, measurement, post-deploy verification |

## Design rule

Do not create product modules merely because the framework has a layer.

The framework organizes reasoning.

The MVP scope is defined separately in `05-mvp-scope.md`.
