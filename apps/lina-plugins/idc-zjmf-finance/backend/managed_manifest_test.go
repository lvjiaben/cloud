package backend

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-zjmf-finance/backend/internal/provider/finance"
)

type managedManifest struct {
	ID                 string                                   `yaml:"id"`
	Type               string                                   `yaml:"type"`
	Distribution       string                                   `yaml:"distribution"`
	BusinessExtensions []*capmodel.BusinessExtensionDeclaration `yaml:"business_extensions"`
	AdminSurfaces      []*capmodel.FrontendSurfaceDeclaration   `yaml:"admin_surfaces"`
	CustomerSurfaces   []*capmodel.FrontendSurfaceDeclaration   `yaml:"customer_surfaces"`
}

func TestManagedManifestSatisfiesOwnerContracts(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest managedManifest
	if yaml.Unmarshal(content, &manifest) != nil || manifest.ID != pluginID || manifest.Type != "dynamic" || manifest.Distribution != "managed" {
		t.Fatalf("invalid managed manifest: %#v", manifest)
	}
	fulfillment := declarationsAt(manifest.BusinessExtensions, capmodel.BusinessExtensionProductFulfillment)
	if err = productfulfillmentcap.ValidateProviderDeclarations(fulfillment); err != nil {
		t.Fatalf("product owner rejected declaration: %v", err)
	}
	providers := declarationsAt(manifest.BusinessExtensions, resourcecap.DynamicProviderPoint)
	if err = resourcecap.ValidateDynamicProviderDeclarations(providers); err != nil {
		t.Fatalf("resource owner rejected declaration: %v", err)
	}
	now := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	contract := resourcecap.ServerFulfillmentContract{Version: resourcecap.ServerFulfillmentContractVersion, Fulfillment: fulfillment[0], Capabilities: finance.DeclaredCapabilitySnapshot(now), AdminSurfaces: manifest.AdminSurfaces, CustomerSurfaces: manifest.CustomerSurfaces}
	if err = contract.ValidateAt(now); err != nil {
		t.Fatalf("server owner rejected release: %v", err)
	}
	if fulfillment[0].InterfaceConfiguration != nil || providers[0].InterfaceConfiguration == nil ||
		providers[0].InterfaceConfigurationCardinality != capmodel.InterfaceConfigurationCardinalityMultiple {
		t.Fatal("resource-owned multi-instance interface declaration is invalid")
	}
	if len(manifest.AdminSurfaces) != 0 || len(manifest.CustomerSurfaces) != 0 {
		t.Fatal("connector must reuse idc-resource server pages unless it ships real connector-owned frontend assets")
	}
}

func declarationsAt(items []*capmodel.BusinessExtensionDeclaration, point string) []*capmodel.BusinessExtensionDeclaration {
	result := make([]*capmodel.BusinessExtensionDeclaration, 0, 1)
	for _, item := range items {
		if item != nil && item.Point == point {
			result = append(result, item)
		}
	}
	return result
}
