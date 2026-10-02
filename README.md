# Site Crawl — mã nguồn tham khảo

Trích nguyên văn từ app desktop **1Scout Marketing** (commit `2e2cbff`, ngày 2026-09-25).
Đây là tính năng crawl toàn bộ một website rồi báo lỗi SEO, tương tự Screaming Frog: mã trạng thái,
title/meta, heading, canonical, hreflang, đồ thị liên kết nội bộ, trang trùng lặp, trang mồ côi,
PageSpeed.

> **Bản này để ĐỌC, không build riêng được.** Code vẫn import vài gói nội bộ khác của app
> (xem mục "Phụ thuộc"). Mọi file vẫn giữ nguyên tên gốc, nên có thể grep và đối chiếu với app.

## Nguồn gốc

Lõi crawl là bản port sang Go của **LibreCrawl**
(giấy phép MIT, Python/Flask): danh sách trường, luật phát hiện lỗi và các ngưỡng lấy từ đó làm
đặc tả. Bản port bổ sung những phần LibreCrawl thiếu mà Screaming Frog có: chuỗi redirect, trùng
title/meta/H1, `rel=nofollow`, trang mồ côi, hreflang hai chiều, chuỗi canonical. Chia sẻ lại thì giữ
dòng ghi công này.

## Cấu trúc thư mục

Trong app, cả 55 file Go nằm phẳng trong **một** package `sitecrawl`
(`desktop/internal/tools/sitecrawl/`). Ở đây chúng được chia nhóm **chỉ để dễ đọc**. Mọi file vẫn là
`package sitecrawl` và gọi thẳng hàm của nhau qua các nhóm.

```
sitecrawl-docs/
├── README.md
├── go/
│   ├── engine/      15 file · ~5.000 dòng  bộ máy crawl, KHÔNG đụng database
│   ├── storage/      9 file · ~3.900 dòng  điều phối + ghi/đọc SQLite
│   ├── pagespeed/    3 file · ~1.050 dòng  đo PageSpeed Insights (Google API)
│   ├── app-glue/     1 file ·    620 dòng  service.go, chỗ nối vào app (Wails, license, kho key)
│   ├── tests/       27 file · ~5.000 dòng  test của package
│   └── deps/         safe/ + httpx/        2 gói tiện ích nhỏ của app mà engine dùng
└── ui/
    ├── site-crawl/  19 file · ~5.000 dòng  màn hình Site Crawl (React 19 + TypeScript)
    ├── bindings/    code TS do Wails tự sinh từ Go (kiểu dữ liệu + hàm gọi sang Go)
    └── i18n-en-siteCrawl.json              toàn bộ chữ hiển thị trên màn hình, kể cả tên và mô tả lỗi
```

## Luồng một lần crawl

```
Service.Start(seeds, opts)                                   app-glue/service.go
  └─ coordinator.run                                         storage/crawler.go
       ├─ prepare
       │    ├─ robots.txt: đọc Crawl-delay và các dòng Sitemap:    engine/robots.go
       │    ├─ đưa URL gốc vào frontier (độ sâu 0)                 engine/frontier.go
       │    └─ tìm sitemap → đưa URL cùng site vào ở độ sâu 0      engine/sitemap.go
       ├─ psiPump.Start (chạy song song, nếu bật PageSpeed)        pagespeed/pagespeed_pump.go
       ├─ loop: MỘT goroutine điều phối + N worker (Concurrency)
       │    ├─ frontier.peekReady(hostGate)  chọn URL mà host đã sẵn sàng   engine/politeness.go
       │    ├─ worker: doOne
       │    │    ├─ robots.Check → bị chặn thì ghi nhận, không tải
       │    │    ├─ fetcher.fetch (theo redirect, retry 1 lần)     engine/fetch.go
       │    │    └─ maybeRender bằng Chrome/Edge có sẵn trên máy   engine/render.go
       │    └─ điều phối: absorb (chỉ goroutine này được ghi)
       │         ├─ 429/503 → xếp lại cuối hàng, thử lại một lần
       │         ├─ buildPage + extractDocument                    engine/page.go, extract.go
       │         ├─ collectLinks → admit link mới vào frontier     engine/links.go
       │         ├─ evaluate → danh sách lỗi SEO của trang         engine/issues.go
       │         └─ gom vào buffer → flush theo lô vào SQLite      storage/persist.go
       │    (checkpoint định kỳ lưu frontier để Pause/Resume sống qua lần tắt app)
       └─ finalize: các luật cần toàn bộ lần crawl                 storage/finalize.go
            inlinks · trang mồ côi · nguồn redirect · ảnh hỏng ·
            chuỗi canonical · hreflang hai chiều · trùng lặp       storage/duplicates.go
```

