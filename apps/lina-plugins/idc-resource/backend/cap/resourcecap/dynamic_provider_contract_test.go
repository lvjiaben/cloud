// This file verifies the dynamic resource Provider declaration and its
// secret-free operation projection.

package resourcecap

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/secretcap"
)

func TestDynamicProviderDeclarationRequiresCompleteOwnerContract(t *testing.T) {
	t.Parallel()
	declaration := &capmodel.BusinessExtensionDeclaration{
		Point: DynamicProviderPoint, Version: DynamicProviderContractVersion,
		Role: capmodel.BusinessExtensionRoleProvider, Owner: OwnerPluginID,
		Methods:                           DynamicProviderMethods(),
		InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalityMultiple,
		InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
			Key: "protocol", Type: capmodel.ConfigurationFieldSingleSelect,
			Label:            &capmodel.LocalizedText{EnUS: "Protocol", ZhCN: "接口协议"},
			StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "legacy", Label: &capmodel.LocalizedText{EnUS: "Legacy", ZhCN: "旧版"}}},
		}, {Key: "username", Type: capmodel.ConfigurationFieldSecret, Label: &capmodel.LocalizedText{EnUS: "Username", ZhCN: "用户名"}}}},
		ProviderMetadata: &capmodel.BusinessExtensionProviderMetadata{
			Type: DynamicProviderMetadataType, Version: DynamicProviderContractVersion,
			Values: map[string]any{"providerId": "zjmfcloud-v1", "interfaceSettingsProjection": []any{"protocol"},
				"authenticationProfiles": map[string]any{"selectorField": "protocol", "profiles": map[string]any{"legacy": map[string]any{"strategy": "per_request_form", "fields": map[string]any{"username": "username"}}}}},
		},
	}
	if err := ValidateDynamicProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}); err != nil {
		t.Fatalf("validate complete dynamic Provider declaration: %v", err)
	}
	declaration.Methods = append(DynamicProviderMethods(), DynamicProviderMethodListCatalog, DynamicProviderMethodReadCatalogProduct)
	if err := ValidateDynamicProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}); err != nil {
		t.Fatalf("validate paired read-only catalog extension: %v", err)
	}
	declaration.Methods = append(DynamicProviderMethods(), DynamicProviderMethodListCatalog)
	if ValidateDynamicProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}) == nil {
		t.Fatal("partial catalog extension was accepted")
	}
	declaration.Methods = append(DynamicProviderMethods(), DynamicProviderMethodListCatalog, "catalog.products.publish")
	if ValidateDynamicProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}) == nil {
		t.Fatal("connector catalog write capability was accepted")
	}
	declaration.Methods = append(DynamicProviderMethods(), DynamicProviderMethodLookupPrincipal, DynamicProviderMethodCreatePrincipal)
	if err := ValidateDynamicProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}); err != nil {
		t.Fatal(err)
	}
	declaration.Methods = append(DynamicProviderMethods(), DynamicProviderMethodLookupPrincipal)
	if ValidateDynamicProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}) == nil {
		t.Fatal("partial principal extension accepted")
	}
	declaration.Methods = DynamicProviderMethods()
	declaration.Methods = declaration.Methods[:len(declaration.Methods)-1]
	if err := ValidateDynamicProviderDeclarations([]*capmodel.BusinessExtensionDeclaration{declaration}); err == nil {
		t.Fatal("incomplete dynamic Provider methods were accepted")
	}
}

func TestDynamicOperationProjectionNeverSerializesSecretHandles(t *testing.T) {
	t.Parallel()
	handle, err := secretcap.NewHandle("opaque-encrypted-reference")
	if err != nil {
		t.Fatal(err)
	}
	projected, err := ProjectDynamicOperationRequest(OperationRequest{
		RequestID: "request-1", Capability: CapabilityProvision,
		ResourceRef: "server-1", Parameters: json.RawMessage(`{}`),
		SecretBindings:      []SecretBinding{{Name: "password", Handle: handle}, {Name: "downstream_token", Handle: handle}},
		ProviderCorrelation: &ProviderCorrelation{DownstreamID: 91, DownstreamURL: "https://platform.example.test/api/plugins/idc-resource/provider-callback"},
	})
	if err != nil {
		t.Fatalf("project dynamic operation: %v", err)
	}
	payload, err := json.Marshal(projected)
	if err != nil {
		t.Fatalf("marshal projected operation: %v", err)
	}
	content, err := NewDynamicProviderInvocationEnvelope(json.RawMessage(`{"protocol":"bms"}`), payload)
	if err != nil {
		t.Fatalf("marshal projected operation: %v", err)
	}
	text := string(content)
	if strings.Contains(text, "opaque-encrypted-reference") || strings.Contains(strings.ToLower(text), "handle") ||
		!strings.Contains(text, `"confidentialInputNames":["downstream_token","password"]`) ||
		!strings.Contains(text, `"downstreamId":91`) {
		t.Fatalf("dynamic projection leaked or lost the confidential boundary: %s", text)
	}
}
