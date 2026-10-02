package sitecrawl

import (
	"context"
	"database/sql"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"path/filepath"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

// localCountingTransport đếm và từ chối mọi request: LookupLocal không được phép
// ra mạng, kể cả khi có ai "trả lời hộ".
type localCountingTransport struct{ n *atomic.Int64 }

func (t localCountingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.n.Add(1)
	return nil, errors.New("LookupLocal đã gọi mạng: " + r.URL.String())
}

func forbidLocalNetwork(t *testing.T) *atomic.Int64 {
	t.Helper()
	n := &atomic.Int64{}
	old := http.DefaultTransport
	http.DefaultTransport = localCountingTransport{n: n}
	t.Cleanup(func() {
		http.DefaultTransport = old
		if got := n.Load(); got != 0 {
			t.Errorf("LookupLocal gửi %d request HTTP", got)
		}
	})
	return n
}

func localDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "w.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	if err := ensureSchema(db); err != nil {
		t.Fatal(err)
	}
	return db
}

type localSeed struct {
	run, url, title, meta, h1, h2 string
	internal, indexable, status   int
	inlinks, words                int
}

func seedLocalCrawl(t *testing.T, db *sql.DB) {
	t.Helper()
	for _, r := range []struct{ id, seed, host string }{
		{"run-abc", "https://www.abc.com/", "www.abc.com"},
		{"run-old", "https://abc.com/", "abc.com"},
		{"run-other", "https://other.com/", "other.com"},
	} {
		if err := insertRun(db, r.id, r.seed, r.host, Options{}); err != nil {
			t.Fatalf("insertRun: %v", err)
		}
	}
	pages := []localSeed{
		{"run-abc", "https://www.abc.com/shoes", "Red shoes guide", "All about shoes", "Shoes", "Sizes", 1, 1, 200, 5, 900},
		{"run-abc", "https://www.abc.com/boots", "Boots", "Winter shoes too", "Boots", "", 1, 1, 200, 9, 700},
		{"run-abc", "https://www.abc.com/100-percent", "100% cotton shoes", "", "", "", 1, 1, 200, 1, 300},
		{"run-abc", "https://www.abc.com/snake_case", "shoe_care tips", "", "", "", 1, 1, 200, 1, 300},
		{"run-abc", "https://www.abc.com/shoe-care", "shoeXcare tips", "", "", "", 1, 1, 200, 1, 300},
		{"run-abc", "https://www.abc.com/old-shoes", "Old shoes", "", "", "", 1, 1, 301, 1, 0},
		{"run-abc", "https://www.abc.com/missing-shoes", "Missing shoes", "", "", "", 1, 1, 404, 1, 0},
		{"run-abc", "https://www.abc.com/noindex-shoes", "Noindex shoes", "", "", "", 1, 0, 200, 1, 400},
		{"run-abc", "https://ext.example.org/shoes", "External shoes", "", "", "", 0, 1, 200, 1, 400},
		{"run-abc", "https://blog.abc.com/shoes", "Blog shoes", "", "", "", 1, 1, 200, 1, 400},
		{"run-old", "https://abc.com/old-run-shoes", "Shoes from an older crawl", "", "", "", 1, 1, 200, 50, 400},
		{"run-other", "https://other.com/shoes", "Other site shoes", "", "", "", 1, 1, 200, 50, 400},
	}
	for i, p := range pages {
		if _, err := db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, is_internal, status,
			title, meta_desc, h1, h2, word_count, indexable, inlinks, crawled_at)
			VALUES(?,?,?,'{}',?,?,?,?,?,?,?,?,?,'2026-09-01T00:00:00Z')`,
			p.run, i+1, p.url, p.internal, p.status, p.title, p.meta, p.h1, p.h2, p.words, p.indexable, p.inlinks); err != nil {
			t.Fatalf("seed page: %v", err)
		}
	}
}

func localURLs(pages []LocalPage) []string {
	out := make([]string, len(pages))
	for i, p := range pages {
		out[i] = p.URL
	}
	return out
}

// Đúng run, đúng host, chỉ trang nội bộ + index được + 2xx, nhiều inlink trước.
func TestLookupLocalRightRunAndHost(t *testing.T) {
	forbidLocalNetwork(t)
	db := localDB(t)
	seedLocalCrawl(t, db)
	ctx := context.Background()

	got, err := LookupLocal(ctx, db, "run-abc", "https://abc.com", "shoes", 0)
	if err != nil {
		t.Fatalf("LookupLocal: %v", err)
	}
	want := []string{"https://www.abc.com/boots", "https://www.abc.com/shoes", "https://www.abc.com/100-percent"}
	if urls := localURLs(got); len(urls) != len(want) || urls[0] != want[0] || urls[1] != want[1] || urls[2] != want[2] {
		t.Fatalf("muốn %v, có %v", want, urls)
	}
	if p := got[1]; p.Title != "Red shoes guide" || p.MetaDescription != "All about shoes" ||
		p.H1 != "Shoes" || p.H2 != "Sizes" || p.WordCount != 900 || p.CrawledAt == "" {
		t.Fatalf("trường sai: %+v", p)
	}

	got, err = LookupLocal(ctx, db, "run-abc", "abc.com", "SIZES", 1)
	if err != nil || len(got) != 1 || got[0].URL != "https://www.abc.com/shoes" {
		t.Fatalf("khớp H2 không phân biệt hoa thường, limit 1: %v, %v", localURLs(got), err)
	}

	for name, c := range map[string]struct{ run, site string }{
		"run crawl website khác": {"run-other", "https://abc.com"},
		"run không tồn tại":      {"run-nope", "https://abc.com"},
		"dự án chưa chọn run":    {"", "https://abc.com"},
		"dự án thiếu website":    {"run-abc", ""},
	} {
		if _, err := LookupLocal(ctx, db, c.run, c.site, "shoes", 5); !errors.Is(err, ErrLocalUnavailable) {
			t.Fatalf("%s: muốn ErrLocalUnavailable, có %v", name, err)
		}
	}

	got, err = LookupLocal(ctx, db, "run-abc", "https://abc.com", "no such thing", 5)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("không trang nào khớp phải là rỗng không lỗi: %v, %v", got, err)
	}
	if _, err := LookupLocal(ctx, db, "run-abc", "https://abc.com", "   ", 5); err == nil || errors.Is(err, ErrLocalUnavailable) {
		t.Fatalf("text rỗng phải là lỗi đầu vào, có %v", err)
	}
}

func TestLookupLocalLikeIsLiteral(t *testing.T) {
	forbidLocalNetwork(t)
	db := localDB(t)
	seedLocalCrawl(t, db)
	ctx := context.Background()

	got, err := LookupLocal(ctx, db, "run-abc", "https://abc.com", "shoe_care", 20)
	if err != nil {
		t.Fatalf("LookupLocal: %v", err)
	}
	if urls := localURLs(got); len(urls) != 1 || urls[0] != "https://www.abc.com/snake_case" {
		t.Fatalf("`_` phải là ký tự thường: %v", urls)
	}

	got, err = LookupLocal(ctx, db, "run-abc", "https://abc.com", "%", 20)
	if err != nil {
		t.Fatalf("LookupLocal: %v", err)
	}
	if urls := localURLs(got); len(urls) != 1 || urls[0] != "https://www.abc.com/100-percent" {
		t.Fatalf("`%%` phải là ký tự thường: %v", urls)
	}
}

func TestLookupLocalNoSchema(t *testing.T) {
	forbidLocalNetwork(t)
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "bare.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := LookupLocal(context.Background(), db, "run-abc", "https://abc.com", "x", 5); !errors.Is(err, ErrLocalUnavailable) {
		t.Fatalf("muốn ErrLocalUnavailable, có %v", err)
	}
}

func TestLookupLocalCancelled(t *testing.T) {
	forbidLocalNetwork(t)
	db := localDB(t)
	seedLocalCrawl(t, db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LookupLocal(ctx, db, "run-abc", "https://abc.com", "shoes", 5); err == nil {
		t.Fatal("context đã huỷ mà vẫn trả kết quả")
	}
}

// Chốt tĩnh: transport giả không bắt được client riêng mà crawler dựng qua
// httpx, nên agent_local.go không được nhắc tới đường nào dẫn ra mạng.
func TestLookupLocalNoNetworkSymbols(t *testing.T) {
	forbidden := map[string]bool{
		"Service": true, "http": true, "httpx": true, "Jobs": true, "newFetcher": true,
		"fetcher": true, "runCrawl": true, "coordinator": true, "Start": true, "Resume": true,
		"crawler": true, "newCrawler": true, "psiClient": true, "robots": true,
	}
	f, err := parser.ParseFile(token.NewFileSet(), "agent_local.go", nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && forbidden[id.Name] {
			t.Errorf("agent_local.go nhắc tới %q — đường đó có thể ra mạng", id.Name)
		}
		return true
	})
	for _, imp := range f.Imports {
		switch imp.Path.Value {
		case `"net/http"`, `"onescout/desktop/internal/core/httpx"`:
			t.Errorf("agent_local.go import %s", imp.Path.Value)
		}
	}
}

// Ảnh/PDF/CSS/JS trả 200 cũng có indexable=1; chúng không phải trang để dẫn link
// và không được chiếm chỗ trong giới hạn kết quả.
func TestLookupLocalOnlyHTML(t *testing.T) {
	forbidLocalNetwork(t)
	db := localDB(t)
	seedLocalCrawl(t, db)
	for i, r := range []struct{ kind, url string }{
		{"image", "https://www.abc.com/img/shoes.jpg"},
		{"pdf", "https://www.abc.com/shoes-catalog.pdf"},
		{"css", "https://www.abc.com/shoes.css"},
		{"js", "https://www.abc.com/shoes.js"},
	} {
		if _, err := db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, is_internal, status,
			indexable, inlinks, crawled_at) VALUES('run-abc',?,?,'{}',?,1,200,1,100,'2026-09-01T00:00:00Z')`,
			1000+i, r.url, r.kind); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	got, err := LookupLocal(context.Background(), db, "run-abc", "https://abc.com", "shoes", 3)
	if err != nil {
		t.Fatalf("LookupLocal: %v", err)
	}
	want := []string{"https://www.abc.com/boots", "https://www.abc.com/shoes", "https://www.abc.com/100-percent"}
	if urls := localURLs(got); len(urls) != 3 || urls[0] != want[0] || urls[1] != want[1] || urls[2] != want[2] {
		t.Fatalf("chỉ trang HTML: muốn %v, có %v", want, urls)
	}
}

// SQLite LIKE chỉ gập hoa thường ASCII; tiêu đề tiếng Việt phải khớp dù người
// dùng gõ khác hoa thường.
func TestLookupLocalUnicodeCaseInsensitive(t *testing.T) {
	forbidLocalNetwork(t)
	db := localDB(t)
	seedLocalCrawl(t, db)
	if _, err := db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, is_internal, status,
		h1, indexable, inlinks, crawled_at) VALUES('run-abc',2000,'https://www.abc.com/tour','{}',1,200,
		'Đặt tour Đà Nẵng',1,3,'2026-09-01T00:00:00Z')`); err != nil {
		t.Fatalf("seed: %v", err)
	}
	for _, q := range []string{"đà nẵng", "ĐÀ NẴNG", "Đà Nẵng"} {
		got, err := LookupLocal(context.Background(), db, "run-abc", "https://abc.com", q, 5)
		if err != nil {
			t.Fatalf("LookupLocal(%q): %v", q, err)
		}
		if urls := localURLs(got); len(urls) != 1 || urls[0] != "https://www.abc.com/tour" {
			t.Fatalf("%q phải khớp H1 tiếng Việt: %v", q, urls)
		}
	}
}
