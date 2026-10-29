// This file verifies independent configuration quantity billing, including
// band boundaries, renewal amounts, zero selection and integer overflow.

package catalog

import (
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"math"
	"testing"
)

func TestConfigurationPriceQuantityModes(t *testing.T) {
	rule := BillingRuleDefinition{ChargeType: ChargeTypeRecurring, PeriodUnit: PeriodUnitMonth, PeriodCount: 1, PricePhase: PricePhaseBoth, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1}
	for _, scenario := range []struct {
		name          string
		mode          ConfigurationPriceMode
		quantity      int64
		phase         QuotePhase
		cycles        int
		charge, setup int64
	}{
		{"unit", ConfigurationPricePerUnit, 100, QuotePhaseNewPurchase, 1, 2000, 7},
		{"band", ConfigurationPriceBandTotal, 100, QuotePhaseNewPurchase, 1, 20, 7},
		{"next band unit", ConfigurationPricePerUnit, 101, QuotePhaseNewPurchase, 1, 1010, 9},
		{"next band total", ConfigurationPriceBandTotal, 101, QuotePhaseNewPurchase, 1, 10, 9},
		{"renew unit", ConfigurationPricePerUnit, 100, QuotePhaseRenewal, 2, 3000, 0},
		{"renew band", ConfigurationPriceBandTotal, 100, QuotePhaseRenewal, 2, 30, 0},
		{"zero unit", ConfigurationPricePerUnit, 0, QuotePhaseNewPurchase, 1, 0, 0},
		{"zero band", ConfigurationPriceBandTotal, 0, QuotePhaseNewPurchase, 1, 0, 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			tiers := []PriceTierDefinition{{MinimumQuantity: 100, AmountMinor: 10, RenewalAmountMinor: int64Pointer(8), SetupFeeMinor: 9}, {MinimumQuantity: 0, MaximumQuantity: int64Pointer(100), AmountMinor: 20, RenewalAmountMinor: int64Pointer(15), SetupFeeMinor: 7}}
			got, err := quoteConfigurationPrice(rule, scenario.mode, tiers, BillingQuoteRequest{Phase: scenario.phase, Cycles: scenario.cycles, Quantity: &scenario.quantity})
			if err != nil {
				t.Fatal(err)
			}
			if *got.ChargeAmountMinor != scenario.charge || *got.SetupFeeMinor != scenario.setup || *got.TotalAmountMinor != scenario.charge+scenario.setup {
				t.Fatalf("unexpected quote: %+v", got)
			}
			if *got.BilledQuantity != scenario.quantity {
				t.Fatal("quantity lost")
			}
			if tiers[0].MinimumQuantity != 100 || tiers[1].MaximumQuantity == nil {
				t.Fatal("input schedule mutated")
			}
		})
	}
}

func TestConfigurationPriceRejectsInvalidSchedulesAndOverflow(t *testing.T) {
	rule := BillingRuleDefinition{ChargeType: ChargeTypeRecurring, PeriodUnit: PeriodUnitMonth, PeriodCount: 1, PricePhase: PricePhaseBoth, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1}
	for _, scenario := range []struct {
		name     string
		mode     ConfigurationPriceMode
		quantity int64
		tiers    []PriceTierDefinition
	}{
		{"invalid mode", ConfigurationPriceMode("unknown"), 1, []PriceTierDefinition{{RenewalAmountMinor: int64Pointer(0)}}},
		{"negative quantity", ConfigurationPricePerUnit, -1, []PriceTierDefinition{{RenewalAmountMinor: int64Pointer(0)}}},
		{"missing renewal", ConfigurationPricePerUnit, 1, []PriceTierDefinition{{AmountMinor: 20}}},
		{"overflow", ConfigurationPricePerUnit, 2, []PriceTierDefinition{{AmountMinor: math.MaxInt64, RenewalAmountMinor: int64Pointer(0)}}},
		{"gap", ConfigurationPriceBandTotal, 100, []PriceTierDefinition{{MaximumQuantity: int64Pointer(50), RenewalAmountMinor: int64Pointer(0)}, {MinimumQuantity: 60, RenewalAmountMinor: int64Pointer(0)}}},
		{"overlap", ConfigurationPricePerUnit, 100, []PriceTierDefinition{{MaximumQuantity: int64Pointer(70), RenewalAmountMinor: int64Pointer(0)}, {MinimumQuantity: 60, RenewalAmountMinor: int64Pointer(0)}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			if _, err := quoteConfigurationPrice(rule, scenario.mode, scenario.tiers, BillingQuoteRequest{Phase: QuotePhaseNewPurchase, Cycles: 1, Quantity: &scenario.quantity}); err == nil {
				t.Fatal("invalid quote accepted")
			}
		})
	}
}

