// This file exposes the staff refund decision for a conclusively failed delivery.

package admin

import (
	"context"
	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// RefundFailedFulfillment records a refund in the existing financial workflow.
func (c *ControllerV1) RefundFailedFulfillment(ctx context.Context, req *v1.RefundFailedFulfillmentReq) (*v1.RefundFailedFulfillmentRes, error) {
	result, err := c.refunds.RefundFailedFulfillment(ctx, billing.FailedFulfillmentRefundInput{IntentNo: req.IntentNo, ExpectedVersion: req.ExpectedVersion, IdempotencyKey: req.IdempotencyKey, Note: req.Note})
	if err != nil {
		return nil, err
	}
	return &v1.RefundFailedFulfillmentRes{Refund: toAPIRefundManagementMutation(result)}, nil
}
