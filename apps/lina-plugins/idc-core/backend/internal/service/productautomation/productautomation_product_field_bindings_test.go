// These tests cover explicit parameter bindings without touching live suppliers.
package productautomation

import (
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestProductChoiceBindingsKeepCommercialIdentityAndValidateProviderValues(t *testing.T) {
	exact := customerSelectionEvidence(t)
	exact.CustomerConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1"}
	delete(exact.ProviderMetadata.Values, "merchantCustomerFields")
	label := &capmodel.LocalizedText{ZhCN: "存储等级", EnUS: "Storage profile"}
	exact.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "storage", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "vendor-a", Label: label}, {Value: "vendor-b", Label: label}}}}}
	selection := CustomerFieldSelection{FieldKey: "local_storage", ProductFieldKey: "storage", ProductChoiceBindings: map[string]string{"standard": "vendor-a", "archive": "vendor-b"}, LocalField: &capmodel.ConfigurationFieldDeclaration{Key: "local_storage", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "standard", Label: label}, {Value: "archive", Label: label}}}}
	values, err := boundMerchantValues(exact, nil, []CustomerFieldSelection{selection})
	if err != nil || len(values) != 1 || len(values[0].OptionTokens) != 1 || values[0].OptionTokens[0] != "vendor-a" {
		t.Fatalf("explicit choice mapping was not used: %+v %v", values, err)
	}
	schema, err := selectCustomerSchema(exact, []CustomerFieldSelection{selection})
	if err != nil || schema.Fields[0].StaticCandidates[1].Value != "archive" {
		t.Fatalf("commercial schema changed: %+v %v", schema, err)
	}
	stored, err := compileCustomerPresentations(schema, []CustomerFieldSelection{selection})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := withCustomerPresentations([]CustomerFieldSelection{{FieldKey: selection.FieldKey}}, stored)
	if err != nil || restored[0].ProductChoiceBindings["archive"] != "vendor-b" {
		t.Fatalf("choice bindings lost from owned snapshot: %+v %v", restored, err)
	}
	selection.ProductChoiceBindings["archive"] = "unknown"
	if _, err = boundMerchantValues(exact, nil, []CustomerFieldSelection{selection}); err == nil {
		t.Fatal("undeclared provider value accepted")
	}
	selection.ProductChoiceBindings["archive"] = "vendor-b"
	selection.ProductFieldKey = ""
	if _, err = boundMerchantValues(exact, nil, []CustomerFieldSelection{selection}); err == nil {
		t.Fatal("unbound information retained executable choice mappings")
	}
	if selection.LocalField.StaticCandidates[1].Value != "archive" {
		t.Fatal("commercial input was mutated")
	}
}

func boundCPUSelection() CustomerFieldSelection {
	return CustomerFieldSelection{FieldKey: "local_cores", ProductFieldKey: "cores", Minimum: "2", Maximum: "8", DisplayControl: "select", NumericChoices: []string{"4", "2", "8"}, LocalField: &capmodel.ConfigurationFieldDeclaration{Key: "local_cores", Type: capmodel.ConfigurationFieldInteger, Required: true, Minimum: "2", Maximum: "8", Label: &capmodel.LocalizedText{ZhCN: "CPU", EnUS: "CPU"}}}
}

func TestProductFieldBindingsUseFirstChoiceAndFreezeExplicitTarget(t *testing.T) {
	exact := customerSelectionEvidence(t)
	exact.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cores", Type: capmodel.ConfigurationFieldInteger, Minimum: "1", Maximum: "64", Label: &capmodel.LocalizedText{ZhCN: "核心", EnUS: "Cores"}}}}
	selection := boundCPUSelection()
	values, err := boundMerchantValues(exact, nil, []CustomerFieldSelection{selection})
	if err != nil || len(values) != 1 || values[0].FieldKey != "cores" || values[0].IntegerTextValue == nil || *values[0].IntegerTextValue != "4" {
		t.Fatalf("first choice did not form base parameter: %+v %v", values, err)
	}
	schema, err := selectCustomerSchema(exact, []CustomerFieldSelection{selection})
	if err != nil {
		t.Fatal(err)
	}
	presentation, err := compileCustomerPresentations(schema, []CustomerFieldSelection{selection})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := frozenCustomerSelections(exact, schema)
	if err != nil {
		t.Fatal(err)
	}
	restored, err = withCustomerPresentations(restored, presentation)
	if err != nil || len(restored) != 1 || restored[0].ProductFieldKey != "cores" {
		t.Fatalf("binding lost from frozen configuration: %+v %v", restored, err)
	}
	if exact.ProductConfiguration.Fields[0].Minimum != "1" {
		t.Fatal("provider declaration was mutated")
	}
	for _, mutate := range []func(*CustomerFieldSelection){
		func(s *CustomerFieldSelection) { s.ProductFieldKey = "invalid name" },
		func(s *CustomerFieldSelection) { s.Minimum = "0" },
		func(s *CustomerFieldSelection) { s.Maximum = "65" },
		func(s *CustomerFieldSelection) { s.LocalField.Required = false },
		func(s *CustomerFieldSelection) { s.LocalField.Type = capmodel.ConfigurationFieldSingleLine },
	} {
		invalid := boundCPUSelection()
		mutate(&invalid)
		if _, err = boundMerchantValues(exact, nil, []CustomerFieldSelection{invalid}); err == nil {
			t.Fatal("invalid binding accepted")
		}
	}
	if _, err = boundMerchantValues(exact, nil, []CustomerFieldSelection{selection, selection}); err == nil {
		t.Fatal("duplicate product target accepted")
	}
	information := boundCPUSelection()
	information.ProductFieldKey = ""
	values, err = boundMerchantValues(exact, nil, []CustomerFieldSelection{information})
	if err != nil || len(values) != 0 {
		t.Fatal("unbound information changed product parameters")
	}
}

