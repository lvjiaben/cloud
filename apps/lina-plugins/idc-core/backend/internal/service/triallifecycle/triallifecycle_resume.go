// This file compensates a confirmed suspension that completes after formal
// payment. Recovery is fenced to that exact formal entitlement revision; it
// cannot resume a service suspended later by an administrator or reset expiry.

package triallifecycle

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-core/backend/cap/lifecycleresourcecap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/service/asynctask"
	"lina-plugin-idc-core/backend/internal/service/catalog"
)

// errResultConflict freezes a real external result that cannot safely project.
var errResultConflict = gerror.New("resource lifecycle result conflicts with the current entitlement")

// maximumResumeRetries bounds fresh operations after confirmed failures.
// Pending and ambiguous supplier results never enter this path.
const maximumResumeRetries = 3

// scheduleFailedResumeRetry preserves the failed task and creates a new intent
// only after a confirmed failed resume. Reusing the old provider task would
// return its immutable failure forever. New attempts still fence the paid
// entitlement revision and never perform any financial operation.
func (s *serviceImpl) scheduleFailedResumeRetry(ctx context.Context, task asynctask.ClaimedTask, payload taskPayload, row *entitlementState) error {
	if payload.Request.Action != lifecycleresourcecap.ActionResume ||
		payload.RecoveryAttempt >= maximumResumeRetries || !canDispatch(row, payload.Request, time.Now()) {
		return nil
	}
	payload.RecoveryAttempt++
	seed := sha256.Sum256([]byte(fmt.Sprintf("%s:confirmed-failure:%d", payload.Request.IdempotencyDigest, payload.RecoveryAttempt)))
	digest := hex.EncodeToString(seed[:])
	payload.Request.LifecycleTaskNo = "TLT_" + digest[:32]
	payload.Request.IdempotencyDigest = digest
	payload.ProviderTaskNo = ""
	payload.Observation = 0
	encoded, err := json.Marshal(payload)
	if err != nil {
		return gerror.Wrap(err, "freeze failed resume retry")
	}
	_, err = s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
		TaskType: resumeTaskType, SubjectType: subjectType, SubjectNo: task.SubjectNo,
		CustomerNo: task.CustomerNo, IdempotencyKey: digest, Payload: encoded,
		AvailableAt: time.Now().Add(time.Duration(1<<payload.RecoveryAttempt) * 30 * time.Second), MaxAttempts: 12,
	})
	return err
}

// validTaskAction prevents a replay from using a suspension queue for recovery.
func validTaskAction(kind string, action lifecycleresourcecap.Action) bool {
	return kind == taskType && action == lifecycleresourcecap.ActionSuspend ||
		kind == resumeTaskType && action == lifecycleresourcecap.ActionResume
}

// canDispatch checks the current owner facts before the first external action.
// Observations always read the frozen dispatched task, even after a revision.
func canDispatch(row *entitlementState, request lifecycleresourcecap.TaskRequest, now time.Time) bool {
	if row.Version != request.ExpectedServiceVersion || row.ExpiresAt == nil || !row.ExpiresAt.Equal(request.ExpectedExpiresAt) {
		return false
	}
	if request.Action == lifecycleresourcecap.ActionResume {
		return row.ChargeTypeCode != string(catalog.ChargeTypeTrial) && row.StatusCode == statusSuspended && row.ExpiresAt.After(now)
	}
	return row.ChargeTypeCode == string(catalog.ChargeTypeTrial) && row.StatusCode == statusActive && !row.ExpiresAt.After(now)
}

