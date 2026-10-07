// This file verifies atomic checkout, inventory, and idempotency behavior in PostgreSQL.
package commerce

import (
	"context"
	"crypto/rand"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/contracttemplatecap"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"lina-plugin-idc-core/backend/internal/service/contractpolicy"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

const checkoutContractTemplateNo = "ctpl_11111111111111111111111111111111"

type checkoutContractTemplateSource struct{}

type checkoutPurchaseRuntimeStub struct{}

func (checkoutPurchaseRuntimeStub) CheckPublicPurchaseAvailability(context.Context, string) (bool, error) {
	return true, nil
}

func (checkoutPurchaseRuntimeStub) ResolvePublicPurchaseCandidates(
	context.Context,
	string,
	*capmodel.ConfigurationSchemaDeclaration,
	[]configurationcontract.ConfigurationValue,
) (configurationcontract.ResolvedCandidates, error) {
	return configurationcontract.ResolvedCandidates{}, nil
}

type checkoutPurchaseFileVerifierStub struct {
	purchaseconfigurationfile.Service
}

func (checkoutPurchaseFileVerifierStub) VerifyFields(
	_ context.Context,
	inputs []purchaseconfigurationfile.VerifyInput,
) ([]purchaseconfigurationfile.VerifiedField, error) {
	result := make([]purchaseconfigurationfile.VerifiedField, 0, len(inputs))
	for _, input := range inputs {
		result = append(result, purchaseconfigurationfile.VerifiedField{Target: input.Target})
	}
	return result, nil
}

func (checkoutContractTemplateSource) ListAvailable(
	context.Context,
	contracttemplatecap.Selection,
) ([]contracttemplatecap.Template, error) {
	return []contracttemplatecap.Template{checkoutContractTemplate()}, nil
}

func (checkoutContractTemplateSource) ResolveAvailable(
	_ context.Context,
	_ contracttemplatecap.Selection,
	templateNos []string,
) ([]contracttemplatecap.Template, error) {
	if len(templateNos) != 1 || templateNos[0] != checkoutContractTemplateNo {
		return nil, contracttemplatecap.ErrTemplateUnavailable
	}
	return []contracttemplatecap.Template{checkoutContractTemplate()}, nil
}

func (checkoutContractTemplateSource) OrderContractExists(context.Context, string) (bool, error) {
	return false, nil
}

func checkoutContractTemplate() contracttemplatecap.Template {
	return contracttemplatecap.Template{
		TemplateNo: checkoutContractTemplateNo, VersionNo: 1,
		Name: "Checkout contract fixture", ScopeCode: "all_customers",
		SigningOrderCode: "customer_first", LanguageCode: "zh-CN",
	}
}

type checkoutTestScenario struct {
	ctx      context.Context
	database gdb.DB
	resolver catalog.PurchaseConfigurationResolver
	purchase catalog.PurchaseConfigurationInput
	service  *serviceImpl
	quote    *Quote
}

type failingCheckoutEventWriter struct{}

func (failingCheckoutEventWriter) Enqueue(
	context.Context,
	outbox.EnqueueInput,
) (*outbox.Event, error) {
	return nil, errors.New("injected checkout outbox failure")
}

func TestCommerceCheckoutAtomicPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 2, true, 0)
	request := CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-concurrent-0001"}

	const workers = 8
	start := make(chan struct{})
	results := make([]*Checkout, workers)
	errorsFound := make([]error, workers)
	var wait sync.WaitGroup
	for index := range workers {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errorsFound[index] = scenario.service.CheckoutAccountQuote(scenario.ctx, request)
		}(index)
	}
	close(start)
	wait.Wait()
	orderNo, invoiceNo := "", ""
	for index := range workers {
		if errorsFound[index] != nil || results[index] == nil {
			t.Fatalf("concurrent checkout %d result=%+v err=%v", index, results[index], errorsFound[index])
		}
		if orderNo == "" {
			orderNo, invoiceNo = results[index].OrderNo, results[index].InvoiceNo
		} else if results[index].OrderNo != orderNo || results[index].InvoiceNo != invoiceNo {
			t.Fatalf("checkout replay diverged: order=%q/%q invoice=%q/%q",
				orderNo, results[index].OrderNo, invoiceNo, results[index].InvoiceNo)
		}
	}
	replayed, err := scenario.service.CheckoutAccountQuote(scenario.ctx, request)
	if err != nil || replayed == nil || !replayed.Reused || replayed.OrderNo != orderNo {
		t.Fatalf("checkout replay result=%+v err=%v", replayed, err)
	}
	if _, err = scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-different-0002",
	}); !bizerr.Is(err, CodeConflict) {
		t.Fatalf("same quote with a different key error=%v", err)
	}

	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_order", 1)
	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_order_item", 1)
	assertCheckoutCount(t, scenario, "plugin_idc_core_billing_invoice", 1)
	assertCheckoutCount(t, scenario, "plugin_idc_core_billing_invoice_item", 1)
	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_inventory_reservation", 1)
	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_fulfillment_unit", 0)
	assertCheckoutCount(t, scenario, "plugin_idc_core_outbox_event", 1)
	assertCheckoutCount(t, scenario, "plugin_idc_core_billing_ledger_transaction", 0)

	var evidence struct {
		CartStatus      string `orm:"cart_status"`
		OrderStatus     string `orm:"order_status"`
		InvoiceStatus   string `orm:"invoice_status"`
		Reservation     string `orm:"reservation_status"`
		ReservedUnits   int    `orm:"reserved_units"`
		MarketingStatus string `orm:"marketing_status"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `
SELECT c.status_code AS cart_status, o.status_code AS order_status,
       i.status_code AS invoice_status, r.status_code AS reservation_status,
       r.reserved_quantity AS reserved_units, m.status_code AS marketing_status
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_commerce_quote_snapshot q ON q.quote_no = o.quote_no
JOIN plugin_idc_core_commerce_cart c ON c.cart_no = q.cart_no
JOIN plugin_idc_core_billing_invoice i ON i.order_no = o.order_no
JOIN plugin_idc_core_commerce_inventory_reservation r ON r.order_no = o.order_no
JOIN plugin_idc_core_marketing_campaign_reservation m ON m.quote_reference = q.quote_no
WHERE o.order_no = ?`, orderNo); err != nil {
		t.Fatalf("load checkout evidence: %v", err)
	}
	if evidence.CartStatus != "checked_out" || evidence.OrderStatus != "pending_payment" ||
		evidence.InvoiceStatus != "pending_payment" || evidence.Reservation != "held" ||
		evidence.ReservedUnits != 2 || evidence.MarketingStatus != marketing.ReservationStatusConsumed {
		t.Fatalf("checkout evidence=%+v", evidence)
	}
	plainKey, err := scenario.database.GetValue(scenario.ctx, `
SELECT COUNT(*) FROM plugin_idc_core_commerce_order
WHERE checkout_idempotency_digest = ?`, request.IdempotencyKey)
	if err != nil || plainKey.Int64() != 0 {
		t.Fatalf("raw idempotency key persistence count=%d err=%v", plainKey.Int64(), err)
	}
}

func TestCommerceCheckoutUsesSimpleProductInventoryWithoutServerFulfillmentUnitsPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenarioWithSetup(
		t, "customer-commerce-service-1", 101, 2, true, 0,
		func(ctx context.Context, database gdb.DB) {
			if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET tracking_enabled=TRUE,sellable_quantity=5,version=1
WHERE version_no='version-commerce-service-1'`); err != nil {
				t.Fatal(err)
			}
		},
	)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-generic-inventory-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		InventorySnapshots, Reservations, LegacyFulfillments int
		VersionNo                                            string
		InventoryVersion                                     int64
		ReservedQuantity                                     int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT
