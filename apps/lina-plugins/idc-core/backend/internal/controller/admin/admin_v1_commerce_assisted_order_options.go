// This controller exposes configured currencies for a permission-checked assistance action.
package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CommerceAssistedOrderOptions(ctx context.Context, req *v1.CommerceAssistedOrderOptionsReq) (res *v1.CommerceAssistedOrderOptionsRes, err error) {
	currencies, err := c.commerceAssistedQuotes.AssistedOrderCurrencies(ctx, req.CustomerNo)
	if err != nil {
		return nil, err
	}
	return &v1.CommerceAssistedOrderOptionsRes{Currencies: currencies}, nil
}
