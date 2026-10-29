// Ordinary native writes persist admission before provider I/O. The resource
// owner claims the exact durable task lease in the same transaction; a repeated
// fence alone never authorizes another provider invocation. Parent locks precede
// fence locks, matching refund and ownership serialization.

package serveroperation

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// nativeQueuedWriteState distinguishes the nonterminal provider execution fence.
type nativeQueuedWriteState string

// nativeQueuedWriteExecuting is admission committed before any provider call.
const nativeQueuedWriteExecuting nativeQueuedWriteState = "executing"

// nativeQueuedWriteRow is the immutable resource task identity and receipt.
type nativeQueuedWriteRow struct {
	OperationNo, TaskNo, EntitlementNo, CustomerNo, RequestDigest string
	ExpectedEntitlementVersion                                    int64
	ActionCode                                                    serveroperationresourcecap.Action
	StatusCode                                                    nativeQueuedWriteState
	EvidenceDigest                                                string
}

// HoldQueuedWrite persists an ordinary write only within the caller's shared
// transaction. The source must also atomically claim its current task lease.
func (s *nativeCommercialService) HoldQueuedWrite(ctx context.Context, input servercommercialcap.QueuedWrite) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || !validNativeQueuedWrite(input) {
		return PublicError(ErrInvalidInput)
	}
	if gdb.TXFromCtx(ctx, s.database.GetGroup()) == nil {
		return PublicError(ErrOperationConflict)
	}
	var parent *struct{ Admissible bool }
	if err := s.database.GetScan(ctx, &parent, `SELECT (owner.customer_no=? AND owner.entitlement_owner_id='idc-resource'
 AND owner.version=? AND owner.status_code='active' AND (owner.expires_at IS NULL OR owner.expires_at>CURRENT_TIMESTAMP)) AS admissible
FROM plugin_idc_core_service_entitlement owner WHERE owner.tenant_id=0 AND owner.entitlement_no=? FOR UPDATE OF owner`,
		input.CustomerNo, input.ExpectedEntitlementVersion, input.EntitlementNo); err != nil {
		return PublicError(err)
	}
	if parent == nil || !parent.Admissible {
		return PublicError(ErrActionUnavailable)
	}
	// This statement runs after acquiring the parent lock: a concurrently
	// committed refund lock must be visible before a new fence is inserted.
	locked, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_refund_entitlement_lock
