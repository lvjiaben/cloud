// This file evaluates complete administrator configuration forms without
// persisting them or exposing provider method names and technical errors.

package productautomation

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/model/do"
	"lina-plugin-idc-core/backend/internal/model/entity"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func (s *serviceImpl) EvaluateInterfaceConfiguration(
	ctx context.Context,
	input EvaluateInterfaceConfigurationInput,
) (*ConfigurationEvaluation, error) {
	authorization, err := s.authorizer.AuthorizeInterfaceConfiguration(ctx, input.InterfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if validateBusinessNo(input.InterfaceInstanceNo, "pfi") != nil || input.ExpectedRevision <= 0 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	resourceOwned, err := s.resourceInterfaceOwned(ctx, input.InterfaceInstanceNo)
	if err != nil || resourceOwned {
		return nil, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	_, exact, err := s.loadExactInterfaceEvidence(ctx, input.InterfaceInstanceNo)
	if err != nil {
		return nil, err
	}
	revision, err := s.loadLatestInterfaceConfigurationRevision(ctx, input.InterfaceInstanceNo, false)
	if err != nil || revision == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	if revision.Revision != input.ExpectedRevision {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	declaration, declarationErr := configurationDeclarationJSON(exact.InterfaceConfiguration)
	if declarationErr != nil || configurationSchemaVersion(exact.InterfaceConfiguration) != revision.SchemaVersion ||
		configurationDigest(declaration) != revision.DeclarationDigest {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	secretStates, err := s.loadInterfaceSecretStates(ctx, revision.Id)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	normalized, _, err := normalizeInterfaceSecretInputs(exact.InterfaceConfiguration, input.Values, secretStates)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	return s.evaluateConfiguration(ctx, exact, exact.InterfaceConfiguration, normalized, configurationSurfaceInterface, revision.Revision,
		dynamicCandidateScope{interfaceInstanceNo: input.InterfaceInstanceNo, configurationRevision: revision.Revision})
}

func (s *serviceImpl) EvaluateVersionConfiguration(
	ctx context.Context,
	input EvaluateVersionConfigurationInput,
) (*ConfigurationEvaluation, error) {
	authorization, err := s.authorizer.AuthorizeVersionConfiguration(ctx, input.VersionNo)
	if err != nil {
		return nil, err
	}
	if err = requireAuthorization(authorization); err != nil {
		return nil, err
	}
	if validateBusinessNo(input.VersionNo, "ver") != nil || input.ExpectedVersion < 0 {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	binding, err := s.loadBinding(ctx, input.VersionNo, false)
	if err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if binding == nil {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	exact, err := s.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{
		PluginID: binding.PluginId, ReleaseVersion: binding.PluginReleaseVersion,
		Point: binding.ExtensionPointCode, ContractVersion: binding.ContractVersion,
		ReleaseDigest: binding.ReleaseDigest, DeclarationDigest: binding.DeclarationDigest,
	})
	if err != nil || !sameProviderContract(bindingEvidence(binding), exact) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	var row *entity.CatalogProductVersionConfiguration
	if err = s.database.Model(do.CatalogProductVersionConfiguration{}).Ctx(ctx).
		Where(do.CatalogProductVersionConfiguration{TenantId: idccap.DefaultTenantID, VersionNo: input.VersionNo}).
		Scan(&row); err != nil {
		return nil, bizerr.WrapCode(err, catalog.CodeManagementUnavailable)
	}
	if row == nil {
		if input.ExpectedVersion != 0 {
			return nil, bizerr.NewCode(catalog.CodeManagementConflict)
		}
		interfaceRevision, loadErr := s.loadLatestInterfaceConfigurationRevision(ctx, binding.InterfaceInstanceNo, false)
		if loadErr != nil || interfaceRevision == nil {
			return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
		}
		return s.evaluateConfiguration(
			ctx, exact, exact.ProductConfiguration, input.MerchantValues,
			configurationSurfaceProduct, 0, dynamicCandidateScope{interfaceInstanceNo: binding.InterfaceInstanceNo, configurationRevision: interfaceRevision.Revision},
		)
	}
	if row.Version != input.ExpectedVersion {
		return nil, bizerr.NewCode(catalog.CodeManagementConflict)
	}
	product, err := decodeConfigurationDeclaration(row.ProductDeclarationSnapshot)
	if err != nil || !sameCompatibleProductConfiguration(product, exact.ProductConfiguration) {
		return nil, bizerr.NewCode(CodeEvidenceUnavailable)
	}
	return s.evaluateConfiguration(
		// Display and save already use the compatible active declaration.
		// Evaluate the same optional fields while keeping the stored revision fence.
		ctx, exact, exact.ProductConfiguration, input.MerchantValues,
		configurationSurfaceProduct, row.Version, dynamicCandidateScope{interfaceInstanceNo: binding.InterfaceInstanceNo, configurationRevision: row.InterfaceConfigurationRevision},
	)
}

func (s *serviceImpl) evaluateConfiguration(
	ctx context.Context,
	exact *capmodel.BusinessExtensionProviderEvidence,
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	revision int64,
	scope dynamicCandidateScope,
) (*ConfigurationEvaluation, error) {
	resolved, err := s.resolveDynamicCandidates(ctx, exact, schema, values, surface, scope)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementUnavailable)
	}
	evaluated, err := configurationcontract.EvaluateResolved(
		schema, values, surface, exact.AuthorizedMethods, resolved,
	)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	if evaluated.Valid && surface == configurationSurfaceProduct {
		compiled, compileErr := configurationcontract.CompileResolved(schema, values, surface, exact.AuthorizedMethods, resolved)
		if compileErr != nil {
			return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
		}
		if validationErr := s.validateProviderProductConfiguration(ctx, exact, compiled.ValuesJSON); validationErr != nil {
			return nil, validationErr
		}
	}
	presentation, err := configurationcontract.MaterializeResolvedSchema(
		schema, surface, exact.AuthorizedMethods, resolved,
	)
	if err != nil {
		return nil, bizerr.NewCode(catalog.CodeManagementInvalid)
	}
	result := &ConfigurationEvaluation{
		Values: evaluated.Values, States: make(map[string]ConfigurationFieldState, len(evaluated.States)),
		Candidates: filteredEvaluationCandidates(presentation, evaluated.States),
		Issues:     make([]ConfigurationIssue, 0, len(evaluated.Issues)),
		Valid:      evaluated.Valid, Revision: revision,
	}
	for fieldKey, state := range evaluated.States {
		result.States[fieldKey] = ConfigurationFieldState{
			Visible: state.Visible, Required: state.Required, Disabled: state.Disabled,
		}
	}
	for _, issue := range evaluated.Issues {
		result.Issues = append(result.Issues, ConfigurationIssue{FieldKey: issue.FieldKey, Code: issue.Code})
	}
	return result, nil
}

func filteredEvaluationCandidates(
	schema *capmodel.ConfigurationSchemaDeclaration,
	states map[string]configurationcontract.FieldState,
) configurationcontract.ResolvedCandidates {
	result := make(configurationcontract.ResolvedCandidates)
	if schema == nil {
		return result
	}
	for _, field := range schema.Fields {
		if field == nil || len(field.StaticCandidates) == 0 {
			continue
		}
		state := states[field.Key]
		allowed := make(map[string]struct{}, len(state.CandidateValues))
		for _, value := range state.CandidateValues {
			allowed[value] = struct{}{}
		}
		for _, candidate := range field.StaticCandidates {
			if candidate == nil {
				continue
			}
			if _, visible := allowed[candidate.Value]; visible {
				result[field.Key] = append(result[field.Key], candidate)
			}
		}
	}
	return result
}
