// This file verifies optional field upgrades preserve the complete old contract
// and rejects changes to required fields, routing, credentials and behavior.

package productfulfillmentcap

import (
	"encoding/json"
	"reflect"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

// A connector may correct help text without changing the accepted payload.
// Existing products must remain readable, while technical changes still fail.
func TestConfigurationDescriptionUpdateRetainsTechnicalContract(t *testing.T) {
	previous := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Minimum: "1", Maximum: "4", Required: true, Description: &capmodel.LocalizedText{ZhCN: "留空使用套餐默认配置", EnUS: "Leave blank for the package size"}}}}
	for _, description := range []*capmodel.LocalizedText{nil, {ZhCN: "选择需要的核心数", EnUS: "Select the required core count"}} {
		current := *previous
		field := *previous.Fields[0]
		field.Description = description
		current.Fields = []*capmodel.ConfigurationFieldDeclaration{&field}
		if !IsAdditiveConfigurationSchema(previous, &current) || !IsAdditiveConfigurationSchema(&current, previous) {
			t.Fatal("help text update invalidated an unchanged technical contract")
		}
		if previous.Fields[0].Description.ZhCN != "留空使用套餐默认配置" || !reflect.DeepEqual(field.Description, description) {
			t.Fatal("compatibility comparison mutated a declaration")
		}
		for _, change := range []func(*capmodel.ConfigurationFieldDeclaration){
			func(f *capmodel.ConfigurationFieldDeclaration) { f.Minimum = "2" },
			func(f *capmodel.ConfigurationFieldDeclaration) { f.Maximum = "8" },
			func(f *capmodel.ConfigurationFieldDeclaration) { f.Type = capmodel.ConfigurationFieldSingleLine },
			func(f *capmodel.ConfigurationFieldDeclaration) { f.Required = false },
			func(f *capmodel.ConfigurationFieldDeclaration) {
				f.DynamicCandidates = &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "configuration.candidates"}
			},
		} {
			changed := field
			change(&changed)
			current.Fields = []*capmodel.ConfigurationFieldDeclaration{&changed}
			if IsAdditiveConfigurationSchema(previous, &current) {
				t.Fatal("help text update hid a changed technical contract")
			}
		}
	}
}

func TestAdditiveConfigurationUpgradePreservesExistingContract(t *testing.T) {
	previous := &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "connector", Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", AuthorizedMethods: []string{"provision", "query"},
		InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine, Required: true}}},
		ProductConfiguration:   &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cpu", Type: capmodel.ConfigurationFieldInteger, Required: true, Minimum: "1"}}},
		CustomerConfiguration:  &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "operating_system", Type: capmodel.ConfigurationFieldSingleSelect, Required: true}}},
		ProviderMetadata:       &capmodel.BusinessExtensionProviderMetadata{Type: ProviderMetadataType, Version: ProviderMetadataVersion, Values: map[string]any{"merchantCustomerFields": []string{}, "ongoingManagement": true, "expiryManagement": "unsupported"}},
	}
	clone := func() *capmodel.BusinessExtensionProviderEvidence {
		raw, err := json.Marshal(previous)
		if err != nil {
			t.Fatal(err)
		}
		var next capmodel.BusinessExtensionProviderEvidence
		if err = json.Unmarshal(raw, &next); err != nil {
			t.Fatal(err)
		}
		next.ProductConfiguration.Fields = append(next.ProductConfiguration.Fields, &capmodel.ConfigurationFieldDeclaration{Key: "inbound_bandwidth", Type: capmodel.ConfigurationFieldInteger, Minimum: "1"})
		next.CustomerConfiguration.Fields = append(next.CustomerConfiguration.Fields, &capmodel.ConfigurationFieldDeclaration{Key: "ip_count", Type: capmodel.ConfigurationFieldInteger, Minimum: "0"})
		next.ProviderMetadata.Values["merchantCustomerFields"] = []string{"ip_count"}
		return &next
	}
	if !IsAdditiveConfigurationUpgrade(previous, clone()) {
		t.Fatal("optional fields blocked the unchanged old configuration")
	}
	for _, test := range []struct {
		name   string
		change func(*capmodel.BusinessExtensionProviderEvidence)
	}{
		{"required addition", func(v *capmodel.BusinessExtensionProviderEvidence) { v.CustomerConfiguration.Fields[1].Required = true }},
		{"changed size bound", func(v *capmodel.BusinessExtensionProviderEvidence) { v.ProductConfiguration.Fields[0].Minimum = "2" }},
		{"removed field", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.ProductConfiguration.Fields = v.ProductConfiguration.Fields[1:]
		}},
		{"changed credential schema", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.InterfaceConfiguration.Fields[0].Type = capmodel.ConfigurationFieldSecret
		}},
		{"changed credential description", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.InterfaceConfiguration.Fields[0].Description = &capmodel.LocalizedText{ZhCN: "接口凭据说明", EnUS: "Credential description"}
		}},
		{"changed expiry owner", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.ProviderMetadata.Values["expiryManagement"] = "provider"
		}},
		{"changed method", func(v *capmodel.BusinessExtensionProviderEvidence) { v.AuthorizedMethods = []string{"create", "query"} }},
		{"new secret", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.CustomerConfiguration.Fields[1].Type = capmodel.ConfigurationFieldSecret
		}},
		{"unproved merchant field", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.ProviderMetadata.Values["merchantCustomerFields"] = []string{"unknown"}
		}},
		{"new condition", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.CustomerConfiguration.Conditions = []*capmodel.ConfigurationRuleDeclaration{{When: &capmodel.ConfigurationConditionDeclaration{Field: "operating_system", Operator: capmodel.ConfigurationConditionEmpty}}}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			next := clone()
			test.change(next)
			if IsAdditiveConfigurationUpgrade(previous, next) {
				t.Fatal("incompatible contract accepted")
			}
		})
	}
	if len(previous.ProductConfiguration.Fields) != 1 || len(previous.CustomerConfiguration.Fields) != 1 {
		t.Fatal("historical declaration mutated")
	}
}

