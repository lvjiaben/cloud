// This file verifies the commerce foundation and its concurrency invariants in PostgreSQL.

package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestCommerceFoundationMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL commerce migration tests", catalogIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create commerce migration admin database: %v", err)
	}
	schema := newCommerceIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated commerce schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_commerce_test_") {
			if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); dropErr != nil {
				t.Errorf("drop isolated commerce schema: %v", dropErr)
			}
		} else {
			t.Errorf("refusing to drop unexpected commerce schema %q", schema)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close commerce migration admin database: %v", closeErr)
		}
	})

	database, err := gdb.New(gdb.ConfigNode{
		Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("create scoped commerce database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped commerce database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatalf("create commerce dictionary fixtures: %v", err)
	}
	paths := commerceMigrationPaths(t)
	for index, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read commerce migration %s: %v", filepath.Base(path), readErr)
		}
		attempts := 1
		if index == len(paths)-1 {
			attempts = 2
		}
		for attempt := 0; attempt < attempts; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute commerce migration %s attempt %d: %v",
					filepath.Base(path), attempt+1, err)
			}
		}
	}

	assertCommerceSchemaShape(t, ctx, database, schema)
	seedCommerceFoundation(t, ctx, database)
	verifyCommerceConstraints(t, ctx, database)
	verifyCommerceCartConcurrency(t, ctx, database)
}

// TestCommerceFoundationCodegenPostgreSQL resets only commerce tables in the
// explicitly named disposable DAO code-generation database.
func TestCommerceFoundationCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare the commerce DAO code-generation database", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create commerce code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close commerce code-generation database: %v", closeErr)
		}
	})
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	if _, err = database.Exec(ctx, commerceCodegenCleanupSQL); err != nil {
		t.Fatalf("reset disposable commerce code-generation tables: %v", err)
	}
	migration, err := os.ReadFile(commerceMigrationPaths(t)[5])
	if err != nil {
		t.Fatalf("read commerce code-generation migration: %v", err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute commerce code-generation migration attempt %d: %v", attempt+1, err)
		}
	}
}

func assertCommerceSchemaShape(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	schema string,
) {
	t.Helper()
	count, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_core_commerce_%'`, schema)
	if err != nil || count.Int() != 12 {
		t.Fatalf("commerce table count=%d want=12 err=%v", count.Int(), err)
	}
	deletedColumns, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_core_commerce_%'
  AND column_name = 'deleted_at'`, schema)
	if err != nil || deletedColumns.Int() != 0 {
		t.Fatalf("commerce soft-delete column count=%d want=0 err=%v", deletedColumns.Int(), err)
	}
	wrongAmountTypes, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_core_commerce_%'
  AND column_name LIKE '%_minor'
  AND data_type <> 'bigint'`, schema)
	if err != nil || wrongAmountTypes.Int() != 0 {
		t.Fatalf("commerce non-bigint minor-unit columns=%d want=0 err=%v", wrongAmountTypes.Int(), err)
	}
	dictTypes, err := database.GetValue(ctx, `
SELECT count(*) FROM sys_dict_type WHERE type LIKE 'idc_commerce_%'`)
	if err != nil || dictTypes.Int() != 14 {
		t.Fatalf("commerce dictionary type count=%d want=14 err=%v", dictTypes.Int(), err)
	}
	immutableTriggers, err := database.GetValue(ctx, `
SELECT count(DISTINCT trigger_name)
FROM information_schema.triggers
WHERE trigger_schema = ? AND trigger_name LIKE 'trg_idc_com_%_immutable'`, schema)
	if err != nil || immutableTriggers.Int() != 7 {
		t.Fatalf("commerce immutable trigger count=%d want=7 err=%v", immutableTriggers.Int(), err)
	}
}

func seedCommerceFoundation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_subject
    (customer_no, customer_type_code, status_code, display_name)
VALUES
    ('cus_commerce_test', 'individual', 'active', 'Commerce Test'),
    ('cus_commerce_race', 'individual', 'active', 'Commerce Race');

INSERT INTO plugin_idc_core_catalog_category
    (category_no, name_zh_cn, name_en_us)
VALUES ('category-commerce', '交易测试', 'Commerce test');
INSERT INTO plugin_idc_core_catalog_product
    (product_no, category_no, status_code, public_slug, name_zh_cn, name_en_us)
VALUES ('product-commerce', 'category-commerce',
        'active', 'commerce-cloud', '交易云服务器', 'Commerce cloud server');
INSERT INTO plugin_idc_core_legacy_server_category
    (tenant_id, category_no, category_type_code)
