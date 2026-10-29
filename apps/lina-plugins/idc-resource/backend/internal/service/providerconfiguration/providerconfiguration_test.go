package providerconfiguration

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	providerspi "lina-plugin-idc-resource/backend/cap/resourcecap/spi"
)

func TestValidEndpointAcceptsExplicitHTTPAndHTTPSRoots(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "valid", value: "https://api.example.com/v1", want: true},
		{name: "valid trailing slash", value: "https://api.example.com/v1/", want: true},
		{name: "connector owned path", value: "https://api.example.com/provider/administrator", want: true},
		{name: "explicit http", value: "http://api.example.com/v1", want: true},
		{name: "unsupported scheme", value: "ftp://api.example.com/v1", want: false},
		{name: "empty query", value: "http://api.example.com/v1?", want: false},
		{name: "credentials rejected", value: "https://user:pass@api.example.com/v1", want: false},
		{name: "query rejected", value: "https://api.example.com/v1?token=secret", want: false},
		{name: "path traversal rejected", value: "https://api.example.com/../private", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validEndpoint(test.value); got != test.want {
				t.Fatalf("validEndpoint(%q)=%v, want %v", test.value, got, test.want)
			}
		})
	}
}

type admissionRegistrar struct {
	evidence []*capmodel.BusinessExtensionProviderEvidence
	stopAt   int
	calls    int
}

func (*admissionRegistrar) ListProviders(context.Context, plugincap.BusinessExtensionProviderListInput) (*capmodel.PageResult[*capmodel.BusinessExtensionProviderEvidence], error) {
	return nil, nil
}
func (*admissionRegistrar) GetProviderExact(context.Context, plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, nil
}
func (*admissionRegistrar) GetProvidersExact(context.Context, []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	return nil, nil
}
func (registrar *admissionRegistrar) WithNewBusinessAdmission(ctx context.Context, _ pluginhost.BusinessExtensionAdmissionInput, action capmodel.BusinessExtensionAdmissionAction) error {
	registrar.calls++
	if registrar.calls == registrar.stopAt {
		return context.Canceled
	}
	return action(ctx, registrar.evidence[registrar.calls-1])
}
func (*admissionRegistrar) InvokeProviderMethod(context.Context, pluginhost.BusinessExtensionMethodInvokeInput) (*capmodel.BusinessExtensionMethodResult, error) {
	return nil, nil
}
func (*admissionRegistrar) WithConfidentialOutboundContext(context.Context, pluginhost.ConfidentialOutboundContextInput, pluginhost.ConfidentialOutboundAction) error {
	return nil
}

func TestDynamicChannelAdmissionUsesOnePluginLifecycleFence(t *testing.T) {
	resourceEvidence := &capmodel.BusinessExtensionProviderEvidence{
		PluginID: "server-connector", Point: resourcecap.DynamicProviderPoint,
		ReleaseVersion: "v1.0.0", ContractVersion: "v1",
		ReleaseDigest: strings.Repeat("a", 64), DeclarationDigest: strings.Repeat("b", 64),
	}
	productEvidence := validProductEvidence(t, resourceEvidence)

	t.Run("both exact", func(t *testing.T) {
		registrar := &admissionRegistrar{evidence: []*capmodel.BusinessExtensionProviderEvidence{resourceEvidence}}
		service := &service{extensions: registrar}
		called := false
		if err := service.withDynamicAdmissions(t.Context(), resourceEvidence, productEvidence, func(context.Context) error { called = true; return nil }); err != nil || !called || registrar.calls != 1 {
			t.Fatalf("single plugin admission err=%v called=%t calls=%d", err, called, registrar.calls)
		}
	})

	t.Run("resource stopped", func(t *testing.T) {
		registrar := &admissionRegistrar{evidence: []*capmodel.BusinessExtensionProviderEvidence{resourceEvidence}, stopAt: 1}
		service := &service{extensions: registrar}
		called := false
		if err := service.withDynamicAdmissions(t.Context(), resourceEvidence, productEvidence, func(context.Context) error { called = true; return nil }); err == nil || called {
			t.Fatalf("stopped resource point admitted channel: err=%v called=%t", err, called)
		}
	})

	t.Run("product release differs", func(t *testing.T) {
		changed := *productEvidence
		changed.ReleaseDigest = strings.Repeat("c", 64)
		registrar := &admissionRegistrar{evidence: []*capmodel.BusinessExtensionProviderEvidence{resourceEvidence}}
		service := &service{extensions: registrar}
		called := false
		if err := service.withDynamicAdmissions(t.Context(), resourceEvidence, &changed, func(context.Context) error { called = true; return nil }); err == nil || called {
			t.Fatalf("different product release admitted channel: err=%v called=%t", err, called)
		}
	})
}

