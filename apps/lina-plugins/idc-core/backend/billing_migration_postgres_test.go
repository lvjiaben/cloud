// This file verifies the billing and ledger foundation in PostgreSQL.

package backend

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestBillingLedgerFoundationMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL billing migration tests", catalogIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create billing migration admin database: %v", err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated billing schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_billing_test_") {
			if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); dropErr != nil {
				t.Errorf("drop isolated billing schema: %v", dropErr)
			}
		} else {
			t.Errorf("refusing to drop unexpected billing schema %q", schema)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close billing migration admin database: %v", closeErr)
		}
	})

	database, err := gdb.New(gdb.ConfigNode{
		Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("create scoped billing database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped billing database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatalf("create billing dictionary fixtures: %v", err)
	}
	paths := billingMigrationPaths(t)
	for _, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read billing migration %s: %v", filepath.Base(path), readErr)
		}
		attempts := 1
		if filepath.Base(path) == "028-billing-invoice-transform.sql" ||
			filepath.Base(path) == "029-billing-reviewed-adjustment.sql" ||
			filepath.Base(path) == "034-credit-account-foundation.sql" ||
			filepath.Base(path) == "053-credit-refund-lifecycle.sql" ||
			filepath.Base(path) == "054-credit-online-refund-integration.sql" ||
			filepath.Base(path) == "055-credit-evidence.sql" {
			attempts = 2
		}
		for attempt := 0; attempt < attempts; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute billing migration %s attempt %d: %v",
					filepath.Base(path), attempt+1, err)
			}
		}
	}

	assertBillingSchemaShape(t, ctx, database, schema)
	verifyBillingDictionaryTranslations(t, ctx, database, filepath.Dir(paths[0]))
	seedCommerceFoundation(t, ctx, database)
	verifyRejectedOrderInvoiceAmounts(t, ctx, database)
	seedBillingFoundation(t, ctx, database)
	verifyBillingConstraints(t, ctx, database)
	verifyBillingConcurrentAccountUniqueness(t, ctx, database)
}

func verifyBillingDictionaryTranslations(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	manifestSQLDir string,
) {
	t.Helper()
	dictionaryTypes := []string{
		"idc_billing_account_type",
		"idc_credit_application_status",
		"idc_credit_account_status",
		"idc_credit_exposure_status",
		"idc_credit_statement_status",
		"idc_credit_repayment_status",
		"idc_credit_delinquency_status",
		"idc_credit_movement_type",
		"idc_credit_writeoff_type",
	}
	type dictionaryRow struct {
		Value string `orm:"value"`
	}
	for _, locale := range []string{"en-US", "zh-CN"} {
		content, err := os.ReadFile(filepath.Join(manifestSQLDir, "..", "i18n", locale, "dict.json"))
		if err != nil {
			t.Fatalf("read %s billing dictionary: %v", locale, err)
		}
		var document struct {
			Dict map[string]map[string]json.RawMessage `json:"dict"`
		}
		if err = json.Unmarshal(content, &document); err != nil {
			t.Fatalf("decode %s billing dictionary: %v", locale, err)
		}
		for _, dictionaryType := range dictionaryTypes {
			var rows []dictionaryRow
			if err = database.Model("sys_dict_data").Ctx(ctx).Fields("value").
				Where("dict_type", dictionaryType).Scan(&rows); err != nil {
				t.Fatalf("load %s dictionary rows: %v", dictionaryType, err)
			}
			databaseValues := make([]string, 0, len(rows))
			for _, row := range rows {
				databaseValues = append(databaseValues, row.Value)
			}
			sort.Strings(databaseValues)
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
			if strings.Join(localizedValues, "\x00") != strings.Join(databaseValues, "\x00") {
				t.Fatalf(
					"%s dictionary %s values=%v database=%v",
					locale, dictionaryType, localizedValues, databaseValues,
				)
			}
		}
	}
}

func verifyRejectedOrderInvoiceAmounts(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) {
	t.Helper()
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_invoice
    (invoice_no, customer_no, order_no, source_code, currency_code, status_code,
     item_count, subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, source_snapshot_digest, content_digest,
     payment_due_at, issued_at)
VALUES ('invoice-order-amount-mismatch', 'cus_commerce_test', 'order-commerce',
        'order', 'CNY', 'pending_payment', 1, 19901, 1000, 18901, 1134, 20035,
        repeat('1', 64), repeat('2', 64),
        CURRENT_TIMESTAMP + INTERVAL '30 minutes', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_invoice_item
    (invoice_item_no, invoice_no, customer_no, currency_code, line_number,
     item_type_code, order_no, order_item_no, source_reference,
     description_zh_cn, description_en_us, subtotal_minor, discount_minor,
     net_amount_minor, tax_amount_minor, total_amount_minor,
     refundable_cap_minor, content_digest)
VALUES ('invoice-item-order-amount-mismatch', 'invoice-order-amount-mismatch',
        'cus_commerce_test', 'CNY', 1, 'order_item', 'order-commerce',
        'order-item-commerce', 'order-item-commerce', '金额不一致',
        'Amount mismatch', 19901, 1000, 18901, 1134, 20035, 20035,
        repeat('3', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing accepted order invoice amounts that differ from the order")
	}
}

// TestBillingLedgerFoundationCodegenPostgreSQL resets only billing tables in
// the explicitly named disposable DAO code-generation database.
func TestBillingLedgerFoundationCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare billing DAO code-generation database", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create billing code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close billing code-generation database: %v", closeErr)
		}
	})
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	if _, err = database.Exec(ctx, billingCodegenCleanupSQL); err != nil {
		t.Fatalf("reset disposable billing code-generation tables: %v", err)
	}
	for _, path := range billingMigrationPaths(t)[6:] {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read billing code-generation migration %s: %v", filepath.Base(path), readErr)
		}
		for attempt := 0; attempt < 2; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute billing code-generation migration %s attempt %d: %v",
					filepath.Base(path), attempt+1, err)
			}
		}
	}
}

