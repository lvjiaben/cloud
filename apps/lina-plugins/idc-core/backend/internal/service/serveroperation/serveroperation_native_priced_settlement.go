// Native priced settlement routes durable IP/resize intent before generic
// fulfillment and reuses the financial receipt/event transition, never new purchase.

package serveroperation

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/billing"
	"lina-plugin-idc-core/backend/internal/service/fulfillment"
)

type serverOperationSettlementConsumer struct{ owner *accessorySettlementConsumer }

// NewServerOperationSettlementConsumer composes native priced and addon admission
// with the same startup-injected, read-only financial evidence owner.
func NewServerOperationSettlementConsumer(database gdb.DB, evidence billing.SettlementEvidenceReader) (fulfillment.AddonSettlementConsumer, error) {
	if nilServerOperationDependency(database) || nilServerOperationDependency(evidence) {
		return nil, ErrServiceUnavailable
	}
	return &serverOperationSettlementConsumer{owner: &accessorySettlementConsumer{database: database, evidence: evidence}}, nil
}

func nativePricedOrderOwned(ctx context.Context, database gdb.DB, orderNo string) (bool, error) {
	value, err := database.GetValue(ctx, `SELECT EXISTS(
 SELECT 1 FROM plugin_idc_core_commerce_order o JOIN plugin_idc_core_channel_service_quote q ON q.tenant_id=o.tenant_id AND q.quote_no=o.quote_no WHERE o.tenant_id=0 AND o.order_no=? AND q.source_code='native'
 UNION ALL SELECT 1 FROM plugin_idc_core_business_operation op JOIN plugin_idc_core_commerce_order o ON o.tenant_id=op.tenant_id AND o.order_no=op.order_no WHERE op.tenant_id=0 AND op.order_no=? AND op.subject_type_code='entitlement' AND op.operation_type_code IN ('resize','ip_change') AND o.source_code<>'channel')`, orderNo, orderNo)
	if err != nil {
		return false, err
	}
	return value.Bool(), nil
}

func (s *serverOperationSettlementConsumer) ConsumeAddonSettlement(ctx context.Context, input fulfillment.SettlementEvent) (*fulfillment.Delivery, bool, error) {
	owned, err := nativePricedOrderOwned(ctx, s.owner.database, input.OrderNo)
	if err != nil {
		return nil, true, err
	}
	if !owned {
		return s.owner.ConsumeAddonSettlement(ctx, input)
	}
	result, err := s.owner.consumeNativePricedSettlement(ctx, input)
	return result, true, err
}

