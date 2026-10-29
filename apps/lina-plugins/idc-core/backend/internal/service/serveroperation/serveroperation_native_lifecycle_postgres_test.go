// These tests execute real accessory DDL in an isolated PostgreSQL schema.
// Parent entitlement, operation and billing boundary tables are explicit local
// fixtures; no provider, shared server, customer document or payment is changed.

package serveroperation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/dialect"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// nativeLifecycleDB applies the real 058/060/151 constraints, including an old
// quote before upgrade. Only unrelated aggregate roots are bounded test doubles.
func nativeLifecycleDB(t *testing.T) gdb.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(transferPostgresDSNEnv))
	if dsn == "" {
		t.Skipf("set %s", transferPostgresDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	schema := transferTestSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + schema})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Close(ctx); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`); err != nil {
			t.Error(err)
		}
		if err := admin.Close(ctx); err != nil {
			t.Error(err)
		}
	})
	for _, statement := range dialect.SplitSQLStatements(nativeLifecycleBoundarySQL) {
		if _, err = database.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	nativeLifecycleMigration(t, database, "058-server-accessory-pricing.sql")
	if _, err = database.Exec(ctx, `ALTER TABLE plugin_idc_core_server_accessory_quote ADD COLUMN expected_service_version BIGINT NOT NULL DEFAULT 1`); err != nil {
		t.Fatal(err)
	}
	nativeLifecycleMigration(t, database, "060-server-accessory-entitlement.sql")
	if err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_price_policy(policy_no,accessory_kind,currency_code,current_version) VALUES('policy','snapshot','CNY',1);
INSERT INTO plugin_idc_core_server_accessory_price_policy_version(policy_no,policy_version,accessory_kind,currency_code,free_count_per_server,unit_amount_minor,period_unit_code,period_count,content_digest,published_by) VALUES('policy',1,'snapshot','CNY',1,100,'month',1,repeat('a',64),1)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_quote
(quote_no,customer_no,service_no,action_code,accessory_kind,currency_code,requested_quantity,existing_object_count,reserved_create_count,free_count_per_server,charged_quantity,unit_amount_minor,total_amount_minor,period_unit_code,period_count,policy_no,policy_version,idempotency_digest,request_digest,content_digest,expires_at,baseline_snapshot_no,baseline_observed_at)
VALUES('old-quote','customer','legacy','snapshot_create','snapshot','CNY',1,0,0,1,0,100,0,'month',1,'policy',1,repeat('a',64),repeat('a',64),repeat('a',64),CURRENT_TIMESTAMP+INTERVAL '1 hour','baseline',CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation(operation_no,subject_type_code,subject_no,customer_no,currency_code,operation_type_code,status_code) VALUES('old-operation','service','legacy','customer','CNY','addon','succeeded');
INSERT INTO plugin_idc_core_server_accessory_entitlement(entitlement_no,customer_no,service_no,create_operation_no,object_no,accessory_kind,billing_mode_code,status_code,currency_code,period_unit_code,period_count,baseline_object_nos,baseline_snapshot_no,baseline_observed_at)
VALUES('old-accessory','customer','legacy','old-operation','old-object','snapshot','free','active','CNY','month',1,'[]','baseline',CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_billing_invoice(invoice_no,status_code) VALUES('old-invoice','pending_payment');
INSERT INTO plugin_idc_core_server_accessory_renewal_work_item(work_item_no,entitlement_no,customer_no,service_no,invoice_no,required_permission,status_code,reason_code,created_at,updated_at)
VALUES('old-work','old-accessory','customer','legacy','old-invoice','idc:admin:resource:accessory-renewal:handle','open','balance_payment_failed',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	nativeLifecycleMigration(t, database, "151-native-server-accessory-lifecycle.sql")
	nativeLifecycleMigration(t, database, "151-native-server-accessory-lifecycle.sql")
	return database
}

// nativeLifecycleMigration uses the production SQL splitter and commits the same
// individual statements as installation, so DO backfill atomicity is exercised.
func nativeLifecycleMigration(t *testing.T, database gdb.DB, name string) {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	content, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", name))
	if err != nil {
		t.Fatal(err)
	}
	for index, statement := range dialect.SplitSQLStatements(string(content)) {
		if _, err = database.Exec(context.Background(), statement); err != nil {
			t.Fatalf("migration %s statement %d: %+v", name, index+1, err)
		}
	}
}

// nativeLifecycleIntent supplies one settled or free source-owned create intent.
func nativeLifecycleIntent(t *testing.T, database gdb.DB, operation, parent, mode, status string) {
	t.Helper()
	_, err := database.Exec(context.Background(), `INSERT INTO plugin_idc_core_business_operation
