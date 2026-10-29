package catalog

import (
	"strings"
	"testing"
	"time"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func TestBuildPurchaseQuoteFreezesExactPriceTaxPolicyAndInventory(t *testing.T) {
	now := time.Date(2026, 8, 10, 8, 0, 0, 0, time.UTC)
	versionNo, productNo, ruleNo := "version-basic-1", "product-basic-1", "billing-basic-1"
	currentVersion := versionNo
	renewalBase, renewalMemory, renewalRegion := int64(90), int64(45), int64(0)
	input := PurchaseQuoteInput{CurrencyCode: "CNY", Lines: []PurchaseQuoteLineInput{{
		ReferenceNo: "cart-item-1", ProductNo: productNo, VersionNo: versionNo,
		BillingRuleNo: ruleNo, Quantity: 2, PurchaseCycles: 3,
	}}}
	productRows := []purchaseQuoteProductRow{{
		ProductNo: productNo, VersionNo: versionNo, CategoryNo: "category-service",
		NameZhCN: "测试服务", NameEnUS: "Test service",
		ProductStatusCode: "active", CategoryStatusCode: "active", VersionStatusCode: "published",
		CurrentVersionNo: &currentVersion, FulfillmentStrategy: FulfillmentStrategyPaymentConfirmedPlugin,
	}}
	rules := []entity.CatalogBillingRule{{
		VersionNo: versionNo, BillingRuleNo: ruleNo, StatusCode: "active",
		ChargeTypeCode: "recurring", PeriodUnitCode: "month", PeriodCount: 1,
		PricePhaseCode: "both", QuantityModeCode: "fixed", MinimumPurchaseCycles: 1,
	}}
	prices := []entity.CatalogPrice{
		{VersionNo: versionNo, BillingRuleNo: ruleNo, CurrencyCode: "CNY", AmountMinor: 100, SetupFeeMinor: 10, RenewalAmountMinor: &renewalBase, StatusCode: "active"},
		{VersionNo: versionNo, BillingRuleNo: ruleNo, OptionNo: stringPointer("option-memory-1"), CurrencyCode: "CNY", AmountMinor: 50, RenewalAmountMinor: &renewalMemory, StatusCode: "active"},
		{VersionNo: versionNo, BillingRuleNo: ruleNo, OptionNo: stringPointer("option-region-1"), CurrencyCode: "CNY", AmountMinor: 0, RenewalAmountMinor: &renewalRegion, StatusCode: "active"},
	}
	taxes := []entity.CatalogTaxPolicy{{
		VersionNo: versionNo, TaxPolicyNo: "tax-basic-1", CurrencyCode: "CNY",
		TaxTreatmentCode: "exempt", TaxCategoryCode: "cloud_service",
		RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1,
	}}
	inventory := []purchaseQuoteSimpleInventoryRow{{VersionNo: versionNo, TrackingEnabled: true, SellableQuantity: 10, Version: 1, UpdatedAt: now}}
	result, err := buildPurchaseQuoteWithSimpleInventory(input, productRows, rules, prices, taxes, inventory)
	if err != nil || len(result) != 1 {
		t.Fatalf("build purchase quote result=%+v err=%v", result, err)
	}
	productRows[0].Deleted = true
	if _, deletedErr := buildPurchaseQuoteWithSimpleInventory(input, productRows, rules, prices, taxes, inventory); !bizerr.Is(deletedErr, CodePublicQuoteNotAvailable) {
		t.Fatalf("deleted product accepted new purchase: %v", deletedErr)
	}
	productRows[0].Deleted = false
	line := result[0]
	if line.CategoryNo != "category-service" || line.Quantity != 2 || line.InventoryDecisionCode != "available" ||
		line.FulfillmentModeCode != "automatic" || line.RiskDecisionCode != "approved" || len(line.Charges) != 1 {
		t.Fatalf("unexpected frozen purchase quote line: %+v", line)
	}
	chargeByOption := make(map[string]PurchaseQuoteCharge, len(line.Charges))
	for _, charge := range line.Charges {
		chargeByOption[charge.OptionNo] = charge
		if len(charge.PriceSourceDigest) != 64 {
			t.Fatalf("price source digest is not frozen: %+v", charge)
		}
	}
	if chargeByOption[""].ChargeAmountMinor != 600 || chargeByOption[""].SetupFeeMinor != 20 {
		t.Fatalf("product quantity or cycles were not applied once: %+v", chargeByOption)
	}
}

func TestBuildPurchaseQuoteKeepsConfigurationDimensionSeparateFromLegacyOption(t *testing.T) {
	dimensionNo := "cpd_11111111111111111111111111111111"
	input := PurchaseQuoteInput{CurrencyCode: "CNY", Lines: []PurchaseQuoteLineInput{{
		ReferenceNo: "cart-phone", ProductNo: "product-phone", VersionNo: "version-phone",
		BillingRuleNo: "billing-phone", Quantity: 1, PurchaseCycles: 1,
		PriceSelections: []ConfigurationPriceSelection{{DimensionNo: dimensionNo, Quantity: 1}},
	}}}
	current := "version-phone"
	lines, err := buildPurchaseQuoteWithSimpleInventory(input,
		[]purchaseQuoteProductRow{{ProductNo: "product-phone", VersionNo: "version-phone", CategoryNo: "category-phone",
			NameZhCN: "手机", NameEnUS: "Phone", ProductStatusCode: "active", CategoryStatusCode: "active",
			VersionStatusCode: "published", CurrentVersionNo: &current, FulfillmentStrategy: FulfillmentStrategyManual}},
		[]entity.CatalogBillingRule{{VersionNo: "version-phone", BillingRuleNo: "billing-phone", StatusCode: "active",
			ChargeTypeCode: "one_time", PeriodUnitCode: "once", PeriodCount: 1, PricePhaseCode: "new_purchase",
			QuantityModeCode: "fixed", MinimumPurchaseCycles: 1}},
		[]entity.CatalogPrice{
			{VersionNo: "version-phone", BillingRuleNo: "billing-phone", CurrencyCode: "CNY", AmountMinor: 100, StatusCode: "active"},
			{VersionNo: "version-phone", BillingRuleNo: "billing-phone", OptionNo: &dimensionNo, CurrencyCode: "CNY", AmountMinor: 25, StatusCode: "active"},
		},
		[]entity.CatalogTaxPolicy{{VersionNo: "version-phone", TaxPolicyNo: "tax-phone", CurrencyCode: "CNY",
			TaxTreatmentCode: "exempt", TaxCategoryCode: "goods", RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1}},
		[]purchaseQuoteSimpleInventoryRow{{VersionNo: "version-phone", TrackingEnabled: false, Version: 1}},
	)
	if err != nil || len(lines) != 1 || len(lines[0].Charges) != 2 {
		t.Fatalf("configuration quote lines=%+v err=%v", lines, err)
	}
	charge := lines[0].Charges[1]
	if charge.ConfigurationPriceDimensionNo != dimensionNo || charge.OptionNo != "" || charge.PricedAmountMinor != 25 {
		t.Fatalf("configuration dimension was mixed with legacy option: %+v", charge)
	}
}

func TestBuildPurchaseQuoteUsesSimpleInventoryWithoutLegacyServerRoute(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	versionNo, productNo, ruleNo := "version-phone-1", "product-phone-1", "billing-phone-1"
	currentVersion := versionNo
	input := PurchaseQuoteInput{CurrencyCode: "CNY", Lines: []PurchaseQuoteLineInput{{
		ReferenceNo: "cart-phone-1", ProductNo: productNo, VersionNo: versionNo,
		BillingRuleNo: ruleNo, Quantity: 2, PurchaseCycles: 1,
	}}}
	products := []purchaseQuoteProductRow{{
		ProductNo: productNo, VersionNo: versionNo, CategoryNo: "category-phone",
		NameZhCN: "手机", NameEnUS: "Phone", ProductStatusCode: "active",
		CategoryStatusCode: "active", VersionStatusCode: "published",
		CurrentVersionNo: &currentVersion, FulfillmentStrategy: FulfillmentStrategyPaymentConfirmedPlugin,
	}}
	rules := []entity.CatalogBillingRule{{
		VersionNo: versionNo, BillingRuleNo: ruleNo, StatusCode: "active",
		ChargeTypeCode: "one_time", PeriodUnitCode: "once", PeriodCount: 1,
		PricePhaseCode: "new_purchase", QuantityModeCode: "fixed", MinimumPurchaseCycles: 1,
	}}
	prices := []entity.CatalogPrice{{
		VersionNo: versionNo, BillingRuleNo: ruleNo, CurrencyCode: "CNY",
		AmountMinor: 50_000, StatusCode: "active",
	}}
	taxes := []entity.CatalogTaxPolicy{{
		VersionNo: versionNo, TaxPolicyNo: "tax-phone-1", CurrencyCode: "CNY",
		TaxTreatmentCode: "exempt", TaxCategoryCode: "goods",
		RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1,
	}}
	simpleInventory := []purchaseQuoteSimpleInventoryRow{{
		VersionNo: versionNo, TrackingEnabled: true, SellableQuantity: 5, ReservedQuantity: 3,
		Version: 7, UpdatedAt: now,
	}}
	result, err := buildPurchaseQuoteWithSimpleInventory(
		input, products, rules, prices, taxes, simpleInventory,
	)
	if err != nil || len(result) != 1 {
		t.Fatalf("generic inventory quote result=%+v err=%v", result, err)
	}
	line := result[0]
	if line.Quantity != 2 || line.InventoryDecisionCode != "available" ||
		!line.InventoryTrackingEnabled || line.InventoryVersion != 7 ||
		line.InventorySellableQuantity != 2 ||
		len(line.RouteDecisionDigest) != 64 || len(line.Charges) != 1 ||
		line.Charges[0].PricedAmountMinor != 100_000 {
		t.Fatalf("simple inventory was not authoritative: %+v", line)
	}
}

func TestBuildPurchaseQuoteSimpleInventoryPreventsLegacyFallbackOnShortage(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	versionNo, productNo, ruleNo := "version-ssl-1", "product-ssl-1", "billing-ssl-1"
	currentVersion := versionNo
	input := PurchaseQuoteInput{CurrencyCode: "CNY", Lines: []PurchaseQuoteLineInput{{
		ReferenceNo: "cart-ssl-1", ProductNo: productNo, VersionNo: versionNo,
		BillingRuleNo: ruleNo, Quantity: 2, PurchaseCycles: 1,
	}}}
	products := []purchaseQuoteProductRow{{
		ProductNo: productNo, VersionNo: versionNo, CategoryNo: "category-ssl",
		ProductStatusCode: "active", CategoryStatusCode: "active", VersionStatusCode: "published",
		CurrentVersionNo: &currentVersion, FulfillmentStrategy: FulfillmentStrategyPaymentConfirmedPlugin,
	}}
	rules := []entity.CatalogBillingRule{{
		VersionNo: versionNo, BillingRuleNo: ruleNo, StatusCode: "active",
		ChargeTypeCode: "one_time", PeriodUnitCode: "once", PeriodCount: 1,
		PricePhaseCode: "new_purchase", QuantityModeCode: "fixed", MinimumPurchaseCycles: 1,
	}}
	prices := []entity.CatalogPrice{{
		VersionNo: versionNo, BillingRuleNo: ruleNo, CurrencyCode: "CNY", StatusCode: "active",
	}}
	taxes := []entity.CatalogTaxPolicy{{
		VersionNo: versionNo, TaxPolicyNo: "tax-ssl-1", CurrencyCode: "CNY",
		TaxTreatmentCode: "exempt", TaxCategoryCode: "service",
		RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1,
	}}
	simpleInventory := []purchaseQuoteSimpleInventoryRow{{
		VersionNo: versionNo, TrackingEnabled: true, SellableQuantity: 5, ReservedQuantity: 4,
		Version: 1, UpdatedAt: now,
	}}
	if _, err := buildPurchaseQuoteWithSimpleInventory(
		input, products, rules, prices, taxes, simpleInventory,
	); !bizerr.Is(err, CodePublicQuoteNotAvailable) {
		t.Fatalf("simple inventory shortage incorrectly used legacy fallback: %v", err)
	}
}

func TestPurchaseQuoteProductsSQLDoesNotRequireLegacyServerProduct(t *testing.T) {
	if strings.Contains(purchaseQuoteProductsSQL, "plugin_idc_core_legacy_server_product") {
		t.Fatal("generic product quote still reads the legacy server product table")
	}
	if strings.Contains(purchaseQuoteProductsSQL, "catalog_provider_draft_completion") ||
		!strings.Contains(purchaseQuoteProductsSQL, "catalog_product_version_fulfillment_policy") {
		t.Fatal("generic product quote does not use the published fulfillment policy")
	}
}

func TestPurchaseQuotePoliciesFreezePublishedFulfillmentStrategy(t *testing.T) {
	tests := []struct {
		name        string
		strategy    string
		fulfillment string
		risk        string
		wantError   bool
	}{
		{name: "manual", strategy: FulfillmentStrategyManual, fulfillment: "automatic", risk: "approved"},
		{name: "approval then plugin", strategy: FulfillmentStrategyApprovalThenPlugin, fulfillment: "automatic", risk: "approved"},
		{name: "payment confirmed plugin", strategy: FulfillmentStrategyPaymentConfirmedPlugin, fulfillment: "automatic", risk: "approved"},
		{name: "missing", wantError: true},
		{name: "unsupported", strategy: "unsupported_strategy", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fulfillment, risk, err := purchaseQuotePolicies(purchaseQuoteProductRow{FulfillmentStrategy: test.strategy})
			if test.wantError {
				if !bizerr.Is(err, CodePublicQuoteNotAvailable) {
					t.Fatalf("strategy %q returned %v", test.strategy, err)
				}
				return
			}
			if err != nil || fulfillment != test.fulfillment || risk != test.risk {
				t.Fatalf("strategy %q mapped to fulfillment=%q risk=%q err=%v", test.strategy, fulfillment, risk, err)
			}
		})
	}
}

