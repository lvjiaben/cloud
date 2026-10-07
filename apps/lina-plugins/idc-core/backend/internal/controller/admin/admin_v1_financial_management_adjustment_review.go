package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementAdjustmentReview(ctx context.Context, req *v1.FinancialManagementAdjustmentReviewReq) (res *v1.FinancialManagementAdjustmentReviewRes, err error) {
	result, err := c.financeCommands.ReviewAdjustment(
		ctx, managementAdjustmentReviewIntent(req.AdjustmentNo, req.FinancialManagementAdjustmentReviewIntent), req.ProofToken,
	)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementAdjustmentReviewRes{Adjustment: toAPIManagementAdjustmentMutation(result)}, nil
}
