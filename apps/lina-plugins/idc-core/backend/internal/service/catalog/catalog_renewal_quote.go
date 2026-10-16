// This file assembles renewal prices from one service's immutable catalog and
// configuration references without consulting inventory or a Provider.

package catalog

import (
	"context"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// RenewalQuoteInput is one explicit-currency batch of existing service
// configurations. Each line is already bound to an immutable service fact.
type RenewalQuoteInput struct {
	CurrencyCode string
	Lines        []RenewalQuoteLineInput
	// TrialConversion uses the current sale price without setup fees for an
	// already provisioned service. The caller must verify frozen configuration
	// compatibility and recheck sale eligibility before payment.
	TrialConversion bool
}

// RenewalQuoteLineInput references one immutable generic long-lived
// entitlement. Plugin-owned configuration remains frozen by its opaque
// configuration snapshot and is never interpreted as server components here.
type RenewalQuoteLineInput struct {
	ReferenceNo     string
	ProductNo       string
	VersionNo       string
	BillingRuleNo   string
	Quantity        int
	PurchaseCycles  int
	PriceSelections []ConfigurationPriceSelection
}

// RenewalQuoteAssembler performs four bounded local reads for product/version,
// billing rules, prices, and tax. Renewal never consumes sale inventory.
type RenewalQuoteAssembler interface {
	AssembleRenewalQuote(context.Context, RenewalQuoteInput) ([]PurchaseQuoteLine, error)
	// RequireRenewalSale locks the product sale state for the caller's transaction.
	// It rejects missing or stopped products without changing paid history.
	RequireRenewalSale(context.Context, string) error
}

var _ RenewalQuoteAssembler = (*publicCatalogReaderImpl)(nil)

// RenewalQuoteAssemblerFromPublicReader keeps the public reader contract narrow.
func RenewalQuoteAssemblerFromPublicReader(reader PublicCatalogReader) (RenewalQuoteAssembler, error) {
	assembler, ok := reader.(RenewalQuoteAssembler)
	if !ok || assembler == nil {
		return nil, gerror.New("public catalog reader does not provide renewal quote assembly")
	}
	return assembler, nil
}

// AssembleRenewalQuote prices the entitlement's exact persisted product
// version with the active renewal schedule in that version and currency.
func (s *publicCatalogReaderImpl) AssembleRenewalQuote(
	ctx context.Context,
	input RenewalQuoteInput,
) ([]PurchaseQuoteLine, error) {
	var currencyErr error
	if input.TrialConversion {
		currencyErr = s.admitCurrency(ctx, input.CurrencyCode)
	} else {
		currencyErr = s.admitRenewalCurrency(ctx, input.CurrencyCode)
	}
	if currencyErr != nil {
		return nil, currencyErr
	}
	purchaseInput := PurchaseQuoteInput{CurrencyCode: input.CurrencyCode, Lines: make([]PurchaseQuoteLineInput, 0, len(input.Lines))}
	for _, line := range input.Lines {
		purchaseInput.Lines = append(purchaseInput.Lines, PurchaseQuoteLineInput{
			ReferenceNo: line.ReferenceNo, ProductNo: line.ProductNo, VersionNo: line.VersionNo,
			BillingRuleNo: line.BillingRuleNo, Quantity: line.Quantity, PurchaseCycles: line.PurchaseCycles,
			PriceSelections: append([]ConfigurationPriceSelection(nil), line.PriceSelections...),
		})
	}
	versions, products, _, _, err := validatePurchaseQuoteInput(purchaseInput)
	if err != nil {
		return nil, err
	}
	if err = verifyConfigurationPriceModes(ctx, s.database, purchaseInput.Lines); err != nil {
		return nil, unavailablePurchaseQuote(err)
	}
	productRows := make([]purchaseQuoteProductRow, 0)
	if err = s.database.GetScan(ctx, &productRows, purchaseQuoteProductsSQL,
		strings.Join(products, ","), strings.Join(versions, ","), len(versions)+1,
	); err != nil {
		return nil, unavailablePurchaseQuote(err)
	}
	if len(productRows) > len(versions) {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	lineVersions := make([]string, 0, len(input.Lines))
	lineBillingRules := make([]string, 0, len(input.Lines))
	for _, line := range input.Lines {
		lineVersions = append(lineVersions, line.VersionNo)
		lineBillingRules = append(lineBillingRules, line.BillingRuleNo)
	}
	ruleRows := make([]entity.CatalogBillingRule, 0)
	if err = s.database.GetScan(ctx, &ruleRows, purchaseQuoteBillingRulesSQL,
		strings.Join(lineVersions, ","), strings.Join(lineBillingRules, ","), len(input.Lines)+1,
	); err != nil {
		return nil, unavailablePurchaseQuote(err)
	}
	priceDimensionNos := make([]string, 0)
	for _, line := range input.Lines {
		for _, selection := range line.PriceSelections {
			priceDimensionNos = append(priceDimensionNos, selection.DimensionNo)
		}
	}
	priceRows := make([]entity.CatalogPrice, 0)
	if err = s.database.GetScan(ctx, &priceRows, purchaseQuotePricesSQL,
		strings.Join(lineVersions, ","), strings.Join(lineBillingRules, ","), input.CurrencyCode,
		strings.Join(priceDimensionNos, ","), maximumPurchaseQuotePrices+1,
	); err != nil {
		return nil, unavailablePurchaseQuote(err)
	}
	if len(priceRows) > maximumPurchaseQuotePrices {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	taxRows := make([]entity.CatalogTaxPolicy, 0)
	if err = s.database.GetScan(ctx, &taxRows, purchaseQuoteTaxPoliciesSQL,
		strings.Join(versions, ","), input.CurrencyCode, len(versions)+1,
	); err != nil {
		return nil, unavailablePurchaseQuote(err)
	}
	lines, err := buildRenewalQuote(input, productRows, ruleRows, priceRows, taxRows)
	if err != nil {
		return nil, err
	}
	return s.applyImportedPurchasePrices(ctx, input.CurrencyCode, versions, lines)
}

func buildRenewalQuote(
	input RenewalQuoteInput,
	products []purchaseQuoteProductRow,
	rules []entity.CatalogBillingRule,
	prices []entity.CatalogPrice,
	taxes []entity.CatalogTaxPolicy,
) ([]PurchaseQuoteLine, error) {
	index, err := indexPurchaseQuoteRows(products, rules, prices, taxes)
	if err != nil {
		return nil, err
	}
	result := make([]PurchaseQuoteLine, 0, len(input.Lines))
	for _, requested := range input.Lines {
		product, found := index.products[purchaseQuotePair(requested.ProductNo, requested.VersionNo)]
		if !found || product.VersionStatusCode != "published" ||
			product.ProductStatusCode != catalogStatusActive {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		if input.TrialConversion && (product.ProductStatusCode != "active" || product.CategoryStatusCode != "active" ||
			product.CurrentVersionNo == nil || *product.CurrentVersionNo != requested.VersionNo) {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		rule, found := index.rules[purchaseQuotePair(requested.VersionNo, requested.BillingRuleNo)]
		if !found || rule.StatusCode != "active" {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		definition := BillingRuleDefinitionFromEntity(rule)
		if !input.TrialConversion && definition.PricePhase == PricePhaseNewPurchase ||
			definition.ChargeType == ChargeTypeOneTime || definition.ChargeType == ChargeTypeTrial ||
			definition.QuantityMode == QuantityModeUsage {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		if input.TrialConversion && (definition.ChargeType != ChargeTypeRecurring || definition.PricePhase == PricePhaseRenewal) {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		tax, found := index.taxes[requested.VersionNo]
		if !found || tax.CurrencyCode != input.CurrencyCode ||
			ValidateTaxPolicy(taxDefinitionFromEntity(tax)) != nil {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		var charges []PurchaseQuoteCharge
		var chargeErr error
		if input.TrialConversion {
			charges, chargeErr = trialConversionQuoteCharges(requested, rule, index.prices)
		} else {
			charges, chargeErr = renewalQuoteCharges(requested, rule, index.prices)
		}
		if chargeErr != nil {
			return nil, chargeErr
		}
		result = append(result, PurchaseQuoteLine{
			ReferenceNo: requested.ReferenceNo, ProductNo: requested.ProductNo,
			VersionNo: requested.VersionNo, BillingRuleNo: requested.BillingRuleNo,
			CategoryNo:      product.CategoryNo,
			ProductNameZhCN: product.NameZhCN, ProductNameEnUS: product.NameEnUS,
			Quantity: requested.Quantity, PurchaseCycles: requested.PurchaseCycles,
			ChargeTypeCode: rule.ChargeTypeCode, PeriodUnitCode: rule.PeriodUnitCode,
			PeriodCount: rule.PeriodCount, RequiresContract: product.RequiresContract,
			FulfillmentModeCode: "automatic", RiskDecisionCode: "approved",
			InventoryDecisionCode: "not_required",
			TaxPolicyNo:           tax.TaxPolicyNo, PriceIncludesTax: tax.PriceIncludesTax,
			TaxTreatmentCode:   tax.TaxTreatmentCode,
			TaxRateBasisPoints: cloneIntPointer(tax.TaxRateBasisPoints),
			TaxCategoryCode:    tax.TaxCategoryCode, RoundingScopeCode: tax.RoundingScopeCode,
			RoundingModeCode:     tax.RoundingModeCode,
			RoundingQuantumMinor: tax.RoundingQuantumMinor, Charges: charges,
		})
	}
	return result, nil
}

func renewalQuoteCharges(
	line RenewalQuoteLineInput,
	rule entity.CatalogBillingRule,
	prices map[string][]entity.CatalogPrice,
) ([]PurchaseQuoteCharge, error) {
	definition := BillingRuleDefinitionFromEntity(rule)
	base, err := renewalQuoteCharge(line, definition, "", false, ConfigurationPriceFixed, 1, prices)
	if err != nil {
		return nil, err
	}
	charges := make([]PurchaseQuoteCharge, 0, len(line.PriceSelections)+1)
	charges = append(charges, base)
	for _, selection := range line.PriceSelections {
		charge, chargeErr := renewalQuoteCharge(
			line, definition, selection.DimensionNo, true, selection.PricingMode, selection.Quantity, prices,
		)
		if chargeErr != nil {
			return nil, chargeErr
		}
		charges = append(charges, charge)
	}
	return charges, nil
}

func renewalQuoteCharge(
	line RenewalQuoteLineInput,
	definition BillingRuleDefinition,
	optionNo string,
	configurationDimension bool,
	pricingMode ConfigurationPriceMode,
	quantity int,
	prices map[string][]entity.CatalogPrice,
) (PurchaseQuoteCharge, error) {
	scheduleRows := prices[purchaseQuotePriceKey(line.VersionNo, line.BillingRuleNo, optionNo)]
	schedule := make([]PriceTierDefinition, 0, len(scheduleRows))
	for _, row := range scheduleRows {
		schedule = append(schedule, PriceTierDefinitionFromEntity(row))
	}
	quotedQuantity := int64(quantity)
	if definition.QuantityMode == QuantityModeFixed && (pricingMode == "" || pricingMode == ConfigurationPriceFixed) {
		quotedQuantity = 1
	}
	request := BillingQuoteRequest{
		Phase: QuotePhaseRenewal, Cycles: line.PurchaseCycles, Quantity: &quotedQuantity,
	}
	var (
		quoted *BillingQuote
		err    error
	)
	if configurationDimension {
		quoted, err = quoteConfigurationPrice(definition, pricingMode, schedule, request)
	} else {
		quoted, err = QuoteBillingRule(definition, schedule, request)
	}
	if err != nil || quoted.Status != BillingQuoteFinal || quoted.ChargeAmountMinor == nil ||
		quoted.SetupFeeMinor == nil || quoted.TotalAmountMinor == nil || *quoted.SetupFeeMinor != 0 {
		return PurchaseQuoteCharge{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	charge, err := multiplyPurchaseQuoteAmount(*quoted.ChargeAmountMinor, int64(line.Quantity))
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	digestMode := pricingMode
	if digestMode == ConfigurationPriceFixed {
		digestMode = ""
	}
	digest, err := purchaseQuoteDigest(struct {
		Phase                              QuotePhase
		VersionNo, BillingRuleNo, OptionNo string
		Quantity, PurchaseQuantity, Cycles int
		PricingMode                        ConfigurationPriceMode `json:",omitempty"`
		Schedule                           []PriceTierDefinition
	}{QuotePhaseRenewal, line.VersionNo, line.BillingRuleNo, optionNo, quantity,
		line.Quantity, line.PurchaseCycles, digestMode, schedule})
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	kind := "option"
	if optionNo == "" {
		kind = "base"
	}
	result := PurchaseQuoteCharge{
		KindCode: kind, Quantity: quantity,
		ChargeAmountMinor: charge, SetupFeeMinor: 0,
		PricedAmountMinor: charge, PriceSourceDigest: digest,
	}
	if configurationDimension {
		result.ConfigurationPriceDimensionNo = optionNo
	} else {
		result.OptionNo = optionNo
	}
	return result, nil
}
