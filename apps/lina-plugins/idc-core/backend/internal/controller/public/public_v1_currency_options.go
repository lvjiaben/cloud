// This file returns public merchant denomination choices without customer or financial data.
package public

import (
	"context"

	"lina-plugin-idc-core/backend/api/public/v1"
)

func (c *ControllerV1) CurrencyOptions(ctx context.Context, req *v1.CurrencyOptionsReq) (res *v1.CurrencyOptionsRes, err error) {
	codes, err := c.currencies.Enabled(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.CurrencyOptionsRes{Currencies: codes}, nil
}
