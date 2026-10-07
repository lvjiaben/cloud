package cron

import (
	"context"
	"errors"
	"testing"

	"lina-core/pkg/plugin/capability"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-compliance/backend/internal/service/personalverification"
)

type recoveryRegistrarStub struct {
	primary     bool
	addCalls    int
	pattern     string
	name        string
	displayName string
	description string
	handler     pluginhost.JobHandler
}

func (s *recoveryRegistrarStub) Add(
	ctx context.Context,
	pattern string,
	name string,
	handler pluginhost.JobHandler,
) error {
	return s.AddWithMetadata(ctx, pattern, name, name, "", handler)
}

func (s *recoveryRegistrarStub) AddWithMetadata(
	_ context.Context,
	pattern string,
	name string,
	displayName string,
	description string,
	handler pluginhost.JobHandler,
) error {
	s.addCalls++
	s.pattern, s.name, s.displayName, s.description = pattern, name, displayName, description
	s.handler = handler
	return nil
}

func (s *recoveryRegistrarStub) IsPrimaryNode() bool         { return s.primary }
func (*recoveryRegistrarStub) Services() capability.Services { return nil }
func (*recoveryRegistrarStub) BusinessExtensions() pluginhost.BusinessExtensionRegistrar {
	return nil
}

type recoveryWorkerStub struct {
	calls int
	limit int
	err   error
}

func (*recoveryWorkerStub) Current(context.Context) (*personalverification.Verification, error) {
	return nil, errors.New("not used")
}

func (*recoveryWorkerStub) Start(
	context.Context,
	personalverification.StartInput,
) (*personalverification.Verification, error) {
	return nil, errors.New("recovery must not replay initiation")
}

func (*recoveryWorkerStub) QueryCurrent(
	context.Context,
	string,
) (*personalverification.Verification, error) {
	return nil, errors.New("not used")
}

func (s *recoveryWorkerStub) Recover(
	_ context.Context,
	limit int,
) (*personalverification.RecoveryResult, error) {
	s.calls++
	s.limit = limit
	return &personalverification.RecoveryResult{Selected: 1, Queried: 1}, s.err
}

func TestPersonalVerificationRecoveryRegistersOneBoundedPrimaryWorker(t *testing.T) {
	registrar := &recoveryRegistrarStub{primary: true}
	worker := &recoveryWorkerStub{}
	if err := RegisterPersonalVerificationRecovery(context.Background(), registrar, worker); err != nil {
		t.Fatal(err)
	}
	if registrar.addCalls != 1 || registrar.pattern != personalVerificationRecoveryPattern ||
		registrar.name != personalVerificationRecoveryName ||
		registrar.displayName != personalVerificationRecoveryDisplayName ||
		registrar.description != personalVerificationRecoveryDescription || registrar.handler == nil {
		t.Fatalf("registration=%+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if worker.calls != 1 || worker.limit != personalVerificationRecoveryBatchLimit {
		t.Fatalf("calls=%d limit=%d", worker.calls, worker.limit)
	}
	secondary := &recoveryRegistrarStub{}
	if err := RegisterPersonalVerificationRecovery(
		context.Background(), secondary, &recoveryWorkerStub{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary calls=%d err=%v", secondary.addCalls, err)
	}
	expected := errors.New("provider query failed")
	failing := &recoveryWorkerStub{err: expected}
	failingRegistrar := &recoveryRegistrarStub{primary: true}
	if err := RegisterPersonalVerificationRecovery(
		context.Background(), failingRegistrar, failing,
	); err != nil {
		t.Fatal(err)
	}
	if err := failingRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("worker error was swallowed: %v", err)
	}
}
