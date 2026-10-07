package plugin

import (
	"testing"

	v1 "lina-core/api/plugin/v1"
	pluginsvc "lina-core/internal/service/plugin"
)

func TestProjectPluginDynamicItemHidesCustomerSurfacesFromAnonymousCallers(t *testing.T) {
	item := runtimeStateWithCustomerSurface()

	projected := projectPluginDynamicItem(item, false)
	if projected == nil {
		t.Fatal("expected runtime item projection")
	}
	if len(projected.CustomerSurfaces) != 0 {
		t.Fatalf("anonymous callers must not receive customer surfaces, got %d", len(projected.CustomerSurfaces))
	}
}

func TestProjectPluginDynamicItemReturnsOnlyCustomerSurfaceFieldsAfterAuthentication(t *testing.T) {
	item := runtimeStateWithCustomerSurface()

	projected := projectPluginDynamicItem(item, true)
	if len(projected.CustomerSurfaces) != 1 {
		t.Fatalf("expected one authenticated customer surface, got %d", len(projected.CustomerSurfaces))
	}
	surface := projected.CustomerSurfaces[0]
	if surface.PluginId != "phone-provider" || surface.PluginType != v1.PluginTypeDynamic {
		t.Fatalf("unexpected customer surface owner: %#v", surface)
	}
	if surface.RoutePath != "/account/plugins/phone-provider/orders" || surface.EntryUrl == "" {
		t.Fatalf("expected host-governed route and frozen entry URL, got %#v", surface)
	}
}

func runtimeStateWithCustomerSurface() *pluginsvc.PluginDynamicStateItem {
	return &pluginsvc.PluginDynamicStateItem{
		Id: "phone-provider", Installed: 1, Enabled: 1, Version: "v1.0.0", Type: "dynamic",
		CustomerSurfaces: []*pluginsvc.FrontendSurfaceBinding{{
			PluginID: "phone-provider", PluginType: "dynamic", Key: "orders", Version: "v1",
			Title: "Phone orders", Slot: "customer.products",
			RoutePath: "/account/plugins/phone-provider/orders", Entry: "customer/index.html",
			EntryURL: "/x-assets/phone-provider/v1.0.0/customer/index.html",
		}},
	}
}
