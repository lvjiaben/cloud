package commerce

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type structuredCheckoutFreezerStub struct{}

func (structuredCheckoutFreezerStub) FreezePurchaseConfiguration(
	context.Context,
	catalog.PublicPurchaseConfigurationInput,
) (*catalog.FrozenPurchaseConfiguration, error) {
	return nil, nil
}

type structuredBindingFreezerStub struct {
	frozen *catalog.FrozenPurchaseConfiguration
}

func (stub structuredBindingFreezerStub) FreezePurchaseConfiguration(
	context.Context,
	catalog.PublicPurchaseConfigurationInput,
) (*catalog.FrozenPurchaseConfiguration, error) {
	return stub.frozen, nil
}

func TestFreezeQuoteConfigurationRejectsValueAndPriceSelectionSwapPostgreSQL(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		swapPrice     bool
		swapValues    bool
		expectSuccess bool
	}{
		{name: "matching evidence", expectSuccess: true},
		{name: "different customer values", swapValues: true},
		{name: "different price selection", swapPrice: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
			quoteItem := loadStructuredQuoteItem(t, scenario, scenario.quote.QuoteNo)
			var bound struct {
				PriceSelectionDigest string `orm:"selection_digest"`
				CustomerValueDigest  string `orm:"customer_value_digest"`
			}
			if err := scenario.database.GetScan(scenario.ctx, &bound, `SELECT configuration.selection_digest,configuration.customer_value_digest
FROM plugin_idc_core_commerce_quote_item_snapshot item
JOIN plugin_idc_core_commerce_configuration_snapshot configuration
  ON configuration.configuration_snapshot_no=item.configuration_snapshot_no
WHERE item.quote_item_no=?`, quoteItem.QuoteItemNo); err != nil {
				t.Fatal(err)
			}
			if testCase.swapPrice {
				bound.PriceSelectionDigest = strings.Repeat("d", 64)
			}
			if testCase.swapValues {
				bound.CustomerValueDigest = strings.Repeat("e", 64)
			}
			consumer, err := purchaseconfigurationfile.NewOrderFileConsumer(scenario.database, scenario.service.principal)
			if err != nil {
				t.Fatal(err)
			}
			freezer := structuredBindingFreezerStub{frozen: &catalog.FrozenPurchaseConfiguration{
				ProductNo: quoteItem.ProductNo, VersionNo: quoteItem.VersionNo,
				ConfigurationRevision: "cfg-bound-1", InterfaceInstanceNo: "pfi_11111111111111111111111111111111",
				InterfaceConfigurationRevision: 1, CustomerDeclarationDigest: strings.Repeat("8", 64),
				CandidateSnapshotDigest: strings.Repeat("9", 64), CustomerValueDigest: bound.CustomerValueDigest,
				PriceSelectionDigest: bound.PriceSelectionDigest, CandidateSnapshotJSON: []byte("{}"), TypedValueSnapshotJSON: []byte("{}"),
			}}
			if err = BindStructuredConfigurationSnapshots(scenario.service, freezer, consumer); err != nil {
				t.Fatal(err)
			}
			result, err := scenario.service.FreezeAccountQuoteItemConfiguration(scenario.ctx, FreezeQuoteItemConfigurationInput{QuoteItemNo: quoteItem.QuoteItemNo})
			if testCase.expectSuccess {
				if err != nil || result == nil || !result.ConfigurationFrozen {
					t.Fatalf("matching configuration was not frozen: result=%+v err=%v", result, err)
				}
				return
			}
			if !bizerr.Is(err, CodeConflict) || result != nil {
				t.Fatalf("configuration swap result=%+v err=%v", result, err)
			}
			count, countErr := scenario.database.GetValue(scenario.ctx, `SELECT count(*) FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE quote_item_no=?`, quoteItem.QuoteItemNo)
			if countErr != nil || count.Int() != 0 {
				t.Fatalf("rejected swap persisted snapshot count=%d err=%v", count.Int(), countErr)
			}
		})
	}
}

