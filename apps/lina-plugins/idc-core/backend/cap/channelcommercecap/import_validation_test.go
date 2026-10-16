// This file verifies bounded resale rule validation before JSON serialization.

package channelcommercecap

import "testing"

func TestImportWholeItemRuleBoundAndCycle(t *testing.T) {
	fixed := int64(7)
	var rule *ImportWholeItemRule
	for i := 0; i < MaximumImportWholeItemRuleDepth; i++ {
		rule = &ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed, Prior: rule}
	}
	if err := rule.Validate(); err != nil {
		t.Fatal(err)
	}
	rule = &ImportWholeItemRule{Kind: "fixed", FixedAmountMinor: &fixed, Prior: rule}
	if rule.Validate() == nil {
		t.Fatal("excessive resale depth accepted")
	}
	rule.Prior = rule
	if rule.Validate() == nil {
		t.Fatal("cyclic resale rule accepted")
	}
	zero := 0
	rule.Prior = &ImportWholeItemRule{Kind: "discount", DiscountBasisPoints: &zero}
	if err := rule.Validate(); err != nil {
		t.Fatal(err)
	}
	rule.Prior.FixedAmountMinor = &fixed
	if rule.Validate() == nil {
		t.Fatal("ambiguous prior rule accepted")
	}
}

// TestImportQuantityPriceBands rejects negative setup amounts even when each
// quantity band has an independent one-time charge.
func TestImportQuantityPriceBands(t *testing.T) {
	maximum := int64(100)
	for _, mode := range []ImportConfigurationPriceMode{ImportConfigurationPricePerUnit, ImportConfigurationPriceBandTotal} {
		t.Run(string(mode), func(t *testing.T) {
			price := ImportPriceSchedule{CurrencyCode: "USD", TermReference: "term-demo", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1,
				Base: []ImportPriceBand{{AmountMinor: 100}}, Options: []ImportOptionPrice{{FieldKey: "disk", PricingMode: mode, Bands: []ImportPriceBand{{MaximumQuantity: &maximum, AmountMinor: 2, SetupAmountMinor: 3}, {MinimumQuantity: 100, AmountMinor: 1, SetupAmountMinor: 5}}}}}
			if err := price.validateImportSchedule(); err != nil {
				t.Fatal(err)
			}
			price.Options[0].Bands[1].SetupAmountMinor = -1
			if price.validateImportSchedule() == nil {
				t.Fatal("negative setup amount accepted")
			}
			price.Options[0].Bands[1].SetupAmountMinor = 5
			price.Options[0].Bands[1].MinimumQuantity = 101
			if price.validateImportSchedule() == nil {
				t.Fatal("quantity band gap accepted")
			}
			price.Options[0].Bands[1].MinimumQuantity = 100
			price.Options[0].Value = "quantity"
			if price.validateImportSchedule() == nil {
				t.Fatal("quantity field with option value accepted")
			}
		})
	}
}
