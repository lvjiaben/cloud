// This file implements the authenticated callback that completes a WeCom binding.

package staff

import (
	"context"

	"lina-plugin-idc-notify/backend/api/staff/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// WeComOAuthCallbackGet consumes the official callback for the same staff session.
func (c *ControllerV1) WeComOAuthCallbackGet(ctx context.Context, req *v1.WeComOAuthCallbackGetReq) (res *v1.WeComOAuthCallbackGetRes, err error) {
	item, err := c.notifications.CompleteOwnWeComBinding(ctx, notification.WeComBindingCompletionInput{
		State: req.State, Code: req.Code,
	})
	if err != nil {
		return nil, err
	}
	return &v1.WeComOAuthCallbackGetRes{Item: weComBindingItem(item)}, nil
}
