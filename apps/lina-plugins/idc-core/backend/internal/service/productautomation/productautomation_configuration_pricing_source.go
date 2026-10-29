// This file reads merchant pricing inputs from the saved local configuration.
// Provider discovery only enriches dynamic choices; it cannot block pricing of
// frozen quantities, static choices or previously saved dynamic dimensions.

package productautomation

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func (s *serviceImpl) loadPriceConfiguration(ctx context.Context, versionNo string) (*VersionConfiguration, []configurationPriceDimensionRow, error) {
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, versionNo)
	if err != nil {
		return nil, nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, nil, err
	}
	if validateBusinessNo(versionNo, "ver") != nil {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	binding, err := s.loadBinding(ctx, versionNo, false)
	if err != nil {
		return nil, nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if binding == nil {
		configuration, readErr := s.readManualConfiguration(ctx, versionNo)
		if readErr != nil || configuration.Version < 1 {
			return configuration, nil, readErr
		}
		rows, readErr := s.loadConfigurationPriceDimensions(ctx, versionNo)
		return configuration, rows, readErr
	}
	var row *entity.CatalogProductVersionConfiguration
	if err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).
		Where(do.CatalogProductVersionConfiguration{TenantId: idccap.DefaultTenantID, VersionNo: versionNo}).Scan(&row); err != nil {
		return nil, nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if row == nil {
		return nil, nil, nil
	}
	if row.InterfaceInstanceNo == "" {
		configuration, readErr := s.readManualConfiguration(ctx, versionNo)
		if readErr != nil {
			return nil, nil, readErr
		}
		rows, readErr := s.loadConfigurationPriceDimensions(ctx, versionNo)
		return configuration, rows, readErr
	}
	if row.InterfaceInstanceNo != binding.InterfaceInstanceNo {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	product, err := decodeConfigurationDeclaration(row.ProductDeclarationSnapshot)
	if err != nil {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	customer, err := decodeConfigurationDeclaration(row.CustomerDeclarationSnapshot)
	if err != nil {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	productJSON, err := configurationDeclarationJSON(product)
	if err != nil {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	customerJSON, err := configurationDeclarationJSON(customer)
	if err != nil || row.Version < 1 || row.ProductSchemaVersion != configurationSchemaVersion(product) || row.CustomerSchemaVersion != configurationSchemaVersion(customer) ||
		row.DeclarationDigest != configurationDigest(append(append([]byte{}, productJSON...), customerJSON...)) {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	rows, err := s.loadConfigurationPriceDimensions(ctx, versionNo)
	if err != nil {
		return nil, nil, err
	}
	configuration := &VersionConfiguration{Version: row.Version, CustomerDeclaration: customer, CustomerResolvedCandidates: configurationcontract.ResolvedCandidates{}}
	configuration.CustomerFieldSelections, err = withCustomerPresentations(nil, row.CustomerPresentationSnapshot)
	if err != nil {
		return nil, nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	if err = s.enrichPriceConfiguration(ctx, versionNo, configuration, rows); err != nil {
		return nil, nil, err
	}
	return configuration, rows, nil
}

func (s *serviceImpl) loadConfigurationPriceDimensions(ctx context.Context, versionNo string) ([]configurationPriceDimensionRow, error) {
	var rows []configurationPriceDimensionRow
	if err := s.database.GetScan(ctx, &rows, `SELECT dimension_no,configuration_version,field_key,candidate_value,
field_label_zh_cn,field_label_en_us,candidate_label_zh_cn,candidate_label_en_us,pricing_mode_code
FROM plugin_idc_core_catalog_configuration_price_dimension
WHERE tenant_id=0 AND version_no=? AND status_code='active'
ORDER BY field_key,candidate_value,id LIMIT ?`, versionNo, maximumConfigurationPriceCandidates+1); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if len(rows) > maximumConfigurationPriceCandidates {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	return rows, nil
}

// enrichPriceConfiguration retains already saved dynamic dimensions and merges
// newly resolved choices only when the original declaration is available.
func (s *serviceImpl) enrichPriceConfiguration(ctx context.Context, versionNo string, configuration *VersionConfiguration, rows []configurationPriceDimensionRow) error {
	dynamicFields := make(map[string]bool)
	if configuration.CustomerDeclaration != nil {
		for _, field := range configuration.CustomerDeclaration.Fields {
			if field != nil && configurationPriceCandidateField(field.Type) && field.DynamicCandidates != nil {
				dynamicFields[field.Key] = true
			}
		}
	}
	if len(dynamicFields) == 0 {
		return nil
	}
	for _, dimension := range rows {
		if dynamicFields[dimension.FieldKey] && dimension.ConfigurationVersion == configuration.Version {
			configuration.CustomerResolvedCandidates[dimension.FieldKey] = append(configuration.CustomerResolvedCandidates[dimension.FieldKey], &capmodel.ConfigurationCandidate{
				Value: dimension.CandidateValue, Label: &capmodel.LocalizedText{ZhCN: dimension.CandidateLabelZhCN, EnUS: dimension.CandidateLabelEnUS},
			})
		}
	}
	current, loadErr := s.GetVersionConfiguration(ctx, versionNo)
	if loadErr != nil {
		if bizerr.Is(loadErr, CodeEvidenceUnavailable) || bizerr.Is(loadErr, catalog.CodeManagementUnavailable) {
			return nil
		}
		return loadErr
	}
	if current == nil || current.Version != configuration.Version {
		return bizerr.NewCode(catalog.CodeManagementConflict)
	}
	for field, candidates := range current.CustomerResolvedCandidates {
		if !dynamicFields[field] {
			continue
		}
		seen := make(map[string]bool)
		for _, candidate := range configuration.CustomerResolvedCandidates[field] {
			seen[candidate.Value] = true
		}
		for _, candidate := range candidates {
			if candidate != nil && !seen[candidate.Value] {
				configuration.CustomerResolvedCandidates[field] = append(configuration.CustomerResolvedCandidates[field], candidate)
				seen[candidate.Value] = true
			}
		}
	}
	return nil
}
