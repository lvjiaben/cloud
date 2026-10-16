// This file submits one idempotent resource operation and observes its durable
// result. Pending observations schedule a successor, never a failed write retry.

package triallifecycle

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// taskPayload carries the original immutable intent across read-only observations.
type taskPayload struct {
	Request         lifecycleresourcecap.TaskRequest
	ProviderTaskNo  string
	Observation     int
	RecoveryAttempt int `json:",omitempty"`
}

// entitlementState is locked before dispatch and before applying a real result.
type entitlementState struct {
	CustomerNo     string
	StatusCode     string
	ChargeTypeCode string
	Version        int64
	ExpiresAt      *time.Time
}

func (s *serviceImpl) process(ctx context.Context, task asynctask.ClaimedTask) error {
	var payload taskPayload
	if err := json.Unmarshal(task.Payload, &payload); err != nil ||
		lifecycleresourcecap.ValidateTaskRequest(payload.Request) != nil ||
		payload.Request.ServiceNo != task.SubjectNo || payload.Request.CustomerNo != task.CustomerNo ||
		payload.Observation < 0 || payload.Observation >= maximumObservations ||
		payload.RecoveryAttempt < 0 || payload.RecoveryAttempt > maximumResumeRetries {
		return s.fail(ctx, task, false, "trial.invalid_payload", "Trial task intent is invalid")
	}
	if task.TaskType == notificationTaskType {
		return s.notify(ctx, task, payload)
	}
	if !validTaskAction(task.TaskType, payload.Request.Action) {
		return s.fail(ctx, task, false, "trial.invalid_action", "Trial suspension action is invalid")
	}
	err := s.database.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		row, err := s.lockEntitlement(ctx, payload.Request)
		if err != nil {
			return err
		}
		if row == nil {
			return gerror.New("trial entitlement ownership is unavailable")
		}
		// Dispatch and recording the successor share the owner's database group.
		// Payment which wins this lock first cancels a not-yet-dispatched suspension.
		if payload.ProviderTaskNo == "" && !canDispatch(row, payload.Request, time.Now()) {
			_, err = s.tasks.CompleteInDatabase(ctx, s.database, asynctask.CompleteInput{
				Lease: task.Lease, ResultSnapshot: json.RawMessage(`{"outcome":"no_longer_due"}`),
			})
			return err
		}
		var provider *lifecycleresourcecap.Task
		if payload.ProviderTaskNo == "" {
			provider, err = s.resource.CreateTask(ctx, payload.Request)
		} else {
			provider, err = s.resource.ReadTask(ctx, lifecycleresourcecap.TaskReadRequest{
				ProviderTaskNo: payload.ProviderTaskNo, Request: payload.Request,
			})
		}
		if err != nil {
			return err
		}
		if provider == nil || lifecycleresourcecap.ValidateTask(*provider) != nil {
			return gerror.New("trial resource result is invalid")
		}
		evidence, err := json.Marshal(provider)
		if err != nil {
			return gerror.Wrap(err, "encode trial resource evidence")
		}
		switch provider.Status {
		case lifecycleresourcecap.TaskStatusPending, lifecycleresourcecap.TaskStatusRunning:
			if payload.Observation+1 >= maximumObservations {
				_, err = s.tasks.MarkResultUnknown(ctx, asynctask.UnknownInput{Lease: task.Lease,
					ExternalReferenceNo: provider.ProviderTaskNo, ResponseSnapshot: evidence,
					ErrorCode: "trial.observation_exhausted", ErrorSummary: "Resource operation still needs reconciliation"})
				return err
			}
			payload.ProviderTaskNo = provider.ProviderTaskNo
			payload.Observation++
			encoded, err := json.Marshal(payload)
			if err != nil {
				return gerror.Wrap(err, "freeze trial observation")
			}
			if _, err = s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
				TaskType: task.TaskType, SubjectType: subjectType, SubjectNo: task.SubjectNo, CustomerNo: task.CustomerNo,
				IdempotencyKey: fmt.Sprintf("%s:observation:%d", payload.Request.IdempotencyDigest, payload.Observation),
				Payload:        encoded, AvailableAt: time.Now().Add(30 * time.Second),
			}); err != nil {
				return err
			}
		case lifecycleresourcecap.TaskStatusSucceeded:
			if err = s.applySuccess(ctx, task, payload, row); err != nil {
				if errors.Is(err, errResultConflict) {
					_, err = s.tasks.MarkResultUnknown(ctx, asynctask.UnknownInput{Lease: task.Lease,
						ExternalReferenceNo: provider.ProviderTaskNo, ResponseSnapshot: evidence,
						ErrorCode: "trial.result_conflict", ErrorSummary: "Resource success conflicts with the current entitlement; reconcile before retry"})
				}
				return err
			}
		case lifecycleresourcecap.TaskStatusFailed:
			_, err = s.tasks.Fail(ctx, asynctask.FailureInput{Lease: task.Lease,
				ExternalReferenceNo: provider.ProviderTaskNo, ResponseSnapshot: evidence,
				ErrorCode: provider.FailureCode, ErrorSummary: "Resource lifecycle operation failed; inspect the resource task"})
			if err != nil {
				return err
			}
			return s.scheduleFailedResumeRetry(ctx, task, payload, row)
		case lifecycleresourcecap.TaskStatusResultUnknown, lifecycleresourcecap.TaskStatusManualReview:
			_, err = s.tasks.MarkResultUnknown(ctx, asynctask.UnknownInput{Lease: task.Lease,
				ExternalReferenceNo: provider.ProviderTaskNo, ResponseSnapshot: evidence,
				ErrorCode: provider.FailureCode, ErrorSummary: "Resource lifecycle operation requires reconciliation"})
			return err
		}
		_, err = s.tasks.CompleteInDatabase(ctx, s.database, asynctask.CompleteInput{
			Lease: task.Lease, ExternalReferenceNo: provider.ProviderTaskNo, ResultSnapshot: evidence,
		})
		return err
	})
	if err != nil {
		if transitionErr := s.fail(ctx, task, true, "trial.source_unavailable", "Trial operation could not be observed; retry the same intent"); transitionErr != nil {
			return gerror.Wrap(transitionErr, "record trial observation retry")
		}
		return gerror.Wrap(err, "observe trial suspension")
	}
	return nil
}

