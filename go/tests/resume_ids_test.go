package sitecrawl

import (
	"testing"
)

// TestResumeKeepsDictionaryIDs is the regression test for resume collapsing a
// run's pages into a single row.
//
// The dictionary stores the raw URL in sitecrawl_urls.url, but the frontier keys
// its seen map by frontierKey — which drops the fragment, sorts query params,
// strips tracking params and lowercases the host. loadSeen used to rebuild the
// map from the raw column, so restore's frontierKey lookup missed and every
// queued item came back with ID 0. writePages then upserts on
// PRIMARY KEY (run_id, url_id), so all of them overwrite one another.
//
// The URLs below are the shapes where raw != key, and they are ordinary: a
// fragment link and a campaign link appear on most real sites.
func TestResumeKeepsDictionaryIDs(t *testing.T) {
	s, db, _ := newTestService(t)
	opts := s.DefaultOptions()

	raw := []string{
		"https://example.com/a#section",          // fragment dropped by frontierKey
		"https://example.com/b?utm_source=x&p=1", // tracking param stripped
		"https://example.com/c?b=2&a=1",          // query params sorted
		"https://EXAMPLE.com/d",                  // host lowercased
		"https://example.com/e",                  // already canonical
	}

	f := newFrontier(opts, "example.com")
	for _, u := range raw {
		if _, queued := f.admit(u, 1, "link", 0); !queued {
			t.Fatalf("admit(%q) refused a same-site URL", u)
		}
	}
	if err := writeURLs(db, "run1", f.takePending()); err != nil {
		t.Fatalf("writeURLs: %v", err)
	}
	want := map[string]int64{}
	for _, u := range raw {
		want[u] = f.seen[frontierKey(u, opts.IgnoreQueryParam)]
	}
	if err := insertRun(db, "run1", "https://example.com/", "example.com", opts); err != nil {
		t.Fatalf("insertRun: %v", err)
	}
	var items []frontierItem
	for _, u := range raw {
		items = append(items, frontierItem{URL: u, Depth: 1, Source: "link"})
	}
	if err := saveFrontier(db, "run1", items); err != nil {
		t.Fatalf("saveFrontier: %v", err)
	}

	// Restart: rebuild the frontier the way service.go does on resume.
	seen, nextID, err := loadSeen(db, "run1", opts.IgnoreQueryParam)
	if err != nil {
		t.Fatalf("loadSeen: %v", err)
	}
	restored, err := loadFrontier(db, "run1")
	if err != nil {
		t.Fatalf("loadFrontier: %v", err)
	}
	f2 := newFrontier(opts, "example.com")
	f2.restore(seen, nextID, restored, 0)

	for _, it := range f2.queue {
		if it.ID == 0 {
			t.Errorf("%s came back with ID 0 — its pages would all collapse onto one row", it.URL)
			continue
		}
		if it.ID != want[it.URL] {
			t.Errorf("%s: ID %d after resume, want %d — edges recorded before the pause are orphaned",
				it.URL, it.ID, want[it.URL])
		}
	}

	// A URL already in the dictionary must not be handed a fresh id on re-admit,
	// or the whole site is fetched again after every resume.
	for _, u := range raw {
		id, fresh := f2.idFor(frontierKey(u, opts.IgnoreQueryParam), u)
		if fresh {
			t.Errorf("%s was treated as unseen after resume — it would be crawled and paid for twice", u)
		}
		if id != want[u] {
			t.Errorf("%s: idFor returned %d after resume, want %d", u, id, want[u])
		}
	}
}
