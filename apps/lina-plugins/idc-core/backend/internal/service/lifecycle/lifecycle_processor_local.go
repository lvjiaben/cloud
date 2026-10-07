// This file converges reminder, grace, and expiry tasks using local transactional facts.

package lifecycle

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

func (p *processor) convergeLocal(ctx context.Context, taskNo string) error {
	return p.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		state, err := loadTaskState(ctx, p.database, taskNo, true)
		if err != nil || state == nil || state.StatusCode != taskStatusPending {
			return err
		}
		blocked, err := earlierLifecycleWorkExists(ctx, p.database, state)
		if err != nil {
			return err
		}
		if blocked {
			return nil
		}
		if !lifecycleFactsCurrent(state) {
			return transitionLifecycleTask(
				ctx, p.database, state, taskStatusCancelled, "lifecycle.fact_changed", nil, nil, true,
			)
		}
		switch state.ActionCode {
		case actionReminder:
			if state.ReminderSequence == nil {
				return gerror.New("lifecycle reminder is missing its sequence")
			}
			payload, metadata, encodeErr := encodeReminderPayload(state)
			if encodeErr != nil {
				return encodeErr
			}
			if _, err = p.outbox.Enqueue(ctx, outbox.EnqueueInput{
				EventType: "idc.service.lifecycle.reminder.requested", EventVersion: 1,
				Topic: "idc.service.notification", AggregateType: "service",
				AggregateNo: state.ServiceNo, CorrelationNo: state.TaskNo,
				DeduplicationKey: state.IdempotencyDigest,
				Payload:          payload, Metadata: metadata, MaxAttempts: 8,
			}); err != nil {
				return err
			}
		case actionEnterGrace:
			if state.EntitlementStatusCode != entitlementStatusActive {
				return transitionLifecycleTask(
					ctx, p.database, state, taskStatusCancelled, "lifecycle.state_changed", nil, nil, true,
				)
			}
			if err = updateLocalEntitlement(ctx, p.database, state, entitlementStatusGrace); err != nil {
				return err
			}
		case actionMarkExpired:
			if state.EntitlementStatusCode != entitlementStatusActive &&
				state.EntitlementStatusCode != entitlementStatusGrace {
				return transitionLifecycleTask(
					ctx, p.database, state, taskStatusCancelled, "lifecycle.state_changed", nil, nil, true,
				)
			}
			if err = updateLocalEntitlement(ctx, p.database, state, entitlementStatusExpired); err != nil {
				return err
			}
		default:
			return gerror.New("unsupported local lifecycle action")
		}
		return transitionLifecycleTask(
			ctx, p.database, state, taskStatusSucceeded, "lifecycle.local_converged", nil, nil, true,
		)
	})
}

func updateLocalEntitlement(
	ctx context.Context,
	database gdb.DB,
	state *taskState,
	status entitlementStatus,
) error {
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_service_instance
SET entitlement_status_code = ?, version = version + 1,
    updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND service_no = ? AND version = ?
  AND entitlement_expires_at = ?
  AND current_lifecycle_policy_no = ?
  AND current_lifecycle_policy_version = ?`, status, state.ServiceNo,
		state.ServiceVersion, state.ExpectedEntitlementExpiresAt,
		state.PolicyNo, state.PolicyVersion)
	if err != nil {
		return gerror.Wrap(err, "update local service entitlement state")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count local service entitlement update")
	}
	if affected != 1 {
		return gerror.New("local service entitlement version fence changed")
	}
	state.ServiceVersion++
	state.EntitlementStatusCode = status
	return nil
}

func earlierLifecycleWorkExists(
	ctx context.Context,
	database gdb.DB,
	state *taskState,
) (bool, error) {
	value, err := database.GetValue(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM plugin_idc_core_service_lifecycle_task earlier
    INNER JOIN plugin_idc_core_service_lifecycle_task current
        ON current.tenant_id = earlier.tenant_id AND current.task_no = ?
    WHERE earlier.tenant_id = 0
      AND earlier.service_no = current.service_no
      AND earlier.status_code IN ('pending', 'claimed', 'dispatched')
      AND (earlier.due_at < current.due_at
           OR (earlier.due_at = current.due_at AND earlier.id < current.id))
)`, state.TaskNo)
	if err != nil {
		return false, gerror.Wrap(err, "check preceding service lifecycle work")
	}
	return value.Bool(), nil
}

func transitionLifecycleTask(
	ctx context.Context,
	database gdb.DB,
	state *taskState,
	toStatus taskStatus,
	reason string,
	operationNo *string,
	previousRuntime *runtimeStatus,
	completed bool,
) error {
	if state == nil || state.StatusCode == toStatus {
		return nil
	}
	operationArgument := ""
	if operationNo != nil {
		operationArgument = *operationNo
	}
	previousRuntimeArgument := ""
	if previousRuntime != nil {
		previousRuntimeArgument = string(*previousRuntime)
	}
	result, err := database.Exec(ctx, `
UPDATE plugin_idc_core_service_lifecycle_task
SET status_code = ?,
    business_operation_no = COALESCE(NULLIF(?, ''), business_operation_no),
    previous_runtime_status_code = COALESCE(NULLIF(?, ''), previous_runtime_status_code),
    claimed_at = CASE WHEN ? THEN COALESCE(claimed_at, CURRENT_TIMESTAMP) ELSE claimed_at END,
    dispatched_at = CASE WHEN ? THEN COALESCE(dispatched_at, CURRENT_TIMESTAMP) ELSE dispatched_at END,
    completed_at = CASE WHEN ? THEN COALESCE(completed_at, CURRENT_TIMESTAMP) ELSE completed_at END,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND task_no = ? AND status_code = ? AND version = ?`,
		toStatus, operationArgument, previousRuntimeArgument,
		toStatus == taskStatusClaimed, toStatus == taskStatusDispatched, completed,
		state.TaskNo, state.StatusCode, state.TaskVersion)
	if err != nil {
		return gerror.Wrap(err, "transition service lifecycle task")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "count service lifecycle task transition")
	}
	if affected != 1 {
		return gerror.New("service lifecycle task version fence changed")
	}
	eventSequence := state.TaskVersion + 1
	evidence, content, eventNo, err := lifecycleTransitionDigests(
		state.TaskNo, eventSequence, state.StatusCode, toStatus, reason,
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
		eventNo, state.TaskNo, state.ServiceNo, eventSequence, state.StatusCode,
		toStatus, reason, evidence, content); err != nil {
		return gerror.Wrap(err, "append service lifecycle task event")
	}
	state.StatusCode = toStatus
	state.TaskVersion = eventSequence
	state.BusinessOperationNo = operationNo
	state.PreviousRuntimeStatusCode = previousRuntime
	return nil
}

func lifecycleTransitionDigests(
	taskNo string,
	sequence int64,
	fromStatus taskStatus,
	toStatus taskStatus,
	reason string,
) (string, string, string, error) {
	evidence, err := digest(struct {
		TaskNo, Reason       string
		FromStatus, ToStatus taskStatus
		Sequence             int64
	}{
		TaskNo: taskNo, Reason: reason, FromStatus: fromStatus,
		ToStatus: toStatus, Sequence: sequence,
	})
	if err != nil {
		return "", "", "", err
	}
	content, err := digest(struct {
		Evidence, Actor string
	}{evidence, "lifecycle-processor"})
	if err != nil {
		return "", "", "", err
	}
	return evidence, content, "LTE_" + evidence[:32], nil
}
