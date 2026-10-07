package composition

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
)

type productFulfillmentCancellationWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *productFulfillmentCancellationWorkerFake) Run(_ context.Context, limit int) (productfulfillment.CancellationWorkerResult, error) {
	f.calls++
	f.limit = limit
	return productfulfillment.CancellationWorkerResult{Claimed: 1, Published: 1}, f.err
}

func TestProductFulfillmentCancellationJobRegistersBoundedPrimaryWorker(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &productFulfillmentCancellationWorkerFake{}
	if err := RegisterProductFulfillmentCancellationJob(context.Background(), registrar, worker); err != nil {
		t.Fatal(err)
	}
	if registrar.addCalls != 1 || registrar.name != "idc-core-product-fulfillment-cancellation" || registrar.handler == nil {
		t.Fatalf("unexpected cancellation job registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil || worker.calls != 1 || worker.limit != productFulfillmentCancellationBatchLimit {
		t.Fatalf("calls=%d limit=%d err=%v", worker.calls, worker.limit, err)
	}

	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterProductFulfillmentCancellationJob(context.Background(), secondary, worker); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("cancellation projection failed")
	failed := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterProductFulfillmentCancellationJob(context.Background(), failed, &productFulfillmentCancellationWorkerFake{err: expected}); err != nil {
		t.Fatal(err)
	}
	if err := failed.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("job swallowed failure: %v", err)
	}
}
