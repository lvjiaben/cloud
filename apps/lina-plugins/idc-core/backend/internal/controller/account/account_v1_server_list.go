package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerList(ctx context.Context, req *v1.ServerListReq) (res *v1.ServerListRes, err error) {
	page, err := c.servers.ListCurrentCustomer(ctx, serveroperation.DirectoryFilter{
		Limit: req.Limit, Cursor: req.Cursor,
	})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	items := make([]*v1.CustomerServer, 0, len(page.Items))
	for index := range page.Items {
		items = append(items, toAPICustomerServer(&page.Items[index]))
	}
	return &v1.ServerListRes{Items: items, NextCursor: page.NextCursor}, nil
}
