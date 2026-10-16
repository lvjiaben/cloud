package catalog

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/model/entity"
)

// importLegacyOptionMapping is the only bridge from the legacy option_no
// price dimension to the structured schema. An option id is never exposed as
// a portable value by itself.
type importLegacyOptionMapping struct {
	OptionNo        string `orm:"option_no"`
	FieldKey        string `orm:"group_code"`
	NormalizedValue string `orm:"normalized_value"`
	Value           string `orm:"-"`
}

type importPriceGroup struct {
	rule     entity.CatalogBillingRule
	currency string
	base     []channelcommercecap.ImportPriceBand
	options  map[string]*channelcommercecap.ImportOptionPrice
	tax      entity.CatalogTaxPolicy
}

//nolint:cyclop // The projection is a fail-closed boundary and validates every persisted input before export.
func projectImportPrices(
	codec *publicConfigurationRevisionCodec,
	slug string,
	snapshot *publicStructuredConfigurationSnapshot,
	schema *importSchemaProjection,
	source *publicQuoteSource,
	dimensions []configurationPriceDimensionIdentity,
	wholePrices map[string]channelcommercecap.ImportWholeItemRule,
) ([]channelcommercecap.ImportPriceSchedule, error) {
	if codec == nil || slug == "" || snapshot == nil || schema == nil || source == nil ||
		snapshot.Target.VersionNo == "" || snapshot.Revision == "" || len(source.BillingRules) == 0 || len(source.Prices) == 0 {
		return nil, gerror.New("import price projection input is invalid")
	}
	rules := make(map[string]entity.CatalogBillingRule, len(source.BillingRules))
	for _, rule := range source.BillingRules {
		if rule.VersionNo != snapshot.Target.VersionNo || rule.BillingRuleNo == "" || ValidateBillingRule(BillingRuleDefinitionFromEntity(rule)) != nil {
			return nil, gerror.New("import billing rule is invalid")
		}
		if _, exists := rules[rule.BillingRuleNo]; exists {
			return nil, gerror.New("duplicate import billing rule")
		}
		rules[rule.BillingRuleNo] = rule
	}
	dimMap := make(map[string]configurationPriceDimensionIdentity, len(dimensions))
	for _, d := range dimensions {
		if d.DimensionNo == "" || d.FieldKey == "" || d.CandidateValue == "" || d.DimensionNo != strings.TrimSpace(d.DimensionNo) {
			return nil, gerror.New("import price dimension is invalid")
		}
		if _, exists := dimMap[d.DimensionNo]; exists {
			return nil, gerror.New("duplicate import price dimension")
		}
		if _, ok := schema.fields[d.FieldKey]; !ok {
			return nil, gerror.New("import price dimension field is unavailable")
		}
		quantity := d.PricingMode == ConfigurationPricePerUnit || d.PricingMode == ConfigurationPriceBandTotal
		if !validConfigurationPriceMode(d.PricingMode) {
			return nil, gerror.New("invalid import configuration algorithm")
		}
		if quantity {
			field := configurationFieldByKey(snapshot.Schema, d.FieldKey)
			if field == nil || field.Type != capmodel.ConfigurationFieldInteger {
				return nil, gerror.New("quantity price requires integer schema")
			}
		}
		if !quantity && !schemaCandidateAvailable(snapshot.Schema, d.FieldKey, d.CandidateValue) {
			return nil, gerror.New("import price dimension candidate is unavailable")
		}
		dimMap[d.DimensionNo] = d
	}
	legacy := make(map[string]importLegacyOptionMapping, len(source.LegacyOptions))
	for _, row := range source.LegacyOptions {
		if row.OptionNo == "" || row.FieldKey == "" {
			return nil, gerror.New("legacy import option is invalid")
		}
		value, err := legacyOptionValue(row.NormalizedValue)
		if err != nil {
			return nil, err
		}
		row.Value = value
		if _, exists := legacy[row.OptionNo]; exists {
			return nil, gerror.New("duplicate legacy import option")
		}
		legacy[row.OptionNo] = row
	}
	taxes := make(map[string]entity.CatalogTaxPolicy, len(source.TaxPolicies))
	for _, tax := range source.TaxPolicies {
		if tax.VersionNo != "" && tax.VersionNo != snapshot.Target.VersionNo {
			return nil, gerror.New("import tax version mismatch")
		}
		if tax.CurrencyCode == "" || ValidateTaxPolicy(taxDefinitionFromEntity(tax)) != nil {
			return nil, gerror.New("import tax policy is invalid")
		}
		if _, exists := taxes[tax.CurrencyCode]; exists {
			return nil, gerror.New("duplicate import tax policy")
		}
		taxes[tax.CurrencyCode] = tax
	}
	groups := make(map[string]*importPriceGroup)
	for _, price := range source.Prices {
		if price.VersionNo != snapshot.Target.VersionNo || price.CurrencyCode == "" || price.BillingRuleNo == "" {
			return nil, gerror.New("import price is invalid")
		}
		if price.OptionNo != nil && *price.OptionNo == "" {
			return nil, gerror.New("import price option is invalid")
		}
		if price.OptionNo != nil && price.ConfigurationPriceDimensionNo != "" {
			return nil, gerror.New("import price dimensions are ambiguous")
		}
		rule, ok := rules[price.BillingRuleNo]
		if !ok {
			return nil, gerror.New("import price references unknown billing rule")
		}
		if _, ok := taxes[price.CurrencyCode]; !ok {
			return nil, gerror.New("import price tax policy is unavailable")
		}
		bands := channelcommercecap.ImportPriceBand{MinimumQuantity: price.MinimumQuantity, MaximumQuantity: cloneInt64Pointer(price.MaximumQuantity), AmountMinor: price.AmountMinor, SetupAmountMinor: price.SetupFeeMinor, RenewalAmountMinor: cloneInt64Pointer(price.RenewalAmountMinor)}
		key := price.BillingRuleNo + "\x00" + price.CurrencyCode
		group := groups[key]
		if group == nil {
			group = &importPriceGroup{rule: rule, currency: price.CurrencyCode, options: map[string]*channelcommercecap.ImportOptionPrice{}, tax: taxes[price.CurrencyCode]}
			groups[key] = group
		}
		if price.OptionNo == nil && price.ConfigurationPriceDimensionNo == "" {
			group.base = append(group.base, bands)
			continue
		}
		var field, value string
		mode := ConfigurationPriceFixed
		if price.ConfigurationPriceDimensionNo != "" {
			d, ok := dimMap[price.ConfigurationPriceDimensionNo]
			if !ok {
				return nil, gerror.New("import price references unknown dimension")
			}
			field, value = d.FieldKey, d.CandidateValue
			mode = d.PricingMode
		} else {
			row, ok := legacy[*price.OptionNo]
			if !ok {
				return nil, gerror.New("import price legacy option mapping is unavailable")
			}
			field, value = row.FieldKey, row.Value
		}
		if _, ok := schema.fields[field]; !ok {
			return nil, gerror.New("import price option field is unavailable")
		}
		if (mode == "" || mode == ConfigurationPriceFixed) && !schemaCandidateAvailable(snapshot.Schema, field, value) {
			return nil, gerror.New("import price option candidate is unavailable")
		}
		originalField := field
		if field = schema.fields[field]; field == "" {
			return nil, gerror.New("import price option field is invalid")
		}
		if mode == ConfigurationPricePerUnit || mode == ConfigurationPriceBandTotal {
			value = ""
		} else {
			var err error
			value, err = schema.optionKey(originalField, value)
			if err != nil {
				return nil, err
			}
		}
		optionKey := field + "\x00" + value
		if option := group.options[optionKey]; option != nil {
			option.Bands = append(option.Bands, bands)
		} else {
			group.options[optionKey] = &channelcommercecap.ImportOptionPrice{FieldKey: field, Value: value, PricingMode: channelcommercecap.ImportConfigurationPriceMode(mode), Bands: []channelcommercecap.ImportPriceBand{bands}}
		}
	}
	result := make([]channelcommercecap.ImportPriceSchedule, 0, len(groups))
	for _, group := range groups {
		if len(group.base) == 0 {
			return nil, gerror.New("import price base schedule is unavailable")
		}
		def := BillingRuleDefinitionFromEntity(group.rule)
		tiers := make([]PriceTierDefinition, len(group.base))
		for i, b := range group.base {
			tiers[i] = PriceTierDefinition{MinimumQuantity: b.MinimumQuantity, MaximumQuantity: cloneInt64Pointer(b.MaximumQuantity), AmountMinor: b.AmountMinor, SetupFeeMinor: b.SetupAmountMinor, RenewalAmountMinor: cloneInt64Pointer(b.RenewalAmountMinor)}
		}
		if ValidatePriceSchedule(def, tiers) != nil {
			return nil, gerror.New("import price schedule is invalid")
		}
		term, err := codec.encodeQuoteTerm(slug, snapshot.Target.VersionNo, group.rule.BillingRuleNo)
		if err != nil {
			return nil, err
		}
		usage := importUsage(def)
		options := make([]channelcommercecap.ImportOptionPrice, 0, len(group.options))
		for _, option := range group.options {
			option.Bands = sortBands(option.Bands)
			optionTiers := make([]PriceTierDefinition, len(option.Bands))
			for i, band := range option.Bands {
				optionTiers[i] = PriceTierDefinition{MinimumQuantity: band.MinimumQuantity, MaximumQuantity: cloneInt64Pointer(band.MaximumQuantity), AmountMinor: band.AmountMinor, SetupFeeMinor: band.SetupAmountMinor, RenewalAmountMinor: cloneInt64Pointer(band.RenewalAmountMinor)}
			}
			if validateConfigurationPriceSchedule(def, ConfigurationPriceMode(option.PricingMode), optionTiers) != nil {
				return nil, gerror.New("import option price schedule is invalid")
			}
			options = append(options, *option)
		}
		sort.Slice(options, func(i, j int) bool {
			if options[i].FieldKey == options[j].FieldKey {
				return options[i].Value < options[j].Value
			}
			return options[i].FieldKey < options[j].FieldKey
		})
		var whole *channelcommercecap.ImportWholeItemRule
		if rule, ok := wholePrices[importWholePriceKey(snapshot.Target.VersionNo, group.rule.BillingRuleNo, group.currency)]; ok {
			if err := rule.Validate(); err != nil {
				return nil, err
			}
			whole = &rule
		}
		result = append(result, channelcommercecap.ImportPriceSchedule{WholeItemRule: whole, CurrencyCode: group.currency, TermReference: term, ChargeType: string(def.ChargeType), PeriodUnit: string(def.PeriodUnit), PeriodCount: def.PeriodCount, PricePhase: string(def.PricePhase), QuantityMode: string(def.QuantityMode), TrialQuantity: def.TrialQuantity, MinimumPurchaseCycles: def.MinimumPurchaseCycles, MaximumRenewalCycles: cloneIntPointer(def.MaximumRenewalCycles), Usage: usage, Base: sortBands(group.base), Options: options, Tax: channelcommercecap.ImportTaxPolicy{PriceIncludesTax: group.tax.PriceIncludesTax, Treatment: group.tax.TaxTreatmentCode, RateBasisPoints: cloneIntPointer(group.tax.TaxRateBasisPoints), CategoryCode: group.tax.TaxCategoryCode, RoundingScope: group.tax.RoundingScopeCode, RoundingMode: group.tax.RoundingModeCode, RoundingQuantumMinor: group.tax.RoundingQuantumMinor}})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CurrencyCode == result[j].CurrencyCode {
			return result[i].TermReference < result[j].TermReference
		}
		return result[i].CurrencyCode < result[j].CurrencyCode
	})
	return result, nil
}

