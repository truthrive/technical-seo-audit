package adapter_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/adapter"
	"github.com/truthrive/technical-seo-audit/internal/sitecrawl"
)

func setupTestRun(t *testing.T, db *sql.DB, runID, seed string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO sitecrawl_runs(id, seed_url, host, options, state, started_at)
		VALUES(?, ?, 'example.com', '{}', 'completed', '2026-10-01T00:00:00Z')`, runID, seed)
	if err != nil {
		t.Fatalf("setup run failed: %v", err)
	}
}

func insertURL(t *testing.T, db *sql.DB, runID string, urlID int, u string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO sitecrawl_urls(run_id, id, url) VALUES(?, ?, ?)`, runID, urlID, u)
	if err != nil {
		t.Fatalf("insert url failed: %v", err)
	}
}

// 1. Generic robots meta: Target "*", ScopeUnknown false, safe effective_noindex
func TestAdapter_Directives_GenericRobotsMeta(t *testing.T) {
	db := newTestDB(t)
	runID := "run:generic:meta"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:generic:meta")
	snapID := audit.SnapshotID("snap:generic:meta")

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		MetaRobots: "noindex, follow",
		MetaTags: map[string]string{
			"robots": "noindex, follow",
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'noindex, follow', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 1 {
		t.Fatalf("expected 1 RobotsDirectiveObservation, got %d", len(res.RobotsDirectiveObservations))
	}
	obs := res.RobotsDirectiveObservations[0]
	if obs.Target != "*" {
		t.Errorf("expected Target '*', got %q", obs.Target)
	}
	if obs.ScopeUnknown {
		t.Errorf("expected ScopeUnknown false")
	}
	if obs.Source != audit.DirectiveSourceMeta {
		t.Errorf("expected Source META, got %q", obs.Source)
	}
	if obs.RawValue != "noindex, follow" {
		t.Errorf("expected RawValue 'noindex, follow', got %q", obs.RawValue)
	}
	if !reflect.DeepEqual(obs.ParsedTokens, []string{"noindex", "follow"}) {
		t.Errorf("unexpected ParsedTokens: %v", obs.ParsedTokens)
	}
	if !obs.EffectiveNoindex {
		t.Errorf("expected EffectiveNoindex true")
	}

	// Verify normalized observations in frozen snapshot
	snap := res.EvidenceSnapshot
	var (
		foundTarget         bool
		foundTokens         bool
		foundSource         bool
		foundEffectiveNoind bool
		foundScopeUnknown   bool
		effectiveNoindexVal string
	)
	for _, no := range snap.NormalizedObservations {
		if no.SubjectRef == "url:audit:generic:meta:1" {
			switch no.Field {
			case "directive_target":
				if no.Value == "*" {
					foundTarget = true
				}
				if len(no.SourceEvidenceRefs) == 0 || no.SourceEvidenceRefs[0] != string(obs.RobotsDirectiveObservationID) {
					t.Errorf("expected SourceEvidenceRefs to reference directive ID, got %v", no.SourceEvidenceRefs)
				}
			case "directive_tokens":
				if no.Value == "noindex, follow" {
					foundTokens = true
				}
			case "directive_source":
				if no.Value == "META" {
					foundSource = true
				}
			case "directive_scope_unknown":
				foundScopeUnknown = true
			case "effective_noindex":
				foundEffectiveNoind = true
				effectiveNoindexVal = no.Value
			}
		}
	}

	if !foundTarget || !foundTokens || !foundSource {
		t.Errorf("missing expected scoped directive normalized observations: target=%v, tokens=%v, source=%v",
			foundTarget, foundTokens, foundSource)
	}
	if foundScopeUnknown {
		t.Errorf("unexpected directive_scope_unknown for generic robots meta")
	}
	if !foundEffectiveNoind || effectiveNoindexVal != "true" {
		t.Errorf("expected effective_noindex 'true', got found=%v, val=%q", foundEffectiveNoind, effectiveNoindexVal)
	}

	// Verify zero scope ambiguity gaps
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDirectiveScopeAmbiguous {
			t.Errorf("unexpected GapDirectiveScopeAmbiguous: %+v", g)
		}
	}
}