func validProductEvidence(t *testing.T, resourceEvidence *capmodel.BusinessExtensionProviderEvidence) *capmodel.BusinessExtensionProviderEvidence {
	t.Helper()
	metadata, err := productfulfillmentcap.NewProviderMetadata(&productfulfillmentcap.ProviderDeclaration{
		Capability:         productfulfillmentcap.ProviderCapability{EnUS: "Server", ZhCN: "服务器"},
		EntitlementOwnerID: "idc-resource", Granularity: productfulfillmentcap.GranularityPerQuantityUnit,
		OngoingManagement: true, RenewalSupport: productfulfillmentcap.RenewalSupportSupported,
		ExpiryManagement: productfulfillmentcap.ExpiryManagementProvider,
		MethodRoles:      productfulfillmentcap.ProviderMethodRoles{Provision: "provision", Query: "query", Renewal: "renew"},
	})
	if err != nil {
		t.Fatal(err)
	}
	selectField := func(key string) *capmodel.ConfigurationFieldDeclaration {
		return &capmodel.ConfigurationFieldDeclaration{Key: key, Type: capmodel.ConfigurationFieldSingleSelect, Label: &capmodel.LocalizedText{EnUS: key, ZhCN: key}}
	}
	numberField := func(key string) *capmodel.ConfigurationFieldDeclaration {
		return &capmodel.ConfigurationFieldDeclaration{Key: key, Type: capmodel.ConfigurationFieldInteger, Label: &capmodel.LocalizedText{EnUS: key, ZhCN: key}, Minimum: "0"}
	}
	return &capmodel.BusinessExtensionProviderEvidence{
		PluginID: resourceEvidence.PluginID, Point: capmodel.BusinessExtensionProductFulfillment,
		ReleaseVersion: resourceEvidence.ReleaseVersion, ContractVersion: "v1",
		ReleaseDigest: resourceEvidence.ReleaseDigest, DeclarationDigest: strings.Repeat("d", 64),
		AuthorizedMethods: []string{"configuration.candidates", "configuration.validate", "provision", "query", "renew"},
		ProviderMetadata:  metadata,
		ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			selectField(resourcecap.ServerConfigurationRegion), selectField(resourcecap.ServerConfigurationLine),
			numberField(resourcecap.ServerConfigurationCPU), numberField(resourcecap.ServerConfigurationMemory),
			numberField(resourcecap.ServerConfigurationSystemDisk), numberField(resourcecap.ServerConfigurationDataDisk),
			numberField(resourcecap.ServerConfigurationBandwidth),
		}},
		CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
			selectField(resourcecap.ServerConfigurationOperatingSystem),
		}},
	}
}

func TestDynamicProviderCandidatesAreValidatedDeduplicatedAndSorted(t *testing.T) {
	interfaceSchema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "username", Type: capmodel.ConfigurationFieldSecret}}}
	metadata := func(providerID string) *capmodel.BusinessExtensionProviderMetadata {
		return &capmodel.BusinessExtensionProviderMetadata{Type: resourcecap.DynamicProviderMetadataType, Version: resourcecap.DynamicProviderContractVersion, Values: map[string]any{
			"providerId": providerID,
			"authenticationProfiles": map[string]any{"profiles": map[string]any{"default": map[string]any{
				"strategy": "login_session", "fields": map[string]any{"username": "username"}, "loginPath": "/login",
				"tokenJSONPath": []any{"jwt"}, "tokenHeader": "Authorization", "sessionTTLSeconds": 3600,
			}}},
		}}
	}
	items := []*capmodel.BusinessExtensionProviderEvidence{
		{
			PluginID: "server-z", DisplayName: "云服务 Z", ReleaseVersion: "1.2.0",
			InterfaceConfiguration: interfaceSchema, ProviderMetadata: metadata("provider-z"),
		},
		{
			PluginID: "server-a", DisplayName: "云服务 A", ReleaseVersion: "1.0.0",
			InterfaceConfiguration: interfaceSchema, ProviderMetadata: metadata("provider-a"),
		},
		{
			PluginID: "server-a", DisplayName: "重复项", ReleaseVersion: "1.0.0",
			InterfaceConfiguration: interfaceSchema, ProviderMetadata: metadata("provider-a"),
		},
		{PluginID: "invalid", DisplayName: "无效", ReleaseVersion: "1.0.0"},
	}
	candidates := providerCandidatesFromEvidence(items)
	if len(candidates) != 2 || candidates[0].PluginID != "server-a" ||
		candidates[1].PluginID != "server-z" {
		t.Fatalf("unexpected governed Provider candidates: %#v", candidates)
	}
}

func TestBusinessNoUsesOpaqueResourcePrefix(t *testing.T) {
	for _, prefix := range []string{"CHN_", "POL_"} {
		value, err := businessNo(prefix)
		if err != nil {
			t.Fatalf("businessNo(%q): %v", prefix, err)
		}
		if !strings.HasPrefix(value, prefix) || len(value) != len(prefix)+20 {
			t.Fatalf("unexpected opaque resource number %q", value)
		}
	}
}

