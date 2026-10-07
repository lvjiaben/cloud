package admin

import (
	"context"

	"lina-plugin-idc-notify/backend/api/admin/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

func (c *ControllerV1) DeliveryUnknownClosureCreate(ctx context.Context, req *v1.DeliveryUnknownClosureCreateReq) (res *v1.DeliveryUnknownClosureCreateRes, err error) {
	item, err := c.notifications.ManualCloseUnknown(ctx, notification.ManualClosureInput{TaskNo: req.TaskNo, ExpectedVersion: req.ExpectedVersion, OperationNo: req.OperationNo, Reason: req.Reason, Confirmed: req.Confirmed})
	if err != nil {
		return nil, err
	}
	return &v1.DeliveryUnknownClosureCreateRes{Item: deliveryItem(*item)}, nil
}
