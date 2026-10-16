// This file validates direct staff identity, precise permissions and supported
// single-merchant scope before any management data is read or written.
package salesmanagement

import (
	"context"
	"strings"

	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

const (
	permissionOverview      = "idc:admin:sales-channel:overview:view"
	permissionQualification = "idc:admin:sales-channel:qualification:manage"
	permissionAssignment    = "idc:admin:sales-channel:assignment:manage"
	permissionPricing       = "idc:admin:sales-channel:pricing:manage"
	permissionCommission    = "idc:admin:sales-channel:commission:manage"
	permissionCustomer      = "idc:admin:customer:query"
	statusActive            = "active"
	statusInactive          = "inactive"
	statusDeleted           = "deleted"
	priceTargetCustomer     = "customer"
	priceTargetLevel        = "level"
	statusSuspended         = "suspended"
	statusEnded             = "ended"
)

func (s *serviceImpl) authorize(ctx context.Context, permission string, customer bool) (int, error) {
	if ctx == nil || ctx.Err() != nil {
		return 0, bizerr.NewCode(codeDenied)
	}
	current := s.bizCtx.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID || current.ActingUserID != 0 || current.ActingAsTenant || current.IsImpersonation || current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(codeDenied)
	}
	audience, err := s.principal.Current(ctx)
	if err != nil || audience == nil || audience.UserID != current.UserID || audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive || audience.CustomerNo != "" {
		return 0, bizerr.NewCode(codeDenied)
	}
	keys := []authz.PermissionKey{authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard)}
	if customer {
		keys = append(keys, authz.PermissionKey(permissionCustomer))
	}
	grants, err := s.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, codeDenied)
	}
	if !grants[authz.PermissionKey(idccap.HostPermissionWildcard)] && (!grants[authz.PermissionKey(permission)] || (customer && !grants[authz.PermissionKey(permissionCustomer)])) {
		return 0, bizerr.NewCode(codeDenied)
	}
	return current.UserID, nil
}
func normalizePage(input PageInput) (PageInput, error) {
	input.Search = strings.TrimSpace(input.Search)
	if input.Page == 0 {
		input.Page = 1
	}
	if input.PageSize == 0 {
		input.PageSize = 20
	}
	if input.Page < 1 || input.Page > 100000 || input.PageSize < 1 || input.PageSize > 100 || len(input.Search) > 100 {
		return PageInput{}, bizerr.NewCode(codeInvalid)
	}
	return input, nil
}
