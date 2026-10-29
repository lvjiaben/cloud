// This file executes the generated customer balance proof route through the
// authorized financial command service; it never writes ledger state directly.

package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementBalanceAdjustmentProof(ctx context.Context, req *v1.FinancialManagementBalanceAdjustmentProofReq) (*v1.FinancialManagementBalanceAdjustmentProofRes, error) {
	intent, err := managementBalanceAdjustmentIntent(req.FinancialManagementBalanceAdjustmentIntent)
	if err != nil {
		return nil, err
	}
	proof, err := c.financeCommands.IssueBalanceAdjustmentProof(ctx, intent)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementBalanceAdjustmentProofRes{Proof: toAPIManagementActionProof(proof)}, nil
}
