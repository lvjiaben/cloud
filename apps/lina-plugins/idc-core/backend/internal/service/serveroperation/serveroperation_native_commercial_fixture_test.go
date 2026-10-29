// Native commercial integration fixtures use real commerce, billing, generic
// entitlement and accessory migrations. Authentication and resource facts are
// explicit local boundaries; no provider or shared account is called.
package serveroperation

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

type nativeCommercialFixture struct {
	accessoryCheckoutFixture
	native *nativeCommercialService
	target servercommercialcap.TargetSnapshot
}

// newNativeCommercialFixture starts with a month/1 paid parent delivery with a
// persisted manual completion, never an imported legacy service alias.
func newNativeCommercialFixture(t *testing.T) nativeCommercialFixture {
	t.Helper()
	return newNativeCommercialFixtureWithSeed(t, accessoryCheckoutSeedSQL)
}

// newNativeCommercialFixtureWithSeed preserves owner DDL while permitting a
// public-catalog fixture to use identifiers created by the real catalog API.
func newNativeCommercialFixtureWithSeed(t *testing.T, seed string) nativeCommercialFixture {
	t.Helper()
	seed = strings.ReplaceAll(seed, "'one_time','once'", "'recurring','month'")
	f := newAccessoryCheckoutFixtureWithSeed(t, seed)
	for _, name := range []string{"056-server-operation-execution.sql", "057-server-accessory-operation.sql", "058-server-accessory-pricing.sql", "059-server-accessory-checkout.sql", "060-server-accessory-entitlement.sql", "064-server-ip-change.sql", "065-server-customer-metadata.sql", "066-server-safe-batch.sql", "067-server-resize-transfer.sql", "087-generic-refund-entitlement.sql", "088-generic-entitlement-renewal.sql", "103-channel-renewal-payer.sql", "104-channel-service-operation.sql", "105-channel-priced-service-operation.sql", "109-server-accessory-settlement-index.sql", "121-peer-service-mapping.sql", "138-recurring-entitlement-activation.sql", "151-native-server-accessory-lifecycle.sql", "152-native-priced-server-operation.sql"} {
		nativeLifecycleMigration(t, f.database, name)
	}
	if err := f.checkout(); err != nil {
		t.Fatal(err)
	}
	f.pay(t)
	if _, err := f.settlementService(t).SettleOrder(f.ctx, "aor_checkout_test"); err != nil {
		t.Fatal(err)
	}
	if err := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := f.database.Exec(ctx, nativeCommercialParentSQL)
		return err
	}); err != nil {
		t.Fatalf("native parent delivery: %+v", err)
	}
	var parent struct {
		EntitlementNo string
		Version       int64
	}
	if err := f.database.GetScan(f.ctx, &parent, `SELECT entitlement_no,version FROM plugin_idc_core_service_entitlement WHERE source_intent_no='INT_native_checkout'`); err != nil || parent.EntitlementNo == "" {
		t.Fatalf("read native parent: %+v / %+v", parent, err)
	}

	if err := f.database.Transaction(f.ctx, func(ctx context.Context, _ gdb.TX) error {
		_, err := f.database.Exec(ctx, nativeCommercialPolicySQL)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	f.service.bizCtx = trafficTestStaff{bizctxcap.CurrentContext{UserID: 101, DataScope: 1}}
	f.service.principal = cancellationPrincipal{}
	f.service.customers = accessoryChainCustomer{}
	f.service.audit = trafficTestAudit{}
	f.service.parameters = &nativePriceSourceFixture{}
	evidence, err := billing.NewSettlementEvidenceReader(f.database)
	if err != nil {
		t.Fatal(err)
	}
	invoices, err := billing.NewInvoiceService(f.database)
	if err != nil {
		t.Fatal(err)
	}
	native := &nativeCommercialService{database: f.database, pricing: f.service, prices: &NativePriceBuilder{database: f.database}, evidence: evidence, invoices: invoices}
	target := servercommercialcap.TargetSnapshot{EntitlementNo: parent.EntitlementNo, CustomerNo: "CUS_checkout_test", LocalResourceNo: "NATIVE_checkout", ChannelNo: "CHN_checkout_test", PoolNo: "POOL_checkout_test", EntitlementVersion: parent.Version, RuntimeStatus: "running", SnapshotNo: "native-fresh-projection", ObservedAt: time.Now().UTC().Add(-time.Second), ExpiresAt: time.Now().UTC().Add(time.Hour), Actions: []serveroperationresourcecap.Action{serveroperationresourcecap.ActionSnapshotCreate, serveroperationresourcecap.ActionBackupCreate, serveroperationresourcecap.ActionTrafficAdd, serveroperationresourcecap.ActionChangeIP, serveroperationresourcecap.ActionResize}}
	return nativeCommercialFixture{f, native, target}
}

// payNativeInvoice uses independent immutable references for each real payment.
func (f nativeCommercialFixture) payNativeInvoice(t *testing.T, invoice string, amount int64) {
	t.Helper()
	balance, err := billing.NewBalanceService(f.database)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = balance.PostConfirmedRecharge(f.ctx, billing.ConfirmedRechargeInput{CustomerNo: f.target.CustomerNo, CurrencyCode: "CNY", AmountMinor: amount, SourceReference: "native-recharge:" + invoice, SourceSnapshotDigest: strings.Repeat("e", 64)}); err != nil {
		t.Fatal(err)
	}
	if _, err = f.native.invoices.PayFromBalance(f.ctx, billing.InvoiceBalancePaymentInput{InvoiceNo: invoice, SourceReference: "native-payment:" + invoice, SourceSnapshotDigest: strings.Repeat("f", 64)}); err != nil {
		t.Fatal(err)
	}
}

const nativeCommercialParentSQL = `
INSERT INTO plugin_idc_core_product_delivery_record(delivery_no,order_no,order_item_no,customer_no,product_no,version_no,quantity,strategy_code,policy_digest,configuration_snapshot_no,interface_instance_no,plugin_id,plugin_release_version,contract_version,declaration_digest,ongoing_management,status_code,content_digest,renewal_mode_code,entitlement_owner_id)
SELECT 'DEL_native_checkout',o.order_no,i.order_item_no,o.customer_no,i.product_no,i.version_no,1,'payment_confirmed_plugin',repeat('a',64),'CFG_checkout_test','pfi_00000000000000000000000000000001','idc-resource','1.0.0','1',repeat('a',64),TRUE,'manual_pending',repeat('a',64),'local_entitlement','idc-resource'
FROM plugin_idc_core_commerce_order o JOIN plugin_idc_core_commerce_order_item i USING(order_no) WHERE o.order_no='aor_checkout_test';
INSERT INTO plugin_idc_core_product_fulfillment_intent(intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,strategy_code,granularity_code,initial_status_code,status_code,configuration_snapshot_no,interface_instance_no,interface_configuration_revision,plugin_id,plugin_release_version,contract_version,release_digest,declaration_digest,provision_method,query_method,policy_digest,evidence_digest,financially_settled_at,ongoing_management,renewal_mode_code,entitlement_owner_id)
SELECT 'INT_native_checkout',s.settlement_no,s.order_no,i.order_item_no,s.customer_no,s.invoice_no,s.currency_code,i.product_no,i.version_no,i.product_name_zh_cn,i.product_name_en_us,1,1,'payment_confirmed_plugin','per_order_item','manual_pending','manual_pending','CFG_checkout_test','pfi_00000000000000000000000000000001',1,'idc-resource','1.0.0','1',repeat('a',64),repeat('a',64),'server.provision','server.query',repeat('a',64),repeat('a',64),s.settled_at,TRUE,'local_entitlement','idc-resource'
FROM plugin_idc_core_financial_settlement s JOIN plugin_idc_core_commerce_order_item i USING(order_no) WHERE s.order_no='aor_checkout_test';
INSERT INTO plugin_idc_core_product_fulfillment_manual_conclusion(conclusion_no,intent_no,outcome_code,reason_code,note,actor_user_id,intent_version) VALUES('CON_native_checkout','INT_native_checkout','completed','manual_fulfillment_completed','Synthetic native delivery completed',101,1);
UPDATE plugin_idc_core_product_fulfillment_intent SET status_code='completed',version=version+1,updated_at=CURRENT_TIMESTAMP WHERE intent_no='INT_native_checkout';`

const nativeCommercialPolicySQL = `
INSERT INTO plugin_idc_core_server_accessory_price_policy(policy_no,accessory_kind,currency_code,current_version,enabled,stock_quantity) VALUES
('POL_native_snapshot','snapshot','CNY',1,TRUE,NULL),('POL_native_backup','backup','CNY',1,TRUE,NULL),('POL_checkout_test','traffic','CNY',1,TRUE,1);
INSERT INTO plugin_idc_core_server_accessory_price_policy_version(policy_no,policy_version,accessory_kind,currency_code,free_count_per_server,unit_amount_minor,period_unit_code,period_count,content_digest,published_by,traffic_gb,name_zh_cn,name_en_us) VALUES
('POL_native_snapshot',1,'snapshot','CNY',1,1000,'month',1,repeat('a',64),101,0,'',''),
('POL_native_backup',1,'backup','CNY',0,1000,'month',1,repeat('a',64),101,0,'',''),
('POL_checkout_test',1,'traffic','CNY',0,1000,'once',1,repeat('a',64),101,100,'测试流量包','Test traffic package');
INSERT INTO plugin_idc_core_traffic_package_product(policy_no,policy_version,product_no) VALUES('POL_checkout_test',1,'prd_checkout_test');`
