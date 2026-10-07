// This file verifies the service entitlement and renewal foundation in PostgreSQL.

package backend

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestServiceRenewalFoundationMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL service migration tests", catalogIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create service migration admin database: %v", err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated service schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_billing_test_") {
			if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); dropErr != nil {
				t.Errorf("drop isolated service schema: %v", dropErr)
			}
		} else {
			t.Errorf("refusing to drop unexpected service schema %q", schema)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close service migration admin database: %v", closeErr)
		}
	})

	database, err := gdb.New(gdb.ConfigNode{
		Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("create scoped service database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped service database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatalf("create service dictionary fixtures: %v", err)
	}
	paths := append(billingMigrationPaths(t), serviceRenewalMigrationPaths(t)...)
	for _, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read service prerequisite %s: %v", filepath.Base(path), readErr)
		}
		attempts := 1
		if filepath.Base(path) == "030-service-renewal-foundation.sql" ||
			filepath.Base(path) == "031-service-renewal-execution.sql" ||
			filepath.Base(path) == "032-service-expiry-scheduling.sql" ||
			filepath.Base(path) == "033-billing-directory-indexes.sql" ||
			filepath.Base(path) == "056-server-operation-execution.sql" ||
			filepath.Base(path) == "057-server-accessory-operation.sql" ||
			filepath.Base(path) == "058-server-accessory-pricing.sql" ||
			filepath.Base(path) == "059-server-accessory-checkout.sql" ||
			filepath.Base(path) == "060-server-accessory-entitlement.sql" ||
			filepath.Base(path) == "065-server-customer-metadata.sql" {
			attempts = 2
		}
		for attempt := 0; attempt < attempts; attempt++ {
			if filepath.Base(path) == "030-service-renewal-foundation.sql" && attempt == 1 {
				if _, err = database.Exec(ctx, `
ALTER TABLE plugin_idc_core_service_renewal_policy
    RENAME COLUMN entitlement_no TO service_no`); err != nil {
					t.Fatalf("prepare legacy renewal policy identity: %v", err)
				}
			}
			if filepath.Base(path) == "031-service-renewal-execution.sql" && attempt == 1 {
				if _, err = database.Exec(ctx, `
ALTER TABLE plugin_idc_core_service_renewal_quote_request
    RENAME COLUMN entitlement_no TO service_no;
ALTER TABLE plugin_idc_core_service_renewal_attempt
    RENAME COLUMN entitlement_no TO service_no`); err != nil {
					t.Fatalf("prepare legacy renewal execution identities: %v", err)
				}
			}
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute service migration %s attempt %d: %v",
					filepath.Base(path), attempt+1, err)
			}
		}
	}

	assertServiceRenewalSchemaShape(t, ctx, database, schema)
	verifyServiceDictionaryTranslations(t, ctx, database, filepath.Dir(paths[0]))
	seedCommerceFoundation(t, ctx, database)
	seedServiceRenewalFoundation(t, ctx, database)
	verifyServiceRenewalConstraints(t, ctx, database)
}

func verifyServiceDictionaryTranslations(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	manifestSQLDir string,
) {
	t.Helper()
	types := []string{
		"idc_service_entitlement_status",
		"idc_service_runtime_status",
		"idc_service_provider_task_status",
		"idc_service_renewal_mode",
		"idc_business_operation_type",
		"idc_business_operation_status",
		"idc_service_price_phase",
		"idc_service_renewal_attempt_outcome",
		"idc_service_lifecycle_action",
		"idc_service_lifecycle_task_status",
	}
	type dictionaryRow struct {
		DictionaryType string `orm:"dict_type"`
		Value          string `orm:"value"`
	}
	var rows []dictionaryRow
	if err := database.Model("sys_dict_data").Ctx(ctx).
		Fields("dict_type", "value").WhereIn("dict_type", types).Scan(&rows); err != nil {
		t.Fatalf("load service dictionary rows: %v", err)
	}
	databaseValues := make(map[string][]string, len(types))
	for _, row := range rows {
		databaseValues[row.DictionaryType] = append(databaseValues[row.DictionaryType], row.Value)
	}
	for _, dictionaryType := range types {
		sort.Strings(databaseValues[dictionaryType])
	}
	for _, locale := range []string{"en-US", "zh-CN"} {
		content, err := os.ReadFile(filepath.Join(manifestSQLDir, "..", "i18n", locale, "dict.json"))
		if err != nil {
			t.Fatalf("read %s service dictionaries: %v", locale, err)
		}
		var document struct {
			Dict map[string]map[string]json.RawMessage `json:"dict"`
		}
		if err = json.Unmarshal(content, &document); err != nil {
			t.Fatalf("decode %s service dictionaries: %v", locale, err)
		}
		for _, dictionaryType := range types {
			entries, exists := document.Dict[dictionaryType]
			if !exists {
				t.Fatalf("%s dictionary is missing type %s", locale, dictionaryType)
			}
			localizedValues := make([]string, 0, len(entries)-2)
			for key := range entries {
				if key != "name" && key != "remark" {
					localizedValues = append(localizedValues, key)
				}
			}
			sort.Strings(localizedValues)
			if strings.Join(localizedValues, "\x00") !=
				strings.Join(databaseValues[dictionaryType], "\x00") {
				t.Fatalf(
					"%s dictionary %s values=%v database=%v",
					locale, dictionaryType, localizedValues, databaseValues[dictionaryType],
				)
			}
		}
	}
}