VALUES (0, 'category-commerce', 'cloud_server');
INSERT INTO plugin_idc_core_legacy_server_product
    (tenant_id, product_no, category_no, product_type_code, configuration_mode_code)
VALUES (0, 'product-commerce', 'category-commerce', 'cloud_server', 'configurable');
INSERT INTO plugin_idc_core_catalog_product_version
    (version_no, product_no, version_number, status_code, source_code, published_at)
VALUES ('version-commerce-1', 'product-commerce', 1, 'published', 'manual', CURRENT_TIMESTAMP);
UPDATE plugin_idc_core_catalog_product
SET current_version_no = 'version-commerce-1'
WHERE product_no = 'product-commerce';
INSERT INTO plugin_idc_core_catalog_configuration_group
    (group_no, version_no, group_code, selection_mode_code,
     name_zh_cn, name_en_us, minimum_selected, maximum_selected)
VALUES ('group-commerce-memory', 'version-commerce-1', 'memory', 'single',
        '内存', 'Memory', 1, 1);
INSERT INTO plugin_idc_core_catalog_configuration_option
    (option_no, version_no, group_no, option_code, component_type_code,
     name_zh_cn, name_en_us, normalized_value,
     minimum_quantity, maximum_quantity, default_quantity)
VALUES
    ('option-commerce-memory', 'version-commerce-1', 'group-commerce-memory',
     'memory-2g', 'memory', '2 GB', '2 GB', '{"megabytes":2048}', 1, 1, 1),
    ('option-commerce-memory-other', 'version-commerce-1', 'group-commerce-memory',
     'memory-4g', 'memory', '4 GB', '4 GB', '{"megabytes":4096}', 1, 1, 1);
INSERT INTO plugin_idc_core_catalog_billing_rule
    (billing_rule_no, version_no, rule_code, charge_type_code,
     period_unit_code, period_count)
VALUES ('billing-commerce-month', 'version-commerce-1', 'monthly',
        'recurring', 'month', 1);
INSERT INTO plugin_idc_core_catalog_resource_pool_binding
    (binding_no, version_no, pool_no, priority, weight)
VALUES ('binding-commerce', 'version-commerce-1', 'pool-commerce', 1, 100);
INSERT INTO plugin_idc_core_catalog_inventory_snapshot
    (snapshot_no, binding_no, source_snapshot_no, source_fingerprint,
     availability_code, available_quantity, safety_margin, sellable_quantity,
     observed_at, source_completed_at, expires_at)
VALUES ('inventory-commerce', 'binding-commerce', 'source-commerce', repeat('1', 64),
        'available', 10, 1, 9, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP,
        CURRENT_TIMESTAMP + INTERVAL '30 minutes');
INSERT INTO plugin_idc_core_catalog_tax_policy
    (tax_policy_no, version_no, currency_code, price_includes_tax,
     tax_treatment_code, tax_rate_basis_points, tax_category_code,
     rounding_scope_code, rounding_mode_code, rounding_quantum_minor)
VALUES ('tax-commerce-cny', 'version-commerce-1', 'CNY', FALSE,
        'taxable', 600, 'cloud_service', 'line', 'half_up', 1);

INSERT INTO plugin_idc_core_commerce_configuration_snapshot
    (configuration_snapshot_no, product_no, version_no, source_revision_digest,
     selection_digest, selected_option_count, total_selected_quantity, content_digest)
VALUES ('config-commerce', 'product-commerce', 'version-commerce-1', repeat('2', 64),
        repeat('3', 64), 1, 1, repeat('4', 64));
INSERT INTO plugin_idc_core_commerce_configuration_snapshot_item
    (snapshot_item_no, configuration_snapshot_no, version_no, group_no, option_no,
     group_code, option_code, component_type_code, group_name_zh_cn, group_name_en_us,
     option_name_zh_cn, option_name_en_us, normalized_value, quantity)
VALUES ('config-item-commerce', 'config-commerce', 'version-commerce-1',
        'group-commerce-memory', 'option-commerce-memory', 'memory', 'memory-2g',
        'memory', '内存', 'Memory', '2 GB', '2 GB', '{"megabytes":2048}', 1);
INSERT INTO plugin_idc_core_commerce_cart
    (cart_no, customer_no, currency_code, expires_at)
VALUES ('cart-commerce', 'cus_commerce_test', 'CNY', CURRENT_TIMESTAMP + INTERVAL '2 hours');
INSERT INTO plugin_idc_core_commerce_cart_item
    (cart_item_no, cart_no, product_no, version_no, billing_rule_no,
     configuration_snapshot_no, server_quantity, purchase_cycles)
