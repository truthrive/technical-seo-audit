package adapter_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

func setupTestRunWithOpts(t *testing.T, db *sql.DB, runID, seed string, opts sitecrawl.Options) {
	t.Helper()
	optBytes, err := json.Marshal(opts)
	if err != nil {
		t.Fatalf("marshal options failed: %v", err)
	}
	_, err = db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', ?, 'completed', '2026-10-01T00:00:00Z')`, runID, seed, string(optBytes))
	if err != nil {
		t.Fatalf("setup run with opts failed: %v", err)
	}
}

// Tests covering all V1.5a Redirect Evidence Enablement requirements:
// 1. non-redirect response
// 2. one-hop complete redirect
// 3. multi-hop complete redirect
// 4. FollowRedirects=false
// 5. explicit redirect loop
// 6. redirect chain too long
// 7. invalid/missing Location
// 8. transport failure after first hop
// 9. deterministic hop ordering
// 10. hop count matches preserved chain
// 11. complete chain emits final URL
// 12. incomplete chain withholds final URL
// 13. loop=true only on explicit loop
// 14. loop=false only on proven complete chain
// 15. unrelated incomplete chain withholds loop
// 16. FinalURLID maps actual final target
// 17. FinalURLID never maps first hop as final
// 18. absent final target leaves FinalURLID nil
// 19. no final_status normalized observation
// 20. existing 7 evaluators remain green
func TestAdapter_RedirectEvidence(t *testing.T) {
	// 1. non-redirect response
	t.Run("1. non-redirect response", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:200"
		seed := "https://example.com/"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)

		pageData, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:200",
			SnapshotID: "snap:run:redirect:200",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:200:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if vals := fields["redirect_initial_observed"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("expected redirect_initial_observed 'false', got %v", vals)
		}
		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "0" {
			t.Errorf("expected redirect_hop_count '0', got %v", vals)
		}
		if vals := fields["redirect_hop"]; len(vals) != 0 {
			t.Errorf("expected no redirect_hop, got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 0 {
			t.Errorf("expected no redirect_traversal_complete, got %v", vals)
		}
		if vals := fields["redirect_loop_detected"]; len(vals) != 0 {
			t.Errorf("expected no redirect_loop_detected, got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("expected no redirect_final_url, got %v", vals)
		}
		if len(res.FetchObservations) != 1 || res.FetchObservations[0].FinalURLID != nil {
			t.Errorf("expected FinalURLID to be nil, got %v", res.FetchObservations[0].FinalURLID)
		}
	})

	// 2. one-hop complete redirect
	t.Run("2. one-hop complete redirect", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:1hop"
		seed := "https://example.com/start"
		target := "https://example.com/final"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)
		insertURL(t, db, runID, 2, target)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: target,
			Redirects: []sitecrawl.Hop{
				{URL: seed, Status: 301, Location: target},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, ?, 1, 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), target)

		page2, _ := json.Marshal(sitecrawl.Page{
			URL:       target,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:11Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 2, ?, ?, 200, 'html', '2026-10-01T00:00:11Z')`, runID, target, string(page2))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:1hop",
			SnapshotID: "snap:run:redirect:1hop",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:1hop:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if vals := fields["redirect_initial_observed"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_initial_observed 'true', got %v", vals)
		}
		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "1" {
			t.Errorf("expected redirect_hop_count '1', got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_traversal_complete 'true', got %v", vals)
		}
		if vals := fields["redirect_loop_detected"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("expected redirect_loop_detected 'false', got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 1 || vals[0] != target {
			t.Errorf("expected redirect_final_url %q, got %v", target, vals)
		}

		// Check redirect_hop JSON structure
		if vals := fields["redirect_hop"]; len(vals) != 1 {
			t.Fatalf("expected 1 redirect_hop, got %d", len(vals))
		} else {
			var hopObj map[string]interface{}
			if err := json.Unmarshal([]byte(vals[0]), &hopObj); err != nil {
				t.Fatalf("failed to parse redirect_hop JSON: %v", err)
			}
			if hopObj["hop_index"] != float64(0) {
				t.Errorf("expected hop_index 0, got %v", hopObj["hop_index"])
			}
			if hopObj["source_url"] != seed {
				t.Errorf("expected source_url %q, got %v", seed, hopObj["source_url"])
			}
			if hopObj["status"] != float64(301) {
				t.Errorf("expected status 301, got %v", hopObj["status"])
			}
			if hopObj["resolved_target_url"] != target {
				t.Errorf("expected resolved_target_url %q, got %v", target, hopObj["resolved_target_url"])
			}
		}

		// Check FinalURLID mapped to url 2
		var fetch1 *audit.FetchObservation
		for i := range res.FetchObservations {
			if res.FetchObservations[i].URLID == "url:audit:run:redirect:1hop:1" {
				fetch1 = &res.FetchObservations[i]
				break
			}
		}
		if fetch1 == nil || fetch1.FinalURLID == nil || *fetch1.FinalURLID != "url:audit:run:redirect:1hop:2" {
			t.Errorf("expected FinalURLID 'url:audit:run:redirect:1hop:2', got %v", fetch1.FinalURLID)
		}
	})

	// 3. multi-hop complete redirect
	t.Run("3. multi-hop complete redirect", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:2hop"
		hop0URL := "https://example.com/step0"
		hop1URL := "https://example.com/step1"
		targetURL := "https://example.com/dest"
		setupTestRunWithOpts(t, db, runID, hop0URL, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, hop0URL)
		insertURL(t, db, runID, 2, hop1URL)
		insertURL(t, db, runID, 3, targetURL)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        hop0URL,
			Status:     301,
			RedirectTo: hop1URL, // SiteCrawl sets RedirectTo to FIRST hop
			Redirects: []sitecrawl.Hop{
				{URL: hop0URL, Status: 301, Location: hop1URL},
				{URL: hop1URL, Status: 302, Location: targetURL},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, ?, 2, 'other', '2026-10-01T00:00:10Z')`, runID, hop0URL, string(page1), hop1URL)

		page3, _ := json.Marshal(sitecrawl.Page{
			URL:       targetURL,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:12Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 3, ?, ?, 200, 'html', '2026-10-01T00:00:12Z')`, runID, targetURL, string(page3))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:2hop",
			SnapshotID: "snap:run:redirect:2hop",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:2hop:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "2" {
			t.Errorf("expected redirect_hop_count '2', got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_traversal_complete 'true', got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 1 || vals[0] != targetURL {
			t.Errorf("expected redirect_final_url %q, got %v", targetURL, vals)
		}

		// Check FinalURLID maps to targetURL (url ID 3), NEVER hop 1 (url ID 2)
		var fetch0 *audit.FetchObservation
		for i := range res.FetchObservations {
			if res.FetchObservations[i].URLID == "url:audit:run:redirect:2hop:1" {
				fetch0 = &res.FetchObservations[i]
				break
			}
		}
		if fetch0 == nil || fetch0.FinalURLID == nil || *fetch0.FinalURLID != "url:audit:run:redirect:2hop:3" {
			t.Errorf("expected FinalURLID 'url:audit:run:redirect:2hop:3', got %v", fetch0.FinalURLID)
		}
		if fetch0 != nil && fetch0.FinalURLID != nil && *fetch0.FinalURLID == "url:audit:run:redirect:2hop:2" {
			t.Errorf("FinalURLID improperly mapped first hop %v", fetch0.FinalURLID)
		}
	})

	// 4. FollowRedirects=false
	t.Run("4. FollowRedirects=false", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:nofollow"
		seed := "https://example.com/start"
		target := "https://example.com/final"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: false})
		insertURL(t, db, runID, 1, seed)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: target,
			Redirects: []sitecrawl.Hop{
				{URL: seed, Status: 301, Location: target},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, ?, 1, 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), target)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:nofollow",
			SnapshotID: "snap:run:redirect:nofollow",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:nofollow:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if vals := fields["redirect_initial_observed"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_initial_observed 'true', got %v", vals)
		}
		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "1" {
			t.Errorf("expected redirect_hop_count '1', got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("expected redirect_traversal_complete 'false', got %v", vals)
		}
		if vals := fields["redirect_loop_detected"]; len(vals) != 0 {
			t.Errorf("expected redirect_loop_detected to be withheld, got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("expected redirect_final_url to be withheld, got %v", vals)
		}
		if len(res.FetchObservations) != 1 || res.FetchObservations[0].FinalURLID != nil {
			t.Errorf("expected FinalURLID to be nil when FollowRedirects=false, got %v", res.FetchObservations[0].FinalURLID)
		}
	})

	// 5. explicit redirect loop
	t.Run("5. explicit redirect loop", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:loop"
		seed := "https://example.com/loop-a"
		stepB := "https://example.com/loop-b"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     302,
			RedirectTo: stepB,
			Error:      "redirect loop",
			ErrorType:  sitecrawl.ErrConnection,
			Redirects: []sitecrawl.Hop{
				{URL: seed, Status: 302, Location: stepB},
				{URL: stepB, Status: 302, Location: seed},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, error_type, kind, crawled_at)
			VALUES(?, 1, ?, ?, 302, ?, 2, 'connection', 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), stepB)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:loop",
			SnapshotID: "snap:run:redirect:loop",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:loop:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if vals := fields["redirect_initial_observed"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_initial_observed 'true', got %v", vals)
		}
		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "2" {
			t.Errorf("expected redirect_hop_count '2', got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("expected redirect_traversal_complete 'false', got %v", vals)
		}
		if vals := fields["redirect_loop_detected"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_loop_detected 'true', got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("expected redirect_final_url to be withheld, got %v", vals)
		}
		if len(res.FetchObservations) != 1 || res.FetchObservations[0].FinalURLID != nil {
			t.Errorf("expected FinalURLID to be nil on loop, got %v", res.FetchObservations[0].FinalURLID)
		}
	})

	// 6. redirect chain too long
	t.Run("6. redirect chain too long", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:toolong"
		seed := "https://example.com/h0"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)

		hops := []sitecrawl.Hop{
			{URL: "https://example.com/h0", Status: 301, Location: "https://example.com/h1"},
			{URL: "https://example.com/h1", Status: 301, Location: "https://example.com/h2"},
			{URL: "https://example.com/h2", Status: 301, Location: "https://example.com/h3"},
			{URL: "https://example.com/h3", Status: 301, Location: "https://example.com/h4"},
			{URL: "https://example.com/h4", Status: 301, Location: "https://example.com/h5"},
		}
		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: "https://example.com/h1",
			Error:      "redirect chain too long",
			ErrorType:  sitecrawl.ErrConnection,
			Redirects:  hops,
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, error_type, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, 'https://example.com/h1', 5, 'connection', 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:toolong",
			SnapshotID: "snap:run:redirect:toolong",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:toolong:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "5" {
			t.Errorf("expected redirect_hop_count '5', got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("expected redirect_traversal_complete 'false', got %v", vals)
		}
		if vals := fields["redirect_loop_detected"]; len(vals) != 0 {
			t.Errorf("expected redirect_loop_detected to be withheld on chain too long, got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("expected redirect_final_url to be withheld, got %v", vals)
		}
	})

	// 7. invalid/missing Location
	t.Run("7. invalid/missing Location", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:badloc"
		seed := "https://example.com/badloc"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:       seed,
			Status:    301,
			Error:     "empty Location header",
			ErrorType: sitecrawl.ErrConnection,
			Redirects: nil,
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, error_type, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, 'connection', 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:badloc",
			SnapshotID: "snap:run:redirect:badloc",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:badloc:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if vals := fields["redirect_initial_observed"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_initial_observed 'true', got %v", vals)
		}
		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "0" {
			t.Errorf("expected redirect_hop_count '0', got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("expected redirect_traversal_complete 'false', got %v", vals)
		}
		if vals := fields["redirect_loop_detected"]; len(vals) != 0 {
			t.Errorf("expected redirect_loop_detected to be withheld, got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("expected redirect_final_url to be withheld, got %v", vals)
		}
	})

	// 8. transport failure after first hop
	t.Run("8. transport failure after first hop", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:neterr"
		seed := "https://example.com/step1"
		target := "https://example.com/step2"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: target,
			Error:      "context deadline exceeded",
			ErrorType:  sitecrawl.ErrTimeout,
			Redirects: []sitecrawl.Hop{
				{URL: seed, Status: 301, Location: target},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, error_type, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, ?, 1, 'timeout', 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), target)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:neterr",
			SnapshotID: "snap:run:redirect:neterr",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:neterr:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		if vals := fields["redirect_initial_observed"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_initial_observed 'true', got %v", vals)
		}
		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "1" {
			t.Errorf("expected redirect_hop_count '1', got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("expected redirect_traversal_complete 'false', got %v", vals)
		}
		if vals := fields["redirect_loop_detected"]; len(vals) != 0 {
			t.Errorf("expected redirect_loop_detected to be withheld on transport failure, got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("expected redirect_final_url to be withheld, got %v", vals)
		}
	})

	// 9. deterministic hop ordering
	t.Run("9. deterministic hop ordering", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:order"
		seed := "https://example.com/h0"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)

		hops := []sitecrawl.Hop{
			{URL: "https://example.com/h0", Status: 301, Location: "https://example.com/h1"},
			{URL: "https://example.com/h1", Status: 302, Location: "https://example.com/h2"},
			{URL: "https://example.com/h2", Status: 307, Location: "https://example.com/final"},
		}
		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: "https://example.com/h1",
			Redirects:  hops,
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, 'https://example.com/h1', 3, 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1))

		res1, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:order",
			SnapshotID: "snap:run:redirect:order",
		})
		if err != nil {
			t.Fatalf("build 1 failed: %v", err)
		}

		res2, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:order",
			SnapshotID: "snap:run:redirect:order",
		})
		if err != nil {
			t.Fatalf("build 2 failed: %v", err)
		}

		var hops1, hops2 []string
		for _, obs := range res1.EvidenceSnapshot.NormalizedObservations {
			if obs.Field == "redirect_hop" {
				hops1 = append(hops1, obs.Value)
			}
		}
		for _, obs := range res2.EvidenceSnapshot.NormalizedObservations {
			if obs.Field == "redirect_hop" {
				hops2 = append(hops2, obs.Value)
			}
		}

		if len(hops1) != 3 || len(hops2) != 3 {
			t.Fatalf("expected 3 hops, got %d and %d", len(hops1), len(hops2))
		}
		for i := 0; i < 3; i++ {
			if hops1[i] != hops2[i] {
				t.Errorf("hop %d mismatch between runs: %s vs %s", i, hops1[i], hops2[i])
			}
		}
	})

	// 10. promoted count mismatch detects inconsistency and withholds authoritative completeness
	t.Run("10. promoted count mismatch detects inconsistency", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:hopcount"
		seed := "https://example.com/start"
		target := "https://example.com/final"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)
		insertURL(t, db, runID, 2, target)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: target,
			Redirects: []sitecrawl.Hop{
				{URL: seed, Status: 301, Location: target},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		// Promoted redirect_hops column in SQL set to 99 intentionally (conflicts with len(Page.Redirects)==1)
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, ?, 99, 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), target)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:hopcount",
			SnapshotID: "snap:run:redirect:hopcount",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		fields := make(map[string][]string)
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.SubjectRef == "url:audit:run:redirect:hopcount:1" {
				fields[obs.Field] = append(fields[obs.Field], obs.Value)
			}
		}

		// Conflict withholds redirect_hop_count
		if vals := fields["redirect_hop_count"]; len(vals) != 0 {
			t.Errorf("expected redirect_hop_count to be withheld on conflict, got %v", vals)
		}
		// Conflict does not emit traversal_complete=true
		for _, v := range fields["redirect_traversal_complete"] {
			if v == "true" {
				t.Errorf("conflict must not emit redirect_traversal_complete=true, got %v", fields["redirect_traversal_complete"])
			}
		}
		// Conflict does not emit loop=false
		for _, v := range fields["redirect_loop_detected"] {
			if v == "false" {
				t.Errorf("conflict must not emit redirect_loop_detected=false, got %v", fields["redirect_loop_detected"])
			}
		}
		// Conflict withholds redirect_final_url
		if vals := fields["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("expected redirect_final_url to be withheld on conflict, got %v", vals)
		}
		// Conflict leaves FinalURLID nil
		if len(res.FetchObservations) != 1 || res.FetchObservations[0].FinalURLID != nil {
			t.Errorf("expected FinalURLID to be nil on conflict, got %v", res.FetchObservations[0].FinalURLID)
		}
		// Conflict records EvidenceGap
		var foundGap bool
		for _, gap := range res.EvidenceGaps {
			if gap.GapCode == adapter.GapRedirectChainInconsistent && gap.SubjectRef == "url:audit:run:redirect:hopcount:1" {
				foundGap = true
				if gap.Field != "redirect_hops" {
					t.Errorf("expected gap field 'redirect_hops', got %q", gap.Field)
				}
				if gap.SourceComponent != "sitecrawl_pages" {
					t.Errorf("expected gap sourceComponent 'sitecrawl_pages', got %q", gap.SourceComponent)
				}
				break
			}
		}
		if !foundGap {
			t.Errorf("expected EvidenceGap %s to be recorded for conflict", adapter.GapRedirectChainInconsistent)
		}
		// Preserved redirect_hop observation still emitted
		if vals := fields["redirect_hop"]; len(vals) != 1 {
			t.Errorf("expected 1 preserved redirect_hop observation, got %v", vals)
		}
		// Typed RedirectHop: LocationRaw is empty, ResolvedTargetURL is target
		if len(res.RedirectHops) != 1 {
			t.Fatalf("expected 1 typed RedirectHop, got %d", len(res.RedirectHops))
		}
		if res.RedirectHops[0].LocationRaw != "" {
			t.Errorf("expected LocationRaw to be empty, got %q", res.RedirectHops[0].LocationRaw)
		}
		if res.RedirectHops[0].ResolvedTargetURL != target {
			t.Errorf("expected ResolvedTargetURL %q, got %q", target, res.RedirectHops[0].ResolvedTargetURL)
		}
	})

	// 11. complete chain emits final URL
	// 12. incomplete chain withholds final URL
	// 13. loop=true only on explicit loop
	// 14. loop=false only on proven complete chain
	// 15. unrelated incomplete chain withholds loop
	t.Run("11-15. loop and final URL conditional semantics", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:matrix"
		seed1 := "https://example.com/complete"
		target1 := "https://example.com/complete-dest"
		seed2 := "https://example.com/loop"
		seed3 := "https://example.com/timeout"

		setupTestRunWithOpts(t, db, runID, seed1, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed1)
		insertURL(t, db, runID, 2, target1)
		insertURL(t, db, runID, 3, seed2)
		insertURL(t, db, runID, 4, seed3)

		// 1: complete chain
		p1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed1,
			Status:     301,
			RedirectTo: target1,
			Redirects:  []sitecrawl.Hop{{URL: seed1, Status: 301, Location: target1}},
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, ?, 1, 'other', '2026-10-01T00:00:10Z')`, runID, seed1, string(p1), target1)

		// 2: dest page
		p2, _ := json.Marshal(sitecrawl.Page{
			URL:       target1,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:11Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 2, ?, ?, 200, 'html', '2026-10-01T00:00:11Z')`, runID, target1, string(p2))

		// 3: loop
		p3, _ := json.Marshal(sitecrawl.Page{
			URL:        seed2,
			Status:     302,
			RedirectTo: seed2,
			Error:      "redirect loop",
			ErrorType:  sitecrawl.ErrConnection,
			Redirects:  []sitecrawl.Hop{{URL: seed2, Status: 302, Location: seed2}},
			CrawledAt:  "2026-10-01T00:00:12Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, error_type, kind, crawled_at)
			VALUES(?, 3, ?, ?, 302, ?, 1, 'connection', 'other', '2026-10-01T00:00:12Z')`, runID, seed2, string(p3), seed2)

		// 4: timeout (unrelated incomplete chain)
		p4, _ := json.Marshal(sitecrawl.Page{
			URL:        seed3,
			Status:     301,
			RedirectTo: "https://example.com/stepX",
			Error:      "timeout waiting for response",
			ErrorType:  sitecrawl.ErrTimeout,
			Redirects:  []sitecrawl.Hop{{URL: seed3, Status: 301, Location: "https://example.com/stepX"}},
			CrawledAt:  "2026-10-01T00:00:13Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, error_type, kind, crawled_at)
			VALUES(?, 4, ?, ?, 301, 'https://example.com/stepX', 1, 'timeout', 'other', '2026-10-01T00:00:13Z')`, runID, seed3, string(p4))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:matrix",
			SnapshotID: "snap:run:redirect:matrix",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		f1 := getFieldsBySubj(res, "url:audit:run:redirect:matrix:1")
		f3 := getFieldsBySubj(res, "url:audit:run:redirect:matrix:3")
		f4 := getFieldsBySubj(res, "url:audit:run:redirect:matrix:4")

		// 11. complete chain emits final URL
		if vals := f1["redirect_final_url"]; len(vals) != 1 || vals[0] != target1 {
			t.Errorf("11: expected final url %q, got %v", target1, vals)
		}
		// 12. incomplete chain withholds final URL
		if vals := f3["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("12: expected final url withheld on loop, got %v", vals)
		}
		if vals := f4["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("12: expected final url withheld on timeout, got %v", vals)
		}

		// 13. loop=true only on explicit loop
		if vals := f3["redirect_loop_detected"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("13: expected loop=true on explicit loop, got %v", vals)
		}
		// 14. loop=false only on proven complete chain
		if vals := f1["redirect_loop_detected"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("14: expected loop=false on complete chain, got %v", vals)
		}
		// 15. unrelated incomplete chain withholds loop
		if vals := f4["redirect_loop_detected"]; len(vals) != 0 {
			t.Errorf("15: expected loop withheld on timeout, got %v", vals)
		}
	})

	// 16. FinalURLID maps actual final target
	// 17. FinalURLID never maps first hop as final
	// 18. absent final target leaves FinalURLID nil
	t.Run("16-18. FinalURLID correlation semantics", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:finalurlid"
		u1 := "https://example.com/h0"
		u2 := "https://example.com/h1"
		u3 := "https://example.com/h2"
		u4 := "https://example.com/missing-dest-src"

		setupTestRunWithOpts(t, db, runID, u1, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, u1)
		insertURL(t, db, runID, 2, u2)
		insertURL(t, db, runID, 3, u3)
		insertURL(t, db, runID, 4, u4)
		// Destination https://example.com/external-dest is NOT inserted into sitecrawl_urls

		// 2-hop chain u1 -> u2 -> u3
		p1, _ := json.Marshal(sitecrawl.Page{
			URL:        u1,
			Status:     301,
			RedirectTo: u2, // first hop!
			Redirects: []sitecrawl.Hop{
				{URL: u1, Status: 301, Location: u2},
				{URL: u2, Status: 302, Location: u3},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, ?, 2, 'other', '2026-10-01T00:00:10Z')`, runID, u1, string(p1), u2)

		p3, _ := json.Marshal(sitecrawl.Page{
			URL:       u3,
			Status:    200,
			CrawledAt: "2026-10-01T00:00:12Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, kind, crawled_at)
			VALUES(?, 3, ?, ?, 200, 'html', '2026-10-01T00:00:12Z')`, runID, u3, string(p3))

		// 1-hop chain pointing to absent target
		p4, _ := json.Marshal(sitecrawl.Page{
			URL:        u4,
			Status:     301,
			RedirectTo: "https://example.com/external-dest",
			Redirects: []sitecrawl.Hop{
				{URL: u4, Status: 301, Location: "https://example.com/external-dest"},
			},
			CrawledAt: "2026-10-01T00:00:13Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 4, ?, ?, 301, 'https://example.com/external-dest', 1, 'other', '2026-10-01T00:00:13Z')`, runID, u4, string(p4))

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:finalurlid",
			SnapshotID: "snap:run:redirect:finalurlid",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		var fetch1, fetch4 *audit.FetchObservation
		for i := range res.FetchObservations {
			if res.FetchObservations[i].URLID == "url:audit:run:redirect:finalurlid:1" {
				fetch1 = &res.FetchObservations[i]
			}
			if res.FetchObservations[i].URLID == "url:audit:run:redirect:finalurlid:4" {
				fetch4 = &res.FetchObservations[i]
			}
		}

		// 16. FinalURLID maps actual final target
		if fetch1 == nil || fetch1.FinalURLID == nil || *fetch1.FinalURLID != "url:audit:run:redirect:finalurlid:3" {
			t.Errorf("16: expected FinalURLID 'url:audit:run:redirect:finalurlid:3', got %v", fetch1.FinalURLID)
		}
		// 17. FinalURLID never maps first hop as final
		if fetch1 != nil && fetch1.FinalURLID != nil && *fetch1.FinalURLID == "url:audit:run:redirect:finalurlid:2" {
			t.Errorf("17: FinalURLID incorrectly mapped first hop u2")
		}
		// 18. absent final target leaves FinalURLID nil
		if fetch4 == nil || fetch4.FinalURLID != nil {
			t.Errorf("18: expected FinalURLID to be nil for absent target, got %v", fetch4.FinalURLID)
		}
	})

	// 19. no final_status normalized observation
	t.Run("19. no final_status normalized observation", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:nofinalstatus"
		seed := "https://example.com/start"
		target := "https://example.com/final"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)
		insertURL(t, db, runID, 2, target)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     301,
			RedirectTo: target,
			Redirects: []sitecrawl.Hop{
				{URL: seed, Status: 301, Location: target},
			},
			CrawledAt: "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, 301, ?, 1, 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), target)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:nofinalstatus",
			SnapshotID: "snap:run:redirect:nofinalstatus",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.Field == "final_status" || obs.Field == "redirect_final_status" {
				t.Fatalf("forbidden normalized observation emitted: field %q, value %q", obs.Field, obs.Value)
			}
		}
	})

	// 20. existing 7 evaluators remain green
	t.Run("20. existing 7 evaluators remain green", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:redirect:eval7"
		seed := "https://example.com/"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:         seed,
			Status:      200,
			ContentType: "text/html",
			Canonical:   seed,
			Canonicals:  []string{seed},
			CrawledAt:   "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, canonical, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'text/html', ?, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), seed)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: "audit:run:redirect:eval7",
			SnapshotID: "snap:run:redirect:eval7",
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		eng, err := engine.New()
		if err != nil {
			t.Fatalf("engine.New failed: %v", err)
		}

		activeRules := []string{
			"AR-ACC-004",
			"AR-CANON-003",
			"AR-CANON-004",
			"AR-CANON-006",
			"AR-CANON-007",
			"AR-INDEX-001",
			"AR-INDEX-002",
		}

		for _, ruleID := range activeRules {
			results, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, ruleID)
			if err != nil {
				t.Errorf("rule %s failed to evaluate: %v", ruleID, err)
			}
			for _, evalRes := range results {
				if evalRes.Status == audit.StatusFail {
					t.Errorf("rule %s failed unexpectedly: status=%s, summary=%s", ruleID, evalRes.Status, evalRes.ObservedSummary)
				}
			}
		}
	})
}

