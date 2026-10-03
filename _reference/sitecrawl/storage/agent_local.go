package sitecrawl

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"strings"
)

// Accessor cục bộ cho chặng Research của AI Writer.
//
// Agent hỏi "trang nào trên site đã nói về X" để gợi ý liên kết nội bộ. Hàm chỉ
// cầm một *sql.DB, không cầm *Service, nên không có đường nào dẫn tới fetcher,
// PageSpeed hay job crawl: đọc lại một lần crawl đã lưu, không bao giờ crawl mới.

// LocalMaxLimit là số trang tối đa một lần hỏi trả về; limit <= 0 cũng lấy số này.
const LocalMaxLimit = 20

// ErrLocalUnavailable nghĩa là KHÔNG CÓ lần crawl nào để trả lời: workspace chưa
// từng mở Site Crawl, run đã bị xoá, hoặc run đó crawl một website khác. Khác hẳn
// kết quả rỗng không lỗi — crawl có đó, chỉ là không trang nào khớp.
var ErrLocalUnavailable = errors.New("sitecrawl: no stored crawl for this website")

// LocalPage là một trang đã crawl, đủ để agent quyết định có dẫn link tới không.
type LocalPage struct {
	URL, Title, MetaDescription, H1, H2, CrawledAt string
	WordCount                                      int
}

// localTables là những bảng accessor đọc. Thiếu bảng = tool chưa từng chạy.
var localTables = []string{"sitecrawl_runs", "sitecrawl_pages"}

// LookupLocal tìm các trang HTML nội bộ, index được, trả 2xx của đúng một lần
// crawl mà url, title, meta description, H1 hoặc H2 chứa text (không phân biệt
// hoa thường, kể cả chữ có dấu).
//
// websiteURL là website của dự án: run phải crawl đúng host đó (bỏ qua `www.`,
// cổng và hoa thường, như cách crawler coi là cùng site), và mỗi trang trả về
// cũng phải nằm trên host đó. `%` và `_` trong text là ký tự thường, không phải
// wildcard.
func LookupLocal(ctx context.Context, db *sql.DB, runID, websiteURL, text string, limit int) ([]LocalPage, error) {
	term := strings.TrimSpace(text)
	if term == "" {
		return nil, errors.New("sitecrawl: local lookup needs a text")
	}
	if limit <= 0 || limit > LocalMaxLimit {
		limit = LocalMaxLimit
	}
	runID = strings.TrimSpace(runID)
	want := localHost(websiteURL)
	if runID == "" || want == "" {
		return nil, ErrLocalUnavailable
	}

	for _, table := range localTables {
		ok, err := localTableExists(ctx, db, table)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrLocalUnavailable
		}
	}

	var runHost string
	err := db.QueryRowContext(ctx, `SELECT host FROM sitecrawl_runs WHERE id = ?`, runID).Scan(&runHost)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrLocalUnavailable
	}
	if err != nil {
		return nil, err
	}
	if registrableHost(hostnameOf("//"+runHost)) != want {
		return nil, ErrLocalUnavailable
	}

	// Lọc chữ ở phía Go chứ không bằng LIKE: LIKE của SQLite chỉ gập hoa thường
	// ASCII nên "đà nẵng" không khớp H1 "Đặt tour Đà Nẵng". Hàng được đọc lần lượt
	// theo thứ tự inlink và dừng ngay khi đủ limit, không nạp cả lần crawl vào bộ nhớ.
	rows, err := db.QueryContext(ctx, `
		SELECT url, title, meta_desc, h1, h2, crawled_at, word_count
		  FROM sitecrawl_pages
		 WHERE run_id = ? AND is_internal = 1 AND kind = 'html' AND indexable = 1
		   AND status BETWEEN 200 AND 299
		 ORDER BY inlinks DESC, url_id`,
		runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	needle := strings.ToLower(term)
	out := []LocalPage{}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var p LocalPage
		if err := rows.Scan(&p.URL, &p.Title, &p.MetaDescription, &p.H1, &p.H2,
			&p.CrawledAt, &p.WordCount); err != nil {
			return nil, err
		}
		// Crawl bật subdomain vẫn đánh dấu trang ở host khác là nội bộ; agent
		// chỉ được gợi ý trang trên chính website của dự án.
		if registrableHost(hostnameOf(p.URL)) != want {
			continue
		}
		if !localPageMatches(p, needle) {
			continue
		}
		out = append(out, p)
		if len(out) == limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// localHost rút host so sánh được từ website của dự án; website lưu có thể
// thiếu scheme.
func localHost(website string) string {
	w := strings.TrimSpace(website)
	if w == "" {
		return ""
	}
	if !strings.Contains(w, "://") {
		w = "https://" + w
	}
	u, err := url.Parse(w)
	if err != nil {
		return ""
	}
	return registrableHost(u.Hostname())
}

// localPageMatches báo url, title, meta description, H1 hoặc H2 có chứa needle
// (đã hạ chữ thường) hay không, gập hoa thường theo Unicode. So khớp chuỗi con
// thuần nên `%` và `_` là ký tự thường.
func localPageMatches(p LocalPage, needle string) bool {
	for _, f := range [...]string{p.URL, p.Title, p.MetaDescription, p.H1, p.H2} {
		if strings.Contains(strings.ToLower(f), needle) {
			return true
		}
	}
	return false
}

// localTableExists hỏi sqlite_master thay vì chạy truy vấn rồi đoán lỗi: chỉ đúng
// trường hợp "bảng chưa có" được coi là không có dữ liệu, mọi lỗi SQL khác nổi lên.
func localTableExists(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}