VALUES ('cart-item-commerce', 'cart-commerce', 'product-commerce', 'version-commerce-1',
        'billing-commerce-month', 'config-commerce', 2, 1);
INSERT INTO plugin_idc_core_commerce_quote_snapshot
    (quote_no, cart_no, customer_no, customer_subject_version, purpose_code,
     source_code, currency_code, item_count, subtotal_minor, discount_minor,
     net_amount_minor, tax_amount_minor, total_amount_minor, input_digest,
     pricing_rule_digest, eligibility_digest, inventory_digest, content_digest,
     expires_at)
VALUES ('quote-commerce', 'cart-commerce', 'cus_commerce_test', 1, 'new_purchase',
        'customer', 'CNY', 1, 19900, 1000, 18900, 1134, 20034, repeat('5', 64),
        repeat('6', 64), repeat('7', 64), repeat('8', 64), repeat('9', 64),
        CURRENT_TIMESTAMP + INTERVAL '30 minutes');
INSERT INTO plugin_idc_core_commerce_quote_item_snapshot
    (quote_item_no, quote_no, cart_no, cart_item_no, line_number, currency_code,
     product_no, version_no, billing_rule_no, configuration_snapshot_no,
     product_type_code, product_name_zh_cn, product_name_en_us, server_quantity,
     purchase_cycles, charge_type_code, period_unit_code, period_count,
     fulfillment_mode_code, risk_decision_code, binding_no, inventory_snapshot_no,
     inventory_decision_code, route_decision_digest, tax_policy_no,
     price_includes_tax, tax_treatment_code, tax_rate_basis_points,
     tax_category_code, rounding_scope_code, rounding_mode_code,
     rounding_quantum_minor, setup_subtotal_minor, period_subtotal_minor,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, content_digest)
VALUES ('quote-item-commerce', 'quote-commerce', 'cart-commerce',
        'cart-item-commerce', 1, 'CNY',
        'product-commerce', 'version-commerce-1', 'billing-commerce-month',
        'config-commerce', 'cloud_server', '交易云服务器', 'Commerce cloud server',
        2, 1, 'recurring', 'month', 1, 'automatic', 'approved', 'binding-commerce',
        'inventory-commerce', 'available', repeat('a', 64), 'tax-commerce-cny',
        FALSE, 'taxable', 600, 'cloud_service', 'line', 'half_up', 1,
        0, 19900, 19900, 1000, 18900, 1134, 20034, repeat('b', 64));
INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot
    (quote_charge_no, quote_item_no, currency_code, line_number, charge_kind_code,
     charge_phase_code, configuration_snapshot_no, option_no, quantity,
     charge_amount_minor, setup_fee_minor, priced_amount_minor, price_source_digest)
VALUES ('quote-charge-commerce', 'quote-item-commerce', 'CNY', 1, 'base',
        'periodic', NULL, NULL, 1, 19900, 0, 19900, repeat('c', 64));
INSERT INTO plugin_idc_core_commerce_quote_adjustment_snapshot
    (quote_adjustment_no, quote_item_no, currency_code, application_order,
     adjustment_type_code, source_reference, source_version_digest,
     reservation_reference, amount_minor, content_digest)
VALUES ('quote-adjust-commerce', 'quote-item-commerce', 'CNY', 1,
        'campaign_discount', 'campaign-commerce', repeat('d', 64),
        'campaign-reservation-commerce', 1000, repeat('e', 64));
UPDATE plugin_idc_core_commerce_cart
SET latest_quote_no = 'quote-commerce'
WHERE cart_no = 'cart-commerce';
INSERT INTO plugin_idc_core_commerce_order
    (order_no, customer_no, quote_no, currency_code, source_code, status_code,
     risk_decision_code, item_count, subtotal_minor, discount_minor,
     net_amount_minor, tax_amount_minor, total_amount_minor,
     checkout_idempotency_digest, checkout_request_digest, payment_due_at)
VALUES ('order-commerce', 'cus_commerce_test', 'quote-commerce', 'CNY', 'portal',
        'pending_payment', 'approved', 1, 19900, 1000, 18900, 1134, 20034,
        repeat('f', 64), repeat('0', 64), CURRENT_TIMESTAMP + INTERVAL '30 minutes');