func (s *accessorySettlementConsumer) consumeNativePricedSettlement(ctx context.Context, input fulfillment.SettlementEvent) (*fulfillment.Delivery, error) {
	if ctx == nil || ctx.Err() != nil || !validReference(input.OrderNo, 40) {
		return nil, ErrInvalidInput
	}
	var result *fulfillment.Delivery
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var order *entity.CommerceOrder
		if err := s.database.Model(dao.CommerceOrder.Table()).Ctx(ctx).Where(do.CommerceOrder{TenantId: 0, OrderNo: input.OrderNo}).LockUpdate().Scan(&order); err != nil {
			return err
		}
		proof, err := s.evidence.GetSettledOrderEvidence(ctx, input.OrderNo)
		if err != nil {
			return err
		}
		if order == nil || order.StatusCode != "paid" || !accessorySettlementEventMatches(input, proof) {
			return ErrOperationConflict
		}
		payload, err := json.Marshal(map[string]any{"settlementNo": input.SettlementNo, "orderNo": input.OrderNo, "invoiceNo": input.InvoiceNo, "customerNo": input.CustomerNo, "currencyCode": input.CurrencyCode, "basisCode": input.BasisCode, "totalAmountMinor": input.TotalAmountMinor, "settledAtUnixMs": input.SettledAtUnixMS})
		if err != nil {
			return err
		}
		metadata, err := json.Marshal(map[string]int{"sourceCount": input.SourceCount})
		if err != nil {
			return err
		}
		if _, err = s.database.GetValue(ctx, `SELECT invoice_no FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=? AND order_no=? FOR UPDATE`, input.InvoiceNo, input.OrderNo); err != nil {
			return err
		}
		if err = lockNativePricedSettlementParent(ctx, s.database, input.OrderNo); err != nil {
			return err
		}
		var rows []accessorySettlementRow
		if err = s.database.GetScan(ctx, &rows, nativePricedSettlementEvidenceSQL, input.OrderNo, input.SettlementNo, input.InvoiceNo, input.CustomerNo, input.CurrencyCode, input.TotalAmountMinor, input.EventNo, fulfillment.FinanciallySettledEventType, input.SettlementNo, string(payload), string(metadata)); err != nil {
			return err
		}
		if len(rows) != 1 || !rows[0].FrozenValid {
			return ErrOperationConflict
		}
		row := &rows[0]
		quote, err := (&nativeCommercialService{database: s.database}).loadNativePricedQuote(ctx, row.QuoteNo, row.CustomerNo, "")
		if err != nil {
			return err
		}
		if quote == nil {
			return ErrOperationConflict
		}
		content, err := nativeAccessoryIntentDigest(row.OperationNo, row.QuoteNo, row.OrderNo, row.InvoiceNo, row.SubjectNo, row.ActionCode, row.Parameters, true, row.ExpectedServiceVersion)
		if err != nil || content != row.ContentDigest {
			return ErrOperationConflict
		}
		evidence, err := digestValue(struct {
			Event                                                               fulfillment.SettlementEvent
			OperationDigest, QuoteDigest, FinancialSourceDigest, IdentityDigest string
		}{input, row.ContentDigest, row.QuoteContentDigest, proof.SourceSnapshotDigest, proof.IdentityEvidenceDigest})
		if err != nil {
			return err
		}
		var receipt *entity.FinancialSettlementDelivery
		if err = s.database.Model(dao.FinancialSettlementDelivery.Table()).Ctx(ctx).Where(do.FinancialSettlementDelivery{TenantId: 0, SettlementNo: input.SettlementNo}).Scan(&receipt); err != nil {
			return err
		}
		if receipt != nil {
			if receipt.EventNo != input.EventNo || receipt.OrderNo != input.OrderNo || receipt.InvoiceNo != input.InvoiceNo || receipt.CustomerNo != input.CustomerNo || receipt.CurrencyCode != input.CurrencyCode || receipt.EvidenceDigest != evidence || row.SettlementNo != input.SettlementNo || row.FinanciallySettledAt == nil || !row.FinanciallySettledAt.Equal(proof.SettledAt) {
				return ErrOperationConflict
			}
			if err = s.validateAccessorySettlementReplay(ctx, row, receipt); err != nil {
				return err
			}
			result = accessorySettlementDelivery(input, receipt.DeliveryNo, true)
			return nil
		}
		if err = s.validateNativeAccessorySettlementReady(ctx, row); err != nil {
			return err
		}
		now, err := databaseNow(ctx, s.database)
		if err != nil {
			return err
		}
		result, err = s.persistAccessorySettlement(ctx, input, row, evidence, proof.SettledAt, now)
		return err
	})
	return result, err
}