// 2. Generic + Googlebot meta separation: distinct observations, no merging, safe effective_noindex
func TestAdapter_Directives_GenericAndGooglebotMetaSeparation(t *testing.T) {
	db := newTestDB(t)
	runID := "run:meta:separation"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:meta:sep")
	snapID := audit.SnapshotID("snap:meta:sep")

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		MetaRobots: "index, follow, noindex",
		MetaTags: map[string]string{
			"robots":    "index, follow",
			"googlebot": "noindex",
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'index, follow, noindex', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 2 {
		t.Fatalf("expected 2 separate RobotsDirectiveObservations, got %d", len(res.RobotsDirectiveObservations))
	}

	var genericObs, googlebotObs *audit.RobotsDirectiveObservation
	for i := range res.RobotsDirectiveObservations {
		o := &res.RobotsDirectiveObservations[i]
		if o.Target == "*" {
			genericObs = o
		} else if o.Target == "googlebot" {
			googlebotObs = o
		}
	}

	if genericObs == nil {
		t.Fatalf("generic target '*' observation not found")
	}
	if googlebotObs == nil {
		t.Fatalf("agent target 'googlebot' observation not found")
	}

	if !reflect.DeepEqual(genericObs.ParsedTokens, []string{"index", "follow"}) {
		t.Errorf("unexpected generic tokens: %v", genericObs.ParsedTokens)
	}
	if genericObs.EffectiveNoindex {
		t.Errorf("generic effective_noindex should be false for 'index, follow'")
	}

	if !reflect.DeepEqual(googlebotObs.ParsedTokens, []string{"noindex"}) {
		t.Errorf("unexpected googlebot tokens: %v", googlebotObs.ParsedTokens)
	}
	if googlebotObs.EffectiveNoindex {
		t.Errorf("googlebot observation EffectiveNoindex must remain false (unqualified noindex cannot come from agent-scoped evidence)")
	}

	// Verify URL-level effective_noindex is false (generic is index, not noindex)
	effectiveNoindexCount := 0
	effectiveNoindexVal := ""
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:meta:sep:1" {
			effectiveNoindexCount++
			effectiveNoindexVal = no.Value
		}
	}

	if effectiveNoindexCount != 1 || effectiveNoindexVal != "false" {
		t.Errorf("expected effective_noindex 'false', got count=%d, val=%q", effectiveNoindexCount, effectiveNoindexVal)
	}
}

// 3. Googlebot-only meta: effective_noindex withheld (generic evidence absent)
func TestAdapter_Directives_GooglebotOnlyMeta(t *testing.T) {
	db := newTestDB(t)
	runID := "run:gbot:only"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:gbot:only")
	snapID := audit.SnapshotID("snap:gbot:only")

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		MetaRobots: "noindex",
		MetaTags: map[string]string{
			"googlebot": "noindex",
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'noindex', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 1 {
		t.Fatalf("expected 1 RobotsDirectiveObservation, got %d", len(res.RobotsDirectiveObservations))
	}
	obs := res.RobotsDirectiveObservations[0]
	if obs.Target != "googlebot" {
		t.Errorf("expected Target 'googlebot', got %q", obs.Target)
	}
	if obs.ScopeUnknown {
		t.Errorf("expected ScopeUnknown false")
	}

	// Unqualified effective_noindex must NOT be emitted because generic directive evidence is absent
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:gbot:only:1" {
			t.Errorf("effective_noindex must be withheld when generic directive evidence is absent, got: %+v", no)
		}
	}
}

