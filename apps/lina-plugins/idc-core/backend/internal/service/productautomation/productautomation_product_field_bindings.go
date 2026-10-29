// This file binds merchant-created purchase options to declared parameters or
// merchant-authored native API names. Bindings are explicit and frozen with the sale;
// ordinary customer information fields never acquire a provider parameter.
package productautomation

import (
	"context"
	"encoding/json"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/productfulfillmentcap"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func validateProductFieldBindings(exact *capmodel.BusinessExtensionProviderEvidence, selections []CustomerFieldSelection) error {
	var (
		targets = map[string]*capmodel.ConfigurationFieldDeclaration{}
		used    = map[string]bool{}
	)
	if exact.ProductConfiguration != nil {
		for _, field := range exact.ProductConfiguration.Fields {
			if field != nil {
				targets[field.Key] = field
			}
		}
	}
	for _, selection := range selections {
		if err := validateSelectionChoiceBindings(selection); err != nil {
			return err
		}
		if selection.ProductFieldKey == "" {
			continue
		}
		target := targets[selection.ProductFieldKey]
		field := selection.LocalField
		if target == nil {
			if field == nil || !strings.HasPrefix(selection.FieldKey, localCustomerFieldPrefix) || !field.Required || used[selection.ProductFieldKey] || productfulfillmentcap.ValidateCustomParameterName(selection.ProductFieldKey) != nil {
				return gerror.New("custom purchase parameter binding is invalid")
			}
			used[selection.ProductFieldKey] = true
			continue
		}
		if !validDeclaredProductBinding(selection, target, used[target.Key]) {
			return gerror.New("purchase parameter binding is invalid")
		}
		used[target.Key] = true
		for _, other := range selections {
			if other.LocalField == nil && (other.FieldKey == target.Key || customerProductFields(exact)[other.FieldKey] == target.Key) {
				return gerror.New("product parameter is already supplied by an interface purchase field")
			}
		}
		for _, option := range field.StaticCandidates {
			if option == nil {
				return gerror.New("bound purchase choice is invalid")
			}
		}
		switch target.Type {
		case capmodel.ConfigurationFieldInteger:
			narrowed := *target
			if err := narrowCustomerField(&narrowed, selection); err != nil {
				return err
			}
		case capmodel.ConfigurationFieldSingleSelect:
			if len(field.StaticCandidates) == 0 {
				return gerror.New("bound purchase choices are empty")
			}
			if target.DynamicCandidates == nil {
				allowed := map[string]bool{}
				for _, option := range target.StaticCandidates {
					if option != nil {
						allowed[option.Value] = true
					}
				}
				for _, option := range field.StaticCandidates {
					if option == nil || !allowed[configurationcontract.ProductChoiceValue(selection.ProductChoiceBindings, option.Value)] {
						return gerror.New("bound purchase choice is not declared by the interface")
					}
				}
			}
		case capmodel.ConfigurationFieldBoolean:
		case capmodel.ConfigurationFieldSingleLine, capmodel.ConfigurationFieldMultiLine:
			if err := validateBoundTextField(mappedProductChoices(selection), target, slices.Contains(productChoiceFields(exact), target.Key)); err != nil {
				return err
			}
		default:
			return gerror.New("interface parameter cannot be bound to a purchase option")
		}
	}
	return nil
}

