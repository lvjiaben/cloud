package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/serveroperation"
)

func (c *ControllerV1) ServerIPChangeQuoteCreate(ctx context.Context,
	req *v1.ServerIPChangeQuoteCreateReq) (*v1.ServerIPChangeQuoteCreateRes, error) {
	quote, err := c.accessoryPricing.PreviewIPChangeCurrentCustomer(ctx, serveroperation.IPChangeQuoteInput{
		ServiceNo: req.ServiceNo, IdempotencyKey: req.IdempotencyKey})
	if err != nil {
		return nil, err
	}
	return &v1.ServerIPChangeQuoteCreateRes{Quote: &v1.ServerIPChangeQuote{
		QuoteNo: quote.QuoteNo, ServiceNo: quote.ServiceNo, CurrencyCode: quote.CurrencyCode,
		IPQuantity: quote.IPQuantity, FreeChange: quote.FreeChange, AmountMinor: quote.AmountMinor,
		ExpiresAt: quote.ExpiresAt.UnixMilli(),
	}}, nil
}

func (c *ControllerV1) ServerIPChangeCheckoutCreate(ctx context.Context,
	req *v1.ServerIPChangeCheckoutCreateReq) (*v1.ServerIPChangeCheckoutCreateRes, error) {
	checkout, err := c.accessoryPricing.CheckoutIPChangeCurrentCustomer(ctx,
		serveroperation.IPChangeCheckoutInput{QuoteNo: req.QuoteNo,
			IdempotencyKey: req.IdempotencyKey, ProofToken: req.ProofToken})
	if err != nil {
		return nil, err
	}
	return &v1.ServerIPChangeCheckoutCreateRes{Checkout: &v1.ServerIPChangeCheckout{
		OperationNo: checkout.OperationNo, OrderNo: checkout.OrderNo, InvoiceNo: checkout.InvoiceNo,
		ServiceNo: checkout.ServiceNo, Status: checkout.Status, AmountMinor: checkout.AmountMinor,
		CurrencyCode: checkout.CurrencyCode, PaymentDueAt: checkout.PaymentDueAt.UnixMilli(),
		PaymentRequired: checkout.PaymentRequired,
	}}, nil
}
