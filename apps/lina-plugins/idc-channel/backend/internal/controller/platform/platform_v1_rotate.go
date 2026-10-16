// This file returns a newly rotated secret once with browser caching disabled.
package platform

import (
	"context"
	"github.com/gogf/gf/v2/net/ghttp"
	"lina-plugin-idc-channel/backend/api/platform/v1"
)

// Rotate preserves all administrator-controlled policy and rejects stale versions.
func (c *ControllerV1) Rotate(ctx context.Context, req *v1.RotateReq) (*v1.RotateRes, error) {
	if r := ghttp.RequestFromCtx(ctx); r != nil {
		r.Response.Header().Set("Cache-Control", "no-store")
	}
	result, err := c.service.RotateOwnSecret(ctx, req.TokenID)
	if err != nil {
		return nil, err
	}
	return &v1.RotateRes{AppID: result.AppID, Secret: result.Token}, nil
}
