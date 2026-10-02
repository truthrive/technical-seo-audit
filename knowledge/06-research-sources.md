# Research Sources

## Purpose

Provide a traceable source registry for rule definitions and product claims.

**Last primary-source verification:** 2026-10-01

## Source policy

Authority order:

1. primary platform / standards documentation;
2. reproducible technical evidence;
3. internal historical audit knowledge;
4. third-party research or commentary.

Third-party claims should not become mandatory rules without independent support.

## SRC-INTERNAL-CHECKLIST-001

**Provider:** Internal legacy audit knowledge  
**Topic:** Historical Technical SEO audit standards and operational checklist  
**Authority:** Internal / Project input  
**URL / location:** Conversation-uploaded Excel files  

### Notes

Derived from the two legacy audit workbooks plus the normalized 2026 MVP checklist. Internal knowledge is not evidence of a search-engine ranking factor by itself.

### Referenced by rules

- `ACC-001`, `ACC-002`, `ACC-004`, `ACC-005`, `ACC-006`, `ACC-007`, `ACC-009`, `INDEX-002`, `INDEX-003`, `INDEX-004`, `INDEX-005`, `INDEX-006`
- `CANON-002`, `CANON-007`, `CANON-008`, `CANON-009`, `LINK-002`, `LINK-003`, `LINK-004`, `LINK-005`, `LINK-006`, `LINK-007`, `DISC-006`, `DISC-007`
- `RETR-002`, `RETR-003`, `RETR-004`, `RETR-005`, `RETR-006`, `RETR-007`, `RETR-008`, `RETR-009`, `ENTITY-004`, `RENDER-003`, `RENDER-006`, `PERF-002`
- `PERF-003`, `PERF-004`, `PERF-005`, `PERF-007`, `PERF-008`, `MEDIA-001`, `MEDIA-002`, `MEDIA-003`, `MEDIA-005`, `AI-005`, `AI-007`, `AI-008`
- `AI-010`, `OBS-001`, `OBS-003`, `OBS-004`, `OBS-005`, `OBS-006`, `OBS-007`, `OBS-008`, `OBS-009`, `OBS-010`, `OBS-011`

## SRC-GOOGLE-ROBOTS-001

**Provider:** Google Search Central  
**Topic:** Robots meta tag, X-Robots-Tag, snippet controls  
**Authority:** Primary platform documentation  
**URL / location:** https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag  

### Notes

Use for noindex/nosnippet/max-snippet/data-nosnippet behavior.

### Referenced by rules

- `ACC-003`, `INDEX-001`, `INDEX-007`, `DISC-004`, `RENDER-005`, `AI-003`

## SRC-GOOGLE-CANONICAL-001

**Provider:** Google Search Central  
**Topic:** Canonicalization and duplicate URL consolidation  
**Authority:** Primary platform documentation  
**URL / location:** https://developers.google.com/search/docs/crawling-indexing/canonicalization  

### Notes

Use for canonical signals, duplicate consolidation, and canonical consistency.

### Referenced by rules

- `ACC-008`, `CANON-001`, `CANON-003`, `CANON-004`, `CANON-005`, `CANON-006`, `DISC-002`, `RENDER-005`, `INTL-004`

## SRC-GOOGLE-JS-001

**Provider:** Google Search Central  
**Topic:** JavaScript SEO and rendering  
**Authority:** Primary platform documentation  
**URL / location:** https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics  

### Notes

Google's March 2026 documentation update explicitly removed older wording that implied JavaScript content is inherently harder for Google simply because it uses JavaScript. Audit the rendered result, not the presence of JavaScript itself.

### Referenced by rules

- `LINK-001`, `RETR-001`, `RENDER-001`, `RENDER-002`, `RENDER-004`, `RENDER-005`, `RENDER-007`, `PERF-006`, `MEDIA-004`, `AI-004`

## SRC-GOOGLE-SITEMAP-001

**Provider:** Google Search Central  
**Topic:** Sitemaps and discovery  
**Authority:** Primary platform documentation  
**URL / location:** https://developers.google.com/search/docs/crawling-indexing/sitemaps/overview  

### Notes

Use for sitemap structure, discovery, and sitemap quality checks.

### Referenced by rules

- `DISC-001`, `DISC-002`, `DISC-003`, `DISC-004`, `DISC-005`, `MEDIA-006`

