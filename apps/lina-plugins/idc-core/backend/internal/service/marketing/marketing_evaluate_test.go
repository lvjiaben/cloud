package marketing

import "testing"

func TestBuildEvaluationUsesStablePriorityStackingAndSequentialIntegerAmounts(t *testing.T) {
	input := EvaluateInput{
		CustomerNo: "customer-1", QuoteReference: "quote-0001", CurrencyCode: "CNY",
		SubtotalMinor: 1000, PurchasePhase: PurchasePhaseRenewal, CycleNumber: 2,
		ProductNo: "product-1", CategoryNo: "category-1",
	}
	currency := "CNY"
	fixed100 := int64(100)
	fixed200 := int64(200)
	rate := 1000
	limited := 3
	rows := []candidateRow{
		promotionCandidate("campaign-c", "benefit-c", 30, "same_group_exclusive", "renewal", "limited", &limited, &currency, &fixed200, nil),
		promotionCandidate("campaign-a", "benefit-a", 10, "stackable", "both", "all", nil, &currency, &fixed100, nil),
		promotionCandidate("campaign-d", "benefit-d", 40, "exclusive", "both", "all", nil, &currency, &fixed200, nil),
		promotionCandidate("campaign-b", "benefit-b", 20, "same_group_exclusive", "renewal", "limited", &limited, &currency, nil, &rate),
	}
	scopes := []scopeRow{
		{CampaignNo: "campaign-a", ScopeKind: "all_products", MatchMode: "include"},
		{CampaignNo: "campaign-b", ScopeKind: "category", MatchMode: "include", ReferenceCode: "category-1"},
		{CampaignNo: "campaign-c", ScopeKind: "product", MatchMode: "include", ReferenceCode: "product-1"},
		{CampaignNo: "campaign-d", ScopeKind: "product", MatchMode: "include", ReferenceCode: "product-1"},
	}
	result, err := buildEvaluation(input, rows, scopes, nil, nil)
	if err != nil {
		t.Fatalf("build promotion evaluation: %v", err)
	}
	if len(result.Applied) != 2 || len(result.Excluded) != 2 ||
		result.Applied[0].CampaignNo != "campaign-a" || result.Applied[0].DiscountAmountMinor != 100 ||
		result.Applied[1].CampaignNo != "campaign-b" || result.Applied[1].DiscountAmountMinor != 90 ||
		result.DiscountTotalMinor != 190 || result.PayableMinor != 810 {
		t.Fatalf("unexpected deterministic evaluation: %+v", result)
	}
	if result.Excluded[0].ReasonCode != "stack_group_conflict" ||
		result.Excluded[1].ReasonCode != "higher_priority_conflict" {
		t.Fatalf("unexpected exclusion evidence: %+v", result.Excluded)
	}
}

func TestBuildEvaluationRejectsExcludedScopeAndWrongCycle(t *testing.T) {
	currency := "CNY"
	amount := int64(100)
	row := promotionCandidate(
		"campaign-a", "benefit-a", 10, "stackable", "new_purchase", "first", nil,
		&currency, &amount, nil,
	)
	input := EvaluateInput{
		CustomerNo: "customer-1", QuoteReference: "quote-0001", CurrencyCode: "CNY",
		SubtotalMinor: 1000, PurchasePhase: PurchasePhaseRenewal, CycleNumber: 2,
		ProductNo: "product-1", CategoryNo: "category-1",
	}
	result, err := buildEvaluation(input, []candidateRow{row}, []scopeRow{
		{CampaignNo: "campaign-a", ScopeKind: "all_products", MatchMode: "include"},
		{CampaignNo: "campaign-a", ScopeKind: "product", MatchMode: "exclude", ReferenceCode: "product-1"},
	}, nil, nil)
	if err != nil || len(result.Applied) != 0 || result.PayableMinor != 1000 {
		t.Fatalf("ineligible promotion leaked into evaluation: result=%+v err=%v", result, err)
	}
}

func promotionCandidate(
	campaignNo string,
	benefitNo string,
	priority int,
	stackPolicy string,
	purchasePhase string,
	cycleScope string,
	cycleCount *int,
	currency *string,
	amount *int64,
	rate *int,
) candidateRow {
	benefitType := "fixed_discount"
	if rate != nil {
		benefitType = "percentage_discount"
	}
	return candidateRow{
		CampaignNo: campaignNo, CampaignType: "discount", Priority: priority,
		StackPolicy: stackPolicy, StackGroup: "renewal", CampaignVersion: 1,
		BenefitNo: benefitNo, BenefitType: benefitType, CurrencyCode: currency,
		AmountMinor: amount, RateBasisPoints: rate, PurchasePhase: purchasePhase,
		CycleScope: cycleScope, CycleCount: cycleCount, BenefitVersion: 1,
	}
}