func assertBillingSchemaShape(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	schema string,
) {
	t.Helper()
	count, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_core_billing_%'`, schema)
	if err != nil || count.Int() != 17 {
		t.Fatalf("billing table count=%d want=17 err=%v", count.Int(), err)
	}
	deletedColumns, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_core_billing_%'
  AND column_name = 'deleted_at'`, schema)
	if err != nil || deletedColumns.Int() != 0 {
		t.Fatalf("billing soft-delete column count=%d want=0 err=%v", deletedColumns.Int(), err)
	}
	wrongAmountTypes, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_core_billing_%'
  AND column_name LIKE '%_minor'
  AND data_type <> 'bigint'`, schema)
	if err != nil || wrongAmountTypes.Int() != 0 {
		t.Fatalf("billing non-bigint minor-unit columns=%d want=0 err=%v", wrongAmountTypes.Int(), err)
	}
	dictTypes, err := database.GetValue(ctx, `
SELECT count(*) FROM sys_dict_type WHERE type LIKE 'idc_billing_%'`)
	if err != nil || dictTypes.Int() != 13 {
		t.Fatalf("billing dictionary type count=%d want=13 err=%v", dictTypes.Int(), err)
	}
	immutableTriggers, err := database.GetValue(ctx, `
SELECT count(DISTINCT trigger_name)
FROM information_schema.triggers
WHERE trigger_schema = ? AND trigger_name LIKE 'trg_idc_bill_%_immutable'`, schema)
	if err != nil || immutableTriggers.Int() != 13 {
		t.Fatalf("billing immutable trigger count=%d want=13 err=%v", immutableTriggers.Int(), err)
	}
	creditTables, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_core_credit_%'`, schema)
	if err != nil || creditTables.Int() != 14 {
		t.Fatalf("credit table count=%d want=14 err=%v", creditTables.Int(), err)
	}
	creditSoftDeletes, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_core_credit_%'
  AND column_name = 'deleted_at'`, schema)
	if err != nil || creditSoftDeletes.Int() != 0 {
		t.Fatalf("credit soft-delete column count=%d want=0 err=%v", creditSoftDeletes.Int(), err)
	}
	creditAmountTypes, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_core_credit_%'
  AND column_name LIKE '%_minor'
  AND data_type <> 'bigint'`, schema)
	if err != nil || creditAmountTypes.Int() != 0 {
		t.Fatalf("credit non-bigint minor-unit columns=%d want=0 err=%v", creditAmountTypes.Int(), err)
	}
	creditDictTypes, err := database.GetValue(ctx, `
SELECT count(*) FROM sys_dict_type WHERE type LIKE 'idc_credit_%'`)
	if err != nil || creditDictTypes.Int() != 8 {
		t.Fatalf("credit dictionary type count=%d want=8 err=%v", creditDictTypes.Int(), err)
	}
	creditImmutableTriggers, err := database.GetValue(ctx, `
SELECT count(DISTINCT trigger_name)
FROM information_schema.triggers
WHERE trigger_schema = ? AND trigger_name LIKE 'trg_idc_credit_%_immutable'`, schema)
	if err != nil || creditImmutableTriggers.Int() != 7 {
		t.Fatalf("credit immutable trigger count=%d want=7 err=%v", creditImmutableTriggers.Int(), err)
	}
}

func seedBillingFoundation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_invoice
    (invoice_no, customer_no, order_no, source_code, currency_code, status_code,
     item_count, subtotal_minor, discount_minor, net_amount_minor, tax_amount_minor,
     total_amount_minor, source_snapshot_digest, content_digest,
     payment_due_at, issued_at)
