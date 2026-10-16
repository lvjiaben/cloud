// This generated controller projects customer products without exposing configuration secrets.
package admin

import (
	"context"
	"fmt"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

// CustomerProducts loads one authorized page and converts times at the API boundary.
func (c *ControllerV1) CustomerProducts(ctx context.Context, req *v1.CustomerProductsReq) (*v1.CustomerProductsRes, error) {
	page, err := c.customerManagement.SearchProducts(ctx, req.CustomerNo, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.CustomerProductsRes{Items: []v1.CustomerProduct{}, Total: page.Total}
	for _, row := range page.Items {
		item := v1.CustomerProduct{Number: row.Number, DeliveryKind: string(row.DeliveryKind), OrderNo: row.OrderNo, ProductNo: row.ProductNo, NameZhCn: row.NameZhCn, NameEnUs: row.NameEnUs, Information: row.Information, InformationEnUs: row.InformationEnUs, AmountMinor: fmt.Sprint(row.AmountMinor), CurrencyCode: row.CurrencyCode, ProductType: row.ProductType, PluginID: row.PluginID, PeriodUnit: row.PeriodUnit, PeriodCount: row.PeriodCount, CreatedAt: row.CreatedAt.UnixMilli(), Status: row.Status}
		if row.ExpiresAt != nil {
			value := row.ExpiresAt.UnixMilli()
			item.ExpiresAt = &value
		}
		result.Items = append(result.Items, item)
	}
	return result, nil
}
