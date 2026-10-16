// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package interfacecandidate

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func (s *catalogSource) EvaluateCatalogConfiguration(ctx context.Context, interfaceNo string, input channelcommercecap.ConfigurationInput) (*channelcommercecap.ConfigurationEvaluation, error) {
	if input.Validate() != nil {
		return nil, bizerr.NewCode(CodeCatalogInvalid)
	}
	var result channelcommercecap.ConfigurationEvaluation
	if _, err := s.read(ctx, interfaceNo, resourcecap.DynamicProviderMethodEvaluateCatalogConfiguration, input, &result); err != nil {
		return nil, err
	}
	if result.Configuration.Validate() != nil || result.ConfigurationRevision != input.ConfigurationRevision || len(result.Issues) > 128 {
		return nil, bizerr.NewCode(CodeCatalogUnavailable)
	}
	return &result, nil
}

func (s *catalogSource) ReadCatalogBillingTerms(ctx context.Context, interfaceNo string, input channelcommercecap.ConfigurationInput) (*channelcommercecap.BillingTerms, error) {
	if input.Validate() != nil || len(input.CurrencyCode) != 3 {
		return nil, bizerr.NewCode(CodeCatalogInvalid)
	}
	var result channelcommercecap.BillingTerms
	if _, err := s.read(ctx, interfaceNo, resourcecap.DynamicProviderMethodCatalogBillingTerms, input, &result); err != nil {
		return nil, err
	}
	if result.Validate() != nil || result.ConfigurationRevision != input.ConfigurationRevision || result.CurrencyCode != input.CurrencyCode {
		return nil, bizerr.NewCode(CodeCatalogUnavailable)
	}
	return &result, nil
}

var _ resourceinterfaceprojectioncap.CatalogConfigurationSource = (*catalogSource)(nil)