## SRC-GOOGLE-STRUCTURED-001

**Provider:** Google Search Central  
**Topic:** Structured data  
**Authority:** Primary platform documentation  
**URL / location:** https://developers.google.com/search/docs/appearance/structured-data/intro-structured-data  

### Notes

Structured data can help Google understand page content and enable supported search features. Do not convert this into a universal GEO guarantee.

### Referenced by rules

- `ENTITY-001`, `ENTITY-002`, `ENTITY-003`, `ENTITY-005`, `ENTITY-006`, `AI-006`, `AI-009`

## SRC-SCHEMA-VALIDATOR-001

**Provider:** Schema.org  
**Topic:** Schema.org markup validation  
**Authority:** Primary standards ecosystem  
**URL / location:** https://validator.schema.org/  

### Notes

Validator extracts JSON-LD, RDFa, and Microdata and reports syntax/graph issues.

### Referenced by rules

- `ENTITY-001`, `ENTITY-002`, `ENTITY-003`, `ENTITY-005`, `ENTITY-006`, `AI-006`, `AI-009`

## SRC-WEBDEV-CWV-001

**Provider:** web.dev / Chrome  
**Topic:** Core Web Vitals  
**Authority:** Primary web performance guidance  
**URL / location:** https://web.dev/articles/vitals  

### Notes

Use CWV for performance and experience evaluation. Do not present good CWV as proof of AI citation visibility.

### Referenced by rules

- `PERF-001`, `MEDIA-004`

## SRC-BING-INDEXNOW-001

**Provider:** Bing Webmaster Tools / IndexNow  
**Topic:** URL submission and IndexNow  
**Authority:** Primary platform documentation  
**URL / location:** https://www.bing.com/webmasters/help/url-submission-62f2860b  

### Notes

Bing recommends IndexNow for automated notification of added, updated, or deleted URLs. Submission does not guarantee indexing.

### Referenced by rules

- `OBS-002`

## SRC-OPENAI-BOTS-001

**Provider:** OpenAI  
**Topic:** OAI-SearchBot and GPTBot  
**Authority:** Primary platform documentation  
**URL / location:** https://developers.openai.com/api/docs/bots  

### Notes

OAI-SearchBot is documented for search; GPTBot controls model-training use. The controls are independent.

### Referenced by rules

- `AI-001`, `AI-002`

## SRC-GOOGLE-I18N-001

**Provider:** Google Search Central  
**Topic:** Localized versions and hreflang  
**Authority:** Primary platform documentation  
**URL / location:** https://developers.google.com/search/docs/specialty/international/localized-versions  

### Notes

Use for hreflang clusters, reciprocal annotations, and localized URL handling.

### Referenced by rules

- `INTL-001`, `INTL-002`, `INTL-003`, `INTL-004`

## SRC-GOOGLE-UPDATES-001

**Provider:** Google Search Central  
**Topic:** Documentation updates  
**Authority:** Primary platform change log  
**URL / location:** https://developers.google.com/search/updates  

### Notes

Useful to identify changed or retired technical guidance; checked for 2026 JavaScript documentation changes.

## Legacy inputs

The knowledge package also derives from two user-provided historical workbooks:

- `Audit technical _ Dự án A`
- `Sharing [Chưa hoàn thiện] Checklist tiêu chuẩn & Quy trình Audit Technical`

These inputs are valuable operational knowledge.

They must not override current primary platform documentation when a conflict exists.

## Evidence interpretation

A source can confirm technical behavior without confirming ranking impact.

Examples:

- Google may document how `noindex` works.
- That does not mean every `noindex` occurrence is an SEO error.
- OpenAI may document OAI-SearchBot.
- That does not imply allowing the crawler guarantees a citation.
- web.dev may define Core Web Vitals.
- That does not prove CWV is a direct AI Search citation factor.

Rule logic must preserve these distinctions.

## Maintenance

Review primary sources when:

- a platform changes crawler names or controls;
- Google deprecates or updates structured-data features;
- rendering guidance changes;
- Bing changes IndexNow/submission guidance;
- OpenAI changes crawler behavior;
- a previously experimental GEO practice gains primary documentation.

When a source changes materially:

1. update this registry;
2. identify affected rule IDs;
3. increment executable rule versions where behavior changes;
4. update tests and documentation together.
