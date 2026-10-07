package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingValueGrantList(ctx context.Context, req *v1.MarketingValueGrantListReq) (*v1.MarketingValueGrantListRes, error) {
	page, err := c.marketingAssetDirectory.SearchValueGrants(ctx, marketing.ValueGrantSearchInput{
		Page: req.Page,
		Type: req.Type, Status: req.Status, Currency: req.Currency, CustomerNo: req.CustomerNo,
		CampaignNo: req.CampaignNo, Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.MarketingValueGrantItem, 0, len(page.Grants))
	for _, item := range page.Grants {
		items = append(items, toAPIMarketingValueGrant(item))
	}
	return &v1.MarketingValueGrantListRes{Grants: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
