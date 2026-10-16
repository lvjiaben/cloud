// This file returns the replacement secret once and disables browser caching.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/net/ghttp"
	"lina-plugin-idc-channel/backend/api/admin/v1"
	platformv1 "lina-plugin-idc-channel/backend/api/platform/v1"
)

// CustomerSecretRotate preserves current authorization policy and rejects stale versions.
func (c *ControllerV1) CustomerSecretRotate(ctx context.Context, req *v1.CustomerSecretRotateReq) (*v1.CustomerSecretRotateRes, error) {
	if r := ghttp.RequestFromCtx(ctx); r != nil {
		r.Response.Header().Set("Cache-Control", "no-store")
	}
	result, err := c.service.RotateStaffSecret(ctx, req.CustomerNo, req.TokenID)
	if err != nil {
		return nil, err
	}
	return &v1.CustomerSecretRotateRes{RotateRes: platformv1.RotateRes{AppID: result.AppID, Secret: result.Token}}, nil
}
