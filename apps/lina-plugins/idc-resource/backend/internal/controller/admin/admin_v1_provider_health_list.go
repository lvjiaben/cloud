// This file serves the bounded Provider health list API.

package admin

import (
	"context"

	"lina-plugin-idc-resource/backend/api/admin/v1"
	"lina-plugin-idc-resource/backend/internal/service/providerhealth"
)

// ProviderHealthList returns one locally projected keyset page.
func (c *ControllerV1) ProviderHealthList(ctx context.Context, req *v1.ProviderHealthListReq) (res *v1.ProviderHealthListRes, err error) {
	page, err := c.health.List(ctx, providerhealth.HealthListInput{
		ChannelNo: req.ChannelNo, Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.ProviderChannelHealthItem, len(page.Items))
	for index := range page.Items {
		items[index] = providerHealthItem(page.Items[index])
	}
	return &v1.ProviderHealthListRes{Items: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
