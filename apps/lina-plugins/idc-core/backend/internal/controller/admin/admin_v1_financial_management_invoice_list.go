package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialManagementInvoiceList(ctx context.Context, req *v1.FinancialManagementInvoiceListReq) (res *v1.FinancialManagementInvoiceListRes, err error) {
	page, err := c.finance.ListManagementInvoices(ctx, billing.ManagementInvoiceDirectoryFilter{
		CustomerNo: req.CustomerNo, StatusCode: req.StatusCode, CurrencyCode: req.CurrencyCode,
		Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementInvoiceListRes{
		Total:    page.Total,
		Invoices: toAPIFinancialManagementInvoices(page.Invoices), NextCursor: page.NextCursor,
	}, nil
}
