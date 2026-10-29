// This file repairs only the resource owner's pure local entitlement mapping.
// It requeues one existing notification with audit and idempotency; provider
// tasks, workflow versions, paid specifications and money remain unchanged.

package productfulfillment

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/provisionresourcecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/entitlementprogress"
)

// These stable values identify a local repair command without a workflow transition.
const (
	actionRepairManagement    = "repair_management"
	reasonManagementRepair    = "management_link_repair"
	managementRepairCompleted = "completed"
)

// managementRepairCandidate carries the stopped event and its financial authority.
type managementRepairCandidate struct {
	EventNo       string
	AttemptCount  int
	LastErrorCode string
	SettlementNo  string
}

// managementRepairCandidate reads one uniquely owned stopped notification.
// Only the already-audited resource sink is known to perform local writes only.
func (s *serviceImpl) managementRepairCandidate(ctx context.Context, current *IntentDetail, lock bool) (*managementRepairCandidate, error) {
	var rows []managementRepairCandidate
	query := `SELECT event.event_no,event.attempt_count,event.last_error_code,intent.settlement_no
 FROM plugin_idc_core_entitlement_created_outbox event
 JOIN plugin_idc_core_service_entitlement entitlement
 ON entitlement.tenant_id=event.tenant_id AND entitlement.entitlement_no=event.entitlement_no
 AND entitlement.source_intent_no=event.intent_no AND entitlement.customer_no=event.customer_no
 AND entitlement.entitlement_owner_id=event.owner_id
 JOIN plugin_idc_core_product_fulfillment_intent intent
 ON intent.tenant_id=event.tenant_id AND intent.intent_no=event.intent_no
 AND intent.customer_no=event.customer_no AND intent.entitlement_owner_id=event.owner_id
 AND intent.order_item_no=entitlement.order_item_no
 WHERE event.tenant_id=0 AND event.intent_no=? AND event.customer_no=? AND event.owner_id=?
 AND intent.order_no=? AND intent.order_item_no=? AND intent.status_code=?
 AND intent.financially_settled_at IS NOT NULL
 AND entitlement.status_code IN ('active','suspended','expired')
 AND event.status_code=? AND event.lease_token='' AND event.lease_expires_at IS NULL
 AND event.owner_reference='' AND event.receipt_evidence_digest=''
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_request_item item
 JOIN plugin_idc_core_refund_request refund ON refund.tenant_id=item.tenant_id AND refund.refund_no=item.refund_no
 WHERE item.tenant_id=event.tenant_id AND item.order_item_no=intent.order_item_no AND refund.status_code<>'rejected')
 LIMIT 2`
	if lock {
		query += " FOR UPDATE OF event"
	}
	if err := s.database.GetScan(ctx, &rows, query, current.IntentNo, current.CustomerNo, provisionresourcecap.OwnerID,
		current.OrderNo, current.OrderItemNo, managementRepairCompleted, projectionManualAttention); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if len(rows) != 1 {
		return nil, bizerr.NewCode(CodeActionConflict)
	}
	return &rows[0], nil
}

