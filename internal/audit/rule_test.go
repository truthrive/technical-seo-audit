package audit_test

import (
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

func TestRuleDefinition_Validate(t *testing.T) {
	validRule := audit.RuleDefinition{
		RuleID:                 "AR-ACC-001",
		ParentCheck:            "ACC-001",
		RuleVersion:            1,
		Name:                   "HTTPS endpoint reachable",
		Category:               "access",
		Lifecycle:              audit.LifecycleCrawl,
		Automation:             audit.AutomationDeterministic,
		DefaultSeverity:        audit.SeverityP0,
		RequiredInputs:         []string{"site_https_url"},
		Preconditions:          []string{"HTTPS origin can be derived"},
		PassCondition:          "TLS connection succeeds",
		WarningCondition:       "not used.",
		FailCondition:          "TLS connection fails",
		ManualReviewCondition:  "not used.",
		UnknownCondition:       "fetch produced no result",
		NotApplicableCondition: "not used.",
		EvidenceFields:         []string{"site_https_url"},
		SourceRefs:             []string{"SRC-INTERNAL-CHECKLIST-001"},
	}

	if err := validRule.Validate(); err != nil {
		t.Fatalf("expected valid rule, got: %v", err)
	}

	// Missing AR- prefix
	invalid := validRule
	invalid.RuleID = "ACC-001"
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for missing AR- prefix")
	}

	// Non-positive version
	invalid = validRule
	invalid.RuleVersion = 0
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for non-positive version")
	}

	// Access as lifecycle
	invalid = validRule
	invalid.Lifecycle = "Access"
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for 'Access' lifecycle")
	}

	// Invalid automation
	invalid = validRule
	invalid.Automation = "auto"
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for invalid automation")
	}

	// Invalid severity
	invalid = validRule
	invalid.DefaultSeverity = "P5"
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for invalid severity")
	}

	// Empty required inputs
	invalid = validRule
	invalid.RequiredInputs = nil
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for empty required_inputs")
	}

	// Empty preconditions
	invalid = validRule
	invalid.Preconditions = nil
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for empty preconditions")
	}

	// Empty condition
	invalid = validRule
	invalid.PassCondition = ""
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for empty pass_condition")
	}
}