## Những quyết định thiết kế đáng đọc

Chú thích trong code giải thích lý do khá kỹ. Đây là những chỗ nên đọc trước:

| Chủ đề | Ở đâu | Ý chính |
|---|---|---|
| Một goroutine ghi duy nhất | `storage/crawler.go` (`loop`, `absorb`) | Mỗi workspace SQLite chỉ có một kết nối. Worker chỉ tải và phân tích, mọi thay đổi frontier và mọi lệnh ghi DB đi qua goroutine điều phối, nên không cần lock |
| Frontier BFS | `engine/frontier.go` | Slice làm hàng FIFO nên tự ra thứ tự theo độ sâu. Map `seen` vừa là tập đã gặp vừa cấp id URL, không phải hỏi SQL |
| Lịch sự với host | `engine/politeness.go` | Nhịp độ riêng từng host, tự lùi khi gặp 429/503/lỗi, tôn trọng Crawl-delay (có giới hạn trần) |
| Không coi 429/503 là lỗi trang | `storage/crawler.go` (`shouldDefer`) | Xếp lại và thử cuối lần crawl. Trước đây crawl thật báo nhầm khoảng 100 trang hỏng, và mất mọi trang chỉ tới được qua chúng |
| Chuẩn hoá URL | `engine/normalize.go` | Bỏ tham số tracking (`utm_*`…) trước khi thành khoá, nếu không một site 500 trang có thể phình thành 5.000 |
| URL từ sitemap vào ở độ sâu 0 | `storage/crawler.go` (`prepare`) | Nhờ vậy mới phát hiện được trang mồ côi, đổi lại là sitemap vượt qua giới hạn MaxDepth |
| Render JavaScript | `engine/render.go` | Không kèm Chromium mà mượn Chrome/Edge đã cài, chạy với `--user-data-dir` riêng. Có giới hạn số trang được render |
| Trùng lặp | `storage/duplicates.go`, `engine/similarity.go` | Bước 1: trùng tuyệt đối title/meta/H1 bằng một lượt quét map. Bước 2: gần trùng bằng SimHash để lọc cặp ứng viên, rồi tỷ lệ Ratcliff/Obershelp (port đúng `difflib.ratio()` của Python) với ngưỡng 0.85 |
| Finalize chạy lại được | `storage/finalize.go` | Mỗi luật xoá mã lỗi của chính nó rồi tính lại. Không dùng transaction, nên tính idempotent là cách duy nhất để hồi phục sau khi app sập |
| Schema | `storage/runs.go` (`schemaStmts`) | Cột trên bảng được "nâng" từ blob JSON ra nhiều hơn các tool khác, vì bảng kết quả chính là sản phẩm. Lỗi nằm ở bảng riêng để GROUP BY và lọc có index |
| PageSpeed | `pagespeed/pagespeed_pump.go` | Hàng đợi lấy từ SQL ("trang nào chưa đo"), cùng một câu truy vấn phục vụ cả crawl đang chạy, crawl tiếp sau Pause và lần đo bổ sung. PSI cố ý **không** đi qua proxy crawl |

## Tuỳ chọn crawl (`engine/types.go` → `Options`)

Mặc định lấy từ `DefaultOptions()` trong `app-glue/service.go`:

| Nhóm | Trường | Mặc định |
|---|---|---|
| Phạm vi | `Mode` (spider = đi theo link / list = chỉ crawl danh sách đưa vào) · `MaxDepth` · `MaxURLs` | spider · 3 · 50.000 |
| Tốc độ | `Concurrency` · `CrawlDelayMs` · `TimeoutSec` · `Retries` | 5 · 0 · 10s · 1 |
| Theo dõi | `FollowRedirects` · `CrawlExternal` · `CrawlSubdomains` · `IgnoreQueryParam` | bật · bật · tắt · tắt |
| Tài nguyên | `CrawlImages` · `CrawlCSS` · `CrawlJS` · `MaxFileSizeMB` | bật · bật · bật · 50 |
| Robots | `RespectRobots` · `RespectCrawlDelay` · `DiscoverSitemaps` | bật · tắt · bật |
| Lọc URL | `Include/ExcludeExtensions` · `Include/ExcludePatterns` (regex) | trống |
| Lọc lỗi | `IssueExclusions` · `UseDefaultExcl` (bỏ qua /wp-admin, checkout…) | trống · bật |
| Trùng lặp | `EnableDuplication` · `DuplicationThreshold` | bật · 0.85 |
| JavaScript | `EnableJavaScript` · `JSMaxPages` · `JSWaitMs` · `JSTimeoutSec` · viewport · `JSConcurrency` | tắt · 500 · 3000ms · 30s · 1920×1080 · 3 |
| Mạng | `UserAgent` (preset trong `engine/useragents.go`) · `AcceptLanguage` · `CustomHeaders` · `UseProxy` · `IgnoreSSL` | UA riêng của app, khai báo trung thực |
| PageSpeed | `EnablePageSpeed` · `PSIStrategy` | tắt · mobile |

