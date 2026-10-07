package adapter_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

	// In V1.3d: Googlebot-applicable noindex is proven by googlebot: noindex (effective_noindex = true)
	var foundEff1 bool
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:meta:sep:1" {
			foundEff1 = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundEff1 {
		t.Errorf("expected effective_noindex observation for url:audit:meta:sep:1")
	}

	// Verify URL-level raw fields separation (Fix 4)
	var (
		foundMetaRobotsRaw     bool
		metaRobotsRawVal       string
		foundGbotMetaRobotsRaw bool
		gbotMetaRobotsRawVal   string
	)
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.SubjectRef == "url:audit:meta:sep:1" {
			if no.Field == "meta_robots_raw" {
				foundMetaRobotsRaw = true
				metaRobotsRawVal = no.Value
			}
			if no.Field == "googlebot_meta_robots_raw" {
				foundGbotMetaRobotsRaw = true
				gbotMetaRobotsRawVal = no.Value
			}
			if no.Field == "x_robots_raw" {
				t.Errorf("unexpected x_robots_raw on meta-only page")
			}
		}
	}
	if !foundMetaRobotsRaw || metaRobotsRawVal != "index, follow" {
		t.Errorf("expected generic meta_robots_raw 'index, follow', got found=%v, val=%q", foundMetaRobotsRaw, metaRobotsRawVal)
	}
	if !foundGbotMetaRobotsRaw || gbotMetaRobotsRawVal != "noindex" {
		t.Errorf("expected googlebot_meta_robots_raw 'noindex', got found=%v, val=%q", foundGbotMetaRobotsRaw, gbotMetaRobotsRawVal)
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

	// In V1.3d: googlebot: noindex produces effective_noindex = "true"
	var foundEff2 bool
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:gbot:only:1" {
			foundEff2 = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundEff2 {
		t.Errorf("expected effective_noindex observation for url:audit:gbot:only:1")
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

	// In V1.3d: googlebot: noindex header produces effective_noindex = "true"
	var foundEff3 bool
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:gbot:xr:1" {
			foundEff3 = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundEff3 {
		t.Errorf("expected effective_noindex observation for url:audit:gbot:xr:1")
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

	// In V1.3d: known applicable noindex (googlebot: noindex) remains TRUE even with ambiguous segment
	var foundEff4 bool
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:ambig:xr:1" {
			foundEff4 = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundEff4 {
		t.Errorf("expected effective_noindex observation for url:audit:ambig:xr:1")
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
	insertURL(t, db, runID, 6, "https://example.com/6")

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

	// URL 3: Generic robots: index + googlebot: noindex -> effective_noindex WITHHELD (Fix 1: agent directive present)
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

	// URL 6: Generic robots: index alone -> effective_noindex = "false"
	p6, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/6",
		MetaRobots: "index",
		MetaTags:   map[string]string{"robots": "index"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 6, 'https://example.com/6', ?, 'html', 'index', '2026-10-01T00:00:10Z')`, runID, string(p6))

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

	// URL 3: effective_noindex = "true" (googlebot noindex is present)
	if v, exists := effBySubj["url:audit:eff:scenarios:3"]; !exists || v != "true" {
		t.Errorf("URL 3: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// URL 4: effective_noindex = "true" (googlebot noindex is present)
	if v, exists := effBySubj["url:audit:eff:scenarios:4"]; !exists || v != "true" {
		t.Errorf("URL 4: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// URL 5: effective_noindex = "true" (googlebot noindex segment is present)
	if v, exists := effBySubj["url:audit:eff:scenarios:5"]; !exists || v != "true" {
		t.Errorf("URL 5: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// URL 6: effective_noindex = "false"
	if v, exists := effBySubj["url:audit:eff:scenarios:6"]; !exists || v != "false" {
		t.Errorf("URL 6: expected effective_noindex 'false', got exists=%v, val=%q", exists, v)
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

	// In V1.3d: home page has googlebot: noindex, so effective_noindex = "true"
	var foundHomeEff bool
	for _, no := range snap.NormalizedObservations {
		if no.SubjectRef == "url:audit:hermetic:crawl:1" && no.Field == "effective_noindex" {
			foundHomeEff = true
			if no.Value != "true" {
				t.Errorf("expected home page effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundHomeEff {
		t.Errorf("expected effective_noindex observation for home page")
	}

	// Verify home page raw fields separation (Fix 4)
	var homeMetaRaw, homeGbotRaw string
	for _, no := range snap.NormalizedObservations {
		if no.SubjectRef == "url:audit:hermetic:crawl:1" {
			if no.Field == "meta_robots_raw" {
				homeMetaRaw = no.Value
			}
			if no.Field == "googlebot_meta_robots_raw" {
				homeGbotRaw = no.Value
			}
		}
	}
	if homeMetaRaw != "index, follow" {
		t.Errorf("home page meta_robots_raw: expected 'index, follow', got %q", homeMetaRaw)
	}
	if homeGbotRaw != "noindex" {
		t.Errorf("home page googlebot_meta_robots_raw: expected 'noindex', got %q", homeGbotRaw)
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

	// Page 2 has purely generic directives; effective_noindex is "false"
	foundP2Eff := false
	for _, no := range snap.NormalizedObservations {
		if no.SubjectRef == "url:audit:hermetic:crawl:2" && no.Field == "effective_noindex" {
			foundP2Eff = true
			if no.Value != "false" {
				t.Errorf("page 2 effective_noindex: expected 'false', got %q", no.Value)
			}
		}
	}
	if !foundP2Eff {
		t.Errorf("expected effective_noindex for page 2")
	}
}

// 13. Flattened MetaRobots extra tokens with both generic and agent scopes:
// unrecovered tokens cannot be assigned, keep unknown scope + GapDirectiveScopeAmbiguous (Fix 2)
func TestAdapter_Directives_FlattenedMetaRobotsExtraTokensBothScopes(t *testing.T) {
	db := newTestDB(t)
	runID := "run:flattened:both"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:flat:both")
	snapID := audit.SnapshotID("snap:flat:both")

	// MetaTags has robots: "index", googlebot: "noindex", but MetaRobots has extra token "noarchive"
	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		MetaRobots: "index, noindex, noarchive",
		MetaTags: map[string]string{
			"robots":    "index",
			"googlebot": "noindex",
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'index, noindex, noarchive', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 3 {
		t.Fatalf("expected 3 RobotsDirectiveObservations (1 generic, 1 agent, 1 unknown extra), got %d", len(res.RobotsDirectiveObservations))
	}

	var genericObs, googlebotObs, extraObs *audit.RobotsDirectiveObservation
	for i := range res.RobotsDirectiveObservations {
		o := &res.RobotsDirectiveObservations[i]
		if o.Target == "*" {
			genericObs = o
		} else if o.Target == "googlebot" {
			googlebotObs = o
		} else if o.ScopeUnknown {
			extraObs = o
		}
	}

	if genericObs == nil {
		t.Fatalf("expected generic observation")
	}
	if googlebotObs == nil {
		t.Fatalf("expected googlebot observation")
	}
	if extraObs == nil {
		t.Fatalf("expected unknown scope extra observation")
	}

	if !reflect.DeepEqual(extraObs.ParsedTokens, []string{"noarchive"}) {
		t.Errorf("unexpected extra tokens: %v", extraObs.ParsedTokens)
	}

	// GapDirectiveScopeAmbiguous emitted for extra tokens because both scopes exist and it cannot be assigned
	foundGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDirectiveScopeAmbiguous && g.SubjectRef == "url:audit:flat:both:1" {
			foundGap = true
		}
	}
	if !foundGap {
		t.Errorf("expected GapDirectiveScopeAmbiguous for unassignable extra tokens")
	}

	// In V1.3d: known Googlebot noindex remains "true" even when ambiguous extra tokens exist
	var foundFlatEff bool
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:flat:both:1" {
			foundFlatEff = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundFlatEff {
		t.Errorf("expected effective_noindex observation for url:audit:flat:both:1")
	}

	// Fix 4: meta_robots_raw contains proven generic ("index") only, not unrecovered or agent tokens
	var metaRobotsRawVal, gbotMetaRawVal string
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.SubjectRef == "url:audit:flat:both:1" {
			if no.Field == "meta_robots_raw" {
				metaRobotsRawVal = no.Value
			}
			if no.Field == "googlebot_meta_robots_raw" {
				gbotMetaRawVal = no.Value
			}
		}
	}
	if metaRobotsRawVal != "index" {
		t.Errorf("expected meta_robots_raw 'index', got %q", metaRobotsRawVal)
	}
	if gbotMetaRawVal != "noindex" {
		t.Errorf("expected googlebot_meta_robots_raw 'noindex', got %q", gbotMetaRawVal)
	}
}

// 14. Fix 2: Repeated generic meta where robots is the ONLY relevant scope present
// Extra flattened tokens remain generic scope, not unknown.
func TestAdapter_Directives_RepeatedGenericMetaPreservesGenericScope(t *testing.T) {
	db := newTestDB(t)
	runID := "run:generic:repeated"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:gen:rep")
	snapID := audit.SnapshotID("snap:gen:rep")

	// SiteCrawl collapsed: <meta name="robots" content="index"> + <meta name="robots" content="noindex">
	// MetaRobots: "index, noindex", MetaTags["robots"]: "noindex" (only last value retained by crawler map)
	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		MetaRobots: "index, noindex",
		MetaTags: map[string]string{
			"robots": "noindex",
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'index, noindex', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Both observations should be generic scope Target "*" with ScopeUnknown false
	if len(res.RobotsDirectiveObservations) != 2 {
		t.Fatalf("expected 2 RobotsDirectiveObservations, got %d", len(res.RobotsDirectiveObservations))
	}

	for i, o := range res.RobotsDirectiveObservations {
		if o.Target != "*" {
			t.Errorf("directive %d: expected Target '*', got %q", i, o.Target)
		}
		if o.ScopeUnknown {
			t.Errorf("directive %d: expected ScopeUnknown false (collapsed generic tags remain generic)", i)
		}
	}

	// Zero GapDirectiveScopeAmbiguous gaps should be emitted
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDirectiveScopeAmbiguous && g.SubjectRef == "url:audit:gen:rep:1" {
			t.Errorf("unexpected GapDirectiveScopeAmbiguous for repeated generic meta: %+v", g)
		}
	}

	// EffectiveNoindex is "true" because noindex was present in generic directives and all evidence is known generic
	foundEff := false
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:gen:rep:1" {
			foundEff = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundEff {
		t.Errorf("expected effective_noindex observation")
	}

	// meta_robots_raw contains the full flattened generic raw value ("index, noindex")
	var foundMetaRaw bool
	var metaRawVal string
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.SubjectRef == "url:audit:gen:rep:1" && no.Field == "meta_robots_raw" {
			foundMetaRaw = true
			metaRawVal = no.Value
		}
	}
	if !foundMetaRaw || metaRawVal != "index, noindex" {
		t.Errorf("expected meta_robots_raw 'index, noindex', got found=%v, val=%q", foundMetaRaw, metaRawVal)
	}
}

// 15. Fix 3: Unknown X-Robots prefixes (e.g. foo: bar) must NOT be classified as agent directives.
// They must remain conservative / unknown scope.
func TestAdapter_Directives_UnknownXRobotsPrefixRemainsUnknownScope(t *testing.T) {
	db := newTestDB(t)
	runID := "run:prefix:unknown"
	seed := "https://example.com/"
	setupTestRun(t, db, runID, seed)
	insertURL(t, db, runID, 1, seed)

	auditRunID := audit.AuditRunID("audit:pfx:unk")
	snapID := audit.SnapshotID("snap:pfx:unk")

	// Header contains initial generic "max-snippet: 50", unknown prefix "foo: bar", and supported agent "googlebot: noindex"
	pageData, _ := json.Marshal(sitecrawl.Page{
		URL:        seed,
		XRobotsTag: "max-snippet: 50, foo: bar, googlebot: noindex",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 1, ?, ?, 'html', 'max-snippet: 50, foo: bar, googlebot: noindex', '2026-10-01T00:00:10Z')`, runID, seed, string(pageData))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	if len(res.RobotsDirectiveObservations) != 3 {
		t.Fatalf("expected 3 RobotsDirectiveObservations, got %d", len(res.RobotsDirectiveObservations))
	}

	// Observation 0: "max-snippet: 50" -> Target "*", ScopeUnknown false
	obs0 := res.RobotsDirectiveObservations[0]
	if obs0.Target != "*" || obs0.ScopeUnknown {
		t.Errorf("obs0: expected Target '*', ScopeUnknown false, got %q, %v", obs0.Target, obs0.ScopeUnknown)
	}

	// Observation 1: "foo: bar" -> must NOT have Target "foo", must be unknown scope
	obs1 := res.RobotsDirectiveObservations[1]
	if obs1.Target != "" {
		t.Errorf("obs1: expected empty Target, got %q (must not invent 'foo' as an agent)", obs1.Target)
	}
	if !obs1.ScopeUnknown {
		t.Errorf("obs1: expected ScopeUnknown true for unrecognized prefix")
	}

	// Observation 2: "googlebot: noindex" -> Target "googlebot", ScopeUnknown false
	obs2 := res.RobotsDirectiveObservations[2]
	if obs2.Target != "googlebot" || obs2.ScopeUnknown {
		t.Errorf("obs2: expected Target 'googlebot', ScopeUnknown false, got %q, %v", obs2.Target, obs2.ScopeUnknown)
	}

	// GapDirectiveScopeAmbiguous must be emitted for foo: bar
	foundGap := false
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDirectiveScopeAmbiguous && g.SubjectRef == "url:audit:pfx:unk:1" {
			foundGap = true
		}
	}
	if !foundGap {
		t.Errorf("expected GapDirectiveScopeAmbiguous for unknown prefix")
	}

	// Fix 4: x_robots_raw contains proven generic header evidence ONLY ("max-snippet: 50")
	// Neither foo: bar nor googlebot: noindex must appear in x_robots_raw!
	var foundXRobotsRaw bool
	var xRobotsRawVal string
	var foundGbotXRaw bool
	var gbotXRawVal string
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.SubjectRef == "url:audit:pfx:unk:1" {
			if no.Field == "x_robots_raw" {
				foundXRobotsRaw = true
				xRobotsRawVal = no.Value
			}
			if no.Field == "googlebot_x_robots_raw" {
				foundGbotXRaw = true
				gbotXRawVal = no.Value
			}
		}
	}
	if !foundXRobotsRaw || xRobotsRawVal != "max-snippet: 50" {
		t.Errorf("expected generic x_robots_raw 'max-snippet: 50', got found=%v, val=%q", foundXRobotsRaw, xRobotsRawVal)
	}
	if !foundGbotXRaw || gbotXRawVal != "googlebot: noindex" {
		t.Errorf("expected googlebot_x_robots_raw 'googlebot: noindex', got found=%v, val=%q", foundGbotXRaw, gbotXRawVal)
	}

	// In V1.3d: known Googlebot noindex remains "true" even with unknown prefix
	var foundPfxEff bool
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" && no.SubjectRef == "url:audit:pfx:unk:1" {
			foundPfxEff = true
			if no.Value != "true" {
				t.Errorf("expected effective_noindex 'true', got %q", no.Value)
			}
		}
	}
	if !foundPfxEff {
		t.Errorf("expected effective_noindex observation for url:audit:pfx:unk:1")
	}
}

