package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) FinancialManagementManualReceiptProof(ctx context.Context, req *v1.FinancialManagementManualReceiptProofReq) (res *v1.FinancialManagementManualReceiptProofRes, err error) {
	intent, err := managementManualReceiptIntent(req.FinancialManagementManualReceiptIntent)
	if err != nil {
		return nil, err
	}
	proof, err := c.financeCommands.IssueManualReceiptProof(ctx, intent)
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementManualReceiptProofRes{Proof: toAPIManagementActionProof(proof)}, nil
}
