package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialManagementTransactionList(ctx context.Context, req *v1.FinancialManagementTransactionListReq) (res *v1.FinancialManagementTransactionListRes, err error) {
	page, err := c.finance.ListManagementTransactions(ctx, billing.ManagementTransactionDirectoryFilter{
		CustomerNo: req.CustomerNo, TransactionTypeCode: req.TransactionTypeCode,
		CurrencyCode: req.CurrencyCode, Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialManagementTransactionListRes{
		Total:        page.Total,
		Transactions: toAPIFinancialManagementTransactions(page.Transactions), NextCursor: page.NextCursor,
	}, nil
}
