package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) EventPolicyList(ctx context.Context, req *v1.EventPolicyListReq) (*v1.EventPolicyListRes, error) {
	service, err := notification.AsEventPolicyService(c.notifications)
	if err != nil {
		return nil, err
	}
	items, err := service.ListEventPolicies(ctx, req.Limit)
	if err != nil {
		return nil, err
	}
	result := make([]*v1.EventPolicyItem, len(items))
	for index := range items {
		result[index] = eventPolicyItem(items[index])
	}
	return &v1.EventPolicyListRes{Items: result}, nil
}

func eventPolicyItem(item notification.EventPolicy) *v1.EventPolicyItem {
	return &v1.EventPolicyItem{EventCode: item.EventCode, Topic: item.Topic,
		RecipientKind: string(item.RecipientKind), ChannelCode: item.ChannelCode,
		Enabled: item.Enabled, Version: item.Version, UpdatedAt: item.UpdatedAt.UnixMilli()}
}
