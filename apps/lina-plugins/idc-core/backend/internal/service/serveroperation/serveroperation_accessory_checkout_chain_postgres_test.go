// This file verifies the complete local accessory checkout/settlement chain
// against real migrations. It never dispatches a provider write or creates a
// new server, and the authenticated customer/audit boundaries are explicit fakes.

package serveroperation

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/dialect"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
	"lina-plugin-idc-core/backend/internal/service/outbox"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

// accessoryChainCustomer is the trusted current-customer boundary for the test.
type accessoryChainCustomer struct{}

func (accessoryChainCustomer) CurrentCustomer(context.Context) (*identitycap.CustomerSubject, error) {
	return &identitycap.CustomerSubject{CustomerNo: "CUS_checkout_test", ActorUserID: 101}, nil
}

// accessoryChainAudit records whether public checkout retained its audit call.
type accessoryChainAudit struct{ calls int }

func (audit *accessoryChainAudit) AppendBusinessTransition(context.Context, idccap.BusinessAuditTransition) error {
	audit.calls++
	return nil
}

func (*accessoryChainAudit) AppendBusinessBatchTransition(context.Context, idccap.BusinessAuditBatchTransition) (idccap.BusinessAuditAppendResult, error) {
	return idccap.BusinessAuditAppendResult{}, errors.New("unexpected batch audit")
}

// accessoryChainGeneric fails if addon settlement reaches new-product fulfillment.
type accessoryChainGeneric struct{ calls int }

func (generic *accessoryChainGeneric) ConsumeFinanciallySettled(context.Context, productfulfillment.SettlementInput) (*productfulfillment.SettlementResult, error) {
	generic.calls++
	return nil, errors.New("addon reached new-product fulfillment")
}

// TestAccessoryCheckoutToOriginalServerPostgreSQL uses the public customer
// checkout, real balance payment, authoritative settlement and routed consumer.
func TestAccessoryCheckoutToOriginalServerPostgreSQL(t *testing.T) {
	fixture := newAccessoryCheckoutFixture(t)
	prepareAccessoryOriginalServer(t, fixture)
	audit := &accessoryChainAudit{}
	fixture.service.customers = accessoryChainCustomer{}
	fixture.service.audit = audit
	input := AccessoryCheckoutInput{QuoteNo: fixture.row.QuoteNo, IdempotencyKey: "checkout-original-server"}
	checkout, err := fixture.service.CheckoutCurrentCustomer(fixture.ctx, input)
	if err != nil {
		t.Fatalf("public accessory checkout: %+v", err)
	}
	if checkout.Status != "awaiting_financial_settlement" || checkout.ServiceNo != "SRV_checkout_original" ||
		checkout.AmountMinor != 1000 || checkout.Reused || audit.calls != 1 {
		t.Fatalf("unexpected paid checkout: %+v audit=%d", checkout, audit.calls)
	}
	replay, err := fixture.service.CheckoutCurrentCustomer(fixture.ctx, input)
	if err != nil || !replay.Reused || replay.OperationNo != checkout.OperationNo || audit.calls != 1 {
		t.Fatalf("checkout replay duplicated intent/audit: %+v / %+v", replay, err)
	}
	fixture.assertSnapshots(t)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_server_operation_execution`, 0)
	fixture.payInvoice(t, checkout.InvoiceNo)
	settled, err := fixture.settlementService(t).SettleOrder(fixture.ctx, checkout.OrderNo)
	if err != nil {
		t.Fatalf("real financial settlement: %+v", err)
	}
	evidence, err := billing.NewSettlementEvidenceReader(fixture.database)
	if err != nil {
		t.Fatal(err)
	}
	addon, err := NewAccessorySettlementConsumer(fixture.database, evidence)
	if err != nil {
		t.Fatal(err)
	}
	generic := &accessoryChainGeneric{}
	consumer, err := fulfillment.NewRoutedConsumer(generic, addon)
	if err != nil {
		t.Fatal(err)
	}
	var event outbox.Event
	if err = fixture.database.GetScan(fixture.ctx, &event, `SELECT event_no,event_type,event_version,topic,
aggregate_type,aggregate_no,correlation_no,payload,metadata FROM plugin_idc_core_outbox_event
WHERE aggregate_no=? AND correlation_no=?`, checkout.OrderNo, settled.SettlementNo); err != nil {
		t.Fatal(err)
	}
	delivery, err := consumer.Consume(fixture.ctx, outbox.ClaimedEvent{Event: event})
	if err != nil {
		t.Fatalf("route real settlement to original server: %+v", err)
	}
	if delivery.Reused || delivery.FulfillmentCount != 0 || generic.calls != 0 {
		t.Fatalf("addon created generic fulfillment: %+v generic=%d", delivery, generic.calls)
	}
	deliveryReplay, err := consumer.Consume(fixture.ctx, outbox.ClaimedEvent{Event: event})
	if err != nil || !deliveryReplay.Reused || deliveryReplay.DeliveryNo != delivery.DeliveryNo || generic.calls != 0 {
		t.Fatalf("repeated event replayed execution: %+v / %+v", deliveryReplay, err)
	}
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_service_instance`, 1)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_inventory_reservation`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_commerce_fulfillment_unit`, 0)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_financial_settlement_delivery`, 1)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_server_operation_execution
WHERE service_no='SRV_checkout_original' AND action_code='traffic_add' AND status_code='ready'
AND original_runtime_status_code='suspended' AND expected_service_version=2
AND parameters='{"trafficGb":100}'::JSONB`, 1)
	fixture.assertCount(t, `SELECT COUNT(*) FROM plugin_idc_core_service_instance
WHERE service_no='SRV_checkout_original' AND runtime_status_code='suspended' AND version=2`, 1)
}

