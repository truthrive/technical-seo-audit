package sitecrawl

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Pins computed with CPython's difflib. The 0.85 threshold is LibreCrawl's
// number measured against difflib's ratio — if these drift, 0.85 changes
// meaning.
func TestSeqRatioMatchesDifflib(t *testing.T) {
	long := ""
	for i := 0; i < 250; i++ {
		long += "a"
	}
	cases := []struct {
		a, b string
		want float64
	}{
		{"abcd", "bcde", 0.75},
		{"abc", "abc", 1.0},
		{"", "", 1.0},
		{"abc", "", 0.0},
		{"kitten", "sitting", 0.6153846153846154},
		{"the quick brown fox", "the quick brown dog", 0.8947368421052632},
		{"trang chu nha cai alo789", "trang chu nha cai alo790", 0.9583333333333334},
		// b ≥ 200 chars of one popular rune: exercises difflib's autojunk path.
		{long + "xyz", long + "abc", 0.9881422924901185},
	}
	for _, c := range cases {
		if got := seqRatio(c.a, c.b); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("seqRatio(%.20q, %.20q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestContentSimilarityWeights(t *testing.T) {
	// Identical everything = 0.35 + 0.35 + 0.20 + 0.10.
	if got := contentSimilarity("t", "m", "h", 100, "t", "m", "h", 100); math.Abs(got-1.0) > 1e-9 {
		t.Errorf("identical pages = %v, want 1.0", got)
	}
	// Only titles match, word counts equal: 0.35 + 0.10. Empty fields score 0,
	// exactly as LibreCrawl's guard clauses do.
	if got := contentSimilarity("same title", "", "", 100, "same title", "", "", 100); math.Abs(got-0.45) > 1e-9 {
		t.Errorf("title-only match = %v, want 0.45", got)
	}
	// Word-count ratio is min/max.
	if got := contentSimilarity("", "", "", 50, "", "", "", 100); math.Abs(got-0.05) > 1e-9 {
		t.Errorf("half word count = %v, want 0.05", got)
	}
}

// TestNearDupBlockingRecall: every pair brute force finds above the threshold,
// the banded path must find too — blocking is allowed to test extra pairs,
// never to lose one.
func TestNearDupBlockingRecall(t *testing.T) {
	base := "huong dan ca cuoc bong da tai nha cai uy tin hang dau viet nam"
	metaBase := "bai viet huong dan chi tiet cach ca cuoc bong da an toan va hieu qua cho nguoi moi bat dau tim hieu"
	var rows []dupInput
	for i := 0; i < 40; i++ {
		r := dupInput{id: int64(i + 1), url: fmt.Sprintf("https://x.example/p%d", i), words: 400}
		switch i % 4 {
		case 0: // near-identical family: one token differs
			r.title = fmt.Sprintf("%s so %d", base, i%3)
			r.meta = metaBase
			r.h1 = base
		case 1: // same family, tiny spelling drift
			r.title = fmt.Sprintf("%s so %d", base, i%3)
			r.meta = metaBase + " nhe"
			r.h1 = base
		case 2: // unrelated
			r.title = fmt.Sprintf("chinh sach bao mat du lieu nguoi dung phien ban %d", i)
			r.meta = fmt.Sprintf("mo ta chinh sach bao mat va dieu khoan su dung so %d cua trang web nay danh cho khach", i)
			r.h1 = "chinh sach bao mat"
			r.words = 900 + i
		case 3: // another unrelated family
			r.title = fmt.Sprintf("lien he hop tac quang cao truyen thong kenh %d", i)
			r.meta = fmt.Sprintf("thong tin lien he hop tac kinh doanh va quang cao truyen thong tren kenh so %d", i)
			r.h1 = "lien he"
			r.words = 150
		}
		rows = append(rows, r)
	}

	const threshold = 0.85
	brute := map[[2]int]bool{}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			s := contentSimilarity(rows[i].title, rows[i].meta, rows[i].h1, rows[i].words,
				rows[j].title, rows[j].meta, rows[j].h1, rows[j].words)
			if s >= threshold {
				brute[[2]int{i, j}] = true
			}
		}
	}
	if len(brute) == 0 {
		t.Fatal("fixture produced no duplicate pairs; the test asserts nothing")
	}

	blocked := map[[2]int]bool{}
	for _, p := range nearDupPairs(rows, threshold) {
		blocked[[2]int{p.i, p.j}] = true
	}
	for pair := range brute {
		if !blocked[pair] {
			t.Errorf("brute force found pair %v (urls %s ~ %s) but blocking missed it",
				pair, rows[pair[0]].url, rows[pair[1]].url)
		}
	}
}

