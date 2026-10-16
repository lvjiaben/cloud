// This generated controller returns bounded sales management candidates.
package admin

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"
)

func (c *ControllerV1) SalesManagementProducts(ctx context.Context, req *v1.SalesManagementProductsReq) (*v1.SalesManagementProductsRes, error) {
	page, err := c.salesManagement.ProductCandidates(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search})
	if err != nil {
		return nil, err
	}
	res := &v1.SalesManagementProductsRes{Items: []v1.SalesManagementProduct{}, Total: page.Total, Currencies: page.Currencies}
	for _, item := range page.Items {
		product := v1.SalesManagementProduct{ProductNo: item.ProductNo, Name: item.Name, EnglishName: item.EnglishName, Terms: []v1.SalesManagementProductTerm{}}
		for _, term := range item.Terms {
			product.Terms = append(product.Terms, v1.SalesManagementProductTerm{CurrencyCode: term.CurrencyCode, ChargeTypeCode: term.ChargeTypeCode, PeriodUnitCode: term.PeriodUnitCode, PeriodCount: term.PeriodCount})
		}
		res.Items = append(res.Items, product)
	}
	return res, nil
}
