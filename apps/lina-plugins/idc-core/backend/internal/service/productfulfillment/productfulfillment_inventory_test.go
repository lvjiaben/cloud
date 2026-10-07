package productfulfillment

import (
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

func TestValidateFrozenItemRejectsLegacyManualInventoryDecision(t *testing.T) {
	authority := &settlementAuthority{
		OrderNo: "ord_inventory", PayerCustomerNo: "cus_inventory", ServiceOwnerCustomerNo: "cus_inventory",
	}
	item := &frozenItem{
		OrderNo: "ord_inventory", CustomerNo: "cus_inventory", Quantity: 1,
		QuantityDigest: strings.Repeat("1", 64), PolicyDigest: strings.Repeat("2", 64),
		MaximumAttempts: maximumQueryAttempts, RetryBaseSeconds: retryIntervalSeconds,
		InventoryDecisionCode: "available", GranularityCode: GranularityPerOrderItem,
		StrategyCode: StrategyManual, RenewalMode: productfulfillmentcap.RenewalModeUnavailable,
	}
	if err := validateFrozenItem(item, authority); err != nil {
		t.Fatalf("available inventory evidence rejected: %v", err)
	}
	item.InventoryDecisionCode = "manual_confirmation"
	if err := validateFrozenItem(item, authority); err == nil {
		t.Fatal("legacy manual inventory evidence accepted")
	}
}
