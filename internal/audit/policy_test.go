package audit_test

import (
	"testing"
	"time"

	"github.com/truthrive/technical-seo-audit/internal/audit"
)

func TestPolicyScopes(t *testing.T) {
	if err := audit.ValidatePolicyScope(audit.PolicyScopeSite); err != nil {
		t.Errorf("expected SITE scope to be valid, got %v", err)
	}
	if err := audit.ValidatePolicyScope(audit.PolicyScopeURL); err != nil {
		t.Errorf("expected URL scope to be valid, got %v", err)
	}

	invalid := []audit.PolicyScope{"DIRECTORY", "TEMPLATE", "SITEWIDE", "GLOBAL", ""}
	for _, inv := range invalid {
		if err := audit.ValidatePolicyScope(inv); err == nil {
			t.Errorf("expected scope %q to be rejected", inv)
		}
	}
}

func TestPolicyProvenance(t *testing.T) {
	valid := []audit.PolicyProvenance{
		audit.PolicyProvenanceUserInput,
		audit.PolicyProvenanceProjectConfiguration,
		audit.PolicyProvenanceImportedConfiguration,
	}

	for _, p := range valid {
		if err := audit.ValidatePolicyProvenance(p); err != nil {
			t.Errorf("expected provenance %s to be valid, got %v", p, err)
		}
	}

	// Invariant: Crawler and LLM inference are strictly prohibited
	prohibited := []audit.PolicyProvenance{
		"CRAWLER_INFERENCE",
		"LLM_INFERENCE",
		"HEURISTIC",
		"INFERRED",
		"",
	}
	for _, p := range prohibited {
		if err := audit.ValidatePolicyProvenance(p); err == nil {
			t.Errorf("expected provenance %q to be rejected", p)
		}
	}
}

func TestPolicyKeysAndControlledValues(t *testing.T) {
	tests := []struct {
		name    string
		key     audit.PolicyKey
		value   string
		scope   audit.PolicyScope
		wantErr bool
	}{
		// Site policies
		{
			name:    "valid preferred_origin",
			key:     audit.PolicyKeyPreferredOrigin,
			value:   "https://example.com",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "preferred_origin empty value",
			key:     audit.PolicyKeyPreferredOrigin,
			value:   "",
			scope:   audit.PolicyScopeSite,
			wantErr: true,
		},
		{
			name:    "preferred_origin wrong scope",
			key:     audit.PolicyKeyPreferredOrigin,
			value:   "https://example.com",
			scope:   audit.PolicyScopeURL,
			wantErr: true,
		},
		{
			name:    "sitemap_expected true",
			key:     audit.PolicyKeySitemapExpected,
			value:   "true",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "sitemap_expected false",
			key:     audit.PolicyKeySitemapExpected,
			value:   "false",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "sitemap_expected unspecified",
			key:     audit.PolicyKeySitemapExpected,
			value:   "unspecified",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "sitemap_expected invalid value",
			key:     audit.PolicyKeySitemapExpected,
			value:   "yes",
			scope:   audit.PolicyScopeSite,
			wantErr: true,
		},
		{
			name:    "snippet_policy allow_unrestricted",
			key:     audit.PolicyKeySnippetPolicy,
			value:   "allow_unrestricted",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "snippet_policy restrict",
			key:     audit.PolicyKeySnippetPolicy,
			value:   "restrict",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "snippet_policy invalid value",
			key:     audit.PolicyKeySnippetPolicy,
			value:   "block_all",
			scope:   audit.PolicyScopeSite,
			wantErr: true,
		},
		{
			name:    "googlebot_access_policy allow",
			key:     audit.PolicyKeyGooglebotAccessPolicy,
			value:   "allow",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "googlebot_access_policy block",
			key:     audit.PolicyKeyGooglebotAccessPolicy,
			value:   "block",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "oai_searchbot_access_policy allow",
			key:     audit.PolicyKeyOAISearchbotAccessPolicy,
			value:   "allow",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "gptbot_training_policy block",
			key:     audit.PolicyKeyGPTBotTrainingPolicy,
			value:   "block",
			scope:   audit.PolicyScopeSite,
			wantErr: false,
		},
		{
			name:    "gptbot_training_policy invalid value",
			key:     audit.PolicyKeyGPTBotTrainingPolicy,
			value:   "disallow",
			scope:   audit.PolicyScopeSite,
			wantErr: true,
		},

		// URL policies
		{
			name:    "expected_url_state live",
			key:     audit.PolicyKeyExpectedURLState,
			value:   "live",
			scope:   audit.PolicyScopeURL,
			wantErr: false,
		},
		{
			name:    "expected_url_state redirect",
			key:     audit.PolicyKeyExpectedURLState,
			value:   "redirect",
			scope:   audit.PolicyScopeURL,
			wantErr: false,
		},
		{
			name:    "expected_url_state missing",
			key:     audit.PolicyKeyExpectedURLState,
			value:   "missing",
			scope:   audit.PolicyScopeURL,
			wantErr: false,
		},
		{
			name:    "expected_url_state unspecified",
			key:     audit.PolicyKeyExpectedURLState,
			value:   "unspecified",
			scope:   audit.PolicyScopeURL,
			wantErr: false,
		},
		{
			name:    "expected_url_state invalid",
			key:     audit.PolicyKeyExpectedURLState,
			value:   "200",
			scope:   audit.PolicyScopeURL,
			wantErr: true,
		},
		{
			name:    "expected_url_state wrong scope",
			key:     audit.PolicyKeyExpectedURLState,
			value:   "live",
			scope:   audit.PolicyScopeSite,
			wantErr: true,
		},
		{
			name:    "expected_crawlable true",
			key:     audit.PolicyKeyExpectedCrawlable,
			value:   "true",
			scope:   audit.PolicyScopeURL,
			wantErr: false,
		},
		{
			name:    "expected_indexable false",
			key:     audit.PolicyKeyExpectedIndexable,
			value:   "false",
			scope:   audit.PolicyScopeURL,
			wantErr: false,
		},
		{
			name:    "priority_page true",
			key:     audit.PolicyKeyPriorityPage,
			value:   "true",
			scope:   audit.PolicyScopeURL,
			wantErr: false,
		},
		{
			name:    "priority_page wrong scope",
			key:     audit.PolicyKeyPriorityPage,
			value:   "true",
			scope:   audit.PolicyScopeSite,
			wantErr: true,
		},
		{
			name:    "unrecognized key",
			key:     "unknown_policy_key",
			value:   "val",
			scope:   audit.PolicyScopeSite,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := audit.ValidatePolicyKeyValue(tt.key, tt.value, tt.scope)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePolicyKeyValue() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProjectPolicyAssignment_Validate(t *testing.T) {
	valid := audit.ProjectPolicyAssignment{
		PolicyAssignmentID: "pol-001",
		AuditRunID:         "run-001",
		PolicyKey:          audit.PolicyKeyGooglebotAccessPolicy,
		PolicyValue:        "allow",
		Scope:              audit.PolicyScopeSite,
		TargetRef:          "site-001",
		Source:             audit.PolicyProvenanceProjectConfiguration,
		SuppliedAt:         time.Now(),
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid assignment, got error: %v", err)
	}

	// Missing ID
	invalid := valid
	invalid.PolicyAssignmentID = ""
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for empty policy_assignment_id")
	}

	// Invalid provenance
	invalid = valid
	invalid.Source = "CRAWLER_INFERENCE"
	if err := invalid.Validate(); err == nil {
		t.Errorf("expected error for invalid source provenance")
	}
}
