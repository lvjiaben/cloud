// This file checks the paid accessory checkout's immutable non-stock snapshots
// against real commerce/billing migrations and the real financial settlement owner.
// Each case owns an isolated PostgreSQL schema; no live account or provider is used.

package serveroperation

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/dialect"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/outbox"
)

// accessoryCheckoutFixture holds only the local financial checkout scenario.
type accessoryCheckoutFixture struct {
	ctx      context.Context
	database gdb.DB
	service  *accessoryPricingImpl
	row      *accessoryCheckoutRow
}

// TestAccessoryCheckoutNonStockSettlementPostgreSQL proves the repaired financial
// phase can settle once without consuming catalog stock or creating a new server.
func TestAccessoryCheckoutNonStockSettlementPostgreSQL(t *testing.T) {
	fixture := newAccessoryCheckoutFixture(t)
	if err := fixture.checkout(); err != nil {
		t.Fatalf("checkout paid accessory: %+v", err)
	}
	fixture.assertSnapshots(t)
	settlements := fixture.settlementService(t)
	if _, err := settlements.SettleOrder(fixture.ctx, "aor_checkout_test"); err == nil {
		t.Fatal("unpaid accessory was financially settled")
	}
	fixture.pay(t)
	settled, err := settlements.SettleOrder(fixture.ctx, "aor_checkout_test")
	if err != nil {
		t.Fatalf("settle paid accessory using non-stock snapshots: %+v", err)
	}
	if settled.Reused || settled.TotalAmountMinor != 1000 || len(settled.Sources) != 1 ||
		settled.Sources[0].SourceType != billing.FinancialSettlementSourceBalance {
		t.Fatalf("unexpected accessory settlement: %+v", settled)
	}
	replayed, err := settlements.SettleOrder(fixture.ctx, "aor_checkout_test")
	if err != nil || !replayed.Reused || replayed.SettlementNo != settled.SettlementNo {
		t.Fatalf("settlement replay diverged: %+v / %+v", replayed, err)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM plugin_idc_core_financial_settlement`,
		`SELECT COUNT(*) FROM plugin_idc_core_outbox_event WHERE event_type='commerce.order.financially_settled'`,
	} {
		fixture.assertCount(t, query, 1)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_inventory_reservation`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_service_instance`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order_item WHERE inventory_decision_code='manual_confirmation'`, 1)
	fixture.assertCount(t, `SELECT sellable_quantity FROM plugin_idc_core_catalog_product_version_inventory WHERE version_no='ver_checkout_test'`, 17)
}

// TestAccessoryCheckoutRejectsConflictingInventoryPostgreSQL proves checkout
// cannot convert an immutable tracked or corrupted quote fact into an addon fact.
func TestAccessoryCheckoutRejectsConflictingInventoryPostgreSQL(t *testing.T) {
	canonicalDigest, err := digestValue(struct {
		QuoteItemNo      string
		VersionNo        string
		TrackingEnabled  bool
		InventoryVersion int64
		SellableQuantity int64
	}{"aqi_checkout_test", "ver_checkout_test", false, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		tracking bool
		version  int64
		quantity int64
		digest   string
	}{
		{"tracked", true, 1, 17, strings.Repeat("a", 64)},
		{"wrong-digest", false, 0, 0, strings.Repeat("b", 64)},
		{"wrong-inventory-version", false, 1, 0, canonicalDigest},
		{"wrong-sellable-quantity", false, 0, 1, canonicalDigest},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newAccessoryCheckoutFixture(t)
			if _, err := fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_commerce_quote_item_inventory_snapshot
(quote_item_no,version_no,tracking_enabled,inventory_version,sellable_quantity,content_digest)
VALUES ('aqi_checkout_test','ver_checkout_test',?,?,?,?)`, test.tracking, test.version, test.quantity, test.digest); err != nil {
				t.Fatal(err)
			}
			if err := fixture.checkout(); err == nil {
				t.Fatal("conflicting immutable inventory allowed accessory checkout")
			}
			fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order`, 0)
			fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_billing_invoice`, 0)
			fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order_item_inventory_snapshot`, 0)
			value, err := fixture.database.GetValue(fixture.ctx, `SELECT content_digest FROM plugin_idc_core_commerce_quote_item_inventory_snapshot WHERE quote_item_no='aqi_checkout_test'`)
			if err != nil || value.String() != test.digest {
				t.Fatalf("existing inventory was changed: %v / %v", value, err)
			}
		})
	}
}

