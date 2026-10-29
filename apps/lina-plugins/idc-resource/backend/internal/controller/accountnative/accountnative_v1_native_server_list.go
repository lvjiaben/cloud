// This file returns one bounded customer-owned native server page.
package accountnative

import (
	"context"
	"lina-plugin-idc-resource/backend/api/accountnative/v1"
)

func (c *ControllerV1) NativeServerList(ctx context.Context, req *v1.NativeServerListReq) (*v1.NativeServerListRes, error) {
	page, err := c.service.List(ctx, req.Page, req.Limit)
	if err != nil {
		return nil, publicError(err)
	}
	result := &v1.NativeServerListRes{Items: make([]v1.NativeServerListItem, 0, len(page.Items)), Total: page.Total}
	for _, item := range page.Items {
		result.Items = append(result.Items, v1.NativeServerListItem{ServerNo: item.ServerNo, ProductNameZhCN: item.ProductNameZhCN, ProductNameEnUS: item.ProductNameEnUS, State: item.State})
	}
	return result, nil
}
