package commerce

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

const commerceIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

type commercePrincipalStub struct {
	audience *principal.Audience
}

type commerceSettlementStub struct{}

func (commerceSettlementStub) SettleOrder(
	context.Context,
	string,
) (*billing.FinancialSettlement, error) {
	return nil, bizerr.NewCode(CodeUnavailable)
}

func (commerceSettlementStub) SettleOrderWithCredit(
	context.Context,
	billing.CreditOrderSettlementInput,
) (*billing.FinancialSettlement, error) {
	return nil, bizerr.NewCode(CodeUnavailable)
}

func (s commercePrincipalStub) Current(context.Context) (*principal.Audience, error) {
	return s.audience, nil
}

type legacyWriteCounts struct {
	Orders           int `orm:"orders"`
	OrderItems       int `orm:"order_items"`
	FulfillmentUnits int `orm:"fulfillment_units"`
	AsyncTasks       int `orm:"async_tasks"`
	Services         int `orm:"services"`
}

func commerceLegacyWriteCounts(t *testing.T, ctx context.Context, database gdb.DB) legacyWriteCounts {
	t.Helper()
	var counts legacyWriteCounts
	if err := database.GetScan(ctx, &counts, `SELECT
(SELECT count(*) FROM plugin_idc_core_commerce_order) AS orders,
(SELECT count(*) FROM plugin_idc_core_commerce_order_item) AS order_items,
(SELECT count(*) FROM plugin_idc_core_commerce_fulfillment_unit) AS fulfillment_units,
(SELECT count(*) FROM plugin_idc_core_async_task) AS async_tasks,
(SELECT count(*) FROM plugin_idc_core_service_instance) AS services`); err != nil {
		t.Fatalf("count legacy writes: %v", err)
	}
	return counts
}

func TestGenericCommerceShadowRemovalMigrationReplayPostgreSQL(t *testing.T) {
	_, _ = newCommerceServiceTestDatabase(t)
}

func TestNonServerStructuredConfigurationChangesRealPricePostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	prepareStructuredCheckoutDatabase(t, ctx, database)
	if _, err := database.Exec(ctx, `
ALTER TABLE plugin_idc_core_catalog_product_version_configuration DISABLE TRIGGER trg_idc_guard_product_version_configuration;
UPDATE plugin_idc_core_catalog_product_version_configuration
SET customer_declaration_snapshot='{"version":"v1","fields":[{"key":"certificate_type","type":"single_select","required":true,"label":{"zhCN":"证书类型","enUS":"Certificate type"},"staticCandidates":[{"value":"wildcard","label":{"zhCN":"通配符证书","enUS":"Wildcard certificate"}}]}]}'::jsonb
WHERE version_no='version-commerce-service-1';
ALTER TABLE plugin_idc_core_catalog_product_version_configuration ENABLE TRIGGER trg_idc_guard_product_version_configuration;
INSERT INTO plugin_idc_core_catalog_configuration_price_dimension
(tenant_id,dimension_no,version_no,configuration_version,field_key,candidate_value,field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,status_code)
VALUES (0,'cpd_11111111111111111111111111111111','version-commerce-service-1',1,'certificate_type','wildcard','证书类型','Certificate type','通配符证书','Wildcard certificate','active');
INSERT INTO plugin_idc_core_catalog_price
(tenant_id,price_no,version_no,billing_rule_no,configuration_price_dimension_no,currency_code,amount_minor,setup_fee_minor,renewal_amount_minor,status_code)
VALUES (0,'prc_11111111111111111111111111111111','version-commerce-service-1','billing-commerce-service-month','cpd_11111111111111111111111111111111','CNY',2500,0,2500,'active')`); err != nil {
		t.Fatalf("seed generic certificate price: %v", err)
	}
	reader, err := catalog.NewPublicCatalogReaderConfigured(database, []byte("0123456789abcdef0123456789abcdef"), checkoutPurchaseRuntimeStub{}, checkoutPurchaseFileVerifierStub{})
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, reader)
	configuration, err := reader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil || len(configuration.Fields) != 1 || len(configuration.Fields[0].Options) != 1 {
		t.Fatalf("certificate configuration=%+v err=%v", configuration, err)
	}
	values := []catalog.PublicPurchaseConfigurationValue{{
		FieldToken: configuration.Fields[0].FieldToken, Type: "single_select",
		OptionTokens: []string{configuration.Fields[0].Options[0].OptionToken},
	}}
	quoted, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision, Values: values},
		CurrencyCode:                     "CNY",
	})
	if err != nil || len(quoted.Terms) != 1 {
		t.Fatalf("certificate quote=%+v err=%v", quoted, err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := resolver.ResolvePurchaseConfiguration(ctx, catalog.PurchaseConfigurationInput{
		PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quoted.Terms[0].BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1, Values: values,
	})
	if err != nil || len(resolved.PriceSelections) != 1 || resolved.PriceSelections[0].DimensionNo != "cpd_11111111111111111111111111111111" {
		t.Fatalf("resolved certificate price=%+v err=%v", resolved, err)
	}
	assembler, err := catalog.PurchaseQuoteAssemblerFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	lines, err := assembler.AssemblePurchaseQuote(ctx, catalog.PurchaseQuoteInput{CurrencyCode: "CNY", Lines: []catalog.PurchaseQuoteLineInput{{
		ReferenceNo: "certificate-order", ProductNo: resolved.ProductNo, VersionNo: resolved.VersionNo,
		BillingRuleNo: resolved.BillingRuleNo, Quantity: 1, PurchaseCycles: 1, PriceSelections: resolved.PriceSelections,
	}}})
	if err != nil || len(lines) != 1 || len(lines[0].Charges) != 2 ||
		lines[0].Charges[1].ConfigurationPriceDimensionNo != "cpd_11111111111111111111111111111111" ||
		lines[0].Charges[1].ChargeAmountMinor != 2500 {
		t.Fatalf("real certificate price lines=%+v err=%v", lines, err)
	}
	expected, err := CalculatePricingAmounts(lines[0], 0)
	if err != nil {
		t.Fatal(err)
	}
	purchase := catalog.PurchaseConfigurationInput{
		PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quoted.Terms[0].BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1, Values: values,
	}
	service, finalQuote := newCheckoutCustomer(
		t, ctx, database, resolver, purchase, "customer-commerce-service-1", 101, 1, false,
	)
	if finalQuote.TotalAmountMinor != expected.TotalAmountMinor || len(finalQuote.Items) != 1 ||
		finalQuote.Items[0].TotalAmountMinor != expected.TotalAmountMinor {
		t.Fatalf("certificate final quote=%+v expected=%+v", finalQuote, expected)
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
	if _, err = service.FreezeAccountQuoteItemConfiguration(ctx, FreezeQuoteItemConfigurationInput{
		QuoteItemNo: finalQuote.Items[0].QuoteItemNo,
		Purchase: catalog.PublicPurchaseConfigurationInput{
			PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision, Values: values,
		},
	}); err != nil {
		t.Fatalf("freeze certificate configuration: %v", err)
	}
	checkout, err := service.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: finalQuote.QuoteNo, IdempotencyKey: "checkout-certificate-price-0001",
	})
	if err != nil || checkout == nil || checkout.TotalAmountMinor != expected.TotalAmountMinor {
		t.Fatalf("certificate checkout=%+v err=%v expected=%+v", checkout, err, expected)
	}
	var amounts struct {
		QuoteTotal, QuoteItemSubtotal, QuoteItemDiscount, QuoteItemNet int64
		QuoteItemTax, QuoteItemTotal, QuoteChargeTotal                 int64
		OrderTotal, OrderItemTotal, InvoiceTotal, InvoiceItemTotal     int64
		DimensionCharges                                               int
	}
	if err = database.GetScan(ctx, &amounts, `SELECT
quote.total_amount_minor AS quote_total,
quote_item.subtotal_minor AS quote_item_subtotal,
quote_item.discount_minor AS quote_item_discount,
quote_item.net_amount_minor AS quote_item_net,
quote_item.tax_amount_minor AS quote_item_tax,
quote_item.total_amount_minor AS quote_item_total,
(SELECT sum(charge.charge_amount_minor)
   FROM plugin_idc_core_commerce_quote_charge_snapshot charge
  WHERE charge.quote_item_no=quote_item.quote_item_no) AS quote_charge_total,
orders.total_amount_minor AS order_total,
order_item.total_amount_minor AS order_item_total,
invoice.total_amount_minor AS invoice_total,
invoice_item.total_amount_minor AS invoice_item_total,
(SELECT count(*) FROM plugin_idc_core_commerce_quote_charge_snapshot charge
 WHERE charge.quote_item_no=quote_item.quote_item_no
   AND charge.configuration_price_dimension_no='cpd_11111111111111111111111111111111') AS dimension_charges
FROM plugin_idc_core_commerce_quote_snapshot quote
JOIN plugin_idc_core_commerce_quote_item_snapshot quote_item ON quote_item.quote_no=quote.quote_no
JOIN plugin_idc_core_commerce_order orders ON orders.quote_no=quote.quote_no
JOIN plugin_idc_core_commerce_order_item order_item ON order_item.order_no=orders.order_no
JOIN plugin_idc_core_billing_invoice invoice ON invoice.order_no=orders.order_no
JOIN plugin_idc_core_billing_invoice_item invoice_item ON invoice_item.invoice_no=invoice.invoice_no
WHERE quote.quote_no=?`, finalQuote.QuoteNo); err != nil {
		t.Fatal(err)
	}
	if amounts.QuoteTotal != expected.TotalAmountMinor || amounts.OrderTotal != expected.TotalAmountMinor ||
		amounts.QuoteItemTotal != expected.TotalAmountMinor || amounts.QuoteChargeTotal != amounts.QuoteItemSubtotal ||
		amounts.QuoteItemSubtotal-amounts.QuoteItemDiscount != amounts.QuoteItemNet ||
		amounts.QuoteItemNet+amounts.QuoteItemTax != amounts.QuoteItemTotal ||
		amounts.OrderItemTotal != expected.TotalAmountMinor || amounts.InvoiceTotal != expected.TotalAmountMinor ||
		amounts.InvoiceItemTotal != expected.TotalAmountMinor || amounts.DimensionCharges != 1 {
		t.Fatalf("certificate frozen transaction amounts=%+v expected=%d", amounts, expected.TotalAmountMinor)
	}
	assertConfigurationPricedQuoteRowsImmutable(t, ctx, database, finalQuote.Items[0].QuoteItemNo)
}

