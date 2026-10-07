// This file verifies AI plugin registration and its embedded manifest identity.

package backend

import (
	"io/fs"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

// TestSourcePluginRegistrationAndManifest verifies the phase-one delivery contract.
func TestSourcePluginRegistrationAndManifest(t *testing.T) {
	if idccap.ContractVersion == "" || resourcecap.ContractVersion == "" {
		t.Fatal("compiled read-only owner contract version is empty")
	}
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded manifest: %v", err)
	}
	for _, fragment := range []string{
		"id: idc-ai",
		"version: v0.1.0",
		"distribution: builtin",
		"id: idc-core",
		"id: idc-resource",
		"version: \">=0.1.0 <1.0.0\"",
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("embedded manifest is missing %q", fragment)
		}
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
