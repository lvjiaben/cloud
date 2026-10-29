package serveroperationsource

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

var _ serveroperationresourcecap.IPCommercialHold = (*Service)(nil)

// BindIPCommercial converts a short-lived quote reservation to an owner-bound
// checkout hold. It survives payment and scheduling delays without pretending
// that the supplier has already changed any address.
func (s *Service) BindIPCommercial(ctx context.Context, request serveroperationresourcecap.IPCommercialHoldRequest) (*serveroperationresourcecap.IPReservation, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || !validIPReference(request.ReservationNo) || !validDirectoryReference(request.OperationNo, 40) {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	var result *serveroperationresourcecap.IPReservation
	err := s.database.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		pool, err := s.database.GetValue(txctx, `SELECT pool.id FROM plugin_idc_resource_pool pool
JOIN plugin_idc_resource_ip_reservation reservation ON reservation.tenant_id=pool.tenant_id AND reservation.channel_no=pool.channel_no AND reservation.pool_no=pool.pool_no
WHERE reservation.tenant_id=0 AND reservation.reservation_no=? FOR UPDATE OF pool`, request.ReservationNo)
		if err != nil || pool.Int64() <= 0 {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		row, err := s.commercialIPRow(txctx, request.ReservationNo)
		if err != nil {
			return err
		}
		if row.CommercialOperationNo == request.OperationNo && (row.Status == string(serveroperationresourcecap.IPReservationCommercialHold) || row.Status == string(serveroperationresourcecap.IPReservationInFlight) || row.Status == string(serveroperationresourcecap.IPReservationConsumed)) {
			result = projectIPReservation(row, true)
			return nil
		}
		if row.CommercialOperationNo != "" || row.Status != string(serveroperationresourcecap.IPReservationReserved) {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		updated, err := s.database.Exec(txctx, `UPDATE plugin_idc_resource_ip_reservation SET status_code='commercial_hold',commercial_operation_no=?,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE tenant_id=0 AND reservation_no=? AND status_code='reserved' AND expires_at>CURRENT_TIMESTAMP`, request.OperationNo, request.ReservationNo)
		if err != nil {
			return err
		}
		count, err := updated.RowsAffected()
		if err != nil || count != 1 {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		row.Status = string(serveroperationresourcecap.IPReservationCommercialHold)
		result = projectIPReservation(row, false)
		return nil
	})
	return result, err
}

// ReleaseIPCommercial accepts only the core owner's final cancellation/refund
// or proven-unsent outcome. Task-bound holds use the resource outcome check.
func (s *Service) ReleaseIPCommercial(ctx context.Context, request serveroperationresourcecap.IPCommercialReleaseRequest) (*serveroperationresourcecap.IPReservation, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || !validIPReference(request.ReservationNo) || !validDirectoryReference(request.OperationNo, 40) {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	switch request.ReasonCode {
	case "core_order_cancelled", "core_completed_refund", "core_unsent_rejected":
	default:
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	var result *serveroperationresourcecap.IPReservation
	err := s.database.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		row, err := s.commercialIPRow(txctx, request.ReservationNo)
		if err != nil {
			return err
		}
		if row.CommercialOperationNo != request.OperationNo {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		if row.Status == string(serveroperationresourcecap.IPReservationReleased) && row.ReleaseReason == request.ReasonCode {
			result = projectIPReservation(row, true)
			return nil
		}
		if row.Status != string(serveroperationresourcecap.IPReservationCommercialHold) || row.ProviderTaskNo != "" {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		if _, err = s.database.Exec(txctx, `UPDATE plugin_idc_resource_ip_reservation SET status_code='released',release_reason_code=?,released_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE tenant_id=0 AND reservation_no=?`, request.ReasonCode, request.ReservationNo); err != nil {
			return err
		}
		row.Status = string(serveroperationresourcecap.IPReservationReleased)
		result = projectIPReservation(row, false)
		return nil
	})
	return result, err
}

func (s *Service) commercialIPRow(ctx context.Context, number string) (*ipReservationRow, error) {
	var row *ipReservationRow
	err := s.database.GetScan(ctx, &row, `SELECT reservation_no,capacity_observation_no,quantity,status_code,request_digest,expires_at,COALESCE(provider_task_no,'') provider_task_no,COALESCE(commercial_operation_no,'') commercial_operation_no,COALESCE(release_reason_code,'') release_reason_code FROM plugin_idc_resource_ip_reservation WHERE tenant_id=0 AND reservation_no=? FOR UPDATE`, number)
	if err != nil || row == nil {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	return row, nil
}