// 16. Fix 4: Strict separation of generic vs agent-scoped vs unknown-scope raw fields.
func TestAdapter_Directives_RawNormalizedFieldsSeparation(t *testing.T) {
	db := newTestDB(t)
	runID := "run:raw:separation"
	seed := "https://example.com/1"
	setupTestRun(t, db, runID, seed)

	insertURL(t, db, runID, 1, "https://example.com/1")
	insertURL(t, db, runID, 2, "https://example.com/2")
	insertURL(t, db, runID, 3, "https://example.com/3")

	auditRunID := audit.AuditRunID("audit:raw:sep")
	snapID := audit.SnapshotID("snap:raw:sep")

	// URL 1: Multi-agent X-Robots header
	// X-Robots-Tag: "noindex, googlebot: nofollow, oai-searchbot: noarchive, gptbot: unavailable_after, unknownprefix: test"
	p1, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/1",
		XRobotsTag: "noindex, googlebot: nofollow, oai-searchbot: noarchive, gptbot: unavailable_after, unknownprefix: test",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 1, 'https://example.com/1', ?, 'html', 'noindex, googlebot: nofollow, oai-searchbot: noarchive, gptbot: unavailable_after, unknownprefix: test', '2026-10-01T00:00:10Z')`,
		runID, string(p1))

	// URL 2: Meta with robots and googlebot
	p2, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/2",
		MetaRobots: "index, follow, noarchive",
		MetaTags: map[string]string{
			"robots":    "index, follow",
			"googlebot": "noarchive",
		},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 2, 'https://example.com/2', ?, 'html', 'index, follow, noarchive', '2026-10-01T00:00:10Z')`,
		runID, string(p2))

	// URL 3: Legacy meta without MetaTags (scope unknown)
	p3, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/3",
		MetaRobots: "noindex",
		MetaTags:   nil,
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 3, 'https://example.com/3', ?, 'html', 'noindex', '2026-10-01T00:00:10Z')`,
		runID, string(p3))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	rawFieldsBySubj := make(map[string]map[string]string)
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if strings.HasSuffix(no.Field, "_raw") {
			if _, ok := rawFieldsBySubj[no.SubjectRef]; !ok {
				rawFieldsBySubj[no.SubjectRef] = make(map[string]string)
			}
			rawFieldsBySubj[no.SubjectRef][no.Field] = no.Value
		}
	}

	// URL 1:
	// x_robots_raw must contain "noindex" only.
	// googlebot_x_robots_raw must contain "googlebot: nofollow".
	// oai-searchbot_x_robots_raw must contain "oai-searchbot: noarchive".
	// gptbot_x_robots_raw must contain "gptbot: unavailable_after".
	// unknownprefix must NOT be in any field.
	// meta_robots_raw must not be emitted.
	u1Fields := rawFieldsBySubj["url:audit:raw:sep:1"]
	if u1Fields["x_robots_raw"] != "noindex" {
		t.Errorf("URL 1 x_robots_raw: expected 'noindex', got %q", u1Fields["x_robots_raw"])
	}
	if u1Fields["googlebot_x_robots_raw"] != "googlebot: nofollow" {
		t.Errorf("URL 1 googlebot_x_robots_raw: expected 'googlebot: nofollow', got %q", u1Fields["googlebot_x_robots_raw"])
	}
	if u1Fields["oai-searchbot_x_robots_raw"] != "oai-searchbot: noarchive" {
		t.Errorf("URL 1 oai-searchbot_x_robots_raw: expected 'oai-searchbot: noarchive', got %q", u1Fields["oai-searchbot_x_robots_raw"])
	}
	if u1Fields["gptbot_x_robots_raw"] != "gptbot: unavailable_after" {
		t.Errorf("URL 1 gptbot_x_robots_raw: expected 'gptbot: unavailable_after', got %q", u1Fields["gptbot_x_robots_raw"])
	}
	if _, exists := u1Fields["meta_robots_raw"]; exists {
		t.Errorf("URL 1: meta_robots_raw should not exist")
	}

	// URL 2:
	// meta_robots_raw must contain "index, follow" only (generic).
	// googlebot_meta_robots_raw must contain "noarchive".
	// x_robots_raw must not be emitted.
	u2Fields := rawFieldsBySubj["url:audit:raw:sep:2"]
	if u2Fields["meta_robots_raw"] != "index, follow" {
		t.Errorf("URL 2 meta_robots_raw: expected 'index, follow', got %q", u2Fields["meta_robots_raw"])
	}
	if u2Fields["googlebot_meta_robots_raw"] != "noarchive" {
		t.Errorf("URL 2 googlebot_meta_robots_raw: expected 'noarchive', got %q", u2Fields["googlebot_meta_robots_raw"])
	}
	if _, exists := u2Fields["x_robots_raw"]; exists {
		t.Errorf("URL 2: x_robots_raw should not exist")
	}

	// URL 3:
	// Legacy meta without scoped tags: meta_robots_raw must NOT be emitted (unknown-scope raw evidence must not appear inside generic fields).
	u3Fields := rawFieldsBySubj["url:audit:raw:sep:3"]
	if _, exists := u3Fields["meta_robots_raw"]; exists {
		t.Errorf("URL 3: meta_robots_raw must NOT appear for legacy unknown scope, got %q", u3Fields["meta_robots_raw"])
	}
}

// 17. Fix 1: X-Robots continuation scope after agent prefix or unknown prefix
// Verifies:
// 1. standalone max-snippet: 50 -> generic
// 2. googlebot: noindex, max-snippet: 50 -> second segment unknown
// 3. explicit new agent prefix after agent prefix remains scoped (oai-searchbot: noarchive)
// 4. ambiguity emits GapDirectiveScopeAmbiguous
// 5. ambiguous segment does not enter generic x_robots_raw
// 6. conservative continuation after unknown prefix-like segment (foo: bar, max-snippet: 50)
func TestAdapter_Directives_XRobotsContinuationScopeAmbiguity(t *testing.T) {
	db := newTestDB(t)
	runID := "run:xr:continuation"
	seed := "https://example.com/1"
	setupTestRun(t, db, runID, seed)

	insertURL(t, db, runID, 1, "https://example.com/1")
	insertURL(t, db, runID, 2, "https://example.com/2")
	insertURL(t, db, runID, 3, "https://example.com/3")

	auditRunID := audit.AuditRunID("audit:xr:cont")
	snapID := audit.SnapshotID("snap:xr:cont")

	// URL 1: Continuation after agent prefix: "googlebot: noindex, max-snippet: 50, oai-searchbot: noarchive"
	// Expected:
	// - segment 0 ("googlebot: noindex") -> target "googlebot", scopeUnknown false
	// - segment 1 ("max-snippet: 50") -> target "", scopeUnknown true (must NOT be generic!)
	// - segment 2 ("oai-searchbot: noarchive") -> target "oai-searchbot", scopeUnknown false
	p1, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/1",
		XRobotsTag: "googlebot: noindex, max-snippet: 50, oai-searchbot: noarchive",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 1, 'https://example.com/1', ?, 'html', 'googlebot: noindex, max-snippet: 50, oai-searchbot: noarchive', '2026-10-01T00:00:10Z')`,
		runID, string(p1))

	// URL 2: Continuation after unknown prefix: "foo: bar, max-snippet: 50"
	// Expected:
	// - segment 0 ("foo: bar") -> target "", scopeUnknown true
	// - segment 1 ("max-snippet: 50") -> target "", scopeUnknown true (scope cannot be proven generic!)
	p2, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/2",
		XRobotsTag: "foo: bar, max-snippet: 50",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 2, 'https://example.com/2', ?, 'html', 'foo: bar, max-snippet: 50', '2026-10-01T00:00:10Z')`,
		runID, string(p2))

	// URL 3: Standalone parameterized directive: "max-snippet: 50"
	// Expected:
	// - segment 0 ("max-snippet: 50") -> target "*", scopeUnknown false (remains generic)
	p3, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/3",
		XRobotsTag: "max-snippet: 50",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 3, 'https://example.com/3', ?, 'html', 'max-snippet: 50', '2026-10-01T00:00:10Z')`,
		runID, string(p3))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// 1. Inspect URL 1 directives
	var u1Directives []audit.RobotsDirectiveObservation
	for _, d := range res.RobotsDirectiveObservations {
		if d.URLID == "url:audit:xr:cont:1" {
			u1Directives = append(u1Directives, d)
		}
	}
	if len(u1Directives) != 3 {
		t.Fatalf("URL 1: expected 3 directives, got %d", len(u1Directives))
	}
	// seg 0: googlebot: noindex
	if u1Directives[0].Target != "googlebot" || u1Directives[0].ScopeUnknown {
		t.Errorf("URL 1 seg 0: expected googlebot / false, got %q / %v", u1Directives[0].Target, u1Directives[0].ScopeUnknown)
	}
	// seg 1: max-snippet: 50 -> must be UNKNOWN scope!
	if u1Directives[1].Target != "" || !u1Directives[1].ScopeUnknown {
		t.Errorf("URL 1 seg 1: expected empty target and ScopeUnknown true, got %q / %v", u1Directives[1].Target, u1Directives[1].ScopeUnknown)
	}
	// seg 2: oai-searchbot: noarchive -> explicit new agent prefix remains scoped
	if u1Directives[2].Target != "oai-searchbot" || u1Directives[2].ScopeUnknown {
		t.Errorf("URL 1 seg 2: expected oai-searchbot / false, got %q / %v", u1Directives[2].Target, u1Directives[2].ScopeUnknown)
	}

	// 2. Inspect URL 2 directives (unknown prefix continuation)
	var u2Directives []audit.RobotsDirectiveObservation
	for _, d := range res.RobotsDirectiveObservations {
		if d.URLID == "url:audit:xr:cont:2" {
			u2Directives = append(u2Directives, d)
		}
	}
	if len(u2Directives) != 2 {
		t.Fatalf("URL 2: expected 2 directives, got %d", len(u2Directives))
	}
	if u2Directives[0].Target != "" || !u2Directives[0].ScopeUnknown {
		t.Errorf("URL 2 seg 0 (foo: bar): expected unknown scope, got %q / %v", u2Directives[0].Target, u2Directives[0].ScopeUnknown)
	}
	if u2Directives[1].Target != "" || !u2Directives[1].ScopeUnknown {
		t.Errorf("URL 2 seg 1 (max-snippet: 50): expected unknown scope after unknown prefix, got %q / %v", u2Directives[1].Target, u2Directives[1].ScopeUnknown)
	}

	// 3. Inspect URL 3 directives (standalone max-snippet: 50 remains generic)
	var u3Directives []audit.RobotsDirectiveObservation
	for _, d := range res.RobotsDirectiveObservations {
		if d.URLID == "url:audit:xr:cont:3" {
			u3Directives = append(u3Directives, d)
		}
	}
	if len(u3Directives) != 1 {
		t.Fatalf("URL 3: expected 1 directive, got %d", len(u3Directives))
	}
	if u3Directives[0].Target != "*" || u3Directives[0].ScopeUnknown {
		t.Errorf("URL 3: expected standalone max-snippet: 50 to have Target '*' and ScopeUnknown false, got %q / %v", u3Directives[0].Target, u3Directives[0].ScopeUnknown)
	}

	// 4. Verify gaps emitted
	var u1Gaps, u2Gaps, u3Gaps []adapter.EvidenceGap
	for _, g := range res.EvidenceGaps {
		if g.GapCode == adapter.GapDirectiveScopeAmbiguous {
			switch g.SubjectRef {
			case "url:audit:xr:cont:1":
				u1Gaps = append(u1Gaps, g)
			case "url:audit:xr:cont:2":
				u2Gaps = append(u2Gaps, g)
			case "url:audit:xr:cont:3":
				u3Gaps = append(u3Gaps, g)
			}
		}
	}
	if len(u1Gaps) != 1 {
		t.Errorf("URL 1: expected 1 GapDirectiveScopeAmbiguous for max-snippet: 50, got %d", len(u1Gaps))
	}
	if len(u2Gaps) != 2 {
		t.Errorf("URL 2: expected 2 GapDirectiveScopeAmbiguous (foo: bar and max-snippet: 50), got %d", len(u2Gaps))
	}
	if len(u3Gaps) != 0 {
		t.Errorf("URL 3: expected 0 GapDirectiveScopeAmbiguous for standalone generic directive, got %d", len(u3Gaps))
	}

	// 5. Verify raw normalized observations
	// For URL 1: ambiguous max-snippet: 50 MUST NOT enter generic x_robots_raw!
	// x_robots_raw must not exist on URL 1.
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.SubjectRef == "url:audit:xr:cont:1" && no.Field == "x_robots_raw" {
			t.Errorf("URL 1: ambiguous segment must NOT enter generic x_robots_raw, found: %q", no.Value)
		}
		if no.SubjectRef == "url:audit:xr:cont:2" && no.Field == "x_robots_raw" {
			t.Errorf("URL 2: ambiguous segment must NOT enter generic x_robots_raw, found: %q", no.Value)
		}
	}

	// URL 1 should have googlebot_x_robots_raw and oai-searchbot_x_robots_raw
	var foundGbotRaw, foundOaiRaw bool
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.SubjectRef == "url:audit:xr:cont:1" {
			if no.Field == "googlebot_x_robots_raw" && no.Value == "googlebot: noindex" {
				foundGbotRaw = true
			}
			if no.Field == "oai-searchbot_x_robots_raw" && no.Value == "oai-searchbot: noarchive" {
				foundOaiRaw = true
			}
		}
	}
	if !foundGbotRaw {
		t.Errorf("URL 1: missing googlebot_x_robots_raw")
	}
	if !foundOaiRaw {
		t.Errorf("URL 1: missing oai-searchbot_x_robots_raw")
	}

	// URL 3 should have generic x_robots_raw = "max-snippet: 50"
	var foundU3XRaw bool
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.SubjectRef == "url:audit:xr:cont:3" && no.Field == "x_robots_raw" {
			foundU3XRaw = true
			if no.Value != "max-snippet: 50" {
				t.Errorf("URL 3: expected x_robots_raw 'max-snippet: 50', got %q", no.Value)
			}
		}
	}
	if !foundU3XRaw {
		t.Errorf("URL 3: missing generic x_robots_raw")
	}
}

