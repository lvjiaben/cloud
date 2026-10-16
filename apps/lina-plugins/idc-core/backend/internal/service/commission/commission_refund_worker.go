// This file dispatches dedicated commission events and bounded renewal/maturity recovery.
package commission

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"

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
		return nil, gerror.New("commission refund worker requires dependencies")
	}
	return &worker{dispatcher: dispatcher, consumer: consumer}, nil
}

func (w *worker) Run(ctx context.Context, limit int) (int, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumWorkerBatch {
		return 0, gerror.New("commission refund worker input is invalid")
	}
	claimed, err := w.dispatcher.Claim(ctx, outbox.ClaimInput{
		WorkerID: "idc-core-commission-refund-reversal", Limit: limit,
		EventTypes: []string{AccrualEventType, RefundReversalEventType},
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
				ErrorSummary: "Commission processing could not be completed",
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
	renewals, renewalErr := w.consumer.AccrueRenewals(ctx, limit)
	if renewalErr != nil {
		return processed + renewals, renewalErr
	}
	matured, maturityErr := w.consumer.Mature(ctx, limit)
	return processed + renewals + matured, maturityErr
}
