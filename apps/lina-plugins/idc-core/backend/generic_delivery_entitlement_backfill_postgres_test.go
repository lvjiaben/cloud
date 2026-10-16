package backend

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"
)

func TestGenericDeliveryEntitlementBackfillPostgreSQL(t *testing.T) {
	ctx := context.Background()
	dsn := strings.TrimSpace(os.Getenv(catalogIntegrationDSNEnv))
	if dsn == "" {
		t.Skipf("set %s to run PostgreSQL delivery backfill tests", catalogIntegrationDSNEnv)
	}
	adminDB, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatalf("open delivery backfill database: %v", err)
	}
	schema := newBillingIntegrationSchema(t)
	if _, err = adminDB.Exec(ctx, "CREATE SCHEMA "+quoteCatalogIdentifier(schema)); err != nil {
		t.Fatalf("create isolated delivery backfill schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.Exec(ctx, "DROP SCHEMA "+quoteCatalogIdentifier(schema)+" CASCADE")
		_ = adminDB.Close(ctx)
	})
	database, err := gdb.New(gdb.ConfigNode{Link: appendCatalogDSNParameter(dsn, "search_path="+url.QueryEscape(schema))})
	if err != nil {
		t.Fatalf("open isolated delivery backfill database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close(ctx) })

	if _, err = database.Exec(ctx, genericDeliveryBackfillFixtureSQL); err != nil {
		t.Fatalf("create delivery backfill fixture: %v", err)
	}
	migration := readDeliveryBackfillMigration(t)
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err = database.Exec(ctx, migration); err != nil {
			t.Fatalf("execute delivery backfill attempt %d: %v", attempt, err)
		}
	}

	var evidence struct {
		Deliveries         int `orm:"deliveries"`
		Entitlements       int `orm:"entitlements"`
		Events             int `orm:"events"`
		ServerOngoing      int `orm:"server_ongoing"`
		PhoneOngoing       int `orm:"phone_ongoing"`
		ServerVersion      int `orm:"server_version"`
		DeliveryVersion    int `orm:"delivery_version"`
		EntitlementVersion int `orm:"entitlement_version"`
	}
	if err = database.GetScan(ctx, &evidence, `SELECT
 (SELECT count(*) FROM plugin_idc_core_product_delivery_record) deliveries,
 (SELECT count(*) FROM plugin_idc_core_service_entitlement) entitlements,
 (SELECT count(*) FROM plugin_idc_core_entitlement_created_outbox) events,
 (SELECT count(*) FROM plugin_idc_core_product_fulfillment_intent WHERE intent_no='intent-server' AND ongoing_management) server_ongoing,
 (SELECT count(*) FROM plugin_idc_core_product_fulfillment_intent WHERE intent_no='intent-phone' AND ongoing_management) phone_ongoing,
 (SELECT version FROM plugin_idc_core_product_fulfillment_intent WHERE intent_no='intent-server') server_version,
 (SELECT version FROM plugin_idc_core_product_delivery_record WHERE order_item_no='item-server') delivery_version,
 (SELECT version FROM plugin_idc_core_service_entitlement WHERE source_intent_no='intent-server') entitlement_version`); err != nil {
		t.Fatalf("read delivery backfill evidence: %v", err)
	}
	if evidence.Deliveries != 2 || evidence.Entitlements != 1 || evidence.Events != 1 ||
		evidence.ServerOngoing != 1 || evidence.PhoneOngoing != 0 ||
		evidence.ServerVersion != 2 || evidence.DeliveryVersion != 1 || evidence.EntitlementVersion != 1 {
		t.Fatalf("unexpected idempotent delivery backfill evidence: %+v", evidence)
	}

	var entitlement struct {
		StatusCode string `orm:"status_code"`
		OwnerID    string `orm:"owner_id"`
		ExpiresAt  string `orm:"expires_at"`
		NameZhCN   string `orm:"product_name_zh_cn"`
		NameEnUS   string `orm:"product_name_en_us"`
	}
	if err = database.GetScan(ctx, &entitlement, `SELECT entitlement.status_code,event.owner_id,
 event.product_name_zh_cn,event.product_name_en_us,
 to_char(entitlement.expires_at AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS') expires_at
 FROM plugin_idc_core_service_entitlement entitlement
 JOIN plugin_idc_core_entitlement_created_outbox event USING(entitlement_no)`); err != nil {
		t.Fatalf("read migrated entitlement: %v", err)
	}
	if entitlement.StatusCode != "active" || entitlement.OwnerID != "idc-resource" ||
		entitlement.NameZhCN != "云服务器" || entitlement.NameEnUS != "Cloud server" ||
		entitlement.ExpiresAt != "2027-01-01 00:00:00" {
		t.Fatalf("legacy server entitlement evidence was not preserved: %+v", entitlement)
	}

	if _, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_service_instance(
 tenant_id,service_no,source_code,source_fulfillment_no,order_no,order_item_no,customer_no,product_no,version_no,
 entitlement_status_code,entitlement_started_at,entitlement_expires_at,created_at,updated_at)
 VALUES(0,'service-unmapped','order','unit-unmapped','order-unmapped','item-unmapped','customer-a','product-a','version-a',
 'active',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 month',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)`); err != nil {
		t.Fatalf("insert ambiguous legacy service: %v", err)
	}
	if _, err = database.Exec(ctx, migration); err == nil || !strings.Contains(err.Error(), "cannot safely migrate 1 legacy order service") {
		t.Fatalf("ambiguous legacy service did not fail closed: %v", err)
	}
}

