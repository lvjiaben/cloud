// Refund recovery consumes the billing owner's immutable completion proof and
// releases only never-dispatched native capacity inside the same transaction.

package serveroperation

import (
	"context"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

type nativeReadyRefundEvidence = billing.CompletedRefundEvidence

// Cancellation precedes provider creation and is distinct from a failed or
// deleted real object. Its financial settlement remains immutable.
const nativeAccessoryCancelled nativeAccessoryState = "creation_cancelled"

const nativeReadyIPReleaseReason = "core_completed_refund"

func (s *nativeCommercialService) nativeReadyRefundProof(ctx context.Context, row *nativeExecutionRow) (nativeReadyRefundEvidence, error) {
	if row.OrderNo == "" {
		return nativeReadyRefundEvidence{}, nil
	}
	if s.refunds == nil {
		return nativeReadyRefundEvidence{}, ErrServiceUnavailable
	}
	proof, err := s.refunds.ReadCompletedRefund(ctx, billing.CompletedRefundScope{OrderNo: row.OrderNo, InvoiceNo: row.InvoiceNo, CustomerNo: row.CustomerNo, CurrencyCode: row.CurrencyCode})
	if err != nil {
		return nativeReadyRefundEvidence{}, err
	}
	if proof == nil || !validNativeLifecycleDigest(proof.EvidenceDigest) {
		return nativeReadyRefundEvidence{}, ErrOperationConflict
	}
	return *proof, nil
}

func (s *nativeCommercialService) cancelRefundedNativeReady(ctx context.Context, row *nativeExecutionRow, proof nativeReadyRefundEvidence) error {
	if err := execOne(ctx, s.database, `UPDATE plugin_idc_core_business_operation
SET status_code='cancelled',last_error_code='',completed_at=CURRENT_TIMESTAMP,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND version=? AND status_code IN ('ready','manual_review') AND provider_task_no IS NULL AND execution_started_at IS NULL`, row.OperationNo, row.Version); err != nil {
		return err
	}
	if err := appendNativeAccessoryEvent(ctx, s.database, row.OperationNo, row.SubjectNo, row.Version+1, row.StatusCode, string(nativeCreateCancelled), nativeReadyRefunded, row.InvoiceNo, proof.EvidenceDigest); err != nil {
		return err
	}
	if err := nativeCancelUnboundAccessory(ctx, s, row, proof.EvidenceDigest); err != nil {
		return err
	}
	if row.Priced == nil || row.Priced.Price.Reservation == nil {
		return nil
	}
	if s.prices == nil {
		return ErrServiceUnavailable
	}
	capacity, ok := s.prices.source.(serveroperationresourcecap.IPCommercialHold)
	if !ok {
		return ErrServiceUnavailable
	}
	number := row.Priced.Price.Reservation.ReservationNo
	released, err := capacity.ReleaseIPCommercial(ctx, serveroperationresourcecap.IPCommercialReleaseRequest{ReservationNo: number, OperationNo: row.OperationNo, ReasonCode: nativeReadyIPReleaseReason})
	if err != nil {
		return err
	}
	if released == nil || released.ReservationNo != number || released.Status != serveroperationresourcecap.IPReservationReleased {
		return ErrOperationConflict
	}
	return nil
}

// nativeCancelUnboundAccessory records cancellation before creation, preserving
// paid evidence and avoiding a fictional provider failure or deletion receipt.
func nativeCancelUnboundAccessory(ctx context.Context, s *nativeCommercialService, op *nativeExecutionRow, evidence string) error {
	var row *struct {
		EntitlementNo, StatusCode string
		ObjectNo                  *string
	}
	if err := s.database.GetScan(ctx, &row, `SELECT entitlement_no,status_code,object_no
FROM plugin_idc_core_server_accessory_entitlement WHERE tenant_id=0 AND create_operation_no=?
 AND subject_type_code='entitlement' AND subject_no=? AND customer_no=? FOR UPDATE`, op.OperationNo, op.SubjectNo, op.CustomerNo); err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	if row.ObjectNo != nil || nativeAccessoryState(row.StatusCode) != nativeAccessoryPending {
		return ErrOperationConflict
	}
	if err := execOne(ctx, s.database, `UPDATE plugin_idc_core_server_accessory_entitlement
SET status_code='creation_cancelled',auto_renew_enabled=FALSE,version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND status_code='pending_binding' AND object_no IS NULL`, row.EntitlementNo); err != nil {
		return err
	}
	event, err := deterministicReference("aee_", row.EntitlementNo+":creation-cancelled:"+op.OperationNo)
	if err != nil {
		return err
	}
	_, err = s.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_accessory_entitlement_event
(tenant_id,event_no,entitlement_no,event_type_code,from_status_code,to_status_code,operation_no,evidence_digest,occurred_at)
VALUES(0,?,?,?, ?,?,?,?,CURRENT_TIMESTAMP)`, event, row.EntitlementNo, nativeAccessoryCancelled, nativeAccessoryPending, nativeAccessoryCancelled, op.OperationNo, evidence)
	return err
}
