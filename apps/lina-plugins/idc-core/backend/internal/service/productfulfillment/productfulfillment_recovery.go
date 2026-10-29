// This file restores resource preparation failures and confirmed rejections through audited
// commands. Counters and closed attempts remain immutable cumulative history.

package productfulfillment

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
)

// recoveryAction identifies the separately authorized recovery command.
type recoveryAction string

const actionRecoverPreparation recoveryAction = "recover_preparation"

// preparationRecoveryTask projects only failures proven safe to recover.
type preparationRecoveryTask struct {
	TaskNo                  string
	SettlementNo            string
	Version                 int64
	QueryCount, SubmitCount int
}

// RecoverPreparation records a bounded recovery after preparation or a confirmed
// rejection. It preserves original identity and never replays an unknown write.
func (s *serviceImpl) RecoverPreparation(ctx context.Context, input ActionInput) (*IntentDetail, error) {
	if s.authorizer == nil || s.audit == nil || s.gates == nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	authorization, err := s.authorizer.AuthorizeRecovery(ctx, input.IntentNo)
	if err != nil {
		return nil, err
	}
	input.IntentNo, input.IdempotencyKey = strings.TrimSpace(input.IntentNo), strings.TrimSpace(input.IdempotencyKey)
	input.Note, input.ReasonCode = strings.TrimSpace(input.Note), strings.TrimSpace(input.ReasonCode)
	if !validRecoveryAction(input, authorization) {
		return nil, errorsNew("product_fulfillment_action_invalid")
	}
	digest, err := digestJSON(input)
	if err != nil {
		return nil, err
	}
	if replayed, preflightErr := s.validateRecoveryCorrection(ctx, input, authorization, digest); preflightErr != nil || replayed != nil {
		return replayed, preflightErr
	}
	correctionJSON, err := json.Marshal(input.RecoveryConfiguration)
	if err != nil {
		return nil, errorsNew("product_fulfillment_configuration_invalid")
	}
	var detail *IntentDetail
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		current, loadErr := s.loadDetailAuthorized(txCtx, input.IntentNo, true, authorization)
		if loadErr != nil {
			return loadErr
		}
		replayed, replayErr := s.commandReplay(txCtx, string(actionRecoverPreparation), input, digest)
		if replayErr != nil {
			return replayErr
		}
		if replayed {
			detail = current
			return nil
		}
		if current.Version != input.ExpectedVersion {
			return errorsNew("product_fulfillment_action_conflict")
		}
		task, taskErr := s.preparationRecoveryTask(txCtx, input.IntentNo, true)
		if taskErr != nil {
			return taskErr
		}
		if task == nil {
			return errorsNew("product_fulfillment_action_conflict")
		}
		authority, authorityErr := s.lockSettlement(txCtx, SettlementInput{SettlementNo: task.SettlementNo, OrderNo: current.OrderNo})
		if authorityErr != nil {
			return authorityErr
		}
		if refundErr := s.ensureRecoveryNotRefunded(txCtx, authority.InvoiceNo, current.OrderItemNo); refundErr != nil {
			return refundErr
		}
		items, itemsErr := s.loadFrozenItems(txCtx, authority, []string{current.OrderItemNo})
		if itemsErr != nil {
			return itemsErr
		}
		if frozenErr := s.validateFrozenConfigurations(txCtx, items); frozenErr != nil {
			return frozenErr
		}
		if releaseErr := s.validateRecoveryOwnerRelease(txCtx, current, input.RecoveryConfiguration); releaseErr != nil {
			return releaseErr
		}
		reason, reasonErr := s.validateReason(txCtx, string(actionRecoverPreparation), input.ReasonCode)
		if reasonErr != nil {
			return reasonErr
		}
		projection, projectionErr := s.resumeResourceProjection(txCtx, current)
		if projectionErr != nil {
			return projectionErr
		}
		commandNo := deterministicNo("pfc_", string(actionRecoverPreparation), input.IdempotencyKey)
		if _, insertErr := s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_command(tenant_id,command_no,intent_no,action_code,idempotency_key,request_digest,reason_code,reason_label_zh_cn,reason_label_en_us,safe_note,actor_user_id,resulting_version,recovery_task_version,recovery_query_count,recovery_submit_count,recovery_configuration) VALUES(0,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?::jsonb)`, commandNo, input.IntentNo, actionRecoverPreparation, input.IdempotencyKey, digest, input.ReasonCode, reason.LabelZhCN, reason.LabelEnUS, input.Note, authorization.ActorUserID, current.Version+1, task.Version, task.QueryCount, task.SubmitCount, correctionJSON); insertErr != nil {
			return insertErr
		}
		result, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='pending_query',phase_code='pre_submit_query',recovery_count=recovery_count+1,recovery_query_baseline=query_attempt_count,recovery_submit_baseline=submit_attempt_count,next_attempt_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND version=?`, task.TaskNo, task.Version)
		if updateErr != nil || affected(result) != 1 {
			return errorsNew("product_fulfillment_action_conflict")
		}
		if updateErr = updateIntentStatus(txCtx, s.database, current, string(recoveryIntentQueued)); updateErr != nil {
			return updateErr
		}
		afterSnapshot := map[string]any{"status": recoveryIntentQueued, "queryBaseline": task.QueryCount, "submitBaseline": task.SubmitCount, "version": current.Version + 1}
		if projection != nil {
			afterSnapshot["serverProjectionRecovery"] = projection
		}
		if auditErr := s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "product-fulfillment.recover_preparation", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "product-fulfillment.recover_preparation", TargetType: "product_fulfillment_intent",
			TargetNo: input.IntentNo, CustomerNo: current.CustomerNo, ResultCode: "succeeded",
			ReasonCode: input.ReasonCode, RiskCode: "high", RequiredPermission: PermissionPreparationRecover,
			BeforeSnapshot: map[string]any{"status": current.internalStatus, "queries": task.QueryCount, "submits": task.SubmitCount, "version": current.Version},
			AfterSnapshot:  afterSnapshot,
		}); auditErr != nil {
			return auditErr
		}
		detail, loadErr = s.loadDetail(txCtx, input.IntentNo, false)
		return loadErr
	})
	if err != nil {
		if _, business := bizerr.As(err); !business {
			err = bizerr.WrapCode(err, CodeManagementUnavailable)
		}
	}
	return detail, err
}

