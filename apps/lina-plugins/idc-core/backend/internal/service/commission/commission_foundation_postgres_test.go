// This file executes the stage-12 sales-channel migration against an isolated PostgreSQL schema.

package commission

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

const commissionIntegrationDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

func TestCommissionFoundationMigrationPostgreSQL(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(commissionIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL commission migration tests", commissionIntegrationDSNEnv)
	}
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("create commission migration admin database: %v", err)
	}
	schema := newCommissionTestSchema(t)
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoteCommissionIdentifier(schema)); err != nil {
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close commission migration admin database after schema failure: %v", closeErr)
		}
		t.Fatalf("create isolated commission schema: %v", err)
	}
	t.Cleanup(func() {
		if strings.HasPrefix(schema, "linapro_commission_test_") {
			// The full host migration creates hundreds of foreign keys and
			// triggers. Dropping the schema in one CASCADE statement can exceed
			// PostgreSQL's max_locks_per_transaction, so remove tables one at a
			// time in separate transactions before dropping the now-empty schema.
			// Functions created by host migrations are schema-scoped but are not
			// returned by pg_tables. Drop them first so the final schema drop does
			// not depend on a single transaction holding every migration object.
			var functions []struct {
				Signature string `orm:"signature"`
			}
			if scanErr := admin.GetScan(ctx, &functions, `SELECT p.oid::regprocedure::text AS signature FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=?`, schema); scanErr != nil {
				t.Errorf("list isolated commission functions: %v", scanErr)
			}
			for _, function := range functions {
				if _, dropErr := admin.Exec(ctx, "DROP FUNCTION IF EXISTS "+function.Signature+" CASCADE"); dropErr != nil {
					t.Errorf("drop isolated commission function %s: %v", function.Signature, dropErr)
				}
			}

			var tables []struct {
				TableName string `orm:"tablename"`
			}
			if scanErr := admin.GetScan(ctx, &tables, `SELECT tablename FROM pg_tables WHERE schemaname=?`, schema); scanErr != nil {
				t.Errorf("list isolated commission tables: %v", scanErr)
			}
			for _, table := range tables {
				if _, dropErr := admin.Exec(ctx, "DROP TABLE IF EXISTS "+quoteCommissionIdentifier(schema)+"."+quoteCommissionIdentifier(table.TableName)+" CASCADE"); dropErr != nil {
					t.Errorf("drop isolated commission table %s.%s: %v", schema, table.TableName, dropErr)
				}
			}
			if _, dropErr := admin.Exec(ctx, "DROP SCHEMA "+quoteCommissionIdentifier(schema)+" CASCADE"); dropErr != nil {
				t.Errorf("drop isolated commission schema: %v", dropErr)
			}
		} else {
			t.Errorf("refusing to drop unexpected commission schema %q", schema)
		}
		if closeErr := admin.Close(ctx); closeErr != nil {
			t.Errorf("close commission migration admin database: %v", closeErr)
		}
	})

	database, err := gdb.New(gdb.ConfigNode{
		Link: appendCommissionDSNParameter(dsn, "search_path="+url.QueryEscape(schema)),
	})
	if err != nil {
		t.Fatalf("create scoped commission database: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := database.Close(ctx); closeErr != nil {
			t.Errorf("close scoped commission database: %v", closeErr)
		}
	})
	if _, err = database.Exec(ctx, commissionHostFixtureSQL); err != nil {
		t.Fatalf("create commission host fixtures: %v", err)
	}
	version, err := database.GetValue(ctx, "SHOW server_version_num")
	if err != nil {
		t.Fatalf("read PostgreSQL version: %v", err)
	}
	t.Logf("PostgreSQL server_version_num=%s; migration syntax is statically limited to PostgreSQL 14 features", version.String())

	paths := commissionMigrationPaths(t)
	for _, path := range paths {
		migration, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read commission migration %s: %v", filepath.Base(path), readErr)
		}
		if _, err = database.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("execute commission migration %s: %v", filepath.Base(path), err)
		}
	}
	// Replay the commission migration itself even after newer modules append SQL.
	commissionPath := filepath.Join(filepath.Dir(paths[0]), "061-commission-refund-reversal.sql")
	latest, err := os.ReadFile(commissionPath)
	if err != nil {
		t.Fatalf("read commission migration for idempotence: %v", err)
	}
	if _, err = database.Exec(ctx, string(latest)); err != nil {
		t.Fatalf("replay commission migration: %v", err)
	}

	assertCommissionSchemaShape(t, ctx, database, schema)
	assertCommissionConstraints(t, ctx, database)
}

