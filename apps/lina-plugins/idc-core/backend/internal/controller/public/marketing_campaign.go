package public

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/public/v1"
	"lina-plugin-idc-core/backend/internal/service/marketing"
)

// MarketingCampaignList returns the active campaign projection for one product.
func (c *ControllerV1) MarketingCampaignList(ctx context.Context, req *v1.MarketingCampaignListReq) (*v1.MarketingCampaignListRes, error) {
	rows, err := c.campaigns.List(ctx, marketing.PublicCampaignListInput{PublicSlug: req.PublicSlug, CurrencyCode: req.CurrencyCode})
	if err != nil {
		return nil, err
	}
	result := &v1.MarketingCampaignListRes{Campaigns: make([]*v1.PublicMarketingCampaignItem, 0, len(rows))}
	for _, row := range rows {
		result.Campaigns = append(result.Campaigns, &v1.PublicMarketingCampaignItem{
			CampaignNo: row.CampaignNo, CampaignType: row.CampaignType,
			NameZhCN: row.NameZhCN, NameEnUS: row.NameEnUS,
			DescriptionZhCN: row.DescriptionZhCN, DescriptionEnUS: row.DescriptionEnUS,
			StartsAt: row.StartsAt.UnixMilli(), EndsAt: row.EndsAt.UnixMilli(),
		})
	}
	return result, nil
}

// MarketingCampaignExposureCreate records a privacy-safe browser exposure fact.
func (c *ControllerV1) MarketingCampaignExposureCreate(ctx context.Context, req *v1.MarketingCampaignExposureCreateReq) (*v1.MarketingCampaignExposureCreateRes, error) {
	recorded, err := c.campaigns.RecordExposure(ctx, marketing.PublicCampaignExposureInput{
		PublicSlug: req.PublicSlug, CampaignNo: req.CampaignNo, CurrencyCode: req.CurrencyCode, DeviceID: req.DeviceID,
	})
	if err != nil {
		return nil, err
	}
	return &v1.MarketingCampaignExposureCreateRes{Recorded: recorded.Recorded}, nil
}
