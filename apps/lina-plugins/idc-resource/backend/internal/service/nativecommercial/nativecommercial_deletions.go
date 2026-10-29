// Free deletion receipts are retried locally until the commercial owner has
// stopped renewal. No retry ever resends a supplier deletion.
package nativecommercial

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

type deletionOutcome struct {
	TaskNo, OperationNo, EntitlementNo, CustomerNo, ObjectNo, StatusCode string
	AccessoryKind                                                        serveroperationresourcecap.AccessoryKind
	Version                                                              int64
	ObservedAt                                                           time.Time
}

func (s *Service) reportDeletions(ctx context.Context) error {
	var rows []deletionOutcome
	err := s.database.GetScan(ctx, &rows, `SELECT receipt.task_no,receipt.operation_no,receipt.entitlement_no,receipt.customer_no,receipt.object_no,receipt.accessory_kind,task.status_code,task.version,task.updated_at AS observed_at
FROM plugin_idc_resource_native_accessory_receipt receipt JOIN plugin_idc_resource_task task ON task.tenant_id=receipt.tenant_id AND task.task_no=receipt.task_no
WHERE receipt.tenant_id=0 AND receipt.completed_at IS NULL AND receipt.next_report_at<=CURRENT_TIMESTAMP
AND task.status_code IN ('succeeded','failed','canceled') AND NOT task.result_unknown
ORDER BY receipt.next_report_at,receipt.task_no LIMIT 16`)
	if err != nil {
		return err
	}
	var failures []error
	for _, row := range rows {
		var reportErr error
		if row.StatusCode == "succeeded" {
			payload, encodeErr := json.Marshal(row)
			if encodeErr != nil {
				reportErr = encodeErr
			} else {
				digest := sha256.Sum256(payload)
				reportErr = s.commercial.CompleteDeletion(ctx, servercommercialcap.Deletion{ObjectRequest: servercommercialcap.ObjectRequest{EntitlementNo: row.EntitlementNo, CustomerNo: row.CustomerNo, ObjectNo: row.ObjectNo, Kind: row.AccessoryKind}, OperationNo: row.OperationNo, TaskNo: row.TaskNo, EvidenceDigest: hex.EncodeToString(digest[:]), ObservedAt: row.ObservedAt})
			}
		}
		// CompleteDeletion may have committed a payment-close request but still
		// require retry. Do not enclose it in a transaction that rolls that back.
		var completed any
		if reportErr == nil {
			completed = time.Now().UTC()
		}
		_, updateErr := s.database.Exec(ctx, `UPDATE plugin_idc_resource_native_accessory_receipt SET next_report_at=CURRENT_TIMESTAMP+INTERVAL '30 seconds',completed_at=?,updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND task_no=? AND completed_at IS NULL`, completed, row.TaskNo)
		if joined := errors.Join(reportErr, updateErr); joined != nil {
			failures = append(failures, joined)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(failures...)
}
