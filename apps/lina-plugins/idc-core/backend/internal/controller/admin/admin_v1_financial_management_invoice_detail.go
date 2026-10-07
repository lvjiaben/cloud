package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialManagementInvoiceDetail(ctx context.Context, req *v1.FinancialManagementInvoiceDetailReq) (res *v1.FinancialManagementInvoiceDetailRes, err error) {
	detail, err := c.finance.GetManagementInvoice(ctx, req.InvoiceNo, billing.InvoiceItemDirectoryFilter{
		Cursor: req.ItemCursor, Limit: req.ItemLimit,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementInvoiceDetailRes{Detail: toAPIFinancialManagementInvoiceDetail(detail)}, nil
}
