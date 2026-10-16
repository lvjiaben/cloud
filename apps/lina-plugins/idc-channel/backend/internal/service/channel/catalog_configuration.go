// This file implements bounded same-system protocol projections and rejects
// missing ownership or configuration evidence rather than guessing it.

package channel

import (
	"context"

	"lina-core/pkg/bizerr"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
)

func (s *service) BrowseCatalogNumbered(ctx context.Context, appID string, input channelcommercecap.CatalogNumberedQuery) (*channelcommercecap.CatalogNumberedPage, error) {
	runtime, err := s.catalogConfigurationRuntime(ctx, appID)
	if err != nil {
		return nil, err
	}
	return runtime.BrowseCatalogNumbered(ctx, input)
}

func (s *service) catalogConfigurationRuntime(ctx context.Context, appID string) (channelcommercecap.CatalogConfigurationRuntime, error) {
	_, err := s.requireChannel(ctx, appID, true)
	if err != nil {
		return nil, err
	}
	runtime, ok := s.core.(channelcommercecap.CatalogConfigurationRuntime)
	if !ok || runtime == nil {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return runtime, nil
}

func (s *service) ReadCatalogConfiguration(ctx context.Context, appID, slug string) (*channelcommercecap.Configuration, error) {
	runtime, err := s.catalogConfigurationRuntime(ctx, appID)
	if err != nil {
		return nil, err
	}
	return runtime.ReadCatalogConfiguration(ctx, slug)
}

func (s *service) ReadProductSnapshot(ctx context.Context, appID, slug string) (*channelcommercecap.ProductSnapshot, error) {
	runtime, err := s.catalogConfigurationRuntime(ctx, appID)
	if err != nil {
		return nil, err
	}
	return runtime.ReadProductSnapshot(ctx, slug)
}

func (s *service) EvaluateCatalogConfiguration(ctx context.Context, appID string, input channelcommercecap.ConfigurationInput) (*channelcommercecap.ConfigurationEvaluation, error) {
	account, accountErr := s.requireChannel(ctx, appID, true)
	if accountErr != nil {
		return nil, accountErr
	}
	input.CustomerNo = account.CustomerNo
	runtime, err := s.catalogConfigurationRuntime(ctx, appID)
	if err != nil {
		return nil, err
	}
	return runtime.EvaluateCatalogConfiguration(ctx, input)
}

func (s *service) PreviewCatalogTerms(ctx context.Context, appID string, input channelcommercecap.ConfigurationInput) (*channelcommercecap.BillingTerms, error) {
	account, accountErr := s.requireChannel(ctx, appID, true)
	if accountErr != nil {
		return nil, accountErr
	}
	input.CustomerNo = account.CustomerNo
	runtime, err := s.catalogConfigurationRuntime(ctx, appID)
	if err != nil {
		return nil, err
	}
	return runtime.PreviewCatalogTerms(ctx, input)
}

// ReadCatalogImportSnapshot retains the same authenticated catalog boundary.
func (s *service) ReadCatalogImportSnapshot(ctx context.Context, appID, slug string) (*channelcommercecap.ImportSnapshot, error) {
	account, err := s.requireChannel(ctx, appID, true)
	if err != nil {
		return nil, err
	}
	runtime, ok := s.core.(channelcommercecap.CatalogImportRuntime)
	if !ok || runtime == nil {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return runtime.ReadCatalogImportSnapshot(ctx, account.CustomerNo, slug)
}

// ReadOrderDeliveries first resolves an application-owned API order identifier.
func (s *service) ReadOrderDeliveries(ctx context.Context, appID string, id int64) (*channelcommercecap.OrderDeliveries, error) {
	if _, err := s.requireChannel(ctx, appID, false); err != nil {
		return nil, err
	}
	order, err := s.GetOrder(ctx, id, appID)
	if err != nil {
		return nil, err
	}
	runtime, ok := s.core.(channelcommercecap.OrderDeliveryRuntime)
	if !ok || runtime == nil {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return runtime.ReadOrderDeliveries(ctx, appID, order.CoreOrderNo)
}

// ReadImportCandidates refreshes safe candidate mappings for authenticated imports.
func (s *service) ReadImportCandidates(ctx context.Context, appID string, input channelcommercecap.ImportCandidatesInput) (*channelcommercecap.ImportCandidates, error) {
	account, err := s.requireChannel(ctx, appID, true)
	if err != nil {
		return nil, err
	}
	input.CustomerNo = account.CustomerNo
	if (channelcommercecap.ConfigurationInput{PublicSlug: input.PublicSlug, ConfigurationRevision: input.ConfigurationRevision, Values: input.Values}).Validate() != nil {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	runtime, ok := s.core.(channelcommercecap.ImportCandidatesRuntime)
	if !ok || runtime == nil {
		return nil, bizerr.NewCode(codeCatalogUnavailable)
	}
	return runtime.ReadImportCandidates(ctx, input)
}