// RepairManagement requires the normal recovery permission and current login.
// It never invokes an execution source, dynamic connector or external network.
func (s *serviceImpl) RepairManagement(ctx context.Context, input ManagementRepairInput) (*IntentDetail, error) {
	if s.authorizer == nil || s.audit == nil {
		return nil, errorsNew("product_fulfillment_management_unavailable")
	}
	authorization, err := s.authorizer.AuthorizeRecovery(ctx, input.IntentNo)
	if err != nil {
		return nil, err
	}
	input.IntentNo, input.IdempotencyKey = strings.TrimSpace(input.IntentNo), strings.TrimSpace(input.IdempotencyKey)
	if authorization.ActorUserID < 1 || authorization.RequiredPermission != PermissionPreparationRecover || !validAuthorizationScope(authorization) ||
		!managementReferencePattern.MatchString(input.IntentNo) || input.ExpectedVersion < 1 || len(input.IdempotencyKey) < 16 || len(input.IdempotencyKey) > 128 || !managementReferencePattern.MatchString(input.IdempotencyKey) {
		return nil, errorsNew("product_fulfillment_action_invalid")
	}
	digest, err := digestJSON(input)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
	}
	var detail *IntentDetail
	err = s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		current, loadErr := s.loadDetailAuthorized(txCtx, input.IntentNo, true, authorization)
		if loadErr != nil {
			return loadErr
		}
		replayed, replayErr := s.commandReplay(txCtx, actionRepairManagement, ActionInput{IntentNo: input.IntentNo, IdempotencyKey: input.IdempotencyKey}, digest)
		if replayErr != nil {
			return replayErr
		}
		if replayed {
			detail = current
			return nil
		}
		if current.Version != input.ExpectedVersion || current.StatusCode != managementRepairCompleted || current.FinanciallySettledAt.IsZero() {
			return bizerr.NewCode(CodeActionConflict)
		}
		candidate, candidateErr := s.managementRepairCandidate(txCtx, current, true)
		if candidateErr != nil {
			return candidateErr
		}
		if candidate == nil {
			return bizerr.NewCode(CodeActionConflict)
		}
		authority, authorityErr := s.lockSettlement(txCtx, SettlementInput{SettlementNo: candidate.SettlementNo, OrderNo: current.OrderNo})
		if authorityErr != nil {
			return authorityErr
		}
		if authority.ServiceOwnerCustomerNo != current.CustomerNo {
			return bizerr.NewCode(CodeActionConflict)
		}
		if refundErr := s.ensureRecoveryNotRefunded(txCtx, authority.InvoiceNo, current.OrderItemNo); refundErr != nil {
			return refundErr
		}
		reason, reasonErr := s.validateReason(txCtx, actionRepairManagement, reasonManagementRepair)
		if reasonErr != nil {
			return reasonErr
		}
		commandNo := deterministicNo("pfc_", actionRepairManagement, input.IdempotencyKey)
		if _, insertErr := s.database.Model(do.ProductFulfillmentCommand{}).Ctx(txCtx).Data(do.ProductFulfillmentCommand{
			TenantId: 0, CommandNo: commandNo, IntentNo: input.IntentNo, ActionCode: actionRepairManagement,
			IdempotencyKey: input.IdempotencyKey, RequestDigest: digest, ReasonCode: reasonManagementRepair,
			ReasonLabelZhCn: reason.LabelZhCN, ReasonLabelEnUs: reason.LabelEnUS, SafeNote: "", ActorUserId: authorization.ActorUserID, ResultingVersion: current.Version,
		}).Insert(); insertErr != nil {
			return insertErr
		}
		// This queue CAS is deliberately one SQL transition. Its predicate preserves
		// the stopped notification and excludes workers holding a live delivery lease.
		result, updateErr := s.database.Exec(txCtx, `UPDATE plugin_idc_core_entitlement_created_outbox
 SET status_code=?,attempt_count=0,next_attempt_at=CURRENT_TIMESTAMP,lease_token='',lease_expires_at=NULL,last_error_code='',updated_at=CURRENT_TIMESTAMP
 WHERE tenant_id=0 AND event_no=? AND status_code=? AND attempt_count=? AND lease_token='' AND lease_expires_at IS NULL`, projectionPending, candidate.EventNo, projectionManualAttention, candidate.AttemptCount)
		if updateErr != nil {
			return updateErr
		}
		if affected(result) != 1 {
			return bizerr.NewCode(CodeActionConflict)
		}
		if auditErr := s.audit.AppendBusinessTransition(txCtx, idccap.BusinessAuditTransition{
			IdempotencyScope: "product-fulfillment.repair_management", IdempotencyKey: input.IdempotencyKey,
			OperationCode: "product-fulfillment.repair_management", TargetType: "product_fulfillment_intent", TargetNo: input.IntentNo,
			CustomerNo: current.CustomerNo, ResultCode: "succeeded", ReasonCode: reasonManagementRepair, RiskCode: "high", RequiredPermission: PermissionPreparationRecover,
			BeforeSnapshot: map[string]any{"managementEvent": candidate.EventNo, "state": projectionManualAttention, "attempts": candidate.AttemptCount, "failureCode": entitlementprogress.SafeFailure(candidate.LastErrorCode), "workflowVersion": current.Version},
			AfterSnapshot:  map[string]any{"managementEvent": candidate.EventNo, "state": projectionPending, "attempts": 0, "workflowVersion": current.Version},
		}); auditErr != nil {
			return auditErr
		}
		detail, loadErr = s.loadDetail(txCtx, input.IntentNo, false)
		return loadErr
	})
	if err != nil {
		if _, business := bizerr.As(err); !business {
			return nil, bizerr.WrapCode(err, CodeManagementUnavailable)
		}
	}
	return detail, err
}
