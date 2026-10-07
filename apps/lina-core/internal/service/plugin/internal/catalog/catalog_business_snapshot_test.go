package catalog_test

import (
	"testing"

	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/internal/service/plugin/internal/testutil"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
)

func TestManifestSnapshotFreezesAuthorizedBusinessExtensionsAndSeparateSurfaces(t *testing.T) {
	owner := pluginhost.NewDeclarations("commerce-core")
	if err := owner.BusinessExtensions().RegisterProviderValidator(
		capmodel.BusinessExtensionProductFulfillment,
		"v1",
		func([]*capmodel.BusinessExtensionDeclaration) error { return nil },
	); err != nil {
		t.Fatal(err)
	}
	cleanup, err := pluginhost.RegisterSourcePluginForTest(owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)

	manifest := &catalog.Manifest{
		ID: "phone-supplier", Name: "Phone supplier", Version: "v0.1.0",
		Type: "dynamic", Distribution: "managed",
		PublicAssets: []*catalog.PublicAssetSpec{{Source: "frontend", Mount: ""}},
		Dependencies: &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{
			ID: "commerce-core", Version: ">=0.1.0 <1.0.0",
		}}},
		BusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
			Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1",
			Role: capmodel.BusinessExtensionRoleProvider, Owner: "commerce-core",
			Methods:                           []string{"fulfill", "query"},
			InterfaceConfigurationCardinality: capmodel.InterfaceConfigurationCardinalityMultiple,
			InterfaceConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
				Key: "endpoint", Type: capmodel.ConfigurationFieldSingleLine,
				Label: &capmodel.LocalizedText{EnUS: "Endpoint", ZhCN: "接口地址"},
			}}},
			ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
				Key: "enabled", Type: capmodel.ConfigurationFieldBoolean,
				Label: &capmodel.LocalizedText{EnUS: "Enabled", ZhCN: "启用"},
			}}},
			CustomerConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{
				Key: "color", Type: capmodel.ConfigurationFieldSingleSelect,
				Label:            &capmodel.LocalizedText{EnUS: "Color", ZhCN: "颜色"},
				StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "black", Label: &capmodel.LocalizedText{EnUS: "Black", ZhCN: "黑色"}}},
			}}},
		}},
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "phone-orders", Version: "v1", Title: "Phone orders", Permission: "phone-supplier:orders:view", Slot: capmodel.FrontendSlotAdminOrdersFinance,
			Route: "phone-orders", Entry: "frontend/admin/index.html",
		}},
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "my-phone-orders", Version: "v1", Title: "My phone orders", Slot: capmodel.FrontendSlotCustomerProducts,
			Route: "phone-orders", Entry: "frontend/customer/index.html",
		}},
	}
	storeService := testutil.NewServices().Store
	content, err := storeService.BuildManifestSnapshot(manifest)
	if err != nil {
		t.Fatalf("build manifest snapshot: %v", err)
	}
	snapshot, err := storeService.ParseManifestSnapshot(content)
	if err != nil {
		t.Fatalf("parse manifest snapshot: %v", err)
	}
	if len(snapshot.BusinessExtensions) != 1 || len(snapshot.AuthorizedBusinessExtensions) != 0 {
		t.Fatalf("discovery snapshot must freeze requests without pretending they are authorized: %#v", snapshot)
	}
	declaration := snapshot.BusinessExtensions[0]
	if declaration.InterfaceConfiguration == nil || declaration.ProductConfiguration == nil || declaration.CustomerConfiguration == nil ||
		declaration.InterfaceConfiguration.Fields[0].Key != "endpoint" || declaration.ProductConfiguration.Fields[0].Key != "enabled" ||
		declaration.CustomerConfiguration.Version != "v1" || declaration.CustomerConfiguration.Fields[0].Key != "color" {
		t.Fatalf("all configuration declarations must be frozen in the requested snapshot: %#v", declaration)
	}
	if len(snapshot.AdminSurfaces) != 1 || len(snapshot.CustomerSurfaces) != 1 {
		t.Fatalf("admin/customer surfaces were not frozen separately: %#v", snapshot)
	}

	tampered := content + `authorizedBusinessExtensions:
    - point: product-fulfillment
      version: v1
      role: provider
      owner: commerce-core
      methods:
        - fulfill
      customer_configuration:
        version: v1
        fields:
          - key: capacity
            type: single_select
            label: {en-US: Capacity, zh-CN: 容量}
            static_candidates:
              - value: large
                label: {en-US: Large, zh-CN: 大}
`
	if _, err = storeService.ParseManifestSnapshot(tampered); err == nil {
		t.Fatal("expected authorized nested configuration differing from the request to be rejected")
	}
}
