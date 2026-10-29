// Native accessory lifecycle records verified resource-owner outcomes without
// creating legacy servers, commercial documents or a second provider executor.

package serveroperation

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// nativeAccessoryState identifies the retained object's lifecycle independently of its parent server.
type nativeAccessoryState string

// Native object states retain the same persisted values as legacy accessory entitlements.
const (
	nativeAccessoryPending  nativeAccessoryState = "pending_binding"
	nativeAccessoryActive   nativeAccessoryState = "active"
	nativeAccessoryUnpaid   nativeAccessoryState = "payment_pending"
	nativeAccessoryManual   nativeAccessoryState = "manual_review"
	nativeAccessoryDeleted  nativeAccessoryState = "deleted"
	nativeAccessoryFailed   nativeAccessoryState = "creation_failed"
	nativeAccessoryDeleting nativeAccessoryState = "deletion_pending"
)

// nativeAccessoryBilling identifies the separately billed object lifecycle.
type nativeAccessoryBilling string

const nativeAccessoryPaid nativeAccessoryBilling = "paid"

// nativeCreateState bounds the operation states used to reconcile reservations.
type nativeCreateState string

const (
	nativeCreateSucceeded nativeCreateState = "succeeded"
	nativeCreateFailed    nativeCreateState = "failed"
	nativeCreateCancelled nativeCreateState = "cancelled"
	nativeCreateExecuting nativeCreateState = "executing"
	nativeCreateManual    nativeCreateState = "manual_review"
)

// nativeCheckRestore uses the current paid period rather than relying on the
// renewal worker having already projected payment_pending. The resource owner
// independently proves that an untracked upstream object belongs to this server.
// An unbound executed create makes new untracked identities ambiguous; only a
// proven old binding may bypass that fence. Unsent purchases create no objects.
func nativeCheckRestore(ctx context.Context, database gdb.DB, input servercommercialcap.ObjectRequest) error {
	if !validReference(input.EntitlementNo, 40) || !validReference(input.CustomerNo, 40) ||
		!validReference(input.ObjectNo, 128) || (input.Kind != serveroperationresourcecap.AccessoryKindSnapshot && input.Kind != serveroperationresourcecap.AccessoryKindBackup) {
		return ErrInvalidInput
	}
	var row *struct{ Restorable bool }
	if err := database.GetScan(ctx, &row, `SELECT
(accessory.entitlement_no IS NULL AND NOT EXISTS(
 SELECT 1 FROM plugin_idc_core_server_accessory_entitlement unbound
 JOIN plugin_idc_core_business_operation operation ON operation.tenant_id=unbound.tenant_id AND operation.operation_no=unbound.create_operation_no
  AND operation.subject_type_code=unbound.subject_type_code AND operation.subject_no=unbound.subject_no
 WHERE unbound.tenant_id=owner.tenant_id AND unbound.subject_type_code='entitlement' AND unbound.subject_no=owner.entitlement_no
  AND unbound.customer_no=owner.customer_no AND unbound.accessory_kind=? AND unbound.object_no IS NULL
  AND unbound.status_code IN ('pending_binding','manual_review')
  AND (operation.status_code IN ('executing','succeeded') OR (operation.status_code='manual_review'
   AND (operation.provider_task_no IS NOT NULL OR operation.execution_started_at IS NOT NULL))))
 OR (accessory.customer_no=owner.customer_no AND accessory.status_code='active'
 AND (accessory.billing_mode_code='free' OR accessory.current_period_end_at>CURRENT_TIMESTAMP))) AS restorable
FROM plugin_idc_core_service_entitlement owner
LEFT JOIN plugin_idc_core_server_accessory_entitlement accessory
 ON accessory.tenant_id=owner.tenant_id AND accessory.subject_type_code='entitlement'
 AND accessory.subject_no=owner.entitlement_no
 AND accessory.accessory_kind=? AND accessory.object_no=?
WHERE owner.tenant_id=0 AND owner.entitlement_no=? AND owner.customer_no=?
 AND owner.entitlement_owner_id='idc-resource' AND owner.status_code='active'
 AND (owner.expires_at IS NULL OR owner.expires_at>CURRENT_TIMESTAMP)
 AND NOT EXISTS(SELECT 1 FROM plugin_idc_core_refund_entitlement_lock refund_lock
  WHERE refund_lock.tenant_id=owner.tenant_id AND refund_lock.entitlement_no=owner.entitlement_no AND refund_lock.released_at IS NULL)`, input.Kind, input.Kind, input.ObjectNo, input.EntitlementNo, input.CustomerNo); err != nil {
		return err
	}
	if row == nil || !row.Restorable {
		return ErrActionUnavailable
	}
	return nil
}

