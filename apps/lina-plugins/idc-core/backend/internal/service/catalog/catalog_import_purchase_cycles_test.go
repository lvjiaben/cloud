// This file verifies source cycle restrictions without narrowing ordinary or
// same-system billing, including mixed checkout batches.
package catalog

import (
	"testing"
)

func TestNativeImportedPurchaseCycles(t *testing.T) {
	limits := map[string]int{"native": 1}
	if err := validateImportedPurchaseCycleLimits([]PurchaseQuoteLine{{VersionNo: "native", PurchaseCycles: 1}, {VersionNo: "ordinary", PurchaseCycles: 12}}, limits); err != nil {
		t.Fatal(err)
	}
	for _, cycles := range []int{0, 2, 12} {
		if err := validateImportedPurchaseCycleLimits([]PurchaseQuoteLine{{VersionNo: "native", PurchaseCycles: cycles}}, limits); err == nil {
			t.Fatalf("native cycles %d accepted", cycles)
		}
	}
}
