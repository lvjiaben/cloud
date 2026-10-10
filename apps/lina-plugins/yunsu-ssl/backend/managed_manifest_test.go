package backend

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

type managedManifest struct {
	ID                 string                                   `yaml:"id"`
	Version            string                                   `yaml:"version"`
	Type               string                                   `yaml:"type"`
	Distribution       string                                   `yaml:"distribution"`
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

func TestManagedManifestDeclaresValidatedOngoingCertificateFulfillment(t *testing.T) {
	manifest := readManagedManifest(t)
	if manifest.ID != pluginID || manifest.Version != "v0.3.6" || manifest.Type != "dynamic" || manifest.Distribution != "managed" {
		t.Fatalf("connector release identity is invalid: %#v", manifest)
	}
	if len(manifest.BusinessExtensions) != 1 {
		t.Fatalf("unexpected business extensions: %#v", manifest.BusinessExtensions)
	}
	declaration := manifest.BusinessExtensions[0]
	if !slices.Equal(declaration.Methods, []string{
		candidateMethod, validationMethod, provisionMethod, queryMethod,
		serviceDetailMethod, serviceSyncMethod, serviceUpdateValidationMethod, serviceRevalidateMethod, serviceCancelMethod,
		serviceDownloadMethod,
	}) {
		t.Fatalf("unexpected fulfillment methods: %#v", declaration.Methods)
	}
	if err := productfulfillmentcap.ValidateProviderDeclarations(manifest.BusinessExtensions); err != nil {
		t.Fatalf("product owner rejected SSL declaration: %v", err)
	}
	parsed, err := productfulfillmentcap.ParseProviderDeclaration(declaration.ProviderMetadata, declaration.Methods)
	if err != nil || !parsed.OngoingManagement || parsed.EntitlementOwnerID != ownerPluginID || parsed.ExpiryManagement != productfulfillmentcap.ExpiryManagementProvider {
		t.Fatalf("SSL long-term ownership declaration is invalid: parsed=%#v err=%v", parsed, err)
	}
	if len(parsed.ServiceActions) != 5 || parsed.ServiceActions[2].Execution != productfulfillmentcap.ServiceActionExecutionAsynchronous ||
		parsed.ServiceActions[2].QueryMethod != serviceSyncMethod || parsed.ServiceActions[4].Availability != productfulfillmentcap.ServiceActionAvailabilityBeforeFinal {
		t.Fatalf("SSL managed service actions are invalid: %#v", parsed.ServiceActions)
	}
	if len(parsed.ServiceDownloads) != 1 || parsed.ServiceDownloads[0].Method != serviceDownloadMethod || parsed.ServiceDownloads[0].Path != "/order/download-cert" {
		t.Fatalf("SSL managed service download is invalid: %#v", parsed.ServiceDownloads)
	}
	if len(manifest.AdminSurfaces) != 1 || len(manifest.CustomerSurfaces) != 1 ||
		!slices.Equal(manifest.AdminSurfaces[0].BridgeCapabilities, []string{"service-entitlements.admin.v1"}) ||
		!slices.Equal(manifest.CustomerSurfaces[0].BridgeCapabilities, []string{"service-entitlements.customer.v1"}) {
		t.Fatal("certificate surfaces must use only the governed owner capabilities")
	}
	if len(manifest.HostServices) != 1 || manifest.HostServices[0].Service != "supplier-interface-runtime" ||
		!slices.Equal(manifest.HostServices[0].Methods, []string{"outbound.http.execute"}) {
		t.Fatalf("unexpected host access: %#v", manifest.HostServices)
	}
}

func TestWASMSourceContainsNoCertificateZIPOrPrivateKeyHandling(t *testing.T) {
	entries := []string{"business_extension.go", "configuration_validation.go", "service_actions.go"}
	for _, entry := range entries {
		content, readErr := os.ReadFile(entry)
		if readErr != nil {
			t.Fatal(readErr)
		}
		lower := strings.ToLower(string(content))
		for _, forbidden := range []string{"download-cert", "private key", "private_key", "archive/zip"} {
			if strings.Contains(lower, forbidden) {
				t.Fatalf("unsafe certificate material handling found in %s: %s", entry, forbidden)
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
