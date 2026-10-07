// This file implements transaction-required, idempotent outbox event creation and projection.

package outbox

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/dao"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// Enqueue persists an event only inside the caller's authoritative transaction.
func (s *serviceImpl) Enqueue(ctx context.Context, input EnqueueInput) (*Event, error) {
	normalized, err := s.normalizeEnqueueInput(input)
	if err != nil {
		return nil, err
	}
	group := dao.OutboxEvent.Group()
	if s.database != nil {
		group = s.database.GetGroup()
	}
	if gdb.TXFromCtx(ctx, group) == nil {
		return nil, bizerr.NewCode(CodeTransactionRequired)
	}
	event, err := s.enqueueInTransaction(ctx, normalized)
	if err != nil {
		if _, businessError := bizerr.As(err); businessError {
			return nil, err
		}
		return nil, bizerr.WrapCode(err, CodeUnavailable)
	}
	return event, nil
}

// enqueueInTransaction converges concurrent requests through the persisted unique scope.
func (s *serviceImpl) enqueueInTransaction(
	ctx context.Context,
	input normalizedEnqueueInput,
) (*Event, error) {
	existing, err := s.loadEventByDedupe(ctx, input, true)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return reuseEvent(existing, input)
	}
	for range maximumTokenAttempts {
		eventNo, err := newOpaqueToken(s.random, eventNumberPrefix)
		if err != nil {
			return nil, err
		}
		now := databaseTime(s.now())
		insertResult, err := s.eventModel(ctx).Data(do.OutboxEvent{
			TenantId:            idccap.DefaultTenantID,
			EventNo:             eventNo,
			EventType:           input.EventType,
			EventVersion:        input.EventVersion,
			Topic:               input.Topic,
			AggregateType:       input.AggregateType,
			AggregateNo:         input.AggregateNo,
			CorrelationNo:       input.CorrelationNo,
			DeduplicationDigest: input.deduplicationDigest,
			StatusCode:          string(StatusPending),
			Payload:             string(input.payload),
			Metadata:            string(input.metadata),
			OccurredAt:          &now,
			AvailableAt:         &input.AvailableAt,
			LockedBy:            "",
			AttemptCount:        0,
			MaxAttempts:         input.MaxAttempts,
			LastErrorCode:       "",
			LastErrorSummary:    "",
		}).InsertIgnore()
		if err != nil {
			return nil, gerror.Wrap(err, "insert outbox event")
		}
		affected, err := insertResult.RowsAffected()
		if err != nil {
			return nil, gerror.Wrap(err, "read outbox insert result")
		}
		created, err := s.loadEventByDedupe(ctx, input, true)
		if err != nil {
			return nil, err
		}
		if created != nil {
			if affected == 1 && created.EventNo != eventNo {
				return nil, gerror.New("outbox event number diverged from inserted request")
			}
			return reuseEvent(created, input)
		}
		if affected != 0 {
			return nil, gerror.New("outbox insert succeeded without a readable event")
		}
		// An unrelated event-number collision can be retried safely because the
		// PostgreSQL INSERT IGNORE path leaves the caller transaction usable.
	}
	return nil, gerror.New("outbox event number collision retry limit reached")
}

// loadEventByDedupe reads the row protected by the event deduplication unique key.
func (s *serviceImpl) loadEventByDedupe(
	ctx context.Context,
	input normalizedEnqueueInput,
	lock bool,
) (*entity.OutboxEvent, error) {
	var event *entity.OutboxEvent
	model := s.eventModel(ctx).Where(do.OutboxEvent{
		TenantId:            idccap.DefaultTenantID,
		EventType:           input.EventType,
		DeduplicationDigest: input.deduplicationDigest,
	})
	if lock {
		model = model.LockUpdate()
	}
	if err := model.Scan(&event); err != nil {
		return nil, gerror.Wrap(err, "load outbox event by deduplication scope")
	}
	return event, nil
}

func (s *serviceImpl) eventModel(ctx context.Context) *gdb.Model {
	if s.database != nil {
		return s.database.Model(dao.OutboxEvent.Table()).Ctx(ctx)
	}
	return dao.OutboxEvent.Ctx(ctx)
}

