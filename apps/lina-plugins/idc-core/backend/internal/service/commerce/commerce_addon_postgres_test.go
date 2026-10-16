// This file verifies add-on target ownership, immutable checkout binding,
// payment-time expiry admission and manual delivery with real isolated SQL.
package commerce

import (
	"lina-core/pkg/bizerr"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	productfulfillmentservice "lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"strings"
	"testing"
)

func TestModernAddonQuoteCheckoutAndManualDeliveryPostgreSQL(t *testing.T) {
	prepared := prepareGenericWorkerScenario(t)
	scenario := prepared.checkout
	ctx, db := scenario.ctx, scenario.database
	// The external provider boundary is represented by an activated entitlement;
	// no supplier is contacted and no production database guards are disabled.
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='processing',version=version+1 WHERE order_no=?;`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active' WHERE order_no=?`, prepared.order.OrderNo); err != nil {
		t.Fatal(err)
	}
	target, err := db.GetValue(ctx, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement WHERE order_no=?`, prepared.order.OrderNo)
	if err != nil || target.IsEmpty() {
		t.Fatalf("target=%v err=%v", target, err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_product_addon(tenant_id,product_no,addon_product_no) VALUES(0,'product-commerce-service','prd_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')`); err != nil {
		t.Fatal(err)
	}
	targets, err := scenario.service.ListAddonTargets(ctx)
	if err != nil || len(targets) != 1 || len(targets[0].Addons) != 1 {
		t.Fatalf("targets=%+v err=%v", targets, err)
	}
	other := newCheckoutCustomerOnDatabase(t, scenario, "customer-commerce-service-2", 102, 1)
	foreign, err := other.service.ListAddonTargets(ctx)
	if err != nil || len(foreign) != 0 {
		t.Fatalf("foreign targets=%+v err=%v", foreign, err)
	}
	if _, err = other.service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: other.quote.CartVersion, TargetEntitlementNo: target.String()}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign target admitted: %v", err)
	}
	publicReader, err := catalog.NewPublicCatalogReaderConfigured(db, []byte("0123456789abcdef0123456789abcdef"), checkoutPurchaseRuntimeStub{}, checkoutPurchaseFileVerifierStub{})
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, publicReader)
	configuration, err := publicReader.GetPurchaseConfiguration(ctx, "generic-phone")
	if err != nil {
		t.Fatal(err)
	}
	terms, err := publicReader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "generic-phone", ConfigurationRevision: configuration.ConfigurationRevision}, CurrencyCode: "CNY"})
	if err != nil || len(terms.Terms) != 1 {
		t.Fatalf("terms=%+v err=%v", terms, err)
	}
	purchase := catalog.PurchaseConfigurationInput{PublicSlug: "generic-phone", ConfigurationRevision: configuration.ConfigurationRevision, BillingTermToken: terms.Terms[0].BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1}
	addonService, ordinary := newCheckoutCustomer(t, ctx, db, scenario.resolver, purchase, "customer-commerce-service-1", 101, 1, false)
	quote, err := addonService.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: ordinary.CartVersion, TargetEntitlementNo: target.String()})
	if err != nil {
		t.Fatal(err)
	}
	if quote.TotalAmountMinor != ordinary.TotalAmountMinor {
		t.Fatalf("add-on unexpectedly prorated/changed independent price: addon=%d ordinary=%d", quote.TotalAmountMinor, ordinary.TotalAmountMinor)
	}
	checkout, err := addonService.CheckoutAccountQuote(ctx, CheckoutRequest{QuoteNo: quote.QuoteNo, IdempotencyKey: "modern-addon-checkout-0001"})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := addonService.CheckoutAccountQuote(ctx, CheckoutRequest{QuoteNo: quote.QuoteNo, IdempotencyKey: "modern-addon-checkout-0001"})
	if err != nil || replay.OrderNo != checkout.OrderNo || !replay.Reused {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	balance, err := billingservice.NewBalanceService(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(ctx, billingservice.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor, SourceReference: "modern-addon-recharge", SourceSnapshotDigest: strings.Repeat("a", 64)}); err != nil {
		t.Fatal(err)
	}
	invoices, err := billingservice.NewInvoiceService(db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='suspended' WHERE entitlement_no=?`, target.String()); err != nil {
		t.Fatal(err)
	}
	payment := billingservice.InvoiceBalancePaymentInput{InvoiceNo: checkout.InvoiceNo, SourceReference: "modern-addon-payment", SourceSnapshotDigest: strings.Repeat("b", 64)}
	if _, err = invoices.PayFromBalance(ctx, payment); err == nil {
		t.Fatal("suspended target accepted payment")
	}
	unpaid, err := db.GetValue(ctx, `SELECT paid_amount_minor FROM plugin_idc_core_billing_invoice WHERE invoice_no=?`, checkout.InvoiceNo)
	if err != nil || unpaid.Int64() != 0 {
		t.Fatalf("failed payment changed money: %v err=%v", unpaid, err)
	}
	if _, err = db.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active' WHERE entitlement_no=?`, target.String()); err != nil {
		t.Fatal(err)
	}
	if _, err = invoices.PayFromBalance(ctx, payment); err != nil {
		t.Fatal(err)
	}
	settlement, err := newFinancialSettlementTestService(t, scenario).SettleOrder(ctx, checkout.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	output, err := prepared.service.ConsumeFinanciallySettled(ctx, productfulfillmentservice.SettlementInput{SettlementNo: settlement.SettlementNo, OrderNo: checkout.OrderNo})
	if err != nil || output.ManualCount != 1 || output.TaskCount != 0 {
		t.Fatalf("materialize=%+v err=%v", output, err)
	}
	var intent struct{ IntentNo, StatusCode, StrategyCode string }
	if err = db.GetScan(ctx, &intent, `SELECT intent_no,status_code,strategy_code FROM plugin_idc_core_product_fulfillment_intent WHERE order_no=?`, checkout.OrderNo); err != nil || intent.StatusCode != "manual_pending" || intent.StrategyCode != "manual" {
		t.Fatalf("add-on invoked ordinary provision: %+v err=%v", intent, err)
	}
	detail, err := prepared.service.Detail(ctx, intent.IntentNo)
	if err != nil || detail.TargetEntitlementNo != target.String() || !detail.CanManualConclude {
		t.Fatalf("manual target detail=%+v err=%v", detail, err)
	}
	tasks, err := db.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_product_fulfillment_task WHERE intent_no=?`, intent.IntentNo)
	if err != nil || tasks.Int() != 0 {
		t.Fatalf("unexpected automatic resource task count=%v err=%v", tasks, err)
	}
}
