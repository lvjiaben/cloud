// These tests prove shared agent evaluation excludes marketing only for matched
// lines, freezes sources and preserves setup/tax computation.
package commerce

import (
	"context"
	"strings"
	"testing"
	"time"

	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

type agentPricingTest struct {
	input salesmanagement.PriceEvaluationInput
}

func (s *agentPricingTest) EvaluatePrices(_ context.Context, input salesmanagement.PriceEvaluationInput) ([]salesmanagement.PriceEvaluation, error) {
	s.input = input
	return []salesmanagement.PriceEvaluation{{ReferenceNo: "agent", PriceNo: "price-1", ContentDigest: strings.Repeat("a", 64), Version: 2, OriginalAmountMinor: 20000, TargetAmountMinor: 16000}}, nil
}

type agentMarketingTest struct {
	input              QuoteAdjustmentInput
	consumed, reversed bool
}

func (s *agentMarketingTest) EvaluateAndReserveQuote(_ context.Context, input QuoteAdjustmentInput) (*QuoteAdjustmentResult, error) {
	s.input = input
	return &QuoteAdjustmentResult{Adjustments: []QuoteAdjustment{{ReferenceNo: "ordinary", TypeCode: QuoteAdjustmentCoupon, SourceReference: "coupon-1", SourceVersionDigest: strings.Repeat("b", 64), AmountMinor: 100}}}, nil
}
func (s *agentMarketingTest) ConsumeQuote(context.Context, string, string) error {
	s.consumed = true
	return nil
}
func (s *agentMarketingTest) ReverseConsumedQuote(context.Context, string, string) error {
	s.reversed = true
	return nil
}
func TestAgentQuoteExcludesMatchedLinesFromMarketing(t *testing.T) {
	pricing := &agentPricingTest{}
	marketing := &agentMarketingTest{}
	owner, err := NewAgentQuoteAdjustmentEvaluator(pricing, marketing)
	if err != nil {
		t.Fatal(err)
	}
	input := QuoteAdjustmentInput{CustomerNo: "customer", CurrencyCode: "CNY", IssuedAt: time.Now(), Lines: []QuoteAdjustmentLineInput{{ReferenceNo: "agent", ProductNo: "p1", Quantity: 2, PurchaseCycles: 1, PeriodSubtotalMinor: 20000, SubtotalMinor: 21000, ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1}, {ReferenceNo: "ordinary", ProductNo: "p2", Quantity: 1, PurchaseCycles: 1, PeriodSubtotalMinor: 1000, SubtotalMinor: 1000}}}
	result, err := owner.EvaluateAndReserveQuote(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Adjustments) != 2 || len(marketing.input.Lines) != 1 || marketing.input.Lines[0].ReferenceNo != "ordinary" || result.Adjustments[0].SourceReference != "price-1:2" {
		t.Fatalf("incorrect mixed pricing: %+v marketing=%+v", result, marketing.input)
	}
	if pricing.input.Lines[0].PeriodSubtotalMinor != 20000 || pricing.input.Lines[0].Quantity != 2 {
		t.Fatal("setup fee or quantity lost at pricing seam")
	}
	if err := owner.(QuoteReservationFinalizer).ConsumeQuote(context.Background(), "c", "q"); err != nil {
		t.Fatal(err)
	}
	if err := owner.(QuoteReservationReverser).ReverseConsumedQuote(context.Background(), "c", "q"); err != nil {
		t.Fatal(err)
	}
	if !marketing.consumed || !marketing.reversed {
		t.Fatal("marketing reservation lifecycle not delegated")
	}
}
func TestAgentQuoteDiscountPreservesSetupFeeAndTax(t *testing.T) {
	rate := 1000
	line := catalog.PurchaseQuoteLine{Quantity: 2, PurchaseCycles: 1, ChargeTypeCode: "recurring", TaxTreatmentCode: "taxable", TaxRateBasisPoints: &rate, RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1, Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", Quantity: 2, ChargeAmountMinor: 20000, SetupFeeMinor: 1000, PricedAmountMinor: 21000}}}
	amount, err := CalculatePricingAmounts(line, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if amount.SetupSubtotalMinor != 1000 || amount.PeriodSubtotalMinor != 20000 || amount.NetAmountMinor != 17000 || amount.TotalAmountMinor != 18700 {
		t.Fatalf("incorrect tax/setup amount: %+v", amount)
	}
}
func TestAgentPriceBasisIncludesOneTimeProductExcludesOnlySetup(t *testing.T) {
	line := catalog.PurchaseQuoteLine{PeriodUnitCode: "once", TaxTreatmentCode: "exempt", Charges: []catalog.PurchaseQuoteCharge{{ChargeAmountMinor: 12000, SetupFeeMinor: 500, PricedAmountMinor: 12500}}}
	basis, err := agentPriceBasis(line)
	if err != nil || basis != 12000 {
		t.Fatalf("one-time pricing basis=%d err=%v", basis, err)
	}
}
func TestAgentPriceBasisExtractsEmbeddedTaxBeforeDiscount(t *testing.T) {
	rate := 1000
	line := catalog.PurchaseQuoteLine{PeriodUnitCode: "month", TaxTreatmentCode: "taxable", PriceIncludesTax: true, TaxRateBasisPoints: &rate, RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1, Charges: []catalog.PurchaseQuoteCharge{{ChargeAmountMinor: 11000, SetupFeeMinor: 1100, PricedAmountMinor: 12100}}}
	basis, err := agentPriceBasis(line)
	if err != nil || basis != 10000 {
		t.Fatalf("tax-exclusive basis=%d err=%v", basis, err)
	}
	amount, err := CalculatePricingAmounts(line, 2000)
	if err != nil || amount.SetupSubtotalMinor != 1000 || amount.NetAmountMinor != 9000 || amount.TotalAmountMinor != 9900 {
		t.Fatalf("tax-inclusive repricing=%+v err=%v", amount, err)
	}
}

func TestAgentPriceSourceSupportsMarkupAndKeepsInstallation(t *testing.T) {
	rate := 1000
	for _, target := range []int64{0, 7000, 8000, 10000} {
		original := int64(8000)
		line := catalog.PurchaseQuoteLine{PeriodUnitCode: "month", TaxTreatmentCode: "taxable", TaxRateBasisPoints: &rate, RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1, Charges: []catalog.PurchaseQuoteCharge{{KindCode: "base", ChargeAmountMinor: 6000, SetupFeeMinor: 1000, PricedAmountMinor: 7000}, {KindCode: "option", ChargeAmountMinor: 2000, PricedAmountMinor: 2000}}}
		adjustment := QuoteAdjustment{TypeCode: QuoteAdjustmentAgentPrice, OriginalPeriodAmountMinor: &original, TargetPeriodAmountMinor: &target, SourceReference: "rule", SourceVersionDigest: strings.Repeat("a", 64)}
		adjusted, err := ApplyAgentPrice(line, []QuoteAdjustment{adjustment})
		if err != nil {
			t.Fatal(err)
		}
		amounts, err := CalculatePricingAmountsWithAdjustments(adjusted, []QuoteAdjustment{adjustment})
		if err != nil || amounts.SetupSubtotalMinor != 1000 || amounts.PeriodSubtotalMinor != target || amounts.DiscountMinor != 0 || amounts.TotalAmountMinor != (target+1000)*11/10 {
			t.Fatalf("target=%d adjusted=%+v err=%v", target, amounts, err)
		}
		if line.Charges[0].ChargeAmountMinor != 6000 {
			t.Fatal("input catalog mutated")
		}
	}
}
func TestAgentPriceSourceRejectsMixedMarketingAndWrongOriginal(t *testing.T) {
	original, target := int64(99), int64(100)
	line := catalog.PurchaseQuoteLine{Charges: []catalog.PurchaseQuoteCharge{{ChargeAmountMinor: 100}}}
	adjustment := QuoteAdjustment{TypeCode: QuoteAdjustmentAgentPrice, OriginalPeriodAmountMinor: &original, TargetPeriodAmountMinor: &target}
	if _, err := ApplyAgentPrice(line, []QuoteAdjustment{adjustment}); err == nil {
		t.Fatal("stale original accepted")
	}
	original = 100
	if _, err := ApplyAgentPrice(line, []QuoteAdjustment{adjustment, {TypeCode: QuoteAdjustmentCoupon, AmountMinor: 1}}); err == nil {
		t.Fatal("marketing stacked")
	}
}
