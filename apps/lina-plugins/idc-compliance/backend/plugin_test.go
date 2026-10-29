// This file verifies compliance plugin registration and its embedded manifest identity.

package backend

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// TestSourcePluginRegistrationAndManifest verifies the current delivery contract.
func TestSourcePluginRegistrationAndManifest(t *testing.T) {
	if idccap.ContractVersion == "" {
		t.Fatal("compiled idccap contract version is empty")
	}
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	if registrars := registered.GetRouteRegistrars(); len(registrars) != 1 {
		t.Fatalf("route registrar count=%d want=1", len(registrars))
	}
	jobs := registered.GetJobRegistrars()
	if len(jobs) != 1 || jobs[0] == nil || jobs[0].Handler == nil ||
		jobs[0].Point != pluginhost.ExtensionPointJobsRegister ||
		jobs[0].Mode != pluginhost.CallbackExecutionModeBlocking {
		t.Fatalf("expected one blocking personal verification recovery job, got %+v", jobs)
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatalf("read embedded manifest: %v", err)
	}
	for _, fragment := range []string{
		"id: idc-compliance",
		"version: v0.1.23",
		"distribution: builtin",
		"id: idc-core",
		"version: \">=0.1.0 <1.0.0\"",
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("embedded manifest is missing %q", fragment)
		}
	}
}

// TestProductDeliveryDeclaresLocalGateway proves the source plugin carries the
// reviewed component recipe while generated platform binaries remain external.
func TestProductDeliveryDeclaresLocalGateway(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatalf("source plugin %q is not registered", pluginID)
	}
	embedded := registered.GetEmbeddedFiles()
	delivery, err := fs.ReadFile(embedded, "manifest/delivery/local-compliance-gateway.yaml")
	if err != nil {
		t.Fatalf("read local gateway delivery declaration: %v", err)
	}
	for _, fragment := range []string{
		"component_id: idc-compliance-local-gateway",
		"product_package_required: true",
		"local_installation_only: true",
		"same_program_for_all_installations: true",
		"supplier_clients_default: disabled",
	} {
		if !strings.Contains(string(delivery), fragment) {
			t.Fatalf("local gateway delivery declaration is missing %q", fragment)
		}
	}
	if _, err := fs.Stat(embedded, "central-gateway"); err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("source plugin unexpectedly embeds gateway source tree: %v", err)
	}
	if err := fs.WalkDir(embedded, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		lowerPath := strings.ToLower(path)
		if strings.HasSuffix(lowerPath, ".exe") || strings.HasSuffix(lowerPath, ".dll") ||
			strings.HasSuffix(lowerPath, ".so") || strings.HasPrefix(lowerPath, "central-gateway/") ||
			strings.HasPrefix(lowerPath, "local-gateway/") {
			return errors.New("generated gateway artifact found in source plugin embedded files")
		}
		return nil
	}); err != nil {
		t.Fatalf("inspect merchant embedded files: %v", err)
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
