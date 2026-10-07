package notification

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/notificationcap"
)

const (
	notificationEventWorkerID        = "idc-notify-event-materializer"
	maximumEventMaterializationBatch = 40
)

// ProcessEvents consumes one bounded source-event batch. Every configured
// channel gets durable materialization evidence before the source event is
// acknowledged, including explicit fail-closed outcomes.
func (s *serviceImpl) ProcessEvents(ctx context.Context, limit int) error {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumEventMaterializationBatch {
		return bizerr.NewCode(CodeInvalid)
	}
	claimed, err := s.sources.Events.Claim(ctx, notificationcap.EventClaimInput{
		WorkerID: notificationEventWorkerID,
		Limit:    limit,
	})
	if err != nil {
		return publicError(err)
	}
	if len(claimed) == 0 {
		return nil
	}
	completed, batchErr := s.materializeEventBatch(ctx, claimed)
	if batchErr != nil {
		code, summary, retryable := notificationEventFailure(batchErr)
		for index := range claimed {
			if _, releaseErr := s.sources.Events.Release(ctx, notificationcap.EventReleaseInput{
				Lease: claimed[index].Lease, Retryable: retryable, ErrorCode: code, ErrorSummary: summary,
			}); releaseErr != nil {
				return publicError(releaseErr)
			}
		}
		return nil
	}
	for index := range claimed {
		if !completed[index] {
			if _, releaseErr := s.sources.Events.Release(ctx, notificationcap.EventReleaseInput{
				Lease: claimed[index].Lease, Retryable: true, ErrorCode: "notification_event_unavailable",
				ErrorSummary: "Notification event could not be materialized",
			}); releaseErr != nil {
				return publicError(releaseErr)
			}
			continue
		}
		if _, ackErr := s.sources.Events.Ack(ctx, claimed[index].Lease); ackErr != nil {
			return publicError(ackErr)
		}
	}
	return nil
}

func notificationEventFailure(err error) (code string, summary string, retryable bool) {
	summary = "Notification event could not be materialized"
	switch {
	case bizerr.Is(err, CodeInvalid):
		return "notification_event_invalid", summary, false
	case bizerr.Is(err, CodeConflict):
		return "notification_event_conflict", summary, false
	default:
		return "notification_event_unavailable", summary, true
	}
}
