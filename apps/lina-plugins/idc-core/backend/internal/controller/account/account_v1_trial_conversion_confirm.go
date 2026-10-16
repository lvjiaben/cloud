// This file handles owned trial conversion confirm using the startup renewal service.

package account

import (
	"context"
	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/renewal"
)

func (c *ControllerV1) TrialConversionConfirm(ctx context.Context, req *v1.TrialConversionConfirmReq) (res *v1.TrialConversionConfirmRes, err error) {
	operation, err := c.renewals.ConfirmCurrentCustomer(ctx, renewal.ConfirmInput{QuoteNo: req.QuoteNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.TrialConversionConfirmRes{Operation: toAPITrialConversionOperation(operation)}, nil
}

// toAPITrialConversionOperation exposes only customer-safe operation identity.
func toAPITrialConversionOperation(operation *renewal.Operation) *v1.TrialConversionOperation {
	if operation == nil {
		return nil
	}
	return &v1.TrialConversionOperation{OperationNo: operation.OperationNo, InvoiceNo: operation.InvoiceNo, StatusCode: operation.Status}
}
