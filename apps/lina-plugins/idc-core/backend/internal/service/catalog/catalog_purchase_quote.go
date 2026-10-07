// This file owns the bounded catalog projection used by immutable checkout quotes.

package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	maximumPurchaseQuoteLines          = 100
	maximumPurchaseQuotePrices         = 100_000
	maximumPurchaseQuoteChargesPerItem = 4_096
)

// PurchaseQuoteLineInput contains only catalog-owned references previously
// persisted by the cart service.
type PurchaseQuoteLineInput struct {
	ReferenceNo     string
	ProductNo       string
	VersionNo       string
	BillingRuleNo   string
	Quantity        int
	PurchaseCycles  int
	PriceSelections []ConfigurationPriceSelection
}

// PurchaseQuoteInput is one explicit-currency, new-purchase batch.
type PurchaseQuoteInput struct {
	CurrencyCode           string
	Lines                  []PurchaseQuoteLineInput
	AuthoritativeTrialUses map[string]int
}

// PurchaseQuoteCharge freezes one exact catalog price schedule result. Amounts
// remain in their catalog tax mode; the commerce domain derives tax-exclusive
// amounts after all quote-wide adjustments are known.
type PurchaseQuoteCharge struct {
	KindCode                      string
	OptionNo                      string
	ConfigurationPriceDimensionNo string
	Quantity                      int
	ChargeAmountMinor             int64
	SetupFeeMinor                 int64
	PricedAmountMinor             int64
	PriceSourceDigest             string
}

// PurchaseQuoteLine is the complete local catalog fact set for one cart item.
type PurchaseQuoteLine struct {
	ReferenceNo               string
	ProductNo                 string
	VersionNo                 string
	BillingRuleNo             string
	CategoryNo                string
	ProductNameZhCN           string
	ProductNameEnUS           string
	PublicSlug                string
	ConfigurationRequired     bool
	Quantity                  int
	PurchaseCycles            int
	ChargeTypeCode            string
	TrialLimitQuantity        int
	PriorTrialUses            int
	PeriodUnitCode            string
	PeriodCount               int
	RequiresContract          bool
	ContractPolicyCode        string
	ContractTemplateNo        string
	FulfillmentModeCode       string
	RiskDecisionCode          string
	InventoryDecisionCode     string
	InventoryTrackingEnabled  bool
	InventoryVersion          int64
	InventorySellableQuantity int64
	RouteDecisionDigest       string
	TaxPolicyNo               string
	PriceIncludesTax          bool
	TaxTreatmentCode          string
	TaxRateBasisPoints        *int
	TaxCategoryCode           string
	RoundingScopeCode         string
	RoundingModeCode          string
	RoundingQuantumMinor      int64
	Charges                   []PurchaseQuoteCharge
}

// PurchaseQuoteAssembler performs a fixed number of bounded local reads and
// never invokes a resource owner or Provider.
type PurchaseQuoteAssembler interface {
	AssemblePurchaseQuote(context.Context, PurchaseQuoteInput) ([]PurchaseQuoteLine, error)
}

var _ PurchaseQuoteAssembler = (*publicCatalogReaderImpl)(nil)

// PurchaseQuoteAssemblerFromPublicReader keeps the anonymous reader contract narrow.
func PurchaseQuoteAssemblerFromPublicReader(reader PublicCatalogReader) (PurchaseQuoteAssembler, error) {
	assembler, ok := reader.(PurchaseQuoteAssembler)
	if !ok || assembler == nil {
		return nil, gerror.New("public catalog reader does not provide purchase quote assembly")
	}
	return assembler, nil
}

type purchaseQuoteProductRow struct {
	ProductNo             string  `orm:"product_no"`
	VersionNo             string  `orm:"version_no"`
	CategoryNo            string  `orm:"category_no"`
	NameZhCN              string  `orm:"name_zh_cn"`
	NameEnUS              string  `orm:"name_en_us"`
	PublicSlug            string  `orm:"public_slug"`
	ConfigurationRequired bool    `orm:"configuration_required"`
	ProductStatusCode     string  `orm:"product_status_code"`
	CategoryStatusCode    string  `orm:"category_status_code"`
	VersionStatusCode     string  `orm:"version_status_code"`
	CurrentVersionNo      *string `orm:"current_version_no"`
	RequiresContract      bool    `orm:"requires_contract"`
	ContractPolicyCode    string  `orm:"contract_policy_code"`
	ContractTemplateNo    string  `orm:"contract_template_no"`
	FulfillmentStrategy   string  `orm:"fulfillment_strategy"`
}

