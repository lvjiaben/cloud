// This file authorizes staff-controlled customer account-state transitions.

package customer

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
)

const permissionCustomerAccountUpdate = "idc:admin:customer:update"

type staffDataScope int

const (
	staffDataScopeAll    staffDataScope = 1
	staffDataScopeTenant staffDataScope = 2
)

type accountStateAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	authorization authz.Service
}

var _ AccountStateAuthorizer = (*accountStateAuthorizerImpl)(nil)

// NewAccountStateAuthorizer creates the production authorization seam for
// customer lifecycle writes. It uses host-governed permission evidence and
// rejects data scopes that cannot yet be represented by customer ownership.
func NewAccountStateAuthorizer(
	bizCtxSvc bizctxcap.Service,
	authorization authz.Service,
) (AccountStateAuthorizer, error) {
	if nilInterface(bizCtxSvc) || nilInterface(authorization) {
		return nil, gerror.New("customer account-state authorizer requires host bizctx and authz services")
	}
	return &accountStateAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, authorization: authorization,
	}, nil
}

// AuthorizeTransition requires one direct management identity, the exact
// account-update grant (or host wildcard/platform authority), and an all or
// tenant data scope. Department and self scopes fail closed until a stable
// customer ownership projection can constrain the target in SQL.
func (a *accountStateAuthorizerImpl) AuthorizeTransition(
	ctx context.Context,
	customerNo string,
) error {
	if a == nil || nilInterface(a.bizCtxSvc) || nilInterface(a.authorization) ||
		!validCustomerNo(customerNo) {
		return bizerr.NewCode(CodeNotAccessible)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		!hasBroadCustomerDataScope(current) {
		return bizerr.NewCode(CodeNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permissionCustomerAccountUpdate),
		authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return bizerr.WrapCode(err, CodeNotAccessible)
	}
	platformAdmin, err := a.authorization.IsPlatformAdmin(
		ctx,
		authz.UserID(strconv.Itoa(current.UserID)),
	)
	if err != nil {
		return bizerr.WrapCode(err, CodeNotAccessible)
	}
	if !grants[authz.PermissionKey(permissionCustomerAccountUpdate)] &&
		!grants[authz.PermissionKey(idccap.HostPermissionWildcard)] &&
		!platformAdmin {
		return bizerr.NewCode(CodeNotAccessible)
	}
	return nil
}

func hasBroadCustomerDataScope(current bizctxcap.CurrentContext) bool {
	if current.DataScopeUnsupported {
		return false
	}
	switch staffDataScope(current.DataScope) {
	case staffDataScopeAll, staffDataScopeTenant:
		return true
	default:
		return false
	}
}
