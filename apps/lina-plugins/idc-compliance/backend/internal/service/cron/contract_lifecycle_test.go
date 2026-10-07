package cron

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-compliance/backend/internal/service/contractlifecycle"
	"lina-plugin-idc-core/backend/cap/idccap"
)

type lifecycleWorkerStub struct {
	calls int
	limit int
	err   error
}

func (s *lifecycleWorkerStub) Consume(
	_ context.Context,
	limit int,
) (contractlifecycle.ConsumeResult, error) {
	s.calls++
	s.limit = limit
	return contractlifecycle.ConsumeResult{Received: 1, Acknowledged: 1}, s.err
}

func (*lifecycleWorkerStub) Apply(
	context.Context,
	contractlifecycle.Observation,
	idccap.BusinessAuditWriter,
) (contractlifecycle.ApplyResult, error) {
	return contractlifecycle.ApplyResult{}, errors.New("not used")
}

func TestContractLifecycleRegistersOneBoundedPrimaryWorker(t *testing.T) {
	registrar := &recoveryRegistrarStub{primary: true}
	worker := &lifecycleWorkerStub{}
	if err := RegisterContractLifecycle(context.Background(), registrar, worker); err != nil {
		t.Fatal(err)
	}
	if registrar.addCalls != 1 || registrar.pattern != contractLifecyclePattern ||
		registrar.name != contractLifecycleName || registrar.displayName != contractLifecycleDisplayName ||
		registrar.description != contractLifecycleDescription || registrar.handler == nil {
		t.Fatalf("registration=%+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if worker.calls != 1 || worker.limit != contractlifecycle.MaximumBatchSize {
		t.Fatalf("calls=%d limit=%d", worker.calls, worker.limit)
	}
	secondary := &recoveryRegistrarStub{}
	if err := RegisterContractLifecycle(context.Background(), secondary, &lifecycleWorkerStub{}); err != nil ||
		secondary.addCalls != 0 {
		t.Fatalf("secondary calls=%d err=%v", secondary.addCalls, err)
	}
}
