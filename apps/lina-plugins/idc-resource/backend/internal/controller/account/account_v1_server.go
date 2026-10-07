package account

import (
	"context"

	"lina-plugin-idc-resource/backend/api/account/v1"
	"lina-plugin-idc-resource/backend/cap/resourcecap"
)

func (controller *ControllerV1) ServerList(ctx context.Context, req *v1.ServerListReq) (*v1.ServerListRes, error) {
	page, err := controller.servers.ListCurrentCustomerServers(ctx, resourcecap.CurrentCustomerServerListRequest{
		Cursor: req.Cursor,
		Limit:  req.Limit,
	})
	if err != nil {
		return nil, err
	}
	result := &v1.ServerListRes{Items: make([]*v1.ServerItem, len(page.Items)), NextCursor: page.NextCursor}
	for index := range page.Items {
		result.Items[index] = serverItem(page.Items[index])
	}
	return result, nil
}

func (controller *ControllerV1) ServerDetail(ctx context.Context, req *v1.ServerDetailReq) (*v1.ServerDetailRes, error) {
	result, err := controller.servers.GetCurrentCustomerServer(ctx, resourcecap.CurrentCustomerServerGetRequest{
		ServerNo: req.ServerNo,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerDetailRes{Item: serverItem(result.Item)}, nil
}

func serverItem(item resourcecap.CurrentCustomerServerItem) *v1.ServerItem {
	actions := make([]string, len(item.AvailableActions))
	for index := range item.AvailableActions {
		actions[index] = string(item.AvailableActions[index])
	}
	return &v1.ServerItem{
		ServerNo:        item.ServerNo,
		ProductNameZhCN: item.ProductNameZhCN,
		ProductNameEnUS: item.ProductNameEnUS,
		ChannelName:     item.ChannelName,
		PoolName:        item.PoolName,
		Kind:            item.Kind, State: item.State,
		LatestOperation: item.LatestOperation, AvailableActions: actions,
		BoundAt: item.BoundAt, UpdatedAt: item.UpdatedAt,
	}
}
