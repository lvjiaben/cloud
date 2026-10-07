// This file serves the current customer's bounded credit statement list.

package account

import (
	"context"

	"lina-plugin-idc-core/backend/api/account/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// CreditStatementList returns one owned credit statement page.
func (c *ControllerV1) CreditStatementList(ctx context.Context, req *v1.CreditStatementListReq) (res *v1.CreditStatementListRes, err error) {
	page, err := c.credit.ListAccountCreditStatements(ctx, billing.CreditStatementDirectoryFilter{
		StatusCode: req.StatusCode, CurrencyCode: req.CurrencyCode,
		Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.CreditStatementListRes{
		Statements: toAPICreditStatements(page.Statements), NextCursor: page.NextCursor, Total: page.Total,
	}, nil
}