func TestStructuredConfigurationCheckoutBindsAndConsumesInOnePostgreSQLTransaction(t *testing.T) {
	scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)

	var quoteItem struct {
		QuoteItemNo string `orm:"quote_item_no"`
		ProductNo   string `orm:"product_no"`
		VersionNo   string `orm:"version_no"`
	}
	if err := scenario.database.GetScan(scenario.ctx, &quoteItem, `
SELECT quote_item_no,product_no,version_no
FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_no=?`, scenario.quote.QuoteNo); err != nil {
		t.Fatalf("load structured checkout quote item: %v", err)
	}
	seedStructuredCheckoutEvidence(t, scenario, quoteItem.QuoteItemNo, quoteItem.ProductNo, quoteItem.VersionNo)

	consumer, err := purchaseconfigurationfile.NewOrderFileConsumer(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatalf("create checkout file consumer: %v", err)
	}
	if err = BindStructuredConfigurationSnapshots(scenario.service, structuredCheckoutFreezerStub{}, consumer); err != nil {
		t.Fatalf("bind structured checkout service: %v", err)
	}
	var preflight struct {
		Required  int `orm:"required"`
		Snapshots int `orm:"snapshots"`
	}
	if err = scenario.database.GetScan(scenario.ctx, &preflight, `
SELECT (SELECT count(*) FROM plugin_idc_core_commerce_quote_item_snapshot item
        JOIN plugin_idc_core_catalog_product_version_configuration configuration ON configuration.tenant_id=item.tenant_id AND configuration.version_no=item.version_no
        WHERE item.quote_no=?) AS required,
       (SELECT count(*) FROM plugin_idc_core_commerce_structured_configuration_snapshot snapshot
        JOIN plugin_idc_core_commerce_quote_item_snapshot item ON item.quote_item_no=snapshot.quote_item_no
        WHERE item.quote_no=? AND snapshot.order_no IS NULL) AS snapshots`, scenario.quote.QuoteNo, scenario.quote.QuoteNo); err != nil || preflight.Required != 1 || preflight.Snapshots != 1 {
		t.Fatalf("structured checkout preflight=%+v err=%v", preflight, err)
	}
	originalOutbox := scenario.service.outbox
	scenario.service.outbox = failingCheckoutEventWriter{}
	request := CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "structured-checkout-atomic-0001"}
	if _, err = scenario.service.CheckoutAccountQuote(scenario.ctx, request); !bizerr.Is(err, CodeUnavailable) {
		t.Fatalf("injected structured checkout failure: %+v", err)
	}
	assertStructuredCheckoutEvidence(t, scenario, "ready", "", "")
	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_order", 0)

	scenario.service.outbox = originalOutbox
	checkout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, request)
	if err != nil || checkout == nil {
		t.Fatalf("structured checkout result=%+v err=%v", checkout, err)
	}
	assertStructuredCheckoutEvidence(t, scenario, "consumed", checkout.OrderNo, checkout.OrderNo)
	replayed, err := scenario.service.CheckoutAccountQuote(scenario.ctx, request)
	if err != nil || replayed == nil || !replayed.Reused || replayed.OrderNo != checkout.OrderNo {
		t.Fatalf("structured checkout replay=%+v err=%v", replayed, err)
	}
	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_order", 1)
	assertStructuredCheckoutEvidence(t, scenario, "consumed", checkout.OrderNo, checkout.OrderNo)
	assertStructuredDatabaseEvidenceIsImmutable(t, scenario)
}

func TestStructuredConfigurationStateComesFromLatestQuotePostgreSQL(t *testing.T) {
	scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
	if len(scenario.quote.Items) != 1 || !scenario.quote.Items[0].ConfigurationRequired ||
		scenario.quote.Items[0].ConfigurationFrozen || scenario.quote.Items[0].ConfigurationRevision != "" {
		t.Fatalf("new quote must start from server-side unfrozen state: %+v", scenario.quote.Items)
	}
	quoteItem := loadStructuredQuoteItem(t, scenario, scenario.quote.QuoteNo)
	seedStructuredCheckoutEvidence(t, scenario, quoteItem.QuoteItemNo, quoteItem.ProductNo, quoteItem.VersionNo)

	// A fresh read has no browser/session state to trust. Both the cart item and
	// the recoverable latest quote must be projected from PostgreSQL evidence.
	cart, err := scenario.service.GetAccountCart(scenario.ctx)
	if err != nil {
		t.Fatalf("reload authoritative structured cart: %v", err)
	}
	if cart.LatestQuote == nil || cart.LatestQuote.QuoteNo != scenario.quote.QuoteNo ||
		len(cart.Items) != 1 || len(cart.LatestQuote.Items) != 1 {
		t.Fatalf("authoritative structured cart=%+v", cart)
	}
	cartItem, frozenItem := cart.Items[0], cart.LatestQuote.Items[0]
	if !cartItem.ConfigurationRequired || !cartItem.ConfigurationFrozen ||
		cartItem.ConfigurationRevision != "cfg-structured-1" {
		t.Fatalf("authoritative cart item=%+v", cartItem)
	}
	if !frozenItem.ConfigurationRequired || !frozenItem.ConfigurationFrozen ||
		frozenItem.ConfigurationRevision != "cfg-structured-1" {
		t.Fatalf("authoritative quote item=%+v", frozenItem)
	}
}

