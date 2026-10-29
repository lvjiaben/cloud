// Native addon settlement checks explicit entitlement identities and never creates a legacy execution.

package serveroperation

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
)

const nativeAccessorySubject = "entitlement"

// isNativeAccessoryOrder uses either durable association so corrupt native rows cannot fall back to legacy processing.
func isNativeAccessoryOrder(ctx context.Context, database gdb.DB, orderNo string) (bool, error) {
	value, err := database.GetValue(ctx, `SELECT EXISTS(
 SELECT 1 FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND order_no=? AND operation_type_code='addon' AND subject_type_code='entitlement'
 UNION ALL SELECT 1 FROM plugin_idc_core_server_accessory_quote quote JOIN plugin_idc_core_commerce_order orders
 ON orders.tenant_id=quote.tenant_id AND orders.quote_no=quote.quote_no
 WHERE orders.tenant_id=0 AND orders.order_no=? AND quote.subject_type_code='entitlement')`, orderNo, orderNo)
	if err != nil {
		return false, err
	}
	return value.Bool(), nil
}

// nativeAccessoryTarget decodes only the immutable source-owned target, with no browser input.
func nativeAccessoryTarget(row *accessorySettlementRow) (*servercommercialcap.TargetSnapshot, error) {
	var target servercommercialcap.TargetSnapshot
	if json.Unmarshal([]byte(row.NativeTargetSnapshot), &target) != nil || target.EntitlementNo != row.SubjectNo || target.CustomerNo != row.CustomerNo ||
		target.EntitlementVersion != row.QuoteServiceVersion || target.ChannelNo != row.ChannelNo || target.PoolNo != row.PoolNo ||
		target.LocalResourceNo == "" || target.SnapshotNo == "" || target.ObservedAt.IsZero() || !target.ExpiresAt.After(target.ObservedAt) {
		return nil, ErrOperationConflict
	}
	return &target, nil
}

// nativeAccessorySettlementIntentDigest shares checkout's exact immutable native identity.
func nativeAccessorySettlementIntentDigest(row *accessorySettlementRow) (string, error) {
	return nativeAccessoryIntentDigest(row.OperationNo, row.QuoteNo, row.OrderNo, row.InvoiceNo, row.SubjectNo, row.ActionCode, row.Parameters, true, row.ExpectedServiceVersion)
}

// validateNativeAccessorySettlementReady leaves current resource admission to WithReadyOperation after financial proof commits.
func (s *accessorySettlementConsumer) validateNativeAccessorySettlementReady(ctx context.Context, row *accessorySettlementRow) error {
	if row.StatusCode != string(accessoryAwaitingSettlement) || row.SettlementNo != "" || row.FinanciallySettledAt != nil ||
		row.CurrentServiceVersion != row.ExpectedServiceVersion || row.QuoteServiceVersion != row.ExpectedServiceVersion ||
		row.CustomerSubjectVersion != row.CurrentCustomerVersion || row.CustomerStatus != "active" {
		return ErrOperationConflict
	}
	target, err := nativeAccessoryTarget(row)
	if err != nil {
		return err
	}
	if (row.EntitlementStatusCode != "active" && row.EntitlementStatusCode != "grace") || !nativeSnapshotAllows(*target, serveroperationresourcecap.Action(row.ActionCode)) {
		return ErrOperationConflict
	}
	busy, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_business_operation
 WHERE tenant_id=0 AND subject_type_code='entitlement' AND subject_no=? AND operation_no<>?
 AND status_code IN ('awaiting_financial_settlement','financially_settled','ready','executing','manual_review')
 UNION ALL SELECT 1 FROM plugin_idc_core_server_operation_execution WHERE tenant_id=0 AND operation_no=?)`, row.SubjectNo, row.OperationNo, row.OperationNo)
	if err != nil {
		return err
	}
	if busy.Bool() {
		return ErrOperationConflict
	}
	return nil
}

// validateNativeAccessorySettlementReplay replays only the committed transition; a later resource outcome is never reset.
func (s *accessorySettlementConsumer) validateNativeAccessorySettlementReplay(ctx context.Context, row *accessorySettlementRow, evidence string) error {
	exists, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_business_operation_event
 WHERE tenant_id=0 AND operation_no=? AND subject_type_code='entitlement' AND subject_no=?
 AND from_status_code='awaiting_financial_settlement' AND to_status_code='ready' AND reason_code=?
 AND actor_reference=? AND evidence_digest=? AND actor_type_code='system')
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_server_operation_execution WHERE tenant_id=0 AND operation_no=?)`, row.OperationNo, row.SubjectNo, accessorySettlementReason, row.SettlementNo, evidence, row.OperationNo)
	if err != nil {
		return err
	}
	if !exists.Bool() {
		return ErrOperationConflict
	}
	return nil
}

