// This file translates native finance sales definitions into immutable portable
// contracts. Supplier IDs remain explicit mappings; unsupported restrictions or
// account transformations never become a fabricated complete sale contract.
package finance

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
	catalogtext "lina-plugin-idc-zjmf-finance/manifest/i18n"
)

type nativeCatalogDefinition struct {
	Flag         json.RawMessage              `json:"flag"`
	Prices       []map[string]json.RawMessage `json:"product_pricings"`
	Products     catalogWireProduct           `json:"products"`
	CustomFields []nativeCustomField          `json:"customfields"`
	Advanced     []nativeRelationWire         `json:"advanced"`
	Groups       []struct {
		Options []nativeCatalogOption `json:"options"`
	} `json:"config_groups"`
}

type nativeCatalogOption struct {
	ID      json.RawMessage       `json:"id"`
	Name    string                `json:"option_name"`
	Type    json.RawMessage       `json:"option_type"`
	Minimum json.RawMessage       `json:"qty_minimum"`
	Maximum json.RawMessage       `json:"qty_maximum"`
	Parent  json.RawMessage       `json:"linkage_pid"`
	Order   json.RawMessage       `json:"order"`
	Choices []nativeCatalogChoice `json:"sub"`
}

type nativeCatalogChoice struct {
	ID      json.RawMessage              `json:"id"`
	Name    string                       `json:"option_name"`
	Minimum json.RawMessage              `json:"qty_minimum"`
	Maximum json.RawMessage              `json:"qty_maximum"`
	Prices  []map[string]json.RawMessage `json:"pricings"`
	Parent  json.RawMessage              `json:"linkage_pid"`
	Order   json.RawMessage              `json:"sort_order"`
}

type nativeCustomField struct {
	ID         json.RawMessage `json:"id"`
	Name       string          `json:"fieldname"`
	Type       string          `json:"fieldtype"`
	Options    string          `json:"fieldoptions"`
	Required   json.RawMessage `json:"required"`
	Expression string          `json:"regexpr"`
}

