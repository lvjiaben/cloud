// This file isolates explicit saved price schedule gaps per staff-preview period.
// It reuses checkout validation and term calculation, without changing checkout
// sellability or suppressing tax, arithmetic, authorization or upstream errors.

package catalog

import (
	"sort"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

func buildProductEditorPriceQuote(currency string, source *publicQuoteSource, selections map[string]int) (*PublicConfigurationQuote, map[string][]CurrencyPriceIssue, error) {
	if source == nil || len(source.TaxPolicies) != 1 {
		return nil, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	policy := taxDefinitionFromEntity(source.TaxPolicies[0])
	if source.TaxPolicies[0].CurrencyCode != currency || ValidateTaxPolicy(policy) != nil {
		return nil, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
	}
	rules := make([]entity.CatalogBillingRule, 0, len(source.BillingRules))
	lineCount := 0
	for _, rule := range source.BillingRules {
		definition := BillingRuleDefinitionFromEntity(rule)
		if definition.PricePhase == PricePhaseRenewal {
			continue
		}
		rules = append(rules, rule)
		if definition.ChargeType == ChargeTypeTrial || definition.QuantityMode == QuantityModeUsage {
			continue
		}
		if len(selections)+1 > maximumPublicQuoteLines-lineCount {
			return nil, nil, bizerr.NewCode(CodePublicQuoteNotAvailable)
		}
		lineCount += len(selections) + 1
	}
	pricing, err := NewCurrencyPricingIndex(rules, source.Prices)
	if err != nil {
		return nil, nil, err
	}
	pricing.configurationModes = source.ConfigurationModes
	options := make([]string, 0, len(selections))
	for option := range selections {
		options = append(options, option)
	}
	sort.Strings(options)
	availability, err := pricing.Evaluate(currency, options)
	if err != nil {
		return nil, nil, err
	}
	issues := make(map[string][]CurrencyPriceIssue)
	for _, issue := range availability.Issues {
		issues[issue.BillingRuleNo] = append(issues[issue.BillingRuleNo], issue)
	}
	quote := &PublicConfigurationQuote{CurrencyCode: currency, Tax: publicTaxSnapshot(policy), Terms: make([]PublicQuoteTerm, 0, len(rules))}
	sort.Slice(rules, func(i, j int) bool { return rules[i].BillingRuleNo < rules[j].BillingRuleNo })
	for _, rule := range rules {
		if len(issues[rule.BillingRuleNo]) > 0 {
			definition := BillingRuleDefinitionFromEntity(rule)
			quote.Terms = append(quote.Terms, PublicQuoteTerm{TermNo: rule.BillingRuleNo, Status: productEditorPriceUnavailable, ChargeType: definition.ChargeType, PeriodUnit: definition.PeriodUnit, PeriodCount: definition.PeriodCount, Cycles: definition.MinimumPurchaseCycles})
			continue
		}
		term, buildErr := buildPublicQuoteTerm(rule, pricing, currency, policy, options, selections)
		if buildErr != nil {
			return nil, nil, buildErr
		}
		quote.Terms = append(quote.Terms, term)
	}
	return quote, issues, nil
}
