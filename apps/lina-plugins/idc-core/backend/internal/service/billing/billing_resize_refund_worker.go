// This file bridges system-required compensation into the unified refund workflow.

package billing

import (
	"context"
	"crypto/rand"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
)

const maximumResizeRefundBatch = 100

// ResizeRefundWorker materializes and reconciles paid resize-failure handoffs
// through the unified refund workflow.
type ResizeRefundWorker interface {
	// Run processes each selected handoff once per bounded call. Database leases
	// and unique digests make concurrent primary-node failover replay safe; an
	// ambiguous payment result is reconciled or moved to manual review, never retried as a new refund.
	Run(context.Context, int) (int, error)
}

type resizeRefundWorker struct {
	database gdb.DB
}

type resizeRefundHandoffRow struct {
	ID             int64  `orm:"id"`
	HandoffNo      string `orm:"handoff_no"`
	OperationNo    string `orm:"operation_no"`
	OrderNo        string `orm:"order_no"`
	InvoiceNo      string `orm:"invoice_no"`
	CustomerNo     string `orm:"customer_no"`
	CurrencyCode   string `orm:"currency_code"`
	AmountMinor    int64  `orm:"amount_minor"`
	StatusCode     string `orm:"status_code"`
	EvidenceDigest string `orm:"evidence_digest"`
	RefundNo       string `orm:"refund_no"`
	LastErrorCode  string `orm:"last_error_code"`
}

// NewResizeRefundWorker constructs the core-owned bridge. It does not call a
// payment channel itself; it only creates the same durable commands used by the
// ordinary refund workflow.
func NewResizeRefundWorker(database gdb.DB) (ResizeRefundWorker, error) {
	if isNilFinanceDirectoryDependency(database) {
		return nil, gerror.New("resize refund worker requires database")
	}
	return &resizeRefundWorker{database: database}, nil
}

func (w *resizeRefundWorker) Run(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumResizeRefundBatch {
		return 0, gerror.New("resize refund worker input is invalid")
	}
	return w.runResizeRefunds(ctx, limit)
}

func (w *resizeRefundWorker) runResizeRefunds(ctx context.Context, limit int) (int, error) {
	processed := 0
	lastID := int64(0)
	for processed < limit {
		hasWork := false
		selectedID := int64(0)
		err := w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
			var handoff *resizeRefundHandoffRow
			if err := w.database.GetScan(ctx, &handoff, `
SELECT id,handoff_no,operation_no,order_no,invoice_no,customer_no,currency_code,
       amount_minor,status_code,evidence_digest,COALESCE(refund_no,'') AS refund_no,last_error_code
FROM plugin_idc_core_server_resize_refund_handoff
WHERE tenant_id=0 AND status_code IN ('ready','processing') AND id>?
ORDER BY created_at,id
LIMIT 1 FOR UPDATE SKIP LOCKED`, lastID); err != nil {
				return err
			}
			if handoff == nil {
				return nil
			}
			hasWork = true
			selectedID = handoff.ID
			if handoff.StatusCode == "processing" {
				return w.reconcileResizeRefund(ctx, handoff)
			}
			return w.materializeResizeRefund(ctx, handoff)
		})
		if err != nil {
			return processed, gerror.Wrap(err, "process resize refund handoff")
		}
		if !hasWork {
			break
		}
		lastID = selectedID
		processed++
	}
	return processed, nil
}

