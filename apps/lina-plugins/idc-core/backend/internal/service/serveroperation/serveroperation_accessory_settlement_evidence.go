// This file binds immutable accessory checkout facts to their original server.

package serveroperation

import (
	"context"
	"encoding/json"
	"time"

	"lina-plugin-idc-core/backend/internal/service/fulfillment"
)

// accessorySettlementRow is a private bounded projection, not a public data model.
type accessorySettlementRow struct {
	OperationNo, OrderNo, InvoiceNo, QuoteNo, ServiceNo, CustomerNo, CurrencyCode string
	ActionCode, AccessoryKind, Parameters, ContentDigest, QuoteContentDigest      string
	StatusCode, SettlementNo, ChannelNo, PoolNo, ServicePoolNo, LastOperationNo   string
	EntitlementStatusCode, RuntimeStatusCode, ProviderTaskStatus, CustomerStatus  string
	Version, ExpectedServiceVersion, CurrentServiceVersion, QuoteServiceVersion   int64
	CustomerSubjectVersion, CurrentCustomerVersion                                int64
	FinanciallySettledAt                                                          *time.Time
	TrafficSuspended, FrozenValid                                                 bool
	SubjectTypeCode, SubjectNo, NativeTargetSnapshot                              string
}

// accessorySettlementClassificationSQL retains both independent durable signals.
// The order predicate must not depend on a possibly broken invoice/quote join.
const accessorySettlementClassificationSQL = `SELECT EXISTS (
SELECT 1 FROM plugin_idc_core_commerce_quote_snapshot WHERE tenant_id=0 AND quote_no=? AND purpose_code='addon'
UNION ALL SELECT 1 FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND order_no=? AND operation_type_code='addon')`

// lockAccessorySettlement locks the original invoice and verifies frozen checkout associations.
func (s *accessorySettlementConsumer) lockAccessorySettlement(ctx context.Context, input fulfillment.SettlementEvent) (*accessorySettlementRow, error) {
	payload, err := json.Marshal(map[string]any{
		"settlementNo": input.SettlementNo, "orderNo": input.OrderNo, "invoiceNo": input.InvoiceNo,
		"customerNo": input.CustomerNo, "currencyCode": input.CurrencyCode, "basisCode": input.BasisCode,
		"totalAmountMinor": input.TotalAmountMinor, "settledAtUnixMs": input.SettledAtUnixMS,
	})
	if err != nil {
		return nil, err
	}
	metadata, err := json.Marshal(map[string]int{"sourceCount": input.SourceCount})
	if err != nil {
		return nil, err
	}
	// Lock invoice before service/customer, matching the billing owner. A paid
	// invoice cannot be cancelled or transformed during local materialization.
	if _, err = s.database.GetValue(ctx, `SELECT invoice_no FROM plugin_idc_core_billing_invoice
WHERE tenant_id=0 AND invoice_no=? AND order_no=? FOR UPDATE`, input.InvoiceNo, input.OrderNo); err != nil {
		return nil, err
	}
	native, err := isNativeAccessoryOrder(ctx, s.database, input.OrderNo)
	if err != nil {
		return nil, err
	}
	query := accessorySettlementEvidenceSQL
	if native {
		if err = lockNativeAccessoryParent(ctx, s.database, input.OrderNo); err != nil {
			return nil, err
		}
		query = nativeAccessorySettlementEvidenceSQL
	}
	var rows []accessorySettlementRow
	err = s.database.GetScan(ctx, &rows, query, input.OrderNo,
		input.SettlementNo, input.InvoiceNo, input.CustomerNo, input.CurrencyCode, input.TotalAmountMinor,
		input.EventNo, fulfillment.FinanciallySettledEventType, input.SettlementNo, string(payload), string(metadata))
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, ErrOperationConflict
	}
	return &rows[0], nil
}

