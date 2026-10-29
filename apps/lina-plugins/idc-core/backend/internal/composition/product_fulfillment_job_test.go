// This file verifies primary-node job registration, bounded execution and non-overlapping fast ticks.

package composition

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

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

// A slow provider cannot create one overlapping batch per clock tick. Skipped
// ticks still permit the next batch once the active callback has finished.
type blockingFulfillmentWorker struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (worker *blockingFulfillmentWorker) Run(ctx context.Context, _ int) (*productfulfillment.WorkerBatch, error) {
	worker.calls.Add(1)
	worker.started <- struct{}{}
	select {
	case <-worker.release:
		return &productfulfillment.WorkerBatch{}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func TestProductFulfillmentFastClockDoesNotOverlapBatches(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	worker := &blockingFulfillmentWorker{started: make(chan struct{}, 2), release: make(chan struct{})}
	registrar := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterProductFulfillmentJob(ctx, registrar, worker); err != nil {
		t.Fatal(err)
	}
	if registrar.pattern != "@every 1s" {
		t.Fatal("unexpected local cadence", registrar.pattern)
	}
	done := make(chan error, 1)
	go func() { done <- registrar.handler(ctx) }()
	select {
	case <-worker.started:
	case <-ctx.Done():
		t.Fatal("worker never started")
	}
	if err := registrar.handler(ctx); err != nil {
		t.Fatal(err)
	}
	if worker.calls.Load() != 1 {
		t.Fatal("fast ticks overlapped a running batch")
	}
	close(worker.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := registrar.handler(ctx); err != nil {
		t.Fatal(err)
	}
	if worker.calls.Load() != 2 {
		t.Fatal("finished batch blocked future work")
	}
}
