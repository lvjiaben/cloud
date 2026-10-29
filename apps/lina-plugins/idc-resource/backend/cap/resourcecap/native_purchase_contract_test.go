// This file verifies explicit native admission independently of the server
// contract and rejects invented provider-token and option-price mappings.
package resourcecap

import (
	"strings"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func TestNativeContractRequiresDeclaredProtocolAndIdentitySchema(t *testing.T) {
	evidence := nativeProductEvidence(t)
	if err := ValidateNativeProductEvidence(evidence); err != nil {
		t.Fatal(err)
	}
	if ValidateServerProductEvidence(evidence) == nil {
		t.Fatal("native product bypassed server hardware contract")
	}
	for _, scenario := range []string{"missing protocol", "missing identity", "wrong revision", "extra hardware", "customer fields", "unauthorized candidates"} {
		t.Run(scenario, func(t *testing.T) {
			current := nativeProductEvidence(t)
			switch scenario {
			case "missing protocol":
				delete(current.ProviderMetadata.Values, "saleImportProtocol")
			case "missing identity":
				current.ProductConfiguration.Fields = current.ProductConfiguration.Fields[:1]
			case "wrong revision":
				current.ProductConfiguration.Fields[1].MaximumLength = 8192
			case "extra hardware":
				current.ProductConfiguration.Fields = append(current.ProductConfiguration.Fields, &capmodel.ConfigurationFieldDeclaration{Key: "cpu", Type: capmodel.ConfigurationFieldInteger})
			case "customer fields":
				current.CustomerConfiguration = &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "operating_system", Type: capmodel.ConfigurationFieldSingleSelect}}}
			case "unauthorized candidates":
				current.AuthorizedMethods = []string{"provision", "query"}
			}
			if ValidateNativeProductEvidence(current) == nil {
				t.Fatal("native source authority was guessed")
			}
		})
	}
}

func TestNativeImportSnapshotRequiresReversibleSourceMappings(t *testing.T) {
	for _, scenario := range []string{"valid", "field token", "choice token", "price field", "price choice", "dynamic candidates"} {
		t.Run(scenario, func(t *testing.T) {
			snapshot := nativeImportContractFixture()
			switch scenario {
			case "field token":
				snapshot.Mappings[0].FieldToken = "config:11"
			case "choice token":
				snapshot.Mappings[0].Options[0].OptionToken = "21"
			case "price field":
				snapshot.Prices[0].Options[0].FieldKey = "cfg_999"
			case "price choice":
				snapshot.Prices[0].Options[0].Value = "999"
			case "dynamic candidates":
				snapshot.Schema.Fields[0].DynamicCandidates = &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "untrusted.method"}
			}
			err := ValidateNativeImportSnapshot(snapshot)
			if scenario == "valid" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unproven native mapping passed admission")
			}
		})
	}
}

func nativeProductEvidence(t *testing.T) *capmodel.BusinessExtensionProviderEvidence {
	t.Helper()
	metadata, err := productfulfillmentcap.NewProviderMetadata(&productfulfillmentcap.ProviderDeclaration{SaleImportProtocol: productfulfillmentcap.SaleImportProtocolNativeFinanceV1, Capability: productfulfillmentcap.ProviderCapability{EnUS: "Native purchase", ZhCN: "原生转售"}, EntitlementOwnerID: OwnerPluginID, Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingManagement: true, RenewalSupport: productfulfillmentcap.RenewalSupportUnsupported, ExpiryManagement: productfulfillmentcap.ExpiryManagementUnsupported, MethodRoles: productfulfillmentcap.ProviderMethodRoles{Provision: "provision", Query: "query"}})
	if err != nil {
		t.Fatal(err)
	}
	return &capmodel.BusinessExtensionProviderEvidence{Point: capmodel.BusinessExtensionProductFulfillment, ContractVersion: "v1", AuthorizedMethods: []string{"configuration.candidates", "provision", "query"}, ProviderMetadata: metadata, ProductConfiguration: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "upstream_product_id", Type: capmodel.ConfigurationFieldSingleSelect, Required: true, DynamicCandidates: &capmodel.DynamicCandidateDeclaration{Version: "v1", Method: "configuration.candidates"}}, {Key: "upstream_configuration_revision", Type: capmodel.ConfigurationFieldSingleLine, Required: true, MinimumLength: 64, MaximumLength: 64}}}}
}

func nativeImportContractFixture() *channelcommercecap.ImportSnapshot {
	label := &capmodel.LocalizedText{ZhCN: "原生参数", EnUS: "Native option"}
	return &channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportPurchaseCycleLimitProtocolVersion, Product: channelcommercecap.CatalogProduct{PublicSlug: "41"}, ConfigurationRevision: strings.Repeat("a", 64), Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "cfg_10", Type: capmodel.ConfigurationFieldSingleSelect, Label: label, StaticCandidates: []*capmodel.ConfigurationCandidate{{Value: "20", Label: label}}}}}, Mappings: []channelcommercecap.ImportFieldMapping{{FieldKey: "cfg_10", FieldToken: "config:10", Options: []channelcommercecap.ImportOptionMapping{{Value: "20", OptionToken: "20"}}}}, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, PricingComplete: true, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingService: true, RenewalMode: productfulfillmentcap.RenewalModeUnavailable, MaximumPurchaseCycles: 1}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 1000}}, Options: []channelcommercecap.ImportOptionPrice{{FieldKey: "cfg_10", Value: "20", Bands: []channelcommercecap.ImportPriceBand{{AmountMinor: 100}}}}}}}
}

func TestNativeV10EvidenceUsesExplicitRemoteProtocol(t *testing.T) {
	evidence := nativeProductEvidence(t)
	evidence.ProviderMetadata.Values["saleImportProtocol"] = string(productfulfillmentcap.SaleImportProtocolNativeV10CommonV1)
	if err := ValidateNativeProductEvidence(evidence); err != nil {
		t.Fatal(err)
	}
	evidence.ProviderMetadata.Values["saleImportProtocol"] = "native_unknown"
	if ValidateNativeProductEvidence(evidence) == nil {
		t.Fatal("unknown native source protocol admitted")
	}
}
