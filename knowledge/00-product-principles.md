# Product Principles

## Purpose

Define the non-negotiable product principles for the Technical Search & GEO Audit Tool.

## Product goal

Build a technical audit system that converts website evidence into clear, traceable, actionable findings.

The product should help an SEO practitioner or developer move from:

`website → evidence → rule evaluation → issue → affected scope → recommended fix → verification`

The product is **not** intended to reproduce every feature of a general-purpose crawler.

## Core model

### Crawler collects evidence

The crawler, renderer, parsers, APIs, and logs collect observations.

They should not invent conclusions.

### Rule Engine evaluates conditions

Deterministic code should decide deterministic checks whenever possible.

Examples:

- HTTP status;
- redirect chain;
- robots directives;
- canonical target state;
- sitemap validity;
- metadata presence;
- broken internal links.

### AI explains and assists

LLMs may help with:

- plain-language explanations;
- grouping related findings;
- root-cause hypotheses;
- summarization;
- manual-review assistance.

LLMs should not replace deterministic evaluation where code can reliably decide the result.

### Human review remains valid

Some questions require intent, business context, editorial judgment, or UX interpretation.

The product must support manual review instead of forcing a false automatic answer.

## Product principles

### 1. Evidence before conclusion

Every issue should point to observable evidence.

A generic SEO best practice is not enough to create a FAIL result.

### 2. Deterministic checks before AI

If a rule can be evaluated reliably using code, evaluate it using code.

Do not ask an LLM whether:

- a response is 404;
- a canonical target is 301;
- a meta robots tag contains `noindex`;
- a sitemap URL returns 500.

### 3. Avoid false positives

Supported outcomes must include:

- `PASS`
- `WARNING`
- `FAIL`
- `MANUAL_REVIEW`
- `NOT_APPLICABLE`
- `UNKNOWN`

An unusual implementation is not automatically an error.

### 4. Context matters

Severity and priority may depend on:

- page role;
- indexability intent;
- canonical state;
- template;
- directory;
- affected URL count;
- business importance;
- whether the issue is isolated or systemic.

### 5. Severity is not priority

Severity describes how serious the rule violation is.

Priority describes what should be fixed first in the current project.

A medium-severity template issue affecting 500,000 URLs can outrank a high-severity issue affecting one unimportant URL.

### 6. SEO and GEO are not interchangeable

A Technical SEO factor must not automatically be presented as an AI Search or GEO ranking factor.

Where evidence is weak, label it weak.

### 7. Do not manufacture certainty

Use explicit evidence labels:

- `CONFIRMED`
- `STRONG_EVIDENCE`
- `PLATFORM_SPECIFIC`
- `EMERGING`
- `EXPERIMENTAL`
- `UNSUPPORTED`

### 8. Explain in plain language

Issue output should be understandable by:

- SEO;
- developer;
- content;
- stakeholder.

Avoid unnecessary jargon.

### 9. Prefer root causes over raw issue counts

The product should eventually distinguish:

- 487 affected URLs;
- 1 underlying template defect.

The template defect is usually the more useful unit of action.

### 10. Preserve traceability

A rule should be traceable through:

`rule ID → source evidence → observed data → result → recommendation`

## Non-goals

The MVP should not claim to:

- predict Google rankings;
- predict ChatGPT citation probability;
- provide a universal GEO score;
- replace Search Console;
- replace a full enterprise crawler;
- determine content quality with certainty;
- infer business importance without input;
- treat all SEO conventions as hard errors.

## Knowledge-change policy

A new audit requirement should enter the knowledge base only when at least one is true:

1. primary platform documentation supports it;
2. technical behavior is reproducible;
3. it is an explicit product decision;
4. it is clearly labeled experimental.

Third-party claims alone should not silently become mandatory rules.
