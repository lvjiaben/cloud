package composition

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/pluginhost"
)

type jobRegistrarFake struct {
	primary     bool
	addCalls    int
	pattern     string
	name        string
	displayName string
	description string
	handler     pluginhost.JobHandler
}

func (fake *jobRegistrarFake) Add(
	ctx context.Context,
	pattern string,
	name string,
	handler pluginhost.JobHandler,
) error {
	return fake.AddWithMetadata(ctx, pattern, name, name, "", handler)
}

func (fake *jobRegistrarFake) AddWithMetadata(
	_ context.Context,
	pattern string,
	name string,
	displayName string,
	description string,
	handler pluginhost.JobHandler,
) error {
	fake.addCalls++
	fake.pattern = pattern
	fake.name = name
	fake.displayName = displayName
	fake.description = description
	fake.handler = handler
	return nil
}

func (fake *jobRegistrarFake) IsPrimaryNode() bool                                  { return fake.primary }
func (*jobRegistrarFake) Services() capability.Services                             { return nil }
func (*jobRegistrarFake) BusinessExtensions() pluginhost.BusinessExtensionRegistrar { return nil }

type accessoryRefreshWorkerFake struct {
	calls     int
	remaining int
	err       error
}

func (f *accessoryRefreshWorkerFake) RunOne(context.Context) (bool, error) {
	f.calls++
	if f.err != nil {
		return true, f.err
	}
	if f.remaining == 0 {
		return false, nil
	}
	f.remaining--
	return true, nil
}

func TestAccessoryRefreshJobRegistersOnlyOnPrimaryAndBoundsWork(t *testing.T) {
	secondary := &jobRegistrarFake{}
	if err := RegisterAccessoryRefreshJob(context.Background(), secondary, &accessoryRefreshWorkerFake{}); err != nil {
		t.Fatal(err)
	}
	if secondary.addCalls != 0 {
		t.Fatal("secondary node registered accessory refresh job")
	}

	registrar := &jobRegistrarFake{primary: true}
	worker := &accessoryRefreshWorkerFake{remaining: accessoryRefreshBatchLimit + 2}
	if err := RegisterAccessoryRefreshJob(context.Background(), registrar, worker); err != nil {
		t.Fatal(err)
	}
	if registrar.name != accessoryRefreshJobName || registrar.pattern != accessoryRefreshJobPattern || registrar.handler == nil {
		t.Fatalf("unexpected accessory refresh registration: %+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if worker.calls != accessoryRefreshBatchLimit || worker.remaining != 2 {
		t.Fatalf("unbounded accessory refresh batch: calls=%d remaining=%d", worker.calls, worker.remaining)
	}
}

func TestAccessoryRefreshJobStopsOnIdleOrFailure(t *testing.T) {
	registrar := &jobRegistrarFake{primary: true}
	idle := &accessoryRefreshWorkerFake{}
	if err := RegisterAccessoryRefreshJob(context.Background(), registrar, idle); err != nil {
		t.Fatal(err)
	}
	if err := registrar.handler(context.Background()); err != nil || idle.calls != 1 {
		t.Fatalf("idle refresh calls=%d err=%v", idle.calls, err)
	}

	expected := errors.New("refresh failed")
	registrar = &jobRegistrarFake{primary: true}
	failing := &accessoryRefreshWorkerFake{err: expected}
	if err := RegisterAccessoryRefreshJob(context.Background(), registrar, failing); err != nil {
		t.Fatal(err)
	}
	if err := registrar.handler(context.Background()); !errors.Is(err, expected) || failing.calls != 1 {
		t.Fatalf("failure refresh calls=%d err=%v", failing.calls, err)
	}
}
