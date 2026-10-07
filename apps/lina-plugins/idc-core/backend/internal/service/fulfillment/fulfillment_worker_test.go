package fulfillment

import (
	"context"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

type workerDispatcherFake struct {
	claimed   []outbox.ClaimedEvent
	claim     outbox.ClaimInput
	published []outbox.LeaseRef
	failures  []outbox.FailureInput
}

func (f *workerDispatcherFake) Claim(_ context.Context, input outbox.ClaimInput) ([]outbox.ClaimedEvent, error) {
	f.claim = input
	return append([]outbox.ClaimedEvent(nil), f.claimed...), nil
}
func (f *workerDispatcherFake) MarkPublished(_ context.Context, lease outbox.LeaseRef) (*outbox.Event, error) {
	f.published = append(f.published, lease)
	return &outbox.Event{EventNo: lease.EventNo, Status: outbox.StatusPublished}, nil
}
func (f *workerDispatcherFake) RecordFailure(_ context.Context, input outbox.FailureInput) (*outbox.Event, error) {
	f.failures = append(f.failures, input)
	return &outbox.Event{EventNo: input.Lease.EventNo, Status: outbox.StatusDeadLetter}, nil
}

type workerConsumerFake struct{ calls int }

func (f *workerConsumerFake) Consume(_ context.Context, event outbox.ClaimedEvent) (*Delivery, error) {
	f.calls++
	if event.EventNo == "EVT_BAD" {
		return nil, permanent("financial_settlement_event_invalid")
	}
	return &Delivery{EventNo: event.EventNo}, nil
}

func TestWorkerClaimsExactTypeAndConvergesEveryLease(t *testing.T) {
	dispatcher := &workerDispatcherFake{claimed: []outbox.ClaimedEvent{
		{Event: outbox.Event{EventNo: "EVT_OK"}, LeaseToken: "LSE_OK", AttemptNo: 1},
		{Event: outbox.Event{EventNo: "EVT_BAD"}, LeaseToken: "LSE_BAD", AttemptNo: 2},
	}}
	consumer := &workerConsumerFake{}
	worker, err := NewWorker(dispatcher, consumer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := worker.Run(context.Background(), 7)
	if err != nil {
		t.Fatalf("run financial delivery worker: %v", err)
	}
	if dispatcher.claim.Limit != 7 || len(dispatcher.claim.EventTypes) != 1 ||
		dispatcher.claim.EventTypes[0] != FinanciallySettledEventType ||
		result.Claimed != 2 || result.Published != 1 || result.Failed != 1 ||
		consumer.calls != 2 || len(dispatcher.published) != 1 || len(dispatcher.failures) != 1 ||
		dispatcher.failures[0].Retryable || dispatcher.failures[0].ErrorCode != "financial_settlement_event_invalid" {
		t.Fatalf("unexpected worker convergence: result=%+v dispatcher=%+v", result, dispatcher)
	}
	if _, runErr := worker.Run(context.Background(), maximumWorkerBatch+1); runErr == nil {
		t.Fatal("worker accepted an unbounded batch")
	}
}

func TestNewWorkerRejectsMissingDependencies(t *testing.T) {
	if _, err := NewWorker(nil, &workerConsumerFake{}); err == nil {
		t.Fatal("missing dispatcher accepted")
	}
	if _, err := NewWorker(&workerDispatcherFake{}, nil); err == nil {
		t.Fatal("missing consumer accepted")
	}
}
