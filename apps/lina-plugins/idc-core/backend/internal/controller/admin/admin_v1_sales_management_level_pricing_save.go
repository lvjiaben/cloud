// This generated controller delegates versioned tier pricing to its owner.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) SalesManagementLevelPricingSave(ctx context.Context, req *v1.SalesManagementLevelPricingSaveReq) (res *v1.SalesManagementLevelPricingSaveRes, err error) {
	item, err := c.salesManagement.SaveLevelPricing(ctx, req.LevelNo, req.PercentageBasisPoints, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementLevelPricingSaveRes{Item: v1.SalesManagementLevelPricing{LevelNo: item.LevelNo, PercentageBasisPoints: item.PercentageBasisPoints, Version: item.Version}}, nil
}
