// This file validates the managed connector manifest and governed frontend sources.
package backend

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	"lina-plugin-idc-zjmfcloud/backend/internal/provider/zjmfcloud"
)

type managedManifest struct {
	ID                 string                                   `yaml:"id"`
	Version            string                                   `yaml:"version"`
	Type               string                                   `yaml:"type"`
	Distribution       string                                   `yaml:"distribution"`
	Description        string                                   `yaml:"description"`
	BusinessExtensions []*capmodel.BusinessExtensionDeclaration `yaml:"business_extensions"`
	AdminSurfaces      []*capmodel.FrontendSurfaceDeclaration   `yaml:"admin_surfaces"`
	CustomerSurfaces   []*capmodel.FrontendSurfaceDeclaration   `yaml:"customer_surfaces"`
	HostServices       []struct {
		Owner   string   `yaml:"owner"`
		Service string   `yaml:"service"`
		Version string   `yaml:"version"`
		Methods []string `yaml:"methods"`
	} `yaml:"hostServices"`
}

func TestManagedManifestSatisfiesProductAndServerOwnerContracts(t *testing.T) {
	t.Parallel()
	manifest := readManagedManifest(t)
	if manifest.ID != pluginID || manifest.Version != "v0.2.9" || manifest.Type != "dynamic" || manifest.Distribution != "managed" {
		t.Fatalf("connector is not a managed dynamic release: %#v", manifest)
	}
	if !strings.Contains(manifest.Description, "product configuration discovery") ||
		!strings.Contains(manifest.Description, "idc-resource-owned server execution") {
		t.Fatalf("connector description does not preserve the product/resource owner boundary: %q", manifest.Description)
	}
	fulfillmentDeclarations := declarationsForPoint(manifest.BusinessExtensions, capmodel.BusinessExtensionProductFulfillment)
	if err := productfulfillmentcap.ValidateProviderDeclarations(fulfillmentDeclarations); err != nil {
		t.Fatalf("product owner rejected connector declaration: %v", err)
	}
	if len(fulfillmentDeclarations) != 1 || fulfillmentDeclarations[0].InterfaceConfiguration != nil {
		t.Fatalf("product owner retained resource-owned interface credentials: %#v", fulfillmentDeclarations)
	}
	resourceDeclarations := declarationsForPoint(manifest.BusinessExtensions, resourcecap.DynamicProviderPoint)
	if err := resourcecap.ValidateDynamicProviderDeclarations(resourceDeclarations); err != nil {
		t.Fatalf("resource owner rejected dynamic Provider declaration: %v", err)
	}
	now := time.Date(2026, 8, 27, 12, 0, 0, 0, time.UTC)
	contract := resourcecap.ServerFulfillmentContract{
		Version:          resourcecap.ServerFulfillmentContractVersion,
		Fulfillment:      fulfillmentDeclarations[0],
		Capabilities:     zjmfcloud.DeclaredCapabilitySnapshot(now),
		AdminSurfaces:    manifest.AdminSurfaces,
		CustomerSurfaces: manifest.CustomerSurfaces,
	}
	if err := contract.ValidateAt(now); err != nil {
		t.Fatalf("resource owner rejected server connector release: %v", err)
	}
	if len(manifest.HostServices) != 2 || manifest.HostServices[0].Service != "confidentialoutbound" ||
		!slices.Equal(manifest.HostServices[0].Methods, []string{"execute"}) ||
		manifest.HostServices[1].Owner != resourcecap.OwnerPluginID ||
		manifest.HostServices[1].Service != resourcecap.ServiceName ||
		manifest.HostServices[1].Version != string(resourcecap.ReadVersion) ||
		!slices.Equal(manifest.HostServices[1].Methods, []string{"current-customer-servers.list", "current-customer-server.get"}) {
		t.Fatalf("connector requested an unexpected host service contract: %#v", manifest.HostServices)
	}
	if len(manifest.CustomerSurfaces) != 1 ||
		!slices.Equal(manifest.CustomerSurfaces[0].BridgeCapabilities, []string{"customer-servers.read"}) {
		t.Fatalf("customer server page is missing its exact bridge capability: %#v", manifest.CustomerSurfaces)
	}
}

func declarationsForPoint(
	declarations []*capmodel.BusinessExtensionDeclaration,
	point string,
) []*capmodel.BusinessExtensionDeclaration {
	items := make([]*capmodel.BusinessExtensionDeclaration, 0, 1)
	for _, declaration := range declarations {
		if declaration != nil && declaration.Point == point {
			items = append(items, declaration)
		}
	}
	return items
}

func TestManagedReleaseHasNoSourceRegistrationOrUnsafeFrontendAccess(t *testing.T) {
	t.Parallel()
	pluginRoot := filepath.Clean("..")
	for _, removed := range []string{"plugin_embed.go"} {
		if _, err := os.Stat(filepath.Join(pluginRoot, removed)); !os.IsNotExist(err) {
			t.Fatalf("source-plugin compatibility file still exists: %s", removed)
		}
	}
	for _, entry := range []string{
		filepath.Join("frontend", "admin", "server-management", "index.html"),
		filepath.Join("frontend", "customer", "servers", "index.html"),
		filepath.Join("frontend", "src", "customer", "servers", "entry.ts"),
		filepath.Join("frontend", "src", "customer", "servers", "model.ts"),
		filepath.Join("frontend", "src", "customer", "servers", "page.ts"),
		filepath.Join("frontend", "src", "admin", "entry.ts"),
		filepath.Join("frontend", "src", "admin", "page.ts"),
		filepath.Join("frontend", "src", "shared", "bridge.ts"),
	} {
		content, err := os.ReadFile(filepath.Join(pluginRoot, entry))
		if err != nil {
			t.Fatalf("read governed frontend entry %s: %v", entry, err)
		}
		lower := strings.ToLower(string(content))
		for _, forbidden := range []string{"localstorage", "sessionstorage", "document.cookie", "authorization", "access-token"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("frontend entry %s attempts unsafe host or token access: %s", entry, forbidden)
			}
		}
	}
}

func readManagedManifest(t *testing.T) *managedManifest {
	t.Helper()
	content, err := os.ReadFile(filepath.Join("..", "plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest managedManifest
	if err = yaml.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	return &manifest
}