// Hermetic integration test: runs real crawler through a multi-hop redirect chain,
// persists to real SQLite schema, builds frozen snapshot through adapter,
// and verifies redirect observations and FinalURLID.
func TestAdapter_HermeticRedirectChainIntegration(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/entry", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/intermediate", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/intermediate", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final-dest", http.StatusFound)
	})
	mux.HandleFunc("/final-dest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><head><title>Final Page</title></head><body><h1>Done</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	db := newTestDB(t)
	runner := sitecrawl.NewRunner(db)

	opts := sitecrawl.Options{
		Mode:            sitecrawl.ModeSpider,
		MaxDepth:        3,
		MaxURLs:         10,
		FollowRedirects: true,
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL + "/entry"}, opts)
	if err != nil {
		t.Fatalf("crawler runner.Crawl failed: %v", err)
	}

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: audit.AuditRunID("audit:" + started.ID),
		SnapshotID: audit.SnapshotID("snap:" + started.ID),
	})
	if err != nil {
		t.Fatalf("adapter.Build failed: %v", err)
	}

	// Locate /entry URL resource and its observations
	var entryURLID string
	var finalURLID string
	for _, ur := range res.UrlResources {
		if ur.URL == srv.URL+"/entry" {
			entryURLID = string(ur.URLID)
		}
		if ur.URL == srv.URL+"/final-dest" {
			finalURLID = string(ur.URLID)
		}
	}

	if entryURLID == "" {
		t.Fatalf("/entry URL resource not found in snapshot")
	}
	if finalURLID == "" {
		t.Fatalf("/final-dest URL resource not found in snapshot")
	}

	entryFields := getFieldsBySubj(res, entryURLID)
	if vals := entryFields["redirect_initial_observed"]; len(vals) != 1 || vals[0] != "true" {
		t.Errorf("expected redirect_initial_observed 'true', got %v", vals)
	}
	if vals := entryFields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "2" {
		t.Errorf("expected redirect_hop_count '2', got %v", vals)
	}
	if vals := entryFields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "true" {
		t.Errorf("expected redirect_traversal_complete 'true', got %v", vals)
	}
	if vals := entryFields["redirect_loop_detected"]; len(vals) != 1 || vals[0] != "false" {
		t.Errorf("expected redirect_loop_detected 'false', got %v", vals)
	}
	if vals := entryFields["redirect_final_url"]; len(vals) != 1 || vals[0] != srv.URL+"/final-dest" {
		t.Errorf("expected redirect_final_url %q, got %v", srv.URL+"/final-dest", vals)
	}

	// Verify FetchObservation.FinalURLID on /entry maps to /final-dest
	var entryFetch *audit.FetchObservation
	for i := range res.FetchObservations {
		if string(res.FetchObservations[i].URLID) == entryURLID {
			entryFetch = &res.FetchObservations[i]
			break
		}
	}

	if entryFetch == nil {
		t.Fatalf("entry fetch observation not found")
	}
	if entryFetch.FinalURLID == nil || string(*entryFetch.FinalURLID) != finalURLID {
		t.Errorf("entry fetch FinalURLID %v != finalURLID %s", entryFetch.FinalURLID, finalURLID)
	}

	// Verify typed RedirectHops have LocationRaw == "" and ResolvedTargetURL populated
	if len(res.RedirectHops) == 0 {
		t.Fatalf("expected typed RedirectHops, got 0")
	}
	for i, hop := range res.RedirectHops {
		if hop.LocationRaw != "" {
			t.Errorf("hop %d expected empty LocationRaw, got %q", i, hop.LocationRaw)
		}
		if hop.ResolvedTargetURL == "" {
			t.Errorf("hop %d expected non-empty ResolvedTargetURL", i)
		}
	}
}