INSERT INTO plugin_idc_core_commerce_order_item
    (order_item_no, order_no, quote_no, quote_item_no, line_number, currency_code,
     product_no, version_no, billing_rule_no, configuration_snapshot_no,
     product_type_code, product_name_zh_cn, product_name_en_us, server_quantity,
     purchase_cycles, charge_type_code, period_unit_code, period_count,
     fulfillment_mode_code, risk_decision_code, initial_binding_no,
     inventory_snapshot_no, inventory_decision_code, route_decision_digest,
     tax_policy_no, price_includes_tax, tax_treatment_code, tax_rate_basis_points,
     tax_category_code, rounding_scope_code, rounding_mode_code,
     rounding_quantum_minor, setup_subtotal_minor, period_subtotal_minor,
     subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, content_digest)
VALUES ('order-item-commerce', 'order-commerce', 'quote-commerce',
        'quote-item-commerce', 1, 'CNY', 'product-commerce', 'version-commerce-1',
        'billing-commerce-month', 'config-commerce', 'cloud_server',
        '交易云服务器', 'Commerce cloud server', 2, 1, 'recurring', 'month', 1,
        'automatic', 'approved', 'binding-commerce', 'inventory-commerce',
        'available', repeat('a', 64), 'tax-commerce-cny', FALSE, 'taxable', 600,
        'cloud_service', 'line', 'half_up', 1, 0, 19900, 19900, 1000, 18900,
        1134, 20034, repeat('b', 64));
INSERT INTO plugin_idc_core_commerce_inventory_reservation
    (reservation_no, order_no, order_item_no, binding_no, inventory_snapshot_no,
     reserved_quantity, content_digest, hold_expires_at)
VALUES ('reservation-commerce', 'order-commerce', 'order-item-commerce',
        'binding-commerce', 'inventory-commerce', 2, repeat('1', 64),
        CURRENT_TIMESTAMP + INTERVAL '20 minutes');
INSERT INTO plugin_idc_core_commerce_fulfillment_unit
    (fulfillment_no, order_no, order_item_no, unit_sequence, reservation_no)
VALUES
    ('fulfillment-commerce-1', 'order-commerce', 'order-item-commerce', 1,
     'reservation-commerce'),
    ('fulfillment-commerce-2', 'order-commerce', 'order-item-commerce', 2,
     'reservation-commerce')`)
	if err != nil {
		t.Fatalf("insert valid commerce graph: %v", err)
	}
}

func verifyCommerceConstraints(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	for description, statement := range map[string]string{
		"duplicate active customer cart": `
INSERT INTO plugin_idc_core_commerce_cart
    (cart_no, customer_no, currency_code, expires_at)
VALUES ('cart-commerce-duplicate', 'cus_commerce_test', 'CNY',
        CURRENT_TIMESTAMP + INTERVAL '1 hour')`,
		"cart with two owners": `
INSERT INTO plugin_idc_core_commerce_cart
    (cart_no, customer_no, guest_owner_digest, currency_code, expires_at)
VALUES ('cart-commerce-two-owners', 'cus_commerce_race', repeat('1', 64), 'CNY',
        CURRENT_TIMESTAMP + INTERVAL '1 hour')`,
		"quote with inconsistent amount equation": `
INSERT INTO plugin_idc_core_commerce_quote_snapshot
    (quote_no, customer_no, customer_subject_version, source_code, currency_code,
     item_count, subtotal_minor, discount_minor, net_amount_minor,
     tax_amount_minor, total_amount_minor, input_digest, pricing_rule_digest,
     eligibility_digest, inventory_digest, content_digest, expires_at)
VALUES ('quote-commerce-invalid', 'cus_commerce_test', 1, 'customer', 'CNY',
        1, 100, 1, 99, 1, 999, repeat('1', 64), repeat('2', 64), repeat('3', 64),
        repeat('4', 64), repeat('5', 64), CURRENT_TIMESTAMP + INTERVAL '10 minutes')`,
		"cart quote with another currency": `
INSERT INTO plugin_idc_core_commerce_quote_snapshot
    (quote_no, cart_no, customer_no, customer_subject_version, source_code,
     currency_code, item_count, subtotal_minor, net_amount_minor, total_amount_minor,
     input_digest, pricing_rule_digest, eligibility_digest, inventory_digest,
     content_digest, expires_at)
VALUES ('quote-commerce-cross-currency', 'cart-commerce', 'cus_commerce_test', 1,
        'customer', 'USD', 1, 100, 100, 100, repeat('1', 64), repeat('2', 64),
        repeat('3', 64), repeat('4', 64), repeat('5', 64),
        CURRENT_TIMESTAMP + INTERVAL '10 minutes')`,
		"charge for an option absent from the configuration": `
