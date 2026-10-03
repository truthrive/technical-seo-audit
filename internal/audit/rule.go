package audit

import (
	"fmt"
	"strings"
)

// RuleDefinition represents the machine-readable declarative specification of an atomic audit rule.
// It reflects the frozen contract from knowledge/07-v1-atomic-rule-manifest.md.
type RuleDefinition struct {
	RuleID                 string          `json:"rule_id"`
	ParentCheck            string          `json:"parent_check"`
	RuleVersion            int             `json:"rule_version"`
	Name                   string          `json:"name"`
	Category               string          `json:"category"`
	Lifecycle              LifecycleStage  `json:"lifecycle"`
	Automation             AutomationClass `json:"automation"`
	DefaultSeverity        Severity        `json:"default_severity"`
	RequiredInputs         []string        `json:"required_inputs"`
	Preconditions          []string        `json:"preconditions"`
	PassCondition          string          `json:"pass_condition"`
	WarningCondition       string          `json:"warning_condition"`
	FailCondition          string          `json:"fail_condition"`
	ManualReviewCondition  string          `json:"manual_review_condition"`
	UnknownCondition       string          `json:"unknown_condition"`
	NotApplicableCondition string          `json:"not_applicable_condition"`
	EvidenceFields         []string        `json:"evidence_fields"`
	SourceRefs             []string        `json:"source_refs"`
	Guardrail              string          `json:"guardrail,omitempty"`
}

// Validate validates that a RuleDefinition strictly conforms to the frozen rule contract.
func (r RuleDefinition) Validate() error {
	if r.RuleID == "" {
		return fmt.Errorf("audit: rule_id cannot be empty")
	}
	if !strings.HasPrefix(r.RuleID, "AR-") {
		return fmt.Errorf("audit: rule_id %q must start with 'AR-' prefix", r.RuleID)
	}
	if r.ParentCheck == "" {
		return fmt.Errorf("audit: rule %s parent_check cannot be empty", r.RuleID)
	}
	if r.RuleVersion <= 0 {
		return fmt.Errorf("audit: rule %s rule_version must be positive, got %d", r.RuleID, r.RuleVersion)
	}
	if r.Name == "" {
		return fmt.Errorf("audit: rule %s name cannot be empty", r.RuleID)
	}
	if r.Category == "" {
		return fmt.Errorf("audit: rule %s category cannot be empty", r.RuleID)
	}
	if err := ValidateLifecycleStage(r.Lifecycle); err != nil {
		return fmt.Errorf("audit: rule %s lifecycle invalid: %w", r.RuleID, err)
	}
	if err := ValidateAutomationClass(r.Automation); err != nil {
		return fmt.Errorf("audit: rule %s automation invalid: %w", r.RuleID, err)
	}
	if err := ValidateSeverity(r.DefaultSeverity); err != nil {
		return fmt.Errorf("audit: rule %s default_severity invalid: %w", r.RuleID, err)
	}
	if len(r.RequiredInputs) == 0 {
		return fmt.Errorf("audit: rule %s required_inputs cannot be empty", r.RuleID)
	}
	if len(r.Preconditions) == 0 {
		return fmt.Errorf("audit: rule %s preconditions cannot be empty", r.RuleID)
	}
	if r.PassCondition == "" {
		return fmt.Errorf("audit: rule %s pass_condition cannot be empty", r.RuleID)
	}
	if r.WarningCondition == "" {
		return fmt.Errorf("audit: rule %s warning_condition cannot be empty", r.RuleID)
	}
	if r.FailCondition == "" {
		return fmt.Errorf("audit: rule %s fail_condition cannot be empty", r.RuleID)
	}
	if r.ManualReviewCondition == "" {
		return fmt.Errorf("audit: rule %s manual_review_condition cannot be empty", r.RuleID)
	}
	if r.UnknownCondition == "" {
		return fmt.Errorf("audit: rule %s unknown_condition cannot be empty", r.RuleID)
	}
	if r.NotApplicableCondition == "" {
		return fmt.Errorf("audit: rule %s not_applicable_condition cannot be empty", r.RuleID)
	}
	if len(r.EvidenceFields) == 0 {
		return fmt.Errorf("audit: rule %s evidence_fields cannot be empty", r.RuleID)
	}
	if len(r.SourceRefs) == 0 {
		return fmt.Errorf("audit: rule %s source_refs cannot be empty", r.RuleID)
	}
	return nil
}