// 4. Legacy MetaRobots without scoped MetaTags: scope remains unknown, GapDirectiveScopeAmbiguous emitted, effective_noindex withheld
func TestAdapter_Directives_LegacyMetaRobotsWithoutScopedMetaTags(t *testing.T) {
	db := newTestDB(t)
	runID := "run:legacy:meta"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:legacy:meta")
	snapID := audit.SnapshotID("snap:legacy:meta")

	// Legacy data: MetaTags is nil/empty, only metaRobots exists
	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		MetaRobots: "noindex, follow",
		MetaTags:   nil,
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'noindex, follow', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 1 {
		t.Fatalf("expected 1 RobotsDirectiveObservation, got %d", len(res.RobotsDirectiveObservations))
	}
	obs := res.RobotsDirectiveObservations[0]
	if obs.Target != "" {
		t.Errorf("expected empty Target for unknown scope, got %q", obs.Target)
	}
	if !obs.ScopeUnknown {
		t.Errorf("expected ScopeUnknown true for legacy MetaRobots")
	}

	// GapDirectiveScopeAmbiguous must be emitted
	foundGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDirectiveScopeAmbiguous && g.SubjectRef == "url:audit:legacy:meta:1" {
			foundGap = true
		}
	}
	if !foundGap {
		t.Errorf("expected GapDirectiveScopeAmbiguous for legacy MetaRobots")
	}

	// effective_noindex must be withheld
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:legacy:meta:1" {
			t.Errorf("effective_noindex must be withheld when scope is unknown, got: %+v", no)
		}
	}
}

// 5. Generic X-Robots: Target "*", ScopeUnknown false, safe effective_noindex
func TestAdapter_Directives_GenericXRobots(t *testing.T) {
	db := newTestDB(t)
	runID := "run:generic:xrobots"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:generic:xr")
	snapID := audit.SnapshotID("snap:generic:xr")

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		XRobotsTag: "noindex, follow",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'noindex, follow', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 2 {
		t.Fatalf("expected 2 RobotsDirectiveObservations, got %d", len(res.RobotsDirectiveObservations))
	}
	for _, o := range res.RobotsDirectiveObservations {
		if o.Target != "*" {
			t.Errorf("expected Target '*', got %q", o.Target)
		}
		if o.ScopeUnknown {
			t.Errorf("expected ScopeUnknown false")
		}
		if o.Source != audit.DirectiveSourceHTTPHeader {
			t.Errorf("expected Source HTTP_HEADER, got %q", o.Source)
		}
	}

	// First segment has noindex
	if !res.RobotsDirectiveObservations[0].EffectiveNoindex {
		t.Errorf("expected EffectiveNoindex true for first segment")
	}

	// effective_noindex is "true"
	foundEff := false
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:generic:xr:1" {
			foundEff = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundEff {
		t.Errorf("expected effective_noindex observation")
	}
}

// 6. Parameterized directive: max-snippet: 50 is NOT confused with an agent prefix
func TestAdapter_Directives_MaxSnippetParameterized(t *testing.T) {
	db := newTestDB(t)
	runID := "run:param:directive"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:param:dir")
	snapID := audit.SnapshotID("snap:param:dir")

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		XRobotsTag: "max-snippet: 50",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'max-snippet: 50', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 1 {
		t.Fatalf("expected 1 RobotsDirectiveObservation, got %d", len(res.RobotsDirectiveObservations))
	}
	obs := res.RobotsDirectiveObservations[0]
	if obs.Target != "*" {
		t.Errorf("expected Target '*', got %q (parameterized directive must not be treated as agent prefix)", obs.Target)
	}
	if obs.ScopeUnknown {
		t.Errorf("expected ScopeUnknown false")
	}
	if !reflect.DeepEqual(obs.ParsedTokens, []string{"max-snippet: 50"}) {
		t.Errorf("unexpected ParsedTokens: %v", obs.ParsedTokens)
	}

	// effective_noindex is "false" (directive is generic, but not noindex)
	foundEff := false
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:param:dir:1" {
			foundEff = true
			if no.Value != "false" {
				t.Errorf("expected effective_noindex 'false', got %q", no.Value)
			}
		}
	}
	if !foundEff {
		t.Errorf("expected effective_noindex observation")
	}
}

