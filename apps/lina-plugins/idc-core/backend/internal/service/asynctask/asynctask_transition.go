// This file implements live-lease transitions, unknown recovery, and narrow manual takeover.

package asynctask

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// Renew extends one still-live, completely fenced lease.
func (s *serviceImpl) Renew(ctx context.Context, lease LeaseRef) (*ClaimedTask, error) {
	normalized, err := normalizeLeaseRef(lease)
	if err != nil {
		return nil, err
	}
	var claimed *ClaimedTask
	err = dao.AsyncTask.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		task, err := loadLiveLeaseTask(ctx, normalized, now)
		if err != nil {
			return err
		}
		expiresAt := databaseTime(now.Add(s.config.LeaseDuration))
		columns := dao.AsyncTask.Columns()
		result, err := dao.AsyncTask.Ctx(ctx).
			Where(do.AsyncTask{
				TenantId:     normalized.TenantID,
				TaskNo:       normalized.TaskNo,
				StatusCode:   string(StatusRunning),
				LockedBy:     normalized.LeaseToken,
				AttemptCount: normalized.AttemptNo,
			}).
			WhereGT(columns.LeaseExpiresAt, now).
			Data(do.AsyncTask{LeaseExpiresAt: &expiresAt}).Update()
		if err != nil {
			return gerror.Wrap(err, "renew asynchronous task lease")
		}
		if err = requireAffected(result, 1, "renew asynchronous task lease"); err != nil {
			return err
		}
		task.LeaseExpiresAt = &expiresAt
		projected, err := projectTask(task)
		if err != nil {
			return err
		}
		claimed = &ClaimedTask{Task: projected, Lease: normalized, LeaseExpiresAt: expiresAt}
		return nil
	})
	if err != nil {
		return nil, publicTransitionError(err)
	}
	return claimed, nil
}

// Complete closes one still-live attempt with non-empty structured success evidence.
func (s *serviceImpl) Complete(ctx context.Context, input CompleteInput) (*Task, error) {
	finishInput, err := s.normalizeCompleteInput(input)
	if err != nil {
		return nil, err
	}
	return s.finishAttempt(ctx, finishInput)
}

// CompleteInDatabase closes a successful attempt inside the caller's existing
// business transaction so the business mutation and durable evidence cannot split.
func (s *serviceImpl) CompleteInDatabase(
	ctx context.Context,
	database gdb.DB,
	input CompleteInput,
) (*Task, error) {
	finishInput, err := s.normalizeCompleteInput(input)
	if err != nil {
		return nil, err
	}
	if database == nil || gdb.TXFromCtx(ctx, database.GetGroup()) == nil {
		return nil, bizerr.NewCode(CodeTransactionRequired)
	}
	task, err := s.finishAttemptInDatabase(ctx, database, finishInput)
	if err != nil {
		return nil, publicTransitionError(err)
	}
	return task, nil
}

func (s *serviceImpl) normalizeCompleteInput(input CompleteInput) (finishAttemptInput, error) {
	lease, err := normalizeLeaseRef(input.Lease)
	if err != nil {
		return finishAttemptInput{}, err
	}
	input.ExternalReferenceNo = strings.TrimSpace(input.ExternalReferenceNo)
	resultSnapshot, err := normalizeNonEmptyJSONObject(input.ResultSnapshot, s.config.MaximumResultSize)
	if err != nil || !validOptionalString(input.ExternalReferenceNo, maximumExternalRefLength) {
		return finishAttemptInput{}, bizerr.NewCode(CodeInvalid)
	}
	return finishAttemptInput{
		lease:               lease,
		attemptStatus:       AttemptStatusSucceeded,
		taskStatus:          StatusSucceeded,
		externalReferenceNo: input.ExternalReferenceNo,
		responseSnapshot:    resultSnapshot,
		resultSnapshot:      resultSnapshot,
	}, nil
}