// 18. V1.3d: Googlebot Effective Noindex Normalization
// Comprehensive verification of all 14 required cases for URL-level effective_noindex.
func TestAdapter_GooglebotEffectiveNoindex_Normalization(t *testing.T) {
	db := newTestDB(t)
	runID := "run:gbot:eff:norm"
	seed := "https://example.com/1"
	setupTestRun(t, db, runID, seed)

	// Register 15 distinct URLs for the test cases
	for i := 1; i <= 15; i++ {
		insertURL(t, db, runID, i, fmt.Sprintf("https://example.com/%d", i))
	}

	auditRunID := audit.AuditRunID("audit:gbot:eff:norm")
	snapID := audit.SnapshotID("snap:gbot:eff:norm")

	// 1. generic noindex -> true
	p1, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/1",
		MetaRobots: "noindex",
		MetaTags:   map[string]string{"robots": "noindex"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 1, 'https://example.com/1', ?, 'html', 'noindex', '2026-10-01T00:00:10Z')`, runID, string(p1))

	// 2. googlebot noindex -> true
	p2, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/2",
		MetaRobots: "noindex",
		MetaTags:   map[string]string{"googlebot": "noindex"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 2, 'https://example.com/2', ?, 'html', 'noindex', '2026-10-01T00:00:10Z')`, runID, string(p2))

	// 3. generic index -> false
	p3, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/3",
		MetaRobots: "index",
		MetaTags:   map[string]string{"robots": "index"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 3, 'https://example.com/3', ?, 'html', 'index', '2026-10-01T00:00:10Z')`, runID, string(p3))

	// 4. googlebot index -> false
	p4, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/4",
		MetaRobots: "index",
		MetaTags:   map[string]string{"googlebot": "index"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 4, 'https://example.com/4', ?, 'html', 'index', '2026-10-01T00:00:10Z')`, runID, string(p4))

	// 5. generic index + googlebot noindex -> true
	p5, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/5",
		MetaRobots: "index, noindex",
		MetaTags:   map[string]string{"robots": "index", "googlebot": "noindex"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 5, 'https://example.com/5', ?, 'html', 'index, noindex', '2026-10-01T00:00:10Z')`, runID, string(p5))

	// 6. generic noindex + googlebot index -> true
	p6, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/6",
		MetaRobots: "noindex, index",
		MetaTags:   map[string]string{"robots": "noindex", "googlebot": "index"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 6, 'https://example.com/6', ?, 'html', 'noindex, index', '2026-10-01T00:00:10Z')`, runID, string(p6))

	// 7. generic index + GPTBot noindex -> false
	p7, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/7",
		MetaRobots: "index",
		MetaTags:   map[string]string{"robots": "index", "gptbot": "noindex"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 7, 'https://example.com/7', ?, 'html', 'index', '2026-10-01T00:00:10Z')`, runID, string(p7))

	// 8. none -> true
	p8, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/8",
		MetaRobots: "none",
		MetaTags:   map[string]string{"robots": "none"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 8, 'https://example.com/8', ?, 'html', 'none', '2026-10-01T00:00:10Z')`, runID, string(p8))

	// 9. index + noindex same scope -> true
	p9, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/9",
		MetaRobots: "index, noindex",
		MetaTags:   map[string]string{"robots": "index, noindex"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, meta_robots, crawled_at)
		VALUES(?, 9, 'https://example.com/9', ?, 'html', 'index, noindex', '2026-10-01T00:00:10Z')`, runID, string(p9))

	// 10. unknown scope without known noindex -> effective_noindex absent
	p10, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/10",
		XRobotsTag: "foo: bar, index",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 10, 'https://example.com/10', ?, 'html', 'foo: bar, index', '2026-10-01T00:00:10Z')`, runID, string(p10))

	// 11. known Googlebot noindex + unknown scope -> true
	p11, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/11",
		XRobotsTag: "googlebot: noindex, foo: bar",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 11, 'https://example.com/11', ?, 'html', 'googlebot: noindex, foo: bar', '2026-10-01T00:00:10Z')`, runID, string(p11))

	// 12. no-directive complete evidence -> false
	p12, _ := json.Marshal(sitecrawl.Page{
		URL:       "https://example.com/12",
		MetaTags:  map[string]string{},
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, crawled_at)
		VALUES(?, 12, 'https://example.com/12', ?, 'html', '2026-10-01T00:00:10Z')`, runID, string(p12))

	// 13a. incomplete/raw-render evidence -> effective_noindex absent (rendered page, no header noindex)
	p13a, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/13",
		Rendered:   true,
		MetaRobots: "index",
		MetaTags:   map[string]string{"robots": "index"},
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, rendered, meta_robots, crawled_at)
		VALUES(?, 13, 'https://example.com/13', ?, 'html', 1, 'index', '2026-10-01T00:00:10Z')`, runID, string(p13a))

	// 13b. incomplete/raw-render evidence -> effective_noindex absent (MetaTags nil, no directives)
	p13b, _ := json.Marshal(sitecrawl.Page{
		URL:       "https://example.com/14",
		MetaTags:  nil,
		CrawledAt: "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, crawled_at)
		VALUES(?, 14, 'https://example.com/14', ?, 'html', '2026-10-01T00:00:10Z')`, runID, string(p13b))

	// URL 15: only unrelated GPTBot directive with complete evidence (MetaTags non-nil) -> false
	p15, _ := json.Marshal(sitecrawl.Page{
		URL:        "https://example.com/15",
		MetaTags:   map[string]string{"description": "A page with no Googlebot directives"},
		XRobotsTag: "gptbot: noindex",
		CrawledAt:  "2026-10-01T00:00:10Z",
	})
	_, _ = db.Exec(`INSERT INTO sitecrawl_pages(run_id, url_id, url, data, kind, x_robots, crawled_at)
		VALUES(?, 15, 'https://example.com/15', ?, 'html', 'gptbot: noindex', '2026-10-01T00:00:10Z')`, runID, string(p15))

	res, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}

	// Verify snapshot NormalizationVersion is bumped to v1.4.0
	if res.EvidenceSnapshot.NormalizationVersion != "v1.4.0" {
		t.Errorf("expected NormalizationVersion 'v1.4.0', got %q", res.EvidenceSnapshot.NormalizationVersion)
	}

	effBySubj := make(map[string]string)
	for _, no := range res.EvidenceSnapshot.NormalizedObservations {
		if no.Field == "effective_noindex" {
			effBySubj[no.SubjectRef] = no.Value
		}
	}

	// 1. generic noindex -> true
	if v, exists := effBySubj["url:audit:gbot:eff:norm:1"]; !exists || v != "true" {
		t.Errorf("Case 1: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// 2. googlebot noindex -> true
	if v, exists := effBySubj["url:audit:gbot:eff:norm:2"]; !exists || v != "true" {
		t.Errorf("Case 2: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// 3. generic index -> false
	if v, exists := effBySubj["url:audit:gbot:eff:norm:3"]; !exists || v != "false" {
		t.Errorf("Case 3: expected effective_noindex 'false', got exists=%v, val=%q", exists, v)
	}

	// 4. googlebot index -> false
	if v, exists := effBySubj["url:audit:gbot:eff:norm:4"]; !exists || v != "false" {
		t.Errorf("Case 4: expected effective_noindex 'false', got exists=%v, val=%q", exists, v)
	}

	// 5. generic index + googlebot noindex -> true
	if v, exists := effBySubj["url:audit:gbot:eff:norm:5"]; !exists || v != "true" {
		t.Errorf("Case 5: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// 6. generic noindex + googlebot index -> true
	if v, exists := effBySubj["url:audit:gbot:eff:norm:6"]; !exists || v != "true" {
		t.Errorf("Case 6: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// 7. generic index + GPTBot noindex -> false
	if v, exists := effBySubj["url:audit:gbot:eff:norm:7"]; !exists || v != "false" {
		t.Errorf("Case 7: expected effective_noindex 'false', got exists=%v, val=%q", exists, v)
	}

	// 8. none -> true
	if v, exists := effBySubj["url:audit:gbot:eff:norm:8"]; !exists || v != "true" {
		t.Errorf("Case 8: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// 9. index + noindex same scope -> true
	if v, exists := effBySubj["url:audit:gbot:eff:norm:9"]; !exists || v != "true" {
		t.Errorf("Case 9: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// 10. unknown scope without known noindex -> absent
	if v, exists := effBySubj["url:audit:gbot:eff:norm:10"]; exists {
		t.Errorf("Case 10: expected effective_noindex to be absent, got %q", v)
	}

	// 11. known Googlebot noindex + unknown scope -> true
	if v, exists := effBySubj["url:audit:gbot:eff:norm:11"]; !exists || v != "true" {
		t.Errorf("Case 11: expected effective_noindex 'true', got exists=%v, val=%q", exists, v)
	}

	// 12. no-directive complete evidence -> false
	if v, exists := effBySubj["url:audit:gbot:eff:norm:12"]; !exists || v != "false" {
		t.Errorf("Case 12: expected effective_noindex 'false', got exists=%v, val=%q", exists, v)
	}

	// 13a. incomplete/raw-render evidence -> absent
	if v, exists := effBySubj["url:audit:gbot:eff:norm:13"]; exists {
		t.Errorf("Case 13a: expected effective_noindex to be absent for rendered page, got %q", v)
	}

	// 13b. incomplete/raw-render evidence -> absent
	if v, exists := effBySubj["url:audit:gbot:eff:norm:14"]; exists {
		t.Errorf("Case 13b: expected effective_noindex to be absent for MetaTags nil, got %q", v)
	}

	// URL 15: only unrelated GPTBot directive with complete evidence -> false
	if v, exists := effBySubj["url:audit:gbot:eff:norm:15"]; !exists || v != "false" {
		t.Errorf("URL 15: expected effective_noindex 'false', got exists=%v, val=%q", exists, v)
	}

	// 14. deterministic normalized output: repeated build produces identical output
	res2, err := adapter.Build(context.Background(), db, adapter.BuildRequest{
		CrawlRunID: runID,
		AuditRunID: auditRunID,
		SnapshotID: snapID,
	})
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}
	if len(res.EvidenceSnapshot.NormalizedObservations) != len(res2.EvidenceSnapshot.NormalizedObservations) {
		t.Fatalf("non-deterministic count: %d vs %d",
			len(res.EvidenceSnapshot.NormalizedObservations),
			len(res2.EvidenceSnapshot.NormalizedObservations))
	}
	for i := range res.EvidenceSnapshot.NormalizedObservations {
		o1 := res.EvidenceSnapshot.NormalizedObservations[i]
		o2 := res2.EvidenceSnapshot.NormalizedObservations[i]
		if o1.ObservationID != o2.ObservationID || o1.Field != o2.Field || o1.Value != o2.Value || o1.SubjectRef != o2.SubjectRef {
			t.Errorf("determinism mismatch at index %d: %+v vs %+v", i, o1, o2)
			break
		}
	}
}