(SELECT count(*) FROM plugin_idc_core_commerce_order_item_inventory_snapshot WHERE order_item_no=item.order_item_no) inventory_snapshots,
(SELECT count(*) FROM plugin_idc_core_commerce_inventory_reservation WHERE order_item_no=item.order_item_no AND version_no IS NOT NULL) reservations,
(SELECT count(*) FROM plugin_idc_core_commerce_fulfillment_unit WHERE order_item_no=item.order_item_no) legacy_fulfillments,
COALESCE(reservation.version_no,'') version_no,COALESCE(reservation.inventory_version,0) inventory_version,
COALESCE(reservation.reserved_quantity,0) reserved_quantity
FROM plugin_idc_core_commerce_order_item item
LEFT JOIN plugin_idc_core_commerce_inventory_reservation reservation ON reservation.order_item_no=item.order_item_no
WHERE item.order_no=?`, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.InventorySnapshots != 1 || evidence.Reservations != 1 || evidence.LegacyFulfillments != 0 ||
		evidence.VersionNo != "version-commerce-service-1" || evidence.InventoryVersion != 1 || evidence.ReservedQuantity != 2 {
		t.Fatalf("generic checkout inventory evidence=%+v", evidence)
	}
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	seedFinancialSettlementMerchant(t, scenario)
	balance, _ := billingservice.NewBalanceService(scenario.database)
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billingservice.ConfirmedRechargeInput{
		CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: checkout.TotalAmountMinor,
		SourceReference: "generic-inventory-recharge", SourceSnapshotDigest: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	}); err != nil {
		t.Fatal(err)
	}
	invoices, _ := billingservice.NewInvoiceService(scenario.database)
	if _, err = invoices.PayFromBalance(scenario.ctx, billingservice.InvoiceBalancePaymentInput{
		InvoiceNo: checkout.InvoiceNo, SourceReference: "generic-inventory-payment",
		SourceSnapshotDigest: "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
	}); err != nil {
		t.Fatal(err)
	}
	settled, err := newFinancialSettlementTestService(t, scenario).SettleOrder(scenario.ctx, checkout.OrderNo)
	if err != nil || settled == nil || settled.OrderNo != checkout.OrderNo {
		t.Fatalf("generic inventory settlement=%+v err=%v", settled, err)
	}
}

func TestCommerceCheckoutSimpleInventoryIsConsistentAcrossServiceInstancesPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenarioWithSetup(
		t, "customer-commerce-service-1", 101, 4, false, 0,
		func(ctx context.Context, database gdb.DB) {
			if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET tracking_enabled=TRUE,sellable_quantity=5,version=1
WHERE version_no='version-commerce-service-1'`); err != nil {
				t.Fatal(err)
			}
		},
	)
	secondDatabase := newIndependentCommerceTestDatabase(t, scenario.ctx, scenario.database)
	secondReader, err := catalog.NewPublicCatalogReader(secondDatabase, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	secondResolver, err := catalog.PurchaseResolverFromPublicReader(secondReader)
	if err != nil {
		t.Fatal(err)
	}
	secondService, secondQuote := newCheckoutCustomer(
		t, scenario.ctx, secondDatabase, secondResolver, scenario.purchase,
		"customer-commerce-service-2", 102, 4, false,
	)

	type checkoutAttempt struct {
		result *Checkout
		err    error
	}
	start := make(chan struct{})
	attempts := make(chan checkoutAttempt, 2)
	go func() {
		<-start
		result, checkoutErr := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
			QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-multi-instance-first-0001",
		})
		attempts <- checkoutAttempt{result: result, err: checkoutErr}
	}()
	go func() {
		<-start
		result, checkoutErr := secondService.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
			QuoteNo: secondQuote.QuoteNo, IdempotencyKey: "checkout-multi-instance-second-0001",
		})
		attempts <- checkoutAttempt{result: result, err: checkoutErr}
	}()
	close(start)
	first, second := <-attempts, <-attempts
	successes := 0
	for _, attempt := range []checkoutAttempt{first, second} {
		if attempt.err == nil && attempt.result != nil {
			successes++
			continue
		}
		if !bizerr.Is(attempt.err, CodeQuoteNotAvailable) {
			t.Fatalf("unexpected competing checkout result=%+v err=%v", attempt.result, attempt.err)
		}
	}
	if successes != 1 {
		t.Fatalf("competing service instances succeeded %d times", successes)
	}
	var evidence struct {
		Orders, HeldQuantity int
	}
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT
(SELECT count(*) FROM plugin_idc_core_commerce_order) orders,
(SELECT COALESCE(sum(reserved_quantity),0) FROM plugin_idc_core_commerce_inventory_reservation
 WHERE version_no='version-commerce-service-1' AND status_code='held') held_quantity`); err != nil {
		t.Fatal(err)
	}
	if evidence.Orders != 1 || evidence.HeldQuantity != 4 {
		t.Fatalf("multi-instance inventory evidence=%+v", evidence)
	}
}

func TestCommerceCheckoutAllowsUntrackedSimpleProductWithoutReservationPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenarioWithSetup(
		t, "customer-commerce-service-1", 101, 3, false, 0,
		func(ctx context.Context, database gdb.DB) {
			if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory
SET tracking_enabled=FALSE,sellable_quantity=0,version=1
WHERE version_no='version-commerce-service-1'`); err != nil {
				t.Fatal(err)
			}
		},
	)
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-untracked-inventory-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct{ InventorySnapshots, Reservations, LegacyFulfillments int }
	if err = scenario.database.GetScan(scenario.ctx, &evidence, `SELECT
(SELECT count(*) FROM plugin_idc_core_commerce_order_item_inventory_snapshot snapshot JOIN plugin_idc_core_commerce_order_item item ON item.order_item_no=snapshot.order_item_no WHERE item.order_no=?) inventory_snapshots,
(SELECT count(*) FROM plugin_idc_core_commerce_inventory_reservation WHERE order_no=?) reservations,
(SELECT count(*) FROM plugin_idc_core_commerce_fulfillment_unit WHERE order_no=?) legacy_fulfillments`, checkout.OrderNo, checkout.OrderNo, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.InventorySnapshots != 1 || evidence.Reservations != 0 || evidence.LegacyFulfillments != 0 {
		t.Fatalf("untracked generic checkout evidence=%+v", evidence)
	}
}