// Fail closes one still-live attempt and chooses retry, dead letter, or failure explicitly.
func (s *serviceImpl) Fail(ctx context.Context, input FailureInput) (*Task, error) {
	lease, err := normalizeLeaseRef(input.Lease)
	if err != nil {
		return nil, err
	}
	input.ExternalReferenceNo = strings.TrimSpace(input.ExternalReferenceNo)
	input.ErrorCode = strings.TrimSpace(input.ErrorCode)
	input.ErrorSummary = strings.TrimSpace(input.ErrorSummary)
	response, err := normalizeJSONObject(input.ResponseSnapshot, s.config.MaximumResultSize)
	if err != nil || !validOptionalString(input.ExternalReferenceNo, maximumExternalRefLength) ||
		!validRequiredString(input.ErrorCode, maximumErrorCodeLength) ||
		!validRequiredRedactedText(input.ErrorSummary, maximumSummaryLength) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var task *Task
	err = dao.AsyncTask.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		row, err := loadLiveLeaseTask(ctx, lease, now)
		if err != nil {
			return err
		}
		targetStatus := StatusFailed
		var (
			availableAt = row.AvailableAt
			completedAt *time.Time
		)
		if input.Retryable {
			if row.AttemptCount < row.MaxAttempts {
				targetStatus = StatusRetryWait
				next := databaseTime(now.Add(s.retryBackoff(row.TaskNo, row.AttemptCount)))
				availableAt = &next
			} else {
				targetStatus = StatusDeadLetter
				completedAt = &now
			}
		} else {
			completedAt = &now
		}
		if err = closeAttempt(ctx, row, AttemptStatusFailed, input.ExternalReferenceNo, response, input.ErrorCode, input.ErrorSummary, now); err != nil {
			return err
		}
		result, err := dao.AsyncTask.Ctx(ctx).
			Where(do.AsyncTask{
				TenantId:     lease.TenantID,
				TaskNo:       lease.TaskNo,
				StatusCode:   string(StatusRunning),
				LockedBy:     lease.LeaseToken,
				AttemptCount: lease.AttemptNo,
			}).
			WhereGT(dao.AsyncTask.Columns().LeaseExpiresAt, now).
			Data(do.AsyncTask{
				StatusCode:          string(targetStatus),
				ExternalReferenceNo: input.ExternalReferenceNo,
				ResultSnapshot:      response,
				AvailableAt:         availableAt,
				LockedBy:            "",
				CompletedAt:         completedAt,
				LastErrorCode:       input.ErrorCode,
				LastErrorSummary:    input.ErrorSummary,
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "record asynchronous task failure")
		}
		if err = requireAffected(result, 1, "record asynchronous task failure"); err != nil {
			return err
		}
		if err = clearTaskLeaseByIDs(ctx, []int64{row.Id}); err != nil {
			return err
		}
		row.StatusCode = string(targetStatus)
		row.ExternalReferenceNo = input.ExternalReferenceNo
		row.ResultSnapshot = string(response)
		row.AvailableAt = availableAt
		row.LockedBy = ""
		row.LeaseExpiresAt = nil
		row.CompletedAt = completedAt
		row.LastErrorCode = input.ErrorCode
		row.LastErrorSummary = input.ErrorSummary
		projected, err := projectTask(row)
		if err != nil {
			return err
		}
		task = &projected
		return nil
	})
	if err != nil {
		return nil, publicTransitionError(err)
	}
	return task, nil
}

// MarkResultUnknown freezes one still-live attempt without scheduling retry.
func (s *serviceImpl) MarkResultUnknown(ctx context.Context, input UnknownInput) (*Task, error) {
	lease, err := normalizeLeaseRef(input.Lease)
	if err != nil {
		return nil, err
	}
	input.ExternalReferenceNo = strings.TrimSpace(input.ExternalReferenceNo)
	input.ErrorCode = strings.TrimSpace(input.ErrorCode)
	input.ErrorSummary = strings.TrimSpace(input.ErrorSummary)
	response, err := normalizeJSONObject(input.ResponseSnapshot, s.config.MaximumResultSize)
	if err != nil || !validOptionalString(input.ExternalReferenceNo, maximumExternalRefLength) ||
		!validRequiredString(input.ErrorCode, maximumErrorCodeLength) ||
		!validRequiredRedactedText(input.ErrorSummary, maximumSummaryLength) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	return s.finishAttempt(ctx, finishAttemptInput{
		lease:               lease,
		attemptStatus:       AttemptStatusResultUnknown,
		taskStatus:          StatusResultUnknown,
		externalReferenceNo: input.ExternalReferenceNo,
		responseSnapshot:    response,
		resultSnapshot:      response,
		errorCode:           input.ErrorCode,
		errorSummary:        input.ErrorSummary,
	})
}

