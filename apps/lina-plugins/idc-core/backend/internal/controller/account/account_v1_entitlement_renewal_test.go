// This file verifies ordinary renewals retain owner-side pricing and retry identity.
package account

import (
	"context"
	"lina-core/pkg/bizerr"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/renewal"
	"testing"
	"time"
)

type ordinaryRenewalFake struct {
	renewal.Service
	quoteInput   renewal.QuoteInput
	confirmInput renewal.ConfirmInput
	fail         bool
}

func (f *ordinaryRenewalFake) QuoteCurrentCustomer(_ context.Context, in renewal.QuoteInput) (*renewal.Quote, error) {
	f.quoteInput = in
	if f.fail {
		return nil, bizerr.NewCode(renewal.CodeNotAccessible)
	}
	return &renewal.Quote{QuoteNo: "quote-owned", CurrencyCode: "CNY", TotalAmountMinor: 9007199254740993, TaxAmountMinor: 0, ProjectedExpiresAt: time.UnixMilli(1800000000000), ExpiresAt: time.UnixMilli(1790000000000)}, nil
}
func (f *ordinaryRenewalFake) ConfirmCurrentCustomer(_ context.Context, in renewal.ConfirmInput) (*renewal.Operation, error) {
	f.confirmInput = in
	if f.fail {
		return nil, bizerr.NewCode(renewal.CodeConflict)
	}
	return &renewal.Operation{OperationNo: "operation-owned", InvoiceNo: "invoice-owned", Status: "pending_payment"}, nil
}
func TestEntitlementRenewalPreservesExactAmountAndOwnerInputs(t *testing.T) {
	fake := &ordinaryRenewalFake{}
	c := &ControllerV1{renewals: fake}
	res, err := c.EntitlementRenewalQuote(t.Context(), &v1.EntitlementRenewalQuoteReq{EntitlementNo: "owned", RenewalCycles: 3, IdempotencyKey: "stable-quote-key"})
	if err != nil || res.Quote.TotalAmountMinor != "9007199254740993" || res.Quote.ProjectedExpiresAt != 1800000000000 {
		t.Fatalf("quote=%+v err=%v", res, err)
	}
	if fake.quoteInput.EntitlementNo != "owned" || fake.quoteInput.RenewalCycles != 3 || fake.quoteInput.IdempotencyKey != "stable-quote-key" || fake.quoteInput.FormalBillingRuleNo != "" {
		t.Fatal("ordinary renewal inputs changed")
	}
	paid, err := c.EntitlementRenewalConfirm(t.Context(), &v1.EntitlementRenewalConfirmReq{QuoteNo: "quote-owned", IdempotencyKey: "stable-confirm-key"})
	if err != nil || paid.Operation.InvoiceNo != "invoice-owned" || paid.Operation.StatusCode != "pending_payment" || fake.confirmInput.IdempotencyKey != "stable-confirm-key" {
		t.Fatalf("confirmation=%+v err=%v", paid, err)
	}
}
func TestEntitlementRenewalPropagatesOwnershipAndQuoteRejection(t *testing.T) {
	c := &ControllerV1{renewals: &ordinaryRenewalFake{fail: true}}
	res, err := c.EntitlementRenewalQuote(t.Context(), &v1.EntitlementRenewalQuoteReq{})
	if res != nil || !bizerr.Is(err, renewal.CodeNotAccessible) {
		t.Fatalf("foreign quote: res=%+v err=%v", res, err)
	}
	confirmed, err := c.EntitlementRenewalConfirm(t.Context(), &v1.EntitlementRenewalConfirmReq{})
	if confirmed != nil || !bizerr.Is(err, renewal.CodeConflict) {
		t.Fatalf("stale confirmation: res=%+v err=%v", confirmed, err)
	}
}
