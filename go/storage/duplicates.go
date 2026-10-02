package sitecrawl

import (
	"database/sql"
	"strconv"
	"strings"
)

// Duplicate group kinds, stored on sitecrawl_dupes so one table serves all of
// them and the grid can filter by which field is duplicated.
const (
	DupeTitle = "title"
	DupeMeta  = "meta"
	DupeH1    = "h1"
	DupePage  = "page"
)

// findExactDuplicates groups indexable pages by identical title, meta
// description and H1.
//
// This is stage one of two. LibreCrawl only has the similarity pass, which is
// O(n²): 50k pages is 1.25 billion pairs, hours of CPU even at implausible
// speeds. Exact grouping is a single map pass, and it is also the answer users
// actually act on — "142 pages share the title 'Trang chủ'" is a fix, whereas
// "these two pages are 0.87 similar" is a research project.
//
// The near-duplicate pass (SimHash blocking + a Ratcliff/Obershelp ratio over
// the candidates only) lands in phase 2.
func findExactDuplicates(db *sql.DB, runID string) error {
	if err := clearIssues(db, runID, []string{IssueTitleDuplicate, IssueMetaDuplicate, IssueH1Duplicate}); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM sitecrawl_dupes WHERE run_id = ? AND kind != ?`, runID, DupePage); err != nil {
		return err
	}

	// Only indexable HTML pages: a noindex or canonicalised page sharing a title
	// with its canonical target is correct behaviour, not a finding.
	rows, err := db.Query(`
		SELECT url_id, title, meta_desc, h1 FROM sitecrawl_pages
		 WHERE run_id = ? AND kind = ? AND status_class = 2 AND indexable = 1 AND is_internal = 1`,
		runID, KindHTML)
	if err != nil {
		return err
	}

	type row struct {
		id              int64
		title, meta, h1 string
	}
	var list []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.title, &r.meta, &r.h1); err != nil {
			rows.Close()
			return err
		}
		list = append(list, r)
	}
	// Drain fully before writing: the workspace DB allows a single connection,
	// so holding this cursor open while inserting would deadlock.
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var (
		issues []issueRow
		dupes  []dupeRow
	)
	collect := func(kind, code string, field func(row) string) {
		groups := map[string][]int64{}
		for _, r := range list {
			// An empty value never groups: "12 pages with no title" is already
			// reported as title-missing, and grouping them here would say the
			// same thing twice in different words.
			if k := norm(field(r)); k != "" {
				groups[k] = append(groups[k], r.id)
			}
		}
		gid := 0
		for _, ids := range groups {
			if len(ids) < 2 {
				continue
			}
			gid++
			for _, id := range ids {
				issues = append(issues, issueRow{URLID: id, issue: issue{Code: code, Detail: itoa(len(ids))}})
				dupes = append(dupes, dupeRow{GroupID: gid, URLID: id, Kind: kind, Score: 1})
			}
		}
	}
	collect(DupeTitle, IssueTitleDuplicate, func(r row) string { return r.title })
	collect(DupeMeta, IssueMetaDuplicate, func(r row) string { return r.meta })
	collect(DupeH1, IssueH1Duplicate, func(r row) string { return r.h1 })

	if err := writeIssues(db, runID, issues); err != nil {
		return err
	}
	return writeDupes(db, runID, dupes)
}

// --- near-duplicate pass (stage two) ---

// dupInput is one page as the similarity pass sees it: pre-normalised text
// plus the word count. Kept as a plain struct so the pairing logic is testable
// without a database.
type dupInput struct {
	id              int64
	url             string
	title, meta, h1 string
	words           int
}

type dupPair struct {
	i, j  int
	score float64
}

// nearDupBucketCap skips degenerate buckets. A site where every page shares
// one title creates a single giant bucket, and pairwise over it is the O(n²)
// LibreCrawl design this pass exists to avoid — those pages are already
// reported by the exact-duplicate stage anyway.
const nearDupBucketCap = 2000

// nearDupCandidates blocks on the union of two signals:
//
//   - SimHash bands (4 × 16 bits): pairs whose combined text differs by ≤3
//     bits ALWAYS share a band (pigeonhole); slightly-edited copies land here.
//   - Exact normalized title / meta / H1: real duplicate content almost always
//     keeps at least one field verbatim ("same article, year bumped in the
//     title" shares meta and H1), and this catches pairs whose SimHashes drift
//     further than banding covers.
//
// A pair differing in ALL THREE fields and by many SimHash bits can still be
// missed — that is the LSH trade every subquadratic scheme makes, and such a
// pair rarely clears the 0.85 weighted threshold anyway.
func nearDupCandidates(rows []dupInput) [][2]int {
	var groups [][]int

	hashBuckets := map[uint32][]int{}
	fieldBuckets := map[string][]int{}
	for i, r := range rows {
		h := simhash64(r.title + " " + r.meta + " " + r.h1)
		for band := 0; band < 4; band++ {
			key := uint32(band)<<16 | uint32(h>>(band*16))&0xffff
			hashBuckets[key] = append(hashBuckets[key], i)
		}
		// Field prefixes keep title-vs-meta collisions apart.
		if r.title != "" {
			fieldBuckets["t\x00"+r.title] = append(fieldBuckets["t\x00"+r.title], i)
		}
		if r.meta != "" {
			fieldBuckets["m\x00"+r.meta] = append(fieldBuckets["m\x00"+r.meta], i)
		}
		if r.h1 != "" {
			fieldBuckets["h\x00"+r.h1] = append(fieldBuckets["h\x00"+r.h1], i)
		}
	}
	for _, idxs := range hashBuckets {
		groups = append(groups, idxs)
	}
	for _, idxs := range fieldBuckets {
		groups = append(groups, idxs)
	}

	seen := map[uint64]bool{}
	var out [][2]int
	for _, idxs := range groups {
		if len(idxs) < 2 || len(idxs) > nearDupBucketCap {
			continue
		}
		for x := 0; x < len(idxs); x++ {
			for y := x + 1; y < len(idxs); y++ {
				i, j := idxs[x], idxs[y]
				if i > j {
					i, j = j, i
				}
				key := uint64(i)<<32 | uint64(j)
				if seen[key] {
					continue
				}
				seen[key] = true
				out = append(out, [2]int{i, j})
			}
		}
	}
	return out
}

// nearDupPairs scores every candidate pair with LibreCrawl's exact formula and
// keeps the ones at or above the threshold.
func nearDupPairs(rows []dupInput, threshold float64) []dupPair {
	var out []dupPair
	for _, c := range nearDupCandidates(rows) {
		a, b := rows[c[0]], rows[c[1]]
		score := contentSimilarity(a.title, a.meta, a.h1, a.words, b.title, b.meta, b.h1, b.words)
		if score >= threshold {
			out = append(out, dupPair{i: c[0], j: c[1], score: score})
		}
	}
	return out
}

// findNearDuplicates is LibreCrawl's detect_duplication_issues, restructured:
// same formula, same 0.85 threshold, but SimHash blocking instead of comparing
// all 1.25 billion pairs a 50k crawl would produce.
func findNearDuplicates(db *sql.DB, runID string, threshold float64) error {
	if err := clearIssues(db, runID, []string{IssueDuplicatePage}); err != nil {
		return err
	}
	if _, err := db.Exec(`DELETE FROM sitecrawl_dupes WHERE run_id = ? AND kind = ?`, runID, DupePage); err != nil {
		return err
	}

	rows, err := db.Query(`
		SELECT url_id, url, title, meta_desc, h1, word_count FROM sitecrawl_pages
		 WHERE run_id = ? AND kind = ? AND status_class = 2 AND indexable = 1 AND is_internal = 1`,
		runID, KindHTML)
	if err != nil {
		return err
	}
	var list []dupInput
	for rows.Next() {
		var r dupInput
		if err := rows.Scan(&r.id, &r.url, &r.title, &r.meta, &r.h1, &r.words); err != nil {
			rows.Close()
			return err
		}
		r.title, r.meta, r.h1 = norm(r.title), norm(r.meta), norm(r.h1)
		if r.title == "" && r.meta == "" && r.h1 == "" {
			continue // nothing to compare; word count alone caps at 0.10
		}
		list = append(list, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	pairs := nearDupPairs(list, threshold)
	if len(pairs) == 0 {
		return nil
	}

	// Pairs above the threshold cluster into groups (union-find), mirroring how
	// the exact stage stores groups — one table shape for both kinds.
	parent := make([]int, len(list))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	type best struct {
		other string
		score float64
	}
	bestOf := map[int]best{}
	for _, p := range pairs {
		parent[find(p.i)] = find(p.j)
		if b := bestOf[p.i]; p.score > b.score {
			bestOf[p.i] = best{other: list[p.j].url, score: p.score}
		}
		if b := bestOf[p.j]; p.score > b.score {
			bestOf[p.j] = best{other: list[p.i].url, score: p.score}
		}
	}

	groupID := map[int]int{}
	next := 0
	var (
		issues []issueRow
		dupes  []dupeRow
	)
	for idx, b := range bestOf {
		root := find(idx)
		gid, ok := groupID[root]
		if !ok {
			next++
			gid = next
			groupID[root] = gid
		}
		issues = append(issues, issueRow{URLID: list[idx].id, issue: issue{Code: IssueDuplicatePage, Detail: b.other}})
		dupes = append(dupes, dupeRow{GroupID: gid, URLID: list[idx].id, Kind: DupePage, Score: b.score})
	}
	if err := writeIssues(db, runID, issues); err != nil {
		return err
	}
	return writeDupes(db, runID, dupes)
}

type dupeRow struct {
	GroupID int
	URLID   int64
	Kind    string
	Score   float64
}

func writeDupes(db *sql.DB, runID string, rows []dupeRow) error {
	return chunked(len(rows), issueBatch, func(lo, hi int) error {
		var sb strings.Builder
		args := make([]any, 0, (hi-lo)*5)
		sb.WriteString(`INSERT OR REPLACE INTO sitecrawl_dupes(run_id, group_id, url_id, kind, score) VALUES `)
		for i := lo; i < hi; i++ {
			if i > lo {
				sb.WriteByte(',')
			}
			sb.WriteString("(?,?,?,?,?)")
			r := rows[i]
			args = append(args, runID, r.GroupID, r.URLID, r.Kind, r.Score)
		}
		_, err := db.Exec(sb.String(), args...)
		return err
	})
}

// norm is the grouping key: case-folded and whitespace-squashed, so two pages
// whose titles differ only in spacing still count as duplicates.
func norm(s string) string { return squash(strings.ToLower(s)) }

func itoa(n int) string { return strconv.Itoa(n) }