// finishAttemptInput contains one validated terminal-attempt transition.
type finishAttemptInput struct {
	lease               LeaseRef
	attemptStatus       AttemptStatus
	taskStatus          Status
	externalReferenceNo string
	responseSnapshot    json.RawMessage
	resultSnapshot      json.RawMessage
	errorCode           string
	errorSummary        string
}

// finishAttempt atomically closes one attempt and its owning task.
func (s *serviceImpl) finishAttempt(ctx context.Context, input finishAttemptInput) (*Task, error) {
	var task *Task
	err := dao.AsyncTask.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		row, err := loadLiveLeaseTask(ctx, input.lease, now)
		if err != nil {
			return err
		}
		if err = closeAttempt(ctx, row, input.attemptStatus, input.externalReferenceNo, input.responseSnapshot, input.errorCode, input.errorSummary, now); err != nil {
			return err
		}
		result, err := dao.AsyncTask.Ctx(ctx).
			Where(do.AsyncTask{
				TenantId:     input.lease.TenantID,
				TaskNo:       input.lease.TaskNo,
				StatusCode:   string(StatusRunning),
				LockedBy:     input.lease.LeaseToken,
				AttemptCount: input.lease.AttemptNo,
			}).
			WhereGT(dao.AsyncTask.Columns().LeaseExpiresAt, now).
			Data(do.AsyncTask{
				StatusCode:          string(input.taskStatus),
				ExternalReferenceNo: input.externalReferenceNo,
				ResultSnapshot:      input.resultSnapshot,
				LockedBy:            "",
				CompletedAt:         &now,
				LastErrorCode:       input.errorCode,
				LastErrorSummary:    input.errorSummary,
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "finish asynchronous task")
		}
		if err = requireAffected(result, 1, "finish asynchronous task"); err != nil {
			return err
		}
		if err = clearTaskLeaseByIDs(ctx, []int64{row.Id}); err != nil {
			return err
		}
		row.StatusCode = string(input.taskStatus)
		row.ExternalReferenceNo = input.externalReferenceNo
		row.ResultSnapshot = string(input.resultSnapshot)
		row.LockedBy = ""
		row.LeaseExpiresAt = nil
		row.CompletedAt = &now
		row.LastErrorCode = input.errorCode
		row.LastErrorSummary = input.errorSummary
		projected, err := projectTask(row)
		if err != nil {
			return err
		}
		task = &projected
		return nil
	})
	if err != nil {
		return nil, publicTransitionError(err)
	}
	return task, nil
}

