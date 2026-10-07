package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialInvoiceMergeCreate(ctx context.Context, req *v1.FinancialInvoiceMergeCreateReq) (res *v1.FinancialInvoiceMergeCreateRes, err error) {
	merged, err := c.financeCommands.MergeAccountInvoices(ctx, billing.AccountInvoiceMergeInput{
		InvoiceNos: append([]string(nil), req.InvoiceNos...), IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialInvoiceMergeCreateRes{Merge: toAPIFinancialInvoiceMerge(merged)}, nil
}
