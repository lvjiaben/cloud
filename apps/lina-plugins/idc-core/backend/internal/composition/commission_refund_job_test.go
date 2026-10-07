package composition

import (
	"context"
	"errors"
	"testing"
)

type commissionRefundWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *commissionRefundWorkerFake) Run(_ context.Context, limit int) (int, error) {
	f.calls++
	f.limit = limit
	return 1, f.err
}

func TestCommissionRefundJobRegistersOnlyBoundedPrimaryWorker(t *testing.T) {
	primary := &orderExpiryRegistrarFake{primary: true}
	worker := &commissionRefundWorkerFake{}
	if err := RegisterCommissionRefundJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register commission refund job: %v", err)
	}
	if primary.pattern != commissionRefundJobPattern || primary.name != commissionRefundJobName ||
		primary.handler == nil {
		t.Fatalf("unexpected commission refund registration: %+v", primary)
	}
	if err := primary.handler(context.Background()); err != nil {
		t.Fatalf("run commission refund job: %v", err)
	}
	if worker.calls != 1 || worker.limit != commissionRefundJobBatchLimit {
		t.Fatalf("unbounded commission refund job calls=%d limit=%d", worker.calls, worker.limit)
	}
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterCommissionRefundJob(
		context.Background(), secondary, &commissionRefundWorkerFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registration calls=%d err=%v", secondary.addCalls, err)
	}
	expected := errors.New("commission refund worker failed")
	failing := &commissionRefundWorkerFake{err: expected}
	failingRegistrar := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterCommissionRefundJob(
		context.Background(), failingRegistrar, failing,
	); err != nil {
		t.Fatalf("register failing commission refund job: %v", err)
	}
	if err := failingRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("commission refund job swallowed failure: %v", err)
	}
}
