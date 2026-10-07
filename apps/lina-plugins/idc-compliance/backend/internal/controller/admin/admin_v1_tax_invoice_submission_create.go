// This file queues one approved tax-invoice request locally.
package admin

import (
	"context"

	"lina-plugin-idc-compliance/backend/api/admin/v1"
	"lina-plugin-idc-compliance/backend/internal/service/taxinvoicemanagement"
)

// TaxInvoiceSubmissionCreate queues or safely reports the current gate blocker.
func (c *ControllerV1) TaxInvoiceSubmissionCreate(
	ctx context.Context,
	req *v1.TaxInvoiceSubmissionCreateReq,
) (*v1.TaxInvoiceSubmissionCreateRes, error) {
	result, err := c.taxInvoiceManagement.Submit(ctx, taxinvoicemanagement.SubmitInput{
		TaxInvoiceNo: req.TaxInvoiceNo, ExpectedVersion: req.ExpectedVersion,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.TaxInvoiceSubmissionCreateRes{Result: taxInvoiceActionResult(result)}, nil
}
