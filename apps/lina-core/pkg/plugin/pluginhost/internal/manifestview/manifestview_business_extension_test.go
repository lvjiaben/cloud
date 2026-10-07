package manifestview

import (
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestSnapshotDetachesBusinessExtensionsAndFrontendSurfaces(t *testing.T) {
	configuration := func() *capmodel.ConfigurationSchemaDeclaration {
		return &capmodel.ConfigurationSchemaDeclaration{
			Version: "v1",
			Fields: []*capmodel.ConfigurationFieldDeclaration{{
				Key: "region", Label: &capmodel.LocalizedText{EnUS: "Region", ZhCN: "地区"},
				StaticCandidates:  []*capmodel.ConfigurationCandidate{{Value: "a", Label: &capmodel.LocalizedText{EnUS: "A", ZhCN: "甲"}}},
				DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "regions.list"},
			}},
			Conditions: []*capmodel.ConfigurationRuleDeclaration{{
				When:    &capmodel.ConfigurationConditionDeclaration{Field: "region", Operator: "equals", Value: "a"},
				Actions: []*capmodel.ConfigurationActionDeclaration{{Action: "requires", Field: "region", Values: []string{"a"}}},
			}},
			UploadPolicies: []*capmodel.ConfigurationUploadPolicyDeclaration{{
				ID: "proof", AllowedMIMETypes: []string{"image/png"}, AllowedExtensions: []string{".png"},
			}},
		}
	}
	input := &capmodel.ManifestSnapshot{
		ID: "phone-supplier",
		BusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
			Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1",
			Role: capmodel.BusinessExtensionRoleProvider, Owner: "commerce-core",
			Methods: []string{"fulfill", "query", "regions.list"},
			ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{Type: "delivery-contract", Version: "v1", Values: map[string]any{
				"nested": map[string]any{"mode": "single"},
			}},
			InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalityMultiple,
			InterfaceConfiguration:            configuration(), ProductConfiguration: configuration(), CustomerConfiguration: configuration(),
		}},
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "phone-orders", Version: "v1", BridgeCapabilities: []string{"orders.query-own"},
		}},
	}
	snapshot := NewSnapshot(input)
	input.BusinessExtensions[0].Methods[0] = "tampered"
	input.BusinessExtensions[0].ProviderMetadata.Values["nested"].(map[string]any)["mode"] = "tampered"
	input.BusinessExtensions[0].InterfaceConfiguration.Fields[0].Label.EnUS = "tampered"
	input.BusinessExtensions[0].ProductConfiguration.Conditions[0].Actions[0].Values[0] = "tampered"
	input.BusinessExtensions[0].CustomerConfiguration.UploadPolicies[0].AllowedMIMETypes[0] = "tampered"
	input.CustomerSurfaces[0].BridgeCapabilities[0] = "tampered"

	first := snapshot.Values()
	if first.BusinessExtensions[0].Methods[0] != "fulfill" || first.BusinessExtensions[0].ProviderMetadata.Values["nested"].(map[string]any)["mode"] != "single" ||
		first.BusinessExtensions[0].InterfaceConfiguration.Fields[0].Label.EnUS != "Region" ||
		first.BusinessExtensions[0].ProductConfiguration.Conditions[0].Actions[0].Values[0] != "a" ||
		first.BusinessExtensions[0].CustomerConfiguration.UploadPolicies[0].AllowedMIMETypes[0] != "image/png" ||
		first.CustomerSurfaces[0].BridgeCapabilities[0] != "orders.query-own" {
		t.Fatalf("snapshot aliased caller input: %#v", first)
	}
	first.BusinessExtensions[0].Methods[0] = "changed"
	first.BusinessExtensions[0].ProviderMetadata.Values["nested"].(map[string]any)["mode"] = "changed"
	first.BusinessExtensions[0].InterfaceConfiguration.Fields[0].StaticCandidates[0].Label.ZhCN = "changed"
	first.BusinessExtensions[0].ProductConfiguration.Conditions[0].When.Value = "changed"
	first.BusinessExtensions[0].CustomerConfiguration.UploadPolicies[0].AllowedExtensions[0] = "changed"
	first.CustomerSurfaces[0].BridgeCapabilities[0] = "changed"
	second := snapshot.Values()
	if second.BusinessExtensions[0].Methods[0] != "fulfill" || second.BusinessExtensions[0].ProviderMetadata.Values["nested"].(map[string]any)["mode"] != "single" ||
		second.BusinessExtensions[0].InterfaceConfiguration.Fields[0].StaticCandidates[0].Label.ZhCN != "甲" ||
		second.BusinessExtensions[0].ProductConfiguration.Conditions[0].When.Value != "a" ||
		second.BusinessExtensions[0].CustomerConfiguration.UploadPolicies[0].AllowedExtensions[0] != ".png" ||
		second.CustomerSurfaces[0].BridgeCapabilities[0] != "orders.query-own" {
		t.Fatalf("snapshot values exposed mutable state: %#v", second)
	}
}
