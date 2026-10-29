// Task-bound IP capacity remains occupied across quote expiry and uncertain
// provider results. This file never performs a supplier write.
package serveroperationsource

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

var _ serveroperationresourcecap.IPExecutionHold = (*Service)(nil)

func (s *Service) HoldIP(ctx context.Context, request serveroperationresourcecap.IPReservationHoldRequest) (*serveroperationresourcecap.IPReservation, error) {
	if s == nil || s.database == nil || ctx == nil || ctx.Err() != nil || !validIPReference(request.ReservationNo) || !validIPReference(request.ProviderTaskNo) {
		return nil, serveroperationresourcecap.ErrIPReservationConflict
	}
	var result *serveroperationresourcecap.IPReservation
	err := s.database.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		// Match ReserveIP's pool-before-reservation lock order and serialize expiry.
		pool, err := s.database.GetValue(txctx, `SELECT pool.id FROM plugin_idc_resource_pool pool
JOIN plugin_idc_resource_ip_reservation reservation ON reservation.tenant_id=pool.tenant_id AND reservation.channel_no=pool.channel_no AND reservation.pool_no=pool.pool_no
WHERE reservation.tenant_id=0 AND reservation.reservation_no=? FOR UPDATE OF pool`, request.ReservationNo)
		if err != nil || pool.Int64() <= 0 {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		var row *ipReservationRow
		if err = s.database.GetScan(txctx, &row, `SELECT reservation_no,capacity_observation_no,quantity,status_code,request_digest,expires_at,COALESCE(provider_task_no,'') provider_task_no
FROM plugin_idc_resource_ip_reservation WHERE tenant_id=0 AND reservation_no=? FOR UPDATE`, request.ReservationNo); err != nil || row == nil {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		if row.Status == string(serveroperationresourcecap.IPReservationInFlight) {
			if row.ProviderTaskNo != request.ProviderTaskNo {
				return serveroperationresourcecap.ErrIPReservationConflict
			}
			result = projectIPReservation(row, true)
			return nil
		}
		if row.Status != string(serveroperationresourcecap.IPReservationReserved) && row.Status != string(serveroperationresourcecap.IPReservationCommercialHold) {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		updated, err := s.database.Exec(txctx, `UPDATE plugin_idc_resource_ip_reservation reservation
SET status_code='in_flight',provider_task_no=?,updated_at=CURRENT_TIMESTAMP,version=reservation.version+1
FROM plugin_idc_resource_task task
WHERE reservation.tenant_id=0 AND reservation.reservation_no=? AND (reservation.status_code='commercial_hold' OR (reservation.status_code='reserved' AND reservation.expires_at>CURRENT_TIMESTAMP))
AND task.tenant_id=reservation.tenant_id AND task.task_no=? AND task.local_resource_no=reservation.service_no
AND task.channel_no=reservation.channel_no AND task.pool_no=reservation.pool_no AND task.capability_code='server.ip.change'
AND (reservation.commercial_operation_no IS NULL OR task.owner_metadata->>'operationNo'=reservation.commercial_operation_no)
AND task.status_code='pending' AND task.attempt_count=0 AND task.result_unknown=false`, request.ProviderTaskNo, request.ReservationNo, request.ProviderTaskNo)
		if err != nil {
			return err
		}
		count, err := updated.RowsAffected()
		if err != nil || count != 1 {
			return serveroperationresourcecap.ErrIPReservationConflict
		}
		row.Status = string(serveroperationresourcecap.IPReservationInFlight)
		result = projectIPReservation(row, false)
		return nil
	})
	return result, err
}

func (s *Service) validateIPTaskOutcome(ctx context.Context, taskNo string, consume bool) error {
	var task *struct {
		StatusCode    string
		ResultUnknown bool
	}
	if err := s.database.GetScan(ctx, &task, `SELECT status_code,result_unknown FROM plugin_idc_resource_task WHERE tenant_id=0 AND task_no=? AND capability_code='server.ip.change' FOR SHARE`, taskNo); err != nil || task == nil || task.ResultUnknown {
		return serveroperationresourcecap.ErrIPReservationConflict
	}
	if (consume && task.StatusCode == "succeeded") || (!consume && (task.StatusCode == "failed" || task.StatusCode == "canceled")) {
		return nil
	}
	return serveroperationresourcecap.ErrIPReservationConflict
}
