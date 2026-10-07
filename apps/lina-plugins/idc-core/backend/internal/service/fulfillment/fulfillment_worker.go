package fulfillment

import (
	"context"
	"errors"

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const maximumWorkerBatch = 50

// Dispatcher is the exact outbox lifecycle needed by the local consumer job.
type Dispatcher interface {
	Claim(context.Context, outbox.ClaimInput) ([]outbox.ClaimedEvent, error)
	MarkPublished(context.Context, outbox.LeaseRef) (*outbox.Event, error)
	RecordFailure(context.Context, outbox.FailureInput) (*outbox.Event, error)
}

// WorkerResult reports bounded polling outcomes without duplicating event state.
type WorkerResult struct {
	Claimed   int
	Published int
	Failed    int
}

// Worker claims only financial settlement events and converges each lease.
type Worker interface {
	Run(ctx context.Context, limit int) (WorkerResult, error)
}

type workerImpl struct {
	dispatcher Dispatcher
	consumer   Consumer
}

var _ Worker = (*workerImpl)(nil)

// NewWorker composes the typed dispatcher and local materializer.
func NewWorker(dispatcher Dispatcher, consumer Consumer) (Worker, error) {
	if dispatcher == nil || consumer == nil {
		return nil, errors.New("financial settlement delivery worker requires dispatcher and consumer")
	}
	return &workerImpl{dispatcher: dispatcher, consumer: consumer}, nil
}

func (w *workerImpl) Run(ctx context.Context, limit int) (WorkerResult, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumWorkerBatch {
		return WorkerResult{}, errors.New("financial settlement delivery worker input is invalid")
	}
	claimed, err := w.dispatcher.Claim(ctx, outbox.ClaimInput{
		WorkerID: "idc-core-financial-settlement-delivery",
		Limit:    limit, EventTypes: []string{FinanciallySettledEventType},
	})
	if err != nil {
		return WorkerResult{}, err
	}
	result := WorkerResult{Claimed: len(claimed)}
	for index := range claimed {
		event := &claimed[index]
		lease := outbox.LeaseRef{
			EventNo: event.EventNo, LeaseToken: event.LeaseToken, AttemptNo: event.AttemptNo,
		}
		if _, consumeErr := w.consumer.Consume(ctx, *event); consumeErr != nil {
			code, summary, retryable := FailureClass(consumeErr)
			if _, transitionErr := w.dispatcher.RecordFailure(ctx, outbox.FailureInput{
				Lease: lease, Retryable: retryable, ErrorCode: code, ErrorSummary: summary,
			}); transitionErr != nil {
				return result, transitionErr
			}
			result.Failed++
			continue
		}
		if _, transitionErr := w.dispatcher.MarkPublished(ctx, lease); transitionErr != nil {
			return result, transitionErr
		}
		result.Published++
	}
	return result, nil
}
