package composition

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/renewal"
)

type automaticRenewalWorkerFake struct {
	calls int
	limit int
	err   error
}

func (*automaticRenewalWorkerFake) AttemptDueService(
	context.Context,
	string,
) (*renewal.AutomaticAttempt, error) {
	return nil, errors.New("not used")
}

func (f *automaticRenewalWorkerFake) RunAutomaticRenewals(
	_ context.Context,
	limit int,
) (*renewal.AutomaticBatch, error) {
	f.calls++
	f.limit = limit
	return &renewal.AutomaticBatch{Selected: 1, Processed: 1}, f.err
}

type providerRenewalWorkerFake struct {
	calls int
	limit int
	err   error
}

func (f *providerRenewalWorkerFake) RunProviderOperations(
	_ context.Context,
	limit int,
) (*renewal.ProviderBatch, error) {
	f.calls++
	f.limit = limit
	return &renewal.ProviderBatch{Selected: 1, Reconciled: 1}, f.err
}

func (*providerRenewalWorkerFake) DispatchProviderOperation(
	context.Context,
	string,
) (*renewal.Operation, error) {
	return nil, errors.New("not used")
}

func (*providerRenewalWorkerFake) ReconcileProviderOperation(
	context.Context,
	string,
) (*renewal.Operation, error) {
	return nil, errors.New("not used")
}

func TestRenewalJobsRegisterBoundedPrimaryWorkers(t *testing.T) {
	automaticRegistrar := &orderExpiryRegistrarFake{primary: true}
	automatic := &automaticRenewalWorkerFake{}
	if err := RegisterAutomaticRenewalJob(
		context.Background(), automaticRegistrar, automatic,
	); err != nil {
		t.Fatalf("register automatic renewal job: %v", err)
	}
	if automaticRegistrar.pattern != renewalAutomaticJobPattern ||
		automaticRegistrar.name != renewalAutomaticJobName || automaticRegistrar.handler == nil {
		t.Fatalf("unexpected automatic renewal registration: %+v", automaticRegistrar)
	}
	if err := automaticRegistrar.handler(context.Background()); err != nil {
		t.Fatalf("run automatic renewal job: %v", err)
	}
	if automatic.calls != 1 || automatic.limit != renewalAutomaticBatchLimit {
		t.Fatalf("unbounded automatic renewal job calls=%d limit=%d", automatic.calls, automatic.limit)
	}

	providerRegistrar := &orderExpiryRegistrarFake{primary: true}
	provider := &providerRenewalWorkerFake{}
	if err := RegisterRenewalProviderJob(
		context.Background(), providerRegistrar, provider,
	); err != nil {
		t.Fatalf("register Provider renewal job: %v", err)
	}
	if providerRegistrar.pattern != renewalProviderJobPattern ||
		providerRegistrar.name != renewalProviderJobName || providerRegistrar.handler == nil {
		t.Fatalf("unexpected Provider renewal registration: %+v", providerRegistrar)
	}
	if err := providerRegistrar.handler(context.Background()); err != nil {
		t.Fatalf("run Provider renewal job: %v", err)
	}
	if provider.calls != 1 || provider.limit != renewalProviderBatchLimit {
		t.Fatalf("unbounded Provider renewal job calls=%d limit=%d", provider.calls, provider.limit)
	}
}

func TestRenewalJobsSkipSecondaryAndPropagateFailures(t *testing.T) {
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterAutomaticRenewalJob(
		context.Background(), secondary, &automaticRenewalWorkerFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary automatic renewal registration calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("renewal worker failed")
	registrar := &orderExpiryRegistrarFake{primary: true}
	worker := &automaticRenewalWorkerFake{err: expected}
	if err := RegisterAutomaticRenewalJob(context.Background(), registrar, worker); err != nil {
		t.Fatalf("register failing automatic renewal job: %v", err)
	}
	if err := registrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("automatic renewal job swallowed failure: %v", err)
	}
}
