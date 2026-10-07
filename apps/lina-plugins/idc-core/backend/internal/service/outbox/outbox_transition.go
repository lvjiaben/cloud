// This file implements fully fenced publication, failure, and unknown-result transitions.

package outbox

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// MarkPublished records a confirmed publication from one still-live claim.
func (s *serviceImpl) MarkPublished(ctx context.Context, lease LeaseRef) (*Event, error) {
	normalized, err := normalizeLeaseRef(lease)
	if err != nil {
		return nil, err
	}
	var event *Event
	err = s.transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		result, err := s.liveLeaseModel(ctx, normalized, now).
			Data(do.OutboxEvent{
				StatusCode:  string(StatusPublished),
				LockedBy:    "",
				PublishedAt: &now,
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "publish outbox event")
		}
		if err = requireLiveLease(result); err != nil {
			return err
		}
		if err = s.clearLeaseByEventNumber(ctx, normalized.EventNo); err != nil {
			return err
		}
		event, err = s.loadProjectedEvent(ctx, normalized.EventNo)
		return err
	})
	return transitionResult(event, err)
}

// RecordFailure stores redacted diagnostics and schedules retry or dead letter.
func (s *serviceImpl) RecordFailure(ctx context.Context, input FailureInput) (*Event, error) {
	normalized, err := normalizeFailureInput(input)
	if err != nil {
		return nil, err
	}
	var event *Event
	err = s.transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		row, err := s.loadLiveLease(ctx, normalized.Lease, now)
		if err != nil {
			return err
		}
		status := StatusDeadLetter
		update := do.OutboxEvent{
			StatusCode:       string(status),
			LockedBy:         "",
			LastErrorCode:    normalized.ErrorCode,
			LastErrorSummary: normalized.ErrorSummary,
		}
		if normalized.Retryable && normalized.Lease.AttemptNo < row.MaxAttempts {
			status = StatusRetryWait
			availableAt := databaseTime(now.Add(s.retryBackoff(row.EventNo, normalized.Lease.AttemptNo)))
			update.StatusCode = string(status)
			update.AvailableAt = &availableAt
		}
		result, err := s.liveLeaseModel(ctx, normalized.Lease, now).Data(update).Update()
		if err != nil {
			return gerror.Wrap(err, "record outbox event failure")
		}
		if err = requireLiveLease(result); err != nil {
			return err
		}
		if err = s.clearLeaseByEventNumber(ctx, normalized.Lease.EventNo); err != nil {
			return err
		}
		event, err = s.loadProjectedEvent(ctx, normalized.Lease.EventNo)
		return err
	})
	return transitionResult(event, err)
}

// MarkResultUnknown freezes an ambiguous result without scheduling another delivery.
func (s *serviceImpl) MarkResultUnknown(ctx context.Context, input UnknownInput) (*Event, error) {
	normalized, err := normalizeUnknownInput(input)
	if err != nil {
		return nil, err
	}
	var event *Event
	err = s.transaction(ctx, func(ctx context.Context, _ gdb.TX) error {
		now := databaseTime(s.now())
		result, err := s.liveLeaseModel(ctx, normalized.Lease, now).
			Data(do.OutboxEvent{
				StatusCode:       string(StatusResultUnknown),
				LockedBy:         "",
				LastErrorCode:    normalized.ErrorCode,
				LastErrorSummary: normalized.ErrorSummary,
			}).Update()
		if err != nil {
			return gerror.Wrap(err, "mark outbox event result unknown")
		}
		if err = requireLiveLease(result); err != nil {
			return err
		}
		if err = s.clearLeaseByEventNumber(ctx, normalized.Lease.EventNo); err != nil {
			return err
		}
		event, err = s.loadProjectedEvent(ctx, normalized.Lease.EventNo)
		return err
	})
	return transitionResult(event, err)
}

// loadLiveLease locks and returns the row protected by the complete live fence.
func (s *serviceImpl) loadLiveLease(ctx context.Context, lease LeaseRef, now time.Time) (*entity.OutboxEvent, error) {
	var row *entity.OutboxEvent
	if err := s.liveLeaseModel(ctx, lease, now).LockUpdate().Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load live outbox event lease")
	}
	if row == nil {
		return nil, bizerr.NewCode(CodeLeaseLost)
	}
	return row, nil
}

// loadProjectedEvent reloads one transitioned event within its short transaction.
func (s *serviceImpl) loadProjectedEvent(ctx context.Context, eventNo string) (*Event, error) {
	row, err := s.loadEventByNumber(ctx, eventNo)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, gerror.New("transitioned outbox event disappeared")
	}
	return projectEvent(row)
}

// transitionResult preserves business errors and hides lower-level persistence details.
func transitionResult(event *Event, err error) (*Event, error) {
	if err == nil {
		return event, nil
	}
	if _, businessError := bizerr.As(err); businessError {
		return nil, err
	}
	return nil, bizerr.WrapCode(err, CodeUnavailable)
}