func TestNonSecretSettingsRejectAdministratorAuthenticationPolicy(t *testing.T) {
	schema := &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "protocol", Type: capmodel.ConfigurationFieldSingleSelect}}}
	for _, payload := range []json.RawMessage{
		json.RawMessage(`{"protocol":"legacy","authStrategy":"per_request_form"}`),
		json.RawMessage(`{"protocol":"legacy","authFields":{"username":"username"}}`),
		json.RawMessage(`{"protocol":"legacy","authTokenHeader":"Authorization"}`),
	} {
		if _, err := normalizeNonSecretSettings(payload, schema); err == nil {
			t.Fatalf("administrator authentication policy was accepted: %s", payload)
		}
	}
}

func TestInterfaceInstanceNoIsIndependentOpaqueProductReference(t *testing.T) {
	t.Parallel()
	value, err := interfaceInstanceNo()
	if err != nil {
		t.Fatalf("interfaceInstanceNo(): %v", err)
	}
	if len(value) != 36 || !strings.HasPrefix(value, "pfi_") || strings.Contains(value, "CHN_") {
		t.Fatalf("unexpected interface instance number %q", value)
	}
}

func TestPoolBoundsRejectUnsafeRoutingValues(t *testing.T) {
	valid := PoolInput{ResourceKind: "virtual_server", RoutingPriority: 10, RoutingWeight: 100}
	if !validPool(valid) {
		t.Fatal("expected ordinary virtual server pool to be valid")
	}
	valid.RoutingWeight = 10001
	if validPool(valid) {
		t.Fatal("expected routing weight beyond the documented bound to be rejected")
	}
}

func TestServerProviderDeclarationsAreSelectedWithoutBrandKnowledge(t *testing.T) {
	t.Parallel()
	declarations := []providerspi.Declaration{
		{
			Key: resourcecap.ProviderKey{
				PluginID: "phone-like-name", ProviderID: "server-api",
				ContractVersion: resourcecap.ContractVersion,
			},
			ProvisionParameters: func(context.Context, resourcecap.ServerProvisionPlan) (json.RawMessage, error) {
				return json.RawMessage(`{"cpu":2}`), nil
			},
		},
		{
			Key: resourcecap.ProviderKey{
				PluginID: "providerCapacity-only", ProviderID: "providerCapacity-api",
				ContractVersion: resourcecap.ContractVersion,
			},
		},
	}
	key, err := selectSoleServerProvider(declarations)
	if err != nil {
		t.Fatalf("selectSoleServerProvider() error = %v", err)
	}
	if key.PluginID != "phone-like-name" || key.ProviderID != "server-api" {
		t.Fatalf("server Provider selection still depends on a brand name: %#v", key)
	}
}

// TestNativePurchaseAdmissionUsesProtocolRatherThanMissingHardware keeps the
// generic native identity contract explicit without weakening server schemas.
func TestNativePurchaseAdmissionUsesProtocolRatherThanMissingHardware(t *testing.T) {
	resource := &capmodel.BusinessExtensionProviderEvidence{PluginID: "native-contract", ReleaseVersion: "v1.0.0", ReleaseDigest: strings.Repeat("a", 64)}
	current := validProductEvidence(t, resource)
	metadata, err := productfulfillmentcap.NewProviderMetadata(&productfulfillmentcap.ProviderDeclaration{SaleImportProtocol: productfulfillmentcap.SaleImportProtocolNativeFinanceV1, Capability: productfulfillmentcap.ProviderCapability{EnUS: "Native sale", ZhCN: "原生转售"}, EntitlementOwnerID: resourcecap.OwnerPluginID, Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingManagement: true, RenewalSupport: productfulfillmentcap.RenewalSupportUnsupported, ExpiryManagement: productfulfillmentcap.ExpiryManagementUnsupported, MethodRoles: productfulfillmentcap.ProviderMethodRoles{Provision: "provision", Query: "query"}})
	if err != nil {
		t.Fatal(err)
	}
	current.ProviderMetadata = metadata
	current.CustomerConfiguration = nil
	current.ProductConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "upstream_product_id", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "configuration.candidates"}}, {Key: "upstream_configuration_revision", Type: capmodel.ConfigurationFieldSingleLine, Required: true, MinimumLength: 64, MaximumLength: 64}}}
	if err = validateProductFulfillmentEvidence(current); err != nil {
		t.Fatalf("explicit native contract was rejected: %v", err)
	}
	current.ProductConfiguration.Fields = current.ProductConfiguration.Fields[:1]
	if validateProductFulfillmentEvidence(current) == nil {
		t.Fatal("native metadata admitted a partial identity schema")
	}
}
