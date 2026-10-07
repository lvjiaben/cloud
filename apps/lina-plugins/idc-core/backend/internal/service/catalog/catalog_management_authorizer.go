// This file authorizes direct staff catalog-management operations.

package catalog

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

type managementAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
}

type productPublicationAuthorizerImpl struct{ management *managementAuthorizerImpl }

var _ PublicationAuthorizer = (*productPublicationAuthorizerImpl)(nil)

var _ ManagementAuthorizer = (*managementAuthorizerImpl)(nil)

var managementPermissions = map[string]struct{}{
	permissionCatalogProductList:     {},
	permissionCatalogProductQuery:    {},
	permissionCatalogCategoryList:    {},
	permissionCatalogCategoryCreate:  {},
	permissionCatalogCategoryUpdate:  {},
	permissionCatalogCategoryDelete:  {},
	permissionCatalogCategoryRestore: {},
	permissionCatalogProductCreate:   {},
	permissionCatalogBillingWrite:    {},
	permissionCatalogPriceWrite:      {},
	permissionCatalogTaxWrite:        {},
	permissionCatalogCostWrite:       {},
}

// NewManagementAuthorizer creates the production permission and data-scope seam.
func NewManagementAuthorizer(
	bizCtxSvc bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
) (ManagementAuthorizer, error) {
	if nilManagementDependency(bizCtxSvc) || nilManagementDependency(principalReader) ||
		nilManagementDependency(authorization) {
		return nil, gerror.New("catalog management authorizer requires bizctx, principal, and authz services")
	}
	return &managementAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, principal: principalReader, authorization: authorization,
	}, nil
}

// NewProductPublicationAuthorizer creates the distinct publication permission seam.
func NewProductPublicationAuthorizer(
	bizCtxSvc bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
) (PublicationAuthorizer, error) {
	if nilManagementDependency(bizCtxSvc) || nilManagementDependency(principalReader) ||
		nilManagementDependency(authorization) {
		return nil, gerror.New("catalog product publication authorizer requires bizctx, principal, and authz services")
	}
	return &productPublicationAuthorizerImpl{management: &managementAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, principal: principalReader, authorization: authorization,
	}}, nil
}

func (a *productPublicationAuthorizerImpl) AuthorizeProductPublication(
	ctx context.Context,
	versionNo string,
) (int, error) {
	if a == nil || a.management == nil || !existingCatalogReferencePattern.MatchString(versionNo) {
		return 0, bizerr.NewCode(CodePublicationNotAccessible)
	}
	return a.management.authorizePermission(ctx, PermissionCatalogProductPublish, CodePublicationNotAccessible)
}

func (a *managementAuthorizerImpl) AuthorizeCategoryCreate(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionCatalogCategoryCreate)
}

func (a *managementAuthorizerImpl) AuthorizeCategoryList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionCatalogCategoryList)
}

func (a *managementAuthorizerImpl) AuthorizeCategoryUpdate(ctx context.Context, categoryNo string) (int, error) {
	if !categoryNumberPattern.MatchString(categoryNo) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionCatalogCategoryUpdate)
}

func (a *managementAuthorizerImpl) AuthorizeCategoryDelete(ctx context.Context, categoryNo string) (int, error) {
	if !categoryNumberPattern.MatchString(categoryNo) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionCatalogCategoryDelete)
}

func (a *managementAuthorizerImpl) AuthorizeCategoryRestore(ctx context.Context, categoryNo string) (int, error) {
	if !categoryNumberPattern.MatchString(categoryNo) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionCatalogCategoryRestore)
}

func (a *managementAuthorizerImpl) AuthorizeProductList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionCatalogProductList)
}

func (a *managementAuthorizerImpl) AuthorizeProductQuery(
	ctx context.Context,
	productNo string,
) (int, error) {
	if !existingCatalogReferencePattern.MatchString(productNo) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionCatalogProductQuery)
}

func (a *managementAuthorizerImpl) AuthorizeProductCreate(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionCatalogProductCreate)
}

func (a *managementAuthorizerImpl) AuthorizeBillingWrite(
	ctx context.Context,
	versionNo string,
) (int, error) {
	if !existingCatalogReferencePattern.MatchString(versionNo) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionCatalogBillingWrite)
}

func (a *managementAuthorizerImpl) AuthorizePriceWrite(
	ctx context.Context,
	versionNo string,
) (int, error) {
	if !existingCatalogReferencePattern.MatchString(versionNo) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionCatalogPriceWrite)
}

func (a *managementAuthorizerImpl) AuthorizeTaxWrite(
	ctx context.Context,
	versionNo string,
) (int, error) {
	if !existingCatalogReferencePattern.MatchString(versionNo) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionCatalogTaxWrite)
}

func (a *managementAuthorizerImpl) AuthorizeCostWrite(
	ctx context.Context,
	versionNo string,
) (int, error) {
	if !existingCatalogReferencePattern.MatchString(versionNo) {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorize(ctx, permissionCatalogCostWrite)
}

func (a *managementAuthorizerImpl) authorize(ctx context.Context, permission string) (int, error) {
	if _, allowed := managementPermissions[permission]; a == nil || !allowed {
		return 0, bizerr.NewCode(CodeManagementNotAccessible)
	}
	return a.authorizePermission(ctx, permission, CodeManagementNotAccessible)
}

func (a *managementAuthorizerImpl) authorizePermission(
	ctx context.Context,
	permission string,
	errorCode *bizerr.Code,
) (int, error) {
	if a == nil {
		return 0, bizerr.NewCode(errorCode)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(errorCode)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil {
		return 0, bizerr.WrapCode(err, errorCode)
	}
	if audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return 0, bizerr.NewCode(errorCode)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, errorCode)
	}
	platformAdmin, err := a.authorization.IsPlatformAdmin(
		ctx, authz.UserID(strconv.Itoa(current.UserID)),
	)
	if err != nil {
		return 0, bizerr.WrapCode(err, errorCode)
	}
	if !grants[authz.PermissionKey(permission)] &&
		!grants[authz.PermissionKey(idccap.HostPermissionWildcard)] && !platformAdmin {
		return 0, bizerr.NewCode(errorCode)
	}
	return current.UserID, nil
}
