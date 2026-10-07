// This file converges authoritative Provider results into service, operation, and task facts.

package lifecycle

import (
	"context"
	"encoding/json"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
)

func (p *processor) convergeProviderResult(
	ctx context.Context,
	observedState *taskState,
	provider *lifecycleresourcecap.Task,
) error {
	if observedState == nil || provider == nil ||
		lifecycleresourcecap.ValidateTask(*provider) != nil {
		return lifecycleresourcecap.ErrTaskConflict
	}
	if provider.Status == lifecycleresourcecap.TaskStatusPending ||
		provider.Status == lifecycleresourcecap.TaskStatusRunning {
		return nil
	}
	return p.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		state, err := loadTaskState(ctx, p.database, observedState.TaskNo, true)
		if err != nil || state == nil || state.StatusCode != taskStatusDispatched ||
			state.BusinessOperationNo == nil {
			return err
		}
		switch provider.Status {
		case lifecycleresourcecap.TaskStatusSucceeded:
			if err = convergeServiceRuntime(ctx, p.database, state); err != nil {
				return err
			}
			if err = transitionBusinessOperation(
				ctx, p.database, *state.BusinessOperationNo, state.ServiceNo,
				operationStatusExecuting, operationStatusSucceeded,
				"lifecycle.provider_succeeded", "",
			); err != nil {
				return err
			}
			return transitionLifecycleTask(
				ctx, p.database, state, taskStatusSucceeded, "lifecycle.provider_succeeded",
				state.BusinessOperationNo, state.PreviousRuntimeStatusCode, true,
			)
		case lifecycleresourcecap.TaskStatusFailed:
			if err = restoreFailedTermination(ctx, p.database, state); err != nil {
				return err
			}
			if err = transitionBusinessOperation(
				ctx, p.database, *state.BusinessOperationNo, state.ServiceNo,
				operationStatusExecuting, operationStatusFailed,
				"lifecycle.provider_failed", provider.FailureCode,
			); err != nil {
				return err
			}
			return transitionLifecycleTaskWithError(
				ctx, p.database, state, taskStatusFailed, "lifecycle.provider_failed",
				provider.FailureCode,
			)
		case lifecycleresourcecap.TaskStatusResultUnknown,
			lifecycleresourcecap.TaskStatusManualReview:
			providerStatus := providerTaskStatusUnknown
			if provider.Status == lifecycleresourcecap.TaskStatusManualReview {
				providerStatus = providerTaskStatusManual
			}
			if err = markProviderReviewState(ctx, p.database, state, providerStatus); err != nil {
				return err
			}
			if err = transitionBusinessOperation(
				ctx, p.database, *state.BusinessOperationNo, state.ServiceNo,
				operationStatusExecuting, operationStatusManualReview,
				"lifecycle.provider_unknown",
				provider.FailureCode,
			); err != nil {
				return err
			}
			return transitionLifecycleTaskWithErrorAndResiduals(
				ctx, p.database, state, taskStatusManualReview, "lifecycle.provider_unknown",
				provider.FailureCode, provider.ResidualResources,
			)
		default:
			return lifecycleresourcecap.ErrTaskConflict
		}
	})
}

// markProviderReviewState exposes the resource owner's bounded uncertain state
// without changing the entitlement or runtime projection. In particular, a
// termination with an unknown result must remain terminating until an
// authoritative observation or a separately authorized takeover resolves it.
func markProviderReviewState(
	ctx context.Context,
	database gdb.DB,
	state *taskState,
	status providerTaskStatus,
) error {
	if state == nil || (status != providerTaskStatusUnknown && status != providerTaskStatusManual) {
		return gerror.New("invalid service lifecycle review state")
	}
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_service_instance
SET provider_task_status_code = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND service_no = ? AND version = ?`,
		status, state.ServiceNo, state.ServiceVersion)
	if err != nil {
		return gerror.Wrap(err, "mark service lifecycle Provider review state")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count service lifecycle Provider review update")
	}
	if affected != 1 {
		return gerror.New("service lifecycle Provider review fence changed")
	}
	state.ServiceVersion++
	state.ProviderTaskStatusCode = status
	return nil
}

func convergeServiceRuntime(
	ctx context.Context,
	database gdb.DB,
	state *taskState,
) error {
	var runtime runtimeStatus
	entitlementStatus := state.EntitlementStatusCode
	switch state.ActionCode {
	case actionSuspend:
		runtime = runtimeStatusSuspended
	case actionResume:
		runtime = runtimeStatusRunning
	case actionTerminate:
		runtime = runtimeStatusTerminated
		entitlementStatus = entitlementStatusTerminated
	default:
		return gerror.New("unsupported Provider lifecycle convergence")
	}
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_service_instance
SET runtime_status_code = ?, entitlement_status_code = ?,
    provider_task_status_code = ?, last_business_operation_no = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND service_no = ? AND version = ?`,
		runtime, entitlementStatus, providerTaskStatusSucceeded, *state.BusinessOperationNo,
		state.ServiceNo, state.ServiceVersion)
	if err != nil {
		return gerror.Wrap(err, "converge service lifecycle Provider result")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count service lifecycle convergence")
	}
	if affected != 1 {
		return gerror.New("service lifecycle convergence fence changed")
	}
	state.ServiceVersion++
	state.RuntimeStatusCode = runtime
	state.EntitlementStatusCode = entitlementStatus
	return nil
}

