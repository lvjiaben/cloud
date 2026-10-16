// This file checks primary-node trial job gating, bounded execution, and errors.

package composition

import (
	"context"
	"errors"
	"testing"
)

// trialLifecycleFake records the actual scheduled batch size.
type trialLifecycleFake struct {
	calls int
	limit int
	err   error
}

func (f *trialLifecycleFake) Run(_ context.Context, limit int) error {
	f.calls++
	f.limit = limit
	return f.err
}

func TestTrialLifecycleJobPrimaryBoundAndFailure(t *testing.T) {
	ctx := context.Background()
	expected := errors.New("trial worker unavailable")
	worker := &trialLifecycleFake{err: expected}
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterTrialLifecycleJob(ctx, secondary, worker); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary registered: %v %+v", err, secondary)
	}
	primary := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterTrialLifecycleJob(ctx, primary, worker); err != nil {
		t.Fatal(err)
	}
	if primary.addCalls != 1 || primary.pattern != "@every 30s" || primary.name != "idc-core-trial-lifecycle" || primary.handler == nil {
		t.Fatalf("wrong trial job registration: %+v", primary)
	}
	if err := primary.handler(ctx); !errors.Is(err, expected) || worker.calls != 1 || worker.limit != 32 {
		t.Fatalf("wrong invocation: error=%v worker=%+v", err, worker)
	}
	if err := RegisterTrialLifecycleJob(ctx, primary, nil); err == nil {
		t.Fatal("missing worker accepted")
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := RegisterTrialLifecycleJob(cancelled, primary, worker); err == nil {
		t.Fatal("cancelled startup accepted")
	}
}
