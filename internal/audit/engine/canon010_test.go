package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func newSnapshotForCANON010(observations []audit.NormalizedObservation) *audit.EvidenceSnapshot {
	now := time.Now().UTC()
	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:run:canon010",
		AuditRunID:             "audit:run:canon010",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now.Add(-1 * time.Minute),
		FrozenAt:               &now,
		NormalizedObservations: observations,
	}
}

func TestAR_CANON_010_Evaluation(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()
	now := time.Now().UTC()
	sourceRef := "url:canon010:source"
	targetRef := "url:canon010:target"

	makeObs := func(subj, field, val string) audit.NormalizedObservation {
		return audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:" + subj + ":" + field + ":" + val),
			AuditRunID:         "audit:run:canon010",
			SnapshotID:         "snap:run:canon010",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         subj,
			Field:              field,
			Value:              val,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:test"},
			ObservedAt:         now,
		}
	}

	t.Run("Non-redirect URL -> NOT_APPLICABLE", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs(sourceRef, "url_identity", "https://example.com/non-redirect"),
			makeObs(sourceRef, "redirect_initial_observed", "false"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForCANON010(obs), "AR-CANON-010")
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

	t.Run("Redirect loop detected -> NOT_APPLICABLE", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs(sourceRef, "url_identity", "https://example.com/loop"),
			makeObs(sourceRef, "redirect_initial_observed", "true"),
			makeObs(sourceRef, "redirect_loop_detected", "true"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForCANON010(obs), "AR-CANON-010")
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

	t.Run("Redirect to final destination returning 200 -> PASS", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs(sourceRef, "url_identity", "https://example.com/redirect"),
			makeObs(sourceRef, "redirect_initial_observed", "true"),
			makeObs(sourceRef, "redirect_final_url", "https://example.com/destination"),
			makeObs(sourceRef, "redirect_final_target_subject_ref", targetRef),
			makeObs(targetRef, "url_identity", "https://example.com/destination"),
			makeObs(targetRef, "http_status", "200"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForCANON010(obs), "AR-CANON-010")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		if len(res) != 2 {
			t.Fatalf("expected 2 results (for both URLs in snapshot), got %d", len(res))
		}
		// Find source result
		var srcRes *audit.RuleResult
		for i := range res {
			if res[i].SubjectRef == sourceRef {
				srcRes = &res[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("source result not found")
		}
		if srcRes.Status != audit.StatusPass {
			t.Errorf("expected StatusPass, got %s", srcRes.Status)
		}
	})

	t.Run("Redirect to final destination returning 404 -> FAIL", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs(sourceRef, "url_identity", "https://example.com/redirect-dead"),
			makeObs(sourceRef, "redirect_initial_observed", "true"),
			makeObs(sourceRef, "redirect_final_url", "https://example.com/destination-404"),
			makeObs(sourceRef, "redirect_final_target_subject_ref", targetRef),
			makeObs(targetRef, "url_identity", "https://example.com/destination-404"),
			makeObs(targetRef, "http_status", "404"),
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForCANON010(obs), "AR-CANON-010")
		if err != nil {
			t.Fatalf("EvaluateRule failed: %v", err)
		}
		var srcRes *audit.RuleResult
		for i := range res {
			if res[i].SubjectRef == sourceRef {
				srcRes = &res[i]
				break
			}
		}
		if srcRes == nil {
			t.Fatalf("source result not found")
		}
		if srcRes.Status != audit.StatusFail {
			t.Errorf("expected StatusFail, got %s", srcRes.Status)
		}
		if srcRes.Severity != audit.SeverityP1 {
			t.Errorf("expected SeverityP1, got %s", srcRes.Severity)
		}
	})

	t.Run("Redirect to unvisited external target -> UNKNOWN", func(t *testing.T) {
		obs := []audit.NormalizedObservation{
			makeObs(sourceRef, "url_identity", "https://example.com/redirect-external"),
			makeObs(sourceRef, "redirect_initial_observed", "true"),
			makeObs(sourceRef, "redirect_final_url", "https://external.example.com/dest"),
			// No redirect_final_target_subject_ref because target was outside crawl scope
		}
		res, err := eng.EvaluateRule(ctx, newSnapshotForCANON010(obs), "AR-CANON-010")
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