INSERT INTO plugin_idc_core_commerce_quote_charge_snapshot
    (quote_charge_no, quote_item_no, currency_code, line_number, charge_kind_code,
     charge_phase_code, configuration_snapshot_no, option_no, quantity,
     charge_amount_minor, priced_amount_minor, price_source_digest)
VALUES ('quote-charge-commerce-invalid', 'quote-item-commerce', 'CNY', 2,
        'option', 'periodic', 'config-commerce', 'option-commerce-memory-other',
        1, 100, 100, repeat('6', 64))`,
		"second active reservation": `
INSERT INTO plugin_idc_core_commerce_inventory_reservation
    (reservation_no, order_no, order_item_no, binding_no, inventory_snapshot_no,
     reserved_quantity, content_digest, hold_expires_at)
VALUES ('reservation-commerce-duplicate', 'order-commerce', 'order-item-commerce',
        'binding-commerce', 'inventory-commerce', 2, repeat('2', 64),
        CURRENT_TIMESTAMP + INTERVAL '10 minutes')`,
		"successful fulfillment without service": `
INSERT INTO plugin_idc_core_commerce_fulfillment_unit
    (fulfillment_no, order_no, order_item_no, unit_sequence, status_code,
     reservation_no, started_at, completed_at)
VALUES ('fulfillment-commerce-invalid', 'order-commerce', 'order-item-commerce', 3,
        'succeeded', 'reservation-commerce', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
	} {
		if _, err := database.Exec(ctx, statement); err == nil {
			t.Fatalf("commerce accepted %s", description)
		}
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_commerce_quote_snapshot
SET total_amount_minor = total_amount_minor
WHERE quote_no = 'quote-commerce'`); err == nil {
		t.Fatal("commerce allowed an immutable quote snapshot to be updated")
	}
	if _, err := database.Exec(ctx, `
DELETE FROM plugin_idc_core_commerce_order_item
WHERE order_item_no = 'order-item-commerce'`); err == nil {
		t.Fatal("commerce allowed immutable order evidence to be deleted")
	}
}

func verifyCommerceCartConcurrency(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	start := make(chan struct{})
	errors := make(chan error, 2)
	var wait sync.WaitGroup
	for index := 1; index <= 2; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_commerce_cart
    (cart_no, customer_no, currency_code, expires_at)
VALUES (?, 'cus_commerce_race', 'CNY', CURRENT_TIMESTAMP + INTERVAL '1 hour')`,
				"cart-commerce-race-"+string(rune('0'+index)))
			errors <- err
		}()
	}
	close(start)
	wait.Wait()
	close(errors)
	successes := 0
	failures := 0
	for err := range errors {
		if err == nil {
			successes++
		} else {
			failures++
		}
	}
	if successes != 1 || failures != 1 {
		t.Fatalf("concurrent active carts successes=%d failures=%d want=1/1", successes, failures)
	}
	count, err := database.GetValue(ctx, `
SELECT count(*)
FROM plugin_idc_core_commerce_cart
WHERE customer_no = 'cus_commerce_race' AND status_code = 'active'`)
	if err != nil || count.Int() != 1 {
		t.Fatalf("active race cart count=%d want=1 err=%v", count.Int(), err)
	}
}

func newCommerceIntegrationSchema(t *testing.T) string {
	t.Helper()
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("generate commerce integration schema suffix: %v", err)
	}
	return "linapro_commerce_test_" + hex.EncodeToString(random)
}

func commerceMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve commerce migration test path")
	}
	migrationRoot := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	return []string{
		filepath.Join(migrationRoot, "001-core-foundation.sql"),
		filepath.Join(migrationRoot, "010-catalog-foundation.sql"),
		filepath.Join(migrationRoot, "013-catalog-billing-policy.sql"),
		filepath.Join(migrationRoot, "014-catalog-inventory-routing.sql"),
		filepath.Join(migrationRoot, "019-catalog-tax-policy.sql"),
		filepath.Join(migrationRoot, "022-commerce-foundation.sql"),
		filepath.Join(migrationRoot, "073-general-product-catalog.sql"),
	}
}

const commerceCodegenCleanupSQL = `
DROP TABLE IF EXISTS plugin_idc_core_commerce_fulfillment_unit CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_inventory_reservation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_order_item CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_order CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_quote_adjustment_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_quote_charge_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_quote_item_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_quote_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_cart_item CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_configuration_snapshot_item CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_configuration_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_commerce_cart CASCADE;`
