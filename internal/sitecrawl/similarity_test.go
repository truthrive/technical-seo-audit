package sitecrawl

import (
	"math"
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

func TestSimhash64(t *testing.T) {
	h1 := simhash64("hello world")
	h2 := simhash64("hello world")
	if h1 != h2 {
		t.Errorf("simhash64 is not deterministic: %x != %x", h1, h2)
	}
	h3 := simhash64("something completely different and unrelated")
	if h1 == h3 {
		t.Errorf("different text produced identical simhash: %x", h1)
	}
}
