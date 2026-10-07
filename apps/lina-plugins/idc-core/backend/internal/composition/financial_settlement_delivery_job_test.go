package composition

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/fulfillment"
)

type financialDeliveryWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *financialDeliveryWorkerFake) Run(_ context.Context, limit int) (fulfillment.WorkerResult, error) {
	f.calls++
	f.limit = limit
	return fulfillment.WorkerResult{Claimed: 1, Published: 1}, f.err
}

func TestFinancialSettlementDeliveryJobRegistersBeforeOptionalProviders(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &financialDeliveryWorkerFake{}
	if err := RegisterFinancialSettlementDeliveryJob(context.Background(), registrar, worker); err != nil {
		t.Fatalf("register financial settlement delivery job: %v", err)
	}
	if registrar.addCalls != 1 || registrar.handler == nil ||
		registrar.pattern != financialSettlementDeliveryJobPattern ||
		registrar.name != financialSettlementDeliveryJobName {
		t.Fatalf("unexpected financial delivery job registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatalf("run financial settlement delivery job: %v", err)
	}
	if worker.calls != 1 || worker.limit != financialSettlementDeliveryBatchLimit {
		t.Fatalf("unbounded financial delivery invocation: calls=%d limit=%d", worker.calls, worker.limit)
	}

	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterFinancialSettlementDeliveryJob(context.Background(), secondary, worker); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary financial delivery registration calls=%d err=%v", secondary.addCalls, err)
	}
	expected := errors.New("delivery failed")
	failedRegistrar := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterFinancialSettlementDeliveryJob(
		context.Background(), failedRegistrar, &financialDeliveryWorkerFake{err: expected},
	); err != nil {
		t.Fatalf("register failing financial delivery job: %v", err)
	}
	if err := failedRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("financial delivery job swallowed failure: %v", err)
	}
}
