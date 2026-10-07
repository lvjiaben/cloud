// This file verifies owner routing without allowing rejected addons to reach new-product handling.

package productfulfillment

import (
	"context"
	"errors"
	"testing"
)

type addonCancellationStub struct {
	handled bool
	err     error
}

func (s addonCancellationStub) ConsumeAddonCancellation(_ context.Context, input OrderCancellationInput) (*OrderCancellationResult, bool, error) {
	return &OrderCancellationResult{OrderNo: input.OrderNo, Cancelled: 1}, s.handled, s.err
}

func TestCancellationRouterPreservesOwnerAndFailureBoundaries(t *testing.T) {
	for _, test := range []struct {
		name         string
		handled      bool
		err          error
		genericCalls int
	}{
		{"generic", false, nil, 1}, {"addon", true, nil, 0},
		{"addon-rejected", true, errors.New("owner rejected"), 0},
		{"classification-failed", false, errors.New("classification failed"), 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			generic := &cancellationConsumerStub{}
			consumer, err := NewRoutedCancellationConsumer(generic, addonCancellationStub{handled: test.handled, err: test.err})
			if err != nil {
				t.Fatal(err)
			}
			input := OrderCancellationInput{OrderNo: "ORD_1", CancellationNo: "CAN_1"}
			result, err := consumer.ConsumeOrderCancelled(context.Background(), input)
			if !errors.Is(err, test.err) || len(generic.inputs) != test.genericCalls || result.OrderNo != input.OrderNo {
				t.Fatalf("result=%+v error=%v generic=%+v", result, err, generic.inputs)
			}
			if test.genericCalls == 1 && generic.inputs[0] != input {
				t.Fatal("generic input changed")
			}
		})
	}
	if _, err := NewRoutedCancellationConsumer(nil, addonCancellationStub{}); err == nil {
		t.Fatal("missing generic accepted")
	}
	if _, err := NewRoutedCancellationConsumer(&cancellationConsumerStub{}, nil); err == nil {
		t.Fatal("missing addon accepted")
	}
}
