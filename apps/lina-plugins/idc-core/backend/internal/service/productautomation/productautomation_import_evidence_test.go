// This file tests that portable customer schemas remain bound to the exact
// provider release and immutable resource read revision.

package productautomation

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"strings"
	"testing"
)

func TestPortableCustomerSchemaEvidence(t *testing.T) {
	exact := automationEvidence(nil)
	exact.PluginID = portableImportProviderID
	snapshot := portableAutomationSnapshot()
	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	binding := &entity.CatalogProductVersionFulfillmentBinding{VersionNo: "ver_" + strings.Repeat("a", 32), InterfaceInstanceNo: "pfi_" + strings.Repeat("b", 32), InterfaceDisplayNameSnapshot: exact.DisplayName, PluginId: exact.PluginID, ExtensionPointCode: exact.Point, PluginReleaseVersion: exact.ReleaseVersion, ContractVersion: exact.ContractVersion, AuthorizedMethods: exact.AuthorizedMethods, ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest}
	revision := &entity.ProductAutomationInterfaceConfigurationRevision{InterfaceInstanceNo: binding.InterfaceInstanceNo, ResourceOwnerConfigurationRevision: "rci_" + strings.Repeat("c", 32)}
	read := resourceinterfaceprojectioncap.CatalogReadEvidence{InterfaceInstanceNo: binding.InterfaceInstanceNo, OwnerID: "idc-resource", ResourceChannelRef: "CHN_demo", InterfaceConfigurationRevision: revision.ResourceOwnerConfigurationRevision, PluginID: exact.PluginID, PluginReleaseVersion: exact.ReleaseVersion, ContractVersion: exact.ContractVersion, AuthorizedMethods: exact.AuthorizedMethods, ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest}
	readJSON, err := json.Marshal(read)
	if err != nil {
		t.Fatal(err)
	}
	contract := &entity.CatalogImportContract{VersionNo: binding.VersionNo, InterfaceInstanceNo: binding.InterfaceInstanceNo, Snapshot: string(encoded), SnapshotDigest: configurationDigest(encoded), ReadEvidence: string(readJSON)}
	got, err := verifiedPortableCustomerSchema(contract, binding, revision, exact, "{}")
	if err != nil || !sameExactConfiguration(got, snapshot.Schema) {
		t.Fatalf("valid import rejected: %v", err)
	}
	for _, scenario := range []struct {
		name   string
		change func(*entity.CatalogImportContract, *entity.CatalogProductVersionFulfillmentBinding, *entity.ProductAutomationInterfaceConfigurationRevision)
	}{
		{"other version", func(c *entity.CatalogImportContract, _ *entity.CatalogProductVersionFulfillmentBinding, _ *entity.ProductAutomationInterfaceConfigurationRevision) {
			c.VersionNo = "ver_" + strings.Repeat("d", 32)
		}},
		{"schema tamper", func(c *entity.CatalogImportContract, _ *entity.CatalogProductVersionFulfillmentBinding, _ *entity.ProductAutomationInterfaceConfigurationRevision) {
			c.Snapshot = strings.Replace(c.Snapshot, "Region", "Changed", 1)
		}},
		{"read revision", func(_ *entity.CatalogImportContract, _ *entity.CatalogProductVersionFulfillmentBinding, r *entity.ProductAutomationInterfaceConfigurationRevision) {
			r.ResourceOwnerConfigurationRevision = "rci_" + strings.Repeat("d", 32)
		}},
		{"exact release", func(_ *entity.CatalogImportContract, b *entity.CatalogProductVersionFulfillmentBinding, _ *entity.ProductAutomationInterfaceConfigurationRevision) {
			b.ReleaseDigest = strings.Repeat("f", 64)
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			c, b, r := *contract, *binding, *revision
			scenario.change(&c, &b, &r)
			if _, e := verifiedPortableCustomerSchema(&c, &b, &r, exact, "{}"); e == nil {
				t.Fatal("invalid import accepted")
			}
		})
	}
}

