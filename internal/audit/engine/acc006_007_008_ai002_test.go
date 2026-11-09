package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func TestEvaluateBotProfileAndResourceRules_VerdictMatrix(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

	buildURLSnap := func(fields map[string]string) *audit.EvidenceSnapshot {
		var obs []audit.NormalizedObservation
		id := 1
		for k, v := range fields {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID("obs:bot:" + string(rune('0'+id))),
				AuditRunID:         "audit:bot",
				SnapshotID:         "snap:bot",
				SubjectType:        audit.SubjectURL,
				SubjectRef:         "url:audit:bot:1",
				Field:              k,
				Value:              v,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:1"},
				ObservedAt:         now,
			})
			id++
		}
		return &audit.EvidenceSnapshot{
			SnapshotID:             "snap:bot",
			AuditRunID:             "audit:bot",
			SnapshotStatus:         audit.SnapshotFrozen,
			NormalizationVersion:   "v1.11.0",
			CrawlComplete:          true,
			CreatedAt:              now,
			FrozenAt:               &now,
			NormalizedObservations: obs,
		}
	}

	// 1. AR-ACC-006 (Googlebot profile comparison)
	t.Run("AR-ACC-006_Disabled_NOT_APPLICABLE", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity": "https://example.com/",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-006")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
			t.Fatalf("expected NOT_APPLICABLE, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-006_Consistent200_PASS", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity":                 "https://example.com/",
			"default_profile_status":       "200",
			"googlebot_profile_status":     "200",
			"default_challenge_detected":   "false",
			"googlebot_challenge_detected": "false",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-006")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-006_Googlebot403_WARNING", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity":                 "https://example.com/",
			"default_profile_status":       "200",
			"googlebot_profile_status":     "403",
			"default_challenge_detected":   "false",
			"googlebot_challenge_detected": "true",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-006")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusWarning {
			t.Fatalf("expected WARNING, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-006_IncompleteComparison_UNKNOWN", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity":           "https://example.com/",
			"default_profile_status": "200",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-006")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusUnknown {
			t.Fatalf("expected UNKNOWN, got %v (err=%v)", res, err)
		}
	})

	// 2. AR-ACC-008 (Bot-profile access restriction)
	t.Run("AR-ACC-008_Disabled_NOT_APPLICABLE", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity": "https://example.com/",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-008")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
			t.Fatalf("expected NOT_APPLICABLE, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-008_Bot429_WARNING", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity":               "https://example.com/",
			"default_profile_status":     "200",
			"bot_profile_status":         "429",
			"default_challenge_detected": "false",
			"bot_challenge_detected":     "false",
			"tested_user_agent":          "Bingbot/2.0",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-008")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusWarning {
			t.Fatalf("expected WARNING, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-008_Consistent_PASS", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity":               "https://example.com/",
			"default_profile_status":     "200",
			"bot_profile_status":         "200",
			"default_challenge_detected": "false",
			"bot_challenge_detected":     "false",
			"tested_user_agent":          "Bingbot/2.0",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-008")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v (err=%v)", res, err)
		}
	})

	// 3. AR-AI-002 (OAI-SearchBot profile anomaly)
	t.Run("AR-AI-002_Disabled_NOT_APPLICABLE", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity": "https://example.com/",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-AI-002")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
			t.Fatalf("expected NOT_APPLICABLE, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-AI-002_Challenge_WARNING", func(t *testing.T) {
		snap := buildURLSnap(map[string]string{
			"url_identity":               "https://example.com/",
			"default_profile_status":     "200",
			"oai_profile_status":         "200",
			"default_challenge_detected": "false",
			"oai_challenge_detected":     "true",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-AI-002")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusWarning {
			t.Fatalf("expected WARNING, got %v (err=%v)", res, err)
		}
	})

	// 4. AR-ACC-007 (Blocked CSS/JS resource affects rendered output)
	buildResSnap := func(subjType audit.EvaluationSubjectType, subjRef string, fields map[string]string) *audit.EvidenceSnapshot {
		var obs []audit.NormalizedObservation
		id := 1
		for k, v := range fields {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID("obs:res:" + string(rune('0'+id))),
				AuditRunID:         "audit:res",
				SnapshotID:         "snap:res",
				SubjectType:        subjType,
				SubjectRef:         subjRef,
				Field:              k,
				Value:              v,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_pages:1"},
				ObservedAt:         now,
			})
			id++
		}
		return &audit.EvidenceSnapshot{
			SnapshotID:             "snap:res",
			AuditRunID:             "audit:res",
			SnapshotStatus:         audit.SnapshotFrozen,
			NormalizationVersion:   "v1.11.0",
			CrawlComplete:          true,
			CreatedAt:              now,
			FrozenAt:               &now,
			NormalizedObservations: obs,
		}
	}

	t.Run("AR-ACC-007_UnrenderedURL_NOT_APPLICABLE", func(t *testing.T) {
		snap := buildResSnap(audit.SubjectURL, "url:audit:res:1", map[string]string{
			"url_identity": "https://example.com/",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-007")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusNotApplicable {
			t.Fatalf("expected NOT_APPLICABLE, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-007_AllowedResource_PASS", func(t *testing.T) {
		snap := buildResSnap(audit.SubjectResource, "res:audit:res:1:0", map[string]string{
			"page_url":                "https://example.com/",
			"resource_url":            "https://example.com/app.js",
			"resource_type":           "JS",
			"resource_robots_allowed": "true",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-007")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-007_BlockedResourceNoDependency_PASS", func(t *testing.T) {
		snap := buildResSnap(audit.SubjectResource, "res:audit:res:1:0", map[string]string{
			"page_url":                   "https://example.com/",
			"resource_url":               "https://example.com/analytics.js",
			"resource_type":              "JS",
			"resource_robots_allowed":    "false",
			"render_dependency_observed": "false",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-007")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusPass {
			t.Fatalf("expected PASS, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-007_BlockedResourceUnknownImpact_WARNING", func(t *testing.T) {
		snap := buildResSnap(audit.SubjectResource, "res:audit:res:1:0", map[string]string{
			"page_url":                "https://example.com/",
			"resource_url":            "https://example.com/bundle.js",
			"resource_type":           "JS",
			"resource_robots_allowed": "false",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-007")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusWarning {
			t.Fatalf("expected WARNING, got %v (err=%v)", res, err)
		}
	})

	t.Run("AR-ACC-007_BlockedResourceProvenDiff_FAIL", func(t *testing.T) {
		snap := buildResSnap(audit.SubjectResource, "res:audit:res:1:0", map[string]string{
			"page_url":                   "https://example.com/",
			"resource_url":               "https://example.com/bundle.js",
			"resource_type":              "JS",
			"resource_robots_allowed":    "false",
			"render_dependency_observed": "true",
			"render_diff_evidence":       "main_text_present_rendered=false; h1_rendered missing",
		})
		res, err := eng.EvaluateRule(ctx, snap, "AR-ACC-007")
		if err != nil || len(res) != 1 || res[0].Status != audit.StatusFail {
			t.Fatalf("expected FAIL, got %v (err=%v)", res, err)
		}
	})
}