func TestBoundStringProfilesKeepCommasAndDeriveFirstChoice(t *testing.T) {
	exact := customerSelectionEvidence(t)
	exact.AuthorizedMethods = append(exact.AuthorizedMethods, "product.configuration.validate")
	exact.ProviderMetadata.Values["productChoiceFields"] = []string{"system_disk_io_limit"}
	label := &capmodel.LocalizedText{ZhCN: "系统盘性能", EnUS: "Disk performance"}
	exact.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "system_disk_io_limit", Type: capmodel.ConfigurationFieldSingleLine, MaximumLength: 43, Label: label}}}
	choice := CustomerFieldSelection{FieldKey: "local_profile", ProductFieldKey: "system_disk_io_limit", LocalField: &capmodel.ConfigurationFieldDeclaration{Key: "local_profile", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "500,500,2000,2000", Label: label}}}}
	values, err := boundMerchantValues(exact, nil, []CustomerFieldSelection{choice})
	if err != nil || len(values) != 1 || values[0].TextValue == nil || *values[0].TextValue != "500,500,2000,2000" {
		t.Fatalf("string profile corrupted: %+v %v", values, err)
	}
	if _, err = selectCustomerSchema(exact, []CustomerFieldSelection{choice}); err != nil {
		t.Fatal(err)
	}
	choice.LocalField.StaticCandidates[0].Value = "premium-storage"
	choice.ProductChoiceBindings = map[string]string{"premium-storage": "500,500,2000,2000"}
	values, err = boundMerchantValues(exact, nil, []CustomerFieldSelection{choice})
	if err != nil || values[0].TextValue == nil || *values[0].TextValue != "500,500,2000,2000" || choice.LocalField.StaticCandidates[0].Value != "premium-storage" {
		t.Fatalf("commercial text profile did not use its explicit provider mapping: %+v %v", values, err)
	}
	choice.LocalField.StaticCandidates[0].Value = "500,500,2000,2000"
	choice.ProductChoiceBindings = nil
	delete(exact.ProviderMetadata.Values, "productChoiceFields")
	if _, err = boundMerchantValues(exact, nil, []CustomerFieldSelection{choice}); err == nil {
		t.Fatal("provider without explicit choice validation accepted text profiles")
	}
	exact.ProviderMetadata.Values["productChoiceFields"] = []string{"system_disk_io_limit"}
	choice.LocalField.StaticCandidates[0].Value = "this-choice-is-deliberately-longer-than-the-declared-text-bound"
	if _, err = boundMerchantValues(exact, nil, []CustomerFieldSelection{choice}); err == nil {
		t.Fatal("oversized profile accepted")
	}
}

func TestProductBindingRejectsAlreadySuppliedCustomerAlias(t *testing.T) {
	exact := customerSelectionEvidence(t)
	exact.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cores", Type: capmodel.ConfigurationFieldInteger, Minimum: "1", Maximum: "64"}}}
	exact.ProviderMetadata.Values["customerProductFields"] = map[string]string{"cpu": "cores"}
	if err := validateProductFieldBindings(exact, []CustomerFieldSelection{boundCPUSelection(), {FieldKey: "cpu", Minimum: "1", Maximum: "4"}}); err == nil {
		t.Fatal("two customer options supplied one product parameter")
	}
}

func TestCustomNativeBindingsKeepNamesValuesAndUpgradeControls(t *testing.T) {
	exact := customerSelectionEvidence(t)
	selection := boundCPUSelection()
	selection.ProductFieldKey = "vendor_size"
	disabled := false
	selection.UpgradeEnabled = &disabled
	selection.UpgradeMinimum = "4"
	selection.UpgradeMaximum = "8"
	values, err := boundMerchantValues(exact, nil, []CustomerFieldSelection{selection})
	if err != nil || len(values) != 0 {
		t.Fatalf("custom native parameter polluted semantic base: %+v %v", values, err)
	}
	schema, err := selectCustomerSchema(exact, []CustomerFieldSelection{selection})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := compileCustomerPresentations(schema, []CustomerFieldSelection{selection})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := withCustomerPresentations([]CustomerFieldSelection{{FieldKey: selection.FieldKey}}, stored)
	if err != nil || restored[0].ProductFieldKey != "vendor_size" || restored[0].UpgradeEnabled == nil || *restored[0].UpgradeEnabled || restored[0].UpgradeMinimum != "4" || restored[0].UpgradeMaximum != "8" {
		t.Fatalf("custom configuration was not retained: %+v %v", restored, err)
	}
	if err = validateProductFieldBindings(exact, []CustomerFieldSelection{selection, selection}); err == nil {
		t.Fatal("duplicate custom name accepted")
	}
	selection.ProductFieldKey = "access_token"
	if err = validateProductFieldBindings(exact, []CustomerFieldSelection{selection}); err == nil {
		t.Fatal("credential field accepted")
	}
}
