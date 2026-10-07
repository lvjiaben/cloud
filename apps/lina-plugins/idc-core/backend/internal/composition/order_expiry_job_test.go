package composition

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

type orderExpiryRegistrarFake struct {
	primary     bool
	addCalls    int
	pattern     string
	name        string
	displayName string
	description string
	handler     pluginhost.JobHandler
}

func (f *orderExpiryRegistrarFake) Add(
	ctx context.Context,
	pattern string,
	name string,
	handler pluginhost.JobHandler,
) error {
	return f.AddWithMetadata(ctx, pattern, name, name, "", handler)
}

func (f *orderExpiryRegistrarFake) AddWithMetadata(
	_ context.Context,
	pattern string,
	name string,
	displayName string,
	description string,
	handler pluginhost.JobHandler,
) error {
	f.addCalls++
	f.pattern, f.name, f.displayName, f.description = pattern, name, displayName, description
	f.handler = handler
	return nil
}

func (f *orderExpiryRegistrarFake) IsPrimaryNode() bool         { return f.primary }
func (*orderExpiryRegistrarFake) Services() capability.Services { return nil }
func (*orderExpiryRegistrarFake) BusinessExtensions() pluginhost.BusinessExtensionRegistrar {
	return nil
}

type orderExpiryLifecycleFake struct {
	calls int
	limit int
	err   error
}

func (f *orderExpiryLifecycleFake) ExpireDueOrders(
	_ context.Context,
	limit int,
) (commerce.OrderExpiryBatch, error) {
	f.calls++
	f.limit = limit
	return commerce.OrderExpiryBatch{Scanned: 1, Cancelled: 1}, f.err
}

func (f *orderExpiryLifecycleFake) FinalizePaymentClosure(
	context.Context,
	commerce.PaymentClosureConfirmation,
) (*commerce.OrderCancellation, error) {
	return nil, errors.New("not used")
}

func TestOrderExpiryJobRegistersBoundedPrimaryWorker(t *testing.T) {
	registrar := &orderExpiryRegistrarFake{primary: true}
	lifecycle := &orderExpiryLifecycleFake{}
	if err := RegisterOrderExpiryJob(context.Background(), registrar, lifecycle); err != nil {
		t.Fatalf("register order expiry job: %v", err)
	}
	if registrar.addCalls != 1 || registrar.handler == nil ||
		registrar.pattern != orderExpiryJobPattern || registrar.name != orderExpiryJobName ||
		registrar.displayName != orderExpiryJobDisplayName ||
		registrar.description != orderExpiryJobDescription {
		t.Fatalf("unexpected order expiry registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatalf("run order expiry job: %v", err)
	}
	if lifecycle.calls != 1 || lifecycle.limit != orderExpiryBatchLimit {
		t.Fatalf("unbounded order expiry invocation: calls=%d limit=%d", lifecycle.calls, lifecycle.limit)
	}
}

func TestOrderExpiryJobSkipsSecondaryAndPropagatesFailure(t *testing.T) {
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterOrderExpiryJob(
		context.Background(), secondary, &orderExpiryLifecycleFake{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary order expiry registration calls=%d err=%v", secondary.addCalls, err)
	}
	expected := errors.New("expiry failed")
	primary := &orderExpiryRegistrarFake{primary: true}
	lifecycle := &orderExpiryLifecycleFake{err: expected}
	if err := RegisterOrderExpiryJob(context.Background(), primary, lifecycle); err != nil {
		t.Fatalf("register failing order expiry job: %v", err)
	}
	if err := primary.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("order expiry job swallowed failure: %v", err)
	}
}