// boundMerchantValues derives the internal base from the first sellable choice.
// It changes only explicitly bound parameters, leaving legacy product facts intact.
func boundMerchantValues(exact *capmodel.BusinessExtensionProviderEvidence, values []ConfigurationValue, selections []CustomerFieldSelection) ([]ConfigurationValue, error) {
	if err := validateProductFieldBindings(exact, selections); err != nil {
		return nil, err
	}
	result := append([]ConfigurationValue(nil), values...)
	for _, selection := range selections {
		if selection.ProductFieldKey == "" {
			continue
		}
		// Native custom fields are compiled from the final customer selection
		// at fulfillment, not inserted into the connector's declared base schema.
		declared := false
		if exact.ProductConfiguration != nil {
			for _, target := range exact.ProductConfiguration.Fields {
				if target != nil && target.Key == selection.ProductFieldKey {
					declared = true
					break
				}
			}
		}
		if !declared {
			continue
		}
		field := selection.LocalField
		value := ConfigurationValue{FieldKey: selection.ProductFieldKey}
		switch field.Type {
		case capmodel.ConfigurationFieldInteger:
			first := selection.Minimum
			if len(selection.NumericChoices) > 0 {
				first = selection.NumericChoices[0]
			}
			if _, err := strconv.ParseInt(first, 10, 64); err != nil {
				return nil, err
			}
			value.IntegerTextValue = &first
		case capmodel.ConfigurationFieldSingleSelect:
			value.OptionTokens = []string{configurationcontract.ProductChoiceValue(selection.ProductChoiceBindings, field.StaticCandidates[0].Value)}
			for _, target := range exact.ProductConfiguration.Fields {
				if target.Key == selection.ProductFieldKey && target.Type == capmodel.ConfigurationFieldSingleLine {
					first := configurationcontract.ProductChoiceValue(selection.ProductChoiceBindings, field.StaticCandidates[0].Value)
					value.OptionTokens = nil
					value.TextValue = &first
				}
			}
		case capmodel.ConfigurationFieldBoolean:
			first := false
			value.BooleanValue = &first
		default:
			continue
		}
		found := false
		for index := range result {
			if result[index].FieldKey == value.FieldKey {
				result[index] = value
				found = true
				break
			}
		}
		if !found {
			result = append(result, value)
		}
	}
	return result, nil
}

// validateProductBoundCandidates uses the same batched local projection as the
// product form; it never starts one upstream request per merchant option.
func (s *serviceImpl) validateProductBoundCandidates(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, input SaveVersionConfigurationInput, scope dynamicCandidateScope) error {
	var dynamic []*capmodel.ConfigurationFieldDeclaration
	if exact.ProductConfiguration != nil {
		for _, target := range exact.ProductConfiguration.Fields {
			if target != nil && target.DynamicCandidates != nil {
				for _, selection := range input.CustomerFieldSelections {
					if selection.ProductFieldKey == target.Key {
						dynamic = append(dynamic, target)
						break
					}
				}
			}
		}
	}
	if len(dynamic) == 0 {
		return nil
	}
	resolved, err := s.resolveDynamicCandidates(ctx, exact, exact.ProductConfiguration, input.MerchantValues, configurationSurfaceProduct, scope)
	if err != nil {
		return err
	}
	for _, target := range dynamic {
		allowed := map[string]bool{}
		for _, option := range resolved[target.Key] {
			if option != nil {
				allowed[option.Value] = true
			}
		}
		for _, selection := range input.CustomerFieldSelections {
			if selection.ProductFieldKey != target.Key {
				continue
			}
			for _, option := range selection.LocalField.StaticCandidates {
				if option == nil || !allowed[configurationcontract.ProductChoiceValue(selection.ProductChoiceBindings, option.Value)] {
					return gerror.New("bound purchase choice is absent from the supplier directory")
				}
			}
		}
	}
	return nil
}

// selectEditableCustomerSchema checks the local declaration and any imported
// product choices together, before the product version transaction starts.
func (s *serviceImpl) selectEditableCustomerSchema(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, input SaveVersionConfigurationInput, scope dynamicCandidateScope, merchant json.RawMessage) (*capmodel.ConfigurationSchemaDeclaration, error) {
	schema, err := selectCustomerSchema(exact, input.CustomerFieldSelections)
	if err != nil {
		return nil, err
	}
	if err = s.validateProductBoundCandidates(ctx, exact, input, scope); err != nil {
		return nil, err
	}
	if err = s.validateCustomerChoiceCandidates(ctx, exact, input, scope); err != nil {
		return nil, err
	}
	if err = s.validateBoundTextChoices(ctx, exact, merchant, input.CustomerFieldSelections); err != nil {
		return nil, err
	}
	return schema, nil
}

// validateBoundTextChoices sends all profile alternatives to the provider once;
// no alternative can bypass the provider's pure cross-field validation.
func (s *serviceImpl) validateBoundTextChoices(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, values json.RawMessage, selections []CustomerFieldSelection) error {
	if exact.ProductConfiguration == nil {
		return nil
	}
	choices := map[string][]json.RawMessage{}
	count := 0
	for _, selection := range selections {
		if selection.ProductFieldKey == "" || selection.LocalField == nil || selection.LocalField.Type != capmodel.ConfigurationFieldSingleSelect {
			continue
		}
		for _, target := range exact.ProductConfiguration.Fields {
			if target.Key != selection.ProductFieldKey || target.Type != capmodel.ConfigurationFieldSingleLine {
				continue
			}
			for _, choice := range selection.LocalField.StaticCandidates {
				count++
				if count > 512 {
					return gerror.New("too many bound text alternatives")
				}
				raw, err := json.Marshal(configurationcontract.ProductChoiceValue(selection.ProductChoiceBindings, choice.Value))
				if err != nil {
					return err
				}
				choices[target.Key] = append(choices[target.Key], raw)
			}
		}
	}
	if len(choices) == 0 {
		return nil
	}
	return s.validateProviderProductConfiguration(ctx, exact, values, choices)
}

