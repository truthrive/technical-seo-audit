package sitecrawl

import (
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/platform/standalone"
)

// TestResumeKeepsDictionaryIDs tests that resume preserves dictionary IDs for
// various URL shapes where raw URL != canonical key (fragments, tracking params,
// reordered query parameters, host casing, already-canonical URLs).
func TestResumeKeepsDictionaryIDs(t *testing.T) {
	db, err := standalone.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	runner := NewRunner(db)
	if err := runner.EnsureSchema(); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	opts := Options{}.normalized()

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

	// Rebuild frontier the way Runner.Resume does on checkpoint resume
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
			t.Errorf("%s came back with ID 0 — pages would collapse onto one row", it.URL)
			continue
		}
		if it.ID != want[it.URL] {
			t.Errorf("%s: ID %d after resume, want %d — edges recorded before pause would be orphaned",
				it.URL, it.ID, want[it.URL])
		}
	}

	// A URL already in dictionary must not be handed a fresh ID on re-admit
	for _, u := range raw {
		id, fresh := f2.idFor(frontierKey(u, opts.IgnoreQueryParam), u)
		if fresh {
			t.Errorf("%s treated as unseen after resume — would crawl twice", u)
		}
		if id != want[u] {
			t.Errorf("%s: idFor returned %d after resume, want %d", u, id, want[u])
		}
	}
}
