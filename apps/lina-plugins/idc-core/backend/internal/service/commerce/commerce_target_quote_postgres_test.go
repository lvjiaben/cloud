// Owned target quote integration tests use production commerce/entitlement DDL,
// actual balance settlement and the public catalog's structured freezer. Provider
// delivery completion is a fixture-owned manual fact; no external service is used.

package commerce

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/dialect"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/marketing"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"lina-plugin-idc-core/backend/internal/service/purchaseconfigurationfile"
)

type targetQuoteFixture struct {
	checkoutTestScenario
	entitlementNo string
}

func newTargetQuoteFixture(t *testing.T) targetQuoteFixture {
	t.Helper()
	scenario := newCheckoutTestScenario(t, "customer-commerce-service-1", 101, 1, false, 0)
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate owned-target fixture migration")
	}
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(source), "..", "..", "..", "..", "manifest", "sql", "142-generic-offer-configuration.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range dialect.SplitSQLStatements(string(migration)) {
		if _, err := scenario.database.Exec(scenario.ctx, statement); err != nil {
			t.Fatal(err)
		}
	}

	insertVerifiedIndividualPaymentIdentity(t, scenario, "customer-commerce-service-1")
	seedFinancialSettlementMerchant(t, scenario)
	order, err := scenario.service.CheckoutAccountQuote(scenario.ctx, CheckoutRequest{QuoteNo: scenario.quote.QuoteNo, IdempotencyKey: "owned-target-parent-checkout"})
	if err != nil {
		t.Fatal(err)
	}
	balance, err := billing.NewBalanceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(scenario.ctx, billing.ConfirmedRechargeInput{CustomerNo: "customer-commerce-service-1", CurrencyCode: "CNY", AmountMinor: order.TotalAmountMinor, SourceReference: "owned-target-parent-recharge", SourceSnapshotDigest: strings.Repeat("d", 64)}); err != nil {
		t.Fatal(err)
	}
	invoices, err := billing.NewInvoiceService(scenario.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = invoices.PayFromBalance(scenario.ctx, billing.InvoiceBalancePaymentInput{InvoiceNo: order.InvoiceNo, SourceReference: "owned-target-parent-payment", SourceSnapshotDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	settled, err := newFinancialSettlementTestService(t, scenario).SettleOrder(scenario.ctx, order.OrderNo)
	if err != nil {
		t.Fatal(err)
	}
	if err = scenario.database.Transaction(scenario.ctx, func(ctx context.Context, _ gdb.TX) error {
		for i, statement := range dialect.SplitSQLStatements(targetQuoteParentSQL) {
			var args []any
			if i < 2 {
				args = []any{settled.SettlementNo}
			}
			if _, err := scenario.database.Exec(ctx, statement, args...); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("prepare owned parent: %+v", err)
	}
	no, err := scenario.database.GetValue(scenario.ctx, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement WHERE source_intent_no='INT_owned_target'`)
	if err != nil || no.IsEmpty() {
		t.Fatalf("native entitlement=%v / %+v", no, err)
	}
	// Create a new independent cart to prove quoting the existing service cannot
	// invalidate, replace, remove or reprice an unrelated customer's current cart.
	scenario.service, scenario.quote = newCheckoutCustomer(t, scenario.ctx, scenario.database, scenario.resolver, scenario.purchase, "customer-commerce-service-1", 101, 2, false)
	reader, err := catalog.NewPublicCatalogReaderConfigured(scenario.database, []byte("0123456789abcdef0123456789abcdef"), checkoutPurchaseRuntimeStub{}, checkoutPurchaseFileVerifierStub{})
	if err != nil {
		t.Fatal(err)
	}
	freezer, err := catalog.PurchaseConfigurationFreezerFromPublicReader(reader)
	if err != nil {
		t.Fatal(err)
	}
	files, err := purchaseconfigurationfile.NewOrderFileConsumer(scenario.database, scenario.service.principal)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindStructuredConfigurationSnapshots(scenario.service, freezer, files); err != nil {
		t.Fatal(err)
	}
	return targetQuoteFixture{scenario, no.String()}
}

func TestAccountTargetQuoteAndFreezePreserveCartPostgreSQL(t *testing.T) {
	f := newTargetQuoteFixture(t)
	marketingOwner, err := marketing.New(f.database, marketing.Config{})
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewMarketingQuoteAdjustmentEvaluator(marketingOwner)
	if err != nil {
		t.Fatal(err)
	}
	f.service.adjustments = evaluator

	before, err := f.database.GetValue(f.ctx, `SELECT to_jsonb(c)::TEXT FROM plugin_idc_core_commerce_cart c WHERE cart_no=?`, f.quote.CartNo)
	if err != nil {
		t.Fatal(err)
	}
	var target *Quote
	var frozen *FrozenQuoteItemConfiguration
	err = f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		var err error
		target, err = f.service.QuoteAccountTarget(ctx, AccountTargetQuoteInput{EntitlementNo: f.entitlementNo, Purchase: f.purchase})
		if err != nil {
			return err
		}
		if len(target.Items) != 1 {
			t.Fatalf("target items=%+v", target.Items)
		}
		frozen, err = f.service.FreezeAccountQuoteItemConfiguration(ctx, FreezeQuoteItemConfigurationInput{QuoteItemNo: target.Items[0].QuoteItemNo, Purchase: catalog.PublicPurchaseConfigurationInput{PublicSlug: f.purchase.PublicSlug, ConfigurationRevision: f.purchase.ConfigurationRevision, Values: f.purchase.Values}})
		return err
	})
	if err != nil {
		t.Fatalf("owned quote+freeze: %+v", err)
	}
	if target.SubtotalMinor != 10000 || target.DiscountMinor != 0 || target.NetAmountMinor != 10000 || target.TaxAmountMinor != 600 || target.TotalAmountMinor != 10600 {
		t.Fatalf("change quote applied new-purchase discount or changed tax: %+v", target)
	}
	reservations, err := f.database.GetValue(f.ctx, `SELECT count(*) FROM plugin_idc_core_marketing_campaign_reservation WHERE quote_reference=?`, target.QuoteNo)
	if err != nil || reservations.Int() != 0 {
		t.Fatalf("change quote reserved purchase marketing: %v / %+v", reservations, err)
	}

	if target.CartNo != "" || target.CartVersion != 0 || target.Items[0].CartItemNo != "" || target.Items[0].Quantity != 1 || frozen == nil || !frozen.ConfigurationFrozen {
		t.Fatalf("detached quote=%+v frozen=%+v", target, frozen)
	}
	after, err := f.database.GetValue(f.ctx, `SELECT to_jsonb(c)::TEXT FROM plugin_idc_core_commerce_cart c WHERE cart_no=?`, f.quote.CartNo)
	if err != nil || after.String() != before.String() {
		t.Fatalf("target quote changed cart: before=%s after=%s / %+v", before, after, err)
	}
	count, err := f.database.GetValue(f.ctx, `SELECT count(*) FROM plugin_idc_core_commerce_quote_snapshot q JOIN plugin_idc_core_commerce_quote_item_snapshot i USING(quote_no) JOIN plugin_idc_core_commerce_quote_item_quantity_snapshot quantity USING(quote_item_no) JOIN plugin_idc_core_commerce_structured_configuration_snapshot sc USING(quote_item_no) WHERE q.quote_no=? AND q.purpose_code='change' AND q.source_code='customer' AND q.created_by=101 AND q.cart_no IS NULL AND i.cart_no IS NULL AND i.cart_item_no IS NULL AND quantity.quantity=1 AND sc.customer_no='customer-commerce-service-1' AND sc.uploader_user_id=101 AND sc.order_no IS NULL`, target.QuoteNo)
	if err != nil || count.Int() != 1 {
		t.Fatalf("detached owned evidence=%v / %+v", count, err)
	}
	if _, err = f.service.CheckoutAccountQuote(f.ctx, CheckoutRequest{QuoteNo: target.QuoteNo, IdempotencyKey: "owned-target-no-new-server"}); err == nil {
		t.Fatal("resize target quote became ordinary new purchase")
	}
	assertCheckoutCount(t, f.checkoutTestScenario, "plugin_idc_core_commerce_order", 1)
	assertCheckoutCount(t, f.checkoutTestScenario, "plugin_idc_core_service_instance", 0)
	// A failed real freezer must roll back the detached quote, configuration and
	// marketing reservations together when callers compose the supported methods.
	var failedQuote string
	rollbackErr := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		q, err := f.service.QuoteAccountTarget(ctx, AccountTargetQuoteInput{EntitlementNo: f.entitlementNo, Purchase: f.purchase})
		if err != nil {
			return err
		}
		failedQuote = q.QuoteNo
		_, err = f.service.FreezeAccountQuoteItemConfiguration(ctx, FreezeQuoteItemConfigurationInput{QuoteItemNo: q.Items[0].QuoteItemNo, Purchase: catalog.PublicPurchaseConfigurationInput{PublicSlug: f.purchase.PublicSlug, ConfigurationRevision: "invalid"}})
		return err
	})
	if rollbackErr == nil || failedQuote == "" {
		t.Fatalf("freezer failure was not exercised: %+v", rollbackErr)
	}
	count, err = f.database.GetValue(f.ctx, `SELECT (SELECT count(*) FROM plugin_idc_core_commerce_quote_snapshot WHERE quote_no=?)+(SELECT count(*) FROM plugin_idc_core_marketing_campaign_reservation WHERE quote_reference=?)`, failedQuote, failedQuote)
	if err != nil || count.Int() != 0 {
		t.Fatalf("failed target quote retained facts: %v / %+v", count, err)
	}

}

func TestAccountTargetQuoteRejectsForeignStaffAndPeriodPostgreSQL(t *testing.T) {
	f := newTargetQuoteFixture(t)
	input := AccountTargetQuoteInput{EntitlementNo: f.entitlementNo, Purchase: f.purchase}
	for _, test := range []struct {
		name     string
		audience *principal.Audience
		mutate   func(*AccountTargetQuoteInput)
	}{
		{"foreign", &principal.Audience{UserID: 102, Code: principal.AudienceCustomer, Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-2"}, nil},
		{"staff", &principal.Audience{UserID: 101, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive, CustomerNo: "customer-commerce-service-1"}, nil},
		{"period", nil, func(in *AccountTargetQuoteInput) { in.Purchase.PurchaseCycles++ }},
		{"currency", nil, func(in *AccountTargetQuoteInput) { in.Purchase.CurrencyCode = "USD" }},
		{"revision", nil, func(in *AccountTargetQuoteInput) { in.Purchase.ConfigurationRevision = "invalid" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			copy := input
			if test.mutate != nil {
				test.mutate(&copy)
			}
			original := f.service.principal
			if test.audience != nil {
				f.service.principal = commercePrincipalStub{test.audience}
			}
			defer func() { f.service.principal = original }()
			if q, err := f.service.QuoteAccountTarget(f.ctx, copy); err == nil || q != nil {
				t.Fatalf("unsafe target quote accepted: %+v / %+v", q, err)
			}
		})
	}
	if _, err := f.database.Exec(f.ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='suspended' WHERE entitlement_no=?`, f.entitlementNo); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.QuoteAccountTarget(f.ctx, input); !bizerr.Is(err, CodeNotAccessible) {
		t.Fatalf("suspended target: %+v", err)
	}
	assertCheckoutCount(t, f.checkoutTestScenario, "plugin_idc_core_commerce_quote_snapshot", 2)
}

const targetQuoteParentSQL = `
INSERT INTO plugin_idc_core_product_delivery_record(delivery_no,order_no,order_item_no,customer_no,product_no,version_no,quantity,strategy_code,policy_digest,configuration_snapshot_no,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,ongoing_management,status_code,content_digest,renewal_mode_code,entitlement_owner_id)
SELECT 'DEL_owned_target',s.order_no,i.order_item_no,s.customer_no,i.product_no,i.version_no,1,'payment_confirmed_plugin',repeat('a',64),i.configuration_snapshot_no,'pfi_00000000000000000000000000000001','idc-resource','1.0.0','1',repeat('a',64),TRUE,'manual_pending',repeat('a',64),'local_entitlement','idc-resource' FROM plugin_idc_core_financial_settlement s JOIN plugin_idc_core_commerce_order_item i USING(order_no) WHERE s.settlement_no=?;
INSERT INTO plugin_idc_core_product_fulfillment_intent(intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,initial_status_code,status_code,configuration_snapshot_no,interface_instance_no,interface_configuration_revision,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,provision_method,query_method,policy_digest,evidence_digest,financially_settled_at,ongoing_management,renewal_mode_code,entitlement_owner_id)
SELECT 'INT_owned_target',s.settlement_no,s.order_no,i.order_item_no,s.customer_no,s.invoice_no,s.currency_code,i.product_no,i.version_no,i.product_name_zh_cn,i.product_name_en_us,1,1,'payment_confirmed_plugin','per_order_item','manual_pending','manual_pending',i.configuration_snapshot_no,'pfi_00000000000000000000000000000001',1,'idc-resource','1.0.0','1',repeat('a',64),repeat('a',64),'server.provision','server.query',repeat('a',64),repeat('a',64),s.settled_at,TRUE,'local_entitlement','idc-resource' FROM plugin_idc_core_financial_settlement s JOIN plugin_idc_core_commerce_order_item i USING(order_no) WHERE s.settlement_no=?;
INSERT INTO plugin_idc_core_product_fulfillment_manual_conclusion(conclusion_no,intent_no,outcome_code,reason_code,note,actor_user_id,intent_version) VALUES('CON_owned_target','INT_owned_target','completed','manual_fulfillment_completed','Synthetic native delivery completed',101,1);
UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE intent_no='INT_owned_target';`
