// This file verifies owner-gate recovery registration is primary-only, bounded and preserves failures.
package composition

import (
	"context"
	"errors"
	"lina-plugin-idc-core/backend/internal/service/productfulfillment"
	"testing"
)

type gateRecoveryWorkerFake struct {
	calls, limit int
	err          error
}

func (f *gateRecoveryWorkerFake) Run(_ context.Context, limit int) (productfulfillment.GateRecoveryResult, error) {
	f.calls++
	f.limit = limit
	return productfulfillment.GateRecoveryResult{}, f.err
}

// TestGateRecoveryJobBinding verifies a scheduled callback delegates to the injected shared worker.
func TestGateRecoveryJobBinding(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	expected := errors.New("owner unavailable")
	worker := &gateRecoveryWorkerFake{err: expected}
	if err := RegisterProductFulfillmentGateRecoveryJob(context.Background(), registrar, worker); err != nil {
		t.Fatal(err)
	}
	if registrar.name != "idc-core-product-fulfillment-gate-recovery" || registrar.handler == nil {
		t.Fatalf("missing binding: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); !errors.Is(err, expected) || worker.calls != 1 || worker.limit != 20 {
		t.Fatalf("callback not delegated: %v %+v", err, worker)
	}
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterProductFulfillmentGateRecoveryJob(context.Background(), secondary, worker); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registered: %v %+v", err, secondary)
	}
	if err := RegisterProductFulfillmentGateRecoveryJob(context.Background(), registrar, nil); err == nil {
		t.Fatal("nil worker accepted")
	}
}
