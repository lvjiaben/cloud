// This file adapts host identity, exact permissions, data scope, and catalog publication authority into action-specific authorization evidence.

package productautomation

import (
	"context"
	"strconv"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/catalog"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	PermissionInterfaceView      = "idc:admin:catalog:automation-interface:view"
	PermissionInterfaceManage    = "idc:admin:catalog:automation-interface:manage"
	PermissionBindingWrite       = "idc:admin:catalog:fulfillment-binding:write"
	PermissionConfigurationWrite = "idc:admin:catalog:configuration:write"
	permissionProductQuery       = "idc:admin:catalog:product:query"
)

type managementAuthorizer struct {
	bizCtx      bizctxcap.Service
	principal   principal.Reader
	authz       authz.Service
	publication catalog.PublicationAuthorizer
}

var _ Authorizer = (*managementAuthorizer)(nil)

func NewManagementAuthorizer(bizCtx bizctxcap.Service, reader principal.Reader, authorization authz.Service, publication catalog.PublicationAuthorizer) (Authorizer, error) {
	if nilDependency(bizCtx) || nilDependency(reader) || nilDependency(authorization) || nilDependency(publication) {
		return nil, gerror.New("product automation authorizer requires bizctx, principal, and authz services")
	}
	return &managementAuthorizer{bizCtx: bizCtx, principal: reader, authz: authorization, publication: publication}, nil
}

func (a *managementAuthorizer) AuthorizeProviderCandidates(ctx context.Context) (Authorization, error) {
	return a.authorize(ctx, PermissionInterfaceView)
}
func (a *managementAuthorizer) AuthorizeInterfaceList(ctx context.Context) (Authorization, error) {
	return a.authorize(ctx, PermissionInterfaceView)
}
func (a *managementAuthorizer) AuthorizeInterfaceCreate(ctx context.Context) (Authorization, error) {
	return a.authorize(ctx, PermissionInterfaceManage)
}
func (a *managementAuthorizer) AuthorizeInterfaceRename(ctx context.Context, no string) (Authorization, error) {
	if validateBusinessNo(no, "pfi") != nil {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return a.authorize(ctx, PermissionInterfaceManage)
}
func (a *managementAuthorizer) AuthorizeInterfaceDelete(ctx context.Context, no string) (Authorization, error) {
	if validateBusinessNo(no, "pfi") != nil {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return a.authorize(ctx, PermissionInterfaceManage)
}
func (a *managementAuthorizer) AuthorizeInterfaceStop(ctx context.Context, no string) (Authorization, error) {
	return a.AuthorizeInterfaceRename(ctx, no)
}
func (a *managementAuthorizer) AuthorizeInterfaceResume(ctx context.Context, no string) (Authorization, error) {
	return a.AuthorizeInterfaceRename(ctx, no)
}
func (a *managementAuthorizer) AuthorizeInterfaceConfiguration(ctx context.Context, no string) (Authorization, error) {
	return a.AuthorizeInterfaceRename(ctx, no)
}
func (a *managementAuthorizer) AuthorizeBindingRead(ctx context.Context, versionNo string) (Authorization, error) {
	if validateBusinessNo(versionNo, "ver") != nil {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionProductQuery)
}
func (a *managementAuthorizer) AuthorizeBindingWrite(ctx context.Context, versionNo string) (Authorization, error) {
	if validateBusinessNo(versionNo, "ver") != nil {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return a.authorizeAll(ctx, PermissionBindingWrite, PermissionInterfaceView)
}
func (a *managementAuthorizer) AuthorizeVersionConfiguration(ctx context.Context, versionNo string) (Authorization, error) {
	if validateBusinessNo(versionNo, "ver") != nil {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	return a.authorizeAll(ctx, PermissionConfigurationWrite, permissionProductQuery)
}
func (a *managementAuthorizer) AuthorizePublicationAdmission(ctx context.Context, versionNo string) (Authorization, error) {
	actor, err := a.publication.AuthorizeProductPublication(ctx, versionNo)
	if err != nil {
		return Authorization{}, err
	}
	return Authorization{ActorUserID: actor, RequiredPermission: catalog.PermissionCatalogProductPublish}, nil
}

func (a *managementAuthorizer) authorize(ctx context.Context, permission string) (Authorization, error) {
	return a.authorizeAll(ctx, permission)
}

func (a *managementAuthorizer) authorizeAll(ctx context.Context, primary string, required ...string) (Authorization, error) {
	if a == nil {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	current := a.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant || current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
	}
	permissions := append([]string{primary}, required...)
	keys := make([]authz.PermissionKey, 0, len(permissions)+1)
	for _, permission := range permissions {
		keys = append(keys, authz.PermissionKey(permission))
	}
	keys = append(keys, authz.PermissionKey(idccap.HostPermissionWildcard))
	grants, err := a.authz.BatchHasPermissions(ctx, keys)
	if err != nil {
		return Authorization{}, bizerr.WrapCode(err, catalog.CodeManagementNotAccessible)
	}
	admin, err := a.authz.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil {
		return Authorization{}, bizerr.WrapCode(err, catalog.CodeManagementNotAccessible)
	}
	if !admin && !grants[authz.PermissionKey(idccap.HostPermissionWildcard)] {
		for _, permission := range permissions {
			if !grants[authz.PermissionKey(permission)] {
				return Authorization{}, bizerr.NewCode(catalog.CodeManagementNotAccessible)
			}
		}
	}
	return Authorization{ActorUserID: current.UserID, RequiredPermission: primary}, nil
}
