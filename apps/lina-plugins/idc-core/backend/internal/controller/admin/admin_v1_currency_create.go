// This file delegates authorized currency configuration to the shared owner service.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CurrencyCreate(ctx context.Context, req *v1.CurrencyCreateReq) (res *v1.CurrencyCreateRes, err error) {
	item, err := c.currencies.Create(ctx, req.CurrencyCode)
	if err != nil {
		return nil, err
	}
	return &v1.CurrencyCreateRes{CurrencyItem: projectCurrency(item)}, nil
}
