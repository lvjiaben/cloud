// This file claims Provider actions, enforces destructive fences, and persists dispatch evidence.

package lifecycle

import (
	"context"
	"errors"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/internal/service/serviceguard"
)

func (p *processor) claimProviderTask(
	ctx context.Context,
	taskNo string,
) (*lifecycleresourcecap.TaskRequest, error) {
	var request *lifecycleresourcecap.TaskRequest
	err := p.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		state, err := loadTaskState(ctx, p.database, taskNo, true)
		if err != nil || state == nil {
			return err
		}
		if state.StatusCode == taskStatusClaimed {
			request, err = resourceRequest(state)
			return err
		}
		if state.StatusCode != taskStatusPending {
			return nil
		}
		blocked, err := earlierLifecycleWorkExists(ctx, p.database, state)
		if err != nil {
			return err
		}
		if blocked {
			return nil
		}
		nowValue, nowErr := p.database.GetValue(ctx, "SELECT CURRENT_TIMESTAMP")
		if nowErr != nil {
			return gerror.Wrap(nowErr, "read lifecycle processing database time")
		}
		if !providerLifecycleEligible(state, nowValue.Time().UTC()) {
			return transitionLifecycleTask(
				ctx, p.database, state, taskStatusCancelled, "lifecycle.fact_changed", nil, nil, true,
			)
		}
		refundLocked, guardErr := serviceguard.HasActiveRefundLock(ctx, p.database, state.ServiceNo)
		if guardErr != nil {
			return guardErr
		}
		if refundLocked {
			return transitionLifecycleTask(
				ctx, p.database, state, taskStatusManualReview,
				"lifecycle.refund_in_progress", nil, nil, true,
			)
		}
		if state.ActionCode == actionTerminate {
			liveRenewal, checkErr := liveRenewalOperationExists(ctx, p.database, state.ServiceNo)
			if checkErr != nil {
				return checkErr
			}
			if liveRenewal {
				return transitionLifecycleTask(
					ctx, p.database, state, taskStatusManualReview,
					"lifecycle.renewal_in_progress", nil, nil, true,
				)
			}
			previous := state.RuntimeStatusCode
			result, updateErr := p.database.Exec(ctx, `
UPDATE plugin_idc_core_service_instance
SET runtime_status_code = ?, provider_task_status_code = ?,
    version = version + 1, updated_at = CURRENT_TIMESTAMP
WHERE tenant_id = 0 AND service_no = ? AND version = ?
  AND entitlement_expires_at = ?
  AND current_lifecycle_policy_no = ?
  AND current_lifecycle_policy_version = ?
  AND runtime_status_code IN (?, ?, ?)`,
				runtimeStatusTerminating, providerTaskStatusPending,
				state.ServiceNo, state.ServiceVersion, state.ExpectedEntitlementExpiresAt,
				state.PolicyNo, state.PolicyVersion, runtimeStatusRunning,
				runtimeStatusStopped, runtimeStatusSuspended)
			if updateErr != nil {
				return gerror.Wrap(updateErr, "reserve service termination")
			}
			affected, affectedErr := result.RowsAffected()
			if affectedErr != nil {
				return gerror.Wrap(affectedErr, "count service termination reservation")
			}
			if affected != 1 {
				return gerror.New("service termination reservation fence changed")
			}
			state.ServiceVersion++
			state.RuntimeStatusCode = runtimeStatusTerminating
			if err = transitionLifecycleTask(
				ctx, p.database, state, taskStatusClaimed, "lifecycle.provider_claimed",
				nil, &previous, false,
			); err != nil {
				return err
			}
		} else if err = transitionLifecycleTask(
			ctx, p.database, state, taskStatusClaimed, "lifecycle.provider_claimed", nil, nil, false,
		); err != nil {
			return err
		}
		request, err = resourceRequest(state)
		return err
	})
	return request, err
}

func providerLifecycleEligible(state *taskState, now time.Time) bool {
	if !lifecycleFactsCurrent(state) || state.ChannelNo == nil || *state.ChannelNo == "" {
		return false
	}
	switch state.ActionCode {
	case actionSuspend:
		return (state.EntitlementStatusCode == entitlementStatusActive ||
			state.EntitlementStatusCode == entitlementStatusGrace ||
			state.EntitlementStatusCode == entitlementStatusExpired) &&
			(state.RuntimeStatusCode == runtimeStatusRunning ||
				state.RuntimeStatusCode == runtimeStatusStopped)
	case actionResume:
		return state.LifecycleSuspended && state.EntitlementStatusCode == entitlementStatusActive &&
			state.RuntimeStatusCode == runtimeStatusSuspended &&
			state.EntitlementExpiresAt != nil &&
			state.EntitlementExpiresAt.After(now)
	case actionTerminate:
		return (state.EntitlementStatusCode == entitlementStatusActive ||
			state.EntitlementStatusCode == entitlementStatusGrace ||
			state.EntitlementStatusCode == entitlementStatusExpired) &&
			(state.RuntimeStatusCode == runtimeStatusRunning ||
				state.RuntimeStatusCode == runtimeStatusStopped ||
				state.RuntimeStatusCode == runtimeStatusSuspended)
	default:
		return false
	}
}