func assertConfigurationPricedQuoteRowsImmutable(t *testing.T, ctx context.Context, database gdb.DB, quoteItemNo string) {
	t.Helper()
	statements := map[string]string{
		"quote item update": `UPDATE plugin_idc_core_commerce_quote_item_snapshot
SET total_amount_minor=total_amount_minor+1 WHERE quote_item_no=?`,
		"quote item delete": `DELETE FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_item_no=?`,
		"quote charge update": `UPDATE plugin_idc_core_commerce_quote_charge_snapshot
SET charge_amount_minor=charge_amount_minor+1 WHERE quote_item_no=? AND configuration_price_dimension_no IS NOT NULL`,
		"quote charge delete": `DELETE FROM plugin_idc_core_commerce_quote_charge_snapshot
WHERE quote_item_no=? AND configuration_price_dimension_no IS NOT NULL`,
	}
	for name, statement := range statements {
		if _, execErr := database.Exec(ctx, statement, quoteItemNo); execErr == nil {
			t.Fatalf("%s unexpectedly changed immutable configuration-priced evidence", name)
		}
	}
}

func bindTestPublicBillingLabels(t *testing.T, reader catalog.PublicCatalogReader) {
	t.Helper()
	err := catalog.BindPublicBillingLabelResources(reader, catalog.PublicBillingLabelResources{
		"charge.one_time": {ZhCN: "一次性", EnUS: "One-time"},
		"period.hour":     {ZhCN: "小时", EnUS: "hour"},
		"period.day":      {ZhCN: "天", EnUS: "day"},
		"period.month":    {ZhCN: "个月", EnUS: "month"},
		"period.year":     {ZhCN: "年", EnUS: "year"},
	})
	if err != nil {
		t.Fatalf("bind public billing labels: %v", err)
	}
}

func TestCommerceCartServicePostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)

	publicReader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create public catalog reader: %v", err)
	}
	bindTestPublicBillingLabels(t, publicReader)
	phone, err := publicReader.GetBySlug(ctx, "generic-phone")
	if err != nil || phone == nil || phone.PublicSlug != "generic-phone" {
		t.Fatalf("generic phone must remain visible in the public catalog: product=%+v err=%v", phone, err)
	}
	beforeLegacyAttempt := commerceLegacyWriteCounts(t, ctx, database)
	phoneConfiguration, err := publicReader.GetConfiguration(ctx, "generic-phone")
	if err != nil || phoneConfiguration == nil || phoneConfiguration.ConfigurationMode != catalog.ConfigurationFixed ||
		len(phoneConfiguration.Groups) != 0 || len(phoneConfiguration.Rules) != 0 {
		t.Fatalf("generic phone fixed configuration=%+v err=%v", phoneConfiguration, err)
	}
	afterLegacyAttempt := commerceLegacyWriteCounts(t, ctx, database)
	if beforeLegacyAttempt != afterLegacyAttempt {
		t.Fatalf("rejected generic phone created legacy commerce or fulfillment data: before=%+v after=%+v", beforeLegacyAttempt, afterLegacyAttempt)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(publicReader)
	if err != nil {
		t.Fatalf("resolve purchase catalog boundary: %v", err)
	}
	configuration, err := publicReader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil {
		t.Fatalf("load public configuration: %v", err)
	}
	quote, err := publicReader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     "CNY",
	})
	if err != nil {
		t.Fatalf("load public quote: %v", err)
	}
	if quote == nil || len(quote.Terms) != 1 {
		t.Fatalf("load public quote result=%+v", quote)
	}
	purchase := catalog.PurchaseConfigurationInput{
		PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quote.Terms[0].BillingTermToken, CurrencyCode: "CNY",
		PurchaseCycles: 1,
	}
	customerOne := commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-1",
	}}
	service, err := newService(
		database, resolver, customerOne, rand.Reader, commerceSettlementStub{},
	)
	if err != nil {
		t.Fatalf("create commerce service: %v", err)
	}
	guest, err := service.CreateGuestCart(ctx, "CNY")
	if err != nil || guest == nil || guest.Cart == nil || guest.Cart.Version != 1 {
		t.Fatalf("create guest cart result=%+v err=%v", guest, err)
	}
	storedDigest, err := database.GetValue(ctx, `
SELECT guest_owner_digest FROM plugin_idc_core_commerce_cart WHERE cart_no = ?`, guest.Cart.CartNo)
	if err != nil || storedDigest.String() == guest.Token || storedDigest.String() != digestGuestToken(guest.Token) {
		t.Fatalf("guest token persistence digest=%q err=%v", storedDigest.String(), err)
	}
	if _, err = service.GetGuestCart(ctx, "gcart_invalid"); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("invalid guest token error=%v", err)
	}
	guestCart, err := service.AddGuestItem(ctx, guest.Token, AddItemInput{
		ExpectedCartVersion: guest.Cart.Version, Quantity: 2, Purchase: purchase,
	})
	if err != nil || len(guestCart.Items) != 1 || guestCart.Version != 2 ||
		guestCart.Items[0].StatusCode != CartItemStatusActive {
		t.Fatalf("add guest item cart=%+v err=%v", guestCart, err)
	}
	finalQuote, err := service.QuoteGuestCart(ctx, guest.Token, QuoteRequest{
		ExpectedCartVersion: guestCart.Version,
	})
	if err != nil || finalQuote == nil || finalQuote.SubtotalMinor != 20_000 ||
		finalQuote.DiscountMinor != 0 || finalQuote.TaxAmountMinor != 1_200 ||
		finalQuote.TotalAmountMinor != 21_200 || finalQuote.CartVersion != guestCart.Version+1 ||
		len(finalQuote.Items) != 1 {
		t.Fatalf("persist guest final quote result=%+v err=%v", finalQuote, err)
	}
	assertImmutableCommerceQuote(t, ctx, database, finalQuote.QuoteNo)
	if _, err = service.QuoteGuestCart(ctx, guest.Token, QuoteRequest{
		ExpectedCartVersion: guestCart.Version,
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("stale final quote request error=%v", err)
	}
	if _, err = service.SwitchGuestCurrency(ctx, guest.Token, "USD", finalQuote.CartVersion); !bizerr.Is(err, CodeCurrencySwitchBlocked) {
		t.Fatalf("non-empty guest currency switch error=%v", err)
	}

	start := make(chan struct{})
	results := make(chan *Cart, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			merged, mergeErr := service.MergeGuestCart(ctx, guest.Token)
			results <- merged
			errors <- mergeErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errors)
	for mergeErr := range errors {
		if mergeErr != nil {
			t.Fatalf("concurrent replay-safe merge failed: %v", mergeErr)
		}
	}
	mergedCartNo := ""
	for merged := range results {
		if merged == nil || len(merged.Items) != 1 {
			t.Fatalf("merged cart result=%+v", merged)
		}
		if mergedCartNo == "" {
			mergedCartNo = merged.CartNo
		} else if mergedCartNo != merged.CartNo {
			t.Fatalf("merge replay targets %q and %q", mergedCartNo, merged.CartNo)
		}
	}
	assertCommerceServiceMergeEvidence(t, ctx, database, guest.Cart.CartNo, mergedCartNo)
	marketingService, err := marketing.New(database, marketing.Config{ReservationTTL: 5 * time.Minute})
	if err != nil {
		t.Fatalf("create quote marketing service: %v", err)
	}
	marketingAdjustments, err := NewMarketingQuoteAdjustmentEvaluator(marketingService)
	if err != nil {
		t.Fatalf("create quote marketing adapter: %v", err)
	}
	service.adjustments = marketingAdjustments
	accountQuote, err := service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: resultsCartVersion(t, ctx, database, mergedCartNo)})
	if err != nil || accountQuote == nil || accountQuote.SubtotalMinor != 20_000 ||
		accountQuote.DiscountMinor != 1_000 || accountQuote.NetAmountMinor != 19_000 ||
		accountQuote.TaxAmountMinor != 1_140 || accountQuote.TotalAmountMinor != 20_140 ||
		len(accountQuote.Items) != 1 || len(accountQuote.Items[0].Adjustments) != 1 {
		t.Fatalf("persist customer quote with marketing result=%+v err=%v", accountQuote, err)
	}
	assertCommerceMarketingReservation(t, ctx, database, accountQuote.QuoteNo)
	accountQuote = assertConcurrentAccountQuote(t, ctx, service, accountQuote)
	assertCommerceMarketingRollback(t, ctx, database, service, accountQuote)
	assertMultiItemGuestQuote(t, ctx, service, purchase)

	if _, err = database.Exec(ctx, `
UPDATE plugin_idc_core_catalog_product_version
SET status_code = 'superseded'
WHERE version_no = 'version-commerce-service-1';
INSERT INTO plugin_idc_core_catalog_product_version
    (version_no, product_no, version_number, status_code, source_code, published_at)
VALUES ('version-commerce-service-2', 'product-commerce-service', 2,
        'published', 'manual', CURRENT_TIMESTAMP);
UPDATE plugin_idc_core_catalog_product
SET current_version_no = 'version-commerce-service-2'
WHERE product_no = 'product-commerce-service'`); err != nil {
		t.Fatalf("replace published version: %v", err)
	}
	stale, err := service.GetAccountCart(ctx)
	if err != nil || len(stale.Items) != 1 || stale.Items[0].IssueCode != catalog.PurchaseIssueVersionReplaced ||
		stale.Items[0].RepairAction != CartRepairReplaceVersion ||
		stale.Items[0].StatusCode != CartItemStatusNeedsReprice {
		t.Fatalf("stale cart projection=%+v err=%v", stale, err)
	}
	removed, err := service.RemoveAccountItem(ctx, stale.Items[0].CartItemNo,
		stale.Version, stale.Items[0].Version,
	)
	if err != nil || len(removed.Items) != 0 || removed.Version != stale.Version+1 {
		t.Fatalf("remove stale cart item result=%+v err=%v", removed, err)
	}
	assertConcurrentAccountCartOpen(t, ctx, database, resolver)

	customerTwo := commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-2",
	}}
	serviceTwo, err := newService(
		database, resolver, customerTwo, rand.Reader, commerceSettlementStub{},
	)
	if err != nil {
		t.Fatalf("create second commerce service: %v", err)
	}
	accountTwo, err := serviceTwo.OpenAccountCart(ctx, "USD")
	if err != nil {
		t.Fatalf("open explicit USD account cart: %v", err)
	}
	guestTwo, err := serviceTwo.CreateGuestCart(ctx, "CNY")
	if err != nil {
		t.Fatalf("create currency-conflict guest cart: %v", err)
	}
	if _, err = serviceTwo.MergeGuestCart(ctx, guestTwo.Token); !bizerr.Is(err, CodeCurrencyConflict) {
		t.Fatalf("currency-conflict merge error=%v", err)
	}
	status, err := database.GetValue(ctx, `
SELECT status_code FROM plugin_idc_core_commerce_cart WHERE cart_no = ?`, guestTwo.Cart.CartNo)
	if err != nil || status.String() != "active" {
		t.Fatalf("currency-conflict source status=%q err=%v", status.String(), err)
	}
	switched, err := serviceTwo.SwitchAccountCurrency(ctx, "CNY", accountTwo.Version)
	if err != nil || switched == nil || switched.CurrencyCode != "CNY" || switched.CartNo == accountTwo.CartNo {
		t.Fatalf("explicit account currency switch result=%+v err=%v", switched, err)
	}
	mergedTwo, err := serviceTwo.MergeGuestCart(ctx, guestTwo.Token)
	if err != nil || mergedTwo == nil || mergedTwo.CartNo != switched.CartNo || mergedTwo.CurrencyCode != "CNY" {
		t.Fatalf("merge after explicit currency switch result=%+v err=%v", mergedTwo, err)
	}
}

