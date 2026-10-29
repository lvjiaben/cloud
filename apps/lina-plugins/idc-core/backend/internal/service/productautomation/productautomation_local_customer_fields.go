// Merchant-only fields share the typed purchase compiler and price dimensions,
// while their reserved identity keeps them out of provider fulfillment input.
package productautomation

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

const localCustomerFieldPrefix = "local_"

func selectCustomerSchema(exact *capmodel.BusinessExtensionProviderEvidence, selections []CustomerFieldSelection) (*capmodel.ConfigurationSchemaDeclaration, error) {
	if err := validateProductFieldBindings(exact, selections); err != nil {
		return nil, err
	}
	controlled, err := merchantCustomerFields(exact)
	if err != nil {
		return nil, err
	}
	var (
		providerSelections = make([]CustomerFieldSelection, 0, len(selections))
		localFields        = make([]*capmodel.ConfigurationFieldDeclaration, 0)
		known              = map[string]bool{}
		seen               = map[string]bool{}
	)
	if exact.CustomerConfiguration != nil {
		for _, field := range exact.CustomerConfiguration.Fields {
			if field != nil {
				known[field.Key] = true
			}
		}
	}
	if len(selections) > 128 {
		return nil, gerror.New("too many merchant fields")
	}
	for _, selection := range selections {
		if seen[selection.FieldKey] {
			return nil, gerror.New("duplicate merchant field")
		}
		seen[selection.FieldKey] = true
		if selection.LocalField != nil {
			if known[selection.FieldKey] {
				return nil, gerror.New("local field cannot replace provider field")
			}
			field, localErr := localCustomerField(selection)
			if localErr != nil {
				return nil, localErr
			}
			localFields = append(localFields, field)
		} else if controlled[selection.FieldKey] {
			providerSelections = append(providerSelections, selection)
		} else if !known[selection.FieldKey] || selection.Minimum != "" || selection.Maximum != "" {
			return nil, gerror.New("core fields allow presentation changes only")
		}
	}
	result, err := selectProviderCustomerSchema(exact, providerSelections)
	if err != nil {
		return nil, err
	}
	if result == nil && len(localFields) == 0 {
		return nil, nil
	}
	if result == nil {
		result = &capmodel.ConfigurationSchemaDeclaration{Version: "v1"}
	} else {
		encoded, encodeErr := json.Marshal(result)
		if encodeErr != nil {
			return nil, encodeErr
		}
		var cloned capmodel.ConfigurationSchemaDeclaration
		if cloneErr := json.Unmarshal(encoded, &cloned); cloneErr != nil {
			return nil, cloneErr
		}
		result = &cloned
	}
	if err = customizeCoreCustomerFields(result, controlled, selections); err != nil {
		return nil, err
	}
	result.Fields = append(result.Fields, localFields...)
	if len(result.Fields) > 128 {
		return nil, gerror.New("too many customer fields")
	}
	if _, err = configurationcontract.ValidateSchema(result, configurationSurfaceCustomer, exact.AuthorizedMethods); err != nil {
		return nil, err
	}
	return result, nil
}

func localCustomerField(selection CustomerFieldSelection) (*capmodel.ConfigurationFieldDeclaration, error) {
	field := selection.LocalField
	if !strings.HasPrefix(selection.FieldKey, localCustomerFieldPrefix) || len(selection.FieldKey) > 64 || len(selection.FieldKey) <= len(localCustomerFieldPrefix) || field.Key != selection.FieldKey || field.DynamicCandidates != nil || field.UploadPolicy != "" {
		return nil, gerror.New("invalid local field identity or external capability")
	}
	for _, r := range selection.FieldKey {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return nil, gerror.New("invalid local field key")
		}
	}
	switch field.Type {
	case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine, capmodel.ConfigurationFieldInteger, capmodel.ConfigurationFieldDecimal, capmodel.ConfigurationFieldBoolean, capmodel.ConfigurationFieldSingleSelect, capmodel.ConfigurationFieldMultiSelect, capmodel.ConfigurationFieldDate, capmodel.ConfigurationFieldDateTime, capmodel.ConfigurationFieldTreeSelect, capmodel.ConfigurationFieldAddress:
	default:
		return nil, gerror.New("unsupported local field type")
	}
	if field.Label == nil || strings.TrimSpace(field.Label.ZhCN) == "" || strings.TrimSpace(field.Label.EnUS) == "" || utf8.RuneCountInString(field.Label.ZhCN) > 128 || utf8.RuneCountInString(field.Label.EnUS) > 128 || len(field.StaticCandidates) > 512 {
		return nil, gerror.New("invalid local field label or choices")
	}
	encoded, err := json.Marshal(field)
	if err != nil {
		return nil, err
	}
	var result capmodel.ConfigurationFieldDeclaration
	if err = json.Unmarshal(encoded, &result); err != nil {
		return nil, err
	}
	if selection.DisplayNameZhCN != "" && selection.DisplayNameEnUS != "" {
		result.Label = &capmodel.LocalizedText{ZhCN: selection.DisplayNameZhCN, EnUS: selection.DisplayNameEnUS}
	}
	if result.Type == capmodel.ConfigurationFieldInteger {
		result.Minimum, result.Maximum = selection.Minimum, selection.Maximum
	}
	return &result, nil
}

func customizeCoreCustomerFields(result *capmodel.ConfigurationSchemaDeclaration, controlled map[string]bool, selections []CustomerFieldSelection) error {
	// Required interface fields retain their identity and technical constraints,
	// while the merchant may rename them like any other purchase option.
	for _, field := range result.Fields {
		for _, selection := range selections {
			if field == nil || field.Key != selection.FieldKey || selection.LocalField != nil || controlled[field.Key] {
				continue
			}
			if (strings.TrimSpace(selection.DisplayNameZhCN) == "") != (strings.TrimSpace(selection.DisplayNameEnUS) == "") || utf8.RuneCountInString(selection.DisplayNameZhCN) > 128 || utf8.RuneCountInString(selection.DisplayNameEnUS) > 128 {
				return gerror.New("invalid interface field display name")
			}
			if len(selection.CandidateValues) > 0 {
				if err := merchantChoiceSubset(field, selection); err != nil {
					return err
				}
			}
			if selection.DisplayNameZhCN != "" {
				field.Label = &capmodel.LocalizedText{ZhCN: strings.TrimSpace(selection.DisplayNameZhCN), EnUS: strings.TrimSpace(selection.DisplayNameEnUS)}
			}
		}
	}
	return nil
}
