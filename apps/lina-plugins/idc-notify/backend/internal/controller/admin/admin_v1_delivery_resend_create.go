package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) DeliveryResendCreate(ctx context.Context, req *v1.DeliveryResendCreateReq) (res *v1.DeliveryResendCreateRes, err error) {
	item, err := c.notifications.ManualResend(ctx, notification.ManualResendInput{TaskNo: req.TaskNo, ExpectedVersion: req.ExpectedVersion, OperationNo: req.OperationNo, Reason: req.Reason, Confirmed: req.Confirmed})
	if err != nil {
		return nil, err
	}
	return &v1.DeliveryResendCreateRes{Item: deliveryItem(*item)}, nil
}