func liveRenewalOperationExists(
	ctx context.Context,
	database gdb.DB,
	serviceNo string,
) (bool, error) {
	value, err := database.GetValue(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM plugin_idc_core_business_operation operation
    WHERE operation.tenant_id = 0 AND operation.service_no = ?
      AND operation.operation_type_code = 'renewal'
      AND operation.status_code IN (
          'financially_settled', 'ready', 'executing', 'compensating',
          'refunding', 'manual_review'
      )
)`, serviceNo)
	if err != nil {
		return false, gerror.Wrap(err, "check live renewal before termination")
	}
	return value.Bool(), nil
}

func (p *processor) freezeProviderUncertainty(
	ctx context.Context,
	taskNo string,
	cause error,
) error {
	return p.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		state, err := loadTaskState(ctx, p.database, taskNo, true)
		if err != nil || state == nil || state.StatusCode != taskStatusClaimed {
			return err
		}
		if errors.Is(cause, lifecycleresourcecap.ErrCapabilityUnavailable) {
			if err = restoreFailedTermination(ctx, p.database, state); err != nil {
				return err
			}
			return transitionLifecycleTaskWithError(
				ctx, p.database, state, taskStatusFailed, "lifecycle.capability_unavailable",
				"resource.capability_unavailable",
			)
		}
		if err = markProviderReviewState(
			ctx, p.database, state, providerTaskStatusUnknown,
		); err != nil {
			return err
		}
		return transitionLifecycleTask(
			ctx, p.database, state, taskStatusManualReview, "lifecycle.provider_unknown",
			state.BusinessOperationNo, state.PreviousRuntimeStatusCode, true,
		)
	})
}

func (p *processor) persistProviderDispatch(
	ctx context.Context,
	request lifecycleresourcecap.TaskRequest,
	provider *lifecycleresourcecap.Task,
) error {
	return p.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		state, err := loadTaskState(ctx, p.database, request.LifecycleTaskNo, true)
		if err != nil || state == nil {
			return err
		}
		if state.StatusCode == taskStatusDispatched {
			return nil
		}
		if state.StatusCode != taskStatusClaimed {
			return gerror.New("lifecycle Provider dispatch lost its claim")
		}
		operationNo, requestDigest, contentDigest, err := operationDigests(state, provider)
		if err != nil {
			return err
		}
		result, err := p.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation
    (operation_no, operation_type_code, customer_no, service_no, currency_code,
     status_code, financial_settlement_required, execution_mode_code,
     resource_owner_code, channel_no, pool_no, provider_task_no,
     capability_evidence_no, capability_evidence_digest,
     idempotency_digest, request_digest, content_digest,
     expected_service_version, execution_started_at)
VALUES (?, ?, ?, ?, ?, ?, FALSE, 'provider_confirmed',
        'idc-resource', ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (operation_no) DO NOTHING`, operationNo, state.ActionCode,
			state.CustomerNo, state.ServiceNo, state.CurrencyCode,
			operationStatusExecuting, *state.ChannelNo, state.PoolNo, provider.ProviderTaskNo,
			provider.CapabilityEvidenceNo,
			provider.CapabilityEvidenceDigest, state.IdempotencyDigest,
			requestDigest, contentDigest, state.ExpectedServiceVersion)
		if err != nil {
			return gerror.Wrap(err, "create lifecycle business operation")
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return gerror.Wrap(err, "count lifecycle business operation creation")
		}
		if affected == 1 {
			evidence, eventContent, eventNo, digestErr := operationTransitionDigests(
				operationNo, 1, "", operationStatusExecuting, "lifecycle.provider_dispatched",
			)
			if digestErr != nil {
				return digestErr
			}
			if _, err = p.database.Exec(ctx, `
INSERT INTO plugin_idc_core_business_operation_event
    (event_no, operation_no, service_no, event_sequence, from_status_code,
     to_status_code, reason_code, actor_type_code, actor_reference,
     evidence_digest, content_digest)
VALUES (?, ?, ?, 1, NULL, 'executing', 'lifecycle.provider_dispatched',
        'system', 'lifecycle-processor', ?, ?)`, eventNo, operationNo,
				state.ServiceNo, evidence, eventContent); err != nil {
				return gerror.Wrap(err, "append lifecycle operation dispatch event")
			}
		}
		validOperation, verifyErr := p.database.GetValue(ctx, `
SELECT COUNT(*)
FROM plugin_idc_core_business_operation
WHERE tenant_id = 0 AND operation_no = ? AND operation_type_code = ?
  AND customer_no = ? AND service_no = ? AND currency_code = ?
  AND status_code = ? AND execution_mode_code = 'provider_confirmed'
  AND channel_no = ? AND pool_no = ? AND provider_task_no = ?
  AND capability_evidence_no = ? AND capability_evidence_digest = ?
  AND idempotency_digest = ? AND request_digest = ? AND content_digest = ?
  AND expected_service_version = ?`, operationNo, state.ActionCode,
			state.CustomerNo, state.ServiceNo, state.CurrencyCode,
			operationStatusExecuting, *state.ChannelNo, state.PoolNo,
			provider.ProviderTaskNo, provider.CapabilityEvidenceNo,
			provider.CapabilityEvidenceDigest, state.IdempotencyDigest,
			requestDigest, contentDigest, state.ExpectedServiceVersion)
		if verifyErr != nil {
			return gerror.Wrap(verifyErr, "verify lifecycle business operation replay")
		}
		if validOperation.Int() != 1 {
			return gerror.New("lifecycle business operation replay conflicts")
		}
		if err = transitionLifecycleTask(
			ctx, p.database, state, taskStatusDispatched, "lifecycle.provider_dispatched",
			&operationNo, state.PreviousRuntimeStatusCode, false,
		); err != nil {
			return err
		}
		serviceResult, serviceErr := p.database.Exec(ctx, `
UPDATE plugin_idc_core_service_instance
	SET provider_task_status_code = ?, version = version + 1,
	    updated_at = CURRENT_TIMESTAMP
	WHERE tenant_id = 0 AND service_no = ? AND version = ?`,
			providerTaskStatusExecuting, state.ServiceNo, state.ServiceVersion)
		if serviceErr != nil {
			return gerror.Wrap(serviceErr, "mark service lifecycle task executing")
		}
		serviceAffected, serviceErr := serviceResult.RowsAffected()
		if serviceErr != nil {
			return gerror.Wrap(serviceErr, "count service lifecycle dispatch")
		}
		if serviceAffected != 1 {
			return gerror.New("service lifecycle dispatch fence changed")
		}
		return nil
	})
}

