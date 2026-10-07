package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
)

func (c *ControllerV1) DeliveryResendEligibilityGet(ctx context.Context, req *v1.DeliveryResendEligibilityGetReq) (res *v1.DeliveryResendEligibilityGetRes, err error) {
	item, err := c.notifications.GetResendEligibility(ctx, req.TaskNo)
	if err != nil {
		return nil, err
	}
	return &v1.DeliveryResendEligibilityGetRes{Item: eligibilityItem(item)}, nil
}
