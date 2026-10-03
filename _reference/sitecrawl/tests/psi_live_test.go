package sitecrawl

import (
	"context"
	"os"
	"testing"
	"time"
)

// Bài kiểm tra chạy thẳng vào PageSpeed Insights thật.
//
// VÌ SAO CÓ FILE NÀY: pagespeed_test.go kiểm toàn bộ nhánh PSI bằng một
// httptest.Server — máy chủ giả trả về đúng khuôn JSON mà người viết test đã
// tự gõ ra. Nó vì thế xác nhận được rằng code đọc đúng cái khuôn ấy, và không
// xác nhận được điều duy nhất đáng lo: Google có thật sự nhận request mình gửi,
// và có thật sự trả về những tên trường mình đang bóc hay không.
//
// Ba phép đoán về luật của Google đang nằm trong pagespeed.go mà chưa lần nào
// được hỏi thật:
//   - tham số `category` phải LẶP bốn lần, không nối bằng dấu phẩy (dòng 305)
//   - key đi ở header `X-goog-api-key`, không ở query string (dòng 319)
//   - id hạng mục có dấu gạch ngang (`best-practices`), tên audit và tên chỉ số
//     CrUX (`INTERACTION_TO_NEXT_PAINT`, `EXPERIMENTAL_TIME_TO_FIRST_BYTE`…)
//     vẫn còn đúng như thế
//
// Đoán sai bất kỳ điều nào ở trên thì trên màn hình Site Crawl, tab PageSpeed
// hiện ra một bảng đầy dấu gạch: mọi cột về -1 "chưa đo", không có lỗi đỏ nào,
// vì -1 là giá trị khởi tạo hợp lệ. Đúng kiểu hỏng câm của vụ Search Console.
//
// Chạy:
//
//	PSI_LIVE_KEY=<khoá PageSpeed> go test -tags nodynamic -run TestLivePSI -v \
//	  -timeout 300s ./internal/tools/sitecrawl/
//
// PageSpeed Insights MIỄN PHÍ (hạn mức 25.000 truy vấn/ngày), nên không có gate
// tốn-tiền ở đây. Mọi câu hỏi chỉ ĐỌC: đo một trang công khai, không ghi gì.
// Mỗi phép đo là một lượt Lighthouse thật, mất ~20 giây.
func psiLiveKey(t *testing.T) string {
	t.Helper()
	key := os.Getenv("PSI_LIVE_KEY")
	if key == "" {
		t.Skip("đặt PSI_LIVE_KEY=<khoá PageSpeed> để hỏi Google thật")
	}
	return key
}

// psiLiveURL là trang đem đi đo. Mặc định chọn một trang công khai, ổn định và
// đủ đông người truy cập để Google có sẵn dữ liệu CrUX cho nó — không có CrUX
// thì cả nửa số trường trong PSIResult không bao giờ được chạm tới.
func psiLiveURL() string {
	if u := os.Getenv("PSI_LIVE_URL"); u != "" {
		return u
	}
	return "https://www.wikipedia.org/"
}

// Cả hai chế độ app cho user chọn đều phải được Google nhận.
//
// Trên màn hình Site Crawl, tab PageSpeed có nút gạt Mobile/Desktop. Nếu Google
// đổi tên một trong hai giá trị, một nửa nút gạt đó chết.
func TestLivePSIAcceptsBothStrategies(t *testing.T) {
	key := psiLiveKey(t)
	url := psiLiveURL()

	for _, strategy := range []string{StrategyMobile, StrategyDesktop} {
		t.Run(strategy, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()

			got := fetchPSI(ctx, psiClient(), key, url, strategy)
			if got.Error != "" {
				t.Fatalf("Google từ chối chế độ %q: %s", strategy, got.Error)
			}
			if got.Strategy != strategy {
				t.Errorf("kết quả ghi chế độ %q, gửi đi %q", got.Strategy, strategy)
			}
			t.Logf("%s: performance=%d a11y=%d seo=%d bp=%d",
				strategy, got.Score, got.A11yScore, got.SEOScore, got.BPScore)
		})
	}
}