func readDeliveryBackfillMigration(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve delivery backfill migration path")
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "manifest", "sql", "090-generic-delivery-entitlement-backfill.sql"))
	if err != nil {
		t.Fatalf("read delivery backfill migration: %v", err)
	}
	return string(content)
}

const genericDeliveryBackfillFixtureSQL = `
CREATE TABLE plugin_idc_core_service_instance (
 tenant_id INT,service_no TEXT,source_code TEXT,source_fulfillment_no TEXT,order_no TEXT,order_item_no TEXT,
 customer_no TEXT,product_no TEXT,version_no TEXT,entitlement_status_code TEXT,
 entitlement_started_at TIMESTAMPTZ,entitlement_expires_at TIMESTAMPTZ,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ
);
CREATE TABLE plugin_idc_core_commerce_fulfillment_unit (
 tenant_id INT,fulfillment_no TEXT,order_item_no TEXT,unit_sequence INT
);
CREATE TABLE plugin_idc_core_commerce_order_item (
 tenant_id INT,order_item_no TEXT,currency_code TEXT,billing_rule_no TEXT,charge_type_code TEXT,period_unit_code TEXT,period_count INT
);
CREATE TABLE plugin_idc_core_product_fulfillment_intent (
 tenant_id INT,intent_no TEXT,order_no TEXT,order_item_no TEXT,customer_no TEXT,product_no TEXT,version_no TEXT,
 product_name_zh_cn TEXT,product_name_en_us TEXT,
 quantity INT,unit_sequence INT,strategy_code TEXT,policy_digest TEXT,configuration_snapshot_no TEXT,
 interface_instance_no TEXT,plugin_id TEXT,plugin_release_version TEXT,contract_version TEXT,declaration_digest TEXT,
 ongoing_management BOOLEAN,renewal_mode_code TEXT,renewal_method TEXT,entitlement_owner_id TEXT,status_code TEXT,
 financially_settled_at TIMESTAMPTZ,version INT,updated_at TIMESTAMPTZ
);
CREATE TABLE plugin_idc_core_product_delivery_record (
 tenant_id INT,delivery_no TEXT,order_no TEXT,order_item_no TEXT UNIQUE,customer_no TEXT,product_no TEXT,version_no TEXT,
 quantity INT,strategy_code TEXT,policy_digest TEXT,configuration_snapshot_no TEXT,interface_instance_no TEXT,plugin_id TEXT,
 plugin_release_version TEXT,contract_version TEXT,declaration_digest TEXT,ongoing_management BOOLEAN,status_code TEXT,
 renewal_mode_code TEXT,renewal_method TEXT,entitlement_owner_id TEXT,started_at TIMESTAMPTZ,completed_at TIMESTAMPTZ,
 version INT,content_digest TEXT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ
);
CREATE TABLE plugin_idc_core_service_entitlement (
 tenant_id INT,entitlement_no TEXT UNIQUE,delivery_no TEXT,source_intent_no TEXT UNIQUE,customer_no TEXT,order_no TEXT,order_item_no TEXT,
 product_no TEXT,version_no TEXT,unit_sequence INT,configuration_snapshot_no TEXT,interface_instance_no TEXT,plugin_id TEXT,
 plugin_release_version TEXT,contract_version TEXT,declaration_digest TEXT,status_code TEXT,started_at TIMESTAMPTZ,expires_at TIMESTAMPTZ,
 renewal_mode_code TEXT,renewal_method TEXT,entitlement_owner_id TEXT,currency_code TEXT,billing_rule_no TEXT,
 charge_type_code TEXT,period_unit_code TEXT,period_count INT,version INT,created_at TIMESTAMPTZ,updated_at TIMESTAMPTZ
);
CREATE TABLE plugin_idc_core_entitlement_created_outbox (
 tenant_id INT,event_no TEXT,entitlement_no TEXT UNIQUE,intent_no TEXT,customer_no TEXT,product_no TEXT,version_no TEXT,
 product_name_zh_cn TEXT,product_name_en_us TEXT,
 configuration_snapshot_no TEXT,interface_instance_no TEXT,plugin_id TEXT,plugin_release_version TEXT,
 contract_version TEXT,declaration_digest TEXT,owner_id TEXT
);
INSERT INTO plugin_idc_core_commerce_order_item VALUES
 (0,'item-server','CNY','bill-month','recurring','month',1),
 (0,'item-phone','CNY','bill-once','one_time','once',1);
INSERT INTO plugin_idc_core_product_fulfillment_intent VALUES
 (0,'intent-server','order-server','item-server','customer-a','product-server','version-server','云服务器','Cloud server',1,1,
  'payment_confirmed_plugin',repeat('a',64),'config-server','pfi_11111111111111111111111111111111','server-plugin','1.0.0','v1',repeat('b',64),
  FALSE,'unavailable','','','completed','2026-01-01 00:00:00+00',1,'2026-01-01 00:10:00+00'),
 (0,'intent-phone','order-phone','item-phone','customer-a','product-phone','version-phone','手机','Phone',1,1,
  'manual',repeat('c',64),'','','','','','',FALSE,'unavailable','','','completed','2026-01-02 00:00:00+00',1,'2026-01-02 00:10:00+00');
INSERT INTO plugin_idc_core_commerce_fulfillment_unit VALUES(0,'unit-server','item-server',1);
INSERT INTO plugin_idc_core_service_instance VALUES(
 0,'service-server','order','unit-server','order-server','item-server','customer-a','product-server','version-server',
 'active','2026-01-01 00:10:00+00','2027-01-01 00:00:00+00','2026-01-01 00:10:00+00','2026-02-01 00:00:00+00');
`
