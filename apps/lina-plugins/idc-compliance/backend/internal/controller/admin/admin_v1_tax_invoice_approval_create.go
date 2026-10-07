// This file confirms the provider-supported tax-invoice type.
package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicemanagement"
)

// TaxInvoiceApprovalCreate approves or safely reports the closed capability gate.
func (c *ControllerV1) TaxInvoiceApprovalCreate(
	ctx context.Context,
	req *v1.TaxInvoiceApprovalCreateReq,
) (*v1.TaxInvoiceApprovalCreateRes, error) {
	result, err := c.taxInvoiceManagement.Approve(ctx, taxinvoicemanagement.ReviewInput{
		TaxInvoiceNo: req.TaxInvoiceNo, InvoiceTypeCode: req.InvoiceTypeCode,
		ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.TaxInvoiceApprovalCreateRes{Result: taxInvoiceActionResult(result)}, nil
}
