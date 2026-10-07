package productfulfillment

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

type cancellationDispatcherStub struct {
	claimed   []outbox.ClaimedEvent
	failed    []outbox.FailureInput
	published []outbox.LeaseRef
}

func (s *cancellationDispatcherStub) Claim(_ context.Context, input outbox.ClaimInput) ([]outbox.ClaimedEvent, error) {
	if input.WorkerID != "idc-core-product-fulfillment-cancellation" || len(input.EventTypes) != 1 || input.EventTypes[0] != OrderCancelledEventType {
		return nil, errors.New("unexpected cancellation claim")
	}
	return append([]outbox.ClaimedEvent(nil), s.claimed...), nil
}
func (s *cancellationDispatcherStub) MarkPublished(_ context.Context, lease outbox.LeaseRef) (*outbox.Event, error) {
	s.published = append(s.published, lease)
	return &outbox.Event{}, nil
}
func (s *cancellationDispatcherStub) RecordFailure(_ context.Context, input outbox.FailureInput) (*outbox.Event, error) {
	s.failed = append(s.failed, input)
	return &outbox.Event{}, nil
}

type cancellationConsumerStub struct {
	inputs []OrderCancellationInput
	err    error
}

func (s *cancellationConsumerStub) ConsumeOrderCancelled(_ context.Context, input OrderCancellationInput) (*OrderCancellationResult, error) {
	s.inputs = append(s.inputs, input)
	return &OrderCancellationResult{OrderNo: input.OrderNo}, s.err
}

func TestCancellationWorkerPublishesOnlyAfterGenericProjection(t *testing.T) {
	payload, err := json.Marshal(cancellationEventPayload{
		CancellationNo: "CAN_0123456789abcdef", OrderNo: "ORD_0123456789abcdef",
		CustomerNo: "CUS_0123456789abcdef", ReasonCode: "customer_request", StatusCode: "completed",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &cancellationDispatcherStub{claimed: []outbox.ClaimedEvent{{
		Event: outbox.Event{EventNo: "EVT_0123456789abcdef", EventType: OrderCancelledEventType,
			EventVersion: 1, AggregateType: "commerce_order", AggregateNo: "ORD_0123456789abcdef", Payload: payload},
		LeaseToken: "lease", AttemptNo: 1,
	}}}
	consumer := &cancellationConsumerStub{}
	worker, err := NewCancellationWorker(dispatcher, consumer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := worker.Run(context.Background(), 20)
	if err != nil || result.Published != 1 || result.Failed != 0 || len(consumer.inputs) != 1 ||
		consumer.inputs[0].CancellationNo != "CAN_0123456789abcdef" || len(dispatcher.published) != 1 {
		t.Fatalf("result=%+v inputs=%+v published=%+v err=%v", result, consumer.inputs, dispatcher.published, err)
	}
}

func TestCancellationWorkerDeadLettersInvalidEventWithoutCallingDomain(t *testing.T) {
	dispatcher := &cancellationDispatcherStub{claimed: []outbox.ClaimedEvent{{
		Event: outbox.Event{EventNo: "EVT_bad", EventType: OrderCancelledEventType,
			EventVersion: 1, AggregateType: "commerce_order", AggregateNo: "ORD_0123456789abcdef",
			Payload: json.RawMessage(`{"cancellationNo":"CAN_0123456789abcdef","orderNo":"ORD_other","customerNo":"CUS_1","reasonCode":"customer_request","statusCode":"completed"}`)},
		LeaseToken: "lease", AttemptNo: 1,
	}}}
	consumer := &cancellationConsumerStub{}
	worker, err := NewCancellationWorker(dispatcher, consumer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := worker.Run(context.Background(), 20)
	if err != nil || result.Failed != 1 || len(consumer.inputs) != 0 || len(dispatcher.failed) != 1 || dispatcher.failed[0].Retryable {
		t.Fatalf("result=%+v inputs=%+v failed=%+v err=%v", result, consumer.inputs, dispatcher.failed, err)
	}
}
