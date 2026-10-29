// This file verifies authoritative public quote composition and exact tax rounding.

package catalog

import (
	"strconv"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestBuildPublicConfigurationQuoteUsesSelectedQuantityAndExplicitTax(t *testing.T) {
	optionNo := "option-memory"
	rate := 1000
	source := &publicQuoteSource{
		BillingRules: []entity.CatalogBillingRule{{
			BillingRuleNo: "billing-monthly", ChargeTypeCode: string(ChargeTypeRecurring),
			PeriodUnitCode: string(PeriodUnitMonth), PeriodCount: 1,
			PricePhaseCode: string(PricePhaseNewPurchase), QuantityModeCode: string(QuantityModePerUnit),
			MinimumPurchaseCycles: 1,
		}},
		Prices: []entity.CatalogPrice{
			{BillingRuleNo: "billing-monthly", CurrencyCode: "CNY", AmountMinor: 1000, SetupFeeMinor: 100},
			{BillingRuleNo: "billing-monthly", OptionNo: &optionNo, CurrencyCode: "CNY", AmountMinor: 200, SetupFeeMinor: 10},
		},
		TaxPolicies: []entity.CatalogTaxPolicy{{
			CurrencyCode: "CNY", TaxTreatmentCode: string(TaxTreatmentTaxable),
			TaxRateBasisPoints: &rate, TaxCategoryCode: "cloud_service",
			RoundingScopeCode: string(TaxRoundingScopeTotal),
			RoundingModeCode:  string(TaxRoundingHalfUp), RoundingQuantumMinor: 1,
		}},
	}
	quote, err := buildPublicConfigurationQuote(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "CNY", source,
		map[string]int{optionNo: 2},
	)
	if err != nil || len(quote.Terms) != 1 {
		t.Fatalf("build public quote=%+v err=%v", quote, err)
	}
	term := quote.Terms[0]
	if term.Status != PublicQuoteFinal || len(term.Lines) != 2 ||
		term.Lines[1].Quantity != 2 || term.Lines[1].ChargeAmountMinor != 400 ||
		term.Lines[1].SetupFeeMinor != 10 || term.NetAmountMinor == nil ||
		*term.NetAmountMinor != 1510 || term.TaxAmountMinor == nil || *term.TaxAmountMinor != 151 ||
		term.TotalAmountMinor == nil || *term.TotalAmountMinor != 1661 {
		t.Fatalf("unexpected public quote term: %+v", term)
	}
}

func TestCalculatePublicQuoteTaxHonorsInclusiveAndRoundingScope(t *testing.T) {
	rate := 5000
	base := TaxPolicyDefinition{
		TaxTreatment: TaxTreatmentTaxable, TaxRateBasisPoints: &rate,
		TaxCategoryCode: "service", RoundingMode: TaxRoundingHalfUp,
		RoundingQuantumMinor: 1,
	}
	lines := []PublicQuoteLine{{PricedAmountMinor: 1}, {PricedAmountMinor: 1}}
	linePolicy := base
	linePolicy.RoundingScope = TaxRoundingScopeLine
	net, tax, total, err := calculatePublicQuoteTax(lines, linePolicy)
	if err != nil || net != 2 || tax != 2 || total != 4 {
		t.Fatalf("line tax net=%d tax=%d total=%d err=%v", net, tax, total, err)
	}
	totalPolicy := base
	totalPolicy.RoundingScope = TaxRoundingScopeTotal
	net, tax, total, err = calculatePublicQuoteTax(lines, totalPolicy)
	if err != nil || net != 2 || tax != 1 || total != 3 {
		t.Fatalf("total tax net=%d tax=%d total=%d err=%v", net, tax, total, err)
	}
	inclusiveRate := 1300
	inclusive := TaxPolicyDefinition{
		PriceIncludesTax: true, TaxTreatment: TaxTreatmentTaxable,
		TaxRateBasisPoints: &inclusiveRate, TaxCategoryCode: "service",
		RoundingScope: TaxRoundingScopeTotal, RoundingMode: TaxRoundingHalfUp,
		RoundingQuantumMinor: 1,
	}
	net, tax, total, err = calculatePublicQuoteTax(
		[]PublicQuoteLine{{PricedAmountMinor: 1130}}, inclusive,
	)
	if err != nil || net != 1000 || tax != 130 || total != 1130 {
		t.Fatalf("inclusive tax net=%d tax=%d total=%d err=%v", net, tax, total, err)
	}
}

func TestBuildPublicQuoteTermDoesNotInventUsageOrTrialAmounts(t *testing.T) {
	usage := entity.CatalogBillingRule{
		BillingRuleNo: "usage", ChargeTypeCode: string(ChargeTypeUsage),
		PeriodUnitCode: string(PeriodUnitMonth), PeriodCount: 1,
		PricePhaseCode: string(PricePhaseNewPurchase), QuantityModeCode: string(QuantityModeUsage),
		MinimumPurchaseCycles: 1,
	}
	trial := entity.CatalogBillingRule{
		BillingRuleNo: "trial", ChargeTypeCode: string(ChargeTypeTrial),
		PeriodUnitCode: string(PeriodUnitDay), PeriodCount: 7,
		PricePhaseCode: string(PricePhaseNewPurchase), QuantityModeCode: string(QuantityModeFixed),
		MinimumPurchaseCycles: 1, TrialQuantity: 1,
	}
	for _, testCase := range []struct {
		rule entity.CatalogBillingRule
		want string
	}{
		{usage, PublicQuotePendingUsage},
		{trial, PublicQuoteRequiresCustomerContext},
	} {
		term, err := buildPublicQuoteTerm(
			testCase.rule, nil, "CNY", TaxPolicyDefinition{}, nil, nil,
		)
		if err != nil || term.Status != testCase.want || term.TotalAmountMinor != nil || len(term.Lines) != 0 {
			t.Fatalf("non-final term=%+v err=%v", term, err)
		}
	}
}

func TestBuildPublicConfigurationQuoteRejectsCompositionAboveLineBound(t *testing.T) {
	rate := 600
	selections := make(map[string]int, maximumPublicQuoteLines)
	for index := 0; index < maximumPublicQuoteLines; index++ {
		selections["option-"+strconv.Itoa(index)] = 1
	}
	source := &publicQuoteSource{
		BillingRules: []entity.CatalogBillingRule{{
			BillingRuleNo: "billing-monthly", ChargeTypeCode: string(ChargeTypeRecurring),
			PeriodUnitCode: string(PeriodUnitMonth), PeriodCount: 1,
			PricePhaseCode: string(PricePhaseNewPurchase), QuantityModeCode: string(QuantityModeFixed),
			MinimumPurchaseCycles: 1,
		}},
		TaxPolicies: []entity.CatalogTaxPolicy{{
			CurrencyCode: "CNY", TaxTreatmentCode: string(TaxTreatmentTaxable),
			TaxRateBasisPoints: &rate, TaxCategoryCode: "cloud_service",
			RoundingScopeCode: string(TaxRoundingScopeTotal),
			RoundingModeCode:  string(TaxRoundingHalfUp), RoundingQuantumMinor: 1,
		}},
	}
	_, err := buildPublicConfigurationQuote(
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "CNY", source, selections,
	)
	if !bizerr.Is(err, CodePublicQuoteNotAvailable) {
		t.Fatalf("expected bounded public quote rejection, got %v", err)
	}
}

// A missing daily price cannot make a fully priced monthly selection unusable.
// Only schedules with actual saved prices may become purchasable term tokens.
func TestPublicQuoteIsolatesUnavailableBillingPeriods(t *testing.T) {
	for _, scenario := range []string{"missing-daily", "invalid-daily", "all-missing", "explicit-zero", "overflow"} {
		t.Run(scenario, func(t *testing.T) {
			option := "memory"
			rate := 0
			source := &publicQuoteSource{ConfigurationModes: map[string]ConfigurationPriceMode{option: ConfigurationPricePerUnit}, TaxPolicies: []entity.CatalogTaxPolicy{{CurrencyCode: "CNY", PriceIncludesTax: true, TaxTreatmentCode: string(TaxTreatmentTaxable), TaxRateBasisPoints: &rate, TaxCategoryCode: "service", RoundingScopeCode: string(TaxRoundingScopeLine), RoundingModeCode: string(TaxRoundingHalfUp), RoundingQuantumMinor: 1}}}
			for _, period := range []struct {
				name string
				unit PeriodUnit
			}{{"daily", PeriodUnitDay}, {"monthly", PeriodUnitMonth}} {
				source.BillingRules = append(source.BillingRules, entity.CatalogBillingRule{BillingRuleNo: period.name, ChargeTypeCode: string(ChargeTypeRecurring), PeriodUnitCode: string(period.unit), PeriodCount: 1, PricePhaseCode: string(PricePhaseNewPurchase), QuantityModeCode: string(QuantityModeFixed), MinimumPurchaseCycles: 1})
				source.Prices = append(source.Prices, entity.CatalogPrice{BillingRuleNo: period.name, CurrencyCode: "CNY", AmountMinor: 100})
			}
			if scenario != "all-missing" {
				source.Prices = append(source.Prices, entity.CatalogPrice{BillingRuleNo: "monthly", OptionNo: &option, CurrencyCode: "CNY", AmountMinor: 200})
			}
			if scenario == "invalid-daily" {
				source.Prices = append(source.Prices, entity.CatalogPrice{BillingRuleNo: "daily", OptionNo: &option, CurrencyCode: "CNY", AmountMinor: -1})
			}
			if scenario == "explicit-zero" {
				source.Prices[2].AmountMinor = 0
				source.Prices = append(source.Prices, entity.CatalogPrice{BillingRuleNo: "daily", OptionNo: &option, CurrencyCode: "CNY", AmountMinor: 0})
			}
			if scenario == "overflow" {
				source.Prices[2].AmountMinor = int64(1<<63 - 1)
			}
			quote, err := buildPublicConfigurationQuote("revision", "CNY", source, map[string]int{option: 2})
			if scenario == "all-missing" || scenario == "overflow" {
				if err == nil || quote != nil {
					t.Fatal("unavailable/overflowed quote became a fabricated price")
				}
				return
			}
			if err != nil || quote == nil {
				t.Fatalf("complete period was blocked: %v", err)
			}
			if scenario == "explicit-zero" {
				if len(quote.Terms) != 2 || *quote.Terms[0].TotalAmountMinor != 100 || *quote.Terms[1].TotalAmountMinor != 100 {
					t.Fatal("explicit zero price was treated as missing")
				}
				return
			}
			if len(quote.Terms) != 1 || quote.Terms[0].TermNo != "monthly" || *quote.Terms[0].TotalAmountMinor != 500 {
				t.Fatalf("unexpected purchasable terms: %+v", quote.Terms)
			}
			// Global tax errors must still fail, rather than becoming period gaps.
			source.TaxPolicies = nil
			if result, taxErr := buildPublicConfigurationQuote("revision", "CNY", source, map[string]int{option: 2}); taxErr == nil || result != nil {
				t.Fatal("missing tax policy was ignored")
			}
		})
	}
}
