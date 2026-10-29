// This file accepts optional server login choices and returns no password or
// owner-private secret reference to the browser.
package account

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/purchaseserverlogin"
)

func (c *ControllerV1) PurchaseServerLoginCreate(ctx context.Context, req *v1.PurchaseServerLoginCreateReq) (*v1.PurchaseServerLoginCreateRes, error) {
	password := req.Password
	req.Password = ""
	item, err := c.serverLogins.Create(ctx, purchaseserverlogin.Input{PublicSlug: req.PublicSlug, ConfigurationRevision: req.ConfigurationRevision, Hostname: req.Hostname, Password: password, PreviousServerLoginToken: req.PreviousServerLoginToken})
	if err != nil {
		return nil, err
	}
	return &v1.PurchaseServerLoginCreateRes{ServerLoginToken: item.ServerLoginToken, Hostname: item.Hostname, PasswordConfigured: item.PasswordConfigured}, nil
}
