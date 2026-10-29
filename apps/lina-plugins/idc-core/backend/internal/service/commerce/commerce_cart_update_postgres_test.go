// This file verifies owned cart updates, quote invalidation and concurrent writes
// against isolated PostgreSQL schemas; no application customer data is changed.
package commerce

import (
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"sync"
	"testing"
)

func TestCartUpdateQuantityAndConfigurationPostgreSQL(t *testing.T) {
	s := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	cart, err := s.service.GetAccountCart(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	item := cart.Items[0]
	input := UpdateItemInput{CartItemNo: item.CartItemNo, ExpectedCartVersion: cart.Version, ExpectedItemVersion: item.Version, Quantity: 2}
	updated, err := s.service.UpdateAccountItem(s.ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != cart.Version+1 || updated.Items[0].Version != item.Version+1 || updated.Items[0].Quantity != 2 || updated.LatestQuote != nil || updated.Items[0].ConfigurationReference != item.ConfigurationReference {
		t.Fatalf("update=%+v", updated)
	}
	if updated.Items[0].PeriodUnitCode != "month" || updated.Items[0].PeriodCount != 1 {
		t.Fatalf("billing period not projected: %+v", updated.Items[0])
	}
	if _, err = s.service.CheckoutAccountQuote(s.ctx, CheckoutRequest{QuoteNo: s.quote.QuoteNo, IdempotencyKey: "cart-update-old-quote-001"}); err == nil {
		t.Fatal("invalidated quote checked out")
	}
	if _, err = s.service.UpdateAccountItem(s.ctx, input); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale versions accepted: %v", err)
	}
	input.ExpectedCartVersion, input.ExpectedItemVersion = updated.Version, updated.Items[0].Version
	unchanged, err := s.service.UpdateAccountItem(s.ctx, input)
	if err != nil || unchanged.Version != updated.Version {
		t.Fatalf("same value update=%+v err=%v", unchanged, err)
	}
	quoted, err := s.service.QuoteAccountCart(s.ctx, QuoteRequest{ExpectedCartVersion: updated.Version})
	if err != nil {
		t.Fatal(err)
	}
	if quoted.TotalAmountMinor != s.quote.TotalAmountMinor*2 {
		t.Fatalf("quantity not repriced: before=%d after=%d", s.quote.TotalAmountMinor, quoted.TotalAmountMinor)
	}
	input.ExpectedCartVersion = quoted.CartVersion
	current, err := s.service.GetAccountCart(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	input.ExpectedCartVersion, input.ExpectedItemVersion = current.Version, current.Items[0].Version
	input.Purchase = &s.purchase
	replaced, err := s.service.UpdateAccountItem(s.ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	if replaced.Items[0].CartItemNo != item.CartItemNo || replaced.Items[0].ConfigurationReference == item.ConfigurationReference || replaced.LatestQuote != nil {
		t.Fatalf("replacement=%+v", replaced)
	}
	for _, quantity := range []int{0, -1, 1001} {
		input.Quantity = quantity
		if _, err = s.service.UpdateAccountItem(s.ctx, input); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("quantity %d accepted: %v", quantity, err)
		}
	}
	foreign := newCheckoutCustomerOnDatabase(t, s, "customer-commerce-service-2", 202, 1)
	foreignCart, err := foreign.service.GetAccountCart(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = foreign.service.UpdateAccountItem(s.ctx, UpdateItemInput{CartItemNo: item.CartItemNo, ExpectedCartVersion: foreignCart.Version, ExpectedItemVersion: replaced.Items[0].Version, Quantity: 3}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("foreign item accepted: %v", err)
	}
}

func TestCartUpdateConcurrentAndRollbackPostgreSQL(t *testing.T) {
	s := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	cart, err := s.service.GetAccountCart(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	input := UpdateItemInput{CartItemNo: cart.Items[0].CartItemNo, ExpectedCartVersion: cart.Version, ExpectedItemVersion: cart.Items[0].Version, Quantity: 3}
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.service.UpdateAccountItem(s.ctx, input); errors <- e }()
	}
	wg.Wait()
	close(errors)
	successes, conflicts := 0, 0
	for e := range errors {
		if e == nil {
			successes++
		} else if bizerr.Is(e, CodeConflict) {
			conflicts++
		} else {
			t.Fatal(e)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("success=%d conflict=%d", successes, conflicts)
	}
	cart, err = s.service.GetAccountCart(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.database.Exec(s.ctx, `CREATE FUNCTION reject_cart_version() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected rollback'; END $$; CREATE TRIGGER reject_update BEFORE UPDATE ON plugin_idc_core_commerce_cart FOR EACH ROW EXECUTE FUNCTION reject_cart_version()`); err != nil {
		t.Fatal(err)
	}
	input.ExpectedCartVersion, input.ExpectedItemVersion, input.Quantity = cart.Version, cart.Items[0].Version, 4
	if _, err = s.service.UpdateAccountItem(s.ctx, input); err == nil {
		t.Fatal("injected failure succeeded")
	}
	actual, err := s.service.GetAccountCart(s.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if actual.Version != cart.Version || actual.Items[0].Quantity != 3 || actual.Items[0].Version != cart.Items[0].Version {
		t.Fatalf("partial write: %+v", actual)
	}
}

// TestCartConfigurationProjectionPostgreSQL proves displayed choices are stored
// server facts and remain available without any browser session draft.
func TestCartConfigurationProjectionPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	prepareStructuredCheckoutDatabase(t, ctx, database)
	if _, err := database.Exec(ctx, `ALTER TABLE plugin_idc_core_catalog_product_version_configuration DISABLE TRIGGER trg_idc_guard_product_version_configuration;
UPDATE plugin_idc_core_catalog_product_version_configuration SET customer_declaration_snapshot='{"version":"v1","fields":[{"key":"cpu","type":"integer","required":true,"minimum":"1","maximum":"8","label":{"zhCN":"CPU核数","enUS":"CPU cores"}},{"key":"plan","type":"single_select","required":true,"label":{"zhCN":"服务模式","enUS":"Service plan"},"staticCandidates":[{"value":"quantity","label":{"zhCN":"按量服务","enUS":"Metered service"}}]}]}'::jsonb WHERE version_no='version-commerce-service-1';
ALTER TABLE plugin_idc_core_catalog_product_version_configuration ENABLE TRIGGER trg_idc_guard_product_version_configuration;
INSERT INTO plugin_idc_core_catalog_configuration_price_dimension (dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code,status_code) VALUES ('cpd_11111111111111111111111111111111','version-commerce-service-1',1,'cpu','quantity','CPU核数','CPU cores','核数','Cores','per_unit','active'),('cpd_22222222222222222222222222222222','version-commerce-service-1',1,'plan','quantity','服务模式','Service plan','按量服务','Metered service','fixed','active');
INSERT INTO plugin_idc_core_catalog_price (price_no,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,minimum_quantity,maximum_quantity,amount_minor,setup_fee_minor,renewal_amount_minor,status_code) VALUES ('prc_11111111111111111111111111111111','version-commerce-service-1','billing-commerce-service-month','cpd_11111111111111111111111111111111','CNY',0,NULL,10,0,10,'active');
INSERT INTO plugin_idc_core_catalog_price (price_no,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,amount_minor,setup_fee_minor,renewal_amount_minor,status_code) VALUES ('prc_22222222222222222222222222222222','version-commerce-service-1','billing-commerce-service-month','cpd_22222222222222222222222222222222','CNY',50,0,50,'active');`); err != nil {
		t.Fatal(err)
	}
	reader, err := catalog.NewPublicCatalogReaderConfigured(database, []byte("0123456789abcdef0123456789abcdef"), checkoutPurchaseRuntimeStub{}, checkoutPurchaseFileVerifierStub{})
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, reader)
	configuration, err := reader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil {
		t.Fatal(err)
	}
	cpu := "4"
	values := []catalog.PublicPurchaseConfigurationValue{{FieldToken: configuration.Fields[0].FieldToken, Type: "integer", IntegerValue: &cpu}, {FieldToken: configuration.Fields[1].FieldToken, Type: "single_select", OptionTokens: []string{configuration.Fields[1].Options[0].OptionToken}}}
	public := catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision, Values: values}
	quoted, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{PublicPurchaseConfigurationInput: public, CurrencyCode: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	purchase := catalog.PurchaseConfigurationInput{PublicSlug: public.PublicSlug, ConfigurationRevision: public.ConfigurationRevision, Values: values, CurrencyCode: "CNY", BillingTermToken: quoted.Terms[0].BillingTermToken, PurchaseCycles: 1}
	service, _ := newCheckoutCustomer(t, ctx, database, resolver, purchase, "customer-commerce-service-1", 101, 1, false)
	cart, err := service.GetAccountCart(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(cart.Items) != 1 || len(cart.Items[0].Configuration) != 2 {
		t.Fatalf("projection=%+v", cart)
	}
	field := cart.Items[0].Configuration[0]
	if field.LabelZhCN != "CPU核数" || field.LabelEnUS != "CPU cores" || field.ValueZhCN != "4" || field.ValueEnUS != "4" {
		t.Fatalf("stored choice=%+v", field)
	}
	fixed := cart.Items[0].Configuration[1]
	if fixed.LabelZhCN != "服务模式" || fixed.ValueZhCN != "按量服务" || fixed.ValueEnUS != "Metered service" {
		t.Fatalf("literal quantity option must keep its labels: %+v", fixed)
	}
}
