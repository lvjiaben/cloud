package catalog

import (
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestCurrencyPricingIndexAllowsOnlyExplicitCompleteCurrencies(t *testing.T) {
	t.Parallel()
	renewal := int64(100)
	rules := []entity.CatalogBillingRule{
		{
			BillingRuleNo: "billing-month", ChargeTypeCode: "recurring",
			PeriodUnitCode: "month", PeriodCount: 1, PricePhaseCode: "both",
			QuantityModeCode: "fixed", MinimumPurchaseCycles: 1,
		},
		{
			BillingRuleNo: "billing-year", ChargeTypeCode: "recurring",
			PeriodUnitCode: "year", PeriodCount: 1, PricePhaseCode: "both",
			QuantityModeCode: "fixed", MinimumPurchaseCycles: 1,
		},
	}
	prices := make([]entity.CatalogPrice, 0)
	for _, rule := range rules {
		prices = append(prices,
			currencyTestPrice(rule.BillingRuleNo, "CNY", "", &renewal),
			currencyTestPrice(rule.BillingRuleNo, "CNY", "option-cpu", &renewal),
			currencyTestPrice(rule.BillingRuleNo, "CNY", "option-memory", &renewal),
			currencyTestPrice(rule.BillingRuleNo, "USD", "", &renewal),
			currencyTestPrice(rule.BillingRuleNo, "USD", "option-cpu", &renewal),
		)
	}
	index, err := NewCurrencyPricingIndex(rules, prices)
	if err != nil {
		t.Fatalf("build currency pricing index: %v", err)
	}
	renewal = -1
	options := []string{"option-memory", "option-cpu"}
	cny, err := index.Evaluate("CNY", options)
	if err != nil || !cny.Sellable || len(cny.Issues) != 0 {
		t.Fatalf("complete CNY result=%+v err=%v", cny, err)
	}
	usd, err := index.Evaluate("USD", options)
	if err != nil || usd.Sellable || len(usd.Issues) != 2 {
		t.Fatalf("incomplete USD result=%+v err=%v", usd, err)
	}
	for _, issue := range usd.Issues {
		if issue.OptionNo != "option-memory" || issue.Kind != CurrencyPriceDimensionOption ||
			issue.Reason != CurrencyPriceIssueMissing {
			t.Fatalf("unexpected USD issue: %+v", issue)
		}
	}
	sellable, err := index.SellableCurrencies(options)
	if err != nil || len(sellable) != 1 || sellable[0] != "CNY" {
		t.Fatalf("sellable currencies=%v err=%v", sellable, err)
	}
	if _, exists := index.ExplicitSchedule("billing-month", "USD", "option-memory"); exists {
		t.Fatal("missing USD option schedule fell back to another currency")
	}
	schedule, exists := index.ExplicitSchedule("billing-month", "CNY", "option-memory")
	if !exists {
		t.Fatal("explicit CNY option schedule missing")
	}
	schedule[0].AmountMinor = -1
	*schedule[0].RenewalAmountMinor = -1
	frozen, exists := index.ExplicitSchedule("billing-month", "CNY", "option-memory")
	if !exists || frozen[0].AmountMinor != 100 || frozen[0].RenewalAmountMinor == nil ||
		*frozen[0].RenewalAmountMinor != 100 {
		t.Fatalf("currency pricing index was mutated through input or output: %+v", frozen)
	}
}

func TestCurrencyPricingIndexReportsInvalidScheduleWithoutHidingOtherCurrency(t *testing.T) {
	t.Parallel()
	renewal := int64(10)
	rule := entity.CatalogBillingRule{
		BillingRuleNo: "billing-month", ChargeTypeCode: "recurring",
		PeriodUnitCode: "month", PeriodCount: 1, PricePhaseCode: "both",
		QuantityModeCode: "fixed", MinimumPurchaseCycles: 1,
	}
	ten := int64(10)
	prices := []entity.CatalogPrice{
		currencyTestPrice(rule.BillingRuleNo, "CNY", "", &renewal),
		currencyTestPrice(rule.BillingRuleNo, "CNY", "option-cpu", &renewal),
		{BillingRuleNo: rule.BillingRuleNo, CurrencyCode: "EUR",
			MaximumQuantity: &ten, RenewalAmountMinor: &renewal},
		{BillingRuleNo: rule.BillingRuleNo, CurrencyCode: "EUR",
			MinimumQuantity: 11, RenewalAmountMinor: &renewal},
		currencyTestPrice(rule.BillingRuleNo, "EUR", "option-cpu", &renewal),
	}
	index, err := NewCurrencyPricingIndex([]entity.CatalogBillingRule{rule}, prices)
	if err != nil {
		t.Fatalf("build mixed-validity currency index: %v", err)
	}
	eur, err := index.Evaluate("EUR", []string{"option-cpu"})
	if err != nil || eur.Sellable || len(eur.Issues) != 1 ||
		eur.Issues[0].Kind != CurrencyPriceDimensionBase ||
		eur.Issues[0].Reason != CurrencyPriceIssueInvalid {
		t.Fatalf("invalid EUR result=%+v err=%v", eur, err)
	}
	cny, err := index.Evaluate("CNY", []string{"option-cpu"})
	if err != nil || !cny.Sellable {
		t.Fatalf("valid CNY was contaminated by EUR result=%+v err=%v", cny, err)
	}
	if _, exists := index.ExplicitSchedule(rule.BillingRuleNo, "EUR", ""); exists {
		t.Fatal("invalid EUR schedule exposed as quoteable")
	}
}

func TestCurrencyPricingIndexRejectsAmbiguousCurrencyOrSelection(t *testing.T) {
	t.Parallel()
	rule := entity.CatalogBillingRule{
		BillingRuleNo: "billing-month", ChargeTypeCode: "recurring",
		PeriodUnitCode: "month", PeriodCount: 1, PricePhaseCode: "new_purchase",
		QuantityModeCode: "fixed", MinimumPurchaseCycles: 1,
	}
	index, err := NewCurrencyPricingIndex(
		[]entity.CatalogBillingRule{rule},
		[]entity.CatalogPrice{currencyTestPrice(rule.BillingRuleNo, "CNY", "", nil)},
	)
	if err != nil {
		t.Fatalf("build currency index: %v", err)
	}
	testCases := []struct {
		currency string
		options  []string
	}{
		{currency: "cny"},
		{currency: "CNY", options: []string{"option-1", "option-1"}},
		{currency: "CNY", options: []string{" "}},
	}
	for _, testCase := range testCases {
		if _, err = index.Evaluate(testCase.currency, testCase.options); !bizerr.Is(err, CodeCurrencyPricingInvalid) {
			t.Fatalf("currency=%q options=%v returned %v", testCase.currency, testCase.options, err)
		}
	}
}

func currencyTestPrice(
	billingRuleNo string,
	currencyCode string,
	optionNo string,
	renewalAmount *int64,
) entity.CatalogPrice {
	price := entity.CatalogPrice{
		BillingRuleNo: billingRuleNo, CurrencyCode: currencyCode,
		AmountMinor: 100, RenewalAmountMinor: renewalAmount,
	}
	if optionNo != "" {
		price.OptionNo = stringPointer(optionNo)
	}
	return price
}
