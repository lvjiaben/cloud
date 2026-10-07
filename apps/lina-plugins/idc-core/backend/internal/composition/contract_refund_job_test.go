package composition

import (
	"context"
	"errors"
	"testing"
)

type contractRefundWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *contractRefundWorkerFake) Run(_ context.Context, limit int) (int, error) {
	f.calls++
	f.limit = limit
	return 1, f.err
}

func TestContractRefundJobRegistersOnlyBoundedPrimaryWorker(t *testing.T) {
	primary := &orderExpiryRegistrarFake{primary: true}
	worker := &contractRefundWorkerFake{}
	if err := RegisterContractRefundJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register contract refund job: %v", err)
	}
	if primary.pattern != contractRefundJobPattern || primary.name != contractRefundJobName ||
		primary.handler == nil {
		t.Fatalf("unexpected contract refund registration: %+v", primary)
	}
	if err := primary.handler(context.Background()); err != nil {
		t.Fatalf("run contract refund job: %v", err)
	}
	if worker.calls != 1 || worker.limit != contractRefundJobBatchLimit {
		t.Fatalf("unbounded contract refund job calls=%d limit=%d", worker.calls, worker.limit)
	}

	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterContractRefundJob(
		context.Background(), secondary, &contractRefundWorkerFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registration calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("contract refund worker failed")
	failing := &contractRefundWorkerFake{err: expected}
	failingRegistrar := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterContractRefundJob(
		context.Background(), failingRegistrar, failing,
	); err != nil {
		t.Fatalf("register failing contract refund job: %v", err)
	}
	if err := failingRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("contract refund job swallowed failure: %v", err)
	}
}
