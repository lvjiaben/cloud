// This file verifies bounded resale rule validation before JSON serialization.

package channelcommercecap

import (
	"encoding/json"
	"lina-core/pkg/plugin/capability/capmodel"
	"testing"
)

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

// TestImportGraduatedQuantityRetainsFirstSetup requires actual marginal bands
// and refuses to normalize a later setup fee away during transport validation.
func TestImportGraduatedQuantityRetainsFirstSetup(t *testing.T) {
	maximum := int64(10)
	price := ImportPriceSchedule{CurrencyCode: "USD", TermReference: "term-demo", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1,
		Base: []ImportPriceBand{{AmountMinor: 100}}, Options: []ImportOptionPrice{{FieldKey: "disk", PricingMode: ImportConfigurationPriceGraduated, Bands: []ImportPriceBand{{MaximumQuantity: &maximum, AmountMinor: 0, SetupAmountMinor: 7}, {MinimumQuantity: 10, AmountMinor: 2}}}}}
	if err := price.validateImportSchedule(); err != nil {
		t.Fatal(err)
	}
	if price.Options[0].Bands[0].SetupAmountMinor != 7 {
		t.Fatal("import mutated setup fee")
	}
	price.Options[0].Bands[1].SetupAmountMinor = 1
	if price.validateImportSchedule() == nil {
		t.Fatal("graduated import accepted repeated setup fee")
	}
}

// TestImportFulfillmentCycleLimit preserves old snapshots while bounding source
// limits that every downstream seller must retain before accepting payment.
func TestImportFulfillmentCycleLimit(t *testing.T) {
	f := ImportFulfillment{Granularity: "per_order_item", RenewalMode: "unavailable"}
	for _, maximum := range []int{0, 1, 120} {
		f.MaximumPurchaseCycles = maximum
		if err := f.validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, maximum := range []int{-1, 121} {
		f.MaximumPurchaseCycles = maximum
		if f.validate() == nil {
			t.Fatalf("invalid cycle maximum %d accepted", maximum)
		}
	}
}

// TestRestrictedImportProtocolCannotLoseItsLimit verifies a down-level
// projection cannot turn a v2 restricted source into an unrestricted sale.
func TestRestrictedImportProtocolCannotLoseItsLimit(t *testing.T) {
	if !validImportSnapshotProtocol(ImportProtocolVersion, 0, nil) || !validImportSnapshotProtocol(ImportPurchaseCycleLimitProtocolVersion, 1, nil) {
		t.Fatal("valid import protocol rejected")
	}
	if validImportSnapshotProtocol(ImportProtocolVersion, 1, nil) || validImportSnapshotProtocol(ImportPurchaseCycleLimitProtocolVersion, 0, nil) {
		t.Fatal("discarded cycle ceiling accepted")
	}
}

func TestImportCustomerPolicyCannotBeStripped(t *testing.T) {
	hidden := false
	mappings := []ImportFieldMapping{{CustomerVisible: &hidden, DefaultValue: json.RawMessage(`0`)}}
	if validImportSnapshotProtocol(ImportProtocolVersion, 0, mappings) || !validImportSnapshotProtocol(ImportPurchaseCycleLimitProtocolVersion, 0, mappings) {
		t.Fatal("visibility capability fence failed")
	}
	for _, scenario := range []struct {
		kind, raw string
		valid     bool
	}{{capmodel.ConfigurationFieldInteger, "0", true}, {capmodel.ConfigurationFieldBoolean, "false", true}, {capmodel.ConfigurationFieldInteger, `"0"`, false}, {capmodel.ConfigurationFieldSecret, `"private"`, false}, {capmodel.ConfigurationFieldFile, `[]`, false}, {capmodel.ConfigurationFieldSingleLine, `{}`, false}, {capmodel.ConfigurationFieldSingleLine, `null`, false}} {
		if err := validateImportDefault(&capmodel.ConfigurationFieldDeclaration{Type: scenario.kind}, ImportFieldMapping{CustomerVisible: &hidden, DefaultValue: json.RawMessage(scenario.raw)}); (err == nil) != scenario.valid {
			t.Fatalf("%s %s: %v", scenario.kind, scenario.raw, err)
		}
	}
	if validateImportDefault(&capmodel.ConfigurationFieldDeclaration{Type: capmodel.ConfigurationFieldInteger}, ImportFieldMapping{CustomerVisible: &hidden}) == nil {
		t.Fatal("hidden missing default accepted")
	}
}
