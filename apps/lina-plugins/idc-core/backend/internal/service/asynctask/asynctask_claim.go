// This file implements bounded skip-locked claiming, attempt batching, and deadline expiry.

package asynctask

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// Claim leases a ready batch and appends all attempt evidence in one transaction.
func (s *serviceImpl) Claim(ctx context.Context, input ClaimInput) ([]ClaimedTask, error) {
	normalized, err := s.normalizeClaimInput(input)
	if err != nil {
		return nil, err
	}
	claimed := make([]ClaimedTask, 0)
	err = dao.AsyncTask.Transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		if err := expireExhaustedTasks(ctx, now, normalized.Limit, normalized.TaskTypes); err != nil {
			return err
		}
		if err := expireMissedDeadlines(ctx, now, normalized.Limit, normalized.TaskTypes); err != nil {
			return err
		}
		rows, err := loadClaimableTasks(ctx, now, normalized.Limit, normalized.TaskTypes)
		if err != nil || len(rows) == 0 {
			return err
		}
		leaseToken, err := newOpaqueToken(s.random, leaseTokenPrefix)
		if err != nil {
			return err
		}
		leaseExpiresAt := databaseTime(now.Add(s.config.LeaseDuration))
		ids := make([]int64, len(rows))
		attempts := make([]do.AsyncTaskAttempt, len(rows))
		for index := range rows {
			ids[index] = rows[index].Id
			attempts[index] = do.AsyncTaskAttempt{
				TenantId:         idccap.DefaultTenantID,
				TaskNo:           rows[index].TaskNo,
				AttemptNo:        rows[index].AttemptCount + 1,
				WorkerId:         normalized.WorkerID,
				StatusCode:       string(AttemptStatusRunning),
				RequestSnapshot:  []byte(rows[index].Payload),
				ResponseSnapshot: []byte("{}"),
			}
		}
		columns := dao.AsyncTask.Columns()
		updateResult, err := dao.AsyncTask.Ctx(ctx).
			WhereIn(columns.Id, ids).
			Data(do.AsyncTask{
				StatusCode:          string(StatusRunning),
				ExternalReferenceNo: "",
				ResultSnapshot:      []byte("{}"),
				LockedBy:            leaseToken,
				LeaseExpiresAt:      &leaseExpiresAt,
				AttemptCount:        gdb.Raw(columns.AttemptCount + " + 1"),
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "lease asynchronous task batch")
		}
		if err = requireAffected(updateResult, len(rows), "lease asynchronous task batch"); err != nil {
			return err
		}
		attemptResult, err := dao.AsyncTaskAttempt.Ctx(ctx).Data(attempts).Batch(len(attempts)).Insert()
		if err != nil {
			return gerror.Wrap(err, "insert asynchronous task attempt batch")
		}
		if err = requireAffected(attemptResult, len(rows), "insert asynchronous task attempt batch"); err != nil {
			return err
		}
		claimed = make([]ClaimedTask, 0, len(rows))
		for index := range rows {
			rows[index].StatusCode = string(StatusRunning)
			rows[index].ExternalReferenceNo = ""
			rows[index].ResultSnapshot = "{}"
			rows[index].LockedBy = leaseToken
			rows[index].LeaseExpiresAt = &leaseExpiresAt
			rows[index].AttemptCount++
			projected, err := projectTask(&rows[index])
			if err != nil {
				return err
			}
			claimed = append(claimed, ClaimedTask{
				Task: projected,
				Lease: LeaseRef{
					TenantID:   idccap.DefaultTenantID,
					TaskNo:     rows[index].TaskNo,
					LeaseToken: leaseToken,
					AttemptNo:  rows[index].AttemptCount,
				},
				LeaseExpiresAt: leaseExpiresAt,
			})
		}
		return nil
	})
	if err != nil {
		if _, businessError := bizerr.As(err); businessError {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return claimed, nil
}

// expireExhaustedTasks converges malformed or migrated ready rows whose retry
// budget is already exhausted so the claim fail-safe cannot strand them forever.
func expireExhaustedTasks(ctx context.Context, now time.Time, limit int, taskTypes []string) error {
	columns := dao.AsyncTask.Columns()
	var rows []entity.AsyncTask
	model := dao.AsyncTask.Ctx(ctx).
		Fields(columns.Id).
		Where(do.AsyncTask{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.StatusCode, []Status{StatusPending, StatusRetryWait}).
		WhereGTE(columns.AttemptCount, gdb.Raw(columns.MaxAttempts))
	if len(taskTypes) > 0 {
		model = model.WhereIn(columns.TaskType, taskTypes)
	}
	err := model.
		OrderAsc(columns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return gerror.Wrap(err, "load attempt-exhausted asynchronous tasks")
	}
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, len(rows))
	for index := range rows {
		ids[index] = rows[index].Id
	}
	result, err := dao.AsyncTask.Ctx(ctx).
		WhereIn(columns.Id, ids).
		Data(do.AsyncTask{
			StatusCode:       string(StatusDeadLetter),
			LockedBy:         "",
			CompletedAt:      &now,
			LastErrorCode:    attemptsExhaustedErrorCode,
			LastErrorSummary: attemptsExhaustedSummary,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "dead-letter attempt-exhausted asynchronous tasks")
	}
	if err = requireAffected(result, len(rows), "dead-letter attempt-exhausted asynchronous tasks"); err != nil {
		return err
	}
	return clearTaskLeaseByIDs(ctx, ids)
}

// loadClaimableTasks selects ready rows in deterministic priority order and
// skips work already locked by another transaction.
func loadClaimableTasks(ctx context.Context, now time.Time, limit int, taskTypes []string) ([]entity.AsyncTask, error) {
	columns := dao.AsyncTask.Columns()
	model := dao.AsyncTask.Ctx(ctx)
	deadline := model.Builder().WhereNull(columns.DeadlineAt).WhereOrGT(columns.DeadlineAt, now)
	model = model.
		Where(do.AsyncTask{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.StatusCode, []Status{StatusPending, StatusRetryWait}).
		WhereLT(columns.AttemptCount, gdb.Raw(columns.MaxAttempts)).
		WhereLTE(columns.AvailableAt, now).
		Where(deadline)
	if len(taskTypes) > 0 {
		model = model.WhereIn(columns.TaskType, taskTypes)
	}
	var rows []entity.AsyncTask
	err := model.
		OrderDesc(columns.Priority).
		OrderAsc(columns.AvailableAt).
		OrderAsc(columns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "claim ready asynchronous tasks")
	}
	return rows, nil
}

// expireMissedDeadlines converges a bounded batch of known-unexecuted tasks to
// failure so they cannot remain claimable or silently start after their deadline.
func expireMissedDeadlines(ctx context.Context, now time.Time, limit int, taskTypes []string) error {
	columns := dao.AsyncTask.Columns()
	var rows []entity.AsyncTask
	model := dao.AsyncTask.Ctx(ctx).
		Fields(columns.Id).
		Where(do.AsyncTask{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.StatusCode, []Status{StatusPending, StatusRetryWait}).
		WhereNotNull(columns.DeadlineAt).
		WhereLTE(columns.DeadlineAt, now)
	if len(taskTypes) > 0 {
		model = model.WhereIn(columns.TaskType, taskTypes)
	}
	err := model.
		OrderAsc(columns.DeadlineAt).
		OrderAsc(columns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return gerror.Wrap(err, "load expired unexecuted asynchronous tasks")
	}
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, len(rows))
	for index := range rows {
		ids[index] = rows[index].Id
	}
	result, err := dao.AsyncTask.Ctx(ctx).
		WhereIn(columns.Id, ids).
		Data(do.AsyncTask{
			StatusCode:       string(StatusFailed),
			LockedBy:         "",
			CompletedAt:      &now,
			LastErrorCode:    deadlineExceededErrorCode,
			LastErrorSummary: deadlineExceededSummary,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "fail expired unexecuted asynchronous tasks")
	}
	if err = requireAffected(result, len(rows), "fail expired unexecuted asynchronous tasks"); err != nil {
		return err
	}
	return clearTaskLeaseByIDs(ctx, ids)
}
