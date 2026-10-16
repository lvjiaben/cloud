// This file verifies formal conversion uses current purchase prices, excludes
// every setup fee, and refuses stopped, replaced, or non-recurring sale terms.

package catalog

import (
	"math"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestTrialConversionPricesCurrentPurchaseWithoutSetup(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		change      func(*purchaseQuoteProductRow, *entity.CatalogBillingRule)
		wantFailure bool
	}{
		{name: "current monthly"},
		{name: "new purchase only", change: func(_ *purchaseQuoteProductRow, r *entity.CatalogBillingRule) {
			r.PricePhaseCode = string(PricePhaseNewPurchase)
		}},
		{name: "stopped", wantFailure: true, change: func(p *purchaseQuoteProductRow, _ *entity.CatalogBillingRule) { p.ProductStatusCode = "suspended" }},
		{name: "category stopped", wantFailure: true, change: func(p *purchaseQuoteProductRow, _ *entity.CatalogBillingRule) { p.CategoryStatusCode = "inactive" }},
		{name: "replaced", wantFailure: true, change: func(p *purchaseQuoteProductRow, _ *entity.CatalogBillingRule) {
			version := "new-version"
			p.CurrentVersionNo = &version
		}},
		{name: "no current version", wantFailure: true, change: func(p *purchaseQuoteProductRow, _ *entity.CatalogBillingRule) { p.CurrentVersionNo = nil }},
		{name: "trial", wantFailure: true, change: func(_ *purchaseQuoteProductRow, r *entity.CatalogBillingRule) {
			r.ChargeTypeCode = string(ChargeTypeTrial)
		}},
		{name: "one time", wantFailure: true, change: func(_ *purchaseQuoteProductRow, r *entity.CatalogBillingRule) {
			r.ChargeTypeCode = string(ChargeTypeOneTime)
		}},
		{name: "renewal only", wantFailure: true, change: func(_ *purchaseQuoteProductRow, r *entity.CatalogBillingRule) {
			r.PricePhaseCode = string(PricePhaseRenewal)
		}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			current := "version-1"
			dimension := "dimension-1"
			renewal := int64(7000)
			product := purchaseQuoteProductRow{
				ProductNo: "product-1", VersionNo: current, CurrentVersionNo: &current,
				ProductStatusCode: "active", CategoryStatusCode: "active", VersionStatusCode: "published",
			}
			rule := entity.CatalogBillingRule{
				VersionNo: current, BillingRuleNo: "monthly", ChargeTypeCode: string(ChargeTypeRecurring),
				PeriodUnitCode: string(PeriodUnitMonth), PeriodCount: 1, PricePhaseCode: string(PricePhaseBoth),
				QuantityModeCode: string(QuantityModeFixed), MinimumPurchaseCycles: 1, StatusCode: "active",
			}
			if testCase.change != nil {
				testCase.change(&product, &rule)
			}
			input := RenewalQuoteInput{CurrencyCode: "CNY", TrialConversion: true, Lines: []RenewalQuoteLineInput{{
				ReferenceNo: "entitlement-1", ProductNo: "product-1", VersionNo: current, BillingRuleNo: "monthly",
				Quantity: 1, PurchaseCycles: 2, PriceSelections: []ConfigurationPriceSelection{{DimensionNo: dimension, Quantity: 1}},
			}}}
			prices := []entity.CatalogPrice{
				{VersionNo: current, BillingRuleNo: "monthly", CurrencyCode: "CNY", AmountMinor: 10000, SetupFeeMinor: 2000, RenewalAmountMinor: &renewal, StatusCode: "active"},
				{VersionNo: current, BillingRuleNo: "monthly", OptionNo: &dimension, CurrencyCode: "CNY", AmountMinor: 2000, SetupFeeMinor: 500, RenewalAmountMinor: &renewal, StatusCode: "active"},
			}
			if rule.PricePhaseCode == string(PricePhaseNewPurchase) {
				for index := range prices {
					prices[index].RenewalAmountMinor = nil
				}
			}
			taxes := []entity.CatalogTaxPolicy{{TaxPolicyNo: "tax-1", VersionNo: current, CurrencyCode: "CNY",
				TaxTreatmentCode: string(TaxTreatmentExempt), TaxCategoryCode: "cloud", RoundingScopeCode: string(TaxRoundingScopeLine),
				RoundingModeCode: string(TaxRoundingHalfUp), RoundingQuantumMinor: 1}}
			lines, err := buildRenewalQuote(input, []purchaseQuoteProductRow{product}, []entity.CatalogBillingRule{rule}, prices, taxes)
			if testCase.wantFailure {
				if !bizerr.Is(err, CodePublicQuoteNotAvailable) {
					t.Fatalf("expected sale rejection: %v", err)
				}
				return
			}
			if err != nil || len(lines) != 1 || len(lines[0].Charges) != 2 {
				t.Fatalf("conversion quote: %#v %v", lines, err)
			}
			for index, want := range []int64{20000, 4000} {
				charge := lines[0].Charges[index]
				if charge.SetupFeeMinor != 0 || charge.ChargeAmountMinor != want || charge.PricedAmountMinor != want {
					t.Fatalf("purchase amount or setup waiver incorrect: %#v", charge)
				}
			}
			if lines[0].InventoryDecisionCode != "not_required" || prices[0].SetupFeeMinor != 2000 || prices[1].SetupFeeMinor != 500 {
				t.Fatal("conversion consumed inventory or changed sale prices")
			}
		})
	}
}

func TestTrialConversionWaivesSetupBeforeOverflowCheck(t *testing.T) {
	current := "version-1"
	line := RenewalQuoteLineInput{ReferenceNo: "entitlement-1", ProductNo: "product-1", VersionNo: current, BillingRuleNo: "monthly", Quantity: 1, PurchaseCycles: 1}
	rule := entity.CatalogBillingRule{VersionNo: current, BillingRuleNo: "monthly", ChargeTypeCode: string(ChargeTypeRecurring), PeriodUnitCode: string(PeriodUnitMonth), PeriodCount: 1, PricePhaseCode: string(PricePhaseNewPurchase), QuantityModeCode: string(QuantityModeFixed), MinimumPurchaseCycles: 1, StatusCode: "active"}
	prices := map[string][]entity.CatalogPrice{purchaseQuotePriceKey(current, "monthly", ""): {
		{VersionNo: current, BillingRuleNo: "monthly", CurrencyCode: "CNY", AmountMinor: 100, SetupFeeMinor: math.MaxInt64, StatusCode: "active"},
	}}
	charges, err := trialConversionQuoteCharges(line, rule, prices)
	if err != nil || len(charges) != 1 || charges[0].PricedAmountMinor != 100 || charges[0].SetupFeeMinor != 0 {
		t.Fatalf("waived setup incorrectly affected formal price: %#v %v", charges, err)
	}
	if prices[purchaseQuotePriceKey(current, "monthly", "")][0].SetupFeeMinor != math.MaxInt64 {
		t.Fatal("formal conversion changed the catalog setup fee")
	}
}
