# Audit Check Catalog
## Purpose
Authoritative domain catalog for broad audit checks. Catalog check IDs are stable domain identifiers. Atomic executable rules use their own stable `AR-*` IDs and reference the source catalog check through `parent_check`.
## Catalog conventions
- `Automation: Full` means the check is a strong candidate for deterministic evaluation.
- `Automation: Partial` means the tool can collect evidence but context or external data may still be required.
- `Automation: Manual` means the tool should guide review rather than invent a machine verdict.
- Default severity is a starting point, not final project priority.
- Evidence labels describe support for the technical requirement, not ranking impact.
- Catalog automation labels describe domain-level automation potential; the frozen V1 executable automation class is defined in `07-v1-atomic-rule-manifest.md`.
- Do not create executable suffix IDs such as `CANON-003a`; use a separate stable atomic rule ID and retain `parent_check: CANON-003`.
## Summary
| Prefix | Category | Checks |
|---|---|---:|
| `ACC` | 1. Truy cập & thu thập dữ liệu | 9 |
| `INDEX` | 2. Lập chỉ mục | 7 |
| `CANON` | 3. URL & Canonical | 9 |
| `LINK` | 4. Điều hướng & Internal Link | 7 |
| `DISC` | 5. Sitemap & Freshness | 7 |
| `RETR` | 6. Nội dung & khả năng máy đọc/hiểu | 9 |
| `ENTITY` | 7. Structured Data & Entity | 6 |
| `RENDER` | 8. JavaScript & Rendering | 7 |
| `PERF` | 9. Tốc độ, Server & Trải nghiệm | 8 |
| `MEDIA` | 10. Hình ảnh & Media | 6 |
| `INTL` | 11. Đa ngôn ngữ / Quốc tế | 4 |
| `AI` | 12. AI Search & GEO | 10 |
| `OBS` | 13. Theo dõi, Logs & kiểm tra sau triển khai | 11 |

# 1. Truy cập & thu thập dữ liệu

## ACC-001 — Website có truy cập được bằng HTTPS ổn định không?
**Lifecycle:** Crawl  
**Automation:** Full  
**Default severity:** P0/P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Trang chính trả về HTTPS hợp lệ; không lỗi chứng chỉ, DNS hoặc kết nối.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Gia hạn/sửa SSL, DNS hoặc cấu hình máy chủ; loại bỏ lỗi kết nối.

### Audit procedure
Mở website và một số URL mẫu → kiểm tra HTTPS → kiểm tra response từ nhiều loại trang.

### Suggested tools / data sources
Browser; DevTools; curl/httpstatus; SSL checker

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## ACC-002 — Googlebot có truy cập được các trang cần SEO không?
**Lifecycle:** Crawl  
**Automation:** Full  
**Default severity:** P0/P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Trang cần SEO không bị robots.txt, đăng nhập, firewall hoặc máy chủ chặn ngoài chủ đích.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Gỡ rule chặn nhầm; cho phép Googlebot truy cập đúng thư mục/trang cần SEO.

### Audit procedure
Kiểm tra robots.txt → crawl site → đối chiếu URL Inspection/Pages trong GSC.

### Suggested tools / data sources
Google Search Console; Screaming Frog/Sitebulb; robots.txt

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://developers.google.com/search/docs/crawling-indexing

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## ACC-003 — robots.txt có chặn nhầm trang quan trọng không?
**Lifecycle:** Crawl  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Không Disallow các thư mục/URL cần crawl; các rule chặn phải đúng mục đích.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa rule Disallow/Allow; kiểm tra lại sau khi deploy.

### Audit procedure
Đọc robots.txt → đối chiếu danh sách URL SEO → test từng rule.

### Suggested tools / data sources
robots.txt; GSC; Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-ROBOTS-001`

## ACC-004 — CSS/JS quan trọng có bị chặn với crawler không?
**Lifecycle:** Crawl  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Crawler có thể tải tài nguyên cần thiết để render nội dung và layout chính.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Gỡ chặn CSS/JS cần thiết trong robots.txt/CDN/WAF.

### Audit procedure
Crawl với Googlebot → kiểm tra blocked resources → so sánh raw HTML và rendered HTML.

### Suggested tools / data sources
Screaming Frog JavaScript rendering; GSC URL Inspection; DevTools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Không cần thêm Allow cho CSS/JS nếu vốn đã không bị chặn.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## ACC-005 — CDN/WAF/Bot protection có chặn crawler hợp lệ không?
**Lifecycle:** Crawl  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Googlebot/Bingbot và bot AI được chủ site cho phép không bị trả 403/429 hoặc challenge bất thường.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Điều chỉnh firewall/bot rule; allowlist theo tài liệu chính thức và IP hợp lệ.

### Audit procedure
Kiểm tra log 403/429 → test user-agent → xác minh IP bot → kiểm tra rule CDN/WAF.

### Suggested tools / data sources
Server/CDN/WAF logs; Cloudflare/Akamai; curl; bot IP verification

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## ACC-006 — Máy chủ có trả lỗi 5xx, timeout hoặc kết nối không ổn định không?
**Lifecycle:** Crawl  
**Automation:** Partial  
**Default severity:** P0/P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Trang quan trọng phản hồi ổn định; không có 5xx/timeout lặp lại.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa server/app/database/CDN; tăng ổn định trước khi tối ưu crawl.

### Audit procedure
Crawl nhiều lần → lọc 5xx/timeouts → đối chiếu GSC và server logs.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC; server logs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## ACC-007 — Response code của URL có đúng mục đích không?
**Lifecycle:** Crawl  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Trang hợp lệ: 200; chuyển vĩnh viễn: 301/308; trang mất: 404/410; tránh trả 200 cho trang lỗi.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa status code và routing tương ứng.

### Audit procedure
Crawl toàn site → phân nhóm 2xx/3xx/4xx/5xx → review theo template.

### Suggested tools / data sources
Screaming Frog/Sitebulb; curl/httpstatus

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## ACC-008 — Có URL phát sinh vô hạn do tham số, filter, search hoặc calendar không?
**Lifecycle:** Crawl  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Crawler không bị cuốn vào không gian URL gần như vô hạn; URL ít giá trị được kiểm soát.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Giảm link sinh tự động; chuẩn hóa filter; dùng robots/noindex/canonical theo đúng trường hợp.

### Audit procedure
Crawl sâu → lọc URL có ?, filter, sort, search, calendar → kiểm tra pattern và số lượng.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC; server logs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-CANONICAL-001`

## ACC-009 — Website có bị malware, redirect lạ hoặc cloaking không?
**Lifecycle:** Crawl  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, fetch status, robots state, network/server evidence  

### Standard / expected state
Không có redirect ngoài ý muốn, nội dung khác biệt bất thường theo user-agent hoặc mã độc.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Làm sạch mã độc/plugin; vá bảo mật; xóa redirect/cloaking.

### Audit procedure
So sánh browser và crawler → kiểm tra Security Issues/Manual Actions → rà soát source.

### Suggested tools / data sources
GSC; Security scanner; DevTools; server logs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

# 2. Lập chỉ mục

## INDEX-001 — Trang cần SEO có cho phép index không?
**Lifecycle:** Index  
**Automation:** Partial  
**Default severity:** P0/P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, meta robots, X-Robots-Tag, indexability intent, optional GSC evidence  

### Standard / expected state
Không có noindex/X-Robots-Tag chặn nhầm; URL có thể crawl để bot đọc directive.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Gỡ noindex/X-Robots-Tag sai; đảm bảo crawler truy cập được trang.

