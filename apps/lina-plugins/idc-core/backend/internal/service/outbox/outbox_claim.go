// This file implements bounded skip-locked claiming, lease renewal, and expiry convergence.

package outbox

import (
	"context"
	"database/sql"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// Claim leases a ready batch and increments every selected event attempt atomically.
func (s *serviceImpl) Claim(ctx context.Context, input ClaimInput) ([]ClaimedEvent, error) {
	normalized, err := s.normalizeClaimInput(input)
	if err != nil {
		return nil, err
	}
	claimed := make([]ClaimedEvent, 0)
	err = s.transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		if err := s.convergeExpiredExhausted(ctx, now, normalized.Limit, normalized.EventTypes); err != nil {
			return err
		}
		if err := s.convergeUnclaimedExhausted(ctx, normalized.Limit, normalized.EventTypes); err != nil {
			return err
		}
		rows, err := s.loadClaimableEvents(ctx, now, normalized.Limit, normalized.EventTypes)
		if err != nil || len(rows) == 0 {
			return err
		}
		leaseToken, err := newLeaseToken(s.random, normalized.WorkerID)
		if err != nil {
			return err
		}
		leaseExpiresAt := databaseTime(now.Add(s.config.LeaseDuration))
		ids := make([]int64, len(rows))
		for index := range rows {
			ids[index] = rows[index].Id
		}
		columns := dao.OutboxEvent.Columns()
		updateResult, err := s.eventModel(ctx).
			Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID}).
			WhereIn(columns.Id, ids).
			Data(do.OutboxEvent{
				StatusCode:     string(StatusProcessing),
				LockedBy:       leaseToken,
				LeaseExpiresAt: &leaseExpiresAt,
				AttemptCount:   gdb.Raw(columns.AttemptCount + " + 1"),
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "lease outbox event batch")
		}
		if err = requireAffected(updateResult, len(rows), "lease outbox event batch"); err != nil {
			return err
		}
		claimed = make([]ClaimedEvent, 0, len(rows))
		for index := range rows {
			rows[index].StatusCode = string(StatusProcessing)
			rows[index].LockedBy = leaseToken
			rows[index].LeaseExpiresAt = &leaseExpiresAt
			rows[index].AttemptCount++
			projected, err := projectClaimedEvent(&rows[index])
			if err != nil {
				return err
			}
			claimed = append(claimed, *projected)
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

// Renew extends one still-live, completely fenced delivery lease.
func (s *serviceImpl) Renew(ctx context.Context, lease LeaseRef) (*ClaimedEvent, error) {
	normalized, err := normalizeLeaseRef(lease)
	if err != nil {
		return nil, err
	}
	var claimed *ClaimedEvent
	err = s.transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		leaseExpiresAt := databaseTime(now.Add(s.config.LeaseDuration))
		result, err := s.liveLeaseModel(ctx, normalized, now).
			Data(do.OutboxEvent{LeaseExpiresAt: &leaseExpiresAt}).Update()
		if err != nil {
			return gerror.Wrap(err, "renew outbox event lease")
		}
		if err = requireLiveLease(result); err != nil {
			return err
		}
		row, err := s.loadEventByNumber(ctx, normalized.EventNo)
		if err != nil {
			return err
		}
		if row == nil {
			return bizerr.NewCode(CodeLeaseLost)
		}
		claimed, err = projectClaimedEvent(row)
		return err
	})
	if err != nil {
		if _, businessError := bizerr.As(err); businessError {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return claimed, nil
}

// loadClaimableEvents selects ready or expired-reclaimable rows in deterministic order.
func (s *serviceImpl) loadClaimableEvents(ctx context.Context, now time.Time, limit int, eventTypes []string) ([]entity.OutboxEvent, error) {
	columns := dao.OutboxEvent.Columns()
	model := s.eventModel(ctx)
	ready := model.Builder().
		WhereIn(columns.StatusCode, []Status{StatusPending, StatusRetryWait}).
		WhereLTE(columns.AvailableAt, now)
	expired := model.Builder().
		Where(do.OutboxEvent{StatusCode: string(StatusProcessing)}).
		WhereNotNull(columns.LeaseExpiresAt).
		WhereLTE(columns.LeaseExpiresAt, now)
	claimable := model.Builder().Where(ready).WhereOr(expired)
	var rows []entity.OutboxEvent
	claimModel := model.
		Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID}).
		WhereLT(columns.AttemptCount, gdb.Raw(columns.MaxAttempts)).
		Where(claimable)
	claimModel = filterEventTypes(claimModel, eventTypes)
	err := claimModel.
		OrderAsc(columns.AvailableAt).
		OrderAsc(columns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return nil, gerror.Wrap(err, "claim ready outbox events")
	}
	return rows, nil
}

// convergeExpiredExhausted freezes a bounded set of ambiguous exhausted claims.
func (s *serviceImpl) convergeExpiredExhausted(ctx context.Context, now time.Time, limit int, eventTypes []string) error {
	columns := dao.OutboxEvent.Columns()
	var rows []entity.OutboxEvent
	model := s.eventModel(ctx).
		Fields(columns.Id).
		Where(do.OutboxEvent{
			TenantId:   idccap.DefaultTenantID,
			StatusCode: string(StatusProcessing),
		}).
		WhereNotNull(columns.LeaseExpiresAt).
		WhereLTE(columns.LeaseExpiresAt, now).
		WhereGTE(columns.AttemptCount, gdb.Raw(columns.MaxAttempts))
	model = filterEventTypes(model, eventTypes)
	err := model.
		OrderAsc(columns.LeaseExpiresAt).
		OrderAsc(columns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return gerror.Wrap(err, "load expired exhausted outbox events")
	}
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, len(rows))
	for index := range rows {
		ids[index] = rows[index].Id
	}
	result, err := s.eventModel(ctx).
		Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.Id, ids).
		Data(do.OutboxEvent{
			StatusCode:       string(StatusResultUnknown),
			LockedBy:         "",
			LastErrorCode:    expiredUnknownErrorCode,
			LastErrorSummary: expiredUnknownErrorSummary,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "freeze expired exhausted outbox events")
	}
	if err = requireAffected(result, len(rows), "freeze expired exhausted outbox events"); err != nil {
		return err
	}
	return s.clearLeaseByIDs(ctx, ids)
}

