// Completion consumes durable resource receipts; it never infers success from
// an HTTP response or releases an unknown operation's object/IP reservation.
package serveroperation

import (
	"context"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"time"
)

func (s *nativeCommercialService) CheckRestore(ctx context.Context, input servercommercialcap.ObjectRequest) error {
	action := serveroperationresourcecap.ActionSnapshotRestore
	if input.Kind == serveroperationresourcecap.AccessoryKindBackup {
		action = serveroperationresourcecap.ActionBackupRestore
	}
	if err := s.authorizeNativeTarget(ctx, input.CustomerNo, action); err != nil {
		return err
	}
	return PublicError(nativeCheckRestore(ctx, s.database, input))
}

func (s *nativeCommercialService) Complete(ctx context.Context, input servercommercialcap.Completion) error {
	if !validNativeCompletion(ctx, input) {
		return PublicError(ErrInvalidInput)
	}
	return PublicError(s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.lockNativeExecution(ctx, input.OperationNo)
		if err != nil {
			return err
		}
		if row.SubjectNo != input.EntitlementNo || row.ProviderTaskNo != input.TaskNo {
			return ErrOperationConflict
		}
		var to string
		switch input.Status {
		case servercommercialcap.CompletionSucceeded:
			to = "succeeded"
		case servercommercialcap.CompletionFailed:
			to = "failed"
		case servercommercialcap.CompletionUnknown:
			to = "manual_review"
		default:
			return ErrInvalidInput
		}
		receipt, err := digestValue(input)
		if err != nil {
			return err
		}
		if row.StatusCode == to {
			match, readErr := s.database.GetValue(ctx, `SELECT EXISTS(SELECT 1 FROM plugin_idc_core_business_operation_event WHERE tenant_id=0 AND operation_no=? AND to_status_code=? AND actor_reference=? AND evidence_digest=?)`, input.OperationNo, to, input.TaskNo, receipt)
			if readErr != nil {
				return readErr
			}
			if !match.Bool() {
				return ErrOperationConflict
			}
			return nil
		}
		if row.StatusCode != "executing" && row.StatusCode != "manual_review" {
			return ErrOperationConflict
		}
		var completed any
		if to == "succeeded" || to == "failed" {
			completed = time.Now().UTC()
		}
		if err = execOne(ctx, s.database, `UPDATE plugin_idc_core_business_operation SET status_code=?,completed_at=?,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND operation_no=? AND status_code=? AND version=?`, to, completed, input.OperationNo, row.StatusCode, row.Version); err != nil {
			return err
		}
		if err = appendNativeAccessoryEvent(ctx, s.database, input.OperationNo, row.SubjectNo, row.Version+1, row.StatusCode, to, "server_commercial.resource_result", input.TaskNo, receipt); err != nil {
			return err
		}
		if input.Status == servercommercialcap.CompletionSucceeded && (row.Operation.Action == serveroperationresourcecap.ActionSnapshotCreate || row.Operation.Action == serveroperationresourcecap.ActionBackupCreate) {
			if err = nativeCompleteCreatedAccessory(ctx, s.database, input); err != nil {
				return err
			}
		}
		if input.Status == servercommercialcap.CompletionFailed && (row.Operation.Action == serveroperationresourcecap.ActionSnapshotCreate || row.Operation.Action == serveroperationresourcecap.ActionBackupCreate) {
			if err = nativeCompleteFailedAccessory(ctx, s.database, input); err != nil {
				return err
			}
		}
		if row.Priced != nil {
			return s.nativeCompletePriced(ctx, input)
		}
		return nil
	}))
}

func validNativeCompletion(ctx context.Context, input servercommercialcap.Completion) bool {
	return ctx != nil && ctx.Err() == nil && validReference(input.OperationNo, 40) && validReference(input.EntitlementNo, 40) && validReference(input.TaskNo, 64) && validNativeLifecycleDigest(input.EvidenceDigest) && !input.ObservedAt.IsZero() && !input.ObservedAt.After(time.Now().UTC().Add(time.Minute))
}

func (s *nativeCommercialService) CompleteDeletion(ctx context.Context, input servercommercialcap.Deletion) error {
	completion := servercommercialcap.Completion{OperationNo: input.OperationNo, EntitlementNo: input.EntitlementNo, TaskNo: input.TaskNo, ObjectNo: input.ObjectNo, EvidenceDigest: input.EvidenceDigest, ObservedAt: input.ObservedAt, Status: servercommercialcap.CompletionSucceeded}
	if !validNativeCompletion(ctx, completion) {
		return PublicError(ErrInvalidInput)
	}
	pending := false
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var closeErr error
		pending, closeErr = nativeClosePendingRenewalInvoice(ctx, s.database, s.invoices, input.ObjectRequest, completion)
		if closeErr != nil {
			return closeErr
		}
		if pending {
			return nil
		}
		return nativeCompleteDeletedAccessory(ctx, s.database, input.ObjectRequest, completion)
	})
	if err == nil && pending {
		err = ErrOperationConflict
	}
	return PublicError(err)
}
