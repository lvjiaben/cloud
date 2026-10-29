// This file verifies the production catalog-management authorizer.

package catalog

import (
	"context"
	"testing"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-core/pkg/plugin/capability/capmodel"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

type managementTestBizCtx struct{ current bizctxcap.CurrentContext }

func (b managementTestBizCtx) Current(context.Context) bizctxcap.CurrentContext { return b.current }

type managementTestPrincipal struct {
	audience *principal.Audience
	calls    int
}

func (p *managementTestPrincipal) Current(context.Context) (*principal.Audience, error) {
	p.calls++
	return p.audience, nil
}

type managementTestAuthz struct {
	grants        map[authz.PermissionKey]bool
	platformAdmin bool
	batchCalls    int
}

func (*managementTestAuthz) BatchGetPermissions(
	context.Context,
	[]authz.PermissionKey,
) (*capmodel.BatchResult[*authz.PermissionInfo, authz.PermissionKey], error) {
	return nil, nil
}

func (a *managementTestAuthz) BatchHasPermissions(
	context.Context,
	[]authz.PermissionKey,
) (map[authz.PermissionKey]bool, error) {
	a.batchCalls++
	return a.grants, nil
}

func (*managementTestAuthz) HasPermission(context.Context, authz.PermissionKey) (bool, error) {
	return false, nil
}

func (a *managementTestAuthz) IsPlatformAdmin(context.Context, authz.UserID) (bool, error) {
	return a.platformAdmin, nil
}

func (*managementTestAuthz) ReplaceRolePermissions(
	context.Context,
	authz.RoleID,
	[]authz.PermissionKey,
) error {
	return nil
}

func TestManagementAuthorizerUsesDistinctDirectStaffPermissions(t *testing.T) {
	t.Parallel()
	current := bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}
	newPrincipal := func() *managementTestPrincipal {
		return &managementTestPrincipal{audience: &principal.Audience{
			UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
		}}
	}
	categoryAuthz := &managementTestAuthz{grants: map[authz.PermissionKey]bool{
		authz.PermissionKey(permissionCatalogCategoryCreate): true,
	}}
	authorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current}, newPrincipal(), categoryAuthz,
	)
	if err != nil {
		t.Fatalf("create management authorizer: %v", err)
	}
	actor, err := authorizer.AuthorizeCategoryCreate(context.Background())
	if err != nil || actor != 7 {
		t.Fatalf("authorize category create actor=%d err=%v", actor, err)
	}
	if _, err = authorizer.AuthorizeProductCreate(context.Background()); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("category permission authorized product create: %v", err)
	}
	if _, err = authorizer.AuthorizeProductList(context.Background()); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("category permission authorized product list: %v", err)
	}
	if _, err = authorizer.AuthorizeCategoryList(context.Background()); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("category-create permission authorized category list: %v", err)
	}
	categoryListAuthorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current}, newPrincipal(),
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogCategoryList): true,
		}},
	)
	if err != nil {
		t.Fatalf("create category-list authorizer: %v", err)
	}
	if actor, err = categoryListAuthorizer.AuthorizeCategoryList(context.Background()); err != nil || actor != 7 {
		t.Fatalf("authorize category list actor=%d err=%v", actor, err)
	}
	if _, err = categoryListAuthorizer.AuthorizeCategoryCreate(context.Background()); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("category-list permission authorized category create: %v", err)
	}
	listAuthorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current}, newPrincipal(),
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogProductList): true,
		}},
	)
	if err != nil {
		t.Fatalf("create list management authorizer: %v", err)
	}
	if actor, err = listAuthorizer.AuthorizeProductList(context.Background()); err != nil || actor != 7 {
		t.Fatalf("authorize product list actor=%d err=%v", actor, err)
	}
	if _, err = listAuthorizer.AuthorizeProductQuery(context.Background(), "product-route"); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("product-list permission authorized product query: %v", err)
	}
	if _, err = listAuthorizer.AuthorizeProductCreate(context.Background()); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("product-list permission authorized product create: %v", err)
	}
	queryAuthorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current}, newPrincipal(),
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogProductQuery): true,
		}},
	)
	if err != nil {
		t.Fatalf("create query management authorizer: %v", err)
	}
	if actor, err = queryAuthorizer.AuthorizeProductQuery(
		context.Background(), "product-route",
	); err != nil || actor != 7 {
		t.Fatalf("authorize product query actor=%d err=%v", actor, err)
	}
	if _, err = queryAuthorizer.AuthorizeProductList(context.Background()); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("product-query permission authorized product list: %v", err)
	}
	productAuthorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current}, newPrincipal(),
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogProductCreate): true,
		}},
	)
	if err != nil {
		t.Fatalf("create product management authorizer: %v", err)
	}
	if actor, err = productAuthorizer.AuthorizeProductCreate(context.Background()); err != nil || actor != 7 {
		t.Fatalf("authorize product create actor=%d err=%v", actor, err)
	}
	if _, err = productAuthorizer.AuthorizeProductUpdate(context.Background(), "product-route"); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("create permission authorized metadata update: %v", err)
	}
	updateAuthorizer, err := NewManagementAuthorizer(managementTestBizCtx{current: current}, newPrincipal(), &managementTestAuthz{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionCatalogProductUpdate): true}})
	if err != nil {
		t.Fatal(err)
	}
	if actor, err = updateAuthorizer.AuthorizeProductUpdate(context.Background(), "product-route"); err != nil || actor != 7 {
		t.Fatalf("metadata update actor=%d err=%v", actor, err)
	}
	if _, err = updateAuthorizer.AuthorizeProductCreate(context.Background()); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatal("metadata update authorized product creation")
	}
	if _, err = updateAuthorizer.AuthorizeProductDelete(context.Background(), "product-route"); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatal("metadata update authorized product deletion")
	}
	deleteAuthorizer, err := NewManagementAuthorizer(managementTestBizCtx{current: current}, newPrincipal(), &managementTestAuthz{grants: map[authz.PermissionKey]bool{authz.PermissionKey(permissionCatalogProductDelete): true}})
	if err != nil {
		t.Fatal(err)
	}
	if actor, err = deleteAuthorizer.AuthorizeProductDelete(context.Background(), "product-route"); err != nil || actor != 7 {
		t.Fatalf("product delete actor=%d err=%v", actor, err)
	}
	if _, err = deleteAuthorizer.AuthorizeProductUpdate(context.Background(), "product-route"); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatal("delete permission authorized metadata update")
	}
	billingAuthorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current}, newPrincipal(),
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogBillingWrite): true,
		}},
	)
	if err != nil {
		t.Fatalf("create billing management authorizer: %v", err)
	}
	if actor, err = billingAuthorizer.AuthorizeBillingWrite(
		context.Background(), "version-draft-1",
	); err != nil || actor != 7 {
		t.Fatalf("authorize billing write actor=%d err=%v", actor, err)
	}
	if _, err = billingAuthorizer.AuthorizePriceWrite(
		context.Background(), "version-draft-1",
	); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("billing-write permission authorized price write: %v", err)
	}
	priceAuthorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current}, newPrincipal(),
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogPriceWrite): true,
		}},
	)
	if err != nil {
		t.Fatalf("create price management authorizer: %v", err)
	}
	if actor, err = priceAuthorizer.AuthorizePriceWrite(
		context.Background(), "version-draft-1",
	); err != nil || actor != 7 {
		t.Fatalf("authorize price write actor=%d err=%v", actor, err)
	}
	if _, err = priceAuthorizer.AuthorizeBillingWrite(
		context.Background(), "version-draft-1",
	); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("price-write permission authorized billing write: %v", err)
	}
	taxAuthorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: current}, newPrincipal(),
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogTaxWrite): true,
		}},
	)
	if err != nil {
		t.Fatalf("create tax management authorizer: %v", err)
	}
	if actor, err = taxAuthorizer.AuthorizeTaxWrite(
		context.Background(), "version-draft-1",
	); err != nil || actor != 7 {
		t.Fatalf("authorize tax write actor=%d err=%v", actor, err)
	}
	if _, err = taxAuthorizer.AuthorizePriceWrite(
		context.Background(), "version-draft-1",
	); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("tax-write permission authorized price write: %v", err)
	}
}

