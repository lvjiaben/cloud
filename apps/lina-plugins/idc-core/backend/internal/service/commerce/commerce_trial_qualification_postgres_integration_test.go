// This file verifies trial quote, hold, settlement, and financial isolation on PostgreSQL.
package commerce

import (
	"context"
	"crypto/rand"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	billingservice "lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestTrialQualificationCheckoutAndSettlementPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	seedCommerceTrialRule(t, ctx, database)
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_catalog_billing_rule
SET trial_quantity = 2
WHERE billing_rule_no = 'billing-commerce-service-trial'`); err != nil {
		t.Fatalf("expand trial quantity for multi-server settlement: %v", err)
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_individual_profile (
    customer_no, legal_name, legal_name_masked, identity_identifier_masked,
    identity_lookup_digest, identity_case_no, identity_status_code,
    identity_verified_at, identity_expires_at
) VALUES ('customer-commerce-service-1', 'Trial Customer', 'T**** C*******',
          '110***********001X', repeat('a', 64), 'trial-identity-case-001',
          'verified', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year')`); err != nil {
		t.Fatalf("insert verified trial identity: %v", err)
	}
	reader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create trial catalog reader: %v", err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatalf("create trial purchase resolver: %v", err)
	}
	events, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatalf("create trial settlement outbox: %v", err)
	}
	credits, err := billingservice.NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create trial settlement credit dependency: %v", err)
	}
	settlements, err := billingservice.NewFinancialSettlementService(database, events, credits)
	if err != nil {
		t.Fatalf("create trial financial settlement service: %v", err)
	}
	identity := commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-1", UserID: 101,
	}}
	service, err := newService(database, resolver, identity, rand.Reader, settlements)
	if err != nil {
		t.Fatalf("create trial commerce service: %v", err)
	}
	purchase := trialPurchaseConfiguration(t, ctx, reader)
	guest, err := service.CreateGuestCart(ctx, "CNY")
	if err != nil {
		t.Fatalf("create guest trial cart: %v", err)
	}
	if _, err = service.AddGuestItem(ctx, guest.Token, AddItemInput{
		ExpectedCartVersion: guest.Cart.Version, Quantity: 1, Purchase: purchase,
	}); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("guest customer-context trial item error=%v", err)
	}
	cart, err := service.OpenAccountCart(ctx, "CNY")
	if err != nil {
		t.Fatalf("open trial account cart: %v", err)
	}
	cart, err = service.AddAccountItem(ctx, AddItemInput{
		ExpectedCartVersion: cart.Version, Quantity: 2, Purchase: purchase,
	})
	if err != nil {
		t.Fatalf("add customer-context trial item: %v", err)
	}
	quote, err := service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: cart.Version})
	if err != nil || quote == nil || quote.TotalAmountMinor != 0 || len(quote.Items) != 1 {
		t.Fatalf("create final trial quote result=%+v err=%v", quote, err)
	}
	checkout, err := service.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: quote.QuoteNo, IdempotencyKey: "trial-checkout-settlement-0001",
	})
	if err != nil || checkout == nil || checkout.TotalAmountMinor != 0 {
		t.Fatalf("checkout and settle trial result=%+v err=%v", checkout, err)
	}
	assertTrialSettlementProjection(t, ctx, database, checkout, 2)
	replayed, err := service.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: quote.QuoteNo, IdempotencyKey: "trial-checkout-settlement-0001",
	})
	if err != nil || replayed == nil || replayed.OrderNo != checkout.OrderNo || !replayed.Reused {
		t.Fatalf("replay settled trial result=%+v err=%v", replayed, err)
	}
	replaceCommerceTrialProductVersion(t, ctx, database)
	purchase = trialPurchaseConfiguration(t, ctx, reader)
	second, err := service.OpenAccountCart(ctx, "CNY")
	if err != nil {
		t.Fatalf("open second trial cart: %v", err)
	}
	second, err = service.AddAccountItem(ctx, AddItemInput{
		ExpectedCartVersion: second.Version, Quantity: 1, Purchase: purchase,
	})
	if err != nil {
		t.Fatalf("add second trial item: %v", err)
	}
	if _, err = service.QuoteAccountCart(ctx, QuoteRequest{
		ExpectedCartVersion: second.Version,
	}); !bizerr.Is(err, CodeQuoteNotAvailable) {
		t.Fatalf("reused customer-and-product trial quote error=%v", err)
	}
}

