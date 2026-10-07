// This file verifies the marketing foundation and concurrent quota invariants in PostgreSQL.

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

func TestMarketingFoundationMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL marketing migration tests", catalogIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create marketing migration admin database: %v", err)
	}
	schema := newMarketingIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		_ = adminDB.Close(ctx)
		t.Fatalf("create isolated marketing schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_marketing_test_") {
			if _, dropErr := adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE"); dropErr != nil {
				t.Errorf("drop isolated marketing schema: %v", dropErr)
			}
		} else {
			t.Errorf("refusing to drop unexpected marketing schema %q", schema)
		}
		if closeErr := adminDB.Close(ctx); closeErr != nil {
			t.Errorf("close marketing migration admin database: %v", closeErr)
		}
	})

	database, err := gdb.New(gdb.ConfigNode{
		Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("create scoped marketing database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped marketing database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, catalogDictionaryFixtureSQL); err != nil {
		t.Fatalf("create marketing dictionary fixtures: %v", err)
	}
	paths := marketingMigrationPaths(t)
	for index, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read marketing migration %s: %v", filepath.Base(path), readErr)
		}
		attempts := 1
		if index == len(paths)-1 {
			attempts = 2
		}
		for attempt := 0; attempt < attempts; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute marketing migration %s attempt %d: %v", filepath.Base(path), attempt+1, err)
			}
		}
	}

	assertMarketingSchemaShape(t, ctx, database, schema)
	seedMarketingFoundation(t, ctx, database)
	verifyMarketingQuotaConcurrency(t, ctx, database)
	verifyMarketingConservationConstraints(t, ctx, database)
}

