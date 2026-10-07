package catalog

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/internal/service/plugin/internal/plugintypes"
	"lina-core/pkg/plugin/capability/capmodel"
	bridgecontract "lina-core/pkg/plugin/pluginbridge/contract"
	"lina-core/pkg/plugin/pluginhost"
)

func TestValidateBusinessExtensionsKeepsManifestDimensionsIndependent(t *testing.T) {
	registerPermissiveOwnerValidator(t, "commerce-core", capmodel.BusinessExtensionProductFulfillment, "v1")
	manifest := &Manifest{
		ID: "phone-supplier", Name: "Phone supplier", Version: "v0.1.0",
		Type: "dynamic", Distribution: "managed",
		PublicAssets: []*PublicAssetSpec{{Source: "frontend", Mount: ""}},
		Dependencies: &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{
			{ID: "commerce-core", Version: ">=0.1.0 <1.0.0"},
		}},
		BusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
			Point: capmodel.BusinessExtensionProductFulfillment, Version: "v1",
			Role: capmodel.BusinessExtensionRoleProvider, Owner: "commerce-core",
			Methods: []string{"query-status", "fulfill"},
		}},
		AdminSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "phone-orders", Version: "v1", Title: "Phone orders", Permission: "phone-supplier:orders:view",
			Slot: capmodel.FrontendSlotAdminOrdersFinance, Route: "phone-orders", Entry: "frontend/admin/index.html",
			BridgeCapabilities: []string{"orders.query", "orders.fulfill"},
		}},
		CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
			Key: "my-phone-orders", Version: "v1", Title: "My phone orders",
			Slot: capmodel.FrontendSlotCustomerProducts, Route: "phone-orders", Entry: "frontend/customer/index.html",
			BridgeCapabilities: []string{"orders.query-own"},
		}},
	}

	if err := plugintypes.ValidateDependencySpec(manifest.ID, manifest.Dependencies); err != nil {
		t.Fatalf("validate dependencies: %v", err)
	}
	if err := ValidateBusinessExtensions(manifest); err != nil {
		t.Fatalf("validate business extensions: %v", err)
	}
	if err := ValidateFrontendSurfaces(manifest); err != nil {
		t.Fatalf("validate frontend surfaces: %v", err)
	}
	if manifest.Type != "dynamic" || manifest.Distribution != "managed" {
		t.Fatalf("runtime/distribution dimensions changed unexpectedly: %#v", manifest)
	}
	if got := manifest.BusinessExtensions[0].Methods; len(got) != 2 || got[0] != "fulfill" || got[1] != "query-status" {
		t.Fatalf("expected normalized method authorization set, got %#v", got)
	}
	if manifest.AdminSurfaces[0].Key == manifest.CustomerSurfaces[0].Key {
		t.Fatal("admin and customer surfaces must remain independent")
	}
}

func TestValidateBusinessExtensionsRequiresVersionedOwnerDependency(t *testing.T) {
	manifest := providerManifest("ssl-provider", "certificate-core", capmodel.BusinessExtensionProductFulfillment)
	manifest.Dependencies = nil
	if err := ValidateBusinessExtensions(manifest); err == nil {
		t.Fatal("expected provider without owner dependency to be rejected")
	}

	manifest.Dependencies = &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{
		ID: "certificate-core",
	}}}
	if err := ValidateBusinessExtensions(manifest); err == nil {
		t.Fatal("expected provider with unversioned owner dependency to be rejected")
	}
}

