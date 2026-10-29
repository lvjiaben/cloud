// This file maps only owner-frozen native common selections. Supplier identity,
// callback secrets, procurement authority and retry correlation remain owned by
// the resource runtime; they cannot be set by the product or the browser.
package v10

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

var commonConfigFormKey = regexp.MustCompile(`^config_options\[configoption\]\[[1-9][0-9]*\](?:\[(?:0|[1-9][0-9]*)\])?$`)
var commonCustomFormKey = regexp.MustCompile(`^self_defined_field\[[1-9][0-9]*\]$`)
var regexpDecimal = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)?$`)
var commonProductResponseFields = []string{"status", "data", "product", "id", "name", "description", "self_defined_field", "field_name", "field_type", "regexpr", "field_option", "is_required"}

type commonPurchaseParameters struct {
	Workflow     string            `json:"workflow"`
	Module       string            `json:"native_module"`
	PID          string            `json:"pid"`
	BillingCycle string            `json:"billingcycle"`
	Form         map[string]string `json:"native_form"`
}

func readCommonCustomFields(raw json.RawMessage) ([]commonCustomField, error) {
	var envelope struct {
		Data struct {
			Fields []commonCustomField `json:"self_defined_field"`
		} `json:"data"`
	}
	if json.Unmarshal(raw, &envelope) != nil || len(envelope.Data.Fields) > 128 {
		return nil, errInvalidResponse
	}
	return envelope.Data.Fields, nil
}
func BuildNativePurchaseParameters(plan resourcecap.NativePurchasePlan) (json.RawMessage, error) {
	purchase := plan.Purchase
	if plan.Validate() != nil || plan.Protocol != productfulfillmentcap.SaleImportProtocolNativeV10CommonV1 || !positiveDecimal(purchase.PublicSlug) || !commonRevision.MatchString(purchase.ConfigurationRevision) || purchase.BillingTermToken != "free" && purchase.BillingTermToken != "onetime" && !positiveDecimal(purchase.BillingTermToken) {
		return nil, errInvalidPlan
	}
	form, err := commonConfigurationForm(purchase.Values)
	if err != nil {
		return nil, err
	}
	form["config_options[cycle]"] = purchase.BillingTermToken
	return json.Marshal(commonPurchaseParameters{Workflow: "console_cart_v1", Module: "idcsmart_common", PID: purchase.PublicSlug, BillingCycle: purchase.BillingTermToken, Form: form})
}
func commonConfigurationForm(values []channelcommercecap.PurchaseValue) (map[string]string, error) {
	form := map[string]string{}
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value.FieldToken] || commonValueArms(value) != 1 {
			return nil, errInvalidRequest
		}
		seen[value.FieldToken] = true
		parts := strings.Split(value.FieldToken, ":")
		if len(parts) < 2 || len(parts) > 3 || !positiveDecimal(parts[1]) {
			return nil, errInvalidRequest
		}
		switch parts[0] {
		case "config":
			if err := commonConfigFormValue(form, parts, value); err != nil {
				return nil, err
			}
		case "custom":
			if len(parts) != 2 {
				return nil, errInvalidRequest
			}
			text, err := commonCustomFormValue(value)
			if err != nil {
				return nil, err
			}
			form["self_defined_field["+parts[1]+"]"] = text
		default:
			return nil, errInvalidRequest
		}
	}
	return form, nil
}
func commonConfigFormValue(form map[string]string, parts []string, value channelcommercecap.PurchaseValue) error {
	key := "config_options[configoption][" + parts[1] + "]"
	if len(parts) == 3 {
		index, err := strconv.Atoi(parts[2])
		if err != nil || index < 0 || index >= 32 || strconv.Itoa(index) != parts[2] || value.Type != capmodel.ConfigurationFieldInteger {
			return errInvalidRequest
		}
		key += "[" + parts[2] + "]"
	}
	switch value.Type {
	case capmodel.ConfigurationFieldInteger:
		if value.IntegerValue == nil {
			return errInvalidRequest
		}
		number, err := strconv.ParseInt(*value.IntegerValue, 10, 64)
		if err != nil || number < 0 || strconv.FormatInt(number, 10) != *value.IntegerValue {
			return errInvalidRequest
		}
		// The source quantity filter accepts only arrays, including non-repeat fields.
		if len(parts) == 2 {
			key += "[0]"
		}
		form[key] = *value.IntegerValue
	case capmodel.ConfigurationFieldSingleSelect:
		if len(parts) != 2 || len(value.OptionTokens) != 1 || !positiveDecimal(value.OptionTokens[0]) {
			return errInvalidRequest
		}
		form[key] = value.OptionTokens[0]
	case capmodel.ConfigurationFieldMultiSelect:
		if len(parts) != 2 || len(value.OptionTokens) > 512 {
			return errInvalidRequest
		}
		tokens := map[string]bool{}
		for index, token := range value.OptionTokens {
			if !positiveDecimal(token) || tokens[token] {
				return errInvalidRequest
			}
			tokens[token] = true
			form[key+"["+strconv.Itoa(index)+"]"] = token
		}
	default:
		return errInvalidRequest
	}
	return nil
}
func commonCustomFormValue(value channelcommercecap.PurchaseValue) (string, error) {
	var text string
	switch value.Type {
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine:
		if value.TextValue == nil {
			return "", errInvalidRequest
		}
		text = *value.TextValue
	case capmodel.ConfigurationFieldSingleSelect:
		if len(value.OptionTokens) != 1 {
			return "", errInvalidRequest
		}
		text = value.OptionTokens[0]
	case capmodel.ConfigurationFieldBoolean:
		if value.BooleanValue == nil {
			return "", errInvalidRequest
		}
		text = "0"
		if *value.BooleanValue {
			text = "1"
		}
	default:
		return "", errInvalidRequest
	}
	if !utf8.ValidString(text) || len(text) > 8192 || strings.ContainsRune(text, 0) {
		return "", errInvalidRequest
	}
	return text, nil
}