// TestNearDupDegenerateBudget: a site where every page shares one title makes a
// single giant bucket, and pairwise over it is the O(n²) work nearDupBucketCap
// exists to refuse.
//
// Asserted by RESULT, not by a stopwatch. The previous version timed 500 such
// pages and demanded under two seconds, which was wrong twice over: 500 is below
// the cap of 2000, so the cap it claimed to test never engaged — it was really
// timing 124,750 similarity scorings on whatever machine was running — and the
// bound broke the day CI ran it under the race detector. Over the cap the bucket
// is skipped outright, so "no pairs came out of it" says the same thing with no
// clock involved and no way for a slow machine to be wrong about it.
func TestNearDupDegenerateBudget(t *testing.T) {
	sameTitled := func(n int) []dupInput {
		var rows []dupInput
		for i := 0; i < n; i++ {
			rows = append(rows, dupInput{
				id: int64(i + 1), url: fmt.Sprintf("https://x.example/%d", i),
				title: "san pham", meta: "san pham cua chung toi", h1: "san pham", words: 50,
			})
		}
		return rows
	}

	// Control: a bucket under the cap still blocks and still reports, so a zero
	// above the cap means "refused", not "this pass never finds anything".
	if got := nearDupPairs(sameTitled(50), 0.85); len(got) == 0 {
		t.Fatal("50 identical pages produced no pairs — the blocking pass is broken, not capped")
	}

	if got := nearDupPairs(sameTitled(nearDupBucketCap+1), 0.85); len(got) != 0 {
		t.Errorf("%d pairs from a bucket of %d — the cap is not engaging, and this is the O(n²) blow-up it exists to refuse",
			len(got), nearDupBucketCap+1)
	}
}

// TestNearDuplicatePagesGetFlagged is the end-to-end pass: two pages that are
// near (not exact) copies come out with duplicate-page issues pointing at each
// other, and a genuinely different page stays clean.
func TestNearDuplicatePagesGetFlagged(t *testing.T) {
	fastCrawl(t)
	s, db, _ := newTestService(t)

	mkPage := func(title, meta, h1 string) string {
		body := ""
		for i := 0; i < 50; i++ {
			body += "<p>noi dung bai viet huong dan chi tiet tung buoc mot cho nguoi moi</p>"
		}
		return fmt.Sprintf(`<!doctype html><html lang="vi"><head><title>%s</title>
<meta name="description" content="%s"><meta name="viewport" content="width=device-width">
</head><body><h1>%s</h1>%s</body></html>`, title, meta, h1, body)
	}

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	title := "Huong dan ca cuoc bong da tai nha cai uy tin hang dau"
	meta := "Bai viet huong dan chi tiet cach ca cuoc bong da an toan va hieu qua cho nguoi moi bat dau."
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, mkPage("Trang chu website nay day", "Trang chu cua website voi day du thong tin can thiet cho khach truy cap moi.",
			`Trang chu</h1><p><a href="/near-a">A</a> <a href="/near-b">B</a> <a href="/khac">C</a></p><h1 hidden>`))
	})
	mux.HandleFunc("/near-a", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, mkPage(title+" 2026", meta, "Huong dan ca cuoc"))
	})
	mux.HandleFunc("/near-b", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, mkPage(title+" 2025", meta+" Cap nhat.", "Huong dan ca cuoc"))
	})
	mux.HandleFunc("/khac", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, mkPage("Chinh sach bao mat du lieu nguoi dung", "Mo ta chinh sach bao mat va dieu khoan su dung cua trang web nay danh cho khach.", "Chinh sach"))
	})

	opts := s.DefaultOptions()
	opts.Concurrency = 2
	opts.RespectRobots = false
	opts.DiscoverSitemaps = false
	started, err := s.Start([]string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitForRun(t, s, started.RunID, StateCompleted, StateFailed)

	var flagged int
	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_issues WHERE run_id = ? AND code = ?`,
		started.RunID, IssueDuplicatePage).Scan(&flagged)
	if flagged != 2 {
		t.Errorf("%d pages flagged duplicate-page, want exactly the near pair (2)", flagged)
	}
	var cleanFlagged int
	db.QueryRow(`
		SELECT COUNT(*) FROM sitecrawl_issues i
		  JOIN sitecrawl_urls u ON u.run_id = i.run_id AND u.id = i.url_id
		 WHERE i.run_id = ? AND i.code = ? AND u.url LIKE '%/khac'`,
		started.RunID, IssueDuplicatePage).Scan(&cleanFlagged)
	if cleanFlagged != 0 {
		t.Error("the genuinely different page was flagged as a duplicate")
	}
}