func TestLatestQuoteConfigurationProjectionUsesOneQueryForMultipleItemsPostgreSQL(t *testing.T) {
	scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
	logger := &checkoutBenchmarkQueryLogger{ILogger: scenario.database.GetLogger(), counts: make(map[int]int)}
	previousLogger, previousDebug := scenario.database.GetLogger(), scenario.database.GetDebug()
	scenario.database.SetLogger(logger)
	scenario.database.SetDebug(true)
	t.Cleanup(func() {
		scenario.database.SetDebug(previousDebug)
		scenario.database.SetLogger(previousLogger)
	})

	row, err := loadCustomerCart(scenario.ctx, scenario.database, "customer-commerce-service-1")
	if err != nil || row == nil {
		t.Fatalf("load one-item cart row=%+v err=%v", row, err)
	}
	oneItemContext := context.WithValue(scenario.ctx, checkoutBenchmarkSampleKey{}, 9101)
	oneItemQuote, err := scenario.service.loadLatestCartQuote(oneItemContext, row)
	if err != nil || oneItemQuote == nil || len(oneItemQuote.Items) != 1 {
		t.Fatalf("one-item latest quote=%+v err=%v", oneItemQuote, err)
	}

	cart, err := scenario.service.GetAccountCart(scenario.ctx)
	if err != nil {
		t.Fatal(err)
	}
	cart, err = scenario.service.AddAccountItem(scenario.ctx, AddItemInput{
		ExpectedCartVersion: cart.Version, Quantity: 1, Purchase: scenario.purchase,
	})
	if err != nil {
		t.Fatalf("add second authoritative cart item: %v", err)
	}
	if _, err = scenario.service.QuoteAccountCart(scenario.ctx, QuoteRequest{ExpectedCartVersion: cart.Version}); err != nil {
		t.Fatalf("quote two authoritative cart items: %v", err)
	}
	row, err = loadCustomerCart(scenario.ctx, scenario.database, "customer-commerce-service-1")
	if err != nil || row == nil {
		t.Fatalf("load two-item cart row=%+v err=%v", row, err)
	}
	twoItemContext := context.WithValue(scenario.ctx, checkoutBenchmarkSampleKey{}, 9102)
	twoItemQuote, err := scenario.service.loadLatestCartQuote(twoItemContext, row)
	if err != nil || twoItemQuote == nil || len(twoItemQuote.Items) != 2 {
		t.Fatalf("two-item latest quote=%+v err=%v", twoItemQuote, err)
	}
	if oneQueries, twoQueries := logger.count(9101), logger.count(9102); oneQueries != 1 || twoQueries != 1 {
		t.Fatalf("latest quote query budget one=%d two=%d", oneQueries, twoQueries)
	}
}

func TestAccountCartProjectionQueryCountDoesNotGrowWithItemsPostgreSQL(t *testing.T) {
	scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
	logger := &checkoutBenchmarkQueryLogger{ILogger: scenario.database.GetLogger(), counts: make(map[int]int)}
	previousLogger, previousDebug := scenario.database.GetLogger(), scenario.database.GetDebug()
	scenario.database.SetLogger(logger)
	scenario.database.SetDebug(true)
	t.Cleanup(func() {
		scenario.database.SetDebug(previousDebug)
		scenario.database.SetLogger(previousLogger)
	})

	oneItemContext := context.WithValue(scenario.ctx, checkoutBenchmarkSampleKey{}, 9201)
	oneItemCart, err := scenario.service.GetAccountCart(oneItemContext)
	if err != nil || oneItemCart == nil || len(oneItemCart.Items) != 1 {
		t.Fatalf("one-item cart=%+v err=%v", oneItemCart, err)
	}

	cart, err := scenario.service.GetAccountCart(scenario.ctx)
	if err != nil {
		t.Fatal(err)
	}
	cart, err = scenario.service.AddAccountItem(scenario.ctx, AddItemInput{
		ExpectedCartVersion: cart.Version, Quantity: 1, Purchase: scenario.purchase,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.service.QuoteAccountCart(scenario.ctx, QuoteRequest{ExpectedCartVersion: cart.Version}); err != nil {
		t.Fatal(err)
	}
	twoItemContext := context.WithValue(scenario.ctx, checkoutBenchmarkSampleKey{}, 9202)
	twoItemCart, err := scenario.service.GetAccountCart(twoItemContext)
	if err != nil || twoItemCart == nil || len(twoItemCart.Items) != 2 {
		t.Fatalf("two-item cart=%+v err=%v", twoItemCart, err)
	}
	if oneQueries, twoQueries := logger.count(9201), logger.count(9202); oneQueries < 1 || twoQueries != oneQueries {
		t.Fatalf("cart projection queries grew with item count: one=%d two=%d", oneQueries, twoQueries)
	}
}

func TestStructuredConfigurationCheckoutRejectsForeignCustomerAndLoginPostgreSQL(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		customerNo string
		userID     int
	}{
		{name: "foreign customer", customerNo: "customer-commerce-service-2", userID: 102},
		{name: "foreign login", customerNo: "customer-commerce-service-1", userID: 102},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
			quoteItem := loadStructuredQuoteItem(t, scenario, scenario.quote.QuoteNo)
			seedStructuredCheckoutEvidenceForOwner(t, scenario, quoteItem.QuoteItemNo, quoteItem.ProductNo, quoteItem.VersionNo, testCase.customerNo, testCase.userID)
			consumer, err := purchaseconfigurationfile.NewOrderFileConsumer(scenario.database, scenario.service.principal)
			if err != nil {
				t.Fatal(err)
			}
			if err = BindStructuredConfigurationSnapshots(scenario.service, structuredCheckoutFreezerStub{}, consumer); err != nil {
				t.Fatal(err)
			}
			if _, err = scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "structured-foreign-owner-0001"}); !bizerr.Is(err, CodeNotAccessible) {
				t.Fatalf("foreign structured checkout error=%v", err)
			}
			assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_order", 0)
			if testCase.name != "missing file child" {
				assertStructuredCheckoutEvidence(t, scenario, "ready", "", "")
			}
		})
	}
}

