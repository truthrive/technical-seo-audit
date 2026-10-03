package audit

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed rules_v1.json
var rulesV1JSON []byte

// Registry holds the frozen, validated V1 audit rule definitions.
type Registry struct {
	rulesByID map[string]RuleDefinition
	rules     []RuleDefinition
}

// LoadV1Registry decodes, validates, and indexes all 47 frozen atomic rules embedded from rules_v1.json.
// It verifies rule uniqueness, automation splits (30 deterministic / 16 assisted / 1 manual),
// lifecycle stages, severity levels, and mandatory manifest fields without executing any rules.
func LoadV1Registry() (*Registry, error) {
	var rules []RuleDefinition
	if err := json.Unmarshal(rulesV1JSON, &rules); err != nil {
		return nil, fmt.Errorf("audit: failed to unmarshal rules_v1.json: %w", err)
	}

	reg := &Registry{
		rulesByID: make(map[string]RuleDefinition, len(rules)),
		rules:     rules,
	}

	// Validate exact count
	if len(rules) != 47 {
		return nil, fmt.Errorf("audit: expected exactly 47 frozen rules in registry, got %d", len(rules))
	}

	var countDeterministic, countAssisted, countManual int

	for i, r := range rules {
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("audit: rule at index %d (%s) failed validation: %w", i, r.RuleID, err)
		}

		if _, exists := reg.rulesByID[r.RuleID]; exists {
			return nil, fmt.Errorf("audit: duplicate rule ID %q in registry", r.RuleID)
		}

		switch r.Automation {
		case AutomationDeterministic:
			countDeterministic++
		case AutomationAssisted:
			countAssisted++
		case AutomationManual:
			countManual++
		default:
			return nil, fmt.Errorf("audit: unrecognized automation class %q for rule %s", r.Automation, r.RuleID)
		}

		reg.rulesByID[r.RuleID] = r
	}

	// Validate exact automation split: 30 deterministic / 16 assisted / 1 manual
	if countDeterministic != 30 {
		return nil, fmt.Errorf("audit: expected exactly 30 deterministic rules, got %d", countDeterministic)
	}
	if countAssisted != 16 {
		return nil, fmt.Errorf("audit: expected exactly 16 assisted rules, got %d", countAssisted)
	}
	if countManual != 1 {
		return nil, fmt.Errorf("audit: expected exactly 1 manual rule, got %d", countManual)
	}

	return reg, nil
}

// Get returns the definition of a rule by its ID, if present.
func (r *Registry) Get(ruleID string) (RuleDefinition, bool) {
	def, ok := r.rulesByID[ruleID]
	return def, ok
}

// All returns a slice copy of all registered rule definitions.
func (r *Registry) All() []RuleDefinition {
	out := make([]RuleDefinition, len(r.rules))
	copy(out, r.rules)
	return out
}

// Count returns the total number of registered rules.
func (r *Registry) Count() int {
	return len(r.rules)
}

// ByAutomation returns all rules belonging to a given automation class.
func (r *Registry) ByAutomation(class AutomationClass) []RuleDefinition {
	var out []RuleDefinition
	for _, rule := range r.rules {
		if rule.Automation == class {
			out = append(out, rule)
		}
	}
	return out
}

// ByLifecycle returns all rules belonging to a given lifecycle stage.
func (r *Registry) ByLifecycle(stage LifecycleStage) []RuleDefinition {
	var out []RuleDefinition
	for _, rule := range r.rules {
		if rule.Lifecycle == stage {
			out = append(out, rule)
		}
	}
	return out
}
