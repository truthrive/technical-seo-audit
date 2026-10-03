package audit_test

import (
	"testing"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

// expected47RuleIDs is an independent test expectation directly derived from
// knowledge/07-v1-atomic-rule-manifest.md.
// It is NOT generated dynamically from the JSON registry under test.
var expected47RuleIDs = []string{
	// Crawl / Access / HTTP (8 rules)
	"AR-ACC-001",
	"AR-ACC-002",
	"AR-ACC-003",
	"AR-ACC-004",
	"AR-ACC-005",
	"AR-ACC-006",
	"AR-ACC-007",
	"AR-ACC-008",

	// Index (6 rules)
	"AR-INDEX-001",
	"AR-INDEX-002",
	"AR-INDEX-003",
	"AR-INDEX-004",
	"AR-INDEX-005",
	"AR-INDEX-006",

	// Consolidate / Canonical / Redirects (11 rules)
	"AR-CANON-001",
	"AR-CANON-002",
	"AR-CANON-003",
	"AR-CANON-004",
	"AR-CANON-005",
	"AR-CANON-006",
	"AR-CANON-007",
	"AR-CANON-008",
	"AR-CANON-009",
	"AR-CANON-010",
	"AR-CANON-011",

	// Discover / Internal Links (6 rules)
	"AR-LINK-001",
	"AR-LINK-002",
	"AR-LINK-003",
	"AR-LINK-004",
	"AR-LINK-005",
	"AR-LINK-006",

	// Discover / Sitemaps (6 rules)
	"AR-DISC-001",
	"AR-DISC-002",
	"AR-DISC-003",
	"AR-DISC-004",
	"AR-DISC-005",
	"AR-DISC-006",

	// Retrieve / Structured Data (1 rule)
	"AR-ENTITY-001",

	// Render (6 rules)
	"AR-RENDER-001",
	"AR-RENDER-002",
	"AR-RENDER-003",
	"AR-RENDER-004",
	"AR-RENDER-005",
	"AR-RENDER-006",

	// AI Search / GEO policy (3 rules)
	"AR-AI-001",
	"AR-AI-002",
	"AR-AI-003",
}

func TestLoadV1Registry_ExactCountAndIDs(t *testing.T) {
	reg, err := audit.LoadV1Registry()
	if err != nil {
		t.Fatalf("LoadV1Registry() failed: %v", err)
	}

	if reg.Count() != 47 {
		t.Fatalf("expected exactly 47 rules, got %d", reg.Count())
	}

	if len(expected47RuleIDs) != 47 {
		t.Fatalf("expected test fixture to declare exactly 47 IDs, got %d", len(expected47RuleIDs))
	}

	seen := make(map[string]bool)
	for _, id := range expected47RuleIDs {
		rule, found := reg.Get(id)
		if !found {
			t.Errorf("expected rule ID %s not found in registry", id)
			continue
		}
		if rule.RuleID != id {
			t.Errorf("expected rule ID %s, got %s", id, rule.RuleID)
		}
		seen[id] = true
	}

	for _, rule := range reg.All() {
		if !seen[rule.RuleID] {
			t.Errorf("unexpected rule ID %s found in registry", rule.RuleID)
		}
	}
}

func TestLoadV1Registry_AutomationBreakdown(t *testing.T) {
	reg, err := audit.LoadV1Registry()
	if err != nil {
		t.Fatalf("LoadV1Registry() failed: %v", err)
	}

	deterministic := reg.ByAutomation(audit.AutomationDeterministic)
	assisted := reg.ByAutomation(audit.AutomationAssisted)
	manual := reg.ByAutomation(audit.AutomationManual)

	if len(deterministic) != 30 {
		t.Errorf("expected exactly 30 deterministic rules, got %d", len(deterministic))
	}
	if len(assisted) != 16 {
		t.Errorf("expected exactly 16 assisted rules, got %d", len(assisted))
	}
	if len(manual) != 1 {
		t.Errorf("expected exactly 1 manual rule, got %d", len(manual))
	}

	if len(manual) == 1 && manual[0].RuleID != "AR-LINK-006" {
		t.Errorf("expected single manual rule to be AR-LINK-006, got %s", manual[0].RuleID)
	}
}

func TestLoadV1Registry_RuleFieldIntegrity(t *testing.T) {
	reg, err := audit.LoadV1Registry()
	if err != nil {
		t.Fatalf("LoadV1Registry() failed: %v", err)
	}

	for _, r := range reg.All() {
		// Identity
		if r.RuleID == "" || len(r.RuleID) < 4 || r.RuleID[:3] != "AR-" {
			t.Errorf("rule ID %q must start with 'AR-'", r.RuleID)
		}
		if r.ParentCheck == "" {
			t.Errorf("rule %s has empty parent_check", r.RuleID)
		}
		if r.RuleVersion <= 0 {
			t.Errorf("rule %s has non-positive rule_version: %d", r.RuleID, r.RuleVersion)
		}
		if r.Name == "" {
			t.Errorf("rule %s has empty name", r.RuleID)
		}
		if r.Category == "" {
			t.Errorf("rule %s has empty category", r.RuleID)
		}

		// Enums
		if err := audit.ValidateLifecycleStage(r.Lifecycle); err != nil {
			t.Errorf("rule %s invalid lifecycle: %v", r.RuleID, err)
		}
		if r.Lifecycle == "Access" {
			t.Errorf("rule %s has prohibited lifecycle 'Access'", r.RuleID)
		}
		if err := audit.ValidateAutomationClass(r.Automation); err != nil {
			t.Errorf("rule %s invalid automation: %v", r.RuleID, err)
		}
		if err := audit.ValidateSeverity(r.DefaultSeverity); err != nil {
			t.Errorf("rule %s invalid severity: %v", r.RuleID, err)
		}

		// Slices
		if len(r.RequiredInputs) == 0 {
			t.Errorf("rule %s has empty required_inputs", r.RuleID)
		}
		if len(r.Preconditions) == 0 {
			t.Errorf("rule %s has empty preconditions", r.RuleID)
		}
		if len(r.EvidenceFields) == 0 {
			t.Errorf("rule %s has empty evidence_fields", r.RuleID)
		}
		if len(r.SourceRefs) == 0 {
			t.Errorf("rule %s has empty source_refs", r.RuleID)
		}

		// Status conditions
		if r.PassCondition == "" {
			t.Errorf("rule %s has empty pass_condition", r.RuleID)
		}
		if r.WarningCondition == "" {
			t.Errorf("rule %s has empty warning_condition", r.RuleID)
		}
		if r.FailCondition == "" {
			t.Errorf("rule %s has empty fail_condition", r.RuleID)
		}
		if r.ManualReviewCondition == "" {
			t.Errorf("rule %s has empty manual_review_condition", r.RuleID)
		}
		if r.UnknownCondition == "" {
			t.Errorf("rule %s has empty unknown_condition", r.RuleID)
		}
		if r.NotApplicableCondition == "" {
			t.Errorf("rule %s has empty not_applicable_condition", r.RuleID)
		}
	}
}

func TestLoadV1Registry_NoDeferredBroadChecksRegistered(t *testing.T) {
	reg, err := audit.LoadV1Registry()
	if err != nil {
		t.Fatalf("LoadV1Registry() failed: %v", err)
	}

	// 5 deferred broad catalog checks from knowledge/07-v1-atomic-rule-manifest.md:
	// - ACC-006 repeated instability
	// - ACC-008 crawl traps
	// - INDEX-005 index pollution
	// - CANON-004 signal-consistency aggregation
	// - CANON-005 parameter/filter strategy
	deferredChecks := []string{
		"ACC-006",
		"ACC-008",
		"INDEX-005",
		"CANON-004",
		"CANON-005",
	}

	for _, rule := range reg.All() {
		for _, deferred := range deferredChecks {
			if rule.ParentCheck == deferred {
				t.Errorf("rule %s has deferred parent check %s", rule.RuleID, deferred)
			}
		}
	}

	// Verify that AR-ACC-006 and AR-ACC-008 are NOT mapped to deferred ACC-006/ACC-008:
	acc006, ok := reg.Get("AR-ACC-006")
	if !ok {
		t.Fatalf("AR-ACC-006 not found")
	}
	if acc006.ParentCheck != "ACC-002" {
		t.Errorf("expected AR-ACC-006 parent to be ACC-002, got %s", acc006.ParentCheck)
	}

	acc008, ok := reg.Get("AR-ACC-008")
	if !ok {
		t.Fatalf("AR-ACC-008 not found")
	}
	if acc008.ParentCheck != "ACC-005" {
		t.Errorf("expected AR-ACC-008 parent to be ACC-005, got %s", acc008.ParentCheck)
	}
}

func TestLoadV1Registry_ByLifecycle(t *testing.T) {
	reg, err := audit.LoadV1Registry()
	if err != nil {
		t.Fatalf("LoadV1Registry() failed: %v", err)
	}

	crawls := reg.ByLifecycle(audit.LifecycleCrawl)
	indexes := reg.ByLifecycle(audit.LifecycleIndex)
	consolidates := reg.ByLifecycle(audit.LifecycleConsolidate)
	discovers := reg.ByLifecycle(audit.LifecycleDiscover)
	renders := reg.ByLifecycle(audit.LifecycleRender)
	retrieves := reg.ByLifecycle(audit.LifecycleRetrieve)

	total := len(crawls) + len(indexes) + len(consolidates) + len(discovers) + len(renders) + len(retrieves)
	if total != 47 {
		t.Errorf("expected sum of lifecycle rules to be 47, got %d", total)
	}
}
