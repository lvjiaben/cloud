// This file authorizes shared merchant configuration and resolves customer
// identity without accepting a customer identifier from an HTTP request.
package productdirectory

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/service/principal"
	"strconv"
)

type productionAccess struct {
	bizCtx     bizctxcap.Service
	principals principal.Reader
	authz      authz.Service
	customers  identitycap.CurrentCustomerReader
}

// NewAccess receives startup-owned identity, permission and shared context services explicitly.
func NewAccess(bizCtx bizctxcap.Service, principals principal.Reader, authorization authz.Service, customers identitycap.CurrentCustomerReader) (Access, error) {
	if bizCtx == nil || principals == nil || authorization == nil || customers == nil {
		return nil, bizerr.NewCode(CodeNotAccessible)
	}
	return &productionAccess{bizCtx: bizCtx, principals: principals, authz: authorization, customers: customers}, nil
}

// Customer resolves the active customer through the authoritative identity owner.
func (a *productionAccess) Customer(ctx context.Context) (string, error) {
	subject, err := a.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || subject.ActorUserID < 1 || subject.CustomerNo == "" {
		return "", bizerr.NewCode(CodeNotAccessible)
	}
	return subject.CustomerNo, nil
}

// Admin authorizes staff audience and explicit global configuration permissions.
func (a *productionAccess) Admin(ctx context.Context, permission string) (int, error) {
	current := a.bizCtx.Current(ctx)
	if permission != PermissionList && permission != PermissionCreate && permission != PermissionUpdate || current.UserID < 1 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported || current.DataScope != 1 && current.DataScope != 2 {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	actor, err := a.principals.Current(ctx)
	if err != nil || actor == nil || actor.UserID != current.UserID || actor.Code != principal.AudienceStaff || actor.Status != principal.AudienceStatusActive {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	grants, err := a.authz.BatchHasPermissions(ctx, []authz.PermissionKey{authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard)})
	if err != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	admin, err := a.authz.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil || !admin && !grants[authz.PermissionKey(permission)] && !grants[authz.PermissionKey(idccap.HostPermissionWildcard)] {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return current.UserID, nil
}
