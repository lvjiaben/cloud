// This file adapts customer authentication to bounded public plugin services.
package public

import (
	"context"
	"lina-plugin-idc-core/backend/cap/customerlogincap"
	"lina-plugin-idc-notify/backend/api/public/v1"
	"lina-plugin-idc-notify/backend/internal/service/notification"
)

// CustomerPasswordLogin preserves host identity ownership and does not return contact projections.
func (c *ControllerV1) CustomerPasswordLogin(ctx context.Context, req *v1.CustomerPasswordLoginReq) (*v1.CustomerPasswordLoginRes, error) {
	password := req.Password
	req.Password = ""
	item, err := c.notifications.CustomerPasswordLogin(ctx, notification.CustomerLoginInput{Kind: customerlogincap.Kind(req.Kind), Account: req.Account, Password: password})
	if err != nil {
		return nil, err
	}
	return &v1.CustomerPasswordLoginRes{AccessToken: item.AccessToken, RefreshToken: item.RefreshToken}, nil
}