func (w *resizeRefundWorker) approveSystemRefund(
	ctx context.Context,
	writer *onlineRefundService,
	refund *OnlineRefund,
	reasonCode string,
) error {
	if refund == nil || refund.Status != "pending_review" {
		return gerror.New("resize refund review state is invalid")
	}
	now, err := refundDatabaseNow(ctx, w.database)
	if err != nil {
		return err
	}
	updated, err := w.database.Exec(ctx, `
UPDATE plugin_idc_core_refund_request
SET status_code='approved',approved_at=?,version=version+1,updated_at=?
WHERE tenant_id=0 AND refund_no=? AND status_code='pending_review' AND version=1`,
		now, now, refund.RefundNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(updated); err != nil {
		return err
	}
	var sources []struct {
		SourceNo               string `orm:"refund_source_no"`
		PaymentSourceType      string `orm:"payment_source_type_code"`
		PaymentSourceReference string `orm:"payment_source_reference"`
		ExecutionOwner         string `orm:"execution_owner_code"`
		AmountMinor            int64  `orm:"amount_minor"`
	}
	if err = w.database.GetScan(ctx, &sources, `
SELECT refund_source_no,payment_source_type_code,payment_source_reference,
       execution_owner_code,amount_minor
FROM plugin_idc_core_refund_source_allocation
WHERE tenant_id=0 AND refund_no=?
ORDER BY id`, refund.RefundNo); err != nil {
		return err
	}
	if len(sources) == 0 {
		return gerror.New("resize refund funding source is unavailable")
	}
	commands := make([]refundExecutionCommandInsert, 0, len(sources))
	for _, source := range sources {
		command, commandErr := writer.newRefundExecutionCommand(
			refund.RefundNo, source.SourceNo, refund.CurrencyCode,
			RefundSourceAllocation{
				PaymentSourceType:      source.PaymentSourceType,
				PaymentSourceReference: source.PaymentSourceReference,
				ExecutionOwner:         source.ExecutionOwner,
				AmountMinor:            source.AmountMinor,
			},
			"original_channel", now,
		)
		if commandErr != nil {
			return commandErr
		}
		commands = append(commands, command)
	}
	if err = writer.insertRefundExecutionCommands(ctx, commands); err != nil {
		return err
	}
	if err = writer.insertRefundDecision(
		ctx, refund.RefundNo, 1, "automatic_approve", "pending_review", "approved",
		"system", 0, reasonCode, "", refund.ImpactDigest, now,
	); err != nil {
		return err
	}
	refund.Status = "approved"
	refund.Version = 2
	refund.ApprovedAt = &now
	return nil
}

func (w *resizeRefundWorker) materializeResizeRefund(
	ctx context.Context,
	handoff *resizeRefundHandoffRow,
) error {
	writer := &onlineRefundService{database: w.database, random: rand.Reader}
	idempotencyDigest := handoff.EvidenceDigest
	if replay, err := writer.loadRefundReplay(ctx, handoff.CustomerNo, idempotencyDigest); err != nil {
		return err
	} else if replay != nil {
		return w.markResizeRefundProcessing(ctx, handoff, replay.RefundNo)
	}
	preview, err := w.buildResizeRefundPreview(ctx, writer, handoff)
	if err != nil {
		return w.markResizeRefundManual(ctx, handoff, "server_resize_refund_facts_changed")
	}
	preview.ApprovalMode = "manual"
	preview.HighAmount = true
	requestDigest, err := contentDigest(struct {
		HandoffNo     string `json:"handoffNo"`
		PreviewDigest string `json:"previewDigest"`
	}{handoff.HandoffNo, preview.ImpactDigest})
	if err != nil {
		return err
	}
	created, err := writer.insertRefundRequest(
		ctx, 1, preview, RefundReasonServiceIssue, "",
		idempotencyDigest, requestDigest,
	)
	if err != nil {
		return err
	}
	if err = w.approveSystemRefund(ctx, writer, created, "server_resize_provider_failed"); err != nil {
		return err
	}
	return w.markResizeRefundProcessing(ctx, handoff, created.RefundNo)
}

func (w *resizeRefundWorker) buildResizeRefundPreview(
	ctx context.Context,
	writer *onlineRefundService,
	handoff *resizeRefundHandoffRow,
) (*RefundPreview, error) {
	invoice, err := writer.loadRefundInvoice(ctx, handoff.CustomerNo, handoff.OrderNo, true)
	if err != nil {
		return nil, err
	}
	if validationErr := validateRefundInvoiceEligibility(invoice); validationErr != nil {
		return nil, validationErr
	}
	if invoice.InvoiceNo != handoff.InvoiceNo || invoice.CurrencyCode != handoff.CurrencyCode ||
		invoice.RefundableAmountMinor != handoff.AmountMinor {
		return nil, gerror.New("resize refund invoice facts changed")
	}
	var itemRefs []struct {
		InvoiceItemNo string `orm:"invoice_item_no"`
	}
	if err = w.database.GetScan(ctx, &itemRefs, `
SELECT invoice_item_no FROM plugin_idc_core_billing_invoice_item
WHERE tenant_id=0 AND invoice_no=? ORDER BY line_number,id LIMIT 2`, handoff.InvoiceNo); err != nil {
		return nil, err
	}
	if len(itemRefs) != 1 {
		return nil, gerror.New("resize refund invoice item boundary is invalid")
	}
	itemNos := []string{itemRefs[0].InvoiceItemNo}
	items, err := writer.loadRefundInvoiceItems(ctx, handoff.InvoiceNo, itemNos)
	if err != nil {
		return nil, err
	}
	if len(items) != 1 || items[0].RemainingAmountMinor != handoff.AmountMinor {
		return nil, gerror.New("resize refund amount is no longer available")
	}
	item := items[0]
	selection := RefundSelectionInput{
		InvoiceItemNo: item.InvoiceItemNo,
		AmountMinor:   handoff.AmountMinor,
	}
	previewItem := RefundPreviewItem{
		InvoiceItemNo:         item.InvoiceItemNo,
		OrderItemNo:           item.OrderItemNo,
		AmountMinor:           handoff.AmountMinor,
		RefundableAmountMinor: item.RemainingAmountMinor,
		DescriptionZhCN:       item.DescriptionZhCN,
		DescriptionEnUS:       item.DescriptionEnUS,
	}
	if invoice.StatusCode == "credit_settled" {
		plan, planErr := writer.previewCreditInvoiceRefund(ctx, invoice, handoff.AmountMinor)
		if planErr != nil {
			return nil, planErr
		}
		allocated, allocationErr := allocateCreditRefundSources([]RefundSelectionInput{selection}, plan)
		if allocationErr != nil {
			return nil, allocationErr
		}
		previewItem.Sources = allocated[0]
	} else {
		sources, sourceErr := writer.loadRefundPaymentSources(ctx, handoff.InvoiceNo)
		if sourceErr != nil {
			return nil, sourceErr
		}
		remaining := make(map[string]int64, len(sources))
		for _, source := range sources {
			remaining[source.AllocationNo] = source.RemainingAmountMinor
		}
		previewItem.Sources, err = allocateRefundSources(
			handoff.AmountMinor, item.InvoiceItemNo, sources, remaining,
		)
		if err != nil {
			return nil, err
		}
	}
	preview := &RefundPreview{
		CustomerNo: handoff.CustomerNo, OrderNo: handoff.OrderNo,
		InvoiceNo: handoff.InvoiceNo, CurrencyCode: handoff.CurrencyCode,
		AmountMinor: handoff.AmountMinor, Items: []RefundPreviewItem{previewItem},
	}
	for _, source := range previewItem.Sources {
		preview.HasFundAnomaly = preview.HasFundAnomaly || source.ExecutionOwner == "manual"
	}
	previewItem.ImpactDigest, err = contentDigest(struct {
		InvoiceItemNo string                   `json:"invoiceItemNo"`
		OrderItemNo   string                   `json:"orderItemNo"`
		AmountMinor   int64                    `json:"amountMinor"`
		Sources       []RefundSourceAllocation `json:"sources"`
	}{item.InvoiceItemNo, item.OrderItemNo, handoff.AmountMinor, previewItem.Sources})
	if err != nil {
		return nil, err
	}
	preview.Items[0] = previewItem
	preview.ImpactDigest, err = contentDigest(struct {
		HandoffNo    string              `json:"handoffNo"`
		CustomerNo   string              `json:"customerNo"`
		OrderNo      string              `json:"orderNo"`
		InvoiceNo    string              `json:"invoiceNo"`
		CurrencyCode string              `json:"currencyCode"`
		AmountMinor  int64               `json:"amountMinor"`
		Items        []RefundPreviewItem `json:"items"`
	}{handoff.HandoffNo, handoff.CustomerNo, handoff.OrderNo, handoff.InvoiceNo,
		handoff.CurrencyCode, handoff.AmountMinor, preview.Items})
	return preview, err
}

func (w *resizeRefundWorker) markResizeRefundProcessing(
	ctx context.Context,
	handoff *resizeRefundHandoffRow,
	refundNo string,
) error {
	result, err := w.database.Exec(ctx, `
UPDATE plugin_idc_core_server_resize_refund_handoff
SET status_code='processing',refund_no=?,last_error_code='',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND handoff_no=? AND status_code='ready'`, refundNo, handoff.HandoffNo)
	if err != nil {
		return err
	}
	return requireSingleRefundExecutionUpdate(result)
}

func (w *resizeRefundWorker) reconcileResizeRefund(
	ctx context.Context,
	handoff *resizeRefundHandoffRow,
) error {
	if strings.TrimSpace(handoff.RefundNo) == "" {
		return w.markResizeRefundManual(ctx, handoff, "server_resize_refund_reference_missing")
	}
	value, err := w.database.GetValue(ctx, `
SELECT status_code FROM plugin_idc_core_refund_request
WHERE tenant_id=0 AND refund_no=?`, handoff.RefundNo)
	if err != nil {
		return err
	}
	switch status := strings.TrimSpace(value.String()); status {
	case "succeeded":
		commissionStatus, statusErr := w.database.GetValue(ctx, `
SELECT status_code FROM plugin_idc_core_outbox_event
WHERE tenant_id=0 AND event_type=? AND aggregate_no=?`, refundCommissionEventType, handoff.RefundNo)
		if statusErr != nil {
			return statusErr
		}
		switch strings.TrimSpace(commissionStatus.String()) {
		case "published":
			return w.completeResizeRefund(ctx, handoff)
		case "dead_letter", "result_unknown":
			return w.markResizeRefundManual(ctx, handoff, "server_resize_refund_commission_reversal_failed")
		case "pending", "processing", "retry_wait":
			return nil
		default:
			return w.markResizeRefundManual(ctx, handoff, "server_resize_refund_commission_event_missing")
		}
	case "failed", "result_unknown", "manual_confirmation", "awaiting_balance_consent", "rejected":
		return w.markResizeRefundManual(ctx, handoff, "server_resize_refund_"+status)
	default:
		return nil
	}
}

func (w *resizeRefundWorker) completeResizeRefund(ctx context.Context, handoff *resizeRefundHandoffRow) error {
	result, err := w.database.Exec(ctx, `
UPDATE plugin_idc_core_server_resize_refund_handoff
SET status_code='succeeded',last_error_code='',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND handoff_no=? AND status_code='processing'`, handoff.HandoffNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(result); err != nil {
		return err
	}
	result, err = w.database.Exec(ctx, `
UPDATE plugin_idc_core_server_resize_intent
SET status_code='failed',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND status_code='refund_pending'`, handoff.OperationNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(result); err != nil {
		return err
	}
	result, err = w.database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code='failed',last_error_code='server_resize.provider_failed_refunded',completed_at=CURRENT_TIMESTAMP,
    updated_at=CURRENT_TIMESTAMP,version=version+1
WHERE tenant_id=0 AND operation_no=? AND status_code='refunding'`, handoff.OperationNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(result); err != nil {
		return err
	}
	if err := w.appendResizeRefundOperationEvent(
		ctx, handoff, "refunding", "failed", "server_resize.provider_failed_refunded",
	); err != nil {
		return err
	}
	result, err = w.database.Exec(ctx, `
UPDATE plugin_idc_core_commerce_order
SET status_code='refunded',completed_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND order_no=? AND status_code='refund_pending'`, handoff.OrderNo)
	if err != nil {
		return err
	}
	return requireSingleRefundExecutionUpdate(result)
}

func (w *resizeRefundWorker) markResizeRefundManual(
	ctx context.Context,
	handoff *resizeRefundHandoffRow,
	reason string,
) error {
	result, err := w.database.Exec(ctx, `
UPDATE plugin_idc_core_server_resize_refund_handoff
SET status_code='manual_review',last_error_code=?,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND handoff_no=? AND status_code IN ('ready','processing')`, reason, handoff.HandoffNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(result); err != nil {
		return err
	}
	result, err = w.database.Exec(ctx, `
UPDATE plugin_idc_core_server_resize_intent
SET status_code='manual_review',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND status_code='refund_pending'`, handoff.OperationNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(result); err != nil {
		return err
	}
	result, err = w.database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code='manual_review',last_error_code=?,updated_at=CURRENT_TIMESTAMP,version=version+1
WHERE tenant_id=0 AND operation_no=? AND status_code='refunding'`, reason, handoff.OperationNo)
	if err != nil {
		return err
	}
	if err = requireSingleRefundExecutionUpdate(result); err != nil {
		return err
	}
	return w.appendResizeRefundOperationEvent(
		ctx, handoff, "refunding", "manual_review", reason,
	)
}

func (w *resizeRefundWorker) appendResizeRefundOperationEvent(
	ctx context.Context,
	handoff *resizeRefundHandoffRow,
	fromStatus string,
	toStatus string,
	reason string,
) error {
	var operation *struct {
		ServiceNo string `orm:"service_no"`
		Sequence  int    `orm:"sequence"`
	}
	if err := w.database.GetScan(ctx, &operation, `
SELECT operation.service_no,COALESCE(MAX(event.event_sequence),0)+1 AS sequence
FROM plugin_idc_core_business_operation operation
LEFT JOIN plugin_idc_core_business_operation_event event
  ON event.operation_no=operation.operation_no
WHERE operation.tenant_id=0 AND operation.operation_no=?
GROUP BY operation.service_no`, handoff.OperationNo); err != nil {
		return err
	}
	if operation == nil || operation.ServiceNo == "" || operation.Sequence < 1 {
		return gerror.New("resize refund operation event reference is unavailable")
	}
	eventNo, err := newBillingNumber(rand.Reader, "boe_")
	if err != nil {
		return err
	}
	content, err := contentDigest(struct {
		OperationNo string `json:"operationNo"`
		Sequence    int    `json:"sequence"`
		FromStatus  string `json:"fromStatus"`
		ToStatus    string `json:"toStatus"`
		Reason      string `json:"reason"`
		Evidence    string `json:"evidence"`
	}{handoff.OperationNo, operation.Sequence, fromStatus, toStatus, reason, handoff.EvidenceDigest})
	if err != nil {
		return err
	}
	_, err = w.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation_event
    (tenant_id,event_no,operation_no,service_no,event_sequence,from_status_code,
     to_status_code,reason_code,actor_type_code,actor_reference,evidence_digest,
     content_digest,created_at)
VALUES (0,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)`, eventNo, handoff.OperationNo,
		operation.ServiceNo, operation.Sequence, fromStatus, toStatus, reason,
		"system", "resize-refund-worker", handoff.EvidenceDigest, content)
	return err
}