// 7. Explicit googlebot: noindex header: Target "googlebot", ScopeUnknown false, effective_noindex withheld
func TestAdapter_Directives_ExplicitGooglebotXRobots(t *testing.T) {
	db := newTestDB(t)
	runID := "run:gbot:xrobots"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:gbot:xr")
	snapID := audit.SnapshotID("snap:gbot:xr")

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		XRobotsTag: "googlebot: noindex",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'googlebot: noindex', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 1 {
		t.Fatalf("expected 1 RobotsDirectiveObservation, got %d", len(res.RobotsDirectiveObservations))
	}
	obs := res.RobotsDirectiveObservations[0]
	if obs.Target != "googlebot" {
		t.Errorf("expected Target 'googlebot', got %q", obs.Target)
	}
	if obs.ScopeUnknown {
		t.Errorf("expected ScopeUnknown false")
	}
	if !reflect.DeepEqual(obs.ParsedTokens, []string{"noindex"}) {
		t.Errorf("unexpected ParsedTokens: %v", obs.ParsedTokens)
	}

	// effective_noindex must be withheld
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:gbot:xr:1" {
			t.Errorf("effective_noindex must be withheld when generic directive evidence is absent, got: %+v", no)
		}
	}
}

// 8. Ambiguous joined X-Robots value: googlebot: noindex, nofollow -> segment 1 does NOT inherit agent prefix
func TestAdapter_Directives_AmbiguousJoinedXRobots(t *testing.T) {
	db := newTestDB(t)
	runID := "run:ambig:xrobots"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:ambig:xr")
	snapID := audit.SnapshotID("snap:ambig:xr")

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		XRobotsTag: "googlebot: noindex, nofollow",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'googlebot: noindex, nofollow', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 2 {
		t.Fatalf("expected 2 RobotsDirectiveObservations, got %d", len(res.RobotsDirectiveObservations))
	}

	// Segment 0: proven googlebot prefix
	seg0 := res.RobotsDirectiveObservations[0]
	if seg0.Target != "googlebot" || seg0.ScopeUnknown {
		t.Errorf("seg0: expected Target 'googlebot', ScopeUnknown false, got target=%q, unknown=%v", seg0.Target, seg0.ScopeUnknown)
	}

	// Segment 1: cannot inherit googlebot across comma boundary, must remain unknown
	seg1 := res.RobotsDirectiveObservations[1]
	if seg1.Target != "" || !seg1.ScopeUnknown {
		t.Errorf("seg1: expected Target '', ScopeUnknown true, got target=%q, unknown=%v", seg1.Target, seg1.ScopeUnknown)
	}

	// GapDirectiveScopeAmbiguous emitted for segment 1
	foundGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDirectiveScopeAmbiguous && g.SubjectRef == "url:audit:ambig:xr:1" {
			foundGap = true
		}
	}
	if !foundGap {
		t.Errorf("expected GapDirectiveScopeAmbiguous for ambiguous segment 1")
	}

	// effective_noindex must be withheld because ambiguous directive evidence is present
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:ambig:xr:1" {
			t.Errorf("effective_noindex must be withheld when ambiguous directive evidence is present, got: %+v", no)
		}
	}
}

