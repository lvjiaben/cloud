// This rollback-only regression drives the real owner release through PostgreSQL and frozen evidence.
package productfulfillment

import (
	"context"
	"errors"
	"os"
	"testing"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/internal/service/productautomation"
)

type recoveryFrozen struct{}

func (recoveryFrozen) Validate(context.Context, productautomation.FrozenCustomerConfigurationInput) (*productautomation.FrozenCustomerConfigurationEvidence, error) {
	return &productautomation.FrozenCustomerConfigurationEvidence{}, nil
}
func (recoveryFrozen) ValidateBatch(_ context.Context, in []productautomation.FrozenCustomerConfigurationInput) ([]productautomation.FrozenCustomerConfigurationEvidence, error) {
	return make([]productautomation.FrozenCustomerConfigurationEvidence, len(in)), nil
}

type recoveryReasons struct{}

func (recoveryReasons) ResolveSystemTransitionReason(string) (SystemTransitionReason, error) {
	return SystemTransitionReason{LabelZhCN: "已停止新业务", LabelEnUS: "New business stopped"}, nil
}

// TestOwnerGateRecoveryPostgreSQL covers pending approval, one-task replay, cancellation and stop-new-business.
func TestOwnerGateRecoveryPostgreSQL(t *testing.T) {
	dsn := os.Getenv("LINAPRO_IDC_CORE_TEST_DSN")
	if dsn == "" {
		t.Skip("set LINAPRO_IDC_CORE_TEST_DSN")
	}
	db, err := gdb.New(gdb.ConfigNode{Link: dsn})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	rollback := errors.New("rollback gate recovery")
	err = db.Transaction(t.Context(), func(ctx context.Context, tx gdb.TX) error {
		if _, err := tx.Exec(`CREATE TEMP TABLE plugin_idc_core_financial_settlement AS SELECT * FROM public.plugin_idc_core_financial_settlement WITH NO DATA; ALTER TABLE plugin_idc_core_financial_settlement ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_commerce_order AS SELECT * FROM public.plugin_idc_core_commerce_order WITH NO DATA; ALTER TABLE plugin_idc_core_commerce_order ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_channel_order_attribution AS SELECT * FROM public.plugin_idc_core_channel_order_attribution WITH NO DATA; ALTER TABLE plugin_idc_core_channel_order_attribution ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_commerce_order_item AS SELECT * FROM public.plugin_idc_core_commerce_order_item WITH NO DATA; ALTER TABLE plugin_idc_core_commerce_order_item ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_commerce_order_item_quantity_snapshot AS SELECT * FROM public.plugin_idc_core_commerce_order_item_quantity_snapshot WITH NO DATA; ALTER TABLE plugin_idc_core_commerce_order_item_quantity_snapshot ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_catalog_product_version_fulfillment_policy AS SELECT * FROM public.plugin_idc_core_catalog_product_version_fulfillment_policy WITH NO DATA; ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_policy ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_catalog_product_version_fulfillment_binding AS SELECT * FROM public.plugin_idc_core_catalog_product_version_fulfillment_binding WITH NO DATA; ALTER TABLE plugin_idc_core_catalog_product_version_fulfillment_binding ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_product_automation_interface_instance AS SELECT * FROM public.plugin_idc_core_product_automation_interface_instance WITH NO DATA; ALTER TABLE plugin_idc_core_product_automation_interface_instance ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_commerce_structured_configuration_snapshot AS SELECT * FROM public.plugin_idc_core_commerce_structured_configuration_snapshot WITH NO DATA; ALTER TABLE plugin_idc_core_commerce_structured_configuration_snapshot ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_catalog_product_version_configuration AS SELECT * FROM public.plugin_idc_core_catalog_product_version_configuration WITH NO DATA; ALTER TABLE plugin_idc_core_catalog_product_version_configuration ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_product_fulfillment_intent AS SELECT * FROM public.plugin_idc_core_product_fulfillment_intent WITH NO DATA; ALTER TABLE plugin_idc_core_product_fulfillment_intent ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_product_fulfillment_task AS SELECT * FROM public.plugin_idc_core_product_fulfillment_task WITH NO DATA; ALTER TABLE plugin_idc_core_product_fulfillment_task ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE TEMP TABLE plugin_idc_core_product_fulfillment_system_transition AS SELECT * FROM public.plugin_idc_core_product_fulfillment_system_transition WITH NO DATA; ALTER TABLE plugin_idc_core_product_fulfillment_system_transition ALTER COLUMN tenant_id SET DEFAULT 0;
CREATE UNIQUE INDEX ON plugin_idc_core_product_fulfillment_task(intent_no);
CREATE UNIQUE INDEX ON plugin_idc_core_product_fulfillment_system_transition(intent_no,event_code);`); err != nil {
			return err
		}
		if _, err := tx.Exec(recoveryFixture); err != nil {
			return err
		}
		gates, err := NewOwnerGateValidator(db)
		if err != nil {
			return err
		}
		owner, err := NewOwnerGateReleaser(db, recoveryFrozen{}, gates, recoveryReasons{})
		if err != nil {
			return err
		}
		worker, err := NewGateRecoveryWorker(db, owner)
		if err != nil {
			return err
		}
		got, err := worker.Run(ctx, 20)
		if err != nil || got.Selected != 1 || got.Queued != 0 {
			t.Fatalf("pending gate: %+v %v", got, err)
		}
		if _, err := tx.Exec("UPDATE plugin_idc_core_commerce_order SET risk_decision_code='approved'"); err != nil {
			return err
		}
		got, err = worker.Run(ctx, 20)
		if err != nil || got.Queued != 1 {
			t.Fatalf("approved recovery: %+v %v", got, err)
		}
		if _, err := owner.ReleaseOwnerGates(ctx, OwnerGateReleaseInput{OrderNo: "ORD_recovery", OrderItemNos: []string{"OIT_recovery"}}); err != nil {
			return err
		}
		count, err := db.GetValue(ctx, "SELECT COUNT(*) FROM plugin_idc_core_product_fulfillment_task")
		if err != nil || count.Int() != 1 {
			t.Fatalf("duplicate task: %v %v", count, err)
		}
		if _, err := tx.Exec("DELETE FROM plugin_idc_core_product_fulfillment_task; UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='canceled'"); err != nil {
			return err
		}
		got, err = worker.Run(ctx, 20)
		if err != nil || got.Selected != 0 {
			t.Fatalf("canceled resurrected: %+v %v", got, err)
		}
		if _, err := tx.Exec("UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='gate_pending'; UPDATE plugin_idc_core_product_automation_interface_instance SET new_business_state='stopped'"); err != nil {
			return err
		}
		got, err = worker.Run(ctx, 20)
		if err != nil || got.Manual != 1 || got.Queued != 0 {
			t.Fatalf("stopped admission: %+v %v", got, err)
		}
		return rollback
	})
	if !errors.Is(err, rollback) {
		t.Fatal(err)
	}
}

