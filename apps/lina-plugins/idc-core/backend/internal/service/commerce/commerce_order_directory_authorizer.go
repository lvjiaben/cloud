// This file derives order-directory staff authority from host-published context.
package commerce

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	permissionOrderDirectoryList   = "idc:admin:commerce:order:list"
	permissionOrderDirectoryDetail = "idc:admin:commerce:order:query"
)

type managementOrderAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
}

var _ ManagementOrderAuthorizer = (*managementOrderAuthorizerImpl)(nil)

func NewManagementOrderAuthorizer(
	bizCtxSvc bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
) (ManagementOrderAuthorizer, error) {
	if isNilOrderDirectoryDependency(bizCtxSvc) || isNilOrderDirectoryDependency(principalReader) ||
		isNilOrderDirectoryDependency(authorization) {
		return nil, gerror.New("management order authorizer requires bizctx, principal, and authz services")
	}
	return &managementOrderAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, principal: principalReader, authorization: authorization,
	}, nil
}

func (a *managementOrderAuthorizerImpl) AuthorizeOrderList(ctx context.Context) error {
	return a.authorize(ctx, permissionOrderDirectoryList)
}

func (a *managementOrderAuthorizerImpl) AuthorizeOrderDetail(ctx context.Context, orderNo string) error {
	if !orderLifecycleIdentifierPattern.MatchString(orderNo) || len(orderNo) > 40 {
		return bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	return a.authorize(ctx, permissionOrderDirectoryDetail)
}

func (a *managementOrderAuthorizerImpl) authorize(ctx context.Context, permission string) error {
	if a == nil {
		return bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 ||
		current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported ||
		(current.DataScope != 1 && current.DataScope != 2) {
		return bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return bizerr.WrapCode(err, CodeOrderDirectoryNotAccessible)
	}
	platformAdmin, err := a.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil {
		return bizerr.WrapCode(err, CodeOrderDirectoryNotAccessible)
	}
	if !grants[authz.PermissionKey(permission)] &&
		!grants[authz.PermissionKey(idccap.HostPermissionWildcard)] && !platformAdmin {
		return bizerr.NewCode(CodeOrderDirectoryNotAccessible)
	}
	return nil
}