// TestAccessoryCheckoutInventoryTransactionPostgreSQL covers rollback, retry,
// and reuse of an already matching immutable quote inventory record.
func TestAccessoryCheckoutInventoryTransactionPostgreSQL(t *testing.T) {
	fixture := newAccessoryCheckoutFixture(t)
	rollback := errors.New("accessory checkout later step failed")
	err := fixture.database.Transaction(fixture.ctx, func(ctx context.Context, _ gdb.TX) error {
		if writeErr := fixture.service.persistAccessoryOrderAndInvoice(ctx, fixture.row,
			"aor_checkout_test", "ain_checkout_test", strings.Repeat("c", 64), strings.Repeat("d", 64), time.Now().UTC()); writeErr != nil {
			return writeErr
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatalf("rollback failed: %+v", err)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_quote_item_inventory_snapshot`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_order_item_inventory_snapshot`, 0)
	quoteDigest, err := digestValue(struct {
		QuoteItemNo      string
		VersionNo        string
		TrackingEnabled  bool
		InventoryVersion int64
		SellableQuantity int64
	}{"aqi_checkout_test", "ver_checkout_test", false, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fixture.database.Exec(fixture.ctx, `INSERT INTO plugin_idc_core_commerce_quote_item_inventory_snapshot
(quote_item_no,version_no,tracking_enabled,inventory_version,sellable_quantity,content_digest)
VALUES ('aqi_checkout_test','ver_checkout_test',FALSE,0,0,?)`, quoteDigest); err != nil {
		t.Fatal(err)
	}
	if err = fixture.checkout(); err != nil {
		t.Fatalf("retry after rollback: %+v", err)
	}
	fixture.assertSnapshots(t)
	if _, err = fixture.database.Exec(fixture.ctx, `UPDATE plugin_idc_core_commerce_order_item_inventory_snapshot SET sellable_quantity=1`); err == nil {
		t.Fatal("inventory snapshot accepted mutation")
	}
}

// checkout invokes the exact transaction-bound financial phase under repair.
func (fixture accessoryCheckoutFixture) checkout() error {
	return fixture.database.Transaction(fixture.ctx, func(ctx context.Context, _ gdb.TX) error {
		return fixture.service.persistAccessoryOrderAndInvoice(ctx, fixture.row,
			"aor_checkout_test", "ain_checkout_test", strings.Repeat("c", 64), strings.Repeat("d", 64), time.Now().UTC())
	})
}

// assertSnapshots verifies both canonical digests and the unchanged addon decision.
func (fixture accessoryCheckoutFixture) assertSnapshots(t *testing.T) {
	t.Helper()
	var quoteItem entity.CommerceQuoteItemSnapshot
	if err := fixture.database.Model("plugin_idc_core_commerce_quote_item_snapshot").Ctx(fixture.ctx).
		Scan(&quoteItem); err != nil {
		t.Fatal(err)
	}
	orderItemNo, err := deterministicReference("aoi_", fixture.row.QuoteNo)
	if err != nil {
		t.Fatal(err)
	}
	quoteDigest, err := digestValue(struct {
		QuoteItemNo      string
		VersionNo        string
		TrackingEnabled  bool
		InventoryVersion int64
		SellableQuantity int64
	}{quoteItem.QuoteItemNo, quoteItem.VersionNo, false, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	orderDigest, err := digestValue(struct {
		OrderItemNo      string
		QuoteItemNo      string
		VersionNo        string
		TrackingEnabled  bool
		InventoryVersion int64
		SellableQuantity int64
	}{orderItemNo, quoteItem.QuoteItemNo, quoteItem.VersionNo, false, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	count, err := fixture.database.GetValue(fixture.ctx, `SELECT COUNT(*)
FROM plugin_idc_core_commerce_order_item_inventory_snapshot oi
JOIN plugin_idc_core_commerce_quote_item_inventory_snapshot qi USING (quote_item_no,version_no)
WHERE oi.order_item_no=? AND qi.quote_item_no=? AND qi.content_digest=? AND oi.content_digest=?
AND NOT oi.tracking_enabled AND NOT qi.tracking_enabled AND oi.inventory_version=0
AND qi.inventory_version=0 AND oi.sellable_quantity=0 AND qi.sellable_quantity=0`,
		orderItemNo, quoteItem.QuoteItemNo, quoteDigest, orderDigest)
	if err != nil || count.Int() != 1 {
		t.Fatalf("non-stock canonical snapshots missing: %v / %+v", count, err)
	}
}

// assertCount reads a fixed fixture-only aggregate.
func (fixture accessoryCheckoutFixture) assertCount(t *testing.T, query string, want int) {
	t.Helper()
	value, err := fixture.database.GetValue(fixture.ctx, query)
	if err != nil || value.Int() != want {
		t.Fatalf("aggregate got %v want %d: %+v (%s)", value, want, err, query)
	}
}

// pay creates and consumes only a fixture-owned confirmed recharge.
func (fixture accessoryCheckoutFixture) pay(t *testing.T) {
	fixture.payInvoice(t, "ain_checkout_test")
}

// payInvoice funds a checkout-generated invoice within this fixture only.
func (fixture accessoryCheckoutFixture) payInvoice(t *testing.T, invoiceNo string) {
	t.Helper()
	balance, err := billing.NewBalanceService(fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(fixture.ctx, billing.ConfirmedRechargeInput{
		CustomerNo: "CUS_checkout_test", CurrencyCode: "CNY", AmountMinor: 1000,
		SourceReference: "recharge_checkout_test", SourceSnapshotDigest: strings.Repeat("e", 64),
	}); err != nil {
		t.Fatal(err)
	}
	invoices, err := billing.NewInvoiceService(fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = invoices.PayFromBalance(fixture.ctx, billing.InvoiceBalancePaymentInput{
		InvoiceNo: invoiceNo, SourceReference: "payment_checkout_test",
		SourceSnapshotDigest: strings.Repeat("f", 64),
	}); err != nil {
		t.Fatal(err)
	}
}

// settlementService builds the real billing authority with fixture-local services.
func (fixture accessoryCheckoutFixture) settlementService(t *testing.T) billing.FinancialSettlementService {
	t.Helper()
	events, err := outbox.NewEnqueuer(fixture.database, outbox.Config{})
	if err != nil {
		t.Fatal(err)
	}
	credits, err := billing.NewCreditUsageService(fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	service, err := billing.NewFinancialSettlementService(fixture.database, events, credits)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

// newAccessoryCheckoutFixture loads the production commerce/billing schema,
// including the inventory foreign keys and append-only triggers from migration 082.
func newAccessoryCheckoutFixture(t *testing.T) accessoryCheckoutFixture {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(transferPostgresDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run accessory checkout PostgreSQL tests", transferPostgresDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	schema := transferTestSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, dropErr := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); dropErr != nil {
			t.Error(dropErr)
		}
	})
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if _, err = database.Exec(ctx, accessoryCheckoutDictionarySQL); err != nil {
		t.Fatal(err)
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve accessory checkout migrations")
	}
	migrationRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	for _, name := range []string{
		"001-core-foundation.sql", "003-customer-account.sql", "010-catalog-foundation.sql",
		"013-catalog-billing-policy.sql", "014-catalog-inventory-routing.sql", "019-catalog-tax-policy.sql",
		"020-marketing-foundation.sql", "022-commerce-foundation.sql", "023-billing-ledger-foundation.sql",
		"024-commerce-order-lifecycle.sql", "025-commerce-payment-identity-gate.sql", "026-commerce-assisted-order.sql",
		"027-billing-invoice-lifecycle.sql", "028-billing-invoice-transform.sql", "029-billing-reviewed-adjustment.sql",
		"030-service-renewal-foundation.sql", "031-service-renewal-execution.sql", "032-service-expiry-scheduling.sql",
		"033-billing-directory-indexes.sql", "034-credit-account-foundation.sql", "035-financial-settlement.sql",
		"036-trial-qualification.sql", "037-financial-settlement-fulfillment.sql", "038-service-provision-execution.sql",
		"042-support-content.sql", "043-contract-policy.sql", "044-contract-failure-compensation.sql",
		"052-online-refund.sql", "053-credit-refund-lifecycle.sql", "054-credit-online-refund-integration.sql",
		"061-commission-refund-reversal.sql", "062-marketing-performance-abuse.sql", "070-customer-short-code.sql",
		"073-general-product-catalog.sql", "074-product-automation-binding.sql", "075-structured-product-configuration.sql",
		"077-management-configuration-files.sql", "078-product-fulfillment-execution.sql", "079-generic-commerce-query-indexes.sql",
		"080-generic-delivery-entitlement.sql", "081-simple-product-version-inventory.sql", "082-generic-commerce-inventory-snapshots.sql",
		"083-fixed-product-empty-configuration.sql", "084-general-product-version-cost.sql", "085-generic-trial-unit-identity.sql",
		"086-generic-commerce-shadow-removal.sql",
	} {
		content, readErr := os.ReadFile(filepath.Join(migrationRoot, name))
		if readErr != nil {
			t.Fatal(readErr)
		}
		for index, statement := range dialect.SplitSQLStatements(string(content)) {
			if _, err = database.Exec(ctx, statement); err != nil {
				t.Fatalf("migration %s statement %d: %v", name, index+1, err)
			}
		}
	}
	for index, statement := range dialect.SplitSQLStatements(accessoryCheckoutSeedSQL) {
		if _, err = database.Exec(ctx, statement); err != nil {
			t.Fatalf("seed accessory checkout statement %d: %+v", index+1, err)
		}
	}
	return accessoryCheckoutFixture{ctx: ctx, database: database, service: &accessoryPricingImpl{database: database},
		row: &accessoryCheckoutRow{QuoteNo: "AQU_checkout_test", CustomerNo: "CUS_checkout_test", CurrencyCode: "CNY",
			SubtotalMinor: 1000, NetAmountMinor: 1000, TotalAmountMinor: 1000,
			ExpiresAt: time.Now().UTC().Add(time.Hour), QuoteContentDigest: strings.Repeat("a", 64)}}
}

// accessoryCheckoutDictionarySQL is the minimal host-owned dictionary boundary
// required by the plugin's real migrations, not a replacement for business tables.
const accessoryCheckoutDictionarySQL = `
CREATE TABLE sys_dict_type (
 id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 name VARCHAR(128) NOT NULL DEFAULT '', type VARCHAR(128) NOT NULL DEFAULT '',
 status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
 allow_tenant_override BOOL NOT NULL DEFAULT FALSE, remark VARCHAR(512) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, UNIQUE (tenant_id,type));
CREATE TABLE sys_dict_data (
 id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 dict_type VARCHAR(128) NOT NULL DEFAULT '', label VARCHAR(128) NOT NULL DEFAULT '',
 value VARCHAR(128) NOT NULL DEFAULT '', sort INT NOT NULL DEFAULT 0, tag_style VARCHAR(128) NOT NULL DEFAULT '',
 status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
 created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ, UNIQUE (tenant_id,dict_type,value));`

// accessoryCheckoutSeedSQL supplies synthetic merchant/customer/catalog/quote
// facts only. All funds are subsequently posted through the real balance service.
const accessoryCheckoutSeedSQL = `
INSERT INTO plugin_idc_core_merchant_profile(merchant_no,display_name,legal_name,default_currency,enabled)
VALUES ('MER_checkout_test','Test Merchant','Test Merchant','CNY',TRUE);
INSERT INTO plugin_idc_core_customer_subject(customer_no,customer_code,customer_type_code,status_code,display_name)
VALUES ('CUS_checkout_test',10001,'individual','active','Synthetic Checkout Customer');
INSERT INTO plugin_idc_core_customer_individual_profile
(customer_no,legal_name,legal_name_masked,identity_identifier_masked,identity_lookup_digest,identity_case_no,
identity_status_code,identity_verified_at,identity_expires_at)
VALUES ('CUS_checkout_test','Synthetic Customer','S*** C***','TEST***',repeat('a',64),'CASE_checkout_test',
'verified',CURRENT_TIMESTAMP-INTERVAL '1 minute',CURRENT_TIMESTAMP+INTERVAL '1 year');
INSERT INTO plugin_idc_core_catalog_category(category_no,status_code,name_zh_cn,name_en_us)
VALUES ('ctg_checkout_test','active','测试分类','Test category');
INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,status_code,public_slug,name_zh_cn,name_en_us)
VALUES ('prd_checkout_test','ctg_checkout_test','active','checkout-test','测试商品','Test product');
INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,version_number,status_code,source_code)
VALUES ('ver_checkout_test','prd_checkout_test',1,'draft','manual');
INSERT INTO plugin_idc_core_catalog_product_version_inventory(version_no,tracking_enabled,sellable_quantity,version)
VALUES ('ver_checkout_test',TRUE,17,1);
INSERT INTO plugin_idc_core_catalog_billing_rule
(billing_rule_no,version_no,rule_code,charge_type_code,period_unit_code,period_count,price_phase_code,status_code)
VALUES ('BILL_checkout_test','ver_checkout_test','once','one_time','once',1,'new_purchase','active');
INSERT INTO plugin_idc_core_catalog_tax_policy
(tax_policy_no,version_no,currency_code,price_includes_tax,tax_treatment_code,tax_category_code,
rounding_scope_code,rounding_mode_code,rounding_quantum_minor)
VALUES ('TAX_checkout_test','ver_checkout_test','CNY',TRUE,'exempt','cloud_service','line','half_up',1);
INSERT INTO plugin_idc_core_commerce_configuration_snapshot
(configuration_snapshot_no,product_no,version_no,source_revision_digest,selection_digest,selected_option_count,total_selected_quantity,content_digest)
VALUES ('CFG_checkout_test','prd_checkout_test','ver_checkout_test',repeat('a',64),repeat('a',64),0,0,repeat('a',64));
INSERT INTO plugin_idc_core_commerce_quote_snapshot
(quote_no,customer_no,customer_subject_version,purpose_code,source_code,currency_code,item_count,
subtotal_minor,discount_minor,net_amount_minor,tax_amount_minor,total_amount_minor,input_digest,pricing_rule_digest,
eligibility_digest,inventory_digest,content_digest,issued_at,expires_at,created_by)
VALUES ('AQU_checkout_test','CUS_checkout_test',1,'addon','customer','CNY',1,1000,0,1000,0,1000,
repeat('a',64),repeat('a',64),repeat('a',64),repeat('a',64),repeat('a',64),CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 hour',0);
INSERT INTO plugin_idc_core_commerce_quote_item_snapshot
(quote_item_no,quote_no,line_number,currency_code,product_no,version_no,billing_rule_no,configuration_snapshot_no,
product_name_zh_cn,product_name_en_us,purchase_cycles,charge_type_code,period_unit_code,period_count,
fulfillment_mode_code,risk_decision_code,inventory_decision_code,route_decision_digest,tax_policy_no,price_includes_tax,
tax_treatment_code,tax_category_code,rounding_scope_code,rounding_mode_code,rounding_quantum_minor,setup_subtotal_minor,
subtotal_minor,net_amount_minor,total_amount_minor,content_digest)
VALUES ('aqi_checkout_test','AQU_checkout_test',1,'CNY','prd_checkout_test','ver_checkout_test','BILL_checkout_test','CFG_checkout_test',
'测试流量包','Test traffic package',1,'one_time','once',1,'automatic','approved','manual_confirmation',repeat('a',64),
'TAX_checkout_test',TRUE,'exempt','cloud_service','line','half_up',1,1000,1000,1000,1000,repeat('a',64));
INSERT INTO plugin_idc_core_commerce_quote_item_quantity_snapshot(quote_item_no,quantity,content_digest)
VALUES ('aqi_checkout_test',1,repeat('a',64));`
