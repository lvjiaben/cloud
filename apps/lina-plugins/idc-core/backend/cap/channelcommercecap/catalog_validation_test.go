// These tests reject incomplete or misleading upstream import contracts.
package channelcommercecap

import "testing"

func TestBillingTermsRejectInvalidPeriodAndCurrency(t *testing.T) {
	valid := BillingTerms{ConfigurationRevision: "revision", CurrencyCode: "CNY", Terms: []BillingTerm{{BillingTermToken: "term", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, SetupAmount: "0", SubtotalAmount: "100", TaxAmount: "0", TotalAmount: "100"}}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*BillingTerms){func(v *BillingTerms) { v.Terms[0].PeriodCount = 0 }, func(v *BillingTerms) { v.Terms[0].PeriodUnit = "fortnight" }, func(v *BillingTerms) { v.Terms[0].ChargeType = "guessed" }, func(v *BillingTerms) { v.CurrencyCode = "" }} {
		candidate := valid
		candidate.Terms = append([]BillingTerm(nil), valid.Terms...)
		change(&candidate)
		if candidate.Validate() == nil {
			t.Fatal("invalid billing meaning accepted")
		}
	}
}

func TestProductSnapshotDoesNotInventPriceOrStaticRules(t *testing.T) {
	snapshot := ProductSnapshot{Product: CatalogProduct{PublicSlug: "cloud"}, Configuration: Configuration{ConfigurationRevision: "revision", Fields: []ConfigurationField{}, Values: []PurchaseValue{}}, ConfigurationMode: "remote_evaluation", Currencies: []CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	snapshot.PricingReady = true
	if snapshot.Validate() == nil {
		t.Fatal("missing price declared ready")
	}
	snapshot.PricingReady = false
	snapshot.ConfigurationMode = "static"
	if snapshot.Validate() == nil {
		t.Fatal("evaluated view declared static schema")
	}
	snapshot.ConfigurationMode = "remote_evaluation"
	snapshot.Configuration.Fields = []ConfigurationField{{FieldToken: "secret", Type: "secret"}}
	if snapshot.Validate() == nil {
		t.Fatal("secret schema escaped import boundary")
	}
}