func TestConfigurationSelectionPreservesQuantityAndAlgorithm(t *testing.T) {
	for _, mode := range []ConfigurationPriceMode{ConfigurationPricePerUnit, ConfigurationPriceBandTotal, ConfigurationPriceGraduated} {
		t.Run(string(mode), func(t *testing.T) {
			rows := []configurationPriceDimensionIdentity{{DimensionNo: "cpd_11111111111111111111111111111111", FieldKey: "disk", CandidateValue: "quantity", PricingMode: mode}}
			for _, quantity := range []int64{0, 100, 129, 2147483647} {
				selections, err := selectConfigurationPriceDimensions(rows, []configurationcontract.ConfigurationValue{{FieldKey: "disk", IntegerValue: &quantity}})
				if err != nil {
					t.Fatal(err)
				}
				if quantity == 0 {
					if len(selections) != 0 {
						t.Fatal("zero selection charged")
					}
					continue
				}
				if len(selections) != 1 || int64(selections[0].Quantity) != quantity || selections[0].PricingMode != mode {
					t.Fatalf("selection lost: %+v", selections)
				}
				if _, err = ConfigurationPriceSelectionDigest(selections); err != nil {
					t.Fatal(err)
				}
			}
			excessive := int64(2147483648)
			if _, err := selectConfigurationPriceDimensions(rows, []configurationcontract.ConfigurationValue{{FieldKey: "disk", IntegerValue: &excessive}}); err == nil {
				t.Fatal("SQL integer overflow allowed")
			}
		})
	}
	fixed := []ConfigurationPriceSelection{{DimensionNo: "cpd_11111111111111111111111111111111", Quantity: 100}}
	legacy, err := ConfigurationPriceSelectionDigest(fixed)
	if err != nil {
		t.Fatal(err)
	}
	fixed[0].PricingMode = ConfigurationPriceFixed
	compatible, err := ConfigurationPriceSelectionDigest(fixed)
	if err != nil || compatible != legacy {
		t.Fatal("legacy fixed digest changed")
	}
	fixed[0].PricingMode = ConfigurationPricePerUnit
	unit, err := ConfigurationPriceSelectionDigest(fixed)
	if err != nil || unit == legacy {
		t.Fatal("algorithm not bound to digest")
	}
	fixed[0].PricingMode = ConfigurationPriceBandTotal
	band, err := ConfigurationPriceSelectionDigest(fixed)
	if err != nil || band == unit {
		t.Fatal("different algorithms share digest")
	}
}

// TestConfigurationGraduatedPrice proves actual marginal quantities rather than
// silently applying a band's rate to the whole selection. Old modes stay above.
func TestConfigurationGraduatedPrice(t *testing.T) {
	rule := BillingRuleDefinition{ChargeType: ChargeTypeRecurring, PeriodUnit: PeriodUnitMonth, PeriodCount: 1, PricePhase: PricePhaseBoth, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1}
	tiers := []PriceTierDefinition{
		{MinimumQuantity: 20, AmountMinor: 100, RenewalAmountMinor: int64Pointer(150)},
		{MaximumQuantity: int64Pointer(10), AmountMinor: 0, RenewalAmountMinor: int64Pointer(0), SetupFeeMinor: 7},
		{MinimumQuantity: 10, MaximumQuantity: int64Pointer(20), AmountMinor: 200, RenewalAmountMinor: int64Pointer(300)},
	}
	for _, scenario := range []struct {
		quantity      int64
		phase         QuotePhase
		cycles        int
		amount, setup int64
		segments      int
	}{
		{0, QuotePhaseNewPurchase, 1, 0, 0, 0},
		{10, QuotePhaseNewPurchase, 1, 0, 7, 1},
		{15, QuotePhaseNewPurchase, 1, 1000, 7, 2},
		{20, QuotePhaseNewPurchase, 1, 2000, 7, 2},
		{25, QuotePhaseNewPurchase, 1, 2500, 7, 3},
		{25, QuotePhaseRenewal, 2, 7500, 0, 3},
	} {
		got, err := quoteConfigurationPrice(rule, ConfigurationPriceGraduated, tiers, BillingQuoteRequest{Phase: scenario.phase, Cycles: scenario.cycles, Quantity: &scenario.quantity})
		if err != nil {
			t.Fatal(err)
		}
		if *got.ChargeAmountMinor != scenario.amount || *got.SetupFeeMinor != scenario.setup || *got.TotalAmountMinor != scenario.amount+scenario.setup || *got.InputQuantity != scenario.quantity || *got.BilledQuantity != scenario.quantity || len(got.Segments) != scenario.segments {
			t.Fatalf("quantity=%d phase=%s quote=%+v", scenario.quantity, scenario.phase, got)
		}
	}
	if tiers[0].MinimumQuantity != 20 || tiers[1].SetupFeeMinor != 7 {
		t.Fatal("graduated quote mutated input")
	}
	invalid := append([]PriceTierDefinition(nil), tiers...)
	invalid[0].SetupFeeMinor = 1
	if err := validateConfigurationPriceSchedule(rule, ConfigurationPriceGraduated, invalid); err == nil {
		t.Fatal("non-first setup fee accepted")
	}
	quantity := int64(2)
	if _, err := quoteConfigurationPrice(rule, ConfigurationPriceGraduated, []PriceTierDefinition{{AmountMinor: math.MaxInt64, RenewalAmountMinor: int64Pointer(0)}}, BillingQuoteRequest{Phase: QuotePhaseNewPurchase, Cycles: 1, Quantity: &quantity}); err == nil {
		t.Fatal("marginal overflow accepted")
	}
	// The same configuration can be used with an ordinary one-time product;
	// its base billing model must not be turned into a recurring tiered product.
	once := BillingRuleDefinition{ChargeType: ChargeTypeOneTime, PeriodUnit: PeriodUnitOnce, PeriodCount: 1, PricePhase: PricePhaseNewPurchase, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1}
	onceTiers := []PriceTierDefinition{{MaximumQuantity: int64Pointer(10), SetupFeeMinor: 5}, {MinimumQuantity: 10, AmountMinor: 200}}
	quantity = 15
	got, err := quoteConfigurationPrice(once, ConfigurationPriceGraduated, onceTiers, BillingQuoteRequest{Phase: QuotePhaseNewPurchase, Cycles: 1, Quantity: &quantity})
	if err != nil || *got.TotalAmountMinor != 1005 {
		t.Fatalf("one-time graduated quote=%+v err=%v", got, err)
	}
}
