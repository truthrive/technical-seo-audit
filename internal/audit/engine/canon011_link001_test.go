package engine_test

import (
	"context"
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
	"github.com/truthrive/technical-seo-audit/internal/audit/engine"
)

func buildCANON011Snapshot(pagDetected, hasNext, nextURL, nextTag, nextHref string) *audit.EvidenceSnapshot {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	var obs []audit.NormalizedObservation
	id := 1
	add := func(field, val string) {
		obs = append(obs, audit.NormalizedObservation{
			ObservationID:      audit.ObservationID("obs:canon011:" + string(rune('0'+id))),
			AuditRunID:         "audit:canon011",
			SnapshotID:         "snap:canon011",
			SubjectType:        audit.SubjectURL,
			SubjectRef:         "url:audit:canon011:1",
			Field:              field,
			Value:              val,
			DerivationType:     audit.DerivationDirect,
			SourceEvidenceRefs: []string{"sitecrawl_pages:1"},
			ObservedAt:         now,
		})
		id++
	}
	add("url_identity", "https://example.com/blog?page=1")
	if pagDetected != "" {
		add("pagination_detected", pagDetected)
	}
	if hasNext != "" {
		add("has_next_page", hasNext)
	}
	if nextURL != "" {
		add("next_page_url", nextURL)
	}
	if nextTag != "" {
		add("next_page_element_tag", nextTag)
	}
	if nextHref != "" {
		add("next_page_href", nextHref)
	}

	return &audit.EvidenceSnapshot{
		SnapshotID:             "snap:canon011",
		AuditRunID:             "audit:canon011",
		SnapshotStatus:         audit.SnapshotFrozen,
		NormalizationVersion:   "v1.11.0",
		CrawlComplete:          true,
		CreatedAt:              now,
		FrozenAt:               &now,
		NormalizedObservations: obs,
	}
}

func TestEvaluateCANON011_VerdictMatrix(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	ctx := context.Background()

	tests := []struct {
		name           string
		pagDetected    string
		hasNext        string
		nextURL        string
		nextTag        string
		nextHref       string
		expectedStatus audit.RuleResultStatus
	}{
		{"NoPagination_NOT_APPLICABLE", "false", "false", "", "", "", audit.StatusNotApplicable},
		{"FinalPage_NOT_APPLICABLE", "true", "false", "", "", "", audit.StatusNotApplicable},
		{"CrawlableAnchor_PASS", "true", "true", "https://example.com/blog?page=2", "a", "/blog?page=2", audit.StatusPass},
		{"ButtonNextPage_WARNING", "true", "true", "", "button", "", audit.StatusWarning},
		{"HashHrefAnchor_WARNING", "true", "true", "", "a", "#", audit.StatusWarning},
		{"JavascriptHrefAnchor_WARNING", "true", "true", "", "a", "javascript:loadMore()", audit.StatusWarning},
		{"MissingPaginationEvidence_UNKNOWN", "", "", "", "", "", audit.StatusUnknown},
		{"MissingNextElementEvidence_UNKNOWN", "true", "true", "", "", "", audit.StatusUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snap := buildCANON011Snapshot(tc.pagDetected, tc.hasNext, tc.nextURL, tc.nextTag, tc.nextHref)
			res, err := eng.EvaluateRule(ctx, snap, "AR-CANON-011")
			if err != nil {
				t.Fatalf("EvaluateRule: %v", err)
			}
			if len(res) != 1 {
				t.Fatalf("expected 1 result, got %d", len(res))
			}
			if res[0].Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s (summary=%q)", tc.expectedStatus, res[0].Status, res[0].ObservedSummary)
			}
		})
	}
}

func TestEvaluateLINK001_VerdictMatrix(t *testing.T) {
	eng, err := engine.New()
	if err != nil {
		t.Fatalf("engine.New: %v", err)
	}
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)

	buildLinkSnap := func(isInternal, navCand, target, tag, href string) *audit.EvidenceSnapshot {
		var obs []audit.NormalizedObservation
		id := 1
		add := func(field, val string) {
			obs = append(obs, audit.NormalizedObservation{
				ObservationID:      audit.ObservationID("obs:link001:" + string(rune('0'+id))),
				AuditRunID:         "audit:link001",
				SnapshotID:         "snap:link001",
				SubjectType:        audit.SubjectLink,
				SubjectRef:         "link:audit:link001:1:0",
				Field:              field,
				Value:              val,
				DerivationType:     audit.DerivationDirect,
				SourceEvidenceRefs: []string{"sitecrawl_links:1:0"},
				ObservedAt:         now,
			})
			id++
		}
		add("link_source_url", "https://example.com/")
		if isInternal != "" {
			add("link_is_internal", isInternal)
		}
		if navCand != "" {
			add("navigation_candidate", navCand)
		}
		if target != "" {
			add("link_target", target)
		}
		if tag != "" {
			add("element_tag", tag)
		}
		if href != "" {
			add("href", href)
		}

		return &audit.EvidenceSnapshot{
			SnapshotID:             "snap:link001",
			AuditRunID:             "audit:link001",
			SnapshotStatus:         audit.SnapshotFrozen,
			NormalizationVersion:   "v1.11.0",
			CrawlComplete:          true,
			CreatedAt:              now,
			FrozenAt:               &now,
			NormalizedObservations: obs,
		}
	}

	tests := []struct {
		name           string
		isInternal     string
		navCand        string
		target         string
		tag            string
		href           string
		expectedStatus audit.RuleResultStatus
	}{
		{"ExternalLink_NOT_APPLICABLE", "false", "false", "https://external.example.org/", "a", "https://external.example.org/", audit.StatusNotApplicable},
		{"BodyNonNavLink_NOT_APPLICABLE", "true", "false", "https://example.com/about", "a", "https://example.com/about", audit.StatusNotApplicable},
		{"NavAnchorCrawlable_PASS", "true", "true", "https://example.com/pricing", "a", "https://example.com/pricing", audit.StatusPass},
		{"NavButtonNonAnchor_WARNING", "true", "true", "https://example.com/pricing", "button", "", audit.StatusWarning},
		{"NavAnchorJavascriptHref_WARNING", "true", "true", "https://example.com/pricing", "a", "javascript:navigate('/pricing')", audit.StatusWarning},
		{"MissingNavCandidate_UNKNOWN", "true", "", "https://example.com/pricing", "a", "https://example.com/pricing", audit.StatusUnknown},
		{"MissingElementTag_UNKNOWN", "true", "true", "https://example.com/pricing", "", "https://example.com/pricing", audit.StatusUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snap := buildLinkSnap(tc.isInternal, tc.navCand, tc.target, tc.tag, tc.href)
			res, err := eng.EvaluateRule(ctx, snap, "AR-LINK-001")
			if err != nil {
				t.Fatalf("EvaluateRule: %v", err)
			}
			if len(res) != 1 {
				t.Fatalf("expected 1 result, got %d", len(res))
			}
			if res[0].Status != tc.expectedStatus {
				t.Errorf("expected status %s, got %s (summary=%q)", tc.expectedStatus, res[0].Status, res[0].ObservedSummary)
			}
		})
	}
}
