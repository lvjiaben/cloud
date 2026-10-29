// This file authorizes direct staff marketing-management reads.

package marketing

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

type managementOverviewAuthorizerImpl struct {
	bizCtxSvc     bizctxcap.Service
	principal     principal.Reader
	authorization authz.Service
}

// ManagementAuthorizer is the shared production authorization seam; each read
// still invokes its own exact permission method.
type ManagementAuthorizer interface {
	ManagementOverviewAuthorizer
	ManagementDirectoryAuthorizer
	AssetDirectoryAuthorizer
	ManagementActionAuthorizer
	MarketingInsightsAuthorizer
}

var _ ManagementOverviewAuthorizer = (*managementOverviewAuthorizerImpl)(nil)
var _ ManagementDirectoryAuthorizer = (*managementOverviewAuthorizerImpl)(nil)
var _ AssetDirectoryAuthorizer = (*managementOverviewAuthorizerImpl)(nil)
var _ ManagementActionAuthorizer = (*managementOverviewAuthorizerImpl)(nil)
var _ MarketingInsightsAuthorizer = (*managementOverviewAuthorizerImpl)(nil)

// NewManagementOverviewAuthorizer creates the production staff and data-scope seam.
func NewManagementOverviewAuthorizer(
	bizCtxSvc bizctxcap.Service,
	principalReader principal.Reader,
	authorization authz.Service,
) (ManagementAuthorizer, error) {
	if isNilMarketingManagementDependency(bizCtxSvc) ||
		isNilMarketingManagementDependency(principalReader) ||
		isNilMarketingManagementDependency(authorization) {
		return nil, gerror.New("marketing management authorizer requires bizctx, principal, and authz services")
	}
	return &managementOverviewAuthorizerImpl{
		bizCtxSvc: bizCtxSvc, principal: principalReader, authorization: authorization,
	}, nil
}

// AuthorizeMarketingOverview requires a direct active staff actor and the exact overview permission.
func (a *managementOverviewAuthorizerImpl) AuthorizeMarketingOverview(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingOverview, CodeManagementOverviewNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeMarketingPerformance(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingPerformance, CodeInsightsNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeMarketingAbuseList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingAbuseList, CodeInsightsNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeMarketingAbuseReview(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingAbuseReview, CodeInsightsNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeMarketingCostList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingCostList, CodeInsightsNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeMarketingCostCorrect(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingCostCorrect, CodeInsightsNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeMarketingNewcomerSet(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingNewcomerSet, CodeInsightsNotAccessible)
}

// AuthorizeSegmentList requires the exact segment queue permission.
func (a *managementOverviewAuthorizerImpl) AuthorizeSegmentList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingSegmentList, CodeManagementDirectoryNotAccessible)
}

// AuthorizeSegmentQuery requires the exact segment detail permission.
func (a *managementOverviewAuthorizerImpl) AuthorizeSegmentQuery(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingSegmentQuery, CodeManagementDirectoryNotAccessible)
}

// AuthorizeCampaignList requires the exact campaign queue permission.
func (a *managementOverviewAuthorizerImpl) AuthorizeCampaignList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingCampaignList, CodeManagementDirectoryNotAccessible)
}

// AuthorizeCampaignQuery requires the exact campaign detail permission.
func (a *managementOverviewAuthorizerImpl) AuthorizeCampaignQuery(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingCampaignQuery, CodeManagementDirectoryNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeAssetList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingAssetList, CodeManagementDirectoryNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeAssetQuery(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingAssetQuery, CodeManagementDirectoryNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizePointsList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingPointsList, CodeManagementDirectoryNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizePointsQuery(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingPointsQuery, CodeManagementDirectoryNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeGrantList(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingGrantList, CodeManagementDirectoryNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeGrantQuery(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingGrantQuery, CodeManagementDirectoryNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeAssetIssue(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingAssetIssue, CodeManagementActionNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeCouponCodeGenerate(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingCouponGenerate, CodeManagementActionNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizeAssetRevoke(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingAssetRevoke, CodeManagementActionNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizePointsCredit(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingPointsCredit, CodeManagementActionNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) AuthorizePointsReverse(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingPointsReverse, CodeManagementActionNotAccessible)
}

// AuthorizeGrantPost requires the exact marketing value posting permission.
func (a *managementOverviewAuthorizerImpl) AuthorizeGrantPost(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingGrantPost, CodeManagementActionNotAccessible)
}

// AuthorizeGrantReverse requires the exact marketing value reversal permission.
func (a *managementOverviewAuthorizerImpl) AuthorizeGrantReverse(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingGrantReverse, CodeManagementActionNotAccessible)
}

// AuthorizeGrantReview requires the adjustment processing permission.
func (a *managementOverviewAuthorizerImpl) AuthorizeGrantReview(ctx context.Context) (int, error) {
	return a.authorize(ctx, permissionMarketingGrantReview, CodeManagementActionNotAccessible)
}

func (a *managementOverviewAuthorizerImpl) authorize(
	ctx context.Context,
	permission string,
	denied *bizerr.Code,
) (int, error) {
	if a == nil {
		return 0, bizerr.NewCode(denied)
	}
	current := a.bizCtxSvc.Current(ctx)
	if current.UserID <= 0 || current.TenantID != idccap.DefaultTenantID ||
		current.ActingUserID != 0 || current.IsImpersonation || current.ActingAsTenant ||
		current.DataScopeUnsupported || (current.DataScope != 1 && current.DataScope != 2) {
		return 0, bizerr.NewCode(denied)
	}
	audience, err := a.principal.Current(ctx)
	if err != nil {
		return 0, bizerr.WrapCode(err, denied)
	}
	if audience == nil || audience.UserID != current.UserID ||
		audience.Code != principal.AudienceStaff || audience.Status != principal.AudienceStatusActive {
		return 0, bizerr.NewCode(denied)
	}
	keys := []authz.PermissionKey{
		authz.PermissionKey(permission), authz.PermissionKey(idccap.HostPermissionWildcard),
	}
	grants, err := a.authorization.BatchHasPermissions(ctx, keys)
	if err != nil {
		return 0, bizerr.WrapCode(err, denied)
	}
	platformAdmin, err := a.authorization.IsPlatformAdmin(
		ctx, authz.UserID(strconv.Itoa(current.UserID)),
	)
	if err != nil {
		return 0, bizerr.WrapCode(err, denied)
	}
	if !grants[authz.PermissionKey(permission)] &&
		!grants[authz.PermissionKey(idccap.HostPermissionWildcard)] && !platformAdmin {
		return 0, bizerr.NewCode(denied)
	}
	return current.UserID, nil
}