// recoveryIntentState identifies the existing durable intent state.
type recoveryIntentState string

const recoveryIntentQueued recoveryIntentState = "queued"

// preparationRecoveryTask is bounded by one intent and locks its task only
// during execution. The owner keeps the same idempotent resource operation ID.
func (s *serviceImpl) preparationRecoveryTask(ctx context.Context, intentNo string, lock bool) (*preparationRecoveryTask, error) {
	query := `SELECT task.task_no,task.version,intent.settlement_no,task.query_attempt_count AS query_count,task.submit_attempt_count AS submit_count
FROM plugin_idc_core_product_fulfillment_intent intent
JOIN plugin_idc_core_product_fulfillment_task task ON task.intent_no=intent.intent_no AND task.tenant_id=intent.tenant_id
WHERE intent.tenant_id=0 AND intent.intent_no=? AND intent.status_code IN ('manual_pending','incomplete') AND intent.entitlement_owner_id='idc-resource'
AND task.status_code='failed' AND task.lease_token=''
AND ((intent.status_code='manual_pending' AND task.phase_code='pre_submit_query' AND task.external_reference='')
 OR (intent.status_code='incomplete' AND task.phase_code='post_submit_query' AND task.external_reference<>''
 AND EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_attempt attempt
 WHERE attempt.tenant_id=task.tenant_id AND attempt.task_no=task.task_no AND attempt.attempt_no=task.attempt_count
 AND attempt.result_status_code='failed' AND attempt.finished_at IS NOT NULL
 AND attempt.safe_result_evidence->>'recoverableRejection'='true'))
 OR (intent.status_code='incomplete' AND task.last_result_code='operator_non_acceptance_verified' AND EXISTS(SELECT 1 FROM plugin_idc_core_fulfillment_non_acceptance_verification fact WHERE fact.tenant_id=task.tenant_id AND fact.task_no=task.task_no AND fact.provider_task_no=task.external_reference)))
AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_request_item item JOIN plugin_idc_core_refund_request refund ON refund.refund_no=item.refund_no AND refund.tenant_id=item.tenant_id WHERE item.tenant_id=intent.tenant_id AND item.order_item_no=intent.order_item_no AND refund.status_code<>'rejected')
AND NOT task.takeover_requested AND NOT task.cancel_requested
AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_manual_conclusion conclusion WHERE conclusion.intent_no=intent.intent_no)
LIMIT 1`
	if lock {
		query += ` FOR UPDATE OF task`
	}
	var task *preparationRecoveryTask
	err := s.database.GetScan(ctx, &task, query, intentNo)
	return task, err
}

// ensureRecoveryNotRefunded serializes with financial allocation and refund reservation.
func (s *serviceImpl) ensureRecoveryNotRefunded(ctx context.Context, invoiceNo, orderItemNo string) error {
	if _, err := s.database.GetValue(ctx, `SELECT invoice_no FROM plugin_idc_core_billing_invoice WHERE tenant_id=0 AND invoice_no=? FOR UPDATE`, invoiceNo); err != nil {
		return err
	}
	refund, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_refund_request_item item JOIN plugin_idc_core_refund_request request ON request.refund_no=item.refund_no AND request.tenant_id=item.tenant_id WHERE item.tenant_id=0 AND item.order_item_no=? AND request.status_code<>'rejected')`, orderItemNo)
	if err != nil {
		return err
	}
	if refund.Bool() {
		return errorsNew("product_fulfillment_action_conflict")
	}
	return nil
}

// validRecoveryAction validates the exact staff authority and bounded command
// identity before any paid configuration or operation facts become visible.
func validRecoveryAction(input ActionInput, authorization Authorization) bool {
	return authorization.ActorUserID > 0 && authorization.RequiredPermission == PermissionPreparationRecover &&
		validAuthorizationScope(authorization) && managementReferencePattern.MatchString(input.IntentNo) &&
		input.ExpectedVersion > 0 && len(input.IdempotencyKey) >= 16 && len(input.IdempotencyKey) <= 128 &&
		managementReferencePattern.MatchString(input.IdempotencyKey) && validSafeNote(input.Note, true)
}