// nativeCompleteCreatedAccessory runs inside the caller's verified completion
// transaction, after its operation transition. A real local object number is
// required; a task number or a pre-existing baseline object cannot bind an intent.
func nativeCompleteCreatedAccessory(ctx context.Context, database gdb.DB, input servercommercialcap.Completion) error {
	if input.Status != servercommercialcap.CompletionSucceeded || !validReference(input.OperationNo, 40) ||
		!validReference(input.EntitlementNo, 40) || !validReference(input.ObjectNo, 128) ||
		!validNativeLifecycleDigest(input.EvidenceDigest) || input.ObservedAt.IsZero() {
		return ErrInvalidInput
	}
	if gdb.TXFromCtx(ctx, database.GetGroup()) == nil {
		return ErrOperationConflict
	}
	var row *struct {
		EntitlementNo, ObjectNo, PeriodUnitCode, BaselineObjectNos string
		StatusCode                                                 nativeAccessoryState
		BillingModeCode                                            nativeAccessoryBilling
		PeriodCount                                                int
	}
	if err := database.GetScan(ctx, &row, `SELECT accessory.entitlement_no,
COALESCE(accessory.object_no,'') AS object_no,accessory.status_code,accessory.billing_mode_code,
accessory.period_unit_code,accessory.period_count,accessory.baseline_object_nos::TEXT AS baseline_object_nos
FROM plugin_idc_core_server_accessory_entitlement accessory
JOIN plugin_idc_core_business_operation operation
 ON operation.tenant_id=accessory.tenant_id AND operation.operation_no=accessory.create_operation_no
 AND operation.subject_type_code=accessory.subject_type_code AND operation.subject_no=accessory.subject_no
 AND operation.customer_no=accessory.customer_no AND operation.currency_code=accessory.currency_code
WHERE accessory.tenant_id=0 AND accessory.subject_type_code='entitlement' AND accessory.subject_no=?
 AND accessory.create_operation_no=? AND operation.operation_type_code='addon' AND operation.status_code='succeeded'
 AND (NOT operation.financial_settlement_required OR
      (operation.financially_settled_at IS NOT NULL AND operation.settlement_no IS NOT NULL))
FOR UPDATE OF accessory`, input.EntitlementNo, input.OperationNo); err != nil {
		return err
	}
	if row == nil {
		return ErrOperationConflict
	}
	if row.StatusCode != nativeAccessoryPending {
		if row.ObjectNo == input.ObjectNo && (row.StatusCode == nativeAccessoryActive || row.StatusCode == nativeAccessoryUnpaid || row.StatusCode == nativeAccessoryDeleted) {
			return nil
		}
		return ErrOperationConflict
	}
	var baseline []string
	if json.Unmarshal([]byte(row.BaselineObjectNos), &baseline) != nil {
		return ErrInvalidInput
	}
	for _, objectNo := range baseline {
		if objectNo == input.ObjectNo {
			return ErrOperationConflict
		}
	}
	var start, end any
	if row.BillingModeCode == nativeAccessoryPaid {
		periodStart := databaseTime(input.ObservedAt)
		periodEnd, ok := addAccessoryPeriod(periodStart, row.PeriodUnitCode, row.PeriodCount)
		if !ok {
			return ErrInvalidInput
		}
		start, end = periodStart, periodEnd
	}
	if err := execOne(ctx, database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET object_no=?,status_code='active',current_period_start_at=?,current_period_end_at=?,
binding_attempt_count=binding_attempt_count+1,last_binding_attempt_at=CURRENT_TIMESTAMP,
version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND status_code='pending_binding'`, input.ObjectNo, start, end, row.EntitlementNo); err != nil {
		return err
	}
	eventNo, err := deterministicReference("aee_", row.EntitlementNo+":bound:"+input.ObjectNo)
	if err != nil {
		return err
	}
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,from_status_code,to_status_code,operation_no,evidence_digest,occurred_at)
VALUES(0,?,?,'object_bound','pending_binding','active',?,?,?) ON CONFLICT(event_no) DO NOTHING`,
		eventNo, row.EntitlementNo, input.OperationNo, input.EvidenceDigest, databaseTime(input.ObservedAt))
	return err
}