// appendNativeAccessoryEvent preserves the existing operation event stream with an explicit entitlement subject.
func appendNativeAccessoryEvent(ctx context.Context, database gdb.DB, operationNo, subjectNo string, sequence int64, from, to, reason, reference, evidence string) error {
	eventNo, err := deterministicReference("soe_", struct {
		OperationNo string
		Sequence    int
	}{operationNo, int(sequence)})
	if err != nil {
		return err
	}
	content, err := digestValue(struct {
		OperationNo, SubjectNo, From, To, Reason, Reference, Evidence string
		Sequence                                                      int64
	}{operationNo, subjectNo, from, to, reason, reference, evidence, sequence})
	if err != nil {
		return err
	}
	var fromStatus *string
	if from != "" {
		fromStatus = &from
	}
	_, err = database.Model(dao.BusinessOperationEvent.Table()).Ctx(ctx).Data(do.BusinessOperationEvent{TenantId: 0, EventNo: eventNo, OperationNo: operationNo, SubjectTypeCode: nativeAccessorySubject, SubjectNo: subjectNo, EventSequence: sequence, FromStatusCode: fromStatus, ToStatusCode: to, ReasonCode: reason, ActorTypeCode: "system", ActorReference: reference, EvidenceDigest: evidence, ContentDigest: content}).Insert()
	return err
}

const nativeAccessorySettlementEvidenceSQL = `SELECT
op.operation_no,op.order_no,op.invoice_no,op.quote_no,COALESCE(op.service_no,'') AS service_no,op.subject_type_code,op.subject_no,quote.native_target_snapshot::TEXT AS native_target_snapshot,op.customer_no,op.currency_code,
op.status_code,COALESCE(op.settlement_no,'') AS settlement_no,op.financially_settled_at,op.content_digest,
op.version,op.expected_subject_version AS expected_service_version,op.pool_no,quote.action_code,quote.accessory_kind,
quote.parameters::TEXT AS parameters,quote.content_digest AS quote_content_digest,
quote.expected_subject_version AS quote_service_version,quote.customer_subject_version,
service.version AS current_service_version,customer.version AS current_customer_version,
customer.status_code AS customer_status,COALESCE(op.channel_no,quote.native_target_snapshot->>'ChannelNo') AS channel_no,op.pool_no AS service_pool_no,
'' AS last_operation_no,service.status_code AS entitlement_status_code,
quote.native_target_snapshot->>'RuntimeStatus' AS runtime_status_code,'none' AS provider_task_status,
COALESCE((quote.native_target_snapshot->>'TrafficSuspended')::BOOLEAN,FALSE) AS traffic_suspended,
(op.service_no IS NULL AND quote.service_no IS NULL AND price.service_no IS NULL
 AND op.subject_type_code='entitlement' AND quote.subject_type_code='entitlement'
 AND price.subject_type_code='entitlement' AND price.subject_no=op.subject_no
 AND service.entitlement_owner_id='idc-resource' AND service.upstream_service_no=''
 AND op.financial_settlement_required AND op.resource_owner_code='idc-resource'
 AND ((op.execution_mode_code='unresolved' AND op.channel_no IS NULL)
   OR (op.execution_mode_code='provider_confirmed' AND op.channel_no=quote.native_target_snapshot->>'ChannelNo'))
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
 AND quote.subject_type_code=op.subject_type_code AND quote.subject_no=op.subject_no AND quote.customer_no=op.customer_no AND quote.currency_code=op.currency_code
JOIN plugin_idc_core_commerce_quote_snapshot commerce ON commerce.tenant_id=op.tenant_id AND commerce.quote_no=op.quote_no
 AND commerce.customer_no=op.customer_no AND commerce.currency_code=op.currency_code AND commerce.purpose_code='addon'
JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=op.tenant_id AND orders.order_no=op.order_no
JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=op.tenant_id AND invoice.invoice_no=op.invoice_no
JOIN plugin_idc_core_service_price_snapshot price ON price.tenant_id=op.tenant_id AND price.price_snapshot_no=op.price_snapshot_no
 AND price.source_quote_no=op.quote_no AND price.subject_no=op.subject_no AND price.customer_no=op.customer_no
 AND price.currency_code=op.currency_code AND price.phase_code='addon'
JOIN plugin_idc_core_service_entitlement service ON service.tenant_id=op.tenant_id AND service.entitlement_no=op.subject_no
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

// lockNativeAccessoryParent follows order/invoice -> customer -> entitlement ->
// operation, matching checkout replays and the shared native executor.
func lockNativeAccessoryParent(ctx context.Context, database gdb.DB, orderNo string) error {
	var parent *struct{ CustomerNo, SubjectNo string }
	if err := database.GetScan(ctx, &parent, `SELECT customer_no,subject_no FROM plugin_idc_core_business_operation WHERE tenant_id=0 AND order_no=? AND subject_type_code='entitlement' AND operation_type_code='addon'`, orderNo); err != nil {
		return err
	}
	if parent == nil {
		return ErrOperationConflict
	}
	if _, err := database.GetValue(ctx, `SELECT customer_no FROM plugin_idc_core_customer_subject WHERE tenant_id=0 AND customer_no=? FOR UPDATE`, parent.CustomerNo); err != nil {
		return err
	}
	if _, err := database.GetValue(ctx, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=? AND customer_no=? AND entitlement_owner_id='idc-resource' FOR UPDATE`, parent.SubjectNo, parent.CustomerNo); err != nil {
		return err
	}
	return nil
}
