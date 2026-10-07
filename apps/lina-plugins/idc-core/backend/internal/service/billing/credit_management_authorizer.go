// This file enforces exact staff permissions and data scopes for credit management.

package billing

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
	permissionCreditApplicationList   = "idc:admin:billing:credit-application:list"
	permissionCreditApplicationQuery  = "idc:admin:billing:credit-application:query"
	permissionCreditApplicationReview = "idc:admin:billing:credit-application:review"
	permissionCreditAccountList       = "idc:admin:billing:credit-account:list"
	permissionCreditAccountQuery      = "idc:admin:billing:credit-account:query"
	permissionCreditStatementList     = "idc:admin:billing:credit-statement:list"
	permissionCreditStatementQuery    = "idc:admin:billing:credit-statement:query"
	permissionCreditPolicyWrite       = "idc:admin:billing:credit-policy:write"
)

// ManagementCreditAuthorizer keeps each staff credit read or command behind
// its own exact permission and a directly authenticated, representable scope.
type ManagementCreditAuthorizer interface {
	// AuthorizeApplicationList requires direct staff application-list authority and returns the staff identifier.
	AuthorizeApplicationList(context.Context) (int, error)
	// AuthorizeApplicationDetail validates the reference and exact detail authority before any target query.
	AuthorizeApplicationDetail(context.Context, string) (int, error)
	// AuthorizeApplicationReview validates the reference and exact review authority before any target query.
	AuthorizeApplicationReview(context.Context, string) (int, error)
	// AuthorizeAccountList requires direct staff account-list authority and returns the staff identifier.
	AuthorizeAccountList(context.Context) (int, error)
	// AuthorizeAccountDetail validates the reference and exact detail authority before any target query.
	AuthorizeAccountDetail(context.Context, string) (int, error)
	// AuthorizeStatementList requires direct staff statement-list authority and returns the staff identifier.
	AuthorizeStatementList(context.Context) (int, error)
	// AuthorizeStatementDetail validates the reference and exact detail authority before any target query.
	AuthorizeStatementDetail(context.Context, string) (int, error)
	// AuthorizePolicyWrite requires direct staff policy-write authority and returns the staff identifier.
	AuthorizePolicyWrite(context.Context) (int, error)
}

type managementCreditAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
}

var _ ManagementCreditAuthorizer = (*managementCreditAuthorizerImpl)(nil)

// NewManagementCreditAuthorizer constructs the direct-staff exact-permission boundary.
func NewManagementCreditAuthorizer(
	bizCtxSvc bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
) (ManagementCreditAuthorizer, error) {
	if isNilFinanceDirectoryDependency(bizCtxSvc) ||
		isNilFinanceDirectoryDependency(principalReader) ||
		isNilFinanceDirectoryDependency(authorization) {
		return nil, gerror.New("management credit authorizer requires bizctx, principal, and authz services")
	}
	return &managementCreditAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, principal: principalReader, authorization: authorization,
	}, nil
}

func (a *managementCreditAuthorizerImpl) AuthorizeApplicationList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionCreditApplicationList)
}

func (a *managementCreditAuthorizerImpl) AuthorizeApplicationDetail(ctx context.Context, reference string) (int, error) {
	return a.authorizeReference(ctx, permissionCreditApplicationQuery, reference)
}

func (a *managementCreditAuthorizerImpl) AuthorizeApplicationReview(ctx context.Context, reference string) (int, error) {
	return a.authorizeReference(ctx, permissionCreditApplicationReview, reference)
}

func (a *managementCreditAuthorizerImpl) AuthorizeAccountList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionCreditAccountList)
}

func (a *managementCreditAuthorizerImpl) AuthorizeAccountDetail(ctx context.Context, reference string) (int, error) {
	return a.authorizeReference(ctx, permissionCreditAccountQuery, reference)
}

func (a *managementCreditAuthorizerImpl) AuthorizeStatementList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionCreditStatementList)
}

func (a *managementCreditAuthorizerImpl) AuthorizeStatementDetail(ctx context.Context, reference string) (int, error) {
	return a.authorizeReference(ctx, permissionCreditStatementQuery, reference)
}

func (a *managementCreditAuthorizerImpl) AuthorizePolicyWrite(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionCreditPolicyWrite)
}

func (a *managementCreditAuthorizerImpl) authorizeReference(
	ctx context.Context,
	permission string,
	reference string,
) (int, error) {
	if !validBusinessReference(reference, 40) {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return a.authorize(ctx, permission)
}

func (a *managementCreditAuthorizerImpl) authorize(ctx context.Context, permission string) (int, error) {
	if a == nil || !idccap.IsCatalogedAdminPermission(permission) {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive ||
		audience.CustomerNo != "" {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeDirectoryNotAccessible)
	}
	platformAdmin, err := a.authorization.IsPlatformAdmin(
		ctx, authz.UserID(strconv.Itoa(current.UserID)),
	)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeDirectoryNotAccessible)
	}
	if !grants[authz.PermissionKey(permission)] &&
		!grants[authz.PermissionKey(idccap.HostPermissionWildcard)] && !platformAdmin {
		return 0, bizerr.NewCode(CodeDirectoryNotAccessible)
	}
	return current.UserID, nil
}
