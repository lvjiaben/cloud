// This file tests plugin-list controller helpers that project startup
// auto-enable configuration into management-list view fields.

package plugin

import (
	"testing"

	v1 "lina-core/api/plugin/v1"
	pluginsvc "lina-core/internal/service/plugin"
)

// TestBuildAutoEnableManagedSetNormalizesPluginIDs verifies startup-managed
// plugin IDs are trimmed and de-duplicated before list projection uses them.
func TestBuildAutoEnableManagedSetNormalizesPluginIDs(t *testing.T) {
	managedSet := buildAutoEnableManagedSet([]string{
		" demo-source ",
		"demo-source",
		"",
		"monitor-server",
	})

	if len(managedSet) != 2 {
		t.Fatalf("expected 2 unique managed plugin IDs, got %d", len(managedSet))
	}
	if !managedSet["demo-source"] {
		t.Fatal("expected demo-source to be marked as managed")
	}
	if !managedSet["monitor-server"] {
		t.Fatal("expected monitor-server to be marked as managed")
	}
	if managedSet[""] {
		t.Fatal("expected blank plugin ID to be ignored")
	}
}

// TestBuildPluginListItemResponseIncludesManagedDistribution verifies API
// projection exposes governance metadata for a managed list row.
func TestBuildPluginListItemResponseIncludesManagedDistribution(t *testing.T) {
	controller := &ControllerV1{}
	pluginItem := &pluginsvc.PluginItem{}
	pluginItem.Id = "plugin-dev-controller-managed"
	pluginItem.Name = "Controller Managed"
	pluginItem.Type = string(v1.PluginTypeSource)
	pluginItem.Distribution = string(v1.PluginDistributionManaged)

	item := controller.buildPluginListItemResponse(pluginItem, false)
	if item == nil {
		t.Fatal("expected plugin list item response")
	}
	if item.Distribution != v1.PluginDistributionManaged {
		t.Fatalf("expected managed distribution in API projection, got %q", item.Distribution)
	}
}