func assertConcurrentAccountCartOpen(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	resolver catalog.PurchaseConfigurationResolver,
) {
	t.Helper()
	identity := commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-3",
	}}
	service, err := newService(
		database, resolver, identity, rand.Reader, commerceSettlementStub{},
	)
	if err != nil {
		t.Fatalf("create concurrent open service: %v", err)
	}
	start := make(chan struct{})
	results := make(chan *Cart, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			cart, openErr := service.OpenAccountCart(ctx, "CNY")
			results <- cart
			errors <- openErr
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errors)
	for openErr := range errors {
		if openErr != nil {
			t.Fatalf("concurrent account cart open failed: %v", openErr)
		}
	}
	cartNo := ""
	for cart := range results {
		if cart == nil {
			t.Fatal("concurrent account cart open returned nil")
		}
		if cartNo == "" {
			cartNo = cart.CartNo
		} else if cartNo != cart.CartNo {
			t.Fatalf("concurrent account cart opens returned %q and %q", cartNo, cart.CartNo)
		}
	}
	count, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_commerce_cart
WHERE customer_no = 'customer-commerce-service-3' AND status_code = 'active'`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("concurrent account active cart count=%d err=%v", count.Int(), err)
	}
}

func assertCommerceServiceMergeEvidence(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	sourceCartNo string,
	targetCartNo string,
) {
	t.Helper()
	var source struct {
		StatusCode       string `orm:"status_code"`
		MergedIntoCartNo string `orm:"merged_into_cart_no"`
	}
	if err := database.GetScan(ctx, &source, `
SELECT status_code, merged_into_cart_no
FROM plugin_idc_core_commerce_cart WHERE cart_no = ?`, sourceCartNo); err != nil {
		t.Fatalf("load merge source evidence: %v", err)
	}
	if source.StatusCode != "merged" || source.MergedIntoCartNo != targetCartNo {
		t.Fatalf("merge source evidence=%+v target=%q", source, targetCartNo)
	}
	targetItems, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_commerce_cart_item
WHERE cart_no = ? AND status_code <> 'removed'`, targetCartNo)
	if err != nil || targetItems.Int() != 1 {
		t.Fatalf("merge target items=%d err=%v", targetItems.Int(), err)
	}
	sourceItems, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_commerce_cart_item
WHERE cart_no = ? AND status_code = 'removed'
  AND issue_code = 'merged_into_customer_cart'`, sourceCartNo)
	if err != nil || sourceItems.Int() != 1 {
		t.Fatalf("merge source retained items=%d err=%v", sourceItems.Int(), err)
	}
}

func assertImmutableCommerceQuote(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	quoteNo string,
) {
	t.Helper()
	var projection struct {
		ItemCount        int   `orm:"item_count"`
		ChargeCount      int   `orm:"charge_count"`
		AdjustmentCount  int   `orm:"adjustment_count"`
		SubtotalMinor    int64 `orm:"subtotal_minor"`
		DiscountMinor    int64 `orm:"discount_minor"`
		NetAmountMinor   int64 `orm:"net_amount_minor"`
		TaxAmountMinor   int64 `orm:"tax_amount_minor"`
		TotalAmountMinor int64 `orm:"total_amount_minor"`
	}
	if err := database.GetScan(ctx, &projection, `
