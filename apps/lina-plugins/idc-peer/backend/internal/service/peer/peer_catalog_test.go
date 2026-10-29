// This file verifies restricted imported snapshots survive the same-system
// confidential response projection and never lose their source cycle ceiling.
package peer

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"slices"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestCatalogImportPreservesRestrictedPurchaseCycles(t *testing.T) {
	snapshot := channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportPurchaseCycleLimitProtocolVersion, Product: channelcommercecap.CatalogProduct{PublicSlug: "restricted", NameZhCN: "受限商品", NameEnUS: "Restricted product"}, ConfigurationRevision: "revision", Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, PricingComplete: true, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: "per_order_item", RenewalMode: "unavailable", MaximumPurchaseCycles: 1}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, PricePhase: "new_purchase", QuantityMode: "fixed", MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 1000}}, Tax: channelcommercecap.ImportTaxPolicy{Treatment: "exempt", CategoryCode: "service", RoundingScope: "line", RoundingMode: "half_up", RoundingQuantumMinor: 1}}}}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	executor := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":` + string(raw) + `}`}}}
	result, err := ExecuteResource(executor, "confidential-context", resourcecap.DynamicProviderMethodReadCatalogProduct, json.RawMessage(`{"reference":"restricted"}`))
	if err != nil {
		t.Fatal(err)
	}
	var detail resourceinterfaceprojectioncap.CatalogDetail
	if json.Unmarshal(result, &detail) != nil || detail.ImportSnapshot == nil || detail.ImportSnapshot.Fulfillment.MaximumPurchaseCycles != 1 || detail.ImportSnapshot.ProtocolVersion != channelcommercecap.ImportPurchaseCycleLimitProtocolVersion {
		t.Fatalf("restricted snapshot lost its ceiling: %s", result)
	}
	if len(executor.requests) != 1 || !slices.Contains(executor.requests[0].ResponseFields, "maximumPurchaseCycles") {
		t.Fatal("confidential response projection drops source maximum")
	}
	snapshot.Fulfillment.MaximumPurchaseCycles = 0
	raw, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	executor = &scriptedExecutor{steps: []step{{response: `{"code":0,"data":` + string(raw) + `}`}}}
	if _, err = ExecuteResource(executor, "confidential-context", resourcecap.DynamicProviderMethodReadCatalogProduct, json.RawMessage(`{"reference":"restricted"}`)); err == nil {
		t.Fatal("v2 snapshot with stripped cycle limit accepted")
	}
}

func TestCatalogImportPreservesHiddenCustomerDefault(t *testing.T) {
	hidden := false
	snapshot := channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportPurchaseCycleLimitProtocolVersion, Product: channelcommercecap.CatalogProduct{PublicSlug: "hidden", NameEnUS: "Hidden"}, ConfigurationRevision: "revision", Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{{Key: "flag", Type: capmodel.ConfigurationFieldBoolean, Label: &capmodel.LocalizedText{EnUS: "Flag", ZhCN: "开关"}}}}, Mappings: []channelcommercecap.ImportFieldMapping{{FieldKey: "flag", FieldToken: "opaque-token", CustomerVisible: &hidden, DefaultValue: json.RawMessage(`false`)}}, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, PricingComplete: true, Fulfillment: channelcommercecap.ImportFulfillment{Granularity: "per_order_item", RenewalMode: "unavailable"}, Prices: []channelcommercecap.ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "once", ChargeType: "one_time", PeriodUnit: "once", PeriodCount: 1, PricePhase: "new_purchase", QuantityMode: "fixed", MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{{AmountMinor: 100}}, Tax: channelcommercecap.ImportTaxPolicy{Treatment: "exempt", CategoryCode: "service", RoundingScope: "line", RoundingMode: "half_up", RoundingQuantumMinor: 1}}}}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	executor := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":` + string(raw) + `}`}}}
	result, err := ExecuteResource(executor, "confidential-context", resourcecap.DynamicProviderMethodReadCatalogProduct, json.RawMessage(`{"reference":"hidden"}`))
	if err != nil {
		t.Fatal(err)
	}
	var detail resourceinterfaceprojectioncap.CatalogDetail
	if err = json.Unmarshal(result, &detail); err != nil || detail.ImportSnapshot == nil {
		t.Fatalf("decode: %s %v", result, err)
	}
	mapping := detail.ImportSnapshot.Mappings[0]
	if mapping.CustomerVisible == nil || *mapping.CustomerVisible || string(mapping.DefaultValue) != "false" {
		t.Fatalf("hidden default lost: %+v", mapping)
	}
	for _, field := range []string{"customerVisible", "defaultValue"} {
		if !slices.Contains(executor.requests[0].ResponseFields, field) {
			t.Fatalf("missing response field %s", field)
		}
	}
}
