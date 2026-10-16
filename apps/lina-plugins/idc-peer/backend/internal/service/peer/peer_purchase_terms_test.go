// This file verifies interval preservation and refusing ambiguous upstream
// terms before any money-moving quote/order operation is attempted.
package peer

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

const testFreshTerm = `{"billingTermToken":"fresh","chargeType":"recurring","periodUnit":"month","periodCount":3,"minimumPurchaseCycles":1,"status":"final","setupAmount":"0","subtotalAmount":"100","taxAmount":"0","totalAmount":"100"}`

func TestPurchaseForScheduleRefreshesTokenAndPreservesQuantity(t *testing.T) {
	e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"configurationRevision":"revision","currencyCode":"CNY","terms":[` + testFreshTerm + `]}}`}, {response: `{"code":0,"data":{"quoteNo":"quote","currency":"CNY","totalAmountMinor":600}}`}, {response: `{"code":0,"data":{"id":19}}`}}}
	purchase := testPurchase()
	purchase.Quantity = 3
	purchase.PurchaseCycles = 2
	billing := &productfulfillmentcap.BillingSchedule{CurrencyCode: "CNY", ChargeType: productfulfillmentcap.BillingChargeRecurring, PeriodUnit: productfulfillmentcap.BillingPeriodMonth, PeriodCount: 3, PurchaseCycles: 2}
	result, err := PurchaseForSchedule(e, "confidential-context", "intent", purchase, billing)
	if err != nil || result.Status != productfulfillmentcap.StatusAccepted || len(e.requests) != 3 {
		t.Fatalf("schedule purchase: %+v %v", result, err)
	}
	if e.requests[0].Method != "GET" || len(e.requests[0].JSONBody) != 0 || e.requests[0].Query["values"][0] != "[]" {
		t.Fatal("billing term read did not use explicit GET query JSON")
	}
	var quoted struct {
		Purchase channelcommercecap.Purchase `json:"purchase"`
	}
	if json.Unmarshal(e.requests[1].JSONBody, &quoted) != nil || quoted.Purchase.BillingTermToken != "fresh" || quoted.Purchase.Quantity != 3 || quoted.Purchase.PurchaseCycles != 2 {
		t.Fatalf("frozen purchase changed: %+v", quoted)
	}
}

func TestPurchaseForScheduleRejectsMissingAndAmbiguousIntervalsBeforeQuote(t *testing.T) {
	for _, terms := range []string{strings.ReplaceAll(testFreshTerm, `"periodCount":3`, `"periodCount":1`), testFreshTerm + "," + strings.ReplaceAll(testFreshTerm, `"fresh"`, `"other"`)} {
		e := &scriptedExecutor{steps: []step{{response: `{"code":0,"data":{"configurationRevision":"revision","currencyCode":"CNY","terms":[` + terms + `]}}`}}}
		billing := &productfulfillmentcap.BillingSchedule{CurrencyCode: "CNY", ChargeType: productfulfillmentcap.BillingChargeRecurring, PeriodUnit: productfulfillmentcap.BillingPeriodMonth, PeriodCount: 3, PurchaseCycles: 1}
		if result, err := PurchaseForSchedule(e, "confidential-context", "intent", testPurchase(), billing); err == nil || result != nil || len(e.requests) != 1 {
			t.Fatal("ambiguous or changed period reached quote")
		}
	}
}