type purchaseQuoteSimpleInventoryRow struct {
	VersionNo        string    `orm:"version_no"`
	TrackingEnabled  bool      `orm:"tracking_enabled"`
	SellableQuantity int64     `orm:"sellable_quantity"`
	ReservedQuantity int64     `orm:"reserved_quantity"`
	Version          int64     `orm:"version"`
	UpdatedAt        time.Time `orm:"updated_at"`
}

// AssemblePurchaseQuote deliberately separates the five catalog projections:
// product/version, billing rules, prices, tax policies, product-neutral
// inventory. Server resource selection belongs to the selected fulfillment
// plugin and never participates in the core quote.
func (s *publicCatalogReaderImpl) AssemblePurchaseQuote(
	ctx context.Context,
	input PurchaseQuoteInput,
) ([]PurchaseQuoteLine, error) {
	versions, products, _, optionNumbers, err := validatePurchaseQuoteInput(input)
	if err != nil {
		return nil, err
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
	lineVersions, lineBillingRules := make([]string, 0, len(input.Lines)), make([]string, 0, len(input.Lines))
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
	priceRows := make([]entity.CatalogPrice, 0)
	if err = s.database.GetScan(ctx, &priceRows, purchaseQuotePricesSQL,
		strings.Join(lineVersions, ","), strings.Join(lineBillingRules, ","), input.CurrencyCode,
		strings.Join(optionNumbers, ","), maximumPurchaseQuotePrices+1,
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
	simpleInventoryRows := make([]purchaseQuoteSimpleInventoryRow, 0)
	if err = s.database.GetScan(ctx, &simpleInventoryRows, purchaseQuoteSimpleInventorySQL,
		strings.Join(versions, ","), strings.Join(versions, ","), len(versions)+1,
	); err != nil {
		return nil, unavailablePurchaseQuote(err)
	}
	if len(simpleInventoryRows) > len(versions) {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	return buildPurchaseQuoteWithSimpleInventory(
		input, productRows, ruleRows, priceRows, taxRows, simpleInventoryRows,
	)
}

func validatePurchaseQuoteInput(input PurchaseQuoteInput) (
	[]string,
	[]string,
	[]string,
	[]string,
	error,
) {
	if input.CurrencyCode != strings.TrimSpace(input.CurrencyCode) ||
		!currencyCodePattern.MatchString(input.CurrencyCode) || len(input.Lines) == 0 ||
		len(input.Lines) > maximumPurchaseQuoteLines {
		return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	versionSet, productSet := make(map[string]struct{}), make(map[string]struct{})
	dimensionSet := make(map[string]struct{})
	ruleSet, referenceSet := make(map[string]struct{}), make(map[string]struct{})
	for _, line := range input.Lines {
		if strings.TrimSpace(line.ReferenceNo) == "" || line.ReferenceNo != strings.TrimSpace(line.ReferenceNo) ||
			len(line.ReferenceNo) > 128 || !existingCatalogReferencePattern.MatchString(line.ProductNo) ||
			!existingCatalogReferencePattern.MatchString(line.VersionNo) ||
			!existingCatalogReferencePattern.MatchString(line.BillingRuleNo) ||
			line.Quantity < 1 || line.Quantity > 1_000 ||
			line.PurchaseCycles < 1 || line.PurchaseCycles > 120 {
			return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
		}
		if _, duplicate := referenceSet[line.ReferenceNo]; duplicate {
			return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
		}
		referenceSet[line.ReferenceNo] = struct{}{}
		versionSet[line.VersionNo], productSet[line.ProductNo], ruleSet[line.BillingRuleNo] = struct{}{}, struct{}{}, struct{}{}
		if len(line.PriceSelections) > maximumSelectedConfigurationPriceDimensions {
			return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
		}
		for _, selection := range line.PriceSelections {
			if !strings.HasPrefix(selection.DimensionNo, "cpd_") || selection.Quantity < 1 || selection.Quantity > 128 {
				return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
			}
			if _, duplicate := dimensionSet[line.ReferenceNo+"\x00"+selection.DimensionNo]; duplicate {
				return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
			}
			dimensionSet[line.ReferenceNo+"\x00"+selection.DimensionNo] = struct{}{}
		}
	}
	if input.AuthoritativeTrialUses != nil {
		if len(input.AuthoritativeTrialUses) > len(productSet) {
			return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
		}
		for productNo, priorUses := range input.AuthoritativeTrialUses {
			if !existingCatalogReferencePattern.MatchString(productNo) || priorUses < 0 || priorUses > 100 {
				return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
			}
			if _, requested := productSet[productNo]; !requested {
				return nil, nil, nil, nil, bizerr.NewCode(CodePublicQuoteInvalid)
			}
		}
	}
	priceDimensions := make(map[string]struct{})
	for _, line := range input.Lines {
		for _, selection := range line.PriceSelections {
			priceDimensions[selection.DimensionNo] = struct{}{}
		}
	}
	return sortedPurchaseQuoteKeys(versionSet), sortedPurchaseQuoteKeys(productSet),
		sortedPurchaseQuoteKeys(ruleSet), sortedPurchaseQuoteKeys(priceDimensions), nil
}

func sortedPurchaseQuoteKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

type purchaseQuoteRowIndex struct {
	products map[string]purchaseQuoteProductRow
	rules    map[string]entity.CatalogBillingRule
	prices   map[string][]entity.CatalogPrice
	taxes    map[string]entity.CatalogTaxPolicy
}

func indexPurchaseQuoteRows(
	products []purchaseQuoteProductRow,
	rules []entity.CatalogBillingRule,
	prices []entity.CatalogPrice,
	taxes []entity.CatalogTaxPolicy,
) (*purchaseQuoteRowIndex, error) {
	result := &purchaseQuoteRowIndex{
		products: make(map[string]purchaseQuoteProductRow, len(products)),
		rules:    make(map[string]entity.CatalogBillingRule, len(rules)),
		prices:   make(map[string][]entity.CatalogPrice),
		taxes:    make(map[string]entity.CatalogTaxPolicy, len(taxes)),
	}
	for _, product := range products {
		key := purchaseQuotePair(product.ProductNo, product.VersionNo)
		if _, duplicate := result.products[key]; duplicate {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		result.products[key] = product
	}
	for _, rule := range rules {
		key := purchaseQuotePair(rule.VersionNo, rule.BillingRuleNo)
		if _, duplicate := result.rules[key]; duplicate {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		result.rules[key] = rule
	}
	for _, price := range prices {
		optionNo := ""
		if price.OptionNo != nil {
			optionNo = *price.OptionNo
		}
		key := purchaseQuotePriceKey(price.VersionNo, price.BillingRuleNo, optionNo)
		result.prices[key] = append(result.prices[key], price)
	}
	for _, tax := range taxes {
		if _, duplicate := result.taxes[tax.VersionNo]; duplicate {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		result.taxes[tax.VersionNo] = tax
	}
	return result, nil
}

func buildPurchaseQuoteWithSimpleInventory(
	input PurchaseQuoteInput,
	products []purchaseQuoteProductRow,
	rules []entity.CatalogBillingRule,
	prices []entity.CatalogPrice,
	taxes []entity.CatalogTaxPolicy,
	simpleInventory []purchaseQuoteSimpleInventoryRow,
) ([]PurchaseQuoteLine, error) {
	index, err := indexPurchaseQuoteRows(products, rules, prices, taxes)
	if err != nil {
		return nil, err
	}
	simpleInventoryByVersion := make(map[string]purchaseQuoteSimpleInventoryRow, len(simpleInventory))
	for _, inventory := range simpleInventory {
		if inventory.VersionNo == "" || inventory.Version < 1 || inventory.SellableQuantity < 0 ||
			inventory.ReservedQuantity < 0 || inventory.ReservedQuantity > inventory.SellableQuantity {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		if _, duplicate := simpleInventoryByVersion[inventory.VersionNo]; duplicate {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		simpleInventoryByVersion[inventory.VersionNo] = inventory
	}
	trialFacts, trialErr := purchaseQuoteTrialFacts(input, index)
	if trialErr != nil {
		return nil, trialErr
	}
	result := make([]PurchaseQuoteLine, 0, len(input.Lines))
	for _, requested := range input.Lines {
		requestedQuantity := requested.Quantity
		product, found := index.products[purchaseQuotePair(requested.ProductNo, requested.VersionNo)]
		if !found || product.ProductStatusCode != "active" || product.CategoryStatusCode != "active" ||
			product.VersionStatusCode != "published" || product.CurrentVersionNo == nil ||
			*product.CurrentVersionNo != requested.VersionNo {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		rule, found := index.rules[purchaseQuotePair(requested.VersionNo, requested.BillingRuleNo)]
		if !found || rule.StatusCode != "active" {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		definition := BillingRuleDefinitionFromEntity(rule)
		if definition.PricePhase == PricePhaseRenewal ||
			definition.QuantityMode == QuantityModeUsage || requested.PurchaseCycles < definition.MinimumPurchaseCycles {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		tax, found := index.taxes[requested.VersionNo]
		if !found || tax.CurrencyCode != input.CurrencyCode || ValidateTaxPolicy(taxDefinitionFromEntity(tax)) != nil {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		fulfillment, risk, policyErr := purchaseQuotePolicies(product)
		if policyErr != nil {
			return nil, policyErr
		}
		trial := trialFacts[requested.ProductNo]
		charges, chargeErr := purchaseQuoteCharges(requested, rule, index.prices, trial.priorUsesPointer())
		if chargeErr != nil {
			return nil, chargeErr
		}
		inventory, inventoryErr := resolvePurchaseQuoteInventory(requested, requestedQuantity, simpleInventoryByVersion)
		if inventoryErr != nil {
			return nil, inventoryErr
		}
		result = append(result, PurchaseQuoteLine{
			ReferenceNo: requested.ReferenceNo, ProductNo: requested.ProductNo,
			VersionNo: requested.VersionNo, BillingRuleNo: requested.BillingRuleNo,
			CategoryNo: product.CategoryNo, ProductNameZhCN: product.NameZhCN,
			ProductNameEnUS: product.NameEnUS, Quantity: requestedQuantity,
			PublicSlug: product.PublicSlug, ConfigurationRequired: product.ConfigurationRequired,
			PurchaseCycles: requested.PurchaseCycles, ChargeTypeCode: rule.ChargeTypeCode,
			TrialLimitQuantity: trial.limitQuantity, PriorTrialUses: trial.priorUses,
			PeriodUnitCode: rule.PeriodUnitCode, PeriodCount: rule.PeriodCount,
			RequiresContract: product.RequiresContract, ContractPolicyCode: product.ContractPolicyCode,
			ContractTemplateNo: product.ContractTemplateNo, FulfillmentModeCode: fulfillment,
			RiskDecisionCode: risk, InventoryDecisionCode: inventory.decision,
			InventoryTrackingEnabled: inventory.trackingEnabled,
			InventoryVersion:         inventory.version, InventorySellableQuantity: inventory.sellableQuantity,
			RouteDecisionDigest: inventory.digest,
			TaxPolicyNo:         tax.TaxPolicyNo, PriceIncludesTax: tax.PriceIncludesTax,
			TaxTreatmentCode:   tax.TaxTreatmentCode,
			TaxRateBasisPoints: cloneIntPointer(tax.TaxRateBasisPoints), TaxCategoryCode: tax.TaxCategoryCode,
			RoundingScopeCode: tax.RoundingScopeCode, RoundingModeCode: tax.RoundingModeCode,
			RoundingQuantumMinor: tax.RoundingQuantumMinor, Charges: charges,
		})
	}
	return result, nil
}

type purchaseQuoteInventoryFact struct {
	decision, digest          string
	trackingEnabled           bool
	version, sellableQuantity int64
}

func resolvePurchaseQuoteInventory(
	requested PurchaseQuoteLineInput,
	requestedQuantity int,
	simpleByVersion map[string]purchaseQuoteSimpleInventoryRow,
) (purchaseQuoteInventoryFact, error) {
	if simple, found := simpleByVersion[requested.VersionNo]; found {
		remainingQuantity := simple.SellableQuantity
		if simple.TrackingEnabled {
			remainingQuantity -= simple.ReservedQuantity
		}
		if simple.TrackingEnabled && remainingQuantity < int64(requestedQuantity) {
			return purchaseQuoteInventoryFact{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		digest, err := purchaseQuoteDigest(struct {
			Kind               string
			VersionNo          string
			TrackingEnabled    bool
			SellableQuantity   int64
			ReservedQuantity   int64
			InventoryVersion   int64
			InventoryUpdatedAt time.Time
			RequestedQuantity  int
		}{"simple_inventory", simple.VersionNo, simple.TrackingEnabled, simple.SellableQuantity, simple.ReservedQuantity,
			simple.Version, simple.UpdatedAt, requestedQuantity})
		if err != nil {
			return purchaseQuoteInventoryFact{}, bizerr.WrapCode(err, CodePublicQuoteNotAvailable)
		}
		return purchaseQuoteInventoryFact{decision: "available", digest: digest,
			trackingEnabled: simple.TrackingEnabled, version: simple.Version,
			sellableQuantity: remainingQuantity}, nil
	}
	return purchaseQuoteInventoryFact{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
}

type purchaseQuoteTrialFact struct {
	trial         bool
	limitQuantity int
	priorUses     int
}

func (fact purchaseQuoteTrialFact) priorUsesPointer() *int {
	if !fact.trial {
		return nil
	}
	value := fact.priorUses
	return &value
}

func purchaseQuoteTrialFacts(
	input PurchaseQuoteInput,
	index *purchaseQuoteRowIndex,
) (map[string]purchaseQuoteTrialFact, error) {
	result := make(map[string]purchaseQuoteTrialFact, len(input.Lines))
	requestedByProduct := make(map[string]int)
	trialLineCount := 0
	for _, line := range input.Lines {
		rule, found := index.rules[purchaseQuotePair(line.VersionNo, line.BillingRuleNo)]
		if !found {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		definition := BillingRuleDefinitionFromEntity(rule)
		if definition.ChargeType != ChargeTypeTrial {
			continue
		}
		trialLineCount++
		if input.AuthoritativeTrialUses == nil || line.PurchaseCycles != 1 {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		priorUses, found := input.AuthoritativeTrialUses[line.ProductNo]
		if !found || definition.TrialQuantity < 1 || definition.TrialQuantity > 100 {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		if existing, duplicate := result[line.ProductNo]; duplicate && existing.limitQuantity != definition.TrialQuantity {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		result[line.ProductNo] = purchaseQuoteTrialFact{
			trial: true, limitQuantity: definition.TrialQuantity, priorUses: priorUses,
		}
		requestedByProduct[line.ProductNo] += line.Quantity
	}
	if trialLineCount != 0 && trialLineCount != len(input.Lines) {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	for productNo, requested := range requestedByProduct {
		fact := result[productNo]
		if requested < 1 || requested > fact.limitQuantity ||
			fact.priorUses > fact.limitQuantity-requested {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
	}
	return result, nil
}

func purchaseQuotePolicies(product purchaseQuoteProductRow) (string, string, error) {
	switch product.FulfillmentStrategy {
	case FulfillmentStrategyManual, FulfillmentStrategyApprovalThenPlugin,
		FulfillmentStrategyPaymentConfirmedPlugin:
		// The order and payment flow must not impersonate fulfillment review.
		// All three supported strategies become manual_pending, review_pending,
		// or queued only after the financial owner publishes authoritative
		// settlement evidence.
		return "automatic", "approved", nil
	default:
		return "", "", bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
}

func purchaseQuoteCharges(
	line PurchaseQuoteLineInput,
	rule entity.CatalogBillingRule,
	prices map[string][]entity.CatalogPrice,
	priorTrialUses *int,
) ([]PurchaseQuoteCharge, error) {
	definition := BillingRuleDefinitionFromEntity(rule)
	result := make([]PurchaseQuoteCharge, 0, 1)
	base, err := purchaseQuoteCharge(line, definition, "", false, 1, prices, priorTrialUses)
	if err != nil {
		return nil, err
	}
	result = append(result, base)
	for _, selection := range line.PriceSelections {
		charge, err := purchaseQuoteCharge(line, definition, selection.DimensionNo, true, selection.Quantity, prices, priorTrialUses)
		if err != nil {
			return nil, err
		}
		result = append(result, charge)
	}
	return result, nil
}

func purchaseQuoteCharge(
	line PurchaseQuoteLineInput,
	definition BillingRuleDefinition,
	optionNo string,
	configurationDimension bool,
	quantity int,
	prices map[string][]entity.CatalogPrice,
	priorTrialUses *int,
) (PurchaseQuoteCharge, error) {
	scheduleRows := prices[purchaseQuotePriceKey(line.VersionNo, line.BillingRuleNo, optionNo)]
	schedule := make([]PriceTierDefinition, 0, len(scheduleRows))
	for _, row := range scheduleRows {
		schedule = append(schedule, PriceTierDefinitionFromEntity(row))
	}
	quotedQuantity := int64(quantity)
	if definition.QuantityMode == QuantityModeFixed {
		quotedQuantity = 1
	}
	quoted, err := QuoteBillingRule(definition, schedule, BillingQuoteRequest{
		Phase: QuotePhaseNewPurchase, Cycles: line.PurchaseCycles, Quantity: &quotedQuantity,
		AuthoritativePriorTrialUses: priorTrialUses,
	})
	if err != nil || quoted.Status != BillingQuoteFinal || quoted.ChargeAmountMinor == nil ||
		quoted.SetupFeeMinor == nil || quoted.TotalAmountMinor == nil {
		return PurchaseQuoteCharge{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	requestedQuantity := line.Quantity
	charge, err := multiplyPurchaseQuoteAmount(*quoted.ChargeAmountMinor, int64(requestedQuantity))
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	setup, err := multiplyPurchaseQuoteAmount(*quoted.SetupFeeMinor, int64(requestedQuantity))
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	total, err := addBillingAmount(charge, setup)
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	digest, err := purchaseQuoteDigest(struct {
		VersionNo, BillingRuleNo, OptionNo string
		Quantity, PurchaseQuantity, Cycles int
		Schedule                           []PriceTierDefinition
	}{line.VersionNo, line.BillingRuleNo, optionNo, quantity, requestedQuantity,
		line.PurchaseCycles, schedule})
	if err != nil {
		return PurchaseQuoteCharge{}, err
	}
	kind := "option"
	if optionNo == "" {
		kind = "base"
	}
	result := PurchaseQuoteCharge{
		KindCode: kind, Quantity: quantity,
		ChargeAmountMinor: charge, SetupFeeMinor: setup,
		PricedAmountMinor: total, PriceSourceDigest: digest,
	}
	if configurationDimension {
		result.ConfigurationPriceDimensionNo = optionNo
	} else {
		result.OptionNo = optionNo
	}
	return result, nil
}

func multiplyPurchaseQuoteAmount(amount, multiplier int64) (int64, error) {
	if amount < 0 || multiplier < 1 || (amount != 0 && multiplier > math.MaxInt64/amount) {
		return 0, bizerr.NewCode(CodeBillingAmountOverflow)
	}
	return amount * multiplier, nil
}

func purchaseQuoteDigest(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func purchaseQuotePair(left, right string) string { return left + "\x00" + right }

func purchaseQuotePriceKey(versionNo, billingRuleNo, optionNo string) string {
	return strings.Join([]string{versionNo, billingRuleNo, optionNo}, "\x00")
}

func unavailablePurchaseQuote(err error) error {
	return bizerr.WrapCode(gerror.Wrap(err, "assemble purchase quote"), CodePublicQuoteUnavailable)
}

const purchaseQuoteProductsSQL = `
SELECT p.product_no, v.version_no, p.category_no, p.public_slug,
       p.name_zh_cn, p.name_en_us, p.status_code AS product_status_code,
       c.status_code AS category_status_code, v.status_code AS version_status_code,
	   p.current_version_no,
       v.requires_contract, v.contract_policy_code, v.contract_template_no,
	   policy.strategy_code AS fulfillment_strategy,
       EXISTS(SELECT 1 FROM plugin_idc_core_catalog_product_version_configuration configured WHERE configured.tenant_id=v.tenant_id AND configured.version_no=v.version_no) AS configuration_required
FROM plugin_idc_core_catalog_product p
INNER JOIN plugin_idc_core_catalog_category c
    ON c.tenant_id = p.tenant_id AND c.category_no = p.category_no
INNER JOIN plugin_idc_core_catalog_product_version v
    ON v.tenant_id = p.tenant_id AND v.product_no = p.product_no
INNER JOIN plugin_idc_core_catalog_product_version_fulfillment_policy policy
	ON policy.tenant_id = v.tenant_id AND policy.version_no = v.version_no
WHERE p.tenant_id = 0
  AND p.product_no = ANY(string_to_array(?, ','))
  AND v.version_no = ANY(string_to_array(?, ','))
ORDER BY p.product_no ASC, v.version_no ASC
LIMIT ?
FOR SHARE OF p, c, v, policy`

const purchaseQuoteSimpleInventorySQL = `
WITH active_reservation AS (
    SELECT version_no, SUM(reserved_quantity)::BIGINT AS reserved_quantity
    FROM plugin_idc_core_commerce_inventory_reservation
    WHERE version_no = ANY(string_to_array(?, ','))
      AND status_code IN ('held','committed')
    GROUP BY version_no
)
SELECT inventory.version_no, inventory.tracking_enabled,
       inventory.sellable_quantity, COALESCE(active_reservation.reserved_quantity,0) AS reserved_quantity,
       inventory.version, inventory.updated_at
FROM plugin_idc_core_catalog_product_version_inventory inventory
LEFT JOIN active_reservation ON active_reservation.version_no=inventory.version_no
WHERE inventory.version_no = ANY(string_to_array(?, ','))
ORDER BY inventory.version_no ASC
LIMIT ?
FOR SHARE OF inventory`

const purchaseQuoteBillingRulesSQL = `
WITH requested(version_no, billing_rule_no) AS (
    SELECT DISTINCT *
    FROM unnest(string_to_array(?, ','), string_to_array(?, ','))
)
SELECT rule.id, rule.tenant_id, rule.billing_rule_no, rule.version_no,
       rule.rule_code, rule.charge_type_code, rule.period_unit_code,
       rule.period_count, rule.price_phase_code, rule.quantity_mode_code,
       rule.trial_quantity, rule.status_code, rule.created_at,
       rule.minimum_purchase_cycles, rule.maximum_renewal_cycles,
       rule.meter_unit_code, rule.tier_mode_code, rule.usage_collection_unit_code,
       rule.usage_collection_count, rule.usage_rounding_mode_code,
       rule.usage_rounding_quantity, rule.usage_minimum_quantity,
       rule.usage_maximum_quantity, rule.usage_missing_policy_code
FROM plugin_idc_core_catalog_billing_rule rule
INNER JOIN requested
    ON requested.version_no = rule.version_no
   AND requested.billing_rule_no = rule.billing_rule_no
WHERE rule.tenant_id = 0
ORDER BY rule.version_no ASC, rule.billing_rule_no ASC
LIMIT ?
FOR SHARE OF rule`

const purchaseQuotePricesSQL = `
WITH requested(version_no, billing_rule_no) AS (
    SELECT DISTINCT *
    FROM unnest(string_to_array(?, ','), string_to_array(?, ','))
)
SELECT price.id, price.tenant_id, price.price_no, price.version_no,
       price.billing_rule_no, COALESCE(price.configuration_price_dimension_no,price.option_no) AS option_no, price.currency_code,
       price.amount_minor, price.setup_fee_minor, price.renewal_amount_minor,
       price.minimum_quantity, price.maximum_quantity, price.status_code,
       price.created_at
FROM plugin_idc_core_catalog_price price
INNER JOIN requested
    ON requested.version_no = price.version_no
   AND requested.billing_rule_no = price.billing_rule_no
WHERE price.tenant_id = 0
  AND price.currency_code = ? AND price.status_code = 'active'
  AND ((price.option_no IS NULL AND price.configuration_price_dimension_no IS NULL)
       OR price.configuration_price_dimension_no = ANY(string_to_array(?, ',')))
ORDER BY price.version_no ASC, price.billing_rule_no ASC,
         price.configuration_price_dimension_no ASC NULLS FIRST,
         price.minimum_quantity ASC, price.id ASC
LIMIT ?
FOR SHARE OF price`

const purchaseQuoteTaxPoliciesSQL = `
SELECT tax.id, tax.tenant_id, tax.tax_policy_no, tax.version_no,
       tax.currency_code, tax.price_includes_tax, tax.tax_treatment_code,
       tax.tax_rate_basis_points, tax.tax_category_code,
       tax.rounding_scope_code, tax.rounding_mode_code,
       tax.rounding_quantum_minor, tax.created_at
FROM plugin_idc_core_catalog_tax_policy tax
WHERE tax.tenant_id = 0
  AND tax.version_no = ANY(string_to_array(?, ','))
  AND tax.currency_code = ?
ORDER BY tax.version_no ASC, tax.id ASC
LIMIT ?
FOR SHARE OF tax`