VALUES ('invoice-billing', 'cus_commerce_test', 'order-commerce', 'order', 'CNY',
        'pending_payment', 1, 19900, 1000, 18900, 1134, 20034,
        repeat('1', 64), repeat('2', 64), CURRENT_TIMESTAMP + INTERVAL '30 minutes',
        CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_invoice_item
    (invoice_item_no, invoice_no, customer_no, currency_code, line_number,
     item_type_code, order_no, order_item_no, source_reference,
     description_zh_cn, description_en_us, subtotal_minor, discount_minor,
     net_amount_minor, tax_amount_minor, total_amount_minor,
     refundable_cap_minor, content_digest)
VALUES ('invoice-item-billing', 'invoice-billing', 'cus_commerce_test', 'CNY', 1,
        'order_item', 'order-commerce', 'order-item-commerce', 'order-item-commerce',
        '交易云服务器', 'Commerce cloud server', 19900, 1000, 18900, 1134,
        20034, 20034, repeat('3', 64))`)
		return execErr
	})
	if err != nil {
		t.Fatalf("insert valid billing invoice: %v", err)
	}

	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_account
    (account_no, owner_type_code, owner_reference, customer_no,
     account_type_code, normal_balance_code, currency_code)
VALUES
    ('account-balance-cny', 'customer', 'cus_commerce_test', 'cus_commerce_test',
     'customer_balance', 'credit', 'CNY'),
    ('account-cash-cny', 'merchant', 'merchant', NULL,
     'merchant_cash', 'debit', 'CNY'),
    ('account-receivable-cny', 'merchant', 'merchant', NULL,
     'accounts_receivable', 'debit', 'CNY'),
    ('account-revenue-cny', 'merchant', 'merchant', NULL,
     'revenue', 'credit', 'CNY'),
    ('account-tax-cny', 'merchant', 'merchant', NULL,
     'tax_payable', 'credit', 'CNY'),
    ('account-cash-usd', 'merchant', 'merchant', NULL,
     'merchant_cash', 'debit', 'USD')`); err != nil {
		t.Fatalf("insert billing accounts: %v", err)
	}

	insertIssuanceLedger(t, ctx, database)
	insertPaymentLedgerAndAllocation(t, ctx, database)
	insertRefundLedgerAndAllocation(t, ctx, database)

	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_fund_hold
    (hold_no, customer_no, account_no, currency_code, source_type_code,
     source_reference, amount_minor, content_digest, expires_at)
VALUES ('hold-billing', 'cus_commerce_test', 'account-balance-cny', 'CNY',
        'invoice_payment', 'invoice-billing', 500, repeat('4', 64),
        CURRENT_TIMESTAMP + INTERVAL '15 minutes')`); err != nil {
		t.Fatalf("insert billing fund hold: %v", err)
	}

	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_adjustment_request
    (adjustment_no, customer_no, account_no, account_type_code, currency_code,
     adjustment_type_code, amount_minor, reason_code, evidence_digest,
     idempotency_digest, request_digest, approval_required, status_code, requested_by)
VALUES ('adjustment-billing', 'cus_commerce_test', 'account-balance-cny',
        'customer_balance', 'CNY',
        'balance_credit', 100, 'manual_correction', repeat('5', 64),
        repeat('6', 64), repeat('7', 64), TRUE, 'pending_review', 1001);
INSERT INTO plugin_idc_core_billing_adjustment_approval
    (approval_no, adjustment_no, requester_id_snapshot, reviewer_id,
     decision_code, comment_digest, evidence_digest)
VALUES ('approval-billing', 'adjustment-billing', 1001, 1002,
        'approved', repeat('8', 64), repeat('9', 64));
UPDATE plugin_idc_core_billing_adjustment_request
SET status_code = 'approved', version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE adjustment_no = 'adjustment-billing' AND status_code = 'pending_review'`)
		return execErr
	})
	if err != nil {
		t.Fatalf("insert reviewed billing adjustment: %v", err)
	}

	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_adjustment_request
    (adjustment_no, customer_no, account_no, account_type_code, currency_code,
     adjustment_type_code, amount_minor, reason_code, evidence_digest,
     idempotency_digest, request_digest, approval_required, status_code, requested_by)
