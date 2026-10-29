// This file executes the generated customer balance create route through the
// authorized financial command service; it never writes ledger state directly.

package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementBalanceAdjustmentCreate(ctx context.Context, req *v1.FinancialManagementBalanceAdjustmentCreateReq) (*v1.FinancialManagementBalanceAdjustmentCreateRes, error) {
	intent, err := managementBalanceAdjustmentIntent(req.FinancialManagementBalanceAdjustmentIntent)
	if err != nil {
		return nil, err
	}
	result, err := c.financeCommands.CreateBalanceAdjustment(ctx, intent, req.ProofToken)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementBalanceAdjustmentCreateRes{Adjustment: toAPIManagementAdjustmentMutation(result)}, nil
}