func TestCommerceGenericPhoneCompletesCartQuoteAndCheckoutPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	reader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	bindTestPublicBillingLabels(t, reader)
	configuration, err := reader.GetPurchaseConfiguration(ctx, "generic-phone")
	if err != nil || configuration.ConfigurationRevision == "" || len(configuration.Fields) != 0 {
		t.Fatalf("phone configuration=%+v err=%v", configuration, err)
	}
	quoted, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "generic-phone", ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     "CNY",
	})
	if err != nil || quoted == nil || len(quoted.Terms) != 1 {
		t.Fatalf("phone quote=%+v err=%v", quoted, err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	purchase := catalog.PurchaseConfigurationInput{
		PublicSlug: "generic-phone", ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quoted.Terms[0].BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1,
	}
	resolved, err := resolver.ResolvePurchaseConfiguration(ctx, purchase)
	if err != nil || resolved == nil {
		t.Fatalf("resolve fixed phone purchase=%+v err=%v", resolved, err)
	}
	service, finalQuote := newCheckoutCustomer(t, ctx, database, resolver, purchase,
		"customer-commerce-service-1", 101, 1, false)
	checkout, err := service.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: finalQuote.QuoteNo, IdempotencyKey: "checkout-generic-phone-0001",
	})
	if err != nil {
		t.Fatal(err)
	}
	var evidence struct {
		SelectedOptions, InventorySnapshots, Reservations, LegacyFulfillments int
	}
	if err = database.GetScan(ctx, &evidence, `SELECT
(SELECT selected_option_count FROM plugin_idc_core_commerce_configuration_snapshot WHERE configuration_snapshot_no=item.configuration_snapshot_no) selected_options,
(SELECT count(*) FROM plugin_idc_core_commerce_order_item_inventory_snapshot WHERE order_item_no=item.order_item_no) inventory_snapshots,
(SELECT count(*) FROM plugin_idc_core_commerce_inventory_reservation WHERE order_item_no=item.order_item_no) reservations,
(SELECT count(*) FROM plugin_idc_core_commerce_fulfillment_unit WHERE order_item_no=item.order_item_no) legacy_fulfillments
FROM plugin_idc_core_commerce_order_item item WHERE item.order_no=?`, checkout.OrderNo); err != nil {
		t.Fatal(err)
	}
	if evidence.SelectedOptions != 0 || evidence.InventorySnapshots != 1 ||
		evidence.Reservations != 0 || evidence.LegacyFulfillments != 0 {
		t.Fatalf("generic phone checkout evidence=%+v", evidence)
	}
}