VALUES ('adjustment-same-person-base', 'cus_commerce_test', 'account-balance-cny',
        'customer_balance', 'CNY', 'balance_credit', 1, 'approval_test',
        repeat('a', 64), repeat('b', 64), repeat('c', 64), TRUE,
        'pending_review', 1010)`); err != nil {
		t.Fatalf("insert pending billing adjustment: %v", err)
	}
}

func insertIssuanceLedger(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (transaction_no, transaction_type_code, currency_code, source_type_code,
     source_reference, source_snapshot_digest, entry_count,
     debit_total_minor, credit_total_minor, content_digest)
VALUES ('transaction-issuance', 'invoice_issuance', 'CNY', 'invoice_issuance',
        'invoice-billing', repeat('a', 64), 3, 20034, 20034, repeat('b', 64));
INSERT INTO plugin_idc_core_billing_ledger_entry
    (entry_no, transaction_no, currency_code, line_number, account_no,
     direction_code, entry_role_code, amount_minor, invoice_no, invoice_item_no,
     reason_code, content_digest)
VALUES
    ('entry-issuance-1', 'transaction-issuance', 'CNY', 1,
     'account-receivable-cny', 'debit', 'receivable', 20034,
     'invoice-billing', 'invoice-item-billing', 'invoice_issued', repeat('c', 64)),
    ('entry-issuance-2', 'transaction-issuance', 'CNY', 2,
     'account-revenue-cny', 'credit', 'revenue', 18900,
     'invoice-billing', 'invoice-item-billing', 'invoice_issued', repeat('d', 64)),
    ('entry-issuance-3', 'transaction-issuance', 'CNY', 3,
     'account-tax-cny', 'credit', 'tax', 1134,
     'invoice-billing', 'invoice-item-billing', 'invoice_issued', repeat('e', 64));
INSERT INTO plugin_idc_core_billing_balance_snapshot
    (snapshot_no, account_no, currency_code, normal_balance_code,
     transaction_no, sequence_no, opening_balance_minor,
     debit_amount_minor, credit_amount_minor, closing_balance_minor, content_digest)
VALUES
    ('snapshot-receivable-1', 'account-receivable-cny', 'CNY', 'debit',
     'transaction-issuance', 1, 0, 20034, 0, 20034, repeat('1', 64)),
    ('snapshot-revenue-1', 'account-revenue-cny', 'CNY', 'credit',
     'transaction-issuance', 1, 0, 0, 18900, 18900, repeat('2', 64)),
    ('snapshot-tax-1', 'account-tax-cny', 'CNY', 'credit',
     'transaction-issuance', 1, 0, 0, 1134, 1134, repeat('3', 64))`)
		return execErr
	})
	if err != nil {
		t.Fatalf("insert invoice issuance ledger: %v", err)
	}
}

func insertPaymentLedgerAndAllocation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (transaction_no, transaction_type_code, currency_code, source_type_code,
     source_reference, source_snapshot_digest, entry_count,
     debit_total_minor, credit_total_minor, content_digest)
VALUES ('transaction-payment', 'payment', 'CNY', 'channel_payment',
        'payment-billing', repeat('4', 64), 2, 20034, 20034, repeat('5', 64));
INSERT INTO plugin_idc_core_billing_ledger_entry
    (entry_no, transaction_no, currency_code, line_number, account_no,
     direction_code, entry_role_code, amount_minor, invoice_no,
     reason_code, content_digest)
VALUES
    ('entry-payment-1', 'transaction-payment', 'CNY', 1,
     'account-cash-cny', 'debit', 'cash', 20034,
     'invoice-billing', 'channel_payment', repeat('6', 64)),
    ('entry-payment-2', 'transaction-payment', 'CNY', 2,
     'account-receivable-cny', 'credit', 'receivable', 20034,
     'invoice-billing', 'channel_payment', repeat('7', 64));
INSERT INTO plugin_idc_core_billing_balance_snapshot
    (snapshot_no, account_no, currency_code, normal_balance_code,
     transaction_no, sequence_no, previous_snapshot_no, previous_sequence_no,
     opening_balance_minor, debit_amount_minor, credit_amount_minor,
     closing_balance_minor, content_digest)
VALUES
    ('snapshot-cash-1', 'account-cash-cny', 'CNY', 'debit',
     'transaction-payment', 1, NULL, NULL, 0, 20034, 0, 20034, repeat('8', 64)),
    ('snapshot-receivable-2', 'account-receivable-cny', 'CNY', 'debit',
     'transaction-payment', 2, 'snapshot-receivable-1', 1,
     20034, 0, 20034, 0, repeat('9', 64));
INSERT INTO plugin_idc_core_billing_payment_allocation
    (allocation_no, invoice_no, invoice_item_no, customer_no, currency_code,
     payment_source_type_code, payment_source_reference, ledger_transaction_no,
     ledger_transaction_type_code, amount_minor, content_digest)
VALUES ('payment-allocation-billing', 'invoice-billing', 'invoice-item-billing',
        'cus_commerce_test', 'CNY', 'channel_payment', 'payment-billing',
        'transaction-payment', 'payment', 20034, repeat('a', 64));
