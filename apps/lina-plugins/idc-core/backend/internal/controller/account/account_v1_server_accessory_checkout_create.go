package account

import (
	"context"

	v1 "lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerAccessoryCheckoutCreate(ctx context.Context,
	req *v1.ServerAccessoryCheckoutCreateReq) (*v1.ServerAccessoryCheckoutCreateRes, error) {
	checkout, err := c.accessoryPricing.CheckoutCurrentCustomer(ctx, serveroperation.AccessoryCheckoutInput{
		QuoteNo: req.QuoteNo, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ServerAccessoryCheckoutCreateRes{Checkout: toAPIAccessoryCheckout(checkout)}, nil
}

func toAPIAccessoryCheckout(value *serveroperation.AccessoryCheckout) *v1.ServerAccessoryCheckout {
	if value == nil {
		return nil
	}
	return &v1.ServerAccessoryCheckout{
		OperationNo: value.OperationNo, OrderNo: value.OrderNo, InvoiceNo: value.InvoiceNo,
		ServiceNo: value.ServiceNo, Status: value.Status, AmountMinor: value.AmountMinor,
		CurrencyCode: value.CurrencyCode, PaymentDueAt: value.PaymentDueAt.UnixMilli(),
		PaymentRequired: value.AmountMinor > 0, Reused: value.Reused,
	}
}