func TestCommerceCheckoutOutboxFailureRollsBackPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, true, 0)
	scenario.service.outbox = failingCheckoutEventWriter{}
	_, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-rollback-0001",
	})
	if !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("injected outbox failure error=%v", err)
	}
	for _, table := range []string{
		"plugin_idc_core_commerce_order",
		"plugin_idc_core_commerce_order_item",
		"plugin_idc_core_billing_invoice",
		"plugin_idc_core_billing_invoice_item",
		"plugin_idc_core_commerce_inventory_reservation",
		"plugin_idc_core_commerce_fulfillment_unit",
		"plugin_idc_core_outbox_event",
	} {
		assertCheckoutCount(t, scenario, table, 0)
	}
	status, err := scenario.database.GetValue(scenario.ctx, `
SELECT status_code FROM plugin_idc_core_commerce_cart WHERE latest_quote_no = ?`, scenario.quote.QuoteNo)
	if err != nil || status.String() != "active" {
		t.Fatalf("rolled-back cart status=%q err=%v", status.String(), err)
	}
	marketingStatus, err := scenario.database.GetValue(scenario.ctx, `
SELECT status_code FROM plugin_idc_core_marketing_campaign_reservation
WHERE quote_reference = ?`, scenario.quote.QuoteNo)
	if err != nil || marketingStatus.String() != marketing.ReservationStatusActive {
		t.Fatalf("rolled-back marketing status=%q err=%v", marketingStatus.String(), err)
	}
}

