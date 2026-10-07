package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingAssetIssue(ctx context.Context, req *v1.MarketingAssetIssueReq) (*v1.MarketingAssetIssueRes, error) {
	result, err := c.marketingActions.IssueAsset(ctx, marketing.AssetIssueInput{
		RequestNo: req.RequestNo, CampaignNo: req.CampaignNo, BenefitNo: req.BenefitNo,
		CustomerNo: req.CustomerNo, ReasonCode: req.ReasonCode,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingAssetIssueRes{Mutation: toAPIManagedAssetMutation(result)}, nil
}
