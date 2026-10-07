package composition

import (
	"context"
	"errors"
	"testing"
)

type productAutomationSecretRecoveryWorkerFake struct {
	calls int
	limit int
	err   error
}

func (w *productAutomationSecretRecoveryWorkerFake) Run(_ context.Context, limit int) (int, error) {
	w.calls++
	w.limit = limit
	return 1, w.err
}

func TestProductAutomationSecretRecoveryJobIsPrimaryBoundedAndPropagatesFailure(t *testing.T) {
	secondary := &orderExpiryRegistrarFake{}
	if err := RegisterProductAutomationSecretRecoveryJob(context.Background(), secondary, &productAutomationSecretRecoveryWorkerFake{}); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary secret recovery registration calls=%d err=%v", secondary.addCalls, err)
	}
	expected := errors.New("recovery failed")
	worker := &productAutomationSecretRecoveryWorkerFake{err: expected}
	primary := &orderExpiryRegistrarFake{primary: true}
	if err := RegisterProductAutomationSecretRecoveryJob(context.Background(), primary, worker); err != nil {
		t.Fatalf("register secret recovery job: %v", err)
	}
	if primary.addCalls != 1 || primary.handler == nil || primary.pattern != productAutomationSecretRecoveryJobPattern || primary.name != productAutomationSecretRecoveryJobName {
		t.Fatalf("unexpected secret recovery registration: %+v", primary)
	}
	if err := primary.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("secret recovery failure was swallowed: %v", err)
	}
	if worker.calls != 1 || worker.limit != productAutomationSecretRecoveryBatchLimit {
		t.Fatalf("secret recovery invocation calls=%d limit=%d", worker.calls, worker.limit)
	}
}
