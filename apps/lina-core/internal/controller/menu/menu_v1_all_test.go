package menu

import (
	"testing"

	"lina-core/internal/model/entity"
	menusvc "lina-core/internal/service/menu"
	"lina-core/pkg/menutype"
)

// TestConvertToRouteItemsBuildsIframeRouteForHostedXAssets verifies hosted
// asset menus default to iframe routes when they are not marked as new-window
// or embedded-mount entries.
func TestConvertToRouteItemsBuildsIframeRouteForHostedXAssets(t *testing.T) {
	routes := convertToRouteItems([]*menusvc.MenuItem{
		{
			Id:      101,
			Name:    "Runtime Iframe Entry",
			Path:    "/x-assets/plugin-runtime-demo/v0.1.0/index.html",
			Type:    menutype.Menu.String(),
			IsFrame: 0,
			Visible: 1,
			Status:  1,
		},
	})

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	route := routes[0]
	if route.Component != "PluginSandboxView" {
		t.Fatalf("expected isolated plugin route component, got %s", route.Component)
	}
	if route.Meta == nil || route.Meta.PluginSandboxSrc != "/x-assets/plugin-runtime-demo/v0.1.0/index.html" || route.Meta.IframeSrc != "" {
		t.Fatalf("expected iframe src to be preserved, got %#v", route.Meta)
	}
	if route.Path == "/x-assets/plugin-runtime-demo/v0.1.0/index.html" {
		t.Fatalf("expected virtual router path instead of raw asset path, got %s", route.Path)
	}
}

func TestConvertToRouteItemsKeepsGovernedPluginPathForIsolatedSurface(t *testing.T) {
	routes := convertToRouteItems([]*menusvc.MenuItem{
		{
			Id: 105, Name: "SSL certificates", Path: "/plugins/ssl-provider/certificates",
			Type: menutype.Menu.String(), Visible: 1, Status: 1,
			QueryParam: `{"pluginSurfaceSrc":"/x-assets/ssl-provider/v1.0.0/admin/index.html","pluginSurfacePlugin":"ssl-provider","pluginSurfaceKey":"certificates","pluginSurfaceVersion":"v1","pluginSurfaceCapabilities":"certificates.query,certificates.download","pluginSurfaceHost":"admin"}`,
		},
	})
	if len(routes) != 1 {
		t.Fatalf("expected one governed surface route, got %d", len(routes))
	}
	route := routes[0]
	if route.Path != "/plugins/ssl-provider/certificates" {
		t.Fatalf("expected the host-governed route path to remain stable, got %s", route.Path)
	}
	if route.Component != "PluginSandboxView" || route.Meta == nil || route.Meta.PluginSandboxSrc != "/x-assets/ssl-provider/v1.0.0/admin/index.html" || route.Meta.IframeSrc != "" {
		t.Fatalf("expected an isolated hosted surface, got %#v", route)
	}
	if route.Meta.PluginSurfaceId != "ssl-provider" || route.Meta.PluginSurfaceKey != "certificates" || route.Meta.PluginSurfaceVer != "v1" || route.Meta.PluginSurfaceHost != "admin" || len(route.Meta.PluginSurfaceCapabilities) != 2 {
		t.Fatalf("expected exact page bridge authorization metadata, got %#v", route.Meta)
	}
}

// TestConvertToRouteItemsKeepsHostedXAssetsIsolatedWhenMarkedAsFrame verifies
// legacy frame flags cannot bypass the hosted plugin sandbox.
func TestConvertToRouteItemsKeepsHostedXAssetsIsolatedWhenMarkedAsFrame(t *testing.T) {
	routes := convertToRouteItems([]*menusvc.MenuItem{
		{
			Id:      102,
			Name:    "Runtime New Window Entry",
			Path:    "/x-assets/plugin-runtime-demo/v0.1.0/index.html",
			Type:    menutype.Menu.String(),
			IsFrame: 1,
			Visible: 1,
			Status:  1,
		},
	})

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	route := routes[0]
	if route.Component != "PluginSandboxView" {
		t.Fatalf("expected hosted asset to remain isolated, got %s", route.Component)
	}
	if route.Meta == nil || route.Meta.PluginSandboxSrc != "/x-assets/plugin-runtime-demo/v0.1.0/index.html" || route.Meta.Link != "" {
		t.Fatalf("expected sandbox target without a direct link, got %#v", route.Meta)
	}
	if route.Meta.OpenInNewWindow {
		t.Fatalf("hosted plugin assets must not open outside the sandbox")
	}
}

func TestNormalizeMenuLinkTargetRejectsHostedAssetTraversal(t *testing.T) {
	for _, target := range []string{
		"/x-assets/plugin/v1/../secret",
		"/x-assets/plugin/v1/%2e%2e/secret",
		"/x-assets/plugin/v1/index.html?token=host",
	} {
		if normalized := normalizeMenuLinkTarget(target); normalized != "" {
			t.Fatalf("expected unsafe hosted target to be rejected, got %s", normalized)
		}
	}
}

