package integration

import (
	"testing"

	"lina-core/internal/service/plugin/internal/catalog"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/pluginhost"
)

func TestManifestWithGovernedAdminSurfaceMenusMountsUnderPublishedParent(t *testing.T) {
	manifest := &catalog.Manifest{
		ID: "ssl-provider", Version: "v1.0.0", Type: "dynamic",
		PublicAssets: []*catalog.PublicAssetSpec{{Source: "frontend", Mount: "admin"}},
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "certificates", Version: "v1", Title: "SSL certificates", Icon: "lucide:shield-check",
			Permission: "ssl-provider:certificates:view", Slot: capmodel.FrontendSlotAdminCatalogProducts,
			Route: "certificates", Entry: "admin/index.html",
		}},
	}
	projected, err := manifestWithGovernedAdminSurfaceMenus(manifest)
	if err != nil {
		t.Fatalf("expected admin surface menu projection to succeed, got error: %v", err)
	}
	if len(projected.Menus) != 1 {
		t.Fatalf("expected one projected admin surface menu, got %d", len(projected.Menus))
	}
	menu := projected.Menus[0]
	if menu.ParentKey != "plugin:idc-core:catalog-products" {
		t.Fatalf("expected governed catalog parent, got %s", menu.ParentKey)
	}
	if menu.Path != "/plugins/ssl-provider/certificates" {
		t.Fatalf("expected host-namespaced route, got %s", menu.Path)
	}
	if menu.Query[pluginhost.GovernedSurfaceSourceQueryKey] != "/x-assets/ssl-provider/v1.0.0/admin/index.html" {
		t.Fatalf("expected version-frozen isolated entry, got %#v", menu.Query)
	}
	if len(manifest.Menus) != 0 {
		t.Fatal("surface projection must not mutate the cached source manifest")
	}
}

func TestManifestWithGovernedAdminSurfaceMenusKeepsSourcePageInHostRoute(t *testing.T) {
	manifest := &catalog.Manifest{
		ID: "server-owner", Version: "v1.0.0", Type: "source",
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "servers", Version: "v1", Title: "Servers", Permission: "server-owner:servers:view",
			Slot: capmodel.FrontendSlotAdminResourceOperations, Route: "servers", Entry: "server-owner-servers",
		}},
	}
	projected, err := manifestWithGovernedAdminSurfaceMenus(manifest)
	if err != nil {
		t.Fatalf("expected source admin surface projection to succeed, got error: %v", err)
	}
	menu := projected.Menus[0]
	if menu.Component != pluginhost.DynamicPageComponentPath ||
		menu.Query[pluginhost.GovernedSurfacePluginQueryKey] != "server-owner" ||
		menu.Query[pluginhost.GovernedSurfaceEntryQueryKey] != "server-owner-servers" {
		t.Fatalf("expected a governed source page owner and entry query, got %#v", menu)
	}
}

func TestManifestWithGovernedAdminSurfaceMenusDropsLegacyDynamicRouteTree(t *testing.T) {
	manifest := &catalog.Manifest{
		ID: "content-owner", Version: "v2.0.0", Type: "dynamic",
		PublicAssets: []*catalog.PublicAssetSpec{{Source: "frontend/dist", Mount: "/"}},
		Menus: []*catalog.MenuSpec{
			{Key: "plugin:content-owner:legacy", ParentKey: "content", Path: "/plugins/content-owner/notices", Type: catalog.MenuTypePage.String()},
			{Key: "plugin:content-owner:legacy:query", ParentKey: "plugin:content-owner:legacy", Perms: "content-owner:notice:query", Type: catalog.MenuTypeButton.String()},
		},
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "notices", Version: "v1", Title: "Notices", Icon: "lucide:megaphone",
			Permission: "content-owner:notice:query", Slot: capmodel.FrontendSlotAdminSupportContent,
			Route: "notices", Entry: "admin/notices/index.html", BridgeCapabilities: []string{"content-owner.admin.v1"},
		}},
	}
	projected, err := manifestWithGovernedAdminSurfaceMenus(manifest)
	if err != nil {
		t.Fatalf("expected legacy dynamic route projection to succeed, got error: %v", err)
	}
	if len(projected.Menus) != 1 || projected.Menus[0].Key != "plugin:content-owner:surface-admin:notices" {
		t.Fatalf("expected only governed surface menu after pruning legacy tree, got %#v", projected.Menus)
	}
	if len(manifest.Menus) != 2 {
		t.Fatal("surface projection must not mutate legacy manifest menus")
	}
}