func TestTrialQualificationCancellationReleasesHeldRightPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	seedCommerceTrialRule(t, ctx, database)
	reader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create release trial catalog reader: %v", err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatalf("create release trial resolver: %v", err)
	}
	events, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatalf("create release trial outbox: %v", err)
	}
	credits, err := billingservice.NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create release trial credit dependency: %v", err)
	}
	settlements, err := billingservice.NewFinancialSettlementService(database, events, credits)
	if err != nil {
		t.Fatalf("create release trial settlement service: %v", err)
	}
	identity := commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-2", UserID: 202,
	}}
	service, err := newService(database, resolver, identity, rand.Reader, settlements)
	if err != nil {
		t.Fatalf("create release trial commerce service: %v", err)
	}
	purchase := trialPurchaseConfiguration(t, ctx, reader)
	cart, err := service.OpenAccountCart(ctx, "CNY")
	if err != nil {
		t.Fatalf("open release trial cart: %v", err)
	}
	cart, err = service.AddAccountItem(ctx, AddItemInput{
		ExpectedCartVersion: cart.Version, Quantity: 1, Purchase: purchase,
	})
	if err != nil {
		t.Fatalf("add release trial item: %v", err)
	}
	quote, err := service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: cart.Version})
	if err != nil {
		t.Fatalf("quote release trial cart: %v", err)
	}
	if _, err = service.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: quote.QuoteNo, IdempotencyKey: "trial-held-before-cancel-0001",
	}); !bizerr.Is(err, billingservice.CodeSettlementNotEligible) {
		t.Fatalf("trial without identity settlement error=%v", err)
	}
	orderNo, err := database.GetValue(ctx, `
SELECT order_no FROM plugin_idc_core_commerce_order WHERE quote_no = ?`, quote.QuoteNo)
	if err != nil || orderNo.String() == "" {
		t.Fatalf("load held trial order=%q err=%v", orderNo.String(), err)
	}
	if _, err = service.CancelAccountOrder(ctx, OrderCancellationRequest{
		OrderNo: orderNo.String(), IdempotencyKey: "trial-held-cancellation-0001",
	}); err != nil {
		t.Fatalf("cancel held trial order: %v", err)
	}
	var released struct {
		Held, Consumed int
		Status         string
		ReleaseMoves   int
	}
	if err = database.GetScan(ctx, &released, `
SELECT scope.held_quantity AS held, scope.consumed_quantity AS consumed,
       entitlement.status_code AS status,
       (SELECT COUNT(*) FROM plugin_idc_core_trial_qualification_movement
         WHERE order_no = ? AND movement_type_code = 'release')::INT AS release_moves
FROM plugin_idc_core_trial_qualification_scope scope
JOIN plugin_idc_core_trial_entitlement entitlement
  ON entitlement.scope_no = scope.scope_no
WHERE entitlement.order_no = ?`, orderNo.String(), orderNo.String()); err != nil {
		t.Fatalf("load released trial projection: %v", err)
	}
	if released.Held != 0 || released.Consumed != 0 ||
		released.Status != "released" || released.ReleaseMoves != 1 {
		t.Fatalf("released trial projection is inconsistent: %+v", released)
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_individual_profile (
    customer_no, legal_name, legal_name_masked, identity_identifier_masked,
    identity_lookup_digest, identity_case_no, identity_status_code,
    identity_verified_at, identity_expires_at
) VALUES ('customer-commerce-service-2', 'Released Trial Customer', 'R******* T****',
          '110***********002X', repeat('b', 64), 'trial-identity-case-002',
          'verified', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year')`); err != nil {
		t.Fatalf("insert released trial identity: %v", err)
	}
	second, err := service.OpenAccountCart(ctx, "CNY")
	if err != nil {
		t.Fatalf("open trial cart after release: %v", err)
	}
	second, err = service.AddAccountItem(ctx, AddItemInput{
		ExpectedCartVersion: second.Version, Quantity: 1, Purchase: purchase,
	})
	if err != nil {
		t.Fatalf("add trial after release: %v", err)
	}
	secondQuote, err := service.QuoteAccountCart(ctx, QuoteRequest{ExpectedCartVersion: second.Version})
	if err != nil {
		t.Fatalf("quote trial after release: %v", err)
	}
	if _, err = service.CheckoutAccountQuote(ctx, CheckoutRequest{
		QuoteNo: secondQuote.QuoteNo, IdempotencyKey: "trial-after-release-0001",
	}); err != nil {
		t.Fatalf("checkout trial after release: %v", err)
	}
}