func TestStructuredConfigurationCheckoutRejectsCrossQuoteItemAndSnapshotReusePostgreSQL(t *testing.T) {
	scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
	other := newCheckoutCustomerOnDatabase(t, scenario, "customer-commerce-service-2", 102, 1)
	otherItem := loadStructuredQuoteItem(t, scenario, other.quote.QuoteNo)
	seedStructuredCheckoutEvidence(t, scenario, otherItem.QuoteItemNo, otherItem.ProductNo, otherItem.VersionNo)
	consumer, err := purchaseconfigurationfile.NewOrderFileConsumer(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(scenario.service, structuredCheckoutFreezerStub{}, consumer); err != nil {
		t.Fatal(err)
	}
	if _, err = scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "structured-cross-quote-0001"}); !bizerr.Is(err, CodeQuoteNotAvailable) {
		t.Fatalf("cross quote-item structured checkout error=%v", err)
	}
	assertCheckoutCount(t, scenario, "plugin_idc_core_commerce_order", 0)
	if _, err = scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot
(tenant_id,snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest)
SELECT tenant_id,snapshot_no,'customer-commerce-service-2',102,quote_no,quote_item_no,product_no,version_no,configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest
FROM plugin_idc_core_commerce_structured_configuration_snapshot WHERE snapshot_no='SCF_33333333333333333333333333333333'`); err == nil {
		t.Fatal("same snapshot number was reused by another customer")
	}
}

func TestStructuredConfigurationDatabaseRejectsIncompleteFileOrderEvidencePostgreSQL(t *testing.T) {
	testCases := []struct {
		name         string
		prepare      func(t *testing.T, scenario checkoutTestScenario)
		foreignOrder bool
	}{
		{name: "missing file child", prepare: func(t *testing.T, scenario checkoutTestScenario) {
			if _, err := scenario.database.Exec(scenario.ctx, `SET CONSTRAINTS ALL IMMEDIATE;
