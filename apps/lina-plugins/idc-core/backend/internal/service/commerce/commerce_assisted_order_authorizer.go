// This file enforces direct-staff identity, representable data scope, and
// exact permissions for direct assistance and historical reads.

package commerce

import (
	"context"

	"github.com/gogf/gf/v2/errors/gerror"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-core/pkg/plugin/capability/bizctxcap"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	permissionAssistedQuoteCreate   = "idc:admin:commerce:assisted-quote:create"
	permissionAssistedQuoteList     = "idc:admin:commerce:assisted-quote:list"
	permissionAssistedQuoteDetail   = "idc:admin:commerce:assisted-quote:query"
	permissionAssistedCustomerQuery = "idc:admin:customer:query"
)

type assistedQuoteAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
}

var _ AssistedQuoteAuthorizer = (*assistedQuoteAuthorizerImpl)(nil)

// NewAssistedQuoteAuthorizer creates the fail-closed staff authorization seam.
func NewAssistedQuoteAuthorizer(
	bizCtxSvc bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
) (AssistedQuoteAuthorizer, error) {
	if isNilOrderDirectoryDependency(bizCtxSvc) || isNilOrderDirectoryDependency(principalReader) ||
		isNilOrderDirectoryDependency(authorization) {
		return nil, gerror.New("assisted quote authorizer requires bizctx, principal, and authz services")
	}
	return &assistedQuoteAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, principal: principalReader, authorization: authorization,
	}, nil
}

func (a *assistedQuoteAuthorizerImpl) AuthorizeAssistedQuoteCreate(
	ctx context.Context,
	customerNo string,
) (int, error) {
	if !validAssistedIdentifier(customerNo) {
		return 0, bizerr.NewCode(CodeAssistedQuoteNotAccessible)
	}
	return a.authorize(ctx, permissionAssistedQuoteCreate)
}

func (a *assistedQuoteAuthorizerImpl) AuthorizeAssistedQuoteList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionAssistedQuoteList)
}

func (a *assistedQuoteAuthorizerImpl) AuthorizeAssistedQuoteDetail(
	ctx context.Context,
	requestNo string,
) (int, error) {
	if !validAssistedIdentifier(requestNo) {
		return 0, bizerr.NewCode(CodeAssistedQuoteNotAccessible)
	}
	return a.authorize(ctx, permissionAssistedQuoteDetail)
}

func (a *assistedQuoteAuthorizerImpl) authorize(
	ctx context.Context,
	permission string,
) (int, error) {
	if a == nil {
		return 0, bizerr.NewCode(CodeAssistedQuoteNotAccessible)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodeAssistedQuoteNotAccessible)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive ||
		audience.CustomerNo != "" {
		return 0, bizerr.NewCode(CodeAssistedQuoteNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission),
		authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeAssistedQuoteNotAccessible)
	}
	bypass := grants[authz.PermissionKey(idccap.HostPermissionWildcard)]
	if !bypass && !grants[authz.PermissionKey(permission)] {
		return 0, bizerr.NewCode(CodeAssistedQuoteNotAccessible)
	}
	return current.UserID, nil
}

func validAssistedIdentifier(value string) bool {
	return value != "" && len(value) <= 40 && orderLifecycleIdentifierPattern.MatchString(value)
}
