// This file projects staff-selected customer API metadata without stored secrets.
package admin

import (
	"context"
	"github.com/gogf/gf/v2/net/ghttp"
	"lina-plugin-idc-channel/backend/api/admin/v1"
	platformv1 "lina-plugin-idc-channel/backend/api/platform/v1"
	"time"
)

// CustomerPlatform reads a customer-bound application under existing staff authority.
func (c *ControllerV1) CustomerPlatform(ctx context.Context, req *v1.CustomerPlatformReq) (*v1.CustomerPlatformRes, error) {
	if r := ghttp.RequestFromCtx(ctx); r != nil {
		r.Response.Header().Set("Cache-Control", "no-store")
	}
	own, err := c.service.StaffPlatform(ctx, req.CustomerNo)
	if err != nil {
		return nil, err
	}
	result := &v1.CustomerPlatformRes{OverviewRes: platformv1.OverviewRes{Authorized: own.Authorized, AppID: own.AppID, DisplayName: own.DisplayName, Status: own.Status, Scopes: []string{}, SourceCIDRs: []string{}}}
	if len(own.Tokens) > 0 {
		token := own.Tokens[0]
		result.TokenID = token.ID
		result.Scopes = token.Scopes
		result.SourceCIDRs = token.SourceCIDRs
		result.CanReset = own.ResetEnabled && own.Status == "approved" && token.RevokedAt == nil && (token.ExpiresAt == nil || token.ExpiresAt.After(time.Now()))
		if token.ExpiresAt != nil {
			value := token.ExpiresAt.UnixMilli()
			result.ExpiresAt = &value
		}
	}
	return result, nil
}
