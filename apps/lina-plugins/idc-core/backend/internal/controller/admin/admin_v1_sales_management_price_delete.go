// This generated controller delegates authorized price management to its owner.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) SalesManagementPriceDelete(ctx context.Context, req *v1.SalesManagementPriceDeleteReq) (res *v1.SalesManagementPriceDeleteRes, err error) {
	item, err := c.salesManagement.DeletePrice(ctx, req.PriceNo, req.ExpectedVersion)
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementPriceDeleteRes{Item: salesManagementPrice(item)}, nil
}
