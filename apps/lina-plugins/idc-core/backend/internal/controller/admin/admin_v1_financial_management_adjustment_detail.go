package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementAdjustmentDetail(ctx context.Context, req *v1.FinancialManagementAdjustmentDetailReq) (res *v1.FinancialManagementAdjustmentDetailRes, err error) {
	detail, err := c.finance.GetManagementAdjustment(ctx, req.AdjustmentNo)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementAdjustmentDetailRes{Detail: toAPIFinancialManagementAdjustmentDetail(detail)}, nil
}
