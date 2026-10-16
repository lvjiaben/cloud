package catalog

import (
	"testing"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestBuildRenewalQuoteUsesRenewalScheduleWithoutSetupOrInventory(t *testing.T) {
	t.Parallel()

	maximumRenewal := 12
	taxRate := 600
	baseRenewal := int64(100)
	input := RenewalQuoteInput{CurrencyCode: "CNY", Lines: []RenewalQuoteLineInput{{
		ReferenceNo: "entitlement-1", ProductNo: "product-1", VersionNo: "version-1",
		BillingRuleNo: "billing-1", Quantity: 2, PurchaseCycles: 3,
	}}}
	products := []purchaseQuoteProductRow{{
		ProductNo: "product-1", VersionNo: "version-1", CategoryNo: "category-1",
		NameZhCN: "持续服务", NameEnUS: "Managed service",
		ProductStatusCode: "active", CategoryStatusCode: "active",
		VersionStatusCode: "published", FulfillmentStrategy: FulfillmentStrategyPaymentConfirmedPlugin,
	}}
	rules := []entity.CatalogBillingRule{{
		BillingRuleNo: "billing-1", VersionNo: "version-1",
		ChargeTypeCode: string(ChargeTypeRecurring), PeriodUnitCode: string(PeriodUnitMonth),
		PeriodCount: 1, PricePhaseCode: string(PricePhaseBoth),
		QuantityModeCode: string(QuantityModeFixed), MinimumPurchaseCycles: 1,
		MaximumRenewalCycles: &maximumRenewal, StatusCode: "active",
	}}
	prices := []entity.CatalogPrice{{
		PriceNo: "price-base", VersionNo: "version-1", BillingRuleNo: "billing-1",
		CurrencyCode: "CNY", AmountMinor: 250, SetupFeeMinor: 999,
		RenewalAmountMinor: &baseRenewal, StatusCode: "active",
	}}
	taxes := []entity.CatalogTaxPolicy{{
		TaxPolicyNo: "tax-1", VersionNo: "version-1", CurrencyCode: "CNY",
		TaxTreatmentCode: string(TaxTreatmentTaxable), TaxRateBasisPoints: &taxRate,
		TaxCategoryCode: "cloud_service", RoundingScopeCode: string(TaxRoundingScopeLine),
		RoundingModeCode: string(TaxRoundingHalfUp), RoundingQuantumMinor: 1,
	}}

	lines, err := buildRenewalQuote(input, products, rules, prices, taxes)
	if err != nil || len(lines) != 1 || len(lines[0].Charges) != 1 {
		t.Fatalf("build renewal quote lines=%+v err=%v", lines, err)
	}
	if lines[0].InventoryDecisionCode != "not_required" || lines[0].FulfillmentModeCode != "automatic" {
		t.Fatalf("renewal inventory projection=%+v", lines[0])
	}
	if lines[0].Charges[0].ChargeAmountMinor != 600 || lines[0].Charges[0].SetupFeeMinor != 0 {
		t.Fatalf("renewal charges=%+v", lines[0].Charges)
	}
	for _, status := range []string{"draft", "suspended", "retired", "inactive"} {
		products[0].ProductStatusCode = status
		if _, err = buildRenewalQuote(input, products, rules, prices, taxes); !bizerr.Is(err, CodePublicQuoteNotAvailable) {
			t.Fatalf("stopped status %s accepted renewal: %v", status, err)
		}
	}
	products[0].ProductStatusCode = "active"

	prices[0].RenewalAmountMinor = nil
	if _, err = buildRenewalQuote(input, products, rules, prices, taxes); !bizerr.Is(err, CodePublicQuoteNotAvailable) {
		t.Fatalf("missing renewal schedule returned %v", err)
	}
}

func TestBuildRenewalQuoteRestoresGenericConfigurationPrice(t *testing.T) {
	t.Parallel()

	maximumRenewal := 12
	baseRenewal := int64(100)
	configurationRenewal := int64(50)
	dimensionNo := "cpd_0123456789abcdef0123456789abcdef"
	input := RenewalQuoteInput{CurrencyCode: "CNY", Lines: []RenewalQuoteLineInput{{
		ReferenceNo: "entitlement-phone", ProductNo: "product-phone", VersionNo: "version-phone",
		BillingRuleNo: "billing-phone", Quantity: 2, PurchaseCycles: 3,
		PriceSelections: []ConfigurationPriceSelection{{DimensionNo: dimensionNo, Quantity: 1}},
	}}}
	products := []purchaseQuoteProductRow{{
		ProductNo: "product-phone", VersionNo: "version-phone", CategoryNo: "category-phone",
		NameZhCN: "手机保障服务", NameEnUS: "Phone protection",
		ProductStatusCode: "active", CategoryStatusCode: "active", VersionStatusCode: "published",
		FulfillmentStrategy: FulfillmentStrategyPaymentConfirmedPlugin,
	}}
	rules := []entity.CatalogBillingRule{{
		BillingRuleNo: "billing-phone", VersionNo: "version-phone",
		ChargeTypeCode: string(ChargeTypeRecurring), PeriodUnitCode: string(PeriodUnitMonth),
		PeriodCount: 1, PricePhaseCode: string(PricePhaseBoth), QuantityModeCode: string(QuantityModeFixed),
		MinimumPurchaseCycles: 1, MaximumRenewalCycles: &maximumRenewal, StatusCode: "active",
	}}
	prices := []entity.CatalogPrice{
		{PriceNo: "price-base", VersionNo: "version-phone", BillingRuleNo: "billing-phone",
			CurrencyCode: "CNY", AmountMinor: 100, RenewalAmountMinor: &baseRenewal, StatusCode: "active"},
		{PriceNo: "price-configuration", VersionNo: "version-phone", BillingRuleNo: "billing-phone",
			OptionNo: &dimensionNo, CurrencyCode: "CNY", AmountMinor: 50,
			RenewalAmountMinor: &configurationRenewal, StatusCode: "active"},
	}
	taxes := []entity.CatalogTaxPolicy{{
		TaxPolicyNo: "tax-phone", VersionNo: "version-phone", CurrencyCode: "CNY",
		TaxTreatmentCode: string(TaxTreatmentExempt), TaxCategoryCode: "physical_goods",
		RoundingScopeCode: string(TaxRoundingScopeLine), RoundingModeCode: string(TaxRoundingHalfUp),
		RoundingQuantumMinor: 1,
	}}

	lines, err := buildRenewalQuote(input, products, rules, prices, taxes)
	if err != nil || len(lines) != 1 || len(lines[0].Charges) != 2 {
		t.Fatalf("build generic configuration renewal lines=%+v err=%v", lines, err)
	}
	configurationCharge := lines[0].Charges[1]
	if configurationCharge.OptionNo != "" || configurationCharge.ConfigurationPriceDimensionNo != dimensionNo ||
		configurationCharge.ChargeAmountMinor != 300 || configurationCharge.SetupFeeMinor != 0 {
		t.Fatalf("generic configuration renewal charge=%+v", configurationCharge)
	}
}
