// This file resolves business interfaces to the current enabled implementation.
// Historical package/declaration hashes are retained only in old business records.
package providerexecution

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
)

func (r *registrar) GetProviderExact(ctx context.Context, input plugincap.BusinessExtensionProviderExactInput) (*capmodel.BusinessExtensionProviderEvidence, error) {
	if evidence, ok := r.admittedProvider(ctx, input); ok {
		return evidence, nil
	}
	page, err := r.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{Point: input.Point, ContractVersion: input.ContractVersion, Page: capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100}})
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, gerror.New("current provider is unavailable")
	}
	return compatibleProvider(input, page.Items, nil)
}

// GetProvidersExact performs one bounded discovery for the whole page.
func (r *registrar) GetProvidersExact(ctx context.Context, inputs []plugincap.BusinessExtensionProviderExactInput) ([]*capmodel.BusinessExtensionProviderEvidence, error) {
	if len(inputs) == 0 {
		return []*capmodel.BusinessExtensionProviderEvidence{}, nil
	}
	if len(inputs) > plugincap.MaxBusinessExtensionProviderExactBatch {
		return nil, gerror.New("provider batch exceeds its bound")
	}
	for _, input := range inputs {
		if input.Point != inputs[0].Point || input.ContractVersion != inputs[0].ContractVersion {
			return nil, gerror.New("provider batch mixes contracts")
		}
	}
	if evidence, ok := r.admittedProviders(ctx, inputs); ok {
		return evidence, nil
	}
	page, err := r.ListProviders(ctx, plugincap.BusinessExtensionProviderListInput{Point: inputs[0].Point, ContractVersion: inputs[0].ContractVersion, Page: capmodel.PageRequest{PageNum: 1, PageSize: 100, Limit: 100}})
	if err != nil {
		return nil, err
	}
	if page == nil {
		return nil, gerror.New("current providers are unavailable")
	}
	result := make([]*capmodel.BusinessExtensionProviderEvidence, len(inputs))
	for index, input := range inputs {
		result[index], err = compatibleProvider(input, page.Items, nil)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