### Audit procedure
Crawl meta robots + HTTP header → kiểm tra URL mẫu trong GSC.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC; curl

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag

### Source references
- `SRC-GOOGLE-ROBOTS-001`

## INDEX-002 — Trang không cần xuất hiện trên Search có được kiểm soát index không?
**Lifecycle:** Index  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, meta robots, X-Robots-Tag, indexability intent, optional GSC evidence  

### Standard / expected state
Trang admin, search nội bộ, test, duplicate hoặc URL ít giá trị được noindex/loại khỏi discovery theo chiến lược.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Áp dụng noindex hoặc loại internal link/sitemap; không dùng robots.txt như cách duy nhất để deindex.

### Audit procedure
Lập nhóm URL không cần index → kiểm tra meta robots → kiểm tra sitemap/internal link.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## INDEX-003 — Có trang lỗi mềm (soft 404) không?
**Lifecycle:** Index  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, meta robots, X-Robots-Tag, indexability intent, optional GSC evidence  

### Standard / expected state
Trang không tồn tại/không có giá trị không trả 200 như một trang nội dung bình thường.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Trả 404/410 hoặc bổ sung nội dung thực nếu trang cần giữ.

### Audit procedure
Lọc soft 404 trong GSC → mở URL → kiểm tra status và nội dung.

### Suggested tools / data sources
GSC; Screaming Frog/Sitebulb; curl

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## INDEX-004 — Trang 404/410 có xử lý đúng không?
**Lifecycle:** Index  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, meta robots, X-Robots-Tag, indexability intent, optional GSC evidence  

### Standard / expected state
URL mất trả 404/410; trang lỗi có điều hướng hữu ích nhưng không giả 200.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa routing/status; tạo 404 page thân thiện nếu cần.

### Audit procedure
Test URL không tồn tại → kiểm tra status → kiểm tra template 404.

### Suggested tools / data sources
Browser; curl/httpstatus; Screaming Frog

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## INDEX-005 — Có index pollution/index bloat không?
**Lifecycle:** Index  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, meta robots, X-Robots-Tag, indexability intent, optional GSC evidence  

### Standard / expected state
Số URL index chủ yếu là các trang có giá trị; không bị tràn bởi filter, search, param, test, tag rác.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Xác định pattern → noindex/canonical/redirect/xóa link phát sinh → cập nhật sitemap.

### Audit procedure
Đối chiếu crawl, GSC Pages và các mẫu URL đang index.

### Suggested tools / data sources
GSC; Screaming Frog/Sitebulb; server logs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## INDEX-006 — Các trang quan trọng có trạng thái index phù hợp trong GSC không?
**Lifecycle:** Index  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, meta robots, X-Robots-Tag, indexability intent, optional GSC evidence  

### Standard / expected state
Trang ưu tiên không nằm trong nhóm Crawled/Discovered - currently not indexed kéo dài không rõ nguyên nhân.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Kiểm tra chất lượng, canonical, internal link, render, server và sitemap trước khi request index.

### Audit procedure
Lấy nhóm URL ưu tiên → kiểm tra Pages/URL Inspection → phân loại nguyên nhân.

### Suggested tools / data sources
Google Search Console

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## INDEX-007 — Quy tắc snippet có vô tình hạn chế Search/AI không?
**Lifecycle:** Index  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, meta robots, X-Robots-Tag, indexability intent, optional GSC evidence  

### Standard / expected state
Không dùng nosnippet, max-snippet quá thấp hoặc data-nosnippet trên phần nội dung cần được trích dẫn ngoài chủ đích.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Nới directive snippet ở nội dung cần hiển thị; chỉ giới hạn phần thật sự cần bảo vệ.

### Audit procedure
Crawl robots directives → review template → kiểm tra phần text có data-nosnippet.

### Suggested tools / data sources
Screaming Frog; source code; GSC

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://developers.google.com/search/docs/crawling-indexing/robots-meta-tag

### Source references
- `SRC-GOOGLE-ROBOTS-001`

# 3. URL & Canonical

## CANON-001 — Website có chỉ một phiên bản host/protocol chính không?
**Lifecycle:** Consolidate  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
HTTP/HTTPS và www/non-www hợp nhất về một phiên bản chính bằng redirect vĩnh viễn.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Thiết lập 301/308 về host chuẩn; đồng bộ canonical, sitemap, internal link.

### Audit procedure
Test 4 biến thể domain → kiểm tra redirect đích và số bước.

### Suggested tools / data sources
curl/httpstatus; Screaming Frog

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-CANONICAL-001`

## CANON-002 — Quy ước dấu / cuối URL có nhất quán không?
**Lifecycle:** Consolidate  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
Mỗi URL chỉ có một phiên bản chính; phiên bản còn lại redirect về chuẩn nếu cả hai cùng tồn tại.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Chuẩn hóa routing, internal link và canonical.

### Audit procedure
Test URL có/không có / → kiểm tra status, canonical và link nội bộ.

### Suggested tools / data sources
Screaming Frog; curl/httpstatus

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## CANON-003 — Canonical của trang indexable có hợp lý không?
**Lifecycle:** Consolidate  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
Trang chính thường self-canonical; canonical trỏ tới URL 200, indexable và cùng nội dung/ý định.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa canonical sai, canonical tới 3xx/4xx/noindex hoặc khác nội dung.

### Audit procedure
Crawl canonical → lọc missing/multiple/non-200 → review mẫu theo template.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://developers.google.com/search/docs/crawling-indexing/consolidate-duplicate-urls

### Source references
- `SRC-GOOGLE-CANONICAL-001`

## CANON-004 — Các tín hiệu canonical có nhất quán không?
**Lifecycle:** Consolidate  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
Redirect, rel=canonical, sitemap và internal link cùng ưu tiên một URL chính.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Đồng bộ tất cả tín hiệu về URL chuẩn.

### Audit procedure
So sánh URL crawl, canonical, sitemap và link nội bộ → tìm trường hợp mâu thuẫn.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-CANONICAL-001`

## CANON-005 — URL tham số/filter có canonical đúng chiến lược không?
**Lifecycle:** Consolidate  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
Canonical phản ánh đúng nội dung tương đương; không canonical tất cả filter về trang gốc nếu filter có giá trị riêng.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Nhóm tham số theo giá trị SEO → canonical/noindex/index riêng tùy mục tiêu.

### Audit procedure
Lấy mẫu từng loại tham số → so sánh nội dung → kiểm tra canonical/index/internal link.

