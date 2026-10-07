package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementAdjustmentPolicyProof(ctx context.Context, req *v1.FinancialManagementAdjustmentPolicyProofReq) (res *v1.FinancialManagementAdjustmentPolicyProofRes, err error) {
	intent, err := managementAdjustmentPolicyIntent(req.FinancialManagementAdjustmentPolicyIntent)
	if err != nil {
		return nil, err
	}
	proof, err := c.financeCommands.IssueAdjustmentPolicyProof(ctx, intent)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementAdjustmentPolicyProofRes{Proof: toAPIManagementActionProof(proof)}, nil
}
