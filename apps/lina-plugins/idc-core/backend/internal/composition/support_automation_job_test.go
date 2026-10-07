package composition

import (
	"context"
	"errors"
	"testing"
)

type supportAutomationWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *supportAutomationWorkerFake) AutoCloseResolved(_ context.Context, limit int) (int, error) {
	f.calls++
	f.limit = limit
	return 1, f.err
}

func TestSupportAutomationJobRegistersBoundedPrimaryWorker(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &supportAutomationWorkerFake{}
	if err := RegisterSupportAutomationJob(context.Background(), registrar, worker); err != nil {
		t.Fatalf("register support automation job: %v", err)
	}
	if registrar.addCalls != 1 || registrar.handler == nil || registrar.pattern != supportAutomationJobPattern ||
		registrar.name != supportAutomationJobName || registrar.displayName != supportAutomationJobDisplayName ||
		registrar.description != supportAutomationJobDescription {
		t.Fatalf("unexpected support automation registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatalf("run support automation job: %v", err)
	}
	if worker.calls != 1 || worker.limit != supportAutomationJobBatchLimit {
		t.Fatalf("unbounded support automation invocation: calls=%d limit=%d", worker.calls, worker.limit)
	}
}

func TestSupportAutomationJobSkipsSecondaryAndPropagatesFailure(t *testing.T) {
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterSupportAutomationJob(context.Background(), secondary, &supportAutomationWorkerFake{}); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary support automation registration calls=%d err=%v", secondary.addCalls, err)
	}
	expected := errors.New("automation failed")
	primary := &orderExpiryRegistrarFake{primary: true}
	worker := &supportAutomationWorkerFake{err: expected}
	if err := RegisterSupportAutomationJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register failing support automation job: %v", err)
	}
	if err := primary.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("support automation job swallowed failure: %v", err)
	}
}
