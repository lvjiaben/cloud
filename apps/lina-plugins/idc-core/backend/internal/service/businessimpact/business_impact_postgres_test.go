// This test verifies provider isolation and blockers through production aggregate SQL.
package businessimpact

import (
	"context"
	"errors"
	"os"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/plugin/pluginhost"
)

// The rollback-only regression uses real owner schema and no host mutations.
func TestBusinessImpactPostgreSQL(t *testing.T) {
	if _, err := NewLazy(nil); err == nil {
		t.Fatal("nil database factory accepted")
	}
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	rollback := errors.New("rollback impact regression")
	handler, err := NewLazy(func() gdb.DB { return db })
	if err != nil {
		t.Fatal(err)
	}
	if _, err = handler(ctx, pluginhost.PluginBusinessImpactInput{}); err == nil {
		t.Fatal("empty target accepted")
	}
	err = db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
		// First validate real owner schema; temporary shadows isolate aggregate fixtures.
		if _, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "impact-unrelated"}); err != nil {
			return err
		}
		if _, err := tx.Exec(`CREATE TEMP TABLE plugin_idc_core_catalog_product AS SELECT * FROM public.plugin_idc_core_catalog_product WITH NO DATA; CREATE TEMP TABLE plugin_idc_core_catalog_product_version AS SELECT * FROM public.plugin_idc_core_catalog_product_version WITH NO DATA; CREATE TEMP TABLE plugin_idc_core_catalog_product_version_fulfillment_binding AS SELECT * FROM public.plugin_idc_core_catalog_product_version_fulfillment_binding WITH NO DATA; CREATE TEMP TABLE plugin_idc_core_product_fulfillment_intent AS SELECT * FROM public.plugin_idc_core_product_fulfillment_intent WITH NO DATA; CREATE TEMP TABLE plugin_idc_core_service_entitlement AS SELECT * FROM public.plugin_idc_core_service_entitlement WITH NO DATA; CREATE TEMP TABLE plugin_idc_core_managed_service_action_task AS SELECT * FROM public.plugin_idc_core_managed_service_action_task WITH NO DATA; CREATE TEMP TABLE plugin_idc_core_business_operation AS SELECT * FROM public.plugin_idc_core_business_operation WITH NO DATA;`); err != nil {
			return err
		}
		if _, err := tx.Exec(`ALTER TABLE plugin_idc_core_catalog_product ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_core_catalog_product_version ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_binding ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_core_product_fulfillment_intent ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_core_service_entitlement ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_core_managed_service_action_task ALTER COLUMN tenant_id SET DEFAULT 0; ALTER TABLE plugin_idc_core_business_operation ALTER COLUMN tenant_id SET DEFAULT 0;`); err != nil {
			return err
		}
		if _, err := tx.Exec(`CREATE TEMP TABLE plugin_idc_core_commerce_order AS SELECT * FROM public.plugin_idc_core_commerce_order WITH NO DATA;
CREATE TEMP TABLE plugin_idc_core_commerce_order_item AS SELECT * FROM public.plugin_idc_core_commerce_order_item WITH NO DATA;
CREATE TEMP TABLE plugin_idc_core_commerce_order_cancellation AS SELECT * FROM public.plugin_idc_core_commerce_order_cancellation WITH NO DATA;
CREATE TEMP TABLE plugin_idc_core_refund_request AS SELECT * FROM public.plugin_idc_core_refund_request WITH NO DATA;
CREATE TEMP TABLE plugin_idc_core_refund_entitlement_lock AS SELECT * FROM public.plugin_idc_core_refund_entitlement_lock WITH NO DATA;`); err != nil {
			return err
		}
		if _, err := tx.Exec(fixtureSQL); err != nil {
			return err
		}
		items, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "impact-test-provider"})
		if err != nil {
			return err
		}
		got := map[pluginhost.BusinessImpactCategory]pluginhost.PluginBusinessImpactItem{}
		for _, item := range items {
			got[item.Category] = item
		}
		for _, key := range []pluginhost.BusinessImpactCategory{pluginhost.BusinessImpactSellingProductVersions, pluginhost.BusinessImpactInFlightTasks, pluginhost.BusinessImpactActiveServices} {
			if got[key].BlockingCount != 1 {
				t.Fatalf("missing blocker %s: %+v", key, got)
			}
		}
		if got[pluginhost.BusinessImpactPendingHistoricalActions].BlockingCount != 4 {
			t.Fatalf("pending compensation or historical action missed: %+v", got)
		}
		if _, err := tx.Exec(`UPDATE plugin_idc_core_catalog_product SET status_code='paused';
UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed';
UPDATE plugin_idc_core_service_entitlement SET status_code='terminated';
UPDATE plugin_idc_core_commerce_order SET status_code='fulfilled';
UPDATE plugin_idc_core_commerce_order_cancellation SET status_code='completed';
UPDATE plugin_idc_core_refund_request SET status_code='succeeded';
UPDATE plugin_idc_core_refund_entitlement_lock SET released_at=CURRENT_TIMESTAMP;`); err != nil {
			return err
		}
		drained, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "impact-test-provider"})
		if err != nil {
			return err
		}
		for _, item := range drained {
			if item.BlockingCount != 0 || item.TotalCount == 0 {
				t.Fatalf("drained history lost or blocks: %+v", item)
			}
		}
		other, err := handler(ctx, pluginhost.PluginBusinessImpactInput{TargetPluginID: "impact-unrelated"})
		if err != nil {
			return err
		}
		for _, item := range other {
			if item.TotalCount != 0 || item.BlockingCount != 0 {
				t.Fatalf("foreign target leaked: %+v", item)
			}
		}
		return rollback
	})
	if err != nil && !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}

