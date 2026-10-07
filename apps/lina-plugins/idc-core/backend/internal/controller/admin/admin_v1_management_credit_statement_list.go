// This file serves the authorized credit statement management list.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// ManagementCreditStatementList returns one authorized credit statement page.
func (c *ControllerV1) ManagementCreditStatementList(ctx context.Context, req *v1.ManagementCreditStatementListReq) (res *v1.ManagementCreditStatementListRes, err error) {
	page, err := c.credit.ListManagementCreditStatements(ctx, billing.CreditStatementDirectoryFilter{
		CustomerNo: req.CustomerNo, StatusCode: req.StatusCode,
		CurrencyCode: req.CurrencyCode, Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditStatementListRes{
		Statements: toAPIManagementCreditStatements(page.Statements), NextCursor: page.NextCursor, Total: page.Total,
	}, nil
}
