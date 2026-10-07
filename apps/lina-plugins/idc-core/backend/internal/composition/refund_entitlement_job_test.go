package composition

import (
	"context"
	"errors"
	"testing"
)

type refundEntitlementWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *refundEntitlementWorkerFake) Run(_ context.Context, limit int) (int, error) {
	f.calls++
	f.limit = limit
	return 1, f.err
}

func TestRefundEntitlementJobRegistersOnlyBoundedPrimaryWorker(t *testing.T) {
	primary := &orderExpiryRegistrarFake{primary: true}
	worker := &refundEntitlementWorkerFake{}
	if err := RegisterRefundEntitlementJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register refund entitlement job: %v", err)
	}
	if primary.pattern != refundEntitlementJobPattern || primary.name != refundEntitlementJobName ||
		primary.handler == nil {
		t.Fatalf("unexpected refund entitlement registration: %+v", primary)
	}
	if err := primary.handler(context.Background()); err != nil {
		t.Fatalf("run refund entitlement job: %v", err)
	}
	if worker.calls != 1 || worker.limit != refundEntitlementJobBatchLimit {
		t.Fatalf("unbounded refund entitlement calls=%d limit=%d", worker.calls, worker.limit)
	}

	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterRefundEntitlementJob(
		context.Background(), secondary, &refundEntitlementWorkerFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registration calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("refund entitlement worker failed")
	failingRegistrar := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterRefundEntitlementJob(
		context.Background(), failingRegistrar, &refundEntitlementWorkerFake{err: expected},
	); err != nil {
		t.Fatalf("register failing refund entitlement job: %v", err)
	}
	if err := failingRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("refund entitlement job swallowed failure: %v", err)
	}
}
