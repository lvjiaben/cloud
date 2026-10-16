// This file verifies trial activation dates against paid, frozen trial orders.
// Isolated provider evidence exercises the database hook without disabling any
// production guards; it does not claim to contact or provision a real supplier.
package commerce

import (
	"testing"
	"time"
)

func TestTrialPeriodStartsAtSuccessfulActivationPostgreSQL(t *testing.T) {
	ctx, database := prepareTrialUnitInvariantScenario(t)
	// The trial checkout above has genuinely settled at zero and consumed its
	// qualification. Seed only the external delivery boundary for that order.
	_, err := database.Exec(ctx, `
INSERT INTO plugin_idc_core_product_delivery_record(
delivery_no,order_no,order_item_no,customer_no,product_no,version_no,quantity,
strategy_code,policy_digest,interface_instance_no,plugin_id,plugin_release_version,
contract_version,declaration_digest,ongoing_management,content_digest,
renewal_mode_code,entitlement_owner_id)
SELECT 'delivery-trial-activation',i.order_no,i.order_item_no,o.customer_no,
i.product_no,i.version_no,q.quantity,'payment_confirmed_plugin',repeat('a',64),
'pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','trial-provider','1.0.0','v1',repeat('b',64),
TRUE,repeat('c',64),'local_entitlement','trial-provider'
FROM plugin_idc_core_commerce_order_item i
JOIN plugin_idc_core_commerce_order_item_quantity_snapshot q ON q.order_item_no=i.order_item_no
JOIN plugin_idc_core_commerce_order o ON o.order_no=i.order_no;
INSERT INTO plugin_idc_core_product_fulfillment_intent(
intent_no,settlement_no,order_no,order_item_no,customer_no,invoice_no,currency_code,
product_no,version_no,product_name_zh_cn,product_name_en_us,quantity,unit_sequence,
strategy_code,granularity_code,initial_status_code,status_code,configuration_snapshot_no,
interface_instance_no,interface_configuration_revision,plugin_id,plugin_release_version,
contract_version,release_digest,declaration_digest,provision_method,query_method,
policy_digest,evidence_digest,financially_settled_at,ongoing_management,
renewal_mode_code,entitlement_owner_id)
SELECT 'intent-trial-activation',s.settlement_no,i.order_no,i.order_item_no,s.customer_no,
s.invoice_no,s.currency_code,i.product_no,i.version_no,'试用服务器','Trial server',
q.quantity,1,'payment_confirmed_plugin','per_order_item','queued','queued',
'configuration-trial-activation','pfi_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',1,
'trial-provider','1.0.0','v1',repeat('a',64),repeat('b',64),'provision','query',
repeat('c',64),repeat('d',64),s.settled_at,TRUE,'local_entitlement','trial-provider'
FROM plugin_idc_core_commerce_order_item i
JOIN plugin_idc_core_commerce_order_item_quantity_snapshot q ON q.order_item_no=i.order_item_no
JOIN plugin_idc_core_financial_settlement s ON s.order_no=i.order_no;
UPDATE plugin_idc_core_product_fulfillment_intent
SET status_code='processing',version=version+1
WHERE intent_no='intent-trial-activation';`)
	if err != nil {
		t.Fatal(err)
	}
	var pending struct {
		Status  string
		Expires *time.Time
	}
	if err = database.GetScan(ctx, &pending, `SELECT status_code AS status,expires_at AS expires
FROM plugin_idc_core_service_entitlement`); err != nil {
		t.Fatal(err)
	}
	if pending.Status != "pending_activation" || pending.Expires != nil {
		t.Fatalf("waiting trial consumed time: %+v", pending)
	}
	// A deliberately old pending start models a slow provision. Activation must
	// replace it, using the immutable seven-day order rather than catalog edits.
	_, err = database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement
SET started_at=CURRENT_TIMESTAMP-INTERVAL '2 days';
UPDATE plugin_idc_core_catalog_billing_rule SET period_count=1
WHERE billing_rule_no='billing-commerce-service-trial';`)
	if err != nil {
		t.Fatal(err)
	}
	before := time.Now().Add(-time.Second)
	_, err = database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active'`)
	if err != nil {
		t.Fatal(err)
	}
	var active struct{ Start, Expires time.Time }
	if err = database.GetScan(ctx, &active, `SELECT started_at AS start,expires_at AS expires
FROM plugin_idc_core_service_entitlement`); err != nil {
		t.Fatal(err)
	}
	if active.Start.Before(before) || active.Start.After(time.Now().Add(time.Second)) || active.Expires.Sub(active.Start) != 7*24*time.Hour {
		t.Fatalf("activation did not preserve the frozen seven-day trial: %+v", active)
	}
	_, err = database.Exec(ctx, `UPDATE plugin_idc_core_service_entitlement SET status_code='active';
UPDATE plugin_idc_core_service_entitlement SET status_code='suspended';
UPDATE plugin_idc_core_service_entitlement SET status_code='active';`)
	if err != nil {
		t.Fatal(err)
	}
	var resumed struct{ Start, Expires time.Time }
	if err = database.GetScan(ctx, &resumed, `SELECT started_at AS start,expires_at AS expires
FROM plugin_idc_core_service_entitlement`); err != nil {
		t.Fatal(err)
	}
	if !active.Start.Equal(resumed.Start) || !active.Expires.Equal(resumed.Expires) {
		t.Fatalf("replay/resume reset trial dates: active=%+v resumed=%+v", active, resumed)
	}
}
