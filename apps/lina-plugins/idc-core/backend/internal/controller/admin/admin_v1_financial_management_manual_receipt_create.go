package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementManualReceiptCreate(ctx context.Context, req *v1.FinancialManagementManualReceiptCreateReq) (res *v1.FinancialManagementManualReceiptCreateRes, err error) {
	intent, err := managementManualReceiptIntent(req.FinancialManagementManualReceiptIntent)
	if err != nil {
		return nil, err
	}
	result, err := c.financeCommands.CreateManualReceipt(ctx, intent, req.ProofToken)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementManualReceiptCreateRes{Adjustment: toAPIManagementAdjustmentMutation(result)}, nil
}