WHERE tenant_id=0 AND entitlement_no=? AND released_at IS NULL)
OR EXISTS(SELECT 1 FROM plugin_idc_core_business_operation operation
 WHERE operation.tenant_id=0 AND operation.subject_type_code='entitlement' AND operation.subject_no=?
 AND operation.service_no IS NULL AND operation.resource_owner_code='idc-resource'
 AND operation.operation_type_code IN ('addon','resize','ip_change')
 AND operation.status_code IN ('awaiting_financial_settlement','financially_settled','ready','executing','manual_review'))`, input.EntitlementNo, input.EntitlementNo)
	if err != nil {
		return PublicError(err)
	}
	if locked.Bool() {
		return PublicError(ErrActionUnavailable)
	}
	row, err := nativeReadQueuedWrite(ctx, s.database, input.TaskNo)
	if err != nil {
		return PublicError(err)
	}
	if row != nil {
		if row.StatusCode == nativeQueuedWriteExecuting && nativeQueuedIdentityMatches(row, input) {
			return nil
		}
		return PublicError(ErrOperationConflict)
	}
	busy, err := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_native_server_write_fence
WHERE tenant_id=0 AND entitlement_no=? AND status_code IN ('executing','result_unknown'))`, input.EntitlementNo)
	if err != nil {
		return PublicError(err)
	}
	if busy.Bool() {
		return PublicError(ErrOperationConflict)
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_native_server_write_fence
(task_no,operation_no,entitlement_no,customer_no,expected_entitlement_version,action_code,request_digest)
VALUES(?,?,?,?,?,?,?)`, input.TaskNo, input.OperationNo, input.EntitlementNo, input.CustomerNo,
		input.ExpectedEntitlementVersion, input.Action, input.RequestDigest)
	return PublicError(err)
}

// CompleteQueuedWrite retains uncertain execution and accepts only a verified
// source receipt. Terminal outcomes cannot change identity, status or evidence.
// Unknown observations replace the current receipt projection while retaining
// the fence; the resource owner's task events preserve observation history.
// An expired or suspended parent does not prevent receipt recovery.
func (s *nativeCommercialService) CompleteQueuedWrite(ctx context.Context, input servercommercialcap.QueuedWriteCompletion) error {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || !validReference(input.TaskNo, 64) ||
		!validReference(input.EntitlementNo, 40) || !validReference(input.CustomerNo, 40) || !validNativeLifecycleDigest(input.EvidenceDigest) ||
		(input.Status != servercommercialcap.CompletionSucceeded && input.Status != servercommercialcap.CompletionFailed && input.Status != servercommercialcap.CompletionUnknown) {
		return PublicError(ErrInvalidInput)
	}
	return PublicError(s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var parent *struct{ EntitlementNo string }
		if err := s.database.GetScan(ctx, &parent, `SELECT entitlement_no FROM plugin_idc_core_service_entitlement
WHERE tenant_id=0 AND entitlement_no=? FOR UPDATE`, input.EntitlementNo); err != nil {
			return err
		}
		if parent == nil {
			return ErrOperationConflict
		}
		row, err := nativeReadQueuedWrite(ctx, s.database, input.TaskNo)
		if err != nil {
			return err
		}
		if row == nil || row.EntitlementNo != input.EntitlementNo || row.CustomerNo != input.CustomerNo {
			return ErrOperationConflict
		}
		if row.StatusCode == nativeQueuedWriteState(input.Status) && row.EvidenceDigest == input.EvidenceDigest {
			return nil
		}
		if row.StatusCode == nativeQueuedWriteState(servercommercialcap.CompletionSucceeded) || row.StatusCode == nativeQueuedWriteState(servercommercialcap.CompletionFailed) {
			return ErrOperationConflict
		}
		return execOne(ctx, s.database, `UPDATE plugin_idc_core_native_server_write_fence
SET status_code=?,evidence_digest=?,updated_at=clock_timestamp(),
completed_at=CASE WHEN ? IN ('succeeded','failed') THEN clock_timestamp() ELSE NULL END
WHERE tenant_id=0 AND task_no=?`, input.Status, input.EvidenceDigest, input.Status, input.TaskNo)
	}))
}

// ListQueuedWrites rotates one bounded batch for source receipt reconciliation.
// It locks only fence rows, never acquires a parent lock after them, and performs
// one set update so uncertain old tasks cannot starve later completed tasks.
func (s *nativeCommercialService) ListQueuedWrites(ctx context.Context, limit int) ([]servercommercialcap.QueuedWrite, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || limit < 1 || limit > 100 {
		return nil, PublicError(ErrInvalidInput)
	}
	if gdb.TXFromCtx(ctx, s.database.GetGroup()) != nil {
		return nil, PublicError(ErrOperationConflict)
	}
	var rows []nativeQueuedWriteRow
	err := s.database.GetScan(ctx, &rows, `WITH candidates AS (
 SELECT task_no FROM plugin_idc_core_native_server_write_fence
 WHERE tenant_id=0 AND status_code IN ('executing','result_unknown')
 ORDER BY updated_at,task_no LIMIT ? FOR UPDATE SKIP LOCKED)
UPDATE plugin_idc_core_native_server_write_fence fence SET updated_at=clock_timestamp()
FROM candidates WHERE fence.tenant_id=0 AND fence.task_no=candidates.task_no
RETURNING fence.operation_no,fence.task_no,fence.entitlement_no,fence.customer_no,
fence.expected_entitlement_version,fence.action_code,fence.request_digest`, limit)
	if err != nil {
		return nil, PublicError(err)
	}
	result := make([]servercommercialcap.QueuedWrite, 0, len(rows))
	for _, row := range rows {
		result = append(result, servercommercialcap.QueuedWrite{OperationNo: row.OperationNo, TaskNo: row.TaskNo,
			EntitlementNo: row.EntitlementNo, CustomerNo: row.CustomerNo, ExpectedEntitlementVersion: row.ExpectedEntitlementVersion,
			Action: row.ActionCode, RequestDigest: row.RequestDigest})
	}
	return result, nil
}

// nativeReadQueuedWrite locks the persisted identity after the parent is locked.
func nativeReadQueuedWrite(ctx context.Context, database gdb.DB, taskNo string) (*nativeQueuedWriteRow, error) {
	query := `SELECT operation_no,task_no,entitlement_no,customer_no,expected_entitlement_version,action_code,request_digest,status_code,
COALESCE(evidence_digest,'') AS evidence_digest FROM plugin_idc_core_native_server_write_fence WHERE tenant_id=0 AND task_no=? FOR UPDATE`
	var row *nativeQueuedWriteRow
	err := database.GetScan(ctx, &row, query, taskNo)
	return row, err
}

// nativeQueuedIdentityMatches prevents replay under a changed immutable task.
func nativeQueuedIdentityMatches(row *nativeQueuedWriteRow, input servercommercialcap.QueuedWrite) bool {
	return row.OperationNo == input.OperationNo && row.TaskNo == input.TaskNo && row.EntitlementNo == input.EntitlementNo &&
		row.CustomerNo == input.CustomerNo && row.ExpectedEntitlementVersion == input.ExpectedEntitlementVersion &&
		row.ActionCode == input.Action && row.RequestDigest == input.RequestDigest
}

// validNativeQueuedWrite excludes all separately priced commercial actions.
func validNativeQueuedWrite(input servercommercialcap.QueuedWrite) bool {
	if !validReference(input.OperationNo, 40) || !strings.HasPrefix(input.OperationNo, "nso_") || !validReference(input.TaskNo, 64) ||
		!validReference(input.EntitlementNo, 40) || !validReference(input.CustomerNo, 40) || input.ExpectedEntitlementVersion < 1 || !validNativeLifecycleDigest(input.RequestDigest) {
		return false
	}
	switch input.Action {
	case serveroperationresourcecap.ActionStart, serveroperationresourcecap.ActionStop, serveroperationresourcecap.ActionReboot,
		serveroperationresourcecap.ActionResetPassword, serveroperationresourcecap.ActionReinstall, serveroperationresourcecap.ActionEnterRescue,
		serveroperationresourcecap.ActionExitRescue, serveroperationresourcecap.ActionConsole, serveroperationresourcecap.ActionSecurityGroupAttach,
		serveroperationresourcecap.ActionSecurityGroupDetach, serveroperationresourcecap.ActionNATCreate, serveroperationresourcecap.ActionNATDelete,
		serveroperationresourcecap.ActionVirtualMediaMount, serveroperationresourcecap.ActionVirtualMediaUnmount, serveroperationresourcecap.ActionSnapshotDelete,
		serveroperationresourcecap.ActionSnapshotRestore, serveroperationresourcecap.ActionBackupDelete, serveroperationresourcecap.ActionBackupRestore:
		return true
	default:
		return false
	}
}