// 9. Safe and unsafe effective_noindex across multiple scenario URLs
func TestAdapter_Directives_SafeAndUnsafeEffectiveNoindex(t *testing.T) {
	db := newTestDB(t)
	runID := "run:eff:scenarios"
	seed := "https://example.com/1"
	setupTestRun(t, db, runID, seed)

	insertURL(t, db, runID, 1, "https://example.com/1")
	insertURL(t, db, runID, 2, "https://example.com/2")
	insertURL(t, db, runID, 3, "https://example.com/3")
	insertURL(t, db, runID, 4, "https://example.com/4")
	insertURL(t, db, runID, 5, "https://example.com/5")

	auditRunID := audit.AuditRunID("audit:eff:scenarios")
	snapID := audit.SnapshotID("snap:eff:scenarios")

	// URL 1: No directives -> effective_noindex NOT emitted (no directive evidence != false)
	p1, _ := json.Marshal(sitecrawl.Page{URL: "https://example.com/1", CrawledAt: "2026-10-01T00:00:10Z"})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, crawled_at)
		VALUES(?, 1, 'https://example.com/1', ?, 'html', '2026-10-01T00:00:10Z')`, runID, string(p1))

	// URL 2: Generic robots: noindex -> effective_noindex = "true"
	p2, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/2",
		MetaRobots: "noindex",
		MetaTags:   map[string]string{"robots": "noindex"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 2, 'https://example.com/2', ?, 'html', 'noindex', '2026-10-01T00:00:10Z')`, runID, string(p2))

	// URL 3: Generic robots: index + googlebot: noindex -> effective_noindex = "false" (not derived from googlebot)
	p3, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/3",
		MetaRobots: "index, noindex",
		MetaTags:   map[string]string{"robots": "index", "googlebot": "noindex"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 3, 'https://example.com/3', ?, 'html', 'index, noindex', '2026-10-01T00:00:10Z')`, runID, string(p3))

	// URL 4: googlebot: noindex only -> effective_noindex WITHHELD
	p4, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/4",
		MetaRobots: "noindex",
		MetaTags:   map[string]string{"googlebot": "noindex"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 4, 'https://example.com/4', ?, 'html', 'noindex', '2026-10-01T00:00:10Z')`, runID, string(p4))

	// URL 5: Generic robots: index + ambiguous X-Robots segment -> effective_noindex WITHHELD
	p5, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/5",
		MetaRobots: "index",
		MetaTags:   map[string]string{"robots": "index"},
		XRobotsTag: "googlebot: noindex, follow",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, x_robots, crawled_at)
		VALUES(?, 5, 'https://example.com/5', ?, 'html', 'index', 'googlebot: noindex, follow', '2026-10-01T00:00:10Z')`, runID, string(p5))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	effBySubj := make(map[string]string)
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" {
			effBySubj[no.SubjectRef] = no.Value
		}
	}

	// URL 1: must not have effective_noindex
	if v, exists := effBySubj["url:audit:eff:scenarios:1"]; exists {
		t.Errorf("URL 1: expected no effective_noindex observation, got %q", v)
	}

	// URL 2: effective_noindex = "true"
	if v, exists := effBySubj["url:audit:eff:scenarios:2"]; !exists || v != "true" {
		t.Errorf("URL 2: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// URL 3: effective_noindex = "false"
	if v, exists := effBySubj["url:audit:eff:scenarios:3"]; !exists || v != "false" {
		t.Errorf("URL 3: expected effective_noindex 'false', got exists=%v, val=%q", exists, v)
	}

	// URL 4: must not have effective_noindex
	if v, exists := effBySubj["url:audit:eff:scenarios:4"]; exists {
		t.Errorf("URL 4: expected no effective_noindex observation, got %q", v)
	}

	// URL 5: must not have effective_noindex
	if v, exists := effBySubj["url:audit:eff:scenarios:5"]; exists {
		t.Errorf("URL 5: expected no effective_noindex observation, got %q", v)
	}
}

// 10. Rendered page regression: raw meta suppressed, X-Robots preserved, safe effective_noindex
func TestAdapter_Directives_RenderedPageRegression(t *testing.T) {
	db := newTestDB(t)
	runID := "run:rendered:dir"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:rendered:dir")
	snapID := audit.SnapshotID("snap:rendered:dir")

	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		Rendered:   true,
		MetaRobots: "index, follow", // should be suppressed because page was rendered
		MetaTags:   map[string]string{"robots": "index, follow"},
		XRobotsTag: "noindex",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, rendered, meta_robots, x_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 1, 'index, follow', 'noindex', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Only HTTP header directive should be emitted
	if len(res.RobotsDirectiveObservations) != 1 {
		t.Fatalf("expected 1 RobotsDirectiveObservation (header only), got %d", len(res.RobotsDirectiveObservations))
	}
	obs := res.RobotsDirectiveObservations[0]
	if obs.Source != audit.DirectiveSourceHTTPHeader {
		t.Errorf("expected Source HTTP_HEADER, got %q", obs.Source)
	}
	if obs.Target != "*" {
		t.Errorf("expected Target '*', got %q", obs.Target)
	}
	if !obs.EffectiveNoindex {
		t.Errorf("expected EffectiveNoindex true")
	}

	// EffectiveNoindex observation = "true"
	foundEff := false
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:rendered:dir:1" {
			foundEff = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundEff {
		t.Errorf("expected effective_noindex observation")
	}
}

// 11. Deterministic output: repeated builds produce byte-for-byte identical directive and normalized observations
func TestAdapter_Directives_DeterministicOutput(t *testing.T) {
	db := newTestDB(t)
	runID := "run:determ:dir"
	seed := "https://example.com/1"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:determ:dir")
	snapID := audit.SnapshotID("snap:determ:dir")

	p1, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		MetaRobots: "index, follow, noindex",
		MetaTags:   map[string]string{"robots": "index, follow", "googlebot": "noindex"},
		XRobotsTag: "googlebot: max-snippet: 50, noarchive",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, x_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'index, follow, noindex', 'googlebot: max-snippet: 50, noarchive', '2026-10-01T00:00:10Z')`, runID, seed, string(p1))

	req := adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	}

	res1, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("build 1 failed: %v", err)
	}

	res2, err := adapter.Build(context.Background(), db, req)
	if err != nil {
		t.Fatalf("build 2 failed: %v", err)
	}

	if len(res1.RobotsDirectiveObservations) != len(res2.RobotsDirectiveObservations) {
		t.Fatalf("directive count mismatch: %d vs %d", len(res1.RobotsDirectiveObservations), len(res2.RobotsDirectiveObservations))
	}
	for i := range res1.RobotsDirectiveObservations {
		d1 := res1.RobotsDirectiveObservations[i]
		d2 := res2.RobotsDirectiveObservations[i]
		if d1.RobotsDirectiveObservationID != d2.RobotsDirectiveObservationID ||
			d1.Target != d2.Target ||
			d1.ScopeUnknown != d2.ScopeUnknown ||
			d1.RawValue != d2.RawValue ||
			d1.Source != d2.Source ||
			d1.EffectiveNoindex != d2.EffectiveNoindex {
			t.Errorf("directive observation %d mismatch: %+v vs %+v", i, d1, d2)
		}
	}

	if len(res1.EvidenceSnapshot.NormalizedObservations) != len(res2.EvidenceSnapshot.NormalizedObservations) {
		t.Fatalf("normalized observations count mismatch: %d vs %d",
			len(res1.EvidenceSnapshot.NormalizedObservations), len(res2.EvidenceSnapshot.NormalizedObservations))
	}
	for i := range res1.EvidenceSnapshot.NormalizedObservations {
		n1 := res1.EvidenceSnapshot.NormalizedObservations[i]
		n2 := res2.EvidenceSnapshot.NormalizedObservations[i]
		if n1.Field != n2.Field || n1.Value != n2.Value || n1.SubjectRef != n2.SubjectRef {
			t.Errorf("normalized observation %d mismatch: %+v vs %+v", i, n1, n2)
		}
	}
}

