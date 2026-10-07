package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementAdjustmentPolicyConfigure(ctx context.Context, req *v1.FinancialManagementAdjustmentPolicyConfigureReq) (res *v1.FinancialManagementAdjustmentPolicyConfigureRes, err error) {
	intent, err := managementAdjustmentPolicyIntent(req.FinancialManagementAdjustmentPolicyIntent)
	if err != nil {
		return nil, err
	}
	result, err := c.financeCommands.ConfigureAdjustmentPolicy(ctx, intent, req.ProofToken)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementAdjustmentPolicyConfigureRes{Policy: toAPIManagementAdjustmentPolicyMutation(result)}, nil
}