const fixtureSQL = `INSERT INTO plugin_idc_core_catalog_product(product_no,category_no,status_code,name_zh_cn,name_en_us,public_slug,current_version_no) VALUES('impact-product','impact-category','active','测试','Test','impact-test-provider','impact-version');
INSERT INTO plugin_idc_core_catalog_product_version(version_no,product_no,status_code,version_number) VALUES('impact-version','impact-product','published',1);
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding(tenant_id,version_no,plugin_id) VALUES(0,'impact-version','impact-test-provider');
INSERT INTO plugin_idc_core_product_fulfillment_intent(intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,initial_status_code,status_code,plugin_id,policy_digest,evidence_digest,financially_settled_at) VALUES('impact-intent','impact-settlement','impact-order','impact-item','impact-customer','impact-invoice','CNY','impact-product','impact-version','测试','Test',1,1,'manual','per_order_item','manual_pending','manual_pending','impact-test-provider',repeat('a',64),repeat('b',64),CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_service_entitlement(entitlement_no,delivery_no,source_intent_no,customer_no,order_no,order_item_no,product_no,version_no,unit_sequence,plugin_id,status_code) VALUES('impact-service','impact-delivery','impact-intent','impact-customer','impact-order','impact-item','impact-product','impact-version',1,'impact-test-provider','active');
INSERT INTO plugin_idc_core_commerce_order(tenant_id,order_no,status_code) VALUES(0,'impact-order','pending_payment');
INSERT INTO plugin_idc_core_commerce_order_item(tenant_id,order_no,version_no) VALUES(0,'impact-order','impact-version');
INSERT INTO plugin_idc_core_commerce_order_cancellation(tenant_id,order_no,status_code) VALUES(0,'impact-order','pending_payment_closure');
INSERT INTO plugin_idc_core_refund_request(tenant_id,order_no,status_code) VALUES(0,'impact-order','executing');
INSERT INTO plugin_idc_core_refund_entitlement_lock(tenant_id,entitlement_no) VALUES(0,'impact-service');`
