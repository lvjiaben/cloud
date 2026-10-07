// This file handles the bounded marketing overview administration endpoint.

package admin

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/admin/v1"
)

// MarketingOverviewGet delegates the authorized read and maps its stable API projection.
func (c *ControllerV1) MarketingOverviewGet(
	ctx context.Context,
	_ *v1.MarketingOverviewGetReq,
) (*v1.MarketingOverviewGetRes, error) {
	overview, err := c.marketingOverview.GetOverview(ctx)
	if err != nil {
		return nil, err
	}
	return toAPIMarketingOverview(overview), nil
}
