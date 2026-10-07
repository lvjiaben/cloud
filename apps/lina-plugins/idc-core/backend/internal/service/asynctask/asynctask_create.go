// This file implements transaction-required, idempotent asynchronous task creation.

package asynctask

import (
	"bytes"
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

// Create persists a task only inside the caller's business transaction.
func (s *serviceImpl) Create(ctx context.Context, input CreateInput) (*CreateResult, error) {
	normalized, err := s.normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}
	transactionGroup := dao.AsyncTask.Group()
	if gdb.TXFromCtx(ctx, transactionGroup) == nil {
		return nil, bizerr.NewCode(CodeTransactionRequired)
	}
	return s.create(ctx, dao.AsyncTask.DB(), transactionGroup, normalized)
}

// CreateInDatabase persists a task through the caller's explicit database and
// therefore joins the transaction associated with that database group.
func (s *serviceImpl) CreateInDatabase(
	ctx context.Context,
	database gdb.DB,
	input CreateInput,
) (*CreateResult, error) {
	normalized, err := s.normalizeCreateInput(input)
	if err != nil {
		return nil, err
	}
	if database == nil {
		return nil, bizerr.NewCode(CodeTransactionRequired)
	}
	return s.create(ctx, database, database.GetGroup(), normalized)
}

func (s *serviceImpl) create(
	ctx context.Context,
	database gdb.DB,
	transactionGroup string,
	input normalizedCreateInput,
) (*CreateResult, error) {
	if gdb.TXFromCtx(ctx, transactionGroup) == nil {
		return nil, bizerr.NewCode(CodeTransactionRequired)
	}
	result, err := s.createInTransaction(ctx, database, input)
	if err != nil {
		if _, businessError := bizerr.As(err); businessError {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return result, nil
}

// createInTransaction converges concurrent requests through persisted unique scopes.
func (s *serviceImpl) createInTransaction(
	ctx context.Context,
	database gdb.DB,
	input normalizedCreateInput,
) (*CreateResult, error) {
	existing, err := loadTaskByRequest(ctx, database, input)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return reuseTask(existing, input)
	}
	if err := validateNewTaskSchedule(input); err != nil {
		return nil, err
	}
	taskNo, err := s.reserveTaskNumber(ctx, database, input)
	if err != nil {
		return nil, err
	}
	insertResult, err := database.Model(dao.AsyncTask.Table()).Safe().Ctx(ctx).Data(do.AsyncTask{
		TenantId:             idccap.DefaultTenantID,
		TaskNo:               taskNo,
		TaskType:             input.TaskType,
		SubjectType:          input.SubjectType,
		SubjectNo:            input.SubjectNo,
		CustomerNo:           input.CustomerNo,
		IdempotencyKeyDigest: input.idempotencyDigest,
		StatusCode:           string(StatusPending),
		Priority:             input.Priority,
		Payload:              input.payload,
		ResultSnapshot:       []byte("{}"),
		AvailableAt:          &input.AvailableAt,
		DeadlineAt:           input.DeadlineAt,
		AttemptCount:         0,
		MaxAttempts:          input.MaxAttempts,
	}).InsertIgnore()
	if err != nil {
		return nil, gerror.Wrap(err, "insert asynchronous task")
	}
	affected, err := insertResult.RowsAffected()
	if err != nil {
		return nil, gerror.Wrap(err, "read asynchronous task insert result")
	}
	created, err := loadTaskByRequest(ctx, database, input)
	if err != nil {
		return nil, err
	}
	if created == nil {
		return nil, gerror.New("asynchronous task insert did not produce a request row")
	}
	if affected == 0 {
		return reuseTask(created, input)
	}
	if created.TaskNo != taskNo {
		return nil, gerror.New("asynchronous task number diverged from its reservation")
	}
	if err = s.assignTaskNumber(ctx, database, created); err != nil {
		return nil, err
	}
	projected, err := projectTask(created)
	if err != nil {
		return nil, err
	}
	return &CreateResult{Task: projected}, nil
}

// loadTaskByRequest reads the row protected by the task request unique constraint.
func loadTaskByRequest(
	ctx context.Context,
	database gdb.DB,
	input normalizedCreateInput,
) (*entity.AsyncTask, error) {
	var task *entity.AsyncTask
	err := database.Model(dao.AsyncTask.Table()).Safe().Ctx(ctx).Where(do.AsyncTask{
		TenantId:             idccap.DefaultTenantID,
		TaskType:             input.TaskType,
		SubjectType:          input.SubjectType,
		SubjectNo:            input.SubjectNo,
		IdempotencyKeyDigest: input.idempotencyDigest,
	}).Scan(&task)
	if err != nil {
		return nil, gerror.Wrap(err, "load asynchronous task by idempotency scope")
	}
	return task, nil
}

// reuseTask rejects a changed immutable request under the same idempotency scope.
func reuseTask(task *entity.AsyncTask, input normalizedCreateInput) (*CreateResult, error) {
	storedPayload, err := normalizeJSONObject([]byte(task.Payload), len(task.Payload)+1)
	if err != nil {
		return nil, gerror.Wrap(err, "read persisted asynchronous task payload")
	}
	if task.CustomerNo != input.CustomerNo ||
		task.Priority != input.Priority ||
		(input.maxAttemptsExplicit && task.MaxAttempts != input.MaxAttempts) ||
		!bytes.Equal(storedPayload, input.payload) ||
		!equalTimePointers(task.DeadlineAt, input.DeadlineAt) ||
		(input.availableAtExplicit && (task.AvailableAt == nil ||
			!databaseTime(*task.AvailableAt).Equal(input.AvailableAt))) {
		return nil, bizerr.NewCode(CodeIdempotencyConflict)
	}
	projected, err := projectTask(task)
	if err != nil {
		return nil, err
	}
	return &CreateResult{Task: projected, Reused: true}, nil
}

// reserveTaskNumber idempotently reserves one opaque number in the caller transaction.
func (s *serviceImpl) reserveTaskNumber(
	ctx context.Context,
	database gdb.DB,
	input normalizedCreateInput,
) (string, error) {
	request := do.BusinessNumber{
		TenantId:               idccap.DefaultTenantID,
		NumberType:             businessNumberTypeTask,
		ReservationScopeDigest: input.reservationScope,
		IdempotencyKeyDigest:   input.idempotencyDigest,
	}
	if existing, err := findReservedTaskNumber(ctx, database, request); err != nil || existing != "" {
		return existing, err
	}
	for range maximumNumberAttempts {
		candidate, err := newOpaqueToken(s.random, taskNumberPrefix)
		if err != nil {
			return "", err
		}
		result, err := database.Model(dao.BusinessNumber.Table()).Safe().Ctx(ctx).Data(do.BusinessNumber{
			TenantId:               idccap.DefaultTenantID,
			NumberType:             businessNumberTypeTask,
			BusinessNo:             candidate,
			ReservationScopeDigest: input.reservationScope,
			IdempotencyKeyDigest:   input.idempotencyDigest,
		}).InsertIgnore()
		if err != nil {
			return "", gerror.Wrap(err, "reserve asynchronous task number")
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return "", gerror.Wrap(err, "read task number reservation result")
		}
		if affected == 1 {
			return candidate, nil
		}
		if existing, err := findReservedTaskNumber(ctx, database, request); err != nil || existing != "" {
			return existing, err
		}
	}
	return "", gerror.New("asynchronous task number collision retry limit reached")
}

// findReservedTaskNumber resolves one reservation by its full idempotency scope.
func findReservedTaskNumber(
	ctx context.Context,
	database gdb.DB,
	request do.BusinessNumber,
) (string, error) {
	var row *entity.BusinessNumber
	if err := database.Model(dao.BusinessNumber.Table()).Safe().Ctx(ctx).Where(request).Scan(&row); err != nil {
		return "", gerror.Wrap(err, "load asynchronous task number reservation")
	}
	if row == nil {
		return "", nil
	}
	return row.BusinessNo, nil
}

// assignTaskNumber freezes the internal owner of a newly inserted task number.
func (s *serviceImpl) assignTaskNumber(
	ctx context.Context,
	database gdb.DB,
	task *entity.AsyncTask,
) error {
	assignedAt := databaseTime(s.now())
	result, err := database.Model(dao.BusinessNumber.Table()).Safe().Ctx(ctx).
		Where(do.BusinessNumber{TenantId: idccap.DefaultTenantID, BusinessNo: task.TaskNo}).
		Data(do.BusinessNumber{
			OwnerType:       businessNumberOwnerTask,
			OwnerInternalId: task.Id,
			AssignedAt:      &assignedAt,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "assign asynchronous task number")
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read asynchronous task number assignment result")
	}
	if affected != 1 {
		return gerror.New("asynchronous task number reservation disappeared before assignment")
	}
	return nil
}

// equalTimePointers compares normalized nullable business timestamps.
func equalTimePointers(left *time.Time, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return databaseTime(*left).Equal(databaseTime(*right))
}
