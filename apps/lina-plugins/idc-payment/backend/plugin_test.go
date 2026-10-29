// This file verifies payment plugin registration and its embedded manifest identity.

package backend

import (
	"io/fs"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// TestSourcePluginRegistrationAndManifest verifies the phase-one delivery contract.
func TestSourcePluginRegistrationAndManifest(t *testing.T) {
	if idccap.ContractVersion == "" {
		t.Fatal("compiled idccap contract version is empty")
	}
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	if routes := registered.GetRouteRegistrars(); len(routes) != 1 || routes[0] == nil || routes[0].Handler == nil ||
		routes[0].Point != pluginhost.ExtensionPointHTTPRouteRegister || routes[0].Mode != pluginhost.CallbackExecutionModeBlocking {
		t.Fatalf("expected one blocking payment route registration, got %+v", routes)
	}
	if jobs := registered.GetJobRegistrars(); len(jobs) != 1 || jobs[0] == nil || jobs[0].Handler == nil ||
		jobs[0].Point != pluginhost.ExtensionPointJobsRegister || jobs[0].Mode != pluginhost.CallbackExecutionModeBlocking {
		t.Fatalf("expected one blocking payment job registration, got %+v", jobs)
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded manifest: %v", err)
	}
	for _, fragment := range []string{
		"id: idc-payment",
		"version: v0.1.14",
		"distribution: builtin",
		"id: idc-core",
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
