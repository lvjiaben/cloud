package commerce

import (
	"strings"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func TestNormalizeCustomerCancellationBindsAudienceAndHashesKey(t *testing.T) {
	audience := principal.Audience{
		UserID: 42, Code: principal.AudienceCustomer,
		Status: principal.AudienceStatusActive, CustomerNo: "CUS_42",
	}
	input, err := normalizeCustomerCancellation(audience, OrderCancellationRequest{
		OrderNo: "ORD_42", IdempotencyKey: "cancel-order-0042",
	})
	if err != nil {
		t.Fatalf("normalize customer cancellation: %v", err)
	}
	if input.orderNo != "ORD_42" || input.source != OrderCancellationSourceCustomer ||
		input.reason != OrderCancellationReasonCustomerRequested || input.requestedBy != 42 ||
		len(input.requestDigest) != 64 || strings.Contains(input.requestDigest, "cancel-order-0042") {
		t.Fatalf("normalized cancellation=%+v", input)
	}
	replay, err := normalizeCustomerCancellation(audience, OrderCancellationRequest{
		OrderNo: "ORD_42", IdempotencyKey: "cancel-order-0042",
	})
	if err != nil || replay.requestDigest != input.requestDigest {
		t.Fatalf("deterministic cancellation replay=%+v err=%v", replay, err)
	}
	for _, invalid := range []OrderCancellationRequest{
		{OrderNo: " ORD_42", IdempotencyKey: "cancel-order-0042"},
		{OrderNo: "ORD_42", IdempotencyKey: "short"},
	} {
		if _, err = normalizeCustomerCancellation(audience, invalid); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("invalid cancellation %+v returned %v", invalid, err)
		}
	}
}

func TestNormalizePaymentClosureRequiresExactEvidence(t *testing.T) {
	valid := PaymentClosureConfirmation{
		CancellationNo: "CAN_0123456789abcdef0123456789abcdef",
		Outcome:        PaymentClosureOutcomeNoAttempts,
		Reference:      "payment-owner-closure-1", EvidenceDigest: strings.Repeat("a", 64),
	}
	if normalized := normalizePaymentClosureConfirmation(valid); normalized != valid {
		t.Fatalf("valid confirmation normalized to %+v", normalized)
	}
	invalid := valid
	invalid.EvidenceDigest = "raw-evidence"
	if normalized := normalizePaymentClosureConfirmation(invalid); normalized.CancellationNo != "" {
		t.Fatalf("invalid evidence accepted: %+v", normalized)
	}
}