func TestCommerceCheckoutInventoryContentionPostgreSQL(t *testing.T) {
	first := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 1)
	second := newCheckoutCustomerOnDatabase(
		t, first, "customer-commerce-service-2", 102, 1,
	)

	start := make(chan struct{})
	results := make([]*Checkout, 2)
	errorsFound := make([]error, 2)
	var wait sync.WaitGroup
	services := []*serviceImpl{first.service, second.service}
	quotes := []*Quote{first.quote, second.quote}
	for index := range services {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			<-start
			results[index], errorsFound[index] = services[index].CheckoutAccountQuote(first.ctx, CheckoutRequest{
				QuoteNo: quotes[index].QuoteNo, IdempotencyKey: "checkout-stock-000" + string(rune('1'+index)),
			})
		}(index)
	}
	close(start)
	wait.Wait()
	succeeded, unavailable := 0, 0
	for index := range services {
		if errorsFound[index] == nil && results[index] != nil {
			succeeded++
		} else if bizerr.Is(errorsFound[index], CodeQuoteNotAvailable) {
			unavailable++
		} else {
			t.Fatalf("inventory contention %d result=%+v err=%v", index, results[index], errorsFound[index])
		}
	}
	if succeeded != 1 || unavailable != 1 {
		var debugRows []struct {
			OrderNo   string `orm:"order_no"`
			VersionNo string `orm:"version_no"`
			Quantity  int    `orm:"reserved_quantity"`
		}
		_ = first.database.GetScan(first.ctx, &debugRows, `
SELECT order_no, version_no, reserved_quantity
FROM plugin_idc_core_commerce_inventory_reservation ORDER BY id`)
		t.Fatalf("inventory contention succeeded=%d unavailable=%d rows=%+v", succeeded, unavailable, debugRows)
	}
	held, err := first.database.GetValue(first.ctx, `
SELECT COALESCE(SUM(reserved_quantity), 0)
FROM plugin_idc_core_commerce_inventory_reservation
WHERE version_no = 'version-commerce-service-1'
  AND status_code IN ('held', 'committed')`)
	if err != nil || held.Int64() != 1 {
		t.Fatalf("inventory held quantity=%d err=%v", held.Int64(), err)
	}
}

