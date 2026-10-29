package backend

import (
	"github.com/gogf/gf/v2/encoding/gyaml"
	"io/fs"
	"lina-core/pkg/plugin/pluginhost"
	"testing"
)

// TestResourceManagementGrantsRetainOnlyContextualRoutes verifies the real
// embedded declaration: selected read grants retain the neutral workspace
// ancestor without granting configuration access or retired global routes.
func TestResourceManagementGrantsRetainOnlyContextualRoutes(t *testing.T) {
	registered, ok := pluginhost.GetSourcePlugin(pluginID)
	if !ok {
		t.Fatal("source plugin missing")
	}
	content, err := fs.ReadFile(registered.GetEmbeddedFiles(), "plugin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	type menu struct {
		Key        string `yaml:"key"`
		Parent     string `yaml:"parent_key"`
		Type       string `yaml:"type"`
		Path       string `yaml:"path"`
		Component  string `yaml:"component"`
		Query      string `yaml:"query_param"`
		Permission string `yaml:"perms"`
	}
	var manifest struct {
		Menus []menu `yaml:"menus"`
	}
	if err := gyaml.DecodeTo(content, &manifest); err != nil {
		t.Fatal(err)
	}
	nodes := map[string]menu{}
	for _, item := range manifest.Menus {
		nodes[item.Key] = item
	}
	root := nodes["plugin:idc-resource:provider-workspace"]
	if root.Type != "M" || root.Path != "/idc/resource/provider-configuration" || root.Permission != "" {
		t.Fatalf("workspace must not confer a business permission: %+v", root)
	}
	for _, key := range []string{"server-management", "provider-operations", "overview", "provider-capacity", "health", "tasks", "api-logs", "server-accessories", "server-renewals"} {
		node := nodes["plugin:idc-resource:"+key]
		if node.Type != "B" || node.Path != "" || node.Component != "" || node.Query != "" {
			t.Fatalf("retired independent route remains: %+v", node)
		}
	}
	for _, key := range []string{"provider-configuration", "overview", "provider-capacity", "health", "tasks", "api-logs"} {
		item := nodes["plugin:idc-resource:"+key]
		if item.Permission == "" {
			t.Fatalf("missing read permission: %s", key)
		}
		for depth := 0; item.Key != root.Key; depth++ {
			if depth > len(nodes) {
				t.Fatalf("missing workspace ancestor for %s", key)
			}
			parent, exists := nodes[item.Parent]
			if !exists {
				t.Fatalf("missing parent for %s: %+v", key, item)
			}
			item = parent
		}
	}
}
