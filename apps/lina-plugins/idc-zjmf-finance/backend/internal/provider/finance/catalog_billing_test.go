package finance

import (
	"encoding/json"
	"testing"

	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
)

func TestCatalogNativeDurationAndAccountPricing(t *testing.T) {
	for _, scenario := range []string{"six hours", "native default hours", "three days", "missing duration", "discount", "missing discount evidence", "resource price", "resale price"} {
		t.Run(scenario, func(t *testing.T) {
			data := catalogDetailFixture(t)
			data["config_groups"] = []any{}
			product := data["products"].(map[string]any)
			row := data["product_pricings"].([]any)[0].(map[string]any)
			wantSnapshot := true
			wantCount := 1
			switch scenario {
			case "six hours":
				product["pay_type"] = `{"pay_type":"recurring","pay_hour_cycle":"6"}`
				row["monthly"], row["hour"] = "-1", "15.00"
				wantCount = 6
			case "native default hours":
				product["pay_type"] = `{"pay_type":"recurring","pay_hour_cycle":"720"}`
				row["monthly"], row["hour"] = "-1", "720.00"
				wantCount = 720
			case "three days":
				product["pay_type"] = `{"pay_type":"recurring","pay_day_cycle":3}`
				row["monthly"], row["day"] = "-1", "20.00"
				wantCount = 3
			case "missing duration":
				row["monthly"], row["hour"] = "-1", "15.00"
				wantSnapshot = false
			case "discount":
				data["flag"] = map[string]any{"type": 1, "bates": 85}
			case "missing discount evidence":
				delete(data, "flag")
				wantSnapshot = false
			case "resource price":
				product["api_type"] = "resource"
				wantSnapshot = false
			case "resale price":
				product["api_type"], product["upstream_price_type"] = "zjmf_api", "percent"
				wantSnapshot = false
			}
			detail, err := projectCatalogDetail(mustCatalogJSON(t, map[string]any{"status": 200, "data": data}), resourceinterfaceprojectioncap.CatalogProductQuery{Reference: "41"})
			if err != nil || detail.PriceScheduleComplete || (detail.ImportSnapshot != nil) != wantSnapshot || len(detail.BasePrices) > 0 {
				t.Fatalf("wrong authoritative schedule: %+v %v", detail, err)
			}
			if wantSnapshot {
				assertNativeRemoteSchedule(t, detail)
				if detail.ImportSnapshot.Prices[0].PeriodCount != wantCount {
					t.Fatalf("native duration changed %+v", detail.ImportSnapshot.Prices)
				}
			}

		})
	}
	for _, raw := range []string{`0`, `-1`, `87601`, `"1.5"`, `null`, `""`} {
		prices := []resourceinterfaceprojectioncap.CatalogPrice{{CurrencyCode: "CNY", Period: resourceinterfaceprojectioncap.CatalogPeriodHour, PeriodCount: 1}}
		if projected, complete := catalogBillingPeriods(prices, json.RawMessage(raw), nil); complete || len(projected) != 0 {
			t.Fatalf("unrepresentable native duration %s was admitted", raw)
		}
	}
}
