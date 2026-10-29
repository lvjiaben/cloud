package channelcommercecap

import "testing"

func TestRemoteImportVersionRejectsPriceFallback(t *testing.T) {
	snapshot := ImportSnapshot{ProtocolVersion: ImportRemoteEvaluationProtocolVersion, PricingMode: ImportPricingRemoteEvaluation, Product: CatalogProduct{PublicSlug: "native"}, ConfigurationRevision: "rev", Currencies: []CatalogCurrency{{CurrencyCode: "CNY", Precision: 2}}, Prices: []ImportPriceSchedule{{CurrencyCode: "CNY", TermReference: "monthly", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, QuantityMode: "fixed", PricePhase: "both"}}, Fulfillment: ImportFulfillment{Granularity: "per_quantity_unit", OngoingService: true, RenewalMode: "unavailable", MaximumPurchaseCycles: 1}}
	if err := snapshot.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*ImportSnapshot){func(v *ImportSnapshot) { v.PricingMode = "" }, func(v *ImportSnapshot) { v.PricingComplete = true }, func(v *ImportSnapshot) {
		v.Prices = append([]ImportPriceSchedule(nil), v.Prices...)
		v.Prices[0].Base = []ImportPriceBand{{}}
	}, func(v *ImportSnapshot) { v.ProtocolVersion = ImportProtocolVersion }} {
		changed := snapshot
		mutate(&changed)
		if changed.Validate() == nil {
			t.Fatal("remote import accepted stripped mode or fabricated static price")
		}
	}
}
