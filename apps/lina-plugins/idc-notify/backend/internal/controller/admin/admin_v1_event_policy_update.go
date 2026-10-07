package admin

import (
	"context"

	"lina-plugin-idc-core/backend/cap/notificationcap"
	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) EventPolicyUpdate(ctx context.Context, req *v1.EventPolicyUpdateReq) (*v1.EventPolicyUpdateRes, error) {
	service, err := notification.AsEventPolicyService(c.notifications)
	if err != nil {
		return nil, err
	}
	item, err := service.UpdateEventPolicy(ctx, notification.EventPolicyWriteInput{
		EventCode: req.EventCode, RecipientKind: notificationcap.RecipientKind(req.RecipientKind),
		ChannelCode: req.ChannelCode, Enabled: req.Enabled, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.EventPolicyUpdateRes{Item: eventPolicyItem(*item)}, nil
}