func TestTrialQualificationConcurrentAssistedCheckoutsAllowOnePostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	applyAssistedQuoteTestMigration(t, ctx, database)
	seedCommerceServiceCatalog(t, ctx, database)
	seedCommerceTrialRule(t, ctx, database)
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_individual_profile (
    customer_no, legal_name, legal_name_masked, identity_identifier_masked,
    identity_lookup_digest, identity_case_no, identity_status_code,
    identity_verified_at, identity_expires_at
) VALUES ('customer-commerce-service-1', 'Concurrent Trial Customer', 'C********* T****',
          '110***********003X', repeat('c', 64), 'trial-identity-case-003',
          'verified', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP + INTERVAL '1 year')`); err != nil {
		t.Fatalf("insert concurrent trial identity: %v", err)
	}
	reader, err := catalog.NewPublicCatalogReader(database, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatalf("create concurrent trial catalog reader: %v", err)
	}
	resolver, err := catalog.PurchaseResolverFromPublicReader(reader)
	if err != nil {
		t.Fatalf("create concurrent trial resolver: %v", err)
	}
	events, err := outbox.NewEnqueuer(database, outbox.Config{})
	if err != nil {
		t.Fatalf("create concurrent trial outbox: %v", err)
	}
	credits, err := billingservice.NewCreditUsageService(database)
	if err != nil {
		t.Fatalf("create concurrent trial credit dependency: %v", err)
	}
	settlements, err := billingservice.NewFinancialSettlementService(database, events, credits)
	if err != nil {
		t.Fatalf("create concurrent trial settlement service: %v", err)
	}
	commerceService, err := newService(database, resolver, commercePrincipalStub{audience: &principal.Audience{
		Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive,
		CustomerNo: "customer-commerce-service-1", UserID: 101,
	}}, rand.Reader, settlements)
	if err != nil {
		t.Fatalf("create concurrent trial commerce service: %v", err)
	}
	purchase := trialPurchaseConfiguration(t, ctx, reader)
	checkoutServices := make([]AssistedQuoteService, 2)
	for index := range checkoutServices {
		checkoutServices[index], err = NewAssistedQuoteService(
			database, resolver, commerceService, &assistedQuoteAuditStub{},
			&assistedQuoteAuthorizerStub{actor: 1003 + index},
			[]byte("0123456789abcdef0123456789abcdef"),
		)
		if err != nil {
			t.Fatalf("create concurrent checkout service %d: %v", index, err)
		}
	}
	results := make([]*AssistedQuoteRequest, 2)
	errorsFound := make([]error, 2)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(2)
	for index := range checkoutServices {
		go func(index int) {
			defer workers.Done()
			<-start
			results[index], errorsFound[index] = checkoutServices[index].CreateAssistedQuote(ctx, AssistedQuoteCreateInput{
				CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", PaymentDueAt: time.Now().Add(time.Hour).UnixMilli(), Reason: "concurrent trial", IdempotencyKey: []string{"trial-direct-create-1", "trial-direct-create-2"}[index], Items: []AssistedQuoteItemInput{{Quantity: 1, Purchase: purchase}},
			})
		}(index)
	}
	close(start)
	workers.Wait()
	successes, failures := 0, 0
	for index := range results {
		if errorsFound[index] == nil && results[index] != nil &&
			results[index].StatusCode == AssistedQuoteStatusOrdered {
			successes++
		} else if errorsFound[index] != nil {
			failures++
		} else {
			t.Fatalf("unexpected concurrent trial checkout %d result=%+v err=%v",
				index, results[index], errorsFound[index])
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent trial checkout successes=%d failures=%d", successes, failures)
	}
	var projection struct {
		Held, Consumed, Orders, Settlements int
	}
	if err = database.GetScan(ctx, &projection, `
