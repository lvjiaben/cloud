// This generated controller delegates scoped sales operations to its owner.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementPriceList(ctx context.Context, req *v1.SalesManagementPriceListReq) (*v1.SalesManagementPriceListRes, error) {
	page, err := c.salesManagement.ListPrices(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search, ProductNo: req.ProductNo, CustomerNo: req.CustomerNo, LevelNo: req.LevelNo, Target: req.Target})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementPriceListRes{Items: []v1.SalesManagementPrice{}, Total: page.Total}
	for _, item := range page.Items {
		res.Items = append(res.Items, salesManagementPrice(item))
	}
	return res, nil
}
