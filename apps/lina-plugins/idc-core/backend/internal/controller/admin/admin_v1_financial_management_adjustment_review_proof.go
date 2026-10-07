package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementAdjustmentReviewProof(ctx context.Context, req *v1.FinancialManagementAdjustmentReviewProofReq) (res *v1.FinancialManagementAdjustmentReviewProofRes, err error) {
	proof, err := c.financeCommands.IssueAdjustmentReviewProof(
		ctx, managementAdjustmentReviewIntent(req.AdjustmentNo, req.FinancialManagementAdjustmentReviewIntent),
	)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementAdjustmentReviewProofRes{Proof: toAPIManagementActionProof(proof)}, nil
}
