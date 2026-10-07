package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingAssetList(ctx context.Context, req *v1.MarketingAssetListReq) (*v1.MarketingAssetListRes, error) {
	page, err := c.marketingAssetDirectory.SearchAssets(ctx, marketing.PromotionAssetSearchInput{
		Page: req.Page,
		Type: req.Type, Status: req.Status, Currency: req.Currency, CustomerNo: req.CustomerNo,
		CampaignNo: req.CampaignNo, Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.MarketingPromotionAssetItem, 0, len(page.Assets))
	for _, item := range page.Assets {
		items = append(items, toAPIMarketingPromotionAsset(item))
	}
	return &v1.MarketingAssetListRes{Assets: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
