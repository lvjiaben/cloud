// This file serves the authorized credit account management list.

package admin

import (
	"context"

	"lina-plugin-idc-core/backend/api/admin/v1"
	"lina-plugin-idc-core/backend/internal/service/billing"
)

// ManagementCreditAccountList returns one authorized credit account page.
func (c *ControllerV1) ManagementCreditAccountList(ctx context.Context, req *v1.ManagementCreditAccountListReq) (res *v1.ManagementCreditAccountListRes, err error) {
	page, err := c.credit.ListManagementCreditAccounts(ctx, billing.CreditAccountDirectoryFilter{
		CustomerNo: req.CustomerNo, StatusCode: req.StatusCode,
		CurrencyCode: req.CurrencyCode, Cursor: req.Cursor, Limit: req.Limit, Page: req.Page,
	})
	if err != nil {
		return nil, err
	}
	return &v1.ManagementCreditAccountListRes{
		Accounts: toAPIManagementCreditAccounts(page.Accounts), NextCursor: page.NextCursor, Total: page.Total,
	}, nil
}
