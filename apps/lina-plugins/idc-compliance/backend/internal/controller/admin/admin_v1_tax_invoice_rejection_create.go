// This file rejects one pending tax-invoice request.
package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicemanagement"
)

// TaxInvoiceRejectionCreate closes one pending request with a stable reason.
func (c *ControllerV1) TaxInvoiceRejectionCreate(
	ctx context.Context,
	req *v1.TaxInvoiceRejectionCreateReq,
) (*v1.TaxInvoiceRejectionCreateRes, error) {
	result, err := c.taxInvoiceManagement.Reject(ctx, taxinvoicemanagement.CloseInput{
		TaxInvoiceNo: req.TaxInvoiceNo, ReasonCode: req.ReasonCode,
		InternalNote: req.InternalNote, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.TaxInvoiceRejectionCreateRes{Result: taxInvoiceActionResult(result)}, nil
}
