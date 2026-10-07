package catalog

import (
	"testing"

	"lina-core/pkg/bizerr"
)

func TestValidateConfigurationGraphAcceptsFixedAndConfigurableModels(t *testing.T) {
	t.Parallel()
	fixedGroups := []GroupDefinition{
		{GroupNo: "group-cpu", SelectionMode: SelectionSingle, MinimumSelected: 1, MaximumSelected: 1},
		{GroupNo: "group-memory", SelectionMode: SelectionSingle, MinimumSelected: 1, MaximumSelected: 1},
	}
	fixedOptions := []OptionDefinition{
		{OptionNo: "option-cpu", GroupNo: "group-cpu", MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1, Active: true},
		{OptionNo: "option-memory", GroupNo: "group-memory", MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1, Active: true},
	}
	if err := ValidateConfigurationGraph(ConfigurationFixed, fixedGroups, fixedOptions); err != nil {
		t.Fatalf("valid fixed graph rejected: %v", err)
	}
	configurableGroups := []GroupDefinition{
		{GroupNo: "group-memory", SelectionMode: SelectionSingle, MinimumSelected: 1, MaximumSelected: 1},
		{GroupNo: "group-disk", SelectionMode: SelectionQuantity, MinimumSelected: 0, MaximumSelected: 2},
	}
	configurableOptions := []OptionDefinition{
		{OptionNo: "option-memory-2g", GroupNo: "group-memory", MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1, Active: true},
		{OptionNo: "option-memory-4g", GroupNo: "group-memory", MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1, Active: true},
		{OptionNo: "option-disk-ssd", GroupNo: "group-disk", MinimumQuantity: 0, MaximumQuantity: 8, DefaultQuantity: 0, Active: true},
		{OptionNo: "option-disk-nvme", GroupNo: "group-disk", MinimumQuantity: 0, MaximumQuantity: 4, DefaultQuantity: 0, Active: true},
	}
	if err := ValidateConfigurationGraph(
		ConfigurationConfigurable,
		configurableGroups,
		configurableOptions,
	); err != nil {
		t.Fatalf("valid configurable graph rejected: %v", err)
	}
}

func TestValidateConfigurationGraphRejectsParallelOrAmbiguousFixedSemantics(t *testing.T) {
	t.Parallel()
	validGroup := GroupDefinition{
		GroupNo: "group-memory", SelectionMode: SelectionSingle, MinimumSelected: 1, MaximumSelected: 1,
	}
	validOption := OptionDefinition{
		OptionNo: "option-memory", GroupNo: "group-memory",
		MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1, Active: true,
	}
	cases := []struct {
		mode    ConfigurationMode
		groups  []GroupDefinition
		options []OptionDefinition
	}{
		{"unknown", []GroupDefinition{validGroup}, []OptionDefinition{validOption}},
		{ConfigurationFixed, []GroupDefinition{{GroupNo: "group-memory", SelectionMode: SelectionMultiple, MinimumSelected: 1, MaximumSelected: 1}}, []OptionDefinition{validOption}},
		{ConfigurationFixed, []GroupDefinition{validGroup}, []OptionDefinition{validOption, {
			OptionNo: "option-memory-2", GroupNo: "group-memory",
			MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1, Active: true,
		}}},
		{ConfigurationFixed, []GroupDefinition{validGroup}, []OptionDefinition{{
			OptionNo: "option-memory", GroupNo: "group-memory",
			MinimumQuantity: 1, MaximumQuantity: 2, DefaultQuantity: 1, Active: true,
		}}},
		{ConfigurationConfigurable, []GroupDefinition{validGroup}, []OptionDefinition{{
			OptionNo: "option-memory", GroupNo: "missing-group",
			MinimumQuantity: 1, MaximumQuantity: 1, DefaultQuantity: 1, Active: true,
		}}},
		{ConfigurationConfigurable, []GroupDefinition{validGroup}, []OptionDefinition{{
			OptionNo: "option-memory", GroupNo: "group-memory",
			MinimumQuantity: 0, MaximumQuantity: 0, DefaultQuantity: 0, Active: true,
		}}},
	}
	for index, testCase := range cases {
		if err := ValidateConfigurationGraph(testCase.mode, testCase.groups, testCase.options); !bizerr.Is(err, CodeConfigurationInvalid) {
			t.Fatalf("case=%d returned %v", index, err)
		}
	}
}
