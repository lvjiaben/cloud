package resourcecap

import (
	"testing"
	"time"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func TestServerFulfillmentContractKeepsServerSemanticsInResourceOwner(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	contract := validServerFulfillmentContract(t, now)
	if err := contract.ValidateAt(now); err != nil {
		t.Fatalf("valid server fulfillment contract was rejected: %v", err)
	}

	configuration := make(map[string]struct{})
	for _, schema := range []*capmodel.ConfigurationSchemaDeclaration{
		contract.Fulfillment.ProductConfiguration,
		contract.Fulfillment.CustomerConfiguration,
	} {
		for _, field := range schema.Fields {
			configuration[field.Key] = struct{}{}
		}
	}
	for _, key := range ServerConfigurationKeys() {
		if _, ok := configuration[key]; !ok {
			t.Fatalf("server configuration %q is not owned by the server contract", key)
		}
	}
	if contract.Fulfillment.Point != capmodel.BusinessExtensionProductFulfillment ||
		contract.Fulfillment.Owner != "idc-core" {
		t.Fatal("server contract must extend the generic product owner without becoming the generic owner")
	}
}

func TestServerFulfillmentContractRequiresOngoingManagement(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	contract := validServerFulfillmentContract(t, now)
	metadata, err := productfulfillmentcap.NewProviderMetadata(&productfulfillmentcap.ProviderDeclaration{
		Capability:         productfulfillmentcap.ProviderCapability{EnUS: "Server provisioning", ZhCN: "服务器开通"},
		EntitlementOwnerID: "idc-resource",
		Granularity:        productfulfillmentcap.GranularityPerQuantityUnit,
		RenewalSupport:     productfulfillmentcap.RenewalSupportUnsupported,
		ExpiryManagement:   productfulfillmentcap.ExpiryManagementUnsupported,
		MethodRoles:        productfulfillmentcap.ProviderMethodRoles{Provision: "provision", Query: "query"},
	})
	if err != nil {
		t.Fatal(err)
	}
	contract.Fulfillment.ProviderMetadata = metadata
	if err = contract.ValidateAt(now); err == nil {
		t.Fatal("server contract without ongoing management was accepted")
	}
}

func TestServerFulfillmentContractRejectsIncompleteFieldsActionsAndInvalidDeclaredSurfaces(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)

	t.Run("missing server field", func(t *testing.T) {
		contract := validServerFulfillmentContract(t, now)
		contract.Fulfillment.CustomerConfiguration.Fields = contract.Fulfillment.CustomerConfiguration.Fields[:0]
		if err := contract.ValidateAt(now); err == nil {
			t.Fatal("incomplete server configuration was accepted")
		}
	})

	t.Run("missing management action", func(t *testing.T) {
		contract := validServerFulfillmentContract(t, now)
		contract.Capabilities.Capabilities = contract.Capabilities.Capabilities[:len(contract.Capabilities.Capabilities)-1]
		if err := contract.ValidateAt(now); err == nil {
			t.Fatal("incomplete server action matrix was accepted")
		}
	})

	t.Run("shared owner pages do not require connector surfaces", func(t *testing.T) {
		contract := validServerFulfillmentContract(t, now)
		contract.AdminSurfaces = nil
		contract.CustomerSurfaces = nil
		if err := contract.ValidateAt(now); err != nil {
			t.Fatalf("server connector without independent surfaces was rejected: %v", err)
		}
	})

	t.Run("invalid declared surface", func(t *testing.T) {
		contract := validServerFulfillmentContract(t, now)
		contract.AdminSurfaces[0].Entry = ""
		if err := contract.ValidateAt(now); err == nil {
			t.Fatal("incomplete connector-owned administration surface was accepted")
		}
	})
}

func validServerFulfillmentContract(t *testing.T, now time.Time) ServerFulfillmentContract {
	t.Helper()
	metadata, err := productfulfillmentcap.NewProviderMetadata(&productfulfillmentcap.ProviderDeclaration{
		Capability:         productfulfillmentcap.ProviderCapability{EnUS: "Server provisioning and management", ZhCN: "服务器开通与管理"},
		EntitlementOwnerID: "idc-resource",
		Granularity:        productfulfillmentcap.GranularityPerQuantityUnit,
		OngoingManagement:  true,
		RenewalSupport:     productfulfillmentcap.RenewalSupportSupported,
		ExpiryManagement:   productfulfillmentcap.ExpiryManagementProvider,
		MethodRoles:        productfulfillmentcap.ProviderMethodRoles{Provision: "provision", Query: "query", Renewal: "renew"},
	})
	if err != nil {
		t.Fatal(err)
	}
	return ServerFulfillmentContract{
		Version: ServerFulfillmentContractVersion,
		Fulfillment: &capmodel.BusinessExtensionDeclaration{
			Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1",
			Role: capmodel.BusinessExtensionRoleProvider, Owner: "idc-core",
			Methods:          []string{"configuration.candidates", "configuration.validate", "provision", "query", "renew"},
			ProviderMetadata: metadata,
			ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
				serverSelectField(ServerConfigurationRegion),
				serverSelectField(ServerConfigurationLine),
				serverNumberField(ServerConfigurationCPU),
				serverNumberField(ServerConfigurationMemory),
				serverNumberField(ServerConfigurationSystemDisk),
				serverNumberField(ServerConfigurationDataDisk),
				serverNumberField(ServerConfigurationBandwidth),
			}},
			CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{
				serverSelectField(ServerConfigurationOperatingSystem),
			}},
		},
		Capabilities: &CapabilitySnapshot{
			AdapterVersion: "v1.0.0", ObservedAt: now,
			Capabilities: explicitServerCapabilityMatrix(),
		},
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "server-management", Version: "v1", Title: "服务器管理",
			Permission: "idc:admin:resource:server:view", Slot: capmodel.FrontendSlotAdminResourceOperations,
			Route: "servers", Entry: "frontend/server-management.js",
		}},
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "servers", Version: "v1", Title: "云服务器",
			Slot: capmodel.FrontendSlotCustomerProducts, Route: "servers", Entry: "frontend/customer-servers.js",
		}},
	}
}

func serverSelectField(key string) *capmodel.ConfigurationFieldDeclaration {
	return &capmodel.ConfigurationFieldDeclaration{
		Key: key, Type: capmodel.ConfigurationFieldSingleSelect,
		Label: &capmodel.LocalizedText{EnUS: key, ZhCN: key},
	}
}

func serverNumberField(key string) *capmodel.ConfigurationFieldDeclaration {
	return &capmodel.ConfigurationFieldDeclaration{
		Key: key, Type: capmodel.ConfigurationFieldInteger,
		Label: &capmodel.LocalizedText{EnUS: key, ZhCN: key}, Minimum: "0",
	}
}

func explicitServerCapabilityMatrix() []CapabilityDescriptor {
	codes := SupportedCapabilityCodes()
	result := make([]CapabilityDescriptor, 0, len(codes))
	for _, code := range codes {
		result = append(result, CapabilityDescriptor{Code: code, Support: SupportLevelSupported})
	}
	return result
}
