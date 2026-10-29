// Merchant price targets join the generic configuration editor to local price
// dimensions. The public pricing API retains opaque identities; this authorized
// editor reuses only fields and values already present in its merchant schema.
package productautomation

import (
	"context"

	"github.com/gogf/gf/v2/database/gdb"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func (s *serviceImpl) GetMerchantConfiguration(ctx context.Context, versionNo string) (*VersionConfiguration, error) {
	configuration, err := s.GetVersionConfiguration(ctx, versionNo)
	if err != nil || configuration == nil || configuration.Version < 1 {
		return configuration, err
	}
	configuration.MerchantPriceTargets, err = s.merchantPriceTargets(ctx, versionNo, configuration)
	if err != nil {
		return nil, err
	}
	return configuration, nil
}

func (s *serviceImpl) merchantPriceTargets(ctx context.Context, versionNo string, configuration *VersionConfiguration) ([]MerchantPriceTarget, error) {
	var rows []configurationPriceDimensionRow
	if err := s.database.GetScan(ctx, &rows, `SELECT dimension_no,field_key,candidate_value,pricing_mode_code
FROM plugin_idc_core_catalog_configuration_price_dimension WHERE tenant_id=0 AND version_no=? AND status_code='active'
ORDER BY field_key,candidate_value,id LIMIT ?`, versionNo, maximumConfigurationPriceCandidates+1); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if len(rows) > maximumConfigurationPriceCandidates {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	return projectMerchantPriceTargets(configuration, rows), nil
}

func projectMerchantPriceTargets(configuration *VersionConfiguration, rows []configurationPriceDimensionRow) []MerchantPriceTarget {
	var (
		fields     = make(map[string]*capmodel.ConfigurationFieldDeclaration)
		selections = make(map[string]CustomerFieldSelection)
		result     = make([]MerchantPriceTarget, 0, len(rows))
	)
	if configuration == nil || configuration.CustomerDeclaration == nil {
		return result
	}
	for _, field := range configuration.CustomerDeclaration.Fields {
		if field != nil {
			fields[field.Key] = field
		}
	}
	for _, selection := range configuration.CustomerFieldSelections {
		selections[selection.FieldKey] = selection
	}
	for _, row := range rows {
		field := fields[row.FieldKey]
		if field == nil {
			continue
		}
		selection := selections[row.FieldKey]
		quantity := field.Type == capmodel.ConfigurationFieldInteger && selection.NumericPricing != configurationcontract.NumericPricingChoice
		allowed := false
		switch field.Type {
		case capmodel.ConfigurationFieldInteger:
			if quantity {
				allowed = row.CandidateValue == configurationQuantityPriceIdentity
			} else {
				for _, value := range selection.NumericChoices {
					if value != "0" && value == row.CandidateValue {
						allowed = true
						break
					}
				}
			}
		case capmodel.ConfigurationFieldBoolean:
			allowed = row.CandidateValue == "true" || row.CandidateValue == "false"
		default:
			if configurationPriceCandidateField(field.Type) {
				for _, choice := range append(append([]*capmodel.ConfigurationCandidate{}, field.StaticCandidates...), configuration.CustomerResolvedCandidates[row.FieldKey]...) {
					if choice != nil && choice.Value == row.CandidateValue {
						allowed = true
						break
					}
				}
			}
		}
		if !allowed {
			continue
		}
		value := row.CandidateValue
		if quantity {
			value = ""
		}
		result = append(result, MerchantPriceTarget{FieldKey: row.FieldKey, CandidateValue: value, Quantity: quantity, DimensionNo: row.DimensionNo, PricingMode: row.PricingMode})
	}
	return result
}

func (s *serviceImpl) ResolveMerchantPriceTargets(ctx context.Context, versionNo string, inputs []MerchantPriceTargetInput) (map[string]string, error) {
	var result map[string]string
	err := s.database.Transaction(ctx, func(txCtx context.Context, _ gdb.TX) error {
		var resolveErr error
		result, resolveErr = s.resolveMerchantPriceTargets(txCtx, versionNo, inputs)
		return resolveErr
	})
	return result, err
}

func (s *serviceImpl) resolveMerchantPriceTargets(ctx context.Context, versionNo string, inputs []MerchantPriceTargetInput) (map[string]string, error) {
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	configuration, _, err := s.loadPriceConfiguration(ctx, versionNo)
	if err != nil {
		return nil, err
	}
	if configuration == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	candidates, err := configurationPriceCandidates(versionNo, configuration, s.booleanPriceLabels)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementInvalid)
	}
	selected, modes, aliases, err := selectMerchantPriceTargets(candidates, inputs)
	if err != nil {
		return nil, err
	}
	if err = s.lockDraftVersion(ctx, versionNo); err != nil {
		return nil, err
	}
	current, err := s.database.GetValue(ctx, `SELECT version FROM plugin_idc_core_catalog_product_version_configuration WHERE tenant_id=0 AND version_no=? FOR SHARE`, versionNo)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if current.IsNil() || current.Int64() != configuration.Version {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	result, err := s.insertConfigurationPriceTargets(ctx, versionNo, selected, authorization.RequiredPermission, modes)
	if err != nil {
		return nil, err
	}
	for alias, token := range aliases {
		result[alias] = result[token]
	}
	return result, nil
}

func selectMerchantPriceTargets(candidates []configurationPriceCandidatePrivate, inputs []MerchantPriceTargetInput) ([]configurationPriceCandidatePrivate, map[string]catalog.ConfigurationPriceMode, map[string]string, error) {
	if len(inputs) == 0 || len(inputs) > 256 {
		return nil, nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	var (
		available  = make(map[string]configurationPriceCandidatePrivate, len(candidates))
		selected   = make([]configurationPriceCandidatePrivate, 0, len(inputs))
		modes      = make(map[string]catalog.ConfigurationPriceMode, len(inputs))
		aliases    = make(map[string]string, len(inputs))
		identities = make(map[string]string, len(inputs))
		tokens     = make(map[string]string, len(inputs))
	)
	for _, candidate := range candidates {
		available[candidate.fieldKey+"\x00"+candidate.candidateValue] = candidate
	}
	for _, input := range inputs {
		value := input.CandidateValue
		if input.Quantity {
			if value != "" {
				return nil, nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
			}
			value = configurationQuantityPriceIdentity
		}
		identity := input.FieldKey + "\x00" + value
		candidate, exists := available[identity]
		if !exists || input.Token == "" || len(input.Token) > 80 || input.Quantity != candidate.Quantity {
			return nil, nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		mode := input.PricingMode
		if mode == "" {
			mode = catalog.ConfigurationPriceFixed
			if input.Quantity {
				mode = catalog.ConfigurationPricePerUnit
			}
		}
		if (!input.Quantity && mode != catalog.ConfigurationPriceFixed) || (input.Quantity && mode != catalog.ConfigurationPricePerUnit && mode != catalog.ConfigurationPriceBandTotal && mode != catalog.ConfigurationPriceGraduated) {
			return nil, nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		if previous, exists := tokens[input.Token]; exists && previous != identity {
			return nil, nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		tokens[input.Token] = identity
		if token, exists := identities[identity]; exists {
			if modes[token] != mode {
				return nil, nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
			}
			aliases[input.Token] = token
			continue
		}
		candidate.CandidateToken = input.Token
		selected = append(selected, candidate)
		identities[identity], modes[input.Token] = input.Token, mode
	}
	return selected, modes, aliases, nil
}