### Suggested tools / data sources
Screaming Frog; GSC; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-CANONICAL-001`

## CANON-006 — Phân trang có crawl được và không tạo duplicate không?
**Lifecycle:** Consolidate  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
Các trang phân trang có URL riêng, link HTML crawlable; canonical theo chiến lược, không bắt buộc rel=prev/next.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa infinite scroll không có URL; tạo paginated URLs và link crawlable.

### Audit procedure
Đi qua pagination → tắt JS thử → crawl page 2/3 → kiểm tra canonical/index.

### Suggested tools / data sources
Screaming Frog; Browser/DevTools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Google không yêu cầu rel=prev/next.

### Source references
- `SRC-GOOGLE-CANONICAL-001`

## CANON-007 — Redirect có đi thẳng đến URL cuối không?
**Lifecycle:** Consolidate  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
Hạn chế redirect chain/loop; internal link trỏ trực tiếp URL cuối.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Cập nhật redirect rule và internal link.

### Audit procedure
Crawl redirects → xuất chains/loops → ưu tiên URL có traffic/backlink.

### Suggested tools / data sources
Screaming Frog/Sitebulb; curl

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## CANON-008 — 302/307 có được dùng đúng khi chuyển hướng tạm thời không?
**Lifecycle:** Consolidate  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
Chuyển vĩnh viễn dùng 301/308; 302/307 chỉ khi thực sự tạm thời.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Đổi loại redirect theo mục đích.

### Audit procedure
Lọc redirect 302/307 → review nguyên nhân và thời gian tồn tại.

### Suggested tools / data sources
Screaming Frog; server config

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## CANON-009 — URL slug có ổn định và dễ hiểu không?
**Lifecycle:** Consolidate  
**Automation:** Manual  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `url`, status, redirect target, canonical target, URL pattern  

### Standard / expected state
URL ngắn gọn, ổn định, không chứa tham số/ID thừa nếu không cần; đổi URL phải có redirect.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Chuẩn hóa URL khi thật sự cần; tránh đổi chỉ vì tối ưu từ khóa.

### Audit procedure
Review pattern URL theo template/directory.

### Suggested tools / data sources
Screaming Frog; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

# 4. Điều hướng & Internal Link

## LINK-001 — Link nội bộ quan trọng có dùng thẻ <a href> crawlable không?
**Lifecycle:** Discover  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `source_url`, `target_url`, anchor, link type, crawl depth / inlinks  

### Standard / expected state
Navigation và internal link chính có href thực, không phụ thuộc chỉ vào onclick/JS.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Chuyển link quan trọng về <a href='...'>.

### Audit procedure
Inspect HTML → crawl links → thử khi JS tắt.

### Suggested tools / data sources
Screaming Frog; DevTools; browser

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics

### Source references
- `SRC-GOOGLE-JS-001`

## LINK-002 — Có internal link trỏ đến 3xx/4xx/5xx không?
**Lifecycle:** Discover  
**Automation:** Full  
**Default severity:** P0/P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `source_url`, `target_url`, anchor, link type, crawl depth / inlinks  

### Standard / expected state
Internal link nên trỏ thẳng URL 200 chính; không trỏ lỗi hoặc redirect không cần thiết.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Cập nhật source link về URL cuối hợp lệ.

### Audit procedure
Crawl inlinks → lọc destination non-200 → sửa theo template.

### Suggested tools / data sources
Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## LINK-003 — Có trang quan trọng bị orphan không?
**Lifecycle:** Discover  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `source_url`, `target_url`, anchor, link type, crawl depth / inlinks  

### Standard / expected state
Trang cần SEO có ít nhất một đường internal link crawlable từ cấu trúc website phù hợp.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Bổ sung link từ hub/category/content liên quan; không chỉ dựa vào sitemap.

### Audit procedure
So sánh URL từ sitemap/GSC/GA/log với URL crawl được → lọc orphan.

### Suggested tools / data sources
Screaming Frog + sitemap/GSC/GA4/logs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## LINK-004 — Độ sâu click của trang quan trọng có hợp lý không?
**Lifecycle:** Discover  
**Automation:** Manual  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `source_url`, `target_url`, anchor, link type, crawl depth / inlinks  

### Standard / expected state
Trang ưu tiên không bị chôn quá sâu; người dùng và crawler có đường đi rõ ràng.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Bổ sung hub, category, breadcrumb hoặc contextual links.

### Audit procedure
Crawl → xem crawl depth → review nhóm URL ưu tiên.

### Suggested tools / data sources
Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## LINK-005 — Breadcrumb có đúng cấu trúc website không?
**Lifecycle:** Discover  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `source_url`, `target_url`, anchor, link type, crawl depth / inlinks  

### Standard / expected state
Breadcrumb hiển thị đường dẫn logic, link crawlable và khớp hierarchy.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa hierarchy/link; bổ sung BreadcrumbList nếu phù hợp.

### Audit procedure
Review template → crawl breadcrumb links → test schema.

### Suggested tools / data sources
Browser; Screaming Frog; Rich Results Test

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## LINK-006 — Anchor text có mô tả đúng trang đích không?
**Lifecycle:** Discover  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `source_url`, `target_url`, anchor, link type, crawl depth / inlinks  

### Standard / expected state
Anchor dễ hiểu, liên quan; tránh lạm dụng một anchor máy móc trên toàn site.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Đổi anchor theo ngữ cảnh và mục đích người dùng.

### Audit procedure
Xuất all inlinks/anchor → review trang ưu tiên và anchor lặp bất thường.

### Suggested tools / data sources
Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## LINK-007 — Có self-link hoặc link lặp vô ích quá nhiều không?
**Lifecycle:** Discover  
**Automation:** Manual  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** `source_url`, `target_url`, anchor, link type, crawl depth / inlinks  

### Standard / expected state
Không tạo khối link dư thừa làm loãng điều hướng; self-link chỉ dùng khi có lý do UX.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Giảm link lặp ở template/menu/footer nếu không cần.

### Audit procedure
Crawl link count → review template có số link bất thường.

### Suggested tools / data sources
Screaming Frog; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

# 5. Sitemap & Freshness

## DISC-001 — Có XML sitemap cho các URL cần index không?
**Lifecycle:** Discover  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** sitemap/feed URL, listed URL, status, indexability, canonical state, timestamps  

### Standard / expected state
Sitemap tồn tại, truy cập được và chỉ chứa URL cần index.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Tạo/cập nhật XML sitemap tự động.

### Audit procedure
Mở sitemap → crawl sitemap → đối chiếu URL indexable.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-SITEMAP-001`

## DISC-002 — Sitemap có chỉ chứa URL 200, canonical và indexable không?
**Lifecycle:** Discover  
**Automation:** Full  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** sitemap/feed URL, listed URL, status, indexability, canonical state, timestamps  

### Standard / expected state
Không chứa 3xx/4xx/5xx, noindex hoặc URL canonical sang trang khác.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Loại URL không hợp lệ khỏi sitemap.

### Audit procedure
Crawl sitemap → join status/indexability/canonical.

### Suggested tools / data sources
Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-CANONICAL-001`
- `SRC-GOOGLE-SITEMAP-001`

## DISC-003 — Sitemap index có tổ chức hợp lý không?
**Lifecycle:** Discover  
**Automation:** Manual  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** sitemap/feed URL, listed URL, status, indexability, canonical state, timestamps  

### Standard / expected state
Sitemap lớn được chia theo loại nội dung/directory hợp lý; file nằm trong giới hạn protocol.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Chia sitemap và sitemap index tự động theo template.

### Audit procedure
Kiểm tra số URL/dung lượng và cấu trúc từng file.

### Suggested tools / data sources
Screaming Frog; sitemap validator

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-SITEMAP-001`

## DISC-004 — Sitemap có được khai báo trong robots.txt/GSC/Bing không?
**Lifecycle:** Discover  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** sitemap/feed URL, listed URL, status, indexability, canonical state, timestamps  

### Standard / expected state
Công cụ tìm kiếm biết sitemap chính; submission không lỗi kéo dài.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Khai báo lại sitemap đúng URL và sửa lỗi fetch.

### Audit procedure
Kiểm tra robots.txt → GSC Sitemaps → Bing Webmaster Tools.

### Suggested tools / data sources
GSC; Bing Webmaster Tools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-ROBOTS-001`
- `SRC-GOOGLE-SITEMAP-001`

## DISC-005 — lastmod có phản ánh thay đổi nội dung thật không?
**Lifecycle:** Discover  
**Automation:** Manual  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** sitemap/feed URL, listed URL, status, indexability, canonical state, timestamps  

### Standard / expected state
lastmod chỉ đổi khi nội dung quan trọng thay đổi, không tự refresh toàn site mỗi ngày.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Sửa logic CMS để lastmod phản ánh lần cập nhật thực.

### Audit procedure
So sánh lastmod với lịch sử sửa bài và timestamp thực.

### Suggested tools / data sources
Sitemap; CMS; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-SITEMAP-001`