// TestServiceRenewalFoundationCodegenPostgreSQL resets only service-owned
// tables in the explicitly named disposable DAO code-generation database.
func TestServiceRenewalFoundationCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare service DAO code-generation database", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create service code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close service code-generation database: %v", closeErr)
		}
	})
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	if _, err = database.Exec(ctx, serviceRenewalCodegenCleanupSQL); err != nil {
		t.Fatalf("reset disposable service code-generation tables: %v", err)
	}
	for _, path := range serviceRenewalMigrationPaths(t) {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read service code-generation migration %s: %v", filepath.Base(path), readErr)
		}
		for attempt := 0; attempt < 2; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute service code-generation migration %s attempt %d: %v",
					filepath.Base(path), attempt+1, err)
			}
		}
	}
}

func assertServiceRenewalSchemaShape(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	schema string,
) {
	t.Helper()
	count, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = ? AND table_name IN (
    'plugin_idc_core_service_instance',
    'plugin_idc_core_service_price_snapshot',
    'plugin_idc_core_service_price_component_snapshot',
    'plugin_idc_core_service_renewal_policy',
    'plugin_idc_core_business_operation',
    'plugin_idc_core_business_operation_event',
    'plugin_idc_core_service_renewal_quote_request',
    'plugin_idc_core_service_renewal_attempt',
    'plugin_idc_core_service_lifecycle_policy',
    'plugin_idc_core_service_lifecycle_reminder_rule',
	    'plugin_idc_core_service_lifecycle_task',
	    'plugin_idc_core_service_lifecycle_task_event',
	    'plugin_idc_core_service_traffic_suspension',
	    'plugin_idc_core_server_accessory_price_policy',
	    'plugin_idc_core_server_accessory_price_policy_version',
	    'plugin_idc_core_server_accessory_quote'
)`, schema)
	if err != nil || count.Int() != 16 {
		t.Fatalf("service foundation table count=%d want=16 err=%v", count.Int(), err)
	}
	deletedColumns, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name IN (
    'plugin_idc_core_service_instance',
    'plugin_idc_core_service_price_snapshot',
    'plugin_idc_core_service_price_component_snapshot',
    'plugin_idc_core_service_renewal_policy',
    'plugin_idc_core_business_operation',
    'plugin_idc_core_business_operation_event',
    'plugin_idc_core_service_renewal_quote_request',
    'plugin_idc_core_service_renewal_attempt',
    'plugin_idc_core_service_lifecycle_policy',
    'plugin_idc_core_service_lifecycle_reminder_rule',
    'plugin_idc_core_service_lifecycle_task',
    'plugin_idc_core_service_lifecycle_task_event'
  ) AND column_name = 'deleted_at'`, schema)
	if err != nil || deletedColumns.Int() != 0 {
		t.Fatalf("service soft-delete column count=%d want=0 err=%v", deletedColumns.Int(), err)
	}
	dictionaryTypes, err := database.GetValue(ctx, `
SELECT count(*) FROM sys_dict_type
WHERE type IN (
    'idc_service_entitlement_status', 'idc_service_runtime_status',
    'idc_service_provider_task_status', 'idc_service_renewal_mode',
    'idc_business_operation_type', 'idc_business_operation_status',
    'idc_service_price_phase', 'idc_service_renewal_attempt_outcome',
    'idc_service_lifecycle_action', 'idc_service_lifecycle_task_status'
)`)
	if err != nil || dictionaryTypes.Int() != 10 {
		t.Fatalf("service dictionary type count=%d want=10 err=%v", dictionaryTypes.Int(), err)
	}
	immutableTriggers, err := database.GetValue(ctx, `
SELECT count(DISTINCT trigger_name)
FROM information_schema.triggers
WHERE trigger_schema = ? AND trigger_name IN (
    'trg_idc_service_price_immutable',
    'trg_idc_service_price_component_immutable',
    'trg_idc_service_renewal_policy_immutable',
    'trg_idc_business_operation_event_immutable',
    'trg_idc_business_operation_intent_guard',
    'trg_idc_service_renewal_quote_request_immutable',
    'trg_idc_service_renewal_attempt_immutable',
    'trg_idc_service_lifecycle_policy_immutable',
    'trg_idc_service_lifecycle_reminder_immutable',
    'trg_idc_service_lifecycle_event_immutable',
    'trg_idc_service_lifecycle_task_guard'
)`, schema)
	if err != nil || immutableTriggers.Int() != 11 {
		t.Fatalf("service history trigger count=%d want=11 err=%v", immutableTriggers.Int(), err)
	}
}

