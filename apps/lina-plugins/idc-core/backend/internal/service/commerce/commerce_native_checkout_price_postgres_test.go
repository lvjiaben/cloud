// Native checkout preflight fences changed supplier evidence before any local
// order or invoice write, while an accepted order remains an immutable replay.
package commerce

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"strings"
	"testing"
)

type nativeCheckoutVerifierStub struct {
	catalog.PurchaseQuoteAssembler
	charge  catalog.PurchaseQuoteCharge
	changed bool
	calls   int
}

func (s *nativeCheckoutVerifierStub) VerifyNativeCheckoutPrices(_ context.Context, input catalog.PurchaseQuoteInput) (map[string]catalog.PurchaseQuoteCharge, error) {
	s.calls++
	result := map[string]catalog.PurchaseQuoteCharge{}
	for _, line := range input.Lines {
		charge := s.charge
		if s.changed {
			charge.PriceSourceDigest = strings.Repeat("0", 64)
		}
		result[line.ReferenceNo] = charge
	}
	return result, nil
}
func TestNativeCheckoutRejectsChangedSourceBeforePaymentPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	var charge catalog.PurchaseQuoteCharge
	if err := scenario.database.GetScan(scenario.ctx, &charge, `SELECT charge_amount_minor,setup_fee_minor,priced_amount_minor,price_source_digest FROM plugin_idc_core_commerce_quote_charge_snapshot charge JOIN plugin_idc_core_commerce_quote_item_snapshot item ON item.quote_item_no=charge.quote_item_no WHERE item.quote_no=?`, scenario.quote.QuoteNo); err != nil {
		t.Fatal(err)
	}
	verifier := &nativeCheckoutVerifierStub{PurchaseQuoteAssembler: scenario.service.quoteCatalog, charge: charge, changed: true}
	scenario.service.quoteCatalog = verifier
	request := CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "native-price-recheck-checkout-1"}
	if _, err := scenario.service.CheckoutAccountQuote(scenario.ctx, request); !bizerr.Is(err, CodeQuoteNotAvailable) {
		t.Fatalf("changed native source admitted: %v", err)
	}
	for _, table := range []string{"plugin_idc_core_commerce_order", "plugin_idc_core_billing_invoice"} {
		count, err := scenario.database.GetCount(scenario.ctx, "SELECT 1 FROM "+table)
		if err != nil || count != 0 {
			t.Fatalf("failed price preflight wrote %s count %d err %v", table, count, err)
		}
	}
	verifier.changed = false
	accepted, err := scenario.service.CheckoutAccountQuote(scenario.ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	calls := verifier.calls
	verifier.changed = true
	replay, err := scenario.service.CheckoutAccountQuote(scenario.ctx, request)
	if err != nil || replay.OrderNo != accepted.OrderNo || !replay.Reused || verifier.calls != calls {
		t.Fatalf("historical order replay touched source: %+v calls=%d err=%v", replay, verifier.calls, err)
	}
}
