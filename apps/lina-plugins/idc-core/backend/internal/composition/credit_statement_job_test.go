// This file verifies primary-node and bounded credit statement job registration.

package composition

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/billing"
)

type creditStatementWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *creditStatementWorkerFake) RunCreditStatementBatch(
	_ context.Context,
	limit int,
) (*billing.CreditStatementBatch, error) {
	f.calls++
	f.limit = limit
	return &billing.CreditStatementBatch{Selected: 1, Issued: 1}, f.err
}

func TestCreditStatementJobRegistersOnlyBoundedPrimaryWorker(t *testing.T) {
	primary := &orderExpiryRegistrarFake{primary: true}
	worker := &creditStatementWorkerFake{}
	if err := RegisterCreditStatementJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register credit statement job: %v", err)
	}
	if primary.pattern != creditStatementJobPattern || primary.name != creditStatementJobName ||
		primary.handler == nil {
		t.Fatalf("unexpected credit statement registration: %+v", primary)
	}
	if err := primary.handler(context.Background()); err != nil {
		t.Fatalf("run credit statement job: %v", err)
	}
	if worker.calls != 1 || worker.limit != creditStatementJobBatchLimit {
		t.Fatalf("unbounded credit statement job calls=%d limit=%d", worker.calls, worker.limit)
	}

	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterCreditStatementJob(
		context.Background(), secondary, &creditStatementWorkerFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registration calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("credit statement worker failed")
	failing := &creditStatementWorkerFake{err: expected}
	failingRegistrar := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterCreditStatementJob(
		context.Background(), failingRegistrar, failing,
	); err != nil {
		t.Fatalf("register failing credit statement job: %v", err)
	}
	if err := failingRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("credit statement job swallowed failure: %v", err)
	}
}
