// This file rejects customer, impersonated and partial-scope currency management before reads.
package currency

import (
	"context"
	"lina-core/pkg/bizerr"
	"lina-core/pkg/plugin/capability/authcap/authz"
	"lina-plugin-idc-core/backend/cap/idccap"
	"lina-plugin-idc-core/backend/internal/service/principal"
)

func (s *serviceImpl) authorize(ctx context.Context, permission string) (int, error) {
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
	grants, err := s.authorization.BatchHasPermissions(ctx, []authz.PermissionKey{authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard)})
	if err != nil {
		return 0, bizerr.WrapCode(err, codeDenied)
	}
	if !grants[authz.PermissionKey(permission)] && !grants[authz.PermissionKey(idccap.HostPermissionWildcard)] {
		return 0, bizerr.NewCode(codeDenied)
	}
	return current.UserID, nil
}
