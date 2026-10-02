# AI Search & GEO Knowledge

## Purpose

Define the evidence boundaries for AI Search / GEO checks so the product remains useful without turning speculative industry claims into hard technical requirements.

## Core principle

AI Search optimization must be based on:

- documented platform behavior;
- reproducible technical observations;
- clearly labeled evidence.

Do not convert correlation, marketing language, or isolated experiments into mandatory audit rules.

## Evidence levels

### CONFIRMED

Directly documented by a platform or supported by repeatable technical behavior.

### STRONG_EVIDENCE

Consistent technical rationale exists, but the platform has not defined it as a direct ranking or citation factor.

### PLATFORM_SPECIFIC

Documented for one platform and must not be generalized to all AI systems.

### EMERGING

Plausible and supported by early research or observed behavior, but not sufficiently validated for deterministic scoring.

### EXPERIMENTAL

May be measured or tested, but should not affect critical audit severity.

### UNSUPPORTED

Do not implement as a recommended requirement.

## OpenAI crawlers

### OAI-SearchBot

**Evidence:** `PLATFORM_SPECIFIC / CONFIRMED`

OpenAI documents OAI-SearchBot as the crawler used for search.

A site may allow OAI-SearchBot while independently disallowing GPTBot.

Audit may check:

- robots.txt access;
- HTTP access;
- WAF/CDN blocking;
- verified server-log activity;
- unintended 403 / 429 responses.

Important product implication:

Blocking OAI-SearchBot can affect whether site content is surfaced in ChatGPT search results.

This does **not** mean crawler access guarantees citation or ranking.

Source:

- `SRC-OPENAI-BOTS-001`

### GPTBot

**Evidence:** `PLATFORM_SPECIFIC / CONFIRMED`

OpenAI documents GPTBot controls separately from OAI-SearchBot.

Product rule:

Do not report:

`GPTBot blocked → ChatGPT Search blocked`

These are separate controls.

A business may intentionally disallow GPTBot while allowing OAI-SearchBot.

## Search bot verification

Do not trust a user-agent string alone for high-confidence bot identification.

Where platform documentation provides IP ranges or verification guidance, use them.

Possible statuses:

- verified bot;
- unverified claimed bot;
- blocked verified bot;
- unknown.

## Snippet controls

**Evidence:** `CONFIRMED` for search-engine directives, platform implications may vary.

Relevant controls include:

- `nosnippet`;
- `max-snippet`;
- `data-nosnippet`.

The audit may warn when content intended for discovery or quotation is intentionally or accidentally restricted.

Do not claim that changing snippet controls guarantees AI citation.

## Machine extractability

**Evidence:** `STRONG_EVIDENCE`

Useful technical conditions include:

- critical information exists as text;
- content is present in rendered DOM;
- headings and sections are structurally clear;
- tables and lists use meaningful markup;
- important facts are not available only inside images/canvas;
- entity naming is consistent.

Treat this as retrieval readiness, not a confirmed universal AI ranking factor.

## Structured data and entity consistency

**Evidence:** `STRONG_EVIDENCE / CONTEXT-DEPENDENT`

Structured data can improve machine interpretation and eligibility for supported search experiences.

The product may evaluate:

- syntax;
- content/schema consistency;
- entity identifiers;
- page-type appropriateness.

Do not claim:

`schema added → AI citation improved`

without specific evidence.

## Citation readiness

**Evidence:** `EMERGING`

A useful review pattern is:

`claim → evidence → source → date → author/entity`

Potential manual-review checks:

- important claims have identifiable sources;
- citations are placed near the claim they support;
- author / organization identity is clear;
- publication/update dates are visible where relevant.

Do not assign P0/P1 solely because citation-readiness is weak.

## Freshness

Freshness infrastructure may help discovery and recrawling:

- accurate sitemap `lastmod`;
- internal discovery;
- IndexNow for supported ecosystems;
- feed updates where appropriate.

Freshness should not be presented as a universal AI citation factor.

## llms.txt

**Evidence:** `EXPERIMENTAL`

Product policy:

- missing `llms.txt` is not an error;
- it must not receive critical severity;
- it must not be required for MVP;
- it may later appear as an informational experiment.

## "Special GEO schema"

**Evidence:** `UNSUPPORTED` as a universal requirement.

Product policy:

Do not recommend invented schema types or unsupported properties as mandatory GEO markup.

Use standards-based structured data and consistent visible content.

## Core Web Vitals and AI Search

Core Web Vitals are valid performance metrics.

Direct evidence that perfect CWV causes AI citations is weak.

Product policy:

- audit CWV for web performance / Search experience;
- do not present CWV as a direct GEO ranking lever.

## AI visibility measurement

Useful observability may include:

- verified AI crawler activity;
- referral traffic from AI/search products;
- sampled citations;
- repeated prompt-set observations.

These are monitoring signals.

They are not direct platform ranking data.

## Forbidden product claims

The tool must not claim:

- a guaranteed ChatGPT ranking score;
- a universal GEO score;
- a numeric probability of AI citation without a validated model;
- a required "special GEO schema";
- that allowing an AI crawler guarantees visibility;
- that blocking GPTBot necessarily blocks ChatGPT Search;
- that perfect CWV guarantees AI citations;
- that `llms.txt` is mandatory for AI visibility.

## Recommended AI Search module naming

Prefer:

- `AI Search Accessibility`
- `Retrieval Readiness`
- `Citation Readiness`
- `AI Search Observability`

Avoid overly certain labels such as:

- `AI Ranking Score`
- `Guaranteed GEO Score`

until such a model is independently validated.
