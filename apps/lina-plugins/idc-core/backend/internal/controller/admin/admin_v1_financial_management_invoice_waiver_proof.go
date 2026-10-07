package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementInvoiceWaiverProof(ctx context.Context, req *v1.FinancialManagementInvoiceWaiverProofReq) (res *v1.FinancialManagementInvoiceWaiverProofRes, err error) {
	intent, err := managementInvoiceWaiverIntent(req.FinancialManagementInvoiceWaiverIntent)
	if err != nil {
		return nil, err
	}
	proof, err := c.financeCommands.IssueInvoiceWaiverProof(ctx, intent)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementInvoiceWaiverProofRes{Proof: toAPIManagementActionProof(proof)}, nil
}
