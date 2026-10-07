package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

func (c *ControllerV1) FinancialTransactionList(ctx context.Context, req *v1.FinancialTransactionListReq) (res *v1.FinancialTransactionListRes, err error) {
	page, err := c.finance.ListAccountTransactions(ctx, billing.TransactionDirectoryFilter{
		CurrencyCode: req.CurrencyCode, AccountType: req.AccountType,
		Cursor: req.Cursor, Limit: req.Limit,
	})
	if err != nil {
		return nil, err
	}
	return &v1.FinancialTransactionListRes{
		Transactions: toAPIFinancialTransactions(page.Transactions), NextCursor: page.NextCursor,
	}, nil
}
