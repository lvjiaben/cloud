package catalog

import (
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
)

func TestBuildFrontendSurfaceDiscoverySeparatesHostsAndNamespacesRoutes(t *testing.T) {
	manifest := &Manifest{
		ID: "phone-provider", Name: "Phone Provider", Version: "v1.2.3", Type: "dynamic",
		PublicAssets: []*PublicAssetSpec{{Source: "frontend", Mount: ""}},
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "phone-orders", Version: "v1", Title: "Phone orders", Icon: "lucide:package",
			Permission: "phone-provider:orders:view", Slot: capmodel.FrontendSlotAdminOrdersFinance,
			Route: "phone-orders", Entry: "admin/phone-orders/index.html",
		}},
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "my-phone-orders", Version: "v1", Title: "My phone orders", Icon: "lucide:smartphone",
			Slot: capmodel.FrontendSlotCustomerProducts, Route: "phone-orders", Entry: "customer/phone-orders/index.html",
		}},
	}

	discovery, err := BuildFrontendSurfaceDiscovery([]*Manifest{manifest})
	if err != nil {
		t.Fatalf("expected surface discovery to succeed, got error: %v", err)
	}
	if len(discovery.Admin) != 1 || len(discovery.Customer) != 1 {
		t.Fatalf("expected separated admin/customer surfaces, got admin=%d customer=%d", len(discovery.Admin), len(discovery.Customer))
	}
	if discovery.Admin[0].RoutePath != "/plugins/phone-provider/phone-orders" {
		t.Fatalf("unexpected governed admin route: %s", discovery.Admin[0].RoutePath)
	}
	if discovery.Customer[0].RoutePath != "/account/plugins/phone-provider/phone-orders" {
		t.Fatalf("unexpected governed customer route: %s", discovery.Customer[0].RoutePath)
	}
	if discovery.Customer[0].EntryURL != "/x-assets/phone-provider/v1.2.3/customer/phone-orders/index.html" {
		t.Fatalf("unexpected version-frozen customer entry URL: %s", discovery.Customer[0].EntryURL)
	}
}

func TestBuildFrontendSurfaceDiscoveryRejectsUnpublishedSlot(t *testing.T) {
	manifest := &Manifest{
		ID: "ssl-provider", Version: "v1.0.0", Type: "dynamic",
		PublicAssets: []*PublicAssetSpec{{Source: "frontend", Mount: "customer"}},
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "certificates", Version: "v1", Title: "Certificates", Slot: "customer.top-level",
			Route: "certificates", Entry: "customer/index.html",
		}},
	}
	if _, err := BuildManifestFrontendSurfaceDiscovery(manifest); err == nil {
		t.Fatal("expected an unpublished customer slot to be rejected")
	}
}

func TestBuildFrontendSurfaceDiscoveryRequiresAdminPermission(t *testing.T) {
	manifest := &Manifest{
		ID: "ssl-provider", Version: "v1.0.0", Type: "dynamic",
		PublicAssets: []*PublicAssetSpec{{Source: "frontend", Mount: "admin"}},
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "certificates", Version: "v1", Title: "Certificates",
			Slot: capmodel.FrontendSlotAdminCatalogProducts, Route: "certificates", Entry: "admin/index.html",
		}},
	}
	if _, err := BuildManifestFrontendSurfaceDiscovery(manifest); err == nil {
		t.Fatal("expected an admin surface without a permission to be rejected")
	}
}

func TestBuildFrontendSurfaceDiscoveryRejectsRouteAndEntryEscapes(t *testing.T) {
	tests := []struct {
		name  string
		route string
		entry string
	}{
		{name: "reserved-looking absolute route", route: "/system/users", entry: "index.html"},
		{name: "route traversal", route: "../system", entry: "index.html"},
		{name: "absolute entry", route: "certificates", entry: "/admin/index.html"},
		{name: "entry traversal", route: "certificates", entry: "../index.html"},
		{name: "encoded entry traversal", route: "certificates", entry: "%2e%2e/index.html"},
		{name: "encoded entry separator", route: "certificates", entry: "admin%2findex.html"},
		{name: "remote executable entry", route: "certificates", entry: "https://evil.invalid/index.js"},
		{name: "entry query injection", route: "certificates", entry: "index.html?token=host"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := &Manifest{
				ID: "ssl-provider", Version: "v1.0.0", Type: "dynamic",
				PublicAssets: []*PublicAssetSpec{{Source: "frontend", Mount: ""}},
				AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
					Key: "certificates", Version: "v1", Title: "Certificates",
					Permission: "ssl-provider:certificates:view",
					Slot:       capmodel.FrontendSlotAdminCatalogProducts, Route: test.route, Entry: test.entry,
				}},
			}
			if _, err := BuildManifestFrontendSurfaceDiscovery(manifest); err == nil {
				t.Fatal("expected unsafe route or entry to be rejected")
			} else if !strings.Contains(err.Error(), "route is invalid") && !strings.Contains(err.Error(), "entry is invalid") {
				t.Fatalf("expected route/entry validation error, got: %v", err)
			}
		})
	}
}

func TestBuildFrontendSurfaceDiscoveryRejectsDuplicateRouteWithinHost(t *testing.T) {
	manifest := &Manifest{
		ID: "phone-provider", Version: "v1.0.0", Type: "source",
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{
			{Key: "phone-orders", Version: "v1", Title: "Phone orders", Slot: capmodel.FrontendSlotCustomerProducts, Route: "orders", Entry: "phone-orders"},
			{Key: "phone-after-sales", Version: "v1", Title: "Phone after-sales", Slot: capmodel.FrontendSlotCustomerSupport, Route: "orders", Entry: "phone-after-sales"},
		},
	}
	if _, err := BuildManifestFrontendSurfaceDiscovery(manifest); err == nil || !strings.Contains(err.Error(), "route is duplicated") {
		t.Fatalf("expected duplicate route to be rejected, got: %v", err)
	}
}

func TestBuildFrontendSurfaceDiscoveryKeepsSourceEntriesInsideGovernedRoutes(t *testing.T) {
	manifest := &Manifest{
		ID: "server-owner", Version: "v1.0.0", Type: "source",
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "servers", Version: "v1", Title: "Servers", Slot: capmodel.FrontendSlotCustomerProducts,
			Route: "servers", Entry: "server-owner-servers",
		}},
	}
	discovery, err := BuildManifestFrontendSurfaceDiscovery(manifest)
	if err != nil {
		t.Fatalf("expected source surface discovery to succeed, got error: %v", err)
	}
	if discovery.Customer[0].EntryURL != "" {
		t.Fatalf("source surfaces must not be converted to runtime asset URLs: %s", discovery.Customer[0].EntryURL)
	}
	if discovery.Customer[0].RoutePath != "/account/plugins/server-owner/servers" {
		t.Fatalf("unexpected source surface route: %s", discovery.Customer[0].RoutePath)
	}
}
