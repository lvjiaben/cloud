package admin

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func TestMarketingMissingCostAPIUsesGenericProductDimensions(t *testing.T) {
	now := time.Unix(1, 0).UTC()
	item := toAPIMarketingMissingCost(&marketing.MarketingMissingCostItem{
		OrderItemNo: "order-item-public", OrderNo: "order-public", VersionNo: "version-public",
		CurrencyCode: "CNY", PeriodUnitCode: "once", PeriodCount: 1, Quantity: 2,
		PurchaseCycles: 1, ProductNameZhCN: "手机", ProductNameEnUS: "Phone",
		StatusCode: "missing", Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if item == nil || item.VersionNo != "version-public" || item.Quantity != 2 {
		t.Fatalf("generic missing-cost projection=%+v", item)
	}
	payload, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacyField := range []string{"bindingNo", "serverQuantity"} {
		if strings.Contains(string(payload), legacyField) {
			t.Fatalf("generic missing-cost response leaked %q: %s", legacyField, payload)
		}
	}
}
