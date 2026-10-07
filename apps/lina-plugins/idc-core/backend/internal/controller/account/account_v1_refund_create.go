// This file handles current-customer online refund submissions.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) RefundCreate(
	ctx context.Context,
	req *v1.RefundCreateReq,
) (*v1.RefundCreateRes, error) {
	preview, err := toRefundPreviewInput(req.OrderNo, req.Selections)
	if err != nil {
		return nil, err
	}
	refund, err := c.refunds.SubmitOnlineRefund(ctx, billing.RefundSubmissionInput{
		PreviewDigest: req.PreviewDigest, Reason: billing.RefundReason(req.ReasonCode),
		ReasonDetail: req.ReasonDetail, IdempotencyKey: req.IdempotencyKey, Preview: preview,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RefundCreateRes{Refund: toAPIRefund(refund)}, nil
}
