// This file verifies source-plugin registration and embedded manifest ownership.

package backend

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/encoding/gyaml"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// TestPluginRegistrationIncludesEmbeddedManifest verifies resource plugin delivery identity.
func TestPluginRegistrationIncludesEmbeddedManifest(t *testing.T) {
	if idccap.ContractVersion == "" {
		t.Fatal("compiled idccap contract version is empty")
	}
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("expected %s to be registered", pluginID)
	}
	if registered.GetEmbeddedFiles() == nil {
		t.Fatal("expected registered plugin to expose embedded files")
	}
	jobs := registered.GetJobRegistrars()
	if len(jobs) != 1 || jobs[0] == nil || jobs[0].Handler == nil ||
		jobs[0].Point != pluginhost.ExtensionPointJobsRegister ||
		jobs[0].Mode != pluginhost.CallbackExecutionModeBlocking {
		t.Fatalf("expected one blocking resource job registration, got %+v", jobs)
	}

	manifest, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded manifest: %v", err)
	}
	var embedded, current struct {
		ID      string `yaml:"id"`
		Version string `yaml:"version"`
	}
	if err = gyaml.DecodeTo(manifest, &embedded); err != nil {
		t.Fatal(err)
	}
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve plugin manifest path")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(sourceFile), "..", "plugin.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err = gyaml.DecodeTo(source, &current); err != nil {
		t.Fatal(err)
	}
	validVersion := regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-[0-9A-Za-z.-]+)?$`).MatchString(embedded.Version)
	if embedded.ID != pluginID || embedded.ID != registered.ID() || embedded != current || !validVersion {
		t.Fatalf("embedded manifest identity/version mismatch: embedded=%+v current=%+v registered=%s", embedded, current, registered.ID())
	}
}

// TestPublishedProviderMethods guards the manifest/code handshake: the host
// rejects dynamic upgrades when the owner omits even an implemented method.
func TestPublishedProviderMethods(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatal("resource source plugin was not registered")
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Extensions []struct {
			Point   string   `yaml:"point"`
			Version string   `yaml:"version"`
			Role    string   `yaml:"role"`
			Owner   string   `yaml:"owner"`
			Methods []string `yaml:"methods"`
		} `yaml:"business_extensions"`
	}
	if err := gyaml.DecodeTo(content, &manifest); err != nil {
		t.Fatal(err)
	}
	want := append(resourcecap.DynamicProviderMethods(), resourcecap.DynamicProviderMethodListCatalog, resourcecap.DynamicProviderMethodReadCatalogProduct, resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration, resourcecap.DynamicProviderMethodCatalogBillingTerms)
	slices.Sort(want)
	for _, extension := range manifest.Extensions {
		if extension.Point == "resource-provider" && extension.Role == "owner" && extension.Owner == pluginID && extension.Version == "v1" {
			slices.Sort(extension.Methods)
			if !slices.Equal(extension.Methods, want) {
				t.Fatalf("published methods do not match the executable contract: got %v, want %v", extension.Methods, want)
			}
			return
		}
	}
	t.Fatal("resource-provider owner declaration is missing")
}

// TestEmbeddedCapabilityLifecycleContract verifies the embedded lifecycle declaration.
func TestEmbeddedCapabilityLifecycleContract(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "manifest/config/capabilities.yaml")
	if err != nil {
		t.Fatalf("read embedded capability lifecycle: %v", err)
	}
	for _, fragment := range []string{
		"stop_new",
		"drain_active",
		"preserve_history",
		"resume_after_validation",
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("embedded capability lifecycle is missing %q", fragment)
		}
	}
}
