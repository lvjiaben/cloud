// This file checks imported whole-item prices across configuration, quantity,
// cycles, tax and setup fees, including fixed markups above ordinary prices.

package catalog

import (
	"math"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func TestImportedWholePriceKeepsSetupAndFixedMarkup(t *testing.T) {
	line := PurchaseQuoteLine{Quantity: 2, PurchaseCycles: 3, Charges: []PurchaseQuoteCharge{
		{ChargeAmountMinor: 300, SetupFeeMinor: 50, PricedAmountMinor: 350},
		{ChargeAmountMinor: 180, SetupFeeMinor: 20, PricedAmountMinor: 200},
	}}
	fixed := int64(100)
	result, err := applyImportedWholePrice(line, channelcommercecap.ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed})
	if err != nil {
		t.Fatal(err)
	}
	if result.Charges[0].ChargeAmountMinor+result.Charges[1].ChargeAmountMinor != 600 || result.Charges[0].SetupFeeMinor != 50 || result.Charges[1].SetupFeeMinor != 20 {
		t.Fatalf("incorrect amount: %+v", result.Charges)
	}
	if line.Charges[0].ChargeAmountMinor != 300 {
		t.Fatal("input mutated")
	}
	fixed = math.MaxInt64
	if _, err = applyImportedWholePrice(line, channelcommercecap.ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed}); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestImportedWholePriceExtractsTaxBeforeDiscount(t *testing.T) {
	rate, bps := 1000, 2000
	for _, scope := range []TaxRoundingScope{TaxRoundingScopeLine, TaxRoundingScopeTotal} {
		line := PurchaseQuoteLine{Quantity: 1, PurchaseCycles: 1, PriceIncludesTax: true, TaxTreatmentCode: string(TaxTreatmentTaxable), TaxRateBasisPoints: &rate,
			RoundingScopeCode: string(scope), RoundingModeCode: string(TaxRoundingHalfUp), RoundingQuantumMinor: 1,
			Charges: []PurchaseQuoteCharge{{ChargeAmountMinor: 11000, SetupFeeMinor: 1100, PricedAmountMinor: 12100}}}
		result, err := applyImportedWholePrice(line, channelcommercecap.ImportWholeItemRule{Kind: "discount", DiscountBasisPoints: &bps})
		if err != nil {
			t.Fatal(err)
		}
		if result.PriceIncludesTax || result.Charges[0].ChargeAmountMinor != 8000 || result.Charges[0].SetupFeeMinor != 1000 {
			t.Fatalf("tax applied twice: %+v", result)
		}
	}
}

func TestImportedWholePriceZeroAndInvalid(t *testing.T) {
	line := PurchaseQuoteLine{Quantity: 1, PurchaseCycles: 1, Charges: []PurchaseQuoteCharge{{}, {}}}
	fixed := int64(5)
	result, err := applyImportedWholePrice(line, channelcommercecap.ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed})
	if err != nil || result.Charges[0].ChargeAmountMinor != 5 {
		t.Fatalf("zero basis markup: %+v %v", result, err)
	}
	if _, err = applyImportedWholePrice(line, channelcommercecap.ImportWholeItemRule{Kind: "fixed"}); err == nil {
		t.Fatal("missing fixed amount accepted")
	}
	for _, weights := range [][]int64{{1, 1, 1}, {0, 2, 1}, {5, 0, 2}} {
		values, err := allocateImportedAmount(10, weights)
		if err != nil {
			t.Fatal(err)
		}
		var total int64
		for _, value := range values {
			total += value
		}
		if total != 10 {
			t.Fatal("allocation lost minor units")
		}
	}
}

// TestImportedWholePriceResaleChain verifies exact per-hop rounding rather than
// collapsing percentages, and replacement of a prior fixed price.
func TestImportedWholePriceResaleChain(t *testing.T) {
	bps := 3333
	line := PurchaseQuoteLine{Quantity: 1, PurchaseCycles: 1, Charges: []PurchaseQuoteCharge{{ChargeAmountMinor: 7, SetupFeeMinor: 2, PricedAmountMinor: 9}}}
	rule := channelcommercecap.ImportWholeItemRule{Kind: "discount", DiscountBasisPoints: &bps, Prior: &channelcommercecap.ImportWholeItemRule{Kind: "discount", DiscountBasisPoints: &bps}}
	result, err := applyImportedWholePrice(line, rule)
	if err != nil || result.Charges[0].ChargeAmountMinor != 2 || result.Charges[0].SetupFeeMinor != 2 {
		t.Fatalf("rounding chain: %+v %v", result, err)
	}
	fixed := int64(100)
	rule.Prior = &channelcommercecap.ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed}
	result, err = applyImportedWholePrice(line, rule)
	if err != nil || result.Charges[0].ChargeAmountMinor != 66 {
		t.Fatalf("discount after fixed: %+v %v", result, err)
	}
	replacement := int64(120)
	rule.Kind, rule.DiscountBasisPoints, rule.FixedAmountMinor = "fixed", nil, &replacement
	result, err = applyImportedWholePrice(line, rule)
	if err != nil || result.Charges[0].ChargeAmountMinor != 120 {
		t.Fatalf("fixed override: %+v %v", result, err)
	}
	rule.Prior = &rule
	if _, err = applyImportedWholePrice(line, rule); err == nil {
		t.Fatal("cyclic rule accepted")
	}
}

// TestImportedWholePriceTaxRemainderMatchesCheckout guards the checkout's
// setup-before-period tie order when extracting one minor unit of tax.
func TestImportedWholePriceTaxRemainderMatchesCheckout(t *testing.T) {
	rate, zero := 10000, 0
	line := PurchaseQuoteLine{Quantity: 1, PurchaseCycles: 1, PriceIncludesTax: true, TaxTreatmentCode: string(TaxTreatmentTaxable), TaxRateBasisPoints: &rate, RoundingScopeCode: string(TaxRoundingScopeLine), RoundingModeCode: string(TaxRoundingHalfUp), RoundingQuantumMinor: 1, Charges: []PurchaseQuoteCharge{{ChargeAmountMinor: 1, SetupFeeMinor: 1, PricedAmountMinor: 2}}}
	result, err := applyImportedWholePrice(line, channelcommercecap.ImportWholeItemRule{Kind: "discount", DiscountBasisPoints: &zero})
	if err != nil || result.Charges[0].ChargeAmountMinor != 1 || result.Charges[0].SetupFeeMinor != 0 {
		t.Fatalf("tax remainder differs from checkout: %+v %v", result, err)
	}
}