(operation_no,subject_type_code,subject_no,customer_no,currency_code,operation_type_code,status_code,financial_settlement_required,financially_settled_at,settlement_no)
VALUES(?,'entitlement',?,'customer','CNY','addon',?,TRUE,CURRENT_TIMESTAMP,'settled')`, operation, parent, status)
	if err != nil {
		t.Fatal(err)
	}
	_, err = database.Exec(context.Background(), `INSERT INTO plugin_idc_core_server_accessory_entitlement
(entitlement_no,customer_no,subject_type_code,subject_no,create_operation_no,accessory_kind,billing_mode_code,status_code,currency_code,period_unit_code,period_count,baseline_object_nos,baseline_snapshot_no,baseline_observed_at)
VALUES(?,'customer','entitlement',?,?,'snapshot',?,'pending_binding','CNY','month',1,'["baseline-object"]','baseline',CURRENT_TIMESTAMP-INTERVAL '1 minute')`,
		"accessory-"+operation, parent, operation, mode)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNativeLifecycleMigrationAndBindingPostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	old, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_quote WHERE quote_no='old-quote' AND subject_type_code='service' AND subject_no=service_no AND expected_subject_version=expected_service_version`)
	if err != nil || old.Int() != 1 {
		t.Fatal("historical quote changed", err)
	}
	old, err = database.GetValue(ctx, `SELECT
(SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement WHERE entitlement_no='old-accessory' AND subject_type_code='service' AND subject_no=service_no)
+(SELECT count(*) FROM plugin_idc_core_server_accessory_renewal_work_item WHERE work_item_no='old-work' AND subject_type_code='service' AND subject_no=service_no)`)
	if err != nil || old.Int() != 2 {
		t.Fatal("historical accessory or work item lost its legacy subject", err)
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_quote SET native_target_snapshot='{"forged":true}' WHERE quote_no='old-quote'`); err == nil {
		t.Fatal("immutable quote snapshot changed")
	}
	for _, check := range []struct {
		name, customer, parent, snapshot string
		version                          any
		valid                            bool
	}{
		{"native", "customer", "native-a", `{"localResourceNo":"local-server"}`, 7, true},
		{"foreign", "other", "native-a", `{"localResourceNo":"local-server"}`, 7, false},
		{"missing", "customer", "missing", `{"localResourceNo":"local-server"}`, 7, false},
		{"empty", "customer", "native-a", `{}`, 7, false},
		{"array", "customer", "native-a", `[]`, 7, false},
		{"null-version", "customer", "native-a", `{"localResourceNo":"local-server"}`, nil, false},
	} {
		_, insertErr := database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_quote
(quote_no,customer_no,service_no,subject_type_code,subject_no,expected_subject_version,native_target_snapshot,
action_code,accessory_kind,currency_code,requested_quantity,existing_object_count,reserved_create_count,free_count_per_server,charged_quantity,unit_amount_minor,total_amount_minor,period_unit_code,period_count,policy_no,policy_version,idempotency_digest,request_digest,content_digest,expires_at,baseline_snapshot_no,baseline_observed_at)
SELECT ?,?,NULL,'entitlement',?,?,?::JSONB,action_code,accessory_kind,currency_code,requested_quantity,
existing_object_count,reserved_create_count,free_count_per_server,charged_quantity,unit_amount_minor,total_amount_minor,
period_unit_code,period_count,policy_no,policy_version,md5(?)||md5(?),request_digest,content_digest,expires_at,baseline_snapshot_no,baseline_observed_at
FROM plugin_idc_core_server_accessory_quote WHERE quote_no='old-quote'`, check.name, check.customer, check.parent, check.version, check.snapshot, check.name, check.name)
		if (insertErr == nil) != check.valid {
			t.Fatalf("native quote %s valid=%t error=%v", check.name, check.valid, insertErr)
		}
	}
	nativeLifecycleMigration(t, database, "151-native-server-accessory-lifecycle.sql")
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_quote SET expected_subject_version=9 WHERE quote_no='native'`); err == nil {
		t.Fatal("native quote version could change after publication")
	}
	nativeLifecycleIntent(t, database, "create", "native-a", "paid", "succeeded")
	input := servercommercialcap.Completion{OperationNo: "create", EntitlementNo: "native-a", ObjectNo: "baseline-object", EvidenceDigest: strings.Repeat("b", 64), Status: servercommercialcap.CompletionSucceeded, ObservedAt: time.Now().UTC()}
	if err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteCreatedAccessory(ctx, database, input) }); err == nil {
		t.Fatal("pre-existing object bound as a newly purchased object")
	}
	input.ObjectNo = "created-object"
	var group sync.WaitGroup
	results := make(chan error, 6)
	for range 6 {
		group.Add(1)
		go func() {
			defer group.Done()
			results <- database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteCreatedAccessory(ctx, database, input) })
		}()
	}
	group.Wait()
	close(results)
	for err = range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	count, err := database.GetValue(ctx, `SELECT count(*) FROM plugin_idc_core_server_accessory_entitlement_event WHERE event_type_code='object_bound'`)
	if err != nil || count.Int() != 1 {
		t.Fatal("concurrent receipts duplicated binding", err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement(entitlement_no,customer_no,subject_type_code,subject_no,create_operation_no,accessory_kind,billing_mode_code,status_code,currency_code,period_unit_code,period_count,baseline_object_nos,baseline_snapshot_no,baseline_observed_at) VALUES('forged','other','entitlement','native-a','create','snapshot','free','pending_binding','CNY','month',1,'[]','baseline',CURRENT_TIMESTAMP)`); err == nil {
		t.Fatal("foreign customer reused native parent")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_entitlement SET subject_no='native-b' WHERE entitlement_no='accessory-create'`); err == nil {
		t.Fatal("bound object changed parent")
	}
}

func TestNativeLifecycleExpiryDeletionAndOccupancyPostgreSQL(t *testing.T) {
	database := nativeLifecycleDB(t)
	ctx := context.Background()
	nativeLifecycleIntent(t, database, "free", "native-a", "free", "succeeded")
	input := servercommercialcap.Completion{OperationNo: "free", EntitlementNo: "native-a", ObjectNo: "free-object", EvidenceDigest: strings.Repeat("b", 64), Status: servercommercialcap.CompletionSucceeded, ObservedAt: time.Now().UTC()}
	if err := database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteCreatedAccessory(ctx, database, input) }); err != nil {
		t.Fatal(err)
	}
	target := servercommercialcap.TargetSnapshot{EntitlementNo: "native-a", CustomerNo: "customer", Items: []serveroperationresourcecap.AccessoryItem{{Kind: "snapshot", ObjectNo: "free-object"}}}
	count, reserved, err := nativeAccessoryOccupancy(ctx, database, target, "snapshot")
	if err != nil || count != 1 || reserved != 0 {
		t.Fatalf("directory/bound object duplicated: %d/%d %v", count, reserved, err)
	}
	nativeLifecycleIntent(t, database, "reserved", "native-a", "free", "awaiting_financial_settlement")
	count, reserved, err = nativeAccessoryOccupancy(ctx, database, target, "snapshot")
	if err != nil || count != 1 || reserved != 1 {
		t.Fatalf("pending reservation lost: %d/%d %v", count, reserved, err)
	}
	target.EntitlementNo = "native-b"
	count, reserved, err = nativeAccessoryOccupancy(ctx, database, target, "snapshot")
	if err != nil || count != 1 || reserved != 0 {
		t.Fatal("occupancy leaked across servers", err)
	}
	target.EntitlementNo = "native-a"
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code='succeeded' WHERE operation_no='reserved'`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = nativeAccessoryOccupancy(ctx, database, target, "snapshot"); err == nil {
		t.Fatal("unbound successful create was guessed as another free slot")
	}
	object := servercommercialcap.ObjectRequest{EntitlementNo: "native-a", CustomerNo: "customer", Kind: "snapshot", ObjectNo: "free-object"}
	input.OperationNo = "delete"
	forced := errors.New("completion audit failed")
	err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		if err := nativeCompleteDeletedAccessory(ctx, database, object, input); err != nil {
			return err
		}
		return forced
	})
	if !errors.Is(err, forced) {
		t.Fatal(err)
	}
	status, _ := database.GetValue(ctx, `SELECT status_code FROM plugin_idc_core_server_accessory_entitlement WHERE entitlement_no='accessory-free'`)
	if status.String() != "active" {
		t.Fatal("failed completion released a free slot")
	}
	input.Status = servercommercialcap.CompletionUnknown
	if err = nativeCompleteDeletedAccessory(ctx, database, object, input); err == nil {
		t.Fatal("unknown upstream result released occupancy")
	}
	input.Status = servercommercialcap.CompletionSucceeded
	if err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		return nativeCompleteDeletedAccessory(ctx, database, object, input)
	}); err != nil {
		t.Fatal(err)
	}
	if err = nativeCheckRestore(ctx, database, object); err == nil {
		t.Fatal("deleted object became restorable")
	}
	if _, err = database.Exec(ctx, `UPDATE plugin_idc_core_business_operation SET status_code='cancelled' WHERE operation_no='reserved'`); err != nil {
		t.Fatal(err)
	}
	count, reserved, err = nativeAccessoryOccupancy(ctx, database, target, "snapshot")
	if err != nil || count != 0 || reserved != 0 {
		t.Fatalf("confirmed delete retained a stale directory slot: %d/%d %v", count, reserved, err)
	}
	nativeLifecycleIntent(t, database, "paid", "native-a", "paid", "succeeded")
	input.OperationNo, input.ObjectNo = "paid", "paid-object"
	input.ObservedAt = time.Now().UTC().AddDate(0, -2, 0)
	if err = database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error { return nativeCompleteCreatedAccessory(ctx, database, input) }); err != nil {
		t.Fatal(err)
	}
	object.ObjectNo = "paid-object"
	if err = nativeCheckRestore(ctx, database, object); err == nil {
		t.Fatal("expired paid object restored before renewal worker projected payment_pending")
	}
}