// 12. Hermetic crawl -> adapter -> snapshot proving robots and googlebot separation end-to-end
func TestAdapter_Directives_HermeticCrawlToSnapshot(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><html><head>
<title>Home</title>
<meta name="robots" content="index, follow">
<meta name="googlebot" content="noindex">
</head>
<body><nav><a href="/page2">Page 2</a></nav><h1>Home</h1></body></html>`)
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("X-Robots-Tag", "max-snippet: 100, noarchive")
		fmt.Fprintf(w, `<!doctype html><html><head><title>Page 2</title></head>
<body><h1>Page 2</h1></body></html>`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	db := newTestDB(t)
	runner := sitecrawl.NewRunner(db)

	opts := sitecrawl.Options{
		Mode:        sitecrawl.ModeSpider,
		MaxDepth:    2,
		MaxURLs:     10,
		Concurrency: 1,
	}

	started, err := runner.Crawl(context.Background(), []string{srv.URL}, opts)
	if err != nil {
		t.Fatalf("start crawl failed: %v", err)
	}

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: started.ID,
		AuditRunID: "audit:hermetic:crawl",
		SnapshotID: "snap:hermetic:crawl",
	})
	if err != nil {
		t.Fatalf("adapter build failed: %v", err)
	}

	snap := res.EvidenceSnapshot
	if snap == nil {
		t.Fatalf("expected non-nil EvidenceSnapshot")
	}

	// Verify Home page has separated robots and googlebot observations
	var homeDirectives []audit.RobotsDirectiveObservation
	for _, d := range res.RobotsDirectiveObservations {
		if d.URLID == "url:audit:hermetic:crawl:1" {
			homeDirectives = append(homeDirectives, d)
		}
	}
	if len(homeDirectives) != 2 {
		t.Fatalf("expected 2 directives for home page, got %d", len(homeDirectives))
	}

	var hasGeneric, hasGooglebot bool
	for _, d := range homeDirectives {
		if d.Target == "*" {
			hasGeneric = true
			if !reflect.DeepEqual(d.ParsedTokens, []string{"index", "follow"}) {
				t.Errorf("unexpected generic tokens: %v", d.ParsedTokens)
			}
		}
		if d.Target == "googlebot" {
			hasGooglebot = true
			if !reflect.DeepEqual(d.ParsedTokens, []string{"noindex"}) {
				t.Errorf("unexpected googlebot tokens: %v", d.ParsedTokens)
			}
		}
	}
	if !hasGeneric || !hasGooglebot {
		t.Errorf("expected both generic and googlebot directives for home page: generic=%v, googlebot=%v",
			hasGeneric, hasGooglebot)
	}

	// Verify effective_noindex for home page is "false" (not contaminated by googlebot)
	foundHomeEff := false
	for _, no := range snap.NormalizedObservations {
		if no.SubjectRef == "url:audit:hermetic:crawl:1" && no.Field == "effective_noindex" {
			foundHomeEff = true
			if no.Value != "false" {
				t.Errorf("home page effective_noindex should be 'false', got %q", no.Value)
			}
		}
	}
	if !foundHomeEff {
		t.Errorf("expected effective_noindex for home page")
	}

	// Verify Page 2 has generic parameterized max-snippet: 100 and noarchive
	var p2Directives []audit.RobotsDirectiveObservation
	for _, d := range res.RobotsDirectiveObservations {
		if d.URLID == "url:audit:hermetic:crawl:2" {
			p2Directives = append(p2Directives, d)
		}
	}
	if len(p2Directives) != 2 {
		t.Fatalf("expected 2 directives for page 2, got %d", len(p2Directives))
	}
	for _, d := range p2Directives {
		if d.Target != "*" {
			t.Errorf("page 2: expected Target '*', got %q", d.Target)
		}
		if d.ScopeUnknown {
			t.Errorf("page 2: expected ScopeUnknown false")
		}
	}
}

// 13. Flattened MetaRobots extra tokens: unrecovered tokens become unknown scope + GapDirectiveScopeAmbiguous
func TestAdapter_Directives_FlattenedMetaRobotsExtraTokens(t *testing.T) {
	db := newTestDB(t)
	runID := "run:flattened:extra"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:flat:extra")
	snapID := audit.SnapshotID("snap:flat:extra")

	// MetaTags has robots: "index", but MetaRobots has extra token "noarchive"
	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		MetaRobots: "index, noarchive",
		MetaTags: map[string]string{
			"robots": "index",
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'index, noarchive', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 2 {
		t.Fatalf("expected 2 RobotsDirectiveObservations (1 generic, 1 unknown extra), got %d", len(res.RobotsDirectiveObservations))
	}

	var genericObs, extraObs *audit.RobotsDirectiveObservation
	for i := range res.RobotsDirectiveObservations {
		o := &res.RobotsDirectiveObservations[i]
		if o.Target == "*" {
			genericObs = o
		} else if o.ScopeUnknown {
			extraObs = o
		}
	}

	if genericObs == nil {
		t.Fatalf("expected generic observation")
	}
	if extraObs == nil {
		t.Fatalf("expected unknown scope extra observation")
	}

	if !reflect.DeepEqual(extraObs.ParsedTokens, []string{"noarchive"}) {
		t.Errorf("unexpected extra tokens: %v", extraObs.ParsedTokens)
	}

	// GapDirectiveScopeAmbiguous emitted for extra tokens
	foundGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDirectiveScopeAmbiguous && g.SubjectRef == "url:audit:flat:extra:1" {
			foundGap = true
		}
	}
	if !foundGap {
		t.Errorf("expected GapDirectiveScopeAmbiguous for extra tokens")
	}

	// effective_noindex must be withheld because an ambiguous directive exists
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:flat:extra:1" {
			t.Errorf("effective_noindex must be withheld when ambiguous extra tokens exist, got: %+v", no)
		}
	}
}

