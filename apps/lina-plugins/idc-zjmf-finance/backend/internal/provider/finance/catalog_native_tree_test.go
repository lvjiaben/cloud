// These tests retain a genuine option_type=20 hierarchy and each ancestor's
// billing/mapping facts. Nonterminal and cross-branch inputs cannot be purchased.
package finance

import (
	"context"
	"encoding/json"
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func nativeTreeFixture(t *testing.T) map[string]any {
	data := catalogDetailFixture(t)
	row := map[string]any{}
	for _, cycle := range catalogCycles {
		row[cycle.amount] = "-1.00"
		row[cycle.setup] = "0.00"
	}
	row["code"] = "CNY"
	row["monthly"] = "2.50"
	childRow := map[string]any{}
	for key, value := range row {
		childRow[key] = value
	}
	childRow["monthly"] = "3.25"
	childRow["msetupfee"] = "0.10"
	data["config_groups"].([]any)[0].(map[string]any)["options"] = []any{
		map[string]any{"id": 10, "option_name": "Location", "option_type": 20, "linkage_pid": 0, "sub": []any{map[string]any{"id": 20, "option_name": "East", "linkage_pid": 0, "pricings": []any{row}}, map[string]any{"id": 21, "option_name": "West", "linkage_pid": 0, "pricings": []any{row}}}},
		map[string]any{"id": 11, "option_name": "Datacenter", "option_type": 20, "linkage_pid": 10, "sub": []any{map[string]any{"id": 30, "option_name": "East A", "linkage_pid": 20, "pricings": []any{childRow}}, map[string]any{"id": 31, "option_name": "West B", "linkage_pid": 21, "pricings": []any{childRow}}}},
	}
	return data
}
func TestNativeTreeImportPreservesHierarchyLeafPricesAndNativeCart(t *testing.T) {
	data := nativeTreeFixture(t)
	detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || detail.ImportSnapshot == nil {
		t.Fatalf("tree source rejected: %+v %v", detail, err)
	}
	snapshot := detail.ImportSnapshot
	if snapshot.Validate() != nil || len(snapshot.Schema.Fields) != 1 || snapshot.Schema.Fields[0].Type != capmodel.ConfigurationFieldTreeSelect || snapshot.Schema.Fields[0].StaticCandidates[1].ParentValue != "10:20" || string(snapshot.Mappings[0].DefaultValue) != `"10:20/11:30"` {
		t.Fatalf("tree was flattened or default changed: %+v", snapshot)
	}
	foundPrice := false
	for _, price := range nativeReferenceSchedule(t, data).Options {
		if price.Value == "10:20/11:30" {
			foundPrice = price.Bands[0].AmountMinor == 575 && price.Bands[0].SetupAmountMinor == 10
		}
	}
	if !foundPrice {
		t.Fatalf("ancestor charges lost %+v", snapshot.Prices)
	}
	evaluated, err := evaluateNativeSelections(snapshot, nil)
	if err != nil || !evaluated.Valid || len(evaluated.Fields[0].Options[0].Children) != 1 || evaluated.Values[0].TreeOptionTokens[0] != "10:20/11:30" {
		t.Fatalf("source default/hierarchy invalid: %+v %v", evaluated, err)
	}
	purchase := channelcommercecap.Purchase{PublicSlug: "41", ConfigurationRevision: snapshot.ConfigurationRevision, CurrencyCode: "CNY", BillingTermToken: "monthly", PurchaseCycles: 1, Quantity: 1, Values: evaluated.Values}
	raw, err := BuildNativePurchaseParameters(context.Background(), purchase)
	if err != nil {
		t.Fatal(err)
	}
	parameters, err := decodePurchaseParameters(raw)
	if err != nil || parameters.ConfigOptions["10"] != "20" || parameters.ConfigOptions["11"] != "30" || len(parameters.ConfigOptions) != 2 {
		t.Fatalf("leaf did not restore whole native branch %s %v", raw, err)
	}
	invalid := []channelcommercecap.PurchaseValue{{FieldToken: "config:10", Type: "tree_select", TreeOptionTokens: []string{"10:20"}}}
	incomplete, err := evaluateNativeSelections(snapshot, invalid)
	if err != nil || incomplete.Valid {
		t.Fatalf("nonterminal became purchasable %+v %v", incomplete, err)
	}
	invalid[0].TreeOptionTokens = []string{"10:20/11:31"}
	crossBranch, err := evaluateNativeSelections(snapshot, invalid)
	if err != nil || crossBranch.Valid {
		t.Fatalf("cross-branch leaf became purchasable %+v %v", crossBranch, err)
	}
}
func TestNativeTreeRejectsCyclesOrUnknownParents(t *testing.T) {
	data := nativeTreeFixture(t)
	options := data["config_groups"].([]any)[0].(map[string]any)["options"].([]any)
	options[1].(map[string]any)["linkage_pid"] = 999
	raw := mustCatalogJSON(t, map[string]any{"status": 200, "data": data})
	var envelope struct {
		Data nativeCatalogDefinition `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		t.Fatal("invalid fixture")
	}
	if _, err := nativeTrees(envelope.Data); err == nil {
		t.Fatal("orphan linked configuration imported")
	}
	for _, path := range []string{"10:20/10:30", "11:20", "10:20/11:evil", "10:20/11:30/12:40/13:50/14:60/15:70/16:80/17:90/18:100"} {
		if _, err := nativeTreePath(path, "10"); err == nil {
			t.Fatalf("invalid source tree token accepted %s", path)
		}
	}
}

func TestNativeAdvancedRulesAddressLinkedChildWithoutFlattening(t *testing.T) {
	data := nativeTreeFixture(t)
	options := data["config_groups"].([]any)[0].(map[string]any)["options"].([]any)
	row := options[0].(map[string]any)["sub"].([]any)[0].(map[string]any)["pricings"]
	options = append(options, map[string]any{"id": 12, "option_name": "Plan", "option_type": 1, "sub": []any{map[string]any{"id": 40, "option_name": "Restricted", "pricings": row}, map[string]any{"id": 41, "option_name": "Flexible", "pricings": row}}})
	data["config_groups"].([]any)[0].(map[string]any)["options"] = options
	data["advanced"] = []any{map[string]any{"id": 1, "config_id": 12, "type": "condition", "relation": "seq", "relation_id": 0, "sub_id": map[string]any{"40": map[string]any{}}}, map[string]any{"id": 2, "config_id": 11, "type": "result", "relation": "seq", "relation_id": 1, "sub_id": map[string]any{"30": map[string]any{}}}}
	project := func() *channelcommercecap.ImportSnapshot {
		t.Helper()
		detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
		if err != nil || detail.ImportSnapshot == nil {
			t.Fatalf("linked advanced source rejected %+v %v", detail, err)
		}
		return detail.ImportSnapshot
	}
	snapshot := project()
	values := []channelcommercecap.PurchaseValue{{FieldToken: "config:10", Type: "tree_select", TreeOptionTokens: []string{"10:21/11:31"}}, {FieldToken: "config:12", Type: "single_select", OptionTokens: []string{"40"}}}
	evaluated, err := evaluateNativeSelections(snapshot, values)
	if err != nil || evaluated.Valid {
		t.Fatalf("linked child restriction disappeared %+v %v", evaluated, err)
	}
	values[1].OptionTokens = []string{"41"}
	evaluated, err = evaluateNativeSelections(snapshot, values)
	if err != nil || !evaluated.Valid {
		t.Fatalf("inactive linked rule blocked valid leaf %+v %v", evaluated, err)
	}
	// The original JS requires the condition's DOM field to exist. An absent
	// child level therefore never activates its sneq condition on another field.
	options[1].(map[string]any)["sub"] = options[1].(map[string]any)["sub"].([]any)[:1]
	data["advanced"] = []any{map[string]any{"id": 1, "config_id": 11, "type": "condition", "relation": "sneq", "relation_id": 0, "sub_id": map[string]any{"30": map[string]any{}}}, map[string]any{"id": 2, "config_id": 12, "type": "result", "relation": "seq", "relation_id": 1, "sub_id": map[string]any{"40": map[string]any{}}}}
	snapshot = project()
	values[0].TreeOptionTokens = []string{"10:21"}
	evaluated, err = evaluateNativeSelections(snapshot, values)
	if err != nil || !evaluated.Valid {
		t.Fatalf("absent source child activated sneq %+v %v", evaluated, err)
	}
}