func (s *serviceImpl) finishAttemptInDatabase(
	ctx context.Context,
	database gdb.DB,
	input finishAttemptInput,
) (*Task, error) {
	now := databaseTime(s.now())
	var row *entity.AsyncTask
	columns := dao.AsyncTask.Columns()
	err := database.Model(dao.AsyncTask.Table()).Safe().Ctx(ctx).
		Where(do.AsyncTask{
			TenantId:     input.lease.TenantID,
			TaskNo:       input.lease.TaskNo,
			StatusCode:   string(StatusRunning),
			LockedBy:     input.lease.LeaseToken,
			AttemptCount: input.lease.AttemptNo,
		}).
		WhereNotNull(columns.LeaseExpiresAt).
		WhereGT(columns.LeaseExpiresAt, now).
		LockUpdate().
		Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "load live asynchronous task lease")
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeLeaseLost)
	}
	attemptResult, err := database.Model(dao.AsyncTaskAttempt.Table()).Safe().Ctx(ctx).
		Where(do.AsyncTaskAttempt{
			TenantId:   row.TenantId,
			TaskNo:     row.TaskNo,
			AttemptNo:  row.AttemptCount,
			StatusCode: string(AttemptStatusRunning),
		}).
		Data(do.AsyncTaskAttempt{
			StatusCode:          string(input.attemptStatus),
			ExternalReferenceNo: input.externalReferenceNo,
			ResponseSnapshot:    input.responseSnapshot,
			EndedAt:             &now,
			ErrorCode:           input.errorCode,
			ErrorSummary:        input.errorSummary,
		}).Update()
	if err != nil {
		return nil, gerror.Wrap(err, "close asynchronous task attempt")
	}
	if err = requireAffected(attemptResult, 1, "close asynchronous task attempt"); err != nil {
		return nil, err
	}
	taskResult, err := database.Model(dao.AsyncTask.Table()).Safe().Ctx(ctx).
		Where(do.AsyncTask{
			TenantId:     input.lease.TenantID,
			TaskNo:       input.lease.TaskNo,
			StatusCode:   string(StatusRunning),
			LockedBy:     input.lease.LeaseToken,
			AttemptCount: input.lease.AttemptNo,
		}).
		WhereGT(columns.LeaseExpiresAt, now).
		Data(do.AsyncTask{
			StatusCode:          string(input.taskStatus),
			ExternalReferenceNo: input.externalReferenceNo,
			ResultSnapshot:      input.resultSnapshot,
			LockedBy:            "",
			CompletedAt:         &now,
			LastErrorCode:       input.errorCode,
			LastErrorSummary:    input.errorSummary,
		}).Update()
	if err != nil {
		return nil, gerror.Wrap(err, "finish asynchronous task")
	}
	if err = requireAffected(taskResult, 1, "finish asynchronous task"); err != nil {
		return nil, err
	}
	leaseResult, err := database.Model(dao.AsyncTask.Table()).Safe().Ctx(ctx).
		Where(columns.Id, row.Id).
		Data(columns.LeaseExpiresAt, nil).
		Update()
	if err != nil {
		return nil, gerror.Wrap(err, "clear asynchronous task lease expiry")
	}
	if err = requireAffected(leaseResult, 1, "clear asynchronous task lease expiry"); err != nil {
		return nil, err
	}
	row.StatusCode = string(input.taskStatus)
	row.ExternalReferenceNo = input.externalReferenceNo
	row.ResultSnapshot = string(input.resultSnapshot)
	row.LockedBy = ""
	row.LeaseExpiresAt = nil
	row.CompletedAt = &now
	row.LastErrorCode = input.errorCode
	row.LastErrorSummary = input.errorSummary
	projected, err := projectTask(row)
	if err != nil {
		return nil, err
	}
	return &projected, nil
}