// nativeLifecycleBoundarySQL contains no provider identity and no real funds.
const nativeLifecycleBoundarySQL = `
CREATE TABLE plugin_idc_core_customer_subject(customer_no VARCHAR(40) PRIMARY KEY);
CREATE TABLE plugin_idc_core_service_instance(tenant_id INT DEFAULT 0,service_no VARCHAR(40) PRIMARY KEY,customer_no VARCHAR(40),currency_code CHAR(3),current_renewal_policy_no VARCHAR(40),current_renewal_policy_version BIGINT,UNIQUE(service_no,customer_no,currency_code));
CREATE TABLE plugin_idc_core_service_entitlement(tenant_id INT DEFAULT 0,entitlement_no VARCHAR(40) PRIMARY KEY,customer_no VARCHAR(40),currency_code CHAR(3),entitlement_owner_id TEXT DEFAULT 'idc-resource',current_renewal_policy_no VARCHAR(40),current_renewal_policy_version BIGINT,current_price_snapshot_no VARCHAR(40),order_no VARCHAR(40),order_item_no VARCHAR(40));
CREATE TABLE plugin_idc_core_business_operation(id BIGINT GENERATED ALWAYS AS IDENTITY,tenant_id INT DEFAULT 0,operation_no VARCHAR(40) PRIMARY KEY,subject_type_code TEXT,subject_no VARCHAR(40),customer_no VARCHAR(40),currency_code CHAR(3),operation_type_code TEXT,status_code TEXT,financial_settlement_required BOOLEAN DEFAULT FALSE,financially_settled_at TIMESTAMPTZ,settlement_no VARCHAR(40));
CREATE TABLE plugin_idc_core_billing_invoice(tenant_id INT DEFAULT 0,invoice_no VARCHAR(40) PRIMARY KEY,customer_no VARCHAR(40),source_code TEXT,currency_code CHAR(3),status_code TEXT,item_count INT,subtotal_minor BIGINT,discount_minor BIGINT,net_amount_minor BIGINT,tax_amount_minor BIGINT,total_amount_minor BIGINT,paid_amount_minor BIGINT,refundable_amount_minor BIGINT,refunded_amount_minor BIGINT,source_snapshot_digest TEXT,content_digest TEXT,payment_due_at TIMESTAMPTZ,issued_at TIMESTAMPTZ,settled_at TIMESTAMPTZ,version BIGINT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_billing_invoice_item(tenant_id INT,invoice_item_no VARCHAR(40) PRIMARY KEY,invoice_no VARCHAR(40),customer_no VARCHAR(40),currency_code CHAR(3),line_number INT,item_type_code TEXT,source_reference TEXT,description_zh_cn TEXT,description_en_us TEXT,subtotal_minor BIGINT,discount_minor BIGINT,net_amount_minor BIGINT,tax_amount_minor BIGINT,total_amount_minor BIGINT,refundable_cap_minor BIGINT,content_digest TEXT);
CREATE TABLE plugin_idc_core_service_renewal_policy(tenant_id INT,policy_no VARCHAR(40),policy_version BIGINT,lead_seconds BIGINT);
CREATE TABLE plugin_idc_core_commerce_order_item(tenant_id INT DEFAULT 0,order_no VARCHAR(40),order_item_no VARCHAR(40),tax_treatment_code TEXT DEFAULT 'exempt',tax_rate_basis_points INT,tax_category_code TEXT DEFAULT 'cloud_service',rounding_scope_code TEXT DEFAULT 'line',rounding_mode_code TEXT DEFAULT 'half_up',rounding_quantum_minor BIGINT DEFAULT 1);
CREATE TABLE plugin_idc_core_service_price_snapshot(tenant_id INT DEFAULT 0,price_snapshot_no VARCHAR(40),subject_type_code TEXT,subject_no VARCHAR(40),tax_treatment_code TEXT,tax_rate_basis_points INT,tax_category_code TEXT,rounding_scope_code TEXT,rounding_mode_code TEXT,rounding_quantum_minor BIGINT);
CREATE TABLE plugin_idc_core_server_accessory_kind(accessory_kind TEXT PRIMARY KEY,name_zh_cn TEXT,name_en_us TEXT);
CREATE TABLE plugin_idc_core_customer_contact_role(tenant_id INT,customer_no VARCHAR(40),contact_no VARCHAR(40),role_code TEXT,status_code TEXT);
CREATE TABLE plugin_idc_core_customer_contact(id BIGINT GENERATED ALWAYS AS IDENTITY,tenant_id INT,customer_no VARCHAR(40),contact_no VARCHAR(40),status_code TEXT);
ALTER TABLE plugin_idc_core_billing_invoice ADD COLUMN id BIGINT GENERATED ALWAYS AS IDENTITY,
ADD COLUMN order_no VARCHAR(40),ADD COLUMN waived_amount_minor BIGINT NOT NULL DEFAULT 0,
ADD COLUMN credit_amount_minor BIGINT NOT NULL DEFAULT 0,ADD COLUMN cancelled_at TIMESTAMPTZ;
CREATE TABLE plugin_idc_core_billing_payment_allocation(tenant_id INT,invoice_no VARCHAR(40),direction_code TEXT,amount_minor BIGINT);
CREATE TABLE plugin_idc_core_billing_invoice_waiver_allocation(tenant_id INT,invoice_no VARCHAR(40),amount_minor BIGINT);
CREATE TABLE plugin_idc_core_billing_refund_allocation(tenant_id INT,invoice_no VARCHAR(40),direction_code TEXT,amount_minor BIGINT);
ALTER TABLE plugin_idc_core_business_operation ADD COLUMN service_no VARCHAR(40),ADD COLUMN resource_owner_code TEXT DEFAULT 'idc-resource',ADD COLUMN provider_task_no TEXT,ADD COLUMN execution_started_at TIMESTAMPTZ,ADD COLUMN updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE plugin_idc_core_billing_refund_allocation ADD COLUMN refund_source_reference TEXT,ADD COLUMN original_refund_allocation_no VARCHAR(40);
CREATE TABLE plugin_idc_core_refund_request(tenant_id INT,order_no VARCHAR(40),invoice_no VARCHAR(40),customer_no VARCHAR(40),currency_code CHAR(3),refund_no VARCHAR(40),status_code TEXT);
ALTER TABLE plugin_idc_core_service_entitlement ADD COLUMN status_code TEXT DEFAULT 'active',ADD COLUMN expires_at TIMESTAMPTZ;
CREATE TABLE plugin_idc_core_refund_entitlement_lock(tenant_id INT,entitlement_no VARCHAR(40),released_at TIMESTAMPTZ);
CREATE TABLE native_notification_intent(event_key TEXT PRIMARY KEY,variables JSONB);
INSERT INTO plugin_idc_core_customer_subject VALUES('customer'),('other');
INSERT INTO plugin_idc_core_service_instance(service_no,customer_no,currency_code) VALUES('legacy','customer','CNY');
INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,customer_no,currency_code,order_no,order_item_no) VALUES('native-a','customer','CNY','order','item'),('native-b','customer','CNY','order','item');
INSERT INTO plugin_idc_core_commerce_order_item(order_no,order_item_no) VALUES('order','item');
INSERT INTO plugin_idc_core_server_accessory_kind VALUES('snapshot','快照','Snapshot');
`
