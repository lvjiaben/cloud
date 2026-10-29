// This file records staff-selected refunds for conclusively failed server delivery.
// It reuses invoice allocation and approval rules, retaining the original payment
// and requiring the same financial permission as a staff refund decision.

package billing

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// FailedFulfillmentRefundInput identifies one failed delivery and an immutable staff decision.
type FailedFulfillmentRefundInput struct {
	IntentNo        string
	ExpectedVersion int64
	IdempotencyKey  string
	Note            string
}

// RefundFailedFulfillment submits to the existing refund policy and execution workflow.
// Unknown, running, canceled, completed and already-refunded deliveries cannot enter.
func (s *managementRefundService) RefundFailedFulfillment(ctx context.Context, input FailedFulfillmentRefundInput) (*OnlineRefund, error) {
	actor, err := s.authorizer.AuthorizeRefundReview(ctx, input.IntentNo)
	if err != nil {
		return nil, err
	}
	input.Note = strings.TrimSpace(input.Note)
	if ctx == nil || ctx.Err() != nil || actor <= 0 || !validFailedFulfillmentRefund(input) {
		return nil, bizerr.NewCode(CodeRefundInvalid)
	}

	requestDigest, err := contentDigest(input)
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	keyDigest, err := contentDigest(struct{ Scope, Key string }{"staff_failed_fulfillment", input.IdempotencyKey})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	var result *OnlineRefund
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var row *struct {
			CustomerNo, OrderNo, OrderItemNo, IntentStatus, TaskStatus, InvoiceNo, InvoiceItemNo, OrderStatus, PhaseCode, ExternalReference string
			Version                                                                                                                         int64
			UnitSequence, Quantity                                                                                                          int
			RefundableCapMinor                                                                                                              int64
			RecoverableRejection                                                                                                            bool
		}
		loadErr := s.database.GetScan(txCtx, &row, `SELECT invoice.customer_no,intent.order_no,intent.order_item_no,intent.status_code AS intent_status,intent.version,intent.unit_sequence,
 task.status_code AS task_status,task.phase_code,task.external_reference,
 EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_attempt attempt
  WHERE attempt.tenant_id=task.tenant_id AND attempt.task_no=task.task_no AND attempt.attempt_no=task.attempt_count
  AND attempt.finished_at IS NOT NULL AND attempt.result_status_code='failed'
  AND attempt.safe_result_evidence->>'recoverableRejection'='true') OR (task.last_result_code='operator_non_acceptance_verified' AND EXISTS(SELECT 1 FROM plugin_idc_core_fulfillment_non_acceptance_verification fact WHERE fact.tenant_id=task.tenant_id AND fact.task_no=task.task_no AND fact.provider_task_no=task.external_reference)) AS recoverable_rejection,
 invoice.invoice_no,item.invoice_item_no,item.refundable_cap_minor,quantity.quantity,orders.status_code AS order_status
 FROM plugin_idc_core_product_fulfillment_intent intent
 JOIN plugin_idc_core_product_fulfillment_task task ON task.tenant_id=intent.tenant_id AND task.intent_no=intent.intent_no
 JOIN plugin_idc_core_commerce_order orders ON orders.tenant_id=intent.tenant_id AND orders.order_no=intent.order_no
 JOIN plugin_idc_core_billing_invoice invoice ON invoice.tenant_id=intent.tenant_id AND invoice.order_no=intent.order_no
 JOIN plugin_idc_core_billing_invoice_item item ON item.tenant_id=intent.tenant_id AND item.invoice_no=invoice.invoice_no AND item.order_item_no=intent.order_item_no
 JOIN plugin_idc_core_commerce_order_item_quantity_snapshot quantity ON quantity.order_item_no=intent.order_item_no
 WHERE intent.tenant_id=0 AND intent.intent_no=? AND intent.entitlement_owner_id='idc-resource'
 FOR UPDATE OF intent,task,invoice`, input.IntentNo)
		if loadErr != nil {
			return loadErr
		}
		if row == nil {
			return bizerr.NewCode(CodeRefundNotAccessible)
		}
		replay, replayErr := s.commands.loadRefundReplay(txCtx, row.CustomerNo, keyDigest)
		if replayErr != nil {
			return replayErr
		}
		if replay != nil {
			storedDigest, digestErr := s.database.GetValue(txCtx, `SELECT request_digest FROM plugin_idc_core_refund_request WHERE tenant_id=0 AND refund_no=?`, replay.RefundNo)
			if digestErr != nil {
				return digestErr
			}
			if storedDigest.String() != requestDigest {
				return bizerr.NewCode(CodeRefundConflict)
			}
			replay.Reused = true
			result = replay
			return nil
		}
		if row.Version != input.ExpectedVersion || row.OrderStatus != "paid" || (row.IntentStatus != "incomplete" || row.PhaseCode != "post_submit_query" || row.ExternalReference == "" || !row.RecoverableRejection) || row.TaskStatus != "failed" || row.Quantity < 1 || row.UnitSequence < 1 || row.UnitSequence > row.Quantity {
			return bizerr.NewCode(CodeRefundConflict)
		}
		existing, existingErr := s.database.GetValue(txCtx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_refund_request_item item JOIN plugin_idc_core_refund_request request ON request.refund_no=item.refund_no AND request.tenant_id=item.tenant_id WHERE item.tenant_id=0 AND item.order_item_no=? AND request.status_code<>'rejected')`, row.OrderItemNo)
		if existingErr != nil {
			return existingErr
		}
		if existing.Bool() {
			return bizerr.NewCode(CodeRefundConflict)
		}
		amount := row.RefundableCapMinor / int64(row.Quantity)
		if int64(row.UnitSequence) <= row.RefundableCapMinor%int64(row.Quantity) {
			amount++
		}
		if amount < 1 {
			return bizerr.NewCode(CodeRefundConflict)
		}
		preview, previewErr := s.commands.previewForCustomer(txCtx, row.CustomerNo, RefundPreviewInput{OrderNo: row.OrderNo, Selections: []RefundSelectionInput{{InvoiceItemNo: row.InvoiceItemNo, Quantity: 1, AmountMinor: amount}}}, true)
		if previewErr != nil {
			return previewErr
		}
		if preview.HasResourceImpact || preview.HasDangerousOperation || preview.HasFundAnomaly {
			return bizerr.NewCode(CodeRefundConflict)
		}
		// Staff explicitly chooses refund; existing financial policy decides execution.
		// Original payment-source allocation and customer balance-consent rules remain authoritative.
		created, createErr := s.commands.insertRefundRequest(txCtx, actor, preview, RefundReasonServiceIssue, input.Note, keyDigest, requestDigest)
		if createErr != nil {
			return createErr
		}
		result = created
		return s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "billing.refund.failed_fulfillment", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "billing.refund.failed_fulfillment", TargetType: "refund_request", TargetNo: created.RefundNo,
			CustomerNo: row.CustomerNo, ResultCode: "succeeded", ReasonCode: string(RefundReasonServiceIssue), RiskCode: "high", RequiredPermission: permissionBillingRefundReview,
			CorrelationNo: input.IntentNo, BeforeSnapshot: map[string]any{"fulfillment_status": row.IntentStatus, "intent_version": row.Version}, AfterSnapshot: refundAuditSnapshot(created),
		})
	})
	if err != nil {
		return nil, normalizeOnlineRefundError(err)
	}
	return result, nil
}

// validFailedFulfillmentRefund rejects unbounded or incomplete staff commands.
func validFailedFulfillmentRefund(input FailedFulfillmentRefundInput) bool {
	return validBusinessReference(input.IntentNo, 40) && input.ExpectedVersion > 0 && len(input.IdempotencyKey) >= 16 && len(input.IdempotencyKey) <= 128 && input.Note != "" && utf8.RuneCountInString(input.Note) <= 1000
}
