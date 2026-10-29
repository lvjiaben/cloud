// This file verifies native schema, quantity tariffs and immutable mappings
// against source-shaped fixtures without invoking an external supplier.
package finance

import (
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func TestNativeCatalogImportsArbitraryOptionsAndCustomerFields(t *testing.T) {
	data := catalogDetailFixture(t)
	data["customfields"] = []any{map[string]any{"id": 51, "fieldname": "联系备注", "fieldtype": "textarea", "required": 1, "regexpr": ""}, map[string]any{"id": 52, "fieldname": "应用版本", "fieldtype": "dropdown", "fieldoptions": "标准版,高级版", "required": 0, "regexpr": ""}}
	detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || detail.ImportSnapshot == nil {
		t.Fatalf("native definition did not import: %+v %v", detail, err)
	}
	snapshot := detail.ImportSnapshot
	if snapshot.Validate() != nil || len(snapshot.Schema.Fields) != 3 || snapshot.Schema.Fields[1].Type != capmodel.ConfigurationFieldMultiLine || !snapshot.Schema.Fields[1].Required || snapshot.Mappings[0].FieldToken != "config:10" || snapshot.Mappings[1].FieldToken != "custom:51" || snapshot.Mappings[2].Options[1].OptionToken != "高级版" {
		t.Fatalf("native choices or custom fields changed: %+v", snapshot)
	}
	assertNativeRemoteSchedule(t, detail)
	before := snapshot.ConfigurationRevision
	data["customfields"].([]any)[0].(map[string]any)["required"] = 0
	changed, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || changed.ImportSnapshot == nil || changed.ImportSnapshot.ConfigurationRevision == before {
		t.Fatalf("source restriction change did not change revision: %+v %v", changed, err)
	}
}

func TestNativeCatalogPreservesQuantityAndGraduatedBands(t *testing.T) {
	for _, kind := range []int{4, 15} {
		t.Run(string(rune('a'+kind)), func(t *testing.T) {
			data := catalogDetailFixture(t)
			option := data["config_groups"].([]any)[0].(map[string]any)["options"].([]any)[0].(map[string]any)
			option["option_type"], option["qty_minimum"], option["qty_maximum"] = kind, 0, 200
			choice := option["sub"].([]any)[0].(map[string]any)
			choice["qty_minimum"], choice["qty_maximum"] = 0, 100
			row := choice["pricings"].([]any)[0].(map[string]any)
			row["monthly"] = "2.00"
			secondRow := map[string]any{}
			for key, value := range row {
				secondRow[key] = value
			}
			secondRow["monthly"] = "1.00"
			option["sub"] = []any{choice, map[string]any{"id": 21, "option_name": "第二档", "qty_minimum": 101, "qty_maximum": 200, "pricings": []any{secondRow}}}
			detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
			if err != nil || detail.ImportSnapshot == nil {
				t.Fatalf("quantity import failed: %+v %v", detail, err)
			}
			snapshot := detail.ImportSnapshot
			price := nativeReferenceSchedule(t, data).Options[0]
			expected := channelcommercecap.ImportConfigurationPricePerUnit
			if kind == 15 {
				expected = channelcommercecap.ImportConfigurationPriceGraduated
			}
			if snapshot.Schema.Fields[0].Type != capmodel.ConfigurationFieldInteger || snapshot.Schema.Fields[0].Minimum != "0" || snapshot.Schema.Fields[0].Maximum != "200" || price.PricingMode != expected || len(price.Bands) != 2 || price.Bands[0].MaximumQuantity == nil || *price.Bands[0].MaximumQuantity != 100 || price.Bands[1].MinimumQuantity != 100 || price.Bands[1].MaximumQuantity != nil {
				t.Fatalf("quantity restrictions or marginal boundaries changed: %+v %+v", snapshot.Schema.Fields[0], price)
			}
		})
	}
}

func TestNativeCatalogDoesNotDropUnrepresentedRestrictions(t *testing.T) {
	for _, scenario := range []string{"advanced", "parent", "expression", "custom kind", "unknown option", "quantity precision", "stage setup"} {
		t.Run(scenario, func(t *testing.T) {
			data := catalogDetailFixture(t)
			option := data["config_groups"].([]any)[0].(map[string]any)["options"].([]any)[0].(map[string]any)
			switch scenario {
			case "advanced":
				data["advanced"] = []any{map[string]any{"id": 1, "config_id": 10, "relation": "unsupported", "type": "condition", "relation_id": 0, "sub_id": map[string]any{"20": map[string]any{}}}}
			case "parent":
				option["linkage_pid"] = 11
			case "expression":
				data["customfields"] = []any{map[string]any{"id": 51, "fieldname": "验证资料", "fieldtype": "text", "regexpr": "/^[A-Z]+$/"}}
			case "custom kind":
				data["customfields"] = []any{map[string]any{"id": 51, "fieldname": "验证资料", "fieldtype": "password"}}
			case "unknown option":
				option["option_type"] = 999
			case "quantity precision":
				option["option_type"], option["qty_minimum"], option["qty_maximum"] = 4, 0, 200
				choice := option["sub"].([]any)[0].(map[string]any)
				choice["qty_minimum"], choice["qty_maximum"] = 0, 200
				choice["pricings"].([]any)[0].(map[string]any)["monthly"] = "2.505"
			case "stage setup":
				option["option_type"], option["qty_minimum"], option["qty_maximum"] = 15, 0, 200
				choice := option["sub"].([]any)[0].(map[string]any)
				choice["qty_minimum"], choice["qty_maximum"] = 0, 100
				row := choice["pricings"].([]any)[0].(map[string]any)
				second := map[string]any{}
				for key, value := range row {
					second[key] = value
				}
				second["msetupfee"] = "2.00"
				option["sub"] = []any{choice, map[string]any{"id": 21, "option_name": "Second", "qty_minimum": 101, "qty_maximum": 200, "pricings": []any{second}}}
			}
			detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
			if scenario == "quantity precision" || scenario == "stage setup" {
				if err != nil || detail.ImportSnapshot == nil {
					t.Fatalf("remote source tariff rejected: %+v %v", detail, err)
				}
				assertNativeRemoteSchedule(t, detail)
				return
			}
			if err != nil || detail.ImportSnapshot != nil || detail.PriceScheduleComplete {
				t.Fatalf("restriction was silently discarded: %+v %v", detail, err)
			}
		})
	}
}

func TestNativeAccountQuoteUsesEffectiveAmountsAndCurrency(t *testing.T) {
	schedule := channelcommercecap.ImportPriceSchedule{TermReference: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1}
	for _, test := range []struct {
		name, raw string
		total     string
		valid     bool
	}{
		{"ordinary", `{"status":200,"currency":{"id":3,"code":"CNY"},"products":{"type":false,"signal_price":"100.00","signal_setupfee":"5.00"}}`, "10500", true},
		{"account discount", `{"status":200,"currency":{"id":3,"code":"CNY"},"products":{"type":{"type":1,"bates":85},"signal_price":"100.00","signal_setupfee":"5.00","sale_signal_price":"85.00","sale_setupfee_total":"4.25"}}`, "8925", true},
		{"missing effective amount", `{"status":200,"currency":{"id":3,"code":"CNY"},"products":{"type":{"type":1},"signal_price":"100.00","signal_setupfee":"5.00"}}`, "", false},
		{"wrong currency", `{"status":200,"currency":{"id":1,"code":"USD"},"products":{"type":false,"signal_price":"100.00","signal_setupfee":"5.00"}}`, "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			term, err := nativeAccountTerm(json.RawMessage(test.raw), "CNY", "3", schedule)
			if test.valid {
				if err != nil || term.TotalAmount != test.total {
					t.Fatalf("wrong account quote: %+v %v", term, err)
				}
			} else if err == nil {
				t.Fatal("invalid authoritative quote was accepted")
			}
		})
	}
}

