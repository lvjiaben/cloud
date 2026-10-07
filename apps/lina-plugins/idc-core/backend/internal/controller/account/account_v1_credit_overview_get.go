// This file serves the current customer's bounded credit overview.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

// CreditOverviewGet returns the current customer's bounded credit overview.
func (c *ControllerV1) CreditOverviewGet(ctx context.Context, req *v1.CreditOverviewGetReq) (res *v1.CreditOverviewGetRes, err error) {
	overview, err := c.credit.GetAccountCreditOverview(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.CreditOverviewGetRes{Overview: toAPICreditOverview(overview)}, nil
}