SELECT q.item_count,
       (SELECT count(*) FROM plugin_idc_core_commerce_quote_charge_snapshot charge
         JOIN plugin_idc_core_commerce_quote_item_snapshot item
           ON item.quote_item_no = charge.quote_item_no WHERE item.quote_no = q.quote_no)::INT AS charge_count,
       (SELECT count(*) FROM plugin_idc_core_commerce_quote_adjustment_snapshot adjustment
         JOIN plugin_idc_core_commerce_quote_item_snapshot item
           ON item.quote_item_no = adjustment.quote_item_no WHERE item.quote_no = q.quote_no)::INT AS adjustment_count,
       q.subtotal_minor, q.discount_minor, q.net_amount_minor, q.tax_amount_minor, q.total_amount_minor
FROM plugin_idc_core_commerce_quote_snapshot q WHERE q.quote_no = ?`, quoteNo); err != nil {
		t.Fatalf("load immutable quote projection: %v", err)
	}
	if projection.ItemCount != 1 || projection.ChargeCount != 1 || projection.AdjustmentCount != 0 ||
		projection.SubtotalMinor-projection.DiscountMinor != projection.NetAmountMinor ||
		projection.NetAmountMinor+projection.TaxAmountMinor != projection.TotalAmountMinor {
		t.Fatalf("immutable quote projection is inconsistent: %+v", projection)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_commerce_quote_snapshot
SET subtotal_minor = subtotal_minor + 1 WHERE quote_no = ?`, quoteNo); err == nil {
		t.Fatal("immutable quote header accepted an update")
	}
}

func resultsCartVersion(t *testing.T, ctx context.Context, database gdb.DB, cartNo string) int64 {
	t.Helper()
	value, err := database.GetValue(ctx, `
SELECT version FROM plugin_idc_core_commerce_cart WHERE cart_no = ?`, cartNo)
	if err != nil || value.Int64() < 1 {
		t.Fatalf("load merged cart version=%d err=%v", value.Int64(), err)
	}
	return value.Int64()
}

func assertCommerceMarketingReservation(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	quoteNo string,
) {
	t.Helper()
	var evidence struct {
		Reservations  int       `orm:"reservations"`
		Adjustments   int       `orm:"adjustments"`
		QuotedValue   int64     `orm:"quoted_value"`
		QuoteExpiry   time.Time `orm:"quote_expiry"`
		ReserveExpiry time.Time `orm:"reserve_expiry"`
	}
	if err := database.GetScan(ctx, &evidence, `
SELECT (SELECT count(*) FROM plugin_idc_core_marketing_campaign_reservation
         WHERE quote_reference = q.quote_no AND status_code = 'active')::INT AS reservations,
       (SELECT count(*) FROM plugin_idc_core_commerce_quote_adjustment_snapshot a
         JOIN plugin_idc_core_commerce_quote_item_snapshot i ON i.quote_item_no = a.quote_item_no
         WHERE i.quote_no = q.quote_no)::INT AS adjustments,
       COALESCE((SELECT sum(quoted_value_minor) FROM plugin_idc_core_marketing_campaign_reservation
         WHERE quote_reference = q.quote_no), 0)::BIGINT AS quoted_value,
       q.expires_at AS quote_expiry,
       (SELECT min(expires_at) FROM plugin_idc_core_marketing_campaign_reservation
         WHERE quote_reference = q.quote_no) AS reserve_expiry
FROM plugin_idc_core_commerce_quote_snapshot q WHERE q.quote_no = ?`, quoteNo); err != nil {
		t.Fatalf("load commerce marketing reservation evidence: %v", err)
	}
	if evidence.Reservations != 1 || evidence.Adjustments != 1 || evidence.QuotedValue != 1_000 ||
		evidence.QuoteExpiry.After(evidence.ReserveExpiry) {
		t.Fatalf("commerce marketing reservation evidence=%+v", evidence)
	}
}

func assertCommerceMarketingRollback(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	service *serviceImpl,
	current *Quote,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
CREATE OR REPLACE FUNCTION reject_commerce_quote_test_insert()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'quote rollback fixture';
END;
$$;
CREATE TRIGGER reject_commerce_quote_test_insert
BEFORE INSERT ON plugin_idc_core_commerce_quote_snapshot
FOR EACH ROW EXECUTE FUNCTION reject_commerce_quote_test_insert()`); err != nil {
		t.Fatalf("install quote rollback fixture: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.Exec(ctx, `
DROP TRIGGER IF EXISTS reject_commerce_quote_test_insert
ON plugin_idc_core_commerce_quote_snapshot;
DROP FUNCTION IF EXISTS reject_commerce_quote_test_insert()`)
	})
	before, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_marketing_campaign_reservation`)
	if err != nil {
		t.Fatalf("count reservations before quote rollback: %v", err)
	}
	if _, err = service.QuoteAccountCart(ctx, QuoteRequest{
		ExpectedCartVersion: current.CartVersion,
	}); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("forced quote persistence failure returned %v", err)
	}
	after, err := database.GetValue(ctx, `
SELECT count(*) FROM plugin_idc_core_marketing_campaign_reservation`)
	if err != nil || after.Int() != before.Int() {
		t.Fatalf("marketing reservation escaped quote rollback before=%d after=%d err=%v",
			before.Int(), after.Int(), err)
	}
	var cart struct {
		Version       int64  `orm:"version"`
		LatestQuoteNo string `orm:"latest_quote_no"`
	}
	if err = database.GetScan(ctx, &cart, `
SELECT version, latest_quote_no FROM plugin_idc_core_commerce_cart WHERE cart_no = ?`, current.CartNo); err != nil ||
		cart.Version != current.CartVersion || cart.LatestQuoteNo != current.QuoteNo {
		t.Fatalf("cart changed during quote rollback cart=%+v err=%v", cart, err)
	}
	if _, err = database.Exec(ctx, `
DROP TRIGGER reject_commerce_quote_test_insert
ON plugin_idc_core_commerce_quote_snapshot;
DROP FUNCTION reject_commerce_quote_test_insert()`); err != nil {
		t.Fatalf("remove quote rollback fixture: %v", err)
	}
}