func validateBoundTextField(field, target *capmodel.ConfigurationFieldDeclaration, choiceValidation bool) error {
	if field.Type == capmodel.ConfigurationFieldSingleSelect {
		if !choiceValidation || len(field.StaticCandidates) == 0 {
			return gerror.New("bound text choices are empty or unsupported")
		}
		for _, choice := range field.StaticCandidates {
			if utf8.RuneCountInString(choice.Value) < target.MinimumLength || (target.MaximumLength > 0 && utf8.RuneCountInString(choice.Value) > target.MaximumLength) {
				return gerror.New("bound text choice exceeds declared length")
			}
		}
		return nil
	}
	if field.TextFormat != target.TextFormat || field.MinimumLength < target.MinimumLength || field.MaximumLength <= 0 || (target.MaximumLength > 0 && field.MaximumLength > target.MaximumLength) {
		return gerror.New("bound text exceeds interface constraints")
	}
	return nil
}

// customerProductFields exposes declared override identities, not conversion
// code. The selected provider remains the sole owner of units and wire mapping.
func customerProductFields(exact *capmodel.BusinessExtensionProviderEvidence) map[string]string {
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(exact.ProviderMetadata, exact.AuthorizedMethods)
	if err != nil {
		return nil
	}
	return declaration.CustomerProductFields
}

func productChoiceFields(exact *capmodel.BusinessExtensionProviderEvidence) []string {
	declaration, err := productfulfillmentcap.ParseProviderDeclaration(exact.ProviderMetadata, exact.AuthorizedMethods)
	if err != nil {
		return nil
	}
	return declaration.ProductChoiceFields
}

// Validate the commercial-to-provider aliases independently from target
// directory membership, which is checked against the authorized projection.
func validateSelectionChoiceBindings(selection CustomerFieldSelection) error {
	if len(selection.ProductChoiceBindings) == 0 {
		return nil
	}
	if selection.LocalField == nil {
		return gerror.New("purchase choice mapping requires a local field")
	}
	field := *selection.LocalField
	field.Key = selection.FieldKey
	return configurationcontract.ValidatePresentations(&capmodel.ConfigurationSchemaDeclaration{Version: "v1", Fields: []*capmodel.ConfigurationFieldDeclaration{&field}}, map[string]configurationcontract.Presentation{selection.FieldKey: {ProductFieldKey: selection.ProductFieldKey, ProductChoiceBindings: selection.ProductChoiceBindings}})
}

// Validation sees the explicitly mapped provider values while the persisted
// customer declaration, labels and price identities remain commercial facts.
func mappedProductChoices(selection CustomerFieldSelection) *capmodel.ConfigurationFieldDeclaration {
	field := *selection.LocalField
	field.StaticCandidates = make([]*capmodel.ConfigurationCandidate, 0, len(selection.LocalField.StaticCandidates))
	for _, choice := range selection.LocalField.StaticCandidates {
		if choice != nil {
			mapped := *choice
			mapped.Value = configurationcontract.ProductChoiceValue(selection.ProductChoiceBindings, choice.Value)
			field.StaticCandidates = append(field.StaticCandidates, &mapped)
		}
	}
	return &field
}

// validDeclaredProductBinding keeps declared technical type checks separate
// from the custom native-name route and duplicate-target bookkeeping.
func validDeclaredProductBinding(selection CustomerFieldSelection, target *capmodel.ConfigurationFieldDeclaration, used bool) bool {
	field := selection.LocalField
	return field != nil && strings.HasPrefix(selection.FieldKey, localCustomerFieldPrefix) && !used && field.Required && (field.Type == target.Type || field.Type == capmodel.ConfigurationFieldSingleSelect && target.Type == capmodel.ConfigurationFieldSingleLine)
}
