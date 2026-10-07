// This file implements bounded list and ownership-checked detail projections for customer servers.

package customer

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-plugin-idc-resource/backend/cap/resourcecap"
	api "lina-plugin-idc-zjmfcloud/backend/api/customer/v1"
)

// ServerList returns one bounded page from the resource owner's current-customer service.
func (controller *Controller) ServerList(ctx context.Context, req *api.ServerListReq) (*api.ServerListRes, error) {
	if controller == nil || controller.servers == nil || controller.initErr != nil || ctx == nil || req == nil {
		return nil, gerror.New("customer server directory is unavailable")
	}
	page, err := controller.servers.ListCurrentCustomerServers(ctx, resourcecap.CurrentCustomerServerListRequest{Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, gerror.Wrap(err, "list current customer servers")
	}
	result := &api.ServerListRes{Items: make([]*api.ServerItem, len(page.Items)), NextCursor: page.NextCursor}
	for index := range page.Items {
		result.Items[index] = projectServerItem(page.Items[index])
	}
	return result, nil
}

// ServerDetail returns one server after the resource owner performs current-customer ownership checks.
func (controller *Controller) ServerDetail(ctx context.Context, req *api.ServerDetailReq) (*api.ServerDetailRes, error) {
	if controller == nil || controller.servers == nil || controller.initErr != nil || ctx == nil || req == nil {
		return nil, gerror.New("customer server directory is unavailable")
	}
	result, err := controller.servers.GetCurrentCustomerServer(ctx, resourcecap.CurrentCustomerServerGetRequest{ServerNo: req.ServerNo})
	if err != nil {
		return nil, gerror.Wrap(err, "get current customer server")
	}
	return &api.ServerDetailRes{Item: projectServerItem(result.Item)}, nil
}

func projectServerItem(item resourcecap.CurrentCustomerServerItem) *api.ServerItem {
	actions := make([]string, len(item.AvailableActions))
	for index := range item.AvailableActions {
		actions[index] = string(item.AvailableActions[index])
	}
	return &api.ServerItem{
		ServerNo: item.ServerNo, ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS,
		ChannelName: item.ChannelName, PoolName: item.PoolName, Kind: item.Kind, State: item.State,
		LatestOperation: item.LatestOperation, AvailableActions: actions, BoundAt: item.BoundAt, UpdatedAt: item.UpdatedAt,
	}
}
