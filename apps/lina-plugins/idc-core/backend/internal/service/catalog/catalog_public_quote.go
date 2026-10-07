// This file calculates the current published configuration price composition.

package catalog

import (
	"context"
	"encoding/json"
	"math/big"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const (
	PublicQuoteFinal                   = "final"
	PublicQuotePendingUsage            = "pending_usage"
	PublicQuoteRequiresCustomerContext = "requires_customer_context"
	PublicQuoteLineBase                = "base"
	PublicQuoteLineOption              = "option"
	maximumPublicQuoteBytes            = 1024 * 1024
	maximumPublicQuoteLines            = 4096
	publicQuoteEnvelopeReserve         = 2048
)

type PublicConfigurationQuoteInput struct {
	PublicSlug   string
	Revision     string
	CurrencyCode string
	Selections   []PublicConfigurationSelection
}

type PublicQuoteTaxSnapshot struct {
	PriceIncludesTax     bool
	TaxTreatment         TaxTreatment
	TaxRateBasisPoints   *int
	TaxCategoryCode      string
	RoundingScope        TaxRoundingScope
	RoundingMode         TaxRoundingMode
	RoundingQuantumMinor int64
}

type PublicQuoteLine struct {
	Kind              string
	OptionNo          string
	Quantity          int
	ChargeAmountMinor int64
	SetupFeeMinor     int64
	PricedAmountMinor int64
}

type PublicQuoteTerm struct {
	TermNo                string
	Status                string
	ChargeType            ChargeType
	PeriodUnit            PeriodUnit
	PeriodCount           int
	Cycles                int
	MinimumPurchaseCycles int
	Lines                 []PublicQuoteLine
	NetAmountMinor        *int64
	TaxAmountMinor        *int64
	TotalAmountMinor      *int64
}

type PublicConfigurationQuote struct {
	Revision     string
	CurrencyCode string
	Tax          PublicQuoteTaxSnapshot
	Terms        []PublicQuoteTerm
}

func (s *publicCatalogReaderImpl) QuoteConfiguration(
	ctx context.Context,
	input PublicConfigurationQuoteInput,
) (*PublicConfigurationQuote, error) {
	slug := strings.ToLower(strings.TrimSpace(input.PublicSlug))
	currency := strings.TrimSpace(input.CurrencyCode)
	selections, err := normalizePublicConfigurationSelections(input.Selections)
	if err != nil || !validPublicConfigurationSlug(slug) ||
		currency != input.CurrencyCode || !currencyCodePattern.MatchString(currency) || s.revision == nil {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	target, err := loadPublicConfigurationTarget(ctx, s.database, slug)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	if target == nil {
		return nil, bizerr.NewCode(CodePublicCatalogNotFound)
	}
	revision := strings.TrimSpace(input.Revision)
	if revision != input.Revision || !s.revision.verify(revision, slug, target.VersionNo) {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	graph, err := s.publicQuoteGraph(ctx, slug, revision, *target)
	if err != nil {
		return nil, err
	}
	validation, err := evaluatePublicConfiguration(graph, selections)
	if err != nil || !validation.Valid {
		return nil, bizerr.NewCode(CodePublicQuoteInvalid)
	}
	source, err := loadPublicQuoteSource(ctx, s.database, target.VersionNo, currency)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	quote, err := buildPublicConfigurationQuote(revision, currency, source, selections)
	if err != nil {
		return nil, err
	}
	for index := range quote.Terms {
		quote.Terms[index].TermNo, err = s.revision.encodeQuoteTerm(
			slug, target.VersionNo, quote.Terms[index].TermNo,
		)
		if err != nil {
			return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
		}
	}
	current, err := s.database.GetValue(ctx, publicConfigurationStillCurrentSQL, slug, target.VersionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	if !current.Bool() {
		return nil, bizerr.NewCode(CodePublicQuoteStale)
	}
	if err = validatePublicConfigurationQuote(quote); err != nil {
		return nil, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	return quote, nil
}

func (s *publicCatalogReaderImpl) publicQuoteGraph(
	ctx context.Context,
	slug string,
	revision string,
	target publicConfigurationTargetRow,
) (PublicConfigurationGraph, error) {
	if graph, found := s.config.get(revision); found {
		return graph, nil
	}
	snapshot, err := loadPublicConfigurationSnapshotForTarget(ctx, s.database, slug, target)
	if err != nil {
		return PublicConfigurationGraph{}, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	if snapshot == nil || !s.revision.verify(revision, slug, snapshot.VersionNo) {
		return PublicConfigurationGraph{}, bizerr.NewCode(CodePublicQuoteStale)
	}
	graph := snapshot.Graph
	graph.Revision = revision
	if err = validatePublicConfigurationGraph(&graph); err != nil {
		return PublicConfigurationGraph{}, bizerr.WrapCode(err, CodePublicQuoteUnavailable)
	}
	s.config.put(graph)
	return graph, nil
}

func buildPublicConfigurationQuote(
	revision string,
	currency string,
	source *publicQuoteSource,
	selections map[string]int,
) (*PublicConfigurationQuote, error) {
	if source == nil || len(source.TaxPolicies) != 1 {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	policy := taxDefinitionFromEntity(source.TaxPolicies[0])
	if source.TaxPolicies[0].CurrencyCode != currency || ValidateTaxPolicy(policy) != nil {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	rules := make([]entity.CatalogBillingRule, 0, len(source.BillingRules))
	for _, rule := range source.BillingRules {
		definition := BillingRuleDefinitionFromEntity(rule)
		if definition.PricePhase != PricePhaseRenewal {
			rules = append(rules, rule)
		}
	}
	if len(rules) == 0 {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	quotedLineCount := 0
	for _, rule := range rules {
		definition := BillingRuleDefinitionFromEntity(rule)
		if definition.ChargeType == ChargeTypeTrial || definition.QuantityMode == QuantityModeUsage {
			continue
		}
		termLineCount := len(selections) + 1
		if termLineCount > maximumPublicQuoteLines-quotedLineCount {
			return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		quotedLineCount += termLineCount
	}
	pricing, err := NewCurrencyPricingIndex(rules, source.Prices)
	if err != nil {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	optionNumbers := make([]string, 0, len(selections))
	for optionNo := range selections {
		optionNumbers = append(optionNumbers, optionNo)
	}
	sort.Strings(optionNumbers)
	availability, err := pricing.Evaluate(currency, optionNumbers)
	if err != nil || !availability.Sellable {
		return nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	quote := &PublicConfigurationQuote{
		Revision: revision, CurrencyCode: currency,
		Tax: publicTaxSnapshot(policy), Terms: make([]PublicQuoteTerm, 0, len(rules)),
	}
	sort.Slice(rules, func(left, right int) bool {
		return rules[left].BillingRuleNo < rules[right].BillingRuleNo
	})
	for _, rule := range rules {
		term, buildErr := buildPublicQuoteTerm(rule, pricing, currency, policy, optionNumbers, selections)
		if buildErr != nil {
			return nil, buildErr
		}
		quote.Terms = append(quote.Terms, term)
	}
	return quote, nil
}

func buildPublicQuoteTerm(
	rule entity.CatalogBillingRule,
	pricing *CurrencyPricingIndex,
	currency string,
	policy TaxPolicyDefinition,
	optionNumbers []string,
	selections map[string]int,
) (PublicQuoteTerm, error) {
	definition := BillingRuleDefinitionFromEntity(rule)
	term := PublicQuoteTerm{
		TermNo: rule.BillingRuleNo, ChargeType: definition.ChargeType,
		PeriodUnit: definition.PeriodUnit, PeriodCount: definition.PeriodCount,
		Cycles:                definition.MinimumPurchaseCycles,
		MinimumPurchaseCycles: definition.MinimumPurchaseCycles,
		Lines:                 make([]PublicQuoteLine, 0),
	}
	if definition.ChargeType == ChargeTypeTrial {
		term.Status = PublicQuoteRequiresCustomerContext
		return term, nil
	}
	if definition.QuantityMode == QuantityModeUsage {
		term.Status = PublicQuotePendingUsage
		return term, nil
	}
	base, err := quotePublicPriceLine(rule, pricing, currency, "", 1)
	if err != nil {
		return PublicQuoteTerm{}, err
	}
	term.Lines = append(term.Lines, base)
	for _, optionNo := range optionNumbers {
		line, lineErr := quotePublicPriceLine(rule, pricing, currency, optionNo, selections[optionNo])
		if lineErr != nil {
			return PublicQuoteTerm{}, lineErr
		}
		term.Lines = append(term.Lines, line)
	}
	net, tax, total, err := calculatePublicQuoteTax(term.Lines, policy)
	if err != nil {
		return PublicQuoteTerm{}, err
	}
	term.Status = PublicQuoteFinal
	term.NetAmountMinor, term.TaxAmountMinor, term.TotalAmountMinor = &net, &tax, &total
	return term, nil
}

func quotePublicPriceLine(
	rule entity.CatalogBillingRule,
	pricing *CurrencyPricingIndex,
	currency string,
	optionNo string,
	quantity int,
) (PublicQuoteLine, error) {
	schedule, found := pricing.ExplicitSchedule(rule.BillingRuleNo, currency, optionNo)
	if !found || quantity < 1 {
		return PublicQuoteLine{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	definition := BillingRuleDefinitionFromEntity(rule)
	requestedQuantity := int64(quantity)
	if definition.QuantityMode == QuantityModeFixed {
		requestedQuantity = 1
	}
	priced, err := QuoteBillingRule(definition, schedule, BillingQuoteRequest{
		Phase: QuotePhaseNewPurchase, Cycles: definition.MinimumPurchaseCycles,
		Quantity: &requestedQuantity,
	})
	if err != nil || priced.Status != BillingQuoteFinal || priced.ChargeAmountMinor == nil ||
		priced.SetupFeeMinor == nil || priced.TotalAmountMinor == nil {
		return PublicQuoteLine{}, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	charge := *priced.ChargeAmountMinor
	setup := *priced.SetupFeeMinor
	total, err := addBillingAmount(charge, setup)
	if err != nil {
		return PublicQuoteLine{}, err
	}
	kind := PublicQuoteLineOption
	if optionNo == "" {
		kind = PublicQuoteLineBase
	}
	return PublicQuoteLine{
		Kind: kind, OptionNo: optionNo, Quantity: quantity,
		ChargeAmountMinor: charge, SetupFeeMinor: setup, PricedAmountMinor: total,
	}, nil
}

func calculatePublicQuoteTax(
	lines []PublicQuoteLine,
	policy TaxPolicyDefinition,
) (int64, int64, int64, error) {
	if len(lines) == 0 || ValidateTaxPolicy(policy) != nil {
		return 0, 0, 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	pricedTotal := int64(0)
	for _, line := range lines {
		var err error
		pricedTotal, err = addBillingAmount(pricedTotal, line.PricedAmountMinor)
		if err != nil {
			return 0, 0, 0, err
		}
	}
	if policy.TaxTreatment == TaxTreatmentExempt || policy.TaxTreatment == TaxTreatmentZeroRated {
		return pricedTotal, 0, pricedTotal, nil
	}
	calculateTax := func(amount int64) (int64, error) {
		denominator := int64(10_000)
		if policy.PriceIncludesTax {
			denominator += int64(*policy.TaxRateBasisPoints)
		}
		return RoundTaxRational(
			amount, int64(*policy.TaxRateBasisPoints), denominator,
			policy.RoundingQuantumMinor, policy.RoundingMode,
		)
	}
	taxAmount := int64(0)
	if policy.RoundingScope == TaxRoundingScopeLine {
		for _, line := range lines {
			lineTax, err := calculateTax(line.PricedAmountMinor)
			if err != nil {
				return 0, 0, 0, err
			}
			taxAmount, err = addBillingAmount(taxAmount, lineTax)
			if err != nil {
				return 0, 0, 0, err
			}
		}
	} else {
		var err error
		taxAmount, err = calculateTax(pricedTotal)
		if err != nil {
			return 0, 0, 0, err
		}
	}
	if policy.PriceIncludesTax {
		if taxAmount > pricedTotal {
			return 0, 0, 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		return pricedTotal - taxAmount, taxAmount, pricedTotal, nil
	}
	total, err := addBillingAmount(pricedTotal, taxAmount)
	return pricedTotal, taxAmount, total, err
}

// RoundTaxRational applies the catalog tax policy's exact integer rounding.
// The denominator is 10,000 for tax-exclusive amounts and 10,000 plus the
// rate for extracting embedded tax from tax-inclusive amounts.
func RoundTaxRational(
	amount int64,
	rate int64,
	denominator int64,
	quantum int64,
	mode TaxRoundingMode,
) (int64, error) {
	if amount < 0 || rate < 0 || denominator <= 0 || quantum <= 0 {
		return 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	numerator := new(big.Int).Mul(big.NewInt(amount), big.NewInt(rate))
	unitDenominator := new(big.Int).Mul(big.NewInt(denominator), big.NewInt(quantum))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, unitDenominator, remainder)
	increment := false
	switch mode {
	case TaxRoundingDown:
	case TaxRoundingUp:
		increment = remainder.Sign() > 0
	case TaxRoundingHalfUp:
		doubled := new(big.Int).Lsh(new(big.Int).Set(remainder), 1)
		increment = doubled.Cmp(unitDenominator) >= 0
	default:
		return 0, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	if increment {
		quotient.Add(quotient, big.NewInt(1))
	}
	result := quotient.Mul(quotient, big.NewInt(quantum))
	if !result.IsInt64() || result.Int64() < 0 {
		return 0, bizerr.NewCode(CodeBillingAmountOverflow)
	}
	return result.Int64(), nil
}

func taxDefinitionFromEntity(policy entity.CatalogTaxPolicy) TaxPolicyDefinition {
	return TaxPolicyDefinition{
		PriceIncludesTax:     policy.PriceIncludesTax,
		TaxTreatment:         TaxTreatment(policy.TaxTreatmentCode),
		TaxRateBasisPoints:   cloneIntPointer(policy.TaxRateBasisPoints),
		TaxCategoryCode:      policy.TaxCategoryCode,
		RoundingScope:        TaxRoundingScope(policy.RoundingScopeCode),
		RoundingMode:         TaxRoundingMode(policy.RoundingModeCode),
		RoundingQuantumMinor: policy.RoundingQuantumMinor,
	}
}

func publicTaxSnapshot(policy TaxPolicyDefinition) PublicQuoteTaxSnapshot {
	return PublicQuoteTaxSnapshot{
		PriceIncludesTax:     policy.PriceIncludesTax,
		TaxTreatment:         policy.TaxTreatment,
		TaxRateBasisPoints:   cloneIntPointer(policy.TaxRateBasisPoints),
		TaxCategoryCode:      policy.TaxCategoryCode,
		RoundingScope:        policy.RoundingScope,
		RoundingMode:         policy.RoundingMode,
		RoundingQuantumMinor: policy.RoundingQuantumMinor,
	}
}

func validatePublicConfigurationQuote(quote *PublicConfigurationQuote) error {
	if quote == nil || len(quote.Revision) != publicConfigurationRevisionLength ||
		!currencyCodePattern.MatchString(quote.CurrencyCode) || len(quote.Terms) == 0 ||
		len(quote.Terms) > maximumPublicationBillingRules {
		return gerror.New("public configuration quote is invalid")
	}
	encoded, err := json.Marshal(quote)
	if err != nil || len(encoded) > maximumPublicQuoteBytes-publicQuoteEnvelopeReserve {
		return gerror.New("public configuration quote exceeds response bound")
	}
	return nil
}
