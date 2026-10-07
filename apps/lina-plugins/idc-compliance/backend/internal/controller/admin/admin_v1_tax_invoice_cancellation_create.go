// This file safely cancels one known-unissued tax-invoice request.
package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicemanagement"
)

// TaxInvoiceCancellationCreate closes one safe request with a stable reason.
func (c *ControllerV1) TaxInvoiceCancellationCreate(
	ctx context.Context,
	req *v1.TaxInvoiceCancellationCreateReq,
) (*v1.TaxInvoiceCancellationCreateRes, error) {
	result, err := c.taxInvoiceManagement.Cancel(ctx, taxinvoicemanagement.CloseInput{
		TaxInvoiceNo: req.TaxInvoiceNo, ReasonCode: req.ReasonCode,
		InternalNote: req.InternalNote, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.TaxInvoiceCancellationCreateRes{Result: taxInvoiceActionResult(result)}, nil
}