// applySuccess changes the visible state only for confirmed resource results.
func (s *serviceImpl) applySuccess(ctx context.Context, task asynctask.ClaimedTask, payload taskPayload, row *entitlementState) error {
	if payload.Request.Action == lifecycleresourcecap.ActionSuspend {
		return s.applySuspension(ctx, task, payload, row)
	}
	if !canDispatch(row, payload.Request, time.Now()) {
		return errResultConflict
	}
	result, err := s.database.Model(do.ServiceEntitlement{}).Ctx(ctx).
		Where(do.ServiceEntitlement{TenantId: 0, EntitlementNo: task.SubjectNo, CustomerNo: task.CustomerNo, Version: row.Version}).
		Data(do.ServiceEntitlement{StatusCode: statusActive, Version: row.Version + 1}).Update()
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return gerror.New("trial recovery entitlement revision changed")
	}
	return nil
}

// scheduleResume records compensation in the same transaction as the confirmed
// late suspension. It uses a new immutable identity and preserves paid expiry.
func (s *serviceImpl) scheduleResume(ctx context.Context, task asynctask.ClaimedTask, original taskPayload, row *entitlementState) error {
	seed := sha256.Sum256([]byte(fmt.Sprintf("%s:resume:%d", original.Request.IdempotencyDigest, row.Version)))
	digest := hex.EncodeToString(seed[:])
	request := lifecycleresourcecap.TaskRequest{
		LifecycleTaskNo: "TLT_" + digest[:32], ServiceNo: task.SubjectNo, CustomerNo: task.CustomerNo,
		Action: lifecycleresourcecap.ActionResume, ExpectedServiceVersion: row.Version,
		ExpectedExpiresAt: *row.ExpiresAt, IdempotencyDigest: digest,
	}
	if err := lifecycleresourcecap.ValidateTaskRequest(request); err != nil {
		return err
	}
	payload, err := json.Marshal(taskPayload{Request: request})
	if err != nil {
		return gerror.Wrap(err, "freeze formal entitlement recovery")
	}
	_, err = s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{
		TaskType: resumeTaskType, SubjectType: subjectType, SubjectNo: task.SubjectNo, CustomerNo: task.CustomerNo,
		IdempotencyKey: digest, Payload: payload, MaxAttempts: 12,
	})
	return err
}

// ScheduleRecovery freezes the exact paid revision before the transaction commits.
func (s *recoveryScheduler) ScheduleRecovery(ctx context.Context, entitlementNo, customerNo string) error {
	var row *entitlementState
	if err := s.database.GetScan(ctx, &row, `SELECT customer_no,status_code,charge_type_code,version,expires_at
 FROM plugin_idc_core_service_entitlement WHERE tenant_id=0 AND entitlement_no=? AND customer_no=?
 AND entitlement_owner_id=? AND upstream_service_no='' FOR UPDATE`, entitlementNo, customerNo, resourceOwner); err != nil {
		return err
	}
	if row == nil || row.ChargeTypeCode != string(catalog.ChargeTypeRecurring) || row.StatusCode != statusSuspended || row.ExpiresAt == nil {
		return gerror.New("paid trial recovery has no eligible owned entitlement")
	}
	seed := sha256.Sum256([]byte(fmt.Sprintf("%s:paid:%s:%d", resumeTaskType, entitlementNo, row.Version)))
	digest := hex.EncodeToString(seed[:])
	request := lifecycleresourcecap.TaskRequest{LifecycleTaskNo: "TLT_" + digest[:32], ServiceNo: entitlementNo, CustomerNo: customerNo,
		Action: lifecycleresourcecap.ActionResume, ExpectedServiceVersion: row.Version, ExpectedExpiresAt: *row.ExpiresAt, IdempotencyDigest: digest}
	if err := lifecycleresourcecap.ValidateTaskRequest(request); err != nil {
		return err
	}
	payload, err := json.Marshal(taskPayload{Request: request})
	if err != nil {
		return gerror.Wrap(err, "freeze paid trial recovery")
	}
	_, err = s.tasks.CreateInDatabase(ctx, s.database, asynctask.CreateInput{TaskType: resumeTaskType, SubjectType: subjectType,
		SubjectNo: entitlementNo, CustomerNo: customerNo, IdempotencyKey: digest, Payload: payload, MaxAttempts: 12})
	return err
}
