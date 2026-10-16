// This file preserves the product-automation service vocabulary while all
// structured-configuration rules live in the shared contract package.

package productautomation

import (
	"context"

	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/configurationcontract"
)

func (s *serviceImpl) compileConfigurationResolved(
	ctx context.Context,
	exact *capmodel.BusinessExtensionProviderEvidence,
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	scope dynamicCandidateScope,
) (*canonicalConfiguration, error) {
	resolved, err := s.resolveDynamicCandidates(ctx, exact, schema, values, surface, scope)
	if err != nil {
		return nil, err
	}
	compiled, err := configurationcontract.CompileResolved(schema, values, surface, exact.AuthorizedMethods, resolved)
	if err != nil {
		return nil, err
	}
	if surface == configurationSurfaceProduct {
		if err = s.validateProviderProductConfiguration(ctx, exact, compiled.ValuesJSON); err != nil {
			return nil, err
		}
	}
	return compiled, nil
}

func compileConfigurationWithResolved(
	exact *capmodel.BusinessExtensionProviderEvidence,
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	resolved configurationcontract.ResolvedCandidates,
) (*canonicalConfiguration, error) {
	return configurationcontract.CompileResolved(schema, values, surface, exact.AuthorizedMethods, resolved)
}

func (s *serviceImpl) validateConfigurationRuntime(
	ctx context.Context,
	exact *capmodel.BusinessExtensionProviderEvidence,
	schema *capmodel.ConfigurationSchemaDeclaration,
	values []ConfigurationValue,
	surface ConfigurationSurface,
	scope dynamicCandidateScope,
) error {
	resolved, err := s.resolveDynamicCandidates(ctx, exact, schema, values, surface, scope)
	if err != nil {
		return err
	}
	_, err = configurationcontract.EvaluateResolved(schema, values, surface, exact.AuthorizedMethods, resolved)
	return err
}

type ConfigurationSurface = configurationcontract.ConfigurationSurface

const (
	configurationSurfaceInterface = configurationcontract.SurfaceInterface
	configurationSurfaceProduct   = configurationcontract.SurfaceProduct
	configurationSurfaceCustomer  = configurationcontract.SurfaceCustomer
	ConfigurationSurfaceInterface = configurationcontract.SurfaceInterface
	ConfigurationSurfaceProduct   = configurationcontract.SurfaceProduct
)

type ConfigurationValue = configurationcontract.ConfigurationValue
type canonicalConfiguration = configurationcontract.CompiledConfiguration

func validateConfigurationSchema(
	schema *capmodel.ConfigurationSchemaDeclaration,
	surface ConfigurationSurface,
	authorizedMethods []string,
) (map[string]*capmodel.ConfigurationFieldDeclaration, error) {
	return configurationcontract.ValidateSchema(schema, surface, authorizedMethods)
}
