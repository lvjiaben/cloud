// This file proves upgrade accounting excludes the discounted installation
// portion rather than subtracting the original installation fee from net.

package commerce

import (
	"strings"
	"testing"

	"lina-plugin-idc-core/backend/internal/service/catalog"
)

func TestCalculatePeriodicNetAmountWithAdjustmentsMatchesPaidQuote(t *testing.T) {
	line := catalog.PurchaseQuoteLine{PeriodUnitCode: "month", TaxTreatmentCode: "exempt", Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", ChargeAmountMinor: 10000, SetupFeeMinor: 2000, PricedAmountMinor: 12000}}}
	for _, test := range []struct {
		name        string
		adjustments []QuoteAdjustment
		want        int64
	}{
		{"undiscounted", nil, 10000},
		{"ordinary_discount_also_discounts_installation", []QuoteAdjustment{{TypeCode: QuoteAdjustmentCampaignDiscount, SourceReference: "campaign", SourceVersionDigest: strings.Repeat("a", 64), AmountMinor: 6000}}, 5000},
		{"all_free", []QuoteAdjustment{{TypeCode: QuoteAdjustmentCampaignDiscount, SourceReference: "campaign", SourceVersionDigest: strings.Repeat("a", 64), AmountMinor: 12000}}, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := CalculatePeriodicNetAmountWithAdjustments(line, test.adjustments)
			if err != nil || got != test.want {
				t.Fatalf("period net=%d err=%v want=%d", got, err, test.want)
			}
		})
	}
	original, target := int64(10000), int64(8000)
	adjustments := []QuoteAdjustment{{TypeCode: QuoteAdjustmentAgentPrice, SourceReference: "rule", SourceVersionDigest: strings.Repeat("a", 64), OriginalPeriodAmountMinor: &original, TargetPeriodAmountMinor: &target}}
	priced, err := ApplyAgentPrice(line, adjustments)
	if err != nil {
		t.Fatal(err)
	}
	got, err := CalculatePeriodicNetAmountWithAdjustments(priced, adjustments)
	if err != nil || got != 8000 {
		t.Fatalf("negotiated period net=%d err=%v", got, err)
	}
}
