package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialManagementTransactionDetail(ctx context.Context, req *v1.FinancialManagementTransactionDetailReq) (res *v1.FinancialManagementTransactionDetailRes, err error) {
	detail, err := c.finance.GetManagementTransaction(ctx, req.TransactionNo, billing.InvoiceItemDirectoryFilter{
		Cursor: req.EntryCursor, Limit: req.EntryLimit,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementTransactionDetailRes{Detail: toAPIFinancialManagementTransactionDetail(detail)}, nil
}