ALTER TABLE plugin_idc_core_commerce_structured_configuration_file_snapshot DISABLE TRIGGER trg_idc_guard_structured_configuration_file_snapshot;
DELETE FROM plugin_idc_core_commerce_structured_configuration_file_snapshot WHERE snapshot_no='SCF_33333333333333333333333333333333';
ALTER TABLE plugin_idc_core_commerce_structured_configuration_file_snapshot ENABLE TRIGGER trg_idc_guard_structured_configuration_file_snapshot`); err != nil {
				t.Fatalf("remove file child fixture: %v", err)
			}
		}},
		{name: "cross order", foreignOrder: true},
		{name: "cross customer", prepare: func(t *testing.T, scenario checkoutTestScenario) {
			corruptStructuredParentOwner(t, scenario, "customer-commerce-service-2", 101)
		}},
		{name: "cross uploader", prepare: func(t *testing.T, scenario checkoutTestScenario) {
			corruptStructuredParentOwner(t, scenario, "customer-commerce-service-1", 202)
		}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
			quoteItem := loadStructuredQuoteItem(t, scenario, scenario.quote.QuoteNo)
			insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
			checkout, checkoutErr := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "constraint-evidence-checkout-0001"})
			if checkoutErr != nil {
				t.Fatalf("create order evidence fixture: %v", checkoutErr)
			}
			orderItemNo := loadStructuredOrderItemNo(t, scenario, checkout.OrderNo)
			seedStructuredCheckoutEvidence(t, scenario, quoteItem.QuoteItemNo, quoteItem.ProductNo, quoteItem.VersionNo)
			if testCase.prepare != nil {
				testCase.prepare(t, scenario)
			}
			uploadOrder := checkout.OrderNo
			if testCase.foreignOrder {
				uploadOrder = "order-foreign-evidence"
			}
			err := bindStructuredOrderEvidence(scenario, checkout.OrderNo, orderItemNo, uploadOrder)
			if err == nil || !strings.Contains(err.Error(), "structured configuration file order evidence is incomplete or mismatched") {
				t.Fatalf("invalid order evidence committed: err=%v", err)
			}
			if testCase.name != "missing file child" {
				assertStructuredCheckoutEvidence(t, scenario, "ready", "", "")
			}
		})
	}
}

func TestStructuredConfigurationDatabaseAllowsOnlyOneConcurrentOrderBindingPostgreSQL(t *testing.T) {
	scenario := newStructuredCheckoutTestScenario(t, "customer-commerce-service-1", 101)
	quoteItem := loadStructuredQuoteItem(t, scenario, scenario.quote.QuoteNo)
	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	firstCheckout, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "concurrent-evidence-checkout-a"})
	if err != nil {
		t.Fatal(err)
	}
	firstItem := loadStructuredOrderItemNo(t, scenario, firstCheckout.OrderNo)
	second := newCheckoutCustomerOnDatabase(t, scenario, "customer-commerce-service-1", 101, 1)
	secondCheckout, err := second.service.CheckoutAccountQuote(second.ctx, CheckoutRequest{QuoteNo: second.quote.QuoteNo, IdempotencyKey: "concurrent-evidence-checkout-b"})
	if err != nil {
		t.Fatal(err)
	}
	secondItem := loadStructuredOrderItemNo(t, second, secondCheckout.OrderNo)
	seedStructuredCheckoutEvidence(t, scenario, quoteItem.QuoteItemNo, quoteItem.ProductNo, quoteItem.VersionNo)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, binding := range []struct{ orderNo, orderItemNo string }{{firstCheckout.OrderNo, firstItem}, {secondCheckout.OrderNo, secondItem}} {
		binding := binding
		go func() {
			<-start
			results <- bindStructuredOrderEvidence(scenario, binding.orderNo, binding.orderItemNo, binding.orderNo)
		}()
	}
	close(start)
	successes := 0
	failures := 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-results:
			if err == nil {
				successes++
			} else {
				failures++
			}
		case <-time.After(10 * time.Second):
			t.Fatal("concurrent order evidence binding did not finish")
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent order evidence binding successes=%d failures=%d", successes, failures)
	}
}

func bindStructuredOrderEvidence(scenario checkoutTestScenario, parentOrder, parentOrderItem, uploadOrder string) error {
	return scenario.database.Transaction(scenario.ctx, func(txCtx context.Context, _ gdb.TX) error {
		if _, err := scenario.database.Exec(txCtx, `UPDATE plugin_idc_core_commerce_structured_configuration_snapshot SET order_no=?,order_item_no=?,bound_at=CURRENT_TIMESTAMP WHERE snapshot_no='SCF_33333333333333333333333333333333'`, parentOrder, parentOrderItem); err != nil {
			return err
		}
		if _, err := scenario.database.Exec(txCtx, `UPDATE plugin_idc_core_purchase_configuration_upload SET status_code='consumed',bound_owner_type='order',bound_owner_no=?,consumed_at=CURRENT_TIMESTAMP WHERE upload_no='CFU_22222222222222222222222222222222'`, uploadOrder); err != nil {
			return err
		}
		return nil
	})
}

func loadStructuredOrderItemNo(t *testing.T, scenario checkoutTestScenario, orderNo string) string {
	t.Helper()
	value, err := scenario.database.GetValue(scenario.ctx, `SELECT order_item_no FROM plugin_idc_core_commerce_order_item WHERE order_no=?`, orderNo)
	if err != nil || value.String() == "" {
		t.Fatalf("load structured order item: value=%q err=%v", value.String(), err)
	}
	return value.String()
}

func corruptStructuredParentOwner(t *testing.T, scenario checkoutTestScenario, customerNo string, uploaderUserID int) {
	t.Helper()
	err := scenario.database.Transaction(scenario.ctx, func(txCtx context.Context, _ gdb.TX) error {
		for _, statement := range []string{
			`SET CONSTRAINTS ALL IMMEDIATE`,
			`ALTER TABLE plugin_idc_core_commerce_structured_configuration_snapshot DISABLE TRIGGER trg_idc_guard_structured_configuration_snapshot`,
		} {
			if _, execErr := scenario.database.Exec(txCtx, statement); execErr != nil {
				return execErr
			}
		}
		if _, execErr := scenario.database.Exec(txCtx, `UPDATE plugin_idc_core_commerce_structured_configuration_snapshot SET customer_no=?,uploader_user_id=? WHERE snapshot_no='SCF_33333333333333333333333333333333'`, customerNo, uploaderUserID); execErr != nil {
			return execErr
		}
		_, execErr := scenario.database.Exec(txCtx, `ALTER TABLE plugin_idc_core_commerce_structured_configuration_snapshot ENABLE TRIGGER trg_idc_guard_structured_configuration_snapshot`)
		return execErr
	})
	if err != nil {
		t.Fatalf("corrupt structured parent owner fixture: %v", err)
	}
}

type structuredQuoteItemFixture struct {
	QuoteItemNo string `orm:"quote_item_no"`
	ProductNo   string `orm:"product_no"`
	VersionNo   string `orm:"version_no"`
}

func loadStructuredQuoteItem(t *testing.T, scenario checkoutTestScenario, quoteNo string) structuredQuoteItemFixture {
	t.Helper()
	var quoteItem structuredQuoteItemFixture
	if err := scenario.database.GetScan(scenario.ctx, &quoteItem, `SELECT quote_item_no,product_no,version_no FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_no=?`, quoteNo); err != nil || quoteItem.QuoteItemNo == "" {
		t.Fatalf("load structured checkout quote item: %+v err=%v", quoteItem, err)
	}
	return quoteItem
}

func prepareStructuredCheckoutSchema(t *testing.T, scenario checkoutTestScenario) {
	t.Helper()
	prepareStructuredCheckoutDatabase(t, scenario.ctx, scenario.database)
}

func newStructuredCheckoutTestScenario(t *testing.T, customerNo string, userID int) checkoutTestScenario {
	t.Helper()
	return newCheckoutTestScenarioWithSetup(t, customerNo, userID, 1, false, 0, func(ctx context.Context, database gdb.DB) {
		prepareStructuredCheckoutDatabase(t, ctx, database)
	})
}

func prepareStructuredCheckoutDatabase(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve structured checkout test path")
	}
	migrationRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	for _, name := range []string{"074-product-automation-binding.sql", "075-structured-product-configuration.sql", "094-generic-configuration-pricing.sql", "096-resource-owned-product-fulfillment.sql", "097-resource-interface-configuration-revision.sql"} {
		content, err := os.ReadFile(filepath.Join(migrationRoot, name))
		if err != nil {
			t.Fatalf("read structured checkout migration %s: %v", name, err)
		}
		attempts := 1
		if name == "075-structured-product-configuration.sql" || name == "094-generic-configuration-pricing.sql" {
			attempts = 2
		}
		for attempt := 1; attempt <= attempts; attempt++ {
			if _, err = database.Exec(ctx, string(content)); err != nil {
				t.Fatalf("execute structured checkout migration %s attempt %d: %v", name, attempt, err)
			}
		}
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_product_automation_interface_instance
(tenant_id,interface_instance_no,display_name,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,new_business_state,created_by,changed_by)
VALUES (0,'pfi_11111111111111111111111111111111','Structured PG','phone-fulfillment','product-fulfillment','1.0.0','v1',ARRAY['provision','query'],repeat('1',64),repeat('2',64),'accepting',101,101);
INSERT INTO plugin_idc_core_product_automation_interface_configuration_revision
(tenant_id,interface_instance_no,revision,schema_version,declaration_snapshot,public_value_snapshot,declaration_digest,value_digest,secret_field_count,created_by)
VALUES (0,'pfi_11111111111111111111111111111111',1,'v1','{}','{}',repeat('3',64),repeat('4',64),0,101);
ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_binding
DISABLE TRIGGER trg_idc_validate_fulfillment_binding;
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding
(tenant_id,version_no,interface_instance_no,interface_display_name_snapshot,plugin_id,extension_point_code,plugin_release_version,contract_version,authorized_methods,release_digest,declaration_digest,bound_by)
VALUES (0,'version-commerce-service-1','pfi_11111111111111111111111111111111','Structured PG','phone-fulfillment','product-fulfillment','1.0.0','v1',ARRAY['provision','query'],repeat('1',64),repeat('2',64),101);
ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_binding
ENABLE TRIGGER trg_idc_validate_fulfillment_binding;
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
DISABLE TRIGGER trg_idc_guard_product_version_configuration;
INSERT INTO plugin_idc_core_catalog_product_version_configuration
(tenant_id,version_no,interface_instance_no,interface_configuration_revision,product_schema_version,product_declaration_snapshot,merchant_value_snapshot,customer_schema_version,customer_declaration_snapshot,declaration_digest,merchant_value_digest,changed_by)
VALUES (0,'version-commerce-service-1','pfi_11111111111111111111111111111111',1,'v1','{}','{}','v1',
'{"version":"v1","fields":[{"key":"identity_document","type":"file","label":{"enUS":"Document","zhCN":"证件"},"uploadPolicy":"customer.identity.v1"}],"uploadPolicies":[{"id":"customer.identity.v1","maximumFiles":1,"maximumFileBytes":1024,"maximumTotalBytes":1024,"allowedMimeTypes":["application/pdf"],"allowedExtensions":[".pdf"],"safetyScanRequired":false}]}',repeat('5',64),repeat('6',64),101);
ALTER TABLE plugin_idc_core_catalog_product_version_configuration
ENABLE TRIGGER trg_idc_guard_product_version_configuration`); err != nil {
		t.Fatalf("seed structured checkout contract: %v", err)
	}
}