func assertMarketingSchemaShape(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	schema string,
) {
	t.Helper()
	count, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.tables
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_core_marketing_%'`, schema)
	if err != nil || count.Int() != 21 {
		t.Fatalf("marketing table count=%d want=21 err=%v", count.Int(), err)
	}
	deletedColumns, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_core_marketing_%'
  AND column_name = 'deleted_at'`, schema)
	if err != nil || deletedColumns.Int() != 0 {
		t.Fatalf("marketing soft-delete column count=%d want=0 err=%v", deletedColumns.Int(), err)
	}
	wrongAmountTypes, err := database.GetValue(ctx, `
SELECT count(*)
FROM information_schema.columns
WHERE table_schema = ?
  AND table_name LIKE 'plugin_idc_core_marketing_%'
  AND (column_name LIKE '%_amount_minor' OR column_name LIKE '%_delta_minor')
  AND data_type <> 'bigint'`, schema)
	if err != nil || wrongAmountTypes.Int() != 0 {
		t.Fatalf("marketing non-bigint amount columns=%d want=0 err=%v", wrongAmountTypes.Int(), err)
	}
}

// TestMarketingFoundationCodegenPostgreSQL resets only marketing tables in
// the explicitly named disposable DAO code-generation database.
func TestMarketingFoundationCodegenPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogCodegenDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to prepare the marketing DAO code-generation database", catalogCodegenDSNEnv)
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create marketing code-generation database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close marketing code-generation database: %v", closeErr)
		}
	})
	name, err := database.GetValue(ctx, "SELECT current_database()")
	if err != nil || name.String() != "linapro_idc_codegen" {
		t.Fatalf("refusing non-codegen database %q: %v", name.String(), err)
	}
	if _, err = database.Exec(ctx, marketingCodegenCleanupSQL); err != nil {
		t.Fatalf("reset disposable marketing code-generation tables: %v", err)
	}
	for _, migrationPath := range marketingMigrationPaths(t)[2:] {
		migration, readErr := os.ReadFile(migrationPath)
		if readErr != nil {
			t.Fatalf("read marketing code-generation migration: %v", readErr)
		}
		for attempt := 0; attempt < 2; attempt++ {
			if _, err = database.Exec(ctx, string(migration)); err != nil {
				t.Fatalf("execute marketing code-generation migration %s attempt %d: %v",
					filepath.Base(migrationPath), attempt+1, err)
			}
		}
	}
}

func seedMarketingFoundation(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_subject
    (customer_no, customer_type_code, status_code, display_name)
VALUES
    ('cus_marketing_test', 'individual', 'active', 'Marketing Test'),
    ('cus_marketing_other', 'individual', 'active', 'Marketing Other');

INSERT INTO plugin_idc_core_marketing_segment
    (segment_no, segment_code, name_zh_cn, name_en_us, mode_code,
     refresh_interval_minutes, definition_digest, status_code)
VALUES
    ('seg_dynamic_test', 'dynamic_test', '动态测试', 'Dynamic test', 'dynamic',
     60, repeat('a', 64), 'active');

INSERT INTO plugin_idc_core_marketing_segment_condition
    (condition_no, segment_no, or_group_no, sort_order, fact_code, operator_code,
     value_kind_code, value_integer_lower, value_integer_upper, currency_code)
VALUES
    ('cond_spend_test', 'seg_dynamic_test', 1, 1, 'paid_amount', 'between',
     'currency_amount', 10000, 50000, 'CNY');

INSERT INTO plugin_idc_core_marketing_segment_member
    (segment_no, customer_no, source_code, segment_version)
VALUES ('seg_dynamic_test', 'cus_marketing_test', 'rule', 1);

INSERT INTO plugin_idc_core_marketing_segment_snapshot
    (snapshot_no, segment_no, segment_version, definition_digest, status_code,
     member_count, content_digest, completed_at)
VALUES
    ('snap_dynamic_test', 'seg_dynamic_test', 1, repeat('a', 64), 'complete',
     1, repeat('d', 64), CURRENT_TIMESTAMP);

INSERT INTO plugin_idc_core_marketing_snapshot_member
    (snapshot_no, customer_no)
VALUES ('snap_dynamic_test', 'cus_marketing_test');

INSERT INTO plugin_idc_core_marketing_campaign
    (campaign_no, campaign_code, name_zh_cn, name_en_us, campaign_type_code,
     target_segment_no, target_snapshot_no, starts_at, ends_at, global_quota_limit,
     per_customer_quota_limit, status_code)
VALUES
    ('cmp_quota_test', 'quota_test', '名额测试', 'Quota test', 'coupon',
     'seg_dynamic_test', 'snap_dynamic_test', CURRENT_TIMESTAMP - INTERVAL '1 hour',
     CURRENT_TIMESTAMP + INTERVAL '1 day', 1, 1, 'active');

INSERT INTO plugin_idc_core_marketing_campaign
    (campaign_no, campaign_code, name_zh_cn, name_en_us, campaign_type_code,
     starts_at, ends_at, status_code)
VALUES
    ('cmp_bonus_test', 'bonus_test', '赠送测试', 'Bonus test', 'recharge_bonus',
     CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '1 day',
     'active');

INSERT INTO plugin_idc_core_marketing_campaign_scope
    (campaign_no, scope_kind_code, match_mode_code, reference_code)
VALUES ('cmp_quota_test', 'all_products', 'include', '');

INSERT INTO plugin_idc_core_marketing_benefit_rule
    (benefit_no, campaign_no, campaign_type_code, benefit_type_code,
     currency_code, amount_minor,
     purchase_phase_code, cycle_scope_code)
VALUES
    ('ben_coupon_test', 'cmp_quota_test', 'coupon', 'fixed_discount', 'CNY', 1000,
     'new_purchase', 'first'),
    ('ben_bonus_test', 'cmp_bonus_test', 'recharge_bonus', 'recharge_bonus', 'CNY', 5000,
     'both', 'all');

INSERT INTO plugin_idc_core_marketing_customer_quota
    (campaign_no, customer_no, quota_limit)
VALUES ('cmp_quota_test', 'cus_marketing_test', 1);

INSERT INTO plugin_idc_core_marketing_promotion_asset
    (asset_no, campaign_no, benefit_no, benefit_type_code, asset_type_code, customer_no,
     claim_code_digest, benefit_snapshot_digest, currency_code, face_amount_minor,
     valid_from, valid_until, status_code, claimed_at)
VALUES
    ('asset_marketing_test', 'cmp_quota_test', 'ben_coupon_test', 'fixed_discount', 'coupon',
     'cus_marketing_test', decode(repeat('01', 32), 'hex'), repeat('b', 64), 'CNY',
     1000, CURRENT_TIMESTAMP - INTERVAL '1 hour', CURRENT_TIMESTAMP + INTERVAL '1 day',
     'active', CURRENT_TIMESTAMP);`)
	if err != nil {
		t.Fatalf("seed marketing foundation: %v", err)
	}
}

