// This file derives exact risk-review authority from host-published context.
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

type orderRiskReviewAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
}

var _ OrderRiskReviewAuthorizer = (*orderRiskReviewAuthorizerImpl)(nil)

// NewOrderRiskReviewAuthorizer creates the direct-staff risk review seam.
func NewOrderRiskReviewAuthorizer(
	bizCtxSvc bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
) (OrderRiskReviewAuthorizer, error) {
	if isNilOrderRiskDependency(bizCtxSvc) || isNilOrderRiskDependency(principalReader) ||
		isNilOrderRiskDependency(authorization) {
		return nil, gerror.New("order risk review authorizer requires bizctx, principal, and authz services")
	}
	return &orderRiskReviewAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, principal: principalReader, authorization: authorization,
	}, nil
}

func (a *orderRiskReviewAuthorizerImpl) AuthorizeRiskReviewApprove(
	ctx context.Context,
	reviewNo string,
) (int, error) {
	return a.authorize(ctx, reviewNo, permissionOrderRiskReviewApprove)
}

func (a *orderRiskReviewAuthorizerImpl) AuthorizeRiskReviewReject(
	ctx context.Context,
	reviewNo string,
) (int, error) {
	return a.authorize(ctx, reviewNo, permissionOrderRiskReviewReject)
}

func (a *orderRiskReviewAuthorizerImpl) authorize(
	ctx context.Context,
	reviewNo string,
	permission string,
) (int, error) {
	if a == nil || !orderLifecycleIdentifierPattern.MatchString(reviewNo) || len(reviewNo) > 40 {
		return 0, bizerr.NewCode(CodeOrderRiskReviewNotAccessible)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(CodeOrderRiskReviewNotAccessible)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeOrderRiskReviewNotAccessible)
	}
	if audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return 0, bizerr.NewCode(CodeOrderRiskReviewNotAccessible)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeOrderRiskReviewNotAccessible)
	}
	platformAdmin, err := a.authorization.IsPlatformAdmin(
		ctx, authz.UserID(strconv.Itoa(current.UserID)),
	)
	if err != nil {
		return 0, bizerr.WrapCode(err, CodeOrderRiskReviewNotAccessible)
	}
	if !grants[authz.PermissionKey(permission)] &&
		!grants[authz.PermissionKey(idccap.HostPermissionWildcard)] && !platformAdmin {
		return 0, bizerr.NewCode(CodeOrderRiskReviewNotAccessible)
	}
	return current.UserID, nil
}
