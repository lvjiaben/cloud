package productfulfillment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"lina-plugin-idc-core/backend/internal/service/outbox"
)

const (
	OrderCancelledEventType        = "commerce.order.cancelled"
	orderCancelledEventVersion     = 1
	maximumCancellationWorkerBatch = 50
)

type CancellationDispatcher interface {
	Claim(context.Context, outbox.ClaimInput) ([]outbox.ClaimedEvent, error)
	MarkPublished(context.Context, outbox.LeaseRef) (*outbox.Event, error)
	RecordFailure(context.Context, outbox.FailureInput) (*outbox.Event, error)
}

type CancellationWorkerResult struct {
	Claimed, Published, Failed int
}

type CancellationWorker interface {
	Run(context.Context, int) (CancellationWorkerResult, error)
}

type cancellationWorker struct {
	dispatcher CancellationDispatcher
	consumer   OrderCancellationConsumer
}

type cancellationEventPayload struct {
	CancellationNo string `json:"cancellationNo"`
	OrderNo        string `json:"orderNo"`
	CustomerNo     string `json:"customerNo"`
	ReasonCode     string `json:"reasonCode"`
	StatusCode     string `json:"statusCode"`
}

func NewCancellationWorker(dispatcher CancellationDispatcher, consumer OrderCancellationConsumer) (CancellationWorker, error) {
	if dispatcher == nil || consumer == nil {
		return nil, errors.New("product fulfillment cancellation worker dependencies are unavailable")
	}
	return &cancellationWorker{dispatcher: dispatcher, consumer: consumer}, nil
}

func (w *cancellationWorker) Run(ctx context.Context, limit int) (CancellationWorkerResult, error) {
	if ctx == nil || ctx.Err() != nil || limit < 1 || limit > maximumCancellationWorkerBatch {
		return CancellationWorkerResult{}, errors.New("product fulfillment cancellation worker input is invalid")
	}
	claimed, err := w.dispatcher.Claim(ctx, outbox.ClaimInput{
		WorkerID: "idc-core-product-fulfillment-cancellation", Limit: limit,
		EventTypes: []string{OrderCancelledEventType},
	})
	if err != nil {
		return CancellationWorkerResult{}, err
	}
	result := CancellationWorkerResult{Claimed: len(claimed)}
	for index := range claimed {
		event := claimed[index]
		lease := outbox.LeaseRef{EventNo: event.EventNo, LeaseToken: event.LeaseToken, AttemptNo: event.AttemptNo}
		payload, decodeErr := decodeCancellationEvent(event)
		if decodeErr == nil {
			_, decodeErr = w.consumer.ConsumeOrderCancelled(ctx, OrderCancellationInput{
				OrderNo: payload.OrderNo, CancellationNo: payload.CancellationNo,
			})
		}
		if decodeErr != nil {
			if _, transitionErr := w.dispatcher.RecordFailure(ctx, outbox.FailureInput{
				Lease: lease, Retryable: !errors.Is(decodeErr, errInvalidCancellationEvent),
				ErrorCode:    "product_fulfillment_cancellation_unavailable",
				ErrorSummary: "Order cancellation could not be applied to fulfillment",
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

var errInvalidCancellationEvent = errors.New("product fulfillment cancellation event is invalid")

func decodeCancellationEvent(event outbox.ClaimedEvent) (*cancellationEventPayload, error) {
	if event.EventType != OrderCancelledEventType || event.EventVersion != orderCancelledEventVersion ||
		event.AggregateType != "commerce_order" || strings.TrimSpace(event.AggregateNo) == "" {
		return nil, errInvalidCancellationEvent
	}
	decoder := json.NewDecoder(bytes.NewReader(event.Payload))
	decoder.DisallowUnknownFields()
	var payload cancellationEventPayload
	if err := decoder.Decode(&payload); err != nil {
		return nil, errInvalidCancellationEvent
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF ||
		payload.OrderNo != event.AggregateNo || payload.StatusCode != "completed" ||
		!managementReferencePattern.MatchString(payload.OrderNo) ||
		!managementReferencePattern.MatchString(payload.CancellationNo) ||
		strings.TrimSpace(payload.CustomerNo) == "" || strings.TrimSpace(payload.ReasonCode) == "" {
		return nil, errInvalidCancellationEvent
	}
	return &payload, nil
}
