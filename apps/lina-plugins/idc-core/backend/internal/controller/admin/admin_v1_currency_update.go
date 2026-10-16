// This file delegates authorized currency configuration to the shared owner service.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CurrencyUpdate(ctx context.Context, req *v1.CurrencyUpdateReq) (res *v1.CurrencyUpdateRes, err error) {
	item, err := c.currencies.Update(ctx, req.CurrencyCode, req.ExpectedVersion, req.Enabled, req.MakeDefault)
	if err != nil {
		return nil, err
	}
	return &v1.CurrencyUpdateRes{CurrencyItem: projectCurrency(item)}, nil
}