func seedServiceRenewalFoundation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_service_instance
    (service_no, customer_no, source_code, source_fulfillment_no,
     order_no, order_item_no, product_no, version_no, billing_rule_no,
     configuration_snapshot_no, product_type_code, currency_code,
     binding_no, pool_no, channel_no, entitlement_status_code,
     runtime_status_code, provider_task_status_code, renewal_mode_code,
     renewal_capability_evidence_no, renewal_capability_digest,
     renewal_capability_observed_at, renewal_capability_expires_at,
     period_unit_code, period_count, entitlement_started_at,
     entitlement_expires_at)
VALUES
    ('service-commerce-1', 'cus_commerce_test', 'order', 'fulfillment-commerce-1',
     'order-commerce', 'order-item-commerce', 'product-commerce',
     'version-commerce-1', 'billing-commerce-month', 'config-commerce',
     'cloud_server', 'CNY', 'binding-commerce', 'pool-commerce',
     'channel-commerce', 'active', 'running', 'succeeded', 'local_entitlement',
     'capability-commerce-local', repeat('a', 64),
     CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP + INTERVAL '30 days',
     'month', 1, CURRENT_TIMESTAMP - INTERVAL '30 days',
     CURRENT_TIMESTAMP + INTERVAL '30 days');

INSERT INTO plugin_idc_core_service_price_snapshot
    (price_snapshot_no, service_no, customer_no, phase_code,
     source_quote_no, source_quote_item_no, product_no, version_no,
     billing_rule_no, configuration_snapshot_no, currency_code,
     purchase_cycles, period_unit_code, period_count, renewal_mode_code,
     capability_evidence_no, capability_evidence_digest,
     capability_evidence_observed_at, capability_evidence_expires_at,
     original_expires_at, projected_expires_at,
     tax_policy_no, price_includes_tax,
     tax_treatment_code, tax_rate_basis_points, tax_category_code,
     rounding_scope_code, rounding_mode_code, rounding_quantum_minor,
     component_count, subtotal_minor, discount_minor, net_amount_minor,
     tax_amount_minor, total_amount_minor, pricing_rule_digest, content_digest)
VALUES
    ('service-price-new-1', 'service-commerce-1', 'cus_commerce_test',
     'new_purchase', 'quote-commerce', 'quote-item-commerce', 'product-commerce',
     'version-commerce-1', 'billing-commerce-month', 'config-commerce', 'CNY',
     1, 'month', 1, 'unresolved', NULL, NULL, NULL, NULL, NULL,
     CURRENT_TIMESTAMP + INTERVAL '30 days',
     'tax-commerce-cny', FALSE, 'taxable', 600, 'cloud_service', 'line',
     'half_up', 1, 3, 19900, 1000, 18900, 1134, 20034,
     repeat('b', 64), repeat('c', 64)),
    ('service-price-renewal-1', 'service-commerce-1', 'cus_commerce_test',
     'renewal', NULL, NULL, 'product-commerce', 'version-commerce-1',
     'billing-commerce-month', 'config-commerce', 'CNY', 1, 'month', 1,
     'local_entitlement', 'capability-commerce-local', repeat('a', 64),
     CURRENT_TIMESTAMP - INTERVAL '1 day', CURRENT_TIMESTAMP + INTERVAL '30 days',
     CURRENT_TIMESTAMP + INTERVAL '30 days', CURRENT_TIMESTAMP + INTERVAL '60 days',
     'tax-commerce-cny', FALSE, 'taxable', 600, 'cloud_service', 'line',
     'half_up', 1, 2, 21000, 0, 21000, 1260, 22260,
     repeat('d', 64), repeat('e', 64));

INSERT INTO plugin_idc_core_service_price_component_snapshot
    (component_snapshot_no, price_snapshot_no, service_no, line_number,
     component_type_code, charge_phase_code, source_reference, quantity,
     unit_amount_minor, amount_minor, source_digest, content_digest)
