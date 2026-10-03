package audit

import (
	"fmt"
	"time"
)

// PolicyScope defines the scope at which a project policy applies.
type PolicyScope string

const (
	PolicyScopeSite PolicyScope = "SITE"
	PolicyScopeURL  PolicyScope = "URL"
)

// ValidatePolicyScope validates that a scope is either SITE or URL.
func ValidatePolicyScope(s PolicyScope) error {
	switch s {
	case PolicyScopeSite, PolicyScopeURL:
		return nil
	default:
		return fmt.Errorf("audit: invalid policy scope %q (allowed: SITE, URL)", s)
	}
}

// PolicyProvenance defines the origin of an explicit project policy.
// GUARDRAIL: Do NOT allow crawler or LLM inference as policy provenance.
type PolicyProvenance string

const (
	PolicyProvenanceUserInput             PolicyProvenance = "USER_INPUT"
	PolicyProvenanceProjectConfiguration  PolicyProvenance = "PROJECT_CONFIGURATION"
	PolicyProvenanceImportedConfiguration PolicyProvenance = "IMPORTED_CONFIGURATION"
)

// ValidatePolicyProvenance validates the provenance of a policy assignment.
func ValidatePolicyProvenance(p PolicyProvenance) error {
	switch p {
	case PolicyProvenanceUserInput, PolicyProvenanceProjectConfiguration, PolicyProvenanceImportedConfiguration:
		return nil
	default:
		return fmt.Errorf("audit: invalid policy provenance %q (crawler and LLM inference are prohibited; allowed: USER_INPUT, PROJECT_CONFIGURATION, IMPORTED_CONFIGURATION)", p)
	}
}

// PolicyKey defines the explicit, frozen project policy keys from knowledge/09-data-model.md.
type PolicyKey string

const (
	// Site-level policy keys
	PolicyKeyPreferredOrigin          PolicyKey = "preferred_origin"
	PolicyKeySitemapExpected          PolicyKey = "sitemap_expected"
	PolicyKeySnippetPolicy            PolicyKey = "snippet_policy"
	PolicyKeyGooglebotAccessPolicy    PolicyKey = "googlebot_access_policy"
	PolicyKeyOAISearchbotAccessPolicy PolicyKey = "oai_searchbot_access_policy"
	PolicyKeyGPTBotTrainingPolicy     PolicyKey = "gptbot_training_policy"

	// URL-level policy keys
	PolicyKeyExpectedURLState  PolicyKey = "expected_url_state"
	PolicyKeyExpectedCrawlable PolicyKey = "expected_crawlable"
	PolicyKeyExpectedIndexable PolicyKey = "expected_indexable"
	PolicyKeyPriorityPage      PolicyKey = "priority_page"
)

// Controlled values for specific policy keys
const (
	// expected_url_state values: live | redirect | missing | unspecified
	PolicyValueURLStateLive        = "live"
	PolicyValueURLStateRedirect    = "redirect"
	PolicyValueURLStateMissing     = "missing"
	PolicyValueURLStateUnspecified = "unspecified"

	// snippet_policy values: allow_unrestricted | restrict | unspecified
	PolicyValueSnippetAllowUnrestricted = "allow_unrestricted"
	PolicyValueSnippetRestrict          = "restrict"
	PolicyValueSnippetUnspecified       = "unspecified"

	// access / training policy values: allow | block | unspecified
	PolicyValueAccessAllow       = "allow"
	PolicyValueAccessBlock       = "block"
	PolicyValueAccessUnspecified = "unspecified"

	// boolean policy values
	PolicyValueTrue        = "true"
	PolicyValueFalse       = "false"
	PolicyValueUnspecified = "unspecified"
)

// ProjectPolicyAssignment represents an explicit policy contract supplied by user or project configuration.
// It is strictly separated from observed evidence.
type ProjectPolicyAssignment struct {
	PolicyAssignmentID PolicyAssignmentID `json:"policy_assignment_id"`
	AuditRunID         AuditRunID         `json:"audit_run_id"`
	PolicyKey          PolicyKey          `json:"policy_key"`
	PolicyValue        string             `json:"policy_value"`
	Scope              PolicyScope        `json:"scope"`
	TargetRef          string             `json:"target_ref"`
	Source             PolicyProvenance   `json:"source"`
	SuppliedAt         time.Time          `json:"supplied_at"`
}