func TestCommerceCheckoutRejectsSupersededInventoryPostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	if _, err := scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory SET sellable_quantity=1,version=version+1 WHERE version_no='version-commerce-service-1'`); err != nil {
		t.Fatal(err)
	}
	_, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-stale-stock-0001",
	})
	if !bizerr.Is(err, CodeQuoteNotAvailable) {
		t.Fatalf("superseded inventory checkout error=%v", err)
	}
	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_order", 0)
	status, err := scenario.database.GetValue(scenario.ctx, `
SELECT status_code FROM plugin_idc_core_commerce_cart WHERE latest_quote_no = ?`, scenario.quote.QuoteNo)
	if err != nil || status.String() != "active" {
		t.Fatalf("superseded inventory cart status=%q err=%v", status.String(), err)
	}
}

func TestCommerceCheckoutRejectsSupersededQuotePostgreSQL(t *testing.T) {
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	fresh, err := scenario.service.QuoteAccountCart(scenario.ctx, QuoteRequest{
		ExpectedCartVersion: scenario.quote.CartVersion,
	})
	if err != nil || fresh == nil || fresh.QuoteNo == scenario.quote.QuoteNo {
		t.Fatalf("create superseding quote result=%+v err=%v", fresh, err)
	}
	_, err = scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{
		QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "checkout-stale-quote-0001",
	})
	if !bizerr.Is(err, CodeQuoteNotAvailable) {
		t.Fatalf("superseded quote checkout error=%v", err)
	}
	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_order", 0)
	var cart struct {
		StatusCode    string `orm:"status_code"`
		LatestQuoteNo string `orm:"latest_quote_no"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &cart, `
SELECT status_code, latest_quote_no
FROM plugin_idc_core_commerce_cart
WHERE cart_no = ?`, scenario.quote.CartNo); err != nil || cart.StatusCode != "active" ||
		cart.LatestQuoteNo != fresh.QuoteNo {
		t.Fatalf("superseded quote changed cart state=%+v err=%v", cart, err)
	}
}

func newCheckoutTestScenario(
	t *testing.T,
	customerNo string,
	userID int,
	quantity int,
	withMarketing bool,
	checkoutSellable int,
) checkoutTestScenario {
	return newCheckoutTestScenarioWithSetup(t, customerNo, userID, quantity, withMarketing, checkoutSellable, nil)
}