VALUES
    ('service-price-component-new', 'service-price-new-1', 'service-commerce-1',
     1, 'base', 'periodic', 'catalog-price-new', 1, 19900, 19900,
     repeat('f', 64), repeat('1', 64)),
    ('service-price-discount-new', 'service-price-new-1', 'service-commerce-1',
     2, 'discount', 'total', 'campaign-commerce', 1, 1000, 1000,
     repeat('2', 64), repeat('3', 64)),
    ('service-price-tax-new', 'service-price-new-1', 'service-commerce-1',
     3, 'tax', 'total', 'tax-commerce-cny', 1, 1134, 1134,
     repeat('4', 64), repeat('5', 64)),
    ('service-price-component-renewal', 'service-price-renewal-1',
     'service-commerce-1', 1, 'base', 'periodic', 'catalog-price-renewal',
     1, 21000, 21000, repeat('6', 64), repeat('7', 64)),
    ('service-price-tax-renewal', 'service-price-renewal-1',
     'service-commerce-1', 2, 'tax', 'total', 'tax-commerce-cny',
     1, 1260, 1260, repeat('8', 64), repeat('9', 64));

INSERT INTO plugin_idc_core_service_renewal_policy
    (policy_no, entitlement_no, policy_version, auto_renew_enabled,
     renewal_cycles, lead_seconds, maximum_attempts, retry_interval_seconds,
     failure_notification_enabled, configured_by, content_digest)
VALUES ('renewal-policy-commerce-1', 'service-commerce-1', 1, TRUE,
        1, 604800, 3, 21600, TRUE, 1, repeat('4', 64));

INSERT INTO plugin_idc_core_business_operation
    (operation_no, operation_type_code, customer_no, service_no, currency_code,
     price_snapshot_no, status_code, financial_settlement_required,
     execution_mode_code, channel_no, pool_no, capability_evidence_no,
     capability_evidence_digest, idempotency_digest, request_digest,
     content_digest, expected_service_version)
VALUES ('operation-renewal-commerce-1', 'renewal', 'cus_commerce_test',
        'service-commerce-1', 'CNY', 'service-price-renewal-1',
        'awaiting_financial_settlement', TRUE, 'local_entitlement',
        'channel-commerce', 'pool-commerce', 'capability-commerce-local', repeat('a', 64),
        repeat('5', 64), repeat('6', 64), repeat('7', 64), 1);

INSERT INTO plugin_idc_core_business_operation_event
    (event_no, operation_no, service_no, event_sequence, from_status_code,
     to_status_code, reason_code, actor_type_code, actor_reference,
     evidence_digest, content_digest)
VALUES ('operation-event-commerce-1', 'operation-renewal-commerce-1',
        'service-commerce-1', 1, NULL, 'awaiting_financial_settlement',
        'renewal.requested', 'customer', 'customer-commerce-user',
        repeat('8', 64), repeat('9', 64));

UPDATE plugin_idc_core_service_instance
SET current_price_snapshot_no = 'service-price-new-1',
    current_renewal_policy_no = 'renewal-policy-commerce-1',
    current_renewal_policy_version = 1,
    last_business_operation_no = 'operation-renewal-commerce-1',
    version = version + 1
WHERE service_no = 'service-commerce-1'`)
	if err != nil {
		t.Fatalf("insert valid service renewal graph: %v", err)
	}
}

func verifyServiceRenewalConstraints(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	for description, statement := range map[string]string{
		"service price mutation": `
UPDATE plugin_idc_core_service_price_snapshot
SET total_amount_minor = total_amount_minor
WHERE price_snapshot_no = 'service-price-renewal-1'`,
		"renewal policy mutation": `
UPDATE plugin_idc_core_service_renewal_policy
SET auto_renew_enabled = FALSE
WHERE policy_no = 'renewal-policy-commerce-1'`,
		"business operation intent mutation": `
UPDATE plugin_idc_core_business_operation
SET pool_no = 'other-pool'
WHERE operation_no = 'operation-renewal-commerce-1'`,
		"local renewal without capability evidence": `
INSERT INTO plugin_idc_core_business_operation
    (operation_no, operation_type_code, customer_no, service_no, currency_code,
     price_snapshot_no, status_code, financial_settlement_required,
     execution_mode_code, pool_no, idempotency_digest, request_digest,
     content_digest, expected_service_version)
VALUES ('operation-renewal-no-evidence', 'renewal', 'cus_commerce_test',
        'service-commerce-1', 'CNY', 'service-price-renewal-1', 'quoted', TRUE,
        'local_entitlement', 'pool-commerce', repeat('a', 64), repeat('b', 64),
        repeat('c', 64), 2)`,
		"provider renewal without channel": `
