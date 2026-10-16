// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementPriceCreate(ctx context.Context, req *v1.SalesManagementPriceCreateReq) (*v1.SalesManagementPriceCreateRes, error) {
	item, err := c.salesManagement.SavePrice(ctx, salesmanagement.PriceInput{PriceNo: req.PriceNo, CustomerNo: req.CustomerNo, LevelNo: req.LevelNo, ProductNo: req.ProductNo, CurrencyCode: req.CurrencyCode, ChargeTypeCode: req.ChargeTypeCode, PeriodUnitCode: req.PeriodUnitCode, PeriodCount: req.PeriodCount, AdjustmentType: req.AdjustmentType, Status: req.Status, FixedPriceMinor: req.FixedPriceMinor, DiscountBasisPoints: req.DiscountBasisPoints, ExpectedVersion: req.ExpectedVersion})
	if err != nil {
		return nil, err
	}
	return &v1.SalesManagementPriceCreateRes{Item: salesManagementPrice(item)}, nil
}