func assertCommissionSchemaShape(t *testing.T, ctx context.Context, database gdb.DB, schema string) {
	t.Helper()
	tables, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM information_schema.tables
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_core_sales_%'`, schema)
	if err != nil || tables.Int() != 29 {
		t.Fatalf("sales foundation table count=%d want=29 err=%v", tables.Int(), err)
	}
	deletedColumns, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_core_sales_%'
  AND column_name = 'deleted_at'`, schema)
	if err != nil || deletedColumns.Int() != 0 {
		t.Fatalf("sales soft-delete column count=%d want=0 err=%v", deletedColumns.Int(), err)
	}
	wrongMoneyTypes, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema = ? AND table_name LIKE 'plugin_idc_core_sales_%'
  AND column_name LIKE '%_minor' AND data_type <> 'bigint'`, schema)
	if err != nil || wrongMoneyTypes.Int() != 0 {
		t.Fatalf("sales non-bigint money columns=%d want=0 err=%v", wrongMoneyTypes.Int(), err)
	}

	for _, indexName := range []string{
		"idx_plugin_idc_core_agent_application_list",
		"idx_plugin_idc_core_affiliate_touch_lookup",
		"idx_plugin_idc_core_sales_assignment_owner",
		"idx_plugin_idc_core_commission_maturity",
		"uk_plugin_idc_core_commission_maturity_terminal",
		"idx_plugin_idc_core_commission_entry_history",
		"idx_plugin_idc_core_commission_settlement_list",
		"idx_plugin_idc_core_commission_payout_list",
		"idx_plugin_idc_core_commission_recovery_due",
	} {
		count, queryErr := database.GetValue(ctx, `
SELECT COUNT(*) FROM pg_indexes WHERE schemaname = ? AND indexname = ?`, schema, indexName)
		if queryErr != nil || count.Int() != 1 {
			t.Fatalf("required index %s count=%d err=%v", indexName, count.Int(), queryErr)
		}
	}
	accrualConstraints, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM pg_constraint
WHERE conrelid = 'plugin_idc_core_commission_accrual'::regclass
  AND conname IN (
    'uk_plugin_idc_core_commission_currency',
    'fk_plugin_idc_core_commission_agent',
    'fk_plugin_idc_core_commission_affiliate',
    'fk_plugin_idc_core_commission_sales',
    'fk_plugin_idc_core_commission_rule',
    'fk_plugin_idc_core_commission_tier',
    'ck_plugin_idc_core_commission_source'
  )`)
	if err != nil || accrualConstraints.Int() != 7 {
		t.Fatalf("commission accrual extension constraints=%d want=7 err=%v", accrualConstraints.Int(), err)
	}
	maturityIndexes, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM pg_indexes
WHERE schemaname = ?
  AND tablename = 'plugin_idc_core_sales_commission_maturity'
  AND indexname = 'uk_plugin_idc_core_commission_maturity_terminal'
  AND indexdef LIKE 'CREATE UNIQUE INDEX%'
  AND indexdef LIKE '%status_code%'
  AND indexdef LIKE '%matured%'
  AND indexdef LIKE '%reversed%'`, schema)
	if err != nil || maturityIndexes.Int() != 1 {
		t.Fatalf("terminal-only commission maturity index=%d want=1 err=%v", maturityIndexes.Int(), err)
	}
	appendOnlyTriggers, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM pg_trigger trigger
JOIN pg_class relation ON relation.oid=trigger.tgrelid
JOIN pg_namespace namespace ON namespace.oid=relation.relnamespace
WHERE NOT trigger.tgisinternal AND trigger.tgname LIKE 'trg_idc_sales_%_append_only'
  AND namespace.nspname=?`, schema)
	if err != nil || appendOnlyTriggers.Int() != 9 {
		t.Fatalf("append-only evidence triggers=%d want=9 err=%v", appendOnlyTriggers.Int(), err)
	}
	dictionaries, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM sys_dict_type WHERE tenant_id=0 AND type LIKE 'idc_sales_%'`)
	if err != nil || dictionaries.Int() < 7 {
		t.Fatalf("sales dictionary types=%d want>=7 err=%v", dictionaries.Int(), err)
	}
	applicationStatuses, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM sys_dict_data
