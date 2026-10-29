// Write receipts release a core fence only from an actual owner task result.
// Uncertain operations keep the fence and are never submitted again here.
package customeroperations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	"lina-plugin-idc-resource/backend/internal/dao"
)

type WriteReconciler struct {
	database gdb.DB
	owner    servercommercialcap.QueuedWriteAccess
}

func NewWriteReconciler(database gdb.DB, owner servercommercialcap.Service) (*WriteReconciler, error) {
	access, ok := owner.(servercommercialcap.QueuedWriteAccess)
	if database == nil || !ok {
		return nil, ErrUnavailable
	}
	return &WriteReconciler{database: database, owner: access}, nil
}

type nativeWriteReceipt struct {
	TaskNo, StatusCode, OwnerMetadata string
	Version                           int64
	UpdatedAt                         time.Time
	ResultUnknown                     bool
}

func (s *WriteReconciler) Run(ctx context.Context) error {
	if ctx == nil || ctx.Err() != nil || gdb.TXFromCtx(ctx, s.database.GetGroup()) != nil {
		return ErrUnavailable
	}
	pending, err := s.owner.ListQueuedWrites(ctx, 16)
	if err != nil || len(pending) == 0 {
		return err
	}
	if len(pending) > 16 {
		return ErrUnavailable
	}
	numbers := make([]string, len(pending))
	for i, item := range pending {
		numbers[i] = item.TaskNo
	}
	var rows []nativeWriteReceipt
	err = s.database.Model(dao.Task.Table()).Safe().Ctx(ctx).Fields("task_no,status_code,owner_metadata::text AS owner_metadata,version,updated_at,result_unknown").Where("tenant_id", 0).WhereIn("task_no", numbers).Scan(&rows)
	if err != nil {
		return err
	}
	receipts := make(map[string]nativeWriteReceipt, len(rows))
	for _, row := range rows {
		receipts[row.TaskNo] = row
	}
	var failures []error
	for _, item := range pending {
		if row, ok := receipts[item.TaskNo]; ok {
			receipt, valid := confirmedWriteReceipt(item, row)
			if valid {
				if completeErr := s.owner.CompleteQueuedWrite(ctx, receipt); completeErr != nil {
					failures = append(failures, completeErr)
				}
			}
		}
		if ctx.Err() != nil {
			failures = append(failures, ctx.Err())
			break
		}
	}
	return errors.Join(failures...)
}
func confirmedWriteReceipt(item servercommercialcap.QueuedWrite, row nativeWriteReceipt) (servercommercialcap.QueuedWriteCompletion, bool) {
	var metadata struct {
		nativeTaskMetadata
		NativeWriteClaim int `json:"nativeWriteClaim"`
	}
	if json.Unmarshal([]byte(row.OwnerMetadata), &metadata) != nil || metadata.NativeWriteClaim < 1 || metadata.OperationNo != item.OperationNo || metadata.Action != item.Action || metadata.ExpectedServiceVersion != item.ExpectedEntitlementVersion || hex.EncodeToString(metadata.RequestDigest[:]) != item.RequestDigest {
		return servercommercialcap.QueuedWriteCompletion{}, false
	}
	status := servercommercialcap.CompletionUnknown
	switch row.StatusCode {
	case "succeeded":
		if !row.ResultUnknown {
			status = servercommercialcap.CompletionSucceeded
		}
	case "failed", "canceled":
		if !row.ResultUnknown {
			status = servercommercialcap.CompletionFailed
		}
	case "result_unknown", "manual_takeover":
	default:
		return servercommercialcap.QueuedWriteCompletion{}, false
	}
	payload, err := json.Marshal(struct {
		TaskNo, OperationNo, RequestDigest, Status string
		Version, UpdatedAt                         int64
		Unknown                                    bool
	}{row.TaskNo, item.OperationNo, item.RequestDigest, row.StatusCode, row.Version, row.UpdatedAt.UnixMicro(), row.ResultUnknown})
	if err != nil {
		return servercommercialcap.QueuedWriteCompletion{}, false
	}
	digest := sha256.Sum256(payload)
	return servercommercialcap.QueuedWriteCompletion{TaskNo: row.TaskNo, EntitlementNo: item.EntitlementNo, CustomerNo: item.CustomerNo, EvidenceDigest: hex.EncodeToString(digest[:]), Status: status}, true
}
