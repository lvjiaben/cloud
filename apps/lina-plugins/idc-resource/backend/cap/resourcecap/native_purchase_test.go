// This file verifies generic native purchase mapping, bounded detached output,
// and refusal of unversioned or hardware-shaped accidental inputs.
package resourcecap

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func TestNativePurchasePlanAndMapperBoundaries(t *testing.T) {
	plan := NativePurchasePlan{Protocol: productfulfillmentcap.SaleImportProtocolNativeFinanceV1, Version: NativePurchasePlanVersion, Purchase: channelcommercecap.Purchase{PublicSlug: "41", ConfigurationRevision: strings.Repeat("a", 64), BillingTermToken: "monthly", CurrencyCode: "CNY", PurchaseCycles: 1, Quantity: 1}}
	output := json.RawMessage(`{"pid":"41","configoption":{}}`)
	result, err := BuildNativePurchaseParameters(t.Context(), func(context.Context, NativePurchasePlan) (json.RawMessage, error) { return output, nil }, plan)
	if err != nil {
		t.Fatal(err)
	}
	output[2] = 'x'
	if string(result) != `{"pid":"41","configoption":{}}` {
		t.Fatal("mapper result aliases guest memory")
	}
	for _, scenario := range []string{"protocol", "version", "quantity", "cycles", "currency", "term", "non-object output", "mapper error"} {
		t.Run(scenario, func(t *testing.T) {
			current := plan
			switch scenario {
			case "protocol":
				current.Protocol = ""
			case "version":
				current.Version = ""
			case "quantity":
				current.Purchase.Quantity = 2
			case "cycles":
				current.Purchase.PurchaseCycles = 2
			case "currency":
				current.Purchase.CurrencyCode = ""
			case "term":
				current.Purchase.BillingTermToken = ""
			}
			mapper := func(context.Context, NativePurchasePlan) (json.RawMessage, error) {
				if scenario == "mapper error" {
					return nil, gerror.New("private connector detail")
				}
				if scenario == "non-object output" {
					return json.RawMessage(`[]`), nil
				}
				return json.RawMessage(`{}`), nil
			}
			_, err := BuildNativePurchaseParameters(t.Context(), mapper, current)
			if err == nil || strings.Contains(err.Error(), "private connector detail") {
				t.Fatalf("native plan boundary failed: %v", err)
			}
		})
	}
}
