// This file validates a source configuration before requesting account-specific
// whole-product prices. No local option-price editing or inferred zero schedule
// is introduced for an upstream module.
package v10

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	catalogtext "lina-plugin-idc-v10/manifest/i18n"
)

func executeCommonEvaluation(executor confidentialoutboundcap.Service, contextID string, payload json.RawMessage, billing bool) (json.RawMessage, error) {
	var input channelcommercecap.ConfigurationInput
	if strictDecode(payload, &input) != nil || input.Validate() != nil || !positiveDecimal(input.PublicSlug) || !commonRevision.MatchString(input.ConfigurationRevision) {
		return nil, errInvalidRequest
	}
	raw, err := commonRequest(executor, contextID, 2, http.MethodGet, "/api/v1/product/"+input.PublicSlug, nil, commonProductResponseFields)
	if err != nil {
		return nil, err
	}
	fields, err := readCommonCustomFields(raw)
	if err != nil {
		return nil, err
	}
	catalog, supported, err := readCommonCatalog(executor, contextID, input.PublicSlug, 3, fields)
	if err != nil {
		return nil, err
	}
	if !supported || catalog.Configuration.ConfigurationRevision != input.ConfigurationRevision || input.CurrencyCode != "" && input.CurrencyCode != catalog.Currency {
		return nil, errInvalidRequest
	}
	values := input.Values
	if values == nil {
		values = catalog.Configuration.Values
	}
	issues := commonValidateValues(catalog, values)
	if billing {
		if len(issues) > 0 {
			return nil, errInvalidRequest
		}
		terms, termErr := commonBillingTerms(executor, contextID, 6, input.PublicSlug, catalog, values)
		if termErr != nil {
			return nil, termErr
		}
		return json.Marshal(terms)
	}
	catalog.Configuration.Values = values
	return json.Marshal(channelcommercecap.ConfigurationEvaluation{Configuration: catalog.Configuration, Valid: len(issues) == 0, Issues: issues})
}
func commonValidateValues(catalog *commonCatalog, values []channelcommercecap.PurchaseValue) []channelcommercecap.ConfigurationIssue {
	issues := []channelcommercecap.ConfigurationIssue{}
	fields := map[string]channelcommercecap.ConfigurationField{}
	seen := map[string]bool{}
	for _, field := range catalog.Configuration.Fields {
		fields[field.FieldToken] = field
	}
	for _, value := range values {
		field, exists := fields[value.FieldToken]
		if !exists || seen[value.FieldToken] || !commonValueValid(field, value) || !commonQuantityAvailable(catalog.Definition, value) {
			issues = append(issues, channelcommercecap.ConfigurationIssue{FieldToken: value.FieldToken, IssueCode: "invalid_value"})
		}
		seen[value.FieldToken] = true
	}
	for _, field := range catalog.Configuration.Fields {
		if field.Required && !seen[field.FieldToken] {
			issues = append(issues, channelcommercecap.ConfigurationIssue{FieldToken: field.FieldToken, IssueCode: "required"})
		}
	}
	return issues
}
func commonQuantityAvailable(def commonDefinition, value channelcommercecap.PurchaseValue) bool {
	if value.Type != capmodel.ConfigurationFieldInteger {
		return true
	}
	id := strings.Split(value.FieldToken, ":")[1]
	for _, option := range def.Options {
		if scalar(option.ID) != id {
			continue
		}
		number, err := strconv.ParseInt(*value.IntegerValue, 10, 64)
		if err != nil {
			return false
		}
		for _, sub := range option.Subs {
			minimum, minErr := strconv.ParseInt(scalar(sub.Minimum), 10, 64)
			maximum, maxErr := strconv.ParseInt(scalar(sub.Maximum), 10, 64)
			if minErr == nil && maxErr == nil && minimum <= number && number <= maximum {
				return true
			}
		}
		return false
	}
	return false
}
func commonValueValid(field channelcommercecap.ConfigurationField, value channelcommercecap.PurchaseValue) bool {
	if value.Type != field.Type || commonValueArms(value) != 1 {
		return false
	}
	switch field.Type {
	case capmodel.ConfigurationFieldInteger:
		if value.IntegerValue == nil {
			return false
		}
		number, err := strconv.ParseInt(*value.IntegerValue, 10, 64)
		minimum, _ := strconv.ParseInt(field.Constraints.Minimum, 10, 64)
		maximum, _ := strconv.ParseInt(field.Constraints.Maximum, 10, 64)
		return err == nil && strconv.FormatInt(number, 10) == *value.IntegerValue && number >= minimum && number <= maximum
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine:
		return commonTextValid(field, value.TextValue)
	case capmodel.ConfigurationFieldBoolean:
		return value.BooleanValue != nil && (!field.Required || *value.BooleanValue)
	case capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect:
		if field.Type == capmodel.ConfigurationFieldSingleSelect && len(value.OptionTokens) != 1 {
			return false
		}
		options := map[string]bool{}
		for _, option := range field.Options {
			options[option.OptionToken] = true
		}
		seen := map[string]bool{}
		for _, token := range value.OptionTokens {
			if !options[token] || seen[token] {
				return false
			}
			seen[token] = true
		}
		return true
	default:
		return false
	}
}
func commonValueArms(value channelcommercecap.PurchaseValue) int {
	count := 0
	for _, present := range []bool{value.TextValue != nil, value.IntegerValue != nil, value.DecimalValue != nil, value.BooleanValue != nil, value.DateValue != nil, value.DateTimeValue != nil, value.OptionTokens != nil, value.TreeOptionTokens != nil, value.AddressOptionTokens != nil, value.FileTokens != nil} {
		if present {
			count++
		}
	}
	return count
}
func commonBillingTerms(executor confidentialoutboundcap.Service, contextID string, sequence uint8, productID string, catalog *commonCatalog, values []channelcommercecap.PurchaseValue) (*channelcommercecap.BillingTerms, error) {
	form, err := commonConfigurationForm(values)
	if err != nil {
		return nil, err
	}
	form["is_downstream"] = "1"
	raw, err := commonRequest(executor, contextID, sequence, http.MethodPost, commonPath(productID)+"/calculate", form, commonResponseFields)
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Data commonDefinition `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil {
		return nil, errInvalidResponse
	}
	terms := &channelcommercecap.BillingTerms{ConfigurationRevision: catalog.Configuration.ConfigurationRevision, CurrencyCode: catalog.Currency, Terms: []channelcommercecap.BillingTerm{}}
	priced := map[string]commonCycle{}
	for _, cycle := range envelope.Data.CustomCycles {
		id := scalar(cycle.ID)
		if priced[id].ID != nil {
			return nil, errInvalidResponse
		}
		priced[id] = cycle
	}
	if _, exists := catalog.Definition.Cycles["free"]; exists && catalog.Definition.Product.PayType == "free" {
		terms.Terms = append(terms.Terms, commonOnceTerm("free", "0"))
	} else if _, exists := catalog.Definition.Cycles["onetime"]; exists && catalog.Definition.Product.PayType == "onetime" {
		amount, amountErr := commonAmount(envelope.Data.Cycles["onetime"], nil)
		if amountErr != nil {
			return nil, amountErr
		}
		terms.Terms = append(terms.Terms, commonOnceTerm("onetime", amount))
	}
	if catalog.Definition.Product.PayType == "free" || catalog.Definition.Product.PayType == "onetime" {
		if terms.Validate() != nil {
			return nil, errInvalidResponse
		}
		return terms, nil
	}
	for _, cycle := range catalog.Definition.CustomCycles {
		id := scalar(cycle.ID)
		price, exists := priced[id]
		if !positiveDecimal(id) || !exists {
			return nil, errInvalidResponse
		}
		term, termErr := commonRecurringTerm(catalog.Currency, cycle, price)
		if termErr != nil {
			return nil, termErr
		}
		terms.Terms = append(terms.Terms, term)
	}
	if terms.Validate() != nil {
		return nil, errInvalidResponse
	}
	return terms, nil
}
func commonOnceTerm(token, amount string) channelcommercecap.BillingTerm {
	label := channelcommercecap.LocalizedText{ZhCN: catalogtext.Render("zh-CN", "plugins.idc_v10.catalog.once", "One-time"), EnUS: catalogtext.Render("en-US", "plugins.idc_v10.catalog.once", "One-time")}
	if token == "free" {
		label = channelcommercecap.LocalizedText{ZhCN: catalogtext.Render("zh-CN", "plugins.idc_v10.catalog.free", "Free"), EnUS: catalogtext.Render("en-US", "plugins.idc_v10.catalog.free", "Free")}
	}
	return channelcommercecap.BillingTerm{BillingTermToken: token, Label: label, ChargeType: "one_time", PeriodUnit: "once", PeriodCount: 1, MinimumPurchaseCycles: 1, Status: "available", SetupAmount: "0", SubtotalAmount: amount, TaxAmount: "0", TotalAmount: amount}
}
func commonRecurringTerm(currency string, cycle, price commonCycle) (channelcommercecap.BillingTerm, error) {
	amount, err := commonAmount(price.Amount, price.Discount)
	if err != nil {
		return channelcommercecap.BillingTerm{}, err
	}
	count, countErr := strconv.Atoi(scalar(cycle.Time))
	unit := cycle.Unit
	// Source common customCycleTime uses hour/day/month. No display-label guessing.
	switch unit {
	case "hour", "day", "month":
	default:
		return channelcommercecap.BillingTerm{}, errInvalidResponse
	}
	schedule := productfulfillmentcap.BillingSchedule{CurrencyCode: currency, ChargeType: productfulfillmentcap.BillingChargeRecurring, PeriodUnit: productfulfillmentcap.BillingPeriodUnit(unit), PeriodCount: count, PurchaseCycles: 1}
	if countErr != nil || schedule.Validate() != nil {
		return channelcommercecap.BillingTerm{}, errInvalidResponse
	}
	label, labelErr := catalogPlainText(cycle.Name, 256, false)
	if labelErr != nil {
		return channelcommercecap.BillingTerm{}, labelErr
	}
	return channelcommercecap.BillingTerm{BillingTermToken: scalar(cycle.ID), Label: commonLabel(label), ChargeType: "recurring", PeriodUnit: unit, PeriodCount: count, MinimumPurchaseCycles: 1, Status: "available", SetupAmount: "0", SubtotalAmount: amount, TaxAmount: "0", TotalAmount: amount}, nil
}
func commonAmount(amount, discount json.RawMessage) (string, error) {
	parse := func(raw json.RawMessage) (*big.Int, error) {
		text := scalar(raw)
		if !regexpDecimal.MatchString(text) {
			return nil, errInvalidResponse
		}
		rational, ok := new(big.Rat).SetString(text)
		if !ok || rational.Sign() < 0 {
			return nil, errInvalidResponse
		}
		rational.Mul(rational, big.NewRat(100, 1))
		return new(big.Int).Quo(rational.Num(), rational.Denom()), nil
	}
	base, err := parse(amount)
	if err != nil {
		return "", err
	}
	if len(discount) > 0 && string(discount) != "null" {
		deduction, dedErr := parse(discount)
		if dedErr != nil {
			return "", dedErr
		}
		base.Sub(base, deduction)
	}
	if base.Sign() < 0 || !base.IsInt64() {
		return "", errInvalidResponse
	}
	return base.String(), nil
}

func commonTextValid(field channelcommercecap.ConfigurationField, value *string) bool {
	if value == nil || !utf8.ValidString(*value) || utf8.RuneCountInString(*value) > field.Constraints.MaximumLength || strings.ContainsRune(*value, 0) || field.Required && *value == "" {
		return false
	}
	if field.Constraints.Format == "url" && *value != "" {
		target, err := url.Parse(*value)
		return err == nil && target.Scheme != "" && target.Hostname() != ""
	}
	return true
}