WHERE tenant_id=0 AND dict_type='idc_sales_agent_application_status'
  AND value IN ('submitted','approved','rejected','cancelled')`)
	if err != nil || applicationStatuses.Int() != 4 {
		t.Fatalf("agent application dictionary statuses=%d want=4 err=%v", applicationStatuses.Int(), err)
	}
	obsoletePending, err := database.GetValue(ctx, `
SELECT COUNT(*) FROM sys_dict_data
WHERE tenant_id=0 AND dict_type='idc_sales_agent_application_status' AND value='pending'`)
	if err != nil || obsoletePending.Int() != 0 {
		t.Fatalf("obsolete agent application pending dictionary rows=%d want=0 err=%v", obsoletePending.Int(), err)
	}

	for _, columnName := range []string{
		"agent_qualification_no", "affiliate_attribution_no", "sales_assignment_no",
		"commission_rule_no", "commission_tier_no", "operation_type_code",
	} {
		count, queryErr := database.GetValue(ctx, `
SELECT COUNT(*) FROM information_schema.columns
WHERE table_schema = ? AND table_name = 'plugin_idc_core_commission_accrual'
  AND column_name = ? AND is_nullable = 'YES'`, schema, columnName)
		if queryErr != nil || count.Int() != 1 {
			t.Fatalf("nullable commission source column %s count=%d err=%v", columnName, count.Int(), queryErr)
		}
	}
}

func assertCommissionConstraints(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_customer_subject
    (customer_no, customer_type_code, status_code, display_name)
	VALUES ('10001','individual','active','Agent'),
	       ('10002','individual','active','Referral'),
	       ('10003','individual','active','Customer'),
	       ('10004','individual','active','Rejected then eligible');
INSERT INTO plugin_idc_core_sales_agent_level
    (level_no, level_code, name_zh_cn, name_en_us)
VALUES ('level_basic','basic','基础代理','Basic agent');
INSERT INTO plugin_idc_core_sales_agent_application
    (application_no, customer_no, requested_level_no, application_digest)
VALUES ('application_001','10001','level_basic',repeat('a',64));
INSERT INTO plugin_idc_core_sales_agent_qualification
    (qualification_no, customer_no, level_no, source_application_no, effective_at)
VALUES ('qualification_001','10001','level_basic','application_001',CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_sales_agent_qualification_event
    (event_no, qualification_no, event_sequence, to_status_code, to_level_no,
     reason_code, actor_reference, evidence_digest)
VALUES ('qualification_event_001','qualification_001',1,'active','level_basic',
        'application_approved','staff:1',repeat('b',64));
INSERT INTO plugin_idc_core_sales_commission_rule
    (rule_no, beneficiary_type_code, operation_type_code, currency_code,
     calculation_type_code, rate_basis_points, version, rule_digest)
VALUES ('commission_rule_global_001','affiliate','new_purchase','CNY',
        'rate',500,1,repeat('0',64));
INSERT INTO plugin_idc_core_sales_affiliate_link
    (link_no, owner_customer_no, code_digest, attribution_policy_code, attribution_window_seconds)
VALUES ('affiliate_link_001','10002',repeat('c',64),'first_touch',86400);
INSERT INTO plugin_idc_core_sales_affiliate_touch
    (touch_no, link_no, visitor_digest, evidence_digest, expires_at)
VALUES ('affiliate_touch_001','affiliate_link_001',repeat('d',64),repeat('e',64),
        CURRENT_TIMESTAMP + INTERVAL '1 day');
INSERT INTO plugin_idc_core_sales_affiliate_attribution
    (attribution_no, link_no, touch_no, referrer_customer_no, referred_customer_no,
     policy_code, evidence_digest, expires_at)
VALUES ('attribution_001','affiliate_link_001','affiliate_touch_001','10002','10003',
        'first_touch',repeat('f',64),CURRENT_TIMESTAMP + INTERVAL '1 day');
INSERT INTO plugin_idc_core_sales_group
    (group_no, group_code, name_zh_cn, name_en_us)
VALUES ('sales_group_001','direct','直销组','Direct sales');
INSERT INTO plugin_idc_core_sales_group_member
    (membership_no, group_no, staff_user_id)
VALUES ('membership_001','sales_group_001',101),('membership_002','sales_group_001',102);
INSERT INTO plugin_idc_core_sales_customer_assignment
    (assignment_no, customer_no, membership_no, source_code)
VALUES ('assignment_001','10003','membership_001','manual');
INSERT INTO plugin_idc_core_sales_customer_collaborator
    (collaboration_no, assignment_no, membership_no)
VALUES ('collaboration_001','assignment_001','membership_002');
	INSERT INTO plugin_idc_core_sales_commission_account
	    (account_no, beneficiary_type_code, beneficiary_reference, currency_code)
	VALUES ('commission_account_cny','affiliate','10002','CNY'),
	       ('commission_account_usd','affiliate','10002','USD');
INSERT INTO plugin_idc_core_sales_commission_account_entry
    (entry_no, account_no, currency_code, entry_sequence, entry_type_code,
     source_type_code, source_reference, source_digest,
     delta_credited_minor, delta_pending_minor, delta_available_minor,
     delta_frozen_minor, delta_paid_minor, delta_reversed_minor,
     after_credited_minor, after_pending_minor, after_available_minor,
     after_frozen_minor, after_paid_minor, after_reversed_minor)
	VALUES ('commission_entry_001','commission_account_cny','CNY',1,'accrual',
	        'commission','commission_001',repeat('1',64),1000,0,1000,0,0,0,
	        1000,0,1000,0,0,0);
	SET session_replication_role = replica;
	INSERT INTO plugin_idc_core_commission_accrual
	    (commission_no,order_no,invoice_no,invoice_item_no,beneficiary_type_code,
	     beneficiary_reference,currency_code,rule_version,basis_amount_minor,
	     invoice_item_amount_minor,original_amount_minor,pending_amount_minor,matures_at)
	VALUES ('commission_001','fixture_order','fixture_invoice','fixture_item','affiliate',
	        '10002','CNY',1,1000,1000,1000,1000,CURRENT_TIMESTAMP + INTERVAL '1 day'),
	       ('commission_other','fixture_order','fixture_invoice','fixture_item','affiliate',
	        '10003','CNY',1,500,500,500,500,CURRENT_TIMESTAMP + INTERVAL '1 day');
	SET session_replication_role = origin;
	INSERT INTO plugin_idc_core_sales_commission_maturity
	    (maturity_no,commission_no,attempt_no,account_no,currency_code,
	     beneficiary_type_code,beneficiary_reference,amount_minor,status_code,reason_code,evidence_digest)
	VALUES ('maturity_001','commission_001',1,'commission_account_cny','CNY',
	        'affiliate','10002',100,'blocked','waiting_period',repeat('1',64));
INSERT INTO plugin_idc_core_sales_commission_payee_profile
    (profile_no, beneficiary_type_code, beneficiary_reference, payee_type_code,
     protected_object_reference, account_digest, status_code, verified_at)
VALUES ('payee_001','affiliate','10002','bank_account','secret://commission/payee/001',
        repeat('2',64),'verified',CURRENT_TIMESTAMP);
	INSERT INTO plugin_idc_core_sales_commission_settlement
	    (settlement_no, account_no, currency_code, payee_profile_no,
	     beneficiary_type_code, beneficiary_reference, requested_amount_minor,
	     frozen_amount_minor, minimum_amount_minor, status_code, request_digest)
	VALUES ('settlement_001','commission_account_cny','CNY','payee_001',
	        'affiliate','10002',500,500,100,'approved',repeat('3',64));
	INSERT INTO plugin_idc_core_sales_commission_settlement_item
	    (item_no,settlement_no,commission_no,currency_code,beneficiary_type_code,
	     beneficiary_reference,amount_minor)
	VALUES ('settlement_item_001','settlement_001','commission_001','CNY','affiliate','10002',500);
INSERT INTO plugin_idc_core_sales_commission_settlement_review
    (review_no, settlement_no, decision_code, reason_code, reviewer_user_id, evidence_digest)
VALUES ('review_001','settlement_001','approved','verified',201,repeat('4',64));
INSERT INTO plugin_idc_core_sales_commission_payout_batch
    (batch_no, currency_code, item_count, total_amount_minor, created_by)
VALUES ('payout_batch_001','CNY',1,500,201);
INSERT INTO plugin_idc_core_sales_commission_payout_item
    (item_no, batch_no, settlement_no, currency_code, amount_minor)
VALUES ('payout_item_001','payout_batch_001','settlement_001','CNY',500);
INSERT INTO plugin_idc_core_sales_commission_payout_evidence
    (evidence_no, payout_item_no, evidence_type_code, protected_object_reference, evidence_digest)
VALUES ('payout_evidence_001','payout_item_001','manual_proof','object://private/payout/001',repeat('5',64));
INSERT INTO plugin_idc_core_sales_commission_payout_recovery
    (recovery_no, payout_item_no, attempt_no, action_code, result_code,
     reason_code, evidence_digest, next_attempt_at)
VALUES ('payout_recovery_001','payout_item_001',1,'query','retry_ready',
        'provider_pending',repeat('6',64),CURRENT_TIMESTAMP + INTERVAL '5 minutes');`); err != nil {
		t.Fatalf("seed valid commission foundation: %v", err)
	}

	assertCommissionExecFails(t, ctx, database, "foreign key", `
INSERT INTO plugin_idc_core_sales_agent_application
    (application_no, customer_no, requested_level_no, application_digest)
VALUES ('application_bad_fk','missing_customer','level_basic',repeat('7',64))`)
	assertCommissionExecFails(t, ctx, database, "cancelled application review fields", `
INSERT INTO plugin_idc_core_sales_agent_application
    (application_no, customer_no, requested_level_no, application_digest,
     status_code, reviewed_by)
VALUES ('application_bad_cancel','10001','level_basic',repeat('7',64),'cancelled',201)`)
	assertCommissionExecFails(t, ctx, database, "global rule uniqueness", `
INSERT INTO plugin_idc_core_sales_commission_rule
    (rule_no, beneficiary_type_code, operation_type_code, currency_code,
     calculation_type_code, rate_basis_points, version, rule_digest)
VALUES ('commission_rule_global_duplicate','affiliate','new_purchase','CNY',
        'rate',600,1,repeat('8',64))`)
	assertCommissionExecFails(t, ctx, database, "unique owner currency", `
INSERT INTO plugin_idc_core_sales_commission_account
    (account_no, beneficiary_type_code, beneficiary_reference, currency_code)
VALUES ('commission_account_duplicate','affiliate','10002','CNY')`)
	assertCommissionExecFails(t, ctx, database, "account conservation", `
INSERT INTO plugin_idc_core_sales_commission_account
    (account_no, beneficiary_type_code, beneficiary_reference, currency_code,
     credited_amount_minor, available_amount_minor)
VALUES ('commission_account_unbalanced','sales','staff:999','CNY',100,99)`)
	assertCommissionExecFails(t, ctx, database, "entry conservation", `
INSERT INTO plugin_idc_core_sales_commission_account_entry
    (entry_no, account_no, currency_code, entry_sequence, entry_type_code,
     source_type_code, source_reference, source_digest,
     delta_credited_minor, delta_pending_minor, delta_available_minor,
     delta_frozen_minor, delta_paid_minor, delta_reversed_minor,
     after_credited_minor, after_pending_minor, after_available_minor,
     after_frozen_minor, after_paid_minor, after_reversed_minor)
VALUES ('commission_entry_unbalanced','commission_account_cny','CNY',2,'maturity',
        'commission','commission_002',repeat('8',64),0,-100,99,0,0,0,
	        1000,0,1000,0,0,0)`)
	assertCommissionExecFails(t, ctx, database, "entry sequence gap", `
	INSERT INTO plugin_idc_core_sales_commission_account_entry
	    (entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,
	     source_reference,source_digest,delta_credited_minor,delta_pending_minor,
	     delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,
	     after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,
	     after_paid_minor,after_reversed_minor)
	VALUES ('commission_entry_gap','commission_account_cny','CNY',3,'maturity','commission',
	        'commission_gap',repeat('8',64),0,0,0,0,0,0,1000,0,1000,0,0,0)`)
	assertCommissionExecFails(t, ctx, database, "entry broken after projection", `
	INSERT INTO plugin_idc_core_sales_commission_account_entry
	    (entry_no,account_no,currency_code,entry_sequence,entry_type_code,source_type_code,
	     source_reference,source_digest,delta_credited_minor,delta_pending_minor,
	     delta_available_minor,delta_frozen_minor,delta_paid_minor,delta_reversed_minor,
	     after_credited_minor,after_pending_minor,after_available_minor,after_frozen_minor,
	     after_paid_minor,after_reversed_minor)
	VALUES ('commission_entry_broken','commission_account_cny','CNY',2,'maturity','commission',
	        'commission_broken',repeat('8',64),0,0,0,0,0,0,1001,0,1001,0,0,0)`)
	assertCommissionExecFails(t, ctx, database, "direct account projection tamper", `
	UPDATE plugin_idc_core_sales_commission_account
	SET credited_amount_minor=1001,available_amount_minor=1001
	WHERE account_no='commission_account_cny'`)
	assertCommissionExecFails(t, ctx, database, "maturity beneficiary mismatch", `
	INSERT INTO plugin_idc_core_sales_commission_maturity
	    (maturity_no,commission_no,attempt_no,account_no,currency_code,
	     beneficiary_type_code,beneficiary_reference,amount_minor,status_code,reason_code,evidence_digest)
	VALUES ('maturity_bad_owner','commission_other',1,'commission_account_cny','CNY',
	        'affiliate','10003',100,'blocked','owner_mismatch',repeat('8',64))`)
	assertCommissionExecFails(t, ctx, database, "settlement payee beneficiary mismatch", `
	INSERT INTO plugin_idc_core_sales_commission_payee_profile
	    (profile_no,beneficiary_type_code,beneficiary_reference,payee_type_code,
	     protected_object_reference,account_digest,status_code,verified_at)
	VALUES ('payee_other','affiliate','10003','manual','secret://commission/payee/other',repeat('8',64),'verified',CURRENT_TIMESTAMP);
	INSERT INTO plugin_idc_core_sales_commission_settlement
	    (settlement_no,account_no,currency_code,payee_profile_no,beneficiary_type_code,
	     beneficiary_reference,requested_amount_minor,frozen_amount_minor,minimum_amount_minor,request_digest)
	VALUES ('settlement_bad_payee','commission_account_cny','CNY','payee_other','affiliate','10002',100,100,1,repeat('8',64))`)
	assertCommissionExecFails(t, ctx, database, "settlement item beneficiary mismatch", `
	INSERT INTO plugin_idc_core_sales_commission_settlement_item
	    (item_no,settlement_no,commission_no,currency_code,beneficiary_type_code,
	     beneficiary_reference,amount_minor)
	VALUES ('settlement_item_bad_owner','settlement_001','commission_other','CNY','affiliate','10002',500)`)
	assertCommissionExecFails(t, ctx, database, "settlement item total", `
	UPDATE plugin_idc_core_sales_commission_settlement
	SET frozen_amount_minor=400,returned_amount_minor=100 WHERE settlement_no='settlement_001'`)
	assertCommissionExecFails(t, ctx, database, "payout locks settlement amount", `
	INSERT INTO plugin_idc_core_sales_commission_payout_batch
	    (batch_no,currency_code,item_count,total_amount_minor,created_by)
	VALUES ('payout_batch_bad_amount','CNY',1,499,201);
	INSERT INTO plugin_idc_core_sales_commission_payout_item
	    (item_no,batch_no,settlement_no,currency_code,amount_minor)
	VALUES ('payout_item_bad_amount','payout_batch_bad_amount','settlement_001','CNY',499)`)
	assertCommissionExecFails(t, ctx, database, "terminal payout batch summary", `
	UPDATE plugin_idc_core_sales_commission_payout_batch
	SET status_code='succeeded',completed_at=CURRENT_TIMESTAMP
	WHERE batch_no='payout_batch_001'`)
	assertAffiliateBindingAndRejectedHistory(t, ctx, database)
	assertAppendOnlyEvidence(t, ctx, database)
	if _, err := database.Exec(ctx, `
UPDATE plugin_idc_core_sales_commission_payout_item
SET status_code='succeeded',processed_at=CURRENT_TIMESTAMP WHERE item_no='payout_item_001';
UPDATE plugin_idc_core_sales_commission_payout_batch
SET succeeded_count=1,succeeded_amount_minor=500,status_code='succeeded',completed_at=CURRENT_TIMESTAMP
WHERE batch_no='payout_batch_001';
UPDATE plugin_idc_core_sales_commission_settlement
SET frozen_amount_minor=0,paid_amount_minor=500,status_code='paid',completed_at=CURRENT_TIMESTAMP
WHERE settlement_no='settlement_001'`); err != nil {
		t.Fatalf("valid payout and settlement terminal convergence: %v", err)
	}
	projection, queryErr := database.GetValue(ctx, `
SELECT available_amount_minor FROM plugin_idc_core_sales_commission_account
WHERE account_no='commission_account_cny'`)
	if queryErr != nil || projection.Int64() != 1000 {
		t.Fatalf("account projection was not advanced by its entry: value=%d err=%v", projection.Int64(), queryErr)
	}
	assertCommissionExecFails(t, ctx, database, "cross-currency settlement", `
INSERT INTO plugin_idc_core_sales_commission_settlement
    (settlement_no, account_no, currency_code, payee_profile_no,
     requested_amount_minor, frozen_amount_minor, minimum_amount_minor, request_digest)
VALUES ('settlement_bad_currency','commission_account_cny','USD','payee_001',100,100,10,repeat('9',64))`)
	assertCommissionExecFails(t, ctx, database, "cross-currency payout", `
INSERT INTO plugin_idc_core_sales_commission_payout_item
    (item_no, batch_no, settlement_no, currency_code, amount_minor)
VALUES ('payout_item_bad_currency','payout_batch_001','settlement_001','USD',500)`)
	assertCommissionExecFails(t, ctx, database, "terminal status timestamp", `
INSERT INTO plugin_idc_core_sales_commission_settlement
    (settlement_no, account_no, currency_code, payee_profile_no,
     requested_amount_minor, frozen_amount_minor, minimum_amount_minor,
     paid_amount_minor, status_code, request_digest)
VALUES ('settlement_bad_status','commission_account_cny','CNY','payee_001',100,0,10,100,
        'paid',repeat('a',64))`)
}

