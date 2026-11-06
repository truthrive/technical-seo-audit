package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForLINK005(crawlComplete bool, observations []audit.NormalizedObservation) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:link005",
		AuditRunID:             "audit:run:link005",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          crawlComplete,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: observations,
	}
}

func TestAR_LINK_005_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	subjectRef := "url:link005:1"

	makeObs := func(field, val string) audit.NormalizedObservation {
		return audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:" + field + ":" + val),
			AuditRunID:         "audit:run:link005",
			SnapshotID:         "snap:run:link005",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subjectRef,
			Field:              field,
			Value:              val,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		}
	}

	t.Run("No discovery sources -> NOT_APPLICABLE", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs("url_identity", "https://example.com/page"),
			makeObs("is_start_url", "false"),
			makeObs("crawl_inlink_count", "3"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForLINK005(true, obs), "AR-LINK-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected StatusNotApplicable, got %s", res[0].Status)
		}
	})

	t.Run("Start URL -> NOT_APPLICABLE", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs("url_identity", "https://example.com/"),
			makeObs("discovery_sources", "sitemap"),
			makeObs("is_start_url", "true"),
			makeObs("crawl_inlink_count", "0"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForLINK005(true, obs), "AR-LINK-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusNotApplicable {
			t.Errorf("expected StatusNotApplicable, got %s", res[0].Status)
		}
	})

	t.Run("Sitemap URL with inlinks > 0 -> PASS", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs("url_identity", "https://example.com/page"),
			makeObs("discovery_sources", "sitemap"),
			makeObs("is_start_url", "false"),
			makeObs("crawl_inlink_count", "5"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForLINK005(true, obs), "AR-LINK-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusPass {
			t.Errorf("expected StatusPass, got %s", res[0].Status)
		}
	})

	t.Run("Sitemap URL with inlinks == 0 -> WARNING", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs("url_identity", "https://example.com/page"),
			makeObs("discovery_sources", "sitemap"),
			makeObs("is_start_url", "false"),
			makeObs("crawl_inlink_count", "0"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForLINK005(true, obs), "AR-LINK-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusWarning {
			t.Errorf("expected StatusWarning, got %s", res[0].Status)
		}
		if res[0].Severity != audit.SeverityP1 {
			t.Errorf("expected SeverityP1, got %s", res[0].Severity)
		}
	})

	t.Run("Incomplete crawl frontier -> UNKNOWN", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs("url_identity", "https://example.com/page"),
			makeObs("discovery_sources", "sitemap"),
			makeObs("is_start_url", "false"),
			makeObs("crawl_inlink_count", "0"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForLINK005(false, obs), "AR-LINK-005")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res) != 1 {
			t.Fatalf("expected 1 result, got %d", len(res))
		}
		if res[0].Status != audit.StatusUnknown {
			t.Errorf("expected StatusUnknown, got %s", res[0].Status)
		}
	})
}