INSERT INTO plugin_idc_core_business_operation
    (operation_no, operation_type_code, customer_no, service_no, currency_code,
     price_snapshot_no, status_code, financial_settlement_required,
     execution_mode_code, pool_no, capability_evidence_no,
     capability_evidence_digest, idempotency_digest, request_digest,
     content_digest, expected_service_version)
VALUES ('operation-renewal-no-channel', 'renewal', 'cus_commerce_test',
        'service-commerce-1', 'CNY', 'service-price-renewal-1', 'quoted', TRUE,
        'provider_confirmed', 'pool-commerce', 'capability-provider', repeat('1', 64),
        repeat('2', 64), repeat('3', 64), repeat('4', 64), 2)`,
		"operation with cross-currency invoice": `
UPDATE plugin_idc_core_business_operation
SET currency_code = 'USD'
WHERE operation_no = 'operation-renewal-commerce-1'`,
		"status update without matching event": `
UPDATE plugin_idc_core_business_operation
SET status_code = 'financially_settled', financially_settled_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE operation_no = 'operation-renewal-commerce-1'`,
	} {
		if _, err := database.Exec(ctx, statement); err == nil {
			t.Fatalf("service foundation accepted %s", description)
		}
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code = 'financially_settled', financially_settled_at = CURRENT_TIMESTAMP,
    version = version + 1
WHERE operation_no = 'operation-renewal-commerce-1';
INSERT INTO plugin_idc_core_business_operation_event
    (event_no, operation_no, service_no, event_sequence, from_status_code,
     to_status_code, reason_code, actor_type_code, actor_reference,
     evidence_digest, content_digest)
VALUES ('operation-event-commerce-2', 'operation-renewal-commerce-1',
        'service-commerce-1', 2, 'awaiting_financial_settlement',
        'financially_settled', 'billing.settled', 'system', 'billing',
        repeat('a', 64), repeat('b', 64))`); err != nil {
		t.Fatalf("advance operation with matching event: %v", err)
	}
}

func serviceRenewalMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve service migration test path")
	}
	directory := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	return []string{
		filepath.Join(directory, "030-service-renewal-foundation.sql"),
		filepath.Join(directory, "031-service-renewal-execution.sql"),
		filepath.Join(directory, "032-service-expiry-scheduling.sql"),
		filepath.Join(directory, "033-billing-directory-indexes.sql"),
		filepath.Join(directory, "056-server-operation-execution.sql"),
		filepath.Join(directory, "057-server-accessory-operation.sql"),
		filepath.Join(directory, "058-server-accessory-pricing.sql"),
		filepath.Join(directory, "059-server-accessory-checkout.sql"),
		filepath.Join(directory, "060-server-accessory-entitlement.sql"),
		filepath.Join(directory, "064-server-ip-change.sql"),
		filepath.Join(directory, "065-server-customer-metadata.sql"),
		filepath.Join(directory, "066-server-safe-batch.sql"),
		filepath.Join(directory, "067-server-resize-transfer.sql"),
		filepath.Join(directory, "068-server-lifecycle-residual-resource.sql"),
	}
}

const serviceRenewalCodegenCleanupSQL = `
ALTER TABLE IF EXISTS plugin_idc_core_commerce_fulfillment_unit
    DROP CONSTRAINT IF EXISTS fk_idc_com_fulfillment_business_operation;
ALTER TABLE IF EXISTS plugin_idc_core_commerce_fulfillment_unit
    DROP CONSTRAINT IF EXISTS fk_idc_com_fulfillment_service;
ALTER TABLE IF EXISTS plugin_idc_core_service_instance
    DROP CONSTRAINT IF EXISTS fk_idc_service_lifecycle_policy_pointer;
DROP TABLE IF EXISTS plugin_idc_core_server_accessory_entitlement_event CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_server_accessory_renewal_work_item CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_server_accessory_entitlement CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_server_accessory_quote CASCADE;
ALTER TABLE IF EXISTS plugin_idc_core_server_accessory_price_policy
    DROP CONSTRAINT IF EXISTS fk_idc_server_accessory_policy_current;
DROP TABLE IF EXISTS plugin_idc_core_server_accessory_price_policy_version CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_server_accessory_price_policy CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_traffic_suspension CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_lifecycle_task_event CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_lifecycle_task CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_lifecycle_reminder_rule CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_lifecycle_policy CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_renewal_attempt CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_renewal_quote_request CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_business_operation_event CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_business_operation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_renewal_policy CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_price_component_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_price_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_service_instance CASCADE;`
