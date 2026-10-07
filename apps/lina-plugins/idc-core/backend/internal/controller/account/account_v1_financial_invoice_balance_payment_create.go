package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialInvoiceBalancePaymentCreate(ctx context.Context, req *v1.FinancialInvoiceBalancePaymentCreateReq) (res *v1.FinancialInvoiceBalancePaymentCreateRes, err error) {
	payment, err := c.financeCommands.PayAccountInvoiceFromBalance(ctx, billing.AccountBalancePaymentInput{
		InvoiceNo: req.InvoiceNo, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialInvoiceBalancePaymentCreateRes{Payment: toAPIFinancialBalancePayment(payment)}, nil
}