func verifyMarketingQuotaConcurrency(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if succeeded := runConcurrentConditionalUpdates(t, ctx, database, `
UPDATE plugin_idc_core_marketing_campaign
SET global_reserved_count = global_reserved_count + 1,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_no = 'cmp_quota_test'
  AND global_quota_limit IS NOT NULL
  AND global_reserved_count + global_consumed_count < global_quota_limit`); succeeded != 1 {
		t.Fatalf("expected one campaign quota reservation, got %d", succeeded)
	}
	if succeeded := runConcurrentConditionalUpdates(t, ctx, database, `
UPDATE plugin_idc_core_marketing_customer_quota
SET reserved_count = reserved_count + 1,
    version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE campaign_no = 'cmp_quota_test' AND customer_no = 'cus_marketing_test'
  AND reserved_count + consumed_count < quota_limit`); succeeded != 1 {
		t.Fatalf("expected one customer quota reservation, got %d", succeeded)
	}
	if succeeded := runConcurrentConditionalUpdates(t, ctx, database, `
UPDATE plugin_idc_core_marketing_promotion_asset
SET reserved_uses = reserved_uses + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE asset_no = 'asset_marketing_test'
  AND reserved_uses + consumed_uses < usage_limit`); succeeded != 1 {
		t.Fatalf("expected one promotion asset reservation, got %d", succeeded)
	}
	reservedValue, err := database.GetValue(ctx, `
SELECT global_reserved_count
FROM plugin_idc_core_marketing_campaign
WHERE campaign_no = 'cmp_quota_test'`)
	if err != nil || reservedValue.Int() != 1 {
		t.Fatalf("campaign reserved count=%d err=%v", reservedValue.Int(), err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_campaign
SET global_consumed_count = 1
WHERE campaign_no = 'cmp_quota_test'`); err == nil {
		t.Fatal("expected database quota conservation constraint to reject over-consumption")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_promotion_asset
SET consumed_uses = 1
WHERE asset_no = 'asset_marketing_test'`); err == nil {
		t.Fatal("expected asset usage conservation constraint to reject over-consumption")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_campaign_reservation
    (reservation_no, campaign_no, benefit_no, customer_no, quote_reference,
     request_snapshot_digest, currency_code, quoted_value_minor,
     benefit_snapshot_digest, expires_at)
VALUES
    ('campaign_res_test', 'cmp_quota_test', 'ben_coupon_test', 'cus_marketing_test',
     'quote-campaign-reservation', repeat('f', 64), 'CNY', 1000, repeat('e', 64),
     CURRENT_TIMESTAMP + INTERVAL '10 minutes')`); err != nil {
		t.Fatalf("insert campaign reservation evidence: %v", err)
	}
}

func runConcurrentConditionalUpdates(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	query string,
) int64 {
	t.Helper()
	start := make(chan struct{})
	results := make(chan int64, 2)
	errors := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			result, err := database.Exec(ctx, query)
			if err != nil {
				errors <- err
				return
			}
			rows, rowsErr := result.RowsAffected()
			if rowsErr != nil {
				errors <- rowsErr
				return
			}
			results <- rows
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatalf("execute concurrent conditional update: %v", err)
		}
	}
	succeeded := int64(0)
	for rows := range results {
		succeeded += rows
	}
	return succeeded
}

func verifyMarketingConservationConstraints(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_points_account
    (customer_no, available_points)
VALUES ('cus_marketing_test', 100);

INSERT INTO plugin_idc_core_marketing_points_lot
    (lot_no, customer_no, source_type_code, source_reference, original_points, available_points)
VALUES ('lot_marketing_test', 'cus_marketing_test', 'activity', 'activity-test', 100, 100);

INSERT INTO plugin_idc_core_marketing_value_grant
    (grant_no, grant_type_code, campaign_no, campaign_type_code,
     benefit_no, benefit_type_code, benefit_snapshot_digest, customer_no, currency_code,
     source_reference, original_amount_minor, pending_amount_minor, available_amount_minor,
     status_code, financial_posting_reference)
VALUES
    ('grant_marketing_test', 'recharge_bonus', 'cmp_bonus_test', 'recharge_bonus',
     'ben_bonus_test', 'recharge_bonus', repeat('c', 64),
     'cus_marketing_test', 'CNY', 'recharge-test', 5000, 0, 5000, 'available',
     'ledger-posting-test'),
    ('grant_pending_test', 'recharge_bonus', 'cmp_bonus_test', 'recharge_bonus',
     'ben_bonus_test', 'recharge_bonus', repeat('c', 64),
     'cus_marketing_test', 'CNY', 'recharge-pending-test', 3000, 3000, 0, 'pending', NULL);`); err != nil {
		t.Fatalf("seed marketing conservation rows: %v", err)
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_lot
SET consumed_points = 1
WHERE lot_no = 'lot_marketing_test'`); err == nil {
		t.Fatal("expected points lot conservation constraint to reject imbalance")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_points_lot
SET reversal_shortfall_points = consumed_points + 1
WHERE lot_no = 'lot_marketing_test'`); err == nil {
		t.Fatal("expected points reversal shortfall to stay within consumed points")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_value_grant
SET consumed_amount_minor = 1
WHERE grant_no = 'grant_marketing_test'`); err == nil {
		t.Fatal("expected marketing value conservation constraint to reject imbalance")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_value_grant
SET withdrawable = TRUE
WHERE grant_no = 'grant_marketing_test'`); err == nil {
		t.Fatal("expected recharge bonus withdrawal constraint to reject withdrawable funds")
	}
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_marketing_value_grant
SET status_code = 'available', financial_posting_reference = 'posting-without-transfer'
WHERE grant_no = 'grant_pending_test'`); err == nil {
		t.Fatal("expected pending grant to transfer its complete amount before posting")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_quiet_hours
    (customer_no, timezone_name, start_minute, end_minute, enabled)
VALUES ('cus_marketing_test', 'UTC', 60, 60, TRUE)`); err == nil {
		t.Fatal("expected enabled marketing quiet hours to require a non-empty window")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_customer_quota
    (campaign_no, customer_no, quota_limit)
VALUES ('cmp_quota_test', 'cus_marketing_other', 2)`); err == nil {
		t.Fatal("expected customer quota to match the campaign per-customer limit")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_points_entry
    (entry_no, customer_no, lot_no, entry_type_code, available_delta,
     source_type_code, source_reference)
VALUES
    ('pte_cross_customer', 'cus_marketing_other', 'lot_marketing_test', 'spend', -1,
     'redemption', 'cross-customer')`); err == nil {
		t.Fatal("expected points entry cross-customer lot reference to fail")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_value_entry
    (entry_no, grant_no, customer_no, currency_code, entry_type_code,
     frozen_delta_minor, consumed_delta_minor, source_type_code, source_reference)
VALUES
    ('mve_cross_customer', 'grant_marketing_test', 'cus_marketing_other', 'CNY',
     'consume', -1, 1, 'order', 'cross-customer')`); err == nil {
		t.Fatal("expected value entry cross-customer grant reference to fail")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_asset_reservation
    (reservation_no, asset_no, customer_no, quote_reference, currency_code,
     discount_amount_minor, expires_at)
VALUES
    ('res_cross_customer', 'asset_marketing_test', 'cus_marketing_other',
     'quote-cross-customer', 'CNY', 1000, CURRENT_TIMESTAMP + INTERVAL '10 minutes')`); err == nil {
		t.Fatal("expected asset reservation cross-customer reference to fail")
	}
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_marketing_promotion_asset
    (asset_no, campaign_no, benefit_no, benefit_type_code, asset_type_code,
     benefit_snapshot_digest,
     currency_code, rate_basis_points, valid_from, valid_until)
VALUES
    ('asset_invalid_voucher', 'cmp_quota_test', 'ben_coupon_test', 'fixed_discount', 'voucher',
     repeat('c', 64), 'CNY', 1000, CURRENT_TIMESTAMP,
     CURRENT_TIMESTAMP + INTERVAL '1 day')`); err == nil {
		t.Fatal("expected percentage voucher to fail face-value constraint")
	}
}

func newMarketingIntegrationSchema(t *testing.T) string {
	t.Helper()
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		t.Fatalf("generate marketing schema suffix: %v", err)
	}
	return "linapro_marketing_test_" + hex.EncodeToString(value[:])
}

func marketingMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve marketing migration test path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "manifest", "sql")
	return []string{
		filepath.Join(root, "001-core-foundation.sql"),
		filepath.Join(root, "003-customer-account.sql"),
		filepath.Join(root, "020-marketing-foundation.sql"),
		filepath.Join(root, "021-marketing-send-governance.sql"),
	}
}

const marketingCodegenCleanupSQL = `
DROP TABLE IF EXISTS plugin_idc_core_marketing_channel_consent CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_channel_consent_event CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_quiet_hours CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_value_entry CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_value_grant CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_points_entry CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_points_lot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_points_account CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_asset_reservation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_promotion_asset CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_campaign_reservation CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_customer_quota CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_benefit_rule CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_campaign_scope CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_campaign CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_snapshot_member CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_segment_snapshot CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_segment_member CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_condition_value CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_segment_condition CASCADE;
DROP TABLE IF EXISTS plugin_idc_core_marketing_segment CASCADE;`
