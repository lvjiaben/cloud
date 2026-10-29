package composition

import (
	"context"
	"errors"
	"testing"
)

type resizeRefundWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *resizeRefundWorkerFake) Run(_ context.Context, limit int) (int, error) {
	f.calls++
	f.limit = limit
	return 1, f.err
}

func TestResizeRefundJobRegistersOnlyBoundedPrimaryWorker(t *testing.T) {
	primary := &orderExpiryRegistrarFake{primary: true}
	worker := &resizeRefundWorkerFake{}
	if err := RegisterResizeRefundJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register resize refund job: %v", err)
	}
	if primary.pattern != resizeRefundJobPattern || primary.name != resizeRefundJobName ||
		primary.handler == nil {
		t.Fatalf("unexpected resize refund registration: %+v", primary)
	}
	if err := primary.handler(context.Background()); err != nil {
		t.Fatalf("run resize refund job: %v", err)
	}
	if worker.calls != 1 || worker.limit != resizeRefundJobBatchLimit {
		t.Fatalf("unbounded resize refund job calls=%d limit=%d", worker.calls, worker.limit)
	}

	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterResizeRefundJob(
		context.Background(), secondary, &resizeRefundWorkerFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registration calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("resize refund worker failed")
	failing := &resizeRefundWorkerFake{err: expected}
	failingRegistrar := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterResizeRefundJob(
		context.Background(), failingRegistrar, failing,
	); err != nil {
		t.Fatalf("register failing resize refund job: %v", err)
	}
	if err := failingRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("resize refund job swallowed failure: %v", err)
	}
}
