// This file owns the shared compatibility-edge semantics used by draft writes and publication.

package catalog

import "lina-core/pkg/bizerr"

// CompatibilityRuleType describes how a selected source constrains its target.
type CompatibilityRuleType string

const (
	CompatibilityRequires CompatibilityRuleType = "requires"
	CompatibilityExcludes CompatibilityRuleType = "excludes"
	CompatibilityAllows   CompatibilityRuleType = "allows"
)

// CompatibilityRuleDefinition is the bounded, version-local option edge.
type CompatibilityRuleDefinition struct {
	RuleType              CompatibilityRuleType
	SourceOptionNo        string
	TargetOptionNo        string
	MinimumTargetQuantity int
	MaximumTargetQuantity *int
}

// ValidateCompatibilityRule proves that one edge has an unambiguous quantity meaning.
func ValidateCompatibilityRule(rule CompatibilityRuleDefinition) error {
	if !catalogBusinessNumberPattern.MatchString(rule.SourceOptionNo) ||
		!catalogBusinessNumberPattern.MatchString(rule.TargetOptionNo) ||
		rule.SourceOptionNo == rule.TargetOptionNo ||
		rule.MinimumTargetQuantity < 0 || rule.MinimumTargetQuantity > 100_000 ||
		(rule.MaximumTargetQuantity != nil &&
			(*rule.MaximumTargetQuantity < rule.MinimumTargetQuantity || *rule.MaximumTargetQuantity > 100_000)) {
		return bizerr.NewCode(CodeConfigurationInvalid)
	}
	switch rule.RuleType {
	case CompatibilityRequires:
		if rule.MinimumTargetQuantity < 1 {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
	case CompatibilityExcludes:
		if rule.MinimumTargetQuantity != 0 || rule.MaximumTargetQuantity != nil {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
	case CompatibilityAllows:
		if rule.MaximumTargetQuantity == nil {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
	default:
		return bizerr.NewCode(CodeConfigurationInvalid)
	}
	return nil
}
