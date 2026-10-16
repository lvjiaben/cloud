// This file delegates configuration deletion to the authorized currency owner.
package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CurrencyDelete(ctx context.Context, req *v1.CurrencyDeleteReq) (*v1.CurrencyDeleteRes, error) {
	if err := c.currencies.Delete(ctx, req.CurrencyCode, req.ExpectedVersion); err != nil {
		return nil, err
	}
	return &v1.CurrencyDeleteRes{}, nil
}
