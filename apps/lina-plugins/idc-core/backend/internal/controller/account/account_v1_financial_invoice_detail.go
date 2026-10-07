package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialInvoiceDetail(ctx context.Context, req *v1.FinancialInvoiceDetailReq) (res *v1.FinancialInvoiceDetailRes, err error) {
	detail, err := c.finance.GetAccountInvoice(ctx, req.InvoiceNo, billing.InvoiceItemDirectoryFilter{
		Cursor: req.ItemCursor, Limit: req.ItemLimit,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialInvoiceDetailRes{Detail: toAPIFinancialInvoiceDetail(detail)}, nil
}