// Mọi trường app bóc ra phải thật sự có trong câu trả lời của Google.
//
// Đây là lưới an toàn cho họ lỗi "hỏng câm". Mỗi khẳng định dưới đây tương ứng
// với một cột trên tab PageSpeed: cột nào về -1 hoặc 0 nghĩa là tên trường bên
// Google đã đổi mà app không hay biết.
//
// Bốn điểm số cùng lúc còn chứng minh luôn phép đoán về `category` lặp bốn lần:
// nếu Google chỉ nhận một hạng mục, ba điểm còn lại sẽ về -1.
func TestLivePSIResponseStillCarriesEveryFieldWeParse(t *testing.T) {
	key := psiLiveKey(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	got := fetchPSI(ctx, psiClient(), key, psiLiveURL(), StrategyMobile)
	if got.Error != "" {
		t.Fatalf("đo thất bại: %s", got.Error)
	}

	// Bốn hạng mục Lighthouse — id "best-practices" có dấu gạch ngang, đúng
	// chỗ dễ gõ sai nhất và cũng là chỗ máy chủ giả không bao giờ cãi lại.
	scores := map[string]int{
		"performance":    got.Score,
		"accessibility":  got.A11yScore,
		"seo":            got.SEOScore,
		"best-practices": got.BPScore,
	}
	for id, v := range scores {
		if v < 0 {
			t.Errorf("hạng mục %q không có điểm (=-1): hoặc Google không nhận đủ bốn tham số category, "+
				"hoặc id hạng mục đã đổi. Cột này trên tab PageSpeed sẽ hiện dấu gạch.", id)
		}
	}

	// Năm chỉ số phòng thí nghiệm, lấy theo id audit. Về 0 nghĩa là id đã đổi:
	// một trang thật không thể có FCP hay LCP bằng 0.
	labs := map[string]int{
		"first-contentful-paint":   got.FCPMs,
		"largest-contentful-paint": got.LCPMs,
		"total-blocking-time":      got.TBTMs,
		"speed-index":              got.SIMs,
	}
	for id, v := range labs {
		if v <= 0 {
			t.Errorf("audit %q trả %d — id audit này có thể đã đổi tên bên Google", id, v)
		}
	}

	// CrUX là dữ liệu người dùng Chrome thật trong 28 ngày. Trang mặc định đủ
	// đông để có; nếu user trỏ PSI_LIVE_URL vào trang vắng thì đây là SKIP chứ
	// không phải FAIL — vắng khách là sự thật về trang đó, không phải lỗi app.
	if got.CruxSource == "" {
		t.Skipf("trang %s chưa đủ lượt truy cập để Google có dữ liệu CrUX — "+
			"đặt PSI_LIVE_URL sang một trang đông hơn để kiểm nốt nhánh này", psiLiveURL())
	}
	if got.CruxSource != "url" && got.CruxSource != "origin" {
		t.Errorf("cruxSource = %q, chỉ được là \"url\" hoặc \"origin\" — "+
			"nhãn này là thứ ngăn app hiện số của cả tên miền như thể là số của riêng trang đó", got.CruxSource)
	}
	if got.CruxVerdict == "" {
		t.Error("cruxVerdict rỗng — trường overall_category có thể đã đổi tên")
	}
	fields := map[string]int{
		"LARGEST_CONTENTFUL_PAINT_MS":     got.CruxLCPMs,
		"INTERACTION_TO_NEXT_PAINT":       got.CruxINPMs,
		"FIRST_CONTENTFUL_PAINT_MS":       got.CruxFCPMs,
		"EXPERIMENTAL_TIME_TO_FIRST_BYTE": got.CruxTTFBMs,
	}
	for name, v := range fields {
		if v < 0 {
			t.Errorf("chỉ số CrUX %q không về (=-1) — tên chỉ số này có thể đã đổi", name)
		}
	}
	if got.CruxCLS < 0 {
		t.Error("CUMULATIVE_LAYOUT_SHIFT_SCORE không về (=-1)")
	}
	t.Logf("CrUX (%s, %s): LCP=%dms INP=%dms CLS=%.2f FCP=%dms TTFB=%dms",
		got.CruxSource, got.CruxVerdict, got.CruxLCPMs, got.CruxINPMs, got.CruxCLS, got.CruxFCPMs, got.CruxTTFBMs)
}

// Key sai phải bị Google từ chối.
//
// Bài đối chứng cho hai bài trên: nếu PageSpeed cho gọi không cần key, cả hai
// bài kia vẫn xanh dù header `X-goog-api-key` bị gõ sai tên và key của user
// không hề được gửi đi. Khi đó user nhập key vào Connections cũng vô nghĩa, và
// app sẽ chạm trần hạn mức ẩn danh của Google mà không ai hiểu vì sao.
func TestLivePSIRejectsABadKey(t *testing.T) {
	psiLiveKey(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	got := fetchPSI(ctx, psiClient(), "khong-phai-mot-khoa-that", psiLiveURL(), StrategyMobile)
	if got.Error == "" {
		t.Fatal("Google nhận cả key rác — nghĩa là request đi mà KHÔNG mang key thật, " +
			"nên hai bài trên xanh vì lý do sai. Kiểm lại tên header ở pagespeed.go:319.")
	}
	t.Logf("key rác bị từ chối đúng như mong đợi: %s", got.Error)
}
