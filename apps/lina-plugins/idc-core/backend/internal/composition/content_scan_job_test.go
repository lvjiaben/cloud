package composition

import (
	"context"
	"errors"
	"testing"
)

type contentScanWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *contentScanWorkerFake) Run(_ context.Context, limit int) (int, error) {
	f.calls++
	f.limit = limit
	return 1, f.err
}

func TestContentScanJobRegistersBoundedPrimaryWorker(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &contentScanWorkerFake{}
	if err := RegisterContentScanJob(context.Background(), registrar, worker); err != nil {
		t.Fatalf("register content scan job: %v", err)
	}
	if registrar.addCalls != 1 || registrar.handler == nil || registrar.pattern != contentScanJobPattern ||
		registrar.name != contentScanJobName || registrar.displayName != contentScanJobDisplayName ||
		registrar.description != contentScanJobDescription {
		t.Fatalf("unexpected content scan registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatalf("run content scan job: %v", err)
	}
	if worker.calls != 1 || worker.limit != contentScanJobBatchLimit {
		t.Fatalf("unbounded content scan invocation: calls=%d limit=%d", worker.calls, worker.limit)
	}
}

func TestContentScanJobSkipsSecondaryAndPropagatesFailure(t *testing.T) {
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterContentScanJob(context.Background(), secondary, &contentScanWorkerFake{}); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary content scan registration calls=%d err=%v", secondary.addCalls, err)
	}
	expected := errors.New("scan failed")
	primary := &orderExpiryRegistrarFake{primary: true}
	worker := &contentScanWorkerFake{err: expected}
	if err := RegisterContentScanJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register failing content scan job: %v", err)
	}
	if err := primary.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("content scan job swallowed failure: %v", err)
	}
}