// nativeImportSnapshot is absent unless all source purchase restrictions and
// every enabled component can be represented. It never guesses an account rate.
func nativeImportSnapshot(raw json.RawMessage, detail *resourceinterfaceprojectioncap.CatalogDetail) (*channelcommercecap.ImportSnapshot, error) {
	var envelope struct {
		Data nativeCatalogDefinition `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || detail == nil {
		return nil, errInvalidResponse
	}
	definition := envelope.Data
	_, knownFlag := nativeAccountFlag(definition.Flag)
	if !knownFlag {
		return nil, nil
	}
	_, _, billingComplete, billingErr := catalogProductPrices(definition.Products, json.RawMessage(`false`), definition.Prices)
	if billingErr != nil {
		return nil, billingErr
	}
	if !billingComplete || definition.CustomFields == nil || definition.Advanced == nil || len(definition.CustomFields) > 128 {
		return nil, nil
	}
	snapshot := &channelcommercecap.ImportSnapshot{
		ProtocolVersion: channelcommercecap.ImportRemoteEvaluationProtocolVersion,
		PricingMode:     channelcommercecap.ImportPricingRemoteEvaluation,
		Product:         channelcommercecap.CatalogProduct{PublicSlug: detail.Product.Reference, NameZhCN: detail.Product.Name, NameEnUS: detail.Product.Name, SummaryZhCN: detail.Product.Description, SummaryEnUS: detail.Product.Description},
		Schema:          &capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{}},
		Mappings:        []channelcommercecap.ImportFieldMapping{}, Currencies: []channelcommercecap.CatalogCurrency{}, Prices: []channelcommercecap.ImportPriceSchedule{}, PricingComplete: false,
		Fulfillment: channelcommercecap.ImportFulfillment{Granularity: productfulfillmentcap.GranularityPerQuantityUnit, OngoingService: true, RenewalMode: productfulfillmentcap.RenewalModeUnavailable, MaximumPurchaseCycles: 1},
	}
	trees, treeErr := nativeTrees(definition)
	if treeErr != nil {
		return nil, treeErr
	}
	treeRules := []*capmodel.ConfigurationRuleDeclaration{}
	for _, tree := range trees {
		field, mapping, rules, err := nativeTreeSchema(tree)
		if err != nil {
			return nil, err
		}
		snapshot.Schema.Fields = append(snapshot.Schema.Fields, field)
		snapshot.Mappings = append(snapshot.Mappings, mapping)
		treeRules = append(treeRules, rules...)
	}
	for _, group := range definition.Groups {
		for _, option := range group.Options {
			if scalar(option.Type) == "20" {
				continue
			}
			field, mapping, err := nativeOptionSchema(option)
			if err != nil {
				return nil, err
			}
			if field == nil {
				return nil, nil
			}
			snapshot.Schema.Fields = append(snapshot.Schema.Fields, field)
			snapshot.Mappings = append(snapshot.Mappings, mapping)
		}
	}
	for _, custom := range definition.CustomFields {
		field, mapping, err := nativeCustomSchema(custom)
		if err != nil {
			return nil, err
		}
		if field == nil {
			return nil, nil
		}
		snapshot.Schema.Fields = append(snapshot.Schema.Fields, field)
		snapshot.Mappings = append(snapshot.Mappings, mapping)
	}
	if len(snapshot.Schema.Fields) > 128 {
		return nil, errInvalidResponse
	}
	relations, supported, err := nativeAdvancedRules(definition.Advanced, snapshot.Schema)
	if err != nil {
		return nil, err
	}
	if !supported {
		return nil, nil
	}
	snapshot.Schema.Conditions = append(relations, treeRules...)
	if len(snapshot.Schema.Conditions) > 128 {
		return nil, nil
	}
	currencies := map[string]bool{}
	for _, base := range detail.BasePrices {
		schedule := nativeRemoteFinanceSchedule(base)
		if schedule == nil {
			return nil, errInvalidResponse
		}
		// Retain source-proven charging metadata when a published tariff is exactly
		// representable. Its amounts never leave this remote-price import contract.
		reference, err := nativePriceSchedule(base, definition)
		if err != nil {
			return nil, err
		}
		if reference != nil {
			schedule.PricePhase = reference.PricePhase
			schedule.QuantityMode = reference.QuantityMode
			schedule.Tax = reference.Tax
		}
		snapshot.Prices = append(snapshot.Prices, *schedule)

		if !currencies[base.CurrencyCode] {
			currencies[base.CurrencyCode] = true
			snapshot.Currencies = append(snapshot.Currencies, channelcommercecap.CatalogCurrency{CurrencyCode: base.CurrencyCode, Precision: 2})
		}
	}

	// Hash source configuration and charging semantics independently of live
	// account amounts. The full snapshot still retains its raw published tariffs.
	canonical := nativeRevisionSnapshot(snapshot)
	encoded, err := json.Marshal(struct {
		Snapshot channelcommercecap.ImportSnapshot
		Options  []nativeCatalogOption
	}{canonical, nativeRevisionOptions(definition)})
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(encoded)
	snapshot.ConfigurationRevision = hex.EncodeToString(digest[:])
	if resourcecap.ValidateNativeImportSnapshot(snapshot) != nil {
		return nil, errInvalidResponse
	}
	return snapshot, nil
}

func nativeOptionSchema(option nativeCatalogOption) (*capmodel.ConfigurationFieldDeclaration, channelcommercecap.ImportFieldMapping, error) {
	var (
		id      = scalar(option.ID)
		mapping = channelcommercecap.ImportFieldMapping{FieldKey: "cfg_" + id, FieldToken: "config:" + id, Options: []channelcommercecap.ImportOptionMapping{}}
	)
	if !positiveDecimal(id) || len(option.Choices) == 0 || len(option.Choices) > 512 {
		return nil, mapping, errInvalidResponse
	}
	name, err := catalogPlainText(option.Name, 256, false)
	if err != nil {
		return nil, mapping, err
	}
	if parent := scalar(option.Parent); parent != "" && parent != "0" {
		return nil, mapping, nil
	}
	field := &capmodel.ConfigurationFieldDeclaration{Key: mapping.FieldKey, Type: capmodel.ConfigurationFieldSingleSelect, Label: nativeLabel(name), Required: true}
	kind, err := strconv.Atoi(scalar(option.Type))
	if err != nil {
		return nil, mapping, nil
	}
	switch kind {
	case 1, 2, 5, 6, 8, 10, 12, 13:
	case 3:
		field.StaticCandidates = append(field.StaticCandidates, &capmodel.ConfigurationCandidate{Value: "0", Label: &capmodel.LocalizedText{EnUS: catalogtext.Render("en-US", "plugins.idc_zjmf_finance.catalog.unselected", "No"), ZhCN: catalogtext.Render("zh-CN", "plugins.idc_zjmf_finance.catalog.unselected", "No")}})
		mapping.Options = append(mapping.Options, channelcommercecap.ImportOptionMapping{Value: "0", OptionToken: "0"})
	case 4, 7, 9, 11, 14, 15, 16, 17, 18, 19:
		minimum, minErr := nativeNonnegative(option.Minimum)
		maximum, maxErr := nativeNonnegative(option.Maximum)
		if minErr != nil || maxErr != nil || maximum < minimum || !nativeQuantityTopology(option, minimum, maximum) {
			return nil, mapping, errInvalidResponse
		}
		field.Type, field.Minimum, field.Maximum = capmodel.ConfigurationFieldInteger, strconv.FormatInt(minimum, 10), strconv.FormatInt(maximum, 10)
		return field, mapping, nil
	default:
		return nil, mapping, nil
	}
	for _, choice := range option.Choices {
		id := scalar(choice.ID)
		label, err := catalogPlainText(choice.Name, 256, false)
		if err != nil || !positiveDecimal(id) {
			return nil, mapping, errInvalidResponse
		}
		field.StaticCandidates = append(field.StaticCandidates, &capmodel.ConfigurationCandidate{Value: id, Label: nativeLabel(label)})
		mapping.Options = append(mapping.Options, channelcommercecap.ImportOptionMapping{Value: id, OptionToken: id})
	}
	return field, mapping, nil
}

func nativeCustomSchema(custom nativeCustomField) (*capmodel.ConfigurationFieldDeclaration, channelcommercecap.ImportFieldMapping, error) {
	var (
		id      = scalar(custom.ID)
		mapping = channelcommercecap.ImportFieldMapping{FieldKey: "custom_" + id, FieldToken: "custom:" + id, Options: []channelcommercecap.ImportOptionMapping{}}
	)
	name, err := catalogPlainText(custom.Name, 256, false)
	if err != nil || !positiveDecimal(id) {
		return nil, mapping, errInvalidResponse
	}
	if custom.Expression != "" {
		return nil, mapping, nil
	}
	field := &capmodel.ConfigurationFieldDeclaration{Key: mapping.FieldKey, Label: nativeLabel(name), Required: scalar(custom.Required) == "1", MaximumLength: 8192}
	switch custom.Type {
	case "text":
		field.Type = capmodel.ConfigurationFieldSingleLine
	case "textarea":
		field.Type = capmodel.ConfigurationFieldMultiLine
	case "dropdown":
		field.Type = capmodel.ConfigurationFieldSingleSelect
		field.MaximumLength = 0
		choices := strings.Split(custom.Options, ",")
		if len(choices) > 512 {
			return nil, mapping, errInvalidResponse
		}
		for _, value := range choices {
			label, err := catalogPlainText(value, 256, false)
			if err != nil || len(value) > 256 {
				return nil, mapping, errInvalidResponse
			}
			field.StaticCandidates = append(field.StaticCandidates, &capmodel.ConfigurationCandidate{Value: value, Label: nativeLabel(label)})
			mapping.Options = append(mapping.Options, channelcommercecap.ImportOptionMapping{Value: value, OptionToken: value})
		}
	default:
		return nil, mapping, nil
	}
	return field, mapping, nil
}

func nativeLabel(value string) *capmodel.LocalizedText {
	return &capmodel.LocalizedText{ZhCN: value, EnUS: value}
}

func nativeNonnegative(raw json.RawMessage) (int64, error) {
	value := scalar(raw)
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number < 0 || strconv.FormatInt(number, 10) != value {
		return 0, errInvalidResponse
	}
	return number, nil
}

func nativeRevisionSnapshot(snapshot *channelcommercecap.ImportSnapshot) channelcommercecap.ImportSnapshot {
	canonical := *snapshot
	canonical.Prices = append([]channelcommercecap.ImportPriceSchedule(nil), snapshot.Prices...)
	for index := range canonical.Prices {
		schedule := &canonical.Prices[index]
		schedule.Base = nativeRevisionBands(schedule.Base)
		schedule.Options = append([]channelcommercecap.ImportOptionPrice(nil), schedule.Options...)
		for choice := range schedule.Options {
			schedule.Options[choice].Bands = nativeRevisionBands(schedule.Options[choice].Bands)
		}
	}
	return canonical
}
func nativeRevisionBands(bands []channelcommercecap.ImportPriceBand) []channelcommercecap.ImportPriceBand {
	result := append([]channelcommercecap.ImportPriceBand(nil), bands...)
	for index := range result {
		result[index].AmountMinor = 0
		result[index].SetupAmountMinor = 0
		if result[index].RenewalAmountMinor != nil {
			zero := int64(0)
			result[index].RenewalAmountMinor = &zero
		}
	}
	return result
}

func nativeAccountFlag(raw json.RawMessage) (bool, bool) {
	if bytes.Equal(bytes.TrimSpace(raw), []byte("false")) {
		return false, true
	}
	var flag struct {
		Type json.RawMessage `json:"type"`
	}
	if json.Unmarshal(raw, &flag) != nil {
		return false, false
	}
	switch scalar(flag.Type) {
	case "1", "2", "3":
		return true, true
	}
	return false, false
}
func nativeRemoteFinanceSchedule(base resourceinterfaceprojectioncap.CatalogPrice) *channelcommercecap.ImportPriceSchedule {
	cycle, ok := nativeCycle(base)
	if !ok {
		return nil
	}
	schedule := &channelcommercecap.ImportPriceSchedule{CurrencyCode: base.CurrencyCode, TermReference: cycle.amount, ChargeType: "recurring", PeriodUnit: string(base.Period), PeriodCount: base.PeriodCount, PricePhase: "both", QuantityMode: "fixed", MinimumPurchaseCycles: 1, Base: []channelcommercecap.ImportPriceBand{}, Options: []channelcommercecap.ImportOptionPrice{}, Tax: channelcommercecap.ImportTaxPolicy{Treatment: "exempt", CategoryCode: "service", RoundingScope: "line", RoundingMode: "half_up", RoundingQuantumMinor: 1}}
	if base.Period == resourceinterfaceprojectioncap.CatalogPeriodOnce {
		schedule.ChargeType = "one_time"
		schedule.PeriodUnit = "once"
		schedule.PeriodCount = 1
		schedule.PricePhase = "purchase"
	}
	return schedule
}

func nativeRevisionOptions(definition nativeCatalogDefinition) []nativeCatalogOption {
	options := []nativeCatalogOption{}
	for _, group := range definition.Groups {
		for _, source := range group.Options {
			option := source
			option.Choices = append([]nativeCatalogChoice(nil), source.Choices...)
			for index := range option.Choices {
				option.Choices[index].Prices = nil
			}
			options = append(options, option)
		}
	}
	return options
}
