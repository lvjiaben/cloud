package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/commerce"
)

func (c *ControllerV1) CommerceOrderCancel(ctx context.Context, req *v1.CommerceOrderCancelReq) (res *v1.CommerceOrderCancelRes, err error) {
	cancellation, err := c.commerce.CancelAccountOrder(ctx, commerce.OrderCancellationRequest{OrderNo: req.OrderNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.CommerceOrderCancelRes{Cancellation: toAPICommerceCancellation(cancellation)}, nil
}