// TestAdapter_RedirectIntegrityCases provides focused test coverage for the 12 requirements:
// 1. promoted redirect_hops equals chain length -> normal behavior unchanged
// 2. promoted redirect_hops differs from len(Page.Redirects) -> inconsistency detected
// 3. conflict does not emit traversal_complete=true
// 4. conflict does not emit loop=false
// 5. conflict withholds redirect_final_url
// 6. conflict leaves FinalURLID nil
// 7. conflict records EvidenceGap
// 8. typed RedirectHop.LocationRaw is empty
// 9. typed RedirectHop.ResolvedTargetURL is correct
// 10. hermetic normal redirect remains green
// 11. no final_status is introduced
// 12. existing 7 evaluators remain green
func TestAdapter_RedirectIntegrityCases(t *testing.T) {
	buildCase := func(t *testing.T, runID string, promotedHops int, hops []sitecrawl.Hop, status int, pageErr string) *adapter.BuildResult {
		t.Helper()
		db := newTestDB(t)
		seed := "https://example.com/start"
		target := "https://example.com/target"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)
		insertURL(t, db, runID, 2, target)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:        seed,
			Status:     status,
			RedirectTo: target,
			Redirects:  hops,
			Error:      pageErr,
			CrawledAt:  "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, redirect_to, redirect_hops, kind, crawled_at)
			VALUES(?, 1, ?, ?, ?, ?, ?, 'other', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), status, target, promotedHops)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: audit.AuditRunID("audit:" + runID),
			SnapshotID: audit.SnapshotID("snap:" + runID),
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}
		return res
	}

	oneHop := []sitecrawl.Hop{
		{URL: "https://example.com/start", Status: 301, Location: "https://example.com/target"},
	}

	t.Run("1. promoted redirect_hops equals chain length -> normal behavior unchanged", func(t *testing.T) {
		res := buildCase(t, "run:case1", 1, oneHop, 301, "")
		fields := getFieldsBySubj(res, "url:audit:run:case1:1")

		if vals := fields["redirect_initial_observed"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_initial_observed 'true', got %v", vals)
		}
		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "1" {
			t.Errorf("expected redirect_hop_count '1', got %v", vals)
		}
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected redirect_traversal_complete 'true', got %v", vals)
		}
		if vals := fields["redirect_loop_detected"]; len(vals) != 1 || vals[0] != "false" {
			t.Errorf("expected redirect_loop_detected 'false', got %v", vals)
		}
		if vals := fields["redirect_final_url"]; len(vals) != 1 || vals[0] != "https://example.com/target" {
			t.Errorf("expected redirect_final_url 'https://example.com/target', got %v", vals)
		}
		if len(res.FetchObservations) != 1 || res.FetchObservations[0].FinalURLID == nil || string(*res.FetchObservations[0].FinalURLID) != "url:audit:run:case1:2" {
			t.Errorf("expected FinalURLID to point to target, got %v", res.FetchObservations[0].FinalURLID)
		}
		for _, gap := range res.EvidenceGaps {
			if gap.GapCode == adapter.GapRedirectChainInconsistent {
				t.Errorf("unexpected GapRedirectChainInconsistent recorded on matching counts")
			}
		}
	})

	t.Run("2. promoted redirect_hops differs from len(Page.Redirects) -> inconsistency detected", func(t *testing.T) {
		res := buildCase(t, "run:case2", 5, oneHop, 301, "")
		var foundGap bool
		for _, gap := range res.EvidenceGaps {
			if gap.GapCode == adapter.GapRedirectChainInconsistent {
				foundGap = true
				if gap.SubjectRef != "url:audit:run:case2:1" {
					t.Errorf("expected SubjectRef 'url:audit:run:case2:1', got %q", gap.SubjectRef)
				}
				break
			}
		}
		if !foundGap {
			t.Errorf("expected inconsistency to be detected and recorded as EvidenceGap")
		}
	})

	t.Run("3. conflict does not emit traversal_complete=true", func(t *testing.T) {
		res := buildCase(t, "run:case3", 3, oneHop, 301, "")
		fields := getFieldsBySubj(res, "url:audit:run:case3:1")
		for _, v := range fields["redirect_traversal_complete"] {
			if v == "true" {
				t.Errorf("expected traversal_complete=true to NOT be emitted on conflict, got %v", fields["redirect_traversal_complete"])
			}
		}
		if len(fields["redirect_traversal_complete"]) != 0 {
			t.Errorf("expected redirect_traversal_complete to be withheld on conflict, got %v", fields["redirect_traversal_complete"])
		}
	})

	t.Run("4. conflict does not emit loop=false", func(t *testing.T) {
		res := buildCase(t, "run:case4", 3, oneHop, 301, "")
		fields := getFieldsBySubj(res, "url:audit:run:case4:1")
		for _, v := range fields["redirect_loop_detected"] {
			if v == "false" {
				t.Errorf("expected loop=false to NOT be emitted on conflict, got %v", fields["redirect_loop_detected"])
			}
		}
		if len(fields["redirect_loop_detected"]) != 0 {
			t.Errorf("expected redirect_loop_detected to be withheld on conflict, got %v", fields["redirect_loop_detected"])
		}
	})

	t.Run("5. conflict withholds redirect_final_url", func(t *testing.T) {
		res := buildCase(t, "run:case5", 3, oneHop, 301, "")
		fields := getFieldsBySubj(res, "url:audit:run:case5:1")
		if vals := fields["redirect_final_url"]; len(vals) != 0 {
			t.Errorf("expected redirect_final_url to be withheld on conflict, got %v", vals)
		}
	})

	t.Run("6. conflict leaves FinalURLID nil", func(t *testing.T) {
		res := buildCase(t, "run:case6", 3, oneHop, 301, "")
		if len(res.FetchObservations) != 1 {
			t.Fatalf("expected 1 fetch observation, got %d", len(res.FetchObservations))
		}
		if res.FetchObservations[0].FinalURLID != nil {
			t.Errorf("expected FinalURLID to remain nil on conflict, got %v", res.FetchObservations[0].FinalURLID)
		}
	})

	t.Run("7. conflict records EvidenceGap", func(t *testing.T) {
		res := buildCase(t, "run:case7", 0, oneHop, 301, "")
		var foundGap bool
		for _, gap := range res.EvidenceGaps {
			if gap.GapCode == adapter.GapRedirectChainInconsistent {
				foundGap = true
				if gap.Field != "redirect_hops" {
					t.Errorf("expected gap field 'redirect_hops', got %q", gap.Field)
				}
				if gap.SourceComponent != "sitecrawl_pages" {
					t.Errorf("expected sourceComponent 'sitecrawl_pages', got %q", gap.SourceComponent)
				}
				if gap.Reason == "" {
					t.Errorf("expected non-empty reason in EvidenceGap")
				}
				break
			}
		}
		if !foundGap {
			t.Errorf("expected GapRedirectChainInconsistent gap to be recorded")
		}
	})

	t.Run("8. typed RedirectHop.LocationRaw is empty", func(t *testing.T) {
		res := buildCase(t, "run:case8", 1, oneHop, 301, "")
		if len(res.RedirectHops) != 1 {
			t.Fatalf("expected 1 typed RedirectHop, got %d", len(res.RedirectHops))
		}
		if res.RedirectHops[0].LocationRaw != "" {
			t.Errorf("expected LocationRaw to be empty string, got %q", res.RedirectHops[0].LocationRaw)
		}
	})

	t.Run("9. typed RedirectHop.ResolvedTargetURL is correct", func(t *testing.T) {
		res := buildCase(t, "run:case9", 1, oneHop, 301, "")
		if len(res.RedirectHops) != 1 {
			t.Fatalf("expected 1 typed RedirectHop, got %d", len(res.RedirectHops))
		}
		expectedTarget := "https://example.com/target"
		if res.RedirectHops[0].ResolvedTargetURL != expectedTarget {
			t.Errorf("expected ResolvedTargetURL %q, got %q", expectedTarget, res.RedirectHops[0].ResolvedTargetURL)
		}
	})

	t.Run("10. hermetic normal redirect remains green", func(t *testing.T) {
		hops2 := []sitecrawl.Hop{
			{URL: "https://example.com/h0", Status: 301, Location: "https://example.com/h1"},
			{URL: "https://example.com/h1", Status: 302, Location: "https://example.com/target"},
		}
		res := buildCase(t, "run:case10", 2, hops2, 301, "")
		fields := getFieldsBySubj(res, "url:audit:run:case10:1")
		if vals := fields["redirect_traversal_complete"]; len(vals) != 1 || vals[0] != "true" {
			t.Errorf("expected traversal_complete 'true', got %v", vals)
		}
		if vals := fields["redirect_hop_count"]; len(vals) != 1 || vals[0] != "2" {
			t.Errorf("expected hop count '2', got %v", vals)
		}
		for i, hop := range res.RedirectHops {
			if hop.LocationRaw != "" {
				t.Errorf("hop %d expected empty LocationRaw, got %q", i, hop.LocationRaw)
			}
			if hop.ResolvedTargetURL == "" {
				t.Errorf("hop %d expected non-empty ResolvedTargetURL", i)
			}
		}
	})

	t.Run("11. no final_status is introduced", func(t *testing.T) {
		res := buildCase(t, "run:case11", 1, oneHop, 301, "")
		for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
			if obs.Field == "final_status" || obs.Field == "redirect_final_status" {
				t.Errorf("forbidden final_status observation detected: field=%s, value=%s", obs.Field, obs.Value)
			}
		}
	})

	t.Run("12. existing 7 evaluators remain green", func(t *testing.T) {
		db := newTestDB(t)
		runID := "run:case12:eval7"
		seed := "https://example.com/"
		setupTestRunWithOpts(t, db, runID, seed, sitecrawl.Options{FollowRedirects: true})
		insertURL(t, db, runID, 1, seed)

		page1, _ := json.Marshal(sitecrawl.Page{
			URL:         seed,
			Status:      200,
			ContentType: "text/html",
			Canonical:   seed,
			Canonicals:  []string{seed},
			CrawledAt:   "2026-10-01T00:00:10Z",
		})
		_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, status, content_type, canonical, kind, crawled_at)
			VALUES(?, 1, ?, ?, 200, 'text/html', ?, 'html', '2026-10-01T00:00:10Z')`, runID, seed, string(page1), seed)

		res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
			CrawlRunID: runID,
			AuditRunID: audit.AuditRunID("audit:" + runID),
			SnapshotID: audit.SnapshotID("snap:" + runID),
		})
		if err != nil {
			t.Fatalf("build failed: %v", err)
		}

		eng, err := engine.New()
		if err != nil {
			t.Fatalf("engine.New failed: %v", err)
		}

		activeRules := []string{
			"AR-ACC-004",
			"AR-CANON-003",
			"AR-CANON-004",
			"AR-CANON-006",
			"AR-CANON-007",
			"AR-INDEX-001",
			"AR-INDEX-002",
		}

		for _, ruleID := range activeRules {
			results, err := eng.EvaluateRule(context.Background(), res.EvidenceSnapshot, ruleID)
			if err != nil {
				t.Errorf("rule %s failed to evaluate: %v", ruleID, err)
			}
			for _, evalRes := range results {
				if evalRes.Status == audit.StatusFail {
					t.Errorf("rule %s failed unexpectedly: status=%s, summary=%s", ruleID, evalRes.Status, evalRes.ObservedSummary)
				}
			}
		}
	})
}

func getFieldsBySubj(res *adapter.BuildResult, subj string) map[string][]string {
	fields := make(map[string][]string)
	for _, obs := range res.EvidenceSnapshot.NormalizedObservations {
		if obs.SubjectRef == subj {
			fields[obs.Field] = append(fields[obs.Field], obs.Value)
		}
	}
	return fields
}
