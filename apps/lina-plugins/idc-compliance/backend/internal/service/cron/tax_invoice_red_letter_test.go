// This file verifies primary-node red-letter job registration and error propagation.

package cron

import (
	"context"
	"errors"
	"testing"

	"lina-plugin-idc-core/backend/cap/taxinvoicecap"
)

type taxInvoiceRedLetterWorkerStub struct {
	calls int
	limit int
	err   error
}

func (s *taxInvoiceRedLetterWorkerStub) Run(_ context.Context, limit int) (int, error) {
	s.calls++
	s.limit = limit
	return 1, s.err
}

func TestTaxInvoiceRedLetterRegistersOneBoundedPrimaryWorker(t *testing.T) {
	registrar := &recoveryRegistrarStub{primary: true}
	worker := &taxInvoiceRedLetterWorkerStub{}
	if err := RegisterTaxInvoiceRedLetter(context.Background(), registrar, worker); err != nil {
		t.Fatal(err)
	}
	if registrar.addCalls != 1 || registrar.pattern != taxInvoiceRedLetterPattern ||
		registrar.name != taxInvoiceRedLetterName ||
		registrar.displayName != taxInvoiceRedLetterDisplayName ||
		registrar.description != taxInvoiceRedLetterDescription || registrar.handler == nil {
		t.Fatalf("registration=%+v", registrar)
	}
	if err := registrar.handler(context.Background()); err != nil {
		t.Fatal(err)
	}
	if worker.calls != 1 || worker.limit != taxinvoicecap.MaximumRefundOperations {
		t.Fatalf("calls=%d limit=%d", worker.calls, worker.limit)
	}

	secondary := &recoveryRegistrarStub{}
	if err := RegisterTaxInvoiceRedLetter(
		context.Background(), secondary, &taxInvoiceRedLetterWorkerStub{},
	); err != nil || secondary.addCalls != 0 {
		t.Fatalf("secondary calls=%d err=%v", secondary.addCalls, err)
	}

	expected := errors.New("intake failed")
	failing := &taxInvoiceRedLetterWorkerStub{err: expected}
	failingRegistrar := &recoveryRegistrarStub{primary: true}
	if err := RegisterTaxInvoiceRedLetter(
		context.Background(), failingRegistrar, failing,
	); err != nil {
		t.Fatal(err)
	}
	if err := failingRegistrar.handler(context.Background()); !errors.Is(err, expected) {
		t.Fatalf("worker error was swallowed: %v", err)
	}
}
