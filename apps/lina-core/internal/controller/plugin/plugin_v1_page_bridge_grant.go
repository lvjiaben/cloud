package plugin

import (
	"context"

	v1 "lina-core/api/plugin/v1"
	middlewaresvc "lina-core/internal/service/middleware"
	pluginsvc "lina-core/internal/service/plugin"
	"lina-core/pkg/bizerr"
)

// PageBridgeGrant returns an opaque short-lived ticket for one exact request.
func (c *ControllerV1) PageBridgeGrant(ctx context.Context, req *v1.PageBridgeGrantReq) (*v1.PageBridgeGrantRes, error) {
	businessCtx := c.bizCtxSvc.Get(ctx)
	if businessCtx == nil || businessCtx.UserId <= 0 || businessCtx.TokenId == "" {
		return nil, bizerr.NewCode(middlewaresvc.CodeMiddlewarePermissionCurrentUserMissing)
	}
	accessContext, err := c.roleSvc.GetUserAccessContext(ctx, businessCtx.UserId)
	if err != nil {
		return nil, bizerr.WrapCode(err, middlewaresvc.CodeMiddlewarePermissionContextLoadFailed)
	}
	if accessContext == nil {
		return nil, bizerr.NewCode(middlewaresvc.CodeMiddlewarePermissionCurrentUserMissing)
	}
	out, err := c.pluginSvc.IssuePageBridgeGrant(ctx, pluginsvc.PageBridgeGrantInput{
		TokenID: businessCtx.TokenId, UserID: int32(businessCtx.UserId), TenantID: int32(businessCtx.TenantId),
		PluginID: req.PluginId, Host: req.Host, SurfaceKey: req.SurfaceKey,
		SurfaceVersion: req.SurfaceVersion, Capability: req.Capability,
		Method: req.Method, Path: req.Path, Permissions: append([]string(nil), accessContext.Permissions...),
		DataScope: accessContext.DataScope, DataScopeUnsupported: accessContext.DataScopeUnsupported,
		IsSuperAdmin: accessContext.IsSuperAdmin, ActingAsTenant: businessCtx.ActingAsTenant,
		IsImpersonation: businessCtx.IsImpersonation,
	})
	if err != nil {
		return nil, err
	}
	return &v1.PageBridgeGrantRes{Grant: out.Grant, ExpiresAt: out.ExpiresAt, OwnerPluginId: out.OwnerPluginID}, nil
}
