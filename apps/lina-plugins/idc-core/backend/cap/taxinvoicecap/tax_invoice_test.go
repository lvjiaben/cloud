// This file verifies the source-only tax-invoice runtime rejects incomplete dependencies.

package taxinvoicecap

import (
	"context"
	"testing"
)

type sourceStub struct{}

func (sourceStub) ListCurrentCustomerOptions(
	context.Context,
	AllocationUsageReader,
) (*RequestOptions, error) {
	return &RequestOptions{}, nil
}

func (sourceStub) PrepareCurrentCustomerRequest(
	context.Context,
	AllocationUsageReader,
	string,
	[]Selection,
) (*RequestSnapshot, error) {
	return &RequestSnapshot{}, nil
}

func (sourceStub) GetPostedRefund(context.Context, string) (*RefundOperation, error) {
	return &RefundOperation{}, nil
}

func (sourceStub) ListPostedRefundOperations(
	context.Context,
	string,
	int,
) ([]PostedRefundReference, error) {
	return nil, nil
}

func TestValidateRuntimeRejectsTypedNil(t *testing.T) {
	var source *sourceStub
	if err := ValidateRuntime(Runtime{Source: source, RefundFeed: source}); err == nil {
		t.Fatal("typed-nil tax invoice source was accepted")
	}
	if err := ValidateRuntime(Runtime{Source: sourceStub{}, RefundFeed: sourceStub{}}); err != nil {
		t.Fatalf("valid tax invoice source was rejected: %v", err)
	}
}

func TestRequestSnapshotClearReleasesItems(t *testing.T) {
	snapshot := &RequestSnapshot{Invoices: []InvoiceSnapshot{{
		Items: []ItemSnapshot{{BillingInvoiceItemNo: "item-1"}},
	}}}
	snapshot.Clear()
	if snapshot.Invoices != nil {
		t.Fatal("tax invoice snapshot item slices were not released")
	}
}
