package commission

import (
	"context"
	"errors"

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

type Dispatcher interface {
	Claim(context.Context, outbox.ClaimInput) ([]outbox.ClaimedEvent, error)
	MarkPublished(context.Context, outbox.LeaseRef) (*outbox.Event, error)
	RecordFailure(context.Context, outbox.FailureInput) (*outbox.Event, error)
}

type Worker interface {
	Run(context.Context, int) (int, error)
}

type worker struct {
	dispatcher Dispatcher
	consumer   Consumer
}

func NewWorker(dispatcher Dispatcher, consumer Consumer) (Worker, error) {
	if dispatcher == nil || consumer == nil {
		return nil, errors.New("commission refund worker requires dependencies")
	}
	return &worker{dispatcher: dispatcher, consumer: consumer}, nil
}

func (w *worker) Run(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumWorkerBatch {
		return 0, errors.New("commission refund worker input is invalid")
	}
	claimed, err := w.dispatcher.Claim(ctx, outbox.ClaimInput{
		WorkerID: "idc-core-commission-refund-reversal", Limit: limit,
		EventTypes: []string{RefundReversalEventType},
	})
	if err != nil {
		return 0, err
	}
	processed := 0
	for index := range claimed {
		event := claimed[index]
		lease := outbox.LeaseRef{EventNo: event.EventNo,
			LeaseToken: event.LeaseToken, AttemptNo: event.AttemptNo}
		if consumeErr := w.consumer.Consume(ctx, event); consumeErr != nil {
			code, canRetry := failureClass(consumeErr)
			if _, transitionErr := w.dispatcher.RecordFailure(ctx, outbox.FailureInput{
				Lease: lease, Retryable: canRetry, ErrorCode: code,
				ErrorSummary: "Commission refund reversal could not be completed",
			}); transitionErr != nil {
				return processed, transitionErr
			}
			continue
		}
		if _, transitionErr := w.dispatcher.MarkPublished(ctx, lease); transitionErr != nil {
			return processed, transitionErr
		}
		processed++
	}
	return processed, nil
}
