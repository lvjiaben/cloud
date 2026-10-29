// This file compares immutable connection revisions inside their resource owner.
// Only equality escapes the owner boundary; endpoints and secret references do not.
package interfacecandidate

import (
	"context"
	"github.com/gogf/gf/v2/errors/gerror"
	"lina-plugin-idc-core/backend/cap/resourceinterfaceprojectioncap"
	"lina-plugin-idc-resource/backend/internal/service/channelconfig"
)

var _ resourceinterfaceprojectioncap.ConfigurationRevisionComparer = (*LazySource)(nil)

// EquivalentConfigurationRevisions returns only equality for the same currently bound interface.
func (source *LazySource) EquivalentConfigurationRevisions(ctx context.Context, interfaceNo, savedRevision, currentRevision string) (bool, error) {
	if source == nil || ctx == nil || ctx.Err() != nil || savedRevision == "" || currentRevision == "" {
		return false, gerror.New("resource configuration comparison is unavailable")
	}
	source.mu.RLock()
	deps := source.deps
	source.mu.RUnlock()
	if deps == nil {
		return false, gerror.New("resource configuration comparison dependencies are unavailable")
	}
	binding, err := loadBusinessBinding(ctx, deps.database, interfaceNo, false)
	if err != nil {
		return false, err
	}
	if binding.ConfigurationRevision != currentRevision {
		return false, nil
	}
	reader, err := channelconfig.NewReader(deps.database)
	if err != nil {
		return false, err
	}
	// Exactly two immutable revisions are read; work never scales with products.
	saved, err := reader.ReadRevision(ctx, binding.ChannelNo, savedRevision)
	if err != nil {
		return false, err
	}
	current, err := reader.ReadRevision(ctx, binding.ChannelNo, currentRevision)
	if err != nil {
		return false, err
	}
	if saved.Config.Endpoint != current.Config.Endpoint || string(saved.Config.Settings) != string(current.Config.Settings) || len(saved.Config.SecretBindings) != len(current.Config.SecretBindings) {
		return false, nil
	}
	refs := make(map[string]string, len(saved.Config.SecretBindings))
	for _, item := range saved.Config.SecretBindings {
		refs[item.Name] = item.Handle.OpaqueRef()
	}
	for _, item := range current.Config.SecretBindings {
		if refs[item.Name] != item.Handle.OpaqueRef() {
			return false, nil
		}
	}
	return true, nil
}
