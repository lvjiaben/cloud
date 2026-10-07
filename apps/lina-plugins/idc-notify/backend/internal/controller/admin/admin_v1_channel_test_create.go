// This file dispatches confirmed channel tests and scrubs raw request values before returning.

package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// ChannelTestCreate routes WeCom tests to the current operator's binding and other tests to typed targets.
func (c *ControllerV1) ChannelTestCreate(ctx context.Context, req *v1.ChannelTestCreateReq) (res *v1.ChannelTestCreateRes, err error) {
	variables := make(map[string]string, len(req.Variables))
	for key, value := range req.Variables {
		variables[key] = value
		req.Variables[key] = ""
		delete(req.Variables, key)
	}
	target := []byte(req.Target)
	req.Target = ""
	input := notification.ChannelTestInput{
		ChannelCode: req.ChannelCode, Target: target, TemplateNo: req.TemplateNo,
		TemplateVersion: req.TemplateVersion, Variables: variables, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey, Confirmed: req.Confirmed,
	}
	var item *notification.ChannelTestResult
	if req.ChannelCode == "wechat_work" {
		item, err = c.notifications.TestOwnWeCom(ctx, input)
	} else {
		item, err = c.notifications.TestChannel(ctx, input)
	}
	if err != nil {
		return nil, err
	}
	return &v1.ChannelTestCreateRes{Item: channelTestItem(item)}, nil
}
