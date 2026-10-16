// This file delegates authorized currency configuration to the shared owner service.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CurrencyList(ctx context.Context, req *v1.CurrencyListReq) (res *v1.CurrencyListRes, err error) {
	page, err := c.currencies.List(ctx, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	result := &v1.CurrencyListRes{Items: make([]v1.CurrencyItem, 0, len(page.Items)), Total: page.Total}
	for _, item := range page.Items {
		result.Items = append(result.Items, projectCurrency(item))
	}
	return result, nil
}