func assertConcurrentAccountQuote(
	t *testing.T,
	ctx context.Context,
	service *serviceImpl,
	current *Quote,
) *Quote {
	t.Helper()
	start := make(chan struct{})
	results := make(chan *Quote, 2)
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			quoted, err := service.QuoteAccountCart(ctx, QuoteRequest{
				ExpectedCartVersion: current.CartVersion,
			})
			results <- quoted
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errors)
	successes, conflicts := 0, 0
	for err := range errors {
		if err == nil {
			successes++
		} else if bizerr.Is(err, CodeConflict) {
			conflicts++
		} else {
			t.Fatalf("concurrent account quote returned %v", err)
		}
	}
	var winner *Quote
	for result := range results {
		if result != nil {
			winner = result
		}
	}
	if successes != 1 || conflicts != 1 || winner == nil ||
		winner.CartVersion != current.CartVersion+1 {
		t.Fatalf("concurrent quote successes=%d conflicts=%d winner=%+v", successes, conflicts, winner)
	}
	return winner
}

func assertMultiItemGuestQuote(
	t *testing.T,
	ctx context.Context,
	service *serviceImpl,
	purchase catalog.PurchaseConfigurationInput,
) {
	t.Helper()
	guest, err := service.CreateGuestCart(ctx, "CNY")
	if err != nil {
		t.Fatalf("create multi-item quote cart: %v", err)
	}
	first, err := service.AddGuestItem(ctx, guest.Token, AddItemInput{
		ExpectedCartVersion: guest.Cart.Version, Quantity: 2, Purchase: purchase,
	})
	if err != nil {
		t.Fatalf("add first multi-item quote line: %v", err)
	}
	second, err := service.AddGuestItem(ctx, guest.Token, AddItemInput{
		ExpectedCartVersion: first.Version, Quantity: 2, Purchase: purchase,
	})
	if err != nil || len(second.Items) != 2 {
		t.Fatalf("add second multi-item quote line cart=%+v err=%v", second, err)
	}
	quoted, err := service.QuoteGuestCart(ctx, guest.Token, QuoteRequest{
		ExpectedCartVersion: second.Version,
	})
	if err != nil || quoted.SubtotalMinor != 40_000 || quoted.TaxAmountMinor != 2_400 ||
		quoted.TotalAmountMinor != 42_400 || len(quoted.Items) != 2 {
		t.Fatalf("multi-item collection quote result=%+v err=%v", quoted, err)
	}
}