func portableAutomationSnapshot() *channelcommercecap.ImportSnapshot {
	label := &capmodel.LocalizedText{ZhCN: "区域", EnUS: "Region"}
	return &channelcommercecap.ImportSnapshot{ProtocolVersion: "1", Product: channelcommercecap.CatalogProduct{PublicSlug: "demo"}, ConfigurationRevision: "revision-a", Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "region", Type: capmodel.ConfigurationFieldSingleLine, Label: label}}}, Mappings: []channelcommercecap.ImportFieldMapping{{FieldKey: "region", FieldToken: "field-token"}}, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "USD", Precision: 2}}, PricingComplete: true, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: "per_order_item", RenewalMode: "unavailable"}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "USD", TermReference: "term-token", ChargeType: "one_time", PeriodUnit: "once", PeriodCount: 1, PricePhase: "new_purchase", QuantityMode: "fixed", MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 100}}, Tax: channelcommercecap.ImportTaxPolicy{Treatment: "exempt", CategoryCode: "service", RoundingScope: "line", RoundingMode: "half_up", RoundingQuantumMinor: 1}}}}
}

func TestNativeImportAdmissionRequiresExactExplicitProtocol(t *testing.T) {
	exact := automationEvidence([]string{"provision", "query.status"})
	exact.PluginID = "unlisted-native-provider"
	declaration := testFulfillmentDeclaration()
	declaration.SaleImportProtocol = productfulfillmentcap.SaleImportProtocolNativeFinanceV1
	declaration.Granularity = productfulfillmentcap.GranularityPerQuantityUnit
	declaration.OngoingManagement = true
	declaration.EntitlementOwnerID = "idc-resource"
	exact.ProviderMetadata = mustTestProviderMetadata(t, declaration)
	if !supportsPortableImport(exact) {
		t.Fatal("explicit native protocol rejected")
	}
	snapshot := portableAutomationSnapshot()
	snapshot.Product.PublicSlug = "41"
	snapshot.Fulfillment.MaximumPurchaseCycles = 1
	snapshot.ProtocolVersion = channelcommercecap.ImportPurchaseCycleLimitProtocolVersion
	snapshot.Fulfillment.Granularity = productfulfillmentcap.GranularityPerQuantityUnit
	snapshot.Fulfillment.OngoingService = true
	read := &resourceinterfaceprojectioncap.CatalogReadEvidence{SaleImportProtocol: declaration.SaleImportProtocol, InterfaceInstanceNo: "pfi_" + strings.Repeat("a", 32), OwnerID: "idc-resource", ResourceChannelRef: "CHN_native", InterfaceConfigurationRevision: "rci_" + strings.Repeat("b", 32), PluginID: exact.PluginID, PluginReleaseVersion: exact.ReleaseVersion, ContractVersion: exact.ContractVersion, AuthorizedMethods: exact.AuthorizedMethods, ReleaseDigest: exact.ReleaseDigest, DeclarationDigest: exact.DeclarationDigest}
	supplied := []resourceinterfaceprojectioncap.CatalogConfigurationValue{{FieldKey: "upstream_product_id", TextValue: &snapshot.Product.PublicSlug}, {FieldKey: "upstream_configuration_revision", TextValue: &snapshot.ConfigurationRevision}}
	if err := validatePortableImportEvidence(snapshot, read, exact, supplied); err != nil {
		t.Fatal(err)
	}
	read.SaleImportProtocol = ""
	if validatePortableImportEvidence(snapshot, read, exact, supplied) == nil {
		t.Fatal("unstamped native read accepted")
	}
	read.SaleImportProtocol = declaration.SaleImportProtocol
	read.ReleaseDigest = strings.Repeat("f", 64)
	if validatePortableImportEvidence(snapshot, read, exact, supplied) == nil {
		t.Fatal("another release read accepted")
	}
	declaration.SaleImportProtocol = ""
	exact.ProviderMetadata = mustTestProviderMetadata(t, declaration)
	if supportsPortableImport(exact) {
		t.Fatal("provider name without explicit native capability accepted")
	}
}