## DISC-006 — Nội dung thay đổi nhanh có cơ chế thông báo cập nhật phù hợp không?
**Lifecycle:** Discover  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** sitemap/feed URL, listed URL, status, indexability, canonical state, timestamps  

### Standard / expected state
Website có quy trình cập nhật sitemap; với Bing có thể dùng IndexNow nếu phù hợp.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Tích hợp IndexNow cho URL mới/cập nhật/xóa; không submit URL không đổi liên tục.

### Audit procedure
Kiểm tra CMS/CDN integration → xem lịch sử submission trong Bing.

### Suggested tools / data sources
Bing Webmaster Tools; IndexNow; CMS

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://www.bing.com/webmasters/help/url-submission-62f2860b

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## DISC-007 — Website tin tức/blog có feed RSS/Atom hữu ích không? (nếu áp dụng)
**Lifecycle:** Discover  
**Automation:** Partial  
**Default severity:** P3  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** sitemap/feed URL, listed URL, status, indexability, canonical state, timestamps  

### Standard / expected state
Feed hoạt động, cập nhật đúng nội dung mới; không cần vô hiệu hóa feed chỉ vì SEO.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa feed hoặc giữ nguyên nếu đang phục vụ discovery/subscriber.

### Audit procedure
Mở feed → kiểm tra URL/date/title → đối chiếu bài mới.

### Suggested tools / data sources
Browser; feed validator

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Chỉ áp dụng website có nhu cầu feed.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

# 6. Nội dung & khả năng máy đọc/hiểu

## RETR-001 — Nội dung chính có xuất hiện rõ trong HTML/rendered DOM không?
**Lifecycle:** Retrieve  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
Tiêu đề, đoạn trả lời chính, dữ kiện quan trọng có thể đọc được sau render và không chỉ nằm trong canvas/ảnh.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Đưa nội dung quan trọng vào HTML/DOM; giảm phụ thuộc thành phần không crawl được.

### Audit procedure
So sánh view-source/raw HTML với rendered HTML → kiểm tra main content.

### Suggested tools / data sources
DevTools; Screaming Frog JS rendering; GSC URL Inspection

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-JS-001`

## RETR-002 — Trang có một vùng nội dung chính rõ ràng không?
**Lifecycle:** Retrieve  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
Cấu trúc semantic hợp lý: main/article/section/nav/header/footer khi phù hợp; tránh DOM lộn xộn.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Chỉnh template và semantic HTML, ưu tiên cấu trúc dễ hiểu.

### Audit procedure
Inspect template → kiểm tra semantic elements và DOM.

### Suggested tools / data sources
DevTools; HTML validator; Screaming Frog

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RETR-003 — Heading có thể hiện đúng cấu trúc nội dung không?
**Lifecycle:** Retrieve  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
H1/H2/H3 mô tả hierarchy; tránh heading trống hoặc dùng chỉ để tạo style.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa heading theo cấu trúc nội dung.

### Audit procedure
Crawl H1/H2 → review template và các trang mẫu.

### Suggested tools / data sources
Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RETR-004 — Title có duy nhất và mô tả đúng trang không?
**Lifecycle:** Retrieve  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
Trang indexable có title hữu ích, không trùng hàng loạt do template lỗi.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa template/title theo intent và nội dung thực.

### Audit procedure
Crawl title → lọc missing/duplicate/quá giống nhau → review.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RETR-005 — Meta description có hợp lý cho các trang quan trọng không?
**Lifecycle:** Retrieve  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
Không thiếu/trùng hàng loạt do lỗi template; mô tả đúng nội dung, không nhồi từ khóa.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Sửa template hoặc viết riêng cho landing page quan trọng.

### Audit procedure
Crawl description → lọc missing/duplicate → review mẫu.

### Suggested tools / data sources
Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RETR-006 — Thông tin quan trọng có được viết thành text thay vì chỉ nằm trong ảnh không?
**Lifecycle:** Retrieve  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
Giá, thông số, tên sản phẩm/dịch vụ, câu trả lời chính có text HTML khi có ý nghĩa.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Bổ sung text tương đương; giữ ảnh là phần minh họa.

### Audit procedure
Review trang mẫu → tắt ảnh/CSS thử → kiểm tra thông tin còn đọc được.

### Suggested tools / data sources
Browser/DevTools; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RETR-007 — Bảng, danh sách và định nghĩa có markup dễ đọc không?
**Lifecycle:** Retrieve  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
Thông tin dạng bảng/list dùng HTML phù hợp; không mô phỏng hoàn toàn bằng div/ảnh khi không cần.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Chuyển sang table/ul/ol/dl hoặc markup rõ ràng.

### Audit procedure
Inspect HTML → review các template có dữ liệu cấu trúc.

### Suggested tools / data sources
DevTools; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RETR-008 — Ngày xuất bản/cập nhật và tác giả có hiển thị rõ khi cần không?
**Lifecycle:** Retrieve  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
Bài có tính thời gian/chuyên môn hiển thị ngày/tác giả nhất quán với dữ liệu cấu trúc.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Bổ sung hoặc đồng bộ thông tin hiển thị và schema.

### Audit procedure
Review article template → đối chiếu visible text với schema.

### Suggested tools / data sources
Browser; Schema Validator/Rich Results Test

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Không cần rel=author.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RETR-009 — Nguồn tham khảo và link dẫn chứng có rõ khi nội dung cần bằng chứng không?
**Lifecycle:** Retrieve  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** raw HTML, rendered DOM, semantic structure, visible text, page template  

### Standard / expected state
Claim quan trọng có link tới nguồn phù hợp; anchor và ngữ cảnh cho biết nguồn hỗ trợ điều gì.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Bổ sung nguồn chính thống/nguồn gốc và đặt link tại đúng claim.

### Audit procedure
Review trang chuyên sâu/YMYL → kiểm tra claim, nguồn, ngày và tác giả.

### Suggested tools / data sources
Manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Tốt cho độ tin cậy và khả năng trích dẫn; không phải 'GEO ranking factor' được xác nhận.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

# 7. Structured Data & Entity

## ENTITY-001 — Structured data có hợp lệ về cú pháp không?
**Lifecycle:** Retrieve  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** structured data graph, visible content, entity identifiers, page type  

### Standard / expected state
Không có lỗi cú pháp; dùng type/property được hỗ trợ hoặc hợp lệ trên Schema.org.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa JSON-LD/Microdata/RDFa lỗi.

### Audit procedure
Crawl schema → test URL mẫu → xử lý errors trước warnings.

### Suggested tools / data sources
Rich Results Test; Schema.org Validator; Screaming Frog

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-STRUCTURED-001`
- `SRC-SCHEMA-VALIDATOR-001`

## ENTITY-002 — Structured data có khớp nội dung người dùng nhìn thấy không?
**Lifecycle:** Retrieve  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** structured data graph, visible content, entity identifiers, page type  

### Standard / expected state
Tên, giá, rating, author, date, availability... không mâu thuẫn với visible content.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Đồng bộ dữ liệu từ một nguồn CMS/API.

### Audit procedure
So sánh schema với giao diện trên các template chính.