// TestStaticChoiceNormalizationDoesNotInvalidateSavedSettings reproduces host
// canonical ordering of hostname/password choices across connector upgrades.
func TestStaticChoiceNormalizationDoesNotInvalidateSavedSettings(t *testing.T) {
	old := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "characters", Type: capmodel.ConfigurationFieldMultiSelect, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "lowercase"}, {Value: "digits"}}}}}
	current := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "characters", Type: capmodel.ConfigurationFieldMultiSelect, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "digits"}, {Value: "lowercase"}}}}}
	current.Conditions = []*capmodel.ConfigurationRuleDeclaration{}
	current.UploadPolicies = []*capmodel.ConfigurationUploadPolicyDeclaration{}
	if !IsAdditiveConfigurationSchema(old, current) {
		t.Fatal("choice ordering invalidated saved settings")
	}
	if old.Fields[0].StaticCandidates[0].Value != "lowercase" {
		t.Fatal("historical choice order mutated")
	}
	current.Fields[0].StaticCandidates[0].Value = "symbols"
	if IsAdditiveConfigurationSchema(old, current) {
		t.Fatal("removed actual choice was accepted")
	}
}

func TestExplicitCustomerProductSourcesMustBeDeclaredAndAdditive(t *testing.T) {
	label := &capmodel.LocalizedText{ZhCN: "内存", EnUS: "Memory"}
	old := &capmodel.BusinessExtensionProviderEvidence{PluginID: "connector", Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", AuthorizedMethods: []string{"provision", "query"}, ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "memory", Type: capmodel.ConfigurationFieldInteger, Label: label}}}, CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "memory_gib", Type: capmodel.ConfigurationFieldInteger, Label: label}}}, ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Type: ProviderMetadataType, Version: ProviderMetadataVersion, Values: map[string]any{}}}
	next := *old
	metadata := *old.ProviderMetadata
	next.ProviderMetadata = &metadata
	metadata.Values = map[string]any{"customerProductFields": map[string]string{"memory_gib": "memory"}}
	if !IsAdditiveConfigurationUpgrade(old, &next) {
		t.Fatal("explicit alias broke old valid product")
	}
	for _, mapping := range []any{map[string]string{"unknown": "memory"}, map[string]string{"memory_gib": "unknown"}, map[string]any{"memory_gib": 123}} {
		metadata.Values["customerProductFields"] = mapping
		if IsAdditiveConfigurationUpgrade(old, &next) {
			t.Fatal("invalid alias evidence accepted")
		}
	}
}
