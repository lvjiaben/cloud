// This file converts a validated native evaluated view into a version-fenced
// import declaration. Its schedules contain only billing metadata: no amount
// from the default selection can become an arbitrary configuration price table.
package resourcecap

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
)

// NativeRemoteImportSnapshot projects the explicitly reviewed common-module
// protocol. The product owner must request live billing terms for each selected
// configuration and preserve the remote mode across nested resale boundaries.
func NativeRemoteImportSnapshot(view *channelcommercecap.ProductSnapshot) (*channelcommercecap.ImportSnapshot, error) {
	if err := ValidateNativeRemoteSnapshot(view); err != nil {
		return nil, err
	}
	snapshot := &channelcommercecap.ImportSnapshot{ProtocolVersion: channelcommercecap.ImportRemoteEvaluationProtocolVersion, Product: view.Product, ConfigurationRevision: view.Configuration.ConfigurationRevision, Schema: &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{}}, Mappings: []channelcommercecap.ImportFieldMapping{}, Currencies: append([]channelcommercecap.CatalogCurrency(nil), view.Currencies...), Prices: []channelcommercecap.ImportPriceSchedule{}, PricingComplete: false, PricingMode: channelcommercecap.ImportPricingRemoteEvaluation, Fulfillment: channelcommercecap.ImportFulfillment{MaximumPurchaseCycles: 1, Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingService: true, RenewalMode: productfulfillmentcap.RenewalModeUnavailable}}
	fieldTokens := map[string]bool{}
	for _, field := range view.Configuration.Fields {
		fieldTokens[field.FieldToken] = true
	}
	defaults := map[string]channelcommercecap.PurchaseValue{}
	for _, value := range view.Configuration.Values {
		if _, duplicate := defaults[value.FieldToken]; duplicate || !fieldTokens[value.FieldToken] {
			return nil, gerror.New("native default mapping is invalid")
		}
		defaults[value.FieldToken] = value
	}
	for _, source := range view.Configuration.Fields {
		field, mapping := nativeRemoteImportField(source)
		if value, exists := defaults[source.FieldToken]; exists {
			raw, err := nativeRemoteDefault(value)
			if err != nil {
				return nil, err
			}
			mapping.DefaultValue = raw
		}
		snapshot.Schema.Fields = append(snapshot.Schema.Fields, field)
		snapshot.Mappings = append(snapshot.Mappings, mapping)
	}
	for _, currency := range view.BillingTerms {
		for _, term := range currency.Terms {
			phase := "both"
			if term.ChargeType == "one_time" {
				phase = "new_purchase"
			}
			snapshot.Prices = append(snapshot.Prices, channelcommercecap.ImportPriceSchedule{CurrencyCode: currency.CurrencyCode, TermReference: term.BillingTermToken, ChargeType: term.ChargeType, PeriodUnit: term.PeriodUnit, PeriodCount: term.PeriodCount, MinimumPurchaseCycles: 1, PricePhase: phase, QuantityMode: "fixed", Base: []channelcommercecap.ImportPriceBand{}, Options: []channelcommercecap.ImportOptionPrice{}, Tax: channelcommercecap.ImportTaxPolicy{Treatment: "exempt", CategoryCode: "service", RoundingScope: "line", RoundingMode: "half_up", RoundingQuantumMinor: 1}})
		}
	}
	if snapshot.Validate() != nil {
		return nil, gerror.New("native remote import projection is invalid")
	}
	return snapshot, nil
}
func nativeRemoteImportField(source channelcommercecap.ConfigurationField) (*capmodel.ConfigurationFieldDeclaration, channelcommercecap.ImportFieldMapping) {
	key := strings.ReplaceAll(source.FieldToken, ":", "_")
	if strings.HasPrefix(key, "config_") {
		key = "cfg_" + strings.TrimPrefix(key, "config_")
	}
	field := &capmodel.ConfigurationFieldDeclaration{Key: key, Type: source.Type, Required: source.Required, Label: &capmodel.LocalizedText{ZhCN: source.Label.ZhCN, EnUS: source.Label.EnUS}, Description: &capmodel.LocalizedText{ZhCN: source.Description.ZhCN, EnUS: source.Description.EnUS}, Minimum: source.Constraints.Minimum, Maximum: source.Constraints.Maximum, MinimumLength: source.Constraints.MinimumLength, MaximumLength: source.Constraints.MaximumLength, TextFormat: source.Constraints.Format, StaticCandidates: []*capmodel.ConfigurationCandidate{}}
	mapping := channelcommercecap.ImportFieldMapping{FieldKey: key, FieldToken: source.FieldToken, Options: []channelcommercecap.ImportOptionMapping{}}
	for _, option := range source.Options {
		field.StaticCandidates = append(field.StaticCandidates, &capmodel.ConfigurationCandidate{Value: option.OptionToken, Label: &capmodel.LocalizedText{ZhCN: option.Label.ZhCN, EnUS: option.Label.EnUS}})
		mapping.Options = append(mapping.Options, channelcommercecap.ImportOptionMapping{Value: option.OptionToken, OptionToken: option.OptionToken})
	}
	return field, mapping
}
func nativeRemoteDefault(value channelcommercecap.PurchaseValue) (json.RawMessage, error) {
	var primitive any
	switch value.Type {
	case capmodel.ConfigurationFieldInteger:
		if value.IntegerValue == nil {
			return nil, gerror.New("native default integer is invalid")
		}
		number, err := strconv.ParseInt(*value.IntegerValue, 10, 64)
		if err != nil {
			return nil, gerror.New("native default integer is invalid")
		}
		primitive = number
	case capmodel.ConfigurationFieldSingleSelect:
		if len(value.OptionTokens) != 1 {
			return nil, gerror.New("native default selection is invalid")
		}
		primitive = value.OptionTokens[0]
	case capmodel.ConfigurationFieldMultiSelect:
		primitive = value.OptionTokens
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine:
		primitive = value.TextValue
	case capmodel.ConfigurationFieldBoolean:
		primitive = value.BooleanValue
	default:
		return nil, gerror.New("native default type is invalid")
	}
	return json.Marshal(primitive)
}
