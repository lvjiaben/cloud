// This file handles the bounded marketing campaign detail endpoint.

package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) MarketingCampaignDetail(ctx context.Context, req *v1.MarketingCampaignDetailReq) (*v1.MarketingCampaignDetailRes, error) {
	detail, err := c.marketingDirectory.GetCampaign(ctx, req.CampaignNo)
	if err != nil {
		return nil, err
	}
	return toAPIMarketingCampaignDetail(detail), nil
}
