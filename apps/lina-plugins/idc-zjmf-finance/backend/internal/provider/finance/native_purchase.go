// This file restores native cart parameters from owner-frozen imported field
// mappings. It does not infer CPU/region roles, credentials, customer identity,
// quantity or a supplier price from display labels.
package finance

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

// BuildNativePurchaseParameters restores one paid quantity unit. The caller
// must supply the immutable imported configuration and billing-term mapping.
// Callback correlation and secret slots are injected later by the resource owner.
func BuildNativePurchaseParameters(ctx context.Context, purchase channelcommercecap.Purchase) (json.RawMessage, error) {
	if ctx == nil || ctx.Err() != nil || !positiveDecimal(purchase.PublicSlug) || len(purchase.ConfigurationRevision) != 64 || purchase.Quantity != 1 || purchase.PurchaseCycles != 1 || len(purchase.Values) > 128 || !nativeBillingCycle(purchase.BillingTermToken) {
		return nil, errInvalidConfig
	}
	if _, err := hex.DecodeString(purchase.ConfigurationRevision); err != nil || strings.ToLower(purchase.ConfigurationRevision) != purchase.ConfigurationRevision {
		return nil, errInvalidConfig
	}
	value := purchaseParameters{Workflow: "serialized_cart_v1", ProductID: purchase.PublicSlug, BillingCycle: purchase.BillingTermToken, Quantity: 1, ConfigOptions: map[string]string{}, CustomFields: map[string]string{}}
	seen := map[string]bool{}
	for _, field := range purchase.Values {
		if seen[field.FieldToken] {
			return nil, errInvalidConfig
		}
		seen[field.FieldToken] = true
		kind, id, found := strings.Cut(field.FieldToken, ":")
		if !found || !positiveDecimal(id) {
			return nil, errInvalidConfig
		}
		selected, err := nativePurchaseValue(field)
		if err != nil {
			return nil, err
		}
		switch kind {
		case "config":
			if field.Type == capmodel.ConfigurationFieldTreeSelect {
				branch, branchErr := nativeTreePath(selected, id)
				if branchErr != nil {
					return nil, branchErr
				}
				for key, choice := range branch {
					if _, exists := value.ConfigOptions[key]; exists {
						return nil, errInvalidConfig
					}
					value.ConfigOptions[key] = choice
				}
				continue
			}
			if field.Type != capmodel.ConfigurationFieldSingleSelect && field.Type != capmodel.ConfigurationFieldInteger {
				return nil, errInvalidConfig
			}
			if _, err = nativeNonnegative(json.RawMessage(`"` + selected + `"`)); err != nil {
				return nil, errInvalidConfig
			}
			value.ConfigOptions[id] = selected
		case "custom":
			if !utf8.ValidString(selected) || len(selected) > 8192 || strings.ContainsRune(selected, '\x00') {
				return nil, errInvalidConfig
			}
			value.CustomFields[id] = selected
		default:
			return nil, errInvalidConfig
		}
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 1<<20 {
		return nil, errInvalidConfig
	}
	return raw, nil
}

func nativePurchaseValue(field channelcommercecap.PurchaseValue) (string, error) {
	arms := 0
	if field.TextValue != nil {
		arms++
	}
	if field.IntegerValue != nil {
		arms++
	}
	if field.DecimalValue != nil {
		arms++
	}
	if field.BooleanValue != nil {
		arms++
	}
	if field.DateValue != nil {
		arms++
	}
	if field.DateTimeValue != nil {
		arms++
	}
	if len(field.OptionTokens) > 0 {
		arms++
	}
	if len(field.TreeOptionTokens) > 0 {
		arms++
	}
	if len(field.AddressOptionTokens) > 0 {
		arms++
	}
	if len(field.FileTokens) > 0 {
		arms++
	}
	if arms != 1 {
		return "", errInvalidConfig
	}
	switch field.Type {
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine:
		if field.TextValue != nil {
			return *field.TextValue, nil
		}
	case capmodel.ConfigurationFieldInteger:
		if field.IntegerValue != nil {
			return *field.IntegerValue, nil
		}
	case capmodel.ConfigurationFieldTreeSelect:
		if len(field.TreeOptionTokens) == 1 {
			return field.TreeOptionTokens[0], nil
		}
	case capmodel.ConfigurationFieldSingleSelect:
		if len(field.OptionTokens) == 1 {
			return field.OptionTokens[0], nil
		}
	}
	return "", errInvalidConfig
}

func nativeBillingCycle(value string) bool {
	for _, cycle := range catalogCycles {
		if cycle.amount == value {
			return true
		}
	}
	return false
}