func (s *serviceImpl) lockEntitlement(ctx context.Context, request lifecycleresourcecap.TaskRequest) (*entitlementState, error) {
	var row *entitlementState
	err := s.database.Model(do.ServiceEntitlement{}).Ctx(ctx).
		Fields("customer_no,status_code,charge_type_code,version,expires_at").
		Where(do.ServiceEntitlement{TenantId: 0, EntitlementNo: request.ServiceNo, CustomerNo: request.CustomerNo,
			EntitlementOwnerId: resourceOwner, UpstreamServiceNo: ""}).LockUpdate().Scan(&row)
	return row, err
}

func (s *serviceImpl) fail(ctx context.Context, task asynctask.ClaimedTask, retryable bool, code, summary string) error {
	_, err := s.tasks.Fail(ctx, asynctask.FailureInput{Lease: task.Lease, Retryable: retryable,
		ErrorCode: code, ErrorSummary: summary})
	return err
}

// applySuspension projects confirmed resource success and queues an independent
// reminder in the same owner transaction. Failed notifications cannot undo it.
func (s *serviceImpl) applySuspension(ctx context.Context, task asynctask.ClaimedTask, payload taskPayload, row *entitlementState) error {
	if row.StatusCode != statusActive && row.StatusCode != statusSuspended {
		return errResultConflict
	}
	if row.ChargeTypeCode != string(catalog.ChargeTypeTrial) &&
		(row.StatusCode != statusActive || row.ExpiresAt == nil || !row.ExpiresAt.After(payload.Request.ExpectedExpiresAt) || !row.ExpiresAt.After(time.Now())) {
		return errResultConflict
	}
	if row.StatusCode == statusActive {
		result, err := s.database.Model(do.ServiceEntitlement{}).Ctx(ctx).
			Where(do.ServiceEntitlement{TenantId: 0, EntitlementNo: task.SubjectNo, CustomerNo: task.CustomerNo, Version: row.Version}).
			Data(do.ServiceEntitlement{StatusCode: statusSuspended, Version: row.Version + 1}).Update()
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return gerror.New("trial entitlement version changed")
		}
		row.Version++
	}
	// A suspension dispatched before payment can finish after the formal
	// entitlement has been committed. Preserve that real resource result, then
	// compensate through a separate durable resume; never report active early.
	if row.ChargeTypeCode != string(catalog.ChargeTypeTrial) {
		return s.scheduleResume(ctx, task, payload, row)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
		TaskType: notificationTaskType, SubjectType: subjectType, SubjectNo: task.SubjectNo, CustomerNo: task.CustomerNo,
		IdempotencyKey: payload.Request.IdempotencyDigest + ":notification", Payload: encoded,
	}); err != nil {
		return err
	}
	return nil
}
