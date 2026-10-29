// This file reports bounded durable task results. A created accessory is never
// attached to billing by guessing its name or its position in a refreshed list.
package nativecommercial

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
	"time"
)

type outcome struct {
	OperationNo, EntitlementNo, TaskNo, TaskStatus, ResultStatus, FailureCode string
	ActionCode                                                                serveroperationresourcecap.Action
	Version                                                                   int64
	UpdatedAt                                                                 time.Time
	ResultUnknown                                                             bool
}

// RunOutcomes reads one joined batch without a provider or per-row task query.
// Core accepts identical receipts idempotently; each local receipt commits with it.
func (s *Service) RunOutcomes(ctx context.Context) error {
	var rows []outcome
	err := s.database.GetScan(ctx, &rows, `SELECT binding.operation_no,binding.entitlement_no,binding.task_no,binding.action_code,binding.result_status,
task.status_code AS task_status,task.version,task.updated_at,task.result_unknown,task.failure_code
FROM plugin_idc_resource_native_commercial_task binding
JOIN plugin_idc_resource_task task ON task.tenant_id=binding.tenant_id AND task.task_no=binding.task_no
WHERE binding.tenant_id=0 AND binding.completed_at IS NULL AND binding.next_report_at<=CURRENT_TIMESTAMP
AND task.status_code IN ('succeeded','failed','canceled','result_unknown','manual_takeover')
ORDER BY binding.next_report_at,binding.operation_no LIMIT 16`)
	if err != nil {
		return err
	}
	var failures []error
	for _, row := range rows {
		if err = s.reportOutcome(ctx, row); err != nil {
			failures = append(failures, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	return errors.Join(append(failures, s.reportDeletions(ctx))...)
}

func (s *Service) reportOutcome(ctx context.Context, row outcome) error {
	status := servercommercialcap.CompletionUnknown
	switch row.TaskStatus {
	case "succeeded":
		if !row.ResultUnknown {
			status = servercommercialcap.CompletionSucceeded
		}
	case "failed", "canceled":
		if !row.ResultUnknown {
			status = servercommercialcap.CompletionFailed
		}
	}
	if status == servercommercialcap.CompletionSucceeded && (row.ActionCode == serveroperationresourcecap.ActionSnapshotCreate || row.ActionCode == serveroperationresourcecap.ActionBackupCreate) {
		// The current upstream task contract has no verified created-object identity.
		// Keep its commercial occupancy for manual reconciliation instead of binding
		// an unrelated object or charging a duplicate create after successful execution.
		status = servercommercialcap.CompletionUnknown
	}
	payload, err := json.Marshal(struct {
		OperationNo, TaskNo, Status, FailureCode string
		Version, UpdatedAt                       int64
		ResultUnknown                            bool
	}{row.OperationNo, row.TaskNo, row.TaskStatus, row.FailureCode, row.Version, row.UpdatedAt.UnixMicro(), row.ResultUnknown})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(payload)
	receipt := servercommercialcap.Completion{OperationNo: row.OperationNo, EntitlementNo: row.EntitlementNo, TaskNo: row.TaskNo, EvidenceDigest: hex.EncodeToString(digest[:]), Status: status, ObservedAt: row.UpdatedAt}
	return s.database.Transaction(ctx, func(txctx context.Context, _ gdb.TX) error {
		if row.ResultStatus != string(status) {
			if completeErr := s.commercial.Complete(txctx, receipt); completeErr != nil {
				return completeErr
			}
		}
		var completed any
		if status != servercommercialcap.CompletionUnknown {
			completed = time.Now().UTC()
		}
		_, updateErr := s.database.Exec(txctx, `UPDATE plugin_idc_resource_native_commercial_task SET result_status=?,completed_at=?,next_report_at=CURRENT_TIMESTAMP+INTERVAL '30 seconds',updated_at=CURRENT_TIMESTAMP WHERE tenant_id=0 AND operation_no=? AND task_no=?`, status, completed, row.OperationNo, row.TaskNo)
		return updateErr
	})
}
