// This file records staff verification of non-acceptance without rewriting any
// closed attempt. Owner evidence and business audit share the caller transaction;
// credential revocation is delivered only after commit through the existing outbox.

package productfulfillment

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/audit"
)

// verificationIntentIncomplete is the existing terminal failure state supported by staff proof.
const verificationIntentIncomplete recoveryIntentState = "incomplete"

// NonAcceptanceInput requires an explicit staff assertion and a bounded note.
type NonAcceptanceInput struct {
	ActionInput
	ConfirmedNoCreation bool
}

// NonAcceptanceRecord is an independent audit fact, not a supplier response.
type NonAcceptanceRecord struct {
	ProviderTaskNo string
	ActorUserID    int64
	Note           string
	VerifiedAt     time.Time
}

// verificationTask binds the current core task to its original resource task.
type verificationTask struct {
	TaskNo, ProviderTaskNo, SettlementNo string
	Version                              int64
}

// VerifyNonAcceptance authorizes the customer scope and records manual proof.
// It performs no upstream request and never starts a recovery or refund itself.
func (s *serviceImpl) VerifyNonAcceptance(ctx context.Context, input NonAcceptanceInput) (*IntentDetail, error) {
	if s.authorizer == nil || s.audit == nil || s.resourceSource == nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	authorization, err := s.authorizer.AuthorizeRecovery(ctx, input.IntentNo)
	if err != nil {
		return nil, err
	}
	input.IntentNo, input.IdempotencyKey = strings.TrimSpace(input.IntentNo), strings.TrimSpace(input.IdempotencyKey)
	input.Note = strings.TrimSpace(input.Note)
	if !validRecoveryAction(input.ActionInput, authorization) || !input.ConfirmedNoCreation || input.RecoveryConfiguration != nil {
		return nil, errorsNew("product_fulfillment_action_invalid")
	}
	digest, err := digestJSON(struct {
		Input       NonAcceptanceInput
		ActorUserID int
	}{input, authorization.ActorUserID})
	if err != nil {
		return nil, err
	}
	var detail *IntentDetail
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		current, loadErr := s.loadDetailAuthorized(txCtx, input.IntentNo, true, authorization)
		if loadErr != nil {
			return loadErr
		}
		replay, loadErr := s.database.GetValue(txCtx, `SELECT request_digest FROM plugin_idc_core_fulfillment_non_acceptance_verification WHERE tenant_id=0 AND intent_no=? AND idempotency_key=?`, input.IntentNo, input.IdempotencyKey)
		if loadErr != nil {
			return loadErr
		}
		if !replay.IsEmpty() {
			if replay.String() != digest {
				return errorsNew("product_fulfillment_action_conflict")
			}
			detail = current
			return nil
		}
		if current.Version != input.ExpectedVersion {
			return errorsNew("product_fulfillment_action_conflict")
		}
		task, loadErr := s.nonAcceptanceTask(txCtx, input.IntentNo, true)
		if loadErr != nil {
			return loadErr
		}
		if task == nil {
			return errorsNew("product_fulfillment_action_conflict")
		}
		authority, loadErr := s.lockSettlement(txCtx, SettlementInput{SettlementNo: task.SettlementNo, OrderNo: current.OrderNo})
		if loadErr != nil {
			return loadErr
		}
		if loadErr = s.ensureRecoveryNotRefunded(txCtx, authority.InvoiceNo, current.OrderItemNo); loadErr != nil {
			return loadErr
		}
		request, loadErr := s.loadPaidResourceRequest(txCtx, input.IntentNo)
		if loadErr != nil {
			return loadErr
		}
		if loadErr = s.resourceSource.VerifyNonAcceptance(txCtx, provisionresourcecap.NonAcceptanceVerification{Task: provisionresourcecap.TaskReadRequest{ProviderTaskNo: task.ProviderTaskNo, Request: request}, ActorUserID: authorization.ActorUserID, Note: input.Note, Digest: digest}); loadErr != nil {
			return bizerr.WrapCode(loadErr, CodeActionConflict)
		}
		if _, loadErr = s.database.Model(do.FulfillmentNonAcceptanceVerification{}).Ctx(txCtx).Data(do.FulfillmentNonAcceptanceVerification{TenantId: 0, ProviderTaskNo: task.ProviderTaskNo, IntentNo: input.IntentNo, TaskNo: task.TaskNo, PreviousTaskVersion: task.Version, ResultingVersion: current.Version + 1, IdempotencyKey: input.IdempotencyKey, RequestDigest: digest, ActorUserId: authorization.ActorUserID, SafeNote: input.Note}).Insert(); loadErr != nil {
			return loadErr
		}
		updated, loadErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_product_fulfillment_task SET status_code='failed',phase_code='post_submit_query',last_result_code=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND version=?`, provisionresourcecap.FailureNonAcceptanceVerified, task.TaskNo, task.Version)
		if loadErr != nil || affected(updated) != 1 {
			return errorsNew("product_fulfillment_action_conflict")
		}
		if loadErr = updateIntentStatus(txCtx, s.database, current, string(verificationIntentIncomplete)); loadErr != nil {
			return loadErr
		}
		projection, loadErr := s.resourceSource.ReadTask(txCtx, provisionresourcecap.TaskReadRequest{ProviderTaskNo: task.ProviderTaskNo, Request: request})
		if loadErr != nil {
			return loadErr
		}
		if projection == nil || projection.Status != provisionresourcecap.TaskStatusFailed || projection.FailureCode != provisionresourcecap.FailureNonAcceptanceVerified {
			return errorsNew("product_fulfillment_action_conflict")
		}
		report := provisionresourcecap.OutcomeReport{ProviderTaskNo: task.ProviderTaskNo, Request: request, Status: provisionresourcecap.OutcomeStatusFailed, TaskEvidenceDigest: projection.EvidenceDigest, CoreEvidenceDigest: digest}
		if provisionresourcecap.ValidateOutcomeReport(report) != nil {
			return errorsNew("product_fulfillment_resource_outcome_invalid")
		}
		payload, loadErr := json.Marshal(report)
		if loadErr != nil {
			return loadErr
		}
		if _, loadErr = s.database.Exec(txCtx, `INSERT INTO plugin_idc_core_product_fulfillment_resource_outcome_outbox(tenant_id,task_no,provider_task_no,report_payload,report_digest,status_code,next_attempt_at) VALUES(0,?,?,?,?,'pending',CURRENT_TIMESTAMP)`, task.TaskNo, task.ProviderTaskNo, payload, sha256Hex(payload)); loadErr != nil {
			return loadErr
		}
		if loadErr = s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{IdempotencyScope: "product-fulfillment.verify_non_acceptance", IdempotencyKey: input.IdempotencyKey, OperationCode: "product-fulfillment.verify_non_acceptance", TargetType: "product_fulfillment_intent", TargetNo: input.IntentNo, CustomerNo: current.CustomerNo, ResultCode: idccap.BusinessAuditResultSucceeded, ReasonCode: provisionresourcecap.FailureNonAcceptanceVerified, RiskCode: string(audit.RiskHigh), RequiredPermission: PermissionPreparationRecover, BeforeSnapshot: map[string]any{"status": current.internalStatus, "version": current.Version}, AfterSnapshot: map[string]any{"status": verificationIntentIncomplete, "version": current.Version + 1, "providerTaskNo": task.ProviderTaskNo, "note": input.Note}}); loadErr != nil {
			return loadErr
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

// nonAcceptanceTask reads one local unknown task, refusing finance and conclusion conflicts.
func (s *serviceImpl) nonAcceptanceTask(ctx context.Context, intentNo string, lock bool) (*verificationTask, error) {
	query := `SELECT task.task_no,task.external_reference AS provider_task_no,task.version,intent.settlement_no
 FROM plugin_idc_core_product_fulfillment_intent intent JOIN plugin_idc_core_product_fulfillment_task task ON task.tenant_id=intent.tenant_id AND task.intent_no=intent.intent_no
 WHERE intent.tenant_id=0 AND intent.intent_no=? AND intent.status_code='manual_pending' AND intent.entitlement_owner_id='idc-resource'
 AND task.status_code='result_unknown' AND task.phase_code='frozen_unknown' AND task.external_reference<>'' AND task.lease_token=''
 AND NOT task.takeover_requested AND NOT task.cancel_requested
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_product_fulfillment_manual_conclusion conclusion WHERE conclusion.intent_no=intent.intent_no)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_request_item item JOIN plugin_idc_core_refund_request refund ON refund.refund_no=item.refund_no AND refund.tenant_id=item.tenant_id WHERE item.tenant_id=0 AND item.order_item_no=intent.order_item_no AND refund.status_code<>'rejected') LIMIT 1`
	if lock {
		query += ` FOR UPDATE OF task`
	}
	var task *verificationTask
	err := s.database.GetScan(ctx, &task, query, intentNo)
	return task, err
}

// loadNonAcceptanceHistory is bounded to the latest 16 independent local staff facts.
func (s *serviceImpl) loadNonAcceptanceHistory(ctx context.Context, intentNo string) ([]NonAcceptanceRecord, error) {
	var records []NonAcceptanceRecord
	err := s.database.GetScan(ctx, &records, `SELECT provider_task_no,actor_user_id,safe_note AS note,verified_at FROM plugin_idc_core_fulfillment_non_acceptance_verification WHERE tenant_id=0 AND intent_no=? ORDER BY verified_at DESC,provider_task_no LIMIT 16`, intentNo)
	return records, err
}
