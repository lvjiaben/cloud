package integration

import (
	"testing"

	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/internal/service/plugin/internal/plugintypes"
)

func TestOrderSourceManifestsDependencyFirst(t *testing.T) {
	manifests := []*catalog.Manifest{
		{ID: "idc-compliance", Type: "source", Dependencies: &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{ID: "idc-notify"}, {ID: "idc-core"}}}},
		{ID: "idc-notify", Type: "source", Dependencies: &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{ID: "idc-core"}}}},
		{ID: "idc-core", Type: "source"},
		{ID: "managed-dynamic", Type: "dynamic"},
	}

	ordered, err := orderSourceManifestsDependencyFirst(manifests)
	if err != nil {
		t.Fatal(err)
	}
	if len(ordered) != 3 || ordered[0].ID != "idc-core" || ordered[1].ID != "idc-notify" || ordered[2].ID != "idc-compliance" {
		t.Fatalf("unexpected source assembly order: %+v", manifestIDs(ordered))
	}
}

func TestOrderSourceManifestsDependencyFirstRejectsCycles(t *testing.T) {
	_, err := orderSourceManifestsDependencyFirst([]*catalog.Manifest{
		{ID: "first", Type: "source", Dependencies: &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{ID: "second"}}}},
		{ID: "second", Type: "source", Dependencies: &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{ID: "first"}}}},
	})
	if err == nil {
		t.Fatal("source plugin dependency cycle was accepted")
	}
}

func manifestIDs(manifests []*catalog.Manifest) []string {
	ids := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		ids = append(ids, manifest.ID)
	}
	return ids
}
