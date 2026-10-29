package backend

import (
	"context"
	"lina-core/pkg/plugin/capability"
	"lina-plugin-idc-core/backend/cap/servercommercialcap"
	servercommercialspi "lina-plugin-idc-core/backend/cap/servercommercialcap/spi"
	"lina-plugin-idc-core/backend/cap/serveroperationresourcecap"
)

// Both HTTP and job startup inject their already-owned resource source. The
// core factory must not recursively construct a second resource service graph.
func loadNativeCommercialOwner(ctx context.Context, services capability.Services, source serveroperationresourcecap.Source) (servercommercialcap.Service, error) {
	snapshot, err := servercommercialspi.Freeze()
	if err != nil {
		return nil, err
	}
	return snapshot.Factory()(ctx, services, servercommercialcap.AllowedCallerPluginID, source)
}
