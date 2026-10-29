package commerce

import (
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestNormalizeCheckoutRequestHashesIdempotencyAndBindsQuote(t *testing.T) {
	first, err := normalizeCheckoutRequest("CUS_test_customer", CheckoutRequest{
		QuoteNo: "QTE_test_quote", IdempotencyKey: "checkout-key-0001",
	})
	if err != nil {
		t.Fatalf("normalize checkout request: %v", err)
	}
	if first.idempotencyDigest == "checkout-key-0001" || len(first.idempotencyDigest) != 64 ||
		len(first.requestDigest) != 64 {
		t.Fatalf("checkout digests=%+v", first)
	}
	replayed, err := normalizeCheckoutRequest("CUS_test_customer", CheckoutRequest{
		QuoteNo: "QTE_test_quote", IdempotencyKey: "checkout-key-0001",
	})
	if err != nil || replayed != first {
		t.Fatalf("checkout normalization replay=%+v err=%v", replayed, err)
	}
	changed, err := normalizeCheckoutRequest("CUS_test_customer", CheckoutRequest{
		QuoteNo: "QTE_changed_quote", IdempotencyKey: "checkout-key-0001",
	})
	if err != nil || changed.idempotencyDigest != first.idempotencyDigest ||
		changed.requestDigest == first.requestDigest {
		t.Fatalf("changed checkout normalization=%+v err=%v", changed, err)
	}
	for _, invalid := range []CheckoutRequest{
		{QuoteNo: "", IdempotencyKey: "checkout-key-0001"},
		{QuoteNo: " QTE_test_quote", IdempotencyKey: "checkout-key-0001"},
		{QuoteNo: "QTE_test_quote", IdempotencyKey: "short"},
		{QuoteNo: "QTE_test_quote", IdempotencyKey: " checkout-key-0001"},
	} {
		if _, err = normalizeCheckoutRequest("CUS_test_customer", invalid); !bizerr.Is(err, CodeInvalid) {
			t.Fatalf("invalid checkout request=%+v err=%v", invalid, err)
		}
	}
}

func TestCheckoutRequirementsUseOnlyGenericProductInventory(t *testing.T) {
	units, manual, err := checkoutRequirements([]entity.CommerceQuoteItemSnapshot{
		{
			QuoteItemNo: "q1", VersionNo: "PV_phone",
			InventoryDecisionCode: "available", RiskDecisionCode: "approved",
			FulfillmentModeCode: "automatic",
		},
		{
			QuoteItemNo: "q2", VersionNo: "PV_service", InventoryDecisionCode: "available",
			RiskDecisionCode: "approved", FulfillmentModeCode: "manual_review",
		},
	}, map[string]int{"q1": 2, "q2": 3}, map[string]checkoutGenericInventoryFact{
		"q1": {VersionNo: "PV_phone", TrackingEnabled: true, InventoryVersion: 2, SellableQuantity: 2},
		"q2": {VersionNo: "PV_service", InventoryVersion: 1, SellableQuantity: 0},
	})
	if err != nil || units != 5 || !manual {
		t.Fatalf("checkout requirements units=%d manual=%v err=%v", units, manual, err)
	}
	if _, _, err = checkoutRequirements([]entity.CommerceQuoteItemSnapshot{{
		QuoteItemNo: "q1", VersionNo: "PV_phone", InventoryDecisionCode: "manual_confirmation",
	}}, map[string]int{"q1": 1}, map[string]checkoutGenericInventoryFact{
		"q1": {VersionNo: "PV_phone", InventoryVersion: 1, SellableQuantity: 1},
	}); !bizerr.Is(err, CodeQuoteNotAvailable) {
		t.Fatalf("legacy manual inventory decision error=%v", err)
	}
	if _, _, err = checkoutRequirements([]entity.CommerceQuoteItemSnapshot{{
		QuoteItemNo: "q1", VersionNo: "PV_phone",
		InventoryDecisionCode: "available",
	}}, map[string]int{"q1": maximumCheckoutFulfillmentUnits + 1}); !bizerr.Is(err, CodeCapacityExceeded) {
		t.Fatalf("oversized checkout requirements error=%v", err)
	}
	if _, _, err = checkoutRequirements([]entity.CommerceQuoteItemSnapshot{{
		QuoteItemNo: "q1", VersionNo: "PV_phone", InventoryDecisionCode: "available",
	}}, map[string]int{"q1": 1}); !bizerr.Is(err, CodeQuoteNotAvailable) {
		t.Fatalf("missing generic inventory snapshot error=%v", err)
	}
	if _, _, err = checkoutRequirements([]entity.CommerceQuoteItemSnapshot{
		{
			QuoteItemNo: "q1", VersionNo: "PV_trial", ChargeTypeCode: "trial", TrialLimitQuantity: 1,
			InventoryDecisionCode: "available", PurchaseCycles: 1,
		},
		{
			QuoteItemNo: "q2", VersionNo: "PV_paid", ChargeTypeCode: "recurring",
			InventoryDecisionCode: "available",
		},
	}, map[string]int{"q1": 1, "q2": 1}, map[string]checkoutGenericInventoryFact{
		"q1": {VersionNo: "PV_trial", InventoryVersion: 1},
		"q2": {VersionNo: "PV_paid", InventoryVersion: 1},
	}); !bizerr.Is(err, CodeQuoteNotAvailable) {
		t.Fatalf("mixed paid and trial checkout error=%v", err)
	}
}