SELECT scope.held_quantity AS held, scope.consumed_quantity AS consumed,
       (SELECT COUNT(*) FROM plugin_idc_core_commerce_order o
         JOIN plugin_idc_core_commerce_order_item i ON i.order_no = o.order_no
        WHERE o.customer_no = 'customer-commerce-service-1'
          AND i.charge_type_code = 'trial')::INT AS orders,
       (SELECT COUNT(*) FROM plugin_idc_core_financial_settlement
        WHERE customer_no = 'customer-commerce-service-1'
          AND basis_code = 'trial')::INT AS settlements
FROM plugin_idc_core_trial_qualification_scope scope
WHERE scope.customer_no = 'customer-commerce-service-1'
  AND scope.product_no = 'product-commerce-service'`); err != nil {
		t.Fatalf("load concurrent trial qualification projection: %v", err)
	}
	if projection.Held != 0 || projection.Consumed != 1 ||
		projection.Orders != 1 || projection.Settlements != 1 {
		t.Fatalf("concurrent trial qualification projection=%+v", projection)
	}
}

func TestTrialQualificationMigrationIsIdempotentAndProjectionClosedPostgreSQL(t *testing.T) {
	ctx, database := newCommerceServiceTestDatabase(t)
	seedCommerceServiceCatalog(t, ctx, database)
	var shape struct {
		Tables, SnapshotColumns, ProjectionTriggers, UnitColumns, LegacyColumns int
	}
	if err := database.GetScan(ctx, &shape, `
SELECT
  (SELECT COUNT(*) FROM information_schema.tables
    WHERE table_schema = current_schema()
      AND table_name IN (
        'plugin_idc_core_trial_qualification_scope',
        'plugin_idc_core_trial_entitlement',
        'plugin_idc_core_trial_qualification_movement'
      ))::INT AS tables,
  (SELECT COUNT(*) FROM information_schema.columns
    WHERE table_schema = current_schema()
      AND table_name IN (
        'plugin_idc_core_commerce_quote_item_snapshot',
        'plugin_idc_core_commerce_order_item'
      )
      AND column_name IN ('trial_limit_quantity', 'prior_trial_uses'))::INT AS snapshot_columns,
  (SELECT COUNT(*) FROM information_schema.columns
    WHERE table_schema=current_schema()
      AND table_name='plugin_idc_core_trial_entitlement'
      AND column_name='unit_sequence')::INT AS unit_columns,
  (SELECT COUNT(*) FROM information_schema.columns
    WHERE table_schema=current_schema()
      AND table_name='plugin_idc_core_trial_entitlement'
      AND column_name='fulfillment_no')::INT AS legacy_columns,
  (SELECT COUNT(DISTINCT trigger_name) FROM information_schema.triggers
    WHERE trigger_schema = current_schema()
      AND trigger_name IN (
        'trg_idc_trial_scope_projection',
        'trg_idc_trial_entitlement_projection',
        'trg_idc_trial_movement_projection'
      ))::INT AS projection_triggers`); err != nil {
		t.Fatalf("load trial qualification migration shape: %v", err)
	}
	if shape.Tables != 3 || shape.SnapshotColumns != 4 || shape.ProjectionTriggers != 3 ||
		shape.UnitColumns != 1 || shape.LegacyColumns != 0 {
		t.Fatalf("trial qualification migration shape=%+v", shape)
	}
	err := database.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		_, insertErr := tx.Ctx(ctx).Exec(`
INSERT INTO plugin_idc_core_trial_qualification_scope
    (scope_no, customer_no, product_no)
