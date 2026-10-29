// Structured target quotes use the real employee identity and native server edit scope.
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

const permissionTargetQuoteEdit = "idc:admin:resource:server:edit"

// TargetQuoteAuthorizer authorizes direct staff under the supported native data scopes.
type TargetQuoteAuthorizer interface {
	AuthorizeTargetQuote(context.Context) (int, error)
}
type targetQuoteAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
}

// NewTargetQuoteAuthorizer binds the same exact permission and identity gates as native resize.
func NewTargetQuoteAuthorizer(bizCtx bizctxcap.Service, reader principal.Reader, authorization authz.Service) (TargetQuoteAuthorizer, error) {
	if isNilOrderDirectoryDependency(bizCtx) || isNilOrderDirectoryDependency(reader) || isNilOrderDirectoryDependency(authorization) {
		return nil, gerror.New("target quote requires bizctx, principal and authz")
	}
	return &targetQuoteAuthorizerImpl{bizCtx, reader, authorization}, nil
}

// BindTargetQuoteAuthorizer supplies the staff seam once during startup.
func BindTargetQuoteAuthorizer(owner Service, authorizer TargetQuoteAuthorizer) error {
	service, ok := owner.(*serviceImpl)
	if !ok || service == nil || isNilOrderDirectoryDependency(authorizer) || service.targetQuoteAuthorizer != nil {
		return gerror.New("target quote authorizer cannot be bound")
	}
	service.targetQuoteAuthorizer = authorizer
	return nil
}

func (a *targetQuoteAuthorizerImpl) AuthorizeTargetQuote(ctx context.Context) (int, error) {
	if a == nil || ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive ||
		audience.CustomerNo != "" {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permissionTargetQuoteEdit),
		authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeNotAccessible)
	}
	platformAdmin, err := a.authorization.IsPlatformAdmin(ctx, authz.UserID(strconv.Itoa(current.UserID)))
	if err != nil {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	bypass := platformAdmin || grants[authz.PermissionKey(idccap.HostPermissionWildcard)]
	if !bypass && !grants[authz.PermissionKey(permissionTargetQuoteEdit)] {
		return 0, bizerr.NewCode(CodeNotAccessible)
	}
	return current.UserID, nil
}