func TestDiscoverBusinessExtensionsPublishesFourPointsAndAuthorizesProvidersInBatch(t *testing.T) {
	registerPermissiveOwnerValidator(t, "commerce-core", capmodel.BusinessExtensionProductFulfillment, "v1")
	registerPermissiveOwnerValidator(t, "notification-core", capmodel.BusinessExtensionSMSChannel, "v1")
	registerPermissiveOwnerValidator(t, "payment-core", capmodel.BusinessExtensionPaymentMethod, "v1")
	registerPermissiveOwnerValidator(t, "website-core", capmodel.BusinessExtensionWebsiteTemplate, "v1")
	owners := []*Manifest{
		ownerManifest("commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query"}),
		ownerManifest("notification-core", capmodel.BusinessExtensionSMSChannel, []string{"send", "query"}),
		ownerManifest("payment-core", capmodel.BusinessExtensionPaymentMethod, []string{"pay", "query"}),
		ownerManifest("website-core", capmodel.BusinessExtensionWebsiteTemplate, []string{"render", "preview"}),
	}
	providers := []*Manifest{
		providerManifestWithMethods("phone-supplier", "commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query"}),
		providerManifestWithMethods("sms-supplier", "notification-core", capmodel.BusinessExtensionSMSChannel, []string{"send"}),
		providerManifestWithMethods("payment-gateway", "payment-core", capmodel.BusinessExtensionPaymentMethod, []string{"pay"}),
		providerManifestWithMethods("theme-pack", "website-core", capmodel.BusinessExtensionWebsiteTemplate, []string{"render"}),
	}
	manifests := append(owners, providers...)

	discovery, err := BuildBusinessExtensionDiscovery(manifests)
	if err != nil {
		t.Fatalf("discover business extensions: %v", err)
	}
	for _, point := range []string{
		capmodel.BusinessExtensionProductFulfillment,
		capmodel.BusinessExtensionSMSChannel,
		capmodel.BusinessExtensionPaymentMethod,
		capmodel.BusinessExtensionWebsiteTemplate,
	} {
		if len(discovery.Providers[point]) != 1 {
			t.Fatalf("expected one authorized provider for %s, got %#v", point, discovery.Providers[point])
		}
	}
}

func TestDiscoverBusinessExtensionsScalesWithOneManifestBatch(t *testing.T) {
	registerPermissiveOwnerValidator(t, "commerce-core", capmodel.BusinessExtensionProductFulfillment, "v1")
	manifests := []*Manifest{ownerManifest("commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query"})}
	for index := 0; index < 1000; index++ {
		manifests = append(manifests, providerManifestWithMethods(
			fmt.Sprintf("supplier-%04d", index), "commerce-core",
			capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query"},
		))
	}
	discovery, err := BuildBusinessExtensionDiscovery(manifests)
	if err != nil {
		t.Fatalf("discover one bounded manifest batch: %v", err)
	}
	if got := len(discovery.Providers[capmodel.BusinessExtensionProductFulfillment]); got != 1000 {
		t.Fatalf("expected 1000 providers from one in-memory batch, got %d", got)
	}
}

func TestDiscoverBusinessExtensionsRejectsUnpublishedMethod(t *testing.T) {
	_, err := BuildBusinessExtensionDiscovery([]*Manifest{
		ownerManifest("commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"query"}),
		providerManifestWithMethods("phone-supplier", "commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill"}),
	})
	if err == nil {
		t.Fatal("expected provider method outside owner publication to be rejected")
	}
}

func TestAuthorizeBusinessExtensionsFromManifestsOnlyAcceptsPublishedContract(t *testing.T) {
	registerPermissiveOwnerValidator(t, "commerce-core", capmodel.BusinessExtensionProductFulfillment, "v1")
	owner := ownerManifest("commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query"})
	provider := providerManifestWithMethods("phone-supplier", "commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query"})
	authorized, err := AuthorizeBusinessExtensionsFromManifests(provider, []*Manifest{owner, provider})
	if err != nil {
		t.Fatalf("authorize published provider: %v", err)
	}
	if len(authorized) != 1 || authorized[0].Point != capmodel.BusinessExtensionProductFulfillment {
		t.Fatalf("unexpected authorization result: %#v", authorized)
	}

	provider.BusinessExtensions[0].Methods = []string{"not-published"}
	if _, err = AuthorizeBusinessExtensionsFromManifests(provider, []*Manifest{owner, provider}); err == nil {
		t.Fatal("expected unpublished provider method to be denied")
	}
}

func TestAuthorizeBusinessExtensionsDoesNotRescanOrTrustUnrelatedProviders(t *testing.T) {
	registerPermissiveOwnerValidator(t, "commerce-core", capmodel.BusinessExtensionProductFulfillment, "v1")
	owner := ownerManifest("commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query"})
	target := providerManifestWithMethods("phone-supplier", "commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"fulfill", "query"})
	unrelated := providerManifestWithMethods("broken-sms", "missing-notification-core", capmodel.BusinessExtensionSMSChannel, []string{"send"})
	authorized, err := AuthorizeBusinessExtensionsFromManifests(target, []*Manifest{owner, target, unrelated})
	if err != nil {
		t.Fatalf("unrelated provider must not block target authorization: %v", err)
	}
	if len(authorized) != 1 {
		t.Fatalf("expected target declaration authorization, got %#v", authorized)
	}
}