func restoreFailedTermination(
	ctx context.Context,
	database gdb.DB,
	state *taskState,
) error {
	if state.ActionCode != actionTerminate ||
		state.PreviousRuntimeStatusCode == nil ||
		state.RuntimeStatusCode != runtimeStatusTerminating {
		result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_service_instance
	SET provider_task_status_code = ?, version = version + 1,
	    updated_at = CURRENT_TIMESTAMP
	WHERE tenant_id = 0 AND service_no = ? AND version = ?`,
			providerTaskStatusFailed, state.ServiceNo, state.ServiceVersion)
		if err != nil {
			return gerror.Wrap(err, "mark failed service lifecycle Provider task")
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return gerror.Wrap(err, "count failed service lifecycle Provider update")
		}
		if affected != 1 {
			return gerror.New("failed service lifecycle Provider fence changed")
		}
		state.ServiceVersion++
		return nil
	}
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_service_instance
SET runtime_status_code = ?, provider_task_status_code = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND service_no = ? AND version = ?
  AND runtime_status_code = ?`, *state.PreviousRuntimeStatusCode,
		providerTaskStatusFailed, state.ServiceNo, state.ServiceVersion,
		runtimeStatusTerminating)
	if err != nil {
		return gerror.Wrap(err, "restore failed service termination")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count failed service termination restoration")
	}
	if affected != 1 {
		return gerror.New("failed service termination restore fence changed")
	}
	state.ServiceVersion++
	state.RuntimeStatusCode = *state.PreviousRuntimeStatusCode
	return nil
}

func transitionLifecycleTaskWithError(
	ctx context.Context,
	database gdb.DB,
	state *taskState,
	toStatus taskStatus,
	reason string,
	failureCode string,
) error {
	return transitionLifecycleTaskWithErrorAndResiduals(
		ctx, database, state, toStatus, reason, failureCode, nil)
}

func transitionLifecycleTaskWithErrorAndResiduals(
	ctx context.Context,
	database gdb.DB,
	state *taskState,
	toStatus taskStatus,
	reason string,
	failureCode string,
	residuals []lifecycleresourcecap.ResidualResource,
) error {
	encodedResiduals, err := json.Marshal(residuals)
	if err != nil {
		return gerror.Wrap(err, "encode service lifecycle residual resources")
	}
	if residuals == nil {
		encodedResiduals = []byte("[]")
	}
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_service_lifecycle_task
SET status_code = ?, last_error_code = ?, completed_at = CURRENT_TIMESTAMP,
    residual_resources = ?::JSONB, version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND task_no = ? AND status_code = ? AND version = ?`,
		toStatus, failureCode, string(encodedResiduals), state.TaskNo, state.StatusCode, state.TaskVersion)
	if err != nil {
		return gerror.Wrap(err, "close failed service lifecycle task")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count failed service lifecycle task closure")
	}
	if affected != 1 {
		return gerror.New("failed service lifecycle task fence changed")
	}
	sequence := state.TaskVersion + 1
	evidence, content, eventNo, err := lifecycleTransitionDigests(
		state.TaskNo, sequence, state.StatusCode, toStatus, reason,
	)
	if err != nil {
		return err
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_service_lifecycle_task_event
    (event_no, task_no, service_no, event_sequence, from_status_code,
     to_status_code, reason_code, actor_type_code, actor_reference,
     evidence_digest, content_digest)
VALUES (?, ?, ?, ?, ?, ?, ?, 'system', 'lifecycle-processor', ?, ?)`,
		eventNo, state.TaskNo, state.ServiceNo, sequence, state.StatusCode,
		toStatus, reason, evidence, content); err != nil {
		return gerror.Wrap(err, "append failed service lifecycle task event")
	}
	return nil
}

func transitionBusinessOperation(
	ctx context.Context,
	database gdb.DB,
	operationNo string,
	serviceNo string,
	fromStatus operationStatus,
	toStatus operationStatus,
	reason string,
	failureCode string,
) error {
	var row *struct {
		Version int64 `orm:"version"`
	}
	if err := database.GetScan(ctx, &row, `
SELECT version
FROM plugin_idc_core_business_operation
WHERE tenant_id = 0 AND operation_no = ? AND service_no = ?
  AND status_code = ?
FOR UPDATE`, operationNo, serviceNo, fromStatus); err != nil {
		return gerror.Wrap(err, "lock lifecycle business operation")
	}
	if row == nil || row.Version <= 0 {
		return gerror.New("lifecycle business operation state changed")
	}
	version := row.Version
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_business_operation
SET status_code = ?, last_error_code = ?, completed_at = CURRENT_TIMESTAMP,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND operation_no = ? AND service_no = ?
  AND status_code = ? AND version = ?`,
		toStatus, failureCode, operationNo, serviceNo, fromStatus, version)
	if err != nil {
		return gerror.Wrap(err, "transition lifecycle business operation")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count lifecycle business operation transition")
	}
	if affected != 1 {
		return gerror.New("lifecycle business operation fence changed")
	}
	sequence := version + 1
	evidence, content, eventNo, err := operationTransitionDigests(
		operationNo, sequence, fromStatus, toStatus, reason,
	)
	if err != nil {
		return err
	}
	if _, err = database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation_event
    (event_no, operation_no, service_no, event_sequence, from_status_code,
     to_status_code, reason_code, actor_type_code, actor_reference,
     evidence_digest, content_digest)
VALUES (?, ?, ?, ?, ?, ?, ?, 'system', 'lifecycle-processor', ?, ?)`,
		eventNo, operationNo, serviceNo, sequence, fromStatus, toStatus,
		reason, evidence, content); err != nil {
		return gerror.Wrap(err, "append lifecycle business operation event")
	}
	return nil
}