// nativeCompleteDeletedAccessory accepts only the source's proven successful
// deletion, and atomically stops renewal without refunding an already-started period.
func nativeCompleteDeletedAccessory(ctx context.Context, database gdb.DB, object servercommercialcap.ObjectRequest, input servercommercialcap.Completion) error {
	if input.Status != servercommercialcap.CompletionSucceeded || input.EntitlementNo != object.EntitlementNo || input.ObjectNo != object.ObjectNo ||
		!validReference(input.OperationNo, 40) || !validReference(object.EntitlementNo, 40) ||
		!validReference(object.CustomerNo, 40) || !validReference(object.ObjectNo, 128) ||
		!validNativeLifecycleDigest(input.EvidenceDigest) || input.ObservedAt.IsZero() ||
		(object.Kind != serveroperationresourcecap.AccessoryKindSnapshot && object.Kind != serveroperationresourcecap.AccessoryKindBackup) {
		return ErrInvalidInput
	}
	if gdb.TXFromCtx(ctx, database.GetGroup()) == nil {
		return ErrOperationConflict
	}
	var row *struct {
		EntitlementNo string
		StatusCode    nativeAccessoryState
		InvoiceReady  bool
	}
	if err := database.GetScan(ctx, &row, `SELECT entitlement_no,status_code,
(pending_invoice_no IS NULL OR EXISTS(SELECT 1 FROM plugin_idc_core_billing_invoice invoice
 WHERE invoice.tenant_id=accessory.tenant_id AND invoice.invoice_no=accessory.pending_invoice_no
 AND invoice.customer_no=accessory.customer_no AND invoice.currency_code=accessory.currency_code
 AND invoice.status_code IN ('paid','cancelled'))) AS invoice_ready
FROM plugin_idc_core_server_accessory_entitlement accessory
WHERE tenant_id=0 AND subject_type_code='entitlement' AND subject_no=? AND customer_no=?
 AND accessory_kind=? AND object_no=? FOR UPDATE`, object.EntitlementNo, object.CustomerNo, object.Kind, object.ObjectNo); err != nil {
		return err
	}
	if row == nil || row.StatusCode == nativeAccessoryDeleted {
		return nil
	}
	if !row.InvoiceReady {
		return ErrOperationConflict
	}
	if err := execOne(ctx, database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET status_code='deleted',auto_renew_enabled=FALSE,pending_invoice_no=NULL,pending_policy_no=NULL,
pending_policy_version=NULL,pending_period_unit_code=NULL,pending_period_count=NULL,pending_total_amount_minor=NULL,
last_renewal_attempt_at=NULL,deleted_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND status_code=?`, databaseTime(input.ObservedAt), row.EntitlementNo, row.StatusCode); err != nil {
		return err
	}
	if _, err := database.Exec(ctx, `UPDATE plugin_idc_core_server_accessory_renewal_work_item
SET status_code='resolved',resolved_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND status_code='open'`, row.EntitlementNo); err != nil {
		return err
	}
	eventNo, err := deterministicReference("aee_", row.EntitlementNo+":deleted:"+input.OperationNo)
	if err != nil {
		return err
	}
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,from_status_code,to_status_code,operation_no,evidence_digest,occurred_at)
VALUES(0,?,?,'deleted',?,'deleted',?,?,?) ON CONFLICT(event_no) DO NOTHING`, eventNo,
		row.EntitlementNo, row.StatusCode, input.OperationNo, input.EvidenceDigest, databaseTime(input.ObservedAt))
	return err
}

// validNativeLifecycleDigest accepts only the canonical digest stored in audit evidence.
func validNativeLifecycleDigest(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}
