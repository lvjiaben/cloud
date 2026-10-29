// Native accessory occupancy combines the resource owner's fresh object set
// with durable unbound reservations, and removes only proven deletion receipts.

package serveroperation

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// nativeAccessoryOccupancy returns per-server, per-kind counts. Its caller must
// hold the parent entitlement lock through checkout. An unfinished ambiguous
// create fails closed rather than guessing which newly observed object it owns.
func nativeAccessoryOccupancy(ctx context.Context, database gdb.DB, target servercommercialcap.TargetSnapshot,
	kind serveroperationresourcecap.AccessoryKind) (int, int, error) {
	if kind == serveroperationresourcecap.AccessoryKindTraffic {
		return 0, 0, nil
	}
	if kind != serveroperationresourcecap.AccessoryKindSnapshot && kind != serveroperationresourcecap.AccessoryKindBackup {
		return 0, 0, ErrInvalidInput
	}
	objects := make(map[string]struct{}, len(target.Items))
	for _, item := range target.Items {
		if item.Kind == kind {
			objects[item.ObjectNo] = struct{}{}
		}
	}
	var rows []struct {
		ObjectNo        string
		StatusCode      nativeAccessoryState
		OperationStatus nativeCreateState
	}
	query := `SELECT COALESCE(accessory.object_no,'') AS object_no,
accessory.status_code,operation.status_code AS operation_status
FROM plugin_idc_core_server_accessory_entitlement accessory
JOIN plugin_idc_core_business_operation operation
 ON operation.tenant_id=accessory.tenant_id AND operation.operation_no=accessory.create_operation_no
WHERE accessory.tenant_id=0 AND accessory.subject_type_code='entitlement' AND accessory.subject_no=?
 AND accessory.customer_no=? AND accessory.accessory_kind=?
 AND accessory.status_code IN ('pending_binding','active','payment_pending','manual_review')`
	arguments := []any{target.EntitlementNo, target.CustomerNo, kind}
	if len(objects) > 0 {
		query += ` UNION ALL SELECT object_no,status_code,'' AS operation_status
FROM plugin_idc_core_server_accessory_entitlement
WHERE tenant_id=0 AND subject_type_code='entitlement' AND subject_no=? AND customer_no=?
 AND accessory_kind=? AND status_code IN ('deleted','deletion_pending') AND object_no IN (` + strings.TrimSuffix(strings.Repeat("?,", len(objects)), ",") + `)`
		arguments = append(arguments, target.EntitlementNo, target.CustomerNo, kind)
		for objectNo := range objects {
			arguments = append(arguments, objectNo)
		}
	}
	if err := database.GetScan(ctx, &rows, query, arguments...); err != nil {
		return 0, 0, err
	}
	reserved := 0
	for _, row := range rows {
		if row.StatusCode == nativeAccessoryDeleted || row.StatusCode == nativeAccessoryDeleting {
			delete(objects, row.ObjectNo)
			continue
		}
		if row.ObjectNo != "" {
			// A missing bound object needs a refreshed directory or a proven
			// deletion, not an invented free slot or an extra paid object.
			if _, present := objects[row.ObjectNo]; !present {
				return 0, 0, ErrOperationConflict
			}
			continue
		}
		if row.OperationStatus == nativeCreateFailed || row.OperationStatus == nativeCreateCancelled {
			continue
		}
		if row.StatusCode == nativeAccessoryManual || row.OperationStatus == nativeCreateSucceeded || row.OperationStatus == nativeCreateExecuting || row.OperationStatus == nativeCreateManual {
			return 0, 0, ErrOperationConflict
		}
		if row.StatusCode == nativeAccessoryPending {
			reserved++
		}
	}
	return len(objects), reserved, nil
}