VALUES ('TRS_orphan_projection', 'customer-commerce-service-1',
        'product-commerce-service')`)
		return insertErr
	})
	if err == nil {
		t.Fatal("trial qualification projection accepted a scope without rights and movement evidence")
	}
}

func trialPurchaseConfiguration(
	t *testing.T,
	ctx context.Context,
	reader catalog.CatalogReader,
) catalog.PurchaseConfigurationInput {
	t.Helper()
	bindTestPublicBillingLabels(t, reader)
	configuration, err := reader.GetPurchaseConfiguration(ctx, "commerce-service-cloud")
	if err != nil {
		t.Fatalf("load trial configuration: %v", err)
	}
	quote, err := reader.QuotePurchaseConfiguration(ctx, catalog.PublicPurchaseConfigurationQuoteInput{
		PublicPurchaseConfigurationInput: catalog.PublicPurchaseConfigurationInput{PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision},
		CurrencyCode:                     "CNY",
	})
	if err != nil {
		t.Fatalf("load customer-context trial term: %v", err)
	}
	for _, term := range quote.Terms {
		if term.Status == catalog.PublicQuoteRequiresCustomerContext {
			return catalog.PurchaseConfigurationInput{
				PublicSlug: "commerce-service-cloud", ConfigurationRevision: configuration.ConfigurationRevision,
				BillingTermToken: term.BillingTermToken, CurrencyCode: "CNY", PurchaseCycles: 1,
			}
		}
	}
	t.Fatalf("customer-context trial term is missing: %+v", quote.Terms)
	return catalog.PurchaseConfigurationInput{}
}

func replaceCommerceTrialProductVersion(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) {
	t.Helper()
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_catalog_product_version
SET status_code = 'superseded'
WHERE version_no = 'version-commerce-service-1';
INSERT INTO plugin_idc_core_catalog_product_version
    (version_no, product_no, version_number, status_code, source_code, published_at)
VALUES ('version-commerce-service-2', 'product-commerce-service', 2,
		'draft', 'manual', NULL);
INSERT INTO plugin_idc_core_catalog_product_version_inventory
    (version_no, tracking_enabled, sellable_quantity, version)
VALUES ('version-commerce-service-2', FALSE, 0, 1);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy
    (version_no, strategy_code, granularity_code, ongoing_management,
     maximum_attempts, retry_base_seconds, policy_digest, created_by)
VALUES ('version-commerce-service-2', 'manual', 'per_order_item', FALSE,
        10, 30, repeat('e', 64), 1);
INSERT INTO plugin_idc_core_catalog_billing_rule
    (billing_rule_no, version_no, rule_code, charge_type_code, period_unit_code,
     period_count, price_phase_code, quantity_mode_code, trial_quantity, status_code)
VALUES ('billing-commerce-service-trial-v2', 'version-commerce-service-2', 'trial-7d',
        'trial', 'day', 7, 'new_purchase', 'fixed', 1, 'active');
INSERT INTO plugin_idc_core_catalog_price
    (price_no, version_no, billing_rule_no, option_no, currency_code,
     amount_minor, setup_fee_minor, renewal_amount_minor, status_code)
VALUES
    ('price-commerce-service-trial-v2-base', 'version-commerce-service-2',
     'billing-commerce-service-trial-v2', NULL, 'CNY', 0, 0, NULL, 'active');
INSERT INTO plugin_idc_core_catalog_tax_policy
    (tax_policy_no, version_no, currency_code, price_includes_tax,
     tax_treatment_code, tax_rate_basis_points, tax_category_code,
     rounding_scope_code, rounding_mode_code, rounding_quantum_minor)
VALUES ('tax-commerce-service-v2-cny', 'version-commerce-service-2', 'CNY', FALSE,
        'taxable', 600, 'cloud_service', 'line', 'half_up', 1);
UPDATE plugin_idc_core_catalog_product_version
SET status_code='published',published_at=CURRENT_TIMESTAMP
WHERE version_no='version-commerce-service-2';
UPDATE plugin_idc_core_catalog_product
SET current_version_no = 'version-commerce-service-2'
WHERE product_no = 'product-commerce-service'`); err != nil {
		t.Fatalf("replace commerce trial product version: %v", err)
	}
}

