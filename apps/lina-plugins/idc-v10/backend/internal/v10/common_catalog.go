// This file adapts the explicitly identified IDCSmart common module. Its
// evaluated configuration is public; module archives and executable code never
// enter the connector. Published starting prices are never a static price table.
package v10

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/confidentialoutboundcap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

type commonDefinition struct {
	Product struct {
		PayType string `json:"pay_type"`
	} `json:"common_product"`
	Options      []commonOption             `json:"configoptions"`
	Cycles       map[string]json.RawMessage `json:"cycles"`
	CustomCycles []commonCycle              `json:"custom_cycles"`
}
type commonOption struct {
	ID            json.RawMessage `json:"id"`
	Name          string          `json:"option_name"`
	Type          string          `json:"option_type"`
	Description   string          `json:"description"`
	Minimum       json.RawMessage `json:"qty_min"`
	Maximum       json.RawMessage `json:"qty_max"`
	AllowRepeat   json.RawMessage `json:"allow_repeat"`
	MaximumRepeat json.RawMessage `json:"max_repeat"`
	Subs          []commonSub     `json:"subs"`
}
type commonSub struct {
	ID       json.RawMessage `json:"id"`
	Name     string          `json:"option_name"`
	Minimum  json.RawMessage `json:"qty_min"`
	Maximum  json.RawMessage `json:"qty_max"`
	OS       string          `json:"os"`
	Versions []commonSub     `json:"version"`
}
type commonCycle struct {
	ID       json.RawMessage `json:"id"`
	Name     string          `json:"name"`
	Time     json.RawMessage `json:"cycle_time"`
	Unit     string          `json:"cycle_unit"`
	Amount   json.RawMessage `json:"cycle_amount"`
	Discount json.RawMessage `json:"cycle_amount_client_level_discount"`
}
type commonCustomField struct {
	ID          json.RawMessage `json:"id"`
	Name        string          `json:"field_name"`
	Type        string          `json:"field_type"`
	Description string          `json:"description"`
	Pattern     string          `json:"regexpr"`
	Options     string          `json:"field_option"`
	Required    json.RawMessage `json:"is_required"`
}
type commonCatalog struct {
	Definition    commonDefinition
	Configuration channelcommercecap.Configuration
	Currency      string
}

var commonRevision = regexp.MustCompile(`^[a-f0-9]{64}$`)

var commonResponseFields = []string{"status", "data", "common_product", "pay_type", "configoptions", "id", "option_name", "option_type", "description", "qty_min", "qty_max", "allow_repeat", "max_repeat", "subs", "os", "version", "cycles", "onetime", "free", "custom_cycles", "name", "cycle_time", "cycle_unit", "cycle_amount", "cycle_amount_client_level_discount"}

func commonRequest(executor confidentialoutboundcap.Service, contextID string, sequence uint8, method, path string, form map[string]string, fields []string) (json.RawMessage, error) {
	request := resourcecap.ConfidentialHTTPRequest{Kind: resourcecap.ConfidentialHTTPKindRequest, Method: method, Path: path, Form: form, ResponseFields: fields, UseAuthentication: true}
	if method == http.MethodGet {
		request.Query = map[string][]string{}
		for key, value := range form {
			request.Query[key] = []string{value}
		}
		request.Form = nil
	}
	response, err := confidentialCall(executor, contextID, sequence, request)
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 || len(response.Data) > catalogMaxResponseBytes || nestedString(response.Data, "status") != "200" {
		return nil, errInvalidResponse
	}
	return response.Data, nil
}

