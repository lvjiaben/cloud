package serveroperation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
)

const transferPostgresDSNEnv = "LINAPRO_IDC_CORE_TEST_DSN"

func TestTransferOwnershipRulesPostgreSQL(t *testing.T) {
	t.Parallel()
	dsn := strings.TrimSpace(os.Getenv(transferPostgresDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run server transfer PostgreSQL tests", transferPostgresDSNEnv)
	}
	ctx := context.Background()
	admin, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	schema := transferTestSchema(t)
	if _, err = admin.Exec(ctx, `CREATE SCHEMA "`+schema+`"`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec(ctx, `DROP SCHEMA "`+schema+`" CASCADE`) })
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	database, err := gdb.New(gdb.ConfigNode{Link: dsn + separator + "search_path=" + url.QueryEscape(schema)})
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close(ctx)
	if _, err = database.Exec(ctx, transferRuleFixtureSQL); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement
(tenant_id,service_no,customer_no,billing_mode_code,status_code,current_period_end_at,pending_invoice_no,auto_renew_enabled,updated_at)
VALUES
(0,'srv-1','old','paid','active',?,NULL,TRUE,?),
(0,'srv-1','old','paid','active',?,NULL,TRUE,?),
(0,'srv-1','old','free','active',NULL,NULL,TRUE,?),
(0,'srv-1','old','paid','payment_pending',?,NULL,TRUE,?)`, now.Add(time.Hour), now, now.Add(-time.Hour), now, now, now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if err = lockTransferAccessoryEntitlements(ctx, database, "srv-1"); err != nil {
		t.Fatal(err)
	}
	if err = transferEligibleAccessoryOwnership(ctx, database, "srv-1", "new", now); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		CustomerNo string `orm:"customer_no"`
		AutoRenew  bool   `orm:"auto_renew_enabled"`
	}
	if err = database.GetScan(ctx, &rows, `SELECT customer_no,auto_renew_enabled
FROM plugin_idc_core_server_accessory_entitlement ORDER BY id`); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 4 || rows[0].CustomerNo != "new" || rows[0].AutoRenew ||
		rows[1].CustomerNo != "old" || rows[2].CustomerNo != "old" || rows[3].CustomerNo != "old" {
		t.Fatalf("unexpected transferred accessory ownership: %+v", rows)
	}
	blocked, blockErr := transferBlockState(ctx, database, "srv-1", "order-1")
	if blockErr != nil {
		t.Fatal(blockErr)
	}
	if blocked {
		t.Fatal("clean server transfer was blocked")
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_billing_invoice VALUES
(0,'inv-addon',NULL,'pending_payment'),(0,'inv-renew',NULL,'overdue')`); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_business_operation
VALUES (0,'srv-1','inv-addon','addon')`); err != nil {
		t.Fatal(err)
	}
	blocked, blockErr = transferBlockState(ctx, database, "srv-1", "order-1")
	if blockErr != nil {
		t.Fatal(blockErr)
	}
	if !blocked {
		t.Fatal("unpaid server add-on invoice did not block transfer")
	}
	if _, err = database.Exec(ctx, `DELETE FROM plugin_idc_core_business_operation;
UPDATE plugin_idc_core_server_accessory_entitlement SET pending_invoice_no='inv-renew' WHERE id=2`); err != nil {
		t.Fatal(err)
	}
	blocked, blockErr = transferBlockState(ctx, database, "srv-1", "order-1")
	if blockErr != nil {
		t.Fatal(blockErr)
	}
	if !blocked {
		t.Fatal("unpaid accessory renewal invoice did not block transfer")
	}
}

func transferTestSchema(t *testing.T) string {
	t.Helper()
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		t.Fatal(err)
	}
	return "linapro_transfer_rule_" + hex.EncodeToString(value)
}

const transferRuleFixtureSQL = `
CREATE TABLE plugin_idc_core_server_operation_execution (tenant_id INT,service_no TEXT,status_code TEXT);
CREATE TABLE plugin_idc_core_service_instance (tenant_id INT,service_no TEXT,source_fulfillment_no TEXT,order_item_no TEXT);
CREATE TABLE plugin_idc_core_commerce_fulfillment_unit (tenant_id INT,fulfillment_no TEXT,unit_sequence INT);
CREATE TABLE plugin_idc_core_service_entitlement (tenant_id INT,order_item_no TEXT,unit_sequence INT,entitlement_no TEXT);
CREATE TABLE plugin_idc_core_refund_entitlement_lock (tenant_id INT,entitlement_no TEXT,released_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_billing_invoice (tenant_id INT,invoice_no TEXT,order_no TEXT,status_code TEXT);
CREATE TABLE plugin_idc_core_billing_invoice_item (tenant_id INT,invoice_no TEXT,source_reference TEXT);
CREATE TABLE plugin_idc_core_business_operation (tenant_id INT,service_no TEXT,invoice_no TEXT,operation_type_code TEXT);
CREATE TABLE plugin_idc_core_server_accessory_entitlement (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,tenant_id INT,service_no TEXT,customer_no TEXT,
 billing_mode_code TEXT,status_code TEXT,current_period_end_at TIMESTAMPTZ,pending_invoice_no TEXT,
 auto_renew_enabled BOOLEAN,deleted_at TIMESTAMPTZ,version BIGINT DEFAULT 1,updated_at TIMESTAMPTZ);
CREATE TABLE plugin_idc_core_commerce_order (tenant_id INT,order_no TEXT,requires_contract BOOLEAN,status_code TEXT);`
