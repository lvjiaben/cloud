// This file implements creation of a session-bound WeCom authorization attempt.

package staff

import (
	"context"

	"lina-plugin-idc-notify/backend/api/staff/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// WeComOAuthAuthorizationCreate starts the current staff member's official authorization flow.
func (c *ControllerV1) WeComOAuthAuthorizationCreate(ctx context.Context, req *v1.WeComOAuthAuthorizationCreateReq) (res *v1.WeComOAuthAuthorizationCreateRes, err error) {
	item, err := c.notifications.StartOwnWeComBinding(ctx, notification.WeComBindingAuthorizationInput{
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.WeComOAuthAuthorizationCreateRes{
		AuthorizationURL: item.AuthorizationURL, QRLoginURL: item.QRLoginURL,
		ExpiresAt: item.ExpiresAt.UTC().UnixMilli(), Reused: item.Reused,
	}, nil
}