### Suggested tools / data sources
Rich Results Test; browser; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-STRUCTURED-001`
- `SRC-SCHEMA-VALIDATOR-001`

## ENTITY-003 — Mỗi loại trang có schema phù hợp khi thực sự có dữ liệu không?
**Lifecycle:** Retrieve  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** structured data graph, visible content, entity identifiers, page type  

### Standard / expected state
Dùng Article/Product/Organization/LocalBusiness/Person/Breadcrumb... theo đúng nội dung; không gắn schema không liên quan.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Bổ sung/bỏ type theo template.

### Audit procedure
Map page type → schema type → test URL mẫu.

### Suggested tools / data sources
Schema.org Validator; Rich Results Test

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-STRUCTURED-001`
- `SRC-SCHEMA-VALIDATOR-001`

## ENTITY-004 — Entity chính có tên/URL/ID nhất quán không?
**Lifecycle:** Retrieve  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** structured data graph, visible content, entity identifiers, page type  

### Standard / expected state
Tên thương hiệu, tổ chức, tác giả, địa điểm nhất quán giữa nội dung, schema và trang profile.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Chuẩn hóa tên, URL entity, @id và sameAs khi có nguồn chính thức.

### Audit procedure
Đối chiếu Organization/Person/LocalBusiness với trang About/Author/Contact.

### Suggested tools / data sources
Schema Validator; Screaming Frog; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## ENTITY-005 — Breadcrumb structured data có khớp breadcrumb hiển thị không?
**Lifecycle:** Retrieve  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** structured data graph, visible content, entity identifiers, page type  

### Standard / expected state
Các item, position và URL phù hợp với breadcrumb trên trang.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa generated schema hoặc breadcrumb UI.

### Audit procedure
Test template category/product/article.

### Suggested tools / data sources
Rich Results Test; Schema Validator

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-STRUCTURED-001`
- `SRC-SCHEMA-VALIDATOR-001`

## ENTITY-006 — Có đang dùng 'schema GEO/AI' tự chế hoặc không có căn cứ không?
**Lifecycle:** Retrieve  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Strong practice / Context-dependent  
**Required data (minimum):** structured data graph, visible content, entity identifiers, page type  

### Standard / expected state
Không coi schema không chuẩn là yêu cầu bắt buộc; ưu tiên schema.org + nội dung hiển thị chính xác.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Loại markup tự chế gây nhiễu; dùng type/property có tài liệu.

### Audit procedure
Review JSON-LD và custom properties.

### Suggested tools / data sources
Schema.org Validator; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Không có 'special GEO schema' bắt buộc.

### Source references
- `SRC-GOOGLE-STRUCTURED-001`
- `SRC-SCHEMA-VALIDATOR-001`

# 8. JavaScript & Rendering

## RENDER-001 — Google render được nội dung chính trên desktop/mobile không?
**Lifecycle:** Render  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** raw HTML, rendered HTML, network requests, metadata before/after render  

### Standard / expected state
Rendered HTML chứa title, main content, internal links và dữ liệu quan trọng.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa JS/hydration/API; SSR/prerender phần cần thiết nếu render client-side gây mất nội dung.

### Audit procedure
Test URL Inspection → xem HTML/screenshot → so sánh browser.

### Suggested tools / data sources
GSC URL Inspection; Screaming Frog JS rendering; DevTools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://developers.google.com/search/docs/crawling-indexing/javascript/javascript-seo-basics

### Source references
- `SRC-GOOGLE-JS-001`

## RENDER-002 — Raw HTML và rendered HTML có khác biệt gây rủi ro SEO không?
**Lifecycle:** Render  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** raw HTML, rendered HTML, network requests, metadata before/after render  

### Standard / expected state
Không có title/canonical/robots/content quan trọng bị đổi sai sau render.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Đồng bộ server/client output; tránh JS ghi đè metadata sai.

### Audit procedure
Lưu raw HTML và rendered HTML → diff các trường SEO chính.

### Suggested tools / data sources
Screaming Frog; DevTools; view-source

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-JS-001`

## RENDER-003 — Navigation và pagination có hoạt động khi JS lỗi/chậm không?
**Lifecycle:** Render  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** raw HTML, rendered HTML, network requests, metadata before/after render  

### Standard / expected state
URL quan trọng vẫn có href crawlable; không chỉ phụ thuộc event JS.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Thêm server-rendered links hoặc progressive enhancement.

### Audit procedure
Tắt JS/giới hạn network → thử menu, category, pagination.

### Suggested tools / data sources
Browser DevTools; Screaming Frog

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RENDER-004 — Lazy load có làm nội dung/hình ảnh quan trọng không được render không?
**Lifecycle:** Render  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** raw HTML, rendered HTML, network requests, metadata before/after render  

### Standard / expected state
Nội dung trong viewport và ảnh quan trọng tải bình thường; crawler có thể phát hiện URL media.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa lazy-load implementation; dùng src/srcset hoặc phương án tương thích.

### Audit procedure
Scroll/test rendered DOM → crawl image URLs → kiểm tra LCP image.

### Suggested tools / data sources
DevTools; Lighthouse/PSI; Screaming Frog

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-JS-001`

## RENDER-005 — Canonical/meta robots được tạo bằng JS có ổn định không?
**Lifecycle:** Render  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** raw HTML, rendered HTML, network requests, metadata before/after render  

### Standard / expected state
Không xuất hiện hai canonical hoặc robots xung đột giữa HTML ban đầu và DOM sau render.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Ưu tiên output ổn định từ server; nếu JS tạo thì chỉ có một giá trị cuối cùng đúng.

### Audit procedure
So sánh source và rendered head.

### Suggested tools / data sources
DevTools; Screaming Frog JS rendering

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Google khuyến nghị tránh JS thay canonical sang giá trị khác với HTML ban đầu.

### Source references
- `SRC-GOOGLE-ROBOTS-001`
- `SRC-GOOGLE-CANONICAL-001`
- `SRC-GOOGLE-JS-001`

## RENDER-006 — API/resource cần cho nội dung có lỗi 4xx/5xx/CORS không?
**Lifecycle:** Render  
**Automation:** Partial  
**Default severity:** P0/P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** raw HTML, rendered HTML, network requests, metadata before/after render  

### Standard / expected state
Resource cần thiết tải được với crawler và người dùng.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa endpoint, auth, CORS, cache hoặc rate limit.

### Audit procedure
DevTools Network → lọc failed requests → đối chiếu rendered content.

### Suggested tools / data sources
DevTools; server logs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## RENDER-007 — Website có dùng infinite scroll mà không có URL riêng không?
**Lifecycle:** Render  
**Automation:** Partial  
**Default severity:** P1  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** raw HTML, rendered HTML, network requests, metadata before/after render  

### Standard / expected state
Mỗi phần nội dung cần discovery có URL/pagination crawlable riêng.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Bổ sung paginated URLs và link <a href>.

### Audit procedure
Scroll đến cuối → kiểm tra URL thay đổi/link page tiếp theo → crawl.

### Suggested tools / data sources
Browser; Screaming Frog

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-JS-001`

# 9. Tốc độ, Server & Trải nghiệm

## PERF-001 — Core Web Vitals thực tế có đạt mức tốt không?
**Lifecycle:** Measure / Experience  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed metric; limited direct GEO evidence  
**Required data (minimum):** field/lab performance data, response headers, server timing, viewport evidence  

### Standard / expected state
Mục tiêu p75: LCP ≤ 2,5s; INP ≤ 200ms; CLS ≤ 0,1 khi có đủ field data.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Ưu tiên fix theo template có traffic: server/LCP resource, JS main thread, layout shifts.

### Audit procedure
Kiểm tra CrUX/PSI/GSC → nhóm theo template → xác định nguyên nhân lab.

### Suggested tools / data sources
PageSpeed Insights; CrUX; GSC; Lighthouse

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://web.dev/articles/vitals