func TestBuildPurchaseQuoteRejectsNonFinalUsage(t *testing.T) {
	rule := entity.CatalogBillingRule{
		ChargeTypeCode: "usage", PeriodUnitCode: "month", PeriodCount: 1,
		PricePhaseCode: "both", QuantityModeCode: "usage", MinimumPurchaseCycles: 1,
	}
	if definition := BillingRuleDefinitionFromEntity(rule); definition.QuantityMode != QuantityModeUsage {
		t.Fatalf("usage rule did not remain explicit: %+v", definition)
	}
}

func TestBuildPurchaseQuoteRequiresAuthoritativeProductScopedTrialUsage(t *testing.T) {
	now := time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
	versionNo, productNo, ruleNo := "version-trial-1", "product-trial-1", "billing-trial-1"
	currentVersion := versionNo
	input := PurchaseQuoteInput{
		CurrencyCode: "CNY", AuthoritativeTrialUses: map[string]int{productNo: 0},
		Lines: []PurchaseQuoteLineInput{{
			ReferenceNo: "cart-trial-1", ProductNo: productNo, VersionNo: versionNo,
			BillingRuleNo: ruleNo, Quantity: 1, PurchaseCycles: 1,
		}},
	}
	products := []purchaseQuoteProductRow{{
		ProductNo: productNo, VersionNo: versionNo, CategoryNo: "category-service",
		NameZhCN: "试用服务", NameEnUS: "Trial service",
		ProductStatusCode: "active", CategoryStatusCode: "active", VersionStatusCode: "published",
		CurrentVersionNo: &currentVersion, FulfillmentStrategy: FulfillmentStrategyPaymentConfirmedPlugin,
	}}
	rules := []entity.CatalogBillingRule{{
		VersionNo: versionNo, BillingRuleNo: ruleNo, StatusCode: "active",
		ChargeTypeCode: "trial", PeriodUnitCode: "day", PeriodCount: 7,
		PricePhaseCode: "new_purchase", QuantityModeCode: "fixed",
		TrialQuantity: 1, MinimumPurchaseCycles: 1,
	}}
	prices := []entity.CatalogPrice{{
		VersionNo: versionNo, BillingRuleNo: ruleNo, CurrencyCode: "CNY", StatusCode: "active",
	}}
	definition := BillingRuleDefinitionFromEntity(rules[0])
	schedule := []PriceTierDefinition{PriceTierDefinitionFromEntity(prices[0])}
	if err := ValidatePriceSchedule(definition, schedule); err != nil {
		t.Fatalf("trial price schedule fixture is invalid: definition=%+v schedule=%+v err=%v",
			definition, schedule, err)
	}
	taxes := []entity.CatalogTaxPolicy{{
		VersionNo: versionNo, TaxPolicyNo: "tax-trial-1", CurrencyCode: "CNY",
		TaxTreatmentCode: "exempt", TaxCategoryCode: "cloud_service",
		RoundingScopeCode: "line", RoundingModeCode: "half_up", RoundingQuantumMinor: 1,
	}}
	inventory := []purchaseQuoteSimpleInventoryRow{{VersionNo: versionNo, TrackingEnabled: true, SellableQuantity: 10, Version: 1, UpdatedAt: now}}
	result, err := buildPurchaseQuoteWithSimpleInventory(input, products, rules, prices, taxes, inventory)
	if err != nil || len(result) != 1 || result[0].ChargeTypeCode != "trial" ||
		result[0].TrialLimitQuantity != 1 || result[0].PriorTrialUses != 0 ||
		len(result[0].Charges) != 1 || result[0].Charges[0].PricedAmountMinor != 0 {
		t.Fatalf("authoritative trial quote result=%+v err=%v", result, err)
	}
	input.AuthoritativeTrialUses = nil
	if _, err = buildPurchaseQuoteWithSimpleInventory(input, products, rules, prices, taxes, inventory); !bizerr.Is(err, CodePublicQuoteNotAvailable) {
		t.Fatalf("trial quote without customer usage error=%v", err)
	}
	input.AuthoritativeTrialUses = map[string]int{productNo: 0}
	input.Lines[0].Quantity = 2
	if _, err = buildPurchaseQuoteWithSimpleInventory(input, products, rules, prices, taxes, inventory); !bizerr.Is(err, CodePublicQuoteNotAvailable) {
		t.Fatalf("trial quote exceeding product limit error=%v", err)
	}
}

