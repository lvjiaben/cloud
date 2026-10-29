// This file checks source-derived advanced inclusion/exclusion and quantity
// relationships, including multiple active inclusion rules retaining a union.
package finance

import (
	"testing"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func TestNativeAdvancedRelationsPreserveUnionAndRejectDisallowedSelections(t *testing.T) {
	data := nativeRelationFixture(t, false)
	data["advanced"] = []any{
		nativeRelationFixtureRow(1, 10, 0, "condition", "seq", map[string]any{"20": map[string]any{}}),
		nativeRelationFixtureRow(2, 11, 1, "result", "seq", map[string]any{"30": map[string]any{}}),
		nativeRelationFixtureRow(3, 10, 0, "condition", "seq", map[string]any{"20": map[string]any{}}),
		nativeRelationFixtureRow(4, 11, 3, "result", "seq", map[string]any{"31": map[string]any{}}),
	}
	detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
	if err != nil || detail.ImportSnapshot == nil || len(detail.ImportSnapshot.Schema.Conditions) == 0 {
		t.Fatalf("advanced definition was discarded: %+v %v", detail, err)
	}
	for _, choice := range []string{"30", "31", "32"} {
		values := []channelcommercecap.PurchaseValue{{FieldToken: "config:10", Type: capmodel.ConfigurationFieldSingleSelect, OptionTokens: []string{"20"}}, {FieldToken: "config:11", Type: capmodel.ConfigurationFieldSingleSelect, OptionTokens: []string{choice}}}
		result, err := evaluateNativeSelections(detail.ImportSnapshot, values)
		if err != nil || result.Valid != (choice != "32") || len(result.Fields[1].Options) != 2 {
			t.Fatalf("active inclusion union changed for %s: %+v %v", choice, result, err)
		}
	}
}

func TestNativeAdvancedRelationsPreserveExclusionAndNumericRanges(t *testing.T) {
	for _, numeric := range []bool{false, true} {
		data := nativeRelationFixture(t, numeric)
		relation := "sneq"
		allowed := map[string]any{"30": map[string]any{}}
		if numeric {
			relation = "seq"
			allowed = map[string]any{"30": map[string]any{"qty_minimum": 2, "qty_maximum": 8}}
		}
		data["advanced"] = []any{nativeRelationFixtureRow(1, 10, 0, "condition", "seq", map[string]any{"20": map[string]any{}}), nativeRelationFixtureRow(2, 11, 1, "result", relation, allowed)}
		detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
		if err != nil || detail.ImportSnapshot == nil {
			t.Fatalf("range definition was discarded: %+v %v", detail, err)
		}
		for _, valid := range []bool{false, true} {
			target := channelcommercecap.PurchaseValue{FieldToken: "config:11", Type: capmodel.ConfigurationFieldSingleSelect, OptionTokens: []string{"30"}}
			if valid {
				target.OptionTokens = []string{"31"}
			}
			if numeric {
				value := "1"
				if valid {
					value = "8"
				}
				target = channelcommercecap.PurchaseValue{FieldToken: "config:11", Type: capmodel.ConfigurationFieldInteger, IntegerValue: &value}
			}
			result, err := evaluateNativeSelections(detail.ImportSnapshot, []channelcommercecap.PurchaseValue{{FieldToken: "config:10", Type: capmodel.ConfigurationFieldSingleSelect, OptionTokens: []string{"20"}}, target})
			if err != nil || result.Valid != valid {
				t.Fatalf("native relationship bypassed: numeric=%v valid=%v result=%+v err=%v", numeric, valid, result, err)
			}
		}
	}
}

func nativeRelationFixtureRow(id, field, parent int, kind, relation string, values map[string]any) map[string]any {
	return map[string]any{"id": id, "config_id": field, "relation_id": parent, "type": kind, "relation": relation, "sub_id": values}
}

func nativeRelationFixture(t *testing.T, numeric bool) map[string]any {
	t.Helper()
	data := catalogDetailFixture(t)
	group := data["config_groups"].([]any)[0].(map[string]any)
	base := group["options"].([]any)[0].(map[string]any)
	raw := base["sub"].([]any)[0].(map[string]any)
	choices := []any{map[string]any{"id": 30, "option_name": "第一档", "qty_minimum": 0, "qty_maximum": 10, "pricings": raw["pricings"]}, map[string]any{"id": 31, "option_name": "第二档", "pricings": raw["pricings"]}, map[string]any{"id": 32, "option_name": "第三档", "pricings": raw["pricings"]}}
	target := map[string]any{"id": 11, "option_name": "依赖配置", "option_type": 1, "sub": choices}
	if numeric {
		target["option_type"] = 4
		target["qty_minimum"], target["qty_maximum"] = 0, 10
		target["sub"] = choices[:1]
	}
	group["options"] = []any{base, target}
	return data
}