func assertAffiliateBindingAndRejectedHistory(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	if _, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_sales_affiliate_link
    (link_no,owner_customer_no,code_digest,attribution_policy_code,attribution_window_seconds)
VALUES ('affiliate_link_rejected','10004',repeat('7',64),'first_touch',86400);
INSERT INTO plugin_idc_core_sales_affiliate_touch
    (touch_no,link_no,visitor_digest,evidence_digest,expires_at)
VALUES ('affiliate_touch_rejected','affiliate_link_rejected',repeat('7',64),repeat('8',64),CURRENT_TIMESTAMP+INTERVAL '1 day');
INSERT INTO plugin_idc_core_sales_affiliate_attribution
    (attribution_no,link_no,touch_no,referrer_customer_no,referred_customer_no,
     policy_code,status_code,risk_reason_code,evidence_digest,expires_at)
VALUES ('attribution_rejected','affiliate_link_rejected','affiliate_touch_rejected','10004','10004',
        'first_touch','rejected_self_referral','same_identity',repeat('9',64),CURRENT_TIMESTAMP+INTERVAL '1 day');
INSERT INTO plugin_idc_core_sales_affiliate_attribution
    (attribution_no,link_no,touch_no,referrer_customer_no,referred_customer_no,
     policy_code,status_code,evidence_digest,expires_at)
VALUES ('attribution_after_rejection','affiliate_link_001','affiliate_touch_001','10002','10004',
        'first_touch','eligible',repeat('a',64),CURRENT_TIMESTAMP+INTERVAL '1 day')`); err != nil {
		t.Fatalf("rejected attribution must not occupy the eligible attribution slot: %v", err)
	}
	assertCommissionExecFails(t, ctx, database, "second eligible attribution", `
INSERT INTO plugin_idc_core_sales_affiliate_attribution
    (attribution_no,link_no,touch_no,referrer_customer_no,referred_customer_no,
     policy_code,status_code,evidence_digest,expires_at)
VALUES ('attribution_second_eligible','affiliate_link_001','affiliate_touch_001','10002','10004',
        'first_touch','eligible',repeat('b',64),CURRENT_TIMESTAMP+INTERVAL '1 day')`)
	assertCommissionExecFails(t, ctx, database, "attribution touch link mismatch", `
INSERT INTO plugin_idc_core_sales_affiliate_attribution
    (attribution_no,link_no,touch_no,referrer_customer_no,referred_customer_no,
     policy_code,status_code,risk_reason_code,evidence_digest,expires_at)
VALUES ('attribution_bad_touch','affiliate_link_rejected','affiliate_touch_001','10004','10003',
        'first_touch','rejected_risk','bad_touch',repeat('c',64),CURRENT_TIMESTAMP+INTERVAL '1 day')`)
	assertCommissionExecFails(t, ctx, database, "attribution link owner mismatch", `
INSERT INTO plugin_idc_core_sales_affiliate_attribution
    (attribution_no,link_no,touch_no,referrer_customer_no,referred_customer_no,
     policy_code,status_code,risk_reason_code,evidence_digest,expires_at)
VALUES ('attribution_bad_owner','affiliate_link_001','affiliate_touch_001','10004','10003',
        'first_touch','rejected_risk','bad_owner',repeat('d',64),CURRENT_TIMESTAMP+INTERVAL '1 day')`)
}

func assertAppendOnlyEvidence(t *testing.T, ctx context.Context, database gdb.DB) {
	t.Helper()
	rows := []struct{ table, key, value string }{
		{"plugin_idc_core_sales_agent_qualification_event", "event_no", "qualification_event_001"},
		{"plugin_idc_core_sales_affiliate_touch", "touch_no", "affiliate_touch_001"},
		{"plugin_idc_core_sales_affiliate_attribution", "attribution_no", "attribution_001"},
		{"plugin_idc_core_sales_commission_account_entry", "entry_no", "commission_entry_001"},
		{"plugin_idc_core_sales_commission_maturity", "maturity_no", "maturity_001"},
		{"plugin_idc_core_sales_commission_settlement_item", "item_no", "settlement_item_001"},
		{"plugin_idc_core_sales_commission_settlement_review", "review_no", "review_001"},
		{"plugin_idc_core_sales_commission_payout_evidence", "evidence_no", "payout_evidence_001"},
		{"plugin_idc_core_sales_commission_payout_recovery", "recovery_no", "payout_recovery_001"},
	}
	for _, row := range rows {
		assertCommissionExecFails(t, ctx, database, row.table+" update", "UPDATE "+row.table+" SET tenant_id=tenant_id WHERE "+row.key+"='"+row.value+"'")
		assertCommissionExecFails(t, ctx, database, row.table+" delete", "DELETE FROM "+row.table+" WHERE "+row.key+"='"+row.value+"'")
	}
}

func assertCommissionExecFails(
	t *testing.T,
	ctx context.Context,
	database gdb.DB,
	name string,
	statement string,
) {
	t.Helper()
	if _, err := database.Exec(ctx, statement); err == nil {
		t.Fatalf("%s violation was accepted", name)
	}
}

func commissionMigrationPaths(t *testing.T) []string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve commission migration path")
	}
	root := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql")
	paths, err := filepath.Glob(filepath.Join(root, "[0-9][0-9][0-9]-*.sql"))
	if err != nil {
		t.Fatalf("list commission migrations: %v", err)
	}
	sort.Strings(paths)
	foundCommission := false
	for _, path := range paths {
		if filepath.Base(path) == "061-commission-refund-reversal.sql" {
			foundCommission = true
		}
	}
	if !foundCommission {
		t.Fatal("commission migration is missing")
	}
	return paths
}

func newCommissionTestSchema(t *testing.T) string {
	t.Helper()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("create commission schema suffix: %v", err)
	}
	return "linapro_commission_test_" + hex.EncodeToString(suffix[:])
}

func quoteCommissionIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func appendCommissionDSNParameter(dsn string, parameter string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + parameter
}

const commissionHostFixtureSQL = `
CREATE TABLE sys_dict_type (
 id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 name VARCHAR(128) NOT NULL DEFAULT '', type VARCHAR(128) NOT NULL DEFAULT '',
 status SMALLINT NOT NULL DEFAULT 1, is_builtin SMALLINT NOT NULL DEFAULT 0,
 allow_tenant_override BOOL NOT NULL DEFAULT FALSE, remark VARCHAR(512) NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
 CONSTRAINT uk_commission_dict_type UNIQUE (tenant_id,type));
CREATE TABLE sys_dict_data (
 id INT GENERATED ALWAYS AS IDENTITY PRIMARY KEY, tenant_id INT NOT NULL DEFAULT 0,
 dict_type VARCHAR(128) NOT NULL DEFAULT '', label VARCHAR(128) NOT NULL DEFAULT '',
 value VARCHAR(128) NOT NULL DEFAULT '', sort INT NOT NULL DEFAULT 0,
 tag_style VARCHAR(128) NOT NULL DEFAULT '', status SMALLINT NOT NULL DEFAULT 1,
 is_builtin SMALLINT NOT NULL DEFAULT 0, created_at TIMESTAMPTZ, updated_at TIMESTAMPTZ,
 CONSTRAINT uk_commission_dict_data UNIQUE (tenant_id,dict_type,value));`