func seedStructuredCheckoutEvidence(t *testing.T, scenario checkoutTestScenario, quoteItemNo, productNo, versionNo string) {
	seedStructuredCheckoutEvidenceForOwner(t, scenario, quoteItemNo, productNo, versionNo, "customer-commerce-service-1", 101)
}

func seedStructuredCheckoutEvidenceForOwner(t *testing.T, scenario checkoutTestScenario, quoteItemNo, productNo, versionNo, customerNo string, userID int) {
	t.Helper()
	const policy = `{"id":"customer.identity.v1","maximumFiles":1,"maximumFileBytes":1024,"maximumTotalBytes":1024,"allowedMimeTypes":["application/pdf"],"allowedExtensions":[".pdf"],"safetyScanRequired":false}`
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_purchase_configuration_upload
(tenant_id,upload_no,customer_no,uploader_user_id,product_version_no,configuration_revision,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,actual_size_bytes,media_type,sha256_hex,object_ref,status_code,scan_status,expires_at,completed_at)
VALUES (0,'CFU_22222222222222222222222222222222',?,?,?,'cfg-structured-1','identity_document','customer.identity.v1',?::jsonb,'identity.pdf',4,4,'application/pdf',repeat('7',64),'private/identity.pdf','ready','not_required',CURRENT_TIMESTAMP+INTERVAL '1 hour',CURRENT_TIMESTAMP)`, customerNo, userID, versionNo, policy); err != nil {
		t.Fatalf("seed structured checkout upload: %v", err)
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot
(tenant_id,snapshot_no,customer_no,uploader_user_id,quote_no,quote_item_no,product_no,version_no,configuration_revision,interface_instance_no,interface_configuration_revision,customer_declaration_digest,candidate_snapshot_digest,customer_value_digest,candidate_snapshot,typed_value_snapshot,content_digest)
VALUES (0,'SCF_33333333333333333333333333333333',?,?,?, ?,?,?,'cfg-structured-1','pfi_11111111111111111111111111111111',1,repeat('8',64),repeat('9',64),repeat('a',64),'{}','{"identity_document":["CFU_22222222222222222222222222222222"]}',repeat('b',64))`, customerNo, userID, scenario.quote.QuoteNo, quoteItemNo, productNo, versionNo); err != nil {
		t.Fatalf("seed structured checkout snapshot: %v", err)
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_commerce_structured_configuration_file_snapshot
(tenant_id,snapshot_no,field_sequence,file_sequence,field_key,upload_no,upload_policy_snapshot)
VALUES (0,'SCF_33333333333333333333333333333333',1,1,'identity_document','CFU_22222222222222222222222222222222',?::jsonb)`, policy); err != nil {
		t.Fatalf("seed structured checkout file snapshot: %v", err)
	}
}

func assertStructuredCheckoutEvidence(t *testing.T, scenario checkoutTestScenario, uploadStatus, uploadOrder, snapshotOrder string) {
	t.Helper()
	var evidence struct {
		UploadStatus  string `orm:"upload_status"`
		UploadOrder   string `orm:"upload_order"`
		SnapshotOrder string `orm:"snapshot_order"`
		OrderItemNo   string `orm:"order_item_no"`
	}
	if err := scenario.database.GetScan(scenario.ctx, &evidence, `
SELECT upload.status_code AS upload_status,upload.bound_owner_no AS upload_order,
       COALESCE(snapshot.order_no,'') AS snapshot_order,COALESCE(snapshot.order_item_no,'') AS order_item_no
FROM plugin_idc_core_purchase_configuration_upload upload
JOIN plugin_idc_core_commerce_structured_configuration_file_snapshot file ON file.upload_no=upload.upload_no
JOIN plugin_idc_core_commerce_structured_configuration_snapshot snapshot ON snapshot.snapshot_no=file.snapshot_no`); err != nil {
		t.Fatalf("load structured checkout evidence: %v", err)
	}
	if evidence.UploadStatus != uploadStatus || evidence.UploadOrder != uploadOrder || evidence.SnapshotOrder != snapshotOrder || (snapshotOrder == "" && evidence.OrderItemNo != "") || (snapshotOrder != "" && evidence.OrderItemNo == "") {
		t.Fatalf("structured checkout evidence=%+v", evidence)
	}
}

func assertStructuredDatabaseEvidenceIsImmutable(t *testing.T, scenario checkoutTestScenario) {
	t.Helper()
	if _, err := scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_purchase_configuration_upload SET status_code=status_code WHERE upload_no='CFU_22222222222222222222222222222222'`); err != nil {
		t.Fatalf("same-order consumed no-op update was rejected: %v", err)
	}
	for name, statement := range map[string]string{
		"consumed upload rollback": `UPDATE plugin_idc_core_purchase_configuration_upload SET status_code='ready',bound_owner_type='',bound_owner_no='',consumed_at=NULL WHERE upload_no='CFU_22222222222222222222222222222222'`,
		"consumed upload delete":   `DELETE FROM plugin_idc_core_purchase_configuration_upload WHERE upload_no='CFU_22222222222222222222222222222222'`,
		"file snapshot update":     `UPDATE plugin_idc_core_commerce_structured_configuration_file_snapshot SET field_key='other' WHERE upload_no='CFU_22222222222222222222222222222222'`,
		"file snapshot delete":     `DELETE FROM plugin_idc_core_commerce_structured_configuration_file_snapshot WHERE upload_no='CFU_22222222222222222222222222222222'`,
	} {
		if _, err := scenario.database.Exec(scenario.ctx, statement); err == nil {
			t.Fatalf("%s unexpectedly changed immutable evidence", name)
		}
	}
	const policy = `{"id":"customer.identity.v1","maximumFiles":1,"maximumFileBytes":1024,"maximumTotalBytes":1024,"allowedMimeTypes":["application/pdf"],"allowedExtensions":[".pdf"],"safetyScanRequired":false}`
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_purchase_configuration_upload
(tenant_id,upload_no,customer_no,uploader_user_id,product_version_no,configuration_revision,field_key,policy_id,policy_snapshot,file_name,declared_size_bytes,actual_size_bytes,media_type,sha256_hex,object_ref,status_code,scan_status,expires_at,completed_at)
VALUES (0,'CFU_44444444444444444444444444444444','customer-commerce-service-1',101,'version-commerce-service-1','cfg-structured-1','identity_document','customer.identity.v1',?::jsonb,'later.pdf',4,4,'application/pdf',repeat('7',64),'private/later.pdf','ready','not_required',CURRENT_TIMESTAMP+INTERVAL '1 hour',CURRENT_TIMESTAMP)`, policy); err != nil {
		t.Fatalf("seed late structured upload: %v", err)
	}
	if _, err := scenario.database.Exec(scenario.ctx, `
INSERT INTO plugin_idc_core_commerce_structured_configuration_file_snapshot
(tenant_id,snapshot_no,field_sequence,file_sequence,field_key,upload_no,upload_policy_snapshot)
VALUES (0,'SCF_33333333333333333333333333333333',2,1,'identity_document','CFU_44444444444444444444444444444444',?::jsonb)`, policy); err == nil {
		t.Fatal("bound structured snapshot accepted a late file child")
	}
	if _, err := scenario.database.Exec(scenario.ctx, `UPDATE plugin_idc_core_purchase_configuration_upload SET status_code='revoked' WHERE upload_no='CFU_44444444444444444444444444444444'`); err != nil {
		t.Fatalf("legal non-consumed upload lifecycle was rejected: %v", err)
	}
}
