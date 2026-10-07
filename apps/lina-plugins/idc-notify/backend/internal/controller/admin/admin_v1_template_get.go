package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
)

func (c *ControllerV1) TemplateGet(ctx context.Context, req *v1.TemplateGetReq) (res *v1.TemplateGetRes, err error) {
	item, err := c.notifications.GetTemplate(ctx, req.TemplateNo, req.Version)
	if err != nil {
		return nil, err
	}
	return &v1.TemplateGetRes{Item: templateItem(*item)}, nil
}
