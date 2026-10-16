// This generated controller delegates authorized price management to its owner.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/internal/service/salesmanagement"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) SalesManagementPriceProducts(ctx context.Context, req *v1.SalesManagementPriceProductsReq) (res *v1.SalesManagementPriceProductsRes, err error) {
	page, err := c.salesManagement.PriceProductCandidates(ctx, salesmanagement.PageInput{Page: req.Page, PageSize: req.PageSize, Search: req.Search, ProductNo: req.ProductNo})
	if err != nil {
		return nil, err
	}
	res = &v1.SalesManagementPriceProductsRes{Items: []v1.SalesManagementProduct{}, Total: page.Total, Currencies: page.Currencies}
	for _, item := range page.Items {
		product := v1.SalesManagementProduct{ProductNo: item.ProductNo, Name: item.Name, EnglishName: item.EnglishName, Terms: []v1.SalesManagementProductTerm{}}
		for _, term := range item.Terms {
			product.Terms = append(product.Terms, v1.SalesManagementProductTerm{CurrencyCode: term.CurrencyCode, ChargeTypeCode: term.ChargeTypeCode, PeriodUnitCode: term.PeriodUnitCode, PeriodCount: term.PeriodCount})
		}
		res.Items = append(res.Items, product)
	}
	return res, nil
}