// RecoverExpired freezes bounded expired running work as result unknown.
func (s *serviceImpl) RecoverExpired(ctx context.Context, input RecoverExpiredInput) ([]Task, error) {
	normalized, err := s.normalizeClaimInput(ClaimInput{WorkerID: "lease-recovery", Limit: input.Limit, TaskTypes: input.TaskTypes})
	if err != nil {
		return nil, err
	}
	input.Limit = normalized.Limit
	recovered := make([]Task, 0)
	err = dao.AsyncTask.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		columns := dao.AsyncTask.Columns()
		var rows []entity.AsyncTask
		model := dao.AsyncTask.Ctx(ctx).
			Where(do.AsyncTask{TenantId: idccap.DefaultTenantID, StatusCode: string(StatusRunning)}).
			WhereNotNull(columns.LeaseExpiresAt).
			WhereLTE(columns.LeaseExpiresAt, now)
		if len(normalized.TaskTypes) > 0 {
			model = model.WhereIn(columns.TaskType, normalized.TaskTypes)
		}
		err := model.
			OrderAsc(columns.LeaseExpiresAt).
			OrderAsc(columns.Id).
			Limit(input.Limit).
			LockUpdateSkipLocked().
			Scan(&rows)
		if err != nil {
			return gerror.Wrap(err, "load expired asynchronous task leases")
		}
		if len(rows) == 0 {
			return nil
		}
		ids := make([]int64, len(rows))
		taskNos := make([]string, len(rows))
		expectedAttemptByTask := make(map[string]int, len(rows))
		for index := range rows {
			ids[index] = rows[index].Id
			taskNos[index] = rows[index].TaskNo
			expectedAttemptByTask[rows[index].TaskNo] = rows[index].AttemptCount
		}
		var runningAttempts []entity.AsyncTaskAttempt
		attemptColumns := dao.AsyncTaskAttempt.Columns()
		err = dao.AsyncTaskAttempt.Ctx(ctx).
			Where(do.AsyncTaskAttempt{TenantId: idccap.DefaultTenantID, StatusCode: string(AttemptStatusRunning)}).
			WhereIn(attemptColumns.TaskNo, taskNos).
			OrderAsc(attemptColumns.TaskNo).
			OrderAsc(attemptColumns.AttemptNo).
			LockUpdate().
			Scan(&runningAttempts)
		if err != nil {
			return gerror.Wrap(err, "load expired asynchronous task attempts")
		}
		if len(runningAttempts) != len(rows) {
			return gerror.Newf(
				"expired asynchronous task attempt cardinality mismatch: tasks=%d attempts=%d",
				len(rows),
				len(runningAttempts),
			)
		}
		attemptIDs := make([]int64, len(runningAttempts))
		seenTasks := make(map[string]struct{}, len(runningAttempts))
		for index := range runningAttempts {
			attempt := &runningAttempts[index]
			expectedAttempt, exists := expectedAttemptByTask[attempt.TaskNo]
			if !exists || expectedAttempt != attempt.AttemptNo {
				return gerror.Newf(
					"expired asynchronous task attempt fence mismatch: task=%s expected=%d actual=%d",
					attempt.TaskNo,
					expectedAttempt,
					attempt.AttemptNo,
				)
			}
			if _, duplicate := seenTasks[attempt.TaskNo]; duplicate {
				return gerror.Newf("multiple running attempts found for expired asynchronous task: task=%s", attempt.TaskNo)
			}
			seenTasks[attempt.TaskNo] = struct{}{}
			attemptIDs[index] = attempt.Id
		}
		attemptResult, err := dao.AsyncTaskAttempt.Ctx(ctx).
			WhereIn(attemptColumns.Id, attemptIDs).
			Where(do.AsyncTaskAttempt{TenantId: idccap.DefaultTenantID, StatusCode: string(AttemptStatusRunning)}).
			Data(do.AsyncTaskAttempt{
				StatusCode:   string(AttemptStatusResultUnknown),
				EndedAt:      &now,
				ErrorCode:    leaseExpiredErrorCode,
				ErrorSummary: leaseExpiredErrorSummary,
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "close expired asynchronous task attempts")
		}
		if err = requireAffected(attemptResult, len(rows), "close expired asynchronous task attempts"); err != nil {
			return err
		}
		taskResult, err := dao.AsyncTask.Ctx(ctx).
			WhereIn(columns.Id, ids).
			Data(do.AsyncTask{
				StatusCode:       string(StatusResultUnknown),
				LockedBy:         "",
				CompletedAt:      &now,
				LastErrorCode:    leaseExpiredErrorCode,
				LastErrorSummary: leaseExpiredErrorSummary,
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "freeze expired asynchronous tasks")
		}
		if err = requireAffected(taskResult, len(rows), "freeze expired asynchronous tasks"); err != nil {
			return err
		}
		if err = clearTaskLeaseByIDs(ctx, ids); err != nil {
			return err
		}
		recovered = make([]Task, 0, len(rows))
		for index := range rows {
			rows[index].StatusCode = string(StatusResultUnknown)
			rows[index].LockedBy = ""
			rows[index].LeaseExpiresAt = nil
			rows[index].CompletedAt = &now
			rows[index].LastErrorCode = leaseExpiredErrorCode
			rows[index].LastErrorSummary = leaseExpiredErrorSummary
			projected, err := projectTask(&rows[index])
			if err != nil {
				return err
			}
			recovered = append(recovered, projected)
		}
		return nil
	})
	if err != nil {
		return nil, publicTransitionError(err)
	}
	return recovered, nil
}

