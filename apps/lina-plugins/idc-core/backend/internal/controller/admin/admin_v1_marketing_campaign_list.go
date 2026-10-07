// This file handles the bounded marketing campaign queue endpoint.

package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

func (c *ControllerV1) MarketingCampaignList(ctx context.Context, req *v1.MarketingCampaignListReq) (*v1.MarketingCampaignListRes, error) {
	page, err := c.marketingDirectory.SearchCampaigns(ctx, marketing.CampaignManagementSearchInput{
		Page:       req.Page,
		NamePrefix: req.NamePrefix, Type: req.Type, Status: req.Status, TargetSegmentNo: req.TargetSegmentNo,
		Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	items := make([]*v1.MarketingCampaignManagementItem, 0, len(page.Campaigns))
	for _, item := range page.Campaigns {
		items = append(items, toAPIMarketingCampaignSummary(item))
	}
	return &v1.MarketingCampaignListRes{Campaigns: items, NextCursor: page.NextCursor, Total: page.Total}, nil
}