UPDATE plugin_idc_core_billing_invoice
SET status_code = 'paid', paid_amount_minor = 20034,
    refundable_amount_minor = 20034, settled_at = CURRENT_TIMESTAMP,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE invoice_no = 'invoice-billing' AND status_code = 'pending_payment'`)
		return execErr
	})
	if err != nil {
		t.Fatalf("insert invoice payment ledger and allocation: %v", err)
	}
}

func insertRefundLedgerAndAllocation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (transaction_no, transaction_type_code, currency_code, source_type_code,
     source_reference, source_snapshot_digest, entry_count,
     debit_total_minor, credit_total_minor, content_digest)
VALUES ('transaction-refund', 'refund', 'CNY', 'channel_refund',
        'refund-billing', repeat('b', 64), 2, 100, 100, repeat('c', 64));
INSERT INTO plugin_idc_core_billing_ledger_entry
    (entry_no, transaction_no, currency_code, line_number, account_no,
     direction_code, entry_role_code, amount_minor, invoice_no, invoice_item_no,
     reason_code, content_digest)
VALUES
    ('entry-refund-1', 'transaction-refund', 'CNY', 1,
     'account-revenue-cny', 'debit', 'refund', 100,
     'invoice-billing', 'invoice-item-billing', 'channel_refund', repeat('d', 64)),
    ('entry-refund-2', 'transaction-refund', 'CNY', 2,
     'account-cash-cny', 'credit', 'cash', 100,
     'invoice-billing', 'invoice-item-billing', 'channel_refund', repeat('e', 64));
INSERT INTO plugin_idc_core_billing_balance_snapshot
    (snapshot_no, account_no, currency_code, normal_balance_code,
     transaction_no, sequence_no, previous_snapshot_no, previous_sequence_no,
     opening_balance_minor, debit_amount_minor, credit_amount_minor,
     closing_balance_minor, content_digest)
VALUES
    ('snapshot-revenue-2', 'account-revenue-cny', 'CNY', 'credit',
     'transaction-refund', 2, 'snapshot-revenue-1', 1,
     18900, 100, 0, 18800, repeat('1', 64)),
    ('snapshot-cash-2', 'account-cash-cny', 'CNY', 'debit',
     'transaction-refund', 2, 'snapshot-cash-1', 1,
     20034, 0, 100, 19934, repeat('2', 64));
INSERT INTO plugin_idc_core_billing_refund_allocation
    (refund_allocation_no, invoice_no, invoice_item_no, customer_no, currency_code,
     payment_allocation_no, refund_source_type_code, refund_source_reference,
     ledger_transaction_no, ledger_transaction_type_code, amount_minor, content_digest)
VALUES ('refund-allocation-billing', 'invoice-billing', 'invoice-item-billing',
        'cus_commerce_test', 'CNY', 'payment-allocation-billing', 'channel_refund',
        'refund-billing', 'transaction-refund', 'refund', 100, repeat('3', 64));
UPDATE plugin_idc_core_billing_invoice
SET status_code = 'partially_refunded', refundable_amount_minor = 19934,
    refunded_amount_minor = 100, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE invoice_no = 'invoice-billing' AND status_code = 'paid'`)
		return execErr
	})
	if err != nil {
		t.Fatalf("insert invoice refund ledger and allocation: %v", err)
	}
}

