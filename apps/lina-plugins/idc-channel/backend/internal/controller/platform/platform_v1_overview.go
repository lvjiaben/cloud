// This file projects safe authorization metadata and does not expose stored secrets.
package platform

import (
	"context"
	"github.com/gogf/gf/v2/net/ghttp"
	"lina-plugin-idc-channel/backend/api/platform/v1"
	"time"
)

// Overview reads the authenticated customer's binding, including disabled history.
func (c *ControllerV1) Overview(ctx context.Context, _ *v1.OverviewReq) (*v1.OverviewRes, error) {
	if r := ghttp.RequestFromCtx(ctx); r != nil {
		r.Response.Header().Set("Cache-Control", "no-store")
	}
	own, err := c.service.OwnPlatform(ctx)
	if err != nil {
		return nil, err
	}
	result := &v1.OverviewRes{Authorized: own.Authorized, AppID: own.AppID, DisplayName: own.DisplayName, Status: own.Status, Scopes: []string{}, SourceCIDRs: []string{}}
	if len(own.Tokens) > 0 {
		token := own.Tokens[0]
		result.TokenID = token.ID
		result.Scopes = token.Scopes
		result.SourceCIDRs = token.SourceCIDRs
		result.CanReset = own.ResetEnabled && own.Status == "approved" && token.RevokedAt == nil && (token.ExpiresAt == nil || token.ExpiresAt.After(time.Now()))
		if token.ExpiresAt != nil {
			millis := token.ExpiresAt.UnixMilli()
			result.ExpiresAt = &millis
		}
	}
	return result, nil
}
