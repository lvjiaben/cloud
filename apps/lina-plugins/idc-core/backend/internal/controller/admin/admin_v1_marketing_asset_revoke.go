package admin

import (
	"context"
	"time"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingAssetRevoke(ctx context.Context, req *v1.MarketingAssetRevokeReq) (*v1.MarketingAssetRevokeRes, error) {
	result, err := c.marketingActions.RevokeAsset(ctx, marketing.AssetRevokeInput{
		RequestNo: req.RequestNo, AssetNo: req.AssetNo,
		ExpectedUpdatedAt: time.UnixMilli(req.ExpectedUpdatedAt).UTC(), ReasonCode: req.ReasonCode,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingAssetRevokeRes{Mutation: toAPIManagedAssetMutation(result)}, nil
}
