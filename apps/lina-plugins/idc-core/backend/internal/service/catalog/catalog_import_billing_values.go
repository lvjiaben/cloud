// This file restores source-owned tokens from compiler-produced local values.
package catalog

import (
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"strconv"
)

// MapImportedBillingValues never accepts unknown keys, candidates or files.
func MapImportedBillingValues(snapshot *channelcommercecap.ImportSnapshot, values []configurationcontract.ConfigurationValue) ([]channelcommercecap.PurchaseValue, error) {
	result := make([]channelcommercecap.PurchaseValue, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		var mapping *channelcommercecap.ImportFieldMapping
		for index := range snapshot.Mappings {
			if snapshot.Mappings[index].FieldKey == value.FieldKey {
				mapping = &snapshot.Mappings[index]
				break
			}
		}
		field := configurationFieldByKey(snapshot.Schema, value.FieldKey)
		if mapping == nil || field == nil || seen[value.FieldKey] || len(value.FileTokens) > 0 {
			return nil, gerror.New("imported billing field invalid")
		}
		seen[value.FieldKey] = true
		out := channelcommercecap.PurchaseValue{FieldToken: mapping.FieldToken, Type: field.Type, TextValue: value.TextValue, IntegerValue: value.IntegerTextValue, DecimalValue: value.DecimalValue, BooleanValue: value.BooleanValue, DateValue: value.DateValue, DateTimeValue: value.DateTimeValue}
		if value.IntegerValue != nil {
			text := strconv.FormatInt(*value.IntegerValue, 10)
			out.IntegerValue = &text
		}
		options := map[string]string{}
		for _, option := range mapping.Options {
			options[option.Value] = option.OptionToken
		}
		for _, arm := range []struct {
			in  []string
			out *[]string
		}{{value.OptionTokens, &out.OptionTokens}, {value.TreeOptionTokens, &out.TreeOptionTokens}, {value.AddressOptionTokens, &out.AddressOptionTokens}} {
			for _, selected := range arm.in {
				token := options[selected]
				if token == "" {
					return nil, gerror.New("imported billing option invalid")
				}
				*arm.out = append(*arm.out, token)
			}
		}
		result = append(result, out)
	}
	return result, nil
}