func operationDigests(
	state *taskState,
	provider *lifecycleresourcecap.Task,
) (string, string, string, error) {
	seed, err := digest(struct {
		TaskNo string
		Action actionCode
	}{state.TaskNo, state.ActionCode})
	if err != nil {
		return "", "", "", err
	}
	requestDigest, err := digest(struct {
		TaskNo, ServiceNo, Idempotency string
		Action                         actionCode
		ExpectedVersion                int64
		ExpiryUnixMicro                int64
	}{
		TaskNo: state.TaskNo, ServiceNo: state.ServiceNo,
		Idempotency: state.IdempotencyDigest, Action: state.ActionCode,
		ExpectedVersion: state.ExpectedServiceVersion,
		ExpiryUnixMicro: state.ExpectedEntitlementExpiresAt.UTC().UnixMicro(),
	})
	if err != nil {
		return "", "", "", err
	}
	contentDigest, err := digest(struct {
		RequestDigest, ProviderTaskNo, CapabilityDigest string
	}{requestDigest, provider.ProviderTaskNo, provider.CapabilityEvidenceDigest})
	return "LCO_" + seed[:32], requestDigest, contentDigest, err
}

func operationTransitionDigests(
	operationNo string,
	sequence int64,
	fromStatus operationStatus,
	toStatus operationStatus,
	reason string,
) (string, string, string, error) {
	evidence, err := digest(struct {
		OperationNo, Reason  string
		FromStatus, ToStatus operationStatus
		Sequence             int64
	}{
		OperationNo: operationNo, Reason: reason, FromStatus: fromStatus,
		ToStatus: toStatus, Sequence: sequence,
	})
	if err != nil {
		return "", "", "", err
	}
	content, err := digest(struct{ Evidence string }{evidence})
	if err != nil {
		return "", "", "", err
	}
	return evidence, content, "LOE_" + evidence[:32], nil
}

func loadProviderTaskNo(
	ctx context.Context,
	database gdb.DB,
	operationNo string,
) (string, error) {
	value, err := database.GetValue(ctx, `
SELECT provider_task_no
FROM plugin_idc_core_business_operation
WHERE tenant_id = 0 AND operation_no = ?`, operationNo)
	if err != nil {
		return "", gerror.Wrap(err, "load lifecycle Provider task number")
	}
	if value.String() == "" {
		return "", gerror.New("lifecycle Provider task number is unavailable")
	}
	return value.String(), nil
}