func newCheckoutTestScenarioWithSetup(
	t *testing.T,
	customerNo string,
	userID int,
	quantity int,
	withMarketing bool,
	checkoutSellable int,
	setup func(context.Context, gdb.DB),
) checkoutTestScenario {
	t.Helper()
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	if setup != nil {
		setup(ctx, database)
	}
	if checkoutSellable > 0 {
		if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_catalog_product_version_inventory SET tracking_enabled=TRUE,sellable_quantity=?,version=version+1 WHERE version_no='version-commerce-service-1'`, checkoutSellable); err != nil {
			t.Fatalf("set checkout product inventory: %v", err)
		}
	}
	publicReader, err := catalog.NewPublicCatalogReaderConfigured(
		database,
		[]byte("0123456789abcdef0123456789abcdef"),
		checkoutPurchaseRuntimeStub{},
		checkoutPurchaseFileVerifierStub{},
	)
	if err != nil {
		t.Fatalf("create checkout public reader: %v", err)
	}
	bindTestPublicBillingLabels(t, publicReader)
	resolver, err := catalog.PurchaseResolverFromPublicReader(publicReader)
	if err != nil {
		t.Fatalf("create checkout resolver: %v", err)
	}
	configuration, err := publicReader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil {
		causes := make([]string, 0, 4)
		for current := err; current != nil; current = errors.Unwrap(current) {
			causes = append(causes, current.Error())
		}
		t.Fatalf("load checkout configuration: %v", causes)
	}
	quoted, err := publicReader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     "CNY",
	})
	if err != nil || quoted == nil || len(quoted.Terms) != 1 {
		t.Fatalf("load checkout public quote result=%+v err=%v", quoted, err)
	}
	purchase := catalog.PurchaseConfigurationInput{
		PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision,
		BillingTermToken: quoted.Terms[0].BillingTermToken, CurrencyCode: "CNY",
		PurchaseCycles: 1,
	}
	scenario := checkoutTestScenario{
		ctx: ctx, database: database, resolver: resolver, purchase: purchase,
	}
	scenario.service, scenario.quote = newCheckoutCustomer(
		t, ctx, database, resolver, purchase, customerNo, userID, quantity, withMarketing,
	)
	return scenario
}

func newCheckoutCustomerOnDatabase(
	t *testing.T,
	scenario checkoutTestScenario,
	customerNo string,
	userID int,
	quantity int,
) checkoutTestScenario {
	t.Helper()
	result := scenario
	result.service, result.quote = newCheckoutCustomer(
		t, scenario.ctx, scenario.database, scenario.resolver, scenario.purchase,
		customerNo, userID, quantity, false,
	)
	return result
}

func newCheckoutCustomer(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	resolver catalog.PurchaseConfigurationResolver,
	purchase catalog.PurchaseConfigurationInput,
	customerNo string,
	userID int,
	quantity int,
	withMarketing bool,
) (*serviceImpl, *Quote) {
	t.Helper()
	principalStub := commercePrincipalStub{audience: &principal.Audience{
		UserID: userID, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: customerNo,
	}}
	settlements := newCheckoutFinancialSettlementService(t, database)
	service, err := newService(
		database, resolver, principalStub, rand.Reader, settlements,
	)
	if err != nil {
		t.Fatalf("create checkout service: %v", err)
	}
	service.contractPolicy = contractpolicy.NewResolver(checkoutContractTemplateSource{})
	if withMarketing {
		marketingService, marketingErr := marketing.New(database, marketing.Config{})
		if marketingErr != nil {
			t.Fatalf("create checkout marketing service: %v", marketingErr)
		}
		evaluator, evaluatorErr := NewMarketingQuoteAdjustmentEvaluator(marketingService)
		if evaluatorErr != nil {
			t.Fatalf("create checkout marketing evaluator: %v", evaluatorErr)
		}
		service.adjustments = evaluator
		service.reservations = evaluator.(QuoteReservationFinalizer)
		service.hasReservationFinalizer = true
	}
	cart, err := service.OpenAccountCart(ctx, "CNY")
	if err != nil {
		t.Fatalf("open checkout cart: %v", err)
	}
	cart, err = service.AddAccountItem(ctx, AddItemInput{
		ExpectedCartVersion: cart.Version, Quantity: quantity, Purchase: purchase,
	})
	if err != nil {
		t.Fatalf("add checkout cart item: %+v", err)
	}
	quote, err := service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: cart.Version})
	if err != nil {
		t.Fatalf("create checkout final quote: %v", err)
	}
	return service, quote
}

func newIndependentCommerceTestDatabase(t *testing.T, ctx context.Context, source gdb.DB) gdb.DB {
	t.Helper()
	value, err := source.GetValue(ctx, `SELECT current_schema()`)
	if err != nil || value.IsNil() {
		t.Fatalf("load commerce test schema: %v", err)
	}
	schema := value.String()
	dsn := strings.TrimSpace(os.Getenv(commerceIntegrationDSNEnv))
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatalf("create independent commerce database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })
	return database
}

func newCheckoutFinancialSettlementService(
	t *testing.T,
	database gdb.DB,
) billingservice.FinancialSettlementService {
	t.Helper()
	events, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatalf("create checkout settlement outbox: %v", err)
	}
	credits, err := billingservice.NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create checkout credit owner: %v", err)
	}
	settlements, err := billingservice.NewFinancialSettlementService(
		database, events, credits,
	)
	if err != nil {
		t.Fatalf("create checkout settlement service: %v", err)
	}
	return settlements
}

func assertCheckoutCount(t *testing.T, scenario checkoutTestScenario, table string, expected int64) {
	t.Helper()
	value, err := scenario.database.GetValue(scenario.ctx, `SELECT COUNT(*) FROM `+table)
	if err != nil || value.Int64() != expected {
		t.Fatalf("checkout table %s count=%d expected=%d err=%v", table, value.Int64(), expected, err)
	}
}