## Mã lỗi SEO (`engine/issues.go`)

Khoảng 55 mã dạng kebab-case, mỗi mã có mức độ (critical / warning / …) và nhóm. Mã cũng là khoá dịch,
nên tên và mô tả hiển thị cho người dùng nằm trong `ui/i18n-en-siteCrawl.json`:

`title-missing` `title-long` `title-short` `title-multiple` `title-duplicate` ·
`meta-missing` `meta-long` `meta-short` `meta-multiple` `meta-duplicate` ·
`thin-content` `low-text-ratio` `duplicate-page` `broken-image` ·
`dns-not-found` `connection-refused` `timeout` `ssl-error` `connection-error` `client-error` `server-error` ·
`redirect` `redirect-chain` `redirect-loop` `internal-redirect` `meta-refresh` ·
`canonical-missing` `canonical-other` `canonical-multiple` `canonical-chain` `canonical-loop`
`canonical-to-non-200` `canonical-to-noindex` ·
`viewport-missing` `lang-missing` `images-no-alt` `image-alt-long` `og-missing` `twitter-missing`
`no-structured-data` · `slow-response` `moderate-response` `large-page` `moderate-page` ·
`noindex` `nofollow` `robots-blocked` `robots-unknown` `bot-blocked` `orphan-page` ·
`hreflang-no-return-tag` `hreflang-missing-self` `hreflang-invalid-code` `hreflang-to-non-200`
`hreflang-non-canonical`

## Từng file

