// This file executes the durable, generic entitlement termination requested by a successful refund.

package billing

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/asynctask"
)

const refundEntitlementWorkerID = "refund-entitlement-termination"

// RefundEntitlementWorker terminates only the generic long-lived entitlement
// selected by a completed refund. It never reads or mutates server instances.
type RefundEntitlementWorker interface {
	Run(context.Context, int) (int, error)
}

type refundEntitlementWorker struct {
	database gdb.DB
	tasks    asynctask.Service
}

type refundEntitlementTaskPayload struct {
	RefundNo      string `json:"refundNo"`
	ImpactNo      string `json:"impactNo"`
	EntitlementNo string `json:"entitlementNo"`
}

type refundEntitlementResult struct {
	RefundNo           string `json:"refundNo"`
	ImpactNo           string `json:"impactNo"`
	EntitlementNo      string `json:"entitlementNo"`
	PreviousStatus     string `json:"previousStatus"`
	ResultStatus       string `json:"resultStatus"`
	PreviousVersion    int64  `json:"previousVersion"`
	ResultVersion      int64  `json:"resultVersion"`
	IdempotentReplay   bool   `json:"idempotentReplay"`
	RefundLockReleased bool   `json:"refundLockReleased"`
}

type permanentEntitlementRefundError struct{ message string }

func (e *permanentEntitlementRefundError) Error() string { return e.message }

// NewRefundEntitlementWorker constructs the bounded durable-task consumer.
func NewRefundEntitlementWorker(database gdb.DB, tasks asynctask.Service) (RefundEntitlementWorker, error) {
	if database == nil || tasks == nil {
		return nil, gerror.New("refund entitlement worker requires database and tasks")
	}
	return &refundEntitlementWorker{database: database, tasks: tasks}, nil
}

func (w *refundEntitlementWorker) Run(ctx context.Context, limit int) (int, error) {
	claimed, err := w.tasks.Claim(ctx, asynctask.ClaimInput{
		WorkerID:  refundEntitlementWorkerID,
		Limit:     limit,
		TaskTypes: []string{refundEntitlementTaskType},
	})
	if err != nil {
		return 0, err
	}
	completed := 0
	for _, task := range claimed {
		_, processErr := w.process(ctx, task)
		if processErr != nil {
			var permanent *permanentEntitlementRefundError
			retryable := !errors.As(processErr, &permanent)
			failureEvidence, evidenceErr := json.Marshal(map[string]any{
				"status": "failed", "taskNo": task.TaskNo,
				"entitlementNo": task.SubjectNo, "retryable": retryable,
			})
			if evidenceErr != nil {
				return completed, gerror.Wrap(evidenceErr, "encode refund entitlement failure evidence")
			}
			if _, failErr := w.tasks.Fail(ctx, asynctask.FailureInput{
				Lease: task.Lease, Retryable: retryable,
				ResponseSnapshot: failureEvidence,
				ErrorCode:        "entitlement_termination_failed",
				ErrorSummary:     "Refund entitlement termination could not be confirmed",
			}); failErr != nil {
				return completed, gerror.Wrap(failErr, "record refund entitlement termination failure")
			}
			continue
		}
		completed++
	}
	return completed, nil
}