// Lock the actual parent before the operation, matching native checkout,
// execution and recovery after the order/invoice finance locks.
func lockNativePricedSettlementParent(ctx context.Context, database gdb.DB, orderNo string) error {
	var parent *struct{ CustomerNo, SubjectNo string }
	if err := database.GetScan(ctx, &parent, `SELECT customer_no,subject_no FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND order_no=? AND subject_type_code='entitlement' AND operation_type_code IN ('resize','ip_change')`, orderNo); err != nil {
		return err
	}
	if parent == nil {
		return ErrOperationConflict
	}
	if _, err := database.GetValue(ctx, `SELECT customer_no FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND customer_no=? FOR UPDATE`, parent.CustomerNo); err != nil {
		return err
	}
	_, err := database.GetValue(ctx, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND entitlement_owner_id='idc-resource' FOR UPDATE`, parent.SubjectNo, parent.CustomerNo)
	return err
}

// This bounded set projection checks all frozen commercial owners and the actual
// financially_settled outbox envelope before advancing the original intent.
const nativePricedSettlementEvidenceSQL = `SELECT
op.operation_no,op.order_no,op.invoice_no,op.quote_no,COALESCE(op.service_no,'') AS service_no,op.subject_type_code,op.subject_no,quote.native_target_snapshot::TEXT AS native_target_snapshot,op.customer_no,op.currency_code,
op.status_code,COALESCE(op.settlement_no,'') AS settlement_no,op.financially_settled_at,op.content_digest,
op.version,op.expected_subject_version AS expected_service_version,op.pool_no,quote.action_code,'' AS accessory_kind,
quote.provider_parameters::TEXT AS parameters,quote.content_digest AS quote_content_digest,
quote.expected_service_version AS quote_service_version,quote.customer_subject_version,
service.version AS current_service_version,customer.version AS current_customer_version,
customer.status_code AS customer_status,COALESCE(op.channel_no,quote.native_target_snapshot->>'ChannelNo') AS channel_no,op.pool_no AS service_pool_no,
'' AS last_operation_no,service.status_code AS entitlement_status_code,
quote.native_target_snapshot->>'RuntimeStatus' AS runtime_status_code,'none' AS provider_task_status,
COALESCE((quote.native_target_snapshot->>'TrafficSuspended')::BOOLEAN,FALSE) AS traffic_suspended,
(op.service_no IS NULL AND price.service_no IS NULL
 AND op.subject_type_code='entitlement' AND quote.source_code='native'
 AND price.subject_type_code='entitlement' AND price.subject_no=op.subject_no
 AND service.entitlement_owner_id='idc-resource' AND service.upstream_service_no=''
 AND op.financial_settlement_required AND op.resource_owner_code='idc-resource'
 AND ((op.execution_mode_code='unresolved' AND op.channel_no IS NULL)
   OR (op.execution_mode_code='provider_confirmed' AND op.channel_no=quote.native_target_snapshot->>'ChannelNo'))
 AND orders.item_count=1 AND orders.risk_decision_code='approved'
 AND orders.customer_no=op.customer_no AND orders.currency_code=op.currency_code AND orders.quote_no=op.quote_no
 AND orders.total_amount_minor=quote.amount_minor AND orders.subtotal_minor=quote.subtotal_minor
 AND orders.discount_minor=0 AND orders.net_amount_minor=quote.net_amount_minor AND orders.tax_amount_minor=quote.tax_amount_minor
 AND invoice.status_code IN ('paid','credit_settled') AND invoice.item_count=1 AND invoice.customer_no=op.customer_no
 AND invoice.currency_code=op.currency_code AND invoice.order_no=op.order_no
 AND invoice.source_snapshot_digest=quote.content_digest AND invoice.total_amount_minor=quote.amount_minor
 AND invoice.subtotal_minor=quote.subtotal_minor AND invoice.discount_minor=0
 AND invoice.net_amount_minor=quote.net_amount_minor AND invoice.tax_amount_minor=quote.tax_amount_minor
 AND commerce.customer_subject_version=quote.customer_subject_version AND commerce.item_count=1
 AND commerce.total_amount_minor=quote.amount_minor AND commerce.subtotal_minor=quote.subtotal_minor
 AND commerce.discount_minor=0 AND commerce.net_amount_minor=quote.net_amount_minor AND commerce.tax_amount_minor=quote.tax_amount_minor
 AND price.total_amount_minor=quote.amount_minor AND price.subtotal_minor=quote.subtotal_minor
 AND price.discount_minor=0 AND price.net_amount_minor=quote.net_amount_minor AND price.tax_amount_minor=quote.tax_amount_minor
 AND (SELECT count(*) FROM plugin_idc_core_commerce_order_item item WHERE item.tenant_id=0 AND item.order_no=op.order_no)=1
 AND (SELECT count(*) FROM plugin_idc_core_billing_invoice_item item WHERE item.tenant_id=0 AND item.invoice_no=op.invoice_no)=1
 AND EXISTS(SELECT 1 FROM plugin_idc_core_commerce_order_item item
 JOIN plugin_idc_core_billing_invoice_item billed ON billed.tenant_id=item.tenant_id AND billed.order_item_no=item.order_item_no
 WHERE item.tenant_id=0 AND item.order_no=op.order_no AND item.quote_no=op.quote_no
 AND billed.invoice_no=op.invoice_no AND billed.order_no=op.order_no AND billed.customer_no=op.customer_no
 AND item.quote_item_no=price.source_quote_item_no AND item.product_no=price.product_no AND item.version_no=price.version_no
 AND item.currency_code=op.currency_code AND billed.currency_code=op.currency_code
 AND item.total_amount_minor=quote.amount_minor AND billed.total_amount_minor=quote.amount_minor)
) AS frozen_valid
FROM plugin_idc_core_business_operation op
JOIN plugin_idc_core_channel_service_quote quote ON quote.tenant_id=op.tenant_id AND quote.quote_no=op.quote_no
 AND quote.source_code='native' AND quote.service_no=op.subject_no AND quote.service_owner_customer_no=op.customer_no AND quote.currency_code=op.currency_code
JOIN plugin_idc_core_commerce_quote_snapshot commerce ON commerce.tenant_id=op.tenant_id AND commerce.quote_no=op.quote_no
 AND commerce.customer_no=op.customer_no AND commerce.currency_code=op.currency_code AND commerce.purpose_code=CASE WHEN quote.action_code='resize' THEN 'change' ELSE 'ip_change' END
JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=op.tenant_id AND orders.order_no=op.order_no
JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=op.tenant_id AND invoice.invoice_no=op.invoice_no
JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=op.tenant_id AND price.price_snapshot_no=op.price_snapshot_no
 AND price.source_quote_no=op.quote_no AND price.subject_no=op.subject_no AND price.customer_no=op.customer_no
 AND price.currency_code=op.currency_code AND price.phase_code=CASE WHEN quote.action_code='resize' THEN 'resize_charge' ELSE 'ip_change' END
JOIN plugin_idc_core_service_entitlement service ON service.tenant_id=op.tenant_id AND service.entitlement_no=op.subject_no
 AND service.customer_no=op.customer_no AND service.currency_code=op.currency_code
JOIN plugin_idc_core_customer_subject customer ON customer.tenant_id=op.tenant_id AND customer.customer_no=op.customer_no
WHERE op.tenant_id=0 AND op.order_no=? AND op.operation_type_code=quote.action_code AND op.operation_type_code IN ('resize','ip_change')
 AND EXISTS(SELECT 1 FROM plugin_idc_core_financial_settlement settled
 WHERE settled.tenant_id=0 AND settled.settlement_no=? AND settled.order_no=op.order_no
 AND settled.invoice_no=? AND settled.invoice_no=op.invoice_no AND settled.customer_no=? AND settled.customer_no=op.customer_no
 AND settled.currency_code=? AND settled.currency_code=op.currency_code
 AND settled.total_amount_minor=? AND settled.total_amount_minor=quote.amount_minor)
 AND EXISTS(SELECT 1 FROM plugin_idc_core_outbox_event event
 WHERE event.tenant_id=0 AND event.event_no=? AND event.event_type=? AND event.event_version=1
 AND event.topic=event.event_type AND event.aggregate_type='commerce_order' AND event.aggregate_no=op.order_no
 AND event.correlation_no=?
 AND event.payload=?::JSONB AND event.metadata=?::JSONB)
LIMIT 2 FOR UPDATE OF op,service,customer`
