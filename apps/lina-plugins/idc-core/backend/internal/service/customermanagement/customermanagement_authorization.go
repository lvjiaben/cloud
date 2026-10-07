// This file enforces exact staff permission and representable data scope.

package customermanagement

import (
	"context"
	"strconv"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/customer"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	permissionCustomerList     = "idc:admin:customer:list"
	permissionCustomerQuery    = "idc:admin:customer:query"
	permissionCustomerTagBatch = "idc:admin:customer:tag:batch"
	permissionCustomerUpdate   = "idc:admin:customer:update"
)

type staffDataScope int

const (
	staffDataScopeAll    staffDataScope = 1
	staffDataScopeTenant staffDataScope = 2
)

func (s *serviceImpl) authorize(ctx context.Context, permission string) error {
	return authorizeStaff(ctx, s.bizCtxSvc, s.principal, s.authorization, permission)
}

func authorizeStaff(
	ctx context.Context,
	bizCtxSvc bizctxcap.Service,
	principalSvc principal.Reader,
	authorization authz.Service,
	permission string,
) error {
	current := bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported ||
		(staffDataScope(current.DataScope) != staffDataScopeAll &&
			staffDataScope(current.DataScope) != staffDataScopeTenant) {
		return bizerr.NewCode(customer.CodeNotAccessible)
	}
	audience, err := principalSvc.Current(ctx)
	if err != nil {
		return bizerr.WrapCode(err, customer.CodeNotAccessible)
	}
	if audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff ||
		audience.Status != principal.AudienceStatusActive || audience.CustomerNo != "" {
		return bizerr.NewCode(customer.CodeNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission),
		authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return bizerr.WrapCode(err, customer.CodeNotAccessible)
	}
	platformAdmin, err := authorization.IsPlatformAdmin(
		ctx,
		authz.UserID(strconv.Itoa(current.UserID)),
	)
	if err != nil {
		return bizerr.WrapCode(err, customer.CodeNotAccessible)
	}
	if !grants[authz.PermissionKey(permission)] &&
		!grants[authz.PermissionKey(idccap.HostPermissionWildcard)] && !platformAdmin {
		return bizerr.NewCode(customer.CodeNotAccessible)
	}
	return nil
}
