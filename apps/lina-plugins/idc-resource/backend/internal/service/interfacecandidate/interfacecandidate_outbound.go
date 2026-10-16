// This file lends the resource owner's confidential transport only for exact
// core-authorized existing business. Stopping new sales does not abandon paid
// work, while disabled channels and mismatched frozen evidence fail closed.

package interfacecandidate

import (
	"context"
	"sync/atomic"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/pluginhost"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

// WithOutbound preserves resource secret ownership while the core registrar
// registers the handler under its own existing-business invocation context.
func (source *LazySource) WithOutbound(ctx context.Context, request resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest, consume func(pluginhost.ConfidentialOutboundHandler) error) error {
	if source == nil || ctx == nil || ctx.Err() != nil || consume == nil || request.InterfaceConfigurationRevision == "" || request.Method == "" {
		return bizerr.NewCode(CodeCatalogUnavailable)
	}
	source.mu.RLock()
	deps := source.deps
	source.mu.RUnlock()
	if deps == nil {
		return bizerr.NewCode(CodeCatalogUnavailable)
	}
	binding, err := loadBusinessBinding(ctx, deps.database, request.InterfaceInstanceNo, false)
	if err != nil || !matchesExistingBusiness(binding, request) {
		return bizerr.NewCode(CodeCatalogUnavailable)
	}
	reader, err := channelconfig.NewReader(deps.database)
	if err != nil {
		return bizerr.NewCode(CodeCatalogUnavailable)
	}
	configuration, err := reader.ReadRevision(ctx, binding.ChannelNo, request.InterfaceConfigurationRevision)
	if err != nil || !matchesConfiguration(binding, configuration) {
		return bizerr.NewCode(CodeCatalogUnavailable)
	}
	handler, cleanup, err := deps.outbound(ctx, configuration, request.Method, nil)
	if err != nil || handler == nil || cleanup == nil {
		return bizerr.NewCode(CodeCatalogUnavailable)
	}
	defer cleanup()
	var active atomic.Bool
	active.Store(true)
	defer active.Store(false)
	return consume(func(callCtx context.Context, input *pluginhost.ConfidentialOutboundRequest) (*pluginhost.ConfidentialOutboundResult, error) {
		if !active.Load() || ctx.Err() != nil {
			return nil, bizerr.NewCode(CodeCatalogUnavailable)
		}
		return handler(callCtx, input)
	})
}

func matchesExistingBusiness(value binding, request resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest) bool {
	return value.ChannelNo == request.ResourceChannelRef &&
		value.ProductPluginID == request.PluginID && value.ProductReleaseVersion == request.PluginReleaseVersion &&
		value.ProductContractVersion == request.ContractVersion && value.ProductReleaseDigest == request.ReleaseDigest &&
		value.ProductDeclarationDigest == request.DeclarationDigest && contains(value.ProductAuthorizedMethods, request.Method)
}

var _ resourceinterfaceprojectioncap.ExistingBusinessOutboundSource = (*LazySource)(nil)
