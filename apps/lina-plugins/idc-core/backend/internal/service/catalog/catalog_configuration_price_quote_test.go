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
	for _, mode := range []ConfigurationPriceMode{ConfigurationPricePerUnit, ConfigurationPriceBandTotal} {
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
