package serviceaction

import (
	"context"
	"strconv"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/cap/identitycap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	PermissionList     = "idc:admin:service-entitlement:list"
	PermissionAction   = "idc:admin:service-entitlement:action"
	PermissionTakeover = "idc:admin:service-entitlement:takeover"
	PermissionDownload = "idc:admin:service-entitlement:download"
)

type Access interface {
	Customer(context.Context) (Actor, error)
	Admin(context.Context, string) (Actor, error)
}

type productionAccess struct {
	bizCtx     bizctxcap.Service
	principals principal.Reader
	authz      authz.Service
	customers  identitycap.CurrentCustomerReader
}

func NewAccess(bizCtx bizctxcap.Service, principals principal.Reader, authorization authz.Service, customers identitycap.CurrentCustomerReader) (Access, error) {
	if bizCtx == nil || principals == nil || authorization == nil || customers == nil {
		return nil, bizerr.NewCode(CodeUnavailable)
	}
	return &productionAccess{bizCtx: bizCtx, principals: principals, authz: authorization, customers: customers}, nil
}

func (a *productionAccess) Customer(ctx context.Context) (Actor, error) {
	subject, err := a.customers.CurrentCustomer(ctx)
	if err != nil || subject == nil || subject.ActorUserID < 1 || subject.CustomerNo == "" {
		return Actor{}, bizerr.NewCode(CodeNotAccessible)
	}
	return Actor{Audience: AudienceCustomer, CustomerNo: subject.CustomerNo, UserID: subject.ActorUserID}, nil
}

func (a *productionAccess) Admin(ctx context.Context, permission string) (Actor, error) {
	allowed := permission == PermissionList || permission == PermissionAction || permission == PermissionTakeover || permission == PermissionDownload
	current := a.bizCtx.Current(ctx)
	if !allowed || current.UserID < 1 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported || current.DataScope != 1 && current.DataScope != 2 {
		return Actor{}, bizerr.NewCode(CodeNotAccessible)
	}
	principalValue, err := a.principals.Current(ctx)
	if err != nil || principalValue == nil || principalValue.UserID != current.UserID || principalValue.Code != principal.AudienceStaff || principalValue.Status != principal.AudienceStatusActive {
		return Actor{}, bizerr.NewCode(CodeNotAccessible)
	}
	grants, err := a.authz.BatchHasPermissions(ctx, []authz.PermissionKey{authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard)})
	if err != nil {
		return Actor{}, bizerr.NewCode(CodeNotAccessible)
	}
	admin, err := a.authz.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil || !admin && !grants[authz.PermissionKey(permission)] && !grants[authz.PermissionKey(idccap.HostPermissionWildcard)] {
		return Actor{}, bizerr.NewCode(CodeNotAccessible)
	}
	return Actor{Audience: AudienceAdmin, UserID: current.UserID}, nil
}
