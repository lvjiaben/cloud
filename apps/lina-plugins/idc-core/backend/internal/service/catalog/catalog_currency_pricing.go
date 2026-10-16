package catalog

import (
	"regexp"
	"sort"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

const maximumCurrencyPricingOptions = 20_000

type CurrencyPriceDimensionKind string

const (
	CurrencyPriceDimensionBase   CurrencyPriceDimensionKind = "base"
	CurrencyPriceDimensionOption CurrencyPriceDimensionKind = "option"
)

type CurrencyPriceIssueReason string

const (
	CurrencyPriceIssueMissing CurrencyPriceIssueReason = "missing"
	CurrencyPriceIssueInvalid CurrencyPriceIssueReason = "invalid"
)

type CurrencyPriceIssue struct {
	BillingRuleNo string
	Kind          CurrencyPriceDimensionKind
	OptionNo      string
	Reason        CurrencyPriceIssueReason
}

type CurrencyPricingResult struct {
	CurrencyCode string
	Sellable     bool
	Issues       []CurrencyPriceIssue
}

type currencyPriceDimension struct {
	billingRuleNo string
	currencyCode  string
	optionNo      string
}

type currencyPricingRule struct {
	billingRuleNo string
	definition    BillingRuleDefinition
}

type CurrencyPricingIndex struct {
	rules              []currencyPricingRule
	definitions        map[string]BillingRuleDefinition
	schedules          map[currencyPriceDimension][]PriceTierDefinition
	currencies         map[string]struct{}
	configurationModes map[string]ConfigurationPriceMode
}

var currencyCodePattern = regexp.MustCompile(`^[A-Z]{3}$`)

func NewCurrencyPricingIndex(
	billingRules []entity.CatalogBillingRule,
	prices []entity.CatalogPrice,
) (*CurrencyPricingIndex, error) {
	if len(billingRules) == 0 || len(billingRules) > maximumPublicationBillingRules ||
		len(prices) > maximumPublicationPrices {
		return nil, currencyPricingInvalid()
	}
	index := &CurrencyPricingIndex{
		rules:       make([]currencyPricingRule, 0, len(billingRules)),
		definitions: make(map[string]BillingRuleDefinition, len(billingRules)),
		schedules:   make(map[currencyPriceDimension][]PriceTierDefinition, len(prices)),
		currencies:  make(map[string]struct{}),
	}
	definitions := make(map[string]BillingRuleDefinition, len(billingRules))
	for _, rule := range billingRules {
		billingRuleNo := strings.TrimSpace(rule.BillingRuleNo)
		definition := BillingRuleDefinitionFromEntity(rule)
		if billingRuleNo == "" || rule.BillingRuleNo != billingRuleNo ||
			ValidateBillingRule(definition) != nil {
			return nil, currencyPricingInvalid()
		}
		if _, duplicate := definitions[billingRuleNo]; duplicate {
			return nil, currencyPricingInvalid()
		}
		definitions[billingRuleNo] = definition
		index.definitions[billingRuleNo] = definition
		index.rules = append(index.rules, currencyPricingRule{
			billingRuleNo: billingRuleNo, definition: definition,
		})
	}
	sort.Slice(index.rules, func(left, right int) bool {
		return index.rules[left].billingRuleNo < index.rules[right].billingRuleNo
	})
	for _, price := range prices {
		if _, activeRule := definitions[price.BillingRuleNo]; !activeRule {
			continue
		}
		if !currencyCodePattern.MatchString(price.CurrencyCode) {
			return nil, currencyPricingInvalid()
		}
		optionNo := ""
		if price.OptionNo != nil {
			optionNo = strings.TrimSpace(*price.OptionNo)
			if optionNo == "" || *price.OptionNo != optionNo {
				return nil, currencyPricingInvalid()
			}
		}
		dimension := currencyPriceDimension{
			billingRuleNo: price.BillingRuleNo,
			currencyCode:  price.CurrencyCode,
			optionNo:      optionNo,
		}
		index.schedules[dimension] = append(
			index.schedules[dimension], PriceTierDefinitionFromEntity(price),
		)
		index.currencies[price.CurrencyCode] = struct{}{}
	}
	return index, nil
}

func (i *CurrencyPricingIndex) Evaluate(
	currencyCode string,
	requiredOptionNumbers []string,
) (CurrencyPricingResult, error) {
	result := CurrencyPricingResult{CurrencyCode: currencyCode}
	if i == nil || !currencyCodePattern.MatchString(currencyCode) ||
		len(requiredOptionNumbers) > maximumCurrencyPricingOptions {
		return result, currencyPricingInvalid()
	}
	options := append([]string(nil), requiredOptionNumbers...)
	sort.Strings(options)
	for index, optionNo := range options {
		if strings.TrimSpace(optionNo) == "" || optionNo != strings.TrimSpace(optionNo) ||
			(index > 0 && optionNo == options[index-1]) {
			return result, currencyPricingInvalid()
		}
	}
	maximumIssues := len(i.rules) * (len(options) + 1)
	if maximumIssues <= 0 || maximumIssues > maximumPublicationPrices {
		return result, currencyPricingInvalid()
	}
	result.Issues = make([]CurrencyPriceIssue, 0)
	for _, rule := range i.rules {
		result.Issues = i.appendDimensionIssue(
			result.Issues, rule, currencyCode, "", CurrencyPriceDimensionBase,
		)
		for _, optionNo := range options {
			result.Issues = i.appendDimensionIssue(
				result.Issues, rule, currencyCode, optionNo, CurrencyPriceDimensionOption,
			)
		}
	}
	result.Sellable = len(result.Issues) == 0
	return result, nil
}

func (i *CurrencyPricingIndex) ConfiguredCurrencies() []string {
	if i == nil {
		return []string{}
	}
	result := make([]string, 0, len(i.currencies))
	for currencyCode := range i.currencies {
		result = append(result, currencyCode)
	}
	sort.Strings(result)
	return result
}

func (i *CurrencyPricingIndex) SellableCurrencies(requiredOptionNumbers []string) ([]string, error) {
	if i == nil {
		return nil, currencyPricingInvalid()
	}
	result := make([]string, 0, len(i.currencies))
	for _, currencyCode := range i.ConfiguredCurrencies() {
		currencyResult, err := i.Evaluate(currencyCode, requiredOptionNumbers)
		if err != nil {
			return nil, err
		}
		if currencyResult.Sellable {
			result = append(result, currencyCode)
		}
	}
	return result, nil
}

func (i *CurrencyPricingIndex) ExplicitSchedule(
	billingRuleNo string,
	currencyCode string,
	optionNo string,
) ([]PriceTierDefinition, bool) {
	if i == nil || strings.TrimSpace(billingRuleNo) == "" ||
		!currencyCodePattern.MatchString(currencyCode) ||
		billingRuleNo != strings.TrimSpace(billingRuleNo) || optionNo != strings.TrimSpace(optionNo) {
		return nil, false
	}
	definition, ruleExists := i.definitions[billingRuleNo]
	if !ruleExists {
		return nil, false
	}
	schedule, exists := i.schedules[currencyPriceDimension{
		billingRuleNo: billingRuleNo, currencyCode: currencyCode, optionNo: optionNo,
	}]
	if !exists || validateConfigurationPriceSchedule(definition, i.configurationModes[optionNo], schedule) != nil {
		return nil, false
	}
	return clonePriceSchedule(schedule), true
}

func (i *CurrencyPricingIndex) appendDimensionIssue(
	issues []CurrencyPriceIssue,
	rule currencyPricingRule,
	currencyCode string,
	optionNo string,
	kind CurrencyPriceDimensionKind,
) []CurrencyPriceIssue {
	schedule, exists := i.schedules[currencyPriceDimension{
		billingRuleNo: rule.billingRuleNo, currencyCode: currencyCode, optionNo: optionNo,
	}]
	if !exists {
		return append(issues, CurrencyPriceIssue{
			BillingRuleNo: rule.billingRuleNo, Kind: kind,
			OptionNo: optionNo, Reason: CurrencyPriceIssueMissing,
		})
	}
	if validateConfigurationPriceSchedule(rule.definition, i.configurationModes[optionNo], schedule) != nil {
		return append(issues, CurrencyPriceIssue{
			BillingRuleNo: rule.billingRuleNo, Kind: kind,
			OptionNo: optionNo, Reason: CurrencyPriceIssueInvalid,
		})
	}
	return issues
}

func currencyPricingInvalid() error {
	return bizerr.NewCode(CodeCurrencyPricingInvalid)
}

func clonePriceSchedule(schedule []PriceTierDefinition) []PriceTierDefinition {
	result := make([]PriceTierDefinition, 0, len(schedule))
	for _, tier := range schedule {
		tier.MaximumQuantity = cloneInt64Pointer(tier.MaximumQuantity)
		tier.RenewalAmountMinor = cloneInt64Pointer(tier.RenewalAmountMinor)
		result = append(result, tier)
	}
	return result
}
