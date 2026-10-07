// This file verifies website-delivery source-plugin registration and lifecycle metadata.
package backend

import (
	"io/fs"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	idcportal "lina-plugin-idc-portal"
)

// TestSourcePluginRegistration verifies host registration and the compiled core contract.
func TestSourcePluginRegistration(t *testing.T) {
	if idccap.ContractVersion == "" {
		t.Fatal("compiled idccap contract version is empty")
	}
	definition, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %s is not registered", pluginID)
	}
	if definition.GetEmbeddedFiles() == nil {
		t.Fatal("registered source plugin has no embedded files")
	}
	if len(definition.GetRouteRegistrars()) != 1 {
		t.Fatalf("expected one password-reset route registrar, got %d", len(definition.GetRouteRegistrars()))
	}
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

// TestEmbeddedDeliveryContract verifies the manifest remains embedded without retired frontend assets.
func TestEmbeddedDeliveryContract(t *testing.T) {
	manifest, err := fs.ReadFile(idcportal.EmbeddedFiles, "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded plugin manifest: %v", err)
	}
	for _, expected := range []string{
		"id: " + pluginID,
		"distribution: builtin",
		"id: idc-core",
		"point: website-template",
	} {
		if !strings.Contains(string(manifest), expected) {
			t.Errorf("embedded manifest does not contain %q", expected)
		}
	}
	entries, err := fs.ReadDir(idcportal.EmbeddedFiles, ".")
	if err != nil {
		t.Fatalf("read embedded root: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() == "portal" {
			t.Fatal("retired Portal frontend is still embedded")
		}
	}
	if _, err := fs.ReadFile(idcportal.EmbeddedFiles, "frontend/dist/mount.js"); err != nil {
		t.Fatalf("governed brand frontend is not embedded: %v", err)
	}
}
