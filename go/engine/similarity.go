package sitecrawl

import "hash/fnv"

// This file is the near-duplicate math: a faithful port of Python difflib's
// SequenceMatcher.ratio() (Ratcliff/Obershelp with autojunk), LibreCrawl's
// weighted content-similarity formula on top of it, and the 64-bit SimHash
// used to avoid comparing every pair.
//
// The ratio port matters because the 0.85 threshold is LibreCrawl's number,
// measured against difflib's ratio — a "close enough" similarity function
// would silently change what 0.85 means.

// seqRatio is difflib.SequenceMatcher(None, a, b).ratio().
//
// ratio = 2*M / (len(a)+len(b)), where M is the total size of the matching
// blocks found by recursively taking the longest match. Autojunk mirrors
// difflib: when b has ≥200 elements, characters occurring in more than 1% of
// b are excluded from matching (and only reattached by the junk-extension
// step).
func seqRatio(sa, sb string) float64 {
	a, b := []rune(sa), []rune(sb)
	la, lb := len(a), len(b)
	if la+lb == 0 {
		return 1
	}
	if la == 0 || lb == 0 {
		return 0
	}

	// b2j: element -> indices in b, with difflib's autojunk applied.
	b2j := make(map[rune][]int, lb)
	for j, r := range b {
		b2j[r] = append(b2j[r], j)
	}
	popular := map[rune]bool{}
	if lb >= 200 {
		ntest := lb/100 + 1
		for r, idxs := range b2j {
			if len(idxs) > ntest {
				popular[r] = true
			}
		}
		for r := range popular {
			delete(b2j, r)
		}
	}

	// findLongestMatch, difflib's j2len rolling map.
	flm := func(alo, ahi, blo, bhi int) (int, int, int) {
		besti, bestj, bestsize := alo, blo, 0
		j2len := map[int]int{}
		for i := alo; i < ahi; i++ {
			newj2len := map[int]int{}
			for _, j := range b2j[a[i]] {
				if j < blo {
					continue
				}
				if j >= bhi {
					break
				}
				k := j2len[j-1] + 1
				newj2len[j] = k
				if k > bestsize {
					besti, bestj, bestsize = i-k+1, j-k+1, k
				}
			}
			j2len = newj2len
		}
		// Extend over non-junk neighbours, then over junk — same order as
		// difflib, which is what keeps ties resolved identically.
		for besti > alo && bestj > blo && !popular[b[bestj-1]] && a[besti-1] == b[bestj-1] {
			besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
		}
		for besti+bestsize < ahi && bestj+bestsize < bhi &&
			!popular[b[bestj+bestsize]] && a[besti+bestsize] == b[bestj+bestsize] {
			bestsize++
		}
		for besti > alo && bestj > blo && popular[b[bestj-1]] && a[besti-1] == b[bestj-1] {
			besti, bestj, bestsize = besti-1, bestj-1, bestsize+1
		}
		for besti+bestsize < ahi && bestj+bestsize < bhi &&
			popular[b[bestj+bestsize]] && a[besti+bestsize] == b[bestj+bestsize] {
			bestsize++
		}
		return besti, bestj, bestsize
	}

	// Sum matching blocks; the order they are found in does not affect M.
	matched := 0
	type span struct{ alo, ahi, blo, bhi int }
	queue := []span{{0, la, 0, lb}}
	for len(queue) > 0 {
		s := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		i, j, k := flm(s.alo, s.ahi, s.blo, s.bhi)
		if k == 0 {
			continue
		}
		matched += k
		queue = append(queue, span{s.alo, i, s.blo, j}, span{i + k, s.ahi, j + k, s.bhi})
	}
	return 2 * float64(matched) / float64(la+lb)
}

// contentSimilarity is LibreCrawl's _calculate_content_similarity, verbatim:
// per-field difflib ratios (0 when either side is empty) plus a word-count
// ratio, combined as 0.35·title + 0.35·meta + 0.20·h1 + 0.10·words.
// Inputs are expected pre-normalised (lowercased, squashed) by the caller.
func contentSimilarity(title1, meta1, h1a string, words1 int, title2, meta2, h1b string, words2 int) float64 {
	fieldSim := func(x, y string) float64 {
		if x == "" || y == "" {
			return 0
		}
		return seqRatio(x, y)
	}
	var wordSim float64
	if words1 > 0 && words2 > 0 {
		lo, hi := words1, words2
		if lo > hi {
			lo, hi = hi, lo
		}
		wordSim = float64(lo) / float64(hi)
	}
	return fieldSim(title1, title2)*0.35 +
		fieldSim(meta1, meta2)*0.35 +
		fieldSim(h1a, h1b)*0.20 +
		wordSim*0.10
}

// simhash64 fingerprints a text through its character 3-shingles. Similar
// texts land within a few bits of each other, which is what makes band
// blocking work.
func simhash64(text string) uint64 {
	runes := []rune(text)
	var v [64]int
	shingle := func(rs []rune) uint64 {
		h := fnv.New64a()
		for _, r := range rs {
			var buf [4]byte
			buf[0] = byte(r)
			buf[1] = byte(r >> 8)
			buf[2] = byte(r >> 16)
			buf[3] = byte(r >> 24)
			h.Write(buf[:])
		}
		return h.Sum64()
	}
	add := func(h uint64) {
		for bit := 0; bit < 64; bit++ {
			if h>>bit&1 == 1 {
				v[bit]++
			} else {
				v[bit]--
			}
		}
	}
	if len(runes) < 3 {
		if len(runes) > 0 {
			add(shingle(runes))
		}
	} else {
		for i := 0; i+3 <= len(runes); i++ {
			add(shingle(runes[i : i+3]))
		}
	}
	var out uint64
	for bit := 0; bit < 64; bit++ {
		if v[bit] > 0 {
			out |= 1 << bit
		}
	}
	return out
}
