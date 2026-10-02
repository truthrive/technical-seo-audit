package sitecrawl

import (
	"fmt"
	"testing"
)

// TestDictionaryStopsGrowingAtMaxURLs: MaxURLs used to bound only the queue.
// idFor allocated an entry in seen, notQueued and pending unconditionally, and
// collectLinks calls it directly for every link, image, stylesheet and script it
// does not follow — so the maps kept growing one entry per distinct URL ever
// sighted, with nothing trimming them.
func TestDictionaryStopsGrowingAtMaxURLs(t *testing.T) {
	opts := Options{MaxURLs: 10}.normalized()
	opts.MaxURLs = 10 // normalized() may clamp; this test is about the cap itself
	f := newFrontier(opts, "example.com")

	for i := 0; i < 500; i++ {
		f.admit(fmt.Sprintf("https://example.com/p/%d", i), 1, SourceLink, 0)
	}
	// The direct path collectLinks uses for non-followed targets.
	for i := 0; i < 500; i++ {
		u := fmt.Sprintf("https://cdn.example.net/asset/%d.js", i)
		f.idFor(frontierKey(u, opts.IgnoreQueryParam), u)
	}

	if len(f.seen) > opts.MaxURLs {
		t.Errorf("dictionary holds %d entries with MaxURLs=%d", len(f.seen), opts.MaxURLs)
	}
	if len(f.notQueued) > opts.MaxURLs {
		t.Errorf("notQueued holds %d entries with MaxURLs=%d", len(f.notQueued), opts.MaxURLs)
	}
	if len(f.pending) > opts.MaxURLs {
		t.Errorf("pending holds %d entries with MaxURLs=%d", len(f.pending), opts.MaxURLs)
	}
	if !f.hitURLCap {
		t.Error("hitURLCap not recorded, so the run would complete without naming the limit that ended it")
	}
}

// TestKnownURLsStillResolveWhenFull: the cap must refuse new entries without
// breaking lookups of ones already allocated, or edges to known pages would
// start coming back as 0.
func TestKnownURLsStillResolveWhenFull(t *testing.T) {
	opts := Options{MaxURLs: 3}.normalized()
	opts.MaxURLs = 3
	f := newFrontier(opts, "example.com")

	first := "https://example.com/a"
	id, queued := f.admit(first, 1, SourceLink, 0)
	if !queued || id == 0 {
		t.Fatalf("admit(%q) = (%d, %v), want a queued URL with a real id", first, id, queued)
	}
	for i := 0; i < 20; i++ {
		f.admit(fmt.Sprintf("https://example.com/filler/%d", i), 1, SourceLink, 0)
	}
	again, fresh := f.idFor(frontierKey(first, opts.IgnoreQueryParam), first)
	if fresh {
		t.Error("a URL already in the dictionary was reported as new")
	}
	if again != id {
		t.Errorf("id for %q changed from %d to %d once the dictionary filled", first, id, again)
	}
}

func TestDepthCapIsRecorded(t *testing.T) {
	opts := Options{MaxDepth: 1}.normalized()
	opts.MaxDepth = 1
	f := newFrontier(opts, "example.com")

	if _, queued := f.admit("https://example.com/shallow", 1, SourceLink, 0); !queued {
		t.Fatal("a URL at MaxDepth should be queued")
	}
	if _, queued := f.admit("https://example.com/deep", 2, SourceLink, 0); queued {
		t.Fatal("a URL past MaxDepth should be refused")
	}
	if !f.hitDepthCap {
		t.Error("hitDepthCap not recorded, so the run would complete without naming the depth limit")
	}
}
