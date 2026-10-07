package composition

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

type serverOperationWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *serverOperationWorkerFake) Run(_ context.Context, limit int) (*serveroperation.Batch, error) {
	f.calls++
	f.limit = limit
	return &serveroperation.Batch{Selected: 1, Dispatched: 1}, f.err
}

func TestServerOperationJobIsPrimaryBoundedAndReportsFailures(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &serverOperationWorkerFake{}
	if err := RegisterServerOperationJob(context.Background(), registrar, worker); err != nil {
		t.Fatal(err)
	}
	if registrar.addCalls != 1 || registrar.handler == nil ||
		registrar.pattern != serverOperationJobPattern || registrar.name != serverOperationJobName {
		t.Fatalf("unexpected server operation registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if worker.calls != 1 || worker.limit != serverOperationBatchLimit {
		t.Fatalf("unbounded server operation job calls=%d limit=%d", worker.calls, worker.limit)
	}
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterServerOperationJob(context.Background(), secondary, worker); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registration calls=%d err=%v", secondary.addCalls, err)
	}
	expected := errors.New("server operation failed")
	failed := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterServerOperationJob(context.Background(), failed,
		&serverOperationWorkerFake{err: expected}); err != nil {
		t.Fatal(err)
	}
	if err := failed.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("server operation job swallowed failure: %v", err)
	}
}