// Validate validates that a ProjectPolicyAssignment conforms to the frozen policy schema.
func (p ProjectPolicyAssignment) Validate() error {
	if p.PolicyAssignmentID == "" {
		return fmt.Errorf("audit: policy_assignment_id cannot be empty")
	}
	if p.AuditRunID == "" {
		return fmt.Errorf("audit: audit_run_id cannot be empty")
	}
	if err := ValidatePolicyScope(p.Scope); err != nil {
		return err
	}
	if err := ValidatePolicyProvenance(p.Source); err != nil {
		return err
	}
	return ValidatePolicyKeyValue(p.PolicyKey, p.PolicyValue, p.Scope)
}

// ValidatePolicyKeyValue validates that a policy key and value adhere to the controlled vocabulary.
func ValidatePolicyKeyValue(key PolicyKey, value string, scope PolicyScope) error {
	switch key {
	// Site policies
	case PolicyKeyPreferredOrigin:
		if scope != PolicyScopeSite {
			return fmt.Errorf("audit: policy %q requires scope SITE, got %q", key, scope)
		}
		if value == "" {
			return fmt.Errorf("audit: policy %q value cannot be empty", key)
		}
		return nil

	case PolicyKeySitemapExpected:
		if scope != PolicyScopeSite {
			return fmt.Errorf("audit: policy %q requires scope SITE, got %q", key, scope)
		}
		switch value {
		case PolicyValueTrue, PolicyValueFalse, PolicyValueUnspecified:
			return nil
		default:
			return fmt.Errorf("audit: invalid value %q for %q (allowed: true, false, unspecified)", value, key)
		}

	case PolicyKeySnippetPolicy:
		switch value {
		case PolicyValueSnippetAllowUnrestricted, PolicyValueSnippetRestrict, PolicyValueSnippetUnspecified:
			return nil
		default:
			return fmt.Errorf("audit: invalid value %q for %q (allowed: allow_unrestricted, restrict, unspecified)", value, key)
		}

	case PolicyKeyGooglebotAccessPolicy, PolicyKeyOAISearchbotAccessPolicy, PolicyKeyGPTBotTrainingPolicy:
		switch value {
		case PolicyValueAccessAllow, PolicyValueAccessBlock, PolicyValueAccessUnspecified:
			return nil
		default:
			return fmt.Errorf("audit: invalid value %q for %q (allowed: allow, block, unspecified)", value, key)
		}

	// URL policies
	case PolicyKeyExpectedURLState:
		if scope != PolicyScopeURL {
			return fmt.Errorf("audit: policy %q requires scope URL, got %q", key, scope)
		}
		switch value {
		case PolicyValueURLStateLive, PolicyValueURLStateRedirect, PolicyValueURLStateMissing, PolicyValueURLStateUnspecified:
			return nil
		default:
			return fmt.Errorf("audit: invalid value %q for %q (allowed: live, redirect, missing, unspecified)", value, key)
		}

	case PolicyKeyExpectedCrawlable, PolicyKeyExpectedIndexable:
		switch value {
		case PolicyValueTrue, PolicyValueFalse, PolicyValueUnspecified:
			return nil
		default:
			return fmt.Errorf("audit: invalid value %q for %q (allowed: true, false, unspecified)", value, key)
		}

	case PolicyKeyPriorityPage:
		if scope != PolicyScopeURL {
			return fmt.Errorf("audit: policy %q requires scope URL, got %q", key, scope)
		}
		switch value {
		case PolicyValueTrue, PolicyValueFalse:
			return nil
		default:
			return fmt.Errorf("audit: invalid value %q for %q (allowed: true, false)", value, key)
		}

	default:
		return fmt.Errorf("audit: unrecognized policy key %q", key)
	}
}