### Source references
- `SRC-WEBDEV-CWV-001`

## PERF-002 — TTFB/server response có bất thường trên trang quan trọng không?
**Lifecycle:** Measure / Experience  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** field/lab performance data, response headers, server timing, viewport evidence  

### Standard / expected state
Không có độ trễ server kéo dài/bất thường theo template, bot hoặc thời điểm.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Tối ưu backend/database/cache/CDN; điều tra request chậm.

### Audit procedure
Test nhiều URL/mốc thời gian → đối chiếu logs/APM.

### Suggested tools / data sources
WebPageTest/DevTools; server logs/APM; curl

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## PERF-003 — Có bật compression cho text assets không?
**Lifecycle:** Measure / Experience  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** field/lab performance data, response headers, server timing, viewport evidence  

### Standard / expected state
HTML/CSS/JS/text response được nén bằng Brotli/Gzip khi phù hợp.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Bật compression ở server/CDN.

### Audit procedure
Kiểm tra response headers Content-Encoding.

### Suggested tools / data sources
DevTools; curl; PageSpeed Insights

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## PERF-004 — Cache headers có hợp lý không?
**Lifecycle:** Measure / Experience  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** field/lab performance data, response headers, server timing, viewport evidence  

### Standard / expected state
Static assets có cache dài và fingerprint khi đổi; HTML/API dùng cache phù hợp nội dung.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Thiết lập Cache-Control/ETag/versioned assets.

### Audit procedure
Inspect response headers cho HTML, CSS, JS, image.

### Suggested tools / data sources
DevTools; curl

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## PERF-005 — HTTP/2 hoặc HTTP/3 có được dùng nếu hạ tầng hỗ trợ không?
**Lifecycle:** Measure / Experience  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** field/lab performance data, response headers, server timing, viewport evidence  

### Standard / expected state
Kết nối hiện đại hoạt động ổn định; không coi đây là điều kiện SEO bắt buộc.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Bật trên CDN/server nếu có lợi và không gây lỗi.

### Audit procedure
Kiểm tra protocol trong DevTools/HTTP test.

### Suggested tools / data sources
DevTools; curl

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## PERF-006 — DOM có quá lớn/phức tạp gây render chậm không?
**Lifecycle:** Measure / Experience  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** field/lab performance data, response headers, server timing, viewport evidence  

### Standard / expected state
Không có DOM phình bất thường do template/component lặp; ưu tiên giảm khi ảnh hưởng performance.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Giảm node lặp, component thừa, hidden DOM không cần thiết.

### Audit procedure
Lighthouse/DevTools → review DOM size theo template.

### Suggested tools / data sources
Lighthouse; DevTools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-JS-001`

## PERF-007 — Trang mobile có viewport và layout sử dụng được không?
**Lifecycle:** Measure / Experience  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** field/lab performance data, response headers, server timing, viewport evidence  

### Standard / expected state
Có viewport phù hợp; không overflow ngang; nội dung/chức năng chính sử dụng được trên mobile.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa responsive CSS/layout và viewport.

### Audit procedure
Test các template ở mobile widths → kiểm tra overflow/tap/zoom.

### Suggested tools / data sources
Chrome DevTools; Lighthouse

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## PERF-008 — Interstitial/banner có che nội dung chính quá mức không?
**Lifecycle:** Measure / Experience  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** field/lab performance data, response headers, server timing, viewport evidence  

### Standard / expected state
Popup không làm người dùng khó truy cập nội dung, đặc biệt trên mobile.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Giảm kích thước/thời điểm hiển thị; tránh che toàn màn hình khi vừa vào.

### Audit procedure
Test trang từ mobile/incognito và các landing page chính.

### Suggested tools / data sources
Browser; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

# 10. Hình ảnh & Media

## MEDIA-001 — Ảnh có alt phù hợp không?
**Lifecycle:** Retrieve  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** media URL, markup, dimensions, alt text, loading behavior  

### Standard / expected state
Ảnh mang thông tin có alt mô tả; ảnh trang trí có thể alt rỗng; tránh nhồi keyword.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Bổ sung/sửa alt theo ngữ cảnh.

### Audit procedure
Crawl images → lọc missing alt → review ảnh quan trọng.

### Suggested tools / data sources
Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## MEDIA-002 — Ảnh có kích thước/tệp hợp lý không?
**Lifecycle:** Retrieve  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** media URL, markup, dimensions, alt text, loading behavior  

### Standard / expected state
Không tải ảnh lớn hơn nhiều so với kích thước hiển thị; ưu tiên format hiện đại khi phù hợp.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Resize/compress; dùng WebP/AVIF hoặc định dạng phù hợp.

### Audit procedure
Lọc image size → kiểm tra dimensions và transfer size.

### Suggested tools / data sources
Screaming Frog; PageSpeed Insights; DevTools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## MEDIA-003 — Ảnh responsive có srcset/sizes khi cần không?
**Lifecycle:** Retrieve  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** media URL, markup, dimensions, alt text, loading behavior  

### Standard / expected state
Thiết bị không phải tải cùng ảnh rất lớn cho mọi viewport.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Triển khai srcset/sizes hoặc image component tối ưu.

### Audit procedure
Inspect img → kiểm tra currentSrc trên mobile/desktop.

### Suggested tools / data sources
DevTools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## MEDIA-004 — Ảnh LCP có bị lazy-load sai không?
**Lifecycle:** Retrieve  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed metric; limited direct GEO evidence  
**Required data (minimum):** media URL, markup, dimensions, alt text, loading behavior  

### Standard / expected state
Ảnh hero/LCP thường không bị trì hoãn bởi lazy loading không cần thiết.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Bỏ lazy cho LCP image; ưu tiên preload/fetchpriority khi phù hợp.

### Audit procedure
PSI/Lighthouse → xác định LCP element → inspect loading behavior.

### Suggested tools / data sources
PageSpeed Insights; Lighthouse; DevTools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-JS-001`
- `SRC-WEBDEV-CWV-001`

## MEDIA-005 — Video/iframe có làm trang nặng hoặc chặn nội dung không?
**Lifecycle:** Retrieve  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** media URL, markup, dimensions, alt text, loading behavior  

### Standard / expected state
Embed tải hợp lý; không làm content quan trọng phụ thuộc hoàn toàn vào iframe.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Dùng facade/lazy load; bổ sung text/tóm tắt HTML khi cần.

### Audit procedure
Review template có video → đo network và rendered content.

### Suggested tools / data sources
DevTools; Lighthouse

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## MEDIA-006 — Image sitemap/video sitemap có cần thiết không? (nếu áp dụng)
**Lifecycle:** Retrieve  
**Automation:** Partial  
**Default severity:** P3  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** media URL, markup, dimensions, alt text, loading behavior  

### Standard / expected state
Chỉ triển khai khi media là tài sản SEO quan trọng và khó discovery qua HTML thông thường.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Tạo media sitemap theo protocol và chỉ chứa URL hợp lệ.

### Audit procedure
Đánh giá website media-heavy → test discovery/indexing.

### Suggested tools / data sources
GSC; sitemap validator

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Không bắt buộc cho mọi website.

### Source references
- `SRC-GOOGLE-SITEMAP-001`

# 11. Đa ngôn ngữ / Quốc tế

## INTL-001 — Thẻ lang của trang có đúng ngôn ngữ không?
**Lifecycle:** Consolidate  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** locale URL, lang, hreflang cluster, canonical, response status  

### Standard / expected state
html lang phản ánh ngôn ngữ chính của trang.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa lang theo template/ngôn ngữ.

### Audit procedure
Crawl/extract lang → so sánh URL locale.

