package sitecrawl

import "testing"

// TestFrontierReoffersPassiveURL is the regression test for silent discovery
// loss: a URL whose first sighting was passive (idFor from a link-graph edge,
// or an admission that failed a check) used to be permanently unqueueable —
// admit saw a known key and returned early, so a later real link to the same
// URL never crawled it.
func TestFrontierReoffersPassiveURL(t *testing.T) {
	f := newFrontier(Options{}.normalized(), "example.com")

	// First sighting: passive, dictionary only.
	id1, _ := f.idFor(frontierKey("https://example.com/page", false), "https://example.com/page")
	if got := f.queued(); got != 0 {
		t.Fatalf("idFor queued something: %d", got)
	}

	// Second sighting: a real link. Must be queued, with the same id.
	id2, queued := f.admit("https://example.com/page", 1, SourceLink, 0)
	if !queued {
		t.Fatal("URL first seen passively was never queued — discovery loss")
	}
	if id1 != id2 {
		t.Errorf("dictionary id changed between sightings: %d then %d", id1, id2)
	}

	// Third sighting changes nothing: it is already queued.
	if _, again := f.admit("https://example.com/page", 2, SourceLink, 0); again {
		t.Error("an already-queued URL was queued twice")
	}
}

// TestFrontierReoffersAfterFailedCheck: a URL declined at admission (too deep)
// is re-offered when rediscovered within limits.
func TestFrontierReoffersAfterFailedCheck(t *testing.T) {
	opts := Options{}.normalized()
	opts.MaxDepth = 2
	f := newFrontier(opts, "example.com")

	if _, queued := f.admit("https://example.com/deep", 5, SourceLink, 0); queued {
		t.Fatal("depth 5 was admitted with maxDepth 2")
	}
	if _, queued := f.admit("https://example.com/deep", 1, SourceLink, 0); !queued {
		t.Error("URL declined for depth was not re-offered at a shallower depth")
	}
}