// TakeOver only assigns already terminal automated work to an internal operator.
func (s *serviceImpl) TakeOver(ctx context.Context, input TakeOverInput) (*Task, error) {
	input.TaskNo = strings.TrimSpace(input.TaskNo)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.TenantID != idccap.DefaultTenantID ||
		!validRequiredString(input.TaskNo, maximumTaskNoLength) || input.AssigneeUserID < 1 ||
		!validRequiredRedactedText(input.Reason, maximumManualReasonLength) {
		return nil, bizerr.NewCode(CodeInvalid)
	}
	var task *Task
	err := dao.AsyncTask.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		var row *entity.AsyncTask
		err := dao.AsyncTask.Ctx(ctx).
			Where(do.AsyncTask{TenantId: input.TenantID, TaskNo: input.TaskNo}).
			WhereIn(dao.AsyncTask.Columns().StatusCode, []Status{
				StatusFailed,
				StatusDeadLetter,
				StatusResultUnknown,
			}).
			LockUpdate().
			Scan(&row)
		if err != nil {
			return gerror.Wrap(err, "load asynchronous task for manual takeover")
		}
		if row == nil {
			return bizerr.NewCode(CodeStateConflict)
		}
		now := databaseTime(s.now())
		result, err := dao.AsyncTask.Ctx(ctx).
			Where(do.AsyncTask{TenantId: input.TenantID, Id: row.Id, StatusCode: row.StatusCode}).
			Data(do.AsyncTask{
				StatusCode:           string(StatusManualTakeover),
				LockedBy:             "",
				ManualTakeoverAt:     &now,
				ManualAssigneeUserId: input.AssigneeUserID,
				ManualReason:         input.Reason,
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "take over asynchronous task")
		}
		if err = requireAffected(result, 1, "take over asynchronous task"); err != nil {
			return err
		}
		if err = clearTaskLeaseByIDs(ctx, []int64{row.Id}); err != nil {
			return err
		}
		row.StatusCode = string(StatusManualTakeover)
		row.LockedBy = ""
		row.LeaseExpiresAt = nil
		row.ManualTakeoverAt = &now
		row.ManualAssigneeUserId = input.AssigneeUserID
		row.ManualReason = input.Reason
		projected, err := projectTask(row)
		if err != nil {
			return err
		}
		task = &projected
		return nil
	})
	if err != nil {
		return nil, publicTransitionError(err)
	}
	return task, nil
}

// loadLiveLeaseTask locks and returns only a task satisfying the complete live fence.
func loadLiveLeaseTask(ctx context.Context, lease LeaseRef, now time.Time) (*entity.AsyncTask, error) {
	var task *entity.AsyncTask
	err := dao.AsyncTask.Ctx(ctx).
		Where(do.AsyncTask{
			TenantId:     lease.TenantID,
			TaskNo:       lease.TaskNo,
			StatusCode:   string(StatusRunning),
			LockedBy:     lease.LeaseToken,
			AttemptCount: lease.AttemptNo,
		}).
		WhereNotNull(dao.AsyncTask.Columns().LeaseExpiresAt).
		WhereGT(dao.AsyncTask.Columns().LeaseExpiresAt, now).
		LockUpdate().
		Scan(&task)
	if err != nil {
		return nil, gerror.Wrap(err, "load live asynchronous task lease")
	}
	if task == nil {
		return nil, bizerr.NewCode(CodeLeaseLost)
	}
	return task, nil
}

// closeAttempt updates exactly the running attempt selected by the owning task fence.
func closeAttempt(
	ctx context.Context,
	task *entity.AsyncTask,
	status AttemptStatus,
	externalReferenceNo string,
	response json.RawMessage,
	errorCode string,
	errorSummary string,
	endedAt time.Time,
) error {
	result, err := dao.AsyncTaskAttempt.Ctx(ctx).
		Where(do.AsyncTaskAttempt{
			TenantId:   task.TenantId,
			TaskNo:     task.TaskNo,
			AttemptNo:  task.AttemptCount,
			StatusCode: string(AttemptStatusRunning),
		}).
		Data(do.AsyncTaskAttempt{
			StatusCode:          string(status),
			ExternalReferenceNo: externalReferenceNo,
			ResponseSnapshot:    response,
			EndedAt:             &endedAt,
			ErrorCode:           errorCode,
			ErrorSummary:        errorSummary,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "close asynchronous task attempt")
	}
	return requireAffected(result, 1, "close asynchronous task attempt")
}

