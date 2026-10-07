// This file verifies lifecycle job metadata, primary-node gating, and bounded execution.

package composition

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/lifecycle"
)

type serviceLifecycleWorkerFake struct {
	calls int
	limit int
	err   error
}

type serviceLifecycleProcessorFake struct {
	calls int
	limit int
	err   error
}

func (f *serviceLifecycleProcessorFake) RunTasks(
	_ context.Context,
	limit int,
) (*lifecycle.ProcessBatch, error) {
	f.calls++
	f.limit = limit
	return &lifecycle.ProcessBatch{Selected: 1, Processed: 1}, f.err
}

func (*serviceLifecycleProcessorFake) ProcessTask(context.Context, string) error {
	return errors.New("not used")
}

func (f *serviceLifecycleWorkerFake) Run(
	_ context.Context,
	limit int,
) (*lifecycle.Batch, error) {
	f.calls++
	f.limit = limit
	return &lifecycle.Batch{Selected: 1, Created: 1}, f.err
}

func TestServiceLifecycleProcessorJobRegistersBoundedPrimaryWorker(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &serviceLifecycleProcessorFake{}
	if err := RegisterServiceLifecycleProcessorJob(
		context.Background(), registrar, worker,
	); err != nil {
		t.Fatalf("register service lifecycle processor job: %v", err)
	}
	if registrar.name != serviceLifecycleProcessorJobName || registrar.handler == nil {
		t.Fatalf("unexpected lifecycle processor registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatalf("run service lifecycle processor job: %v", err)
	}
	if worker.calls != 1 || worker.limit != serviceLifecycleBatchLimit {
		t.Fatalf("unbounded lifecycle processor calls=%d limit=%d", worker.calls, worker.limit)
	}
}

func TestServiceLifecycleJobRegistersBoundedPrimaryWorker(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &serviceLifecycleWorkerFake{}
	if err := RegisterServiceLifecycleJob(context.Background(), registrar, worker); err != nil {
		t.Fatalf("register service lifecycle job: %v", err)
	}
	if registrar.pattern != serviceLifecycleJobPattern ||
		registrar.name != serviceLifecycleJobName || registrar.handler == nil {
		t.Fatalf("unexpected service lifecycle registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatalf("run service lifecycle job: %v", err)
	}
	if worker.calls != 1 || worker.limit != serviceLifecycleBatchLimit {
		t.Fatalf("unbounded lifecycle worker calls=%d limit=%d", worker.calls, worker.limit)
	}
}

func TestServiceLifecycleJobSkipsSecondaryAndPropagatesFailures(t *testing.T) {
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterServiceLifecycleJob(
		context.Background(), secondary, &serviceLifecycleWorkerFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary lifecycle registration calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("lifecycle worker failed")
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &serviceLifecycleWorkerFake{err: expected}
	if err := RegisterServiceLifecycleJob(context.Background(), registrar, worker); err != nil {
		t.Fatalf("register failing lifecycle job: %v", err)
	}
	if err := registrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("service lifecycle job swallowed failure: %v", err)
	}
}
