package serveroperation

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type ipReservationWork struct {
	OperationNo, ServiceNo, ReservationNo, OldIPSetDigest string
	ChannelNo, PoolNo, ExecutionStatus                    string
	IPQuantity                                            int
}

func (w *workerImpl) reconcileIPReservations(ctx context.Context, limit int) (int, int, error) {
	var rows []ipReservationWork
	if err := w.database.GetScan(ctx, &rows, `SELECT intent.operation_no,intent.service_no,
intent.resource_reservation_no AS reservation_no,intent.old_ip_set_digest,intent.ip_quantity,
execution.channel_no,execution.pool_no,execution.status_code AS execution_status
FROM plugin_idc_core_server_ip_change_intent intent
JOIN plugin_idc_core_server_operation_execution execution
 ON execution.tenant_id=intent.tenant_id AND execution.operation_no=intent.operation_no
WHERE intent.tenant_id=0 AND intent.reservation_status_code IN ('reserved','unknown')
 AND execution.status_code IN ('succeeded','failed','manual_review')
ORDER BY intent.updated_at,intent.id LIMIT ?`, limit); err != nil {
		return 0, 0, gerror.Wrap(err, "list IP reservation reconciliation")
	}
	reconciled, manual := 0, 0
	var batchErr error
	for index := range rows {
		row := &rows[index]
		switch row.ExecutionStatus {
		case "manual_review":
			if err := w.markIPReservationUnknown(ctx, row.OperationNo); err != nil {
				batchErr = err
			} else {
				manual++
			}
		case "failed":
			reservation, err := w.ipCapacity.ReleaseIP(ctx,
				serveroperationresourcecap.IPReservationReleaseRequest{
					ReservationNo: row.ReservationNo, ReasonCode: "provider_definitive_failure"})
			if err != nil || reservation == nil ||
				(reservation.Status != serveroperationresourcecap.IPReservationReleased &&
					reservation.Status != serveroperationresourcecap.IPReservationExpired) {
				batchErr = errors.Join(batchErr, err, serveroperationresourcecap.ErrIPCapacityUnavailable)
				continue
			}
			if err = w.finishIPReservation(ctx, row, "released", "", "server_ip_change.inventory_released"); err != nil {
				batchErr = errors.Join(batchErr, err)
			} else {
				reconciled++
			}
		case "succeeded":
			directory, err := w.directory.GetAccessories(ctx, serveroperationresourcecap.CapabilityRoute{
				ServiceNo: row.ServiceNo, ChannelNo: row.ChannelNo, PoolNo: row.PoolNo})
			if err != nil || directory == nil || directory.ServiceNo != row.ServiceNo ||
				directory.ExpiresAt.Before(time.Now().UTC()) {
				batchErr = errors.Join(batchErr, err, serveroperationresourcecap.ErrSourceUnavailable)
				continue
			}
			addresses, err := currentIPAddresses(directory.Items)
			if err != nil || len(addresses) != row.IPQuantity {
				batchErr = errors.Join(batchErr, err, serveroperationresourcecap.ErrSourceUnavailable)
				continue
			}
			newDigest, err := digestValue(struct {
				Domain, ServiceNo string
				Addresses         []string
			}{"server-ip-change-new-address-set/v1", row.ServiceNo, addresses})
			if err != nil || newDigest == row.OldIPSetDigest {
				batchErr = errors.Join(batchErr, err, serveroperationresourcecap.ErrSourceUnavailable)
				continue
			}
			providerTaskNo, err := w.database.GetValue(ctx, `SELECT provider_task_no
FROM plugin_idc_core_server_operation_execution
WHERE tenant_id=0 AND operation_no=? AND status_code='succeeded'`, row.OperationNo)
			if err != nil || providerTaskNo.String() == "" {
				batchErr = errors.Join(batchErr, err, serveroperationresourcecap.ErrTaskUnavailable)
				continue
			}
			reservation, err := w.ipCapacity.ConsumeIP(ctx,
				serveroperationresourcecap.IPReservationConsumeRequest{
					ReservationNo: row.ReservationNo, ProviderTaskNo: providerTaskNo.String()})
			if err != nil || reservation == nil || reservation.Status != serveroperationresourcecap.IPReservationConsumed {
				batchErr = errors.Join(batchErr, err, serveroperationresourcecap.ErrIPCapacityUnavailable)
				continue
			}
			if err = w.finishIPReservation(ctx, row, "consumed", newDigest,
				"server_ip_change.inventory_consumed"); err != nil {
				batchErr = errors.Join(batchErr, err)
			} else {
				reconciled++
			}
		}
	}
	return reconciled, manual, batchErr
}

func (w *workerImpl) markIPReservationUnknown(ctx context.Context, operationNo string) error {
	_, err := w.database.Exec(ctx, `UPDATE plugin_idc_core_server_ip_change_intent
SET reservation_status_code='unknown',updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND operation_no=? AND reservation_status_code='reserved'`, operationNo)
	return err
}

func (w *workerImpl) finishIPReservation(ctx context.Context, row *ipReservationWork,
	status, newDigest, reason string) error {
	return w.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var locked *struct {
			Status, OldDigest string
		}
		if err := w.database.GetScan(ctx, &locked, `SELECT reservation_status_code AS status,
old_ip_set_digest AS old_digest FROM plugin_idc_core_server_ip_change_intent
WHERE tenant_id=0 AND operation_no=? FOR UPDATE`, row.OperationNo); err != nil {
			return err
		}
		if locked == nil {
			return ErrOperationConflict
		}
		if locked.Status == status {
			return nil
		}
		if locked.Status != "reserved" && locked.Status != "unknown" {
			return ErrOperationConflict
		}
		now, err := databaseNow(ctx, w.database)
		if err != nil {
			return err
		}
		var newValue any
		if newDigest != "" {
			newValue = newDigest
		}
		if err = execOne(ctx, w.database, `UPDATE plugin_idc_core_server_ip_change_intent
SET reservation_status_code=?,new_ip_set_digest=?,updated_at=?
WHERE tenant_id=0 AND operation_no=? AND reservation_status_code IN ('reserved','unknown')`,
			status, newValue, now, row.OperationNo); err != nil {
			return err
		}
		evidence, err := digestValue(struct {
			OperationNo, ReservationNo, OldDigest, NewDigest, Status string
		}{row.OperationNo, row.ReservationNo, locked.OldDigest, newDigest, status})
		if err != nil {
			return err
		}
		eventNo, _ := deterministicReference("ipe_", struct{ OperationNo, Status string }{
			row.OperationNo, status,
		})
		_, err = w.database.Exec(ctx, `INSERT INTO plugin_idc_core_server_ip_change_event
(tenant_id,event_no,operation_no,service_no,reservation_no,event_type_code,old_ip_set_digest,
new_ip_set_digest,evidence_digest,created_at)
VALUES (0,?,?,?,?,?,?,?,?,?)`, eventNo, row.OperationNo, row.ServiceNo, row.ReservationNo,
			status, locked.OldDigest, newValue, evidence, now)
		return err
	})
}
