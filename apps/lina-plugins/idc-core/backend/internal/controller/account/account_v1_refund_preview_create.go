// This file handles current-customer refund impact preview requests.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
)

func (c *ControllerV1) RefundPreviewCreate(
	ctx context.Context,
	req *v1.RefundPreviewCreateReq,
) (*v1.RefundPreviewCreateRes, error) {
	input, err := toRefundPreviewInput(req.OrderNo, req.Selections)
	if err != nil {
		return nil, err
	}
	preview, err := c.refunds.PreviewOnlineRefund(ctx, input)
	if err != nil {
		return nil, err
	}
	return &v1.RefundPreviewCreateRes{Preview: toAPIRefundPreview(preview)}, nil
}