// TestConvertToRouteItemsBuildsEmbeddedMountRouteForHostedXAssets verifies
// embedded-mount runtime menus are forced into the isolated page container.
func TestConvertToRouteItemsBuildsEmbeddedMountRouteForHostedXAssets(t *testing.T) {
	routes := convertToRouteItems([]*menusvc.MenuItem{
		{
			Id:         103,
			Name:       "Runtime Embedded Entry",
			Path:       "/x-assets/plugin-runtime-demo/v0.1.0/mount.js",
			Component:  "system/plugin/dynamic-page",
			Type:       menutype.Menu.String(),
			IsFrame:    0,
			Visible:    1,
			Status:     1,
			QueryParam: `{"pluginAccessMode":"embedded-mount"}`,
		},
	})

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	route := routes[0]
	if route.Component != "PluginSandboxView" {
		t.Fatalf("expected embedded route to use isolated plugin component, got %s", route.Component)
	}
	if route.Meta == nil || route.Meta.PluginSandboxSrc != "/x-assets/plugin-runtime-demo/v0.1.0/mount.js" || route.Meta.IframeSrc != "" {
		t.Fatalf("expected isolated asset url to be preserved, got %#v", route.Meta)
	}
	if route.Path == "/x-assets/plugin-runtime-demo/v0.1.0/mount.js" {
		t.Fatalf("expected virtual host route instead of raw asset path, got %s", route.Path)
	}
}

// TestConvertToRouteItemsKeepsRegularViewRouteUnchanged verifies normal
// workspace views are not rewritten by hosted-link conversion logic.
func TestConvertToRouteItemsKeepsRegularViewRouteUnchanged(t *testing.T) {
	routes := convertToRouteItems([]*menusvc.MenuItem{
		{
			Id:        104,
			Name:      "Plugin Demo Source",
			Path:      "demo-source-sidebar-entry",
			Component: "system/plugin/dynamic-page",
			Type:      menutype.Menu.String(),
			IsFrame:   0,
			Visible:   1,
			Status:    1,
		},
	})

	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}

	route := routes[0]
	if route.Component != "#/views/system/plugin/dynamic-page" {
		t.Fatalf("expected host view component, got %s", route.Component)
	}
	if route.Meta == nil || route.Meta.IframeSrc != "" || route.Meta.Link != "" {
		t.Fatalf("expected regular route meta to stay without link semantics, got %#v", route.Meta)
	}
	if route.Path != "/demo-source-sidebar-entry" {
		t.Fatalf("expected normal menu path, got %s", route.Path)
	}
}

// TestConvertToRouteItemsKeepsAbsoluteChildPath verifies grouped directory
// menus can keep child routes on their original absolute URLs.
func TestConvertToRouteItemsKeepsAbsoluteChildPath(t *testing.T) {
	routes := convertToRouteItems([]*menusvc.MenuItem{
		{
			Id:      201,
			Name:    "定时任务",
			Path:    "scheduled-job",
			Type:    menutype.Directory.String(),
			Visible: 1,
			Status:  1,
			Children: []*menusvc.MenuItem{
				{
					Id:        202,
					ParentId:  201,
					Name:      "任务管理",
					Path:      "/system/job",
					Component: "system/job/index",
					Type:      menutype.Menu.String(),
					Visible:   1,
					Status:    1,
				},
			},
		},
	})

	if len(routes) != 1 {
		t.Fatalf("expected 1 directory route, got %d", len(routes))
	}
	if len(routes[0].Children) != 1 {
		t.Fatalf("expected 1 child route, got %#v", routes[0].Children)
	}
	if routes[0].Children[0].Path != "/system/job" {
		t.Fatalf("expected absolute child path to be preserved, got %q", routes[0].Children[0].Path)
	}
}

// TestConvertToRouteItemsSkipsDirectoryWithoutVisibleChildren verifies host
// directory menus disappear once all child nodes are filtered out.
func TestConvertToRouteItemsSkipsDirectoryWithoutVisibleChildren(t *testing.T) {
	routes := convertToRouteItems([]*menusvc.MenuItem{
		{
			Id:      301,
			Name:    "系统监控",
			Path:    "monitor",
			Type:    menutype.Directory.String(),
			Visible: 1,
			Status:  1,
			Children: []*menusvc.MenuItem{
				{
					Id:       302,
					ParentId: 301,
					Name:     "操作日志查看",
					Path:     "monitor-operlog-view",
					Type:     menutype.Button.String(),
					Visible:  1,
					Status:   1,
				},
			},
		},
	})

	if len(routes) != 0 {
		t.Fatalf("expected empty directory route to be hidden, got %#v", routes)
	}
}

// TestBuildFilteredTreeKeepsAncestors verifies selected leaf menus project the
// full ancestor chain required by the stable host catalog tree.
func TestBuildFilteredTreeKeepsAncestors(t *testing.T) {
	menuTree := buildFilteredTree([]*entity.SysMenu{
		{Id: 1, Name: "权限管理", Path: "iam", Type: menutype.Directory.String(), Visible: 1, Status: 1},
		{Id: 2, ParentId: 1, Name: "用户治理", Path: "iam-user", Type: menutype.Directory.String(), Visible: 1, Status: 1},
		{Id: 3, ParentId: 2, Name: "用户管理", Path: "/system/user", Component: "system/user/index", Type: menutype.Menu.String(), Visible: 1, Status: 1},
	}, []int{3})

	if len(menuTree) != 1 {
		t.Fatalf("expected one root ancestor, got %#v", menuTree)
	}
	if len(menuTree[0].Children) != 1 {
		t.Fatalf("expected one middle ancestor, got %#v", menuTree[0].Children)
	}
	if len(menuTree[0].Children[0].Children) != 1 {
		t.Fatalf("expected selected leaf to remain attached, got %#v", menuTree[0].Children[0].Children)
	}
	if menuTree[0].Children[0].Children[0].Id != 3 {
		t.Fatalf("expected selected leaf id=3, got %#v", menuTree[0].Children[0].Children[0])
	}
}
