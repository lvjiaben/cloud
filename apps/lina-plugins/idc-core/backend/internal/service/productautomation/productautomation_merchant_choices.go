// Merchant choice lists retain the declared parameter identity and requiredness.
// Dynamic choice IDs are verified against the supplier projection before saving;
// frozen static lists keep sale labels, order and historical choices independent.
package productautomation

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
	"strings"
	"unicode/utf8"
)

func merchantChoiceSubset(field *capmodel.ConfigurationFieldDeclaration, selection CustomerFieldSelection) error {
	if field.Type != capmodel.ConfigurationFieldSingleSelect || selection.Minimum != "" || selection.Maximum != "" || len(selection.CandidateValues) < 1 || len(selection.CandidateValues) > 512 {
		return gerror.New("invalid merchant choice list")
	}
	var (
		original = map[string]*capmodel.ConfigurationCandidate{}
		seen     = map[string]bool{}
		choices  = make([]*capmodel.ConfigurationCandidate, 0, len(selection.CandidateValues))
	)
	for _, item := range field.StaticCandidates {
		if item != nil {
			original[item.Value] = item
		}
	}
	for _, value := range selection.CandidateValues {
		if value == "" || len(value) > 256 || seen[value] || (field.DynamicCandidates == nil && original[value] == nil) {
			return gerror.New("unknown or repeated merchant choice")
		}
		seen[value] = true
		label := selection.CandidateLabels[value]
		if label == nil && original[value] != nil {
			label = original[value].Label
		}
		if label == nil {
			label = &capmodel.LocalizedText{ZhCN: value, EnUS: value}
		}
		if strings.TrimSpace(label.ZhCN) == "" || strings.TrimSpace(label.EnUS) == "" || utf8.RuneCountInString(label.ZhCN) > 128 || utf8.RuneCountInString(label.EnUS) > 128 {
			return gerror.New("invalid merchant choice label")
		}
		choices = append(choices, &capmodel.ConfigurationCandidate{Value: value, Label: label})
	}
	field.StaticCandidates, field.DynamicCandidates = choices, nil
	return nil
}

// validateCustomerChoiceCandidates resolves all selected dynamic fields in one
// local directory projection, preventing arbitrary image IDs from being saved.
func (s *serviceImpl) validateCustomerChoiceCandidates(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, input SaveVersionConfigurationInput, scope dynamicCandidateScope) error {
	var dynamic []*capmodel.ConfigurationFieldDeclaration
	if exact.CustomerConfiguration == nil {
		return nil
	}
	for _, field := range exact.CustomerConfiguration.Fields {
		if field == nil || field.DynamicCandidates == nil {
			continue
		}
		for _, selection := range input.CustomerFieldSelections {
			if selection.FieldKey == field.Key && len(selection.CandidateValues) > 0 {
				dynamic = append(dynamic, field)
				break
			}
		}
	}
	if len(dynamic) == 0 {
		return nil
	}
	resolved, err := s.resolveDynamicCandidates(ctx, exact, exact.CustomerConfiguration, nil, configurationSurfaceCustomer, scope)
	if err != nil {
		return err
	}
	for _, field := range dynamic {
		allowed := map[string]bool{}
		for _, choice := range resolved[field.Key] {
			if choice != nil {
				allowed[choice.Value] = true
			}
		}
		for _, selection := range input.CustomerFieldSelections {
			if selection.FieldKey != field.Key {
				continue
			}
			for _, value := range selection.CandidateValues {
				if !allowed[value] {
					return gerror.New("merchant choice is absent from the supplier directory")
				}
			}
		}
	}
	return nil
}

// restoreMerchantChoiceSources keeps the complete local import directory available
// to administrators after a product freezes a smaller customer-facing list.
func (s *serviceImpl) restoreMerchantChoiceSources(ctx context.Context, exact *capmodel.BusinessExtensionProviderEvidence, customer *capmodel.ConfigurationSchemaDeclaration, scope dynamicCandidateScope, candidates *configurationcontract.ResolvedCandidates) error {
	if exact.CustomerConfiguration != nil && customer != nil {
		needsSource := false
		for _, original := range exact.CustomerConfiguration.Fields {
			if original == nil || original.DynamicCandidates == nil {
				continue
			}
			for _, selected := range customer.Fields {
				if selected.Key == original.Key && selected.DynamicCandidates == nil {
					needsSource = true
				}
			}
		}
		if needsSource {
			source, sourceErr := s.resolveDynamicCandidates(ctx, exact, exact.CustomerConfiguration, nil, configurationSurfaceCustomer, scope)
			if sourceErr != nil {
				return sourceErr
			}
			if *candidates == nil {
				*candidates = configurationcontract.ResolvedCandidates{}
			}
			for key, choices := range source {
				(*candidates)[key] = choices
			}
		}
	}
	return nil
}
