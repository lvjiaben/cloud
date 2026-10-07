// This file delegates the bounded list to the existing account controller with
// the original request context; no customer identity is accepted from the page.

package serverpage

import (
	"context"

	accountv1 "lina-plugin-idc-resource/backend/api/account/v1"
	"lina-plugin-idc-resource/backend/api/serverpage/v1"
)

// ServerList rechecks the account's current identity and returns its safe page.
func (c *ControllerV1) ServerList(ctx context.Context, req *v1.ServerListReq) (*v1.ServerListRes, error) {
	page, err := c.account.ServerList(ctx, &accountv1.ServerListReq{Cursor: req.Cursor, Limit: req.Limit})
	if err != nil {
		return nil, err
	}
	return &v1.ServerListRes{Items: page.Items, NextCursor: page.NextCursor}, nil
}
