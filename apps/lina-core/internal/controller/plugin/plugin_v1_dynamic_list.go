// This file exposes the public dynamic plugin runtime-state endpoint on the
// shared plugin controller.

package plugin

import (
	"context"

	v1 "lina-core/api/plugin/v1"
	pluginsvc "lina-core/internal/service/plugin"
	"lina-core/pkg/statusflag"
)

// DynamicList returns public dynamic-plugin states for shell slot rendering.
func (c *ControllerV1) DynamicList(ctx context.Context, req *v1.DynamicListReq) (res *v1.DynamicListRes, err error) {
	out, err := c.pluginSvc.ListRuntimeStates(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*v1.PluginDynamicItem, 0, len(out.List))
	businessCtx := c.bizCtxSvc.Get(ctx)
	authenticated := businessCtx != nil && businessCtx.UserId > 0
	for _, item := range out.List {
		items = append(items, projectPluginDynamicItem(item, authenticated))
	}
	return &v1.DynamicListRes{List: items}, nil
}

func projectPluginDynamicItem(item *pluginsvc.PluginDynamicStateItem, authenticated bool) *v1.PluginDynamicItem {
	if item == nil {
		return nil
	}
	customerSurfaces := make([]*v1.PluginRuntimeFrontendSurfaceItem, 0, len(item.CustomerSurfaces))
	if authenticated {
		for _, surface := range item.CustomerSurfaces {
			if surface == nil {
				continue
			}
			customerSurfaces = append(customerSurfaces, &v1.PluginRuntimeFrontendSurfaceItem{
				PluginId: surface.PluginID, PluginType: v1.PluginType(surface.PluginType), Key: surface.Key,
				Version: surface.Version, Title: surface.Title, Icon: surface.Icon, Permission: surface.Permission,
				Slot: surface.Slot, RoutePath: surface.RoutePath, Entry: surface.Entry, EntryUrl: surface.EntryURL,
				Status: surface.Status, BridgeCapabilities: append([]string(nil), surface.BridgeCapabilities...),
			})
		}
	}
	return &v1.PluginDynamicItem{
		Id: item.Id, Installed: statusflag.Installation(item.Installed), Enabled: statusflag.Enabled(item.Enabled),
		Version: item.Version, Generation: item.Generation, StatusKey: item.StatusKey,
		RuntimeState: v1.RuntimeState(item.RuntimeState.String()), Type: v1.PluginType(item.Type),
		CustomerSurfaces: customerSurfaces,
	}
}