### Suggested tools / data sources
Screaming Frog; browser

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Chỉ cần nếu site có/quan tâm ngôn ngữ rõ ràng.

### Source references
- `SRC-GOOGLE-I18N-001`

## INTL-002 — hreflang có đúng cặp ngôn ngữ/khu vực không? (nếu áp dụng)
**Lifecycle:** Consolidate  
**Automation:** Full  
**Default severity:** P3  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** locale URL, lang, hreflang cluster, canonical, response status  

### Standard / expected state
URL alternate tồn tại, 200, indexable và liên kết đối ứng; mã ngôn ngữ hợp lệ.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa URL/mã hreflang và reciprocal tags.

### Audit procedure
Crawl hreflang → lọc missing return/non-200/noindex.

### Suggested tools / data sources
Screaming Frog/Sitebulb; hreflang validator

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Chỉ áp dụng site đa ngôn ngữ/đa quốc gia.

### Source references
- `SRC-GOOGLE-I18N-001`

## INTL-003 — x-default có được dùng đúng khi cần không?
**Lifecycle:** Consolidate  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** locale URL, lang, hreflang cluster, canonical, response status  

### Standard / expected state
x-default trỏ tới trang mặc định/language selector phù hợp, không bắt buộc cho mọi site.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Bổ sung hoặc sửa x-default theo kiến trúc locale.

### Audit procedure
Review hreflang cluster và URL mặc định.

### Suggested tools / data sources
Screaming Frog; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-I18N-001`

## INTL-004 — Canonical có xung đột với hreflang không?
**Lifecycle:** Consolidate  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** locale URL, lang, hreflang cluster, canonical, response status  

### Standard / expected state
Mỗi alternate thường canonical về chính nó trong cluster tương ứng; không canonical tất cả locale về một ngôn ngữ.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Sửa canonical/hreflang đồng bộ.

### Audit procedure
Join canonical + hreflang theo cluster.

### Suggested tools / data sources
Screaming Frog/Sitebulb

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-CANONICAL-001`
- `SRC-GOOGLE-I18N-001`

# 12. AI Search & GEO

## AI-001 — OAI-SearchBot có được phép crawl nếu muốn xuất hiện trong ChatGPT Search không?
**Lifecycle:** Crawl  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Platform-specific / Confirmed  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
robots.txt không chặn OAI-SearchBot trên nội dung muốn được ChatGPT Search khám phá; WAF không chặn IP hợp lệ.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Cho phép OAI-SearchBot theo mục tiêu; kiểm tra CDN/WAF và IP ranges chính thức.

### Audit procedure
Đọc robots.txt → test access → kiểm tra logs 403/429 cho OAI-SearchBot.

### Suggested tools / data sources
robots.txt; CDN/WAF logs; server logs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
https://developers.openai.com/api/docs/bots

### Source references
- `SRC-OPENAI-BOTS-001`

## AI-002 — GPTBot policy có được tách riêng khỏi ChatGPT Search không?
**Lifecycle:** Crawl  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Platform-specific / Confirmed  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Quyết định allow/disallow GPTBot theo chính sách training của doanh nghiệp; không nhầm GPTBot với OAI-SearchBot.

### Failure or review condition
Blocking GPTBot is not inherently an audit failure. Compare the observed GPTBot robots policy with an explicitly supplied business training policy. Return `PASS` when they match; return `WARNING` when an explicit policy conflicts with the observed configuration; return `MANUAL_REVIEW` when business training policy has not been supplied; return `UNKNOWN` when the effective robots state cannot be determined. Do not infer ChatGPT Search accessibility from GPTBot state.

### Recommended fix
Cấu hình robots.txt riêng cho GPTBot theo policy nội bộ khi policy đã được xác định; không thay đổi GPTBot chỉ để tối ưu ChatGPT Search.

### Audit procedure
Review robots rules cho OAI-SearchBot và GPTBot riêng biệt.

### Suggested tools / data sources
robots.txt

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
GPTBot phục vụ training; OAI-SearchBot phục vụ Search.

### Source references
- `SRC-OPENAI-BOTS-001`

## AI-003 — Các crawler AI mục tiêu khác có bị WAF/robots chặn nhầm không?
**Lifecycle:** Retrieve / Cite  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Strong evidence  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Chỉ allow các bot phù hợp chiến lược; xác minh theo tài liệu/IP chính thức thay vì tin user-agent đơn thuần.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Điều chỉnh robots/WAF từng bot; tránh allowlist mù.

### Audit procedure
Kiểm tra logs → xác minh bot → đối chiếu policy nhà cung cấp.

### Suggested tools / data sources
Server/CDN logs; robots.txt; vendor docs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-ROBOTS-001`

## AI-004 — Nội dung cần được trích dẫn có nằm trong phần crawl/render được không?
**Lifecycle:** Retrieve / Cite  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Emerging  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Định nghĩa, số liệu, thông tin dịch vụ/sản phẩm và câu trả lời chính có text HTML rõ ràng.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Đưa thông tin cốt lõi khỏi ảnh/canvas/interaction khó truy cập; giữ cấu trúc text rõ.

### Audit procedure
Kiểm tra raw/rendered HTML → tìm các facts quan trọng.

### Suggested tools / data sources
DevTools; Screaming Frog JS rendering; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-JS-001`

## AI-005 — Snippet controls có làm giảm khả năng trích xuất nội dung ngoài chủ đích không?
**Lifecycle:** Retrieve / Cite  
**Automation:** Full  
**Default severity:** P2  
**Evidence strength:** Platform-specific / Confirmed  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Không chặn snippet trên phần muốn Search/AI sử dụng; chỉ giới hạn phần nhạy cảm/không muốn trích.

### Failure or review condition
Return `FAIL` when the observed state materially violates the standard above. Implementation must convert this requirement into explicit testable conditions before coding.

### Recommended fix
Điều chỉnh nosnippet/max-snippet/data-nosnippet có chủ đích.

### Audit procedure
Crawl directives → review theo template và content blocks.

### Suggested tools / data sources
Screaming Frog; source code

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## AI-006 — Thông tin entity có nhất quán giữa trang, schema và profile không?
**Lifecycle:** Retrieve / Cite  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Strong evidence  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Brand/organization/author/location dùng cùng tên, URL, thông tin cốt lõi; tránh nhiều phiên bản mâu thuẫn.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Chuẩn hóa entity data từ CMS/knowledge source.

### Audit procedure
Đối chiếu About/Author/Contact/Local pages với schema.

