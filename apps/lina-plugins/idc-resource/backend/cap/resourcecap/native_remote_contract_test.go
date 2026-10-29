// These tests preserve dynamic pricing as explicit metadata and reject attempts
// to treat a default selection amount as a complete static option price table.
package resourcecap

import (
	"encoding/json"
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func nativeRemoteFixture() *channelcommercecap.ProductSnapshot {
	initial := "3"
	return &channelcommercecap.ProductSnapshot{Product: channelcommercecap.CatalogProduct{PublicSlug: "81"}, ConfigurationMode: "remote_evaluation", PricingReady: true, Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: "USD", Precision: 2}}, Configuration: channelcommercecap.Configuration{ConfigurationRevision: strings.Repeat("a", 64), Fields: []channelcommercecap.ConfigurationField{{FieldToken: "config:10:0", Type: "integer", Visible: true, Required: true, Constraints: channelcommercecap.ConfigurationConstraints{Minimum: "1", Maximum: "10"}, Options: []channelcommercecap.ConfigurationOption{}}}, Values: []channelcommercecap.PurchaseValue{{FieldToken: "config:10:0", Type: "integer", IntegerValue: &initial}}}, BillingTerms: []channelcommercecap.CurrencyBillingTerms{{CurrencyCode: "USD", Precision: 2, Terms: []channelcommercecap.BillingTerm{{BillingTermToken: "3", ChargeType: "recurring", PeriodUnit: "month", PeriodCount: 1, MinimumPurchaseCycles: 1, Status: "available", SetupAmount: "0", SubtotalAmount: "1777", TaxAmount: "0", TotalAmount: "1777"}}}}}
}
func TestNativeRemoteImportDoesNotInventStaticPrices(t *testing.T) {
	view := nativeRemoteFixture()
	snapshot, err := NativeRemoteImportSnapshot(view)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Validate() != nil || snapshot.ProtocolVersion != channelcommercecap.ImportRemoteEvaluationProtocolVersion || snapshot.PricingComplete || snapshot.PricingMode != channelcommercecap.ImportPricingRemoteEvaluation || snapshot.Fulfillment.MaximumPurchaseCycles != 1 || len(snapshot.Prices) != 1 || len(snapshot.Prices[0].Base) != 0 || len(snapshot.Prices[0].Options) != 0 {
		t.Fatalf("dynamic prices became static: %+v", snapshot)
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "1777") {
		t.Fatalf("default configuration amount escaped into price table %s", raw)
	}
	if snapshot.Mappings[0].FieldKey != "cfg_10_0" || snapshot.Mappings[0].FieldToken != "config:10:0" || string(snapshot.Mappings[0].DefaultValue) != "3" {
		t.Fatalf("source quantity initial value was replaced by minimum: %+v", snapshot.Mappings[0])
	}
	for _, scenario := range []string{"hidden", "identity", "slot", "field type", "tax", "term", "revision"} {
		t.Run(scenario, func(t *testing.T) {
			current := nativeRemoteFixture()
			switch scenario {
			case "hidden":
				current.Configuration.Fields[0].Visible = false
			case "identity":
				current.Product.PublicSlug = "source-name"
			case "slot":
				current.Configuration.Fields[0].FieldToken = "config:10:32"
			case "field type":
				current.Configuration.Fields[0].Type = "file"
			case "tax":
				current.BillingTerms[0].Terms[0].TaxAmount = "1"
			case "term":
				current.BillingTerms[0].Terms[0].BillingTermToken = "monthly"
			case "revision":
				current.Configuration.ConfigurationRevision = strings.Repeat("g", 64)
			}
			if _, err := NativeRemoteImportSnapshot(current); err == nil {
				t.Fatal("unsupported native source facts were admitted")
			}
		})
	}
}

func TestNativeRemoteOneTimeImportUsesNewPurchasePhase(t *testing.T) {
	for _, token := range []string{"onetime", "free"} {
		t.Run(token, func(t *testing.T) {
			view := nativeRemoteFixture()
			view.BillingTerms[0].Terms[0] = channelcommercecap.BillingTerm{BillingTermToken: token, ChargeType: "one_time", PeriodUnit: "once", PeriodCount: 1, MinimumPurchaseCycles: 1, Status: "available", SetupAmount: "0", SubtotalAmount: "0", TaxAmount: "0", TotalAmount: "0"}
			snapshot, err := NativeRemoteImportSnapshot(view)
			if err != nil {
				t.Fatal(err)
			}
			schedule := snapshot.Prices[0]
			if snapshot.Validate() != nil || schedule.PricePhase != "new_purchase" || schedule.ChargeType != "one_time" || schedule.PeriodUnit != "once" || schedule.PeriodCount != 1 || schedule.TermReference != token || len(schedule.Base) != 0 || len(schedule.Options) != 0 {
				t.Fatalf("one-time source billing facts changed: %+v", schedule)
			}
		})
	}
}
