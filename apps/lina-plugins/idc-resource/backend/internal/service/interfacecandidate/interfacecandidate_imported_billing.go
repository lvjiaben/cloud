// This file provides the owner-to-owner price read for an already-authorized
// published imported product. It cannot accept caller-selected methods, routes,
// supplier identities or credentials, and has no direct HTTP entry point.
package interfacecandidate

import (
	"context"
	"encoding/json"
	"slices"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-core/pkg/plugin/capability/plugincap"
	"lina-plugin-idc-core/backend/cap/channelcommercecap"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func (s *catalogSource) ReadImportedBillingTerms(ctx context.Context, request resourceinterfaceprojectioncap.ImportedBillingRequest) (*channelcommercecap.BillingTerms, error) {
	if s == nil || s.reader == nil {
		return nil, gerror.New("imported billing runtime is unavailable")
	}
	return s.reader.ReadImportedBillingTerms(ctx, request)
}
func (source *LazySource) ReadImportedBillingTerms(ctx context.Context, request resourceinterfaceprojectioncap.ImportedBillingRequest) (*channelcommercecap.BillingTerms, error) {
	if source == nil || ctx == nil || ctx.Err() != nil || request.Evidence.Validate() != nil || !request.Evidence.SaleImportProtocol.NativePurchase() || request.Input.Validate() != nil || len(request.Input.CurrencyCode) != 3 {
		return nil, gerror.New("imported billing request is invalid")
	}
	if err := source.validateImportedBinding(ctx, request.Evidence); err != nil {
		return nil, err
	}
	query := resourceinterfaceprojectioncap.CatalogProductQuery{Reference: request.Input.PublicSlug}
	var detail resourceinterfaceprojectioncap.CatalogDetail
	if err := source.readImported(ctx, request.Evidence, resourcecap.DynamicProviderMethodReadCatalogProduct, query, &detail); err != nil {
		return nil, err
	}
	read := request.Evidence
	if err := source.validateNativeCatalogSnapshot(ctx, &read, &detail); err != nil {
		return nil, err
	}
	if detail.Validate(query) != nil || read.SaleImportProtocol != request.Evidence.SaleImportProtocol || !importedRevisionMatches(detail, request.Input.ConfigurationRevision) {
		return nil, gerror.New("imported billing source revision changed")
	}
	var result channelcommercecap.BillingTerms
	if err := source.readImported(ctx, request.Evidence, resourcecap.DynamicProviderMethodCatalogBillingTerms, request.Input, &result); err != nil {
		return nil, err
	}
	if err := source.validateImportedBinding(ctx, request.Evidence); err != nil {
		return nil, err
	}
	if result.Validate() != nil || result.ConfigurationRevision != request.Input.ConfigurationRevision || result.CurrencyCode != request.Input.CurrencyCode || !importedTermsMatch(detail, result) {
		return nil, gerror.New("imported billing response is invalid")
	}
	return &result, nil
}
func importedRevisionMatches(detail resourceinterfaceprojectioncap.CatalogDetail, revision string) bool {
	if detail.ImportSnapshot != nil {
		return detail.ImportSnapshot.ConfigurationRevision == revision
	}
	return detail.PeerPurchase != nil && detail.PeerPurchase.Configuration.ConfigurationRevision == revision
}
func importedTermsMatch(detail resourceinterfaceprojectioncap.CatalogDetail, result channelcommercecap.BillingTerms) bool {
	allowed := map[string]bool{}
	if detail.ImportSnapshot != nil {
		for _, price := range detail.ImportSnapshot.Prices {
			if price.CurrencyCode == result.CurrencyCode {
				allowed[price.TermReference] = true
			}
		}
	}
	if detail.PeerPurchase != nil {
		for _, currency := range detail.PeerPurchase.BillingTerms {
			if currency.CurrencyCode == result.CurrencyCode {
				for _, term := range currency.Terms {
					allowed[term.BillingTermToken] = true
				}
			}
		}
	}
	if len(allowed) != len(result.Terms) {
		return false
	}
	for _, term := range result.Terms {
		if !allowed[term.BillingTermToken] || term.MinimumPurchaseCycles != 1 {
			return false
		}
		delete(allowed, term.BillingTermToken)
	}
	return len(allowed) == 0
}
func (source *LazySource) readImported(ctx context.Context, expected resourceinterfaceprojectioncap.CatalogReadEvidence, method string, input, target any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return gerror.New("imported billing input is invalid")
	}
	request := resourceinterfaceprojectioncap.CandidateRequest{InterfaceInstanceNo: expected.InterfaceInstanceNo, ExpectedConfigurationRevision: expected.InterfaceConfigurationRevision, Method: method, Values: payload}
	response, actual, err := source.invokeRead(ctx, request, []string{"StatusCode", "Data", "Authenticated"})
	if err != nil || actual == nil || !importedReadMatches(expected, *actual) {
		return gerror.New("imported billing source binding changed")
	}
	return decodeCatalogResponse(response, target)
}
func importedReadMatches(expected, actual resourceinterfaceprojectioncap.CatalogReadEvidence) bool {
	return expected.InterfaceInstanceNo == actual.InterfaceInstanceNo && expected.OwnerID == actual.OwnerID && expected.ResourceChannelRef == actual.ResourceChannelRef && expected.InterfaceConfigurationRevision == actual.InterfaceConfigurationRevision && expected.PluginID == actual.PluginID && expected.PluginReleaseVersion == actual.PluginReleaseVersion && expected.ContractVersion == actual.ContractVersion && expected.ReleaseDigest == actual.ReleaseDigest && expected.DeclarationDigest == actual.DeclarationDigest && slices.Equal(expected.AuthorizedMethods, actual.AuthorizedMethods)
}
func (source *LazySource) validateImportedBinding(ctx context.Context, expected resourceinterfaceprojectioncap.CatalogReadEvidence) error {
	source.mu.RLock()
	deps := source.deps
	source.mu.RUnlock()
	if deps == nil {
		return gerror.New("imported billing runtime is unavailable")
	}
	binding, err := loadBinding(ctx, deps.database, expected.InterfaceInstanceNo)
	if err != nil || !importedReadMatches(expected, *binding.readEvidence(expected.InterfaceInstanceNo)) {
		return gerror.New("imported billing source binding is unavailable")
	}
	exact, err := deps.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{PluginID: expected.PluginID, Point: capmodel.BusinessExtensionProductFulfillment, ReleaseVersion: expected.PluginReleaseVersion, ContractVersion: expected.ContractVersion, ReleaseDigest: expected.ReleaseDigest, DeclarationDigest: expected.DeclarationDigest})
	if err != nil || exact == nil || exact.PluginID != expected.PluginID || exact.Point != capmodel.BusinessExtensionProductFulfillment || exact.ReleaseVersion != expected.PluginReleaseVersion || exact.ContractVersion != expected.ContractVersion || exact.ReleaseDigest != expected.ReleaseDigest || exact.DeclarationDigest != expected.DeclarationDigest || !slices.Equal(exact.AuthorizedMethods, expected.AuthorizedMethods) || resourcecap.ValidateNativeProductEvidence(exact) != nil {
		return gerror.New("imported billing exact product authority is unavailable")
	}
	if exact.ProviderMetadata.Values["saleImportProtocol"] != string(expected.SaleImportProtocol) {
		return gerror.New("imported billing protocol changed")
	}
	resourceEvidence, err := deps.extensions.GetProviderExact(ctx, plugincap.BusinessExtensionProviderExactInput{PluginID: binding.PluginID, Point: resourcecap.DynamicProviderPoint, ReleaseVersion: binding.PluginReleaseVersion, ContractVersion: binding.ContractVersion, ReleaseDigest: binding.ReleaseDigest, DeclarationDigest: binding.DeclarationDigest})
	if err != nil || resourceEvidence == nil || resourceEvidence.PluginID != binding.PluginID || resourceEvidence.Point != resourcecap.DynamicProviderPoint || resourceEvidence.ReleaseVersion != binding.PluginReleaseVersion || resourceEvidence.ContractVersion != binding.ContractVersion || resourceEvidence.ReleaseDigest != binding.ReleaseDigest || resourceEvidence.DeclarationDigest != binding.DeclarationDigest || !slices.Equal(resourceEvidence.AuthorizedMethods, binding.AuthorizedMethods) || !contains(resourceEvidence.AuthorizedMethods, resourcecap.DynamicProviderMethodCatalogBillingTerms) {
		return gerror.New("imported billing exact resource authority is unavailable")
	}
	return nil
}

var _ resourceinterfaceprojectioncap.ImportedBillingSource = (*catalogSource)(nil)

var _ resourceinterfaceprojectioncap.ImportedBillingSource = (*LazySource)(nil)
