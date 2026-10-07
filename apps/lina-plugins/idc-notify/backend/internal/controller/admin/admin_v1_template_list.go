package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
)

func (c *ControllerV1) TemplateList(ctx context.Context, req *v1.TemplateListReq) (res *v1.TemplateListRes, err error) {
	items, err := c.notifications.ListTemplates(ctx, req.EventCode, req.ChannelCode, req.Limit)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.TemplateItem, len(items))
	for index := range items {
		result[index] = templateItem(items[index])
	}
	return &v1.TemplateListRes{Items: result}, nil
}
