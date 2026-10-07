// This file implements removal of the current staff member's own WeCom binding.

package staff

import (
	"context"

	"lina-plugin-idc-notify/backend/api/staff/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// WeComBindingRemovalCreate starts an idempotent self-service binding removal.
func (c *ControllerV1) WeComBindingRemovalCreate(ctx context.Context, req *v1.WeComBindingRemovalCreateReq) (res *v1.WeComBindingRemovalCreateRes, err error) {
	item, err := c.notifications.RemoveOwnWeComBinding(ctx, notification.WeComBindingRemovalInput{
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey, Confirmed: req.Confirmed,
	})
	if err != nil {
		return nil, err
	}
	return &v1.WeComBindingRemovalCreateRes{Item: weComBindingItem(item)}, nil
}