func TestValidatePageBridgeRouteBindingsRequiresDeclaredExactRoute(t *testing.T) {
	manifest := &Manifest{CustomerSurfaces: []*capmodel.FrontendSurfaceDeclaration{{
		Key: "phone-orders", Version: "v1", BridgeCapabilities: []string{"phone-orders.query-own"},
	}}}
	validRoute := &bridgecontract.RouteContract{
		Method: "GET", Path: "/api/v1/phone-orders", Access: bridgecontract.AccessLogin,
		Meta: map[string]string{pluginhost.RouteMetaPageBridgeCapability: "phone-orders.query-own"},
	}
	if err := ValidatePageBridgeRouteBindings(manifest, []*bridgecontract.RouteContract{validRoute}); err != nil {
		t.Fatalf("expected exact page bridge route binding to pass: %v", err)
	}
	if err := ValidatePageBridgeRouteBindings(manifest, nil); err == nil {
		t.Fatal("expected an unbound surface capability to be rejected")
	}
	invalidRoute := *validRoute
	invalidRoute.Meta = map[string]string{pluginhost.RouteMetaPageBridgeCapability: "phone-orders.update"}
	if err := ValidatePageBridgeRouteBindings(manifest, []*bridgecontract.RouteContract{&invalidRoute}); err == nil {
		t.Fatal("expected an undeclared route capability to be rejected")
	}
}

func ownerManifest(id, point string, methods []string) *Manifest {
	return &Manifest{
		ID: id, Name: id, Version: "v0.1.0", Type: "source", Distribution: "builtin",
		BusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
			Point: point, Version: "v1", Role: capmodel.BusinessExtensionRoleOwner,
			Owner: id, Methods: append([]string(nil), methods...),
		}},
	}
}

func providerManifest(id, owner, point string) *Manifest {
	return providerManifestWithMethods(id, owner, point, nil)
}

func providerManifestWithMethods(id, owner, point string, methods []string) *Manifest {
	return &Manifest{
		ID: id, Name: id, Version: "v0.1.0", Type: "dynamic", Distribution: "managed",
		Dependencies: &plugintypes.DependencySpec{Plugins: []*plugintypes.PluginDependencySpec{{
			ID: owner, Version: ">=0.1.0 <1.0.0",
		}}},
		BusinessExtensions: []*capmodel.BusinessExtensionDeclaration{{
			Point: point, Version: "v1", Role: capmodel.BusinessExtensionRoleProvider,
			Owner: owner, Methods: append([]string(nil), methods...),
		}},
	}
}

func testProviderMetadata() *capmodel.BusinessExtensionProviderMetadata {
	return &capmodel.BusinessExtensionProviderMetadata{
		Type: "delivery-contract", Version: "v1", Values: map[string]any{"mode": "single"},
	}
}

func TestValidateBusinessExtensionsUsesOwnerValidatorForSourceAndDynamic(t *testing.T) {
	for _, pluginType := range []string{"source", "dynamic"} {
		t.Run(pluginType, func(t *testing.T) {
			registerTestOwnerValidator(t, "commerce-core", "v1", requireSingleDeliveryMetadata)
			manifest := providerManifestWithMethods("phone-supplier", "commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"provision", "query"})
			manifest.Type = pluginType
			manifest.BusinessExtensions[0].ProviderMetadata = testProviderMetadata()
			if err := ValidateBusinessExtensions(manifest); err != nil {
				t.Fatalf("owner-valid provider metadata was rejected: %v", err)
			}

			manifest.BusinessExtensions[0].ProviderMetadata = nil
			if err := ValidateBusinessExtensions(manifest); err == nil {
				t.Fatal("owner-required provider metadata was accepted when missing")
			}
		})
	}
}

func TestValidateBusinessExtensionsRejectsUnsafeOpaqueMetadataAndMissingOwnerValidator(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*capmodel.BusinessExtensionProviderMetadata)
	}{
		{name: "unsafe type", mutate: func(value *capmodel.BusinessExtensionProviderMetadata) { value.Type = "Delivery Contract" }},
		{name: "unsafe version", mutate: func(value *capmodel.BusinessExtensionProviderMetadata) { value.Version = "latest" }},
		{name: "oversized values", mutate: func(value *capmodel.BusinessExtensionProviderMetadata) {
			value.Values["large"] = strings.Repeat("x", 17*1024)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			registerTestOwnerValidator(t, "commerce-core", "v1", requireSingleDeliveryMetadata)
			manifest := providerManifestWithMethods("phone-supplier", "commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"provision", "query"})
			manifest.BusinessExtensions[0].ProviderMetadata = testProviderMetadata()
			test.mutate(manifest.BusinessExtensions[0].ProviderMetadata)
			if err := ValidateBusinessExtensions(manifest); err == nil {
				t.Fatal("unsafe opaque provider metadata was accepted")
			}
		})
	}
	manifest := providerManifestWithMethods("phone-supplier", "missing-owner", capmodel.BusinessExtensionProductFulfillment, []string{"provision", "query"})
	manifest.BusinessExtensions[0].ProviderMetadata = testProviderMetadata()
	if err := ValidateBusinessExtensions(manifest); err == nil {
		t.Fatal("provider metadata without its owner validator was accepted")
	}
	manifest.BusinessExtensions[0].ProviderMetadata = nil
	if err := ValidateBusinessExtensions(manifest); err == nil {
		t.Fatal("provider declaration without metadata or its owner validator was accepted")
	}
}

