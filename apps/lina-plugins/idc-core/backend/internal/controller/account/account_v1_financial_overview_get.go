package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) FinancialOverviewGet(ctx context.Context, req *v1.FinancialOverviewGetReq) (res *v1.FinancialOverviewGetRes, err error) {
	overview, err := c.finance.GetAccountFinancialOverview(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialOverviewGetRes{Overview: toAPIFinancialOverview(overview)}, nil
}