func TestManagementQueryAuthorizerRejectsMalformedProductBeforeDependencyReads(t *testing.T) {
	t.Parallel()
	principalReader := &managementTestPrincipal{}
	authorization := &managementTestAuthz{}
	authorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}},
		principalReader,
		authorization,
	)
	if err != nil {
		t.Fatalf("create query management authorizer: %v", err)
	}
	if _, err = authorizer.AuthorizeProductQuery(context.Background(), "../../unsafe"); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("malformed product query returned %v", err)
	}
	if principalReader.calls != 0 || authorization.batchCalls != 0 {
		t.Fatalf("malformed product query reached dependencies: principal=%d authz=%d",
			principalReader.calls, authorization.batchCalls)
	}
}

func TestManagementPricingAuthorizerRejectsMalformedVersionBeforeDependencyReads(t *testing.T) {
	t.Parallel()
	principalReader := &managementTestPrincipal{}
	authorization := &managementTestAuthz{}
	authorizer, err := NewManagementAuthorizer(
		managementTestBizCtx{current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}},
		principalReader,
		authorization,
	)
	if err != nil {
		t.Fatalf("create configuration management authorizer: %v", err)
	}
	if _, err = authorizer.AuthorizeBillingWrite(
		context.Background(), "../../unsafe",
	); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("malformed billing version returned %v", err)
	}
	if _, err = authorizer.AuthorizePriceWrite(
		context.Background(), "../../unsafe",
	); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("malformed price version returned %v", err)
	}
	if _, err = authorizer.AuthorizeTaxWrite(
		context.Background(), "../../unsafe",
	); !bizerr.Is(err, CodeManagementNotAccessible) {
		t.Fatalf("malformed tax version returned %v", err)
	}
	if principalReader.calls != 0 || authorization.batchCalls != 0 {
		t.Fatalf("malformed billing version reached dependencies: principal=%d authz=%d",
			principalReader.calls, authorization.batchCalls)
	}
}