**engine/**: không import `database/sql`, đây là phần dễ tái sử dụng nhất
- `types.go`: `Options`, `Page`, các hằng mode/trạng thái/sự kiện, giá trị mặc định
- `useragents.go`: 6 preset User-Agent: của app, Googlebot, Googlebot smartphone, Bingbot, Chrome, Chrome mobile
- `normalize.go`: chuẩn hoá URL, khoá frontier, cùng site/tên miền gốc, bỏ tham số tracking
- `frontier.go`: hàng đợi BFS + từ điển URL→id, lọc theo đuôi file và regex, snapshot/restore
- `politeness.go`: `hostGate`, nhịp độ từng host, tự lùi khi host quá tải
- `robots.go`: cache robots.txt mỗi run, Check / Crawl-delay / Sitemap
- `sitemap.go`: tìm và đọc sitemap (kể cả sitemap index), thử các đường dẫn phổ biến
- `fetch.go`: HTTP client, theo chuỗi redirect, retry, phân loại lỗi mạng, giới hạn kích thước
- `render.go`: tìm Chrome/Edge, render bằng `chromedp`, giới hạn số trang render
- `extract.go`: tokenizer HTML một lượt: meta, heading, link, ảnh, hreflang, schema (JSON-LD + microdata), analytics
- `page.go`: gộp kết quả tải + tài liệu đã phân tích thành bản ghi `Page`
- `links.go`: cạnh liên kết + cờ `rel` (nofollow/ugc/sponsored), URL canonical/hreflang/phân trang
- `issues.go`: bảng mã lỗi, `evaluate()` luật theo từng trang, tính indexable
- `exclusions.go`: các đường dẫn không báo lỗi (glob), bộ mặc định lấy từ LibreCrawl
- `similarity.go`: `seqRatio` (port difflib), công thức tương đồng nội dung, SimHash 64-bit

**storage/**
- `crawler.go`: `coordinator` (prepare, loop, doOne, absorb, flush, checkpoint, pause), tim của tính năng
- `finalize.go`: các luật chạy sau khi crawl xong
- `persist.go`: ghi theo lô (URL, page, link, issue), giữ dưới giới hạn biến bind của SQLite
- `runs.go`: schema bảng, vòng đời run, lưu/khôi phục frontier, dọn run bị treo
- `query.go`: dữ liệu cho bảng kết quả: tab, bộ lọc, tìm kiếm FTS, sắp xếp an toàn, inlinks/outlinks
- `duplicates.go`: nhóm trùng tuyệt đối + gần trùng
- `graph.go`: dữ liệu tab Visualization (top N trang theo inlinks + cạnh giữa chúng)
- `export.go`: xuất CSV/JSON/XML đúng những gì đang hiện trên màn hình
- `agent_local.go`: truy vấn chỉ đọc cho AI Writer ("trang nào trên site đã nói về X")

**pagespeed/**
- `pagespeed.go`: gọi PSI API, lưu kết quả, giới hạn đồng thời theo hạn mức Google
- `pagespeed_pump.go`: bơm đo song song với crawl, hàng đợi lấy từ SQL
- `pagespeed_opps.go`: tab Opportunities, gom audit theo toàn site

**app-glue/service.go**: các hàm Wails gọi từ UI (`Start`, `Pause`, `Resume`, `Stop`, `Recrawl`,
`Status`, `Runs`, `DeleteRun`…), lấy workspace/DB, proxy, kho key, kiểm license, hàng đợi job.

## Phụ thuộc

- Thư viện ngoài: `golang.org/x/net/html` · `github.com/temoto/robotstxt` · `github.com/chromedp/chromedp`
  · `github.com/wailsapp/wails/v3` (chỉ trong `service.go`)
- Gói nội bộ **có kèm** trong `go/deps/`: `core/safe` (chặn panic của một goroutine làm sập app) ·
  `core/httpx` (HTTP client dùng chung, hook proxy)
- Gói nội bộ **không kèm**, chỉ `storage/`, `pagespeed/`, `app-glue/` dùng:
  `core/runs` (lịch sử run chung các tool) · `core/workspace` (mỗi workspace một file SQLite) ·
  `core/schema` (chạy DDL và nâng cấp schema) · `core/credset` (danh sách key của provider) ·
  `core/jobs` (chạy việc dài ở nền, gửi tiến độ qua event Wails) · `core/license` · `tools` (registry)

Muốn dùng lại engine ở nơi khác: lấy `engine/` + `deps/`, sửa đường dẫn import, rồi viết một
coordinator nhỏ thay cho `storage/crawler.go` (bỏ phần SQLite, giữ vòng loop/absorb).

## Test (`go/tests/`)

Test nằm cùng package, nên đọc test là cách nhanh nhất để thấy hành vi thật. Hầu hết dựng một
`httptest.Server` làm site giả. Tên file cho biết phủ phần nào: `frontier_*`, `politeness_*`,
`sitemap_*`, `extract_*`, `render_*`, `similarity_*`, `pause_*` / `resume_ids_*` / `cancel_*`,
`deferred_*` (429/503), `seedredirect_*`, `schema_golden_*`, `perf_*`…

`live_test.go` (crawl một site thật, `CRAWL_LIVE=<url>`) và `psi_live_test.go` (gọi Google thật,
`PSI_LIVE_KEY=<key>`) tự bỏ qua (`t.Skip`) khi chưa đặt biến môi trường.

## Giao diện (`ui/`)

React 19 + TypeScript. Component `@/components/ui/*` là shadcn/Base UI dùng chung của app, **không kèm**.

- `page.tsx`: màn hình chính, ghép mọi phần bên dưới
- `crawl-bar.tsx` · `status-strip.tsx`: ô URL + nút chạy/tạm dừng/dừng, dòng trạng thái
- `config-dialog.tsx` · `options-store.ts`: hộp cấu hình crawl
- `tab-strip.tsx` · `columns.ts` · `cells.tsx`: 17 tab kiểu Screaming Frog, định nghĩa cột và ô
- `use-crawl-grid.ts`: cache hàng theo cửa sổ cho bảng lớn
- `grid-toolbar.tsx`: tìm kiếm, bộ lọc, xuất file
- `detail-pane.tsx` · `splitter.tsx`: khung chi tiết phía dưới, thanh kéo
- `overview-panel.tsx`: cây tổng quan bên phải (số lượng theo tab/lỗi)
- `pagespeed-panel.tsx` · `opportunities-panel.tsx`: tab PageSpeed và Opportunities
- `visualization-panel.tsx`: đồ thị liên kết vẽ bằng canvas + `d3-force`
- `runs-page.tsx`: lịch sử các lần crawl
- `use-site-crawl.ts`: nhận event từ Go: `sitecrawl:progress`, `run-state`, `render-state`, `psi-progress`, `psi-result`
- `layout-store.ts`: nhớ độ rộng cột và bố cục

`ui/bindings/` là code do `wails3 generate bindings` sinh ra từ `service.go`/`types.go`: kiểu dữ liệu
TS và hàm gọi sang Go. Đọc file này để biết UI nói chuyện với Go qua những hàm nào.
