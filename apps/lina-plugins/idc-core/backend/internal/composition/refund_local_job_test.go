// This file verifies primary-node and bounded local refund job registration.

package composition

import (
	"context"
	"errors"
	"testing"
)

type localRefundWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *localRefundWorkerFake) Run(_ context.Context, limit int) (int, error) {
	f.calls++
	f.limit = limit
	return 1, f.err
}

func TestLocalRefundJobRegistersOnlyBoundedPrimaryWorker(t *testing.T) {
	primary := &orderExpiryRegistrarFake{primary: true}
	worker := &localRefundWorkerFake{}
	if err := RegisterLocalRefundJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register local refund job: %v", err)
	}
	if primary.pattern != refundLocalJobPattern || primary.name != refundLocalJobName ||
		primary.handler == nil {
		t.Fatalf("unexpected local refund registration: %+v", primary)
	}
	if err := primary.handler(context.Background()); err != nil {
		t.Fatalf("run local refund job: %v", err)
	}
	if worker.calls != 1 || worker.limit != refundLocalJobBatchLimit {
		t.Fatalf("unbounded local refund job calls=%d limit=%d", worker.calls, worker.limit)
	}

	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterLocalRefundJob(
		context.Background(), secondary, &localRefundWorkerFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registration calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("local refund worker failed")
	failing := &localRefundWorkerFake{err: expected}
	failingRegistrar := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterLocalRefundJob(
		context.Background(), failingRegistrar, failing,
	); err != nil {
		t.Fatalf("register failing local refund job: %v", err)
	}
	if err := failingRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("local refund job swallowed failure: %v", err)
	}
}
