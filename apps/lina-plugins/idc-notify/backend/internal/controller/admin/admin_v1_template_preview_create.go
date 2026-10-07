package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) TemplatePreviewCreate(ctx context.Context, req *v1.TemplatePreviewCreateReq) (res *v1.TemplatePreviewCreateRes, err error) {
	item, err := c.notifications.PreviewTemplate(ctx, notification.TemplatePreviewInput{TemplateNo: req.TemplateNo, Version: req.Version, Variables: req.Variables})
	if err != nil {
		return nil, err
	}
	return &v1.TemplatePreviewCreateRes{Subject: item.Subject, Body: item.Body}, nil
}