func (w *refundEntitlementWorker) process(
	ctx context.Context,
	task asynctask.ClaimedTask,
) (*refundEntitlementResult, error) {
	var payload refundEntitlementTaskPayload
	if task.TaskType != refundEntitlementTaskType || task.SubjectType != "service_entitlement" ||
		json.Unmarshal(task.Payload, &payload) != nil {
		return nil, permanentRefundEntitlementError("refund entitlement task payload is invalid")
	}
	payload.RefundNo = strings.TrimSpace(payload.RefundNo)
	payload.ImpactNo = strings.TrimSpace(payload.ImpactNo)
	payload.EntitlementNo = strings.TrimSpace(payload.EntitlementNo)
	if payload.RefundNo == "" || payload.ImpactNo == "" || payload.EntitlementNo == "" ||
		payload.EntitlementNo != task.SubjectNo {
		return nil, permanentRefundEntitlementError("refund entitlement task identity is invalid")
	}

	var result *refundEntitlementResult
	err := w.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var row *struct {
			EntitlementNo   string `orm:"entitlement_no"`
			StatusCode      string `orm:"status_code"`
			Version         int64  `orm:"version"`
			ExpectedVersion int64  `orm:"expected_version"`
		}
		if err := w.database.GetScan(txCtx, &row, `
SELECT entitlement.entitlement_no,entitlement.status_code,entitlement.version,
       impact.entitlement_version_snapshot AS expected_version
FROM plugin_idc_core_refund_entitlement_impact impact
JOIN plugin_idc_core_refund_request request
  ON request.tenant_id=impact.tenant_id AND request.refund_no=impact.refund_no
JOIN plugin_idc_core_refund_entitlement_lock entitlement_lock
  ON entitlement_lock.tenant_id=impact.tenant_id
 AND entitlement_lock.refund_no=impact.refund_no
 AND entitlement_lock.refund_item_no=impact.refund_item_no
 AND entitlement_lock.impact_no=impact.impact_no
 AND entitlement_lock.entitlement_no=impact.entitlement_no
JOIN plugin_idc_core_service_entitlement entitlement
  ON entitlement.tenant_id=impact.tenant_id
 AND entitlement.entitlement_no=impact.entitlement_no
WHERE impact.tenant_id=0 AND impact.refund_no=? AND impact.impact_no=?
  AND impact.entitlement_no=? AND impact.requires_termination
  AND request.status_code='succeeded' AND request.customer_no=?
FOR UPDATE OF entitlement,entitlement_lock`, payload.RefundNo, payload.ImpactNo,
			payload.EntitlementNo, task.CustomerNo); err != nil {
			return err
		}
		if row == nil {
			return permanentRefundEntitlementError("refund entitlement evidence is unavailable")
		}
		result = &refundEntitlementResult{
			RefundNo: payload.RefundNo, ImpactNo: payload.ImpactNo,
			EntitlementNo: payload.EntitlementNo, PreviousStatus: row.StatusCode,
			PreviousVersion: row.Version, ResultStatus: "terminated",
		}
		switch row.StatusCode {
		case "active", "suspended", "expired":
			if row.Version != row.ExpectedVersion {
				return permanentRefundEntitlementError("refund entitlement version changed")
			}
			update, err := w.database.Exec(txCtx, `
UPDATE plugin_idc_core_service_entitlement
SET status_code='terminated',version=version+1,updated_at=CURRENT_TIMESTAMP
WHERE tenant_id=0 AND entitlement_no=? AND version=?
  AND status_code IN ('active','suspended','expired')`, row.EntitlementNo, row.ExpectedVersion)
			if err != nil {
				return err
			}
			affected, err := update.RowsAffected()
			if err != nil || affected != 1 {
				if err != nil {
					return err
				}
				return permanentRefundEntitlementError("refund entitlement update lost its version fence")
			}
			result.ResultVersion = row.ExpectedVersion + 1
		case "terminated":
			if row.Version != row.ExpectedVersion+1 {
				return permanentRefundEntitlementError("terminated refund entitlement version is inconsistent")
			}
			result.IdempotentReplay = true
			result.ResultVersion = row.Version
		default:
			return permanentRefundEntitlementError("refund entitlement status is invalid")
		}
		lockUpdate, err := w.database.Exec(txCtx, `
UPDATE plugin_idc_core_refund_entitlement_lock
SET released_at=COALESCE(released_at,CURRENT_TIMESTAMP),
    release_reason_code=COALESCE(release_reason_code,'succeeded')
WHERE tenant_id=0 AND refund_no=? AND impact_no=? AND entitlement_no=?`,
			payload.RefundNo, payload.ImpactNo, payload.EntitlementNo)
		if err != nil {
			return err
		}
		affected, err := lockUpdate.RowsAffected()
		if err != nil || affected != 1 {
			if err != nil {
				return err
			}
			return permanentRefundEntitlementError("refund entitlement lock is unavailable")
		}
		result.RefundLockReleased = true
		evidence, err := json.Marshal(result)
		if err != nil {
			return err
		}
		if _, err = w.tasks.CompleteInDatabase(txCtx, w.database, asynctask.CompleteInput{
			Lease: task.Lease, ExternalReferenceNo: result.EntitlementNo,
			ResultSnapshot: evidence,
		}); err != nil {
			return gerror.Wrap(err, "complete refund entitlement termination task")
		}
		return nil
	})
	return result, err
}

func permanentRefundEntitlementError(message string) error {
	return &permanentEntitlementRefundError{message: message}
}
