package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"lina-core/pkg/plugin/pluginbridge"
)

func TestDynamicPluginDeclarationAndManifest(t *testing.T) {
	if err := RegisterPlugin(pluginbridge.NewDeclarations()); err != nil {
		t.Fatalf("register dynamic routes: %v", err)
	}
	content, err := os.ReadFile(filepath.Join("..", "plugin.yaml"))
	if err != nil {
		t.Fatalf("read dynamic manifest: %v", err)
	}
	for _, fragment := range []string{
		"id: idc-zjmfmigrate",
		"version: v0.2.1",
		"type: dynamic",
		"distribution: managed",
		"service: data",
		"plugin_idc_zjmfmigrate_batch",
		"entry: admin/migration/index.html",
		"id: vben-adapter",
	} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("dynamic manifest is missing %q", fragment)
		}
	}
}

func TestManagedMigrationLifecycleContractRemainsSideEffectFree(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "manifest", "config", "capabilities.yaml"))
	if err != nil {
		t.Fatalf("read migration lifecycle: %v", err)
	}
	for _, fragment := range []string{"manual_enable: true", "migration_mode: true", "no_external_side_effects: true"} {
		if !strings.Contains(string(content), fragment) {
			t.Fatalf("migration lifecycle is missing %q", fragment)
		}
	}
}