func TestValidateBusinessExtensionsRejectsProviderMetadataOnOwnerAndAcrossContractVersions(t *testing.T) {
	owner := ownerManifest("commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"provision", "query"})
	owner.BusinessExtensions[0].ProviderMetadata = testProviderMetadata()
	if err := ValidateBusinessExtensions(owner); err == nil {
		t.Fatal("owner was accepted with provider metadata")
	}

	registerTestOwnerValidators(t, "commerce-core", []string{"v1", "v2"}, requireSingleDeliveryMetadata)
	provider := providerManifestWithMethods("phone-supplier", "commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"provision", "query"})
	provider.BusinessExtensions[0].ProviderMetadata = testProviderMetadata()
	second := *provider.BusinessExtensions[0]
	second.Version = "v2"
	second.ProviderMetadata = testProviderMetadata()
	provider.BusinessExtensions = append(provider.BusinessExtensions, &second)
	if err := ValidateBusinessExtensions(provider); err == nil {
		t.Fatal("one provider release was accepted with multiple contract versions")
	}
}

func TestValidateBusinessExtensionsHostRejectsMultipleContractVersionsWithPermissiveOwner(t *testing.T) {
	registerTestOwnerValidators(t, "commerce-core", []string{"v1", "v2"}, func([]*capmodel.BusinessExtensionDeclaration) error {
		return nil
	})
	provider := providerManifestWithMethods("phone-supplier", "commerce-core", capmodel.BusinessExtensionProductFulfillment, []string{"provision", "query"})
	provider.BusinessExtensions[0].ProviderMetadata = testProviderMetadata()
	second := *provider.BusinessExtensions[0]
	second.Version = "v2"
	second.ProviderMetadata = testProviderMetadata()
	provider.BusinessExtensions = append(provider.BusinessExtensions, &second)
	if err := ValidateBusinessExtensions(provider); err == nil {
		t.Fatal("host accepted multiple contract versions because the owner validator was permissive")
	}
}

func registerTestOwnerValidator(t *testing.T, owner, version string, validator pluginhost.BusinessExtensionProviderValidator) {
	registerTestOwnerValidators(t, owner, []string{version}, validator)
}

func registerPermissiveOwnerValidator(t *testing.T, owner, point, version string) {
	t.Helper()
	declarations := pluginhost.NewDeclarations(owner)
	if err := declarations.BusinessExtensions().RegisterProviderValidator(point, version, func([]*capmodel.BusinessExtensionDeclaration) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	cleanup, err := pluginhost.RegisterSourcePluginForTest(declarations)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
}

func registerTestOwnerValidators(t *testing.T, owner string, versions []string, validator pluginhost.BusinessExtensionProviderValidator) {
	t.Helper()
	declarations := pluginhost.NewDeclarations(owner)
	for _, version := range versions {
		if err := declarations.BusinessExtensions().RegisterProviderValidator(capmodel.BusinessExtensionProductFulfillment, version, validator); err != nil {
			t.Fatal(err)
		}
	}
	cleanup, err := pluginhost.RegisterSourcePluginForTest(declarations)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanup)
}

func requireSingleDeliveryMetadata(declarations []*capmodel.BusinessExtensionDeclaration) error {
	if len(declarations) != 1 || declarations[0] == nil || declarations[0].ProviderMetadata == nil {
		return gerror.New("exactly one provider metadata declaration is required")
	}
	metadata := declarations[0].ProviderMetadata
	if metadata.Type != "delivery-contract" || metadata.Version != "v1" || metadata.Values["mode"] != "single" {
		return gerror.New("provider metadata is invalid")
	}
	return nil
}