func seedCommerceTrialRule(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_catalog_billing_rule
    (billing_rule_no, version_no, rule_code, charge_type_code, period_unit_code,
     period_count, price_phase_code, quantity_mode_code, trial_quantity, status_code)
VALUES ('billing-commerce-service-trial', 'version-commerce-service-1', 'trial-7d',
        'trial', 'day', 7, 'new_purchase', 'fixed', 1, 'active');
INSERT INTO plugin_idc_core_catalog_price
    (price_no, version_no, billing_rule_no, option_no, currency_code,
     amount_minor, setup_fee_minor, renewal_amount_minor, status_code)
VALUES
    ('price-commerce-service-trial-base', 'version-commerce-service-1',
	 'billing-commerce-service-trial', NULL, 'CNY', 0, 0, NULL, 'active')`); err != nil {
		t.Fatalf("seed commerce trial billing rule: %v", err)
	}
}

func assertTrialSettlementProjection(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	checkout *Checkout,
	expectedConsumedQuantity int,
) {
	t.Helper()
	var projection struct {
		OrderStatus, InvoiceStatus, BasisCode, SourceType, EntitlementStatus string
		HeldQuantity, ConsumedQuantity, EntitlementCount                     int
		HoldMovements, ConsumeMovements                                      int
		LedgerTransactions, PaymentAllocations, BalanceSnapshots             int
	}
	if err := database.GetScan(ctx, &projection, `
SELECT o.status_code AS order_status, i.status_code AS invoice_status,
       s.basis_code, source.source_type_code AS source_type,
       entitlement.status_code AS entitlement_status,
       scope.held_quantity, scope.consumed_quantity,
       (SELECT COUNT(*) FROM plugin_idc_core_trial_entitlement
         WHERE order_no = o.order_no)::INT AS entitlement_count,
       (SELECT COUNT(*) FROM plugin_idc_core_trial_qualification_movement
         WHERE order_no = o.order_no AND movement_type_code = 'hold')::INT AS hold_movements,
       (SELECT COUNT(*) FROM plugin_idc_core_trial_qualification_movement
         WHERE order_no = o.order_no AND movement_type_code = 'consume')::INT AS consume_movements,
       (SELECT COUNT(*) FROM plugin_idc_core_billing_ledger_transaction)::INT
           AS ledger_transactions,
       (SELECT COUNT(*) FROM plugin_idc_core_billing_payment_allocation)::INT
           AS payment_allocations,
       (SELECT COUNT(*) FROM plugin_idc_core_billing_balance_snapshot)::INT
           AS balance_snapshots
FROM plugin_idc_core_commerce_order o
JOIN plugin_idc_core_billing_invoice i ON i.order_no = o.order_no
JOIN plugin_idc_core_financial_settlement s ON s.order_no = o.order_no
JOIN plugin_idc_core_financial_settlement_source source
  ON source.settlement_no = s.settlement_no
JOIN plugin_idc_core_trial_entitlement entitlement
  ON entitlement.order_no = o.order_no
JOIN plugin_idc_core_trial_qualification_scope scope
  ON scope.scope_no = entitlement.scope_no
WHERE o.order_no = ?`, checkout.OrderNo); err != nil {
		t.Fatalf("load trial settlement projection: %v", err)
	}
	if projection.OrderStatus != "paid" || projection.InvoiceStatus != "paid" ||
		projection.BasisCode != "trial" || projection.SourceType != "trial" ||
		projection.EntitlementStatus != "consumed" || projection.HeldQuantity != 0 ||
		projection.ConsumedQuantity != expectedConsumedQuantity ||
		projection.EntitlementCount != expectedConsumedQuantity || projection.HoldMovements != 1 ||
		projection.ConsumeMovements != 1 || projection.LedgerTransactions != 0 ||
		projection.PaymentAllocations != 0 || projection.BalanceSnapshots != 0 {
		t.Fatalf("trial settlement projection is inconsistent: %+v", projection)
	}
}