### Suggested tools / data sources
Manual review; Schema Validator; crawler

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-STRUCTURED-001`
- `SRC-SCHEMA-VALIDATOR-001`

## AI-007 — Các claim quan trọng có nguồn/dẫn chứng và ngữ cảnh rõ không?
**Lifecycle:** Retrieve / Cite  
**Automation:** Manual  
**Default severity:** P2  
**Evidence strength:** Emerging  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Trang chuyên sâu có thể xác định claim, nguồn, ngày và tác giả; nguồn đặt gần nội dung liên quan.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Bổ sung nguồn đáng tin; cập nhật nguồn lỗi/thời.

### Audit procedure
Review content quan trọng → test outbound source links.

### Suggested tools / data sources
Manual review; link checker

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Đây là 'citation readiness', không phải yếu tố xếp hạng AI đã được xác nhận.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## AI-008 — Có đang coi llms.txt là yêu cầu bắt buộc không?
**Lifecycle:** Retrieve / Cite  
**Automation:** Manual  
**Default severity:** P3  
**Evidence strength:** Experimental  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Không xem llms.txt là điều kiện cần cho Google/ChatGPT Search khi chưa có bằng chứng chính thức.

### Failure or review condition
Return `MANUAL_REVIEW` when a reviewer must determine whether the observed implementation satisfies the standard above. Do not fabricate a deterministic FAIL condition.

### Recommended fix
Chỉ thử nghiệm nếu có use case; không ưu tiên hơn crawl/index/render/canonical.

### Audit procedure
Review backlog technical → đảm bảo core issues được ưu tiên trước.

### Suggested tools / data sources
Manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Experimental/optional.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## AI-009 — Structured data có đang được dùng như 'mẹo GEO' thay vì dữ liệu đúng không?
**Lifecycle:** Retrieve / Cite  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Guardrail / No confirmed requirement  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Schema phục vụ mô tả entity/content chính xác; không kỳ vọng markup tự tạo sẽ đảm bảo citation.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Quay về schema.org hợp lệ và nội dung visible nhất quán.

### Audit procedure
Review schema custom/unsupported và claims marketing.

### Suggested tools / data sources
Schema Validator; manual review

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-GOOGLE-STRUCTURED-001`
- `SRC-SCHEMA-VALIDATOR-001`

## AI-010 — Nội dung mới/cập nhật có được discovery nhanh trên các nền tảng phù hợp không?
**Lifecycle:** Retrieve / Cite  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Strong evidence  
**Required data (minimum):** robots policy, AI bot accessibility, rendered content, snippet controls, entity/source evidence  

### Standard / expected state
Sitemap/lastmod hoạt động; Bing/participating engines có thể nhận IndexNow; AI crawler không bị chặn nếu chủ site muốn.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa feed/sitemap/IndexNow/robots theo từng nền tảng.

### Audit procedure
Đăng/cập nhật URL test → theo dõi crawl/log/submission.

### Suggested tools / data sources
GSC; Bing Webmaster Tools; server logs

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

# 13. Theo dõi, Logs & kiểm tra sau triển khai

## OBS-001 — Google Search Console đã được xác minh và theo dõi đúng property chưa?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Property đúng domain/protocol; sitemap, Pages, CWV, Manual Actions/Security có dữ liệu.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Xác minh Domain property và phân quyền phù hợp.

### Audit procedure
Kiểm tra property → sitemap → Pages → Enhancements/Security.

### Suggested tools / data sources
Google Search Console

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-002 — Bing Webmaster Tools đã được thiết lập nếu dự án cần Bing/AI discovery chưa?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Site được verify; sitemap/IndexNow/index status có thể theo dõi.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Verify site và submit sitemap/IndexNow nếu phù hợp.

### Audit procedure
Kiểm tra site, sitemap và IndexNow dashboard.

### Suggested tools / data sources
Bing Webmaster Tools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-BING-INDEXNOW-001`

## OBS-003 — GA4/GTM có ghi nhận organic và referral đúng không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Không duplicate tag/pageview; organic/referral không bị sai do self-referral hoặc cross-domain lỗi.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa tagging/cross-domain/consent config.

### Audit procedure
Debug visit → kiểm tra realtime/debugview/source-medium.

### Suggested tools / data sources
GA4; GTM Preview; browser

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-004 — Có access log đủ dữ liệu để phân tích bot khi cần không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Log có timestamp, URL, status, user-agent, IP, response time/bytes khi hạ tầng cho phép.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Bật/giữ log hợp lý; chuẩn hóa timezone và retention.

### Audit procedure
Lấy mẫu log 7–30 ngày → kiểm tra trường dữ liệu.

### Suggested tools / data sources
Server/CDN logs; log analyzer

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Ưu tiên site lớn/JS phức tạp/indexing bất thường.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-005 — Bot trong log có được xác minh, tránh user-agent giả không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Google/OpenAI bot quan trọng được xác minh bằng DNS/IP ranges hoặc phương pháp chính thức.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Loại traffic bot giả khỏi phân tích; allow/block theo bot thật.

### Audit procedure
Lọc user-agent → kiểm tra IP → xác minh nguồn.

### Suggested tools / data sources
Server logs; DNS lookup; published bot IP ranges

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-006 — Bot có dành nhiều crawl cho URL ít giá trị không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Tỷ lệ crawl tập trung vào URL hữu ích; filter/search/error không chiếm bất thường.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Giảm discovery URL rác, sửa internal link/robots/routing, dọn index bloat.

### Audit procedure
Group log theo directory/status/query pattern → so với URL SEO.

### Suggested tools / data sources
Log analyzer; Screaming Frog

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-007 — Trang quan trọng có được crawl đủ thường xuyên theo nhu cầu không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
URL cập nhật thường xuyên/quan trọng không bị bỏ quên bất thường trong log.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Cải thiện internal link, sitemap/lastmod, server health; dùng IndexNow cho Bing nếu phù hợp.

### Audit procedure
Join crawl data với logs → xem last crawl/frequency theo nhóm URL.

### Suggested tools / data sources
Server logs; Screaming Frog; GSC

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-008 — Có response không nhất quán theo thời gian/bot không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Một URL không thường xuyên đổi 200↔5xx/403/redirect ngoài chủ đích.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Sửa load balancer/CDN/app/rate limit.

### Audit procedure
Group log theo URL → tìm nhiều status khác nhau → điều tra theo thời điểm.

### Suggested tools / data sources
Server/CDN logs; monitoring/APM

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-009 — Sau khi fix có kiểm tra lại bằng crawl và URL Inspection không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Lỗi đã hết ở môi trường production; bot thấy đúng phiên bản mới.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Re-crawl và retest URL mẫu; không chỉ kiểm tra bằng browser.

### Audit procedure
Re-crawl affected URLs → URL Inspection live test → lưu bằng chứng.

### Suggested tools / data sources
Screaming Frog/Sitebulb; GSC; DevTools

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-010 — Có theo dõi thay đổi index/crawl sau deploy lớn không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Migration/template/JS/robots/canonical changes được theo dõi ít nhất trên GSC + crawl + logs phù hợp.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Thiết lập baseline trước deploy và so sánh sau deploy.

### Audit procedure
Lưu snapshot trước → deploy → crawl lại → so GSC/logs.

### Suggested tools / data sources
GSC; crawler; server logs; spreadsheet

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`

## OBS-011 — Có theo dõi referral/citation từ AI Search ở mức khả thi không?
**Lifecycle:** Measure  
**Automation:** Partial  
**Default severity:** P2  
**Evidence strength:** Confirmed technical behavior / Strong practice  
**Required data (minimum):** GSC/Bing/analytics/log data, timestamps, user-agent/IP, deployment baseline  

### Standard / expected state
Theo dõi referral traffic và các citation/mention mẫu; không dùng một chỉ số duy nhất làm 'AI ranking'.

### Failure or review condition
Return `WARNING`, `UNKNOWN`, or `MANUAL_REVIEW` when required external/contextual evidence is missing. Return `FAIL` only when the collected evidence clearly violates the standard above.

### Recommended fix
Tạo nhóm nguồn/referrer và bộ prompt/citation sample theo thời gian nếu dự án cần.

### Audit procedure
Kiểm tra GA4 referrals → lưu citation samples → so sánh định kỳ.

### Suggested tools / data sources
GA4; manual prompt set; spreadsheet

### Evidence to store
- rule ID;
- affected URL or site-level target;
- observed value/state;
- expected value/state;
- acquisition source/tool;
- timestamp;
- supporting URL/sample where relevant.

### Notes / exceptions
Dùng như observability, không coi là dữ liệu xếp hạng trực tiếp.

### Source references
- `SRC-INTERNAL-CHECKLIST-001`
