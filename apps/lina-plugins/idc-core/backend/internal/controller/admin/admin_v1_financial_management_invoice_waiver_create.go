package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementInvoiceWaiverCreate(ctx context.Context, req *v1.FinancialManagementInvoiceWaiverCreateReq) (res *v1.FinancialManagementInvoiceWaiverCreateRes, err error) {
	intent, err := managementInvoiceWaiverIntent(req.FinancialManagementInvoiceWaiverIntent)
	if err != nil {
		return nil, err
	}
	result, err := c.financeCommands.CreateInvoiceWaiver(ctx, intent, req.ProofToken)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementInvoiceWaiverCreateRes{Adjustment: toAPIManagementAdjustmentMutation(result)}, nil
}