// The query is one bounded projection, not a per-item lookup. Snapshot joins
// deliberately never use product names or current catalog/provider settings.
const accessorySettlementEvidenceSQL = `SELECT
op.operation_no,op.order_no,op.invoice_no,op.quote_no,op.service_no,op.customer_no,op.currency_code,
op.status_code,COALESCE(op.settlement_no,'') AS settlement_no,op.financially_settled_at,op.content_digest,
op.version,op.expected_service_version,op.pool_no,quote.action_code,quote.accessory_kind,
quote.parameters::TEXT AS parameters,quote.content_digest AS quote_content_digest,
quote.expected_service_version AS quote_service_version,quote.customer_subject_version,
service.version AS current_service_version,customer.version AS current_customer_version,
customer.status_code AS customer_status,service.channel_no,service.pool_no AS service_pool_no,
COALESCE(service.last_business_operation_no,'') AS last_operation_no,
service.entitlement_status_code,service.runtime_status_code,
service.provider_task_status_code AS provider_task_status,
EXISTS(SELECT 1 FROM plugin_idc_core_service_traffic_suspension suspension
 WHERE suspension.tenant_id=0 AND suspension.service_no=service.service_no AND suspension.active) AS traffic_suspended,
(op.financial_settlement_required AND op.resource_owner_code='idc-resource' AND op.execution_mode_code='unresolved'
 AND orders.item_count=1 AND orders.risk_decision_code='approved'
 AND orders.customer_no=op.customer_no AND orders.currency_code=op.currency_code AND orders.quote_no=op.quote_no
 AND orders.total_amount_minor=quote.total_amount_minor AND orders.subtotal_minor=quote.subtotal_minor
 AND orders.discount_minor=0 AND orders.net_amount_minor=quote.net_amount_minor AND orders.tax_amount_minor=quote.tax_amount_minor
 AND invoice.status_code IN ('paid','credit_settled') AND invoice.item_count=1 AND invoice.customer_no=op.customer_no
 AND invoice.currency_code=op.currency_code AND invoice.order_no=op.order_no
 AND invoice.source_snapshot_digest=quote.content_digest AND invoice.total_amount_minor=quote.total_amount_minor
 AND invoice.subtotal_minor=quote.subtotal_minor AND invoice.discount_minor=0
 AND invoice.net_amount_minor=quote.net_amount_minor AND invoice.tax_amount_minor=quote.tax_amount_minor
 AND commerce.customer_subject_version=quote.customer_subject_version AND commerce.item_count=1
 AND commerce.total_amount_minor=quote.total_amount_minor AND commerce.subtotal_minor=quote.subtotal_minor
 AND commerce.discount_minor=0 AND commerce.net_amount_minor=quote.net_amount_minor AND commerce.tax_amount_minor=quote.tax_amount_minor
 AND price.total_amount_minor=quote.total_amount_minor AND price.subtotal_minor=quote.subtotal_minor
 AND price.discount_minor=0 AND price.net_amount_minor=quote.net_amount_minor AND price.tax_amount_minor=quote.tax_amount_minor
 AND (SELECT count(*) FROM plugin_idc_core_commerce_order_item item WHERE item.tenant_id=0 AND item.order_no=op.order_no)=1
 AND (SELECT count(*) FROM plugin_idc_core_billing_invoice_item item WHERE item.tenant_id=0 AND item.invoice_no=op.invoice_no)=1
 AND EXISTS(SELECT 1 FROM plugin_idc_core_commerce_order_item item
 JOIN plugin_idc_core_billing_invoice_item billed ON billed.tenant_id=item.tenant_id AND billed.order_item_no=item.order_item_no
 WHERE item.tenant_id=0 AND item.order_no=op.order_no AND item.quote_no=op.quote_no
 AND billed.invoice_no=op.invoice_no AND billed.order_no=op.order_no AND billed.customer_no=op.customer_no
 AND item.quote_item_no=price.source_quote_item_no AND item.product_no=price.product_no AND item.version_no=price.version_no
 AND item.currency_code=op.currency_code AND billed.currency_code=op.currency_code
 AND item.total_amount_minor=quote.total_amount_minor AND billed.total_amount_minor=quote.total_amount_minor)
) AS frozen_valid
FROM plugin_idc_core_business_operation op
JOIN plugin_idc_core_server_accessory_quote quote ON quote.tenant_id=op.tenant_id AND quote.quote_no=op.quote_no
 AND quote.service_no=op.service_no AND quote.customer_no=op.customer_no AND quote.currency_code=op.currency_code
JOIN plugin_idc_core_commerce_quote_snapshot commerce ON commerce.tenant_id=op.tenant_id AND commerce.quote_no=op.quote_no
 AND commerce.customer_no=op.customer_no AND commerce.currency_code=op.currency_code AND commerce.purpose_code='addon'
JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=op.tenant_id AND orders.order_no=op.order_no
JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=op.tenant_id AND invoice.invoice_no=op.invoice_no
JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=op.tenant_id AND price.price_snapshot_no=op.price_snapshot_no
 AND price.source_quote_no=op.quote_no AND price.service_no=op.service_no AND price.customer_no=op.customer_no
 AND price.currency_code=op.currency_code AND price.phase_code='addon'
JOIN plugin_idc_core_service_instance service ON service.tenant_id=op.tenant_id AND service.service_no=op.service_no
 AND service.customer_no=op.customer_no AND service.currency_code=op.currency_code
JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=op.tenant_id AND customer.customer_no=op.customer_no
WHERE op.tenant_id=0 AND op.order_no=? AND op.operation_type_code='addon'
 AND EXISTS(SELECT 1 FROM plugin_idc_core_financial_settlement settled
 WHERE settled.tenant_id=0 AND settled.settlement_no=? AND settled.order_no=op.order_no
 AND settled.invoice_no=? AND settled.invoice_no=op.invoice_no AND settled.customer_no=? AND settled.customer_no=op.customer_no
 AND settled.currency_code=? AND settled.currency_code=op.currency_code
 AND settled.total_amount_minor=? AND settled.total_amount_minor=quote.total_amount_minor)
 AND EXISTS(SELECT 1 FROM plugin_idc_core_outbox_event event
 WHERE event.tenant_id=0 AND event.event_no=? AND event.event_type=? AND event.event_version=1
 AND event.topic=event.event_type AND event.aggregate_type='commerce_order' AND event.aggregate_no=op.order_no
 AND event.correlation_no=?
 AND event.payload=?::JSONB AND event.metadata=?::JSONB)
LIMIT 2 FOR UPDATE OF op,service,customer`
