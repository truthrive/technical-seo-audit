package audit_test

import (
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

func TestRuleResultStatuses(t *testing.T) {
	valid := audit.ValidRuleResultStatuses()
	if len(valid) != 6 {
		t.Fatalf("expected exactly 6 valid rule result statuses, got %d", len(valid))
	}

	expected := map[audit.RuleResultStatus]bool{
		audit.StatusPass:          true,
		audit.StatusWarning:       true,
		audit.StatusFail:          true,
		audit.StatusManualReview:  true,
		audit.StatusNotApplicable: true,
		audit.StatusUnknown:       true,
	}

	for _, s := range valid {
		if !expected[s] {
			t.Errorf("unexpected status in ValidRuleResultStatuses: %s", s)
		}
		if err := audit.ValidateRuleResultStatus(s); err != nil {
			t.Errorf("expected valid status %s, got error: %v", s, err)
		}
	}

	// Strictly reject INFO and other invalid values
	prohibited := []audit.RuleResultStatus{
		"INFO",
		"info",
		"Pass",
		"pass",
		"SUCCESS",
		"ERROR",
		"SKIP",
		"",
	}

	for _, p := range prohibited {
		if err := audit.ValidateRuleResultStatus(p); err == nil {
			t.Errorf("expected status %q to be rejected, but validation succeeded", p)
		}
	}
}

func TestPresentationClassification(t *testing.T) {
	valid := []audit.PresentationClassification{
		audit.PresentationIssue,
		audit.PresentationWarning,
		audit.PresentationManualReview,
		audit.PresentationInformation,
	}

	for _, p := range valid {
		if err := audit.ValidatePresentationClassification(p); err != nil {
			t.Errorf("expected valid presentation classification %s, got error: %v", p, err)
		}
	}

	invalid := []audit.PresentationClassification{"FAIL", "PASS", "INFO", "UNKNOWN", ""}
	for _, inv := range invalid {
		if err := audit.ValidatePresentationClassification(inv); err == nil {
			t.Errorf("expected presentation classification %q to be rejected", inv)
		}
	}
}

func TestSeverity(t *testing.T) {
	valid := []audit.Severity{
		audit.SeverityP0,
		audit.SeverityP1,
		audit.SeverityP2,
		audit.SeverityP3,
	}

	for _, s := range valid {
		if err := audit.ValidateSeverity(s); err != nil {
			t.Errorf("expected valid severity %s, got error: %v", s, err)
		}
	}

	// Prohibit weights, scores, and numbers
	invalid := []audit.Severity{"P4", "CRITICAL", "HIGH", "1", "100", "0", ""}
	for _, inv := range invalid {
		if err := audit.ValidateSeverity(inv); err == nil {
			t.Errorf("expected severity %q to be rejected", inv)
		}
	}
}

func TestAutomationClass(t *testing.T) {
	valid := []audit.AutomationClass{
		audit.AutomationDeterministic,
		audit.AutomationAssisted,
		audit.AutomationManual,
	}

	for _, a := range valid {
		if err := audit.ValidateAutomationClass(a); err != nil {
			t.Errorf("expected valid automation class %s, got error: %v", a, err)
		}
	}

	invalid := []audit.AutomationClass{"Full", "Partial", "Manual", "automated", "semi-automated", ""}
	for _, inv := range invalid {
		if err := audit.ValidateAutomationClass(inv); err == nil {
			t.Errorf("expected automation class %q to be rejected", inv)
		}
	}
}

func TestLifecycleStage(t *testing.T) {
	canonical := []audit.LifecycleStage{
		audit.LifecycleDiscover,
		audit.LifecycleCrawl,
		audit.LifecycleRender,
		audit.LifecycleIndex,
		audit.LifecycleConsolidate,
		audit.LifecycleRetrieve,
		audit.LifecycleCite,
		audit.LifecycleMeasure,
	}

	for _, l := range canonical {
		if err := audit.ValidateLifecycleStage(l); err != nil {
			t.Errorf("expected valid lifecycle stage %s, got error: %v", l, err)
		}
	}

	// Critical invariant: "Access" is NOT a lifecycle stage
	if err := audit.ValidateLifecycleStage("Access"); err == nil {
		t.Errorf("Access must NOT be accepted as a lifecycle stage")
	}

	invalid := []audit.LifecycleStage{"access", "CRAWL", "Discovery", "Rank", "Ranking", ""}
	for _, inv := range invalid {
		if err := audit.ValidateLifecycleStage(inv); err == nil {
			t.Errorf("expected lifecycle stage %q to be rejected", inv)
		}
	}
}

func TestAuditRunStatus(t *testing.T) {
	valid := []audit.AuditRunStatus{
		audit.RunStatusCreated,
		audit.RunStatusAcquiring,
		audit.RunStatusNormalizing,
		audit.RunStatusSnapshotFrozen,
		audit.RunStatusEvaluating,
		audit.RunStatusAggregating,
		audit.RunStatusCompleted,
		audit.RunStatusFailed,
		audit.RunStatusCancelled,
	}

	for _, s := range valid {
		if err := audit.ValidateAuditRunStatus(s); err != nil {
			t.Errorf("expected valid audit run status %s, got error: %v", s, err)
		}
	}

	invalid := []audit.AuditRunStatus{"RUNNING", "PAUSED", "STOPPED", "pending", ""}
	for _, inv := range invalid {
		if err := audit.ValidateAuditRunStatus(inv); err == nil {
			t.Errorf("expected audit run status %q to be rejected", inv)
		}
	}
}

func TestDiscoveryType(t *testing.T) {
	valid := []audit.DiscoveryType{
		audit.DiscoveryStartURL,
		audit.DiscoveryInternalLink,
		audit.DiscoverySitemap,
		audit.DiscoveryPagination,
		audit.DiscoverySuppliedURLList,
	}

	for _, d := range valid {
		if err := audit.ValidateDiscoveryType(d); err != nil {
			t.Errorf("expected valid discovery type %s, got error: %v", d, err)
		}
	}

	// Prohibited probe / synthetic types as discovery
	invalid := []audit.DiscoveryType{
		"CANONICAL_TARGET",
		"REDIRECT_TARGET",
		"PROBE",
		"BOT_PROFILE",
		"",
	}
	for _, inv := range invalid {
		if err := audit.ValidateDiscoveryType(inv); err == nil {
			t.Errorf("expected discovery type %q to be rejected", inv)
		}
	}
}

func TestAcquisitionPurpose(t *testing.T) {
	valid := []audit.AcquisitionPurpose{
		audit.PurposeCrawl,
		audit.PurposeRobotsFetch,
		audit.PurposeSitemapFetch,
		audit.PurposeCanonicalTarget,
		audit.PurposeRedirectTarget,
		audit.PurposeHostVariantProbe,
		audit.PurposeSlashVariantProbe,
		audit.PurposeKnownMissingProbe,
		audit.PurposeBotProfileCompare,
		audit.PurposeRenderSource,
		audit.PurposeRenderResource,
	}

	for _, p := range valid {
		if err := audit.ValidateAcquisitionPurpose(p); err != nil {
			t.Errorf("expected valid acquisition purpose %s, got error: %v", p, err)
		}
	}

	if err := audit.ValidateAcquisitionPurpose("UNKNOWN_PURPOSE"); err == nil {
		t.Errorf("expected unknown acquisition purpose to be rejected")
	}
}

func TestRequestProfile(t *testing.T) {
	valid := []audit.RequestProfile{
		audit.ProfileDefault,
		audit.ProfileGooglebot,
		audit.ProfileOAISearchbot,
		audit.ProfileGPTBot,
		audit.ProfileCustomBot,
	}

	for _, r := range valid {
		if err := audit.ValidateRequestProfile(r); err != nil {
			t.Errorf("expected valid request profile %s, got error: %v", r, err)
		}
	}

	if err := audit.ValidateRequestProfile("VERIFIED_GOOGLEBOT"); err == nil {
		t.Errorf("expected VERIFIED_GOOGLEBOT to be rejected (profiles are never verified bot identities)")
	}
}
