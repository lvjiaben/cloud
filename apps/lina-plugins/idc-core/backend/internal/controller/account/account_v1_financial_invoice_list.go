package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialInvoiceList(ctx context.Context, req *v1.FinancialInvoiceListReq) (res *v1.FinancialInvoiceListRes, err error) {
	page, err := c.finance.ListAccountInvoices(ctx, billing.InvoiceDirectoryFilter{
		StatusCode: req.StatusCode, CurrencyCode: req.CurrencyCode,
		Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialInvoiceListRes{
		Invoices: toAPIFinancialInvoices(page.Invoices), NextCursor: page.NextCursor,
	}, nil
}