func TestNativeRevisionChangesForRestrictionsButNotLiveAmounts(t *testing.T) {
	data := catalogDetailFixture(t)
	before, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || before.ImportSnapshot == nil {
		t.Fatal(err)
	}
	data["product_pricings"].([]any)[0].(map[string]any)["monthly"] = "99.99"
	after, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || after.ImportSnapshot == nil {
		t.Fatal(err)
	}
	if before.ImportSnapshot.ConfigurationRevision != after.ImportSnapshot.ConfigurationRevision {
		t.Fatal("live amount changed configuration identity")
	}
	data["config_groups"].([]any)[0].(map[string]any)["options"].([]any)[0].(map[string]any)["option_name"] = "Changed configuration"
	renamed, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || renamed.ImportSnapshot == nil || renamed.ImportSnapshot.ConfigurationRevision == after.ImportSnapshot.ConfigurationRevision {
		t.Fatal("source schema change retained stale identity")
	}
}

func TestNativeAccountDiscountPublishesRemotePriceMetadata(t *testing.T) {
	data := catalogDetailFixture(t)
	data["flag"] = map[string]any{"type": 1, "bates": 85}
	detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || detail.ImportSnapshot == nil || detail.ImportSnapshot.Validate() != nil || detail.PriceScheduleComplete || detail.ImportSnapshot.PricingMode != channelcommercecap.ImportPricingRemoteEvaluation || detail.ImportSnapshot.ProtocolVersion != channelcommercecap.ImportRemoteEvaluationProtocolVersion || len(detail.ImportSnapshot.Prices[0].Base) > 0 || len(detail.ImportSnapshot.Prices[0].Options) > 0 {
		t.Fatalf("discounted source became static or disappeared %+v %v", detail, err)
	}
}

// Published component amounts are reference-only; the account quote owns every
// final amount. This helper exercises exact tariff parsing without exporting it.
func nativeReferenceSchedule(t *testing.T, data map[string]any) *channelcommercecap.ImportPriceSchedule {
	t.Helper()
	var definition nativeCatalogDefinition
	if json.Unmarshal(mustCatalogJSON(t, data), &definition) != nil {
		t.Fatal("invalid fixture")
	}
	_, prices, complete, err := catalogProductPrices(definition.Products, json.RawMessage(`false`), definition.Prices)
	if err != nil || !complete || len(prices) == 0 {
		t.Fatalf("invalid source tariff %v", err)
	}
	schedule, err := nativePriceSchedule(prices[0], definition)
	if err != nil || schedule == nil {
		t.Fatalf("unrepresentable reference tariff %v", err)
	}
	return schedule
}
func assertNativeRemoteSchedule(t *testing.T, detail *resourceinterfaceprojectioncap.CatalogDetail) {
	t.Helper()
	snapshot := detail.ImportSnapshot
	if snapshot == nil || snapshot.Validate() != nil || resourcecap.ValidateNativeImportSnapshot(snapshot) != nil || snapshot.PricingMode != channelcommercecap.ImportPricingRemoteEvaluation || snapshot.PricingComplete || detail.PriceScheduleComplete || len(detail.BasePrices) > 0 || len(detail.Options) > 0 || len(snapshot.Prices) == 0 {
		t.Fatalf("source became a static tariff: %+v", detail)
	}
	for _, price := range snapshot.Prices {
		if len(price.Base) > 0 || len(price.Options) > 0 {
			t.Fatalf("raw component amounts escaped: %+v", price)
		}
	}
}
