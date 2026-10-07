package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingPointsAccountList(ctx context.Context, req *v1.MarketingPointsAccountListReq) (*v1.MarketingPointsAccountListRes, error) {
	page, err := c.marketingAssetDirectory.SearchPointsAccounts(ctx, marketing.PointsAccountSearchInput{
		Page:       req.Page,
		CustomerNo: req.CustomerNo, Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.MarketingPointsAccountItem, 0, len(page.Accounts))
	for _, item := range page.Accounts {
		items = append(items, toAPIMarketingPointsAccount(item))
	}
	return &v1.MarketingPointsAccountListRes{Accounts: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
