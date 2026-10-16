// This generated controller delegates versioned tier pricing to its owner.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) SalesManagementLevelPricingGet(ctx context.Context, req *v1.SalesManagementLevelPricingGetReq) (res *v1.SalesManagementLevelPricingGetRes, err error) {
	item, err := c.salesManagement.GetLevelPricing(ctx, req.LevelNo)
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementLevelPricingGetRes{Item: v1.SalesManagementLevelPricing{LevelNo: item.LevelNo, PercentageBasisPoints: item.PercentageBasisPoints, Version: item.Version}}, nil
}
