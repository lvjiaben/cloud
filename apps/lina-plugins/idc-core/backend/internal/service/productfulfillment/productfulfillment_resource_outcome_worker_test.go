package productfulfillment

import (
	"context"
	"errors"
	"testing"
)

func TestResourceOutcomeWorkerRequiresDurableDependencies(t *testing.T) {
	t.Parallel()
	if _, err := NewResourceOutcomeWorker(nil, &resourceSourceStub{}); err == nil {
		t.Fatal("NewResourceOutcomeWorker() accepted a missing durable database")
	}
	worker := &resourceOutcomeWorker{}
	if _, err := worker.Run(context.Background(), 1); err == nil {
		t.Fatal("Run() accepted missing durable dependencies")
	}
}

func TestResourceOutcomeDeliveryStateIsBoundedAndRetryable(t *testing.T) {
	t.Parallel()
	if status, code, manual := resourceOutcomeDeliveryState(1, errors.New("temporary")); status != "retry_wait" || code == "" || manual {
		t.Fatalf("first failure state = %q %q %t", status, code, manual)
	}
	if status, code, manual := resourceOutcomeDeliveryState(10, errors.New("still failing")); status != "manual_attention" || code == "" || !manual {
		t.Fatalf("exhausted failure state = %q %q %t", status, code, manual)
	}
	if status, code, manual := resourceOutcomeDeliveryState(2, nil); status != "reported" || code != "" || manual {
		t.Fatalf("success state = %q %q %t", status, code, manual)
	}
}
