package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) TemplateUpdate(ctx context.Context, req *v1.TemplateUpdateReq) (res *v1.TemplateUpdateRes, err error) {
	var sms *notification.SMSTemplateMapping
	if req.SMS != nil {
		mapping := make(map[string]string, len(req.SMS.DataMapping))
		for key, value := range req.SMS.DataMapping {
			mapping[key] = value
		}
		sms = &notification.SMSTemplateMapping{ProviderCode: req.SMS.ProviderCode, TemplateID: req.SMS.TemplateID,
			VariableOrder: append([]string(nil), req.SMS.VariableOrder...), DataMapping: mapping}
	}
	item, err := c.notifications.SaveTemplate(ctx, notification.TemplateWriteInput{TemplateNo: req.TemplateNo,
		EventCode: req.EventCode, ChannelCode: req.ChannelCode, Locale: req.Locale,
		Subject: req.Subject, Body: req.Body, Status: req.Status,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
		SMS: sms})
	if err != nil {
		return nil, err
	}
	return &v1.TemplateUpdateRes{Item: templateItem(*item)}, nil
}
