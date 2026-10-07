package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementOverviewGet(ctx context.Context, req *v1.FinancialManagementOverviewGetReq) (res *v1.FinancialManagementOverviewGetRes, err error) {
	overview, err := c.finance.GetManagementFinancialOverview(ctx)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementOverviewGetRes{Overview: toAPIFinancialManagementOverview(overview)}, nil
}