func verifyBillingConstraints(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	for description, statement := range map[string]string{
		"duplicate original order invoice": `
INSERT INTO plugin_idc_core_billing_invoice
    (invoice_no, customer_no, order_no, source_code, currency_code, status_code,
     item_count, subtotal_minor, net_amount_minor, total_amount_minor,
     source_snapshot_digest, content_digest, payment_due_at, issued_at)
VALUES ('invoice-duplicate', 'cus_commerce_test', 'order-commerce', 'order', 'CNY',
        'pending_payment', 1, 1, 1, 1, repeat('1', 64), repeat('2', 64),
        CURRENT_TIMESTAMP + INTERVAL '10 minutes', CURRENT_TIMESTAMP)`,
		"order invoice customer mismatch": `
INSERT INTO plugin_idc_core_billing_invoice
    (invoice_no, customer_no, order_no, source_code, currency_code, status_code,
     item_count, subtotal_minor, net_amount_minor, total_amount_minor,
     source_snapshot_digest, content_digest, payment_due_at, issued_at)
VALUES ('invoice-order-customer-mismatch', 'cus_commerce_race', 'order-commerce',
        'order', 'CNY', 'pending_payment', 1, 1, 1, 1,
        repeat('1', 64), repeat('2', 64),
        CURRENT_TIMESTAMP + INTERVAL '10 minutes', CURRENT_TIMESTAMP)`,
		"cross-currency ledger entry": `
INSERT INTO plugin_idc_core_billing_ledger_entry
    (entry_no, transaction_no, currency_code, line_number, account_no,
     direction_code, entry_role_code, amount_minor, reason_code, content_digest)
VALUES ('entry-cross-currency', 'transaction-payment', 'CNY', 3,
        'account-cash-usd', 'debit', 'cash', 1, 'invalid_currency', repeat('3', 64))`,
		"payment allocation with mismatched transaction semantic": `
INSERT INTO plugin_idc_core_billing_payment_allocation
    (allocation_no, invoice_no, invoice_item_no, customer_no, currency_code,
     payment_source_type_code, payment_source_reference, ledger_transaction_no,
     ledger_transaction_type_code, amount_minor, content_digest)
VALUES ('payment-allocation-wrong-semantic', 'invoice-billing',
        'invoice-item-billing', 'cus_commerce_test', 'CNY', 'account_balance',
        'payment-wrong-semantic', 'transaction-payment', 'balance_payment', 1,
        repeat('3', 64))`,
		"withdrawable customer balance": `
INSERT INTO plugin_idc_core_billing_ledger_account
    (account_no, owner_type_code, owner_reference, customer_no,
     account_type_code, normal_balance_code, currency_code, withdrawable)
VALUES ('account-withdrawable', 'customer', 'cus_commerce_test', 'cus_commerce_test',
        'customer_balance', 'credit', 'USD', TRUE)`,
		"invalid hold buckets": `
INSERT INTO plugin_idc_core_billing_fund_hold
    (hold_no, customer_no, account_no, currency_code, source_type_code,
     source_reference, amount_minor, consumed_amount_minor, released_amount_minor,
     status_code, content_digest, expires_at, released_at)
VALUES ('hold-invalid', 'cus_commerce_test', 'account-balance-cny', 'CNY',
        'invoice_payment', 'invalid', 100, 80, 30, 'released', repeat('4', 64),
        CURRENT_TIMESTAMP + INTERVAL '10 minutes', CURRENT_TIMESTAMP)`,
		"same-person adjustment approval": `
INSERT INTO plugin_idc_core_billing_adjustment_approval
    (approval_no, adjustment_no, requester_id_snapshot, reviewer_id,
     decision_code, comment_digest, evidence_digest)
VALUES ('approval-invalid', 'adjustment-same-person-base', 1010, 1010,
        'approved', repeat('5', 64), repeat('6', 64))`,
		"customer adjustment targeting merchant account": `
INSERT INTO plugin_idc_core_billing_adjustment_request
    (adjustment_no, customer_no, account_no, account_type_code, currency_code,
     adjustment_type_code, amount_minor, reason_code, evidence_digest,
     idempotency_digest, request_digest, approval_required, status_code, requested_by)
VALUES ('adjustment-merchant-account', 'cus_commerce_test', 'account-cash-cny',
        'customer_balance', 'CNY', 'balance_credit', 1, 'invalid_target',
        repeat('7', 64), repeat('8', 64), repeat('9', 64), FALSE, 'approved', 3001)`,
	} {
		if _, err := database.Exec(ctx, statement); err == nil {
			t.Fatalf("billing accepted %s", description)
		}
	}

	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (transaction_no, transaction_type_code, currency_code, source_type_code,
     source_reference, source_snapshot_digest, entry_count,
     debit_total_minor, credit_total_minor, content_digest)
VALUES ('transaction-unbalanced', 'adjustment', 'CNY', 'test_unbalanced',
        'unbalanced', repeat('1', 64), 2, 100, 100, repeat('2', 64));
INSERT INTO plugin_idc_core_billing_ledger_entry
    (entry_no, transaction_no, currency_code, line_number, account_no,
     direction_code, entry_role_code, amount_minor, reason_code, content_digest)
VALUES
    ('entry-unbalanced-1', 'transaction-unbalanced', 'CNY', 1,
     'account-cash-cny', 'debit', 'adjustment', 100, 'test', repeat('3', 64)),
    ('entry-unbalanced-2', 'transaction-unbalanced', 'CNY', 2,
     'account-revenue-cny', 'credit', 'adjustment', 99, 'test', repeat('4', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing committed an unbalanced ledger transaction")
	}

	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_balance_snapshot
    (snapshot_no, account_no, currency_code, normal_balance_code,
     transaction_no, sequence_no, previous_snapshot_no, previous_sequence_no,
     opening_balance_minor, debit_amount_minor, credit_amount_minor,
     closing_balance_minor, content_digest)
VALUES ('snapshot-without-entry', 'account-revenue-cny', 'CNY', 'credit',
        'transaction-payment', 3, 'snapshot-revenue-2', 2,
        18800, 0, 1, 18801, repeat('2', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing committed a balance snapshot without a matching entry")
	}

	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (transaction_no, transaction_type_code, currency_code, source_type_code,
     source_reference, source_snapshot_digest, original_transaction_no,
     entry_count, debit_total_minor, credit_total_minor, content_digest)
VALUES ('transaction-reversal-over', 'reversal', 'CNY', 'test_reversal',
        'reversal-over', repeat('1', 64), 'transaction-refund',
        2, 101, 101, repeat('2', 64));
INSERT INTO plugin_idc_core_billing_ledger_entry
    (entry_no, transaction_no, currency_code, line_number, account_no,
     direction_code, entry_role_code, amount_minor, reason_code, content_digest)
VALUES
    ('entry-reversal-over-1', 'transaction-reversal-over', 'CNY', 1,
     'account-revenue-cny', 'credit', 'reversal', 101, 'test', repeat('3', 64)),
    ('entry-reversal-over-2', 'transaction-reversal-over', 'CNY', 2,
     'account-cash-cny', 'debit', 'reversal', 101, 'test', repeat('4', 64));
INSERT INTO plugin_idc_core_billing_balance_snapshot
    (snapshot_no, account_no, currency_code, normal_balance_code,
     transaction_no, sequence_no, previous_snapshot_no, previous_sequence_no,
     opening_balance_minor, debit_amount_minor, credit_amount_minor,
     closing_balance_minor, content_digest)
VALUES
    ('snapshot-reversal-revenue-over', 'account-revenue-cny', 'CNY', 'credit',
     'transaction-reversal-over', 3, 'snapshot-revenue-2', 2,
     18800, 0, 101, 18901, repeat('5', 64)),
    ('snapshot-reversal-cash-over', 'account-cash-cny', 'CNY', 'debit',
     'transaction-reversal-over', 3, 'snapshot-cash-2', 2,
     19934, 101, 0, 20035, repeat('6', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing committed a reversal above the original transaction")
	}

	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_invoice
    (invoice_no, customer_no, source_code, currency_code, status_code,
     item_count, subtotal_minor, net_amount_minor, total_amount_minor,
     source_snapshot_digest, content_digest, payment_due_at, issued_at)
VALUES ('invoice-item-mismatch', 'cus_commerce_test', 'manual', 'CNY',
        'pending_payment', 1, 100, 100, 100, repeat('1', 64), repeat('2', 64),
        CURRENT_TIMESTAMP + INTERVAL '10 minutes', CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_invoice_item
    (invoice_item_no, invoice_no, customer_no, currency_code, line_number,
     item_type_code, source_reference, description_zh_cn, description_en_us,
     subtotal_minor, net_amount_minor, total_amount_minor,
     refundable_cap_minor, content_digest)
VALUES ('invoice-item-mismatch-line', 'invoice-item-mismatch', 'cus_commerce_test',
        'CNY', 1, 'manual', 'mismatch', '不匹配', 'Mismatch', 99, 99, 99, 99,
        repeat('3', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing committed invoice totals that differ from immutable items")
	}

	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_ledger_transaction
    (transaction_no, transaction_type_code, currency_code, source_type_code,
     source_reference, source_snapshot_digest, entry_count,
     debit_total_minor, credit_total_minor, content_digest)
VALUES ('transaction-negative-balance', 'adjustment', 'CNY', 'test_negative_balance',
        'negative-balance', repeat('1', 64), 2, 1, 1, repeat('2', 64));
INSERT INTO plugin_idc_core_billing_ledger_entry
    (entry_no, transaction_no, currency_code, line_number, account_no,
     direction_code, entry_role_code, amount_minor, reason_code, content_digest)
VALUES
    ('entry-negative-balance-1', 'transaction-negative-balance', 'CNY', 1,
     'account-balance-cny', 'debit', 'balance', 1, 'test', repeat('3', 64)),
    ('entry-negative-balance-2', 'transaction-negative-balance', 'CNY', 2,
     'account-revenue-cny', 'credit', 'revenue', 1, 'test', repeat('4', 64));
INSERT INTO plugin_idc_core_billing_balance_snapshot
    (snapshot_no, account_no, currency_code, normal_balance_code,
     transaction_no, sequence_no, previous_snapshot_no, previous_sequence_no,
     opening_balance_minor, debit_amount_minor, credit_amount_minor,
     closing_balance_minor, content_digest)
VALUES
    ('snapshot-negative-balance', 'account-balance-cny', 'CNY', 'credit',
     'transaction-negative-balance', 1, NULL, NULL, 0, 1, 0, -1, repeat('5', 64)),
    ('snapshot-revenue-negative-3', 'account-revenue-cny', 'CNY', 'credit',
     'transaction-negative-balance', 3, 'snapshot-revenue-2', 2,
     18800, 0, 1, 18801, repeat('6', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing committed a negative protected customer balance")
	}

	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_payment_allocation
    (allocation_no, invoice_no, invoice_item_no, customer_no, currency_code,
     payment_source_type_code, payment_source_reference, ledger_transaction_no,
     ledger_transaction_type_code, amount_minor, content_digest)
VALUES ('payment-allocation-over', 'invoice-billing', 'invoice-item-billing',
        'cus_commerce_test', 'CNY', 'channel_payment', 'payment-over',
        'transaction-payment', 'payment', 1, repeat('5', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing committed payment allocations above invoice paid amount")
	}

	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_refund_allocation
    (refund_allocation_no, invoice_no, invoice_item_no, customer_no, currency_code,
     payment_allocation_no, refund_source_type_code, refund_source_reference,
     ledger_transaction_no, ledger_transaction_type_code, amount_minor, content_digest)
VALUES ('refund-allocation-over', 'invoice-billing', 'invoice-item-billing',
        'cus_commerce_test', 'CNY', 'payment-allocation-billing', 'channel_refund',
        'refund-over', 'transaction-refund', 'refund', 20035, repeat('6', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing committed refund allocations above item and payment caps")
	}

	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, execErr := database.Exec(ctx, `
INSERT INTO plugin_idc_core_billing_adjustment_request
    (adjustment_no, customer_no, account_no, account_type_code, currency_code,
     adjustment_type_code, amount_minor, reason_code, evidence_digest,
     idempotency_digest, request_digest, approval_required, status_code, requested_by)
VALUES ('adjustment-pending-invalid', 'cus_commerce_test', 'account-balance-cny',
        'customer_balance', 'CNY',
        'balance_credit', 1, 'test', repeat('1', 64), repeat('2', 64),
        repeat('3', 64), TRUE, 'pending_review', 2001);
INSERT INTO plugin_idc_core_billing_adjustment_approval
    (approval_no, adjustment_no, requester_id_snapshot, reviewer_id,
     decision_code, comment_digest, evidence_digest)
VALUES ('approval-pending-invalid', 'adjustment-pending-invalid', 2001, 2002,
        'approved', repeat('4', 64), repeat('5', 64))`)
		return execErr
	}); err == nil {
		t.Fatal("billing left a decided adjustment in pending review")
	}

	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_billing_ledger_entry
SET amount_minor = amount_minor
WHERE entry_no = 'entry-payment-1'`); err == nil {
		t.Fatal("billing allowed immutable ledger entry update")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_billing_invoice
SET customer_no = 'cus_commerce_race'
WHERE invoice_no = 'invoice-billing'`); err == nil {
		t.Fatal("billing allowed invoice commercial identity update")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_billing_fund_hold
SET amount_minor = amount_minor + 1
WHERE hold_no = 'hold-billing'`); err == nil {
		t.Fatal("billing allowed fund hold identity update")
	}
	if _, err := database.Exec(ctx, `
DELETE FROM plugin_idc_core_billing_payment_allocation
WHERE allocation_no = 'payment-allocation-billing'`); err == nil {
		t.Fatal("billing allowed immutable payment allocation delete")
	}
}

func verifyBillingConcurrentAccountUniqueness(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
) {
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
INSERT INTO plugin_idc_core_billing_ledger_account
    (account_no, owner_type_code, owner_reference, customer_no,
     account_type_code, normal_balance_code, currency_code)
VALUES (?, 'customer', 'cus_commerce_test', 'cus_commerce_test',
        'customer_credit', 'debit', 'USD')`,
				"account-credit-race-"+string(rune('0'+index)))
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
		t.Fatalf("concurrent billing accounts successes=%d failures=%d want=1/1",
			successes, failures)
	}
}

func newBillingIntegrationSchema(t *testing.T) string {
	t.Helper()
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("generate billing integration schema suffix: %v", err)
	}
	return "linapro_billing_test_" + hex.EncodeToString(random)
}

func billingMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve billing migration test path")
	}
	migrationRoot := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	return []string{
		filepath.Join(migrationRoot, "001-core-foundation.sql"),
		filepath.Join(migrationRoot, "010-catalog-foundation.sql"),
		filepath.Join(migrationRoot, "013-catalog-billing-policy.sql"),
		filepath.Join(migrationRoot, "014-catalog-inventory-routing.sql"),
		filepath.Join(migrationRoot, "019-catalog-tax-policy.sql"),
		filepath.Join(migrationRoot, "022-commerce-foundation.sql"),
		filepath.Join(migrationRoot, "023-billing-ledger-foundation.sql"),
		filepath.Join(migrationRoot, "027-billing-invoice-lifecycle.sql"),
		filepath.Join(migrationRoot, "028-billing-invoice-transform.sql"),
		filepath.Join(migrationRoot, "029-billing-reviewed-adjustment.sql"),
		filepath.Join(migrationRoot, "030-service-renewal-foundation.sql"),
		filepath.Join(migrationRoot, "031-service-renewal-execution.sql"),
		filepath.Join(migrationRoot, "032-service-expiry-scheduling.sql"),
		filepath.Join(migrationRoot, "033-billing-directory-indexes.sql"),
		filepath.Join(migrationRoot, "034-credit-account-foundation.sql"),
		filepath.Join(migrationRoot, "035-financial-settlement.sql"),
		filepath.Join(migrationRoot, "036-trial-qualification.sql"),
		filepath.Join(migrationRoot, "037-financial-settlement-fulfillment.sql"),
		filepath.Join(migrationRoot, "051-balance-recharge-context.sql"),
		filepath.Join(migrationRoot, "052-online-refund.sql"),
		filepath.Join(migrationRoot, "053-credit-refund-lifecycle.sql"),
		filepath.Join(migrationRoot, "054-credit-online-refund-integration.sql"),
		filepath.Join(migrationRoot, "055-credit-evidence.sql"),
		filepath.Join(migrationRoot, "073-general-product-catalog.sql"),
	}
}

const billingCodegenCleanupSQL = `
DROP TABLE IF EXISTS plugin_idc_core_credit_account_movement CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_writeoff CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_delinquency_case CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_repayment_allocation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_repayment CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_statement_line CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_statement CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_exposure CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_account_terms CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_account CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_application_review CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_evidence_upload CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_application CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_credit_policy_revision CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_invoice_waiver_allocation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_adjustment_policy CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_invoice_item_derivation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_invoice_transform_destination CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_invoice_transform_source CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_invoice_transform CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_adjustment_approval CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_adjustment_request CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_refund_allocation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_payment_allocation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_fund_hold CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_balance_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_ledger_entry CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_ledger_transaction CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_ledger_account CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_invoice_item CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_billing_invoice CASCADE;`
