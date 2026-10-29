// Verified creation failures release only an unbound reservation. Paid purchase
// evidence stays intact; unknown outcomes never reach this terminal transition.

package serveroperation

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
)

// nativeCompleteFailedAccessory follows the root's verified failed operation
// transition in the same transaction and requires affirmative no-object evidence.
func nativeCompleteFailedAccessory(ctx context.Context, database gdb.DB, input servercommercialcap.Completion) error {
	if input.Status != servercommercialcap.CompletionFailed || input.ObjectNo != "" ||
		!validReference(input.OperationNo, 40) || !validReference(input.EntitlementNo, 40) ||
		!validNativeLifecycleDigest(input.EvidenceDigest) || input.ObservedAt.IsZero() ||
		gdb.TXFromCtx(ctx, database.GetGroup()) == nil {
		return ErrOperationConflict
	}
	var row *struct {
		EntitlementNo string
		StatusCode    nativeAccessoryState
	}
	if err := database.GetScan(ctx, &row, `SELECT accessory.entitlement_no,accessory.status_code
FROM plugin_idc_core_server_accessory_entitlement accessory
JOIN plugin_idc_core_business_operation operation
 ON operation.tenant_id=accessory.tenant_id AND operation.operation_no=accessory.create_operation_no
 AND operation.subject_type_code=accessory.subject_type_code AND operation.subject_no=accessory.subject_no
WHERE accessory.tenant_id=0 AND accessory.subject_type_code='entitlement' AND accessory.subject_no=?
 AND accessory.create_operation_no=? AND accessory.object_no IS NULL AND operation.status_code='failed'
FOR UPDATE OF accessory`, input.EntitlementNo, input.OperationNo); err != nil {
		return err
	}
	if row == nil {
		return ErrOperationConflict
	}
	if row.StatusCode == nativeAccessoryFailed {
		return nil
	}
	if row.StatusCode != nativeAccessoryPending {
		return ErrOperationConflict
	}
	if err := execOne(ctx, database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET status_code='creation_failed',auto_renew_enabled=FALSE,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND status_code='pending_binding' AND object_no IS NULL`, row.EntitlementNo); err != nil {
		return err
	}
	eventNo, err := deterministicReference("aee_", row.EntitlementNo+":creation-failed:"+input.OperationNo)
	if err != nil {
		return err
	}
	_, err = database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,from_status_code,to_status_code,operation_no,evidence_digest,occurred_at)
VALUES(0,?,?,'creation_failed','pending_binding','creation_failed',?,?,?) ON CONFLICT(event_no) DO NOTHING`,
		eventNo, row.EntitlementNo, input.OperationNo, input.EvidenceDigest, databaseTime(input.ObservedAt))
	return err
}