// reuseEvent returns the existing event only when its immutable envelope is identical.
func reuseEvent(row *entity.OutboxEvent, input normalizedEnqueueInput) (*Event, error) {
	payload, err := canonicalPersistedEvidence(row.Payload)
	if err != nil {
		return nil, err
	}
	metadata, err := canonicalPersistedEvidence(row.Metadata)
	if err != nil {
		return nil, err
	}
	if row.EventType != input.EventType ||
		row.EventVersion != input.EventVersion ||
		row.Topic != input.Topic ||
		row.AggregateType != input.AggregateType ||
		row.AggregateNo != input.AggregateNo ||
		row.CorrelationNo != input.CorrelationNo ||
		row.DeduplicationDigest != input.deduplicationDigest ||
		row.AvailableAt == nil ||
		(input.availableAtExplicit && !databaseTime(*row.AvailableAt).Equal(input.AvailableAt)) ||
		(input.maxAttemptsExplicit && row.MaxAttempts != input.MaxAttempts) ||
		!bytes.Equal(payload, input.payload) ||
		!bytes.Equal(metadata, input.metadata) {
		return nil, bizerr.NewCode(CodeIdempotencyConflict)
	}
	return projectEvent(row)
}

// loadEventByNumber reads one tenant-scoped event without exposing its internal ID.
func (s *serviceImpl) loadEventByNumber(ctx context.Context, eventNo string) (*entity.OutboxEvent, error) {
	var row *entity.OutboxEvent
	if err := s.eventModel(ctx).Where(do.OutboxEvent{
		TenantId: idccap.DefaultTenantID,
		EventNo:  eventNo,
	}).Scan(&row); err != nil {
		return nil, gerror.Wrap(err, "load outbox event")
	}
	return row, nil
}

// canonicalPersistedEvidence normalizes trusted JSONB text for stable comparisons.
func canonicalPersistedEvidence(value string) (json.RawMessage, error) {
	canonical, err := normalizeEvidence(json.RawMessage(value), len(value)+1)
	if err != nil {
		return nil, gerror.Wrap(err, "read persisted outbox evidence")
	}
	return canonical, nil
}

// projectEvent maps generated persistence state into the plugin-internal contract.
func projectEvent(row *entity.OutboxEvent) (*Event, error) {
	if row == nil || row.OccurredAt == nil || row.AvailableAt == nil {
		return nil, gerror.New("outbox event is missing required persisted fields")
	}
	payload, err := canonicalPersistedEvidence(row.Payload)
	if err != nil {
		return nil, err
	}
	metadata, err := canonicalPersistedEvidence(row.Metadata)
	if err != nil {
		return nil, err
	}
	return &Event{
		EventNo:          row.EventNo,
		EventType:        row.EventType,
		EventVersion:     row.EventVersion,
		Topic:            row.Topic,
		AggregateType:    row.AggregateType,
		AggregateNo:      row.AggregateNo,
		CorrelationNo:    row.CorrelationNo,
		Status:           Status(row.StatusCode),
		Payload:          append(json.RawMessage(nil), payload...),
		Metadata:         append(json.RawMessage(nil), metadata...),
		OccurredAt:       databaseTime(*row.OccurredAt),
		AvailableAt:      databaseTime(*row.AvailableAt),
		AttemptCount:     row.AttemptCount,
		MaxAttempts:      row.MaxAttempts,
		PublishedAt:      copyDatabaseTime(row.PublishedAt),
		LastErrorCode:    row.LastErrorCode,
		LastErrorSummary: row.LastErrorSummary,
	}, nil
}

// projectClaimedEvent maps one processing row into its complete fencing contract.
func projectClaimedEvent(row *entity.OutboxEvent) (*ClaimedEvent, error) {
	event, err := projectEvent(row)
	if err != nil {
		return nil, err
	}
	if row.LeaseExpiresAt == nil || row.LockedBy == "" || row.AttemptCount < 1 {
		return nil, gerror.New("claimed outbox event is missing lease evidence")
	}
	return &ClaimedEvent{
		Event:          *event,
		LeaseToken:     row.LockedBy,
		AttemptNo:      row.AttemptCount,
		LeaseExpiresAt: databaseTime(*row.LeaseExpiresAt),
	}, nil
}

// copyDatabaseTime copies and normalizes a nullable PostgreSQL timestamp.
func copyDatabaseTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := databaseTime(*value)
	return &copyValue
}