// TestConfigurationQuantityPurchaseAndRenewalCharge preserves exact quantities,
// cycle multiplication, setup fees and historical fixed-price source digests.
func TestConfigurationQuantityPurchaseAndRenewalCharge(t *testing.T) {
	maximum, renewal := int64(100), int64(1)
	definition := BillingRuleDefinition{ChargeType: ChargeTypeRecurring, PeriodUnit: PeriodUnitMonth, PeriodCount: 1, PricePhase: PricePhaseBoth, QuantityMode: QuantityModeFixed, MinimumPurchaseCycles: 1}
	line := PurchaseQuoteLineInput{VersionNo: "version-demo", BillingRuleNo: "billing-demo", Quantity: 2, PurchaseCycles: 3}
	prices := map[string][]entity.CatalogPrice{purchaseQuotePriceKey(line.VersionNo, line.BillingRuleNo, "cpd_demo"): {
		{AmountMinor: 2, SetupFeeMinor: 3, RenewalAmountMinor: &renewal, MaximumQuantity: &maximum},
		{MinimumQuantity: 100, AmountMinor: 1, SetupFeeMinor: 5, RenewalAmountMinor: &renewal},
	}}
	for _, test := range []struct {
		mode                   ConfigurationPriceMode
		quantity               int
		charge, setup, renewal int64
	}{
		{ConfigurationPricePerUnit, 100, 1200, 6, 600}, {ConfigurationPricePerUnit, 101, 606, 10, 606},
		{ConfigurationPriceBandTotal, 100, 12, 6, 6}, {ConfigurationPriceBandTotal, 101, 6, 10, 6},
	} {
		charge, err := purchaseQuoteCharge(line, definition, "cpd_demo", true, test.mode, test.quantity, prices, nil)
		if err != nil || charge.ChargeAmountMinor != test.charge || charge.SetupFeeMinor != test.setup || charge.Quantity != test.quantity {
			t.Fatalf("purchase %+v got=%+v err=%v", test, charge, err)
		}
		renewed, err := renewalQuoteCharge(RenewalQuoteLineInput{VersionNo: line.VersionNo, BillingRuleNo: line.BillingRuleNo, Quantity: 2, PurchaseCycles: 3}, definition, "cpd_demo", true, test.mode, test.quantity, prices)
		if err != nil || renewed.ChargeAmountMinor != test.renewal || renewed.SetupFeeMinor != 0 {
			t.Fatalf("renewal %+v got=%+v err=%v", test, renewed, err)
		}
	}
	// Graduated quantity keeps the ordered product count and cycle multiplier
	// separate from configuration quantities, including the renewal path.
	graduatedPrices := map[string][]entity.CatalogPrice{purchaseQuotePriceKey(line.VersionNo, line.BillingRuleNo, "cpd_graduated"): {
		{AmountMinor: 0, SetupFeeMinor: 5, RenewalAmountMinor: int64Pointer(0), MaximumQuantity: int64Pointer(10)},
		{MinimumQuantity: 10, AmountMinor: 200, RenewalAmountMinor: int64Pointer(300)},
	}}
	graduated, err := purchaseQuoteCharge(line, definition, "cpd_graduated", true, ConfigurationPriceGraduated, 15, graduatedPrices, nil)
	if err != nil || graduated.ChargeAmountMinor != 6000 || graduated.SetupFeeMinor != 10 || graduated.Quantity != 15 {
		t.Fatalf("graduated purchase charge=%+v err=%v", graduated, err)
	}
	graduatedRenewal, err := renewalQuoteCharge(RenewalQuoteLineInput{VersionNo: line.VersionNo, BillingRuleNo: line.BillingRuleNo, Quantity: 2, PurchaseCycles: 3}, definition, "cpd_graduated", true, ConfigurationPriceGraduated, 15, graduatedPrices)
	if err != nil || graduatedRenewal.ChargeAmountMinor != 9000 || graduatedRenewal.SetupFeeMinor != 0 {
		t.Fatalf("graduated renewal charge=%+v err=%v", graduatedRenewal, err)
	}
	prices[purchaseQuotePriceKey(line.VersionNo, line.BillingRuleNo, "")] = []entity.CatalogPrice{{AmountMinor: 2, RenewalAmountMinor: &renewal}}
	fixed, err := purchaseQuoteCharge(line, definition, "", false, ConfigurationPriceFixed, 1, prices, nil)
	if err != nil {
		t.Fatal(err)
	}
	legacyDigest, err := purchaseQuoteDigest(struct {
		VersionNo, BillingRuleNo, OptionNo string
		Quantity, PurchaseQuantity, Cycles int
		Schedule                           []PriceTierDefinition
	}{line.VersionNo, line.BillingRuleNo, "", 1, line.Quantity, line.PurchaseCycles, []PriceTierDefinition{{AmountMinor: 2, RenewalAmountMinor: &renewal}}})
	if err != nil || fixed.PriceSourceDigest != legacyDigest {
		t.Fatalf("historical fixed digest changed: got=%s want=%s err=%v", fixed.PriceSourceDigest, legacyDigest, err)
	}
}
