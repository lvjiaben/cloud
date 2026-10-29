// This file verifies optional principal methods never weaken a frozen interface.
package resourcecap

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"testing"
)

func TestOptionalPrincipalUpgradeRequiresCompleteUnchangedOldContract(t *testing.T) {

	previous := &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "connector", Point: DynamicProviderPoint, ContractVersion: "v1", AuthorizedMethods: DynamicProviderMethods(),
		InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalityMultiple,
		InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			{Key: "username", Type: capmodel.ConfigurationFieldSecret, Label: &capmodel.LocalizedText{EnUS: "Username", ZhCN: "用户名"}},
		}},
		ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Type: DynamicProviderMetadataType, Version: "v1", Values: map[string]any{
			"providerId": "zjmfcloud-v1",
			"authenticationProfiles": map[string]any{"profiles": map[string]any{"default": map[string]any{
				"strategy": "per_request_form", "fields": map[string]any{"username": "username"},
			}}},
		}},
	}

	clone := func() *capmodel.BusinessExtensionProviderEvidence {
		raw, _ := json.Marshal(previous)
		var current capmodel.BusinessExtensionProviderEvidence
		if err := json.Unmarshal(raw, &current); err != nil {
			t.Fatal(err)
		}
		current.AuthorizedMethods = append(current.AuthorizedMethods, DynamicProviderMethodLookupPrincipal, DynamicProviderMethodCreatePrincipal)
		return &current
	}
	if !IsOptionalPrincipalUpgrade(previous, clone()) {
		t.Fatal("safe paired addition rejected")
	}
	for _, test := range []struct {
		name   string
		change func(*capmodel.BusinessExtensionProviderEvidence)
	}{
		{"partial pair", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.AuthorizedMethods = v.AuthorizedMethods[:len(v.AuthorizedMethods)-1]
		}},
		{"removed old method", func(v *capmodel.BusinessExtensionProviderEvidence) { v.AuthorizedMethods = v.AuthorizedMethods[1:] }},
		{"unrelated method", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.AuthorizedMethods = append(v.AuthorizedMethods, "secret.inspect")
		}},
		{"credential schema", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.InterfaceConfiguration.Fields[0].Required = true
		}},
		{"provider behavior", func(v *capmodel.BusinessExtensionProviderEvidence) {
			v.ProviderMetadata.Values["providerId"] = "another-provider"
		}},
		{"missing declaration", func(v *capmodel.BusinessExtensionProviderEvidence) { v.ProviderMetadata = nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			current := clone()
			test.change(current)
			if IsOptionalPrincipalUpgrade(previous, current) {
				t.Fatal("incompatible extension accepted")
			}
		})
	}
}
