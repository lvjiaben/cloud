// This file authorizes streaming file transfers against exact paid-business
// interface evidence before borrowing the resource owner's transport.
package interfacecandidate

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

type FileTransferFactory func(context.Context, channelconfig.Configuration, resourceinterfaceprojectioncap.PurchaseFileTransfer) (string, error)

func (source *LazySource) BindFileTransfer(factory FileTransferFactory) error {
	if source == nil || factory == nil {
		return gerror.New("purchase file transport is unavailable")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if source.deps == nil || source.deps.files != nil {
		return gerror.New("purchase file transport binding is invalid")
	}
	source.deps.files = factory
	return nil
}

func (source *LazySource) TransferPurchaseFile(ctx context.Context, request resourceinterfaceprojectioncap.ExistingBusinessOutboundRequest, file resourceinterfaceprojectioncap.PurchaseFileTransfer) (string, error) {
	if source == nil || ctx == nil || ctx.Err() != nil || request.PluginID != "idc-peer" || request.Method != "provision" {
		return "", gerror.New("purchase file transfer is unavailable")
	}
	source.mu.RLock()
	deps := source.deps
	source.mu.RUnlock()
	if deps == nil || deps.files == nil {
		return "", gerror.New("purchase file transport is unavailable")
	}
	binding, err := loadBusinessBinding(ctx, deps.database, request.InterfaceInstanceNo, false)
	if err != nil || !matchesExistingBusiness(binding, request) {
		return "", gerror.New("purchase file interface evidence is invalid")
	}
	reader, err := channelconfig.NewReader(deps.database)
	if err != nil {
		return "", err
	}
	configuration, err := reader.ReadRevision(ctx, binding.ChannelNo, request.InterfaceConfigurationRevision)
	if err != nil || configuration.PluginID != "idc-peer" || !matchesConfiguration(binding, configuration) {
		return "", gerror.New("purchase file channel is unavailable")
	}
	return deps.files(ctx, configuration, file)
}

var _ resourceinterfaceprojectioncap.PurchaseFileTransferSource = (*LazySource)(nil)