func legacyOptionValue(raw string) (string, error) {
	var obj map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &obj) != nil {
		return "", gerror.New("legacy option value is invalid")
	}
	var value string
	if data, ok := obj["value"]; ok && json.Unmarshal(data, &value) == nil && value != "" {
		return value, nil
	}
	return "", gerror.New("legacy option value field is unavailable")
}
func importUsage(def BillingRuleDefinition) *channelcommercecap.ImportUsagePolicy {
	if def.ChargeType != ChargeTypeUsage {
		return nil
	}
	return &channelcommercecap.ImportUsagePolicy{MeterUnit: def.MeterUnit, TierMode: def.TierMode, CollectionUnit: string(def.UsageCollectionUnit), CollectionCount: derefInt(def.UsageCollectionCount), RoundingMode: string(def.UsageRoundingMode), RoundingQuantity: derefInt64(def.UsageRoundingQuantity), MinimumQuantity: cloneInt64Pointer(def.UsageMinimumQuantity), MaximumQuantity: cloneInt64Pointer(def.UsageMaximumQuantity), MissingPolicy: string(def.UsageMissingDataPolicy)}
}
func sortBands(v []channelcommercecap.ImportPriceBand) []channelcommercecap.ImportPriceBand {
	sort.Slice(v, func(i, j int) bool { return v[i].MinimumQuantity < v[j].MinimumQuantity })
	return v
}
func derefInt(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}
func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
func schemaCandidateAvailable(schema *capmodel.ConfigurationSchemaDeclaration, fieldKey, value string) bool {
	field := configurationFieldByKey(schema, fieldKey)
	if field == nil || value == "" {
		return false
	}
	if len(field.StaticCandidates) == 0 {
		return field.DynamicCandidates != nil
	}
	for _, candidate := range field.StaticCandidates {
		if candidate != nil && candidate.Value == value {
			return true
		}
	}
	return false
}