func TestManagementAuthorizerRejectsUnsafeIdentityBeforePermissionRead(t *testing.T) {
	t.Parallel()
	for name, current := range map[string]bizctxcap.CurrentContext{
		"delegated":         {UserID: 7, TenantID: 0, DataScope: 2, ActingUserID: 8},
		"unsupported scope": {UserID: 7, TenantID: 0, DataScope: 3},
		"wrong tenant":      {UserID: 7, TenantID: 9, DataScope: 2},
	} {
		t.Run(name, func(t *testing.T) {
			principalReader := &managementTestPrincipal{audience: &principal.Audience{
				UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
			}}
			authorization := &managementTestAuthz{grants: map[authz.PermissionKey]bool{
				authz.PermissionKey(permissionCatalogCategoryCreate): true,
			}}
			authorizer, err := NewManagementAuthorizer(
				managementTestBizCtx{current: current}, principalReader, authorization,
			)
			if err != nil {
				t.Fatalf("create management authorizer: %v", err)
			}
			if _, err = authorizer.AuthorizeCategoryCreate(context.Background()); !bizerr.Is(err, CodeManagementNotAccessible) {
				t.Fatalf("unsafe identity returned %v", err)
			}
			if _, err = authorizer.AuthorizeProductDelete(context.Background(), "product-route"); !bizerr.Is(err, CodeManagementNotAccessible) {
				t.Fatalf("unsafe identity authorized deletion: %v", err)
			}
			if principalReader.calls != 0 || authorization.batchCalls != 0 {
				t.Fatalf("unsafe identity reached dependencies: principal=%d authz=%d",
					principalReader.calls, authorization.batchCalls)
			}
		})
	}
}

func TestNewManagementAuthorizerRejectsMissingDependencies(t *testing.T) {
	t.Parallel()
	bizCtx := managementTestBizCtx{}
	principalReader := &managementTestPrincipal{}
	authorization := &managementTestAuthz{}
	if _, err := NewManagementAuthorizer(nil, principalReader, authorization); err == nil {
		t.Fatal("nil management bizctx accepted")
	}
	if _, err := NewManagementAuthorizer(bizCtx, nil, authorization); err == nil {
		t.Fatal("nil management principal accepted")
	}
	if _, err := NewManagementAuthorizer(bizCtx, principalReader, nil); err == nil {
		t.Fatal("nil management authz accepted")
	}
}

func TestProductPublicationAuthorizerKeepsDistinctPermission(t *testing.T) {
	t.Parallel()
	current := managementTestBizCtx{current: bizctxcap.CurrentContext{
		UserID: 7, TenantID: 0, DataScope: 2,
	}}
	principalReader := &managementTestPrincipal{audience: &principal.Audience{
		UserID: 7, Code: principal.AudienceStaff, Status: principal.AudienceStatusActive,
	}}
	publication, err := NewProductPublicationAuthorizer(
		current,
		principalReader,
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(PermissionCatalogProductPublish): true,
		}},
	)
	if err != nil {
		t.Fatalf("create product publication authorizer: %v", err)
	}
	actor, err := publication.AuthorizeProductPublication(context.Background(), "version-draft-1")
	if err != nil || actor != 7 {
		t.Fatalf("authorize product publication actor=%d err=%v", actor, err)
	}

	managementOnly, err := NewProductPublicationAuthorizer(
		current,
		&managementTestPrincipal{audience: principalReader.audience},
		&managementTestAuthz{grants: map[authz.PermissionKey]bool{
			authz.PermissionKey(permissionCatalogProductCreate): true,
		}},
	)
	if err != nil {
		t.Fatalf("create management-only publication authorizer: %v", err)
	}
	if _, err = managementOnly.AuthorizeProductPublication(
		context.Background(), "version-draft-1",
	); !bizerr.Is(err, CodePublicationNotAccessible) {
		t.Fatalf("product-create permission authorized publication: %v", err)
	}
}

func TestProductPublicationAuthorizerRejectsMalformedVersionBeforeDependencyReads(t *testing.T) {
	t.Parallel()
	principalReader := &managementTestPrincipal{}
	authorization := &managementTestAuthz{}
	publication, err := NewProductPublicationAuthorizer(
		managementTestBizCtx{current: bizctxcap.CurrentContext{UserID: 7, TenantID: 0, DataScope: 2}},
		principalReader,
		authorization,
	)
	if err != nil {
		t.Fatalf("create product publication authorizer: %v", err)
	}
	if _, err = publication.AuthorizeProductPublication(
		context.Background(), "../../unsafe",
	); !bizerr.Is(err, CodePublicationNotAccessible) {
		t.Fatalf("malformed publication version returned %v", err)
	}
	if principalReader.calls != 0 || authorization.batchCalls != 0 {
		t.Fatalf("malformed publication reached dependencies: principal=%d authz=%d",
			principalReader.calls, authorization.batchCalls)
	}
}
