package commerce

import (
	"math"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestCalculateQuoteItemAmountsExtractsInclusiveTaxThenDiscountsNet(t *testing.T) {
	rate := 2_000
	line := catalog.PurchaseQuoteLine{
		PeriodUnitCode: string(catalog.PeriodUnitMonth), PriceIncludesTax: true,
		TaxTreatmentCode: string(catalog.TaxTreatmentTaxable), TaxRateBasisPoints: &rate,
		RoundingScopeCode: string(catalog.TaxRoundingScopeLine),
		RoundingModeCode:  string(catalog.TaxRoundingHalfUp), RoundingQuantumMinor: 1,
		Charges: []catalog.PurchaseQuoteCharge{
			{ChargeAmountMinor: 12_000, SetupFeeMinor: 1_200, PricedAmountMinor: 13_200},
			{ChargeAmountMinor: 6_000, PricedAmountMinor: 6_000},
		},
	}
	amounts, err := calculateQuoteItemAmounts(line, 1_600)
	if err != nil {
		t.Fatalf("calculate inclusive quote amounts: %v", err)
	}
	if amounts.setupSubtotal != 1_000 || amounts.periodSubtotal != 15_000 ||
		amounts.subtotal != 16_000 || amounts.discount != 1_600 || amounts.net != 14_400 ||
		amounts.tax != 2_880 || amounts.total != 17_280 {
		t.Fatalf("unexpected inclusive quote amounts: %+v", amounts)
	}
}

func TestCalculateQuoteItemAmountsUsesOneTotalTaxRounding(t *testing.T) {
	rate := 700
	line := catalog.PurchaseQuoteLine{
		PeriodUnitCode: string(catalog.PeriodUnitMonth), PriceIncludesTax: false,
		TaxTreatmentCode: string(catalog.TaxTreatmentTaxable), TaxRateBasisPoints: &rate,
		RoundingScopeCode: string(catalog.TaxRoundingScopeTotal),
		RoundingModeCode:  string(catalog.TaxRoundingHalfUp), RoundingQuantumMinor: 5,
		Charges: []catalog.PurchaseQuoteCharge{
			{ChargeAmountMinor: 101, SetupFeeMinor: 52, PricedAmountMinor: 153},
			{ChargeAmountMinor: 203, PricedAmountMinor: 203},
		},
	}
	amounts, err := calculateQuoteItemAmounts(line, 17)
	if err != nil {
		t.Fatalf("calculate total-rounded quote amounts: %v", err)
	}
	if amounts.subtotal != 356 || amounts.net != 339 || amounts.tax != 25 || amounts.total != 364 {
		t.Fatalf("unexpected total-rounded amounts: %+v", amounts)
	}
}

func TestQuoteAmountAllocationAndOverflowFailClosed(t *testing.T) {
	allocated, err := allocateQuoteAmount(7, []int64{5, 5, 5})
	if err != nil || len(allocated) != 3 || allocated[0] != 3 || allocated[1] != 2 || allocated[2] != 2 {
		t.Fatalf("stable largest-remainder allocation=%v err=%v", allocated, err)
	}
	if _, err = safeQuoteAmountAdd(math.MaxInt64, 1); err == nil {
		t.Fatal("integer amount overflow was accepted")
	}
	if _, err = allocateQuoteAmount(2, []int64{1}); err == nil {
		t.Fatal("over-allocation was accepted")
	}
}