// clearTaskLeaseByIDs uses the sole narrow fixed-field Data(nil) exception in
// this component. A nil DO pointer is omitted by GoFrame, so this explicit column
// assignment is required to persist SQL NULL instead of a zero timestamp.
func clearTaskLeaseByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	result, err := dao.AsyncTask.Ctx(ctx).
		WhereIn(dao.AsyncTask.Columns().Id, ids).
		Data(dao.AsyncTask.Columns().LeaseExpiresAt, nil).
		Update()
	if err != nil {
		return gerror.Wrap(err, "clear asynchronous task lease expiry")
	}
	return requireAffected(result, len(ids), "clear asynchronous task lease expiry")
}

// projectTask converts generated persistence state into a package-owned projection.
func projectTask(row *entity.AsyncTask) (Task, error) {
	if row == nil || row.AvailableAt == nil {
		return Task{}, gerror.New("persisted asynchronous task is incomplete")
	}
	payload, err := normalizeJSONObject([]byte(row.Payload), maximumStoredJSONSize(row.Payload))
	if err != nil {
		return Task{}, gerror.Wrap(err, "project asynchronous task payload")
	}
	resultSnapshot, err := normalizeJSONObject([]byte(row.ResultSnapshot), maximumStoredJSONSize(row.ResultSnapshot))
	if err != nil {
		return Task{}, gerror.Wrap(err, "project asynchronous task result")
	}
	return Task{
		TenantID:             row.TenantId,
		TaskNo:               row.TaskNo,
		TaskType:             row.TaskType,
		SubjectType:          row.SubjectType,
		SubjectNo:            row.SubjectNo,
		CustomerNo:           row.CustomerNo,
		ExternalReferenceNo:  row.ExternalReferenceNo,
		Status:               Status(row.StatusCode),
		Priority:             row.Priority,
		Payload:              payload,
		ResultSnapshot:       resultSnapshot,
		AvailableAt:          databaseTime(*row.AvailableAt),
		DeadlineAt:           cloneTime(row.DeadlineAt),
		AttemptCount:         row.AttemptCount,
		MaxAttempts:          row.MaxAttempts,
		CompletedAt:          cloneTime(row.CompletedAt),
		ManualTakeoverAt:     cloneTime(row.ManualTakeoverAt),
		ManualAssigneeUserID: row.ManualAssigneeUserId,
		ManualReason:         row.ManualReason,
		LastErrorCode:        row.LastErrorCode,
		LastErrorSummary:     row.LastErrorSummary,
	}, nil
}

// normalizeLeaseRef trims caller values before validating the complete lease syntax.
func normalizeLeaseRef(lease LeaseRef) (LeaseRef, error) {
	lease.TaskNo = strings.TrimSpace(lease.TaskNo)
	lease.LeaseToken = strings.TrimSpace(lease.LeaseToken)
	if err := validateLeaseRef(lease); err != nil {
		return LeaseRef{}, err
	}
	return lease, nil
}

// normalizeNonEmptyJSONObject requires structured success evidence with at least one field.
func normalizeNonEmptyJSONObject(raw json.RawMessage, maximumSize int) (json.RawMessage, error) {
	canonical, err := normalizeJSONObject(raw, maximumSize)
	if err != nil {
		return nil, err
	}
	var object map[string]json.RawMessage
	if err = json.Unmarshal(canonical, &object); err != nil || len(object) == 0 {
		return nil, gerror.New("success evidence must contain at least one field")
	}
	return canonical, nil
}

// maximumStoredJSONSize provides a strict bound while projecting already-persisted JSON.
func maximumStoredJSONSize(value string) int {
	if len(value) < 2 {
		return 2
	}
	return len(value) + 1
}

// cloneTime copies a nullable timestamp at PostgreSQL comparison precision.
func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := databaseTime(*value)
	return &cloned
}

// requireAffected enforces exact cardinality for fenced and batched state transitions.
func requireAffected(result sql.Result, expected int, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrapf(err, "read %s result", operation)
	}
	if affected != int64(expected) {
		return gerror.Newf("%s affected %d rows, expected %d", operation, affected, expected)
	}
	return nil
}

// publicTransitionError preserves stable business errors and hides storage details.
func publicTransitionError(err error) error {
	if err == nil {
		return nil
	}
	if _, businessError := bizerr.As(err); businessError {
		return err
	}
	return bizerr.WrapCode(err, CodeUnavailable)
}