const recoveryFixture = `
INSERT INTO plugin_idc_core_financial_settlement(settlement_no,order_no,customer_no,invoice_no,currency_code,settled_at) VALUES('FST_recovery','ORD_recovery','CUS_recovery','INV_recovery','CNY',CURRENT_TIMESTAMP);
INSERT INTO plugin_idc_core_commerce_order(order_no,customer_no,status_code,risk_decision_code,requires_contract,item_count) VALUES('ORD_recovery','CUS_recovery','paid','pending',false,1);
INSERT INTO plugin_idc_core_commerce_order_item(order_no,order_item_no,product_no,version_no,product_name_zh_cn,product_name_en_us,inventory_decision_code,line_number) VALUES('ORD_recovery','OIT_recovery','PRD_recovery','VER_recovery','测试','Test','available',1);
INSERT INTO plugin_idc_core_commerce_order_item_quantity_snapshot(order_item_no,quantity,content_digest) VALUES('OIT_recovery',1,repeat('a',64));
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_policy(version_no,strategy_code,granularity_code,ongoing_management,renewal_mode_code,renewal_method,entitlement_owner_id,provision_method,query_method,maximum_attempts,retry_base_seconds,policy_digest) VALUES('VER_recovery','payment_confirmed_plugin','per_order_item',false,'unavailable','','','provision','query',10,30,repeat('b',64));
INSERT INTO plugin_idc_core_catalog_product_version_fulfillment_binding(version_no,interface_instance_no,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,authorized_methods) VALUES('VER_recovery','PFI_recovery','idc-peer','v0.1.0','v1',repeat('a',64),repeat('b',64),ARRAY['provision','query']);
INSERT INTO plugin_idc_core_product_automation_interface_instance(interface_instance_no,new_business_state) VALUES('PFI_recovery','accepting');
INSERT INTO plugin_idc_core_commerce_structured_configuration_snapshot(order_item_no,snapshot_no,configuration_revision,interface_instance_no,interface_configuration_revision,content_digest,candidate_snapshot,typed_value_snapshot) VALUES('OIT_recovery','CFS_recovery','revision','PFI_recovery',1,repeat('a',64),'{}','{}');
INSERT INTO plugin_idc_core_catalog_product_version_configuration(version_no,customer_declaration_snapshot) VALUES('VER_recovery','{"version":"v1","fields":[]}');
INSERT INTO plugin_idc_core_product_fulfillment_intent(intent_no,order_no,order_item_no,strategy_code,status_code,maximum_attempts,retry_base_seconds) VALUES('PFI_intent','ORD_recovery','OIT_recovery','payment_confirmed_plugin','gate_pending',10,30);
`
