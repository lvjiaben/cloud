// This generated controller delegates staff product edits to the existing authorized owner workflow.
package admin

import (
	"context"
	account "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"

	"lina-plugin-idc-core/backend/api/admin/v1"
)

func (c *ControllerV1) CustomerServerResizeCheckout(ctx context.Context, req *v1.CustomerServerResizeCheckoutReq) (res *v1.CustomerServerResizeCheckoutRes, err error) {
	value, err := c.accessoryPricing.CheckoutResizeForStaff(ctx, req.ServiceNo, serveroperation.ResizeCheckoutInput{QuoteNo: req.QuoteNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, serveroperation.PublicError(err)
	}
	return &account.ServerResizeCheckoutCreateRes{Checkout: &account.ServerResizeCheckout{OperationNo: value.OperationNo, OrderNo: value.OrderNo, InvoiceNo: value.InvoiceNo,
		ServiceNo: value.ServiceNo, Direction: value.Direction, Status: value.Status, CurrencyCode: value.CurrencyCode, ChargeAmountMinor: value.ChargeAmountMinor,
		EffectiveAt: value.EffectiveAt.UnixMilli(), Reused: value.Reused}}, nil
}