func newCommerceServiceTestDatabase(t *testing.T) (context.Context, gdb.DB) {
	t.Helper()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(commerceIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL cart service tests", commerceIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create cart service admin database: %v", err)
	}
	var randomSuffix [8]byte
	if _, err = rand.Read(randomSuffix[:]); err != nil {
		t.Fatalf("generate cart service schema: %v", err)
	}
	schema := "linapro_cart_service_" + hex.EncodeToString(randomSuffix[:])
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatalf("create cart service schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_cart_service_") {
			_, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`)
		}
		_ = admin.Close(ctx)
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("create scoped cart service database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	if _, err = database.Exec(ctx, commerceServiceDictionarySQL); err != nil {
		t.Fatalf("create cart service dictionary fixture: %v", err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve cart service test path")
	}
	migrationRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	for _, name := range []string{
		"001-core-foundation.sql", "003-customer-account.sql", "010-catalog-foundation.sql",
		"013-catalog-billing-policy.sql",
		"014-catalog-inventory-routing.sql", "019-catalog-tax-policy.sql",
		"020-marketing-foundation.sql", "022-commerce-foundation.sql",
		"023-billing-ledger-foundation.sql",
		"024-commerce-order-lifecycle.sql",
		"025-commerce-payment-identity-gate.sql",
		"026-commerce-assisted-order.sql",
		"027-billing-invoice-lifecycle.sql",
		"028-billing-invoice-transform.sql",
		"029-billing-reviewed-adjustment.sql",
		"030-service-renewal-foundation.sql",
		"031-service-renewal-execution.sql",
		"032-service-expiry-scheduling.sql",
		"033-billing-directory-indexes.sql",
		"034-credit-account-foundation.sql",
		"035-financial-settlement.sql",
		"036-trial-qualification.sql",
		"037-financial-settlement-fulfillment.sql",
		"038-service-provision-execution.sql",
		"042-support-content.sql",
		"043-contract-policy.sql",
		"044-contract-failure-compensation.sql",
		"052-online-refund.sql",
		"053-credit-refund-lifecycle.sql",
		"054-credit-online-refund-integration.sql",
		"061-commission-refund-reversal.sql",
		"062-marketing-performance-abuse.sql",
		"070-customer-short-code.sql",
		"073-general-product-catalog.sql",
		"074-product-automation-binding.sql",
		"075-structured-product-configuration.sql",
		"077-management-configuration-files.sql",
		"078-product-fulfillment-execution.sql",
		"079-generic-commerce-query-indexes.sql",
		"080-generic-delivery-entitlement.sql",
		"081-simple-product-version-inventory.sql",
		"082-generic-commerce-inventory-snapshots.sql",
		"083-fixed-product-empty-configuration.sql",
		"084-general-product-version-cost.sql",
		"085-generic-trial-unit-identity.sql",
		"086-generic-commerce-shadow-removal.sql",
		"087-generic-refund-entitlement.sql",
		"088-generic-entitlement-renewal.sql",
		"089-retire-provider-catalog-draft.sql",
		"090-generic-delivery-entitlement-backfill.sql",
		"091-support-entitlement-association.sql",
		"092-entitlement-created-safe-product-labels.sql",
		"093-simple-inventory-fail-closed.sql",
		"094-generic-configuration-pricing.sql",
		"102-channel-commerce-attribution.sql",
	} {
		content, readErr := os.ReadFile(filepath.Join(migrationRoot, name))
		if readErr != nil {
			t.Fatalf("read cart service migration %s: %v", name, readErr)
		}
		if name == "086-generic-commerce-shadow-removal.sql" {
			assertQuantityShadowCleanupPrecedesColumnRemoval(t, string(content))
		}
		attempts := 1
		if name == "061-commission-refund-reversal.sql" || name == "062-marketing-performance-abuse.sql" || name == "078-product-fulfillment-execution.sql" || name == "079-generic-commerce-query-indexes.sql" || name == "080-generic-delivery-entitlement.sql" || name == "081-simple-product-version-inventory.sql" || name == "082-generic-commerce-inventory-snapshots.sql" || name == "083-fixed-product-empty-configuration.sql" || name == "084-general-product-version-cost.sql" || name == "085-generic-trial-unit-identity.sql" || name == "086-generic-commerce-shadow-removal.sql" || name == "087-generic-refund-entitlement.sql" || name == "088-generic-entitlement-renewal.sql" || name == "089-retire-provider-catalog-draft.sql" || name == "090-generic-delivery-entitlement-backfill.sql" || name == "091-support-entitlement-association.sql" || name == "092-entitlement-created-safe-product-labels.sql" || name == "093-simple-inventory-fail-closed.sql" || name == "094-generic-configuration-pricing.sql" || name == "102-channel-commerce-attribution.sql" {
			attempts = 2
		}
		for attempt := 0; attempt < attempts; attempt++ {
			if name == "085-generic-trial-unit-identity.sql" ||
				name == "087-generic-refund-entitlement.sql" ||
				name == "088-generic-entitlement-renewal.sql" {
				executeCommerceMigrationStatements(t, ctx, database, name, string(content))
				continue
			}
			if _, err = database.Exec(ctx, string(content)); err != nil {
				t.Fatalf("execute cart service migration %s attempt %d: %v", name, attempt+1, err)
			}
		}
	}
	assertLegacyQuantityShadowsRemoved(t, ctx, database)
	return ctx, database
}

func assertQuantityShadowCleanupPrecedesColumnRemoval(t *testing.T, migration string) {
	t.Helper()
	columnRemoval := strings.Index(migration, `DROP COLUMN IF EXISTS "server_quantity"`)
	if columnRemoval < 0 {
		t.Fatal("generic commerce shadow migration does not remove server_quantity")
	}
	for _, dependency := range []string{
		"trg_idc_validate_cart_item_quantity", "trg_idc_validate_assisted_item_quantity",
		"trg_idc_validate_quote_item_quantity", "trg_idc_validate_order_item_quantity",
		"trg_idc_check_cart_item_legacy_quantity", "trg_idc_check_assisted_item_legacy_quantity",
		"trg_idc_check_quote_item_legacy_quantity", "trg_idc_check_order_item_legacy_quantity",
		"plugin_idc_core_validate_generic_quantity_shadow",
		"plugin_idc_core_validate_legacy_quantity_shadow",
	} {
		position := strings.Index(migration, dependency)
		if position < 0 || position > columnRemoval {
			t.Fatalf("generic commerce shadow dependency %s is not removed before server_quantity", dependency)
		}
	}
}

func assertLegacyQuantityShadowsRemoved(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	remaining, err := database.GetValue(ctx, `SELECT
	(SELECT count(*) FROM pg_trigger trigger_row
	 JOIN pg_class relation ON relation.oid=trigger_row.tgrelid
	 JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
	 WHERE namespace.nspname=current_schema() AND NOT trigger_row.tgisinternal AND trigger_row.tgname IN (
	 'trg_idc_validate_cart_item_quantity','trg_idc_validate_assisted_item_quantity',
	 'trg_idc_validate_quote_item_quantity','trg_idc_validate_order_item_quantity',
	 'trg_idc_check_cart_item_legacy_quantity','trg_idc_check_assisted_item_legacy_quantity',
	 'trg_idc_check_quote_item_legacy_quantity','trg_idc_check_order_item_legacy_quantity')) +
	(SELECT count(*) FROM pg_proc procedure
	 JOIN pg_namespace namespace ON namespace.oid=procedure.pronamespace
	 WHERE namespace.nspname=current_schema() AND procedure.proname IN (
	 'plugin_idc_core_validate_generic_quantity_shadow',
	 'plugin_idc_core_validate_legacy_quantity_shadow'))`)
	if err != nil {
		t.Fatalf("inspect legacy quantity shadows: %v", err)
	}
	if remaining.Int() != 0 {
		t.Fatalf("generic commerce migration retained %d legacy quantity shadow dependencies", remaining.Int())
	}
}

func seedCommerceServiceCatalog(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, commerceServiceCatalogSQL); err != nil {
		t.Fatalf("seed cart service catalog: %v", err)
	}
}

const commerceServiceDictionarySQL = `
CREATE TABLE sys_dict_type (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
    name VARCHAR(128) NOT NULL DEFAULT '', type VARCHAR(128) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
    allow_tenant_override BOOL NOT NULL DEFAULT FALSE, remark VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    UNIQUE (tenant_id, type)
);
CREATE TABLE sys_dict_data (
    id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
    dict_type VARCHAR(128) NOT NULL DEFAULT '', label VARCHAR(128) NOT NULL DEFAULT '',
    value VARCHAR(128) NOT NULL DEFAULT '', sort INT NOT NULL DEFAULT 0,
    tag_style VARCHAR(128) NOT NULL DEFAULT '', status SMALLINT NOT NULL DEFAULT 1,
    is_builtin SMALLINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
    UNIQUE (tenant_id, dict_type, value)
);`

const commerceServiceCatalogSQL = `
INSERT INTO plugin_idc_core_customer_subject
    (customer_no, customer_code, customer_type_code, status_code, display_name)
VALUES
    ('customer-commerce-service-1', 10001, 'individual', 'active', 'Customer One'),
    ('customer-commerce-service-2', 10002, 'individual', 'active', 'Customer Two'),
    ('customer-commerce-service-3', 10003, 'individual', 'active', 'Customer Three');
INSERT INTO plugin_idc_core_customer_contact
    (contact_no, customer_no, display_name, phone_country_code, phone, phone_lookup_digest)
VALUES
    ('contact-commerce-service-1', 'customer-commerce-service-1', 'Customer One', '+86', '13800138001', repeat('1',64)),
    ('contact-commerce-service-2', 'customer-commerce-service-2', 'Customer Two', '+86', '13800138002', repeat('2',64)),
    ('contact-commerce-service-3', 'customer-commerce-service-3', 'Customer Three', '+86', '13800138003', repeat('3',64));
INSERT INTO plugin_idc_core_customer_contact_role
    (customer_no, contact_no, role_code, status_code)
VALUES
    ('customer-commerce-service-1', 'contact-commerce-service-1', 'primary', 'active'),
    ('customer-commerce-service-2', 'contact-commerce-service-2', 'primary', 'active'),
    ('customer-commerce-service-3', 'contact-commerce-service-3', 'primary', 'active');
INSERT INTO plugin_idc_core_catalog_category
    (category_no, status_code, name_zh_cn, name_en_us)
VALUES ('category-commerce-service', 'active', '人工服务', 'Manual services');
INSERT INTO plugin_idc_core_catalog_category
    (category_no, status_code, name_zh_cn, name_en_us)
VALUES ('ctg_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'active', '手机', 'Phones');
INSERT INTO plugin_idc_core_catalog_product
    (product_no, category_no, status_code, public_slug, name_zh_cn, name_en_us)
VALUES ('product-commerce-service', 'category-commerce-service',
		'active', 'commerce-service-cloud', '商务咨询服务',
		'Business consulting service');
INSERT INTO plugin_idc_core_catalog_product
    (product_no, category_no, status_code, public_slug, name_zh_cn, name_en_us)
VALUES ('prd_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        'ctg_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'active', 'generic-phone',
        '通用手机', 'Generic phone');
INSERT INTO plugin_idc_core_catalog_product_version
    (version_no, product_no, version_number, status_code, source_code, published_at)
VALUES ('version-commerce-service-1', 'product-commerce-service', 1,
		'draft', 'manual', NULL);
INSERT INTO plugin_idc_core_catalog_product_version
    (version_no, product_no, version_number, status_code, source_code, published_at)
VALUES ('ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
		'prd_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 1, 'draft', 'manual', NULL);
INSERT INTO plugin_idc_core_catalog_product_version_inventory
    (version_no, tracking_enabled, sellable_quantity, version)
VALUES
	('version-commerce-service-1', TRUE, 100, 1),
	('ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', FALSE, 0, 1);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
	(version_no, strategy_code, granularity_code, ongoing_management, maximum_attempts,
	 retry_base_seconds, policy_digest, created_by)
VALUES
	('version-commerce-service-1', 'manual', 'per_order_item', FALSE, 10, 30, repeat('b', 64), 1),
	('ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'manual', 'per_order_item', FALSE, 10, 30, repeat('c', 64), 1);
UPDATE plugin_idc_core_catalog_product_version
SET status_code = 'published', published_at = CURRENT_TIMESTAMP
WHERE version_no IN ('version-commerce-service-1', 'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
UPDATE plugin_idc_core_catalog_product
SET current_version_no = 'version-commerce-service-1'
WHERE product_no = 'product-commerce-service';
UPDATE plugin_idc_core_catalog_product
SET current_version_no = 'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'
WHERE product_no = 'prd_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
INSERT INTO plugin_idc_core_catalog_billing_rule
    (billing_rule_no, version_no, rule_code, charge_type_code, period_unit_code,
     period_count, status_code)
VALUES ('billing-commerce-service-month', 'version-commerce-service-1', 'monthly',
        'recurring', 'month', 1, 'active');
INSERT INTO plugin_idc_core_catalog_billing_rule
    (billing_rule_no, version_no, rule_code, charge_type_code, period_unit_code,
     period_count, price_phase_code, status_code)
VALUES ('bil_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'once',
        'one_time', 'once', 1, 'new_purchase', 'active');
INSERT INTO plugin_idc_core_catalog_price
    (price_no, version_no, billing_rule_no, option_no, currency_code, amount_minor,
     setup_fee_minor, renewal_amount_minor, status_code)
VALUES
	('price-commerce-service-base', 'version-commerce-service-1',
	 'billing-commerce-service-month', NULL, 'CNY', 10000, 0, 10000, 'active');
INSERT INTO plugin_idc_core_catalog_price
    (price_no, version_no, billing_rule_no, option_no, currency_code, amount_minor,
     setup_fee_minor, renewal_amount_minor, status_code)
VALUES ('prc_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        'bil_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', NULL, 'CNY', 699900, 0, NULL, 'active');
INSERT INTO plugin_idc_core_catalog_tax_policy
    (tax_policy_no, version_no, currency_code, price_includes_tax,
     tax_treatment_code, tax_rate_basis_points, tax_category_code,
     rounding_scope_code, rounding_mode_code, rounding_quantum_minor)
VALUES ('tax-commerce-service-cny', 'version-commerce-service-1', 'CNY', FALSE,
        'taxable', 600, 'cloud_service', 'line', 'half_up', 1);
INSERT INTO plugin_idc_core_catalog_tax_policy
    (tax_policy_no, version_no, currency_code, price_includes_tax,
     tax_treatment_code, tax_rate_basis_points, tax_category_code,
     rounding_scope_code, rounding_mode_code, rounding_quantum_minor)
VALUES ('tax_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',
        'ver_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa', 'CNY', TRUE,
        'exempt', NULL, 'goods', 'line', 'half_up', 1);
INSERT INTO plugin_idc_core_catalog_resource_pool_binding
    (binding_no, version_no, pool_no, region_code, line_code, priority, weight,
     new_order_enabled, status_code, maintenance_mode_code,
     minimum_sellable_quantity, uncertain_sale_policy_code)
VALUES ('binding-commerce-service', 'version-commerce-service-1',
        'pool-commerce-service', '', '', 1, 100, TRUE, 'active', 'normal', 1, 'stop_sale');
INSERT INTO plugin_idc_core_catalog_inventory_snapshot
    (snapshot_no, binding_no, source_snapshot_no, source_fingerprint,
     availability_code, available_quantity, safety_margin, sellable_quantity,
     observed_at, source_completed_at, expires_at)
VALUES ('inventory-commerce-service', 'binding-commerce-service', 'source-commerce-service',
        repeat('a', 64), 'available', 100, 0, 100,
        CURRENT_TIMESTAMP - INTERVAL '1 minute',
        CURRENT_TIMESTAMP - INTERVAL '59 seconds',
        CURRENT_TIMESTAMP + INTERVAL '1 hour');
INSERT INTO plugin_idc_core_marketing_campaign (
    campaign_no, campaign_code, name_zh_cn, name_en_us, campaign_type_code,
    starts_at, ends_at, priority, stack_policy_code, stack_group_code, status_code
) VALUES (
    'campaign-commerce-service', 'campaign_commerce_service',
    '购物车立减', 'Cart discount', 'discount',
    CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '1 day',
    1, 'stackable', 'cart', 'active'
);
INSERT INTO plugin_idc_core_marketing_campaign_scope
    (campaign_no, scope_kind_code, match_mode_code, reference_code)
VALUES ('campaign-commerce-service', 'product', 'include', 'product-commerce-service');
INSERT INTO plugin_idc_core_marketing_benefit_rule (
    benefit_no, campaign_no, campaign_type_code, benefit_type_code, currency_code,
    amount_minor, purchase_phase_code, cycle_scope_code
) VALUES (
    'benefit-commerce-service', 'campaign-commerce-service', 'discount',
    'fixed_discount', 'CNY', 1000, 'new_purchase', 'first'
);`
