// This file verifies quantity pricing through public configuration, frozen quotes,
// checkout and invoices, including zero quantities and historical price isolation.

package commerce

import (
	"strconv"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

// TestQuantityConfigurationCheckoutPostgreSQL exercises both charging algorithms
// against real isolated PostgreSQL schemas rather than a calculated-price stub.
func TestQuantityConfigurationCheckoutPostgreSQL(t *testing.T) {
	for _, mode := range []string{"per_unit", "band_total"} {
		for _, quantity := range []int{0, 100, 101} {
			t.Run(mode+"/"+strconv.Itoa(quantity), func(t *testing.T) {
				testQuantityConfigurationCheckout(t, mode, quantity)
			})
		}
	}
}

func testQuantityConfigurationCheckout(t *testing.T, mode string, quantity int) {
	t.Helper()
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	prepareStructuredCheckoutDatabase(t, ctx, database)
	if _, err := database.Exec(ctx, `
ALTER TABLE plugin_idc_core_catalog_product_version_configuration DISABLE TRIGGER trg_idc_guard_product_version_configuration;
UPDATE plugin_idc_core_catalog_product_version_configuration
SET customer_declaration_snapshot='{"version":"v1","fields":[{"key":"disk","type":"integer","required":true,"minimum":"0","maximum":"200","label":{"zhCN":"数据盘容量","enUS":"Data disk capacity"}}]}'::jsonb
WHERE version_no='version-commerce-service-1';
ALTER TABLE plugin_idc_core_catalog_product_version_configuration ENABLE TRIGGER trg_idc_guard_product_version_configuration;`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_configuration_price_dimension
(dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code)
VALUES ('cpd_11111111111111111111111111111111','version-commerce-service-1',1,'disk','quantity','数据盘容量','Data disk capacity','容量','Capacity',?,'active')`, mode); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_catalog_price
(price_no,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,minimum_quantity,maximum_quantity,amount_minor,setup_fee_minor,renewal_amount_minor,status_code)
VALUES
('prc_11111111111111111111111111111111','version-commerce-service-1','billing-commerce-service-month','cpd_11111111111111111111111111111111','CNY',0,100,20,30,10,'active'),
('prc_22222222222222222222222222222222','version-commerce-service-1','billing-commerce-service-month','cpd_11111111111111111111111111111111','CNY',100,NULL,15,40,8,'active'),
('prc_33333333333333333333333333333333','version-commerce-service-1','billing-commerce-service-month','cpd_11111111111111111111111111111111','USD',0,NULL,999999,99999,999999,'active')`); err != nil {
		t.Fatal(err)
	}
	reader, err := catalog.NewPublicCatalogReaderConfigured(database, []byte("0123456789abcdef0123456789abcdef"), checkoutPurchaseRuntimeStub{}, checkoutPurchaseFileVerifierStub{})
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, reader)
	configuration, err := reader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil || len(configuration.Fields) != 1 {
		t.Fatalf("configuration=%+v err=%v", configuration, err)
	}
	text := strconv.Itoa(quantity)
	values := []catalog.PublicPurchaseConfigurationValue{{FieldToken: configuration.Fields[0].FieldToken, Type: "integer", IntegerValue: &text}}
	publicPurchase := catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision, Values: values}
	quoted, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: publicPurchase, CurrencyCode: "CNY"})
	if err != nil || len(quoted.Terms) != 1 {
		t.Fatalf("public quote=%+v err=%v", quoted, err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	purchase := catalog.PurchaseConfigurationInput{PublicSlug: publicPurchase.PublicSlug, ConfigurationRevision: publicPurchase.ConfigurationRevision, Values: values, BillingTermToken: quoted.Terms[0].BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1}
	service, quote := newCheckoutCustomer(t, ctx, database, resolver, purchase, "customer-commerce-service-1", 101, 1, false)
	var charge, setup int64
	if quantity > 0 {
		charge, setup = 20, 30
		if quantity > 100 {
			charge, setup = 15, 40
		}
		if mode == "per_unit" {
			charge *= int64(quantity)
		}
	}
	// The seeded product has a 10000 base price and 6% tax, rounded half up.
	net := int64(10000) + charge + setup
	expected := net + (net*600+5000)/10000
	if len(quote.Items) != 1 || quote.TotalAmountMinor != expected {
		t.Fatalf("quantity=%d mode=%s quote=%+v expected=%d", quantity, mode, quote, expected)
	}
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	files, err := purchaseconfigurationfile.NewOrderFileConsumer(database, service.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(service, freezer, files); err != nil {
		t.Fatal(err)
	}
	if _, err = service.FreezeAccountQuoteItemConfiguration(ctx, FreezeQuoteItemConfigurationInput{QuoteItemNo: quote.Items[0].QuoteItemNo, Purchase: publicPurchase}); err != nil {
		t.Fatal(err)
	}
	checkout, err := service.CheckoutAccountQuote(ctx, CheckoutRequest{QuoteNo: quote.QuoteNo, IdempotencyKey: "checkout-quantity-configuration-0001"})
	if err != nil || checkout == nil || checkout.TotalAmountMinor != expected {
		t.Fatalf("checkout=%+v err=%v expected=%d", checkout, err, expected)
	}
	var frozen struct {
		OrderTotal, InvoiceTotal, ConfigurationCharge, ConfigurationSetup int64
		SelectedQuantity                                                  int
	}
	if err = database.GetScan(ctx, &frozen, `SELECT
o.total_amount_minor AS order_total, i.total_amount_minor AS invoice_total,
COALESCE((SELECT sum(c.charge_amount_minor) FROM plugin_idc_core_commerce_quote_charge_snapshot c WHERE c.quote_item_no=? AND c.configuration_price_dimension_no IS NOT NULL),0) AS configuration_charge,
COALESCE((SELECT sum(c.setup_fee_minor) FROM plugin_idc_core_commerce_quote_charge_snapshot c WHERE c.quote_item_no=? AND c.configuration_price_dimension_no IS NOT NULL),0) AS configuration_setup,
COALESCE((SELECT sum(s.quantity) FROM plugin_idc_core_commerce_configuration_price_selection s JOIN plugin_idc_core_commerce_quote_item_snapshot qi ON qi.configuration_snapshot_no=s.configuration_snapshot_no WHERE qi.quote_item_no=?),0) AS selected_quantity
FROM plugin_idc_core_commerce_order o JOIN plugin_idc_core_billing_invoice i ON i.order_no=o.order_no WHERE o.order_no=?`, quote.Items[0].QuoteItemNo, quote.Items[0].QuoteItemNo, quote.Items[0].QuoteItemNo, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	if frozen.OrderTotal != expected || frozen.InvoiceTotal != expected || frozen.ConfigurationCharge != charge || frozen.ConfigurationSetup != setup || frozen.SelectedQuantity != quantity {
		t.Fatalf("frozen=%+v expected total=%d charge=%d setup=%d quantity=%d", frozen, expected, charge, setup, quantity)
	}
	if quantity > 0 {
		assertConfigurationPricedQuoteRowsImmutable(t, ctx, database, quote.Items[0].QuoteItemNo)
	} else if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_commerce_quote_item_snapshot SET total_amount_minor=total_amount_minor+1 WHERE quote_item_no=?`, quote.Items[0].QuoteItemNo); err == nil {
		t.Fatal("zero-quantity quote item unexpectedly mutable")
	}
	// A later catalog change must affect new quotes while the existing order,
	// invoice and charge evidence retain their original settled amounts.
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_catalog_price
SET amount_minor=amount_minor+100,setup_fee_minor=setup_fee_minor+50
WHERE configuration_price_dimension_no='cpd_11111111111111111111111111111111' AND currency_code='CNY'`); err != nil {
		t.Fatal(err)
	}
	newQuote, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: publicPurchase, CurrencyCode: "CNY"})
	if err != nil || len(newQuote.Terms) != 1 {
		t.Fatalf("quote after price change=%+v err=%v", newQuote, err)
	}
	if quantity == 0 && newQuote.Terms[0].TotalAmount != quoted.Terms[0].TotalAmount {
		t.Fatal("zero quantity started charging after a catalog price change")
	}
	if quantity > 0 && newQuote.Terms[0].TotalAmount == quoted.Terms[0].TotalAmount {
		t.Fatal("new quantity quote did not adopt the changed catalog prices")
	}
	var historical struct{ OrderTotal, InvoiceTotal int64 }
	if err = database.GetScan(ctx, &historical, `SELECT o.total_amount_minor AS order_total,i.total_amount_minor AS invoice_total
FROM plugin_idc_core_commerce_order o JOIN plugin_idc_core_billing_invoice i ON i.order_no=o.order_no WHERE o.order_no=?`, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	if historical.OrderTotal != expected || historical.InvoiceTotal != expected {
		t.Fatalf("catalog price change rewrote history: %+v expected=%d", historical, expected)
	}
}
