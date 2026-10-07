package marketing

import (
	"testing"
	"time"

	"lina-core/pkg/bizerr"
)

func TestBuildQuotePromotionEvaluationSelectsCampaignOnceAndAllocatesLines(t *testing.T) {
	currency := "CNY"
	fixed := int64(150)
	rate := 1_000
	exclusive := int64(500)
	input := QuoteReserveInput{
		CustomerNo: "customer-1", QuoteReference: "quote-0001", CurrencyCode: currency,
		PurchasePhase: PurchasePhaseNewPurchase, CycleNumber: 1,
		Lines: []QuoteLineInput{
			{ReferenceNo: "line-1", ProductNo: "product-1", CategoryNo: "category-1", SubtotalMinor: 1_000},
			{ReferenceNo: "line-2", ProductNo: "product-2", CategoryNo: "category-2", SubtotalMinor: 2_000},
		},
	}
	source := &promotionEvaluationSource{
		now: time.Date(2026, 8, 10, 9, 0, 0, 0, time.UTC),
		candidates: []candidateRow{
			promotionCandidate("campaign-all", "benefit-all", 10, "stackable", "new_purchase", "first", nil, &currency, &fixed, nil),
			promotionCandidate("campaign-category", "benefit-category", 20, "stackable", "new_purchase", "first", nil, &currency, nil, &rate),
			promotionCandidate("campaign-exclusive", "benefit-exclusive", 30, "exclusive", "new_purchase", "first", nil, &currency, &exclusive, nil),
		},
		scopes: []scopeRow{
			{CampaignNo: "campaign-all", ScopeKind: "all_products", MatchMode: "include"},
			{CampaignNo: "campaign-category", ScopeKind: "category", MatchMode: "include", ReferenceCode: "category-2"},
			{CampaignNo: "campaign-exclusive", ScopeKind: "all_products", MatchMode: "include"},
		},
	}
	result, err := buildQuotePromotionEvaluation(input, source)
	if err != nil {
		t.Fatalf("build quote-wide promotion evaluation: %v", err)
	}
	if len(result) != 2 || result[0].CampaignNo != "campaign-all" ||
		result[0].DiscountAmountMinor != 150 || len(result[0].Allocations) != 1 ||
		result[0].Allocations[0] != (QuoteBenefitAllocation{LineReference: "line-1", AmountMinor: 150}) ||
		result[1].CampaignNo != "campaign-category" || result[1].DiscountAmountMinor != 200 ||
		len(result[1].Allocations) != 1 ||
		result[1].Allocations[0] != (QuoteBenefitAllocation{LineReference: "line-2", AmountMinor: 200}) {
		t.Fatalf("unexpected quote-wide promotion result: %+v", result)
	}
}

func TestNormalizeQuoteReserveInputRejectsDuplicateOrUnboundedLines(t *testing.T) {
	input := QuoteReserveInput{
		CustomerNo: "customer-1", QuoteReference: "quote-0001", CurrencyCode: "CNY",
		PurchasePhase: PurchasePhaseNewPurchase, CycleNumber: 1,
		Lines: []QuoteLineInput{
			{ReferenceNo: "line-1", ProductNo: "product-1", CategoryNo: "category-1", SubtotalMinor: 1},
			{ReferenceNo: "line-1", ProductNo: "product-2", CategoryNo: "category-2", SubtotalMinor: 1},
		},
	}
	if _, err := normalizeQuoteReserveInput(input); !bizerr.Is(err, CodeInvalid) {
		t.Fatalf("duplicate quote lines returned %v", err)
	}
}