func readCommonCatalog(executor confidentialoutboundcap.Service, contextID, productID string, sequence uint8, customFields []commonCustomField) (*commonCatalog, bool, error) {
	raw, err := commonRequest(executor, contextID, sequence, http.MethodGet, "/api/v1/product/"+productID+"/resource", nil, []string{"status", "data", "module", "version"})
	if err != nil {
		return nil, false, err
	}
	if nestedString(raw, "data", "module") != "idcsmart_common" {
		return nil, false, nil
	}
	currencyRaw, err := commonRequest(executor, contextID, sequence+1, http.MethodGet, "/api/v1/product", nil, []string{"status", "data", "currency_code"})
	if err != nil {
		return nil, false, err
	}
	currency := nestedString(currencyRaw, "data", "currency_code")
	if !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(currency) {
		return nil, false, errInvalidResponse
	}
	raw, err = commonRequest(executor, contextID, sequence+2, http.MethodGet, commonPath(productID), map[string]string{"is_downstream": "1"}, commonResponseFields)
	if err != nil {
		return nil, false, err
	}
	var envelope struct {
		Data commonDefinition `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || envelope.Data.Options == nil || len(envelope.Data.Options) > 128 || len(envelope.Data.CustomCycles) > 120 {
		return nil, false, errInvalidResponse
	}
	configuration, supported, err := commonConfiguration(envelope.Data, customFields)
	if err != nil || !supported {
		return nil, supported, err
	}
	revision, revisionErr := commonSourceRevision(envelope.Data, customFields, currency)
	if revisionErr != nil {
		return nil, false, revisionErr
	}
	configuration.ConfigurationRevision = revision
	if configuration.Validate() != nil {
		return nil, false, errInvalidResponse
	}
	return &commonCatalog{Definition: envelope.Data, Configuration: configuration, Currency: currency}, true, nil
}
func commonPath(productID string) string {
	return "/console/v1/reidcsmart_common/product/" + productID + "/configoption"
}
func commonLabel(value string) channelcommercecap.LocalizedText {
	return channelcommercecap.LocalizedText{ZhCN: value, EnUS: value}
}

func commonConfiguration(def commonDefinition, custom []commonCustomField) (channelcommercecap.Configuration, bool, error) {
	switch def.Product.PayType {
	case "free", "onetime", "recurring_prepayment", "recurring_postpaid":
	default:
		return channelcommercecap.Configuration{}, false, nil
	}
	config := channelcommercecap.Configuration{Fields: []channelcommercecap.ConfigurationField{}, Values: []channelcommercecap.PurchaseValue{}}
	seen := map[string]bool{}
	for _, option := range def.Options {
		fields, values, supported, err := commonOptionFields(option)
		if err != nil || !supported {
			return config, supported, err
		}
		for _, field := range fields {
			if seen[field.FieldToken] {
				return config, false, errInvalidResponse
			}
			seen[field.FieldToken] = true
			config.Fields = append(config.Fields, field)
		}
		config.Values = append(config.Values, values...)
	}
	for _, field := range custom {
		projected, supported, err := commonCustomerField(field)
		if err != nil || !supported {
			return config, supported, err
		}
		if seen[projected.FieldToken] {
			return config, false, errInvalidResponse
		}
		seen[projected.FieldToken] = true
		config.Fields = append(config.Fields, projected)
	}
	if len(config.Fields) > 128 {
		return config, false, errInvalidResponse
	}
	return config, true, nil
}
func commonOptionFields(option commonOption) ([]channelcommercecap.ConfigurationField, []channelcommercecap.PurchaseValue, bool, error) {
	id := scalar(option.ID)
	if !positiveDecimal(id) || len(option.Subs) > 512 {
		return nil, nil, false, errInvalidResponse
	}
	label, err := catalogPlainText(option.Name, 256, false)
	if err != nil {
		return nil, nil, false, err
	}
	description, err := catalogPlainText(option.Description, 1024, true)
	if err != nil {
		return nil, nil, false, err
	}
	field := channelcommercecap.ConfigurationField{FieldToken: "config:" + id, Label: commonLabel(label), Description: commonLabel(description), Visible: true, Required: true, Options: []channelcommercecap.ConfigurationOption{}}
	switch option.Type {
	case "quantity", "quantity_range":
		return commonQuantityFields(field, option)
	case "select", "radio", "yes_no", "area", "os":
		field.Type = capmodel.ConfigurationFieldSingleSelect
	case "multi_select":
		field.Type = capmodel.ConfigurationFieldMultiSelect
		field.Required = false
	default:
		return nil, nil, false, nil
	}
	subs := option.Subs
	if option.Type == "os" {
		subs = nil
		for _, group := range option.Subs {
			for _, version := range group.Versions {
				version.Name = group.OS + " / " + version.Name
				subs = append(subs, version)
			}
		}
	}
	seen := map[string]bool{}
	for _, sub := range subs {
		token := scalar(sub.ID)
		name, nameErr := catalogPlainText(sub.Name, 256, false)
		if !positiveDecimal(token) || seen[token] || nameErr != nil {
			return nil, nil, false, errInvalidResponse
		}
		seen[token] = true
		field.Options = append(field.Options, channelcommercecap.ConfigurationOption{OptionToken: token, Label: commonLabel(name), Children: []channelcommercecap.ConfigurationOption{}})
	}
	if len(field.Options) == 0 {
		return nil, nil, false, errInvalidResponse
	}
	value := channelcommercecap.PurchaseValue{FieldToken: field.FieldToken, Type: field.Type, OptionTokens: []string{}}
	if field.Required {
		value.OptionTokens = []string{field.Options[0].OptionToken}
	}
	return []channelcommercecap.ConfigurationField{field}, []channelcommercecap.PurchaseValue{value}, true, nil
}
func commonQuantityFields(field channelcommercecap.ConfigurationField, option commonOption) ([]channelcommercecap.ConfigurationField, []channelcommercecap.PurchaseValue, bool, error) {
	minimum, err := strconv.ParseInt(scalar(option.Minimum), 10, 64)
	maximum, maxErr := strconv.ParseInt(scalar(option.Maximum), 10, 64)
	if err != nil || maxErr != nil || minimum < 0 || maximum < minimum || len(option.Subs) == 0 {
		return nil, nil, false, errInvalidResponse
	}
	// The first source quantity range defines its initial selection.
	initial, err := strconv.ParseInt(scalar(option.Subs[0].Minimum), 10, 64)
	if err != nil || initial < minimum || initial > maximum {
		return nil, nil, false, errInvalidResponse
	}
	count := 1
	if scalar(option.AllowRepeat) == "1" {
		n, parseErr := strconv.Atoi(scalar(option.MaximumRepeat))
		if parseErr != nil || n < 1 || n > 32 {
			return nil, nil, false, nil
		}
		count = n
	}
	field.Type = capmodel.ConfigurationFieldInteger
	field.Constraints.Minimum = strconv.FormatInt(minimum, 10)
	field.Constraints.Maximum = strconv.FormatInt(maximum, 10)
	fields := []channelcommercecap.ConfigurationField{}
	values := []channelcommercecap.PurchaseValue{}
	for index := 0; index < count; index++ {
		item := field
		if count > 1 {
			item.FieldToken += ":" + strconv.Itoa(index)
			item.Required = index == 0
			if index > 0 {
				item.Label = commonLabel(field.Label.ZhCN + " " + strconv.Itoa(index+1))
			}
		}
		fields = append(fields, item)
		if index == 0 {
			text := strconv.FormatInt(initial, 10)
			values = append(values, channelcommercecap.PurchaseValue{FieldToken: item.FieldToken, Type: item.Type, IntegerValue: &text})
		}
	}
	return fields, values, true, nil
}
func commonCustomerField(source commonCustomField) (channelcommercecap.ConfigurationField, bool, error) {
	field := channelcommercecap.ConfigurationField{FieldToken: "custom:" + scalar(source.ID), Visible: true, Required: scalar(source.Required) == "1", Options: []channelcommercecap.ConfigurationOption{}}
	if !positiveDecimal(scalar(source.ID)) {
		return field, false, errInvalidResponse
	}
	name, err := catalogPlainText(source.Name, 256, false)
	if err != nil {
		return field, false, err
	}
	field.Label = commonLabel(name)
	desc, err := catalogPlainText(source.Description, 1024, true)
	if err != nil {
		return field, false, err
	}
	field.Description = commonLabel(desc)
	if source.Pattern != "" {
		return field, false, nil
	}
	switch source.Type {
	case "text", "link":
		field.Type = capmodel.ConfigurationFieldSingleLine
		field.Constraints.MaximumLength = 500
		if source.Type == "link" {
			field.Constraints.Format = "url"
		}
	case "textarea":
		field.Type = capmodel.ConfigurationFieldMultiLine
		field.Constraints.MaximumLength = 500
	case "tickbox":
		field.Type = capmodel.ConfigurationFieldBoolean
		field.Required = false
	case "dropdown":
		field.Type = capmodel.ConfigurationFieldSingleSelect
		for _, value := range strings.Split(source.Options, ",") {
			if value == "" || len(value) > 256 {
				return field, false, errInvalidResponse
			}
			field.Options = append(field.Options, channelcommercecap.ConfigurationOption{OptionToken: value, Label: commonLabel(value), Children: []channelcommercecap.ConfigurationOption{}})
		}
	default:
		return field, false, nil
	}
	return field, true, nil
}
func attachCommonSnapshot(executor confidentialoutboundcap.Service, contextID string, detail *resourceinterfaceprojectioncap.CatalogDetail, custom []commonCustomField) error {
	catalog, supported, err := readCommonCatalog(executor, contextID, detail.Product.Reference, 3, custom)
	if err != nil {
		return err
	}
	if !supported {
		return nil
	}
	terms, err := commonBillingTerms(executor, contextID, 6, detail.Product.Reference, catalog, catalog.Configuration.Values)
	if err != nil {
		return err
	}
	snapshot := &channelcommercecap.ProductSnapshot{Product: channelcommercecap.CatalogProduct{PublicSlug: detail.Product.Reference, NameZhCN: detail.Product.Name, NameEnUS: detail.Product.Name, SummaryZhCN: detail.Product.Description, SummaryEnUS: detail.Product.Description}, Configuration: catalog.Configuration, ConfigurationMode: "remote_evaluation", Currencies: []channelcommercecap.CatalogCurrency{{CurrencyCode: catalog.Currency, Precision: 2}}, BillingTerms: []channelcommercecap.CurrencyBillingTerms{{CurrencyCode: catalog.Currency, Precision: 2, Terms: terms.Terms}}, PricingReady: true}
	if snapshot.Validate() != nil {
		return errInvalidResponse
	}
	detail.PeerPurchase = snapshot
	revision := snapshot.Configuration.ConfigurationRevision
	detail.ProductConfiguration = append(detail.ProductConfiguration, resourceinterfaceprojectioncap.CatalogConfigurationValue{FieldKey: "upstream_configuration_revision", TextValue: &revision})
	return nil
}

// commonSourceRevision retains billing identity and constraints while prices
// remain live. A changed source amount must not impersonate a schema change.
func commonSourceRevision(definition commonDefinition, fields []commonCustomField, currency string) (string, error) {
	metadata := definition
	metadata.CustomCycles = append([]commonCycle(nil), definition.CustomCycles...)
	for index := range metadata.CustomCycles {
		metadata.CustomCycles[index].Amount = nil
		metadata.CustomCycles[index].Discount = nil
	}
	metadata.Cycles = map[string]json.RawMessage{}
	for token := range definition.Cycles {
		metadata.Cycles[token] = json.RawMessage(`true`)
	}
	encoded, err := json.Marshal(struct {
		Definition commonDefinition
		Fields     []commonCustomField
		Currency   string
	}{metadata, fields, currency})
	if err != nil {
		return "", errInvalidResponse
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}
