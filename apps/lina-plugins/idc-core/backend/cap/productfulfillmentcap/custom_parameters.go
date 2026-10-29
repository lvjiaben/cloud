// This file defines explicit merchant-authored provisioning parameters. Names
// and scalar contents are frozen with the purchase; credentials stay separate.

package productfulfillmentcap

import (
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
)

// CustomParametersKey carries native API names separately from semantic configuration.
const CustomParametersKey = "custom_parameters"

// ValidateCustomParameterName accepts API field names, excluding the envelope
// itself and authentication inputs which use the confidential owner path.
func ValidateCustomParameterName(name string) error {
	if name == "" || len(name) > 128 || !utf8.ValidString(name) || strings.IndexFunc(name, func(value rune) bool { return unicode.IsControl(value) || unicode.IsSpace(value) }) >= 0 || name == CustomParametersKey {
		return gerror.New("custom provisioning parameter name is invalid")
	}
	normalized := strings.ToLower(strings.ReplaceAll(name, "_", ""))
	for _, reserved := range []string{"password", "secret", "token", "authorization", "cookie", "apikey", "accesskey", "privatekey"} {
		if strings.Contains(normalized, reserved) {
			return gerror.New("custom provisioning parameters cannot contain authentication fields")
		}
	}
	return nil
}

// CustomParameters decodes only the explicit envelope, never ordinary customer
// information. Missing data means no custom parameters, preserving old orders.
func CustomParameters(configuration json.RawMessage) (map[string]json.RawMessage, error) {
	if len(configuration) == 0 {
		return nil, nil
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(configuration, &envelope) != nil {
		return nil, gerror.New("custom provisioning parameter configuration is invalid")
	}
	raw, exists := envelope[CustomParametersKey]
	if !exists {
		return nil, nil
	}
	var values map[string]json.RawMessage
	if len(raw) > 65536 || json.Unmarshal(raw, &values) != nil || values == nil || len(values) > 128 {
		return nil, gerror.New("custom provisioning parameters exceed their bounds")
	}
	for key, rawValue := range values {
		if err := ValidateCustomParameterName(key); err != nil {
			return nil, err
		}
		var value any
		if len(rawValue) > 8192 || json.Unmarshal(rawValue, &value) != nil || value == nil {
			return nil, gerror.New("custom provisioning parameter content is invalid")
		}
		switch value.(type) {
		case string, float64, bool:
		default:
			return nil, gerror.New("custom provisioning parameter must be a scalar value")
		}
	}
	return values, nil
}