// convergeUnclaimedExhausted dead-letters bounded anomalous ready-state rows
// whose attempt counters already reached their configured maximum.
func (s *serviceImpl) convergeUnclaimedExhausted(ctx context.Context, limit int, eventTypes []string) error {
	columns := dao.OutboxEvent.Columns()
	var rows []entity.OutboxEvent
	model := s.eventModel(ctx).
		Fields(columns.Id).
		Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.StatusCode, []Status{StatusPending, StatusRetryWait}).
		WhereGTE(columns.AttemptCount, gdb.Raw(columns.MaxAttempts))
	model = filterEventTypes(model, eventTypes)
	err := model.
		OrderAsc(columns.AvailableAt).
		OrderAsc(columns.Id).
		Limit(limit).
		LockUpdateSkipLocked().
		Scan(&rows)
	if err != nil {
		return gerror.Wrap(err, "load unclaimed exhausted outbox events")
	}
	if len(rows) == 0 {
		return nil
	}
	ids := make([]int64, len(rows))
	for index := range rows {
		ids[index] = rows[index].Id
	}
	result, err := s.eventModel(ctx).
		Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.Id, ids).
		Data(do.OutboxEvent{
			StatusCode:       string(StatusDeadLetter),
			LockedBy:         "",
			LastErrorCode:    attemptsExhaustedErrorCode,
			LastErrorSummary: attemptsExhaustedSummary,
		}).Update()
	if err != nil {
		return gerror.Wrap(err, "dead-letter unclaimed exhausted outbox events")
	}
	if err = requireAffected(result, len(rows), "dead-letter unclaimed exhausted outbox events"); err != nil {
		return err
	}
	return s.clearLeaseByIDs(ctx, ids)
}

// filterEventTypes applies an exact, already-normalized event-type allowlist.
// An empty set intentionally preserves the generic all-event dispatcher contract.
func filterEventTypes(model *gdb.Model, eventTypes []string) *gdb.Model {
	if len(eventTypes) == 0 {
		return model
	}
	return model.WhereIn(dao.OutboxEvent.Columns().EventType, eventTypes)
}

// liveLeaseModel applies the full tenant, event, state, token, attempt, and expiry fence.
func (s *serviceImpl) liveLeaseModel(ctx context.Context, lease LeaseRef, now time.Time) *gdb.Model {
	columns := dao.OutboxEvent.Columns()
	return s.eventModel(ctx).
		Where(do.OutboxEvent{
			TenantId:     idccap.DefaultTenantID,
			EventNo:      lease.EventNo,
			StatusCode:   string(StatusProcessing),
			LockedBy:     lease.LeaseToken,
			AttemptCount: lease.AttemptNo,
		}).
		WhereNotNull(columns.LeaseExpiresAt).
		WhereGT(columns.LeaseExpiresAt, now)
}

// clearLeaseByIDs writes a real SQL NULL because DO nil pointers are intentionally omitted.
func (s *serviceImpl) clearLeaseByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	columns := dao.OutboxEvent.Columns()
	// This narrow column/value form is required to clear a nullable timestamp;
	// Data(do.OutboxEvent{LeaseExpiresAt:nil}) would omit the field entirely.
	result, err := s.eventModel(ctx).
		Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID}).
		WhereIn(columns.Id, ids).
		Data(columns.LeaseExpiresAt, nil).
		Update()
	if err != nil {
		return gerror.Wrap(err, "clear outbox event leases")
	}
	return requireAffected(result, len(ids), "clear outbox event leases")
}

// clearLeaseByEventNumber clears one already-fenced event lease in the same transaction.
func (s *serviceImpl) clearLeaseByEventNumber(ctx context.Context, eventNo string) error {
	columns := dao.OutboxEvent.Columns()
	// This narrow column/value form is required to persist SQL NULL instead of
	// having GoFrame omit a nil DO pointer during an update.
	result, err := s.eventModel(ctx).
		Where(do.OutboxEvent{TenantId: idccap.DefaultTenantID, EventNo: eventNo}).
		Data(columns.LeaseExpiresAt, nil).
		Update()
	if err != nil {
		return gerror.Wrap(err, "clear outbox event lease")
	}
	return requireAffected(result, 1, "clear outbox event lease")
}

// requireLiveLease converts a zero-row fenced update into a non-enumerating lease error.
func requireLiveLease(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read fenced outbox transition result")
	}
	if affected != 1 {
		return bizerr.NewCode(CodeLeaseLost)
	}
	return nil
}

// requireAffected verifies a bounded set mutation did not silently lose locked rows.
func requireAffected(result sql.Result, expected int, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return gerror.Wrap(err, "read "+operation+" result")
	}
	if affected != int64(expected) {
		return gerror.Newf("%s affected %d rows, expected %d", operation, affected, expected)
	}
	return nil
}