// prepareAccessoryOriginalServer extends the real financial fixture with an
// already existing manually imported server and an immutable addon quote.
func prepareAccessoryOriginalServer(t *testing.T, fixture accessoryCheckoutFixture) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve accessory server migrations")
	}
	for _, name := range []string{"056-server-operation-execution.sql", "057-server-accessory-operation.sql",
		"058-server-accessory-pricing.sql", "059-server-accessory-checkout.sql", "060-server-accessory-entitlement.sql",
		"109-server-accessory-settlement-index.sql"} {
		content, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "manifest", "sql", name))
		if err != nil {
			t.Fatal(err)
		}
		for index, statement := range dialect.SplitSQLStatements(string(content)) {
			if _, err = fixture.database.Exec(fixture.ctx, statement); err != nil {
				t.Fatalf("migration %s statement %d: %+v", name, index+1, err)
			}
		}
	}
	priceNo, err := deterministicReference("aps_", fixture.row.QuoteNo)
	if err != nil {
		t.Fatal(err)
	}
	if err = fixture.database.Transaction(fixture.ctx, func(ctx context.Context, _ gdb.TX) error {
		for _, statement := range dialect.SplitSQLStatements(accessoryOriginalServerSeedSQL) {
			if _, seedErr := fixture.database.Exec(ctx, statement); seedErr != nil {
				return seedErr
			}
		}
		_, insertErr := fixture.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_snapshot
(price_snapshot_no,service_no,customer_no,phase_code,source_quote_no,source_quote_item_no,product_no,version_no,
billing_rule_no,configuration_snapshot_no,currency_code,purchase_cycles,period_unit_code,period_count,renewal_mode_code,
capability_evidence_no,capability_evidence_digest,capability_evidence_observed_at,capability_evidence_expires_at,
tax_policy_no,price_includes_tax,tax_treatment_code,tax_category_code,rounding_scope_code,rounding_mode_code,
rounding_quantum_minor,component_count,subtotal_minor,net_amount_minor,total_amount_minor,pricing_rule_digest,content_digest)
SELECT ?,'SRV_checkout_original','CUS_checkout_test','addon',quote_no,quote_item_no,product_no,version_no,
billing_rule_no,configuration_snapshot_no,currency_code,purchase_cycles,period_unit_code,period_count,'provider_confirmed',
'CAP_checkout_test',repeat('a',64),CURRENT_TIMESTAMP,CURRENT_TIMESTAMP+INTERVAL '1 hour',
tax_policy_no,price_includes_tax,tax_treatment_code,tax_category_code,rounding_scope_code,rounding_mode_code,
rounding_quantum_minor,1,subtotal_minor,net_amount_minor,total_amount_minor,repeat('a',64),repeat('a',64)
FROM plugin_idc_core_commerce_quote_item_snapshot WHERE quote_item_no='aqi_checkout_test'`, priceNo)
		if insertErr != nil {
			return insertErr
		}
		_, insertErr = fixture.database.Exec(ctx, `INSERT INTO plugin_idc_core_service_price_component_snapshot
(component_snapshot_no,price_snapshot_no,service_no,line_number,component_type_code,charge_phase_code,
source_reference,quantity,unit_amount_minor,amount_minor,source_digest,content_digest)
VALUES ('PCM_checkout_test',?,'SRV_checkout_original',1,'base','setup','POL_checkout_test',1,1000,1000,
repeat('a',64),repeat('a',64))`, priceNo)
		return insertErr
	}); err != nil {
		t.Fatalf("prepare original server: %+v", err)
	}
}

// accessoryOriginalServerSeedSQL uses production foreign keys, including a
// deferred price-policy pointer, without weakening the real schema constraints.
const accessoryOriginalServerSeedSQL = `
INSERT INTO plugin_idc_core_catalog_resource_pool_binding
(binding_no,version_no,pool_no,region_code,line_code,priority,weight,new_order_enabled,status_code,
maintenance_mode_code,minimum_sellable_quantity,uncertain_sale_policy_code)
VALUES ('BIND_checkout_test','ver_checkout_test','POOL_checkout_test','','',1,100,TRUE,'active','normal',1,'stop_sale');
INSERT INTO plugin_idc_core_service_instance
(service_no,customer_no,source_code,product_no,version_no,billing_rule_no,configuration_snapshot_no,product_type_code,
currency_code,binding_no,pool_no,channel_no,entitlement_status_code,runtime_status_code,
period_unit_code,period_count,entitlement_started_at)
VALUES ('SRV_checkout_original','CUS_checkout_test','manual','prd_checkout_test','ver_checkout_test','BILL_checkout_test',
'CFG_checkout_test','cloud_server','CNY','BIND_checkout_test','POOL_checkout_test','CHN_checkout_test','active','suspended',
'once',1,CURRENT_TIMESTAMP-INTERVAL '1 day');
INSERT INTO plugin_idc_core_service_traffic_suspension(service_no,source_operation_no,suspended_at)
VALUES ('SRV_checkout_original','OP_suspension_test',CURRENT_TIMESTAMP-INTERVAL '1 hour');
INSERT INTO plugin_idc_core_server_accessory_price_policy(policy_no,accessory_kind,currency_code,current_version,enabled,stock_quantity)
VALUES ('POL_checkout_test','traffic','CNY',1,TRUE,1);
INSERT INTO plugin_idc_core_server_accessory_price_policy_version
(policy_no,policy_version,accessory_kind,currency_code,free_count_per_server,unit_amount_minor,
period_unit_code,period_count,content_digest,published_by,traffic_gb,name_zh_cn,name_en_us)
VALUES ('POL_checkout_test',1,'traffic','CNY',0,1000,'once',1,repeat('a',64),1,100,'测试流量包','Test traffic package');
INSERT INTO plugin_idc_core_traffic_package_product(policy_no,policy_version,product_no)
VALUES ('POL_checkout_test',1,'prd_checkout_test');
INSERT INTO plugin_idc_core_server_accessory_quote
(quote_no,customer_no,service_no,action_code,accessory_kind,currency_code,requested_quantity,existing_object_count,
reserved_create_count,free_count_per_server,charged_quantity,unit_amount_minor,total_amount_minor,period_unit_code,
period_count,policy_no,policy_version,idempotency_digest,request_digest,content_digest,expires_at,parameters,
subtotal_minor,net_amount_minor,tax_policy_no,tax_treatment_code,tax_category_code,
rounding_scope_code,rounding_mode_code,rounding_quantum_minor)
VALUES ('AQU_checkout_test','CUS_checkout_test','SRV_checkout_original','traffic_add','traffic','CNY',1,0,0,0,1,1000,1000,
'once',1,'POL_checkout_test',1,repeat('a',64),repeat('a',64),repeat('a',64),CURRENT_TIMESTAMP+INTERVAL '1 hour',
'{"trafficGb":100}',1000,1000,'TAX_checkout_test','exempt','cloud_service','line','half_up',1);`
