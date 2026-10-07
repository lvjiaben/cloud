// This file validates fixed and configurable products on one shared graph model.

package catalog

import (
	"regexp"

	"lina-core/pkg/bizerr"
)

const maximumConfigurationRows = 20000

// ConfigurationMode selects fixed or customer-configurable graph invariants.
type ConfigurationMode string

const (
	ConfigurationFixed        ConfigurationMode = "fixed"
	ConfigurationConfigurable ConfigurationMode = "configurable"
)

// SelectionMode mirrors the persisted configuration-group contract.
type SelectionMode string

const (
	SelectionSingle   SelectionMode = "single"
	SelectionMultiple SelectionMode = "multiple"
	SelectionQuantity SelectionMode = "quantity"
)

// GroupDefinition contains the graph fields required for mode validation.
type GroupDefinition struct {
	GroupNo         string
	SelectionMode   SelectionMode
	MinimumSelected int
	MaximumSelected int
}

// OptionDefinition contains the graph fields required for mode validation.
type OptionDefinition struct {
	OptionNo        string
	GroupNo         string
	MinimumQuantity int
	MaximumQuantity int
	DefaultQuantity int
	Active          bool
}

var catalogBusinessNumberPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$`)

// ValidateConfigurationGraph proves one shared graph satisfies its fixed or
// configurable customer-selection semantics.
func ValidateConfigurationGraph(
	mode ConfigurationMode,
	groups []GroupDefinition,
	options []OptionDefinition,
) error {
	if (mode != ConfigurationFixed && mode != ConfigurationConfigurable) ||
		len(groups) > maximumConfigurationRows || len(options) > maximumConfigurationRows {
		return bizerr.NewCode(CodeConfigurationInvalid)
	}
	if mode == ConfigurationFixed && len(groups) == 0 && len(options) == 0 {
		return nil
	}
	if len(groups) == 0 || len(options) == 0 {
		return bizerr.NewCode(CodeConfigurationInvalid)
	}
	groupByNumber := make(map[string]GroupDefinition, len(groups))
	activeCounts := make(map[string]int, len(groups))
	for _, group := range groups {
		if !catalogBusinessNumberPattern.MatchString(group.GroupNo) ||
			!validGroupSelection(group) {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
		if _, duplicate := groupByNumber[group.GroupNo]; duplicate {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
		groupByNumber[group.GroupNo] = group
	}
	seenOptions := make(map[string]struct{}, len(options))
	for _, option := range options {
		if !validOptionDefinition(option) {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
		if _, exists := groupByNumber[option.GroupNo]; !exists {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
		if _, duplicate := seenOptions[option.OptionNo]; duplicate {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
		seenOptions[option.OptionNo] = struct{}{}
		if option.Active {
			activeCounts[option.GroupNo]++
		}
	}
	for _, group := range groups {
		activeCount := activeCounts[group.GroupNo]
		if activeCount == 0 || group.MinimumSelected > activeCount ||
			group.MaximumSelected > activeCount {
			return bizerr.NewCode(CodeConfigurationInvalid)
		}
		if mode == ConfigurationFixed {
			if group.SelectionMode != SelectionSingle || group.MinimumSelected != 1 ||
				group.MaximumSelected != 1 || activeCount != 1 ||
				!fixedActiveOption(group.GroupNo, options) {
				return bizerr.NewCode(CodeConfigurationInvalid)
			}
		}
	}
	return nil
}

func validOptionDefinition(option OptionDefinition) bool {
	return catalogBusinessNumberPattern.MatchString(option.OptionNo) &&
		catalogBusinessNumberPattern.MatchString(option.GroupNo) &&
		option.MinimumQuantity >= 0 && option.MaximumQuantity >= option.MinimumQuantity &&
		option.MaximumQuantity <= 100000 && option.DefaultQuantity >= option.MinimumQuantity &&
		option.DefaultQuantity <= option.MaximumQuantity && (!option.Active || option.MaximumQuantity >= 1)
}

func validGroupSelection(group GroupDefinition) bool {
	if group.MinimumSelected < 0 || group.MaximumSelected < group.MinimumSelected ||
		group.MaximumSelected > 1000 {
		return false
	}
	switch group.SelectionMode {
	case SelectionSingle:
		return group.MaximumSelected <= 1
	case SelectionMultiple, SelectionQuantity:
		return true
	default:
		return false
	}
}

func fixedActiveOption(groupNo string, options []OptionDefinition) bool {
	for _, option := range options {
		if option.GroupNo == groupNo && option.Active {
			return option.MinimumQuantity > 0 && option.MinimumQuantity == option.MaximumQuantity &&
				option.DefaultQuantity == option.MinimumQuantity
		}
	}
	return false
}
